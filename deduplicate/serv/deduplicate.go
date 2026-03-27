package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	embedding "smartFlow/deduplicate/pkg"

	"github.com/qdrant/go-client/qdrant"
)

func main() {
	config := qdrant.Config{
		Host: "localhost",
		Port: 6334,
	}

	client, err := qdrant.NewClient(&config)
	if err != nil {
		log.Fatal("Qdrant:", err)
	}

	ctx := context.Background()

	collection := qdrant.CreateCollection{
		CollectionName: "news",
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     1024,
			Distance: qdrant.Distance_Cosine,
		}),
	}

	_ = client.CreateCollection(ctx, &collection)

	scanner := bufio.NewScanner(os.Stdin)
	var id uint64 = 1
	for {
		fmt.Println("Вставьте текст новости:")
		fmt.Print("> ")

		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		if input == "exit" {
			fmt.Print("goodbye")
			break
		}
		if input == "" {
			continue
		}
		if input == "clear" {
			fmt.Print("\033[H\033[2J")
		}

		embedding, err := embedding.GetEmbedding(input)
		if err != nil {
			log.Fatal("Embedding: ", err)
		}
		fmt.Println("Embedding: ", embedding)

		point := &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(id),
			Vectors: qdrant.NewVectors(embedding...),
		}

		client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "news",
			Points:         []*qdrant.PointStruct{point},
		})
		id++
	}
}
