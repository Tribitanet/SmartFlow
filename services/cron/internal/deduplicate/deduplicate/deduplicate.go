package deduplicate

import (
	"context"
	"smartFlow/internal/models"
	"smartFlow/services/cron/internal/deduplicate/embedding"
	"smartFlow/services/cron/internal/logger"
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
		emb, err := embedding.GetEmbedding(n.Body)
		if err != nil {
			return nil, err
		}

		point := &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(uint64(n.ID)),
			Vectors: qdrant.NewVectors(emb...),
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

// addDuplicate возвращает true если связь была записана (новая), false если уже существовала
func addDuplicate(db *gorm.DB, a, b uint) (bool, error) {
	var added bool
	err := db.Transaction(func(tx *gorm.DB) error {
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

		added = true
		return nil
	})
	return added, err
}

func CronDeduplicateTask(db *gorm.DB, client *qdrant.Client, ctx context.Context) {
	logger.Section("Дедупликация")

	remainingNews, err := getRemainingNews(db)
	if err != nil {
		logger.Error("Ошибка получения новостей: %v", err)
		return
	}

	if len(remainingNews) == 0 {
		logger.Info("Нет новых новостей для обработки")
		logger.Done("Дедупликация")
		return
	}

	logger.Info("Новостей к обработке: %d", len(remainingNews))

	points, err := getQdrantPoints(remainingNews)
	if err != nil {
		logger.Error("Ошибка получения эмбеддингов: %v", err)
		return
	}

	logger.Info("Эмбеддинги получены: %d", len(points))

	for _, point := range points {
		_, err = client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "news",
			Points:         []*qdrant.PointStruct{point},
		})
		if err != nil {
			logger.Error("Ошибка загрузки в Qdrant (ID=%d): %v", point.Id.GetNum(), err)
			continue
		}

		response, err := client.GetPointsClient().Recommend(ctx, &qdrant.RecommendPoints{
			CollectionName: "news",
			Positive:       []*qdrant.PointId{point.Id},
			Limit:          100,
			ScoreThreshold: qdrant.PtrOf(float32(0.79)),
		})
		if err != nil {
			logger.Error("Ошибка поиска дубликатов (ID=%d): %v", point.Id.GetNum(), err)
			continue
		}

		for _, simPoint := range response.GetResult() {
			if simPoint.Id.GetNum() == point.Id.GetNum() {
				continue
			}
			added, err := addDuplicate(db, uint(simPoint.Id.GetNum()), uint(point.Id.GetNum()))
			if err != nil {
				logger.Error("Ошибка записи дубликата: %v", err)
			} else if added {
				logger.Info("Дубликат: ID=%d <-> ID=%d (score=%.2f)", point.Id.GetNum(), simPoint.Id.GetNum(), simPoint.Score)
			}
		}

		now := time.Now()
		db.Model(&models.News{}).Where("id = ?", point.Id.GetNum()).Update("deduplication_checked_at", now)
	}

	logger.Done("Дедупликация")
}
