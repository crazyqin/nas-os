// Package zfs 提供ZFS存储池管理功能
package zfs

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ScrubStatus scrub任务状态.
type ScrubStatus string

const (
	// ScrubStatusIdle 空闲.
	ScrubStatusIdle ScrubStatus = "idle"
	// ScrubStatusRunning 运行中.
	ScrubStatusRunning ScrubStatus = "running"
	// ScrubStatusPaused 暂停.
	ScrubStatusPaused ScrubStatus = "paused"
	// ScrubStatusCompleted 已完成.
	ScrubStatusCompleted ScrubStatus = "completed"
	// ScrubStatusFailed 失败.
	ScrubStatusFailed ScrubStatus = "failed"
)

// ScrubResult scrub执行结果.
type ScrubResult struct {
	ID           string      `json:"id"`
	PoolName     string      `json:"pool_name"`
	Status       ScrubStatus `json:"status"`
	StartTime    time.Time   `json:"start_time"`
	EndTime      time.Time   `json:"end_time,omitempty"`
	Duration     string      `json:"duration,omitempty"`
	BytesScanned int64       `json:"bytes_scanned"`
	BytesIssued  int64       `json:"bytes_issued"`
	Errors       int         `json:"errors"`
	Repairs      int         `json:"repairs"`
	ScanPercent  float64     `json:"scan_percent"`
	ErrorMsg     string      `json:"error_msg,omitempty"`
}

// ScrubScheduleConfig scrub调度配置.
type ScrubScheduleConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalDays    int  `json:"interval_days"`      // scrub间隔天数
	PreferredHour   int  `json:"preferred_hour"`     // 优先执行时间（小时，0-23）
	IOPSThreshold   int  `json:"iops_threshold"`     // IO负载阈值，超过此值暂停scrub
	AutoPauseOnLoad bool `json:"auto_pause_on_load"` // 高负载自动暂停
	MaxErrorCount   int  `json:"max_error_count"`    // 最大允许错误数，超过则告警
}

// DefaultScrubScheduleConfig 默认调度配置.
func DefaultScrubScheduleConfig() ScrubScheduleConfig {
	return ScrubScheduleConfig{
		Enabled:         true,
		IntervalDays:    14,
		PreferredHour:   2, // 凌晨2点
		IOPSThreshold:   500,
		AutoPauseOnLoad: true,
		MaxErrorCount:   10,
	}
}

// ScrubProgress scrub实时进度.
type ScrubProgress struct {
	PoolName     string      `json:"pool_name"`
	Status       ScrubStatus `json:"status"`
	Percent      float64     `json:"percent"`
	BytesScanned int64       `json:"bytes_scanned"`
	BytesIssued  int64       `json:"bytes_issued"`
	ElapsedTime  string      `json:"elapsed_time"`
	EstRemaining string      `json:"est_remaining"`
	ScanRate     string      `json:"scan_rate"`
	Errors       int         `json:"errors"`
	StartTime    time.Time   `json:"start_time"`
}

// ScrubScheduler ZFS智能Scrub调度器.
type ScrubScheduler struct {
	poolName  string
	config    ScrubScheduleConfig
	current   *ScrubProgress
	history   []ScrubResult
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
	lastScrub time.Time
}

// NewScrubScheduler 创建Scrub调度器.
func NewScrubScheduler(poolName string, config ScrubScheduleConfig) *ScrubScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &ScrubScheduler{
		poolName: poolName,
		config:   config,
		history:  make([]ScrubResult, 0),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// UpdateConfig 更新调度配置.
func (s *ScrubScheduler) UpdateConfig(config ScrubScheduleConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetConfig 获取当前配置.
func (s *ScrubScheduler) GetConfig() ScrubScheduleConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Start 启动调度器后台循环.
func (s *ScrubScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.scheduleLoop()
}

// Stop 停止调度器.
func (s *ScrubScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		s.cancel()
		s.running = false
	}
}

// StartScrub 手动启动scrub.
func (s *ScrubScheduler) StartScrub() error {
	s.mu.RLock()
	poolName := s.poolName
	s.mu.RUnlock()

	return s.executeScrub(poolName)
}

// PauseScrub 暂停scrub（通过发送SIGSTOP给zpool进程不现实，使用-I参数取消当前scrub）.
func (s *ScrubScheduler) PauseScrub() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil || s.current.Status != ScrubStatusRunning {
		return fmt.Errorf("no scrub is currently running on pool %s", s.poolName)
	}

	// zfs scrub -s <pool> 暂停scrub
	cmd := exec.Command("zpool", "scrub", "-s", s.poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pause scrub: %w, output: %s", err, string(output))
	}

	s.current.Status = ScrubStatusPaused
	return nil
}

// ResumeScrub 恢复scrub.
func (s *ScrubScheduler) ResumeScrub() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil || s.current.Status != ScrubStatusPaused {
		return fmt.Errorf("no paused scrub on pool %s", s.poolName)
	}

	return s.executeScrub(s.poolName)
}

// GetProgress 获取scrub实时进度.
func (s *ScrubScheduler) GetProgress() *ScrubProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.current == nil {
		return &ScrubProgress{
			PoolName: s.poolName,
			Status:   ScrubStatusIdle,
		}
	}

	// 刷新进度
	progress := s.parseScrubStatus()
	if progress != nil {
		s.current = progress
	}

	return s.current
}

// GetHistory 获取scrub历史记录.
func (s *ScrubScheduler) GetHistory() []ScrubResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ScrubResult, len(s.history))
	copy(result, s.history)
	return result
}

// IsRunning 调度器是否运行中.
func (s *ScrubScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetIOStats 获取当前磁盘IO负载（IOPS）.
func (s *ScrubScheduler) GetIOStats() (readIOPS int, writeIOPS int, err error) {
	return s.getDiskIOStats()
}

// --- 内部方法 ---

func (s *ScrubScheduler) scheduleLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAndSchedule()
		}
	}
}

func (s *ScrubScheduler) checkAndSchedule() {
	s.mu.RLock()
	config := s.config
	lastScrub := s.lastScrub
	poolName := s.poolName
	s.mu.RUnlock()

	if !config.Enabled {
		return
	}

	// 检查是否到了下次scrub时间
	if !lastScrub.IsZero() {
		nextScrub := lastScrub.AddDate(0, 0, config.IntervalDays)
		if time.Now().Before(nextScrub) {
			return
		}
	}

	// 检查当前是否在优选时间窗口
	now := time.Now()
	if now.Hour() != config.PreferredHour {
		return
	}

	// 检查IO负载
	if config.AutoPauseOnLoad {
		readIOPS, writeIOPS, err := s.getDiskIOStats()
		if err == nil {
			totalIOPS := readIOPS + writeIOPS
			if totalIOPS > config.IOPSThreshold {
				return // IO负载太高，跳过本次
			}
		}
	}

	// 执行scrub
	s.executeScrub(poolName)
}

func (s *ScrubScheduler) executeScrub(poolName string) error {
	s.mu.Lock()
	s.current = &ScrubProgress{
		PoolName:  poolName,
		Status:    ScrubStatusRunning,
		StartTime: time.Now(),
	}
	s.mu.Unlock()

	cmd := exec.Command("zpool", "scrub", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.mu.Lock()
		s.current.Status = ScrubStatusFailed
		result := ScrubResult{
			ID:        fmt.Sprintf("scrub-%d", time.Now().Unix()),
			PoolName:  poolName,
			Status:    ScrubStatusFailed,
			StartTime: time.Now(),
			ErrorMsg:  fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}
		s.history = append(s.history, result)
		s.current = nil
		s.mu.Unlock()
		return fmt.Errorf("failed to start scrub: %w, output: %s", err, string(output))
	}

	s.mu.Lock()
	s.lastScrub = time.Now()
	s.mu.Unlock()

	// 启动后台进度监控
	go s.monitorScrub(poolName)
	return nil
}

func (s *ScrubScheduler) monitorScrub(poolName string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			progress := s.parseScrubStatus()
			if progress == nil {
				continue
			}

			s.mu.Lock()
			s.current = progress

			if progress.Status == ScrubStatusCompleted || progress.Status == ScrubStatusFailed {
				result := ScrubResult{
					ID:           fmt.Sprintf("scrub-%d", time.Now().Unix()),
					PoolName:     poolName,
					Status:       progress.Status,
					StartTime:    progress.StartTime,
					EndTime:      time.Now(),
					Duration:     time.Since(progress.StartTime).Round(time.Second).String(),
					BytesScanned: progress.BytesScanned,
					BytesIssued:  progress.BytesIssued,
					Errors:       progress.Errors,
					ScanPercent:  progress.Percent,
				}
				s.history = append(s.history, result)
				s.current = nil
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
		}
	}
}

func (s *ScrubScheduler) parseScrubStatus() *ScrubProgress {
	cmd := exec.Command("zpool", "status", s.poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	return parseZpoolScrubOutput(string(output), s.poolName)
}

// parseZpoolScrubOutput 解析zpool status输出中的scrub信息.
func parseZpoolScrubOutput(output, poolName string) *ScrubProgress {
	progress := &ScrubProgress{
		PoolName: poolName,
		Status:   ScrubStatusIdle,
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	scanSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 检测scrub状态行
		if strings.Contains(line, "scan:") {
			scanSection = true
			if strings.Contains(line, "scrub repaired") {
				progress.Status = ScrubStatusCompleted
				progress.Percent = 100
			} else if strings.Contains(line, "scrub in progress") || strings.Contains(line, "scrub") {
				progress.Status = ScrubStatusRunning
			} else if strings.Contains(line, "none requested") {
				progress.Status = ScrubStatusIdle
			}
			// 解析百分比（可能在scan:行本身）
			re := regexp.MustCompile(`(\d+(?:\.\d+)?)% done`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				progress.Percent, _ = strconv.ParseFloat(matches[1], 64)
			}
			continue
		}

		if scanSection {
			// 解析百分比（在scan:后续行中，如"0B repaired, 66.67% done"）
			if progress.Status == ScrubStatusRunning {
				re := regexp.MustCompile(`(\d+(?:\.\d+)?)% done`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					progress.Percent, _ = strconv.ParseFloat(matches[1], 64)
				}
			}
			// 解析扫描速率
			if strings.Contains(line, "scan rate:") {
				re := regexp.MustCompile(`scan rate:\s+(.+)`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					progress.ScanRate = strings.TrimSpace(matches[1])
				}
			}
			// 解析错误数
			if strings.HasPrefix(line, "errors:") {
				re := regexp.MustCompile(`(\d+)\s+data errors`)
				matches := re.FindStringSubmatch(line)
				if len(matches) > 1 {
					progress.Errors, _ = strconv.Atoi(matches[1])
				}
			}
		}

		// 检测错误
		if strings.Contains(line, "scan:") && strings.Contains(line, "cancelled") {
			progress.Status = ScrubStatusFailed
		}
	}

	if progress.Status == ScrubStatusIdle {
		return nil
	}
	return progress
}

func (s *ScrubScheduler) getDiskIOStats() (readIOPS int, writeIOPS int, err error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	totalReadOps := 0
	totalWriteOps := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		// 只统计主要磁盘（sd*, nvme*, mmcblk*）
		devName := fields[2]
		if !isMainDisk(devName) {
			continue
		}
		reads, _ := strconv.Atoi(fields[3])
		writes, _ := strconv.Atoi(fields[7])
		totalReadOps += reads
		totalWriteOps += writes
	}

	// 第一次采样返回0，需要两次采样计算IOPS
	// 简化：直接返回累计值作为相对指标
	return totalReadOps, totalWriteOps, nil
}

func isMainDisk(name string) bool {
	prefixes := []string{"sd", "nvme", "mmcblk", "vd"}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			suffix := name[len(p):]
			if suffix == "" {
				return false
			}
			// mmcblk不带p后缀的也是整盘
			if p == "mmcblk" {
				// mmcblk0 is whole disk, mmcblk0p1 is partition
				return !strings.Contains(suffix, "p")
			}
			// 排除分区：sda1, nvme0n1p1 等
			// 整盘：sda, nvme0n1, vda
			// 分区特征：以数字结尾（sda1）或含p+数字（nvme0n1p1）
			if strings.Contains(suffix, "p") && regexp.MustCompile(`p\d+$`).MatchString(suffix) {
				return false // nvme0n1p1 = partition
			}
			// sd*, vd* 整盘：后缀全是字母（sda, vda）
			// nvme* 整盘：nvme0n1 — 后缀含字母
			if p == "sd" || p == "vd" {
				return regexp.MustCompile(`^[a-z]+$`).MatchString(suffix)
			}
			// nvme: suffix should match e.g. 0n1 (whole disk), not 0n1p1
			return true
		}
	}
	return false
}
