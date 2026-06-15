package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get user stop themes
// @Description Returns all stop themes the authenticated user has configured
// @Tags user-stop-themes
// @Produce json
// @Success 200 {array} models.StopTheme
// @Failure 404 {object} map[string]string
// @Router /users/stop-themes [get]
func (h *Handler) getUserStopThemes(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var stopThemes []*models.StopTheme
	if err := h.DB.Model(&user).Association("StopThemes").Find(&stopThemes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stopThemes)
}

// @Summary Add stop theme to user
// @Description Adds a stop theme to the authenticated user's filter list. Creates it if it doesn't exist.
// @Tags user-stop-themes
// @Accept json
// @Produce json
// @Param stopTheme body object true "StopTheme data (name)"
// @Success 200 {object} models.StopTheme
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/stop-themes [post]
func (h *Handler) addUserStopTheme(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var input struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var stopTheme models.StopTheme
	result := h.DB.Where("name = ?", input.Name).First(&stopTheme)
	if result.Error != nil {
		stopTheme = models.StopTheme{Name: input.Name}
		if err := h.DB.Create(&stopTheme).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if err := h.DB.Model(&user).Association("StopThemes").Append(&stopTheme); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stopTheme)
}

// @Summary Remove stop theme from user
// @Description Removes a stop theme from the authenticated user's filter list. The stop theme itself is kept in the database.
// @Tags user-stop-themes
// @Produce json
// @Param stopThemeId path string true "StopTheme ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/stop-themes/{stopThemeId} [delete]
func (h *Handler) removeUserStopTheme(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var stopTheme models.StopTheme
	if err := h.DB.First(&stopTheme, c.Param("stopThemeId")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stop theme not found"})
		return
	}

	// Отвязываем тему только от пользователя; саму тему из БД не удаляем
	if err := h.DB.Model(&user).Association("StopThemes").Delete(&stopTheme); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Stop theme removed from user"})
}
