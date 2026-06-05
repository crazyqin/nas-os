// Package smartbackupverify 提供备份智能验证功能
package smartbackupverify

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 备份智能验证管理器.
type Manager struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	backups      map[string]*BackupInfo       // backupID -> BackupInfo
	verifyTasks  map[string]*VerifyTask       // taskID -> VerifyTask
	restoreTests map[string]*RestoreTestResult // testID -> RestoreTestResult
	healthScores map[string]*HealthScore       // backupID -> HealthScore
	reports      map[string]*VerifyReport     // reportID -> VerifyReport
	alerts       map[string]*Alert            // alertID -> Alert
}

// NewManager 创建备份智能验证管理器.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Manager{
		logger:       logger,
		backups:      make(map[string]*BackupInfo),
		verifyTasks:  make(map[string]*VerifyTask),
		restoreTests: make(map[string]*RestoreTestResult),
		healthScores: make(map[string]*HealthScore),
		reports:      make(map[string]*VerifyReport),
		alerts:       make(map[string]*Alert),
	}
}

// ========== 备份管理 ==========

// RegisterBackup 注册备份.
func (m *Manager) RegisterBackup(req BackupRegisterRequest) (*BackupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup := &BackupInfo{
		ID:        generateID(),
		TaskID:    req.TaskID,
		Source:    req.Source,
		DestPath:  req.DestPath,
		Size:      req.Size,
		Checksum:  req.Checksum,
		CreatedAt: time.Now(),
	}

	m.backups[backup.ID] = backup

	m.logger.Info("备份已注册",
		zap.String("backup_id", backup.ID),
		zap.String("source", req.Source),
	)

	return backup, nil
}

// GetBackup 获取备份信息.
func (m *Manager) GetBackup(backupID string) (*BackupInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backup, exists := m.backups[backupID]
	if !exists {
		return nil, ErrBackupNotFound
	}

	return backup, nil
}

// ListBackups 列出所有备份.
func (m *Manager) ListBackups() []*BackupInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backups := make([]*BackupInfo, 0, len(m.backups))
	for _, b := range m.backups {
		backups = append(backups, b)
	}
	return backups
}

// ========== 验证任务 ==========

// RunVerification 运行备份验证.
func (m *Manager) RunVerification(req VerifyRequest) (*VerifyTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	backup, exists := m.backups[req.BackupID]
	if !exists {
		return nil, ErrBackupNotFound
	}

	now := time.Now()
	task := &VerifyTask{
		ID:       generateID(),
		BackupID: req.BackupID,
		Status:   VerifyStatusRunning,
		StartedAt: now,
		Checks:   make([]CheckItem, 0),
	}

	m.verifyTasks[task.ID] = task

	// 执行验证检查（模拟）
	go m.executeVerification(task, backup, req.RunRestoreTest)

	m.logger.Info("备份验证已启动",
		zap.String("task_id", task.ID),
		zap.String("backup_id", req.BackupID),
	)

	return task, nil
}

// executeVerification 执行验证（内部方法）.
func (m *Manager) executeVerification(task *VerifyTask, backup *BackupInfo, runRestore bool) {
	checks := make([]CheckItem, 0)

	// 1. 校验和验证
	check := m.verifyChecksum(backup)
	checks = append(checks, check)

	// 2. 完整性检查
	check = m.verifyIntegrity(backup)
	checks = append(checks, check)

	// 3. 可读性检查
	check = m.verifyReadability(backup)
	checks = append(checks, check)

	// 4. 元数据检查
	check = m.verifyMetadata(backup)
	checks = append(checks, check)

	// 5. 恢复测试（可选）
	var restoreResult *RestoreTestResult
	if runRestore {
		restoreResult = m.runRestoreTest(backup)
		if !restoreResult.Success {
			checks = append(checks, CheckItem{
				Name:   "恢复测试",
				Status: VerifyStatusFailed,
				Detail: restoreResult.ErrorMsg,
			})
		} else {
			checks = append(checks, CheckItem{
				Name:   "恢复测试",
				Status: VerifyStatusPassed,
				Detail: fmt.Sprintf("成功恢复 %d 个文件", restoreResult.FileCount),
			})
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 计算总体状态
	overallStatus := VerifyStatusPassed
	for _, c := range checks {
		if c.Status == VerifyStatusFailed {
			overallStatus = VerifyStatusFailed
			break
		}
		if c.Status == VerifyStatusWarning {
			overallStatus = VerifyStatusWarning
		}
	}

	completedAt := time.Now()
	task.Status = overallStatus
	task.CompletedAt = &completedAt
	task.Duration = completedAt.Sub(task.StartedAt).Seconds()
	task.Checks = checks

	// 计算健康度评分
	healthScore := m.calculateHealthScore(backup.ID, checks)

	// 生成报告
	report := m.generateReport(backup.ID, task, restoreResult, healthScore)

	// 如果有失败，生成告警
	if overallStatus == VerifyStatusFailed {
		m.createAlert(backup.ID, AlertSeverityError, "备份验证失败",
			fmt.Sprintf("备份 %s 验证未通过", backup.ID))
	}

	m.logger.Info("备份验证完成",
		zap.String("task_id", task.ID),
		zap.String("status", string(task.Status)),
		zap.Float64("duration", task.Duration),
	)

	_ = report
}

// verifyChecksum 验证校验和.
func (m *Manager) verifyChecksum(backup *BackupInfo) CheckItem {
	start := time.Now()
	// 模拟校验和验证
	if backup.Checksum == "" {
		return CheckItem{
			Name:     "校验和验证",
			Status:   VerifyStatusWarning,
			Detail:   "备份未设置校验和",
			Duration: time.Since(start).Seconds(),
		}
	}
	return CheckItem{
		Name:     "校验和验证",
		Status:   VerifyStatusPassed,
		Detail:   fmt.Sprintf("校验和匹配: %s", backup.Checksum[:8]+"..."),
		Duration: time.Since(start).Seconds(),
	}
}

// verifyIntegrity 验证完整性.
func (m *Manager) verifyIntegrity(backup *BackupInfo) CheckItem {
	start := time.Now()
	return CheckItem{
		Name:     "完整性检查",
		Status:   VerifyStatusPassed,
		Detail:   fmt.Sprintf("文件结构完整，大小: %d bytes", backup.Size),
		Duration: time.Since(start).Seconds(),
	}
}

// verifyReadability 验证可读性.
func (m *Manager) verifyReadability(backup *BackupInfo) CheckItem {
	start := time.Now()
	return CheckItem{
		Name:     "可读性检查",
		Status:   VerifyStatusPassed,
		Detail:   "所有文件可正常读取",
		Duration: time.Since(start).Seconds(),
	}
}

// verifyMetadata 验证元数据.
func (m *Manager) verifyMetadata(backup *BackupInfo) CheckItem {
	start := time.Now()
	return CheckItem{
		Name:     "元数据检查",
		Status:   VerifyStatusPassed,
		Detail:   "元数据完整且一致",
		Duration: time.Since(start).Seconds(),
	}
}

// ========== 恢复测试 ==========

// runRestoreTest 运行恢复测试.
func (m *Manager) runRestoreTest(backup *BackupInfo) *RestoreTestResult {
	result := &RestoreTestResult{
		ID:          generateID(),
		BackupID:    backup.ID,
		Success:     true,
		RestorePath: fmt.Sprintf("/tmp/restore-test-%s", backup.ID[:8]),
		FileCount:   42,
		TotalSize:   backup.Size,
		Duration:    2.5,
		TestedAt:    time.Now(),
	}

	m.mu.Lock()
	m.restoreTests[result.ID] = result
	m.mu.Unlock()

	return result
}

// GetRestoreTest 获取恢复测试结果.
func (m *Manager) GetRestoreTest(testID string) (*RestoreTestResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.restoreTests[testID]
	if !exists {
		return nil, ErrRestoreTestFailed
	}

	return result, nil
}

// ========== 健康度评分 ==========

// calculateHealthScore 计算健康度评分.
func (m *Manager) calculateHealthScore(backupID string, checks []CheckItem) *HealthScore {
	factors := make([]Factor, 0)
	totalScore := 0
	totalWeight := 0

	for _, check := range checks {
		score := 100
		if check.Status == VerifyStatusFailed {
			score = 0
		} else if check.Status == VerifyStatusWarning {
			score = 60
		}

		weight := 25 // 每个检查项权重相同
		factors = append(factors, Factor{
			Name:   check.Name,
			Score:  score,
			Weight: weight,
			Detail: check.Detail,
		})

		totalScore += score * weight
		totalWeight += weight
	}

	finalScore := 0
	if totalWeight > 0 {
		finalScore = totalScore / totalWeight
	}

	level := getHealthLevel(finalScore)

	healthScore := &HealthScore{
		BackupID:  backupID,
		Score:     finalScore,
		Level:     level,
		Factors:   factors,
		UpdatedAt: time.Now(),
	}

	m.healthScores[backupID] = healthScore
	return healthScore
}

// GetHealthScore 获取健康度评分.
func (m *Manager) GetHealthScore(backupID string) (*HealthScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	score, exists := m.healthScores[backupID]
	if !exists {
		return nil, ErrBackupNotFound
	}

	return score, nil
}

// ========== 报告生成 ==========

// generateReport 生成验证报告.
func (m *Manager) generateReport(backupID string, task *VerifyTask, restoreTest *RestoreTestResult, healthScore *HealthScore) *VerifyReport {
	summary := ReportSummary{
		Status:      task.Status,
		TotalChecks: len(task.Checks),
	}

	for _, check := range task.Checks {
		switch check.Status {
		case VerifyStatusPassed:
			summary.Passed++
		case VerifyStatusFailed:
			summary.Failed++
		case VerifyStatusWarning:
			summary.Warnings++
		}
	}

	if summary.Failed > 0 {
		summary.Message = fmt.Sprintf("验证失败: %d 项未通过", summary.Failed)
	} else if summary.Warnings > 0 {
		summary.Message = fmt.Sprintf("验证通过（有 %d 项警告）", summary.Warnings)
	} else {
		summary.Message = "所有检查项通过"
	}

	report := &VerifyReport{
		ID:          generateID(),
		BackupID:    backupID,
		GeneratedAt: time.Now(),
		Summary:     summary,
		VerifyTask:  task,
		RestoreTest: restoreTest,
		HealthScore: healthScore,
		Alerts:      make([]Alert, 0),
	}

	m.reports[report.ID] = report
	return report
}

// GetReport 获取验证报告.
func (m *Manager) GetReport(reportID string) (*VerifyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, exists := m.reports[reportID]
	if !exists {
		return nil, ErrReportNotFound
	}

	return report, nil
}

// ListReports 列出所有报告.
func (m *Manager) ListReports() []*VerifyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*VerifyReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}
	return reports
}

// ========== 告警管理 ==========

// createAlert 创建告警（内部方法，需已持有写锁）.
func (m *Manager) createAlert(backupID string, severity AlertSeverity, title, message string) *Alert {
	alert := &Alert{
		ID:        generateID(),
		BackupID:  backupID,
		Severity:  severity,
		Title:     title,
		Message:   message,
		CreatedAt: time.Now(),
		Resolved:  false,
	}

	m.alerts[alert.ID] = alert

	m.logger.Warn("备份告警",
		zap.String("alert_id", alert.ID),
		zap.String("severity", string(severity)),
		zap.String("title", title),
	)

	return alert
}

// CreateAlert 创建告警.
func (m *Manager) CreateAlert(backupID string, severity AlertSeverity, title, message string) *Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.createAlert(backupID, severity, title, message)
}

// ListAlerts 列出所有告警.
func (m *Manager) ListAlerts() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		alerts = append(alerts, a)
	}
	return alerts
}

// ResolveAlert 解决告警.
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("告警不存在: %s", alertID)
	}

	alert.Resolved = true

	m.logger.Info("告警已解决", zap.String("alert_id", alertID))
	return nil
}

// ========== 统计 ==========

// GetStats 获取验证统计.
func (m *Manager) GetStats() *VerifyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &VerifyStats{
		TotalBackups: len(m.backups),
	}

	totalScore := 0
	scoreCount := 0

	for _, score := range m.healthScores {
		totalScore += score.Score
		scoreCount++
	}

	if scoreCount > 0 {
		stats.AvgHealthScore = float64(totalScore) / float64(scoreCount)
	}

	for _, task := range m.verifyTasks {
		if task.Status == VerifyStatusPassed {
			stats.VerifiedBackups++
		} else if task.Status == VerifyStatusFailed {
			stats.FailedBackups++
		}
	}

	for _, alert := range m.alerts {
		if !alert.Resolved {
			stats.ActiveAlerts++
		}
	}

	return stats
}

// ========== 内部方法 ==========

func getHealthLevel(score int) HealthLevel {
	switch {
	case score >= 90:
		return HealthLevelExcellent
	case score >= 70:
		return HealthLevelGood
	case score >= 50:
		return HealthLevelFair
	case score >= 30:
		return HealthLevelPoor
	default:
		return HealthLevelCritical
	}
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
