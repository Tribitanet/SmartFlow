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
		if i >= 5 {
			break
		}

		var link string
		link = item.ID

		rawHTML := item.Summary // Исходный HTML текст новости

		// 1. Удаляем отметку о пересылке
		reForwardHTML := regexp.MustCompile(`(?i)<p>\s*Forwarded\s+From\s*<b>(<a[^>]*>.*?</a>|.*?)</b>\s*</p>\s*`)
		rawHTML = reForwardHTML.ReplaceAllString(rawHTML, "")

		// 2. Удаляем блок цитаты <blockquote> в самом конце
		reBlockquote := regexp.MustCompile(`(?i)<blockquote>[\s\S]*?</blockquote>\s*`)
		rawHTML = reBlockquote.ReplaceAllString(rawHTML, "")

		// 3. Удаляем абсолютно все теги ссылок вместе с их текстом
		reAllLinks := regexp.MustCompile(`(?i)<a[^>]*>[\s\S]*?</a>`)
		rawHTML = reAllLinks.ReplaceAllString(rawHTML, "")

		// 4. Очищаем от слов-призывов, которые стояли рядом со ссылками
		reGarbageWords := regexp.MustCompile(`(?i)(Открыть|Зеркало|Сайт|Ссылка|Источник|Перейти|Подписаться|телеграм|читать)`)
		rawHTML = reGarbageWords.ReplaceAllString(rawHTML, "")

		// Удаляем оставшиеся знаки разделители (| , ↓ , / , -)
		reSeparators := regexp.MustCompile(`[\s|/\\↓—–-⏳]+`)
		rawHTML = reSeparators.ReplaceAllString(rawHTML, " ")

		// Заменяем закрывающие теги абзацев </p> и переносы <br> на символ \n
		reLines := regexp.MustCompile(`(?i)</p>|<br\s*/?>`)
		rawHTML = reLines.ReplaceAllString(rawHTML, "\n")

		// 5. Очищаем текст от стандартных HTML-тегов через библиотеку
		cleanBody := CleanHTML(rawHTML)

		// 6. ЖЕСТКАЯ ЗАЧИСТКА ХВОСТОВ:
		// Удаляем любые сломанные теги с пробелами внутри (например, < b>, <  p>, </ b>),
		// которые библиотека bluemonday могла пропустить, посчитав обычным текстом
		reBrokenTags := regexp.MustCompile(`</?\s*[a-zA-Z0-9_-]+\s*>`)
		cleanBody = reBrokenTags.ReplaceAllString(cleanBody, "")

		// 7. Передаем в функцию финальной очистки (эмодзи, двойные пробелы, имя канала)
		cleanBody = CleanTextWithGomoji(cleanBody, re_title)

		newsItem := models.News{
			Body:        cleanBody,
			MessageLink: link,
			ChannelID:   channel.ID,
		}

		// Если новости нет — GORM запишет её. Если есть — просто пропустит.
		db.Where(models.News{MessageLink: newsItem.MessageLink}).FirstOrCreate(&newsItem)
	}
}

func CleanTextWithGomoji(input string, channelRe *regexp.Regexp) string {
	if input == "" {
		return ""
	}

	// 1. Удаляем эмодзи
	text := gomoji.RemoveEmojis(input)

	// 2. Удаляем декоративные спецсимволы и маркеры (\p{So}, \p{Pi}, \p{Pf})
	reAllIcons := regexp.MustCompile(`[\p{So}\p{Pi}\p{Pf}]`)
	text = reAllIcons.ReplaceAllString(text, "")

	// 3. Удаляем упоминания самого канала
	if channelRe != nil {
		text = channelRe.ReplaceAllString(text, "")
	}

	// 4. Удаляем оставшиеся текстовые @username
	reUsernames := regexp.MustCompile(`(^|\s|[,.!?;:])@([A-Za-z0-9_]+)`)
	text = reUsernames.ReplaceAllString(text, "")

	// 5. Удаляем слово "подписаться", если оно осталось без ссылки в конце
	reSubscribe := regexp.MustCompile(`(?i)подписаться[\s!.,:-]*$`)
	text = reSubscribe.ReplaceAllString(text, "")

	// 6. Схлопываем двойные пробелы
	reSpaces := regexp.MustCompile(`\s+`)
	text = reSpaces.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func CleanHTML(htmlContent string) string {
	// 1. Создаем строгую политику (она удаляет абсолютно все теги)
	p := bluemonday.StrictPolicy()

	// 2. Очищаем текст от HTML-тегов
	plainText := p.Sanitize(htmlContent)

	// 3. Декодируем HTML-сущности (например, &amp; превратит в &, а &nbsp; в пробел)
	return html.UnescapeString(plainText)
}

func CleanChannelName(name string) string {
	// 1. Очищаем от пробелов по краям
	name = strings.TrimSpace(name)

	// 2. Удаляем варианты приписки Telegram в конце
	name = strings.TrimSuffix(name, " - Telegram Channel")
	name = strings.TrimSuffix(name, " - Telegram channel")
	name = strings.TrimSuffix(name, " - Telegram CHANNEL")

	// 3. Заменяем точки, тире и нижние подчеркивания на пробелы
	// Создаем Replacer для быстрой массовой замены символов
	replacer := strings.NewReplacer(".", " ", "-", " ", "_", " ")
	name = replacer.Replace(name)

	// 4. Удаляем множественные пробелы, заменяя их на один
	reSpaces := regexp.MustCompile(`\s+`)
	name = reSpaces.ReplaceAllString(name, " ")

	// 5. Финальная очистка краев строки
	return strings.TrimSpace(name)
}

func CreateChannelRegex(channelName string) (*regexp.Regexp, error) {
	// 1. Очищаем имя от лишних пробелов по краям
	trimmed := strings.TrimSpace(channelName)
	if trimmed == "" {
		return nil, fmt.Errorf("название канала пустое")
	}

	// 2. Разбиваем название на отдельные слова по пробелам
	words := strings.Fields(trimmed)

	// 3. Экранируем каждое слово на случай, если там есть спецсимволы регулярных выражений (например, + или $)
	for i, word := range words {
		words[i] = regexp.QuoteMeta(word)
	}

	// 4. Соединяем слова через шаблон "любые символы" -> [\s\S]*?
	// [\s\S] находит абсолютно любой символ, включая переносы строк \n
	// Знак вопроса *? делает поиск ленивым, чтобы регулярка не захватывала лишний текст между совпадениями
	pattern := strings.Join(words, `[\s\S]*?`)

	// 5. Добавляем флаг (?i) для игнорирования регистра букв
	finalPattern := "(?i)" + pattern + "."

	// 6. Компилируем регулярное выражение
	return regexp.Compile(finalPattern)
}
