// Package storage provides Btrfs scrub functionality
// 对标群晖DSM数据清洗scrub + 文件自愈功能
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== Btrfs Scrub 类型定义 ==========

// ScrubStatus scrub状态
type ScrubStatus string

const (
	ScrubStatusIdle      ScrubStatus = "idle"      // 空闲
	ScrubStatusRunning   ScrubStatus = "running"   // 运行中
	ScrubStatusPaused    ScrubStatus = "paused"    // 已暂停
	ScrubStatusCompleted ScrubStatus = "completed" // 已完成
	ScrubStatusCancelled ScrubStatus = "cancelled" // 已取消
	ScrubStatusError     ScrubStatus = "error"     // 错误
)

// ScrubMode scrub模式（对标群晖）
type ScrubMode string

const (
	// ScrubModeFull 全量清洗（校验所有数据块）
	ScrubModeFull ScrubMode = "full"
	// ScrubModeIncremental 增量清洗（只校验使用扇区）
	ScrubModeIncremental ScrubMode = "incremental"
	// ScrubModeQuick 快速清洗（只校验元数据）
	ScrubModeQuick ScrubMode = "quick"
)

// ScrubProgress scrub进度
type ScrubProgress struct {
	// 卷名称
	VolumeName string `json:"volumeName"`

	// 状态
	Status ScrubStatus `json:"status"`

	// 模式
	Mode ScrubMode `json:"mode"`

	// 进度百分比 (0-100)
	Percent float64 `json:"percent"`

	// 已校验数据量（字节）
	DataScanned uint64 `json:"dataScanned"`

	// 总数据量（字节）
	DataTotal uint64 `json:"dataTotal"`

	// 发现错误数
	Errors uint64 `json:"errors"`

	// 已修复错误数
	ErrorsFixed uint64 `json:"errorsFixed"`

	// 未修复错误数
	ErrorsUnfixed uint64 `json:"errorsUnfixed"`

	// 开始时间
	StartTime *time.Time `json:"startTime,omitempty"`

	// 预计结束时间
	EstimatedEndTime *time.Time `json:"estimatedEndTime,omitempty"`

	// 校验速度（MB/s）
	SpeedMBps float64 `json:"speedMBps"`

	// 运行时长（秒）
	DurationSeconds uint64 `json:"durationSeconds"`
}

// ScrubConfig scrub配置
type ScrubConfig struct {
	// 卷名称
	VolumeName string `json:"volumeName"`

	// scrub模式
	Mode ScrubMode `json:"mode"`

	// 是否自动修复（对标群晖：只修复使用扇区）
	AutoRepair bool `json:"autoRepair"`

	// 优先级（low/normal/high）
	Priority string `json:"priority"`

	// 速率限制（MB/s，0表示不限）
	RateLimitMBps uint64 `json:"rateLimitMbps"`

	// IOPS限制（0表示不限）
	IOPSLimit uint64 `json:"iopsLimit"`

	// 暂停/恢复支持
	PauseResume bool `json:"pauseResume"`

	// 调度时间（cron表达式）
	Schedule string `json:"schedule"`

	// 错误阈值（超过此数值告警）
	ErrorThreshold uint64 `json:"errorThreshold"`

	// 完成后通知
	NotifyOnComplete bool `json:"notifyOnComplete"`

	// 错误时通知
	NotifyOnError bool `json:"notifyOnError"`
}

// ScrubReport scrub报告（对标群晖详细报告）
type ScrubReport struct {
	// 报告ID
	ID string `json:"id"`

	// 卷名称
	VolumeName string `json:"volumeName"`

	// 模式
	Mode ScrubMode `json:"mode"`

	// 状态
	Status ScrubStatus `json:"status"`

	// 开始时间
	StartTime time.Time `json:"startTime"`

	// 结束时间
	EndTime time.Time `json:"endTime"`

	// 运行时长
	Duration time.Duration `json:"duration"`

	// 已校验数据量（字节）
	DataScanned uint64 `json:"dataScanned"`

	// 发现错误详情
	ErrorDetails []ScrubErrorDetail `json:"errorDetails"`

	// 错误统计
	ErrorSummary ScrubErrorSummary `json:"errorSummary"`

	// 修复统计
	RepairSummary ScrubRepairSummary `json:"repairSummary"`

	// 健康评分（0-100）
	HealthScore int `json:"healthScore"`

	// 建议
	Recommendations []string `json:"recommendations"`

	// 生成时间
	GeneratedAt time.Time `json:"generatedAt"`
}

// ScrubErrorDetail 错误详情
type ScrubErrorDetail struct {
	// 错误类型
	Type string `json:"type"` // checksum, metadata, data, etc.

	// 错误位置（字节偏移）
	Offset uint64 `json:"offset"`

	// 错误设备
	Device string `json:"device"`

	// 是否已修复
	Fixed bool `json:"fixed"`

	// 修复方法
	RepairMethod string `json:"repairMethod,omitempty"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// ScrubErrorSummary 错误统计
type ScrubErrorSummary struct {
	// 校验和错误
	ChecksumErrors uint64 `json:"checksumErrors"`

	// 元数据错误
	MetadataErrors uint64 `json:"metadataErrors"`

	// 数据错误
	DataErrors uint64 `json:"dataErrors"`

	// 总错误数
	TotalErrors uint64 `json:"totalErrors"`
}

// ScrubRepairSummary 修复统计
type ScrubRepairSummary struct {
	// 已修复数
	Fixed uint64 `json:"fixed"`

	// 未修复数
	Unfixed uint64 `json:"unfixed"`

	// 自动修复数（从镜像副本恢复）
	AutoFixed uint64 `json:"autoFixed"`

	// 手动修复数
	ManualFixed uint64 `json:"manualFixed"`

	// 无法修复数（标记为坏块）
	MarkedBad uint64 `json:"markedBad"`
}

// ========== Btrfs Scrub Manager ==========

// BtrfsScrubManager btrfs scrub管理器
type BtrfsScrubManager struct {
	mu sync.RWMutex

	// 进度跟踪
	progress map[string]*ScrubProgress

	// 配置
	configs map[string]*ScrubConfig

	// 报告历史
	reports map[string][]*ScrubReport

	// 取消函数
	cancelFuncs map[string]context.CancelFunc

	// 日志
	logger *zap.Logger

	// 事件回调
	callbacks ScrubCallbacks
}

// ScrubCallbacks scrub事件回调
type ScrubCallbacks struct {
	// 进度更新回调
	OnProgress func(volumeName string, progress ScrubProgress)

	// 错误发现回调
	OnError func(volumeName string, errorDetail ScrubErrorDetail)

	// 完成回调
	OnComplete func(volumeName string, report ScrubReport)
}

// NewBtrfsScrubManager 创建scrub管理器
func NewBtrfsScrubManager(logger *zap.Logger) *BtrfsScrubManager {
	return &BtrfsScrubManager{
		progress:    make(map[string]*ScrubProgress),
		configs:     make(map[string]*ScrubConfig),
		reports:     make(map[string][]*ScrubReport),
		cancelFuncs: make(map[string]context.CancelFunc),
		logger:      logger,
	}
}

// SetCallbacks 设置回调
func (m *BtrfsScrubManager) SetCallbacks(callbacks ScrubCallbacks) {
	m.callbacks = callbacks
}

// ========== 核心API（对标群晖DSM scrub功能） ==========

// StartScrub 启动scrub任务
func (m *BtrfsScrubManager) StartScrub(config ScrubConfig) (*ScrubProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有scrub运行
	if existing, exists := m.progress[config.VolumeName]; exists {
		if existing.Status == ScrubStatusRunning {
			return nil, fmt.Errorf("卷 %s 已有scrub任务运行中", config.VolumeName)
		}
	}

	// 创建进度对象
	progress := &ScrubProgress{
		VolumeName: config.VolumeName,
		Status:     ScrubStatusRunning,
		Mode:       config.Mode,
		Percent:    0,
		StartTime:  &time.Time{},
	}
	*progress.StartTime = time.Now()

	m.progress[config.VolumeName] = progress
	m.configs[config.VolumeName] = &config

	// 启动scrub任务
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFuncs[config.VolumeName] = cancel

	go m.runScrub(ctx, config)

	m.logger.Info("启动scrub任务",
		zap.String("volume", config.VolumeName),
		zap.String("mode", string(config.Mode)),
		zap.Bool("auto_repair", config.AutoRepair))

	return progress, nil
}

// runScrub 执行scrub
func (m *BtrfsScrubManager) runScrub(ctx context.Context, config ScrubConfig) {
	// 执行btrfs scrub命令
	// 根据模式选择不同参数
	args := []string{"device", "scrub", "start"}

	if config.Mode == ScrubModeIncremental {
		// 增量模式：只校验使用扇区（对标群晖）
		args = append(args, "-B") // 后台运行
	}

	// 查找卷挂载点
	mountPoint := m.findMountPoint(config.VolumeName)
	if mountPoint == "" {
		m.mu.Lock()
		if p, exists := m.progress[config.VolumeName]; exists {
			p.Status = ScrubStatusError
		}
		m.mu.Unlock()
		return
	}

	cmd := exec.CommandContext(ctx, "btrfs", append(args, mountPoint)...)
	output, err := cmd.CombinedOutput()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新进度状态
	progress, exists := m.progress[config.VolumeName]
	if !exists {
		return
	}

	if err != nil {
		progress.Status = ScrubStatusError
		m.logger.Error("scrub执行失败",
			zap.String("volume", config.VolumeName),
			zap.Error(err),
			zap.String("output", string(output)))
		return
	}

	// 解析scrub状态
	m.parseScrubStatus(config.VolumeName, string(output))

	// 创建报告
	report := m.generateReport(config.VolumeName)

	// 保存报告
	m.reports[config.VolumeName] = append(m.reports[config.VolumeName], report)
	if len(m.reports[config.VolumeName]) > 100 {
		m.reports[config.VolumeName] = m.reports[config.VolumeName][:100]
	}

	// 触发完成回调
	if m.callbacks.OnComplete != nil {
		go m.callbacks.OnComplete(config.VolumeName, *report)
	}
}

// PauseScrub 暂停scrub（对标群晖暂停恢复功能）
func (m *BtrfsScrubManager) PauseScrub(volumeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	progress, exists := m.progress[volumeName]
	if !exists || progress.Status != ScrubStatusRunning {
		return fmt.Errorf("卷 %s 没有运行中的scrub任务", volumeName)
	}

	// 执行btrfs scrub pause
	mountPoint := m.findMountPoint(volumeName)
	cmd := exec.Command("btrfs", "device", "scrub", "pause", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("暂停scrub失败: %w", err)
	}

	progress.Status = ScrubStatusPaused
	m.logger.Info("暂停scrub任务", zap.String("volume", volumeName))

	return nil
}

// ResumeScrub 恢复scrub
func (m *BtrfsScrubManager) ResumeScrub(volumeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	progress, exists := m.progress[volumeName]
	if !exists || progress.Status != ScrubStatusPaused {
		return fmt.Errorf("卷 %s 没有已暂停的scrub任务", volumeName)
	}

	// 执行btrfs scrub resume
	mountPoint := m.findMountPoint(volumeName)
	cmd := exec.Command("btrfs", "device", "scrub", "resume", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("恢复scrub失败: %w", err)
	}

	progress.Status = ScrubStatusRunning
	m.logger.Info("恢复scrub任务", zap.String("volume", volumeName))

	return nil
}

// CancelScrub 取消scrub
func (m *BtrfsScrubManager) CancelScrub(volumeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cancel, exists := m.cancelFuncs[volumeName]
	if !exists {
		return fmt.Errorf("卷 %s 没有scrub任务", volumeName)
	}

	// 取消上下文
	cancel()

	// 执行btrfs scrub cancel
	mountPoint := m.findMountPoint(volumeName)
	cmd := exec.Command("btrfs", "device", "scrub", "cancel", mountPoint)
	_ = cmd.Run() // 忽略错误

	if progress, exists := m.progress[volumeName]; exists {
		progress.Status = ScrubStatusCancelled
	}

	m.logger.Info("取消scrub任务", zap.String("volume", volumeName))

	return nil
}

// GetProgress 获取scrub进度
func (m *BtrfsScrubManager) GetProgress(volumeName string) (*ScrubProgress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	progress, exists := m.progress[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 没有scrub进度", volumeName)
	}

	// 如果正在运行，实时获取状态
	if progress.Status == ScrubStatusRunning {
		m.updateProgressFromSystem(volumeName, progress)
	}

	return progress, nil
}

// GetReport 获取scrub报告
func (m *BtrfsScrubManager) GetReport(volumeName string, reportID string) (*ScrubReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports, exists := m.reports[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 没有scrub报告", volumeName)
	}

	for _, r := range reports {
		if r.ID == reportID {
			return r, nil
		}
	}

	return nil, fmt.Errorf("报告 %s 不存在", reportID)
}

// GetReportHistory 获取报告历史
func (m *BtrfsScrubManager) GetReportHistory(volumeName string, limit int) []*ScrubReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports, exists := m.reports[volumeName]
	if !exists {
		return nil
	}

	if limit > 0 && len(reports) > limit {
		return reports[:limit]
	}

	return reports
}

// ========== 辅助方法 ==========

// findMountPoint 查找卷挂载点
func (m *BtrfsScrubManager) findMountPoint(volumeName string) string {
	// 从/proc/mounts或mount命令解析
	cmd := exec.Command("mount", "-t", "btrfs")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, volumeName) {
			// 格式: /dev/sdX on /mount/point type btrfs ...
			parts := strings.Split(line, " ")
			if len(parts) >= 3 && parts[2] == "on" {
				return parts[3]
			}
		}
	}

	return ""
}

// parseScrubStatus 解析scrub状态输出
func (m *BtrfsScrubManager) parseScrubStatus(volumeName string, output string) {
	progress, exists := m.progress[volumeName]
	if !exists {
		return
	}

	// 解析btrfs scrub status输出
	// 典型输出格式:
	// scrub status for <mount>
	// scrub started at <time>
	// data_extents_scrubbed: <count>
	// tree_extents_scrubbed: <count>
	// data_bytes_scrubbed: <bytes>
	// tree_bytes_scrubbed: <bytes>
	// read_errors: <count>
	// csum_errors: <count>
	// ...

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "data_bytes_scrubbed:") {
			val := strings.TrimPrefix(line, "data_bytes_scrubbed:")
			progress.DataScanned = parseUint64(strings.TrimSpace(val))
		}

		if strings.HasPrefix(line, "csum_errors:") {
			val := strings.TrimPrefix(line, "csum_errors:")
			progress.Errors += parseUint64(strings.TrimSpace(val))
		}

		if strings.HasPrefix(line, "read_errors:") {
			val := strings.TrimPrefix(line, "read_errors:")
			progress.Errors += parseUint64(strings.TrimSpace(val))
		}

		if strings.Contains(line, "finished") {
			progress.Status = ScrubStatusCompleted
			progress.Percent = 100
		}
	}
}

// updateProgressFromSystem 从系统获取实时进度
func (m *BtrfsScrubManager) updateProgressFromSystem(volumeName string, progress *ScrubProgress) {
	mountPoint := m.findMountPoint(volumeName)
	if mountPoint == "" {
		return
	}

	cmd := exec.Command("btrfs", "device", "scrub", "status", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	m.parseScrubStatus(volumeName, string(output))
}

// generateReport 生成scrub报告
func (m *BtrfsScrubManager) generateReport(volumeName string) *ScrubReport {
	progress, exists := m.progress[volumeName]
	if !exists {
		return nil
	}

	report := &ScrubReport{
		ID:              fmt.Sprintf("scrub-%d", time.Now().UnixNano()),
		VolumeName:      volumeName,
		Mode:            progress.Mode,
		Status:          progress.Status,
		StartTime:       *progress.StartTime,
		EndTime:         time.Now(),
		Duration:        time.Since(*progress.StartTime),
		DataScanned:     progress.DataScanned,
		ErrorDetails:    []ScrubErrorDetail{},
		ErrorSummary:    ScrubErrorSummary{},
		RepairSummary:   ScrubRepairSummary{},
		HealthScore:     100 - int(progress.Errors),
		Recommendations: []string{},
		GeneratedAt:     time.Now(),
	}

	// 如果有错误，添加建议
	if progress.Errors > 0 {
		report.Recommendations = append(report.Recommendations,
			"建议检查磁盘SMART状态",
			"如有镜像副本，可尝试自动修复",
			"建议创建新快照保护数据")
	}

	if progress.Errors > 10 {
		report.Recommendations = append(report.Recommendations,
			"错误数过多，建议立即更换磁盘")
	}

	return report
}

// SaveConfig 保存配置到文件
func (m *BtrfsScrubManager) SaveConfig(configPath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.configs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// LoadConfig 从文件加载配置
func (m *BtrfsScrubManager) LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return json.Unmarshal(data, &m.configs)
}
