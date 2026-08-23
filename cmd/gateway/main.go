package main

import (
	"log"
	"net/http"
	"time"
	"fmt"

	"github.com/soham312/api-gateway-go/internal/balancer"
	"github.com/soham312/api-gateway-go/internal/config"
	"github.com/soham312/api-gateway-go/internal/health"
	"github.com/soham312/api-gateway-go/internal/middleware"
	"github.com/soham312/api-gateway-go/internal/proxy"
	"github.com/soham312/api-gateway-go/internal/router"
)

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil || d == 0 {
		return defaultVal
	}
	return d
}

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var allBackends []*health.Backend
	var routes []router.Route

	for _, routeCfg := range cfg.Routes {
		var routeBackends []*health.Backend
		for _, b := range routeCfg.Backends {
			backend := health.NewBackend(b.URL, b.Weight)
			routeBackends = append(routeBackends, backend)
			allBackends = append(allBackends, backend)
		}

		var b balancer.Balancer
		if routeCfg.Balancer == "least_conn" {
			b = balancer.NewLeastConnections(routeBackends)
		} else {
			b = balancer.NewWeightedRoundRobin(routeBackends)
		}

		routes = append(routes, router.Route{
			Prefix:      routeCfg.PathPrefix,
			StripPrefix: routeCfg.StripPrefix,
			Balancer:    b,
		})
	}

	// Start health poller
	poller := health.NewPoller(allBackends)
	poller.Start()

	// Setup Router and Proxy
	r := router.NewRouter(routes)
	p := proxy.New(r)

	// Setup Middlewares
	var handler http.Handler = p
	
	rl := middleware.NewRateLimiter(
		cfg.Middleware.RateLimit.RequestsPerSecond,
		cfg.Middleware.RateLimit.Burst,
		parseDuration(cfg.Middleware.RateLimit.TTL, 5*time.Minute),
		parseDuration(cfg.Middleware.RateLimit.CleanupInterval, 1*time.Minute),
	)
	handler = rl.Middleware(handler)

	if cfg.Middleware.JWT.Secret != "" {
		jwtAuth := middleware.NewJWTAuth(cfg.Middleware.JWT.Secret)
		handler = jwtAuth.Middleware(handler)
	}

	cors := middleware.NewCORS(cfg.Middleware.CORS.AllowedOrigins)
	handler = cors.Middleware(handler)

	handler = middleware.LoggingMiddleware(handler)

	readTimeout := parseDuration(cfg.Server.ReadTimeout, 10*time.Second)
	writeTimeout := parseDuration(cfg.Server.WriteTimeout, 10*time.Second)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	if cfg.Server.TLS.Enabled {
		log.Printf("🚀 API Gateway running on https://localhost:%d", cfg.Server.Port)
		err = srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
	} else {
		log.Printf("🚀 API Gateway running on http://localhost:%d", cfg.Server.Port)
		err = srv.ListenAndServe()
	}

	if err != nil {
		log.Fatal(err)
	}
}
