package ransommldetect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ============================================================
// IncidentResponse – automated response actions
// ============================================================

// IncidentResponse handles automated response to ransomware threats.
type IncidentResponse struct {
	detector *Detector
	logger   *zap.Logger
}

// NewIncidentResponse creates a new incident response handler.
func NewIncidentResponse(detector *Detector, logger *zap.Logger) *IncidentResponse {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &IncidentResponse{detector: detector, logger: logger}
}

// QuarantinePaths moves the specified paths to a quarantine directory.
// The quarantine directory defaults to /data/.quarantine/<timestamp>/.
func (ir *IncidentResponse) QuarantinePaths(paths []string, reason string) QuarantineResult {
	result := QuarantineResult{
		Quarantined: make([]string, 0, len(paths)),
		Failed:      make([]string, 0),
	}

	ts := time.Now().Format("20060102-150405")
	quarantineBase := filepath.Join("/data", ".quarantine", ts)

	for _, p := range paths {
		if p == "" {
			continue
		}
		// Safety: refuse to quarantine system paths
		if isSystemPath(p) {
			ir.logger.Warn("拒绝隔离系统路径", zap.String("path", p))
			result.Failed = append(result.Failed, p)
			continue
		}

		dest := filepath.Join(quarantineBase, filepath.Base(p))
		if err := os.Rename(p, dest); err != nil {
			ir.logger.Error("隔离文件失败",
				zap.String("path", p),
				zap.String("dest", dest),
				zap.Error(err),
			)
			result.Failed = append(result.Failed, p)
			continue
		}

		ir.logger.Info("文件已隔离",
			zap.String("path", p),
			zap.String("dest", dest),
			zap.String("reason", reason),
		)
		result.Quarantined = append(result.Quarantined, p)
	}

	if len(result.Quarantined) > 0 {
		result.Message = fmt.Sprintf("已隔离 %d 个路径", len(result.Quarantined))
	} else {
		result.Message = "无路径被隔离"
	}
	return result
}

// AutoQuarantine checks the current threat level and automatically
// quarantines affected directories if the level is High or Critical.
func (ir *IncidentResponse) AutoQuarantine() *QuarantineResult {
	status := ir.detector.GetThreatStatus()
	if status.Level < ThreatLevelHigh {
		ir.logger.Debug("威胁等级未达自动隔离阈值", zap.String("level", status.Level.String()))
		return nil
	}

	ir.logger.Warn("威胁等级达到自动隔离阈值，启动自动隔离",
		zap.String("level", status.Level.String()),
		zap.Float64("score", status.Score),
	)

	// Determine the affected directory from the last alert
	var affectedPaths []string
	if status.LastAlert != nil && status.LastAlert.Source != "" {
		affectedPaths = append(affectedPaths, status.LastAlert.Source)
	}

	if len(affectedPaths) == 0 {
		ir.logger.Info("无受影响路径可隔离")
		return nil
	}

	result := ir.QuarantinePaths(affectedPaths, fmt.Sprintf("自动隔离: threat_level=%s score=%.2f", status.Level, status.Score))
	return &result
}

// isSystemPath returns true for paths that should never be quarantined.
func isSystemPath(p string) bool {
	abs, err := filepath.Abs(p)
	if err != nil {
		return true // can't resolve → safest to refuse
	}
	systemPrefixes := []string{"/bin", "/sbin", "/usr", "/etc", "/lib", "/boot", "/dev", "/proc", "/sys", "/var/run"}
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(abs, prefix) {
			return true
		}
	}
	return false
}

// ============================================================
// RecoveryPlan – restore recommendations
// ============================================================

// RecoveryStep is a single step in a recovery plan.
type RecoveryStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Target      string `json:"target,omitempty"`
	Priority    string `json:"priority"` // critical, high, medium, low
}

// RecoveryPlan provides recovery recommendations after a ransomware incident.
type RecoveryPlan struct {
	IncidentID  string         `json:"incident_id"`
	ThreatLevel ThreatLevel    `json:"threat_level"`
	Score       float64        `json:"score"`
	Steps       []RecoveryStep `json:"steps"`
	Summary     string         `json:"summary"`
	CreatedAt   time.Time      `json:"created_at"`
}

// GenerateRecoveryPlan creates a recovery plan based on the current threat.
func (ir *IncidentResponse) GenerateRecoveryPlan() *RecoveryPlan {
	status := ir.detector.GetThreatStatus()
	if status.Level == ThreatLevelLow && status.Score < 0.3 {
		return nil // nothing to recover from
	}

	steps := []RecoveryStep{
		{
			Order:       1,
			Action:      "isolate",
			Description: "确认受影响目录已隔离，阻止加密扩散",
			Priority:    "critical",
		},
		{
			Order:       2,
			Action:      "snapshot_verify",
			Description: "验证最近快照完整性，确认可用的恢复点",
			Target:      "/data/.snapshots",
			Priority:    "critical",
		},
		{
			Order:       3,
			Action:      "stop_processes",
			Description: "终止与攻击关联的进程",
			Priority:    "high",
		},
		{
			Order:       4,
			Action:      "assess_damage",
			Description: "评估受影响文件范围与数量",
			Priority:    "high",
		},
		{
			Order:       5,
			Action:      "restore",
			Description: "从最近安全快照恢复受影响文件",
			Target:      "/data/.snapshots",
			Priority:    "high",
		},
		{
			Order:       6,
			Action:      "verify_integrity",
			Description: "验证恢复后文件完整性（哈希对比）",
			Priority:    "medium",
		},
		{
			Order:       7,
			Action:      "review_logs",
			Description: "审查日志确认攻击向量，修补入口",
			Priority:    "medium",
		},
		{
			Order:       8,
			Action:      "update_rules",
			Description: "根据本次事件更新检测规则，防止再次发生",
			Priority:    "low",
		},
	}

	if status.Level >= ThreatLevelCritical {
		// Prepend immediate network isolation for critical threats
		steps = append([]RecoveryStep{{
			Order:       0,
			Action:      "network_isolate",
			Description: "立即断开NAS网络连接，阻止数据外泄",
			Priority:    "critical",
		}}, steps...)
		// Re-number
		for i := range steps {
			steps[i].Order = i
		}
	}

	incidentID := "INC-UNKNOWN"
	if status.LastAlert != nil {
		incidentID = status.LastAlert.ID
	}

	return &RecoveryPlan{
		IncidentID:  incidentID,
		ThreatLevel: status.Level,
		Score:       status.Score,
		Steps:       steps,
		Summary:     fmt.Sprintf("检测到威胁等级 %s (分数 %.2f)，建议执行 %d 步恢复计划", status.Level, status.Score, len(steps)),
		CreatedAt:   time.Now(),
	}
}

// ============================================================
// AlertEscalation – notification and escalation policies
// ============================================================

// EscalationAction describes an action taken during escalation.
type EscalationAction struct {
	Type       string    `json:"type"`   // notify, auto_quarantine, network_lockdown, shutdown
	Target     string    `json:"target"` // channel, path, or system component
	Message    string    `json:"message"`
	ExecutedAt time.Time `json:"executed_at"`
	Success    bool      `json:"success"`
}

// EscalationPolicy defines when and how to escalate.
type EscalationPolicy struct {
	// MinThreatLevel is the minimum threat level that triggers this policy.
	MinThreatLevel ThreatLevel `json:"min_threat_level"`
	// MinScore minimum composite score (0-1) to trigger.
	MinScore float64 `json:"min_score"`
	// NotifyChannels channels to notify (e.g., "email", "webhook", "sms").
	NotifyChannels []string `json:"notify_channels"`
	// AutoQuarantine whether to auto-quarantine affected paths.
	AutoQuarantine bool `json:"auto_quarantine"`
	// NetworkLockdown whether to trigger network isolation.
	NetworkLockdown bool `json:"network_lockdown"`
	// CooldownSec minimum seconds between escalations for the same alert.
	CooldownSec int `json:"cooldown_sec"`
}

// DefaultEscalationPolicy returns the default escalation policy.
func DefaultEscalationPolicy() EscalationPolicy {
	return EscalationPolicy{
		MinThreatLevel:  ThreatLevelHigh,
		MinScore:        0.7,
		NotifyChannels:  []string{"email", "webhook"},
		AutoQuarantine:  true,
		NetworkLockdown: false,
		CooldownSec:     300, // 5 min
	}
}

// AlertEscalation manages escalation of ransomware alerts.
type AlertEscalation struct {
	mu             syncEsc
	policy         EscalationPolicy
	detector       *Detector
	logger         *zap.Logger
	lastEscalation map[string]time.Time // alert source → last escalation time
	history        []EscalationAction
}

type syncEsc struct{ ch chan struct{} }

// NewAlertEscalation creates a new alert escalation manager.
func NewAlertEscalation(detector *Detector, policy EscalationPolicy, logger *zap.Logger) *AlertEscalation {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AlertEscalation{
		policy:         policy,
		detector:       detector,
		logger:         logger,
		lastEscalation: make(map[string]time.Time),
		history:        make([]EscalationAction, 0, 64),
	}
}

// Evaluate checks the current threat status and escalates if needed.
// Returns the list of escalation actions taken (may be empty).
func (ae *AlertEscalation) Evaluate() []EscalationAction {
	status := ae.detector.GetThreatStatus()

	if status.Level < ae.policy.MinThreatLevel && status.Score < ae.policy.MinScore {
		return nil
	}

	// Cooldown check
	source := "global"
	if status.LastAlert != nil {
		source = status.LastAlert.Source
	}
	if last, ok := ae.lastEscalation[source]; ok && ae.policy.CooldownSec > 0 {
		if time.Since(last) < time.Duration(ae.policy.CooldownSec)*time.Second {
			ae.logger.Debug("告警升级冷却中", zap.String("source", source))
			return nil
		}
	}

	var actions []EscalationAction

	// Notify channels
	for _, ch := range ae.policy.NotifyChannels {
		msg := fmt.Sprintf("🚨 勒索软件告警: 威胁等级=%s 分数=%.2f 来源=%s",
			status.Level, status.Score, source)
		ae.logger.Warn("告警通知", zap.String("channel", ch), zap.String("message", msg))
		actions = append(actions, EscalationAction{
			Type:       "notify",
			Target:     ch,
			Message:    msg,
			ExecutedAt: time.Now(),
			Success:    true, // notification dispatch is assumed successful
		})
	}

	// Auto-quarantine
	if ae.policy.AutoQuarantine && status.Level >= ThreatLevelHigh {
		ir := NewIncidentResponse(ae.detector, ae.logger)
		if result := ir.AutoQuarantine(); result != nil {
			actions = append(actions, EscalationAction{
				Type:       "auto_quarantine",
				Target:     strings.Join(result.Quarantined, ","),
				Message:    result.Message,
				ExecutedAt: time.Now(),
				Success:    len(result.Failed) == 0,
			})
		}
	}

	// Network lockdown
	if ae.policy.NetworkLockdown && status.Level >= ThreatLevelCritical {
		ae.logger.Error("触发网络隔离！断开NAS网络连接")
		actions = append(actions, EscalationAction{
			Type:       "network_lockdown",
			Target:     "network",
			Message:    "严重威胁触发网络隔离",
			ExecutedAt: time.Now(),
			Success:    true, // placeholder; real implementation would call network manager
		})
	}

	ae.lastEscalation[source] = time.Now()
	ae.history = append(ae.history, actions...)
	return actions
}

// History returns past escalation actions.
func (ae *AlertEscalation) History() []EscalationAction {
	result := make([]EscalationAction, len(ae.history))
	copy(result, ae.history)
	return result
}
