// Package cloudfuse provides cloud storage mounting via FUSE
package cloudfuse

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"nas-os/internal/cloudsync"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== 类型测试 ====================

func TestMountTypes(t *testing.T) {
	tests := []struct {
		name     string
		mt       MountType
		expected string
	}{
		{"115网盘", MountType115, "115"},
		{"夸克网盘", MountTypeQuark, "quark"},
		{"阿里云盘", MountTypeAliyunPan, "aliyun_pan"},
		{"OneDrive", MountTypeOneDrive, "onedrive"},
		{"Google Drive", MountTypeGoogleDrive, "google_drive"},
		{"WebDAV", MountTypeWebDAV, "webdav"},
		{"S3", MountTypeS3, "s3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.mt))
		})
	}
}

func TestMountStatuses(t *testing.T) {
	assert.Equal(t, "idle", string(MountStatusIdle))
	assert.Equal(t, "mounting", string(MountStatusMounting))
	assert.Equal(t, "mounted", string(MountStatusMounted))
	assert.Equal(t, "unmounting", string(MountStatusUnmounting))
	assert.Equal(t, "error", string(MountStatusError))
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()

	assert.NotEmpty(t, providers)
	assert.Len(t, providers, 7)

	// 检查必需字段
	for _, p := range providers {
		assert.NotEmpty(t, p.Type)
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Description)
		assert.NotEmpty(t, p.Features)
	}

	// 验证包含中国网盘
	var has115, hasQuark, hasAliyunPan bool
	for _, p := range providers {
		switch p.Type {
		case MountType115:
			has115 = true
		case MountTypeQuark:
			hasQuark = true
		case MountTypeAliyunPan:
			hasAliyunPan = true
		}
	}

	assert.True(t, has115, "应该支持115网盘")
	assert.True(t, hasQuark, "应该支持夸克网盘")
	assert.True(t, hasAliyunPan, "应该支持阿里云盘")
}

// ==================== Manager 测试 ====================

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudfuse.json")

	m := NewManager(configPath)
	assert.NotNil(t, m)
	assert.NotNil(t, m.mounts)
	assert.NotNil(t, m.config)
}

func TestManager_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudfuse.json")

	m := NewManager(configPath)
	err := m.Initialize()
	require.NoError(t, err)

	// 验证配置文件已创建
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestManager_CreateMount(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudfuse.json")
	mountPoint := filepath.Join(tmpDir, "mnt")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	cfg := &MountConfig{
		ID:         "test-mount-1",
		Name:       "Test Mount",
		Type:       MountType115,
		MountPoint: mountPoint,
		Enabled:    true,
		CreatedAt:  time.Now(),
	}

	// 不测试实际挂载（需要 root 权限），只测试配置管理
	err := m.validateMountConfig(cfg)
	require.NoError(t, err)

	m.AddMountConfig(cfg)

	// 验证配置已保存
	assert.Len(t, m.config.Mounts, 1)
	assert.Equal(t, "test-mount-1", m.config.Mounts[0].ID)
}

func TestManager_UpdateMountConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudfuse.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	// 添加初始配置
	cfg := &MountConfig{
		ID:         "test-mount-1",
		Name:       "Test Mount",
		Type:       MountType115,
		MountPoint: "/mnt/test",
		CreatedAt:  time.Now(),
	}
	m.AddMountConfig(cfg)

	// 更新配置
	updatedCfg := &MountConfig{
		ID:         "test-mount-1",
		Name:       "Updated Mount",
		Type:       MountTypeQuark,
		MountPoint: "/mnt/updated",
	}

	err := m.UpdateMountConfig("test-mount-1", updatedCfg)
	require.NoError(t, err)

	// 验证更新
	assert.Equal(t, "Updated Mount", m.config.Mounts[0].Name)
	assert.Equal(t, MountTypeQuark, m.config.Mounts[0].Type)
}

func TestManager_RemoveMountConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudfuse.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	// 添加配置
	cfg := &MountConfig{
		ID:         "test-mount-1",
		Name:       "Test Mount",
		Type:       MountType115,
		MountPoint: "/mnt/test",
	}
	m.AddMountConfig(cfg)

	// 删除配置
	err := m.RemoveMountConfig("test-mount-1")
	require.NoError(t, err)

	// 验证删除
	assert.Len(t, m.config.Mounts, 0)
}

func TestManager_ValidateMountConfig(t *testing.T) {
	m := NewManager("")

	tests := []struct {
		name    string
		cfg     *MountConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			cfg: &MountConfig{
				Name:       "Test",
				MountPoint: "/mnt/test",
				Type:       MountType115,
			},
			wantErr: false,
		},
		{
			name: "缺少名称",
			cfg: &MountConfig{
				MountPoint: "/mnt/test",
				Type:       MountType115,
			},
			wantErr: true,
		},
		{
			name: "缺少挂载点",
			cfg: &MountConfig{
				Name: "Test",
				Type: MountType115,
			},
			wantErr: true,
		},
		{
			name: "缺少类型",
			cfg: &MountConfig{
				Name:       "Test",
				MountPoint: "/mnt/test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.validateMountConfig(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ==================== 缓存管理器测试 ====================

func TestCacheManager_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cm, err := NewCacheManager(cacheDir, 100) // 100MB
	require.NoError(t, err)
	defer cm.Close()

	assert.NotNil(t, cm)
	assert.Equal(t, int64(0), cm.UsedSize())
	assert.Equal(t, 0.0, cm.HitRate())
}

func TestCacheManager_PutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cm, err := NewCacheManager(cacheDir, 100)
	require.NoError(t, err)
	defer cm.Close()

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// 添加缓存
	cachePath := cm.GetCachePath("/remote/test.txt")
	err = os.MkdirAll(filepath.Dir(cachePath), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cachePath, []byte("test content"), 0644)
	require.NoError(t, err)

	err = cm.Put("/remote/test.txt", cachePath, 12)
	require.NoError(t, err)

	// 获取缓存
	path, ok := cm.Get("/remote/test.txt")
	assert.True(t, ok)
	assert.Equal(t, cachePath, path)

	// 检查统计
	hits, misses, _, usedSize, _ := cm.Stats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(0), misses)
	assert.Equal(t, int64(12), usedSize)
}

func TestCacheManager_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cm, err := NewCacheManager(cacheDir, 100)
	require.NoError(t, err)
	defer cm.Close()

	// 创建并添加缓存
	cachePath := cm.GetCachePath("/test/file.txt")
	err = os.MkdirAll(filepath.Dir(cachePath), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cachePath, []byte("content"), 0644)
	require.NoError(t, err)
	err = cm.Put("/test/file.txt", cachePath, 7)
	require.NoError(t, err)

	// 删除缓存
	cm.Remove("/test/file.txt")

	// 验证已删除
	_, ok := cm.Get("/test/file.txt")
	assert.False(t, ok)

	assert.Equal(t, int64(0), cm.UsedSize())
}

func TestCacheManager_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cm, err := NewCacheManager(cacheDir, 100)
	require.NoError(t, err)
	defer cm.Close()

	// 添加多个缓存
	for i := 0; i < 5; i++ {
		cachePath := cm.GetCachePath("/test/file" + string(rune('0'+i)) + ".txt")
		err = os.MkdirAll(filepath.Dir(cachePath), 0750)
		require.NoError(t, err)
		err = os.WriteFile(cachePath, []byte("content"), 0644)
		require.NoError(t, err)
		err = cm.Put("/test/file"+string(rune('0'+i))+".txt", cachePath, 7)
		require.NoError(t, err)
	}

	assert.Equal(t, int64(35), cm.UsedSize())

	// 清空缓存
	err = cm.Clear()
	require.NoError(t, err)

	assert.Equal(t, int64(0), cm.UsedSize())
}

func TestCacheManager_HitRate(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cm, err := NewCacheManager(cacheDir, 100)
	require.NoError(t, err)
	defer cm.Close()

	// 添加缓存
	cachePath := cm.GetCachePath("/test/file.txt")
	err = os.MkdirAll(filepath.Dir(cachePath), 0750)
	require.NoError(t, err)
	err = os.WriteFile(cachePath, []byte("content"), 0644)
	require.NoError(t, err)
	err = cm.Put("/test/file.txt", cachePath, 7)
	require.NoError(t, err)

	// 命中
	cm.Get("/test/file.txt")
	cm.Get("/test/file.txt")

	// 未命中
	cm.Get("/not/exist")

	// 命中率应该是 2/3
	hitRate := cm.HitRate()
	assert.InDelta(t, 0.6667, hitRate, 0.01)
}

// ==================== Provider 创建测试 ====================

func TestManager_CreateProvider_115(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(filepath.Join(tmpDir, "config.json"))

	cfg := &MountConfig{
		Type:        MountType115,
		AccessToken: "test-token",
		UserID:      "test-user",
	}

	provider, err := m.CreateProvider(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, cloudsync.Provider115, provider.GetType())
}

func TestManager_CreateProvider_Quark(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(filepath.Join(tmpDir, "config.json"))

	cfg := &MountConfig{
		Type:        MountTypeQuark,
		AccessToken: "test-token",
	}

	provider, err := m.CreateProvider(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, cloudsync.ProviderQuark, provider.GetType())
}

func TestManager_CreateProvider_AliyunPan(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(filepath.Join(tmpDir, "config.json"))

	cfg := &MountConfig{
		Type:         MountTypeAliyunPan,
		RefreshToken: "test-refresh-token",
		DriveID:      "test-drive-id",
	}

	provider, err := m.CreateProvider(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, cloudsync.ProviderAliyunPan, provider.GetType())
}

func TestManager_CreateProvider_OneDrive(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(filepath.Join(tmpDir, "config.json"))

	cfg := &MountConfig{
		Type:         MountTypeOneDrive,
		ClientID:     "test-client-id",
		TenantID:     "test-tenant-id",
		RefreshToken: "test-refresh-token",
	}

	provider, err := m.CreateProvider(cfg)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, cloudsync.ProviderOneDrive, provider.GetType())
}

// ==================== 并发测试 ====================

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudfuse.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	var wg sync.WaitGroup

	// 并发添加配置
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := &MountConfig{
				ID:         "mount-" + string(rune('0'+i)),
				Name:       "Mount " + string(rune('0'+i)),
				Type:       MountType115,
				MountPoint: "/mnt/test" + string(rune('0'+i)),
			}
			m.AddMountConfig(cfg)
		}(i)
	}

	wg.Wait()

	// 验证所有配置都已添加
	m.mu.RLock()
	count := len(m.config.Mounts)
	m.mu.RUnlock()

	assert.Equal(t, 10, count)
}

// ==================== 配置持久化测试 ====================

func TestConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudfuse.json")

	// 创建管理器并添加配置
	m1 := NewManager(configPath)
	require.NoError(t, m1.Initialize())

	cfg := &MountConfig{
		ID:         "persistent-mount",
		Name:       "Persistent Mount",
		Type:       MountType115,
		MountPoint: "/mnt/persistent",
		AutoMount:  true,
	}
	m1.AddMountConfig(cfg)

	// 创建新的管理器加载配置
	m2 := NewManager(configPath)
	require.NoError(t, m2.Initialize())

	// 验证配置已加载
	assert.Len(t, m2.config.Mounts, 1)
	assert.Equal(t, "persistent-mount", m2.config.Mounts[0].ID)
	assert.Equal(t, "Persistent Mount", m2.config.Mounts[0].Name)
}

// ==================== Mock Provider 测试 ====================

// MockProvider 用于测试的模拟提供商
type MockProvider struct {
	files map[string]cloudsync.FileInfo
	mu    sync.RWMutex
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		files: make(map[string]cloudsync.FileInfo),
	}
}

func (m *MockProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	m.files[remotePath] = cloudsync.FileInfo{
		Path:    remotePath,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   false,
	}
	return nil
}

func (m *MockProvider) Download(ctx context.Context, remotePath, localPath string) error {
	return nil
}

func (m *MockProvider) Delete(ctx context.Context, remotePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, remotePath)
	return nil
}

func (m *MockProvider) List(ctx context.Context, prefix string, recursive bool) ([]cloudsync.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var files []cloudsync.FileInfo
	for _, f := range m.files {
		files = append(files, f)
	}
	return files, nil
}

func (m *MockProvider) Stat(ctx context.Context, remotePath string) (*cloudsync.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	f, ok := m.files[remotePath]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &f, nil
}

func (m *MockProvider) CreateDir(ctx context.Context, remotePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.files[remotePath] = cloudsync.FileInfo{
		Path:  remotePath,
		IsDir: true,
	}
	return nil
}

func (m *MockProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return m.Delete(ctx, remotePath)
}

func (m *MockProvider) TestConnection(ctx context.Context) (*cloudsync.ConnectionTestResult, error) {
	return &cloudsync.ConnectionTestResult{
		Success:  true,
		Provider: "mock",
		Message:  "mock provider",
	}, nil
}

func (m *MockProvider) Close() error {
	return nil
}

func (m *MockProvider) GetType() cloudsync.ProviderType {
	return "mock"
}

func (m *MockProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list"}
}

func TestManager_RegisterProvider(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(filepath.Join(tmpDir, "config.json"))

	mockProvider := NewMockProvider()
	m.RegisterProvider("mock-provider", mockProvider)

	assert.NotNil(t, m.providers["mock-provider"])
}
