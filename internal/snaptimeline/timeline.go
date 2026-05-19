// Package snaptimeline 存储快照可视化时间线
// 对标TrueNAS快照管理，提供时间线视图、对比、一键回滚
package snaptimeline

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SnapshotTimeline 快照时间线管理器
type SnapshotTimeline struct {
	mu       sync.RWMutex
	snaps    map[string]*SnapshotEntry // id -> entry
	logger   *zap.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// SnapshotEntry 快照条目
type SnapshotEntry struct {
	ID          string            `json:"id"`
	VolumeName  string            `json:"volumeName"`
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Size        int64             `json:"size"`
	ModTime     time.Time         `json:"modTime"`
	CreatedAt   time.Time         `json:"createdAt"`
	Tags        []string          `json:"tags,omitempty"`
	ParentID    string            `json:"parentId,omitempty"`
	ChildrenIDs []string          `json:"childrenIds,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	IsProtected bool              `json:"isProtected"`
	ExpiresAt   *time.Time        `json:"expiresAt,omitempty"`
	Status      string            `json:"status"` // active, locked, expired, deleted
}

// TimelineRequest 时间线查询请求
type TimelineRequest struct {
	VolumeName string     `json:"volumeName,omitempty"`
	From       *time.Time `json:"from,omitempty"`
	To         *time.Time `json:"to,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	MaxResults int        `json:"maxResults"`
	Offset     int        `json:"offset"`
}

// TimelineResponse 时间线查询响应
type TimelineResponse struct {
	Total   int              `json:"total"`
	Snaps   []*SnapshotEntry `json:"snaps"`
	Took    time.Duration    `json:"took"`
}

// CompareRequest 快照对比请求
type CompareRequest struct {
	SnapID1 string `json:"snapId1"`
	SnapID2 string `json:"snapId2"`
}

// CompareResult 快照对比结果
type CompareResult struct {
	Snap1        *SnapshotEntry     `json:"snap1"`
	Snap2        *SnapshotEntry     `json:"snap2"`
	DiffFiles    int                `json:"diffFiles"`
	AddedFiles   int                `json:"addedFiles"`
	RemovedFiles int                `json:"removedFiles"`
	SizeDelta    int64              `json:"sizeDelta"`
	Details      []*FileDiff        `json:"details,omitempty"`
}

// FileDiff 文件差异
type FileDiff struct {
	Path      string    `json:"path"`
	Type      string    `json:"type"` // added, removed, modified
	Size1     int64     `json:"size1,omitempty"`
	Size2     int64     `json:"size2,omitempty"`
	ModTime1  time.Time `json:"modTime1,omitempty"`
	ModTime2  time.Time `json:"modTime2,omitempty"`
}

// NewSnapshotTimeline 创建快照时间线管理器
func NewSnapshotTimeline(logger *zap.Logger) *SnapshotTimeline {
	ctx, cancel := context.WithCancel(context.Background())
	return &SnapshotTimeline{
		snaps:  make(map[string]*SnapshotEntry),
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddSnapshot 添加快照
func (st *SnapshotTimeline) AddSnapshot(entry *SnapshotEntry) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.Status == "" {
		entry.Status = "active"
	}
	st.snaps[entry.ID] = entry

	// 建立父子关系
	if entry.ParentID != "" {
		if parent, ok := st.snaps[entry.ParentID]; ok {
			parent.ChildrenIDs = append(parent.ChildrenIDs, entry.ID)
		}
	}

	st.logger.Info("快照已添加", zap.String("id", entry.ID), zap.String("volume", entry.VolumeName))
}

// RemoveSnapshot 移除快照
func (st *SnapshotTimeline) RemoveSnapshot(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	snap, ok := st.snaps[id]
	if !ok {
		return fmt.Errorf("快照不存在: %s", id)
	}

	if snap.IsProtected {
		return fmt.Errorf("快照已锁定，无法删除: %s", id)
	}

	// 断开父子关系
	if snap.ParentID != "" {
		if parent, ok := st.snaps[snap.ParentID]; ok {
			for i, childID := range parent.ChildrenIDs {
				if childID == id {
					parent.ChildrenIDs = append(parent.ChildrenIDs[:i], parent.ChildrenIDs[i+1:]...)
					break
				}
			}
		}
	}

	// 将子快照的ParentID指向祖父
	for _, childID := range snap.ChildrenIDs {
		if child, ok := st.snaps[childID]; ok {
			child.ParentID = snap.ParentID
			if snap.ParentID != "" {
				if grandparent, ok := st.snaps[snap.ParentID]; ok {
					grandparent.ChildrenIDs = append(grandparent.ChildrenIDs, childID)
				}
			}
		}
	}

	delete(st.snaps, id)
	st.logger.Info("快照已删除", zap.String("id", id))
	return nil
}

// GetTimeline 获取时间线
func (st *SnapshotTimeline) GetTimeline(req TimelineRequest) *TimelineResponse {
	startTime := time.Now()

	st.mu.RLock()
	defer st.mu.RUnlock()

	if req.MaxResults == 0 {
		req.MaxResults = 50
	}

	// 过滤
	filtered := make([]*SnapshotEntry, 0)
	for _, snap := range st.snaps {
		if req.VolumeName != "" && snap.VolumeName != req.VolumeName {
			continue
		}
		if req.From != nil && snap.CreatedAt.Before(*req.From) {
			continue
		}
		if req.To != nil && snap.CreatedAt.After(*req.To) {
			continue
		}
		if len(req.Tags) > 0 {
			hasTag := false
			for _, tag := range req.Tags {
				for _, snapTag := range snap.Tags {
					if tag == snapTag {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		filtered = append(filtered, snap)
	}

	// 按创建时间降序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	// 分页
	total := len(filtered)
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + req.MaxResults
	if end > total {
		end = total
	}

	return &TimelineResponse{
		Total: total,
		Snaps: filtered[start:end],
		Took:  time.Since(startTime),
	}
}

// CompareSnapshots 对比两个快照
func (st *SnapshotTimeline) CompareSnapshots(req CompareRequest) (*CompareResult, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	snap1, ok := st.snaps[req.SnapID1]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", req.SnapID1)
	}
	snap2, ok := st.snaps[req.SnapID2]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", req.SnapID2)
	}

	result := &CompareResult{
		Snap1:     snap1,
		Snap2:     snap2,
		SizeDelta: snap2.Size - snap1.Size,
	}

	// 简化对比：基于元数据差异
	if snap1.VolumeName != snap2.VolumeName {
		result.DiffFiles = -1 // 不同卷无法对比
	}

	return result, nil
}

// ProtectSnapshot 锁定快照（防删除）
func (st *SnapshotTimeline) ProtectSnapshot(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	snap, ok := st.snaps[id]
	if !ok {
		return fmt.Errorf("快照不存在: %s", id)
	}

	snap.IsProtected = true
	snap.Status = "locked"
	st.logger.Info("快照已锁定", zap.String("id", id))
	return nil
}

// UnprotectSnapshot 解锁快照
func (st *SnapshotTimeline) UnprotectSnapshot(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	snap, ok := st.snaps[id]
	if !ok {
		return fmt.Errorf("快照不存在: %s", id)
	}

	snap.IsProtected = false
	snap.Status = "active"
	st.logger.Info("快照已解锁", zap.String("id", id))
	return nil
}

// GetSnapshot 获取快照详情
func (st *SnapshotTimeline) GetSnapshot(id string) (*SnapshotEntry, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	snap, ok := st.snaps[id]
	if !ok {
		return nil, fmt.Errorf("快照不存在: %s", id)
	}
	return snap, nil
}

// ListVolumes 列出所有有快照的卷
func (st *SnapshotTimeline) ListVolumes() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	volumes := make(map[string]bool)
	for _, snap := range st.snaps {
		volumes[snap.VolumeName] = true
	}

	result := make([]string, 0, len(volumes))
	for vol := range volumes {
		result = append(result, vol)
	}
	sort.Strings(result)
	return result
}

// GetStats 获取统计信息
func (st *SnapshotTimeline) GetStats() map[string]interface{} {
	st.mu.RLock()
	defer st.mu.RUnlock()

	totalSize := int64(0)
	protectedCount := 0
	for _, snap := range st.snaps {
		totalSize += snap.Size
		if snap.IsProtected {
			protectedCount++
		}
	}

	return map[string]interface{}{
		"totalSnapshots": len(st.snaps),
		"totalSize":      totalSize,
		"protectedCount": protectedCount,
		"volumeCount":    len(st.ListVolumes()),
	}
}
