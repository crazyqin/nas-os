package presto

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("默认配置", func(t *testing.T) {
		mgr := NewManager(nil, logger)
		assert.NotNil(t, mgr)
		assert.NotNil(t, mgr.config)
		assert.Equal(t, 8, mgr.config.MaxConcurrent)
		assert.Equal(t, 4*1024*1024, mgr.config.ChunkSize)
		assert.True(t, mgr.config.EnableCompression)
		assert.True(t, mgr.config.EnableEncryption)
	})

	t.Run("自定义配置", func(t *testing.T) {
		cfg := &Config{
			ListenAddr:    ":8443",
			MaxConcurrent: 4,
			ChunkSize:     1024 * 1024,
		}
		mgr := NewManager(cfg, logger)
		assert.Equal(t, 4, mgr.config.MaxConcurrent)
		assert.Equal(t, 1024*1024, mgr.config.ChunkSize)
	})

	t.Run("nil logger", func(t *testing.T) {
		mgr := NewManager(nil, nil)
		assert.NotNil(t, mgr)
		assert.NotNil(t, mgr.logger)
	})
}

func TestCreateTransfer(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("创建发送任务", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.StorageRoot = t.TempDir()
		cfg.TempDir = t.TempDir()
		mgr := NewManager(cfg, logger)

		// 创建临时文件
		tmpFile := createTempFile(t, cfg.StorageRoot, "test.txt", "hello world")

		transfer, err := mgr.CreateTransfer("test-transfer", tmpFile, "/dest/test.txt", ModeSend)
		require.NoError(t, err)
		assert.NotNil(t, transfer)
		assert.NotEmpty(t, transfer.ID)
		assert.Equal(t, "test-transfer", transfer.Name)
		assert.Equal(t, StatusPending, transfer.Status)
		assert.Equal(t, ModeSend, transfer.Mode)
		assert.True(t, transfer.Compressed)
		assert.True(t, transfer.Encrypted)
	})

	t.Run("创建接收任务", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.StorageRoot = t.TempDir()
		cfg.TempDir = t.TempDir()
		mgr := NewManager(cfg, logger)

		transfer, err := mgr.CreateTransfer("recv-transfer", "", "/dest/test.txt", ModeRecv)
		require.NoError(t, err)
		assert.NotNil(t, transfer)
		assert.Equal(t, ModeRecv, transfer.Mode)
	})

	t.Run("超过并发限制", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxConcurrent = 4
		cfg.StorageRoot = t.TempDir()
		cfg.TempDir = t.TempDir()
		mgr := NewManager(cfg, logger)

		// 创建多个任务直到达到限制
		for i := 0; i < cfg.MaxConcurrent; i++ {
			tmpFile := createTempFile(t, cfg.StorageRoot, fmt.Sprintf("test%d.txt", i), "data")
			_, err := mgr.CreateTransfer("transfer", tmpFile, "/dest", ModeSend)
			require.NoError(t, err)
		}

		// 下一个应该失败
		tmpFile := createTempFile(t, cfg.StorageRoot, "overflow.txt", "data")
		_, err := mgr.CreateTransfer("overflow", tmpFile, "/dest", ModeSend)
		assert.ErrorIs(t, err, ErrMaxConcurrent)
	})

	t.Run("文件不存在", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.StorageRoot = t.TempDir()
		cfg.TempDir = t.TempDir()
		mgr := NewManager(cfg, logger)

		_, err := mgr.CreateTransfer("no-file", "/nonexistent/file.txt", "/dest", ModeSend)
		assert.Error(t, err)
	})
}

func TestGetTransfer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	cfg.StorageRoot = t.TempDir()
	cfg.TempDir = t.TempDir()

	mgr := NewManager(cfg, logger)

	// 创建任务
	tmpFile := createTempFile(t, cfg.StorageRoot, "test.txt", "hello")
	transfer, _ := mgr.CreateTransfer("test", tmpFile, "/dest", ModeSend)

	t.Run("获取存在的任务", func(t *testing.T) {
		result, err := mgr.GetTransfer(transfer.ID)
		require.NoError(t, err)
		assert.Equal(t, transfer.ID, result.ID)
	})

	t.Run("获取不存在的任务", func(t *testing.T) {
		_, err := mgr.GetTransfer("nonexistent-id")
		assert.ErrorIs(t, err, ErrTransferNotFound)
	})
}

func TestListTransfers(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	cfg.StorageRoot = t.TempDir()
	cfg.TempDir = t.TempDir()

	mgr := NewManager(cfg, logger)

	// 创建多个任务
	for i := 0; i < 3; i++ {
		tmpFile := createTempFile(t, cfg.StorageRoot, "test.txt", "hello")
		mgr.CreateTransfer("test", tmpFile, "/dest", ModeSend)
	}

	transfers := mgr.ListTransfers()
	assert.Len(t, transfers, 3)
}

func TestCancelTransfer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	cfg.StorageRoot = t.TempDir()
	cfg.TempDir = t.TempDir()

	mgr := NewManager(cfg, logger)

	t.Run("取消等待中的任务", func(t *testing.T) {
		tmpFile := createTempFile(t, cfg.StorageRoot, "test.txt", "hello")
		transfer, _ := mgr.CreateTransfer("test", tmpFile, "/dest", ModeSend)

		err := mgr.CancelTransfer(transfer.ID)
		require.NoError(t, err)

		result, _ := mgr.GetTransfer(transfer.ID)
		assert.Equal(t, StatusCancelled, result.Status)
	})

	t.Run("取消不存在的任务", func(t *testing.T) {
		err := mgr.CancelTransfer("nonexistent")
		assert.ErrorIs(t, err, ErrTransferNotFound)
	})
}

func TestPauseResumeTransfer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	cfg.StorageRoot = t.TempDir()
	cfg.TempDir = t.TempDir()

	mgr := NewManager(cfg, logger)

	tmpFile := createTempFile(t, cfg.StorageRoot, "test.txt", "hello")
	transfer, _ := mgr.CreateTransfer("test", tmpFile, "/dest", ModeSend)

	// 设置为运行状态
	transfer.mu.Lock()
	transfer.Status = StatusRunning
	transfer.mu.Unlock()

	t.Run("暂停任务", func(t *testing.T) {
		err := mgr.PauseTransfer(transfer.ID)
		require.NoError(t, err)

		result, _ := mgr.GetTransfer(transfer.ID)
		assert.Equal(t, StatusPaused, result.Status)
	})

	t.Run("恢复任务", func(t *testing.T) {
		err := mgr.ResumeTransfer(transfer.ID)
		require.NoError(t, err)

		result, _ := mgr.GetTransfer(transfer.ID)
		assert.Equal(t, StatusRunning, result.Status)
	})

	t.Run("暂停非运行任务", func(t *testing.T) {
		transfer.mu.Lock()
		transfer.Status = StatusPending
		transfer.mu.Unlock()

		err := mgr.PauseTransfer(transfer.ID)
		assert.Error(t, err)
	})
}

func TestGetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	cfg.StorageRoot = t.TempDir()
	cfg.TempDir = t.TempDir()

	mgr := NewManager(cfg, logger)

	// 创建一些任务
	for i := 0; i < 5; i++ {
		tmpFile := createTempFile(t, cfg.StorageRoot, "test.txt", "hello")
		mgr.CreateTransfer("test", tmpFile, "/dest", ModeSend)
	}

	// 修改一些任务的状态
	transfers := mgr.ListTransfers()
	transfers[0].mu.Lock()
	transfers[0].Status = StatusCompleted
	transfers[0].mu.Unlock()

	transfers[1].mu.Lock()
	transfers[1].Status = StatusFailed
	transfers[1].mu.Unlock()

	stats := mgr.GetStats()
	assert.Equal(t, int64(5), stats.TotalTransfers)
	assert.Equal(t, 3, stats.ActiveTransfers)
	assert.Equal(t, 1, stats.CompletedCount)
	assert.Equal(t, 1, stats.FailedCount)
}

func TestCleanup(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := DefaultConfig()
	cfg.StorageRoot = t.TempDir()
	cfg.TempDir = t.TempDir()

	mgr := NewManager(cfg, logger)

	// 创建并完成一些任务
	tmpFile := createTempFile(t, cfg.StorageRoot, "test.txt", "hello")
	transfer, _ := mgr.CreateTransfer("test", tmpFile, "/dest", ModeSend)

	// 设置为已完成状态
	transfer.mu.Lock()
	transfer.Status = StatusCompleted
	pastTime := time.Now().Add(-2 * time.Hour)
	transfer.CompletedAt = &pastTime
	transfer.mu.Unlock()

	// 清理 1 小时前的任务
	count := mgr.Cleanup(1 * time.Hour)
	assert.Equal(t, 1, count)

	// 确认任务已被清理
	_, err := mgr.GetTransfer(transfer.ID)
	assert.ErrorIs(t, err, ErrTransferNotFound)
}

func TestEncryptDecrypt(t *testing.T) {
	t.Run("加密解密成功", func(t *testing.T) {
		key, err := GenerateEncryptionKey()
		require.NoError(t, err)

		plaintext := []byte("Hello, Presto!")

		// 加密
		ciphertext, err := Encrypt(plaintext, key)
		require.NoError(t, err)
		assert.NotEqual(t, plaintext, ciphertext)

		// 解密
		decrypted, err := Decrypt(ciphertext, key)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("错误密钥长度", func(t *testing.T) {
		shortKey := []byte("short-key")
		_, err := Encrypt([]byte("test"), shortKey)
		assert.Error(t, err)

		_, err = Decrypt([]byte("test"), shortKey)
		assert.Error(t, err)
	})

	t.Run("篡改密文", func(t *testing.T) {
		key, _ := GenerateEncryptionKey()
		plaintext := []byte("test data")

		ciphertext, _ := Encrypt(plaintext, key)
		ciphertext[10] ^= 0xFF // 篡改

		_, err := Decrypt(ciphertext, key)
		assert.Error(t, err)
	})
}

func TestComputeChecksum(t *testing.T) {
	data := []byte("Hello, World!")
	checksum := ComputeChecksum(data)
	assert.NotEmpty(t, checksum)
	assert.Len(t, checksum, 64) // SHA-256 hex

	// 相同数据应产生相同校验和
	checksum2 := ComputeChecksum(data)
	assert.Equal(t, checksum, checksum2)

	// 不同数据应产生不同校验和
	checksum3 := ComputeChecksum([]byte("Different data"))
	assert.NotEqual(t, checksum, checksum3)
}

func TestCalculateChunks(t *testing.T) {
	t.Run("正常分块", func(t *testing.T) {
		chunks := CalculateChunks(10*1024*1024, 4*1024*1024) // 10MB, 4MB chunks
		assert.Len(t, chunks, 3)

		assert.Equal(t, 0, chunks[0].Index)
		assert.Equal(t, int64(0), chunks[0].Offset)
		assert.Equal(t, int64(4*1024*1024), chunks[0].Size)

		assert.Equal(t, 1, chunks[1].Index)
		assert.Equal(t, int64(4*1024*1024), chunks[1].Offset)
		assert.Equal(t, int64(4*1024*1024), chunks[1].Size)

		assert.Equal(t, 2, chunks[2].Index)
		assert.Equal(t, int64(8*1024*1024), chunks[2].Offset)
		assert.Equal(t, int64(2*1024*1024), chunks[2].Size)
	})

	t.Run("小文件", func(t *testing.T) {
		chunks := CalculateChunks(1024, 4*1024*1024)
		assert.Len(t, chunks, 1)
		assert.Equal(t, int64(1024), chunks[0].Size)
	})

	t.Run("默认块大小", func(t *testing.T) {
		chunks := CalculateChunks(10*1024*1024, 0)
		assert.Len(t, chunks, 3)
	})
}

func TestMessageEncoding(t *testing.T) {
	t.Run("编码解码消息", func(t *testing.T) {
		payload := HandshakePayload{
			Version:  "1.0",
			ClientID: "test-client",
			Compress: true,
		}

		msg, err := NewMessage(MsgTypeHandshake, payload)
		require.NoError(t, err)

		// 编码
		data, err := EncodeMessage(msg)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		// 解码
		reader := newMockReader(data)
		decoded, err := DecodeMessage(reader)
		require.NoError(t, err)

		assert.Equal(t, msg.Type, decoded.Type)
		assert.Equal(t, msg.ID, decoded.ID)

		// 验证载荷
		var decodedPayload HandshakePayload
		err = json.Unmarshal(decoded.Payload, &decodedPayload)
		require.NoError(t, err)
		assert.Equal(t, payload.Version, decodedPayload.Version)
		assert.Equal(t, payload.ClientID, decodedPayload.ClientID)
		assert.Equal(t, payload.Compress, decodedPayload.Compress)
	})

	t.Run("nil 载荷", func(t *testing.T) {
		msg, err := NewMessage(MsgTypeHeartbeat, nil)
		require.NoError(t, err)
		assert.Nil(t, msg.Payload)

		data, err := EncodeMessage(msg)
		require.NoError(t, err)

		reader := newMockReader(data)
		decoded, err := DecodeMessage(reader)
		require.NoError(t, err)
		assert.Nil(t, decoded.Payload)
	})
}

func TestTransferInfo(t *testing.T) {
	transfer := &Transfer{
		ID:           "test-id",
		Name:         "test.txt",
		SourcePath:   "/src/test.txt",
		DestPath:     "/dst/test.txt",
		Mode:         ModeSend,
		Status:       StatusRunning,
		TotalBytes:   1024 * 1024,
		Transferred:  512 * 1024,
		ChunkCount:   10,
		ChunksDone:   5,
		SpeedBps:     1024 * 1024,
		Compressed:   true,
		Encrypted:    true,
		FileChecksum: "abc123",
		StartedAt:    time.Now().Add(-1 * time.Minute),
	}

	info := transfer.GetTransferInfo()

	assert.Equal(t, transfer.ID, info.ID)
	assert.Equal(t, transfer.Name, info.Name)
	assert.Equal(t, transfer.Status, info.Status)
	assert.InDelta(t, 50.0, info.Progress, 0.01)
	assert.NotEmpty(t, info.SpeedHuman)
	assert.NotEmpty(t, info.ElapsedHuman)
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		bps      float64
		expected string
	}{
		{0, "0 B/s"},
		{1024, "1.00 KB/s"},
		{1024 * 1024, "1.00 MB/s"},
		{1024 * 1024 * 1024, "1.00 GB/s"},
		{1.5 * 1024 * 1024, "1.50 MB/s"},
	}

	for _, tt := range tests {
		result := formatSpeed(tt.bps)
		assert.Equal(t, tt.expected, result)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "500ms"},
		{5 * time.Second, "5.0s"},
		{2*time.Minute + 30*time.Second, "2m30s"},
		{1*time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		assert.Equal(t, tt.expected, result)
	}
}

func TestSaveLoadTransferState(t *testing.T) {
	tmpDir := t.TempDir()

	transfer := &Transfer{
		ID:           "test-transfer-id",
		Name:         "test.txt",
		SourcePath:   "/src/test.txt",
		DestPath:     "/dst/test.txt",
		Mode:         ModeSend,
		TotalBytes:   1024 * 1024,
		Transferred:  512 * 1024,
		ChunkCount:   4,
		ChunksDone:   2,
		FileChecksum: "abc123",
		chunks: []ChunkState{
			{Index: 0, Offset: 0, Size: 256 * 1024, Status: "done", Checksum: "sum0"},
			{Index: 1, Offset: 256 * 1024, Size: 256 * 1024, Status: "done", Checksum: "sum1"},
			{Index: 2, Offset: 512 * 1024, Size: 256 * 1024, Status: "pending"},
			{Index: 3, Offset: 768 * 1024, Size: 256 * 1024, Status: "pending"},
		},
		checksums: map[int]string{
			0: "sum0",
			1: "sum1",
		},
	}

	// 保存状态
	err := transfer.SaveTransferState(tmpDir)
	require.NoError(t, err)

	// 加载状态
	loaded, err := LoadTransferState(tmpDir, transfer.ID)
	require.NoError(t, err)

	assert.Equal(t, transfer.ID, loaded.ID)
	assert.Equal(t, transfer.Name, loaded.Name)
	assert.Equal(t, transfer.TotalBytes, loaded.TotalBytes)
	assert.Equal(t, transfer.Transferred, loaded.Transferred)
	assert.Equal(t, transfer.ChunkCount, loaded.ChunkCount)
	assert.Equal(t, transfer.ChunksDone, loaded.ChunksDone)
	assert.Equal(t, transfer.FileChecksum, loaded.FileChecksum)
	assert.Len(t, loaded.chunks, 4)
	assert.Equal(t, "done", loaded.chunks[0].Status)
	assert.Equal(t, "pending", loaded.chunks[2].Status)
}

// 辅助函数

func createTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

// mockReader 模拟 io.Reader.
type mockReader struct {
	data []byte
	pos  int
}

func newMockReader(data []byte) *mockReader {
	return &mockReader{data: data}
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
