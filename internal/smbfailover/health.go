package smbfailover

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthChecker performs health checks on cluster nodes
type HealthChecker struct {
	mu              sync.RWMutex
	config          HealthConfig
	logger          *zap.Logger
	nodes           map[string]*HealthNode
	localNodeID     string
	checks          map[string]HealthCheckFunc
	results         map[string]*HealthResult
	running         bool
	stopCh          chan struct{}
	onHealthChange  func(nodeID string, healthy bool)
}

// HealthConfig configures health checking
type HealthConfig struct {
	CheckInterval      time.Duration `json:"check_interval"`
	Timeout            time.Duration `json:"timeout"`
	HealthyThreshold   int           `json:"healthy_threshold"`
	UnhealthyThreshold int           `json:"unhealthy_threshold"`
	RetryInterval      time.Duration `json:"retry_interval"`
	EnableHTTPCheck    bool          `json:"enable_http_check"`
	HTTPEndpoint       string        `json:"http_endpoint"`
	HTTPPort           int           `json:"http_port"`
	EnableTCPCheck     bool          `json:"enable_tcp_check"`
	TCPPorts           []int         `json:"tcp_ports"`
	EnableCustomChecks bool          `json:"enable_custom_checks"`
}

// DefaultHealthConfig returns sensible defaults
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		CheckInterval:      5 * time.Second,
		Timeout:            3 * time.Second,
		HealthyThreshold:   2,
		UnhealthyThreshold: 3,
		RetryInterval:      1 * time.Second,
		EnableHTTPCheck:    true,
		HTTPEndpoint:       "/health",
		HTTPPort:           8080,
		EnableTCPCheck:     true,
		TCPPorts:           []int{445, 139},
		EnableCustomChecks: true,
	}
}

// HealthNode represents a node being health checked
type HealthNode struct {
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

// HealthResult represents the result of a health check
type HealthResult struct {
	mu        sync.RWMutex
	NodeID    string        `json:"node_id"`
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency"`
	Message   string        `json:"message,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Checks    []CheckDetail `json:"checks"`
}

// CheckDetail represents details of an individual check
type CheckDetail struct {
	Name      string        `json:"name"`
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency"`
	Message   string        `json:"message,omitempty"`
	Error     error         `json:"error,omitempty"`
}

// HealthCheckFunc is a custom health check function
type HealthCheckFunc func(ctx context.Context, node *HealthNode) CheckDetail

// HealthResponse represents the response from an HTTP health endpoint
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    time.Duration     `json:"uptime"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config HealthConfig, logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		config:   config,
		logger:   logger,
		nodes:    make(map[string]*HealthNode),
		checks:   make(map[string]HealthCheckFunc),
		results:  make(map[string]*HealthResult),
		stopCh:   make(chan struct{}),
	}
}

// SetLocalNode sets the local node ID
func (hc *HealthChecker) SetLocalNode(nodeID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.localNodeID = nodeID
}

// AddNode adds a node for health checking
func (hc *HealthChecker) AddNode(nodeID, hostname, address string, port int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.nodes[nodeID] = &HealthNode{
		NodeID:   nodeID,
		Hostname: hostname,
		Address:  address,
		Port:     port,
		Healthy:  true,
		LastCheck: time.Now(),
		LastOK:   time.Now(),
	}

	hc.logger.Info("health check node added",
		zap.String("node_id", nodeID),
		zap.String("address", fmt.Sprintf("%s:%d", address, port)))
}

// RemoveNode removes a node from health checking
func (hc *HealthChecker) RemoveNode(nodeID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.nodes, nodeID)
	delete(hc.results, nodeID)
	hc.logger.Info("health check node removed", zap.String("node_id", nodeID))
}

// RegisterCheck registers a custom health check
func (hc *HealthChecker) RegisterCheck(name string, checkFunc HealthCheckFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = checkFunc
	hc.logger.Info("custom health check registered", zap.String("name", name))
}

// SetHealthChangeCallback sets the callback for health status changes
func (hc *HealthChecker) SetHealthChangeCallback(fn func(nodeID string, healthy bool)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onHealthChange = fn
}

// Start starts the health checker
func (hc *HealthChecker) Start() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return fmt.Errorf("health checker already running")
	}

	hc.running = true
	go hc.checkLoop()

	hc.logger.Info("health checker started",
		zap.Duration("check_interval", hc.config.CheckInterval),
		zap.Duration("timeout", hc.config.Timeout))

	return nil
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return
	}

	close(hc.stopCh)
	hc.running = false
	hc.logger.Info("health checker stopped")
}

// checkLoop performs periodic health checks
func (hc *HealthChecker) checkLoop() {
	ticker := time.NewTicker(hc.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkAllNodes()
		}
	}
}

// checkAllNodes checks health of all nodes
func (hc *HealthChecker) checkAllNodes() {
	hc.mu.RLock()
	nodes := make([]*HealthNode, 0, len(hc.nodes))
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

// checkNode performs health checks on a single node
func (hc *HealthChecker) checkNode(node *HealthNode) *HealthResult {
	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	node.mu.RLock()
	nodeID := node.NodeID
	address := node.Address
	port := node.Port
	node.mu.RUnlock()

	result := &HealthResult{
		NodeID:    nodeID,
		Healthy:   true,
		Timestamp: time.Now(),
		Checks:    make([]CheckDetail, 0),
	}

	startTime := time.Now()

	// TCP connectivity check
	if hc.config.EnableTCPCheck {
		for _, tcpPort := range hc.config.TCPPorts {
			detail := hc.checkTCP(ctx, address, tcpPort)
			result.Checks = append(result.Checks, detail)
			if !detail.Healthy {
				result.Healthy = false
			}
		}
	}

	// HTTP health endpoint check
	if hc.config.EnableHTTPCheck {
		detail := hc.checkHTTP(ctx, address, port)
		result.Checks = append(result.Checks, detail)
		if !detail.Healthy {
			result.Healthy = false
		}
	}

	// Custom checks
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

// checkTCP checks TCP connectivity
func (hc *HealthChecker) checkTCP(ctx context.Context, address string, port int) CheckDetail {
	startTime := time.Now()
	addr := fmt.Sprintf("%s:%d", address, port)

	detail := CheckDetail{
		Name: fmt.Sprintf("tcp:%d", port),
	}

	// Simple TCP check using HTTP with short timeout
	client := &http.Client{
		Timeout: hc.config.Timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	// Try a simple HEAD request
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

// checkHTTP checks HTTP health endpoint
func (hc *HealthChecker) checkHTTP(ctx context.Context, address string, port int) CheckDetail {
	startTime := time.Now()
	url := fmt.Sprintf("http://%s:%d%s", address, port, hc.config.HTTPEndpoint)

	detail := CheckDetail{
		Name: "http_health",
	}

	client := &http.Client{
		Timeout: hc.config.Timeout,
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
		detail.Message = "health endpoint responding"
	} else {
		detail.Healthy = false
		detail.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return detail
}

// processResult processes health check results
func (hc *HealthChecker) processResult(node *HealthNode, result *HealthResult) {
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
			hc.logger.Info("node became healthy",
				zap.String("node_id", node.NodeID),
				zap.Int("consecutive_ok", node.ConsecutiveOK))
		}
	} else {
		node.ConsecutiveFail++
		node.ConsecutiveOK = 0
		node.LastFailure = time.Now()
		node.FailureReason = result.Message
		node.TotalFailures++

		if node.Healthy && node.ConsecutiveFail >= hc.config.UnhealthyThreshold {
			node.Healthy = false
			hc.logger.Warn("node became unhealthy",
				zap.String("node_id", node.NodeID),
				zap.String("reason", result.Message),
				zap.Int("consecutive_fail", node.ConsecutiveFail))
		}
	}

	node.Latency = result.Latency
	hc.results[node.NodeID] = result

	node.mu.Unlock()
	hc.mu.Unlock()

	// Notify health change
	if previousHealthy != node.Healthy {
		hc.notifyHealthChange(node.NodeID, node.Healthy)
	}
}

// notifyHealthChange notifies health status change
func (hc *HealthChecker) notifyHealthChange(nodeID string, healthy bool) {
	hc.mu.RLock()
	callback := hc.onHealthChange
	hc.mu.RUnlock()

	if callback != nil {
		go callback(nodeID, healthy)
	}
}

// IsNodeHealthy returns true if a node is healthy
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

// GetNodeHealth returns detailed health status of a node
func (hc *HealthChecker) GetNodeHealth(nodeID string) (*HealthNode, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	node, ok := hc.nodes[nodeID]
	if !ok {
		return nil, false
	}

	node.mu.RLock()
	copy := &HealthNode{
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

// GetNodeResult returns the latest health check result for a node
func (hc *HealthChecker) GetNodeResult(nodeID string) (*HealthResult, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result, ok := hc.results[nodeID]
	if !ok {
		return nil, false
	}

	result.mu.RLock()
	checksCopy := make([]CheckDetail, len(result.Checks))
	copy(checksCopy, result.Checks)
	resultCopy := &HealthResult{
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

// GetAllNodeHealth returns health status of all nodes
func (hc *HealthChecker) GetAllNodeHealth() map[string]*HealthNode {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]*HealthNode, len(hc.nodes))
	for id, node := range hc.nodes {
		node.mu.RLock()
		copy := &HealthNode{
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

// GetHealthyNodes returns IDs of healthy nodes
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

// GetUnhealthyNodes returns IDs of unhealthy nodes
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

// GetClusterHealth returns overall cluster health
func (hc *HealthChecker) GetClusterHealth() map[string]interface{} {
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

	return map[string]interface{}{
		"total_nodes":     totalNodes,
		"healthy_nodes":   healthyNodes,
		"unhealthy_nodes": unhealthyNodes,
		"cluster_healthy": unhealthyNodes == 0,
	}
}

// GetHealthStats returns health check statistics
func (hc *HealthChecker) GetHealthStats() map[string]interface{} {
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

	return map[string]interface{}{
		"total_checks":   totalChecks,
		"total_failures": totalFailures,
		"average_latency": avgLatency,
		"registered_checks": len(hc.checks),
	}
}

// PerformImmediateCheck performs an immediate health check on a node
func (hc *HealthChecker) PerformImmediateCheck(ctx context.Context, nodeID string) (*HealthResult, error) {
	hc.mu.RLock()
	node, ok := hc.nodes[nodeID]
	hc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("node %s not registered", nodeID)
	}

	result := hc.checkNode(node)
	hc.processResult(node, result)

	return result, nil
}

// HealthCheckHandler returns an HTTP handler for health check endpoint
func (hc *HealthChecker) HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := hc.GetClusterHealth()

		status := http.StatusOK
		if !health["cluster_healthy"].(bool) {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"status":"%s","healthy_nodes":%d,"total_nodes":%d}`,
			health["cluster_healthy"],
			health["healthy_nodes"],
			health["total_nodes"])
	}
}
