package main

import (
	"log"
	database "smartFlow/internal/database"
	models "smartFlow/internal/models"
	handler "smartFlow/services/backend/internal/handler"
	server "smartFlow/services/backend/internal/server"
)

// @title SmartFlow REST API
// @version 1.0
// @description API Server for SmartFlow application
// @host localhost:8080
// @BasePath /
func main() {
	err := database.CheckDB()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.Init(database.GetDSN())
	if err != nil {
		log.Fatal(err)
	}

	err = db.AutoMigrate(&models.User{}, &models.News{}, &models.Topic{}, &models.StopTheme{}, &models.UserReadNews{}, &models.UserSavedNews{})
	if err != nil {
		log.Fatal(err)
	}

	handlers := &handler.Handler{
		DB: db,
	}

	srv := new(server.Server)

	err = srv.Run("8080", handlers.InitRoutes())
	if err != nil {
		log.Fatal(err)
	}

}
