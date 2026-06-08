package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"smartFlow/internal/database"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/deduplicate/deduplicate"
	"smartFlow/services/cron/internal/deduplicate/vectordb"
	"smartFlow/services/cron/internal/parse"
	"smartFlow/services/cron/internal/stopthemes"
	"smartFlow/services/cron/internal/topics"

	"github.com/go-co-op/gocron/v2"
	"github.com/qdrant/go-client/qdrant"
)

func main() {

	qdrantHost := os.Getenv("QDRANT_HOST")
	if qdrantHost == "" {
		qdrantHost = "localhost"
	}

	//поднимаем qdrant
	config := qdrant.Config{
		Host: qdrantHost,
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

	//Cron
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal("Scheduler creation error:", err)
	}
	defer scheduler.Shutdown()

	// Единый последовательный воркфлоу обработки новостей
	_, err = scheduler.NewJob(
		gocron.DurationJob(10*time.Second),
		gocron.NewTask(func() {
			parse.ParseTelegramChannels(db)

			deduplicate.CronDeduplicateTask(db, client, ctx)

			topics.CronTopicsTask(db)

			stopthemes.CronStopThemesTask(db)

		}),
	)
	if err != nil {
		log.Fatal("Job creation error:", err)
	}
	scheduler.Start()

	select {}
}
