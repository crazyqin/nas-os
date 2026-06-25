package arch

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Module 模块接口 - 所有模块必须实现
type Module interface {
	// Name 模块名称
	Name() string
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

// Container 依赖注入容器
type Container struct {
	mu       sync.RWMutex
	services map[string]interface{}
	modules  map[string]Module
	logger   *zap.Logger
}

// NewContainer 创建容器
func NewContainer(logger *zap.Logger) *Container {
	return &Container{
		services: make(map[string]interface{}),
		modules:  make(map[string]Module),
		logger:   logger,
	}
}

// Register 注册服务
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
	c.logger.Debug("Registered service", zap.String("name", name))
}

// Get 获取服务
func (c *Container) Get(name string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.services[name]
	return s, ok
}

// MustGet 获取服务(必须存在)
func (c *Container) MustGet(name string) interface{} {
	s, ok := c.Get(name)
	if !ok {
		panic(fmt.Sprintf("service %s not found", name))
	}
	return s
}

// RegisterModule 注册模块
func (c *Container) RegisterModule(mod Module) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modules[mod.Name()] = mod
	c.logger.Info("Registered module", zap.String("name", mod.Name()))
}

// InitAll 按依赖顺序初始化所有模块
func (c *Container) InitAll(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 拓扑排序
	sorted, err := c.topoSort()
	if err != nil {
		return err
	}

	for _, name := range sorted {
		mod := c.modules[name]
		c.logger.Info("Initializing module", zap.String("name", name))
		if err := mod.Init(ctx); err != nil {
			return fmt.Errorf("init module %s failed: %w", name, err)
		}
	}
	return nil
}

// StartAll 按依赖顺序启动所有模块
func (c *Container) StartAll(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sorted, err := c.topoSort()
	if err != nil {
		return err
	}

	for _, name := range sorted {
		mod := c.modules[name]
		c.logger.Info("Starting module", zap.String("name", name))
		if err := mod.Start(ctx); err != nil {
			return fmt.Errorf("start module %s failed: %w", name, err)
		}
	}
	return nil
}

// StopAll 按依赖逆序停止所有模块
func (c *Container) StopAll(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sorted, err := c.topoSort()
	if err != nil {
		return err
	}

	// 逆序停止
	for i := len(sorted) - 1; i >= 0; i-- {
		name := sorted[i]
		mod := c.modules[name]
		c.logger.Info("Stopping module", zap.String("name", name))
		if err := mod.Stop(ctx); err != nil {
			c.logger.Error("Stop module failed", zap.String("name", name), zap.Error(err))
		}
	}
	return nil
}

// HealthAll 检查所有模块健康状态
func (c *Container) HealthAll(ctx context.Context) map[string]error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]error)
	for name, mod := range c.modules {
		results[name] = mod.Health(ctx)
	}
	return results
}

// topoSort 拓扑排序
func (c *Container) topoSort() ([]string, error) {
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for name := range c.modules {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
		for _, dep := range c.modules[name].Dependencies() {
			graph[dep] = append(graph[dep], name)
			inDegree[name]++
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		for _, next := range graph[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(sorted) != len(c.modules) {
		return nil, fmt.Errorf("circular dependency detected")
	}
	return sorted, nil
}

// BaseModule 基础模块 - 可嵌入以提供默认实现
type BaseModule struct {
	NameStr string
	Deps    []string
	Logger  *zap.Logger
}

func (b *BaseModule) Name() string                { return b.NameStr }
func (b *BaseModule) Init(ctx context.Context) error { return nil }
func (b *BaseModule) Start(ctx context.Context) error { return nil }
func (b *BaseModule) Stop(ctx context.Context) error  { return nil }
func (b *BaseModule) Health(ctx context.Context) error { return nil }
func (b *BaseModule) Dependencies() []string         { return b.Deps }
