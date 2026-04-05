package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	_ "smartFlow/services/backend/docs"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/files"
)

type Handler struct {
	DB *gorm.DB
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	users := router.Group("/users")
	{
		users.POST("", h.createUser)
		users.GET("/:id", h.getUser)
		users.DELETE("/:id", h.deleteUser)
		users.PUT("/:id", h.updateUser)
	}

	news := router.Group("/news")
	{
		news.POST("", h.createNews)
		news.DELETE("/:id", h.deleteNews)
		news.DELETE("", h.deleteAllNews)
	}

	channels := router.Group("/channels")
	{
		channels.POST("", h.createChannel)
	}

	fields := router.Group("/fields")
	{
		fields.GET("/stop-themes", h.getStopThemes)
		fields.GET("/topics", h.getTopics)
		fields.GET("/channels", h.getChannels)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
