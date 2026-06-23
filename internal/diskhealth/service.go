package diskhealth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Service 磁盘健康服务
type Service struct {
	config    PredictorConfig
	collector *SMARTCollector
	predictor *PredictiveAnalyzer
	alerts    *AlertManager
	handler   *Handler

	disks       map[string]*DiskInfo
	assessments map[string]*HealthAssessment

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// NewService 创建服务
func NewService(config PredictorConfig) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	collector := NewSMARTCollector()
	analyzer := NewPredictiveAnalyzer()
	alertManager := NewAlertManager()

	svc := &Service{
		config:      config,
		collector:   collector,
		predictor:   analyzer,
		alerts:      alertManager,
		disks:       make(map[string]*DiskInfo),
		assessments: make(map[string]*HealthAssessment),
		ctx:         ctx,
		cancel:      cancel,
	}

	svc.handler = NewHandler(svc)

	return svc
}

// Start 启动服务
func (s *Service) Start() error {
	// 初始扫描
	if err := s.scanAllDisks(); err != nil {
		return fmt.Errorf("initial scan failed: %w", err)
	}

	// 启动定时检查
	go s.checkLoop()

	// 启动告警清理
	go s.alertCleanLoop()

	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.cancel()
}

// GetHandler 获取 HTTP 处理器
func (s *Service) GetHandler() *Handler {
	return s.handler
}

// GetDiskInfo 获取磁盘信息
func (s *Service) GetDiskInfo(device string) (*DiskInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	disk, ok := s.disks[device]
	return disk, ok
}

// GetAllDisks 获取所有磁盘
func (s *Service) GetAllDisks() []*DiskInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*DiskInfo, 0, len(s.disks))
	for _, d := range s.disks {
		result = append(result, d)
	}
	return result
}

// GetAssessment 获取评估
func (s *Service) GetAssessment(device string) (*HealthAssessment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.assessments[device]
	return a, ok
}

// GetAllAssessments 获取所有评估
func (s *Service) GetAllAssessments() []*HealthAssessment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*HealthAssessment, 0, len(s.assessments))
	for _, a := range s.assessments {
		result = append(result, a)
	}
	return result
}

// GetAlerts 获取告警
func (s *Service) GetAlerts(device string, includeAcked bool) []*Alert {
	return s.alerts.GetAlerts(device, includeAcked)
}

// AckAlert 确认告警
func (s *Service) AckAlert(alertID string) error {
	return s.alerts.AckAlert(alertID)
}

// GetHealthSummary 获取健康摘要
func (s *Service) GetHealthSummary() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := map[string]interface{}{
		"totalDisks":    len(s.disks),
		"healthyDisks":  0,
		"warningDisks":  0,
		"criticalDisks": 0,
		"failedDisks":   0,
		"averageScore":  0,
		"totalCapacity": int64(0),
		"timestamp":     time.Now(),
	}

	var totalScore int
	for _, a := range s.assessments {
		switch a.Status {
		case DiskStatusHealthy:
			summary["healthyDisks"] = summary["healthyDisks"].(int) + 1
		case DiskStatusWarning:
			summary["warningDisks"] = summary["warningDisks"].(int) + 1
		case DiskStatusCritical:
			summary["criticalDisks"] = summary["criticalDisks"].(int) + 1
		case DiskStatusFailed:
			summary["failedDisks"] = summary["failedDisks"].(int) + 1
		}
		totalScore += int(a.Score)
	}

	for _, d := range s.disks {
		summary["totalCapacity"] = summary["totalCapacity"].(int64) + d.Capacity
	}

	if len(s.assessments) > 0 {
		summary["averageScore"] = totalScore / len(s.assessments)
	}

	return summary
}

// RefreshDisk 刷新磁盘
func (s *Service) RefreshDisk(device string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	disk, err := s.collector.GetDiskInfo(device)
	if err != nil {
		return err
	}

	s.disks[device] = disk

	// 评估健康状态
	assessment := s.collector.assessHealth(disk)
	s.assessments[device] = assessment

	// 添加到预测分析器
	s.predictor.AddSnapshot(device, &HealthSnapshot{
		Timestamp:   time.Now(),
		Score:       assessment.Score,
		Temperature: disk.Temperature,
		PowerOnHrs:  disk.PowerOnHours,
	})

	// 检查告警
	s.alerts.CheckAndAlert(disk, assessment)

	return nil
}

// scanAllDisks 扫描所有磁盘
func (s *Service) scanAllDisks() error {
	disks, err := s.collector.ScanDisks()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, disk := range disks {
		s.disks[disk.Device] = disk

		// 评估健康状态
		assessment := s.collector.assessHealth(disk)
		s.assessments[disk.Device] = assessment

		// 添加到预测分析器
		s.predictor.AddSnapshot(disk.Device, &HealthSnapshot{
			Timestamp:   time.Now(),
			Score:       assessment.Score,
			Temperature: disk.Temperature,
			PowerOnHrs:  disk.PowerOnHours,
		})

		// 检查告警
		s.alerts.CheckAndAlert(disk, assessment)
	}

	return nil
}

// checkLoop 定时检查循环
func (s *Service) checkLoop() {
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.scanAllDisks()
		}
	}
}

// alertCleanLoop 告警清理循环
func (s *Service) alertCleanLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.alerts.CleanOldAlerts(time.Duration(s.config.MaxHistoryDays) * 24 * time.Hour)
		}
	}
}
