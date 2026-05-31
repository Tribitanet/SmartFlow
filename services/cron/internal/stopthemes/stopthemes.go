package stopthemes

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"smartFlow/internal/models"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var lastProcessedTime *time.Time

const ScoreThreshold = 0.8

type SimpleStopTheme struct {
	ID   uint
	Name string
}

type SimpleNews struct {
	ID   uint
	Body string
}

const apiURL = "https://router.huggingface.co/hf-inference/models/joeddav/xlm-roberta-large-xnli"

var apiKey string

type RequestBody struct {
	Inputs     string            `json:"inputs"`
	Parameters RequestParameters `json:"parameters"`
}

type RequestParameters struct {
	CandidateLabels    []string `json:"candidate_labels"`
	HypothesisTemplate string   `json:"hypothesis_template"`
	MultiLabel         bool     `json:"multi_label"`
}

type ModelResponse struct {
	Sequence string    `json:"sequence"`
	Labels   []string  `json:"labels"`
	Scores   []float64 `json:"scores"`
}

type StopThemeWithScore struct {
	StopThemeID uint
	Score       float64
}

type NewsWithStopThemes struct {
	NewsID     uint
	StopThemes []StopThemeWithScore
}

func init() {
	envPath := []string{
		"../../../.env",
		"../../../../.env",
		"../../../../../.env",
		".env",
	}

	var loaded bool
	for _, path := range envPath {
		if err := godotenv.Load(path); err == nil {
			loaded = true
			break
		}
	}

	if !loaded {
		log.Fatal("Не удалось найти .env файл")
	}

	apiKey = os.Getenv("HF_TOPICS_TOKEN")
	if apiKey == "" {
		log.Fatal("HF_TOPICS_TOKEN not set in .env")
	}
}

func getIdByStopThemeName(stopThemes []SimpleStopTheme, name string) uint {
	for _, st := range stopThemes {
		if st.Name == name {
			return st.ID
		}
	}
	return 0
}

func getUnprocessedNews(db *gorm.DB) ([]SimpleNews, error) {
	var news []models.News
	err := db.Where("stop_themes_checked_at IS NULL").Find(&news).Error
	if err != nil {
		return nil, err
	}

	var simpleNews []SimpleNews
	for _, n := range news {
		simpleNews = append(simpleNews, SimpleNews{
			ID:   n.ID,
			Body: n.Body,
		})
	}
	return simpleNews, nil
}

func getProcessedNews(db *gorm.DB) ([]SimpleNews, error) {
	var news []models.News
	err := db.Where("stop_themes_checked_at IS NOT NULL").Find(&news).Error
	if err != nil {
		return nil, err
	}

	var simpleNews []SimpleNews
	for _, n := range news {
		simpleNews = append(simpleNews, SimpleNews{
			ID:   n.ID,
			Body: n.Body,
		})
	}
	return simpleNews, nil
}

func getNewStopThemes(db *gorm.DB, since time.Time) ([]SimpleStopTheme, error) {
	var stopThemes []models.StopTheme
	err := db.Where("created_at > ?", since).Find(&stopThemes).Error
	if err != nil {
		return nil, err
	}

	var simpleStopThemes []SimpleStopTheme
	for _, st := range stopThemes {
		simpleStopThemes = append(simpleStopThemes, SimpleStopTheme{
			ID:   st.ID,
			Name: st.Name,
		})
	}
	return simpleStopThemes, nil
}

func getAllStopThemes(db *gorm.DB) ([]SimpleStopTheme, error) {
	var stopThemes []models.StopTheme
	err := db.Find(&stopThemes).Error

	var simpleStopThemes []SimpleStopTheme
	for _, st := range stopThemes {
		simpleStopThemes = append(simpleStopThemes, SimpleStopTheme{
			ID:   st.ID,
			Name: st.Name,
		})
	}
	return simpleStopThemes, err
}

func processNewsAgainstStopThemes(news []SimpleNews, stopThemes []SimpleStopTheme, db *gorm.DB) {
	for _, n := range news {
		newsWithStopThemes := getStopThemesForNews(n, stopThemes)
		filteredStopThemeIDs := filterStopThemes(newsWithStopThemes)

		now := time.Now()

		if len(filteredStopThemeIDs) == 0 {
			log.Printf("Новость ID=%d: стоп-тем не найдено", n.ID)
			db.Model(&models.News{}).Where("id = ?", n.ID).Update("stop_themes_checked_at", now)
			continue
		}

		var stopThemeModels []*models.StopTheme
		err := db.Where("id IN ?", filteredStopThemeIDs).Find(&stopThemeModels).Error
		if err != nil {
			log.Printf("Ошибка загрузки стоп-тем для новости ID=%d: %v", n.ID, err)
			continue
		}

		var newsModel models.News
		err = db.First(&newsModel, n.ID).Error
		if err != nil {
			log.Printf("Новость ID=%d не найдена в БД: %v", n.ID, err)
			continue
		}

		err = db.Model(&newsModel).Association("StopThemes").Append(stopThemeModels)
		if err != nil {
			log.Printf("Ошибка привязки стоп-тем к новости ID=%d: %v", n.ID, err)
			continue
		}

		db.Model(&models.News{}).Where("id = ?", n.ID).Update("stop_themes_checked_at", now)
		log.Printf("Новость ID=%d: привязано %d стоп-тем", n.ID, len(stopThemeModels))
	}
}

func getStopThemesForNews(news SimpleNews, stopThemes []SimpleStopTheme) NewsWithStopThemes {
	labels := make([]string, 0, len(stopThemes))
	for _, st := range stopThemes {
		labels = append(labels, st.Name)
	}

	payload := RequestBody{
		Inputs: news.Body,
		Parameters: RequestParameters{
			CandidateLabels:    labels,
			HypothesisTemplate: "Этот текст можно классифицировать как: {}.",
			MultiLabel:         true,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}

	request, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}

	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	if response.StatusCode != http.StatusOK {
		log.Fatalf("Hugging Face API error (status %d): %s", response.StatusCode, string(responseBody))
	}

	var modelResponse ModelResponse

	err = json.Unmarshal(responseBody, &modelResponse)
	if err != nil {
		log.Fatal(err)
	}

	var newsWithStopThemes NewsWithStopThemes

	for i, label := range modelResponse.Labels {
		stopThemeID := getIdByStopThemeName(stopThemes, label)
		if stopThemeID != 0 {
			newsWithStopThemes.StopThemes = append(newsWithStopThemes.StopThemes, StopThemeWithScore{
				StopThemeID: stopThemeID,
				Score:       modelResponse.Scores[i],
			})
		}
	}

	return newsWithStopThemes
}

func filterStopThemes(newsWithStopThemes NewsWithStopThemes) []uint {
	var filteredStopThemes []uint
	for _, st := range newsWithStopThemes.StopThemes {
		if st.Score > ScoreThreshold {
			filteredStopThemes = append(filteredStopThemes, st.StopThemeID)
		}
	}
	return filteredStopThemes
}

func CronStopThemesTask(db *gorm.DB) {
	now := time.Now()

	// 1. Новые новости (StopThemesCheckedAt IS NULL)
	newNews, err := getUnprocessedNews(db)
	if err != nil {
		log.Printf("Ошибка получения необработанных новостей: %v", err)
		return
	}

	// 2. Новые стоп-темы (CreatedAt > lastProcessedTime). При первом запуске — пусто
	var newStopThemes []SimpleStopTheme
	if lastProcessedTime != nil {
		newStopThemes, err = getNewStopThemes(db, *lastProcessedTime)
		if err != nil {
			log.Printf("Ошибка получения новых стоп-тем: %v", err)
			return
		}
	}

	hasNewNews := len(newNews) > 0
	hasNewStopThemes := len(newStopThemes) > 0

	if !hasNewNews && !hasNewStopThemes {
		log.Println("Нет новых новостей и стоп-тем, пропускаем")
		lastProcessedTime = &now
		return
	}

	// Сценарий 1 + 3: Новые новости - прогнать по ВСЕМ стоп-темам
	if hasNewNews {
		allStopThemes, err := getAllStopThemes(db)
		if err != nil {
			log.Printf("Ошибка получения всех стоп-тем: %v", err)
			return
		}

		log.Printf("Обработка %d новых новостей по %d стоп-темам", len(newNews), len(allStopThemes))
		processNewsAgainstStopThemes(newNews, allStopThemes, db)
	}

	// Сценарий 2 + 3: Новые стоп-темы - прогнать СТАРЫЕ новости по новым стоп-темам
	if hasNewStopThemes {
		oldNews, err := getProcessedNews(db)
		if err != nil {
			log.Printf("Ошибка получения старых новостей: %v", err)
			lastProcessedTime = &now
			return
		}
		log.Printf("Перепроверка %d старых новостей по %d новым стоп-темам", len(oldNews), len(newStopThemes))
		processNewsAgainstStopThemes(oldNews, newStopThemes, db)
	}

	lastProcessedTime = &now
	log.Println("CronStopThemesTask завершён")
}
