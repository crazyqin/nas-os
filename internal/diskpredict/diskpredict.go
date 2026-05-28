// Package diskpredict - 磁盘故障预测引擎实现
// SMART 数据采集与解析、健康评分算法、故障预测、生命周期管理
package diskpredict

import (
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PredictEngine 磁盘故障预测引擎
type PredictEngine struct {
	mu sync.RWMutex

	config      PredictConfig
	manager     *DiskPredictManager
	history     map[string][]*DiskHealth // device -> health records
}

// NewPredictEngine 创建预测引擎
func NewPredictEngine(config *PredictConfig) *PredictEngine {
	cfg := DefaultPredictConfig()
	if config != nil {
		cfg = *config
	}

	// 确保权重归一化
	totalWeight := cfg.WeightReallocatedSectors + cfg.WeightPendingSectors +
		cfg.WeightCRCError + cfg.WeightTemperature + cfg.WeightPowerOnHours
	if totalWeight > 0 && totalWeight != 1.0 {
		cfg.WeightReallocatedSectors /= totalWeight
		cfg.WeightPendingSectors /= totalWeight
		cfg.WeightCRCError /= totalWeight
		cfg.WeightTemperature /= totalWeight
		cfg.WeightPowerOnHours /= totalWeight
	}

	return &PredictEngine{
		config:  cfg,
		manager: NewDiskPredictManager(),
		history: make(map[string][]*DiskHealth),
	}
}

// CollectSMART 采集 SMART 数据
func (d *PredictEngine) CollectSMART(device string) (*SMARTData, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 使用 smartctl 采集数据
	output, err := exec.Command("smartctl", "-A", device).Output()
	if err != nil {
		return nil, fmt.Errorf("smartctl failed for %s: %w", device, err)
	}

	smart := &SMARTData{
		Device:      device,
		CollectedAt: time.Now(),
	}

	// 解析 SMART 输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 解析型号
		if strings.Contains(line, "Device Model:") || strings.Contains(line, "Model Number:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				smart.Model = strings.TrimSpace(parts[1])
			}
		}

		// 解析序列号
		if strings.Contains(line, "Serial Number:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				smart.Serial = strings.TrimSpace(parts[1])
			}
		}

		// 解析容量
		if strings.Contains(line, "User Capacity:") {
			re := regexp.MustCompile(`\[(\d+) bytes\]`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				capacity, _ := strconv.ParseUint(matches[1], 10, 64)
				smart.Capacity = capacity
			}
		}

		// 解析 SMART 属性
		attr := parseSMARTLine(line)
		if attr != nil {
			smart.Attributes = append(smart.Attributes, *attr)
			// 更新关键字段
			switch attr.ID {
			case 194: // Temperature
				smart.Temperature = int(attr.RawValue)
			case 9: // Power-On Hours
				smart.PowerOnHours = attr.RawValue
			}
		}
	}

	// 注册磁盘并更新 SMART 数据
	disk := &DiskInfo{
		Device:       device,
		Model:        smart.Model,
		Serial:       smart.Serial,
		Capacity:     smart.Capacity,
		SMARTEnabled: true,
		Status:       StatusHealthy,
	}
	_ = d.manager.RegisterDisk(disk)
	_ = d.manager.UpdateSMARTData(smart)

	return smart, nil
}

// parseSMARTLine 解析单行 SMART 属性
func parseSMARTLine(line string) *SMARTAttribute {
	// 跳过非属性行
	if !strings.Contains(line, "Pre-fail") && !strings.Contains(line, "Old_age") {
		return nil
	}

	fields := strings.Fields(line)
	if len(fields) < 10 {
		return nil
	}

	// 格式: ID NAME FLAG VALUE WORST THRESH TYPE UPDATED WHEN RAW_VALUE
	id, err := strconv.ParseUint(fields[0], 10, 8)
	if err != nil {
		return nil
	}

	value, _ := strconv.ParseUint(fields[3], 10, 8)
	worst, _ := strconv.ParseUint(fields[4], 10, 8)
	threshold, _ := strconv.ParseUint(fields[5], 10, 8)

	// 解析原始值（最后一个字段）
	rawStr := fields[len(fields)-1]
	rawValue, _ := strconv.ParseUint(rawStr, 10, 64)

	attrName := strings.Join(fields[1:len(fields)-6], " ")

	// 判断是否为关键属性
	criticalIDs := map[uint8]bool{5: true, 187: true, 188: true, 197: true, 198: true}

	attr := &SMARTAttribute{
		ID:         uint8(id),
		Name:       attrName,
		Value:      uint8(value),
		Worst:      uint8(worst),
		Threshold:  uint8(threshold),
		RawValue:   rawValue,
		IsFailed:   false,
		IsCritical: criticalIDs[uint8(id)],
	}

	// 判断是否失败
	if threshold > 0 && value <= threshold {
		attr.IsFailed = true
	}

	return attr
}

// AssessHealth 评估磁盘健康状态
func (d *PredictEngine) AssessHealth(device string) (*DiskHealth, error) {
	// 采集 SMART 数据
	smart, err := d.CollectSMART(device)
	if err != nil {
		return nil, fmt.Errorf("collect SMART failed: %w", err)
	}

	// 使用 Manager 进行预测
	result, err := d.manager.PredictFailure(device)
	if err != nil {
		return nil, fmt.Errorf("predict failed: %w", err)
	}

	health := &DiskHealth{
		Device:           device,
		Score:            result.HealthScore,
		Status:           string(result.Status),
		PredictedFailure: result.EstimatedLifeDays < 90,
		LastCheck:        time.Now(),
		SMARTData:        *smart,
	}

	// 保存历史记录
	d.mu.Lock()
	d.history[device] = append(d.history[device], health)
	// 裁剪历史
	maxRecords := d.config.MaxHistoryDays
	if len(d.history[device]) > maxRecords {
		d.history[device] = d.history[device][len(d.history[device])-maxRecords:]
	}
	d.mu.Unlock()

	return health, nil
}

// PredictFailure 预测故障
func (d *PredictEngine) PredictFailure(device string) (*FailurePrediction, error) {
	// 确保有 SMART 数据
	d.mu.RLock()
	_, hasHistory := d.history[device]
	d.mu.RUnlock()

	if !hasHistory {
		_, err := d.AssessHealth(device)
		if err != nil {
			return nil, fmt.Errorf("assess health failed: %w", err)
		}
	}

	// 使用 Manager 获取预测结果
	result, err := d.manager.PredictFailure(device)
	if err != nil {
		return nil, fmt.Errorf("predict failed: %w", err)
	}

	// 转换为 FailurePrediction
	prediction := &FailurePrediction{
		Device:      device,
		RiskFactors: result.RiskFactors,
	}

	// 计算故障概率
	switch {
	case result.HealthScore >= 80:
		prediction.Probability = 0.1
		prediction.EstimatedDays = 365
		prediction.Confidence = "high"
		prediction.FailureType = "unlikely"
	case result.HealthScore >= 60:
		prediction.Probability = 0.3
		prediction.EstimatedDays = 180
		prediction.Confidence = "medium"
		prediction.FailureType = "possible"
	case result.HealthScore >= 40:
		prediction.Probability = 0.6
		prediction.EstimatedDays = 90
		prediction.Confidence = "medium"
		prediction.FailureType = "likely"
	default:
		prediction.Probability = 0.9
		prediction.EstimatedDays = 30
		prediction.Confidence = "high"
		prediction.FailureType = "imminent"
	}

	return prediction, nil
}

// ScanAllDisks 巡检所有磁盘
func (d *PredictEngine) ScanAllDisks() (*HealthReport, error) {
	startTime := time.Now()

	// 获取所有磁盘设备
	devices, err := d.listBlockDevices()
	if err != nil {
		return nil, fmt.Errorf("list devices failed: %w", err)
	}

	report := &HealthReport{
		Disks:    make([]*DiskHealth, 0),
		ScanTime: startTime,
	}

	for _, device := range devices {
		// 应用设备过滤
		if !d.matchDeviceFilter(device) {
			continue
		}

		health, err := d.AssessHealth(device)
		if err != nil {
			// 跳过无法访问的设备
			continue
		}

		report.Disks = append(report.Disks, health)

		// 统计
		switch health.Status {
		case "critical", "failed":
			report.CriticalCount++
		case "warning":
			report.WarningCount++
		}
		if health.PredictedFailure {
			report.PredictedFailures++
		}
	}

	// 确定整体健康状态
	report.Duration = time.Since(startTime)
	report.OverallHealth = d.calculateOverallHealth(report)

	return report, nil
}

// listBlockDevices 列出块设备
func (d *PredictEngine) listBlockDevices() ([]string, error) {
	output, err := exec.Command("lsblk", "-dpno", "NAME").Output()
	if err != nil {
		return nil, err
	}

	devices := []string{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(line, "/dev/") {
			devices = append(devices, line)
		}
	}
	return devices, nil
}

// matchDeviceFilter 检查设备是否匹配过滤器
func (d *PredictEngine) matchDeviceFilter(device string) bool {
	if len(d.config.DeviceFilter) == 0 {
		return true
	}

	for _, pattern := range d.config.DeviceFilter {
		matched, err := regexp.MatchString(pattern, device)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// calculateOverallHealth 计算整体健康状态
func (d *PredictEngine) calculateOverallHealth(report *HealthReport) string {
	if len(report.Disks) == 0 {
		return "unknown"
	}

	if report.CriticalCount > 0 {
		return "critical"
	}

	if report.WarningCount > 0 {
		return "warning"
	}

	// 检查平均分数
	totalScore := 0.0
	for _, disk := range report.Disks {
		totalScore += disk.Score
	}
	avgScore := totalScore / float64(len(report.Disks))

	if avgScore >= 80 {
		return "healthy"
	} else if avgScore >= 60 {
		return "warning"
	}
	return "critical"
}

// SetThreshold 设置告警阈值
func (d *PredictEngine) SetThreshold(metric string, threshold int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	validMetrics := map[string]bool{
		"reallocated_sectors": true,
		"pending_sectors":     true,
		"crc_error":          true,
		"temperature":        true,
		"power_on_hours":     true,
	}

	if !validMetrics[metric] {
		return fmt.Errorf("invalid metric: %s", metric)
	}

	if threshold < 0 {
		return fmt.Errorf("threshold must be non-negative")
	}

	d.config.Thresholds[metric] = threshold
	return nil
}

// GetHistory 获取历史记录
func (d *PredictEngine) GetHistory(device string, days int) []*DiskHealth {
	d.mu.RLock()
	defer d.mu.RUnlock()

	records, exists := d.history[device]
	if !exists {
		return []*DiskHealth{}
	}

	if days <= 0 {
		return records
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	result := []*DiskHealth{}
	for _, record := range records {
		if record.LastCheck.After(cutoff) {
			result = append(result, record)
		}
	}
	return result
}

// calculateHealthScore 计算健康评分 (0-100) - 兼容性方法
func (d *PredictEngine) calculateHealthScore(smart *SMARTData) float64 {
	score := 100.0

	for _, attr := range smart.Attributes {
		switch {
		case attr.ID == 5 && attr.RawValue > 0: // Reallocated Sectors
			score -= math.Min(50, float64(attr.RawValue)*5) * d.config.WeightReallocatedSectors
		case attr.ID == 197 && attr.RawValue > 0: // Pending Sectors
			score -= math.Min(40, float64(attr.RawValue)*8) * d.config.WeightPendingSectors
		case attr.ID == 199 && attr.RawValue > 0: // CRC Errors
			score -= math.Min(30, float64(attr.RawValue)*2) * d.config.WeightCRCError
		}
	}

	// 温度扣分
	if smart.Temperature > 50 {
		score -= math.Min(30, float64(smart.Temperature-50)*3) * d.config.WeightTemperature
	}

	// 通电时间扣分
	if smart.PowerOnHours > 30000 {
		score -= math.Min(20, float64(smart.PowerOnHours-30000)/500) * d.config.WeightPowerOnHours
	}

	if score < 0 {
		score = 0
	}
	return math.Round(score*100) / 100
}

// average 计算平均值
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}
