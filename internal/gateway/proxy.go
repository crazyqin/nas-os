package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Backend represents a backend server
type Backend struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Weight   int    `json:"weight"`
	Alive    bool   `json:"alive"`
	mu       sync.RWMutex
}

// SetAlive sets the alive status of the backend
func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	b.Alive = alive
	b.mu.Unlock()
}

// IsAlive returns the alive status of the backend
func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Alive
}

// ReverseProxy represents the reverse proxy core
type ReverseProxy struct {
	backends    []*Backend
	currentIdx  uint64
	mu          sync.RWMutex
	limiter     *rate.Limiter
	proxyConfig *ProxyConfig
}

// ProxyConfig holds proxy configuration
type ProxyConfig struct {
	MaxIdleConns        int           `json:"maxIdleConns"`
	MaxIdleConnsPerHost int           `json:"maxIdleConnsPerHost"`
	IdleConnTimeout     time.Duration `json:"idleConnTimeout"`
	RequestTimeout      time.Duration `json:"requestTimeout"`
}

// DefaultProxyConfig returns default proxy configuration
func DefaultProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		RequestTimeout:      30 * time.Second,
	}
}

// NewReverseProxy creates a new reverse proxy
func NewReverseProxy(config *ProxyConfig) *ReverseProxy {
	if config == nil {
		config = DefaultProxyConfig()
	}
	return &ReverseProxy{
		backends:    make([]*Backend, 0),
		proxyConfig: config,
	}
}

// AddBackend adds a backend server
func (rp *ReverseProxy) AddBackend(backend *Backend) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.backends = append(rp.backends, backend)
}

// RemoveBackend removes a backend server by ID
func (rp *ReverseProxy) RemoveBackend(id string) bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	for i, b := range rp.backends {
		if b.ID == id {
			rp.backends = append(rp.backends[:i], rp.backends[i+1:]...)
			return true
		}
	}
	return false
}

// GetBackends returns all backends
func (rp *ReverseProxy) GetBackends() []*Backend {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	backends := make([]*Backend, len(rp.backends))
	copy(backends, rp.backends)
	return backends
}

// NextBackend returns the next alive backend using round-robin
func (rp *ReverseProxy) NextBackend() (*Backend, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if len(rp.backends) == 0 {
		return nil, fmt.Errorf("no backends available")
	}

	// Try to find an alive backend
	for i := 0; i < len(rp.backends); i++ {
		idx := rp.currentIdx % uint64(len(rp.backends))
		rp.currentIdx++
		if rp.backends[idx].IsAlive() {
			return rp.backends[idx], nil
		}
	}

	return nil, fmt.Errorf("no alive backends available")
}

// ProxyRequest proxies the request to the selected backend
func (rp *ReverseProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) error {
	backend, err := rp.NextBackend()
	if err != nil {
		return err
	}

	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		return fmt.Errorf("invalid backend URL: %v", err)
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Configure transport
	proxy.Transport = &http.Transport{
		MaxIdleConns:        rp.proxyConfig.MaxIdleConns,
		MaxIdleConnsPerHost: rp.proxyConfig.MaxIdleConnsPerHost,
		IdleConnTimeout:     rp.proxyConfig.IdleConnTimeout,
	}

	// Set request timeout
	ctx, cancel := context.WithTimeout(r.Context(), rp.proxyConfig.RequestTimeout)
	defer cancel()
	r = r.WithContext(ctx)

	// Modify request
	r.URL.Host = targetURL.Host
	r.URL.Scheme = targetURL.Scheme
	r.Host = targetURL.Host

	// Add proxy headers
	r.Header.Set("X-Forwarded-For", r.RemoteAddr)
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Forwarded-Proto", r.URL.Scheme)

	// Proxy the request
	proxy.ServeHTTP(w, r)

	return nil
}

// HealthCheck performs health checks on all backends
func (rp *ReverseProxy) HealthCheck(ctx context.Context, interval time.Duration, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	client := &http.Client{Timeout: timeout}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rp.mu.RLock()
			backends := make([]*Backend, len(rp.backends))
			copy(backends, rp.backends)
			rp.mu.RUnlock()

			for _, backend := range backends {
				go func(b *Backend) {
					resp, err := client.Get(b.URL + "/health")
					if err != nil {
						b.SetAlive(false)
						return
					}
					resp.Body.Close()
					b.SetAlive(resp.StatusCode == http.StatusOK)
				}(backend)
			}
		}
	}
}

// ProxyMetrics contains proxy metrics
type ProxyMetrics struct {
	TotalRequests   int64 `json:"totalRequests"`
	SuccessRequests int64 `json:"successRequests"`
	FailedRequests  int64 `json:"failedRequests"`
	AvgResponseTime int64 `json:"avgResponseTime"`
}

// GetMetrics returns proxy metrics (placeholder implementation)
func (rp *ReverseProxy) GetMetrics() *ProxyMetrics {
	return &ProxyMetrics{
		TotalRequests:   0,
		SuccessRequests: 0,
		FailedRequests:  0,
		AvgResponseTime: 0,
	}
}
