package tags

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestLabelManager(t *testing.T) (*LabelManager, func()) {
	t.Helper()
	dbPath := "/tmp/nas-os-shared-labels-test.db"
	os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	mgr, err := NewLabelManager(db)
	require.NoError(t, err)

	cleanup := func() {
		mgr.Close()
		db.Close()
		os.Remove(dbPath)
	}
	return mgr, cleanup
}

func TestSharedLabelCRUD(t *testing.T) {
	mgr, cleanup := setupTestLabelManager(t)
	defer cleanup()

	t.Run("CreateLabel", func(t *testing.T) {
		label, err := mgr.CreateLabel(SharedLabelInput{
			Name:        "重要文件",
			Color:       "#e74c3c",
			Description: "标记重要文件",
			Owner:       "user1",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, label.ID)
		assert.Equal(t, "重要文件", label.Name)
		assert.Equal(t, "#e74c3c", label.Color)
		assert.Equal(t, "标记重要文件", label.Description)
		assert.Equal(t, "user1", label.Owner)
		assert.Empty(t, label.SharedWith)
	})

	t.Run("CreateLabel_DuplicateName", func(t *testing.T) {
		_, err := mgr.CreateLabel(SharedLabelInput{Name: "重要文件", Owner: "user1"})
		assert.ErrorIs(t, err, ErrSharedLabelExists)
	})

	t.Run("CreateLabel_SameName_DiffOwner", func(t *testing.T) {
		label, err := mgr.CreateLabel(SharedLabelInput{Name: "重要文件", Owner: "user2"})
		require.NoError(t, err)
		assert.Equal(t, "user2", label.Owner)
	})

	t.Run("GetLabel", func(t *testing.T) {
		label, _ := mgr.CreateLabel(SharedLabelInput{Name: "查询测试", Owner: "user1"})
		got, err := mgr.GetLabel(label.ID)
		require.NoError(t, err)
		assert.Equal(t, label.ID, got.ID)
		assert.Equal(t, "查询测试", got.Name)
	})

	t.Run("GetLabel_NotFound", func(t *testing.T) {
		_, err := mgr.GetLabel("nonexistent")
		assert.ErrorIs(t, err, ErrSharedLabelNotFound)
	})

	t.Run("UpdateLabel", func(t *testing.T) {
		label, _ := mgr.CreateLabel(SharedLabelInput{Name: "待更新", Owner: "user1"})
		updated, err := mgr.UpdateLabel(label.ID, SharedLabelInput{
			Name:        "已更新",
			Color:       "#2ecc71",
			Description: "更新后的描述",
			Owner:       "user1",
		})
		require.NoError(t, err)
		assert.Equal(t, "已更新", updated.Name)
		assert.Equal(t, "#2ecc71", updated.Color)
		assert.Equal(t, "更新后的描述", updated.Description)
	})

	t.Run("UpdateLabel_NotOwner", func(t *testing.T) {
		label, _ := mgr.CreateLabel(SharedLabelInput{Name: "权限测试", Owner: "user1"})
		_, err := mgr.UpdateLabel(label.ID, SharedLabelInput{Name: "尝试修改", Owner: "user2"})
		assert.ErrorIs(t, err, ErrNotSharedOwner)
	})

	t.Run("DeleteLabel", func(t *testing.T) {
		label, _ := mgr.CreateLabel(SharedLabelInput{Name: "待删除", Owner: "user1"})
		err := mgr.DeleteLabel(label.ID, "user1")
		require.NoError(t, err)

		_, err = mgr.GetLabel(label.ID)
		assert.ErrorIs(t, err, ErrSharedLabelNotFound)
	})

	t.Run("DeleteLabel_NotOwner", func(t *testing.T) {
		label, _ := mgr.CreateLabel(SharedLabelInput{Name: "删除权限测试", Owner: "user1"})
		err := mgr.DeleteLabel(label.ID, "user2")
		assert.ErrorIs(t, err, ErrNotSharedOwner)
	})
}

func TestShareLabel(t *testing.T) {
	mgr, cleanup := setupTestLabelManager(t)
	defer cleanup()

	// 创建标签
	label, err := mgr.CreateLabel(SharedLabelInput{
		Name:  "共享测试",
		Owner: "owner1",
	})
	require.NoError(t, err)

	t.Run("ShareLabel", func(t *testing.T) {
		err := mgr.ShareLabel(label.ID, []string{"user1", "user2", "user3"}, "owner1")
		require.NoError(t, err)

		// 验证分享列表
		got, _ := mgr.GetLabel(label.ID)
		assert.Len(t, got.SharedWith, 3)
		assert.Contains(t, got.SharedWith, "user1")
		assert.Contains(t, got.SharedWith, "user2")
		assert.Contains(t, got.SharedWith, "user3")
	})

	t.Run("ShareLabel_SkipSelf", func(t *testing.T) {
		// 分享给自己应该被跳过
		err := mgr.ShareLabel(label.ID, []string{"owner1", "user4"}, "owner1")
		require.NoError(t, err)

		got, _ := mgr.GetLabel(label.ID)
		assert.Contains(t, got.SharedWith, "user4")
		// owner1 不应该出现在分享列表中
		for _, u := range got.SharedWith {
			assert.NotEqual(t, "owner1", u)
		}
	})

	t.Run("ShareLabel_DuplicateIgnored", func(t *testing.T) {
		err := mgr.ShareLabel(label.ID, []string{"user1"}, "owner1")
		require.NoError(t, err) // INSERT OR IGNORE 不报错
	})

	t.Run("ShareLabel_NotOwner", func(t *testing.T) {
		err := mgr.ShareLabel(label.ID, []string{"user5"}, "notowner")
		assert.ErrorIs(t, err, ErrNotSharedOwner)
	})

	t.Run("UnshareLabel", func(t *testing.T) {
		err := mgr.UnshareLabel(label.ID, []string{"user1", "user2"}, "owner1")
		require.NoError(t, err)

		got, _ := mgr.GetLabel(label.ID)
		assert.NotContains(t, got.SharedWith, "user1")
		assert.NotContains(t, got.SharedWith, "user2")
		assert.Contains(t, got.SharedWith, "user3") // user3 仍保留
	})

	t.Run("ListLabels_SeesShared", func(t *testing.T) {
		// user3 能看到被分享的标签
		labels, err := mgr.ListLabels("user3")
		require.NoError(t, err)

		found := false
		for _, l := range labels {
			if l.ID == label.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "user3 应该能看到被分享的标签")
	})

	t.Run("ListLabels_OwnerSeesOwn", func(t *testing.T) {
		labels, err := mgr.ListLabels("owner1")
		require.NoError(t, err)

		found := false
		for _, l := range labels {
			if l.ID == label.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "owner1 应该能看到自己的标签")
	})

	t.Run("ListLabels_UnrelatedUserSeesNothing", func(t *testing.T) {
		labels, err := mgr.ListLabels("stranger")
		require.NoError(t, err)

		for _, l := range labels {
			assert.NotEqual(t, label.ID, l.ID)
		}
	})
}

func TestLabelFileAssociation(t *testing.T) {
	mgr, cleanup := setupTestLabelManager(t)
	defer cleanup()

	label, _ := mgr.CreateLabel(SharedLabelInput{
		Name:  "文件关联测试",
		Owner: "user1",
	})

	t.Run("AssignLabel", func(t *testing.T) {
		err := mgr.AssignLabel("file-001", label.ID)
		require.NoError(t, err)

		files, err := mgr.GetFilesByLabel(label.ID)
		require.NoError(t, err)
		assert.Contains(t, files, "file-001")
	})

	t.Run("AssignLabel_DuplicateIgnored", func(t *testing.T) {
		err := mgr.AssignLabel("file-001", label.ID)
		require.NoError(t, err) // INSERT OR IGNORE

		files, _ := mgr.GetFilesByLabel(label.ID)
		count := 0
		for _, f := range files {
			if f == "file-001" {
				count++
			}
		}
		assert.Equal(t, 1, count, "不应有重复记录")
	})

	t.Run("AssignLabel_MultipleFiles", func(t *testing.T) {
		_ = mgr.AssignLabel("file-002", label.ID)
		_ = mgr.AssignLabel("file-003", label.ID)

		files, err := mgr.GetFilesByLabel(label.ID)
		require.NoError(t, err)
		assert.Len(t, files, 3)
	})

	t.Run("AssignLabel_InvalidLabel", func(t *testing.T) {
		err := mgr.AssignLabel("file-999", "nonexistent-label")
		assert.ErrorIs(t, err, ErrSharedLabelNotFound)
	})

	t.Run("RemoveLabel", func(t *testing.T) {
		err := mgr.RemoveLabel("file-002", label.ID)
		require.NoError(t, err)

		files, _ := mgr.GetFilesByLabel(label.ID)
		assert.NotContains(t, files, "file-002")
		assert.Len(t, files, 2)
	})

	t.Run("RemoveLabel_NotAssigned", func(t *testing.T) {
		err := mgr.RemoveLabel("file-999", label.ID)
		assert.ErrorIs(t, err, ErrLabelNotAssigned)
	})

	t.Run("DeleteLabel_CascadesFileAssociations", func(t *testing.T) {
		// 创建新标签并关联文件
		tempLabel, _ := mgr.CreateLabel(SharedLabelInput{Name: "级联测试", Owner: "user1"})
		_ = mgr.AssignLabel("cascade-file-1", tempLabel.ID)
		_ = mgr.AssignLabel("cascade-file-2", tempLabel.ID)

		files, _ := mgr.GetFilesByLabel(tempLabel.ID)
		assert.Len(t, files, 2)

		// 删除标签
		err := mgr.DeleteLabel(tempLabel.ID, "user1")
		require.NoError(t, err)

		// 关联应该已被清理
		files, _ = mgr.GetFilesByLabel(tempLabel.ID)
		assert.Len(t, files, 0)
	})
}

func TestSearchLabels(t *testing.T) {
	mgr, cleanup := setupTestLabelManager(t)
	defer cleanup()

	// 创建测试数据
	mgr.CreateLabel(SharedLabelInput{Name: "重要文档", Description: "工作相关", Owner: "user1"})
	mgr.CreateLabel(SharedLabelInput{Name: "重要照片", Description: "个人照片", Owner: "user1"})
	mgr.CreateLabel(SharedLabelInput{Name: "临时文件", Description: "待清理", Owner: "user1"})
	mgr.CreateLabel(SharedLabelInput{Name: "他人标签", Description: "不可见", Owner: "user2"})

	// 给 user1 分享一个 user2 的标签
	shared, _ := mgr.CreateLabel(SharedLabelInput{Name: "共享的重要", Description: "已共享", Owner: "user2"})
	mgr.ShareLabel(shared.ID, []string{"user1"}, "user2")

	t.Run("SearchByName", func(t *testing.T) {
		labels, err := mgr.SearchLabels("重要", "user1")
		require.NoError(t, err)
		assert.Len(t, labels, 3) // 重要文档 + 重要照片 + 共享的重要
	})

	t.Run("SearchByDescription", func(t *testing.T) {
		labels, err := mgr.SearchLabels("工作", "user1")
		require.NoError(t, err)
		assert.Len(t, labels, 1)
		assert.Equal(t, "重要文档", labels[0].Name)
	})

	t.Run("SearchNoResult", func(t *testing.T) {
		labels, err := mgr.SearchLabels("不存在的关键词", "user1")
		require.NoError(t, err)
		assert.Len(t, labels, 0)
	})

	t.Run("SearchRespectVisibility", func(t *testing.T) {
		// user2 只能看到自己的和被分享的
		labels, err := mgr.SearchLabels("重要", "user2")
		require.NoError(t, err)
		// user2 能看到：重要文档(分享了？没有)、重要照片(没有)、共享的重要(自己的)
		// user2 不能看到 user1 的 "重要文档" 和 "重要照片"
		for _, l := range labels {
			assert.Equal(t, "user2", l.Owner, "user2 只应看到自己的标签")
		}
	})
}

func TestLabelStats(t *testing.T) {
	mgr, cleanup := setupTestLabelManager(t)
	defer cleanup()

	label, _ := mgr.CreateLabel(SharedLabelInput{Name: "统计测试", Owner: "user1"})
	mgr.ShareLabel(label.ID, []string{"user2", "user3"}, "user1")
	mgr.AssignLabel("file-a", label.ID)
	mgr.AssignLabel("file-b", label.ID)
	mgr.AssignLabel("file-c", label.ID)

	fileCount, shareCount, err := mgr.GetLabelStats(label.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, fileCount)
	assert.Equal(t, 2, shareCount) // user2, user3（不包含 owner）
}

func TestDefaultColor(t *testing.T) {
	mgr, cleanup := setupTestLabelManager(t)
	defer cleanup()

	label, err := mgr.CreateLabel(SharedLabelInput{Name: "默认颜色", Owner: "user1"})
	require.NoError(t, err)
	assert.Equal(t, "#3498db", label.Color, "未指定颜色时应使用默认值")
}
