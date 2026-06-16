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

	// Раздача статических файлов (CSS, JS)
	staticPath := filepath.Join(basePath, "..", "..", "web", "static")
	router.Static("/static", staticPath)

	// Страницы фронтенда
	router.GET("/", h.feedPage)
	router.GET("/feed", h.feedPage)
	router.GET("/automate", h.automatePage)
	router.GET("/search", h.searchPage)
	router.GET("/addfeed", h.addfeedPage)
	router.GET("/login", h.authPage)

	// Публичные маршруты 
	auth := router.Group("/auth")
	{
		auth.POST("/register", h.createUser)
		auth.POST("/login", h.loginUser)
	}

	// Защищённые маршруты
	users := router.Group("/users")
	users.Use(AuthMiddleware())
	{
		users.GET("/me", h.getUser)
		users.PUT("/me", h.updateUser)
		users.DELETE("/me", h.deleteUser)

		// Каналы пользователя
		users.GET("/channels", h.getUserChannels)
		users.POST("/channels", h.addUserChannel)
		users.DELETE("/channels/:channelId", h.removeUserChannel)

		// Категории пользователя
		users.GET("/topics", h.getUserTopics)
		users.POST("/topics", h.addUserTopic)
		users.DELETE("/topics/:topicId", h.removeUserTopic)

		// Стоп-темы пользователя
		users.GET("/stop-themes", h.getUserStopThemes)
		users.POST("/stop-themes", h.addUserStopTheme)
		users.DELETE("/stop-themes/:stopThemeId", h.removeUserStopTheme)

		// Лента новостей пользователя
		users.GET("/news", h.getUserNews)
		users.GET("/blocked-news/:stopThemeId", h.getUserBlockedNews)

		// Прочитанные новости
		users.GET("/news/read", h.getReadNews)
		users.POST("/news/:newsId/read", h.markNewsRead)
		users.DELETE("/news/:newsId/read", h.unmarkNewsRead)

		// Сохранённые новости
		users.GET("/news/saved", h.getSavedNews)
		users.POST("/news/:newsId/save", h.saveNews)
		users.DELETE("/news/:newsId/save", h.unsaveNews)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
