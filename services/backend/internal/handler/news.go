package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get all news
// @Description Returns news with preloaded Topics, Channel, Duplicates. Optional filter by topic_id.
// @Tags news
// @Produce json
// @Param topic_id query string false "Filter by topic ID"
// @Success 200 {array} models.News
// @Failure 500 {object} map[string]string
// @Router /news [get]
func (h *Handler) getAllNews(c *gin.Context) {
	query := h.DB.Preload("Topics").Preload("Channel").Preload("Duplicates").Preload("StopThemes")

	if topicID := c.Query("topic_id"); topicID != "" {
		query = query.Joins("JOIN news_topics ON news_topics.news_id = news.id").
			Where("news_topics.topic_id = ?", topicID)
	}

	var news []models.News
	if err := query.Order("created_at DESC").Find(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, news)
}
