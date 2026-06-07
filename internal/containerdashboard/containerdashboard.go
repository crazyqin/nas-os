package containerdashboard

import (
	"fmt"
	"sync"
	"time"
)

// ContainerStatus 容器状态
type ContainerStatus string

const (
	StatusCreated    ContainerStatus = "created"
	StatusRunning    ContainerStatus = "running"
	StatusPaused     ContainerStatus = "paused"
	StatusRestarting ContainerStatus = "restarting"
	StatusRemoving   ContainerStatus = "removing"
	StatusExited     ContainerStatus = "exited"
	StatusDead       ContainerStatus = "dead"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthStarting  HealthStatus = "starting"
	HealthNone      HealthStatus = "none"
)

// Container 容器信息
type Container struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Status       ContainerStatus   `json:"status"`
	Health       HealthStatus      `json:"health"`
	Ports        []PortMapping     `json:"ports,omitempty"`
	Networks     []string          `json:"networks,omitempty"`
	Volumes      []VolumeMount     `json:"volumes,omitempty"`
	Env          []string          `json:"env,omitempty"`
	Command      string            `json:"command,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	RestartCount int               `json:"restart_count"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// ResourceUsage 资源使用
type ResourceUsage struct {
	ContainerID   string    `json:"container_id"`
	Timestamp     time.Time `json:"timestamp"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsage   int64     `json:"memory_usage"`
	MemoryLimit   int64     `json:"memory_limit"`
	MemoryPercent float64   `json:"memory_percent"`
	NetworkRx     int64     `json:"network_rx"`
	NetworkTx     int64     `json:"network_tx"`
	BlockRead     int64     `json:"block_read"`
	BlockWrite    int64     `json:"block_write"`
	PIDs          int       `json:"pids"`
}

// ResourceTimeSeries 资源时间序列
type ResourceTimeSeries struct {
	ContainerID string          `json:"container_id"`
	Metrics     []ResourceUsage `json:"metrics"`
	StartTime   time.Time       `json:"start_time"`
	EndTime     time.Time       `json:"end_time"`
	Interval    time.Duration   `json:"interval"`
}

// ContainerLog 容器日志
type ContainerLog struct {
	ContainerID string    `json:"container_id"`
	Timestamp   time.Time `json:"timestamp"`
	Stream      string    `json:"stream"` // stdout, stderr
	Content     string    `json:"content"`
}

// LogStream 日志流
type LogStream struct {
	ContainerID string
	Logs        chan *ContainerLog
	Done        chan struct{}
}

// DeployTemplate 部署模板
type DeployTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Image       string            `json:"image"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	Volumes     []VolumeMount     `json:"volumes,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Command     string            `json:"command,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Resources   ResourceLimits    `json:"resources,omitempty"`
	Network     string            `json:"network,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	HealthCheck *HealthCheck      `json:"health_check,omitempty"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPUShares  int   `json:"cpu_shares,omitempty"`
	Memory     int64 `json:"memory,omitempty"`
	MemorySwap int64 `json:"memory_swap,omitempty"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	Test        []string      `json:"test"`
	Interval    time.Duration `json:"interval"`
	Timeout     time.Duration `json:"timeout"`
	Retries     int           `json:"retries"`
	StartPeriod time.Duration `json:"start_period"`
}

// NetworkTopology 网络拓扑
type NetworkTopology struct {
	Networks []NetworkInfo  `json:"networks"`
	Nodes    []TopologyNode `json:"nodes"`
	Edges    []TopologyEdge `json:"edges"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Subnet     string   `json:"subnet"`
	Gateway    string   `json:"gateway"`
	Containers []string `json:"containers"`
}

// TopologyNode 拓扑节点
type TopologyNode struct {
	ID     string `json:"id"`
	Type   string `json:"type"` // container, network
	Label  string `json:"label"`
	Status string `json:"status"`
}

// TopologyEdge 拓扑边
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
}

// DashboardStats 仪表盘统计
type DashboardStats struct {
	TotalContainers     int            `json:"total_containers"`
	RunningContainers   int            `json:"running_containers"`
	StoppedContainers   int            `json:"stopped_containers"`
	HealthyContainers   int            `json:"healthy_containers"`
	UnhealthyContainers int            `json:"unhealthy_containers"`
	TotalImages         int            `json:"total_images"`
	TotalNetworks       int            `json:"total_networks"`
	TotalVolumes        int            `json:"total_volumes"`
	ResourcesByStatus   map[string]int `json:"resources_by_status"`
	CPUUsageTotal       float64        `json:"cpu_usage_total"`
	MemoryUsageTotal    int64          `json:"memory_usage_total"`
}

// ContainerDashboard 容器仪表盘
type ContainerDashboard struct {
	mu         sync.RWMutex
	containers map[string]*Container
	resources  map[string][]ResourceUsage
	logs       map[string][]*ContainerLog
	logStreams map[string]*LogStream
	templates  map[string]*DeployTemplate
	networks   map[string]*NetworkInfo
	volumes    map[string]string
}

// NewContainerDashboard 创建容器仪表盘
func NewContainerDashboard() *ContainerDashboard {
	return &ContainerDashboard{
		containers: make(map[string]*Container),
		resources:  make(map[string][]ResourceUsage),
		logs:       make(map[string][]*ContainerLog),
		logStreams: make(map[string]*LogStream),
		templates:  make(map[string]*DeployTemplate),
		networks:   make(map[string]*NetworkInfo),
		volumes:    make(map[string]string),
	}
}

// RegisterContainer 注册容器
func (cd *ContainerDashboard) RegisterContainer(container *Container) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, exists := cd.containers[container.ID]; exists {
		return fmt.Errorf("容器 %s 已注册", container.ID)
	}

	container.CreatedAt = time.Now()
	cd.containers[container.ID] = container
	return nil
}

// UnregisterContainer 注销容器
func (cd *ContainerDashboard) UnregisterContainer(containerID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, exists := cd.containers[containerID]; !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	delete(cd.containers, containerID)
	delete(cd.resources, containerID)
	delete(cd.logs, containerID)
	return nil
}

// GetContainer 获取容器
func (cd *ContainerDashboard) GetContainer(containerID string) (*Container, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}
	return container, nil
}

// ListContainers 列出容器
func (cd *ContainerDashboard) ListContainers(status ContainerStatus) []*Container {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	containers := make([]*Container, 0)
	for _, container := range cd.containers {
		if status != "" && container.Status != status {
			continue
		}
		containers = append(containers, container)
	}
	return containers
}

// StartContainer 启动容器
func (cd *ContainerDashboard) StartContainer(containerID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if container.Status == StatusRunning {
		return fmt.Errorf("容器 %s 已在运行", containerID)
	}

	now := time.Now()
	container.Status = StatusRunning
	container.StartedAt = &now
	container.Health = HealthStarting
	return nil
}

// StopContainer 停止容器
func (cd *ContainerDashboard) StopContainer(containerID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if container.Status == StatusExited {
		return fmt.Errorf("容器 %s 已停止", containerID)
	}

	now := time.Now()
	container.Status = StatusExited
	container.FinishedAt = &now
	container.Health = HealthNone
	return nil
}

// RestartContainer 重启容器
func (cd *ContainerDashboard) RestartContainer(containerID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	container.Status = StatusRestarting
	container.RestartCount++

	// 模拟重启完成
	now := time.Now()
	container.Status = StatusRunning
	container.StartedAt = &now
	container.Health = HealthStarting
	return nil
}

// DeleteContainer 删除容器
func (cd *ContainerDashboard) DeleteContainer(containerID string, force bool) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	if container.Status == StatusRunning && !force {
		return fmt.Errorf("容器 %s 正在运行，请使用 force=true 强制删除", containerID)
	}

	delete(cd.containers, containerID)
	delete(cd.resources, containerID)
	delete(cd.logs, containerID)
	return nil
}

// UpdateResourceUsage 更新资源使用
func (cd *ContainerDashboard) UpdateResourceUsage(usage *ResourceUsage) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, exists := cd.containers[usage.ContainerID]; !exists {
		return fmt.Errorf("容器 %s 不存在", usage.ContainerID)
	}

	usage.Timestamp = time.Now()
	cd.resources[usage.ContainerID] = append(cd.resources[usage.ContainerID], *usage)

	// 保留最近1000条记录
	if len(cd.resources[usage.ContainerID]) > 1000 {
		cd.resources[usage.ContainerID] = cd.resources[usage.ContainerID][len(cd.resources[usage.ContainerID])-1000:]
	}
	return nil
}

// GetResourceUsage 获取资源使用
func (cd *ContainerDashboard) GetResourceUsage(containerID string) (*ResourceUsage, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	usages, exists := cd.resources[containerID]
	if !exists || len(usages) == 0 {
		return nil, fmt.Errorf("容器 %s 无资源数据", containerID)
	}

	return &usages[len(usages)-1], nil
}

// GetResourceTimeSeries 获取资源时间序列
func (cd *ContainerDashboard) GetResourceTimeSeries(containerID string, duration time.Duration) (*ResourceTimeSeries, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	if _, exists := cd.containers[containerID]; !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	usages, exists := cd.resources[containerID]
	if !exists {
		return &ResourceTimeSeries{
			ContainerID: containerID,
			Metrics:     []ResourceUsage{},
			StartTime:   time.Now().Add(-duration),
			EndTime:     time.Now(),
			Interval:    duration,
		}, nil
	}

	startTime := time.Now().Add(-duration)
	filtered := make([]ResourceUsage, 0)
	for _, usage := range usages {
		if usage.Timestamp.After(startTime) {
			filtered = append(filtered, usage)
		}
	}

	return &ResourceTimeSeries{
		ContainerID: containerID,
		Metrics:     filtered,
		StartTime:   startTime,
		EndTime:     time.Now(),
		Interval:    duration,
	}, nil
}

// AddContainerLog 添加容器日志
func (cd *ContainerDashboard) AddContainerLog(log *ContainerLog) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, exists := cd.containers[log.ContainerID]; !exists {
		return fmt.Errorf("容器 %s 不存在", log.ContainerID)
	}

	log.Timestamp = time.Now()
	cd.logs[log.ContainerID] = append(cd.logs[log.ContainerID], log)

	// 保留最近10000条日志
	if len(cd.logs[log.ContainerID]) > 10000 {
		cd.logs[log.ContainerID] = cd.logs[log.ContainerID][len(cd.logs[log.ContainerID])-10000:]
	}

	// 推送到日志流
	if stream, exists := cd.logStreams[log.ContainerID]; exists {
		select {
		case stream.Logs <- log:
		default:
			// 流满了丢弃
		}
	}
	return nil
}

// GetContainerLogs 获取容器日志
func (cd *ContainerDashboard) GetContainerLogs(containerID string, tail int) ([]*ContainerLog, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	if _, exists := cd.containers[containerID]; !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	logs, exists := cd.logs[containerID]
	if !exists {
		return []*ContainerLog{}, nil
	}

	if tail <= 0 || tail > len(logs) {
		tail = len(logs)
	}

	return logs[len(logs)-tail:], nil
}

// StreamLogs 创建日志流
func (cd *ContainerDashboard) StreamLogs(containerID string) (*LogStream, error) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, exists := cd.containers[containerID]; !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	if _, exists := cd.logStreams[containerID]; exists {
		return nil, fmt.Errorf("容器 %s 日志流已存在", containerID)
	}

	stream := &LogStream{
		ContainerID: containerID,
		Logs:        make(chan *ContainerLog, 100),
		Done:        make(chan struct{}),
	}

	cd.logStreams[containerID] = stream
	return stream, nil
}

// StopLogStream 停止日志流
func (cd *ContainerDashboard) StopLogStream(containerID string) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if stream, exists := cd.logStreams[containerID]; exists {
		close(stream.Done)
		delete(cd.logStreams, containerID)
	}
}

// RegisterTemplate 注册部署模板
func (cd *ContainerDashboard) RegisterTemplate(template *DeployTemplate) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, exists := cd.templates[template.ID]; exists {
		return fmt.Errorf("模板 %s 已存在", template.ID)
	}

	cd.templates[template.ID] = template
	return nil
}

// GetTemplate 获取部署模板
func (cd *ContainerDashboard) GetTemplate(templateID string) (*DeployTemplate, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	template, exists := cd.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}
	return template, nil
}

// ListTemplates 列出部署模板
func (cd *ContainerDashboard) ListTemplates(category string) []*DeployTemplate {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	templates := make([]*DeployTemplate, 0)
	for _, template := range cd.templates {
		if category != "" && template.Category != category {
			continue
		}
		templates = append(templates, template)
	}
	return templates
}

// DeployFromTemplate 从模板部署容器
func (cd *ContainerDashboard) DeployFromTemplate(templateID string, name string, overrides map[string]string) (*Container, error) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	template, exists := cd.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}

	containerID := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	env := make([]string, 0)
	for k, v := range template.Env {
		if override, ok := overrides[k]; ok {
			v = override
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	container := &Container{
		ID:       containerID,
		Name:     name,
		Image:    template.Image,
		Status:   StatusCreated,
		Health:   HealthNone,
		Ports:    template.Ports,
		Volumes:  template.Volumes,
		Env:      env,
		Command:  template.Command,
		Networks: []string{template.Network},
		Labels:   template.Labels,
	}

	cd.containers[containerID] = container
	return container, nil
}

// CheckContainerHealth 检查容器健康状态
func (cd *ContainerDashboard) CheckContainerHealth(containerID string) (*HealthStatus, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	if container.Status != StatusRunning {
		health := HealthNone
		return &health, nil
	}

	// 检查资源使用
	usages, exists := cd.resources[containerID]
	if !exists || len(usages) == 0 {
		health := HealthStarting
		return &health, nil
	}

	latest := usages[len(usages)-1]

	// 简单健康检查：CPU或内存使用率过高
	if latest.CPUPercent > 95 || latest.MemoryPercent > 95 {
		health := HealthUnhealthy
		return &health, nil
	}

	health := HealthHealthy
	return &health, nil
}

// UpdateContainerHealth 更新容器健康状态
func (cd *ContainerDashboard) UpdateContainerHealth(containerID string, health HealthStatus) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	container.Health = health
	return nil
}

// RegisterNetwork 注册网络
func (cd *ContainerDashboard) RegisterNetwork(network *NetworkInfo) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if _, exists := cd.networks[network.ID]; exists {
		return fmt.Errorf("网络 %s 已存在", network.ID)
	}

	cd.networks[network.ID] = network
	return nil
}

// GetNetworkTopology 获取网络拓扑
func (cd *ContainerDashboard) GetNetworkTopology() *NetworkTopology {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	topology := &NetworkTopology{
		Networks: make([]NetworkInfo, 0),
		Nodes:    make([]TopologyNode, 0),
		Edges:    make([]TopologyEdge, 0),
	}

	// 添加网络节点
	for _, network := range cd.networks {
		topology.Networks = append(topology.Networks, *network)
		topology.Nodes = append(topology.Nodes, TopologyNode{
			ID:    network.ID,
			Type:  "network",
			Label: network.Name,
		})

		// 添加容器节点和边
		for _, containerID := range network.Containers {
			if container, exists := cd.containers[containerID]; exists {
				topology.Nodes = append(topology.Nodes, TopologyNode{
					ID:     containerID,
					Type:   "container",
					Label:  container.Name,
					Status: string(container.Status),
				})
				topology.Edges = append(topology.Edges, TopologyEdge{
					Source: containerID,
					Target: network.ID,
					Label:  network.Name,
				})
			}
		}
	}

	return topology
}

// ConnectContainerToNetwork 连接容器到网络
func (cd *ContainerDashboard) ConnectContainerToNetwork(containerID, networkID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	network, exists := cd.networks[networkID]
	if !exists {
		return fmt.Errorf("网络 %s 不存在", networkID)
	}

	// 检查是否已连接
	for _, cid := range network.Containers {
		if cid == containerID {
			return nil
		}
	}

	network.Containers = append(network.Containers, containerID)
	container.Networks = append(container.Networks, networkID)
	return nil
}

// DisconnectContainerFromNetwork 断开容器与网络的连接
func (cd *ContainerDashboard) DisconnectContainerFromNetwork(containerID, networkID string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 不存在", containerID)
	}

	network, exists := cd.networks[networkID]
	if !exists {
		return fmt.Errorf("网络 %s 不存在", networkID)
	}

	// 从网络中移除容器
	for i, cid := range network.Containers {
		if cid == containerID {
			network.Containers = append(network.Containers[:i], network.Containers[i+1:]...)
			break
		}
	}

	// 从容器中移除网络
	for i, nid := range container.Networks {
		if nid == networkID {
			container.Networks = append(container.Networks[:i], container.Networks[i+1:]...)
			break
		}
	}

	return nil
}

// GetDashboardStats 获取仪表盘统计
func (cd *ContainerDashboard) GetDashboardStats() *DashboardStats {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	stats := &DashboardStats{
		ResourcesByStatus: make(map[string]int),
	}

	for _, container := range cd.containers {
		stats.TotalContainers++
		stats.ResourcesByStatus[string(container.Status)]++

		switch container.Status {
		case StatusRunning:
			stats.RunningContainers++
		case StatusExited, StatusDead:
			stats.StoppedContainers++
		}

		switch container.Health {
		case HealthHealthy:
			stats.HealthyContainers++
		case HealthUnhealthy:
			stats.UnhealthyContainers++
		}
	}

	stats.TotalNetworks = len(cd.networks)

	// 计算资源使用
	for _, usages := range cd.resources {
		if len(usages) > 0 {
			latest := usages[len(usages)-1]
			stats.CPUUsageTotal += latest.CPUPercent
			stats.MemoryUsageTotal += latest.MemoryUsage
		}
	}

	return stats
}

// GetContainerStats 获取单个容器统计
func (cd *ContainerDashboard) GetContainerStats(containerID string) (map[string]interface{}, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	container, exists := cd.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 不存在", containerID)
	}

	stats := map[string]interface{}{
		"container": container,
	}

	usages, exists := cd.resources[containerID]
	if exists && len(usages) > 0 {
		latest := usages[len(usages)-1]
		stats["current_cpu"] = latest.CPUPercent
		stats["current_memory"] = latest.MemoryUsage
		stats["current_memory_percent"] = latest.MemoryPercent
		stats["current_network_rx"] = latest.NetworkRx
		stats["current_network_tx"] = latest.NetworkTx
		stats["current_block_read"] = latest.BlockRead
		stats["current_block_write"] = latest.BlockWrite
		stats["current_pids"] = latest.PIDs
	}

	logs, exists := cd.logs[containerID]
	if exists {
		stats["log_count"] = len(logs)
	}

	return stats, nil
}

// PruneStoppedContainers 清理已停止的容器
func (cd *ContainerDashboard) PruneStoppedContainers() int {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	count := 0
	for id, container := range cd.containers {
		if container.Status == StatusExited || container.Status == StatusDead {
			delete(cd.containers, id)
			delete(cd.resources, id)
			delete(cd.logs, id)
			count++
		}
	}

	return count
}
