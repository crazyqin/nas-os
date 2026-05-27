package reverseproxy

import (
	"time"
)

// ReverseProxy represents a reverse proxy configuration
type ReverseProxy struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Domain     string            `json:"domain"`
	TargetURL  string            `json:"target_url"`
	SSLEnabled bool              `json:"ssl_enabled"`
	CertID     string            `json:"cert_id,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Status     string            `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// ProxyRule represents a routing rule for a reverse proxy
type ProxyRule struct {
	ID            string   `json:"id"`
	Path          string   `json:"path"`
	TargetURL     string   `json:"target_url"`
	LoadBalancing string   `json:"load_balancing,omitempty"` // round-robin, least-connections, ip-hash
	HealthCheck   string   `json:"health_check,omitempty"`
	RateLimit     int      `json:"rate_limit,omitempty"` // requests per second
	IPWhitelist   []string `json:"ip_whitelist,omitempty"`
}

// CreateProxyRequest represents a request to create a new reverse proxy
type CreateProxyRequest struct {
	Name       string            `json:"name" validate:"required"`
	Domain     string            `json:"domain" validate:"required"`
	TargetURL  string            `json:"target_url" validate:"required"`
	SSLEnabled bool              `json:"ssl_enabled,omitempty"`
	CertID     string            `json:"cert_id,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// UpdateProxyRequest represents a request to update a reverse proxy
type UpdateProxyRequest struct {
	Name       *string            `json:"name,omitempty"`
	Domain     *string            `json:"domain,omitempty"`
	TargetURL  *string            `json:"target_url,omitempty"`
	SSLEnabled *bool              `json:"ssl_enabled,omitempty"`
	CertID     *string            `json:"cert_id,omitempty"`
	Headers    *map[string]string `json:"headers,omitempty"`
	Status     *string            `json:"status,omitempty"`
}

// AddRuleRequest represents a request to add a routing rule
type AddRuleRequest struct {
	Path          string   `json:"path" validate:"required"`
	TargetURL     string   `json:"target_url" validate:"required"`
	LoadBalancing string   `json:"load_balancing,omitempty"`
	HealthCheck   string   `json:"health_check,omitempty"`
	RateLimit     int      `json:"rate_limit,omitempty"`
	IPWhitelist   []string `json:"ip_whitelist,omitempty"`
}

// ProxyStats represents aggregated reverse proxy statistics
type ProxyStats struct {
	TotalProxies    int     `json:"total_proxies"`
	ActiveProxies   int     `json:"active_proxies"`
	TotalRequests   int64   `json:"total_requests"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
	ErrorRate       float64 `json:"error_rate"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a generic API success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}
