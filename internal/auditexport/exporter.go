package auditexport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"sort"
	"time"

	"go.uber.org/zap"
)

// Exporter 审计日志导出引擎
type Exporter struct {
	logger  *zap.Logger
	entries []AuditEntry
}

// NewExporter 创建导出引擎
func NewExporter(logger *zap.Logger, entries []AuditEntry) *Exporter {
	return &Exporter{
		logger:  logger,
		entries: entries,
	}
}

// filterEntries 内部过滤方法，根据过滤条件筛选日志
func (e *Exporter) filterEntries(filter ExportFilter) []AuditEntry {
	var result []AuditEntry

	for _, entry := range e.entries {
		// 时间范围过滤
		if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
			continue
		}

		// 用户过滤
		if len(filter.UserIDs) > 0 {
			found := false
			for _, uid := range filter.UserIDs {
				if entry.UserID == uid {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 操作类型过滤
		if len(filter.Actions) > 0 {
			found := false
			for _, a := range filter.Actions {
				if entry.Action == a {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 结果过滤
		if len(filter.Results) > 0 {
			found := false
			for _, r := range filter.Results {
				if entry.Result == r {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 严重级别过滤
		if len(filter.Severities) > 0 {
			found := false
			for _, s := range filter.Severities {
				if entry.Severity == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 资源过滤（前缀匹配）
		if filter.Resource != "" {
			if entry.Resource != filter.Resource {
				continue
			}
		}

		result = append(result, entry)
	}

	// 限制数量
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

// ExportCSV 导出 CSV 格式
func (e *Exporter) ExportCSV(filter ExportFilter) ([]byte, error) {
	filtered := e.filterEntries(filter)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// 写入表头
	header := []string{
		"时间戳", "用户ID", "用户名", "操作", "资源", "结果", "IP", "详情", "严重级别",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	// 写入数据
	for _, entry := range filtered {
		record := []string{
			entry.Timestamp.Format(time.RFC3339),
			entry.UserID,
			entry.UserName,
			entry.Action,
			entry.Resource,
			entry.Result,
			entry.IP,
			entry.Details,
			entry.Severity,
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	e.logger.Info("导出CSV完成", zap.Int("count", len(filtered)))
	return buf.Bytes(), nil
}

// ExportJSON 导出 JSON 格式
func (e *Exporter) ExportJSON(filter ExportFilter) ([]byte, error) {
	filtered := e.filterEntries(filter)

	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return nil, err
	}

	e.logger.Info("导出JSON完成", zap.Int("count", len(filtered)))
	return data, nil
}

// GenerateReport 生成合规报告
func (e *Exporter) GenerateReport(start, end time.Time) ComplianceReport {
	// 筛选时间范围内的条目
	filter := ExportFilter{
		StartTime: &start,
		EndTime:   &end,
	}
	entries := e.filterEntries(filter)

	report := ComplianceReport{
		GeneratedAt: time.Now(),
		PeriodStart: start,
		PeriodEnd:   end,
		TotalEvents: len(entries),
		ActionStats: make(map[string]int),
		ResultStats: make(map[string]int),
	}

	// 用户活跃统计
	userMap := make(map[string]*UserActivity)

	for _, entry := range entries {
		// 操作统计
		report.ActionStats[entry.Action]++

		// 结果统计
		report.ResultStats[entry.Result]++

		// 用户活跃统计
		if ua, ok := userMap[entry.UserID]; ok {
			ua.ActionCount++
			if entry.Timestamp.After(ua.LastActive) {
				ua.LastActive = entry.Timestamp
			}
		} else {
			userMap[entry.UserID] = &UserActivity{
				UserID:      entry.UserID,
				UserName:    entry.UserName,
				ActionCount: 1,
				LastActive:  entry.Timestamp,
			}
		}

		// 安全事件（critical 级别或 denied/failed 结果）
		if entry.Severity == "critical" || entry.Result == "denied" || entry.Result == "failed" {
			report.SecurityEvents = append(report.SecurityEvents, entry)
		}
	}

	// 构建 TopUsers（Top 10）
	var users []UserActivity
	for _, ua := range userMap {
		users = append(users, *ua)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].ActionCount > users[j].ActionCount
	})
	if len(users) > 10 {
		users = users[:10]
	}
	report.TopUsers = users

	// 异常登录检测
	report.AnomalyLogins = e.DetectAnomalies(entries)

	return report
}

// DetectAnomalies 检测异常登录
// 规则：
// 1. 同一用户从不同 IP 登录（短时间内）
// 2. 非常规时间（00:00-06:00）的登录
func (e *Exporter) DetectAnomalies(entries []AuditEntry) []AuditEntry {
	var anomalies []AuditEntry

	// 按用户分组
	userLogins := make(map[string][]AuditEntry)
	for _, entry := range entries {
		if entry.Action == "login" {
			userLogins[entry.UserID] = append(userLogins[entry.UserID], entry)
		}
	}

	for _, logins := range userLogins {
		// 检测不同 IP 登录
		ipSet := make(map[string]bool)
		for _, login := range logins {
			ipSet[login.IP] = true
		}
		if len(ipSet) > 1 {
			for _, login := range logins {
				anomalies = append(anomalies, login)
			}
		}
	}

	// 检测非常规时间登录（00:00-06:00）
	for _, entry := range entries {
		if entry.Action == "login" {
			hour := entry.Timestamp.Hour()
			if hour >= 0 && hour < 6 {
				// 避免重复添加
				found := false
				for _, a := range anomalies {
					if a.Timestamp == entry.Timestamp && a.UserID == entry.UserID {
						found = true
						break
					}
				}
				if !found {
					anomalies = append(anomalies, entry)
				}
			}
		}
	}

	return anomalies
}
