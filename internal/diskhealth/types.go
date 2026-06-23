// Package diskhealth 磁盘健康预测系统
// 提供 S.M.A.R.T. 数据采集、AI 故障预测、主动告警等功能
package diskhealth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DiskStatus 磁盘状态
type DiskStatus string

const (
	DiskStatusHealthy  DiskStatus = "healthy"
	DiskStatusWarning  DiskStatus = "warning"
	DiskStatusCritical DiskStatus = "critical"
	DiskStatusFailed   DiskStatus = "failed"
	DiskStatusUnknown  DiskStatus = "unknown"
)

// HealthScore 健康评分 (0-100)
type HealthScore int

const (
	HealthScoreExcellent HealthScore = 90
	HealthScoreGood      HealthScore = 70
	HealthScoreFair      HealthScore = 50
	HealthScorePoor      HealthScore = 30
	HealthScoreCritical  HealthScore = 10
)

// SMARTAttribute S.M.A.R.T. 属性
type SMARTAttribute struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Worst      int    `json:"worst"`
	Threshold  int    `json:"threshold"`
	RawValue   int64  `json:"rawValue"`
	Failed     bool   `json:"failed"`
	Flags      string `json:"flags"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device      string           `json:"device"`      // /dev/sda
	Model       string           `json:"model"`       // 磁盘型号
	Serial      string           `json:"serial"`      // 序列号
	Interface   string           `json:"interface"`   // SATA/NVMe/SCSI
	Capacity    int64            `json:"capacity"`    // 字节
	Firmware    string           `json:"firmware"`    // 固件版本
	PowerOnHours int64           `json:"powerOnHours"` // 通电小时数
	Temperature  int             `json:"temperature"` // 温度 ℃
	SMARTAttrs  []SMARTAttribute `json:"smartAttributes"`
}

// HealthAssessment 健康评估
type HealthAssessment struct {
	Device         string      `json:"device"`
	Score          HealthScore `json:"score"`
	Status         DiskStatus  `json:"status"`
	PredictedLife  string      `json:"predictedLife"`  // 预计剩余寿命
	FailureProb    float64     `json:"failureProb"`    // 30天内故障概率
	RiskFactors    []string    `json:"riskFactors"`    // 风险因素
	Recommendations []string   `json:"recommendations"` // 建议
	AssessedAt     time.Time   `json:"assessedAt"`
	NextCheck      time.Time   `json:"nextCheck"`
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
	AlertLevelEmergency AlertLevel = "emergency"
)

// Alert 告警
type Alert struct {
	ID        string     `json:"id"`
	Device    string     `json:"device"`
	Level     AlertLevel `json:"level"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	CreatedAt time.Time  `json:"createdAt"`
	AckedAt   *time.Time `json:"ackedAt,omitempty"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

// PredictorConfig 预测器配置
type PredictorConfig struct {
	CheckInterval    time.Duration `json:"checkInterval"`    // 检查间隔
	EnableAI         bool          `json:"enableAI"`         // 启用 AI 预测
	AlertThreshold   int           `json:"alertThreshold"`   // 告警阈值
	MaxHistoryDays   int           `json:"maxHistoryDays"`   // 历史数据保留天数
	TemperatureWarn  int           `json:"temperatureWarn"`  // 温度告警阈值
	TemperatureCrit  int           `json:"temperatureCrit"`  // 温度危险阈值
}

// DefaultPredictorConfig 默认配置
func DefaultPredictorConfig() PredictorConfig {
	return PredictorConfig{
		CheckInterval:   1 * time.Hour,
		EnableAI:        true,
		AlertThreshold:  70,
		MaxHistoryDays:  90,
		TemperatureWarn: 55,
		TemperatureCrit: 65,
	}
}

// DiskHealthPredictor 磁盘健康预测器
type DiskHealthPredictor struct {
	config      PredictorConfig
	disks       map[string]*DiskInfo
	assessments map[string]*HealthAssessment
	alerts      []*Alert
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewPredictor 创建预测器
func NewPredictor(config PredictorConfig) *DiskHealthPredictor {
	ctx, cancel := context.WithCancel(context.Background())
	return &DiskHealthPredictor{
		config:      config,
		disks:       make(map[string]*DiskInfo),
		assessments: make(map[string]*HealthAssessment),
		alerts:      make([]*Alert, 0),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动预测器
func (p *DiskHealthPredictor) Start() error {
	// 初始扫描
	if err := p.scanDisks(); err != nil {
		return fmt.Errorf("initial scan failed: %w", err)
	}

	// 启动定时检查
	go p.runCheckLoop()
	return nil
}

// Stop 停止预测器
func (p *DiskHealthPredictor) Stop() {
	p.cancel()
}

// GetAssessment 获取磁盘评估
func (p *DiskHealthPredictor) GetAssessment(device string) (*HealthAssessment, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	assessment, ok := p.assessments[device]
	return assessment, ok
}

// GetAllAssessments 获取所有评估
func (p *DiskHealthPredictor) GetAllAssessments() []*HealthAssessment {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*HealthAssessment, 0, len(p.assessments))
	for _, a := range p.assessments {
		result = append(result, a)
	}
	return result
}

// GetAlerts 获取告警
func (p *DiskHealthPredictor) GetAlerts(includeAcked bool) []*Alert {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if includeAcked {
		return p.alerts
	}
	var result []*Alert
	for _, a := range p.alerts {
		if a.AckedAt == nil {
			result = append(result, a)
		}
	}
	return result
}

// AckAlert 确认告警
func (p *DiskHealthPredictor) AckAlert(alertID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, a := range p.alerts {
		if a.ID == alertID {
			now := time.Now()
			a.AckedAt = &now
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// GetDiskInfo 获取磁盘信息
func (p *DiskHealthPredictor) GetDiskInfo(device string) (*DiskInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	info, ok := p.disks[device]
	return info, ok
}

// GetAllDisks 获取所有磁盘
func (p *DiskHealthPredictor) GetAllDisks() []*DiskInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*DiskInfo, 0, len(p.disks))
	for _, d := range p.disks {
		result = append(result, d)
	}
	return result
}

// RefreshDisk 刷新指定磁盘
func (p *DiskHealthPredictor) RefreshDisk(device string) error {
	return p.refreshDisk(device)
}

// scanDisks 扫描磁盘
func (p *DiskHealthPredictor) scanDisks() error {
	collector := NewSMARTCollector()
	disks, err := collector.ScanDisks()
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, disk := range disks {
		p.disks[disk.Device] = disk
		// 简单评估
		assessment := collector.assessHealth(disk)
		p.assessments[disk.Device] = assessment
	}
	return nil
}

// refreshDisk 刷新单个磁盘
func (p *DiskHealthPredictor) refreshDisk(device string) error {
	collector := NewSMARTCollector()
	disk, err := collector.GetDiskInfo(device)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.disks[device] = disk
	assessment := collector.assessHealth(disk)
	p.assessments[device] = assessment
	return nil
}

// runCheckLoop 定时检查循环
func (p *DiskHealthPredictor) runCheckLoop() {
	ticker := time.NewTicker(p.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.scanDisks()
		}
	}
}
