package topics

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

const ScoreThreshold = 0.74

type SimpleTopic struct {
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

type TopicWithScore struct {
	TopicID uint
	Score   float64
}

type NewsWithTopics struct {
	NewsID uint
	Topics []TopicWithScore
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

	apiKey = os.Getenv("HF_TOPICS_TOKEN")
	if apiKey == "" {
		logger.Error("HF_TOPICS_TOKEN не задан в .env")
		os.Exit(1)
	}
}

func getIdByTopicName(topics []SimpleTopic, topicName string) uint {
	for _, topic := range topics {
		if topic.Name == topicName {
			return topic.ID
		}
	}
	return 0
}

func getUnprocessedNews(db *gorm.DB) ([]SimpleNews, error) {
	var news []models.News
	err := db.Where("topics_checked_at IS NULL").Find(&news).Error
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
	err := db.Where("topics_checked_at IS NOT NULL").Find(&news).Error
	if err != nil {
		return nil, err
	}

	var simpleNews []SimpleNews
	for _, n := range news {
		simpleNews = append(simpleNews, SimpleNews{ID: n.ID, Body: n.Body})
	}
	return simpleNews, nil
}

func getNewTopics(db *gorm.DB, since time.Time) ([]SimpleTopic, error) {
	var topics []models.Topic
	err := db.Where("created_at > ?", since).Find(&topics).Error
	if err != nil {
		return nil, err
	}

	var simpleTopics []SimpleTopic
	for _, t := range topics {
		simpleTopics = append(simpleTopics, SimpleTopic{ID: t.ID, Name: t.Name})
	}
	return simpleTopics, nil
}

func getAllTopics(db *gorm.DB) ([]SimpleTopic, error) {
	var topics []models.Topic
	err := db.Find(&topics).Error

	var simpleTopics []SimpleTopic
	for _, t := range topics {
		simpleTopics = append(simpleTopics, SimpleTopic{ID: t.ID, Name: t.Name})
	}
	return simpleTopics, err
}

func processNewsAgainstTopics(news []SimpleNews, topics []SimpleTopic, db *gorm.DB, isFullCheck bool) {
	for _, n := range news {
		newsWithTopics, err := getTopicsForNews(n, topics)
		if err != nil {
			logger.Error("ID=%d: ошибка запроса к HuggingFace: %v", n.ID, err)
			continue
		}

		filteredTopicIDs := filterTopics(newsWithTopics)
		now := time.Now()

		if len(filteredTopicIDs) == 0 {
			logger.Info("[SKIP] ID=%d: подходящих тем не найдено", n.ID)
			if isFullCheck {
				db.Model(&models.News{}).Where("id = ?", n.ID).Update("without_topics", true)
			}
			db.Model(&models.News{}).Where("id = ?", n.ID).Update("topics_checked_at", now)
			continue
		}

		var topicModels []*models.Topic
		if err := db.Where("id IN ?", filteredTopicIDs).Find(&topicModels).Error; err != nil {
			logger.Error("ID=%d: ошибка загрузки тем из БД: %v", n.ID, err)
			continue
		}

		var newsModel models.News
		if err := db.First(&newsModel, n.ID).Error; err != nil {
			logger.Error("ID=%d: новость не найдена в БД: %v", n.ID, err)
			continue
		}

		if err := db.Model(&newsModel).Association("Topics").Append(topicModels); err != nil {
			logger.Error("ID=%d: ошибка привязки тем: %v", n.ID, err)
			continue
		}

		if newsModel.WithoutTopics {
			db.Model(&models.News{}).Where("id = ?", n.ID).Update("without_topics", false)
		}

		db.Model(&models.News{}).Where("id = ?", n.ID).Update("topics_checked_at", now)

		names := make([]string, 0, len(topicModels))
		for _, t := range topicModels {
			names = append(names, t.Name)
		}
		logger.Info("[OK] ID=%d: %v", n.ID, names)
	}
}

func getTopicsForNews(news SimpleNews, topics []SimpleTopic) (NewsWithTopics, error) {
	labels := make([]string, 0, len(topics))
	for _, topic := range topics {
		labels = append(labels, topic.Name)
	}

	payload := RequestBody{
		Inputs: news.Body,
		Parameters: RequestParameters{
			CandidateLabels:    labels,
			HypothesisTemplate: "Эта новость относится к теме: {}.",
			MultiLabel:         true,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return NewsWithTopics{}, fmt.Errorf("marshal: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return NewsWithTopics{}, fmt.Errorf("new request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+ apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return NewsWithTopics{}, fmt.Errorf("http: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return NewsWithTopics{}, fmt.Errorf("read body: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return NewsWithTopics{}, fmt.Errorf("HuggingFace API status %d: %s", response.StatusCode, string(responseBody))
	}

	var modelResponse ModelResponse
	if err := json.Unmarshal(responseBody, &modelResponse); err != nil {
		return NewsWithTopics{}, fmt.Errorf("unmarshal: %w", err)
	}

	var newsWithTopics NewsWithTopics
	for i, label := range modelResponse.Labels {
		topicID := getIdByTopicName(topics, label)
		if topicID != 0 {
			newsWithTopics.Topics = append(newsWithTopics.Topics, TopicWithScore{
				TopicID: topicID,
				Score:   modelResponse.Scores[i],
			})
		}
	}

	return newsWithTopics, nil
}

func filterTopics(newsWithTopics NewsWithTopics) []uint {
	var filtered []uint
	for _, topic := range newsWithTopics.Topics {
		if topic.Score > ScoreThreshold {
			filtered = append(filtered, topic.TopicID)
		}
	}
	return filtered
}

func CronTopicsTask(db *gorm.DB) {
	logger.Section("Категоризация")
	now := time.Now()

	newNews, err := getUnprocessedNews(db)
	if err != nil {
		logger.Error("Ошибка получения необработанных новостей: %v", err)
		return
	}

	var newTopics []SimpleTopic
	if lastProcessedTime != nil {
		newTopics, err = getNewTopics(db, *lastProcessedTime)
		if err != nil {
			logger.Error("Ошибка получения новых тем: %v", err)
			return
		}
	}

	hasNewNews := len(newNews) > 0
	hasNewTopics := len(newTopics) > 0

	if !hasNewNews && !hasNewTopics {
		logger.Info("Нет новых новостей и тем, пропускаем")
		lastProcessedTime = &now
		logger.Done("Категоризация")
		return
	}

	// Новые новости — прогнать по всем темам
	if hasNewNews {
		allTopics, err := getAllTopics(db)
		if err != nil {
			logger.Error("Ошибка получения всех тем: %v", err)
			return
		}

		if len(allTopics) == 0 {
			logger.Warn("Тем нет в БД, помечаем %d новостей как проверенные", len(newNews))
			t := time.Now()
			for _, n := range newNews {
				db.Model(&models.News{}).Where("id = ?", n.ID).
					Updates(map[string]interface{}{"topics_checked_at": t, "without_topics": true})
			}
		} else {
			logger.Info("Новых новостей: %d, тем: %d", len(newNews), len(allTopics))
			processNewsAgainstTopics(newNews, allTopics, db, true)
		}
	}

	// Новые темы — перепроверить старые новости
	if hasNewTopics {
		oldNews, err := getProcessedNews(db)
		if err != nil {
			logger.Error("Ошибка получения старых новостей: %v", err)
			lastProcessedTime = &now
			return
		}
		logger.Info("Перепроверка %d старых новостей по %d новым темам", len(oldNews), len(newTopics))
		processNewsAgainstTopics(oldNews, newTopics, db, false)
	}

	lastProcessedTime = &now
	logger.Done("Категоризация")
}
