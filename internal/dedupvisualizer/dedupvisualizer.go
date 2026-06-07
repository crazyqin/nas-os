// Package dedupvisualizer 提供去重可视化功能
// 展示存储空间的去重效果、冗余分析和空间节省情况
// 参考TrueNAS的去重可视化和群晖的存储分析器
package dedupvisualizer

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// 数据块类型
const (
	BlockTypeFile   = "file"   // 文件块
	BlockTypeMeta   = "meta"   // 元数据块
	BlockTypeInline = "inline" // 内联数据
	BlockTypeExtent = "extent" // 区块
)

// 冗余级别
const (
	RedundancyNone   = "none"   // 无冗余
	RedundancyLow    = "low"    // 低冗余（2-3副本）
	RedundancyMedium = "medium" // 中冗余（4-10副本）
	RedundancyHigh   = "high"   // 高冗余（>10副本）
)

var (
	ErrSnapshotNotFound = errors.New("快照不存在")
	ErrVolumeNotFound   = errors.New("卷不存在")
	ErrNoData           = errors.New("无数据")
)

// DataBlock 数据块
type DataBlock struct {
	Hash       string    `json:"hash"`        // 数据哈希
	Size       int64     `json:"size"`        // 块大小
	Type       string    `json:"type"`        // 块类型
	RefCount   int       `json:"ref_count"`   // 引用计数
	FilePaths  []string  `json:"file_paths"`  // 引用文件列表
	FirstSeen  time.Time `json:"first_seen"`  // 首次出现
	LastAccess time.Time `json:"last_access"` // 最后访问
}

// DedupSnapshot 去重快照
type DedupSnapshot struct {
	ID             string    `json:"id"`              // 快照ID
	VolumeID       string    `json:"volume_id"`       // 卷ID
	Timestamp      time.Time `json:"timestamp"`       // 快照时间
	TotalBlocks    int64     `json:"total_blocks"`    // 总块数
	UniqueBlocks   int64     `json:"unique_blocks"`   // 唯一块数
	TotalSize      int64     `json:"total_size"`      // 总逻辑大小
	PhysicalSize   int64     `json:"physical_size"`   // 物理大小
	DedupRatio     float64   `json:"dedup_ratio"`     // 去重率
	SavingsBytes   int64     `json:"savings_bytes"`   // 节省空间
	SavingsPercent float64   `json:"savings_percent"` // 节省百分比
}

// VolumeStats 卷统计
type VolumeStats struct {
	VolumeID        string           `json:"volume_id"`
	VolumeName      string           `json:"volume_name"`
	TotalSize       int64            `json:"total_size"`
	UsedSize        int64            `json:"used_size"`
	DedupEnabled    bool             `json:"dedup_enabled"`
	CompressionType string           `json:"compression_type"`
	CurrentSnapshot *DedupSnapshot   `json:"current_snapshot"`
	History         []*DedupSnapshot `json:"history"`
	TopDuplicates   []*DataBlock     `json:"top_duplicates"`
	ByType          map[string]int64 `json:"by_type"`
}

// DedupReport 去重报告
type DedupReport struct {
	GeneratedAt     time.Time      `json:"generated_at"`
	Volumes         []*VolumeStats `json:"volumes"`
	TotalSaved      int64          `json:"total_saved"`
	OverallRatio    float64        `json:"overall_ratio"`
	Recommendations []string       `json:"recommendations"`
}

// DedupVisualizer 去重可视化引擎
type DedupVisualizer struct {
	mu        sync.RWMutex
	volumes   map[string]*VolumeStats
	blocks    map[string][]*DataBlock // hash -> blocks
	snapshots map[string][]*DedupSnapshot
}

// NewDedupVisualizer 创建去重可视化引擎
func NewDedupVisualizer() *DedupVisualizer {
	return &DedupVisualizer{
		volumes:   make(map[string]*VolumeStats),
		blocks:    make(map[string][]*DataBlock),
		snapshots: make(map[string][]*DedupSnapshot),
	}
}

// RegisterVolume 注册卷
func (v *DedupVisualizer) RegisterVolume(volID, volName string, totalSize int64, dedupEnabled bool, compressionType string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.volumes[volID] = &VolumeStats{
		VolumeID:        volID,
		VolumeName:      volName,
		TotalSize:       totalSize,
		DedupEnabled:    dedupEnabled,
		CompressionType: compressionType,
		History:         make([]*DedupSnapshot, 0),
		TopDuplicates:   make([]*DataBlock, 0),
		ByType:          make(map[string]int64),
	}
}

// AddBlock 添加数据块
func (v *DedupVisualizer) AddBlock(block *DataBlock) {
	v.mu.Lock()
	defer v.mu.Unlock()

	block.FirstSeen = time.Now()
	block.LastAccess = time.Now()
	v.blocks[block.Hash] = append(v.blocks[block.Hash], block)

	// 更新引用计数
	count := len(v.blocks[block.Hash])
	for _, b := range v.blocks[block.Hash] {
		b.RefCount = count
	}
}

// TakeSnapshot 创建去重快照
func (v *DedupVisualizer) TakeSnapshot(volID string) (*DedupSnapshot, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	vol, ok := v.volumes[volID]
	if !ok {
		return nil, ErrVolumeNotFound
	}

	snap := &DedupSnapshot{
		ID:        fmt.Sprintf("snap-%s-%d", volID, time.Now().Unix()),
		VolumeID:  volID,
		Timestamp: time.Now(),
	}

	var totalSize, physicalSize int64
	uniqueHashes := make(map[string]bool)

	for hash, blocks := range v.blocks {
		blockSize := blocks[0].Size
		totalSize += blockSize * int64(len(blocks))
		physicalSize += blockSize
		uniqueHashes[hash] = true
	}

	snap.TotalBlocks = int64(len(v.blocks))
	snap.UniqueBlocks = int64(len(uniqueHashes))
	snap.TotalSize = totalSize
	snap.PhysicalSize = physicalSize
	if physicalSize > 0 {
		snap.DedupRatio = float64(totalSize) / float64(physicalSize)
	}
	snap.SavingsBytes = totalSize - physicalSize
	if totalSize > 0 {
		snap.SavingsPercent = float64(snap.SavingsBytes) / float64(totalSize) * 100
	}

	v.snapshots[volID] = append(v.snapshots[volID], snap)
	vol.CurrentSnapshot = snap
	vol.History = append(vol.History, snap)

	// 更新Top Duplicates
	topDups := make([]*DataBlock, 0)
	for _, blocks := range v.blocks {
		if len(blocks) > 1 {
			topDups = append(topDups, blocks[0])
		}
	}
	sort.Slice(topDups, func(i, j int) bool {
		return topDups[i].RefCount > topDups[j].RefCount
	})
	if len(topDups) > 20 {
		topDups = topDups[:20]
	}
	vol.TopDuplicates = topDups

	return snap, nil
}

// GetVolumeStats 获取卷统计
func (v *DedupVisualizer) GetVolumeStats(volID string) (*VolumeStats, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	vol, ok := v.volumes[volID]
	if !ok {
		return nil, ErrVolumeNotFound
	}
	return vol, nil
}

// GenerateReport 生成去重报告
func (v *DedupVisualizer) GenerateReport() *DedupReport {
	v.mu.RLock()
	defer v.mu.RUnlock()

	report := &DedupReport{
		GeneratedAt: time.Now(),
		Volumes:     make([]*VolumeStats, 0),
	}

	var totalSaved int64
	var totalLogical int64
	var totalPhysical int64

	for _, vol := range v.volumes {
		report.Volumes = append(report.Volumes, vol)
		if vol.CurrentSnapshot != nil {
			totalSaved += vol.CurrentSnapshot.SavingsBytes
			totalLogical += vol.CurrentSnapshot.TotalSize
			totalPhysical += vol.CurrentSnapshot.PhysicalSize
		}
	}

	report.TotalSaved = totalSaved
	if totalPhysical > 0 {
		report.OverallRatio = float64(totalLogical) / float64(totalPhysical)
	}

	// 生成建议
	report.Recommendations = make([]string, 0)
	for _, vol := range v.volumes {
		if !vol.DedupEnabled && vol.UsedSize > 100*1024*1024*1024 {
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("卷 %s 容量超过100GB，建议开启去重以节省空间", vol.VolumeName))
		}
		if vol.CurrentSnapshot != nil && vol.CurrentSnapshot.DedupRatio > 3.0 {
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("卷 %s 去重率 %.1fx，存在大量重复数据", vol.VolumeName, vol.CurrentSnapshot.DedupRatio))
		}
	}

	return report
}

// ExportJSON 导出JSON报告
func (v *DedupVisualizer) ExportJSON() ([]byte, error) {
	report := v.GenerateReport()
	return json.MarshalIndent(report, "", "  ")
}
