package embedding

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

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
		log.Fatal("Не удалось найти .env файл")
	}

	apiKey = os.Getenv("JINA_API_KEY")
	if apiKey == "" {
		log.Fatal("JINA_API_KEY not set in .env")
	}
}

func GetEmbedding(text string) ([]float32, error) {
	requestBody, err := json.Marshal(jinaRequest{
		Model: "jina-embeddings-v3",
		Input: []string{text},
	})
	if err != nil {
		log.Fatal("Marshal error:", err)
		return nil, err
	}

	request, err := http.NewRequest("POST", "https://api.jina.ai/v1/embeddings", bytes.NewReader(requestBody))
	if err != nil {
		log.Fatal("Request error:", err)
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal("HTTP error:", err)
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal("Read body error:", err)
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		log.Fatalf("Jina API error (status %d): %s", response.StatusCode, string(body))
		return nil, err
	}

	var jinaResp jinaResponse
	if err := json.Unmarshal(body, &jinaResp); err != nil {
		log.Fatal("Unmarshal error:", err)
		return nil, err
	}

	if len(jinaResp.Data) == 0 {
		log.Fatal("Empty response from Jina API")
		return nil, err
	}

	return jinaResp.Data[0].Embedding, nil
}
