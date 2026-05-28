// Package enclosure 提供 SGPIO (Serial General Purpose Input/Output) 控制
package enclosure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SGPIOController SGPIO 控制器
type SGPIOController struct {
	// basePath sysfs 中 SGPIO 的基础路径
	basePath string
}

// NewSGPIOController 创建 SGPIO 控制器
func NewSGPIOController(basePath string) *SGPIOController {
	if basePath == "" {
		basePath = "/sys/class/sgpio"
	}
	return &SGPIOController{basePath: basePath}
}

// DiscoverControllers 发现系统中的 SGPIO 控制器
func DiscoverControllers() ([]string, error) {
	basePath := "/sys/class/sgpio"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		// 备用路径
		basePath = "/sys/bus/gpio/devices"
		entries, err = os.ReadDir(basePath)
		if err != nil {
			return nil, fmt.Errorf("未找到 SGPIO 控制器: %w", err)
		}
	}

	var controllers []string
	for _, entry := range entries {
		if entry.IsDir() {
			controllers = append(controllers, filepath.Join(basePath, entry.Name()))
		}
	}
	return controllers, nil
}

// SetLED 通过 SGPIO 控制 LED
func (s *SGPIOController) SetLED(slotID int, ledType LEDType, state LEDState) error {
	gpioPath, err := s.findGPIOPath(slotID, ledType)
	if err != nil {
		return err
	}

	value := "0"
	if state == LEDOn {
		value = "1"
	} else if state == LEDBlink {
		// 对于闪烁，使用内核的定时器驱动
		value = "2"
	}

	if err := os.WriteFile(gpioPath, []byte(value), 0644); err != nil {
		return fmt.Errorf("写入 SGPIO 失败: %w", err)
	}
	return nil
}

// findGPIOPath 查找 SGPIO 路径
func (s *SGPIOController) findGPIOPath(slotID int, ledType LEDType) (string, error) {
	var suffix string
	switch ledType {
	case LEDLocate:
		suffix = "locate"
	case LEDFault:
		suffix = "fault"
	case LEDActivity:
		suffix = "activity"
	default:
		return "", fmt.Errorf("不支持的 LED 类型: %s", ledType)
	}

	// 尝试标准路径格式
	pattern := filepath.Join(s.basePath, fmt.Sprintf("slot%d", slotID), suffix)
	if _, err := os.Stat(pattern); err == nil {
		return pattern, nil
	}

	// 尝试备用路径格式
	pattern = filepath.Join(s.basePath, fmt.Sprintf("gpio%d_%s", slotID, suffix))
	if _, err := os.Stat(pattern); err == nil {
		return pattern, nil
	}

	return "", fmt.Errorf("未找到 SGPIO 路径: slot=%d, type=%s", slotID, ledType)
}

// GetLED 获取 SGPIO LED 状态
func (s *SGPIOController) GetLED(slotID int, ledType LEDType) (LEDState, error) {
	gpioPath, err := s.findGPIOPath(slotID, ledType)
	if err != nil {
		return LEDOff, err
	}

	data, err := os.ReadFile(gpioPath)
	if err != nil {
		return LEDOff, fmt.Errorf("读取 SGPIO 状态失败: %w", err)
	}

	value := strings.TrimSpace(string(data))
	switch value {
	case "1":
		return LEDOn, nil
	case "2":
		return LEDBlink, nil
	default:
		return LEDOff, nil
	}
}

// ScanSlots 扫描所有 SGPIO 槽位
func (s *SGPIOController) ScanSlots() ([]int, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("扫描 SGPIO 失败: %w", err)
	}

	var slots []int
	for _, entry := range entries {
		name := entry.Name()
		var slotID int
		if _, err := fmt.Sscanf(name, "slot%d", &slotID); err == nil {
			slots = append(slots, slotID)
		} else if _, err := fmt.Sscanf(name, "gpio%d", &slotID); err == nil {
			slots = append(slots, slotID)
		}
	}
	return slots, nil
}

// ResetAllLEDs 重置所有 LED 到默认状态
func (s *SGPIOController) ResetAllLEDs() error {
	slots, err := s.ScanSlots()
	if err != nil {
		return err
	}

	for _, slotID := range slots {
		for _, ledType := range []LEDType{LEDLocate, LEDFault, LEDActivity} {
			if err := s.SetLED(slotID, ledType, LEDOff); err != nil {
				// 忽略单个 LED 错误，继续处理
				continue
			}
		}
	}
	return nil
}
