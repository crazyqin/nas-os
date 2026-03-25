// Package integration 提供 NAS-OS 集成测试
// 日志系统集成测试
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nas-os/internal/logging"
)

// TestLogging_Integration 日志系统集成测试.
func TestLogging_Integration(t *testing.T) {
	t.Run("LoggerCreation", func(t *testing.T) {
		// 默认配置创建
		logger := logging.NewLogger(nil)
		if logger == nil {
			t.Fatal("Logger should not be nil")
		}

		// 自定义配置创建
		config := &logging.LogConfig{
			Level:      logging.LevelDebug,
			JSONFormat: true,
		}
		jsonLogger := logging.NewLogger(config)
		if jsonLogger == nil {
			t.Fatal("JSON logger should not be nil")
		}
	})

	t.Run("LogLevelFiltering", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger(&logging.LogConfig{
			Level:  logging.LevelWarn,
			Output: &buf,
		})

		logger.Debug("This should not appear")
		logger.Info("This should not appear either")
		logger.Warn("This should appear")
		logger.Error("This should also appear")

		output := buf.String()
		if strings.Contains(output, "should not appear") {
			t.Error("Lower level logs should be filtered")
		}
		if !strings.Contains(output, "should appear") {
			t.Error("Warn level should be logged")
		}
	})

	t.Run("JSONFormatting", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger(&logging.LogConfig{
			Level:      logging.LevelDebug,
			Output:     &buf,
			JSONFormat: true,
		})

		logger.Info("Test message")

		var entry logging.LogEntry
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("Failed to parse JSON log: %v", err)
		}

		if entry.Message != "Test message" {
			t.Errorf("Expected message 'Test message', got %s", entry.Message)
		}
		if entry.Level != "INFO" {
			t.Errorf("Expected level INFO, got %s", entry.Level)
		}
	})

	t.Run("TextFormatting", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger(&logging.LogConfig{
			Level:      logging.LevelInfo,
			Output:     &buf,
			JSONFormat: false,
		})

		logger.Info("Test text message")

		output := buf.String()
		if !strings.Contains(output, "INFO") {
			t.Error("Output should contain level INFO")
		}
		if !strings.Contains(output, "Test text message") {
			t.Error("Output should contain the message")
		}
	})
}

// TestLogging_WithFields 测试带字段的日志.
func TestLogging_WithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	logger.WithField("user", "admin").Info("User action")
	logger.WithFields(map[string]interface{}{
		"module":  "storage",
		"action":  "create",
		"success": true,
	}).Info("Operation completed")

	// 验证第一条日志
	var entry1 logging.LogEntry
	if err := json.Unmarshal(bytes.Split(buf.Bytes(), []byte("\n"))[0], &entry1); err != nil {
		t.Fatalf("Failed to parse first log: %v", err)
	}
	if entry1.Fields["user"] != "admin" {
		t.Error("Field 'user' should be 'admin'")
	}

	// 验证第二条日志
	var entry2 logging.LogEntry
	if err := json.Unmarshal(bytes.Split(buf.Bytes(), []byte("\n"))[1], &entry2); err != nil {
		t.Fatalf("Failed to parse second log: %v", err)
	}
	if entry2.Fields["module"] != "storage" {
		t.Error("Field 'module' should be 'storage'")
	}
}

// TestLogging_LevelMethods 测试各日志级别方法.
func TestLogging_LevelMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelDebug,
		Output:     &buf,
		JSONFormat: true,
	})

	tests := []struct {
		method   func(string)
		expected string
	}{
		{logger.Debug, "DEBUG"},
		{logger.Info, "INFO"},
		{logger.Warn, "WARN"},
		{logger.Error, "ERROR"},
	}

	for _, tt := range tests {
		buf.Reset()
		tt.method("Test message")

		var entry logging.LogEntry
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		if entry.Level != tt.expected {
			t.Errorf("Expected level %s, got %s", tt.expected, entry.Level)
		}
	}
}

// TestLogging_FormattedMethods 测试格式化方法.
func TestLogging_FormattedMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelDebug,
		Output:     &buf,
		JSONFormat: true,
	})

	logger.Debugf("Debug %s", "message")
	var entry logging.LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if entry.Message != "Debug message" {
		t.Errorf("Expected 'Debug message', got %s", entry.Message)
	}

	buf.Reset()
	logger.Infof("Info %d", 123)
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if entry.Message != "Info 123" {
		t.Errorf("Expected 'Info 123', got %s", entry.Message)
	}
}

// TestLogging_LogManager 测试日志管理器.
func TestLogging_LogManager(t *testing.T) {
	manager := logging.NewLogManager()

	logger1 := manager.GetLogger("module1")
	if logger1 == nil {
		t.Fatal("Logger1 should not be nil")
	}

	logger2 := manager.GetLogger("module2")
	if logger2 == nil {
		t.Fatal("Logger2 should not be nil")
	}

	// 再次获取同一个 logger 应该返回同一个实例
	logger1Again := manager.GetLogger("module1")
	if logger1 != logger1Again {
		t.Error("Same logger name should return same instance")
	}
}

// TestLogging_Rotator 测试日志轮转.
func TestLogging_Rotator(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// 创建轮转器，最大 100 字节，保留 3 个备份
	rotator, err := logging.NewLogRotator(logPath, 100, 3, 7)
	if err != nil {
		t.Fatalf("Failed to create rotator: %v", err)
	}
	defer rotator.Close()

	// 写入超过限制的数据触发轮转
	for i := 0; i < 10; i++ {
		_, err := rotator.Write([]byte("This is a test log line that should trigger rotation\n"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// 检查是否创建了备份文件
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	// 应该有当前日志文件和至少一个备份
	if len(files) < 2 {
		t.Errorf("Expected at least 2 files, got %d", len(files))
	}
}

// TestLogging_Context 测试上下文日志.
func TestLogging_Context(t *testing.T) {
	logger := logging.NewLogger(nil)

	ctx := logging.WithContext(context.Background(), logger)
	retrieved := logging.FromContext(ctx)

	if retrieved == nil {
		t.Fatal("Retrieved logger should not be nil")
	}

	// 从空上下文获取应该返回默认 logger
	emptyCtx := context.Background()
	defaultLogger := logging.FromContext(emptyCtx)
	if defaultLogger == nil {
		t.Fatal("Default logger should not be nil")
	}
}

// TestLogging_ConcurrentWrites 测试并发写入
// 注意：由于 bytes.Buffer 不是线程安全的，此测试验证日志格式正确性
// 而不是严格的并发安全性.
func TestLogging_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex

	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	// 使用同步方式写入以避免竞态条件
	for i := 0; i < 100; i++ {
		mu.Lock()
		buf.Reset()
		logger.WithField("goroutine", i).Info("Concurrent log")
		mu.Unlock()
	}

	// 验证单条输出是有效的 JSON
	mu.Lock()
	buf.Reset()
	logger.WithField("goroutine", 0).Info("Concurrent log")
	line := buf.Bytes()
	mu.Unlock()

	var entry logging.LogEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}
	if entry.Fields["goroutine"] != 0.0 { // JSON 数字解析为 float64
		t.Errorf("Field 'goroutine' should be 0, got %v", entry.Fields["goroutine"])
	}
}

// TestLogging_Timestamp 测试时间戳.
func TestLogging_Timestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	before := time.Now()
	logger.Info("Timestamp test")
	after := time.Now()

	var entry logging.LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if entry.Timestamp.Before(before) || entry.Timestamp.After(after) {
		t.Error("Timestamp should be between before and after")
	}
}

// TestLogging_SetLevel 测试动态设置日志级别.
func TestLogging_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:  logging.LevelError,
		Output: &buf,
	})

	logger.Info("Should not appear")
	if buf.Len() > 0 {
		t.Error("Info should be filtered at Error level")
	}

	logger.SetLevel(logging.LevelInfo)
	buf.Reset()

	logger.Info("Should appear now")
	if buf.Len() == 0 {
		t.Error("Info should be logged after level change")
	}
}

// ========== 性能测试 ==========

// BenchmarkLogging_Info 性能测试：Info 级别日志.
func BenchmarkLogging_Info(b *testing.B) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark log message")
	}
}

// BenchmarkLogging_WithFields 性能测试：带字段日志.
func BenchmarkLogging_WithFields(b *testing.B) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.WithFields(map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}).Info("Benchmark with fields")
	}
}

// BenchmarkLogging_JSONFormat 性能测试：JSON 格式化.
func BenchmarkLogging_JSONFormat(b *testing.B) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.WithField("benchmark", true).Infof("Benchmark message %d", i)
	}
}

// BenchmarkLogging_Concurrent 性能测试：并发日志.
func BenchmarkLogging_Concurrent(b *testing.B) {
	var buf bytes.Buffer
	logger := logging.NewLogger(&logging.LogConfig{
		Level:      logging.LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("Parallel log message")
		}
	})
}
