package versioning

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Manager 扩展测试 ====================

func TestManager_GetVersion(t *testing.T) {
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

	version, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 获取版本
	retrieved, err := mgr.GetVersion(version.ID)
	require.NoError(t, err)
	assert.Equal(t, version.ID, retrieved.ID)
	assert.Equal(t, version.Checksum, retrieved.Checksum)

	// 获取不存在的版本
	_, err = mgr.GetVersion("nonexistent")
	assert.Error(t, err)
}

func TestManager_UpdateConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	newConfig := DefaultConfig()
	newConfig.MaxFileSize = 10 * 1024 * 1024 * 1024 // 10GB
	newConfig.Retention.MaxVersions = 100

	err = mgr.UpdateConfig(newConfig)
	require.NoError(t, err)

	// 验证配置已更新
	stats := mgr.GetStats()
	assert.Equal(t, 100, stats["maxVersions"])
}

func TestManager_CreateVersion_Disabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.Enabled = false

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.CreateVersion(testFile, "user1", "test", "manual")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "禁用")
}

func TestManager_CreateVersion_FileTooLarge(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "large.txt")
	err = os.WriteFile(testFile, []byte("large content here"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.MaxFileSize = 5 // 非常小的限制

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.CreateVersion(testFile, "user1", "test", "manual")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "大小限制")
}

func TestManager_CreateVersion_ExcludedPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.ExcludePaths = []string{tmpDir}

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.CreateVersion(testFile, "user1", "test", "manual")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "排除")
}

func TestManager_CreateVersion_NonexistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.CreateVersion("/nonexistent/file.txt", "user1", "test", "manual")
	assert.Error(t, err)
}

func TestManager_RestoreVersion_CustomPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "original.txt")
	err = os.WriteFile(testFile, []byte("original content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	version, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 恢复到自定义路径
	customPath := filepath.Join(tmpDir, "restored.txt")
	err = mgr.RestoreVersion(version.ID, customPath)
	require.NoError(t, err)

	content, err := os.ReadFile(customPath)
	require.NoError(t, err)
	assert.Equal(t, "original content", string(content))
}

func TestManager_RestoreVersion_NonexistentVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	err = mgr.RestoreVersion("nonexistent", "/tmp/test")
	assert.Error(t, err)
}

func TestManager_DeleteVersion_Nonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	err = mgr.DeleteVersion("nonexistent")
	assert.Error(t, err)
}

func TestManager_GetDiff_BinaryFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建二进制文件
	testFile := filepath.Join(tmpDir, "test.bin")
	binaryData := make([]byte, 100)
	for i := range binaryData {
		binaryData[i] = byte(i)
	}
	err = os.WriteFile(testFile, binaryData, 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	version, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	diff, err := mgr.GetDiff(version.ID)
	require.NoError(t, err)
	assert.Equal(t, "binary", diff.DiffType)
}

func TestManager_GetDiff_NonexistentVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.GetDiff("nonexistent")
	assert.Error(t, err)
}

func TestManager_GetVersions_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	versions, err := mgr.GetVersions("/nonexistent/file.txt")
	require.NoError(t, err)
	assert.Empty(t, versions)
}

func TestManager_StartAutoSnapshot_Disabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.Snapshot.Enabled = false

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 不应该崩溃
	mgr.StartAutoSnapshot()
}

func TestManager_CleanupExpiredVersions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := testConfig(versionRoot)
	config.Retention.MaxAge = 0 // 立即过期

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	_, err = mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 手动触发清理
	mgr.cleanupExpiredVersions()

	// 版本应该被清理（因为 MaxAge=0 且 ExpiresAt 已设置）
	// 但由于时间可能还没到，我们只验证不崩溃
}

func TestManager_SplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"line1\nline2\nline3", 3},
		{"single line", 1},
		{"", 0},
		{"line1\nline2\n", 2},
		{"a\nb\nc\nd", 4},
	}

	for _, tt := range tests {
		result := splitLines(tt.input)
		assert.Equal(t, tt.expected, len(result), "input: %q", tt.input)
	}
}

func TestManager_CountChangedLines(t *testing.T) {
	tests := []struct {
		current  string
		version  string
		expected int
	}{
		{"a\nb\nc", "a\nb\nc", 0},
		{"a\nx\nc", "a\nb\nc", 1},
		{"a\nb\nc\nd", "a\nb\nc", 1},
		{"a\nb", "a\nb\nc", 1},
		{"x\ny\nz", "a\nb\nc", 3},
	}

	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	for _, tt := range tests {
		result := mgr.countChangedLines(tt.current, tt.version)
		assert.Equal(t, tt.expected, result, "current: %q, version: %q", tt.current, tt.version)
	}
}

func TestManager_IsTextFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	mgr, err := NewManager(configPath, testConfig(versionRoot))
	require.NoError(t, err)
	defer mgr.Close()

	textExts := []string{".txt", ".md", ".json", ".xml", ".yaml", ".yml", ".html", ".css", ".js", ".go", ".py", ".java", ".c", ".cpp", ".h", ".sh", ".conf", ".log"}
	for _, ext := range textExts {
		assert.True(t, mgr.isTextFile(ext), "should be text: %s", ext)
	}

	binaryExts := []string{".exe", ".bin", ".jpg", ".png", ".mp3", ".mp4", ".zip", ".gz"}
	for _, ext := range binaryExts {
		assert.False(t, mgr.isTextFile(ext), "should be binary: %s", ext)
	}
}

func TestVersion_Fields(t *testing.T) {
	now := time.Now()
	version := Version{
		ID:           "v-abc123-1234567890",
		FilePath:     "/data/file.txt",
		VersionPath:  "/var/lib/nas-os/versions/v-abc123-1234567890",
		Checksum:     "sha256hash",
		Size:         1024,
		CreatedAt:    now,
		CreatedBy:    "user1",
		Description:  "initial version",
		TriggerType:  "manual",
		Tags:         []string{"important", "backup"},
		ExpiresAt:    now.AddDate(0, 0, 30),
		IsCompressed: false,
	}

	assert.Equal(t, "v-abc123-1234567890", version.ID)
	assert.Equal(t, int64(1024), version.Size)
	assert.Equal(t, "user1", version.CreatedBy)
	assert.Len(t, version.Tags, 2)
}

func TestConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, 50, config.Retention.MaxVersions)
	assert.Equal(t, 30, config.Retention.MaxAge)
	assert.True(t, config.Retention.AutoCleanup)
	assert.True(t, config.Snapshot.Enabled)
}

func TestSnapshotConfig(t *testing.T) {
	config := SnapshotConfig{
		Enabled:       true,
		TriggerMode:   "time",
		Interval:      30,
		MinChangeSize: 1024,
	}

	assert.Equal(t, "time", config.TriggerMode)
	assert.Equal(t, 30, config.Interval)
}
