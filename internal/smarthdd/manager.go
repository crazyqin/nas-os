package smarthdd

import (
	"fmt"
	"time"
)

// PredictFailure 预测磁盘故障
func (m *SmartHDDManager) PredictFailure(diskID string) (*FailurePrediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[diskID]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", diskID)
	}

	prediction := &FailurePrediction{
		DiskID:      diskID,
		Device:      disk.Device,
		PredictedAt: time.Now(),
	}

	// 基于SMART属性预测
	score := 100.0

	// 温度影响
	if disk.Temperature > 50 {
		score -= float64(disk.Temperature-50) * 2
	}

	// 重分配扇区影响
	if disk.ReallocSectors > 0 {
		score -= float64(disk.ReallocSectors) * 0.5
	}

	// 待处理扇区影响
	if disk.PendingSectors > 0 {
		score -= float64(disk.PendingSectors) * 0.3
	}

	// 通电时间影响
	if disk.PowerOnHours > 30000 {
		score -= float64(disk.PowerOnHours-30000) * 0.001
	}

	if score < 0 {
		score = 0
	}

	prediction.HealthScore = score
	prediction.RiskLevel = m.calculateRiskLevel(score)

	// 预测剩余寿命
	if score > 80 {
		prediction.EstimatedLifeDays = 365 * 3
	} else if score > 60 {
		prediction.EstimatedLifeDays = 365
	} else if score > 40 {
		prediction.EstimatedLifeDays = 180
	} else if score > 20 {
		prediction.EstimatedLifeDays = 90
	} else {
		prediction.EstimatedLifeDays = 30
	}

	return prediction, nil
}

// FailurePrediction 故障预测
type FailurePrediction struct {
	DiskID            string    `json:"disk_id"`
	Device            string    `json:"device"`
	HealthScore       float64   `json:"health_score"`
	RiskLevel         string    `json:"risk_level"`
	EstimatedLifeDays int       `json:"estimated_life_days"`
	Factors           []string  `json:"factors,omitempty"`
	PredictedAt       time.Time `json:"predicted_at"`
}

// calculateRiskLevel 计算风险等级
func (m *SmartHDDManager) calculateRiskLevel(score float64) string {
	if score >= 80 {
		return "low"
	} else if score >= 60 {
		return "medium"
	} else if score >= 40 {
		return "high"
	}
	return "critical"
}

// GetHealthHistory 获取健康历史
func (m *SmartHDDManager) GetHealthHistory(diskID string) ([]*HealthRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[diskID]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", diskID)
	}

	// 返回模拟的历史数据
	records := make([]*HealthRecord, 0)
	now := time.Now()

	for i := 30; i >= 0; i-- {
		record := &HealthRecord{
			DiskID:      diskID,
			Device:      disk.Device,
			Temperature: disk.Temperature,
			Health:      string(disk.Health),
			RecordedAt:  now.AddDate(0, 0, -i),
		}
		records = append(records, record)
	}

	return records, nil
}

// HealthRecord 健康记录
type HealthRecord struct {
	DiskID      string    `json:"disk_id"`
	Device      string    `json:"device"`
	Temperature int       `json:"temperature"`
	Health      string    `json:"health"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// GetDiskIOStats 获取磁盘IO统计
func (m *SmartHDDManager) GetDiskIOStats(diskID string) (*DiskIOStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[diskID]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", diskID)
	}

	return &DiskIOStats{
		DiskID:      diskID,
		Device:      disk.Device,
		ReadBytes:   1024 * 1024 * 100, // 模拟数据
		WriteBytes:  1024 * 1024 * 50,
		ReadOps:     1000,
		WriteOps:    500,
		AvgReadMs:   0.5,
		AvgWriteMs:  1.2,
		CollectedAt: time.Now(),
	}, nil
}

// DiskIOStats 磁盘IO统计
type DiskIOStats struct {
	DiskID      string    `json:"disk_id"`
	Device      string    `json:"device"`
	ReadBytes   int64     `json:"read_bytes"`
	WriteBytes  int64     `json:"write_bytes"`
	ReadOps     int64     `json:"read_ops"`
	WriteOps    int64     `json:"write_ops"`
	AvgReadMs   float64   `json:"avg_read_ms"`
	AvgWriteMs  float64   `json:"avg_write_ms"`
	CollectedAt time.Time `json:"collected_at"`
}

// GetDisksByHealth 按健康状态获取磁盘
func (m *SmartHDDManager) GetDisksByHealth(status HealthStatus) []*DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var disks []*DiskInfo
	for _, disk := range m.disks {
		if disk.Health == status {
			disks = append(disks, disk)
		}
	}
	return disks
}

// GetCriticalDisks 获取临界状态磁盘
func (m *SmartHDDManager) GetCriticalDisks() []*DiskInfo {
	return m.GetDisksByHealth(HealthCritical)
}

// GetWarningDisks 获取警告状态磁盘
func (m *SmartHDDManager) GetWarningDisks() []*DiskInfo {
	return m.GetDisksByHealth(HealthWarning)
}

// GetHealthyDisks 获取健康磁盘
func (m *SmartHDDManager) GetHealthyDisks() []*DiskInfo {
	return m.GetDisksByHealth(HealthGood)
}

// ExportReport 导出健康报告
func (m *SmartHDDManager) ExportReport() *HealthReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &HealthReport{
		GeneratedAt: time.Now(),
		TotalDisks:  len(m.disks),
		Disks:       make([]*DiskReport, 0),
	}

	for _, disk := range m.disks {
		diskReport := &DiskReport{
			Device:       disk.Device,
			Model:        disk.Model,
			Size:         disk.Size,
			Health:       string(disk.Health),
			Temperature:  disk.Temperature,
			PowerOnHours: disk.PowerOnHours,
		}
		report.Disks = append(report.Disks, diskReport)
	}

	return report
}

// HealthReport 健康报告
type HealthReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	TotalDisks  int           `json:"total_disks"`
	Disks       []*DiskReport `json:"disks"`
}

// DiskReport 磁盘报告
type DiskReport struct {
	Device       string `json:"device"`
	Model        string `json:"model"`
	Size         int64  `json:"size"`
	Health       string `json:"health"`
	Temperature  int    `json:"temperature"`
	PowerOnHours int64  `json:"power_on_hours"`
}
