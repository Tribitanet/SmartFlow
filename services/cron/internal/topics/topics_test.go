package topics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"smartFlow/internal/database"
	"smartFlow/internal/models"
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

func TestGetTopicsForNews(t *testing.T) {
	// === 1. Подключение к БД ===
	db, err := database.Init(database.GetDSN())
	if err != nil {
		t.Fatalf("Не удалось подключиться к PostgreSQL: %v", err)
	}

	// === 2. Получаем все темы из БД ===
	topics, err := getAllTopics(db)
	if err != nil {
		t.Fatalf("Ошибка получения тем: %v", err)
	}
	t.Logf("Найдено тем: %d", len(topics))

	if len(topics) == 0 {
		t.Fatal("В базе нет тем (Topics). Сначала создайте пользователя с темами.")
	}

	for _, topic := range topics {
		t.Logf("  Тема: %s (ID=%d)", topic.Name, topic.ID)
	}

	// === 3. Тестовая новость (не из БД, просто текст) ===
	testNews := SimpleNews{
		ID:   0,
		Body: "Компания SpaceX Илона Маска успешно запустила тяжелую ракету Falcon Heavy, которая вывела на орбиту Земли новую партию интернет-спутников Starlink.",
	}

	t.Logf("Тестовая новость: %s", testNews.Body)

	// === 4. Вызываем классификацию ===
	t.Log("Отправляем запрос в Hugging Face API...")
	result := getTopicsForNews(testNews, topics)

	t.Logf("Найдено тем для новости: %d", len(result.Topics))
	for _, topic := range result.Topics {
		// Ищем название темы по ID
		var name string
		for _, t := range topics {
			if t.ID == topic.TopicID {
				name = t.Name
				break
			}
		}
		t.Logf("  Тема: %s (ID=%d), Score: %.4f", name, topic.TopicID, topic.Score)
	}

	if len(result.Topics) == 0 {
		t.Error("Ожидалась хотя бы одна тема для новости о SpaceX")
	}

	t.Log("=== Тест завершён ===")
}

func TestCronTopicsTask(t *testing.T) {
	// === 1. Подключение к БД ===
	db, err := database.Init(database.GetDSN())
	if err != nil {
		t.Fatalf("Не удалось подключиться к PostgreSQL: %v", err)
	}

	// === 2. Очистка ===
	t.Log("Очистка новостей...")
	httpDelete("http://localhost:8080/news/delete-all-news")

	// === 3. Создание канала (если нет) ===
	t.Log("Создание канала...")
	_ = postJSON("http://localhost:8080/channels/create-channel", map[string]string{
		"Link": "https://t.me/test_channel",
		"Name": "Тестовый канал",
	})

	// === 4. Создание тестовых новостей ===
	testNews := []map[string]interface{}{
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

	for i, item := range testNews {
		t.Logf("Создание новости %d...", i+1)
		if err := postJSON("http://localhost:8080/news/create-news", item); err != nil {
			t.Fatalf("Не удалось создать новость %d: %v", i+1, err)
		}
	}

	// === 5. Запуск CronTopicsTask ===
	t.Log("Запуск CronTopicsTask...")
	CronTopicsTask(db)

	// === 6. Проверка результатов ===
	t.Log("Проверка привязки тем к новостям...")

	var allNews []models.News
	if err := db.Preload("Topics").Find(&allNews).Error; err != nil {
		t.Fatalf("Не удалось загрузить новости с темами: %v", err)
	}

	for _, n := range allNews {
		topicNames := make([]string, 0)
		for _, topic := range n.Topics {
			topicNames = append(topicNames, topic.Name)
		}
		t.Logf("Новость ID=%d: темы=%v (Body=%.60s...)", n.ID, topicNames, n.Body)

		if len(n.Topics) == 0 {
			t.Errorf("Новость ID=%d должна иметь хотя бы одну тему", n.ID)
		}
	}

	t.Log("=== Тест CronTopicsTask завершён ===")
}
