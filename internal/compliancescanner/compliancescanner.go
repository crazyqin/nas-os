// Package compliancescanner 提供安全合规扫描功能
package compliancescanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Engine 合规扫描引擎.
type Engine struct {
	mu             sync.RWMutex
	logger         *zap.Logger
	config         *ScanConfig
	ruleManager    *RuleManager
	scanner        *Scanner
	remediation    *RemediationEngine
	cron           *cron.Cron
	schedules      map[string]*ScanSchedule
	reports        map[string]*ComplianceReport
	stats          *ScanStats
	isRunning      bool
	cancelFunc     context.CancelFunc
}

// NewEngine 创建引擎.
func NewEngine(config *ScanConfig, logger *zap.Logger) *Engine {
	if config == nil {
		config = DefaultScanConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	scanner := NewScanner(logger, config.Timeout)
	ruleManager := NewRuleManager(logger)
	remediation := NewRemediationEngine(logger, scanner)

	return &Engine{
		logger:      logger,
		config:      config,
		ruleManager: ruleManager,
		scanner:     scanner,
		remediation: remediation,
		cron:        cron.New(),
		schedules:   make(map[string]*ScanSchedule),
		reports:     make(map[string]*ComplianceReport),
		stats: &ScanStats{
			TotalScans: 0,
		},
	}
}

// Start 启动引擎.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning {
		return fmt.Errorf("引擎已在运行中")
	}

	ctx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel
	e.isRunning = true

	e.cron.Start()

	e.logger.Info("合规扫描引擎已启动",
		zap.Any("standards", e.config.Standards),
		zap.Any("categories", e.config.Categories),
	)

	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return nil
	}

	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	e.cron.Stop()
	e.isRunning = false

	e.logger.Info("合规扫描引擎已停止")
	return nil
}

// RunScan 执行扫描.
func (e *Engine) RunScan(ctx context.Context, config *ScanConfig) (*ComplianceReport, error) {
	e.mu.Lock()
	if e.isRunning && e.cancelFunc != nil {
		// 使用引擎的 context
	}
	e.mu.Unlock()

	if config == nil {
		config = e.config
	}

	scanID := uuid.New().String()
	startTime := time.Now()

	e.logger.Info("开始执行合规扫描",
		zap.String("scan_id", scanID),
		zap.Any("standards", config.Standards),
	)

	// 获取要扫描的规则
	rules := e.getScanRules(config)
	if len(rules) == 0 {
		return nil, fmt.Errorf("没有找到要扫描的规则")
	}

	// 执行扫描
	results := make([]ScanResult, 0, len(rules))
	for _, rule := range rules {
		result, err := e.scanRule(ctx, scanID, rule)
		if err != nil {
			e.logger.Error("扫描规则失败",
				zap.String("rule_id", rule.ID),
				zap.Error(err),
			)
			continue
		}
		results = append(results, *result)
	}

	endTime := time.Now()

	// 生成报告
	report := e.generateReport(scanID, startTime, endTime, results, config)

	// 存储报告
	e.mu.Lock()
	e.reports[report.ID] = report
	e.stats.TotalScans++
	lastScanTime := endTime
	e.stats.LastScanTime = &lastScanTime
	// 计算平均分
	totalScore := 0.0
	for _, r := range e.reports {
		totalScore += r.OverallScore
	}
	e.stats.AverageScore = totalScore / float64(len(e.reports))
	// 计算通过率
	passRate := 0.0
	if report.TotalChecks > 0 {
		passRate = float64(report.PassedChecks) / float64(report.TotalChecks) * 100
	}
	e.stats.PassRate = passRate
	e.mu.Unlock()

	e.logger.Info("合规扫描完成",
		zap.String("scan_id", scanID),
		zap.Float64("score", report.OverallScore),
		zap.Int("total_checks", report.TotalChecks),
		zap.Int("passed", report.PassedChecks),
		zap.Int("failed", report.FailedChecks),
		zap.Duration("duration", report.Duration),
	)

	return report, nil
}

// getScanRules 获取要扫描的规则.
func (e *Engine) getScanRules(config *ScanConfig) []*ScanRule {
	rules := make([]*ScanRule, 0)

	for _, standard := range config.Standards {
		standardRules := e.ruleManager.GetRulesByStandard(standard)
		for _, rule := range standardRules {
			// 检查是否启用
			if !rule.Enabled && !config.IncludeDisabled {
				continue
			}

			// 检查类别过滤
			if len(config.Categories) > 0 {
				found := false
				for _, cat := range config.Categories {
					if rule.Category == cat {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// 检查跳过类别
			skip := false
			for _, cat := range config.SkipCategories {
				if rule.Category == cat {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			// 检查跳过规则
			for _, ruleID := range config.SkipRules {
				if rule.ID == ruleID {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			rules = append(rules, rule)
		}
	}

	return rules
}

// scanRule 扫描单个规则.
func (e *Engine) scanRule(ctx context.Context, scanID string, rule *ScanRule) (*ScanResult, error) {
	startTime := time.Now()

	result, err := e.scanner.ExecuteCheck(ctx, rule.CheckFunc)
	if err != nil {
		return nil, err
	}

	result.ID = uuid.New().String()
	result.ScanID = scanID
	result.RuleID = rule.ID
	result.RuleName = rule.Name
	result.Severity = rule.Severity
	result.Duration = time.Since(startTime)

	return result, nil
}

// generateReport 生成报告.
func (e *Engine) generateReport(scanID string, startTime, endTime time.Time, results []ScanResult, config *ScanConfig) *ComplianceReport {
	report := &ComplianceReport{
		ID:          uuid.New().String(),
		ScanID:      scanID,
		GeneratedAt: time.Now(),
		StartTime:   startTime,
		EndTime:     endTime,
		Duration:    endTime.Sub(startTime),
		Results:     results,
		Standards:   config.Standards,
	}

	// 统计结果
	categoryStats := make(map[ScanCategory]*CategorySummary)
	severityStats := make(map[SeverityLevel]*SeveritySummary)

	for _, result := range results {
		report.TotalChecks++

		switch result.Result {
		case ResultPass:
			report.PassedChecks++
		case ResultFail:
			report.FailedChecks++
		case ResultWarning:
			report.WarningChecks++
		case ResultSkip:
			report.SkippedChecks++
		case ResultError:
			report.ErrorChecks++
		}

		// 分类统计
		if _, exists := categoryStats[result.Category]; !exists {
			categoryStats[result.Category] = &CategorySummary{Category: result.Category}
		}
		catStat := categoryStats[result.Category]
		catStat.Total++
		switch result.Result {
		case ResultPass:
			catStat.Passed++
		case ResultFail:
			catStat.Failed++
		case ResultWarning:
			catStat.Warnings++
		}

		// 严重级别统计
		if _, exists := severityStats[result.Severity]; !exists {
			severityStats[result.Severity] = &SeveritySummary{Severity: result.Severity}
		}
		sevStat := severityStats[result.Severity]
		sevStat.Total++
		if result.Result == ResultFail {
			sevStat.Failed++
		} else if result.Result == ResultWarning {
			sevStat.Warnings++
		}
	}

	// 转换为切片
	report.CategorySummary = make([]CategorySummary, 0, len(categoryStats))
	for _, stat := range categoryStats {
		if stat.Total > 0 {
			stat.Score = float64(stat.Passed) / float64(stat.Total) * 100
		}
		report.CategorySummary = append(report.CategorySummary, *stat)
	}

	report.SeveritySummary = make([]SeveritySummary, 0, len(severityStats))
	for _, stat := range severityStats {
		report.SeveritySummary = append(report.SeveritySummary, *stat)
	}

	// 计算总体评分
	if report.TotalChecks > 0 {
		passWeight := float64(report.PassedChecks) * 1.0
		warnWeight := float64(report.WarningChecks) * 0.5
		report.OverallScore = (passWeight + warnWeight) / float64(report.TotalChecks) * 100
	}

	// 确定合规等级
	report.ComplianceLevel = e.calculateComplianceLevel(report.OverallScore)

	// 生成整改建议
	report.Recommendations = e.generateRecommendations(results)

	return report
}

// calculateComplianceLevel 计算合规等级.
func (e *Engine) calculateComplianceLevel(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	default:
		return "D"
	}
}

// generateRecommendations 生成整改建议.
func (e *Engine) generateRecommendations(results []ScanResult) []Recommendation {
	recommendations := make([]Recommendation, 0)
	recID := 1

	for _, result := range results {
		if result.Result == ResultFail || result.Result == ResultWarning {
			rec := Recommendation{
				ID:       fmt.Sprintf("rec-%d", recID),
				Category: result.Category,
				Title:    fmt.Sprintf("修复 %s", result.RuleName),
				Actions:  e.remediation.SuggestRemediation(&result),
			}

			switch result.Severity {
			case SeverityCritical:
				rec.Priority = SeverityCritical
			case SeverityHigh:
				rec.Priority = SeverityHigh
			case SeverityMedium:
				rec.Priority = SeverityMedium
			default:
				rec.Priority = SeverityLow
			}

			recommendations = append(recommendations, rec)
			recID++
		}
	}

	return recommendations
}

// GetReport 获取报告.
func (e *Engine) GetReport(id string) (*ComplianceReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report, exists := e.reports[id]
	if !exists {
		return nil, fmt.Errorf("报告不存在: %s", id)
	}
	return report, nil
}

// GetAllReports 获取所有报告.
func (e *Engine) GetAllReports() []*ComplianceReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	reports := make([]*ComplianceReport, 0, len(e.reports))
	for _, report := range e.reports {
		reports = append(reports, report)
	}
	return reports
}

// ScheduleScan 调度扫描.
func (e *Engine) ScheduleScan(schedule *ScanSchedule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	// 添加定时任务
	entryID, err := e.cron.AddFunc(schedule.CronExpr, func() {
		ctx := context.Background()
		config := &ScanConfig{
			Standards:  schedule.Standards,
			Categories: schedule.Categories,
		}
		report, err := e.RunScan(ctx, config)
		if err != nil {
			e.logger.Error("定时扫描失败", zap.String("schedule_id", schedule.ID), zap.Error(err))
			return
		}
		e.logger.Info("定时扫描完成",
			zap.String("schedule_id", schedule.ID),
			zap.String("report_id", report.ID),
			zap.Float64("score", report.OverallScore),
		)
	})

	if err != nil {
		return fmt.Errorf("添加定时任务失败: %v", err)
	}

	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now
	// 计算下次运行时间
	e.schedules[schedule.ID] = schedule

	e.logger.Info("添加扫描调度",
		zap.String("schedule_id", schedule.ID),
		zap.String("cron_expr", schedule.CronExpr),
		zap.Int("entry_id", int(entryID)),
	)

	return nil
}

// RemoveSchedule 移除调度.
func (e *Engine) RemoveSchedule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	schedule, exists := e.schedules[id]
	if !exists {
		return fmt.Errorf("调度不存在: %s", id)
	}

	// 这里应该移除 cron entry，但简化处理
	delete(e.schedules, id)

	e.logger.Info("移除扫描调度", zap.String("schedule_id", schedule.ID))
	return nil
}

// GetSchedule 获取调度.
func (e *Engine) GetSchedule(id string) (*ScanSchedule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	schedule, exists := e.schedules[id]
	if !exists {
		return nil, fmt.Errorf("调度不存在: %s", id)
	}
	return schedule, nil
}

// GetAllSchedules 获取所有调度.
func (e *Engine) GetAllSchedules() []*ScanSchedule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	schedules := make([]*ScanSchedule, 0, len(e.schedules))
	for _, schedule := range e.schedules {
		schedules = append(schedules, schedule)
	}
	return schedules
}

// Remediate 自动修复.
func (e *Engine) Remediate(ctx context.Context, result *ScanResult) (*RemediationRecord, error) {
	return e.remediation.AutoRemediate(ctx, result)
}

// GetRemediationEngine 获取修复引擎.
func (e *Engine) GetRemediationEngine() *RemediationEngine {
	return e.remediation
}

// GetRuleManager 获取规则管理器.
func (e *Engine) GetRuleManager() *RuleManager {
	return e.ruleManager
}

// GetScanner 获取扫描器.
func (e *Engine) GetScanner() *Scanner {
	return e.scanner
}

// GetStats 获取统计信息.
func (e *Engine) GetStats() *ScanStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// IsRunning 是否运行中.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isRunning
}
