// Package snapverify 提供快照自动验证测试功能
package snapverify

import (
	"time"
)

// TestType 测试类型枚举
type TestType int

const (
	TestTypeIntegrity   TestType = iota + 1 // 完整性校验
	TestTypeRestore                         // 恢复测试
	TestTypeFileCheck                       // 文件检查
	TestTypePerformance                     // 性能测试
	TestTypeFull                            // 完整测试
)

var testTypeNames = map[TestType]string{
	TestTypeIntegrity:   "integrity",
	TestTypeRestore:     "restore",
	TestTypeFileCheck:   "filecheck",
	TestTypePerformance: "performance",
	TestTypeFull:        "full",
}

// String 返回测试类型名称
func (t TestType) String() string {
	if name, ok := testTypeNames[t]; ok {
		return name
	}
	return "unknown"
}

// ParseTestType 从字符串解析测试类型
func ParseTestType(s string) TestType {
	for t, name := range testTypeNames {
		if name == s {
			return t
		}
	}
	return 0
}

// TestStatus 测试状态
type TestStatus string

const (
	TestStatusPending   TestStatus = "pending"
	TestStatusRunning   TestStatus = "running"
	TestStatusPassed    TestStatus = "passed"
	TestStatusFailed    TestStatus = "failed"
	TestStatusCancelled TestStatus = "cancelled"
)

// Severity 错误严重级别
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// SnapshotTest 快照测试记录
type SnapshotTest struct {
	ID           string     `json:"id"`
	SnapshotID   string     `json:"snapshot_id"`
	SnapshotPath string     `json:"snapshot_path"`
	TestType     TestType   `json:"test_type"`
	Status       TestStatus `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Duration     float64    `json:"duration"` // 秒
}

// TestError 测试错误
type TestError struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	FilePath string   `json:"file_path,omitempty"`
	Severity Severity `json:"severity"`
}

// TestResult 测试结果
type TestResult struct {
	TestID   string                 `json:"test_id"`
	Passed   bool                   `json:"passed"`
	Errors   []TestError            `json:"errors,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// VerifyPolicy 验证策略
type VerifyPolicy struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Schedule      string   `json:"schedule"` // cron 表达式
	TestType      TestType `json:"test_type"`
	AutoRepair    bool     `json:"auto_repair"`
	RetentionDays int      `json:"retention_days"`
	Enabled       bool     `json:"enabled"`
}

// IntegrityReport 完整性报告
type IntegrityReport struct {
	TotalFiles      int  `json:"total_files"`
	VerifiedFiles   int  `json:"verified_files"`
	CorruptedFiles  int  `json:"corrupted_files"`
	MissingFiles    int  `json:"missing_files"`
	ChecksumMatches bool `json:"checksum_matches"`
}

// RestoreTest 恢复测试结果
type RestoreTest struct {
	SnapshotID    string        `json:"snapshot_id"`
	RestorePath   string        `json:"restore_path"`
	FilesRestored int           `json:"files_restored"`
	DataMatch     bool          `json:"data_match"`
	Duration      time.Duration `json:"duration"`
}

// PerformanceTest 性能测试结果
type PerformanceTest struct {
	ReadSpeed  float64 `json:"read_speed"`  // MB/s
	WriteSpeed float64 `json:"write_speed"` // MB/s
	IOPS       int64   `json:"iops"`
	Latency    float64 `json:"latency"` // ms
}

// VerifyStats 验证统计
type VerifyStats struct {
	TotalTests  int        `json:"total_tests"`
	PassedTests int        `json:"passed_tests"`
	FailedTests int        `json:"failed_tests"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	AvgDuration float64    `json:"avg_duration"` // 秒
}

// RunTestRequest 运行测试请求
type RunTestRequest struct {
	SnapshotID string   `json:"snapshot_id" binding:"required"`
	TestType   TestType `json:"test_type" binding:"required,min=1,max=5"`
}

// CreatePolicyRequest 创建策略请求
type CreatePolicyRequest struct {
	Name          string   `json:"name" binding:"required"`
	Schedule      string   `json:"schedule" binding:"required"`
	TestType      TestType `json:"test_type" binding:"required,min=1,max=5"`
	AutoRepair    bool     `json:"auto_repair"`
	RetentionDays int      `json:"retention_days"`
	Enabled       bool     `json:"enabled"`
}

// UpdatePolicyRequest 更新策略请求
type UpdatePolicyRequest struct {
	Name          *string   `json:"name,omitempty"`
	Schedule      *string   `json:"schedule,omitempty"`
	TestType      *TestType `json:"test_type,omitempty"`
	AutoRepair    *bool     `json:"auto_repair,omitempty"`
	RetentionDays *int      `json:"retention_days,omitempty"`
	Enabled       *bool     `json:"enabled,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}
