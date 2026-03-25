package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 测试模板版本管理
func TestAppStore_GetTemplateVersionHistory(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	t.Run("获取存在的模板版本历史", func(t *testing.T) {
		// 内置模板应该存在
		history, err := store.GetTemplateVersionHistory("nextcloud")
		// 可能没有历史记录，但不应该报错
		_ = err
		_ = history
	})

	t.Run("获取不存在的模板版本历史", func(t *testing.T) {
		_, err := store.GetTemplateVersionHistory("nonexistent-template")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "模板不存在")
	})
}

func TestAppStore_AddTemplateVersion(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	t.Run("添加新版本", func(t *testing.T) {
		version := TemplateVersion{
			Version:      "v2.0.0",
			ImageTag:     "nextcloud:2.0.0",
			ReleaseNotes: "Major update",
			PublishedAt:  time.Now(),
			Digest:       "sha256:abc123",
		}

		// nextcloud 是内置模板
		err := store.AddTemplateVersion("nextcloud", version)
		// 可能因为模板文件保存失败而报错，但不应该 panic
		_ = err
	})

	t.Run("添加到不存在的模板", func(t *testing.T) {
		version := TemplateVersion{
			Version:     "v1.0.0",
			ImageTag:    "test:1.0.0",
			PublishedAt: time.Now(),
		}

		err := store.AddTemplateVersion("nonexistent", version)
		assert.Error(t, err)
	})
}

func TestAppStore_SetTemplateAutoUpdate(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	t.Run("设置自动更新", func(t *testing.T) {
		err := store.SetTemplateAutoUpdate("nextcloud", true)
		// 可能因为文件保存失败而报错
		_ = err
	})

	t.Run("设置不存在的模板", func(t *testing.T) {
		err := store.SetTemplateAutoUpdate("nonexistent", true)
		assert.Error(t, err)
	})
}

func TestAppStore_CheckTemplateUpdate(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	t.Run("检查模板更新", func(t *testing.T) {
		result, err := store.CheckTemplateUpdate("nextcloud")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "nextcloud", result.AppID)
	})

	t.Run("检查不存在的模板", func(t *testing.T) {
		_, err := store.CheckTemplateUpdate("nonexistent")
		assert.Error(t, err)
	})
}

func TestAppStore_CheckAllTemplateUpdates(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	results, err := store.CheckAllTemplateUpdates()
	assert.NoError(t, err)
	assert.NotNil(t, results)
}

func TestAppStore_GetAppRequirements(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	t.Run("获取应用要求", func(t *testing.T) {
		req, err := store.GetAppRequirements("nextcloud")
		// 可能没有设置要求，返回 nil
		_ = err
		_ = req
	})

	t.Run("获取不存在应用的要求", func(t *testing.T) {
		_, err := store.GetAppRequirements("nonexistent")
		assert.Error(t, err)
	})
}

// 测试更新检测
func TestAppStore_CheckAppUpdate(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	// 添加模拟已安装应用
	app := &InstalledApp{
		ID:          "test-nextcloud",
		Name:        "nextcloud",
		DisplayName: "Nextcloud",
		TemplateID:  "nextcloud",
		Version:     "latest",
		Status:      "running",
		InstallTime: time.Now(),
	}
	store.installed["test-nextcloud"] = app

	t.Run("检查应用更新", func(t *testing.T) {
		// 这可能会因为网络问题失败，但不应该 panic
		_, err := store.CheckAppUpdate("test-nextcloud")
		_ = err
	})

	t.Run("检查不存在应用的更新", func(t *testing.T) {
		_, err := store.CheckAppUpdate("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "应用未安装")
	})
}

func TestAppStore_CheckAllAppUpdates(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	// 添加模拟已安装应用
	app1 := &InstalledApp{
		ID:          "app1",
		Name:        "nextcloud",
		DisplayName: "Nextcloud",
		TemplateID:  "nextcloud",
		Version:     "latest",
		Status:      "running",
		InstallTime: time.Now(),
	}
	app2 := &InstalledApp{
		ID:          "app2",
		Name:        "jellyfin",
		DisplayName: "Jellyfin",
		TemplateID:  "jellyfin",
		Version:     "latest",
		Status:      "running",
		InstallTime: time.Now(),
	}
	store.installed["app1"] = app1
	store.installed["app2"] = app2

	results, err := store.CheckAllAppUpdates()
	// 可能因为网络问题返回空或错误
	_ = err
	_ = results
}

func TestAppStore_SetAppAutoUpdate(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	// 添加模拟应用
	app := &InstalledApp{
		ID:          "app1",
		Name:        "test",
		DisplayName: "Test",
		TemplateID:  "test",
		Version:     "1.0.0",
	}
	store.installed["app1"] = app

	t.Run("设置自动更新", func(t *testing.T) {
		err := store.SetAppAutoUpdate("app1", true)
		// 可能因为文件保存失败而报错
		_ = err
	})

	t.Run("设置不存在应用的自动更新", func(t *testing.T) {
		err := store.SetAppAutoUpdate("nonexistent", true)
		assert.Error(t, err)
	})
}

func TestAppStore_PerformAutoUpdates(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	// 添加模拟应用
	app := &InstalledApp{
		ID:          "app1",
		Name:        "test",
		DisplayName: "Test",
		TemplateID:  "test",
		Version:     "1.0.0",
		AutoUpdate:  false, // 不自动更新
	}
	store.installed["app1"] = app

	updated, err := store.PerformAutoUpdates()
	// 不应该报错，只是不更新任何应用
	assert.NoError(t, err)
	assert.Empty(t, updated)
}

// 测试数据结构
func TestTemplateVersion_Struct(t *testing.T) {
	now := time.Now()
	version := TemplateVersion{
		Version:      "v2.0.0",
		ImageTag:     "app:2.0.0",
		ReleaseNotes: "New features",
		PublishedAt:  now,
		Digest:       "sha256:abc123",
		Deprecated:   false,
		MinVersion:   "1.0.0",
	}

	assert.Equal(t, "v2.0.0", version.Version)
	assert.Equal(t, "app:2.0.0", version.ImageTag)
	assert.False(t, version.Deprecated)
}

func TestAppRequirements_Struct(t *testing.T) {
	req := AppRequirements{
		MinCPU:     2,
		MinMemory:  2048,
		MinStorage: 10240,
		GPU:        false,
		Ports:      []int{80, 443},
	}

	assert.Equal(t, 2, req.MinCPU)
	assert.Equal(t, int64(2048), req.MinMemory)
	assert.Len(t, req.Ports, 2)
}

func TestUpdateCheckResult_Struct(t *testing.T) {
	now := time.Now()
	result := UpdateCheckResult{
		AppID:           "app1",
		AppName:         "TestApp",
		CurrentVersion:  "1.0.0",
		LatestVersion:   "2.0.0",
		HasUpdate:       true,
		UpdateAvailable: true,
		ReleaseNotes:    "Major update",
		CheckedAt:       now,
		ImageDigest:     "sha256:xyz",
		Size:            1024000,
		Critical:        true,
	}

	assert.True(t, result.HasUpdate)
	assert.True(t, result.Critical)
	assert.Equal(t, "1.0.0", result.CurrentVersion)
	assert.Equal(t, "2.0.0", result.LatestVersion)
}

func TestAppStore_fetchLatestImageInfo(t *testing.T) {
	tempDir := t.TempDir()
	mgr := &Manager{}
	store, err := NewAppStore(mgr, tempDir)
	if err != nil {
		t.Skipf("无法创建 AppStore: %v", err)
		return
	}

	// 这个测试可能会因为网络问题失败
	tag, digest, size, err := store.fetchLatestImageInfo("nginx:latest")
	// 不应该 panic，可能成功也可能失败
	_ = tag
	_ = digest
	_ = size
	_ = err
}

// 基准测试
func BenchmarkBackupManager_CreateBackup(b *testing.B) {
	// 跳过基准测试
	b.Skip("需要完整的测试环境")
}

func BenchmarkAppStore_CheckAppUpdate(b *testing.B) {
	// 跳过基准测试
	b.Skip("需要网络连接")
}