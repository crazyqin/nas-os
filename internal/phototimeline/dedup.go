// Package phototimeline provides photo timeline management for NAS-OS.
package phototimeline

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// DedupManager 去重管理器
type DedupManager struct {
	mu            sync.RWMutex
	photos        map[string]*Photo // 共享照片存储引用
	config        Config
	hashIndex     map[string][]string // hash -> photo IDs
}

// NewDedupManager 创建去重管理器
func NewDedupManager(config Config, photos map[string]*Photo) *DedupManager {
	return &DedupManager{
		photos:    photos,
		config:    config,
		hashIndex: make(map[string][]string),
	}
}

// ComputeFileHash 计算文件哈希
func (dm *DedupManager) ComputeFileHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ComputePerceptualHash 计算感知哈希 (简化版 - pHash)
// 实际实现需要图像处理库，这里提供框架
func (dm *DedupManager) ComputePerceptualHash(imageData []byte) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	// 简化的感知哈希实现
	// 实际应用中应该使用:
	// 1. 缩小图像到 32x32
	// 2. 转灰度
	// 3. DCT 变换
	// 4. 取左上 8x8
	// 5. 计算中位数生成 64 位哈希

	// 这里使用简化的哈希作为示例
	hash := sha256.Sum256(imageData)
	return hex.EncodeToString(hash[:16]), nil // 返回前 16 字节作为感知哈希
}

// IndexPhoto 索引照片用于去重
func (dm *DedupManager) IndexPhoto(photo *Photo) error {
	if photo == nil || photo.ID == "" {
		return fmt.Errorf("invalid photo")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 按文件哈希索引
	if photo.Hash != "" {
		dm.hashIndex[photo.Hash] = append(dm.hashIndex[photo.Hash], photo.ID)
	}

	// 按感知哈希索引
	if photo.PerceptualHash != "" {
		key := "phash:" + photo.PerceptualHash
		dm.hashIndex[key] = append(dm.hashIndex[key], photo.ID)
	}

	return nil
}

// FindDuplicates 查找重复照片
func (dm *DedupManager) FindDuplicates() []DuplicateGroup {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	groups := make([]DuplicateGroup, 0)
	processed := make(map[string]bool)

	// 查找完全相同的文件 (基于文件哈希)
	for hash, photoIDs := range dm.hashIndex {
		if len(hash) > 6 && hash[:6] == "phash:" {
			continue // 跳过感知哈希索引
		}
		if len(photoIDs) < 2 {
			continue
		}

		groupID := "dup:" + hash
		if len(hash) > 16 {
			groupID = "dup:" + hash[:16]
		}
		if processed[groupID] {
			continue
		}
		processed[groupID] = true

		photos := make([]Photo, 0, len(photoIDs))
		var totalSize int64
		var bestPhoto *Photo

		for _, pid := range photoIDs {
			if p, exists := dm.photos[pid]; exists {
				photos = append(photos, *p)
				totalSize += p.Size
				if bestPhoto == nil || p.Rating > bestPhoto.Rating {
					bestPhoto = p
				}
			}
		}

		if len(photos) >= 2 {
			group := DuplicateGroup{
				ID:         groupID,
				Photos:     photos,
				Hash:       hash,
				Similarity: 1.0, // 完全相同
				TotalSize:  totalSize,
				WastedSize: totalSize - photos[0].Size,
			}
			if bestPhoto != nil {
				group.Recommended = bestPhoto.ID
			}
			groups = append(groups, group)
		}
	}

	// 查找相似文件 (基于感知哈希)
	for key, photoIDs := range dm.hashIndex {
		if len(key) < 7 || key[:6] != "phash:" {
			continue
		}
		if len(photoIDs) < 2 {
			continue
		}

		phash := key[6:]
		groupID := "sim:" + phash[:16]
		if processed[groupID] {
			continue
		}
		processed[groupID] = true

		photos := make([]Photo, 0, len(photoIDs))
		var totalSize int64
		var bestPhoto *Photo

		for _, pid := range photoIDs {
			if p, exists := dm.photos[pid]; exists {
				photos = append(photos, *p)
				totalSize += p.Size
				if bestPhoto == nil || p.Rating > bestPhoto.Rating {
					bestPhoto = p
				}
			}
		}

		if len(photos) >= 2 {
			group := DuplicateGroup{
				ID:         groupID,
				Photos:     photos,
				Hash:       phash,
				Similarity: dm.config.SimilarityThreshold,
				TotalSize:  totalSize,
				WastedSize: totalSize - photos[0].Size,
			}
			if bestPhoto != nil {
				group.Recommended = bestPhoto.ID
			}
			groups = append(groups, group)
		}
	}

	return groups
}

// GetDedupStats 获取去重统计
func (dm *DedupManager) GetDedupStats() *DedupStats {
	duplicates := dm.FindDuplicates()

	stats := &DedupStats{
		TotalPhotos:     len(dm.photos),
		DuplicateGroups: len(duplicates),
	}

	for _, g := range duplicates {
		stats.DuplicatePhotos += len(g.Photos) - 1 // 每组减去保留的一张
		stats.WastedSpace += g.WastedSize
	}

	return stats
}

// RemoveDuplicates 删除重复照片
func (dm *DedupManager) RemoveDuplicates(groupID string, keepPhotoID string) error {
	// 先查找分组 (不持锁)
	duplicates := dm.findGroupByID(groupID)
	if duplicates == nil {
		return fmt.Errorf("duplicate group not found: %s", groupID)
	}

	// 验证保留的照片在分组中
	found := false
	for _, p := range duplicates.Photos {
		if p.ID == keepPhotoID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("photo %s not in duplicate group %s", keepPhotoID, groupID)
	}

	// 删除其他重复照片
	dm.mu.Lock()
	defer dm.mu.Unlock()
	for _, p := range duplicates.Photos {
		if p.ID != keepPhotoID {
			if photo, exists := dm.photos[p.ID]; exists {
				photo.Trashed = true
			}
		}
	}

	return nil
}

// findGroupByID 根据 ID 查找重复组
func (dm *DedupManager) findGroupByID(groupID string) *DuplicateGroup {
	duplicates := dm.FindDuplicates()
	for _, g := range duplicates {
		if g.ID == groupID {
			return &g
		}
	}
	return nil
}

// CalculateSimilarity 计算两个哈希的相似度 (汉明距离)
func CalculateSimilarity(hash1, hash2 string) float64 {
	if len(hash1) != len(hash2) {
		return 0
	}

	bits1 := hexToBits(hash1)
	bits2 := hexToBits(hash2)

	if len(bits1) != len(bits2) {
		return 0
	}

	matching := 0
	for i := 0; i < len(bits1); i++ {
		if bits1[i] == bits2[i] {
			matching++
		}
	}

	return float64(matching) / float64(len(bits1))
}

// hexToBits 将十六进制字符串转换为比特数组
func hexToBits(hexStr string) []byte {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil
	}

	bits := make([]byte, len(bytes)*8)
	for i, b := range bytes {
		for j := 0; j < 8; j++ {
			bits[i*8+j] = (b >> (7 - j)) & 1
		}
	}
	return bits
}
