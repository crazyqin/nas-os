package notes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 笔记 CRUD 测试 ==========

func TestStore_CreateNote(t *testing.T) {
	store := NewStore()

	t.Run("创建Markdown笔记", func(t *testing.T) {
		note, err := store.CreateNote(NoteInput{
			Title:   "测试笔记",
			Content: "# Hello World\n这是一个测试笔记",
		}, "user1", "User 1")
		require.NoError(t, err)
		assert.NotEmpty(t, note.ID)
		assert.Equal(t, "测试笔记", note.Title)
		assert.Equal(t, FormatMarkdown, note.Format)
		assert.Equal(t, "user1", note.OwnerID)
		assert.True(t, note.WordCount > 0)
		assert.Nil(t, note.DeletedAt)
	})

	t.Run("创建富文本笔记", func(t *testing.T) {
		note, err := store.CreateNote(NoteInput{
			Title:   "富文本笔记",
			Content: "<p>Hello</p>",
			Format:  FormatRichText,
		}, "user1", "User 1")
		require.NoError(t, err)
		assert.Equal(t, FormatRichText, note.Format)
	})

	t.Run("创建笔记-空标题", func(t *testing.T) {
		_, err := store.CreateNote(NoteInput{
			Content: "内容",
		}, "user1", "User 1")
		assert.Error(t, err)
	})

	t.Run("创建笔记-无效格式", func(t *testing.T) {
		_, err := store.CreateNote(NoteInput{
			Title:  "测试",
			Format: "invalid",
		}, "user1", "User 1")
		assert.Error(t, err)
	})

	t.Run("创建笔记-带标签", func(t *testing.T) {
		note, err := store.CreateNote(NoteInput{
			Title:   "带标签笔记",
			Content: "内容",
			Tags:    []string{"Go", "测试", "NAS"},
		}, "user1", "User 1")
		require.NoError(t, err)
		assert.Len(t, note.Tags, 3)
	})

	t.Run("创建笔记-带笔记本", func(t *testing.T) {
		nb, _ := store.CreateNotebook(NotebookInput{Name: "测试笔记本"}, "user1")
		note, err := store.CreateNote(NoteInput{
			Title:      "笔记本中的笔记",
			Content:    "内容",
			NotebookID: nb.ID,
		}, "user1", "User 1")
		require.NoError(t, err)
		assert.Equal(t, nb.ID, note.NotebookID)
	})

	t.Run("创建笔记-笔记本不存在", func(t *testing.T) {
		_, err := store.CreateNote(NoteInput{
			Title:      "测试",
			NotebookID: "nonexistent",
		}, "user1", "User 1")
		assert.ErrorIs(t, err, ErrNotebookNotFound)
	})
}

func TestStore_GetNote(t *testing.T) {
	store := NewStore()

	t.Run("获取存在的笔记", func(t *testing.T) {
		created, _ := store.CreateNote(NoteInput{Title: "获取测试", Content: "内容"}, "user1", "User 1")
		got, err := store.GetNote(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "获取测试", got.Title)
	})

	t.Run("获取不存在的笔记", func(t *testing.T) {
		_, err := store.GetNote("nonexistent")
		assert.ErrorIs(t, err, ErrNoteNotFound)
	})

	t.Run("获取回收站中的笔记", func(t *testing.T) {
		created, _ := store.CreateNote(NoteInput{Title: "待删除", Content: "内容"}, "user1", "User 1")
		store.DeleteNote(created.ID)
		_, err := store.GetNote(created.ID)
		assert.ErrorIs(t, err, ErrNoteInTrash)
	})
}

func TestStore_UpdateNote(t *testing.T) {
	store := NewStore()

	t.Run("更新笔记标题和内容", func(t *testing.T) {
		created, _ := store.CreateNote(NoteInput{Title: "原始标题", Content: "原始内容"}, "user1", "User 1")
		updated, err := store.UpdateNote(created.ID, NoteInput{
			Title:   "更新后的标题",
			Content: "更新后的内容",
		})
		require.NoError(t, err)
		assert.Equal(t, "更新后的标题", updated.Title)
		assert.Equal(t, "更新后的内容", updated.Content)
	})

	t.Run("更新笔记标签", func(t *testing.T) {
		created, _ := store.CreateNote(NoteInput{Title: "标签测试", Tags: []string{"old"}}, "user1", "User 1")
		updated, err := store.UpdateNote(created.ID, NoteInput{
			Tags: []string{"new1", "new2"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"new1", "new2"}, updated.Tags)
	})

	t.Run("更新不存在的笔记", func(t *testing.T) {
		_, err := store.UpdateNote("nonexistent", NoteInput{Title: "test"})
		assert.ErrorIs(t, err, ErrNoteNotFound)
	})
}

func TestStore_DeleteAndRestore(t *testing.T) {
	store := NewStore()

	t.Run("软删除笔记", func(t *testing.T) {
		created, _ := store.CreateNote(NoteInput{Title: "待删除", Content: "内容"}, "user1", "User 1")
		err := store.DeleteNote(created.ID)
		require.NoError(t, err)

		// 在回收站中应该获取不到
		_, err = store.GetNote(created.ID)
		assert.ErrorIs(t, err, ErrNoteInTrash)
	})

	t.Run("恢复笔记", func(t *testing.T) {
		created, _ := store.CreateNote(NoteInput{Title: "待恢复", Content: "内容"}, "user1", "User 1")
		store.DeleteNote(created.ID)
		err := store.RestoreNote(created.ID)
		require.NoError(t, err)

		got, err := store.GetNote(created.ID)
		require.NoError(t, err)
		assert.Equal(t, "待恢复", got.Title)
		assert.Nil(t, got.DeletedAt)
	})

	t.Run("永久删除", func(t *testing.T) {
		created, _ := store.CreateNote(NoteInput{Title: "永久删除", Content: "内容"}, "user1", "User 1")
		err := store.PermanentDeleteNote(created.ID)
		require.NoError(t, err)

		_, err = store.GetNote(created.ID)
		assert.ErrorIs(t, err, ErrNoteNotFound)
	})

	t.Run("删除不存在的笔记", func(t *testing.T) {
		err := store.DeleteNote("nonexistent")
		assert.ErrorIs(t, err, ErrNoteNotFound)
	})
}

func TestStore_ListNotes(t *testing.T) {
	store := NewStore()

	// 创建多个笔记
	store.CreateNote(NoteInput{Title: "笔记1", Content: "内容1"}, "user1", "User 1")
	store.CreateNote(NoteInput{Title: "笔记2", Content: "内容2"}, "user1", "User 1")
	store.CreateNote(NoteInput{Title: "笔记3", Content: "内容3"}, "user1", "User 1")

	t.Run("列出所有笔记", func(t *testing.T) {
		notes, total := store.ListNotes("", 10, 0)
		assert.Equal(t, 3, total)
		assert.Len(t, notes, 3)
	})

	t.Run("分页", func(t *testing.T) {
		notes, total := store.ListNotes("", 2, 0)
		assert.Equal(t, 3, total)
		assert.Len(t, notes, 2)

		notes, _ = store.ListNotes("", 2, 2)
		assert.Len(t, notes, 1)
	})

	t.Run("按笔记本过滤", func(t *testing.T) {
		nb, _ := store.CreateNotebook(NotebookInput{Name: "过滤测试"}, "user1")
		store.CreateNote(NoteInput{Title: "笔记本笔记", NotebookID: nb.ID}, "user1", "User 1")

		notes, total := store.ListNotes(nb.ID, 10, 0)
		assert.Equal(t, 1, total)
		assert.Len(t, notes, 1)
	})

	t.Run("不列出回收站笔记", func(t *testing.T) {
		store2 := NewStore()
		n1, _ := store2.CreateNote(NoteInput{Title: "A"}, "u1", "U1")
		store2.CreateNote(NoteInput{Title: "B"}, "u1", "U1")
		store2.DeleteNote(n1.ID)

		notes, total := store2.ListNotes("", 10, 0)
		assert.Equal(t, 1, total)
		assert.Equal(t, "B", notes[0].Title)
	})
}

func TestStore_ToggleFavorite(t *testing.T) {
	store := NewStore()

	note, _ := store.CreateNote(NoteInput{Title: "收藏测试"}, "user1", "User 1")
	assert.False(t, note.Favorite)

	toggled, err := store.ToggleFavorite(note.ID)
	require.NoError(t, err)
	assert.True(t, toggled.Favorite)

	toggled, _ = store.ToggleFavorite(note.ID)
	assert.False(t, toggled.Favorite)
}

func TestStore_TogglePin(t *testing.T) {
	store := NewStore()

	note, _ := store.CreateNote(NoteInput{Title: "置顶测试"}, "user1", "User 1")
	assert.False(t, note.Pinned)

	toggled, err := store.TogglePin(note.ID)
	require.NoError(t, err)
	assert.True(t, toggled.Pinned)
}

// ========== 笔记本测试 ==========

func TestStore_NotebookCRUD(t *testing.T) {
	store := NewStore()

	t.Run("创建笔记本", func(t *testing.T) {
		nb, err := store.CreateNotebook(NotebookInput{
			Name:        "工作笔记",
			Description: "工作相关",
			Color:       "#FF0000",
		}, "user1")
		require.NoError(t, err)
		assert.NotEmpty(t, nb.ID)
		assert.Equal(t, "工作笔记", nb.Name)
		assert.Equal(t, "#FF0000", nb.Color)
	})

	t.Run("创建笔记本-空名称", func(t *testing.T) {
		_, err := store.CreateNotebook(NotebookInput{}, "user1")
		assert.Error(t, err)
	})

	t.Run("获取笔记本", func(t *testing.T) {
		created, _ := store.CreateNotebook(NotebookInput{Name: "获取测试"}, "user1")
		got, err := store.GetNotebook(created.ID)
		require.NoError(t, err)
		assert.Equal(t, "获取测试", got.Name)
	})

	t.Run("获取不存在的笔记本", func(t *testing.T) {
		_, err := store.GetNotebook("nonexistent")
		assert.ErrorIs(t, err, ErrNotebookNotFound)
	})

	t.Run("更新笔记本", func(t *testing.T) {
		created, _ := store.CreateNotebook(NotebookInput{Name: "原始"}, "user1")
		updated, err := store.UpdateNotebook(created.ID, NotebookInput{
			Name:  "已更新",
			Color: "#00FF00",
		})
		require.NoError(t, err)
		assert.Equal(t, "已更新", updated.Name)
		assert.Equal(t, "#00FF00", updated.Color)
	})

	t.Run("删除笔记本", func(t *testing.T) {
		nb, _ := store.CreateNotebook(NotebookInput{Name: "待删除"}, "user1")
		store.CreateNote(NoteInput{Title: "笔记", NotebookID: nb.ID}, "user1", "User 1")

		err := store.DeleteNotebook(nb.ID, "")
		require.NoError(t, err)

		_, err = store.GetNotebook(nb.ID)
		assert.ErrorIs(t, err, ErrNotebookNotFound)
	})

	t.Run("列出笔记本", func(t *testing.T) {
		store2 := NewStore()
		store2.CreateNotebook(NotebookInput{Name: "A"}, "user1")
		store2.CreateNotebook(NotebookInput{Name: "B"}, "user1")
		store2.CreateNotebook(NotebookInput{Name: "C"}, "user2")

		nbs := store2.ListNotebooks("user1")
		assert.Len(t, nbs, 2)

		all := store2.ListNotebooks("")
		assert.Len(t, all, 3)
	})
}

// ========== 分享测试 ==========

func TestStore_ShareCRUD(t *testing.T) {
	store := NewStore()
	note, _ := store.CreateNote(NoteInput{Title: "分享测试", Content: "内容"}, "user1", "User 1")

	t.Run("创建分享-公开链接", func(t *testing.T) {
		share, err := store.CreateShare(note.ID, ShareInput{
			Permission: "view",
		}, "user1")
		require.NoError(t, err)
		assert.NotEmpty(t, share.Token)
		assert.Equal(t, "view", share.Permission)
		assert.False(t, share.HasPassword)
		assert.Nil(t, share.ExpiresAt)
	})

	t.Run("创建分享-密码保护", func(t *testing.T) {
		share, err := store.CreateShare(note.ID, ShareInput{
			Permission: "view",
			Password:   "secret123",
		}, "user1")
		require.NoError(t, err)
		assert.True(t, share.HasPassword)
		assert.Equal(t, "secret123", share.Password)
	})

	t.Run("创建分享-过期时间", func(t *testing.T) {
		share, err := store.CreateShare(note.ID, ShareInput{
			Permission: "view",
			ExpiresIn:  24,
		}, "user1")
		require.NoError(t, err)
		assert.NotNil(t, share.ExpiresAt)
	})

	t.Run("创建分享-最大访问次数", func(t *testing.T) {
		share, err := store.CreateShare(note.ID, ShareInput{
			Permission: "view",
			MaxViews:   10,
		}, "user1")
		require.NoError(t, err)
		assert.Equal(t, 10, share.MaxViews)
	})

	t.Run("访问分享-无密码", func(t *testing.T) {
		share, _ := store.CreateShare(note.ID, ShareInput{Permission: "view"}, "user1")
		accessed, err := store.AccessShare(share.Token, "")
		require.NoError(t, err)
		assert.Equal(t, note.ID, accessed.ID)
	})

	t.Run("访问分享-需要密码", func(t *testing.T) {
		share, _ := store.CreateShare(note.ID, ShareInput{
			Permission: "view",
			Password:   "pass",
		}, "user1")

		_, err := store.AccessShare(share.Token, "")
		assert.ErrorIs(t, err, ErrPasswordRequired)

		_, err = store.AccessShare(share.Token, "wrong")
		assert.ErrorIs(t, err, ErrPasswordWrong)

		accessed, err := store.AccessShare(share.Token, "pass")
		require.NoError(t, err)
		assert.Equal(t, note.ID, accessed.ID)
	})

	t.Run("列出笔记分享", func(t *testing.T) {
		store.CreateShare(note.ID, ShareInput{Permission: "view"}, "user1")
		store.CreateShare(note.ID, ShareInput{Permission: "edit"}, "user1")

		shares := store.ListNoteShares(note.ID)
		assert.GreaterOrEqual(t, len(shares), 2)
	})

	t.Run("删除分享", func(t *testing.T) {
		share, _ := store.CreateShare(note.ID, ShareInput{Permission: "view"}, "user1")
		err := store.DeleteShare(share.ID)
		require.NoError(t, err)

		_, err = store.GetShare(share.ID)
		assert.ErrorIs(t, err, ErrShareNotFound)
	})

	t.Run("删除不存在的分享", func(t *testing.T) {
		err := store.DeleteShare("nonexistent")
		assert.ErrorIs(t, err, ErrShareNotFound)
	})

	t.Run("创建分享-笔记不存在", func(t *testing.T) {
		_, err := store.CreateShare("nonexistent", ShareInput{Permission: "view"}, "user1")
		assert.ErrorIs(t, err, ErrNoteNotFound)
	})
}

// ========== 搜索测试 ==========

func TestStore_Search(t *testing.T) {
	store := NewStore()

	// 创建测试数据
	store.CreateNote(NoteInput{Title: "Go语言教程", Content: "Go是一种静态类型语言", Tags: []string{"Go", "编程"}}, "user1", "User 1")
	store.CreateNote(NoteInput{Title: "NAS搭建指南", Content: "如何搭建家庭NAS", Tags: []string{"NAS", "硬件"}}, "user1", "User 1")
	store.CreateNote(NoteInput{Title: "Docker入门", Content: "Docker容器技术", Tags: []string{"Docker", "容器"}}, "user1", "User 1")
	store.CreateNote(NoteInput{Title: "Linux命令", Content: "常用Linux命令", Tags: []string{"Linux", "运维"}}, "user1", "User 1")

	t.Run("关键词搜索-标题", func(t *testing.T) {
		result := store.Search(SearchQuery{Keyword: "Go", Limit: 10})
		assert.GreaterOrEqual(t, result.Total, 1)
		found := false
		for _, n := range result.Notes {
			if n.Title == "Go语言教程" {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("关键词搜索-内容", func(t *testing.T) {
		result := store.Search(SearchQuery{Keyword: "容器", Limit: 10})
		assert.GreaterOrEqual(t, result.Total, 1)
	})

	t.Run("关键词搜索-标签", func(t *testing.T) {
		result := store.Search(SearchQuery{Keyword: "NAS", Limit: 10})
		assert.GreaterOrEqual(t, result.Total, 1)
	})

	t.Run("按标签过滤", func(t *testing.T) {
		result := store.Search(SearchQuery{Tags: []string{"Go"}, Limit: 10})
		assert.Equal(t, 1, result.Total)
	})

	t.Run("无结果搜索", func(t *testing.T) {
		result := store.Search(SearchQuery{Keyword: "不存在的关键词xyz", Limit: 10})
		assert.Equal(t, 0, result.Total)
		assert.Empty(t, result.Notes)
	})

	t.Run("空关键词", func(t *testing.T) {
		result := store.Search(SearchQuery{Limit: 10})
		assert.Equal(t, 4, result.Total)
	})

	t.Run("排序", func(t *testing.T) {
		result := store.Search(SearchQuery{
			SortBy:    "title",
			SortOrder: "asc",
			Limit:     10,
		})
		require.Len(t, result.Notes, 4)
		assert.Equal(t, "Docker入门", result.Notes[0].Title)
	})

	t.Run("分页", func(t *testing.T) {
		result := store.Search(SearchQuery{Limit: 2, Offset: 0})
		assert.Equal(t, 4, result.Total)
		assert.Len(t, result.Notes, 2)

		result = store.Search(SearchQuery{Limit: 2, Offset: 2})
		assert.Len(t, result.Notes, 2)
	})

	t.Run("收藏过滤", func(t *testing.T) {
		// 先收藏一个笔记
		store.CreateNote(NoteInput{Title: "收藏的笔记", Content: "内容", Favorite: boolPtr(true)}, "user1", "User 1")
		fav := true
		result := store.Search(SearchQuery{Favorite: &fav, Limit: 10})
		assert.GreaterOrEqual(t, result.Total, 1)
		for _, n := range result.Notes {
			assert.True(t, n.Favorite)
		}
	})
}

// ========== 标签测试 ==========

func TestStore_Tags(t *testing.T) {
	store := NewStore()

	store.CreateNote(NoteInput{Title: "A", Tags: []string{"Go", "编程"}}, "u1", "U1")
	store.CreateNote(NoteInput{Title: "B", Tags: []string{"Go", "NAS"}}, "u1", "U1")
	store.CreateNote(NoteInput{Title: "C", Tags: []string{"Docker"}}, "u1", "U1")

	t.Run("获取所有标签", func(t *testing.T) {
		tags := store.GetAllTags()
		assert.Equal(t, 2, tags["go"]) // Go出现在2个笔记中（大小写不敏感，存储为小写）
		assert.Equal(t, 1, tags["nas"])
		assert.Equal(t, 1, tags["docker"])
	})

	t.Run("标签索引更新-删除笔记", func(t *testing.T) {
		// 删除一个笔记后标签计数应更新
		notes, _ := store.ListNotes("", 10, 0)
		for _, n := range notes {
			if n.Title == "B" {
				store.DeleteNote(n.ID)
				break
			}
		}

		tags := store.GetAllTags()
		assert.Equal(t, 1, tags["go"])
		assert.Equal(t, 0, tags["nas"]) // nas标签不再有任何有效笔记
	})
}

// ========== 统计测试 ==========

func TestStore_Stats(t *testing.T) {
	store := NewStore()

	store.CreateNote(NoteInput{Title: "A", Content: "12345"}, "user1", "User 1")
	store.CreateNote(NoteInput{Title: "B", Content: "1234567890", Favorite: boolPtr(true)}, "user1", "User 1")
	store.CreateNotebook(NotebookInput{Name: "NB1"}, "user1")
	deleted, _ := store.CreateNote(NoteInput{Title: "C"}, "user1", "User 1")
	store.DeleteNote(deleted.ID)

	stats := store.GetNoteStats("user1")
	assert.Equal(t, 2, stats["total_notes"])
	assert.Equal(t, 1, stats["deleted_notes"])
	assert.Equal(t, 1, stats["total_notebooks"])
	assert.Equal(t, 1, stats["favorites"])
}

// ========== 回收站测试 ==========

func TestStore_Trash(t *testing.T) {
	store := NewStore()

	store.CreateNote(NoteInput{Title: "A"}, "u1", "U1")
	b, _ := store.CreateNote(NoteInput{Title: "B"}, "u1", "U1")
	c, _ := store.CreateNote(NoteInput{Title: "C"}, "u1", "U1")
	store.DeleteNote(b.ID)
	store.DeleteNote(c.ID)

	t.Run("列出回收站", func(t *testing.T) {
		notes, total := store.ListTrashNotes(10, 0)
		assert.Equal(t, 2, total)
		assert.Len(t, notes, 2)
	})

	t.Run("清空回收站", func(t *testing.T) {
		count := store.EmptyTrash()
		assert.Equal(t, 2, count)

		notes, total := store.ListTrashNotes(10, 0)
		assert.Equal(t, 0, total)
		assert.Empty(t, notes)
	})
}

// ========== 辅助函数 ==========

func boolPtr(b bool) *bool { return &b }

func TestCountWords(t *testing.T) {
	assert.Equal(t, 0, countWords(""))
	assert.Equal(t, 5, countWords("hello"))
	assert.Equal(t, 6, countWords("你好世界Go"))
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	assert.NotEmpty(t, id1)
	assert.NotEqual(t, id1, id2)
}

func TestGenerateToken(t *testing.T) {
	token1 := generateToken()
	token2 := generateToken()
	assert.NotEmpty(t, token1)
	assert.NotEqual(t, token1, token2)
	assert.True(t, len(token1) > 20) // Token should be reasonably long
}
