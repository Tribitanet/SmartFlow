package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"smartFlow/internal/database"
	"smartFlow/internal/models"

	"github.com/joho/godotenv"
)

const (
	modelXLM     = "https://router.huggingface.co/hf-inference/models/joeddav/xlm-roberta-large-xnli"
	modelDeBERTa = "https://router.huggingface.co/hf-inference/models/MoritzLaurer/mDeBERTa-v3-base-mnli-xnli"
	threshold    = 0.8
)

type RequestBody struct {
	Inputs     string            `json:"inputs"`
	Parameters RequestParameters `json:"parameters"`
}

type RequestParameters struct {
	CandidateLabels    []string `json:"candidate_labels"`
	HypothesisTemplate string   `json:"hypothesis_template"`
	MultiLabel         bool     `json:"multi_label"`
}

// xlm-roberta: {"labels": [...], "scores": [...]}
type XLMResponse struct {
	Labels []string  `json:"labels"`
	Scores []float64 `json:"scores"`
}

// mDeBERTa: [{"label": "...", "score": ...}]
type DeBERTaItem struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type LabelScore struct {
	Label string
	Score float64
}

var apiKey string

func classify(modelURL, text string, labels []string) ([]LabelScore, error) {
	payload := RequestBody{
		Inputs: text,
		Parameters: RequestParameters{
			CandidateLabels:    labels,
			HypothesisTemplate: "Эта новость относится к теме: {}.",
			MultiLabel:         true,
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, modelURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API status %d: %s", resp.StatusCode, string(respBody))
	}

	var result []LabelScore

	if len(respBody) > 0 && respBody[0] == '[' {
		// mDeBERTa формат
		var arr []DeBERTaItem
		if err := json.Unmarshal(respBody, &arr); err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
		for _, item := range arr {
			result = append(result, LabelScore{item.Label, item.Score})
		}
	} else {
		// xlm-roberta формат
		var r XLMResponse
		if err := json.Unmarshal(respBody, &r); err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
		for i := range r.Labels {
			result = append(result, LabelScore{r.Labels[i], r.Scores[i]})
		}
	}

	// Сортируем по убыванию
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result, nil
}

func scoreByLabel(scores []LabelScore, label string) float64 {
	for _, s := range scores {
		if s.Label == label {
			return s.Score
		}
	}
	return 0
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func mark(score float64) string {
	if score >= threshold {
		return "✓"
	}
	return "—"
}

func main() {
	// Загружаем .env
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	apiKey = os.Getenv("HF_TOPICS_TOKEN")
	if apiKey == "" {
		log.Fatal("HF_TOPICS_TOKEN не задан в .env")
	}

	db, err := database.Init(database.GetDSN())
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	var newsList []models.News
	if err := db.Find(&newsList).Error; err != nil {
		log.Fatalf("Ошибка получения новостей: %v", err)
	}

	var topics []models.Topic
	if err := db.Find(&topics).Error; err != nil {
		log.Fatalf("Ошибка получения категорий: %v", err)
	}
	if len(topics) == 0 {
		log.Fatal("В БД нет категорий")
	}

	labels := make([]string, 0, len(topics))
	for _, t := range topics {
		labels = append(labels, t.Name)
	}

	fmt.Printf("Новостей: %d | Категорий: %s\n", len(newsList), strings.Join(labels, ", "))
	fmt.Printf("Порог: %.1f\n\n", threshold)

	for _, news := range newsList {
		fmt.Printf("━━━ Новость ID=%d ━━━\n", news.ID)
		fmt.Printf("%s\n\n", truncate(news.Body, 120))

		fmt.Printf("  Запрос к xlm-roberta... ")
		xlmScores, err := classify(modelXLM, news.Body, labels)
		if err != nil {
			fmt.Printf("ОШИБКА: %v\n", err)
			xlmScores = nil
		} else {
			fmt.Println("OK")
		}

		fmt.Printf("  Запрос к mDeBERTa...    ")
		debertaScores, err := classify(modelDeBERTa, news.Body, labels)
		if err != nil {
			fmt.Printf("ОШИБКА: %v\n", err)
			debertaScores = nil
		} else {
			fmt.Println("OK")
		}

		// Таблица сравнения
		fmt.Println()
		fmt.Println("  ┌─────────────────────────┬──────────────────────┬──────────────────────┐")
		fmt.Println("  │ Категория               │   xlm-roberta-large  │   mDeBERTa-v3-base   │")
		fmt.Println("  ├─────────────────────────┼──────────────────────┼──────────────────────┤")
		for _, label := range labels {
			xlmScore := scoreByLabel(xlmScores, label)
			debScore := scoreByLabel(debertaScores, label)
			fmt.Printf("  │ %-23s │   %.4f  %s             │   %.4f  %s             │\n",
				label, xlmScore, mark(xlmScore), debScore, mark(debScore))
		}
		fmt.Println("  └─────────────────────────┴──────────────────────┴──────────────────────┘")
		fmt.Println()
	}
}
