package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 错误定义
var (
	ErrPermissionDenied   = errors.New("权限不足")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrRoleNotFound       = errors.New("角色不存在")
	ErrGroupNotFound      = errors.New("用户组不存在")
	ErrPolicyNotFound     = errors.New("策略不存在")
	ErrInvalidPermission  = errors.New("无效的权限格式")
	ErrCircularDependency = errors.New("检测到循环依赖")
	ErrCacheExpired       = errors.New("权限缓存已过期")
)

// Config RBAC 配置
type Config struct {
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`
	ConfigPath   string        `json:"config_path"`
	StrictMode   bool          `json:"strict_mode"`   // 严格模式：默认拒绝
	AuditEnabled bool          `json:"audit_enabled"` // 是否记录权限检查日志
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		CacheEnabled: true,
		CacheTTL:     time.Minute * 5,
		StrictMode:   true,
		AuditEnabled: true,
	}
}

// Manager RBAC 管理器
type Manager struct {
	config Config
	mu     sync.RWMutex

	// 权限存储
	userPermissions map[string]*UserPermission  // userID -> UserPermission
	groupPerms      map[string]*GroupPermission // groupID -> GroupPermission
	policies        map[string]*Policy          // policyID -> Policy

	// 权限缓存
	cache   map[string]*PermissionCache // userID -> cache
	cacheMu sync.RWMutex

	// 自定义角色
	customRoles map[Role]RoleInfo

	// 审计回调
	auditCallback func(userID, resource, action string, result *CheckResult)
}

// NewManager 创建 RBAC 管理器
func NewManager(config Config) (*Manager, error) {
	m := &Manager{
		config:          config,
		userPermissions: make(map[string]*UserPermission),
		groupPerms:      make(map[string]*GroupPermission),
		policies:        make(map[string]*Policy),
		cache:           make(map[string]*PermissionCache),
		customRoles:     make(map[Role]RoleInfo),
	}

	// 加载配置
	if config.ConfigPath != "" {
		if err := m.load(); err != nil {
			// 配置文件不存在不报错
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("加载 RBAC 配置失败: %w", err)
			}
		}
	}

	return m, nil
}

// SetAuditCallback 设置审计回调
func (m *Manager) SetAuditCallback(callback func(userID, resource, action string, result *CheckResult)) {
	m.auditCallback = callback
}

// ========== 用户权限管理 ==========

// SetUserRole 设置用户角色
func (m *Manager) SetUserRole(userID, username string, role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证角色
	if !m.isValidRole(role) {
		return ErrRoleNotFound
	}

	// 获取或创建用户权限
	up, exists := m.userPermissions[userID]
	if !exists {
		up = &UserPermission{
			UserID:           userID,
			Username:         username,
			DirectPerms:      []string{},
			InheritedPerms:   []string{},
			GroupMemberships: []GroupMember{},
		}
		m.userPermissions[userID] = up
	}

	up.Role = role
	up.LastChecked = time.Now()

	// 清除缓存
	m.invalidateCache(userID)

	// 保存
	return m.save()
}
func (m *Manager) GetUserPermissions(userID string) (*UserPermission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	up, exists := m.userPermissions[userID]
	if !exists {
		return nil, ErrUserNotFound
	}

	// 计算有效权限
	m.calculateEffectivePermissions(up)

	return up, nil
}

// GrantPermission 授予用户权限
func (m *Manager) GrantPermission(userID, username, permission string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证权限格式
	if !m.isValidPermission(permission) {
		return ErrInvalidPermission
	}

	up, exists := m.userPermissions[userID]
	if !exists {
		up = &UserPermission{
			UserID:           userID,
			Username:         username,
			Role:             RoleGuest,
			DirectPerms:      []string{},
			InheritedPerms:   []string{},
			GroupMemberships: []GroupMember{},
		}
		m.userPermissions[userID] = up
	}

	// 检查是否已存在
	for _, p := range up.DirectPerms {
		if p == permission {
			return nil // 已存在
		}
	}

	up.DirectPerms = append(up.DirectPerms, permission)
	up.LastChecked = time.Now()

	m.invalidateCache(userID)
	return m.save()
}

// RevokePermission 撤销用户权限
func (m *Manager) RevokePermission(userID, permission string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	up, exists := m.userPermissions[userID]
	if !exists {
		return ErrUserNotFound
	}

	newPerms := make([]string, 0, len(up.DirectPerms))
	for _, p := range up.DirectPerms {
		if p != permission {
			newPerms = append(newPerms, p)
		}
	}
	up.DirectPerms = newPerms

	m.invalidateCache(userID)
	return m.save()
}

// ========== 权限检查 ==========

// CheckPermission 检查用户是否有指定权限
func (m *Manager) CheckPermission(userID, resource, action string) *CheckResult {
	result := &CheckResult{
		Allowed: false,
		Reason:  "默认拒绝",
	}

	// 尝试使用缓存
	if m.config.CacheEnabled {
		if cached := m.getCachedPermissions(userID); cached != nil {
			result = m.checkWithCache(cached, resource, action)
			m.recordAudit(userID, resource, action, result)
			return result
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	up, exists := m.userPermissions[userID]
	if !exists {
		result.Reason = "用户不存在"
		m.recordAudit(userID, resource, action, result)
		return result
	}

	// 计算有效权限
	m.calculateEffectivePermissions(up)

	// 检查权限
	result = m.checkWithUserPermission(up, resource, action)

	// 更新缓存
	if m.config.CacheEnabled {
		m.updateCache(userID, up)
	}

	m.recordAudit(userID, resource, action, result)
	return result
}

// CheckPermissionFast 快速权限检查（不返回详细信息）
func (m *Manager) CheckPermissionFast(userID, resource, action string) bool {
	return m.CheckPermission(userID, resource, action).Allowed
}

// checkWithCache 使用缓存检查权限
func (m *Manager) checkWithCache(cached *PermissionCache, resource, action string) *CheckResult {
	perm := PermissionString(resource, action)

	// 检查是否是管理员
	if cached.Role == RoleAdmin {
		return &CheckResult{
			Allowed:   true,
			Reason:    "管理员拥有所有权限",
			MatchedBy: string(cached.Role),
		}
	}

	// 检查通配符权限
	for _, p := range cached.EffectivePerms {
		if m.matchPermission(p, perm) {
			return &CheckResult{
				Allowed:   true,
				Reason:    "权限匹配",
				MatchedBy: p,
			}
		}
	}

	return &CheckResult{
		Allowed:      false,
		Reason:       "权限不足",
		MissingPerms: []string{perm},
	}
}

// checkWithUserPermission 使用用户权限检查
func (m *Manager) checkWithUserPermission(up *UserPermission, resource, action string) *CheckResult {
	perm := PermissionString(resource, action)

	// 管理员拥有所有权限
	if up.Role == RoleAdmin {
		return &CheckResult{
			Allowed:   true,
			Reason:    "管理员拥有所有权限",
			MatchedBy: string(up.Role),
		}
	}

	// 检查策略（先检查拒绝策略）
	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		// 检查是否匹配主体
		matched := false
		for _, principal := range policy.Principals {
			if principal == up.UserID || principal == "user:"+up.Username {
				matched = true
				break
			}
			// 检查组
			for _, g := range up.GroupMemberships {
				if principal == "group:"+g.GroupID || principal == "group:"+g.GroupName {
					matched = true
					break
				}
			}
		}

		if !matched {
			continue
		}

		// 检查资源和操作
		resourceMatched := false
		for _, r := range policy.Resources {
			if r == "*" || r == resource || m.matchPattern(r, resource) {
				resourceMatched = true
				break
			}
		}

		actionMatched := false
		for _, a := range policy.Actions {
			if a == "*" || a == action || m.matchPattern(a, action) {
				actionMatched = true
				break
			}
		}

		if resourceMatched && actionMatched {
			if policy.Effect == EffectDeny {
				return &CheckResult{
					Allowed:  false,
					Reason:   "策略明确拒绝",
					DeniedBy: policy.Name,
				}
			}
			return &CheckResult{
				Allowed:   true,
				Reason:    "策略允许",
				MatchedBy: policy.Name,
			}
		}
	}

	// 检查角色权限
	roleInfo, ok := DefaultRoles[up.Role]
	if !ok {
		roleInfo, ok = m.customRoles[up.Role]
	}

	if ok {
		for _, p := range roleInfo.Permissions {
			if m.matchPermission(p, perm) {
				return &CheckResult{
					Allowed:   true,
					Reason:    "角色权限匹配",
					MatchedBy: string(up.Role),
				}
			}
		}
	}

	// 检查有效权限
	for _, p := range up.EffectivePerms {
		if m.matchPermission(p, perm) {
			return &CheckResult{
				Allowed:   true,
				Reason:    "直接权限匹配",
				MatchedBy: p,
			}
		}
	}

	return &CheckResult{
		Allowed:      false,
		Reason:       "权限不足",
		MissingPerms: []string{perm},
	}
}

// calculateEffectivePermissions 计算有效权限
func (m *Manager) calculateEffectivePermissions(up *UserPermission) {
	effective := make(map[string]bool)

	// 角色权限
	roleInfo, ok := DefaultRoles[up.Role]
	if !ok {
		roleInfo, ok = m.customRoles[up.Role]
	}
	if ok {
		for _, p := range roleInfo.Permissions {
			effective[p] = true
		}
	}

	// 直接权限
	for _, p := range up.DirectPerms {
		effective[p] = true
	}

	// 继承权限
	inherited := make(map[string]bool)
	for _, gm := range up.GroupMemberships {
		if gp, exists := m.groupPerms[gm.GroupID]; exists {
			for _, p := range gp.Permissions {
				effective[p] = true
				inherited[p] = true
			}
		}
	}

	// 转换为切片
	up.EffectivePerms = make([]string, 0, len(effective))
	for p := range effective {
		up.EffectivePerms = append(up.EffectivePerms, p)
	}

	up.InheritedPerms = make([]string, 0, len(inherited))
	for p := range inherited {
		up.InheritedPerms = append(up.InheritedPerms, p)
	}
}

// ========== 用户组权限管理 ==========

// CreateGroupPermission 创建用户组权限
func (m *Manager) CreateGroupPermission(groupID, groupName string, permissions []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gp := &GroupPermission{
		GroupID:     groupID,
		GroupName:   groupName,
		Permissions: permissions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.groupPerms[groupID] = gp

	// 更新组成员的有效权限
	for _, up := range m.userPermissions {
		for _, gm := range up.GroupMemberships {
			if gm.GroupID == groupID {
				m.invalidateCache(up.UserID)
				break
			}
		}
	}

	return m.save()
}

// UpdateGroupPermission 更新用户组权限
func (m *Manager) UpdateGroupPermission(groupID string, permissions []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gp, exists := m.groupPerms[groupID]
	if !exists {
		return ErrGroupNotFound
	}

	gp.Permissions = permissions
	gp.UpdatedAt = time.Now()

	// 更新组成员缓存
	for _, up := range m.userPermissions {
		for _, gm := range up.GroupMemberships {
			if gm.GroupID == groupID {
				m.invalidateCache(up.UserID)
				break
			}
		}
	}

	return m.save()
}

// DeleteGroupPermission 删除用户组权限
func (m *Manager) DeleteGroupPermission(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.groupPerms[groupID]; !exists {
		return ErrGroupNotFound
	}

	delete(m.groupPerms, groupID)

	// 更新组成员缓存
	for _, up := range m.userPermissions {
		for _, gm := range up.GroupMemberships {
			if gm.GroupID == groupID {
				m.invalidateCache(up.UserID)
				break
			}
		}
	}

	m.save()
	return nil
}

// AddUserToGroup 将用户添加到组
func (m *Manager) AddUserToGroup(userID, username, groupID, groupName string, isOwner bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	up, exists := m.userPermissions[userID]
	if !exists {
		up = &UserPermission{
			UserID:           userID,
			Username:         username,
			Role:             RoleGuest,
			DirectPerms:      []string{},
			GroupMemberships: []GroupMember{},
		}
		m.userPermissions[userID] = up
	}

	// 检查是否已在组中
	for _, gm := range up.GroupMemberships {
		if gm.GroupID == groupID {
			return nil // 已在组中
		}
	}

	up.GroupMemberships = append(up.GroupMemberships, GroupMember{
		GroupID:   groupID,
		GroupName: groupName,
		IsOwner:   isOwner,
	})

	m.invalidateCache(userID)
	m.save()
	return nil
}

// RemoveUserFromGroup 从组中移除用户
func (m *Manager) RemoveUserFromGroup(userID, groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	up, exists := m.userPermissions[userID]
	if !exists {
		return ErrUserNotFound
	}

	newMemberships := make([]GroupMember, 0)
	for _, gm := range up.GroupMemberships {
		if gm.GroupID != groupID {
			newMemberships = append(newMemberships, gm)
		}
	}
	up.GroupMemberships = newMemberships

	m.invalidateCache(userID)
	m.save()
	return nil
}

// ========== 策略管理 ==========

// CreatePolicy 创建策略
func (m *Manager) CreatePolicy(name, description string, effect PolicyEffect, principals, resources, actions []string, priority int) (*Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy := &Policy{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Effect:      effect,
		Principals:  principals,
		Resources:   resources,
		Actions:     actions,
		Priority:    priority,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.policies[policy.ID] = policy

	// 清除相关用户缓存
	for _, principal := range principals {
		if strings.HasPrefix(principal, "user:") {
			// 清除用户缓存
			for _, up := range m.userPermissions {
				if "user:"+up.Username == principal {
					m.invalidateCache(up.UserID)
				}
			}
		}
	}

	m.save()
	return policy, nil
}

// UpdatePolicy 更新策略
func (m *Manager) UpdatePolicy(policyID string, enabled *bool, priority *int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[policyID]
	if !exists {
		return ErrPolicyNotFound
	}

	if enabled != nil {
		policy.Enabled = *enabled
	}
	if priority != nil {
		policy.Priority = *priority
	}
	policy.UpdatedAt = time.Now()

	m.save()
	return nil
}

// DeletePolicy 删除策略
func (m *Manager) DeletePolicy(policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[policyID]; !exists {
		return ErrPolicyNotFound
	}

	delete(m.policies, policyID)
	m.save()
	return nil
}

// ListPolicies 列出所有策略
func (m *Manager) ListPolicies() []*Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*Policy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// ========== 缓存管理 ==========

func (m *Manager) getCachedPermissions(userID string) *PermissionCache {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	cached, exists := m.cache[userID]
	if !exists || time.Now().After(cached.ExpiresAt) {
		return nil
	}

	return cached
}

func (m *Manager) updateCache(userID string, up *UserPermission) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	m.cache[userID] = &PermissionCache{
		UserID:         userID,
		EffectivePerms: up.EffectivePerms,
		Groups:         getGroupIDs(up.GroupMemberships),
		Role:           up.Role,
		UpdatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(m.config.CacheTTL),
	}
}

func (m *Manager) invalidateCache(userID string) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	delete(m.cache, userID)
}

// InvalidateAllCaches 清除所有缓存
func (m *Manager) InvalidateAllCaches() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	m.cache = make(map[string]*PermissionCache)
}

// ========== 辅助方法 ==========

func (m *Manager) isValidRole(role Role) bool {
	if _, ok := DefaultRoles[role]; ok {
		return true
	}
	if _, ok := m.customRoles[role]; ok {
		return true
	}
	return false
}

func (m *Manager) isValidPermission(perm string) bool {
	if perm == "*:*" {
		return true
	}

	resource, action := ParsePermission(perm)
	if resource == "" || action == "" {
		return false
	}

	return true
}

func (m *Manager) matchPermission(pattern, perm string) bool {
	if pattern == "*:*" || pattern == "*" {
		return true
	}

	if pattern == perm {
		return true
	}

	// 支持通配符: resource:* 或 *:action
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(perm, prefix)
	}

	if strings.HasPrefix(pattern, "*:") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(perm, suffix)
	}

	return false
}

func (m *Manager) matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(value, pattern[1:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(value, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, pattern[:len(pattern)-1])
	}
	return pattern == value
}

func (m *Manager) recordAudit(userID, resource, action string, result *CheckResult) {
	if m.config.AuditEnabled && m.auditCallback != nil {
		m.auditCallback(userID, resource, action, result)
	}
}

func getGroupIDs(memberships []GroupMember) []string {
	ids := make([]string, len(memberships))
	for i, m := range memberships {
		ids[i] = m.GroupID
	}
	return ids
}

// ========== 持久化 ==========

type persistentData struct {
	UserPermissions map[string]*UserPermission  `json:"user_permissions"`
	GroupPerms      map[string]*GroupPermission `json:"group_permissions"`
	Policies        map[string]*Policy          `json:"policies"`
	CustomRoles     map[Role]RoleInfo           `json:"custom_roles"`
}

func (m *Manager) load() error {
	if m.config.ConfigPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.config.ConfigPath)
	if err != nil {
		return err
	}

	var pd persistentData
	if err := json.Unmarshal(data, &pd); err != nil {
		return err
	}

	if pd.UserPermissions != nil {
		m.userPermissions = pd.UserPermissions
	}
	if pd.GroupPerms != nil {
		m.groupPerms = pd.GroupPerms
	}
	if pd.Policies != nil {
		m.policies = pd.Policies
	}
	if pd.CustomRoles != nil {
		m.customRoles = pd.CustomRoles
	}

	return nil
}

func (m *Manager) save() error {
	if m.config.ConfigPath == "" {
		return nil
	}

	pd := persistentData{
		UserPermissions: m.userPermissions,
		GroupPerms:      m.groupPerms,
		Policies:        m.policies,
		CustomRoles:     m.customRoles,
	}

	data, err := json.MarshalIndent(pd, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(m.config.ConfigPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(m.config.ConfigPath, data, 0644)
}

// DeleteUser 删除用户权限
func (m *Manager) DeleteUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.userPermissions[userID]; !exists {
		return ErrUserNotFound
	}

	delete(m.userPermissions, userID)
	m.invalidateCache(userID)
	m.save()

	return nil
}

// ListUserPermissions 列出所有用户权限
func (m *Manager) ListUserPermissions() []*UserPermission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*UserPermission, 0, len(m.userPermissions))
	for _, up := range m.userPermissions {
		m.calculateEffectivePermissions(up)
		result = append(result, up)
	}
	return result
}

// GetRoleInfo 获取角色信息
func (m *Manager) GetRoleInfo(role Role) (*RoleInfo, bool) {
	if info, ok := DefaultRoles[role]; ok {
		return &info, true
	}
	if info, ok := m.customRoles[role]; ok {
		return &info, true
	}
	return nil, false
}

// ListRoles 列出所有角色
func (m *Manager) ListRoles() []RoleInfo {
	roles := make([]RoleInfo, 0, len(DefaultRoles)+len(m.customRoles))

	for _, info := range DefaultRoles {
		roles = append(roles, info)
	}
	for _, info := range m.customRoles {
		roles = append(roles, info)
	}

	return roles
}
