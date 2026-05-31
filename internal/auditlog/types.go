// Package auditlog 安全审计日志分析
// 对标群晖Log Center + TrueNAS Audit
// 日志采集分析、异常检测、合规报告
package auditlog

import (
	"fmt"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LevelDebug   LogLevel = "debug"
	LevelInfo    LogLevel = "info"
	LevelNotice  LogLevel = "notice"
	LevelWarning LogLevel = "warning"
	LevelError   LogLevel = "error"
	LevelCritical LogLevel = "critical"
)

// LogSource 日志来源
type LogSource string

const (
	SourceAuth    LogSource = "auth"
	SourceSystem  LogSource = "system"
	SourceStorage LogSource = "storage"
	SourceNetwork LogSource = "network"
	SourceApp     LogSource = "app"
	SourceAudit   LogSource = "audit"
)

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Source    LogSource `json:"source"`
	User      string    `json:"user"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	Success   bool      `json:"success"`
}

// Anomaly 异常检测结果
type Anomaly struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Severity    LogLevel  `json:"severity"`
	Description string    `json:"description"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	User        string    `json:"user,omitempty"`
	IP          string    `json:"ip,omitempty"`
	Resolved    bool      `json:"resolved"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID           string           `json:"id"`
	Period       string           `json:"period"`
	TotalEvents  int              `json:"total_events"`
	ByLevel      map[LogLevel]int `json:"by_level"`
	BySource     map[LogSource]int `json:"by_source"`
	TopActions   []ActionCount    `json:"top_actions"`
	FailedLogins int              `json:"failed_logins"`
	Anomalies    int              `json:"anomalies"`
	Score        int              `json:"score"` // 0-100
	GeneratedAt  time.Time        `json:"generated_at"`
}

// ActionCount 操作计数
type ActionCount struct {
	Action string `json:"action"`
	Count  int    `json:"count"`
}

// LogFilter 日志过滤器
type LogFilter struct {
	Level     LogLevel  `json:"level"`
	Source    LogSource `json:"source"`
	User      string    `json:"user"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit"`
}

// Manager 审计日志管理器
type Manager struct {
	mu        sync.RWMutex
	entries   []AuditEntry
	anomalies []Anomaly
	reports   map[string]*ComplianceReport
	maxEntry  int
}

// NewManager 创建审计日志管理器
func NewManager() *Manager {
	return &Manager{
		entries:   make([]AuditEntry, 0),
		anomalies: make([]Anomaly, 0),
		reports:   make(map[string]*ComplianceReport),
		maxEntry:  10000,
	}
}

// AddEntry 添加审计日志
func (m *Manager) AddEntry(entry AuditEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry.ID = fmt.Sprintf("audit_%d", time.Now().UnixNano())
	entry.Timestamp = time.Now()
	m.entries = append([]AuditEntry{entry}, m.entries...)
	if len(m.entries) > m.maxEntry {
		m.entries = m.entries[:m.maxEntry]
	}

	// 简单异常检测
	m.detectAnomaly(entry)
}

// Query 查询日志
func (m *Manager) Query(filter LogFilter) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AuditEntry, 0)
	for _, e := range m.entries {
		if filter.Level != "" && e.Level != filter.Level {
			continue
		}
		if filter.Source != "" && e.Source != filter.Source {
			continue
		}
		if filter.User != "" && e.User != filter.User {
			continue
		}
		if !filter.StartTime.IsZero() && e.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && e.Timestamp.After(filter.EndTime) {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result
}

// GetAnomalies 获取异常列表
func (m *Manager) GetAnomalies(resolved bool) []Anomaly {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Anomaly, 0)
	for _, a := range m.anomalies {
		if a.Resolved == resolved {
			result = append(result, a)
		}
	}
	return result
}

// ResolveAnomaly 解决异常
func (m *Manager) ResolveAnomaly(anomalyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.anomalies {
		if m.anomalies[i].ID == anomalyID {
			m.anomalies[i].Resolved = true
			return nil
		}
	}
	return fmt.Errorf("异常不存在: %s", anomalyID)
}

// GenerateReport 生成合规报告
func (m *Manager) GenerateReport(period string) *ComplianceReport {
	m.mu.RLock()
	entries := make([]AuditEntry, len(m.entries))
	copy(entries, m.entries)
	unresolvedAnomalies := 0
	for _, a := range m.anomalies {
		if !a.Resolved {
			unresolvedAnomalies++
		}
	}
	m.mu.RUnlock()

	report := &ComplianceReport{
		ID:          fmt.Sprintf("report_%d", time.Now().UnixNano()),
		Period:      period,
		ByLevel:     make(map[LogLevel]int),
		BySource:    make(map[LogSource]int),
		TopActions:  make([]ActionCount, 0),
		GeneratedAt: time.Now(),
	}

	actionCount := make(map[string]int)
	for _, e := range entries {
		report.TotalEvents++
		report.ByLevel[e.Level]++
		report.BySource[e.Source]++
		actionCount[e.Action]++
		if e.Source == SourceAuth && !e.Success {
			report.FailedLogins++
		}
	}

	// Top 10 actions
	type kv struct {
		k string
		v int
	}
	sorted := make([]kv, 0)
	for k, v := range actionCount {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted) && i < 10; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
		report.TopActions = append(report.TopActions, ActionCount{
			Action: sorted[i].k,
			Count:  sorted[i].v,
		})
	}

	// 计算合规分数
	score := 100
	if report.FailedLogins > 100 {
		score -= 20
	}
	score -= unresolvedAnomalies * 5
	if score < 0 {
		score = 0
	}
	report.Score = score
	report.Anomalies = unresolvedAnomalies

	m.mu.Lock()
	m.reports[report.ID] = report
	m.mu.Unlock()

	return report
}

// GetReport 获取报告
func (m *Manager) GetReport(reportID string) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[reportID]
	if !ok {
		return nil, fmt.Errorf("报告不存在: %s", reportID)
	}
	return report, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	unresolvedAnomalies := 0
	for _, a := range m.anomalies {
		if !a.Resolved {
			unresolvedAnomalies++
		}
	}

	return map[string]interface{}{
		"total_entries":        len(m.entries),
		"total_anomalies":      len(m.anomalies),
		"unresolved_anomalies": unresolvedAnomalies,
		"total_reports":        len(m.reports),
	}
}

func (m *Manager) detectAnomaly(entry AuditEntry) {
	// 检测暴力破解
	if entry.Source == SourceAuth && !entry.Success {
		count := 0
		for _, e2 := range m.entries {
			if e2.Source == SourceAuth && !e2.Success && e2.IP == entry.IP &&
				time.Since(e2.Timestamp) < 10*time.Minute {
				count++
			}
		}
		if count >= 5 {
			m.anomalies = append(m.anomalies, Anomaly{
				ID:          fmt.Sprintf("anomaly_%d", time.Now().UnixNano()),
				Type:        "brute_force",
				Severity:    LevelCritical,
				Description: fmt.Sprintf("检测到暴力破解尝试: IP %s, %d次失败登录", entry.IP, count),
				Count:       count,
				FirstSeen:   entry.Timestamp,
				LastSeen:    entry.Timestamp,
				IP:          entry.IP,
			})
		}
	}
}
