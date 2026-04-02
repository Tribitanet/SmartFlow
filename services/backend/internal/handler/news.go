package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) createNews(c *gin.Context) {
	var news models.News

	if err := c.ShouldBindJSON(&news); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Create(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, news)
}

func (h *Handler) deleteNews(c *gin.Context) {
	var news models.News

	if err := c.ShouldBindJSON(&news); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Delete(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "News deleted successfully"})
}

func (h *Handler) deleteAllNews(c *gin.Context) {
	// Очищаем таблицы связей (многие ко многим), чтобы избежать ошибки "violates foreign key constraint"
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM news_duplicates").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM news_topics").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM news_stop_themes").Error; err != nil {
			return err
		}
		// Теперь безопасно удаляем все новости
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.News{}).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All news deleted successfully"})
}
