// Package enhanced provides SMB audit tests
package enhanced

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSMAuditConfig 测试审计配置
func TestSMAuditConfig(t *testing.T) {
	tests := []struct {
		name     string
		level    SMAuditLevel
		opType   string
		expected bool
	}{
		{"None级别不记录任何操作", SMAuditLevelNone, "file_read", false},
		{"None级别不记录连接", SMAuditLevelNone, "connect", false},
		{"Minimal级别记录连接", SMAuditLevelMinimal, "connect", true},
		{"Minimal级别不记录文件读取", SMAuditLevelMinimal, "file_read", false},
		{"Standard级别记录文件读取", SMAuditLevelStandard, "file_read", true},
		{"Standard级别记录文件写入", SMAuditLevelStandard, "file_write", true},
		{"Standard级别不记录权限变更", SMAuditLevelStandard, "permission_change", false},
		{"Detailed级别记录权限变更", SMAuditLevelDetailed, "permission_change", true},
		{"Full级别记录所有", SMAuditLevelFull, "file_read", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SMAuditConfig{
				Enabled:             true,
				Level:               tt.level,
				LogFileRead:         true,
				LogFileWrite:        true,
				LogPermissionChange: true,
			}

			result := config.ShouldLog(tt.opType)
			if result != tt.expected {
				t.Errorf("ShouldLog(%s) with level %s: got %v, want %v", tt.opType, tt.level, result, tt.expected)
			}
		})
	}
}

// TestSMAuditManager 测试审计管理器
func TestSMAuditManager(t *testing.T) {
	// 创建临时日志目录
	tmpDir, err := os.MkdirTemp("", "smb-audit-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建审计管理器
	config := DefaultSMAuditConfig()
	config.LogPath = tmpDir
	config.Level = SMAuditLevelStandard

	manager := NewSMAuditManager(config)
	defer manager.Stop()

	// 测试记录连接
	t.Run("记录连接", func(t *testing.T) {
		session := &SMBSession{
			SessionID:       "test-session-1",
			Username:        "testuser",
			Domain:          "WORKGROUP",
			ClientIP:        "192.168.1.100",
			ClientPort:      445,
			ComputerName:    "TESTPC",
			ProtocolVersion: "SMB3",
			ConnectedAt:     time.Now(),
			State:           SessionStateActive,
		}

		manager.LogConnect(session)
		time.Sleep(100 * time.Millisecond) // 等待事件处理

		// 验证统计更新
		stats := manager.GetStatistics()
		if stats.TotalEvents != 1 {
			t.Errorf("期望1个事件，得到 %d", stats.TotalEvents)
		}
	})

	// 测试记录文件操作
	t.Run("记录文件操作", func(t *testing.T) {
		manager.LogFileRead("test-session-1", "share1", "testuser", "192.168.1.100", "/path/to/file.txt", 0, 1024)
		manager.LogFileWrite("test-session-1", "share1", "testuser", "192.168.1.100", "/path/to/file.txt", 0, 512, "abc123")
		manager.LogFileDelete("test-session-1", "share1", "testuser", "192.168.1.100", "/path/to/oldfile.txt", false)

		time.Sleep(100 * time.Millisecond)

		stats := manager.GetStatistics()
		if stats.BytesRead != 1024 {
			t.Errorf("期望读取1024字节，得到 %d", stats.BytesRead)
		}
		if stats.BytesWritten != 512 {
			t.Errorf("期望写入512字节，得到 %d", stats.BytesWritten)
		}
		if stats.FilesDeleted != 1 {
			t.Errorf("期望删除1个文件，得到 %d", stats.FilesDeleted)
		}
	})

	// 测试查询
	t.Run("查询事件", func(t *testing.T) {
		opts := SMAuditQueryOptions{
			Username: "testuser",
			Limit:    100,
		}

		events, total := manager.QueryEvents(opts)
		if total < 3 {
			t.Errorf("期望至少3个事件，得到 %d", total)
		}
		if len(events) == 0 {
			t.Error("期望返回事件列表，但为空")
		}
	})

	// 测试断开连接
	t.Run("记录断开", func(t *testing.T) {
		manager.LogDisconnect("test-session-1", "testuser", "192.168.1.100", 1024, 512)
		time.Sleep(100 * time.Millisecond)

		stats := manager.GetStatistics()
		// 检查断开事件是否记录
		if stats.EventsByType["disconnect"] != 1 {
			t.Errorf("期望1个断开事件，得到 %d", stats.EventsByType["disconnect"])
		}
	})
}

// TestSMAuditHook 测试审计钩子
func TestSMAuditHook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "smb-audit-hook-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultSMAuditConfig()
	config.LogPath = tmpDir
	config.Level = SMAuditLevelStandard

	manager := NewSMAuditManager(config)
	defer manager.Stop()

	hook := NewSMAuditHook(manager)

	// 测试会话钩子
	t.Run("会话钩子", func(t *testing.T) {
		hook.OnSessionConnect("session-1", "user1", "10.0.0.1", "SMB3", "PC1")

		info := hook.GetSessionInfo("session-1")
		if info == nil {
			t.Fatal("会话信息不应为空")
		}
		if info.Username != "user1" {
			t.Errorf("期望用户名 user1，得到 %s", info.Username)
		}

		hook.OnSessionDisconnect("session-1")
		info = hook.GetSessionInfo("session-1")
		if info != nil {
			t.Error("断开后会话信息应为空")
		}
	})

	// 测试文件操作钩子
	t.Run("文件操作钩子", func(t *testing.T) {
		hook.OnSessionConnect("session-2", "user2", "10.0.0.2", "SMB3", "PC2")

		hook.OnFileOpen("session-2", "share1", "/test/file.txt", "read-write", false)
		openFiles := hook.GetOpenFiles("session-2")
		if len(openFiles) != 1 {
			t.Errorf("期望1个打开文件，得到 %d", len(openFiles))
		}

		hook.OnFileRead("session-2", "share1", "/test/file.txt", 0, 1024)
		hook.OnFileWrite("session-2", "share1", "/test/file.txt", 1024, 512, "hash123")

		hook.OnFileClose("session-2", "share1", "/test/file.txt", 1024, 512)

		info := hook.GetSessionInfo("session-2")
		if info.BytesRead != 1024 {
			t.Errorf("期望读取1024字节，得到 %d", info.BytesRead)
		}
		if info.BytesWritten != 512 {
			t.Errorf("期望写入512字节，得到 %d", info.BytesWritten)
		}

		// 文件应该已关闭
		openFiles = hook.GetOpenFiles("session-2")
		if len(openFiles) != 0 {
			t.Errorf("期望0个打开文件，得到 %d", len(openFiles))
		}

		hook.OnSessionDisconnect("session-2")
	})
}

// TestSMAuditExclusion 测试排除规则
func TestSMAuditExclusion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "smb-audit-exclude-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultSMAuditConfig()
	config.LogPath = tmpDir
	config.Level = SMAuditLevelStandard
	config.ExcludeShares = []string{"IPC$", "admin$"}
	config.ExcludeUsers = []string{"SYSTEM"}
	config.ExcludePaths = []string{"/tmp", "/var/tmp"}

	manager := NewSMAuditManager(config)
	defer manager.Stop()

	// 测试排除的共享
	t.Run("排除共享", func(t *testing.T) {
		manager.LogFileRead("s1", "IPC$", "user1", "10.0.0.1", "/path/file", 0, 100)
		time.Sleep(50 * time.Millisecond)

		stats := manager.GetStatistics()
		if stats.TotalEvents != 0 {
			t.Error("IPC$ 共享的操作应该被排除")
		}
	})

	// 测试排除的用户
	t.Run("排除用户", func(t *testing.T) {
		manager.LogFileRead("s2", "share1", "SYSTEM", "10.0.0.1", "/path/file", 0, 100)
		time.Sleep(50 * time.Millisecond)

		stats := manager.GetStatistics()
		initialTotal := stats.TotalEvents

		manager.LogFileRead("s3", "share1", "normaluser", "10.0.0.1", "/path/file", 0, 100)
		time.Sleep(50 * time.Millisecond)

		stats = manager.GetStatistics()
		if stats.TotalEvents != initialTotal+1 {
			t.Error("SYSTEM用户的操作应该被排除")
		}
	})

	// 测试排除的路径
	t.Run("排除路径", func(t *testing.T) {
		manager.LogFileRead("s4", "share1", "user1", "10.0.0.1", "/tmp/file", 0, 100)
		manager.LogFileRead("s5", "share1", "user1", "10.0.0.1", "/home/user/file", 0, 100)
		time.Sleep(50 * time.Millisecond)

		// /tmp 路径应该被排除，/home 路径应该记录
		opts := SMAuditQueryOptions{FilePath: "/home/user/file"}
		_, total := manager.QueryEvents(opts)
		if total < 1 {
			t.Error("/home 路径的操作应该被记录")
		}
	})
}

// TestSMAuditLevelConfig 测试审计级别配置
func TestSMAuditLevelConfig(t *testing.T) {
	config := SMAuditConfig{
		Enabled:             true,
		Level:               SMAuditLevelDetailed,
		LogFileRead:         false, // 显式禁用
		LogFileWrite:        true,
		LogPermissionChange: true,
	}

	// 即使级别足够，但如果显式禁用了某操作，也不应该记录
	if config.ShouldLog("file_read") {
		t.Error("文件读取被显式禁用，不应该记录")
	}

	if !config.ShouldLog("file_write") {
		t.Error("文件写入应该被记录")
	}

	if !config.ShouldLog("permission_change") {
		t.Error("权限变更在Detailed级别应该被记录")
	}
}

// TestSMAuditLogRotation 测试日志轮转
func TestSMAuditLogRotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "smb-audit-rotate-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultSMAuditConfig()
	config.LogPath = tmpDir
	config.MaxLogSizeMB = 1 // 1MB
	config.MaxLogAgeDays = 1

	manager := NewSMAuditManager(config)
	defer manager.Stop()

	// 生成大量事件
	for i := 0; i < 100; i++ {
		manager.LogFileRead("session-1", "share1", "user1", "10.0.0.1", "/path/file.txt", 0, 1024)
	}
	time.Sleep(200 * time.Millisecond)

	// 检查日志文件是否创建
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("读取日志目录失败: %v", err)
	}

	found := false
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".log" {
			found = true
			// 验证文件内容是有效的JSON
			data, err := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
			if err != nil {
				t.Errorf("读取日志文件失败: %v", err)
				continue
			}

			// 检查是否是有效的JSON行
			lines := 0
			for _, line := range splitLines(string(data)) {
				if line == "" {
					continue
				}
				var entry SMAuditEntry
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Errorf("无效的JSON行: %v", err)
				}
				lines++
			}
			t.Logf("日志文件 %s 包含 %d 条记录", entry.Name(), lines)
		}
	}

	if !found {
		t.Error("未找到日志文件")
	}
}

// splitLines 分割字符串为行
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// BenchmarkSMAudit 性能基准测试
func BenchmarkSMAudit(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "smb-audit-bench-*")
	if err != nil {
		b.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultSMAuditConfig()
	config.LogPath = tmpDir
	config.Level = SMAuditLevelStandard

	manager := NewSMAuditManager(config)
	defer manager.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.LogFileRead("session-1", "share1", "user1", "10.0.0.1", "/path/file.txt", 0, 1024)
	}
}

// BenchmarkSMAuditParallel 并发性能测试
func BenchmarkSMAuditParallel(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "smb-audit-bench-*")
	if err != nil {
		b.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultSMAuditConfig()
	config.LogPath = tmpDir
	config.Level = SMAuditLevelStandard

	manager := NewSMAuditManager(config)
	defer manager.Stop()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sessionID := fmt.Sprintf("session-%d", i%100)
			manager.LogFileRead(sessionID, "share1", "user1", "10.0.0.1", "/path/file.txt", 0, 1024)
			i++
		}
	})
}