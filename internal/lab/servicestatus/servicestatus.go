// Package servicestatus 服务状态仪表盘
// 提供服务监控、依赖拓扑、性能指标、告警联动、服务编排等功能
package servicestatus

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ============================================================
// 服务类型枚举
// ============================================================

// ServiceType 服务类型.
type ServiceType string

const (
	ServiceTypeDocker  ServiceType = "docker"  // Docker 容器
	ServiceTypeSystemd ServiceType = "systemd" // Systemd 服务
	ServiceTypeBinary  ServiceType = "binary"  // 独立二进制进程
	ServiceTypePod     ServiceType = "pod"     // Pod (k3s/k8s)
	ServiceTypeCron    ServiceType = "cron"    // 定时任务
)

// ============================================================
// 服务状态枚举
// ============================================================

// ServiceStatus 服务状态.
type ServiceStatus string

const (
	StatusRunning  ServiceStatus = "running"  // 运行中
	StatusStopped  ServiceStatus = "stopped"  // 已停止
	StatusDegraded ServiceStatus = "degraded" // 降级运行
	StatusStarting ServiceStatus = "starting" // 启动中
	StatusError    ServiceStatus = "error"    // 异常
	StatusUnknown  ServiceStatus = "unknown"  // 未知
)

// ============================================================
// 重启策略
// ============================================================

// RestartPolicy 重启策略.
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"     // 总是重启
	RestartOnFailure RestartPolicy = "on-failure" // 仅失败时重启
	RestartNever     RestartPolicy = "never"      // 不重启
)

// ============================================================
// 健康检查
// ============================================================

// HealthCheck 健康检查配置.
type HealthCheck struct {
	URL                 string        `json:"url"`                  // 检查 URL
	Interval            time.Duration `json:"interval"`             // 检查间隔
	Timeout             time.Duration `json:"timeout"`              // 超时时间
	LastCheck           time.Time     `json:"last_check"`           // 最后检查时间
	LastStatus          string        `json:"last_status"`          // 最后检查状态
	ConsecutiveFailures int           `json:"consecutive_failures"` // 连续失败次数
	Healthy             bool          `json:"healthy"`              // 是否健康
}

// ============================================================
// 资源配额
// ============================================================

// ResourceQuota 资源配额.
type ResourceQuota struct {
	MaxCPU       float64 `json:"max_cpu"`       // CPU 上限（百分比）
	MaxMemory    int64   `json:"max_memory"`    // 内存上限（字节）
	MaxFDs       int     `json:"max_fds"`       // 文件描述符上限
	MaxConns     int     `json:"max_conns"`     // 最大连接数
	CurrentCPU   float64 `json:"current_cpu"`   // 当前 CPU 使用
	CurrentMem   int64   `json:"current_mem"`   // 当前内存使用
	CurrentFDs   int     `json:"current_fds"`   // 当前文件描述符
	CurrentConns int     `json:"current_conns"` // 当前连接数
}

// ============================================================
// 性能指标
// ============================================================

// MetricPoint 指标数据点.
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu"`     // CPU 使用率
	Memory    int64     `json:"memory"`  // 内存使用（字节）
	NetIn     int64     `json:"net_in"`  // 入站字节数
	NetOut    int64     `json:"net_out"` // 出站字节数
	DiskRead  int64     `json:"disk_read"`
	DiskWrite int64     `json:"disk_write"`
	Latency   float64   `json:"latency"` // 请求延迟（毫秒）
}

// ServiceMetrics 服务性能指标.
type ServiceID string

type ServiceMetrics struct {
	ServiceID ServiceID     `json:"service_id"`
	Current   *MetricPoint  `json:"current"`           // 当前指标
	History   []MetricPoint `json:"history,omitempty"` // 历史记录（最近 24h）
}

// ============================================================
// 服务结构体
// ============================================================

// Service 服务定义.
type Service struct {
	ID            ServiceID         `json:"id"`
	Name          string            `json:"name"`
	Type          ServiceType       `json:"type"`
	Status        ServiceStatus     `json:"status"`
	Port          int               `json:"port,omitempty"`
	PID           int               `json:"pid,omitempty"`
	Uptime        time.Duration     `json:"uptime"`
	CPU           float64           `json:"cpu"`
	Memory        int64             `json:"memory"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
	Dependencies  []ServiceID       `json:"dependencies,omitempty"` // 依赖的服务 ID
	Group         string            `json:"group,omitempty"`        // 服务组
	RestartPolicy RestartPolicy     `json:"restart_policy"`
	Quota         *ResourceQuota    `json:"quota,omitempty"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	StoppedAt     *time.Time        `json:"stopped_at,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// ============================================================
// 拓扑结构
// ============================================================

// TopologyNode 拓扑节点.
type TopologyNode struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Type   ServiceType   `json:"type"`
	Status ServiceStatus `json:"status"`
}

// TopologyEdge 拓扑边.
type TopologyEdge struct {
	Source ServiceID `json:"source"`
	Target ServiceID `json:"target"`
}

// ServiceTopology 服务拓扑.
type ServiceTopology struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// ============================================================
// 系统总览
// ============================================================

// DashboardOverview 系统总览.
type DashboardOverview struct {
	TotalServices     int            `json:"total_services"`
	RunningServices   int            `json:"running_services"`
	StoppedServices   int            `json:"stopped_services"`
	DegradedServices  int            `json:"degraded_services"`
	ErrorServices     int            `json:"error_services"`
	StartingServices  int            `json:"starting_services"`
	UnknownServices   int            `json:"unknown_services"`
	ServicesByType    map[string]int `json:"services_by_type"`
	ServicesByGroup   map[string]int `json:"services_by_group"`
	TotalAlerts       int            `json:"total_alerts"`
	HealthyServices   int            `json:"healthy_services"`
	UnhealthyServices int            `json:"unhealthy_services"`
}

// ============================================================
// 告警
// ============================================================

// Alert 告警信息.
type Alert struct {
	ID         string     `json:"id"`
	ServiceID  ServiceID  `json:"service_id"`
	Level      string     `json:"level"` // info, warning, critical
	Message    string     `json:"message"`
	Timestamp  time.Time  `json:"timestamp"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// ============================================================
// ServiceDashboard 服务仪表盘
// ============================================================

// ServiceDashboard 服务仪表盘.
type ServiceDashboard struct {
	mu       sync.RWMutex
	services map[ServiceID]*Service
	metrics  map[ServiceID][]MetricPoint
	alerts   []*Alert
	hcClient *http.Client
}

// NewServiceDashboard 创建服务仪表盘.
func NewServiceDashboard() *ServiceDashboard {
	return &ServiceDashboard{
		services: make(map[ServiceID]*Service),
		metrics:  make(map[ServiceID][]MetricPoint),
		alerts:   make([]*Alert, 0),
		hcClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// ============================================================
// 服务 CRUD
// ============================================================

// RegisterService 注册服务.
func (d *ServiceDashboard) RegisterService(svc *Service) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if svc.ID == "" {
		return fmt.Errorf("服务 ID 不能为空")
	}
	if _, exists := d.services[svc.ID]; exists {
		return fmt.Errorf("服务 %s 已注册", svc.ID)
	}
	if svc.Status == "" {
		svc.Status = StatusUnknown
	}
	if svc.RestartPolicy == "" {
		svc.RestartPolicy = RestartNever
	}

	d.services[svc.ID] = svc
	return nil
}

// UnregisterService 注销服务.
func (d *ServiceDashboard) UnregisterService(id ServiceID) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.services[id]; !exists {
		return fmt.Errorf("服务 %s 不存在", id)
	}

	delete(d.services, id)
	delete(d.metrics, id)
	return nil
}

// GetService 获取服务.
func (d *ServiceDashboard) GetService(id ServiceID) (*Service, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	svc, exists := d.services[id]
	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", id)
	}
	copy := *svc
	return &copy, nil
}

// ListServices 列出服务.
func (d *ServiceDashboard) ListServices(status ServiceStatus, svcType ServiceType, group string) []*Service {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*Service, 0, len(d.services))
	for _, svc := range d.services {
		if status != "" && svc.Status != status {
			continue
		}
		if svcType != "" && svc.Type != svcType {
			continue
		}
		if group != "" && svc.Group != group {
			continue
		}
		copy := *svc
		result = append(result, &copy)
	}

	sort.Slice(result, func(i, j int) bool {
		return string(result[i].ID) < string(result[j].ID)
	})
	return result
}

// UpdateService 更新服务信息.
func (d *ServiceDashboard) UpdateService(id ServiceID, update func(*Service)) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	svc, exists := d.services[id]
	if !exists {
		return fmt.Errorf("服务 %s 不存在", id)
	}

	update(svc)
	return nil
}

// ============================================================
// 服务控制：启动 / 停止 / 重启
// ============================================================

// StartService 启动服务.
func (d *ServiceDashboard) StartService(id ServiceID) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	svc, exists := d.services[id]
	if !exists {
		return fmt.Errorf("服务 %s 不存在", id)
	}

	if svc.Status == StatusRunning {
		return fmt.Errorf("服务 %s 已在运行", id)
	}

	// 检查依赖是否满足
	for _, depID := range svc.Dependencies {
		dep, ok := d.services[depID]
		if !ok {
			return fmt.Errorf("依赖服务 %s 不存在", depID)
		}
		if dep.Status != StatusRunning && dep.Status != StatusDegraded {
			return fmt.Errorf("依赖服务 %s 未就绪（状态: %s）", depID, dep.Status)
		}
	}

	svc.Status = StatusStarting
	now := time.Now()
	svc.StartedAt = &now
	svc.StoppedAt = nil

	// 异步模拟启动完成
	go func() {
		time.Sleep(1 * time.Second)
		d.mu.Lock()
		defer d.mu.Unlock()
		if s, ok := d.services[id]; ok && s.Status == StatusStarting {
			s.Status = StatusRunning
			s.Uptime = 0
		}
	}()

	return nil
}

// StopService 停止服务.
func (d *ServiceDashboard) StopService(id ServiceID) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	svc, exists := d.services[id]
	if !exists {
		return fmt.Errorf("服务 %s 不存在", id)
	}

	if svc.Status == StatusStopped {
		return fmt.Errorf("服务 %s 已停止", id)
	}

	svc.Status = StatusStopped
	now := time.Now()
	svc.StoppedAt = &now
	return nil
}

// RestartService 重启服务.
func (d *ServiceDashboard) RestartService(id ServiceID) error {
	if err := d.StopService(id); err != nil {
		return err
	}
	return d.StartService(id)
}

// ============================================================
// 批量操作（服务组）
// ============================================================

// GroupAction 服务组操作类型.
type GroupAction string

const (
	ActionStart   GroupAction = "start"
	ActionStop    GroupAction = "stop"
	ActionRestart GroupAction = "restart"
)

// GroupActionResult 服务组操作结果.
type GroupActionResult struct {
	ServiceID ServiceID `json:"service_id"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// ExecuteGroupAction 对服务组执行批量操作.
func (d *ServiceDashboard) ExecuteGroupAction(group string, action GroupAction) []GroupActionResult {
	d.mu.RLock()
	// 收集组内服务 ID
	ids := make([]ServiceID, 0)
	for _, svc := range d.services {
		if svc.Group == group {
			ids = append(ids, svc.ID)
		}
	}
	d.mu.RUnlock()

	results := make([]GroupActionResult, 0, len(ids))
	for _, id := range ids {
		var err error
		switch action {
		case ActionStart:
			err = d.StartService(id)
		case ActionStop:
			err = d.StopService(id)
		case ActionRestart:
			err = d.RestartService(id)
		default:
			err = fmt.Errorf("未知操作: %s", action)
		}

		r := GroupActionResult{ServiceID: id, Success: err == nil}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results
}

// ============================================================
// 健康检查
// ============================================================

// RunHealthCheck 执行单个服务的健康检查.
func (d *ServiceDashboard) RunHealthCheck(ctx context.Context, id ServiceID) (*HealthCheck, error) {
	d.mu.RLock()
	svc, exists := d.services[id]
	if !exists {
		d.mu.RUnlock()
		return nil, fmt.Errorf("服务 %s 不存在", id)
	}
	hc := svc.HealthCheck
	d.mu.RUnlock()

	if hc == nil {
		return nil, fmt.Errorf("服务 %s 未配置健康检查", id)
	}

	hc.LastCheck = time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, hc.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", hc.URL, nil)
	if err != nil {
		hc.ConsecutiveFailures++
		hc.Healthy = false
		hc.LastStatus = fmt.Sprintf("创建请求失败: %v", err)
		d.raiseAlert(id, "critical", hc.LastStatus)
		return hc, nil
	}

	resp, err := d.hcClient.Do(req)
	if err != nil {
		hc.ConsecutiveFailures++
		hc.Healthy = false
		hc.LastStatus = fmt.Sprintf("请求失败: %v", err)
		if hc.ConsecutiveFailures >= 3 {
			d.raiseAlert(id, "critical", hc.LastStatus)
		}
		return hc, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		hc.ConsecutiveFailures = 0
		hc.Healthy = true
		hc.LastStatus = "healthy"
	} else {
		hc.ConsecutiveFailures++
		hc.Healthy = false
		hc.LastStatus = fmt.Sprintf("HTTP %d", resp.StatusCode)
		if hc.ConsecutiveFailures >= 3 {
			d.raiseAlert(id, "warning", hc.LastStatus)
		}
	}

	return hc, nil
}

// RunAllHealthChecks 执行所有已配置健康检查的服务.
func (d *ServiceDashboard) RunAllHealthChecks(ctx context.Context) {
	d.mu.RLock()
	ids := make([]ServiceID, 0)
	for id, svc := range d.services {
		if svc.HealthCheck != nil && svc.Status == StatusRunning {
			ids = append(ids, id)
		}
	}
	d.mu.RUnlock()

	for _, id := range ids {
		_, _ = d.RunHealthCheck(ctx, id)
	}
}

// ============================================================
// 告警
// ============================================================

// raiseAlert 产生告警.
func (d *ServiceDashboard) raiseAlert(svcID ServiceID, level, message string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	alert := &Alert{
		ID:        fmt.Sprintf("%s-%d", svcID, time.Now().UnixNano()),
		ServiceID: svcID,
		Level:     level,
		Message:   message,
		Timestamp: time.Now(),
	}
	d.alerts = append(d.alerts, alert)

	// 最多保留 1000 条
	if len(d.alerts) > 1000 {
		d.alerts = d.alerts[len(d.alerts)-1000:]
	}
}

// GetAlerts 获取告警列表.
func (d *ServiceDashboard) GetAlerts(unresolvedOnly bool, limit int) []*Alert {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*Alert, 0)
	for i := len(d.alerts) - 1; i >= 0; i-- {
		if unresolvedOnly && d.alerts[i].Resolved {
			continue
		}
		result = append(result, d.alerts[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ResolveAlert 解决告警.
func (d *ServiceDashboard) ResolveAlert(alertID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, a := range d.alerts {
		if a.ID == alertID {
			now := time.Now()
			a.Resolved = true
			a.ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// ============================================================
// 性能指标
// ============================================================

// RecordMetrics 记录服务性能指标.
func (d *ServiceDashboard) RecordMetrics(svcID ServiceID, point *MetricPoint) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.services[svcID]; !exists {
		return fmt.Errorf("服务 %s 不存在", svcID)
	}

	if point.Timestamp.IsZero() {
		point.Timestamp = time.Now()
	}

	d.metrics[svcID] = append(d.metrics[svcID], *point)

	// 更新服务当前资源
	svc := d.services[svcID]
	svc.CPU = point.CPU
	svc.Memory = point.Memory

	// 检查资源配额
	if svc.Quota != nil {
		svc.Quota.CurrentCPU = point.CPU
		svc.Quota.CurrentMem = point.Memory
		if svc.Quota.MaxCPU > 0 && point.CPU > svc.Quota.MaxCPU {
			d.raiseAlert(svcID, "warning", fmt.Sprintf("CPU 超限: %.1f%% > %.1f%%", point.CPU, svc.Quota.MaxCPU))
		}
		if svc.Quota.MaxMemory > 0 && point.Memory > svc.Quota.MaxMemory {
			d.raiseAlert(svcID, "warning", fmt.Sprintf("内存超限: %d > %d", point.Memory, svc.Quota.MaxMemory))
		}
	}

	// 保留最近 24h 数据
	cutoff := time.Now().Add(-24 * time.Hour)
	history := d.metrics[svcID]
	idx := 0
	for idx < len(history) && history[idx].Timestamp.Before(cutoff) {
		idx++
	}
	if idx > 0 {
		d.metrics[svcID] = history[idx:]
	}

	return nil
}

// GetMetrics 获取服务性能指标（最近 24h）.
func (d *ServiceDashboard) GetMetrics(svcID ServiceID) (*ServiceMetrics, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if _, exists := d.services[svcID]; !exists {
		return nil, fmt.Errorf("服务 %s 不存在", svcID)
	}

	history := d.metrics[svcID]
	result := &ServiceMetrics{
		ServiceID: svcID,
		History:   make([]MetricPoint, len(history)),
	}
	copy(result.History, history)

	if len(history) > 0 {
		last := history[len(history)-1]
		result.Current = &last
	}

	return result, nil
}

// ============================================================
// 依赖拓扑（DAG 拓扑排序）
// ============================================================

// GetTopology 获取服务依赖拓扑.
func (d *ServiceDashboard) GetTopology() *ServiceTopology {
	d.mu.RLock()
	defer d.mu.RUnlock()

	topo := &ServiceTopology{
		Nodes: make([]TopologyNode, 0, len(d.services)),
		Edges: make([]TopologyEdge, 0),
	}

	for _, svc := range d.services {
		topo.Nodes = append(topo.Nodes, TopologyNode{
			ID:     string(svc.ID),
			Name:   svc.Name,
			Type:   svc.Type,
			Status: svc.Status,
		})
		for _, dep := range svc.Dependencies {
			topo.Edges = append(topo.Edges, TopologyEdge{
				Source: svc.ID,
				Target: dep,
			})
		}
	}

	sort.Slice(topo.Nodes, func(i, j int) bool {
		return topo.Nodes[i].ID < topo.Nodes[j].ID
	})
	return topo
}

// TopologicalSort 对服务进行拓扑排序（依赖在前）.
func (d *ServiceDashboard) TopologicalSort() ([]ServiceID, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 构建邻接表和入度
	inDegree := make(map[ServiceID]int)
	dependents := make(map[ServiceID][]ServiceID) // dep -> dependents

	for id := range d.services {
		inDegree[id] = 0
	}

	for id, svc := range d.services {
		for _, dep := range svc.Dependencies {
			if _, exists := d.services[dep]; !exists {
				return nil, fmt.Errorf("依赖服务 %s 不存在（被 %s 引用）", dep, id)
			}
			inDegree[id]++
			dependents[dep] = append(dependents[dep], id)
		}
	}

	// Kahn 算法
	queue := make([]ServiceID, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		return queue[i] < queue[j]
	})

	result := make([]ServiceID, 0, len(d.services))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		for _, dep := range dependents[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
		sort.Slice(queue, func(i, j int) bool {
			return queue[i] < queue[j]
		})
	}

	if len(result) != len(d.services) {
		return nil, fmt.Errorf("依赖图中存在循环依赖")
	}

	return result, nil
}

// ============================================================
// 自动故障恢复
// ============================================================

// AutoRecover 自动故障恢复：检查所有运行中服务的健康状态，对异常服务按策略重启.
func (d *ServiceDashboard) AutoRecover(ctx context.Context) []GroupActionResult {
	d.mu.RLock()
	candidates := make([]ServiceID, 0)
	for id, svc := range d.services {
		if svc.Status == StatusError || svc.Status == StatusDegraded {
			if svc.RestartPolicy == RestartAlways || svc.RestartPolicy == RestartOnFailure {
				candidates = append(candidates, id)
			}
		}
	}
	d.mu.RUnlock()

	results := make([]GroupActionResult, 0, len(candidates))
	for _, id := range candidates {
		err := d.RestartService(id)
		r := GroupActionResult{ServiceID: id, Success: err == nil}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results
}

// ============================================================
// 系统总览
// ============================================================

// GetOverview 获取系统总览.
func (d *ServiceDashboard) GetOverview() *DashboardOverview {
	d.mu.RLock()
	defer d.mu.RUnlock()

	overview := &DashboardOverview{
		ServicesByType:  make(map[string]int),
		ServicesByGroup: make(map[string]int),
	}

	for _, svc := range d.services {
		overview.TotalServices++
		overview.ServicesByType[string(svc.Type)]++
		if svc.Group != "" {
			overview.ServicesByGroup[svc.Group]++
		}

		switch svc.Status {
		case StatusRunning:
			overview.RunningServices++
		case StatusStopped:
			overview.StoppedServices++
		case StatusDegraded:
			overview.DegradedServices++
		case StatusError:
			overview.ErrorServices++
		case StatusStarting:
			overview.StartingServices++
		case StatusUnknown:
			overview.UnknownServices++
		}

		if svc.HealthCheck != nil {
			if svc.HealthCheck.Healthy {
				overview.HealthyServices++
			} else {
				overview.UnhealthyServices++
			}
		}
	}

	// 统计未解决告警
	for _, a := range d.alerts {
		if !a.Resolved {
			overview.TotalAlerts++
		}
	}

	return overview
}
