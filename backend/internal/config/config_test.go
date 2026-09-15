package config

import (
	"os"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	// Clear any overrides for test
	os.Unsetenv("PORT")
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("CORS_ALLOWED_ORIGINS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected Load() to succeed with defaults, got: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Expected default Port '8080', got '%s'", cfg.Port)
	}

	if cfg.Environment != "development" {
		t.Errorf("Expected default Environment 'development', got '%s'", cfg.Environment)
	}

	if cfg.IsProduction() {
		t.Errorf("Expected IsProduction() to be false in development")
	}

	if len(cfg.CORSAllowedOrigins) == 0 {
		t.Errorf("Expected at least one default CORS allowed origin")
	}
}

func TestConfigCustomEnvironment(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("ENVIRONMENT", "test")
	os.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.com, https://admin.com")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("CORS_ALLOWED_ORIGINS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected Load() to succeed, got: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Expected Port '9090', got '%s'", cfg.Port)
	}

	if cfg.Environment != "test" {
		t.Errorf("Expected Environment 'test', got '%s'", cfg.Environment)
	}

	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("Expected 2 CORS origins, got %d", len(cfg.CORSAllowedOrigins))
	}

	if cfg.CORSAllowedOrigins[0] != "https://frontend.com" || cfg.CORSAllowedOrigins[1] != "https://admin.com" {
		t.Errorf("Unexpected CORS origins: %v", cfg.CORSAllowedOrigins)
	}
}

func TestConfigProductionValidation(t *testing.T) {
	os.Setenv("ENVIRONMENT", "production")
	os.Unsetenv("DATABASE_URL")
	defer func() {
		os.Unsetenv("ENVIRONMENT")
	}()

	_, err := Load()
	if err == nil {
		t.Errorf("Expected Load() to fail in production when DATABASE_URL is missing")
	}
}
