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
	apiURL    = "https://router.huggingface.co/hf-inference/models/joeddav/xlm-roberta-large-xnli"
	threshold = 0.8
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

type ModelResponse struct {
	Labels []string  `json:"labels"`
	Scores []float64 `json:"scores"`
}

type LabelScore struct {
	Label string
	Score float64
}

var apiKey string

func classify(text string, labels []string) ([]LabelScore, error) {
	payload := RequestBody{
		Inputs: text,
		Parameters: RequestParameters{
			CandidateLabels:    labels,
			HypothesisTemplate: "В этой новости речь идёт о: {}.",
			MultiLabel:         true,
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
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

	var r ModelResponse
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	result := make([]LabelScore, len(r.Labels))
	for i := range r.Labels {
		result[i] = LabelScore{r.Labels[i], r.Scores[i]}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	return result, nil
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
		return "✓ БЛОК"
	}
	return "—"
}

func main() {
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

	var stopThemes []models.StopTheme
	if err := db.Find(&stopThemes).Error; err != nil {
		log.Fatalf("Ошибка получения стоп-тем: %v", err)
	}
	if len(stopThemes) == 0 {
		log.Fatal("В БД нет стоп-тем")
	}

	labels := make([]string, 0, len(stopThemes))
	for _, st := range stopThemes {
		labels = append(labels, st.Name)
	}

	fmt.Printf("Новостей: %d | Стоп-темы: %s\n", len(newsList), strings.Join(labels, ", "))
	fmt.Printf("Порог: %.1f\n\n", threshold)

	for _, news := range newsList {
		fmt.Printf("━━━ Новость ID=%d ━━━\n", news.ID)
		fmt.Printf("%s\n\n", truncate(news.Body, 120))

		fmt.Printf("  Запрос к модели... ")
		scores, err := classify(news.Body, labels)
		if err != nil {
			fmt.Printf("ОШИБКА: %v\n\n", err)
			continue
		}
		fmt.Println("OK")

		fmt.Println()
		fmt.Println("  ┌─────────────────────────┬────────┬──────────┐")
		fmt.Println("  │ Стоп-тема               │  Скор  │ Результат│")
		fmt.Println("  ├─────────────────────────┼────────┼──────────┤")
		for _, s := range scores {
			fmt.Printf("  │ %-23s │ %.4f │ %-8s │\n", s.Label, s.Score, mark(s.Score))
		}
		fmt.Println("  └─────────────────────────┴────────┴──────────┘")
		fmt.Println()
	}
}
