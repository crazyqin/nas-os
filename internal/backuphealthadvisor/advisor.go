// Package backuphealthadvisor converts backup posture signals into actionable NAS protection guidance.
package backuphealthadvisor

import (
	"sort"
	"strings"
	"time"
)

// Signal describes backup, snapshot, and restore-readiness signals from a NAS.
type Signal struct {
	ProtectedDevices       int
	TotalDevices           int
	LastBackupHours        int
	FailedBackups          int
	IncrementalEnabled     bool
	DedupEnabled           bool
	SnapshotCount          int
	ImmutableSnapshots     int
	OffsiteCopies          int
	RestoreTestsLast30Days int
	RecoveryMediaCreated   bool
	RansomwareAlerts       int
	CriticalShares         int
}

// Recommendation is an actionable user-facing backup hardening suggestion.
type Recommendation struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Priority string   `json:"priority"`
	Reason   string   `json:"reason"`
	Actions  []string `json:"actions"`
}

// Report summarizes backup readiness for dashboards, notifications, and setup wizards.
type Report struct {
	GeneratedAt       time.Time        `json:"generated_at"`
	ProtectionPercent int              `json:"protection_percent"`
	ReadinessScore    int              `json:"readiness_score"`
	Recommendations   []Recommendation `json:"recommendations"`
}

// Advisor evaluates Synology Active Backup, TrueNAS snapshot/scrub, and fnOS-style simple recovery signals.
type Advisor struct {
	now func() time.Time
}

// New creates a backup health advisor.
func New() *Advisor { return &Advisor{now: time.Now} }

// WithNow returns a copy using a deterministic clock for tests.
func (a Advisor) WithNow(now func() time.Time) Advisor {
	if now != nil {
		a.now = now
	}
	return a
}

// Generate builds a deterministic report from current protection signals.
func (a Advisor) Generate(s Signal) Report {
	protected := protectionPercent(s.ProtectedDevices, s.TotalDevices)
	recs := make([]Recommendation, 0, 7)

	if s.TotalDevices > 0 && protected < 100 {
		recs = append(recs, Recommendation{
			ID:       "expand-device-protection",
			Title:    "补齐终端备份保护",
			Priority: priorityByProtection(protected),
			Reason:   "仍有电脑或移动设备未纳入统一备份，故障或勒索时恢复面不足。",
			Actions: []string{
				"为未保护设备下发备份代理或 WebDAV 备份入口",
				"使用模板统一备份频率、保留策略和限速窗口",
				"在仪表盘展示未保护设备清单和最后在线时间",
			},
		})
	}

	if s.LastBackupHours > 24 || s.FailedBackups > 0 {
		recs = append(recs, Recommendation{
			ID:       "repair-backup-failures",
			Title:    "修复备份失败与过期任务",
			Priority: "high",
			Reason:   "最近备份不新鲜或已有失败任务，恢复点目标可能无法满足家庭/小团队使用。",
			Actions: []string{
				"优先重试失败任务并展示错误原因",
				"对超过 24 小时未完成的任务发送通知",
				"检查网络、凭据、目标卷容量和任务锁定状态",
			},
		})
	}

	if !s.IncrementalEnabled || !s.DedupEnabled {
		recs = append(recs, Recommendation{
			ID:       "optimize-backup-efficiency",
			Title:    "启用增量与去重节省空间",
			Priority: "medium",
			Reason:   "未启用增量或全局去重时，多设备备份会消耗更多容量和网络带宽。",
			Actions: []string{
				"为终端备份启用块级增量或文件级增量",
				"对相同系统镜像、照片和办公文档启用全局去重",
				"把节省容量写入备份健康报告",
			},
		})
	}

	if s.CriticalShares > 0 && s.SnapshotCount == 0 {
		recs = append(recs, Recommendation{
			ID:       "enable-share-snapshots",
			Title:    "为关键共享开启快照",
			Priority: "high",
			Reason:   "关键共享缺少快照保护，误删、覆盖或勒索加密后缺少快速回滚点。",
			Actions: []string{
				"为关键共享创建只读快照计划",
				"保留小时、日、周多层恢复点",
				"在文件管理器中提供快照时间线恢复入口",
			},
		})
	}

	if s.RansomwareAlerts > 0 && s.ImmutableSnapshots == 0 {
		recs = append(recs, Recommendation{
			ID:       "add-immutable-recovery-points",
			Title:    "补充不可变恢复点",
			Priority: "high",
			Reason:   "存在勒索或异常写入告警，但缺少不可变快照会放大备份被篡改风险。",
			Actions: []string{
				"为最新健康备份加锁不可变保留期",
				"将异常写入前后的恢复点标记为高风险",
				"限制普通管理员删除不可变快照",
			},
		})
	}

	if s.RestoreTestsLast30Days == 0 {
		recs = append(recs, Recommendation{
			ID:       "schedule-restore-drill",
			Title:    "安排恢复演练",
			Priority: "medium",
			Reason:   "备份只有经过校验和试恢复后才可信，最近 30 天没有恢复演练记录。",
			Actions: []string{
				"每月自动抽样执行文件级试恢复",
				"记录恢复耗时、失败文件和校验结果",
				"把演练结果推送到健康报告和审计日志",
			},
		})
	}

	if s.OffsiteCopies == 0 || !s.RecoveryMediaCreated {
		recs = append(recs, Recommendation{
			ID:       "prepare-disaster-recovery",
			Title:    "准备异地副本与恢复介质",
			Priority: "medium",
			Reason:   "本机故障、火灾或系统盘损坏时，需要异地副本和可启动恢复介质降低停机时间。",
			Actions: []string{
				"配置 USB、另一台 NAS 或云端异地副本",
				"生成并定期刷新 USB/ISO 恢复介质",
				"在灾备向导中显示 RPO/RTO 预估",
			},
		})
	}

	sort.SliceStable(recs, func(i, j int) bool {
		left, right := priorityRank(recs[i].Priority), priorityRank(recs[j].Priority)
		if left == right {
			return recs[i].ID < recs[j].ID
		}
		return left < right
	})

	return Report{
		GeneratedAt:       a.now(),
		ProtectionPercent: protected,
		ReadinessScore:    readinessScore(s, protected, recs),
		Recommendations:   recs,
	}
}

// SummarizeActions returns compact next steps for notifications.
func SummarizeActions(recs []Recommendation) string {
	parts := make([]string, 0, len(recs))
	for _, rec := range recs {
		if len(rec.Actions) == 0 {
			continue
		}
		parts = append(parts, rec.Title+": "+rec.Actions[0])
	}
	return strings.Join(parts, "; ")
}

func protectionPercent(protected, total int) int {
	if total <= 0 || protected <= 0 {
		return 0
	}
	if protected >= total {
		return 100
	}
	return protected * 100 / total
}

func priorityByProtection(protected int) string {
	if protected < 70 {
		return "high"
	}
	return "medium"
}

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func readinessScore(s Signal, protected int, recs []Recommendation) int {
	score := 35 + protected/2
	if s.LastBackupHours > 0 && s.LastBackupHours <= 24 && s.FailedBackups == 0 {
		score += 10
	}
	if s.IncrementalEnabled {
		score += 5
	}
	if s.DedupEnabled {
		score += 5
	}
	if s.SnapshotCount > 0 {
		score += 10
	}
	if s.ImmutableSnapshots > 0 {
		score += 10
	}
	if s.OffsiteCopies > 0 {
		score += 10
	}
	if s.RestoreTestsLast30Days > 0 {
		score += 10
	}
	if s.RecoveryMediaCreated {
		score += 5
	}
	for _, rec := range recs {
		if rec.Priority == "high" {
			score -= 8
		}
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
