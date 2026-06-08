package armadapter

import (
	"fmt"
	"log"
	"sync"
)

// ========== ARM 适配管理器 ==========

// Manager ARM 架构适配管理器
type Manager struct {
	mu          sync.RWMutex
	detector    *Detector
	checker     *CompatibilityChecker
	optimizer   *Optimizer
	info        *ARMHardwareInfo
	compatReport *CompatReport
	optProfile  *OptProfile
	initialized bool
}

// NewManager 创建 ARM 适配管理器
func NewManager() *Manager {
	return &Manager{
		detector:  NewDetector(),
		checker:   NewCompatibilityChecker(),
		optimizer: NewOptimizer(),
	}
}

// Init 初始化并执行完整检测
func (m *Manager) Init() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	// 1. 检测硬件
	info, err := m.detector.Detect()
	if err != nil {
		return fmt.Errorf("hardware detection failed: %w", err)
	}
	m.info = info

	// 2. 兼容性检查
	report, err := m.checker.Check(info)
	if err != nil {
		return fmt.Errorf("compatibility check failed: %w", err)
	}
	m.compatReport = report

	// 3. 生成优化建议
	profile, err := m.optimizer.GenerateProfile(info)
	if err != nil {
		return fmt.Errorf("optimization generation failed: %w", err)
	}
	m.optProfile = profile

	m.initialized = true

	log.Printf("[ARM适配] 初始化完成: arch=%s soc=%s compat=%s score=%d opts=%d",
		info.ArchType, info.SoCModel, report.Overall, report.Score, len(profile.Optimizations))

	return nil
}

// GetHardwareInfo 获取硬件信息
func (m *Manager) GetHardwareInfo() (*ARMHardwareInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, fmt.Errorf("manager not initialized, call Init() first")
	}
	return m.info, nil
}

// GetCompatReport 获取兼容性报告
func (m *Manager) GetCompatReport() (*CompatReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, fmt.Errorf("manager not initialized, call Init() first")
	}
	return m.compatReport, nil
}

// GetOptProfile 获取优化配置档案
func (m *Manager) GetOptProfile() (*OptProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, fmt.Errorf("manager not initialized, call Init() first")
	}
	return m.optProfile, nil
}

// GetSupportedDevices 获取支持的设备列表
func (m *Manager) GetSupportedDevices() []SupportedDevice {
	return m.checker.GetSupportedDevices()
}

// IsCompatible 检查当前设备是否兼容
func (m *Manager) IsCompatible() (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return false, fmt.Errorf("manager not initialized, call Init() first")
	}

	return m.compatReport.Overall >= CompatPartial, nil
}

// GetOptimizationsByCategory 按类别获取优化建议
func (m *Manager) GetOptimizationsByCategory(category OptCategory) ([]Optimization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, fmt.Errorf("manager not initialized, call Init() first")
	}

	var result []Optimization
	for _, opt := range m.optProfile.Optimizations {
		if opt.Category == category {
			result = append(result, opt)
		}
	}
	return result, nil
}

// GetHighPriorityOpts 获取高优先级优化建议
func (m *Manager) GetHighPriorityOpts() ([]Optimization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, fmt.Errorf("manager not initialized, call Init() first")
	}

	var result []Optimization
	for _, opt := range m.optProfile.Optimizations {
		if opt.Priority == OptPriorityHigh {
			result = append(result, opt)
		}
	}
	return result, nil
}

// GetSummary 获取适配摘要
func (m *Manager) GetSummary() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.initialized {
		return nil, fmt.Errorf("manager not initialized, call Init() first")
	}

	summary := map[string]interface{}{
		"arch_type":     string(m.info.ArchType),
		"arch_version":  m.info.ArchVersion,
		"bits":          m.info.Bits,
		"soc":           string(m.info.SoC),
		"soc_model":     m.info.SoCModel,
		"cpu_cores":     m.info.CPUCores,
		"big_cores":     m.info.BigCores,
		"little_cores":  m.info.LittleCores,
		"max_freq_mhz":  m.info.MaxFreqMHz,
		"memory_mb":     m.info.MemoryMB,
		"features":      m.info.Features,
		"has_usb3":      m.info.HasUSB3,
		"has_pcie":      m.info.HasPCIe,
		"has_sata":      m.info.HasSATA,
		"has_gbe":       m.info.HasGbE,
		"has_2_5gbe":    m.info.Has2_5GbE,
		"compat_overall": m.compatReport.Overall.String(),
		"compat_score":   m.compatReport.Score,
		"compat_issues":  len(m.compatReport.Issues),
		"opt_count":      len(m.optProfile.Optimizations),
		"compatible":     m.compatReport.Overall >= CompatPartial,
	}

	return summary, nil
}

// Reset 重置管理器，强制重新检测
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.detector.Reset()
	m.info = nil
	m.compatReport = nil
	m.optProfile = nil
	m.initialized = false

	log.Println("[ARM适配] 管理器已重置")
}
