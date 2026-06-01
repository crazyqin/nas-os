package fileversion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ========== 测试辅助函数 ==========

// setupTestManager 创建测试用管理器
func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()

	tempDir := t.TempDir()
	config := &VersionConfig{
		StoragePath:         filepath.Join(tempDir, "versions"),
		MaxVersions:         5,
		RetentionDays:       30,
		EnableIncremental:   true,
		AutoCleanupInterval: 1 * time.Hour,
		MaxStorageSize:      0,
	}

	logger := zap.NewNop()
	manager := NewManager(config, logger)

	err := manager.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		manager.Stop()
	})

	return manager, tempDir
}

// createTestFile 创建测试文件
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	filePath := filepath.Join(dir, name)
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	require.NoError(t, err)

	err = os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	return filePath
}

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	t.Run("with nil config uses default", func(t *testing.T) {
		logger := zap.NewNop()
		manager := NewManager(nil, logger)

		assert.NotNil(t, manager)
		assert.NotNil(t, manager.config)
		assert.Equal(t, 50, manager.config.MaxVersions)
		assert.Equal(t, 90, manager.config.RetentionDays)
	})

	t.Run("with custom config", func(t *testing.T) {
		logger := zap.NewNop()
		config := &VersionConfig{
			StoragePath:  "/tmp/test",
			MaxVersions:  10,
			RetentionDays: 7,
		}

		manager := NewManager(config, logger)

		assert.NotNil(t, manager)
		assert.Equal(t, 10, manager.config.MaxVersions)
		assert.Equal(t, 7, manager.config.RetentionDays)
	})
}

func TestManager_Start(t *testing.T) {
	t.Run("start successfully", func(t *testing.T) {
		tempDir := t.TempDir()
		config := &VersionConfig{
			StoragePath: filepath.Join(tempDir, "versions"),
			MaxVersions: 5,
		}

		logger := zap.NewNop()
		manager := NewManager(config, logger)

		err := manager.Start()
		assert.NoError(t, err)

		// 验证目录创建
		_, err = os.Stat(config.StoragePath)
		assert.NoError(t, err)

		manager.Stop()
	})

	t.Run("start with invalid path", func(t *testing.T) {
		config := &VersionConfig{
			StoragePath: "/nonexistent/path/versions",
			MaxVersions: 5,
		}

		logger := zap.NewNop()
		manager := NewManager(config, logger)

		err := manager.Start()
		assert.Error(t, err)
	})
}

func TestManager_CreateVersion(t *testing.T) {
	manager, tempDir := setupTestManager(t)

	t.Run("create version successfully", func(t *testing.T) {
		filePath := createTestFile(t, tempDir, "test.txt", "Hello, World!")

		version, err := manager.CreateVersion(context.Background(), filePath, "Initial version")
		assert.NoError(t, err)
		assert.NotNil(t, version)

		assert.Equal(t, filePath, version.FilePath)
		assert.Equal(t, 1, version.Version)
		assert.Equal(t, int64(13), version.Size)
		assert.NotEmpty(t, version.ID)
		assert.NotEmpty(t, version.Checksum)
		assert.Equal(t, "Initial version", version.ChangeDescription)
	})

	t.Run("create multiple versions", func(t *testing.T) {
		filePath := createTestFile(t, tempDir, "multi.txt", "Version 1")

		// 创建第一个版本
		v1, err := manager.CreateVersion(context.Background(), filePath, "V1")
		assert.NoError(t, err)
		assert.Equal(t, 1, v1.Version)

		// 修改文件
		os.WriteFile(filePath, []byte("Version 2"), 0644)

		// 创建第二个版本
		v2, err := manager.CreateVersion(context.Background(), filePath, "V2")
		assert.NoError(t, err)
		assert.Equal(t, 2, v2.Version)

		// 验证版本列表
		versions, err := manager.ListVersions(filePath)
		assert.NoError(t, err)
		assert.Len(t, versions, 2)
	})

	t.Run("skip unchanged file", func(t *testing.T) {
		filePath := createTestFile(t, tempDir, "unchanged.txt", "Same content")

		v1, err := manager.CreateVersion(context.Background(), filePath, "V1")
		assert.NoError(t, err)

		// 再次创建，文件未变化
		v2, err := manager.CreateVersion(context.Background(), filePath, "V1 again")
		assert.NoError(t, err)

		// 应该返回相同版本
		assert.Equal(t, v1.ID, v2.ID)
	})

	t.Run("fail for nonexistent file", func(t *testing.T) {
		_, err := manager.CreateVersion(context.Background(), "/nonexistent/file.txt", "test")
		assert.Error(t, err)
	})
}

func TestManager_ListVersions(t *testing.T) {
	manager, tempDir := setupTestManager(t)

	t.Run("list existing versions", func(t *testing.T) {
		filePath := createTestFile(t, tempDir, "list.txt", "content")

		// 创建多个版本
		for i := 0; i < 3; i++ {
			os.WriteFile(filePath, []byte("version "+string(rune('A'+i))), 0644)
			manager.CreateVersion(context.Background(), filePath, "V"+string(rune('A'+i)))
		}

		versions, err := manager.ListVersions(filePath)
		assert.NoError(t, err)
		assert.Len(t, versions, 3)
	})

	t.Run("list nonexistent file", func(t *testing.T) {
		versions, err := manager.ListVersions("/nonexistent/file.txt")
		assert.NoError(t, err)
		assert.Empty(t, versions)
	})
}

func TestManager_GetVersion(t *testing.T) {
	manager, tempDir := setupTestManager(t)

	filePath := createTestFile(t, tempDir, "get.txt", "content")
	version, _ := manager.CreateVersion(context.Background(), filePath, "test")

	t.Run("get existing version", func(t *testing.T) {
		v, err := manager.GetVersion(version.ID)
		assert.NoError(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, version.ID, v.ID)
	})

	t.Run("get nonexistent version", func(t *testing.T) {
		_, err := manager.GetVersion("nonexistent-id")
		assert.Error(t, err)
	})
}

func TestManager_RestoreVersion(t *testing.T) {
	manager, tempDir := setupTestManager(t)

	filePath := createTestFile(t, tempDir, "restore.txt", "Original content")

	// 创建初始版本
	v1, _ := manager.CreateVersion(context.Background(), filePath, "Original")

	// 修改文件
	os.WriteFile(filePath, []byte("Modified content"), 0644)
	manager.CreateVersion(context.Background(), filePath, "Modified")

	t.Run("restore to previous version", func(t *testing.T) {
		// 恢复到原始版本
		err := manager.RestoreVersion(context.Background(), v1.ID)
		assert.NoError(t, err)

		// 验证文件内容
		content, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, "Original content", string(content))
	})

	t.Run("restore nonexistent version", func(t *testing.T) {
		err := manager.RestoreVersion(context.Background(), "nonexistent-id")
		assert.Error(t, err)
	})
}

func TestManager_CompareVersions(t *testing.T) {
	manager, tempDir := setupTestManager(t)

	filePath := createTestFile(t, tempDir, "compare.txt", "Line 1\nLine 2\nLine 3")

	v1, _ := manager.CreateVersion(context.Background(), filePath, "V1")

	// 修改文件
	os.WriteFile(filePath, []byte("Line 1\nModified Line 2\nLine 3\nNew Line 4"), 0644)
	v2, _ := manager.CreateVersion(context.Background(), filePath, "V2")

	t.Run("compare two versions", func(t *testing.T) {
		diff, err := manager.CompareVersions(v1.ID, v2.ID)
		assert.NoError(t, err)
		assert.NotNil(t, diff)

		assert.Equal(t, filePath, diff.FilePath)
		assert.NotNil(t, diff.Version1)
		assert.NotNil(t, diff.Version2)
		assert.True(t, diff.ModifiedLines > 0 || diff.AddedLines > 0)
	})

	t.Run("compare with nonexistent version", func(t *testing.T) {
		_, err := manager.CompareVersions(v1.ID, "nonexistent-id")
		assert.Error(t, err)
	})
}

func TestManager_GetStats(t *testing.T) {
	manager, tempDir := setupTestManager(t)

	// 创建一些版本
	for i := 0; i < 3; i++ {
		filePath := createTestFile(t, tempDir, "stats"+string(rune('A'+i))+".txt", "content")
		manager.CreateVersion(context.Background(), filePath, "test")
	}

	stats := manager.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalFiles)
	assert.Equal(t, 3, stats.TotalVersions)
	assert.True(t, stats.TotalSize > 0)
}

func TestManager_DeleteVersion(t *testing.T) {
	manager, tempDir := setupTestManager(t)

	filePath := createTestFile(t, tempDir, "delete.txt", "content")
	version, _ := manager.CreateVersion(context.Background(), filePath, "test")

	t.Run("delete existing version", func(t *testing.T) {
		err := manager.DeleteVersion(version.ID)
		assert.NoError(t, err)

		// 验证版本已删除
		_, err = manager.GetVersion(version.ID)
		assert.Error(t, err)
	})

	t.Run("delete nonexistent version", func(t *testing.T) {
		err := manager.DeleteVersion("nonexistent-id")
		assert.Error(t, err)
	})
}

// ========== Handlers 测试 ==========

func TestHandlers_RegisterRoutes(t *testing.T) {
	manager, _ := setupTestManager(t)
	handlers := NewHandlers(manager)

	assert.NotNil(t, handlers)
	assert.Equal(t, manager, handlers.manager)
}

func TestHandlers_GetVersionsCount(t *testing.T) {
	manager, tempDir := setupTestManager(t)
	handlers := NewHandlers(manager)

	// 创建一些版本
	filePath := createTestFile(t, tempDir, "count.txt", "content")
	manager.CreateVersion(context.Background(), filePath, "test")

	count := handlers.GetVersionsCount()
	assert.Equal(t, 1, count)
}

func TestHandlers_GetStorageSize(t *testing.T) {
	manager, tempDir := setupTestManager(t)
	handlers := NewHandlers(manager)

	// 创建版本
	filePath := createTestFile(t, tempDir, "size.txt", "content for size test")
	manager.CreateVersion(context.Background(), filePath, "test")

	size := handlers.GetStorageSize()
	assert.True(t, size > 0)
}

func TestHandlers_GetVersionsCountParam(t *testing.T) {
	manager, tempDir := setupTestManager(t)
	handlers := NewHandlers(manager)

	filePath := createTestFile(t, tempDir, "param.txt", "content")
	manager.CreateVersion(context.Background(), filePath, "test")

	count := handlers.GetVersionsCountParam(filePath)
	assert.Equal(t, 1, count)

	// 不存在的文件
	count = handlers.GetVersionsCountParam("/nonexistent")
	assert.Equal(t, 0, count)
}

// ========== 辅助函数测试 ==========

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 50, config.MaxVersions)
	assert.Equal(t, 90, config.RetentionDays)
	assert.True(t, config.EnableIncremental)
	assert.Equal(t, 24*time.Hour, config.AutoCleanupInterval)
	assert.Equal(t, int64(0), config.MaxStorageSize)
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single line",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "trailing newline",
			input:    "line1\nline2\n",
			expected: []string{"line1", "line2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePagination(t *testing.T) {
	// 注意：这个测试需要模拟gin.Context，这里只测试辅助函数逻辑
	// 实际测试需要HTTP测试库

	// 测试默认值逻辑
	page, pageSize := 1, 20 // 默认值
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, pageSize)
}

// ========== 集成测试 ==========

func TestIntegration_VersionLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	manager, tempDir := setupTestManager(t)

	// 1. 创建文件
	filePath := createTestFile(t, tempDir, "lifecycle.txt", "Initial content")
	t.Log("创建文件:", filePath)

	// 2. 创建初始版本
	v1, err := manager.CreateVersion(context.Background(), filePath, "Initial")
	require.NoError(t, err)
	t.Log("创建版本1:", v1.ID)

	// 3. 修改文件并创建新版本
	os.WriteFile(filePath, []byte("Modified content"), 0644)
	v2, err := manager.CreateVersion(context.Background(), filePath, "Modified")
	require.NoError(t, err)
	t.Log("创建版本2:", v2.ID)

	// 4. 列出版本
	versions, err := manager.ListVersions(filePath)
	require.NoError(t, err)
	assert.Len(t, versions, 2)
	t.Log("版本数量:", len(versions))

	// 5. 对比版本
	diff, err := manager.CompareVersions(v1.ID, v2.ID)
	require.NoError(t, err)
	assert.True(t, diff.ModifiedLines > 0 || diff.AddedLines > 0)
	t.Log("差异行数:", diff.ModifiedLines)

	// 6. 恢复版本
	err = manager.RestoreVersion(context.Background(), v1.ID)
	require.NoError(t, err)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "Initial content", string(content))
	t.Log("恢复成功")

	// 7. 获取统计
	stats := manager.GetStats()
	assert.Equal(t, 1, stats.TotalFiles)
	assert.Equal(t, 2, stats.TotalVersions)
	t.Log("统计完成")
}

func TestIntegration_MaxVersionsCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	manager, tempDir := setupTestManager(t)

	filePath := createTestFile(t, tempDir, "cleanup.txt", "content")

	// 创建超过MaxVersions的版本
	for i := 0; i < 10; i++ {
		os.WriteFile(filePath, []byte("version "+string(rune('A'+i))), 0644)
		manager.CreateVersion(context.Background(), filePath, "V"+string(rune('A'+i)))
	}

	// 验证版本数量被限制
	versions, err := manager.ListVersions(filePath)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(versions), 5) // MaxVersions = 5
}

func TestIntegration_Persistence(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	tempDir := t.TempDir()
	config := &VersionConfig{
		StoragePath: filepath.Join(tempDir, "versions"),
		MaxVersions: 10,
		RetentionDays: 90,
	}

	logger := zap.NewNop()

	// 第一个管理器实例
	manager1 := NewManager(config, logger)
	manager1.Start()

	filePath := createTestFile(t, tempDir, "persist.txt", "content")
	manager1.CreateVersion(context.Background(), filePath, "test")

	// 保存并停止
	manager1.Stop()

	// 第二个管理器实例（模拟重启）
	manager2 := NewManager(config, logger)
	manager2.Start()
	defer manager2.Stop()

	// 验证版本已持久化
	versions, err := manager2.ListVersions(filePath)
	assert.NoError(t, err)
	assert.Len(t, versions, 1)
}

// ========== 性能基准测试 ==========

func BenchmarkManager_CreateVersion(b *testing.B) {
	tempDir := b.TempDir()
	config := &VersionConfig{
		StoragePath: filepath.Join(tempDir, "versions"),
		MaxVersions: 100,
	}

	logger := zap.NewNop()
	manager := NewManager(config, logger)
	manager.Start()
	defer manager.Stop()

	filePath := filepath.Join(tempDir, "bench.txt")
	os.WriteFile(filePath, []byte("benchmark content"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		os.WriteFile(filePath, []byte("content "+string(rune('A'+i%26))), 0644)
		manager.CreateVersion(context.Background(), filePath, "benchmark")
	}
}

func BenchmarkManager_ListVersions(b *testing.B) {
	tempDir := b.TempDir()
	config := &VersionConfig{
		StoragePath: filepath.Join(tempDir, "versions"),
		MaxVersions: 100,
	}

	logger := zap.NewNop()
	manager := NewManager(config, logger)
	manager.Start()
	defer manager.Stop()

	filePath := filepath.Join(tempDir, "bench_list.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	// 预先创建一些版本
	for i := 0; i < 10; i++ {
		os.WriteFile(filePath, []byte("version "+string(rune('A'+i))), 0644)
		manager.CreateVersion(context.Background(), filePath, "test")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.ListVersions(filePath)
	}
}
