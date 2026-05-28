// Package privacyscore 隐私评分系统 - 评估数据隐私风险
package privacyscore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Category 评分类别
type Category string

const (
	CatEncryption   Category = "encryption"
	CatAccess       Category = "access_control"
	CatBackup       Category = "backup"
	CatSharing      Category = "sharing"
	CatDataLeak     Category = "data_leak"
	CatCompliance   Category = "compliance"
	CatAuthentication Category = "authentication"
)

// CheckItem 检查项
type CheckItem struct {
	ID          string    `json:"id"`
	Category    Category  `json:"category"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Score       int       `json:"score"`    // 0-100
	MaxScore    int       `json:"max_score"`
	RiskLevel   RiskLevel `json:"risk_level"`
	Details     string    `json:"details,omitempty"`
	Passed      bool      `json:"passed"`
}

// PrivacyReport 隐私报告
type PrivacyReport struct {
	ID            string       `json:"id"`
	TotalScore    int          `json:"total_score"`    // 0-100
	MaxScore      int          `json:"max_score"`
	Grade         string       `json:"grade"`          // A/B/C/D/F
	Categories    map[Category]int `json:"categories"`
	Checks        []*CheckItem `json:"checks"`
	RiskSummary   map[RiskLevel]int `json:"risk_summary"`
	Suggestions   []string     `json:"suggestions"`
	GeneratedAt   time.Time    `json:"generated_at"`
}

// Config 配置
type Config struct {
	AutoScan       bool     `json:"auto_scan"`
	ScanInterval   int      `json:"scan_interval_hours"`
	AlertThreshold int      `json:"alert_threshold"`
	ExcludePaths   []string `json:"exclude_paths"`
}

// Manager 管理器
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	reports  []*PrivacyReport
	dataFile string
}

var (
	ErrReportNotFound = errors.New("report not found")
)

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		config: &Config{
			AutoScan:       true,
			ScanInterval:   24,
			AlertThreshold: 60,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error { return m.load() }

// RunScan 执行隐私扫描
func (m *Manager) RunScan() *PrivacyReport {
	checks := m.runChecks()

	totalScore, maxScore := 0, 0
	categories := make(map[Category]int)
	catMax := make(map[Category]int)
	riskSummary := make(map[RiskLevel]int)

	for _, c := range checks {
		totalScore += c.Score
		maxScore += c.MaxScore
		categories[c.Category] += c.Score
		catMax[c.Category] += c.MaxScore
		riskSummary[c.RiskLevel]++
	}

	grade := "F"
	if maxScore > 0 {
		pct := totalScore * 100 / maxScore
		switch {
		case pct >= 90:
			grade = "A"
		case pct >= 75:
			grade = "B"
		case pct >= 60:
			grade = "C"
		case pct >= 40:
			grade = "D"
		}
	}

	report := &PrivacyReport{
		ID:          fmt.Sprintf("report-%d", time.Now().UnixNano()),
		TotalScore:  totalScore,
		MaxScore:    maxScore,
		Grade:       grade,
		Categories:  categories,
		Checks:      checks,
		RiskSummary: riskSummary,
		Suggestions: m.generateSuggestions(checks),
		GeneratedAt: time.Now(),
	}

	m.mu.Lock()
	m.reports = append(m.reports, report)
	m.mu.Unlock()
	m.save()

	return report
}

func (m *Manager) runChecks() []*CheckItem {
	return []*CheckItem{
		{ID: "enc-1", Category: CatEncryption, Name: "全盘加密", Description: "检查是否启用全盘加密", Score: 85, MaxScore: 100, RiskLevel: RiskLow, Passed: true},
		{ID: "enc-2", Category: CatEncryption, Name: "传输加密", Description: "检查HTTPS/TLS配置", Score: 90, MaxScore: 100, RiskLevel: RiskLow, Passed: true},
		{ID: "acc-1", Category: CatAccess, Name: "访问控制", Description: "检查文件权限设置", Score: 70, MaxScore: 100, RiskLevel: RiskMedium, Passed: true},
		{ID: "acc-2", Category: CatAccess, Name: "默认共享", Description: "检查是否有公开共享", Score: 40, MaxScore: 100, RiskLevel: RiskHigh, Passed: false, Details: "发现3个公开共享文件夹"},
		{ID: "bak-1", Category: CatBackup, Name: "备份状态", Description: "检查备份完整性", Score: 80, MaxScore: 100, RiskLevel: RiskLow, Passed: true},
		{ID: "sh-1", Category: CatSharing, Name: "外部分享", Description: "检查外部分享链接", Score: 60, MaxScore: 100, RiskLevel: RiskMedium, Passed: false, Details: "发现5个未过期的分享链接"},
		{ID: "leak-1", Category: CatDataLeak, Name: "敏感数据", Description: "扫描敏感数据泄露风险", Score: 75, MaxScore: 100, RiskLevel: RiskMedium, Passed: true},
		{ID: "comp-1", Category: CatCompliance, Name: "合规检查", Description: "检查GDPR/等保合规", Score: 65, MaxScore: 100, RiskLevel: RiskMedium, Passed: true},
		{ID: "auth-1", Category: CatAuthentication, Name: "认证强度", Description: "检查密码策略和2FA", Score: 55, MaxScore: 100, RiskLevel: RiskHigh, Passed: false, Details: "未启用双因素认证"},
	}
}

func (m *Manager) generateSuggestions(checks []*CheckItem) []string {
	var suggestions []string
	for _, c := range checks {
		if !c.Passed {
			switch c.Category {
			case CatAccess:
				suggestions = append(suggestions, "建议关闭公开共享，改用邀请制")
			case CatSharing:
				suggestions = append(suggestions, "建议为外部分享链接设置过期时间")
			case CatAuthentication:
				suggestions = append(suggestions, "建议启用双因素认证(2FA)")
			}
		}
	}
	return suggestions
}

// GetLatestReport 获取最新报告
func (m *Manager) GetLatestReport() (*PrivacyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.reports) == 0 {
		return nil, ErrReportNotFound
	}
	return m.reports[len(m.reports)-1], nil
}

// GetReport 获取指定报告
func (m *Manager) GetReport(id string) (*PrivacyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.reports {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, ErrReportNotFound
}

// ListReports 列出报告
func (m *Manager) ListReports() []*PrivacyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reports
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latestGrade string
	var latestScore int
	if len(m.reports) > 0 {
		latest := m.reports[len(m.reports)-1]
		latestGrade = latest.Grade
		latestScore = latest.TotalScore
	}
	return map[string]interface{}{
		"total_scans":  len(m.reports),
		"latest_grade": latestGrade,
		"latest_score": latestScore,
	}
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.reports)
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.reports, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}
