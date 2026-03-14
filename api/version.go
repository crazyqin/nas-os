// Package api provides API versioning support for NAS-OS
// Version: 2.0 - API versioning with backward compatibility
package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// VersionInfo represents API version information
type VersionInfo struct {
	Version     string `json:"version"`
	Deprecated  bool   `json:"deprecated"`
	SunsetDate  string `json:"sunsetDate,omitempty"`
	Description string `json:"description"`
}

// VersionRouter manages API version routing
type VersionRouter struct {
	versions map[string]*VersionConfig
	current  string
}

// VersionConfig holds configuration for a specific API version
type VersionConfig struct {
	Version    string
	Deprecated bool
	SunsetDate string
	Router     func(*gin.RouterGroup)
}

// NewVersionRouter creates a new version router
func NewVersionRouter(currentVersion string) *VersionRouter {
	return &VersionRouter{
		versions: make(map[string]*VersionConfig),
		current:  currentVersion,
	}
}

// RegisterVersion registers an API version
func (vr *VersionRouter) RegisterVersion(version string, config *VersionConfig) {
	vr.versions[version] = config
}

// GetCurrentVersion returns the current API version
func (vr *VersionRouter) GetCurrentVersion() string {
	return vr.current
}

// GetVersions returns all registered versions
func (vr *VersionRouter) GetVersions() []VersionInfo {
	var versions []VersionInfo
	for v, config := range vr.versions {
		info := VersionInfo{
			Version:     v,
			Deprecated:  config.Deprecated,
			SunsetDate:  config.SunsetDate,
			Description: fmt.Sprintf("API version %s", v),
		}
		versions = append(versions, info)
	}
	return versions
}

// SetupRoutes sets up versioned routes
func (vr *VersionRouter) SetupRoutes(engine *gin.Engine, basePath string) {
	// Version discovery endpoint
	engine.GET(basePath+"/versions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"current":  vr.current,
				"versions": vr.GetVersions(),
			},
		})
	})

	// Register each version
	for version, config := range vr.versions {
		group := engine.Group(basePath + "/" + version)

		// Add deprecation warning header if deprecated
		if config.Deprecated {
			group.Use(func(c *gin.Context) {
				c.Header("X-API-Deprecated", "true")
				if config.SunsetDate != "" {
					c.Header("X-API-Sunset", config.SunsetDate)
					c.Header("Deprecation", "true")
					c.Header("Sunset", config.SunsetDate)
				}
				c.Next()
			})
		}

		// Add version header
		group.Use(func(c *gin.Context) {
			c.Header("X-API-Version", version)
			c.Next()
		})

		// Register routes
		if config.Router != nil {
			config.Router(group)
		}
	}

	// Default route redirects to current version
	engine.GET(basePath, func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, basePath+"/"+vr.current+"/")
	})
}

// VersionMiddleware extracts and validates API version from request
func VersionMiddleware(currentVersion string, supportedVersions []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Extract version from path
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && strings.HasPrefix(parts[0], "api") {
			requestedVersion := parts[1]

			// Validate version
			valid := false
			for _, v := range supportedVersions {
				if v == requestedVersion {
					valid = true
					break
				}
			}

			if !valid && requestedVersion != "versions" {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    400,
					"message": fmt.Sprintf("Unsupported API version: %s", requestedVersion),
					"data": gin.H{
						"currentVersion":    currentVersion,
						"supportedVersions": supportedVersions,
					},
				})
				c.Abort()
				return
			}

			// Store version in context
			c.Set("apiVersion", requestedVersion)
		}

		c.Next()
	}
}

// AcceptVersionMiddleware extracts version from Accept header
// Example: Accept: application/vnd.nas-os.v1+json
func AcceptVersionMiddleware(currentVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		accept := c.GetHeader("Accept")

		// Parse Accept header for version
		// Format: application/vnd.nas-os.v1+json
		if strings.Contains(accept, "application/vnd.nas-os.") {
			parts := strings.Split(accept, ".")
			if len(parts) >= 3 {
				versionPart := strings.Split(parts[2], "+")[0]
				c.Set("apiVersion", versionPart)
			}
		} else {
			c.Set("apiVersion", currentVersion)
		}

		c.Next()
	}
}

// VersionHandler provides API version management handlers
type VersionHandler struct {
	router *VersionRouter
}

// NewVersionHandler creates a new version handler
func NewVersionHandler(router *VersionRouter) *VersionHandler {
	return &VersionHandler{router: router}
}

// GetVersions returns all API versions
// @Summary Get API versions
// @Description Get all supported API versions
// @Tags api
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/versions [get]
func (h *VersionHandler) GetVersions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"current":  h.router.GetCurrentVersion(),
			"versions": h.router.GetVersions(),
		},
	})
}

// DeprecationNotice represents a deprecation notice
type DeprecationNotice struct {
	Version      string   `json:"version"`
	Endpoint     string   `json:"endpoint"`
	Reason       string   `json:"reason"`
	Migration    string   `json:"migration"`
	RemovalDate  string   `json:"removalDate"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// DeprecationManager manages API deprecations
type DeprecationManager struct {
	notices []DeprecationNotice
}

// NewDeprecationManager creates a new deprecation manager
func NewDeprecationManager() *DeprecationManager {
	return &DeprecationManager{
		notices: make([]DeprecationNotice, 0),
	}
}

// AddNotice adds a deprecation notice
func (dm *DeprecationManager) AddNotice(notice DeprecationNotice) {
	dm.notices = append(dm.notices, notice)
}

// GetNotices returns all deprecation notices
func (dm *DeprecationManager) GetNotices() []DeprecationNotice {
	return dm.notices
}

// Middleware adds deprecation headers for deprecated endpoints
func (dm *DeprecationManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if endpoint is deprecated
		for _, notice := range dm.notices {
			if strings.HasPrefix(c.Request.URL.Path, notice.Endpoint) {
				c.Header("X-API-Deprecated", "true")
				c.Header("X-API-Deprecation-Reason", notice.Reason)
				c.Header("X-API-Removal-Date", notice.RemovalDate)
				if len(notice.Alternatives) > 0 {
					c.Header("X-API-Alternatives", strings.Join(notice.Alternatives, ", "))
				}
				break
			}
		}
		c.Next()
	}
}
