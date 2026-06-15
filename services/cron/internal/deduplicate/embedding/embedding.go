package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"smartFlow/services/cron/internal/logger"

	"github.com/joho/godotenv"
)

type jinaRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type jinaResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

var apiKey string

func init() {
	envPath := []string{
		"../../../.env",
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

	apiKey = os.Getenv("JINA_API_KEY")
	if apiKey == "" {
		logger.Error("JINA_API_KEY не задан в .env")
		os.Exit(1)
	}
}

func GetEmbedding(text string) ([]float32, error) {
	requestBody, err := json.Marshal(jinaRequest{
		Model: "jina-embeddings-v3",
		Input: []string{text},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	request, err := http.NewRequest("POST", "https://api.jina.ai/v1/embeddings", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jina API status %d: %s", response.StatusCode, string(body))
	}

	var jinaResp jinaResponse
	if err := json.Unmarshal(body, &jinaResp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if len(jinaResp.Data) == 0 {
		return nil, fmt.Errorf("пустой ответ от Jina API")
	}

	return jinaResp.Data[0].Embedding, nil
}
