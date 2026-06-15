package parse

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"smartFlow/internal/models"
	"strings"
	"smartFlow/services/cron/internal/logger"

	"github.com/SlyMarbo/rss/v2"
	"github.com/forPelevin/gomoji"
	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)

func getRSSHubHost() string {
	host := os.Getenv("RSSHUB_HOST")
	if host == "" {
		host = "localhost"
	}
	return host
}

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
	return fmt.Sprintf("http://%s:1200/telegram/channel/%s", getRSSHubHost(), link)
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
	for i, item := range feed.Items {
		if i >= 10 {
			break
		}

		reSystemMessage := regexp.MustCompile(`(?i)Channel\s+(created|updated|photo|description)`)
		if reSystemMessage.MatchString(item.Title) {
			logger.Info("[%s] Пропущено системное сообщение: %s", channel.Name, item.Title)
			continue
		}
		var link string
		link = item.ID

		rawHTML := item.Summary

		//Удаляем отметку о пересылке
		reForwardHTML := regexp.MustCompile(`(?i)<p>\s*Forwarded\s+From\s*<b>(<a[^>]*>.*?</a>|.*?)</b>\s*</p>\s*`)
		rawHTML = reForwardHTML.ReplaceAllString(rawHTML, "")

		//Удаляем блок цитаты в самом конце
		reBlockquote := regexp.MustCompile(`(?i)<blockquote>[\s\S]*?</blockquote>\s*`)
		rawHTML = reBlockquote.ReplaceAllString(rawHTML, "")

		//Удалние гиперссылок
		reAllLinks := regexp.MustCompile(`(?i)<a[^>]*>[\s\S]*?</a>`)
		rawHTML = reAllLinks.ReplaceAllString(rawHTML, "")

		reGarbageWords := regexp.MustCompile(`(?i)(Открыть|Зеркало|Сайт|Ссылка|Источник|Перейти|Подписаться|телеграм|читать)`)
		rawHTML = reGarbageWords.ReplaceAllString(rawHTML, "")

		// Удаляем оставшиеся знаки разделители
		reSeparators := regexp.MustCompile(`[\s|/\\↓—–-⏳]+`)
		rawHTML = reSeparators.ReplaceAllString(rawHTML, " ")

		//Оставляем переносы
		reLines := regexp.MustCompile(`(?i)</p>|<br\s*/?>`)
		rawHTML = reLines.ReplaceAllString(rawHTML, "\n")

		//Очищаем текст от стандартных HTML-тегов через библиотеку
		cleanBody := CleanHTML(rawHTML)

		//Удаление оставшихся html тегов
		reBrokenTags := regexp.MustCompile(`</?\s*[a-zA-Z0-9_-]+\s*>`)
		cleanBody = reBrokenTags.ReplaceAllString(cleanBody, "")

		//Передаем в функцию очистки
		cleanBody = CleanTextWithGomoji(cleanBody, re_title)

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

	//Удаляем эмодзи
	text := gomoji.RemoveEmojis(input)

	reAllIcons := regexp.MustCompile(`[\p{So}\p{Pi}\p{Pf}]`)
	text = reAllIcons.ReplaceAllString(text, "")

	//Удаляем упоминания самого канала
	if channelRe != nil {
		text = channelRe.ReplaceAllString(text, "")
	}

	//Удаляем оставшиеся текстовые @username
	reUsernames := regexp.MustCompile(`(^|\s|[,.!?;:])@([A-Za-z0-9_]+)`)
	text = reUsernames.ReplaceAllString(text, "")

	//Удаляем слово "подписаться", если оно осталось без ссылки в конце
	reSubscribe := regexp.MustCompile(`(?i)подписаться[\s!.,:-]*$`)
	text = reSubscribe.ReplaceAllString(text, "")

	//Схлопываем двойные пробелы
	reSpaces := regexp.MustCompile(`\s+`)
	text = reSpaces.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}
func CleanHTML(htmlContent string) string {
	p := bluemonday.StrictPolicy()

	//Очищаем текст от HTML-тегов
	plainText := p.Sanitize(htmlContent)

	return html.UnescapeString(plainText)
}

func CleanChannelName(name string) string {
	//Очищаем от пробелов по краям
	name = strings.TrimSpace(name)

	name = strings.TrimSuffix(name, " - Telegram Channel")
	name = strings.TrimSuffix(name, " - Telegram channel")
	name = strings.TrimSuffix(name, " - Telegram CHANNEL")

	replacer := strings.NewReplacer(".", " ", "-", " ", "_", " ")
	name = replacer.Replace(name)

	//Удаляем несколько пробелов, заменяя их на один
	reSpaces := regexp.MustCompile(`\s+`)
	name = reSpaces.ReplaceAllString(name, " ")

	//Очистка краев строки
	return strings.TrimSpace(name)
}
func CreateChannelRegex(channelName string) (*regexp.Regexp, error) {
	trimmed := strings.TrimSpace(channelName)
	if trimmed == "" {
		return nil, fmt.Errorf("название канала пустое")
	}

	//Разбиваем название на отдельные слова по пробелам
	words := strings.Fields(trimmed)

	//Экранируем каждое слово на случай, если там есть спецсимволы регулярных выражений (например, + или $)
	for i, word := range words {
		words[i] = regexp.QuoteMeta(word)
	}

	pattern := strings.Join(words, `[\s\S]*?`)

	finalPattern := "(?i)" + pattern + "."

	return regexp.Compile(finalPattern)
}
