package securitycompliance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), t.TempDir())
}

func TestRunSecurityScan(t *testing.T) {
	mgr := setupTestManager(t)
	report := mgr.RunSecurityScan()
	assert.True(t, report.TotalChecks >= 10)
	assert.True(t, report.Score >= 0)
	assert.NotEmpty(t, report.Checks)
}

func TestAddAuditLog(t *testing.T) {
	mgr := setupTestManager(t)
	mgr.AddAuditLog("admin", "login", "/api", "success", "192.168.1.1", "")
	logs := mgr.GetAuditLogs("", 10)
	assert.Len(t, logs, 1)
	assert.Equal(t, "admin", logs[0].User)
}

func TestGetAuditLogsByUser(t *testing.T) {
	mgr := setupTestManager(t)
	mgr.AddAuditLog("admin", "login", "/", "ok", "1.1.1.1", "")
	mgr.AddAuditLog("user", "download", "/file", "ok", "2.2.2.2", "")
	logs := mgr.GetAuditLogs("admin", 10)
	assert.Len(t, logs, 1)
	assert.Equal(t, "admin", logs[0].User)
}

func TestGetLatestReport(t *testing.T) {
	mgr := setupTestManager(t)
	assert.Nil(t, mgr.GetLatestReport())
	_ = mgr.RunSecurityScan()
	report := mgr.GetLatestReport()
	require.NotNil(t, report)
	assert.True(t, report.TotalChecks > 0)
}

func TestGetAuditLogsLimit(t *testing.T) {
	mgr := setupTestManager(t)
	for i := 0; i < 10; i++ {
		mgr.AddAuditLog("admin", "action", "/", "ok", "1.1.1.1", "")
	}
	logs := mgr.GetAuditLogs("", 5)
	assert.Len(t, logs, 5)
}
