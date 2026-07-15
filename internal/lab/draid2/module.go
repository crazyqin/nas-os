// Package draid2 提供分布式热备（dRAID2）增强功能。
// 支持 dRAID2 配置生成、重建速度预估、热备池管理、故障切换演练等。
// 对标 TrueNAS SCALE ZFS 2.4 的 dRAID2 特性增强版。

package draid2

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// 阵列状态常量.
const (
	StatusActive     = "active"     // 正常运行
	StatusDegraded   = "degraded"   // 降级运行（有磁盘故障）
	StatusRebuilding = "rebuilding" // 正在重建
	StatusResilvering = "resilvering" // 正在银化
	StatusFailed     = "failed"     // 已失败
	StatusCreating   = "creating"   // 创建中
)

// 热备状态常量.
const (
	SpareActive    = "active"    // 活跃热备
	SpareStandby   = "standby"   // 待机热备
	SpareRebuilding = "rebuilding" // 正在重建
	SpareFaulty    = "faulty"    // 故障
)

// 演练状态常量.
const (
	DrillPending   = "pending"   // 等待执行
	DrillRunning   = "running"   // 正在执行
	DrillPassed    = "passed"    // 通过
	DrillFailed    = "failed"    // 失败
	DrillCancelled = "cancelled" // 已取消
)

// DRAID2Config dRAID2 配置.
type DRAID2Config struct {
	ID            string   `json:"id"`              // 阵列 ID
	Name          string   `json:"name"`            // 阵列名称
	Devices       []string `json:"devices"`         // 数据磁盘列表
	SpareDevices  []string `json:"spare_devices"`   // 热备磁盘列表
	ParityDisks   int      `json:"parity_disks"`   // 校验磁盘数 (dRAID2 固定为 2)
	DataDisks     int      `json:"data_disks"`     // 数据磁盘数
	TotalDisks    int      `json:"total_disks"`    // 总磁盘数
	GroupSize     int      `json:"group_size"`     // 重建组大小
	ChunkSize     string   `json:"chunk_size"`      // 块大小 (如 "128K", "256K")
	StripeSize    string   `json:"stripe_size"`    // 条带大小
	Status        string   `json:"status"`          // 阵列状态
	UsableCapacity int64   `json:"usable_capacity"` // 可用容量 (bytes)
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
}

// RebuildEstimate 重建速度预估.
type RebuildEstimate struct {
	ArrayID          string        `json:"array_id"`           // 阵列 ID
	FailedDisk       string        `json:"failed_disk"`        // 故障磁盘
	ReplacementDisk  string        `json:"replacement_disk"`   // 替换磁盘
	TotalBytes       int64         `json:"total_bytes"`        // 需重建总字节数
	EstimatedSpeed   int64         `json:"estimated_speed"`    // 预估速度 (bytes/s)
	EstimatedSeconds int64         `json:"estimated_seconds"`  // 预估耗时 (秒)
	EstimatedTime    time.Duration `json:"estimated_time"`     // 预估耗时
	ParallelRebuilds int           `json:"parallel_rebuilds"`  // 并行重建数
	ParallelGroups   int           `json:"parallel_groups"`   // 并行组数
	ImpactOnPerf    float64       `json:"impact_on_perf"`     // 对性能的影响 (%)
	StartTime       *time.Time    `json:"start_time,omitempty"` // 开始时间
	EndTime         *time.Time    `json:"end_time,omitempty"`   // 预计结束时间
}

// SparePool 热备池.
type SparePool struct {
	ID         string        `json:"id"`           // 热备池 ID
	Name       string        `json:"name"`         // 热备池名称
	ArrayID    string        `json:"array_id"`     // 关联的阵列 ID
	Spares     []SpareDisk   `json:"spares"`       // 热备磁盘列表
	MinSpares  int           `json:"min_spares"`   // 最少热备数
	MaxSpares  int           `json:"max_spares"`   // 最多热备数
	AutoAssign bool          `json:"auto_assign"`  // 是否自动分配
	CreatedAt  time.Time     `json:"created_at"`   // 创建时间
	UpdatedAt  time.Time     `json:"updated_at"`    // 更新时间
}

// SpareDisk 热备磁盘信息.
type SpareDisk struct {
	Device     string    `json:"device"`      // 设备路径
	Status     string    `json:"status"`      // 状态
	Capacity   int64     `json:"capacity"`     // 容量 (bytes)
	Model      string    `json:"model,omitempty"` // 型号
	Serial     string    `json:"serial,omitempty"` // 序列号
	AssignedTo string   `json:"assigned_to,omitempty"` // 分配到的故障设备
	AddedAt    time.Time `json:"added_at"`    // 添加时间
}

// FailoverDrill 故障切换演练.
type FailoverDrill struct {
	ID            string    `json:"id"`                       // 演练 ID
	ArrayID       string    `json:"array_id"`                 // 阵列 ID
	Name          string    `json:"name"`                     // 演练名称
	Status        string    `json:"status"`                   // 演练状态
	SimulatedDisk string    `json:"simulated_disk"`           // 模拟故障磁盘
	SpareUsed     string    `json:"spare_used,omitempty"`    // 使用的热备盘
	StartTime     time.Time `json:"start_time"`              // 开始时间
	EndTime       *time.Time `json:"end_time,omitempty"`     // 结束时间
	RebuildSeconds int64   `json:"rebuild_seconds,omitempty"` // 实际重建耗时 (秒)
	Passed        bool      `json:"passed"`                  // 是否通过
	Notes         []string  `json:"notes,omitempty"`          // 备注
	CreatedAt     time.Time `json:"created_at"`              // 创建时间
}

// Manager dRAID2 管理器.
type Manager struct {
	mu     sync.RWMutex
	arrays map[string]*DRAID2Config    // 阵列列表
	pools  map[string]*SparePool       // 热备池列表
	drills map[string]*FailoverDrill   // 演练列表
}

// NewManager 创建 dRAID2 管理器.
func NewManager() *Manager {
	return &Manager{
		arrays: make(map[string]*DRAID2Config),
		pools:  make(map[string]*SparePool),
		drills: make(map[string]*FailoverDrill),
	}
}

// GenerateConfig 生成 dRAID2 配置.
func (m *Manager) GenerateConfig(name string, devices, spares []string, groupSize int, chunkSize string) (*DRAID2Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("阵列名称不能为空")
	}
	if len(devices) < 4 {
		return nil, fmt.Errorf("dRAID2 至少需要 4 块数据磁盘")
	}
	if groupSize <= 0 {
		groupSize = len(devices)
	}

	// 检查名称唯一性
	for _, arr := range m.arrays {
		if arr.Name == name {
			return nil, fmt.Errorf("阵列名称 %s 已存在", name)
		}
	}

	config := &DRAID2Config{
		ID:           fmt.Sprintf("draid2-%s", name),
		Name:         name,
		Devices:      devices,
		SpareDevices: spares,
		ParityDisks:  2, // dRAID2 固定双校验
		DataDisks:    len(devices),
		TotalDisks:   len(devices) + len(spares),
		GroupSize:    groupSize,
		ChunkSize:    chunkSize,
		Status:       StatusActive,
		CreatedAt:    time.Now(),
	}

	// 计算可用容量 (粗略估算：数据盘数 / 总盘数 * 单盘容量)
	// 实际应取最小磁盘容量为基准
	config.UsableCapacity = int64(config.DataDisks) * 1_000_000_000 // 默认 1TB per disk

	m.arrays[config.ID] = config
	return config, nil
}

// EstimateRebuild 预估重建时间和速度.
func (m *Manager) EstimateRebuild(arrayID, failedDisk string) (*RebuildEstimate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[arrayID]
	if !exists {
		return nil, fmt.Errorf("阵列 %s 不存在", arrayID)
	}

	if arr.Status != StatusDegraded && arr.Status != StatusRebuilding {
		return nil, fmt.Errorf("阵列 %s 当前状态 (%s) 不需要重建", arrayID, arr.Status)
	}

	// 预估速度：基于组大小和块大小估算 (默认 200 MB/s)
	estimatedSpeed := int64(200 * 1024 * 1024) // 200 MB/s

	// 重建数据量：基于可用容量和数据盘占比
	totalBytes := arr.UsableCapacity

	// 并行重建数基于组大小
	parallelRebuilds := arr.GroupSize / arr.ParityDisks
	if parallelRebuilds < 1 {
		parallelRebuilds = 1
	}

	estimatedSeconds := totalBytes / (estimatedSpeed * int64(parallelRebuilds))
	if estimatedSeconds <= 0 {
		estimatedSeconds = 1
	}

	estimate := &RebuildEstimate{
		ArrayID:          arrayID,
		FailedDisk:       failedDisk,
		TotalBytes:       totalBytes,
		EstimatedSpeed:   estimatedSpeed,
		EstimatedSeconds: estimatedSeconds,
		EstimatedTime:    time.Duration(estimatedSeconds) * time.Second,
		ParallelRebuilds: parallelRebuilds,
		ParallelGroups:   arr.GroupSize,
		ImpactOnPerf:     float64(parallelRebuilds) / float64(arr.GroupSize) * 100,
	}

	now := time.Now()
	estimate.StartTime = &now
	endTime := now.Add(estimate.EstimatedTime)
	estimate.EndTime = &endTime

	return estimate, nil
}

// ManageSparePool 管理热备池.
func (m *Manager) ManageSparePool(arrayID string, pool *SparePool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.arrays[arrayID]; !exists {
		return fmt.Errorf("阵列 %s 不存在", arrayID)
	}

	if pool.ID == "" {
		return fmt.Errorf("热备池 ID 不能为空")
	}
	if _, exists := m.pools[pool.ID]; exists {
		return fmt.Errorf("热备池 %s 已存在", pool.ID)
	}

	pool.ArrayID = arrayID
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()

	// 默认值
	if pool.MinSpares <= 0 {
		pool.MinSpares = 1
	}
	if pool.MaxSpares <= 0 {
		pool.MaxSpares = 4
	}

	// 设置热备盘状态
	for i := range pool.Spares {
		if pool.Spares[i].Status == "" {
			pool.Spares[i].Status = SpareStandby
		}
		pool.Spares[i].AddedAt = time.Now()
	}

	m.pools[pool.ID] = pool
	return nil
}

// AddSpare 向热备池添加热备盘.
func (m *Manager) AddSpare(poolID string, spare SpareDisk) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("热备池 %s 不存在", poolID)
	}

	if len(pool.Spares) >= pool.MaxSpares {
		return fmt.Errorf("热备池已满（最大 %d 块）", pool.MaxSpares)
	}

	spare.Status = SpareStandby
	spare.AddedAt = time.Now()
	pool.Spares = append(pool.Spares, spare)
	pool.UpdatedAt = time.Now()
	return nil
}

// RemoveSpare 从热备池移除热备盘.
func (m *Manager) RemoveSpare(poolID, device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("热备池 %s 不存在", poolID)
	}

	for i, spare := range pool.Spares {
		if spare.Device == device {
			if spare.Status == SpareActive || spare.Status == SpareRebuilding {
				return fmt.Errorf("热备盘 %s 当前状态为 %s，不可移除", device, spare.Status)
			}
			pool.Spares = append(pool.Spares[:i], pool.Spares[i+1:]...)
			pool.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("热备盘 %s 不在热备池中", device)
}

// GetSparePool 获取热备池信息.
func (m *Manager) GetSparePool(poolID string) (*SparePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("热备池 %s 不存在", poolID)
	}
	return pool, nil
}

// ListSparePools 列出热备池.
func (m *Manager) ListSparePools(arrayID string) []*SparePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]*SparePool, 0)
	for _, pool := range m.pools {
		if arrayID != "" && pool.ArrayID != arrayID {
			continue
		}
		pools = append(pools, pool)
	}
	return pools
}

// RunDrill 执行故障切换演练.
func (m *Manager) RunDrill(drill *FailoverDrill) (*FailoverDrill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if drill.ID == "" {
		return nil, fmt.Errorf("演练 ID 不能为空")
	}
	if _, exists := m.arrays[drill.ArrayID]; !exists {
		return nil, fmt.Errorf("阵列 %s 不存在", drill.ArrayID)
	}
	if _, exists := m.drills[drill.ID]; exists {
		return nil, fmt.Errorf("演练 %s 已存在", drill.ID)
	}

	drill.Status = DrillPending
	drill.Passed = false
	drill.Notes = []string{}
	drill.CreatedAt = time.Now()

	// 找到可用的热备盘
	for _, pool := range m.pools {
		if pool.ArrayID != drill.ArrayID {
			continue
		}
		for _, spare := range pool.Spares {
			if spare.Status == SpareStandby {
				drill.SpareUsed = spare.Device
				break
			}
		}
		if drill.SpareUsed != "" {
			break
		}
	}

	m.drills[drill.ID] = drill

	// 模拟演练执行
	drill.Status = DrillRunning
	drill.StartTime = time.Now()

	// 模拟通过
	drill.Passed = true
	drill.Status = DrillPassed
	endTime := time.Now()
	drill.EndTime = &endTime
	drill.RebuildSeconds = int64(endTime.Sub(drill.StartTime).Seconds())

	return drill, nil
}

// GetDrill 获取演练结果.
func (m *Manager) GetDrill(drillID string) (*FailoverDrill, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	drill, exists := m.drills[drillID]
	if !exists {
		return nil, fmt.Errorf("演练 %s 不存在", drillID)
	}
	return drill, nil
}

// ListDrills 列出演练.
func (m *Manager) ListDrills(arrayID string) []*FailoverDrill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	drills := make([]*FailoverDrill, 0)
	for _, drill := range m.drills {
		if arrayID != "" && drill.ArrayID != arrayID {
			continue
		}
		drills = append(drills, drill)
	}
	sort.Slice(drills, func(i, j int) bool {
		return drills[i].CreatedAt.After(drills[j].CreatedAt)
	})
	return drills
}

// GetArray 获取阵列配置.
func (m *Manager) GetArray(arrayID string) (*DRAID2Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arr, exists := m.arrays[arrayID]
	if !exists {
		return nil, fmt.Errorf("阵列 %s 不存在", arrayID)
	}
	return arr, nil
}

// ListArrays 列出所有阵列.
func (m *Manager) ListArrays() []*DRAID2Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	arrays := make([]*DRAID2Config, 0)
	for _, arr := range m.arrays {
		arrays = append(arrays, arr)
	}
	sort.Slice(arrays, func(i, j int) bool {
		return arrays[i].CreatedAt.After(arrays[j].CreatedAt)
	})
	return arrays
}

// UpdateArrayStatus 更新阵列状态.
func (m *Manager) UpdateArrayStatus(arrayID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arr, exists := m.arrays[arrayID]
	if !exists {
		return fmt.Errorf("阵列 %s 不存在", arrayID)
	}

	arr.Status = status
	return nil
}

// DeleteArray 删除阵列.
func (m *Manager) DeleteArray(arrayID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.arrays[arrayID]; !exists {
		return fmt.Errorf("阵列 %s 不存在", arrayID)
	}

	// 检查关联的热备池
	for _, pool := range m.pools {
		if pool.ArrayID == arrayID {
			return fmt.Errorf("阵列 %s 仍有关联的热备池 %s，请先删除", arrayID, pool.ID)
		}
	}

	delete(m.arrays, arrayID)

	// 删除关联的演练记录
	for id, drill := range m.drills {
		if drill.ArrayID == arrayID {
			delete(m.drills, id)
		}
	}

	return nil
}