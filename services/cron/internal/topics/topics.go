package topics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"smartFlow/internal/models"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

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

func getNewsWithoutTopics(db *gorm.DB) ([]SimpleNews, error) {
	var news []models.News
	err := db.Where("NOT EXISTS (SELECT 1 FROM news_topics WHERE news_topics.news_id = news.id)").Find(&news).Error
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

	fmt.Println("Raw API response:", string(responseBody))

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
		if topic.Score > 0.7 {
			filteredTopics = append(filteredTopics, topic.TopicID)
		}
	}
	return filteredTopics
}

func CronTopicsTask(db *gorm.DB) {
	news, err := getNewsWithoutTopics(db)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Новости получены")

	topics, err := getAllTopics(db)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Темы получены")

	for _, n := range news {
		newsWithTopics := getTopicsForNews(n, topics)
		filteredTopicIDs := filterTopics(newsWithTopics)

		if len(filteredTopicIDs) == 0 {
			log.Printf("Новость ID=%d: подходящих тем не найдено", n.ID)
			continue
		}

		var topicModels []*models.Topic
		if err := db.Where("id IN ?", filteredTopicIDs).Find(&topicModels).Error; err != nil {
			log.Printf("Ошибка загрузки тем для новости ID=%d: %v", n.ID, err)
			continue
		}

		var newsModel models.News
		if err := db.First(&newsModel, n.ID).Error; err != nil {
			log.Printf("Новость ID=%d не найдена в БД: %v", n.ID, err)
			continue
		}

		if err := db.Model(&newsModel).Association("Topics").Append(topicModels); err != nil {
			log.Printf("Ошибка привязки тем к новости ID=%d: %v", n.ID, err)
			continue
		}

		log.Printf("Новость ID=%d: привязано %d тем", n.ID, len(topicModels))
	}
	log.Println("Темы обновлены")
}