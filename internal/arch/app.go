package arch

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AppModule 应用模块 - 整合所有子模块.
type AppModule struct {
	*BaseModule
	container *Container
	engine    *gin.Engine
	server    *http.Server
}

// NewAppModule 创建应用模块.
func NewAppModule(logger *zap.Logger, container *Container) *AppModule {
	return &AppModule{
		BaseModule: &BaseModule{
			NameStr: "app",
			Logger:  logger,
		},
		container: container,
	}
}

// Init 初始化应用.
func (a *AppModule) Init(ctx context.Context) error {
	a.engine = gin.New()
	a.engine.Use(gin.Recovery())

	// 注册所有模块的路由
	a.registerRoutes()
	return nil
}

// Start 启动HTTP服务器.
func (a *AppModule) Start(ctx context.Context) error {
	a.server = &http.Server{
		Addr:    ":8080",
		Handler: a.engine,
	}

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.Logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	a.Logger.Info("HTTP server started", zap.String("addr", ":8080"))
	return nil
}

// Stop 停止HTTP服务器.
func (a *AppModule) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return a.server.Shutdown(shutdownCtx)
}

func (a *AppModule) registerRoutes() {
	api := a.engine.Group("/api/v1")

	// 遍历容器中注册的模块，注册路由
	for name, mod := range a.container.modules {
		if r, ok := mod.(RouteRegistrar); ok {
			a.Logger.Info("Registering routes", zap.String("module", name))
			r.RegisterRoutes(api)
		}
	}
}

// RouteRegistrar 路由注册接口.
type RouteRegistrar interface {
	RegisterRoutes(rg *gin.RouterGroup)
}

// ModuleAdapter 模块适配器 - 将现有Manager适配为Module接口.
type ModuleAdapter struct {
	*BaseModule
	initFn  func(ctx context.Context) error
	startFn func(ctx context.Context) error
	stopFn  func(ctx context.Context) error
}

// NewModuleAdapter 创建模块适配器.
func NewModuleAdapter(name string, deps []string, logger *zap.Logger) *ModuleAdapter {
	return &ModuleAdapter{
		BaseModule: &BaseModule{
			NameStr: name,
			Deps:    deps,
			Logger:  logger,
		},
	}
}

func (m *ModuleAdapter) WithInit(fn func(ctx context.Context) error) *ModuleAdapter {
	m.initFn = fn
	return m
}

func (m *ModuleAdapter) WithStart(fn func(ctx context.Context) error) *ModuleAdapter {
	m.startFn = fn
	return m
}

func (m *ModuleAdapter) WithStop(fn func(ctx context.Context) error) *ModuleAdapter {
	m.stopFn = fn
	return m
}

func (m *ModuleAdapter) Init(ctx context.Context) error {
	if m.initFn != nil {
		return m.initFn(ctx)
	}
	return nil
}

func (m *ModuleAdapter) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn(ctx)
	}
	return nil
}

func (m *ModuleAdapter) Stop(ctx context.Context) error {
	if m.stopFn != nil {
		return m.stopFn(ctx)
	}
	return nil
}

// ListModules 列出所有已注册模块（按名称排序）.
func (c *Container) ListModules() []string {
	c.mu.RLock()
	names := make([]string, 0, len(c.modules))
	for name := range c.modules {
		names = append(names, name)
	}
	c.mu.RUnlock()
	sort.Strings(names)
	return names
}

// GetModule 获取模块.
func (c *Container) GetModule(name string) (Module, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.modules[name]
	return m, ok
}

// ModuleStatus 模块状态.
type ModuleStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

// GetModulesStatus 获取所有模块状态。健康回调在容器锁外执行，避免阻塞注册和查询。
func (c *Container) GetModulesStatus(ctx context.Context) []ModuleStatus {
	c.mu.RLock()
	modules := make(map[string]Module, len(c.modules))
	names := make([]string, 0, len(c.modules))
	for name, mod := range c.modules {
		modules[name] = mod
		names = append(names, name)
	}
	c.mu.RUnlock()
	sort.Strings(names)

	statuses := make([]ModuleStatus, 0, len(names))
	for _, name := range names {
		status := ModuleStatus{Name: name, Healthy: true}
		if err := modules[name].Health(ctx); err != nil {
			status.Healthy = false
			status.Error = err.Error()
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// RestartModule 重启模块.
func (c *Container) RestartModule(ctx context.Context, name string) error {
	c.mu.RLock()
	mod, ok := c.modules[name]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("module %s not found", name)
	}

	if err := mod.Stop(ctx); err != nil {
		c.logger.Warn("Module stop failed", zap.String("name", name), zap.Error(err))
	}

	if err := mod.Init(ctx); err != nil {
		return fmt.Errorf("re-init module %s failed: %w", name, err)
	}

	return mod.Start(ctx)
}

// PublicRouteRegistrar 注册无需身份认证的公开路由。
type PublicRouteRegistrar interface {
	RegisterPublicRoutes(rg *gin.RouterGroup)
}

// AuthenticatedRouteRegistrar 注册仅要求登录的路由。
type AuthenticatedRouteRegistrar interface {
	RegisterAuthenticatedRoutes(rg *gin.RouterGroup)
}
