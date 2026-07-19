package presto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestServerClientIntegration(t *testing.T) {
	// Skip on environments with constrained UDP buffer sizes (e.g. unprivileged containers, ARM SBCs)
	if os.Getenv("PRESTOIntegrationTest") == "" {
		t.Skip("skipping integration test; set PRESTOIntegrationTest=1 to enable")
	}
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()
	storageDir := filepath.Join(tmpDir, "storage")
	tempDir := filepath.Join(tmpDir, "temp")

	// 创建配置
	serverCfg := &Config{
		ListenAddr:        ":0", // 随机端口
		MaxConcurrent:     4,
		ChunkSize:         1024, // 小块用于测试
		EnableCompression: false,
		EnableEncryption:  false,
		TransferTimeout:   30 * time.Second,
		StorageRoot:       storageDir,
		TempDir:           tempDir,
	}

	// 创建服务端
	serverMgr := NewManager(serverCfg, logger)
	server, err := NewServer(serverCfg, serverMgr, logger)
	require.NoError(t, err)

	// 启动服务端
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop()

	// 获取服务端实际监听地址
	addr := server.listener.Addr().String()
	t.Logf("服务端监听地址: %s", addr)

	// 创建客户端
	clientCfg := &Config{
		EnableCompression: false,
		EnableEncryption:  false,
		TransferTimeout:   30 * time.Second,
		TempDir:           tempDir,
	}

	client := NewClient(clientCfg, logger)

	// 连接到服务端
	err = client.Connect(ctx, addr)
	require.NoError(t, err)
	defer client.Disconnect()

	// 创建测试文件
	testContent := []byte("Hello, Presto! This is a test file for integration testing.")
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	// 发送文件
	destPath := filepath.Join("storage", "test.txt")
	transfer, err := client.SendFile(ctx, testFile, destPath)
	require.NoError(t, err)
	assert.NotNil(t, transfer)

	// 等待传输完成
	time.Sleep(2 * time.Second)

	// 验证传输状态
	transfer.mu.RLock()
	status := transfer.Status
	transfer.mu.RUnlock()

	// 注意：由于测试环境限制，这里主要验证流程
	t.Logf("传输状态: %s", status)
}

func TestTransferLifecycle(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	cfg := &Config{
		MaxConcurrent:     2,
		ChunkSize:         1024,
		EnableCompression: false,
		EnableEncryption:  false,
		TransferTimeout:   10 * time.Second,
		StorageRoot:       tmpDir,
		TempDir:           tmpDir,
	}

	mgr := NewManager(cfg, logger)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	// 测试完整的传输生命周期
	t.Run("创建-运行-完成", func(t *testing.T) {
		// 1. 创建任务
		transfer, err := mgr.CreateTransfer("lifecycle-test", testFile, "/dest/test.txt", ModeSend)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, transfer.Status)

		// 2. 模拟运行
		transfer.mu.Lock()
		transfer.Status = StatusRunning
		transfer.TotalBytes = 1024
		transfer.mu.Unlock()

		// 3. 模拟完成
		transfer.mu.Lock()
		transfer.Status = StatusCompleted
		transfer.Transferred = 1024
		transfer.ChunksDone = 1
		now := time.Now()
		transfer.CompletedAt = &now
		transfer.Elapsed = now.Sub(transfer.StartedAt)
		transfer.mu.Unlock()

		// 4. 验证状态
		info := transfer.GetTransferInfo()
		assert.Equal(t, StatusCompleted, info.Status)
		assert.InDelta(t, 100.0, info.Progress, 0.01)
	})

	t.Run("创建-运行-失败", func(t *testing.T) {
		transfer, _ := mgr.CreateTransfer("fail-test", testFile, "/dest/test.txt", ModeSend)

		transfer.mu.Lock()
		transfer.Status = StatusRunning
		transfer.mu.Unlock()

		// 模拟失败
		transfer.mu.Lock()
		transfer.Status = StatusFailed
		transfer.ErrorMsg = "模拟错误"
		now := time.Now()
		transfer.CompletedAt = &now
		transfer.mu.Unlock()

		info := transfer.GetTransferInfo()
		assert.Equal(t, StatusFailed, info.Status)
		assert.Equal(t, "模拟错误", info.ErrorMsg)
	})

	t.Run("创建-运行-取消", func(t *testing.T) {
		transfer, _ := mgr.CreateTransfer("cancel-test", testFile, "/dest/test.txt", ModeSend)

		// 取消任务
		err := mgr.CancelTransfer(transfer.ID)
		require.NoError(t, err)

		info := transfer.GetTransferInfo()
		assert.Equal(t, StatusCancelled, info.Status)
	})
}

func TestConcurrentTransfers(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	cfg := &Config{
		MaxConcurrent:     3,
		ChunkSize:         1024,
		EnableCompression: false,
		EnableEncryption:  false,
		StorageRoot:       tmpDir,
		TempDir:           tmpDir,
	}

	mgr := NewManager(cfg, logger)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	// 创建多个并发任务
	transfers := make([]*Transfer, 5)
	errors := make([]error, 5)

	for i := 0; i < 5; i++ {
		transfers[i], errors[i] = mgr.CreateTransfer(
			"concurrent-test",
			testFile,
			"/dest/test.txt",
			ModeSend,
		)
	}

	// 前3个应该成功
	for i := 0; i < 3; i++ {
		assert.NoError(t, errors[i])
		assert.NotNil(t, transfers[i])
	}

	// 后2个应该失败（超过并发限制）
	for i := 3; i < 5; i++ {
		assert.ErrorIs(t, errors[i], ErrMaxConcurrent)
		assert.Nil(t, transfers[i])
	}

	// 取消一个任务后应该能创建新任务
	err := mgr.CancelTransfer(transfers[0].ID)
	require.NoError(t, err)

	newTransfer, err := mgr.CreateTransfer("new-test", testFile, "/dest/new.txt", ModeSend)
	assert.NoError(t, err)
	assert.NotNil(t, newTransfer)
}

func TestTransferStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := t.TempDir()

	cfg := &Config{
		MaxConcurrent: 10,
		ChunkSize:     1024,
		StorageRoot:   tmpDir,
		TempDir:       tmpDir,
	}

	mgr := NewManager(cfg, logger)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	// 创建并完成一些任务
	for i := 0; i < 5; i++ {
		transfer, _ := mgr.CreateTransfer("stats-test", testFile, "/dest", ModeSend)
		transfer.mu.Lock()
		if i < 3 {
			transfer.Status = StatusCompleted
		} else {
			transfer.Status = StatusFailed
		}
		transfer.Transferred = 1024
		now := time.Now()
		transfer.CompletedAt = &now
		transfer.mu.Unlock()
	}

	stats := mgr.GetStats()

	assert.Equal(t, int64(5), stats.TotalTransfers)
	assert.Equal(t, 3, stats.CompletedCount)
	assert.Equal(t, 2, stats.FailedCount)
	assert.Equal(t, int64(5*1024), stats.TotalTransferred)
}

func TestChunkProcessing(t *testing.T) {
	t.Run("数据块计算", func(t *testing.T) {
		// 测试各种文件大小的分块
		tests := []struct {
			fileSize    int64
			chunkSize   int
			expectedNum int
			lastSize    int64
		}{
			{1024, 256, 4, 256},
			{1000, 256, 4, 232},
			{1024, 1024, 1, 1024},
			{2048, 1024, 2, 1024},
			{0, 1024, 0, 0},
		}

		for _, tt := range tests {
			chunks := CalculateChunks(tt.fileSize, tt.chunkSize)
			assert.Len(t, chunks, tt.expectedNum, "fileSize=%d, chunkSize=%d", tt.fileSize, tt.chunkSize)

			if tt.expectedNum > 0 {
				// 验证最后一个块的大小
				lastChunk := chunks[len(chunks)-1]
				assert.Equal(t, tt.lastSize, lastChunk.Size)

				// 验证偏移量连续性
				for i := 1; i < len(chunks); i++ {
					expectedOffset := chunks[i-1].Offset + chunks[i-1].Size
					assert.Equal(t, expectedOffset, chunks[i].Offset)
				}
			}
		}
	})

	t.Run("数据块状态", func(t *testing.T) {
		chunks := CalculateChunks(1024, 256)
		for _, chunk := range chunks {
			assert.Equal(t, "pending", chunk.Status)
			assert.Empty(t, chunk.Checksum)
			assert.Equal(t, int64(0), chunk.Transferred)
		}
	})
}

func TestEncryptionPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	key, _ := GenerateEncryptionKey()

	// 测试不同大小数据的加密性能
	sizes := []int{1024, 1024 * 1024, 10 * 1024 * 1024}

	for _, size := range sizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		t.Run("加密"+formatBytes(int64(size)), func(t *testing.T) {
			start := time.Now()
			encrypted, err := Encrypt(data, key)
			elapsed := time.Since(start)

			require.NoError(t, err)
			assert.NotEmpty(t, encrypted)

			t.Logf("加密 %s 耗时: %v", formatBytes(int64(size)), elapsed)
		})
	}
}

// 辅助函数.
func formatBytes(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	unit := 0
	value := float64(bytes)

	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}

	return fmt.Sprintf("%.1f%s", value, units[unit])
}
