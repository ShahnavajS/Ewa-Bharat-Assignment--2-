package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/config"
	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler handles health check and diagnostic requests.
type HealthHandler struct {
	cfg  *config.Config
	pool *pgxpool.Pool
}

// NewHealthHandler creates a new HealthHandler instance with optional database pool.
func NewHealthHandler(cfg *config.Config, pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{cfg: cfg, pool: pool}
}

// HealthCheck responds with the operational status, database connectivity, and server timestamp.
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	dbStatus := "unconfigured"
	status := "ok"
	httpStatus := http.StatusOK

	if h.pool != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := h.pool.Ping(ctx); err == nil {
			dbStatus = "connected"
		} else {
			dbStatus = "unreachable"
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}
	}

	c.JSON(httpStatus, domain.ResponseEnvelope{
		Success: status == "ok",
		Data: gin.H{
			"status":      status,
			"service":     "eva-media-sequencer",
			"environment": h.cfg.Environment,
			"database":    dbStatus,
			"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
}
