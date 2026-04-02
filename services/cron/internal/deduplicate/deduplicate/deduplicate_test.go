package deduplicate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"

	"smartFlow/internal/database"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/deduplicate/vectordb"

	"github.com/qdrant/go-client/qdrant"
)

const (
	backendURL = "http://localhost:8080"
	qdrantURL  = "http://localhost:6333"
)

// postJSON — вспомогательная функция для отправки POST-запросов с JSON телом
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

// httpDelete — вспомогательная функция для отправки DELETE-запросов
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

// setupClean — Шаг 1-3: Очистка БД и Qdrant
func setupClean(t *testing.T) {
	t.Log("Очистка новостей в БД...")
	if err := httpDelete(backendURL + "/news/delete-all-news"); err != nil {
		t.Fatalf("Не удалось очистить новости: %v", err)
	}

	t.Log("Удаление коллекции news в Qdrant...")
	_ = httpDelete(qdrantURL + "/collections/news") // Игнорируем ошибку, если коллекции нет
}

// setupSeed — Шаг 4-5: Создание канала и новостей
func setupSeed(t *testing.T) {
	t.Log("Создание тестового канала...")
	// Канал может уже существовать, поэтому ошибку игнорируем
	_ = postJSON(backendURL+"/channels/create-channel", map[string]string{
		"Link": "https://t.me/test_channel",
		"Name": "Тестовый канал",
	})

	// Тестовые новости: 3 о космосе, 2 об экономике, 1 шум
	newsItems := []map[string]interface{}{
		// Группа "Космос" (должны быть дубликатами друг друга)
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
		// Группа "Экономика" (должны быть дубликатами друг друга)
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
		// "Шум" — НЕ дубликат ни к одной из групп
		{
			"Body":        "Илон Маск представил новую бюджетную модель электромобиля Tesla Model 2 стоимостью около 25 тысяч долларов.",
			"MessageLink": "https://t.me/auto/66",
			"ChannelID":   1,
		},
	}

	for i, item := range newsItems {
		t.Logf("Создание новости %d...", i+1)
		if err := postJSON(backendURL+"/news/create-news", item); err != nil {
			t.Fatalf("Не удалось создать новость %d: %v", i+1, err)
		}
	}
}

// setupQdrantCollection — пересоздание коллекции Qdrant
func setupQdrantCollection(t *testing.T, client *qdrant.Client, ctx context.Context) {
	exist, err := vectordb.CollectionExists("news")
	if err != nil {
		t.Fatalf("Qdrant CollectionExists error: %v", err)
	}

	if !exist {
		t.Log("Создание коллекции news в Qdrant...")
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
}

func TestCronDeduplicateTask(t *testing.T) {
	// === 1. Подключение к PostgreSQL ===
	db, err := database.Init(database.GetDSN())
	if err != nil {
		t.Fatalf("Не удалось подключиться к PostgreSQL: %v", err)
	}

	// === 2. Подключение к Qdrant ===
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		t.Fatalf("Не удалось подключиться к Qdrant: %v", err)
	}
	ctx := context.Background()

	// === 3. Очистка ===
	setupClean(t)

	// === 4. Пересоздание коллекции Qdrant ===
	setupQdrantCollection(t, client, ctx)

	// === 5. Засеивание данных ===
	setupSeed(t)

	// === 6. Запуск дедупликации ===
	t.Log("Запуск CronDeduplicateTask...")
	CronDeduplicateTask(db, client, ctx)

	// === 7. Проверка результатов ===
	t.Log("Проверка дубликатов в базе данных...")

	var allNews []models.News
	if err := db.Preload("Duplicates").Find(&allNews).Error; err != nil {
		t.Fatalf("Не удалось загрузить новости с дубликатами: %v", err)
	}

	if len(allNews) != 6 {
		t.Fatalf("Ожидалось 6 новостей, получено %d", len(allNews))
	}

	for _, news := range allNews {
		log.Printf("Новость ID=%d, Body=%.50s..., Дубликатов=%d",
			news.ID, news.Body, len(news.Duplicates))
	}

	// Космические новости (первые 3) — должны иметь дубликаты
	for _, news := range allNews[:3] {
		if len(news.Duplicates) == 0 {
			t.Errorf("Космическая новость ID=%d должна иметь дубликаты, но имеет 0", news.ID)
		}
	}

	// Экономические новости (4 и 5) — должны иметь дубликат друг друга
	for _, news := range allNews[3:5] {
		if len(news.Duplicates) == 0 {
			t.Errorf("Экономическая новость ID=%d должна иметь дубликаты, но имеет 0", news.ID)
		}
	}

	// "Шум" (6) — НЕ должен иметь дубликатов
	noiseNews := allNews[5]
	if len(noiseNews.Duplicates) != 0 {
		t.Errorf("Новость-шум ID=%d НЕ должна иметь дубликатов, но имеет %d",
			noiseNews.ID, len(noiseNews.Duplicates))
	}

	t.Log("=== Тест дедупликации завершён ===")
}
