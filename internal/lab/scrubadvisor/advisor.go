// Package scrubadvisor converts pool, snapshot, and SMART signals into safe data-scrub guidance.
package scrubadvisor

import (
	"sort"
	"strings"
	"time"
)

// Signal describes NAS data-integrity signals inspired by TrueNAS scrubs,
// Synology Storage Manager health checks, and simple fnOS-style reminders.
type Signal struct {
	PoolName              string
	DaysSinceLastScrub    int
	ScrubInProgress       bool
	ParityOrMirrorEnabled bool
	SMARTWarnings         int
	ChecksumErrors        int
	SnapshotCount         int
	ImmutableSnapshots    int
	FreePercent           int
	RecentPowerLosses     int
	CriticalShares        int
}

// Recommendation is an actionable data-integrity hardening suggestion.
type Recommendation struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Priority string   `json:"priority"`
	Reason   string   `json:"reason"`
	Actions  []string `json:"actions"`
}

// Report summarizes current scrub and integrity readiness.
type Report struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	PoolName        string           `json:"pool_name"`
	IntegrityScore  int              `json:"integrity_score"`
	ScrubFreshness  string           `json:"scrub_freshness"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Advisor evaluates data-integrity posture for dashboards and notifications.
type Advisor struct{ now func() time.Time }

// New creates a scrub advisor.
func New() *Advisor { return &Advisor{now: time.Now} }

// WithNow returns a copy using a deterministic clock for tests.
func (a Advisor) WithNow(now func() time.Time) Advisor {
	if now != nil {
		a.now = now
	}
	return a
}

// Generate builds a deterministic report from current pool signals.
func (a Advisor) Generate(s Signal) Report {
	recs := make([]Recommendation, 0, 7)
	freshness := scrubFreshness(s.DaysSinceLastScrub, s.ScrubInProgress)

	if !s.ScrubInProgress && s.DaysSinceLastScrub > 30 {
		recs = append(recs, Recommendation{
			ID:       "schedule-pool-scrub",
			Title:    "安排存储池巡检",
			Priority: priorityByScrubAge(s.DaysSinceLastScrub),
			Reason:   "存储池长期未执行数据巡检，静默损坏和坏块可能无法及时暴露。",
			Actions: []string{
				"在低峰时段创建每月巡检计划",
				"巡检前确认备份任务已完成且 UPS 在线",
				"将巡检结果写入健康报告和通知中心",
			},
		})
	}

	if s.ChecksumErrors > 0 {
		recs = append(recs, Recommendation{
			ID:       "investigate-checksum-errors",
			Title:    "排查校验错误",
			Priority: "critical",
			Reason:   "已发现校验错误，应优先定位磁盘、内存、线缆或文件系统问题。",
			Actions: []string{
				"标记受影响文件和最近写入路径",
				"立即触发 SMART 长测和内存稳定性检查",
				"从快照或备份验证受影响数据可恢复性",
			},
		})
	}

	if s.SMARTWarnings > 0 {
		recs = append(recs, Recommendation{
			ID:       "replace-risky-drives",
			Title:    "处理磁盘健康告警",
			Priority: "high",
			Reason:   "存在 SMART 告警，继续运行会增加重建失败或数据不可读风险。",
			Actions: []string{
				"展示告警磁盘槽位、序列号和关键属性",
				"建议先完成最新备份再执行更换或重建",
				"重建期间降低非必要下载和转码任务优先级",
			},
		})
	}

	if !s.ParityOrMirrorEnabled && s.CriticalShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "add-redundancy-for-critical-shares",
			Title:    "为关键数据补充冗余",
			Priority: "high",
			Reason:   "关键共享位于无冗余池时，单盘故障即可造成停机或数据丢失。",
			Actions: []string{
				"迁移关键共享到镜像、RAIDZ 或 SHR 类冗余存储池",
				"迁移前创建只读快照和外部备份",
				"在仪表盘持续提示无冗余风险",
			},
		})
	}

	if s.CriticalShares > 0 && s.SnapshotCount == 0 {
		recs = append(recs, Recommendation{
			ID:       "enable-integrity-snapshots",
			Title:    "为巡检配套快照",
			Priority: "medium",
			Reason:   "没有快照时，误删、勒索和巡检发现的问题缺少快速回退锚点。",
			Actions: []string{
				"为关键共享创建小时/日/周快照计划",
				"巡检前后记录快照 ID 便于定位变化",
				"在文件管理器提供时间线恢复入口",
			},
		})
	}

	if s.SnapshotCount > 0 && s.ImmutableSnapshots == 0 {
		recs = append(recs, Recommendation{
			ID:       "lock-golden-snapshots",
			Title:    "锁定关键快照",
			Priority: "medium",
			Reason:   "普通快照可被误删或恶意删除，关键恢复点应提供不可变保留期。",
			Actions: []string{
				"将最近健康巡检后的快照设为黄金恢复点",
				"限制普通管理员删除不可变快照",
				"到期前通知容量影响并提供续期入口",
			},
		})
	}

	if s.FreePercent > 0 && s.FreePercent < 15 {
		recs = append(recs, Recommendation{
			ID:       "reserve-scrub-headroom",
			Title:    "释放巡检安全余量",
			Priority: "medium",
			Reason:   "可用容量过低会影响快照、重建和巡检期间的写入安全。",
			Actions: []string{
				"清理回收站、过期下载和旧版本备份",
				"将冷数据迁移到外置盘或对象存储",
				"把低容量状态纳入巡检前置检查",
			},
		})
	}

	if s.RecentPowerLosses > 0 {
		recs = append(recs, Recommendation{
			ID:       "stabilize-power-before-scrub",
			Title:    "巡检前稳定供电",
			Priority: "medium",
			Reason:   "近期断电会增加写入中断和巡检重跑概率。",
			Actions: []string{
				"接入 UPS 并启用低电量自动关机",
				"恢复供电后先完成文件系统检查",
				"把断电事件关联到健康时间线",
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
		GeneratedAt:     a.now(),
		PoolName:        s.PoolName,
		IntegrityScore:  integrityScore(s, recs),
		ScrubFreshness:  freshness,
		Recommendations: recs,
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

func scrubFreshness(days int, inProgress bool) string {
	if inProgress {
		return "running"
	}
	switch {
	case days <= 30:
		return "fresh"
	case days <= 60:
		return "stale"
	default:
		return "overdue"
	}
}

func priorityByScrubAge(days int) string {
	if days > 90 {
		return "high"
	}
	return "medium"
}

func priorityRank(priority string) int {
	switch priority {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

func integrityScore(s Signal, recs []Recommendation) int {
	score := 100
	for _, rec := range recs {
		switch rec.Priority {
		case "critical":
			score -= 30
		case "high":
			score -= 18
		case "medium":
			score -= 9
		default:
			score -= 4
		}
	}
	if s.ScrubInProgress {
		score += 5
	}
	if s.ParityOrMirrorEnabled {
		score += 5
	}
	if s.ImmutableSnapshots > 0 {
		score += 5
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
