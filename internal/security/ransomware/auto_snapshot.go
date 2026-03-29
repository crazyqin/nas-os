package ransomware

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AutoSnapshotManager 自动快照管理器.
// 在检测到勒索软件威胁时自动创建快照保护文件.
type AutoSnapshotManager struct {
	config      AutoSnapshotConfig
	snapshots   map[string]*ProtectionSnapshot
	snapshotMu  sync.RWMutex
	manifest    string
	snapshotSvc SnapshotServiceInterface
	stats       SnapshotStats
	statsMu     sync.RWMutex
}

// AutoSnapshotConfig 自动快照配置.
type AutoSnapshotConfig struct {
	Enabled           bool          `json:"enabled"`
	SnapshotDir       string        `json:"snapshot_dir"`
	MaxSnapshots      int           `json:"max_snapshots"`       // 最大快照数
	MaxSnapshotSize   int64         `json:"max_snapshot_size"`   // 最大快照总大小（字节）
	SnapshotRetention time.Duration `json:"snapshot_retention"`  // 快照保留时间
	AutoTrigger       bool          `json:"auto_trigger"`        // 自动触发快照
	TriggerThreshold  int           `json:"trigger_threshold"`   // 触发阈值（威胁分数）
	ProtectedPaths    []string      `json:"protected_paths"`     // 保护的路径
	ExcludePaths      []string      `json:"exclude_paths"`       // 排除的路径
	Compression       bool          `json:"compression"`         // 是否压缩快照
}

// DefaultAutoSnapshotConfig 默认配置.
func DefaultAutoSnapshotConfig() AutoSnapshotConfig {
	return AutoSnapshotConfig{
		Enabled:           true,
		SnapshotDir:       "/var/lib/nas-os/ransomware-snapshots",
		MaxSnapshots:      100,
		MaxSnapshotSize:   50 * 1024 * 1024 * 1024, // 50GB
		SnapshotRetention: 7 * 24 * time.Hour,      // 7天
		AutoTrigger:       true,
		TriggerThreshold:  30,
		ProtectedPaths:    []string{"/data", "/shares", "/home"},
		ExcludePaths:      []string{"/proc", "/sys", "/dev", "/run", "/tmp"},
		Compression:       false,
	}
}

// ProtectionSnapshot 保护快照.
type ProtectionSnapshot struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	TriggerReason string                 `json:"trigger_reason"`    // 触发原因
	ThreatLevel   ThreatLevel            `json:"threat_level"`      // 威胁级别
	DetectionID   string                 `json:"detection_id"`      // 关联的检测ID
	ProtectedPath string                 `json:"protected_path"`    // 保护的路径
	SnapshotPath  string                 `json:"snapshot_path"`     // 快照存储路径
	FileCount     int                    `json:"file_count"`        // 快照文件数
	Size          int64                  `json:"size"`              // 快照大小
	Status        SnapshotStatus         `json:"status"`            // 快照状态
	ExpiresAt     time.Time              `json:"expires_at"`        // 过期时间
	Restored      bool                   `json:"restored"`          // 是否已恢复
	RestoredAt    *time.Time             `json:"restored_at,omitempty"`
	RestoredTo    string                 `json:"restored_to,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SnapshotStatus 快照状态.
type SnapshotStatus string

const (
	SnapshotStatusCreating   SnapshotStatus = "creating"
	SnapshotStatusComplete   SnapshotStatus = "complete"
	SnapshotStatusFailed     SnapshotStatus = "failed"
	SnapshotStatusExpired    SnapshotStatus = "expired"
	SnapshotStatusRestoring  SnapshotStatus = "restoring"
	SnapshotStatusRestored   SnapshotStatus = "restored"
	SnapshotStatusDeleted    SnapshotStatus = "deleted"
)

// SnapshotStats 快照统计.
type SnapshotStats struct {
	TotalSnapshots    int64         `json:"total_snapshots"`
	TotalSize         int64         `json:"total_size"`
	ByThreatLevel     map[ThreatLevel]int64 `json:"by_threat_level"`
	ByStatus          map[SnapshotStatus]int64 `json:"by_status"`
	LastSnapshotTime  *time.Time    `json:"last_snapshot_time,omitempty"`
	RestoredCount     int64         `json:"restored_count"`
}

// SnapshotServiceInterface 快照服务接口.
// 用于与系统快照服务集成.
type SnapshotServiceInterface interface {
	CreateSnapshot(path, name string) (string, error)
	RestoreSnapshot(snapshotID, targetPath string) error
	DeleteSnapshot(snapshotID string) error
	ListSnapshots() ([]string, error)
}

// NewAutoSnapshotManager 创建自动快照管理器.
func NewAutoSnapshotManager(config AutoSnapshotConfig) (*AutoSnapshotManager, error) {
	// 确保快照目录存在
	if err := os.MkdirAll(config.SnapshotDir, 0700); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	asm := &AutoSnapshotManager{
		config:     config,
		snapshots:  make(map[string]*ProtectionSnapshot),
		manifest:   filepath.Join(config.SnapshotDir, "manifest.json"),
		stats:      SnapshotStats{
			ByThreatLevel: make(map[ThreatLevel]int64),
			ByStatus:      make(map[SnapshotStatus]int64),
		},
	}

	// 加载现有清单
	if err := asm.loadManifest(); err != nil {
		// 清单不存在或损坏，初始化空清单
		asm.snapshots = make(map[string]*ProtectionSnapshot)
	}

	// 清理过期快照
	asm.CleanupExpired()

	return asm, nil
}

// SetSnapshotService 设置快照服务.
func (asm *AutoSnapshotManager) SetSnapshotService(svc SnapshotServiceInterface) {
	asm.snapshotSvc = svc
}

// TriggerSnapshot 触发保护快照.
// 当检测到勒索软件威胁时调用.
func (asm *AutoSnapshotManager) TriggerSnapshot(result *DetectionResult) (*ProtectionSnapshot, error) {
	if !asm.config.Enabled {
		return nil, fmt.Errorf("自动快照功能未启用")
	}

	// 检查威胁级别是否达到阈值
	if !asm.shouldTrigger(result.ThreatLevel, result.Confidence) {
		return nil, nil
	}

	// 确定要保护的路径
	protectedPath := asm.determineProtectedPath(result)
	if protectedPath == "" {
		return nil, fmt.Errorf("无法确定保护路径")
	}

	// 检查是否在排除路径中
	if asm.isExcluded(protectedPath) {
		return nil, fmt.Errorf("路径被排除: %s", protectedPath)
	}

	// 检查空间限制
	if err := asm.checkSpaceLimit(); err != nil {
		return nil, err
	}

	// 创建快照
	snapshot, err := asm.createSnapshot(protectedPath, result)
	if err != nil {
		return nil, fmt.Errorf("创建快照失败: %w", err)
	}

	// 存储快照记录
	asm.storeSnapshot(snapshot)

	// 更新统计
	asm.updateStats(snapshot)

	return snapshot, nil
}

// shouldTrigger 检查是否应该触发快照.
func (asm *AutoSnapshotManager) shouldTrigger(level ThreatLevel, confidence float64) bool {
	if !asm.config.AutoTrigger {
		return false
	}

	// 基于威胁级别和置信度计算分数
	levelScore := map[ThreatLevel]int{
		ThreatLevelLow:      10,
		ThreatLevelMedium:   20,
		ThreatLevelHigh:     40,
		ThreatLevelCritical: 60,
	}

	score := levelScore[level]
	confidenceScore := int(confidence * 50)

	totalScore := score + confidenceScore
	return totalScore >= asm.config.TriggerThreshold
}

// determineProtectedPath 确定要保护的路径.
func (asm *AutoSnapshotManager) determineProtectedPath(result *DetectionResult) string {
	// 首先检查受影响的路径
	if result.FilePath != "" {
		// 找到包含该文件的保护路径
		for _, protected := range asm.config.ProtectedPaths {
			if strings.HasPrefix(result.FilePath, protected) {
				return protected
			}
		}
		// 如果不在保护路径中，使用文件所在目录
		return filepath.Dir(result.FilePath)
	}

	// 如果有多个受影响文件，找到共同的父目录
	if len(result.AffectedFiles) > 0 {
		return asm.findCommonParent(result.AffectedFiles)
	}

	// 默认使用第一个保护路径
	if len(asm.config.ProtectedPaths) > 0 {
		return asm.config.ProtectedPaths[0]
	}

	return ""
}

// findCommonParent 找到共同父目录.
func (asm *AutoSnapshotManager) findCommonParent(files []string) string {
	if len(files) == 0 {
		return ""
	}

	common := filepath.Dir(files[0])
	for _, f := range files[1:] {
		dir := filepath.Dir(f)
		for !strings.HasPrefix(dir, common) && common != "/" {
			common = filepath.Dir(common)
		}
		if common == "/" {
			break
		}
	}

	// 确保在保护路径内
	for _, protected := range asm.config.ProtectedPaths {
		if strings.HasPrefix(common, protected) {
			return protected
		}
	}

	return common
}

// isExcluded 检查路径是否被排除.
func (asm *AutoSnapshotManager) isExcluded(path string) bool {
	for _, excluded := range asm.config.ExcludePaths {
		if strings.HasPrefix(path, excluded) {
			return true
		}
	}
	return false
}

// checkSpaceLimit 检查空间限制.
func (asm *AutoSnapshotManager) checkSpaceLimit() error {
	asm.snapshotMu.RLock()
	var currentSize int64
	var count int
	for _, s := range asm.snapshots {
		if s.Status == SnapshotStatusComplete || s.Status == SnapshotStatusCreating {
			currentSize += s.Size
			count++
		}
	}
	asm.snapshotMu.RUnlock()

	if count >= asm.config.MaxSnapshots {
		return fmt.Errorf("快照数量已达上限: %d", asm.config.MaxSnapshots)
	}

	if currentSize >= asm.config.MaxSnapshotSize {
		return fmt.Errorf("快照总大小已达上限: %d GB", asm.config.MaxSnapshotSize/(1024*1024*1024))
	}

	return nil
}

// createSnapshot 创建快照.
func (asm *AutoSnapshotManager) createSnapshot(path string, result *DetectionResult) (*ProtectionSnapshot, error) {
	// 生成快照ID
	snapshotID := generateSnapshotID()
	snapshotDir := filepath.Join(asm.config.SnapshotDir, snapshotID)

	// 创建快照目录
	if err := os.MkdirAll(snapshotDir, 0700); err != nil {
		return nil, err
	}

	snapshot := &ProtectionSnapshot{
		ID:            snapshotID,
		Timestamp:     time.Now(),
		TriggerReason: result.SuggestedAction,
		ThreatLevel:   result.ThreatLevel,
		DetectionID:   result.ID,
		ProtectedPath: path,
		SnapshotPath:  snapshotDir,
		Status:        SnapshotStatusCreating,
		ExpiresAt:     time.Now().Add(asm.config.SnapshotRetention),
		Metadata: map[string]interface{}{
			"detection_type": result.DetectionType,
			"confidence":     result.Confidence,
			"signature":      result.SignatureName,
		},
	}

	// 如果有系统快照服务，使用系统快照
	if asm.snapshotSvc != nil {
		systemSnapshotID, err := asm.snapshotSvc.CreateSnapshot(path, snapshotID)
		if err != nil {
			snapshot.Status = SnapshotStatusFailed
			asm.storeSnapshot(snapshot)
			return nil, fmt.Errorf("系统快照创建失败: %w", err)
		}
		snapshot.Metadata["system_snapshot_id"] = systemSnapshotID
	} else {
		// 否则创建文件级快照（复制关键文件）
		fileCount, size, err := asm.createFileSnapshot(path, snapshotDir)
		if err != nil {
			snapshot.Status = SnapshotStatusFailed
			asm.storeSnapshot(snapshot)
			return nil, fmt.Errorf("文件快照创建失败: %w", err)
		}
		snapshot.FileCount = fileCount
		snapshot.Size = size
	}

	snapshot.Status = SnapshotStatusComplete
	return snapshot, nil
}

// createFileSnapshot 创建文件级快照.
func (asm *AutoSnapshotManager) createFileSnapshot(srcPath, dstPath string) (int, int64, error) {
	var fileCount int
	var totalSize int64

	// 记录快照元数据
	metadataPath := filepath.Join(dstPath, "snapshot.meta")
	metadata := map[string]interface{}{
		"source_path": srcPath,
		"created_at":  time.Now(),
		"version":     "1.0",
	}
	metaData, _ := json.MarshalIndent(metadata, "", "  ")
	if err := os.WriteFile(metadataPath, metaData, 0600); err != nil {
		return 0, 0, err
	}

	// 创建文件列表（记录文件信息而非复制全部内容）
	fileListPath := filepath.Join(dstPath, "files.list")
	var fileList []FileInfoRecord

	err := walkDirectory(srcPath, func(path string, info os.FileInfo) error {
		// 跳过排除路径
		if asm.isExcluded(path) {
			return nil
		}

		fileList = append(fileList, FileInfoRecord{
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode(),
			IsDir:    info.IsDir(),
		})

		fileCount++
		totalSize += info.Size()

		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	// 保存文件列表
	listData, _ := json.MarshalIndent(fileList, "", "  ")
	if err := os.WriteFile(fileListPath, listData, 0600); err != nil {
		return 0, 0, err
	}

	// 对于关键文件（小文件），创建实际副本
	criticalFilesPath := filepath.Join(dstPath, "critical")
	if err := os.MkdirAll(criticalFilesPath, 0700); err != nil {
		return 0, 0, err
	}

	// 复制关键文件（小于10MB的文件）
	for _, fi := range fileList {
		if !fi.IsDir && fi.Size < 10*1024*1024 {
			relPath := strings.TrimPrefix(fi.Path, srcPath)
			dstFile := filepath.Join(criticalFilesPath, relPath)
			if err := copyFile(fi.Path, dstFile); err != nil {
				// 记录错误但继续
				continue
			}
		}
	}

	return fileCount, totalSize, nil
}

// FileInfoRecord 文件信息记录.
type FileInfoRecord struct {
	Path    string      `json:"path"`
	Size    int64       `json:"size"`
	ModTime time.Time   `json:"mod_time"`
	Mode    os.FileMode `json:"mode"`
	IsDir   bool        `json:"is_dir"`
}

// storeSnapshot 存储快照记录.
func (asm *AutoSnapshotManager) storeSnapshot(snapshot *ProtectionSnapshot) {
	asm.snapshotMu.Lock()
	asm.snapshots[snapshot.ID] = snapshot
	asm.snapshotMu.Unlock()

	// 保存清单
	_ = asm.saveManifest()
}

// GetSnapshot 获取快照.
func (asm *AutoSnapshotManager) GetSnapshot(id string) (*ProtectionSnapshot, bool) {
	asm.snapshotMu.RLock()
	defer asm.snapshotMu.RUnlock()

	snapshot, ok := asm.snapshots[id]
	return snapshot, ok
}

// ListSnapshots 列出快照.
func (asm *AutoSnapshotManager) ListSnapshots(limit, offset int, threatLevel *ThreatLevel) []*ProtectionSnapshot {
	asm.snapshotMu.RLock()
	defer asm.snapshotMu.RUnlock()

	var snapshots []*ProtectionSnapshot
	for _, s := range asm.snapshots {
		if threatLevel != nil && s.ThreatLevel != *threatLevel {
			continue
		}
		snapshots = append(snapshots, s)
	}

	// 按时间排序
	sortSnapshotsByTime(snapshots)

	// 分页
	if offset >= len(snapshots) {
		return []*ProtectionSnapshot{}
	}
	end := offset + limit
	if end > len(snapshots) {
		end = len(snapshots)
	}

	return snapshots[offset:end]
}

// RestoreSnapshot 恢复快照.
func (asm *AutoSnapshotManager) RestoreSnapshot(id, targetPath string) error {
	asm.snapshotMu.Lock()
	defer asm.snapshotMu.Unlock()

	snapshot, ok := asm.snapshots[id]
	if !ok {
		return ErrSnapshotNotFound
	}

	if snapshot.Status != SnapshotStatusComplete {
		return fmt.Errorf("快照状态无效: %s", snapshot.Status)
	}

	if snapshot.Restored {
		return fmt.Errorf("快照已恢复")
	}

	snapshot.Status = SnapshotStatusRestoring

	// 如果有系统快照服务，使用系统恢复
	if asm.snapshotSvc != nil {
		if systemID, ok := snapshot.Metadata["system_snapshot_id"].(string); ok {
			if err := asm.snapshotSvc.RestoreSnapshot(systemID, targetPath); err != nil {
				snapshot.Status = SnapshotStatusComplete
				return fmt.Errorf("系统快照恢复失败: %w", err)
			}
		}
	} else {
		// 文件级恢复
		if err := asm.restoreFileSnapshot(snapshot, targetPath); err != nil {
			snapshot.Status = SnapshotStatusComplete
			return fmt.Errorf("文件快照恢复失败: %w", err)
		}
	}

	snapshot.Status = SnapshotStatusRestored
	snapshot.Restored = true
	now := time.Now()
	snapshot.RestoredAt = &now
	snapshot.RestoredTo = targetPath

	// 更新统计
	asm.statsMu.Lock()
	asm.stats.RestoredCount++
	asm.statsMu.Unlock()

	// 保存清单
	_ = asm.saveManifest()

	return nil
}

// restoreFileSnapshot 恢复文件快照.
func (asm *AutoSnapshotManager) restoreFileSnapshot(snapshot *ProtectionSnapshot, targetPath string) error {
	criticalPath := filepath.Join(snapshot.SnapshotPath, "critical")
	if _, err := os.Stat(criticalPath); os.IsNotExist(err) {
		return fmt.Errorf("关键文件目录不存在")
	}

	// 复制关键文件回目标位置
	return walkDirectory(criticalPath, func(path string, info os.FileInfo) error {
		relPath := strings.TrimPrefix(path, criticalPath)
		dstPath := filepath.Join(targetPath, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// DeleteSnapshot 删除快照.
func (asm *AutoSnapshotManager) DeleteSnapshot(id string) error {
	asm.snapshotMu.Lock()
	defer asm.snapshotMu.Unlock()

	snapshot, ok := asm.snapshots[id]
	if !ok {
		return ErrSnapshotNotFound
	}

	// 删除快照目录
	if err := os.RemoveAll(snapshot.SnapshotPath); err != nil {
		// 记录错误但继续
	}

	// 如果有系统快照，也删除
	if asm.snapshotSvc != nil {
		if systemID, ok := snapshot.Metadata["system_snapshot_id"].(string); ok {
			_ = asm.snapshotSvc.DeleteSnapshot(systemID)
		}
	}

	delete(asm.snapshots, id)

	// 更新统计
	asm.statsMu.Lock()
	asm.stats.TotalSnapshots--
	asm.stats.TotalSize -= snapshot.Size
	asm.statsMu.Unlock()

	// 保存清单
	_ = asm.saveManifest()

	return nil
}

// CleanupExpired 清理过期快照.
func (asm *AutoSnapshotManager) CleanupExpired() int {
	asm.snapshotMu.Lock()
	defer asm.snapshotMu.Unlock()

	now := time.Now()
	var cleaned int

	for id, snapshot := range asm.snapshots {
		if snapshot.ExpiresAt.Before(now) && snapshot.Status == SnapshotStatusComplete {
			// 删除快照目录
			_ = os.RemoveAll(snapshot.SnapshotPath)

			// 如果有系统快照，也删除
			if asm.snapshotSvc != nil {
				if systemID, ok := snapshot.Metadata["system_snapshot_id"].(string); ok {
					_ = asm.snapshotSvc.DeleteSnapshot(systemID)
				}
			}

			delete(asm.snapshots, id)
			cleaned++
		}
	}

	if cleaned > 0 {
		asm.stats.TotalSnapshots -= int64(cleaned)
		_ = asm.saveManifest()
	}

	return cleaned
}

// GetStats 获取统计信息.
func (asm *AutoSnapshotManager) GetStats() SnapshotStats {
	asm.statsMu.RLock()
	defer asm.statsMu.RUnlock()

	stats := asm.stats
	asm.snapshotMu.RLock()
	stats.TotalSnapshots = int64(len(asm.snapshots))
	var totalSize int64
	for _, s := range asm.snapshots {
		totalSize += s.Size
	}
	stats.TotalSize = totalSize
	asm.snapshotMu.RUnlock()

	return stats
}

// loadManifest 加载清单.
func (asm *AutoSnapshotManager) loadManifest() error {
	data, err := os.ReadFile(asm.manifest)
	if err != nil {
		return err
	}

	var snapshots []*ProtectionSnapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return err
	}

	for _, s := range snapshots {
		asm.snapshots[s.ID] = s
	}

	return nil
}

// saveManifest 保存清单.
func (asm *AutoSnapshotManager) saveManifest() error {
	snapshots := make([]*ProtectionSnapshot, 0, len(asm.snapshots))
	for _, s := range asm.snapshots {
		snapshots = append(snapshots, s)
	}

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(asm.manifest, data, 0600)
}

// updateStats 更新统计.
func (asm *AutoSnapshotManager) updateStats(snapshot *ProtectionSnapshot) {
	asm.statsMu.Lock()
	defer asm.statsMu.Unlock()

	asm.stats.TotalSnapshots++
	asm.stats.TotalSize += snapshot.Size
	asm.stats.ByThreatLevel[snapshot.ThreatLevel]++
	asm.stats.ByStatus[snapshot.Status]++
	asm.stats.LastSnapshotTime = &snapshot.Timestamp
}

// Helper functions

func generateSnapshotID() string {
	return fmt.Sprintf("snap_%d", time.Now().UnixNano())
}

func sortSnapshotsByTime(snapshots []*ProtectionSnapshot) {
	for i := 0; i < len(snapshots)-1; i++ {
		for j := i + 1; j < len(snapshots); j++ {
			if snapshots[i].Timestamp.Before(snapshots[j].Timestamp) {
				snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
			}
		}
	}
}

func walkDirectory(path string, fn func(string, os.FileInfo) error) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}
		return fn(p, info)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	// 确保目标目录存在
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	// 复制内容
	buf := make([]byte, 32*1024)
	for {
		n, err := srcFile.Read(buf)
		if n == 0 || err != nil {
			break
		}
		if _, err := dstFile.Write(buf[:n]); err != nil {
			return err
		}
	}

	// 同步
	return dstFile.Sync()
}

// ErrSnapshotNotFound 快照不存在错误.
var ErrSnapshotNotFound = fmt.Errorf("快照不存在")

// BatchTriggerSnapshots 批量触发快照.
func (asm *AutoSnapshotManager) BatchTriggerSnapshots(results []*DetectionResult) ([]*ProtectionSnapshot, []error) {
	var snapshots []*ProtectionSnapshot
	var errors []error

	for _, result := range results {
		snapshot, err := asm.TriggerSnapshot(result)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", result.FilePath, err))
			continue
		}
		if snapshot != nil {
			snapshots = append(snapshots, snapshot)
		}
	}

	return snapshots, errors
}

// GetSnapshotsForDetection 获取与特定检测关联的快照.
func (asm *AutoSnapshotManager) GetSnapshotsForDetection(detectionID string) []*ProtectionSnapshot {
	asm.snapshotMu.RLock()
	defer asm.snapshotMu.RUnlock()

	var snapshots []*ProtectionSnapshot
	for _, s := range asm.snapshots {
		if s.DetectionID == detectionID {
			snapshots = append(snapshots, s)
		}
	}
	return snapshots
}

// VerifySnapshot 验证快照完整性.
func (asm *AutoSnapshotManager) VerifySnapshot(id string) (bool, error) {
	asm.snapshotMu.RLock()
	snapshot, ok := asm.snapshots[id]
	asm.snapshotMu.RUnlock()

	if !ok {
		return false, ErrSnapshotNotFound
	}

	// 检查快照目录是否存在
	if _, err := os.Stat(snapshot.SnapshotPath); os.IsNotExist(err) {
		return false, fmt.Errorf("快照目录不存在")
	}

	// 检查元数据文件
	metadataPath := filepath.Join(snapshot.SnapshotPath, "snapshot.meta")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return false, fmt.Errorf("快照元数据不存在")
	}

	// 检查文件列表
	fileListPath := filepath.Join(snapshot.SnapshotPath, "files.list")
	if _, err := os.Stat(fileListPath); os.IsNotExist(err) {
		return false, fmt.Errorf("文件列表不存在")
	}

	return true, nil
}