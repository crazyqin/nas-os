// Package containerorch 提供容器编排核心业务逻辑
package containerorch

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 常量 ==========

const (
	// 默认健康检查间隔
	DefaultHealthCheckInterval = 30 * time.Second
	// 默认健康检查超时
	DefaultHealthCheckTimeout = 10 * time.Second
	// 默认重启次数限制
	DefaultMaxRestarts = 3
	// 默认扩缩容冷却时间
	DefaultScaleCooldown = 5 * time.Minute
	// 最大实例数
	MaxInstances = 100
)

// ========== 错误类型 ==========

// NotFoundError 资源未找到错误
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

// DependencyError 依赖错误
type DependencyError struct {
	Message string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependency error: %s", e.Message)
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ScaleError 扩缩容错误
type ScaleError struct {
	Service string
	Message string
}

func (e *ScaleError) Error() string {
	return fmt.Sprintf("scale error for service %s: %s", e.Service, e.Message)
}

// ========== Manager ==========

// Manager 容器编排管理器
type Manager struct {
	projects     map[string]*OrchestrationProject
	healthChecks map[string]*HealthReport
	scaleEvents  []AutoScaleEvent
	logStreams   map[string]*LogStream
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	stopCh       chan struct{}
}

// NewManager 创建容器编排管理器
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		projects:     make(map[string]*OrchestrationProject),
		healthChecks: make(map[string]*HealthReport),
		scaleEvents:  make([]AutoScaleEvent, 0),
		logStreams:   make(map[string]*LogStream),
		ctx:          ctx,
		cancel:       cancel,
		stopCh:       make(chan struct{}),
	}
}

// ========== 项目 CRUD ==========

// CreateProject 创建编排项目
func (m *Manager) CreateProject(req CreateProjectRequest) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证服务名称
	for name := range req.Services {
		if !isValidServiceName(name) {
			return nil, &ValidationError{
				Field:   "services",
				Message: fmt.Sprintf("invalid service name: %s", name),
			}
		}
	}

	// 设置默认值
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	// 初始化服务状态
	for name, svc := range req.Services {
		svc.Name = name
		svc.Status = ServiceStatusPending
		if svc.DesiredCount == 0 {
			svc.DesiredCount = 1
		}
		if svc.Deploy != nil && svc.Deploy.Replicas > 0 {
			svc.DesiredCount = svc.Deploy.Replicas
		}
		if svc.Resources == nil {
			svc.Resources = &ResourceLimits{
				CPU:    &CPULimit{Cores: 1, Shares: 1024},
				Memory: &MemoryLimit{Limit: 512 * 1024 * 1024}, // 512MB
			}
		}
	}

	project := &OrchestrationProject{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Namespace:   req.Namespace,
		Services:    req.Services,
		Networks:    req.Networks,
		Volumes:     req.Volumes,
		Status:      ProjectStatusCreating,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Labels:      req.Labels,
	}

	// 验证依赖关系
	if err := m.validateDependencies(project); err != nil {
		return nil, err
	}

	m.projects[project.ID] = project
	return project, nil
}

// GetProject 获取项目
func (m *Manager) GetProject(id string) (*OrchestrationProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: id}
	}
	return project, nil
}

// ListProjects 列出所有项目
func (m *Manager) ListProjects(namespace string) []*OrchestrationProject {
	m.mu.RLock()
	defer m.mu.RUnlock()

	projects := make([]*OrchestrationProject, 0, len(m.projects))
	for _, p := range m.projects {
		if namespace == "" || p.Namespace == namespace {
			projects = append(projects, p)
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].CreatedAt.After(projects[j].CreatedAt)
	})

	return projects
}

// UpdateProject 更新项目
func (m *Manager) UpdateProject(id string, req UpdateProjectRequest) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: id}
	}

	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.Services != nil {
		// 初始化新服务
		for name, svc := range req.Services {
			svc.Name = name
			if existing, ok := project.Services[name]; ok {
				// 保留现有状态
				svc.Status = existing.Status
				svc.ContainerIDs = existing.ContainerIDs
				svc.Instances = existing.Instances
			} else {
				svc.Status = ServiceStatusPending
			}
			if svc.Deploy != nil && svc.Deploy.Replicas > 0 {
				svc.DesiredCount = svc.Deploy.Replicas
			} else {
				svc.DesiredCount = 1
			}
		}
		project.Services = req.Services
	}
	if req.Networks != nil {
		project.Networks = req.Networks
	}
	if req.Volumes != nil {
		project.Volumes = req.Volumes
	}
	if req.Labels != nil {
		project.Labels = req.Labels
	}

	project.UpdatedAt = time.Now()
	project.Status = ProjectStatusUpdating

	return project, nil
}

// DeleteProject 删除项目
func (m *Manager) DeleteProject(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[id]
	if !ok {
		return &NotFoundError{Resource: "project", ID: id}
	}

	// 检查是否所有服务已停止
	if !force && project.Status == ProjectStatusRunning {
		return &ValidationError{
			Field:   "status",
			Message: "project is still running, use force=true to delete",
		}
	}

	// 清理资源
	delete(m.projects, id)
	delete(m.healthChecks, id)
	return nil
}

// ========== 服务生命周期 ==========

// StartProject 启动项目
func (m *Manager) StartProject(id string) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: id}
	}

	// 计算启动顺序
	order, err := m.calculateStartupOrder(project)
	if err != nil {
		return nil, err
	}

	// 按阶段启动服务
	project.Status = ProjectStatusRunning
	for _, stage := range order.Stages {
		for _, serviceName := range stage {
			svc, ok := project.Services[serviceName]
			if !ok {
				continue
			}
			svc.Status = ServiceStatusRunning
			svc.Instances = svc.DesiredCount
			// 生成模拟容器 ID
			for i := 0; i < svc.Instances; i++ {
				containerID := uuid.New().String()[:12]
				svc.ContainerIDs = append(svc.ContainerIDs, containerID)
			}
		}
	}

	project.UpdatedAt = time.Now()
	return project, nil
}

// StopProject 停止项目
func (m *Manager) StopProject(id string) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: id}
	}

	project.Status = ProjectStatusStopped
	for _, svc := range project.Services {
		svc.Status = ServiceStatusStopped
		svc.ContainerIDs = nil
		svc.Instances = 0
	}

	project.UpdatedAt = time.Now()
	return project, nil
}

// RestartProject 重启项目
func (m *Manager) RestartProject(id string) (*OrchestrationProject, error) {
	if _, err := m.StopProject(id); err != nil {
		return nil, err
	}
	return m.StartProject(id)
}

// ScaleService 扩缩容服务
func (m *Manager) ScaleService(projectID, serviceName string, replicas int) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if replicas < 0 {
		return nil, &ScaleError{
			Service: serviceName,
			Message: "replicas cannot be negative",
		}
	}
	if replicas > MaxInstances {
		return nil, &ScaleError{
			Service: serviceName,
			Message: fmt.Sprintf("replicas cannot exceed %d", MaxInstances),
		}
	}

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	svc, ok := project.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "service", ID: serviceName}
	}

	oldCount := svc.Instances
	svc.DesiredCount = replicas
	svc.Instances = replicas
	svc.Status = ServiceStatusRunning

	// 更新容器 ID
	if replicas > oldCount {
		// 扩容：添加新容器
		for i := oldCount; i < replicas; i++ {
			containerID := uuid.New().String()[:12]
			svc.ContainerIDs = append(svc.ContainerIDs, containerID)
		}
	} else if replicas < oldCount {
		// 缩容：移除多余容器
		svc.ContainerIDs = svc.ContainerIDs[:replicas]
	}

	project.UpdatedAt = time.Now()

	// 记录扩缩容事件
	event := AutoScaleEvent{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		ServiceName: serviceName,
		Action:      "scale",
		From:        oldCount,
		To:          replicas,
		Reason:      "manual",
		Timestamp:   time.Now(),
		Success:     true,
	}
	m.scaleEvents = append(m.scaleEvents, event)

	return svc, nil
}

// ========== 依赖管理 ==========

// validateDependencies 验证服务依赖
func (m *Manager) validateDependencies(project *OrchestrationProject) error {
	graph := m.buildDependencyGraph(project)

	// 检查循环依赖
	sorter := &TopologicalSorter{
		graph:    graph,
		visited:  make(map[string]bool),
		recStack: make(map[string]bool),
	}

	if sorter.HasCycle() {
		return &DependencyError{
			Message: fmt.Sprintf("circular dependency detected: %s", strings.Join(sorter.CyclePath(), " -> ")),
		}
	}

	return nil
}

// buildDependencyGraph 构建依赖图
func (m *Manager) buildDependencyGraph(project *OrchestrationProject) *DependencyGraph {
	graph := &DependencyGraph{
		Nodes: make(map[string]*DependencyNode),
		Edges: make(map[string][]string),
	}

	// 添加节点
	for name, svc := range project.Services {
		node := &DependencyNode{
			Name:    name,
			Service: svc,
		}
		graph.Nodes[name] = node
		graph.Edges[name] = make([]string, 0)
	}

	// 添加边
	for name, svc := range project.Services {
		for _, dep := range svc.DependsOn {
			if _, ok := project.Services[dep.ServiceName]; ok {
				graph.Edges[name] = append(graph.Edges[name], dep.ServiceName)
				graph.Nodes[dep.ServiceName].Dependents = append(
					graph.Nodes[dep.ServiceName].Dependents, name)
			}
		}
	}

	return graph
}

// calculateStartupOrder 计算启动顺序
func (m *Manager) calculateStartupOrder(project *OrchestrationProject) (*StartupOrder, error) {
	graph := m.buildDependencyGraph(project)

	// 使用拓扑排序
	order, err := m.topologicalSort(graph)
	if err != nil {
		return nil, err
	}

	// 按层级分组
	levels := m.calculateLevels(graph, order)

	startupOrder := &StartupOrder{
		Stages: levels,
		Total:  len(order),
	}

	return startupOrder, nil
}

// topologicalSort 拓扑排序
func (m *Manager) topologicalSort(graph *DependencyGraph) ([]string, error) {
	// 计算入度
	inDegree := make(map[string]int)
	for name := range graph.Nodes {
		inDegree[name] = 0
	}
	for _, deps := range graph.Edges {
		for _, dep := range deps {
			inDegree[dep]++
		}
	}

	// 使用 Kahn 算法
	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	result := make([]string, 0, len(graph.Nodes))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// 更新依赖此节点的节点的入度
		for _, dep := range graph.Edges[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// 检查是否有循环
	if len(result) != len(graph.Nodes) {
		return nil, &DependencyError{
			Message: "circular dependency detected",
		}
	}

	return result, nil
}

// calculateLevels 计算层级
func (m *Manager) calculateLevels(graph *DependencyGraph, order []string) [][]string {
	levels := make([][]string, 0)
	levelMap := make(map[string]int)

	// 计算每个节点的层级
	for _, node := range order {
		maxLevel := -1
		for _, dep := range graph.Edges[node] {
			if level, ok := levelMap[dep]; ok && level > maxLevel {
				maxLevel = level
			}
		}
		levelMap[node] = maxLevel + 1

		// 确保有足够的层级
		for len(levels) <= levelMap[node] {
			levels = append(levels, make([]string, 0))
		}
		levels[levelMap[node]] = append(levels[levelMap[node]], node)
	}

	return levels
}

// HasCycle 检查是否有循环依赖
func (s *TopologicalSorter) HasCycle() bool {
	for name := range s.graph.Nodes {
		if !s.visited[name] {
			if s.detectCycle(name) {
				return true
			}
		}
	}
	return false
}

// detectCycle 检测循环
func (s *TopologicalSorter) detectCycle(node string) bool {
	s.visited[node] = true
	s.recStack[node] = true

	for _, dep := range s.graph.Edges[node] {
		if !s.visited[dep] {
			if s.detectCycle(dep) {
				s.cyclePath = append([]string{node}, s.cyclePath...)
				return true
			}
		} else if s.recStack[dep] {
			s.cyclePath = []string{dep, node}
			return true
		}
	}

	s.recStack[node] = false
	return false
}

// CyclePath 获取循环路径
func (s *TopologicalSorter) CyclePath() []string {
	return s.cyclePath
}

// GetStartupOrder 获取启动顺序
func (m *Manager) GetStartupOrder(projectID string) (*StartupOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	return m.calculateStartupOrder(project)
}

// ========== 健康检查 ==========

// GetHealthReport 获取健康报告
func (m *Manager) GetHealthReport(projectID string) (*HealthReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	report, ok := m.healthChecks[projectID]
	if !ok {
		// 创建新的健康报告
		report = &HealthReport{
			ProjectID: projectID,
			Timestamp: time.Now(),
			Services:  make(map[string]*ServiceHealth),
			Overall:   HealthStatusNone,
		}
		m.healthChecks[projectID] = report
	}

	// 更新服务健康状态
	for name, svc := range project.Services {
		health := &ServiceHealth{
			ServiceName: name,
			Status:      HealthStatusNone,
			LastCheck:   time.Now(),
			Instances:   make([]InstanceHealth, 0),
		}

		// 根据服务状态推断健康状态
		switch svc.Status {
		case ServiceStatusRunning:
			health.Status = HealthStatusHealthy
		case ServiceStatusError, ServiceStatusUnhealthy:
			health.Status = HealthStatusUnhealthy
		case ServiceStatusCreating, ServiceStatusScaling:
			health.Status = HealthStatusStarting
		}

		// 为每个容器实例创建健康状态
		for _, containerID := range svc.ContainerIDs {
			instance := InstanceHealth{
				ContainerID: containerID,
				Status:      health.Status,
				CPU:         0.5,               // 模拟数据
				Memory:      128 * 1024 * 1024, // 128MB
				Uptime:      time.Since(project.CreatedAt),
			}
			health.Instances = append(health.Instances, instance)
		}

		report.Services[name] = health
	}

	// 计算整体健康状态
	overallStatus := HealthStatusHealthy
	for _, health := range report.Services {
		if health.Status == HealthStatusUnhealthy {
			overallStatus = HealthStatusUnhealthy
			break
		}
		if health.Status == HealthStatusStarting {
			overallStatus = HealthStatusStarting
		}
	}
	report.Overall = overallStatus

	return report, nil
}

// UpdateServiceHealthCheck 更新服务健康检查配置
func (m *Manager) UpdateServiceHealthCheck(projectID, serviceName string, config *HealthCheckConfig) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	svc, ok := project.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "service", ID: serviceName}
	}

	svc.HealthCheck = config
	project.UpdatedAt = time.Now()

	return svc, nil
}

// UpdateServiceResources 更新服务资源限制
func (m *Manager) UpdateServiceResources(projectID, serviceName string, limits *ResourceLimits) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	svc, ok := project.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "service", ID: serviceName}
	}

	svc.Resources = limits
	project.UpdatedAt = time.Now()

	return svc, nil
}

// ========== 自动扩缩容 ==========

// UpdateAutoScalePolicy 更新自动扩缩容策略
func (m *Manager) UpdateAutoScalePolicy(projectID, serviceName string, policy *AutoScalePolicy) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	svc, ok := project.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "service", ID: serviceName}
	}

	if svc.Deploy == nil {
		svc.Deploy = &DeployConfig{Replicas: svc.DesiredCount}
	}
	svc.Deploy.AutoScale = policy
	project.UpdatedAt = time.Now()

	return svc, nil
}

// EvaluateAutoScale 评估是否需要扩缩容
func (m *Manager) EvaluateAutoScale(projectID, serviceName string, metrics *ContainerMetrics) (*AutoScaleEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	svc, ok := project.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "service", ID: serviceName}
	}

	if svc.Deploy == nil || svc.Deploy.AutoScale == nil || !svc.Deploy.AutoScale.Enabled {
		return nil, nil
	}

	policy := svc.Deploy.AutoScale
	currentReplicas := svc.Instances
	newReplicas := currentReplicas
	reason := ""

	// 评估每个指标
	for _, metric := range policy.Metrics {
		switch metric.Type {
		case "cpu":
			if metrics.CPU.Percent > metric.Target {
				newReplicas = currentReplicas + policy.ScaleUp.StepSize
				reason = fmt.Sprintf("CPU usage %.2f%% exceeds target %.2f%%", metrics.CPU.Percent, metric.Target)
			}
		case "memory":
			if metrics.Memory.Percent > metric.Target {
				newReplicas = currentReplicas + policy.ScaleUp.StepSize
				reason = fmt.Sprintf("Memory usage %.2f%% exceeds target %.2f%%", metrics.Memory.Percent, metric.Target)
			}
		}
	}

	// 检查是否需要缩容
	if newReplicas == currentReplicas {
		shouldScaleDown := true
		for _, metric := range policy.Metrics {
			switch metric.Type {
			case "cpu":
				if metrics.CPU.Percent > metric.Target*0.5 {
					shouldScaleDown = false
				}
			case "memory":
				if metrics.Memory.Percent > metric.Target*0.5 {
					shouldScaleDown = false
				}
			}
		}
		if shouldScaleDown && currentReplicas > policy.MinReplicas {
			newReplicas = currentReplicas - policy.ScaleDown.StepSize
			reason = "low resource utilization"
		}
	}

	// 限制范围
	if newReplicas < policy.MinReplicas {
		newReplicas = policy.MinReplicas
	}
	if newReplicas > policy.MaxReplicas {
		newReplicas = policy.MaxReplicas
	}

	// 如果没有变化，返回 nil
	if newReplicas == currentReplicas {
		return nil, nil
	}

	// 执行扩缩容
	svc.Instances = newReplicas
	svc.DesiredCount = newReplicas

	// 更新容器 ID
	if newReplicas > currentReplicas {
		for i := currentReplicas; i < newReplicas; i++ {
			containerID := uuid.New().String()[:12]
			svc.ContainerIDs = append(svc.ContainerIDs, containerID)
		}
	} else {
		svc.ContainerIDs = svc.ContainerIDs[:newReplicas]
	}

	event := AutoScaleEvent{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		ServiceName: serviceName,
		Action:      "scale",
		From:        currentReplicas,
		To:          newReplicas,
		Reason:      reason,
		Timestamp:   time.Now(),
		Success:     true,
	}
	m.scaleEvents = append(m.scaleEvents, event)

	log.Printf("AutoScale: %s/%s scaled from %d to %d replicas (reason: %s)",
		projectID, serviceName, currentReplicas, newReplicas, reason)

	return &event, nil
}

// GetAutoScaleEvents 获取扩缩容事件历史
func (m *Manager) GetAutoScaleEvents(projectID string, limit int) []AutoScaleEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]AutoScaleEvent, 0)
	for _, event := range m.scaleEvents {
		if projectID == "" || event.ProjectID == projectID {
			events = append(events, event)
		}
	}

	// 按时间倒序
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}

	return events
}

// ========== 日志聚合 ==========

// GetServiceLogs 获取服务日志
func (m *Manager) GetServiceLogs(projectID string, query LogQuery) ([]LogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	// 确定要查询的服务
	services := query.Services
	if len(services) == 0 {
		// 查询所有服务
		services = make([]string, 0, len(project.Services))
		for name := range project.Services {
			services = append(services, name)
		}
	}

	// 模拟日志数据
	entries := make([]LogEntry, 0)
	now := time.Now()

	for _, serviceName := range services {
		svc, ok := project.Services[serviceName]
		if !ok {
			continue
		}

		// 为每个容器生成模拟日志
		for _, containerID := range svc.ContainerIDs {
			entry := LogEntry{
				Timestamp:   now.Add(-time.Duration(len(entries)) * time.Minute),
				Service:     serviceName,
				ContainerID: containerID,
				Stream:      "stdout",
				Message:     fmt.Sprintf("[%s] Service %s is running", containerID[:8], serviceName),
			}
			entries = append(entries, entry)
		}
	}

	// 应用 tail 限制
	if query.Tail > 0 && len(entries) > query.Tail {
		entries = entries[len(entries)-query.Tail:]
	}

	// 应用时间过滤
	if !query.Since.IsZero() {
		filtered := make([]LogEntry, 0)
		for _, entry := range entries {
			if entry.Timestamp.After(query.Since) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	return entries, nil
}

// StreamServiceLogs 流式获取服务日志
func (m *Manager) StreamServiceLogs(projectID string, query LogQuery) (*LogStream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	stream := &LogStream{
		Entries: make(chan LogEntry, 100),
		Close:   func() {},
	}

	// 启动 goroutine 模拟日志流
	go func() {
		defer close(stream.Entries)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				// 生成模拟日志
				for serviceName, svc := range project.Services {
					if len(query.Services) > 0 {
						found := false
						for _, s := range query.Services {
							if s == serviceName {
								found = true
								break
							}
						}
						if !found {
							continue
						}
					}

					for _, containerID := range svc.ContainerIDs {
						entry := LogEntry{
							Timestamp:   time.Now(),
							Service:     serviceName,
							ContainerID: containerID,
							Stream:      "stdout",
							Message:     fmt.Sprintf("[%s] heartbeat", containerID[:8]),
						}
						stream.Entries <- entry
					}
				}
			}
		}
	}()

	return stream, nil
}

// ========== 统计 ==========

// GetProjectStats 获取项目统计
func (m *Manager) GetProjectStats(projectID string) (*ProjectStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "project", ID: projectID}
	}

	stats := &ProjectStats{
		ProjectID: projectID,
	}

	for _, svc := range project.Services {
		stats.TotalServices++
		if svc.Status == ServiceStatusRunning {
			stats.RunningServices++
		}
		stats.TotalInstances += svc.DesiredCount
		stats.RunningInstances += svc.Instances

		if svc.Resources != nil {
			if svc.Resources.CPU != nil {
				stats.TotalCPU += svc.Resources.CPU.Cores * float64(svc.Instances)
			}
			if svc.Resources.Memory != nil {
				stats.TotalMemory += svc.Resources.Memory.Limit * int64(svc.Instances)
			}
		}
	}

	stats.Uptime = time.Since(project.CreatedAt)

	return stats, nil
}

// ========== 辅助函数 ==========

// isValidServiceName 验证服务名称
func isValidServiceName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	// 只允许小写字母、数字、连字符
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	// 不能以连字符开头或结尾
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	return true
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	close(m.stopCh)
}

// ========== 容器编排增强方法 ==========

// stacks 存储 Compose 栈
var stacks = make(map[string]*ComposeStack)
var stacksMu sync.RWMutex
var imageCaches = make(map[string]*ImageCache)
var imageCachesMu sync.RWMutex
var recoveryPolicies = make(map[string]*RecoveryPolicy)
var recoveryPoliciesMu sync.RWMutex
var autoScaleRules = make(map[string]*AutoScaleRule)
var autoScaleRulesMu sync.RWMutex

// ListStacks 列出所有 Compose 栈
func (m *Manager) ListStacks() []*ComposeStack {
	stacksMu.RLock()
	defer stacksMu.RUnlock()

	result := make([]*ComposeStack, 0, len(stacks))
	for _, s := range stacks {
		result = append(result, s)
	}
	return result
}

// DeployStack 部署 Compose 栈
func (m *Manager) DeployStack(req DeployStackRequest) (*ComposeStack, error) {
	stacksMu.Lock()
	defer stacksMu.Unlock()

	stack := &ComposeStack{
		ID:          uuid.New().String(),
		Name:        req.Name,
		ProjectName: req.Name,
		ComposeFile: req.ComposeFile,
		Status:      StackStatusDeploying,
		EnvVars:     req.EnvVars,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Services:    make([]ComposeService, 0),
	}

	stacks[stack.ID] = stack
	return stack, nil
}

// GetContainerHealth 获取容器健康状态
func (m *Manager) GetContainerHealth(stackID string) ([]*ContainerHealth, error) {
	stacksMu.RLock()
	defer stacksMu.RUnlock()

	if _, ok := stacks[stackID]; !ok {
		return nil, &NotFoundError{Resource: "stack", ID: stackID}
	}

	health := make([]*ContainerHealth, 0)
	for _, svc := range stacks[stackID].Services {
		h := &ContainerHealth{
			ContainerID:  svc.Name + "-1",
			ServiceName:  svc.Name,
			StackID:      stackID,
			Status:       HealthStatusHealthy,
			ChecksPassed: 10,
			ChecksFailed: 0,
			LastCheck:    time.Now(),
			Uptime:       time.Since(stacks[stackID].CreatedAt),
		}
		health = append(health, h)
	}
	return health, nil
}

// SetAutoScale 设置自动扩缩容规则
func (m *Manager) SetAutoScale(stackID string, req SetAutoScaleRequest) (*AutoScaleRule, error) {
	autoScaleRulesMu.Lock()
	defer autoScaleRulesMu.Unlock()

	rule := &AutoScaleRule{
		ID:          uuid.New().String(),
		StackID:     stackID,
		ServiceName: req.ServiceName,
		Enabled:     req.Enabled,
		MinReplicas: req.MinReplicas,
		MaxReplicas: req.MaxReplicas,
		MetricType:  req.MetricType,
		TargetValue: req.TargetValue,
		Cooldown:    5 * time.Minute,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	autoScaleRules[rule.ID] = rule
	return rule, nil
}

// CacheImage 缓存镜像
func (m *Manager) CacheImage(req CacheImageRequest) (*ImageCache, error) {
	imageCachesMu.Lock()
	defer imageCachesMu.Unlock()

	cache := &ImageCache{
		ImageName: req.Image,
		Tag:       req.Tag,
		Digest:    "sha256:abc123",
		Size:      1024 * 1024 * 100, // 100MB
		LastUsed:  time.Now(),
		PullCount: 1,
		Cached:    true,
	}

	imageCaches[req.Image+":"+req.Tag] = cache
	return cache, nil
}

// SetRecovery 设置容器恢复策略
func (m *Manager) SetRecovery(stackID string, req SetRecoveryRequest) (*RecoveryPolicy, error) {
	recoveryPoliciesMu.Lock()
	defer recoveryPoliciesMu.Unlock()

	policy := &RecoveryPolicy{
		ID:               uuid.New().String(),
		StackID:          stackID,
		ServiceName:      req.ServiceName,
		Enabled:          req.Enabled,
		RestartOnFailure: req.RestartOnFailure,
		MaxRetries:       req.MaxRetries,
		RetryInterval:    30 * time.Second,
		AutoRemove:       false,
		HealthCheck:      req.HealthCheck,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	recoveryPolicies[policy.ID] = policy
	return policy, nil
}
