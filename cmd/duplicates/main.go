package main

import (
	"fmt"
	"log"
	"os"

	"smartFlow/internal/database"
	"smartFlow/internal/models"

	"github.com/joho/godotenv"
)

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func main() {
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	db, err := database.Init(database.GetDSN())
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	// Загружаем все новости с их дубликатами
	var newsList []models.News
	if err := db.Preload("Duplicates").Find(&newsList).Error; err != nil {
		log.Fatalf("Ошибка получения новостей: %v", err)
	}

	// Собираем группы дубликатов (избегаем повторов)
	visited := make(map[uint]bool)
	groups := [][]models.News{}

	for _, news := range newsList {
		if visited[news.ID] {
			continue
		}
		if len(news.Duplicates) == 0 {
			continue
		}

		group := []models.News{news}
		visited[news.ID] = true

		for _, dup := range news.Duplicates {
			if !visited[dup.ID] {
				// Загружаем полную новость (Duplicates не preload во вложенных)
				var fullDup models.News
				db.First(&fullDup, dup.ID)
				group = append(group, fullDup)
				visited[dup.ID] = true
			}
		}

		groups = append(groups, group)
	}

	// Новости без дубликатов
	unique := []models.News{}
	for _, news := range newsList {
		if !visited[news.ID] {
			unique = append(unique, news)
		}
	}

	// Статистика
	fmt.Printf("Всего новостей: %d\n", len(newsList))
	fmt.Printf("Групп дубликатов: %d\n", len(groups))
	fmt.Printf("Уникальных новостей: %d\n\n", len(unique))

	// Группы дубликатов
	for i, group := range groups {
		fmt.Printf("━━━ Группа %d (%d новости) ━━━\n", i+1, len(group))
		fmt.Println()
		fmt.Println("  ┌──────┬───────────────────────────────────────────────────────────────┐")
		fmt.Println("  │  ID  │ Текст                                                         │")
		fmt.Println("  ├──────┼───────────────────────────────────────────────────────────────┤")
		for _, news := range group {
			fmt.Printf("  │ %4d │ %-61s │\n", news.ID, truncate(news.Body, 61))
		}
		fmt.Println("  └──────┴───────────────────────────────────────────────────────────────┘")
		fmt.Println()
	}

	// Уникальные новости
	if len(unique) > 0 {
		fmt.Println("━━━ Уникальные новости (без дубликатов) ━━━")
		fmt.Println()
		fmt.Println("  ┌──────┬───────────────────────────────────────────────────────────────┐")
		fmt.Println("  │  ID  │ Текст                                                         │")
		fmt.Println("  ├──────┼───────────────────────────────────────────────────────────────┤")
		for _, news := range unique {
			fmt.Printf("  │ %4d │ %-61s │\n", news.ID, truncate(news.Body, 61))
		}
		fmt.Println("  └──────┴───────────────────────────────────────────────────────────────┘")

		// Проверяем — все ли уникальные помечены как обработанные
		unchecked := 0
		for _, news := range unique {
			if news.DeduplicationCheckedAt == nil {
				unchecked++
			}
		}
		if unchecked > 0 {
			fmt.Printf("\n  [!] %d новостей ещё не обработаны дедупликатором\n", unchecked)
		}
	}

	os.Exit(0)
}
