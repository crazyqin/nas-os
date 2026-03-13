package webdav

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config == nil {
		t.Fatal("DefaultConfig 返回 nil")
	}
	if !config.Enabled {
		t.Error("默认配置应该启用 WebDAV")
	}
	if config.Port != 8081 {
		t.Errorf("期望端口 8081，实际为 %d", config.Port)
	}
	if config.RootPath != "/data" {
		t.Errorf("期望根路径 /data，实际为 %s", config.RootPath)
	}
	if config.AllowGuest {
		t.Error("默认配置不应该允许访客访问")
	}
}

func TestNewServer(t *testing.T) {
	// 测试使用默认配置
	srv, err := NewServer(nil)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}
	if srv == nil {
		t.Fatal("服务器不应为 nil")
	}
	if srv.lockManager == nil {
		t.Error("lockManager 应该被初始化")
	}
	if srv.quotaProvider == nil {
		t.Error("quotaProvider 应该被初始化")
	}

	// 测试使用自定义配置
	config := &Config{
		Enabled:       true,
		Port:          9090,
		RootPath:      "/tmp/webdav-test",
		AllowGuest:    true,
		MaxUploadSize: 1024 * 1024,
	}
	srv, err = NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}
	if srv.config.Port != 9090 {
		t.Errorf("期望端口 9090，实际为 %d", srv.config.Port)
	}
	if srv.config.MaxUploadSize != 1024*1024 {
		t.Errorf("期望 MaxUploadSize %d，实际为 %d", 1024*1024, srv.config.MaxUploadSize)
	}
}

func TestServerStartStop(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "webdav-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &Config{
		Enabled:    true,
		Port:       18081,
		RootPath:   tmpDir,
		AllowGuest: true,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 启动服务器
	if err := srv.Start(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 停止服务器
	if err := srv.Stop(); err != nil {
		t.Fatalf("停止服务器失败: %v", err)
	}
}

func TestServerDisabled(t *testing.T) {
	config := &Config{
		Enabled:  false,
		Port:     18082,
		RootPath: "/tmp/webdav-disabled",
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 禁用的服务器启动应该返回 nil
	if err := srv.Start(); err != nil {
		t.Errorf("禁用的服务器启动应该返回 nil，实际返回: %v", err)
	}
}

func TestServerConfig(t *testing.T) {
	config := &Config{
		Enabled:       true,
		Port:          8081,
		RootPath:      "/data",
		AllowGuest:    false,
		MaxUploadSize: 0,
	}

	srv, err := NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 测试获取配置
	gotConfig := srv.GetConfig()
	if gotConfig.Port != config.Port {
		t.Errorf("期望端口 %d，实际为 %d", config.Port, gotConfig.Port)
	}

	// 测试更新配置
	newConfig := &Config{
		Enabled:       true,
		Port:          9090,
		RootPath:      "/newdata",
		AllowGuest:    true,
		MaxUploadSize: 10 * 1024 * 1024,
	}
	if err := srv.UpdateConfig(newConfig); err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	gotConfig = srv.GetConfig()
	if gotConfig.Port != 9090 {
		t.Errorf("期望端口 9090，实际为 %d", gotConfig.Port)
	}
	if gotConfig.MaxUploadSize != 10*1024*1024 {
		t.Errorf("期望 MaxUploadSize %d，实际为 %d", 10*1024*1024, gotConfig.MaxUploadSize)
	}
}

func TestSetAuthFunc(t *testing.T) {
	srv, err := NewServer(nil)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	called := false
	srv.SetAuthFunc(func(username, password string) bool {
		called = true
		return username == "admin" && password == "secret"
	})

	if srv.authFunc == nil {
		t.Error("认证函数应该被设置")
	}

	// 测试认证函数
	result := srv.authFunc("admin", "secret")
	if !result {
		t.Error("认证应该成功")
	}
	if !called {
		t.Error("认证函数应该被调用")
	}
}

func TestSetGetUserHome(t *testing.T) {
	srv, err := NewServer(nil)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	srv.SetGetUserHome(func(username string) string {
		return "/home/" + username
	})

	if srv.getUserHome == nil {
		t.Error("getUserHome 应该被设置")
	}

	result := srv.getUserHome("testuser")
	if result != "/home/testuser" {
		t.Errorf("期望 /home/testuser，实际为 %s", result)
	}
}

func TestSetQuotaProvider(t *testing.T) {
	srv, err := NewServer(nil)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	provider := &MockQuotaProvider{Available: 1024 * 1024}
	srv.SetQuotaProvider(provider)

	if srv.quotaProvider == nil {
		t.Error("quotaProvider 应该被设置")
	}
}

func TestAuthenticate(t *testing.T) {
	srv, err := NewServer(nil)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 没有认证函数时应该拒绝
	if srv.authenticate("user", "pass") {
		t.Error("没有认证函数时应该拒绝访问")
	}

	// 设置认证函数
	srv.SetAuthFunc(func(username, password string) bool {
		return username == "admin"
	})

	if !srv.authenticate("admin", "any") {
		t.Error("admin 用户应该可以认证")
	}

	if srv.authenticate("user", "any") {
		t.Error("非 admin 用户不应该可以认证")
	}
}

func TestGetStatus(t *testing.T) {
	srv, err := NewServer(nil)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	status := srv.GetStatus()
	if status == nil {
		t.Fatal("状态不应为 nil")
	}

	if _, ok := status["enabled"]; !ok {
		t.Error("状态应该包含 enabled")
	}
	if _, ok := status["port"]; !ok {
		t.Error("状态应该包含 port")
	}
	if _, ok := status["running"]; !ok {
		t.Error("状态应该包含 running")
	}
	if _, ok := status["lock_count"]; !ok {
		t.Error("状态应该包含 lock_count")
	}
}

// MockQuotaProvider 模拟配额提供者
type MockQuotaProvider struct {
	Available int64
	Used      int64
}

func (m *MockQuotaProvider) CheckQuota(username string) (int64, error) {
	return m.Available, nil
}

func (m *MockQuotaProvider) ConsumeQuota(username string, size int64) error {
	m.Used += size
	m.Available -= size
	return nil
}

func (m *MockQuotaProvider) ReleaseQuota(username string, size int64) error {
	m.Used -= size
	m.Available += size
	return nil
}

func (m *MockQuotaProvider) GetUsage(username string) (used, total int64, err error) {
	return m.Used, m.Used + m.Available, nil
}
