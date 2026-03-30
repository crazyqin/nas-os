// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Service 报表服务.
type Service struct {
	TemplateManager *TemplateManager
	Generator       *ReportGenerator
	ScheduleManager *ScheduleManager
	Exporter        *Exporter
	Handlers        *Handlers

	dataDir string
	mu      sync.RWMutex
}

// Config 服务配置.
type Config struct {
	DataDir string `json:"data_dir"`
}

// NewService 创建报表服务.
func NewService(config Config) (*Service, error) {
	dataDir := config.DataDir
	if dataDir == "" {
		dataDir = "/var/lib/nas-os/reports"
	}

	// 确保目录存在
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 创建子目录
	subDirs := []string{"templates", "custom", "schedules", "outputs"}
	for _, subDir := range subDirs {
		if err := os.MkdirAll(filepath.Join(dataDir, subDir), 0750); err != nil {
			return nil, fmt.Errorf("创建子目录 %s 失败: %w", subDir, err)
		}
	}

	// 创建模板管理器
	templateManager, err := NewTemplateManager(filepath.Join(dataDir, "templates"))
	if err != nil {
		return nil, fmt.Errorf("创建模板管理器失败: %w", err)
	}

	// 创建导出器
	exporter := NewExporter(filepath.Join(dataDir, "outputs"))

	// 创建报表生成器
	generator := NewReportGenerator(templateManager, filepath.Join(dataDir, "custom"))

	// 创建定时任务管理器
	scheduleManager := NewScheduleManager(generator, exporter, filepath.Join(dataDir, "schedules"))

	// 创建处理器
	handlers := NewHandlers(templateManager, generator, scheduleManager, exporter)

	service := &Service{
		TemplateManager: templateManager,
		Generator:       generator,
		ScheduleManager: scheduleManager,
		Exporter:        exporter,
		Handlers:        handlers,
		dataDir:         dataDir,
	}

	return service, nil
}

// Start 启动服务.
func (s *Service) Start() error {
	// 启动定时任务调度器
	s.ScheduleManager.Start()
	return nil
}

// Stop 停止服务.
func (s *Service) Stop() {
	// 停止定时任务调度器
	s.ScheduleManager.Stop()
}

// RegisterDataSource 注册数据源.
func (s *Service) RegisterDataSource(source DataSource) {
	s.Generator.RegisterDataSource(source)
}

// SetNotifyEmail 设置邮件通知回调.
func (s *Service) SetNotifyEmail(fn func(scheduleID string, emails []string, report *GeneratedReport, path string) error) {
	s.ScheduleManager.SetNotifyEmail(fn)
}

// SetNotifyWebhook 设置 Webhook 通知回调.
func (s *Service) SetNotifyWebhook(fn func(scheduleID string, webhooks []string, report *GeneratedReport, path string) error) {
	s.ScheduleManager.SetNotifyWebhook(fn)
}

// GetStats 获取服务统计.
func (s *Service) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"data_dir":       s.dataDir,
		"templates":      len(s.TemplateManager.ListTemplates("", false)),
		"custom_reports": len(s.Generator.ListCustomReports("")),
		"schedules":      len(s.ScheduleManager.ListSchedules(nil)),
		"data_sources":   len(s.Generator.ListDataSources()),
	}

	return stats
}

// ========== 便捷方法 ==========

// CreateReportFromTemplate 从模板快速创建报表.
func (s *Service) CreateReportFromTemplate(templateID string, parameters map[string]interface{}) (*GeneratedReport, error) {
	return s.Generator.GenerateFromTemplate(templateID, parameters, nil)
}

// CreateReportFromCustom 从自定义报表快速创建.
func (s *Service) CreateReportFromCustom(reportID string, parameters map[string]interface{}) (*GeneratedReport, error) {
	return s.Generator.GenerateFromCustomReport(reportID, parameters, nil)
}

// ExportReport 导出报表.
func (s *Service) ExportReport(report *GeneratedReport, format ExportFormat, outputPath string) (*ExportResult, error) {
	return s.Exporter.Export(report, format, outputPath, ExportOptions{
		Title:         report.Name,
		IncludeHeader: true,
		Summary:       true,
	})
}

// ScheduleReport 创建定时报表.
func (s *Service) ScheduleReport(name string, templateID string, frequency ScheduleFrequency, format ExportFormat, emails []string) (*ScheduledReport, error) {
	input := ScheduledReportInput{
		Name:         name,
		TemplateID:   templateID,
		Frequency:    frequency,
		ExportFormat: format,
		NotifyEmail:  emails,
		Enabled:      true,
	}

	return s.ScheduleManager.CreateSchedule(input, "system")
}

// ========== 全局实例 ==========

var (
	globalService *Service
	once          sync.Once
)

// Init 初始化全局服务.
func Init(config Config) error {
	var err error
	once.Do(func() {
		globalService, err = NewService(config)
	})
	return err
}

// GetService 获取全局服务.
func GetService() *Service {
	return globalService
}
