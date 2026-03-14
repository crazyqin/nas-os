package auth

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig LDAP/AD 配置
type LDAPConfig struct {
	Name           string   `json:"name"`
	URL            string   `json:"url"`             // ldap://host:port 或 ldaps://host:port
	BindDN         string   `json:"bind_dn"`         // 绑定 DN（用于搜索）
	BindPassword   string   `json:"bind_password"`   // 绑定密码
	BaseDN         string   `json:"base_dn"`         // 搜索基础 DN
	UserFilter     string   `json:"user_filter"`     // 用户搜索过滤器，如 (uid=%s)
	GroupFilter    string   `json:"group_filter"`    // 组搜索过滤器
	AttributeMap   AttributeMap `json:"attribute_map"` // 属性映射
	UseTLS         bool     `json:"use_tls"`         // 是否使用 TLS
	SkipTLSVerify  bool     `json:"skip_tls_verify"` // 跳过 TLS 验证（仅测试用）
	CACertPath     string   `json:"ca_cert_path"`    // CA 证书路径
	Enabled        bool     `json:"enabled"`
	IsAD           bool     `json:"is_ad"`           // 是否为 Active Directory
}

// AttributeMap 属性映射
type AttributeMap struct {
	Username  string `json:"username"`   // 用户名属性
	Email     string `json:"email"`      // 邮箱属性
	FirstName string `json:"first_name"` // 名
	LastName  string `json:"last_name"`  // 姓
	FullName  string `json:"full_name"`  // 全名
	Groups    string `json:"groups"`     // 组属性
}

// LDAPUser LDAP 用户信息
type LDAPUser struct {
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	FullName  string   `json:"full_name"`
	Groups    []string `json:"groups"`
	DN        string   `json:"dn"`
}

// LDAPManager LDAP/AD 管理器
type LDAPManager struct {
	mu     sync.RWMutex
	configs map[string]*LDAPConfig
}

var (
	ErrLDAPConfigNotFound  = errors.New("LDAP 配置未找到")
	ErrLDAPConnectionFailed = errors.New("LDAP 连接失败")
	ErrLDAPBindFailed      = errors.New("LDAP 绑定失败")
	ErrLDAPUserNotFound    = errors.New("LDAP 用户未找到")
	ErrLDAPAuthFailed      = errors.New("LDAP 认证失败")
	ErrLDAPSearchFailed    = errors.New("LDAP 搜索失败")
)

// NewLDAPManager 创建 LDAP 管理器
func NewLDAPManager() *LDAPManager {
	return &LDAPManager{
		configs: make(map[string]*LDAPConfig),
	}
}

// RegisterConfig 注册 LDAP 配置
func (m *LDAPManager) RegisterConfig(config LDAPConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configs[config.Name] = &config
	return nil
}

// GetConfig 获取 LDAP 配置
func (m *LDAPManager) GetConfig(name string) (*LDAPConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, ok := m.configs[name]
	if !ok {
		return nil, ErrLDAPConfigNotFound
	}
	return config, nil
}

// ListConfigs 列出所有已启用的配置
func (m *LDAPManager) ListConfigs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var configs []string
	for name, c := range m.configs {
		if c.Enabled {
			configs = append(configs, name)
		}
	}
	return configs
}

// Authenticate LDAP 认证
func (m *LDAPManager) Authenticate(configName, username, password string) (*LDAPUser, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	// 连接 LDAP 服务器
	conn, err := m.connect(config)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// 搜索用户
	user, err := m.searchUser(conn, config, username)
	if err != nil {
		return nil, err
	}

	// 使用用户 DN 和密码进行绑定验证
	if err := conn.Bind(user.DN, password); err != nil {
		return nil, ErrLDAPAuthFailed
	}

	// 获取用户组
	groups, err := m.getUserGroups(conn, config, user.DN)
	if err == nil {
		user.Groups = groups
	}

	return user, nil
}

// connect 连接 LDAP 服务器
func (m *LDAPManager) connect(config *LDAPConfig) (*ldap.Conn, error) {
	var conn *ldap.Conn
	var err error

	if config.UseTLS {
		// LDAPS 连接
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify,
		}

		// 加载自定义 CA 证书
		if config.CACertPath != "" {
			caCert, err := ioutil.ReadFile(config.CACertPath)
			if err != nil {
				return nil, fmt.Errorf("读取 CA 证书失败: %w", err)
			}

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}

		conn, err = ldap.DialTLS("tcp", config.URL[7:], tlsConfig) // 移除 ldaps:// 前缀
	} else {
		// 普通 LDAP 连接
		conn, err = ldap.DialURL(config.URL)
	}

	if err != nil {
		return nil, ErrLDAPConnectionFailed
	}

	// 如果需要 StartTLS
	if !config.UseTLS && config.URL[:5] == "ldap:" {
		if config.SkipTLSVerify {
			err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
		} else {
			err = conn.StartTLS(&tls.Config{ServerName: config.URL[7:]})
		}
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("StartTLS 失败: %w", err)
		}
	}

	// 绑定管理员账号（用于搜索）
	if config.BindDN != "" {
		if err := conn.Bind(config.BindDN, config.BindPassword); err != nil {
			conn.Close()
			return nil, ErrLDAPBindFailed
		}
	}

	return conn, nil
}

// searchUser 搜索用户
func (m *LDAPManager) searchUser(conn *ldap.Conn, config *LDAPConfig, username string) (*LDAPUser, error) {
	// 构建搜索过滤器
	filter := fmt.Sprintf(config.UserFilter, ldap.EscapeFilter(username))
	if config.IsAD {
		// Active Directory 特殊处理
		filter = fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", ldap.EscapeFilter(username))
	}

	// 搜索请求
	searchRequest := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{
			config.AttributeMap.Username,
			config.AttributeMap.Email,
			config.AttributeMap.FirstName,
			config.AttributeMap.LastName,
			config.AttributeMap.FullName,
			"dn",
		},
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, ErrLDAPSearchFailed
	}

	if len(sr.Entries) == 0 {
		return nil, ErrLDAPUserNotFound
	}

	entry := sr.Entries[0]

	user := &LDAPUser{
		DN:        entry.DN,
		Username:  entry.GetAttributeValue(config.AttributeMap.Username),
		Email:     entry.GetAttributeValue(config.AttributeMap.Email),
		FirstName: entry.GetAttributeValue(config.AttributeMap.FirstName),
		LastName:  entry.GetAttributeValue(config.AttributeMap.LastName),
		FullName:  entry.GetAttributeValue(config.AttributeMap.FullName),
	}

	// 默认值处理
	if user.Username == "" {
		user.Username = username
	}

	return user, nil
}

// getUserGroups 获取用户组
func (m *LDAPManager) getUserGroups(conn *ldap.Conn, config *LDAPConfig, userDN string) ([]string, error) {
	if config.GroupFilter == "" {
		return nil, nil
	}

	filter := fmt.Sprintf(config.GroupFilter, ldap.EscapeFilter(userDN))
	if config.IsAD {
		filter = fmt.Sprintf("(&(objectClass=group)(member=%s))", ldap.EscapeFilter(userDN))
	}

	searchRequest := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"cn"},
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	groups := make([]string, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		groups = append(groups, entry.GetAttributeValue("cn"))
	}

	return groups, nil
}

// SearchUsers 搜索用户（管理功能）
func (m *LDAPManager) SearchUsers(configName, query string) ([]*LDAPUser, error) {
	config, err := m.GetConfig(configName)
	if err != nil {
		return nil, err
	}

	conn, err := m.connect(config)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	filter := fmt.Sprintf("(|(%s=*%s*)(%s=*%s*))",
		config.AttributeMap.Username, ldap.EscapeFilter(query),
		config.AttributeMap.Email, ldap.EscapeFilter(query))

	searchRequest := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 100, 0, false,
		filter,
		[]string{
			config.AttributeMap.Username,
			config.AttributeMap.Email,
			config.AttributeMap.FullName,
			"dn",
		},
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, ErrLDAPSearchFailed
	}

	users := make([]*LDAPUser, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		users = append(users, &LDAPUser{
			DN:        entry.DN,
			Username:  entry.GetAttributeValue(config.AttributeMap.Username),
			Email:     entry.GetAttributeValue(config.AttributeMap.Email),
			FullName:  entry.GetAttributeValue(config.AttributeMap.FullName),
		})
	}

	return users, nil
}

// TestConnection 测试 LDAP 连接
func (m *LDAPManager) TestConnection(configName string) error {
	config, err := m.GetConfig(configName)
	if err != nil {
		return err
	}

	conn, err := m.connect(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

// ========== 预定义配置模板 ==========

// GetOpenLDAPConfig 获取 OpenLDAP 配置模板
func GetOpenLDAPConfig(name, url, bindDN, bindPassword, baseDN string) LDAPConfig {
	return LDAPConfig{
		Name:         name,
		URL:          url,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		BaseDN:       baseDN,
		UserFilter:   "(uid=%s)",
		GroupFilter:  "(memberUid=%s)",
		AttributeMap: AttributeMap{
			Username:  "uid",
			Email:     "mail",
			FirstName: "givenName",
			LastName:  "sn",
			FullName:  "cn",
			Groups:    "memberOf",
		},
		UseTLS:  true,
		Enabled: true,
		IsAD:    false,
	}
}

// GetADConfig 获取 Active Directory 配置模板
func GetADConfig(name, url, bindDN, bindPassword, baseDN string) LDAPConfig {
	return LDAPConfig{
		Name:         name,
		URL:          url,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		BaseDN:       baseDN,
		UserFilter:   "(sAMAccountName=%s)",
		GroupFilter:  "(member=%s)",
		AttributeMap: AttributeMap{
			Username:  "sAMAccountName",
			Email:     "mail",
			FirstName: "givenName",
			LastName:  "sn",
			FullName:  "displayName",
			Groups:    "memberOf",
		},
		UseTLS:  true,
		Enabled: true,
		IsAD:    true,
	}
}

// GetFreeIPAConfig 获取 FreeIPA 配置模板
func GetFreeIPAConfig(name, url, bindDN, bindPassword, baseDN string) LDAPConfig {
	return LDAPConfig{
		Name:         name,
		URL:          url,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		BaseDN:       baseDN,
		UserFilter:   "(uid=%s)",
		GroupFilter:  "(member=%s)",
		AttributeMap: AttributeMap{
			Username:  "uid",
			Email:     "mail",
			FirstName: "givenName",
			LastName:  "sn",
			FullName:  "cn",
			Groups:    "memberOf",
		},
		UseTLS:  true,
		Enabled: true,
		IsAD:    false,
	}
}