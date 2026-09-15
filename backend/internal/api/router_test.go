package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eva-bharat/media-sequencer/internal/api/handlers"
	"github.com/eva-bharat/media-sequencer/internal/config"
	"github.com/eva-bharat/media-sequencer/internal/domain"
)

func setupTestRouter() *RouterDeps {
	cfg := &config.Config{
		Port:               "8080",
		Environment:        "test",
		CORSAllowedOrigins: []string{"http://localhost:5173", "https://app.example.com"},
	}
	healthHandler := handlers.NewHealthHandler(cfg, nil)

	return &RouterDeps{
		Config:        cfg,
		HealthHandler: healthHandler,
	}
}

func TestHealthCheckEndpoint(t *testing.T) {
	deps := setupTestRouter()
	router := SetupRouter(deps)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", w.Code)
	}

	var resp domain.ResponseEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected resp.Success to be true")
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("Expected data to be a JSON object, got: %T", resp.Data)
	}

	if dataMap["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", dataMap["status"])
	}

	if dataMap["service"] != "eva-media-sequencer" {
		t.Errorf("Expected service 'eva-media-sequencer', got '%v'", dataMap["service"])
	}

	if dataMap["timestamp"] == nil || dataMap["timestamp"] == "" {
		t.Errorf("Expected non-empty timestamp")
	}
}

func TestCORSMiddleware(t *testing.T) {
	deps := setupTestRouter()
	router := SetupRouter(deps)

	// Test 1: Allowed Origin
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	router.ServeHTTP(w, req)

	originHeader := w.Header().Get("Access-Control-Allow-Origin")
	if originHeader != "http://localhost:5173" {
		t.Errorf("Expected CORS origin 'http://localhost:5173', got '%s'", originHeader)
	}

	// Test 2: Preflight OPTIONS
	wPreflight := httptest.NewRecorder()
	reqPreflight, _ := http.NewRequest(http.MethodOptions, "/api/health", nil)
	reqPreflight.Header.Set("Origin", "https://app.example.com")
	router.ServeHTTP(wPreflight, reqPreflight)

	if wPreflight.Code != http.StatusNoContent {
		t.Errorf("Expected preflight 204 No Content, got %d", wPreflight.Code)
	}

	// Test 3: Disallowed Origin should not receive Access-Control-Allow-Origin
	wDisallowed := httptest.NewRecorder()
	reqDisallowed, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
	reqDisallowed.Header.Set("Origin", "https://evil.com")
	router.ServeHTTP(wDisallowed, reqDisallowed)

	disallowedOrigin := wDisallowed.Header().Get("Access-Control-Allow-Origin")
	if disallowedOrigin != "" {
		t.Errorf("Expected empty Access-Control-Allow-Origin for unauthorized origin, got '%s'", disallowedOrigin)
	}
}

func TestNotFoundRoute(t *testing.T) {
	deps := setupTestRouter()
	router := SetupRouter(deps)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", w.Code)
	}
}
