package directplay

import (
	"context"
	"testing"
	"time"
)

func TestDefaultDirectPlayConfig(t *testing.T) {
	config := DefaultDirectPlayConfig()

	if !config.Enabled {
		t.Error("默认配置应该启用直链播放")
	}
	if !config.BaiduPanEnabled {
		t.Error("默认配置应该启用百度网盘")
	}
	if !config.Pan123Enabled {
		t.Error("默认配置应该启用123云盘")
	}
	if !config.AliyunPanEnabled {
		t.Error("默认配置应该启用阿里云盘")
	}
	if config.CacheTTL != 30*time.Minute {
		t.Errorf("缓存TTL应该是30分钟，实际是%v", config.CacheTTL)
	}
	if config.MaxConcurrent != 10 {
		t.Errorf("最大并发应该是10，实际是%d", config.MaxConcurrent)
	}
}

func TestNewManager(t *testing.T) {
	config := DefaultDirectPlayConfig()
	manager := NewManager(config)

	if manager == nil {
		t.Fatal("创建管理器失败")
	}

	// 检查提供商是否初始化
	if len(manager.providers) != 3 {
		t.Errorf("应该初始化3个提供商，实际是%d", len(manager.providers))
	}

	// 检查缓存是否初始化
	if manager.cache == nil {
		t.Error("缓存应该被初始化")
	}
}

func TestManagerGetStatus(t *testing.T) {
	config := DefaultDirectPlayConfig()
	manager := NewManager(config)

	status := manager.GetStatus()

	if !status.Enabled {
		t.Error("状态应该显示已启用")
	}
	if len(status.Providers) != 3 {
		t.Errorf("应该有3个提供商，实际是%d", len(status.Providers))
	}
}

func TestLinkCache(t *testing.T) {
	cache := NewLinkCache(time.Hour, 100)

	link := &DirectLinkInfo{
		FileID:   "test123",
		FileName: "test.mp4",
		URL:      "https://example.com/test.mp4",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	// 测试设置
	cache.Set("test:key", link)

	// 测试获取
	got, ok := cache.Get("test:key")
	if !ok {
		t.Error("应该能获取到缓存")
	}
	if got.FileID != link.FileID {
		t.Errorf("FileID不匹配，期望%s，实际%s", link.FileID, got.FileID)
	}

	// 测试不存在的键
	_, ok = cache.Get("not:exist")
	if ok {
		t.Error("不存在的键应该返回false")
	}

	// 测试删除
	cache.Delete("test:key")
	_, ok = cache.Get("test:key")
	if ok {
		t.Error("删除后应该获取不到")
	}

	// 测试清空
	cache.Set("key1", link)
	cache.Set("key2", link)
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("清空后缓存应该为空，实际大小是%d", cache.Size())
	}
}

func TestLinkCacheExpiry(t *testing.T) {
	cache := NewLinkCache(time.Hour, 100)

	// 已过期的链接
	expiredLink := &DirectLinkInfo{
		FileID:    "expired",
		ExpiresAt: time.Now().Add(-time.Hour), // 已过期
	}
	cache.Set("expired:key", expiredLink)

	_, ok := cache.Get("expired:key")
	if ok {
		t.Error("过期的链接不应该被获取到")
	}
}

func TestProviderType(t *testing.T) {
	providers := []ProviderType{
		ProviderBaiduPan,
		Provider123Pan,
		ProviderAliyunPan,
	}

	for _, p := range providers {
		if p == "" {
			t.Error("提供商类型不应为空")
		}
	}
}

func TestBaiduPanProviderGetType(t *testing.T) {
	provider := NewBaiduPanProvider()
	if provider.GetType() != ProviderBaiduPan {
		t.Errorf("期望类型是%s，实际是%s", ProviderBaiduPan, provider.GetType())
	}
}

func Test123PanProviderGetType(t *testing.T) {
	provider := New123PanProvider()
	if provider.GetType() != Provider123Pan {
		t.Errorf("期望类型是%s，实际是%s", Provider123Pan, provider.GetType())
	}
}

func TestAliyunPanProviderGetType(t *testing.T) {
	provider := NewAliyunPanProvider()
	if provider.GetType() != ProviderAliyunPan {
		t.Errorf("期望类型是%s，实际是%s", ProviderAliyunPan, provider.GetType())
	}
}

func TestManagerProviderDisabled(t *testing.T) {
	config := &DirectPlayConfig{
		Enabled:         true,
		BaiduPanEnabled: false,
		Pan123Enabled:   false,
		AliyunPanEnabled: false,
	}
	manager := NewManager(config)

	// 所有网盘都禁用，应该没有提供商
	if len(manager.providers) != 0 {
		t.Errorf("所有网盘禁用时，提供商数量应该是0，实际是%d", len(manager.providers))
	}
}

func TestManagerDisabled(t *testing.T) {
	config := &DirectPlayConfig{
		Enabled:         false,
		BaiduPanEnabled: true,
	}
	manager := NewManager(config)

	req := &DirectPlayRequest{
		Provider: ProviderBaiduPan,
		FileID:   "test",
	}

	_, err := manager.GetDirectLink(context.Background(), req)
	if err == nil {
		t.Error("禁用时获取直链应该返回错误")
	}
}

func TestGetMimeType(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{"mp4", "video/mp4"},
		{"mkv", "video/x-matroska"},
		{"avi", "video/x-msvideo"},
		{"mp3", "audio/mpeg"},
		{"jpg", "image/jpeg"},
		{"pdf", "application/pdf"},
		{"unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		got := getMimeType(tt.ext)
		if got != tt.expected {
			t.Errorf("扩展名%s的MIME类型应该是%s，实际是%s", tt.ext, tt.expected, got)
		}
	}
}

func TestStreamSession(t *testing.T) {
	config := DefaultDirectPlayConfig()
	manager := NewManager(config)

	// 模拟创建会话（不实际调用API）
	session := &StreamSession{
		ID:         "test_session",
		Provider:   ProviderBaiduPan,
		FileID:     "test_file",
		FileName:   "test.mp4",
		StartTime:  time.Now(),
		LastAccess: time.Now(),
		Viewers:    1,
	}

	manager.mu.Lock()
	manager.sessions[session.ID] = session
	manager.mu.Unlock()

	// 测试获取会话
	got, err := manager.GetStreamSession(session.ID)
	if err != nil {
		t.Errorf("获取会话失败: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("会话ID不匹配")
	}

	// 测试关闭会话
	err = manager.CloseStreamSession(session.ID)
	if err != nil {
		t.Errorf("关闭会话失败: %v", err)
	}

	// 确认会话已删除
	_, err = manager.GetStreamSession(session.ID)
	if err == nil {
		t.Error("会话应该已被删除")
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	config := DefaultDirectPlayConfig()
	manager := NewManager(config)

	// 添加一个过期会话
	oldSession := &StreamSession{
		ID:         "old_session",
		LastAccess: time.Now().Add(-2 * time.Hour),
	}
	manager.mu.Lock()
	manager.sessions[oldSession.ID] = oldSession
	manager.mu.Unlock()

	// 添加一个活跃会话
	activeSession := &StreamSession{
		ID:         "active_session",
		LastAccess: time.Now(),
	}
	manager.mu.Lock()
	manager.sessions[activeSession.ID] = activeSession
	manager.mu.Unlock()

	// 清理1小时未访问的会话
	count := manager.CleanupExpiredSessions(time.Hour)

	if count != 1 {
		t.Errorf("应该清理1个会话，实际清理了%d个", count)
	}

	// 确认活跃会话还在
	_, err := manager.GetStreamSession(activeSession.ID)
	if err != nil {
		t.Error("活跃会话不应该被清理")
	}
}