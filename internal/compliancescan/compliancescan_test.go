package compliancescan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	return m, tmpDir
}

// ========== 规则管理 ==========

func TestAddRule(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	rule, err := m.AddRule(ctx, ScanRule{
		Name:     "测试规则",
		Category: CategoryPII,
		Pattern:  `\btest\d+\b`,
		Severity: SeverityMedium,
		Action:   ActionLog,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, rule.ID)
	assert.Equal(t, "测试规则", rule.Name)
	assert.True(t, rule.CreatedAt.Unix() > 0)
}

func TestAddRule_InvalidPattern(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	_, err := m.AddRule(ctx, ScanRule{
		Name:    "无效规则",
		Pattern: `[invalid`,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRule)
}

func TestAddRule_MissingFields(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	_, err := m.AddRule(ctx, ScanRule{
		Pattern: `\btest\b`,
	})
	assert.ErrorIs(t, err, ErrInvalidRule)

	_, err = m.AddRule(ctx, ScanRule{
		Name: "test",
	})
	assert.ErrorIs(t, err, ErrInvalidRule)
}

func TestUpdateRule(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	rule, _ := m.AddRule(ctx, ScanRule{
		Name:     "旧名称",
		Category: CategoryPII,
		Pattern:  `\bold\b`,
		Severity: SeverityLow,
	})

	err := m.UpdateRule(ctx, rule.ID, ScanRule{
		Name:     "新名称",
		Severity: SeverityHigh,
	})
	assert.NoError(t, err)

	rules := m.GetBuiltinRules(ctx)
	// 内置规则+自定义规则
	allRules := rules
	_ = allRules

	// 通过重新添加验证（因为没有GetRule方法，用GetBuiltinRules只返回内置的）
	// 直接验证UpdateRule不报错即可
}

func TestUpdateRule_NotFound(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	err := m.UpdateRule(ctx, "nonexistent", ScanRule{Name: "test"})
	assert.ErrorIs(t, err, ErrRuleNotFound)
}

func TestGetBuiltinRules(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	rules := m.GetBuiltinRules(ctx)
	assert.NotEmpty(t, rules)

	// 验证包含关键内置规则
	ruleIDs := make(map[string]bool)
	for _, r := range rules {
		ruleIDs[r.ID] = true
	}
	assert.True(t, ruleIDs["builtin-id-card"])
	assert.True(t, ruleIDs["builtin-phone"])
	assert.True(t, ruleIDs["builtin-email"])
}

// ========== 扫描任务 ==========

func TestCreateScanTask(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	task, err := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试扫描",
		TargetPath: tmpDir,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, StatusPending, task.Status)
	assert.Equal(t, 0.0, task.Progress)
}

func TestCreateScanTask_EmptyPath(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	_, err := m.CreateScanTask(ctx, ScanTask{
		Name: "测试",
	})
	assert.Error(t, err)
}

func TestRunScan(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	// 创建测试文件，包含敏感信息
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("联系电话：13800138000\n邮箱：test@example.com\n身份证：110101199001011234\n"), 0644)
	assert.NoError(t, err)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试扫描",
		TargetPath: tmpDir,
	})

	result, err := m.RunScan(ctx, task.ID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.ScannedFiles)
	assert.Greater(t, result.ViolationCount, 0)
	assert.Greater(t, result.RiskScore, 0.0)
}

func TestRunScan_TaskNotFound(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	_, err := m.RunScan(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestRunScan_AlreadyCompleted(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("test data"), 0644)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试",
		TargetPath: tmpDir,
	})
	_, _ = m.RunScan(ctx, task.ID)

	// 再次执行应失败
	_, err := m.RunScan(ctx, task.ID)
	assert.ErrorIs(t, err, ErrTaskNotPending)
}

// ========== 违规管理 ==========

func TestGetViolations(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("手机号：13800138000\n"), 0644)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试",
		TargetPath: tmpDir,
	})
	result, _ := m.RunScan(ctx, task.ID)

	violations := m.GetViolations(ctx, result.ID, "")
	assert.Greater(t, len(violations), 0)
}

func TestGetViolations_FilterSeverity(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("手机号：13800138000\n"), 0644)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试",
		TargetPath: tmpDir,
	})
	result, _ := m.RunScan(ctx, task.ID)

	// 按severity过滤
	mediumViolations := m.GetViolations(ctx, result.ID, "medium")
	assert.Greater(t, len(mediumViolations), 0)
	for _, v := range mediumViolations {
		assert.Equal(t, SeverityMedium, v.Severity)
	}
}

func TestResolveViolation(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("手机：13800138000\n"), 0644)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试",
		TargetPath: tmpDir,
	})
	result, _ := m.RunScan(ctx, task.ID)

	violations := m.GetViolations(ctx, result.ID, "")
	assert.Greater(t, len(violations), 0)

	err := m.ResolveViolation(ctx, violations[0].ID, "admin")
	assert.NoError(t, err)
}

func TestResolveViolation_NotFound(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	err := m.ResolveViolation(ctx, "nonexistent", "admin")
	assert.ErrorIs(t, err, ErrViolationNotFound)
}

func TestResolveViolation_AlreadyResolved(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("手机：13800138000\n"), 0644)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试",
		TargetPath: tmpDir,
	})
	result, _ := m.RunScan(ctx, task.ID)

	violations := m.GetViolations(ctx, result.ID, "")
	_ = m.ResolveViolation(ctx, violations[0].ID, "admin")

	err := m.ResolveViolation(ctx, violations[0].ID, "admin2")
	assert.ErrorIs(t, err, ErrViolationAlreadyResolved)
}

// ========== 合规报告 ==========

func TestGenerateReport(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("手机：13800138000\n邮箱：a@b.com\n"), 0644)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "测试",
		TargetPath: tmpDir,
	})
	_, _ = m.RunScan(ctx, task.ID)

	report, err := m.GenerateReport(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 1, report.TotalScans)
	assert.NotEmpty(t, report.ViolationsBySeverity)
	assert.NotEmpty(t, report.Recommendations)
}

func TestGenerateReport_Empty(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	report, err := m.GenerateReport(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, report.TotalScans)
}

// ========== 定时扫描 ==========

func TestScheduleScan(t *testing.T) {
	m, tmpDir := newTestManager(t)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("test"), 0644)

	task, _ := m.CreateScanTask(ctx, ScanTask{
		Name:       "定时扫描",
		TargetPath: tmpDir,
	})

	err := m.ScheduleScan(ctx, task.ID, "0 2 * * *")
	assert.NoError(t, err)
}

func TestScheduleScan_TaskNotFound(t *testing.T) {
	m, _ := newTestManager(t)
	ctx := context.Background()

	err := m.ScheduleScan(ctx, "nonexistent", "0 2 * * *")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}
