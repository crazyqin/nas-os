// Package containerorchpro 容器编排增强
// 对标 TrueNAS 容器 HA 和群晖容器管理
package containerorchpro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// StackStatus 堆栈状态
type StackStatus string

const (
	StackStatusRunning  StackStatus = "running"
	StackStatusStopped  StackStatus = "stopped"
	StackStatusDegraded StackStatus = "degraded"
	StackStatusError    StackStatus = "error"
)

// ServiceStatus 服务状态
type ServiceStatus string

const (
	ServiceStatusRunning  ServiceStatus = "running"
	ServiceStatusStopped  ServiceStatus = "stopped"
	ServiceStatusStarting ServiceStatus = "starting"
	ServiceStatusError    ServiceStatus = "error"
)

// RestartPolicy 重启策略
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartUnless    RestartPolicy = "unless-stopped"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartNo        RestartPolicy = "no"
)

// HealthCheck 健康检查配置
type HealthCheck struct {
	Test        []string      `json:"test"`
	Interval    time.Duration `json:"interval"`
	Timeout     time.Duration `json:"timeout"`
	Retries     int           `json:"retries"`
	StartPeriod time.Duration `json:"start_period"`
}

// ResourceLimit 资源限制
type ResourceLimit struct {
	CPU    float64 `json:"cpu"`    // CPU 核数
	Memory string  `json:"memory"` // 内存限制
	IO     *IOLimit `json:"io,omitempty"`
}

// IOLimit IO 限制
type IOLimit struct {
	ReadBPS  string `json:"read_bps"`
	WriteBPS string `json:"write_bps"`
	ReadIOPS int    `json:"read_iops"`
	WriteIOPS int   `json:"write_iops"`
}

// Service 容器服务定义
type Service struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Tag           string            `json:"tag"`
	Ports         []PortMapping     `json:"ports,omitempty"`
	Volumes       []VolumeMount     `json:"volumes,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Entrypoint    []string          `json:"entrypoint,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	User          string            `json:"user,omitempty"`
	Restart       RestartPolicy     `json:"restart"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
	Resources     *ResourceLimit    `json:"resources,omitempty"`
	DependsOn     []string          `json:"depends_on,omitempty"`
	Networks      []string          `json:"networks,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Privileged    bool              `json:"privileged"`
	ReadOnly      bool              `json:"read_only"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp, udp
	HostIP        string `json:"host_ip,omitempty"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
	Type          string `json:"type"` // bind, volume, tmpfs
}

// Stack 容器堆栈
type Stack struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Services    []Service   `json:"services"`
	Networks    []Network   `json:"networks,omitempty"`
	Volumes     []Volume    `json:"volumes,omitempty"`
	Status      StackStatus `json:"status"`
	ComposeFile string      `json:"compose_file,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
}

// Network 网络定义
type Network struct {
	Name     string            `json:"name"`
	Driver   string            `json:"driver"` // bridge, overlay, host
	Internal bool              `json:"internal"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// Volume 卷定义
type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"` // local, nfs, cifs
	DriverOpts map[string]string `json:"driver_opts,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// ServiceInstance 服务实例
type ServiceInstance struct {
	ID          string        `json:"id"`
	ServiceName string        `json:"service_name"`
	StackID     string        `json:"stack_id"`
	NodeID      string        `json:"node_id"`
	Status      ServiceStatus `json:"status"`
	Health      string        `json:"health"` // healthy, unhealthy, starting
	IP          string        `json:"ip"`
	Ports       []int         `json:"ports"`
	StartedAt   time.Time     `json:"started_at"`
	RestartCount int          `json:"restart_count"`
	CPUUsage    float64       `json:"cpu_usage"`
	MemoryUsage int64         `json:"memory_usage"`
}

// ClusterNode 集群节点
type ClusterNode struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IP          string    `json:"ip"`
	Status      string    `json:"status"` // online, offline, draining
	Role        string    `json:"role"`   // manager, worker
	Labels      map[string]string `json:"labels,omitempty"`
	Resources   NodeResources `json:"resources"`
	LastSeen    time.Time `json:"last_seen"`
}

// NodeResources 节点资源
type NodeResources struct {
	CPU     int   `json:"cpu"`      // CPU 核数
	Memory  int64 `json:"memory"`   // 内存字节
	Disk    int64 `json:"disk"`     // 磁盘字节
	GPU     int   `json:"gpu"`      // GPU 数量
}

// Manager 容器编排管理器
type Manager struct {
	mu        sync.RWMutex
	stacks    map[string]*Stack
	instances map[string][]*ServiceInstance
	nodes     map[string]*ClusterNode
	logger    Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建容器编排管理器
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		stacks:    make(map[string]*Stack),
		instances: make(map[string][]*ServiceInstance),
		nodes:     make(map[string]*ClusterNode),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}

	// 启动健康检查循环
	m.wg.Add(1)
	go m.healthCheckLoop()

	// 启动资源监控循环
	m.wg.Add(1)
	go m.resourceMonitorLoop()

	return m
}

// CreateStack 创建堆栈
func (m *Manager) CreateStack(stack *Stack) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stack.ID == "" {
		stack.ID = generateStackID()
	}
	stack.CreatedAt = time.Now()
	stack.UpdatedAt = time.Now()
	stack.Status = StackStatusStopped

	// 验证服务依赖
	if err := m.validateDependencies(stack); err != nil {
		return err
	}

	m.stacks[stack.ID] = stack
	m.logger.Info("堆栈创建成功: %s (%s)", stack.Name, stack.ID)
	return nil
}

// validateDependencies 验证服务依赖
func (m *Manager) validateDependencies(stack *Stack) error {
	serviceNames := make(map[string]bool)
	for _, svc := range stack.Services {
		serviceNames[svc.Name] = true
	}

	for _, svc := range stack.Services {
		for _, dep := range svc.DependsOn {
			if !serviceNames[dep] {
				return fmt.Errorf("服务 %s 依赖的服务 %s 不存在", svc.Name, dep)
			}
		}
	}
	return nil
}

// StartStack 启动堆栈
func (m *Manager) StartStack(ctx context.Context, stackID string) error {
	m.mu.Lock()
	stack, ok := m.stacks[stackID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("堆栈不存在: %s", stackID)
	}
	m.mu.Unlock()

	// 按依赖顺序启动服务
	startOrder := m.calculateStartOrder(stack)
	for _, svcName := range startOrder {
		for _, svc := range stack.Services {
			if svc.Name == svcName {
				if err := m.startService(ctx, stackID, &svc); err != nil {
					m.logger.Error("启动服务失败 %s: %v", svcName, err)
					return err
				}
				break
			}
		}
	}

	m.mu.Lock()
	stack.Status = StackStatusRunning
	now := time.Now()
	stack.StartedAt = &now
	stack.UpdatedAt = now
	m.mu.Unlock()

	m.logger.Info("堆栈启动成功: %s", stack.Name)
	return nil
}

// calculateStartOrder 计算服务启动顺序
func (m *Manager) calculateStartOrder(stack *Stack) []string {
	// 拓扑排序
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for _, svc := range stack.Services {
		if _, ok := inDegree[svc.Name]; !ok {
			inDegree[svc.Name] = 0
		}
		for _, dep := range svc.DependsOn {
			graph[dep] = append(graph[dep], svc.Name)
			inDegree[svc.Name]++
		}
	}

	// BFS 拓扑排序
	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	order := make([]string, 0)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, neighbor := range graph[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	return order
}

// startService 启动单个服务
func (m *Manager) startService(ctx context.Context, stackID string, svc *Service) error {
	// 模拟启动服务
	instance := &ServiceInstance{
		ID:          generateInstanceID(),
		ServiceName: svc.Name,
		StackID:     stackID,
		Status:      ServiceStatusRunning,
		Health:      "starting",
		StartedAt:   time.Now(),
	}

	m.mu.Lock()
	m.instances[stackID] = append(m.instances[stackID], instance)
	m.mu.Unlock()

	m.logger.Info("服务启动成功: %s", svc.Name)
	return nil
}

// StopStack 停止堆栈
func (m *Manager) StopStack(ctx context.Context, stackID string) error {
	m.mu.Lock()
	stack, ok := m.stacks[stackID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("堆栈不存在: %s", stackID)
	}

	// 停止所有服务实例
	instances := m.instances[stackID]
	m.mu.Unlock()

	for _, instance := range instances {
		instance.Status = ServiceStatusStopped
		m.logger.Info("服务停止: %s", instance.ServiceName)
	}

	m.mu.Lock()
	stack.Status = StackStatusStopped
	stack.UpdatedAt = time.Now()
	m.mu.Unlock()

	m.logger.Info("堆栈停止成功: %s", stack.Name)
	return nil
}

// ScaleService 扩缩容服务
func (m *Manager) ScaleService(ctx context.Context, stackID, serviceName string, replicas int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stack, ok := m.stacks[stackID]
	if !ok {
		return fmt.Errorf("堆栈不存在: %s", stackID)
	}

	// 查找服务
	var targetService *Service
	for i, svc := range stack.Services {
		if svc.Name == serviceName {
			targetService = &stack.Services[i]
			break
		}
	}
	if targetService == nil {
		return fmt.Errorf("服务不存在: %s", serviceName)
	}

	// 计算当前实例数
	currentInstances := 0
	for _, inst := range m.instances[stackID] {
		if inst.ServiceName == serviceName && inst.Status == ServiceStatusRunning {
			currentInstances++
		}
	}

	if replicas > currentInstances {
		// 扩容
		for i := 0; i < replicas-currentInstances; i++ {
			instance := &ServiceInstance{
				ID:          generateInstanceID(),
				ServiceName: serviceName,
				StackID:     stackID,
				Status:      ServiceStatusRunning,
				Health:      "starting",
				StartedAt:   time.Now(),
			}
			m.instances[stackID] = append(m.instances[stackID], instance)
		}
		m.logger.Info("服务扩容: %s, 从 %d 到 %d", serviceName, currentInstances, replicas)
	} else if replicas < currentInstances {
		// 缩容
		toRemove := currentInstances - replicas
		removed := 0
		newInstances := make([]*ServiceInstance, 0)
		for _, inst := range m.instances[stackID] {
			if inst.ServiceName == serviceName && removed < toRemove {
				removed++
				continue
			}
			newInstances = append(newInstances, inst)
		}
		m.instances[stackID] = newInstances
		m.logger.Info("服务缩容: %s, 从 %d 到 %d", serviceName, currentInstances, replicas)
	}

	stack.UpdatedAt = time.Now()
	return nil
}

// GetStackStatus 获取堆栈状态
func (m *Manager) GetStackStatus(stackID string) (*Stack, map[string][]*ServiceInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stack, ok := m.stacks[stackID]
	if !ok {
		return nil, nil, fmt.Errorf("堆栈不存在: %s", stackID)
	}

	instances := make(map[string][]*ServiceInstance)
	instances[stackID] = m.instances[stackID]
	return stack, instances, nil
}

// ListStacks 列出所有堆栈
func (m *Manager) ListStacks() []*Stack {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stacks := make([]*Stack, 0, len(m.stacks))
	for _, stack := range m.stacks {
		stacks = append(stacks, stack)
	}
	return stacks
}

// RegisterNode 注册集群节点
func (m *Manager) RegisterNode(node *ClusterNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node.ID == "" {
		node.ID = generateNodeID()
	}
	node.LastSeen = time.Now()
	m.nodes[node.ID] = node
	m.logger.Info("节点注册成功: %s (%s)", node.Name, node.ID)
	return nil
}

// GetClusterNodes 获取集群节点
func (m *Manager) GetClusterNodes() []*ClusterNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*ClusterNode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// healthCheckLoop 健康检查循环
func (m *Manager) healthCheckLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthChecks()
		}
	}
}

// performHealthChecks 执行健康检查
func (m *Manager) performHealthChecks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for stackID, instances := range m.instances {
		stack, ok := m.stacks[stackID]
		if !ok {
			continue
		}

		for _, instance := range instances {
			if instance.Status != ServiceStatusRunning {
				continue
			}

			// 查找服务健康检查配置
			for _, svc := range stack.Services {
				if svc.Name == instance.ServiceName && svc.HealthCheck != nil {
					// 执行健康检查
					healthy := m.checkInstanceHealth(instance)
					if healthy {
						instance.Health = "healthy"
					} else {
						instance.Health = "unhealthy"
						instance.RestartCount++
					}
					break
				}
			}
		}
	}
}

// checkInstanceHealth 检查实例健康状态
func (m *Manager) checkInstanceHealth(instance *ServiceInstance) bool {
	// 模拟健康检查
	return true
}

// resourceMonitorLoop 资源监控循环
func (m *Manager) resourceMonitorLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateResourceUsage()
		}
	}
}

// updateResourceUsage 更新资源使用情况
func (m *Manager) updateResourceUsage() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟更新资源使用
	for _, instances := range m.instances {
		for _, instance := range instances {
			if instance.Status == ServiceStatusRunning {
				// 模拟 CPU 和内存使用
				instance.CPUUsage = 10.5
				instance.MemoryUsage = 1024 * 1024 * 100 // 100MB
			}
		}
	}
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

func generateStackID() string {
	return fmt.Sprintf("stack_%d", time.Now().UnixNano())
}

func generateInstanceID() string {
	return fmt.Sprintf("instance_%d", time.Now().UnixNano())
}

func generateNodeID() string {
	return fmt.Sprintf("node_%d", time.Now().UnixNano())
}

// RegisterHandlers 注册 HTTP 处理器
func (m *Manager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/stacks", m.handleStacks)
	mux.HandleFunc("/api/stacks/start", m.handleStartStack)
	mux.HandleFunc("/api/stacks/stop", m.handleStopStack)
	mux.HandleFunc("/api/stacks/scale", m.handleScaleService)
	mux.HandleFunc("/api/stacks/status", m.handleStackStatus)
	mux.HandleFunc("/api/nodes", m.handleNodes)
}

func (m *Manager) handleStacks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		stacks := m.ListStacks()
		writeJSON(w, stacks)
	case http.MethodPost:
		var stack Stack
		if err := json.NewDecoder(r.Body).Decode(&stack); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateStack(&stack); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, stack)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleStartStack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StackID string `json:"stack_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.StartStack(r.Context(), req.StackID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "started"})
}

func (m *Manager) handleStopStack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StackID string `json:"stack_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.StopStack(r.Context(), req.StackID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (m *Manager) handleScaleService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		StackID     string `json:"stack_id"`
		ServiceName string `json:"service_name"`
		Replicas    int    `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.ScaleService(r.Context(), req.StackID, req.ServiceName, req.Replicas); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "scaled"})
}

func (m *Manager) handleStackStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stackID := r.URL.Query().Get("stack_id")
	if stackID == "" {
		http.Error(w, "stack_id is required", http.StatusBadRequest)
		return
	}

	stack, instances, err := m.GetStackStatus(stackID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]interface{}{
		"stack":     stack,
		"instances": instances,
	})
}

func (m *Manager) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes := m.GetClusterNodes()
	writeJSON(w, nodes)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
