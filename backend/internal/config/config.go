package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration values loaded from the environment.
type Config struct {
	Port               string
	Environment        string
	DatabaseURL        string
	CORSAllowedOrigins []string
}

// Load loads configuration from environment variables, optionally loading from .env in development.
func Load() (*Config, error) {
	// Attempt to load .env file if it exists; ignore error in environments where vars are injected directly
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	env := getEnv("ENVIRONMENT", "development")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/eva_media?sslmode=disable")
	corsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")

	if env == "production" {
		if os.Getenv("DATABASE_URL") == "" {
			return nil, fmt.Errorf("DATABASE_URL is required in production environment")
		}
	}

	origins := parseOrigins(corsRaw)

	return &Config{
		Port:               port,
		Environment:        env,
		DatabaseURL:        dbURL,
		CORSAllowedOrigins: origins,
	}, nil
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

// getEnv retrieves an environment variable or returns the fallback value if unset
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

// parseOrigins parses a comma-separated list of CORS origins
func parseOrigins(raw string) []string {
	var origins []string
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:5173"}
	}
	return origins
}
