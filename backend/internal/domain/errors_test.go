package domain

import (
	"strings"
	"testing"
)

func TestDomainErrors(t *testing.T) {
	valErr := NewValidationError("duration must be positive", map[string]string{"field": "duration"})
	if valErr.Code != ErrCodeValidation {
		t.Errorf("Expected code %s, got %s", ErrCodeValidation, valErr.Code)
	}
	if !strings.Contains(valErr.Error(), "duration must be positive") {
		t.Errorf("Unexpected error message: %s", valErr.Error())
	}

	winErr := NewWindowNotFoundError(99)
	if winErr.Code != ErrCodeWindowNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeWindowNotFound, winErr.Code)
	}

	medErr := NewMediaNotFoundError(101)
	if medErr.Code != ErrCodeMediaNotFound {
		t.Errorf("Expected code %s, got %s", ErrCodeMediaNotFound, medErr.Code)
	}

	syncErr := NewSyncAlreadyActiveError()
	if syncErr.Code != ErrCodeSyncAlreadyActive {
		t.Errorf("Expected code %s, got %s", ErrCodeSyncAlreadyActive, syncErr.Code)
	}
}
