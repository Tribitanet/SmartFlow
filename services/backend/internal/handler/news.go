package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// @Summary Create news
// @Description Adds a new news entry to the database
// @Tags news
// @Accept json
// @Produce json
// @Param news body models.News true "News Data"
// @Success 200 {object} models.News
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /news [post]
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

// @Summary Delete news
// @Description Deletes a specific news item by ID
// @Tags news
// @Produce json
// @Param id path string true "News ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /news/{id} [delete]
func (h *Handler) deleteNews(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	if err := h.DB.Delete(&models.News{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "News deleted successfully"})
}


// @Summary Delete all news
// @Description Cleans junction tables then safely removes all news
// @Tags news
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /news [delete]
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
