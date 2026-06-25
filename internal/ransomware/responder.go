// Package ransomshield - 自动响应机制
// 多级响应策略、进程隔离、网络阻断、快照保护、自动化编排
package ransomware

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 自动响应引擎
// ============================================================

// Responder 自动响应引擎
type Responder struct {
	mu sync.RWMutex

	// policies 响应策略 (level -> []ResponsePolicy)
	policies map[ThreatLevel][]ResponsePolicy

	// actions 执行中的动作
	activeActions map[string]*ResponseAction

	// actionLog 动作日志
	actionLog []ResponseAction

	// quarantineDir 隔离目录
	quarantineDir string

	// blockedIPs 已阻断的 IP
	blockedIPs map[string]time.Time

	// blockedProcesses 已阻断的进程
	blockedProcesses map[int]BlockedProcess

	// networkRules 网络阻断规则
	networkRules []NetworkRule

	// callbacks 回调函数
	onSnapshot     func(path string) (string, error)
	onIsolate      func(pid int) error
	onNetworkBlock func(ip string, port int) error

	// stats 统计
	stats ResponseStats

	// running 运行状态
	running  bool
	stopChan chan struct{}
}

// ResponsePolicy 响应策略
type ResponsePolicy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Level       ThreatLevel  `json:"level"`
	Actions     []ActionType `json:"actions"`
	Priority    int          `json:"priority"`
	CooldownSec int          `json:"cooldown_sec"`
	Enabled     bool         `json:"enabled"`
}

// ResponseAction 响应动作记录
type ResponseAction struct {
	ID         string     `json:"id"`
	Type       ActionType `json:"type"`
	ThreatID   string     `json:"threat_id"`
	Target     string     `json:"target"`
	Status     string     `json:"status"` // pending, executing, completed, failed
	Result     string     `json:"result"`
	Error      string     `json:"error,omitempty"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	DurationMs int64      `json:"duration_ms"`
}

// BlockedProcess 已阻断的进程
type BlockedProcess struct {
	PID         int        `json:"pid"`
	Name        string     `json:"name"`
	BlockedAt   time.Time  `json:"blocked_at"`
	Reason      string     `json:"reason"`
	ThreatID    string     `json:"threat_id"`
	AutoUnblock bool       `json:"auto_unblock"`
	UnblockAt   *time.Time `json:"unblock_at,omitempty"`
}

// NetworkRule 网络阻断规则
type NetworkRule struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"` // block-ip, block-port, block-process
	Target    string     `json:"target"`
	Port      int        `json:"port,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// ResponseStats 响应统计
type ResponseStats struct {
	TotalActions      int64     `json:"total_actions"`
	SuccessfulActions int64     `json:"successful_actions"`
	FailedActions     int64     `json:"failed_actions"`
	SnapshotsTaken    int64     `json:"snapshots_taken"`
	ProcessesKilled   int64     `json:"processes_killed"`
	IPsBlocked        int64     `json:"ips_blocked"`
	FilesQuarantined  int64     `json:"files_quarantined"`
	LastResponseTime  time.Time `json:"last_response_time"`
	AvgResponseMs     int64     `json:"avg_response_ms"`
}

// ============================================================
// 构造与生命周期
// ============================================================

// NewResponder 创建自动响应引擎
func NewResponder(quarantineDir string) *Responder {
	r := &Responder{
		policies:         make(map[ThreatLevel][]ResponsePolicy),
		activeActions:    make(map[string]*ResponseAction),
		actionLog:        make([]ResponseAction, 0, 5000),
		quarantineDir:    quarantineDir,
		blockedIPs:       make(map[string]time.Time),
		blockedProcesses: make(map[int]BlockedProcess),
		stopChan:         make(chan struct{}),
	}

	r.initDefaultPolicies()
	return r
}

// initDefaultPolicies 初始化默认响应策略
func (r *Responder) initDefaultPolicies() {
	r.policies[ThreatLevelCritical] = []ResponsePolicy{
		{
			ID: "critical-auto", Name: "严重威胁自动响应", Level: ThreatLevelCritical,
			Actions:  []ActionType{ActionTypeSnapshot, ActionTypeKillProcess, ActionTypeQuarantine, ActionTypeLockdown},
			Priority: 1, CooldownSec: 10, Enabled: true,
		},
	}
	r.policies[ThreatLevelHigh] = []ResponsePolicy{
		{
			ID: "high-auto", Name: "高威胁自动响应", Level: ThreatLevelHigh,
			Actions:  []ActionType{ActionTypeSnapshot, ActionTypeBlock, ActionTypeAlert},
			Priority: 2, CooldownSec: 30, Enabled: true,
		},
	}
	r.policies[ThreatLevelMedium] = []ResponsePolicy{
		{
			ID: "medium-auto", Name: "中等威胁自动响应", Level: ThreatLevelMedium,
			Actions:  []ActionType{ActionTypeSnapshot, ActionTypeAlert},
			Priority: 3, CooldownSec: 60, Enabled: true,
		},
	}
	r.policies[ThreatLevelLow] = []ResponsePolicy{
		{
			ID: "low-auto", Name: "低威胁自动响应", Level: ThreatLevelLow,
			Actions:  []ActionType{ActionTypeAlert},
			Priority: 4, CooldownSec: 300, Enabled: true,
		},
	}
}

// SetSnapshotCallback 设置快照回调
func (r *Responder) SetSnapshotCallback(fn func(path string) (string, error)) {
	r.mu.Lock()
	r.onSnapshot = fn
	r.mu.Unlock()
}

// SetIsolateCallback 设置进程隔离回调
func (r *Responder) SetIsolateCallback(fn func(pid int) error) {
	r.mu.Lock()
	r.onIsolate = fn
	r.mu.Unlock()
}

// SetNetworkBlockCallback 设置网络阻断回调
func (r *Responder) SetNetworkBlockCallback(fn func(ip string, port int) error) {
	r.mu.Lock()
	r.onNetworkBlock = fn
	r.mu.Unlock()
}

// Start 启动响应引擎
func (r *Responder) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	// 创建隔离目录
	os.MkdirAll(r.quarantineDir, 0700)

	go r.cleanupLoop()
	log.Println("[Responder] 自动响应引擎已启动")
}

// Stop 停止响应引擎
func (r *Responder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	close(r.stopChan)
	r.running = false
	log.Println("[Responder] 自动响应引擎已停止")
}

// ============================================================
// 响应执行
// ============================================================

// HandleThreat 处理威胁事件，执行自动响应
func (r *Responder) HandleThreat(event ThreatEvent) []ResponseAction {
	r.mu.RLock()
	policies := r.policies[event.Level]
	r.mu.RUnlock()

	var actions []ResponseAction

	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		for _, actionType := range policy.Actions {
			action := r.executeAction(actionType, event)
			actions = append(actions, action)
		}
	}

	// 记录统计
	r.mu.Lock()
	r.stats.TotalActions += int64(len(actions))
	r.stats.LastResponseTime = time.Now()
	r.mu.Unlock()

	return actions
}

// executeAction 执行单个响应动作
func (r *Responder) executeAction(actionType ActionType, event ThreatEvent) ResponseAction {
	startTime := time.Now()
	action := ResponseAction{
		ID:        uuid.New().String(),
		Type:      actionType,
		ThreatID:  event.ID,
		Target:    event.SourcePath,
		Status:    "executing",
		StartTime: startTime,
	}

	r.mu.Lock()
	r.activeActions[action.ID] = &action
	r.mu.Unlock()

	var err error
	switch actionType {
	case ActionTypeSnapshot:
		err = r.executeSnapshot(event)
	case ActionTypeKillProcess:
		err = r.executeKillProcess(event)
	case ActionTypeQuarantine:
		err = r.executeQuarantine(event)
	case ActionTypeBlock:
		err = r.executeBlock(event)
	case ActionTypeLockdown:
		err = r.executeLockdown(event)
	case ActionTypeAlert:
		err = r.executeAlert(event)
	default:
		err = fmt.Errorf("未知动作类型: %s", actionType)
	}

	endTime := time.Now()
	action.EndTime = &endTime
	action.DurationMs = endTime.Sub(startTime).Milliseconds()

	if err != nil {
		action.Status = "failed"
		action.Error = err.Error()
		r.mu.Lock()
		r.stats.FailedActions++
		r.mu.Unlock()
	} else {
		action.Status = "completed"
		r.mu.Lock()
		r.stats.SuccessfulActions++
		r.mu.Unlock()
	}

	// 记录日志
	r.mu.Lock()
	r.actionLog = append(r.actionLog, action)
	delete(r.activeActions, action.ID)
	if len(r.actionLog) > 10000 {
		r.actionLog = r.actionLog[1:]
	}
	r.mu.Unlock()

	log.Printf("[Responder] 执行动作: 类型=%s, 目标=%s, 状态=%s, 耗时=%dms",
		actionType, event.SourcePath, action.Status, action.DurationMs)

	return action
}

// ============================================================
// 具体响应动作
// ============================================================

// executeSnapshot 执行快照
func (r *Responder) executeSnapshot(event ThreatEvent) error {
	dir := filepath.Dir(event.SourcePath)

	if r.onSnapshot != nil {
		snapshotID, err := r.onSnapshot(dir)
		if err != nil {
			return fmt.Errorf("快照创建失败: %w", err)
		}
		r.mu.Lock()
		r.stats.SnapshotsTaken++
		r.mu.Unlock()
		log.Printf("[Responder] 快照已创建: %s (路径: %s)", snapshotID, dir)
		return nil
	}

	log.Printf("[Responder] 快照回调未设置，跳过快照: %s", dir)
	return nil
}

// executeKillProcess 执行进程终止
func (r *Responder) executeKillProcess(event ThreatEvent) error {
	if event.ProcessID <= 0 {
		return fmt.Errorf("无效的进程ID: %d", event.ProcessID)
	}

	// 先尝试 SIGTERM，再 SIGKILL
	p, err := os.FindProcess(event.ProcessID)
	if err != nil {
		return fmt.Errorf("查找进程失败: %w", err)
	}

	if err := p.Signal(os.Interrupt); err != nil {
		// SIGTERM 失败，使用 SIGKILL
		if err := p.Kill(); err != nil {
			return fmt.Errorf("终止进程失败: %w", err)
		}
	}

	r.mu.Lock()
	r.stats.ProcessesKilled++
	r.blockedProcesses[event.ProcessID] = BlockedProcess{
		PID:       event.ProcessID,
		Name:      event.ProcessName,
		BlockedAt: time.Now(),
		Reason:    fmt.Sprintf("勒索威胁响应 (事件: %s)", event.ID),
		ThreatID:  event.ID,
	}
	r.mu.Unlock()

	log.Printf("[Responder] 进程已终止: PID=%d, Name=%s", event.ProcessID, event.ProcessName)
	return nil
}

// executeQuarantine 执行文件隔离
func (r *Responder) executeQuarantine(event ThreatEvent) error {
	if event.SourcePath == "" {
		return fmt.Errorf("源路径为空")
	}

	// 检查源文件是否存在
	if _, err := os.Stat(event.SourcePath); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", event.SourcePath)
	}

	// 生成隔离路径
	quarantineName := fmt.Sprintf("%s_%s_%s",
		uuid.New().String()[:8],
		time.Now().Format("20060102_150405"),
		filepath.Base(event.SourcePath),
	)
	destPath := filepath.Join(r.quarantineDir, quarantineName)

	// 移动文件到隔离目录
	if err := os.Rename(event.SourcePath, destPath); err != nil {
		// 跨分区时 rename 可能失败，改用复制+删除
		return r.copyAndRemove(event.SourcePath, destPath)
	}

	// 设置只读权限
	os.Chmod(destPath, 0400)

	// 记录原始路径元数据
	metaPath := destPath + ".meta"
	meta := fmt.Sprintf("original_path: %s\nquarantine_time: %s\nthreat_id: %s\nprocess: %s(%d)\n",
		event.SourcePath, time.Now().Format(time.RFC3339), event.ID, event.ProcessName, event.ProcessID)
	os.WriteFile(metaPath, []byte(meta), 0400)

	r.mu.Lock()
	r.stats.FilesQuarantined++
	r.mu.Unlock()

	log.Printf("[Responder] 文件已隔离: %s -> %s", event.SourcePath, destPath)
	return nil
}

// copyAndRemove 复制并删除文件（跨分区隔离）
func (r *Responder) copyAndRemove(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	buf := make([]byte, 64*1024)
	for {
		n, readErr := srcFile.Read(buf)
		if n > 0 {
			if _, writeErr := dstFile.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if readErr != nil {
			break
		}
	}

	os.Remove(src)
	return nil
}

// executeBlock 执行阻断（进程阻断）
func (r *Responder) executeBlock(event ThreatEvent) error {
	if event.ProcessID <= 0 {
		return fmt.Errorf("无效的进程ID")
	}

	// 使用 cgroup 或 freezer 阻断进程
	if r.onIsolate != nil {
		if err := r.onIsolate(event.ProcessID); err != nil {
			log.Printf("[Responder] 进程隔离失败: %v", err)
		}
	}

	// 记录阻断
	r.mu.Lock()
	r.blockedProcesses[event.ProcessID] = BlockedProcess{
		PID:         event.ProcessID,
		Name:        event.ProcessName,
		BlockedAt:   time.Now(),
		Reason:      "威胁阻断",
		ThreatID:    event.ID,
		AutoUnblock: true,
	}
	r.mu.Unlock()

	return nil
}

// executeLockdown 执行系统锁定
func (r *Responder) executeLockdown(event ThreatEvent) error {
	log.Printf("[Responder] 执行系统锁定: 威胁=%s, 级别=%s", event.ID, event.Level.String())

	// 1. 禁用 SMB/NFS 共享
	r.disableNetworkShares()

	// 2. 断开所有非管理会话
	r.disconnectNonAdminSessions()

	// 3. 启用只读模式
	r.enableReadOnlyMode(event.SourcePath)

	return nil
}

// executeAlert 执行告警
func (r *Responder) executeAlert(event ThreatEvent) error {
	log.Printf("[Responder] 告警: 威胁=%s, 级别=%s, 路径=%s, 进程=%s",
		event.ID, event.Level.String(), event.SourcePath, event.ProcessName)
	return nil
}

// ============================================================
// 网络隔离
// ============================================================

// BlockIP 阻断 IP 地址
func (r *Responder) BlockIP(ip string, duration time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.onNetworkBlock != nil {
		if err := r.onNetworkBlock(ip, 0); err != nil {
			return err
		}
	}

	var expires *time.Time
	if duration > 0 {
		t := time.Now().Add(duration)
		expires = &t
	}

	r.blockedIPs[ip] = time.Now()
	r.networkRules = append(r.networkRules, NetworkRule{
		ID:        uuid.New().String(),
		Type:      "block-ip",
		Target:    ip,
		CreatedAt: time.Now(),
		ExpiresAt: expires,
	})
	r.stats.IPsBlocked++

	log.Printf("[Responder] IP已阻断: %s (时长: %v)", ip, duration)
	return nil
}

// UnblockIP 解除 IP 阻断
func (r *Responder) UnblockIP(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.blockedIPs, ip)
	log.Printf("[Responder] IP已解除阻断: %s", ip)
}

// IsIPBlocked 检查 IP 是否被阻断
func (r *Responder) IsIPBlocked(ip string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, blocked := r.blockedIPs[ip]
	return blocked
}

// ============================================================
// 辅助操作
// ============================================================

// disableNetworkShares 禁用网络共享
func (r *Responder) disableNetworkShares() {
	// 停止 SMB 服务
	if err := exec.Command("systemctl", "stop", "smbd").Run(); err != nil {
		log.Printf("[Responder] 停止SMB失败: %v", err)
	}
	// 停止 NFS 服务
	if err := exec.Command("systemctl", "stop", "nfs-kernel-server").Run(); err != nil {
		log.Printf("[Responder] 停止NFS失败: %v", err)
	}
	log.Println("[Responder] 网络共享已禁用")
}

// disconnectNonAdminSessions 断开非管理员会话
func (r *Responder) disconnectNonAdminSessions() {
	log.Println("[Responder] 断开非管理员会话（TODO: 与SMB/NFS集成）")
}

// enableReadOnlyMode 启用只读模式
func (r *Responder) enableReadOnlyMode(path string) {
	dir := filepath.Dir(path)
	// 尝试设置目录只读
	if err := os.Chmod(dir, 0555); err != nil {
		log.Printf("[Responder] 设置只读模式失败: %s, %v", dir, err)
	} else {
		log.Printf("[Responder] 已启用只读模式: %s", dir)
	}
}

// ============================================================
// 查询接口
// ============================================================

// GetActionLog 获取动作日志
func (r *Responder) GetActionLog(limit int) []ResponseAction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > len(r.actionLog) {
		limit = len(r.actionLog)
	}

	start := len(r.actionLog) - limit
	result := make([]ResponseAction, limit)
	copy(result, r.actionLog[start:])
	return result
}

// GetBlockedProcesses 获取已阻断的进程
func (r *Responder) GetBlockedProcesses() []BlockedProcess {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]BlockedProcess, 0, len(r.blockedProcesses))
	for _, bp := range r.blockedProcesses {
		result = append(result, bp)
	}
	return result
}

// GetStats 获取响应统计
func (r *Responder) GetStats() ResponseStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// GetNetworkRules 获取网络规则
func (r *Responder) GetNetworkRules() []NetworkRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]NetworkRule, len(r.networkRules))
	copy(result, r.networkRules)
	return result
}

// ============================================================
// 清理循环
// ============================================================

// cleanupLoop 清理过期的阻断和规则
func (r *Responder) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.cleanup()
		}
	}
}

// cleanup 清理过期数据
func (r *Responder) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// 清理过期的网络规则
	var activeRules []NetworkRule
	for _, rule := range r.networkRules {
		if rule.ExpiresAt != nil && now.After(*rule.ExpiresAt) {
			// 解除阻断
			if rule.Type == "block-ip" {
				delete(r.blockedIPs, rule.Target)
			}
			continue
		}
		activeRules = append(activeRules, rule)
	}
	r.networkRules = activeRules

	// 清理长时间阻断的进程（超过 MaxBlockDurationSec）
	for pid, bp := range r.blockedProcesses {
		if bp.AutoUnblock && now.Sub(bp.BlockedAt) > 5*time.Minute {
			delete(r.blockedProcesses, pid)
		}
	}
}

// AddPolicy 添加响应策略
func (r *Responder) AddPolicy(policy ResponsePolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policies[policy.Level] = append(r.policies[policy.Level], policy)
}

// GetPolicies 获取所有响应策略
func (r *Responder) GetPolicies() map[ThreatLevel][]ResponsePolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[ThreatLevel][]ResponsePolicy)
	for level, policies := range r.policies {
		copied := make([]ResponsePolicy, len(policies))
		copy(copied, policies)
		result[level] = copied
	}
	return result
}

// containsStr 字符串包含检查
func containsStr(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}
