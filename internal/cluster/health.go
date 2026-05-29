// Package cluster 提供集群健康检查与综合监控功能
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// HealthLevel 健康级别
type HealthLevel string

const (
	HealthLevelHealthy   HealthLevel = "healthy"   // 健康
	HealthLevelDegraded  HealthLevel = "degraded"  // 降级
	HealthLevelUnhealthy HealthLevel = "unhealthy" // 不健康
	HealthLevelCritical  HealthLevel = "critical"  // 严重
	HealthLevelUnknown   HealthLevel = "unknown"   // 未知
)

// CheckType 检查类型
type CheckType string

const (
	CheckTypeHeartbeat   CheckType = "heartbeat"   // 心跳检查
	CheckTypeCPU         CheckType = "cpu"         // CPU 检查
	CheckTypeMemory      CheckType = "memory"      // 内存检查
	CheckTypeDisk        CheckType = "disk"        // 磁盘检查
	CheckTypeNetwork     CheckType = "network"     // 网络检查
	CheckTypeService     CheckType = "service"     // 服务检查
	CheckTypeCustom      CheckType = "custom"      // 自定义检查
)

// HealthCheck 健康检查项
type HealthCheck struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        CheckType   `json:"type"`
	NodeID      string      `json:"node_id"`
	Level       HealthLevel `json:"level"`
	Message     string      `json:"message,omitempty"`
	Details     interface{} `json:"details,omitempty"`
	Duration    time.Duration `json:"duration"`
	LastCheck   time.Time   `json:"last_check"`
	NextCheck   time.Time   `json:"next_check"`
	Enabled     bool        `json:"enabled"`
	Threshold   float64     `json:"threshold,omitempty"` // 阈值（如 CPU 使用率 80%）
}

// ClusterHealthStatus 集群健康状态
type ClusterHealthStatus struct {
	ClusterName   string        `json:"cluster_name"`
	OverallLevel  HealthLevel   `json:"overall_level"`
	TotalNodes    int           `json:"total_nodes"`
	HealthyNodes  int           `json:"healthy_nodes"`
	DegradedNodes int           `json:"degraded_nodes"`
	UnhealthyNodes int          `json:"unhealthy_nodes"`
	OfflineNodes  int           `json:"offline_nodes"`
	Checks        []*HealthCheck `json:"checks"`
	LastUpdate    time.Time     `json:"last_update"`
	Uptime        time.Duration `json:"uptime"`
}

// AlertSeverity 告警级别
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Alert 告警
type Alert struct {
	ID        string        `json:"id"`
	Severity  AlertSeverity `json:"severity"`
	NodeID    string        `json:"node_id"`
	CheckID   string        `json:"check_id"`
	Message   string        `json:"message"`
	Details   interface{}   `json:"details,omitempty"`
	Status    string        `json:"status"` // active, acknowledged, resolved
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	ResolvedAt *time.Time   `json:"resolved_at,omitempty"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	CheckType   CheckType   `json:"check_type"`
	Condition   string      `json:"condition"` // gt, lt, eq, ne
	Threshold   float64     `json:"threshold"`
	Severity    AlertSeverity `json:"severity"`
	Duration    time.Duration `json:"duration"` // 持续多久才触发
	Enabled     bool        `json:"enabled"`
	NodeFilter  []string    `json:"node_filter,omitempty"` // 为空表示所有节点
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	// 检查间隔
	CheckInterval time.Duration `json:"check_interval"`

	// 心跳超时
	HeartbeatTimeout time.Duration `json:"heartbeat_timeout"`

	// CPU 阈值（百分比）
	CPUThreshold float64 `json:"cpu_threshold"`

	// 内存阈值（百分比）
	MemoryThreshold float64 `json:"memory_threshold"`

	// 磁盘阈值（百分比）
	DiskThreshold float64 `json:"disk_threshold"`

	// 网络延迟阈值（毫秒）
	NetworkLatencyThreshold float64 `json:"network_latency_threshold"`

	// 保留历史记录数量
	MaxHistoryCount int `json:"max_history_count"`

	// 自动恢复检查
	AutoRecoveryCheck bool `json:"auto_recovery_check"`

	// 恢复检查间隔
	RecoveryCheckInterval time.Duration `json:"recovery_check_interval"`
}

// DefaultHealthCheckConfig 默认健康检查配置
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		CheckInterval:           30 * time.Second,
		HeartbeatTimeout:        60 * time.Second,
		CPUThreshold:            80.0,
		MemoryThreshold:         85.0,
		DiskThreshold:           90.0,
		NetworkLatencyThreshold: 1000.0, // 1秒
		MaxHistoryCount:         1000,
		AutoRecoveryCheck:       true,
		RecoveryCheckInterval:   5 * time.Minute,
	}
}

// HealthChecker 集群健康检查器
type HealthChecker struct {
	mu            sync.RWMutex
	manager       *Manager
	config        *HealthCheckConfig
	checks        map[string]*HealthCheck
	alerts        map[string]*Alert
	alertRules    map[string]*AlertRule
	history       []*HealthCheckStatus
	alertHandlers []func(alert *Alert)
	stopChan      chan struct{}
	running       bool
	startTime     time.Time
}

// HealthCheckStatus 健康检查状态快照
type HealthCheckStatus struct {
	Timestamp time.Time          `json:"timestamp"`
	Level     HealthLevel        `json:"level"`
	Checks    map[string]HealthLevel `json:"checks"`
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(manager *Manager, config *HealthCheckConfig) *HealthChecker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	return &HealthChecker{
		manager:    manager,
		config:     config,
		checks:     make(map[string]*HealthCheck),
		alerts:     make(map[string]*Alert),
		alertRules: make(map[string]*AlertRule),
		history:    make([]*HealthCheckStatus, 0),
		stopChan:   make(chan struct{}),
	}
}

// Start 启动健康检查器
func (hc *HealthChecker) Start(ctx context.Context) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return fmt.Errorf("health checker is already running")
	}

	hc.running = true
	hc.startTime = time.Now()

	// 初始化默认检查
	hc.initDefaultChecks()

	// 初始化默认告警规则
	hc.initDefaultAlertRules()

	// 启动检查循环
	go hc.checkLoop(ctx)

	// 启动告警处理循环
	go hc.alertLoop(ctx)

	return nil
}

// Stop 停止健康检查器
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return
	}

	close(hc.stopChan)
	hc.running = false
}

// initDefaultChecks 初始化默认检查
func (hc *HealthChecker) initDefaultChecks() {
	defaultChecks := []*HealthCheck{
		{
			ID:        "check-heartbeat",
			Name:      "心跳检查",
			Type:      CheckTypeHeartbeat,
			Enabled:   true,
			Threshold: float64(hc.config.HeartbeatTimeout.Seconds()),
		},
		{
			ID:        "check-cpu",
			Name:      "CPU 使用率检查",
			Type:      CheckTypeCPU,
			Enabled:   true,
			Threshold: hc.config.CPUThreshold,
		},
		{
			ID:        "check-memory",
			Name:      "内存使用率检查",
			Type:      CheckTypeMemory,
			Enabled:   true,
			Threshold: hc.config.MemoryThreshold,
		},
		{
			ID:        "check-disk",
			Name:      "磁盘使用率检查",
			Type:      CheckTypeDisk,
			Enabled:   true,
			Threshold: hc.config.DiskThreshold,
		},
		{
			ID:        "check-network",
			Name:      "网络延迟检查",
			Type:      CheckTypeNetwork,
			Enabled:   true,
			Threshold: hc.config.NetworkLatencyThreshold,
		},
	}

	for _, check := range defaultChecks {
		hc.checks[check.ID] = check
	}
}

// initDefaultAlertRules 初始化默认告警规则
func (hc *HealthChecker) initDefaultAlertRules() {
	defaultRules := []*AlertRule{
		{
			ID:        "rule-cpu-high",
			Name:      "CPU 使用率过高",
			CheckType: CheckTypeCPU,
			Condition: "gt",
			Threshold: hc.config.CPUThreshold,
			Severity:  AlertSeverityWarning,
			Duration:  5 * time.Minute,
			Enabled:   true,
		},
		{
			ID:        "rule-memory-high",
			Name:      "内存使用率过高",
			CheckType: CheckTypeMemory,
			Condition: "gt",
			Threshold: hc.config.MemoryThreshold,
			Severity:  AlertSeverityWarning,
			Duration:  5 * time.Minute,
			Enabled:   true,
		},
		{
			ID:        "rule-disk-high",
			Name:      "磁盘使用率过高",
			CheckType: CheckTypeDisk,
			Condition: "gt",
			Threshold: hc.config.DiskThreshold,
			Severity:  AlertSeverityError,
			Duration:  10 * time.Minute,
			Enabled:   true,
		},
		{
			ID:        "rule-heartbeat-timeout",
			Name:      "心跳超时",
			CheckType: CheckTypeHeartbeat,
			Condition: "gt",
			Threshold: hc.config.HeartbeatTimeout.Seconds(),
			Severity:  AlertSeverityCritical,
			Duration:  0,
			Enabled:   true,
		},
	}

	for _, rule := range defaultRules {
		hc.alertRules[rule.ID] = rule
	}
}

// AddCheck 添加自定义检查
func (hc *HealthChecker) AddCheck(check *HealthCheck) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if check.ID == "" {
		return fmt.Errorf("check ID is required")
	}

	if _, exists := hc.checks[check.ID]; exists {
		return fmt.Errorf("check %s already exists", check.ID)
	}

	hc.checks[check.ID] = check
	return nil
}

// RemoveCheck 移除检查
func (hc *HealthChecker) RemoveCheck(checkID string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if _, exists := hc.checks[checkID]; !exists {
		return fmt.Errorf("check %s not found", checkID)
	}

	delete(hc.checks, checkID)
	return nil
}

// GetCheck 获取检查项
func (hc *HealthChecker) GetCheck(checkID string) (*HealthCheck, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	check, exists := hc.checks[checkID]
	if !exists {
		return nil, fmt.Errorf("check %s not found", checkID)
	}

	checkCopy := *check
	return &checkCopy, nil
}

// ListChecks 列出所有检查项
func (hc *HealthChecker) ListChecks() []*HealthCheck {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	checks := make([]*HealthCheck, 0, len(hc.checks))
	for _, check := range hc.checks {
		checkCopy := *check
		checks = append(checks, &checkCopy)
	}
	return checks
}

// AddAlertRule 添加告警规则
func (hc *HealthChecker) AddAlertRule(rule *AlertRule) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}

	if _, exists := hc.alertRules[rule.ID]; exists {
		return fmt.Errorf("rule %s already exists", rule.ID)
	}

	hc.alertRules[rule.ID] = rule
	return nil
}

// RemoveAlertRule 移除告警规则
func (hc *HealthChecker) RemoveAlertRule(ruleID string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if _, exists := hc.alertRules[ruleID]; !exists {
		return fmt.Errorf("rule %s not found", ruleID)
	}

	delete(hc.alertRules, ruleID)
	return nil
}

// ListAlertRules 列出所有告警规则
func (hc *HealthChecker) ListAlertRules() []*AlertRule {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(hc.alertRules))
	for _, rule := range hc.alertRules {
		ruleCopy := *rule
		rules = append(rules, &ruleCopy)
	}
	return rules
}

// OnAlert 注册告警处理器
func (hc *HealthChecker) OnAlert(handler func(alert *Alert)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.alertHandlers = append(hc.alertHandlers, handler)
}

// GetClusterHealth 获取集群健康状态
func (hc *HealthChecker) GetClusterHealth() *ClusterHealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := &ClusterHealthStatus{
		ClusterName: hc.manager.config.Name,
		OverallLevel: HealthLevelHealthy,
		LastUpdate:   time.Now(),
		Uptime:       time.Since(hc.startTime),
	}

	// 统计节点状态
	hc.manager.nodesMutex.RLock()
	defer hc.manager.nodesMutex.RUnlock()

	status.TotalNodes = len(hc.manager.nodes)
	for _, node := range hc.manager.nodes {
		switch node.Status {
		case StatusOnline:
			status.HealthyNodes++
		case StatusDegraded:
			status.DegradedNodes++
		case StatusOffline:
			status.OfflineNodes++
		default:
			status.UnhealthyNodes++
		}
	}

	// 计算整体健康级别
	if status.UnhealthyNodes > 0 || status.OfflineNodes > status.TotalNodes/2 {
		status.OverallLevel = HealthLevelCritical
	} else if status.OfflineNodes > 0 || status.DegradedNodes > status.TotalNodes/3 {
		status.OverallLevel = HealthLevelUnhealthy
	} else if status.DegradedNodes > 0 {
		status.OverallLevel = HealthLevelDegraded
	}

	// 添加检查结果
	status.Checks = make([]*HealthCheck, 0, len(hc.checks))
	for _, check := range hc.checks {
		checkCopy := *check
		status.Checks = append(status.Checks, &checkCopy)
	}

	return status
}

// GetNodeHealth 获取节点健康状态
func (hc *HealthChecker) GetNodeHealth(nodeID string) (HealthLevel, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	// 检查节点是否存在
	hc.manager.nodesMutex.RLock()
	node, exists := hc.manager.nodes[nodeID]
	hc.manager.nodesMutex.RUnlock()

	if !exists {
		return HealthLevelUnknown, fmt.Errorf("node %s not found", nodeID)
	}

	// 根据节点状态返回健康级别
	switch node.Status {
	case StatusOnline:
		return HealthLevelHealthy, nil
	case StatusDegraded:
		return HealthLevelDegraded, nil
	case StatusOffline:
		return HealthLevelUnhealthy, nil
	default:
		return HealthLevelUnknown, nil
	}
}

// GetAlerts 获取告警列表
func (hc *HealthChecker) GetAlerts(status string, severity AlertSeverity, limit int) []*Alert {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	alerts := make([]*Alert, 0)
	for _, alert := range hc.alerts {
		if status != "" && alert.Status != status {
			continue
		}
		if severity != "" && alert.Severity != severity {
			continue
		}
		alertCopy := *alert
		alerts = append(alerts, &alertCopy)
	}

	// 按时间倒序排序
	for i := 0; i < len(alerts)-1; i++ {
		for j := i + 1; j < len(alerts); j++ {
			if alerts[j].CreatedAt.After(alerts[i].CreatedAt) {
				alerts[i], alerts[j] = alerts[j], alerts[i]
			}
		}
	}

	if limit > 0 && limit < len(alerts) {
		alerts = alerts[:limit]
	}

	return alerts
}

// AcknowledgeAlert 确认告警
func (hc *HealthChecker) AcknowledgeAlert(alertID string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	alert, exists := hc.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	if alert.Status != "active" {
		return fmt.Errorf("alert %s is not active", alertID)
	}

	alert.Status = "acknowledged"
	alert.UpdatedAt = time.Now()
	return nil
}

// ResolveAlert 解决告警
func (hc *HealthChecker) ResolveAlert(alertID string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	alert, exists := hc.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	if alert.Status == "resolved" {
		return fmt.Errorf("alert %s is already resolved", alertID)
	}

	now := time.Now()
	alert.Status = "resolved"
	alert.UpdatedAt = now
	alert.ResolvedAt = &now
	return nil
}

// GetHistory 获取健康检查历史
func (hc *HealthChecker) GetHistory(limit int) []*HealthCheckStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if limit <= 0 || limit > len(hc.history) {
		limit = len(hc.history)
	}

	start := len(hc.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*HealthCheckStatus, limit)
	copy(result, hc.history[start:])
	return result
}

// checkLoop 检查循环
func (hc *HealthChecker) checkLoop(ctx context.Context) {
	ticker := time.NewTicker(hc.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopChan:
			return
		case <-ticker.C:
			hc.runChecks()
		}
	}
}

// runChecks 执行所有检查
func (hc *HealthChecker) runChecks() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	status := &HealthCheckStatus{
		Timestamp: time.Now(),
		Level:     HealthLevelHealthy,
		Checks:    make(map[string]HealthLevel),
	}

	for _, check := range hc.checks {
		if !check.Enabled {
			continue
		}

		level := hc.executeCheck(check)
		check.Level = level
		check.LastCheck = time.Now()
		check.NextCheck = time.Now().Add(hc.config.CheckInterval)

		status.Checks[check.ID] = level

		// 更新整体状态
		if level == HealthLevelCritical || level == HealthLevelUnhealthy {
			status.Level = level
		} else if level == HealthLevelDegraded && status.Level == HealthLevelHealthy {
			status.Level = HealthLevelDegraded
		}
	}

	// 保存历史
	hc.history = append(hc.history, status)
	if len(hc.history) > hc.config.MaxHistoryCount {
		hc.history = hc.history[len(hc.history)-hc.config.MaxHistoryCount:]
	}
}

// executeCheck 执行单个检查
func (hc *HealthChecker) executeCheck(check *HealthCheck) HealthLevel {
	start := time.Now()

	var level HealthLevel

	switch check.Type {
	case CheckTypeHeartbeat:
		level = hc.checkHeartbeat(check)
	case CheckTypeCPU:
		level = hc.checkCPU(check)
	case CheckTypeMemory:
		level = hc.checkMemory(check)
	case CheckTypeDisk:
		level = hc.checkDisk(check)
	case CheckTypeNetwork:
		level = hc.checkNetwork(check)
	default:
		level = HealthLevelUnknown
	}

	check.Duration = time.Since(start)
	return level
}

// checkHeartbeat 心跳检查
func (hc *HealthChecker) checkHeartbeat(check *HealthCheck) HealthLevel {
	hc.manager.nodesMutex.RLock()
	defer hc.manager.nodesMutex.RUnlock()

	timeout := time.Duration(check.Threshold) * time.Second
	offlineCount := 0

	for _, node := range hc.manager.nodes {
		if time.Since(node.Heartbeat) > timeout {
			offlineCount++
		}
	}

	if offlineCount == 0 {
		return HealthLevelHealthy
	} else if offlineCount < len(hc.manager.nodes)/2 {
		return HealthLevelDegraded
	}
	return HealthLevelUnhealthy
}

// checkCPU CPU 检查
func (hc *HealthChecker) checkCPU(check *HealthCheck) HealthLevel {
	hc.manager.nodesMutex.RLock()
	defer hc.manager.nodesMutex.RUnlock()

	highCPUCount := 0
	for _, node := range hc.manager.nodes {
		if node.Metrics.CPUUsage > check.Threshold {
			highCPUCount++
		}
	}

	if highCPUCount == 0 {
		return HealthLevelHealthy
	} else if highCPUCount < len(hc.manager.nodes)/3 {
		return HealthLevelDegraded
	}
	return HealthLevelUnhealthy
}

// checkMemory 内存检查
func (hc *HealthChecker) checkMemory(check *HealthCheck) HealthLevel {
	hc.manager.nodesMutex.RLock()
	defer hc.manager.nodesMutex.RUnlock()

	highMemCount := 0
	for _, node := range hc.manager.nodes {
		if node.Metrics.MemoryUsage > check.Threshold {
			highMemCount++
		}
	}

	if highMemCount == 0 {
		return HealthLevelHealthy
	} else if highMemCount < len(hc.manager.nodes)/3 {
		return HealthLevelDegraded
	}
	return HealthLevelUnhealthy
}

// checkDisk 磁盘检查
func (hc *HealthChecker) checkDisk(check *HealthCheck) HealthLevel {
	hc.manager.nodesMutex.RLock()
	defer hc.manager.nodesMutex.RUnlock()

	highDiskCount := 0
	for _, node := range hc.manager.nodes {
		if node.Metrics.DiskUsage > check.Threshold {
			highDiskCount++
		}
	}

	if highDiskCount == 0 {
		return HealthLevelHealthy
	} else if highDiskCount < len(hc.manager.nodes)/3 {
		return HealthLevelDegraded
	}
	return HealthLevelUnhealthy
}

// checkNetwork 网络检查
func (hc *HealthChecker) checkNetwork(check *HealthCheck) HealthLevel {
	// 简化实现：检查节点连接数
	hc.manager.nodesMutex.RLock()
	defer hc.manager.nodesMutex.RUnlock()

	// 网络检查逻辑
	return HealthLevelHealthy
}

// alertLoop 告警处理循环
func (hc *HealthChecker) alertLoop(ctx context.Context) {
	ticker := time.NewTicker(hc.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopChan:
			return
		case <-ticker.C:
			hc.evaluateAlertRules()
		}
	}
}

// evaluateAlertRules 评估告警规则
func (hc *HealthChecker) evaluateAlertRules() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for _, rule := range hc.alertRules {
		if !rule.Enabled {
			continue
		}

		// 获取相关检查结果
		check, exists := hc.checks[string(rule.CheckType)]
		if !exists {
			continue
		}

		// 检查是否满足条件
		triggered := false
		switch rule.Condition {
		case "gt":
			triggered = check.Threshold > rule.Threshold
		case "lt":
			triggered = check.Threshold < rule.Threshold
		case "eq":
			triggered = check.Threshold == rule.Threshold
		case "ne":
			triggered = check.Threshold != rule.Threshold
		}

		if triggered {
			// 检查是否已有相同告警
			alertKey := fmt.Sprintf("%s-%s", rule.ID, check.NodeID)
			if existingAlert, exists := hc.alerts[alertKey]; exists && existingAlert.Status == "active" {
				continue
			}

			// 创建新告警
			alert := &Alert{
				ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
				Severity:  rule.Severity,
				NodeID:    check.NodeID,
				CheckID:   check.ID,
				Message:   fmt.Sprintf("%s: 当前值 %.2f 超过阈值 %.2f", rule.Name, check.Threshold, rule.Threshold),
				Status:    "active",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			hc.alerts[alertKey] = alert

			// 触发告警处理器
			for _, handler := range hc.alertHandlers {
				go handler(alert)
			}
		}
	}
}

// GetStats 获取统计信息
func (hc *HealthChecker) GetStats() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	stats := map[string]interface{}{
		"total_checks":    len(hc.checks),
		"enabled_checks":  0,
		"total_rules":     len(hc.alertRules),
		"enabled_rules":   0,
		"active_alerts":   0,
		"total_alerts":    len(hc.alerts),
		"history_count":   len(hc.history),
		"uptime":          time.Since(hc.startTime).String(),
	}

	for _, check := range hc.checks {
		if check.Enabled {
			stats["enabled_checks"] = stats["enabled_checks"].(int) + 1
		}
	}

	for _, rule := range hc.alertRules {
		if rule.Enabled {
			stats["enabled_rules"] = stats["enabled_rules"].(int) + 1
		}
	}

	for _, alert := range hc.alerts {
		if alert.Status == "active" {
			stats["active_alerts"] = stats["active_alerts"].(int) + 1
		}
	}

	return stats
}

// ToJSON 导出为 JSON
func (hc *HealthChecker) ToJSON() ([]byte, error) {
	status := hc.GetClusterHealth()
	return json.MarshalIndent(status, "", "  ")
}

// IsRunning 检查是否运行中
func (hc *HealthChecker) IsRunning() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.running
}
