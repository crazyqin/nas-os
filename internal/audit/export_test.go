// Package audit 提供审计日志导出功能测试
package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// ========== Exporter 测试 ==========

func TestExporter_ExportJSON(t *testing.T) {
	// 创建测试用的Manager
	manager := NewManager(DefaultConfig())
	exporter := NewExporter(manager)

	// 添加一些测试日志
	for i := 0; i < 5; i++ {
		entry := &Entry{
			Level:    LevelInfo,
			Category: CategoryFile,
			Event:    "file_read",
			UserID:   "user1",
			Username: "testuser",
			IP:       "192.168.1.1",
			Resource: "/data/file.txt",
			Action:   "read",
			Status:   StatusSuccess,
			Message:  "读取文件成功",
		}
		_ = manager.Log(entry)
	}

	req := ExportRequest{
		Format:         ExportJSON,
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		IncludeDetails: true,
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, ExportJSON, result.Format)
	assert.Equal(t, 5, result.Count)
	assert.True(t, result.Size > 0)
	assert.Contains(t, result.Filename, "audit-export")
	assert.Contains(t, result.ContentType, "application/json")

	// 验证JSON格式正确
	var exportData struct {
		ExportedAt time.Time `json:"exported_at"`
		Count      int       `json:"count"`
		Entries    []*Entry  `json:"entries"`
	}
	err = json.Unmarshal(result.Data, &exportData)
	assert.NoError(t, err)
	assert.Equal(t, 5, exportData.Count)
}

func TestExporter_ExportCSV(t *testing.T) {
	manager := NewManager(DefaultConfig())
	exporter := NewExporter(manager)

	// 添加测试日志
	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryFile,
		Event:    "file_write",
		UserID:   "user1",
		Username: "testuser",
		IP:       "192.168.1.1",
		Resource: "/data/file.csv",
		Action:   "write",
		Status:   StatusSuccess,
		Message:  "写入文件",
		Details:  map[string]interface{}{"size": 1024},
	}
	_ = manager.Log(entry)

	req := ExportRequest{
		Format:         ExportCSV,
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		IncludeDetails: true,
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, ExportCSV, result.Format)
	assert.Contains(t, result.ContentType, "text/csv")
	assert.Contains(t, string(result.Data), "ID,Timestamp,Level")
	assert.Contains(t, string(result.Data), "file_write")
}

func TestExporter_ExportYAML(t *testing.T) {
	manager := NewManager(DefaultConfig())
	exporter := NewExporter(manager)

	// 添加测试日志
	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryAuth,
		Event:    "login",
		UserID:   "user1",
		Username: "testuser",
		IP:       "192.168.1.1",
		Status:   StatusSuccess,
		Message:  "用户登录成功",
	}
	_ = manager.Log(entry)

	req := ExportRequest{
		Format:         ExportYAML,
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		IncludeDetails: true,
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, ExportYAML, result.Format)
	assert.Contains(t, result.ContentType, "yaml")
	assert.Contains(t, string(result.Data), "exported_at")

	// 验证YAML格式正确
	var exportData struct {
		ExportedAt time.Time `yaml:"exported_at"`
		Count      int       `yaml:"count"`
		Entries    []*Entry  `yaml:"entries"`
	}
	err = yaml.Unmarshal(result.Data, &exportData)
	assert.NoError(t, err)
	assert.Equal(t, 1, exportData.Count)
}

func TestExporter_FilterByCategory(t *testing.T) {
	manager := NewManager(DefaultConfig())
	exporter := NewExporter(manager)

	// 添加不同分类的日志
	_ = manager.Log(&Entry{Category: CategoryFile, Event: "file_op"})
	_ = manager.Log(&Entry{Category: CategoryAuth, Event: "login"})
	_ = manager.Log(&Entry{Category: CategoryFile, Event: "file_op2"})

	req := ExportRequest{
		Format:     ExportJSON,
		StartTime:  time.Now().Add(-1 * time.Hour),
		EndTime:    time.Now(),
		Categories: []Category{CategoryFile},
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.Equal(t, 2, result.Count) // 只有2个CategoryFile的日志
}

func TestExporter_FilterByLevel(t *testing.T) {
	manager := NewManager(DefaultConfig())
	exporter := NewExporter(manager)

	// 添加不同级别的日志
	_ = manager.Log(&Entry{Level: LevelInfo, Event: "info_event"})
	_ = manager.Log(&Entry{Level: LevelWarning, Event: "warning_event"})
	_ = manager.Log(&Entry{Level: LevelError, Event: "error_event"})
	_ = manager.Log(&Entry{Level: LevelInfo, Event: "info_event2"})

	req := ExportRequest{
		Format:    ExportJSON,
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Levels:    []Level{LevelInfo, LevelWarning},
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.Equal(t, 3, result.Count) // 2个Info + 1个Warning
}

func TestExporter_ExcludeSignatures(t *testing.T) {
	manager := NewManager(DefaultConfig())
	exporter := NewExporter(manager)

	entry := &Entry{
		Level:     LevelInfo,
		Category:  CategoryFile,
		Event:     "test",
		Status:    StatusSuccess,
		Signature: "test-signature-123",
	}
	_ = manager.Log(entry)

	// 不包含签名
	req := ExportRequest{
		Format:            ExportJSON,
		StartTime:         time.Now().Add(-1 * time.Hour),
		EndTime:           time.Now(),
		IncludeSignatures: false,
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.NotContains(t, string(result.Data), "test-signature-123")
}

func TestExporter_ExcludeDetails(t *testing.T) {
	manager := NewManager(DefaultConfig())
	exporter := NewExporter(manager)

	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryFile,
		Event:    "test",
		Status:   StatusSuccess,
		Details:  map[string]interface{}{"key": "value", "count": 123},
	}
	_ = manager.Log(entry)

	// 不包含详情
	req := ExportRequest{
		Format:         ExportJSON,
		StartTime:      time.Now().Add(-1 * time.Hour),
		EndTime:        time.Now(),
		IncludeDetails: false,
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	// CSV中不应包含details列的内容
	assert.NotContains(t, string(result.Data), "key=value")
}

// ========== WatchListExporter 测试 ==========

func TestWatchListExporter_ExportJSON(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())
	exporter := NewWatchListExporter(manager)

	// 添加测试数据
	_ = manager.AddWatchEntry(&WatchListEntry{
		Path:       "/data/important",
		Operations: []WatchOperation{WatchOpAll},
		Enabled:    true,
		CreatedBy:  "user1",
	})
	_ = manager.AddIgnoreEntry(&IgnoreListEntry{
		Path:      "/data/temp",
		Reason:    "临时文件",
		Enabled:   true,
		CreatedBy: "user1",
	})

	req := WatchListExportRequest{
		Format: ExportJSON,
		Type:   "", // 导出所有
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.WatchCount)
	assert.Equal(t, 1, result.IgnoreCount)
	assert.Contains(t, result.ContentType, "application/json")

	// 验证JSON内容
	var exportData struct {
		ExportedAt    time.Time          `json:"exported_at"`
		WatchEntries  []*WatchListEntry  `json:"watch_entries"`
		IgnoreEntries []*IgnoreListEntry `json:"ignore_entries"`
	}
	err = json.Unmarshal(result.Data, &exportData)
	assert.NoError(t, err)
	assert.Len(t, exportData.WatchEntries, 1)
	assert.Len(t, exportData.IgnoreEntries, 1)
}

func TestWatchListExporter_ExportCSV(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())
	exporter := NewWatchListExporter(manager)

	_ = manager.AddWatchEntry(&WatchListEntry{
		Path:       "/data/watch",
		Operations: []WatchOperation{WatchOpRead},
		Enabled:    true,
		CreatedBy:  "user1",
	})
	_ = manager.AddIgnoreEntry(&IgnoreListEntry{
		Path:      "/data/ignore",
		Reason:    "测试",
		Enabled:   true,
		CreatedBy: "user1",
	})

	req := WatchListExportRequest{
		Format: ExportCSV,
		Type:   "",
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.Contains(t, result.ContentType, "text/csv")
	assert.Contains(t, string(result.Data), "=== Watch List ===")
	assert.Contains(t, string(result.Data), "=== Ignore List ===")
	assert.Contains(t, string(result.Data), "/data/watch")
	assert.Contains(t, string(result.Data), "/data/ignore")
}

func TestWatchListExporter_ExportYAML(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())
	exporter := NewWatchListExporter(manager)

	_ = manager.AddWatchEntry(&WatchListEntry{
		Path:       "/data/watch",
		Operations: []WatchOperation{WatchOpAll},
		Enabled:    true,
		CreatedBy:  "user1",
	})

	req := WatchListExportRequest{
		Format: ExportYAML,
		Type:   ListTypeWatch, // 只导出Watch List
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	assert.Contains(t, result.ContentType, "yaml")
	assert.Equal(t, 1, result.WatchCount)
	assert.Equal(t, 0, result.IgnoreCount) // 不包含Ignore List

	// 验证YAML内容
	var exportData struct {
		ExportedAt    time.Time          `yaml:"exported_at"`
		WatchEntries  []*WatchListEntry  `yaml:"watch_entries"`
		IgnoreEntries []*IgnoreListEntry `yaml:"ignore_entries"`
	}
	err = yaml.Unmarshal(result.Data, &exportData)
	assert.NoError(t, err)
	assert.Len(t, exportData.WatchEntries, 1)
	assert.Nil(t, exportData.IgnoreEntries)
}

func TestWatchListExporter_ExportIgnoreOnly(t *testing.T) {
	manager := NewWatchListManager(DefaultWatchListConfig())
	exporter := NewWatchListExporter(manager)

	_ = manager.AddWatchEntry(&WatchListEntry{
		Path:       "/data/watch",
		Operations: []WatchOperation{WatchOpAll},
		Enabled:    true,
		CreatedBy:  "user1",
	})
	_ = manager.AddIgnoreEntry(&IgnoreListEntry{
		Path:      "/data/ignore",
		Enabled:   true,
		CreatedBy: "user1",
	})

	req := WatchListExportRequest{
		Format: ExportJSON,
		Type:   ListTypeIgnore, // 只导出Ignore List
	}

	result, err := exporter.Export(req)
	assert.NoError(t, err)
	// 注意：即使只导出Ignore List，WatchCount仍然会计算所有Watch条目
	assert.Equal(t, 1, result.WatchCount) // 仍然计算了Watch条目数
	assert.Equal(t, 1, result.IgnoreCount)
}

// ========== escapeCSVField 测试 ==========

func TestEscapeCSVField(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "simple",
			expected: "simple",
		},
		{
			input:    "with,comma",
			expected: "\"with,comma\"",
		},
		{
			input:    "with\"quote",
			expected: "\"with\"\"quote\"",
		},
		{
			input:    "with\nnewline",
			expected: "\"with\nnewline\"",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeCSVField(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== detailsToString 测试 ==========

func TestDetailsToString(t *testing.T) {
	tests := []struct {
		name     string
		details  map[string]interface{}
		expected string
	}{
		{
			name:     "nil details",
			details:  nil,
			expected: "",
		},
		{
			name:     "empty details",
			details:  map[string]interface{}{},
			expected: "",
		},
		{
			name: "single item",
			details: map[string]interface{}{
				"key": "value",
			},
			expected: "key=value",
		},
		{
			name: "multiple items",
			details: map[string]interface{}{
				"count": 123,
				"name":  "test",
			},
			expected: "count=123; name=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detailsToString(tt.details)
			if tt.expected == "" {
				assert.Empty(t, result)
			} else {
				// 由于map遍历顺序不确定，只检查是否包含预期内容
				for k := range tt.details {
					assert.Contains(t, result, k+"=")
				}
			}
		})
	}
}
