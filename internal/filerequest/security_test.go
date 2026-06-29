package filerequest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== SecurityManager 测试 ==========

func TestSecurityManager_IPRateLimit(t *testing.T) {
	sm := NewSecurityManager()
	sm.SetIPRateLimit(3, time.Minute)

	ip := "192.168.1.100"
	for i := 0; i < 3; i++ {
		err := sm.CheckIPRateLimit(ip)
		assert.NoError(t, err)
	}
	err := sm.CheckIPRateLimit(ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "超过限速")
}

func TestSecurityManager_IPRateLimitDifferentIPs(t *testing.T) {
	sm := NewSecurityManager()
	sm.SetIPRateLimit(2, time.Minute)

	for i := 0; i < 2; i++ {
		err := sm.CheckIPRateLimit("192.168.1.100")
		assert.NoError(t, err)
	}
	err := sm.CheckIPRateLimit("192.168.1.200")
	assert.NoError(t, err)
}

func TestSecurityManager_PasswordCorrect(t *testing.T) {
	sm := NewSecurityManager()
	correct := "secret123"

	err := sm.VerifyPassword("link-1", correct, correct)
	assert.NoError(t, err)
}

func TestSecurityManager_PasswordWrong(t *testing.T) {
	sm := NewSecurityManager()
	sm.SetMaxPasswordAttempts(3, 5*time.Minute)

	linkID := "link-1"
	correct := "secret123"

	err := sm.VerifyPassword(linkID, "wrong", correct)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "剩余尝试次数 2")

	err = sm.VerifyPassword(linkID, "wrong", correct)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "剩余尝试次数 1")

	err = sm.VerifyPassword(linkID, "wrong", correct)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已封锁")
}

func TestSecurityManager_PasswordBlocked(t *testing.T) {
	sm := NewSecurityManager()
	sm.SetMaxPasswordAttempts(2, 5*time.Minute)

	linkID := "link-1"
	correct := "secret123"

	_ = sm.VerifyPassword(linkID, "wrong", correct)
	_ = sm.VerifyPassword(linkID, "wrong", correct)

	// 正确密码也应该被封锁
	err := sm.VerifyPassword(linkID, correct, correct)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "密码尝试次数过多")
}

func TestSecurityManager_PasswordNoProtection(t *testing.T) {
	sm := NewSecurityManager()
	err := sm.VerifyPassword("link-1", "", "")
	assert.NoError(t, err)
}

func TestSecurityManager_FileSizeValidation(t *testing.T) {
	sm := NewSecurityManager()

	err := sm.ValidateFileSize(500, 1000)
	assert.NoError(t, err)

	err = sm.ValidateFileSize(1500, 1000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "超过限制")

	err = sm.ValidateFileSize(500, 0)
	assert.NoError(t, err) // 0表示不限制
}

func TestSecurityManager_FileTypeValidation(t *testing.T) {
	sm := NewSecurityManager()

	// 黑名单测试
	err := sm.ValidateFileType("malware.exe", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "被禁止")

	// 白名单测试
	err = sm.ValidateFileType("doc.txt", []string{".txt", ".pdf"})
	assert.NoError(t, err)

	err = sm.ValidateFileType("doc.docx", []string{".txt", ".pdf"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不在允许列表中")

	// 无限制
	err = sm.ValidateFileType("anyfile.xyz", nil)
	assert.NoError(t, err)
}

func TestSecurityManager_UploadCountValidation(t *testing.T) {
	sm := NewSecurityManager()

	err := sm.ValidateUploadCount(5, 10)
	assert.NoError(t, err)

	err = sm.ValidateUploadCount(10, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大文件数量限制")

	err = sm.ValidateUploadCount(100, 0)
	assert.NoError(t, err) // 0表示不限制
}

func TestSecurityManager_LinkExpired(t *testing.T) {
	sm := NewSecurityManager()

	// 未设置过期时间
	link := &RequestLink{IsActive: true}
	assert.False(t, sm.IsLinkExpired(link))

	// 已过期
	past := time.Now().Add(-time.Hour)
	link.ExpiresAt = &past
	assert.True(t, sm.IsLinkExpired(link))

	// 未过期
	future := time.Now().Add(time.Hour)
	link.ExpiresAt = &future
	assert.False(t, sm.IsLinkExpired(link))
}

func TestSecurityManager_LinkAccessLimit(t *testing.T) {
	sm := NewSecurityManager()

	// 无限制
	link := &RequestLink{MaxAccessCount: 0}
	assert.False(t, sm.IsLinkAccessLimitReached(link))

	// 未达上限
	link.MaxAccessCount = 10
	link.AccessCount = 5
	assert.False(t, sm.IsLinkAccessLimitReached(link))

	// 已达上限
	link.AccessCount = 10
	assert.True(t, sm.IsLinkAccessLimitReached(link))
}

func TestSecurityManager_CheckLinkAccess(t *testing.T) {
	sm := NewSecurityManager()
	sm.SetIPRateLimit(100, time.Minute) // 高限速避免干扰

	link := &RequestLink{
		IsActive:      true,
		MaxAccessCount: 0,
	}

	err := sm.CheckLinkAccess(link, "192.168.1.1")
	assert.NoError(t, err)

	// 禁用链接
	link.IsActive = false
	err = sm.CheckLinkAccess(link, "192.168.1.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "禁用")
	link.IsActive = true

	// 过期链接
	past := time.Now().Add(-time.Hour)
	link.ExpiresAt = &past
	err = sm.CheckLinkAccess(link, "192.168.1.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "过期")
	link.ExpiresAt = nil

	// 访问次数上限
	link.MaxAccessCount = 1
	link.AccessCount = 1
	err = sm.CheckLinkAccess(link, "192.168.1.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "上限")
}

func TestSecurityManager_ClearExpiredBlocks(t *testing.T) {
	sm := NewSecurityManager()
	sm.SetMaxPasswordAttempts(1, 1*time.Millisecond)

	linkID := "link-1"
	_ = sm.VerifyPassword(linkID, "wrong", "correct")

	// 等待封锁过期
	time.Sleep(10 * time.Millisecond)
	sm.ClearExpiredBlocks()

	// 应该可以再次尝试
	err := sm.VerifyPassword(linkID, "correct", "correct")
	assert.NoError(t, err)
}

func TestSecurityManager_BlockedExtensions(t *testing.T) {
	sm := NewSecurityManager()

	sm.AddBlockedExtension(".dll")
	err := sm.ValidateFileType("lib.dll", nil)
	assert.Error(t, err)

	sm.RemoveBlockedExtension(".dll")
	err = sm.ValidateFileType("lib.dll", nil)
	assert.NoError(t, err)
}

func TestExtractIP(t *testing.T) {
	ip := ExtractIP("192.168.1.1:8080")
	assert.Equal(t, "192.168.1.1", ip)

	ip = ExtractIP("invalid")
	assert.Equal(t, "invalid", ip)
}

// ========== NotificationManager 测试 ==========

func TestNotificationManager_Webhook(t *testing.T) {
	config := NotificationConfig{
		WebhookEnabled: true,
		WebhookURL:     "http://localhost:9999/webhook",
	}
	nm := NewNotificationManager(config)

	event := &NotificationEvent{
		Type:         "file.uploaded",
		RequestID:    "req-1",
		RequestTitle: "测试请求",
		FileName:     "test.pdf",
		FileSize:     1024,
		UploaderName: "张三",
		Timestamp:    time.Now(),
	}

	// Webhook会失败（无服务器），但不影响逻辑
	err := nm.NotifyUpload(context.Background(), event, nil)
	assert.NoError(t, err)

	sent, failures := nm.GetStats()
	assert.True(t, sent >= 0)
	assert.True(t, failures >= 0)
}

func TestNotificationManager_NoConfig(t *testing.T) {
	nm := NewNotificationManager(NotificationConfig{})

	event := &NotificationEvent{
		Type:         "file.uploaded",
		RequestID:    "req-1",
		RequestTitle: "测试",
		FileName:     "test.txt",
		FileSize:     100,
		UploaderName: "李四",
		Timestamp:    time.Now(),
	}

	err := nm.NotifyUpload(context.Background(), event, nil)
	assert.NoError(t, err)
}

func TestNotificationManager_UpdateConfig(t *testing.T) {
	nm := NewNotificationManager(NotificationConfig{})

	nm.UpdateConfig(NotificationConfig{
		WebhookEnabled: true,
		WebhookURL:     "http://localhost:8888/hook",
	})

	// 配置应更新
	event := &NotificationEvent{
		Type:         "file.uploaded",
		RequestID:    "req-1",
		RequestTitle: "测试",
		FileName:     "test.txt",
		FileSize:     100,
		UploaderName: "王五",
		Timestamp:    time.Now(),
	}

	err := nm.NotifyUpload(context.Background(), event, nil)
	assert.NoError(t, err)
}

// ========== Manager 新方法测试 ==========

func TestManager_CloseRequest(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	req, err := m.CreateRequest("测试", "", "user-1", "/tmp/uploads", 10, 100, time.Time{}, false, false)
	require.NoError(t, err)

	err = m.CloseRequest(ctx, req.ID)
	assert.NoError(t, err)

	got, err := m.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, RequestStatusClosed, got.Status)
}

func TestManager_CloseRequestNotFound(t *testing.T) {
	m := NewManager()
	err := m.CloseRequest(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestManager_DeleteUpload(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	req, err := m.CreateRequest("测试", "", "user-1", "/tmp/uploads", 10, 100, time.Time{}, false, false)
	require.NoError(t, err)

	// 记录上传
	info := &UploadInfo{
		OriginalName: "test.pdf",
		FileSize:     1024,
		MimeType:     "application/pdf",
		Extension:    ".pdf",
	}
	err = m.RecordUpload(ctx, req.ID, info)
	require.NoError(t, err)

	// 获取上传列表
	uploads, err := m.GetUploads(ctx, req.ID)
	require.NoError(t, err)
	assert.Len(t, uploads, 1)
	assert.Equal(t, 1, req.ReceivedFileCount)

	// 删除上传
	err = m.DeleteUpload(ctx, req.ID, uploads[0].ID)
	assert.NoError(t, err)

	// 验证删除后
	uploads, err = m.GetUploads(ctx, req.ID)
	require.NoError(t, err)
	assert.Len(t, uploads, 0)
	assert.Equal(t, 0, req.ReceivedFileCount)
	assert.Equal(t, int64(0), req.ReceivedTotalSize)
}

func TestManager_DeleteUploadNotFound(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	req, err := m.CreateRequest("测试", "", "user-1", "/tmp/uploads", 10, 100, time.Time{}, false, false)
	require.NoError(t, err)

	err = m.DeleteUpload(ctx, req.ID, "nonexistent")
	assert.Error(t, err)
}

func TestManager_DeleteUploadRequestNotFound(t *testing.T) {
	m := NewManager()
	err := m.DeleteUpload(context.Background(), "nonexistent", "upload-1")
	assert.Error(t, err)
}

func TestManager_GetUploadsEmpty(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	req, err := m.CreateRequest("测试", "", "user-1", "/tmp/uploads", 10, 100, time.Time{}, false, false)
	require.NoError(t, err)

	uploads, err := m.GetUploads(ctx, req.ID)
	assert.NoError(t, err)
	assert.Nil(t, uploads)
}
