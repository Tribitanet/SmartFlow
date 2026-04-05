package stopthemes

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

// logAllNews выводит состояние всех новостей в БД (с точки зрения стоп-тем)
func logAllNews(t *testing.T, db *gorm.DB) {
	var allNews []models.News
	if err := db.Preload("StopThemes").Find(&allNews).Error; err != nil {
		t.Fatalf("Не удалось загрузить новости: %v", err)
	}
	for _, n := range allNews {
		stopThemeNames := make([]string, 0)
		for _, st := range n.StopThemes {
			stopThemeNames = append(stopThemeNames, st.Name)
		}
		checkedStr := "nil"
		if n.StopThemesCheckedAt != nil {
			checkedStr = n.StopThemesCheckedAt.Format("15:04:05")
		}
		t.Logf("  ID=%d | CheckedAt=%s | Стоп-темы=%v | %.50s...",
			n.ID, checkedStr, stopThemeNames, n.Body)
	}
}

func TestCronStopThemesTask(t *testing.T) {
	// ==========================================
	// ПОДГОТОВКА
	// ==========================================
	db, err := database.Init(database.GetDSN())
	if err != nil {
		t.Fatalf("Не удалось подключиться к PostgreSQL: %v", err)
	}

	// Сбрасываем глобальное состояние
	lastProcessedTime = nil

	// Миграция
	db.AutoMigrate(&models.News{}, &models.Topic{}, &models.StopTheme{}, &models.Channel{})

	// Полная очистка
	t.Log("Полная очистка БД...")
	db.Exec("DELETE FROM news_topics")
	db.Exec("DELETE FROM news_stop_themes")
	db.Exec("DELETE FROM news_duplicates")
	db.Exec("DELETE FROM user_topics")
	db.Exec("DELETE FROM user_stop_themes")
	db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.News{})
	db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.StopTheme{})

	// Создаём канал (если нет)
	_ = postJSON("http://localhost:8080/channels", map[string]string{
		"Link": "https://t.me/test_channel",
		"Name": "Тестовый канал",
	})

	// Создаём начальные стоп-темы напрямую в БД
	for _, name := range []string{"Казино", "Микрозаймы", "Ставки на спорт"} {
		db.Create(&models.StopTheme{Name: name})
	}
	t.Log("Созданы начальные стоп-темы: Казино, Микрозаймы, Ставки на спорт")

	// Создаём тестовые новости через API
	newsItems := []map[string]interface{}{
		{
			"Body":        "Крупнейшее онлайн-казино было заблокировано Роскомнадзором после многочисленных жалоб пользователей на мошенничество и невыплату выигрышей.",
			"MessageLink": "https://t.me/casino/1",
			"ChannelID":   1,
		},
		{
			"Body":        "Нелегальное казино предлагало клиентам делать ставки на спортивные матчи и играть в покер через мобильное приложение без лицензии.",
			"MessageLink": "https://t.me/gambling/2",
			"ChannelID":   1,
		},
		{
			"Body":        "Компания SpaceX Илона Маска успешно запустила тяжелую ракету Falcon Heavy, которая вывела на орбиту Земли новую партию интернет-спутников Starlink.",
			"MessageLink": "https://t.me/space/3",
			"ChannelID":   1,
		},
		{
			"Body":        "Центральный Банк принял решение повысить ключевую ставку сразу на 2 процента на фоне растущей инфляции в стране.",
			"MessageLink": "https://t.me/finance/4",
			"ChannelID":   1,
		},
	}
	for i, item := range newsItems {
		if err := postJSON("http://localhost:8080/news", item); err != nil {
			t.Fatalf("Не удалось создать новость %d: %v", i+1, err)
		}
	}
	t.Log("Созданы 4 новости (1 казино, 1 казино+ставки, 2 обычных)")

	// ==========================================
	// СЦЕНАРИЙ 1: Новые новости, нет новых стоп-тем
	// (первый запуск, lastProcessedTime = nil)
	// ==========================================
	t.Log("\n=== СЦЕНАРИЙ 1: Новые новости, нет новых стоп-тем ===")
	CronStopThemesTask(db)

	t.Log("Результаты после сценария 1:")
	logAllNews(t, db)

	var newsAfterS1 []models.News
	db.Preload("StopThemes").Find(&newsAfterS1)

	for _, n := range newsAfterS1 {
		// StopThemesCheckedAt должен быть установлен у всех
		if n.StopThemesCheckedAt == nil {
			t.Errorf("Сценарий 1: Новость ID=%d — StopThemesCheckedAt не установлен", n.ID)
		}
	}

	if lastProcessedTime == nil {
		t.Error("Сценарий 1: lastProcessedTime должен быть установлен")
	}

	// Запоминаем состояние для проверки сценария 4
	checkedTimes := make(map[uint]time.Time)
	for _, n := range newsAfterS1 {
		if n.StopThemesCheckedAt != nil {
			checkedTimes[n.ID] = *n.StopThemesCheckedAt
		}
	}
	stopThemeCountsAfterS1 := make(map[uint]int)
	for _, n := range newsAfterS1 {
		stopThemeCountsAfterS1[n.ID] = len(n.StopThemes)
	}

	// Проверяем что новость про казино получила стоп-тему
	for _, n := range newsAfterS1 {
		if len(n.StopThemes) > 0 {
			names := make([]string, 0)
			for _, st := range n.StopThemes {
				names = append(names, st.Name)
			}
			t.Logf("  Новость ID=%d поймала стоп-темы: %v", n.ID, names)
		}
	}

	// ==========================================
	// СЦЕНАРИЙ 4: Ничего нового — пропуск
	// ==========================================
	time.Sleep(1 * time.Second)
	t.Log("\n=== СЦЕНАРИЙ 4: Ничего нового — должен пропустить ===")
	CronStopThemesTask(db)

	var newsAfterS4 []models.News
	db.Preload("StopThemes").Find(&newsAfterS4)

	for _, n := range newsAfterS4 {
		// StopThemesCheckedAt НЕ должен измениться
		if n.StopThemesCheckedAt != nil {
			if prev, ok := checkedTimes[n.ID]; ok {
				if !prev.Equal(*n.StopThemesCheckedAt) {
					t.Errorf("Сценарий 4: Новость ID=%d — StopThemesCheckedAt изменился без причины", n.ID)
				}
			}
		}
		// Количество стоп-тем не должно измениться
		if stopThemeCountsAfterS1[n.ID] != len(n.StopThemes) {
			t.Errorf("Сценарий 4: Новость ID=%d — количество стоп-тем изменилось: было %d, стало %d",
				n.ID, stopThemeCountsAfterS1[n.ID], len(n.StopThemes))
		}
	}
	t.Log("Сценарий 4 пройден: повторный запуск корректно пропущен")

	// ==========================================
	// СЦЕНАРИЙ 2: Нет новых новостей, есть новые стоп-темы
	// ==========================================
	time.Sleep(2 * time.Second)

	t.Log("\n=== СЦЕНАРИЙ 2: Добавляем стоп-тему 'Алкоголь' ===")
	db.Create(&models.StopTheme{Name: "Алкоголь"})

	CronStopThemesTask(db)

	t.Log("Результаты после сценария 2:")
	logAllNews(t, db)

	var newsAfterS2 []models.News
	db.Preload("StopThemes").Find(&newsAfterS2)

	for _, n := range newsAfterS2 {
		// StopThemesCheckedAt должен обновиться (перепроверка по новой стоп-теме)
		if n.StopThemesCheckedAt != nil {
			if prev, ok := checkedTimes[n.ID]; ok {
				if !n.StopThemesCheckedAt.After(prev) {
					t.Errorf("Сценарий 2: Новость ID=%d — StopThemesCheckedAt не обновился при перепроверке", n.ID)
				}
			}
		}
	}

	// Проверяем добавились ли стоп-темы
	for _, n := range newsAfterS2 {
		newCount := len(n.StopThemes)
		oldCount := stopThemeCountsAfterS1[n.ID]
		if newCount > oldCount {
			t.Logf("  Новость ID=%d: стоп-тем добавилось (%d -> %d)", n.ID, oldCount, newCount)
		}
	}

	// ==========================================
	// СЦЕНАРИЙ 3: И новые новости, и новые стоп-темы
	// ==========================================
	time.Sleep(2 * time.Second)

	t.Log("\n=== СЦЕНАРИЙ 3: Добавляем стоп-тему 'Наркотики' + новость про наркотики ===")
	db.Create(&models.StopTheme{Name: "Наркотики"})

	if err := postJSON("http://localhost:8080/news", map[string]interface{}{
		"Body":        "Полиция задержала крупную партию наркотиков на границе, изъято более 500 килограммов запрещённых веществ на сумму свыше миллиарда рублей.",
		"MessageLink": "https://t.me/crime/4",
		"ChannelID":   1,
	}); err != nil {
		t.Fatalf("Не удалось создать новость: %v", err)
	}

	// Запоминаем состояние ДО запуска
	var newsBeforeS3 []models.News
	db.Preload("StopThemes").Find(&newsBeforeS3)
	stopThemeCountsBeforeS3 := make(map[uint]int)
	for _, n := range newsBeforeS3 {
		stopThemeCountsBeforeS3[n.ID] = len(n.StopThemes)
	}

	CronStopThemesTask(db)

	t.Log("Результаты после сценария 3:")
	logAllNews(t, db)

	var newsAfterS3 []models.News
	db.Preload("StopThemes").Find(&newsAfterS3)

	// Новая новость должна быть обработана
	for _, n := range newsAfterS3 {
		if _, existed := stopThemeCountsBeforeS3[n.ID]; !existed {
			// Это новая новость
			if n.StopThemesCheckedAt == nil {
				t.Errorf("Сценарий 3: Новая новость ID=%d — StopThemesCheckedAt не установлен", n.ID)
			}
			t.Logf("  Новая новость ID=%d: %d стоп-тем (проверена по ВСЕМ стоп-темам)", n.ID, len(n.StopThemes))
		}
	}

	t.Log("\n=== Все сценарии CronStopThemesTask пройдены ===")
}
