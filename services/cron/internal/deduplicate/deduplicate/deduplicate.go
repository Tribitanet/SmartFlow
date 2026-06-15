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

func duplicateExists(tx *gorm.DB, newsID, duplicateID uint) bool {
	var count int64
	tx.Table("news_duplicates").
		Where("news_id = ? AND duplicate_id = ?", newsID, duplicateID).
		Count(&count)
	return count > 0
}

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

	for _, n := range remainingNews {
		// Получаем эмбеддинг — если ошибка, пропускаем (не помечаем как обработанную)
		emb, err := embedding.GetEmbedding(n.Body)
		if err != nil {
			logger.Error("ID=%d: ошибка эмбеддинга: %v", n.ID, err)
			continue
		}

		point := &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(uint64(n.ID)),
			Vectors: qdrant.NewVectors(emb...),
		}

		_, err = client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "news",
			Points:         []*qdrant.PointStruct{point},
		})
		if err != nil {
			logger.Error("ID=%d: ошибка загрузки в Qdrant: %v", n.ID, err)
			continue
		}

		response, err := client.GetPointsClient().Recommend(ctx, &qdrant.RecommendPoints{
			CollectionName: "news",
			Positive:       []*qdrant.PointId{point.Id},
			Limit:          100,
			ScoreThreshold: qdrant.PtrOf(float32(0.79)),
		})
		if err != nil {
			logger.Error("ID=%d: ошибка поиска дубликатов: %v", n.ID, err)
			continue
		}

		for _, simPoint := range response.GetResult() {
			if simPoint.Id.GetNum() == point.Id.GetNum() {
				continue
			}
			added, err := addDuplicate(db, uint(simPoint.Id.GetNum()), uint(point.Id.GetNum()))
			if err != nil {
				logger.Error("Ошибка записи дубликата ID=%d <-> ID=%d: %v", point.Id.GetNum(), simPoint.Id.GetNum(), err)
			} else if added {
				logger.Info("Дубликат: ID=%d <-> ID=%d (score=%.2f)", point.Id.GetNum(), simPoint.Id.GetNum(), simPoint.Score)
			}
		}

		now := time.Now()
		db.Model(&models.News{}).Where("id = ?", n.ID).Update("deduplication_checked_at", now)
	}

	logger.Done("Дедупликация")
}
