package unifiedgateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AppRoute represents an application route configuration
type AppRoute struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	PathPrefix  string    `json:"path_prefix"`
	TargetURL   string    `json:"target_url"`
	WebSocket   bool      `json:"websocket"`
	Auth        bool      `json:"auth"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GatewayConfig represents the unified gateway configuration
type GatewayConfig struct {
	Domain      string            `json:"domain"`
	Routes      map[string]*AppRoute `json:"routes"`
	TLSEnabled  bool              `json:"tls_enabled"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Gateway handles unified gateway routing
type Gateway struct {
	mu     sync.RWMutex
	config GatewayConfig
}

// NewGateway creates a new unified gateway
func NewGateway(domain string) *Gateway {
	return &Gateway{
		config: GatewayConfig{
			Domain:     domain,
			Routes:     make(map[string]*AppRoute),
			TLSEnabled: false,
			UpdatedAt:  time.Now(),
		},
	}
}

// AddRoute adds a new application route
func (g *Gateway) AddRoute(route AppRoute) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	route.CreatedAt = time.Now()
	route.UpdatedAt = time.Now()
	route.Enabled = true
	g.config.Routes[route.ID] = &route
	g.config.UpdatedAt = time.Now()
	return nil
}

// RemoveRoute removes an application route
func (g *Gateway) RemoveRoute(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.config.Routes[id]; !exists {
		return fmt.Errorf("route not found: %s", id)
	}
	delete(g.config.Routes, id)
	g.config.UpdatedAt = time.Now()
	return nil
}

// UpdateRoute updates an existing route
func (g *Gateway) UpdateRoute(id string, updates AppRoute) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	route, exists := g.config.Routes[id]
	if !exists {
		return fmt.Errorf("route not found: %s", id)
	}

	if updates.Name != "" {
		route.Name = updates.Name
	}
	if updates.TargetURL != "" {
		route.TargetURL = updates.TargetURL
	}
	if updates.PathPrefix != "" {
		route.PathPrefix = updates.PathPrefix
	}
	route.WebSocket = updates.WebSocket
	route.Auth = updates.Auth
	route.Enabled = updates.Enabled
	route.UpdatedAt = time.Now()
	g.config.UpdatedAt = time.Now()
	return nil
}

// GetRoute returns a route by ID
func (g *Gateway) GetRoute(id string) (*AppRoute, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	route, exists := g.config.Routes[id]
	if !exists {
		return nil, fmt.Errorf("route not found: %s", id)
	}
	return route, nil
}

// ListRoutes returns all routes
func (g *Gateway) ListRoutes() []*AppRoute {
	g.mu.RLock()
	defer g.mu.RUnlock()

	routes := make([]*AppRoute, 0, len(g.config.Routes))
	for _, route := range g.config.Routes {
		routes = append(routes, route)
	}
	return routes
}

// ServeHTTP implements http.Handler for the gateway
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	host := r.Host
	path := r.URL.Path

	// Find matching route
	for _, route := range g.config.Routes {
		if !route.Enabled {
			continue
		}

		// Check domain match
		if route.Domain != "" && route.Domain != host {
			continue
		}

		// Check path prefix match
		if route.PathPrefix != "" && !strings.HasPrefix(path, route.PathPrefix) {
			continue
		}

		// Route matched, proxy request
		g.proxyRequest(w, r, route)
		return
	}

	http.NotFound(w, r)
}

func (g *Gateway) proxyRequest(w http.ResponseWriter, r *http.Request, route *AppRoute) {
	target, err := url.Parse(route.TargetURL)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		// Strip path prefix if configured
		if route.PathPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, route.PathPrefix)
		}

		// Add WebSocket headers if needed
		if route.WebSocket {
			req.Header.Set("X-Forwarded-Host", r.Host)
			req.Header.Set("X-Forwarded-Proto", "http")
			if g.config.TLSEnabled {
				req.Header.Set("X-Forwarded-Proto", "https")
			}
		}
	}

	proxy.ServeHTTP(w, r)
}

// RegisterRoutes registers gateway management HTTP routes
func (g *Gateway) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/gateway/routes", g.handleRoutes)
	mux.HandleFunc("/api/gateway/routes/add", g.handleAddRoute)
	mux.HandleFunc("/api/gateway/routes/remove", g.handleRemoveRoute)
	mux.HandleFunc("/api/gateway/routes/update", g.handleUpdateRoute)
}

func (g *Gateway) handleRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	routes := g.ListRoutes()
	json.NewEncoder(w).Encode(routes)
}

func (g *Gateway) handleAddRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var route AppRoute
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := g.AddRoute(route); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleRemoveRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := g.RemoveRoute(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      string    `json:"id"`
		Updates AppRoute  `json:"updates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := g.UpdateRoute(req.ID, req.Updates); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
