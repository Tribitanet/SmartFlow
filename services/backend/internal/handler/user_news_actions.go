package handler

import (
	"net/http"
	"smartFlow/internal/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ── READ ──────────────────────────────────────────────────────────────────────

// @Summary Mark news as read
// @Tags user-news
// @Param newsId path int true "News ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Router /users/news/{newsId}/read [post]
func (h *Handler) markNewsRead(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	newsID, err := strconv.ParseUint(c.Param("newsId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid news id"})
		return
	}

	record := models.UserReadNews{UserID: userID, NewsID: uint(newsID)}
	h.DB.Where(record).FirstOrCreate(&record)
	c.Status(http.StatusNoContent)
}

// @Summary Unmark news as read
// @Tags user-news
// @Param newsId path int true "News ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Router /users/news/{newsId}/read [delete]
func (h *Handler) unmarkNewsRead(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	newsID, err := strconv.ParseUint(c.Param("newsId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid news id"})
		return
	}

	h.DB.Where("user_id = ? AND news_id = ?", userID, newsID).Delete(&models.UserReadNews{})
	c.Status(http.StatusNoContent)
}

// @Summary Get read news
// @Description Returns news marked as read by the current user, grouped by duplicates.
// @Tags user-news
// @Produce json
// @Success 200 {array} NewsGroup
// @Failure 500 {object} map[string]string
// @Router /users/news/read [get]
func (h *Handler) getReadNews(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var records []models.UserReadNews
	h.DB.Where("user_id = ?", userID).Find(&records)

	newsIDs := make([]uint, len(records))
	for i, r := range records {
		newsIDs[i] = r.NewsID
	}

	if len(newsIDs) == 0 {
		c.JSON(http.StatusOK, []NewsGroup{})
		return
	}

	var news []models.News
	if err := h.DB.Preload("Topics").Preload("Channel").Preload("Duplicates").
		Where("id IN ?", newsIDs).
		Order("created_at DESC").
		Find(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groupByDuplicates(news))
}

// ── SAVED ─────────────────────────────────────────────────────────────────────

// @Summary Save news
// @Tags user-news
// @Param newsId path int true "News ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Router /users/news/{newsId}/save [post]
func (h *Handler) saveNews(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	newsID, err := strconv.ParseUint(c.Param("newsId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid news id"})
		return
	}

	record := models.UserSavedNews{UserID: userID, NewsID: uint(newsID)}
	h.DB.Where(record).FirstOrCreate(&record)
	c.Status(http.StatusNoContent)
}

// @Summary Unsave news
// @Tags user-news
// @Param newsId path int true "News ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Router /users/news/{newsId}/save [delete]
func (h *Handler) unsaveNews(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	newsID, err := strconv.ParseUint(c.Param("newsId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid news id"})
		return
	}

	h.DB.Where("user_id = ? AND news_id = ?", userID, newsID).Delete(&models.UserSavedNews{})
	c.Status(http.StatusNoContent)
}

// @Summary Get saved news
// @Description Returns news saved by the current user, grouped by duplicates.
// @Tags user-news
// @Produce json
// @Success 200 {array} NewsGroup
// @Failure 500 {object} map[string]string
// @Router /users/news/saved [get]
func (h *Handler) getSavedNews(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var records []models.UserSavedNews
	h.DB.Where("user_id = ?", userID).Find(&records)

	newsIDs := make([]uint, len(records))
	for i, r := range records {
		newsIDs[i] = r.NewsID
	}

	if len(newsIDs) == 0 {
		c.JSON(http.StatusOK, []NewsGroup{})
		return
	}

	var news []models.News
	if err := h.DB.Preload("Topics").Preload("Channel").Preload("Duplicates").
		Where("id IN ?", newsIDs).
		Order("created_at DESC").
		Find(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groupByDuplicates(news))
}
