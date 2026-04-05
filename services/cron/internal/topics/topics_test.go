package topics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"smartFlow/internal/database"
	"smartFlow/internal/models"

	"gorm.io/gorm"
)

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

// logAllNews выводит состояние всех новостей в БД
func logAllNews(t *testing.T, db *gorm.DB) {
	var allNews []models.News
	if err := db.Preload("Topics").Find(&allNews).Error; err != nil {
		t.Fatalf("Не удалось загрузить новости: %v", err)
	}
	for _, n := range allNews {
		topicNames := make([]string, 0)
		for _, topic := range n.Topics {
			topicNames = append(topicNames, topic.Name)
		}
		checkedStr := "nil"
		if n.TopicsCheckedAt != nil {
			checkedStr = n.TopicsCheckedAt.Format("15:04:05")
		}
		t.Logf("  ID=%d | WithoutTopics=%v | CheckedAt=%s | Темы=%v | %.50s...",
			n.ID, n.WithoutTopics, checkedStr, topicNames, n.Body)
	}
}

func TestCronTopicsTask(t *testing.T) {
	// ==========================================
	// ПОДГОТОВКА
	// ==========================================
	db, err := database.Init(database.GetDSN())
	if err != nil {
		t.Fatalf("Не удалось подключиться к PostgreSQL: %v", err)
	}

	// Сбрасываем глобальное состояние
	lastProcessedTime = nil

	// Миграция (чтобы новые колонки created_at, topics_checked_at были в БД)
	db.AutoMigrate(&models.News{}, &models.Topic{}, &models.StopTheme{}, &models.Channel{})

	// Полная очистка: связи, новости, темы
	t.Log("Полная очистка БД...")
	db.Exec("DELETE FROM news_topics")
	db.Exec("DELETE FROM news_stop_themes")
	db.Exec("DELETE FROM news_duplicates")
	db.Exec("DELETE FROM user_topics")
	db.Exec("DELETE FROM user_stop_themes")
	db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.News{})
	db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Topic{})

	// Создаём канал (если нет)
	_ = postJSON("http://localhost:8080/channels", map[string]string{
		"Link": "https://t.me/test_channel",
		"Name": "Тестовый канал",
	})

	// Создаём начальные темы напрямую в БД
	for _, name := range []string{"Экономика", "Спорт"} {
		db.Create(&models.Topic{Name: name})
	}
	t.Log("Созданы начальные темы: Экономика, Спорт")

	// Создаём начальные новости через API
	newsItems := []map[string]interface{}{
		{
			"Body":        "Компания SpaceX Илона Маска успешно запустила тяжелую ракету Falcon Heavy, которая вывела на орбиту Земли новую партию интернет-спутников Starlink.",
			"MessageLink": "https://t.me/space/1",
			"ChannelID":   1,
		},
		{
			"Body":        "Центральный Банк принял решение повысить ключевую ставку сразу на 2 процента на фоне растущей инфляции в стране.",
			"MessageLink": "https://t.me/finance/2",
			"ChannelID":   1,
		},
		{
			"Body":        "Сборная России по футболу одержала победу над командой Бразилии со счётом 3:1 в товарищеском матче.",
			"MessageLink": "https://t.me/sport/3",
			"ChannelID":   1,
		},
	}
	for i, item := range newsItems {
		if err := postJSON("http://localhost:8080/news", item); err != nil {
			t.Fatalf("Не удалось создать новость %d: %v", i+1, err)
		}
	}
	t.Log("Созданы 3 новости")

	// ==========================================
	// СЦЕНАРИЙ 1: Новые новости, нет новых тем
	// (первый запуск, lastProcessedTime = nil)
	// ==========================================
	t.Log("\n=== СЦЕНАРИЙ 1: Новые новости, нет новых тем ===")
	CronTopicsTask(db)

	// Проверки:
	t.Log("Результаты после сценария 1:")
	logAllNews(t, db)

	var newsAfterS1 []models.News
	db.Preload("Topics").Find(&newsAfterS1)

	for _, n := range newsAfterS1 {
		// TopicsCheckedAt должен быть установлен у всех
		if n.TopicsCheckedAt == nil {
			t.Errorf("Сценарий 1: Новость ID=%d — TopicsCheckedAt не установлен", n.ID)
		}
		// Каждая новость должна иметь темы ИЛИ WithoutTopics=true
		if len(n.Topics) == 0 && !n.WithoutTopics {
			t.Errorf("Сценарий 1: Новость ID=%d — нет тем и WithoutTopics=false (некорректное состояние)", n.ID)
		}
	}

	if lastProcessedTime == nil {
		t.Error("Сценарий 1: lastProcessedTime должен быть установлен")
	}

	// Запоминаем TopicsCheckedAt для проверки сценария 4
	checkedTimes := make(map[uint]time.Time)
	for _, n := range newsAfterS1 {
		if n.TopicsCheckedAt != nil {
			checkedTimes[n.ID] = *n.TopicsCheckedAt
		}
	}
	topicCountsAfterS1 := make(map[uint]int)
	for _, n := range newsAfterS1 {
		topicCountsAfterS1[n.ID] = len(n.Topics)
	}

	// ==========================================
	// СЦЕНАРИЙ 4: Ничего нового — пропуск
	// ==========================================
	time.Sleep(1 * time.Second)
	t.Log("\n=== СЦЕНАРИЙ 4: Ничего нового — должен пропустить ===")
	CronTopicsTask(db)

	var newsAfterS4 []models.News
	db.Preload("Topics").Find(&newsAfterS4)

	for _, n := range newsAfterS4 {
		// TopicsCheckedAt НЕ должен измениться
		if n.TopicsCheckedAt != nil {
			if prev, ok := checkedTimes[n.ID]; ok {
				if !prev.Equal(*n.TopicsCheckedAt) {
					t.Errorf("Сценарий 4: Новость ID=%d — TopicsCheckedAt изменился без причины", n.ID)
				}
			}
		}
		// Количество тем не должно измениться
		if topicCountsAfterS1[n.ID] != len(n.Topics) {
			t.Errorf("Сценарий 4: Новость ID=%d — количество тем изменилось: было %d, стало %d",
				n.ID, topicCountsAfterS1[n.ID], len(n.Topics))
		}
	}
	t.Log("Сценарий 4 пройден: повторный запуск корректно пропущен")

	// ==========================================
	// СЦЕНАРИЙ 2: Нет новых новостей, есть новые темы
	// ==========================================
	time.Sleep(2 * time.Second) // чтобы CreatedAt новой темы > lastProcessedTime

	t.Log("\n=== СЦЕНАРИЙ 2: Добавляем темы 'Технологии и Наука' и 'Космос' ==="  )
	db.Create(&models.Topic{Name: "Технологии и Наука"})
	db.Create(&models.Topic{Name: "Космос"})

	CronTopicsTask(db)

	t.Log("Результаты после сценария 2:")
	logAllNews(t, db)

	var newsAfterS2 []models.News
	db.Preload("Topics").Find(&newsAfterS2)

	for _, n := range newsAfterS2 {
		// TopicsCheckedAt должен обновиться (перепроверка по новой теме)
		if n.TopicsCheckedAt != nil {
			if prev, ok := checkedTimes[n.ID]; ok {
				if !n.TopicsCheckedAt.After(prev) {
					t.Errorf("Сценарий 2: Новость ID=%d — TopicsCheckedAt не обновился при перепроверке", n.ID)
				}
			}
		}
	}

	// Новость о SpaceX могла получить тему "Технологии и Наука"
	for _, n := range newsAfterS2 {
		newCount := len(n.Topics)
		oldCount := topicCountsAfterS1[n.ID]
		if newCount > oldCount {
			t.Logf("  Новость ID=%d: тем добавилось (%d -> %d)", n.ID, oldCount, newCount)
		}
	}

	// ==========================================
	// СЦЕНАРИЙ 3: И новые новости, и новые темы
	// ==========================================
	time.Sleep(2 * time.Second)

	t.Log("\n=== СЦЕНАРИЙ 3: Добавляем тему 'Культура' + новость о культуре ===")
	db.Create(&models.Topic{Name: "Культура"})

	if err := postJSON("http://localhost:8080/news", map[string]interface{}{
		"Body":        "Государственный Эрмитаж представил публике уникальную экспозицию, объединившую редчайшие полотна французских импрессионистов из закрытых частных собраний.",
		"MessageLink": "https://t.me/culture/4",
		"ChannelID":   1,
	}); err != nil {
		t.Fatalf("Не удалось создать новость о культуре: %v", err)
	}

	// Запоминаем состояние ДО запуска
	var newsBeforeS3 []models.News
	db.Preload("Topics").Find(&newsBeforeS3)
	topicCountsBeforeS3 := make(map[uint]int)
	for _, n := range newsBeforeS3 {
		topicCountsBeforeS3[n.ID] = len(n.Topics)
	}

	CronTopicsTask(db)

	t.Log("Результаты после сценария 3:")
	logAllNews(t, db)

	var newsAfterS3 []models.News
	db.Preload("Topics").Find(&newsAfterS3)

	// Новая новость (о культуре) должна быть обработана
	for _, n := range newsAfterS3 {
		if _, existed := topicCountsBeforeS3[n.ID]; !existed {
			// Это новая новость
			if n.TopicsCheckedAt == nil {
				t.Errorf("Сценарий 3: Новая новость ID=%d — TopicsCheckedAt не установлен", n.ID)
			}
			if len(n.Topics) == 0 && !n.WithoutTopics {
				t.Errorf("Сценарий 3: Новая новость ID=%d — нет тем и WithoutTopics=false", n.ID)
			}
			t.Logf("  Новая новость ID=%d: %d тем (проверена по ВСЕМ темам)", n.ID, len(n.Topics))
		}
	}

	t.Log("\n=== Все сценарии CronTopicsTask пройдены ===")
}
