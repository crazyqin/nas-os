package ransomware

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// QuarantineManager 隔离管理器.
type QuarantineManager struct {
	config    QuarantineConfig
	entries   map[string]*QuarantineEntry
	entryMu   sync.RWMutex
	manifest  string // 清单文件路径
	stats     Statistics
	statsMu   sync.RWMutex
}

// NewQuarantineManager 创建隔离管理器.
func NewQuarantineManager(config QuarantineConfig) (*QuarantineManager, error) {
	// 确保隔离目录存在
	if err := os.MkdirAll(config.QuarantineDir, 0700); err != nil {
		return nil, fmt.Errorf("创建隔离目录失败: %w", err)
	}

	qm := &QuarantineManager{
		config:   config,
		entries:  make(map[string]*QuarantineEntry),
		manifest: filepath.Join(config.QuarantineDir, "manifest.json"),
	}

	// 加载现有清单
	if err := qm.loadManifest(); err != nil {
		// 清单不存在或损坏，忽略错误
		qm.entries = make(map[string]*QuarantineEntry)
	}

	return qm, nil
}

// QuarantineFile 隔离文件.
func (qm *QuarantineManager) QuarantineFile(filePath, reason, detectionID string, threatLevel ThreatLevel, signatureName string) (*QuarantineEntry, error) {
	if !qm.config.Enabled {
		return nil, errors.New("隔离功能未启用")
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在或无法访问: %w", err)
	}

	// 检查隔离区空间
	if err := qm.checkSpace(fileInfo.Size()); err != nil {
		return nil, err
	}

	// 计算文件哈希
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// 生成隔离条目
	entryID := uuid.New().String()
	quarantineFileName := entryID + ".quarantine"
	quarantinePath := filepath.Join(qm.config.QuarantineDir, quarantineFileName)

	// 移动文件到隔离区
	if err := qm.moveFileToQuarantine(filePath, quarantinePath); err != nil {
		return nil, fmt.Errorf("移动文件到隔离区失败: %w", err)
	}

	entry := &QuarantineEntry{
		ID:             entryID,
		OriginalPath:   filePath,
		QuarantinePath: quarantinePath,
		FileSize:       fileInfo.Size(),
		FileHash:       fileHash,
		Timestamp:      time.Now(),
		Reason:         reason,
		DetectionID:    detectionID,
		ThreatLevel:    threatLevel,
		SignatureName:  signatureName,
		Restored:       false,
	}

	// 存储条目
	qm.entryMu.Lock()
	qm.entries[entryID] = entry
	qm.entryMu.Unlock()

	// 保存清单
	if err := qm.saveManifest(); err != nil {
		// 清单保存失败，回滚
		os.Remove(quarantinePath)
		qm.entryMu.Lock()
		delete(qm.entries, entryID)
		qm.entryMu.Unlock()
		return nil, fmt.Errorf("保存隔离清单失败: %w", err)
	}

	// 更新统计
	qm.updateStats(entry.FileSize, true)

	return entry, nil
}

// moveFileToQuarantine 移动文件到隔离区.
func (qm *QuarantineManager) moveFileToQuarantine(src, dst string) error {
	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 复制内容（可以在此处添加加密）
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(dst)
		return err
	}

	// 确保数据写入磁盘
	if err := dstFile.Sync(); err != nil {
		os.Remove(dst)
		return err
	}

	// 删除原文件
	srcFile.Close()
	if err := os.Remove(src); err != nil {
		// 删除失败，但文件已复制到隔离区
		// 记录日志但返回成功
	}

	return nil
}

// RestoreFile 恢复文件.
func (qm *QuarantineManager) RestoreFile(entryID, restoredBy string) error {
	qm.entryMu.Lock()
	defer qm.entryMu.Unlock()

	entry, ok := qm.entries[entryID]
	if !ok {
		return ErrQuarantineEntryNotFound
	}

	if entry.Restored {
		return errors.New("文件已恢复")
	}

	// 检查目标路径是否存在
	if _, err := os.Stat(entry.OriginalPath); err == nil {
		// 目标位置已有文件，生成新文件名
		ext := filepath.Ext(entry.OriginalPath)
		base := strings.TrimSuffix(entry.OriginalPath, ext)
		entry.OriginalPath = fmt.Sprintf("%s_restored_%s%s", base, time.Now().Format("20060102150405"), ext)
	}

	// 从隔离区复制回原位置
	srcFile, err := os.Open(entry.QuarantinePath)
	if err != nil {
		return fmt.Errorf("无法访问隔离文件: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(entry.OriginalPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("无法创建目标文件: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(entry.OriginalPath)
		return fmt.Errorf("恢复文件失败: %w", err)
	}

	// 更新条目
	entry.Restored = true
	now := time.Now()
	entry.RestoredAt = &now
	entry.RestoredBy = restoredBy

	// 删除隔离文件
	os.Remove(entry.QuarantinePath)

	// 保存清单
	if err := qm.saveManifest(); err != nil {
		// 记录错误但继续
	}

	// 更新统计
	qm.statsMu.Lock()
	qm.stats.TotalQuarantined--
	qm.stats.QuarantineSize -= entry.FileSize
	qm.statsMu.Unlock()

	return nil
}

// DeleteEntry 删除隔离条目.
func (qm *QuarantineManager) DeleteEntry(entryID string) error {
	qm.entryMu.Lock()
	defer qm.entryMu.Unlock()

	entry, ok := qm.entries[entryID]
	if !ok {
		return ErrQuarantineEntryNotFound
	}

	// 删除隔离文件
	if err := os.Remove(entry.QuarantinePath); err != nil && !os.IsNotExist(err) {
		// 记录错误但继续
	}

	// 从清单中移除
	delete(qm.entries, entryID)

	// 保存清单
	if err := qm.saveManifest(); err != nil {
		// 记录错误但继续
	}

	// 更新统计
	qm.statsMu.Lock()
	qm.stats.TotalQuarantined--
	qm.stats.QuarantineSize -= entry.FileSize
	qm.statsMu.Unlock()

	return nil
}

// GetEntry 获取隔离条目.
func (qm *QuarantineManager) GetEntry(entryID string) (*QuarantineEntry, bool) {
	qm.entryMu.RLock()
	defer qm.entryMu.RUnlock()

	entry, ok := qm.entries[entryID]
	return entry, ok
}

// ListEntries 列出隔离条目.
func (qm *QuarantineManager) ListEntries(limit, offset int, threatLevel *ThreatLevel) []*QuarantineEntry {
	qm.entryMu.RLock()
	defer qm.entryMu.RUnlock()

	var entries []*QuarantineEntry
	for _, entry := range qm.entries {
		if threatLevel != nil && entry.ThreatLevel != *threatLevel {
			continue
		}
		entries = append(entries, entry)
	}

	// 按时间排序（最新的在前）
	sortEntriesByTime(entries)

	// 应用分页
	if offset >= len(entries) {
		return []*QuarantineEntry{}
	}

	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}

	return entries[offset:end]
}

// GetStats 获取统计信息.
func (qm *QuarantineManager) GetStats() map[string]interface{} {
	qm.entryMu.RLock()
	defer qm.entryMu.RUnlock()

	var totalSize int64
	var restoredCount int

	for _, entry := range qm.entries {
		if !entry.Restored {
			totalSize += entry.FileSize
		} else {
			restoredCount++
		}
	}

	return map[string]interface{}{
		"total_entries":    len(qm.entries),
		"active_entries":   len(qm.entries) - restoredCount,
		"restored_count":   restoredCount,
		"total_size":       totalSize,
		"max_size":         qm.config.MaxSize,
		"usage_percent":    float64(totalSize) / float64(qm.config.MaxSize) * 100,
		"quarantine_dir":   qm.config.QuarantineDir,
	}
}

// CleanupExpired 清理过期条目.
func (qm *QuarantineManager) CleanupExpired() int {
	if !qm.config.AutoDelete {
		return 0
	}

	qm.entryMu.Lock()
	defer qm.entryMu.Unlock()

	cutoff := time.Now().Add(-qm.config.MaxAge)
	var cleaned int

	for id, entry := range qm.entries {
		if entry.Timestamp.Before(cutoff) && !entry.Restored {
			// 删除隔离文件
			os.Remove(entry.QuarantinePath)
			delete(qm.entries, id)
			cleaned++
		}
	}

	if cleaned > 0 {
		qm.saveManifest()
	}

	return cleaned
}

// checkSpace 检查隔离区空间.
func (qm *QuarantineManager) checkSpace(additionalSize int64) error {
	qm.entryMu.RLock()
	var currentSize int64
	for _, entry := range qm.entries {
		if !entry.Restored {
			currentSize += entry.FileSize
		}
	}
	qm.entryMu.RUnlock()

	if currentSize+additionalSize > qm.config.MaxSize {
		return errors.New("隔离区空间不足")
	}

	return nil
}

// loadManifest 加载清单.
func (qm *QuarantineManager) loadManifest() error {
	data, err := os.ReadFile(qm.manifest)
	if err != nil {
		return err
	}

	var entries []*QuarantineEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	for _, entry := range entries {
		qm.entries[entry.ID] = entry
	}

	return nil
}

// saveManifest 保存清单.
func (qm *QuarantineManager) saveManifest() error {
	var entries []*QuarantineEntry
	for _, entry := range qm.entries {
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(qm.manifest, data, 0600)
}

// updateStats 更新统计.
func (qm *QuarantineManager) updateStats(size int64, add bool) {
	qm.statsMu.Lock()
	defer qm.statsMu.Unlock()

	if add {
		qm.stats.TotalQuarantined++
		qm.stats.QuarantineSize += size
	} else {
		qm.stats.TotalQuarantined--
		qm.stats.QuarantineSize -= size
	}
}

// VerifyIntegrity 验证隔离文件完整性.
func (qm *QuarantineManager) VerifyIntegrity(entryID string) (bool, error) {
	qm.entryMu.RLock()
	entry, ok := qm.entries[entryID]
	qm.entryMu.RUnlock()

	if !ok {
		return false, ErrQuarantineEntryNotFound
	}

	// 检查文件是否存在
	if _, err := os.Stat(entry.QuarantinePath); err != nil {
		return false, err
	}

	// 计算当前哈希
	currentHash, err := CalculateFileHash(entry.QuarantinePath)
	if err != nil {
		return false, err
	}

	// 比较哈希
	return currentHash == entry.FileHash, nil
}

// BatchQuarantine 批量隔离文件.
func (qm *QuarantineManager) BatchQuarantine(files []string, reason, detectionID string, threatLevel ThreatLevel, signatureName string) ([]*QuarantineEntry, []error) {
	var entries []*QuarantineEntry
	var errors []error

	for _, file := range files {
		entry, err := qm.QuarantineFile(file, reason, detectionID, threatLevel, signatureName)
		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", file, err))
			continue
		}
		entries = append(entries, entry)
	}

	return entries, errors
}

// Helper functions

func sortEntriesByTime(entries []*QuarantineEntry) {
	// 简单的冒泡排序（对于小量数据足够）
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Timestamp.Before(entries[j].Timestamp) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

// Error definitions
var ErrQuarantineEntryNotFound = errors.New("隔离条目不存在")