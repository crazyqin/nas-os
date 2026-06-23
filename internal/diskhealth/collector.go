package diskhealth

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SMARTCollector S.M.A.R.T. 数据采集器
type SMARTCollector struct {
	smartctlPath string
}

// NewSMARTCollector 创建采集器
func NewSMARTCollector() *SMARTCollector {
	return &SMARTCollector{
		smartctlPath: "smartctl",
	}
}

// ScanDisks 扫描系统磁盘
func (c *SMARTCollector) ScanDisks() ([]*DiskInfo, error) {
	// 尝试使用 smartctl --scan
	output, err := c.runSmartctl("--scan")
	if err == nil {
		return c.parseScanOutput(output)
	}

	// 回退：从 /sys/block 读取
	return c.scanSysBlock()
}

// GetDiskInfo 获取指定磁盘信息
func (c *SMARTCollector) GetDiskInfo(device string) (*DiskInfo, error) {
	// 获取基本信息
	info := &DiskInfo{
		Device: device,
	}

	// 从 /sys/block 获取基本信息
	if err := c.fillSysInfo(info); err != nil {
		return nil, fmt.Errorf("failed to get sys info: %w", err)
	}

	// 获取 S.M.A.R.T. 数据
	if err := c.fillSMARTData(info); err != nil {
		// SMART 不可用不是致命错误
		info.SMARTAttrs = []SMARTAttribute{}
	}

	return info, nil
}

// GetHealth 获取健康评估
func (c *SMARTCollector) GetHealth(device string) (*HealthAssessment, error) {
	info, err := c.GetDiskInfo(device)
	if err != nil {
		return nil, err
	}

	return c.assessHealth(info), nil
}

// assessHealth 评估磁盘健康
func (c *SMARTCollector) assessHealth(info *DiskInfo) *HealthAssessment {
	assessment := &HealthAssessment{
		Device:     info.Device,
		Score:      100,
		Status:     DiskStatusHealthy,
		AssessedAt: timeNow(),
		NextCheck:  timeNow().Add(1 * time.Hour),
	}

	riskFactors := []string{}
	recommendations := []string{}

	// 检查 S.M.A.R.T. 属性
	for _, attr := range info.SMARTAttrs {
		if attr.Failed {
			assessment.Score -= 30
			riskFactors = append(riskFactors, fmt.Sprintf("S.M.A.R.T. 属性 %s 已失败", attr.Name))
		}

		// 检查关键属性
		switch attr.ID {
		case 5: // Reallocated Sector Count
			if attr.RawValue > 0 {
				penalty := min(int(attr.RawValue/10), 40)
				assessment.Score -= HealthScore(penalty)
				riskFactors = append(riskFactors, fmt.Sprintf("重分配扇区数: %d", attr.RawValue))
			}
		case 187: // Reported Uncorrectable Errors
			if attr.RawValue > 0 {
				penalty := min(int(attr.RawValue/5), 30)
				assessment.Score -= HealthScore(penalty)
				riskFactors = append(riskFactors, fmt.Sprintf("不可纠正错误: %d", attr.RawValue))
			}
		case 188: // Command Timeout
			if attr.RawValue > 100 {
				assessment.Score -= 15
				riskFactors = append(riskFactors, fmt.Sprintf("命令超时: %d", attr.RawValue))
			}
		case 197: // Current Pending Sector Count
			if attr.RawValue > 0 {
				penalty := min(int(attr.RawValue/5), 25)
				assessment.Score -= HealthScore(penalty)
				riskFactors = append(riskFactors, fmt.Sprintf("待定扇区数: %d", attr.RawValue))
			}
		case 198: // Offline Uncorrectable
			if attr.RawValue > 0 {
				penalty := min(int(attr.RawValue/5), 25)
				assessment.Score -= HealthScore(penalty)
				riskFactors = append(riskFactors, fmt.Sprintf("离线不可纠正: %d", attr.RawValue))
			}
		}
	}

	// 检查温度
	if info.Temperature > 65 {
		assessment.Score -= 20
		riskFactors = append(riskFactors, fmt.Sprintf("温度过高: %d°C", info.Temperature))
		recommendations = append(recommendations, "改善散热，检查风扇工作状态")
	} else if info.Temperature > 55 {
		assessment.Score -= 10
		riskFactors = append(riskFactors, fmt.Sprintf("温度偏高: %d°C", info.Temperature))
	}

	// 检查通电时间
	if info.PowerOnHours > 50000 {
		assessment.Score -= 15
		riskFactors = append(riskFactors, fmt.Sprintf("通电时间较长: %d小时", info.PowerOnHours))
		recommendations = append(recommendations, "磁盘已使用较长时间，建议准备替换")
	} else if info.PowerOnHours > 30000 {
		assessment.Score -= 5
	}

	// 确保评分在 0-100
	if assessment.Score < 0 {
		assessment.Score = 0
	}

	// 设置状态
	switch {
	case assessment.Score >= 80:
		assessment.Status = DiskStatusHealthy
		assessment.PredictedLife = "3年以上"
		assessment.FailureProb = 0.01
	case assessment.Score >= 60:
		assessment.Status = DiskStatusWarning
		assessment.PredictedLife = "1-3年"
		assessment.FailureProb = 0.05
		recommendations = append(recommendations, "建议定期备份重要数据")
	case assessment.Score >= 40:
		assessment.Status = DiskStatusCritical
		assessment.PredictedLife = "6-12个月"
		assessment.FailureProb = 0.15
		recommendations = append(recommendations, "强烈建议尽快备份数据并准备替换磁盘")
	default:
		assessment.Status = DiskStatusFailed
		assessment.PredictedLife = "3个月以内"
		assessment.FailureProb = 0.40
		recommendations = append(recommendations, "立即备份数据！磁盘可能随时故障")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "磁盘状态良好，继续保持定期检查")
	}

	assessment.RiskFactors = riskFactors
	assessment.Recommendations = recommendations
	return assessment
}

// runSmartctl 执行 smartctl 命令
func (c *SMARTCollector) runSmartctl(args ...string) (string, error) {
	cmd := exec.Command(c.smartctlPath, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// parseScanOutput 解析扫描输出
func (c *SMARTCollector) parseScanOutput(output string) ([]*DiskInfo, error) {
	var disks []*DiskInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	re := regexp.MustCompile(`^/dev/(\w+)\s+`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			device := "/dev/" + matches[1]
			info, err := c.GetDiskInfo(device)
			if err == nil {
				disks = append(disks, info)
			}
		}
	}
	return disks, nil
}

// scanSysBlock 从 /sys/block 扫描
func (c *SMARTCollector) scanSysBlock() ([]*DiskInfo, error) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}

	var disks []*DiskInfo
	for _, entry := range entries {
		name := entry.Name()
		// 只扫描 sd* 和 nvme* 设备
		if !strings.HasPrefix(name, "sd") && !strings.HasPrefix(name, "nvme") {
			continue
		}
		// 跳过分区
		if strings.ContainsAny(name[len(name)-1:], "0123456789") {
			continue
		}

		device := "/dev/" + name
		info, err := c.GetDiskInfo(device)
		if err == nil {
			disks = append(disks, info)
		}
	}
	return disks, nil
}

// fillSysInfo 从 /sys 填充信息
func (c *SMARTCollector) fillSysInfo(info *DiskInfo) error {
	devName := strings.TrimPrefix(info.Device, "/dev/")

	// 读取型号
	modelPath := fmt.Sprintf("/sys/block/%s/device/model", devName)
	if data, err := os.ReadFile(modelPath); err == nil {
		info.Model = strings.TrimSpace(string(data))
	}

	// 读取容量
	sizePath := fmt.Sprintf("/sys/block/%s/size", devName)
	if data, err := os.ReadFile(sizePath); err == nil {
		if sectors, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			info.Capacity = sectors * 512 // 512 bytes per sector
		}
	}

	// 判断接口类型
	if strings.HasPrefix(devName, "nvme") {
		info.Interface = "NVMe"
	} else {
		info.Interface = "SATA"
	}

	return nil
}

// fillSMARTData 填充 S.M.A.R.T. 数据
func (c *SMARTCollector) fillSMARTData(info *DiskInfo) error {
	output, err := c.runSmartctl("-A", info.Device)
	if err != nil {
		return err
	}

	info.SMARTAttrs = c.parseSMARTOutput(output)

	// 获取温度和通电时间
	infoOutput, _ := c.runSmartctl("-i", info.Device)
	c.parseInfoOutput(info, infoOutput)

	return nil
}

// parseSMARTOutput 解析 S.M.A.R.T. 输出
func (c *SMARTCollector) parseSMARTOutput(output string) []SMARTAttribute {
	var attrs []SMARTAttribute
	scanner := bufio.NewScanner(strings.NewReader(output))
	re := regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+(\d+)\s+(\d+)\s+(\d+)\s+\S+\s+\S+\s+\S+\s+(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			id, _ := strconv.Atoi(matches[1])
			value, _ := strconv.Atoi(matches[3])
			worst, _ := strconv.Atoi(matches[4])
			threshold, _ := strconv.Atoi(matches[5])
			rawValue, _ := strconv.ParseInt(matches[6], 10, 64)

			attr := SMARTAttribute{
				ID:        id,
				Name:      matches[2],
				Value:     value,
				Worst:     worst,
				Threshold: threshold,
				RawValue:  rawValue,
				Failed:    value <= threshold,
			}
			attrs = append(attrs, attr)
		}
	}
	return attrs
}

// parseInfoOutput 解析设备信息
func (c *SMARTCollector) parseInfoOutput(info *DiskInfo, output string) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Serial Number:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.Serial = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "Firmware Version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.Firmware = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "Power_On_Hours") {
			re := regexp.MustCompile(`\s+(\d+)\s*$`)
			if matches := re.FindStringSubmatch(line); matches != nil {
				info.PowerOnHours, _ = strconv.ParseInt(matches[1], 10, 64)
			}
		}
		if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "Airflow_Temperature") {
			re := regexp.MustCompile(`\s+(\d+)\s+\(`)
			if matches := re.FindStringSubmatch(line); matches != nil {
				info.Temperature, _ = strconv.Atoi(matches[1])
			}
		}
	}
}

func timeNow() time.Time {
	return time.Now()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
