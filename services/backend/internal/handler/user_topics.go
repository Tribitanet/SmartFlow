package handler

import (
	"net/http"
	"smartFlow/internal/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get user topics
// @Description Returns all topics (categories) the user is subscribed to
// @Tags user-topics
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {array} models.Topic
// @Failure 404 {object} map[string]string
// @Router /users/{id}/topics [get]
func (h *Handler) getUserTopics(c *gin.Context) {
	var user models.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var topics []*models.Topic
	if err := h.DB.Model(&user).Association("Topics").Find(&topics); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, topics)
}

// @Summary Add topic to user
// @Description Subscribes the user to a topic. Creates the topic if it doesn't exist.
// @Tags user-topics
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param topic body models.Topic true "Topic data (Name)"
// @Success 200 {object} models.Topic
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/{id}/topics [post]
func (h *Handler) addUserTopic(c *gin.Context) {
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

	// Ищем тему по имени, если не найдена — создаём
	var topic models.Topic
	result := h.DB.Where("name = ?", input.Name).First(&topic)
	if result.Error != nil {
		topic = models.Topic{Name: input.Name}
		if err := h.DB.Create(&topic).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if err := h.DB.Model(&user).Association("Topics").Append(&topic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, topic)
}

// @Summary Remove topic from user
// @Description Unsubscribes the user from a topic. Deletes the topic if no other users are subscribed.
// @Tags user-topics
// @Produce json
// @Param id path string true "User ID"
// @Param topicId path string true "Topic ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/{id}/topics/{topicId} [delete]
func (h *Handler) removeUserTopic(c *gin.Context) {
	var user models.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var topic models.Topic
	if err := h.DB.First(&topic, c.Param("topicId")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Topic not found"})
		return
	}

	if err := h.DB.Model(&user).Association("Topics").Delete(&topic); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Проверяем, есть ли ещё пользователи с этой темой
	var usersCount int64
	h.DB.Table("user_topics").Where("topic_id = ?", topic.ID).Count(&usersCount)
	if usersCount == 0 {
		if err := h.DB.Delete(&topic).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Topic removed from user and deleted"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Topic removed from user"})
}
