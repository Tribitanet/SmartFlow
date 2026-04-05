package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get all topics
// @Description Fetch a list of all defined topics
// @Tags fields
// @Produce json
// @Success 200 {array} models.Topic
// @Failure 500 {object} map[string]string
// @Router /fields/topics [get]
func (h *Handler) getTopics(c *gin.Context) {
	var topics []models.Topic

	if err := h.DB.Find(&topics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, topics)
}

// @Summary Get all stop themes
// @Description Fetch a list of all defined stop themes
// @Tags fields
// @Produce json
// @Success 200 {array} models.StopTheme
// @Failure 500 {object} map[string]string
// @Router /fields/stop-themes [get]
func (h *Handler) getStopThemes(c *gin.Context) {
	var stopThemes []models.StopTheme

	if err := h.DB.Find(&stopThemes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stopThemes)
}

// @Summary Get all channels
// @Description Fetch a list of all stored channels
// @Tags fields
// @Produce json
// @Success 200 {array} models.Channel
// @Failure 500 {object} map[string]string
// @Router /fields/channels [get]
func (h *Handler) getChannels(c *gin.Context) {
	var channels []models.Channel

	if err := h.DB.Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channels)
}
