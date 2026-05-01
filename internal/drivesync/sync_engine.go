// Package drivesync 提供增量同步引擎
package drivesync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
)

// 默认块大小 64KB.
const defaultBlockSize = 64 * 1024

// SyncEngine 增量同步引擎（基于 rsync 算法的块级差异检测）.
type SyncEngine struct {
	mu        sync.RWMutex
	blockSize int // 块大小（字节）
}

// NewSyncEngine 创建增量同步引擎.
func NewSyncEngine(blockSize int) *SyncEngine {
	if blockSize <= 0 {
		blockSize = defaultBlockSize
	}
	return &SyncEngine{
		blockSize: blockSize,
	}
}

// ComputeBlockChecksums 计算文件的块级校验和.
func (e *SyncEngine) ComputeBlockChecksums(filePath string) ([]BlockInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	var blocks []BlockInfo
	buf := make([]byte, e.blockSize)
	index := 0
	offset := int64(0)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			h := sha256.New()
			h.Write(buf[:n])
			checksum := hex.EncodeToString(h.Sum(nil))

			blocks = append(blocks, BlockInfo{
				Index:    index,
				Offset:   offset,
				Size:     n,
				Checksum: checksum,
			})

			offset += int64(n)
			index++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
	}

	return blocks, nil
}

// ComputeFileChecksum 计算整个文件的 SHA256 校验和.
func (e *SyncEngine) ComputeFileChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("计算校验和失败: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeDelta 计算两个文件之间的增量差异.
func (e *SyncEngine) ComputeDelta(localBlocks []BlockInfo, remoteBlocks []BlockInfo) *DeltaSyncResponse {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 构建远程块的校验和索引
	remoteIndex := make(map[string]int, len(remoteBlocks))
	for _, block := range remoteBlocks {
		remoteIndex[block.Checksum] = block.Index
	}

	// 找出匹配的块
	var matchedBlocks []int
	var newBlocks []BlockInfo

	for _, localBlock := range localBlocks {
		if remoteIdx, exists := remoteIndex[localBlock.Checksum]; exists {
			matchedBlocks = append(matchedBlocks, remoteIdx)
		} else {
			newBlocks = append(newBlocks, localBlock)
		}
	}

	// 计算节省的字节数
	var savedBytes int64
	for _, idx := range matchedBlocks {
		if idx < len(remoteBlocks) {
			savedBytes += int64(remoteBlocks[idx].Size)
		}
	}

	return &DeltaSyncResponse{
		NeedFullSync:  false,
		MatchedBlocks: matchedBlocks,
		NewBlocks:     newBlocks,
		TotalBlocks:   len(localBlocks),
		SavedBytes:    savedBytes,
	}
}

// ShouldFullSync 判断是否需要全量同步.
func (e *SyncEngine) ShouldFullSync(localSize int64, localChecksum string, remoteSize int64, remoteChecksum string) bool {
	// 文件大小相同且校验和相同，无需同步
	if localSize == remoteSize && localChecksum == remoteChecksum {
		return false
	}

	// 文件大小差异过大（超过50%），直接全量同步
	if localSize == 0 || remoteSize == 0 {
		return true
	}

	ratio := float64(localSize) / float64(remoteSize)
	if ratio < 0.5 || ratio > 2.0 {
		return true
	}

	return false
}

// BlockSize 返回块大小.
func (e *SyncEngine) BlockSize() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.blockSize
}
