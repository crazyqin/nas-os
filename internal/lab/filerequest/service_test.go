package filerequest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 创建请求测试 ==========

func TestCreateRequest(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	req := CreateRequestRequest{
		Title:           "收集项目文档",
		Description:     "请上传项目相关文档",
		CreatorID:       "user1",
		CreatorName:     "张三",
		DestinationPath: "/shared/projects/docs",
		MaxFileCount:    10,
		MaxFileSize:     100 * 1024 * 1024,
	}

	result, err := svc.CreateRequest(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "收集项目文档", result.Title)
	assert.Equal(t, RequestStatusActive, result.Status)
	assert.Equal(t, 10, result.MaxFileCount)
	assert.False(t, result.HasPassword)
}

func TestCreateRequest_WithPassword(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	req := CreateRequestRequest{
		Title:           "加密收集",
		CreatorID:       "user1",
		CreatorName:     "李四",
		DestinationPath: "/secure/uploads",
		Password:        "secret123",
	}

	result, err := svc.CreateRequest(ctx, req)
	require.NoError(t, err)
	assert.True(t, result.HasPassword)
}

func TestCreateRequest_WithExpiry(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	expires := time.Now().Add(24 * time.Hour)
	req := CreateRequestRequest{
		Title:           "限时收集",
		CreatorID:       "user1",
		CreatorName:     "王五",
		DestinationPath: "/uploads",
		ExpiresAt:       &expires,
	}

	result, err := svc.CreateRequest(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result.ExpiresAt)
	assert.True(t, result.ExpiresAt.After(time.Now()))
}

// ========== 列出请求测试 ==========

func TestListRequests(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	svc.CreateRequest(ctx, CreateRequestRequest{Title: "R1", CreatorID: "u1", CreatorName: "A", DestinationPath: "/a"})
	svc.CreateRequest(ctx, CreateRequestRequest{Title: "R2", CreatorID: "u1", CreatorName: "A", DestinationPath: "/b"})
	svc.CreateRequest(ctx, CreateRequestRequest{Title: "R3", CreatorID: "u2", CreatorName: "B", DestinationPath: "/c"})

	// 按创建者过滤
	list, total, err := svc.ListRequests(ctx, "u1", "", 1, 10)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, 2, total)

	// 全部
	list, total, _ = svc.ListRequests(ctx, "", "", 1, 10)
	assert.Len(t, list, 3)
	assert.Equal(t, 3, total)

	// 分页
	list, total, _ = svc.ListRequests(ctx, "", "", 1, 2)
	assert.Len(t, list, 2)
	assert.Equal(t, 3, total)
}

func TestListRequests_Empty(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	list, total, err := svc.ListRequests(ctx, "", "", 1, 10)
	require.NoError(t, err)
	assert.Nil(t, list)
	assert.Equal(t, 0, total)
}

// ========== 获取请求测试 ==========

func TestGetRequest(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "测试", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	got, err := svc.GetRequest(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "测试", got.Title)
}

func TestGetRequest_NotFound(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.GetRequest(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ========== 令牌访问测试 ==========

func TestGetRequestByToken(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "令牌测试", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	// 获取链接
	link, err := svc.GetLinkByToken(ctx, "")
	// 令牌为空应失败
	assert.Error(t, err)

	// 通过创建时的链接获取
	// 遍历 links 找到对应 token
	link, err = svc.GetLinkByToken(ctx, getFirstToken(svc, created.ID))
	require.NoError(t, err)
	assert.Equal(t, created.ID, link.RequestID)

	// 通过令牌获取请求
	req, err := svc.GetRequestByToken(ctx, link.Token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, req.ID)
}

func TestGetRequestByToken_InvalidToken(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	_, err := svc.GetRequestByToken(ctx, "invalid-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效")
}

// ========== 密码验证测试 ==========

func TestVerifyPassword_NoPassword(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "无密码", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	token := getFirstToken(svc, created.ID)
	err := svc.VerifyPassword(ctx, token, "")
	assert.NoError(t, err) // 无密码保护
}

func TestVerifyPassword_Correct(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "有密码", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
		Password: "mypassword",
	})

	token := getFirstToken(svc, created.ID)
	err := svc.VerifyPassword(ctx, token, "mypassword")
	assert.NoError(t, err)
}

func TestVerifyPassword_Wrong(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "有密码", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
		Password: "correct",
	})

	token := getFirstToken(svc, created.ID)
	err := svc.VerifyPassword(ctx, token, "wrong")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "密码错误")
}

// ========== 上传记录测试 ==========

func TestRecordUpload(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "上传测试", CreatorID: "u1", CreatorName: "A", DestinationPath: "/uploads",
		MaxFileCount: 5, MaxFileSize: 1024 * 1024,
	})

	token := getFirstToken(svc, created.ID)

	upload := &UploadFileRequest{
		OriginalName: "report.pdf",
		FileSize:     50000,
		MimeType:     "application/pdf",
		UploaderName: "上传者A",
	}

	result, err := svc.RecordUpload(ctx, token, upload, "192.168.1.100")
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "report.pdf", result.OriginalName)
	assert.Equal(t, UploadStatusSuccess, result.Status)
	assert.Equal(t, ".pdf", result.Extension)
}

func TestRecordUpload_ExceedFileCount(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "限制数量", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
		MaxFileCount: 2,
	})

	token := getFirstToken(svc, created.ID)

	// 上传2个文件（达到上限）
	svc.RecordUpload(ctx, token, &UploadFileRequest{OriginalName: "f1.txt", FileSize: 10}, "")
	svc.RecordUpload(ctx, token, &UploadFileRequest{OriginalName: "f2.txt", FileSize: 10}, "")

	// 第3个应失败
	_, err := svc.RecordUpload(ctx, token, &UploadFileRequest{OriginalName: "f3.txt", FileSize: 10}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "最大文件数量限制")
}

func TestRecordUpload_ExceedFileSize(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "限制大小", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
		MaxFileSize: 1000,
	})

	token := getFirstToken(svc, created.ID)

	_, err := svc.RecordUpload(ctx, token, &UploadFileRequest{
		OriginalName: "big.bin", FileSize: 2000,
	}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "超过限制")
}

func TestRecordUpload_ExtensionFilter(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "限制类型", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
		AllowedExtensions: []string{".txt", ".pdf"},
	})

	token := getFirstToken(svc, created.ID)

	// 允许的文件
	_, err := svc.RecordUpload(ctx, token, &UploadFileRequest{
		OriginalName: "doc.txt", FileSize: 10,
	}, "")
	assert.NoError(t, err)

	// 不允许的文件
	_, err = svc.RecordUpload(ctx, token, &UploadFileRequest{
		OriginalName: "script.exe", FileSize: 10,
	}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不在允许列表中")
}

func TestRecordUpload_ClosedRequest(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "已关闭", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	token := getFirstToken(svc, created.ID)
	svc.CloseRequest(ctx, created.ID)

	_, err := svc.RecordUpload(ctx, token, &UploadFileRequest{
		OriginalName: "f.txt", FileSize: 10,
	}, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已关闭")
}

// ========== 上传列表测试 ==========

func TestGetUploads(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "上传列表", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	token := getFirstToken(svc, created.ID)

	svc.RecordUpload(ctx, token, &UploadFileRequest{OriginalName: "a.txt", FileSize: 100}, "")
	svc.RecordUpload(ctx, token, &UploadFileRequest{OriginalName: "b.txt", FileSize: 200}, "")

	uploads, err := svc.GetUploads(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, uploads, 2)
}

func TestGetUploads_Empty(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "空", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	uploads, err := svc.GetUploads(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, uploads)
}

// ========== 删除请求测试 ==========

func TestDeleteRequest(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "待删除", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	token := getFirstToken(svc, created.ID)

	// 上传一个文件
	svc.RecordUpload(ctx, token, &UploadFileRequest{OriginalName: "f.txt", FileSize: 10}, "")

	// 删除请求
	err := svc.DeleteRequest(ctx, created.ID)
	require.NoError(t, err)

	// 请求已删除
	_, err = svc.GetRequest(ctx, created.ID)
	assert.Error(t, err)

	// 令牌也失效
	_, err = svc.GetRequestByToken(ctx, token)
	assert.Error(t, err)

	// 上传列表也清理
	uploads, _ := svc.GetUploads(ctx, created.ID)
	assert.Nil(t, uploads)
}

func TestDeleteRequest_NotFound(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	err := svc.DeleteRequest(ctx, "nonexistent")
	assert.Error(t, err)
}

// ========== 关闭请求测试 ==========

func TestCloseRequest(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "待关闭", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	err := svc.CloseRequest(ctx, created.ID)
	require.NoError(t, err)

	got, _ := svc.GetRequest(ctx, created.ID)
	assert.Equal(t, RequestStatusClosed, got.Status)
}

func TestCloseRequest_NotFound(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	err := svc.CloseRequest(ctx, "nonexistent")
	assert.Error(t, err)
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	req1, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "S1", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})
	req2, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "S2", CreatorID: "u1", CreatorName: "A", DestinationPath: "/y",
	})

	token1 := getFirstToken(svc, req1.ID)
	token2 := getFirstToken(svc, req2.ID)

	svc.RecordUpload(ctx, token1, &UploadFileRequest{OriginalName: "a.txt", FileSize: 1000}, "")
	svc.RecordUpload(ctx, token1, &UploadFileRequest{OriginalName: "b.txt", FileSize: 2000}, "")
	svc.RecordUpload(ctx, token2, &UploadFileRequest{OriginalName: "c.txt", FileSize: 500}, "")

	stats, err := svc.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalRequests)
	assert.Equal(t, 2, stats.ActiveRequests)
	assert.Equal(t, 3, stats.TotalUploads)
	assert.Equal(t, int64(3500), stats.TotalUploadSize)
}

// ========== 过期检查测试 ==========

func TestCheckExpired(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	// 创建已过期的请求
	past := time.Now().Add(-1 * time.Hour)
	svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "已过期", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
		ExpiresAt: &past,
	})

	// 创建未过期的请求
	future := time.Now().Add(1 * time.Hour)
	svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "未过期", CreatorID: "u1", CreatorName: "A", DestinationPath: "/y",
		ExpiresAt: &future,
	})

	// 创建无过期时间的请求
	svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "无过期", CreatorID: "u1", CreatorName: "A", DestinationPath: "/z",
	})

	count := svc.CheckExpired(ctx)
	assert.Equal(t, 1, count)

	// 验证过期状态
	list, _, _ := svc.ListRequests(ctx, "", RequestStatusExpired, 1, 10)
	assert.Len(t, list, 1)
}

// ========== 删除上传记录测试 ==========

func TestDeleteUpload(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "删除上传", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	token := getFirstToken(svc, created.ID)

	result, _ := svc.RecordUpload(ctx, token, &UploadFileRequest{
		OriginalName: "del.txt", FileSize: 100,
	}, "")

	err := svc.DeleteUpload(ctx, created.ID, result.ID)
	require.NoError(t, err)

	uploads, _ := svc.GetUploads(ctx, created.ID)
	assert.Empty(t, uploads)
}

func TestDeleteUpload_NotFound(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "测试", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	err := svc.DeleteUpload(ctx, created.ID, "nonexistent")
	assert.Error(t, err)
}

// ========== 更新请求测试 ==========

func TestUpdateRequest(t *testing.T) {
	svc := NewService()
	ctx := context.Background()

	created, _ := svc.CreateRequest(ctx, CreateRequestRequest{
		Title: "原标题", CreatorID: "u1", CreatorName: "A", DestinationPath: "/x",
	})

	newExpiry := time.Now().Add(48 * time.Hour)
	updated, err := svc.UpdateRequest(ctx, created.ID, "新标题", "新描述", &newExpiry)
	require.NoError(t, err)
	assert.Equal(t, "新标题", updated.Title)
	assert.Equal(t, "新描述", updated.Description)
}

// ========== 辅助函数 ==========

// getFirstToken 获取请求的第一个链接令牌（测试辅助）.
func getFirstToken(svc *Service, requestID string) string {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	for _, link := range svc.links {
		if link.RequestID == requestID {
			return link.Token
		}
	}
	return ""
}
