package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) indexPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}
