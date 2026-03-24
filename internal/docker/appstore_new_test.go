package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// 模板版本管理测试
// =============================================================================

func TestTemplateVersionManager_AddVersion(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "template-version-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建 AppStore 和模板版本管理器
	store, _ := createTestAppStore(tmpDir)
	tvm, err := NewTemplateVersionManager(store, tmpDir)
	if err != nil {
		t.Fatalf("创建模板版本管理器失败: %v", err)
	}

	// 添加版本
	version := &TemplateVersion{
		Version:      "1.0.0",
		ImageTag:     "latest",
		ReleaseNotes: "初始版本",
		PublishedAt:  time.Now(),
	}

	err = tvm.AddVersion("nextcloud", version)
	if err != nil {
		t.Fatalf("添加版本失败: %v", err)
	}

	// 验证版本已添加
	versions := tvm.GetVersions("nextcloud")
	if len(versions) != 1 {
		t.Errorf("期望 1 个版本，实际 %d 个", len(versions))
	}

	if versions[0].Version != "1.0.0" {
		t.Errorf("期望版本 1.0.0，实际 %s", versions[0].Version)
	}
}

func TestTemplateVersionManager_GetLatestVersion(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "template-version-test")
	defer os.RemoveAll(tmpDir)

	store, _ := createTestAppStore(tmpDir)
	tvm, _ := NewTemplateVersionManager(store, tmpDir)

	// 添加多个版本
	versions := []*TemplateVersion{
		{Version: "1.0.0", ImageTag: "1.0.0", PublishedAt: time.Now().Add(-2 * time.Hour)},
		{Version: "1.1.0", ImageTag: "1.1.0", PublishedAt: time.Now().Add(-time.Hour)},
		{Version: "1.2.0", ImageTag: "1.2.0", PublishedAt: time.Now()},
	}

	for _, v := range versions {
		_ = tvm.AddVersion("nextcloud", v)
	}

	latest := tvm.GetLatestVersion("nextcloud")
	if latest == nil {
		t.Fatal("期望获取最新版本，得到 nil")
	}

	if latest.Version != "1.2.0" {
		t.Errorf("期望最新版本 1.2.0，实际 %s", latest.Version)
	}
}

func TestTemplateVersionManager_DeprecateVersion(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "template-version-test")
	defer os.RemoveAll(tmpDir)

	store, _ := createTestAppStore(tmpDir)
	tvm, _ := NewTemplateVersionManager(store, tmpDir)

	_ = tvm.AddVersion("nextcloud", &TemplateVersion{Version: "1.0.0"})

	err := tvm.DeprecateVersion("nextcloud", "1.0.0")
	if err != nil {
		t.Fatalf("弃用版本失败: %v", err)
	}

	version := tvm.GetVersion("nextcloud", "1.0.0")
	if version == nil {
		t.Fatal("版本不存在")
	}

	if !version.Deprecated {
		t.Error("期望版本已弃用")
	}
}

// =============================================================================
// 备份管理测试
// =============================================================================

func TestBackupManager_BackupApp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := createTestAppStore(tmpDir)
	bm, err := NewBackupManager(store, tmpDir)
	if err != nil {
		t.Fatalf("创建备份管理器失败: %v", err)
	}

	// 创建一个模拟的已安装应用
	app := &InstalledApp{
		ID:          "test-app",
		Name:        "testapp",
		DisplayName: "Test App",
		TemplateID:  "test",
		Version:     "1.0.0",
		InstallTime: time.Now(),
		Ports:       map[int]int{80: 8080},
		Volumes:     map[string]string{"/data": filepath.Join(tmpDir, "test-data")},
		Environment: map[string]string{"TEST": "value"},
	}

	store.mu.Lock()
	store.installed["test-app"] = app
	store.mu.Unlock()

	// 创建数据目录
	dataDir := filepath.Join(tmpDir, "test-data")
	_ = os.MkdirAll(dataDir, 0750)
	_ = os.WriteFile(filepath.Join(dataDir, "test.txt"), []byte("test data"), 0640)

	// 创建备份
	opts := BackupOptions{
		IncludeConfig:  true,
		IncludeCompose: true,
		IncludeData:    false, // 跳过数据备份以简化测试
		Notes:          "测试备份",
	}

	info, err := bm.BackupApp("test-app", opts)
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}

	if info == nil {
		t.Fatal("期望返回备份信息，得到 nil")
	}

	if info.AppID != "test-app" {
		t.Errorf("期望 AppID test-app，实际 %s", info.AppID)
	}

	if info.Notes != "测试备份" {
		t.Errorf("期望备注 '测试备份'，实际 %s", info.Notes)
	}
}

func TestBackupManager_ListBackups(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "backup-test")
	defer os.RemoveAll(tmpDir)

	store, _ := createTestAppStore(tmpDir)
	bm, _ := NewBackupManager(store, tmpDir)

	// 列出空的备份列表
	backups, err := bm.ListBackups("")
	if err != nil {
		t.Fatalf("列出备份失败: %v", err)
	}

	if len(backups) != 0 {
		t.Errorf("期望 0 个备份，实际 %d 个", len(backups))
	}
}

func TestBackupManager_DeleteBackup(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "backup-test")
	defer os.RemoveAll(tmpDir)

	store, _ := createTestAppStore(tmpDir)
	bm, _ := NewBackupManager(store, tmpDir)

	// 删除不存在的备份
	err := bm.DeleteBackup("non-existent")
	if err != nil {
		t.Errorf("删除不存在的备份应该成功，实际返回错误: %v", err)
	}
}

// =============================================================================
// 健康检查测试
// =============================================================================

func TestHealthChecker_CheckHealth(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "health-test")
	defer os.RemoveAll(tmpDir)

	store, _ := createTestAppStore(tmpDir)
	mgr := &Manager{}
	hc := NewHealthChecker(store, mgr, DefaultHealthCheckConfig)

	// 检查不存在的应用
	_, err := hc.CheckHealth("non-existent")
	if err == nil {
		t.Error("期望返回错误，得到 nil")
	}
}

func TestHealthChecker_AggregateStatus(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "health-test")
	defer os.RemoveAll(tmpDir)

	store, _ := createTestAppStore(tmpDir)
	mgr := &Manager{}
	hc := NewHealthChecker(store, mgr, DefaultHealthCheckConfig)

	tests := []struct {
		name     string
		checks   []HealthCheck
		expected string
	}{
		{
			name:     "all healthy",
			checks:   []HealthCheck{{Status: "healthy"}, {Status: "healthy"}},
			expected: "healthy",
		},
		{
			name:     "one degraded",
			checks:   []HealthCheck{{Status: "healthy"}, {Status: "degraded"}},
			expected: "degraded",
		},
		{
			name:     "one unhealthy",
			checks:   []HealthCheck{{Status: "healthy"}, {Status: "unhealthy"}},
			expected: "unhealthy",
		},
		{
			name:     "mixed with unhealthy",
			checks:   []HealthCheck{{Status: "healthy"}, {Status: "degraded"}, {Status: "unhealthy"}},
			expected: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hc.aggregateStatus(tt.checks)
			if result != tt.expected {
				t.Errorf("期望状态 %s，实际 %s", tt.expected, result)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, 期望 %s", tt.bytes, result, tt.expected)
		}
	}
}

// =============================================================================
// 更新检测测试
// =============================================================================

func TestUpdateChecker_ParseImageName(t *testing.T) {
	tests := []struct {
		image       string
		expectedName string
		expectedTag string
	}{
		{"nginx", "nginx", "latest"},
		{"nginx:latest", "nginx", "latest"},
		{"nginx:1.21", "nginx", "1.21"},
		{"library/nginx:1.21", "library/nginx", "1.21"},
		{"gcr.io/project/image:v1", "gcr.io/project/image", "v1"},
	}

	for _, tt := range tests {
		name, tag := parseImageName(tt.image)
		if name != tt.expectedName || tag != tt.expectedTag {
			t.Errorf("parseImageName(%s) = (%s, %s), 期望 (%s, %s)",
				tt.image, name, tag, tt.expectedName, tt.expectedTag)
		}
	}
}

func TestUpdateChecker_ParseImageNamespace(t *testing.T) {
	tests := []struct {
		imageName          string
		expectedNamespace  string
		expectedName       string
	}{
		{"nginx", "library", "nginx"},
		{"library/nginx", "library", "nginx"},
		{"gcr.io/project/image", "gcr.io", "project/image"},
	}

	for _, tt := range tests {
		ns, name := parseImageNamespace(tt.imageName)
		if ns != tt.expectedNamespace || name != tt.expectedName {
			t.Errorf("parseImageNamespace(%s) = (%s, %s), 期望 (%s, %s)",
				tt.imageName, ns, name, tt.expectedNamespace, tt.expectedName)
		}
	}
}

func TestFindLatestStableTag(t *testing.T) {
	tests := []struct {
		name     string
		tags     []TagInfo
		expected string
	}{
		{
			name: "skip latest and pre-release",
			tags: []TagInfo{
				{Name: "latest"},
				{Name: "1.0.0-beta"},
				{Name: "1.0.0"},
			},
			expected: "1.0.0",
		},
		{
			name: "only latest available",
			tags: []TagInfo{
				{Name: "latest"},
			},
			expected: "",
		},
		{
			name: "skip rc tags",
			tags: []TagInfo{
				{Name: "latest"},
				{Name: "2.0.0-rc1"},
				{Name: "1.9.0"},
			},
			expected: "1.9.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findLatestStableTag(tt.tags)
			if result == nil {
				if tt.expected != "" {
					t.Errorf("期望标签 %s，得到 nil", tt.expected)
				}
				return
			}

			if result.Name != tt.expected {
				t.Errorf("期望标签 %s，实际 %s", tt.expected, result.Name)
			}
		})
	}
}

// =============================================================================
// 辅助函数
// =============================================================================

func createTestAppStore(dataDir string) (*AppStore, error) {
	// 创建必要的目录
	_ = os.MkdirAll(filepath.Join(dataDir, "app-templates"), 0750)
	_ = os.MkdirAll(filepath.Join(dataDir, "apps"), 0750)

	return &AppStore{
		templateDir: filepath.Join(dataDir, "app-templates"),
		installDir:  filepath.Join(dataDir, "apps"),
		dataFile:    filepath.Join(dataDir, "installed-apps.json"),
		templates:   make(map[string]*AppTemplate),
		installed:   make(map[string]*InstalledApp),
	}, nil
}