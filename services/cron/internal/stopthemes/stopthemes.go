package stopthemes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/logger"
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
		logger.Error("Не удалось найти .env файл")
		os.Exit(1)
	}

	apiKey = os.Getenv("HF_STOPTHEMES_TOKEN")
	if apiKey == "" {
		logger.Error("HF_STOPTHEMES_TOKEN не задан в .env")
		os.Exit(1)
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
		simpleNews = append(simpleNews, SimpleNews{ID: n.ID, Body: n.Body})
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
		simpleNews = append(simpleNews, SimpleNews{ID: n.ID, Body: n.Body})
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
		simpleStopThemes = append(simpleStopThemes, SimpleStopTheme{ID: st.ID, Name: st.Name})
	}
	return simpleStopThemes, nil
}

func getAllStopThemes(db *gorm.DB) ([]SimpleStopTheme, error) {
	var stopThemes []models.StopTheme
	err := db.Find(&stopThemes).Error

	var simpleStopThemes []SimpleStopTheme
	for _, st := range stopThemes {
		simpleStopThemes = append(simpleStopThemes, SimpleStopTheme{ID: st.ID, Name: st.Name})
	}
	return simpleStopThemes, err
}

func processNewsAgainstStopThemes(news []SimpleNews, stopThemes []SimpleStopTheme, db *gorm.DB) {
	for _, n := range news {
		newsWithStopThemes, err := getStopThemesForNews(n, stopThemes)
		if err != nil {
			logger.Error("ID=%d: ошибка запроса к HuggingFace: %v", n.ID, err)
			continue
		}

		filteredIDs := filterStopThemes(newsWithStopThemes)
		now := time.Now()

		if len(filteredIDs) == 0 {
			logger.Info("[SKIP] ID=%d: стоп-тем не найдено", n.ID)
			db.Model(&models.News{}).Where("id = ?", n.ID).Update("stop_themes_checked_at", now)
			continue
		}

		var stopThemeModels []*models.StopTheme
		if err := db.Where("id IN ?", filteredIDs).Find(&stopThemeModels).Error; err != nil {
			logger.Error("ID=%d: ошибка загрузки стоп-тем из БД: %v", n.ID, err)
			continue
		}

		var newsModel models.News
		if err := db.First(&newsModel, n.ID).Error; err != nil {
			logger.Error("ID=%d: новость не найдена в БД: %v", n.ID, err)
			continue
		}

		if err := db.Model(&newsModel).Association("StopThemes").Append(stopThemeModels); err != nil {
			logger.Error("ID=%d: ошибка привязки стоп-тем: %v", n.ID, err)
			continue
		}

		db.Model(&models.News{}).Where("id = ?", n.ID).Update("stop_themes_checked_at", now)

		names := make([]string, 0, len(stopThemeModels))
		for _, st := range stopThemeModels {
			names = append(names, st.Name)
		}
		logger.Info("[BLOCKED] ID=%d: %v", n.ID, names)
	}
}

func getStopThemesForNews(news SimpleNews, stopThemes []SimpleStopTheme) (NewsWithStopThemes, error) {
	labels := make([]string, 0, len(stopThemes))
	for _, st := range stopThemes {
		labels = append(labels, st.Name)
	}

	payload := RequestBody{
		Inputs: news.Body,
		Parameters: RequestParameters{
			CandidateLabels:    labels,
			HypothesisTemplate: "В этой новости речь идёт о: {}.",
			MultiLabel:         true,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return NewsWithStopThemes{}, fmt.Errorf("marshal: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return NewsWithStopThemes{}, fmt.Errorf("new request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return NewsWithStopThemes{}, fmt.Errorf("http: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return NewsWithStopThemes{}, fmt.Errorf("read body: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return NewsWithStopThemes{}, fmt.Errorf("HuggingFace API status %d: %s", response.StatusCode, string(responseBody))
	}

	var modelResponse ModelResponse
	if err := json.Unmarshal(responseBody, &modelResponse); err != nil {
		return NewsWithStopThemes{}, fmt.Errorf("unmarshal: %w", err)
	}

	var result NewsWithStopThemes
	for i, label := range modelResponse.Labels {
		id := getIdByStopThemeName(stopThemes, label)
		if id != 0 {
			result.StopThemes = append(result.StopThemes, StopThemeWithScore{
				StopThemeID: id,
				Score:       modelResponse.Scores[i],
			})
		}
	}

	return result, nil
}

func filterStopThemes(newsWithStopThemes NewsWithStopThemes) []uint {
	var filtered []uint
	for _, st := range newsWithStopThemes.StopThemes {
		if st.Score > ScoreThreshold {
			filtered = append(filtered, st.StopThemeID)
		}
	}
	return filtered
}

func CronStopThemesTask(db *gorm.DB) {
	logger.Section("Стоп-темы")
	now := time.Now()

	newNews, err := getUnprocessedNews(db)
	if err != nil {
		logger.Error("Ошибка получения необработанных новостей: %v", err)
		return
	}

	var newStopThemes []SimpleStopTheme
	if lastProcessedTime != nil {
		newStopThemes, err = getNewStopThemes(db, *lastProcessedTime)
		if err != nil {
			logger.Error("Ошибка получения новых стоп-тем: %v", err)
			return
		}
	}

	hasNewNews := len(newNews) > 0
	hasNewStopThemes := len(newStopThemes) > 0

	if !hasNewNews && !hasNewStopThemes {
		logger.Info("Нет новых новостей и стоп-тем, пропускаем")
		lastProcessedTime = &now
		logger.Done("Стоп-темы")
		return
	}

	// Новые новости — прогнать по всем стоп-темам
	if hasNewNews {
		allStopThemes, err := getAllStopThemes(db)
		if err != nil {
			logger.Error("Ошибка получения всех стоп-тем: %v", err)
			return
		}

		if len(allStopThemes) == 0 {
			logger.Warn("Стоп-тем нет в БД, помечаем %d новостей как проверенные", len(newNews))
			t := time.Now()
			for _, n := range newNews {
				db.Model(&models.News{}).Where("id = ?", n.ID).Update("stop_themes_checked_at", t)
			}
		} else {
			logger.Info("Новых новостей: %d, стоп-тем: %d", len(newNews), len(allStopThemes))
			processNewsAgainstStopThemes(newNews, allStopThemes, db)
		}
	}

	// Новые стоп-темы — перепроверить старые новости
	if hasNewStopThemes {
		oldNews, err := getProcessedNews(db)
		if err != nil {
			logger.Error("Ошибка получения старых новостей: %v", err)
			lastProcessedTime = &now
			return
		}
		logger.Info("Перепроверка %d старых новостей по %d новым стоп-темам", len(oldNews), len(newStopThemes))
		processNewsAgainstStopThemes(oldNews, newStopThemes, db)
	}

	lastProcessedTime = &now
	logger.Done("Стоп-темы")
}
