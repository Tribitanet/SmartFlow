package deduplicate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"smartFlow/internal/database"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/deduplicate/vectordb"

	"github.com/qdrant/go-client/qdrant"
	"gorm.io/gorm"
)

const (
	backendURL = "http://localhost:8080"
	qdrantURL  = "http://localhost:6333"
)

var testNewsItems = []map[string]interface{}{
	// Группа "Космос" — 3 дубликата
	{
		"Body":        "Компания SpaceX Илона Маска успешно запустила тяжелую ракету Falcon Heavy, которая вывела на орбиту Земли новую партию интернет-спутников Starlink.",
		"MessageLink": "https://t.me/space/1",
		"ChannelID":   1,
	},
	{
		"Body":        "Очередная партия спутников Starlink выведена на околоземную орбиту с помощью ракетоносителя Falcon Heavy от корпорации SpaceX.",
		"MessageLink": "https://t.me/tech/22",
		"ChannelID":   1,
	},
	{
		"Body":        "Тяжелая ракета Falcon Heavy успешно стартовала сегодня утром. SpaceX доставила на орбиту Земли новые телекоммуникационные спутники Starlink Илона Маска.",
		"MessageLink": "https://t.me/news/333",
		"ChannelID":   1,
	},
	// Группа "Экономика" — 2 дубликата
	{
		"Body":        "Центральный Банк принял решение повысить ключевую ставку сразу на 2 процента на фоне растущей инфляции в стране.",
		"MessageLink": "https://t.me/finance/44",
		"ChannelID":   1,
	},
	{
		"Body":        "Из-за ускоряющейся инфляции регулятор (ЦБ) вынужденно увеличил ставку рефинансирования на 200 базисных пунктов (2%).",
		"MessageLink": "https://t.me/bank/55",
		"ChannelID":   1,
	},
	// Шум — не дубликат ни к чему
	{
		"Body":        "Илон Маск представил новую бюджетную модель электромобиля Tesla Model 2 стоимостью около 25 тысяч долларов.",
		"MessageLink": "https://t.me/auto/66",
		"ChannelID":   1,
	},
}

// --- Хелперы ---

func postJSON(url string, body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("POST %s error: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s вернул статус %d", url, resp.StatusCode)
	}
	return nil
}

func httpDelete(url string) error {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s error: %w", url, err)
	}
	defer resp.Body.Close()
	return nil
}

// initTestEnv подключается к PostgreSQL и Qdrant, очищает данные и создаёт коллекцию
func initTestEnv(t *testing.T) (*gorm.DB, *qdrant.Client, context.Context) {
	t.Helper()

	db, err := database.Init(database.GetDSN())
	if err != nil {
		t.Fatalf("Не удалось подключиться к PostgreSQL: %v", err)
	}

	client, err := qdrant.NewClient(&qdrant.Config{Host: "localhost", Port: 6334})
	if err != nil {
		t.Fatalf("Не удалось подключиться к Qdrant: %v", err)
	}

	ctx := context.Background()

	// Очистка БД и Qdrant
	if err := httpDelete(backendURL + "/news"); err != nil {
		t.Fatalf("Не удалось очистить новости: %v", err)
	}
	_ = httpDelete(qdrantURL + "/collections/news")

	// Создание коллекции если нет
	exist, err := vectordb.CollectionExists("news")
	if err != nil {
		t.Fatalf("Qdrant CollectionExists error: %v", err)
	}
	if !exist {
		err = client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: "news",
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     1024,
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			t.Fatalf("Qdrant CreateCollection error: %v", err)
		}
	}

	// Создание тестового канала (игнорируем ошибку если уже есть)
	_ = postJSON(backendURL+"/channels", map[string]string{
		"Link": "https://t.me/test_channel",
		"Name": "Тестовый канал",
	})

	return db, client, ctx
}

// seedNews создаёт все тестовые новости через API
func seedNews(t *testing.T) {
	t.Helper()
	for i, item := range testNewsItems {
		if err := postJSON(backendURL+"/news", item); err != nil {
			t.Fatalf("Не удалось создать новость %d: %v", i+1, err)
		}
	}
}

// seedNewsWithDeduplication создаёт новости по одной, запуская дедупликацию после каждой вставки
func seedNewsWithDeduplication(t *testing.T, db *gorm.DB, client *qdrant.Client, ctx context.Context) {
	t.Helper()
	for i, item := range testNewsItems {
		if err := postJSON(backendURL+"/news", item); err != nil {
			t.Fatalf("Не удалось создать новость %d: %v", i+1, err)
		}
		CronDeduplicateTask(db, client, ctx)
	}
}

// loadNewsWithDuplicates загружает все новости из БД с предзагрузкой дубликатов
func loadNewsWithDuplicates(t *testing.T, db *gorm.DB) []models.News {
	t.Helper()
	var allNews []models.News
	if err := db.Preload("Duplicates").Find(&allNews).Error; err != nil {
		t.Fatalf("Не удалось загрузить новости с дубликатами: %v", err)
	}
	return allNews
}

// assertDuplicateGroups проверяет, что космос-новости и экономика-новости имеют дубликаты, а шум — нет
func assertDuplicateGroups(t *testing.T, allNews []models.News) {
	t.Helper()

	if len(allNews) != 6 {
		t.Fatalf("Ожидалось 6 новостей, получено %d", len(allNews))
	}

	for _, news := range allNews {
		t.Logf("Новость ID=%d, Body=%.50s..., Дубликатов=%d", news.ID, news.Body, len(news.Duplicates))
	}

	// Космос (первые 3) — должны иметь дубликаты
	for _, news := range allNews[:3] {
		if len(news.Duplicates) == 0 {
			t.Errorf("Космическая новость ID=%d должна иметь дубликаты, но имеет 0", news.ID)
		}
	}

	// Экономика (4 и 5) — должны иметь дубликаты друг друга
	for _, news := range allNews[3:5] {
		if len(news.Duplicates) == 0 {
			t.Errorf("Экономическая новость ID=%d должна иметь дубликаты, но имеет 0", news.ID)
		}
	}

	// Шум (6) — не должен иметь дубликатов
	noiseNews := allNews[5]
	if len(noiseNews.Duplicates) != 0 {
		t.Errorf("Новость-шум ID=%d НЕ должна иметь дубликатов, но имеет %d", noiseNews.ID, len(noiseNews.Duplicates))
	}
}

// --- Тесты ---

// TestDeduplicateIncremental — новости добавляются по одной, дедупликация запускается после каждой вставки
func TestDeduplicateIncremental(t *testing.T) {
	db, client, ctx := initTestEnv(t)
	seedNewsWithDeduplication(t, db, client, ctx)

	allNews := loadNewsWithDuplicates(t, db)
	assertDuplicateGroups(t, allNews)
}

// TestCheckDuplicate — вставляет 2 новости и проверяет, считает ли система их дубликатами.
// Меняй newsA, newsB и isDuplicate для ручной проверки конкретных пар.
func TestCheckDuplicate(t *testing.T) {
	newsA := "США уже начали вести переговоры с иранским руководством о прекращении огня. Об этом на своей странице в соцсети Truth Social сообщил американский лидер Дональд Трамп. «Соединенные Штаты Америки ведут серьезные переговоры с новым и более разумным режимом о прекращении наших военных операций в Иране. Достигнут большой прогресс», — написал хозяин Белого дома. В случае если мир не будет достигнут, а Ормузский пролив не будет открыт, подчеркивает Трамп, американцы «взорвут и полностью уничтожат все электростанции, нефтяные скважины и остров Харк (а возможно, и все опреснительные заводы!)». «Это будет местью за наших многочисленных солдат и других, которых Иран уничтожил и убил в течение 47-летнего террора старого режима», — подчеркнул глава государства. Ранее высокопоставленные источники газеты The Wall Street Journal рассказали, что Трамп рассматривает сценарий проведения операции по захвату иранского урана и завершению конфликта к середине апреля."
	newsB := "Новое руководство Ирана предложило США вступить в переговоры, сообщил The Atlantic американский президент Дональд Трамп. «Они хотят поговорить, и я согласился поговорить, поэтому я буду с ними разговаривать. Им следовало сделать это раньше. Им следовало предложить то, что было бы очень практично и легко сделать, раньше. Они слишком долго ждали», — рассказал Трамп. Президент США отказался отвечать на вопрос, состоятся ли переговоры 1 или 2 марта. Он уточнил, что некоторые из представителей Ирана, кто участвовал в переговорах с США в последние недели, уже погибли. Республиканец подчеркнул, что Тегеран должен был «сделать это раньше». «Они могли бы заключить сделку. Им следовало сделать это раньше. Они слишком хитрили», — пояснил Трамп. Американский лидер в телефонном интервью CNBC также отметил, что американская операция в Иране «продвигается очень хорошо». «Это очень жестокий режим, один из самых жестоких режимов в истории, — сказал Трамп. — Мы выполняем свою работу не только для себя, но и для всего мира. И все идет с опережением графика». Он добавил, что события разворачиваются в «очень позитивном направлении». Накануне глава Белого дома заявил, что Тегеран был близок к сделке, но отступил, из чего он сделал вывод, что на самом деле республика не хотела этого. По его словам, у него много вариантов выхода из ситуации: США могут «затянуть» операцию и «взять все под контроль или закончить за несколько дней и сказать иранцам, что мы снова увидимся через несколько лет, если они начнут восстанавливаться». «В любом случае им потребуется несколько лет, чтобы оправиться от этой атаки», — убежден Трамп. На днях The Wall Street Journal писала, что Иран на февральских переговорах в Женеве отверг условия США, которые требовали ликвидировать три ядерных объекта в Фордо, Натанзе и Исфахане и отдать им весь запас обогащенного урана."
	isDuplicate := true

	db, client, ctx := initTestEnv(t)

	// Вставляем 2 новости
	for i, body := range []string{newsA, newsB} {
		if err := postJSON(backendURL+"/news", map[string]interface{}{
			"Body":        body,
			"MessageLink": fmt.Sprintf("https://t.me/test/%d", i+1),
			"ChannelID":   1,
		}); err != nil {
			t.Fatalf("Не удалось создать новость %d: %v", i+1, err)
		}
	}

	CronDeduplicateTask(db, client, ctx)

	allNews := loadNewsWithDuplicates(t, db)
	if len(allNews) != 2 {
		t.Fatalf("Ожидалось 2 новости, получено %d", len(allNews))
	}

	newsAResult := allNews[0]
	newsBResult := allNews[1]

	// Запрос в Qdrant без порога — узнать фактический скор схожести
	recRes, err := client.GetPointsClient().Recommend(ctx, &qdrant.RecommendPoints{
		CollectionName: "news",
		Positive:       []*qdrant.PointId{qdrant.NewIDNum(uint64(newsBResult.ID))},
		Limit:          100,
		ScoreThreshold: qdrant.PtrOf(float32(0.0)),
	})
	if err == nil {
		for _, p := range recRes.GetResult() {
			if p.Id.GetNum() == uint64(newsAResult.ID) {
				t.Logf("Фактический скор схожести: %f", p.Score)
			}
		}
	}

	t.Logf("Новость A (ID=%d): %.80s...", newsAResult.ID, newsAResult.Body)
	t.Logf("Новость B (ID=%d): %.80s...", newsBResult.ID, newsBResult.Body)
	t.Logf("Дубликатов у A: %d, у B: %d", len(newsAResult.Duplicates), len(newsBResult.Duplicates))

	aHasDup := len(newsAResult.Duplicates) > 0
	bHasDup := len(newsBResult.Duplicates) > 0

	if isDuplicate {
		if !aHasDup || !bHasDup {
			t.Errorf("Ожидали дубликат, но система не распознала: A.dups=%d, B.dups=%d",
				len(newsAResult.Duplicates), len(newsBResult.Duplicates))
		} else {
			t.Log("OK: новости распознаны как дубликаты")
		}
	} else {
		if aHasDup || bHasDup {
			t.Errorf("Ожидали НЕ дубликат, но система нашла связь: A.dups=%d, B.dups=%d",
				len(newsAResult.Duplicates), len(newsBResult.Duplicates))
		} else {
			t.Log("OK: новости корректно распознаны как НЕ дубликаты")
		}
	}
}
