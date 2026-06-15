package handler

import (
	"net/http"
	"smartFlow/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

// NewsGroup — одна группа в ленте: основная новость + её дубликаты
type NewsGroup struct {
	Main           NewsItem   `json:"main"`
	Duplicates     []NewsItem `json:"duplicates"`
	DuplicateCount int        `json:"duplicate_count"`
}

// NewsItem — краткое представление новости для ленты
type NewsItem struct {
	ID          uint       `json:"id"`
	Body        string     `json:"body"`
	MessageLink string     `json:"message_link"`
	CreatedAt   time.Time  `json:"created_at"`
	Channel     ChannelRef `json:"channel"`
	Topics      []TopicRef `json:"topics"`
}

// ChannelRef — канал в формате для ленты
type ChannelRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Link string `json:"link"`
}

// TopicRef — тема в формате для ленты
type TopicRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// newsToItem конвертирует модель News в краткий NewsItem
func newsToItem(n models.News) NewsItem {
	item := NewsItem{
		ID:          n.ID,
		Body:        n.Body,
		MessageLink: n.MessageLink,
		CreatedAt:   n.CreatedAt,
		Channel: ChannelRef{
			ID:   n.Channel.ID,
			Name: n.Channel.Name,
			Link: n.Channel.Link,
		},
	}
	for _, t := range n.Topics {
		item.Topics = append(item.Topics, TopicRef{ID: t.ID, Name: t.Name})
	}
	return item
}

// groupByDuplicates группирует новости: одна главная + дубликаты рядом.
// Новости, которые уже попали как дубликат, не становятся отдельной группой.
func groupByDuplicates(newsList []models.News) []NewsGroup {
	// Индекс для быстрого доступа по ID
	newsMap := make(map[uint]models.News, len(newsList))
	for _, n := range newsList {
		newsMap[n.ID] = n
	}

	// Трекер: какие ID уже попали в какую-то группу как дубликат
	used := make(map[uint]bool)

	var groups []NewsGroup

	for _, n := range newsList {
		if used[n.ID] {
			continue
		}

		// Собираем все новости группы: текущую + её дубликаты
		candidates := []models.News{n}
		used[n.ID] = true

		for _, dup := range n.Duplicates {
			if used[dup.ID] {
				continue
			}
			if dupNews, ok := newsMap[dup.ID]; ok {
				candidates = append(candidates, dupNews)
				used[dup.ID] = true
			}
		}

		// Главная — самая старая (earliest CreatedAt)
		oldest := 0
		for i, c := range candidates {
			if c.CreatedAt.Before(candidates[oldest].CreatedAt) {
				oldest = i
			}
		}

		group := NewsGroup{
			Main: newsToItem(candidates[oldest]),
		}
		for i, c := range candidates {
			if i != oldest {
				group.Duplicates = append(group.Duplicates, newsToItem(c))
			}
		}

		group.DuplicateCount = len(group.Duplicates)
		groups = append(groups, group)
	}

	return groups
}

// @Summary Get user news feed
// @Description Returns news feed grouped by duplicates. Excludes stop-themed news. Optional filter by topic_id.
// @Tags user-news
// @Produce json
// @Param topic_id query string false "Filter by topic ID"
// @Success 200 {array} NewsGroup
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/news [get]
func (h *Handler) getUserNews(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.Preload("Channels").Preload("StopThemes").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	channelIDs := make([]uint, len(user.Channels))
	for i, ch := range user.Channels {
		channelIDs[i] = ch.ID
	}

	if len(channelIDs) == 0 {
		c.JSON(http.StatusOK, []NewsGroup{})
		return
	}

	stopThemeIDs := make([]uint, len(user.StopThemes))
	for i, st := range user.StopThemes {
		stopThemeIDs[i] = st.ID
	}

	query := h.DB.Preload("Topics").Preload("Channel").Preload("Duplicates").
		Where("channel_id IN ?", channelIDs)

	// Исключаем новости со стоп-темами пользователя
	if len(stopThemeIDs) > 0 {
		query = query.Where("news.id NOT IN (?)",
			h.DB.Table("news_stop_themes").
				Select("news_id").
				Where("stop_theme_id IN ?", stopThemeIDs),
		)
	}

	// Фильтр по категории
	if topicID := c.Query("topic_id"); topicID != "" {
		query = query.Joins("JOIN news_topics ON news_topics.news_id = news.id").
			Where("news_topics.topic_id = ?", topicID)
	}

	var news []models.News
	if err := query.Order("created_at DESC").Find(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	groups := groupByDuplicates(news)
	c.JSON(http.StatusOK, groups)
}

// @Summary Get user's blocked news
// @Description Returns news from user's channels blocked by a specific stop-theme, grouped by duplicates.
// @Tags user-news
// @Produce json
// @Param stopThemeId path string true "Stop Theme ID"
// @Success 200 {array} NewsGroup
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/blocked-news/{stopThemeId} [get]
func (h *Handler) getUserBlockedNews(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	var user models.User
	if err := h.DB.Preload("Channels").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var stopTheme models.StopTheme
	if err := h.DB.First(&stopTheme, c.Param("stopThemeId")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stop theme not found"})
		return
	}

	channelIDs := make([]uint, len(user.Channels))
	for i, ch := range user.Channels {
		channelIDs[i] = ch.ID
	}

	if len(channelIDs) == 0 {
		c.JSON(http.StatusOK, []NewsGroup{})
		return
	}

	var news []models.News
	err := h.DB.Preload("Topics").Preload("Channel").Preload("Duplicates").
		Joins("JOIN news_stop_themes ON news_stop_themes.news_id = news.id").
		Where("news_stop_themes.stop_theme_id = ?", stopTheme.ID).
		Where("channel_id IN ?", channelIDs).
		Order("created_at DESC").
		Find(&news).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	groups := groupByDuplicates(news)
	c.JSON(http.StatusOK, groups)
}
