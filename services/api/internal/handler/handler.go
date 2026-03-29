package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	DB *gorm.DB
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	users := router.Group("/users")
	{
		users.POST("/create-user", h.createUser)
		users.GET("/get-user", h.getUser)
		users.DELETE("/delete-user", h.deleteUser)
		users.PUT("/update-user", h.updateUser)
	}

	news := router.Group("/news")
	{
		news.POST("/create-news", h.createNews)
	}

	fields := router.Group("/fields")
	{
		fields.GET("/stop-themes", h.getStopThemes)
		fields.GET("/topics", h.getTopics)
		fields.GET("/channels", h.getChannels)
	}

	return router
}
