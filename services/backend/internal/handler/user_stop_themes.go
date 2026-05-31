package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get user stop themes
// @Description Returns all stop themes the user has configured
// @Tags user-stop-themes
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {array} models.StopTheme
// @Failure 404 {object} map[string]string
// @Router /users/{id}/stop-themes [get]
func (h *Handler) getUserStopThemes(c *gin.Context) {
	var user models.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
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
// @Description Adds a stop theme to the user's filter list. Creates the stop theme if it doesn't exist.
// @Tags user-stop-themes
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param stopTheme body models.StopTheme true "StopTheme data (Name)"
// @Success 200 {object} models.StopTheme
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/{id}/stop-themes [post]
func (h *Handler) addUserStopTheme(c *gin.Context) {
	var user models.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
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

	// Ищем стоп-тему по имени, если не найдена — создаём
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
// @Description Removes a stop theme from the user's filter list. Deletes the stop theme if no other users use it.
// @Tags user-stop-themes
// @Produce json
// @Param id path string true "User ID"
// @Param stopThemeId path string true "StopTheme ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/{id}/stop-themes/{stopThemeId} [delete]
func (h *Handler) removeUserStopTheme(c *gin.Context) {
	var user models.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var stopTheme models.StopTheme
	if err := h.DB.First(&stopTheme, c.Param("stopThemeId")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stop theme not found"})
		return
	}

	if err := h.DB.Model(&user).Association("StopThemes").Delete(&stopTheme); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Проверяем, есть ли ещё пользователи с этой стоп-темой
	var usersCount int64
	h.DB.Table("user_stop_themes").Where("stop_theme_id = ?", stopTheme.ID).Count(&usersCount)
	if usersCount == 0 {
		if err := h.DB.Delete(&stopTheme).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Stop theme removed from user and deleted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Stop theme removed from user"})
}
