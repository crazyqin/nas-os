package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestReverseProxy tests the reverse proxy functionality.
func TestReverseProxy(t *testing.T) {
	t.Run("NewReverseProxy", func(t *testing.T) {
		proxy := NewReverseProxy(nil)
		if proxy == nil {
			t.Fatal("Expected proxy to be created")
		}
		if len(proxy.GetBackends()) != 0 {
			t.Fatal("Expected empty backends")
		}
	})

	t.Run("AddBackend", func(t *testing.T) {
		proxy := NewReverseProxy(nil)
		backend := &Backend{
			ID:     "backend1",
			URL:    "http://localhost:8081",
			Weight: 1,
			Alive:  true,
		}

		proxy.AddBackend(backend)
		backends := proxy.GetBackends()

		if len(backends) != 1 {
			t.Fatalf("Expected 1 backend, got %d", len(backends))
		}
		if backends[0].ID != "backend1" {
			t.Fatalf("Expected backend ID 'backend1', got '%s'", backends[0].ID)
		}
	})

	t.Run("RemoveBackend", func(t *testing.T) {
		proxy := NewReverseProxy(nil)
		backend := &Backend{
			ID:     "backend1",
			URL:    "http://localhost:8081",
			Weight: 1,
			Alive:  true,
		}

		proxy.AddBackend(backend)
		removed := proxy.RemoveBackend("backend1")

		if !removed {
			t.Fatal("Expected backend to be removed")
		}
		if len(proxy.GetBackends()) != 0 {
			t.Fatal("Expected empty backends")
		}
	})

	t.Run("NextBackend", func(t *testing.T) {
		proxy := NewReverseProxy(nil)

		_, err := proxy.NextBackend()
		if err == nil {
			t.Fatal("Expected error when no backends")
		}

		backend1 := &Backend{ID: "b1", URL: "http://localhost:8081", Alive: true}
		backend2 := &Backend{ID: "b2", URL: "http://localhost:8082", Alive: true}
		proxy.AddBackend(backend1)
		proxy.AddBackend(backend2)

		// Round-robin should alternate
		b1, _ := proxy.NextBackend()
		b2, _ := proxy.NextBackend()
		if b1.ID == b2.ID {
			t.Fatal("Expected different backends")
		}
	})

	t.Run("BackendAlive", func(t *testing.T) {
		backend := &Backend{ID: "b1", URL: "http://localhost:8081", Alive: true}
		if !backend.IsAlive() {
			t.Fatal("Expected backend to be alive")
		}

		backend.SetAlive(false)
		if backend.IsAlive() {
			t.Fatal("Expected backend to be dead")
		}
	})
}

// TestRouter tests the router functionality.
func TestRouter(t *testing.T) {
	t.Run("NewRouter", func(t *testing.T) {
		router := NewRouter()
		if router == nil {
			t.Fatal("Expected router to be created")
		}
		if len(router.GetRoutes()) != 0 {
			t.Fatal("Expected empty routes")
		}
	})

	t.Run("AddRoute", func(t *testing.T) {
		router := NewRouter()
		route := &Route{
			ID:         "route1",
			Name:       "Test Route",
			Domain:     "example.com",
			Path:       "/api",
			BackendURL: "http://localhost:8081",
			Priority:   1,
		}

		err := router.AddRoute(route)
		if err != nil {
			t.Fatalf("Failed to add route: %v", err)
		}

		routes := router.GetRoutes()
		if len(routes) != 1 {
			t.Fatalf("Expected 1 route, got %d", len(routes))
		}
	})

	t.Run("RemoveRoute", func(t *testing.T) {
		router := NewRouter()
		route := &Route{
			ID:         "route1",
			Name:       "Test Route",
			Domain:     "example.com",
			BackendURL: "http://localhost:8081",
			Priority:   1,
		}

		router.AddRoute(route)
		removed := router.RemoveRoute("route1")

		if !removed {
			t.Fatal("Expected route to be removed")
		}
		if len(router.GetRoutes()) != 0 {
			t.Fatal("Expected empty routes")
		}
	})

	t.Run("MatchRoute", func(t *testing.T) {
		router := NewRouter()

		route1 := &Route{
			ID:         "route1",
			Name:       "API Route",
			Domain:     "api.example.com",
			Path:       "/",
			BackendURL: "http://localhost:8081",
			Priority:   1,
			Enabled:    true,
		}
		route2 := &Route{
			ID:         "route2",
			Name:       "Web Route",
			Domain:     "www.example.com",
			Path:       "/",
			BackendURL: "http://localhost:8082",
			Priority:   1,
			Enabled:    true,
		}

		router.AddRoute(route1)
		router.AddRoute(route2)

		// Test domain matching
		req := httptest.NewRequest("GET", "http://api.example.com/test", nil)
		matched := router.MatchRoute(req)
		if matched == nil || matched.ID != "route1" {
			t.Fatal("Expected to match route1")
		}

		req = httptest.NewRequest("GET", "http://www.example.com/test", nil)
		matched = router.MatchRoute(req)
		if matched == nil || matched.ID != "route2" {
			t.Fatal("Expected to match route2")
		}
	})

	t.Run("MatchPathPattern", func(t *testing.T) {
		router := NewRouter()

		route := &Route{
			ID:          "route1",
			Name:        "API Route",
			Path:        "",
			PathPattern: "^/api/v[0-9]+/.*",
			BackendURL:  "http://localhost:8081",
			Priority:    1,
			Enabled:     true,
		}

		router.AddRoute(route)

		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		req.Host = "example.com"
		matched := router.MatchRoute(req)
		if matched == nil {
			t.Fatal("Expected to match route with pattern")
		}
	})

	t.Run("ValidateRoute", func(t *testing.T) {
		router := NewRouter()

		// Missing ID
		route := &Route{
			Domain:     "example.com",
			BackendURL: "http://localhost:8081",
		}
		err := router.ValidateRoute(route)
		if err == nil {
			t.Fatal("Expected validation error for missing ID")
		}

		// Missing backend URL
		route = &Route{
			ID:     "route1",
			Domain: "example.com",
		}
		err = router.ValidateRoute(route)
		if err == nil {
			t.Fatal("Expected validation error for missing backend URL")
		}

		// Invalid path pattern
		route = &Route{
			ID:          "route1",
			Domain:      "example.com",
			BackendURL:  "http://localhost:8081",
			PathPattern: "[invalid",
		}
		err = router.ValidateRoute(route)
		if err == nil {
			t.Fatal("Expected validation error for invalid path pattern")
		}
	})

	t.Run("EnableDisableRoute", func(t *testing.T) {
		router := NewRouter()
		route := &Route{
			ID:         "route1",
			Name:       "Test Route",
			Domain:     "example.com",
			BackendURL: "http://localhost:8081",
			Priority:   1,
			Enabled:    true,
		}

		router.AddRoute(route)

		err := router.DisableRoute("route1")
		if err != nil {
			t.Fatalf("Failed to disable route: %v", err)
		}

		r, _ := router.GetRoute("route1")
		if r.Enabled {
			t.Fatal("Expected route to be disabled")
		}

		err = router.EnableRoute("route1")
		if err != nil {
			t.Fatalf("Failed to enable route: %v", err)
		}

		r, _ = router.GetRoute("route1")
		if !r.Enabled {
			t.Fatal("Expected route to be enabled")
		}
	})
}

// TestMiddleware tests the middleware functionality.
func TestMiddleware(t *testing.T) {
	t.Run("LoggingMiddleware", func(t *testing.T) {
		handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("CORSMiddleware", func(t *testing.T) {
		config := &CORSConfig{
			AllowOrigins: []string{"http://localhost:3000"},
			AllowMethods: []string{"GET", "POST"},
			AllowHeaders: []string{"Content-Type"},
		}

		handler := CORSMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test preflight
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Fatal("Expected CORS origin header")
		}
	})

	t.Run("RateLimitMiddleware", func(t *testing.T) {
		config := &RateLimitConfig{
			Rate:  1,
			Burst: 1,
			ByIP:  true,
		}

		handler := RateLimitMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// First request should succeed
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected first request to succeed, got %d", w.Code)
		}

		// Second request should be rate limited
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("Expected second request to be rate limited, got %d", w.Code)
		}
	})

	t.Run("SecurityHeadersMiddleware", func(t *testing.T) {
		handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("Expected security header")
		}
		if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
			t.Fatal("Expected security header")
		}
	})

	t.Run("RequestIDMiddleware", func(t *testing.T) {
		handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("X-Request-Id") == "" {
			t.Fatal("Expected request ID header")
		}
	})

	t.Run("RecoveryMiddleware", func(t *testing.T) {
		handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("Expected status 500, got %d", w.Code)
		}
	})

	t.Run("HealthCheckMiddleware", func(t *testing.T) {
		handler := HealthCheckMiddleware("/health")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test health endpoint
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		if w.Header().Get("Content-Type") != "application/json" {
			t.Fatal("Expected JSON content type")
		}
	})

	t.Run("MiddlewareChain", func(t *testing.T) {
		var order []string

		chain := NewChain(
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					order = append(order, "first")
					next.ServeHTTP(w, r)
				})
			},
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					order = append(order, "second")
					next.ServeHTTP(w, r)
				})
			},
		)

		handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "handler")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if len(order) != 3 {
			t.Fatalf("Expected 3 elements in order, got %d", len(order))
		}
		if order[0] != "first" || order[1] != "second" || order[2] != "handler" {
			t.Fatal("Expected middleware order: first, second, handler")
		}
	})
}

// TestGatewayAPI tests the API endpoints.
func TestGatewayAPI(t *testing.T) {
	t.Run("HandleRoutes", func(t *testing.T) {
		gw := NewGateway(nil)

		// Test GET routes
		req := httptest.NewRequest("GET", "/api/v1/routes", nil)
		w := httptest.NewRecorder()

		gw.handleRoutes(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var resp APIResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if !resp.Success {
			t.Fatal("Expected success response")
		}
	})

	t.Run("HandleRoutesPost", func(t *testing.T) {
		gw := NewGateway(nil)

		routeReq := RouteRequest{
			ID:         "test-route",
			Name:       "Test Route",
			Domain:     "example.com",
			BackendURL: "http://localhost:8081",
			Priority:   1,
		}

		body, _ := json.Marshal(routeReq)
		req := httptest.NewRequest("POST", "/api/v1/routes", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		gw.handleRoutes(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("Expected status 201, got %d", w.Code)
		}

		var resp APIResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if !resp.Success {
			t.Fatal("Expected success response")
		}
	})

	t.Run("HandleRouteByID", func(t *testing.T) {
		gw := NewGateway(nil)

		// Add a route first
		route := &Route{
			ID:         "test-route",
			Name:       "Test Route",
			Domain:     "example.com",
			BackendURL: "http://localhost:8081",
			Priority:   1,
		}
		gw.router.AddRoute(route)

		// Test GET
		req := httptest.NewRequest("GET", "/api/v1/routes/test-route", nil)
		w := httptest.NewRecorder()

		gw.handleRouteByID(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		// Test DELETE
		req = httptest.NewRequest("DELETE", "/api/v1/routes/test-route", nil)
		w = httptest.NewRecorder()

		gw.handleRouteByID(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("HandleBackends", func(t *testing.T) {
		gw := NewGateway(nil)

		// Test GET backends
		req := httptest.NewRequest("GET", "/api/v1/backends", nil)
		w := httptest.NewRecorder()

		gw.handleBackends(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		// Test POST backend
		backendReq := BackendRequest{
			ID:  "test-backend",
			URL: "http://localhost:8081",
		}

		body, _ := json.Marshal(backendReq)
		req = httptest.NewRequest("POST", "/api/v1/backends", bytes.NewBuffer(body))
		w = httptest.NewRecorder()

		gw.handleBackends(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("Expected status 201, got %d", w.Code)
		}
	})

	t.Run("HandleStatus", func(t *testing.T) {
		gw := NewGateway(nil)

		req := httptest.NewRequest("GET", "/api/v1/status", nil)
		w := httptest.NewRecorder()

		gw.handleStatus(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		var resp APIResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if !resp.Success {
			t.Fatal("Expected success response")
		}
	})

	t.Run("HandleHealth", func(t *testing.T) {
		gw := NewGateway(nil)

		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()

		gw.handleHealth(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})
}

// TestGateway tests the gateway functionality.
func TestGateway(t *testing.T) {
	t.Run("NewGateway", func(t *testing.T) {
		gw := NewGateway(nil)
		if gw == nil {
			t.Fatal("Expected gateway to be created")
		}
	})

	t.Run("GatewayServeHTTP", func(t *testing.T) {
		gw := NewGateway(nil)

		// Add a route
		route := &Route{
			ID:         "test-route",
			Name:       "Test Route",
			Domain:     "example.com",
			Path:       "/",
			BackendURL: "http://localhost:8081",
			Priority:   1,
			Enabled:    true,
		}
		gw.router.AddRoute(route)

		// Add a backend
		backend := &Backend{
			ID:    "backend1",
			URL:   "http://localhost:8081",
			Alive: true,
		}
		gw.proxy.AddBackend(backend)

		// Test request
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		w := httptest.NewRecorder()

		gw.ServeHTTP(w, req)

		// Should get bad gateway since backend is not real
		if w.Code != http.StatusBadGateway {
			t.Fatalf("Expected status 502, got %d", w.Code)
		}
	})

	t.Run("GatewayMatchRoute", func(t *testing.T) {
		gw := NewGateway(nil)

		route := &Route{
			ID:         "test-route",
			Name:       "Test Route",
			Domain:     "example.com",
			BackendURL: "http://localhost:8081",
			Priority:   1,
			Enabled:    true,
		}
		gw.router.AddRoute(route)

		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		matched := gw.MatchRoute(req)

		if matched == nil || matched.ID != "test-route" {
			t.Fatal("Expected to match test-route")
		}
	})
}

// TestCertificateManager tests the certificate manager.
func TestCertificateManager(t *testing.T) {
	t.Run("AddGetRemove", func(t *testing.T) {
		cm := NewCertificateManager()

		cert := &Certificate{
			Domain:    "example.com",
			CertFile:  "/path/to/cert.pem",
			KeyFile:   "/path/to/key.pem",
			ExpiresAt: time.Now().Add(365 * 24 * time.Hour),
			Issuer:    "Let's Encrypt",
		}

		cm.AddCertificate(cert)

		got, ok := cm.GetCertificate("example.com")
		if !ok {
			t.Fatal("Expected to find certificate")
		}
		if got.Domain != "example.com" {
			t.Fatal("Expected domain 'example.com'")
		}

		cm.RemoveCertificate("example.com")

		_, ok = cm.GetCertificate("example.com")
		if ok {
			t.Fatal("Expected certificate to be removed")
		}
	})

	t.Run("ListCertificates", func(t *testing.T) {
		cm := NewCertificateManager()

		cm.AddCertificate(&Certificate{Domain: "example.com"})
		cm.AddCertificate(&Certificate{Domain: "api.example.com"})

		certs := cm.ListCertificates()
		if len(certs) != 2 {
			t.Fatalf("Expected 2 certificates, got %d", len(certs))
		}
	})
}

// TestAccessLogger tests the access logger.
func TestAccessLogger(t *testing.T) {
	t.Run("LogAndFilter", func(t *testing.T) {
		logger := NewAccessLogger()

		logger.Log(AccessLogEntry{
			Timestamp:  time.Now(),
			RemoteAddr: "192.168.1.1:12345",
			Method:     "GET",
			Path:       "/api/v1/users",
			StatusCode: 200,
		})

		logger.Log(AccessLogEntry{
			Timestamp:  time.Now(),
			RemoteAddr: "192.168.1.1:12345",
			Method:     "POST",
			Path:       "/api/v1/users",
			StatusCode: 201,
		})

		entries := logger.GetEntries()
		if len(entries) != 2 {
			t.Fatalf("Expected 2 entries, got %d", len(entries))
		}

		// Filter by method
		filtered := logger.FilterEntries("GET", "", 0)
		if len(filtered) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(filtered))
		}

		// Filter by status code
		filtered = logger.FilterEntries("", "", 201)
		if len(filtered) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(filtered))
		}

		// Clear entries
		logger.ClearEntries()
		if len(logger.GetEntries()) != 0 {
			t.Fatal("Expected empty entries after clear")
		}
	})
}

// TestGatewayManager tests the gateway manager.
func TestGatewayManager(t *testing.T) {
	t.Run("AddGetRemove", func(t *testing.T) {
		manager := NewGatewayManager()

		gw := NewGateway(nil)
		manager.AddGateway("main", gw)

		got, ok := manager.GetGateway("main")
		if !ok {
			t.Fatal("Expected to find gateway")
		}
		if got != gw {
			t.Fatal("Expected same gateway instance")
		}

		manager.RemoveGateway("main")

		_, ok = manager.GetGateway("main")
		if ok {
			t.Fatal("Expected gateway to be removed")
		}
	})

	t.Run("GetStatusAll", func(t *testing.T) {
		manager := NewGatewayManager()

		gw1 := NewGateway(nil)
		gw2 := NewGateway(nil)

		manager.AddGateway("gw1", gw1)
		manager.AddGateway("gw2", gw2)

		status := manager.GetStatusAll()
		if len(status) != 2 {
			t.Fatalf("Expected 2 gateways in status, got %d", len(status))
		}
	})
}

// TestRBACMiddleware tests the RBAC middleware.
func TestRBACMiddleware(t *testing.T) {
	config := &RBACConfig{
		Roles: map[string][]string{
			"admin": {"*"},
			"user":  {"/api/v1/users"},
		},
		DefaultRole: "user",
	}

	handler := RBACMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("AdminAccess", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin", nil)
		req.Header.Set("X-User-Role", "admin")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("UserAccessAllowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		req.Header.Set("X-User-Role", "user")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("UserAccessDenied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin", nil)
		req.Header.Set("X-User-Role", "user")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("Expected status 403, got %d", w.Code)
		}
	})

	t.Run("NoRole", func(t *testing.T) {
		noDefaultConfig := &RBACConfig{
			Roles: map[string][]string{
				"admin": {"*"},
				"user":  {"/api/v1/users"},
			},
		}

		noDefaultHandler := RBACMiddleware(noDefaultConfig)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		w := httptest.NewRecorder()

		noDefaultHandler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("Expected status 403, got %d", w.Code)
		}
	})
}

// TestBasicAuthMiddleware tests the basic auth middleware.
func TestBasicAuthMiddleware(t *testing.T) {
	config := &AuthConfig{
		Enabled: true,
		Users: []User{
			{Username: "admin", Password: "password", Role: "admin"},
		},
		APIKeys:     []string{"test-api-key"},
		PublicPaths: []string{"/public"},
	}

	handler := BasicAuthMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("PublicPath", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/public/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("APIKeyAuth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("X-API-Key", "test-api-key")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("BasicAuth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.SetBasicAuth("admin", "password")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("Expected status 401, got %d", w.Code)
		}
	})
}

// TestSSLManager tests the SSL manager.
func TestSSLManager(t *testing.T) {
	config := &SSLConfig{
		Enabled:  true,
		CertFile: "/path/to/cert.pem",
		KeyFile:  "/path/to/key.pem",
		Domains:  []string{"example.com", "api.example.com"},
	}

	manager := NewSSLManager(config)

	if !manager.IsEnabled() {
		t.Fatal("Expected SSL to be enabled")
	}

	if manager.GetCertFile() != "/path/to/cert.pem" {
		t.Fatal("Expected cert file path")
	}

	if manager.GetKeyFile() != "/path/to/key.pem" {
		t.Fatal("Expected key file path")
	}

	domains := manager.GetDomains()
	if len(domains) != 2 {
		t.Fatalf("Expected 2 domains, got %d", len(domains))
	}
}

// TestLoadBalancer tests the load balancer.
func TestLoadBalancer(t *testing.T) {
	config := &LoadBalancerConfig{
		Algorithm: "round-robin",
	}

	backends := []*Backend{
		{ID: "b1", URL: "http://localhost:8081", Alive: true},
		{ID: "b2", URL: "http://localhost:8082", Alive: true},
	}

	proxy := NewLoadBalancer(config, backends)

	if len(proxy.GetBackends()) != 2 {
		t.Fatalf("Expected 2 backends, got %d", len(proxy.GetBackends()))
	}
}

// TestAccessLogMiddleware tests the access log middleware.
func TestAccessLogMiddleware(t *testing.T) {
	logger := NewAccessLogger()

	handler := AccessLogMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	entries := logger.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	if entries[0].Method != "GET" {
		t.Fatal("Expected method GET")
	}

	if entries[0].Path != "/test" {
		t.Fatal("Expected path /test")
	}
}
