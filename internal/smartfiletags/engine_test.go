package smartfiletags

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTagEngine(t *testing.T) {
	engine := NewTagEngine(nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.tags)
	assert.NotNil(t, engine.fileTags)
	assert.NotNil(t, engine.tagFiles)
	assert.NotNil(t, engine.userTags)
	assert.NotNil(t, engine.nameIndex)
	assert.NotNil(t, engine.categories)
	assert.NotNil(t, engine.autoTagger)
}

func TestTagEngine_StartStop(t *testing.T) {
	engine := NewTagEngine(nil)

	err := engine.Start()
	require.NoError(t, err)
	assert.True(t, engine.running)

	err = engine.Start()
	require.NoError(t, err)

	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.running)

	err = engine.Stop()
	require.NoError(t, err)
}

func TestTagEngine_CreateTag(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:       "tag-1",
		Name:     "Important",
		Color:    "#FF0000",
		Category: "priority",
	}

	err := engine.CreateTag(tag)
	require.NoError(t, err)
	assert.Equal(t, 1, engine.stats.TotalTags)

	// 创建重复名称标签
	duplicate := &Tag{
		ID:   "tag-2",
		Name: "Important",
	}
	err = engine.CreateTag(duplicate)
	assert.Error(t, err)
	assert.Equal(t, ErrTagExists, err)

	// 创建无ID标签
	invalid := &Tag{Name: "Invalid"}
	err = engine.CreateTag(invalid)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidTagID, err)
}

func TestTagEngine_UpdateTag(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Original",
	}
	engine.CreateTag(tag)

	tag.Name = "Updated"
	tag.Color = "#00FF00"
	err := engine.UpdateTag(tag)
	require.NoError(t, err)

	updated, _ := engine.GetTag("tag-1")
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "#00FF00", updated.Color)

	// 更新不存在的标签
	invalid := &Tag{ID: "non-existent", Name: "Test"}
	err = engine.UpdateTag(invalid)
	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
}

func TestTagEngine_DeleteTag(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:       "tag-1",
		Name:     "Test",
		Category: "test",
	}
	engine.CreateTag(tag)

	// 添加文件关联
	engine.ApplyTagToFile("file-1", "tag-1", "user-1", false, 1.0)
	engine.ApplyTagToFile("file-2", "tag-1", "user-1", false, 1.0)

	err := engine.DeleteTag("tag-1")
	require.NoError(t, err)
	assert.Equal(t, 0, engine.stats.TotalTags)

	// 验证文件标签已清理
	fileTags := engine.GetFileTags("file-1")
	assert.Len(t, fileTags, 0)

	// 删除不存在的标签
	err = engine.DeleteTag("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
}

func TestTagEngine_GetTag(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Test",
	}
	engine.CreateTag(tag)

	result, err := engine.GetTag("tag-1")
	require.NoError(t, err)
	assert.Equal(t, "Test", result.Name)

	_, err = engine.GetTag("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
}

func TestTagEngine_GetTagByName(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Important",
	}
	engine.CreateTag(tag)

	result, err := engine.GetTagByName("Important")
	require.NoError(t, err)
	assert.Equal(t, "tag-1", result.ID)

	_, err = engine.GetTagByName("non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
}

func TestTagEngine_ListTags(t *testing.T) {
	engine := NewTagEngine(nil)

	tags := []*Tag{
		{ID: "tag-1", Name: "Tag1", Category: "cat1"},
		{ID: "tag-2", Name: "Tag2", Category: "cat1"},
		{ID: "tag-3", Name: "Tag3", Category: "cat2"},
	}

	for _, tag := range tags {
		engine.CreateTag(tag)
	}

	// 列出所有标签
	allTags := engine.ListTags("")
	assert.Len(t, allTags, 3)

	// 按分类列出
	cat1Tags := engine.ListTags("cat1")
	assert.Len(t, cat1Tags, 2)
}

func TestTagEngine_ApplyTagToFile(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Test",
	}
	engine.CreateTag(tag)

	err := engine.ApplyTagToFile("file-1", "tag-1", "user-1", false, 1.0)
	require.NoError(t, err)

	// 重复应用
	err = engine.ApplyTagToFile("file-1", "tag-1", "user-1", false, 1.0)
	require.NoError(t, err) // 应该成功但不重复

	// 检查文件标签
	fileTags := engine.GetFileTags("file-1")
	assert.Len(t, fileTags, 1)

	// 检查标签文件
	tagFiles := engine.GetTagFiles("tag-1")
	assert.Len(t, tagFiles, 1)

	// 应用到不存在的标签
	err = engine.ApplyTagToFile("file-1", "non-existent", "user-1", false, 1.0)
	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
}

func TestTagEngine_RemoveTagFromFile(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Test",
	}
	engine.CreateTag(tag)

	engine.ApplyTagToFile("file-1", "tag-1", "user-1", false, 1.0)

	err := engine.RemoveTagFromFile("file-1", "tag-1")
	require.NoError(t, err)

	fileTags := engine.GetFileTags("file-1")
	assert.Len(t, fileTags, 0)

	// 移除不存在的标签
	err = engine.RemoveTagFromFile("file-1", "tag-1")
	assert.Error(t, err)
	assert.Equal(t, ErrTagNotFound, err)
}

func TestTagEngine_SearchByTag(t *testing.T) {
	engine := NewTagEngine(nil)

	tags := []*Tag{
		{ID: "tag-1", Name: "Important"},
		{ID: "tag-2", Name: "Work"},
	}

	for _, tag := range tags {
		engine.CreateTag(tag)
	}

	engine.ApplyTagToFile("file-1", "tag-1", "user-1", false, 1.0)
	engine.ApplyTagToFile("file-1", "tag-2", "user-1", false, 1.0)
	engine.ApplyTagToFile("file-2", "tag-1", "user-1", false, 1.0)
	engine.ApplyTagToFile("file-3", "tag-2", "user-1", false, 1.0)

	// 搜索单个标签
	files := engine.SearchByTag([]string{"Important"})
	assert.Len(t, files, 2)

	// 搜索多个标签（交集）
	files = engine.SearchByTag([]string{"Important", "Work"})
	assert.Len(t, files, 1)
	assert.Equal(t, "file-1", files[0])

	// 搜索不存在的标签
	files = engine.SearchByTag([]string{"non-existent"})
	assert.Len(t, files, 0)
}

func TestTagEngine_SuggestTags(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Photo",
	}
	engine.CreateTag(tag)

	// 添加关键词
	engine.autoTagger.keywords["Photo"] = []string{"jpg", "png", "image"}

	suggestions := engine.SuggestTags("file-1", "vacation.jpg", "")
	assert.NotNil(t, suggestions)
}

func TestTagEngine_AutoTag(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Document",
	}
	engine.CreateTag(tag)

	// 添加自动标签规则
	rule := &AutoTagRule{
		ID:         "rule-1",
		Name:       "Document Rule",
		TagID:      "tag-1",
		TagName:    "Document",
		Patterns:   []string{"pdf", "doc", "docx"},
		Confidence: 0.8,
		Enabled:    true,
	}
	engine.AddAutoTagRule(rule)

	// 测试自动标签
	applied, err := engine.AutoTag("file-1", "report.pdf", "")
	require.NoError(t, err)
	assert.Len(t, applied, 1)
	assert.Equal(t, "tag-1", applied[0].TagID)

	// 禁用自动标签
	engine.config.AutoTagEnabled = false
	applied, err = engine.AutoTag("file-2", "report.doc", "")
	require.NoError(t, err)
	assert.Len(t, applied, 0)
}

func TestTagEngine_AddAutoTagRule(t *testing.T) {
	engine := NewTagEngine(nil)

	rule := &AutoTagRule{
		ID:       "rule-1",
		Name:     "Test Rule",
		TagID:    "tag-1",
		TagName:  "Test",
		Patterns: []string{"test", "demo"},
		Enabled:  true,
	}

	engine.AddAutoTagRule(rule)
	assert.Len(t, engine.autoTagger.rules, 1)
}

func TestTagEngine_GetStats(t *testing.T) {
	engine := NewTagEngine(nil)

	tags := []*Tag{
		{ID: "tag-1", Name: "Tag1", Category: "cat1"},
		{ID: "tag-2", Name: "Tag2", Category: "cat1"},
		{ID: "tag-3", Name: "Tag3", Category: "cat2"},
	}

	for _, tag := range tags {
		engine.CreateTag(tag)
	}

	engine.ApplyTagToFile("file-1", "tag-1", "user-1", false, 1.0)
	engine.ApplyTagToFile("file-2", "tag-1", "user-1", false, 1.0)
	engine.ApplyTagToFile("file-3", "tag-2", "user-1", false, 1.0)

	stats := engine.GetStats()
	assert.Equal(t, 3, stats.TotalTags)
	assert.Equal(t, 3, stats.TotalFiles)
	assert.Equal(t, 3, stats.TotalRelations)
	assert.Len(t, stats.TopTags, 3)
	assert.Len(t, stats.CategoryStats, 2)
	assert.Equal(t, 2, stats.CategoryStats["cat1"])
	assert.Equal(t, 1, stats.CategoryStats["cat2"])
}

func TestTagEngine_MultipleFiles(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Shared",
	}
	engine.CreateTag(tag)

	// 多个文件同一个标签
	for i := 0; i < 10; i++ {
		fileID := "file-" + string(rune(i+'0'))
		engine.ApplyTagToFile(fileID, "tag-1", "user-1", false, 1.0)
	}

	tagFiles := engine.GetTagFiles("tag-1")
	assert.Len(t, tagFiles, 10)
}

func TestTagEngine_UserTags(t *testing.T) {
	engine := NewTagEngine(nil)

	tags := []*Tag{
		{ID: "tag-1", Name: "Tag1", CreatedBy: "user-1"},
		{ID: "tag-2", Name: "Tag2", CreatedBy: "user-1"},
		{ID: "tag-3", Name: "Tag3", CreatedBy: "user-2"},
	}

	for _, tag := range tags {
		engine.CreateTag(tag)
	}

	userTags := engine.userTags["user-1"]
	assert.Len(t, userTags, 2)

	userTags = engine.userTags["user-2"]
	assert.Len(t, userTags, 1)
}

func TestTagEngine_TagWithTimestamps(t *testing.T) {
	engine := NewTagEngine(nil)

	tag := &Tag{
		ID:   "tag-1",
		Name: "Test",
	}
	engine.CreateTag(tag)

	originalCreatedAt := tag.CreatedAt
	time.Sleep(10 * time.Millisecond)

	tag.Name = "Updated"
	engine.UpdateTag(tag)

	updated, _ := engine.GetTag("tag-1")
	assert.Equal(t, originalCreatedAt, updated.CreatedAt)
	assert.True(t, updated.UpdatedAt.After(originalCreatedAt))
}

func TestTagEngine_PublicPrivateTags(t *testing.T) {
	engine := NewTagEngine(nil)

	publicTag := &Tag{
		ID:       "tag-1",
		Name:     "Public",
		IsPublic: true,
	}
	privateTag := &Tag{
		ID:       "tag-2",
		Name:     "Private",
		IsPublic: false,
	}

	engine.CreateTag(publicTag)
	engine.CreateTag(privateTag)

	allTags := engine.ListTags("")
	assert.Len(t, allTags, 2)
}

func TestTagEngine_SystemTags(t *testing.T) {
	engine := NewTagEngine(nil)

	systemTag := &Tag{
		ID:       "tag-1",
		Name:     "System",
		IsSystem: true,
	}
	userTag := &Tag{
		ID:       "tag-2",
		Name:     "User",
		IsSystem: false,
	}

	engine.CreateTag(systemTag)
	engine.CreateTag(userTag)

	allTags := engine.ListTags("")
	assert.Len(t, allTags, 2)
}

func TestTagEngine_DefaultConfig(t *testing.T) {
	config := DefaultTagConfig()
	assert.Equal(t, 20, config.MaxTagsPerFile)
	assert.Equal(t, 500, config.MaxTagsPerUser)
	assert.True(t, config.AllowUserCreate)
	assert.True(t, config.AutoTagEnabled)
	assert.True(t, config.SuggestionEnabled)
	assert.Equal(t, 10, config.MaxSuggestions)
	assert.Equal(t, "default", config.ColorPalette)
}
