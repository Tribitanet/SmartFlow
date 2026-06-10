package deduplicate

import (
	"context"
	"log"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/deduplicate/embedding"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"gorm.io/gorm"
)

type SimpleNews struct {
	ID   uint
	Body string
}

func getRemainingNews(db *gorm.DB) ([]SimpleNews, error) {
	var news []models.News
	err := db.Where("deduplication_checked_at IS NULL").Find(&news).Error
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

func duplicateExists(tx *gorm.DB, newsID, duplicateID uint) bool {
	var count int64
	tx.Table("news_duplicates").
		Where("news_id = ? AND duplicate_id = ?", newsID, duplicateID).
		Count(&count)
	return count > 0
}

func addDuplicate(db *gorm.DB, a, b uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if duplicateExists(tx, a, b) {
			return nil
		}

		var newsA, newsB models.News

		if err := tx.First(&newsA, a).Error; err != nil {
			return err
		}

		if err := tx.First(&newsB, b).Error; err != nil {
			return err
		}

		if err := tx.Model(&newsA).Association("Duplicates").Append(&newsB); err != nil {
			return err
		}

		if err := tx.Model(&newsB).Association("Duplicates").Append(&newsA); err != nil {
			return err
		}

		return nil
	})
}


func CronDeduplicateTask(db *gorm.DB, client *qdrant.Client, ctx context.Context) {
	remainingNews, err := getRemainingNews(db)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Новостей получено: %d", len(remainingNews))

	points, err := getQdrantPoints(remainingNews)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Эмбеддинги получены: %d", len(points))

	
	for _, point := range points {
		_, err = client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "news",
			Points:         []*qdrant.PointStruct{point},
		})
		if err != nil {
			log.Fatal(err)
		}

		response, err := client.GetPointsClient().Recommend(ctx, &qdrant.RecommendPoints{
			CollectionName: "news",
			Positive: []*qdrant.PointId{point.Id},
			Limit: 100,
			ScoreThreshold: qdrant.PtrOf(float32(0.79)),
		})
		if err != nil {
			log.Fatal(err)
		}

		similarPoints := response.GetResult()
		for _, simPoint := range similarPoints {
			log.Printf("Найден дубликат! ID: %d <-> %d, Скор схожести: %f", point.Id.GetNum(), simPoint.Id.GetNum(), simPoint.Score)
			addDuplicate(db, uint(simPoint.Id.GetNum()), uint(point.Id.GetNum()))
		}

		now := time.Now()
		db.Model(&models.News{}).Where("id = ?", point.Id.GetNum()).Update("deduplication_checked_at", now)
	}
	log.Println("Дубликаты обновлены")
}