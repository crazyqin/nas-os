package smartdatalifecycle

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"hash"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"go.uber.org/zap"
)

// Deduplicator 重复数据检测器
// 检测和清理重复数据，节省存储空间
type Deduplicator struct {
	config  DedupConfig
	manager *Manager
	logger  *zap.Logger

	// 内容哈希索引
	hashIndex sync.Map // map[string][]*DataItem
}

// NewDeduplicator 创建重复数据检测器
func NewDeduplicator(config DedupConfig, manager *Manager, logger *zap.Logger) *Deduplicator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Deduplicator{
		config:  config,
		manager: manager,
		logger:  logger,
	}
}

// Run 执行重复数据检测
func (d *Deduplicator) Run(ctx context.Context) error {
	d.logger.Info("deduplication check started")

	result, err := d.Scan(ctx)
	if err != nil {
		return err
	}

	d.logger.Info("deduplication check completed",
		zap.Int("scanned_items", result.ScannedItems),
		zap.Int("duplicate_groups", result.DuplicateGroups),
		zap.Int64("wasted_space", result.WastedSpace))

	// 自动清理
	if d.config.AutoCleanup && result.WastedSpace > 0 {
		reclaimed, err := d.AutoCleanupDuplicates(ctx)
		if err != nil {
			d.logger.Error("auto cleanup failed", zap.Error(err))
		} else {
			d.logger.Info("auto cleanup completed",
				zap.Int64("reclaimed_space", reclaimed))
		}
	}

	return nil
}

// Scan 扫描重复数据
func (d *Deduplicator) Scan(ctx context.Context) (*DeduplicationResult, error) {
	result := &DeduplicationResult{
		ProcessedAt: time.Now(),
		Errors:      make([]string, 0),
	}

	// 清空索引
	d.hashIndex = sync.Map{}

	// 获取所有数据项
	items := d.manager.ListItems("", 0, 0)

	for _, item := range items {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		// 跳过小文件
		if d.config.MinFileSizeBytes > 0 && item.Size < d.config.MinFileSizeBytes {
			continue
		}

		result.ScannedItems++

		// 计算或使用现有哈希
		hash := item.ContentHash
		if hash == "" {
			// 在实际实现中，这里应该读取文件内容计算哈希
			// 这里使用模拟的哈希
			hash = d.calculateHash(item)
		}

		// 添加到索引
		if existing, ok := d.hashIndex.Load(hash); ok {
			items := existing.([]*DataItem)
			items = append(items, item)
			d.hashIndex.Store(hash, items)
		} else {
			d.hashIndex.Store(hash, []*DataItem{item})
		}
	}

	// 统计重复组
	d.hashIndex.Range(func(key, value interface{}) bool {
		items := value.([]*DataItem)
		if len(items) > 1 {
			result.DuplicateGroups++
			result.TotalDuplicates += len(items) - 1

			// 计算浪费的空间
			for i := 1; i < len(items); i++ {
				result.WastedSpace += items[i].Size
			}
		}
		return true
	})

	return result, nil
}

// calculateHash 计算数据项哈希
func (d *Deduplicator) calculateHash(item *DataItem) string {
	// 在实际实现中，应该读取文件内容计算哈希
	// 这里使用文件大小和路径作为模拟
	var h hash.Hash

	switch d.config.Algorithm {
	case "md5":
		h = md5.New()
	case "sha256":
		h = sha256.New()
	default:
		h = xxhash.New()
	}

	// 模拟哈希计算
	data := fmt.Sprintf("%s:%d", item.Path, item.Size)
	h.Write([]byte(data))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// GetDuplicateGroups 获取重复数据组
func (d *Deduplicator) GetDuplicateGroups() []*DuplicateGroup {
	groups := make([]*DuplicateGroup, 0)

	d.hashIndex.Range(func(key, value interface{}) bool {
		items := value.([]*DataItem)
		if len(items) > 1 {
			group := &DuplicateGroup{
				ID:          fmt.Sprintf("dup-%s", key),
				ContentHash: key.(string),
				Items:       items,
				LastChecked: time.Now(),
			}

			// 计算总大小和浪费空间
			for i, item := range items {
				group.TotalSize += item.Size
				if i > 0 {
					group.WastedSize += item.Size
				}
			}

			// 设置首次发现时间
			if items[0].CreatedAt.Before(items[1].CreatedAt) {
				group.FirstFound = items[0].CreatedAt
			} else {
				group.FirstFound = items[1].CreatedAt
			}

			groups = append(groups, group)
		}
		return true
	})

	return groups
}

// CleanupDuplicates 清理重复数据（保留最旧的副本）
func (d *Deduplicator) CleanupDuplicates(ctx context.Context, groupID string) (int64, error) {
	reclaimed := int64(0)

	// 查找重复组
	var targetGroup *DuplicateGroup
	d.hashIndex.Range(func(key, value interface{}) bool {
		items := value.([]*DataItem)
		if len(items) > 1 {
			group := &DuplicateGroup{
				ID:          fmt.Sprintf("dup-%s", key),
				ContentHash: key.(string),
				Items:       items,
			}
			if group.ID == groupID {
				targetGroup = group
				return false
			}
		}
		return true
	})

	if targetGroup == nil {
		return 0, fmt.Errorf("duplicate group not found: %s", groupID)
	}

	// 保留最旧的副本（第一个），删除其他
	for i := 1; i < len(targetGroup.Items); i++ {
		if ctx.Err() != nil {
			return reclaimed, ctx.Err()
		}

		item := targetGroup.Items[i]

		// 法律冻结检查
		if item.LegalHold {
			d.logger.Info("skipping legally held duplicate",
				zap.String("item_id", item.ID))
			continue
		}

		if d.manager.config.DryRun {
			d.logger.Info("dry run: would delete duplicate",
				zap.String("item_id", item.ID),
				zap.String("path", item.Path))
			reclaimed += item.Size
			continue
		}

		if err := d.manager.UpdateItemStage(item.ID, StageDeleted, "dedup"); err != nil {
			d.logger.Error("failed to delete duplicate",
				zap.String("item_id", item.ID),
				zap.Error(err))
			continue
		}

		reclaimed += item.Size
		d.logger.Info("duplicate deleted",
			zap.String("item_id", item.ID),
			zap.String("path", item.Path),
			zap.Int64("size", item.Size))
	}

	return reclaimed, nil
}

// AutoCleanupDuplicates 自动清理所有重复数据
func (d *Deduplicator) AutoCleanupDuplicates(ctx context.Context) (int64, error) {
	totalReclaimed := int64(0)

	d.hashIndex.Range(func(key, value interface{}) bool {
		if ctx.Err() != nil {
			return false
		}

		items := value.([]*DataItem)
		if len(items) <= 1 {
			return true
		}

		// 按创建时间排序，保留最旧的
		oldestIdx := 0
		for i, item := range items {
			if item.CreatedAt.Before(items[oldestIdx].CreatedAt) {
				oldestIdx = i
			}
		}

		// 删除其他副本
		for i, item := range items {
			if i == oldestIdx {
				continue
			}

			if item.LegalHold {
				continue
			}

			if err := d.manager.UpdateItemStage(item.ID, StageDeleted, "dedup"); err != nil {
				d.logger.Error("failed to delete duplicate",
					zap.String("item_id", item.ID),
					zap.Error(err))
				continue
			}

			totalReclaimed += item.Size
		}

		return true
	})

	return totalReclaimed, nil
}

// GetDedupStats 获取去重统计
func (d *Deduplicator) GetDedupStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_groups":      0,
		"total_duplicates":  0,
		"total_wasted":      int64(0),
		"potential_savings": int64(0),
	}

	d.hashIndex.Range(func(key, value interface{}) bool {
		items := value.([]*DataItem)
		if len(items) > 1 {
			stats["total_groups"] = stats["total_groups"].(int) + 1
			stats["total_duplicates"] = stats["total_duplicates"].(int) + (len(items) - 1)

			for i := 1; i < len(items); i++ {
				wasted := items[i].Size
				stats["total_wasted"] = stats["total_wasted"].(int64) + wasted
				stats["potential_savings"] = stats["potential_savings"].(int64) + wasted
			}
		}
		return true
	})

	return stats
}

// UpdateContentHash 更新数据项的内容哈希
func (d *Deduplicator) UpdateContentHash(itemID string, content []byte) error {
	d.manager.mu.Lock()
	defer d.manager.mu.Unlock()

	item, ok := d.manager.dataItems[itemID]
	if !ok {
		return fmt.Errorf("item not found: %s", itemID)
	}

	var h hash.Hash
	switch d.config.Algorithm {
	case "md5":
		h = md5.New()
	case "sha256":
		h = sha256.New()
	default:
		h = xxhash.New()
	}

	h.Write(content)
	item.ContentHash = fmt.Sprintf("%x", h.Sum(nil))

	return nil
}
