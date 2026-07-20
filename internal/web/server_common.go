package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"nas-os/internal/arch"
	"nas-os/internal/auth"
	"nas-os/internal/system"
	"nas-os/internal/users"
	appversion "nas-os/internal/version"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// applySecurityMiddleware installs the shared middleware stack used by Core and Full.
func applySecurityMiddleware(engine *gin.Engine) {
	securityConfig := DefaultSecurityConfig()
	engine.Use(inputValidationMiddleware())
	engine.Use(loggerMiddleware())
	engine.Use(securityHeadersMiddleware())
	engine.Use(corsMiddleware(securityConfig))
	engine.Use(rateLimitMiddleware(securityConfig))
	engine.Use(csrfMiddleware(securityConfig))
	engine.Use(auditLogMiddleware())
}

// newEngineWithSecurity creates a gin engine with recovery + shared security stack.
func newEngineWithSecurity() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	applySecurityMiddleware(engine)
	return engine
}

// registerCorePublicAndAdminGroups creates public, authenticated, and admin API groups,
// registers module route contracts, and returns the admin group for further mounts.
func (s *Server) registerCorePublicAndAdminGroups() *gin.RouterGroup {
	publicAPI := s.engine.Group("/api/v1")
	for _, module := range s.modules {
		if registrar, ok := module.(arch.PublicRouteRegistrar); ok {
			registrar.RegisterPublicRoutes(publicAPI.Group("/auth"))
			registrar.RegisterPublicRoutes(publicAPI)
		}
	}
	publicAPI.GET("/system/health", s.getHealth)
	publicAPI.GET("/health", s.getHealth)

	authenticatedAPI := s.engine.Group("/api/v1")
	authenticatedAPI.Use(users.AuthMiddleware(s.userMgr))
	for _, module := range s.modules {
		if registrar, ok := module.(arch.AuthenticatedRouteRegistrar); ok {
			registrar.RegisterAuthenticatedRoutes(authenticatedAPI)
		}
	}

	api := s.engine.Group("/api/v1")
	api.Use(users.AuthMiddleware(s.userMgr), users.RequireAdmin(s.userMgr))
	s.adminAPI = api
	for _, module := range s.modules {
		if registrar, ok := module.(arch.RouteRegistrar); ok {
			registrar.RegisterRoutes(api)
		}
	}
	return api
}

// registerCoreIdentityAndDocs mounts MFA/RBAC (if present), Core system info when
// no bulk system monitor is active, the single production storage contract,
// swagger, and WebUI. Both Core and Full builds must call this once.
//
// Storage routes: ONLY here (not via arch storageModule.RegisterRoutes) so gin
// never sees duplicate /api/v1/storage/* registrations.
func (s *Server) registerCoreIdentityAndDocs(api *gin.RouterGroup) {
	if api == nil || s == nil || s.engine == nil {
		return
	}
	if s.mfaMgr != nil {
		auth.NewHandlers(s.mfaMgr).RegisterRoutes(api)
	}
	if s.rbacMgr != nil {
		auth.NewRBACHandlers(s.rbacMgr).RegisterRoutes(api)
	}

	// Full bulk may set systemMonitor; Core keeps it nil (any).
	// Callers on Full must register system.Monitor handlers before this if non-nil.
	if s.systemMonitor == nil {
		api.GET("/system/info", s.getSystemInfo)
		api.GET("/system/status", s.getSystemStatus)
	}

	// Single ownership for nasd storage contract (confirm_name / allow_wipe gate).
	if s.storageMgr != nil {
		NewStorageHandlers(s.storageMgr).RegisterRoutes(api)
	}

	s.engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
		ginSwagger.URL("/swagger/doc.json"),
		ginSwagger.DefaultModelsExpandDepth(-1),
	))
	s.registerWebUI(resolveWebUIRoot())
}

// getHealth aggregates Core module health (shared by both builds).
func (s *Server) getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, AggregateCoreHealth(c.Request.Context(), s.modules))
}

// getSystemInfo returns version/build surface (shared by both builds).
func (s *Server) getSystemInfo(c *gin.Context) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "nas-os"
	}
	build := appversion.GetBuildInfo()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"hostname":           hostname,
			"version":            appversion.GetVersion(),
			"build_date":         build["build_date"],
			"git_commit":         build["git_commit"],
			"products_linked":    ProductsLinked(),
			"extensions_linked":  ExtensionsLinked(),
			"surface":            map[bool]string{true: "full", false: "core"}[ProductsLinked()],
		},
	})
}

func (s *Server) getSystemStatus(c *gin.Context) {
	system.GetSystemStatus(c)
}

// coreWebUIPages are always served (Core surface).
var coreWebUIPages = map[string]bool{
	"login.html":      true,
	"dashboard.html":  true,
	"storage.html":    true,
	"shares.html":     true,
	"users.html":      true,
	"network.html":    true,
	"settings.html":   true,
	"api-docs.html":   true,
	"app-center.html": true,
	"plugins.html":    true,
}

func resolveWebUIRoot() string {
	candidates := []string{
		"webui",
		"./webui",
		"/usr/share/nas-os/webui",
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return "webui"
}

func (s *Server) registerWebUI(webuiRoot string) {
	optional := s.cfg != nil && (s.cfg.OptionalProductsEnabled() || s.cfg.Modules.Optional || len(bootWantProducts(s.cfg)) > 0)
	// Only expose full pages tree when products are both wanted and linked.
	fullPages := optional && ProductsLinked()

	s.engine.Static("/webui/css", webuiRoot+"/css")
	s.engine.Static("/webui/js", webuiRoot+"/js")
	s.engine.Static("/webui/i18n", webuiRoot+"/i18n")
	s.engine.StaticFile("/webui/index.html", webuiRoot+"/index.html")
	s.engine.StaticFile("/webui/manifest.json", webuiRoot+"/manifest.json")
	s.engine.StaticFile("/", webuiRoot+"/index.html")
	s.engine.StaticFile("/index.html", webuiRoot+"/index.html")

	if fullPages {
		s.engine.Static("/webui/pages", webuiRoot+"/pages")
	} else {
		s.engine.GET("/webui/pages/*filepath", func(c *gin.Context) {
			rel := strings.TrimPrefix(c.Param("filepath"), "/")
			base := filepath.Base(rel)
			if !coreWebUIPages[base] {
				msg := "optional product UI disabled; set packages.recommended_system=true or modules.optional=true"
				if optional && !ProductsLinked() {
					msg = "product UI requested but this binary is Core-only (rebuild with -tags nasd_full)"
				}
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": msg})
				return
			}
			c.File(filepath.Join(webuiRoot, "pages", rel))
		})
	}

	s.engine.StaticFile("/login", webuiRoot+"/pages/login.html")
	s.engine.StaticFile("/dashboard", webuiRoot+"/pages/dashboard.html")
	s.engine.StaticFile("/storage", webuiRoot+"/pages/storage.html")
	s.engine.StaticFile("/shares", webuiRoot+"/pages/shares.html")
	s.engine.StaticFile("/users", webuiRoot+"/pages/users.html")
	s.engine.StaticFile("/network", webuiRoot+"/pages/network.html")
	s.engine.StaticFile("/settings", webuiRoot+"/pages/settings.html")
	s.engine.StaticFile("/app-center", webuiRoot+"/pages/app-center.html")

	if fullPages {
		s.engine.StaticFile("/downloader", webuiRoot+"/pages/downloader/index.html")
		s.engine.StaticFile("/containers", webuiRoot+"/pages/containers.html")
		s.engine.StaticFile("/vms", webuiRoot+"/pages/vms.html")
		s.engine.StaticFile("/cloudsync", webuiRoot+"/pages/cloudsync.html")
	}
}
