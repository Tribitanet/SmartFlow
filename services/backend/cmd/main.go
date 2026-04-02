package main

import (
	"log"
	"os"
	database "smartFlow/internal/database"
	models "smartFlow/internal/models"
	handler "smartFlow/services/backend/internal/handler"
	server "smartFlow/services/backend/internal/server"
)

func main() {
	err := database.CheckDB()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	db, err := database.Init(database.GetDSN())
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	err = db.AutoMigrate(&models.User{}, &models.News{}, &models.Topic{}, &models.StopTheme{})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	handlers := &handler.Handler{
		DB: db,
	}

	srv := new(server.Server)

	err = srv.Run("8080", handlers.InitRoutes())
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

}
