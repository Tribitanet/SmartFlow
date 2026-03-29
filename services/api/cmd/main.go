package main

import (
	"flag"
	"fmt"
	"os"
	database "smartFlow/services/api/internal/database"
	models "smartFlow/internal/models"
	"smartFlow/services/api/internal/handler"
	"smartFlow/services/api/internal/server"
)

func main() {
	isWork := flag.Bool("check_db", false, "check db")
	flag.Parse()

	if *isWork {
		fmt.Println("Cheking DB")
		err := database.CheckDB()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("DB is OK")
		os.Exit(0)
	}

	fmt.Println("Start service")

	db, err := database.Init(database.GetDSN())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = db.AutoMigrate(&models.User{}, &models.News{}, &models.Topic{}, &models.StopTheme{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	handlers := &handler.Handler{
		DB: db,
	}
	srv := new(server.Server)
	if err := srv.Run("8080", handlers.InitRoutes()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
