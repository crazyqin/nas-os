// Package snapverify 提供快照自动验证测试功能
package snapverify

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SnapVerifyManager 快照验证管理器
type SnapVerifyManager struct {
	mu       sync.RWMutex
	tests    map[string]*SnapshotTest
	results  map[string]*TestResult
	policies map[string]*VerifyPolicy
}

// NewSnapVerifyManager 创建快照验证管理器
func NewSnapVerifyManager() *SnapVerifyManager {
	m := &SnapVerifyManager{
		tests:    make(map[string]*SnapshotTest),
		results:  make(map[string]*TestResult),
		policies: make(map[string]*VerifyPolicy),
	}

	// 添加默认策略
	m.addDefaultPolicies()

	return m
}

// addDefaultPolicies 添加默认验证策略
func (m *SnapVerifyManager) addDefaultPolicies() {
	defaults := []VerifyPolicy{
		{
			ID:            uuid.New().String(),
			Name:          "每日完整性检查",
			Schedule:      "0 2 * * *",
			TestType:      TestTypeIntegrity,
			AutoRepair:    false,
			RetentionDays: 30,
			Enabled:       true,
		},
		{
			ID:            uuid.New().String(),
			Name:          "每周恢复测试",
			Schedule:      "0 3 * * 0",
			TestType:      TestTypeRestore,
			AutoRepair:    true,
			RetentionDays: 90,
			Enabled:       true,
		},
	}

	for _, p := range defaults {
		pCopy := p
		m.policies[p.ID] = &pCopy
	}
}

// RunTest 运行快照测试
func (m *SnapVerifyManager) RunTest(ctx context.Context, snapshotID string, testType TestType) (*SnapshotTest, error) {
	if snapshotID == "" {
		return nil, fmt.Errorf("snapshot ID is required")
	}
	if testType < TestTypeIntegrity || testType > TestTypeFull {
		return nil, fmt.Errorf("invalid test type: %d", testType)
	}

	test := &SnapshotTest{
		ID:           uuid.New().String(),
		SnapshotID:   snapshotID,
		SnapshotPath: fmt.Sprintf("/snapshots/%s", snapshotID),
		TestType:     testType,
		Status:       TestStatusRunning,
		StartedAt:    time.Now(),
	}

	m.mu.Lock()
	m.tests[test.ID] = test
	m.mu.Unlock()

	// 模拟测试执行
	go m.executeTest(ctx, test)

	return test, nil
}

// executeTest 执行测试（模拟实现）
func (m *SnapVerifyManager) executeTest(ctx context.Context, test *SnapshotTest) {
	// 模拟测试耗时
	duration := time.Duration(500+rand.Intn(2000)) * time.Millisecond
	select {
	case <-ctx.Done():
		m.mu.Lock()
		test.Status = TestStatusCancelled
		now := time.Now()
		test.CompletedAt = &now
		test.Duration = now.Sub(test.StartedAt).Seconds()
		m.mu.Unlock()
		return
	case <-time.After(duration):
	}

	// 生成模拟结果
	result := m.generateTestResult(test)

	m.mu.Lock()
	test.Status = TestStatusPassed
	if !result.Passed {
		test.Status = TestStatusFailed
	}
	now := time.Now()
	test.CompletedAt = &now
	test.Duration = now.Sub(test.StartedAt).Seconds()
	m.results[test.ID] = result
	m.mu.Unlock()
}

// generateTestResult 生成测试结果（模拟实现）
func (m *SnapVerifyManager) generateTestResult(test *SnapshotTest) *TestResult {
	result := &TestResult{
		TestID:  test.ID,
		Passed:  true,
		Errors:  []TestError{},
		Warnings: []string{},
		Details: make(map[string]interface{}),
	}

	switch test.TestType {
	case TestTypeIntegrity:
		report := &IntegrityReport{
			TotalFiles:      100 + rand.Intn(900),
			VerifiedFiles:   0,
			CorruptedFiles:  0,
			MissingFiles:    0,
			ChecksumMatches: true,
		}
		// 模拟少量损坏
		if rand.Float64() < 0.1 {
			report.CorruptedFiles = rand.Intn(5) + 1
			report.ChecksumMatches = false
			result.Passed = false
			result.Errors = append(result.Errors, TestError{
				Code:     "INTEGRITY_CORRUPTED",
				Message:  fmt.Sprintf("发现 %d 个损坏文件", report.CorruptedFiles),
				Severity: SeverityHigh,
			})
		}
		report.VerifiedFiles = report.TotalFiles - report.CorruptedFiles - report.MissingFiles
		result.Details["integrity_report"] = report

	case TestTypeRestore:
		restore := &RestoreTest{
			SnapshotID:    test.SnapshotID,
			RestorePath:   fmt.Sprintf("/tmp/restore-test/%s", test.ID),
			FilesRestored: 100 + rand.Intn(900),
			DataMatch:     true,
			Duration:      time.Duration(rand.Intn(30)) * time.Second,
		}
		if rand.Float64() < 0.05 {
			restore.DataMatch = false
			result.Passed = false
			result.Errors = append(result.Errors, TestError{
				Code:    "RESTORE_MISMATCH",
				Message: "恢复数据与原始快照不匹配",
				Severity: SeverityCritical,
			})
		}
		result.Details["restore_test"] = restore

	case TestTypeFileCheck:
		total := 100 + rand.Intn(900)
		missing := rand.Intn(3)
		result.Details["total_files"] = total
		result.Details["checked_files"] = total - missing
		result.Details["missing_files"] = missing
		if missing > 0 {
			result.Passed = false
			for i := 0; i < missing; i++ {
				result.Errors = append(result.Errors, TestError{
					Code:     "FILE_MISSING",
					Message:  fmt.Sprintf("文件丢失: /data/file_%d.dat", i),
					FilePath: fmt.Sprintf("/data/file_%d.dat", i),
					Severity: SeverityMedium,
				})
			}
		}

	case TestTypePerformance:
		perf := &PerformanceTest{
			ReadSpeed:  100 + rand.Float64()*400,
			WriteSpeed: 50 + rand.Float64()*200,
			IOPS:       int64(1000 + rand.Intn(9000)),
			Latency:    1 + rand.Float64()*10,
		}
		if perf.Latency > 8 {
			result.Warnings = append(result.Warnings, "延迟较高，建议检查存储性能")
		}
		result.Details["performance_test"] = perf

	case TestTypeFull:
		// 完整测试包含所有子测试
		integrityResult := m.generateTestResult(&SnapshotTest{
			ID:         test.ID + "-integrity",
			SnapshotID: test.SnapshotID,
			TestType:   TestTypeIntegrity,
		})
		restoreResult := m.generateTestResult(&SnapshotTest{
			ID:         test.ID + "-restore",
			SnapshotID: test.SnapshotID,
			TestType:   TestTypeRestore,
		})
		perfResult := m.generateTestResult(&SnapshotTest{
			ID:         test.ID + "-perf",
			SnapshotID: test.SnapshotID,
			TestType:   TestTypePerformance,
		})

		result.Passed = integrityResult.Passed && restoreResult.Passed && perfResult.Passed
		result.Errors = append(result.Errors, integrityResult.Errors...)
		result.Errors = append(result.Errors, restoreResult.Errors...)
		result.Warnings = append(result.Warnings, perfResult.Warnings...)
		result.Details["integrity"] = integrityResult.Details
		result.Details["restore"] = restoreResult.Details
		result.Details["performance"] = perfResult.Details
	}

	return result
}

// GetTestResult 获取测试结果
func (m *SnapVerifyManager) GetTestResult(testID string) (*TestResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.results[testID]
	if !ok {
		return nil, fmt.Errorf("test result not found: %s", testID)
	}
	return result, nil
}

// ListTests 列出快照的所有测试
func (m *SnapVerifyManager) ListTests(snapshotID string) []SnapshotTest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tests []SnapshotTest
	for _, t := range m.tests {
		if snapshotID == "" || t.SnapshotID == snapshotID {
			tests = append(tests, *t)
		}
	}
	return tests
}

// CreatePolicy 创建验证策略
func (m *SnapVerifyManager) CreatePolicy(ctx context.Context, policy VerifyPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if policy.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	if policy.TestType < TestTypeIntegrity || policy.TestType > TestTypeFull {
		return fmt.Errorf("invalid test type: %d", policy.TestType)
	}

	// 检查名称唯一性
	for _, p := range m.policies {
		if p.Name == policy.Name {
			return fmt.Errorf("policy with name '%s' already exists", policy.Name)
		}
	}

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	if policy.RetentionDays <= 0 {
		policy.RetentionDays = 30
	}

	m.policies[policy.ID] = &policy
	return nil
}

// UpdatePolicy 更新验证策略
func (m *SnapVerifyManager) UpdatePolicy(id string, policy VerifyPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.policies[id]
	if !ok {
		return fmt.Errorf("policy not found: %s", id)
	}

	if policy.Name != "" {
		// 检查名称唯一性（排除自身）
		for pid, p := range m.policies {
			if pid != id && p.Name == policy.Name {
				return fmt.Errorf("policy with name '%s' already exists", policy.Name)
			}
		}
		existing.Name = policy.Name
	}
	if policy.Schedule != "" {
		existing.Schedule = policy.Schedule
	}
	if policy.TestType >= TestTypeIntegrity && policy.TestType <= TestTypeFull {
		existing.TestType = policy.TestType
	}
	existing.AutoRepair = policy.AutoRepair
	if policy.RetentionDays > 0 {
		existing.RetentionDays = policy.RetentionDays
	}
	existing.Enabled = policy.Enabled

	return nil
}

// DeletePolicy 删除验证策略
func (m *SnapVerifyManager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}

	delete(m.policies, id)
	return nil
}

// ListPolicies 列出所有验证策略
func (m *SnapVerifyManager) ListPolicies() []VerifyPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]VerifyPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, *p)
	}
	return policies
}

// RunScheduledTests 运行计划任务测试
func (m *SnapVerifyManager) RunScheduledTests() error {
	m.mu.RLock()
	var enabledPolicies []VerifyPolicy
	for _, p := range m.policies {
		if p.Enabled {
			enabledPolicies = append(enabledPolicies, *p)
		}
	}
	m.mu.RUnlock()

	for _, policy := range enabledPolicies {
		// 模拟运行测试
		test := &SnapshotTest{
			ID:           uuid.New().String(),
			SnapshotID:   "scheduled-" + policy.ID,
			SnapshotPath: fmt.Sprintf("/snapshots/scheduled/%s", policy.ID),
			TestType:     policy.TestType,
			Status:       TestStatusRunning,
			StartedAt:    time.Now(),
		}

		m.mu.Lock()
		m.tests[test.ID] = test
		m.mu.Unlock()

		// 模拟执行
		go func(t *SnapshotTest, p VerifyPolicy) {
			time.Sleep(time.Duration(100+rand.Intn(500)) * time.Millisecond)
			result := m.generateTestResult(t)

			m.mu.Lock()
			t.Status = TestStatusPassed
			if !result.Passed {
				t.Status = TestStatusFailed
			}
			now := time.Now()
			t.CompletedAt = &now
			t.Duration = now.Sub(t.StartedAt).Seconds()
			m.results[t.ID] = result
			m.mu.Unlock()
		}(test, policy)
	}

	return nil
}

// AutoRepair 自动修复
func (m *SnapVerifyManager) AutoRepair(ctx context.Context, testID string) error {
	m.mu.RLock()
	result, ok := m.results[testID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("test result not found: %s", testID)
	}

	if result.Passed {
		return fmt.Errorf("test %s already passed, no repair needed", testID)
	}

	// 模拟修复过程
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(500+rand.Intn(1000)) * time.Millisecond):
	}

	// 清除错误，标记为已修复
	m.mu.Lock()
	result.Passed = true
	result.Errors = nil
	result.Warnings = append(result.Warnings, "已通过自动修复解决")
	result.Details["auto_repaired"] = true
	result.Details["repaired_at"] = time.Now()
	m.mu.Unlock()

	return nil
}

// GetStats 获取验证统计
func (m *SnapVerifyManager) GetStats() VerifyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := VerifyStats{}
	var totalDuration float64

	for _, t := range m.tests {
		if t.Status == TestStatusPassed || t.Status == TestStatusFailed {
			stats.TotalTests++
			totalDuration += t.Duration

			if t.Status == TestStatusPassed {
				stats.PassedTests++
			} else {
				stats.FailedTests++
			}

			if stats.LastRun == nil || t.CompletedAt.After(*stats.LastRun) {
				stats.LastRun = t.CompletedAt
			}
		}
	}

	if stats.TotalTests > 0 {
		stats.AvgDuration = totalDuration / float64(stats.TotalTests)
	}

	return stats
}

// GenerateReport 生成测试报告
func (m *SnapVerifyManager) GenerateReport(testID string, format string) ([]byte, error) {
	m.mu.RLock()
	test, testOK := m.tests[testID]
	result, resultOK := m.results[testID]
	m.mu.RUnlock()

	if !testOK {
		return nil, fmt.Errorf("test not found: %s", testID)
	}
	if !resultOK {
		return nil, fmt.Errorf("test result not found: %s", testID)
	}

	report := map[string]interface{}{
		"test":   test,
		"result": result,
		"generated_at": time.Now(),
	}

	switch format {
	case "json":
		return json.MarshalIndent(report, "", "  ")
	case "text":
		return m.generateTextReport(test, result), nil
	default:
		return json.MarshalIndent(report, "", "  ")
	}
}

// generateTextReport 生成文本报告
func (m *SnapVerifyManager) generateTextReport(test *SnapshotTest, result *TestResult) []byte {
	text := fmt.Sprintf("=== 快照验证测试报告 ===\n")
	text += fmt.Sprintf("测试ID: %s\n", test.ID)
	text += fmt.Sprintf("快照ID: %s\n", test.SnapshotID)
	text += fmt.Sprintf("测试类型: %s\n", test.TestType)
	text += fmt.Sprintf("状态: %s\n", test.Status)
	text += fmt.Sprintf("开始时间: %s\n", test.StartedAt.Format(time.RFC3339))
	if test.CompletedAt != nil {
		text += fmt.Sprintf("完成时间: %s\n", test.CompletedAt.Format(time.RFC3339))
	}
	text += fmt.Sprintf("耗时: %.2f秒\n", test.Duration)
	text += fmt.Sprintf("\n--- 结果 ---\n")
	text += fmt.Sprintf("通过: %v\n", result.Passed)

	if len(result.Errors) > 0 {
		text += fmt.Sprintf("\n错误:\n")
		for _, e := range result.Errors {
			text += fmt.Sprintf("  [%s] %s: %s\n", e.Severity, e.Code, e.Message)
		}
	}

	if len(result.Warnings) > 0 {
		text += fmt.Sprintf("\n警告:\n")
		for _, w := range result.Warnings {
			text += fmt.Sprintf("  - %s\n", w)
		}
	}

	return []byte(text)
}
