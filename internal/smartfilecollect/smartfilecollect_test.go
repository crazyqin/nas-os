// Package smartfilecollect - 单元测试
package smartfilecollect

import (
	"bytes"
	"testing"
	"time"
)

func TestDefaultCollectConfig(t *testing.T) {
	config := DefaultCollectConfig()

	if config.MaxFileSize != 1<<30 {
		t.Errorf("MaxFileSize 期望 1GB, 实际 %d", config.MaxFileSize)
	}

	if config.MaxTotalSize != 10<<30 {
		t.Errorf("MaxTotalSize 期望 10GB, 实际 %d", config.MaxTotalSize)
	}

	if config.DefaultExpireDays != 7 {
		t.Errorf("DefaultExpireDays 期望 7, 实际 %d", config.DefaultExpireDays)
	}

	if config.MaxExpireDays != 30 {
		t.Errorf("MaxExpireDays 期望 30, 实际 %d", config.MaxExpireDays)
	}
}

func TestNewCollectManager(t *testing.T) {
	manager := NewCollectManager(nil)
	if manager == nil {
		t.Fatal("NewCollectManager 返回 nil")
	}

	if manager.requests == nil {
		t.Fatal("requests map 为 nil")
	}

	if manager.submissions == nil {
		t.Fatal("submissions map 为 nil")
	}
}

func TestCreateCollectRequest(t *testing.T) {
	manager := NewCollectManager(nil)

	req := &CreateCollectRequest{
		Title:       "测试收集",
		Description: "测试描述",
		TargetPath:  "/data/test",
		ExpiresIn:   7,
	}

	result, err := manager.CreateCollectRequest(req, "user1", "测试用户")
	if err != nil {
		t.Fatalf("CreateCollectRequest 失败: %v", err)
	}

	if result.ID == "" {
		t.Error("ID 为空")
	}

	if result.Title != "测试收集" {
		t.Errorf("Title 期望 '测试收集', 实际 '%s'", result.Title)
	}

	if result.Status != CollectStatusActive {
		t.Errorf("Status 期望 active, 实际 %s", result.Status)
	}

	if result.ExpiresAt == nil {
		t.Error("ExpiresAt 为 nil")
	}

	// 验证过期时间
	expectedExpiry := time.Now().AddDate(0, 0, 7)
	if result.ExpiresAt.Sub(expectedExpiry) > time.Minute {
		t.Error("过期时间计算错误")
	}
}

func TestGetCollectRequest(t *testing.T) {
	manager := NewCollectManager(nil)

	req := &CreateCollectRequest{
		Title:      "测试",
		TargetPath: "/data/test",
	}

	created, _ := manager.CreateCollectRequest(req, "user1", "用户1")

	// 测试获取存在的请求
	result, err := manager.GetCollectRequest(created.ID)
	if err != nil {
		t.Fatalf("GetCollectRequest 失败: %v", err)
	}

	if result.ID != created.ID {
		t.Errorf("ID 不匹配")
	}

	// 测试获取不存在的请求
	_, err = manager.GetCollectRequest("not-exist")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestListCollectRequests(t *testing.T) {
	manager := NewCollectManager(nil)

	// 创建多个请求
	req1 := &CreateCollectRequest{Title: "请求1", TargetPath: "/data/1"}
	req2 := &CreateCollectRequest{Title: "请求2", TargetPath: "/data/2"}
	req3 := &CreateCollectRequest{Title: "请求3", TargetPath: "/data/3"}

	manager.CreateCollectRequest(req1, "user1", "用户1")
	manager.CreateCollectRequest(req2, "user1", "用户1")
	manager.CreateCollectRequest(req3, "user2", "用户2")

	// 列出所有
	all := manager.ListCollectRequests("")
	if len(all) != 3 {
		t.Errorf("期望 3 个请求, 实际 %d", len(all))
	}

	// 列出特定用户的
	user1 := manager.ListCollectRequests("user1")
	if len(user1) != 2 {
		t.Errorf("期望 2 个请求, 实际 %d", len(user1))
	}
}

func TestUpdateCollectRequest(t *testing.T) {
	manager := NewCollectManager(nil)

	req := &CreateCollectRequest{Title: "原始标题", TargetPath: "/data/test"}
	created, _ := manager.CreateCollectRequest(req, "user1", "用户1")

	// 更新标题
	err := manager.UpdateCollectRequest(created.ID, map[string]interface{}{
		"title": "新标题",
	})
	if err != nil {
		t.Fatalf("UpdateCollectRequest 失败: %v", err)
	}

	updated, _ := manager.GetCollectRequest(created.ID)
	if updated.Title != "新标题" {
		t.Errorf("Title 期望 '新标题', 实际 '%s'", updated.Title)
	}
}

func TestDeleteCollectRequest(t *testing.T) {
	manager := NewCollectManager(nil)

	req := &CreateCollectRequest{Title: "待删除", TargetPath: "/data/test"}
	created, _ := manager.CreateCollectRequest(req, "user1", "用户1")

	// 删除
	err := manager.DeleteCollectRequest(created.ID)
	if err != nil {
		t.Fatalf("DeleteCollectRequest 失败: %v", err)
	}

	// 验证已删除
	_, err = manager.GetCollectRequest(created.ID)
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestSubmitFile(t *testing.T) {
	config := DefaultCollectConfig()
	config.TempPath = t.TempDir()
	config.StoragePath = t.TempDir()
	manager := NewCollectManager(&config)

	req := &CreateCollectRequest{
		Title:      "测试收集",
		TargetPath: t.TempDir(),
	}
	created, _ := manager.CreateCollectRequest(req, "user1", "用户1")

	// 提交文件
	fileContent := []byte("测试文件内容")
	reader := bytes.NewReader(fileContent)

	submitReq := &SubmitFileRequest{
		SubmitterName:  "测试提交者",
		SubmitterEmail: "test@example.com",
	}

	submission, err := manager.SubmitFile(created.ID, reader, "test.txt", submitReq, "192.168.1.1")
	if err != nil {
		t.Fatalf("SubmitFile 失败: %v", err)
	}

	if submission.FileName != "test.txt" {
		t.Errorf("FileName 期望 'test.txt', 实际 '%s'", submission.FileName)
	}

	if submission.FileSize != int64(len(fileContent)) {
		t.Errorf("FileSize 期望 %d, 实际 %d", len(fileContent), submission.FileSize)
	}

	if submission.SubmitterName != "测试提交者" {
		t.Errorf("SubmitterName 期望 '测试提交者', 实际 '%s'", submission.SubmitterName)
	}
}

func TestSubmitDuplicateFile(t *testing.T) {
	config := DefaultCollectConfig()
	config.TempPath = t.TempDir()
	config.StoragePath = t.TempDir()
	config.EnableDedup = true
	manager := NewCollectManager(&config)

	req := &CreateCollectRequest{
		Title:      "去重测试",
		TargetPath: t.TempDir(),
	}
	created, _ := manager.CreateCollectRequest(req, "user1", "用户1")

	submitReq := &SubmitFileRequest{SubmitterName: "用户"}

	// 第一次提交
	content := []byte("相同内容")
	reader1 := bytes.NewReader(content)
	_, err := manager.SubmitFile(created.ID, reader1, "file1.txt", submitReq, "192.168.1.1")
	if err != nil {
		t.Fatalf("第一次提交失败: %v", err)
	}

	// 第二次提交相同内容
	reader2 := bytes.NewReader(content)
	submission2, err := manager.SubmitFile(created.ID, reader2, "file2.txt", submitReq, "192.168.1.1")
	if err != nil {
		t.Fatalf("第二次提交失败: %v", err)
	}

	if submission2.Status != SubmissionStatusDuplicate {
		t.Errorf("期望状态 duplicate, 实际 %s", submission2.Status)
	}
}

func TestFileClassification(t *testing.T) {
	manager := NewCollectManager(nil)

	tests := []struct {
		ext      string
		expected FileCategory
	}{
		{".doc", CategoryDocument},
		{".pdf", CategoryDocument},
		{".jpg", CategoryImage},
		{".png", CategoryImage},
		{".mp4", CategoryVideo},
		{".mp3", CategoryAudio},
		{".zip", CategoryArchive},
		{".go", CategoryCode},
		{".xyz", CategoryOther},
	}

	for _, test := range tests {
		result := manager.classifyFile(test.ext)
		if result != test.expected {
			t.Errorf("扩展名 %s: 期望 %s, 实际 %s", test.ext, test.expected, result)
		}
	}
}

func TestIsAllowedExt(t *testing.T) {
	manager := NewCollectManager(nil)

	// 测试白名单为空（允许所有）
	if !manager.isAllowedExt(".txt", []string{}, []string{}) {
		t.Error("白名单为空时应允许所有")
	}

	// 测试白名单
	allowed := []string{".txt", ".pdf"}
	if !manager.isAllowedExt(".txt", allowed, []string{}) {
		t.Error(".txt 应被允许")
	}
	if manager.isAllowedExt(".exe", allowed, []string{}) {
		t.Error(".exe 不应被允许")
	}

	// 测试黑名单
	blocked := []string{".exe", ".bat"}
	if manager.isAllowedExt(".exe", []string{}, blocked) {
		t.Error(".exe 应被阻止")
	}
	if !manager.isAllowedExt(".txt", []string{}, blocked) {
		t.Error(".txt 不应被阻止")
	}
}

func TestPauseResumeClose(t *testing.T) {
	manager := NewCollectManager(nil)

	req := &CreateCollectRequest{Title: "测试", TargetPath: "/data/test"}
	created, _ := manager.CreateCollectRequest(req, "user1", "用户1")

	// 暂停
	manager.PauseCollectRequest(created.ID)
	result, _ := manager.GetCollectRequest(created.ID)
	if result.Status != CollectStatusPaused {
		t.Errorf("期望 paused, 实际 %s", result.Status)
	}

	// 恢复
	manager.ResumeCollectRequest(created.ID)
	result, _ = manager.GetCollectRequest(created.ID)
	if result.Status != CollectStatusActive {
		t.Errorf("期望 active, 实际 %s", result.Status)
	}

	// 关闭
	manager.CloseCollectRequest(created.ID)
	result, _ = manager.GetCollectRequest(created.ID)
	if result.Status != CollectStatusClosed {
		t.Errorf("期望 closed, 实际 %s", result.Status)
	}
}

func TestValidateAccessToken(t *testing.T) {
	manager := NewCollectManager(nil)

	req := &CreateCollectRequest{Title: "测试", TargetPath: "/data/test"}
	created, _ := manager.CreateCollectRequest(req, "user1", "用户1")

	// 验证正确的令牌
	if !manager.ValidateAccessToken(created.ID, created.AccessToken) {
		t.Error("正确的令牌验证失败")
	}

	// 验证错误的令牌
	if manager.ValidateAccessToken(created.ID, "wrong-token") {
		t.Error("错误的令牌不应通过验证")
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultCollectConfig()
	config.TempPath = t.TempDir()
	config.StoragePath = t.TempDir()
	manager := NewCollectManager(&config)

	// 创建请求
	req1 := &CreateCollectRequest{Title: "请求1", TargetPath: t.TempDir()}
	req2 := &CreateCollectRequest{Title: "请求2", TargetPath: t.TempDir()}
	manager.CreateCollectRequest(req1, "user1", "用户1")
	manager.CreateCollectRequest(req2, "user1", "用户1")

	stats := manager.GetStats("user1")
	if stats.TotalRequests != 2 {
		t.Errorf("TotalRequests 期望 2, 实际 %d", stats.TotalRequests)
	}
	if stats.ActiveRequests != 2 {
		t.Errorf("ActiveRequests 期望 2, 实际 %d", stats.ActiveRequests)
	}
}

func TestCollectStatus(t *testing.T) {
	statuses := []CollectStatus{
		CollectStatusActive,
		CollectStatusPaused,
		CollectStatusExpired,
		CollectStatusClosed,
		CollectStatusFull,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("状态不应为空")
		}
	}
}

func TestSubmissionStatus(t *testing.T) {
	statuses := []SubmissionStatus{
		SubmissionStatusPending,
		SubmissionStatusScanning,
		SubmissionStatusClean,
		SubmissionStatusInfected,
		SubmissionStatusDuplicate,
		SubmissionStatusRejected,
		SubmissionStatusAccepted,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("状态不应为空")
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("ID 不应为空")
	}

	if id1 == id2 {
		t.Error("ID 应该唯一")
	}
}

func TestGenerateToken(t *testing.T) {
	token1 := generateToken()
	token2 := generateToken()

	if token1 == "" {
		t.Error("Token 不应为空")
	}

	if token1 == token2 {
		t.Error("Token 应该唯一")
	}
}
