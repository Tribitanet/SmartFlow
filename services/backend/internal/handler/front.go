package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) feedPage(c *gin.Context) {
	c.HTML(http.StatusOK, "feed.html", nil)
}

func (h *Handler) automatePage(c *gin.Context) {
	c.HTML(http.StatusOK, "automate.html", nil)
}

func (h *Handler) searchPage(c *gin.Context) {
	c.HTML(http.StatusOK, "search.html", nil)
}

func (h *Handler) addfeedPage(c *gin.Context) {
	c.HTML(http.StatusOK, "addfeed.html", nil)
}

func (h *Handler) authPage(c *gin.Context) {
	c.HTML(http.StatusOK, "auth.html", nil)
}
