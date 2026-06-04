// Package ransomshield - 恢复点管理
// 多级快照策略、增量快照、生命周期管理、回滚验证
package ransomshield

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 恢复点管理器
// ============================================================

// RecoveryManager 恢复点管理器
type RecoveryManager struct {
	mu sync.RWMutex

	// points 恢复点 (id -> RecoveryPoint)
	points map[string]*RecoveryPoint

	// policies 快照策略
	policies []SnapshotPolicy

	// config 配置
	config RecoveryConfig

	// callbacks
	snapshotFunc   func(path string) (string, error)
	restoreFunc    func(snapshotID, targetPath string) error
	deleteFunc     func(snapshotID string) error

	// stats 统计
	stats RecoveryStats

	// running 运行状态
	running bool
	stopChan chan struct{}
}

// SnapshotPolicy 快照策略
type SnapshotPolicy struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Enabled      bool          `json:"enabled"`
	Paths        []string      `json:"paths"`
	Interval     time.Duration `json:"interval"`
	MaxRetention int           `json:"max_retention"` // 最大保留数
	MinRetention int           `json:"min_retention"` // 最小保留数
	RetentionByAge time.Duration `json:"retention_by_age"` // 最大保留时长
	Type         RecoveryType  `json:"type"`
	PreThreat    bool          `json:"pre_threat"` // 威胁前自动快照
}

// RecoveryConfig 恢复配置
type RecoveryConfig struct {
	MaxTotalPoints  int           `json:"max_total_points"`
	MaxDiskUsageGB  int64         `json:"max_disk_usage_gb"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	VerifyOnCreate  bool          `json:"verify_on_create"`
	CompressionEnabled bool       `json:"compression_enabled"`
}

// RecoveryStats 恢复统计
type RecoveryStats struct {
	TotalCreated     int64     `json:"total_created"`
	TotalDeleted     int64     `json:"total_deleted"`
	TotalRestored    int64     `json:"total_restored"`
	TotalVerified    int64     `json:"total_verified"`
	ActivePoints     int       `json:"active_points"`
	DiskUsageBytes   int64     `json:"disk_usage_bytes"`
	LastCleanupTime  time.Time `json:"last_cleanup_time"`
	LastCreateTime   time.Time `json:"last_create_time"`
	LastRestoreTime  time.Time `json:"last_restore_time"`
	AvgCreateMs      int64     `json:"avg_create_ms"`
	AvgRestoreMs     int64     `json:"avg_restore_ms"`
}

// ============================================================
// 构造与生命周期
// ============================================================

// NewRecoveryManager 创建恢复点管理器
func NewRecoveryManager(config RecoveryConfig) *RecoveryManager {
	rm := &RecoveryManager{
		points:   make(map[string]*RecoveryPoint),
		config:   config,
		stopChan: make(chan struct{}),
	}

	rm.initDefaultPolicies()
	return rm
}

// initDefaultPolicies 初始化默认快照策略
func (rm *RecoveryManager) initDefaultPolicies() {
	rm.policies = []SnapshotPolicy{
		{
			ID: "hourly", Name: "每小时快照", Enabled: true,
			Paths: []string{"/data"},
			Interval: 1 * time.Hour, MaxRetention: 24, MinRetention: 6,
			RetentionByAge: 24 * time.Hour, Type: RecoveryTypeScheduled,
		},
		{
			ID: "daily", Name: "每日快照", Enabled: true,
			Paths: []string{"/data"},
			Interval: 24 * time.Hour, MaxRetention: 7, MinRetention: 3,
			RetentionByAge: 7 * 24 * time.Hour, Type: RecoveryTypeScheduled,
		},
		{
			ID: "pre-threat", Name: "威胁前快照", Enabled: true,
			Paths: []string{"/data"},
			Interval: 0, MaxRetention: 10, MinRetention: 5,
			Type: RecoveryTypePreemptive, PreThreat: true,
		},
	}
}

// SetSnapshotFunc 设置快照创建函数
func (rm *RecoveryManager) SetSnapshotFunc(fn func(path string) (string, error)) {
	rm.mu.Lock()
	rm.snapshotFunc = fn
	rm.mu.Unlock()
}

// SetRestoreFunc 设置快照恢复函数
func (rm *RecoveryManager) SetRestoreFunc(fn func(snapshotID, targetPath string) error) {
	rm.mu.Lock()
	rm.restoreFunc = fn
	rm.mu.Unlock()
}

// SetDeleteFunc 设置快照删除函数
func (rm *RecoveryManager) SetDeleteFunc(fn func(snapshotID string) error) {
	rm.mu.Lock()
	rm.deleteFunc = fn
	rm.mu.Unlock()
}

// Start 启动恢复点管理器
func (rm *RecoveryManager) Start() {
	rm.mu.Lock()
	if rm.running {
		rm.mu.Unlock()
		return
	}
	rm.running = true
	rm.mu.Unlock()

	go rm.scheduledSnapshotLoop()
	go rm.cleanupLoop()

	log.Println("[Recovery] 恢复点管理器已启动")
}

// Stop 停止恢复点管理器
func (rm *RecoveryManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if !rm.running {
		return
	}
	close(rm.stopChan)
	rm.running = false
	log.Println("[Recovery] 恢复点管理器已停止")
}

// ============================================================
// 快照创建
// ============================================================

// CreateRecoveryPoint 创建恢复点
func (rm *RecoveryManager) CreateRecoveryPoint(name, path, description string, rType RecoveryType, threatLevel ThreatLevel) (*RecoveryPoint, error) {
	startTime := time.Now()

	if rm.snapshotFunc == nil {
		return nil, fmt.Errorf("快照函数未设置")
	}

	// 创建快照
	snapshotID, err := rm.snapshotFunc(path)
	if err != nil {
		return nil, fmt.Errorf("快照创建失败: %w", err)
	}

	// 统计文件数量
	fileCount := 0
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})

	now := time.Now()
	rp := &RecoveryPoint{
		ID:           uuid.New().String(),
		Name:         name,
		Description:  description,
		Type:         rType,
		Path:         snapshotID,
		ThreatLevel:  threatLevel,
		FilesCount:   fileCount,
		Status:       RecoveryStatusReady,
		CreatedAt:    now,
	}

	rm.mu.Lock()
	rm.points[rp.ID] = rp
	rm.stats.TotalCreated++
	rm.stats.ActivePoints = len(rm.points)
	rm.stats.LastCreateTime = now

	// 更新平均创建时间
	createMs := time.Since(startTime).Milliseconds()
	if rm.stats.TotalCreated == 1 {
		rm.stats.AvgCreateMs = createMs
	} else {
		rm.stats.AvgCreateMs = (rm.stats.AvgCreateMs*(rm.stats.TotalCreated-1) + createMs) / rm.stats.TotalCreated
	}
	rm.mu.Unlock()

	// 验证
	if rm.config.VerifyOnCreate {
		go rm.verifyRecoveryPoint(rp.ID)
	}

	log.Printf("[Recovery] 恢复点已创建: ID=%s, Name=%s, Path=%s, Files=%d",
		rp.ID, name, path, fileCount)

	return rp, nil
}

// CreateAutoRecoveryPoint 创建自动恢复点（威胁触发）
func (rm *RecoveryManager) CreateAutoRecoveryPoint(triggerPath, threatID string, level ThreatLevel) (*RecoveryPoint, error) {
	dir := filepath.Dir(triggerPath)
	name := fmt.Sprintf("auto-threat-%s-%s", level.String(), time.Now().Format("20060102-150405"))
	desc := fmt.Sprintf("威胁触发的自动恢复点 (威胁ID: %s, 级别: %s)", threatID, level.String())

	return rm.CreateRecoveryPoint(name, dir, desc, RecoveryTypeAuto, level)
}

// CreateScheduledRecoveryPoint 创建定时恢复点
func (rm *RecoveryManager) CreateScheduledRecoveryPoint(path string) (*RecoveryPoint, error) {
	name := fmt.Sprintf("scheduled-%s", time.Now().Format("20060102-150405"))
	return rm.CreateRecoveryPoint(name, path, "定时自动快照", RecoveryTypeScheduled, ThreatLevelNone)
}

// ============================================================
// 快照恢复
// ============================================================

// Restore 恢复到指定恢复点
func (rm *RecoveryManager) Restore(recoveryPointID, targetPath string, dryRun bool) (*RestoreResult, error) {
	startTime := time.Now()

	rm.mu.RLock()
	rp, ok := rm.points[recoveryPointID]
	if !ok {
		rm.mu.RUnlock()
		return nil, fmt.Errorf("恢复点不存在: %s", recoveryPointID)
	}

	if rp.Status != RecoveryStatusReady {
		rm.mu.RUnlock()
		return nil, fmt.Errorf("恢复点状态不可用: %s", rp.Status)
	}

	rpID := rp.ID
	rpPath := rp.Path
	rm.mu.RUnlock()

	result := &RestoreResult{
		RecoveryPointID: rpID,
		TargetPath:      targetPath,
		DryRun:          dryRun,
		StartTime:       startTime,
	}

	if dryRun {
		result.Status = "dry_run_ok"
		result.Details = "试运行成功，未执行实际恢复"
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	// 标记为回滚中
	rm.mu.Lock()
	rp.Status = RecoveryStatusRollback
	rm.mu.Unlock()

	// 执行恢复
	if rm.restoreFunc != nil {
		if err := rm.restoreFunc(rpPath, targetPath); err != nil {
			rm.mu.Lock()
			rp.Status = RecoveryStatusReady
			rm.mu.Unlock()
			return nil, fmt.Errorf("恢复失败: %w", err)
		}
	} else {
		// 使用默认恢复逻辑（快照回滚）
		if err := rm.defaultRestore(rpPath, targetPath); err != nil {
			rm.mu.Lock()
			rp.Status = RecoveryStatusReady
			rm.mu.Unlock()
			return nil, err
		}
	}

	endTime := time.Now()

	rm.mu.Lock()
	rp.Status = RecoveryStatusReady
	rm.stats.TotalRestored++
	rm.stats.LastRestoreTime = endTime
	restoreMs := endTime.Sub(startTime).Milliseconds()
	if rm.stats.TotalRestored == 1 {
		rm.stats.AvgRestoreMs = restoreMs
	} else {
		rm.stats.AvgRestoreMs = (rm.stats.AvgRestoreMs*(rm.stats.TotalRestored-1) + restoreMs) / rm.stats.TotalRestored
	}
	rm.mu.Unlock()

	result.Status = "completed"
	result.DurationMs = restoreMs
	result.Details = fmt.Sprintf("成功恢复到恢复点 %s", rpID)

	log.Printf("[Recovery] 恢复完成: 恢复点=%s, 目标=%s, 耗时=%dms",
		rpID, targetPath, restoreMs)

	return result, nil
}

// RestoreResult 恢复结果
type RestoreResult struct {
	RecoveryPointID string `json:"recovery_point_id"`
	TargetPath      string `json:"target_path"`
	DryRun          bool   `json:"dry_run"`
	Status          string `json:"status"`
	Details         string `json:"details"`
	DurationMs      int64  `json:"duration_ms"`
	StartTime       time.Time `json:"start_time"`
}

// defaultRestore 默认恢复逻辑
func (rm *RecoveryManager) defaultRestore(snapshotID, targetPath string) error {
	// 检查快照路径是否存在
	if _, err := os.Stat(snapshotID); os.IsNotExist(err) {
		return fmt.Errorf("快照路径不存在: %s", snapshotID)
	}

	// 尝试使用 Btrfs/ZFS 命令恢复
	if err := rm.tryBtrfsRestore(snapshotID, targetPath); err == nil {
		return nil
	}

	if err := rm.tryZfsRestore(snapshotID, targetPath); err == nil {
		return nil
	}

	// 回退到 rsync
	return rm.tryRsyncRestore(snapshotID, targetPath)
}

// tryBtrfsRestore 尝试 Btrfs 快照恢复
func (rm *RecoveryManager) tryBtrfsRestore(snapshot, target string) error {
	// btrfs subvolume snapshot <snapshot> <target>
	cmd := exec.Command("btrfs", "subvolume", "snapshot", snapshot, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs 恢复失败: %s", string(output))
	}
	return nil
}

// tryZfsRestore 尝试 ZFS 快照恢复
func (rm *RecoveryManager) tryZfsRestore(snapshot, target string) error {
	// zfs rollback <snapshot>
	cmd := exec.Command("zfs", "rollback", snapshot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zfs 恢复失败: %s", string(output))
	}
	return nil
}

// tryRsyncRestore 使用 rsync 恢复
func (rm *RecoveryManager) tryRsyncRestore(source, target string) error {
	cmd := exec.Command("rsync", "-av", "--delete", source+"/", target+"/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync 恢复失败: %s", string(output))
	}
	return nil
}

// ============================================================
// 验证
// ============================================================

// verifyRecoveryPoint 验证恢复点完整性
func (rm *RecoveryManager) verifyRecoveryPoint(id string) {
	rm.mu.RLock()
	rp, ok := rm.points[id]
	if !ok {
		rm.mu.RUnlock()
		return
	}
	rpPath := rp.Path
	rm.mu.RUnlock()

	// 检查快照路径是否存在
	if _, err := os.Stat(rpPath); os.IsNotExist(err) {
		log.Printf("[Recovery] 验证失败: 快照路径不存在 %s", rpPath)
		return
	}

	rm.mu.Lock()
	rm.stats.TotalVerified++
	rm.mu.Unlock()

	log.Printf("[Recovery] 恢复点验证通过: %s", id)
}

// ============================================================
// 查询接口
// ============================================================

// GetRecoveryPoint 获取恢复点
func (rm *RecoveryManager) GetRecoveryPoint(id string) (*RecoveryPoint, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	rp, ok := rm.points[id]
	if !ok {
		return nil, false
	}
	result := *rp
	return &result, true
}

// ListRecoveryPoints 列出恢复点
func (rm *RecoveryManager) ListRecoveryPoints(status RecoveryStatus, rType RecoveryType, limit int) []RecoveryPoint {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []RecoveryPoint
	for _, rp := range rm.points {
		if status != "" && rp.Status != status {
			continue
		}
		if rType != "" && rp.Type != rType {
			continue
		}
		result = append(result, *rp)
	}

	// 按创建时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetStats 获取统计信息
func (rm *RecoveryManager) GetStats() RecoveryStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := rm.stats
	stats.ActivePoints = len(rm.points)
	return stats
}

// GetPolicies 获取快照策略
func (rm *RecoveryManager) GetPolicies() []SnapshotPolicy {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]SnapshotPolicy, len(rm.policies))
	copy(result, rm.policies)
	return result
}

// AddPolicy 添加快照策略
func (rm *RecoveryManager) AddPolicy(policy SnapshotPolicy) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	rm.policies = append(rm.policies, policy)
}

// ============================================================
// 清理
// ============================================================

// scheduledSnapshotLoop 定时快照循环
func (rm *RecoveryManager) scheduledSnapshotLoop() {
	// 使用最短的策略间隔
	minInterval := time.Hour
	for _, p := range rm.policies {
		if p.Enabled && p.Type == RecoveryTypeScheduled && p.Interval > 0 && p.Interval < minInterval {
			minInterval = p.Interval
		}
	}

	ticker := time.NewTicker(minInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.stopChan:
			return
		case <-ticker.C:
			rm.runScheduledSnapshots()
		}
	}
}

// runScheduledSnapshots 执行定时快照
func (rm *RecoveryManager) runScheduledSnapshots() {
	rm.mu.RLock()
	var policies []SnapshotPolicy
	for _, p := range rm.policies {
		if p.Enabled && p.Type == RecoveryTypeScheduled && p.Interval > 0 {
			policies = append(policies, p)
		}
	}
	rm.mu.RUnlock()

	for _, policy := range policies {
		// 检查是否到了该策略的快照时间
		lastSnapshot := rm.getLastSnapshotTime(policy.ID)
		if time.Since(lastSnapshot) < policy.Interval {
			continue
		}

		for _, path := range policy.Paths {
			_, err := rm.CreateRecoveryPoint(
				fmt.Sprintf("%s-%s", policy.Name, time.Now().Format("20060102-150405")),
				path,
				fmt.Sprintf("策略 %s 触发的定时快照", policy.Name),
				policy.Type,
				ThreatLevelNone,
			)
			if err != nil {
				log.Printf("[Recovery] 定时快照失败: %v", err)
			}
		}
	}
}

// getLastSnapshotTime 获取策略的最后快照时间
func (rm *RecoveryManager) getLastSnapshotTime(policyID string) time.Time {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var latest time.Time
	for _, rp := range rm.points {
		if strings.Contains(rp.Name, policyID) && rp.CreatedAt.After(latest) {
			latest = rp.CreatedAt
		}
	}
	return latest
}

// cleanupLoop 清理循环
func (rm *RecoveryManager) cleanupLoop() {
	interval := rm.config.CleanupInterval
	if interval == 0 {
		interval = 1 * time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.stopChan:
			return
		case <-ticker.C:
			rm.cleanup()
		}
	}
}

// cleanup 清理过期和超额的恢复点
func (rm *RecoveryManager) cleanup() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	var toDelete []string

	// 1. 按策略清理超龄的快照
	for _, policy := range rm.policies {
		if policy.RetentionByAge <= 0 {
			continue
		}

		var policyPoints []*RecoveryPoint
		for _, rp := range rm.points {
			if rp.Type == policy.Type {
				policyPoints = append(policyPoints, rp)
			}
		}

		// 按时间排序
		sort.Slice(policyPoints, func(i, j int) bool {
			return policyPoints[i].CreatedAt.After(policyPoints[j].CreatedAt)
		})

		// 保留最小数量
		for i, rp := range policyPoints {
			if i < policy.MinRetention {
				continue
			}
			if now.Sub(rp.CreatedAt) > policy.RetentionByAge {
				toDelete = append(toDelete, rp.ID)
			}
			// 超过最大数量
			if i >= policy.MaxRetention {
				toDelete = append(toDelete, rp.ID)
			}
		}
	}

	// 2. 全局限制
	if rm.config.MaxTotalPoints > 0 {
		var allPoints []*RecoveryPoint
		for _, rp := range rm.points {
			allPoints = append(allPoints, rp)
		}
		sort.Slice(allPoints, func(i, j int) bool {
			return allPoints[i].CreatedAt.After(allPoints[j].CreatedAt)
		})

		if len(allPoints) > rm.config.MaxTotalPoints {
			for _, rp := range allPoints[rm.config.MaxTotalPoints:] {
				toDelete = append(toDelete, rp.ID)
			}
		}
	}

	// 3. 执行删除
	seen := make(map[string]bool)
	for _, id := range toDelete {
		if seen[id] {
			continue
		}
		seen[id] = true

		rp := rm.points[id]
		if rp.Status == RecoveryStatusRollback {
			continue // 跳过正在回滚的
		}

		// 删除快照
		if rm.deleteFunc != nil {
			if err := rm.deleteFunc(rp.Path); err != nil {
				log.Printf("[Recovery] 删除快照失败: %s, %v", rp.Path, err)
				continue
			}
		}

		delete(rm.points, id)
		rm.stats.TotalDeleted++
		rm.stats.ActivePoints = len(rm.points)
		log.Printf("[Recovery] 恢复点已清理: %s", id)
	}

	rm.stats.LastCleanupTime = now
}

// DeleteRecoveryPoint 手动删除恢复点
func (rm *RecoveryManager) DeleteRecoveryPoint(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rp, ok := rm.points[id]
	if !ok {
		return fmt.Errorf("恢复点不存在: %s", id)
	}

	if rp.Status == RecoveryStatusRollback {
		return fmt.Errorf("恢复点正在使用中")
	}

	if rm.deleteFunc != nil {
		if err := rm.deleteFunc(rp.Path); err != nil {
			return fmt.Errorf("删除快照失败: %w", err)
		}
	}

	delete(rm.points, id)
	rm.stats.TotalDeleted++
	rm.stats.ActivePoints = len(rm.points)

	log.Printf("[Recovery] 恢复点已删除: %s", id)
	return nil
}
