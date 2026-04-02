package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

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