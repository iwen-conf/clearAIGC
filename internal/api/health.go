package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	iredis "github.com/iwen-conf/Naturalize/internal/infra/redis"
)

type HealthHandler struct {
	db        *pgxpool.Pool
	redis     *iredis.Client
	startedAt time.Time
	version   string
	buildTime string
}

func NewHealthHandler(db *pgxpool.Pool, redis *iredis.Client, version, buildTime string) *HealthHandler {
	return &HealthHandler{
		db:        db,
		redis:     redis,
		startedAt: time.Now(),
		version:   version,
		buildTime: buildTime,
	}
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	if err := h.redis.Ping(c.Request.Context()).Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	postgresStatus := "healthy"
	redisStatus := "healthy"
	if err := h.db.Ping(ctx); err != nil {
		postgresStatus = "unhealthy"
	}
	if err := h.redis.Ping(ctx).Err(); err != nil {
		redisStatus = "unhealthy"
	}
	status := "healthy"
	if postgresStatus != "healthy" || redisStatus != "healthy" {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"uptime":    time.Since(h.startedAt).String(),
		"checks":    gin.H{"postgres": postgresStatus, "redis": redisStatus, "llm": "configured"},
		"version":   h.version,
		"buildTime": h.buildTime,
	})
}
