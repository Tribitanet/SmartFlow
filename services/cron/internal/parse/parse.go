package parse

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"smartFlow/internal/models"
	"strings"

	"github.com/SlyMarbo/rss/v2"
	"gorm.io/gorm"
)

func convertTelegramToRSSHub(originalLink string) string {
	link := strings.TrimSpace(originalLink)
	link = strings.TrimPrefix(link, "https://")
	link = strings.TrimPrefix(link, "http://")
	link = strings.TrimPrefix(link, "t.me/")
	link = strings.TrimPrefix(link, "@")
	link = strings.TrimPrefix(link, "s/")

	if link == "" {
		return ""
	}
	return fmt.Sprintf("http://localhost:1200/telegram/channel/%s", link)
}

func ParseTelegramChannels(db *gorm.DB) {
	var allChannels []models.Channel

	if err := db.Find(&allChannels).Error; err != nil {
		log.Printf("Ошибка получения каналов из БД: %v", err)
		return
	}

	for _, channel := range allChannels {
		if !strings.Contains(channel.Link, "t.me") && !strings.HasPrefix(channel.Link, "@") {
			continue
		}

		rssHubURL := convertTelegramToRSSHub(channel.Link)
		if rssHubURL == "" {
			continue
		}

		rssParsing(db, channel, rssHubURL)
	}
}

func rssParsing(db *gorm.DB, channel models.Channel, targetURL string) {
	resp, err := http.Get(targetURL)
	if err != nil {
		log.Printf("Ошибка подключения к локальному RSSHub (%s): %v.", targetURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Локальный RSSHub (%s) вернул ошибку: %d", targetURL, resp.StatusCode)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	reader, err := rss.NewReader()
	if err != nil {
		return
	}

	feed, err := reader.Parse(targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return
	}

	for i, item := range feed.Items {
		if i >= 5 {
			break
		}

		var link string
		if len(item.Links) > 0 && item.Links[0] != nil {
			link = item.Links[0].Href
		}
		if link == "" {
			link = item.ID
		}

		newsItem := models.News{
			Body:        item.Title,
			MessageLink: link,
			ChannelID:   channel.ID,
		}

		// Если новости нет — GORM запишет её. Если есть — просто пропустит.
		db.Where(models.News{MessageLink: newsItem.MessageLink}).FirstOrCreate(&newsItem)
	}
}
