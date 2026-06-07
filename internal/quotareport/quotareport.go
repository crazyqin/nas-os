// Package quotareport 配额报告 - 存储配额统计与报告
// 对标TrueNAS配额管理
package quotareport

import (
	"encoding/json"
	"sync"
	"time"
)

// QuotaType 配额类型
type QuotaType string

const (
	QuotaTypeUser  QuotaType = "user"
	QuotaTypeGroup QuotaType = "group"
	QuotaTypePool  QuotaType = "pool"
	QuotaTypeShare QuotaType = "share"
)

// QuotaStatus 配额状态
type QuotaStatus string

const (
	QuotaStatusNormal   QuotaStatus = "normal"
	QuotaStatusWarning  QuotaStatus = "warning"
	QuotaStatusExceeded QuotaStatus = "exceeded"
	QuotaStatusDisabled QuotaStatus = "disabled"
)

// QuotaUnit 配额单位
type QuotaUnit string

const (
	UnitBytes QuotaUnit = "bytes"
	UnitKB    QuotaUnit = "KB"
	UnitMB    QuotaUnit = "MB"
	UnitGB    QuotaUnit = "GB"
	UnitTB    QuotaUnit = "TB"
	UnitFiles QuotaUnit = "files"
)

// QuotaEntry 配额条目
type QuotaEntry struct {
	ID       string    `json:"id"`
	Type     QuotaType `json:"type"`
	Name     string    `json:"name"`      // 用户名/组名/池名
	TargetID string    `json:"target_id"` // UID/GID/Pool ID

	// 配额限制
	HardLimit   int64 `json:"hard_limit"`   // 硬限制
	SoftLimit   int64 `json:"soft_limit"`   // 软限制
	GracePeriod int   `json:"grace_period"` // 宽限期（天）

	// 当前使用
	CurrentUsage int64 `json:"current_usage"`
	FileCount    int64 `json:"file_count"`

	// 状态
	Status      QuotaStatus `json:"status"`
	LastChecked time.Time   `json:"last_checked"`

	// 时间戳
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QuotaReport 配额报告
type QuotaReport struct {
	ID          string       `json:"id"`
	GeneratedAt time.Time    `json:"generated_at"`
	Period      ReportPeriod `json:"period"`

	// 统计摘要
	TotalQuotas   int `json:"total_quotas"`
	ExceededCount int `json:"exceeded_count"`
	WarningCount  int `json:"warning_count"`
	NormalCount   int `json:"normal_count"`

	// 使用统计
	TotalUsage   int64   `json:"total_usage"`
	TotalLimit   int64   `json:"total_limit"`
	UsagePercent float64 `json:"usage_percent"`

	// 详细数据
	Entries   []*QuotaEntry `json:"entries"`
	TopUsers  []*UsageRank  `json:"top_users"`
	TopGroups []*UsageRank  `json:"top_groups"`

	// 趋势数据
	TrendData []*TrendPoint `json:"trend_data"`
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Type  string    `json:"type"` // daily, weekly, monthly
}

// UsageRank 使用排名
type UsageRank struct {
	Name       string  `json:"name"`
	Usage      int64   `json:"usage"`
	FileCount  int64   `json:"file_count"`
	Percent    float64 `json:"percent"`
	GrowthRate float64 `json:"growth_rate"` // 增长率
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	TotalUsage int64     `json:"total_usage"`
	FileCount  int64     `json:"file_count"`
	UserCount  int       `json:"user_count"`
}

// QuotaAlert 配额告警
type QuotaAlert struct {
	ID           string        `json:"id"`
	QuotaID      string        `json:"quota_id"`
	Type         AlertType     `json:"type"`
	Severity     AlertSeverity `json:"severity"`
	Message      string        `json:"message"`
	Threshold    float64       `json:"threshold"`
	Current      float64       `json:"current"`
	CreatedAt    time.Time     `json:"created_at"`
	Acknowledged bool          `json:"acknowledged"`
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeWarning  AlertType = "warning"
	AlertTypeExceeded AlertType = "exceeded"
	AlertTypeCritical AlertType = "critical"
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// QuotaManager 配额管理器
type QuotaManager struct {
	mu      sync.RWMutex
	quotas  map[string]*QuotaEntry  // id -> entry
	reports map[string]*QuotaReport // id -> report
	alerts  []*QuotaAlert
	config  *QuotaConfig
}

// QuotaConfig 配额配置
type QuotaConfig struct {
	Enabled            bool     `json:"enabled"`
	WarningThreshold   float64  `json:"warning_threshold"`    // 80%
	ExceededThreshold  float64  `json:"exceeded_threshold"`   // 100%
	CriticalThreshold  float64  `json:"critical_threshold"`   // 95%
	DefaultGracePeriod int      `json:"default_grace_period"` // 7天
	ReportInterval     int      `json:"report_interval"`      // 小时
	MaxReports         int      `json:"max_reports"`
	EnableAlerts       bool     `json:"enable_alerts"`
	AlertEmails        []string `json:"alert_emails,omitempty"`
}

// DefaultQuotaConfig 默认配置
func DefaultQuotaConfig() *QuotaConfig {
	return &QuotaConfig{
		Enabled:            true,
		WarningThreshold:   80.0,
		ExceededThreshold:  100.0,
		CriticalThreshold:  95.0,
		DefaultGracePeriod: 7,
		ReportInterval:     24,
		MaxReports:         100,
		EnableAlerts:       true,
	}
}

// NewQuotaManager 创建配额管理器
func NewQuotaManager(config *QuotaConfig) *QuotaManager {
	if config == nil {
		config = DefaultQuotaConfig()
	}

	return &QuotaManager{
		quotas:  make(map[string]*QuotaEntry),
		reports: make(map[string]*QuotaReport),
		alerts:  make([]*QuotaAlert, 0),
		config:  config,
	}
}

// AddQuota 添加配额
func (m *QuotaManager) AddQuota(quota *QuotaEntry) error {
	if quota == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	now := time.Now()
	if quota.CreatedAt.IsZero() {
		quota.CreatedAt = now
	}
	quota.UpdatedAt = now
	quota.LastChecked = now

	if quota.Status == "" {
		quota.Status = QuotaStatusNormal
	}

	if quota.GracePeriod == 0 {
		quota.GracePeriod = m.config.DefaultGracePeriod
	}

	m.quotas[quota.ID] = quota

	return nil
}

// GetQuota 获取配额
func (m *QuotaManager) GetQuota(id string) (*QuotaEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.quotas[id]
	return quota, exists
}

// UpdateQuota 更新配额
func (m *QuotaManager) UpdateQuota(id string, update func(*QuotaEntry)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.quotas[id]
	if !exists {
		return nil
	}

	update(quota)
	quota.UpdatedAt = time.Now()

	// 更新状态
	m.updateQuotaStatus(quota)

	return nil
}

// DeleteQuota 删除配额
func (m *QuotaManager) DeleteQuota(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.quotas, id)
	return nil
}

// ListQuotas 列出配额
func (m *QuotaManager) ListQuotas(quotaType *QuotaType) []*QuotaEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quotas := make([]*QuotaEntry, 0)

	for _, quota := range m.quotas {
		if quotaType != nil && quota.Type != *quotaType {
			continue
		}
		quotas = append(quotas, quota)
	}

	return quotas
}

// UpdateUsage 更新使用量
func (m *QuotaManager) UpdateUsage(id string, usage int64, fileCount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.quotas[id]
	if !exists {
		return nil
	}

	quota.CurrentUsage = usage
	quota.FileCount = fileCount
	quota.LastChecked = time.Now()
	quota.UpdatedAt = time.Now()

	// 更新状态
	m.updateQuotaStatus(quota)

	return nil
}

// updateQuotaStatus 更新配额状态
func (m *QuotaManager) updateQuotaStatus(quota *QuotaEntry) {
	if quota.HardLimit == 0 {
		quota.Status = QuotaStatusDisabled
		return
	}

	percent := float64(quota.CurrentUsage) / float64(quota.HardLimit) * 100

	switch {
	case percent >= m.config.ExceededThreshold:
		quota.Status = QuotaStatusExceeded
	case percent >= m.config.WarningThreshold:
		quota.Status = QuotaStatusWarning
	default:
		quota.Status = QuotaStatusNormal
	}

	// 检查是否需要告警
	m.checkAlert(quota, percent)
}

// checkAlert 检查告警
func (m *QuotaManager) checkAlert(quota *QuotaEntry, percent float64) {
	if !m.config.EnableAlerts {
		return
	}

	var alertType AlertType
	var severity AlertSeverity
	var message string

	switch {
	case percent >= m.config.CriticalThreshold:
		alertType = AlertTypeCritical
		severity = AlertSeverityCritical
		message = "配额严重超标"
	case percent >= m.config.ExceededThreshold:
		alertType = AlertTypeExceeded
		severity = AlertSeverityHigh
		message = "配额已超标"
	case percent >= m.config.WarningThreshold:
		alertType = AlertTypeWarning
		severity = AlertSeverityMedium
		message = "配额接近上限"
	default:
		return
	}

	alert := &QuotaAlert{
		ID:        generateAlertID(),
		QuotaID:   quota.ID,
		Type:      alertType,
		Severity:  severity,
		Message:   message + ": " + quota.Name,
		Threshold: percent,
		Current:   percent,
		CreatedAt: time.Now(),
	}

	m.alerts = append(m.alerts, alert)
}

// GenerateReport 生成报告
func (m *QuotaManager) GenerateReport(period ReportPeriod) *QuotaReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	report := &QuotaReport{
		ID:          generateReportID(),
		GeneratedAt: time.Now(),
		Period:      period,
		Entries:     make([]*QuotaEntry, 0),
		TopUsers:    make([]*UsageRank, 0),
		TopGroups:   make([]*UsageRank, 0),
		TrendData:   make([]*TrendPoint, 0),
	}

	// 统计配额
	for _, quota := range m.quotas {
		report.Entries = append(report.Entries, quota)
		report.TotalQuotas++

		switch quota.Status {
		case QuotaStatusExceeded:
			report.ExceededCount++
		case QuotaStatusWarning:
			report.WarningCount++
		case QuotaStatusNormal:
			report.NormalCount++
		}

		report.TotalUsage += quota.CurrentUsage
		report.TotalLimit += quota.HardLimit
	}

	// 计算使用百分比
	if report.TotalLimit > 0 {
		report.UsagePercent = float64(report.TotalUsage) / float64(report.TotalLimit) * 100
	}

	// 保存报告
	m.reports[report.ID] = report

	// 清理旧报告
	m.cleanupReports()

	return report
}

// cleanupReports 清理旧报告
func (m *QuotaManager) cleanupReports() {
	if len(m.reports) <= m.config.MaxReports {
		return
	}

	// 找到最旧的报告
	var oldestID string
	var oldestTime time.Time

	for id, report := range m.reports {
		if oldestID == "" || report.GeneratedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = report.GeneratedAt
		}
	}

	if oldestID != "" {
		delete(m.reports, oldestID)
	}
}

// GetReport 获取报告
func (m *QuotaManager) GetReport(id string) (*QuotaReport, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, exists := m.reports[id]
	return report, exists
}

// ListReports 列出报告
func (m *QuotaManager) ListReports(limit int) []*QuotaReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*QuotaReport, 0, len(m.reports))
	for _, report := range m.reports {
		reports = append(reports, report)
	}

	// 按时间排序
	for i := 0; i < len(reports); i++ {
		for j := i + 1; j < len(reports); j++ {
			if reports[j].GeneratedAt.After(reports[i].GeneratedAt) {
				reports[i], reports[j] = reports[j], reports[i]
			}
		}
	}

	if limit > 0 && limit < len(reports) {
		reports = reports[:limit]
	}

	return reports
}

// GetAlerts 获取告警
func (m *QuotaManager) GetAlerts(limit int) []*QuotaAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}

	// 返回最近的告警
	start := len(m.alerts) - limit
	if start < 0 {
		start = 0
	}

	return m.alerts[start:]
}

// AcknowledgeAlert 确认告警
func (m *QuotaManager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == id {
			alert.Acknowledged = true
			return nil
		}
	}

	return nil
}

// GetStats 获取统计信息
func (m *QuotaManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_quotas":  len(m.quotas),
		"total_reports": len(m.reports),
		"total_alerts":  len(m.alerts),
		"by_type":       make(map[QuotaType]int),
		"by_status":     make(map[QuotaStatus]int),
	}

	byType := stats["by_type"].(map[QuotaType]int)
	byStatus := stats["by_status"].(map[QuotaStatus]int)

	for _, quota := range m.quotas {
		byType[quota.Type]++
		byStatus[quota.Status]++
	}

	return stats
}

// GetConfig 获取配置
func (m *QuotaManager) GetConfig() *QuotaConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置
func (m *QuotaManager) UpdateConfig(config *QuotaConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
}

// ExportJSON 导出为JSON
func (m *QuotaManager) ExportJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quotas := make([]*QuotaEntry, 0, len(m.quotas))
	for _, quota := range m.quotas {
		quotas = append(quotas, quota)
	}

	return json.Marshal(quotas)
}

// generateReportID 生成报告ID
func generateReportID() string {
	return "report-" + time.Now().Format("20060102150405")
}

// generateAlertID 生成告警ID
func generateAlertID() string {
	return "alert-" + time.Now().Format("20060102150405") + "-" + randomHex(4)
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, n)
	for i := range result {
		result[i] = hexChars[time.Now().UnixNano()%16]
	}
	return string(result)
}
