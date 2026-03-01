package handler

import "github.com/gin-gonic/gin"

type Handler struct {

}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()
	
	users := router.Group("/users")
	{
		users.POST("/create-user", h.createUser)
		users.GET("/", h.getUser)
		users.DELETE("/", h.deleteUser)
	}

	return router
}