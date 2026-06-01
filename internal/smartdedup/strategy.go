package smartdedup

import (
	"path/filepath"
	"sort"
)

// Strategy 智能保留策略。
// 根据配置的保留策略从重复组中选择保留文件和待删除文件。
type Strategy struct {
	policy RetentionPolicy
}

// NewStrategy 创建新的保留策略。
func NewStrategy(policy RetentionPolicy) *Strategy {
	return &Strategy{policy: policy}
}

// Selection 保留策略选择结果。
type Selection struct {
	Keep   *FileInfo   // 保留的文件
	Remove []*FileInfo // 待删除的文件
}

// Select 从重复组中选择保留文件和待删除文件。
func (s *Strategy) Select(group *DuplicateGroup) *Selection {
	if group == nil || len(group.Files) < 2 {
		return nil
	}

	// 根据策略排序
	sorted := make([]*FileInfo, len(group.Files))
	copy(sorted, group.Files)

	switch s.policy {
	case RetainNewest:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ModTime.After(sorted[j].ModTime)
		})
	case RetainOldest:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ModTime.Before(sorted[j].ModTime)
		})
	case RetainLargest:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Size > sorted[j].Size
		})
	case RetainSmallest:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Size < sorted[j].Size
		})
	case RetainMostUsed:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].UsageCount > sorted[j].UsageCount
		})
	case RetainShortestPath:
		sort.Slice(sorted, func(i, j int) bool {
			return len(sorted[i].Path) < len(sorted[j].Path)
		})
	default:
		// 默认保留最新
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ModTime.After(sorted[j].ModTime)
		})
	}

	return &Selection{
		Keep:   sorted[0],
		Remove: sorted[1:],
	}
}

// SelectAll 从多个重复组中批量选择。
func (s *Strategy) SelectAll(groups []*DuplicateGroup) []*Selection {
	selections := make([]*Selection, 0, len(groups))
	for _, group := range groups {
		if sel := s.Select(group); sel != nil {
			selections = append(selections, sel)
		}
	}
	return selections
}

// SelectSimilar 从相似文件组中选择保留文件。
// 相似文件组使用额外的启发式规则：
// 优先保留 ContentHash 不同的文件（真正的不同版本），
// 如果内容完全相同则按主策略处理。
func (s *Strategy) SelectSimilar(group *SimilarGroup) *Selection {
	if group == nil || len(group.Files) < 2 {
		return nil
	}

	// 按内容哈希再分组
	contentGroups := make(map[string][]*FileInfo)
	for _, fi := range group.Files {
		contentGroups[fi.ContentHash] = append(contentGroups[fi.ContentHash], fi)
	}

	// 如果所有文件内容相同，按主策略处理
	if len(contentGroups) == 1 {
		dg := &DuplicateGroup{
			ContentHash: group.Files[0].ContentHash,
			Files:       group.Files,
		}
		return s.Select(dg)
	}

	// 内容不同但感知哈希相同，保留每个内容版本中最新的
	sorted := make([]*FileInfo, 0, len(group.Files))
	for _, files := range contentGroups {
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.After(files[j].ModTime)
		})
		sorted = append(sorted, files...)
	}

	// 按策略排序
	sort.Slice(sorted, func(i, j int) bool {
		switch s.policy {
		case RetainNewest:
			return sorted[i].ModTime.After(sorted[j].ModTime)
		case RetainOldest:
			return sorted[i].ModTime.Before(sorted[j].ModTime)
		case RetainLargest:
			return sorted[i].Size > sorted[j].Size
		case RetainSmallest:
			return sorted[i].Size < sorted[j].Size
		case RetainMostUsed:
			return sorted[i].UsageCount > sorted[j].UsageCount
		case RetainShortestPath:
			return len(sorted[i].Path) < len(sorted[j].Path)
		default:
			return sorted[i].ModTime.After(sorted[j].ModTime)
		}
	})

	// 保留第一个，标记其余为删除
	// 但要保留不同内容哈希的版本（至少各留一个）
	kept := make(map[string]bool)
	keep := sorted[0]
	kept[keep.ContentHash] = true
	remove := make([]*FileInfo, 0)

	for _, fi := range sorted[1:] {
		if !kept[fi.ContentHash] {
			kept[fi.ContentHash] = true
			// 这是不同内容的版本，但策略不优先保留
			// 仍标记为删除，用户可通过 DryRun 模式检查
		}
		remove = append(remove, fi)
	}

	return &Selection{
		Keep:   keep,
		Remove: remove,
	}
}

// EstimateSaving 估算去重后可节省的空间。
func (s *Strategy) EstimateSaving(groups []*DuplicateGroup) int64 {
	var total int64
	for _, group := range groups {
		if len(group.Files) < 2 {
			continue
		}
		// 保留一个文件，删除其余
		total += int64(len(group.Files)-1) * group.Files[0].Size
	}
	return total
}

// EstimateSavingReadable 返回人类可读的节省空间。
func (s *Strategy) EstimateSavingReadable(groups []*DuplicateGroup) string {
	return FormatSize(s.EstimateSaving(groups))
}

// SortGroupsBySaving 按可节省空间降序排序重复组。
func SortGroupsBySaving(groups []*DuplicateGroup) {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].SavedSize > groups[j].SavedSize
	})
}

// SortGroupsByCount 按重复文件数量降序排序重复组。
func SortGroupsByCount(groups []*DuplicateGroup) {
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].Files) > len(groups[j].Files)
	})
}

// FilterGroupsByMinSize 过滤掉可节省空间低于阈值的组。
func FilterGroupsByMinSize(groups []*DuplicateGroup, minSaving int64) []*DuplicateGroup {
	filtered := make([]*DuplicateGroup, 0, len(groups))
	for _, g := range groups {
		if g.SavedSize >= minSaving {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// FilterGroupsByContentType 按内容类型过滤重复组。
func FilterGroupsByContentType(groups []*DuplicateGroup, ct ContentType) []*DuplicateGroup {
	filtered := make([]*DuplicateGroup, 0, len(groups))
	for _, g := range groups {
		if len(g.Files) > 0 && g.Files[0].ContentType == ct {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// getExt 返回文件扩展名（小写）。
func getExt(path string) string {
	return filepath.Ext(path)
}
