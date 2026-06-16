package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get user channels
// @Description Returns all channels the authenticated user is subscribed to
// @Tags user-channels
// @Produce json
// @Success 200 {array} models.Channel
// @Failure 404 {object} map[string]string
// @Router /users/channels [get]
func (h *Handler) getUserChannels(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var channels []*models.Channel
	if err := h.DB.Model(&user).Association("Channels").Find(&channels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channels)
}

// @Summary Add channel to user
// @Description Subscribes the authenticated user to a channel. Creates the channel if it doesn't exist.
// @Tags user-channels
// @Accept json
// @Produce json
// @Param channel body models.Channel true "Channel data (Link and Name)"
// @Success 200 {object} models.Channel
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/channels [post]
func (h *Handler) addUserChannel(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var input models.Channel
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Ищем канал по ссылке, нету - создаём
	var channel models.Channel
	result := h.DB.Where("link = ?", input.Link).First(&channel)
	if result.Error != nil {
		channel = models.Channel{Link: input.Link, Name: input.Name}
		if err := h.DB.Create(&channel).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if err := h.DB.Model(&user).Association("Channels").Append(&channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}

// @Summary Remove channel from user
// @Description Unsubscribes the authenticated user from a channel. Deletes the channel if no other users are subscribed.
// @Tags user-channels
// @Produce json
// @Param channelId path string true "Channel ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/channels/{channelId} [delete]
func (h *Handler) removeUserChannel(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var channel models.Channel
	if err := h.DB.First(&channel, c.Param("channelId")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if err := h.DB.Model(&user).Association("Channels").Delete(&channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Уддвляем если больше ни один пользователь не подписан
	usersCount := h.DB.Model(&channel).Association("Users").Count()
	if usersCount == 0 {
		if err := h.DB.Delete(&channel).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Channel removed from user and deleted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Channel removed from user"})
}
