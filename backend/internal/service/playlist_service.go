package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
)

// AddPlaylistItemRequest represents the payload to add an item to a window's playlist
type AddPlaylistItemRequest struct {
	MediaID         int `json:"media_id"`
	DurationSeconds int `json:"duration_seconds"`
}

// ReorderPlaylistRequest represents the payload to reorder a window's playlist
type ReorderPlaylistRequest struct {
	ItemIDs []int `json:"item_ids"`
}

// PlaylistResponse encapsulates the window ID and ordered sequence items for frontend display
type PlaylistResponse struct {
	WindowID int                   `json:"window_id"`
	Items    []domain.PlaylistItem `json:"items"`
}

// PlaylistService defines business logic for window playlist sequences
type PlaylistService interface {
	GetByWindowID(ctx context.Context, windowID int) (*PlaylistResponse, error)
	AddItem(ctx context.Context, windowID int, req *AddPlaylistItemRequest) (*domain.PlaylistItem, error)
	DeleteItem(ctx context.Context, windowID, itemID int) error
	Reorder(ctx context.Context, windowID int, req *ReorderPlaylistRequest) (*PlaylistResponse, error)
}

type playlistService struct {
	playlistRepo postgres.PlaylistRepository
	windowRepo   postgres.WindowRepository
	mediaRepo    postgres.MediaRepository
}

// NewPlaylistService creates a new PlaylistService
func NewPlaylistService(
	playlistRepo postgres.PlaylistRepository,
	windowRepo postgres.WindowRepository,
	mediaRepo postgres.MediaRepository,
) PlaylistService {
	return &playlistService{
		playlistRepo: playlistRepo,
		windowRepo:   windowRepo,
		mediaRepo:    mediaRepo,
	}
}

func (s *playlistService) GetByWindowID(ctx context.Context, windowID int) (*PlaylistResponse, error) {
	// Verify window exists
	if _, err := s.windowRepo.GetByID(ctx, windowID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewWindowNotFoundError(windowID)
		}
		return nil, domain.NewInternalServerError("Failed to verify window")
	}

	items, err := s.playlistRepo.GetByWindowID(ctx, windowID)
	if err != nil {
		return nil, domain.NewInternalServerError("Failed to retrieve playlist")
	}

	return &PlaylistResponse{
		WindowID: windowID,
		Items:    items,
	}, nil
}

func (s *playlistService) AddItem(ctx context.Context, windowID int, req *AddPlaylistItemRequest) (*domain.PlaylistItem, error) {
	// 1. Verify window exists
	if _, err := s.windowRepo.GetByID(ctx, windowID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewWindowNotFoundError(windowID)
		}
		return nil, domain.NewInternalServerError("Failed to verify window")
	}

	// 2. Verify media item exists
	media, err := s.mediaRepo.GetByID(ctx, req.MediaID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewMediaNotFoundError(req.MediaID)
		}
		return nil, domain.NewInternalServerError("Failed to verify media item")
	}

	// 3. Resolve duration
	duration := req.DurationSeconds
	if duration == 0 {
		duration = media.DefaultDurationSeconds
	} else if duration < 0 {
		return nil, domain.NewInvalidDurationError("Duration must be greater than 0")
	}

	// 4. Atomically append item via repository
	item, err := s.playlistRepo.Add(ctx, windowID, req.MediaID, duration)
	if err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, domain.NewInternalServerError("Failed to append playlist item")
	}

	return item, nil
}

func (s *playlistService) DeleteItem(ctx context.Context, windowID, itemID int) error {
	// 1. Verify window exists
	if _, err := s.windowRepo.GetByID(ctx, windowID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewWindowNotFoundError(windowID)
		}
		return domain.NewInternalServerError("Failed to verify window")
	}

	// 2. Verify playlist item exists
	item, err := s.playlistRepo.GetByID(ctx, itemID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewPlaylistItemNotFoundError(itemID)
		}
		return domain.NewInternalServerError("Failed to verify playlist item")
	}

	// 3. Verify ownership: item must belong to the specified window
	if item.WindowID != windowID {
		return domain.NewPlaylistItemNotFoundError(itemID)
	}

	// 4. Delete item and compact remaining positions atomically
	if err := s.playlistRepo.Delete(ctx, windowID, itemID); err != nil {
		return domain.NewInternalServerError("Failed to delete playlist item")
	}

	return nil
}

func (s *playlistService) Reorder(ctx context.Context, windowID int, req *ReorderPlaylistRequest) (*PlaylistResponse, error) {
	// 1. Verify window exists
	if _, err := s.windowRepo.GetByID(ctx, windowID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewWindowNotFoundError(windowID)
		}
		return nil, domain.NewInternalServerError("Failed to verify window")
	}

	// 2. Fetch existing playlist items
	existingItems, err := s.playlistRepo.GetByWindowID(ctx, windowID)
	if err != nil {
		return nil, domain.NewInternalServerError("Failed to retrieve existing playlist")
	}

	if len(existingItems) == 0 {
		if len(req.ItemIDs) > 0 {
			return nil, domain.NewInvalidReorderError("Cannot reorder an empty playlist with item IDs")
		}
		return &PlaylistResponse{WindowID: windowID, Items: []domain.PlaylistItem{}}, nil
	}

	if len(req.ItemIDs) == 0 {
		return nil, domain.NewInvalidReorderError("item_ids list cannot be empty for a non-empty playlist")
	}

	if len(req.ItemIDs) != len(existingItems) {
		return nil, domain.NewInvalidReorderError(
			fmt.Sprintf("item_ids count (%d) must exactly match existing playlist items count (%d)", len(req.ItemIDs), len(existingItems)),
		)
	}

	// 3. Validate IDs: no duplicates, and every ID must belong to this window
	existingIDMap := make(map[int]bool, len(existingItems))
	for _, it := range existingItems {
		existingIDMap[it.ID] = true
	}

	seen := make(map[int]bool, len(req.ItemIDs))
	var reorderPlan []postgres.ReorderItem

	for posIdx, id := range req.ItemIDs {
		if seen[id] {
			return nil, domain.NewInvalidReorderError(fmt.Sprintf("Duplicate item ID %d found in reorder list", id))
		}
		seen[id] = true

		if !existingIDMap[id] {
			return nil, domain.NewInvalidReorderError(fmt.Sprintf("Item ID %d does not belong to window %d", id, windowID))
		}

		reorderPlan = append(reorderPlan, postgres.ReorderItem{
			ID:       id,
			Position: posIdx + 1, // 1-indexed
		})
	}

	// 4. Execute collision-safe reorder
	if err := s.playlistRepo.Reorder(ctx, windowID, reorderPlan); err != nil {
		return nil, domain.NewInternalServerError("Failed to apply playlist reordering")
	}

	// 5. Return updated sequence
	updatedItems, err := s.playlistRepo.GetByWindowID(ctx, windowID)
	if err != nil {
		return nil, domain.NewInternalServerError("Failed to fetch reordered playlist")
	}

	return &PlaylistResponse{
		WindowID: windowID,
		Items:    updatedItems,
	}, nil
}
