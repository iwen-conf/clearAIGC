package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/iwen-conf/Naturalize/internal/api/handler"
)

func NewRouter(h *handler.Handler, allowedOrigins []string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), corsMiddleware(allowedOrigins))

	v1 := router.Group("/api/v1")
	{
		v1.POST("/sessions", h.CreateSession)
		v1.POST("/sessions/batch", h.CreateSessionBatch)
		v1.GET("/sessions", h.ListSessions)
		v1.GET("/sessions/:id", h.GetSession)
		v1.GET("/sessions/:id/state", h.GetSessionState)
		v1.GET("/sessions/:id/history", h.GetSessionHistory)
		v1.DELETE("/sessions/:id", h.DeleteSession)

		v1.POST("/sessions/:id/start", h.StartRound)
		v1.POST("/sessions/:id/pause", h.PauseRound)
		v1.POST("/sessions/:id/resume", h.ResumeRound)
		v1.GET("/sessions/:id/rounds/:num", h.GetRound)
		v1.GET("/sessions/:id/stream", h.Stream)
		v1.GET("/sessions/:id/output", h.ReadOutput)
		v1.GET("/sessions/:id/diff", h.ReadDiff)
		v1.GET("/sessions/:id/cards", h.ListCards)
		v1.POST("/sessions/:id/cards/apply-all", h.ApplyAllCards)
		v1.POST("/sessions/:id/cards/:cardId/accept", h.AcceptCard)
		v1.POST("/sessions/:id/cards/:cardId/reject", h.RejectCard)
		v1.GET("/sessions/:id/export", h.Export)

		v1.GET("/health", h.Health)
		v1.GET("/health/live", h.Live)
		v1.GET("/health/ready", h.Ready)

		v1.GET("/agents", h.ListAgents)
		v1.PUT("/agents/:name", h.UpdateAgent)
		v1.POST("/agents/:name/models", h.FetchAgentModels)
	}

	serveFrontend(router, "web/dist")

	return router
}

func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowAll := len(allowedOrigins) == 0
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
			break
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if allowAll && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			for _, allowed := range allowedOrigins {
				if strings.EqualFold(allowed, origin) {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func serveFrontend(router *gin.Engine, distDir string) {
	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return
	}

	assetsDir := filepath.Join(distDir, "assets")
	if _, err := os.Stat(assetsDir); err == nil {
		router.Static("/assets", assetsDir)
	}

	faviconPath := filepath.Join(distDir, "favicon.svg")
	if _, err := os.Stat(faviconPath); err == nil {
		router.StaticFile("/favicon.svg", faviconPath)
	}

	router.GET("/", func(c *gin.Context) {
		c.File(indexPath)
	})

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "The requested page could not be found.",
					"details": gin.H{},
				},
			})
			return
		}
		c.File(indexPath)
	})
}
