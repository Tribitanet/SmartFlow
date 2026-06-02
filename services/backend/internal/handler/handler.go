package handler

import (
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	_ "smartFlow/services/backend/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handler struct {
	DB *gorm.DB
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	// Загрузка HTML-шаблонов
	_, b, _, _ := runtime.Caller(0)
	basePath := filepath.Dir(b)
	templatesPath := filepath.Join(basePath, "..", "..", "web", "pages", "*")
	router.LoadHTMLGlob(templatesPath)

	// Главная страница (фронтенд)
	router.GET("/", h.indexPage)

	// === Публичные маршруты (без авторизации) ===
	auth := router.Group("/auth")
	{
		auth.POST("/register", h.createUser)
		auth.POST("/login", h.loginUser)
	}

	// === Защищённые маршруты (сюда позже добавится auth middleware) ===
	// users.Use(AuthMiddleware())
	users := router.Group("/users")
	{
		users.GET("/:id", h.getUser)
		users.PUT("/:id", h.updateUser)
		users.DELETE("/:id", h.deleteUser)

		// Каналы пользователя
		users.GET("/:id/channels", h.getUserChannels)
		users.POST("/:id/channels", h.addUserChannel)
		users.DELETE("/:id/channels/:channelId", h.removeUserChannel)

		// Категории пользователя
		users.GET("/:id/topics", h.getUserTopics)
		users.POST("/:id/topics", h.addUserTopic)
		users.DELETE("/:id/topics/:topicId", h.removeUserTopic)

		// Стоп-темы пользователя
		users.GET("/:id/stop-themes", h.getUserStopThemes)
		users.POST("/:id/stop-themes", h.addUserStopTheme)
		users.DELETE("/:id/stop-themes/:stopThemeId", h.removeUserStopTheme)

		// Лента новостей пользователя
		users.GET("/:id/news", h.getUserNews)
		users.GET("/:id/blocked-news/:stopThemeId", h.getUserBlockedNews)
	}

	news := router.Group("/news")
	{
		news.GET("", h.getAllNews)
		news.POST("", h.createNews)
		news.DELETE("/:id", h.deleteNews)
		news.DELETE("", h.deleteAllNews)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
