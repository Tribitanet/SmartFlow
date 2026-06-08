package parse

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"smartFlow/internal/models"
	"strings"

	"github.com/SlyMarbo/rss/v2"
	"github.com/forPelevin/gomoji"
	"github.com/microcosm-cc/bluemonday"
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

	title := CleanChannelName(feed.Title)
	re_title, err := CreateChannelRegex(title)
	if err != nil {
		fmt.Println("Ошибка:", err)
		return
	}
	fmt.Printf("Название %s", title)
	for i, item := range feed.Items {
		if i >= 5 {
			break
		}

		var link string
		link = item.ID
		cleanBody := CleanTextWithGomoji(CleanHTML(item.Summary), re_title)
		newsItem := models.News{
			Body:        cleanBody,
			MessageLink: link,
			ChannelID:   channel.ID,
		}
		db.Where(models.News{MessageLink: newsItem.MessageLink}).FirstOrCreate(&newsItem)
	}
}

func CleanTextWithGomoji(input string, channelRe *regexp.Regexp) string {
	if input == "" {
		return ""
	}

	text := gomoji.RemoveEmojis(input)
	reAllIcons := regexp.MustCompile(`[\p{So}\p{Pi}\p{Pf}]`)
	text = reAllIcons.ReplaceAllString(text, "")

	if channelRe != nil {
		text = channelRe.ReplaceAllString(text, "")
	}

	reUsernames := regexp.MustCompile(`(^|\s|[,.!?;:])@([A-Za-z0-9_]+)`)
	text = reUsernames.ReplaceAllString(text, "$1")

	reSubscribe := regexp.MustCompile(`(?i)подписаться[\s!.,:-]*$`)
	text = reSubscribe.ReplaceAllString(text, "")

	reSpaces := regexp.MustCompile(`\s+`)
	text = reSpaces.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func CleanHTML(htmlContent string) string {
	p := bluemonday.StrictPolicy()

	plainText := p.Sanitize(htmlContent)

	return html.UnescapeString(plainText)
}

func CleanChannelName(name string) string {
	name = strings.TrimSpace(name)

	name = strings.TrimSuffix(name, " - Telegram Channel")
	name = strings.TrimSuffix(name, " - Telegram channel")
	name = strings.TrimSuffix(name, " - Telegram CHANNEL")

	replacer := strings.NewReplacer(".", " ", "-", " ", "_", " ")
	name = replacer.Replace(name)

	reSpaces := regexp.MustCompile(`\s+`)
	name = reSpaces.ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

func CreateChannelRegex(channelName string) (*regexp.Regexp, error) {
	trimmed := strings.TrimSpace(channelName)
	if trimmed == "" {
		return nil, fmt.Errorf("название канала пустое")
	}

	words := strings.Fields(trimmed)

	for i, word := range words {
		words[i] = regexp.QuoteMeta(word)
	}

	pattern := strings.Join(words, `[\s\S]*?`)

	finalPattern := "(?i)" + pattern + "."

	return regexp.Compile(finalPattern)
}
