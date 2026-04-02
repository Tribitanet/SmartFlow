package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
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
