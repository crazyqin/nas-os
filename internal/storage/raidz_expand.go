// Package storage RAIDZ 扩展进度监控 API
// 兵部 Round 153 - RAIDZ Expansion 进度监控与异步任务模式
// 对标 TrueNAS 24.10 Electric Eel RAIDZ Expansion UI 特性

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 进度监控核心结构 ==========

// RAIDZExpandProgress 扩展进度详情（UI展示）
type RAIDZExpandProgress struct {
	// 任务标识
	TaskID    string    `json:"taskId"`    // 任务唯一ID
	PoolName  string    `json:"poolName"`  // 池名称
	VdevName  string    `json:"vdevName"`  // VDEV名称 (如 raidz2-0)

	// 进度信息
	Percent      float64 `json:"percent"`      // 完成百分比 (0-100)
	BytesDone    uint64  `json:"bytesDone"`    // 已处理字节
	BytesTotal   uint64  `json:"bytesTotal"`   // 总字节
	SpeedMBps    float64 `json:"speedMBps"`    // 当前速度 MB/s
	ETASeconds   int64   `json:"etaSeconds"`   // 预估剩余秒数
	ETAFormatted string  `json:"etaFormatted"` // 格式化ETA (如 "2h 15m")

	// 时间信息
	StartTime    time.Time  `json:"startTime"`    // 开始时间
	Elapsed      int64      `json:"elapsed"`      // 已耗时秒数
	ElapsedFormatted string `json:"elapsedFormatted"` // 格式化耗时
	LastUpdate   time.Time  `json:"lastUpdate"`   // 最后更新时间

	// 状态信息
	Status       string `json:"status"`       // running/paused/completed/failed
	StatusText   string `json:"statusText"`   // 状态中文描述
	CanPause     bool   `json:"canPause"`     // 是否可暂停
	CanResume    bool   `json:"canResume"`    // 是否可恢复
	CanCancel    bool   `json:"canCancel"`    // 是否可取消

	// 阶段信息（扩展过程的细分阶段）
	Phase         string  `json:"phase"`         // 当前阶段
	PhasePercent  float64 `json:"phasePercent"`  // 阶段内进度
	Phases        []PhaseInfo `json:"phases"`     // 所有阶段信息

	// 磁盘信息
	OriginalDisks []DiskSlot `json:"originalDisks"` // 原始磁盘
	NewDisk       DiskSlot   `json:"newDisk"`       // 新增磁盘
	WidthBefore   int        `json:"widthBefore"`   // 扩展前宽度
	WidthAfter    int        `json:"widthAfter"`    // 扩展后宽度

	// 容量信息
	CapacityBeforeGB  float64 `json:"capacityBeforeGB"`  // 扩展前容量GB
	CapacityAfterGB   float64 `json:"capacityAfterGB"`   // 扩展后预估容量GB
	CapacityGainGB    float64 `json:"capacityGainGB"`    // 容量增益GB

	// 错误与警告
	Errors   []string `json:"errors"`   // 错误列表
	Warnings []string `json:"warnings"` // 警告列表
}

// PhaseInfo 阶段信息
type PhaseInfo struct {
	Name        string  `json:"name"`        // 阶段名称
	Description string  `json:"description"` // 阶段描述
	Percent     float64 `json:"percent"`     // 阶段进度
	Status      string  `json:"status"`      // pending/running/completed
	Duration    int64   `json:"duration"`    // 阶段耗时秒数
}

// DiskSlot 磁盘槽位信息（UI展示）
type DiskSlot struct {
	Path      string `json:"path"`      // 磁盘路径
	Model     string `json:"model"`     // 型号
	SizeGB    int    `json:"sizeGB"`    // 容量GB
	State     string `json:"state"`     // 状态: original/expanding/new
	IsNew     bool   `json:"isNew"`     // 是否新盘
	Indicator string `json:"indicator"` // UI指示器颜色
}

// ========== 扩展阶段定义 ==========

// ExpansionPhase 扩展阶段常量
const (
	PhasePreparing      = "preparing"      // 准备阶段
	PhaseDataScan       = "data_scan"      // 数据扫描
	PhaseDataMigration  = "data_migration" // 数据迁移（主要阶段）
	PhaseVerification   = "verification"   // 数据校验
	PhaseFinalization   = "finalization"   // 最终化
	PhaseCompleted      = "completed"      // 完成
)

// DefaultPhases 默认阶段列表
var DefaultPhases = []PhaseInfo{
	{Name: PhasePreparing, Description: "准备扩展环境", Status: "pending"},
	{Name: PhaseDataScan, Description: "扫描数据布局", Status: "pending"},
	{Name: PhaseDataMigration, Description: "重分布数据块", Status: "pending"},
	{Name: PhaseVerification, Description: "校验数据完整性", Status: "pending"},
	{Name: PhaseFinalization, Description: "更新元数据", Status: "pending"},
	{Name: PhaseCompleted, Description: "扩展完成", Status: "pending"},
}

// ========== 扩展进度监控器 ==========

// RAIDZExpandMonitor 扩展进度监控器
type RAIDZExpandMonitor struct {
	mu sync.RWMutex

	// 当前活跃进度
	activeProgress map[string]*RAIDZExpandProgress

	// 进度历史（用于UI展示历史记录）
	progressHistory []RAIDZExpandProgress

	// 进度回调（实时推送）
	progressCallbacks map[string]func(*RAIDZExpandProgress)

	// 状态变更回调
	stateCallbacks []func(string, string) // poolName, status

	// 配置路径
	configPath string

	// 更新间隔
	updateInterval time.Duration

	// 停止信号
	stopChan chan struct{}
}

// NewRAIDZExpandMonitor 创建扩展进度监控器
func NewRAIDZExpandMonitor(configPath string) *RAIDZExpandMonitor {
	return &RAIDZExpandMonitor{
		activeProgress:    make(map[string]*RAIDZExpandProgress),
		progressHistory:   make([]RAIDZExpandProgress, 0, 50),
		progressCallbacks: make(map[string]func(*RAIDZExpandProgress)),
		stateCallbacks:    make([]func(string, string), 0),
		configPath:        configPath,
		updateInterval:    5 * time.Second,
		stopChan:          make(chan struct{}),
	}
}

// ========== 核心进度API ==========

// StartMonitoring 开始监控扩展进度
func (m *RAIDZExpandMonitor) StartMonitoring(ctx context.Context, task *ExpansionTask) (*RAIDZExpandProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建进度对象
	progress := &RAIDZExpandProgress{
		TaskID:           task.ID,
		PoolName:         task.PoolName,
		VdevName:         m.extractVdevName(task),
		Percent:          task.Progress,
		BytesDone:        task.BytesProcessed,
		BytesTotal:       task.TotalBytes,
		SpeedMBps:        task.SpeedMBps,
		StartTime:        task.StartTime,
		LastUpdate:       time.Now(),
		Status:           string(task.Status),
		StatusText:       m.statusToText(task.Status),
		CanPause:         task.CanPause,
		CanResume:        task.CanResume,
		CanCancel:        task.CanCancel,
		Phase:            PhaseDataMigration, // 默认主阶段
		Phases:           DefaultPhases,
		WidthBefore:      m.extractWidthBefore(task),
		WidthAfter:       m.extractWidthAfter(task),
		CapacityBeforeGB: float64(task.TotalBytes) / (1024 * 1024 * 1024),
		Errors:           task.Errors,
		Warnings:         task.Warnings,
	}

	// 计算ETA
	m.calculateETA(progress)

	// 格式化时间显示
	progress.ElapsedFormatted = formatDuration(progress.Elapsed)
	progress.ETAFormatted = formatDuration(progress.ETASeconds)

	// 存储活跃进度
	m.activeProgress[task.PoolName] = progress

	// 启动后台监控（如果状态为running）
	if task.Status == StatusRunning {
		go m.monitorLoop(ctx, task.PoolName)
	}

	return progress, nil
}

// GetProgress 获取当前扩展进度
func (m *RAIDZExpandMonitor) GetProgress(poolName string) (*RAIDZExpandProgress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	progress, exists := m.activeProgress[poolName]
	if !exists {
		// 返回空闲状态
		return &RAIDZExpandProgress{
			PoolName:   poolName,
			Status:     "idle",
			StatusText: "无扩展任务",
			Phases:     DefaultPhases,
		}, nil
	}

	// 返回副本
	copy := *progress
	copy.Phases = make([]PhaseInfo, len(progress.Phases))
	for i, p := range progress.Phases {
		copy.Phases[i] = p
	}

	return &copy, nil
}

// GetAllProgress 获取所有活跃扩展进度
func (m *RAIDZExpandMonitor) GetAllProgress() []*RAIDZExpandProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RAIDZExpandProgress, 0, len(m.activeProgress))
	for _, progress := range m.activeProgress {
		copy := *progress
		copy.Phases = make([]PhaseInfo, len(progress.Phases))
		for i, p := range progress.Phases {
			copy.Phases[i] = p
		}
		result = append(result, &copy)
	}

	return result
}

// UpdateProgress 更新扩展进度（由服务层调用）
func (m *RAIDZExpandMonitor) UpdateProgress(poolName string, task *ExpansionTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	progress, exists := m.activeProgress[poolName]
	if !exists {
		// 如果不存在，创建新的
		progress = &RAIDZExpandProgress{
			TaskID:    task.ID,
			PoolName:  poolName,
			Phases:    DefaultPhases,
		}
		m.activeProgress[poolName] = progress
	}

	// 更新进度字段
	progress.Percent = task.Progress
	progress.BytesDone = task.BytesProcessed
	progress.BytesTotal = task.TotalBytes
	progress.SpeedMBps = task.SpeedMBps
	progress.Status = string(task.Status)
	progress.StatusText = m.statusToText(task.Status)
	progress.CanPause = task.CanPause
	progress.CanResume = task.CanResume
	progress.CanCancel = task.CanCancel
	progress.LastUpdate = time.Now()
	progress.Elapsed = int64(time.Since(progress.StartTime).Seconds())
	progress.Errors = task.Errors
	progress.Warnings = task.Warnings

	// 计算ETA
	m.calculateETA(progress)

	// 格式化显示
	progress.ElapsedFormatted = formatDuration(progress.Elapsed)
	progress.ETAFormatted = formatDuration(progress.ETASeconds)

	// 更新阶段进度
	m.updatePhaseProgress(progress)

	// 触发回调
	if callback, ok := m.progressCallbacks[poolName]; ok {
		go callback(progress)
	}

	// 状态变更回调
	for _, cb := range m.stateCallbacks {
		go cb(poolName, string(task.Status))
	}

	// 如果完成，添加到历史
	if task.Status == StatusCompleted || task.Status == StatusFailed || task.Status == StatusCancelled {
		m.addToHistory(*progress)
		delete(m.activeProgress, poolName)
	}

	return nil
}

// ========== 进度回调API ==========

// RegisterProgressCallback 注册进度回调（用于WebSocket推送）
func (m *RAIDZExpandMonitor) RegisterProgressCallback(poolName string, callback func(*RAIDZExpandProgress)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressCallbacks[poolName] = callback
}

// RegisterStateCallback 注册状态变更回调
func (m *RAIDZExpandMonitor) RegisterStateCallback(callback func(string, string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateCallbacks = append(m.stateCallbacks, callback)
}

// UnregisterProgressCallback 移除进度回调
func (m *RAIDZExpandMonitor) UnregisterProgressCallback(poolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.progressCallbacks, poolName)
}

// ========== 历史记录API ==========

// GetProgressHistory 获取进度历史（UI展示）
func (m *RAIDZExpandMonitor) GetProgressHistory(limit int) []RAIDZExpandProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.progressHistory) {
		limit = len(m.progressHistory)
	}

	start := len(m.progressHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]RAIDZExpandProgress, limit)
	copy(result, m.progressHistory[start:])
	return result
}

// GetLatestProgressForPool 获取指定池的最新历史进度
func (m *RAIDZExpandMonitor) GetLatestProgressForPool(poolName string) *RAIDZExpandProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先检查活跃
	if progress, ok := m.activeProgress[poolName]; ok {
		copy := *progress
		return &copy
	}

	// 查找历史（倒序）
	for i := len(m.progressHistory) - 1; i >= 0; i-- {
		if m.progressHistory[i].PoolName == poolName {
			copy := m.progressHistory[i]
			return &copy
		}
	}

	return nil
}

// ========== 内部方法 ==========

// monitorLoop 监控循环（定期更新）
func (m *RAIDZExpandMonitor) monitorLoop(ctx context.Context, poolName string) {
	ticker := time.NewTicker(m.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查是否仍在活跃
			m.mu.RLock()
			progress, exists := m.activeProgress[poolName]
			active := exists && progress.Status == "running"
			m.mu.RUnlock()

			if !active {
				return // 监控结束
			}

			// 触发回调（如果有注册）
			if callback, ok := m.progressCallbacks[poolName]; ok {
				m.mu.RLock()
				copy := *progress
				m.mu.RUnlock()
				go callback(&copy)
			}

		case <-m.stopChan:
			return

		case <-ctx.Done():
			return
		}
	}
}

// calculateETA 计算预估剩余时间
func (m *RAIDZExpandMonitor) calculateETA(progress *RAIDZExpandProgress) {
	if progress.Percent <= 0 || progress.Percent >= 100 {
		progress.ETASeconds = 0
		return
	}

	// 基于当前速度估算
	if progress.SpeedMBps > 0 {
		bytesRemaining := progress.BytesTotal - progress.BytesDone
		secondsRemaining := float64(bytesRemaining) / (progress.SpeedMBps * 1024 * 1024)
		progress.ETASeconds = int64(secondsRemaining)
	} else {
		// 基于平均速度估算（用已用时间推算）
		if progress.Elapsed > 0 && progress.Percent > 0 {
			avgSpeedPercent := progress.Percent / float64(progress.Elapsed)
			remainingPercent := 100 - progress.Percent
			progress.ETASeconds = int64(remainingPercent / avgSpeedPercent)
		}
	}
}

// updatePhaseProgress 更新阶段进度
func (m *RAIDZExpandMonitor) updatePhaseProgress(progress *RAIDZExpandProgress) {
	// 根据总进度分配阶段进度
	// 假设阶段权重: preparing(5%), scan(10%), migration(70%), verify(10%), finalize(5%)
	phaseWeights := []float64{5, 10, 70, 10, 5}

	percent := progress.Percent
	phaseIndex := 0
	accumulatedWeight := 0.0

	for i, weight := range phaseWeights {
		accumulatedWeight += weight
		if percent <= accumulatedWeight {
			phaseIndex = i
			break
		}
		phaseIndex = i + 1
	}

	// 更新阶段状态
	for i := range progress.Phases {
		if i < phaseIndex {
			progress.Phases[i].Status = "completed"
			progress.Phases[i].Percent = 100
		} else if i == phaseIndex {
			progress.Phases[i].Status = "running"
			// 计算阶段内进度
			prevWeight := 0.0
			for j := 0; j < i; j++ {
				prevWeight += phaseWeights[j]
			}
			phasePercent := (percent - prevWeight) / phaseWeights[i] * 100
			if phasePercent > 100 {
				phasePercent = 100
			}
			progress.Phases[i].Percent = phasePercent
			progress.Phase = progress.Phases[i].Name
			progress.PhasePercent = phasePercent
		} else {
			progress.Phases[i].Status = "pending"
			progress.Phases[i].Percent = 0
		}
	}

	// 如果总进度100%，标记所有阶段完成
	if progress.Percent >= 100 {
		progress.Phase = PhaseCompleted
		for i := range progress.Phases {
			progress.Phases[i].Status = "completed"
			progress.Phases[i].Percent = 100
		}
	}
}

// addToHistory 添加到历史
func (m *RAIDZExpandMonitor) addToHistory(progress RAIDZExpandProgress) {
	m.progressHistory = append(m.progressHistory, progress)

	// 保留最近50条
	if len(m.progressHistory) > 50 {
		m.progressHistory = m.progressHistory[len(m.progressHistory)-50:]
	}

	// 保存到文件
	if m.configPath != "" {
		_ = m.saveHistory()
	}
}

// saveHistory 保存历史到文件
func (m *RAIDZExpandMonitor) saveHistory() error {
	if m.configPath == "" {
		return nil
	}

	historyPath := filepath.Join(filepath.Dir(m.configPath), "raidz_expand_history.json")
	data, err := json.MarshalIndent(m.progressHistory, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, data, 0640)
}

// loadHistory 加载历史
func (m *RAIDZExpandMonitor) loadHistory() error {
	if m.configPath == "" {
		return nil
	}

	historyPath := filepath.Join(filepath.Dir(m.configPath), "raidz_expand_history.json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &m.progressHistory)
}

// ========== 辅助方法 ==========

// statusToText 状态转中文描述
func (m *RAIDZExpandMonitor) statusToText(status ExpansionStatus) string {
	switch status {
	case StatusIdle:
		return "空闲"
	case StatusPreparing:
		return "准备中"
	case StatusRunning:
		return "扩展中"
	case StatusPaused:
		return "已暂停"
	case StatusCompleted:
		return "已完成"
	case StatusFailed:
		return "失败"
	case StatusCancelled:
		return "已取消"
	default:
		return "未知"
	}
}

// extractVdevName 提取VDEV名称
func (m *RAIDZExpandMonitor) extractVdevName(task *ExpansionTask) string {
	// 从metadata或RAIDZ级别推断
	if task.Metadata != nil {
		if vdev, ok := task.Metadata["vdev_name"]; ok {
			return vdev
		}
	}

	// 默认命名
	if task.RAIDZLevel != "" {
		return task.RAIDZLevel + "-0"
	}
	return "raidz-0"
}

// extractWidthBefore 提取扩展前宽度
func (m *RAIDZExpandMonitor) extractWidthBefore(task *ExpansionTask) int {
	if task.Metadata != nil {
		if width, ok := task.Metadata["width_before"]; ok {
			if w, err := fmt.Sscanf(width, "%d"); err == nil {
				return w
			}
		}
	}

	// 从RAIDZ级别推断（默认）
	switch task.RAIDZLevel {
	case "raidz1":
		return 3 // 最小3盘
	case "raidz2":
		return 4 // 最小4盘
	case "raidz3":
		return 5 // 最小5盘
	default:
		return 3
	}
}

// extractWidthAfter 提取扩展后宽度
func (m *RAIDZExpandMonitor) extractWidthAfter(task *ExpansionTask) int {
	return m.extractWidthBefore(task) + 1
}

// formatDuration 格式化时长
func formatDuration(seconds int64) string {
	if seconds <= 0 {
		return "-"
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}

	if minutes > 0 {
		if secs > 0 {
			return fmt.Sprintf("%dm %ds", minutes, secs)
		}
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", secs)
}

// Stop 停止监控
func (m *RAIDZExpandMonitor) Stop() {
	close(m.stopChan)
}

// ========== 磁盘槽位构建 ==========

// BuildDiskSlots 构建磁盘槽位信息（UI展示）
func BuildDiskSlots(poolName string, originalDisks []string, newDisk string) []DiskSlot {
	var slots []DiskSlot

	// 原始磁盘
	for _, path := range originalDisks {
		slot := DiskSlot{
			Path:      path,
			State:     "original",
			IsNew:     false,
			Indicator: "green", // 正常状态
		}

		// 尝试获取磁盘信息
		if info, err := getDiskBasicInfo(path); err == nil {
			slot.Model = info.Model
			slot.SizeGB = info.SizeGB
		}

		slots = append(slots, slot)
	}

	// 新磁盘
	if newDisk != "" {
		slot := DiskSlot{
			Path:      newDisk,
			State:     "expanding",
			IsNew:     true,
			Indicator: "blue", // 新盘状态
		}

		if info, err := getDiskBasicInfo(newDisk); err == nil {
			slot.Model = info.Model
			slot.SizeGB = info.SizeGB
		}

		slots = append(slots, slot)
	}

	return slots
}

// DiskBasicInfo 磁盘基本信息
type DiskBasicInfo struct {
	Model  string
	SizeGB int
}

// getDiskBasicInfo 获取磁盘基本信息
func getDiskBasicInfo(path string) (*DiskBasicInfo, error) {
	// 使用系统命令获取（简化实现）
	// 实际实现应调用 NVMeDetector 或 SMARTMonitor
	return &DiskBasicInfo{
		Model:  "Unknown",
		SizeGB: 0,
	}, nil
}

// ========== UI展示专用API ==========

// GetExpandSummary 获取扩展摘要（用于Dashboard卡片）
func (m *RAIDZExpandMonitor) GetExpandSummary() *ExpandSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &ExpandSummary{
		ActiveCount:  len(m.activeProgress),
		TotalHistory: len(m.progressHistory),
		ActiveTasks:  make([]ExpandTaskSummary, 0),
	}

	for _, progress := range m.activeProgress {
		taskSummary := ExpandTaskSummary{
			PoolName:    progress.PoolName,
			Percent:     progress.Percent,
			Status:      progress.Status,
			StatusText:  progress.StatusText,
			ETAFormatted: progress.ETAFormatted,
		}
		summary.ActiveTasks = append(summary.ActiveTasks, taskSummary)
	}

	return summary
}

// ExpandSummary 扩展摘要（Dashboard展示）
type ExpandSummary struct {
	ActiveCount  int                `json:"activeCount"`  // 活跃任务数
	TotalHistory int                `json:"totalHistory"` // 历史总数
	ActiveTasks  []ExpandTaskSummary `json:"activeTasks"` // 活跃任务摘要
}

// ExpandTaskSummary 任务摘要
type ExpandTaskSummary struct {
	PoolName     string  `json:"poolName"`
	Percent      float64 `json:"percent"`
	Status       string  `json:"status"`
	StatusText   string  `json:"statusText"`
	ETAFormatted string  `json:"etaFormatted"`
}

// ========== 容量计算辅助 ==========

// CalculateCapacityGain 计算容量增益（UI展示）
func CalculateCapacityGain(raidzLevel string, widthBefore int, diskSizeGB float64) float64 {
	// RAIDZ容量公式: 有效容量 = (N-P) * DiskSize
	// N=盘数, P=奇偶校验盘数
	var parity int
	switch raidzLevel {
	case "raidz1":
		parity = 1
	case "raidz2":
		parity = 2
	case "raidz3":
		parity = 3
	default:
		parity = 1
	}

	// 扩展前数据盘数
	dataDisksBefore := widthBefore - parity

	// 容量增益 = 新增数据盘容量（扩展后增加1个数据盘）
	_ = dataDisksBefore + 1 // dataDisksAfter

	return diskSizeGB
}

// CalculateEfficiency 计算存储效率
func CalculateEfficiency(raidzLevel string, width int) float64 {
	var parity int
	switch raidzLevel {
	case "raidz1":
		parity = 1
	case "raidz2":
		parity = 2
	case "raidz3":
		parity = 3
	default:
		parity = 1
	}

	if width <= parity {
		return 0
	}

	return float64(width - parity) / float64(width) * 100
}