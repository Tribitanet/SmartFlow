package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"smartFlow/internal/database"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/deduplicate/embedding"
	"smartFlow/services/cron/internal/deduplicate/vectordb"

	"github.com/qdrant/go-client/qdrant"
)

func main() {

	//поднимаем qdrant
	config := qdrant.Config{
		Host: "localhost",
		Port: 6334,
	}

	client, err := qdrant.NewClient(&config)
	if err != nil {
		log.Fatal("Qdrant:", err)
	}

	ctx := context.Background()

	exist, err := vectordb.CollectionExists("news")
	if err != nil {
		log.Fatal(err)
	}

	if exist {
		fmt.Println("Коллекция уже существует")
	} else {
		collection := qdrant.CreateCollection{
			CollectionName: "news",
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     1024,
				Distance: qdrant.Distance_Cosine,
			}),
		}

		err = client.CreateCollection(ctx, &collection)
		if err != nil {
			log.Fatal("Qdrant:", err)
		}
	}

	//на всякий поднимаем БД
	err = database.CheckDB()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	db, err := database.Init(database.GetDSN())
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	err = db.AutoMigrate(&models.User{}, &models.News{}, &models.Topic{}, &models.StopTheme{})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

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
