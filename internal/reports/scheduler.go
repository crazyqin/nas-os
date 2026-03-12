// Package reports 提供报表生成和管理功能
package reports

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ========== 定时任务调度器 ==========

// ScheduleManager 定时任务管理器
type ScheduleManager struct {
	mu              sync.RWMutex
	schedules       map[string]*ScheduledReport
	executions      map[string][]*ScheduledReportExecution
	cron            *cron.Cron
	entryMap        map[string]cron.EntryID
	generator       *ReportGenerator
	exporter        *Exporter
	dataDir         string
	notifyEmail     func(scheduleID string, emails []string, report *GeneratedReport, path string) error
	notifyWebhook   func(scheduleID string, webhooks []string, report *GeneratedReport, path string) error
}

// NewScheduleManager 创建定时任务管理器
func NewScheduleManager(generator *ReportGenerator, exporter *Exporter, dataDir string) *ScheduleManager {
	sm := &ScheduleManager{
		schedules:  make(map[string]*ScheduledReport),
		executions: make(map[string][]*ScheduledReportExecution),
		entryMap:   make(map[string]cron.EntryID),
		generator:  generator,
		exporter:   exporter,
		dataDir:    dataDir,
	}
	
	// 创建 cron 调度器，支持秒级
	sm.cron = cron.New(cron.WithSeconds(), cron.WithLocation(time.Local))
	
	os.MkdirAll(dataDir, 0755)
	sm.loadSchedules()
	
	return sm
}

// SetNotifyEmail 设置邮件通知函数
func (sm *ScheduleManager) SetNotifyEmail(fn func(scheduleID string, emails []string, report *GeneratedReport, path string) error) {
	sm.notifyEmail = fn
}

// SetNotifyWebhook 设置 Webhook 通知函数
func (sm *ScheduleManager) SetNotifyWebhook(fn func(scheduleID string, webhooks []string, report *GeneratedReport, path string) error) {
	sm.notifyWebhook = fn
}

// loadSchedules 加载定时任务
func (sm *ScheduleManager) loadSchedules() {
	files, err := filepath.Glob(filepath.Join(sm.dataDir, "schedule_*.json"))
	if err != nil {
		return
	}
	
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		var schedule ScheduledReport
		if err := json.Unmarshal(data, &schedule); err != nil {
			continue
		}
		
		sm.schedules[schedule.ID] = &schedule
		
		// 如果任务启用，重新调度
		if schedule.Enabled {
			sm.scheduleTask(&schedule)
		}
	}
}

// saveSchedule 保存定时任务
func (sm *ScheduleManager) saveSchedule(schedule *ScheduledReport) error {
	data, err := json.MarshalIndent(schedule, "", "  ")
	if err != nil {
		return err
	}
	
	path := filepath.Join(sm.dataDir, "schedule_"+schedule.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// deleteScheduleFile 删除定时任务文件
func (sm *ScheduleManager) deleteScheduleFile(id string) error {
	path := filepath.Join(sm.dataDir, "schedule_"+id+".json")
	return os.Remove(path)
}

// Start 启动调度器
func (sm *ScheduleManager) Start() {
	sm.cron.Start()
}

// Stop 停止调度器
func (sm *ScheduleManager) Stop() {
	sm.cron.Stop()
}

// CreateSchedule 创建定时任务
func (sm *ScheduleManager) CreateSchedule(input ScheduledReportInput, createdBy string) (*ScheduledReport, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// 验证报表或模板存在
	if input.ReportID != "" {
		if _, err := sm.generator.GetCustomReport(input.ReportID); err != nil {
			return nil, err
		}
	} else if input.TemplateID != "" {
		// 模板验证
	} else {
		return nil, errors.New("必须指定报表或模板")
	}
	
	// 生成 cron 表达式
	cronExpr := input.CronExpr
	if cronExpr == "" {
		cronExpr = sm.frequencyToCron(input.Frequency)
	}
	
	// 验证 cron 表达式
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return nil, ErrInvalidCronExpr
	}
	
	// 设置时区
	timezone := input.Timezone
	if timezone == "" {
		timezone = "Local"
	}
	
	now := time.Now()
	nextRun := schedule.Next(now)
	
	report := &ScheduledReport{
		ID:            uuid.New().String(),
		Name:          input.Name,
		Description:   input.Description,
		ReportID:      input.ReportID,
		TemplateID:    input.TemplateID,
		Frequency:     input.Frequency,
		CronExpr:      cronExpr,
		Timezone:      timezone,
		NextRun:       &nextRun,
		Enabled:       input.Enabled,
		ExportFormat:  input.ExportFormat,
		OutputPath:    input.OutputPath,
		NotifyEmail:   input.NotifyEmail,
		NotifyWebhook: input.NotifyWebhook,
		Retention:     input.Retention,
		CreatedAt:     now,
		UpdatedAt:     now,
		CreatedBy:     createdBy,
	}
	
	sm.schedules[report.ID] = report
	
	if err := sm.saveSchedule(report); err != nil {
		delete(sm.schedules, report.ID)
		return nil, err
	}
	
	// 如果启用，添加到调度器
	if report.Enabled {
		sm.scheduleTask(report)
	}
	
	return report, nil
}

// GetSchedule 获取定时任务
func (sm *ScheduleManager) GetSchedule(id string) (*ScheduledReport, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	schedule, exists := sm.schedules[id]
	if !exists {
		return nil, ErrScheduleNotFound
	}
	
	return schedule, nil
}

// ListSchedules 列出定时任务
func (sm *ScheduleManager) ListSchedules(enabled *bool) []*ScheduledReport {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	result := make([]*ScheduledReport, 0)
	for _, s := range sm.schedules {
		if enabled != nil && s.Enabled != *enabled {
			continue
		}
		result = append(result, s)
	}
	
	return result
}

// UpdateSchedule 更新定时任务
func (sm *ScheduleManager) UpdateSchedule(id string, input ScheduledReportInput) (*ScheduledReport, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	schedule, exists := sm.schedules[id]
	if !exists {
		return nil, ErrScheduleNotFound
	}
	
	// 如果修改了频率或 cron 表达式，需要重新验证
	cronExpr := input.CronExpr
	if cronExpr == "" {
		cronExpr = sm.frequencyToCron(input.Frequency)
	}
	
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronSchedule, err := parser.Parse(cronExpr)
	if err != nil {
		return nil, ErrInvalidCronExpr
	}
	
	// 停止旧任务
	sm.unscheduleTask(id)
	
	// 更新字段
	schedule.Name = input.Name
	schedule.Description = input.Description
	schedule.ReportID = input.ReportID
	schedule.TemplateID = input.TemplateID
	schedule.Frequency = input.Frequency
	schedule.CronExpr = cronExpr
	schedule.Timezone = input.Timezone
	schedule.ExportFormat = input.ExportFormat
	schedule.OutputPath = input.OutputPath
	schedule.NotifyEmail = input.NotifyEmail
	schedule.NotifyWebhook = input.NotifyWebhook
	schedule.Retention = input.Retention
	schedule.Enabled = input.Enabled
	schedule.UpdatedAt = time.Now()
	
	// 计算下次运行时间
	nextRun := cronSchedule.Next(time.Now())
	schedule.NextRun = &nextRun
	
	if err := sm.saveSchedule(schedule); err != nil {
		return nil, err
	}
	
	// 如果启用，重新调度
	if schedule.Enabled {
		sm.scheduleTask(schedule)
	}
	
	return schedule, nil
}

// DeleteSchedule 删除定时任务
func (sm *ScheduleManager) DeleteSchedule(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if _, exists := sm.schedules[id]; !exists {
		return ErrScheduleNotFound
	}
	
	// 停止任务
	sm.unscheduleTask(id)
	
	delete(sm.schedules, id)
	delete(sm.executions, id)
	
	return sm.deleteScheduleFile(id)
}

// EnableSchedule 启用定时任务
func (sm *ScheduleManager) EnableSchedule(id string) error {
	return sm.setScheduleEnabled(id, true)
}

// DisableSchedule 禁用定时任务
func (sm *ScheduleManager) DisableSchedule(id string) error {
	return sm.setScheduleEnabled(id, false)
}

// setScheduleEnabled 设置启用状态
func (sm *ScheduleManager) setScheduleEnabled(id string, enabled bool) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	schedule, exists := sm.schedules[id]
	if !exists {
		return ErrScheduleNotFound
	}
	
	if enabled && !schedule.Enabled {
		// 启用
		schedule.Enabled = true
		sm.scheduleTask(schedule)
	} else if !enabled && schedule.Enabled {
		// 禁用
		schedule.Enabled = false
		sm.unscheduleTask(id)
	}
	
	schedule.UpdatedAt = time.Now()
	return sm.saveSchedule(schedule)
}

// RunNow 立即执行定时任务
func (sm *ScheduleManager) RunNow(id string) (*ScheduledReportExecution, error) {
	sm.mu.RLock()
	schedule, exists := sm.schedules[id]
	if !exists {
		sm.mu.RUnlock()
		return nil, ErrScheduleNotFound
	}
	sm.mu.RUnlock()
	
	return sm.executeReport(schedule)
}

// scheduleTask 调度任务
func (sm *ScheduleManager) scheduleTask(schedule *ScheduledReport) {
	// 如果已存在，先移除
	if entryID, exists := sm.entryMap[schedule.ID]; exists {
		sm.cron.Remove(entryID)
	}
	
	// 添加新任务
	entryID, err := sm.cron.AddFunc(schedule.CronExpr, func() {
		sm.executeReport(schedule)
	})
	
	if err != nil {
		return
	}
	
	sm.entryMap[schedule.ID] = entryID
	
	// 更新下次运行时间
	entry := sm.cron.Entry(entryID)
	if entry.ID != 0 {
		nextRun := entry.Next
		schedule.NextRun = &nextRun
	}
}

// unscheduleTask 取消调度
func (sm *ScheduleManager) unscheduleTask(id string) {
	if entryID, exists := sm.entryMap[id]; exists {
		sm.cron.Remove(entryID)
		delete(sm.entryMap, id)
	}
}

// executeReport 执行报表生成
func (sm *ScheduleManager) executeReport(schedule *ScheduledReport) (*ScheduledReportExecution, error) {
	now := time.Now()
	execution := &ScheduledReportExecution{
		ID:         uuid.New().String(),
		ScheduleID: schedule.ID,
		StartedAt:  now,
		Status:     "running",
	}
	
	sm.mu.Lock()
	sm.executions[schedule.ID] = append(sm.executions[schedule.ID], execution)
	schedule.LastRun = &now
	sm.mu.Unlock()
	
	defer func() {
		sm.mu.Lock()
		schedule.LastStatus = execution.Status
		if execution.CompletedAt != nil {
			schedule.LastRun = execution.CompletedAt
		}
		sm.saveSchedule(schedule)
		sm.mu.Unlock()
	}()
	
	// 生成报表
	var report *GeneratedReport
	var err error
	
	if schedule.ReportID != "" {
		report, err = sm.generator.GenerateFromCustomReport(schedule.ReportID, nil, nil)
	} else if schedule.TemplateID != "" {
		report, err = sm.generator.GenerateFromTemplate(schedule.TemplateID, nil, nil)
	} else {
		err = errors.New("未指定报表或模板")
	}
	
	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
		completedAt := time.Now()
		execution.CompletedAt = &completedAt
		return execution, err
	}
	
	// 设置报表名称
	report.Name = schedule.Name
	
	// 确定输出格式
	format := schedule.ExportFormat
	if format == "" {
		format = ExportJSON
	}
	
	// 确定输出路径
	outputPath := schedule.OutputPath
	if outputPath == "" {
		outputPath = sm.getDefaultOutputPath(schedule.ID, format)
	}
	
	// 导出报表
	exportResult, err := sm.exporter.Export(report, format, outputPath, ExportOptions{
		Title: schedule.Name,
	})
	
	if err != nil {
		execution.Status = "failed"
		execution.Error = fmt.Sprintf("导出失败: %v", err)
		completedAt := time.Now()
		execution.CompletedAt = &completedAt
		return execution, err
	}
	
	execution.OutputPath = exportResult.Path
	execution.OutputSize = exportResult.Size
	execution.RecordsCount = report.TotalRecords
	
	// 发送通知
	if len(schedule.NotifyEmail) > 0 && sm.notifyEmail != nil {
		sm.notifyEmail(schedule.ID, schedule.NotifyEmail, report, exportResult.Path) //nolint:errcheck
	}
	
	if len(schedule.NotifyWebhook) > 0 && sm.notifyWebhook != nil {
		sm.notifyWebhook(schedule.ID, schedule.NotifyWebhook, report, exportResult.Path) //nolint:errcheck
	}
	
	// 清理旧文件
	if schedule.Retention > 0 {
		sm.cleanupOldFiles(schedule.ID, schedule.Retention)
	}
	
	completedAt := time.Now()
	execution.Status = "success"
	execution.CompletedAt = &completedAt
	
	return execution, nil
}

// GetExecutions 获取执行记录
func (sm *ScheduleManager) GetExecutions(scheduleID string, limit int) []*ScheduledReportExecution {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	executions, exists := sm.executions[scheduleID]
	if !exists {
		return []*ScheduledReportExecution{}
	}
	
	if limit > 0 && len(executions) > limit {
		return executions[len(executions)-limit:]
	}
	
	return executions
}

// frequencyToCron 将频率转换为 cron 表达式
func (sm *ScheduleManager) frequencyToCron(frequency ScheduleFrequency) string {
	switch frequency {
	case FrequencyHourly:
		return "0 0 * * * *" // 每小时整点
	case FrequencyDaily:
		return "0 0 9 * * *" // 每天 9:00
	case FrequencyWeekly:
		return "0 0 9 * * 1" // 每周一 9:00
	case FrequencyMonthly:
		return "0 0 9 1 * *" // 每月 1 日 9:00
	default:
		return "0 0 9 * * *" // 默认每天
	}
}

// getDefaultOutputPath 获取默认输出路径
func (sm *ScheduleManager) getDefaultOutputPath(scheduleID string, format ExportFormat) string {
	timestamp := time.Now().Format("20060102_150405")
	ext := string(format)
	return filepath.Join(sm.dataDir, "outputs", scheduleID, fmt.Sprintf("report_%s.%s", timestamp, ext))
}

// cleanupOldFiles 清理旧文件
func (sm *ScheduleManager) cleanupOldFiles(scheduleID string, retentionDays int) {
	outputDir := filepath.Join(sm.dataDir, "outputs", scheduleID)
	
	files, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return
	}
	
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		
		if info.ModTime().Before(cutoff) {
			os.Remove(file)
		}
	}
}