package handlers

import (
	"net/http"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/gin-gonic/gin"
)

// SyncHandler handles REST API endpoints for global synchronized playback management.
type SyncHandler struct {
	syncSvc service.SyncService
}

// NewSyncHandler creates a new SyncHandler with the given SyncService.
func NewSyncHandler(syncSvc service.SyncService) *SyncHandler {
	return &SyncHandler{syncSvc: syncSvc}
}

// TriggerSync godoc
// POST /api/sync
// Initiates a global synchronized playback override across all display windows.
func (h *SyncHandler) TriggerSync(c *gin.Context) {
	var req service.StartSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid JSON request body: "+err.Error()))
		return
	}

	event, err := h.syncSvc.Start(c.Request.Context(), &req)
	if err != nil {
		respondSyncError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    event,
	})
}

// GetActiveState godoc
// GET /api/sync/state
// Returns the currently active global sync state or active: false.
func (h *SyncHandler) GetActiveState(c *gin.Context) {
	state, err := h.syncSvc.GetActive(c.Request.Context())
	if err != nil {
		respondSyncError(c, err)
		return
	}

	if !state.Active {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"active":  false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"active":            true,
		"sync_id":           state.SyncID,
		"media":             state.Media,
		"started_at":        state.StartedAt,
		"ends_at":           state.EndsAt,
		"duration_seconds":  state.DurationSeconds,
		"remaining_seconds": state.RemainingSeconds,
		"server_time":       state.ServerTime,
	})
}

// CancelSync godoc
// POST /api/sync/cancel
// Prematurely cancels the ongoing global sync playback event.
func (h *SyncHandler) CancelSync(c *gin.Context) {
	event, err := h.syncSvc.Cancel(c.Request.Context())
	if err != nil {
		respondSyncError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Sync playback cancelled successfully",
		"data":    event,
	})
}

// respondSyncError maps domain errors to the correct HTTP status code.
func respondSyncError(c *gin.Context, err error) {
	domErr, ok := err.(*domain.AppError)
	if !ok {
		c.JSON(http.StatusInternalServerError, errorBody("INTERNAL_SERVER_ERROR", "An unexpected error occurred"))
		return
	}

	status := http.StatusInternalServerError
	switch domErr.Code {
	case domain.ErrCodeValidation,
		domain.ErrCodeInvalidDuration,
		domain.ErrCodeInvalidMediaType,
		domain.ErrCodeInvalidSyncDuration:
		status = http.StatusBadRequest
	case domain.ErrCodeMediaNotFound,
		domain.ErrCodeSyncNotFound:
		status = http.StatusNotFound
	case domain.ErrCodeSyncAlreadyActive,
		domain.ErrCodeSyncNotActive,
		domain.ErrCodeMediaInUse,
		domain.ErrCodeConflict:
		status = http.StatusConflict
	}

	c.JSON(status, errorBody(domErr.Code, domErr.Message))
}
