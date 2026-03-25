package lock

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 标签类型定义 ==========

// TagType 标签类型.
type TagType int

const (
	// TagTypeUser 用户自定义标签.
	TagTypeUser TagType = iota
	// TagTypeSystem 系统标签.
	TagTypeSystem
	// TagTypeShared 共享标签（多用户可见）.
	TagTypeShared
	// TagTypeCategory 分类标签.
	TagTypeCategory
	// TagTypeStatus 状态标签.
	TagTypeStatus
)

func (tt TagType) String() string {
	switch tt {
	case TagTypeUser:
		return "user"
	case TagTypeSystem:
		return "system"
	case TagTypeShared:
		return "shared"
	case TagTypeCategory:
		return "category"
	case TagTypeStatus:
		return "status"
	default:
		return "unknown"
	}
}

// ParseTagType 解析标签类型.
func ParseTagType(s string) TagType {
	switch s {
	case "user":
		return TagTypeUser
	case "system":
		return TagTypeSystem
	case "shared":
		return TagTypeShared
	case "category":
		return TagTypeCategory
	case "status":
		return TagTypeStatus
	default:
		return TagTypeUser
	}
}

// TagColor 标签颜色.
type TagColor string

const (
	// TagColorRed 红色.
	TagColorRed TagColor = "red"
	// TagColorOrange 橙色.
	TagColorOrange TagColor = "orange"
	// TagColorYellow 黄色.
	TagColorYellow TagColor = "yellow"
	// TagColorGreen 绿色.
	TagColorGreen TagColor = "green"
	// TagColorBlue 蓝色.
	TagColorBlue TagColor = "blue"
	// TagColorPurple 紫色.
	TagColorPurple TagColor = "purple"
	// TagColorGray 灰色.
	TagColorGray TagColor = "gray"
)

// ========== 文件标签 ==========

// FileTag 文件标签.
type FileTag struct {
	// ID 标签唯一标识
	ID string `json:"id"`
	// Name 标签名称
	Name string `json:"name"`
	// Type 标签类型
	Type TagType `json:"type"`
	// Color 标签颜色
	Color TagColor `json:"color"`
	// Description 标签描述
	Description string `json:"description,omitempty"`
	// Icon 标签图标
	Icon string `json:"icon,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// CreatedBy 创建者
	CreatedBy string `json:"createdBy"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
	// IsShared 是否共享
	IsShared bool `json:"isShared"`
	// SharedWith 共享给的用户列表（空表示所有人可见）
	SharedWith []string `json:"sharedWith,omitempty"`
	// FileCount 关联的文件数量
	FileCount int64 `json:"fileCount"`
	// Metadata 元数据
	Metadata map[string]string `json:"metadata,omitempty"`

	mu sync.RWMutex
}

// NewFileTag 创建新标签.
func NewFileTag(name string, tagType TagType, color TagColor, creator string) *FileTag {
	now := time.Now()
	return &FileTag{
		ID:        uuid.New().String(),
		Name:      name,
		Type:      tagType,
		Color:     color,
		CreatedAt: now,
		CreatedBy: creator,
		UpdatedAt: now,
		Metadata:  make(map[string]string),
	}
}

// Update 更新标签.
func (t *FileTag) Update(name string, color TagColor, description string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Name = name
	t.Color = color
	t.Description = description
	t.UpdatedAt = time.Now()
}

// Share 共享标签.
func (t *FileTag) Share(withUsers []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.IsShared = true
	t.SharedWith = withUsers
	t.Type = TagTypeShared
}

// IsVisibleTo 检查标签是否对用户可见.
func (t *FileTag) IsVisibleTo(userID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 系统标签对所有用户可见
	if t.Type == TagTypeSystem {
		return true
	}

	// 创建者始终可见
	if t.CreatedBy == userID {
		return true
	}

	// 共享标签检查共享列表
	if t.IsShared {
		if len(t.SharedWith) == 0 {
			return true // 共享给所有人
		}
		for _, u := range t.SharedWith {
			if u == userID {
				return true
			}
		}
	}

	return false
}

// ========== 文件标签关联 ==========

// FileTagAssociation 文件标签关联.
type FileTagAssociation struct {
	// ID 关联ID
	ID string `json:"id"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// FileName 文件名
	FileName string `json:"fileName"`
	// TagID 标签ID
	TagID string `json:"tagId"`
	// TagName 标签名称（冗余，便于查询）
	TagName string `json:"tagName"`
	// TagColor 标签颜色（冗余，便于显示）
	TagColor TagColor `json:"tagColor"`
	// AddedBy 添加者
	AddedBy string `json:"addedBy"`
	// AddedAt 添加时间
	AddedAt time.Time `json:"addedAt"`
	// Notes 备注
	Notes string `json:"notes,omitempty"`
}

// NewFileTagAssociation 创建文件标签关联.
func NewFileTagAssociation(filePath, fileName, tagID, tagName string, tagColor TagColor, addedBy string) *FileTagAssociation {
	return &FileTagAssociation{
		ID:       uuid.New().String(),
		FilePath: filePath,
		FileName: fileName,
		TagID:    tagID,
		TagName:  tagName,
		TagColor: tagColor,
		AddedBy:  addedBy,
		AddedAt:  time.Now(),
	}
}

// ========== 标签管理器 ==========

// TagManager 标签管理器.
type TagManager struct {
	// tags 标签存储
	tags sync.Map // map[string]*FileTag

	// fileTags 文件-标签关联
	fileTags sync.Map // map[string][]*FileTagAssociation (key: filePath)

	// tagFiles 标签-文件关联
	tagFiles sync.Map // map[string][]*FileTagAssociation (key: tagID)

	// userTags 用户标签索引
	userTags sync.Map // map[string][]string (key: userID, value: tagIDs)
}

// NewTagManager 创建标签管理器.
func NewTagManager() *TagManager {
	return &TagManager{}
}

// ========== 标签 CRUD ==========

// CreateTag 创建标签.
func (tm *TagManager) CreateTag(name string, tagType TagType, color TagColor, creator string) (*FileTag, error) {
	if name == "" {
		return nil, errors.New("tag name is required")
	}

	// 检查重名
	if existing := tm.FindTagByName(name, creator); existing != nil {
		return nil, errors.New("tag with this name already exists")
	}

	tag := NewFileTag(name, tagType, color, creator)
	tm.tags.Store(tag.ID, tag)

	// 更新用户标签索引
	tm.addToUserIndex(creator, tag.ID)

	return tag, nil
}

// GetTag 获取标签.
func (tm *TagManager) GetTag(tagID string) (*FileTag, error) {
	raw, ok := tm.tags.Load(tagID)
	if !ok {
		return nil, errors.New("tag not found")
	}
	tag, ok := raw.(*FileTag)
	if !ok {
		return nil, errors.New("invalid tag type")
	}
	return tag, nil
}

// UpdateTag 更新标签.
func (tm *TagManager) UpdateTag(tagID, name string, color TagColor, description string, userID string) (*FileTag, error) {
	tag, err := tm.GetTag(tagID)
	if err != nil {
		return nil, err
	}

	// 权限检查：只有创建者可以更新
	if tag.CreatedBy != userID {
		return nil, errors.New("not authorized to update this tag")
	}

	tag.Update(name, color, description)

	// 更新关联中的冗余信息
	tm.updateTagAssociations(tagID, name, color)

	return tag, nil
}

// DeleteTag 删除标签.
func (tm *TagManager) DeleteTag(tagID, userID string) error {
	tag, err := tm.GetTag(tagID)
	if err != nil {
		return err
	}

	// 权限检查：只有创建者可以删除
	if tag.CreatedBy != userID {
		return errors.New("not authorized to delete this tag")
	}

	// 删除所有关联
	tm.removeAllFileAssociations(tagID)

	// 从用户索引中移除
	tm.removeFromUserIndex(tag.CreatedBy, tagID)

	// 删除标签
	tm.tags.Delete(tagID)
	tm.tagFiles.Delete(tagID)

	return nil
}

// ListTags 列出用户可见的标签.
func (tm *TagManager) ListTags(userID string, tagType TagType) []*FileTag {
	var result []*FileTag

	tm.tags.Range(func(key, value interface{}) bool {
		tag, ok := value.(*FileTag)
		if !ok {
			return true
		}

		// 类型过滤
		if tagType != 0 && tag.Type != tagType {
			return true
		}

		// 可见性过滤
		if !tag.IsVisibleTo(userID) {
			return true
		}

		result = append(result, tag)
		return true
	})

	return result
}

// FindTagByName 按名称查找标签.
func (tm *TagManager) FindTagByName(name, userID string) *FileTag {
	var result *FileTag

	tm.tags.Range(func(key, value interface{}) bool {
		tag, ok := value.(*FileTag)
		if !ok {
			return true
		}

		if tag.Name == name && tag.IsVisibleTo(userID) {
			result = tag
			return false
		}
		return true
	})

	return result
}

// ShareTag 共享标签.
func (tm *TagManager) ShareTag(tagID, userID string, withUsers []string) error {
	tag, err := tm.GetTag(tagID)
	if err != nil {
		return err
	}

	// 权限检查
	if tag.CreatedBy != userID {
		return errors.New("not authorized to share this tag")
	}

	tag.Share(withUsers)
	return nil
}

// ========== 文件标签操作 ==========

// AddTagToFile 为文件添加标签.
func (tm *TagManager) AddTagToFile(filePath, fileName, tagID, userID string, notes string) (*FileTagAssociation, error) {
	tag, err := tm.GetTag(tagID)
	if err != nil {
		return nil, err
	}

	// 检查是否已关联
	if tm.IsFileTaggedWith(filePath, tagID) {
		return nil, errors.New("file already has this tag")
	}

	assoc := NewFileTagAssociation(filePath, fileName, tagID, tag.Name, tag.Color, userID)
	assoc.Notes = notes

	// 添加到文件-标签索引
	tm.addToFileTagIndex(filePath, assoc)

	// 添加到标签-文件索引
	tm.addToTagFileIndex(tagID, assoc)

	// 更新标签文件计数
	tag.mu.Lock()
	tag.FileCount++
	tag.mu.Unlock()

	return assoc, nil
}

// RemoveTagFromFile 从文件移除标签.
func (tm *TagManager) RemoveTagFromFile(filePath, tagID, userID string) error {
	// 查找关联
	assoc := tm.findAssociation(filePath, tagID)
	if assoc == nil {
		return errors.New("tag association not found")
	}

	// 权限检查：添加者或标签创建者可以移除
	tag, _ := tm.GetTag(tagID)
	if tag != nil && tag.CreatedBy != userID && assoc.AddedBy != userID {
		return errors.New("not authorized to remove this tag")
	}

	// 从文件-标签索引移除
	tm.removeFromFileTagIndex(filePath, tagID)

	// 从标签-文件索引移除
	tm.removeFromTagFileIndex(tagID, filePath)

	// 更新标签文件计数
	if tag != nil {
		tag.mu.Lock()
		if tag.FileCount > 0 {
			tag.FileCount--
		}
		tag.mu.Unlock()
	}

	return nil
}

// GetFileTags 获取文件的所有标签.
func (tm *TagManager) GetFileTags(filePath string) []*FileTagAssociation {
	raw, ok := tm.fileTags.Load(filePath)
	if !ok {
		return nil
	}
	assocs, ok := raw.(*[]*FileTagAssociation)
	if !ok {
		return nil
	}
	return *assocs
}

// GetTaggedFiles 获取标签关联的所有文件.
func (tm *TagManager) GetTaggedFiles(tagID string) []*FileTagAssociation {
	raw, ok := tm.tagFiles.Load(tagID)
	if !ok {
		return nil
	}
	assocs, ok := raw.(*[]*FileTagAssociation)
	if !ok {
		return nil
	}
	return *assocs
}

// IsFileTaggedWith 检查文件是否有指定标签.
func (tm *TagManager) IsFileTaggedWith(filePath, tagID string) bool {
	assocs := tm.GetFileTags(filePath)
	for _, a := range assocs {
		if a.TagID == tagID {
			return true
		}
	}
	return false
}

// SearchByTags 按标签搜索文件.
func (tm *TagManager) SearchByTags(tagIDs []string, matchAll bool) []string {
	if len(tagIDs) == 0 {
		return nil
	}

	fileSet := make(map[string]int)

	for _, tagID := range tagIDs {
		files := tm.GetTaggedFiles(tagID)
		for _, f := range files {
			fileSet[f.FilePath]++
		}
	}

	var result []string
	if matchAll {
		// 必须匹配所有标签
		for filePath, count := range fileSet {
			if count == len(tagIDs) {
				result = append(result, filePath)
			}
		}
	} else {
		// 匹配任意标签
		for filePath := range fileSet {
			result = append(result, filePath)
		}
	}

	return result
}

// ========== 批量操作 ==========

// BatchAddTags 批量为文件添加标签.
func (tm *TagManager) BatchAddTags(filePaths []string, tagIDs []string, userID string) (int, error) {
	count := 0
	for _, filePath := range filePaths {
		fileName := filePath
		if idx := len(filePath) - 1; idx >= 0 {
			for i := len(filePath) - 1; i >= 0; i-- {
				if filePath[i] == '/' {
					fileName = filePath[i+1:]
					break
				}
			}
		}
		for _, tagID := range tagIDs {
			_, err := tm.AddTagToFile(filePath, fileName, tagID, userID, "")
			if err == nil {
				count++
			}
		}
	}
	return count, nil
}

// BatchRemoveTags 批量移除标签.
func (tm *TagManager) BatchRemoveTags(filePaths []string, tagIDs []string, userID string) (int, error) {
	count := 0
	for _, filePath := range filePaths {
		for _, tagID := range tagIDs {
			err := tm.RemoveTagFromFile(filePath, tagID, userID)
			if err == nil {
				count++
			}
		}
	}
	return count, nil
}

// ========== 内部方法 ==========

func (tm *TagManager) addToFileTagIndex(filePath string, assoc *FileTagAssociation) {
	raw, _ := tm.fileTags.LoadOrStore(filePath, &[]*FileTagAssociation{})
	assocs, ok := raw.(*[]*FileTagAssociation)
	if ok {
		*assocs = append(*assocs, assoc)
	}
}

func (tm *TagManager) removeFromFileTagIndex(filePath, tagID string) {
	raw, ok := tm.fileTags.Load(filePath)
	if !ok {
		return
	}
	assocs, ok := raw.(*[]*FileTagAssociation)
	if !ok {
		return
	}
	for i, a := range *assocs {
		if a.TagID == tagID {
			*assocs = append((*assocs)[:i], (*assocs)[i+1:]...)
			return
		}
	}
}

func (tm *TagManager) addToTagFileIndex(tagID string, assoc *FileTagAssociation) {
	raw, _ := tm.tagFiles.LoadOrStore(tagID, &[]*FileTagAssociation{})
	assocs, ok := raw.(*[]*FileTagAssociation)
	if ok {
		*assocs = append(*assocs, assoc)
	}
}

func (tm *TagManager) removeFromTagFileIndex(tagID, filePath string) {
	raw, ok := tm.tagFiles.Load(tagID)
	if !ok {
		return
	}
	assocs, ok := raw.(*[]*FileTagAssociation)
	if !ok {
		return
	}
	for i, a := range *assocs {
		if a.FilePath == filePath {
			*assocs = append((*assocs)[:i], (*assocs)[i+1:]...)
			return
		}
	}
}

func (tm *TagManager) removeAllFileAssociations(tagID string) {
	raw, ok := tm.tagFiles.Load(tagID)
	if !ok {
		return
	}
	assocs, ok := raw.(*[]*FileTagAssociation)
	if !ok {
		return
	}
	for _, a := range *assocs {
		tm.removeFromFileTagIndex(a.FilePath, tagID)
	}
}

func (tm *TagManager) addToUserIndex(userID, tagID string) {
	raw, _ := tm.userTags.LoadOrStore(userID, &[]string{})
	tagIDs, ok := raw.(*[]string)
	if ok {
		*tagIDs = append(*tagIDs, tagID)
	}
}

func (tm *TagManager) removeFromUserIndex(userID, tagID string) {
	raw, ok := tm.userTags.Load(userID)
	if !ok {
		return
	}
	tagIDs, ok := raw.(*[]string)
	if !ok {
		return
	}
	for i, id := range *tagIDs {
		if id == tagID {
			*tagIDs = append((*tagIDs)[:i], (*tagIDs)[i+1:]...)
			return
		}
	}
}

func (tm *TagManager) updateTagAssociations(tagID, name string, color TagColor) {
	assocs := tm.GetTaggedFiles(tagID)
	for _, a := range assocs {
		a.TagName = name
		a.TagColor = color
	}
}

func (tm *TagManager) findAssociation(filePath, tagID string) *FileTagAssociation {
	assocs := tm.GetFileTags(filePath)
	for _, a := range assocs {
		if a.TagID == tagID {
			return a
		}
	}
	return nil
}

// ========== 标签统计 ==========

// TagStats 标签统计.
type TagStats struct {
	TotalTags    int64            `json:"totalTags"`
	ByType       map[string]int64 `json:"byType"`
	ByColor      map[string]int64 `json:"byColor"`
	TotalTagged  int64            `json:"totalTagged"`
	MostUsedTags []TagUsageCount  `json:"mostUsedTags"`
}

// TagUsageCount 标签使用次数.
type TagUsageCount struct {
	TagID   string `json:"tagId"`
	TagName string `json:"tagName"`
	Count   int64  `json:"count"`
}

// GetTagStats 获取标签统计.
func (tm *TagManager) GetTagStats() *TagStats {
	stats := &TagStats{
		ByType:  make(map[string]int64),
		ByColor: make(map[string]int64),
	}

	tm.tags.Range(func(key, value interface{}) bool {
		tag, ok := value.(*FileTag)
		if !ok {
			return true
		}

		stats.TotalTags++
		stats.ByType[tag.Type.String()]++
		stats.ByColor[string(tag.Color)]++

		tag.mu.RLock()
		stats.TotalTagged += tag.FileCount
		if tag.FileCount > 0 {
			stats.MostUsedTags = append(stats.MostUsedTags, TagUsageCount{
				TagID:   tag.ID,
				TagName: tag.Name,
				Count:   tag.FileCount,
			})
		}
		tag.mu.RUnlock()

		return true
	})

	// 排序最常用标签
	for i := 0; i < len(stats.MostUsedTags)-1; i++ {
		for j := i + 1; j < len(stats.MostUsedTags); j++ {
			if stats.MostUsedTags[j].Count > stats.MostUsedTags[i].Count {
				stats.MostUsedTags[i], stats.MostUsedTags[j] = stats.MostUsedTags[j], stats.MostUsedTags[i]
			}
		}
	}

	if len(stats.MostUsedTags) > 10 {
		stats.MostUsedTags = stats.MostUsedTags[:10]
	}

	return stats
}

// ========== 预定义标签 ==========

// PredefinedTags 预定义系统标签.
var PredefinedTags = []struct {
	Name        string
	Type        TagType
	Color       TagColor
	Description string
}{
	{"重要", TagTypeSystem, TagColorRed, "重要文件"},
	{"待办", TagTypeSystem, TagColorOrange, "待处理文件"},
	{"已完成", TagTypeSystem, TagColorGreen, "已完成文件"},
	{"存档", TagTypeSystem, TagColorGray, "已归档文件"},
	{"共享", TagTypeSystem, TagColorBlue, "共享文件"},
	{"草稿", TagTypeSystem, TagColorYellow, "草稿文件"},
}

// InitPredefinedTags 初始化预定义标签.
func (tm *TagManager) InitPredefinedTags() {
	for _, pt := range PredefinedTags {
		tag := NewFileTag(pt.Name, pt.Type, pt.Color, "system")
		tag.Description = pt.Description
		tm.tags.Store(tag.ID, tag)
	}
}

// ========== 标签 API 响应类型 ==========

// TagInfo 标签信息（API响应）.
type TagInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Color       string            `json:"color"`
	Description string            `json:"description,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	CreatedBy   string            `json:"createdBy"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	IsShared    bool              `json:"isShared"`
	SharedWith  []string          `json:"sharedWith,omitempty"`
	FileCount   int64             `json:"fileCount"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ToInfo 转换为TagInfo.
func (t *FileTag) ToInfo() *TagInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &TagInfo{
		ID:          t.ID,
		Name:        t.Name,
		Type:        t.Type.String(),
		Color:       string(t.Color),
		Description: t.Description,
		Icon:        t.Icon,
		CreatedAt:   t.CreatedAt,
		CreatedBy:   t.CreatedBy,
		UpdatedAt:   t.UpdatedAt,
		IsShared:    t.IsShared,
		SharedWith:  t.SharedWith,
		FileCount:   t.FileCount,
		Metadata:    t.Metadata,
	}
}

// TagRequest 标签请求.
type TagRequest struct {
	Name        string   `json:"name" binding:"required"`
	Type        TagType  `json:"type"`
	Color       TagColor `json:"color"`
	Description string   `json:"description,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	IsShared    bool     `json:"isShared"`
	SharedWith  []string `json:"sharedWith,omitempty"`
}

// AddTagRequest 添加标签到文件请求.
type AddTagRequest struct {
	FilePath string `json:"filePath" binding:"required"`
	FileName string `json:"fileName"`
	TagID    string `json:"tagId" binding:"required"`
	Notes    string `json:"notes,omitempty"`
}

// SearchByTagsRequest 按标签搜索请求.
type SearchByTagsRequest struct {
	TagIDs   []string `json:"tagIds" binding:"required"`
	MatchAll bool     `json:"matchAll"`
}

// BatchTagRequest 批量标签请求.
type BatchTagRequest struct {
	FilePaths []string `json:"filePaths" binding:"required"`
	TagIDs    []string `json:"tagIds" binding:"required"`
}

// ========== 上下文支持 ==========

// TagManagerWithLock 带锁管理的标签管理器.
type TagManagerWithLock struct {
	tagManager  *TagManager
	lockManager *Manager
}

// NewTagManagerWithLock 创建带锁的标签管理器.
func NewTagManagerWithLock(tagManager *TagManager, lockManager *Manager) *TagManagerWithLock {
	return &TagManagerWithLock{
		tagManager:  tagManager,
		lockManager: lockManager,
	}
}

// AddTagToFileSafe 安全地为文件添加标签（带锁检查）.
func (tm *TagManagerWithLock) AddTagToFileSafe(ctx context.Context, filePath, tagID, userID string) (*FileTagAssociation, error) {
	// 检查文件是否被锁定
	if tm.lockManager.IsLocked(filePath) {
		info, err := tm.lockManager.GetLockByPath(filePath)
		if err == nil && info.Owner != userID {
			return nil, errors.New("file is locked by another user")
		}
	}

	fileName := filePath
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' {
			fileName = filePath[i+1:]
			break
		}
	}

	return tm.tagManager.AddTagToFile(filePath, fileName, tagID, userID, "")
}

// RemoveTagFromFileSafe 安全地从文件移除标签（带锁检查）.
func (tm *TagManagerWithLock) RemoveTagFromFileSafe(ctx context.Context, filePath, tagID, userID string) error {
	// 检查文件是否被锁定
	if tm.lockManager.IsLocked(filePath) {
		info, err := tm.lockManager.GetLockByPath(filePath)
		if err == nil && info.Owner != userID {
			return errors.New("file is locked by another user")
		}
	}

	return tm.tagManager.RemoveTagFromFile(filePath, tagID, userID)
}
