package config

import (
	"encoding/json"
	"log"
	"os"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

var activeConfig atomic.Value

func Get() *Config {
	v := activeConfig.Load()
	if v != nil {
		return v.(*Config)
	}
	return nil
}

type Config struct {
	Server             ServerConfig     `json:"server"`
	Middleware         MiddlewareConfig `json:"middleware"`
	Routes             []RouteConfig    `json:"routes"`
	HealthCheckPath    string           `json:"health_check_path"`
	RetryMaxAttempts   int              `json:"retry_max_attempts"`
	RetryBaseDelay     string           `json:"retry_base_delay"`
	RetryMaxDelay      string           `json:"retry_max_delay"`
	CBFailureThreshold int              `json:"cb_failure_threshold"`
	CBSuccessThreshold int              `json:"cb_success_threshold"`
	CBTimeout          string           `json:"cb_timeout"`
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
		AllowedMethods []string `json:"allowed_methods"`
		AllowedHeaders []string `json:"allowed_headers"`
	} `json:"cors"`
	JWT struct {
		Secret string `json:"secret"`
	} `json:"jwt"`
	RateLimit struct {
		RequestsPerSecond float64 `json:"requests_per_second"`
		Burst             int     `json:"burst"`
		CleanupInterval   string  `json:"cleanup_interval"`
		TTL               string  `json:"ttl"`
		// TrustProxyHeaders enables reading the client IP from
		// X-Forwarded-For/X-Real-IP. Only enable this when the gateway is
		// deployed behind a proxy/load balancer that sets these headers
		// itself, otherwise a client can spoof them to bypass rate limiting.
		TrustProxyHeaders bool `json:"trust_proxy_headers"`
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
	cfg, err := parseConfig(path)
	if err != nil {
		return nil, err
	}

	activeConfig.Store(cfg)
	return cfg, nil
}

func parseConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	strData := os.ExpandEnv(string(data))

	var cfg Config
	if err := json.Unmarshal([]byte(strData), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Watch(path string, onChange func(*Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					log.Printf("Config file changed: %s", event.Name)
					newCfg, err := parseConfig(path)
					if err != nil {
						log.Printf("Error loading new config: %v", err)
					} else {
						activeConfig.Store(newCfg)
						if onChange != nil {
							onChange(newCfg)
						}
						log.Println("Successfully reloaded configuration in memory")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	return watcher.Add(path)
}
