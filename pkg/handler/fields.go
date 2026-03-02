package handler

import (
	"net/http"
	"smartFlow/Models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) getTopics(c *gin.Context) {
	var topics []models.Topic

	if err := h.DB.Find(&topics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, topics)
}

func (h *Handler) getStopThemes(c *gin.Context) {
	var stopThemes []models.StopTheme

	if err := h.DB.Find(&stopThemes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stopThemes)
}

func (h *Handler) getChannels(c *gin.Context) {
	var channels []models.Channel

	if err := h.DB.Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channels)
}
