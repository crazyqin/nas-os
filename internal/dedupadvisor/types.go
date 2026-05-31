package dedupadvisor

import (
	"time"
)

// Types 类型定义

// DedupStats 去重统计
type DedupStats struct {
	TotalScans      int       `json:"total_scans"`
	TotalFiles      int       `json:"total_files_scanned"`
	TotalDuplicates int       `json:"total_duplicates_found"`
	TotalSaveable   int64     `json:"total_saveable_bytes"`
	LastScanTime    time.Time `json:"last_scan_time"`
}

// FileGroup 文件分组
type FileGroup struct {
	Type     FileType       `json:"type"`
	Count    int            `json:"count"`
	TotalSize int64         `json:"total_size"`
	DupCount int            `json:"dup_count"`
	DupSize  int64          `json:"dup_size"`
	Files    []FileInfo     `json:"files"`
}

// DedupAction 去重动作
type DedupAction struct {
	ID        string    `json:"id"`
	ScanID    string    `json:"scan_id"`
	Hash      string    `json:"hash"`
	Action    string    `json:"action"` // keep, remove, symlink
	KeepPath  string    `json:"keep_path"`
	RemovePaths []string `json:"remove_paths"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // pending, completed, failed
}

// GetStats 获取统计信息
func (a *Advisor) GetStats() DedupStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := DedupStats{
		TotalScans: len(a.results),
	}

	for _, result := range a.results {
		stats.TotalFiles += result.TotalFiles
		stats.TotalDuplicates += result.DuplicateFiles
		stats.TotalSaveable += result.SaveableSize
		if result.EndTime.After(stats.LastScanTime) {
			stats.LastScanTime = result.EndTime
		}
	}

	return stats
}

// GroupByType 按类型分组
func (r *ScanResult) GroupByType() map[FileType]*FileGroup {
	groups := make(map[FileType]*FileGroup)

	for _, candidate := range r.Candidates {
		group, ok := groups[candidate.FileType]
		if !ok {
			group = &FileGroup{
				Type: candidate.FileType,
			}
			groups[candidate.FileType] = group
		}

		group.Count += candidate.Count
		group.TotalSize += candidate.TotalSize
		group.DupCount += candidate.Count - 1
		group.DupSize += candidate.PotentialSave
		group.Files = append(group.Files, candidate.Files...)
	}

	return groups
}

// SaveToFile 保存结果到文件
func (r *ScanResult) SaveToFile(path string) error {
	// Implementation would save JSON to file
	return nil
}
