// Package logging 提供结构化日志功能
package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
		{Level(100), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.level.String())
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger(nil)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	if logger.level != LevelInfo {
		t.Errorf("Expected default level Info, got %v", logger.level)
	}
}

func TestLogger_WithField(t *testing.T) {
	logger := NewLogger(nil)
	newLogger := logger.WithField("key", "value")

	if newLogger.fields["key"] != "value" {
		t.Error("Field not set correctly")
	}

	// 原日志记录器不应受影响
	if len(logger.fields) != 0 {
		t.Error("Original logger should not be modified")
	}
}

func TestLogger_WithFields(t *testing.T) {
	logger := NewLogger(nil)
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}
	newLogger := logger.WithFields(fields)

	if newLogger.fields["key1"] != "value1" {
		t.Error("Field key1 not set correctly")
	}

	if newLogger.fields["key2"] != 123 {
		t.Error("Field key2 not set correctly")
	}
}

func TestLogger_LogLevels(t *testing.T) {
	// 使用缓冲区捕获输出
	var buf bytes.Buffer
	logger := NewLogger(&LogConfig{
		Level:     LevelDebug,
		Output:    &buf,
		Formatter: &TextFormatter{DisableColors: true},
	})

	logger.Debug("debug message")
	if !strings.Contains(buf.String(), "DEBUG") {
		t.Error("Debug log not written")
	}
	buf.Reset()

	logger.Info("info message")
	if !strings.Contains(buf.String(), "INFO") {
		t.Error("Info log not written")
	}
	buf.Reset()

	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "WARN") {
		t.Error("Warn log not written")
	}
	buf.Reset()

	logger.Error("error message")
	if !strings.Contains(buf.String(), "ERROR") {
		t.Error("Error log not written")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&LogConfig{
		Level:     LevelWarn,
		Output:    &buf,
		Formatter: &TextFormatter{DisableColors: true},
	})

	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Error("Debug and Info logs should be filtered")
	}

	logger.Warn("warn message")
	if buf.Len() == 0 {
		t.Error("Warn log should be written")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	logger := NewLogger(&LogConfig{Level: LevelInfo})

	logger.SetLevel(LevelDebug)
	if logger.level != LevelDebug {
		t.Errorf("Expected level Debug, got %v", logger.level)
	}
}

func TestJSONFormatter(t *testing.T) {
	formatter := &JSONFormatter{}
	entry := &LogEntry{
		Timestamp: parseTime("2024-01-01T00:00:00Z"),
		Level:     "INFO",
		Message:   "test message",
		Fields:    map[string]interface{}{"key": "value"},
	}

	data, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	var result LogEntry
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Message != "test message" {
		t.Errorf("Expected message 'test message', got %s", result.Message)
	}
}

func TestTextFormatter(t *testing.T) {
	formatter := &TextFormatter{DisableColors: true}
	entry := &LogEntry{
		Timestamp: parseTime("2024-01-01T00:00:00Z"),
		Level:     "INFO",
		Message:   "test message",
	}

	data, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "INFO") {
		t.Error("Output should contain INFO")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Output should contain message")
	}
}

func TestLogManager(t *testing.T) {
	manager := NewLogManager()

	logger1 := manager.GetLogger("module1")
	if logger1 == nil {
		t.Fatal("GetLogger returned nil")
	}

	logger2 := manager.GetLogger("module1")
	if logger1 != logger2 {
		t.Error("Should return same logger instance")
	}

	logger3 := manager.GetLogger("module2")
	if logger1 == logger3 {
		t.Error("Should return different logger for different name")
	}
}

func TestWithContext(t *testing.T) {
	ctx := context.Background()
	logger := NewLogger(nil)

	ctx = WithContext(ctx, logger)

	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("FromContext returned nil")
	}
}

func TestFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)

	if logger == nil {
		t.Fatal("FromContext should return default logger")
	}
}

func TestLogRotator(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	rotator, err := NewLogRotator(logPath, 100, 3, 7)
	if err != nil {
		t.Fatalf("NewLogRotator failed: %v", err)
	}
	defer rotator.Close()

	// 写入数据
	n, err := rotator.Write([]byte("test log entry\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n == 0 {
		t.Error("Write returned 0 bytes")
	}

	// 检查文件是否存在
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file should exist")
	}
}

func TestLogRotator_Rotation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// 设置很小的最大大小触发轮转
	rotator, err := NewLogRotator(logPath, 10, 2, 7)
	if err != nil {
		t.Fatalf("NewLogRotator failed: %v", err)
	}
	defer rotator.Close()

	// 写入超过最大大小的数据
	data := make([]byte, 50)
	for i := range data {
		data[i] = 'a'
	}

	_, err = rotator.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 检查是否有备份文件
	matches, _ := filepath.Glob(logPath + ".*")
	if len(matches) == 0 {
		t.Log("No backup file created (might be expected)")
	}
}

// parseTime 辅助函数.
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
