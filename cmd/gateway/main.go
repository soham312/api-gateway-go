package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soham312/api-gateway-go/internal/balancer"
	"github.com/soham312/api-gateway-go/internal/config"
	"github.com/soham312/api-gateway-go/internal/health"
	"github.com/soham312/api-gateway-go/internal/metrics"
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

	buildRoutesAndPoller := func(c *config.Config) ([]router.Route, []*health.Backend) {
		var allBackends []*health.Backend
		var routes []router.Route

		for _, routeCfg := range c.Routes {
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
		return routes, allBackends
	}

	routes, allBackends := buildRoutesAndPoller(cfg)

	// Setup Router and Proxy
	r := router.NewRouter(routes)
	p := proxy.New(r)

	// Start health poller
	poller := health.NewPoller(allBackends)
	poller.Start()

	// Watch config for hot-reloading
	go func() {
		err := config.Watch("config.json", func(newCfg *config.Config) {
			newRoutes, newBackends := buildRoutesAndPoller(newCfg)
			r.UpdateRoutes(newRoutes)
			poller.UpdateBackends(newBackends)
		})
		if err != nil {
			log.Printf("Failed to watch config.json: %v", err)
		}
	}()

	// Setup Middlewares
	var handler http.Handler = p

	rl := middleware.NewRateLimiter(
		cfg.Middleware.RateLimit.RequestsPerSecond,
		cfg.Middleware.RateLimit.Burst,
		parseDuration(cfg.Middleware.RateLimit.TTL, 5*time.Minute),
		parseDuration(cfg.Middleware.RateLimit.CleanupInterval, 1*time.Minute),
		cfg.Middleware.RateLimit.TrustProxyHeaders,
	)
	handler = rl.Middleware(handler)

	if cfg.Middleware.JWT.Secret != "" {
		jwtAuth := middleware.NewJWTAuth(cfg.Middleware.JWT.Secret)
		handler = jwtAuth.Middleware(handler)
	} else {
		log.Println("⚠️  JWT authentication is DISABLED: no middleware.jwt.secret configured. All routes are unauthenticated.")
	}

	cors := middleware.NewCORS(cfg.Middleware.CORS.AllowedOrigins)
	handler = cors.Middleware(handler)

	handler = middleware.LoggingMiddleware(handler)

	m := metrics.New()
	handler = m.Middleware(handler)

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler(func() map[string]int {
		states := make(map[string]int)
		for _, b := range poller.Backends() {
			states[b.URL] = int(b.GetState())
		}
		return states
	}))
	mux.Handle("/", handler)

	readTimeout := parseDuration(cfg.Server.ReadTimeout, 10*time.Second)
	writeTimeout := parseDuration(cfg.Server.WriteTimeout, 10*time.Second)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	go func() {
		var serveErr error
		if cfg.Server.TLS.Enabled {
			log.Printf("🚀 API Gateway running on https://localhost:%d", cfg.Server.Port)
			serveErr = srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			log.Printf("🚀 API Gateway running on http://localhost:%d", cfg.Server.Port)
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", serveErr)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutdown signal received, draining in-flight requests...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}
	log.Println("✅ Gateway shut down cleanly")
}
