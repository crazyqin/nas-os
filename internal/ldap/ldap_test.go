package ldap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 类型测试 ==========

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, ServerTypeGeneric, config.ServerType)
	assert.True(t, config.Enabled)
	assert.True(t, config.UseTLS)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 30*time.Minute, config.IdleTimeout)
	assert.Equal(t, 10*time.Second, config.ConnectTimeout)
	assert.Equal(t, 30*time.Second, config.OperationTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
}

func TestDefaultAttributeMapping(t *testing.T) {
	attrs := DefaultAttributeMapping()

	assert.Equal(t, "uid", attrs.Username)
	assert.Equal(t, "mail", attrs.Email)
	assert.Equal(t, "givenName", attrs.FirstName)
	assert.Equal(t, "sn", attrs.LastName)
	assert.Equal(t, "cn", attrs.FullName)
	assert.Equal(t, "displayName", attrs.DisplayName)
	assert.Equal(t, "inetOrgPerson", attrs.UserObjectClass)
	assert.Equal(t, "groupOfNames", attrs.GroupObjectClass)
}

func TestADAttributeMapping(t *testing.T) {
	attrs := ADAttributeMapping()

	assert.Equal(t, "sAMAccountName", attrs.Username)
	assert.Equal(t, "mail", attrs.Email)
	assert.Equal(t, "givenName", attrs.FirstName)
	assert.Equal(t, "sn", attrs.LastName)
	assert.Equal(t, "user", attrs.UserObjectClass)
	assert.Equal(t, "group", attrs.GroupObjectClass)
}

func TestDefaultSyncConfig(t *testing.T) {
	syncConfig := DefaultSyncConfig()

	assert.False(t, syncConfig.Enabled)
	assert.Equal(t, SyncModeFull, syncConfig.Mode)
	assert.Equal(t, SyncDirectionImport, syncConfig.Direction)
	assert.Equal(t, 1*time.Hour, syncConfig.Interval)
	assert.True(t, syncConfig.SyncUsers)
	assert.True(t, syncConfig.CreateUsers)
	assert.True(t, syncConfig.UpdateUsers)
	assert.True(t, syncConfig.SyncGroups)
	assert.Equal(t, "merge", syncConfig.ConflictResolution)
}

// ========== 配置构建测试 ==========

func TestBuildADConfig(t *testing.T) {
	config := BuildADConfig(
		"test-ad",
		"ldaps://ad.example.com:636",
		"CN=admin,DC=example,DC=com",
		"password",
		"DC=example,DC=com",
		"example.com",
	)

	assert.Equal(t, "test-ad", config.Name)
	assert.Equal(t, ServerTypeAD, config.ServerType)
	assert.Equal(t, "ldaps://ad.example.com:636", config.URL)
	assert.Equal(t, "CN=admin,DC=example,DC=com", config.BindDN)
	assert.Equal(t, "DC=example,DC=com", config.BaseDN)
	assert.Equal(t, "example.com", config.DomainName)
	assert.True(t, config.Enabled)
	assert.True(t, config.UseTLS)

	// 验证 AD 属性映射
	assert.Equal(t, "sAMAccountName", config.Attributes.Username)
	assert.Equal(t, "user", config.Attributes.UserObjectClass)
}

func TestBuildOpenLDAPConfig(t *testing.T) {
	config := BuildOpenLDAPConfig(
		"test-openldap",
		"ldaps://ldap.example.com:636",
		"cn=admin,dc=example,dc=com",
		"password",
		"dc=example,dc=com",
	)

	assert.Equal(t, "test-openldap", config.Name)
	assert.Equal(t, ServerTypeOpenLDAP, config.ServerType)
	assert.Equal(t, "ldaps://ldap.example.com:636", config.URL)
	assert.Equal(t, "cn=admin,dc=example,dc=com", config.BindDN)
	assert.Equal(t, "dc=example,dc=com", config.BaseDN)
	assert.True(t, config.Enabled)
	assert.True(t, config.UseTLS)

	// 验证 OpenLDAP 属性映射
	assert.Equal(t, "uid", config.Attributes.Username)
	assert.Equal(t, "inetOrgPerson", config.Attributes.UserObjectClass)
}

func TestBuildFreeIPAConfig(t *testing.T) {
	config := BuildFreeIPAConfig(
		"test-freeipa",
		"ldaps://ipa.example.com:636",
		"cn=admin,cn=users,cn=accounts,dc=example,dc=com",
		"password",
		"dc=example,dc=com",
	)

	assert.Equal(t, "test-freeipa", config.Name)
	assert.Equal(t, ServerTypeFreeIPA, config.ServerType)
	assert.Equal(t, "ldaps://ipa.example.com:636", config.URL)
	assert.True(t, config.Enabled)

	// 验证 FreeIPA 属性映射
	assert.Equal(t, "uid", config.Attributes.Username)
	assert.Equal(t, "posixAccount", config.Attributes.UserObjectClass)
	assert.Equal(t, "posixGroup", config.Attributes.GroupObjectClass)
	assert.Equal(t, "memberUid", config.Attributes.MemberAttribute)
}

// ========== 管理器测试 ==========

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.configs)
	assert.NotNil(t, manager.authenticators)
	assert.NotNil(t, manager.adClients)
	assert.NotNil(t, manager.synchronizers)
	assert.NotNil(t, manager.pools)
}

func TestManagerRegisterConfig(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	err := manager.RegisterConfig(config)
	assert.NoError(t, err)

	// 验证配置已注册
	registered, err := manager.GetConfig("test")
	assert.NoError(t, err)
	assert.Equal(t, "test", registered.Name)
	assert.Equal(t, "ldap://localhost:389", registered.URL)

	// 验证默认值已应用
	assert.Equal(t, 10, registered.PoolSize)
	assert.Equal(t, 30*time.Minute, registered.IdleTimeout)
}

func TestManagerGetConfigNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetConfig("nonexistent")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerDeleteConfig(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test-delete",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    true,
	}

	err := manager.RegisterConfig(config)
	assert.NoError(t, err)

	err = manager.DeleteConfig("test-delete")
	assert.NoError(t, err)

	_, err = manager.GetConfig("test-delete")
	assert.Equal(t, ErrConfigNotFound, err)
}

func TestManagerListConfigs(t *testing.T) {
	manager := NewManager()

	configs := []Config{
		{Name: "config1", URL: "ldap://localhost:389", BaseDN: "dc=example,dc=com", ServerType: ServerTypeOpenLDAP},
		{Name: "config2", URL: "ldap://localhost:390", BaseDN: "dc=example,dc=org", ServerType: ServerTypeAD},
	}

	for _, c := range configs {
		c.Enabled = true
		err := manager.RegisterConfig(c)
		assert.NoError(t, err)
	}

	list := manager.ListConfigs()
	assert.Len(t, list, 2)
}

func TestManagerEnableDisable(t *testing.T) {
	manager := NewManager()

	config := Config{
		Name:       "test-toggle",
		URL:        "ldap://localhost:389",
		BaseDN:     "dc=example,dc=com",
		ServerType: ServerTypeOpenLDAP,
		Enabled:    false,
	}

	err := manager.RegisterConfig(config)
	assert.NoError(t, err)

	// 启用
	err = manager.EnableConfig("test-toggle")
	assert.NoError(t, err)

	enabled, err := manager.GetConfig("test-toggle")
	assert.NoError(t, err)
	assert.True(t, enabled.Enabled)

	// 禁用
	err = manager.DisableConfig("test-toggle")
	assert.NoError(t, err)

	disabled, err := manager.GetConfig("test-toggle")
	assert.NoError(t, err)
	assert.False(t, disabled.Enabled)
}

// ========== 辅助函数测试 ==========

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "valid config",
			config: Config{
				Name:   "test",
				URL:    "ldap://localhost:389",
				BaseDN: "dc=example,dc=com",
			},
			wantErr: nil,
		},
		{
			name: "missing name",
			config: Config{
				URL:    "ldap://localhost:389",
				BaseDN: "dc=example,dc=com",
			},
			wantErr: ErrInvalidConfig,
		},
		{
			name: "missing URL",
			config: Config{
				Name:   "test",
				BaseDN: "dc=example,dc=com",
			},
			wantErr: ErrInvalidConfig,
		},
		{
			name: "missing BaseDN",
			config: Config{
				Name: "test",
				URL:  "ldap://localhost:389",
			},
			wantErr: ErrInvalidConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestMergeAttributeMaps(t *testing.T) {
	base := DefaultAttributeMapping()
	override := AttributeMapping{
		Username:   "sAMAccountName",
		Email:      "mail",
		Department: "department",
	}

	result := MergeAttributeMaps(base, override)

	assert.Equal(t, "sAMAccountName", result.Username)
	assert.Equal(t, "mail", result.Email)
	assert.Equal(t, "department", result.Department)
	// 未覆盖的保持原值
	assert.Equal(t, "givenName", result.FirstName)
	assert.Equal(t, "sn", result.LastName)
}

// ========== AD 时间戳解析测试 ==========

func TestParseADTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool // 是否应该返回非 nil
	}{
		{"zero value", "0", false},
		{"empty value", "", false},
		{"valid timestamp", "132871489500000000", true}, // 2021-03-15
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseADTimestamp(tt.value)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestParseADGeneralizedTime(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"empty value", "", false},
		{"valid time", "20210315120000.0Z", true},
		{"invalid format", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseADGeneralizedTime(tt.value)
			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestADFunctionalLevelName(t *testing.T) {
	tests := []struct {
		version  string
		expected string
	}{
		{"0", "Windows 2000"},
		{"2", "Windows Server 2003"},
		{"3", "Windows Server 2008"},
		{"4", "Windows Server 2008 R2"},
		{"5", "Windows Server 2012"},
		{"6", "Windows Server 2012 R2"},
		{"7", "Windows Server 2016"},
		{"8", "Windows Server 2019"},
		{"9", "Windows Server 2022"},
		{"99", "Unknown (99)"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := adFunctionalLevelName(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 用户和组结构测试 ==========

func TestUserStruct(t *testing.T) {
	now := time.Now()
	user := User{
		DN:          "cn=test,ou=users,dc=example,dc=com",
		Username:    "testuser",
		Email:       "test@example.com",
		FirstName:   "Test",
		LastName:    "User",
		FullName:    "Test User",
		DisplayName: "Test User",
		Department:  "Engineering",
		Title:       "Developer",
		EmployeeID:  "12345",
		Disabled:    false,
		Locked:      false,
		Groups:      []string{"developers", "admins"},
		LastLogin:   &now,
	}

	assert.Equal(t, "cn=test,ou=users,dc=example,dc=com", user.DN)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, []string{"developers", "admins"}, user.Groups)
	assert.False(t, user.Disabled)
	assert.False(t, user.Locked)
}

func TestGroupStruct(t *testing.T) {
	group := Group{
		DN:          "cn=developers,ou=groups,dc=example,dc=com",
		Name:        "developers",
		Description: "Development Team",
		GID:         "1001",
		Members:     []string{"cn=user1,ou=users,dc=example,dc=com", "cn=user2,ou=users,dc=example,dc=com"},
		Type:        "security",
		Scope:       "global",
	}

	assert.Equal(t, "cn=developers,ou=groups,dc=example,dc=com", group.DN)
	assert.Equal(t, "developers", group.Name)
	assert.Equal(t, "Development Team", group.Description)
	assert.Len(t, group.Members, 2)
}

func TestSyncResultStruct(t *testing.T) {
	now := time.Now()
	result := SyncResult{
		StartTime:        now,
		EndTime:          now.Add(5 * time.Second),
		Duration:         5 * time.Second,
		UsersCreated:     10,
		UsersUpdated:     5,
		UsersDeactivated: 2,
		GroupsCreated:    3,
		GroupsUpdated:    1,
		Success:          true,
		Message:          "同步完成",
	}

	assert.True(t, result.Success)
	assert.Equal(t, 10, result.UsersCreated)
	assert.Equal(t, 5, result.UsersUpdated)
	assert.Equal(t, 2, result.UsersDeactivated)
	assert.Equal(t, 3, result.GroupsCreated)
	assert.Equal(t, 5*time.Second, result.Duration)
}

// ========== AD 客户端测试 ==========

func TestNewADClient(t *testing.T) {
	config := Config{
		Name:       "test-ad",
		ServerType: ServerTypeAD,
		URL:        "ldaps://ad.example.com:636",
		BaseDN:     "DC=example,DC=com",
	}

	client, err := NewADClient(config)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, ServerTypeAD, client.config.ServerType)

	// 验证 AD 属性映射已设置
	assert.Equal(t, "sAMAccountName", client.config.Attributes.Username)
}

// ========== 错误定义测试 ==========

func TestErrorDefinitions(t *testing.T) {
	errors := []error{
		ErrConfigNotFound,
		ErrConnectionFailed,
		ErrBindFailed,
		ErrUserNotFound,
		ErrAuthFailed,
		ErrSearchFailed,
		ErrGroupNotFound,
		ErrSyncFailed,
		ErrInvalidConfig,
		ErrTLSCertInvalid,
		ErrTimeout,
		ErrServerUnavailable,
		ErrPermissionDenied,
		ErrAlreadyExists,
		ErrNotImplemented,
		ErrOperationFailed,
	}

	for _, err := range errors {
		assert.NotNil(t, err)
		assert.NotEmpty(t, err.Error())
	}
}

// ========== 连接状态测试 ==========

func TestConnectionStatus(t *testing.T) {
	status := ConnectionStatus{
		ConfigName:  "test",
		Connected:   true,
		LastChecked: time.Now(),
		Latency:     50 * time.Millisecond,
	}

	assert.Equal(t, "test", status.ConfigName)
	assert.True(t, status.Connected)
	assert.Equal(t, 50*time.Millisecond, status.Latency)
}

// ========== 服务器信息测试 ==========

func TestServerInfo(t *testing.T) {
	info := ServerInfo{
		Vendor:          "Microsoft Corporation",
		Version:         "10.0.19041",
		ProductName:     "Active Directory",
		FunctionalLevel: "Windows Server 2016",
		ForestName:      "example.com",
		DomainName:      "example.com",
		Realm:           "EXAMPLE.COM",
	}

	assert.Equal(t, "Microsoft Corporation", info.Vendor)
	assert.Equal(t, "Active Directory", info.ProductName)
	assert.Equal(t, "Windows Server 2016", info.FunctionalLevel)
}

// ========== 认证结果测试 ==========

func TestAuthResult(t *testing.T) {
	user := &User{
		Username: "testuser",
		Email:    "test@example.com",
	}

	result := AuthResult{
		Success:    true,
		User:       user,
		Token:      "test-token",
		Message:    "认证成功",
		AuthMethod: "simple",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "testuser", result.User.Username)
	assert.Equal(t, "simple", result.AuthMethod)
}

// ========== 辅助函数测试 ==========

func TestEscapeLDAP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"test*user", "test\\2auser"},
		{"test(user)", "test\\28user\\29"},
		{"test\\user", "test\\5cuser"},
		{"(test*)", "\\28test\\2a\\29"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeLDAP(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"-5", 0},  // 负数处理
		{"abc", 0}, // 非数字
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
