// Package config loads process configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPAddress         string
	DatabaseURL         string
	SupabaseURL         string
	SupabaseServiceKey  string
	SupabasePhotoBucket string
	CORSAllowedOrigins  []string
	TemporalAddress     string
	TemporalNamespace   string
	TemporalAPIKey      string
	GeminiAPIKey        string
}

// Load reads the environment without requiring deployment secrets during local
// compilation or while running the liveness endpoint.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

func LoadFrom(lookup func(string) (string, bool)) (Config, error) {
	address := ""
	if value, ok := lookup("HTTP_ADDRESS"); ok {
		address = strings.TrimSpace(value)
	}
	if address == "" {
		if port, ok := lookup("PORT"); ok && strings.TrimSpace(port) != "" {
			address = ":" + strings.TrimSpace(port)
		} else {
			address = ":8080"
		}
	}
	cfg := Config{
		HTTPAddress:         address,
		DatabaseURL:         valueOr(lookup, "DATABASE_URL", ""),
		SupabaseURL:         valueOr(lookup, "SUPABASE_URL", ""),
		SupabaseServiceKey:  valueOr(lookup, "SUPABASE_SERVICE_ROLE_KEY", ""),
		SupabasePhotoBucket: valueOr(lookup, "SUPABASE_PHOTO_BUCKET", "mythborn-photos"),
		CORSAllowedOrigins:  splitCSV(valueOr(lookup, "CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		TemporalAddress:     valueOr(lookup, "TEMPORAL_ADDRESS", ""),
		TemporalNamespace:   valueOr(lookup, "TEMPORAL_NAMESPACE", ""),
		TemporalAPIKey:      valueOr(lookup, "TEMPORAL_API_KEY", ""),
		GeminiAPIKey:        valueOr(lookup, "GEMINI_API_KEY", ""),
	}
	if cfg.HTTPAddress == "" {
		return Config{}, fmt.Errorf("HTTP_ADDRESS must not be empty")
	}
	return cfg, nil
}

func splitCSV(value string) []string {
	var values []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func valueOr(lookup func(string) (string, bool), key, fallback string) string {
	if value, ok := lookup(key); ok {
		return value
	}
	return fallback
}
