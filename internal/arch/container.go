package arch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"
)

// ModuleTier 描述模块在生产架构中的收敛层级。
type ModuleTier string

const (
	ModuleTierCore      ModuleTier = "core"
	ModuleTierExtension ModuleTier = "extension"
	ModuleTierLab       ModuleTier = "lab"
)

// Module 模块接口 - 所有模块必须实现.
type Module interface {
	// Name 模块名称
	Name() string
	// Tier 返回模块层级，用于 Core / Extension / Lab 收敛治理。
	Tier() ModuleTier
	// Init 初始化模块
	Init(ctx context.Context) error
	// Start 启动模块
	Start(ctx context.Context) error
	// Stop 停止模块
	Stop(ctx context.Context) error
	// Health 健康检查
	Health(ctx context.Context) error
	// Dependencies 返回依赖的模块名
	Dependencies() []string
}

// Container 依赖注入容器.
type Container struct {
	mu       sync.RWMutex
	services map[string]interface{}
	modules  map[string]Module
	logger   *zap.Logger
}

// NewContainer 创建容器.
func NewContainer(logger *zap.Logger) *Container {
	return &Container{
		services: make(map[string]interface{}),
		modules:  make(map[string]Module),
		logger:   logger,
	}
}

// Register 注册服务.
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
	c.logger.Debug("Registered service", zap.String("name", name))
}

// Get 获取服务.
func (c *Container) Get(name string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.services[name]
	return s, ok
}

// MustGet 获取服务(必须存在).
func (c *Container) MustGet(name string) interface{} {
	s, ok := c.Get(name)
	if !ok {
		panic(fmt.Sprintf("service %s not found", name))
	}
	return s
}

// RegisterModule 注册模块.
func (c *Container) RegisterModule(mod Module) error {
	if mod == nil {
		return fmt.Errorf("module is nil")
	}
	name := mod.Name()
	if name == "" {
		return fmt.Errorf("module name is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.modules[name]; exists {
		return fmt.Errorf("module %s already registered", name)
	}
	c.modules[name] = mod
	c.logger.Info("Registered module", zap.String("name", name))
	return nil
}

// InitAll 按依赖顺序初始化所有模块；初始化失败时逆序清理已初始化模块。
func (c *Container) InitAll(ctx context.Context) error {
	modules, sorted, err := c.lifecycleSnapshot()
	if err != nil {
		return err
	}

	initialized := make([]string, 0, len(sorted))
	for _, name := range sorted {
		c.logger.Info("Initializing module", zap.String("name", name))
		if err := modules[name].Init(ctx); err != nil {
			errs := []error{fmt.Errorf("init module %s failed: %w", name, err)}
			for i := len(initialized) - 1; i >= 0; i-- {
				initializedName := initialized[i]
				if stopErr := modules[initializedName].Stop(ctx); stopErr != nil {
					errs = append(errs, fmt.Errorf("rollback initialized module %s failed: %w", initializedName, stopErr))
				}
			}
			return errors.Join(errs...)
		}
		initialized = append(initialized, name)
	}
	return nil
}

// StartAll 按依赖顺序启动所有模块；启动失败时逆序回滚已启动模块.
func (c *Container) StartAll(ctx context.Context) error {
	modules, sorted, err := c.lifecycleSnapshot()
	if err != nil {
		return err
	}

	started := make([]string, 0, len(sorted))
	for _, name := range sorted {
		c.logger.Info("Starting module", zap.String("name", name))
		if err := modules[name].Start(ctx); err != nil {
			errs := []error{fmt.Errorf("start module %s failed: %w", name, err)}
			for i := len(started) - 1; i >= 0; i-- {
				startedName := started[i]
				if stopErr := modules[startedName].Stop(ctx); stopErr != nil {
					errs = append(errs, fmt.Errorf("rollback module %s failed: %w", startedName, stopErr))
				}
			}
			return errors.Join(errs...)
		}
		started = append(started, name)
	}
	return nil
}

// StopAll 按依赖逆序停止所有模块并聚合错误.
func (c *Container) StopAll(ctx context.Context) error {
	modules, sorted, err := c.lifecycleSnapshot()
	if err != nil {
		return err
	}

	var errs []error
	for i := len(sorted) - 1; i >= 0; i-- {
		name := sorted[i]
		c.logger.Info("Stopping module", zap.String("name", name))
		if err := modules[name].Stop(ctx); err != nil {
			wrapped := fmt.Errorf("stop module %s failed: %w", name, err)
			errs = append(errs, wrapped)
			c.logger.Error("Stop module failed", zap.String("name", name), zap.Error(err))
		}
	}
	return errors.Join(errs...)
}

func (c *Container) lifecycleSnapshot() (map[string]Module, []string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sorted, err := c.topoSortLocked()
	if err != nil {
		return nil, nil, err
	}
	modules := make(map[string]Module, len(c.modules))
	for name, mod := range c.modules {
		modules[name] = mod
	}
	return modules, sorted, nil
}

// HealthAll 检查所有模块健康状态；健康回调在容器锁外执行。
func (c *Container) HealthAll(ctx context.Context) map[string]error {
	c.mu.RLock()
	modules := make(map[string]Module, len(c.modules))
	names := make([]string, 0, len(c.modules))
	for name, mod := range c.modules {
		modules[name] = mod
		names = append(names, name)
	}
	c.mu.RUnlock()
	sort.Strings(names)

	results := make(map[string]error, len(names))
	for _, name := range names {
		results[name] = modules[name].Health(ctx)
	}
	return results
}

// topoSort 拓扑排序.
func (c *Container) topoSort() ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.topoSortLocked()
}

func (c *Container) topoSortLocked() ([]string, error) {
	inDegree := make(map[string]int, len(c.modules))
	graph := make(map[string][]string, len(c.modules))

	for name := range c.modules {
		inDegree[name] = 0
	}
	for name, mod := range c.modules {
		for _, dep := range mod.Dependencies() {
			if _, exists := c.modules[dep]; !exists {
				return nil, fmt.Errorf("module %s depends on missing module %s", name, dep)
			}
			graph[dep] = append(graph[dep], name)
			inDegree[name]++
		}
	}
	for dep := range graph {
		sort.Strings(graph[dep])
	}

	queue := make([]string, 0, len(inDegree))
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	sorted := make([]string, 0, len(c.modules))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)
		for _, next := range graph[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}
	if len(sorted) != len(c.modules) {
		return nil, fmt.Errorf("circular dependency detected")
	}
	return sorted, nil
}

// BaseModule 基础模块 - 可嵌入以提供默认实现.
type BaseModule struct {
	NameStr string
	Deps    []string
	Logger  *zap.Logger
}

func (b *BaseModule) Name() string                     { return b.NameStr }
func (b *BaseModule) Tier() ModuleTier                 { return ModuleTierExtension }
func (b *BaseModule) Init(ctx context.Context) error   { return nil }
func (b *BaseModule) Start(ctx context.Context) error  { return nil }
func (b *BaseModule) Stop(ctx context.Context) error   { return nil }
func (b *BaseModule) Health(ctx context.Context) error { return nil }
func (b *BaseModule) Dependencies() []string           { return b.Deps }
