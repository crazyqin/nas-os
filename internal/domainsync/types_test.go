package domainsync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== types_test.go ==========

func TestDefaultSyncConfig(t *testing.T) {
	cfg := DefaultSyncConfig()

	assert.Equal(t, 389, cfg.DCConfig.Port)
	assert.Equal(t, 10*time.Second, cfg.DCConfig.ConnectTimeout)
	assert.Equal(t, SyncStrategyFull, cfg.Strategy)
	assert.True(t, cfg.SyncUsers)
	assert.True(t, cfg.SyncGroups)
	assert.Equal(t, "merge", cfg.ConflictResolution)
	assert.Equal(t, 5, cfg.PoolSize)
}

func TestSyncStrategyConstants(t *testing.T) {
	assert.Equal(t, SyncStrategy("full"), SyncStrategyFull)
	assert.Equal(t, SyncStrategy("incremental"), SyncStrategyIncremental)
	assert.Equal(t, SyncStrategy("scheduled"), SyncStrategyScheduled)
}

func TestSyncStatusConstants(t *testing.T) {
	assert.Equal(t, SyncStatus("idle"), SyncStatusIdle)
	assert.Equal(t, SyncStatus("running"), SyncStatusRunning)
	assert.Equal(t, SyncStatus("completed"), SyncStatusCompleted)
	assert.Equal(t, SyncStatus("failed"), SyncStatusFailed)
	assert.Equal(t, SyncStatus("cancelled"), SyncStatusCancelled)
}

func TestOU(t *testing.T) {
	ou := OU{
		DN:          "OU=Users,DC=example,DC=com",
		Name:        "Users",
		Description: "用户 OU",
		ParentDN:    "DC=example,DC=com",
		Level:       1,
		Enabled:     true,
		UserCount:   100,
		GroupCount:  5,
	}

	assert.Equal(t, "OU=Users,DC=example,DC=com", ou.DN)
	assert.Equal(t, "Users", ou.Name)
	assert.Equal(t, 1, ou.Level)
	assert.True(t, ou.Enabled)
}

func TestDCConfig(t *testing.T) {
	cfg := DCConfig{
		Host:           "dc.example.com",
		Port:           636,
		Domain:         "example.com",
		BaseDN:         "DC=example,DC=com",
		BindDN:         "CN=admin,DC=example,DC=com",
		BindPassword:   "secret",
		UseTLS:         true,
		SkipTLSVerify:  false,
		ConnectTimeout: 15 * time.Second,
	}

	assert.Equal(t, "dc.example.com", cfg.Host)
	assert.Equal(t, 636, cfg.Port)
	assert.True(t, cfg.UseTLS)
	assert.False(t, cfg.SkipTLSVerify)
}

func TestSyncResult(t *testing.T) {
	now := time.Now()
	result := SyncResult{
		ID:            "test-sync-001",
		StartTime:     now,
		EndTime:       now.Add(5 * time.Second),
		Duration:      5 * time.Second,
		Status:        SyncStatusCompleted,
		Strategy:      SyncStrategyFull,
		UsersSynced:   50,
		GroupsSynced:  10,
		OUSynced:      3,
		UsersCreated:  5,
		UsersUpdated:  45,
		GroupsCreated: 2,
		GroupsUpdated: 8,
		Progress:      100,
		Message:       "同步完成",
		Success:       true,
	}

	assert.Equal(t, "test-sync-001", result.ID)
	assert.Equal(t, SyncStatusCompleted, result.Status)
	assert.Equal(t, 50, result.UsersSynced)
	assert.Equal(t, 100, result.Progress)
}

func TestSyncError(t *testing.T) {
	e := SyncError{
		Type:    "user",
		DN:      "CN=test,DC=example,DC=com",
		Message: "用户同步失败",
		Code:    "SYNC_ERR_001",
	}

	assert.Equal(t, "user", e.Type)
	assert.Equal(t, "SYNC_ERR_001", e.Code)
}

func TestDomainSyncStatus(t *testing.T) {
	now := time.Now()
	lastResult := &SyncResult{
		ID:     "last-sync",
		Status: SyncStatusCompleted,
	}

	status := DomainSyncStatus{
		Enabled:       true,
		Status:        SyncStatusIdle,
		Strategy:      SyncStrategyIncremental,
		LastSyncTime:  &now,
		LastSyncID:    "last-sync",
		DCConnected:   true,
		OUCount:       5,
		SelectedCount: 3,
		LastResult:    lastResult,
	}

	assert.True(t, status.Enabled)
	assert.Equal(t, SyncStatusIdle, status.Status)
	assert.True(t, status.DCConnected)
	assert.Equal(t, 5, status.OUCount)
	assert.Equal(t, 3, status.SelectedCount)
	assert.NotNil(t, status.LastResult)
}

func TestSyncConfigWithSelectedOUs(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.SelectedOUs = []string{
		"OU=Engineering,DC=example,DC=com",
		"OU=Marketing,DC=example,DC=com",
	}

	require.Len(t, cfg.SelectedOUs, 2)
	assert.Equal(t, "OU=Engineering,DC=example,DC=com", cfg.SelectedOUs[0])
}
