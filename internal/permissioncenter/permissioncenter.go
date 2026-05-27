// Package permissioncenter 提供集中式RBAC权限管理功能
package permissioncenter

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// PermissionCenter 权限管理中心.
type PermissionCenter struct {
	mu sync.RWMutex

	// roles 角色存储，key=roleID.
	roles map[string]*Role
	// permissions 权限存储，key=permissionID.
	permissions map[string]*Permission
	// userRoles 用户-角色关联，key=userID -> []roleID.
	userRoles map[string][]*UserRole
	// delegations 权限委托，key=delegationID.
	delegations map[string]*Delegation
	// tempGrants 临时授权，key=grantID.
	tempGrants map[string]*TempGrant
	// auditLogs 审计日志.
	auditLogs []*AuditLog
	// maxAuditLogs 最大审计日志数量.
	maxAuditLogs int
}

// NewPermissionCenter 创建权限管理中心.
func NewPermissionCenter() *PermissionCenter {
	pc := &PermissionCenter{
		roles:        make(map[string]*Role),
		permissions:  make(map[string]*Permission),
		userRoles:    make(map[string][]*UserRole),
		delegations:  make(map[string]*Delegation),
		tempGrants:   make(map[string]*TempGrant),
		auditLogs:    make([]*AuditLog, 0),
		maxAuditLogs: 10000,
	}
	// 初始化系统内置角色
	pc.initSystemRoles()
	return pc
}

// initSystemRoles 初始化系统内置角色.
func (pc *PermissionCenter) initSystemRoles() {
	// 超级管理员
	pc.roles["admin"] = &Role{
		ID:          "admin",
		Name:        "超级管理员",
		Description: "系统超级管理员，拥有所有权限",
		IsSystem:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	// 普通用户
	pc.roles["user"] = &Role{
		ID:          "user",
		Name:        "普通用户",
		Description: "普通用户，拥有基本权限",
		IsSystem:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	// 访客
	pc.roles["guest"] = &Role{
		ID:          "guest",
		Name:        "访客",
		Description: "访客，仅有只读权限",
		IsSystem:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// ========== 角色管理 ==========

// CreateRole 创建角色.
func (pc *PermissionCenter) CreateRole(role *Role) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if role.ID == "" || role.Name == "" {
		return ErrInvalidParams
	}

	if _, exists := pc.roles[role.ID]; exists {
		return ErrRoleAlreadyExists
	}

	// 检查父角色是否存在
	if role.ParentID != "" {
		if _, exists := pc.roles[role.ParentID]; !exists {
			return ErrRoleNotFound
		}
		// 检查是否会造成循环继承
		if pc.wouldCreateCycle(role.ID, role.ParentID) {
			return ErrCircularInheritance
		}
	}

	now := time.Now()
	role.CreatedAt = now
	role.UpdatedAt = now
	if role.Permissions == nil {
		role.Permissions = make([]string, 0)
	}

	pc.roles[role.ID] = role
	pc.addAuditLog(AuditRoleCreated, "", role.ID, fmt.Sprintf("创建角色: %s", role.Name), "", toJSON(role))

	return nil
}

// UpdateRole 更新角色.
func (pc *PermissionCenter) UpdateRole(role *Role) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	existing, exists := pc.roles[role.ID]
	if !exists {
		return ErrRoleNotFound
	}

	if existing.IsSystem {
		return ErrInvalidParams
	}

	// 检查父角色
	if role.ParentID != "" {
		if _, exists := pc.roles[role.ParentID]; !exists {
			return ErrRoleNotFound
		}
		if pc.wouldCreateCycle(role.ID, role.ParentID) {
			return ErrCircularInheritance
		}
	}

	before := toJSON(existing)
	role.UpdatedAt = time.Now()
	role.CreatedAt = existing.CreatedAt
	pc.roles[role.ID] = role
	pc.addAuditLog(AuditRoleUpdated, "", role.ID, fmt.Sprintf("更新角色: %s", role.Name), before, toJSON(role))

	return nil
}

// DeleteRole 删除角色.
func (pc *PermissionCenter) DeleteRole(roleID string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	role, exists := pc.roles[roleID]
	if !exists {
		return ErrRoleNotFound
	}

	if role.IsSystem {
		return ErrInvalidParams
	}

	// 检查是否有子角色
	for _, r := range pc.roles {
		if r.ParentID == roleID {
			return ErrInvalidParams
		}
	}

	// 检查是否有用户使用该角色
	for _, userRolesList := range pc.userRoles {
		for _, ur := range userRolesList {
			if ur.RoleID == roleID {
				return ErrInvalidParams
			}
		}
	}

	before := toJSON(role)
	delete(pc.roles, roleID)
	pc.addAuditLog(AuditRoleDeleted, "", roleID, fmt.Sprintf("删除角色: %s", role.Name), before, "")

	return nil
}

// GetRole 获取角色.
func (pc *PermissionCenter) GetRole(roleID string) (*Role, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	role, exists := pc.roles[roleID]
	if !exists {
		return nil, ErrRoleNotFound
	}

	return role, nil
}

// ListRoles 列出角色.
func (pc *PermissionCenter) ListRoles(params *QueryParams) *RoleListResult {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	roles := make([]*Role, 0, len(pc.roles))
	for _, r := range pc.roles {
		roles = append(roles, r)
	}

	total := len(roles)
	offset := 0
	limit := 50

	if params != nil {
		if params.Offset > 0 {
			offset = params.Offset
		}
		if params.Limit > 0 {
			limit = params.Limit
		}
	}

	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return &RoleListResult{
		Roles:  roles[offset:end],
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// wouldCreateCycle 检查是否会造成循环继承.
func (pc *PermissionCenter) wouldCreateCycle(roleID, parentID string) bool {
	visited := make(map[string]bool)
	current := parentID
	for current != "" {
		if current == roleID {
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		role, exists := pc.roles[current]
		if !exists {
			return false
		}
		current = role.ParentID
	}
	return false
}

// ========== 权限管理 ==========

// CreatePermission 创建权限.
func (pc *PermissionCenter) CreatePermission(perm *Permission) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if perm.ID == "" || perm.Name == "" || perm.Resource == "" || perm.Action == "" {
		return ErrInvalidParams
	}

	if _, exists := pc.permissions[perm.ID]; exists {
		return ErrPermissionAlreadyExists
	}

	now := time.Now()
	perm.CreatedAt = now
	perm.UpdatedAt = now
	pc.permissions[perm.ID] = perm

	pc.addAuditLog(AuditPermAssigned, "", perm.ID, fmt.Sprintf("创建权限: %s", perm.Name), "", toJSON(perm))

	return nil
}

// UpdatePermission 更新权限.
func (pc *PermissionCenter) UpdatePermission(perm *Permission) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	existing, exists := pc.permissions[perm.ID]
	if !exists {
		return ErrPermissionNotFound
	}

	before := toJSON(existing)
	perm.UpdatedAt = time.Now()
	perm.CreatedAt = existing.CreatedAt
	pc.permissions[perm.ID] = perm

	pc.addAuditLog(AuditPermAssigned, "", perm.ID, fmt.Sprintf("更新权限: %s", perm.Name), before, toJSON(perm))

	return nil
}

// DeletePermission 删除权限.
func (pc *PermissionCenter) DeletePermission(permID string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	perm, exists := pc.permissions[permID]
	if !exists {
		return ErrPermissionNotFound
	}

	// 检查是否有角色引用该权限
	for _, role := range pc.roles {
		for _, pid := range role.Permissions {
			if pid == permID {
				return ErrInvalidParams
			}
		}
	}

	before := toJSON(perm)
	delete(pc.permissions, permID)
	pc.addAuditLog(AuditPermRevoked, "", permID, fmt.Sprintf("删除权限: %s", perm.Name), before, "")

	return nil
}

// GetPermission 获取权限.
func (pc *PermissionCenter) GetPermission(permID string) (*Permission, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	perm, exists := pc.permissions[permID]
	if !exists {
		return nil, ErrPermissionNotFound
	}

	return perm, nil
}

// ListPermissions 列出权限.
func (pc *PermissionCenter) ListPermissions(permType *PermissionType, params *QueryParams) *PermissionListResult {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	perms := make([]*Permission, 0, len(pc.permissions))
	for _, p := range pc.permissions {
		if permType == nil || p.Type == *permType {
			perms = append(perms, p)
		}
	}

	total := len(perms)
	offset := 0
	limit := 50

	if params != nil {
		if params.Offset > 0 {
			offset = params.Offset
		}
		if params.Limit > 0 {
			limit = params.Limit
		}
	}

	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return &PermissionListResult{
		Permissions: perms[offset:end],
		Total:       total,
		Limit:       limit,
		Offset:      offset,
	}
}

// ========== 角色-权限关联 ==========

// AssignPermissionToRole 为角色分配权限.
func (pc *PermissionCenter) AssignPermissionToRole(roleID, permID string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	role, exists := pc.roles[roleID]
	if !exists {
		return ErrRoleNotFound
	}

	if _, exists := pc.permissions[permID]; !exists {
		return ErrPermissionNotFound
	}

	// 检查是否已分配
	for _, pid := range role.Permissions {
		if pid == permID {
			return nil
		}
	}

	role.Permissions = append(role.Permissions, permID)
	role.UpdatedAt = time.Now()

	pc.addAuditLog(AuditPermAssigned, "", roleID, fmt.Sprintf("为角色分配权限: %s -> %s", roleID, permID), "", "")

	return nil
}

// RevokePermissionFromRole 从角色回收权限.
func (pc *PermissionCenter) RevokePermissionFromRole(roleID, permID string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	role, exists := pc.roles[roleID]
	if !exists {
		return ErrRoleNotFound
	}

	found := false
	newPerms := make([]string, 0, len(role.Permissions))
	for _, pid := range role.Permissions {
		if pid == permID {
			found = true
		} else {
			newPerms = append(newPerms, pid)
		}
	}

	if !found {
		return ErrPermissionNotFound
	}

	role.Permissions = newPerms
	role.UpdatedAt = time.Now()

	pc.addAuditLog(AuditPermRevoked, "", roleID, fmt.Sprintf("从角色回收权限: %s -> %s", roleID, permID), "", "")

	return nil
}

// GetRolePermissions 获取角色的所有权限（包括继承的）.
func (pc *PermissionCenter) GetRolePermissions(roleID string) ([]*Permission, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	role, exists := pc.roles[roleID]
	if !exists {
		return nil, ErrRoleNotFound
	}

	permSet := make(map[string]bool)
	result := make([]*Permission, 0)

	// 递归收集权限
	pc.collectRolePermissions(role, permSet, &result, make(map[string]bool))

	return result, nil
}

// collectRolePermissions 递归收集角色权限.
func (pc *PermissionCenter) collectRolePermissions(role *Role, permSet map[string]bool, result *[]*Permission, visited map[string]bool) {
	if visited[role.ID] {
		return
	}
	visited[role.ID] = true

	// 收集当前角色的权限
	for _, pid := range role.Permissions {
		if !permSet[pid] {
			permSet[pid] = true
			if perm, exists := pc.permissions[pid]; exists {
				*result = append(*result, perm)
			}
		}
	}

	// 递归收集父角色的权限
	if role.ParentID != "" {
		if parent, exists := pc.roles[role.ParentID]; exists {
			pc.collectRolePermissions(parent, permSet, result, visited)
		}
	}
}

// ========== 用户-角色管理 ==========

// AssignRoleToUser 为用户分配角色.
func (pc *PermissionCenter) AssignRoleToUser(userID, roleID, grantedBy string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if _, exists := pc.roles[roleID]; !exists {
		return ErrRoleNotFound
	}

	// 检查是否已分配
	for _, ur := range pc.userRoles[userID] {
		if ur.RoleID == roleID {
			return nil
		}
	}

	ur := &UserRole{
		UserID:    userID,
		RoleID:    roleID,
		GrantedBy: grantedBy,
		GrantedAt: time.Now(),
	}

	pc.userRoles[userID] = append(pc.userRoles[userID], ur)
	pc.addAuditLog(AuditUserRoleAssigned, grantedBy, userID, fmt.Sprintf("为用户分配角色: %s -> %s", userID, roleID), "", "")

	return nil
}

// RevokeRoleFromUser 从用户回收角色.
func (pc *PermissionCenter) RevokeRoleFromUser(userID, roleID string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	found := false
	newRoles := make([]*UserRole, 0)
	for _, ur := range pc.userRoles[userID] {
		if ur.RoleID == roleID {
			found = true
		} else {
			newRoles = append(newRoles, ur)
		}
	}

	if !found {
		return ErrRoleNotFound
	}

	pc.userRoles[userID] = newRoles
	pc.addAuditLog(AuditUserRoleRevoked, "", userID, fmt.Sprintf("从用户回收角色: %s -> %s", userID, roleID), "", "")

	return nil
}

// GetUserRoles 获取用户的角色列表.
func (pc *PermissionCenter) GetUserRoles(userID string) []*Role {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	roles := make([]*Role, 0)
	for _, ur := range pc.userRoles[userID] {
		if role, exists := pc.roles[ur.RoleID]; exists {
			roles = append(roles, role)
		}
	}

	return roles
}

// ========== 权限继承 ==========

// GetEffectivePermissions 获取用户的有效权限（包括继承、委托、临时授权）.
func (pc *PermissionCenter) GetEffectivePermissions(userID string) (*UserPermissionSummary, error) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	summary := &UserPermissionSummary{
		UserID:          userID,
		Roles:           make([]*Role, 0),
		DirectPermissions: make([]*Permission, 0),
		DelegatedPermissions: make([]*DelegatedPermission, 0),
		TempPermissions: make([]*TempGrant, 0),
		AllPermissions:  make([]*Permission, 0),
	}

	permSet := make(map[string]bool)
	permList := make([]*Permission, 0)

	// 1. 收集用户直接角色的权限
	for _, ur := range pc.userRoles[userID] {
		role, exists := pc.roles[ur.RoleID]
		if !exists {
			continue
		}
		summary.Roles = append(summary.Roles, role)

		// 收集角色权限（包括继承）
		pc.collectRolePermissions(role, permSet, &permList, make(map[string]bool))
	}

	// 2. 收集委托的权限
	now := time.Now()
	for _, d := range pc.delegations {
		if d.ToUserID == userID && d.IsActive && now.After(d.StartTime) && now.Before(d.EndTime) {
			// 获取被委托的角色权限
			if role, exists := pc.roles[d.RoleID]; exists {
				delegatedPerms := make([]*Permission, 0)
				if len(d.DelegatedPermissions) > 0 {
					// 只委托指定权限
					for _, pid := range d.DelegatedPermissions {
						if perm, exists := pc.permissions[pid]; exists {
							delegatedPerms = append(delegatedPerms, perm)
							if !permSet[pid] {
								permSet[pid] = true
								permList = append(permList, perm)
							}
						}
					}
				} else {
					// 委托角色的所有权限
					rolePerms := make([]*Permission, 0)
					pc.collectRolePermissions(role, permSet, &rolePerms, make(map[string]bool))
					delegatedPerms = rolePerms
					permList = append(permList, rolePerms...)
				}

				for _, perm := range delegatedPerms {
					summary.DelegatedPermissions = append(summary.DelegatedPermissions, &DelegatedPermission{
						Delegation: d,
						Permission: perm,
					})
				}
			}
		}
	}

	// 3. 收集临时授权
	for _, tg := range pc.tempGrants {
		if tg.UserID == userID && tg.IsActive && now.After(tg.StartTime) && now.Before(tg.EndTime) {
			summary.TempPermissions = append(summary.TempPermissions, tg)
			if perm, exists := pc.permissions[tg.PermissionID]; exists {
				if !permSet[perm.ID] {
					permSet[perm.ID] = true
					permList = append(permList, perm)
				}
			}
		}
	}

	summary.DirectPermissions = permList
	summary.AllPermissions = permList

	return summary, nil
}

// ========== 权限委托 ==========

// CreateDelegation 创建权限委托.
func (pc *PermissionCenter) CreateDelegation(d *Delegation) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if d.ID == "" || d.FromUserID == "" || d.ToUserID == "" || d.RoleID == "" {
		return ErrInvalidParams
	}

	if _, exists := pc.roles[d.RoleID]; !exists {
		return ErrRoleNotFound
	}

	// 检查被委托人是否拥有该角色
	hasRole := false
	for _, ur := range pc.userRoles[d.FromUserID] {
		if ur.RoleID == d.RoleID {
			hasRole = true
			break
		}
	}
	if !hasRole {
		return ErrInvalidDelegation
	}

	// 检查时间有效性
	if d.EndTime.Before(d.StartTime) {
		return ErrInvalidParams
	}

	now := time.Now()
	d.IsActive = true
	d.CreatedAt = now

	pc.delegations[d.ID] = d
	pc.addAuditLog(AuditDelegationCreated, d.FromUserID, d.ID, fmt.Sprintf("创建委托: %s -> %s", d.FromUserID, d.ToUserID), "", toJSON(d))

	return nil
}

// RevokeDelegation 撤销委托.
func (pc *PermissionCenter) RevokeDelegation(delegationID string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	d, exists := pc.delegations[delegationID]
	if !exists {
		return ErrDelegationNotFound
	}

	d.IsActive = false
	pc.addAuditLog(AuditDelegationRevoked, "", delegationID, fmt.Sprintf("撤销委托: %s", delegationID), "", "")

	return nil
}

// GetUserDelegations 获取用户的委托列表（作为委托人或被委托人）.
func (pc *PermissionCenter) GetUserDelegations(userID string, asDelegator bool) []*Delegation {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := make([]*Delegation, 0)
	for _, d := range pc.delegations {
		if asDelegator && d.FromUserID == userID {
			result = append(result, d)
		} else if !asDelegator && d.ToUserID == userID {
			result = append(result, d)
		}
	}

	return result
}

// ========== 临时授权 ==========

// CreateTempGrant 创建临时授权.
func (pc *PermissionCenter) CreateTempGrant(tg *TempGrant) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if tg.ID == "" || tg.UserID == "" || tg.PermissionID == "" {
		return ErrInvalidParams
	}

	if _, exists := pc.permissions[tg.PermissionID]; !exists {
		return ErrPermissionNotFound
	}

	if tg.EndTime.Before(tg.StartTime) {
		return ErrInvalidParams
	}

	now := time.Now()
	tg.IsActive = true
	tg.CreatedAt = now

	pc.tempGrants[tg.ID] = tg
	pc.addAuditLog(AuditTempGrantCreated, tg.GrantedBy, tg.ID, fmt.Sprintf("创建临时授权: %s -> %s", tg.UserID, tg.PermissionID), "", toJSON(tg))

	return nil
}

// RevokeTempGrant 撤销临时授权.
func (pc *PermissionCenter) RevokeTempGrant(grantID string) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	tg, exists := pc.tempGrants[grantID]
	if !exists {
		return ErrTempGrantNotFound
	}

	tg.IsActive = false
	pc.addAuditLog(AuditTempGrantRevoked, "", grantID, fmt.Sprintf("撤销临时授权: %s", grantID), "", "")

	return nil
}

// GetUserTempGrants 获取用户的临时授权列表.
func (pc *PermissionCenter) GetUserTempGrants(userID string) []*TempGrant {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := make([]*TempGrant, 0)
	now := time.Now()
	for _, tg := range pc.tempGrants {
		if tg.UserID == userID && tg.IsActive && now.After(tg.StartTime) && now.Before(tg.EndTime) {
			result = append(result, tg)
		}
	}

	return result
}

// ========== 访问控制 ==========

// CheckAccess 检查用户是否有权访问资源.
func (pc *PermissionCenter) CheckAccess(req *AccessRequest) *AccessResult {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := &AccessResult{
		Allowed:            false,
		MatchedPermissions: make([]string, 0),
		AppliedScopes:      make([]DataScope, 0),
	}

	if req.UserID == "" || req.Resource == "" || req.Action == "" {
		result.Reason = "无效的请求参数"
		pc.addAuditLog(AuditAccessChecked, req.UserID, "", fmt.Sprintf("访问检查失败: %s %s %s - %s", req.UserID, req.Resource, req.Action, result.Reason), "", "")
		return result
	}

	// 1. 检查用户角色权限
	permSet := make(map[string]bool)
	for _, ur := range pc.userRoles[req.UserID] {
		role, exists := pc.roles[ur.RoleID]
		if !exists {
			continue
		}

		// 管理员角色拥有所有权限
		if role.ID == "admin" {
			result.Allowed = true
			result.Reason = "管理员权限"
			pc.addAuditLog(AuditAccessChecked, req.UserID, "", fmt.Sprintf("访问检查通过: %s %s %s - 管理员权限", req.UserID, req.Resource, req.Action), "", "")
			return result
		}

		// 收集角色权限
		rolePerms := make([]*Permission, 0)
		pc.collectRolePermissions(role, permSet, &rolePerms, make(map[string]bool))

		for _, perm := range rolePerms {
			if pc.matchPermission(perm, req) {
				result.Allowed = true
				result.MatchedPermissions = append(result.MatchedPermissions, perm.ID)
				if perm.DataScope != "" {
					result.AppliedScopes = append(result.AppliedScopes, perm.DataScope)
				}
			}
		}
	}

	// 2. 检查委托权限
	now := time.Now()
	for _, d := range pc.delegations {
		if d.ToUserID == req.UserID && d.IsActive && now.After(d.StartTime) && now.Before(d.EndTime) {
			if role, exists := pc.roles[d.RoleID]; exists {
				rolePerms := make([]*Permission, 0)
				pc.collectRolePermissions(role, permSet, &rolePerms, make(map[string]bool))

				for _, perm := range rolePerms {
					if len(d.DelegatedPermissions) > 0 {
						// 检查是否在委托权限列表中
						delegated := false
						for _, pid := range d.DelegatedPermissions {
							if pid == perm.ID {
								delegated = true
								break
							}
						}
						if !delegated {
							continue
						}
					}

					if pc.matchPermission(perm, req) {
						result.Allowed = true
						result.IsDelegated = true
						result.MatchedPermissions = append(result.MatchedPermissions, perm.ID)
						if perm.DataScope != "" {
							result.AppliedScopes = append(result.AppliedScopes, perm.DataScope)
						}
					}
				}
			}
		}
	}

	// 3. 检查临时授权
	for _, tg := range pc.tempGrants {
		if tg.UserID == req.UserID && tg.IsActive && now.After(tg.StartTime) && now.Before(tg.EndTime) {
			if perm, exists := pc.permissions[tg.PermissionID]; exists {
				// 如果临时授权覆盖了资源，使用覆盖的资源
				permCopy := *perm
				if tg.Resource != "" {
					permCopy.Resource = tg.Resource
				}
				if pc.matchPermission(&permCopy, req) {
					result.Allowed = true
					result.IsTempGrant = true
					result.MatchedPermissions = append(result.MatchedPermissions, perm.ID)
					if perm.DataScope != "" {
						result.AppliedScopes = append(result.AppliedScopes, perm.DataScope)
					}
				}
			}
		}
	}

	if !result.Allowed {
		result.Reason = "没有匹配的权限"
	}

	auditDetail := fmt.Sprintf("访问检查: %s %s %s - 允许=%v", req.UserID, req.Resource, req.Action, result.Allowed)
	pc.addAuditLog(AuditAccessChecked, req.UserID, "", auditDetail, "", "")

	return result
}

// matchPermission 检查权限是否匹配请求.
func (pc *PermissionCenter) matchPermission(perm *Permission, req *AccessRequest) bool {
	// 检查资源匹配
	if !pc.matchResource(perm.Resource, req.Resource) {
		return false
	}

	// 检查操作匹配
	if perm.Action != "*" && perm.Action != req.Action {
		return false
	}

	// 检查额外条件
	if len(perm.Conditions) > 0 && req.Context != nil {
		for key, value := range perm.Conditions {
			if ctxValue, exists := req.Context[key]; !exists || ctxValue != value {
				return false
			}
		}
	}

	return true
}

// matchResource 匹配资源路径（支持通配符）.
func (pc *PermissionCenter) matchResource(pattern, resource string) bool {
	// 精确匹配
	if pattern == resource {
		return true
	}

	// 通配符匹配
	if pattern == "*" {
		return true
	}

	// 前缀匹配: /api/* 匹配 /api/users
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(resource, prefix+"/") || resource == prefix
	}

	// 后缀匹配: *.txt 匹配 file.txt
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(resource, suffix)
	}

	return false
}

// CheckBatchAccess 批量检查权限.
func (pc *PermissionCenter) CheckBatchAccess(userID string, requests []AccessRequest) []*PermissionCheckResult {
	results := make([]*PermissionCheckResult, len(requests))

	for i, req := range requests {
		req.UserID = userID
		accessResult := pc.CheckAccess(&req)
		results[i] = &PermissionCheckResult{
			Resource: req.Resource,
			Action:   string(req.Action),
			Allowed:  accessResult.Allowed,
			Reason:   accessResult.Reason,
		}
	}

	return results
}

// ========== 审计日志 ==========

// addAuditLog 添加审计日志.
func (pc *PermissionCenter) addAuditLog(action AuditAction, userID, targetID, details, before, after string) {
	log := &AuditLog{
		ID:        fmt.Sprintf("audit_%d_%d", time.Now().UnixNano(), len(pc.auditLogs)),
		Timestamp: time.Now(),
		UserID:    userID,
		Action:    action,
		TargetID:  targetID,
		Details:   details,
		Before:    before,
		After:     after,
	}

	pc.auditLogs = append(pc.auditLogs, log)

	// 限制日志数量
	if len(pc.auditLogs) > pc.maxAuditLogs {
		pc.auditLogs = pc.auditLogs[len(pc.auditLogs)-pc.maxAuditLogs:]
	}
}

// GetAuditLogs 获取审计日志.
func (pc *PermissionCenter) GetAuditLogs(userID string, action *AuditAction, params *QueryParams) *AuditLogListResult {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	logs := make([]*AuditLog, 0)
	for _, log := range pc.auditLogs {
		if userID != "" && log.UserID != userID {
			continue
		}
		if action != nil && log.Action != *action {
			continue
		}
		logs = append(logs, log)
	}

	// 倒序排列（最新的在前）
	total := len(logs)
	for i, j := 0, total-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	offset := 0
	limit := 50

	if params != nil {
		if params.Offset > 0 {
			offset = params.Offset
		}
		if params.Limit > 0 {
			limit = params.Limit
		}
	}

	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return &AuditLogListResult{
		Logs:   logs[offset:end],
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// ========== 辅助函数 ==========

// toJSON 将对象转为JSON字符串.
func toJSON(v interface{}) string {
	// 简化实现，实际应使用json.Marshal
	return fmt.Sprintf("%+v", v)
}

// GetUserPermissionSummary 获取用户权限汇总.
func (pc *PermissionCenter) GetUserPermissionSummary(userID string) (*UserPermissionSummary, error) {
	summary, err := pc.GetEffectivePermissions(userID)
	if err != nil {
		return nil, err
	}

	// 获取委托的权限
	summary.DelegatedPermissions = make([]*DelegatedPermission, 0)
	now := time.Now()
	for _, d := range pc.delegations {
		if d.ToUserID == userID && d.IsActive && now.After(d.StartTime) && now.Before(d.EndTime) {
			if role, exists := pc.roles[d.RoleID]; exists {
				rolePerms := make([]*Permission, 0)
				pc.collectRolePermissions(role, make(map[string]bool), &rolePerms, make(map[string]bool))

				for _, perm := range rolePerms {
					if len(d.DelegatedPermissions) > 0 {
						delegated := false
						for _, pid := range d.DelegatedPermissions {
							if pid == perm.ID {
								delegated = true
								break
							}
						}
						if !delegated {
							continue
						}
					}
					summary.DelegatedPermissions = append(summary.DelegatedPermissions, &DelegatedPermission{
						Delegation: d,
						Permission: perm,
					})
				}
			}
		}
	}

	return summary, nil
}
