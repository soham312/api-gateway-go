package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Server     ServerConfig     `json:"server"`
	Middleware MiddlewareConfig `json:"middleware"`
	Routes     []RouteConfig    `json:"routes"`
}

type ServerConfig struct {
	Port         int       `json:"port"`
	ReadTimeout  string    `json:"read_timeout"`
	WriteTimeout string    `json:"write_timeout"`
	TLS          TLSConfig `json:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type MiddlewareConfig struct {
	CORS struct {
		AllowedOrigins []string `json:"allowed_origins"`
	} `json:"cors"`
	JWT struct {
		Secret string `json:"secret"`
	} `json:"jwt"`
	RateLimit struct {
		RequestsPerSecond float64 `json:"requests_per_second"`
		Burst             int     `json:"burst"`
		CleanupInterval   string  `json:"cleanup_interval"`
		TTL               string  `json:"ttl"`
	} `json:"rate_limit"`
}

type RouteConfig struct {
	PathPrefix  string          `json:"path_prefix"`
	StripPrefix bool            `json:"strip_prefix"`
	Balancer    string          `json:"balancer"`
	Backends    []BackendConfig `json:"backends"`
}

type BackendConfig struct {
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	// Environment variable interpolation
	strData := os.ExpandEnv(string(data))
	
	var cfg Config
	if err := json.Unmarshal([]byte(strData), &cfg); err != nil {
		return nil, err
	}
	
	return &cfg, nil
}
