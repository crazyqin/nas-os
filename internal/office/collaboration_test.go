package office

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== OT 操作测试 ==========

func TestCollabEngine_OpenDocument(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())

	t.Run("打开新文档", func(t *testing.T) {
		doc := engine.OpenDocument("doc1", "Hello World")
		assert.Equal(t, "doc1", doc.DocID)
		assert.Equal(t, "Hello World", doc.Content)
		assert.Equal(t, int64(1), doc.Version)
		assert.Len(t, doc.Versions, 1) // 初始版本
	})

	t.Run("打开已存在的文档", func(t *testing.T) {
		doc := engine.OpenDocument("doc1", "New Content")
		assert.Equal(t, "Hello World", doc.Content) // 应返回已有文档
	})

	t.Run("打开空文档", func(t *testing.T) {
		doc := engine.OpenDocument("doc2", "")
		assert.Equal(t, "", doc.Content)
	})
}

func TestCollabEngine_GetDocument(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())

	t.Run("获取存在的文档", func(t *testing.T) {
		engine.OpenDocument("doc1", "Content")
		doc, err := engine.GetDocument("doc1")
		require.NoError(t, err)
		assert.Equal(t, "doc1", doc.DocID)
	})

	t.Run("获取不存在的文档", func(t *testing.T) {
		_, err := engine.GetDocument("nonexistent")
		assert.Error(t, err)
	})
}

func TestCollabEngine_CloseDocument(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())

	engine.OpenDocument("doc1", "Content")
	err := engine.CloseDocument("doc1")
	require.NoError(t, err)

	_, err = engine.GetDocument("doc1")
	assert.Error(t, err)
}

// ========== OT 操作应用测试 ==========

func TestCollabEngine_ApplyOperation_Insert(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Hello")

	t.Run("在末尾插入", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     OpInsert,
			Position: 5,
			Text:     " World",
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.True(t, result.Applied)

		doc, _ := engine.GetDocument("doc1")
		doc.mu.RLock()
		assert.Equal(t, "Hello World", doc.Content)
		doc.mu.RUnlock()
	})

	t.Run("在开头插入", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     OpInsert,
			Position: 0,
			Text:     "Say ",
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.True(t, result.Applied)

		doc, _ := engine.GetDocument("doc1")
		doc.mu.RLock()
		assert.Equal(t, "Say Hello World", doc.Content)
		doc.mu.RUnlock()
	})

	t.Run("在中间插入", func(t *testing.T) {
		engine2 := NewCollabEngine(DefaultCollabEngineConfig())
		engine2.OpenDocument("doc-mid", "AB")
		result, err := engine2.ApplyOperation("doc-mid", &Operation{
			Type:     OpInsert,
			Position: 1,
			Text:     "X",
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.True(t, result.Applied)

		doc, _ := engine2.GetDocument("doc-mid")
		doc.mu.RLock()
		assert.Equal(t, "AXB", doc.Content)
		doc.mu.RUnlock()
	})
}

func TestCollabEngine_ApplyOperation_Delete(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Hello World")

	t.Run("删除部分内容", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     OpDelete,
			Position: 5,
			Length:   6, // 删除 " World"
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.True(t, result.Applied)

		doc, _ := engine.GetDocument("doc1")
		doc.mu.RLock()
		assert.Equal(t, "Hello", doc.Content)
		doc.mu.RUnlock()
	})
}

func TestCollabEngine_ApplyOperation_Validation(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Hello")

	t.Run("插入位置超出范围", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     OpInsert,
			Position: 100,
			Text:     "test",
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.False(t, result.Applied)
		assert.NotEmpty(t, result.Error)
	})

	t.Run("删除位置超出范围", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     OpDelete,
			Position: 100,
			Length:   1,
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.False(t, result.Applied)
	})

	t.Run("删除长度超出范围", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     OpDelete,
			Position: 3,
			Length:   10,
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.False(t, result.Applied)
	})

	t.Run("空文本插入", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     OpInsert,
			Position: 0,
			Text:     "",
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.False(t, result.Applied)
	})

	t.Run("未知操作类型", func(t *testing.T) {
		result, err := engine.ApplyOperation("doc1", &Operation{
			Type:     "unknown",
			Position: 0,
			UserID:   "user1",
		})
		require.NoError(t, err)
		assert.False(t, result.Applied)
	})

	t.Run("不存在的文档", func(t *testing.T) {
		_, err := engine.ApplyOperation("nonexistent", &Operation{
			Type:     OpInsert,
			Position: 0,
			Text:     "test",
			UserID:   "user1",
		})
		assert.Error(t, err)
	})
}

func TestCollabEngine_ApplyOperation_Retain(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Hello")

	result, err := engine.ApplyOperation("doc1", &Operation{
		Type:     OpRetain,
		Position: 3,
		UserID:   "user1",
	})
	require.NoError(t, err)
	assert.True(t, result.Applied)

	doc, _ := engine.GetDocument("doc1")
	doc.mu.RLock()
	assert.Equal(t, "Hello", doc.Content) // 内容不变
	doc.mu.RUnlock()
}

// ========== 版本历史测试 ==========

func TestCollabEngine_SaveVersion(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Initial")

	// 添加一些操作
	engine.ApplyOperation("doc1", &Operation{
		Type: OpInsert, Position: 7, Text: " Content", UserID: "user1",
	})

	t.Run("保存版本", func(t *testing.T) {
		snapshot, err := engine.SaveVersion("doc1", "user1", "User 1", "First save")
		require.NoError(t, err)
		assert.Equal(t, int64(2), snapshot.Version)
		assert.Equal(t, "Initial Content", snapshot.Content)
		assert.Equal(t, "user1", snapshot.UserID)
		assert.Equal(t, "First save", snapshot.Message)
	})

	t.Run("保存多个版本", func(t *testing.T) {
		engine.SaveVersion("doc1", "user1", "User 1", "Second save")
		engine.SaveVersion("doc1", "user2", "User 2", "Third save")

		versions, total, err := engine.GetVersionHistory("doc1", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 4, total) // 1 initial + 3 saved
		assert.Len(t, versions, 4)
	})

	t.Run("保存不存在的文档", func(t *testing.T) {
		_, err := engine.SaveVersion("nonexistent", "user1", "User 1", "test")
		assert.Error(t, err)
	})
}

func TestCollabEngine_GetVersionHistory(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Content")

	engine.SaveVersion("doc1", "user1", "User 1", "v1")
	engine.SaveVersion("doc1", "user1", "User 1", "v2")
	engine.SaveVersion("doc1", "user1", "User 1", "v3")

	t.Run("获取全部版本", func(t *testing.T) {
		versions, total, err := engine.GetVersionHistory("doc1", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 4, total) // 1 initial + 3 saved
		assert.Len(t, versions, 4)
	})

	t.Run("分页获取", func(t *testing.T) {
		versions, total, _ := engine.GetVersionHistory("doc1", 2, 0)
		assert.Equal(t, 4, total)
		assert.Len(t, versions, 2)
	})

	t.Run("偏移超出范围", func(t *testing.T) {
		versions, total, _ := engine.GetVersionHistory("doc1", 10, 100)
		assert.Equal(t, 4, total)
		assert.Empty(t, versions)
	})
}

func TestCollabEngine_GetVersion(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Content")

	snapshot, _ := engine.SaveVersion("doc1", "user1", "User 1", "test")

	t.Run("获取存在的版本", func(t *testing.T) {
		version, err := engine.GetVersion("doc1", snapshot.Version)
		require.NoError(t, err)
		assert.Equal(t, snapshot.Version, version.Version)
	})

	t.Run("获取不存在的版本", func(t *testing.T) {
		_, err := engine.GetVersion("doc1", 999)
		assert.Error(t, err)
	})
}

func TestCollabEngine_RestoreVersion(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Original")

	// 修改内容
	engine.ApplyOperation("doc1", &Operation{
		Type: OpInsert, Position: 8, Text: " Modified", UserID: "user1",
	})
	v1Snapshot, _ := engine.SaveVersion("doc1", "user1", "User 1", "v1")

	// 再次修改
	engine.ApplyOperation("doc1", &Operation{
		Type: OpDelete, Position: 0, Length: 8, UserID: "user1",
	})
	engine.SaveVersion("doc1", "user1", "User 1", "v2")

	t.Run("恢复到v1版本", func(t *testing.T) {
		err := engine.RestoreVersion("doc1", v1Snapshot.Version, "user1", "User 1")
		require.NoError(t, err)

		doc, _ := engine.GetDocument("doc1")
		doc.mu.RLock()
		assert.Equal(t, "Original Modified", doc.Content)
		doc.mu.RUnlock()
	})

	t.Run("恢复不存在的版本", func(t *testing.T) {
		err := engine.RestoreVersion("doc1", 999, "user1", "User 1")
		assert.Error(t, err)
	})
}

// ========== 评论测试 ==========

func TestCollabEngine_Comments(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Hello World")

	t.Run("添加评论", func(t *testing.T) {
		comment, err := engine.AddComment("doc1", "user1", "User 1", "This looks good", nil)
		require.NoError(t, err)
		assert.NotEmpty(t, comment.CommentID)
		assert.Equal(t, "user1", comment.UserID)
		assert.Equal(t, "This looks good", comment.Content)
		assert.False(t, comment.Resolved)
	})

	t.Run("添加带选区的评论", func(t *testing.T) {
		rng := &CommentRange{StartOffset: 0, EndOffset: 5}
		comment, err := engine.AddComment("doc1", "user1", "User 1", "Range comment", rng)
		require.NoError(t, err)
		assert.NotNil(t, comment.Range)
		assert.Equal(t, 0, comment.Range.StartOffset)
		assert.Equal(t, 5, comment.Range.EndOffset)
	})

	t.Run("获取评论列表", func(t *testing.T) {
		comments, err := engine.GetComments("doc1")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(comments), 2)
	})

	t.Run("解决评论", func(t *testing.T) {
		comment, _ := engine.AddComment("doc1", "user1", "User 1", "To resolve", nil)
		err := engine.ResolveComment("doc1", comment.CommentID)
		require.NoError(t, err)

		comments, _ := engine.GetComments("doc1")
		for _, c := range comments {
			if c.CommentID == comment.CommentID {
				assert.True(t, c.Resolved)
			}
		}
	})

	t.Run("回复评论", func(t *testing.T) {
		comment, _ := engine.AddComment("doc1", "user1", "User 1", "Original", nil)
		err := engine.ReplyComment("doc1", comment.CommentID, "user2", "User 2", "Reply content")
		require.NoError(t, err)

		comments, _ := engine.GetComments("doc1")
		for _, c := range comments {
			if c.CommentID == comment.CommentID {
				assert.Len(t, c.Replies, 1)
				assert.Equal(t, "Reply content", c.Replies[0].Content)
			}
		}
	})

	t.Run("删除评论", func(t *testing.T) {
		comment, _ := engine.AddComment("doc1", "user1", "User 1", "To delete", nil)
		err := engine.DeleteComment("doc1", comment.CommentID)
		require.NoError(t, err)
	})

	t.Run("删除不存在的评论", func(t *testing.T) {
		err := engine.DeleteComment("doc1", "nonexistent")
		assert.Error(t, err)
	})

	t.Run("评论不存在的文档", func(t *testing.T) {
		_, err := engine.AddComment("nonexistent", "user1", "User 1", "test", nil)
		assert.Error(t, err)
	})
}

// ========== 统计测试 ==========

func TestCollabEngine_GetStats(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	engine.OpenDocument("doc1", "Hello World")

	engine.ApplyOperation("doc1", &Operation{
		Type: OpInsert, Position: 11, Text: "!", UserID: "user1",
	})
	engine.SaveVersion("doc1", "user1", "User 1", "save")
	engine.AddComment("doc1", "user1", "User 1", "comment", nil)
	engine.AddComment("doc1", "user2", "User 2", "another", nil)

	stats, err := engine.GetStats("doc1")
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.Version)
	assert.Equal(t, 1, stats.OpCount)
	assert.Equal(t, 2, stats.CommentCount)
	assert.Equal(t, 2, stats.UnresolvedCount) // 两个评论都没有标记为已解决
	assert.Equal(t, 12, stats.ContentLength)  // "Hello World!"
}

func TestCollabEngine_GetStats_NotFound(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())
	_, err := engine.GetStats("nonexistent")
	assert.Error(t, err)
}

// ========== OT 变换算法测试 ==========

func TestTransformPair_InsertInsert(t *testing.T) {
	// 两个并发插入
	op1 := &Operation{Type: OpInsert, Position: 5, Text: "ABC", UserID: "user1"}
	op2 := &Operation{Type: OpInsert, Position: 3, Text: "XY", UserID: "user2"}

	// op2 在 op1 之前插入，op1 的位置应该后移
	transformed := transformPair(op1, op2)
	assert.Equal(t, 7, transformed.Position) // 5 + len("XY") = 7
}

func TestTransformPair_InsertDelete(t *testing.T) {
	// 插入 vs 删除
	op := &Operation{Type: OpInsert, Position: 5, Text: "X", UserID: "user1"}
	against := &Operation{Type: OpDelete, Position: 2, Length: 2, UserID: "user2"}

	// against 删除了 position 2-3 的内容，op 的位置应该前移
	transformed := transformPair(op, against)
	assert.Equal(t, 3, transformed.Position) // 5 - 2 = 3
}

func TestTransformPair_DeleteInsert(t *testing.T) {
	op := &Operation{Type: OpDelete, Position: 5, Length: 2, UserID: "user1"}
	against := &Operation{Type: OpInsert, Position: 3, Text: "XY", UserID: "user2"}

	// against 在 position 3 插入，op 的位置应该后移
	transformed := transformPair(op, against)
	assert.Equal(t, 7, transformed.Position) // 5 + 2 = 7
}

func TestTransformPair_DeleteDelete(t *testing.T) {
	// 两个删除不重叠
	op := &Operation{Type: OpDelete, Position: 10, Length: 2, UserID: "user1"}
	against := &Operation{Type: OpDelete, Position: 3, Length: 2, UserID: "user2"}

	transformed := transformPair(op, against)
	assert.Equal(t, 8, transformed.Position) // 10 - 2 = 8
	assert.Equal(t, 2, transformed.Length)
}

// ========== 空操作测试 ==========

func TestCollabEngine_ApplyOp(t *testing.T) {
	engine := NewCollabEngine(DefaultCollabEngineConfig())

	t.Run("插入操作", func(t *testing.T) {
		result, _ := engine.applyOp("Hello", &Operation{Type: OpInsert, Position: 5, Text: " World"})
		assert.Equal(t, "Hello World", result)
	})

	t.Run("删除操作", func(t *testing.T) {
		result, _ := engine.applyOp("Hello World", &Operation{Type: OpDelete, Position: 5, Length: 6})
		assert.Equal(t, "Hello", result)
	})

	t.Run("保留操作", func(t *testing.T) {
		result, _ := engine.applyOp("Hello", &Operation{Type: OpRetain, Position: 3})
		assert.Equal(t, "Hello", result)
	})

	t.Run("中文字符操作", func(t *testing.T) {
		result, _ := engine.applyOp("你好", &Operation{Type: OpInsert, Position: 2, Text: "世界"})
		assert.Equal(t, "你好世界", result)
	})
}
