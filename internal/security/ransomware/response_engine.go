package ransomware

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 响应级别定义 ==========

// ResponseLevel 响应级别.
type ResponseLevel int

const (
	// ResponseLevelAlert 告警通知（Level 1）：检测到可疑行为时发出告警.
	ResponseLevelAlert ResponseLevel = 1
	// ResponseLevelProcessIsolation 进程隔离（Level 2）：终止可疑进程.
	ResponseLevelProcessIsolation ResponseLevel = 2
	// ResponseLevelSnapshot 快照保护（Level 3）：触发btrfs快照保护数据.
	ResponseLevelSnapshot ResponseLevel = 3
	// ResponseLevelNetworkIsolation 网络阻断（Level 4）：隔离受影响网段.
	ResponseLevelNetworkIsolation ResponseLevel = 4
)

// String 返回响应级别的中文描述.
func (l ResponseLevel) String() string {
	switch l {
	case ResponseLevelAlert:
		return "告警通知"
	case ResponseLevelProcessIsolation:
		return "进程隔离"
	case ResponseLevelSnapshot:
		return "快照保护"
	case ResponseLevelNetworkIsolation:
		return "网络阻断"
	default:
		return "未知级别"
	}
}

// ========== 响应动作 ==========

// ResponseAction 响应动作，记录单次响应操作的详情.
type ResponseAction struct {
	ID          string        `json:"id"`
	Timestamp   time.Time     `json:"timestamp"`
	Level       ResponseLevel `json:"level"`
	ActionType  string        `json:"action_type"`  // 动作类型（如 alert, kill_process, snapshot, isolate）
	Target      string        `json:"target"`        // 操作目标（进程名、网段等）
	Success     bool          `json:"success"`       // 是否成功
	Error       string        `json:"error,omitempty"` // 错误信息
	DetectionID string        `json:"detection_id"`  // 关联的检测ID
	Details     string        `json:"details"`       // 附加详情
}

// ========== 响应配置 ==========

// ResponseConfig 响应引擎配置.
type ResponseConfig struct {
	Enabled               bool          `json:"enabled"`                 // 是否启用响应引擎
	AutoEscalate          bool          `json:"auto_escalate"`           // 是否自动升级响应级别
	MaxLevel              ResponseLevel `json:"max_level"`               // 允许的最大响应级别
	EscalateDelay         time.Duration `json:"escalate_delay"`          // 升级延迟
	ConfidenceThreshold   float64       `json:"confidence_threshold"`    // 置信度阈值（低于此值仅告警）
	ProcessKillTimeout    time.Duration `json:"process_kill_timeout"`    // 进程终止超时
	SnapshotPrefix        string        `json:"snapshot_prefix"`         // 快照名称前缀
	NetworkIsolationCIDRs []string      `json:"network_isolation_cidrs"` // 允许隔离的网段白名单
	NotifyChannel         chan<- ResponseAction `json:"-"`              // 通知通道
}

// DefaultResponseConfig 返回默认响应配置.
func DefaultResponseConfig() ResponseConfig {
	return ResponseConfig{
		Enabled:             true,
		AutoEscalate:        true,
		MaxLevel:            ResponseLevelNetworkIsolation,
		EscalateDelay:       30 * time.Second,
		ConfidenceThreshold: 0.6,
		ProcessKillTimeout:  10 * time.Second,
		SnapshotPrefix:      "ransomware-protection",
		NetworkIsolationCIDRs: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		},
	}
}

// ========== 依赖接口 ==========

// ThreatDetector 威胁检测器接口，用于获取当前威胁状态.
type ThreatDetector interface {
	// EvaluateThreat 评估给定检测结果的威胁等级.
	EvaluateThreat(result *DetectionResult) ThreatLevel
	// GetActiveThreats 获取当前活跃的威胁列表.
	GetActiveThreats() []*DetectionResult
}

// SnapshotCreator 快照创建器接口，用于创建btrfs快照.
type SnapshotCreator interface {
	// CreateSnapshot 创建btrfs快照，返回快照名称和错误.
	CreateSnapshot(name string, subvolume string) (string, error)
	// ListSnapshots 列出指定子卷的快照.
	ListSnapshots(subvolume string) ([]string, error)
}

// NetworkIsolator 网络隔离器接口，用于隔离受影响网段.
type NetworkIsolator interface {
	// IsolateCIDR 隔离指定CIDR网段.
	IsolateCIDR(cidr string) error
	// RestoreCIDR 恢复指定CIDR网段.
	RestoreCIDR(cidr string) error
	// GetIsolatedCIDRs 获取当前被隔离的网段.
	GetIsolatedCIDRs() []string
}

// ProcessKiller 进程终止器接口，用于终止可疑进程.
type ProcessKiller interface {
	// KillProcess 终止指定PID的进程.
	KillProcess(pid int) error
	// KillProcessByName 按名称终止进程.
	KillProcessByName(name string) error
	// IsProcessRunning 检查进程是否仍在运行.
	IsProcessRunning(pid int) bool
}

// ========== 响应引擎 ==========

// ResponseEngine 勒索软件自动化响应引擎，根据威胁等级执行多级响应策略.
type ResponseEngine struct {
	config    ResponseConfig
	detector  ThreatDetector
	snapshot  SnapshotCreator
	isolator  NetworkIsolator
	killer    ProcessKiller
	alertMgr  *AlertManager
	actions   []*ResponseAction
	actionMu  sync.RWMutex
	stats     ResponseStats
	statsMu   sync.RWMutex
	running   bool
	runningMu sync.RWMutex
}

// ResponseStats 响应引擎统计信息.
type ResponseStats struct {
	TotalResponses    int64                `json:"total_responses"`
	ByLevel           map[ResponseLevel]int64 `json:"by_level"`
	SuccessCount      int64                `json:"success_count"`
	FailureCount      int64                `json:"failure_count"`
	LastResponseTime  *time.Time           `json:"last_response_time,omitempty"`
	IsolatedCIDRs     []string             `json:"isolated_cidrs"`
	ActiveSnapshots   []string             `json:"active_snapshots"`
}

// ========== 构造函数 ==========

// NewResponseEngine 创建响应引擎.
func NewResponseEngine(
	config ResponseConfig,
	detector ThreatDetector,
	snapshot SnapshotCreator,
	isolator NetworkIsolator,
	killer ProcessKiller,
	alertMgr *AlertManager,
) *ResponseEngine {
	return &ResponseEngine{
		config:   config,
		detector: detector,
		snapshot: snapshot,
		isolator: isolator,
		killer:   killer,
		alertMgr: alertMgr,
		actions:  make([]*ResponseAction, 0),
		stats: ResponseStats{
			ByLevel: make(map[ResponseLevel]int64),
		},
	}
}

// ========== 核心方法 ==========

// HandleThreat 处理威胁，根据检测结果评估响应级别并执行响应.
func (re *ResponseEngine) HandleThreat(result *DetectionResult) ([]*ResponseAction, error) {
	if !re.config.Enabled {
		return nil, nil
	}

	// 评估响应级别
	level := re.evaluateResponseLevel(result)

	// 执行响应并收集所有动作
	var allActions []*ResponseAction

	// 从评估的级别开始，逐步执行到该级别
	for l := ResponseLevelAlert; l <= level; l++ {
		// 检查是否超过最大允许级别
		if l > re.config.MaxLevel {
			break
		}

		action := re.executeResponse(l, result)
		allActions = append(allActions, action)

		// 如果当前级别执行失败，记录但继续
		if !action.Success {
			log.Printf("[响应引擎] 级别 %d (%s) 执行失败: %s", l, l.String(), action.Error)
		}

		// 更新统计
		re.updateStats(action)
	}

	return allActions, nil
}

// evaluateResponseLevel 根据检测结果评估应采取的响应级别.
func (re *ResponseEngine) evaluateResponseLevel(result *DetectionResult) ResponseLevel {
	if result == nil {
		return ResponseLevelAlert
	}

	// 低置信度：仅告警
	if result.Confidence < re.config.ConfidenceThreshold {
		return ResponseLevelAlert
	}

	// 根据威胁等级和置信度决定响应级别
	switch result.ThreatLevel {
	case ThreatLevelCritical:
		// 关键威胁：根据是否有进程信息决定是否网络阻断
		if result.Confidence >= 0.9 && result.ProcessInfo != nil {
			return ResponseLevelNetworkIsolation
		}
		return ResponseLevelSnapshot

	case ThreatLevelHigh:
		// 高威胁：快照保护
		if result.Confidence >= 0.8 && result.ProcessInfo != nil {
			return ResponseLevelSnapshot
		}
		return ResponseLevelProcessIsolation

	case ThreatLevelMedium:
		// 中等威胁：终止进程
		if result.ProcessInfo != nil {
			return ResponseLevelProcessIsolation
		}
		return ResponseLevelAlert

	default:
		// 低威胁或未知：仅告警
		return ResponseLevelAlert
	}
}

// executeResponse 执行指定级别的响应.
func (re *ResponseEngine) executeResponse(level ResponseLevel, result *DetectionResult) *ResponseAction {
	action := &ResponseAction{
		ID:          uuid.New().String(),
		Timestamp:   time.Now(),
		Level:       level,
		DetectionID: result.ID,
	}

	switch level {
	case ResponseLevelAlert:
		re.executeAlertResponse(action, result)
	case ResponseLevelProcessIsolation:
		re.executeProcessIsolation(action, result)
	case ResponseLevelSnapshot:
		re.executeSnapshotProtection(action, result)
	case ResponseLevelNetworkIsolation:
		re.executeNetworkIsolation(action, result)
	default:
		action.Success = false
		action.Error = fmt.Sprintf("未知响应级别: %d", level)
	}

	// 存储动作记录
	re.storeAction(action)

	// 发送通知
	re.sendNotification(action)

	return action
}

// escalateResponse 升级响应级别，在当前级别执行失败或威胁升级时调用.
func (re *ResponseEngine) escalateResponse(result *DetectionResult, currentLevel ResponseLevel) *ResponseAction {
	nextLevel := currentLevel + 1

	// 检查是否超过最大级别
	if nextLevel > re.config.MaxLevel {
		return &ResponseAction{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Level:     currentLevel,
			Success:   false,
			Error:     fmt.Sprintf("已达最大响应级别 %d，无法继续升级", re.config.MaxLevel),
		}
	}

	log.Printf("[响应引擎] 威胁升级：从级别 %d (%s) 升级到 %d (%s)",
		currentLevel, currentLevel.String(), nextLevel, nextLevel.String())

	return re.executeResponse(nextLevel, result)
}

// ========== 各级别响应实现 ==========

// executeAlertResponse 执行告警通知响应（Level 1）.
func (re *ResponseEngine) executeAlertResponse(action *ResponseAction, result *DetectionResult) {
	action.ActionType = "alert"
	action.Target = result.FilePath

	if re.alertMgr != nil {
		alert := re.alertMgr.CreateAlert(result)
		if alert != nil {
			action.Success = true
			action.Details = fmt.Sprintf("告警已创建: %s", alert.ID)

			// 记录到告警的动作列表
			if err := re.alertMgr.AddActionTaken(alert.ID, fmt.Sprintf("响应引擎触发告警通知 (动作ID: %s)", action.ID)); err != nil {
				log.Printf("[响应引擎] 记录告警动作失败: %v", err)
			}
			return
		}
	}

	// 告警管理器不可用或未创建告警
	action.Success = true
	action.Details = "告警通知已记录（告警管理器不可用或未达阈值）"
}

// executeProcessIsolation 执行进程隔离响应（Level 2）.
func (re *ResponseEngine) executeProcessIsolation(action *ResponseAction, result *DetectionResult) {
	action.ActionType = "kill_process"

	if result.ProcessInfo == nil {
		action.Success = false
		action.Error = "无进程信息，无法执行进程隔离"
		return
	}

	action.Target = fmt.Sprintf("%s (PID: %d)", result.ProcessInfo.Name, result.ProcessInfo.PID)

	if re.killer == nil {
		action.Success = false
		action.Error = "进程终止器未配置"
		return
	}

	// 先尝试按PID终止
	if err := re.killer.KillProcess(result.ProcessInfo.PID); err != nil {
		// 按PID失败，尝试按名称终止
		if nameErr := re.killer.KillProcessByName(result.ProcessInfo.Name); nameErr != nil {
			action.Success = false
			action.Error = fmt.Sprintf("终止进程失败: PID=%d err=%v; 按名称 err=%v",
				result.ProcessInfo.PID, err, nameErr)
			return
		}
		action.Details = fmt.Sprintf("按名称终止进程成功: %s", result.ProcessInfo.Name)
	} else {
		action.Details = fmt.Sprintf("按PID终止进程成功: %d", result.ProcessInfo.PID)
	}

	action.Success = true
}

// executeSnapshotProtection 执行快照保护响应（Level 3）.
func (re *ResponseEngine) executeSnapshotProtection(action *ResponseAction, result *DetectionResult) {
	action.ActionType = "snapshot"
	action.Target = result.FilePath

	if re.snapshot == nil {
		action.Success = false
		action.Error = "快照创建器未配置"
		return
	}

	// 生成快照名称：前缀-时间戳-检测ID前8位
	snapshotName := fmt.Sprintf("%s-%s-%s",
		re.config.SnapshotPrefix,
		time.Now().Format("20060102-150405"),
		result.ID[:8])

	// 从受影响文件路径推断子卷路径
	subvolume := extractSubvolume(result.FilePath)
	if subvolume == "" {
		subvolume = "/" // 默认根卷
	}

	snapPath, err := re.snapshot.CreateSnapshot(snapshotName, subvolume)
	if err != nil {
		action.Success = false
		action.Error = fmt.Sprintf("创建快照失败: %v", err)
		return
	}

	action.Success = true
	action.Details = fmt.Sprintf("快照已创建: %s (路径: %s)", snapshotName, snapPath)

	// 更新统计中的活跃快照
	re.statsMu.Lock()
	re.stats.ActiveSnapshots = append(re.stats.ActiveSnapshots, snapshotName)
	re.statsMu.Unlock()
}

// executeNetworkIsolation 执行网络阻断响应（Level 4）.
func (re *ResponseEngine) executeNetworkIsolation(action *ResponseAction, result *DetectionResult) {
	action.ActionType = "isolate_network"
	action.Target = "受影响网段"

	if re.isolator == nil {
		action.Success = false
		action.Error = "网络隔离器未配置"
		return
	}

	// 根据配置的白名单网段执行隔离
	isolatedCount := 0
	var errors []string

	for _, cidr := range re.config.NetworkIsolationCIDRs {
		if err := re.isolator.IsolateCIDR(cidr); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", cidr, err))
			continue
		}
		isolatedCount++
		log.Printf("[响应引擎] 已隔离网段: %s", cidr)
	}

	if isolatedCount == 0 {
		action.Success = false
		action.Error = fmt.Sprintf("所有网段隔离失败: %v", errors)
		return
	}

	action.Success = true
	action.Details = fmt.Sprintf("已隔离 %d 个网段", isolatedCount)
	if len(errors) > 0 {
		action.Details += fmt.Sprintf("（%d 个失败）", len(errors))
	}

	// 更新统计中的隔离网段
	re.statsMu.Lock()
	re.stats.IsolatedCIDRs = re.isolator.GetIsolatedCIDRs()
	re.statsMu.Unlock()
}

// ========== 辅助方法 ==========

// storeAction 存储响应动作记录.
func (re *ResponseEngine) storeAction(action *ResponseAction) {
	re.actionMu.Lock()
	defer re.actionMu.Unlock()

	re.actions = append(re.actions, action)
}

// sendNotification 发送响应动作通知.
func (re *ResponseEngine) sendNotification(action *ResponseAction) {
	if re.config.NotifyChannel != nil {
		select {
		case re.config.NotifyChannel <- *action:
		default:
			// 通道阻塞，丢弃通知
			log.Printf("[响应引擎] 通知通道阻塞，丢弃动作通知: %s", action.ID)
		}
	}
}

// updateStats 更新响应统计.
func (re *ResponseEngine) updateStats(action *ResponseAction) {
	re.statsMu.Lock()
	defer re.statsMu.Unlock()

	re.stats.TotalResponses++
	re.stats.ByLevel[action.Level]++

	if action.Success {
		re.stats.SuccessCount++
	} else {
		re.stats.FailureCount++
	}

	now := time.Now()
	re.stats.LastResponseTime = &now
}

// extractSubvolume 从文件路径提取btrfs子卷路径.
func extractSubvolume(filePath string) string {
	// 常见NAS子卷路径映射
	subvolumes := []string{
		"/mnt/data",
		"/mnt/storage",
		"/data",
		"/shares",
		"/home",
	}

	for _, sv := range subvolumes {
		if len(filePath) >= len(sv) && filePath[:len(sv)] == sv {
			return sv
		}
	}

	return ""
}

// ========== 查询方法 ==========

// GetActions 获取响应动作列表.
func (re *ResponseEngine) GetActions(limit int) []*ResponseAction {
	re.actionMu.RLock()
	defer re.actionMu.RUnlock()

	if limit <= 0 || limit > len(re.actions) {
		limit = len(re.actions)
	}

	// 返回最新的记录
	start := len(re.actions) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*ResponseAction, limit)
	copy(result, re.actions[start:])
	return result
}

// GetStats 获取响应引擎统计.
func (re *ResponseEngine) GetStats() ResponseStats {
	re.statsMu.RLock()
	defer re.statsMu.RUnlock()

	// 深拷贝
	stats := re.stats
	stats.ByLevel = make(map[ResponseLevel]int64)
	for k, v := range re.stats.ByLevel {
		stats.ByLevel[k] = v
	}
	stats.IsolatedCIDRs = make([]string, len(re.stats.IsolatedCIDRs))
	copy(stats.IsolatedCIDRs, re.stats.IsolatedCIDRs)
	stats.ActiveSnapshots = make([]string, len(re.stats.ActiveSnapshots))
	copy(stats.ActiveSnapshots, re.stats.ActiveSnapshots)

	return stats
}

// RestoreNetwork 恢复所有被隔离的网段.
func (re *ResponseEngine) RestoreNetwork() error {
	if re.isolator == nil {
		return fmt.Errorf("网络隔离器未配置")
	}

	isolated := re.isolator.GetIsolatedCIDRs()
	var restoreErrors []string

	for _, cidr := range isolated {
		if err := re.isolator.RestoreCIDR(cidr); err != nil {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: %v", cidr, err))
		}
	}

	if len(restoreErrors) > 0 {
		return fmt.Errorf("部分网段恢复失败: %v", restoreErrors)
	}

	// 清空统计中的隔离记录
	re.statsMu.Lock()
	re.stats.IsolatedCIDRs = nil
	re.statsMu.Unlock()

	return nil
}
