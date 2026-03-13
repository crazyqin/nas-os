package versioning

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := DefaultConfig()
	config.VersionRoot = versionRoot
	config.ExcludePaths = []string{} // 清空排除路径以便测试

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	require.NotNil(t, mgr)
	defer mgr.Close()

	// 验证目录已创建
	assert.DirExists(t, versionRoot)
}

func TestCreateVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "versioning-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello world"), 0644)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.json")
	versionRoot := filepath.Join(tmpDir, "versions")

	config := DefaultConfig()
	config.VersionRoot = versionRoot

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建版本
	version, err := mgr.CreateVersion(testFile, "user1", "initial version", "manual")
	require.NoError(t, err)
	require.NotNil(t, version)

	assert.NotEmpty(t, version.ID)
	assert.Equal(t, testFile, version.FilePath)
	assert.Equal(t, int64(11), version.Size)
	assert.Equal(t, "user1", version.CreatedBy)
	assert.Equal(t, "manual", version.TriggerType)

	// 验证版本文件存在
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

	config := DefaultConfig()
	config.VersionRoot = versionRoot

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建多个版本
	_, err = mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 修改文件
	err = os.WriteFile(testFile, []byte("version 2"), 0644)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // 确保时间戳不同

	_, err = mgr.CreateVersion(testFile, "user1", "v2", "manual")
	require.NoError(t, err)

	// 获取版本列表
	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 2)

	// 验证最新版本在前
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

	config := DefaultConfig()
	config.VersionRoot = versionRoot

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建版本
	version, err := mgr.CreateVersion(testFile, "user1", "original", "manual")
	require.NoError(t, err)

	// 修改文件
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// 验证内容已改变
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "modified content", string(content))

	// 恢复版本
	err = mgr.RestoreVersion(version.ID, "")
	require.NoError(t, err)

	// 验证内容已恢复
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

	config := DefaultConfig()
	config.VersionRoot = versionRoot

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建版本
	version, err := mgr.CreateVersion(testFile, "user1", "test", "manual")
	require.NoError(t, err)

	// 验证版本存在
	versions, err := mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 1)

	// 删除版本
	err = mgr.DeleteVersion(version.ID)
	require.NoError(t, err)

	// 验证版本已删除
	versions, err = mgr.GetVersions(testFile)
	require.NoError(t, err)
	assert.Len(t, versions, 0)

	// 验证版本文件已删除
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

	config := DefaultConfig()
	config.VersionRoot = versionRoot

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建版本
	version, err := mgr.CreateVersion(testFile, "user1", "test", "manual")
	require.NoError(t, err)

	// 修改文件
	err = os.WriteFile(testFile, []byte("line1\nmodified\nline3\n"), 0644)
	require.NoError(t, err)

	// 获取差异
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

	config := DefaultConfig()
	config.VersionRoot = versionRoot
	config.Retention.MaxVersions = 3 // 最多保留 3 个版本

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建 5 个版本
	for i := 0; i < 5; i++ {
		err = os.WriteFile(testFile, []byte("version"+string(rune('0'+i))), 0644)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
		_, err = mgr.CreateVersion(testFile, "user1", "", "manual")
		require.NoError(t, err)
	}

	// 验证只保留了 3 个版本
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

	config := DefaultConfig()
	config.VersionRoot = versionRoot

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建第一个版本
	v1, err := mgr.CreateVersion(testFile, "user1", "v1", "manual")
	require.NoError(t, err)

	// 不修改文件，创建第二个版本
	v2, err := mgr.CreateVersion(testFile, "user1", "v2", "manual")
	require.NoError(t, err)

	// 相同内容的版本应该返回已存在的版本
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

	config := DefaultConfig()
	config.VersionRoot = versionRoot

	mgr, err := NewManager(configPath, config)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建版本
	_, err = mgr.CreateVersion(testFile, "user1", "test", "manual")
	require.NoError(t, err)

	// 获取统计信息
	stats := mgr.GetStats()
	assert.Equal(t, true, stats["enabled"])
	assert.Equal(t, 1, stats["totalFiles"])
	assert.Equal(t, 1, stats["totalVersions"])
}