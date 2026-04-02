package deduplicate

import (
	"context"
	"log"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/deduplicate/embedding"

	"github.com/qdrant/go-client/qdrant"
	"gorm.io/gorm"
)

type SimpleNews struct {
	ID   uint
	Body string
}

func getRemainingNews(db *gorm.DB) ([]SimpleNews, error) {
	var news []models.News
	err := db.Joins("LEFT JOIN news_duplicates ON news.id = news_duplicates.news_id").Where("news_duplicates.news_id IS NULL").Find(&news).Error
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

func getQdrantPoints(news []SimpleNews) ([]*qdrant.PointStruct, error) {
	var points []*qdrant.PointStruct
	for _, n := range news {
		embedding, err := embedding.GetEmbedding(n.Body)
		if err != nil {
			return nil, err
		}

		point := &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(uint64(n.ID)),
			Vectors: qdrant.NewVectors(embedding...),
		}

		points = append(points, point)
	}

	return points, nil
}

func AddDuplicate(db *gorm.DB, a, b uint) error {
	var newsA, newsB models.News

	err := db.First(&newsA, a).Error
	if err != nil {
		return err
	}

	err = db.First(&newsB, b).Error
	if err != nil {
		return err
	}

	err = db.Model(&newsA).Association("Duplicates").Append(&newsB)
	if err != nil {
		return err
	}

	err = db.Model(&newsB).Association("Duplicates").Append(&newsA)
	if err != nil {
		return err
	}

	return nil
}


func CronDeduplicateTask(db *gorm.DB, client *qdrant.Client, ctx context.Context) {
	remainingNews, err := getRemainingNews(db)
	if err != nil {
		log.Fatal(err)
	}

	points, err := getQdrantPoints(remainingNews)
	if err != nil {
		log.Fatal(err)
	}

	for _, point := range points {
		client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "news",
			Points:         []*qdrant.PointStruct{point},
		})

		response, err := client.GetPointsClient().Recommend(ctx, &qdrant.RecommendPoints{
			CollectionName: "news",
			Positive: []*qdrant.PointId{point.Id},
			Limit: 100,
			ScoreThreshold: qdrant.PtrOf(float32(0.85)),
		})
		if err != nil {
			log.Fatal(err)
		}

		similarPoints := response.GetResult()
		for _, simPoint := range similarPoints {
			AddDuplicate(db, uint(simPoint.Id.GetNum()), uint(point.Id.GetNum()))
		}
	}
}