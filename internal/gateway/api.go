package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// Gateway represents the unified gateway.
type Gateway struct {
	proxy  *ReverseProxy
	router *Router
	config *GatewayConfig
	server *http.Server
}

// GatewayConfig holds gateway configuration.
type GatewayConfig struct {
	ListenAddr     string           `json:"listenAddr"`
	TLSCertFile    string           `json:"tlsCertFile"`
	TLSKeyFile     string           `json:"tlsKeyFile"`
	ReadTimeout    time.Duration    `json:"readTimeout"`
	WriteTimeout   time.Duration    `json:"writeTimeout"`
	IdleTimeout    time.Duration    `json:"idleTimeout"`
	MaxHeaderBytes int              `json:"maxHeaderBytes"`
	CORS           *CORSConfig      `json:"cors"`
	RateLimit      *RateLimitConfig `json:"rateLimit"`
	Auth           *AuthConfig      `json:"auth"`
	RBAC           *RBACConfig      `json:"rbac"`
}

// DefaultGatewayConfig returns default gateway configuration.
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		ListenAddr:     ":8080",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
		CORS:           DefaultCORSConfig(),
		RateLimit:      DefaultRateLimitConfig(),
		Auth:           &AuthConfig{Enabled: false},
		RBAC:           &RBACConfig{Roles: make(map[string][]string)},
	}
}

// NewGateway creates a new gateway.
func NewGateway(config *GatewayConfig) *Gateway {
	if config == nil {
		config = DefaultGatewayConfig()
	}

	gw := &Gateway{
		proxy:  NewReverseProxy(nil),
		router: NewRouter(),
		config: config,
	}

	return gw
}

// GetProxy returns the reverse proxy.
func (gw *Gateway) GetProxy() *ReverseProxy {
	return gw.proxy
}

// GetRouter returns the router.
func (gw *Gateway) GetRouter() *Router {
	return gw.router
}

// ServeHTTP implements http.Handler.
func (gw *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Match route
	route := gw.router.MatchRoute(r)
	if route == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Add route headers
	for key, value := range route.Headers {
		r.Header.Set(key, value)
	}

	// Proxy the request
	if err := gw.proxy.ProxyRequest(w, r); err != nil {
		log.Printf("Proxy error: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
}

// Start starts the gateway server.
func (gw *Gateway) Start() error {
	// Build middleware chain
	chain := NewChain(
		RecoveryMiddleware,
		RequestIDMiddleware,
		LoggingMiddleware,
		MetricsMiddleware,
		SecurityHeadersMiddleware,
		HealthCheckMiddleware("/health"),
		CORSMiddleware(gw.config.CORS),
		RateLimitMiddleware(gw.config.RateLimit),
		BasicAuthMiddleware(gw.config.Auth),
		RBACMiddleware(gw.config.RBAC),
	)

	// Create router
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/v1/routes", gw.handleRoutes)
	mux.HandleFunc("/api/v1/routes/", gw.handleRouteByID)
	mux.HandleFunc("/api/v1/backends", gw.handleBackends)
	mux.HandleFunc("/api/v1/backends/", gw.handleBackendByID)
	mux.HandleFunc("/api/v1/status", gw.handleStatus)
	mux.HandleFunc("/api/v1/health", gw.handleHealth)
	mux.HandleFunc("/api/v1/metrics", gw.handleMetrics)

	// Gateway handler (proxy)
	mux.Handle("/", gw)

	// Apply middleware
	handler := chain.Then(mux)

	// Create server
	gw.server = &http.Server{
		Addr:           gw.config.ListenAddr,
		Handler:        handler,
		ReadTimeout:    gw.config.ReadTimeout,
		WriteTimeout:   gw.config.WriteTimeout,
		IdleTimeout:    gw.config.IdleTimeout,
		MaxHeaderBytes: gw.config.MaxHeaderBytes,
	}

	// Start health check
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go gw.proxy.HealthCheck(ctx, 30*time.Second, 5*time.Second)

	// Start server
	log.Printf("Starting gateway on %s", gw.config.ListenAddr)

	if gw.config.TLSCertFile != "" && gw.config.TLSKeyFile != "" {
		return gw.server.ListenAndServeTLS(gw.config.TLSCertFile, gw.config.TLSKeyFile)
	}

	return gw.server.ListenAndServe()
}

// Stop stops the gateway server.
func (gw *Gateway) Stop(ctx context.Context) error {
	if gw.server != nil {
		return gw.server.Shutdown(ctx)
	}
	return nil
}

// API response structures

// APIResponse represents a generic API response.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// RouteRequest represents a route creation/update request.
type RouteRequest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Domain      string            `json:"domain"`
	Path        string            `json:"path"`
	PathPattern string            `json:"pathPattern"`
	BackendURL  string            `json:"backendUrl"`
	Priority    int               `json:"priority"`
	Type        RouteType         `json:"type"`
	Headers     map[string]string `json:"headers"`
}

// BackendRequest represents a backend creation request.
type BackendRequest struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Weight int    `json:"weight"`
}

// handleRoutes handles /api/v1/routes.
func (gw *Gateway) handleRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		routes := gw.router.GetRoutes()
		gw.sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    routes,
		})

	case http.MethodPost:
		var req RouteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			gw.sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		route := &Route{
			ID:          req.ID,
			Name:        req.Name,
			Domain:      req.Domain,
			Path:        req.Path,
			PathPattern: req.PathPattern,
			BackendURL:  req.BackendURL,
			Priority:    req.Priority,
			Type:        req.Type,
			Headers:     req.Headers,
		}

		if err := gw.router.ValidateRoute(route); err != nil {
			gw.sendError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := gw.router.AddRoute(route); err != nil {
			gw.sendError(w, http.StatusInternalServerError, err.Error())
			return
		}

		gw.sendJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    route,
			Message: "Route created successfully",
		})

	default:
		gw.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleRouteByID handles /api/v1/routes/{id}.
func (gw *Gateway) handleRouteByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/routes/")
	if id == "" {
		gw.sendError(w, http.StatusBadRequest, "Route ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		route, err := gw.router.GetRoute(id)
		if err != nil {
			gw.sendError(w, http.StatusNotFound, err.Error())
			return
		}

		gw.sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    route,
		})

	case http.MethodPut:
		var req RouteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			gw.sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		route := &Route{
			ID:          id,
			Name:        req.Name,
			Domain:      req.Domain,
			Path:        req.Path,
			PathPattern: req.PathPattern,
			BackendURL:  req.BackendURL,
			Priority:    req.Priority,
			Type:        req.Type,
			Headers:     req.Headers,
		}

		if err := gw.router.UpdateRoute(route); err != nil {
			gw.sendError(w, http.StatusNotFound, err.Error())
			return
		}

		gw.sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    route,
			Message: "Route updated successfully",
		})

	case http.MethodDelete:
		if !gw.router.RemoveRoute(id) {
			gw.sendError(w, http.StatusNotFound, "Route not found")
			return
		}

		gw.sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Message: "Route deleted successfully",
		})

	default:
		gw.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleBackends handles /api/v1/backends.
func (gw *Gateway) handleBackends(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		backends := gw.proxy.GetBackends()
		gw.sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    backends,
		})

	case http.MethodPost:
		var req BackendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			gw.sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		backend := &Backend{
			ID:     req.ID,
			URL:    req.URL,
			Weight: req.Weight,
			Alive:  true,
		}

		gw.proxy.AddBackend(backend)

		gw.sendJSON(w, http.StatusCreated, APIResponse{
			Success: true,
			Data:    backend,
			Message: "Backend added successfully",
		})

	default:
		gw.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleBackendByID handles /api/v1/backends/{id}.
func (gw *Gateway) handleBackendByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/backends/")
	if id == "" {
		gw.sendError(w, http.StatusBadRequest, "Backend ID is required")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if !gw.proxy.RemoveBackend(id) {
			gw.sendError(w, http.StatusNotFound, "Backend not found")
			return
		}

		gw.sendJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Message: "Backend removed successfully",
		})

	default:
		gw.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleStatus handles /api/v1/status.
func (gw *Gateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gw.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status := map[string]interface{}{
		"status":    "running",
		"timestamp": time.Now().Format(time.RFC3339),
		"config": map[string]interface{}{
			"listenAddr": gw.config.ListenAddr,
			"tlsEnabled": gw.config.TLSCertFile != "",
		},
		"routes":   len(gw.router.GetRoutes()),
		"backends": len(gw.proxy.GetBackends()),
	}

	gw.sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    status,
	})
}

// handleHealth handles /api/v1/health.
func (gw *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gw.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"services":  gw.checkServices(),
	}

	gw.sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    health,
	})
}

// handleMetrics handles /api/v1/metrics.
func (gw *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gw.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	metrics := gw.proxy.GetMetrics()

	gw.sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    metrics,
	})
}

// checkServices checks the health of all backend services.
func (gw *Gateway) checkServices() map[string]string {
	backends := gw.proxy.GetBackends()
	services := make(map[string]string)

	for _, backend := range backends {
		if backend.IsAlive() {
			services[backend.ID] = "healthy"
		} else {
			services[backend.ID] = "unhealthy"
		}
	}

	return services
}

// sendJSON sends a JSON response.
func (gw *Gateway) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response.
func (gw *Gateway) sendError(w http.ResponseWriter, statusCode int, message string) {
	gw.sendJSON(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}

// EnableRoute enables a route.
func (gw *Gateway) EnableRoute(id string) error {
	return gw.router.EnableRoute(id)
}

// DisableRoute disables a route.
func (gw *Gateway) DisableRoute(id string) error {
	return gw.router.DisableRoute(id)
}

// GetRoute returns a route by ID.
func (gw *Gateway) GetRoute(id string) (*Route, error) {
	return gw.router.GetRoute(id)
}

// AddRoute adds a new route.
func (gw *Gateway) AddRoute(route *Route) error {
	return gw.router.AddRoute(route)
}

// RemoveRoute removes a route.
func (gw *Gateway) RemoveRoute(id string) bool {
	return gw.router.RemoveRoute(id)
}

// AddBackend adds a new backend.
func (gw *Gateway) AddBackend(backend *Backend) {
	gw.proxy.AddBackend(backend)
}

// RemoveBackend removes a backend.
func (gw *Gateway) RemoveBackend(id string) bool {
	return gw.proxy.RemoveBackend(id)
}

// GetStatus returns gateway status.
func (gw *Gateway) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"status":    "running",
		"timestamp": time.Now().Format(time.RFC3339),
		"routes":    len(gw.router.GetRoutes()),
		"backends":  len(gw.proxy.GetBackends()),
	}
}

// MatchRoute finds the matching route for a request.
func (gw *Gateway) MatchRoute(req *http.Request) *Route {
	return gw.router.MatchRoute(req)
}

// GatewayManager manages multiple gateways.
type GatewayManager struct {
	gateways map[string]*Gateway
}

// NewGatewayManager creates a new gateway manager.
func NewGatewayManager() *GatewayManager {
	return &GatewayManager{
		gateways: make(map[string]*Gateway),
	}
}

// AddGateway adds a gateway.
func (gm *GatewayManager) AddGateway(name string, gw *Gateway) {
	gm.gateways[name] = gw
}

// RemoveGateway removes a gateway.
func (gm *GatewayManager) RemoveGateway(name string) {
	delete(gm.gateways, name)
}

// GetGateway returns a gateway by name.
func (gm *GatewayManager) GetGateway(name string) (*Gateway, bool) {
	gw, ok := gm.gateways[name]
	return gw, ok
}

// StartAll starts all gateways.
func (gm *GatewayManager) StartAll() error {
	for name, gw := range gm.gateways {
		go func(n string, g *Gateway) {
			if err := g.Start(); err != nil {
				log.Printf("Gateway %s error: %v", n, err)
			}
		}(name, gw)
	}
	return nil
}

// StopAll stops all gateways.
func (gm *GatewayManager) StopAll(ctx context.Context) error {
	for _, gw := range gm.gateways {
		if err := gw.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// GetStatusAll returns status of all gateways.
func (gm *GatewayManager) GetStatusAll() map[string]interface{} {
	status := make(map[string]interface{})
	for name, gw := range gm.gateways {
		status[name] = gw.GetStatus()
	}
	return status
}

// LoadBalancerConfig holds load balancer configuration.
type LoadBalancerConfig struct {
	Algorithm   string `json:"algorithm"` // round-robin, least-connections, ip-hash
	HealthCheck struct {
		Enabled  bool          `json:"enabled"`
		Path     string        `json:"path"`
		Interval time.Duration `json:"interval"`
		Timeout  time.Duration `json:"timeout"`
	} `json:"healthCheck"`
}

// NewLoadBalancer creates a load balancer based on configuration.
func NewLoadBalancer(config *LoadBalancerConfig, backends []*Backend) *ReverseProxy {
	proxy := NewReverseProxy(nil)

	for _, backend := range backends {
		proxy.AddBackend(backend)
	}

	return proxy
}

// WebSocketConfig holds WebSocket configuration.
type WebSocketConfig struct {
	Enabled          bool          `json:"enabled"`
	ReadBufferSize   int           `json:"readBufferSize"`
	WriteBufferSize  int           `json:"writeBufferSize"`
	HandshakeTimeout time.Duration `json:"handshakeTimeout"`
	CheckOrigin      bool          `json:"checkOrigin"`
}

// DefaultWebSocketConfig returns default WebSocket configuration.
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		Enabled:          true,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin:      false,
	}
}

// WebSocketMiddleware handles WebSocket upgrades.
func WebSocketMiddleware(config *WebSocketConfig) Middleware {
	if config == nil {
		config = DefaultWebSocketConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check for WebSocket upgrade request
			if strings.EqualFold(r.Header.Get("Connection"), "upgrade") &&
				strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				// Handle WebSocket connection
				log.Printf("WebSocket connection from %s", r.RemoteAddr)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SSLConfig holds SSL/TLS configuration.
type SSLConfig struct {
	Enabled  bool     `json:"enabled"`
	CertFile string   `json:"certFile"`
	KeyFile  string   `json:"keyFile"`
	AutoCert bool     `json:"autoCert"` // automatic certificate management
	Domains  []string `json:"domains"`
}

// SSLManager manages SSL certificates.
type SSLManager struct {
	config *SSLConfig
}

// NewSSLManager creates a new SSL manager.
func NewSSLManager(config *SSLConfig) *SSLManager {
	return &SSLManager{
		config: config,
	}
}

// IsEnabled returns whether SSL is enabled.
func (sm *SSLManager) IsEnabled() bool {
	return sm.config.Enabled
}

// GetCertFile returns the certificate file path.
func (sm *SSLManager) GetCertFile() string {
	return sm.config.CertFile
}

// GetKeyFile returns the key file path.
func (sm *SSLManager) GetKeyFile() string {
	return sm.config.KeyFile
}

// GetDomains returns the configured domains.
func (sm *SSLManager) GetDomains() []string {
	return sm.config.Domains
}

// Certificate represents an SSL certificate.
type Certificate struct {
	Domain    string    `json:"domain"`
	CertFile  string    `json:"certFile"`
	KeyFile   string    `json:"keyFile"`
	ExpiresAt time.Time `json:"expiresAt"`
	Issuer    string    `json:"issuer"`
}

// CertificateManager manages SSL certificates.
type CertificateManager struct {
	certificates map[string]*Certificate
}

// NewCertificateManager creates a new certificate manager.
func NewCertificateManager() *CertificateManager {
	return &CertificateManager{
		certificates: make(map[string]*Certificate),
	}
}

// AddCertificate adds a certificate.
func (cm *CertificateManager) AddCertificate(cert *Certificate) {
	cm.certificates[cert.Domain] = cert
}

// GetCertificate returns a certificate for a domain.
func (cm *CertificateManager) GetCertificate(domain string) (*Certificate, bool) {
	cert, ok := cm.certificates[domain]
	return cert, ok
}

// RemoveCertificate removes a certificate.
func (cm *CertificateManager) RemoveCertificate(domain string) {
	delete(cm.certificates, domain)
}

// ListCertificates returns all certificates.
func (cm *CertificateManager) ListCertificates() []*Certificate {
	certs := make([]*Certificate, 0, len(cm.certificates))
	for _, cert := range cm.certificates {
		certs = append(certs, cert)
	}
	return certs
}

// AccessLogEntry represents an access log entry.
type AccessLogEntry struct {
	Timestamp  time.Time     `json:"timestamp"`
	RemoteAddr string        `json:"remoteAddr"`
	Method     string        `json:"method"`
	Path       string        `json:"path"`
	StatusCode int           `json:"statusCode"`
	Duration   time.Duration `json:"duration"`
	UserAgent  string        `json:"userAgent"`
	RequestID  string        `json:"requestId"`
}

// AccessLogger logs access requests.
type AccessLogger struct {
	entries []AccessLogEntry
}

// NewAccessLogger creates a new access logger.
func NewAccessLogger() *AccessLogger {
	return &AccessLogger{
		entries: make([]AccessLogEntry, 0),
	}
}

// Log logs an access entry.
func (al *AccessLogger) Log(entry AccessLogEntry) {
	al.entries = append(al.entries, entry)
}

// GetEntries returns all logged entries.
func (al *AccessLogger) GetEntries() []AccessLogEntry {
	return al.entries
}

// FilterEntries filters entries by criteria.
func (al *AccessLogger) FilterEntries(method string, path string, statusCode int) []AccessLogEntry {
	var filtered []AccessLogEntry
	for _, entry := range al.entries {
		if method != "" && entry.Method != method {
			continue
		}
		if path != "" && !strings.HasPrefix(entry.Path, path) {
			continue
		}
		if statusCode != 0 && entry.StatusCode != statusCode {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// ClearEntries clears all logged entries.
func (al *AccessLogger) ClearEntries() {
	al.entries = make([]AccessLogEntry, 0)
}

// AccessLogMiddleware logs access requests.
func AccessLogMiddleware(logger *AccessLogger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			entry := AccessLogEntry{
				Timestamp:  time.Now(),
				RemoteAddr: r.RemoteAddr,
				Method:     r.Method,
				Path:       r.URL.Path,
				StatusCode: wrapped.statusCode,
				Duration:   time.Since(start),
				UserAgent:  r.UserAgent(),
				RequestID:  r.Header.Get("X-Request-Id"),
			}

			logger.Log(entry)
		})
	}
}
