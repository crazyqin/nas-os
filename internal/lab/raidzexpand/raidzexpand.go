package raidzexpand

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RAIDZLevel RAIDZ 级别.
type RAIDZLevel string

const (
	RAIDZ1 RAIDZLevel = "raidz1"
	RAIDZ2 RAIDZLevel = "raidz2"
	RAIDZ3 RAIDZLevel = "raidz3"
)

// ExpansionPhase 扩展阶段.
type ExpansionPhase string

const (
	PhaseIdle        ExpansionPhase = "idle"
	PhaseValidating  ExpansionPhase = "validating"
	PhaseAddingDisk  ExpansionPhase = "adding_disk"
	PhaseResilvering ExpansionPhase = "resilvering"
	PhaseCompleted   ExpansionPhase = "completed"
	PhaseFailed      ExpansionPhase = "failed"
	PhaseCancelled   ExpansionPhase = "cancelled"
)

// PoolHealth 池健康状态.
type PoolHealth string

const (
	HealthOnline   PoolHealth = "ONLINE"
	HealthDegraded PoolHealth = "DEGRADED"
	HealthFaulted  PoolHealth = "FAULTED"
	HealthOffline  PoolHealth = "OFFLINE"
)

// ExpansionConfig RAIDZ vdev 扩展配置.
type ExpansionConfig struct {
	PoolName   string     `json:"poolName"`   // 目标池名
	VDevPath   string     `json:"vdevPath"`   // 目标 RAIDZ vdev 路径
	NewDisks   []string   `json:"newDisks"`   // 待添加的新盘设备路径列表
	RAIDZLevel RAIDZLevel `json:"raidzLevel"` // RAIDZ 级别
	Force      bool       `json:"force"`      // 强制执行（跳过部分检查）
	DryRun     bool       `json:"dryRun"`     // 仅模拟，不实际执行
	AutoResize bool       `json:"autoResize"` // 完成后自动调整池大小
}

// ExpansionStatus 扩展进度状态.
type ExpansionStatus struct {
	PoolName       string         `json:"poolName"`
	VDevPath       string         `json:"vdevPath"`
	Phase          ExpansionPhase `json:"phase"`
	TotalDisks     int            `json:"totalDisks"`  // 计划添加的盘数
	AddedDisks     int            `json:"addedDisks"`  // 已添加的盘数
	CurrentDisk    string         `json:"currentDisk"` // 当前正在处理的盘
	PercentDone    float64        `json:"percentDone"` // 总进度百分比
	DiskPercent    float64        `json:"diskPercent"` // 当前盘进度百分比
	StartTime      time.Time      `json:"startTime"`
	EstimatedEnd   time.Time      `json:"estimatedEnd"`
	Elapsed        time.Duration  `json:"elapsed"`
	Errors         []string       `json:"errors,omitempty"`
	CompletedDisks []string       `json:"completedDisks"`
}

// ExpansionManager 管理 RAIDZ vdev 扩展生命周期.
type ExpansionManager struct {
	mu         sync.Mutex
	status     *ExpansionStatus
	config     *ExpansionConfig
	health     *HealthChecker
	cancelFunc context.CancelFunc
}

// HealthChecker 扩展前池健康验证.
type HealthChecker struct {
	// 可以注入 PoolStatusProvider 用于实际查询池状态
	provider PoolStatusProvider
}

// PoolStatusProvider 池状态查询接口.
type PoolStatusProvider interface {
	GetPoolHealth(ctx context.Context, poolName string) (PoolHealth, error)
	GetPoolCapacity(ctx context.Context, poolName string) (totalBytes, usedBytes, freeBytes uint64, err error)
	GetVDevDisks(ctx context.Context, poolName, vdevPath string) ([]string, error)
	IsDiskAvailable(ctx context.Context, devicePath string) (bool, error)
}

// NewHealthChecker 创建健康检查器.
func NewHealthChecker(provider PoolStatusProvider) *HealthChecker {
	return &HealthChecker{provider: provider}
}

// HealthCheckResult 健康检查结果.
type HealthCheckResult struct {
	PoolHealthy      bool     `json:"poolHealthy"`
	CapacityOK       bool     `json:"capacityOk"`
	DisksAvailable   bool     `json:"disksAvailable"`
	VDevExists       bool     `json:"vdevExists"`
	Issues           []string `json:"issues,omitempty"`
	CurrentDiskCount int      `json:"currentDiskCount"`
	CurrentFreeBytes uint64   `json:"currentFreeBytes"`
}

// Check 执行扩展前健康检查.
func (hc *HealthChecker) Check(ctx context.Context, cfg *ExpansionConfig) (*HealthCheckResult, error) {
	result := &HealthCheckResult{Issues: []string{}}

	// 1. 检查池健康状态
	health, err := hc.provider.GetPoolHealth(ctx, cfg.PoolName)
	if err != nil {
		return nil, fmt.Errorf("查询池健康状态失败: %w", err)
	}
	if health != HealthOnline {
		result.PoolHealthy = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("池 %s 状态为 %s，需要 ONLINE 才能扩展", cfg.PoolName, health))
	} else {
		result.PoolHealthy = true
	}

	// 2. 检查池容量
	_, usedBytes, freeBytes, err := hc.provider.GetPoolCapacity(ctx, cfg.PoolName)
	if err != nil {
		return nil, fmt.Errorf("查询池容量失败: %w", err)
	}
	result.CurrentFreeBytes = freeBytes
	// 扩展期间需要足够的空闲空间来重分布数据
	// 至少需要当前已用空间的 5% 作为余量
	minFreeRequired := usedBytes / 20
	if freeBytes < minFreeRequired {
		result.CapacityOK = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("空闲空间不足: 当前 %d bytes, 需要至少 %d bytes", freeBytes, minFreeRequired))
	} else {
		result.CapacityOK = true
	}

	// 3. 检查 vdev 是否存在
	existingDisks, err := hc.provider.GetVDevDisks(ctx, cfg.PoolName, cfg.VDevPath)
	if err != nil {
		return nil, fmt.Errorf("查询 vdev 磁盘失败: %w", err)
	}
	if len(existingDisks) == 0 {
		result.VDevExists = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("vdev %s 在池 %s 中未找到", cfg.VDevPath, cfg.PoolName))
	} else {
		result.VDevExists = true
		result.CurrentDiskCount = len(existingDisks)
	}

	// 4. 检查新盘是否可用
	allAvailable := true
	for _, disk := range cfg.NewDisks {
		available, err := hc.provider.IsDiskAvailable(ctx, disk)
		if err != nil {
			result.Issues = append(result.Issues,
				fmt.Sprintf("检查磁盘 %s 可用性失败: %v", disk, err))
			allAvailable = false
			continue
		}
		if !available {
			result.Issues = append(result.Issues,
				fmt.Sprintf("磁盘 %s 不可用或已被使用", disk))
			allAvailable = false
		}
	}
	result.DisksAvailable = allAvailable

	return result, nil
}

// NewExpansionManager 创建扩展管理器.
func NewExpansionManager(healthChecker *HealthChecker) *ExpansionManager {
	return &ExpansionManager{
		health: healthChecker,
		status: &ExpansionStatus{Phase: PhaseIdle},
	}
}

// StartExpansion 启动 RAIDZ vdev 扩展.
func (em *ExpansionManager) StartExpansion(ctx context.Context, cfg *ExpansionConfig) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 检查是否已有扩展在进行中
	if em.status.Phase != PhaseIdle && em.status.Phase != PhaseCompleted &&
		em.status.Phase != PhaseFailed && em.status.Phase != PhaseCancelled {
		return fmt.Errorf("扩展已在进行中, 当前阶段: %s", em.status.Phase)
	}

	// 验证配置
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 设置 DryRun 模式时只做检查
	if cfg.DryRun {
		em.config = cfg
		em.status = &ExpansionStatus{
			PoolName:       cfg.PoolName,
			VDevPath:       cfg.VDevPath,
			Phase:          PhaseValidating,
			TotalDisks:     len(cfg.NewDisks),
			StartTime:      time.Now(),
			CompletedDisks: []string{},
		}
		// 在 DryRun 中执行健康检查但不实际扩展
		if em.health != nil && em.health.provider != nil {
			_, err := em.health.Check(ctx, cfg)
			if err != nil {
				em.status.Phase = PhaseFailed
				em.status.Errors = append(em.status.Errors, err.Error())
				return err
			}
		}
		em.status.Phase = PhaseCompleted
		em.status.PercentDone = 100
		return nil
	}

	// 执行健康检查
	if em.health != nil && em.health.provider != nil {
		em.status.Phase = PhaseValidating
		hcResult, err := em.health.Check(ctx, cfg)
		if err != nil {
			em.status.Phase = PhaseFailed
			em.status.Errors = append(em.status.Errors, err.Error())
			return fmt.Errorf("健康检查失败: %w", err)
		}
		if !cfg.Force {
			if !hcResult.PoolHealthy || !hcResult.VDevExists || !hcResult.DisksAvailable {
				em.status.Phase = PhaseFailed
				em.status.Errors = hcResult.Issues
				return fmt.Errorf("扩展前检查未通过, 问题: %v", hcResult.Issues)
			}
			if !hcResult.CapacityOK {
				em.status.Phase = PhaseFailed
				em.status.Errors = hcResult.Issues
				return fmt.Errorf("容量检查未通过, 问题: %v", hcResult.Issues)
			}
		}
	}

	// 初始化扩展状态
	em.config = cfg
	em.status = &ExpansionStatus{
		PoolName:       cfg.PoolName,
		VDevPath:       cfg.VDevPath,
		Phase:          PhaseAddingDisk,
		TotalDisks:     len(cfg.NewDisks),
		AddedDisks:     0,
		PercentDone:    0,
		StartTime:      time.Now(),
		CompletedDisks: []string{},
		Errors:         []string{},
	}

	return nil
}

// AddNextDisk 添加下一块磁盘到 RAIDZ vdev
// 在实际实现中，这会调用 `zpool add <pool> <vdev> <disk>` 或类似命令
// 此处返回 DiskAddResult 供调用者执行实际的 zpool 命令.
func (em *ExpansionManager) AddNextDisk(ctx context.Context) (*DiskAddResult, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.config == nil || em.status == nil {
		return nil, fmt.Errorf("没有进行中的扩展")
	}
	if em.status.Phase != PhaseAddingDisk {
		return nil, fmt.Errorf("当前阶段不支持添加磁盘: %s", em.status.Phase)
	}
	if em.status.AddedDisks >= em.status.TotalDisks {
		return nil, fmt.Errorf("所有磁盘已添加完成")
	}

	nextDisk := em.config.NewDisks[em.status.AddedDisks]
	em.status.CurrentDisk = nextDisk
	em.status.DiskPercent = 0

	// 计算 zpool 命令参数
	// 实际命令: zpool add <pool> <vdevType> <disk>
	// 注意: RAIDZ expansion 使用 `zpool add` 将新盘加入现有 RAIDZ vdev
	result := &DiskAddResult{
		DiskPath:   nextDisk,
		PoolName:   em.config.PoolName,
		VDevPath:   em.config.VDevPath,
		RAIDZLevel: em.config.RaidZLevel(),
		DiskIndex:  em.status.AddedDisks,
		Cmd:        fmt.Sprintf("zpool add %s %s", em.config.PoolName, nextDisk),
	}

	return result, nil
}

// DiskAddResult 添加磁盘操作结果.
type DiskAddResult struct {
	DiskPath   string     `json:"diskPath"`
	PoolName   string     `json:"poolName"`
	VDevPath   string     `json:"vdevPath"`
	RAIDZLevel RAIDZLevel `json:"raidzLevel"`
	DiskIndex  int        `json:"diskIndex"`
	Cmd        string     `json:"cmd"` // 建议执行的命令
}

// MarkDiskAdded 标记当前磁盘已添加完成.
func (em *ExpansionManager) MarkDiskAdded(diskPath string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.status == nil || em.config == nil {
		return
	}

	em.status.CompletedDisks = append(em.status.CompletedDisks, diskPath)
	em.status.AddedDisks++
	em.status.DiskPercent = 100
	em.status.CurrentDisk = ""

	// 更新总进度
	em.status.PercentDone = float64(em.status.AddedDisks) / float64(em.status.TotalDisks) * 100

	// 更新预估完成时间
	if em.status.AddedDisks > 0 && em.status.AddedDisks < em.status.TotalDisks {
		elapsed := time.Since(em.status.StartTime)
		perDisk := elapsed / time.Duration(em.status.AddedDisks)
		remaining := time.Duration(em.status.TotalDisks-em.status.AddedDisks) * perDisk
		em.status.EstimatedEnd = time.Now().Add(remaining)
		em.status.Elapsed = elapsed
	}

	// 检查是否全部完成
	if em.status.AddedDisks >= em.status.TotalDisks {
		em.status.Phase = PhaseCompleted
		em.status.PercentDone = 100
		em.status.Elapsed = time.Since(em.status.StartTime)
	} else {
		// 准备添加下一块盘
		em.status.Phase = PhaseAddingDisk
	}
}

// UpdateDiskProgress 更新当前盘的扩展进度（resilver 进度）.
func (em *ExpansionManager) UpdateDiskProgress(percent float64) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.status == nil || em.status.Phase != PhaseAddingDisk {
		return
	}

	em.status.DiskPercent = percent
	// 总进度 = 已完成盘数/总盘数 + 当前盘进度/总盘数
	if em.status.TotalDisks > 0 {
		base := float64(em.status.AddedDisks) / float64(em.status.TotalDisks) * 100
		currentContribution := percent / float64(em.status.TotalDisks)
		em.status.PercentDone = base + currentContribution
	}
}

// GetExpansionStatus 获取扩展状态.
func (em *ExpansionManager) GetExpansionStatus() *ExpansionStatus {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.status == nil {
		return &ExpansionStatus{Phase: PhaseIdle}
	}
	// 返回副本
	status := *em.status
	status.CompletedDisks = make([]string, len(em.status.CompletedDisks))
	copy(status.CompletedDisks, em.status.CompletedDisks)
	status.Errors = make([]string, len(em.status.Errors))
	copy(status.Errors, em.status.Errors)
	return &status
}

// CompleteExpansion 完成扩展，执行收尾工作.
func (em *ExpansionManager) CompleteExpansion(ctx context.Context) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.status == nil {
		return fmt.Errorf("没有扩展操作")
	}
	if em.status.Phase == PhaseCompleted {
		return nil // 已经完成
	}
	if em.status.Phase == PhaseFailed || em.status.Phase == PhaseCancelled {
		return fmt.Errorf("扩展已 %s, 无法完成", em.status.Phase)
	}

	em.status.Phase = PhaseCompleted
	em.status.PercentDone = 100
	em.status.Elapsed = time.Since(em.status.StartTime)
	if em.status.CurrentDisk != "" {
		em.status.CompletedDisks = append(em.status.CompletedDisks, em.status.CurrentDisk)
		em.status.AddedDisks++
		em.status.CurrentDisk = ""
	}
	return nil
}

// CancelExpansion 取消扩展.
func (em *ExpansionManager) CancelExpansion() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.status == nil || em.status.Phase == PhaseIdle {
		return fmt.Errorf("没有进行中的扩展")
	}
	if em.status.Phase == PhaseCompleted {
		return fmt.Errorf("扩展已完成, 无法取消")
	}

	em.status.Phase = PhaseCancelled
	return nil
}

// FailExpansion 标记扩展失败.
func (em *ExpansionManager) FailExpansion(reason string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.status == nil {
		return
	}
	em.status.Phase = PhaseFailed
	em.status.Errors = append(em.status.Errors, reason)
}

// IsExpansionInProgress 检查扩展是否进行中.
func (em *ExpansionManager) IsExpansionInProgress() bool {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.status == nil {
		return false
	}
	return em.status.Phase == PhaseAddingDisk ||
		em.status.Phase == PhaseValidating ||
		em.status.Phase == PhaseResilvering
}

// RaidZLevel 返回当前扩展的 RAIDZ 级别.
func (c *ExpansionConfig) RaidZLevel() RAIDZLevel {
	return c.RAIDZLevel
}

// validateConfig 验证扩展配置.
func validateConfig(cfg *ExpansionConfig) error {
	if cfg.PoolName == "" {
		return fmt.Errorf("池名不能为空")
	}
	if cfg.VDevPath == "" {
		return fmt.Errorf("vdev 路径不能为空")
	}
	if len(cfg.NewDisks) == 0 {
		return fmt.Errorf("新盘列表不能为空")
	}
	// 检查磁盘路径唯一性
	seen := make(map[string]bool)
	for _, disk := range cfg.NewDisks {
		if disk == "" {
			return fmt.Errorf("磁盘路径不能为空")
		}
		if seen[disk] {
			return fmt.Errorf("磁盘路径重复: %s", disk)
		}
		seen[disk] = true
	}
	// 验证 RAIDZ 级别
	switch cfg.RAIDZLevel {
	case RAIDZ1, RAIDZ2, RAIDZ3:
		// valid
	default:
		return fmt.Errorf("不支持的 RAIDZ 级别: %s", cfg.RAIDZLevel)
	}
	return nil
}

// EstimateCapacityGain 估算扩展后的容量增加
// RAIDZ 容量计算: (N - P) * minDiskSize, 其中 N 是总盘数, P 是校验盘数.
func EstimateCapacityGain(currentDisks int, newDisks int, level RAIDZLevel, diskSize uint64) uint64 {
	parityDisks := 0
	switch level {
	case RAIDZ1:
		parityDisks = 1
	case RAIDZ2:
		parityDisks = 2
	case RAIDZ3:
		parityDisks = 3
	}
	// 扩展后总数据盘数 = (currentDisks + newDisks) - parityDisks
	// 但 RAIDZ expansion 逐盘添加时，每加一个盘增加一个数据盘的容量
	// 因为校验盘数量不变
	totalDataDisks := (currentDisks + newDisks) - parityDisks
	currentDataDisks := currentDisks - parityDisks
	additionalDataDisks := totalDataDisks - currentDataDisks
	// 每加一个盘就是加一个数据盘的容量
	return uint64(additionalDataDisks) * diskSize
}

// MockPoolStatusProvider 内存模拟池状态提供者（用于测试）.
type MockPoolStatusProvider struct {
	Health         PoolHealth
	TotalBytes     uint64
	UsedBytes      uint64
	FreeBytes      uint64
	VDevDisks      []string
	AvailableDisks map[string]bool
	HealthErr      error
	CapacityErr    error
	VDevErr        error
	DiskErr        error
}

func (m *MockPoolStatusProvider) GetPoolHealth(ctx context.Context, poolName string) (PoolHealth, error) {
	if m.HealthErr != nil {
		return "", m.HealthErr
	}
	return m.Health, nil
}

func (m *MockPoolStatusProvider) GetPoolCapacity(ctx context.Context, poolName string) (uint64, uint64, uint64, error) {
	if m.CapacityErr != nil {
		return 0, 0, 0, m.CapacityErr
	}
	return m.TotalBytes, m.UsedBytes, m.FreeBytes, nil
}

func (m *MockPoolStatusProvider) GetVDevDisks(ctx context.Context, poolName, vdevPath string) ([]string, error) {
	if m.VDevErr != nil {
		return nil, m.VDevErr
	}
	return m.VDevDisks, nil
}

func (m *MockPoolStatusProvider) IsDiskAvailable(ctx context.Context, devicePath string) (bool, error) {
	if m.DiskErr != nil {
		return false, m.DiskErr
	}
	return m.AvailableDisks[devicePath], nil
}
