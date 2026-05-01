// Package drivesync 提供冲突检测和解决
package drivesync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ConflictDetector 冲突检测器.
type ConflictDetector struct {
	mu       sync.RWMutex
	strategy ConflictResolution
}

// NewConflictDetector 创建冲突检测器.
func NewConflictDetector(strategy ConflictResolution) *ConflictDetector {
	if strategy == "" {
		strategy = ConflictNewerWins
	}
	return &ConflictDetector{
		strategy: strategy,
	}
}

// DetectConflict 检测文件冲突.
// 基于时间戳和哈希值检测：如果两端都修改了文件（校验和不同），则存在冲突.
func (cd *ConflictDetector) DetectConflict(localPath, remotePath string, localModTime, remoteModTime time.Time, localDeviceID, remoteDeviceID string) (*FileConflict, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	// 计算本地文件校验和
	localChecksum, err := computeFileChecksum(localPath)
	if err != nil {
		return nil, fmt.Errorf("计算本地文件校验和失败: %w", err)
	}

	// 获取本地文件大小
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("获取本地文件信息失败: %w", err)
	}

	// 远程文件信息由调用方提供，这里用本地信息作为模拟
	remoteChecksum := localChecksum // 实际应从远程获取
	var remoteSize int64

	// 判断是否冲突：校验和不同且双方都有修改
	if localChecksum == remoteChecksum {
		return nil, nil // 无冲突
	}

	// 获取远程文件大小（如果远程文件存在）
	remoteInfo, err := os.Stat(remotePath)
	if err == nil {
		remoteSize = remoteInfo.Size()
		// 重新计算远程校验和
		if rcs, err := computeFileChecksum(remotePath); err == nil {
			remoteChecksum = rcs
		}
	}

	// 校验和相同则无冲突
	if localChecksum == remoteChecksum {
		return nil, nil
	}

	conflict := &FileConflict{
		ID:             generateID(),
		FilePath:       localPath,
		LocalChecksum:  localChecksum,
		RemoteChecksum: remoteChecksum,
		LocalModTime:   localModTime,
		RemoteModTime:  remoteModTime,
		LocalSize:      localInfo.Size(),
		RemoteSize:     remoteSize,
		LocalDeviceID:  localDeviceID,
		RemoteDeviceID: remoteDeviceID,
		Status:         ConflictStatusPending,
		CreatedAt:      time.Now(),
	}

	return conflict, nil
}

// ResolveConflict 根据策略自动解决冲突.
func (cd *ConflictDetector) ResolveConflict(conflict *FileConflict) (ConflictResolution, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	switch cd.strategy {
	case ConflictKeepLocal:
		return ConflictKeepLocal, nil
	case ConflictKeepRemote:
		return ConflictKeepRemote, nil
	case ConflictKeepBoth:
		return ConflictKeepBoth, nil
	case ConflictNewerWins:
		if conflict.LocalModTime.After(conflict.RemoteModTime) {
			return ConflictKeepLocal, nil
		}
		return ConflictKeepRemote, nil
	case ConflictManualMerge:
		return ConflictManualMerge, nil
	default:
		return ConflictNewerWins, nil
	}
}

// RenameConflictFile 为冲突文件生成重命名路径.
// 保留双方文件时，在文件名后添加冲突标记.
func RenameConflictFile(filePath string) string {
	ext := ""
	base := filePath
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '.' {
			ext = filePath[i:]
			base = filePath[:i]
			break
		}
	}

	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s (conflict %s)%s", base, timestamp, ext)
}

// computeFileChecksum 计算文件 SHA256 校验和.
func computeFileChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetStrategy 获取冲突解决策略.
func (cd *ConflictDetector) GetStrategy() ConflictResolution {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	return cd.strategy
}

// SetStrategy 设置冲突解决策略.
func (cd *ConflictDetector) SetStrategy(strategy ConflictResolution) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.strategy = strategy
}
