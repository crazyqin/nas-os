package bitrotheal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NewHealEngine 创建自愈引擎.
func NewHealEngine(config *HealConfig) *HealEngine {
	if config == nil {
		config = DefaultConfig()
	}
	if config.Algorithm == "" {
		config.Algorithm = AlgorithmSHA256
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 4
	}
	return &HealEngine{
		config:   config,
		checksum: make(map[string]*ChecksumEntry),
		stopCh:   make(chan struct{}),
	}
}

// CalculateChecksum 计算数据的校验和.
func (h *HealEngine) CalculateChecksum(path string, data []byte) string {
	switch h.config.Algorithm {
	case AlgorithmCRC32:
		return fmt.Sprintf("%08x", crc32.ChecksumIEEE(data))
	case AlgorithmSHA256:
		fallthrough
	default:
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
}

// Verify 校验文件完整性.
// 返回 true 表示数据完整，false 表示数据已损坏.
func (h *HealEngine) Verify(path string, data []byte) (bool, error) {
	if path == "" {
		return false, ErrPathRequired
	}

	currentChecksum := h.CalculateChecksum(path, data)

	entry, err := h.GetChecksum(path)
	if err != nil {
		return false, err
	}

	if entry.Checksum != currentChecksum {
		return false, ErrChecksumMismatch
	}

	// 更新最后校验时间
	h.mu.Lock()
	entry.LastVerified = time.Now()
	h.mu.Unlock()

	return true, nil
}

// Repair 修复损坏的文件.
func (h *HealEngine) Repair(path string) (*RepairResult, error) {
	if path == "" {
		return nil, ErrPathRequired
	}

	start := time.Now()
	result := &RepairResult{
		Path: path,
	}

	// 尝试从冗余副本修复
	if len(h.config.ReplicaPaths) > 0 {
		for _, replicaDir := range h.config.ReplicaPaths {
			replicaPath := filepath.Join(replicaDir, filepath.Base(path))
			if h.tryRepairFromReplica(path, replicaPath) {
				result.Success = true
				result.Strategy = RepairFromReplica
				result.SourcePath = replicaPath
				result.Duration = time.Since(start)

				h.mu.Lock()
				if entry, ok := h.checksum[path]; ok {
					entry.RepairCount++
				}
				h.mu.Unlock()

				return result, nil
			}
		}
	}

	// 尝试从备份恢复
	if h.config.BackupRoot != "" {
		backupPath := filepath.Join(h.config.BackupRoot, filepath.Base(path))
		if h.tryRepairFromReplica(path, backupPath) {
			result.Success = true
			result.Strategy = RepairFromBackup
			result.SourcePath = backupPath
			result.Duration = time.Since(start)

			h.mu.Lock()
			if entry, ok := h.checksum[path]; ok {
				entry.RepairCount++
			}
			h.mu.Unlock()

			return result, nil
		}
	}

	// 修复失败
	result.Success = false
	result.Strategy = RepairManual
	result.Error = ErrNoRedundancy.Error()
	result.Duration = time.Since(start)
	return result, ErrRepairFailed
}

// Scan 扫描目录下的文件完整性.
func (h *HealEngine) Scan(root string) (*IntegrityReport, error) {
	if root == "" {
		return nil, ErrPathRequired
	}

	start := time.Now()
	report := &IntegrityReport{
		StartTime: start,
	}

	// 使用信号量控制并发
	sem := make(chan struct{}, h.config.MaxConcurrent)
	var mu sync.Mutex
	var wg sync.WaitGroup

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if info.IsDir() {
			return nil
		}

		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := os.ReadFile(filePath)
			if err != nil {
				return
			}

			mu.Lock()
			report.ScannedFiles++
			mu.Unlock()

			// 检查是否有已存储的校验和
			entry, err := h.GetChecksum(filePath)
			if err != nil {
				// 没有记录，计算并存储
				checksum := h.CalculateChecksum(filePath, data)
				newEntry := &ChecksumEntry{
					Path:         filePath,
					Algorithm:    h.config.Algorithm,
					Checksum:     checksum,
					LastVerified: time.Now(),
					FileSize:     info.Size(),
				}
				h.AddChecksum(newEntry)
				return
			}

			// 验证校验和
			currentChecksum := h.CalculateChecksum(filePath, data)
			if entry.Checksum != currentChecksum {
				mu.Lock()
				report.CorruptedFiles++
				report.CorruptedPaths = append(report.CorruptedPaths, filePath)
				mu.Unlock()

				// 尝试自动修复
				if h.config.AutoRepair {
					result, err := h.Repair(filePath)
					if err == nil && result.Success {
						mu.Lock()
						report.RepairedFiles++
						mu.Unlock()
					} else {
						mu.Lock()
						report.UnrepairableFiles++
						report.UnrepairablePaths = append(report.UnrepairablePaths, filePath)
						mu.Unlock()
					}
				} else {
					mu.Lock()
					report.UnrepairableFiles++
					report.UnrepairablePaths = append(report.UnrepairablePaths, filePath)
					mu.Unlock()
				}
			} else {
				// 更新校验时间
				h.mu.Lock()
				entry.LastVerified = time.Now()
				h.mu.Unlock()
			}
		}(path)

		return nil
	})

	wg.Wait()
	report.ScanDuration = time.Since(start)

	return report, err
}

// AddChecksum 添加或更新校验和记录.
func (h *HealEngine) AddChecksum(entry *ChecksumEntry) error {
	if entry == nil || entry.Path == "" {
		return ErrPathRequired
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.checksum[entry.Path] = entry
	return nil
}

// GetChecksum 获取文件的校验和记录.
func (h *HealEngine) GetChecksum(path string) (*ChecksumEntry, error) {
	if path == "" {
		return nil, ErrPathRequired
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	entry, ok := h.checksum[path]
	if !ok {
		return nil, ErrChecksumNotFound
	}
	return entry, nil
}

// Stop 停止引擎.
func (h *HealEngine) Stop() {
	close(h.stopCh)
}

// tryRepairFromReplica 尝试从副本路径修复.
func (h *HealEngine) tryRepairFromReplica(targetPath, sourcePath string) bool {
	// 检查源文件是否存在
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false
	}

	// 读取源文件数据
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return false
	}

	// 如果有校验和记录，验证源文件完整性
	entry, err := h.GetChecksum(targetPath)
	if err == nil {
		sourceChecksum := h.CalculateChecksum(sourcePath, sourceData)
		if entry.Checksum != sourceChecksum {
			return false // 源文件也不完整
		}
	}

	// 复制文件
	if err := os.WriteFile(targetPath, sourceData, sourceInfo.Mode()); err != nil {
		return false
	}

	return true
}
