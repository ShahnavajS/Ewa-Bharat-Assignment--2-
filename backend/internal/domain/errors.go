package domain

import (
	"errors"
	"fmt"
)

// Repository and Service Sentinel Errors
var (
	ErrNotFound            = errors.New("record not found")
	ErrConstraintViolation = errors.New("database constraint violation")
	ErrConflict            = errors.New("resource conflict")
)

// Standardized Domain Error Codes for API responses
const (
	ErrCodeValidation           = "VALIDATION_ERROR"
	ErrCodeWindowNotFound       = "WINDOW_NOT_FOUND"
	ErrCodeMediaNotFound        = "MEDIA_NOT_FOUND"
	ErrCodePlaylistItemNotFound = "PLAYLIST_ITEM_NOT_FOUND"
	ErrCodeInvalidMediaType     = "INVALID_MEDIA_TYPE"
	ErrCodeInvalidDuration      = "INVALID_DURATION"
	ErrCodeInvalidReorder       = "INVALID_REORDER"
	ErrCodePlaylistTooLong      = "PLAYLIST_DURATION_EXCEEDED"
	ErrCodeMediaInUse           = "MEDIA_IN_USE"
	ErrCodeConflict             = "CONFLICT"
	ErrCodeSyncAlreadyActive    = "SYNC_ALREADY_ACTIVE"
	ErrCodeSyncNotActive        = "SYNC_NOT_ACTIVE"
	ErrCodeSyncNotFound         = "SYNC_NOT_FOUND"
	ErrCodeInvalidSyncDuration  = "INVALID_SYNC_DURATION"
	ErrCodeInternalError        = "INTERNAL_ERROR"
	ErrCodeInternalServer       = "INTERNAL_SERVER_ERROR"
)

// AppError represents a structured, domain-level error with a machine-readable code
// and human-readable message.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ErrorResponse wraps AppError to provide standard JSON: { "error": { "code": "...", "message": "..." } }
type ErrorResponse struct {
	Error *AppError `json:"error"`
}

// NewAppError creates a new domain AppError
func NewAppError(code, message string, details any) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// Predefined constructors for standard domain errors
func NewValidationError(message string, details any) *AppError {
	return NewAppError(ErrCodeValidation, message, details)
}

func NewWindowNotFoundError(windowID int) *AppError {
	return NewAppError(ErrCodeWindowNotFound, fmt.Sprintf("Window with ID %d not found", windowID), nil)
}

func NewMediaNotFoundError(mediaID int) *AppError {
	return NewAppError(ErrCodeMediaNotFound, fmt.Sprintf("Media item with ID %d not found", mediaID), nil)
}

func NewPlaylistItemNotFoundError(itemID int) *AppError {
	return NewAppError(ErrCodePlaylistItemNotFound, fmt.Sprintf("Playlist item with ID %d not found", itemID), nil)
}

func NewInvalidMediaTypeError(msg string) *AppError {
	return NewAppError(ErrCodeInvalidMediaType, msg, nil)
}

func NewInvalidDurationError(msg string) *AppError {
	return NewAppError(ErrCodeInvalidDuration, msg, nil)
}

func NewInvalidReorderError(msg string) *AppError {
	return NewAppError(ErrCodeInvalidReorder, msg, nil)
}

func NewPlaylistTooLongError(total, limit int) *AppError {
	return NewAppError(
		ErrCodePlaylistTooLong,
		fmt.Sprintf("Playlist duration would be %d seconds; window cycle limit is %d seconds", total, limit),
		nil,
	)
}

func NewMediaInUseError(mediaID int) *AppError {
	return NewAppError(ErrCodeMediaInUse, fmt.Sprintf("Media item with ID %d is actively configured in playlists and cannot be deleted", mediaID), nil)
}

func NewConflictError(message string) *AppError {
	return NewAppError(ErrCodeConflict, message, nil)
}

func NewSyncAlreadyActiveError() *AppError {
	return NewAppError(ErrCodeSyncAlreadyActive, "A synchronized playback event is already active", nil)
}

func NewSyncNotActiveError() *AppError {
	return NewAppError(ErrCodeSyncNotActive, "No synchronized playback event is currently active", nil)
}

func NewSyncNotFoundError(eventID int) *AppError {
	return NewAppError(ErrCodeSyncNotFound, fmt.Sprintf("Sync event with ID %d not found", eventID), nil)
}

func NewInvalidSyncDurationError(msg string) *AppError {
	return NewAppError(ErrCodeInvalidSyncDuration, msg, nil)
}

func NewInternalServerError(message string) *AppError {
	return NewAppError(ErrCodeInternalError, message, nil)
}
