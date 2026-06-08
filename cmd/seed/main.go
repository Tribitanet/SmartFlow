package main

import (
	"log"
	"smartFlow/internal/database"
	"smartFlow/internal/models"
	"os"
)

func main() {
	db, err := database.Init(database.GetDSN())
	if err != nil {
		log.Fatalf("Не удалось подключиться к БД: %v", err)
	}

	err = db.AutoMigrate(&models.User{}, &models.News{}, &models.Topic{}, &models.StopTheme{})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Создаём тестовые каналы
	channels := []models.Channel{
		{Link: "https://t.me/space_news", Name: "Space News"},
		{Link: "https://t.me/techcrunch_ru", Name: "TechCrunch RU"},
		{Link: "https://t.me/rbc_news", Name: "РБК"},
	}

	for i := range channels {
		if err := db.FirstOrCreate(&channels[i], models.Channel{Link: channels[i].Link}).Error; err != nil {
			log.Fatalf("Ошибка создания канала: %v", err)
		}
		log.Printf("Канал: %s (ID=%d)", channels[i].Name, channels[i].ID)
	}

	// Тестовые новости
	news := []models.News{
		// Группа "Космос" — должны стать дубликатами
		{
			Body:        "Компания SpaceX Илона Маска успешно запустила тяжелую ракету Falcon Heavy, которая вывела на орбиту Земли новую партию интернет-спутников Starlink.",
			MessageLink: "https://t.me/space_news/1",
			ChannelID:   channels[0].ID,
		},
		{
			Body:        "Очередная партия спутников Starlink выведена на околоземную орбиту с помощью ракетоносителя Falcon Heavy от корпорации SpaceX.",
			MessageLink: "https://t.me/techcrunch_ru/2",
			ChannelID:   channels[1].ID,
		},
		{
			Body:        "Тяжелая ракета Falcon Heavy успешно стартовала сегодня утром. SpaceX доставила на орбиту Земли новые телекоммуникационные спутники Starlink Илона Маска.",
			MessageLink: "https://t.me/rbc_news/3",
			ChannelID:   channels[2].ID,
		},
		// Группа "Экономика" — должны стать дубликатами
		{
			Body:        "Центральный Банк принял решение повысить ключевую ставку сразу на 2 процента на фоне растущей инфляции в стране.",
			MessageLink: "https://t.me/rbc_news/4",
			ChannelID:   channels[2].ID,
		},
		{
			Body:        "Из-за ускоряющейся инфляции регулятор (ЦБ) вынужденно увеличил ставку рефинансирования на 200 базисных пунктов (2%).",
			MessageLink: "https://t.me/techcrunch_ru/5",
			ChannelID:   channels[1].ID,
		},
		// Шум — не дубликат
		{
			Body:        "Илон Маск представил новую бюджетную модель электромобиля Tesla Model 2 стоимостью около 25 тысяч долларов.",
			MessageLink: "https://t.me/space_news/6",
			ChannelID:   channels[0].ID,
		},
	}

	for i := range news {
		if err := db.Create(&news[i]).Error; err != nil {
			log.Fatalf("Ошибка создания новости %d: %v", i+1, err)
		}
		log.Printf("Новость %d создана (ID=%d, ChannelID=%d)", i+1, news[i].ID, news[i].ChannelID)
	}

	log.Println("Сидинг завершён. Запусти крон — он обработает новости.")
}
