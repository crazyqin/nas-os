package smb

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled            bool     `json:"enabled"`
	CheckIntervalMs    int      `json:"check_interval_ms"`     // 检查间隔(毫秒)
	TimeoutMs          int      `json:"timeout_ms"`            // 检查超时(毫秒)
	HealthyThreshold   int      `json:"healthy_threshold"`     // 健康阈值
	UnhealthyThreshold int      `json:"unhealthy_threshold"`   // 不健康阈值
	RetryIntervalMs    int      `json:"retry_interval_ms"`     // 重试间隔(毫秒)
	EnableHTTPCheck    bool     `json:"enable_http_check"`     // 启用HTTP健康检查
	HTTPEndpoint       string   `json:"http_endpoint"`         // HTTP健康检查端点
	HTTPPort           int      `json:"http_port"`             // HTTP端口
	EnableTCPCheck     bool     `json:"enable_tcp_check"`      // 启用TCP连接检查
	TCPPorts           []int    `json:"tcp_ports"`             // TCP检查端口
	EnableCustomChecks bool     `json:"enable_custom_checks"`  // 启用自定义检查
	SMBServiceCheck    bool     `json:"smb_service_check"`     // SMB服务检查
	DiskSpaceCheck     bool     `json:"disk_space_check"`      // 磁盘空间检查
	MemoryCheck        bool     `json:"memory_check"`          // 内存检查
	CPULoadCheck       bool     `json:"cpu_load_check"`        // CPU负载检查
}

// DefaultHealthCheckConfig 返回默认健康检查配置
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		Enabled:            true,
		CheckIntervalMs:    5000,
		TimeoutMs:          3000,
		HealthyThreshold:   2,
		UnhealthyThreshold: 3,
		RetryIntervalMs:    1000,
		EnableHTTPCheck:    true,
		HTTPEndpoint:       "/health",
		HTTPPort:           8080,
		EnableTCPCheck:     true,
		TCPPorts:           []int{445, 139},
		EnableCustomChecks: true,
		SMBServiceCheck:    true,
		DiskSpaceCheck:     true,
		MemoryCheck:        true,
		CPULoadCheck:       true,
	}
}

// NodeHealthStatus 节点健康状态
type NodeHealthStatus struct {
	mu              sync.RWMutex
	NodeID          string        `json:"node_id"`
	Hostname        string        `json:"hostname"`
	Address         string        `json:"address"`
	Port            int           `json:"port"`
	Healthy         bool          `json:"healthy"`
	ConsecutiveOK   int           `json:"consecutive_ok"`
	ConsecutiveFail int           `json:"consecutive_fail"`
	LastCheck       time.Time     `json:"last_check"`
	LastOK          time.Time     `json:"last_ok"`
	LastFailure     time.Time     `json:"last_failure"`
	FailureReason   string        `json:"failure_reason,omitempty"`
	Latency         time.Duration `json:"latency"`
	TotalChecks     int64         `json:"total_checks"`
	TotalFailures   int64         `json:"total_failures"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	mu        sync.RWMutex
	NodeID    string        `json:"node_id"`
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency"`
	Message   string        `json:"message,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Checks    []CheckDetail `json:"checks"`
}

// CheckDetail 单项检查详情
type CheckDetail struct {
	Name    string        `json:"name"`
	Healthy bool          `json:"healthy"`
	Latency time.Duration `json:"latency"`
	Message string        `json:"message,omitempty"`
	Error   error         `json:"error,omitempty"`
}

// HealthCheckFunc 自定义健康检查函数
type HealthCheckFunc func(ctx context.Context, node *NodeHealthStatus) CheckDetail

// HealthCheckResponse HTTP健康检查响应
type HealthCheckResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    time.Duration          `json:"uptime"`
	Version   string                 `json:"version"`
	Services  map[string]string      `json:"services"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// ClusterHealth 集群健康状态
type ClusterHealth struct {
	TotalNodes     int  `json:"total_nodes"`
	HealthyNodes   int  `json:"healthy_nodes"`
	UnhealthyNodes int  `json:"unhealthy_nodes"`
	ClusterHealthy bool `json:"cluster_healthy"`
}

// HealthStats 健康检查统计
type HealthStats struct {
	TotalChecks      int64         `json:"total_checks"`
	TotalFailures    int64         `json:"total_failures"`
	AverageLatency   time.Duration `json:"average_latency"`
	RegisteredChecks int           `json:"registered_checks"`
}

// HealthChecker 健康检查器
type HealthChecker struct {
	mu             sync.RWMutex
	config         *HealthCheckConfig
	nodes          map[string]*NodeHealthStatus
	localNodeID    string
	checks         map[string]HealthCheckFunc
	results        map[string]*HealthCheckResult
	running        bool
	stopChan       chan struct{}
	onHealthChange func(nodeID string, healthy bool)
	startTime      time.Time
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(config *HealthCheckConfig) *HealthChecker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	return &HealthChecker{
		config:   config,
		nodes:    make(map[string]*NodeHealthStatus),
		checks:   make(map[string]HealthCheckFunc),
		results:  make(map[string]*HealthCheckResult),
		stopChan: make(chan struct{}),
	}
}

// SetLocalNode 设置本地节点ID
func (hc *HealthChecker) SetLocalNode(nodeID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.localNodeID = nodeID
}

// AddNode 添加节点进行健康检查
func (hc *HealthChecker) AddNode(nodeID, hostname, address string, port int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.nodes[nodeID] = &NodeHealthStatus{
		NodeID:    nodeID,
		Hostname:  hostname,
		Address:   address,
		Port:      port,
		Healthy:   true,
		LastCheck: time.Now(),
		LastOK:    time.Now(),
	}

	logInfo("健康检查节点已添加", "node_id", nodeID, "address", fmt.Sprintf("%s:%d", address, port))
}

// RemoveNode 移除节点
func (hc *HealthChecker) RemoveNode(nodeID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.nodes, nodeID)
	delete(hc.results, nodeID)
	logInfo("健康检查节点已移除", "node_id", nodeID)
}

// RegisterCheck 注册自定义健康检查
func (hc *HealthChecker) RegisterCheck(name string, checkFunc HealthCheckFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = checkFunc
	logInfo("自定义健康检查已注册", "name", name)
}

// SetHealthChangeCallback 设置健康状态变化回调
func (hc *HealthChecker) SetHealthChangeCallback(fn func(nodeID string, healthy bool)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onHealthChange = fn
}

// Start 启动健康检查器
func (hc *HealthChecker) Start() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return fmt.Errorf("健康检查器已在运行")
	}

	hc.running = true
	hc.startTime = time.Now()
	go hc.checkLoop()

	logInfo("健康检查器已启动", "check_interval_ms", hc.config.CheckIntervalMs, "timeout_ms", hc.config.TimeoutMs)

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
	logInfo("健康检查器已停止")
}

// checkLoop 周期性健康检查循环
func (hc *HealthChecker) checkLoop() {
	interval := time.Duration(hc.config.CheckIntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopChan:
			return
		case <-ticker.C:
			hc.checkAllNodes()
		}
	}
}

// checkAllNodes 检查所有节点健康状态
func (hc *HealthChecker) checkAllNodes() {
	hc.mu.RLock()
	nodes := make([]*NodeHealthStatus, 0, len(hc.nodes))
	for _, node := range hc.nodes {
		if node.NodeID != hc.localNodeID {
			nodes = append(nodes, node)
		}
	}
	hc.mu.RUnlock()

	for _, node := range nodes {
		result := hc.checkNode(node)
		hc.processResult(node, result)
	}
}

// checkNode 对单个节点执行健康检查
func (hc *HealthChecker) checkNode(node *NodeHealthStatus) *HealthCheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(hc.config.TimeoutMs)*time.Millisecond)
	defer cancel()

	node.mu.RLock()
	nodeID := node.NodeID
	address := node.Address
	port := node.Port
	node.mu.RUnlock()

	result := &HealthCheckResult{
		NodeID:    nodeID,
		Healthy:   true,
		Timestamp: time.Now(),
		Checks:    make([]CheckDetail, 0),
	}

	startTime := time.Now()

	// TCP连接检查
	if hc.config.EnableTCPCheck {
		for _, tcpPort := range hc.config.TCPPorts {
			detail := hc.checkTCP(ctx, address, tcpPort)
			result.Checks = append(result.Checks, detail)
			if !detail.Healthy {
				result.Healthy = false
			}
		}
	}

	// HTTP健康端点检查
	if hc.config.EnableHTTPCheck {
		detail := hc.checkHTTP(ctx, address, port)
		result.Checks = append(result.Checks, detail)
		if !detail.Healthy {
			result.Healthy = false
		}
	}

	// SMB服务检查
	if hc.config.SMBServiceCheck {
		detail := hc.checkSMBService(ctx)
		result.Checks = append(result.Checks, detail)
		if !detail.Healthy {
			result.Healthy = false
		}
	}

	// 磁盘空间检查
	if hc.config.DiskSpaceCheck {
		detail := hc.checkDiskSpace(ctx)
		result.Checks = append(result.Checks, detail)
		if !detail.Healthy {
			result.Healthy = false
		}
	}

	// 内存检查
	if hc.config.MemoryCheck {
		detail := hc.checkMemory(ctx)
		result.Checks = append(result.Checks, detail)
		if !detail.Healthy {
			result.Healthy = false
		}
	}

	// 自定义检查
	hc.mu.RLock()
	checks := make(map[string]HealthCheckFunc, len(hc.checks))
	for name, checkFunc := range hc.checks {
		checks[name] = checkFunc
	}
	hc.mu.RUnlock()

	for name, checkFunc := range checks {
		detail := checkFunc(ctx, node)
		detail.Name = name
		result.Checks = append(result.Checks, detail)
		if !detail.Healthy {
			result.Healthy = false
		}
	}

	result.Latency = time.Since(startTime)

	if !result.Healthy {
		for _, detail := range result.Checks {
			if !detail.Healthy && detail.Error != nil {
				result.Message = detail.Error.Error()
				break
			}
		}
	}

	return result
}

// checkTCP 检查TCP连接
func (hc *HealthChecker) checkTCP(ctx context.Context, address string, port int) CheckDetail {
	startTime := time.Now()
	addr := fmt.Sprintf("%s:%d", address, port)

	detail := CheckDetail{
		Name: fmt.Sprintf("tcp:%d", port),
	}

	// 使用HTTP客户端进行TCP检查
	client := &http.Client{
		Timeout: time.Duration(hc.config.TimeoutMs) * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", fmt.Sprintf("http://%s", addr), nil)
	if err != nil {
		detail.Healthy = false
		detail.Error = err
		detail.Latency = time.Since(startTime)
		return detail
	}

	resp, err := client.Do(req)
	if err != nil {
		detail.Healthy = false
		detail.Error = err
		detail.Latency = time.Since(startTime)
		return detail
	}
	resp.Body.Close()

	detail.Healthy = true
	detail.Latency = time.Since(startTime)
	return detail
}

// checkHTTP 检查HTTP健康端点
func (hc *HealthChecker) checkHTTP(ctx context.Context, address string, port int) CheckDetail {
	startTime := time.Now()
	url := fmt.Sprintf("http://%s:%d%s", address, port, hc.config.HTTPEndpoint)

	detail := CheckDetail{
		Name: "http_health",
	}

	client := &http.Client{
		Timeout: time.Duration(hc.config.TimeoutMs) * time.Millisecond,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		detail.Healthy = false
		detail.Error = err
		detail.Latency = time.Since(startTime)
		return detail
	}

	resp, err := client.Do(req)
	if err != nil {
		detail.Healthy = false
		detail.Error = err
		detail.Latency = time.Since(startTime)
		return detail
	}
	defer resp.Body.Close()

	detail.Latency = time.Since(startTime)

	if resp.StatusCode == http.StatusOK {
		detail.Healthy = true
		detail.Message = "健康端点响应正常"
	} else {
		detail.Healthy = false
		detail.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return detail
}

// checkSMBService 检查SMB服务状态
func (hc *HealthChecker) checkSMBService(ctx context.Context) CheckDetail {
	startTime := time.Now()

	detail := CheckDetail{
		Name: "smb_service",
	}

	// 检查SMB端口是否可连接
	conn, err := net.DialTimeout("tcp", "localhost:445", time.Duration(hc.config.TimeoutMs)*time.Millisecond)
	if err != nil {
		detail.Healthy = false
		detail.Error = fmt.Errorf("SMB服务不可用: %w", err)
		detail.Latency = time.Since(startTime)
		return detail
	}
	conn.Close()

	detail.Healthy = true
	detail.Message = "SMB服务运行正常"
	detail.Latency = time.Since(startTime)
	return detail
}

// checkDiskSpace 检查磁盘空间
func (hc *HealthChecker) checkDiskSpace(ctx context.Context) CheckDetail {
	startTime := time.Now()

	detail := CheckDetail{
		Name: "disk_space",
	}

	// 使用已有的checkDiskSpace方法逻辑
	score := checkDiskSpaceScore()

	if score < 20 {
		detail.Healthy = false
		detail.Message = fmt.Sprintf("磁盘空间不足，健康分数: %d", score)
	} else {
		detail.Healthy = true
		detail.Message = fmt.Sprintf("磁盘空间充足，健康分数: %d", score)
	}

	detail.Latency = time.Since(startTime)
	return detail
}

// checkMemory 检查内存状态
func (hc *HealthChecker) checkMemory(ctx context.Context) CheckDetail {
	startTime := time.Now()

	detail := CheckDetail{
		Name: "memory",
	}

	// 使用已有的checkMemoryPressure方法逻辑
	score := checkMemoryPressureScore()

	if score < 20 {
		detail.Healthy = false
		detail.Message = fmt.Sprintf("内存压力过大，健康分数: %d", score)
	} else {
		detail.Healthy = true
		detail.Message = fmt.Sprintf("内存状态正常，健康分数: %d", score)
	}

	detail.Latency = time.Since(startTime)
	return detail
}

// processResult 处理健康检查结果
func (hc *HealthChecker) processResult(node *NodeHealthStatus, result *HealthCheckResult) {
	hc.mu.Lock()
	node.mu.Lock()

	previousHealthy := node.Healthy
	node.LastCheck = time.Now()
	node.TotalChecks++

	if result.Healthy {
		node.ConsecutiveOK++
		node.ConsecutiveFail = 0
		node.LastOK = time.Now()

		if !node.Healthy && node.ConsecutiveOK >= hc.config.HealthyThreshold {
			node.Healthy = true
			logInfo("节点恢复健康", "node_id", node.NodeID, "consecutive_ok", node.ConsecutiveOK)
		}
	} else {
		node.ConsecutiveFail++
		node.ConsecutiveOK = 0
		node.LastFailure = time.Now()
		node.FailureReason = result.Message
		node.TotalFailures++

		if node.Healthy && node.ConsecutiveFail >= hc.config.UnhealthyThreshold {
			node.Healthy = false
			logInfo("节点变为不健康", "node_id", node.NodeID, "reason", result.Message, "consecutive_fail", node.ConsecutiveFail)
		}
	}

	node.Latency = result.Latency
	hc.results[node.NodeID] = result

	node.mu.Unlock()
	hc.mu.Unlock()

	// 通知健康状态变化
	if previousHealthy != node.Healthy {
		hc.notifyHealthChange(node.NodeID, node.Healthy)
	}
}

// notifyHealthChange 通知健康状态变化
func (hc *HealthChecker) notifyHealthChange(nodeID string, healthy bool) {
	hc.mu.RLock()
	callback := hc.onHealthChange
	hc.mu.RUnlock()

	if callback != nil {
		go callback(nodeID, healthy)
	}
}

// IsNodeHealthy 检查节点是否健康
func (hc *HealthChecker) IsNodeHealthy(nodeID string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	node, ok := hc.nodes[nodeID]
	if !ok {
		return false
	}

	node.mu.RLock()
	defer node.mu.RUnlock()
	return node.Healthy
}

// GetNodeHealth 返回节点详细健康状态
func (hc *HealthChecker) GetNodeHealth(nodeID string) (*NodeHealthStatus, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	node, ok := hc.nodes[nodeID]
	if !ok {
		return nil, false
	}

	node.mu.RLock()
	copy := &NodeHealthStatus{
		NodeID:          node.NodeID,
		Hostname:        node.Hostname,
		Address:         node.Address,
		Port:            node.Port,
		Healthy:         node.Healthy,
		ConsecutiveOK:   node.ConsecutiveOK,
		ConsecutiveFail: node.ConsecutiveFail,
		LastCheck:       node.LastCheck,
		LastOK:          node.LastOK,
		LastFailure:     node.LastFailure,
		FailureReason:   node.FailureReason,
		Latency:         node.Latency,
		TotalChecks:     node.TotalChecks,
		TotalFailures:   node.TotalFailures,
	}
	node.mu.RUnlock()
	return copy, true
}

// GetNodeResult 返回节点最新健康检查结果
func (hc *HealthChecker) GetNodeResult(nodeID string) (*HealthCheckResult, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result, ok := hc.results[nodeID]
	if !ok {
		return nil, false
	}

	result.mu.RLock()
	checksCopy := make([]CheckDetail, len(result.Checks))
	copy(checksCopy, result.Checks)
	resultCopy := &HealthCheckResult{
		NodeID:    result.NodeID,
		Healthy:   result.Healthy,
		Latency:   result.Latency,
		Message:   result.Message,
		Timestamp: result.Timestamp,
		Checks:    checksCopy,
	}
	result.mu.RUnlock()
	return resultCopy, true
}

// GetAllNodeHealth 返回所有节点健康状态
func (hc *HealthChecker) GetAllNodeHealth() map[string]*NodeHealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]*NodeHealthStatus, len(hc.nodes))
	for id, node := range hc.nodes {
		node.mu.RLock()
		copy := &NodeHealthStatus{
			NodeID:          node.NodeID,
			Hostname:        node.Hostname,
			Address:         node.Address,
			Port:            node.Port,
			Healthy:         node.Healthy,
			ConsecutiveOK:   node.ConsecutiveOK,
			ConsecutiveFail: node.ConsecutiveFail,
			LastCheck:       node.LastCheck,
			LastOK:          node.LastOK,
			LastFailure:     node.LastFailure,
			FailureReason:   node.FailureReason,
			Latency:         node.Latency,
			TotalChecks:     node.TotalChecks,
			TotalFailures:   node.TotalFailures,
		}
		node.mu.RUnlock()
		result[id] = copy
	}
	return result
}

// GetHealthyNodes 返回健康节点ID列表
func (hc *HealthChecker) GetHealthyNodes() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var healthy []string
	for id, node := range hc.nodes {
		node.mu.RLock()
		if node.Healthy {
			healthy = append(healthy, id)
		}
		node.mu.RUnlock()
	}
	return healthy
}

// GetUnhealthyNodes 返回不健康节点ID列表
func (hc *HealthChecker) GetUnhealthyNodes() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var unhealthy []string
	for id, node := range hc.nodes {
		node.mu.RLock()
		if !node.Healthy {
			unhealthy = append(unhealthy, id)
		}
		node.mu.RUnlock()
	}
	return unhealthy
}

// GetClusterHealth 返回集群整体健康状态
func (hc *HealthChecker) GetClusterHealth() *ClusterHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	totalNodes := len(hc.nodes)
	healthyNodes := 0
	unhealthyNodes := 0

	for _, node := range hc.nodes {
		node.mu.RLock()
		if node.Healthy {
			healthyNodes++
		} else {
			unhealthyNodes++
		}
		node.mu.RUnlock()
	}

	return &ClusterHealth{
		TotalNodes:     totalNodes,
		HealthyNodes:   healthyNodes,
		UnhealthyNodes: unhealthyNodes,
		ClusterHealthy: unhealthyNodes == 0,
	}
}

// GetHealthStats 返回健康检查统计信息
func (hc *HealthChecker) GetHealthStats() *HealthStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	totalChecks := int64(0)
	totalFailures := int64(0)
	avgLatency := time.Duration(0)
	nodeCount := 0

	for _, node := range hc.nodes {
		node.mu.RLock()
		totalChecks += node.TotalChecks
		totalFailures += node.TotalFailures
		avgLatency += node.Latency
		nodeCount++
		node.mu.RUnlock()
	}

	if nodeCount > 0 {
		avgLatency = avgLatency / time.Duration(nodeCount)
	}

	return &HealthStats{
		TotalChecks:      totalChecks,
		TotalFailures:    totalFailures,
		AverageLatency:   avgLatency,
		RegisteredChecks: len(hc.checks),
	}
}

// PerformImmediateCheck 立即执行健康检查
func (hc *HealthChecker) PerformImmediateCheck(ctx context.Context, nodeID string) (*HealthCheckResult, error) {
	hc.mu.RLock()
	node, ok := hc.nodes[nodeID]
	hc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("节点 %s 未注册", nodeID)
	}

	result := hc.checkNode(node)
	hc.processResult(node, result)

	return result, nil
}

// checkDiskSpaceScore 检查磁盘空间健康分数 (0-100)
func checkDiskSpaceScore() int {
	cmd := exec.CommandContext(context.Background(), "df", "-B1", "/")
	out, err := cmd.Output()
	if err != nil {
		return 100
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 100
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return 100
	}
	usage := strings.TrimSuffix(fields[4], "%")
	var percent int
	if _, err := fmt.Sscanf(usage, "%d", &percent); err != nil {
		return 100
	}
	if percent >= 95 {
		return 0
	}
	if percent >= 90 {
		return 20
	}
	if percent >= 80 {
		return 60
	}
	if percent >= 70 {
		return 80
	}
	return 100
}

// checkMemoryPressureScore 检查内存压力健康分数 (0-100)
func checkMemoryPressureScore() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 100
	}
	var memTotal, memAvailable int64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		var val int64
		fmt.Sscanf(fields[1], "%d", &val)
		switch fields[0] {
		case "MemTotal:":
			memTotal = val * 1024
		case "MemAvailable:":
			memAvailable = val * 1024
		}
	}
	if memTotal == 0 {
		return 100
	}
	availPct := float64(memAvailable) / float64(memTotal) * 100
	if availPct < 5 {
		return 0
	}
	if availPct < 10 {
		return 20
	}
	if availPct < 20 {
		return 60
	}
	if availPct < 30 {
		return 80
	}
	return 100
}

// HealthCheckHandler 返回健康检查HTTP处理器
func (hc *HealthChecker) HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := hc.GetClusterHealth()

		status := http.StatusOK
		if !health.ClusterHealthy {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"status":"%v","healthy_nodes":%d,"total_nodes":%d}`,
			health.ClusterHealthy,
			health.HealthyNodes,
			health.TotalNodes)
	}
}

// IsRunning 检查健康检查器是否在运行
func (hc *HealthChecker) IsRunning() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.running
}

// GetUptime 返回运行时间
func (hc *HealthChecker) GetUptime() time.Duration {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	if !hc.running {
		return 0
	}
	return time.Since(hc.startTime)
}
