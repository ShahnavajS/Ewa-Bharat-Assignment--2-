package api

import (
	"net/http"

	"github.com/eva-bharat/media-sequencer/internal/api/handlers"
	"github.com/eva-bharat/media-sequencer/internal/api/middleware"
	"github.com/eva-bharat/media-sequencer/internal/config"
	"github.com/eva-bharat/media-sequencer/internal/ws"
	"github.com/gin-gonic/gin"
)

// RouterDeps holds all service and handler dependencies injected into the router.
type RouterDeps struct {
	Config          *config.Config
	HealthHandler   *handlers.HealthHandler
	WindowHandler   *handlers.WindowHandler
	MediaHandler    *handlers.MediaHandler
	PlaylistHandler *handlers.PlaylistHandler
	WSHub           *ws.Hub
	SyncHandler     *handlers.SyncHandler
}

// SetupRouter initializes the Gin engine with standard middleware and routes.
func SetupRouter(deps *RouterDeps) *gin.Engine {
	if deps.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	// Base recovery and custom structured logging middleware
	r.Use(gin.Recovery())
	r.Use(middleware.StructuredLogger())
	r.Use(middleware.CORS(deps.Config.CORSAllowedOrigins))

	// API route group
	apiGroup := r.Group("/api")
	{
		// ── Health & Diagnostics ─────────────────────────────────────────────
		apiGroup.GET("/health", deps.HealthHandler.HealthCheck)

		// ── WebSocket ────────────────────────────────────────────────────────
		// GET /api/ws — upgrades to WebSocket for real-time communication
		if deps.WSHub != nil {
			apiGroup.GET("/ws", func(c *gin.Context) {
				deps.WSHub.ServeWS(c.Writer, c.Request)
			})
		}

		// ── Windows ──────────────────────────────────────────────────────────
		windows := apiGroup.Group("/windows")
		{
			windows.GET("", deps.WindowHandler.List)
			windows.POST("", deps.WindowHandler.Create)
			windows.GET("/:id", deps.WindowHandler.GetByID)
			windows.PATCH("/:id", deps.WindowHandler.Update)
			windows.DELETE("/:id", deps.WindowHandler.Delete)

			// ── Playlists (nested under /windows/:id/playlist) ───────────────
			playlist := windows.Group("/:id/playlist")
			{
				playlist.GET("", deps.PlaylistHandler.GetByWindow)
				playlist.POST("", deps.PlaylistHandler.AddItem)
				playlist.DELETE("/:itemId", deps.PlaylistHandler.DeleteItem)
				playlist.PATCH("/reorder", deps.PlaylistHandler.Reorder)
			}
		}

		// ── Media Library ─────────────────────────────────────────────────────
		media := apiGroup.Group("/media")
		{
			media.GET("", deps.MediaHandler.List)
			media.POST("", deps.MediaHandler.Create)
			media.GET("/:id", deps.MediaHandler.GetByID)
			media.PATCH("/:id", deps.MediaHandler.Update)
			media.DELETE("/:id", deps.MediaHandler.Delete)
		}

		// ── Synchronized playback ──────────────────────────────────────────────
		if deps.SyncHandler != nil {
			syncGroup := apiGroup.Group("/sync")
			{
				syncGroup.POST("", deps.SyncHandler.TriggerSync)
				syncGroup.GET("/state", deps.SyncHandler.GetActiveState)
				syncGroup.GET("/status", deps.SyncHandler.GetActiveState)
				syncGroup.POST("/cancel", deps.SyncHandler.CancelSync)
			}
		}
	}

	// Fallback 404 handler for unknown routes
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "The requested resource does not exist",
			},
		})
	})

	return r
}
