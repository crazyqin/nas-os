package versioning

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConfig 创建测试用的配置
func testConfig(versionRoot string) *Config {
	config := DefaultConfig()
	config.VersionRoot = versionRoot
	config.ExcludePaths = []string{} // 清空排除路径以便测试
	return config
}

func TestNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	require.NotNil(t, mgr)
	defer mgr.Close()

	assert.DirExists(t, versionRoot)
}

func TestCreateVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello world"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	version, err := mgr.CreateVersion(testFile, "user1", "initial version", "manual")
	require.NoError(t, err)
	require.NotNil(t, version)

	assert.NotEmpty(t, version.ID)
	assert.Equal(t, testFile, version.FilePath)
	assert.Equal(t, int64(11), version.Size)
	assert.Equal(t, "user1", version.CreatedBy)
	assert.Equal(t, "manual", version.TriggerType)
	assert.FileExists(t, version.VersionPath)
}

func TestGetVersions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("version 1"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	err = os.WriteFile(testFile, []byte("version 2"), 0644)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	_, err = mgr.CreateVersion(testFile, "user1", "v2", "manual")
	require.NoError(t, err)

	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 2)

	assert.Equal(t, "v2", versions[0].Description)
	assert.Equal(t, "v1", versions[1].Description)
}

func TestRestoreVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("original content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	version, err := mgr.CreateVersion(testFile, "user1", "original", "manual")
	require.NoError(t, err)

	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "modified content", string(content))

	err = mgr.RestoreVersion(version.ID, "")
	require.NoError(t, err)

	content, err = os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "original content", string(content))
}

func TestDeleteVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	version, err := mgr.CreateVersion(testFile, "user1", "test", "manual")
	require.NoError(t, err)

	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 1)

	err = mgr.DeleteVersion(version.ID)
	require.NoError(t, err)

	versions, err = mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 0)

	assert.NoFileExists(t, version.VersionPath)
}

func TestGetDiff(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	version, err := mgr.CreateVersion(testFile, "user1", "test", "manual")
	require.NoError(t, err)

	err = os.WriteFile(testFile, []byte("line1\nmodified\nline3\n"), 0644)
	require.NoError(t, err)

	diff, err := mgr.GetDiff(version.ID)
	require.NoError(t, err)
	require.NotNil(t, diff)

	assert.Equal(t, version.ID, diff.VersionID)
	assert.Equal(t, "text", diff.DiffType)
	assert.Equal(t, ".txt", diff.FileType)
	assert.GreaterOrEqual(t, diff.ChangedLines, 1)
}

func TestRetentionPolicy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.Retention.MaxVersions = 3

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	for i := 0; i < 5; i++ {
		err = os.WriteFile(testFile, []byte("version"+string(rune('0'+i))), 0644)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
		_, err = mgr.CreateVersion(testFile, "user1", "", "manual")
		require.NoError(t, err)
	}

	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(versions), 3)
}

func TestChecksumDeduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("same content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	v1, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	v2, err := mgr.CreateVersion(testFile, "user1", "v2", "manual")
	require.NoError(t, err)

	assert.Equal(t, v1.ID, v2.ID)
}

func TestGetStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.CreateVersion(testFile, "user1", "test", "manual")
	require.NoError(t, err)

	stats := mgr.GetStats()
	assert.Equal(t, true, stats["enabled"])
	assert.Equal(t, 1, stats["totalFiles"])
	assert.Equal(t, 1, stats["totalVersions"])
}

func TestWatchFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	// 添加文件监控
	err = mgr.WatchFile(testFile)
	require.NoError(t, err)

	// 验证监控列表
	watched := mgr.GetWatchedFiles()
	assert.Contains(t, watched, testFile)

	// 移除监控
	mgr.UnwatchFile(testFile)
	watched = mgr.GetWatchedFiles()
	assert.NotContains(t, watched, testFile)
}

func TestWatchNonExistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	// 监控不存在的文件应该失败
	err = mgr.WatchFile("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestWatchDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	// 监控目录应该失败
	err = mgr.WatchFile(tmpDir)
	assert.Error(t, err)
}

func TestChangeBasedSnapshot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.Snapshot.Enabled = true
	config.Snapshot.TriggerMode = "change"
	config.Snapshot.MinChangeSize = 0 // 任何变更都触发

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 添加文件监控
	err = mgr.WatchFile(testFile)
	require.NoError(t, err)

	// 手动触发变更检查（模拟时间触发）
	mgr.checkAndSnapshotChanges()

	// 此时应该没有版本（因为文件没有被修改）
	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 0)

	// 修改文件
	time.Sleep(10 * time.Millisecond)
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// 触发变更检查
	mgr.checkAndSnapshotChanges()

	// 此时应该有版本
	versions, err = mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 1)
}

func TestMinChangeSizeThreshold(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.Snapshot.Enabled = true
	config.Snapshot.TriggerMode = "change"
	config.Snapshot.MinChangeSize = 1000 // 需要 1000 字节变更才触发

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 先创建一个初始版本
	_, err = mgr.CreateVersion(testFile, "user1", "initial", "manual")
	require.NoError(t, err)

	// 添加文件监控
	err = mgr.WatchFile(testFile)
	require.NoError(t, err)

	// 小幅度修改文件（不满足阈值）
	time.Sleep(10 * time.Millisecond)
	err = os.WriteFile(testFile, []byte("small change"), 0644)
	require.NoError(t, err)

	// 触发变更检查
	mgr.checkAndSnapshotChanges()

	// 不应该创建新版本（变更太小）
	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 1) // 只有初始版本
}

func TestTimeBasedSnapshot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.Snapshot.Enabled = true
	config.Snapshot.TriggerMode = "time"
	config.Snapshot.Interval = 1 // 1 分钟

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 添加文件监控
	err = mgr.WatchFile(testFile)
	require.NoError(t, err)

	// 手动触发时间快照
	mgr.snapshotWatchedFiles("time")

	// 应该有版本
	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 1)
}
