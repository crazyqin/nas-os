// Package ha 故障转移控制器
// 实现 SMB 有状态故障转移，参考 TrueNAS 26 SMB Stateful Failover
package ha

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FailoverController 故障转移控制器.
type FailoverController struct {
	manager    *HAManager
	config     *HAConfig
	state      *FailoverState
	strategies map[string]FailoverStrategy
	hooks      []FailoverHook
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     *zap.Logger
}

// FailoverState 故障转移状态.
type FailoverState struct {
	InProgress     bool             `json:"in_progress"`
	StartTime      time.Time        `json:"start_time"`
	FailedNode     *NodeHAInfo      `json:"failed_node"`
	TargetNode     *NodeHAInfo      `json:"target_node"`
	CurrentPhase   FailoverPhase    `json:"current_phase"`
	AttemptCount   int              `json:"attempt_count"`
	LastFailover   time.Time        `json:"last_failover"`
	TotalFailovers int              `json:"total_failovers"`
	History        []FailoverRecord `json:"history"`
}

// FailoverPhase 故障转移阶段.
type FailoverPhase string

const (
	PhaseDetection    FailoverPhase = "detection"    // 故障检测
	PhaseConfirmation FailoverPhase = "confirmation" // 故障确认
	PhasePreparation  FailoverPhase = "preparation"  // 准备阶段
	PhaseSMBTransfer  FailoverPhase = "smb_transfer" // SMB状态转移
	PhaseTakeover     FailoverPhase = "takeover"     // 接管服务
	PhaseVerification FailoverPhase = "verification" // 验证阶段
	PhaseCleanup      FailoverPhase = "cleanup"      // 清理阶段
	PhaseComplete     FailoverPhase = "complete"     // 完成
	PhaseFailed       FailoverPhase = "failed"       // 失败
)

// FailoverRecord 故障转移记录.
type FailoverRecord struct {
	ID           string        `json:"id"`
	Timestamp    time.Time     `json:"timestamp"`
	Type         string        `json:"type"` // automatic, manual
	FailedNode   string        `json:"failed_node"`
	TargetNode   string        `json:"target_node"`
	Duration     time.Duration `json:"duration"`
	PhaseRecords []PhaseRecord `json:"phase_records"`
	Success      bool          `json:"success"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// PhaseRecord 阶段记录.
type PhaseRecord struct {
	Phase     FailoverPhase `json:"phase"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
}

// FailoverStrategy 故障转移策略接口.
type FailoverStrategy interface {
	Name() string
	SelectTarget(nodes []*NodeHAInfo, failedNode *NodeHAInfo) (*NodeHAInfo, error)
	ValidateTarget(target *NodeHAInfo) error
	PreFailover(failedNode *NodeHAInfo) error
	PostFailover(newPrimary *NodeHAInfo) error
}

// FailoverHook 故障转移钩子接口.
type FailoverHook interface {
	OnFailoverStart(failedNode, targetNode *NodeHAInfo)
	OnPhaseStart(phase FailoverPhase)
	OnPhaseComplete(phase FailoverPhase, duration time.Duration, err error)
	OnFailoverComplete(record *FailoverRecord)
	OnFailoverFailed(record *FailoverRecord)
}

// SMBFailoverStrategy SMB故障转移策略
// 参考 TrueNAS SMB Stateful Failover 实现.
type SMBFailoverStrategy struct {
	policy string
	logger *zap.Logger
}

// NewSMBFailoverStrategy 创建 SMB 故障转移策略.
func NewSMBFailoverStrategy(policy string, logger *zap.Logger) *SMBFailoverStrategy {
	return &SMBFailoverStrategy{
		policy: policy,
		logger: logger,
	}
}

// Name 策略名称.
func (s *SMBFailoverStrategy) Name() string {
	return "smb_stateful"
}

// SelectTarget 选择目标节点.
func (s *SMBFailoverStrategy) SelectTarget(nodes []*NodeHAInfo, failedNode *NodeHAInfo) (*NodeHAInfo, error) {
	var candidates []*NodeHAInfo

	// 筛选可用节点
	for _, node := range nodes {
		if node.ID == failedNode.ID {
			continue
		}
		if node.State == HAStatePassive || node.State == HAStateActive {
			if node.HealthScore >= 50 {
				candidates = append(candidates, node)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, ErrClusterNotReady
	}

	// 根据策略选择
	switch s.policy {
	case "priority":
		// 选择优先级最高的
		var best *NodeHAInfo
		for _, node := range candidates {
			if best == nil || node.Priority > best.Priority {
				best = node
			}
		}
		return best, nil

	case "round-robin":
		// 轮询选择第一个
		return candidates[0], nil

	case "random":
		// 随机选择（简化）
		return candidates[0], nil

	default:
		return candidates[0], nil
	}
}

// ValidateTarget 验证目标节点.
func (s *SMBFailoverStrategy) ValidateTarget(target *NodeHAInfo) error {
	if target == nil {
		return errors.New("target node is nil")
	}

	if target.State != HAStatePassive && target.State != HAStateActive {
		return fmt.Errorf("target node state invalid: %s", target.State)
	}

	if target.HealthScore < 50 {
		return fmt.Errorf("target node health score too low: %.2f", target.HealthScore)
	}

	return nil
}

// PreFailover 故障转移前准备.
func (s *SMBFailoverStrategy) PreFailover(failedNode *NodeHAInfo) error {
	s.logger.Info("Pre-failover preparation",
		zap.String("failed_node", failedNode.ID),
	)

	// 在实际实现中，这里需要：
	// 1. 暂停新的 SMB 连接
	// 2. 开始 SMB 会话状态同步
	// 3. 准备必要的元数据

	return nil
}

// PostFailover 故障转移后处理.
func (s *SMBFailoverStrategy) PostFailover(newPrimary *NodeHAInfo) error {
	s.logger.Info("Post-failover cleanup",
		zap.String("new_primary", newPrimary.ID),
	)

	// 在实际实现中，这里需要：
	// 1. 通知客户端重连
	// 2. 验证 SMB 服务状态
	// 3. 更新 DNS/VIP 配置

	return nil
}

// NewFailoverController 创建故障转移控制器.
func NewFailoverController(manager *HAManager, config *HAConfig, logger *zap.Logger) *FailoverController {
	ctx, cancel := context.WithCancel(context.Background())

	fc := &FailoverController{
		manager: manager,
		config:  config,
		state: &FailoverState{
			History: make([]FailoverRecord, 0, 50),
		},
		strategies: make(map[string]FailoverStrategy),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}

	// 注册默认策略
	fc.strategies["smb_stateful"] = NewSMBFailoverStrategy(config.PriorityPolicy, logger)

	return fc
}

// Start 启动故障转移控制器.
func (fc *FailoverController) Start(ctx context.Context) error {
	fc.ctx = ctx

	// 启动监控循环
	fc.wg.Add(1)
	go fc.monitorLoop()

	fc.logger.Info("Failover controller started",
		zap.Bool("auto_failover", fc.config.FailoverEnabled),
	)

	return nil
}

// Stop 停止故障转移控制器.
func (fc *FailoverController) Stop() {
	fc.cancel()
	fc.wg.Wait()
	fc.logger.Info("Failover controller stopped")
}

// monitorLoop 监控循环.
func (fc *FailoverController) monitorLoop() {
	defer fc.wg.Done()

	ticker := time.NewTicker(fc.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fc.ctx.Done():
			return
		case <-ticker.C:
			fc.checkForFailover()
		}
	}
}

// checkForFailover 检查是否需要故障转移.
func (fc *FailoverController) checkForFailover() {
	if !fc.config.FailoverEnabled {
		return
	}

	fc.mu.RLock()
	if fc.state.InProgress {
		fc.mu.RUnlock()
		return
	}
	fc.mu.RUnlock()

	// 检查主节点状态
	primary := fc.manager.GetPrimary()
	if primary == nil {
		// 无主节点，触发选举
		fc.logger.Info("No primary node, triggering election")
		return
	}

	// 使用 Phi 检测器判断
	if !fc.manager.heartbeatMgr.IsNodeHealthy(primary.ID) {
		fc.logger.Warn("Primary node unhealthy detected",
			zap.String("primary_id", primary.ID),
			zap.Float64("phi", fc.manager.heartbeatMgr.Phi(primary.ID)),
		)
		fc.TriggerFailover(primary)
	}
}

// TriggerFailover 触发故障转移.
func (fc *FailoverController) TriggerFailover(failedNode *NodeHAInfo) error {
	fc.mu.Lock()

	// 检查是否已经在进行中
	if fc.state.InProgress {
		fc.mu.Unlock()
		return ErrFailoverInProgress
	}

	fc.state.InProgress = true
	fc.state.StartTime = time.Now()
	fc.state.FailedNode = failedNode
	fc.state.CurrentPhase = PhaseDetection
	fc.state.AttemptCount++

	fc.mu.Unlock()

	// 异步执行
	go fc.executeFailover(failedNode, "automatic")

	return nil
}

// ExecuteManualFailover 执行手动故障转移.
func (fc *FailoverController) ExecuteManualFailover(target *NodeHAInfo) error {
	fc.mu.Lock()

	if fc.state.InProgress {
		fc.mu.Unlock()
		return ErrFailoverInProgress
	}

	// 获取当前主节点
	primary := fc.manager.GetPrimary()
	if primary == nil {
		fc.mu.Unlock()
		return ErrClusterNotReady
	}

	fc.state.InProgress = true
	fc.state.StartTime = time.Now()
	fc.state.FailedNode = primary
	fc.state.TargetNode = target
	fc.state.CurrentPhase = PhasePreparation

	fc.mu.Unlock()

	// 执行
	return fc.executeFailover(primary, "manual")
}

// executeFailover 执行故障转移.
func (fc *FailoverController) executeFailover(failedNode *NodeHAInfo, failoverType string) error {
	record := &FailoverRecord{
		ID:           fmt.Sprintf("failover-%d", time.Now().UnixNano()),
		Timestamp:    time.Now(),
		Type:         failoverType,
		FailedNode:   failedNode.ID,
		PhaseRecords: make([]PhaseRecord, 0),
	}

	var targetNode *NodeHAInfo
	var err error

	// 调用钩子
	fc.callHooksStart(failedNode, targetNode)

	// 阶段1: 故障检测确认
	phaseStart := time.Now()
	fc.setCurrentPhase(PhaseConfirmation)
	fc.callHooksPhaseStart(PhaseConfirmation)

	// 等待确认时间
	time.Sleep(fc.config.FailoverConfirmation)

	// 重新检查节点状态
	if fc.manager.heartbeatMgr.IsNodeHealthy(failedNode.ID) {
		// 节点恢复，取消故障转移
		fc.logger.Info("Node recovered, canceling failover",
			zap.String("node_id", failedNode.ID),
		)
		fc.cancelFailover()
		return nil
	}

	fc.recordPhase(record, PhaseConfirmation, phaseStart, nil)

	// 阶段2: 选择目标节点
	phaseStart = time.Now()
	fc.setCurrentPhase(PhasePreparation)
	fc.callHooksPhaseStart(PhasePreparation)

	nodes := fc.manager.GetNodes()

	strategy := fc.strategies["smb_stateful"]

	if targetNode == nil {
		targetNode, err = strategy.SelectTarget(nodes, failedNode)
		if err != nil {
			fc.handleFailoverFailed(record, err)
			return err
		}
	}

	// 验证目标
	if err := strategy.ValidateTarget(targetNode); err != nil {
		fc.handleFailoverFailed(record, err)
		return err
	}

	fc.state.TargetNode = targetNode
	fc.recordPhase(record, PhasePreparation, phaseStart, nil)

	// 阶段3: 预故障转移
	phaseStart = time.Now()
	fc.setCurrentPhase(PhaseSMBTransfer)
	fc.callHooksPhaseStart(PhaseSMBTransfer)

	if err := strategy.PreFailover(failedNode); err != nil {
		fc.handleFailoverFailed(record, err)
		return err
	}

	// SMB 状态同步（这里是模拟，实际需要调用 SMB 模块）
	fc.smbStateSync(failedNode, targetNode)

	fc.recordPhase(record, PhaseSMBTransfer, phaseStart, nil)

	// 阶段4: 服务接管
	phaseStart = time.Now()
	fc.setCurrentPhase(PhaseTakeover)
	fc.callHooksPhaseStart(PhaseTakeover)

	// 执行角色切换
	if err := fc.promoteNode(targetNode); err != nil {
		fc.handleFailoverFailed(record, err)
		return err
	}

	fc.recordPhase(record, PhaseTakeover, phaseStart, nil)

	// 阶段5: 验证
	phaseStart = time.Now()
	fc.setCurrentPhase(PhaseVerification)
	fc.callHooksPhaseStart(PhaseVerification)

	// 验证服务状态
	if err := fc.verifyTakeover(targetNode); err != nil {
		fc.handleFailoverFailed(record, err)
		return err
	}

	fc.recordPhase(record, PhaseVerification, phaseStart, nil)

	// 阶段6: 后处理
	phaseStart = time.Now()
	fc.setCurrentPhase(PhaseCleanup)
	fc.callHooksPhaseStart(PhaseCleanup)

	if err := strategy.PostFailover(targetNode); err != nil {
		fc.logger.Warn("Post-failover hook failed", zap.Error(err))
	}

	fc.recordPhase(record, PhaseCleanup, phaseStart, nil)

	// 完成
	record.Duration = time.Since(record.Timestamp)
	record.TargetNode = targetNode.ID
	record.Success = true

	fc.mu.Lock()
	fc.state.InProgress = false
	fc.state.LastFailover = time.Now()
	fc.state.TotalFailovers++
	fc.state.History = append(fc.state.History, *record)
	if len(fc.state.History) > 50 {
		fc.state.History = fc.state.History[len(fc.state.History)-50:]
	}
	fc.mu.Unlock()

	fc.setCurrentPhase(PhaseComplete)
	fc.callHooksComplete(record)

	fc.logger.Info("Failover completed successfully",
		zap.String("from", failedNode.ID),
		zap.String("to", targetNode.ID),
		zap.Duration("duration", record.Duration),
	)

	return nil
}

// smbStateSync SMB状态同步
// 参考 TrueNAS SMB Stateful Failover 实现.
func (fc *FailoverController) smbStateSync(failedNode, targetNode *NodeHAInfo) {
	fc.logger.Info("SMB state synchronization",
		zap.String("from", failedNode.ID),
		zap.String("to", targetNode.ID),
	)

	// 在实际实现中需要：
	// 1. 同步 SMB 配置文件 (smb.conf)
	// 2. 同步用户数据库 (passdb.tdb)
	// 3. 同步会话状态 (session.tdb)
	// 4. 同步锁定状态 (locking.tdb)
	// 5. 同步共享状态

	// 这里使用模拟实现
	time.Sleep(1 * time.Second)
}

// promoteNode 提升节点为主节点.
func (fc *FailoverController) promoteNode(node *NodeHAInfo) error {
	// 更新节点角色
	node.Role = HARolePrimary
	node.State = HAStateActive

	// 更新其他节点角色
	for _, n := range fc.manager.GetNodes() {
		if n.ID != node.ID {
			n.Role = HARoleSecondary
			if n.State != HAStateFailed {
				n.State = HAStatePassive
			}
		}
	}

	fc.manager.mu.Lock()
	fc.manager.primary = node
	fc.manager.mu.Unlock()

	fc.logger.Info("Node promoted to primary",
		zap.String("node_id", node.ID),
	)

	return nil
}

// verifyTakeover 验证接管.
func (fc *FailoverController) verifyTakeover(node *NodeHAInfo) error {
	// 检查节点状态
	if node.State != HAStateActive {
		return fmt.Errorf("node state not active: %s", node.State)
	}

	// 检查健康分数
	if node.HealthScore < 80 {
		return fmt.Errorf("node health score low: %.2f", node.HealthScore)
	}

	fc.logger.Info("Takeover verified",
		zap.String("node_id", node.ID),
	)

	return nil
}

// handleFailoverFailed 处理故障转移失败.
func (fc *FailoverController) handleFailoverFailed(record *FailoverRecord, err error) {
	record.Success = false
	record.ErrorMessage = err.Error()
	record.Duration = time.Since(record.Timestamp)

	fc.mu.Lock()
	fc.state.InProgress = false
	fc.state.History = append(fc.state.History, *record)
	fc.mu.Unlock()

	fc.setCurrentPhase(PhaseFailed)
	fc.callHooksFailed(record)

	fc.logger.Error("Failover failed",
		zap.Error(err),
		zap.Int("attempt", fc.state.AttemptCount),
	)
}

// cancelFailover 取消故障转移.
func (fc *FailoverController) cancelFailover() {
	fc.mu.Lock()
	fc.state.InProgress = false
	fc.state.FailedNode = nil
	fc.state.TargetNode = nil
	fc.state.CurrentPhase = ""
	fc.mu.Unlock()

	fc.logger.Info("Failover canceled")
}

// setCurrentPhase 设置当前阶段.
func (fc *FailoverController) setCurrentPhase(phase FailoverPhase) {
	fc.mu.Lock()
	fc.state.CurrentPhase = phase
	fc.mu.Unlock()
}

// recordPhase 记录阶段.
func (fc *FailoverController) recordPhase(record *FailoverRecord, phase FailoverPhase, start time.Time, err error) {
	now := time.Now()
	pr := PhaseRecord{
		Phase:     phase,
		StartTime: start,
		EndTime:   now,
		Duration:  now.Sub(start),
		Status:    "success",
	}

	if err != nil {
		pr.Status = "failed"
		pr.Message = err.Error()
	}

	record.PhaseRecords = append(record.PhaseRecords, pr)

	fc.callHooksPhaseComplete(phase, pr.Duration, err)
}

// 钩子调用.
func (fc *FailoverController) callHooksStart(failedNode, targetNode *NodeHAInfo) {
	for _, hook := range fc.hooks {
		go hook.OnFailoverStart(failedNode, targetNode)
	}
}

func (fc *FailoverController) callHooksPhaseStart(phase FailoverPhase) {
	for _, hook := range fc.hooks {
		go hook.OnPhaseStart(phase)
	}
}

func (fc *FailoverController) callHooksPhaseComplete(phase FailoverPhase, duration time.Duration, err error) {
	for _, hook := range fc.hooks {
		go hook.OnPhaseComplete(phase, duration, err)
	}
}

func (fc *FailoverController) callHooksComplete(record *FailoverRecord) {
	for _, hook := range fc.hooks {
		go hook.OnFailoverComplete(record)
	}
}

func (fc *FailoverController) callHooksFailed(record *FailoverRecord) {
	for _, hook := range fc.hooks {
		go hook.OnFailoverFailed(record)
	}
}

// RegisterStrategy 注册策略.
func (fc *FailoverController) RegisterStrategy(name string, strategy FailoverStrategy) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.strategies[name] = strategy
}

// RegisterHook 注册钩子.
func (fc *FailoverController) RegisterHook(hook FailoverHook) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.hooks = append(fc.hooks, hook)
}

// GetState 获取故障转移状态.
func (fc *FailoverController) GetState() *FailoverState {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.state
}

// LastFailover 最后故障转移时间.
func (fc *FailoverController) LastFailover() time.Time {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.state.LastFailover
}

// FailoverCount 故障转移次数.
func (fc *FailoverController) FailoverCount() int {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.state.TotalFailovers
}

// GetHistory 获取故障转移历史.
func (fc *FailoverController) GetHistory(limit int) []FailoverRecord {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	if limit <= 0 || limit > len(fc.state.History) {
		limit = len(fc.state.History)
	}

	start := len(fc.state.History) - limit
	if start < 0 {
		start = 0
	}

	return fc.state.History[start:]
}
