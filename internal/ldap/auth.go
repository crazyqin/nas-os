package ldap

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// Authenticator LDAP 认证器
type Authenticator struct {
	client *Client
	config Config
}

// NewAuthenticator 创建认证器
func NewAuthenticator(config Config) (*Authenticator, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	return &Authenticator{
		client: client,
		config: config,
	}, nil
}

// Authenticate 用户认证
func (a *Authenticator) Authenticate(username, password string) (*AuthResult, error) {
	result := &AuthResult{
		Success:    false,
		AuthMethod: "simple",
	}

	// 连接并绑定服务账号
	if err := a.client.Bind(); err != nil {
		result.Message = fmt.Sprintf("LDAP 连接失败: %v", err)
		return result, err
	}

	// 搜索用户
	user, err := a.findUser(username)
	if err != nil {
		result.Message = fmt.Sprintf("用户查找失败: %v", err)
		return result, err
	}

	if user == nil {
		result.Message = "用户不存在"
		return result, ErrUserNotFound
	}

	// 使用用户凭据绑定验证密码
	if err := a.client.BindWithCredential(user.DN, password); err != nil {
		result.Message = "认证失败"
		return result, ErrAuthFailed
	}

	// 获取用户组
	groups, err := a.getUserGroups(user.DN)
	if err == nil {
		user.Groups = groups
	}

	result.Success = true
	result.User = user
	result.Message = "认证成功"

	return result, nil
}

// AuthenticateWithDN 使用 DN 直接认证
func (a *Authenticator) AuthenticateWithDN(dn, password string) (*AuthResult, error) {
	result := &AuthResult{
		Success:    false,
		AuthMethod: "simple",
	}

	if err := a.client.BindWithCredential(dn, password); err != nil {
		result.Message = "认证失败"
		return result, ErrAuthFailed
	}

	result.Success = true
	result.Message = "认证成功"

	return result, nil
}

// findUser 查找用户
func (a *Authenticator) findUser(username string) (*User, error) {
	filter := a.buildUserFilter(username)
	
	searchDN := a.config.BaseDN
	if a.config.UserSearchDN != "" {
		searchDN = a.config.UserSearchDN
	}

	attributes := a.getUserAttributes()

	searchRequest := ldap.NewSearchRequest(
		searchDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, // 无限制
		0, // 无时间限制
		false,
		filter,
		attributes,
		nil,
	)

	result, err := a.client.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSearchFailed, err)
	}

	if len(result.Entries) == 0 {
		return nil, nil
	}

	// 返回第一个匹配的用户
	return a.entryToUser(result.Entries[0]), nil
}

// buildUserFilter 构建用户搜索过滤器
func (a *Authenticator) buildUserFilter(username string) string {
	escaped := ldap.EscapeFilter(username)

	// 如果配置了自定义过滤器
	if a.config.UserFilter != "" {
		if strings.Contains(a.config.UserFilter, "%s") {
			return fmt.Sprintf(a.config.UserFilter, escaped)
		}
		return fmt.Sprintf("(&%s(%s=%s))", a.config.UserFilter, a.config.Attributes.Username, escaped)
	}

	// 根据服务器类型构建默认过滤器
	switch a.config.ServerType {
	case ServerTypeAD:
		return fmt.Sprintf("(&(objectClass=%s)(%s=%s))",
			a.config.Attributes.UserObjectClass,
			a.config.Attributes.Username,
			escaped)
	default:
		return fmt.Sprintf("(&(objectClass=%s)(%s=%s))",
			a.config.Attributes.UserObjectClass,
			a.config.Attributes.Username,
			escaped)
	}
}

// getUserAttributes 获取需要查询的用户属性
func (a *Authenticator) getUserAttributes() []string {
	attrs := a.config.Attributes
	return []string{
		"dn",
		attrs.Username,
		attrs.Email,
		attrs.FirstName,
		attrs.LastName,
		attrs.FullName,
		attrs.DisplayName,
		attrs.Phone,
		attrs.Mobile,
		attrs.Department,
		attrs.Title,
		attrs.EmployeeID,
		attrs.MemberOfAttribute,
	}
}

// entryToUser 将 LDAP 条目转换为用户对象
func (a *Authenticator) entryToUser(entry *ldap.Entry) *User {
	attrs := a.config.Attributes
	user := &User{
		DN:           entry.DN,
		Username:     entry.GetAttributeValue(attrs.Username),
		Email:        entry.GetAttributeValue(attrs.Email),
		FirstName:    entry.GetAttributeValue(attrs.FirstName),
		LastName:     entry.GetAttributeValue(attrs.LastName),
		FullName:     entry.GetAttributeValue(attrs.FullName),
		DisplayName:  entry.GetAttributeValue(attrs.DisplayName),
		Phone:        entry.GetAttributeValue(attrs.Phone),
		Mobile:       entry.GetAttributeValue(attrs.Mobile),
		Department:   entry.GetAttributeValue(attrs.Department),
		Title:        entry.GetAttributeValue(attrs.Title),
		EmployeeID:   entry.GetAttributeValue(attrs.EmployeeID),
	}

	// 处理 memberOf 属性
	memberOf := entry.GetAttributeValues(attrs.MemberOfAttribute)
	user.Groups = make([]string, 0, len(memberOf))
	for _, dn := range memberOf {
		// 提取组名
		if cn := a.extractCN(dn); cn != "" {
			user.Groups = append(user.Groups, cn)
		}
	}

	// AD 特定处理
	if a.config.ServerType == ServerTypeAD {
		a.parseADAttributes(entry, user)
	}

	// 默认值
	if user.Username == "" {
		user.Username = user.DN
	}

	return user
}

// parseADAttributes 解析 AD 特定属性
func (a *Authenticator) parseADAttributes(entry *ldap.Entry, user *User) {
	// 检查账户状态
	userAccountControl := entry.GetAttributeValue("userAccountControl")
	if userAccountControl != "" {
		// 解析 userAccountControl 标志
		// 0x0002 = ACCOUNTDISABLE
		// 0x0010 = LOCKOUT
		// 0x00800000 = PASSWORD_EXPIRED
		uac := parseUAC(userAccountControl)
		user.Disabled = uac&0x0002 != 0
		user.Locked = uac&0x0010 != 0
		user.PasswordExpired = uac&0x00800000 != 0
	}

	// 解析时间
	if lastLogon := entry.GetAttributeValue("lastLogon"); lastLogon != "" {
		if t := parseADTimestamp(lastLogon); t != nil {
			user.LastLogin = t
		}
	}

	if pwdLastSet := entry.GetAttributeValue("pwdLastSet"); pwdLastSet != "" {
		if t := parseADTimestamp(pwdLastSet); t != nil {
			user.PasswordSet = t
		}
	}

	if whenCreated := entry.GetAttributeValue("whenCreated"); whenCreated != "" {
		if t := parseADGeneralizedTime(whenCreated); t != nil {
			user.AccountCreated = t
		}
	}
}

// getUserGroups 获取用户组
func (a *Authenticator) getUserGroups(userDN string) ([]string, error) {
	filter := a.buildGroupFilter(userDN)

	searchDN := a.config.BaseDN
	if a.config.GroupSearchDN != "" {
		searchDN = a.config.GroupSearchDN
	}

	searchRequest := ldap.NewSearchRequest(
		searchDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		[]string{"cn", "dn"},
		nil,
	)

	result, err := a.client.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	groups := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		groups = append(groups, entry.GetAttributeValue("cn"))
	}

	return groups, nil
}

// buildGroupFilter 构建组搜索过滤器
func (a *Authenticator) buildGroupFilter(userDN string) string {
	escaped := ldap.EscapeFilter(userDN)

	// 如果配置了自定义过滤器
	if a.config.GroupFilter != "" {
		if strings.Contains(a.config.GroupFilter, "%s") {
			return fmt.Sprintf(a.config.GroupFilter, escaped)
		}
	}

	// 根据服务器类型构建默认过滤器
	switch a.config.ServerType {
	case ServerTypeAD:
		return fmt.Sprintf("(&(objectClass=%s)(%s=%s))",
			a.config.Attributes.GroupObjectClass,
			a.config.Attributes.MemberAttribute,
			escaped)
	default:
		// OpenLDAP 通常使用 memberUid
		return fmt.Sprintf("(&(objectClass=%s)(%s=%s))",
			a.config.Attributes.GroupObjectClass,
			a.config.Attributes.MemberAttribute,
			escaped)
	}
}

// extractCN 从 DN 中提取 CN
func (a *Authenticator) extractCN(dn string) string {
	parsed, err := ldap.ParseDN(dn)
	if err != nil {
		return ""
	}

	for _, rdn := range parsed.RDNs {
		for _, attr := range rdn.Attributes {
			if strings.EqualFold(attr.Type, "cn") {
				return attr.Value
			}
		}
	}

	return ""
}

// Close 关闭认证器
func (a *Authenticator) Close() error {
	return a.client.Close()
}

// VerifyPassword 验证密码（不获取用户信息）
func (a *Authenticator) VerifyPassword(username, password string) (bool, error) {
	// 连接并绑定服务账号
	if err := a.client.Bind(); err != nil {
		return false, err
	}

	// 搜索用户 DN
	user, err := a.findUser(username)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, ErrUserNotFound
	}

	// 使用用户凭据绑定验证密码
	if err := a.client.BindWithCredential(user.DN, password); err != nil {
		return false, nil // 密码错误，但不返回错误
	}

	return true, nil
}

// ChangePassword 修改用户密码
func (a *Authenticator) ChangePassword(username, oldPassword, newPassword string) error {
	// 先验证旧密码
	valid, err := a.VerifyPassword(username, oldPassword)
	if err != nil {
		return err
	}
	if !valid {
		return ErrAuthFailed
	}

	// 获取用户信息
	user, err := a.findUser(username)
	if err != nil {
		return err
	}

	// 修改密码
	return a.client.PasswordModify(user.DN, oldPassword, newPassword)
}

// parseUAC 解析 userAccountControl
func parseUAC(value string) uint32 {
	var uac uint32
	for _, c := range value {
		if c >= '0' && c <= '9' {
			uac = uac*10 + uint32(c-'0')
		}
	}
	return uac
}