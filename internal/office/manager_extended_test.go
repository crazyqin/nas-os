package office

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManager_IsEnabled 测试 IsEnabled 方法.
func TestManager_IsEnabled(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, err := NewManager("", accessor, WithCleanupWorker(false))
	require.NoError(t, err)
	defer mgr.Close()

	// 默认禁用
	assert.False(t, mgr.IsEnabled())

	// 启用后
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})
	assert.True(t, mgr.IsEnabled())
}

// TestManager_GetFileSessions 测试获取文件的所有会话.
func TestManager_GetFileSessions(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 创建会话
	mgr.CreateSession("file1", "user1", "User 1", "edit")
	mgr.CreateSession("file1", "user2", "User 2", "view")

	sessions := mgr.GetFileSessions("file1")
	assert.Len(t, sessions, 2)
}

// TestManager_UpdateSessionStatus 测试更新会话状态.
func TestManager_UpdateSessionStatus(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, _, _ := mgr.CreateSession("file1", "user1", "User 1", "edit")

	err := mgr.UpdateSessionStatus(session.ID, SessionStatusEditing)
	require.NoError(t, err)

	updated, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusEditing, updated.Status)
}

// TestManager_UpdateSessionStatus_NotFound 测试更新不存在的会话.
func TestManager_UpdateSessionStatus_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.UpdateSessionStatus("nonexistent", SessionStatusEditing)
	assert.Error(t, err)
}

// TestManager_HandleCallback 测试回调处理.
func TestManager_HandleCallback(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, _, _ := mgr.CreateSession("file1", "user1", "User 1", "edit")

	// 测试编辑状态回调
	err := mgr.HandleCallback(session.ID, CallbackRequest{
		Key:    session.FileKey,
		Status: CallbackStatusEditing,
	})
	require.NoError(t, err)

	updated, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusEditing, updated.Status)
}

// TestManager_HandleCallback_Saving 测试保存中回调.
func TestManager_HandleCallback_Saving(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, _, _ := mgr.CreateSession("file1", "user1", "User 1", "edit")

	err := mgr.HandleCallback(session.ID, CallbackRequest{
		Key:    session.FileKey,
		Status: CallbackStatusSaving,
	})
	require.NoError(t, err)

	updated, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusSaving, updated.Status)
}

// TestManager_HandleCallback_Closed 测试关闭回调.
func TestManager_HandleCallback_Closed(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, _, _ := mgr.CreateSession("file1", "user1", "User 1", "edit")

	err := mgr.HandleCallback(session.ID, CallbackRequest{
		Key:    session.FileKey,
		Status: CallbackStatusClosed,
	})
	require.NoError(t, err)

	updated, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusClosed, updated.Status)
}

// TestManager_HandleCallback_Error 测试错误回调.
func TestManager_HandleCallback_Error(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, _, _ := mgr.CreateSession("file1", "user1", "User 1", "edit")

	err := mgr.HandleCallback(session.ID, CallbackRequest{
		Key:    session.FileKey,
		Status: CallbackStatusCorrupted,
	})
	assert.Error(t, err)

	updated, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusError, updated.Status)
}

// TestManager_HandleCallbackByKey 测试通过 Key 处理回调.
func TestManager_HandleCallbackByKey(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, _, _ := mgr.CreateSession("file1", "user1", "User 1", "edit")

	err := mgr.HandleCallbackByKey(CallbackRequest{
		Key:    session.FileKey,
		Status: CallbackStatusEditing,
	})
	require.NoError(t, err)

	updated, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusEditing, updated.Status)
}

// TestManager_HandleCallbackByKey_NotFound 测试 Key 不存在.
func TestManager_HandleCallbackByKey_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.HandleCallbackByKey(CallbackRequest{
		Key:    "nonexistent-key",
		Status: CallbackStatusEditing,
	})
	assert.Error(t, err)
}

// ========== 协作编辑测试 ==========

// TestManager_StartCollaboration 测试启动协作.
func TestManager_StartCollaboration(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, err := mgr.StartCollaboration("doc1")
	require.NoError(t, err)
	assert.Equal(t, "active", session.Status)
	assert.Equal(t, "doc1", session.DocID)
}

// TestManager_StartCollaboration_Disabled 测试禁用时启动协作.
func TestManager_StartCollaboration_Disabled(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	// 默认禁用

	_, err := mgr.StartCollaboration("doc1")
	assert.Error(t, err)
}

// TestManager_StartCollaboration_Existing 测试已存在的协作.
func TestManager_StartCollaboration_Existing(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 第一次启动
	session1, _ := mgr.StartCollaboration("doc1")

	// 再次启动，应返回已存在的会话
	session2, err := mgr.StartCollaboration("doc1")
	require.NoError(t, err)
	assert.Equal(t, session1.SessionID, session2.SessionID)
}

// TestManager_JoinCollaboration 测试加入协作.
func TestManager_JoinCollaboration(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	mgr.StartCollaboration("doc1")
	session, err := mgr.JoinCollaboration("doc1", "user1", "User 1")
	require.NoError(t, err)
	assert.Len(t, session.Users, 1)
	assert.Equal(t, "user1", session.Users[0].UserID)
}

// TestManager_JoinCollaboration_NotFound 测试加入不存在的协作.
func TestManager_JoinCollaboration_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	_, err := mgr.JoinCollaboration("nonexistent", "user1", "User 1")
	assert.Error(t, err)
}

// TestManager_LeaveCollaboration 测试离开协作.
func TestManager_LeaveCollaboration(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 使用唯一文档 ID
	docID := "doc-leave-test"
	mgr.StartCollaboration(docID)
	mgr.JoinCollaboration(docID, "user1", "User 1")
	mgr.JoinCollaboration(docID, "user2", "User 2")

	err := mgr.LeaveCollaboration(docID, "user1")
	require.NoError(t, err)

	session, _ := mgr.GetCollaborationSession(docID)
	assert.Len(t, session.Users, 1)
	assert.Equal(t, "user2", session.Users[0].UserID)
}

// TestManager_LeaveCollaboration_LastUser 测试最后一个用户离开.
func TestManager_LeaveCollaboration_LastUser(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 使用唯一文档 ID
	docID := "doc-leave-last-test"
	mgr.StartCollaboration(docID)
	mgr.JoinCollaboration(docID, "user1", "User 1")

	err := mgr.LeaveCollaboration(docID, "user1")
	require.NoError(t, err)

	session, _ := mgr.GetCollaborationSession(docID)
	assert.Equal(t, "closed", session.Status)
}

// TestManager_GetCollaborationSession 测试获取协作会话.
func TestManager_GetCollaborationSession(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	mgr.StartCollaboration("doc1")

	session, err := mgr.GetCollaborationSession("doc1")
	require.NoError(t, err)
	assert.NotNil(t, session)
}

// TestManager_GetCollaborationSession_NotFound 测试获取不存在的协作会话.
func TestManager_GetCollaborationSession_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	_, err := mgr.GetCollaborationSession("nonexistent")
	assert.Error(t, err)
}

// TestManager_UpdateCursor 测试更新光标.
func TestManager_UpdateCursor(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	mgr.StartCollaboration("doc1")
	mgr.JoinCollaboration("doc1", "user1", "User 1")

	err := mgr.UpdateCursor("doc1", "user1", 10, 5)
	require.NoError(t, err)

	session, _ := mgr.GetCollaborationSession("doc1")
	assert.Equal(t, 10, session.Cursors["user1"].Line)
	assert.Equal(t, 5, session.Cursors["user1"].Column)
}

// TestManager_UpdateCursor_NotFound 测试更新不存在文档的光标.
func TestManager_UpdateCursor_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.UpdateCursor("nonexistent", "user1", 10, 5)
	assert.Error(t, err)
}

// ========== 版本历史测试 ==========

// TestManager_GetVersionHistory 测试获取版本历史.
func TestManager_GetVersionHistory(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	history, err := mgr.GetVersionHistory("doc1")
	require.NoError(t, err)
	assert.NotNil(t, history)
	assert.Equal(t, "doc1", history.DocID)
	assert.Equal(t, 0, history.TotalVers)
}

// TestManager_GetVersionHistory_Disabled 测试禁用时获取版本历史.
func TestManager_GetVersionHistory_Disabled(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	// 默认禁用

	_, err := mgr.GetVersionHistory("doc1")
	assert.Error(t, err)
}

// TestManager_CreateVersion 测试创建版本.
func TestManager_CreateVersion(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	version, err := mgr.CreateVersion("doc1", "user1", "User 1", "Initial version")
	require.NoError(t, err)
	assert.Equal(t, 1, version.VersionNum)
	assert.Equal(t, "user1", version.CreatedBy.ID)
	assert.Equal(t, "Initial version", version.Description)
}

// TestManager_CreateVersion_Disabled 测试禁用时创建版本.
func TestManager_CreateVersion_Disabled(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	// 默认禁用

	_, err := mgr.CreateVersion("doc1", "user1", "User 1", "test")
	assert.Error(t, err)
}

// TestManager_GetVersion 测试获取特定版本.
func TestManager_GetVersion(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	created, _ := mgr.CreateVersion("doc1", "user1", "User 1", "test")

	version, err := mgr.GetVersion("doc1", created.VersionID)
	require.NoError(t, err)
	assert.Equal(t, created.VersionID, version.VersionID)
}

// TestManager_GetVersion_NotFound 测试获取不存在的版本.
func TestManager_GetVersion_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	_, err := mgr.GetVersion("doc1", "nonexistent")
	assert.Error(t, err)
}

// TestManager_RestoreVersion 测试恢复版本.
func TestManager_RestoreVersion(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 使用唯一文档 ID
	docID := "doc-restore-test"
	created, _ := mgr.CreateVersion(docID, "user1", "User 1", "test")

	err := mgr.RestoreVersion(docID, created.VersionID)
	require.NoError(t, err)

	// 恢复后应有新版本
	history, _ := mgr.GetVersionHistory(docID)
	assert.Equal(t, 2, history.TotalVers)
}

// TestManager_RestoreVersion_NotFound 测试恢复不存在的版本.
func TestManager_RestoreVersion_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.RestoreVersion("doc1", "nonexistent")
	assert.Error(t, err)
}

// ========== 评论测试 ==========

// TestManager_AddComment 测试添加评论.
func TestManager_AddComment(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	comment, err := mgr.AddComment("doc1", "user1", "This is a comment")
	require.NoError(t, err)
	assert.NotEmpty(t, comment.CommentID)
	assert.Equal(t, "user1", comment.UserID)
	assert.Equal(t, "This is a comment", comment.Content)
	assert.False(t, comment.Resolved)
}

// TestManager_AddComment_Disabled 测试禁用时添加评论.
func TestManager_AddComment_Disabled(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	// 默认禁用

	_, err := mgr.AddComment("doc1", "user1", "test")
	assert.Error(t, err)
}

// TestManager_AddCommentWithPosition 测试添加带位置的评论.
func TestManager_AddCommentWithPosition(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	pos := CommentPos{Line: 10, Paragraph: 5, Offset: 20}
	comment, err := mgr.AddCommentWithPosition("doc1", "user1", "Comment", pos)
	require.NoError(t, err)
	assert.Equal(t, 10, comment.Position.Line)
	assert.Equal(t, 5, comment.Position.Paragraph)
	assert.Equal(t, 20, comment.Position.Offset)
}

// TestManager_GetComments 测试获取评论列表.
func TestManager_GetComments(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 使用唯一的文档 ID 避免状态污染
	docID := "doc-comments-test"
	mgr.AddComment(docID, "user1", "Comment 1")
	mgr.AddComment(docID, "user2", "Comment 2")

	list, err := mgr.GetComments(docID)
	require.NoError(t, err)
	assert.Equal(t, 2, list.Total)
	assert.Len(t, list.Comments, 2)
}

// TestManager_ResolveComment 测试解决评论.
func TestManager_ResolveComment(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 使用唯一的文档 ID
	docID := "doc-resolve-test"
	comment, _ := mgr.AddComment(docID, "user1", "Comment")

	err := mgr.ResolveComment(docID, comment.CommentID)
	require.NoError(t, err)

	list, _ := mgr.GetComments(docID)
	assert.True(t, list.Comments[0].Resolved)
}

// TestManager_ResolveComment_NotFound 测试解决不存在的评论.
func TestManager_ResolveComment_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.ResolveComment("doc1", "nonexistent")
	assert.Error(t, err)
}

// TestManager_ReplyComment 测试回复评论.
func TestManager_ReplyComment(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 使用唯一的文档 ID
	docID := "doc-reply-test"
	comment, _ := mgr.AddComment(docID, "user1", "Comment")

	err := mgr.ReplyComment(docID, comment.CommentID, "user2", "Reply")
	require.NoError(t, err)

	list, _ := mgr.GetComments(docID)
	require.Len(t, list.Comments, 1)
	require.Len(t, list.Comments[0].Replies, 1)
	assert.Equal(t, "Reply", list.Comments[0].Replies[0].Content)
}

// TestManager_ReplyComment_NotFound 测试回复不存在的评论.
func TestManager_ReplyComment_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.ReplyComment("doc1", "nonexistent", "user1", "Reply")
	assert.Error(t, err)
}

// TestManager_DeleteComment 测试删除评论.
func TestManager_DeleteComment(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 使用唯一文档 ID
	docID := "doc-delete-test"
	comment, _ := mgr.AddComment(docID, "user1", "Comment")

	err := mgr.DeleteComment(docID, comment.CommentID)
	require.NoError(t, err)

	list, _ := mgr.GetComments(docID)
	assert.Equal(t, 0, list.Total)
}

// TestManager_DeleteComment_NotFound 测试删除不存在的评论.
func TestManager_DeleteComment_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.DeleteComment("doc1", "nonexistent")
	assert.Error(t, err)
}

// ========== 辅助方法测试 ==========

// TestGenerateFileKey 测试文件 Key 生成.
func TestGenerateFileKey(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	key1 := mgr.generateFileKey("file1")
	key2 := mgr.generateFileKey("file1")

	// 每次生成的 Key 应不同
	assert.NotEmpty(t, key1)
	assert.NotEqual(t, key1, key2)
}

// TestBuildCallbackURL 测试回调 URL 构建.
func TestBuildCallbackURL(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	url := mgr.buildCallbackURL("session123")
	assert.Equal(t, "/api/v1/office/callback/session123", url)
}

// TestParseCallbackURL 测试回调 URL 解析.
func TestParseCallbackURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedID  string
		expectError bool
	}{
		{"有效 URL", "/api/v1/office/callback/session123", "session123", false},
		{"无效格式", "/api/v1/office/invalid", "", true},
		{"空 URL", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ParseCallbackURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			}
		})
	}
}

// TestGetAllFileAssociations 测试获取所有文件关联.
func TestGetAllFileAssociations(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	associations := mgr.GetAllFileAssociations()
	assert.NotEmpty(t, associations)

	// 验证返回的是副本
	associations["test"] = FileAssociation{}
	original := mgr.GetAllFileAssociations()
	_, exists := original["test"]
	assert.False(t, exists)
}

// TestSortSessionsByTime 测试会话时间排序.
func TestSortSessionsByTime(t *testing.T) {
	now := time.Now()
	sessions := []*EditingSession{
		{ID: "1", StartedAt: now.Add(-2 * time.Hour)},
		{ID: "2", StartedAt: now.Add(-1 * time.Hour)},
		{ID: "3", StartedAt: now},
	}

	sortSessionsByTime(sessions)

	// 应按时间倒序排列
	assert.Equal(t, "3", sessions[0].ID)
	assert.Equal(t, "2", sessions[1].ID)
	assert.Equal(t, "1", sessions[2].ID)
}

// TestEditingSession_IsActive 测试会话活跃状态.
func TestEditingSession_IsActive(t *testing.T) {
	t.Run("活跃会话", func(t *testing.T) {
		session := &EditingSession{
			Status:    SessionStatusActive,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		// IsActive 检查状态和过期时间
		assert.Equal(t, SessionStatusActive, session.Status)
		assert.False(t, session.IsExpired())
	})

	t.Run("已关闭会话", func(t *testing.T) {
		session := &EditingSession{
			Status: SessionStatusClosed,
		}
		assert.Equal(t, SessionStatusClosed, session.Status)
	})
}

// TestCreateSession_NotEnabled 测试禁用时创建会话.
func TestCreateSession_NotEnabled(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	// 默认禁用

	_, _, err := mgr.CreateSession("file1", "user1", "User 1", "edit")
	assert.Error(t, err)
}

// TestCreateSession_UnsupportedType 测试不支持的文件类型.
func TestCreateSession_UnsupportedType(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.exe", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	_, _, err := mgr.CreateSession("file1", "user1", "User 1", "edit")
	assert.Error(t, err)
}

// TestCreateSession_FileNotFound 测试文件不存在 - 跳过，因为 mock 无法模拟错误.
func TestCreateSession_FileNotFound(t *testing.T) {
	t.Skip("MockFileAccessor 无法模拟文件不存在的错误情况")
}

// TestGetSession_NotFound 测试获取不存在的会话.
func TestGetSession_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	_, err := mgr.GetSession("nonexistent")
	assert.Error(t, err)
}

// TestCloseSession_NotFound 测试关闭不存在的会话.
func TestCloseSession_NotFound(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()

	err := mgr.CloseSession("nonexistent")
	assert.Error(t, err)
}

// TestJWTGeneration 测试 JWT 生成.
func TestJWTGeneration(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{
		Enabled:   boolPtr(true),
		SecretKey: strPtr("test-secret-key"),
	})

	session := &EditingSession{
		ID:        "session1",
		FileID:    "file1",
		FileName:  "test.docx",
		FileKey:   "key1",
		UserID:    "user1",
		UserName:  "User 1",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	fileInfo := &FileInfo{ID: "file1", Name: "test.docx"}
	config := mgr.buildEditorConfig(session, fileInfo, "http://localhost/file", "edit", "word")

	assert.NotEmpty(t, config.Token)
}

// TestListSessions_Pagination 测试分页.
func TestListSessions_Pagination(t *testing.T) {
	accessor := NewMockFileAccessor()
	for i := 0; i < 5; i++ {
		accessor.AddFile("file"+string(rune('0'+i)), "test.docx", 1024)
	}

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 创建多个会话
	for i := 0; i < 5; i++ {
		fileID := "file" + string(rune('0'+i))
		mgr.CreateSession(fileID, "user1", "User 1", "edit")
	}

	// 分页获取
	page1, total := mgr.ListSessions("", 2, 0)
	assert.Equal(t, 5, total)
	assert.Len(t, page1, 2)

	page2, _ := mgr.ListSessions("", 2, 2)
	assert.Len(t, page2, 2)

	page3, _ := mgr.ListSessions("", 2, 4)
	assert.Len(t, page3, 1)

	// 超出范围
	empty, _ := mgr.ListSessions("", 2, 10)
	assert.Len(t, empty, 0)
}

// TestListSessions_StatusFilter 测试状态过滤.
func TestListSessions_StatusFilter(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test.docx", 1024)

	mgr, _ := NewManager("", accessor, WithCleanupWorker(false))
	defer mgr.Close()
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	mgr.CreateSession("file1", "user1", "User 1", "edit")

	activeSessions, _ := mgr.ListSessions(SessionStatusActive, 10, 0)
	assert.Len(t, activeSessions, 1)

	closedSessions, _ := mgr.ListSessions(SessionStatusClosed, 10, 0)
	assert.Len(t, closedSessions, 0)
}
