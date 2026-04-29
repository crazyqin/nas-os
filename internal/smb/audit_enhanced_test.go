package smb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

// ========== EnhancedAuditLogger 基础测试 ==========

func TestNewEnhancedAuditLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	config := EnhancedAuditLoggerConfig{
		FilePath:     logPath,
		AutoFlush:    true,
		FlushEntries: 10,
	}

	al, err := NewEnhancedAuditLogger(config, logger)
	if err != nil {
		t.Fatalf("创建 EnhancedAuditLogger 失败: %v", err)
	}
	defer al.Close()

	if al == nil {
		t.Fatal("EnhancedAuditLogger 不应为 nil")
	}
	if al.filePath != logPath {
		t.Errorf("文件路径错误: got %s, want %s", al.filePath, logPath)
	}
	if !al.autoFlush {
		t.Error("AutoFlush 应为 true")
	}
	if al.flushEntries != 10 {
		t.Errorf("FlushEntries 错误: got %d, want 10", al.flushEntries)
	}
}

func TestNewEnhancedAuditLogger_EmptyPath(t *testing.T) {
	logger := zap.NewNop().Sugar()
	config := EnhancedAuditLoggerConfig{
		FilePath: "",
	}

	_, err := NewEnhancedAuditLogger(config, logger)
	if err == nil {
		t.Error("空路径应该返回错误")
	}
}

func TestNewEnhancedAuditLogger_NilLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	config := EnhancedAuditLoggerConfig{
		FilePath:     logPath,
		AutoFlush:    false,
		FlushEntries: 10,
	}

	al, err := NewEnhancedAuditLogger(config, nil)
	if err != nil {
		t.Fatalf("nil logger 应该使用 nop logger: %v", err)
	}
	defer al.Close()
}

func TestNewEnhancedAuditLogger_DefaultFlushEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	config := EnhancedAuditLoggerConfig{
		FilePath: logPath,
	}

	al, err := NewEnhancedAuditLogger(config, logger)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	defer al.Close()

	if al.flushEntries != 100 {
		t.Errorf("默认 FlushEntries 应为 100，实际: %d", al.flushEntries)
	}
}

// ========== 日志记录测试 ==========

func TestEnhancedAuditLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	entry := EnhancedAuditEntry{
		Operation: OpFileCreate,
		IP:        "192.168.1.100",
		Username:  "admin",
		ShareName: "documents",
		FilePath:  "/share/test.txt",
		Result:    "success",
	}

	al.Log(entry)

	if al.TotalEntries() != 1 {
		t.Errorf("条目数错误: got %d, want 1", al.TotalEntries())
	}

	// 验证自动填充的字段
	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("查询结果应为 1 条")
	}
	if entries[0].ID == "" {
		t.Error("ID 不应为空")
	}
	if entries[0].Timestamp.IsZero() {
		t.Error("Timestamp 不应为零值")
	}
}

func TestEnhancedAuditLogger_Log_AutoID(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	entry := EnhancedAuditEntry{
		ID:        "custom-id-123",
		Operation: OpFileRead,
		IP:        "10.0.0.1",
		Result:    "success",
	}

	al.Log(entry)

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("查询结果应为 1 条")
	}
	if entries[0].ID != "custom-id-123" {
		t.Errorf("自定义 ID 未保留: got %s", entries[0].ID)
	}
}

func TestEnhancedAuditLogger_AutoFlush(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{
		FilePath:     logPath,
		AutoFlush:    true,
		FlushEntries: 3,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	// 写入3条触发自动刷新
	for i := 0; i < 3; i++ {
		al.Log(EnhancedAuditEntry{
			Operation: OpFileCreate,
			IP:        "192.168.1.100",
			Result:    "success",
		})
	}

	// 验证文件已写入
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if len(data) == 0 {
		t.Error("自动刷新后文件不应为空")
	}
}

// ========== 便捷记录方法测试 ==========

func TestEnhancedAuditLogger_LogFileOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogFileOperation(OpFileCreate, "192.168.1.1", "admin", "docs", "/share/new.txt", "success",
		WithFileSize(1024),
		WithDetails("创建新文件"),
	)

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpFileCreate {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
	if e.FileSize != 1024 {
		t.Errorf("文件大小错误: %d", e.FileSize)
	}
	if e.Details != "创建新文件" {
		t.Errorf("详情错误: %s", e.Details)
	}
}

func TestEnhancedAuditLogger_LogFileRename(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogFileRename("192.168.1.1", "admin", "docs", "/share/old.txt", "/share/new.txt", "success")

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpFileRename {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
	if e.OldPath != "/share/old.txt" {
		t.Errorf("旧路径错误: %s", e.OldPath)
	}
	if e.NewPath != "/share/new.txt" {
		t.Errorf("新路径错误: %s", e.NewPath)
	}
}

func TestEnhancedAuditLogger_LogDirOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogDirOperation(OpDirCreate, "192.168.1.1", "admin", "docs", "/share/newdir", "success")

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpDirCreate {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
	if !e.IsDirectory {
		t.Error("IsDirectory 应为 true")
	}
}

func TestEnhancedAuditLogger_LogPermissionChange(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogPermissionChange("192.168.1.1", "admin", "docs", "/share/test.txt", "read", "read_write")

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpPermissionChange {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
	if e.OldPermission != "read" {
		t.Errorf("旧权限错误: %s", e.OldPermission)
	}
	if e.NewPermission != "read_write" {
		t.Errorf("新权限错误: %s", e.NewPermission)
	}
}

func TestEnhancedAuditLogger_LogPermissionGrant(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogPermissionGrant("192.168.1.1", "admin", "docs", "user1", "read_write")

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpPermissionGrant {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
}

func TestEnhancedAuditLogger_LogPermissionRevoke(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogPermissionRevoke("192.168.1.1", "admin", "docs", "user1", "read_write")

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpPermissionRevoke {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
}

func TestEnhancedAuditLogger_LogConnection(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogConnection("192.168.1.100", "admin", "DESKTOP-PC", "SMB3", "documents")

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpConnection {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
	if e.ClientName != "DESKTOP-PC" {
		t.Errorf("客户端名错误: %s", e.ClientName)
	}
	if e.Protocol != "SMB3" {
		t.Errorf("协议错误: %s", e.Protocol)
	}
}

func TestEnhancedAuditLogger_LogDisconnection(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.LogDisconnection("192.168.1.100", "admin", "DESKTOP-PC", "SMB3", "documents", "正常断开")

	entries := al.Query(AuditQueryFilter{})
	if len(entries) != 1 {
		t.Fatal("应有 1 条记录")
	}
	e := entries[0]
	if e.Operation != OpDisconnection {
		t.Errorf("操作类型错误: %s", e.Operation)
	}
	if e.Details != "正常断开" {
		t.Errorf("详情错误: %s", e.Details)
	}
}

// ========== 查询测试 ==========

func TestEnhancedAuditLogger_Query_ByTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	// 写入不同时间的条目
	now := time.Now()
	for i := 0; i < 5; i++ {
		al.Log(EnhancedAuditEntry{
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Operation: OpFileCreate,
			IP:        "192.168.1.100",
			Result:    "success",
		})
	}

	// 查询第2-4小时的条目
	startTime := now.Add(1 * time.Hour)
	endTime := now.Add(3*time.Hour + 30*time.Minute)
	entries := al.Query(AuditQueryFilter{
		StartTime: &startTime,
		EndTime:   &endTime,
	})

	if len(entries) != 3 {
		t.Errorf("时间范围查询结果错误: got %d, want 3", len(entries))
	}
}

func TestEnhancedAuditLogger_Query_ByUsername(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "admin", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "user1", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileDelete, Username: "admin", Result: "success"})

	entries := al.Query(AuditQueryFilter{Username: "admin"})
	if len(entries) != 2 {
		t.Errorf("用户过滤结果错误: got %d, want 2", len(entries))
	}
	for _, e := range entries {
		if e.Username != "admin" {
			t.Errorf("返回了非 admin 用户的记录: %s", e.Username)
		}
	}
}

func TestEnhancedAuditLogger_Query_ByOperations(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileDelete, Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileRename, Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpConnection, Result: "success"})

	entries := al.Query(AuditQueryFilter{
		Operations: []AuditOperationType{OpFileCreate, OpFileDelete},
	})
	if len(entries) != 2 {
		t.Errorf("操作类型过滤结果错误: got %d, want 2", len(entries))
	}
}

func TestEnhancedAuditLogger_Query_ByResult(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Result: "denied"})
	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Result: "error"})

	entries := al.Query(AuditQueryFilter{Result: "denied"})
	if len(entries) != 1 {
		t.Errorf("结果过滤错误: got %d, want 1", len(entries))
	}
}

func TestEnhancedAuditLogger_Query_ByShareName(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, ShareName: "docs", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, ShareName: "media", Result: "success"})

	entries := al.Query(AuditQueryFilter{ShareName: "docs"})
	if len(entries) != 1 {
		t.Errorf("共享名过滤错误: got %d, want 1", len(entries))
	}
}

func TestEnhancedAuditLogger_Query_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	for i := 0; i < 10; i++ {
		al.Log(EnhancedAuditEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Operation: OpFileCreate,
			Result:    "success",
		})
	}

	// 查询前3条
	entries := al.Query(AuditQueryFilter{Limit: 3})
	if len(entries) != 3 {
		t.Errorf("Limit 测试失败: got %d, want 3", len(entries))
	}

	// 跳过前3条，再取3条
	entries = al.Query(AuditQueryFilter{Limit: 3, Offset: 3})
	if len(entries) != 3 {
		t.Errorf("Offset 测试失败: got %d, want 3", len(entries))
	}
}

func TestEnhancedAuditLogger_Query_CombinedFilter(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "admin", ShareName: "docs", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "admin", ShareName: "media", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileDelete, Username: "user1", ShareName: "docs", Result: "success"})

	entries := al.Query(AuditQueryFilter{
		Username:  "admin",
		ShareName: "docs",
	})
	if len(entries) != 1 {
		t.Errorf("组合过滤错误: got %d, want 1", len(entries))
	}
}

// ========== 统计测试 ==========

func TestEnhancedAuditLogger_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "admin", ShareName: "docs", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileDelete, Username: "admin", ShareName: "docs", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "user1", ShareName: "media", Result: "denied"})

	stats := al.GetStats()

	if stats.TotalEntries != 3 {
		t.Errorf("总条目数错误: %d", stats.TotalEntries)
	}
	if stats.ByOperation[string(OpFileCreate)] != 2 {
		t.Errorf("OpFileCreate 统计错误: %d", stats.ByOperation[string(OpFileCreate)])
	}
	if stats.ByResult["success"] != 2 {
		t.Errorf("success 统计错误: %d", stats.ByResult["success"])
	}
	if stats.ByResult["denied"] != 1 {
		t.Errorf("denied 统计错误: %d", stats.ByResult["denied"])
	}
	if stats.ByUser["admin"] != 2 {
		t.Errorf("admin 统计错误: %d", stats.ByUser["admin"])
	}
	if stats.ByShare["docs"] != 2 {
		t.Errorf("docs 统计错误: %d", stats.ByShare["docs"])
	}
	if stats.TimeRange == nil {
		t.Error("TimeRange 不应为 nil")
	}
}

func TestEnhancedAuditLogger_GetStats_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	stats := al.GetStats()

	if stats.TotalEntries != 0 {
		t.Errorf("空日志总条目应为 0: %d", stats.TotalEntries)
	}
	if stats.TimeRange != nil {
		t.Error("空日志 TimeRange 应为 nil")
	}
}

// ========== 持久化测试 ==========

func TestEnhancedAuditLogger_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()

	// 创建并写入日志
	al1, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}

	al1.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "admin", Result: "success"})
	al1.Log(EnhancedAuditEntry{Operation: OpFileDelete, Username: "admin", Result: "success"})

	if err := al1.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	// 重新加载并验证
	al2, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al2.Close()

	if al2.TotalEntries() != 2 {
		t.Errorf("持久化后条目数错误: got %d, want 2", al2.TotalEntries())
	}
}

func TestEnhancedAuditLogger_Flush(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Result: "success"})

	// 手动刷新
	if err := al.Flush(); err != nil {
		t.Fatalf("Flush 失败: %v", err)
	}

	// 验证文件存在
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if len(data) == 0 {
		t.Error("Flush 后文件不应为空")
	}

	// 验证JSON格式
	var entries []EnhancedAuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("JSON 条目数错误: %d", len(entries))
	}
}

// ========== 导出测试 ==========

func TestEnhancedAuditLogger_ExportJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Username: "admin", Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileDelete, Username: "user1", Result: "success"})

	// 导出全部
	data, err := al.ExportJSON(AuditQueryFilter{})
	if err != nil {
		t.Fatalf("ExportJSON 失败: %v", err)
	}

	var entries []EnhancedAuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("导出条目数错误: %d", len(entries))
	}

	// 带过滤导出
	data, err = al.ExportJSON(AuditQueryFilter{Username: "admin"})
	if err != nil {
		t.Fatalf("ExportJSON 过滤失败: %v", err)
	}

	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("过滤导出条目数错误: %d", len(entries))
	}
}

// ========== 辅助方法测试 ==========

func TestEnhancedAuditLogger_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	al.Log(EnhancedAuditEntry{Operation: OpFileCreate, Result: "success"})
	al.Log(EnhancedAuditEntry{Operation: OpFileDelete, Result: "success"})

	if al.TotalEntries() != 2 {
		t.Fatalf("Clear 前条目数错误: %d", al.TotalEntries())
	}

	al.Clear()

	if al.TotalEntries() != 0 {
		t.Errorf("Clear 后条目数应为 0: %d", al.TotalEntries())
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" || id2 == "" {
		t.Error("GenerateID 不应返回空字符串")
	}
	// 连续调用一般不会相同
	if id1 == id2 {
		t.Log("连续 GenerateID 相同（极小概率），不视为错误")
	}
}

// ========== 选项函数测试 ==========

func TestWithFileSize(t *testing.T) {
	entry := &EnhancedAuditEntry{}
	WithFileSize(2048)(entry)
	if entry.FileSize != 2048 {
		t.Errorf("FileSize 错误: %d", entry.FileSize)
	}
}

func TestWithDetails(t *testing.T) {
	entry := &EnhancedAuditEntry{}
	WithDetails("测试详情")(entry)
	if entry.Details != "测试详情" {
		t.Errorf("Details 错误: %s", entry.Details)
	}
}

func TestWithError(t *testing.T) {
	entry := &EnhancedAuditEntry{}
	WithError("权限不足")(entry)
	if entry.ErrorMsg != "权限不足" {
		t.Errorf("ErrorMsg 错误: %s", entry.ErrorMsg)
	}
	if entry.Result != "error" {
		t.Errorf("Result 错误: %s", entry.Result)
	}
}

func TestWithIsDirectory(t *testing.T) {
	entry := &EnhancedAuditEntry{}
	WithIsDirectory(true)(entry)
	if !entry.IsDirectory {
		t.Error("IsDirectory 应为 true")
	}
}

// ========== JSON序列化测试 ==========

func TestEnhancedAuditEntry_JSON(t *testing.T) {
	entry := EnhancedAuditEntry{
		ID:            "test-001",
		Timestamp:     time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC),
		Operation:     OpFileCreate,
		IP:            "192.168.1.100",
		Username:      "admin",
		ShareName:     "documents",
		FilePath:      "/share/test.txt",
		Result:        "success",
		Details:       "创建新文件",
		FileSize:      1024,
		IsDirectory:   false,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}

	var decoded EnhancedAuditEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON 反序列化失败: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("ID 不匹配: %s != %s", decoded.ID, entry.ID)
	}
	if decoded.Operation != entry.Operation {
		t.Errorf("Operation 不匹配: %s != %s", decoded.Operation, entry.Operation)
	}
	if decoded.FileSize != entry.FileSize {
		t.Errorf("FileSize 不匹配: %d != %d", decoded.FileSize, entry.FileSize)
	}
}

// ========== 运行完整场景测试 ==========

func TestFullAuditScenario(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit_enhanced.json")

	logger := zap.NewNop().Sugar()
	al, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{
		FilePath:     logPath,
		AutoFlush:    true,
		FlushEntries: 10,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()

	// 模拟完整的SMB操作审计流程

	// 1. 用户连接
	al.LogConnection("192.168.1.100", "admin", "WORKSTATION-01", "SMB3", "documents")

	// 2. 创建文件
	al.LogFileOperation(OpFileCreate, "192.168.1.100", "admin", "documents", "/share/report.docx", "success",
		WithFileSize(50240),
	)

	// 3. 修改文件
	al.LogFileOperation(OpFileModify, "192.168.1.100", "admin", "documents", "/share/report.docx", "success",
		WithFileSize(51200),
	)

	// 4. 创建目录
	al.LogDirOperation(OpDirCreate, "192.168.1.100", "admin", "documents", "/share/archive", "success")

	// 5. 重命名文件
	al.LogFileRename("192.168.1.100", "admin", "documents", "/share/report.docx", "/share/final_report.docx", "success")

	// 6. 权限变更
	al.LogPermissionChange("192.168.1.100", "admin", "documents", "/share/final_report.docx", "read", "read_write")

	// 7. 授权其他用户
	al.LogPermissionGrant("192.168.1.100", "admin", "documents", "user2", "read")

	// 8. 删除文件
	al.LogFileOperation(OpFileDelete, "192.168.1.100", "admin", "documents", "/share/old_file.txt", "success")

	// 9. 失败的访问尝试
	al.Log(EnhancedAuditEntry{
		Operation: OpFileRead,
		IP:        "10.0.0.50",
		Username:  "unknown",
		ShareName: "documents",
		FilePath:  "/share/secret.txt",
		Result:    "denied",
		ErrorMsg:  "权限不足",
	})

	// 10. 断开连接
	al.LogDisconnection("192.168.1.100", "admin", "WORKSTATION-01", "SMB3", "documents", "正常断开")

	// 验证总条目
	if al.TotalEntries() != 10 {
		t.Errorf("总条目数错误: %d, want 10", al.TotalEntries())
	}

	// 验证统计
	stats := al.GetStats()
	if stats.TotalEntries != 10 {
		t.Errorf("统计总条目错误: %d", stats.TotalEntries)
	}
	if stats.ByResult["success"] != 9 {
		t.Errorf("成功操作统计错误: %d", stats.ByResult["success"])
	}
	if stats.ByResult["denied"] != 1 {
		t.Errorf("拒绝操作统计错误: %d", stats.ByResult["denied"])
	}

	// 按用户查询
	adminEntries := al.Query(AuditQueryFilter{Username: "admin"})
	if len(adminEntries) != 9 {
		t.Errorf("admin 用户条目错误: %d", len(adminEntries))
	}

	// 按操作类型查询
	fileOps := al.Query(AuditQueryFilter{
		Operations: []AuditOperationType{OpFileCreate, OpFileModify, OpFileDelete},
	})
	if len(fileOps) != 3 {
		t.Errorf("文件操作条目错误: %d", len(fileOps))
	}

	// 导出JSON
	jsonData, err := al.ExportJSON(AuditQueryFilter{Username: "admin"})
	if err != nil {
		t.Fatalf("导出JSON失败: %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("导出JSON不应为空")
	}

	// 持久化并重新加载
	if err := al.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	al2, err := NewEnhancedAuditLogger(EnhancedAuditLoggerConfig{FilePath: logPath}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer al2.Close()

	if al2.TotalEntries() != 10 {
		t.Errorf("重新加载后条目数错误: %d", al2.TotalEntries())
	}
}
