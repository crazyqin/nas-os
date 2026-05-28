package containerhealthpro

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"time"
)

// RegisterContainer 注册容器到增强健康监控
func (m *Manager) RegisterContainer(container *ContainerHealthPro) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if container.ContainerID == "" {
		return fmt.Errorf("container ID 不能为空")
	}
	if _, exists := m.containers[container.ContainerID]; exists {
		return fmt.Errorf("容器 %s 已注册", container.ContainerID)
	}

	// 设置默认值
	if container.HealthCheck.MaxRetries == 0 {
		container.HealthCheck.MaxRetries = 3
	}
	if container.RecoveryPolicy == "" {
		container.RecoveryPolicy = RecoveryRestart
	}
	container.Status = StatusStarting

	m.containers[container.ContainerID] = container
	log.Printf("容器增强健康监控已注册: %s (%s)", container.Name, container.ContainerID)
	return nil
}

// UnregisterContainer 注销容器健康监控
func (m *Manager) UnregisterContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.containers[containerID]; !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}

	delete(m.containers, containerID)
	delete(m.dependencies, containerID)
	delete(m.alerts, containerID)
	delete(m.history, containerID)
	log.Printf("容器增强健康监控已注销: %s", containerID)
	return nil
}

// GetContainer 获取容器健康状态详情
func (m *Manager) GetContainer(containerID string) (*ContainerHealthPro, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 未注册", containerID)
	}
	result := *container
	return &result, nil
}

// ListContainers 列出所有监控中的容器
func (m *Manager) ListContainers() []ContainerHealthPro {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ContainerHealthPro, 0, len(m.containers))
	for _, c := range m.containers {
		result = append(result, *c)
	}
	return result
}

// CheckHealth 检查指定容器健康状态
func (m *Manager) CheckHealth(containerID string) (*ContainerHealthPro, error) {
	m.mu.RLock()
	container, exists := m.containers[containerID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("容器 %s 未注册", containerID)
	}

	startTime := time.Now()
	healthy := m.doHealthCheck(&container.HealthCheck)
	responseTime := time.Since(startTime)

	m.mu.Lock()
	container.LastCheck = time.Now()

	// 记录历史
	history := HealthHistory{
		Timestamp:    time.Now(),
		ResponseTime: responseTime,
	}
	if healthy {
		container.Status = StatusHealthy
		container.LastHealthy = time.Now()
		container.FailCount = 0
		container.ErrorMessage = ""
		history.Status = StatusHealthy
	} else {
		container.FailCount++
		history.Status = StatusUnhealthy
		history.Error = "健康检查失败"
		container.ErrorMessage = "健康检查失败"

		if container.FailCount >= container.HealthCheck.MaxRetries {
			container.Status = StatusUnhealthy
			// 触发告警
			m.triggerAlert(containerID, AlertCritical, fmt.Sprintf("容器 %s 连续%d次健康检查失败", container.Name, container.FailCount))
			// 执行恢复策略
			if container.AutoRestart {
				m.executeRecovery(containerID, container.RecoveryPolicy)
			}
		} else {
			container.Status = StatusDegraded
		}
	}

	// 添加历史记录
	m.addHistory(containerID, history)

	result := *container
	m.mu.Unlock()

	return &result, nil
}

// CheckAllHealth 检查所有容器健康状态
func (m *Manager) CheckAllHealth() []ContainerHealthPro {
	m.mu.RLock()
	ids := make([]string, 0, len(m.containers))
	for id := range m.containers {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	var results []ContainerHealthPro
	for _, id := range ids {
		ch, err := m.CheckHealth(id)
		if err != nil {
			log.Printf("检查容器 %s 失败: %v", id, err)
			continue
		}
		results = append(results, *ch)
	}
	return results
}

// SetAutoRestart 设置容器自动重启开关
func (m *Manager) SetAutoRestart(containerID string, enable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}
	container.AutoRestart = enable
	log.Printf("容器 %s 自动重启设置为: %v", containerID, enable)
	return nil
}

// SetRecoveryPolicy 设置容器恢复策略
func (m *Manager) SetRecoveryPolicy(containerID string, policy RecoveryPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}
	container.RecoveryPolicy = policy
	log.Printf("容器 %s 恢复策略设置为: %v", containerID, policy)
	return nil
}

// RestartContainer 手动重启指定容器
func (m *Manager) RestartContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.containers[containerID]; !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}
	return m.executeRecovery(containerID, RecoveryRestart)
}

// SetDependency 设置容器依赖关系
func (m *Manager) SetDependency(containerID string, dependsOn []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.containers[containerID]; !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}

	dep := &ContainerDependency{
		ContainerID: containerID,
		DependsOn:   dependsOn,
		RequiredBy:  make([]string, 0),
		StartOrder:  len(dependsOn) + 1,
	}
	m.dependencies[containerID] = dep
	m.containers[containerID].Dependency = *dep

	// 更新被依赖关系
	for _, depID := range dependsOn {
		if d, exists := m.dependencies[depID]; exists {
			d.RequiredBy = append(d.RequiredBy, containerID)
		}
	}

	log.Printf("容器 %s 依赖关系已设置: %v", containerID, dependsOn)
	return nil
}

// GetDependencyGraph 获取容器依赖关系图
func (m *Manager) GetDependencyGraph() map[string]*ContainerDependency {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ContainerDependency)
	for id, dep := range m.dependencies {
		d := *dep
		result[id] = &d
	}
	return result
}

// UpdateResourceUsage 更新容器资源使用情况
func (m *Manager) UpdateResourceUsage(containerID string, usage ResourceUsage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}

	usage.Timestamp = time.Now()
	container.ResourceUsage = usage

	// 检查资源是否超限
	deviations := m.checkResourceDeviation(container)
	if len(deviations) > 0 {
		container.Deviations = deviations
		for _, dev := range deviations {
			if dev.Alert {
				m.triggerAlert(containerID, AlertWarning, fmt.Sprintf("容器 %s %s 超过阈值: %.2f%% (阈值: %.2f%%)", container.Name, dev.Metric, dev.Current, dev.Threshold))
			}
		}
	}

	return nil
}

// AnalyzeLogs 分析容器日志
func (m *Manager) AnalyzeLogs(containerID string, logs []LogEntry) []LogPattern {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil
	}

	patterns := make([]LogPattern, 0)
	for _, pattern := range m.logPatterns {
		count := 0
		var lastSeen time.Time
		for _, log := range logs {
			if log.ContainerID == containerID && matchPattern(log.Message, pattern.Pattern) {
				count++
				if log.Timestamp.After(lastSeen) {
					lastSeen = log.Timestamp
				}
			}
		}
		if count > 0 {
			p := LogPattern{
				Pattern:     pattern.Pattern,
				Severity:    pattern.Severity,
				Count:       count,
				LastSeen:    lastSeen,
				Description: pattern.Description,
			}
			patterns = append(patterns, p)

			if pattern.Severity == AlertCritical {
				m.triggerAlert(containerID, AlertCritical, fmt.Sprintf("容器 %s 检测到严重日志模式: %s (出现%d次)", container.Name, pattern.Pattern, count))
			}
		}
	}

	return patterns
}

// AddLogPattern 添加日志异常模式
func (m *Manager) AddLogPattern(pattern LogPattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logPatterns = append(m.logPatterns, pattern)
}

// UpdateBaseline 更新容器性能基线
func (m *Manager) UpdateBaseline(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}

	history := m.history[containerID]
	if len(history) == 0 {
		return fmt.Errorf("容器 %s 无历史数据", containerID)
	}

	// 计算基线（简化版，实际应从历史数据计算）
	container.Baseline = PerformanceBaseline{
		CPUPercentAvg:    container.ResourceUsage.CPUPercent,
		MemoryPercentAvg: container.ResourceUsage.MemoryPercent,
		SampleCount:      len(history),
		LastUpdated:      time.Now(),
	}

	return nil
}

// RunSecurityScan 执行安全扫描
func (m *Manager) RunSecurityScan(containerID string) (*SecurityScanResult, error) {
	m.mu.RLock()
	container, exists := m.containers[containerID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("容器 %s 未注册", containerID)
	}

	// 模拟安全扫描
	result := &SecurityScanResult{
		ContainerID: containerID,
		ImageName:   container.Image,
		ScanTime:    time.Now(),
		Score:       85.0,
		Status:      "pass",
		Vulnerabilities: []Vulnerability{
			{
				ID:       "CVE-2024-0001",
				Severity: "low",
				Package:  "example-pkg",
				Version:  "1.0.0",
				FixedIn:  "1.0.1",
			},
		},
	}

	m.mu.Lock()
	container.SecurityScan = result
	m.mu.Unlock()

	return result, nil
}

// GetHealthTrend 获取容器健康趋势
func (m *Manager) GetHealthTrend(containerID string, period string) (*HealthTrend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.containers[containerID]; !exists {
		return nil, fmt.Errorf("容器 %s 未注册", containerID)
	}

	history := m.history[containerID]
	totalChecks := len(history)
	if totalChecks == 0 {
		return &HealthTrend{
			ContainerID: containerID,
			Period:      period,
		}, nil
	}

	healthyChecks := 0
	var totalResponseTime time.Duration
	for _, h := range history {
		if h.Status == StatusHealthy {
			healthyChecks++
		}
		totalResponseTime += h.ResponseTime
	}

	uptimePercent := float64(healthyChecks) / float64(totalChecks) * 100
	avgResponseTime := float64(totalResponseTime.Milliseconds()) / float64(totalChecks)

	return &HealthTrend{
		ContainerID:     containerID,
		Period:          period,
		UptimePercent:   uptimePercent,
		AvgResponseTime: avgResponseTime,
		IncidentCount:   totalChecks - healthyChecks,
	}, nil
}

// GetAlerts 获取容器告警列表
func (m *Manager) GetAlerts(containerID string) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := m.alerts[containerID]
	result := make([]Alert, len(alerts))
	copy(result, alerts)
	return result
}

// GetHealthReport 生成健康统计报告
func (m *Manager) GetHealthReport() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.containers)
	healthy := 0
	unhealthy := 0
	degraded := 0
	starting := 0
	stopped := 0
	totalRestarts := 0
	totalAlerts := 0

	for _, c := range m.containers {
		switch c.Status {
		case StatusHealthy:
			healthy++
		case StatusUnhealthy:
			unhealthy++
		case StatusDegraded:
			degraded++
		case StatusStarting:
			starting++
		case StatusStopped:
			stopped++
		}
		totalRestarts += c.RestartCount
	}

	for _, alerts := range m.alerts {
		totalAlerts += len(alerts)
	}

	return map[string]interface{}{
		"total_containers": total,
		"healthy":          healthy,
		"unhealthy":        unhealthy,
		"degraded":         degraded,
		"starting":         starting,
		"stopped":          stopped,
		"total_restarts":   totalRestarts,
		"total_alerts":     totalAlerts,
		"timestamp":        time.Now(),
	}
}

// 内部方法

func (m *Manager) doHealthCheck(cfg *HealthCheckConfig) bool {
	timeout := 5 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	switch cfg.Type {
	case HealthCheckHTTP:
		return checkHTTP(cfg.Endpoint, cfg.ExpectedStatus, cfg.Headers, timeout)
	case HealthCheckTCP:
		return checkTCP(cfg.Endpoint, cfg.Port, timeout)
	case HealthCheckCmd:
		return checkCommand(cfg.Command, timeout)
	case HealthCheckProcess:
		return checkProcess(cfg.ProcessName)
	default:
		return false
	}
}

func checkHTTP(endpoint string, expectedStatus int, headers map[string]string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return false
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if expectedStatus > 0 {
		return resp.StatusCode == expectedStatus
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func checkTCP(host string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func checkCommand(command string, timeout time.Duration) bool {
	ctx := fmt.Sprintf("timeout %ds", int(timeout.Seconds()))
	cmd := exec.Command("sh", "-c", ctx+" "+command)
	err := cmd.Run()
	return err == nil
}

func checkProcess(processName string) bool {
	cmd := exec.Command("pgrep", "-f", processName)
	err := cmd.Run()
	return err == nil
}

func (m *Manager) executeRecovery(containerID string, policy RecoveryPolicy) error {
	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}

	log.Printf("执行恢复策略 %s 容器: %s (%s)", policy, container.Name, containerID)

	switch policy {
	case RecoveryRestart:
		// 模拟重启延迟
		time.Sleep(100 * time.Millisecond)
		container.RestartCount++
		container.Status = StatusStarting
		container.FailCount = 0
		container.ErrorMessage = ""
		log.Printf("容器 %s 重启完成 (第%d次)", container.Name, container.RestartCount)
	case RecoveryRedeploy:
		// 模拟重新部署
		time.Sleep(200 * time.Millisecond)
		container.RestartCount++
		container.Status = StatusStarting
		container.FailCount = 0
		log.Printf("容器 %s 重新部署完成", container.Name)
	case RecoveryFailover:
		// 模拟故障转移
		container.Status = StatusStopped
		log.Printf("容器 %s 故障转移完成", container.Name)
	case RecoveryNone:
		container.Status = StatusUnhealthy
		log.Printf("容器 %s 标记为不健康", container.Name)
	}

	return nil
}

func (m *Manager) checkResourceDeviation(container *ContainerHealthPro) []PerformanceDeviation {
	var deviations []PerformanceDeviation

	if container.ResourceLimits.CPUPercent > 0 {
		dev := PerformanceDeviation{
			Metric:    "cpu",
			Baseline:  container.Baseline.CPUPercentAvg,
			Current:   container.ResourceUsage.CPUPercent,
			Threshold: container.ResourceLimits.CPUPercent,
		}
		dev.Deviation = (dev.Current - dev.Baseline) / dev.Baseline * 100
		dev.Alert = dev.Current > dev.Threshold
		deviations = append(deviations, dev)
	}

	if container.ResourceLimits.MemoryPercent > 0 {
		dev := PerformanceDeviation{
			Metric:    "memory",
			Baseline:  container.Baseline.MemoryPercentAvg,
			Current:   container.ResourceUsage.MemoryPercent,
			Threshold: container.ResourceLimits.MemoryPercent,
		}
		dev.Deviation = (dev.Current - dev.Baseline) / dev.Baseline * 100
		dev.Alert = dev.Current > dev.Threshold
		deviations = append(deviations, dev)
	}

	return deviations
}

func (m *Manager) triggerAlert(containerID string, severity AlertSeverity, message string) {
	alert := Alert{
		ID:          fmt.Sprintf("%s-%d", containerID, time.Now().UnixNano()),
		ContainerID: containerID,
		Severity:    severity,
		Message:     message,
		Timestamp:   time.Now(),
	}
	m.alerts[containerID] = append(m.alerts[containerID], alert)
	log.Printf("告警 [%s] 容器 %s: %s", severity, containerID, message)
}

func (m *Manager) addHistory(containerID string, history HealthHistory) {
	m.history[containerID] = append(m.history[containerID], history)
	// 限制历史记录数量
	if len(m.history[containerID]) > m.maxHistory {
		m.history[containerID] = m.history[containerID][len(m.history[containerID])-m.maxHistory:]
	}
}

func matchPattern(message, pattern string) bool {
	// 简化版模式匹配，实际应使用正则表达式
	return len(message) > 0 && len(pattern) > 0
}
