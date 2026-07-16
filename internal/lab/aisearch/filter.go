// Package aisearch 提供搜索过滤器功能
package aisearch

import (
	"strings"
	"time"
)

// Filter 搜索过滤器.
type Filter struct {
	fileTypes []FileType
	tags      []string
	dateFrom  *time.Time
	dateTo    *time.Time
	sizeMin   *int64
	sizeMax   *int64
	paths     []string
}

// NewFilter 创建过滤器.
func NewFilter() *Filter {
	return &Filter{}
}

// WithFileTypes 设置文件类型过滤.
func (f *Filter) WithFileTypes(types ...FileType) *Filter {
	f.fileTypes = types
	return f
}

// WithTags 设置标签过滤.
func (f *Filter) WithTags(tags ...string) *Filter {
	f.tags = tags
	return f
}

// WithDateRange 设置日期范围过滤.
func (f *Filter) WithDateRange(from, to time.Time) *Filter {
	f.dateFrom = &from
	f.dateTo = &to
	return f
}

// WithSizeRange 设置文件大小范围过滤 (bytes).
func (f *Filter) WithSizeRange(min, max int64) *Filter {
	f.sizeMin = &min
	f.sizeMax = &max
	return f
}

// WithPaths 设置路径过滤.
func (f *Filter) WithPaths(paths ...string) *Filter {
	f.paths = paths
	return f
}

// Apply 应用过滤器.
func (f *Filter) Apply(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0)

	for _, result := range results {
		if f.match(result) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// match 检查结果是否匹配过滤条件.
func (f *Filter) match(result SearchResult) bool {
	// 文件类型过滤
	if len(f.fileTypes) > 0 {
		matched := false
		for _, ft := range f.fileTypes {
			if result.FileType == ft {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 标签过滤
	if len(f.tags) > 0 {
		matched := false
		for _, filterTag := range f.tags {
			for _, resultTag := range result.Tags {
				if strings.EqualFold(filterTag, resultTag) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 日期过滤
	if f.dateFrom != nil && result.ModifiedAt.Before(*f.dateFrom) {
		return false
	}
	if f.dateTo != nil && result.ModifiedAt.After(*f.dateTo) {
		return false
	}

	// 文件大小过滤
	if f.sizeMin != nil && result.FileSize < *f.sizeMin {
		return false
	}
	if f.sizeMax != nil && result.FileSize > *f.sizeMax {
		return false
	}

	// 路径过滤
	if len(f.paths) > 0 {
		matched := false
		for _, path := range f.paths {
			if strings.HasPrefix(result.FilePath, path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// FilterBuilder 过滤器构建器.
type FilterBuilder struct {
	filters []*Filter
}

// NewFilterBuilder 创建过滤器构建器.
func NewFilterBuilder() *FilterBuilder {
	return &FilterBuilder{
		filters: make([]*Filter, 0),
	}
}

// Add 添加过滤器.
func (fb *FilterBuilder) Add(filter *Filter) *FilterBuilder {
	fb.filters = append(fb.filters, filter)
	return fb
}

// Apply 应用所有过滤器 (AND 逻辑).
func (fb *FilterBuilder) Apply(results []SearchResult) []SearchResult {
	current := results
	for _, filter := range fb.filters {
		current = filter.Apply(current)
	}
	return current
}

// DateFilter 日期过滤器.
type DateFilter struct {
	Preset string // today, yesterday, this_week, this_month, this_year, last_7_days, last_30_days, last_90_days, last_year
}

// NewDateFilter 创建日期过滤器.
func NewDateFilter(preset string) *DateFilter {
	return &DateFilter{Preset: preset}
}

// Apply 应用日期过滤器.
func (df *DateFilter) Apply(results []SearchResult) []SearchResult {
	now := time.Now()
	var from, to time.Time

	switch df.Preset {
	case "today":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = now
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		from = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "this_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		from = now.AddDate(0, 0, -(weekday - 1))
		from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
		to = now
	case "this_month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = now
	case "this_year":
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		to = now
	case "last_7_days":
		from = now.AddDate(0, 0, -7)
		to = now
	case "last_30_days":
		from = now.AddDate(0, 0, -30)
		to = now
	case "last_90_days":
		from = now.AddDate(0, 0, -90)
		to = now
	case "last_year":
		from = now.AddDate(-1, 0, 0)
		to = now
	default:
		return results
	}

	filter := NewFilter().WithDateRange(from, to)
	return filter.Apply(results)
}

// SizeFilter 文件大小过滤器.
type SizeFilter struct {
	Preset string // tiny, small, medium, large, huge
}

// NewSizeFilter 创建文件大小过滤器.
func NewSizeFilter(preset string) *SizeFilter {
	return &SizeFilter{Preset: preset}
}

// Apply 应用文件大小过滤器.
func (sf *SizeFilter) Apply(results []SearchResult) []SearchResult {
	var min, max int64

	switch sf.Preset {
	case "tiny": // < 10KB
		max = 10 * 1024
	case "small": // 10KB - 100KB
		min = 10 * 1024
		max = 100 * 1024
	case "medium": // 100KB - 1MB
		min = 100 * 1024
		max = 1024 * 1024
	case "large": // 1MB - 100MB
		min = 1024 * 1024
		max = 100 * 1024 * 1024
	case "huge": // > 100MB
		min = 100 * 1024 * 1024
	default:
		return results
	}

	filter := NewFilter().WithSizeRange(min, max)
	return filter.Apply(results)
}

// FileTypeFilter 文件类型过滤器.
type FileTypeFilter struct {
	Category string // documents, images, videos, audio, archives, code
}

// NewFileTypeFilter 创建文件类型过滤器.
func NewFileTypeFilter(category string) *FileTypeFilter {
	return &FileTypeFilter{Category: category}
}

// Apply 应用文件类型过滤器.
func (ftf *FileTypeFilter) Apply(results []SearchResult) []SearchResult {
	var types []FileType

	switch ftf.Category {
	case "documents":
		types = []FileType{FileTypeDocument}
	case "images":
		types = []FileType{FileTypeImage}
	case "videos":
		types = []FileType{FileTypeVideo}
	case "audio":
		types = []FileType{FileTypeAudio}
	case "archives":
		types = []FileType{FileTypeArchive}
	case "code":
		types = []FileType{FileTypeCode}
	case "media":
		types = []FileType{FileTypeImage, FileTypeVideo, FileTypeAudio}
	default:
		return results
	}

	filter := NewFilter().WithFileTypes(types...)
	return filter.Apply(results)
}

// PathFilter 路径过滤器.
type PathFilter struct {
	RootPath  string
	Recursive bool
}

// NewPathFilter 创建路径过滤器.
func NewPathFilter(rootPath string, recursive bool) *PathFilter {
	return &PathFilter{
		RootPath:  rootPath,
		Recursive: recursive,
	}
}

// Apply 应用路径过滤器.
func (pf *PathFilter) Apply(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0)

	for _, result := range results {
		if pf.matchPath(result.FilePath) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// matchPath 检查路径是否匹配.
func (pf *PathFilter) matchPath(filePath string) bool {
	if pf.Recursive {
		return strings.HasPrefix(filePath, pf.RootPath)
	}

	// 非递归：检查是否在同一目录
	dir := filePath[:strings.LastIndex(filePath, "/")]
	return dir == pf.RootPath
}

// TagFilter 标签过滤器.
type TagFilter struct {
	Tags []string
	Mode string // any, all
}

// NewTagFilter 创建标签过滤器.
func NewTagFilter(tags []string, mode string) *TagFilter {
	if mode == "" {
		mode = "any"
	}
	return &TagFilter{
		Tags: tags,
		Mode: mode,
	}
}

// Apply 应用标签过滤器.
func (tf *TagFilter) Apply(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0)

	for _, result := range results {
		if tf.matchTags(result.Tags) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// matchTags 检查标签是否匹配.
func (tf *TagFilter) matchTags(resultTags []string) bool {
	if len(tf.Tags) == 0 {
		return true
	}

	matched := 0
	for _, filterTag := range tf.Tags {
		for _, resultTag := range resultTags {
			if strings.EqualFold(filterTag, resultTag) {
				matched++
				break
			}
		}
	}

	switch tf.Mode {
	case "all":
		return matched == len(tf.Tags)
	default: // "any"
		return matched > 0
	}
}

// CompositeFilter 组合过滤器.
type CompositeFilter struct {
	filters []FilterApplier
	mode    string // and, or
}

// FilterApplier 过滤器应用接口.
type FilterApplier interface {
	Apply(results []SearchResult) []SearchResult
}

// NewCompositeFilter 创建组合过滤器.
func NewCompositeFilter(mode string, filters ...FilterApplier) *CompositeFilter {
	if mode == "" {
		mode = "and"
	}
	return &CompositeFilter{
		filters: filters,
		mode:    mode,
	}
}

// Apply 应用组合过滤器.
func (cf *CompositeFilter) Apply(results []SearchResult) []SearchResult {
	if len(cf.filters) == 0 {
		return results
	}

	switch cf.mode {
	case "or":
		return cf.applyOr(results)
	default: // "and"
		return cf.applyAnd(results)
	}
}

// applyAnd 应用 AND 逻辑.
func (cf *CompositeFilter) applyAnd(results []SearchResult) []SearchResult {
	current := results
	for _, filter := range cf.filters {
		current = filter.Apply(current)
	}
	return current
}

// applyOr 应用 OR 逻辑.
func (cf *CompositeFilter) applyOr(results []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	merged := make([]SearchResult, 0)

	for _, filter := range cf.filters {
		filtered := filter.Apply(results)
		for _, result := range filtered {
			if !seen[result.ID] {
				seen[result.ID] = true
				merged = append(merged, result)
			}
		}
	}

	return merged
}
