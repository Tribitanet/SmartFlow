package handler

import (
	"net/http"
	"smartFlow/DB"
	"smartFlow/Models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) createUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.GetDB().Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *Handler) getUser(c *gin.Context) {

}

func (h *Handler) deleteUser(c *gin.Context) {
}
