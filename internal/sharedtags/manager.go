package sharedtags

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// TagManager manages tags and categories
type TagManager struct {
	mu         sync.RWMutex
	tags       map[string]*Tag
	categories map[string]*TagCategory
	tagIndex   map[string]map[string]bool // categoryID -> tagID set
	nextID     int64
}

// NewTagManager creates a new TagManager instance
func NewTagManager() *TagManager {
	m := &TagManager{
		tags:       make(map[string]*Tag),
		categories: make(map[string]*TagCategory),
		tagIndex:   make(map[string]map[string]bool),
	}
	// Initialize default categories
	m.initDefaultCategories()
	log.Println("标签管理器已初始化")
	return m
}

// initDefaultCategories creates the built-in category types
func (m *TagManager) initDefaultCategories() {
	defaults := []struct {
		name string
		typ  TagCategoryType
		desc string
	}{
		{"项目", CategoryProject, "项目相关标签分类"},
		{"部门", CategoryDepartment, "部门组织标签分类"},
		{"优先级", CategoryPriority, "优先级标签分类"},
		{"自定义", CategoryCustom, "用户自定义标签分类"},
	}
	for i, d := range defaults {
		cat := &TagCategory{
			ID:          fmt.Sprintf("cat-system-%s", d.typ),
			Name:        d.name,
			Type:        d.typ,
			Description: d.desc,
			Level:       0,
			SortOrder:   i,
			IsSystem:    true,
			Owner:       "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		m.categories[cat.ID] = cat
		m.tagIndex[cat.ID] = make(map[string]bool)
	}
}

// CreateTag creates a new tag
func (m *TagManager) CreateTag(req CreateTagRequest) (*Tag, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check category exists if specified
	if req.CategoryID != "" {
		if _, ok := m.categories[req.CategoryID]; !ok {
			return nil, ErrCategoryNotFound
		}
	}

	// Check duplicate name within same category
	for _, t := range m.tags {
		if t.Name == req.Name && t.CategoryID == req.CategoryID {
			return nil, ErrTagExists
		}
	}

	m.nextID++
	tag := &Tag{
		ID:          fmt.Sprintf("tag-%d", m.nextID),
		Name:        req.Name,
		Description: req.Description,
		CategoryID:  req.CategoryID,
		Color:       req.Color,
		Icon:        req.Icon,
		Owner:       req.Owner,
		IsSystem:    false,
		IsShared:    false,
		UsageCount:  0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    req.Metadata,
	}

	m.tags[tag.ID] = tag

	// Update index
	if tag.CategoryID != "" {
		if _, ok := m.tagIndex[tag.CategoryID]; !ok {
			m.tagIndex[tag.CategoryID] = make(map[string]bool)
		}
		m.tagIndex[tag.CategoryID][tag.ID] = true
	}

	log.Printf("标签已创建: %s (%s)", tag.Name, tag.ID)
	return tag, nil
}

// GetTag retrieves a tag by ID
func (m *TagManager) GetTag(id string) (*Tag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tag, ok := m.tags[id]
	if !ok {
		return nil, ErrTagNotFound
	}
	return tag, nil
}

// UpdateTag updates an existing tag
func (m *TagManager) UpdateTag(id string, req UpdateTagRequest) (*Tag, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[id]
	if !ok {
		return nil, ErrTagNotFound
	}

	if tag.IsSystem {
		return nil, ErrSystemTag
	}

	if req.Name != nil {
		tag.Name = *req.Name
	}
	if req.Description != nil {
		tag.Description = *req.Description
	}
	if req.CategoryID != nil {
		// Remove from old category index
		if tag.CategoryID != "" {
			if idx, ok := m.tagIndex[tag.CategoryID]; ok {
				delete(idx, tag.ID)
			}
		}
		tag.CategoryID = *req.CategoryID
		// Add to new category index
		if tag.CategoryID != "" {
			if _, ok := m.tagIndex[tag.CategoryID]; !ok {
				m.tagIndex[tag.CategoryID] = make(map[string]bool)
			}
			m.tagIndex[tag.CategoryID][tag.ID] = true
		}
	}
	if req.Color != nil {
		tag.Color = *req.Color
	}
	if req.Icon != nil {
		tag.Icon = *req.Icon
	}
	if req.Metadata != nil {
		tag.Metadata = req.Metadata
	}
	tag.UpdatedAt = time.Now()

	log.Printf("标签已更新: %s (%s)", tag.Name, tag.ID)
	return tag, nil
}

// DeleteTag deletes a tag by ID
func (m *TagManager) DeleteTag(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[id]
	if !ok {
		return ErrTagNotFound
	}
	if tag.IsSystem {
		return ErrSystemTag
	}

	// Remove from category index
	if tag.CategoryID != "" {
		if idx, ok := m.tagIndex[tag.CategoryID]; ok {
			delete(idx, tag.ID)
		}
	}

	delete(m.tags, id)
	log.Printf("标签已删除: %s (%s)", tag.Name, tag.ID)
	return nil
}

// ListTags lists all tags with optional category filter
func (m *TagManager) ListTags(categoryID string) []*Tag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Tag
	for _, tag := range m.tags {
		if categoryID == "" || tag.CategoryID == categoryID {
			result = append(result, tag)
		}
	}

	// Sort by usage count descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].UsageCount > result[j].UsageCount
	})

	return result
}

// SearchTags searches tags by keyword
func (m *TagManager) SearchTags(keyword string) []*Tag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyword = strings.ToLower(keyword)
	var result []*Tag

	for _, tag := range m.tags {
		if strings.Contains(strings.ToLower(tag.Name), keyword) ||
			strings.Contains(strings.ToLower(tag.Description), keyword) {
			result = append(result, tag)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UsageCount > result[j].UsageCount
	})

	return result
}

// CreateCategory creates a new tag category
func (m *TagManager) CreateCategory(req CreateCategoryRequest) (*TagCategory, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check duplicate name
	for _, c := range m.categories {
		if c.Name == req.Name && c.ParentID == req.ParentID {
			return nil, ErrCategoryExists
		}
	}

	// Determine level
	level := 0
	if req.ParentID != "" {
		parent, ok := m.categories[req.ParentID]
		if !ok {
			return nil, ErrCategoryNotFound
		}
		level = parent.Level + 1
	}

	m.nextID++
	cat := &TagCategory{
		ID:          fmt.Sprintf("cat-%d", m.nextID),
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		ParentID:    req.ParentID,
		Level:       level,
		SortOrder:   req.SortOrder,
		IsSystem:    false,
		Owner:       req.Owner,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.categories[cat.ID] = cat
	m.tagIndex[cat.ID] = make(map[string]bool)

	log.Printf("标签分类已创建: %s (%s)", cat.Name, cat.ID)
	return cat, nil
}

// GetCategory retrieves a category by ID
func (m *TagManager) GetCategory(id string) (*TagCategory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cat, ok := m.categories[id]
	if !ok {
		return nil, ErrCategoryNotFound
	}
	return cat, nil
}

// UpdateCategory updates an existing category
func (m *TagManager) UpdateCategory(id string, req UpdateCategoryRequest) (*TagCategory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cat, ok := m.categories[id]
	if !ok {
		return nil, ErrCategoryNotFound
	}
	if cat.IsSystem {
		return nil, &TagError{Code: "SYSTEM_CATEGORY", Message: "系统分类不可修改"}
	}

	if req.Name != nil {
		cat.Name = *req.Name
	}
	if req.Description != nil {
		cat.Description = *req.Description
	}
	if req.SortOrder != nil {
		cat.SortOrder = *req.SortOrder
	}
	cat.UpdatedAt = time.Now()

	log.Printf("标签分类已更新: %s (%s)", cat.Name, cat.ID)
	return cat, nil
}

// DeleteCategory deletes a category by ID
func (m *TagManager) DeleteCategory(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cat, ok := m.categories[id]
	if !ok {
		return ErrCategoryNotFound
	}
	if cat.IsSystem {
		return &TagError{Code: "SYSTEM_CATEGORY", Message: "系统分类不可删除"}
	}

	// Check if category has tags
	if tagIDs, ok := m.tagIndex[id]; ok && len(tagIDs) > 0 {
		return &TagError{Code: "CATEGORY_NOT_EMPTY", Message: "分类下还有标签，无法删除"}
	}

	delete(m.tagIndex, id)
	delete(m.categories, id)

	log.Printf("标签分类已删除: %s (%s)", cat.Name, cat.ID)
	return nil
}

// ListCategories lists all categories with optional type filter
func (m *TagManager) ListCategories(categoryType TagCategoryType) []*TagCategory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TagCategory
	for _, cat := range m.categories {
		if categoryType == "" || cat.Type == categoryType {
			result = append(result, cat)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Level != result[j].Level {
			return result[i].Level < result[j].Level
		}
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

// GetChildCategories returns direct child categories of a parent
func (m *TagManager) GetChildCategories(parentID string) []*TagCategory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TagCategory
	for _, cat := range m.categories {
		if cat.ParentID == parentID {
			result = append(result, cat)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})

	return result
}

// IncrementUsage increments the usage count of a tag
func (m *TagManager) IncrementUsage(tagID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tag, ok := m.tags[tagID]
	if !ok {
		return ErrTagNotFound
	}
	tag.UsageCount++
	tag.UpdatedAt = time.Now()
	return nil
}

// GetTagsByIDs returns multiple tags by their IDs
func (m *TagManager) GetTagsByIDs(ids []string) []*Tag {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Tag
	for _, id := range ids {
		if tag, ok := m.tags[id]; ok {
			result = append(result, tag)
		}
	}
	return result
}
