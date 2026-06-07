// Package active 备份策略管理模块
// 负责备份调度、保留策略、时间窗口和带宽限制管理
package active

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ScheduleManager 备份调度管理器
type ScheduleManager struct {
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewScheduleManager 创建调度管理器
func NewScheduleManager(logger *zap.Logger) (*ScheduleManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ScheduleManager{
		logger: logger,
	}, nil
}

// TimeWindow 时间窗口
type TimeWindow struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	MaxCount    int `json:"max_count"`    // 最大保留份数
	MaxDays     int `json:"max_days"`     // 最大保留天数
	KeepDaily   int `json:"keep_daily"`   // 保留最近 N 天每天一份
	KeepWeekly  int `json:"keep_weekly"`  // 保留最近 N 周每周一份
	KeepMonthly int `json:"keep_monthly"` // 保留最近 N 月每月一份
}

// BandwidthLimit 带宽限制
type BandwidthLimit struct {
	MaxMBps  int          `json:"max_mbps"` // 最大带宽 MB/s（0=不限）
	Schedule []TimeWindow `json:"schedule"` // 限速时间段
	Priority int          `json:"priority"` // 优先级（1-10，10最高）
}

// IsWithinTimeWindow 检查当前时间是否在允许备份的时间窗口内
func (sm *ScheduleManager) IsWithinTimeWindow(schedule ScheduleConfig, now time.Time) bool {
	if schedule.StartTime == "" || schedule.EndTime == "" {
		return true // 无时间窗口限制
	}

	start, err := parseTime(schedule.StartTime, now)
	if err != nil {
		sm.logger.Warn("解析开始时间失败", zap.String("time", schedule.StartTime), zap.Error(err))
		return true
	}
	end, err := parseTime(schedule.EndTime, now)
	if err != nil {
		sm.logger.Warn("解析结束时间失败", zap.String("time", schedule.EndTime), zap.Error(err))
		return true
	}

	// 处理跨午夜的情况（如 22:00 - 06:00）
	if end.Before(start) {
		return now.After(start) || now.Before(end)
	}

	return now.After(start) && now.Before(end)
}

// CalculateNextRun 计算下次执行时间
func (sm *ScheduleManager) CalculateNextRun(schedule ScheduleConfig) time.Time {
	if schedule.Cron == "" {
		// 默认每天同一时间
		return time.Now().Add(24 * time.Hour)
	}

	return sm.parseCronNextRun(schedule.Cron, time.Now())
}

// ApplyRetentionPolicy 应用保留策略，返回应该删除的快照 ID 列表
func (sm *ScheduleManager) ApplyRetentionPolicy(policy RetentionPolicy, snapshots []*BackupSnapshot) []string {
	if len(snapshots) == 0 {
		return nil
	}

	toDelete := make(map[string]bool)
	keep := make(map[string]bool)

	// 按时间排序（最新在前）
	sorted := make([]*BackupSnapshot, len(snapshots))
	copy(sorted, snapshots)
	sortSnapshots(sorted)

	now := time.Now()

	// 按数量限制
	if policy.MaxCount > 0 && len(sorted) > policy.MaxCount {
		for i := policy.MaxCount; i < len(sorted); i++ {
			toDelete[sorted[i].ID] = true
		}
	}

	// 按天数限制
	if policy.MaxDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.MaxDays)
		for _, snap := range sorted {
			if snap.CreatedAt.Before(cutoff) {
				toDelete[snap.ID] = true
			}
		}
	}

	// 保留策略：每日保留
	if policy.KeepDaily > 0 {
		daysKept := 0
		lastDay := ""
		for _, snap := range sorted {
			day := snap.CreatedAt.Format("2006-01-02")
			if day != lastDay && daysKept < policy.KeepDaily {
				keep[snap.ID] = true
				lastDay = day
				daysKept++
			}
		}
	}

	// 保留策略：每周保留
	if policy.KeepWeekly > 0 {
		weeksKept := 0
		lastWeek := ""
		for _, snap := range sorted {
			year, week := snap.CreatedAt.ISOWeek()
			weekKey := fmt.Sprintf("%d-W%02d", year, week)
			if weekKey != lastWeek && weeksKept < policy.KeepWeekly {
				keep[snap.ID] = true
				lastWeek = weekKey
				weeksKept++
			}
		}
	}

	// 保留策略：每月保留
	if policy.KeepMonthly > 0 {
		monthsKept := 0
		lastMonth := ""
		for _, snap := range sorted {
			month := snap.CreatedAt.Format("2006-01")
			if month != lastMonth && monthsKept < policy.KeepMonthly {
				keep[snap.ID] = true
				lastMonth = month
				monthsKept++
			}
		}
	}

	// 最终列表：需要删除但不在 keep 列表中的
	result := make([]string, 0)
	for id := range toDelete {
		if !keep[id] {
			result = append(result, id)
		}
	}

	return result
}

// CalculateBandwidth 计算当前允许的带宽限制
func (sm *ScheduleManager) CalculateBandwidth(limit BandwidthLimit, now time.Time) int {
	if limit.MaxMBps <= 0 {
		return 0 // 不限速
	}

	if len(limit.Schedule) == 0 {
		return limit.MaxMBps
	}

	// 检查是否在限速时段内
	for _, window := range limit.Schedule {
		if isInTimeWindow(window, now) {
			return limit.MaxMBps
		}
	}

	return 0 // 不在限速时段内，不限速
}

// parseCronNextRun 简易 cron 解析器，计算下次执行时间
// 支持格式: "分 时 日 月 周" (标准5位cron)
func (sm *ScheduleManager) parseCronNextRun(expr string, from time.Time) time.Time {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		// 无法解析，返回明天
		return from.Add(24 * time.Hour)
	}

	// 简化实现：支持 "* * * * *" 等常见模式
	minute := parseCronField(fields[0], 0, 59)
	hour := parseCronField(fields[1], 0, 23)

	// 寻找下一个匹配时间
	candidate := from.Add(1 * time.Minute)
	candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(),
		candidate.Hour(), candidate.Minute(), 0, 0, candidate.Location())

	for i := 0; i < 1440*7; i++ { // 最多搜索一周
		if containsValue(minute, candidate.Minute()) && containsValue(hour, candidate.Hour()) {
			// 检查日期字段
			dayOfMonth := parseCronField(fields[2], 1, 31)
			month := parseCronField(fields[3], 1, 12)
			dayOfWeek := parseCronField(fields[4], 0, 6)

			if containsValue(dayOfMonth, candidate.Day()) &&
				containsValue(month, int(candidate.Month())) &&
				containsValue(dayOfWeek, int(candidate.Weekday())) {
				return candidate
			}
		}
		candidate = candidate.Add(1 * time.Minute)
	}

	// 兜底
	return from.Add(24 * time.Hour)
}

// parseCronField 解析 cron 字段，返回所有匹配值
func parseCronField(field string, min, max int) []int {
	if field == "*" {
		result := make([]int, max-min+1)
		for i := min; i <= max; i++ {
			result[i-min] = i
		}
		return result
	}

	// 处理 "*/N" 步进
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return []int{min}
		}
		result := make([]int, 0)
		for i := min; i <= max; i += step {
			result = append(result, i)
		}
		return result
	}

	// 处理范围 "N-M"
	rangeRe := regexp.MustCompile(`^(\d+)-(\d+)$`)
	if m := rangeRe.FindStringSubmatch(field); m != nil {
		start, _ := strconv.Atoi(m[1])
		end, _ := strconv.Atoi(m[2])
		result := make([]int, 0)
		for i := start; i <= end; i++ {
			result = append(result, i)
		}
		return result
	}

	// 处理列表 "N,M,K"
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		result := make([]int, 0, len(parts))
		for _, p := range parts {
			v, err := strconv.Atoi(strings.TrimSpace(p))
			if err == nil {
				result = append(result, v)
			}
		}
		return result
	}

	// 单个值
	v, err := strconv.Atoi(field)
	if err != nil {
		return []int{min}
	}
	return []int{v}
}

func containsValue(values []int, target int) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func parseTime(timeStr string, ref time.Time) (time.Time, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("无效的时间格式: %s，期望 HH:MM", timeStr)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("无效的小时: %s", parts[0])
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("无效的分钟: %s", parts[1])
	}

	return time.Date(ref.Year(), ref.Month(), ref.Day(), hour, minute, 0, 0, ref.Location()), nil
}

func isInTimeWindow(window TimeWindow, now time.Time) bool {
	start, err := parseTime(window.Start, now)
	if err != nil {
		return false
	}
	end, err := parseTime(window.End, now)
	if err != nil {
		return false
	}

	if end.Before(start) {
		return now.After(start) || now.Before(end)
	}
	return now.After(start) && now.Before(end)
}

// sortSnapshots 按创建时间排序（最新在前）
func sortSnapshots(snapshots []*BackupSnapshot) {
	for i := 1; i < len(snapshots); i++ {
		key := snapshots[i]
		j := i - 1
		for j >= 0 && snapshots[j].CreatedAt.Before(key.CreatedAt) {
			snapshots[j+1] = snapshots[j]
			j--
		}
		snapshots[j+1] = key
	}
}
