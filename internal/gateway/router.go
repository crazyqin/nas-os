package gateway

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// RouteType represents the type of route.
type RouteType string

const (
	RouteTypeDomain RouteType = "domain"
	RouteTypePath   RouteType = "path"
	RouteTypeBoth   RouteType = "both"
)

// Route represents a routing rule.
type Route struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Domain      string            `json:"domain"`
	Path        string            `json:"path"`
	PathPattern string            `json:"pathPattern"` // regex pattern
	BackendURL  string            `json:"backendUrl"`
	Priority    int               `json:"priority"`
	Type        RouteType         `json:"type"`
	Enabled     bool              `json:"enabled"`
	Headers     map[string]string `json:"headers"`
	pathRegex   *regexp.Regexp
}

// Router represents the routing engine.
type Router struct {
	routes []*Route
	mu     sync.RWMutex
}

// NewRouter creates a new router.
func NewRouter() *Router {
	return &Router{
		routes: make([]*Route, 0),
	}
}

// AddRoute adds a routing rule.
func (r *Router) AddRoute(route *Route) error {
	if route.Domain == "" && route.Path == "" && route.PathPattern == "" {
		return fmt.Errorf("route must have at least a domain, path, or path pattern")
	}

	// Compile path regex if provided
	if route.PathPattern != "" {
		regex, err := regexp.Compile(route.PathPattern)
		if err != nil {
			return fmt.Errorf("invalid path pattern: %v", err)
		}
		route.pathRegex = regex
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate ID
	for _, existing := range r.routes {
		if existing.ID == route.ID {
			return fmt.Errorf("route with ID %s already exists", route.ID)
		}
	}

	// Set defaults
	if route.Type == "" {
		if route.Domain != "" && route.Path != "" {
			route.Type = RouteTypeBoth
		} else if route.Domain != "" {
			route.Type = RouteTypeDomain
		} else {
			route.Type = RouteTypePath
		}
	}

	route.Enabled = true
	r.routes = append(r.routes, route)

	// Sort by priority (higher priority first)
	r.sortRoutes()

	return nil
}

// RemoveRoute removes a routing rule by ID.
func (r *Router) RemoveRoute(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, route := range r.routes {
		if route.ID == id {
			r.routes = append(r.routes[:i], r.routes[i+1:]...)
			return true
		}
	}
	return false
}

// UpdateRoute updates a routing rule.
func (r *Router) UpdateRoute(route *Route) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.routes {
		if existing.ID == route.ID {
			// Compile path regex if provided
			if route.PathPattern != "" {
				regex, err := regexp.Compile(route.PathPattern)
				if err != nil {
					return fmt.Errorf("invalid path pattern: %v", err)
				}
				route.pathRegex = regex
			}

			// Set type if empty
			if route.Type == "" {
				if route.Domain != "" && route.Path != "" {
					route.Type = RouteTypeBoth
				} else if route.Domain != "" {
					route.Type = RouteTypeDomain
				} else {
					route.Type = RouteTypePath
				}
			}

			r.routes[i] = route
			r.sortRoutes()
			return nil
		}
	}

	return fmt.Errorf("route with ID %s not found", route.ID)
}

// GetRoute returns a route by ID.
func (r *Router) GetRoute(id string) (*Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, route := range r.routes {
		if route.ID == id {
			return route, nil
		}
	}

	return nil, fmt.Errorf("route with ID %s not found", id)
}

// GetRoutes returns all routes.
func (r *Router) GetRoutes() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*Route, len(r.routes))
	copy(routes, r.routes)
	return routes
}

// MatchRoute finds the matching route for the given request.
func (r *Router) MatchRoute(req *http.Request) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	host := req.Host
	// Remove port from host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	path := req.URL.Path

	for _, route := range r.routes {
		if !route.Enabled {
			continue
		}

		if r.matchRoute(route, host, path) {
			return route
		}
	}

	return nil
}

// matchRoute checks if a route matches the given host and path.
func (r *Router) matchRoute(route *Route, host, path string) bool {
	switch route.Type {
	case RouteTypeDomain:
		return r.matchDomain(route, host)
	case RouteTypePath:
		return r.matchPath(route, path)
	case RouteTypeBoth:
		return r.matchDomain(route, host) && r.matchPath(route, path)
	}
	return false
}

// matchDomain checks if the domain matches.
func (r *Router) matchDomain(route *Route, host string) bool {
	if route.Domain == "" {
		return true
	}

	// Support wildcard domains (e.g., *.example.com)
	if strings.HasPrefix(route.Domain, "*.") {
		suffix := route.Domain[1:]
		return strings.HasSuffix(host, suffix)
	}

	return host == route.Domain
}

// matchPath checks if the path matches.
func (r *Router) matchPath(route *Route, path string) bool {
	if route.Path == "" && route.pathRegex == nil {
		return true
	}

	// Check exact path match
	if route.Path != "" && path == route.Path {
		return true
	}

	// Check prefix path match
	if route.Path != "" && strings.HasPrefix(path, route.Path) {
		return true
	}

	// Check regex pattern match
	if route.pathRegex != nil {
		return route.pathRegex.MatchString(path)
	}

	return false
}

// sortRoutes sorts routes by priority (higher priority first).
func (r *Router) sortRoutes() {
	for i := 0; i < len(r.routes)-1; i++ {
		for j := 0; j < len(r.routes)-i-1; j++ {
			if r.routes[j].Priority < r.routes[j+1].Priority {
				r.routes[j], r.routes[j+1] = r.routes[j+1], r.routes[j]
			}
		}
	}
}

// EnableRoute enables a route.
func (r *Router) EnableRoute(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, route := range r.routes {
		if route.ID == id {
			route.Enabled = true
			return nil
		}
	}

	return fmt.Errorf("route with ID %s not found", id)
}

// DisableRoute disables a route.
func (r *Router) DisableRoute(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, route := range r.routes {
		if route.ID == id {
			route.Enabled = false
			return nil
		}
	}

	return fmt.Errorf("route with ID %s not found", id)
}

// GetRoutesByDomain returns routes for a specific domain.
func (r *Router) GetRoutesByDomain(domain string) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var routes []*Route
	for _, route := range r.routes {
		if route.Domain == domain || (strings.HasPrefix(route.Domain, "*.") && strings.HasSuffix(domain, route.Domain[1:])) {
			routes = append(routes, route)
		}
	}
	return routes
}

// GetRoutesByPath returns routes for a specific path prefix.
func (r *Router) GetRoutesByPath(path string) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var routes []*Route
	for _, route := range r.routes {
		if strings.HasPrefix(path, route.Path) {
			routes = append(routes, route)
		}
	}
	return routes
}

// ValidateRoute validates a route configuration.
func (r *Router) ValidateRoute(route *Route) error {
	if route.ID == "" {
		return fmt.Errorf("route ID is required")
	}

	if route.Domain == "" && route.Path == "" {
		return fmt.Errorf("route must have at least a domain or path")
	}

	if route.BackendURL == "" {
		return fmt.Errorf("backend URL is required")
	}

	// Validate path pattern if provided
	if route.PathPattern != "" {
		_, err := regexp.Compile(route.PathPattern)
		if err != nil {
			return fmt.Errorf("invalid path pattern: %v", err)
		}
	}

	return nil
}
