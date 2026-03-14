package cluster

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 负载均衡算法
const (
	LBAlgorithmRoundRobin = "round-robin"
	LBAlgorithmLeastConn  = "least-conn"
	LBAlgorithmWeighted   = "weighted"
	LBAlgorithmIPHash     = "ip-hash"
)

// Backend 后端节点
type Backend struct {
	NodeID      string    `json:"node_id"`
	Address     string    `json:"address"`
	Weight      int       `json:"weight"`
	Active      bool      `json:"active"`
	Connections int       `json:"connections"`
	Healthy     bool      `json:"healthy"`
	LastCheck   time.Time `json:"last_check"`
	Failures    int       `json:"failures"`
}

// LBConfig 负载均衡配置
type LBConfig struct {
	Algorithm      string `json:"algorithm"`
	HealthCheckURL string `json:"health_check_url"`
	HealthInterval int    `json:"health_interval"` // 秒
	HealthTimeout  int    `json:"health_timeout"`  // 秒
	MaxFailures    int    `json:"max_failures"`
	StickySession  bool   `json:"sticky_session"`
}

// LBStats 负载均衡统计
type LBStats struct {
	TotalRequests   int64                    `json:"total_requests"`
	ActiveRequests  int64                    `json:"active_requests"`
	FailedRequests  int64                    `json:"failed_requests"`
	AvgResponseTime time.Duration            `json:"avg_response_time"`
	RequestsPerSec  float64                  `json:"requests_per_sec"`
	BackendStats    map[string]*BackendStats `json:"backend_stats"`
}

// BackendStats 后端节点统计
type BackendStats struct {
	Requests    int64         `json:"requests"`
	Failed      int64         `json:"failed"`
	AvgLatency  time.Duration `json:"avg_latency"`
	LastRequest time.Time     `json:"last_request"`
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	config        LBConfig
	backends      map[string]*Backend
	backendsMutex sync.RWMutex
	// proxy       *httputil.ReverseProxy - 保留用于未来反向代理功能
	currentIndex int
	sessionMap   map[string]string // session -> backend
	sessionMutex sync.RWMutex
	stats        LBStats
	statsMutex   sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *zap.Logger
	cluster      *ClusterManager
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(config LBConfig, logger *zap.Logger, cluster *ClusterManager) (*LoadBalancer, error) {
	if config.Algorithm == "" {
		config.Algorithm = LBAlgorithmRoundRobin
	}
	if config.HealthCheckURL == "" {
		config.HealthCheckURL = "/api/v1/health"
	}
	if config.HealthInterval == 0 {
		config.HealthInterval = 10
	}
	if config.HealthTimeout == 0 {
		config.HealthTimeout = 5
	}
	if config.MaxFailures == 0 {
		config.MaxFailures = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	lb := &LoadBalancer{
		config:     config,
		backends:   make(map[string]*Backend),
		sessionMap: make(map[string]string),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		cluster:    cluster,
	}

	// 初始化统计
	lb.stats.BackendStats = make(map[string]*BackendStats)

	return lb, nil
}

// Initialize 初始化负载均衡器
func (lb *LoadBalancer) Initialize() error {
	lb.logger.Info("初始化负载均衡器", zap.String("algorithm", lb.config.Algorithm))

	// 从集群管理器同步节点
	if err := lb.syncBackendsFromCluster(); err != nil {
		lb.logger.Error("同步后端节点失败", zap.Error(err))
	}

	// 启动健康检查
	go lb.healthCheckWorker()

	lb.logger.Info("负载均衡器初始化完成")
	return nil
}

// syncBackendsFromCluster 从集群同步后端节点
func (lb *LoadBalancer) syncBackendsFromCluster() error {
	nodes := lb.cluster.GetOnlineNodes()

	lb.backendsMutex.Lock()
	defer lb.backendsMutex.Unlock()

	for _, node := range nodes {
		address := fmt.Sprintf("http://%s:%d", node.IP, node.Port)

		if _, exists := lb.backends[node.ID]; !exists {
			lb.backends[node.ID] = &Backend{
				NodeID:    node.ID,
				Address:   address,
				Weight:    1,
				Active:    true,
				Healthy:   true,
				LastCheck: time.Now(),
			}
			lb.stats.BackendStats[node.ID] = &BackendStats{}

			lb.logger.Info("添加后端节点", zap.String("node_id", node.ID), zap.String("address", address))
		}
	}

	return nil
}

// AddBackend 添加后端节点
func (lb *LoadBalancer) AddBackend(nodeID, address string, weight int) error {
	lb.backendsMutex.Lock()
	defer lb.backendsMutex.Unlock()

	lb.backends[nodeID] = &Backend{
		NodeID:    nodeID,
		Address:   address,
		Weight:    weight,
		Active:    true,
		Healthy:   true,
		LastCheck: time.Now(),
	}

	if _, exists := lb.stats.BackendStats[nodeID]; !exists {
		lb.stats.BackendStats[nodeID] = &BackendStats{}
	}

	lb.logger.Info("添加后端节点", zap.String("node_id", nodeID), zap.String("address", address))
	return nil
}

// RemoveBackend 移除后端节点
func (lb *LoadBalancer) RemoveBackend(nodeID string) error {
	lb.backendsMutex.Lock()
	defer lb.backendsMutex.Unlock()

	if _, exists := lb.backends[nodeID]; !exists {
		return fmt.Errorf("后端节点不存在：%s", nodeID)
	}

	delete(lb.backends, nodeID)
	delete(lb.stats.BackendStats, nodeID)

	lb.logger.Info("移除后端节点", zap.String("node_id", nodeID))
	return nil
}

// UpdateBackendWeight 更新后端节点权重
func (lb *LoadBalancer) UpdateBackendWeight(nodeID string, weight int) error {
	lb.backendsMutex.Lock()
	defer lb.backendsMutex.Unlock()

	backend, exists := lb.backends[nodeID]
	if !exists {
		return fmt.Errorf("后端节点不存在：%s", nodeID)
	}

	backend.Weight = weight
	lb.logger.Info("更新后端权重", zap.String("node_id", nodeID), zap.Int("weight", weight))
	return nil
}

// GetBackend 获取后端节点
func (lb *LoadBalancer) GetBackend(nodeID string) (*Backend, bool) {
	lb.backendsMutex.RLock()
	defer lb.backendsMutex.RUnlock()

	backend, exists := lb.backends[nodeID]
	return backend, exists
}

// GetBackends 获取所有后端节点
func (lb *LoadBalancer) GetBackends() []*Backend {
	lb.backendsMutex.RLock()
	defer lb.backendsMutex.RUnlock()

	backends := make([]*Backend, 0, len(lb.backends))
	for _, backend := range lb.backends {
		backends = append(backends, backend)
	}
	return backends
}

// GetHealthyBackends 获取健康的后端节点
func (lb *LoadBalancer) GetHealthyBackends() []*Backend {
	lb.backendsMutex.RLock()
	defer lb.backendsMutex.RUnlock()

	backends := make([]*Backend, 0)
	for _, backend := range lb.backends {
		if backend.Active && backend.Healthy {
			backends = append(backends, backend)
		}
	}
	return backends
}

// SelectBackend 选择后端节点
func (lb *LoadBalancer) SelectBackend(clientIP string) (*Backend, error) {
	lb.backendsMutex.RLock()
	defer lb.backendsMutex.RUnlock()

	// 获取健康后端
	healthyBackends := make([]*Backend, 0)
	for _, backend := range lb.backends {
		if backend.Active && backend.Healthy {
			healthyBackends = append(healthyBackends, backend)
		}
	}

	if len(healthyBackends) == 0 {
		return nil, fmt.Errorf("没有可用的后端节点")
	}

	// 根据算法选择后端
	var selected *Backend
	switch lb.config.Algorithm {
	case LBAlgorithmRoundRobin:
		selected = lb.selectRoundRobin(healthyBackends)
	case LBAlgorithmLeastConn:
		selected = lb.selectLeastConn(healthyBackends)
	case LBAlgorithmWeighted:
		selected = lb.selectWeighted(healthyBackends)
	case LBAlgorithmIPHash:
		selected = lb.selectIPHash(healthyBackends, clientIP)
	default:
		selected = healthyBackends[0]
	}

	if selected != nil {
		selected.Connections++
	}

	return selected, nil
}

// selectRoundRobin 轮询选择
func (lb *LoadBalancer) selectRoundRobin(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	selected := backends[lb.currentIndex%len(backends)]
	lb.currentIndex = (lb.currentIndex + 1) % len(backends)
	return selected
}

// selectLeastConn 最少连接选择
func (lb *LoadBalancer) selectLeastConn(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	selected := backends[0]
	for _, backend := range backends {
		if backend.Connections < selected.Connections {
			selected = backend
		}
	}
	return selected
}

// selectWeighted 加权选择
func (lb *LoadBalancer) selectWeighted(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	totalWeight := 0
	for _, backend := range backends {
		totalWeight += backend.Weight
	}

	if totalWeight == 0 {
		return backends[0]
	}

	// 加权随机
	r := rand.Intn(totalWeight)
	current := 0
	for _, backend := range backends {
		current += backend.Weight
		if r < current {
			return backend
		}
	}

	return backends[len(backends)-1]
}

// selectIPHash IP 哈希选择
func (lb *LoadBalancer) selectIPHash(backends []*Backend, clientIP string) *Backend {
	if len(backends) == 0 {
		return nil
	}

	// 检查会话保持
	if lb.config.StickySession {
		lb.sessionMutex.RLock()
		if backendID, exists := lb.sessionMap[clientIP]; exists {
			lb.sessionMutex.RUnlock()
			if backend, exists := lb.backends[backendID]; exists && backend.Healthy {
				return backend
			}
		} else {
			lb.sessionMutex.RUnlock()
		}
	}

	// 简单哈希
	hash := 0
	for _, c := range clientIP {
		hash = (hash*31 + int(c)) % len(backends)
	}

	selected := backends[hash]

	// 记录会话
	if lb.config.StickySession {
		lb.sessionMutex.Lock()
		lb.sessionMap[clientIP] = selected.NodeID
		lb.sessionMutex.Unlock()
	}

	return selected
}

// ReleaseBackend 释放后端连接
func (lb *LoadBalancer) ReleaseBackend(nodeID string) {
	lb.backendsMutex.Lock()
	defer lb.backendsMutex.Unlock()

	if backend, exists := lb.backends[nodeID]; exists {
		if backend.Connections > 0 {
			backend.Connections--
		}
	}
}

// healthCheckWorker 健康检查工作线程
func (lb *LoadBalancer) healthCheckWorker() {
	ticker := time.NewTicker(time.Duration(lb.config.HealthInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lb.ctx.Done():
			return
		case <-ticker.C:
			lb.checkHealth()
		}
	}
}

// checkHealth 检查所有后端健康状态
func (lb *LoadBalancer) checkHealth() {
	lb.backendsMutex.RLock()
	backends := make([]*Backend, 0, len(lb.backends))
	for _, backend := range lb.backends {
		backends = append(backends, backend)
	}
	lb.backendsMutex.RUnlock()

	for _, backend := range backends {
		go lb.checkBackendHealth(backend)
	}
}

// checkBackendHealth 检查单个后端健康状态
func (lb *LoadBalancer) checkBackendHealth(backend *Backend) {
	client := &http.Client{
		Timeout: time.Duration(lb.config.HealthTimeout) * time.Second,
	}

	healthURL := backend.Address + lb.config.HealthCheckURL
	resp, err := client.Get(healthURL)

	lb.backendsMutex.Lock()
	defer lb.backendsMutex.Unlock()

	if err != nil || resp.StatusCode != http.StatusOK {
		backend.Failures++
		if backend.Failures >= lb.config.MaxFailures {
			backend.Healthy = false
			lb.logger.Warn("后端节点不健康",
				zap.String("node_id", backend.NodeID),
				zap.Int("failures", backend.Failures))
		}
	} else {
		backend.Failures = 0
		backend.Healthy = true
	}

	backend.LastCheck = time.Now()
}

// RecordRequest 记录请求统计
func (lb *LoadBalancer) RecordRequest(nodeID string, success bool, duration time.Duration) {
	lb.statsMutex.Lock()
	defer lb.statsMutex.Unlock()

	lb.stats.TotalRequests++
	if !success {
		lb.stats.FailedRequests++
	}

	if backendStats, exists := lb.stats.BackendStats[nodeID]; exists {
		backendStats.Requests++
		if !success {
			backendStats.Failed++
		}
		backendStats.LastRequest = time.Now()

		// 计算平均延迟
		total := backendStats.AvgLatency * time.Duration(backendStats.Requests-1)
		backendStats.AvgLatency = (total + duration) / time.Duration(backendStats.Requests)
	}
}

// GetStats 获取统计信息
func (lb *LoadBalancer) GetStats() LBStats {
	lb.statsMutex.RLock()
	defer lb.statsMutex.RUnlock()

	return lb.stats
}

// ResetStats 重置统计
func (lb *LoadBalancer) ResetStats() {
	lb.statsMutex.Lock()
	defer lb.statsMutex.Unlock()

	lb.stats = LBStats{
		BackendStats: make(map[string]*BackendStats),
	}
}

// UpdateConfig 更新配置
func (lb *LoadBalancer) UpdateConfig(config LBConfig) error {
	lb.config = config
	lb.logger.Info("负载均衡配置已更新", zap.String("algorithm", config.Algorithm))
	return nil
}

// GetConfig 获取配置
func (lb *LoadBalancer) GetConfig() LBConfig {
	return lb.config
}

// Shutdown 关闭负载均衡器
func (lb *LoadBalancer) Shutdown() error {
	lb.cancel()
	lb.logger.Info("负载均衡器已关闭")
	return nil
}

// CreateProxy 创建反向代理
func (lb *LoadBalancer) CreateProxy() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 选择后端
		clientIP := r.RemoteAddr
		backend, err := lb.SelectBackend(clientIP)
		if err != nil {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		// 解析后端地址
		target, err := url.Parse(backend.Address)
		if err != nil {
			lb.logger.Error("解析后端地址失败", zap.Error(err))
			http.Error(w, "Internal Error", http.StatusInternalServerError)
			return
		}

		// 创建代理
		proxy := httputil.NewSingleHostReverseProxy(target)

		// 记录请求
		start := time.Now()
		proxy.Rewrite = func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
		}

		// 包装响应写入器以记录状态
		lb.proxyRequest(w, r, proxy, backend.NodeID, start)
	})
}

func (lb *LoadBalancer) proxyRequest(w http.ResponseWriter, r *http.Request, proxy *httputil.ReverseProxy, nodeID string, start time.Time) {
	// 简化实现，实际应该包装 ResponseWriter 来捕获状态码
	proxy.ServeHTTP(w, r)

	duration := time.Since(start)
	lb.RecordRequest(nodeID, true, duration)
	lb.ReleaseBackend(nodeID)
}
