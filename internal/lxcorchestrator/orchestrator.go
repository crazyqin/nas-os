package lxcorchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Orchestrator LXC 容器编排引擎
type Orchestrator struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	config     *OrchestratorConfig
	containers map[string]*ContainerInstance
	networks   map[string]*NetworkConfig
	volumes    map[string]*VolumeConfig
	templates  map[string]*ContainerTemplate
	scheduler  *Scheduler
	networkMgr *NetworkManager
	volumeMgr  *VolumeManager
	running    bool
	cancel     context.CancelFunc
}

// NewOrchestrator 创建编排器实例
func NewOrchestrator(logger *zap.Logger, config *OrchestratorConfig) *Orchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	o := &Orchestrator{
		logger:     logger,
		config:     config,
		containers: make(map[string]*ContainerInstance),
		networks:   make(map[string]*NetworkConfig),
		volumes:    make(map[string]*VolumeConfig),
		templates:  make(map[string]*ContainerTemplate),
	}

	o.scheduler = NewScheduler(logger, o)
	o.networkMgr = NewNetworkManager(logger, o)
	o.volumeMgr = NewVolumeManager(logger, o)

	return o
}

// Start 启动编排器
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.running {
		return fmt.Errorf("orchestrator already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	o.cancel = cancel
	o.running = true

	o.logger.Info("starting LXC orchestrator",
		zap.Int("max_containers", o.config.MaxContainers),
		zap.Bool("health_check", o.config.HealthCheck),
	)

	// 初始化默认网络
	if err := o.networkMgr.InitDefaultNetwork(ctx); err != nil {
		o.logger.Warn("failed to init default network", zap.Error(err))
	}

	// 初始化默认存储卷
	if err := o.volumeMgr.InitDefaultVolume(ctx); err != nil {
		o.logger.Warn("failed to init default volume", zap.Error(err))
	}

	// 启动健康检查
	if o.config.HealthCheck {
		go o.runHealthCheckLoop(ctx)
	}

	// 启动调度器
	go o.scheduler.Run(ctx)

	return nil
}

// Stop 停止编排器
func (o *Orchestrator) Stop(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.running {
		return nil
	}

	o.logger.Info("stopping LXC orchestrator")

	if o.cancel != nil {
		o.cancel()
	}

	// 停止所有运行中的容器
	for _, container := range o.containers {
		if container.State == StateRunning {
			if err := o.stopContainer(ctx, container.Config.ID); err != nil {
				o.logger.Error("failed to stop container",
					zap.String("container_id", container.Config.ID),
					zap.Error(err),
				)
			}
		}
	}

	o.running = false
	return nil
}

// GetStatus 获取编排状态
func (o *Orchestrator) GetStatus() *OrchestrationStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()

	status := &OrchestrationStatus{
		TotalContainers: len(o.containers),
		TotalNetworks:   len(o.networks),
		TotalVolumes:    len(o.volumes),
	}

	for _, c := range o.containers {
		switch c.State {
		case StateRunning:
			status.RunningContainers++
		case StateStopped:
			status.StoppedContainers++
		case StateError:
			status.ErrorContainers++
		}
		status.Containers = append(status.Containers, *c)
	}

	for _, n := range o.networks {
		status.Networks = append(status.Networks, *n)
	}

	for _, v := range o.volumes {
		status.Volumes = append(status.Volumes, *v)
	}

	return status
}

// CreateContainer 创建容器
func (o *Orchestrator) CreateContainer(ctx context.Context, config *ContainerConfig) (*ContainerInstance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.running {
		return nil, fmt.Errorf("orchestrator not running")
	}

	if len(o.containers) >= o.config.MaxContainers {
		return nil, fmt.Errorf("maximum container limit reached: %d", o.config.MaxContainers)
	}

	// 生成 ID
	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// 检查重复
	if _, exists := o.containers[config.ID]; exists {
		return nil, fmt.Errorf("container already exists: %s", config.ID)
	}

	// 检查依赖
	for _, depID := range config.Dependencies {
		if _, exists := o.containers[depID]; !exists {
			return nil, fmt.Errorf("dependency not found: %s", depID)
		}
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	instance := &ContainerInstance{
		Config: *config,
		State:  StateCreated,
	}

	o.containers[config.ID] = instance

	o.logger.Info("container created",
		zap.String("id", config.ID),
		zap.String("name", config.Name),
		zap.String("image", config.Image),
	)

	return instance, nil
}

// StartContainer 启动容器
func (o *Orchestrator) StartContainer(ctx context.Context, containerID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	container, exists := o.containers[containerID]
	if !exists {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if container.State == StateRunning {
		return fmt.Errorf("container already running: %s", containerID)
	}

	// 检查依赖是否满足
	if err := o.checkDependencies(container.Config.Dependencies); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// 分配资源
	if err := o.scheduler.AllocateResources(container); err != nil {
		return fmt.Errorf("resource allocation failed: %w", err)
	}

	// 配置网络
	if err := o.networkMgr.ConfigureContainer(ctx, container); err != nil {
		return fmt.Errorf("network configuration failed: %w", err)
	}

	// 挂载卷
	if err := o.volumeMgr.MountVolumes(ctx, container); err != nil {
		return fmt.Errorf("volume mount failed: %w", err)
	}

	container.State = StateStarting

	// 启动容器 (模拟 LXC 启动)
	if err := o.startContainerProcess(ctx, container); err != nil {
		container.State = StateError
		container.Error = err.Error()
		return fmt.Errorf("container start failed: %w", err)
	}

	now := time.Now()
	container.State = StateRunning
	container.StartedAt = &now
	container.IPAddress = o.networkMgr.GetContainerIP(containerID)

	o.logger.Info("container started",
		zap.String("id", containerID),
		zap.String("ip", container.IPAddress),
	)

	return nil
}

// StopContainer 停止容器
func (o *Orchestrator) StopContainer(ctx context.Context, containerID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.stopContainer(ctx, containerID)
}

func (o *Orchestrator) stopContainer(ctx context.Context, containerID string) error {
	container, exists := o.containers[containerID]
	if !exists {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if container.State != StateRunning {
		return fmt.Errorf("container not running: %s", containerID)
	}

	container.State = StateStopping

	// 停止容器
	if err := o.stopContainerProcess(ctx, container); err != nil {
		o.logger.Error("failed to stop container process",
			zap.String("id", containerID),
			zap.Error(err),
		)
	}

	now := time.Now()
	container.State = StateStopped
	container.StoppedAt = &now
	container.IPAddress = ""

	o.logger.Info("container stopped", zap.String("id", containerID))
	return nil
}

// RestartContainer 重启容器
func (o *Orchestrator) RestartContainer(ctx context.Context, containerID string) error {
	o.mu.Lock()
	container, exists := o.containers[containerID]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("container not found: %s", containerID)
	}
	o.mu.Unlock()

	if container.State != StateRunning {
		return fmt.Errorf("container not running: %s", containerID)
	}

	if err := o.StopContainer(ctx, containerID); err != nil {
		return err
	}

	return o.StartContainer(ctx, containerID)
}

// DestroyContainer 销毁容器
func (o *Orchestrator) DestroyContainer(ctx context.Context, containerID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	container, exists := o.containers[containerID]
	if !exists {
		return fmt.Errorf("container not found: %s", containerID)
	}

	// 检查是否被其他容器依赖
	for _, c := range o.containers {
		for _, dep := range c.Config.Dependencies {
			if dep == containerID {
				return fmt.Errorf("container is dependency of: %s", c.Config.ID)
			}
		}
	}

	// 停止容器
	if container.State == StateRunning {
		if err := o.stopContainer(ctx, containerID); err != nil {
			o.logger.Error("failed to stop container before destroy",
				zap.String("id", containerID),
				zap.Error(err),
			)
		}
	}

	// 清理资源
	o.volumeMgr.UnmountVolumes(ctx, container)
	o.networkMgr.RemoveContainer(ctx, container)

	container.State = StateDestroyed
	delete(o.containers, containerID)

	o.logger.Info("container destroyed", zap.String("id", containerID))
	return nil
}

// GetContainer 获取容器信息
func (o *Orchestrator) GetContainer(containerID string) (*ContainerInstance, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	container, exists := o.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("container not found: %s", containerID)
	}

	return container, nil
}

// ListContainers 列出容器
func (o *Orchestrator) ListContainers(filters map[string]string) []*ContainerInstance {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*ContainerInstance
	for _, c := range o.containers {
		if o.matchFilters(c, filters) {
			result = append(result, c)
		}
	}

	return result
}

// DeployFromTemplate 从模板部署容器
func (o *Orchestrator) DeployFromTemplate(ctx context.Context, req *DeployRequest) ([]*ContainerInstance, error) {
	o.mu.RLock()
	template, exists := o.templates[req.TemplateID]
	o.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("template not found: %s", req.TemplateID)
	}

	// 应用变量
	config := template.Config
	if req.Name != "" {
		config.Name = req.Name
	}
	if req.Image != "" {
		config.Image = req.Image
	}
	if req.Resources != (ResourceLimits{}) {
		config.Resources = req.Resources
	}
	if req.RestartPolicy != "" {
		config.RestartPolicy = req.RestartPolicy
	}
	if req.Labels != nil {
		if config.Labels == nil {
			config.Labels = make(map[string]string)
		}
		for k, v := range req.Labels {
			config.Labels[k] = v
		}
	}
	if req.Dependencies != nil {
		config.Dependencies = req.Dependencies
	}

	// 替换模板变量
	for k, v := range req.Variables {
		if config.Environment == nil {
			config.Environment = make(map[string]string)
		}
		config.Environment[k] = v
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}

	var instances []*ContainerInstance
	for i := 0; i < count; i++ {
		instanceConfig := config
		if count > 1 {
			instanceConfig.Name = fmt.Sprintf("%s-%d", config.Name, i+1)
		}

		instance, err := o.CreateContainer(ctx, &instanceConfig)
		if err != nil {
			return instances, fmt.Errorf("failed to create container %d: %w", i+1, err)
		}

		// 自动启动
		if err := o.StartContainer(ctx, instance.Config.ID); err != nil {
			o.logger.Error("failed to start deployed container",
				zap.String("id", instance.Config.ID),
				zap.Error(err),
			)
		}

		instances = append(instances, instance)
	}

	return instances, nil
}

// ScaleContainer 扩缩容
func (o *Orchestrator) ScaleContainer(ctx context.Context, req *ScaleRequest) error {
	o.mu.RLock()
	container, exists := o.containers[req.ContainerID]
	o.mu.RUnlock()

	if !exists {
		return fmt.Errorf("container not found: %s", req.ContainerID)
	}

	// 获取当前同名容器数量
	currentCount := o.getContainerCountByName(container.Config.Name)

	if req.Count == currentCount {
		return nil
	}

	if req.Count > currentCount {
		// 扩容
		for i := currentCount; i < req.Count; i++ {
			config := container.Config
			config.ID = ""
			config.Name = fmt.Sprintf("%s-%d", container.Config.Name, i+1)

			instance, err := o.CreateContainer(ctx, &config)
			if err != nil {
				return fmt.Errorf("scale up failed: %w", err)
			}

			if err := o.StartContainer(ctx, instance.Config.ID); err != nil {
				return fmt.Errorf("scale up start failed: %w", err)
			}
		}
	} else {
		// 缩容
		containers := o.getContainersByName(container.Config.Name)
		for i := currentCount - 1; i >= req.Count; i-- {
			if err := o.DestroyContainer(ctx, containers[i].Config.ID); err != nil {
				return fmt.Errorf("scale down failed: %w", err)
			}
		}
	}

	o.logger.Info("container scaled",
		zap.String("name", container.Config.Name),
		zap.Int("from", currentCount),
		zap.Int("to", req.Count),
	)

	return nil
}

// GetContainerStats 获取容器统计信息
func (o *Orchestrator) GetContainerStats(containerID string) (*ContainerStats, error) {
	o.mu.RLock()
	container, exists := o.containers[containerID]
	o.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("container not found: %s", containerID)
	}

	if container.State != StateRunning {
		return nil, fmt.Errorf("container not running: %s", containerID)
	}

	// 模拟统计数据 (实际实现需要读取 cgroup)
	return &ContainerStats{
		ContainerID: containerID,
		CPUUsage:    15.5,
		MemoryUsage: 256 * 1024 * 1024, // 256MB
		MemoryLimit: container.Config.Resources.MemoryLimit,
		NetworkRx:   1024 * 1024,
		NetworkTx:   512 * 1024,
		DiskRead:    10 * 1024 * 1024,
		DiskWrite:   5 * 1024 * 1024,
		PIDs:        10,
		Timestamp:   time.Now(),
	}, nil
}

// checkDependencies 检查依赖是否满足
func (o *Orchestrator) checkDependencies(dependencies []string) error {
	for _, depID := range dependencies {
		dep, exists := o.containers[depID]
		if !exists {
			return fmt.Errorf("dependency not found: %s", depID)
		}
		if dep.State != StateRunning {
			return fmt.Errorf("dependency not running: %s (state: %s)", depID, dep.State)
		}
	}
	return nil
}

// matchFilters 检查容器是否匹配过滤器
func (o *Orchestrator) matchFilters(c *ContainerInstance, filters map[string]string) bool {
	for k, v := range filters {
		switch k {
		case "state":
			if string(c.State) != v {
				return false
			}
		case "name":
			if c.Config.Name != v {
				return false
			}
		case "image":
			if c.Config.Image != v {
				return false
			}
		case "tag":
			found := false
			for _, tag := range c.Config.Tags {
				if tag == v {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// getContainerCountByName 获取同名容器数量
func (o *Orchestrator) getContainerCountByName(name string) int {
	count := 0
	for _, c := range o.containers {
		if c.Config.Name == name || len(c.Config.Name) > len(name)+1 && c.Config.Name[:len(name)+1] == name+"-" {
			count++
		}
	}
	return count
}

// getContainersByName 获取同名容器列表
func (o *Orchestrator) getContainersByName(name string) []*ContainerInstance {
	var result []*ContainerInstance
	for _, c := range o.containers {
		if c.Config.Name == name || len(c.Config.Name) > len(name)+1 && c.Config.Name[:len(name)+1] == name+"-" {
			result = append(result, c)
		}
	}
	return result
}

// startContainerProcess 启动容器进程
func (o *Orchestrator) startContainerProcess(ctx context.Context, container *ContainerInstance) error {
	// 模拟 LXC 容器启动
	// 实际实现需要调用 lxc-start 或类似命令
	o.logger.Debug("starting container process",
		zap.String("id", container.Config.ID),
		zap.String("image", container.Config.Image),
	)

	// 模拟启动延迟
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	return nil
}

// stopContainerProcess 停止容器进程
func (o *Orchestrator) stopContainerProcess(ctx context.Context, container *ContainerInstance) error {
	// 模拟 LXC 容器停止
	o.logger.Debug("stopping container process",
		zap.String("id", container.Config.ID),
	)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(50 * time.Millisecond):
	}

	return nil
}

// runHealthCheckLoop 运行健康检查循环
func (o *Orchestrator) runHealthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(o.config.HealthInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks 执行健康检查
func (o *Orchestrator) performHealthChecks(ctx context.Context) {
	o.mu.RLock()
	containers := make([]*ContainerInstance, 0, len(o.containers))
	for _, c := range o.containers {
		if c.State == StateRunning {
			containers = append(containers, c)
		}
	}
	o.mu.RUnlock()

	for _, c := range containers {
		result := o.checkContainerHealth(ctx, c)
		if !result.Healthy {
			o.logger.Warn("container unhealthy",
				zap.String("id", c.Config.ID),
				zap.String("message", result.Message),
			)

			// 自动重启
			if o.config.AutoRestart && c.Config.RestartPolicy == RestartAlways {
				o.logger.Info("auto-restarting container", zap.String("id", c.Config.ID))
				if err := o.RestartContainer(ctx, c.Config.ID); err != nil {
					o.logger.Error("auto-restart failed",
						zap.String("id", c.Config.ID),
						zap.Error(err),
					)
				}
			}
		}
	}
}

// checkContainerHealth 检查容器健康状态
func (o *Orchestrator) checkContainerHealth(ctx context.Context, container *ContainerInstance) *HealthCheckResult {
	// 模拟健康检查
	return &HealthCheckResult{
		ContainerID: container.Config.ID,
		Healthy:     true,
		Latency:     5,
		Timestamp:   time.Now(),
	}
}

// GetConfig 获取编排器配置
func (o *Orchestrator) GetConfig() *OrchestratorConfig {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.config
}

// UpdateConfig 更新编排器配置
func (o *Orchestrator) UpdateConfig(config *OrchestratorConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.config = config
	o.logger.Info("orchestrator config updated")
}
