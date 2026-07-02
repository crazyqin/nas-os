// Package smartqos 实现智能存储服务质量(QoS)引擎
// 对标: TrueNAS 存储QoS + 群晖 存储性能管理
// 根据应用类型、优先级、时间等因素动态调整IO资源分配
package smartqos

import (
	"fmt"
	"sync"
	"time"
)

// IOPriority IO优先级.
type IOPriority int

const (
	PriorityCritical IOPriority = 0 // 关键业务
	PriorityHigh     IOPriority = 1 // 高优先级
	PriorityNormal   IOPriority = 2 // 普通
	PriorityLow      IOPriority = 3 // 低优先级
	PriorityBatch    IOPriority = 4 // 批处理
)

// AppType 应用类型.
type AppType string

const (
	AppDatabase   AppType = "database"   // 数据库
	AppWebServer  AppType = "webserver"  // Web服务
	AppBackup     AppType = "backup"     // 备份
	AppMedia      AppType = "media"      // 媒体流
	AppVM         AppType = "vm"         // 虚拟机
	AppDocker     AppType = "docker"     // 容器
	AppFileServer AppType = "fileserver" // 文件服务
	AppAI         AppType = "ai"         // AI计算
	AppArchive    AppType = "archive"    // 归档
	AppDefault    AppType = "default"    // 默认
)

// QoSPolicy QoS策略.
type QoSPolicy struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	AppType       AppType    `json:"app_type"`
	Priority      IOPriority `json:"priority"`
	MaxIOPS       int64      `json:"max_iops"`       // 最大IOPS
	MinIOPS       int64      `json:"min_iops"`       // 最小保障IOPS
	MaxBandwidth  int64      `json:"max_bandwidth"`  // 最大带宽(MB/s)
	MinBandwidth  int64      `json:"min_bandwidth"`  // 最小保障带宽(MB/s)
	MaxLatency    int64      `json:"max_latency"`    // 最大延迟(ms)
	BurstIOPS     int64      `json:"burst_iops"`     // 突发IOPS
	BurstDuration int        `json:"burst_duration"` // 突发持续时间(秒)
	Enabled       bool       `json:"enabled"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// IOMetric IO指标.
type IOMetric struct {
	Timestamp    time.Time `json:"timestamp"`
	IOPS         int64     `json:"iops"`
	ReadIOPS     int64     `json:"read_iops"`
	WriteIOPS    int64     `json:"write_iops"`
	Bandwidth    int64     `json:"bandwidth"` // MB/s
	ReadBW       int64     `json:"read_bw"`
	WriteBW      int64     `json:"write_bw"`
	Latency      int64     `json:"latency"` // ms
	ReadLatency  int64     `json:"read_latency"`
	WriteLatency int64     `json:"write_latency"`
	QueueDepth   int       `json:"queue_depth"`
	Utilization  float64   `json:"utilization"` // 0-100
}

// QoSNode QoS节点(被管理的存储资源).
type QoSNode struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"` // disk, pool, volume
	Path       string    `json:"path"`
	PolicyID   string    `json:"policy_id"`
	Metric     *IOMetric `json:"metric"`
	Throttled  bool      `json:"throttled"`
	LastUpdate time.Time `json:"last_update"`
}

// Engine QoS引擎.
type Engine struct {
	mu            sync.RWMutex
	policies      map[string]*QoSPolicy
	nodes         map[string]*QoSNode
	appDefaults   map[AppType]*QoSPolicy
	metrics       map[string][]*IOMetric // nodeID -> metrics
	maxMetrics    int
	totalThrottle int64
	totalAllow    int64
}

// NewEngine 创建QoS引擎.
func NewEngine() *Engine {
	e := &Engine{
		policies:    make(map[string]*QoSPolicy),
		nodes:       make(map[string]*QoSNode),
		appDefaults: make(map[AppType]*QoSPolicy),
		metrics:     make(map[string][]*IOMetric),
		maxMetrics:  1000,
	}
	e.registerDefaults()
	return e
}

// registerDefaults 注册默认策略.
func (e *Engine) registerDefaults() {
	defaults := map[AppType]*QoSPolicy{
		AppDatabase: {
			ID: "default-db", Name: "数据库默认策略", AppType: AppDatabase,
			Priority: PriorityCritical, MinIOPS: 10000, MaxIOPS: 100000,
			MinBandwidth: 500, MaxBandwidth: 5000, MaxLatency: 1,
			BurstIOPS: 150000, BurstDuration: 30, Enabled: true,
		},
		AppWebServer: {
			ID: "default-web", Name: "Web服务默认策略", AppType: AppWebServer,
			Priority: PriorityHigh, MinIOPS: 5000, MaxIOPS: 50000,
			MinBandwidth: 200, MaxBandwidth: 2000, MaxLatency: 5,
			BurstIOPS: 80000, BurstDuration: 60, Enabled: true,
		},
		AppBackup: {
			ID: "default-backup", Name: "备份默认策略", AppType: AppBackup,
			Priority: PriorityLow, MinIOPS: 1000, MaxIOPS: 20000,
			MinBandwidth: 100, MaxBandwidth: 1000, MaxLatency: 100,
			BurstIOPS: 30000, BurstDuration: 120, Enabled: true,
		},
		AppMedia: {
			ID: "default-media", Name: "媒体流默认策略", AppType: AppMedia,
			Priority: PriorityHigh, MinIOPS: 3000, MaxIOPS: 30000,
			MinBandwidth: 300, MaxBandwidth: 3000, MaxLatency: 10,
			BurstIOPS: 50000, BurstDuration: 60, Enabled: true,
		},
		AppAI: {
			ID: "default-ai", Name: "AI计算默认策略", AppType: AppAI,
			Priority: PriorityHigh, MinIOPS: 8000, MaxIOPS: 80000,
			MinBandwidth: 500, MaxBandwidth: 8000, MaxLatency: 5,
			BurstIOPS: 120000, BurstDuration: 30, Enabled: true,
		},
		AppArchive: {
			ID: "default-archive", Name: "归档默认策略", AppType: AppArchive,
			Priority: PriorityBatch, MinIOPS: 500, MaxIOPS: 10000,
			MinBandwidth: 50, MaxBandwidth: 500, MaxLatency: 500,
			BurstIOPS: 15000, BurstDuration: 300, Enabled: true,
		},
	}
	for _, p := range defaults {
		p.CreatedAt = time.Now()
		p.UpdatedAt = time.Now()
		e.appDefaults[p.AppType] = p
	}
}

// CreatePolicy 创建策略.
func (e *Engine) CreatePolicy(policy *QoSPolicy) error {
	if policy.ID == "" {
		return fmt.Errorf("策略ID不能为空")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[policy.ID]; exists {
		return fmt.Errorf("策略 %s 已存在", policy.ID)
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	e.policies[policy.ID] = policy
	return nil
}

// UpdatePolicy 更新策略.
func (e *Engine) UpdatePolicy(policy *QoSPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.policies[policy.ID]
	if !ok {
		return fmt.Errorf("策略 %s 不存在", policy.ID)
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()
	e.policies[policy.ID] = policy
	return nil
}

// DeletePolicy 删除策略.
func (e *Engine) DeletePolicy(policyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.policies[policyID]; !ok {
		return fmt.Errorf("策略 %s 不存在", policyID)
	}

	// 检查是否有节点使用此策略
	for _, node := range e.nodes {
		if node.PolicyID == policyID {
			return fmt.Errorf("策略 %s 正在被节点 %s 使用", policyID, node.ID)
		}
	}

	delete(e.policies, policyID)
	return nil
}

// GetPolicy 获取策略.
func (e *Engine) GetPolicy(policyID string) (*QoSPolicy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy, ok := e.policies[policyID]
	if !ok {
		// 检查默认策略
		for _, p := range e.appDefaults {
			if p.ID == policyID {
				return p, nil
			}
		}
		return nil, fmt.Errorf("策略 %s 不存在", policyID)
	}
	return policy, nil
}

// RegisterNode 注册QoS节点.
func (e *Engine) RegisterNode(node *QoSNode) error {
	if node.ID == "" {
		return fmt.Errorf("节点ID不能为空")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已存在", node.ID)
	}

	node.LastUpdate = time.Now()
	e.nodes[node.ID] = node
	return nil
}

// UnregisterNode 注销节点.
func (e *Engine) UnregisterNode(nodeID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.nodes[nodeID]; !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	delete(e.nodes, nodeID)
	delete(e.metrics, nodeID)
	return nil
}

// AssignPolicy 分配策略给节点.
func (e *Engine) AssignPolicy(nodeID, policyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, ok := e.nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	// 检查策略是否存在
	_, exists := e.policies[policyID]
	if !exists {
		// 检查默认策略
		found := false
		for _, p := range e.appDefaults {
			if p.ID == policyID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("策略 %s 不存在", policyID)
		}
	}

	node.PolicyID = policyID
	node.LastUpdate = time.Now()
	return nil
}

// ReportMetric 上报指标.
func (e *Engine) ReportMetric(nodeID string, metric *IOMetric) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	node, ok := e.nodes[nodeID]
	if !ok {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	metric.Timestamp = time.Now()
	node.Metric = metric
	node.LastUpdate = time.Now()

	// 存储指标历史
	e.metrics[nodeID] = append(e.metrics[nodeID], metric)
	if len(e.metrics[nodeID]) > e.maxMetrics {
		e.metrics[nodeID] = e.metrics[nodeID][1:]
	}

	return nil
}

// EvaluateQoS 评估QoS - 判断是否需要限流.
func (e *Engine) EvaluateQoS(nodeID string) (allowed bool, reason string, err error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	node, ok := e.nodes[nodeID]
	if !ok {
		return false, "", fmt.Errorf("节点 %s 不存在", nodeID)
	}

	if node.Metric == nil {
		return true, "无指标数据，默认允许", nil
	}

	// 获取策略
	policy := e.getPolicyForNode(node)
	if policy == nil || !policy.Enabled {
		return true, "无策略或策略禁用，默认允许", nil
	}

	// 检查IOPS限制
	if policy.MaxIOPS > 0 && node.Metric.IOPS > policy.MaxIOPS {
		// 检查突发
		if policy.BurstIOPS > 0 && node.Metric.IOPS <= policy.BurstIOPS {
			return true, fmt.Sprintf("突发IOPS %d 在限制 %d 内", node.Metric.IOPS, policy.BurstIOPS), nil
		}
		e.totalThrottle++
		node.Throttled = true
		return false, fmt.Sprintf("IOPS %d 超过限制 %d", node.Metric.IOPS, policy.MaxIOPS), nil
	}

	// 检查带宽限制
	if policy.MaxBandwidth > 0 && node.Metric.Bandwidth > policy.MaxBandwidth {
		e.totalThrottle++
		node.Throttled = true
		return false, fmt.Sprintf("带宽 %dMB/s 超过限制 %dMB/s", node.Metric.Bandwidth, policy.MaxBandwidth), nil
	}

	// 检查延迟
	if policy.MaxLatency > 0 && node.Metric.Latency > policy.MaxLatency {
		e.totalThrottle++
		node.Throttled = true
		return false, fmt.Sprintf("延迟 %dms 超过限制 %dms", node.Metric.Latency, policy.MaxLatency), nil
	}

	e.totalAllow++
	node.Throttled = false
	return true, "在QoS限制内", nil
}

// getPolicyForNode 获取节点的策略.
func (e *Engine) getPolicyForNode(node *QoSNode) *QoSPolicy {
	if node.PolicyID != "" {
		if p, ok := e.policies[node.PolicyID]; ok {
			return p
		}
		for _, p := range e.appDefaults {
			if p.ID == node.PolicyID {
				return p
			}
		}
	}
	return nil
}

// GetNode 获取节点.
func (e *Engine) GetNode(nodeID string) (*QoSNode, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	node, ok := e.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("节点 %s 不存在", nodeID)
	}
	return node, nil
}

// ListNodes 列出节点.
func (e *Engine) ListNodes() []*QoSNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	nodes := make([]*QoSNode, 0)
	for _, node := range e.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// ListPolicies 列出策略.
func (e *Engine) ListPolicies() []*QoSPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*QoSPolicy, 0)
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	// 包含默认策略
	for _, p := range e.appDefaults {
		policies = append(policies, p)
	}
	return policies
}

// GetNodeMetrics 获取节点指标历史.
func (e *Engine) GetNodeMetrics(nodeID string, limit int) []*IOMetric {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metrics, ok := e.metrics[nodeID]
	if !ok {
		return nil
	}

	if limit > 0 && len(metrics) > limit {
		return metrics[len(metrics)-limit:]
	}
	return metrics
}

// GetStats 获取统计信息.
func (e *Engine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	throttledCount := 0
	for _, node := range e.nodes {
		if node.Throttled {
			throttledCount++
		}
	}

	return map[string]interface{}{
		"total_nodes":     len(e.nodes),
		"total_policies":  len(e.policies) + len(e.appDefaults),
		"throttled_nodes": throttledCount,
		"total_throttle":  e.totalThrottle,
		"total_allow":     e.totalAllow,
	}
}

// GetDefaultPolicyForApp 获取应用默认策略.
func (e *Engine) GetDefaultPolicyForApp(appType AppType) *QoSPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.appDefaults[appType]
}
