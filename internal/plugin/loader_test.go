// Package plugin 提供插件加载器测试
package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader("/tmp/plugins")
	if loader == nil {
		t.Fatal("Expected loader, got nil")
	}
	if loader.pluginDir != "/tmp/plugins" {
		t.Errorf("Expected pluginDir '/tmp/plugins', got %s", loader.pluginDir)
	}
}

func TestLoaderDiscoverEmpty(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	loader := NewLoader(tmpDir)
	plugins, err := loader.Discover()
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("Expected no plugins in empty dir, got %d", len(plugins))
	}
}

func TestLoaderDiscoverWithManifest(t *testing.T) {
	// 创建临时目录和 manifest
	tmpDir, err := os.MkdirTemp("", "plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建插件目录
	pluginDir := filepath.Join(tmpDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 创建 manifest.json
	manifest := `{
		"id": "com.nas-os.test",
		"name": "Test Plugin",
		"version": "1.0.0",
		"author": "Test",
		"description": "A test plugin",
		"category": "other",
		"entrypoint": "New",
		"mainFile": "plugin.so"
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	plugins, err := loader.Discover()
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(plugins))
		return
	}
	if plugins[0].ID != "com.nas-os.test" {
		t.Errorf("Expected ID 'com.nas-os.test', got %s", plugins[0].ID)
	}
}

func TestLoaderLoadManifest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建有效的 manifest
	manifest := `{
		"id": "com.test.plugin",
		"name": "Test",
		"version": "1.0.0",
		"author": "Test Author",
		"description": "Test Description",
		"category": "storage"
	}`
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	info, err := loader.loadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loadManifest failed: %v", err)
	}
	if info.ID != "com.test.plugin" {
		t.Errorf("Expected ID 'com.test.plugin', got %s", info.ID)
	}
	if info.Name != "Test" {
		t.Errorf("Expected Name 'Test', got %s", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", info.Version)
	}
}

func TestLoaderLoadManifestInvalid(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建无效的 manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir)
	_, err = loader.loadManifest(manifestPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoaderLoadManifestNotFound(t *testing.T) {
	loader := NewLoader("/nonexistent")
	_, err := loader.loadManifest("/nonexistent/manifest.json")
	if err == nil {
		t.Error("Expected error for missing manifest")
	}
}

func TestLoaderGetInstance(t *testing.T) {
	loader := NewLoader("/tmp/plugins")

	// 不存在的实例
	_, exists := loader.GetInstance("nonexistent")
	if exists {
		t.Error("Expected nonexistent instance to not exist")
	}
}

func TestLoaderListInstancesEmpty(t *testing.T) {
	loader := NewLoader("/tmp/plugins")
	instances := loader.ListInstances()
	if len(instances) != 0 {
		t.Errorf("Expected empty list, got %d", len(instances))
	}
}

func TestPluginInfoFields(t *testing.T) {
	info := PluginInfo{
		ID:          "com.test.plugin",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Author:      "Test Author",
		Description: "Test Description",
		Category:    CategoryStorage,
		Tags:        []string{"test", "storage"},
		Entrypoint:  "New",
		MainFile:    "plugin.so",
	}

	if info.ID != "com.test.plugin" {
		t.Errorf("Expected ID 'com.test.plugin', got %s", info.ID)
	}
	if info.Category != CategoryStorage {
		t.Errorf("Expected Category '%s', got %s", CategoryStorage, info.Category)
	}
	if len(info.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(info.Tags))
	}
}

func TestPluginCategories(t *testing.T) {
	categories := []Category{
		CategoryStorage,
		CategoryFileManager,
		CategoryNetwork,
		CategorySystem,
		CategorySecurity,
		CategoryMedia,
		CategoryBackup,
		CategoryTheme,
		CategoryIntegration,
		CategoryDeveloper,
		CategoryProductivity,
		CategoryOther,
	}

	for _, cat := range categories {
		if cat == "" {
			t.Error("Category should not be empty")
		}
	}
}

func TestDependencyFields(t *testing.T) {
	dep := Dependency{
		ID:       "com.test.dep",
		Version:  ">=1.0.0",
		Optional: false,
	}

	if dep.ID != "com.test.dep" {
		t.Errorf("Expected ID 'com.test.dep', got %s", dep.ID)
	}
	if dep.Optional {
		t.Error("Expected Optional to be false")
	}
}

func TestPermissionFields(t *testing.T) {
	perm := Permission{
		Name:        "read-files",
		Description: "Read file access",
	}

	if perm.Name != "read-files" {
		t.Errorf("Expected Name 'read-files', got %s", perm.Name)
	}
}

func TestConfigSchema(t *testing.T) {
	schema := &ConfigSchema{
		Properties: map[string]Property{
			"enabled": {
				Type:    "boolean",
				Default: true,
			},
			"port": {
				Type:    "number",
				Default: 8080,
			},
		},
		Required: []string{"enabled"},
	}

	if len(schema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(schema.Properties))
	}
	if len(schema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(schema.Required))
	}
}

func TestPropertyFields(t *testing.T) {
	minVal := 0.0
	maxVal := 100.0
	minLen := 1
	maxLen := 100

	prop := Property{
		Type:        "string",
		Title:       "Name",
		Description: "The name field",
		Default:     "default",
		Enum:        []string{"a", "b", "c"},
		Minimum:     &minVal,
		Maximum:     &maxVal,
		MinLength:   &minLen,
		MaxLength:   &maxLen,
	}

	if prop.Type != "string" {
		t.Errorf("Expected Type 'string', got %s", prop.Type)
	}
	if len(prop.Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(prop.Enum))
	}
}

func TestPluginState(t *testing.T) {
	state := PluginState{
		ID:        "com.test.plugin",
		Enabled:   true,
		Running:   true,
		Installed: true,
		Version:   "1.0.0",
		Error:     "",
	}

	if !state.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if !state.Running {
		t.Error("Expected Running to be true")
	}
}

func TestPluginInstance(t *testing.T) {
	info := PluginInfo{
		ID:      "com.test.plugin",
		Name:    "Test",
		Version: "1.0.0",
	}

	instance := &PluginInstance{
		Info:    info,
		Path:    "/plugins/test.so",
		Enabled: true,
		Running: false,
	}

	if instance.Info.ID != "com.test.plugin" {
		t.Errorf("Expected ID 'com.test.plugin', got %s", instance.Info.ID)
	}
	if instance.Path != "/plugins/test.so" {
		t.Errorf("Expected Path '/plugins/test.so', got %s", instance.Path)
	}
}

func TestExtensionPoint(t *testing.T) {
	point := &ExtensionPoint{
		ID:          "file-manager.context-menu",
		Name:        "Context Menu",
		Description: "File manager context menu extension",
		Extensions:  []*Extension{},
	}

	if point.ID != "file-manager.context-menu" {
		t.Errorf("Expected ID 'file-manager.context-menu', got %s", point.ID)
	}
}

func TestExtension(t *testing.T) {
	ext := &Extension{
		PluginID: "com.test.plugin",
		PointID:  "file-manager.context-menu",
		Priority: 10,
	}

	if ext.PluginID != "com.test.plugin" {
		t.Errorf("Expected PluginID 'com.test.plugin', got %s", ext.PluginID)
	}
}

func TestHookContext(t *testing.T) {
	ctx := HookContext{
		Event:     "afterMount",
		Data:      map[string]interface{}{"key": "value"},
		PluginID:  "com.test.plugin",
		Timestamp: timeNow(),
	}

	if ctx.Event != "afterMount" {
		t.Errorf("Expected Event 'afterMount', got %s", ctx.Event)
	}
}

func TestHookTypes(t *testing.T) {
	hooks := []HookType{
		HookBeforeMount,
		HookAfterMount,
		HookBeforeUnmount,
		HookAfterUnmount,
		HookBeforeCreate,
		HookAfterCreate,
		HookBeforeDelete,
		HookAfterDelete,
		HookBeforeStart,
		HookAfterStart,
		HookBeforeStop,
		HookAfterStop,
	}

	for _, hook := range hooks {
		if hook == "" {
			t.Error("Hook type should not be empty")
		}
	}
}
