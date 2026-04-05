package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

// @Summary Create a channel
// @Description Creates a new channel in the database
// @Tags channels
// @Accept json
// @Produce json
// @Param channel body models.Channel true "Channel Data"
// @Success 200 {object} models.Channel
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /channels [post]
func (h *Handler) createChannel(c *gin.Context) {
	var channel models.Channel

	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.DB.Create(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}