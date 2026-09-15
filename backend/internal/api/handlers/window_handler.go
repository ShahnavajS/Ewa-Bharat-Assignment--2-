package handlers

import (
	"net/http"
	"strconv"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/gin-gonic/gin"
)

// WindowHandler handles HTTP requests for window management.
type WindowHandler struct {
	svc         service.WindowService
	playbackSvc service.PlaybackService
}

// NewWindowHandler creates a new WindowHandler.
func NewWindowHandler(svc service.WindowService, playbackSvc service.PlaybackService) *WindowHandler {
	return &WindowHandler{svc: svc, playbackSvc: playbackSvc}
}

// List godoc
// GET /api/windows
// Returns a JSON array of all display windows.
func (h *WindowHandler) List(c *gin.Context) {
	windows, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": windows})
}

// GetByID godoc
// GET /api/windows/:id
// Returns a single window with its playlist and current deterministic playback
// state computed at time.Now() UTC.
func (h *WindowHandler) GetByID(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	resp, err := h.playbackSvc.GetWindowWithPlayback(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// Create godoc
// POST /api/windows
// Creates a new display window.
func (h *WindowHandler) Create(c *gin.Context) {
	var req service.CreateWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid JSON body: "+err.Error()))
		return
	}

	window, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": window})
}

// Update godoc
// PATCH /api/windows/:id
// Partially updates a display window (name, description, cycle_duration_seconds).
func (h *WindowHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req service.UpdateWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid JSON body: "+err.Error()))
		return
	}

	window, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": window})
}

// Delete godoc
// DELETE /api/windows/:id
// Deletes a display window and all its playlist items (cascade).
func (h *WindowHandler) Delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Window deleted successfully"})
}

// --- helpers ---

// parseID extracts and validates an integer path parameter by name.
func parseID(c *gin.Context, param string) (int, bool) {
	raw := c.Param(param)
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid "+param+": must be a positive integer"))
		return 0, false
	}
	return id, true
}

// respondError maps a domain error to the appropriate HTTP status and response body.
func respondError(c *gin.Context, err error) {
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
		domain.ErrCodeInvalidReorder,
		domain.ErrCodePlaylistTooLong:
		status = http.StatusBadRequest
	case domain.ErrCodeWindowNotFound,
		domain.ErrCodeMediaNotFound,
		domain.ErrCodePlaylistItemNotFound:
		status = http.StatusNotFound
	case domain.ErrCodeMediaInUse:
		status = http.StatusConflict
	}

	c.JSON(status, errorBody(domErr.Code, domErr.Message))
}

// errorBody constructs the standard error JSON envelope.
func errorBody(code, message string) gin.H {
	return gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	}
}
