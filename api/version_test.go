package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVersionRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := NewVersionRouter("v1")

	// Test initial state
	if router.GetCurrentVersion() != "v1" {
		t.Errorf("Expected current version v1, got %s", router.GetCurrentVersion())
	}

	// Test registering a version
	config := &VersionConfig{
		Version: "v1",
		Router: func(g *gin.RouterGroup) {
			g.GET("/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "ok"})
			})
		},
	}
	router.RegisterVersion("v1", config)

	// Test getting versions
	versions := router.GetVersions()
	if len(versions) != 1 {
		t.Errorf("Expected 1 version, got %d", len(versions))
	}
}

func TestVersionRouterSetupRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	router := NewVersionRouter("v1")

	// Register v1
	router.RegisterVersion("v1", &VersionConfig{
		Version: "v1",
		Router: func(g *gin.RouterGroup) {
			g.GET("/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "v1"})
			})
		},
	})

	// Register deprecated v0
	router.RegisterVersion("v0", &VersionConfig{
		Version:    "v0",
		Deprecated: true,
		SunsetDate: "2026-06-01",
		Router: func(g *gin.RouterGroup) {
			g.GET("/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "v0"})
			})
		},
	})

	router.SetupRoutes(engine, "/api")

	// Test version discovery
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/versions", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})
	if data["current"] != "v1" {
		t.Errorf("Expected current version v1, got %v", data["current"])
	}
}

func TestVersionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		path              string
		supportedVersions []string
		expectedStatus    int
	}{
		{
			name:              "valid version v1",
			path:              "/api/v1/test",
			supportedVersions: []string{"v1", "v2"},
			expectedStatus:    http.StatusOK,
		},
		{
			name:              "valid version v2",
			path:              "/api/v2/test",
			supportedVersions: []string{"v1", "v2"},
			expectedStatus:    http.StatusOK,
		},
		{
			name:              "invalid version",
			path:              "/api/v3/test",
			supportedVersions: []string{"v1", "v2"},
			expectedStatus:    http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(VersionMiddleware("v1", tt.supportedVersions))
			engine.GET("/api/v1/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"ok": true})
			})
			engine.GET("/api/v2/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"ok": true})
			})
			engine.GET("/api/v3/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"ok": true})
			})

			req := httptest.NewRequestWithContext(context.Background(), "GET", tt.path, nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestDeprecationManager(t *testing.T) {
	dm := NewDeprecationManager()

	// Test adding notices
	dm.AddNotice(DeprecationNotice{
		Version:     "v0",
		Endpoint:    "/api/v0",
		Reason:      "Upgrade to v1",
		Migration:   "Replace /api/v0 with /api/v1",
		RemovalDate: "2026-06-01",
	})

	notices := dm.GetNotices()
	if len(notices) != 1 {
		t.Errorf("Expected 1 notice, got %d", len(notices))
	}

	if notices[0].Version != "v0" {
		t.Errorf("Expected version v0, got %s", notices[0].Version)
	}
}

func TestDeprecationMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dm := NewDeprecationManager()
	dm.AddNotice(DeprecationNotice{
		Endpoint:    "/api/v0/",
		Reason:      "Upgrade to v1",
		RemovalDate: "2026-06-01",
	})

	engine := gin.New()
	engine.Use(dm.Middleware())
	engine.GET("/api/v0/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v0/test", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	// Check deprecation headers
	if w.Header().Get("X-API-Deprecated") != "true" {
		t.Error("Expected X-API-Deprecated header")
	}

	if w.Header().Get("X-API-Deprecation-Reason") != "Upgrade to v1" {
		t.Error("Expected deprecation reason header")
	}
}
