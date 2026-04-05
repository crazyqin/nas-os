// Package nvmeof ANA (Asymmetric Namespace Access) 多路径支持
// 实现NVMe-oF高可用多路径访问，对标TrueNAS Enterprise HA
package nvmeof

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ========== Logger 接口定义 ==========

// Logger 日志接口（ANA模块使用）
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// ========== ANA 状态定义 ==========

// ANAState ANA访问状态
type ANAState uint8

const (
	// ANAOptimizedPath 优化路径（首选）
	ANAOptimizedPath ANAState = 0x01
	// ANANonOptimizedPath 非优化路径（备用）
	ANANonOptimizedPath ANAState = 0x02
	// ANAInaccessible 不可访问
	ANAInaccessible ANAState = 0x03
	// ANAPersistentLoss 持久丢失
	ANAPersistentLoss ANAState = 0x04
	// ANAChangeInProgress 状态变更中
	ANAChangeInProgress ANAState = 0x0F
)

// String 返回ANA状态字符串
func (s ANAState) String() string {
	switch s {
	case ANAOptimizedPath:
		return "optimized"
	case ANANonOptimizedPath:
		return "non_optimized"
	case ANAInaccessible:
		return "inaccessible"
	case ANAPersistentLoss:
		return "persistent_loss"
	case ANAChangeInProgress:
		return "change_in_progress"
	default:
		return "unknown"
	}
}

// ========== ANA 组定义 ==========

// ANAGroup ANA组配置
type ANAGroup struct {
	ID           uint32       `json:"id"`             // ANA组ID
	Name         string       `json:"name"`           // 组名称
	State        ANAState     `json:"state"`          // 当前状态
	Grpid        uint32       `json:"grpid"`          // ANA组标识
	Paths        []*ANAPath   `json:"paths"`          // 组内路径列表
	Priority     int          `json:"priority"`       // 优先级（数值越小优先级越高）
	CreatedAt    time.Time    `json:"created_at"`     // 创建时间
	UpdatedAt    time.Time    `json:"updated_at"`     // 更新时间
	mu           sync.RWMutex `json:"-"`              // 保护内部状态
}

// ANAPath 单个路径配置
type ANAPath struct {
	ID            string       `json:"id"`              // 路径ID
	ControllerID  string       `json:"controller_id"`   // 控制器ID
	TransportType TransportType `json:"transport_type"`  // 传输类型
	Address       string       `json:"address"`         // 地址（IP:Port或RDMA地址）
	NQN           string       `json:"nqn"`             // NQN标识
	State         ANAState     `json:"state"`           // 路径状态
	Weight        int          `json:"weight"`          // 路径权重
	Latency       time.Duration `json:"latency"`        // 延迟
	IOPS          uint64       `json:"iops"`            // IOPS统计
	Throughput    uint64       `json:"throughput"`      // 吞吐量统计
	LastError     error        `json:"-"`               // 最后错误
	LastCheck     time.Time    `json:"last_check"`      // 最后检查时间
}

// ========== ANA 管理器 ==========

// ANAManager ANA多路径管理器
type ANAManager struct {
	groups    map[uint32]*ANAGroup    // ANA组映射
	paths     map[string]*ANAPath     // 路径映射
	selectors map[string]*PathSelector // 路径选择器
	config    *ANAConfig              // 配置
	running   atomic.Bool             // 运行状态
	ctx       context.Context         // 上下文
	cancel    context.CancelFunc      // 取消函数
	mu        sync.RWMutex            // 保护状态
	logger    Logger                  // 日志接口
}

// ANAConfig ANA配置
type ANAConfig struct {
	EnableANA           bool          `json:"enable_ana"`           // 启用ANA
	PathCheckInterval   time.Duration `json:"path_check_interval"`  // 路径检查间隔
	PathTimeout         time.Duration `json:"path_timeout"`         // 路径超时
	FailoverThreshold   int           `json:"failover_threshold"`   // 故障切换阈值
	RecoveryThreshold   int           `json:"recovery_threshold"`   // 恢复阈值
	LoadBalancePolicy   LBPolicy      `json:"load_balance_policy"`  // 负载均衡策略
	MaxPaths            int           `json:"max_paths"`            // 最大路径数
	HealthCheckEnabled  bool          `json:"health_check_enabled"` // 启用健康检查
}

// LBPolicy 负载均衡策略
type LBPolicy string

const (
	// LBPolicyRoundRobin 轮询
	LBPolicyRoundRobin LBPolicy = "round_robin"
	// LBPolicyWeighted 加权轮询
	LBPolicyWeighted LBPolicy = "weighted"
	// LBPolicyLeastLatency 最小延迟
	LBPolicyLeastLatency LBPolicy = "least_latency"
	// LBPolicyLeastLoad 最小负载
	LBPolicyLeastLoad LBPolicy = "least_load"
	// LBPolicyAdaptive 自适应
	LBPolicyAdaptive LBPolicy = "adaptive"
)

// DefaultANAConfig 默认ANA配置
func DefaultANAConfig() *ANAConfig {
	return &ANAConfig{
		EnableANA:          true,
		PathCheckInterval:  5 * time.Second,
		PathTimeout:        30 * time.Second,
		FailoverThreshold:  3,
		RecoveryThreshold:  2,
		LoadBalancePolicy:  LBPolicyWeighted,
		MaxPaths:           32,
		HealthCheckEnabled: true,
	}
}

// ========== ANA 管理器方法 ==========

var (
	ErrANAGroupNotFound      = errors.New("ana group not found")
	ErrANAPathNotFound       = errors.New("ana path not found")
	ErrANAGroupExists        = errors.New("ana group already exists")
	ErrANAPathExists         = errors.New("ana path already exists")
	ErrANANotEnabled         = errors.New("ana not enabled")
	ErrANAInvalidState       = errors.New("invalid ana state transition")
	ErrANANoAvailablePath    = errors.New("no available ana path")
	ErrANAFailoverFailed     = errors.New("ana failover failed")
)

// NewANAManager 创建ANA管理器
func NewANAManager(config *ANAConfig, logger Logger) *ANAManager {
	if config == nil {
		config = DefaultANAConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ANAManager{
		groups:    make(map[uint32]*ANAGroup),
		paths:     make(map[string]*ANAPath),
		selectors: make(map[string]*PathSelector),
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
	}
}

// Start 启动ANA管理器
func (m *ANAManager) Start() error {
	if !m.config.EnableANA {
		return ErrANANotEnabled
	}
	m.running.Store(true)
	
	// 启动路径健康检查
	if m.config.HealthCheckEnabled {
		go m.healthCheckLoop()
	}
	
	m.logger.Infof("ANA manager started")
	return nil
}

// Stop 停止ANA管理器
func (m *ANAManager) Stop() {
	m.running.Store(false)
	m.cancel()
	m.logger.Infof("ANA manager stopped")
}

// CreateGroup 创建ANA组
func (m *ANAManager) CreateGroup(id uint32, name string, priority int) (*ANAGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.groups[id]; exists {
		return nil, ErrANAGroupExists
	}
	
	group := &ANAGroup{
		ID:        id,
		Name:      name,
		State:     ANAOptimizedPath,
		Grpid:     id,
		Paths:     make([]*ANAPath, 0),
		Priority:  priority,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	m.groups[id] = group
	m.logger.Infof("ANA group created: id=%d, name=%s", id, name)
	return group, nil
}

// AddPath 向ANA组添加路径
func (m *ANAManager) AddPath(groupID uint32, path *ANAPath) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	group, exists := m.groups[groupID]
	if !exists {
		return ErrANAGroupNotFound
	}
	
	if _, exists := m.paths[path.ID]; exists {
		return ErrANAPathExists
	}
	
	path.State = ANAOptimizedPath
	path.LastCheck = time.Now()
	
	group.mu.Lock()
	group.Paths = append(group.Paths, path)
	group.UpdatedAt = time.Now()
	group.mu.Unlock()
	
	m.paths[path.ID] = path
	
	// 创建路径选择器
	m.selectors[group.Name] = NewPathSelector(m.config.LoadBalancePolicy, group.Paths)
	
	m.logger.Infof("ANA path added: group=%d, path=%s, controller=%s", 
		groupID, path.ID, path.ControllerID)
	return nil
}

// RemovePath 移除路径
func (m *ANAManager) RemovePath(pathID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.paths[pathID]; !exists {
		return ErrANAPathNotFound
	}
	
	// 从组中移除
	for _, group := range m.groups {
		group.mu.Lock()
		for i, p := range group.Paths {
			if p.ID == pathID {
				group.Paths = append(group.Paths[:i], group.Paths[i+1:]...)
				group.UpdatedAt = time.Now()
				break
			}
		}
		group.mu.Unlock()
	}
	
	delete(m.paths, pathID)
	m.logger.Infof("ANA path removed: path=%s", pathID)
	return nil
}

// GetOptimalPath 获取最优路径
func (m *ANAManager) GetOptimalPath(groupID uint32) (*ANAPath, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	group, exists := m.groups[groupID]
	if !exists {
		return nil, ErrANAGroupNotFound
	}
	
	selector, exists := m.selectors[group.Name]
	if !exists {
		return nil, ErrANANoAvailablePath
	}
	
	return selector.Select(), nil
}

// Failover 执行故障切换
func (m *ANAManager) Failover(groupID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	group, exists := m.groups[groupID]
	if !exists {
		return ErrANAGroupNotFound
	}
	
	// 查找备用路径
	var backupPath *ANAPath
	for _, path := range group.Paths {
		if path.State == ANANonOptimizedPath || path.State == ANAOptimizedPath {
			if backupPath == nil || path.Weight > backupPath.Weight {
				backupPath = path
			}
		}
	}
	
	if backupPath == nil {
		group.State = ANAInaccessible
		return ErrANAFailoverFailed
	}
	
	// 切换到备用路径
	group.State = ANAChangeInProgress
	backupPath.State = ANAOptimizedPath
	group.State = ANAOptimizedPath
	group.UpdatedAt = time.Now()
	
	m.logger.Infof("ANA failover completed: group=%d, new_primary=%s", 
		groupID, backupPath.ID)
	return nil
}

// ========== 健康检查循环 ==========

func (m *ANAManager) healthCheckLoop() {
	ticker := time.NewTicker(m.config.PathCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllPaths()
		}
	}
}

func (m *ANAManager) checkAllPaths() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, path := range m.paths {
		m.checkPathHealth(path)
	}
}

func (m *ANAManager) checkPathHealth(path *ANAPath) {
	// 模拟健康检查（实际实现需要调用nvme-cli或内核接口）
	// 这里简化为延迟检查
	start := time.Now()
	
	// 假设检查成功
	path.Latency = time.Since(start)
	path.LastCheck = time.Now()
	
	// 根据延迟更新状态
	if path.Latency > m.config.PathTimeout {
		path.State = ANAInaccessible
		m.logger.Warnf("ANA path degraded: path=%s, latency=%v", 
			path.ID, path.Latency)
	}
}

// ========== 路径选择器 ==========

// PathSelector 路径选择器
type PathSelector struct {
	policy  LBPolicy
	paths   []*ANAPath
	current atomic.Int64
	mu      sync.RWMutex
}

// NewPathSelector 创建路径选择器
func NewPathSelector(policy LBPolicy, paths []*ANAPath) *PathSelector {
	return &PathSelector{
		policy: policy,
		paths:  paths,
	}
}

// Select 选择路径
func (s *PathSelector) Select() *ANAPath {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if len(s.paths) == 0 {
		return nil
	}
	
	// 过滤可用路径
	available := make([]*ANAPath, 0)
	for _, p := range s.paths {
		if p.State == ANAOptimizedPath {
			available = append(available, p)
		}
	}
	
	if len(available) == 0 {
		// 尝试备用路径
		for _, p := range s.paths {
			if p.State == ANANonOptimizedPath {
				return p
			}
		}
		return nil
	}
	
	switch s.policy {
	case LBPolicyRoundRobin:
		idx := s.current.Add(1) % int64(len(available))
		return available[idx]
		
	case LBPolicyWeighted:
		// 加权选择（简化实现）
		totalWeight := 0
		for _, p := range available {
			totalWeight += p.Weight
		}
		if totalWeight == 0 {
			return available[0]
		}
		// 随机选择（实际应按权重分布）
		idx := s.current.Add(1) % int64(len(available))
		return available[idx]
		
	case LBPolicyLeastLatency:
		// 最小延迟
		best := available[0]
		for _, p := range available {
			if p.Latency < best.Latency {
				best = p
			}
		}
		return best
		
	default:
		return available[0]
	}
}

// ========== 统计信息 ==========

// GetStats 获取ANA统计
func (m *ANAManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	groupStats := make([]map[string]interface{}, 0)
	for _, group := range m.groups {
		group.mu.RLock()
		pathStats := make([]map[string]interface{}, 0)
		for _, path := range group.Paths {
			pathStats = append(pathStats, map[string]interface{}{
				"id":           path.ID,
				"controller":   path.ControllerID,
				"state":        path.State.String(),
				"latency_ms":   path.Latency.Milliseconds(),
				"iops":         path.IOPS,
				"throughput":   path.Throughput,
			})
		}
		groupStats = append(groupStats, map[string]interface{}{
			"id":       group.ID,
			"name":     group.Name,
			"state":    group.State.String(),
			"priority": group.Priority,
			"paths":    pathStats,
		})
		group.mu.RUnlock()
	}
	
	return map[string]interface{}{
		"enabled":     m.running.Load(),
		"groups":      groupStats,
		"total_paths": len(m.paths),
		"policy":      m.config.LoadBalancePolicy,
	}
}