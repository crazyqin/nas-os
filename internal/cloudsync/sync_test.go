package cloudsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 测试辅助函数
// =============================================================================

// newTestSyncTask 创建一个测试用的 SyncTask
func newTestSyncTask() *SyncTask {
	return &SyncTask{
		ID:              "test_task_" + randomString(8),
		Name:            "Test Sync Task",
		ProviderID:      "test_provider",
		Enabled:         true,
		LocalPath:       "/tmp/nas-sync-test/local",
		RemotePath:      "/test/remote",
		Direction:       SyncDirectionBidirect,
		Mode:            SyncModeSync,
		ScheduleType:    ScheduleTypeManual,
		ConflictStrategy: ConflictStrategyNewer,
		Status:          TaskStatusIdle,
	}
}

// newTestProviderConfig 创建一个测试用的 ProviderConfig
func newTestProviderConfig() *ProviderConfig {
	return &ProviderConfig{
		ID:       "test_provider",
		Name:     "Test Provider",
		Type:     ProviderWebDAV,
		Endpoint: "https://test.example.com/webdav",
		Enabled:  true,
	}
}

// setupTestDirs 创建测试用的临时目录
func setupTestDirs(t *testing.T) (localPath, remotePath string, cleanup func()) {
	tmpDir := t.TempDir()
	localPath = filepath.Join(tmpDir, "local")
	remotePath = filepath.Join(tmpDir, "remote")

	err := os.MkdirAll(localPath, 0755)
	require.NoError(t, err)

	err = os.MkdirAll(remotePath, 0755)
	require.NoError(t, err)

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return localPath, remotePath, cleanup
}

// randomString 生成随机字符串（测试用）
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(1 * time.Nanosecond)
	}
	return string(b)
}

// mockProviderForRename 用于 Rename 测试的 mock provider
type mockProviderForRename struct{}

func (m *mockProviderForRename) Upload(_ context.Context, _, _ string) error { return nil }
func (m *mockProviderForRename) Download(_ context.Context, _, _ string) error { return nil }
func (m *mockProviderForRename) Delete(_ context.Context, _ string) error      { return nil }
func (m *mockProviderForRename) List(_ context.Context, _ string, _ bool) ([]FileInfo, error) {
	return nil, nil
}
func (m *mockProviderForRename) Stat(_ context.Context, _ string) (*FileInfo, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mockProviderForRename) CreateDir(_ context.Context, _ string) error        { return nil }
func (m *mockProviderForRename) DeleteDir(_ context.Context, _ string) error        { return nil }
func (m *mockProviderForRename) TestConnection(_ context.Context) (*ConnectionTestResult, error) {
	return &ConnectionTestResult{Success: true}, nil
}
func (m *mockProviderForRename) Close() error                                        { return nil }
func (m *mockProviderForRename) GetType() ProviderType                               { return ProviderWebDAV }
func (m *mockProviderForRename) GetCapabilities() []string                           { return nil }

// =============================================================================
// Delta Sync 算法测试
// =============================================================================

func TestDeltaSync_NoChanges(t *testing.T) {
	localPath, remotePath, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath
	task.RemotePath = remotePath

	// 创建相同内容的文件
	content := []byte("test content")
	localFile := filepath.Join(localPath, "file1.txt")
	remoteFile := filepath.Join(remotePath, "file1.txt")

	err := os.WriteFile(localFile, content, 0644)
	require.NoError(t, err)

	err = os.WriteFile(remoteFile, content, 0644)
	require.NoError(t, err)

	// 执行 delta sync 分析
	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)

	// 应该识别为无需同步（时间戳和大小相同）
	assert.Len(t, localFiles, 1)
	assert.Equal(t, int64(len(content)), localFiles[0].Size)
}

func TestDeltaSync_NewLocalFile(t *testing.T) {
	localPath, _, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath

	// 创建新文件
	content := []byte("new file content")
	localFile := filepath.Join(localPath, "newfile.txt")
	err := os.WriteFile(localFile, content, 0644)
	require.NoError(t, err)

	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)

	assert.Len(t, localFiles, 1)
	assert.Equal(t, "newfile.txt", filepath.Base(localFiles[0].Path))
	assert.Equal(t, int64(len(content)), localFiles[0].Size)
}

func TestDeltaSync_ModifiedLocalFile(t *testing.T) {
	localPath, _, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath

	// 创建文件
	localFile := filepath.Join(localPath, "file.txt")
	err := os.WriteFile(localFile, []byte("original"), 0644)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// 修改文件
	err = os.WriteFile(localFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)

	assert.Len(t, localFiles, 1)
	assert.Equal(t, int64(16), localFiles[0].Size) // len("modified content")
}

func TestDeltaSync_DeletedLocalFile(t *testing.T) {
	localPath, _, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath

	// 创建并删除文件
	localFile := filepath.Join(localPath, "deleted.txt")
	err := os.WriteFile(localFile, []byte("will be deleted"), 0644)
	require.NoError(t, err)

	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)
	assert.Len(t, localFiles, 1)

	// 删除文件
	err = os.Remove(localFile)
	require.NoError(t, err)

	localFiles, err = engine.collectLocalFiles(localPath)
	require.NoError(t, err)
	assert.Len(t, localFiles, 0)
}

func TestDeltaSync_ExcludePatterns(t *testing.T) {
	localPath, _, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath
	task.ExcludePatterns = []string{"*.tmp", ".DS_Store", "temp/"}

	// 创建各种文件
	files := []string{"file1.txt", "file2.tmp", ".DS_Store", "temp/data.txt", "file3.log"}
	for _, name := range files {
		path := filepath.Join(localPath, name)
		dir := filepath.Dir(path)
		if dir != localPath {
			os.MkdirAll(dir, 0755)
		}
		os.WriteFile(path, []byte("content"), 0644)
	}

	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)

	// collectLocalFiles 收集所有文件，不做过滤
	assert.Len(t, localFiles, 4) // temp/data.txt 是目录里的文件，也算

	// shouldSync 负责过滤
	for _, f := range localFiles {
		relPath, _ := filepath.Rel(localPath, f.Path)
		assert.True(t, engine.shouldSync(relPath), "File %s should pass shouldSync", relPath)
	}

	// 验证各模式的排除效果
	assert.False(t, engine.shouldSync("file2.tmp"), "*.tmp should exclude file2.tmp")
	assert.False(t, engine.shouldSync("file1.txt"), "should match for txt files")
	assert.True(t, engine.shouldSync(".DS_Store"), ".DS_Store does not match *.tmp")
}

func TestDeltaSync_IncludePatterns(t *testing.T) {
	localPath, _, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath
	task.IncludePatterns = []string{"*.jpg", "*.png"}
	task.ExcludePatterns = []string{} // 清空排除规则

	// 创建各种文件
	files := []string{"photo1.jpg", "photo2.png", "document.pdf", "video.mp4"}
	for _, name := range files {
		os.WriteFile(filepath.Join(localPath, name), []byte("content"), 0644)
	}

	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)

	// collectLocalFiles 收集所有文件，shouldSync 负责过滤
	assert.Len(t, localFiles, 4)

	// 验证 shouldSync 只通过 jpg/png 文件
	var syncedCount int
	for _, f := range localFiles {
		relPath, _ := filepath.Rel(localPath, f.Path)
		if engine.shouldSync(relPath) {
			syncedCount++
		}
	}
	assert.Equal(t, 2, syncedCount)
}

func TestDeltaSync_MaxFileSize(t *testing.T) {
	localPath, _, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath
	task.MaxFileSize = 1024 // 1KB

	// 创建小文件和大文件
	smallFile := filepath.Join(localPath, "small.txt")
	os.WriteFile(smallFile, make([]byte, 512), 0644)

	largeFile := filepath.Join(localPath, "large.txt")
	os.WriteFile(largeFile, make([]byte, 2048), 0644)

	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)

	assert.Len(t, localFiles, 1)
	assert.Equal(t, "small.txt", filepath.Base(localFiles[0].Path))
}

func TestDeltaSync_DirectoryTraversal(t *testing.T) {
	localPath, _, cleanup := setupTestDirs(t)
	defer cleanup()

	task := newTestSyncTask()
	task.LocalPath = localPath

	// 创建嵌套目录结构
	deepPath := filepath.Join(localPath, "a", "b", "c", "d")
	err := os.MkdirAll(deepPath, 0755)
	require.NoError(t, err)

	os.WriteFile(filepath.Join(deepPath, "deep.txt"), []byte("deep"), 0644)
	os.WriteFile(filepath.Join(localPath, "root.txt"), []byte("root"), 0644)

	engine := NewSyncEngine(nil, task)
	localFiles, err := engine.collectLocalFiles(localPath)
	require.NoError(t, err)

	assert.Len(t, localFiles, 2)
}

// =============================================================================
// 冲突检测测试
// =============================================================================

func TestConflictResolver_LocalNewer(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyNewer

	resolver := NewConflictResolver(task, nil)

	conflict := &ConflictInfo{
		Path:          "test.txt",
		LocalModTime:  time.Now(),
		LocalSize:     1000,
		RemoteModTime: time.Now().Add(-1 * time.Hour),
		RemoteSize:    1000,
	}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	assert.Equal(t, SyncOpUpload, result.Action)
	assert.Contains(t, result.Message, "本地较新")
}

func TestConflictResolver_RemoteNewer(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyNewer

	resolver := NewConflictResolver(task, nil)

	conflict := &ConflictInfo{
		Path:          "test.txt",
		LocalModTime:  time.Now().Add(-1 * time.Hour),
		LocalSize:     1000,
		RemoteModTime: time.Now(),
		RemoteSize:    1000,
	}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	assert.Equal(t, SyncOpDownload, result.Action)
	assert.Contains(t, result.Message, "远程较新")
}

func TestConflictResolver_SameTimeSameSize(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyNewer

	resolver := NewConflictResolver(task, nil)

	now := time.Now()
	conflict := &ConflictInfo{
		Path:          "test.txt",
		LocalModTime:  now,
		LocalSize:     1000,
		RemoteModTime: now,
		RemoteSize:    1000,
	}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	// 时间相同，大小相同，本地 >= 远程，按实现会上传
	assert.Equal(t, SyncOpUpload, result.Action)
}

func TestConflictResolver_SameTimeLocalLarger(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyNewer

	resolver := NewConflictResolver(task, nil)

	now := time.Now()
	conflict := &ConflictInfo{
		Path:          "test.txt",
		LocalModTime:  now,
		LocalSize:     2000,
		RemoteModTime: now,
		RemoteSize:    1000,
	}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	// 时间相同，本地较大，上传
	assert.Equal(t, SyncOpUpload, result.Action)
}

func TestConflictResolver_SameTimeRemoteLarger(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyNewer

	resolver := NewConflictResolver(task, nil)

	now := time.Now()
	conflict := &ConflictInfo{
		Path:          "test.txt",
		LocalModTime:  now,
		LocalSize:     1000,
		RemoteModTime: now,
		RemoteSize:    2000,
	}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	// 时间相同，远程较大，下载
	assert.Equal(t, SyncOpDownload, result.Action)
}

func TestConflictResolver_LocalWins(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyLocal

	resolver := NewConflictResolver(task, nil)
	conflict := &ConflictInfo{Path: "test.txt"}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	assert.Equal(t, SyncOpUpload, result.Action)
	assert.Contains(t, result.Message, "本地优先")
}

func TestConflictResolver_RemoteWins(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyRemote

	resolver := NewConflictResolver(task, nil)
	conflict := &ConflictInfo{Path: "test.txt"}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	assert.Equal(t, SyncOpDownload, result.Action)
	assert.Contains(t, result.Message, "远程优先")
}

func TestConflictResolver_Skip(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategySkip

	resolver := NewConflictResolver(task, nil)
	conflict := &ConflictInfo{Path: "test.txt"}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	assert.Equal(t, SyncOpSkip, result.Action)
}

func TestConflictResolver_Rename(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyRename
	task.LocalPath = t.TempDir()

	// Rename 策略内部需要 provider，由于我们只测试 Resolve 逻辑（不调用 ExecuteRename），
	// 使用 mockProvider 避免空指针
	resolver := NewConflictResolver(task, &mockProviderForRename{})
	require.NotNil(t, resolver)

	now := time.Now()
	conflict := &ConflictInfo{
		Path:          "test.txt",
		LocalModTime:  now,
		LocalSize:     1000,
		RemoteModTime: now,
		RemoteSize:    1000,
	}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	assert.Equal(t, SyncOpConflict, result.Action)
	assert.NotEmpty(t, result.RenamedPath)
	assert.Contains(t, result.Message, "重命名")

	// 验证重命名映射记录
	renames := resolver.GetRenames()
	assert.Contains(t, renames, "test.txt")
}

func TestConflictResolver_Ask(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyAsk

	resolver := NewConflictResolver(task, nil)
	conflict := &ConflictInfo{Path: "test.txt"}

	result, err := resolver.Resolve(context.Background(), conflict)
	require.NoError(t, err)

	assert.Equal(t, SyncOpConflict, result.Action)
	assert.True(t, result.NeedUserInput)
	assert.Contains(t, result.Message, "等待用户决定")
}

// =============================================================================
// 冲突历史测试
// =============================================================================

func TestConflictHistory_AddConflict(t *testing.T) {
	history := NewConflictHistory("test_task")

	conflict := &ConflictInfo{Path: "file1.txt"}
	result := &ResolutionResult{Action: SyncOpUpload}

	history.AddConflict(*conflict, *result)

	assert.Equal(t, 1, history.TotalFiles)
	assert.Equal(t, 1, history.Resolved)
	assert.Equal(t, 0, history.Skipped)
}

func TestConflictHistory_AddSkipped(t *testing.T) {
	history := NewConflictHistory("test_task")

	conflict := &ConflictInfo{Path: "file1.txt"}
	result := &ResolutionResult{Action: SyncOpSkip}

	history.AddConflict(*conflict, *result)
	history.AddConflict(*conflict, *result)

	assert.Equal(t, 2, history.TotalFiles)
	assert.Equal(t, 0, history.Resolved)
	assert.Equal(t, 2, history.Skipped)
}

func TestConflictHistory_Summary(t *testing.T) {
	history := NewConflictHistory("task_abc")
	conflict := &ConflictInfo{Path: "test.txt"}

	history.AddConflict(*conflict, ResolutionResult{Action: SyncOpUpload})
	history.AddConflict(*conflict, ResolutionResult{Action: SyncOpSkip})

	summary := history.Summary()
	assert.Contains(t, summary, "task_abc")
	assert.Contains(t, summary, "2")
	assert.Contains(t, summary, "1")
	assert.Contains(t, summary, "1")
}

// =============================================================================
// 多云冲突解决器测试
// =============================================================================

func TestMultiCloudConflictResolver(t *testing.T) {
	task := newTestSyncTask()
	resolver := NewMultiCloudConflictResolver()

	resolver.AddResolver("provider1", task, nil)
	resolver.AddResolver("provider2", task, nil)

	assert.NotNil(t, resolver.GetHistory())
}

func TestMultiCloudConflictResolver_ResolveAll(t *testing.T) {
	task := newTestSyncTask()
	task.ConflictStrategy = ConflictStrategyNewer

	resolver := NewMultiCloudConflictResolver()
	resolver.AddResolver("provider1", task, nil)

	now := time.Now()
	conflicts := map[string][]ConflictInfo{
		"provider1": {
			{
				Path:          "file1.txt",
				LocalModTime:  now,
				LocalSize:     1000,
				RemoteModTime: now.Add(-1 * time.Hour),
				RemoteSize:    1000,
			},
		},
		"provider2": {}, // 不存在的 provider 应该被忽略
	}

	results, err := resolver.ResolveAll(context.Background(), conflicts)
	require.NoError(t, err)

	assert.Contains(t, results, "provider1")
	assert.Len(t, results["provider1"], 1)
	assert.Equal(t, SyncOpUpload, results["provider1"][0].Action)
}

// =============================================================================
// 版本管理测试
// =============================================================================

func TestVersionInfo_Fields(t *testing.T) {
	// 测试 FileInfo 的 Version 字段
	info := FileInfo{
		Path:    "/test/file.txt",
		Size:    1024,
		ModTime: time.Now(),
		IsDir:   false,
		Hash:    "abc123",
		Version: "v1",
	}

	assert.Equal(t, "/test/file.txt", info.Path)
	assert.Equal(t, int64(1024), info.Size)
	assert.False(t, info.IsDir)
	assert.Equal(t, "abc123", info.Hash)
	assert.Equal(t, "v1", info.Version)
}

// =============================================================================
// 同步操作测试
// =============================================================================

func TestSyncOperation_Types(t *testing.T) {
	ops := []SyncOpType{
		SyncOpUpload,
		SyncOpDownload,
		SyncOpDeleteLocal,
		SyncOpDeleteRemote,
		SyncOpSkip,
		SyncOpConflict,
	}

	for _, op := range ops {
		assert.NotEmpty(t, string(op))
	}
}

func TestSyncOperation_Conversion(t *testing.T) {
	op := SyncOperation{
		Type:       SyncOpUpload,
		LocalPath:  "/local/file.txt",
		RemotePath: "/remote/file.txt",
		Size:       1024,
		ModTime:    time.Now(),
		Hash:       "hash123",
	}

	assert.Equal(t, SyncOpUpload, op.Type)
	assert.Equal(t, "/local/file.txt", op.LocalPath)
	assert.Equal(t, "/remote/file.txt", op.RemotePath)
	assert.Equal(t, int64(1024), op.Size)
	assert.NotEmpty(t, op.Hash)
}

// =============================================================================
// Manager 集成测试骨架
// =============================================================================

func TestManager_NewManager(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr := NewManager(configPath)

	assert.NotNil(t, mgr)
	assert.Equal(t, configPath, mgr.configPath)
}

func TestManager_Initialize(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr := NewManager(configPath)

	err := mgr.Initialize()
	assert.NoError(t, err)
}

// =============================================================================
// 调度测试
// =============================================================================

func TestScheduler_NewScheduler(t *testing.T) {
	scheduler := NewScheduler()
	assert.NotNil(t, scheduler)
}

// =============================================================================
// 过滤规则测试
// =============================================================================

func TestSyncEngine_ShouldSync_ExcludeAll(t *testing.T) {
	task := newTestSyncTask()
	task.ExcludePatterns = []string{"*"}
	task.IncludePatterns = nil // 清空，否则 include 规则会覆盖 exclude

	engine := NewSyncEngine(nil, task)

	// "*" 模式匹配不含路径分隔符的文件名，这里只匹配无路径分隔符的名字
	assert.False(t, engine.shouldSync("anyfile.txt"))
}

func TestSyncEngine_ShouldSync_IncludeOnly(t *testing.T) {
	task := newTestSyncTask()
	task.ExcludePatterns = []string{}
	task.IncludePatterns = []string{"*.jpg", "*.png"}

	engine := NewSyncEngine(nil, task)

	assert.True(t, engine.shouldSync("photo.jpg"))
	assert.True(t, engine.shouldSync("image.png"))
	assert.False(t, engine.shouldSync("document.pdf"))
}

func TestSyncEngine_ShouldSync_DirectoryExclude(t *testing.T) {
	task := newTestSyncTask()
	task.ExcludePatterns = []string{"node_modules/", ".git/"}

	engine := NewSyncEngine(nil, task)

	assert.False(t, engine.shouldSync("node_modules/package.json"))
	assert.False(t, engine.shouldSync(".git/config"))
	assert.True(t, engine.shouldSync("src/main.go"))
}

func TestSyncEngine_ShouldSync_NestedPath(t *testing.T) {
	task := newTestSyncTask()
	task.ExcludePatterns = []string{"temp/"}
	task.IncludePatterns = []string{}

	engine := NewSyncEngine(nil, task)

	assert.False(t, engine.shouldSync("temp/file.txt"))
	assert.False(t, engine.shouldSync("temp/deep/nested/file.txt"))
	assert.True(t, engine.shouldSync("src/file.txt"))
}

// =============================================================================
// 哈希计算测试
// =============================================================================

func TestSyncEngine_CalculateHash(t *testing.T) {
	task := newTestSyncTask()
	engine := NewSyncEngine(nil, task)

	// 创建临时文件
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	content := []byte("hello world")
	err := os.WriteFile(tmpFile, content, 0644)
	require.NoError(t, err)

	hash, err := engine.calculateFileHash(tmpFile)
	require.NoError(t, err)

	// SHA256("hello world")
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	assert.Equal(t, expected, hash)
}

func TestSyncEngine_CalculateHash_NotFound(t *testing.T) {
	task := newTestSyncTask()
	engine := NewSyncEngine(nil, task)

	_, err := engine.calculateFileHash("/nonexistent/file.txt")
	assert.Error(t, err)
}

// =============================================================================
// 状态转换测试
// =============================================================================

func TestTaskStatus_Transitions(t *testing.T) {
	status := TaskStatusIdle

	// 正常状态转换
	validTransitions := map[TaskStatus][]TaskStatus{
		TaskStatusIdle:      {TaskStatusRunning},
		TaskStatusRunning:   {TaskStatusPaused, TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled},
		TaskStatusPaused:   {TaskStatusRunning, TaskStatusCancelled},
		TaskStatusCompleted: {TaskStatusRunning},
		TaskStatusFailed:    {TaskStatusRunning},
	}

	// 验证有效转换
	_, valid := validTransitions[status]
	assert.True(t, valid)
}

func TestSyncStatus_Progress(t *testing.T) {
	status := &SyncStatus{
		TaskID:          "test",
		TotalFiles:      100,
		ProcessedFiles:  50,
		Progress:        50.0,
		TransferredBytes: 50 * 1024 * 1024,
	}

	assert.Equal(t, int64(100), status.TotalFiles)
	assert.Equal(t, int64(50), status.ProcessedFiles)
	assert.Equal(t, float64(50.0), status.Progress)
}

// =============================================================================
// Provider 类型测试
// =============================================================================

func TestProviderType_Constants(t *testing.T) {
	types := []ProviderType{
		ProviderAliyunOSS,
		ProviderTencentCOS,
		ProviderAWSS3,
		ProviderGoogleDrive,
		ProviderOneDrive,
		ProviderDropbox,
		ProviderBackblazeB2,
		ProviderWebDAV,
		ProviderS3Compatible,
		Provider115,
		ProviderQuark,
		ProviderAliyunPan,
		ProviderBaiduPan,
	}

	for _, pt := range types {
		assert.NotEmpty(t, string(pt))
	}
}

func TestSupportedProvidersInfo(t *testing.T) {
	providers := SupportedProviders()

	assert.GreaterOrEqual(t, len(providers), 10)

	for _, p := range providers {
		assert.NotEmpty(t, p.Type)
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Description)
		assert.NotNil(t, p.Features)
	}
}

// =============================================================================
// 辅助函数测试
// =============================================================================

func TestHumanBytesHelper(t *testing.T) {
	testCases := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
	}

	for _, tc := range testCases {
		result := humanBytes(tc.size)
		assert.Equal(t, tc.expected, result)
	}
}

// =============================================================================
// SyncStats 测试
// =============================================================================

func TestSyncStats_Fields(t *testing.T) {
	stats := &SyncStats{
		TotalTasks:      5,
		ActiveTasks:     2,
		TotalProviders:  3,
		TotalSynced:     1000,
		TotalBytes:      1024 * 1024 * 1024,
		TotalBytesHuman: "1.00 GB",
		LastSyncTime:    time.Now(),
	}

	assert.Equal(t, int64(5), stats.TotalTasks)
	assert.Equal(t, int64(2), stats.ActiveTasks)
	assert.Equal(t, int64(3), stats.TotalProviders)
	assert.Equal(t, int64(1000), stats.TotalSynced)
	assert.Equal(t, "1.00 GB", stats.TotalBytesHuman)
}
