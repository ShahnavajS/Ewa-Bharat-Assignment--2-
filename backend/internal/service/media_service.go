package service

import (
	"context"
	"errors"
	"strings"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
)

// CreateMediaRequest represents the payload to create a new media asset
type CreateMediaRequest struct {
	Title                  string           `json:"title"`
	Type                   domain.MediaType `json:"type"`
	URL                    string           `json:"url"`
	DefaultDurationSeconds int              `json:"default_duration_seconds"`
}

// UpdateMediaRequest represents the payload to update an existing media asset
type UpdateMediaRequest struct {
	Title                  *string           `json:"title"`
	Type                   *domain.MediaType `json:"type"`
	URL                    *string           `json:"url"`
	DefaultDurationSeconds *int              `json:"default_duration_seconds"`
}

// MediaService defines business logic for the global media asset library
type MediaService interface {
	GetAll(ctx context.Context) ([]domain.MediaItem, error)
	GetByID(ctx context.Context, id int) (*domain.MediaItem, error)
	Create(ctx context.Context, req *CreateMediaRequest) (*domain.MediaItem, error)
	Update(ctx context.Context, id int, req *UpdateMediaRequest) (*domain.MediaItem, error)
	Delete(ctx context.Context, id int) error
}

type mediaService struct {
	repo postgres.MediaRepository
}

// NewMediaService creates a new MediaService
func NewMediaService(repo postgres.MediaRepository) MediaService {
	return &mediaService{repo: repo}
}

func (s *mediaService) GetAll(ctx context.Context) ([]domain.MediaItem, error) {
	return s.repo.GetAll(ctx)
}

func (s *mediaService) GetByID(ctx context.Context, id int) (*domain.MediaItem, error) {
	media, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewMediaNotFoundError(id)
		}
		return nil, domain.NewInternalServerError("Failed to retrieve media item")
	}
	return media, nil
}

func (s *mediaService) Create(ctx context.Context, req *CreateMediaRequest) (*domain.MediaItem, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, domain.NewValidationError("Media title is required and cannot be empty", nil)
	}

	mediaType := domain.MediaType(strings.ToLower(string(req.Type)))
	switch mediaType {
	case domain.MediaTypeImage, domain.MediaTypeVideo, domain.MediaTypeBlank:
		// Valid type
	default:
		return nil, domain.NewInvalidMediaTypeError("Invalid media type; must be 'image', 'video', or 'blank'")
	}

	url := strings.TrimSpace(req.URL)
	if mediaType == domain.MediaTypeBlank {
		if url == "" {
			url = "blank"
		}
	} else {
		if url == "" {
			return nil, domain.NewValidationError("URL is required for image and video media types", nil)
		}
	}

	duration := req.DefaultDurationSeconds
	if duration <= 0 {
		return nil, domain.NewInvalidDurationError("Default duration must be greater than 0")
	}

	media := &domain.MediaItem{
		Title:                  title,
		Type:                   mediaType,
		URL:                    url,
		DefaultDurationSeconds: duration,
	}

	if err := s.repo.Create(ctx, media); err != nil {
		return nil, domain.NewInternalServerError("Failed to create media item")
	}

	return media, nil
}

func (s *mediaService) Update(ctx context.Context, id int, req *UpdateMediaRequest) (*domain.MediaItem, error) {
	media, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewMediaNotFoundError(id)
		}
		return nil, domain.NewInternalServerError("Failed to retrieve media item for update")
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, domain.NewValidationError("Media title cannot be empty", nil)
		}
		media.Title = title
	}

	if req.Type != nil {
		mediaType := domain.MediaType(strings.ToLower(string(*req.Type)))
		switch mediaType {
		case domain.MediaTypeImage, domain.MediaTypeVideo, domain.MediaTypeBlank:
			media.Type = mediaType
		default:
			return nil, domain.NewInvalidMediaTypeError("Invalid media type; must be 'image', 'video', or 'blank'")
		}
	}

	if req.URL != nil {
		url := strings.TrimSpace(*req.URL)
		if media.Type == domain.MediaTypeBlank {
			if url == "" {
				url = "blank"
			}
		} else {
			if url == "" {
				return nil, domain.NewValidationError("URL is required for image and video media types", nil)
			}
		}
		media.URL = url
	}

	if req.DefaultDurationSeconds != nil {
		if *req.DefaultDurationSeconds <= 0 {
			return nil, domain.NewInvalidDurationError("Default duration must be greater than 0")
		}
		media.DefaultDurationSeconds = *req.DefaultDurationSeconds
	}

	if err := s.repo.Update(ctx, media); err != nil {
		return nil, domain.NewInternalServerError("Failed to update media item")
	}

	return media, nil
}

func (s *mediaService) Delete(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewMediaNotFoundError(id)
		}
		if errors.Is(err, domain.ErrConstraintViolation) {
			return domain.NewMediaInUseError(id)
		}
		return domain.NewInternalServerError("Failed to delete media item")
	}
	return nil
}
