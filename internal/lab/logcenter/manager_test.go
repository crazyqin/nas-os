// Package logcenter 测试
package logcenter

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	config := DefaultConfig()
	config.MaxEntries = 1000
	config.RetentionDays = 7
	return NewManager(logger, config)
}

func TestNewManager(t *testing.T) {
	m := newTestManager(t)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.MaxEntries != 1000 {
		t.Errorf("MaxEntries: 期望 1000, 实际 %d", m.config.MaxEntries)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.MaxEntries != 100000 {
		t.Errorf("MaxEntries: 期望 100000, 实际 %d", config.MaxEntries)
	}
	if config.RetentionDays != 30 {
		t.Errorf("RetentionDays: 期望 30, 实际 %d", config.RetentionDays)
	}
}

func TestAddLog(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{
		Level:   LogLevelInfo,
		Source:  SourceSystem,
		Message: "测试日志消息",
	})

	stats := m.GetStats()
	if stats.TotalCount != 1 {
		t.Errorf("TotalCount: 期望 1, 实际 %d", stats.TotalCount)
	}
}

func TestAddMultipleLogs(t *testing.T) {
	m := newTestManager(t)

	for i := 0; i < 10; i++ {
		m.Add(LogEntry{
			Level:   LogLevelInfo,
			Source:  SourceSystem,
			Message: "日志消息",
		})
	}

	stats := m.GetStats()
	if stats.TotalCount != 10 {
		t.Errorf("TotalCount: 期望 10, 实际 %d", stats.TotalCount)
	}
}

func TestQueryByLevel(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "info"})
	m.Add(LogEntry{Level: LogLevelError, Source: SourceSystem, Message: "error"})
	m.Add(LogEntry{Level: LogLevelWarn, Source: SourceSystem, Message: "warn"})

	result := m.Query(LogQuery{Level: LogLevelError})
	if len(result.Logs) != 1 {
		t.Errorf("期望 1 条错误日志, 实际 %d", len(result.Logs))
	}
	if result.Logs[0].Message != "error" {
		t.Errorf("消息: 期望 error, 实际 %s", result.Logs[0].Message)
	}
}

func TestQueryBySource(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "sys"})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceAuth, Message: "auth"})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceDocker, Message: "docker"})

	result := m.Query(LogQuery{Source: SourceAuth})
	if len(result.Logs) != 1 {
		t.Errorf("期望 1 条 auth 日志, 实际 %d", len(result.Logs))
	}
}

func TestQueryByKeywords(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "用户登录成功"})
	m.Add(LogEntry{Level: LogLevelError, Source: SourceSystem, Message: "连接数据库失败"})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "系统启动完成"})

	result := m.Query(LogQuery{Keywords: "数据库"})
	if len(result.Logs) != 1 {
		t.Errorf("期望 1 条匹配日志, 实际 %d", len(result.Logs))
	}
}

func TestQueryPagination(t *testing.T) {
	m := newTestManager(t)

	for i := 0; i < 100; i++ {
		m.Add(LogEntry{
			Level:   LogLevelInfo,
			Source:  SourceSystem,
			Message: "日志",
		})
	}

	result := m.Query(LogQuery{Page: 1, PageSize: 10})
	if len(result.Logs) != 10 {
		t.Errorf("期望 10 条, 实际 %d", len(result.Logs))
	}
	if result.Total != 100 {
		t.Errorf("Total: 期望 100, 实际 %d", result.Total)
	}
	if result.TotalPages != 10 {
		t.Errorf("TotalPages: 期望 10, 实际 %d", result.TotalPages)
	}
}

func TestQueryByTimeRange(t *testing.T) {
	m := newTestManager(t)

	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "old", Timestamp: old})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "recent", Timestamp: recent})

	result := m.Query(LogQuery{StartTime: time.Now().Add(-1 * time.Hour)})
	if len(result.Logs) != 1 {
		t.Errorf("期望 1 条, 实际 %d", len(result.Logs))
	}
	if result.Logs[0].Message != "recent" {
		t.Errorf("消息: 期望 recent, 实际 %s", result.Logs[0].Message)
	}
}

func TestGetStats(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "info"})
	m.Add(LogEntry{Level: LogLevelError, Source: SourceSystem, Message: "error"})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceAuth, Message: "auth"})

	stats := m.GetStats()
	if stats.TotalCount != 3 {
		t.Errorf("TotalCount: 期望 3, 实际 %d", stats.TotalCount)
	}
	if stats.LevelCounts["info"] != 2 {
		t.Errorf("info count: 期望 2, 实际 %d", stats.LevelCounts["info"])
	}
	if stats.LevelCounts["error"] != 1 {
		t.Errorf("error count: 期望 1, 实际 %d", stats.LevelCounts["error"])
	}
	if stats.SourceCounts["system"] != 2 {
		t.Errorf("system count: 期望 2, 实际 %d", stats.SourceCounts["system"])
	}
}

func TestClearLogs(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "test"})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "test"})

	m.Clear()

	stats := m.GetStats()
	if stats.TotalCount != 0 {
		t.Errorf("清空后期望 0, 实际 %d", stats.TotalCount)
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	m := newTestManager(t)

	ch := m.Subscribe()
	if ch == nil {
		t.Fatal("Subscribe 返回 nil")
	}

	// 添加日志，应该收到通知
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "test"})

	select {
	case msg := <-ch:
		if msg.Type != "log" {
			t.Errorf("消息类型: 期望 log, 实际 %s", msg.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("超时未收到日志通知")
	}

	m.Unsubscribe(ch)
}

func TestGetSources(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "sys"})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceAuth, Message: "auth"})

	sources := m.GetSources()
	if len(sources) != 2 {
		t.Errorf("期望 2 个来源, 实际 %d", len(sources))
	}
}

func TestGetCategories(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Category: "kernel", Message: "test"})
	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Category: "network", Message: "test"})

	categories := m.GetCategories()
	if len(categories) != 2 {
		t.Errorf("期望 2 个分类, 实际 %d", len(categories))
	}
}

func TestUpdateConfig(t *testing.T) {
	m := newTestManager(t)

	newConfig := LogConfig{
		MaxEntries:    50000,
		RetentionDays: 60,
		EnableSyslog:  false,
	}

	m.UpdateConfig(newConfig)

	config := m.GetConfig()
	if config.MaxEntries != 50000 {
		t.Errorf("MaxEntries: 期望 50000, 实际 %d", config.MaxEntries)
	}
	if config.RetentionDays != 60 {
		t.Errorf("RetentionDays: 期望 60, 实际 %d", config.RetentionDays)
	}
	if config.EnableSyslog {
		t.Error("EnableSyslog 应为 false")
	}
}

func TestMaxEntriesLimit(t *testing.T) {
	m := newTestManager(t)
	m.config.MaxEntries = 5

	for i := 0; i < 10; i++ {
		m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "test"})
	}

	stats := m.GetStats()
	if stats.TotalCount > 5 {
		t.Errorf("超过上限: 期望 <=5, 实际 %d", stats.TotalCount)
	}
}

func TestExportJSON(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "test"})

	data, err := m.Export(LogQuery{}, "json")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if len(data) == 0 {
		t.Error("导出数据为空")
	}
}

func TestExportCSV(t *testing.T) {
	m := newTestManager(t)

	m.Add(LogEntry{Level: LogLevelInfo, Source: SourceSystem, Message: "test"})

	data, err := m.Export(LogQuery{}, "csv")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if len(data) == 0 {
		t.Error("导出数据为空")
	}
}

func TestExportUnsupportedFormat(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Export(LogQuery{}, "xml")
	if err == nil {
		t.Error("不支持的格式应返回错误")
	}
}
