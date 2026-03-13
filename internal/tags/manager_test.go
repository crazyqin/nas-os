package tags

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	// 使用临时数据库
	dbPath := "/tmp/nas-os-tags-test.db"
	os.Remove(dbPath) // 确保开始前清理
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	t.Run("CreateTag", func(t *testing.T) {
		tag, err := mgr.CreateTag(TagInput{
			Name:  "重要",
			Color: "#e74c3c",
			Icon:  "star",
			Group: "优先级",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, tag.ID)
		assert.Equal(t, "重要", tag.Name)
		assert.Equal(t, "#e74c3c", tag.Color)
		assert.Equal(t, "star", tag.Icon)
		assert.Equal(t, "优先级", tag.Group)
		assert.False(t, tag.CreatedAt.IsZero())
		assert.False(t, tag.UpdatedAt.IsZero())
	})

	t.Run("CreateTag_DuplicateName", func(t *testing.T) {
		_, err := mgr.CreateTag(TagInput{Name: "重要"})
		assert.ErrorIs(t, err, ErrTagExists)
	})

	t.Run("GetTag", func(t *testing.T) {
		// 先创建标签
		tag, _ := mgr.CreateTag(TagInput{Name: "测试标签"})

		// 获取标签
		got, err := mgr.GetTag(tag.ID)
		require.NoError(t, err)
		assert.Equal(t, tag.ID, got.ID)
		assert.Equal(t, tag.Name, got.Name)
	})

	t.Run("GetTag_NotFound", func(t *testing.T) {
		_, err := mgr.GetTag("nonexistent")
		assert.ErrorIs(t, err, ErrTagNotFound)
	})

	t.Run("GetTagByName", func(t *testing.T) {
		tag, _ := mgr.CreateTag(TagInput{Name: "文档"})

		got, err := mgr.GetTagByName("文档")
		require.NoError(t, err)
		assert.Equal(t, tag.ID, got.ID)
	})

	t.Run("ListTags", func(t *testing.T) {
		// 创建多个标签
		_, _ = mgr.CreateTag(TagInput{Name: "标签A"})
		_, _ = mgr.CreateTag(TagInput{Name: "标签B"})

		tags, err := mgr.ListTags()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(tags), 2)
	})

	t.Run("ListTagsByGroup", func(t *testing.T) {
		_, _ = mgr.CreateTag(TagInput{Name: "高优先级A", Group: "优先级测试"})
		_, _ = mgr.CreateTag(TagInput{Name: "中优先级B", Group: "优先级测试"})
		_, _ = mgr.CreateTag(TagInput{Name: "照片C", Group: "类型测试"})

		tags, err := mgr.ListTagsByGroup("优先级测试")
		require.NoError(t, err)
		assert.Len(t, tags, 2)

		tags, err = mgr.ListTagsByGroup("类型测试")
		require.NoError(t, err)
		assert.Len(t, tags, 1)
	})

	t.Run("UpdateTag", func(t *testing.T) {
		tag, _ := mgr.CreateTag(TagInput{Name: "待更新"})

		updated, err := mgr.UpdateTag(tag.ID, TagInput{
			Name:  "已更新",
			Color: "#2ecc71",
			Icon:  "check",
			Group: "状态",
		})
		require.NoError(t, err)
		assert.Equal(t, "已更新", updated.Name)
		assert.Equal(t, "#2ecc71", updated.Color)
		assert.Equal(t, "check", updated.Icon)
		assert.Equal(t, "状态", updated.Group)
	})

	t.Run("UpdateTag_NotFound", func(t *testing.T) {
		_, err := mgr.UpdateTag("nonexistent", TagInput{Name: "测试"})
		assert.ErrorIs(t, err, ErrTagNotFound)
	})

	t.Run("DeleteTag", func(t *testing.T) {
		tag, _ := mgr.CreateTag(TagInput{Name: "待删除"})

		err := mgr.DeleteTag(tag.ID)
		require.NoError(t, err)

		// 验证已删除
		_, err = mgr.GetTag(tag.ID)
		assert.ErrorIs(t, err, ErrTagNotFound)
	})

	t.Run("DeleteTag_NotFound", func(t *testing.T) {
		err := mgr.DeleteTag("nonexistent")
		assert.ErrorIs(t, err, ErrTagNotFound)
	})
}

func TestFileTags(t *testing.T) {
	dbPath := "/tmp/nas-os-filetags-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建测试标签
	tag1, _ := mgr.CreateTag(TagInput{Name: "标签1"})
	tag2, _ := mgr.CreateTag(TagInput{Name: "标签2"})
	tag3, _ := mgr.CreateTag(TagInput{Name: "标签3"})

	t.Run("AddTagsToFile", func(t *testing.T) {
		err := mgr.AddTagsToFile("/path/to/file1.txt", []string{tag1.ID, tag2.ID})
		require.NoError(t, err)

		tags, err := mgr.GetTagsForFile("/path/to/file1.txt")
		require.NoError(t, err)
		assert.Len(t, tags, 2)
	})

	t.Run("AddTagsToFile_Duplicate", func(t *testing.T) {
		// 重复添加应该不报错
		err := mgr.AddTagsToFile("/path/to/file1.txt", []string{tag1.ID})
		require.NoError(t, err)
	})

	t.Run("AddTagsToFile_InvalidTagID", func(t *testing.T) {
		err := mgr.AddTagsToFile("/path/to/file.txt", []string{"invalid-id"})
		assert.ErrorIs(t, err, ErrInvalidTagID)
	})

	t.Run("GetTagsForFile", func(t *testing.T) {
		mgr.AddTagsToFile("/path/to/file2.txt", []string{tag1.ID})

		tags, err := mgr.GetTagsForFile("/path/to/file2.txt")
		require.NoError(t, err)
		assert.Len(t, tags, 1)
		assert.Equal(t, tag1.ID, tags[0].ID)
	})

	t.Run("RemoveTagsFromFile", func(t *testing.T) {
		mgr.AddTagsToFile("/path/to/file3.txt", []string{tag1.ID, tag2.ID, tag3.ID})

		err := mgr.RemoveTagsFromFile("/path/to/file3.txt", []string{tag1.ID})
		require.NoError(t, err)

		tags, _ := mgr.GetTagsForFile("/path/to/file3.txt")
		assert.Len(t, tags, 2)
	})

	t.Run("SetFileTags", func(t *testing.T) {
		mgr.AddTagsToFile("/path/to/file4.txt", []string{tag1.ID})

		err := mgr.SetFileTags("/path/to/file4.txt", []string{tag2.ID, tag3.ID})
		require.NoError(t, err)

		tags, _ := mgr.GetTagsForFile("/path/to/file4.txt")
		assert.Len(t, tags, 2)
	})

	t.Run("ClearFileTags", func(t *testing.T) {
		mgr.AddTagsToFile("/path/to/file5.txt", []string{tag1.ID, tag2.ID})

		err := mgr.ClearFileTags("/path/to/file5.txt")
		require.NoError(t, err)

		tags, _ := mgr.GetTagsForFile("/path/to/file5.txt")
		assert.Len(t, tags, 0)
	})
}

func TestGetFilesByTags(t *testing.T) {
	dbPath := "/tmp/nas-os-filesbytags-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建标签
	tag1, _ := mgr.CreateTag(TagInput{Name: "标签A"})
	tag2, _ := mgr.CreateTag(TagInput{Name: "标签B"})
	tag3, _ := mgr.CreateTag(TagInput{Name: "标签C"})

	// 添加文件标签
	// file1: tag1, tag2
	mgr.AddTagsToFile("/data/file1.txt", []string{tag1.ID, tag2.ID})
	// file2: tag1, tag3
	mgr.AddTagsToFile("/data/file2.txt", []string{tag1.ID, tag3.ID})
	// file3: tag2
	mgr.AddTagsToFile("/data/file3.txt", []string{tag2.ID})
	// file4: tag1, tag2, tag3
	mgr.AddTagsToFile("/data/file4.txt", []string{tag1.ID, tag2.ID, tag3.ID})

	_ = tag3 // 使用 tag3

	t.Run("MatchAny", func(t *testing.T) {
		files, err := mgr.GetFilesByTags([]string{tag1.ID}, false)
		require.NoError(t, err)
		assert.Len(t, files, 3) // file1, file2, file4

		files, err = mgr.GetFilesByTags([]string{tag2.ID, tag3.ID}, false)
		require.NoError(t, err)
		assert.Len(t, files, 4) // 所有文件都有 tag2 或 tag3
	})

	t.Run("MatchAll", func(t *testing.T) {
		files, err := mgr.GetFilesByTags([]string{tag1.ID, tag2.ID}, true)
		require.NoError(t, err)
		assert.Len(t, files, 2) // file1, file4

		files, err = mgr.GetFilesByTags([]string{tag1.ID, tag2.ID, tag3.ID}, true)
		require.NoError(t, err)
		assert.Len(t, files, 1) // 只有 file4
	})
}

func TestBatchOperations(t *testing.T) {
	dbPath := "/tmp/nas-os-batch-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	tag1, _ := mgr.CreateTag(TagInput{Name: "批量标签"})

	t.Run("BatchAddTagsToFile", func(t *testing.T) {
		files := []string{"/batch/file1.txt", "/batch/file2.txt", "/batch/file3.txt"}
		err := mgr.BatchAddTagsToFile(files, []string{tag1.ID})
		require.NoError(t, err)

		for _, f := range files {
			tags, _ := mgr.GetTagsForFile(f)
			assert.Len(t, tags, 1)
		}
	})
}

func TestSearch(t *testing.T) {
	dbPath := "/tmp/nas-os-search-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	mgr.CreateTag(TagInput{Name: "重要文档"})
	mgr.CreateTag(TagInput{Name: "重要照片"})
	mgr.CreateTag(TagInput{Name: "临时文件"})

	t.Run("SearchTags", func(t *testing.T) {
		tags, err := mgr.SearchTags("重要")
		require.NoError(t, err)
		assert.Len(t, tags, 2)

		tags, err = mgr.SearchTags("文档")
		require.NoError(t, err)
		assert.Len(t, tags, 1)
	})
}

func TestStats(t *testing.T) {
	dbPath := "/tmp/nas-os-stats-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建标签
	tag1, _ := mgr.CreateTag(TagInput{Name: "标签1", Group: "组1"})
	tag2, _ := mgr.CreateTag(TagInput{Name: "标签2", Group: "组1"})
	mgr.CreateTag(TagInput{Name: "标签3"})

	// 添加文件标签
	mgr.AddTagsToFile("/file1.txt", []string{tag1.ID})
	mgr.AddTagsToFile("/file2.txt", []string{tag2.ID})

	stats, err := mgr.GetStats()
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalTags)
	assert.Equal(t, 2, stats.TotalFiles)
	assert.Equal(t, 2, stats.TotalGrouped)
}

func TestGroups(t *testing.T) {
	dbPath := "/tmp/nas-os-groups-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	mgr.CreateTag(TagInput{Name: "高", Group: "优先级"})
	mgr.CreateTag(TagInput{Name: "中", Group: "优先级"})
	mgr.CreateTag(TagInput{Name: "低", Group: "优先级"})
	mgr.CreateTag(TagInput{Name: "工作", Group: "分类"})
	mgr.CreateTag(TagInput{Name: "个人", Group: "分类"})

	groups, err := mgr.ListGroups()
	require.NoError(t, err)
	assert.Len(t, groups, 2)

	// 验证计数
	for _, g := range groups {
		if g.Name == "优先级" {
			assert.Equal(t, 3, g.Count)
		} else if g.Name == "分类" {
			assert.Equal(t, 2, g.Count)
		}
	}
}

func TestTagUsageCount(t *testing.T) {
	dbPath := "/tmp/nas-os-usage-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	tag, _ := mgr.CreateTag(TagInput{Name: "使用次数测试"})

	count, err := mgr.GetTagUsageCount(tag.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	mgr.AddTagsToFile("/file1.txt", []string{tag.ID})
	mgr.AddTagsToFile("/file2.txt", []string{tag.ID})
	mgr.AddTagsToFile("/file3.txt", []string{tag.ID})

	count, err = mgr.GetTagUsageCount(tag.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestDefaultValues(t *testing.T) {
	dbPath := "/tmp/nas-os-defaults-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建不带颜色和图标的标签
	tag, err := mgr.CreateTag(TagInput{Name: "默认值测试"})
	require.NoError(t, err)
	assert.Equal(t, "#3498db", tag.Color) // 默认颜色
	assert.Empty(t, tag.Icon)
	assert.Empty(t, tag.Group)
}

func TestConcurrentAccess(t *testing.T) {
	dbPath := "/tmp/nas-os-concurrent-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	tag, _ := mgr.CreateTag(TagInput{Name: "并发测试"})

	// 并发添加标签
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			filePath := "/concurrent/file" + string(rune('0'+idx)) + ".txt"
			err := mgr.AddTagsToFile(filePath, []string{tag.ID})
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证
	files, _ := mgr.GetFilesByTags([]string{tag.ID}, false)
	assert.Len(t, files, 10)
}

func TestDeleteTagRemovesFileAssociations(t *testing.T) {
	dbPath := "/tmp/nas-os-cascade-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	tag, _ := mgr.CreateTag(TagInput{Name: "级联删除测试"})
	mgr.AddTagsToFile("/file.txt", []string{tag.ID})

	// 验证文件有标签
	tags, _ := mgr.GetTagsForFile("/file.txt")
	assert.Len(t, tags, 1)

	// 删除标签
	err = mgr.DeleteTag(tag.ID)
	require.NoError(t, err)

	// 验证文件的标签也被移除
	tags, _ = mgr.GetTagsForFile("/file.txt")
	assert.Len(t, tags, 0)
}

func TestSearchFilesByTags(t *testing.T) {
	dbPath := "/tmp/nas-os-search-files-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	tag1, _ := mgr.CreateTag(TagInput{Name: "工作"})
	tag2, _ := mgr.CreateTag(TagInput{Name: "重要"})

	mgr.AddTagsToFile("/documents/report.doc", []string{tag1.ID})
	mgr.AddTagsToFile("/documents/important.doc", []string{tag1.ID, tag2.ID})
	mgr.AddTagsToFile("/photos/vacation.jpg", []string{tag2.ID})

	t.Run("SearchWithKeyword", func(t *testing.T) {
		files, err := mgr.SearchFilesByTags("doc", []string{tag1.ID}, false)
		require.NoError(t, err)
		assert.Len(t, files, 2)
	})

	t.Run("SearchWithKeywordAndMatchAll", func(t *testing.T) {
		files, err := mgr.SearchFilesByTags("important", []string{tag1.ID, tag2.ID}, true)
		require.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Equal(t, "/documents/important.doc", files[0])
	})
}

func TestClearAllTags(t *testing.T) {
	dbPath := "/tmp/nas-os-clear-all-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	mgr.CreateTag(TagInput{Name: "标签1"})
	mgr.CreateTag(TagInput{Name: "标签2"})

	// 验证有标签
	tags, _ := mgr.ListTags()
	assert.Len(t, tags, 2)

	// 清除所有
	err = mgr.ClearAllTags()
	require.NoError(t, err)

	// 验证已清空
	tags, _ = mgr.ListTags()
	assert.Len(t, tags, 0)
}

func TestTimestamps(t *testing.T) {
	dbPath := "/tmp/nas-os-timestamps-test.db"
	os.Remove(dbPath) // 确保开始前清理
	defer os.Remove(dbPath)

	mgr, err := NewManager(dbPath)
	require.NoError(t, err)
	defer mgr.Close()

	beforeCreate := time.Now()
	tag, _ := mgr.CreateTag(TagInput{Name: "时间测试"})
	afterCreate := time.Now()

	// 验证创建时间
	assert.True(t, tag.CreatedAt.After(beforeCreate) || tag.CreatedAt.Equal(beforeCreate))
	assert.True(t, tag.CreatedAt.Before(afterCreate) || tag.CreatedAt.Equal(afterCreate))
	assert.True(t, tag.UpdatedAt.Equal(tag.CreatedAt))

	time.Sleep(100 * time.Millisecond)

	beforeUpdate := time.Now()
	updated, _ := mgr.UpdateTag(tag.ID, TagInput{Name: "已更新"})
	afterUpdate := time.Now()

	// 验证更新时间
	assert.True(t, updated.UpdatedAt.After(beforeUpdate) || updated.UpdatedAt.Equal(beforeUpdate))
	assert.True(t, updated.UpdatedAt.Before(afterUpdate) || updated.UpdatedAt.Equal(afterUpdate))
	assert.True(t, updated.UpdatedAt.After(tag.CreatedAt))
}