// DRAID (分布式 RAID) 管理模块
// 提供分布式 RAID 阵列的创建、删除、状态查询、热备管理、数据重分布等功能
// 参考 TrueNAS SCALE 的 dRAID 特性实现

package draid

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DRAID 级别常量
const (
	DRAID1 = "DRAID1" // 单重校验，等效 RAID5 的分布式版本
	DRAID2 = "DRAID2" // 双重校验，等效 RAID6 的分布式版本
	DRAID3 = "DRAID3" // 三重校验，最高冗余级别
)

// 阵列状态常量
const (
	StatusActive     = "active"     // 正常运行
	StatusDegraded   = "degraded"   // 降级运行（有磁盘故障）
	StatusRebuilding = "rebuilding" // 正在重建
	StatusResharing  = "resharing"  // 正在重分布数据
	StatusFailed     = "failed"     // 已失败
	StatusCreating   = "creating"   // 创建中
)

// draidLevels 存储合法的 DRAID 级别
var draidLevels = map[string]bool{
	DRAID1: true,
	DRAID2: true,
	DRAID3: true,
}

// levelParityMap 映射 DRAID 级别到校验磁盘数量
var levelParityMap = map[string]int{
	DRAID1: 1,
	DRAID2: 2,
	DRAID3: 3,
}

// PerformanceMetrics 性能监控指标
type PerformanceMetrics struct {
	IOPSRead        int64     `json:"iops_read"`        // 每秒读操作数
	IOPSWrite       int64     `json:"iops_write"`       // 每秒写操作数
	ThroughputRead  int64     `json:"throughput_read"`  // 读吞吐量 (bytes/s)
	ThroughputWrite int64     `json:"throughput_write"` // 写吞吐量 (bytes/s)
	LatencyRead     float64   `json:"latency_read"`     // 读延迟 (ms)
	LatencyWrite    float64   `json:"latency_write"`    // 写延迟 (ms)
	Timestamp       time.Time `json:"timestamp"`        // 采集时间
}

// DistributedSpare 分布式热备信息
type DistributedSpare struct {
	Device      string    `json:"device"`       // 设备路径
	Status      string    `json:"status"`       // 状态: active/inactive/replacing
	AssignedTo  string    `json:"assigned_to"`  // 分配到的故障设备
	ActivatedAt time.Time `json:"activated_at"` // 激活时间
}

// DRAIDArray 表示一个 DRAID 阵列
type DRAIDArray struct {
	Name              string              `json:"name"`               // 阵列名称
	Level             string              `json:"level"`              // DRAID 级别 (DRAID1/DRAID2/DRAID3)
	Devices           []string            `json:"devices"`            // 数据+校验设备列表
	DistributedSpares []*DistributedSpare `json:"distributed_spares"` // 分布式热备列表
	GroupSize         int                 `json:"group_size"`         // 每组设备数（数据盘+校验盘）
	DataDisks         int                 `json:"data_disks"`         // 每组数据盘数量
	ParityDisks       int                 `json:"parity_disks"`       // 每组校验盘数量
	ChunkSize         string              `json:"chunk_size"`         // 条带大小
	Status            string              `json:"status"`             // 阵列状态
	TotalSize         int64               `json:"total_size"`         // 总容量 (bytes)
	UsedSize          int64               `json:"used_size"`          // 已用容量 (bytes)
	RebuildProgress   float64             `json:"rebuild_progress"`   // 重建进度 (0-100)
	ReshareProgress   float64             `json:"reshare_progress"`   // 重分布进度 (0-100)
	Metrics           *PerformanceMetrics `json:"metrics"`            // 性能指标
	FailedDevices     []string            `json:"failed_devices"`     // 故障设备列表
	CreatedAt         time.Time           `json:"created_at"`         // 创建时间
	UpdatedAt         time.Time           `json:"updated_at"`         // 更新时间
}

// Manager 管理 DRAID 阵列
type Manager struct {
	mu     sync.RWMutex
	arrays map[string]*DRAIDArray
}

// NewManager 创建新的 DRAID 管理器
func NewManager() *Manager {
	return &Manager{
		arrays: make(map[string]*DRAIDArray),
	}
}

// validateDRAIDParams 验证 DRAID 创建参数
func validateDRAIDParams(name, level string, devices []string, groupSize, dataDisks int) error {
	if name == "" {
		return fmt.Errorf("阵列名称不能为空")
	}
	if !draidLevels[level] {
		return fmt.Errorf("无效的 DRAID 级别: %s (支持: DRAID1, DRAID2, DRAID3)", level)
	}
	parityDisks := levelParityMap[level]
	if dataDisks < 1 {
		return fmt.Errorf("数据盘数量必须 >= 1")
	}
	if groupSize < dataDisks+parityDisks {
		return fmt.Errorf("组大小必须 >= 数据盘(%d) + 校验盘(%d) = %d", dataDisks, parityDisks, dataDisks+parityDisks)
	}
	if len(devices) < groupSize {
		return fmt.Errorf("设备数量(%d) 必须 >= 组大小(%d)", len(devices), groupSize)
	}
	if len(devices)%groupSize != 0 {
		return fmt.Errorf("设备数量(%d) 必须是组大小(%d) 的整数倍", len(devices), groupSize)
	}
	return nil
}

// CreateArray 创建新的 DRAID 阵列
func (m *Manager) CreateArray(name, level string, devices []string, spareDevices []string, groupSize, dataDisks int, chunkSize string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.arrays[name]; exists {
		return fmt.Errorf("阵列已存在: %s", name)
	}

	if err := validateDRAIDParams(name, level, devices, groupSize, dataDisks); err != nil {
		return err
	}

	// 检查设备重复
	deviceSet := make(map[string]bool)
	for _, d := range devices {
		if deviceSet[d] {
			return fmt.Errorf("设备重复: %s", d)
		}
		deviceSet[d] = true
	}

	parityDisks := levelParityMap[level]

	// 创建分布式热备列表
	distSpares := make([]*DistributedSpare, 0, len(spareDevices))
	for _, sp := range spareDevices {
		if deviceSet[sp] {
			return fmt.Errorf("热备设备与阵列设备重复: %s", sp)
		}
		distSpares = append(distSpares, &DistributedSpare{
			Device: sp,
			Status: "active",
		})
	}

	// 设置默认条带大小
	if chunkSize == "" {
		chunkSize = "128K"
	}

	now := time.Now()
	m.arrays[name] = &DRAIDArray{
		Name:              name,
		Level:             level,
		Devices:           devices,
		DistributedSpares: distSpares,
		GroupSize:         groupSize,
		DataDisks:         dataDisks,
		ParityDisks:       parityDisks,
		ChunkSize:         chunkSize,
		Status:            StatusActive,
		TotalSize:         int64(dataDisks) * int64(len(devices)/groupSize) * 1024 * 1024 * 1024,
		UsedSize:          0,
		RebuildProgress:   0,
		ReshareProgress:   0,
		Metrics:           &PerformanceMetrics{},
		FailedDevices:     []string{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	log.Printf("DRAID 阵列已创建: %s (级别: %s, 组大小: %d, 数据盘: %d, 设备数: %d)",
		name, level, groupSize, dataDisks, len(devices))
	return nil
}

// DeleteArray 删除 DRAID 阵列
func (m *Manager) DeleteArray(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}
	if arr.Status == StatusRebuilding || arr.Status == StatusResharing {
		return fmt.Errorf("阵列正在 %s，无法删除", arr.Status)
	}

	delete(m.arrays, name)
	log.Printf("DRAID 阵列已删除: %s", name)
	return nil
}

// GetArray 获取指定 DRAID 阵列信息
func (m *Manager) GetArray(name string) (*DRAIDArray, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[name]
	if !exists {
		return nil, fmt.Errorf("阵列不存在: %s", name)
	}
	return arr, nil
}

// ListArrays 列出所有 DRAID 阵列
func (m *Manager) ListArrays() []DRAIDArray {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]DRAIDArray, 0, len(m.arrays))
	for _, arr := range m.arrays {
		result = append(result, *arr)
	}
	return result
}

// AddDistributedSpare 添加分布式热备
func (m *Manager) AddDistributedSpare(name, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}

	// 检查设备是否已在使用
	for _, d := range arr.Devices {
		if d == device {
			return fmt.Errorf("设备已在阵列中使用: %s", device)
		}
	}
	for _, sp := range arr.DistributedSpares {
		if sp.Device == device {
			return fmt.Errorf("设备已是分布式热备: %s", device)
		}
	}

	arr.DistributedSpares = append(arr.DistributedSpares, &DistributedSpare{
		Device: device,
		Status: "active",
	})
	arr.UpdatedAt = time.Now()

	log.Printf("已向 DRAID 阵列 %s 添加分布式热备: %s", name, device)
	return nil
}

// RemoveDistributedSpare 移除分布式热备
func (m *Manager) RemoveDistributedSpare(name, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}

	for i, sp := range arr.DistributedSpares {
		if sp.Device == device {
			if sp.Status == "replacing" {
				return fmt.Errorf("热备正在替换设备，无法移除: %s", device)
			}
			arr.DistributedSpares = append(arr.DistributedSpares[:i], arr.DistributedSpares[i+1:]...)
			arr.UpdatedAt = time.Now()
			log.Printf("已从 DRAID 阵列 %s 移除分布式热备: %s", name, device)
			return nil
		}
	}
	return fmt.Errorf("设备不在分布式热备列表中: %s", device)
}

// ListDistributedSpares 列出所有分布式热备
func (m *Manager) ListDistributedSpares(name string) ([]*DistributedSpare, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[name]
	if !exists {
		return nil, fmt.Errorf("阵列不存在: %s", name)
	}

	result := make([]*DistributedSpare, len(arr.DistributedSpares))
	copy(result, arr.DistributedSpares)
	return result, nil
}

// ReportDeviceFailure 报告设备故障，自动触发分布式热备替换
func (m *Manager) ReportDeviceFailure(name, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}

	// 检查设备是否在阵列中
	found := false
	for _, d := range arr.Devices {
		if d == device {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("设备不在阵列中: %s", device)
	}

	// 检查是否已报告故障
	for _, fd := range arr.FailedDevices {
		if fd == device {
			return fmt.Errorf("设备已报告故障: %s", device)
		}
	}

	// 查找可用的分布式热备
	var spare *DistributedSpare
	for _, sp := range arr.DistributedSpares {
		if sp.Status == "active" {
			spare = sp
			break
		}
	}

	arr.FailedDevices = append(arr.FailedDevices, device)
	arr.UpdatedAt = time.Now()

	// 根据故障设备数量更新状态
	parityDisks := arr.ParityDisks
	if len(arr.FailedDevices) > parityDisks {
		arr.Status = StatusFailed
		log.Printf("DRAID 阵列 %s 已失败: 故障设备数(%d) > 校验盘数(%d)", name, len(arr.FailedDevices), parityDisks)
	} else {
		arr.Status = StatusDegraded
	}

	// 自动分配热备（保持 degraded 状态，等待显式调用 RebuildArray 触发重建）
	if spare != nil {
		spare.Status = "assigned"
		spare.AssignedTo = device
		spare.ActivatedAt = time.Now()
		log.Printf("DRAID 阵列 %s: 分配热备 %s 替换故障设备 %s（等待重建）", name, spare.Device, device)
	}

	return nil
}

// RebuildArray 触发阵列重建（使用分布式热备）
func (m *Manager) RebuildArray(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}
	if arr.Status != StatusDegraded && arr.Status != StatusFailed {
		return fmt.Errorf("只有降级或失败的阵列才能重建，当前状态: %s", arr.Status)
	}

	// 检查是否有可用热备（active 或已分配的）
	hasActiveSpare := false
	for _, sp := range arr.DistributedSpares {
		if sp.Status == "active" || sp.Status == "assigned" {
			hasActiveSpare = true
			sp.Status = "rebuilding"
			break
		}
	}
	if !hasActiveSpare {
		return fmt.Errorf("没有可用的分布式热备进行重建")
	}

	arr.Status = StatusRebuilding
	arr.RebuildProgress = 0
	arr.UpdatedAt = time.Now()

	log.Printf("DRAID 阵列 %s 开始重建", name)
	return nil
}

// UpdateRebuildProgress 更新重建进度
func (m *Manager) UpdateRebuildProgress(name string, progress float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}
	if arr.Status != StatusRebuilding {
		return fmt.Errorf("阵列当前不在重建状态: %s", arr.Status)
	}
	if progress < 0 || progress > 100 {
		return fmt.Errorf("进度必须在 0-100 之间")
	}

	arr.RebuildProgress = progress
	arr.UpdatedAt = time.Now()

	// 重建完成
	if progress >= 100 {
		arr.Status = StatusActive
		arr.FailedDevices = []string{}
		arr.RebuildProgress = 100
		// 将替换中的热备标记为已完成
		for _, sp := range arr.DistributedSpares {
			if sp.Status == "replacing" {
				sp.Status = "active"
				sp.AssignedTo = ""
			}
		}
		log.Printf("DRAID 阵列 %s 重建完成", name)
	}
	return nil
}

// ReshareData 重分布数据（当添加新设备后重新分配数据）
func (m *Manager) ReshareData(name string, newDevices []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}
	if arr.Status != StatusActive {
		return fmt.Errorf("只有正常状态的阵列才能重分布数据，当前状态: %s", arr.Status)
	}
	if len(newDevices) == 0 {
		return fmt.Errorf("新设备列表不能为空")
	}

	// 检查新设备是否与现有设备冲突
	deviceSet := make(map[string]bool)
	for _, d := range arr.Devices {
		deviceSet[d] = true
	}
	for _, nd := range newDevices {
		if deviceSet[nd] {
			return fmt.Errorf("设备已在阵列中: %s", nd)
		}
	}

	arr.Status = StatusResharing
	arr.ReshareProgress = 0
	arr.UpdatedAt = time.Now()

	log.Printf("DRAID 阵列 %s 开始数据重分布，新增 %d 个设备", name, len(newDevices))
	return nil
}

// UpdateReshareProgress 更新数据重分布进度
func (m *Manager) UpdateReshareProgress(name string, progress float64, newDevices []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}
	if arr.Status != StatusResharing {
		return fmt.Errorf("阵列当前不在重分布状态: %s", arr.Status)
	}
	if progress < 0 || progress > 100 {
		return fmt.Errorf("进度必须在 0-100 之间")
	}

	arr.ReshareProgress = progress
	arr.UpdatedAt = time.Now()

	// 重分布完成
	if progress >= 100 {
		arr.Devices = append(arr.Devices, newDevices...)
		arr.Status = StatusActive
		arr.ReshareProgress = 100
		// 更新总容量
		arr.TotalSize = int64(arr.DataDisks) * int64(len(arr.Devices)/arr.GroupSize) * 1024 * 1024 * 1024
		log.Printf("DRAID 阵列 %s 数据重分布完成，设备数: %d", name, len(arr.Devices))
	}
	return nil
}

// UpdateMetrics 更新性能指标
func (m *Manager) UpdateMetrics(name string, metrics *PerformanceMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[name]
	if !exists {
		return fmt.Errorf("阵列不存在: %s", name)
	}

	metrics.Timestamp = time.Now()
	arr.Metrics = metrics
	arr.UpdatedAt = time.Now()
	return nil
}

// GetMetrics 获取性能指标
func (m *Manager) GetMetrics(name string) (*PerformanceMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[name]
	if !exists {
		return nil, fmt.Errorf("阵列不存在: %s", name)
	}
	return arr.Metrics, nil
}

// GetArrayStatus 获取 DRAID 阵列详细状态
func (m *Manager) GetArrayStatus(name string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[name]
	if !exists {
		return nil, fmt.Errorf("阵列不存在: %s", name)
	}

	activeSpares := 0
	replacingSpares := 0
	for _, sp := range arr.DistributedSpares {
		switch sp.Status {
		case "active":
			activeSpares++
		case "replacing":
			replacingSpares++
		}
	}

	status := map[string]interface{}{
		"name":               arr.Name,
		"level":              arr.Level,
		"status":             arr.Status,
		"devices":            arr.Devices,
		"distributed_spares": arr.DistributedSpares,
		"group_size":         arr.GroupSize,
		"data_disks":         arr.DataDisks,
		"parity_disks":       arr.ParityDisks,
		"chunk_size":         arr.ChunkSize,
		"total_size":         arr.TotalSize,
		"used_size":          arr.UsedSize,
		"rebuild_progress":   arr.RebuildProgress,
		"reshare_progress":   arr.ReshareProgress,
		"failed_devices":     arr.FailedDevices,
		"metrics":            arr.Metrics,
		"device_count":       len(arr.Devices),
		"spare_count":        len(arr.DistributedSpares),
		"active_spares":      activeSpares,
		"replacing_spares":   replacingSpares,
		"failed_count":       len(arr.FailedDevices),
		"created_at":         arr.CreatedAt,
		"updated_at":         arr.UpdatedAt,
	}
	return status, nil
}
