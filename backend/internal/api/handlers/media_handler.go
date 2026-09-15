package handlers

import (
	"net/http"

	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/gin-gonic/gin"
)

// MediaHandler handles HTTP requests for the global media asset library.
type MediaHandler struct {
	svc service.MediaService
}

// NewMediaHandler creates a new MediaHandler.
func NewMediaHandler(svc service.MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

// List godoc
// GET /api/media
// Returns all registered media assets.
func (h *MediaHandler) List(c *gin.Context) {
	items, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// GetByID godoc
// GET /api/media/:id
// Returns a single media asset by its integer ID.
func (h *MediaHandler) GetByID(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	media, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": media})
}

// Create godoc
// POST /api/media
// Registers a new media asset (image/video/blank) in the library.
//
// Request body:
//
//	{
//	  "title": "My Clip",
//	  "type": "video",        // "image" | "video" | "blank"
//	  "url": "https://...",
//	  "default_duration_seconds": 30
//	}
func (h *MediaHandler) Create(c *gin.Context) {
	var req service.CreateMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid JSON body: "+err.Error()))
		return
	}

	media, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": media})
}

// Update godoc
// PATCH /api/media/:id
// Partially updates a media asset.
func (h *MediaHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req service.UpdateMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid JSON body: "+err.Error()))
		return
	}

	media, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": media})
}

// Delete godoc
// DELETE /api/media/:id
// Deletes a media asset. Returns 409 if the asset is still referenced by any playlist.
func (h *MediaHandler) Delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Media item deleted successfully"})
}
