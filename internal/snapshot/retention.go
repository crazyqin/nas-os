package snapshot

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"
)

// RetentionCleaner 保留策略清理器
type RetentionCleaner struct {
	storageMgr StorageManager
}

// NewRetentionCleaner 创建清理器
func NewRetentionCleaner(storageMgr StorageManager) *RetentionCleaner {
	return &RetentionCleaner{
		storageMgr: storageMgr,
	}
}

// Info 快照信息（用于清理）
type Info struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
}

// SnapshotInfo 是 Info 的别名，保持向后兼容
// Deprecated: Use Info instead
type SnapshotInfo = Info //nolint:revive // 向后兼容别名

// Clean 执行清理
func (c *RetentionCleaner) Clean(policy *Policy) ([]string, error) {
	if policy.Retention == nil {
		return nil, nil
	}

	// 获取该策略相关的快照列表
	snapshots, err := c.listPolicySnapshots(policy)
	if err != nil {
		return nil, fmt.Errorf("获取快照列表失败: %w", err)
	}

	var toDelete []Info

	switch policy.Retention.Type {
	case RetentionByCount:
		toDelete = c.cleanByCount(snapshots, policy.Retention.MaxCount)

	case RetentionByAge:
		toDelete = c.cleanByAge(snapshots, policy.Retention.MaxAgeDays)

	case RetentionBySize:
		toDelete = c.cleanBySize(snapshots, policy.Retention.MaxSizeBytes)

	case RetentionCombined:
		toDelete = c.cleanCombined(snapshots, policy.Retention)
	}

	// 执行删除
	var deleted []string
	for _, snap := range toDelete {
		if err := c.deleteSnapshot(policy, snap); err != nil {
			log.Printf("删除快照 %s 失败: %v", snap.Name, err)
			continue
		}
		deleted = append(deleted, snap.Name)
	}

	return deleted, nil
}

// listPolicySnapshots 获取策略相关的快照列表
func (c *RetentionCleaner) listPolicySnapshots(policy *Policy) ([]Info, error) {
	// 检查存储管理器是否存在
	if c.storageMgr == nil {
		return []Info{}, nil
	}

	// 通过存储管理器获取快照列表
	snapshots, err := c.storageMgr.ListSnapshots(policy.VolumeName)
	if err != nil {
		return nil, err
	}

	var result []Info

	// 过滤属于该策略的快照
	prefix := policy.SnapshotPrefix
	if prefix == "" {
		prefix = ""
	}

	for _, snap := range snapshots {
		// 类型断言获取快照信息
		// 这里需要根据实际的存储管理器返回类型处理
		if si, ok := snap.(Info); ok {
			// 检查是否属于该策略
			if prefix != "" && !hasPrefix(si.Name, prefix) {
				continue
			}
			result = append(result, si)
		}
	}

	// 按创建时间排序（旧 -> 新）
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
}

// hasPrefix 检查名称是否有前缀
func hasPrefix(name, prefix string) bool {
	return len(name) >= len(prefix) && name[:len(prefix)] == prefix
}

// cleanByCount 按数量清理
func (c *RetentionCleaner) cleanByCount(snapshots []Info, maxCount int) []Info {
	if maxCount <= 0 || len(snapshots) <= maxCount {
		return nil
	}

	// 删除最旧的快照，保留最新的 maxCount 个
	return snapshots[:len(snapshots)-maxCount]
}

// cleanByAge 按时间清理
func (c *RetentionCleaner) cleanByAge(snapshots []Info, maxAgeDays int) []Info {
	if maxAgeDays <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	var toDelete []Info

	for _, snap := range snapshots {
		if snap.CreatedAt.Before(cutoff) {
			toDelete = append(toDelete, snap)
		}
	}

	return toDelete
}

// cleanBySize 按大小清理
func (c *RetentionCleaner) cleanBySize(snapshots []Info, maxSizeBytes int64) []Info {
	if maxSizeBytes <= 0 {
		return nil
	}

	// 计算总大小
	var totalSize int64
	for _, snap := range snapshots {
		totalSize += snap.Size
	}

	if totalSize <= maxSizeBytes {
		return nil
	}

	var toDelete []Info
	var deletedSize int64

	// 从最旧的开始删除，直到总大小符合要求
	for _, snap := range snapshots {
		if totalSize-deletedSize <= maxSizeBytes {
			break
		}
		toDelete = append(toDelete, snap)
		deletedSize += snap.Size
	}

	return toDelete
}

// cleanCombined 组合清理策略
func (c *RetentionCleaner) cleanCombined(snapshots []Info, policy *RetentionPolicy) []Info {
	var toDelete []Info
	deletedSet := make(map[string]bool)

	// 按数量清理
	if policy.CountPolicy != nil && policy.CountPolicy.MaxCount > 0 {
		countDeletes := c.cleanByCount(snapshots, policy.CountPolicy.MaxCount)
		for _, snap := range countDeletes {
			if !deletedSet[snap.Name] {
				toDelete = append(toDelete, snap)
				deletedSet[snap.Name] = true
			}
		}
	}

	// 按时间清理
	if policy.AgePolicy != nil && policy.AgePolicy.MaxAgeDays > 0 {
		ageDeletes := c.cleanByAge(snapshots, policy.AgePolicy.MaxAgeDays)
		for _, snap := range ageDeletes {
			if !deletedSet[snap.Name] {
				toDelete = append(toDelete, snap)
				deletedSet[snap.Name] = true
			}
		}
	}

	// 按大小清理
	if policy.SizePolicy != nil && policy.SizePolicy.MaxSizeBytes > 0 {
		// 重新计算剩余快照
		var remaining []Info
		for _, snap := range snapshots {
			if !deletedSet[snap.Name] {
				remaining = append(remaining, snap)
			}
		}
		sizeDeletes := c.cleanBySize(remaining, policy.SizePolicy.MaxSizeBytes)
		for _, snap := range sizeDeletes {
			if !deletedSet[snap.Name] {
				toDelete = append(toDelete, snap)
				deletedSet[snap.Name] = true
			}
		}
	}

	return toDelete
}

// deleteSnapshot 删除快照
func (c *RetentionCleaner) deleteSnapshot(policy *Policy, snap Info) error {
	return c.storageMgr.DeleteSnapshot(policy.VolumeName, snap.Name)
}

// PreviewDryRun 预览清理（不实际删除）
func (c *RetentionCleaner) PreviewDryRun(policy *Policy) (*CleanupPreview, error) {
	snapshots, err := c.listPolicySnapshots(policy)
	if err != nil {
		return nil, err
	}

	var toDelete []Info
	switch policy.Retention.Type {
	case RetentionByCount:
		toDelete = c.cleanByCount(snapshots, policy.Retention.MaxCount)
	case RetentionByAge:
		toDelete = c.cleanByAge(snapshots, policy.Retention.MaxAgeDays)
	case RetentionBySize:
		toDelete = c.cleanBySize(snapshots, policy.Retention.MaxSizeBytes)
	case RetentionCombined:
		toDelete = c.cleanCombined(snapshots, policy.Retention)
	}

	preview := &CleanupPreview{
		TotalSnapshots: len(snapshots),
		ToDelete:       len(toDelete),
		ToKeep:         len(snapshots) - len(toDelete),
		Snapshots:      toDelete,
	}

	// 计算可回收空间
	for _, snap := range toDelete {
		preview.ReclaimableBytes += snap.Size
	}

	return preview, nil
}

// CleanupPreview 清理预览结果，展示将要删除的快照信息
type CleanupPreview struct {
	TotalSnapshots   int    `json:"totalSnapshots"`
	ToDelete         int    `json:"toDelete"`
	ToKeep           int    `json:"toKeep"`
	ReclaimableBytes int64  `json:"reclaimableBytes"`
	Snapshots        []Info `json:"snapshots,omitempty"`
}

// EstimateRetention 估算保留策略效果
func (c *RetentionCleaner) EstimateRetention(policy *Policy, snapshotCount int, avgSize int64) *RetentionEstimate {
	estimate := &RetentionEstimate{
		PolicyType: policy.Retention.Type,
	}

	switch policy.Retention.Type {
	case RetentionByCount:
		estimate.MaxSnapshots = policy.Retention.MaxCount
		estimate.EstimatedStorage = int64(policy.Retention.MaxCount) * avgSize

	case RetentionByAge:
		// 假设每天创建一个快照
		estimate.MaxSnapshots = policy.Retention.MaxAgeDays
		estimate.EstimatedStorage = int64(policy.Retention.MaxAgeDays) * avgSize

	case RetentionBySize:
		estimate.EstimatedStorage = policy.Retention.MaxSizeBytes
		if avgSize > 0 {
			// 安全转换：int64 到 int 可能溢出
			maxSnaps := policy.Retention.MaxSizeBytes / avgSize
			if maxSnaps > int64(math.MaxInt) {
				estimate.MaxSnapshots = math.MaxInt
			} else {
				estimate.MaxSnapshots = int(maxSnaps)
			}
		}

	case RetentionCombined:
		// 取最严格的限制
		if policy.Retention.CountPolicy != nil {
			estimate.MaxSnapshots = policy.Retention.CountPolicy.MaxCount
		}
		if policy.Retention.SizePolicy != nil {
			estimate.EstimatedStorage = policy.Retention.SizePolicy.MaxSizeBytes
		}
	}

	return estimate
}

// RetentionEstimate 保留策略估算结果，预测策略对存储的影响
type RetentionEstimate struct {
	PolicyType       RetentionPolicyType `json:"policyType"`
	MaxSnapshots     int                 `json:"maxSnapshots"`
	EstimatedStorage int64               `json:"estimatedStorage"`
	Warnings         []string            `json:"warnings,omitempty"`
}
