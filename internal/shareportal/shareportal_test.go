// Package shareportal 测试
package shareportal

import (
	"context"
	"testing"
	"time"
)

func TestManager_CreateShare(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建分享链接
	link := ShareLink{
		Name:          "测试分享",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		CreatorName:   "测试用户",
		AllowPreview:  true,
		AllowDownload: true,
		AllowUpload:   false,
	}

	result, err := mgr.CreateShare(ctx, link)
	if err != nil {
		t.Fatalf("创建分享失败: %v", err)
	}

	if result.ID == "" {
		t.Error("分享ID不应为空")
	}
	if result.ShortURL == "" {
		t.Error("短链不应为空")
	}
	if !result.IsActive {
		t.Error("分享应该是激活状态")
	}
	if result.Name != "测试分享" {
		t.Errorf("名称不匹配，期望: 测试分享, 实际: %s", result.Name)
	}
}

func TestManager_GetShare(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 先创建
	link := ShareLink{
		Name:     "测试分享",
		FilePath: "/test/file.txt",
		CreatorID: "user1",
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 获取
	result, err := mgr.GetShare(ctx, created.ID)
	if err != nil {
		t.Fatalf("获取分享失败: %v", err)
	}
	if result.ID != created.ID {
		t.Errorf("ID不匹配，期望: %s, 实际: %s", created.ID, result.ID)
	}
}

func TestManager_GetShareByShortURL(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建分享
	link := ShareLink{
		Name:     "短链测试",
		FilePath: "/test/file.txt",
		CreatorID: "user1",
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 通过短链获取
	result, err := mgr.GetShareByShortURL(ctx, created.ShortURL)
	if err != nil {
		t.Fatalf("通过短链获取失败: %v", err)
	}
	if result.ID != created.ID {
		t.Errorf("ID不匹配，期望: %s, 实际: %s", created.ID, result.ID)
	}
}

func TestManager_UpdateShare(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建
	link := ShareLink{
		Name:          "原始名称",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		AllowPreview:  true,
		AllowDownload: true,
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 更新
	updates := ShareLink{
		Name:           "新名称",
		AllowUpload:    true,
		AllowPreview:   true,
		AllowDownload:  true,
	}
	result, err := mgr.UpdateShare(ctx, created.ID, updates)
	if err != nil {
		t.Fatalf("更新分享失败: %v", err)
	}
	if result.Name != "新名称" {
		t.Errorf("名称不匹配，期望: 新名称, 实际: %s", result.Name)
	}
	if !result.AllowUpload {
		t.Error("AllowUpload应该是true")
	}
}

func TestManager_DeleteShare(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建
	link := ShareLink{
		Name:     "待删除",
		FilePath: "/test/file.txt",
		CreatorID: "user1",
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 删除
	err := mgr.DeleteShare(ctx, created.ID)
	if err != nil {
		t.Fatalf("删除分享失败: %v", err)
	}

	// 验证已删除
	_, err = mgr.GetShare(ctx, created.ID)
	if err != ErrShareNotFound {
		t.Errorf("期望 ErrShareNotFound, 实际: %v", err)
	}
}

func TestManager_ValidateAccess(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建无密码分享
	link := ShareLink{
		Name:          "无密码",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		AllowDownload: true,
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 验证无密码访问
	valid, err := mgr.ValidateAccess(ctx, created.ID, "")
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}
	if !valid {
		t.Error("应该验证通过")
	}

	// 创建有密码分享
	pwdLink := ShareLink{
		Name:          "有密码",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		Password:      "test123",
		AllowDownload: true,
	}
	pwdCreated, _ := mgr.CreateShare(ctx, pwdLink)

	// 无密码访问 - 应该失败
	_, err = mgr.ValidateAccess(ctx, pwdCreated.ID, "")
	if err != ErrPasswordRequired {
		t.Errorf("期望 ErrPasswordRequired, 实际: %v", err)
	}

	// 错误密码 - 应该失败
	_, err = mgr.ValidateAccess(ctx, pwdCreated.ID, "wrong")
	if err != ErrPasswordWrong {
		t.Errorf("期望 ErrPasswordWrong, 实际: %v", err)
	}

	// 正确密码
	valid, err = mgr.ValidateAccess(ctx, pwdCreated.ID, "test123")
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}
	if !valid {
		t.Error("应该验证通过")
	}
}

func TestManager_ValidateAccess_Expired(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建已过期的分享
	pastTime := time.Now().Add(-1 * time.Hour)
	link := ShareLink{
		Name:          "已过期",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		ExpiresAt:     &pastTime,
		AllowDownload: true,
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 验证 - 应该过期
	_, err := mgr.ValidateAccess(ctx, created.ID, "")
	if err != ErrShareExpired {
		t.Errorf("期望 ErrShareExpired, 实际: %v", err)
	}
}

func TestManager_ValidateAccess_Inactive(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建分享（默认激活）
	link := ShareLink{
		Name:          "待停用",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		AllowDownload: true,
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 停用分享
	mgr.UpdateShare(ctx, created.ID, ShareLink{IsActive: false})

	// 验证 - 应该停用
	_, err := mgr.ValidateAccess(ctx, created.ID, "")
	if err != ErrShareInactive {
		t.Errorf("期望 ErrShareInactive, 实际: %v", err)
	}
}

func TestManager_RecordAccess(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建分享
	link := ShareLink{
		Name:          "统计测试",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		AllowDownload: true,
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 记录访问
	access := ShareAccess{
		ShareLinkID: created.ID,
		VisitorIP:   "192.168.1.1",
		VisitorUA:   "test-agent",
		Action:      ActionView,
	}
	err := mgr.RecordAccess(ctx, access)
	if err != nil {
		t.Fatalf("记录访问失败: %v", err)
	}

	// 检查计数
	result, _ := mgr.GetShare(ctx, created.ID)
	if result.ViewCount != 1 {
		t.Errorf("查看计数不匹配，期望: 1, 实际: %d", result.ViewCount)
	}

	// 记录下载
	downloadAccess := ShareAccess{
		ShareLinkID:      created.ID,
		VisitorIP:        "192.168.1.1",
		VisitorUA:        "test-agent",
		Action:           ActionDownload,
		FileName:         "file.txt",
		BytesTransferred: 1024,
	}
	mgr.RecordAccess(ctx, downloadAccess)

	result, _ = mgr.GetShare(ctx, created.ID)
	if result.DownloadCount != 1 {
		t.Errorf("下载计数不匹配，期望: 1, 实际: %d", result.DownloadCount)
	}
}

func TestManager_GetStats(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建分享
	link := ShareLink{
		Name:          "统计测试",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		AllowDownload: true,
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 记录一些访问
	for i := 0; i < 5; i++ {
		mgr.RecordAccess(ctx, ShareAccess{
			ShareLinkID: created.ID,
			VisitorIP:   "192.168.1.1",
			Action:      ActionView,
		})
	}
	for i := 0; i < 3; i++ {
		mgr.RecordAccess(ctx, ShareAccess{
			ShareLinkID: created.ID,
			VisitorIP:   "192.168.1.1",
			Action:      ActionDownload,
			FileName:    "file.txt",
		})
	}

	// 获取统计
	stats, err := mgr.GetStats(ctx, created.ID)
	if err != nil {
		t.Fatalf("获取统计失败: %v", err)
	}

	if stats.TotalViews != 5 {
		t.Errorf("总查看数不匹配，期望: 5, 实际: %d", stats.TotalViews)
	}
	if stats.TotalDownloads != 3 {
		t.Errorf("总下载数不匹配，期望: 3, 实际: %d", stats.TotalDownloads)
	}
	if stats.UniqueVisitors != 1 {
		t.Errorf("唯一访客数不匹配，期望: 1, 实际: %d", stats.UniqueVisitors)
	}
}

func TestManager_Branding(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 设置品牌
	branding := ShareBranding{
		Name:           "测试品牌",
		LogoURL:        "https://example.com/logo.png",
		PrimaryColor:   "#007bff",
		SecondaryColor: "#6c757d",
		FooterText:     "© 2026 测试",
		IsDefault:      true,
	}
	result, err := mgr.SetBranding(ctx, branding)
	if err != nil {
		t.Fatalf("设置品牌失败: %v", err)
	}
	if result.ID == "" {
		t.Error("品牌ID不应为空")
	}

	// 获取品牌
	got, err := mgr.GetBranding(ctx, result.ID)
	if err != nil {
		t.Fatalf("获取品牌失败: %v", err)
	}
	if got.Name != "测试品牌" {
		t.Errorf("品牌名称不匹配，期望: 测试品牌, 实际: %s", got.Name)
	}
	if !got.IsDefault {
		t.Error("应该是默认品牌")
	}
}

func TestManager_CreatePortal(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建一些分享
	link1, _ := mgr.CreateShare(ctx, ShareLink{
		Name:     "分享1",
		FilePath: "/test/file1.txt",
		CreatorID: "user1",
	})
	link2, _ := mgr.CreateShare(ctx, ShareLink{
		Name:     "分享2",
		FilePath: "/test/file2.txt",
		CreatorID: "user1",
	})

	// 创建门户
	portal := SharePortal{
		Name:        "测试门户",
		Description: "包含多个分享的门户",
		ShareIDs:    []string{link1.ID, link2.ID},
		IsPublic:    true,
	}
	result, err := mgr.CreatePortal(ctx, portal)
	if err != nil {
		t.Fatalf("创建门户失败: %v", err)
	}
	if result.ID == "" {
		t.Error("门户ID不应为空")
	}
	if len(result.ShareIDs) != 2 {
		t.Errorf("分享数量不匹配，期望: 2, 实际: %d", len(result.ShareIDs))
	}
}

func TestManager_NotFound(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 获取不存在的分享
	_, err := mgr.GetShare(ctx, "nonexistent")
	if err != ErrShareNotFound {
		t.Errorf("期望 ErrShareNotFound, 实际: %v", err)
	}

	// 更新不存在的分享
	_, err = mgr.UpdateShare(ctx, "nonexistent", ShareLink{})
	if err != ErrShareNotFound {
		t.Errorf("期望 ErrShareNotFound, 实际: %v", err)
	}

	// 删除不存在的分享
	err = mgr.DeleteShare(ctx, "nonexistent")
	if err != ErrShareNotFound {
		t.Errorf("期望 ErrShareNotFound, 实际: %v", err)
	}

	// 获取不存在的品牌
	_, err = mgr.GetBranding(ctx, "nonexistent")
	if err != ErrBrandingNotFound {
		t.Errorf("期望 ErrBrandingNotFound, 实际: %v", err)
	}
}

func TestManager_MaxDownloads(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	// 创建限制下载次数的分享
	link := ShareLink{
		Name:          "限制下载",
		FilePath:      "/test/file.txt",
		CreatorID:     "user1",
		MaxDownloads:  2,
		AllowDownload: true,
	}
	created, _ := mgr.CreateShare(ctx, link)

	// 下载 2 次
	for i := 0; i < 2; i++ {
		mgr.RecordAccess(ctx, ShareAccess{
			ShareLinkID: created.ID,
			VisitorIP:   "192.168.1.1",
			Action:      ActionDownload,
		})
	}

	// 第 3 次应该失败
	_, err := mgr.ValidateAccess(ctx, created.ID, "")
	if err != ErrMaxDownloadsExceeded {
		t.Errorf("期望 ErrMaxDownloadsExceeded, 实际: %v", err)
	}
}

func TestManager_GenerateShortURL(t *testing.T) {
	mgr := NewManager("/tmp/test-shareportal")
	ctx := context.Background()

	url1 := mgr.GenerateShortURL(ctx)
	url2 := mgr.GenerateShortURL(ctx)

	if url1 == "" || url2 == "" {
		t.Error("短链不应为空")
	}
	// 注意：极小概率下可能相同，但这不是测试的重点
}
