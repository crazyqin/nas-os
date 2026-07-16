// Package compliancereport 自动化合规审计模块
// 定时执行合规扫描，生成报告，发现违规立即通知
package compliance

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// AuditScheduleType 审计调度类型.
type AuditScheduleType string

const (
	ScheduleHourly  AuditScheduleType = "hourly"  // 每小时
	ScheduleDaily   AuditScheduleType = "daily"   // 每天
	ScheduleWeekly  AuditScheduleType = "weekly"  // 每周
	ScheduleMonthly AuditScheduleType = "monthly" // 每月
)

// AlertSeverity 告警严重程度.
type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

// AuditConfig 审计配置.
type AuditConfig struct {
	Enabled           bool                       `json:"enabled"`
	ScheduleType      AuditScheduleType          `json:"schedule_type"`
	ScheduleTime      string                     `json:"schedule_time"` // "02:00" for daily
	ScheduleDay       int                        `json:"schedule_day"`  // 0-6 for weekly, 1-31 for monthly
	Standards         []ComplianceStandard       `json:"standards"`
	BaselineStandards []SecurityBaselineStandard `json:"baseline_standards"`
	AlertOnViolation  bool                       `json:"alert_on_violation"`
	AlertThreshold    int                        `json:"alert_threshold"` // 分数低于此值触发告警
	RetentionDays     int                        `json:"retention_days"`  // 报告保留天数
}

// AuditResult 审计结果.
type AuditResult struct {
	ID                 string              `json:"id"`
	StartTime          time.Time           `json:"start_time"`
	EndTime            time.Time           `json:"end_time"`
	Duration           time.Duration       `json:"duration"`
	ComplianceReports  []*ComplianceReport `json:"compliance_reports,omitempty"`
	BaselineReports    []*BaselineReport   `json:"baseline_reports,omitempty"`
	OverallScore       int                 `json:"overall_score"`
	TotalViolations    int                 `json:"total_violations"`
	CriticalViolations int                 `json:"critical_violations"`
	Alerts             []AuditAlert        `json:"alerts,omitempty"`
	Status             ScanStatus          `json:"status"`
	Summary            string              `json:"summary"`
}

// AuditAlert 审计告警.
type AuditAlert struct {
	ID        string        `json:"id"`
	Severity  AlertSeverity `json:"severity"`
	Title     string        `json:"title"`
	Message   string        `json:"message"`
	CheckID   string        `json:"check_id,omitempty"`
	Standard  string        `json:"standard,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Notified  bool          `json:"notified"`
}

// AlertHandler 告警处理器接口.
type AlertHandler interface {
	HandleAlert(alert AuditAlert) error
}

// LogAlertHandler 日志告警处理器.
type LogAlertHandler struct{}

// HandleAlert 处理告警（记录日志）.
func (h *LogAlertHandler) HandleAlert(alert AuditAlert) error {
	log.Printf("[COMPLIANCE ALERT] [%s] %s: %s", alert.Severity, alert.Title, alert.Message)
	return nil
}

// AutoAuditEngine 自动化审计引擎.
type AutoAuditEngine struct {
	config          AuditConfig
	reportGen       *ReportGenerator
	baselineScanner *SecurityBaselineScanner
	alertHandlers   []AlertHandler
	results         []*AuditResult
	mu              sync.RWMutex
	stopCh          chan struct{}
	running         bool
}

// NewAutoAuditEngine 创建自动化审计引擎.
func NewAutoAuditEngine(config AuditConfig, reportGen *ReportGenerator) *AutoAuditEngine {
	engine := &AutoAuditEngine{
		config:          config,
		reportGen:       reportGen,
		baselineScanner: NewSecurityBaselineScanner(),
		alertHandlers:   []AlertHandler{&LogAlertHandler{}},
		results:         make([]*AuditResult, 0),
		stopCh:          make(chan struct{}),
	}

	// 设置默认值
	if engine.config.AlertThreshold == 0 {
		engine.config.AlertThreshold = 70
	}
	if engine.config.RetentionDays == 0 {
		engine.config.RetentionDays = 90
	}

	return engine
}

// RegisterAlertHandler 注册告警处理器.
func (e *AutoAuditEngine) RegisterAlertHandler(handler AlertHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alertHandlers = append(e.alertHandlers, handler)
}

// SetConfig 更新审计配置.
func (e *AutoAuditEngine) SetConfig(config AuditConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// GetConfig 获取审计配置.
func (e *AutoAuditEngine) GetConfig() AuditConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// Start 启动自动审计.
func (e *AutoAuditEngine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go e.run()
	log.Println("[AutoAudit] 自动合规审计引擎已启动")
}

// Stop 停止自动审计.
func (e *AutoAuditEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	close(e.stopCh)
	e.running = false
	log.Println("[AutoAudit] 自动合规审计引擎已停止")
}

// IsRunning 检查是否运行中.
func (e *AutoAuditEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// run 主运行循环.
func (e *AutoAuditEngine) run() {
	for {
		e.mu.RLock()
		config := e.config
		e.mu.RUnlock()

		if !config.Enabled {
			time.Sleep(1 * time.Minute)
			continue
		}

		nextRun := e.calculateNextRun(config)
		now := time.Now()

		if nextRun.Before(now) {
			// 如果错过了调度时间，立即执行
			nextRun = now
		}

		waitDuration := nextRun.Sub(now)
		log.Printf("[AutoAudit] 下次审计时间: %s (等待 %v)", nextRun.Format("2006-01-02 15:04:05"), waitDuration)

		select {
		case <-e.stopCh:
			return
		case <-time.After(waitDuration):
			e.executeAudit()
		}
	}
}

// calculateNextRun 计算下次运行时间.
func (e *AutoAuditEngine) calculateNextRun(config AuditConfig) time.Time {
	now := time.Now()

	switch config.ScheduleType {
	case ScheduleHourly:
		return now.Add(1 * time.Hour)

	case ScheduleDaily:
		hour, minute := 0, 0
		fmt.Sscanf(config.ScheduleTime, "%d:%d", &hour, &minute)
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next

	case ScheduleWeekly:
		hour, minute := 0, 0
		fmt.Sscanf(config.ScheduleTime, "%d:%d", &hour, &minute)
		targetDay := time.Weekday(config.ScheduleDay)
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		for next.Weekday() != targetDay || next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next

	case ScheduleMonthly:
		hour, minute := 0, 0
		fmt.Sscanf(config.ScheduleTime, "%d:%d", &hour, &minute)
		day := config.ScheduleDay
		if day < 1 {
			day = 1
		}
		if day > 28 {
			day = 28
		}
		next := time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, now.Location())
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
		return next

	default:
		return now.Add(24 * time.Hour)
	}
}

// ExecuteAudit 手动触发审计（公开方法）.
func (e *AutoAuditEngine) ExecuteAudit() *AuditResult {
	return e.executeAudit()
}

// executeAudit 执行审计.
func (e *AutoAuditEngine) executeAudit() *AuditResult {
	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	startTime := time.Now()
	log.Printf("[AutoAudit] 开始执行合规审计...")

	result := &AuditResult{
		ID:        GenerateID("audit"),
		StartTime: startTime,
		Status:    ScanStatusRunning,
	}

	ctx := context.Background()

	// 执行合规标准扫描
	for _, std := range config.Standards {
		report, err := e.reportGen.GenerateReport(ctx, ScanRequest{
			Standard: std,
		})
		if err != nil {
			log.Printf("[AutoAudit] 扫描标准 %s 失败: %v", std, err)
			continue
		}
		result.ComplianceReports = append(result.ComplianceReports, report)
	}

	// 执行安全基线扫描
	for _, baselineStd := range config.BaselineStandards {
		report := e.baselineScanner.GenerateBaselineReport(ctx, baselineStd, nil)
		result.BaselineReports = append(result.BaselineReports, report)
	}

	// 计算总体分数和违规
	e.calculateAuditResult(result)

	// 生成告警
	if config.AlertOnViolation {
		result.Alerts = e.generateAlerts(result, config.AlertThreshold)
		e.sendAlerts(result.Alerts)
	}

	// 完成
	endTime := time.Now()
	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Status = ScanStatusComplete
	result.Summary = e.generateAuditSummary(result)

	// 保存结果
	e.mu.Lock()
	e.results = append(e.results, result)
	// 清理过期报告
	e.cleanupOldResults()
	e.mu.Unlock()

	log.Printf("[AutoAudit] 审计完成: ID=%s, 耗时=%v, 分数=%d, 违规=%d",
		result.ID, result.Duration, result.OverallScore, result.TotalViolations)

	return result
}

// calculateAuditResult 计算审计结果.
func (e *AutoAuditEngine) calculateAuditResult(result *AuditResult) {
	totalScore := 0
	scoreCount := 0
	totalViolations := 0
	criticalViolations := 0

	// 合规报告统计
	for _, report := range result.ComplianceReports {
		totalScore += report.Score
		scoreCount++
		totalViolations += report.Failed
		for _, r := range report.Results {
			if r.Status == CheckItemFail && r.Severity == SeverityCritical {
				criticalViolations++
			}
		}
	}

	// 基线报告统计
	for _, report := range result.BaselineReports {
		totalScore += report.Score
		scoreCount++
		totalViolations += report.Failed
		for _, r := range report.Results {
			if r.Status == CheckItemFail && r.Severity == SeverityCritical {
				criticalViolations++
			}
		}
	}

	if scoreCount > 0 {
		result.OverallScore = totalScore / scoreCount
	}

	result.TotalViolations = totalViolations
	result.CriticalViolations = criticalViolations
}

// generateAlerts 生成告警.
func (e *AutoAuditEngine) generateAlerts(result *AuditResult, threshold int) []AuditAlert {
	var alerts []AuditAlert

	// 总体分数告警
	if result.OverallScore < threshold {
		alerts = append(alerts, AuditAlert{
			ID:        GenerateID("alert"),
			Severity:  AlertCritical,
			Title:     "合规分数低于阈值",
			Message:   fmt.Sprintf("当前合规分数 %d 低于阈值 %d，需要立即关注", result.OverallScore, threshold),
			Timestamp: time.Now(),
		})
	}

	// 关键违规告警
	if result.CriticalViolations > 0 {
		alerts = append(alerts, AuditAlert{
			ID:        GenerateID("alert"),
			Severity:  AlertCritical,
			Title:     "存在关键安全违规",
			Message:   fmt.Sprintf("检测到 %d 个关键安全违规项，需要立即修复", result.CriticalViolations),
			Timestamp: time.Now(),
		})
	}

	// 各合规报告的失败项告警
	for _, report := range result.ComplianceReports {
		for _, r := range report.Results {
			if r.Status == CheckItemFail {
				severity := AlertWarning
				if r.Severity == SeverityCritical {
					severity = AlertCritical
				}
				alerts = append(alerts, AuditAlert{
					ID:        GenerateID("alert"),
					Severity:  severity,
					Title:     fmt.Sprintf("合规检查失败: %s", r.Name),
					Message:   r.Message,
					CheckID:   r.CheckID,
					Standard:  string(report.Standard),
					Timestamp: time.Now(),
				})
			}
		}
	}

	// 基线报告的失败项告警
	for _, report := range result.BaselineReports {
		for _, r := range report.Results {
			if r.Status == CheckItemFail {
				severity := AlertWarning
				if r.Severity == SeverityCritical {
					severity = AlertCritical
				}
				alerts = append(alerts, AuditAlert{
					ID:        GenerateID("alert"),
					Severity:  severity,
					Title:     fmt.Sprintf("基线检查失败: %s", r.Name),
					Message:   r.Message,
					CheckID:   r.CheckID,
					Standard:  string(report.Standard),
					Timestamp: time.Now(),
				})
			}
		}
	}

	return alerts
}

// sendAlerts 发送告警.
func (e *AutoAuditEngine) sendAlerts(alerts []AuditAlert) {
	e.mu.RLock()
	handlers := e.alertHandlers
	e.mu.RUnlock()

	for _, alert := range alerts {
		for _, handler := range handlers {
			if err := handler.HandleAlert(alert); err != nil {
				log.Printf("[AutoAudit] 发送告警失败: %v", err)
			} else {
				alert.Notified = true
			}
		}
	}
}

// generateAuditSummary 生成审计摘要.
func (e *AutoAuditEngine) generateAuditSummary(result *AuditResult) string {
	statusDesc := "合规"
	if result.OverallScore < 60 {
		statusDesc = "不合规"
	} else if result.OverallScore < 90 {
		statusDesc = "待改进"
	}

	return fmt.Sprintf("自动合规审计完成: 总体得分 %d/100 (%s), 合规报告 %d 份, 基线报告 %d 份, 违规 %d 项 (关键 %d 项), 告警 %d 条, 耗时 %v",
		result.OverallScore, statusDesc,
		len(result.ComplianceReports), len(result.BaselineReports),
		result.TotalViolations, result.CriticalViolations,
		len(result.Alerts), result.Duration)
}

// cleanupOldResults 清理过期结果.
func (e *AutoAuditEngine) cleanupOldResults() {
	if e.config.RetentionDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -e.config.RetentionDays)
	var retained []*AuditResult

	for _, r := range e.results {
		if r.StartTime.After(cutoff) {
			retained = append(retained, r)
		}
	}

	removed := len(e.results) - len(retained)
	if removed > 0 {
		log.Printf("[AutoAudit] 清理 %d 条过期审计记录", removed)
	}

	e.results = retained
}

// GetResults 获取审计结果列表.
func (e *AutoAuditEngine) GetResults() []*AuditResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]*AuditResult, len(e.results))
	copy(results, e.results)
	return results
}

// GetLatestResult 获取最新的审计结果.
func (e *AutoAuditEngine) GetLatestResult() *AuditResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.results) == 0 {
		return nil
	}
	return e.results[len(e.results)-1]
}

// GetResultByID 根据 ID 获取审计结果.
func (e *AutoAuditEngine) GetResultByID(id string) (*AuditResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, r := range e.results {
		if r.ID == id {
			return r, true
		}
	}
	return nil, false
}

// GetAlerts 获取告警列表.
func (e *AutoAuditEngine) GetAlerts() []AuditAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var allAlerts []AuditAlert
	for _, r := range e.results {
		allAlerts = append(allAlerts, r.Alerts...)
	}
	return allAlerts
}

// GetAuditStatus 获取审计状态.
func (e *AutoAuditEngine) GetAuditStatus() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	config := e.config
	latest := e.GetLatestResult()

	status := map[string]interface{}{
		"enabled":            config.Enabled,
		"running":            e.running,
		"schedule_type":      config.ScheduleType,
		"schedule_time":      config.ScheduleTime,
		"total_results":      len(e.results),
		"standards":          config.Standards,
		"baseline_standards": config.BaselineStandards,
		"alert_threshold":    config.AlertThreshold,
	}

	if latest != nil {
		status["last_audit_id"] = latest.ID
		status["last_audit_time"] = latest.StartTime
		status["last_score"] = latest.OverallScore
		status["last_violations"] = latest.TotalViolations
	}

	return status
}

// DefaultAuditConfig 返回默认审计配置.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:      true,
		ScheduleType: ScheduleDaily,
		ScheduleTime: "02:00",
		Standards: []ComplianceStandard{
			StandardGDPR,
			StandardSOC2,
			StandardDJBH,
		},
		BaselineStandards: []SecurityBaselineStandard{
			BaselineCIS,
			BaselineNIST,
		},
		AlertOnViolation: true,
		AlertThreshold:   70,
		RetentionDays:    90,
	}
}
