package sharedtags

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileTagger manages file-tag associations with batch operations and auto-suggestions
type FileTagger struct {
	mu          sync.RWMutex
	fileTags    map[string][]*FileTag   // filePath -> []*FileTag
	tagFiles    map[string][]*FileTag   // tagID -> []*FileTag
	manager     *TagManager
	autoRules   []*AutoTagRule
	nextID      int64
}

// AutoTagRule represents a rule for automatic tagging
type AutoTagRule struct {
	ID         string   `json:"id"`         // 规则ID
	Name       string   `json:"name"`       // 规则名称
	Extensions []string `json:"extensions"` // 文件扩展名匹配
	PathPattern string  `json:"pathPattern"` // 路径模式匹配
	Tags       []string `json:"tags"`       // 匹配时添加的标签
	Enabled    bool     `json:"enabled"`    // 是否启用
	Owner      string   `json:"owner"`      // 创建者
	CreatedAt  time.Time `json:"createdAt"` // 创建时间
}

// NewFileTagger creates a new FileTagger instance
func NewFileTagger(manager *TagManager) *FileTagger {
	t := &FileTagger{
		fileTags:  make(map[string][]*FileTag),
		tagFiles:  make(map[string][]*FileTag),
		manager:   manager,
		autoRules: make([]*AutoTagRule, 0),
	}
	log.Println("文件标签关联器已初始化")
	return t
}

// AddTagToFile adds a tag to a file
func (t *FileTagger) AddTagToFile(filePath, tagID, taggedBy string, isAuto bool, confidence float64) (*FileTag, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, &ValidationError{Field: "filePath", Message: "文件路径不能为空"}
	}
	if strings.TrimSpace(tagID) == "" {
		return nil, &ValidationError{Field: "tagId", Message: "标签ID不能为空"}
	}

	// Verify tag exists
	tag, err := t.manager.GetTag(tagID)
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Check for duplicate
	for _, ft := range t.fileTags[filePath] {
		if ft.TagID == tagID {
			return ft, nil // Already tagged
		}
	}

	t.nextID++
	fileTag := &FileTag{
		ID:         fmt.Sprintf("ft-%d", t.nextID),
		FilePath:   filePath,
		TagID:      tagID,
		TagName:    tag.Name,
		TaggedBy:   taggedBy,
		IsAuto:     isAuto,
		Confidence: confidence,
		CreatedAt:  time.Now(),
	}

	t.fileTags[filePath] = append(t.fileTags[filePath], fileTag)
	t.tagFiles[tagID] = append(t.tagFiles[tagID], fileTag)

	// Increment tag usage
	_ = t.manager.IncrementUsage(tagID)

	log.Printf("文件已打标签: %s -> %s", filePath, tag.Name)
	return fileTag, nil
}

// RemoveTagFromFile removes a tag from a file
func (t *FileTagger) RemoveTagFromFile(filePath, tagID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	fileTags, ok := t.fileTags[filePath]
	if !ok {
		return ErrFileNotTagged
	}

	found := false
	var newFileTags []*FileTag
	for _, ft := range fileTags {
		if ft.TagID == tagID {
			found = true
			continue
		}
		newFileTags = append(newFileTags, ft)
	}

	if !found {
		return ErrFileNotTagged
	}

	if len(newFileTags) == 0 {
		delete(t.fileTags, filePath)
	} else {
		t.fileTags[filePath] = newFileTags
	}

	// Update tag files index
	tagFiles := t.tagFiles[tagID]
	var newTagFiles []*FileTag
	for _, ft := range tagFiles {
		if ft.FilePath != filePath {
			newTagFiles = append(newTagFiles, ft)
		}
	}
	if len(newTagFiles) == 0 {
		delete(t.tagFiles, tagID)
	} else {
		t.tagFiles[tagID] = newTagFiles
	}

	log.Printf("文件标签已移除: %s - %s", filePath, tagID)
	return nil
}

// GetFileTags returns all tags for a file
func (t *FileTagger) GetFileTags(filePath string) []*FileTag {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := t.fileTags[filePath]
	result := make([]*FileTag, len(tags))
	copy(result, tags)
	return result
}

// GetTagFiles returns all files for a tag
func (t *FileTagger) GetTagFiles(tagID string) []*FileTag {
	t.mu.RLock()
	defer t.mu.RUnlock()

	files := t.tagFiles[tagID]
	result := make([]*FileTag, len(files))
	copy(result, files)
	return result
}

// BatchTagFiles adds multiple tags to multiple files
func (t *FileTagger) BatchTagFiles(req BatchTagRequest) ([]*FileTag, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var results []*FileTag
	var errors []string

	for _, filePath := range req.Files {
		for _, tagID := range req.Tags {
			ft, err := t.AddTagToFile(filePath, tagID, req.TaggedBy, false, 1.0)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s:%s - %v", filePath, tagID, err))
				continue
			}
			results = append(results, ft)
		}
	}

	if len(errors) > 0 {
		log.Printf("批量打标签部分失败: %s", strings.Join(errors, "; "))
	}

	return results, nil
}

// RemoveAllTagsFromFile removes all tags from a file
func (t *FileTagger) RemoveAllTagsFromFile(filePath string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	fileTags, ok := t.fileTags[filePath]
	if !ok {
		return nil
	}

	for _, ft := range fileTags {
		tagFiles := t.tagFiles[ft.TagID]
		var newTagFiles []*FileTag
		for _, tf := range tagFiles {
			if tf.FilePath != filePath {
				newTagFiles = append(newTagFiles, tf)
			}
		}
		if len(newTagFiles) == 0 {
			delete(t.tagFiles, ft.TagID)
		} else {
			t.tagFiles[ft.TagID] = newTagFiles
		}
	}

	delete(t.fileTags, filePath)
	return nil
}

// AddAutoTagRule adds an automatic tagging rule
func (t *FileTagger) AddAutoTagRule(name string, extensions []string, pathPattern string, tags []string, owner string) *AutoTagRule {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.nextID++
	rule := &AutoTagRule{
		ID:          fmt.Sprintf("rule-%d", t.nextID),
		Name:        name,
		Extensions:  extensions,
		PathPattern: pathPattern,
		Tags:        tags,
		Enabled:     true,
		Owner:       owner,
		CreatedAt:   time.Now(),
	}

	t.autoRules = append(t.autoRules, rule)
	log.Printf("自动标签规则已添加: %s", name)
	return rule
}

// RemoveAutoTagRule removes an auto tag rule by ID
func (t *FileTagger) RemoveAutoTagRule(ruleID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, rule := range t.autoRules {
		if rule.ID == ruleID {
			t.autoRules = append(t.autoRules[:i], t.autoRules[i+1:]...)
			return nil
		}
	}
	return &TagError{Code: "RULE_NOT_FOUND", Message: "规则不存在"}
}

// GetAutoSuggestions returns automatic tag suggestions for a file
func (t *FileTagger) GetAutoSuggestions(filePath string) []*AutoTagSuggestion {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var suggestions []*AutoTagSuggestion
	ext := strings.ToLower(filepath.Ext(filePath))
	dir := filepath.Dir(filePath)

	for _, rule := range t.autoRules {
		if !rule.Enabled {
			continue
		}

		matched := false

		// Check extension match
		for _, ruleExt := range rule.Extensions {
			if ext == strings.ToLower(ruleExt) {
				matched = true
				break
			}
		}

		// Check path pattern match
		if !matched && rule.PathPattern != "" {
			if strings.Contains(dir, rule.PathPattern) {
				matched = true
			}
		}

		if matched {
			for _, tagID := range rule.Tags {
				if tag, err := t.manager.GetTag(tagID); err == nil {
					suggestions = append(suggestions, &AutoTagSuggestion{
						TagID:      tag.ID,
						TagName:    tag.Name,
						Confidence: 0.85,
						Reason:     fmt.Sprintf("匹配规则: %s", rule.Name),
					})
				}
			}
		}
	}

	return suggestions
}

// GetUntaggedFiles returns files that have no tags (from a provided list)
func (t *FileTagger) GetUntaggedFiles(filePaths []string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var untagged []string
	for _, fp := range filePaths {
		if _, ok := t.fileTags[fp]; !ok {
			untagged = append(untagged, fp)
		}
	}
	return untagged
}

// CountFilesWithTag returns the number of files with a specific tag
func (t *FileTagger) CountFilesWithTag(tagID string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return int64(len(t.tagFiles[tagID]))
}

// GetAutoRules returns all auto tag rules
func (t *FileTagger) GetAutoRules() []*AutoTagRule {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*AutoTagRule, len(t.autoRules))
	copy(result, t.autoRules)
	return result
}

// ApplyAutoTags applies auto tagging to a file based on rules
func (t *FileTagger) ApplyAutoTags(filePath, taggedBy string) ([]*FileTag, error) {
	suggestions := t.GetAutoSuggestions(filePath)
	var results []*FileTag

	for _, s := range suggestions {
		ft, err := t.AddTagToFile(filePath, s.TagID, taggedBy, true, s.Confidence)
		if err != nil {
			continue
		}
		results = append(results, ft)
	}

	return results, nil
}
