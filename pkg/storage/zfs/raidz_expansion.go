// Package zfs 实现 RAIDZ 单盘扩展功能
// 参考 TrueNAS Electric Eel 实现，支持在线扩展 RAIDZ 阵列
package zfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========== 核心错误定义 ==========

var (
	// ErrExpansionInProgress indicates an expansion is already in progress
	ErrExpansionInProgress = errors.New("RAIDZ expansion already in progress")
	// ErrNoExpansionInProgress indicates no expansion is in progress
	ErrNoExpansionInProgress = errors.New("no RAIDZ expansion in progress")
	// ErrInvalidRAIDZLevel indicates invalid RAIDZ level
	ErrInvalidRAIDZLevel = errors.New("invalid RAIDZ level, must be 1, 2, or 3")
	// ErrDiskNotFound indicates disk not found
	ErrDiskNotFound = errors.New("disk not found")
	// ErrDiskInUse indicates disk is already in use
	ErrDiskInUse = errors.New("disk is already in use")
	// ErrPoolNotRAIDZ indicates pool is not a RAIDZ vdev
	ErrPoolNotRAIDZ = errors.New("pool does not contain a RAIDZ vdev")
	// ErrExpansionNotSupported indicates expansion not supported
	ErrExpansionNotSupported = errors.New("RAIDZ expansion not supported on this pool")
	// ErrExpansionFailed indicates expansion failed
	ErrExpansionFailed = errors.New("RAIDZ expansion failed")
	// ErrExpansionCancelled indicates expansion was cancelled
	ErrExpansionCancelled = errors.New("RAIDZ expansion was cancelled")
	// ErrExpansionPaused indicates expansion is paused
	ErrExpansionPaused = errors.New("RAIDZ expansion is paused")
)

// ========== RAIDZ 扩展相关类型定义 ==========

// RAIDZLevel RAIDZ 级别
type RAIDZLevel int

const (
	// RAIDZ1 单奇偶校验
	RAIDZ1 RAIDZLevel = 1
	// RAIDZ2 双奇偶校验
	RAIDZ2 RAIDZLevel = 2
	// RAIDZ3 三奇偶校验
	RAIDZ3 RAIDZLevel = 3
)

// String 返回 RAIDZ 级别的字符串表示
func (l RAIDZLevel) String() string {
	switch l {
	case RAIDZ1:
		return "raidz1"
	case RAIDZ2:
		return "raidz2"
	case RAIDZ3:
		return "raidz3"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

// ParseRAIDZLevel 从字符串解析 RAIDZ 级别
func ParseRAIDZLevel(s string) (RAIDZLevel, error) {
	switch strings.ToLower(s) {
	case "raidz", "raidz1":
		return RAIDZ1, nil
	case "raidz2":
		return RAIDZ2, nil
	case "raidz3":
		return RAIDZ3, nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrInvalidRAIDZLevel, s)
	}
}

// ExpansionState 扩展状态
type ExpansionState string

const (
	// ExpansionStateIdle 空闲，无扩展进行
	ExpansionStateIdle ExpansionState = "idle"
	// ExpansionStatePreparing 准备中
	ExpansionStatePreparing ExpansionState = "preparing"
	// ExpansionStateRunning 扩展进行中
	ExpansionStateRunning ExpansionState = "running"
	// ExpansionStatePaused 已暂停
	ExpansionStatePaused ExpansionState = "paused"
	// ExpansionStateCompleted 已完成
	ExpansionStateCompleted ExpansionState = "completed"
	// ExpansionStateFailed 失败
	ExpansionStateFailed ExpansionState = "failed"
	// ExpansionStateCancelled 已取消
	ExpansionStateCancelled ExpansionState = "cancelled"
)

// ExpansionConfig 扩展配置
type ExpansionConfig struct {
	// PoolName 池名称
	PoolName string `json:"poolName"`

	// NewDisk 新增磁盘路径
	NewDisk string `json:"newDisk"`

	// RAIDZLevel RAIDZ 级别（用于验证）
	RAIDZLevel RAIDZLevel `json:"raidzLevel"`

	// Force 强制执行
	Force bool `json:"force"`

	// DryRun 仅模拟运行
	DryRun bool `json:"dryRun"`

	// AutoStart 自动开始扩展
	AutoStart bool `json:"autoStart"`

	// ProgressCallback 进度回调函数（可选）
	ProgressCallback func(progress float64) `json:"-"`
}

// ExpansionStatus 扩展状态
type ExpansionStatus struct {
	// ID 扩展任务 ID
	ID string `json:"id"`

	// PoolName 池名称
	PoolName string `json:"poolName"`

	// State 当前状态
	State ExpansionState `json:"state"`

	// NewDisk 新增磁盘
	NewDisk string `json:"newDisk"`

	// RAIDZLevel RAIDZ 级别
	RAIDZLevel RAIDZLevel `json:"raidzLevel"`

	// OriginalWidth 原始宽度（磁盘数）
	OriginalWidth int `json:"originalWidth"`

	// NewWidth 新宽度（扩展后磁盘数）
	NewWidth int `json:"newWidth"`

	// Progress 进度百分比 (0-100)
	Progress float64 `json:"progress"`

	// BytesProcessed 已处理字节数
	BytesProcessed uint64 `json:"bytesProcessed"`

	// TotalBytes 总字节数
	TotalBytes uint64 `json:"totalBytes"`

	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`

	// EndTime 结束时间
	EndTime time.Time `json:"endTime,omitempty"`

	// EstimatedTimeRemaining 预计剩余时间
	EstimatedTimeRemaining time.Duration `json:"estimatedTimeRemaining"`

	// Speed 当前速度 (MB/s)
	Speed float64 `json:"speed"`

	// Errors 错误信息
	Errors []string `json:"errors,omitempty"`

	// Warnings 警告信息
	Warnings []string `json:"warnings,omitempty"`

	// CanResume 是否可恢复
	CanResume bool `json:"canResume"`

	// CanCancel 是否可取消
	CanCancel bool `json:"canCancel"`

	// PauseCount 暂停次数
	PauseCount int `json:"pauseCount"`

	// LastUpdateTime 最后更新时间
	LastUpdateTime time.Time `json:"lastUpdateTime"`
}

// ExpansionHistory 扩展历史记录
type ExpansionHistory struct {
	// Expansions 扩展记录列表
	Expansions []ExpansionStatus `json:"expansions"`

	// LastUpdated 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`
}

// VdevExpansionInfo VDEV 扩展信息
type VdevExpansionInfo struct {
	// VdevType VDEV 类型
	VdevType string `json:"vdevType"`

	// Width 当前宽度
	Width int `json:"width"`

	// ParityDisks 奇偶校验盘数
	ParityDisks int `json:"parityDisks"`

	// DataDisks 数据盘数
	DataDisks int `json:"dataDisks"`

	// CanExpand 是否可扩展
	CanExpand bool `json:"canExpand"`

	// ExpansionSupported 扩展是否受支持
	ExpansionSupported bool `json:"expansionSupported"`
}

// PoolExpansionInfo 池扩展信息
type PoolExpansionInfo struct {
	// PoolName 池名称
	PoolName string `json:"poolName"`

	// PoolState 池状态
	PoolState string `json:"poolState"`

	// TotalSize 总大小
	TotalSize uint64 `json:"totalSize"`

	// AllocatedSize 已分配大小
	AllocatedSize uint64 `json:"allocatedSize"`

	// Vdevs VDEV 信息
	Vdevs []VdevExpansionInfo `json:"vdevs"`

	// CanExpand 是否可扩展
	CanExpand bool `json:"canExpand"`

	// Reason 不可扩展原因
	Reason string `json:"reason,omitempty"`
}

// RAIDZExpansionManager RAIDZ 扩展管理器
type RAIDZExpansionManager struct {
	mu sync.RWMutex

	// 当前扩展状态
	currentStatus *ExpansionStatus

	// 扩展历史
	history *ExpansionHistory

	// 配置保存路径
	configPath string

	// ZFS 可用性
	available bool

	// 取消通道
	cancelChan chan struct{}

	// 暂停通道
	pauseChan chan struct{}

	// 恢复通道
	resumeChan chan struct{}

	// 状态变更回调
	onStateChange func(status *ExpansionStatus)
}

// NewRAIDZExpansionManager 创建 RAIDZ 扩展管理器
func NewRAIDZExpansionManager(configPath string) (*RAIDZExpansionManager, error) {
	m := &RAIDZExpansionManager{
		configPath: configPath,
		history: &ExpansionHistory{
			Expansions: []ExpansionStatus{},
		},
		cancelChan: make(chan struct{}, 1),
		pauseChan:  make(chan struct{}, 1),
		resumeChan: make(chan struct{}, 1),
	}

	// 检查 ZFS 可用性
	m.checkAvailable()

	// 加载历史记录
	if configPath != "" {
		_ = m.loadHistory()
	}

	return m, nil
}

// checkAvailable 检查 ZFS 可用性
func (m *RAIDZExpansionManager) checkAvailable() {
	if _, err := exec.LookPath("zpool"); err == nil {
		m.available = true
		return
	}
	m.available = false
}

// IsAvailable 检查管理器是否可用
func (m *RAIDZExpansionManager) IsAvailable() bool {
	return m.available
}

// loadHistory 加载扩展历史
func (m *RAIDZExpansionManager) loadHistory() error {
	if m.configPath == "" {
		return nil
	}

	historyPath := filepath.Join(filepath.Dir(m.configPath), "raidz_expansion_history.json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &m.history)
}

// saveHistory 保存扩展历史
func (m *RAIDZExpansionManager) saveHistory() error {
	if m.configPath == "" {
		return nil
	}

	m.history.LastUpdated = time.Now()
	historyPath := filepath.Join(filepath.Dir(m.configPath), "raidz_expansion_history.json")

	data, err := json.MarshalIndent(m.history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, data, 0640)
}

// SetStateChangeCallback 设置状态变更回调
func (m *RAIDZExpansionManager) SetStateChangeCallback(callback func(status *ExpansionStatus)) {
	m.mu.Lock()
	m.onStateChange = callback
	m.mu.Unlock()
}

// GetExpansionStatus 获取当前扩展状态
func (m *RAIDZExpansionManager) GetExpansionStatus() *ExpansionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentStatus == nil {
		return &ExpansionStatus{State: ExpansionStateIdle}
	}

	// 返回副本
	status := *m.currentStatus
	return &status
}

// GetExpansionHistory 获取扩展历史
func (m *RAIDZExpansionManager) GetExpansionHistory() []ExpansionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ExpansionStatus, len(m.history.Expansions))
	copy(result, m.history.Expansions)
	return result
}

// GetPoolExpansionInfo 获取池扩展信息
func (m *RAIDZExpansionManager) GetPoolExpansionInfo(ctx context.Context, poolName string) (*PoolExpansionInfo, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	info := &PoolExpansionInfo{
		PoolName: poolName,
	}

	// 获取池状态
	cmd := exec.CommandContext(ctx, "zpool", "status", "-P", poolName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get pool status: %w", err)
	}

	// 解析池状态
	info.PoolState = parsePoolState(string(output))

	// 获取池大小信息
	sizeCmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-o", "size,allocated", "-p", poolName)
	sizeOutput, err := sizeCmd.Output()
	if err == nil {
		fields := strings.Fields(string(sizeOutput))
		if len(fields) >= 2 {
			info.TotalSize, _ = strconv.ParseUint(fields[0], 10, 64)
			info.AllocatedSize, _ = strconv.ParseUint(fields[1], 10, 64)
		}
	}

	// 解析 VDEV 信息
	vdevs, err := parseVdevInfo(string(output))
	if err != nil {
		return nil, err
	}
	info.Vdevs = vdevs

	// 检查是否可扩展
	info.CanExpand, info.Reason = m.checkCanExpand(vdevs)

	return info, nil
}

// parsePoolState 解析池状态
func parsePoolState(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "state:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "state:"))
		}
	}
	return "unknown"
}

// parseVdevInfo 解析 VDEV 信息
func parseVdevInfo(output string) ([]VdevExpansionInfo, error) {
	var vdevs []VdevExpansionInfo

	lines := strings.Split(output, "\n")
	var currentVdev *VdevExpansionInfo
	var inVdevSection bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行和配置部分
		if line == "" || strings.HasPrefix(line, "config:") || strings.HasPrefix(line, "errors:") {
			continue
		}

		// 检测 RAIDZ VDEV
		if strings.Contains(line, "raidz") {
			inVdevSection = true
			vdevType := "raidz"

			// 确定具体 RAIDZ 类型
			if strings.Contains(line, "raidz3") {
				vdevType = "raidz3"
			} else if strings.Contains(line, "raidz2") {
				vdevType = "raidz2"
			} else if strings.Contains(line, "raidz1") || strings.Contains(line, "raidz-") {
				vdevType = "raidz1"
			}

			currentVdev = &VdevExpansionInfo{
				VdevType: vdevType,
			}
			continue
		}

		// 检测镜像 VDEV
		if strings.Contains(line, "mirror") {
			currentVdev = &VdevExpansionInfo{
				VdevType: "mirror",
			}
			inVdevSection = true
			continue
		}

		// 计算磁盘数
		if inVdevSection && currentVdev != nil {
			// 检测磁盘行（以 /dev/ 开头或包含磁盘标识符）
			if strings.HasPrefix(line, "/dev/") ||
				strings.HasPrefix(line, "da") ||
				strings.HasPrefix(line, "sd") ||
				strings.HasPrefix(line, "nvme") ||
				strings.HasPrefix(line, "wd-") ||
				strings.HasPrefix(line, "disk") {

				currentVdev.Width++

				// 更新奇偶校验盘数
				switch currentVdev.VdevType {
				case "raidz1":
					currentVdev.ParityDisks = 1
				case "raidz2":
					currentVdev.ParityDisks = 2
				case "raidz3":
					currentVdev.ParityDisks = 3
				}
			}
		}

		// 如果遇到新的配置块，保存当前 VDEV
		if inVdevSection && currentVdev != nil && currentVdev.Width > 0 {
			if strings.HasPrefix(line, "NAME") ||
				strings.HasPrefix(line, "pool:") ||
				strings.HasPrefix(line, "state:") ||
				strings.HasPrefix(line, "scan:") {

				currentVdev.DataDisks = currentVdev.Width - currentVdev.ParityDisks
				currentVdev.ExpansionSupported = currentVdev.VdevType == "raidz1" ||
					currentVdev.VdevType == "raidz2" ||
					currentVdev.VdevType == "raidz3"
				currentVdev.CanExpand = currentVdev.ExpansionSupported && currentVdev.DataDisks > 0

				vdevs = append(vdevs, *currentVdev)
				currentVdev = nil
				inVdevSection = false
			}
		}
	}

	// 处理最后一个 VDEV
	if currentVdev != nil && currentVdev.Width > 0 {
		currentVdev.DataDisks = currentVdev.Width - currentVdev.ParityDisks
		currentVdev.ExpansionSupported = currentVdev.VdevType == "raidz1" ||
			currentVdev.VdevType == "raidz2" ||
			currentVdev.VdevType == "raidz3"
		currentVdev.CanExpand = currentVdev.ExpansionSupported && currentVdev.DataDisks > 0

		vdevs = append(vdevs, *currentVdev)
	}

	return vdevs, nil
}

// checkCanExpand 检查是否可扩展
func (m *RAIDZExpansionManager) checkCanExpand(vdevs []VdevExpansionInfo) (bool, string) {
	raidzCount := 0
	for _, vdev := range vdevs {
		if vdev.VdevType == "raidz1" || vdev.VdevType == "raidz2" || vdev.VdevType == "raidz3" {
			raidzCount++
		}
	}

	if raidzCount == 0 {
		return false, "no RAIDZ vdev found in pool"
	}

	if raidzCount > 1 {
		return false, "multiple RAIDZ vdevs detected, expansion not supported"
	}

	return true, ""
}

// ValidateDisk 验证磁盘是否可用于扩展
func (m *RAIDZExpansionManager) ValidateDisk(ctx context.Context, diskPath string) error {
	if !m.available {
		return ErrZFSNotAvailable
	}

	// 检查磁盘是否存在
	if _, err := os.Stat(diskPath); os.IsNotExist(err) {
		return ErrDiskNotFound
	}

	// 检查磁盘是否已在 ZFS 中使用
	cmd := exec.CommandContext(ctx, "zpool", "status")
	output, err := cmd.Output()
	if err == nil {
		if strings.Contains(string(output), diskPath) {
			return ErrDiskInUse
		}
	}

	return nil
}

// StartExpansion 开始 RAIDZ 扩展
func (m *RAIDZExpansionManager) StartExpansion(ctx context.Context, config ExpansionConfig) (*ExpansionStatus, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有扩展在进行
	if m.currentStatus != nil && m.currentStatus.State == ExpansionStateRunning {
		return nil, ErrExpansionInProgress
	}

	// 验证池信息
	poolInfo, err := m.GetPoolExpansionInfo(ctx, config.PoolName)
	if err != nil {
		return nil, err
	}

	if !poolInfo.CanExpand {
		return nil, fmt.Errorf("%w: %s", ErrExpansionNotSupported, poolInfo.Reason)
	}

	// 验证磁盘
	if err := m.ValidateDisk(ctx, config.NewDisk); err != nil {
		return nil, err
	}

	// 查找 RAIDZ VDEV
	var targetVdev *VdevExpansionInfo
	for i := range poolInfo.Vdevs {
		if poolInfo.Vdevs[i].VdevType == "raidz1" ||
			poolInfo.Vdevs[i].VdevType == "raidz2" ||
			poolInfo.Vdevs[i].VdevType == "raidz3" {
			targetVdev = &poolInfo.Vdevs[i]
			break
		}
	}

	if targetVdev == nil {
		return nil, ErrPoolNotRAIDZ
	}

	// 创建扩展状态
	status := &ExpansionStatus{
		ID:             generateExpansionID(config.PoolName),
		PoolName:       config.PoolName,
		State:          ExpansionStatePreparing,
		NewDisk:        config.NewDisk,
		RAIDZLevel:     config.RAIDZLevel,
		OriginalWidth:  targetVdev.Width,
		NewWidth:       targetVdev.Width + 1,
		Progress:       0,
		TotalBytes:     poolInfo.TotalSize,
		StartTime:      time.Now(),
		CanResume:      false,
		CanCancel:      true,
		LastUpdateTime: time.Now(),
	}

	m.currentStatus = status

	// 如果是 dry-run 模式，只返回预期结果
	if config.DryRun {
		status.State = ExpansionStateCompleted
		status.Warnings = append(status.Warnings, "dry-run mode, no actual expansion performed")
		return status, nil
	}

	// 开始异步扩展
	go m.runExpansion(config, status)

	return status, nil
}

// generateExpansionID 生成扩展 ID
func generateExpansionID(poolName string) string {
	return fmt.Sprintf("exp-%s-%d", poolName, time.Now().UnixNano())
}

// runExpansion 执行扩展
func (m *RAIDZExpansionManager) runExpansion(config ExpansionConfig, status *ExpansionStatus) {
	ctx := context.Background()

	// 更新状态为运行中
	m.updateStatus(func(s *ExpansionStatus) {
		s.State = ExpansionStateRunning
		s.StartTime = time.Now()
	})

	// 执行 zpool expand 命令
	// 注意：这是基于 OpenZFS RAIDZ Expansion 特性的实现
	// 该特性在 OpenZFS 2.2.0+ 中引入
	args := []string{"expand", config.PoolName, config.NewDisk}
	if config.Force {
		args = append([]string{"-f"}, args...)
	}

	cmd := exec.CommandContext(ctx, "zpool", args...)

	// 监控进度
	done := make(chan error, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			done <- fmt.Errorf("%w: %s (%s)", ErrExpansionFailed, err.Error(), string(output))
			return
		}
		done <- nil
	}()

	// 进度监控循环
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil {
				m.updateStatus(func(s *ExpansionStatus) {
					s.State = ExpansionStateFailed
					s.EndTime = time.Now()
					s.Errors = append(s.Errors, err.Error())
				})
			} else {
				m.updateStatus(func(s *ExpansionStatus) {
					s.State = ExpansionStateCompleted
					s.Progress = 100
					s.EndTime = time.Now()
				})
			}

			// 记录历史
			m.addToHistory(*m.currentStatus)
			return

		case <-ticker.C:
			// 更新进度
			m.updateProgress(ctx, config.PoolName)

		case <-m.cancelChan:
			// 取消扩展
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			m.updateStatus(func(s *ExpansionStatus) {
				s.State = ExpansionStateCancelled
				s.EndTime = time.Now()
			})
			m.addToHistory(*m.currentStatus)
			return

		case <-m.pauseChan:
			// 暂停扩展
			m.updateStatus(func(s *ExpansionStatus) {
				s.State = ExpansionStatePaused
				s.PauseCount++
				s.CanResume = true
			})

			// 等待恢复或取消
			select {
			case <-m.resumeChan:
				m.updateStatus(func(s *ExpansionStatus) {
					s.State = ExpansionStateRunning
				})
			case <-m.cancelChan:
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				m.updateStatus(func(s *ExpansionStatus) {
					s.State = ExpansionStateCancelled
					s.EndTime = time.Now()
				})
				m.addToHistory(*m.currentStatus)
				return
			}

		case <-ctx.Done():
			// 上下文取消
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			m.updateStatus(func(s *ExpansionStatus) {
				s.State = ExpansionStateCancelled
				s.EndTime = time.Now()
			})
			m.addToHistory(*m.currentStatus)
			return
		}
	}
}

// updateProgress 更新进度
func (m *RAIDZExpansionManager) updateProgress(ctx context.Context, poolName string) {
	// 获取扩展进度
	cmd := exec.CommandContext(ctx, "zpool", "status", "-P", "-v", poolName)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// 解析扩展进度
	progress := parseExpansionProgress(string(output))

	elapsed := time.Since(m.currentStatus.StartTime)
	var speed float64
	var eta time.Duration

	if progress > 0 && progress < 100 {
		// 计算速度和 ETA
		bytesProcessed := uint64(float64(m.currentStatus.TotalBytes) * progress / 100)
		if elapsed.Seconds() > 0 {
			speed = float64(bytesProcessed) / elapsed.Seconds() / 1024 / 1024 // MB/s

			remainingProgress := 100 - progress
			if progress > 0 {
				eta = time.Duration(float64(elapsed) / progress * remainingProgress)
			}
		}
	}

	m.updateStatus(func(s *ExpansionStatus) {
		s.Progress = progress
		s.BytesProcessed = uint64(float64(s.TotalBytes) * progress / 100)
		s.Speed = speed
		s.EstimatedTimeRemaining = eta
		s.LastUpdateTime = time.Now()
	})
}

// parseExpansionProgress 解析扩展进度
func parseExpansionProgress(output string) float64 {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 查找扩展进度行
		if strings.Contains(line, "expand") && strings.Contains(line, "%") {
			// 尝试提取百分比
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.HasSuffix(field, "%") {
					percent := strings.TrimSuffix(field, "%")
					if val, err := strconv.ParseFloat(percent, 64); err == nil {
						return val
					}
				}
			}
		}

		// 检查 scan 行（resilver 进度）
		if strings.Contains(line, "scan:") || strings.Contains(line, "resilver") {
			for _, field := range strings.Fields(line) {
				if strings.HasSuffix(field, "%") {
					percent := strings.TrimSuffix(field, "%")
					if val, err := strconv.ParseFloat(percent, 64); err == nil {
						return val
					}
				}
			}
		}
	}

	// 如果未找到具体进度，基于当前状态估算
	return 0
}

// updateStatus 更新状态
func (m *RAIDZExpansionManager) updateStatus(update func(*ExpansionStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus != nil {
		update(m.currentStatus)
		m.currentStatus.LastUpdateTime = time.Now()

		// 触发回调
		if m.onStateChange != nil {
			go m.onStateChange(m.currentStatus)
		}
	}
}

// addToHistory 添加到历史记录
func (m *RAIDZExpansionManager) addToHistory(status ExpansionStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history.Expansions = append(m.history.Expansions, status)

	// 只保留最近 100 条记录
	if len(m.history.Expansions) > 100 {
		m.history.Expansions = m.history.Expansions[len(m.history.Expansions)-100:]
	}

	_ = m.saveHistory()
}

// PauseExpansion 暂停扩展
func (m *RAIDZExpansionManager) PauseExpansion() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus == nil {
		return ErrNoExpansionInProgress
	}

	if m.currentStatus.State != ExpansionStateRunning {
		return fmt.Errorf("cannot pause expansion in state: %s", m.currentStatus.State)
	}

	select {
	case m.pauseChan <- struct{}{}:
		return nil
	default:
		return ErrExpansionPaused
	}
}

// ResumeExpansion 恢复扩展
func (m *RAIDZExpansionManager) ResumeExpansion() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus == nil {
		return ErrNoExpansionInProgress
	}

	if m.currentStatus.State != ExpansionStatePaused {
		return fmt.Errorf("cannot resume expansion in state: %s", m.currentStatus.State)
	}

	select {
	case m.resumeChan <- struct{}{}:
		return nil
	default:
		return nil
	}
}

// CancelExpansion 取消扩展
func (m *RAIDZExpansionManager) CancelExpansion() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentStatus == nil {
		return ErrNoExpansionInProgress
	}

	if m.currentStatus.State != ExpansionStateRunning &&
		m.currentStatus.State != ExpansionStatePaused {
		return fmt.Errorf("cannot cancel expansion in state: %s", m.currentStatus.State)
	}

	select {
	case m.cancelChan <- struct{}{}:
		return nil
	default:
		return ErrExpansionCancelled
	}
}

// EstimateExpansionTime 估算扩展时间
func (m *RAIDZExpansionManager) EstimateExpansionTime(ctx context.Context, poolName string) (time.Duration, error) {
	if !m.available {
		return 0, ErrZFSNotAvailable
	}

	// 获取池信息
	poolInfo, err := m.GetPoolExpansionInfo(ctx, poolName)
	if err != nil {
		return 0, err
	}

	// 估算时间：基于已分配数据和假设的扩展速度
	// 典型扩展速度约为 100-500 MB/s，取决于硬件
	const assumedSpeedMBps = 200 // MB/s

	dataSizeMB := float64(poolInfo.AllocatedSize) / 1024 / 1024
	estimatedSeconds := dataSizeMB / assumedSpeedMBps

	return time.Duration(estimatedSeconds) * time.Second, nil
}

// CheckExpansionSupport 检查系统是否支持 RAIDZ 扩展
func (m *RAIDZExpansionManager) CheckExpansionSupport() (bool, string) {
	if !m.available {
		return false, "ZFS not available"
	}

	// 检查 ZFS 版本
	cmd := exec.CommandContext(context.Background(), "zfs", "version")
	output, err := cmd.Output()
	if err != nil {
		return false, "cannot determine ZFS version"
	}

	versionStr := string(output)

	// RAIDZ 扩展需要 OpenZFS 2.2.0+
	// 检查版本号
	if strings.Contains(versionStr, "2.2") ||
		strings.Contains(versionStr, "2.3") ||
		strings.Contains(versionStr, "2.4") ||
		strings.Contains(versionStr, "2.5") {
		return true, ""
	}

	// 对于更高版本，也支持
	for i := 2; i <= 10; i++ {
		if strings.Contains(versionStr, fmt.Sprintf("2.%d", i)) {
			return true, ""
		}
	}

	return false, "RAIDZ expansion requires OpenZFS 2.2.0 or later"
}

// Close 关闭管理器
func (m *RAIDZExpansionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果有正在进行的扩展，取消它
	if m.currentStatus != nil && m.currentStatus.State == ExpansionStateRunning {
		select {
		case m.cancelChan <- struct{}{}:
		default:
		}
	}

	// 保存历史
	_ = m.saveHistory()

	return nil
}

// ListAvailableDisks 列出可用磁盘
func (m *RAIDZExpansionManager) ListAvailableDisks(ctx context.Context) ([]string, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	// 获取系统磁盘列表
	cmd := exec.CommandContext(ctx, "lsblk", "-d", "-n", "-o", "PATH,TYPE")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list disks: %w", err)
	}

	// 获取已在 ZFS 中使用的磁盘
	zpoolCmd := exec.CommandContext(ctx, "zpool", "status", "-P")
	zpoolOutput, err := zpoolCmd.Output()
	usedDisks := make(map[string]bool)
	if err == nil {
		lines := strings.Split(string(zpoolOutput), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "/dev/") {
				usedDisks[line] = true
			}
		}
	}

	// 解析可用磁盘
	var availableDisks []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "disk" {
			diskPath := fields[0]
			if !usedDisks[diskPath] {
				availableDisks = append(availableDisks, diskPath)
			}
		}
	}

	return availableDisks, nil
}
