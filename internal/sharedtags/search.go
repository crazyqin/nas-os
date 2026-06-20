package sharedtags

import (
	"log"
	"sort"
	"strings"
	"time"
)

// TagSearch provides advanced tag-based search with AND/OR/NOT operators
type TagSearch struct {
	tagger  *FileTagger
	manager *TagManager
}

// NewTagSearch creates a new TagSearch instance
func NewTagSearch(tagger *FileTagger, manager *TagManager) *TagSearch {
	s := &TagSearch{
		tagger:  tagger,
		manager: manager,
	}
	log.Println("标签搜索引擎已初始化")
	return s
}

// Search performs a tag-based search with the given query
func (s *TagSearch) Search(query SearchQuery) (*SearchResult, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	var matchedFiles map[string]bool

	switch query.Operator {
	case OpAnd:
		matchedFiles = s.searchAND(query)
	case OpOr:
		matchedFiles = s.searchOR(query)
	case OpNot:
		matchedFiles = s.searchNOT(query)
	default:
		matchedFiles = s.searchAND(query)
	}

	// Apply additional filters
	filteredFiles := s.applyFilters(matchedFiles, query)

	// Collect results
	var allResults []FileTag
	for filePath := range filteredFiles {
		fileTags := s.tagger.GetFileTags(filePath)
		for _, ft := range fileTags {
			allResults = append(allResults, *ft)
		}
	}

	// Sort by creation time descending
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].CreatedAt.After(allResults[j].CreatedAt)
	})

	total := int64(len(allResults))
	start := query.Offset
	if start > len(allResults) {
		start = len(allResults)
	}
	end := start + query.Limit
	if end > len(allResults) {
		end = len(allResults)
	}

	pagedResults := allResults[start:end]

	return &SearchResult{
		Files:   pagedResults,
		Total:   total,
		HasMore: int64(end) < total,
		Query:   query,
	}, nil
}

// searchAND returns files that have ALL specified tags
func (s *TagSearch) searchAND(query SearchQuery) map[string]bool {
	if len(query.Tags) == 0 {
		return make(map[string]bool)
	}

	// Start with files from first tag
	firstTagFiles := s.getFileSet(query.Tags[0])
	result := make(map[string]bool)
	for fp := range firstTagFiles {
		result[fp] = true
	}

	// Intersect with files from remaining tags
	for i := 1; i < len(query.Tags); i++ {
		tagFiles := s.getFileSet(query.Tags[i])
		intersection := make(map[string]bool)
		for fp := range result {
			if tagFiles[fp] {
				intersection[fp] = true
			}
		}
		result = intersection
	}

	return result
}

// searchOR returns files that have ANY of the specified tags
func (s *TagSearch) searchOR(query SearchQuery) map[string]bool {
	result := make(map[string]bool)
	for _, tagID := range query.Tags {
		tagFiles := s.getFileSet(tagID)
		for fp := range tagFiles {
			result[fp] = true
		}
	}
	return result
}

// searchNOT returns files that do NOT have any of the specified tags
func (s *TagSearch) searchNOT(query SearchQuery) map[string]bool {
	// Get all files
	allFiles := s.getAllFiles()

	// Get files with specified tags
	excludedFiles := make(map[string]bool)
	for _, tagID := range query.Tags {
		tagFiles := s.getFileSet(tagID)
		for fp := range tagFiles {
			excludedFiles[fp] = true
		}
	}

	// Return files not in excluded set
	result := make(map[string]bool)
	for fp := range allFiles {
		if !excludedFiles[fp] {
			result[fp] = true
		}
	}
	return result
}

// getFileSet returns a set of file paths that have a specific tag
func (s *TagSearch) getFileSet(tagID string) map[string]bool {
	files := s.tagger.GetTagFiles(tagID)
	result := make(map[string]bool)
	for _, ft := range files {
		result[ft.FilePath] = true
	}
	return result
}

// getAllFiles returns all files that have any tag
func (s *TagSearch) getAllFiles() map[string]bool {
	result := make(map[string]bool)
	// This is a simplified implementation
	// In production, this would iterate all file tags
	return result
}

// applyFilters applies additional filters (category, keyword, owner, date range)
func (s *TagSearch) applyFilters(files map[string]bool, query SearchQuery) map[string]bool {
	if len(files) == 0 {
		return files
	}

	result := make(map[string]bool)

	for filePath := range files {
		fileTags := s.tagger.GetFileTags(filePath)
		include := true

		// Filter by category
		if query.CategoryID != "" {
			hasCategory := false
			for _, ft := range fileTags {
				tag, err := s.manager.GetTag(ft.TagID)
				if err == nil && tag.CategoryID == query.CategoryID {
					hasCategory = true
					break
				}
			}
			if !hasCategory {
				include = false
			}
		}

		// Filter by keyword
		if include && query.Keyword != "" {
			keyword := strings.ToLower(query.Keyword)
			hasKeyword := false
			for _, ft := range fileTags {
				if strings.Contains(strings.ToLower(ft.TagName), keyword) {
					hasKeyword = true
					break
				}
			}
			if !hasKeyword && !strings.Contains(strings.ToLower(filePath), keyword) {
				include = false
			}
		}

		// Filter by owner
		if include && query.Owner != "" {
			hasOwner := false
			for _, ft := range fileTags {
				if ft.TaggedBy == query.Owner {
					hasOwner = true
					break
				}
			}
			if !hasOwner {
				include = false
			}
		}

		// Filter by date range
		if include && (query.DateFrom != nil || query.DateTo != nil) {
			hasDateMatch := false
			for _, ft := range fileTags {
				if query.DateFrom != nil && ft.CreatedAt.Before(*query.DateFrom) {
					continue
				}
				if query.DateTo != nil && ft.CreatedAt.After(*query.DateTo) {
					continue
				}
				hasDateMatch = true
				break
			}
			if !hasDateMatch {
				include = false
			}
		}

		if include {
			result[filePath] = true
		}
	}

	return result
}

// SearchByCategory searches files by category
func (s *TagSearch) SearchByCategory(categoryID string, limit int) (*SearchResult, error) {
	return s.Search(SearchQuery{
		Operator:   OpOr,
		CategoryID: categoryID,
		Limit:      limit,
	})
}

// SearchByKeyword searches files by keyword in tag names
func (s *TagSearch) SearchByKeyword(keyword string, limit int) (*SearchResult, error) {
	return s.Search(SearchQuery{
		Operator: OpOr,
		Keyword:  keyword,
		Limit:    limit,
	})
}

// RecentTagged returns recently tagged files
func (s *TagSearch) RecentTagged(limit int) *SearchResult {
	since := time.Now().Add(-7 * 24 * time.Hour) // Last 7 days
	result, _ := s.Search(SearchQuery{
		Operator: OpOr,
		DateFrom: &since,
		Limit:    limit,
	})
	return result
}

// GetRelatedTags returns tags frequently used together with the given tag
func (s *TagSearch) GetRelatedTags(tagID string, limit int) []*Tag {
	files := s.tagger.GetTagFiles(tagID)
	tagCount := make(map[string]int64)

	for _, ft := range files {
		fileTags := s.tagger.GetFileTags(ft.FilePath)
		for _, otherFT := range fileTags {
			if otherFT.TagID != tagID {
				tagCount[otherFT.TagID]++
			}
		}
	}

	type tagScore struct {
		tagID string
		count int64
	}
	var scores []tagScore
	for id, count := range tagCount {
		scores = append(scores, tagScore{id, count})
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].count > scores[j].count
	})

	if limit > len(scores) {
		limit = len(scores)
	}

	var result []*Tag
	for i := 0; i < limit; i++ {
		if tag, err := s.manager.GetTag(scores[i].tagID); err == nil {
			result = append(result, tag)
		}
	}
	return result
}
