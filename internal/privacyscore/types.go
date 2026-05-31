// Package privacyscore 隐私评分系统 - NAS隐私安全评估
// 扫描维度：文件权限、共享暴露、加密状态、密码策略、访问日志
package privacyscore

import (
	"fmt"
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

// ScanDimension 扫描维度
type ScanDimension string

const (
	DimFilePermission   ScanDimension = "file_permission"   // 文件权限
	DimSharingExposure  ScanDimension = "sharing_exposure"  // 共享暴露
	DimEncryptionStatus ScanDimension = "encryption_status" // 加密状态
	DimPasswordPolicy   ScanDimension = "password_policy"   // 密码策略
	DimAccessLog        ScanDimension = "access_log"        // 访问日志
)

// SuggestionPriority 建议优先级
type SuggestionPriority int

const (
	PriorityHigh   SuggestionPriority = 1 // 高优先级
	PriorityMedium SuggestionPriority = 2 // 中优先级
	PriorityLow    SuggestionPriority = 3 // 低优先级
)

// PrivacyRisk 隐私风险
type PrivacyRisk struct {
	ID          string         `json:"id"`
	Dimension   ScanDimension  `json:"dimension"`
	Level       RiskLevel      `json:"level"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Location    string         `json:"location,omitempty"` // 风险位置（文件路径、配置项等）
	DetectedAt  time.Time      `json:"detected_at"`
	Ignored     bool           `json:"ignored"`
}

// Suggestion 改进建议
type Suggestion struct {
	ID          string             `json:"id"`
	RiskID      string             `json:"risk_id"`
	Dimension   ScanDimension      `json:"dimension"`
	Priority    SuggestionPriority `json:"priority"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Action      string             `json:"action"` // 建议采取的行动
}

// DimensionScore 维度评分
type DimensionScore struct {
	Dimension   ScanDimension `json:"dimension"`
	Score       int           `json:"score"`        // 0-100
	MaxScore    int           `json:"max_score"`
	RiskCount   int           `json:"risk_count"`
	Description string        `json:"description"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID              string                    `json:"id"`
	TotalScore      int                       `json:"total_score"` // 0-100
	Grade           string                    `json:"grade"`       // A/B/C/D/F
	Dimensions      map[ScanDimension]*DimensionScore `json:"dimensions"`
	Risks           []*PrivacyRisk            `json:"risks"`
	Suggestions     []*Suggestion             `json:"suggestions"`
	ScannedAt       time.Time                 `json:"scanned_at"`
	Duration        time.Duration             `json:"duration_ms"`
}

// ScoreHistory 评分历史记录
type ScoreHistory struct {
	Timestamp time.Time `json:"timestamp"`
	Score     int       `json:"score"`
	Grade     string    `json:"grade"`
	RiskCount int       `json:"risk_count"`
}

// ScanSchedule 扫描计划
type ScanSchedule struct {
	Enabled    bool   `json:"enabled"`
	Interval   int    `json:"interval_hours"` // 扫描间隔（小时）
	CronExpr   string `json:"cron_expr,omitempty"`
	LastScan   *time.Time `json:"last_scan,omitempty"`
	NextScan   *time.Time `json:"next_scan,omitempty"`
}

// PrivacyReport 完整隐私报告
type PrivacyReport struct {
	Summary     *ScanResult              `json:"summary"`
	History     []ScoreHistory           `json:"history"`
	Trends      map[string][]int         `json:"trends"` // 维度 -> 历史分数
	Schedule    *ScanSchedule            `json:"schedule"`
	GeneratedAt time.Time                `json:"generated_at"`
}

// Manager 隐私评分管理器
type Manager struct {
	mu          sync.RWMutex
	latestScan  *ScanResult
	history     []ScoreHistory
	risks       map[string]*PrivacyRisk // id -> risk
	suggestions map[string]*Suggestion  // id -> suggestion
	schedule    *ScanSchedule
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		risks:       make(map[string]*PrivacyRisk),
		suggestions: make(map[string]*Suggestion),
		schedule: &ScanSchedule{
			Enabled:  true,
			Interval: 24, // 默认每天扫描一次
		},
	}
}

// RunScan 执行隐私扫描
func (m *Manager) RunScan() *ScanResult {
	start := time.Now()

	// 执行各维度扫描
	dimensions := make(map[ScanDimension]*DimensionScore)
	var allRisks []*PrivacyRisk
	var allSuggestions []*Suggestion

	// 文件权限扫描
	permScore, permRisks, permSuggestions := m.scanFilePermissions()
	dimensions[DimFilePermission] = permScore
	allRisks = append(allRisks, permRisks...)
	allSuggestions = append(allSuggestions, permSuggestions...)

	// 共享暴露扫描
	shareScore, shareRisks, shareSuggestions := m.scanSharingExposure()
	dimensions[DimSharingExposure] = shareScore
	allRisks = append(allRisks, shareRisks...)
	allSuggestions = append(allSuggestions, shareSuggestions...)

	// 加密状态扫描
	encScore, encRisks, encSuggestions := m.scanEncryptionStatus()
	dimensions[DimEncryptionStatus] = encScore
	allRisks = append(allRisks, encRisks...)
	allSuggestions = append(allSuggestions, encSuggestions...)

	// 密码策略扫描
	pwdScore, pwdRisks, pwdSuggestions := m.scanPasswordPolicy()
	dimensions[DimPasswordPolicy] = pwdScore
	allRisks = append(allRisks, pwdRisks...)
	allSuggestions = append(allSuggestions, pwdSuggestions...)

	// 访问日志扫描
	logScore, logRisks, logSuggestions := m.scanAccessLog()
	dimensions[DimAccessLog] = logScore
	allRisks = append(allRisks, logRisks...)
	allSuggestions = append(allSuggestions, logSuggestions...)

	// 计算总分（加权平均）
	totalScore := m.calculateTotalScore(dimensions)
	grade := calculateGrade(totalScore)

	// 存储结果
	m.mu.Lock()
	m.latestScan = &ScanResult{
		ID:         fmt.Sprintf("scan-%d", time.Now().UnixNano()),
		TotalScore: totalScore,
		Grade:      grade,
		Dimensions: dimensions,
		Risks:      allRisks,
		Suggestions: allSuggestions,
		ScannedAt:  start,
		Duration:   time.Since(start),
	}

	// 更新风险和建议
	for _, r := range allRisks {
		m.risks[r.ID] = r
	}
	for _, s := range allSuggestions {
		m.suggestions[s.ID] = s
	}

	// 记录历史
	m.history = append(m.history, ScoreHistory{
		Timestamp: start,
		Score:     totalScore,
		Grade:     grade,
		RiskCount: len(allRisks),
	})

	// 更新扫描计划
	now := time.Now()
	m.schedule.LastScan = &now
	next := now.Add(time.Duration(m.schedule.Interval) * time.Hour)
	m.schedule.NextScan = &next
	m.mu.Unlock()

	return m.latestScan
}

// GetCurrentScore 获取当前评分
func (m *Manager) GetCurrentScore() (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.latestScan == nil {
		return nil, fmt.Errorf("尚未执行扫描")
	}
	return m.latestScan, nil
}

// GetRisks 获取风险列表
func (m *Manager) GetRisks(includeIgnored bool) []*PrivacyRisk {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PrivacyRisk, 0, len(m.risks))
	for _, r := range m.risks {
		if includeIgnored || !r.Ignored {
			result = append(result, r)
		}
	}
	return result
}

// IgnoreRisk 忽略风险
func (m *Manager) IgnoreRisk(riskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	risk, exists := m.risks[riskID]
	if !exists {
		return fmt.Errorf("风险不存在: %s", riskID)
	}
	risk.Ignored = true
	return nil
}

// GetSuggestions 获取改进建议（按优先级排序）
func (m *Manager) GetSuggestions() []*Suggestion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Suggestion, 0, len(m.suggestions))
	for _, s := range m.suggestions {
		result = append(result, s)
	}

	// 按优先级排序（Priority 值越小越优先）
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Priority < result[i].Priority {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// GetHistory 获取评分历史
func (m *Manager) GetHistory() []ScoreHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.history
}

// ListReports 获取报告列表
func (m *Manager) ListReports() []*PrivacyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &PrivacyReport{
		Summary:     m.latestScan,
		History:     m.history,
		Schedule:    m.schedule,
		GeneratedAt: time.Now(),
	}
	return []*PrivacyReport{report}
}

// GetLatestReport 获取最新报告
func (m *Manager) GetLatestReport() (*PrivacyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.latestScan == nil {
		return nil, fmt.Errorf("尚未执行扫描")
	}
	return &PrivacyReport{
		Summary:     m.latestScan,
		History:     m.history,
		Schedule:    m.schedule,
		GeneratedAt: time.Now(),
	}, nil
}

// GetReport 获取指定报告（当前仅支持latest）
func (m *Manager) GetReport(id string) (*PrivacyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.latestScan == nil {
		return nil, fmt.Errorf("报告不存在: %s", id)
	}
	return &PrivacyReport{
		Summary:     m.latestScan,
		History:     m.history,
		Schedule:    m.schedule,
		GeneratedAt: time.Now(),
	}, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_scans":    len(m.history),
		"total_risks":    len(m.risks),
		"total_suggestions": len(m.suggestions),
	}
	if m.latestScan != nil {
		stats["latest_score"] = m.latestScan.TotalScore
		stats["latest_grade"] = m.latestScan.Grade
	}
	return stats
}

// SetSchedule 设置扫描计划
func (m *Manager) SetSchedule(intervalHours int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedule.Interval = intervalHours
	m.schedule.Enabled = intervalHours > 0
	if intervalHours > 0 {
		now := time.Now()
		next := now.Add(time.Duration(intervalHours) * time.Hour)
		m.schedule.NextScan = &next
	}
}

// GetSchedule 获取扫描计划
func (m *Manager) GetSchedule() *ScanSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.schedule
}

// GenerateReport 生成完整隐私报告
func (m *Manager) GenerateReport() *PrivacyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trends := make(map[string][]int)
	for _, h := range m.history {
		// 简化：将总体分数作为趋势数据
		trends["total"] = append(trends["total"], h.Score)
	}

	return &PrivacyReport{
		Summary:     m.latestScan,
		History:     m.history,
		Trends:      trends,
		Schedule:    m.schedule,
		GeneratedAt: time.Now(),
	}
}

// calculateTotalScore 计算加权总分
func (m *Manager) calculateTotalScore(dimensions map[ScanDimension]*DimensionScore) int {
	// 权重配置
	weights := map[ScanDimension]int{
		DimFilePermission:   20,
		DimSharingExposure:  25,
		DimEncryptionStatus: 25,
		DimPasswordPolicy:   20,
		DimAccessLog:        10,
	}

	totalWeight := 0
	weightedSum := 0
	for dim, w := range weights {
		if ds, ok := dimensions[dim]; ok {
			weightedSum += ds.Score * w
			totalWeight += w
		}
	}

	if totalWeight == 0 {
		return 0
	}
	return weightedSum / totalWeight
}

// calculateGrade 计算等级
func calculateGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

// === 扫描实现 ===

// scanFilePermissions 扫描文件权限
func (m *Manager) scanFilePermissions() (*DimensionScore, []*PrivacyRisk, []*Suggestion) {
	var risks []*PrivacyRisk
	var suggestions []*Suggestion

	// 模拟检测：检查敏感文件权限
	riskID := fmt.Sprintf("risk-fp-%d", time.Now().UnixNano())
	risks = append(risks, &PrivacyRisk{
		ID:          riskID,
		Dimension:   DimFilePermission,
		Level:       RiskHigh,
		Title:       "敏感文件权限过宽",
		Description: "发现 /shared/private 目录权限为 777，任何用户可读写",
		Location:    "/shared/private",
		DetectedAt:  time.Now(),
	})
	suggestions = append(suggestions, &Suggestion{
		ID:          fmt.Sprintf("sug-fp-%d", time.Now().UnixNano()),
		RiskID:      riskID,
		Dimension:   DimFilePermission,
		Priority:    PriorityHigh,
		Title:       "收紧敏感目录权限",
		Description: "将 /shared/private 目录权限修改为 750，仅允许属主和组访问",
		Action:      "chmod 750 /shared/private",
	})

	score := 60 // 权限有问题，扣分
	return &DimensionScore{
		Dimension:   DimFilePermission,
		Score:       score,
		MaxScore:    100,
		RiskCount:   len(risks),
		Description: "文件权限检查",
	}, risks, suggestions
}

// scanSharingExposure 扫描共享暴露
func (m *Manager) scanSharingExposure() (*DimensionScore, []*PrivacyRisk, []*Suggestion) {
	var risks []*PrivacyRisk
	var suggestions []*Suggestion

	// 模拟检测：公开共享
	riskID := fmt.Sprintf("risk-se-%d", time.Now().UnixNano())
	risks = append(risks, &PrivacyRisk{
		ID:          riskID,
		Dimension:   DimSharingExposure,
		Level:       RiskCritical,
		Title:       "存在公开共享文件夹",
		Description: "发现 3 个文件夹设置为公开共享，无需认证即可访问",
		Location:    "/shared/public",
		DetectedAt:  time.Now(),
	})
	suggestions = append(suggestions, &Suggestion{
		ID:          fmt.Sprintf("sug-se-%d", time.Now().UnixNano()),
		RiskID:      riskID,
		Dimension:   DimSharingExposure,
		Priority:    PriorityHigh,
		Title:       "关闭不必要的公开共享",
		Description: "将公开共享改为需要认证的私有共享，或设置访问密码",
		Action:      "关闭公开共享或添加访问控制",
	})

	score := 40 // 严重暴露
	return &DimensionScore{
		Dimension:   DimSharingExposure,
		Score:       score,
		MaxScore:    100,
		RiskCount:   len(risks),
		Description: "共享暴露检查",
	}, risks, suggestions
}

// scanEncryptionStatus 扫描加密状态
func (m *Manager) scanEncryptionStatus() (*DimensionScore, []*PrivacyRisk, []*Suggestion) {
	var risks []*PrivacyRisk
	var suggestions []*Suggestion

	// 模拟检测：未加密传输
	riskID := fmt.Sprintf("risk-es-%d", time.Now().UnixNano())
	risks = append(risks, &PrivacyRisk{
		ID:          riskID,
		Dimension:   DimEncryptionStatus,
		Level:       RiskHigh,
		Title:       "存在未加密传输通道",
		Description: "检测到 FTP 和 HTTP 服务仍在运行，数据传输未加密",
		Location:    "FTP/HTTP 服务",
		DetectedAt:  time.Now(),
	})
	suggestions = append(suggestions, &Suggestion{
		ID:          fmt.Sprintf("sug-es-%d", time.Now().UnixNano()),
		RiskID:      riskID,
		Dimension:   DimEncryptionStatus,
		Priority:    PriorityHigh,
		Title:       "启用加密传输",
		Description: "关闭 FTP 服务，使用 SFTP/FTPS 替代；启用 HTTPS 并配置 TLS 证书",
		Action:      "禁用 FTP，启用 SFTP 和 HTTPS",
	})

	score := 55
	return &DimensionScore{
		Dimension:   DimEncryptionStatus,
		Score:       score,
		MaxScore:    100,
		RiskCount:   len(risks),
		Description: "加密状态检查",
	}, risks, suggestions
}

// scanPasswordPolicy 扫描密码策略
func (m *Manager) scanPasswordPolicy() (*DimensionScore, []*PrivacyRisk, []*Suggestion) {
	var risks []*PrivacyRisk
	var suggestions []*Suggestion

	// 模拟检测：弱密码策略
	riskID := fmt.Sprintf("risk-pp-%d", time.Now().UnixNano())
	risks = append(risks, &PrivacyRisk{
		ID:          riskID,
		Dimension:   DimPasswordPolicy,
		Level:       RiskHigh,
		Title:       "密码策略过于宽松",
		Description: "当前密码最小长度为 4 位，未启用双因素认证",
		Location:    "系统认证设置",
		DetectedAt:  time.Now(),
	})
	suggestions = append(suggestions, &Suggestion{
		ID:          fmt.Sprintf("sug-pp-%d", time.Now().UnixNano()),
		RiskID:      riskID,
		Dimension:   DimPasswordPolicy,
		Priority:    PriorityHigh,
		Title:       "加强密码策略",
		Description: "设置密码最小长度为 8 位，要求包含大小写字母和数字，启用双因素认证",
		Action:      "修改密码策略，启用 2FA",
	})

	score := 50
	return &DimensionScore{
		Dimension:   DimPasswordPolicy,
		Score:       score,
		MaxScore:    100,
		RiskCount:   len(risks),
		Description: "密码策略检查",
	}, risks, suggestions
}

// scanAccessLog 扫描访问日志
func (m *Manager) scanAccessLog() (*DimensionScore, []*PrivacyRisk, []*Suggestion) {
	var risks []*PrivacyRisk
	var suggestions []*Suggestion

	// 模拟检测：异常访问
	riskID := fmt.Sprintf("risk-al-%d", time.Now().UnixNano())
	risks = append(risks, &PrivacyRisk{
		ID:          riskID,
		Dimension:   DimAccessLog,
		Level:       RiskMedium,
		Title:       "检测到异常登录尝试",
		Description: "过去 24 小时内检测到 15 次来自未知 IP 的登录失败",
		Location:    "访问日志",
		DetectedAt:  time.Now(),
	})
	suggestions = append(suggestions, &Suggestion{
		ID:          fmt.Sprintf("sug-al-%d", time.Now().UnixNano()),
		RiskID:      riskID,
		Dimension:   DimAccessLog,
		Priority:    PriorityMedium,
		Title:       "启用登录保护",
		Description: "配置登录失败锁定策略（5次失败后锁定30分钟），启用 IP 黑名单",
		Action:      "配置登录保护策略",
	})

	score := 70
	return &DimensionScore{
		Dimension:   DimAccessLog,
		Score:       score,
		MaxScore:    100,
		RiskCount:   len(risks),
		Description: "访问日志检查",
	}, risks, suggestions
}
