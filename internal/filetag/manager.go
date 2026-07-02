package filetag

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 文件标签管理器.
type Manager struct {
	mu         sync.RWMutex
	tags       map[string]*Tag       // tagID -> Tag
	fileTags   map[string][]*FileTag // filePath -> []*FileTag
	tagFiles   map[string][]*FileTag // tagID -> []*FileTag
	categories map[string]*TagCategory
	stats      map[string]*TagStats // tagID -> stats
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		tags:       make(map[string]*Tag),
		fileTags:   make(map[string][]*FileTag),
		tagFiles:   make(map[string][]*FileTag),
		categories: make(map[string]*TagCategory),
		stats:      make(map[string]*TagStats),
	}
}

// CreateTag 创建标签.
func (m *Manager) CreateTag(name, color, description, category, createdBy string) (*Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查标签名是否重复
	for _, t := range m.tags {
		if t.Name == name {
			return nil, fmt.Errorf("标签 '%s' 已存在", name)
		}
	}

	tag := &Tag{
		ID:          uuid.New().String(),
		Name:        name,
		Color:       color,
		Description: description,
		Category:    category,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.tags[tag.ID] = tag
	m.stats[tag.ID] = &TagStats{
		TagID:   tag.ID,
		TagName: tag.Name,
	}

	// 更新分类计数
	if category != "" {
		if cat, ok := m.categories[category]; ok {
			cat.TagCount++
		} else {
			m.categories[category] = &TagCategory{
				ID:        uuid.New().String(),
				Name:      category,
				TagCount:  1,
				CreatedAt: time.Now(),
			}
		}
	}

	return tag, nil
}

// GetTag 获取标签.
func (m *Manager) GetTag(tagID string) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tag, ok := m.tags[tagID]
	if !ok {
		return nil, fmt.Errorf("标签 %s 不存在", tagID)
	}
	return tag, nil
}

// ListTags 列出所有标签.
func (m *Manager) ListTags(category string) []*Tag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tags []*Tag
	for _, tag := range m.tags {
		if category == "" || tag.Category == category {
			tags = append(tags, tag)
		}
	}
	return tags
}

// UpdateTag 更新标签.
func (m *Manager) UpdateTag(tagID, name, color, description, category string) (*Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[tagID]
	if !ok {
		return nil, fmt.Errorf("标签 %s 不存在", tagID)
	}

	if name != "" {
		// 检查新名称是否重复
		for _, t := range m.tags {
			if t.ID != tagID && t.Name == name {
				return nil, fmt.Errorf("标签 '%s' 已存在", name)
			}
		}
		tag.Name = name
	}
	if color != "" {
		tag.Color = color
	}
	if description != "" {
		tag.Description = description
	}
	if category != "" {
		// 更新分类计数
		if oldCat, ok := m.categories[tag.Category]; ok {
			oldCat.TagCount--
		}
		tag.Category = category
		if cat, ok := m.categories[category]; ok {
			cat.TagCount++
		} else {
			m.categories[category] = &TagCategory{
				ID:        uuid.New().String(),
				Name:      category,
				TagCount:  1,
				CreatedAt: time.Now(),
			}
		}
	}
	tag.UpdatedAt = time.Now()

	// 更新统计信息
	if stat, ok := m.stats[tagID]; ok {
		stat.TagName = tag.Name
	}

	// 更新关联的FileTag记录
	if fileTags, ok := m.tagFiles[tagID]; ok {
		for _, ft := range fileTags {
			ft.TagName = tag.Name
			ft.TagColor = tag.Color
		}
	}

	return tag, nil
}

// DeleteTag 删除标签.
func (m *Manager) DeleteTag(tagID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[tagID]
	if !ok {
		return fmt.Errorf("标签 %s 不存在", tagID)
	}

	// 移除所有文件关联
	if fileTags, ok := m.tagFiles[tagID]; ok {
		for _, ft := range fileTags {
			// 从 fileTags 映射中移除
			if tags, exists := m.fileTags[ft.FilePath]; exists {
				for i, t := range tags {
					if t.TagID == tagID {
						m.fileTags[ft.FilePath] = append(tags[:i], tags[i+1:]...)
						break
					}
				}
				if len(m.fileTags[ft.FilePath]) == 0 {
					delete(m.fileTags, ft.FilePath)
				}
			}
		}
		delete(m.tagFiles, tagID)
	}

	// 更新分类计数
	if cat, ok := m.categories[tag.Category]; ok {
		cat.TagCount--
		if cat.TagCount <= 0 {
			delete(m.categories, tag.Category)
		}
	}

	delete(m.stats, tagID)
	delete(m.tags, tagID)
	return nil
}

// TagFile 为文件添加标签.
func (m *Manager) TagFile(filePath, tagID, taggedBy, note string) (*FileTag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[tagID]
	if !ok {
		return nil, fmt.Errorf("标签 %s 不存在", tagID)
	}

	// 检查是否已存在
	if fileTags, ok := m.fileTags[filePath]; ok {
		for _, ft := range fileTags {
			if ft.TagID == tagID {
				return nil, fmt.Errorf("文件 '%s' 已有标签 '%s'", filePath, tag.Name)
			}
		}
	}

	fileTag := &FileTag{
		ID:       uuid.New().String(),
		FilePath: filePath,
		TagID:    tagID,
		TagName:  tag.Name,
		TagColor: tag.Color,
		TaggedBy: taggedBy,
		TaggedAt: time.Now(),
		Note:     note,
	}

	m.fileTags[filePath] = append(m.fileTags[filePath], fileTag)
	m.tagFiles[tagID] = append(m.tagFiles[tagID], fileTag)

	// 更新统计
	if stat, ok := m.stats[tagID]; ok {
		stat.FileCount++
		stat.UsageCount++
	}

	return fileTag, nil
}

// UntagFile 移除文件标签.
func (m *Manager) UntagFile(filePath, tagID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fileTags, ok := m.fileTags[filePath]
	if !ok {
		return fmt.Errorf("文件 '%s' 没有标签", filePath)
	}

	found := false
	for i, ft := range fileTags {
		if ft.TagID == tagID {
			// 从 fileTags 中移除
			m.fileTags[filePath] = append(fileTags[:i], fileTags[i+1:]...)
			if len(m.fileTags[filePath]) == 0 {
				delete(m.fileTags, filePath)
			}

			// 从 tagFiles 中移除
			if tagFileTags, ok := m.tagFiles[tagID]; ok {
				for j, tft := range tagFileTags {
					if tft.ID == ft.ID {
						m.tagFiles[tagID] = append(tagFileTags[:j], tagFileTags[j+1:]...)
						break
					}
				}
				if len(m.tagFiles[tagID]) == 0 {
					delete(m.tagFiles, tagID)
				}
			}

			// 更新统计
			if stat, ok := m.stats[tagID]; ok {
				stat.FileCount--
				if stat.FileCount < 0 {
					stat.FileCount = 0
				}
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("文件 '%s' 没有标签 '%s'", filePath, tagID)
	}
	return nil
}

// GetFileTags 获取文件的所有标签.
func (m *Manager) GetFileTags(filePath string) []*FileTag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.fileTags[filePath]
}

// GetTagFiles 获取标签关联的所有文件.
func (m *Manager) GetTagFiles(tagID string) []*FileTag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.tagFiles[tagID]
}

// SearchFiles 搜索文件.
func (m *Manager) SearchFiles(req *SearchRequest) *SearchResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	// 收集所有匹配的文件
	fileSet := make(map[string]bool)

	if len(req.Tags) > 0 || len(req.TagNames) > 0 {
		// 按标签搜索
		tagIDSet := make(map[string]bool)
		for _, tagID := range req.Tags {
			tagIDSet[tagID] = true
		}
		for _, tagName := range req.TagNames {
			for _, tag := range m.tags {
				if tag.Name == tagName {
					tagIDSet[tag.ID] = true
					break
				}
			}
		}

		if req.Operator == "or" {
			// OR 操作：任一标签匹配
			for tagID := range tagIDSet {
				if fileTags, ok := m.tagFiles[tagID]; ok {
					for _, ft := range fileTags {
						fileSet[ft.FilePath] = true
					}
				}
			}
		} else {
			// AND 操作：所有标签都匹配
			first := true
			for tagID := range tagIDSet {
				if fileTags, ok := m.tagFiles[tagID]; ok {
					currentSet := make(map[string]bool)
					for _, ft := range fileTags {
						currentSet[ft.FilePath] = true
					}
					if first {
						fileSet = currentSet
						first = false
					} else {
						// 取交集
						for path := range fileSet {
							if !currentSet[path] {
								delete(fileSet, path)
							}
						}
					}
				} else {
					// 标签没有关联文件，结果为空
					fileSet = make(map[string]bool)
					break
				}
			}
		}
	} else if req.FilePath != "" {
		// 按文件路径搜索
		if fileTags, ok := m.fileTags[req.FilePath]; ok {
			for _, ft := range fileTags {
				fileSet[ft.FilePath] = true
			}
		}
	} else {
		// 返回所有文件
		for path := range m.fileTags {
			fileSet[path] = true
		}
	}

	// 转换为切片并分页
	var allFiles []*FileTag
	for path := range fileSet {
		if fileTags, ok := m.fileTags[path]; ok {
			allFiles = append(allFiles, fileTags...)
		}
	}

	total := len(allFiles)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &SearchResponse{
		Files:    allFiles[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}
}

// BatchTag 批量打标签.
func (m *Manager) BatchTag(req *BatchTagRequest) ([]*FileTag, error) {
	var results []*FileTag
	for _, filePath := range req.FilePaths {
		for _, tagID := range req.TagIDs {
			ft, err := m.TagFile(filePath, tagID, req.TaggedBy, req.Note)
			if err != nil {
				// 忽略已存在的标签，继续处理
				continue
			}
			results = append(results, ft)
		}
	}
	return results, nil
}

// BatchUntag 批量移除标签.
func (m *Manager) BatchUntag(req *BatchUntagRequest) error {
	for _, filePath := range req.FilePaths {
		if len(req.TagIDs) == 0 {
			// 移除所有标签
			fileTags := m.GetFileTags(filePath)
			for _, ft := range fileTags {
				m.UntagFile(filePath, ft.TagID)
			}
		} else {
			for _, tagID := range req.TagIDs {
				m.UntagFile(filePath, tagID)
			}
		}
	}
	return nil
}

// GetTagStats 获取标签统计.
func (m *Manager) GetTagStats(tagID string) (*TagStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stat, ok := m.stats[tagID]
	if !ok {
		return nil, fmt.Errorf("标签 %s 统计不存在", tagID)
	}
	return stat, nil
}

// GetAllStats 获取所有标签统计.
func (m *Manager) GetAllStats() []*TagStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats []*TagStats
	for _, stat := range m.stats {
		stats = append(stats, stat)
	}
	return stats
}

// GetCategories 获取所有分类.
func (m *Manager) GetCategories() []*TagCategory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var categories []*TagCategory
	for _, cat := range m.categories {
		categories = append(categories, cat)
	}
	return categories
}
