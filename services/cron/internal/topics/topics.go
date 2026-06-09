package topics

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
		log.Fatal("Не удалось найти .env файл")
	}

	apiKey = os.Getenv("HF_TOPICS_TOKEN")
	if apiKey == "" {
		log.Fatal("HF_TOPICS_TOKEN not set in .env")
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
		simpleNews = append(simpleNews, SimpleNews{
			ID:   n.ID,
			Body: n.Body,
		})
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
		simpleNews = append(simpleNews, SimpleNews{
			ID:   n.ID,
			Body: n.Body,
		})
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
		simpleTopics = append(simpleTopics, SimpleTopic{
			ID:   t.ID,
			Name: t.Name,
		})
	}
	return simpleTopics, nil
}

func getAllTopics(db *gorm.DB) ([]SimpleTopic, error) {
	var topics []models.Topic
	err := db.Find(&topics).Error

	var simpleTopics []SimpleTopic
	for _, t := range topics {
		simpleTopics = append(simpleTopics, SimpleTopic{
			ID:   t.ID,
			Name: t.Name,
		})
	}
	return simpleTopics, err
}

func processNewsAgainstTopics(news []SimpleNews, topics []SimpleTopic, db *gorm.DB, isFullCheck bool) {
	for _, n := range news {
		newsWithTopics := getTopicsForNews(n, topics)
		filteredTopicIDs := filterTopics(newsWithTopics)

		now := time.Now()

		if len(filteredTopicIDs) == 0 {
			log.Printf("Новость ID=%d: подходящих тем не найдено", n.ID)

			// WithoutTopics ставим только при полной проверке (по всем темам)
			if isFullCheck {
				db.Model(&models.News{}).Where("id = ?", n.ID).Update("without_topics", true)
			}

			db.Model(&models.News{}).Where("id = ?", n.ID).Update("topics_checked_at", now)
			continue
		}


		var topicModels []*models.Topic
		err := db.Where("id IN ?", filteredTopicIDs).Find(&topicModels).Error
		if err != nil {
			log.Printf("Ошибка загрузки тем для новости ID=%d: %v", n.ID, err)
			continue
		}

		var newsModel models.News
		err = db.First(&newsModel, n.ID).Error
		if err != nil {
			log.Printf("Новость ID=%d не найдена в БД: %v", n.ID, err)
			continue
		}

		err = db.Model(&newsModel).Association("Topics").Append(topicModels)
		if err != nil {
			log.Printf("Ошибка привязки тем к новости ID=%d: %v", n.ID, err)
			continue
		}

		// Если раньше стоял WithoutTopics — сбрасываем, т.к. теперь темы нашлись
		if newsModel.WithoutTopics {
			db.Model(&models.News{}).Where("id = ?", n.ID).Update("without_topics", false)
		}

		db.Model(&models.News{}).Where("id = ?", n.ID).Update("topics_checked_at", now)
		log.Printf("Новость ID=%d: привязано %d тем", n.ID, len(topicModels))
	}
}

func getTopicsForNews(news SimpleNews, topics []SimpleTopic) NewsWithTopics {
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

	return newsWithTopics
}

func filterTopics(newsWithTopics NewsWithTopics) []uint {
	var filteredTopics []uint
	for _, topic := range newsWithTopics.Topics {
		if topic.Score > ScoreThreshold {
			filteredTopics = append(filteredTopics, topic.TopicID)
		}
	}
	return filteredTopics
}

func CronTopicsTask(db *gorm.DB) {
	now := time.Now()

	// 1. Новые новости (TopicsCheckedAt IS NULL)
	newNews, err := getUnprocessedNews(db)
	if err != nil {
		log.Printf("Ошибка получения необработанных новостей: %v", err)
		return
	}

	// 2. Новые темы (CreatedAt > lastProcessedTime). При первом запуске — пусто
	var newTopics []SimpleTopic
	if lastProcessedTime != nil {
		newTopics, err = getNewTopics(db, *lastProcessedTime)
		if err != nil {
			log.Printf("Ошибка получения новых тем: %v", err)
			return
		}
	}

	hasNewNews := len(newNews) > 0
	hasNewTopics := len(newTopics) > 0

	if !hasNewNews && !hasNewTopics {
		log.Println("Нет новых новостей и тем, пропускаем")
		lastProcessedTime = &now
		return
	}

	// Сценарий 1 + 3: Новые новости - прогнать по ВСЕМ темам
	if hasNewNews {
		allTopics, err := getAllTopics(db)
		if err != nil {
			log.Printf("Ошибка получения всех тем: %v", err)
			return
		}

		if len(allTopics) == 0 {
			log.Println("Тем нет в БД, помечаем новости как проверенные без тем")
			now := time.Now()
			for _, n := range newNews {
				db.Model(&models.News{}).Where("id = ?", n.ID).
					Updates(map[string]interface{}{"topics_checked_at": now, "without_topics": true})
			}
		} else {
			log.Printf("Обработка %d новых новостей по %d темам", len(newNews), len(allTopics))
			processNewsAgainstTopics(newNews, allTopics, db, true)
		}
	}

	// Сценарий 2 + 3: Новые темы - прогнать СТАРЫЕ новости по новым темам
	if hasNewTopics {
		oldNews, err := getProcessedNews(db)
		if err != nil {
			log.Printf("Ошибка получения старых новостей: %v", err)
			lastProcessedTime = &now
			return
		}
		log.Printf("Перепроверка %d старых новостей по %d новым темам", len(oldNews), len(newTopics))
		processNewsAgainstTopics(oldNews, newTopics, db, false)
	}

	lastProcessedTime = &now
	log.Println("CronTopicsTask завершён")
}
