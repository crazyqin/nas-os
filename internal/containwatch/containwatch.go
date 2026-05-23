package containwatch

import (
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalyCPU         AnomalyType = "cpu_spike"          // CPU异常飙升
	AnomalyMemory      AnomalyType = "memory_spike"       // 内存异常飙升
	AnomalyNetwork     AnomalyType = "suspicious_network"  // 可疑网络连接
	AnomalyFileSystem  AnomalyType = "filesystem_anomaly"  // 文件系统异常
	AnomalyPrivEsc     AnomalyType = "privilege_escalation" // 特权提升
	AnomalyEscape      AnomalyType = "container_escape"    // 容器逃逸
)

// Severity 严重程度
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// AnomalyEvent 异常事件
type AnomalyEvent struct {
	ID          string      `json:"id"`
	ContainerID string      `json:"container_id"`
	Type        AnomalyType `json:"type"`
	Severity    Severity    `json:"severity"`
	Description string      `json:"description"`
	Timestamp   time.Time   `json:"timestamp"`
	Resolved    bool        `json:"resolved"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceBaseline 资源使用基线
type ResourceBaseline struct {
	CPUAvg      float64   `json:"cpu_avg"`       // 平均CPU使用率 (%)
	CPUPeak     float64   `json:"cpu_peak"`      // CPU峰值 (%)
	MemoryAvg   float64   `json:"memory_avg"`    // 平均内存使用 (bytes)
	MemoryPeak  float64   `json:"memory_peak"`   // 内存峰值 (bytes)
	NetInAvg    float64   `json:"net_in_avg"`    // 平均入站流量 (bytes/s)
	NetOutAvg   float64   `json:"net_out_avg"`   // 平均出站流量 (bytes/s)
	IOReadAvg   float64   `json:"io_read_avg"`   // 平均读IO (bytes/s)
	IOWriteAvg  float64   `json:"io_write_avg"`  // 平均写IO (bytes/s)
	SampleCount int       `json:"sample_count"`  // 采样次数
	LastUpdated time.Time `json:"last_updated"`
}

// ContainerMetrics 容器实时指标
type ContainerMetrics struct {
	ContainerID   string    `json:"container_id"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsage   float64   `json:"memory_usage"`
	MemoryLimit   float64   `json:"memory_limit"`
	NetBytesIn    float64   `json:"net_bytes_in"`
	NetBytesOut   float64   `json:"net_bytes_out"`
	IOReadBytes   float64   `json:"io_read_bytes"`
	IOWriteBytes  float64   `json:"io_write_bytes"`
	Connections   []NetworkConnection `json:"connections"`
	Timestamp     time.Time `json:"timestamp"`
}

// NetworkConnection 网络连接信息
type NetworkConnection struct {
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr"`
	Protocol   string `json:"protocol"`
	State      string `json:"state"`
}

// WatchConfig 监控配置
type WatchConfig struct {
	BaselineWindow      time.Duration `json:"baseline_window"`       // 基线建立窗口期
	AnomalyThreshold    float64       `json:"anomaly_threshold"`     // 异常阈值倍数 (默认 3.0)
	ScanInterval        time.Duration `json:"scan_interval"`         // 扫描间隔
	MaxAnomalyHistory   int           `json:"max_anomaly_history"`   // 最大异常记录数
	TrustedPorts        []int         `json:"trusted_ports"`         // 可信端口列表
	TrustedNetworks     []string      `json:"trusted_networks"`      // 可信网络CIDR列表
}

// DefaultWatchConfig 返回默认监控配置
func DefaultWatchConfig() WatchConfig {
	return WatchConfig{
		BaselineWindow:   24 * time.Hour,
		AnomalyThreshold: 3.0,
		ScanInterval:     30 * time.Second,
		MaxAnomalyHistory: 1000,
		TrustedPorts:     []int{80, 443, 8080, 3000, 5432, 6379, 27017},
		TrustedNetworks:  []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
	}
}

// ContainerWatcher 容器安全监控器
type ContainerWatcher struct {
	mu          sync.RWMutex
	config      WatchConfig
	baselines   map[string]*ResourceBaseline  // 容器ID -> 基线
	metrics     map[string][]ContainerMetrics  // 容器ID -> 指标历史
	anomalies   map[string][]AnomalyEvent      // 容器ID -> 异常事件
	monitored   map[string]bool                // 容器ID -> 是否正在监控
	stopCh      chan struct{}
	nextID      int64
}

// NewContainerWatcher 创建容器安全监控器
func NewContainerWatcher(config WatchConfig) *ContainerWatcher {
	return &ContainerWatcher{
		config:    config,
		baselines: make(map[string]*ResourceBaseline),
		metrics:   make(map[string][]ContainerMetrics),
		anomalies: make(map[string][]AnomalyEvent),
		monitored: make(map[string]bool),
		stopCh:    make(chan struct{}),
	}
}

// StartContainerWatch 开始监控指定容器
func (w *ContainerWatcher) StartContainerWatch(containerID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.monitored[containerID] {
		return fmt.Errorf("容器 %s 已在监控中", containerID)
	}

	w.monitored[containerID] = true
	w.baselines[containerID] = &ResourceBaseline{}
	w.metrics[containerID] = make([]ContainerMetrics, 0)
	w.anomalies[containerID] = make([]AnomalyEvent, 0)

	log.Printf("开始监控容器安全: %s", containerID)
	return nil
}

// StopContainerWatch 停止监控指定容器
func (w *ContainerWatcher) StopContainerWatch(containerID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.monitored[containerID] {
		return fmt.Errorf("容器 %s 未在监控中", containerID)
	}

	delete(w.monitored, containerID)
	log.Printf("停止监控容器安全: %s", containerID)
	return nil
}

// RecordMetrics 记录容器指标（用于基线建立和异常检测）
func (w *ContainerWatcher) RecordMetrics(metrics ContainerMetrics) []AnomalyEvent {
	w.mu.Lock()
	defer w.mu.Unlock()

	containerID := metrics.ContainerID
	if !w.monitored[containerID] {
		return nil
	}

	// 保存指标
	w.metrics[containerID] = append(w.metrics[containerID], metrics)

	// 清理过期指标
	maxSamples := int(w.config.BaselineWindow / w.config.ScanInterval)
	if maxSamples < 100 {
		maxSamples = 100
	}
	if len(w.metrics[containerID]) > maxSamples {
		w.metrics[containerID] = w.metrics[containerID][len(w.metrics[containerID])-maxSamples:]
	}

	// 更新基线
	w.updateBaseline(containerID)

	// 检测异常
	anomalies := w.detectAnomalies(containerID, metrics)

	// 保存异常事件
	for _, a := range anomalies {
		w.anomalies[containerID] = append(w.anomalies[containerID], a)
		// 清理过旧的异常记录
		if len(w.anomalies[containerID]) > w.config.MaxAnomalyHistory {
			w.anomalies[containerID] = w.anomalies[containerID][1:]
		}
	}

	return anomalies
}

// updateBaseline 更新容器资源使用基线（指数移动平均）
func (w *ContainerWatcher) updateBaseline(containerID string) {
	history := w.metrics[containerID]
	baseline := w.baselines[containerID]

	if len(history) == 0 {
		return
	}

	n := float64(len(history))
	alpha := 2.0 / (n + 1) // EMA 权重因子

	latest := history[len(history)-1]

	if baseline.SampleCount == 0 {
		// 首次采样，直接赋值
		baseline.CPUAvg = latest.CPUPercent
		baseline.CPUPeak = latest.CPUPercent
		baseline.MemoryAvg = latest.MemoryUsage
		baseline.MemoryPeak = latest.MemoryUsage
		baseline.NetInAvg = latest.NetBytesIn
		baseline.NetOutAvg = latest.NetBytesOut
		baseline.IOReadAvg = latest.IOReadBytes
		baseline.IOWriteAvg = latest.IOWriteBytes
	} else {
		// EMA 更新
		baseline.CPUAvg = alpha*latest.CPUPercent + (1-alpha)*baseline.CPUAvg
		baseline.CPUPeak = math.Max(baseline.CPUPeak, latest.CPUPercent)
		baseline.MemoryAvg = alpha*latest.MemoryUsage + (1-alpha)*baseline.MemoryAvg
		baseline.MemoryPeak = math.Max(baseline.MemoryPeak, latest.MemoryUsage)
		baseline.NetInAvg = alpha*latest.NetBytesIn + (1-alpha)*baseline.NetInAvg
		baseline.NetOutAvg = alpha*latest.NetBytesOut + (1-alpha)*baseline.NetOutAvg
		baseline.IOReadAvg = alpha*latest.IOReadBytes + (1-alpha)*baseline.IOReadAvg
		baseline.IOWriteAvg = alpha*latest.IOWriteBytes + (1-alpha)*baseline.IOWriteAvg
	}

	baseline.SampleCount++
	baseline.LastUpdated = time.Now()
}

// detectAnomalies 检测异常行为
func (w *ContainerWatcher) detectAnomalies(containerID string, metrics ContainerMetrics) []AnomalyEvent {
	var anomalies []AnomalyEvent
	baseline := w.baselines[containerID]
	threshold := w.config.AnomalyThreshold

	// 只有基线有足够数据才进行异常检测
	if baseline.SampleCount < 5 {
		return anomalies
	}

	// 1. CPU异常飙升检测
	if baseline.CPUAvg > 0 && metrics.CPUPercent > baseline.CPUAvg*threshold {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyCPU, SeverityHigh,
			fmt.Sprintf("CPU使用率异常飙升: 当前 %.1f%%, 基线平均 %.1f%%", metrics.CPUPercent, baseline.CPUAvg),
			map[string]interface{}{
				"current":   metrics.CPUPercent,
				"baseline":  baseline.CPUAvg,
				"threshold": threshold,
			}))
	}

	// 2. 内存异常飙升检测
	if baseline.MemoryAvg > 0 && metrics.MemoryUsage > baseline.MemoryAvg*threshold {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyMemory, SeverityHigh,
			fmt.Sprintf("内存使用异常飙升: 当前 %.2f MB, 基线平均 %.2f MB",
				metrics.MemoryUsage/1024/1024, baseline.MemoryAvg/1024/1024),
			map[string]interface{}{
				"current":   metrics.MemoryUsage,
				"baseline":  baseline.MemoryAvg,
				"threshold": threshold,
			}))
	}

	// 3. 异常网络连接检测
	for _, conn := range metrics.Connections {
		if suspicious := w.checkSuspiciousConnection(conn); suspicious != nil {
			anomalies = append(anomalies, *suspicious)
		}
	}

	// 4. IO异常检测（防勒索 - 大量写入）
	if baseline.IOWriteAvg > 0 && metrics.IOWriteBytes > baseline.IOWriteAvg*threshold*2 {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyFileSystem, SeverityCritical,
			fmt.Sprintf("文件系统写入异常激增，可能存在加密行为: 当前 %.2f MB/s, 基线 %.2f MB/s",
				metrics.IOWriteBytes/1024/1024, baseline.IOWriteAvg/1024/1024),
			map[string]interface{}{
				"current":   metrics.IOWriteBytes,
				"baseline":  baseline.IOWriteAvg,
				"pattern":   "possible_ransomware",
			}))
	}

	return anomalies
}

// checkSuspiciousConnection 检查可疑网络连接
func (w *ContainerWatcher) checkSuspiciousConnection(conn NetworkConnection) *AnomalyEvent {
	host, portStr, err := net.SplitHostPort(conn.RemoteAddr)
	if err != nil {
		return nil
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}

	// 检查是否为外部IP（不在可信网络中）
	isTrusted := false
	for _, cidr := range w.config.TrustedNetworks {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			isTrusted = true
			break
		}
	}

	// 检查端口是否为可信端口
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	isTrustedPort := false
	for _, p := range w.config.TrustedPorts {
		if p == port {
			isTrustedPort = true
			break
		}
	}

	severity := SeverityMedium
	if !isTrusted && !isTrustedPort {
		severity = SeverityHigh
	} else if !isTrusted || !isTrustedPort {
		severity = SeverityLow
	} else {
		return nil // 都是可信的，跳过
	}

	w.nextID++
	return &AnomalyEvent{
		ID:          fmt.Sprintf("anomaly-%d-%d", time.Now().UnixMilli(), w.nextID),
		Type:        AnomalyNetwork,
		Severity:    severity,
		Description: fmt.Sprintf("可疑网络连接: %s -> %s (%s)", conn.LocalAddr, conn.RemoteAddr, conn.Protocol),
		Metadata: map[string]interface{}{
			"local_addr":  conn.LocalAddr,
			"remote_addr": conn.RemoteAddr,
			"protocol":    conn.Protocol,
			"is_external": !isTrusted,
			"is_non_standard_port": !isTrustedPort,
		},
	}
}

// CheckPrivilegeEscalation 检测特权提升
func (w *ContainerWatcher) CheckPrivilegeEscalation(containerID string, hasCapSysAdmin, hasCapSysPtrace bool, isPrivileged bool) []AnomalyEvent {
	w.mu.Lock()
	defer w.mu.Unlock()

	var anomalies []AnomalyEvent

	if isPrivileged {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyPrivEsc, SeverityCritical,
			"容器运行在特权模式，存在严重安全风险",
			map[string]interface{}{"privileged": true}))
	}

	if hasCapSysAdmin {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyPrivEsc, SeverityHigh,
			"容器拥有SYS_ADMIN能力，可用于特权提升",
			map[string]interface{}{"capability": "SYS_ADMIN"}))
	}

	if hasCapSysPtrace {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyPrivEsc, SeverityHigh,
			"容器拥有SYS_PTRACE能力，可用于进程注入攻击",
			map[string]interface{}{"capability": "SYS_PTRACE"}))
	}

	for _, a := range anomalies {
		w.anomalies[containerID] = append(w.anomalies[containerID], a)
	}

	return anomalies
}

// CheckContainerEscape 检测容器逃逸尝试
func (w *ContainerWatcher) CheckContainerEscape(containerID string, mounts []string, pidNamespace, networkNamespace bool) []AnomalyEvent {
	w.mu.Lock()
	defer w.mu.Unlock()

	var anomalies []AnomalyEvent

	// 检查危险挂载
	dangerousMounts := []string{"/proc", "/sys", "/dev", "/etc", "/var/run/docker.sock"}
	for _, mount := range mounts {
		for _, dangerous := range dangerousMounts {
			if mount == dangerous || (len(mount) > len(dangerous) && mount[:len(dangerous)] == dangerous) {
				anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyEscape, SeverityCritical,
					fmt.Sprintf("检测到危险挂载点: %s，可能被用于容器逃逸", mount),
					map[string]interface{}{"mount": mount}))
			}
		}
	}

	// 检查是否共享主机PID命名空间
	if pidNamespace {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyEscape, SeverityHigh,
			"容器共享主机PID命名空间，可查看/杀死主机进程",
			map[string]interface{}{"shared_pid_namespace": true}))
	}

	// 检查是否共享主机网络命名空间
	if networkNamespace {
		anomalies = append(anomalies, w.newAnomaly(containerID, AnomalyEscape, SeverityMedium,
			"容器共享主机网络命名空间",
			map[string]interface{}{"shared_network_namespace": true}))
	}

	for _, a := range anomalies {
		w.anomalies[containerID] = append(w.anomalies[containerID], a)
	}

	return anomalies
}

// newAnomaly 创建新的异常事件
func (w *ContainerWatcher) newAnomaly(containerID string, atype AnomalyType, severity Severity, desc string, meta map[string]interface{}) AnomalyEvent {
	w.nextID++
	return AnomalyEvent{
		ID:          fmt.Sprintf("anomaly-%d-%d", time.Now().UnixMilli(), w.nextID),
		ContainerID: containerID,
		Type:        atype,
		Severity:    severity,
		Description: desc,
		Timestamp:   time.Now(),
		Resolved:    false,
		Metadata:    meta,
	}
}

// GetAnomalies 获取容器异常事件列表
func (w *ContainerWatcher) GetAnomalies(containerID string, resolved *bool) []AnomalyEvent {
	w.mu.RLock()
	defer w.mu.RUnlock()

	events, exists := w.anomalies[containerID]
	if !exists {
		return nil
	}

	if resolved == nil {
		result := make([]AnomalyEvent, len(events))
		copy(result, events)
		return result
	}

	var filtered []AnomalyEvent
	for _, e := range events {
		if e.Resolved == *resolved {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// ResolveAnomaly 标记异常为已解决
func (w *ContainerWatcher) ResolveAnomaly(containerID, anomalyID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	events, exists := w.anomalies[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 无异常记录", containerID)
	}

	for i := range events {
		if events[i].ID == anomalyID {
			now := time.Now()
			events[i].Resolved = true
			events[i].ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("异常事件 %s 未找到", anomalyID)
}

// GetBaseline 获取容器资源基线
func (w *ContainerWatcher) GetBaseline(containerID string) (*ResourceBaseline, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	baseline, exists := w.baselines[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 无基线数据", containerID)
	}

	result := *baseline
	return &result, nil
}

// IsMonitoring 检查容器是否正在被监控
func (w *ContainerWatcher) IsMonitoring(containerID string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.monitored[containerID]
}

// ListMonitored 列出所有正在监控的容器ID
func (w *ContainerWatcher) ListMonitored() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	ids := make([]string, 0, len(w.monitored))
	for id := range w.monitored {
		ids = append(ids, id)
	}
	return ids
}

// GetWatchOverview 获取监控状态概览
func (w *ContainerWatcher) GetWatchOverview() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	totalMonitored := len(w.monitored)
	totalAnomalies := 0
	unresolvedAnomalies := 0
	criticalCount := 0

	for _, events := range w.anomalies {
		for _, e := range events {
			totalAnomalies++
			if !e.Resolved {
				unresolvedAnomalies++
				if e.Severity == SeverityCritical {
					criticalCount++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_monitored":     totalMonitored,
		"total_anomalies":     totalAnomalies,
		"unresolved_anomalies": unresolvedAnomalies,
		"critical_count":      criticalCount,
		"timestamp":           time.Now(),
	}
}
