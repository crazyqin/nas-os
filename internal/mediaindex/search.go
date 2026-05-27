package mediaindex

import (
	"sort"
	"strings"
)

// SearchEngine 媒体搜索引擎.
type SearchEngine struct {
	indexer *Indexer
}

// NewSearchEngine 创建搜索引擎.
func NewSearchEngine(indexer *Indexer) *SearchEngine {
	return &SearchEngine{indexer: indexer}
}

// Search 搜索媒体文件.
func (se *SearchEngine) Search(query SearchQuery) *SearchResult {
	se.indexer.mu.RLock()
	defer se.indexer.mu.RUnlock()

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	var matched []*MediaFile
	for _, f := range se.indexer.files {
		if se.matchFile(f, query) {
			matched = append(matched, f)
		}
	}

	// 排序
	se.sortFiles(matched, query.SortBy, query.SortOrder)

	// 分页
	total := len(matched)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}

	return &SearchResult{
		Files:    matched[start:end],
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}
}

// matchFile 检查文件是否匹配查询.
func (se *SearchEngine) matchFile(f *MediaFile, q SearchQuery) bool {
	// 关键词匹配（文件名）
	if q.Keyword != "" {
		if !strings.Contains(strings.ToLower(f.Name), strings.ToLower(q.Keyword)) {
			return false
		}
	}

	// 类型过滤
	if q.Type != "" && f.Type != q.Type {
		return false
	}

	// 日期范围
	if q.DateFrom != nil && f.TakenAt != nil && f.TakenAt.Before(*q.DateFrom) {
		return false
	}
	if q.DateTo != nil && f.TakenAt != nil && f.TakenAt.After(*q.DateTo) {
		return false
	}

	// 大小范围
	if q.MinSize > 0 && f.Size < q.MinSize {
		return false
	}
	if q.MaxSize > 0 && f.Size > q.MaxSize {
		return false
	}

	// 标签过滤
	if len(q.Tags) > 0 {
		tagMatch := false
		for _, qt := range q.Tags {
			for _, ft := range f.Tags {
				if ft == qt {
					tagMatch = true
					break
				}
			}
			if tagMatch {
				break
			}
		}
		if !tagMatch {
			return false
		}
	}

	// 合集过滤
	if len(q.Collections) > 0 {
		colMatch := false
		for _, qc := range q.Collections {
			for _, fc := range f.Collections {
				if fc == qc {
					colMatch = true
					break
				}
			}
			if colMatch {
				break
			}
		}
		if !colMatch {
			return false
		}
	}

	// 位置过滤
	if q.Location != "" && f.GPS != nil {
		if !strings.Contains(strings.ToLower(f.GPS.Location), strings.ToLower(q.Location)) {
			return false
		}
	}

	return true
}

// sortFiles 排序文件.
func (se *SearchEngine) sortFiles(files []*MediaFile, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "date"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(files, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = files[i].Name < files[j].Name
		case "size":
			less = files[i].Size < files[j].Size
		case "date":
			ti := files[i].IndexedAt
			if files[i].TakenAt != nil {
				ti = *files[i].TakenAt
			}
			tj := files[j].IndexedAt
			if files[j].TakenAt != nil {
				tj = *files[j].TakenAt
			}
			less = ti.Before(tj)
		default:
			less = files[i].IndexedAt.Before(files[j].IndexedAt)
		}
		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// SearchByTag 按标签搜索.
func (se *SearchEngine) SearchByTag(tagID string) []*MediaFile {
	se.indexer.mu.RLock()
	defer se.indexer.mu.RUnlock()

	tag, ok := se.indexer.tags[tagID]
	if !ok {
		return nil
	}
	var result []*MediaFile
	for _, fid := range tag.FileIDs {
		if f, ok := se.indexer.files[fid]; ok {
			result = append(result, f)
		}
	}
	return result
}

// SearchByType 按类型搜索.
func (se *SearchEngine) SearchByType(mt MediaType) []*MediaFile {
	se.indexer.mu.RLock()
	defer se.indexer.mu.RUnlock()

	var result []*MediaFile
	for _, f := range se.indexer.files {
		if f.Type == mt {
			result = append(result, f)
		}
	}
	return result
}

// SearchByLocation 按位置搜索.
func (se *SearchEngine) SearchByLocation(location string) []*MediaFile {
	se.indexer.mu.RLock()
	defer se.indexer.mu.RUnlock()

	var result []*MediaFile
	for _, f := range se.indexer.files {
		if f.GPS != nil && strings.Contains(strings.ToLower(f.GPS.Location), strings.ToLower(location)) {
			result = append(result, f)
		}
	}
	return result
}

// SearchDuplicates 搜索重复文件.
func (se *SearchEngine) SearchDuplicates() []*MediaFile {
	se.indexer.mu.RLock()
	defer se.indexer.mu.RUnlock()

	var result []*MediaFile
	for _, f := range se.indexer.files {
		if f.IsDuplicate {
			result = append(result, f)
		}
	}
	return result
}

// GetRecent 获取最近索引的文件.
func (se *SearchEngine) GetRecent(limit int) []*MediaFile {
	se.indexer.mu.RLock()
	defer se.indexer.mu.RUnlock()

	all := make([]*MediaFile, 0, len(se.indexer.files))
	for _, f := range se.indexer.files {
		all = append(all, f)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].IndexedAt.After(all[j].IndexedAt)
	})
	if limit > len(all) {
		limit = len(all)
	}
	return all[:limit]
}
