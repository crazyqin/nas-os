package ldap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager LDAP 管理器
type Manager struct {
	mu             sync.RWMutex
	configs        map[string]*Config
	authenticators map[string]*Authenticator
	adClients      map[string]*ADClient
	synchronizers  map[string]*Synchronizer
	pools          map[string]*Pool
	configPath     string
}

// NewManager 创建 LDAP 管理器
func NewManager() *Manager {
	return &Manager{
		configs:        make(map[string]*Config),
		authenticators: make(map[string]*Authenticator),
		adClients:      make(map[string]*ADClient),
		synchronizers:  make(map[string]*Synchronizer),
		pools:          make(map[string]*Pool),
	}
}

// NewManagerWithConfig 创建带配置文件的管理器
func NewManagerWithConfig(configPath string) (*Manager, error) {
	m := NewManager()
	m.configPath = configPath

	if err := m.loadConfigs(); err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	return m, nil
}

// loadConfigs 从文件加载配置
func (m *Manager) loadConfigs() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var configs map[string]Config
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	for name, config := range configs {
		config.Name = name
		m.configs[name] = &config
	}

	return nil
}

// saveConfigs 保存配置到文件
func (m *Manager) saveConfigs() error {
	if m.configPath == "" {
		return nil
	}

	configs := make(map[string]Config)
	for name, config := range m.configs {
		configs[name] = *config
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// ========== 配置管理 ==========

// RegisterConfig 注册 LDAP 配置
func (m *Manager) RegisterConfig(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Name == "" {
		return fmt.Errorf("%w: 配置名称不能为空", ErrInvalidConfig)
	}

	// 设置默认值
	m.applyDefaults(&config)

	m.configs[config.Name] = &config

	if err := m.saveConfigs(); err != nil {
		return err
	}

	return nil
}

// UpdateConfig 更新 LDAP 配置
func (m *Manager) UpdateConfig(name string, config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[name]; !exists {
		return ErrConfigNotFound
	}

	config.Name = name
	m.applyDefaults(&config)
	m.configs[name] = &config

	// 清理旧的客户端
	delete(m.authenticators, name)
	delete(m.adClients, name)
	if pool, exists := m.pools[name]; exists {
		_ = pool.Close()
		delete(m.pools, name)
	}
	if sync, exists := m.synchronizers[name]; exists {
		if err := sync.Close(); err != nil {
			// 同步器关闭失败不影响配置更新
			_ = err
		}
		delete(m.synchronizers, name)
	}

	if err := m.saveConfigs(); err != nil {
		return err
	}

	return nil
}

// DeleteConfig 删除 LDAP 配置
func (m *Manager) DeleteConfig(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.configs[name]; !exists {
		return ErrConfigNotFound
	}

	// 清理资源
	delete(m.configs, name)
	delete(m.authenticators, name)
	delete(m.adClients, name)
	if pool, exists := m.pools[name]; exists {
		_ = pool.Close()
		delete(m.pools, name)
	}
	if sync, exists := m.synchronizers[name]; exists {
		if err := sync.Close(); err != nil {
			// 同步器关闭失败不影响配置删除
			_ = err
		}
		delete(m.synchronizers, name)
	}

	if err := m.saveConfigs(); err != nil {
		return err
	}

	return nil
}

// GetConfig 获取 LDAP 配置
func (m *Manager) GetConfig(name string) (*Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.configs[name]
	if !exists {
		return nil, ErrConfigNotFound
	}

	// 返回副本
	copy := *config
	return &copy, nil
}

// ListConfigs 列出所有配置
func (m *Manager) ListConfigs() []Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]Config, 0, len(m.configs))
	for _, config := range m.configs {
		configs = append(configs, *config)
	}
	return configs
}

// ListEnabledConfigs 列出已启用的配置
func (m *Manager) ListEnabledConfigs() []Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]Config, 0)
	for _, config := range m.configs {
		if config.Enabled {
			configs = append(configs, *config)
		}
	}
	return configs
}

// EnableConfig 启用配置
func (m *Manager) EnableConfig(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.configs[name]
	if !exists {
		return ErrConfigNotFound
	}

	config.Enabled = true
	return m.saveConfigs()
}

// DisableConfig 禁用配置
func (m *Manager) DisableConfig(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.configs[name]
	if !exists {
		return ErrConfigNotFound
	}

	config.Enabled = false
	return m.saveConfigs()
}

// applyDefaults 应用默认值
func (m *Manager) applyDefaults(config *Config) {
	defaults := DefaultConfig()

	if config.ServerType == "" {
		config.ServerType = defaults.ServerType
	}
	if config.PoolSize <= 0 {
		config.PoolSize = defaults.PoolSize
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = defaults.IdleTimeout
	}
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = defaults.ConnectTimeout
	}
	if config.OperationTimeout <= 0 {
		config.OperationTimeout = defaults.OperationTimeout
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = defaults.MaxRetries
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = defaults.RetryDelay
	}

	// 应用属性映射默认值
	if config.Attributes.Username == "" {
		if config.ServerType == ServerTypeAD {
			config.Attributes = ADAttributeMapping()
		} else {
			config.Attributes = defaults.Attributes
		}
	}
}

// ========== 认证 ==========

// Authenticate 使用指定配置进行认证
func (m *Manager) Authenticate(configName, username, password string) (*AuthResult, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	if !config.Enabled {
		return nil, fmt.Errorf("LDAP 配置已禁用")
	}

	// 使用 AD 客户端处理 AD 类型
	if config.ServerType == ServerTypeAD {
		adClient, err := m.getADClient(configName)
		if err != nil {
			return nil, err
		}
		return adClient.Authenticate(username, password)
	}

	// 使用通用认证器
	auth, err := m.getAuthenticator(configName)
	if err != nil {
		return nil, err
	}

	return auth.Authenticate(username, password)
}

// getAuthenticator 获取或创建认证器
func (m *Manager) getAuthenticator(configName string) (*Authenticator, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if auth, exists := m.authenticators[configName]; exists {
		return auth, nil
	}

	config, exists := m.configs[configName]
	if !exists {
		return nil, ErrConfigNotFound
	}

	auth, err := NewAuthenticator(*config)
	if err != nil {
		return nil, err
	}

	m.authenticators[configName] = auth
	return auth, nil
}

// getADClient 获取或创建 AD 客户端
func (m *Manager) getADClient(configName string) (*ADClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, exists := m.adClients[configName]; exists {
		return client, nil
	}

	config, exists := m.configs[configName]
	if !exists {
		return nil, ErrConfigNotFound
	}

	client, err := NewADClient(*config)
	if err != nil {
		return nil, err
	}

	m.adClients[configName] = client
	return client, nil
}

// ========== 连接池 ==========

// GetPool 获取连接池
func (m *Manager) GetPool(configName string) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool, exists := m.pools[configName]; exists {
		return pool, nil
	}

	config, exists := m.configs[configName]
	if !exists {
		return nil, ErrConfigNotFound
	}

	pool, err := NewPool(*config)
	if err != nil {
		return nil, err
	}

	m.pools[configName] = pool
	return pool, nil
}

// TestConnection 测试连接
func (m *Manager) TestConnection(configName string) (*ConnectionStatus, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	status := &ConnectionStatus{
		ConfigName:  configName,
		LastChecked: time.Now(),
	}

	start := time.Now()
	client, err := NewClient(*config)
	if err != nil {
		status.Error = err.Error()
		return status, err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			// 测试连接后关闭失败不影响结果
			_ = closeErr
		}
	}()

	if err := client.Bind(); err != nil {
		status.Error = err.Error()
		return status, err
	}

	status.Connected = true
	status.Latency = time.Since(start)

	return status, nil
}

// TestAllConnections 测试所有连接
func (m *Manager) TestAllConnections() map[string]*ConnectionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]*ConnectionStatus)
	for name := range m.configs {
		status, _ := m.TestConnection(name) //nolint:errcheck // 错误信息已包含在 status.Error 中
		results[name] = status
	}

	return results
}

// ========== 同步 ==========

// StartSync 启动同步
func (m *Manager) StartSync(ctx context.Context, configName string, userHandler UserSyncHandler, groupHandler GroupSyncHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.configs[configName]
	if !exists {
		return ErrConfigNotFound
	}

	if !config.SyncConfig.Enabled {
		return fmt.Errorf("同步未启用")
	}

	if sync, exists := m.synchronizers[configName]; exists && sync.IsRunning() {
		return fmt.Errorf("同步已在运行")
	}

	sync, err := NewSynchronizer(*config, userHandler, groupHandler)
	if err != nil {
		return err
	}

	if err := sync.Start(ctx); err != nil {
		return err
	}

	m.synchronizers[configName] = sync
	return nil
}

// StopSync 停止同步
func (m *Manager) StopSync(configName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sync, exists := m.synchronizers[configName]
	if !exists {
		return nil
	}

	sync.Stop()
	delete(m.synchronizers, configName)
	return nil
}

// SyncAll 执行一次性全量同步
func (m *Manager) SyncAll(configName string, userHandler UserSyncHandler, groupHandler GroupSyncHandler) (*SyncResult, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	sync, err := NewSynchronizer(*config, userHandler, groupHandler)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := sync.Close(); closeErr != nil {
			// 同步器关闭失败，不影响主流程
			_ = closeErr
		}
	}()

	return sync.SyncAll()
}

// GetSyncStatus 获取同步状态
func (m *Manager) GetSyncStatus(configName string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sync, exists := m.synchronizers[configName]
	if !exists {
		return nil, fmt.Errorf("同步未运行")
	}

	return sync.GetStatus(), nil
}

// ========== 用户操作 ==========

// SearchUsers 搜索用户
func (m *Manager) SearchUsers(configName, query string) ([]*User, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	client, err := NewClient(*config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			_ = closeErr // 客户端关闭失败，不影响主流程
		}
	}()

	if err := client.Bind(); err != nil {
		return nil, err
	}

	// 构建搜索过滤器
	filter := fmt.Sprintf("(|(%s=*%s*)(%s=*%s*))",
		config.Attributes.Username,
		escapeLDAP(query),
		config.Attributes.Email,
		escapeLDAP(query))

	if config.UserFilter != "" {
		filter = fmt.Sprintf("(&%s%s)", config.UserFilter, filter)
	}

	searchDN := config.BaseDN
	if config.UserSearchDN != "" {
		searchDN = config.UserSearchDN
	}

	searchRequest := createSearchRequest(searchDN, filter, []string{
		config.Attributes.Username,
		config.Attributes.Email,
		config.Attributes.FullName,
		"dn",
	})

	result, err := client.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	users := make([]*User, 0, len(result.Entries))
	for _, entry := range result.Entries {
		users = append(users, &User{
			DN:       entry.DN,
			Username: entry.GetAttributeValue(config.Attributes.Username),
			Email:    entry.GetAttributeValue(config.Attributes.Email),
			FullName: entry.GetAttributeValue(config.Attributes.FullName),
		})
	}

	return users, nil
}

// GetUser 获取用户详细信息
func (m *Manager) GetUser(configName, username string) (*User, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	if config.ServerType == ServerTypeAD {
		adClient, err := m.getADClient(configName)
		if err != nil {
			return nil, err
		}

		info, err := adClient.GetUserAccountInfo(username)
		if err != nil {
			return nil, err
		}

		return &User{
			DN:          info.DN,
			Username:    info.SAMAccountName,
			Email:       info.UserPrincipalName,
			DisplayName: info.DisplayName,
			Disabled:    info.Disabled,
			Locked:      info.Locked,
			LastLogin:   info.LastLogon,
		}, nil
	}

	auth, err := m.getAuthenticator(configName)
	if err != nil {
		return nil, err
	}

	// 搜索用户
	if err := auth.client.Bind(); err != nil {
		return nil, err
	}

	user, err := auth.findUser(username)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	// 获取用户组
	groups, err := auth.getUserGroups(user.DN)
	if err == nil {
		user.Groups = groups
	}

	return user, nil
}

// ========== 组操作 ==========

// GetGroups 获取组列表
func (m *Manager) GetGroups(configName string) ([]*Group, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	client, err := NewClient(*config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			_ = closeErr // 客户端关闭失败，不影响主流程
		}
	}()

	if err := client.Bind(); err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("(objectClass=%s)", config.Attributes.GroupObjectClass)

	searchDN := config.BaseDN
	if config.GroupSearchDN != "" {
		searchDN = config.GroupSearchDN
	}

	searchRequest := createSearchRequest(searchDN, filter, []string{
		config.Attributes.GroupName,
		config.Attributes.GroupDescription,
		config.Attributes.GroupMember,
		"dn",
	})

	result, err := client.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	groups := make([]*Group, 0, len(result.Entries))
	for _, entry := range result.Entries {
		groups = append(groups, &Group{
			DN:          entry.DN,
			Name:        entry.GetAttributeValue(config.Attributes.GroupName),
			Description: entry.GetAttributeValue(config.Attributes.GroupDescription),
			Members:     entry.GetAttributeValues(config.Attributes.GroupMember),
		})
	}

	return groups, nil
}

// ========== 关闭 ==========

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭所有认证器
	for _, auth := range m.authenticators {
		if err := auth.Close(); err != nil {
			_ = err // 记录错误但继续关闭其他资源
		}
	}

	// 关闭所有 AD 客户端
	for _, client := range m.adClients {
		if err := client.Close(); err != nil {
			_ = err // 记录错误但继续关闭其他资源
		}
	}

	// 关闭所有连接池
	for _, pool := range m.pools {
		if err := pool.Close(); err != nil {
			_ = err // 记录错误但继续关闭其他资源
		}
	}

	// 停止所有同步器
	for _, sync := range m.synchronizers {
		if err := sync.Close(); err != nil {
			_ = err // 记录错误但继续关闭其他资源
		}
	}

	// 清空映射
	m.authenticators = make(map[string]*Authenticator)
	m.adClients = make(map[string]*ADClient)
	m.pools = make(map[string]*Pool)
	m.synchronizers = make(map[string]*Synchronizer)

	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_configs":   len(m.configs),
		"enabled_configs": len(m.ListEnabledConfigs()),
		"active_pools":    len(m.pools),
		"active_syncs":    len(m.synchronizers),
		"config_names":    m.getConfigNames(),
	}
}

func (m *Manager) getConfigNames() []string {
	names := make([]string, 0, len(m.configs))
	for name := range m.configs {
		names = append(names, name)
	}
	return names
}

// 辅助函数

func escapeLDAP(s string) string {
	// LDAP 过滤器特殊字符转义
	replacer := strings.NewReplacer(
		"\\", "\\5c",
		"*", "\\2a",
		"(", "\\28",
		")", "\\29",
		"\x00", "\\00",
	)
	return replacer.Replace(s)
}
