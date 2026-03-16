package ldap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== NewManagerWithConfig 测试 ==========

func TestNewManagerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ldap.json")

	configs := map[string]Config{
		"test-ldap": {
			Name:       "test-ldap",
			URL:        "ldap://localhost:389",
			BaseDN:     "dc=example,dc=com",
			ServerType: ServerTypeOpenLDAP,
			Enabled:    true,
		},
		"test-ad": {
			Name:       "test-ad",
			URL:        "ldaps://ad.example.com:636",
			BaseDN:     "DC=example,DC=com",
			ServerType: ServerTypeAD,
			Enabled:    true,
		},
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	manager, err := NewManagerWithConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// 验证配置已加载
	config, err := manager.GetConfig("test-ldap")
	require.NoError(t, err)
	assert.Equal(t, "test-ldap", config.Name)

	adConfig, err := manager.GetConfig("test-ad")
	require.NoError(t, err)
	assert.Equal(t, ServerTypeAD, adConfig.ServerType)
}

func TestNewManagerWithConfigNoFile(t *testing.T) {
	manager, err := NewManagerWithConfig("/nonexistent/path/ldap.json")
	require.NoError(t, err)
	require.NotNil(t, manager)
}

func TestNewManagerWithConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(configPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	_, err = NewManagerWithConfig(configPath)
	assert.Error(t, err)
}

// ========== Manager 方法测试 ==========

func TestManagerUpdateConfig(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test-update",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}
	err := manager.RegisterConfig(config)
	require.NoError(t, err)

	// 更新配置
	updated := Config{
		URL:        "ldap://newhost:389",
		BaseDN:     "dc=new,dc=com",
		ServerType: ServerTypeAD,
		Enabled:    true,
	}
	err = manager.UpdateConfig("test-update", updated)
	require.NoError(t, err)

	config2, err := manager.GetConfig("test-update")
	require.NoError(t, err)
	assert.Equal(t, "ldap://newhost:389", config2.URL)
	assert.Equal(t, "dc=new,dc=com", config2.BaseDN)
	assert.Equal(t, ServerTypeAD, config2.ServerType)
}

func TestManagerUpdateConfigNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.UpdateConfig("nonexistent", Config{})
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerDeleteConfigNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.DeleteConfig("nonexistent")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerEnableConfigNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.EnableConfig("nonexistent")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerDisableConfigNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.DisableConfig("nonexistent")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerListEnabledConfigs(t *testing.T) {
	manager := NewManager()

	configs := []Config{
		{Name: "enabled1", URL: "ldap://localhost:389", BaseDN: "dc=example,dc=com", ServerType: ServerTypeOpenLDAP, Enabled: true},
		{Name: "enabled2", URL: "ldap://localhost:390", BaseDN: "dc=example,dc=org", ServerType: ServerTypeOpenLDAP, Enabled: true},
		{Name: "disabled1", URL: "ldap://localhost:391", BaseDN: "dc=example,dc=net", ServerType: ServerTypeOpenLDAP, Enabled: false},
	}

	for _, c := range configs {
		err := manager.RegisterConfig(c)
		require.NoError(t, err)
	}

	enabled := manager.ListEnabledConfigs()
	assert.Len(t, enabled, 2)
}

func TestManagerGetStats(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test-stats",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}
	err := manager.RegisterConfig(config)
	require.NoError(t, err)

	stats := manager.GetStats()
	assert.Equal(t, 1, stats["total_configs"])
	assert.Equal(t, 1, stats["enabled_configs"])
	assert.Equal(t, 0, stats["active_pools"])
	assert.Equal(t, 0, stats["active_syncs"])
}

func TestManagerClose(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test-close",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}
	err := manager.RegisterConfig(config)
	require.NoError(t, err)

	err = manager.Close()
	assert.NoError(t, err)

	// 验证映射已清空
	assert.Empty(t, manager.authenticators)
	assert.Empty(t, manager.adClients)
	assert.Empty(t, manager.pools)
	assert.Empty(t, manager.synchronizers)
}

// ========== Synchronizer 测试 ==========

func TestNewSynchronizer(t *testing.T) {
	config := Config{
		Name:       "test-sync",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	sync, err := NewSynchronizer(config, &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	require.NoError(t, err)
	require.NotNil(t, sync)
}

func TestSynchronizerIsRunning(t *testing.T) {
	config := Config{
		Name:       "test-sync-running",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	sync, err := NewSynchronizer(config, &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	require.NoError(t, err)

	assert.False(t, sync.IsRunning())
}

func TestSynchronizerStop(t *testing.T) {
	config := Config{
		Name:       "test-sync-stop",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	sync, err := NewSynchronizer(config, &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	require.NoError(t, err)

	// 停止未启动的同步器不应报错
	sync.Stop()
	assert.False(t, sync.IsRunning())
}

func TestSynchronizerGetLastSync(t *testing.T) {
	config := Config{
		Name:       "test-sync-last",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	sync, err := NewSynchronizer(config, &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	require.NoError(t, err)

	lastSync, result := sync.GetLastSync()
	assert.True(t, lastSync.IsZero())
	assert.Nil(t, result)
}

func TestSynchronizerGetStatus(t *testing.T) {
	config := Config{
		Name:       "test-sync-status",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	sync, err := NewSynchronizer(config, &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	require.NoError(t, err)

	status := sync.GetStatus()
	assert.False(t, status["running"].(bool))
	assert.Equal(t, "test-sync-status", status["config"])
}

func TestSynchronizerClose(t *testing.T) {
	config := Config{
		Name:       "test-sync-close",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	sync, err := NewSynchronizer(config, &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	require.NoError(t, err)

	err = sync.Close()
	assert.NoError(t, err)
}

func TestSynchronizerMergeMembers(t *testing.T) {
	config := Config{
		Name:       "test-merge",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	sync, err := NewSynchronizer(config, &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	require.NoError(t, err)

	local := []string{"user1", "user2"}
	ldap := []string{"user2", "user3"}

	result := sync.mergeMembers(local, ldap)

	assert.Contains(t, result, "user1")
	assert.Contains(t, result, "user2")
	assert.Contains(t, result, "user3")
	assert.Len(t, result, 3)
}

// ========== Sync 测试 (使用 Manager) ==========

func TestManagerSyncAllConfigNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.SyncAll("nonexistent", &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerStartSyncConfigNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.StartSync(context.Background(), "nonexistent", &mockUserSyncHandler{}, &mockGroupSyncHandler{})
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerStopSync(t *testing.T) {
	manager := NewManager()

	// 停止不存在的同步不应报错
	err := manager.StopSync("nonexistent")
	assert.NoError(t, err)
}

func TestManagerGetSyncStatusNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetSyncStatus("nonexistent")
	assert.Error(t, err)
}

// ========== Pool 测试 ==========

func TestNewPool(t *testing.T) {
	config := Config{
		Name:       "test-pool",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		PoolSize:   5,
	}

	pool, err := NewPool(config)
	require.NoError(t, err)
	require.NotNil(t, pool)
}

func TestPoolStats(t *testing.T) {
	config := Config{
		Name:       "test-pool-stats",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		PoolSize:   10,
	}

	pool, err := NewPool(config)
	require.NoError(t, err)

	stats := pool.Stats()
	assert.Equal(t, 10, stats["pool_size"])
	assert.Equal(t, "test-pool-stats", stats["config_name"])
}

func TestPoolClose(t *testing.T) {
	config := Config{
		Name:       "test-pool-close",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		PoolSize:   5,
	}

	pool, err := NewPool(config)
	require.NoError(t, err)

	pool.Close()
}

func TestManagerGetPool(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test-pool-manager",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		PoolSize:   5,
		Enabled:    true,
	}
	err := manager.RegisterConfig(config)
	require.NoError(t, err)

	pool, err := manager.GetPool("test-pool-manager")
	require.NoError(t, err)
	require.NotNil(t, pool)

	// 再次获取应该返回同一个 pool
	pool2, err := manager.GetPool("test-pool-manager")
	require.NoError(t, err)
	assert.Equal(t, pool, pool2)
}

func TestManagerGetPoolNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetPool("nonexistent")
	assert.Equal(t, ErrConfigNotFound, err)
}

// ========== Client 测试 ==========

func TestNewClient(t *testing.T) {
	config := Config{
		Name:       "test-client",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestClientIsConnected(t *testing.T) {
	config := Config{
		Name:       "test-client-connected",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	assert.False(t, client.IsConnected())
}

func TestClientClose(t *testing.T) {
	config := Config{
		Name:       "test-client-close",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// 关闭未连接的客户端不应报错
	err = client.Close()
	assert.NoError(t, err)
}

func TestClientRawConn(t *testing.T) {
	config := Config{
		Name:       "test-client-raw",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	conn := client.RawConn()
	assert.Nil(t, conn)
}

// ========== ConnectionStatus 测试 ==========

func TestManagerTestConnectionNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.TestConnection("nonexistent")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerTestAllConnections(t *testing.T) {
	manager := NewManager()

	// 没有配置时返回空 map
	results := manager.TestAllConnections()
	assert.Empty(t, results)

	// 添加配置
	config := Config{
		Name:       "test-conn",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}
	err := manager.RegisterConfig(config)
	require.NoError(t, err)

	results = manager.TestAllConnections()
	assert.Len(t, results, 1)
	assert.Contains(t, results, "test-conn")
}

// ========== Authenticate 测试 ==========

func TestManagerAuthenticateConfigNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.Authenticate("nonexistent", "user", "pass")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerAuthenticateDisabled(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test-auth-disabled",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    false,
	}
	err := manager.RegisterConfig(config)
	require.NoError(t, err)

	_, err = manager.Authenticate("test-auth-disabled", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已禁用")
}

// ========== 搜索测试 ==========

func TestManagerSearchUsersConfigNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.SearchUsers("nonexistent", "query")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerGetUserConfigNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetUser("nonexistent", "username")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerGetGroupsConfigNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetGroups("nonexistent")
	assert.Equal(t, ErrConfigNotFound, err)
}

// ========== ADPassword 编码测试 ==========

func TestEncodeADPassword(t *testing.T) {
	password := "TestPassword123!"

	encoded, err := encodeADPassword(password)
	require.NoError(t, err)
	require.NotNil(t, encoded)

	// AD 密码应该是 UTF-16LE 编码
	// 检查长度 (带引号的密码长度 * 2)
	expectedLen := (len(password) + 2) * 2
	assert.Equal(t, expectedLen, len(encoded))
}

// ========== Mock Handlers ==========

type mockUserSyncHandler struct {
	users map[string]*User
}

func (m *mockUserSyncHandler) CreateUser(user *User) error {
	if m.users == nil {
		m.users = make(map[string]*User)
	}
	m.users[user.Username] = user
	return nil
}

func (m *mockUserSyncHandler) UpdateUser(user *User) error {
	if m.users == nil {
		m.users = make(map[string]*User)
	}
	m.users[user.Username] = user
	return nil
}

func (m *mockUserSyncHandler) DeactivateUser(username string) error {
	return nil
}

func (m *mockUserSyncHandler) GetUser(username string) (*User, error) {
	if m.users == nil {
		return nil, nil
	}
	return m.users[username], nil
}

func (m *mockUserSyncHandler) ListUsers() ([]*User, error) {
	if m.users == nil {
		return []*User{}, nil
	}
	users := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}

type mockGroupSyncHandler struct {
	groups map[string]*Group
}

func (m *mockGroupSyncHandler) CreateGroup(group *Group) error {
	if m.groups == nil {
		m.groups = make(map[string]*Group)
	}
	m.groups[group.Name] = group
	return nil
}

func (m *mockGroupSyncHandler) UpdateGroup(group *Group) error {
	if m.groups == nil {
		m.groups = make(map[string]*Group)
	}
	m.groups[group.Name] = group
	return nil
}

func (m *mockGroupSyncHandler) DeleteGroup(name string) error {
	delete(m.groups, name)
	return nil
}

func (m *mockGroupSyncHandler) GetGroup(name string) (*Group, error) {
	if m.groups == nil {
		return nil, nil
	}
	return m.groups[name], nil
}

func (m *mockGroupSyncHandler) ListGroups() ([]*Group, error) {
	if m.groups == nil {
		return []*Group{}, nil
	}
	groups := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups, nil
}

func (m *mockGroupSyncHandler) AddUserToGroup(username, groupName string) error {
	return nil
}

func (m *mockGroupSyncHandler) RemoveUserFromGroup(username, groupName string) error {
	return nil
}
