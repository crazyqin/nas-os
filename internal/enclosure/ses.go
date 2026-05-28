// Package enclosure 提供 SES (SCSI Enclosure Services) 协议实现
package enclosure

import (
	"fmt"
	"os/exec"
	"strings"
)

// SESClient SES 协议客户端
type SESClient struct {
	// device SES 设备路径（如 /dev/sg0）
	device string
}

// NewSESClient 创建 SES 客户端
func NewSESClient(device string) *SESClient {
	return &SESClient{device: device}
}

// DiscoverEnclosures 发现系统中的 SES 设备
func DiscoverEnclosures() ([]string, error) {
	// 使用 sg_ses 工具发现 SES 设备
	out, err := exec.Command("sg_ses", "--list").CombinedOutput()
	if err != nil {
		// 如果 sg_ses 不可用，尝试扫描 /dev/sg* 设备
		return discoverByScanning()
	}

	var devices []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/dev/sg") {
			devices = append(devices, line)
		}
	}
	return devices, nil
}

// discoverByScanning 扫描 /dev/sg* 设备
func discoverByScanning() ([]string, error) {
	out, err := exec.Command("ls", "/dev/sg*").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("未找到 SES 设备: %w", err)
	}

	var devices []string
	for _, dev := range strings.Fields(string(out)) {
		// 简单验证设备是否响应 SES 命令
		if isSESDevice(dev) {
			devices = append(devices, dev)
		}
	}
	return devices, nil
}

// isSESDevice 检查设备是否为 SES 设备
func isSESDevice(device string) bool {
	// 通过 sg_inq 检查设备类型
	out, err := exec.Command("sg_inq", device).CombinedOutput()
	if err != nil {
		return false
	}
	// SES 设备的 peripheral type 为 0x0D (13)
	return strings.Contains(string(out), "Enclosure services device")
}

// GetEnclosureInfo 获取机箱信息
func (s *SESClient) GetEnclosureInfo() (*Enclosure, error) {
	if s.device == "" {
		return nil, fmt.Errorf("SES 设备路径为空")
	}

	enc := &Enclosure{
		ID:     s.device,
		Status: StatusOnline,
	}

	// 解析 SES 状态页面
	if err := s.parseStatusPage(enc); err != nil {
		return nil, fmt.Errorf("解析 SES 状态失败: %w", err)
	}

	return enc, nil
}

// parseStatusPage 解析 SES 状态页面
func (s *SESClient) parseStatusPage(enc *Enclosure) error {
	// 使用 sg_ses 获取状态
	out, err := exec.Command("sg_ses", "--page=status", s.device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("读取 SES 状态失败: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	slotID := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Array Device Slot") || strings.Contains(line, "Device Slot") {
			slot := &Slot{
				ID:         slotID,
				Status:     SlotActive,
				LEDStates:  make(map[LEDType]LEDState),
				DiskPresent: true,
			}
			enc.Slots = append(enc.Slots, slot)
			slotID++
		}
	}
	return nil
}

// SetSlotLED 设置槽位 LED 状态
func (s *SESClient) SetSlotLED(slotID int, ledType LEDType, state LEDState) error {
	if s.device == "" {
		return fmt.Errorf("SES 设备路径为空")
	}

	// 构建 sg_ses 控制命令
	controlByte := buildControlByte(ledType, state)
	cmd := exec.Command("sg_ses", "--index", fmt.Sprintf("%d", slotID),
		"--set", controlByte, s.device)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("设置 LED 失败: %s: %w", string(output), err)
	}
	return nil
}

// buildControlByte 构建 SES 控制字节
func buildControlByte(ledType LEDType, state LEDState) string {
	switch ledType {
	case LEDLocate:
		if state == LEDOn || state == LEDBlink {
			return "locate=1"
		}
		return "locate=0"
	case LEDFault:
		if state == LEDOn {
			return "fault=1"
		}
		return "fault=0"
	default:
		return ""
	}
}

// GetSensors 获取 SES 传感器信息
func (s *SESClient) GetSensors() ([]*Sensor, error) {
	if s.device == "" {
		return nil, fmt.Errorf("SES 设备路径为空")
	}

	out, err := exec.Command("sg_ses", "--page=emc", s.device).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("读取传感器失败: %w", err)
	}

	var sensors []*Sensor
	lines := strings.Split(string(out), "\n")
	sensorID := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Temperature") {
			sensor := &Sensor{
				ID:     sensorID,
				Name:   fmt.Sprintf("温度传感器 %d", sensorID),
				Type:   SensorTemperature,
				Unit:   "°C",
				Status: SensorNormal,
			}
			sensors = append(sensors, sensor)
			sensorID++
		}
	}
	return sensors, nil
}
