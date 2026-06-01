// Package multiclusterfed 提供多集群联邦管理功能
// 将多个 NAS 集群统一管理，实现跨集群的文件访问、负载均衡、故障转移和数据同步
package multiclusterfed

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ==================== 错误定义 ====================
var (
	ErrClusterNotFound    = errors.New("cluster not found")
	ErrClusterAlreadyExists = errors.New("cluster already exists")
	ErrClusterOffline     = errors.New("cluster offline")
	ErrClusterUnhealthy   = errors.New("cluster unhealthy")
	ErrNoHealthyCluster   = errors.New("no healthy cluster available")
	ErrNamespaceConflict  = errors.New("namespace conflict")
	ErrSyncInProgress     = errors.New("sync in progress")
	ErrDiscoveryFailed    = errors.New("discovery failed")
	ErrManagerNotRunning  = errors.New("manager not running")
)

// ==================== 类型定义 ====================

// ClusterState 集群状态枚举
type ClusterState string

const (
	ClusterStateOnline  ClusterState = "online"
	ClusterStateOffline ClusterState = "offline"
	ClusterStateJoining ClusterState = "joining"
	ClusterStateLeaving ClusterState = "leaving"
	ClusterStateSyncing ClusterState = "syncing"
	ClusterStateDegraded ClusterState = "degraded"
)

// HealthStatus 健康状态枚举
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// LoadBalanceStrategy 负载均衡策略枚举
type LoadBalanceStrategy string

const (
	LoadBalanceRoundRobin LoadBalanceStrategy = "round_robin"
	LoadBalanceLeastConn  LoadBalanceStrategy = "least_conn"
	LoadBalanceWeighted   LoadBalanceStrategy = "weighted"
	LoadBalanceLocality   LoadBalanceStrategy = "locality"
	LoadBalanceCapacity   LoadBalanceStrategy = "capacity"
)

// SyncMode 数据同步模式枚举
type SyncMode string

const (
	SyncModeAsync   SyncMode = "async"
	SyncModeSync    SyncMode = "sync"
	SyncModeSnapshot SyncMode = "snapshot"
)

// ==================== 数据结构定义 ====================

// ClusterNode 集群中的单个节点
type ClusterNode struct {
	ID            string       `json:"id"`
	Hostname      string       `json:"hostname"`
	Address       string       `json:"address"`
	State         ClusterState `json:"state"`
	Weight        int          `json:"weight"`
	Connections   int          `json:"connections"`
	Capacity      int64        `json:"capacity"`
	Used          int64        `json:"used"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	Tags          []string     `json:"tags"`
}

// Cluster 集群信息
type Cluster struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Endpoint         string            `json:"endpoint"`
	State            ClusterState      `json:"state"`
	HealthStatus     HealthStatus      `json:"health_status"`
	Nodes            map[string]*ClusterNode `json:"nodes"`
	Namespaces       []string          `json:"namespaces"`
	Weight           int               `json:"weight"`
	Region           string            `json:"region"`
	Tags             []string          `json:"tags"`
	LastHealthCheck  time.Time         `json:"last_health_check"`
	LastSyncTime     time.Time         `json:"last_sync_time"`
	FailoverPriority int               `json:"failover_priority"`
	Metadata         map[string]string `json:"metadata"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// Namespace 统一命名空间定义
type Namespace struct {
	Path            string   `json:"path"`
	PrimaryCluster string   `json:"primary_cluster"`
	ReplicaClusters []string `json:"replica_clusters"`
	SyncMode        SyncMode `json:"sync_mode"`
	ReadOnly        bool     `json:"read_only"`
	CreatedAt       time.Time `json:"created_at"`
}

// ClusterStatus 集群状态报告
type ClusterStatus struct {
	ClusterID         string        `json:"cluster_id"`
	ClusterName       string        `json:"cluster_name"`
	State             ClusterState  `json:"state"`
	HealthStatus      HealthStatus  `json:"health_status"`
	NodeCount         int           `json:"node_count"`
	OnlineNodes       int           `json:"online_nodes"`
	TotalCapacity     int64         `json:"total_capacity"`
	UsedCapacity      int64         `json:"used_capacity"`
	ActiveConnections int           `json:"active_connections"`
	PendingSyncs      int           `json:"pending_syncs"`
	LastHealthCheck   time.Time     `json:"last_health_check"`
	Uptime            time.Duration `json:"uptime"`
}

// FederationStats 联邦统计信息
type FederationStats struct {
	TotalClusters     int   `json:"total_clusters"`
	HealthyClusters   int   `json:"healthy_clusters"`
	TotalNodes        int   `json:"total_nodes"`
	TotalNamespaces   int   `json:"total_namespaces"`
	TotalCapacity     int64 `json:"total_capacity"`
	TotalUsedCapacity int64 `json:"total_used_capacity"`
	ActiveSyncs       int   `json:"active_syncs"`
	FailoverEvents    int64 `json:"failover_events"`
}

// DiscoveryConfig 集群发现配置
type DiscoveryConfig struct {
	Enabled           bool          `json:"enabled"`
	DiscoveryInterval time.Duration `json:"discovery_interval"`
	MulticastAddress  string        `json:"multicast_address"`
	MulticastPort     int           `json:"multicast_port"`
	DiscoveryTimeout  time.Duration `json:"discovery_timeout"`
	AutoJoin          bool          `json:"auto_join"`
	SubnetFilter      string        `json:"subnet_filter"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Interval          time.Duration `json:"interval"`
	Timeout           time.Duration `json:"timeout"`
	FailureThreshold  int           `json:"failure_threshold"`
	RecoveryThreshold int           `json:"recovery_threshold"`
}

// FailoverConfig 故障转移配置
type FailoverConfig struct {
	Enabled             bool          `json:"enabled"`
	Strategy            string        `json:"strategy"`
	AutoFailback        bool          `json:"auto_failback"`
	FailbackDelay       time.Duration `json:"failback_delay"`
	MaxFailoverAttempts int           `json:"max_failover_attempts"`
}

// SyncConfig 数据同步配置
type SyncConfig struct {
	Mode           SyncMode      `json:"mode"`
	Workers        int           `json:"workers"`
	ChunkSize      int64         `json:"chunk_size"`
	BandwidthLimit int64         `json:"bandwidth_limit"`
	RetryAttempts  int           `json:"retry_attempts"`
	RetryDelay     time.Duration `json:"retry_delay"`
}

// FederationConfig 联邦配置
type FederationConfig struct {
	ClusterID          string              `json:"cluster_id"`
	ClusterName        string              `json:"cluster_name"`
	ListenPort         int                 `json:"listen_port"`
	LoadBalanceStrategy LoadBalanceStrategy `json:"load_balance_strategy"`
	Discovery          DiscoveryConfig     `json:"discovery"`
	HealthCheck        HealthCheckConfig   `json:"health_check"`
	Failover           FailoverConfig      `json:"failover"`
	Sync               SyncConfig          `json:"sync"`
}

// syncTask 内部同步任务
type syncTask struct {
	ID            string
	SourceCluster string
	TargetCluster string
	Namespace     string
	Status        string
	Progress      float64
	StartedAt     time.Time
	CompletedAt   time.Time
	Error         error
}

// failoverEvent 内部故障转移事件记录
type failoverEvent struct {
	Timestamp     time.Time
	SourceCluster string
	TargetCluster string
	Namespace     string
	Reason        string
	Success       bool
}

// ==================== 主管理器结构体 ====================

// ClusterFederationManager 多集群联邦管理器
// 提供集群发现、统一命名空间、负载均衡、故障转移、数据同步和集中监控
type ClusterFederationManager struct {
	mu               sync.RWMutex
	config           FederationConfig
	clusters         map[string]*Cluster
	namespaces       map[string]*Namespace
	syncTasks        []*syncTask
	failoverEvents   []*failoverEvent
	roundRobinIndex  int
	stats            FederationStats
	running          bool
	ctx              context.Context
	cancel           context.CancelFunc
	startedAt        time.Time
}

// ==================== 初始化 ====================

// init 包初始化，注册 multiclusterfed 模块
func init() {
	fmt.Println("[multiclusterfed] 多集群联邦管理模块已加载")
}

// New 创建新的集群联邦管理器实例
func New(config FederationConfig) *ClusterFederationManager {
	ctx, cancel := context.WithCancel(context.Background())

	if config.LoadBalanceStrategy == "" {
		config.LoadBalanceStrategy = LoadBalanceRoundRobin
	}
	if config.Discovery.DiscoveryInterval == 0 {
		config.Discovery.DiscoveryInterval = 30 * time.Second
	}
	if config.Discovery.DiscoveryTimeout == 0 {
		config.Discovery.DiscoveryTimeout = 5 * time.Second
	}
	if config.Discovery.MulticastPort == 0 {
		config.Discovery.MulticastPort = 9999
	}
	if config.HealthCheck.Interval == 0 {
		config.HealthCheck.Interval = 10 * time.Second
	}
	if config.HealthCheck.Timeout == 0 {
		config.HealthCheck.Timeout = 5 * time.Second
	}
	if config.HealthCheck.FailureThreshold == 0 {
		config.HealthCheck.FailureThreshold = 3
	}
	if config.HealthCheck.RecoveryThreshold == 0 {
		config.HealthCheck.RecoveryThreshold = 2
	}
	if config.Failover.MaxFailoverAttempts == 0 {
		config.Failover.MaxFailoverAttempts = 3
	}
	if config.Failover.FailbackDelay == 0 {
		config.Failover.FailbackDelay = 5 * time.Minute
	}
	if config.Sync.Workers == 0 {
		config.Sync.Workers = 4
	}
	if config.Sync.ChunkSize == 0 {
		config.Sync.ChunkSize = 4 * 1024 * 1024
	}
	if config.Sync.RetryAttempts == 0 {
		config.Sync.RetryAttempts = 3
	}
	if config.Sync.RetryDelay == 0 {
		config.Sync.RetryDelay = 5 * time.Second
	}

	return &ClusterFederationManager{
		config:         config,
		clusters:       make(map[string]*Cluster),
		namespaces:     make(map[string]*Namespace),
		syncTasks:      make([]*syncTask, 0),
		failoverEvents: make([]*failoverEvent, 0),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// ==================== 生命周期管理 ====================

// Start 启动联邦管理器
func (m *ClusterFederationManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	m.running = true
	m.startedAt = time.Now()
	m.ctx, m.cancel = context.WithCancel(context.Background())

	fmt.Printf("[multiclusterfed] 联邦管理器启动，集群: %s (%s)\n",
		m.config.ClusterID, m.config.ClusterName)

	if m.config.Discovery.Enabled {
		go m.discoveryLoop()
	}
	go m.healthCheckLoop()
	go m.loadBalanceUpdateLoop()
	for i := 0; i < m.config.Sync.Workers; i++ {
		go m.syncWorker(i)
	}
	if m.config.Failover.Enabled {
		go m.failoverMonitorLoop()
	}

	return nil
}

// Stop 停止联邦管理器
func (m *ClusterFederationManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	fmt.Println("[multiclusterfed] 联邦管理器正在停止...")
	m.cancel()
	m.running = false
	fmt.Printf("[multiclusterfed] 联邦管理器已停止，运行时长: %s\n", time.Since(m.startedAt))
	return nil
}

// IsRunning 返回管理器是否正在运行
func (m *ClusterFederationManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ==================== 集群管理 ====================

// AddCluster 添加集群到联邦
func (m *ClusterFederationManager) AddCluster(cluster *Cluster) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return ErrManagerNotRunning
	}
	if _, exists := m.clusters[cluster.ID]; exists {
		return ErrClusterAlreadyExists
	}

	now := time.Now()
	cluster.State = ClusterStateJoining
	cluster.HealthStatus = HealthStatusUnknown
	cluster.LastHealthCheck = now
	cluster.CreatedAt = now
	cluster.UpdatedAt = now
	if cluster.Nodes == nil {
		cluster.Nodes = make(map[string]*ClusterNode)
	}
	if cluster.Metadata == nil {
		cluster.Metadata = make(map[string]string)
	}

	m.clusters[cluster.ID] = cluster
	fmt.Printf("[multiclusterfed] 集群 %s (%s) 正在加入，端点: %s\n",
		cluster.ID, cluster.Name, cluster.Endpoint)

	go m.verifyClusterConnectivity(cluster.ID)
	return nil
}

// RemoveCluster 从联邦中移除集群
func (m *ClusterFederationManager) RemoveCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.clusters[clusterID]
	if !exists {
		return ErrClusterNotFound
	}

	for path, ns := range m.namespaces {
		if ns.PrimaryCluster == clusterID {
			return fmt.Errorf("集群 %s 是命名空间 %s 的主集群，请先迁移", clusterID, path)
		}
	}

	cluster.State = ClusterStateLeaving
	cluster.UpdatedAt = time.Now()

	for _, ns := range m.namespaces {
		filtered := make([]string, 0, len(ns.ReplicaClusters))
		for _, id := range ns.ReplicaClusters {
			if id != clusterID {
				filtered = append(filtered, id)
			}
		}
		ns.ReplicaClusters = filtered
	}

	delete(m.clusters, clusterID)
	fmt.Printf("[multiclusterfed] 集群 %s 已移除\n", clusterID)
	return nil
}

// GetClusterStatus 获取集群状态
func (m *ClusterFederationManager) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getClusterStatusUnsafe(clusterID)
}

// GetAllClusterStatus 获取所有集群状态
func (m *ClusterFederationManager) GetAllClusterStatus() []*ClusterStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]*ClusterStatus, 0, len(m.clusters))
	for id := range m.clusters {
		if s, _ := m.getClusterStatusUnsafe(id); s != nil {
			statuses = append(statuses, s)
		}
	}
	return statuses
}

func (m *ClusterFederationManager) getClusterStatusUnsafe(clusterID string) (*ClusterStatus, error) {
	cluster, exists := m.clusters[clusterID]
	if !exists {
		return nil, ErrClusterNotFound
	}

	onlineNodes := 0
	var totalCap, usedCap int64
	var activeConns int
	for _, n := range cluster.Nodes {
		if n.State == ClusterStateOnline {
			onlineNodes++
		}
		totalCap += n.Capacity
		usedCap += n.Used
		activeConns += n.Connections
	}

	pendingSyncs := 0
	for _, t := range m.syncTasks {
		if (t.SourceCluster == clusterID || t.TargetCluster == clusterID) &&
			t.Status != "completed" && t.Status != "failed" {
			pendingSyncs++
		}
	}

	uptime := time.Duration(0)
	if m.running {
		uptime = time.Since(m.startedAt)
	}

	return &ClusterStatus{
		ClusterID:         cluster.ID,
		ClusterName:       cluster.Name,
		State:             cluster.State,
		HealthStatus:      cluster.HealthStatus,
		NodeCount:         len(cluster.Nodes),
		OnlineNodes:       onlineNodes,
		TotalCapacity:     totalCap,
		UsedCapacity:      usedCap,
		ActiveConnections: activeConns,
		PendingSyncs:      pendingSyncs,
		LastHealthCheck:   cluster.LastHealthCheck,
		Uptime:            uptime,
	}, nil
}

// ==================== 统一命名空间 ====================

// CreateNamespace 创建统一命名空间
func (m *ClusterFederationManager) CreateNamespace(ns *Namespace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.namespaces[ns.Path]; exists {
		return ErrNamespaceConflict
	}
	if _, exists := m.clusters[ns.PrimaryCluster]; !exists {
		return fmt.Errorf("主集群 %s 不存在: %w", ns.PrimaryCluster, ErrClusterNotFound)
	}
	for _, rid := range ns.ReplicaClusters {
		if _, exists := m.clusters[rid]; !exists {
			return fmt.Errorf("副本集群 %s 不存在: %w", rid, ErrClusterNotFound)
		}
	}

	ns.CreatedAt = time.Now()
	m.namespaces[ns.Path] = ns
	fmt.Printf("[multiclusterfed] 命名空间 %s 创建完成，主: %s，副本: %v\n",
		ns.Path, ns.PrimaryCluster, ns.ReplicaClusters)

	if ns.SyncMode != "" {
		for _, rid := range ns.ReplicaClusters {
			m.createSyncTask(ns.PrimaryCluster, rid, ns.Path)
		}
	}
	return nil
}

// DeleteNamespace 删除命名空间
func (m *ClusterFederationManager) DeleteNamespace(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.namespaces[path]; !exists {
		return fmt.Errorf("命名空间 %s 不存在: %w", path, ErrClusterNotFound)
	}
	delete(m.namespaces, path)
	fmt.Printf("[multiclusterfed] 命名空间 %s 已删除\n", path)
	return nil
}

// GetNamespace 获取命名空间
func (m *ClusterFederationManager) GetNamespace(path string) (*Namespace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ns, exists := m.namespaces[path]
	if !exists {
		return nil, fmt.Errorf("命名空间 %s 不存在: %w", path, ErrClusterNotFound)
	}
	return ns, nil
}

// ListNamespaces 列出所有命名空间
func (m *ClusterFederationManager) ListNamespaces() []*Namespace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nss := make([]*Namespace, 0, len(m.namespaces))
	for _, ns := range m.namespaces {
		nss = append(nss, ns)
	}
	return nss
}

// ==================== 负载均衡 ====================

// SelectCluster 根据负载均衡策略选择目标集群
func (m *ClusterFederationManager) SelectCluster(namespace string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ns, exists := m.namespaces[namespace]
	if !exists {
		return "", fmt.Errorf("命名空间 %s 不存在: %w", namespace, ErrClusterNotFound)
	}

	candidates := []string{ns.PrimaryCluster}
	candidates = append(candidates, ns.ReplicaClusters...)

	healthy := make([]string, 0)
	for _, cid := range candidates {
		if c, ok := m.clusters[cid]; ok {
			if c.HealthStatus == HealthStatusHealthy && c.State == ClusterStateOnline {
				healthy = append(healthy, cid)
			}
		}
	}

	if len(healthy) == 0 {
		return "", ErrNoHealthyCluster
	}

	switch m.config.LoadBalanceStrategy {
	case LoadBalanceRoundRobin:
		return m.selectRoundRobin(healthy), nil
	case LoadBalanceLeastConn:
		return m.selectLeastConn(healthy), nil
	case LoadBalanceWeighted:
		return m.selectWeighted(healthy), nil
	case LoadBalanceCapacity:
		return m.selectByCapacity(healthy), nil
	case LoadBalanceLocality:
		return m.selectLocality(healthy), nil
	default:
		return m.selectRoundRobin(healthy), nil
	}
}

func (m *ClusterFederationManager) selectRoundRobin(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	m.roundRobinIndex = (m.roundRobinIndex + 1) % len(candidates)
	return candidates[m.roundRobinIndex]
}

func (m *ClusterFederationManager) selectLeastConn(candidates []string) string {
	minConn := -1
	var selected string
	for _, cid := range candidates {
		c, ok := m.clusters[cid]
		if !ok {
			continue
		}
		total := 0
		for _, n := range c.Nodes {
			total += n.Connections
		}
		if minConn == -1 || total < minConn {
			minConn = total
			selected = cid
		}
	}
	return selected
}

func (m *ClusterFederationManager) selectWeighted(candidates []string) string {
	selected := candidates[0]
	maxW := 0
	for _, cid := range candidates {
		if c, ok := m.clusters[cid]; ok && c.Weight > maxW {
			maxW = c.Weight
			selected = cid
		}
	}
	return selected
}

func (m *ClusterFederationManager) selectByCapacity(candidates []string) string {
	var maxFree int64 = -1
	var selected string
	for _, cid := range candidates {
		c, ok := m.clusters[cid]
		if !ok {
			continue
		}
		free := int64(0)
		for _, n := range c.Nodes {
			free += n.Capacity - n.Used
		}
		if free > maxFree {
			maxFree = free
			selected = cid
		}
	}
	return selected
}

func (m *ClusterFederationManager) selectLocality(candidates []string) string {
	for _, cid := range candidates {
		if cid == m.config.ClusterID {
			return cid
		}
	}
	return m.selectRoundRobin(candidates)
}

// ==================== 集群发现 ====================

func (m *ClusterFederationManager) discoveryLoop() {
	ticker := time.NewTicker(m.config.Discovery.DiscoveryInterval)
	defer ticker.Stop()
	fmt.Println("[multiclusterfed] 集群发现服务已启动")

	for {
		select {
		case <-m.ctx.Done():
			fmt.Println("[multiclusterfed] 集群发现服务已停止")
			return
		case <-ticker.C:
			m.discoverClusters()
		}
	}
}

func (m *ClusterFederationManager) discoverClusters() {
	if !m.config.Discovery.Enabled {
		return
	}

	var subnet *net.IPNet
	if m.config.Discovery.SubnetFilter != "" {
		var err error
		_, subnet, err = net.ParseCIDR(m.config.Discovery.SubnetFilter)
		if err != nil {
			fmt.Printf("[multiclusterfed] 子网解析失败: %v\n", err)
			return
		}
	}

	addr := fmt.Sprintf("%s:%d", m.config.Discovery.MulticastAddress, m.config.Discovery.MulticastPort)
	conn, err := net.DialTimeout("udp", addr, m.config.Discovery.DiscoveryTimeout)
	if err != nil {
		if subnet != nil {
			m.scanSubnet(subnet)
		}
		return
	}
	defer conn.Close()

	msg := fmt.Sprintf("FEDERATION_DISCOVER|%s|%s|%d",
		m.config.ClusterID, m.config.ClusterName, m.config.ListenPort)
	if _, err := conn.Write([]byte(msg)); err != nil {
		return
	}

	buf := make([]byte, 4096)
	_ = conn.SetReadDeadline(time.Now().Add(m.config.Discovery.DiscoveryTimeout))
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	m.processDiscoveryResponse(string(buf[:n]))
}

func (m *ClusterFederationManager) scanSubnet(subnet *net.IPNet) {
	ports := []int{9999, 10000, 10001}
	for ip := cloneIP(subnet.IP); subnet.Contains(ip); incIP(ip) {
		for _, port := range ports {
			addr := fmt.Sprintf("%s:%d", ip.String(), port)
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err != nil {
				continue
			}
			conn.Close()
			fmt.Printf("[multiclusterfed] 发现潜在节点: %s\n", addr)
		}
	}
}

func cloneIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (m *ClusterFederationManager) processDiscoveryResponse(response string) {
	fmt.Printf("[multiclusterfed] 发现响应: %s\n", response)
}

// ==================== 健康检查 ====================

func (m *ClusterFederationManager) healthCheckLoop() {
	ticker := time.NewTicker(m.config.HealthCheck.Interval)
	defer ticker.Stop()
	fmt.Println("[multiclusterfed] 健康检查服务已启动")

	for {
		select {
		case <-m.ctx.Done():
			fmt.Println("[multiclusterfed] 健康检查服务已停止")
			return
		case <-ticker.C:
			m.performHealthChecks()
		}
	}
}

func (m *ClusterFederationManager) performHealthChecks() {
	m.mu.Lock()
	clusterIDs := make([]string, 0, len(m.clusters))
	for id := range m.clusters {
		clusterIDs = append(clusterIDs, id)
	}
	m.mu.Unlock()

	for _, cid := range clusterIDs {
		m.checkClusterHealth(cid)
	}
}

func (m *ClusterFederationManager) checkClusterHealth(clusterID string) {
	m.mu.Lock()
	cluster, exists := m.clusters[clusterID]
	if !exists {
		m.mu.Unlock()
		return
	}
	endpoint := cluster.Endpoint
	m.mu.Unlock()

	// 尝试 TCP 连接作为健康检查
	conn, err := net.DialTimeout("tcp", endpoint, m.config.HealthCheck.Timeout)

	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists = m.clusters[clusterID]
	if !exists {
		return
	}

	if err != nil {
		cluster.HealthStatus = HealthStatusUnhealthy
		cluster.State = ClusterStateOffline
		fmt.Printf("[multiclusterfed] 集群 %s 健康检查失败: %v\n", clusterID, err)
	} else {
		conn.Close()
		cluster.HealthStatus = HealthStatusHealthy
		if cluster.State == ClusterStateOffline || cluster.State == ClusterStateJoining {
			cluster.State = ClusterStateOnline
		}
	}
	cluster.LastHealthCheck = time.Now()
}

// verifyClusterConnectivity 验证集群连通性
func (m *ClusterFederationManager) verifyClusterConnectivity(clusterID string) {
	m.mu.RLock()
	cluster, exists := m.clusters[clusterID]
	if !exists {
		m.mu.RUnlock()
		return
	}
	endpoint := cluster.Endpoint
	m.mu.RUnlock()

	conn, err := net.DialTimeout("tcp", endpoint, 10*time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists = m.clusters[clusterID]
	if !exists {
		return
	}

	if err != nil {
		cluster.State = ClusterStateOffline
		cluster.HealthStatus = HealthStatusUnhealthy
	} else {
		conn.Close()
		cluster.State = ClusterStateOnline
		cluster.HealthStatus = HealthStatusHealthy
	}
	cluster.UpdatedAt = time.Now()
}

// ==================== 数据同步 ====================

func (m *ClusterFederationManager) createSyncTask(source, target, namespace string) {
	task := &syncTask{
		ID:            fmt.Sprintf("sync-%s-%s-%d", source, target, time.Now().UnixNano()),
		SourceCluster: source,
		TargetCluster: target,
		Namespace:     namespace,
		Status:        "pending",
		StartedAt:     time.Now(),
	}
	m.syncTasks = append(m.syncTasks, task)
}

func (m *ClusterFederationManager) syncWorker(workerID int) {
	fmt.Printf("[multiclusterfed] 同步 worker %d 已启动\n", workerID)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			fmt.Printf("[multiclusterfed] 同步 worker %d 已停止\n", workerID)
			return
		case <-ticker.C:
			m.processSyncTasks()
		}
	}
}

func (m *ClusterFederationManager) processSyncTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.syncTasks {
		if task.Status != "pending" {
			continue
		}

		// 检查源和目标集群是否健康
		src, srcOk := m.clusters[task.SourceCluster]
		tgt, tgtOk := m.clusters[task.TargetCluster]
		if !srcOk || !tgtOk {
			task.Status = "failed"
			task.Error = ErrClusterNotFound
			continue
		}
		if src.HealthStatus != HealthStatusHealthy || tgt.HealthStatus != HealthStatusHealthy {
			continue // 等待集群恢复
		}

		task.Status = "running"
		task.StartedAt = time.Now()

		// 模拟同步过程
		go m.executeSync(task)
	}
}

func (m *ClusterFederationManager) executeSync(task *syncTask) {
	fmt.Printf("[multiclusterfed] 开始同步: %s -> %s, 命名空间: %s\n",
		task.SourceCluster, task.TargetCluster, task.Namespace)

	// 模拟同步进度
	for i := 0; i <= 100; i += 10 {
		select {
		case <-m.ctx.Done():
			task.Status = "failed"
			task.Error = context.Canceled
			return
		case <-time.After(500 * time.Millisecond):
			m.mu.Lock()
			task.Progress = float64(i) / 100.0
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task.Status = "completed"
	task.Progress = 1.0
	task.CompletedAt = time.Now()
	fmt.Printf("[multiclusterfed] 同步完成: %s -> %s\n", task.SourceCluster, task.TargetCluster)
}

// ==================== 故障转移 ====================

func (m *ClusterFederationManager) failoverMonitorLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	fmt.Println("[multiclusterfed] 故障转移监控已启动")

	for {
		select {
		case <-m.ctx.Done():
			fmt.Println("[multiclusterfed] 故障转移监控已停止")
			return
		case <-ticker.C:
			m.checkFailoverNeeded()
		}
	}
}

func (m *ClusterFederationManager) checkFailoverNeeded() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ns := range m.namespaces {
		primary, exists := m.clusters[ns.PrimaryCluster]
		if !exists {
			continue
		}

		// 主集群不健康时触发故障转移
		if primary.HealthStatus == HealthStatusUnhealthy && len(ns.ReplicaClusters) > 0 {
			newPrimary := m.selectFailoverTarget(ns)
			if newPrimary == "" {
				fmt.Printf("[multiclusterfed] 命名空间 %s 故障转移失败：无可用副本\n", ns.Path)
				m.recordFailoverEvent(ns.PrimaryCluster, "", ns.Path, "无可用副本", false)
				continue
			}

			oldPrimary := ns.PrimaryCluster
			ns.PrimaryCluster = newPrimary
			// 从副本列表中移除新的主集群，加入旧的主集群作为副本
			newReplicas := make([]string, 0, len(ns.ReplicaClusters))
			for _, rid := range ns.ReplicaClusters {
				if rid != newPrimary {
					newReplicas = append(newReplicas, rid)
				}
			}
			newReplicas = append(newReplicas, oldPrimary)
			ns.ReplicaClusters = newReplicas

			fmt.Printf("[multiclusterfed] 命名空间 %s 故障转移: %s -> %s\n",
				ns.Path, oldPrimary, newPrimary)
			m.recordFailoverEvent(oldPrimary, newPrimary, ns.Path, "主集群不健康", true)
		}

		// 自动回切检查
		if m.config.Failover.AutoFailback && primary.HealthStatus == HealthStatusHealthy {
			// 检查是否需要回切到原始主集群
			for _, event := range m.failoverEvents {
				if event.TargetCluster == ns.PrimaryCluster && event.Success {
					if original, ok := m.clusters[event.SourceCluster]; ok {
						if original.HealthStatus == HealthStatusHealthy &&
							time.Since(event.Timestamp) > m.config.Failover.FailbackDelay {
							// 执行回切
							oldTarget := ns.PrimaryCluster
							ns.PrimaryCluster = event.SourceCluster
							ns.ReplicaClusters = append(ns.ReplicaClusters, oldTarget)
							fmt.Printf("[multiclusterfed] 命名空间 %s 自动回切: %s -> %s\n",
								ns.Path, oldTarget, event.SourceCluster)
						}
					}
					break
				}
			}
		}
	}
}

func (m *ClusterFederationManager) selectFailoverTarget(ns *Namespace) string {
	var bestCandidate string
	bestPriority := int(^uint(0) >> 1) // max int

	for _, rid := range ns.ReplicaClusters {
		cluster, exists := m.clusters[rid]
		if !exists || cluster.HealthStatus != HealthStatusHealthy {
			continue
		}
		if cluster.FailoverPriority < bestPriority {
			bestPriority = cluster.FailoverPriority
			bestCandidate = rid
		}
	}

	if bestCandidate != "" {
		return bestCandidate
	}

	// 回退策略：选择第一个健康的副本
	for _, rid := range ns.ReplicaClusters {
		if c, ok := m.clusters[rid]; ok && c.HealthStatus == HealthStatusHealthy {
			return rid
		}
	}
	return ""
}

func (m *ClusterFederationManager) recordFailoverEvent(source, target, namespace, reason string, success bool) {
	event := &failoverEvent{
		Timestamp:     time.Now(),
		SourceCluster: source,
		TargetCluster: target,
		Namespace:     namespace,
		Reason:        reason,
		Success:       success,
	}
	m.failoverEvents = append(m.failoverEvents, event)
	m.stats.FailoverEvents++
}

// ==================== 负载均衡统计更新 ====================

func (m *ClusterFederationManager) loadBalanceUpdateLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case
		<-ticker.C:
			m.updateStats()
		}
	}
}

func (m *ClusterFederationManager) updateStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalClusters = len(m.clusters)
	m.stats.TotalNamespaces = len(m.namespaces)
	m.stats.HealthyClusters = 0
	m.stats.TotalNodes = 0
	m.stats.TotalCapacity = 0
	m.stats.TotalUsedCapacity = 0
	m.stats.ActiveSyncs = 0

	for _, cluster := range m.clusters {
		if cluster.HealthStatus == HealthStatusHealthy {
			m.stats.HealthyClusters++
		}
		m.stats.TotalNodes += len(cluster.Nodes)
		for _, node := range cluster.Nodes {
			m.stats.TotalCapacity += node.Capacity
			m.stats.TotalUsedCapacity += node.Used
		}
	}

	for _, task := range m.syncTasks {
		if task.Status == "running" {
			m.stats.ActiveSyncs++
		}
	}
}

// ==================== 监控接口 ====================

// GetStats 获取联邦统计信息
func (m *ClusterFederationManager) GetStats() FederationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetFailoverEvents 获取故障转移事件历史
func (m *ClusterFederationManager) GetFailoverEvents() []*failoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*failoverEvent, len(m.failoverEvents))
	copy(events, m.failoverEvents)
	return events
}

// GetSyncTasks 获取同步任务列表
func (m *ClusterFederationManager) GetSyncTasks() []*syncTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*syncTask, len(m.syncTasks))
	copy(tasks, m.syncTasks)
	return tasks
}

// GetConfig 获取当前联邦配置
func (m *ClusterFederationManager) GetConfig() FederationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ListClusters 列出所有已注册集群
func (m *ClusterFederationManager) ListClusters() []*Cluster {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]*Cluster, 0, len(m.clusters))
	for _, c := range m.clusters {
		clusters = append(clusters, c)
	}
	return clusters
}

// AddNode 向集群添加节点
func (m *ClusterFederationManager) AddNode(clusterID string, node *ClusterNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.clusters[clusterID]
	if !exists {
		return ErrClusterNotFound
	}

	if _, exists := cluster.Nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已存在于集群 %s", node.ID, clusterID)
	}

	node.LastHeartbeat = time.Now()
	cluster.Nodes[node.ID] = node
	cluster.UpdatedAt = time.Now()

	fmt.Printf("[multiclusterfed] 节点 %s (%s) 已加入集群 %s\n",
		node.ID, node.Hostname, clusterID)
	return nil
}

// RemoveNode 从集群移除节点
func (m *ClusterFederationManager) RemoveNode(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.clusters[clusterID]
	if !exists {
		return ErrClusterNotFound
	}

	if _, exists := cluster.Nodes[nodeID]; !exists {
		return fmt.Errorf("节点 %s 不存在于集群 %s", nodeID, clusterID)
	}

	delete(cluster.Nodes, nodeID)
	cluster.UpdatedAt = time.Now()

	fmt.Printf("[multiclusterfed] 节点 %s 已从集群 %s 移除\n", nodeID, clusterID)
	return nil
}

// UpdateNodeHeartbeat 更新节点心跳时间
func (m *ClusterFederationManager) UpdateNodeHeartbeat(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, exists := m.clusters[clusterID]
	if !exists {
		return ErrClusterNotFound
	}

	node, exists := cluster.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在于集群 %s", nodeID, clusterID)
	}

	node.LastHeartbeat = time.Now()
	return nil
}

// TriggerSync 手动触发同步任务
func (m *ClusterFederationManager) TriggerSync(sourceCluster, targetCluster, namespace string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return "", ErrManagerNotRunning
	}

	if _, exists := m.clusters[sourceCluster]; !exists {
		return "", ErrClusterNotFound
	}
	if _, exists := m.clusters[targetCluster]; !exists {
		return "", ErrClusterNotFound
	}

	m.createSyncTask(sourceCluster, targetCluster, namespace)
	taskID := m.syncTasks[len(m.syncTasks)-1].ID

	fmt.Printf("[multiclusterfed] 手动触发同步任务: %s (源: %s, 目标: %s)\n",
		taskID, sourceCluster, targetCluster)
	return taskID, nil
}
