package config

import (
	"testing"
)

func TestLoad(t *testing.T) {
	cfg, err := Load("../../testdata/config.json")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Middleware.RateLimit.Burst != 5 {
		t.Errorf("Expected burst 5, got %d", cfg.Middleware.RateLimit.Burst)
	}
}
