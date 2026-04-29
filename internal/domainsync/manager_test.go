package domainsync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	require.NotNil(t, m)

	config := m.GetConfig()
	assert.Equal(t, SyncStrategyFull, config.Strategy)
	assert.True(t, config.SyncUsers)
	assert.True(t, config.SyncGroups)
}

func TestNewManagerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "domainsync.json")

	cfg := SyncConfig{
		DCConfig: DCConfig{
			Host:           "dc.example.com",
			Port:           636,
			Domain:         "example.com",
			BaseDN:         "DC=example,DC=com",
			BindDN:         "CN=admin,DC=example,DC=com",
			BindPassword:   "secret",
			UseTLS:         true,
			ConnectTimeout: 15 * time.Second,
		},
		Strategy:           SyncStrategyIncremental,
		SyncUsers:          true,
		SyncGroups:         false,
		ConflictResolution: "overwrite",
		PoolSize:           10,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	m, err := NewManagerWithConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, m)

	loaded := m.GetConfig()
	assert.Equal(t, "dc.example.com", loaded.DCConfig.Host)
	assert.Equal(t, 636, loaded.DCConfig.Port)
	assert.Equal(t, SyncStrategyIncremental, loaded.Strategy)
	assert.False(t, loaded.SyncGroups)
	assert.Equal(t, "overwrite", loaded.ConflictResolution)
	assert.Equal(t, 10, loaded.PoolSize)
}

func TestNewManagerWithConfigNoFile(t *testing.T) {
	m, err := NewManagerWithConfig("/nonexistent/path/domainsync.json")
	require.NoError(t, err)
	require.NotNil(t, m)
}

func TestNewManagerWithConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(configPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	_, err = NewManagerWithConfig(configPath)
	assert.Error(t, err)
}

func TestManagerUpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "domainsync.json")

	m, err := NewManagerWithConfig(configPath)
	require.NoError(t, err)

	cfg := SyncConfig{
		DCConfig: DCConfig{
			Host:   "dc.example.com",
			Domain: "example.com",
			Port:   636,
		},
		Strategy:           SyncStrategyFull,
		SyncUsers:          true,
		SyncGroups:         true,
		ConflictResolution: "merge",
	}

	err = m.UpdateConfig(cfg)
	require.NoError(t, err)

	// 验证配置已保存
	loaded := m.GetConfig()
	assert.Equal(t, "dc.example.com", loaded.DCConfig.Host)
	assert.Equal(t, 636, loaded.DCConfig.Port)

	// 验证文件已写入
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var fileConfig SyncConfig
	err = json.Unmarshal(data, &fileConfig)
	require.NoError(t, err)
	assert.Equal(t, "dc.example.com", fileConfig.DCConfig.Host)
}

func TestManagerUpdateConfigValidation(t *testing.T) {
	m := NewManager()

	// 空 host
	cfg := SyncConfig{
		DCConfig: DCConfig{
			Domain: "example.com",
		},
	}
	err := m.UpdateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域控制器地址不能为空")

	// 空 domain 和 baseDN
	cfg = SyncConfig{
		DCConfig: DCConfig{
			Host: "dc.example.com",
		},
	}
	err = m.UpdateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名和基础 DN 不能同时为空")
}

func TestManagerGetStatus(t *testing.T) {
	m := NewManager()

	status := m.GetStatus()
	require.NotNil(t, status)

	assert.False(t, status.Enabled) // 没有配置 host，应该是 false
	assert.Equal(t, SyncStatusIdle, status.Status)
	assert.Equal(t, SyncStrategyFull, status.Strategy)
	assert.False(t, status.DCConnected) // 没有真实的 DC
}

func TestManagerGetStatusWithConfig(t *testing.T) {
	m := NewManager()

	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"
	cfg.SelectedOUs = []string{
		"OU=Engineering,DC=example,DC=com",
		"OU=Marketing,DC=example,DC=com",
	}

	err := m.UpdateConfig(cfg)
	require.NoError(t, err)

	status := m.GetStatus()
	require.NotNil(t, status)

	assert.True(t, status.Enabled)
	assert.Equal(t, 2, status.OUCount)
	assert.Equal(t, 2, status.SelectedCount)
}

func TestManagerStartSyncNotConnected(t *testing.T) {
	m := NewManager()

	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "nonexistent.local"
	cfg.DCConfig.Domain = "nonexistent.local"
	err := m.UpdateConfig(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = m.StartSync(ctx)
	// 应该失败，因为无法连接到域控制器
	assert.Error(t, err)
}

func TestManagerClose(t *testing.T) {
	m := NewManager()
	err := m.Close()
	assert.NoError(t, err)
}

func TestManagerSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "domainsync.json")

	// 更新配置会自动创建目录
	m, err := NewManagerWithConfig(configPath)
	require.NoError(t, err)

	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.test.com"
	cfg.DCConfig.Domain = "test.com"
	cfg.DCConfig.BindDN = "CN=admin,DC=test,DC=com"
	cfg.DCConfig.BindPassword = "password123"
	cfg.Strategy = SyncStrategyIncremental
	cfg.ScheduleInterval = 30 * time.Minute
	cfg.SelectedOUs = []string{"OU=Test,DC=test,DC=com"}

	err = m.UpdateConfig(cfg)
	require.NoError(t, err)

	// 重新加载
	m2, err := NewManagerWithConfig(configPath)
	require.NoError(t, err)

	loaded := m2.GetConfig()
	assert.Equal(t, "dc.test.com", loaded.DCConfig.Host)
	assert.Equal(t, "test.com", loaded.DCConfig.Domain)
	assert.Equal(t, SyncStrategyIncremental, loaded.Strategy)
	assert.Equal(t, 30*time.Minute, loaded.ScheduleInterval)
	assert.Len(t, loaded.SelectedOUs, 1)
}

func TestManagerConfigWithSelectedOUs(t *testing.T) {
	m := NewManager()

	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"
	cfg.SelectedOUs = []string{
		"OU=Engineering,DC=example,DC=com",
		"OU=HR,DC=example,DC=com",
		"OU=Finance,DC=example,DC=com",
	}

	err := m.UpdateConfig(cfg)
	require.NoError(t, err)

	loaded := m.GetConfig()
	require.Len(t, loaded.SelectedOUs, 3)
	assert.Contains(t, loaded.SelectedOUs, "OU=Engineering,DC=example,DC=com")
	assert.Contains(t, loaded.SelectedOUs, "OU=HR,DC=example,DC=com")
	assert.Contains(t, loaded.SelectedOUs, "OU=Finance,DC=example,DC=com")
}
