// Package hardware SMART cron调度配置
// 兵部 Round 184 - SMART cron API实现
package hardware

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/internal/hardware/nvme"
	"nas-os/internal/monitoring"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ============================================================================
// 类型定义
// ============================================================================

// SMARTCheckType SMART检查类型
type SMARTCheckType string

const (
	SMARTCheckNVMe SMARTCheckType = "nvme" // 仅NVMe设备
	SMARTCheckSATA SMARTCheckType = "sata" // 仅SATA SSD
	SMARTCheckAll  SMARTCheckType = "all"  // 所有SSD设备
)

// SMARTThresholds SMART告警阈值配置
type SMARTThresholds struct {
	TemperatureThreshold    int     `json:"temperature_threshold"`     // 温度阈值(°C)，默认70
	PercentUsedThreshold    float64 `json:"percent_used_threshold"`    // 寿命使用阈值(%)，默认90
	AvailableSpareThreshold float64 `json:"available_spare_threshold"` // 备用空间阈值(%)，默认10
	MediaErrorThreshold     uint64  `json:"media_error_threshold"`     // 媒体错误阈值，默认10
}

// DefaultSMARTThresholds 默认SMART阈值
func DefaultSMARTThresholds() SMARTThresholds {
	return SMARTThresholds{
		TemperatureThreshold:    70,
		PercentUsedThreshold:    90,
		AvailableSpareThreshold: 10,
		MediaErrorThreshold:     10,
	}
}

// SMARTCronConfig SMART cron任务配置
type SMARTCronConfig struct {
	ID          string           `json:"id"`                    // 配置ID
	Name        string           `json:"name"`                  // 任务名称
	Schedule    string           `json:"schedule"`              // cron表达式，默认"0 6 * * *"
	Devices     []string         `json:"devices"`               // 设备过滤列表(空=全部)
	CheckType   SMARTCheckType   `json:"check_type"`            // 检查类型: nvme/sata/all
	Thresholds  SMARTThresholds  `json:"thresholds"`            // 告警阈值配置
	ReportPath  string           `json:"report_path"`           // 报告保存路径
	WebhookURL  string           `json:"webhook_url,omitempty"` // 告警webhook通知URL
	Enabled     bool             `json:"enabled"`               // 是否启用
	CreatedAt   time.Time        `json:"created_at"`            // 创建时间
	UpdatedAt   time.Time        `json:"updated_at"`            // 更新时间
	LastRun     *time.Time       `json:"last_run,omitempty"`    // 最后执行时间
	NextRun     *time.Time       `json:"next_run,omitempty"`    // 下次执行时间
	entryID     cron.EntryID     `json:"-"`                     // cron entry ID
}

// SMARTCronConfigRequest 创建/更新配置请求
type SMARTCronConfigRequest struct {
	Name       string          `json:"name" binding:"required"`
	Schedule   string          `json:"schedule" binding:"required"`
	Devices    []string        `json:"devices"`
	CheckType  SMARTCheckType  `json:"check_type" binding:"required,oneof=nvme sata all"`
	Thresholds SMARTThresholds `json:"thresholds"`
	ReportPath string          `json:"report_path"`
	WebhookURL string          `json:"webhook_url"`
	Enabled    bool            `json:"enabled"`
}

// SMARTCronReport SMART检查报告
type SMARTCronReport struct {
	ConfigID    string                 `json:"config_id"`
	ConfigName  string                 `json:"config_name"`
	RunTime     time.Time              `json:"run_time"`
	Duration    time.Duration          `json:"duration"`
	Devices     []SMARTDeviceReport    `json:"devices"`
	Summary     SMARTReportSummary     `json:"summary"`
	Alerts      []SMARTReportAlert     `json:"alerts,omitempty"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
}

// SMARTDeviceReport 单设备报告
type SMARTDeviceReport struct {
	Device        string  `json:"device"`
	Model         string  `json:"model"`
	HealthPercent float64 `json:"health_percent"`
	Temperature   int     `json:"temperature"`
	MediaErrors   uint64  `json:"media_errors"`
	Status        string  `json:"status"` // healthy/warning/critical
	AlertTriggered bool   `json:"alert_triggered"`
}

// SMARTReportSummary 报告汇总
type SMARTReportSummary struct {
	TotalDevices   int `json:"total_devices"`
	HealthyCount   int `json:"healthy_count"`
	WarningCount   int `json:"warning_count"`
	CriticalCount  int `json:"critical_count"`
	OfflineCount   int `json:"offline_count"`
}

// SMARTReportAlert 报告告警
type SMARTReportAlert struct {
	Device    string    `json:"device"`
	Type      string    `json:"type"`    // temperature/lifespan/spare/media_error
	Severity  string    `json:"severity"` // warning/critical
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ============================================================================
// SMART Cron调度器
// ============================================================================

// SMARTCronScheduler SMART cron调度器
type SMARTCronScheduler struct {
	mu sync.RWMutex

	// Cron调度器
	cronScheduler *cron.Cron

	// 配置存储
	configs map[string]*SMARTCronConfig

	// NVMe监控器
	nvmeMonitor *nvme.NVMeMonitor

	// SSD健康监控器
	ssdMonitor *monitoring.SSDHealthMonitor

	// 报告历史
	reports map[string][]*SMARTCronReport

	// 日志
	logger *zap.Logger

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 配置文件路径
	configPath string

	// 运行状态
	running bool
}

// NewSMARTCronScheduler 创建SMART cron调度器
func NewSMARTCronScheduler(logger *zap.Logger) *SMARTCronScheduler {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &SMARTCronScheduler{
		cronScheduler: cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
		configs:       make(map[string]*SMARTCronConfig),
		nvmeMonitor:   nvme.NewNVMeMonitor(nvme.DefaultAlertConfig()),
		ssdMonitor:    monitoring.NewSSDHealthMonitor(monitoring.DefaultSSDMonitorConfig),
		reports:       make(map[string][]*SMARTCronReport),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		configPath:    "/etc/nas-os/smart-cron-config.json",
	}

	// 加载已保存的配置
	s.loadConfigs()

	return s
}

// Start 启动调度器
func (s *SMARTCronScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("调度器已在运行")
	}

	s.cronScheduler.Start()
	s.running = true

	s.logger.Info("SMART cron调度器已启动")

	return nil
}

// Stop 停止调度器
func (s *SMARTCronScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("调度器已停止")
	}

	ctx := s.cronScheduler.Stop()
	<-ctx.Done()

	s.cancel()
	s.running = false

	s.logger.Info("SMART cron调度器已停止")

	return nil
}

// ============================================================================
// 配置管理
// ============================================================================

// CreateConfig 创建新的SMART cron配置
func (s *SMARTCronScheduler) CreateConfig(req *SMARTCronConfigRequest) (*SMARTCronConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证cron表达式
	schedule, err := cron.ParseStandard(req.Schedule)
	if err != nil {
		return nil, fmt.Errorf("无效的cron表达式: %w", err)
	}

	// 设置默认值
	if req.CheckType == "" {
		req.CheckType = SMARTCheckAll
	}
	if req.Thresholds.TemperatureThreshold == 0 {
		req.Thresholds = DefaultSMARTThresholds()
	}
	if req.ReportPath == "" {
		req.ReportPath = "/var/log/nas-os/smart-reports"
	}

	config := &SMARTCronConfig{
		ID:         generateConfigID(),
		Name:       req.Name,
		Schedule:   req.Schedule,
		Devices:    req.Devices,
		CheckType:  req.CheckType,
		Thresholds: req.Thresholds,
		ReportPath: req.ReportPath,
		WebhookURL: req.WebhookURL,
		Enabled:    req.Enabled,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 计算下次执行时间
	now := time.Now()
	nextRun := schedule.Next(now)
	config.NextRun = &nextRun

	// 如果启用，添加到调度器
	if config.Enabled {
		entryID, err := s.cronScheduler.AddFunc(config.Schedule, func() {
			s.executeConfig(config.ID)
		})
		if err != nil {
			return nil, fmt.Errorf("添加调度任务失败: %w", err)
		}
		config.entryID = entryID
	}

	s.configs[config.ID] = config

	// 保存配置
	s.saveConfigs()

	s.logger.Info("SMART cron配置已创建",
		zap.String("config_id", config.ID),
		zap.String("name", config.Name),
		zap.String("schedule", config.Schedule),
	)

	return config, nil
}

// GetConfig 获取配置
func (s *SMARTCronScheduler) GetConfig(id string) (*SMARTCronConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.configs[id]
	if !exists {
		return nil, fmt.Errorf("配置不存在: %s", id)
	}

	return config, nil
}

// ListConfigs 列出所有配置
func (s *SMARTCronScheduler) ListConfigs() []*SMARTCronConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*SMARTCronConfig, 0, len(s.configs))
	for _, config := range s.configs {
		result = append(result, config)
	}

	return result
}

// UpdateConfig 更新配置
func (s *SMARTCronScheduler) UpdateConfig(id string, req *SMARTCronConfigRequest) (*SMARTCronConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.configs[id]
	if !exists {
		return nil, fmt.Errorf("配置不存在: %s", id)
	}

	// 验证新的cron表达式
	schedule, err := cron.ParseStandard(req.Schedule)
	if err != nil {
		return nil, fmt.Errorf("无效的cron表达式: %w", err)
	}

	// 如果之前已启用，先移除旧的调度
	if config.Enabled && config.entryID != 0 {
		s.cronScheduler.Remove(config.entryID)
	}

	// 更新配置
	config.Name = req.Name
	config.Schedule = req.Schedule
	config.Devices = req.Devices
	config.CheckType = req.CheckType
	config.Thresholds = req.Thresholds
	config.ReportPath = req.ReportPath
	config.WebhookURL = req.WebhookURL
	config.Enabled = req.Enabled
	config.UpdatedAt = time.Now()

	// 计算下次执行时间
	now := time.Now()
	nextRun := schedule.Next(now)
	config.NextRun = &nextRun

	// 如果启用，重新添加调度
	if config.Enabled {
		entryID, err := s.cronScheduler.AddFunc(config.Schedule, func() {
			s.executeConfig(config.ID)
		})
		if err != nil {
			return nil, fmt.Errorf("添加调度任务失败: %w", err)
		}
		config.entryID = entryID
	} else {
		config.entryID = 0
	}

	// 保存配置
	s.saveConfigs()

	s.logger.Info("SMART cron配置已更新",
		zap.String("config_id", config.ID),
		zap.String("name", config.Name),
	)

	return config, nil
}

// DeleteConfig 删除配置
func (s *SMARTCronScheduler) DeleteConfig(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.configs[id]
	if !exists {
		return fmt.Errorf("配置不存在: %s", id)
	}

	// 如果已调度，移除
	if config.Enabled && config.entryID != 0 {
		s.cronScheduler.Remove(config.entryID)
	}

	delete(s.configs, id)
	delete(s.reports, id)

	// 保存配置
	s.saveConfigs()

	s.logger.Info("SMART cron配置已删除", zap.String("config_id", id))

	return nil
}

// ToggleConfig 启用/禁用配置
func (s *SMARTCronScheduler) ToggleConfig(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.configs[id]
	if !exists {
		return fmt.Errorf("配置不存在: %s", id)
	}

	if config.Enabled == enabled {
		// 状态未改变
		return nil
	}

	// 移除旧的调度
	if config.entryID != 0 {
		s.cronScheduler.Remove(config.entryID)
	}

	config.Enabled = enabled
	config.UpdatedAt = time.Now()

	// 如果启用，添加新的调度
	if enabled {
		entryID, err := s.cronScheduler.AddFunc(config.Schedule, func() {
			s.executeConfig(config.ID)
		})
		if err != nil {
			return fmt.Errorf("添加调度任务失败: %w", err)
		}
		config.entryID = entryID
	} else {
		config.entryID = 0
	}

	// 保存配置
	s.saveConfigs()

	s.logger.Info("SMART cron配置状态已切换",
		zap.String("config_id", id),
		zap.Bool("enabled", enabled),
	)

	return nil
}

// ============================================================================
// 执行与报告
// ============================================================================

// executeConfig 执行SMART检查配置
func (s *SMARTCronScheduler) executeConfig(configID string) {
	s.mu.RLock()
	config, exists := s.configs[configID]
	s.mu.RUnlock()

	if !exists || !config.Enabled {
		return
	}

	startTime := time.Now()
	report := &SMARTCronReport{
		ConfigID:   configID,
		ConfigName: config.Name,
		RunTime:    startTime,
		Success:    true,
	}

	s.logger.Info("开始执行SMART cron检查",
		zap.String("config_id", configID),
		zap.String("name", config.Name),
	)

	// 执行检查
	devices, alerts, err := s.performCheck(config)
	if err != nil {
		report.Success = false
		report.Error = err.Error()
		s.logger.Error("SMART检查执行失败",
			zap.String("config_id", configID),
			zap.Error(err),
		)
	} else {
		report.Devices = devices
		report.Alerts = alerts
		report.Summary = s.calculateSummary(devices)
	}

	report.Duration = time.Since(startTime)

	// 保存报告
	s.saveReport(configID, report)

	// 发送告警通知
	if len(alerts) > 0 && config.WebhookURL != "" {
		s.sendWebhookNotification(config, report)
	}

	// 更新配置的执行时间
	s.mu.Lock()
	config.LastRun = &startTime
	schedule, _ := cron.ParseStandard(config.Schedule)
	nextRun := schedule.Next(startTime)
	config.NextRun = &nextRun
	s.mu.Unlock()

	s.logger.Info("SMART cron检查完成",
		zap.String("config_id", configID),
		zap.Duration("duration", report.Duration),
		zap.Bool("success", report.Success),
	)
}

// performCheck 执行SMART检查
func (s *SMARTCronScheduler) performCheck(config *SMARTCronConfig) ([]SMARTDeviceReport, []SMARTReportAlert, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	var devices []SMARTDeviceReport
	var alerts []SMARTReportAlert

	// 根据检查类型选择设备
	switch config.CheckType {
	case SMARTCheckNVMe:
		// 检查NVMe设备
		nvmeStatuses, err := s.nvmeMonitor.CheckAllHealth(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("NVMe检查失败: %w", err)
		}

		for _, status := range nvmeStatuses {
			// 过滤设备
			if len(config.Devices) > 0 && !containsDevice(config.Devices, status.Device) {
				continue
			}

			deviceReport := SMARTDeviceReport{
				Device:        status.Device,
				Model:         status.Model,
				HealthPercent: 100 - status.PercentUsed,
				Temperature:   status.Temperature,
				MediaErrors:   status.MediaErrors,
				Status:        status.SmartStatus,
			}

			// 检查阈值触发告警
			alert, triggered := s.checkThresholdsNVMe(config.Thresholds, status)
			if triggered {
				deviceReport.AlertTriggered = triggered
				alerts = append(alerts, alert)
			}

			devices = append(devices, deviceReport)
		}

	case SMARTCheckSATA:
		// 检查SATA SSD设备
		ssdHealths := s.ssdMonitor.GetAllSSDs()

		for _, health := range ssdHealths {
			// 只检查SATA接口
			if health.InterfaceType != "SATA" {
				continue
			}

			// 过滤设备
			if len(config.Devices) > 0 && !containsDevice(config.Devices, health.Device) {
				continue
			}

			deviceReport := SMARTDeviceReport{
				Device:        health.Device,
				Model:         health.Model,
				HealthPercent: health.HealthPercent,
				Temperature:   health.Temperature,
				MediaErrors:   health.MediaErrors,
				Status:        string(health.Status),
			}

			// 检查阈值触发告警
			alert, triggered := s.checkThresholdsSSD(config.Thresholds, health)
			if triggered {
				deviceReport.AlertTriggered = triggered
				alerts = append(alerts, alert)
			}

			devices = append(devices, deviceReport)
		}

	case SMARTCheckAll:
		// 检查所有SSD设备
		// NVMe
		nvmeStatuses, err := s.nvmeMonitor.CheckAllHealth(ctx)
		if err == nil {
			for _, status := range nvmeStatuses {
				if len(config.Devices) > 0 && !containsDevice(config.Devices, status.Device) {
					continue
				}

				deviceReport := SMARTDeviceReport{
					Device:        status.Device,
					Model:         status.Model,
					HealthPercent: 100 - status.PercentUsed,
					Temperature:   status.Temperature,
					MediaErrors:   status.MediaErrors,
					Status:        status.SmartStatus,
				}

				alert, triggered := s.checkThresholdsNVMe(config.Thresholds, status)
				if triggered {
					deviceReport.AlertTriggered = triggered
					alerts = append(alerts, alert)
				}

				devices = append(devices, deviceReport)
			}
		}

		// SATA
		ssdHealths := s.ssdMonitor.GetAllSSDs()
		for _, health := range ssdHealths {
			if len(config.Devices) > 0 && !containsDevice(config.Devices, health.Device) {
				continue
			}

			deviceReport := SMARTDeviceReport{
				Device:        health.Device,
				Model:         health.Model,
				HealthPercent: health.HealthPercent,
				Temperature:   health.Temperature,
				MediaErrors:   health.MediaErrors,
				Status:        string(health.Status),
			}

			alert, triggered := s.checkThresholdsSSD(config.Thresholds, health)
			if triggered {
				deviceReport.AlertTriggered = triggered
				alerts = append(alerts, alert)
			}

			devices = append(devices, deviceReport)
		}
	}

	return devices, alerts, nil
}

// checkThresholdsNVMe 检查NVMe阈值
func (s *SMARTCronScheduler) checkThresholdsNVMe(thresholds SMARTThresholds, status nvme.HealthStatus) (SMARTReportAlert, bool) {
	// 温度告警
	if status.Temperature >= thresholds.TemperatureThreshold {
		severity := "warning"
		if status.Temperature >= 80 {
			severity = "critical"
		}
		return SMARTReportAlert{
			Device:    status.Device,
			Type:      "temperature",
			Severity:  severity,
			Message:   fmt.Sprintf("NVMe温度过高: %d°C", status.Temperature),
			Timestamp: time.Now(),
		}, true
	}

	// 寿命告警
	if status.PercentUsed >= thresholds.PercentUsedThreshold {
		severity := "warning"
		if status.PercentUsed >= 95 {
			severity = "critical"
		}
		return SMARTReportAlert{
			Device:    status.Device,
			Type:      "lifespan",
			Severity:  severity,
			Message:   fmt.Sprintf("NVMe寿命告警: 已使用%.1f%%", status.PercentUsed),
			Timestamp: time.Now(),
		}, true
	}

	// 备用空间告警
	if status.AvailableSpare <= thresholds.AvailableSpareThreshold {
		return SMARTReportAlert{
			Device:    status.Device,
			Type:      "spare",
			Severity:  "critical",
			Message:   fmt.Sprintf("NVMe备用空间不足: %.1f%%", status.AvailableSpare),
			Timestamp: time.Now(),
		}, true
	}

	// 媒体错误告警
	if status.MediaErrors >= thresholds.MediaErrorThreshold {
		return SMARTReportAlert{
			Device:    status.Device,
			Type:      "media_error",
			Severity:  "warning",
			Message:   fmt.Sprintf("NVMe媒体错误: %d次", status.MediaErrors),
			Timestamp: time.Now(),
		}, true
	}

	return SMARTReportAlert{}, false
}

// checkThresholdsSSD 检查SATA SSD阈值
func (s *SMARTCronScheduler) checkThresholdsSSD(thresholds SMARTThresholds, health *monitoring.SSDHealth) (SMARTReportAlert, bool) {
	// 温度告警
	if health.Temperature >= thresholds.TemperatureThreshold {
		severity := "warning"
		if health.Temperature >= 80 {
			severity = "critical"
		}
		return SMARTReportAlert{
			Device:    health.Device,
			Type:      "temperature",
			Severity:  severity,
			Message:   fmt.Sprintf("SSD温度过高: %d°C", health.Temperature),
			Timestamp: time.Now(),
		}, true
	}

	// 寿命告警
	if health.LifeUsedPercent >= thresholds.PercentUsedThreshold {
		severity := "warning"
		if health.LifeUsedPercent >= 95 {
			severity = "critical"
		}
		return SMARTReportAlert{
			Device:    health.Device,
			Type:      "lifespan",
			Severity:  severity,
			Message:   fmt.Sprintf("SSD寿命告警: 已使用%.1f%%", health.LifeUsedPercent),
			Timestamp: time.Now(),
		}, true
	}

	// 媒体错误告警
	if health.MediaErrors >= thresholds.MediaErrorThreshold {
		return SMARTReportAlert{
			Device:    health.Device,
			Type:      "media_error",
			Severity:  "warning",
			Message:   fmt.Sprintf("SSD媒体错误: %d次", health.MediaErrors),
			Timestamp: time.Now(),
		}, true
	}

	return SMARTReportAlert{}, false
}

// calculateSummary 计算汇总
func (s *SMARTCronScheduler) calculateSummary(devices []SMARTDeviceReport) SMARTReportSummary {
	summary := SMARTReportSummary{
		TotalDevices: len(devices),
	}

	for _, d := range devices {
		switch d.Status {
		case "healthy":
			summary.HealthyCount++
		case "warning":
			summary.WarningCount++
		case "critical", "emergency":
			summary.CriticalCount++
		case "offline", "unknown":
			summary.OfflineCount++
		}
	}

	return summary
}

// ============================================================================
// 报告管理
// ============================================================================

// GetReports 获取配置的报告历史
func (s *SMARTCronScheduler) GetReports(configID string, limit int) []*SMARTCronReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reports, exists := s.reports[configID]
	if !exists {
		return nil
	}

	if limit > 0 && len(reports) > limit {
		return reports[len(reports)-limit:]
	}

	return reports
}

// GetLatestReport 获取最新报告
func (s *SMARTCronScheduler) GetLatestReport(configID string) *SMARTCronReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reports, exists := s.reports[configID]
	if !exists || len(reports) == 0 {
		return nil
	}

	return reports[len(reports)-1]
}

// saveReport 保存报告
func (s *SMARTCronScheduler) saveReport(configID string, report *SMARTCronReport) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 添加到历史
	reports := s.reports[configID]
	reports = append(reports, report)

	// 保留最近100条
	if len(reports) > 100 {
		reports = reports[len(reports)-100:]
	}

	s.reports[configID] = reports

	// 保存到文件
	if report.Success && len(report.Devices) > 0 {
		s.saveReportToFile(configID, report)
	}
}

// saveReportToFile 保存报告到文件
func (s *SMARTCronScheduler) saveReportToFile(configID string, report *SMARTCronReport) {
	s.mu.RLock()
	config, exists := s.configs[configID]
	s.mu.RUnlock()

	if !exists || config.ReportPath == "" {
		return
	}

	// 创建目录
	if err := os.MkdirAll(config.ReportPath, 0755); err != nil {
		s.logger.Warn("创建报告目录失败", zap.Error(err))
		return
	}

	// 生成报告文件名
	filename := fmt.Sprintf("smart-report-%s-%s.json", configID, report.RunTime.Format("20060102-150405"))
	filepath := filepath.Join(config.ReportPath, filename)

	// 写入报告
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		s.logger.Warn("序列化报告失败", zap.Error(err))
		return
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		s.logger.Warn("写入报告文件失败", zap.Error(err))
		return
	}

	s.logger.Info("SMART报告已保存", zap.String("path", filepath))
}

// ============================================================================
// 通知
// ============================================================================

// sendWebhookNotification 发送webhook通知
func (s *SMARTCronScheduler) sendWebhookNotification(config *SMARTCronConfig, report *SMARTCronReport) {
	// 这里实现webhook通知逻辑
	// 可以使用HTTP client发送POST请求
	s.logger.Info("发送SMART告警通知",
		zap.String("webhook_url", config.WebhookURL),
		zap.Int("alerts", len(report.Alerts)),
	)

	// TODO: 实现实际的webhook调用
}

// ============================================================================
// 配置持久化
// ============================================================================

// loadConfigs 加载配置文件
func (s *SMARTCronScheduler) loadConfigs() {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，使用空配置
			return
		}
		s.logger.Warn("读取SMART cron配置失败", zap.Error(err))
		return
	}

	var configs []*SMARTCronConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		s.logger.Warn("解析SMART cron配置失败", zap.Error(err))
		return
	}

	s.mu.Lock()
	for _, config := range configs {
		s.configs[config.ID] = config

		// 如果启用，重新调度
		if config.Enabled {
			entryID, err := s.cronScheduler.AddFunc(config.Schedule, func() {
				s.executeConfig(config.ID)
			})
			if err != nil {
				s.logger.Warn("恢复调度失败",
					zap.String("config_id", config.ID),
					zap.Error(err),
				)
			} else {
				config.entryID = entryID
			}
		}
	}
	s.mu.Unlock()

	s.logger.Info("SMART cron配置已加载", zap.Int("count", len(configs)))
}

// saveConfigs 保存配置文件
func (s *SMARTCronScheduler) saveConfigs() {
	s.mu.RLock()
	configs := make([]*SMARTCronConfig, 0, len(s.configs))
	for _, config := range s.configs {
		// 复制配置，不保存entryID
		configCopy := *config
		configCopy.entryID = 0
		configs = append(configs, &configCopy)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		s.logger.Warn("序列化SMART cron配置失败", zap.Error(err))
		return
	}

	// 创建目录
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.logger.Warn("创建配置目录失败", zap.Error(err))
		return
	}

	if err := os.WriteFile(s.configPath, data, 0644); err != nil {
		s.logger.Warn("保存SMART cron配置失败", zap.Error(err))
		return
	}

	s.logger.Info("SMART cron配置已保存")
}

// ============================================================================
// 辅助函数
// ============================================================================

// generateConfigID 生成配置ID
func generateConfigID() string {
	return fmt.Sprintf("smart-cron-%d", time.Now().UnixNano())
}

// containsDevice 检查设备是否在列表中
func containsDevice(devices []string, device string) bool {
	for _, d := range devices {
		if d == device {
			return true
		}
	}
	return false
}

// IsRunning 检查调度器是否运行
func (s *SMARTCronScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}