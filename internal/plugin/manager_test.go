// Package plugin 提供插件管理器测试
package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("Expected manager, got nil")
	}
}

func TestNewManagerDefaultDirs(t *testing.T) {
	// 测试默认目录值（不实际创建目录）
	cfg := ManagerConfig{}
	
	// 验证默认值会正确设置
	expectedPluginDir := "/opt/nas/plugins"
	expectedConfigDir := "/etc/nas-os/plugins"
	expectedDataDir := "/var/lib/nas-os/plugins"
	
	// 由于需要 root 权限创建这些目录，我们只验证配置逻辑
	// 实际创建目录的功能在集成测试中验证
	if cfg.PluginDir == "" {
		// 默认值逻辑正确
		t.Logf("默认插件目录将设置为: %s", expectedPluginDir)
	}
	if cfg.ConfigDir == "" {
		t.Logf("默认配置目录将设置为: %s", expectedConfigDir)
	}
	if cfg.DataDir == "" {
		t.Logf("默认数据目录将设置为: %s", expectedDataDir)
	}
	
	// 尝试创建管理器（可能在 CI 环境中因权限失败）
	mgr, err := NewManager(cfg)
	if err != nil {
		// 如果是权限错误，跳过测试而不是失败
		if strings.Contains(err.Error(), "permission denied") {
			t.Skipf("跳过测试：需要 root 权限创建系统目录: %v", err)
		}
		t.Fatalf("NewManager with defaults failed: %v", err)
	}
	
	// 如果成功创建，验证目录值
	if mgr.pluginDir != expectedPluginDir {
		t.Errorf("Expected default pluginDir '%s', got %s", expectedPluginDir, mgr.pluginDir)
	}
	if mgr.configDir != expectedConfigDir {
		t.Errorf("Expected default configDir '%s', got %s", expectedConfigDir, mgr.configDir)
	}
	if mgr.dataDir != expectedDataDir {
		t.Errorf("Expected default dataDir '%s', got %s", expectedDataDir, mgr.dataDir)
	}
}

func TestManagerListEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	list := mgr.List()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d", len(list))
	}
}

func TestManagerGetNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent plugin")
	}
}

func TestManagerDiscoverEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	plugins, err := mgr.Discover()
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("Expected no plugins, got %d", len(plugins))
	}
}

func TestManagerRegisterHook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	called := false
	hook := func(ctx HookContext) error {
		called = true
		return nil
	}

	mgr.RegisterHook(HookAfterMount, hook)

	// 执行钩子
	err = mgr.ExecuteHooks(HookAfterMount, HookContext{Event: "afterMount"})
	if err != nil {
		t.Fatalf("ExecuteHooks failed: %v", err)
	}
	if !called {
		t.Error("Expected hook to be called")
	}
}

func TestManagerRegisterExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	ext := &Extension{
		PluginID: "com.test.plugin",
		PointID:  "file-manager.context-menu",
		Priority: 10,
	}

	err = mgr.RegisterExtension(ext)
	if err != nil {
		t.Fatalf("RegisterExtension failed: %v", err)
	}

	extensions := mgr.GetExtensions("file-manager.context-menu")
	if len(extensions) != 1 {
		t.Errorf("Expected 1 extension, got %d", len(extensions))
	}
}

func TestManagerGetExtensionsNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	extensions := mgr.GetExtensions("nonexistent-point")
	if extensions != nil {
		t.Errorf("Expected nil for nonexistent point, got %v", extensions)
	}
}

func TestManagerSaveStates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// 手动添加状态
	mgr.mu.Lock()
	mgr.states["test-plugin"] = &PluginState{
		ID:        "test-plugin",
		Enabled:   true,
		Installed: true,
	}
	mgr.mu.Unlock()

	// 保存状态
	err = mgr.saveStates()
	if err != nil {
		t.Fatalf("saveStates failed: %v", err)
	}

	// 检查文件是否创建
	if _, err := os.Stat(mgr.stateFile); os.IsNotExist(err) {
		t.Error("Expected state file to be created")
	}
}

func TestManagerLoadStates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	// 创建状态文件
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	stateContent := `{
		"test-plugin": {
			"id": "test-plugin",
			"enabled": true,
			"running": false,
			"installed": true,
			"version": "1.0.0"
		}
	}`
	stateFile := filepath.Join(configDir, "states.json")
	if err := os.WriteFile(stateFile, []byte(stateContent), 0644); err != nil {
		t.Fatal(err)
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// 检查状态是否加载
	state, exists := mgr.states["test-plugin"]
	if !exists {
		t.Fatal("Expected state to be loaded")
	}
	if !state.Enabled {
		t.Error("Expected plugin to be enabled")
	}
}

func TestManagerConfigureNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	err = mgr.Configure("nonexistent", map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("Expected error for nonexistent plugin")
	}
}

func TestManagerEnableNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	err = mgr.Enable("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent plugin")
	}
}

func TestManagerDisableNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	err = mgr.Disable("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent plugin")
	}
}

func TestManagerUninstallNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	err = mgr.Uninstall("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent plugin")
	}
}

func TestManagerUpdateNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Update("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent plugin")
	}
}

func TestIsDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if !isDir(tmpDir) {
		t.Error("Expected isDir to return true for directory")
	}

	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if isDir(file) {
		t.Error("Expected isDir to return false for file")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	if err := os.WriteFile(src, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test content" {
		t.Errorf("Expected 'test content', got %s", string(data))
	}
}

func TestCopyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(filepath.Join(dstDir, "file.txt")); os.IsNotExist(err) {
		t.Error("Expected file to be copied")
	}
}

func TestManagerCheckDependencies(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nas-os-plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := ManagerConfig{
		PluginDir: filepath.Join(tmpDir, "plugins"),
		ConfigDir: filepath.Join(tmpDir, "config"),
		DataDir:   filepath.Join(tmpDir, "data"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// 测试空依赖
	err = mgr.checkDependencies([]Dependency{})
	if err != nil {
		t.Errorf("Expected no error for empty dependencies, got %v", err)
	}

	// 测试可选依赖（缺失但不报错）
	err = mgr.checkDependencies([]Dependency{
		{ID: "optional-plugin", Optional: true},
	})
	if err != nil {
		t.Errorf("Expected no error for optional dependency, got %v", err)
	}

	// 测试必需依赖（缺失）
	err = mgr.checkDependencies([]Dependency{
		{ID: "required-plugin", Optional: false},
	})
	if err == nil {
		t.Error("Expected error for missing required dependency")
	}
}
