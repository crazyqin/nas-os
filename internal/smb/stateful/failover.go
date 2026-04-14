package stateful

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// FailoverCallback 故障转移回调
type FailoverCallback func(event FailoverEvent)

// TransparentReconnectConfig 透明重连配置
type TransparentReconnectConfig struct {
	MaxRetries        int           // 最大重试次数
	RetryInterval     time.Duration // 重试间隔
	SessionTimeout    time.Duration // 会话超时
	GracefulTimeout   time.Duration // 优雅切换超时
}

// FailoverIntegration 故障转移集成器
// Phase3: 负责与HA模块深度集成、故障检测、自动切换、会话回归
type FailoverIntegration struct {
	manager  *StatefulFailoverManager
	lb       *SMBClientLoadBalancer
	config   *FailoverConfig

	mu              sync.RWMutex
	reconnectCfg    TransparentReconnectConfig
	callbacks       []FailoverCallback
	pendingFailover sync.Map // string -> chan struct{}
	failoverCount   atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
}

// FailoverConfig 故障转移配置
type FailoverConfig struct {
	Enabled                   bool          `json:"enabled"`
	AutoFailover              bool          `json:"auto_failover"`
	FailoverThreshold         int           `json:"failover_threshold"`          // 连续失败次数阈值
	HealthCheckInterval       time.Duration `json:"health_check_interval"`      // 健康检查间隔
	FailoverTimeout           time.Duration `json:"failover_timeout"`           // 故障转移超时
	RecoveryCheckInterval     time.Duration `json:"recovery_check_interval"`    // 恢复检查间隔
	GracePeriod               time.Duration `json:"grace_period"`                // 故障判定宽限期
	TransparentReconnect      bool          `json:"transparent_reconnect"`      // 透明重连
	MaxReconnectRetries       int           `json:"max_reconnect_retries"`       // 最大重连次数
	ReconnectBackoff         time.Duration `json:"reconnect_backoff"`           // 重连退避间隔
	SessionMigrationTimeout   time.Duration `json:"session_migration_timeout"`   // 会话迁移超时
	EnableAutoReturn         bool          `json:"enable_auto_return"`           // 启用自动回归
	AutoReturnThreshold      int           `json:"auto_return_threshold"`       // 自动回归判定次数
}

// DefaultFailoverConfig 默认配置
func DefaultFailoverConfig() *FailoverConfig {
	return &FailoverConfig{
		Enabled:                 true,
		AutoFailover:            true,
		FailoverThreshold:       3,
		HealthCheckInterval:     5 * time.Second,
		FailoverTimeout:         30 * time.Second,
		RecoveryCheckInterval:   30 * time.Second,
		GracePeriod:             10 * time.Second,
		TransparentReconnect:    true,
		MaxReconnectRetries:     5,
		ReconnectBackoff:        2 * time.Second,
		SessionMigrationTimeout: 60 * time.Second,
		EnableAutoReturn:        true,
		AutoReturnThreshold:     5,
	}
}

// NewFailoverIntegration 创建故障转移集成器
func NewFailoverIntegration(manager *StatefulFailoverManager, lb *SMBClientLoadBalancer, cfg *FailoverConfig) *FailoverIntegration {
	if cfg == nil {
		cfg = DefaultFailoverConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	fi := &FailoverIntegration{
		manager:  manager,
		lb:       lb,
		config:   cfg,
		reconnectCfg: TransparentReconnectConfig{
			MaxRetries:      cfg.MaxReconnectRetries,
			RetryInterval:   cfg.ReconnectBackoff,
			SessionTimeout:  cfg.SessionMigrationTimeout,
			GracefulTimeout:  cfg.FailoverTimeout,
		},
		callbacks: make([]FailoverCallback, 0),
		ctx:       ctx,
		cancel:    cancel,
	}
	return fi
}

// RegisterCallback 注册故障转移事件回调
func (fi *FailoverIntegration) RegisterCallback(cb FailoverCallback) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.callbacks = append(fi.callbacks, cb)
}

// Start 启动故障转移集成器
func (fi *FailoverIntegration) Start() error {
	if !fi.config.Enabled {
		return nil
	}
	go fi.failoverDetectionLoop()
	go fi.recoveryDetectionLoop()
	go fi.failoverEventWatcher()
	return nil
}

// Stop 停止故障转移集成器
func (fi *FailoverIntegration) Stop() error {
	fi.cancel()
	return nil
}

// TriggerFailoverWithReconnect 触发故障转移并等待客户端透明重连
// 返回: 迁移成功的会话数、失败的会话数、错误
func (fi *FailoverIntegration) TriggerFailoverWithReconnect(failedNodeID string) (int, int, error) {
	if !fi.config.AutoFailover {
		return 0, 0, fmt.Errorf("自动故障转移已禁用")
	}

	// 1. 查找目标节点
	target := fi.lb.SelectNode("")
	if target == nil || target.NodeID == failedNodeID {
		target = fi.manager.findBestTarget()
	}
	if target == nil {
		return 0, 0, fmt.Errorf("无可用目标节点")
	}

	// 2. 获取待迁移会话
	sessions := fi.manager.registry.GetByNode(failedNodeID)
	if len(sessions) == 0 {
		return 0, 0, nil
	}

	// 3. 标记故障转移进行中
	failoverKey := failedNodeID
	doneCh := make(chan struct{})
	fi.pendingFailover.Store(failoverKey, doneCh)
	defer fi.pendingFailover.Delete(failoverKey)

	// 4. 执行故障转移
	recovered, failed := fi.migrateSessionsWithRetry(sessions, target)

	// 5. 触发HA切换
	fi.notifyHAForSwitchover(failedNodeID, target.NodeID)

	// 6. 等待客户端重连
	if fi.config.TransparentReconnect {
		go fi.waitForClientReconnect(failedNodeID, target, sessions)
	}

	fi.failoverCount.Add(1)
	return recovered, failed, nil
}

// migrateSessionsWithRetry 带重试的会话迁移
func (fi *FailoverIntegration) migrateSessionsWithRetry(sessions []*SessionState, target *FailoverNode) (int, int) {
	recovered, failed := 0, 0
	concurrency := fi.manager.config.RecoveryConcurrency
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, session := range sessions {
		wg.Add(1)
		sem <- struct{}{}
		go func(s *SessionState) {
			defer wg.Done()
			defer func() { <-sem }()

			var lastErr error
			for attempt := 0; attempt <= fi.config.MaxReconnectRetries; attempt++ {
				if err := fi.manager.migrateSession(s, target); err != nil {
					lastErr = err
					if attempt < fi.config.MaxReconnectRetries {
						time.Sleep(fi.config.ReconnectBackoff * time.Duration(attempt+1))
						continue
					}
				} else {
					mu.Lock()
					recovered++
					mu.Unlock()
					return
				}
			}

			mu.Lock()
			failed++
			mu.Unlock()
			fi.manager.eventCh <- FailoverEvent{
				Type:      EventFailoverFailed,
				Timestamp: time.Now(),
				NodeID:    target.NodeID,
				SessionID: s.SessionID,
				ShareName: s.ShareName,
				Message:   fmt.Sprintf("会话 %s 迁移失败 (重试%d次): %v", s.SessionID, fi.config.MaxReconnectRetries, lastErr),
			}
		}(session)
	}
	wg.Wait()
	return recovered, failed
}

// waitForClientReconnect 等待客户端重连
func (fi *FailoverIntegration) waitForClientReconnect(failedNodeID string, target *FailoverNode, sessions []*SessionState) {
	timeout := time.After(fi.config.FailoverTimeout)
	ticker := time.NewTicker(fi.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			// 检查有多少客户端已在新节点建立连接
			migratedCount := 0
			for _, s := range sessions {
				if fi.manager.registry.Get(s.SessionID) != nil &&
					fi.manager.registry.Get(s.SessionID).NodeID == target.NodeID {
					migratedCount++
				}
			}
			if migratedCount >= len(sessions)/2 {
				// 超过一半客户端已迁移，认为切换成功
				return
			}
		}
	}
}

// notifyHAForSwitchover 通知HA模块执行切换
func (fi *FailoverIntegration) notifyHAForSwitchover(fromNode, toNode string) {
	// Phase3: 与外部HA模块（如keepalived, CTDB）集成的钩子
	// 当前实现触发内部事件，实际生产环境应调用外部API或发送网络消息
	fi.manager.eventCh <- FailoverEvent{
		Type:      EventFailoverComplete,
		Timestamp: time.Now(),
		NodeID:    toNode,
		Message:   fmt.Sprintf("HA切换: 从 %s 到 %s", fromNode, toNode),
	}
}

// failoverDetectionLoop 故障检测循环
func (fi *FailoverIntegration) failoverDetectionLoop() {
	ticker := time.NewTicker(fi.config.HealthCheckInterval)
	defer ticker.Stop()

	failureCount := make(map[string]int) // nodeID -> 连续失败次数

	for {
		select {
		case <-fi.ctx.Done():
			return
		case <-ticker.C:
			fi.checkAndHandleFailover(failureCount)
		}
	}
}

// checkAndHandleFailover 检查并处理故障
func (fi *FailoverIntegration) checkAndHandleFailover(failureCount map[string]int) {
	fi.manager.mu.RLock()
	defer fi.manager.mu.RUnlock()

	for _, node := range fi.manager.peerNodes {
		if node.IsLocal {
			continue
		}

		isUnhealthy := fi.isNodeUnhealthy(node)
		if isUnhealthy {
			failureCount[node.NodeID]++
			if failureCount[node.NodeID] >= fi.config.FailoverThreshold {
				// 触发自动故障转移
				go func(nodeID string) {
					_, _, _ = fi.TriggerFailoverWithReconnect(nodeID)
				}(node.NodeID)
				delete(failureCount, node.NodeID)
			}
		} else {
			delete(failureCount, node.NodeID)
		}
	}
}

// isNodeUnhealthy 判断节点是否不健康
func (fi *FailoverIntegration) isNodeUnhealthy(node *FailoverNode) bool {
	// 检查节点状态
	if node.Status == NodeStatusOffline || node.Status == NodeStatusFailing {
		return true
	}

	// 检查心跳超时
	if time.Since(node.LastHB) > fi.config.GracePeriod+fi.config.HealthCheckInterval {
		return true
	}

	// 检查健康分数
	if node.HealthScore < 30 {
		return true
	}

	// 主动TCP检测
	if fi.config.HealthCheckInterval >= 10*time.Second { // 避免过于频繁
		if !fi.tcpProbeNode(node) {
			return true
		}
	}

	return false
}

// tcpProbeNode TCP探活节点
func (fi *FailoverIntegration) tcpProbeNode(node *FailoverNode) bool {
	addr := fmt.Sprintf("%s:%d", node.Address, node.Port)
	ctx, cancel := context.WithTimeout(fi.ctx, 3*time.Second)
	defer cancel()

	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// recoveryDetectionLoop 恢复检测循环（节点恢复后触发会话回归）
func (fi *FailoverIntegration) recoveryDetectionLoop() {
	if !fi.config.EnableAutoReturn {
		return
	}
	ticker := time.NewTicker(fi.config.RecoveryCheckInterval)
	defer ticker.Stop()

	recoveryCount := make(map[string]int) // nodeID -> 连续健康次数

	for {
		select {
		case <-fi.ctx.Done():
			return
		case <-ticker.C:
			fi.checkAndHandleRecovery(recoveryCount)
		}
	}
}

// checkAndHandleRecovery 检查并处理节点恢复
func (fi *FailoverIntegration) checkAndHandleRecovery(recoveryCount map[string]int) {
	fi.manager.mu.RLock()
	defer fi.manager.mu.RUnlock()

	for _, node := range fi.manager.peerNodes {
		if node.IsLocal {
			continue
		}

		isHealthy := fi.isNodeHealthy(node)
		if isHealthy {
			recoveryCount[node.NodeID]++
			if recoveryCount[node.NodeID] >= fi.config.AutoReturnThreshold {
				// 触发会话回归
				go fi.triggerSessionReturn(node.NodeID)
				delete(recoveryCount, node.NodeID)
			}
		} else {
			delete(recoveryCount, node.NodeID)
		}
	}
}

// isNodeHealthy 判断节点是否健康
func (fi *FailoverIntegration) isNodeHealthy(node *FailoverNode) bool {
	if node.Status == NodeStatusActive || node.Status == NodeStatusStandby {
		if time.Since(node.LastHB) < fi.config.HealthCheckInterval*3 {
			if node.HealthScore >= 70 {
				return true
			}
		}
	}
	return false
}

// triggerSessionReturn 触发会话回归到恢复的节点
func (fi *FailoverIntegration) triggerSessionReturn(nodeID string) {
	// 获取当前归属本节点但不在此节点上的会话
	allSessions := fi.manager.registry.ListAll()
	var returnable []*SessionState

	for _, s := range allSessions {
		if s.NodeID != nodeID && s.NodeID == fi.manager.localNode.NodeID {
			returnable = append(returnable, s)
		}
	}

	if len(returnable) == 0 {
		return
	}

	// 获取目标节点引用
	fi.manager.mu.RLock()
	target, ok := fi.manager.peerNodes[nodeID]
	fi.manager.mu.RUnlock()
	if !ok {
		return
	}

	recovered, failed := fi.migrateSessionsWithRetry(returnable, target)
	fi.manager.eventCh <- FailoverEvent{
		Type:      EventNodeRecovered,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Message:   fmt.Sprintf("会话回归完成: 恢复 %d, 失败 %d", recovered, failed),
	}
}

// failoverEventWatcher 监听故障转移事件并通知回调
func (fi *FailoverIntegration) failoverEventWatcher() {
	for {
		select {
		case <-fi.ctx.Done():
			return
		case event, ok := <-fi.manager.eventCh:
			if !ok {
				return
			}
			fi.notifyCallbacks(event)
		}
	}
}

// notifyCallbacks 通知所有回调
func (fi *FailoverIntegration) notifyCallbacks(event FailoverEvent) {
	fi.mu.RLock()
	cbs := fi.callbacks
	fi.mu.RUnlock()

	for _, cb := range cbs {
		go cb(event)
	}
}

// GetFailoverCount 获取累计故障转移次数
func (fi *FailoverIntegration) GetFailoverCount() int64 {
	return fi.failoverCount.Load()
}

// GetPendingFailovers 获取正在进行的故障转移
func (fi *FailoverIntegration) GetPendingFailovers() []string {
	var keys []string
	fi.pendingFailover.Range(func(key, _ interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

// TransparentReconnectInfo 透明重连信息
type TransparentReconnectInfo struct {
	SessionID     string    `json:"session_id"`
	ClientIP      string    `json:"client_ip"`
	OldNodeID     string    `json:"old_node_id"`
	NewNodeID     string    `json:"new_node_id"`
	RetryCount    int       `json:"retry_count"`
	ConnectedAt   time.Time `json:"connected_at"`
}

// PrepareTransparentReconnect 准备透明重连（为客户端生成重连令牌）
func (fi *FailoverIntegration) PrepareTransparentReconnect(sessionID, clientIP, oldNodeID, newNodeID string) (*TransparentReconnectInfo, error) {
	info := &TransparentReconnectInfo{
		SessionID:   sessionID,
		ClientIP:    clientIP,
		OldNodeID:   oldNodeID,
		NewNodeID:   newNodeID,
		RetryCount:  0,
		ConnectedAt: time.Now(),
	}

	// 将重连信息序列化到会话metadata
	session := fi.manager.registry.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("会话 %s 不存在", sessionID)
	}

	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	data, _ := json.Marshal(info)
	session.Metadata["reconnect_info"] = string(data)
	session.Metadata["pending_reconnect"] = "true"

	return info, nil
}

// CompleteTransparentReconnect 完成透明重连
func (fi *FailoverIntegration) CompleteTransparentReconnect(sessionID string) error {
	session := fi.manager.registry.Get(sessionID)
	if session == nil {
		return fmt.Errorf("会话 %s 不存在", sessionID)
	}

	if session.Metadata != nil {
		delete(session.Metadata, "pending_reconnect")
		delete(session.Metadata, "reconnect_info")
	}
	return nil
}

// HAIntegrationInterface HA模块集成接口（供外部HA系统调用）
type HAIntegrationInterface interface {
	// GetCurrentMaster 获取当前主节点
	GetCurrentMaster() (string, error)
	// RequestSwitchover 请求切换到指定节点
	RequestSwitchover(targetNodeID string) error
	// GetClusterHealth 获取集群健康状态
	GetClusterHealth() (*ClusterHealth, error)
}

// ClusterHealth 集群健康状态
type ClusterHealth struct {
	ClusterName string                    `json:"cluster_name"`
	Nodes       map[string]*NodeHealthInfo `json:"nodes"`
	Overall     string                    `json:"overall"` // "healthy", "degraded", "critical"
}

// NodeHealthInfo 节点健康信息
type NodeHealthInfo struct {
	NodeID      string    `json:"node_id"`
	Status      NodeStatus `json:"status"`
	HealthScore int       `json:"health_score"`
	LoadAvg     float64   `json:"load_avg"`
	MemUsage    float64   `json:"mem_usage"`
	CPUUsage    float64   `json:"cpu_usage"`
}

// GetClusterHealth 获取集群健康状态
func (fi *FailoverIntegration) GetClusterHealth() (*ClusterHealth, error) {
	fi.manager.mu.RLock()
	defer fi.manager.mu.RUnlock()

	ch := &ClusterHealth{
		ClusterName: fi.manager.config.ClusterName,
		Nodes:       make(map[string]*NodeHealthInfo),
		Overall:     "healthy",
	}

	// 本地节点
	ch.Nodes[fi.manager.localNode.NodeID] = &NodeHealthInfo{
		NodeID:      fi.manager.localNode.NodeID,
		Status:      fi.manager.localNode.Status,
		HealthScore: fi.manager.localNode.HealthScore,
	}
	if fi.manager.localNode.Status == NodeStatusDegraded {
		ch.Overall = "degraded"
	}

	// 对等节点
	for _, node := range fi.manager.peerNodes {
		ch.Nodes[node.NodeID] = &NodeHealthInfo{
			NodeID:      node.NodeID,
			Status:      node.Status,
			HealthScore: node.HealthScore,
		}
		if node.Status == NodeStatusOffline || node.Status == NodeStatusFailing {
			ch.Overall = "critical"
		} else if node.Status == NodeStatusDegraded && ch.Overall != "critical" {
			ch.Overall = "degraded"
		}
	}

	return ch, nil
}
