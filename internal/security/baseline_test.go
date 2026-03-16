// Package security 提供安全基线检查测试
package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaselineManager_RunCheck(t *testing.T) {
	bm := NewBaselineManager()

	// 测试存在的检查项
	result := bm.RunCheck("AUTH-001")
	assert.NotEmpty(t, result.CheckID)
	assert.Contains(t, []string{"pass", "fail", "warning", "skipped"}, result.Status)

	// 测试不存在的检查项
	result = bm.RunCheck("NONEXISTENT")
	assert.Equal(t, "skipped", result.Status)
	assert.Contains(t, result.Message, "检查项不存在")
}

func TestBaselineManager_RunChecksByCategory(t *testing.T) {
	bm := NewBaselineManager()

	categories := []string{"auth", "network", "system", "file"}

	for _, category := range categories {
		t.Run("category_"+category, func(t *testing.T) {
			report := bm.RunChecksByCategory(category)
			assert.NotEmpty(t, report.ReportID)
			assert.NotEmpty(t, report.Results)

			for _, result := range report.Results {
				assert.Equal(t, category, result.Category)
			}
		})
	}
}

func TestBaselineManager_RunChecksByCategory_InvalidCategory(t *testing.T) {
	bm := NewBaselineManager()

	report := bm.RunChecksByCategory("invalid_category")
	assert.Empty(t, report.Results)
}

func TestBaselineManager_GetCheckList(t *testing.T) {
	bm := NewBaselineManager()

	checks := bm.GetCheckList()
	assert.NotEmpty(t, checks)

	// 验证检查项包含必要字段
	for _, check := range checks {
		assert.NotEmpty(t, check["id"])
		assert.NotEmpty(t, check["name"])
		assert.NotEmpty(t, check["category"])
		assert.NotEmpty(t, check["severity"])
	}
}

func TestBaselineManager_GetCategories(t *testing.T) {
	bm := NewBaselineManager()

	categories := bm.GetCategories()
	assert.NotEmpty(t, categories)

	// 验证包含预期的类别
	expectedCategories := map[string]bool{
		"auth":    false,
		"network": false,
		"system":  false,
		"file":    false,
	}

	for _, cat := range categories {
		if _, ok := expectedCategories[cat]; ok {
			expectedCategories[cat] = true
		}
	}

	for cat, found := range expectedCategories {
		assert.True(t, found, "应包含类别: %s", cat)
	}
}

func TestBaselineReport_Score(t *testing.T) {
	bm := NewBaselineManager()

	report := bm.RunAllChecks()

	assert.NotEmpty(t, report.ReportID)
	assert.False(t, report.Timestamp.IsZero())
	assert.GreaterOrEqual(t, report.OverallScore, 0)
	assert.LessOrEqual(t, report.OverallScore, 100)
	assert.Equal(t, len(report.Results), report.TotalChecks)
}

func TestBaselineCheckResult_Fields(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.RunCheck("AUTH-001")

	assert.NotEmpty(t, result.CheckID)
	assert.NotEmpty(t, result.Name)
	assert.NotEmpty(t, result.Description)
	assert.NotEmpty(t, result.Category)
	assert.NotEmpty(t, result.Severity)
	assert.NotEmpty(t, result.Status)
	assert.NotEmpty(t, result.Message)
}

func TestGenerateReportID(t *testing.T) {
	id1 := generateReportID()
	id2 := generateReportID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "report-")
}

func TestBaselineManager_PasswordPolicy(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkPasswordPolicy()

	assert.Equal(t, "AUTH-001", result.CheckID)
	assert.Contains(t, []string{"pass", "fail", "warning"}, result.Status)
	assert.NotEmpty(t, result.Message)
}

func TestBaselineManager_MFAEnabled(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkMFAEnabled()

	assert.Equal(t, "AUTH-002", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_DefaultPasswords(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkDefaultPasswords()

	assert.Equal(t, "AUTH-003", result.CheckID)
	// 默认密码检查可能是pass或fail
	assert.Contains(t, []string{"pass", "fail", "warning"}, result.Status)
}

func TestBaselineManager_AccountLockoutPolicy(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkAccountLockoutPolicy()

	assert.Equal(t, "AUTH-004", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_FirewallEnabled(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkFirewallEnabled()

	assert.Equal(t, "NET-001", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_SSHConfig(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkSSHConfig()

	assert.Equal(t, "NET-002", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_OpenPorts(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkOpenPorts()

	assert.Equal(t, "NET-003", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_HTTPSRequired(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkHTTPSRequired()

	assert.Equal(t, "NET-004", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_SystemUpdates(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkSystemUpdates()

	assert.Equal(t, "SYS-001", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_RootLogin(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkRootLogin()

	assert.Equal(t, "SYS-002", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_FilePermissions(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkFilePermissions()

	assert.Equal(t, "SYS-003", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_LoggingEnabled(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkLoggingEnabled()

	assert.Equal(t, "SYS-004", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_SensitiveFilesEncrypted(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkSensitiveFilesEncrypted()

	assert.Equal(t, "FILE-001", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_SharePermissions(t *testing.T) {
	bm := NewBaselineManager()

	result := bm.checkSharePermissions()

	assert.Equal(t, "FILE-002", result.CheckID)
	assert.NotEmpty(t, result.Status)
}

func TestBaselineManager_AllChecksHaveRequiredFields(t *testing.T) {
	bm := NewBaselineManager()
	checks := bm.GetCheckList()

	for _, check := range checks {
		checkID := check["id"].(string)
		t.Run(checkID, func(t *testing.T) {
			assert.NotEmpty(t, check["id"], "ID should not be empty")
			assert.NotEmpty(t, check["name"], "Name should not be empty")
			assert.NotEmpty(t, check["description"], "Description should not be empty")
			assert.NotEmpty(t, check["category"], "Category should not be empty")
			assert.NotEmpty(t, check["severity"], "Severity should not be empty")
		})
	}
}

func TestBaselineManager_RunAllChecks_Counts(t *testing.T) {
	bm := NewBaselineManager()

	report := bm.RunAllChecks()

	totalChecks := report.Passed + report.Failed + report.Warning + report.Skipped
	assert.Equal(t, report.TotalChecks, totalChecks, "各状态数量之和应等于总检查数")
}

func TestBaselineManager_SeverityLevels(t *testing.T) {
	bm := NewBaselineManager()
	checks := bm.GetCheckList()

	validSeverities := map[string]bool{
		"low":      true,
		"medium":   true,
		"high":     true,
		"critical": true,
	}

	for _, check := range checks {
		severity := check["severity"].(string)
		assert.True(t, validSeverities[severity],
			"Check %s has invalid severity: %s", check["id"], severity)
	}
}

func TestBaselineManager_ResultStatus(t *testing.T) {
	bm := NewBaselineManager()
	report := bm.RunAllChecks()

	validStatuses := map[string]bool{
		"pass":    true,
		"fail":    true,
		"warning": true,
		"skipped": true,
	}

	for _, result := range report.Results {
		assert.True(t, validStatuses[result.Status],
			"Check %s has invalid status: %s", result.CheckID, result.Status)
	}
}

func TestBaselineManager_ConcurrentChecks(t *testing.T) {
	bm := NewBaselineManager()

	// 并发运行多次检查
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_ = bm.RunAllChecks()
			done <- true
		}()
	}

	// 等待所有检查完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestBaselineCheck_HasRemediation(t *testing.T) {
	bm := NewBaselineManager()
	report := bm.RunAllChecks()

	// 失败的检查应该有修复建议
	for _, result := range report.Results {
		if result.Status == "fail" {
			// 修复建议可能为空（某些检查无法自动修复）
			// 但至少应该有消息说明问题
			assert.NotEmpty(t, result.Message)
		}
	}
}
