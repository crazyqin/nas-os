package raid

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// RAID 级别枚举
const (
	RAID0    = "RAID0"
	RAID1    = "RAID1"
	RAID5    = "RAID5"
	RAID6    = "RAID6"
	RAID10   = "RAID10"
	RAIDZ1   = "RAIDZ1"
	RAIDZ2   = "RAIDZ2"
	RAIDZ3   = "RAIDZ3"
	DRAID1   = "DRAID1"
	DRAID2   = "DRAID2"
	DRAID3   = "DRAID3"
)

// validLevels 存储所有合法的 RAID 级别
var validLevels = map[string]bool{
	RAID0:  true,
	RAID1:  true,
	RAID5:  true,
	RAID6:  true,
	RAID10: true,
	RAIDZ1: true,
	RAIDZ2: true,
	RAIDZ3: true,
	DRAID1: true,
	DRAID2: true,
	DRAID3: true,
}

// RAIDArray 表示一个 RAID 阵列
type RAIDArray struct {
	Name         string    `json:"name"`
	Level        string    `json:"level"`
	Devices      []string  `json:"devices"`
	SpareDevices []string  `json:"spare_devices"`
	ChunkSize    string    `json:"chunk_size"`
	Status       string    `json:"status"`
	TotalSize    int64     `json:"total_size"`
	UsedSize     int64     `json:"used_size"`
	CreatedAt    time.Time `json:"created_at"`
}

// Manager 管理 RAID 阵列
type Manager struct {
	mu     sync.RWMutex
	arrays map[string]*RAIDArray
}

// NewManager 创建新的 RAID 管理器
func NewManager() *Manager {
	return &Manager{
		arrays: make(map[string]*RAIDArray),
	}
}

// CreateArray 创建新的 RAID 阵列
func (m *Manager) CreateArray(name, level string, devices []string, spareDevices []string, chunkSize string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return fmt.Errorf("阵列名称不能为空")
	}
	if _, exists := m.arrays[name]; exists {
		return fmt.Errorf("阵列已存在: %s", name)
	}
	if !validLevels[level] {
		return fmt.Errorf("无效的 RAID 级别: %s", level)
	}
	if len(devices) < 2 {
		return fmt.Errorf("至少需要 2 个设备")
	}

	now := time.Now()
	m.arrays[name] = &RAIDArray{
		Name:         name,
		Level:        level,
		Devices:      devices,
		SpareDevices: spareDevices,
		ChunkSize:    chunkSize,
		Status:       "active",
		TotalSize:    int64(len(devices)) * 1024 * 1024 * 1024, // 模拟容量
		UsedSize:     0,
		CreatedAt:    now,
	}
	log.Printf("RAID 阵列已创建: %s (级别: %s, 设备数: %d)", name, level, len(devices))
	return nil
}

// DeleteArray 删除 RAID 阵列
func (m *Manager) DeleteArray(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.arrays[name]; !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}
	delete(m.arrays, name)
	log.Printf("RAID 阵列已删除: %s", name)
	return nil
}

// GetArray 获取指定 RAID 阵列信息
func (m *Manager) GetArray(name string) (*RAIDArray, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[name]
	if !exists {
		return nil, fmt.Errorf("阵列不存在: %s", name)
	}
	return arr, nil
}

// ListArrays 列出所有 RAID 阵列
func (m *Manager) ListArrays() []RAIDArray {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RAIDArray, 0, len(m.arrays))
	for _, arr := range m.arrays {
		result = append(result, *arr)
	}
	return result
}

// AddSpare 向阵列添加备用设备
func (m *Manager) AddSpare(name, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}

	for _, d := range arr.SpareDevices {
		if d == device {
			return fmt.Errorf("设备已是备用设备: %s", device)
		}
	}
	for _, d := range arr.Devices {
		if d == device {
			return fmt.Errorf("设备已在阵列中使用: %s", device)
		}
	}

	arr.SpareDevices = append(arr.SpareDevices, device)
	log.Printf("已向阵列 %s 添加备用设备: %s", name, device)
	return nil
}

// RemoveSpare 从阵列移除备用设备
func (m *Manager) RemoveSpare(name, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}

	for i, d := range arr.SpareDevices {
		if d == device {
			arr.SpareDevices = append(arr.SpareDevices[:i], arr.SpareDevices[i+1:]...)
			log.Printf("已从阵列 %s 移除备用设备: %s", name, device)
			return nil
		}
	}
	return fmt.Errorf("设备不在备用列表中: %s", device)
}

// RebuildArray 重建 RAID 阵列
func (m *Manager) RebuildArray(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}

	arr.Status = "rebuilding"
	log.Printf("RAID 阵列开始重建: %s", name)
	return nil
}

// ExpandArray 扩展 RAID 阵列
func (m *Manager) ExpandArray(name string, newDevices []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}
	if len(newDevices) == 0 {
		return fmt.Errorf("新设备列表不能为空")
	}

	for _, nd := range newDevices {
		for _, d := range arr.Devices {
			if d == nd {
				return fmt.Errorf("设备已在阵列中: %s", nd)
			}
		}
	}

	arr.Devices = append(arr.Devices, newDevices...)
	arr.TotalSize += int64(len(newDevices)) * 1024 * 1024 * 1024
	log.Printf("RAID 阵列 %s 已扩展, 新增 %d 个设备", name, len(newDevices))
	return nil
}

// GetArrayStatus 获取 RAID 阵列状态
func (m *Manager) GetArrayStatus(name string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[name]
	if !exists {
		return nil, fmt.Errorf("阵列不存在: %s", name)
	}

	status := map[string]interface{}{
		"name":          arr.Name,
		"level":         arr.Level,
		"status":        arr.Status,
		"devices":       arr.Devices,
		"spare_devices": arr.SpareDevices,
		"chunk_size":    arr.ChunkSize,
		"total_size":    arr.TotalSize,
		"used_size":     arr.UsedSize,
		"created_at":    arr.CreatedAt,
		"device_count":  len(arr.Devices),
		"spare_count":   len(arr.SpareDevices),
	}
	return status, nil
}
