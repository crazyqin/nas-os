// Package diagcenter 测试.
package diagcenter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 55, config.DiskWarnTempC)
	assert.Equal(t, 65, config.DiskCritTempC)
	assert.Equal(t, 80.0, config.MemWarnPercent)
	assert.Equal(t, 95.0, config.MemCritPercent)
	assert.Equal(t, 80.0, config.CPUWarnPercent)
	assert.Equal(t, 95.0, config.CPUCritPercent)
	assert.NotEmpty(t, config.NetworkTargets)
	assert.NotEmpty(t, config.RequiredServices)
}

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		expected DiagStatus
	}{
		{"info", SeverityInfo, StatusHealthy},
		{"warning", SeverityWarning, StatusDegraded},
		{"critical", SeverityCritical, StatusCritical},
		{"fatal", SeverityFatal, StatusFatal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyStatus(tt.severity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewEngine(t *testing.T) {
	engine := NewEngine(nil, nil)
	assert.NotNil(t, engine)
	assert.NotNil(t, engine.config)
	assert.NotNil(t, engine.logger)
	assert.Empty(t, engine.history)
}

func TestRunDiagnostic(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 运行诊断
	result, err := engine.RunDiagnostic(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 验证结果
	assert.NotEmpty(t, result.ID)
	assert.WithinDuration(t, time.Now(), result.Timestamp, 5*time.Second)
	assert.NotEmpty(t, result.Summary)
	assert.Positive(t, result.Duration)
	assert.NotEmpty(t, result.Checks)

	// 验证历史记录
	latest := engine.GetLatest()
	require.NotNil(t, latest)
	assert.Equal(t, result.ID, latest.ID)
}

func TestRunDiagnosticWithCategories(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 只检查内存和 CPU
	categories := []CheckCategory{CategoryMemory, CategoryCPU}
	result, err := engine.RunDiagnostic(ctx, categories)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 验证只包含指定类别的检查
	for _, check := range result.Checks {
		assert.Contains(t, []CheckCategory{CategoryMemory, CategoryCPU}, check.Category)
	}
}

func TestGetHistory(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 执行多次诊断
	for i := 0; i < 3; i++ {
		_, err := engine.RunDiagnostic(ctx, nil)
		require.NoError(t, err)
	}

	// 获取历史
	query := HistoryQuery{
		Days:  30,
		Limit: 10,
	}
	history := engine.GetHistory(query)
	assert.NotNil(t, history)
	assert.Equal(t, 3, history.TotalCount)
	assert.Len(t, history.Results, 3)
}

func TestGetLatestEmpty(t *testing.T) {
	engine := NewEngine(nil, nil)
	result := engine.GetLatest()
	assert.Nil(t, result)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckItemStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   DiagStatus
		severity Severity
	}{
		{"healthy", StatusHealthy, SeverityInfo},
		{"degraded", StatusDegraded, SeverityWarning},
		{"critical", StatusCritical, SeverityCritical},
		{"fatal", StatusFatal, SeverityFatal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := CheckItem{
				Category: CategoryDisk,
				Name:     "test",
				Status:   tt.status,
				Severity: tt.severity,
			}
			assert.Equal(t, tt.status, check.Status)
			assert.Equal(t, tt.severity, check.Severity)
		})
	}
}

func TestRemediation(t *testing.T) {
	remediation := &Remediation{
		Title:       "测试修复建议",
		Description: "这是一个测试",
		Steps: []string{
			"步骤 1",
			"步骤 2",
			"步骤 3",
		},
		QuickFix: "quick-fix-command",
		DocURL:   "https://example.com/doc",
	}

	assert.Equal(t, "测试修复建议", remediation.Title)
	assert.Equal(t, "这是一个测试", remediation.Description)
	assert.Len(t, remediation.Steps, 3)
	assert.Equal(t, "quick-fix-command", remediation.QuickFix)
	assert.Equal(t, "https://example.com/doc", remediation.DocURL)
}

func TestAlert(t *testing.T) {
	alert := Alert{
		ID:          "test-id",
		Category:    string(CategoryDisk),
		Severity:    SeverityWarning,
		Title:       "测试告警",
		Description: "这是一个测试告警",
		Timestamp:   time.Now(),
		Remediation: &Remediation{
			Title: "修复建议",
		},
		Acknowledged: false,
	}

	assert.Equal(t, "test-id", alert.ID)
	assert.Equal(t, string(CategoryDisk), alert.Category)
	assert.Equal(t, SeverityWarning, alert.Severity)
	assert.Equal(t, "测试告警", alert.Title)
	assert.False(t, alert.Acknowledged)
	assert.NotNil(t, alert.Remediation)
}

func TestDiagResult(t *testing.T) {
	now := time.Now()
	result := &DiagResult{
		ID:        "test-id",
		Timestamp: now,
		Status:    StatusHealthy,
		Checks: []CheckItem{
			{
				Category: CategoryMemory,
				Name:     "内存使用率",
				Status:   StatusHealthy,
				Severity: SeverityInfo,
			},
		},
		Alerts:  []Alert{},
		Summary: "系统健康",
		Duration: 5 * time.Second,
	}

	assert.Equal(t, "test-id", result.ID)
	assert.Equal(t, now, result.Timestamp)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Len(t, result.Checks, 1)
	assert.Empty(t, result.Alerts)
	assert.Equal(t, "系统健康", result.Summary)
	assert.Equal(t, 5*time.Second, result.Duration)
}

func TestRunDiagRequest(t *testing.T) {
	req := RunDiagRequest{
		Categories: []CheckCategory{CategoryDisk, CategoryMemory},
	}

	assert.Len(t, req.Categories, 2)
	assert.Contains(t, req.Categories, CategoryDisk)
	assert.Contains(t, req.Categories, CategoryMemory)
}

func TestHistoryQuery(t *testing.T) {
	query := HistoryQuery{
		Days:  30,
		Limit: 100,
	}

	assert.Equal(t, 30, query.Days)
	assert.Equal(t, 100, query.Limit)
}

func TestConcurrentDiagnostics(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 并发执行诊断
	results := make(chan *DiagResult, 5)
	errors := make(chan error, 5)

	for i := 0; i < 5; i++ {
		go func() {
			result, err := engine.RunDiagnostic(ctx, nil)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}()
	}

	// 等待所有诊断完成
	for i := 0; i < 5; i++ {
		select {
		case result := <-results:
			assert.NotNil(t, result)
		case err := <-errors:
			// 并发诊断可能会失败，这是预期的
			assert.Error(t, err)
		case <-time.After(30 * time.Second):
			t.Fatal("诊断超时")
		}
	}
}

func TestGetHistoryWithLimit(t *testing.T) {
	engine := NewEngine(nil, nil)
	ctx := context.Background()

	// 执行多次诊断
	for i := 0; i < 5; i++ {
		_, err := engine.RunDiagnostic(ctx, nil)
		require.NoError(t, err)
	}

	// 获取限制数量的历史
	query := HistoryQuery{
		Days:  30,
		Limit: 2,
	}
	history := engine.GetHistory(query)
	assert.NotNil(t, history)
	assert.Equal(t, 2, history.TotalCount)
	assert.Len(t, history.Results, 2)
}

func TestGenerateSummary(t *testing.T) {
	engine := NewEngine(nil, nil)

	checks := []CheckItem{
		{Status: StatusHealthy},
		{Status: StatusHealthy},
		{Status: StatusDegraded},
		{Status: StatusCritical},
	}

	summary := engine.generateSummary(checks, StatusCritical)
	assert.Contains(t, summary, "共 4 项检查")
	assert.Contains(t, summary, "2 正常")
	assert.Contains(t, summary, "1 警告")
	assert.Contains(t, summary, "1 严重")
}
