// Package audit 提供审计日志单元测试
package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== FileAuditLogger 测试 ==========

func TestNewFileAuditLogger(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := FileAuditConfig{
		Enabled:          true,
		LogPath:          tmpDir,
		MaxMemorySize:    1000,
		MaxFileSize:      10,
		MaxFileCount:     5,
		MaxAgeDays:       7,
		CompressAge:      3,
		EnableSignatures: true,
		FlushInterval:    time.Second,
	}

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	require.NotNil(t, logger)

	defer logger.Stop()

	assert.True(t, logger.config.Enabled)
	assert.Equal(t, tmpDir, logger.config.LogPath)
}

func TestFileAuditLogger_Log(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil // 不排除任何操作

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	// 测试记录日志
	entry := &FileAuditEntry{
		Protocol:  ProtocolSMB,
		ShareName: "test-share",
		SharePath: "/data/share",
		UserID:    "user-001",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Operation: OpCreate,
		FilePath:  "/data/share/test.txt",
		FileName:  "test.txt",
		Status:    StatusSuccess,
	}

	err = logger.Log(context.Background(), entry)
	assert.NoError(t, err)

	// 验证日志被记录
	assert.Len(t, logger.entries, 1)
	assert.NotEmpty(t, logger.entries[0].ID)
	assert.NotEmpty(t, logger.entries[0].Timestamp)
	assert.NotEmpty(t, logger.entries[0].Signature)
}

func TestFileAuditLogger_LogSMBOperation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	err = logger.LogSMBOperation(
		context.Background(),
		"share1",
		"/data/share1",
		"user-001",
		"testuser",
		"192.168.1.100",
		OpWrite,
		"/data/share1/file.txt",
		StatusSuccess,
		map[string]interface{}{"bytes": 1024},
	)

	assert.NoError(t, err)
	assert.Len(t, logger.entries, 1)
	assert.Equal(t, ProtocolSMB, logger.entries[0].Protocol)
	assert.Equal(t, OpWrite, logger.entries[0].Operation)
}

func TestFileAuditLogger_LogNFSOperation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	err = logger.LogNFSOperation(
		context.Background(),
		"/exports/data",
		"user-002",
		"nfsuser",
		"10.0.0.1",
		OpDelete,
		"/exports/data/old.txt",
		StatusSuccess,
		nil,
	)

	assert.NoError(t, err)
	assert.Len(t, logger.entries, 1)
	assert.Equal(t, ProtocolNFS, logger.entries[0].Protocol)
	assert.Equal(t, OpDelete, logger.entries[0].Operation)
}

func TestFileAuditLogger_ExcludeOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = []FileOperation{OpRead, OpList}

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	// 记录被排除的操作
	err = logger.LogSMBOperation(
		context.Background(), "share", "/data", "user", "test", "127.0.0.1",
		OpRead, "/data/file.txt", StatusSuccess, nil,
	)
	assert.NoError(t, err)
	assert.Len(t, logger.entries, 0) // 应该被排除

	// 记录未被排除的操作
	err = logger.LogSMBOperation(
		context.Background(), "share", "/data", "user", "test", "127.0.0.1",
		OpCreate, "/data/file.txt", StatusSuccess, nil,
	)
	assert.NoError(t, err)
	assert.Len(t, logger.entries, 1) // 不应该被排除
}

func TestFileAuditLogger_Query(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	// 添加多条日志
	for i := 0; i < 10; i++ {
		protocol := ProtocolSMB
		if i%2 == 0 {
			protocol = ProtocolNFS
		}

		entry := &FileAuditEntry{
			Protocol:  protocol,
			UserID:    "user-001",
			Username:  "testuser",
			ClientIP:  "192.168.1.100",
			Operation: FileOperation([]string{"create", "delete", "write"}[i%3]),
			FilePath:  "/data/share/file.txt",
			Status:    StatusSuccess,
		}
		logger.Log(context.Background(), entry)
	}

	// 测试查询全部
	result, err := logger.Query(FileAuditQueryOptions{Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 10, result.Total)
	assert.Len(t, result.Entries, 10)

	// 测试协议过滤
	result, err = logger.Query(FileAuditQueryOptions{Protocol: ProtocolSMB, Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total) // 5条SMB日志

	// 测试分页
	result, err = logger.Query(FileAuditQueryOptions{Limit: 3, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, 10, result.Total)
	assert.Len(t, result.Entries, 3)

	result, err = logger.Query(FileAuditQueryOptions{Limit: 3, Offset: 5})
	require.NoError(t, err)
	assert.Equal(t, 10, result.Total)
	assert.Len(t, result.Entries, 3)
}

func TestFileAuditLogger_GetByID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	// 记录日志
	entry := &FileAuditEntry{
		Protocol:  ProtocolSMB,
		UserID:    "user-001",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Operation: OpCreate,
		FilePath:  "/data/test.txt",
		Status:    StatusSuccess,
	}
	logger.Log(context.Background(), entry)

	// 根据ID查询
	found, err := logger.GetByID(entry.ID)
	require.NoError(t, err)
	assert.Equal(t, entry.ID, found.ID)
	assert.Equal(t, "testuser", found.Username)

	// 查询不存在的ID
	_, err = logger.GetByID("non-existent")
	assert.Error(t, err)
}

func TestFileAuditLogger_GetStatistics(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	// 添加不同类型的日志
	logs := []struct {
		protocol  Protocol
		operation FileOperation
		status    Status
		user      string
		ip        string
	}{
		{ProtocolSMB, OpCreate, StatusSuccess, "user1", "192.168.1.1"},
		{ProtocolSMB, OpCreate, StatusSuccess, "user1", "192.168.1.1"},
		{ProtocolSMB, OpDelete, StatusFailure, "user2", "192.168.1.2"},
		{ProtocolNFS, OpWrite, StatusSuccess, "user1", "192.168.1.1"},
		{ProtocolNFS, OpRead, StatusDenied, "user3", "192.168.1.3"},
	}

	for _, log := range logs {
		entry := &FileAuditEntry{
			Protocol:  log.protocol,
			UserID:    log.user,
			Username:  log.user,
			ClientIP:  log.ip,
			Operation: log.operation,
			FilePath:  "/data/file.txt",
			Status:    log.status,
		}
		logger.Log(context.Background(), entry)
	}

	stats := logger.GetStatistics()

	assert.Equal(t, 5, stats.TotalOperations)
	assert.Equal(t, 3, stats.SuccessCount)
	assert.Equal(t, 1, stats.FailureCount)
	assert.Equal(t, 1, stats.DeniedCount)
	assert.Equal(t, 3, stats.ByProtocol["smb"])
	assert.Equal(t, 2, stats.ByProtocol["nfs"])
	assert.Equal(t, 2, stats.ByOperation["create"])
	assert.Len(t, stats.ByUser, 3)
	assert.Len(t, stats.ByIP, 3)
}

func TestFileAuditLogger_Signature(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.EnableSignatures = true
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	entry := &FileAuditEntry{
		Protocol:  ProtocolSMB,
		UserID:    "user-001",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Operation: OpCreate,
		FilePath:  "/data/test.txt",
		Status:    StatusSuccess,
	}
	logger.Log(context.Background(), entry)

	// 验证签名
	assert.NotEmpty(t, entry.Signature)
	assert.True(t, logger.VerifySignature(entry))

	// 修改内容后验证失败
	entry.FilePath = "/data/modified.txt"
	assert.False(t, logger.VerifySignature(entry))
}

func TestFileAuditLogger_Disable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	// 禁用审计
	logger.Disable()

	entry := &FileAuditEntry{
		Protocol:  ProtocolSMB,
		Operation: OpCreate,
		Status:    StatusSuccess,
	}

	err = logger.Log(context.Background(), entry)
	assert.NoError(t, err)
	assert.Len(t, logger.entries, 0) // 禁用后不应记录

	// 重新启用
	logger.Enable()
	err = logger.Log(context.Background(), entry)
	assert.NoError(t, err)
	assert.Len(t, logger.entries, 1)
}

func TestFileAuditLogger_Export(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)
	defer logger.Stop()

	// 添加日志
	for i := 0; i < 5; i++ {
		entry := &FileAuditEntry{
			Protocol:  ProtocolSMB,
			UserID:    "user-001",
			Username:  "testuser",
			ClientIP:  "192.168.1.100",
			Operation: OpCreate,
			FilePath:  "/data/file.txt",
			Status:    StatusSuccess,
		}
		logger.Log(context.Background(), entry)
	}

	// 测试JSON导出
	now := time.Now()
	opts := FileExportOptions{
		Format:    "json",
		StartTime: now.Add(-time.Hour),
		EndTime:   now.Add(time.Hour),
	}

	data, err := logger.Export(opts)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// 验证JSON格式
	var entries []*FileAuditEntry
	err = json.Unmarshal(data, &entries)
	require.NoError(t, err)
	assert.Len(t, entries, 5)

	// 测试CSV导出
	opts.Format = "csv"
	data, err = logger.Export(opts)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "ID,Timestamp,Protocol")
}

// ========== FileAuditStorage 测试 ==========

func TestNewFileAuditStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewFileAuditStorage(tmpDir, 10, 5, 7, 3)
	require.NoError(t, err)
	require.NotNil(t, storage)

	defer storage.Close()

	assert.Equal(t, tmpDir, storage.basePath)
}

func TestFileAuditStorage_Write(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewFileAuditStorage(tmpDir, 10, 5, 7, 3)
	require.NoError(t, err)
	defer storage.Close()

	entry := &FileAuditEntry{
		ID:        "test-001",
		Timestamp: time.Now(),
		Protocol:  ProtocolSMB,
		UserID:    "user-001",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Operation: OpCreate,
		FilePath:  "/data/test.txt",
		Status:    StatusSuccess,
	}

	err = storage.Write(entry)
	require.NoError(t, err)

	// 刷新缓冲区
	err = storage.FlushBuffer()
	require.NoError(t, err)

	// 验证文件已创建
	today := time.Now().Format("2006-01-02")
	filename := filepath.Join(tmpDir, "file-audit-"+today+".log")
	_, err = os.Stat(filename)
	require.NoError(t, err)
}

func TestFileAuditStorage_Load(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewFileAuditStorage(tmpDir, 10, 5, 7, 3)
	require.NoError(t, err)
	defer storage.Close()

	// 写入日志
	entry := &FileAuditEntry{
		ID:        "test-001",
		Timestamp: time.Now(),
		Protocol:  ProtocolSMB,
		UserID:    "user-001",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Operation: OpCreate,
		FilePath:  "/data/test.txt",
		Status:    StatusSuccess,
	}

	storage.Write(entry)
	storage.FlushBuffer()

	// 加载日志
	today := time.Now().Format("2006-01-02")
	entries, err := storage.Load(today)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "test-001", entries[0].ID)
	assert.Equal(t, "testuser", entries[0].Username)
}

func TestFileAuditStorage_ListAvailableDates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 先创建存储（会创建当日文件）
	storage, err := NewFileAuditStorage(tmpDir, 10, 5, 7, 3)
	require.NoError(t, err)
	storage.Close()

	// 清空目录
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		os.Remove(filepath.Join(tmpDir, e.Name()))
	}

	// 创建几个不同日期的日志文件
	dates := []string{"2026-01-01", "2026-01-02", "2026-01-03"}
	for _, date := range dates {
		filename := filepath.Join(tmpDir, "file-audit-"+date+".log")
		err := os.WriteFile(filename, []byte("{}\n"), 0600)
		require.NoError(t, err)
	}

	// 获取日期列表
	available, err := storage.ListAvailableDates()
	require.NoError(t, err)
	assert.Len(t, available, 3)
	assert.Contains(t, available, "2026-01-01")
	assert.Contains(t, available, "2026-01-02")
	assert.Contains(t, available, "2026-01-03")
}

func TestFileAuditStorage_GetStorageInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 先创建存储
	storage, err := NewFileAuditStorage(tmpDir, 10, 5, 7, 3)
	require.NoError(t, err)
	storage.Close()

	// 清空目录
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		os.Remove(filepath.Join(tmpDir, e.Name()))
	}

	// 创建日志文件
	filename := filepath.Join(tmpDir, "file-audit-2026-01-01.log")
	err = os.WriteFile(filename, []byte("{\"test\": \"data\"}\n"), 0600)
	require.NoError(t, err)

	// 获取存储信息
	info, err := storage.GetStorageInfo()
	require.NoError(t, err)
	assert.Equal(t, 1, info.FileCount)
	assert.Greater(t, info.TotalSize, int64(0))
}

func TestFileAuditStorage_Cleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewFileAuditStorage(tmpDir, 10, 5, 7, 3)
	require.NoError(t, err)
	defer storage.Close()

	// 创建一个过期文件（修改时间设为很久以前）
	oldFile := filepath.Join(tmpDir, "file-audit-2020-01-01.log")
	err = os.WriteFile(oldFile, []byte("old data"), 0600)
	require.NoError(t, err)

	// 设置旧文件的修改时间
	oldTime := time.Now().AddDate(0, 0, -100)
	os.Chtimes(oldFile, oldTime, oldTime)

	// 创建一个新文件
	newFile := filepath.Join(tmpDir, "file-audit-2026-01-01.log")
	err = os.WriteFile(newFile, []byte("new data"), 0600)
	require.NoError(t, err)

	// 执行清理
	err = storage.Cleanup()
	require.NoError(t, err)

	// 验证旧文件被删除
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err))

	// 验证新文件仍然存在
	_, err = os.Stat(newFile)
	assert.NoError(t, err)
}

// ========== FileAuditEntry 测试 ==========

func TestFileAuditEntry_JSON(t *testing.T) {
	now := time.Now()
	entry := &FileAuditEntry{
		ID:          "test-001",
		Timestamp:   now,
		Protocol:    ProtocolSMB,
		ShareName:   "share1",
		SharePath:   "/data/share1",
		UserID:      "user-001",
		Username:    "testuser",
		GroupID:     "group-001",
		GroupName:   "users",
		ClientIP:    "192.168.1.100",
		ClientPort:  12345,
		Operation:   OpCreate,
		Status:      StatusSuccess,
		FilePath:    "/data/share1/test.txt",
		FileName:    "test.txt",
		FileSize:    1024,
		FileMode:    "0644",
		IsDirectory: false,
		SessionID:   "session-001",
		ProcessID:   1234,
		Duration:    50,
		Details:     map[string]interface{}{"key": "value"},
	}

	// 序列化
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	// 反序列化
	var decoded FileAuditEntry
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, entry.ID, decoded.ID)
	assert.Equal(t, entry.Protocol, decoded.Protocol)
	assert.Equal(t, entry.Username, decoded.Username)
	assert.Equal(t, entry.Operation, decoded.Operation)
	assert.Equal(t, entry.FilePath, decoded.FilePath)
}

// ========== FileAuditConfig 测试 ==========

func TestDefaultFileAuditConfig(t *testing.T) {
	config := DefaultFileAuditConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, "/var/log/nas-os/audit/file-operations", config.LogPath)
	assert.Equal(t, 100000, config.MaxMemorySize)
	assert.Equal(t, int64(100), config.MaxFileSize)
	assert.Equal(t, 30, config.MaxFileCount)
	assert.Equal(t, 90, config.MaxAgeDays)
	assert.Equal(t, 7, config.CompressAge)
	assert.True(t, config.EnableSignatures)
	assert.Contains(t, config.ExcludeOperations, OpRead)
	assert.Contains(t, config.ExcludeOperations, OpList)
}

// ========== 集成测试 ==========

func TestIntegration_FullWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit-integration-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建日志记录器
	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	require.NoError(t, err)

	// 模拟SMB操作
	err = logger.LogSMBOperation(
		context.Background(),
		"documents",
		"/shares/documents",
		"user-001",
		"alice",
		"192.168.1.100",
		OpCreate,
		"/shares/documents/report.docx",
		StatusSuccess,
		map[string]interface{}{"size": 10240},
	)
	require.NoError(t, err)

	// 模拟NFS操作
	err = logger.LogNFSOperation(
		context.Background(),
		"/exports/backup",
		"user-002",
		"bob",
		"10.0.0.50",
		OpDelete,
		"/exports/backup/old_backup.tar.gz",
		StatusSuccess,
		nil,
	)
	require.NoError(t, err)

	// 模拟文件重命名
	err = logger.LogFileRename(
		context.Background(),
		ProtocolSMB,
		"documents",
		"/shares/documents",
		"user-001",
		"alice",
		"192.168.1.100",
		"/shares/documents/draft.docx",
		"/shares/documents/final.docx",
		StatusSuccess,
	)
	require.NoError(t, err)

	// 查询所有日志
	result, err := logger.Query(FileAuditQueryOptions{Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)

	// 按用户查询
	result, err = logger.Query(FileAuditQueryOptions{Username: "alice", Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)

	// 获取统计
	stats := logger.GetStatistics()
	assert.Equal(t, 3, stats.TotalOperations)
	assert.Equal(t, 2, stats.ByProtocol["smb"])
	assert.Equal(t, 1, stats.ByProtocol["nfs"])

	// 导出日志
	exportData, err := logger.Export(FileExportOptions{
		Format:    "json",
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, exportData)

	// 停止日志记录器
	logger.Stop()

	// 验证日志文件已写入
	today := time.Now().Format("2006-01-02")
	filename := filepath.Join(tmpDir, "file-audit-"+today+".log")
	_, err = os.Stat(filename)
	require.NoError(t, err)
}

// ========== 基准测试 ==========

func BenchmarkFileAuditLogger_Log(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "audit-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Stop()

	entry := &FileAuditEntry{
		Protocol:  ProtocolSMB,
		UserID:    "user-001",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Operation: OpWrite,
		FilePath:  "/data/test.txt",
		Status:    StatusSuccess,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Log(context.Background(), entry)
	}
}

func BenchmarkFileAuditLogger_Query(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "audit-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultFileAuditConfig()
	config.LogPath = tmpDir
	config.ExcludeOperations = nil

	logger, err := NewFileAuditLogger(config)
	if err != nil {
		b.Fatal(err)
	}
	defer logger.Stop()

	// 预填充日志
	for i := 0; i < 10000; i++ {
		entry := &FileAuditEntry{
			Protocol:  ProtocolSMB,
			UserID:    "user-001",
			Username:  "testuser",
			ClientIP:  "192.168.1.100",
			Operation: OpWrite,
			FilePath:  "/data/test.txt",
			Status:    StatusSuccess,
		}
		logger.Log(context.Background(), entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Query(FileAuditQueryOptions{Limit: 50})
	}
}
