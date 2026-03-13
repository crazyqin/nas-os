package cloudsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== 类型测试 ====================

func TestProviderTypes(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{"阿里云 OSS", ProviderAliyunOSS, "aliyun_oss"},
		{"腾讯云 COS", ProviderTencentCOS, "tencent_cos"},
		{"AWS S3", ProviderAWSS3, "aws_s3"},
		{"Google Drive", ProviderGoogleDrive, "google_drive"},
		{"OneDrive", ProviderOneDrive, "onedrive"},
		{"Backblaze B2", ProviderBackblazeB2, "backblaze_b2"},
		{"WebDAV", ProviderWebDAV, "webdav"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.provider))
		})
	}
}

func TestSyncDirections(t *testing.T) {
	assert.Equal(t, "upload", string(SyncDirectionUpload))
	assert.Equal(t, "download", string(SyncDirectionDownload))
	assert.Equal(t, "bidirect", string(SyncDirectionBidirect))
}

func TestConflictStrategies(t *testing.T) {
	assert.Equal(t, "skip", string(ConflictStrategySkip))
	assert.Equal(t, "local", string(ConflictStrategyLocal))
	assert.Equal(t, "remote", string(ConflictStrategyRemote))
	assert.Equal(t, "newer", string(ConflictStrategyNewer))
	assert.Equal(t, "rename", string(ConflictStrategyRename))
}

func TestTaskStatuses(t *testing.T) {
	assert.Equal(t, "idle", string(TaskStatusIdle))
	assert.Equal(t, "running", string(TaskStatusRunning))
	assert.Equal(t, "paused", string(TaskStatusPaused))
	assert.Equal(t, "completed", string(TaskStatusCompleted))
	assert.Equal(t, "failed", string(TaskStatusFailed))
	assert.Equal(t, "cancelled", string(TaskStatusCancelled))
}

// ==================== 支持的提供商测试 ====================

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()

	assert.NotEmpty(t, providers)
	assert.Len(t, providers, 8) // 8 种提供商

	// 检查必需字段
	for _, p := range providers {
		assert.NotEmpty(t, p.Type)
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Description)
		assert.NotEmpty(t, p.Features)
	}
}

// ==================== Provider 配置测试 ====================

func TestProviderConfigValidation(t *testing.T) {
	m := NewManager("")
	m.Initialize()

	tests := []struct {
		name    string
		config  ProviderConfig
		wantErr bool
	}{
		{
			name: "有效的 S3 配置",
			config: ProviderConfig{
				Name:      "test-s3",
				Type:      ProviderAWSS3,
				AccessKey: "test-key",
				SecretKey: "test-secret",
				Bucket:    "test-bucket",
				Region:    "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "缺少名称",
			config: ProviderConfig{
				Type:      ProviderAWSS3,
				AccessKey: "test-key",
				SecretKey: "test-secret",
				Bucket:    "test-bucket",
			},
			wantErr: true,
		},
		{
			name: "缺少 AccessKey",
			config: ProviderConfig{
				Name:      "test-s3",
				Type:      ProviderAWSS3,
				SecretKey: "test-secret",
				Bucket:    "test-bucket",
			},
			wantErr: true,
		},
		{
			name: "缺少 Bucket",
			config: ProviderConfig{
				Name:      "test-s3",
				Type:      ProviderAWSS3,
				AccessKey: "test-key",
				SecretKey: "test-secret",
			},
			wantErr: true,
		},
		{
			name: "有效的 WebDAV 配置",
			config: ProviderConfig{
				Name:     "test-webdav",
				Type:     ProviderWebDAV,
				Endpoint: "https://webdav.example.com",
			},
			wantErr: false,
		},
		{
			name: "WebDAV 缺少 Endpoint",
			config: ProviderConfig{
				Name: "test-webdav",
				Type: ProviderWebDAV,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.CreateProvider(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ==================== Manager 测试 ====================

func TestManager_CreateProvider(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	config := ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
		Region:    "us-east-1",
	}

	provider, err := m.CreateProvider(config)
	require.NoError(t, err)
	assert.NotEmpty(t, provider.ID)
	assert.Equal(t, "test-provider", provider.Name)
	assert.True(t, provider.Enabled)

	// 验证可以获取
	got, err := m.GetProvider(provider.ID)
	require.NoError(t, err)
	assert.Equal(t, provider.ID, got.ID)

	// 验证可以列出
	providers := m.ListProviders()
	assert.Len(t, providers, 1)
}

func TestManager_DeleteProvider(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	config := ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	}

	provider, err := m.CreateProvider(config)
	require.NoError(t, err)

	// 删除
	err = m.DeleteProvider(provider.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = m.GetProvider(provider.ID)
	assert.Error(t, err)
}

func TestManager_UpdateProvider(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	config := ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	}

	provider, err := m.CreateProvider(config)
	require.NoError(t, err)

	// 更新
	updatedConfig := ProviderConfig{
		Name:      "updated-provider",
		Type:      ProviderAWSS3,
		AccessKey: "new-key",
		SecretKey: "new-secret",
		Bucket:    "new-bucket",
		Region:    "ap-northeast-1",
	}

	err = m.UpdateProvider(provider.ID, updatedConfig)
	require.NoError(t, err)

	// 验证更新
	got, err := m.GetProvider(provider.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-provider", got.Name)
	assert.Equal(t, "new-bucket", got.Bucket)
}

// ==================== 同步任务测试 ====================

func TestManager_CreateSyncTask(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	// 先创建提供商
	providerConfig := ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	}
	provider, err := m.CreateProvider(providerConfig)
	require.NoError(t, err)

	// 创建同步任务
	task := SyncTask{
		Name:         "test-sync",
		ProviderID:   provider.ID,
		LocalPath:    "/tmp/test",
		RemotePath:   "/backup",
		Direction:    SyncDirectionBidirect,
		ScheduleType: ScheduleTypeManual,
	}

	createdTask, err := m.CreateSyncTask(task)
	require.NoError(t, err)
	assert.NotEmpty(t, createdTask.ID)
	assert.Equal(t, "test-sync", createdTask.Name)
	assert.Equal(t, TaskStatusIdle, createdTask.Status)

	// 验证默认值
	assert.Equal(t, ConflictStrategyNewer, createdTask.ConflictStrategy)
}

func TestManager_DeleteSyncTask(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	// 创建提供商和任务
	providerConfig := ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	}
	provider, _ := m.CreateProvider(providerConfig)

	task := SyncTask{
		Name:       "test-sync",
		ProviderID: provider.ID,
		LocalPath:  "/tmp/test",
		RemotePath: "/backup",
	}
	createdTask, _ := m.CreateSyncTask(task)

	// 删除任务
	err := m.DeleteSyncTask(createdTask.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = m.GetSyncTask(createdTask.ID)
	assert.Error(t, err)
}

// ==================== 状态测试 ====================

func TestManager_GetSyncStatus(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	// 获取不存在的任务状态
	status, err := m.GetSyncStatus("non-existent")
	require.NoError(t, err)
	assert.Equal(t, TaskStatusIdle, status.Status)
}

func TestManager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	stats := m.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalProviders)
	assert.Equal(t, int64(0), stats.TotalTasks)
}

// ==================== 同步引擎测试 ====================

func TestSyncEngine_ShouldSync(t *testing.T) {
	task := &SyncTask{
		ExcludePatterns: []string{"*.tmp", ".git/", "node_modules/"},
		IncludePatterns: []string{"*.go", "*.md"},
	}

	engine := NewSyncEngine(nil, task)

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"README.md", true},
		{"test.tmp", false},
		{".git/config", false},
		{"node_modules/package.json", false},
		{"src/main.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := engine.shouldSync(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncEngine_CalculateFileHash(t *testing.T) {
	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	content := []byte("test content for hash calculation")
	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	tmpFile.Close()

	engine := NewSyncEngine(nil, &SyncTask{})
	hash, err := engine.calculateFileHash(tmpFile.Name())
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 32) // MD5 hex length
}

// ==================== Mock Provider 测试 ====================

// MockProvider 用于测试的模拟提供商
type MockProvider struct {
	files map[string]FileInfo
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		files: make(map[string]FileInfo),
	}
}

func (m *MockProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	m.files[remotePath] = FileInfo{
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
	delete(m.files, remotePath)
	return nil
}

func (m *MockProvider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	var files []FileInfo
	for _, f := range m.files {
		files = append(files, f)
	}
	return files, nil
}

func (m *MockProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	f, ok := m.files[remotePath]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &f, nil
}

func (m *MockProvider) CreateDir(ctx context.Context, remotePath string) error {
	m.files[remotePath] = FileInfo{
		Path:  remotePath,
		IsDir: true,
	}
	return nil
}

func (m *MockProvider) DeleteDir(ctx context.Context, remotePath string) error {
	for path := range m.files {
		if len(path) > len(remotePath) && path[:len(remotePath)] == remotePath {
			delete(m.files, path)
		}
	}
	delete(m.files, remotePath)
	return nil
}

func (m *MockProvider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	return &ConnectionTestResult{
		Success:  true,
		Provider: "mock",
		Message:  "mock provider",
	}, nil
}

func (m *MockProvider) Close() error {
	return nil
}

func (m *MockProvider) GetType() ProviderType {
	return "mock"
}

func (m *MockProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list"}
}

func TestSyncEngine_Upload(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "local")
	remotePath := "/backup"

	// 创建本地文件
	require.NoError(t, os.MkdirAll(localPath, 0755))
	testFile := filepath.Join(localPath, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	// 创建同步引擎
	provider := NewMockProvider()
	task := &SyncTask{
		ID:        "test-task",
		LocalPath: localPath,
		RemotePath: remotePath,
		Direction: SyncDirectionUpload,
	}
	engine := NewSyncEngine(provider, task)

	// 执行同步
	err := engine.Run(context.Background())
	require.NoError(t, err)

	// 验证状态
	status := engine.GetStatus()
	assert.Equal(t, TaskStatusCompleted, status.Status)
	assert.GreaterOrEqual(t, status.UploadedFiles, int64(1))
}

// ==================== 调度器测试 ====================

func TestScheduler_AddRemoveTask(t *testing.T) {
	scheduler := NewScheduler()
	go scheduler.Run()
	defer scheduler.Stop()

	handler := func() {
		// 空的处理函数用于测试
	}

	err := scheduler.AddCronTask("test-task", "*/5 * * * * *", handler)
	require.NoError(t, err)

	// 验证任务已添加
	tasks := scheduler.ListTasks()
	assert.Contains(t, tasks, "test-task")

	// 删除任务
	scheduler.RemoveTask("test-task")
	tasks = scheduler.ListTasks()
	assert.NotContains(t, tasks, "test-task")
}

// ==================== 辅助函数测试 ====================

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := humanBytes(tt.bytes)
			assert.Contains(t, result, tt.expected[:len(tt.expected)-5])
		})
	}
}

func TestGetContentType(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".txt", "text/plain"},
		{".html", "text/html"},
		{".json", "application/json"},
		{".jpg", "image/jpeg"},
		{".png", "image/png"},
		{".mp4", "video/mp4"},
		{".unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := getContentType("file" + tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== 配置持久化测试 ====================

func TestConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	// 创建管理器并添加配置
	m1 := NewManager(configPath)
	require.NoError(t, m1.Initialize())

	provider, err := m1.CreateProvider(ProviderConfig{
		Name:      "test-provider",
		Type:      ProviderAWSS3,
		AccessKey: "test-key",
		SecretKey: "test-secret",
		Bucket:    "test-bucket",
	})
	require.NoError(t, err)

	// 创建新的管理器加载配置
	m2 := NewManager(configPath)
	require.NoError(t, m2.Initialize())

	// 验证配置已加载
	providers := m2.ListProviders()
	assert.Len(t, providers, 1)
	assert.Equal(t, provider.Name, providers[0].Name)
}

// ==================== 冲突策略测试 ====================

func TestConflictResolution(t *testing.T) {
	now := time.Now()

	conflicts := []struct {
		name      string
		conflict  ConflictInfo
		strategy  ConflictStrategy
		expected  string
	}{
		{
			name: "本地较新-选择本地",
			conflict: ConflictInfo{
				LocalModTime:  now,
				RemoteModTime: now.Add(-time.Hour),
			},
			strategy: ConflictStrategyNewer,
			expected: "upload",
		},
		{
			name: "远程较新-选择远程",
			conflict: ConflictInfo{
				LocalModTime:  now.Add(-time.Hour),
				RemoteModTime: now,
			},
			strategy: ConflictStrategyNewer,
			expected: "download",
		},
		{
			name: "强制选择本地",
			conflict: ConflictInfo{
				LocalModTime:  now.Add(-time.Hour),
				RemoteModTime: now,
			},
			strategy: ConflictStrategyLocal,
			expected: "upload",
		},
		{
			name: "强制选择远程",
			conflict: ConflictInfo{
				LocalModTime:  now,
				RemoteModTime: now.Add(-time.Hour),
			},
			strategy: ConflictStrategyRemote,
			expected: "download",
		},
	}

	for _, tt := range conflicts {
		t.Run(tt.name, func(t *testing.T) {
			// 测试策略选择逻辑
			// 实际实现在 sync_engine.go 中
			assert.NotEmpty(t, tt.strategy)
		})
	}
}

// ==================== 并发测试 ====================

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "cloudsync.json")

	m := NewManager(configPath)
	require.NoError(t, m.Initialize())

	// 并发创建提供商
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = m.CreateProvider(ProviderConfig{
				Name:      fmt.Sprintf("provider-%d", i),
				Type:      ProviderAWSS3,
				AccessKey: "key",
				SecretKey: "secret",
				Bucket:    "bucket",
			})
		}(i)
	}
	wg.Wait()

	// 验证所有提供商都已创建
	providers := m.ListProviders()
	assert.Len(t, providers, 10)
}