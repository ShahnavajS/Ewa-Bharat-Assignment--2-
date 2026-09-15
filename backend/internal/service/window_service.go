package service

import (
	"context"
	"errors"
	"strings"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
)

// CreateWindowRequest represents the payload to create a new display window
type CreateWindowRequest struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	CycleDurationSeconds int    `json:"cycle_duration_seconds"`
}

// UpdateWindowRequest represents the payload to update an existing window
type UpdateWindowRequest struct {
	Name                 *string `json:"name"`
	Description          *string `json:"description"`
	CycleDurationSeconds *int    `json:"cycle_duration_seconds"`
}

// WindowService defines the business logic for window management
type WindowService interface {
	GetAll(ctx context.Context) ([]domain.Window, error)
	GetByID(ctx context.Context, id int) (*domain.Window, error)
	Create(ctx context.Context, req *CreateWindowRequest) (*domain.Window, error)
	Update(ctx context.Context, id int, req *UpdateWindowRequest) (*domain.Window, error)
	Delete(ctx context.Context, id int) error
}

type windowService struct {
	repo postgres.WindowRepository
}

// NewWindowService creates a new WindowService
func NewWindowService(repo postgres.WindowRepository) WindowService {
	return &windowService{repo: repo}
}

func (s *windowService) GetAll(ctx context.Context) ([]domain.Window, error) {
	return s.repo.GetAll(ctx)
}

func (s *windowService) GetByID(ctx context.Context, id int) (*domain.Window, error) {
	window, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewWindowNotFoundError(id)
		}
		return nil, domain.NewInternalServerError("Failed to retrieve window")
	}
	return window, nil
}

func (s *windowService) Create(ctx context.Context, req *CreateWindowRequest) (*domain.Window, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, domain.NewValidationError("Window name is required and cannot be empty", nil)
	}

	cycleSeconds := req.CycleDurationSeconds
	if cycleSeconds == 0 {
		cycleSeconds = 18000 // 5-hour cycle default
	} else if cycleSeconds < 0 {
		return nil, domain.NewInvalidDurationError("Cycle duration must be greater than 0")
	}

	window := &domain.Window{
		Name:                 name,
		Description:          strings.TrimSpace(req.Description),
		CycleDurationSeconds: cycleSeconds,
	}

	if err := s.repo.Create(ctx, window); err != nil {
		return nil, domain.NewInternalServerError("Failed to create window")
	}

	return window, nil
}

func (s *windowService) Update(ctx context.Context, id int, req *UpdateWindowRequest) (*domain.Window, error) {
	window, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewWindowNotFoundError(id)
		}
		return nil, domain.NewInternalServerError("Failed to retrieve window for update")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, domain.NewValidationError("Window name cannot be empty", nil)
		}
		window.Name = name
	}

	if req.Description != nil {
		window.Description = strings.TrimSpace(*req.Description)
	}

	if req.CycleDurationSeconds != nil {
		if *req.CycleDurationSeconds <= 0 {
			return nil, domain.NewInvalidDurationError("Cycle duration must be greater than 0")
		}
		window.CycleDurationSeconds = *req.CycleDurationSeconds
	}

	if err := s.repo.Update(ctx, window); err != nil {
		return nil, domain.NewInternalServerError("Failed to update window")
	}

	return window, nil
}

func (s *windowService) Delete(ctx context.Context, id int) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewWindowNotFoundError(id)
		}
		return domain.NewInternalServerError("Failed to delete window")
	}
	return nil
}
