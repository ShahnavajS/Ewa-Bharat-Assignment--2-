package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eva-bharat/media-sequencer/internal/api"
	"github.com/eva-bharat/media-sequencer/internal/api/handlers"
	"github.com/eva-bharat/media-sequencer/internal/config"
	"github.com/eva-bharat/media-sequencer/internal/domain"
)

func TestHealthEndpointIntegration(t *testing.T) {
	cfg := &config.Config{
		Port:               "8080",
		Environment:        "integration-test",
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}

	healthHandler := handlers.NewHealthHandler(cfg, nil)
	router := api.SetupRouter(&api.RouterDeps{
		Config:        cfg,
		HealthHandler: healthHandler,
	})

	req, err := http.NewRequest(http.MethodGet, "/api/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 OK, got: %d", rec.Code)
	}

	var envelope domain.ResponseEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if !envelope.Success {
		t.Errorf("Expected envelope.Success to be true")
	}

	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("Expected response Data to be JSON object")
	}

	if data["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", data["status"])
	}

	if data["environment"] != "integration-test" {
		t.Errorf("Expected environment 'integration-test', got '%v'", data["environment"])
	}
}
