package middleware

import (
	"net/http"
	"strings"
	
	"github.com/soham312/api-gateway-go/internal/config"
)

type CORS struct {
	AllowedOrigins []string
}

func NewCORS(origins []string) *CORS {
	return &CORS{AllowedOrigins: origins}
}

func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Get()
		allowedOrigins := c.AllowedOrigins
		allowedMethods := "GET, POST, PUT, DELETE, OPTIONS"
		allowedHeaders := "Content-Type, Authorization"
		
		if cfg != nil {
			if len(cfg.Middleware.CORS.AllowedOrigins) > 0 {
				allowedOrigins = cfg.Middleware.CORS.AllowedOrigins
			}
			if len(cfg.Middleware.CORS.AllowedMethods) > 0 {
				allowedMethods = strings.Join(cfg.Middleware.CORS.AllowedMethods, ", ")
			}
			if len(cfg.Middleware.CORS.AllowedHeaders) > 0 {
				allowedHeaders = strings.Join(cfg.Middleware.CORS.AllowedHeaders, ", ")
			}
		}

		origin := r.Header.Get("Origin")
		allowed := false
		
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}
		
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
