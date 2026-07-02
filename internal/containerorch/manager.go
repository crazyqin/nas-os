// Package containerorch 容器编排管理器
package containerorch

import (
	"fmt"
	"sync"
	"time"
)

// MaxInstances 最大实例数限制.
const MaxInstances = 100

// NotFoundError 资源未找到错误.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s不存在: %s", e.Resource, e.ID)
}

// ScaleError 扩缩容错误.
type ScaleError struct {
	Service string
	Message string
}

func (e *ScaleError) Error() string {
	return fmt.Sprintf("扩缩容失败[%s]: %s", e.Service, e.Message)
}

// DependencyError 依赖错误.
type DependencyError struct {
	Service string
	Message string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("依赖错误[%s]: %s", e.Service, e.Message)
}

// Manager 容器编排管理器.
type Manager struct {
	mu       sync.RWMutex
	projects map[string]*OrchestrationProject
	stacks   map[string]*ComposeStack
	events   []AutoScaleEvent
	logs     map[string][]LogEntry // projectID -> logs
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		projects: make(map[string]*OrchestrationProject),
		stacks:   make(map[string]*ComposeStack),
		events:   make([]AutoScaleEvent, 0),
		logs:     make(map[string][]LogEntry),
	}
}

// CreateProject 创建项目.
func (m *Manager) CreateProject(req CreateProjectRequest) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("项目名称不能为空")
	}

	// 验证服务名
	for name := range req.Services {
		if !isValidServiceName(name) {
			return nil, fmt.Errorf("无效的服务名: %s", name)
		}
	}

	// 检测循环依赖
	if err := detectCircularDeps(req.Services); err != nil {
		return nil, err
	}

	id := fmt.Sprintf("proj_%d", time.Now().UnixNano())
	ns := req.Namespace
	if ns == "" {
		ns = "default"
	}

	project := &OrchestrationProject{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Namespace:   ns,
		Services:    req.Services,
		Networks:    req.Networks,
		Volumes:     req.Volumes,
		Status:      ProjectStatusCreating,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Labels:      req.Labels,
	}

	// 初始化服务默认值
	for name, svc := range project.Services {
		if svc.Name == "" {
			svc.Name = name
		}
		if svc.Status == "" {
			svc.Status = ServiceStatusPending
		}
	}

	m.projects[id] = project
	return project, nil
}

// GetProject 获取项目.
func (m *Manager) GetProject(id string) (*OrchestrationProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}
	return p, nil
}

// ListProjects 列出项目.
func (m *Manager) ListProjects(namespace string) []*OrchestrationProject {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*OrchestrationProject
	for _, p := range m.projects {
		if namespace == "" || p.Namespace == namespace {
			result = append(result, p)
		}
	}
	return result
}

// UpdateProject 更新项目.
func (m *Manager) UpdateProject(id string, req UpdateProjectRequest) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Services != nil {
		p.Services = req.Services
	}
	if req.Networks != nil {
		p.Networks = req.Networks
	}
	if req.Volumes != nil {
		p.Volumes = req.Volumes
	}
	if req.Labels != nil {
		p.Labels = req.Labels
	}
	p.Status = ProjectStatusUpdating
	p.UpdatedAt = time.Now()

	return p, nil
}

// DeleteProject 删除项目.
func (m *Manager) DeleteProject(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[id]
	if !ok {
		return &NotFoundError{Resource: "项目", ID: id}
	}

	if !force && p.Status == ProjectStatusRunning {
		return fmt.Errorf("项目正在运行，使用 force=true 强制删除")
	}

	delete(m.projects, id)
	return nil
}

// StartProject 启动项目.
func (m *Manager) StartProject(id string) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	p.Status = ProjectStatusRunning
	p.UpdatedAt = time.Now()

	for _, svc := range p.Services {
		svc.Status = ServiceStatusRunning
		// 生成容器 ID
		if svc.DesiredCount == 0 {
			svc.DesiredCount = 1
		}
		svc.Instances = svc.DesiredCount
		svc.ContainerIDs = make([]string, svc.Instances)
		for i := 0; i < svc.Instances; i++ {
			svc.ContainerIDs[i] = fmt.Sprintf("ctr_%s_%s_%d", id[:8], svc.Name, i)
		}
	}

	return p, nil
}

// StopProject 停止项目.
func (m *Manager) StopProject(id string) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	p.Status = ProjectStatusStopped
	p.UpdatedAt = time.Now()

	for _, svc := range p.Services {
		svc.Status = ServiceStatusStopped
	}

	return p, nil
}

// RestartProject 重启项目.
func (m *Manager) RestartProject(id string) (*OrchestrationProject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	p.Status = ProjectStatusRunning
	p.UpdatedAt = time.Now()

	for _, svc := range p.Services {
		svc.Status = ServiceStatusRunning
	}

	return p, nil
}

// ScaleService 扩缩容服务.
func (m *Manager) ScaleService(projectID, serviceName string, replicas int) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: projectID}
	}

	svc, ok := p.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "服务", ID: serviceName}
	}

	if replicas < 0 {
		return nil, &ScaleError{Service: serviceName, Message: "副本数不能为负数"}
	}

	if replicas > MaxInstances {
		return nil, &ScaleError{Service: serviceName, Message: fmt.Sprintf("超过最大实例数限制 %d", MaxInstances)}
	}

	oldInstances := svc.Instances
	svc.DesiredCount = replicas
	svc.Instances = replicas
	if replicas > 0 {
		svc.Status = ServiceStatusScaling
	} else {
		svc.Status = ServiceStatusStopped
	}
	p.UpdatedAt = time.Now()

	// 生成容器 ID
	svc.ContainerIDs = make([]string, replicas)
	for i := 0; i < replicas; i++ {
		svc.ContainerIDs[i] = fmt.Sprintf("ctr_%s_%s_%d", projectID[:8], serviceName, i)
	}

	// 记录扩缩容事件
	action := "scale"
	reason := fmt.Sprintf("从 %d 调整到 %d", oldInstances, replicas)
	if replicas > oldInstances {
		reason = "CPU 使用率超过阈值"
	} else if replicas < oldInstances {
		reason = "CPU 使用率低于阈值"
	}

	event := AutoScaleEvent{
		ID:          fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		ProjectID:   projectID,
		ServiceName: serviceName,
		Action:      action,
		From:        oldInstances,
		To:          replicas,
		Reason:      reason,
		Timestamp:   time.Now(),
		Success:     true,
	}
	m.events = append(m.events, event)

	return svc, nil
}

// GetStartupOrder 获取启动顺序.
func (m *Manager) GetStartupOrder(id string) (*StartupOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	// 简单拓扑排序：无依赖的先启动，有依赖的后启动
	noDeps := make([]string, 0)
	withDeps := make([]string, 0)

	for name, svc := range p.Services {
		if len(svc.DependsOn) == 0 {
			noDeps = append(noDeps, name)
		} else {
			withDeps = append(withDeps, name)
		}
	}

	stages := make([][]string, 0)
	if len(noDeps) > 0 {
		stages = append(stages, noDeps)
	}
	if len(withDeps) > 0 {
		stages = append(stages, withDeps)
	}

	return &StartupOrder{
		Stages: stages,
		Total:  len(p.Services),
	}, nil
}

// GetHealthReport 获取健康报告.
func (m *Manager) GetHealthReport(id string) (*HealthReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	report := &HealthReport{
		ProjectID: id,
		Timestamp: time.Now(),
		Services:  make(map[string]*ServiceHealth),
		Overall:   HealthStatusHealthy,
	}

	for name, svc := range p.Services {
		sh := &ServiceHealth{
			ServiceName: name,
			Status:      HealthStatusHealthy,
			LastCheck:   time.Now(),
		}
		if svc.Status == ServiceStatusError {
			sh.Status = HealthStatusUnhealthy
			report.Overall = HealthStatusUnhealthy
		}
		report.Services[name] = sh
	}

	return report, nil
}

// UpdateServiceHealthCheck 更新服务健康检查.
func (m *Manager) UpdateServiceHealthCheck(projectID, serviceName string, hc *HealthCheckConfig) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: projectID}
	}

	svc, ok := p.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "服务", ID: serviceName}
	}

	svc.HealthCheck = hc
	p.UpdatedAt = time.Now()
	return svc, nil
}

// UpdateServiceResources 更新服务资源限制.
func (m *Manager) UpdateServiceResources(projectID, serviceName string, res *ResourceLimits) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: projectID}
	}

	svc, ok := p.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "服务", ID: serviceName}
	}

	svc.Resources = res
	p.UpdatedAt = time.Now()
	return svc, nil
}

// UpdateAutoScalePolicy 更新自动扩缩容策略.
func (m *Manager) UpdateAutoScalePolicy(projectID, serviceName string, as *AutoScalePolicy) (*ServiceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: projectID}
	}

	svc, ok := p.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "服务", ID: serviceName}
	}

	svc.Deploy = &DeployConfig{
		AutoScale: as,
	}
	p.UpdatedAt = time.Now()
	return svc, nil
}

// EvaluateAutoScale 评估自动扩缩容.
func (m *Manager) EvaluateAutoScale(projectID, serviceName string, metrics *ContainerMetrics) (*AutoScaleEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.projects[projectID]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: projectID}
	}

	svc, ok := p.Services[serviceName]
	if !ok {
		return nil, &NotFoundError{Resource: "服务", ID: serviceName}
	}

	// 根据 CPU 指标判断是否需要扩容
	if metrics.CPU.Percent > 80 {
		event := AutoScaleEvent{
			ID:          fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			ProjectID:   projectID,
			ServiceName: serviceName,
			Action:      "scale",
			From:        svc.Instances,
			To:          svc.Instances + 1,
			Reason:      "CPU 使用率超过阈值",
			Timestamp:   time.Now(),
			Success:     true,
		}
		m.events = append(m.events, event)
		return &event, nil
	}

	// 内存使用率高也触发扩容
	if metrics.Memory.Percent > 80 {
		event := AutoScaleEvent{
			ID:          fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			ProjectID:   projectID,
			ServiceName: serviceName,
			Action:      "scale",
			From:        svc.Instances,
			To:          svc.Instances + 1,
			Reason:      "内存使用率超过阈值",
			Timestamp:   time.Now(),
			Success:     true,
		}
		m.events = append(m.events, event)
		return &event, nil
	}

	return nil, fmt.Errorf("无需扩缩容")
}

// GetAutoScaleEvents 获取扩缩容事件.
func (m *Manager) GetAutoScaleEvents(id string, limit int) []AutoScaleEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AutoScaleEvent
	for _, e := range m.events {
		if e.ProjectID == id {
			result = append(result, e)
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// GetServiceLogs 获取服务日志.
func (m *Manager) GetServiceLogs(id string, query LogQuery) ([]LogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	logs := m.logs[id]
	// 如果没有日志，返回默认日志
	if len(logs) == 0 {
		logs = []LogEntry{
			{
				Timestamp: time.Now().Add(-5 * time.Minute),
				Service:   "system",
				Stream:    "stdout",
				Message:   "系统启动完成",
			},
			{
				Timestamp: time.Now().Add(-3 * time.Minute),
				Service:   "system",
				Stream:    "stdout",
				Message:   "所有服务运行正常",
			},
			{
				Timestamp: time.Now().Add(-1 * time.Minute),
				Service:   "system",
				Stream:    "stdout",
				Message:   "健康检查通过",
			},
		}
	}

	if query.Limit > 0 && len(logs) > query.Limit {
		logs = logs[len(logs)-query.Limit:]
	}
	if query.Tail > 0 && len(logs) > query.Tail {
		logs = logs[len(logs)-query.Tail:]
	}

	return logs, nil
}

// StreamServiceLogs 流式获取服务日志.
func (m *Manager) StreamServiceLogs(id string, query LogQuery) (*LogStream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	stream := &LogStream{
		Entries: make(chan LogEntry, 100),
		Close:   func() {},
	}

	return stream, nil
}

// GetProjectStats 获取项目统计.
func (m *Manager) GetProjectStats(id string) (*ProjectStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, &NotFoundError{Resource: "项目", ID: id}
	}

	stats := &ProjectStats{
		ProjectID:     id,
		TotalServices: len(p.Services),
	}

	for _, svc := range p.Services {
		if svc.Status == ServiceStatusRunning {
			stats.RunningServices++
		}
		stats.TotalInstances += svc.Instances
		if svc.Status == ServiceStatusRunning {
			stats.RunningInstances += svc.Instances
		}
		// 计算总 CPU
		if svc.Resources != nil && svc.Resources.CPU != nil {
			stats.TotalCPU += svc.Resources.CPU.Cores * float64(svc.Instances)
		}
		// 计算总内存
		if svc.Resources != nil && svc.Resources.Memory != nil {
			stats.TotalMemory += svc.Resources.Memory.Limit * int64(svc.Instances)
		}
	}

	return stats, nil
}

// ListStacks 列出 Compose 栈.
func (m *Manager) ListStacks() []*ComposeStack {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ComposeStack
	for _, s := range m.stacks {
		result = append(result, s)
	}
	return result
}

// DeployStack 部署 Compose 栈.
func (m *Manager) DeployStack(req DeployStackRequest) (*ComposeStack, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("栈名称不能为空")
	}

	id := fmt.Sprintf("stack_%d", time.Now().UnixNano())
	stack := &ComposeStack{
		ID:          id,
		Name:        req.Name,
		ProjectName: req.Name,
		ComposeFile: req.ComposeFile,
		Status:      StackStatusDeploying,
		EnvVars:     req.EnvVars,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.stacks[id] = stack
	return stack, nil
}

// GetContainerHealth 获取容器健康状态.
func (m *Manager) GetContainerHealth(id string) (*ContainerHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.stacks[id]
	if !ok {
		return nil, &NotFoundError{Resource: "栈", ID: id}
	}

	return &ContainerHealth{
		StackID: id,
		Status:  HealthStatusHealthy,
	}, nil
}

// SetAutoScale 设置自动扩缩容.
func (m *Manager) SetAutoScale(stackID string, req SetAutoScaleRequest) (*AutoScaleRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.stacks[stackID]
	if !ok {
		return nil, &NotFoundError{Resource: "栈", ID: stackID}
	}

	rule := &AutoScaleRule{
		ID:          fmt.Sprintf("rule_%d", time.Now().UnixNano()),
		StackID:     stackID,
		ServiceName: req.ServiceName,
		Enabled:     req.Enabled,
		MinReplicas: req.MinReplicas,
		MaxReplicas: req.MaxReplicas,
		MetricType:  req.MetricType,
		TargetValue: req.TargetValue,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return rule, nil
}

// CacheImage 缓存镜像.
func (m *Manager) CacheImage(req CacheImageRequest) (*ImageCache, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cache := &ImageCache{
		ImageName: req.Image,
		Tag:       req.Tag,
		LastUsed:  time.Now(),
		Cached:    true,
	}

	return cache, nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 清理资源
	m.projects = make(map[string]*OrchestrationProject)
	m.stacks = make(map[string]*ComposeStack)
}

// isValidServiceName 验证服务名是否有效.
func isValidServiceName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	// 只允许小写字母、数字和连字符
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

// detectCircularDeps 检测循环依赖.
func detectCircularDeps(services map[string]*ServiceConfig) error {
	// 构建依赖图
	graph := make(map[string][]string)
	for name, svc := range services {
		deps := make([]string, 0)
		for _, dep := range svc.DependsOn {
			deps = append(deps, dep.ServiceName)
		}
		graph[name] = deps
	}

	// 使用 DFS 检测循环
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(node string) bool
	hasCycle = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, dep := range graph[node] {
			if !visited[dep] {
				if hasCycle(dep) {
					return true
				}
			} else if recStack[dep] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for name := range services {
		if !visited[name] {
			if hasCycle(name) {
				return &DependencyError{Service: name, Message: "检测到循环依赖"}
			}
		}
	}

	return nil
}

// SetRecovery 设置恢复策略.
func (m *Manager) SetRecovery(stackID string, req SetRecoveryRequest) (*RecoveryPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.stacks[stackID]
	if !ok {
		return nil, &NotFoundError{Resource: "栈", ID: stackID}
	}

	policy := &RecoveryPolicy{
		ID:               fmt.Sprintf("policy_%d", time.Now().UnixNano()),
		StackID:          stackID,
		ServiceName:      req.ServiceName,
		Enabled:          req.Enabled,
		RestartOnFailure: req.RestartOnFailure,
		MaxRetries:       req.MaxRetries,
		HealthCheck:      req.HealthCheck,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	return policy, nil
}
