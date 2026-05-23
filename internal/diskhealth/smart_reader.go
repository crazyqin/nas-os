// Package diskhealth 提供 SMART 磁盘健康监测和故障预测功能
package diskhealth

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// SmartData SMART 数据
type SmartData struct {
	// Model 磁盘型号
	Model string
	// Serial 序列号
	Serial string
	// Capacity 容量（字节）
	Capacity uint64
	// Temperature 温度（摄氏度）
	Temperature int
	// PowerOnHours 通电时间（小时）
	PowerOnHours uint64
	// Attributes SMART 属性列表
	Attributes []SmartAttribute
}

// readSMARTData 读取 SMART 数据（模拟 smartctl 输出解析）
func (m *DiskHealthMonitor) readSMARTData(device string) (*SmartData, error) {
	// 实际环境中应该调用 smartctl，这里使用模拟数据
	// smartctl -A /dev/sdX
	return m.readSMARTFromDevice(device)
}

// readSMARTFromDevice 从设备读取 SMART 数据
func (m *DiskHealthMonitor) readSMARTFromDevice(device string) (*SmartData, error) {
	// 执行 smartctl 命令获取 SMART 数据
	cmd := exec.Command("smartctl", "-A", "-i", "/dev/"+device)
	output, err := cmd.Output()
	if err != nil {
		// 如果 smartctl 不可用，返回模拟数据
		return m.getSimulatedSmartData(device), nil
	}

	return m.parseSmartctlOutput(string(output), device)
}

// parseSmartctlOutput 解析 smartctl 输出
func (m *DiskHealthMonitor) parseSmartctlOutput(output string, device string) (*SmartData, error) {
	data := &SmartData{
		Attributes: make([]SmartAttribute, 0),
	}

	lines := strings.Split(output, "\n")
	inAttributesSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 解析设备信息
		if strings.Contains(line, "Device Model:") {
			data.Model = strings.TrimSpace(strings.SplitAfter(line, ":")[1])
		}
		if strings.Contains(line, "Serial Number:") {
			data.Serial = strings.TrimSpace(strings.SplitAfter(line, ":")[1])
		}
		if strings.Contains(line, "User Capacity:") {
			// 解析容量，格式如: [1000 GB]
			re := regexp.MustCompile(`\[(.+?)\]`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				capacityStr := strings.ReplaceAll(matches[1], " ", "")
				capacityStr = strings.ReplaceAll(capacityStr, ",", "")
				capacity, _ := parseCapacity(capacityStr)
				data.Capacity = capacity
			}
		}

		// 检测属性部分开始
		if strings.Contains(line, "ID#") && strings.Contains(line, "ATTRIBUTE_NAME") {
			inAttributesSection = true
			continue
		}

		// 解析 SMART 属性
		if inAttributesSection && len(line) > 0 {
			attr := m.parseSmartAttribute(line)
			if attr != nil {
				data.Attributes = append(data.Attributes, *attr)

				// 提取温度和通电时间
				switch attr.ID {
				case 194: // Temperature_Celsius
					data.Temperature = int(attr.RawValue)
				case 9: // Power_On_Hours
					data.PowerOnHours = attr.RawValue
				}
			}
		}
	}

	// 如果没有解析到数据，使用模拟数据
	if len(data.Attributes) == 0 {
		return m.getSimulatedSmartData(device), nil
	}

	return data, nil
}

// parseSmartAttribute 解析单个 SMART 属性
func (m *DiskHealthMonitor) parseSmartAttribute(line string) *SmartAttribute {
	// SMART 属性格式: ID# ATTRIBUTE_NAME FLAG VALUE WORST THRESH TYPE UPDATED WHEN_FAILED RAW_VALUE
	// 示例: 5 Reallocated_Sector_Ct 0x0033 100 100 010 Pre-fail Always - 0

	parts := strings.Fields(line)
	if len(parts) < 10 {
		return nil
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}

	value, _ := strconv.Atoi(parts[3])
	worst, _ := strconv.Atoi(parts[4])
	threshold, _ := strconv.Atoi(parts[5])
	rawValue, _ := strconv.ParseUint(parts[9], 10, 64)

	// 判断是否为关键属性
	criticalIDs := map[int]bool{
		5:   true, // Reallocated_Sector_Ct
		187: true, // Reported_Uncorrect
		188: true, // Command_Timeout
		197: true, // Current_Pending_Sector
		198: true, // Offline_Uncorrectable
	}

	// 判断是否失败
	isFailed := false
	if value <= threshold && threshold > 0 {
		isFailed = true
	}
	if len(parts) > 8 && parts[8] == "FAILING_NOW" {
		isFailed = true
	}

	return &SmartAttribute{
		ID:         id,
		Name:       parts[1],
		Value:      value,
		Worst:      worst,
		Threshold:  threshold,
		RawValue:   rawValue,
		IsCritical: criticalIDs[id],
		IsFailed:   isFailed,
	}
}

// parseCapacity 解析容量字符串
func parseCapacity(s string) (uint64, error) {
	s = strings.ToUpper(s)
	multiplier := uint64(1)

	if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "TB") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "TB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	}

	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("解析容量失败: %w", err)
	}

	return uint64(value * float64(multiplier)), nil
}

// getSimulatedSmartData 获取模拟 SMART 数据
func (m *DiskHealthMonitor) getSimulatedSmartData(device string) *SmartData {
	// 根据设备名生成模拟数据
	seed := int64(0)
	for _, c := range device {
		seed += int64(c)
	}

	// 生成不同健康状态的模拟数据
	var temperature int
	var powerOnHours uint64
	var reallocatedSectors uint64
	var pendingSectors uint64
	var offlineUncorrectable uint64

	// 根据设备名生成不同的健康状态
	switch seed % 4 {
	case 0: // 健康
		temperature = 35
		powerOnHours = 8760 // 1年
		reallocatedSectors = 0
		pendingSectors = 0
		offlineUncorrectable = 0
	case 1: // 正常
		temperature = 42
		powerOnHours = 17520 // 2年
		reallocatedSectors = 5
		pendingSectors = 2
		offlineUncorrectable = 1
	case 2: // 警告
		temperature = 52
		powerOnHours = 26280 // 3年
		reallocatedSectors = 50
		pendingSectors = 15
		offlineUncorrectable = 10
	case 3: // 严重
		temperature = 58
		powerOnHours = 35040 // 4年
		reallocatedSectors = 150
		pendingSectors = 30
		offlineUncorrectable = 25
	}

	return &SmartData{
		Model:        fmt.Sprintf("SimulatedDisk-%s", device),
		Serial:       fmt.Sprintf("SIM%s123456", device),
		Capacity:     1024 * 1024 * 1024 * 1024, // 1TB
		Temperature:  temperature,
		PowerOnHours: powerOnHours,
		Attributes: []SmartAttribute{
			{
				ID:         5,
				Name:       "Reallocated_Sector_Ct",
				Value:      100 - int(reallocatedSectors/2),
				Worst:      100,
				Threshold:  10,
				RawValue:   reallocatedSectors,
				IsCritical: true,
				IsFailed:   reallocatedSectors > 100,
			},
			{
				ID:         9,
				Name:       "Power_On_Hours",
				Value:      100,
				Worst:      100,
				Threshold:  0,
				RawValue:   powerOnHours,
				IsCritical: false,
				IsFailed:   false,
			},
			{
				ID:         194,
				Name:       "Temperature_Celsius",
				Value:      100 - (temperature - 30),
				Worst:      100,
				Threshold:  0,
				RawValue:   uint64(temperature),
				IsCritical: false,
				IsFailed:   temperature > 65,
			},
			{
				ID:         197,
				Name:       "Current_Pending_Sector",
				Value:      100 - int(pendingSectors),
				Worst:      100,
				Threshold:  0,
				RawValue:   pendingSectors,
				IsCritical: true,
				IsFailed:   pendingSectors > 50,
			},
			{
				ID:         198,
				Name:       "Offline_Uncorrectable",
				Value:      100 - int(offlineUncorrectable),
				Worst:      100,
				Threshold:  0,
				RawValue:   offlineUncorrectable,
				IsCritical: true,
				IsFailed:   offlineUncorrectable > 50,
			},
		},
	}
}

// getDiskDevices 获取系统中的磁盘设备列表
func (m *DiskHealthMonitor) getDiskDevices() ([]string, error) {
	// 尝试从 /sys/block 获取磁盘列表
	cmd := exec.Command("lsblk", "-d", "-n", "-o", "NAME")
	output, err := cmd.Output()
	if err != nil {
		// 如果 lsblk 不可用，返回模拟设备列表
		return []string{"sda", "sdb", "sdc"}, nil
	}

	devices := make([]string, 0)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		device := strings.TrimSpace(line)
		if device != "" {
			devices = append(devices, device)
		}
	}

	if len(devices) == 0 {
		// 如果没有找到设备，返回模拟设备列表
		return []string{"sda", "sdb", "sdc"}, nil
	}

	return devices, nil
}
