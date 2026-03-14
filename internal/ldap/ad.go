package ldap

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// ADClient Active Directory 客户端
type ADClient struct {
	client *Client
	config Config
}

// NewADClient 创建 AD 客户端
func NewADClient(config Config) (*ADClient, error) {
	if config.ServerType != ServerTypeAD {
		config.ServerType = ServerTypeAD
	}
	
	// 设置 AD 默认属性映射
	if config.Attributes.Username == "" || config.Attributes.Username == "uid" {
		config.Attributes = ADAttributeMapping()
	}

	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	return &ADClient{
		client: client,
		config: config,
	}, nil
}

// Authenticate AD 用户认证
func (a *ADClient) Authenticate(username, password string) (*AuthResult, error) {
	// AD 可以使用 userPrincipalName (user@domain) 或 sAMAccountName
	auth := &Authenticator{
		client: a.client,
		config: a.config,
	}
	return auth.Authenticate(username, password)
}

// AuthenticateWithUPN 使用 UPN 认证
func (a *ADClient) AuthenticateWithUPN(upn, password string) (*AuthResult, error) {
	return a.Authenticate(upn, password)
}

// AuthenticateWithSAM 使用 sAMAccountName 认证
func (a *ADClient) AuthenticateWithSAM(samAccountName, password string) (*AuthResult, error) {
	return a.Authenticate(samAccountName, password)
}

// GetDomainInfo 获取域信息
func (a *ADClient) GetDomainInfo() (*DomainInfo, error) {
	if err := a.client.Bind(); err != nil {
		return nil, err
	}

	// 搜索域信息
	filter := "(objectClass=domainDNS)"
	searchRequest := ldap.NewSearchRequest(
		a.config.BaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"*", "msDS-Behavior-Version", "creationTime", "modificationTime"},
		nil,
	)

	result, err := a.client.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("获取域信息失败: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("未找到域信息")
	}

	entry := result.Entries[0]
	info := &DomainInfo{
		DN:             entry.DN,
		Name:           entry.GetAttributeValue("name"),
		DistinguishedName: entry.DN,
	}

	// 功能级别
	if version := entry.GetAttributeValue("msDS-Behavior-Version"); version != "" {
		info.FunctionalLevel = adFunctionalLevelName(version)
	}

	return info, nil
}

// GetForestInfo 获取林信息
func (a *ADClient) GetForestInfo() (*ForestInfo, error) {
	if err := a.client.Bind(); err != nil {
		return nil, err
	}

	// 搜索 Partitions 容器
	partitionsDN := fmt.Sprintf("CN=Partitions,CN=Configuration,%s", a.config.BaseDN)
	filter := "(objectClass=crossRef)"
	
	searchRequest := ldap.NewSearchRequest(
		partitionsDN,
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"cn", "nCName", "dnsRoot"},
		nil,
	)

	result, err := a.client.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("获取林信息失败: %w", err)
	}

	info := &ForestInfo{
		Domains: make([]DomainRef, 0),
	}

	for _, entry := range result.Entries {
		ref := DomainRef{
			Name:  entry.GetAttributeValue("cn"),
			DN:    entry.GetAttributeValue("nCName"),
			DNS:   entry.GetAttributeValue("dnsRoot"),
		}
		info.Domains = append(info.Domains, ref)
	}

	return info, nil
}

// GetUserAccountInfo 获取用户账户详细信息
func (a *ADClient) GetUserAccountInfo(username string) (*ADUserAccountInfo, error) {
	if err := a.client.Bind(); err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		a.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{
			"dn", "sAMAccountName", "userPrincipalName", "displayName",
			"userAccountControl", "accountExpires", "pwdLastSet", "lastLogon",
			"lockoutTime", "badPwdCount", "badPasswordTime",
			"memberOf", "primaryGroupID",
		},
		nil,
	)

	result, err := a.client.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("搜索用户失败: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, ErrUserNotFound
	}

	entry := result.Entries[0]
	return a.parseADUserAccountInfo(entry), nil
}

// parseADUserAccountInfo 解析 AD 用户账户信息
func (a *ADClient) parseADUserAccountInfo(entry *ldap.Entry) *ADUserAccountInfo {
	info := &ADUserAccountInfo{
		DN:                entry.DN,
		SAMAccountName:    entry.GetAttributeValue("sAMAccountName"),
		UserPrincipalName: entry.GetAttributeValue("userPrincipalName"),
		DisplayName:       entry.GetAttributeValue("displayName"),
	}

	// 解析 userAccountControl
	uacStr := entry.GetAttributeValue("userAccountControl")
	if uacStr != "" {
		uac := parseUAC(uacStr)
		info.Disabled = uac&0x0002 != 0
		info.Locked = uac&0x0010 != 0
		info.PasswordNotRequired = uac&0x0020 != 0
		info.PasswordCantChange = uac&0x0040 != 0
		info.PasswordNeverExpires = uac&0x10000 != 0
		info.SmartcardRequired = uac&0x40000 != 0
	}

	// 解析时间
	if accountExpires := entry.GetAttributeValue("accountExpires"); accountExpires != "" {
		info.AccountExpires = parseADTimestamp(accountExpires)
	}

	if pwdLastSet := entry.GetAttributeValue("pwdLastSet"); pwdLastSet != "" {
		info.PasswordLastSet = parseADTimestamp(pwdLastSet)
	}

	if lastLogon := entry.GetAttributeValue("lastLogon"); lastLogon != "" {
		info.LastLogon = parseADTimestamp(lastLogon)
	}

	if lockoutTime := entry.GetAttributeValue("lockoutTime"); lockoutTime != "" {
		info.LockoutTime = parseADTimestamp(lockoutTime)
	}

	if badPasswordTime := entry.GetAttributeValue("badPasswordTime"); badPasswordTime != "" {
		info.BadPasswordTime = parseADTimestamp(badPasswordTime)
	}

	// 解析计数
	if badPwdCount := entry.GetAttributeValue("badPwdCount"); badPwdCount != "" {
		info.BadPasswordCount = parseInt(badPwdCount)
	}

	// 解析组成员关系
	info.MemberOf = entry.GetAttributeValues("memberOf")
	info.PrimaryGroupID = entry.GetAttributeValue("primaryGroupID")

	return info
}

// EnableUser 启用用户
func (a *ADClient) EnableUser(userDN string) error {
	return a.setUserAccountControlBit(userDN, 0x0002, false) // 清除 ACCOUNTDISABLE 位
}

// DisableUser 禁用用户
func (a *ADClient) DisableUser(userDN string) error {
	return a.setUserAccountControlBit(userDN, 0x0002, true) // 设置 ACCOUNTDISABLE 位
}

// UnlockUser 解锁用户
func (a *ADClient) UnlockUser(userDN string) error {
	// 清除 lockoutTime
	request := ldap.NewModifyRequest(userDN, nil)
	request.Replace("lockoutTime", []string{"0"})
	return a.client.Modify(request)
}

// ForcePasswordChange 强制用户下次登录时修改密码
func (a *ADClient) ForcePasswordChange(userDN string) error {
	// 设置 pwdLastSet 为 0
	request := ldap.NewModifyRequest(userDN, nil)
	request.Replace("pwdLastSet", []string{"0"})
	return a.client.Modify(request)
}

// setUserAccountControlBit 设置 userAccountControl 位
func (a *ADClient) setUserAccountControlBit(userDN string, bit uint32, set bool) error {
	// 先获取当前 userAccountControl
	filter := fmt.Sprintf("(distinguishedName=%s)", ldap.EscapeFilter(userDN))
	searchRequest := ldap.NewSearchRequest(
		a.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		filter,
		[]string{"userAccountControl"},
		nil,
	)

	result, err := a.client.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("获取用户信息失败: %w", err)
	}

	if len(result.Entries) == 0 {
		return ErrUserNotFound
	}

	uacStr := result.Entries[0].GetAttributeValue("userAccountControl")
	uac := parseUAC(uacStr)

	if set {
		uac |= bit
	} else {
		uac &^= bit
	}

	// 更新 userAccountControl
	request := ldap.NewModifyRequest(userDN, nil)
	request.Replace("userAccountControl", []string{fmt.Sprintf("%d", uac)})
	return a.client.Modify(request)
}

// GetGroupMembership 获取用户组成员关系
func (a *ADClient) GetGroupMembership(userDN string, recursive bool) ([]*Group, error) {
	if err := a.client.Bind(); err != nil {
		return nil, err
	}

	groups := make([]*Group, 0)
	visited := make(map[string]bool)

	var getGroups func(dn string) error
	getGroups = func(dn string) error {
		if visited[dn] {
			return nil
		}
		visited[dn] = true

		filter := fmt.Sprintf("(&(objectClass=group)(member=%s))", ldap.EscapeFilter(dn))
		searchRequest := ldap.NewSearchRequest(
			a.config.BaseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0, 0, false,
			filter,
			[]string{"dn", "cn", "description", "groupType"},
			nil,
		)

		result, err := a.client.Search(searchRequest)
		if err != nil {
			return err
		}

		for _, entry := range result.Entries {
			group := &Group{
				DN:          entry.DN,
				Name:        entry.GetAttributeValue("cn"),
				Description: entry.GetAttributeValue("description"),
			}

			// 解析组类型
			if groupType := entry.GetAttributeValue("groupType"); groupType != "" {
				gt := parseInt(groupType)
				group.Type = "security"
				if gt&0x80000000 == 0 {
					group.Type = "distribution"
				}
				switch gt & 0xF {
				case 2:
					group.Scope = "global"
				case 4:
					group.Scope = "domain_local"
				case 8:
					group.Scope = "universal"
				}
			}

			groups = append(groups, group)

			// 递归获取嵌套组
			if recursive {
				getGroups(entry.DN)
			}
		}

		return nil
	}

	if err := getGroups(userDN); err != nil {
		return nil, err
	}

	return groups, nil
}

// Close 关闭客户端
func (a *ADClient) Close() error {
	return a.client.Close()
}

// DomainInfo 域信息
type DomainInfo struct {
	DN               string    `json:"dn"`
	Name             string    `json:"name"`
	DistinguishedName string   `json:"distinguished_name"`
	FunctionalLevel  string    `json:"functional_level,omitempty"`
	Created          *time.Time `json:"created,omitempty"`
	Modified         *time.Time `json:"modified,omitempty"`
}

// ForestInfo 林信息
type ForestInfo struct {
	Domains []DomainRef `json:"domains"`
}

// DomainRef 域引用
type DomainRef struct {
	Name string `json:"name"`
	DN   string `json:"dn"`
	DNS  string `json:"dns"`
}

// ADUserAccountInfo AD 用户账户信息
type ADUserAccountInfo struct {
	DN                string     `json:"dn"`
	SAMAccountName    string     `json:"sam_account_name"`
	UserPrincipalName string     `json:"user_principal_name,omitempty"`
	DisplayName       string     `json:"display_name,omitempty"`
	
	// 账户状态
	Disabled              bool      `json:"disabled"`
	Locked                bool      `json:"locked"`
	PasswordNotRequired   bool      `json:"password_not_required"`
	PasswordCantChange    bool      `json:"password_cant_change"`
	PasswordNeverExpires  bool      `json:"password_never_expires"`
	SmartcardRequired     bool      `json:"smartcard_required"`
	
	// 时间信息
	AccountExpires     *time.Time `json:"account_expires,omitempty"`
	PasswordLastSet    *time.Time `json:"password_last_set,omitempty"`
	LastLogon          *time.Time `json:"last_logon,omitempty"`
	LockoutTime        *time.Time `json:"lockout_time,omitempty"`
	BadPasswordTime    *time.Time `json:"bad_password_time,omitempty"`
	BadPasswordCount   int        `json:"bad_password_count"`
	
	// 组信息
	MemberOf         []string `json:"member_of,omitempty"`
	PrimaryGroupID   string   `json:"primary_group_id,omitempty"`
}

// parseADTimestamp 解析 AD 时间戳
func parseADTimestamp(value string) *time.Time {
	if value == "0" || value == "" {
		return nil
	}

	// AD 时间戳是从 1601-01-01 开始的 100 纳秒间隔
	var ticks int64
	for _, c := range value {
		if c >= '0' && c <= '9' {
			ticks = ticks*10 + int64(c-'0')
		}
	}

	if ticks == 0 {
		return nil
	}

	// 转换为 Unix 时间
	// 116444736000000000 = 从 1601-01-01 到 1970-01-01 的 100 纳秒间隔
	seconds := ticks/10000000 - 11644473600
	if seconds < 0 {
		return nil
	}

	t := time.Unix(seconds, 0)
	return &t
}

// parseADGeneralizedTime 解析 AD Generalized Time
func parseADGeneralizedTime(value string) *time.Time {
	if value == "" {
		return nil
	}

	// 格式: YYYYMMDDHHMMSS.0Z
	t, err := time.Parse("20060102150405.0Z", value)
	if err != nil {
		return nil
	}

	return &t
}

// adFunctionalLevelName 获取功能级别名称
func adFunctionalLevelName(version string) string {
	switch version {
	case "0":
		return "Windows 2000"
	case "1":
		return "Windows Server 2003 Interim"
	case "2":
		return "Windows Server 2003"
	case "3":
		return "Windows Server 2008"
	case "4":
		return "Windows Server 2008 R2"
	case "5":
		return "Windows Server 2012"
	case "6":
		return "Windows Server 2012 R2"
	case "7":
		return "Windows Server 2016"
	case "8":
		return "Windows Server 2019"
	case "9":
		return "Windows Server 2022"
	default:
		return fmt.Sprintf("Unknown (%s)", version)
	}
}

// parseInt 解析整数，负数返回 0
func parseInt(value string) int {
	var result int
	for _, c := range value {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		} else if c == '-' {
			// 负数不支持，返回 0
			return 0
		}
	}
	return result
}

// BuildADConfig 构建 AD 配置
func BuildADConfig(name, url, bindDN, bindPassword, baseDN, domainName string) Config {
	return Config{
		Name:         name,
		ServerType:   ServerTypeAD,
		URL:          url,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		BaseDN:       baseDN,
		UserFilter:   "(sAMAccountName=%s)",
		GroupFilter:  "(member=%s)",
		Attributes:   ADAttributeMapping(),
		DomainName:   domainName,
		UseTLS:       true,
		Enabled:      true,
		PoolSize:     10,
		IdleTimeout:  30 * time.Minute,
		ConnectTimeout: 10 * time.Second,
		OperationTimeout: 30 * time.Second,
	}
}

// BuildOpenLDAPConfig 构建 OpenLDAP 配置
func BuildOpenLDAPConfig(name, url, bindDN, bindPassword, baseDN string) Config {
	return Config{
		Name:         name,
		ServerType:   ServerTypeOpenLDAP,
		URL:          url,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		BaseDN:       baseDN,
		UserFilter:   "(uid=%s)",
		GroupFilter:  "(memberUid=%s)",
		Attributes:   DefaultAttributeMapping(),
		UseTLS:       true,
		Enabled:      true,
		PoolSize:     10,
		IdleTimeout:  30 * time.Minute,
		ConnectTimeout: 10 * time.Second,
		OperationTimeout: 30 * time.Second,
	}
}

// BuildFreeIPAConfig 构建 FreeIPA 配置
func BuildFreeIPAConfig(name, url, bindDN, bindPassword, baseDN string) Config {
	attrs := DefaultAttributeMapping()
	attrs.UserObjectClass = "posixAccount"
	attrs.GroupObjectClass = "posixGroup"
	attrs.MemberAttribute = "memberUid"

	return Config{
		Name:         name,
		ServerType:   ServerTypeFreeIPA,
		URL:          url,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		BaseDN:       baseDN,
		UserFilter:   "(uid=%s)",
		GroupFilter:  "(memberUid=%s)",
		Attributes:   attrs,
		UseTLS:       true,
		Enabled:      true,
		PoolSize:     10,
		IdleTimeout:  30 * time.Minute,
		ConnectTimeout: 10 * time.Second,
		OperationTimeout: 30 * time.Second,
	}
}

// DetectServerType 从服务器信息检测类型
func DetectServerType(client *Client) (ServerType, error) {
	if err := client.Bind(); err != nil {
		return ServerTypeGeneric, err
	}

	// 搜索 Root DSE
	searchRequest := ldap.NewSearchRequest(
		"",
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"vendorName", "vendorVersion", "objectClass", "supportedLDAPVersion"},
		nil,
	)

	result, err := client.Search(searchRequest)
	if err != nil {
		return ServerTypeGeneric, err
	}

	if len(result.Entries) == 0 {
		return ServerTypeGeneric, nil
	}

	entry := result.Entries[0]
	vendor := strings.ToLower(entry.GetAttributeValue("vendorName"))
	version := strings.ToLower(entry.GetAttributeValue("vendorVersion"))

	// 检测 Active Directory
	objectClasses := entry.GetAttributeValues("objectClass")
	for _, oc := range objectClasses {
		if strings.EqualFold(oc, "domainDNS") {
			return ServerTypeAD, nil
		}
	}

	// 检测 FreeIPA
	if strings.Contains(vendor, "red hat") || strings.Contains(vendor, "freeipa") {
		return ServerTypeFreeIPA, nil
	}

	// 检测 OpenLDAP
	if strings.Contains(vendor, "openldap") || strings.Contains(version, "openldap") {
		return ServerTypeOpenLDAP, nil
	}

	return ServerTypeGeneric, nil
}

// IsUserInGroup 检查用户是否在指定组中
func (a *ADClient) IsUserInGroup(userDN, groupCN string) (bool, error) {
	groups, err := a.GetGroupMembership(userDN, true)
	if err != nil {
		return false, err
	}

	for _, g := range groups {
		if strings.EqualFold(g.Name, groupCN) {
			return true, nil
		}
	}

	return false, nil
}

// GetNestedGroupMembers 获取组的所有成员（包括嵌套组成员）
func (a *ADClient) GetNestedGroupMembers(groupDN string) ([]*User, error) {
	if err := a.client.Bind(); err != nil {
		return nil, err
	}

	users := make([]*User, 0)
	visited := make(map[string]bool)

	var getMembers func(dn string) error
	getMembers = func(dn string) error {
		if visited[dn] {
			return nil
		}
		visited[dn] = true

		// 获取组成员
		filter := fmt.Sprintf("(memberOf=%s)", ldap.EscapeFilter(dn))
		searchRequest := ldap.NewSearchRequest(
			a.config.BaseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0, 0, false,
			filter,
			[]string{"dn", "sAMAccountName", "cn", "mail", "objectClass"},
			nil,
		)

		result, err := a.client.Search(searchRequest)
		if err != nil {
			return err
		}

		for _, entry := range result.Entries {
			objectClasses := entry.GetAttributeValues("objectClass")
			isUser := false
			for _, oc := range objectClasses {
				if strings.EqualFold(oc, "user") {
					isUser = true
					break
				}
			}

			if isUser {
				user := &User{
					DN:       entry.DN,
					Username: entry.GetAttributeValue("sAMAccountName"),
					FullName: entry.GetAttributeValue("cn"),
					Email:    entry.GetAttributeValue("mail"),
				}
				users = append(users, user)
			} else {
				// 可能是嵌套组，递归处理
				getMembers(entry.DN)
			}
		}

		return nil
	}

	if err := getMembers(groupDN); err != nil {
		return nil, err
	}

	return users, nil
}