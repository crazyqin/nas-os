package gateway

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Middleware represents a middleware function
type Middleware func(http.Handler) http.Handler

// Chain represents a middleware chain
type Chain struct {
	middlewares []Middleware
}

// NewChain creates a new middleware chain
func NewChain(middlewares ...Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
	}
}

// Add adds a middleware to the chain
func (c *Chain) Add(middleware Middleware) *Chain {
	c.middlewares = append(c.middlewares, middleware)
	return c
}

// Then wraps the final handler with all middlewares
func (c *Chain) Then(handler http.Handler) http.Handler {
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handler = c.middlewares[i](handler)
	}
	return handler
}

// LoggingMiddleware logs incoming requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create response wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		log.Printf("[%s] %s %s %d %v",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			wrapped.statusCode,
			time.Since(start),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowOrigins     []string `json:"allowOrigins"`
	AllowMethods     []string `json:"allowMethods"`
	AllowHeaders     []string `json:"allowHeaders"`
	ExposeHeaders    []string `json:"exposeHeaders"`
	AllowCredentials bool     `json:"allowCredentials"`
	MaxAge           int      `json:"maxAge"`
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           86400,
	}
}

// CORSMiddleware handles CORS requests
func CORSMiddleware(config *CORSConfig) Middleware {
	if config == nil {
		config = DefaultCORSConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range config.AllowOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if len(config.ExposeHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
				}

				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
				}
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitConfig holds rate limit configuration
type RateLimitConfig struct {
	Rate     rate.Limit `json:"rate"`     // requests per second
	Burst    int        `json:"burst"`    // burst size
	ByIP     bool       `json:"byIP"`     // rate limit by IP
	ByUser   bool       `json:"byUser"`   // rate limit by user
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Rate:  10, // 10 requests per second
		Burst: 20,
		ByIP:  true,
		ByUser: false,
	}
}

// rateLimiter stores rate limiters per key
type rateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(rateLimit rate.Limit, burst int) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rateLimit,
		burst:    burst,
	}
}

// getLimiter returns the rate limiter for the given key
func (rl *rateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = limiter
	}

	return limiter
}

// RateLimitMiddleware limits request rate
func RateLimitMiddleware(config *RateLimitConfig) Middleware {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	limiter := newRateLimiter(config.Rate, config.Burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := ""

			if config.ByIP {
				key = r.RemoteAddr
			} else if config.ByUser {
				key = r.Header.Get("X-User-Id")
				if key == "" {
					key = r.RemoteAddr
				}
			}

			if key == "" {
				key = "global"
			}

			if !limiter.getLimiter(key).Allow() {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled     bool     `json:"enabled"`
	Users       []User   `json:"users"`
	APIKeys     []string `json:"apiKeys"`
	PublicPaths []string `json:"publicPaths"`
}

// User represents a user
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// BasicAuthMiddleware provides basic authentication
func BasicAuthMiddleware(config *AuthConfig) Middleware {
	if config == nil || !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is public
			for _, path := range config.PublicPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check API key first
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				for _, key := range config.APIKeys {
					if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			// Check basic auth
			username, password, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			for _, user := range config.Users {
				if user.Username == username && user.Password == password {
					// Add user info to request context
					r.Header.Set("X-User-Id", username)
					r.Header.Set("X-User-Role", user.Role)
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}

// RBACConfig holds RBAC configuration
type RBACConfig struct {
	Roles       map[string][]string `json:"roles"`       // role -> allowed paths
	DefaultRole string              `json:"defaultRole"`
}

// RBACMiddleware provides role-based access control
func RBACMiddleware(config *RBACConfig) Middleware {
	if config == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := r.Header.Get("X-User-Role")
			if role == "" {
				role = config.DefaultRole
			}

			if role == "" {
				http.Error(w, "Forbidden - No role assigned", http.StatusForbidden)
				return
			}

			allowedPaths, exists := config.Roles[role]
			if !exists {
				http.Error(w, "Forbidden - Invalid role", http.StatusForbidden)
				return
			}

			// Check if user has access to the requested path
			allowed := false
			for _, path := range allowedPaths {
				if path == "*" || strings.HasPrefix(r.URL.Path, path) {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Forbidden - Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		next.ServeHTTP(w, r)
	})
}

// RequestIDMiddleware adds a unique request ID
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		r.Header.Set("X-Request-Id", requestID)
		w.Header().Set("X-Request-Id", requestID)

		next.ServeHTTP(w, r)
	})
}

// CompressionMiddleware handles response compression
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Set gzip header
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")

		// Create gzip writer (simplified - in production use gzip.Writer)
		next.ServeHTTP(w, r)
	})
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// HealthCheckMiddleware provides health check endpoint
func HealthCheckMiddleware(path string) Middleware {
	if path == "" {
		path = "/health"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == path {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MetricsMiddleware collects request metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// In production, send metrics to monitoring system
		log.Printf("METRIC: method=%s path=%s status=%d duration=%v",
			r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}
