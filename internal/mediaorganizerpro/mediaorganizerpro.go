// mediaorganizerpro 智能媒体库管理Pro模块
// 提供智能媒体文件分类、标签管理、元数据提取、重复检测、智能推荐功能
package mediaorganizerpro

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MediaType 媒体类型
type MediaType string

const (
	// MediaTypeImage 图片类型
	MediaTypeImage MediaType = "image"
	// MediaTypeVideo 视频类型
	MediaTypeVideo MediaType = "video"
	// MediaTypeAudio 音频类型
	MediaTypeAudio MediaType = "audio"
)

// MediaItem 媒体文件项
type MediaItem struct {
	// ID 媒体唯一标识
	ID string
	// Name 文件名
	Name string
	// Path 文件路径
	Path string
	// Type 媒体类型
	Type MediaType
	// Size 文件大小(字节)
	Size int64
	// PerceptualHash 感知哈希值(用于重复检测)
	PerceptualHash string
	// Tags 标签列表
	Tags []string
	// Metadata 元数据
	Metadata map[string]string
	// CreatedAt 创建时间
	CreatedAt time.Time
	// UpdatedAt 更新时间
	UpdatedAt time.Time
	// AccessCount 访问次数
	AccessCount int
	// LastAccessedAt 最后访问时间
	LastAccessedAt time.Time
}

// Library 媒体库
type Library struct {
	// ID 库唯一标识
	ID string
	// Name 库名
	Name string
	// Path 库路径
	Path string
	// Items 媒体文件列表
	Items map[string]*MediaItem
	// CreatedAt 创建时间
	CreatedAt time.Time
	// UpdatedAt 更新时间
	UpdatedAt time.Time
}

// DuplicateGroup 重复媒体组
type DuplicateGroup struct {
	// Hash 感知哈希
	Hash string
	// Items 重复的媒体文件列表
	Items []*MediaItem
}

// LibraryStats 媒体库统计信息
type LibraryStats struct {
	// TotalItems 总文件数
	TotalItems int
	// TotalSize 总大小(字节)
	TotalSize int64
	// ImageCount 图片数量
	ImageCount int
	// VideoCount 视频数量
	VideoCount int
	// AudioCount 音频数量
	AudioCount int
	// TagCount 标签数量
	TagCount int
	// DuplicateCount 重复文件数量
	DuplicateCount int
}

// MediaOrganizerPro 智能媒体库管理Pro主结构体
type MediaOrganizerPro struct {
	// mu 读写锁
	mu sync.RWMutex
	// libraries 媒体库集合
	libraries map[string]*Library
	// tagIndex 标签索引
	tagIndex map[string]map[string]*MediaItem // tag -> mediaID -> item
	// userPreferences 用户偏好(用于推荐)
	userPreferences map[string]int // tag -> weight
}

// NewMediaOrganizerPro 创建新的智能媒体库管理器实例
func NewMediaOrganizerPro() *MediaOrganizerPro {
	return &MediaOrganizerPro{
		libraries:       make(map[string]*Library),
		tagIndex:        make(map[string]map[string]*MediaItem),
		userPreferences: make(map[string]int),
	}
}

// CreateLibrary 创建媒体库
func (m *MediaOrganizerPro) CreateLibrary(id, name, path string) (*Library, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.libraries[id]; exists {
		return nil, fmt.Errorf("媒体库 %s 已存在", id)
	}

	library := &Library{
		ID:        id,
		Name:      name,
		Path:      path,
		Items:     make(map[string]*MediaItem),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.libraries[id] = library
	return library, nil
}

// ScanLibrary 扫描媒体库
func (m *MediaOrganizerPro) ScanLibrary(libraryID string, items []*MediaItem) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return 0, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	addedCount := 0
	for _, item := range items {
		if _, exists := library.Items[item.ID]; !exists {
			now := time.Now()
			if item.CreatedAt.IsZero() {
				item.CreatedAt = now
			}
			item.UpdatedAt = now
			library.Items[item.ID] = item
			addedCount++

			// 更新标签索引
			for _, tag := range item.Tags {
				if m.tagIndex[tag] == nil {
					m.tagIndex[tag] = make(map[string]*MediaItem)
				}
				m.tagIndex[tag][item.ID] = item
			}
		}
	}

	library.UpdatedAt = time.Now()
	return addedCount, nil
}

// AutoClassify 智能分类方法
// 按类型/日期/人物/场景自动分类媒体文件
func (m *MediaOrganizerPro) AutoClassify(libraryID string) (map[string][]*MediaItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return nil, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	result := make(map[string][]*MediaItem)

	for _, item := range library.Items {
		// 按类型分类
		typeKey := "type_" + string(item.Type)
		result[typeKey] = append(result[typeKey], item)

		// 按日期分类(按年月)
		dateKey := "date_" + item.CreatedAt.Format("2006-01")
		result[dateKey] = append(result[dateKey], item)

		// 按场景分类(从元数据提取)
		if scene, ok := item.Metadata["scene"]; ok {
			sceneKey := "scene_" + scene
			result[sceneKey] = append(result[sceneKey], item)
		}

		// 按人物分类(从元数据提取)
		if people, ok := item.Metadata["people"]; ok {
			peopleKey := "people_" + people
			result[peopleKey] = append(result[peopleKey], item)
		}
	}

	return result, nil
}

// AddTag 添加标签
func (m *MediaOrganizerPro) AddTag(libraryID, mediaID string, tags ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	item, exists := library.Items[mediaID]
	if !exists {
		return fmt.Errorf("媒体文件 %s 不存在", mediaID)
	}

	for _, tag := range tags {
		// 检查标签是否已存在
		found := false
		for _, existingTag := range item.Tags {
			if existingTag == tag {
				found = true
				break
			}
		}

		if !found {
			item.Tags = append(item.Tags, tag)

			// 更新标签索引
			if m.tagIndex[tag] == nil {
				m.tagIndex[tag] = make(map[string]*MediaItem)
			}
			m.tagIndex[tag][mediaID] = item

			// 更新用户偏好
			m.userPreferences[tag]++
		}
	}

	item.UpdatedAt = time.Now()
	return nil
}

// RemoveTag 移除标签
func (m *MediaOrganizerPro) RemoveTag(libraryID, mediaID string, tags ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	item, exists := library.Items[mediaID]
	if !exists {
		return fmt.Errorf("媒体文件 %s 不存在", mediaID)
	}

	for _, tag := range tags {
		// 从标签列表移除
		newTags := make([]string, 0, len(item.Tags))
		for _, existingTag := range item.Tags {
			if existingTag != tag {
				newTags = append(newTags, existingTag)
			}
		}
		item.Tags = newTags

		// 更新标签索引
		if tagItems, ok := m.tagIndex[tag]; ok {
			delete(tagItems, mediaID)
			if len(tagItems) == 0 {
				delete(m.tagIndex, tag)
			}
		}

		// 更新用户偏好
		if m.userPreferences[tag] > 0 {
			m.userPreferences[tag]--
		}
	}

	item.UpdatedAt = time.Now()
	return nil
}

// SearchByTag 按标签搜索媒体文件
func (m *MediaOrganizerPro) SearchByTag(tags ...string) []*MediaItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(tags) == 0 {
		return nil
	}

	// 获取第一个标签的媒体集合
	firstTagItems, exists := m.tagIndex[tags[0]]
	if !exists {
		return nil
	}

	// 初始化结果集
	resultIDs := make(map[string]*MediaItem)
	for id, item := range firstTagItems {
		resultIDs[id] = item
	}

	// 与其他标签取交集
	for _, tag := range tags[1:] {
		tagItems, exists := m.tagIndex[tag]
		if !exists {
			return nil
		}

		// 取交集
		newResult := make(map[string]*MediaItem)
		for id, item := range resultIDs {
			if _, ok := tagItems[id]; ok {
				newResult[id] = item
			}
		}
		resultIDs = newResult
	}

	// 转换为切片
	result := make([]*MediaItem, 0, len(resultIDs))
	for _, item := range resultIDs {
		result = append(result, item)
	}

	// 按访问次数排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].AccessCount > result[j].AccessCount
	})

	return result
}

// DetectDuplicates 检测重复媒体文件
// 基于感知哈希进行检测
func (m *MediaOrganizerPro) DetectDuplicates(libraryID string) ([]*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return nil, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	// 按哈希分组
	hashGroups := make(map[string][]*MediaItem)
	for _, item := range library.Items {
		if item.PerceptualHash != "" {
			hashGroups[item.PerceptualHash] = append(hashGroups[item.PerceptualHash], item)
		}
	}

	// 过滤出有重复的组
	var duplicates []*DuplicateGroup
	for hash, items := range hashGroups {
		if len(items) > 1 {
			duplicates = append(duplicates, &DuplicateGroup{
				Hash:  hash,
				Items: items,
			})
		}
	}

	// 按组内文件数量排序
	sort.Slice(duplicates, func(i, j int) bool {
		return len(duplicates[i].Items) > len(duplicates[j].Items)
	})

	return duplicates, nil
}

// ExtractMetadata 提取元数据
func (m *MediaOrganizerPro) ExtractMetadata(libraryID, mediaID string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return nil, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	item, exists := library.Items[mediaID]
	if !exists {
		return nil, fmt.Errorf("媒体文件 %s 不存在", mediaID)
	}

	// 返回元数据的副本
	result := make(map[string]string)
	for k, v := range item.Metadata {
		result[k] = v
	}

	// 添加自动检测的元数据
	result["file_name"] = item.Name
	result["file_type"] = string(item.Type)
	result["file_size"] = fmt.Sprintf("%d", item.Size)
	result["created_at"] = item.CreatedAt.Format(time.RFC3339)
	result["updated_at"] = item.UpdatedAt.Format(time.RFC3339)
	result["tag_count"] = fmt.Sprintf("%d", len(item.Tags))
	result["access_count"] = fmt.Sprintf("%d", item.AccessCount)

	return result, nil
}

// UpdateMetadata 更新元数据
func (m *MediaOrganizerPro) UpdateMetadata(libraryID, mediaID string, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	item, exists := library.Items[mediaID]
	if !exists {
		return fmt.Errorf("媒体文件 %s 不存在", mediaID)
	}

	if item.Metadata == nil {
		item.Metadata = make(map[string]string)
	}

	for k, v := range metadata {
		item.Metadata[k] = v
	}

	item.UpdatedAt = time.Now()
	return nil
}

// Recommend 智能推荐方法
// 基于用户习惯推荐媒体文件
func (m *MediaOrganizerPro) Recommend(libraryID string, limit int) ([]*MediaItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return nil, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	// 计算每个媒体文件的推荐分数
	type scoredItem struct {
		item  *MediaItem
		score float64
	}

	var scoredItems []scoredItem
	for _, item := range library.Items {
		score := 0.0

		// 基于用户偏好的分数
		for _, tag := range item.Tags {
			if weight, ok := m.userPreferences[tag]; ok {
				score += float64(weight)
			}
		}

		// 基于访问次数的分数
		score += float64(item.AccessCount) * 0.5

		// 基于最近访问时间的分数
		if !item.LastAccessedAt.IsZero() {
			hoursSinceAccess := time.Since(item.LastAccessedAt).Hours()
			if hoursSinceAccess < 24 {
				score += 2.0
			} else if hoursSinceAccess < 168 { // 一周内
				score += 1.0
			}
		}

		scoredItems = append(scoredItems, scoredItem{item: item, score: score})
	}

	// 按分数排序
	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].score > scoredItems[j].score
	})

	// 返回推荐结果
	result := make([]*MediaItem, 0, limit)
	for i, si := range scoredItems {
		if i >= limit {
			break
		}
		result = append(result, si.item)
	}

	return result, nil
}

// GetLibraryStats 获取媒体库统计信息
func (m *MediaOrganizerPro) GetLibraryStats(libraryID string) (*LibraryStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return nil, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	stats := &LibraryStats{
		TotalItems: len(library.Items),
	}

	// 统计标签数量
	tagSet := make(map[string]bool)

	for _, item := range library.Items {
		// 按类型统计
		switch item.Type {
		case MediaTypeImage:
			stats.ImageCount++
		case MediaTypeVideo:
			stats.VideoCount++
		case MediaTypeAudio:
			stats.AudioCount++
		}

		// 累加大小
		stats.TotalSize += item.Size

		// 收集标签
		for _, tag := range item.Tags {
			tagSet[tag] = true
		}
	}

	stats.TagCount = len(tagSet)

	// 检测重复文件
	hashCount := make(map[string]int)
	for _, item := range library.Items {
		if item.PerceptualHash != "" {
			hashCount[item.PerceptualHash]++
		}
	}

	for _, count := range hashCount {
		if count > 1 {
			stats.DuplicateCount += count
		}
	}

	return stats, nil
}

// GetLibrary 获取媒体库
func (m *MediaOrganizerPro) GetLibrary(libraryID string) (*Library, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return nil, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	return library, nil
}

// ListLibraries 列出所有媒体库
func (m *MediaOrganizerPro) ListLibraries() []*Library {
	m.mu.RLock()
	defer m.mu.RUnlock()

	libraries := make([]*Library, 0, len(m.libraries))
	for _, lib := range m.libraries {
		libraries = append(libraries, lib)
	}

	// 按名称排序
	sort.Slice(libraries, func(i, j int) bool {
		return libraries[i].Name < libraries[j].Name
	})

	return libraries
}

// SearchMedia 搜索媒体文件
func (m *MediaOrganizerPro) SearchMedia(libraryID, keyword string) ([]*MediaItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return nil, fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	keyword = strings.ToLower(keyword)
	var results []*MediaItem

	for _, item := range library.Items {
		// 在文件名中搜索
		if strings.Contains(strings.ToLower(item.Name), keyword) {
			results = append(results, item)
			continue
		}

		// 在标签中搜索
		for _, tag := range item.Tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				results = append(results, item)
				break
			}
		}

		// 在元数据中搜索
		for _, v := range item.Metadata {
			if strings.Contains(strings.ToLower(v), keyword) {
				results = append(results, item)
				break
			}
		}
	}

	// 按更新时间排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})

	return results, nil
}

// RecordAccess 记录媒体访问
func (m *MediaOrganizerPro) RecordAccess(libraryID, mediaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	item, exists := library.Items[mediaID]
	if !exists {
		return fmt.Errorf("媒体文件 %s 不存在", mediaID)
	}

	item.AccessCount++
	item.LastAccessedAt = time.Now()

	// 更新用户偏好
	for _, tag := range item.Tags {
		m.userPreferences[tag]++
	}

	return nil
}

// DeleteMedia 删除媒体文件
func (m *MediaOrganizerPro) DeleteMedia(libraryID, mediaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	item, exists := library.Items[mediaID]
	if !exists {
		return fmt.Errorf("媒体文件 %s 不存在", mediaID)
	}

	// 从标签索引中移除
	for _, tag := range item.Tags {
		if tagItems, ok := m.tagIndex[tag]; ok {
			delete(tagItems, mediaID)
			if len(tagItems) == 0 {
				delete(m.tagIndex, tag)
			}
		}
	}

	// 从媒体库中移除
	delete(library.Items, mediaID)
	library.UpdatedAt = time.Now()

	return nil
}

// ExportLibrary 导出媒体库信息
func (m *MediaOrganizerPro) ExportLibrary(libraryID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	library, exists := m.libraries[libraryID]
	if !exists {
		return "", fmt.Errorf("媒体库 %s 不存在", libraryID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("媒体库: %s\n", library.Name))
	sb.WriteString(fmt.Sprintf("路径: %s\n", library.Path))
	sb.WriteString(fmt.Sprintf("创建时间: %s\n", library.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("媒体文件数量: %d\n\n", len(library.Items)))

	for _, item := range library.Items {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", item.Type, item.Name))
		sb.WriteString(fmt.Sprintf("  路径: %s\n", item.Path))
		sb.WriteString(fmt.Sprintf("  大小: %d bytes\n", item.Size))
		if len(item.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("  标签: %s\n", strings.Join(item.Tags, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
