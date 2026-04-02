package vectordb

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type collectionExistsResponse struct {
	Result struct {
		Exists bool `json:"exists"`
	} `json:"result"`
	Status string  `json:"status"`
	Time   float64 `json:"time"`
}

func CollectionExists(collectionName string) (bool, error) {
	req := fmt.Sprintf("http://localhost:6333/collections/%s/exists", collectionName)
	resp, err := http.Get(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var collectionExistsResponse collectionExistsResponse
	err = json.NewDecoder(resp.Body).Decode(&collectionExistsResponse)
	if err != nil {
		return false, err
	}

	return collectionExistsResponse.Result.Exists, nil
}

