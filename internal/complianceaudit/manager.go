// Package complianceaudit 提供合规审计功能
package complianceaudit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 合规审计管理器
type Manager struct {
	checks      map[string]ComplianceCheck
	config      *ScanConfig
	store       *Store
	logger      *zap.Logger
	mu          sync.RWMutex
	stopChan    chan struct{}
	running     bool
	lastScan    *ComplianceReport
	lastScanMu  sync.RWMutex
	scoreHistory []ScoreTrend
}

// NewManager 创建合规审计管理器
func NewManager(store *Store, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		checks: make(map[string]ComplianceCheck),
		config: &ScanConfig{
			Standards:     []ComplianceStandard{StandardGDPR, StandardMLPS2, StandardISO27001, StandardSOC2},
			Schedule:      "0 0 * * *", // 每天凌晨执行
			Enabled:       true,
			NotifyOnFail:  true,
			AutoRemediate: false,
		},
		store:        store,
		logger:       logger,
		stopChan:     make(chan struct{}),
		scoreHistory: make([]ScoreTrend, 0),
	}
}

// RegisterCheck 注册合规检查项
func (m *Manager) RegisterCheck(check ComplianceCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checks[check.Name()] = check
	m.logger.Info("registered compliance check",
		zap.String("name", check.Name()),
		zap.String("standard", string(check.Standard())),
	)
}

// UnregisterCheck 注销合规检查项
func (m *Manager) UnregisterCheck(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.checks, name)
}

// GetCheck 获取合规检查项
func (m *Manager) GetCheck(name string) (ComplianceCheck, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.checks[name]
	return c, ok
}

// ListChecks 列出所有已注册的检查项
func (m *Manager) ListChecks() []CheckInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	infos := make([]CheckInfo, 0, len(m.checks))
	for _, c := range m.checks {
		infos = append(infos, CheckInfo{
			Name:        c.Name(),
			Standard:    c.Standard(),
			Category:    c.Category(),
			Description: c.Description(),
		})
	}
	return infos
}

// CheckInfo 检查项信息
type CheckInfo struct {
	Name        string              `json:"name"`
	Standard    ComplianceStandard  `json:"standard"`
	Category    CheckCategory       `json:"category"`
	Description string              `json:"description"`
}

// UpdateConfig 更新扫描配置
func (m *Manager) UpdateConfig(cfg *ScanConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg == nil {
		return
	}
	m.config = cfg
}

// GetConfig 获取当前配置
func (m *Manager) GetConfig() *ScanConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := &ScanConfig{
		Schedule:      m.config.Schedule,
		Enabled:       m.config.Enabled,
		NotifyOnFail:  m.config.NotifyOnFail,
		AutoRemediate: m.config.AutoRemediate,
	}
	cp.Standards = make([]ComplianceStandard, len(m.config.Standards))
	copy(cp.Standards, m.config.Standards)
	if m.config.Categories != nil {
		cp.Categories = make([]CheckCategory, len(m.config.Categories))
		copy(cp.Categories, m.config.Categories)
	}
	return cp
}

// Start 启动定时扫描调度器
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	go m.runScheduler()
	m.logger.Info("compliance audit scheduler started")
}

// Stop 停止调度器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	m.logger.Info("compliance audit scheduler stopped")
}

// IsRunning 检查调度器是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// runScheduler 调度器主循环
func (m *Manager) runScheduler() {
	// 简化实现：每天执行一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 启动后立即执行一次
	m.RunFullScan(context.Background())

	for {
		select {
		case <-ticker.C:
			m.RunFullScan(context.Background())
		case <-m.stopChan:
			return
		}
	}
}

// RunFullScan 执行完整的合规扫描
func (m *Manager) RunFullScan(ctx context.Context) *ComplianceReport {
	m.mu.RLock()
	checks := make([]ComplianceCheck, 0, len(m.checks))
	for _, c := range m.checks {
		checks = append(checks, c)
	}
	config := m.GetConfig()
	m.mu.RUnlock()

	report := &ComplianceReport{
		ID:          fmt.Sprintf("report_%d", time.Now().Unix()),
		Title:       "合规审计报告",
		GeneratedAt: time.Now(),
		Period: ReportPeriod{
			Start: time.Now().AddDate(0, 0, -1),
			End:   time.Now(),
		},
		Summary:     &ReportSummary{},
		Standards:   make([]*StandardReport, 0),
		Findings:    make([]*Finding, 0),
		Remediations: make([]*RemediationItem, 0),
		Format:      FormatJSON,
	}

	// 按标准分组
	standardResults := make(map[ComplianceStandard][]*CheckResult)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, check := range checks {
		// 检查是否在配置的标准范围内
		if !m.isStandardEnabled(config, check.Standard()) {
			continue
		}

		wg.Add(1)
		go func(c ComplianceCheck) {
			defer wg.Done()

			result := m.runSingleCheck(ctx, c)

			mu.Lock()
			standardResults[c.Standard()] = append(standardResults[c.Standard()], result)

			// 统计
			report.Summary.TotalChecks++
			switch result.Status {
			case StatusPass:
				report.Summary.Passed++
			case StatusFail:
				report.Summary.Failed++
				finding := &Finding{
					ID:          fmt.Sprintf("finding_%s_%d", result.Name, time.Now().UnixNano()),
					Title:       result.Message,
					Description: fmt.Sprintf("检查项 %s 未通过", result.Name),
					RiskLevel:   result.RiskLevel,
					Standard:    result.Standard,
					Category:    result.Category,
					Status:      result.Status,
				}
				report.Findings = append(report.Findings, finding)

				// 添加整改建议
				remediation := c.GetRemediation(result)
				if remediation != nil {
					item := &RemediationItem{
						FindingID:   finding.ID,
						Title:       remediation.Title,
						Description: remediation.Description,
						Steps:       remediation.Steps,
						Priority:    remediation.Priority,
						Status:      "pending",
					}
					report.Remediations = append(report.Remediations, item)
				}
			case StatusWarn:
				report.Summary.Warnings++
			}
			mu.Unlock()
		}(check)
	}

	wg.Wait()

	// 生成标准报告
	for standard, results := range standardResults {
		standardReport := &StandardReport{
			Standard: standard,
			Checks:   results,
		}
		// 计算标准得分
		passCount := 0
		for _, r := range results {
			if r.Status == StatusPass {
				passCount++
			}
		}
		if len(results) > 0 {
			standardReport.Score = float64(passCount) / float64(len(results)) * 100
		}
		report.Standards = append(report.Standards, standardReport)
	}

	// 计算总分
	if report.Summary.TotalChecks > 0 {
		report.Summary.OverallScore = float64(report.Summary.Passed) / float64(report.Summary.TotalChecks) * 100
	}

	// 确定风险等级
	report.Summary.RiskLevel = m.calculateRiskLevel(report.Summary.OverallScore)

	// 持久化
	if m.store != nil {
		_ = m.store.SaveReport(report)
	}

	// 更新缓存
	m.lastScanMu.Lock()
	m.lastScan = report
	m.lastScanMu.Unlock()

	// 更新分数历史
	m.updateScoreHistory(report.Summary.OverallScore)

	return report
}

// RunStandardScan 执行指定标准的扫描
func (m *Manager) RunStandardScan(ctx context.Context, standard ComplianceStandard) *ComplianceReport {
	m.mu.RLock()
	checks := make([]ComplianceCheck, 0)
	for _, c := range m.checks {
		if c.Standard() == standard {
			checks = append(checks, c)
		}
	}
	m.mu.RUnlock()

	report := &ComplianceReport{
		ID:          fmt.Sprintf("report_%s_%d", standard, time.Now().Unix()),
		Title:       fmt.Sprintf("%s 合规审计报告", standard),
		GeneratedAt: time.Now(),
		Period: ReportPeriod{
			Start: time.Now().AddDate(0, 0, -1),
			End:   time.Now(),
		},
		Summary:     &ReportSummary{},
		Standards:   make([]*StandardReport, 0),
		Findings:    make([]*Finding, 0),
		Remediations: make([]*RemediationItem, 0),
		Format:      FormatJSON,
	}

	results := make([]*CheckResult, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, check := range checks {
		wg.Add(1)
		go func(c ComplianceCheck) {
			defer wg.Done()
			result := m.runSingleCheck(ctx, c)

			mu.Lock()
			results = append(results, result)
			report.Summary.TotalChecks++
			switch result.Status {
			case StatusPass:
				report.Summary.Passed++
			case StatusFail:
				report.Summary.Failed++
			case StatusWarn:
				report.Summary.Warnings++
			}
			mu.Unlock()
		}(check)
	}

	wg.Wait()

	// 标准报告
	standardReport := &StandardReport{
		Standard: standard,
		Checks:   results,
	}
	if len(results) > 0 {
		passCount := 0
		for _, r := range results {
			if r.Status == StatusPass {
				passCount++
			}
		}
		standardReport.Score = float64(passCount) / float64(len(results)) * 100
	}
	report.Standards = append(report.Standards, standardReport)

	if report.Summary.TotalChecks > 0 {
		report.Summary.OverallScore = float64(report.Summary.Passed) / float64(report.Summary.TotalChecks) * 100
	}
	report.Summary.RiskLevel = m.calculateRiskLevel(report.Summary.OverallScore)

	if m.store != nil {
		_ = m.store.SaveReport(report)
	}

	return report
}

// RunSingleCheck 执行单个检查
func (m *Manager) RunSingleCheck(ctx context.Context, name string) (*CheckResult, error) {
	m.mu.RLock()
	check, exists := m.checks[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("check %q not found", name)
	}

	return m.runSingleCheck(ctx, check), nil
}

// runSingleCheck 内部执行单个检查
func (m *Manager) runSingleCheck(ctx context.Context, check ComplianceCheck) *CheckResult {
	checkCtx := &CheckContext{
		Timeout: 30 * time.Second,
		Forced:  true,
	}

	start := time.Now()
	result := check.Check(checkCtx)
	result.Duration = time.Since(start)
	if result.Timestamp.IsZero() {
		result.Timestamp = time.Now()
	}

	return result
}

// GetLastReport 获取最近一次扫描报告
func (m *Manager) GetLastReport() *ComplianceReport {
	m.lastScanMu.RLock()
	defer m.lastScanMu.RUnlock()
	return m.lastScan
}

// GetComplianceScore 获取合规评分
func (m *Manager) GetComplianceScore() *ComplianceScore {
	m.mu.RLock()
	checks := make([]ComplianceCheck, 0, len(m.checks))
	for _, c := range m.checks {
		checks = append(checks, c)
	}
	m.mu.RUnlock()

	score := &ComplianceScore{
		ByStandard:  make(map[ComplianceStandard]float64),
		ByCategory:  make(map[CheckCategory]float64),
		Trend:       m.scoreHistory,
		LastUpdated: time.Now(),
	}

	// 如果有最近的报告，从报告计算
	report := m.GetLastReport()
	if report != nil && report.Summary != nil {
		score.Overall = report.Summary.OverallScore
		for _, sr := range report.Standards {
			score.ByStandard[sr.Standard] = sr.Score
		}
	}

	return score
}

// GetDashboard 获取仪表盘数据
func (m *Manager) GetDashboard() *DashboardData {
	report := m.GetLastReport()
	score := m.GetComplianceScore()

	dashboard := &DashboardData{
		Score:           score,
		RecentFindings:  make([]*Finding, 0),
		Trends:          m.scoreHistory,
		LastScanTime:    time.Time{},
		StandardsStatus: make(map[ComplianceStandard]StandardStatus),
	}

	if report != nil {
		dashboard.LastScanTime = report.GeneratedAt
		dashboard.RecentFindings = report.Findings
		if len(report.Findings) > 10 {
			dashboard.RecentFindings = report.Findings[:10]
		}

		// 统计整改项
		for _, r := range report.Remediations {
			if r.Status == "pending" || r.Status == "in_progress" {
				dashboard.ActiveRemediations++
			}
		}

		// 标准状态
		for _, sr := range report.Standards {
			passCount := 0
			for _, c := range sr.Checks {
				if c.Status == StatusPass {
					passCount++
				}
			}
			passRate := 0.0
			if len(sr.Checks) > 0 {
				passRate = float64(passCount) / float64(len(sr.Checks)) * 100
			}
			dashboard.StandardsStatus[sr.Standard] = StandardStatus{
				Score:       sr.Score,
				LastChecked: report.GeneratedAt,
				CheckCount:  len(sr.Checks),
				PassRate:    passRate,
			}
		}
	}

	return dashboard
}

// GetReports 获取历史报告
func (m *Manager) GetReports(limit int) ([]*ComplianceReport, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	return m.store.GetReports(limit)
}

// CollectAuditLog 收集审计日志
func (m *Manager) CollectAuditLog(log *AuditLog) error {
	if m.store == nil {
		return fmt.Errorf("store not configured")
	}
	return m.store.SaveAuditLog(log)
}

// GetAuditLogs 获取审计日志
func (m *Manager) GetAuditLogs(actor string, limit int) ([]*AuditLog, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	return m.store.GetAuditLogs(actor, limit)
}

// isStandardEnabled 检查标准是否在配置中启用
func (m *Manager) isStandardEnabled(config *ScanConfig, standard ComplianceStandard) bool {
	if len(config.Standards) == 0 {
		return true
	}
	for _, s := range config.Standards {
		if s == standard {
			return true
		}
	}
	return false
}

// calculateRiskLevel 根据得分计算风险等级
func (m *Manager) calculateRiskLevel(score float64) RiskLevel {
	switch {
	case score >= 90:
		return RiskLow
	case score >= 70:
		return RiskMedium
	case score >= 50:
		return RiskHigh
	default:
		return RiskCritical
	}
}

// updateScoreHistory 更新分数历史
func (m *Manager) updateScoreHistory(score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.scoreHistory = append(m.scoreHistory, ScoreTrend{
		Date:  time.Now(),
		Score: score,
	})

	// 保留最近30天
	if len(m.scoreHistory) > 30 {
		m.scoreHistory = m.scoreHistory[len(m.scoreHistory)-30:]
	}
}
