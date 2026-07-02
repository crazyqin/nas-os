package acl

import (
	"fmt"
	"sync"
	"time"
)

// Extended permission types (13 granular permissions, inspired by 飞牛fnOS ACL).
const (
	PermReadFile       = "read_file"
	PermWriteFile      = "write_file"
	PermDeleteFile     = "delete_file"
	PermCreateDir      = "create_dir"
	PermListDir        = "list_dir"
	PermShareExternal  = "share_external"
	PermManageACL      = "manage_acl"
	PermViewMetadata   = "view_metadata"
	PermModifyMetadata = "modify_metadata"
	PermExecuteScript  = "execute_script"
	PermMountRemote    = "mount_remote"
	PermBackup         = "backup"
	PermRestore        = "restore"
)

// Role represents a named collection of permissions.
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	Builtin     bool      `json:"builtin"` // system roles cannot be deleted
	CreatedAt   time.Time `json:"created_at"`
}

// UserRoleAssignment maps users to roles with optional path scope.
type UserRoleAssignment struct {
	UserID    string     `json:"user_id"`
	RoleID    string     `json:"role_id"`
	Path      string     `json:"path"` // empty = global scope
	Recursive bool       `json:"recursive"`
	GrantedBy string     `json:"granted_by"`
	GrantedAt time.Time  `json:"granted_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = never expires
}

// RBACManager extends ACL with Role-Based Access Control.
type RBACManager struct {
	mu          sync.RWMutex
	roles       map[string]*Role
	assignments []*UserRoleAssignment
	aclMgr      *Manager
	auditLog    []RBACAuditEntry
}

// RBACAuditEntry records permission-related actions.
type RBACAuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	UserID    string    `json:"user_id"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail"`
	Result    string    `json:"result"` // "allowed" or "denied"
}

// NewRBACManager creates a new RBAC manager with built-in roles.
func NewRBACManager(aclMgr *Manager) *RBACManager {
	rm := &RBACManager{
		roles:    make(map[string]*Role),
		aclMgr:   aclMgr,
		auditLog: make([]RBACAuditEntry, 0, 100),
	}
	rm.initBuiltinRoles()
	return rm
}

// initBuiltinRoles creates default system roles.
func (rm *RBACManager) initBuiltinRoles() {
	builtinRoles := []Role{
		{
			ID: "owner", Name: "所有者", Description: "完全控制权限",
			Permissions: []string{
				string(PermReadFile), string(PermWriteFile), string(PermDeleteFile), string(PermCreateDir),
				string(PermListDir), string(PermShareExternal), string(PermManageACL), string(PermViewMetadata),
				string(PermModifyMetadata), string(PermExecuteScript), string(PermMountRemote),
				string(PermBackup), string(PermRestore), string(PermRead), string(PermWrite), string(PermExecute),
				string(PermAdmin), string(PermDelete), string(PermShare),
			},
			Builtin: true,
		},
		{
			ID: "editor", Name: "编辑者", Description: "读写和删除文件",
			Permissions: []string{
				string(PermReadFile), string(PermWriteFile), string(PermDeleteFile), string(PermCreateDir),
				string(PermListDir), string(PermViewMetadata), string(PermModifyMetadata), string(PermBackup),
				string(PermRead), string(PermWrite),
			},
			Builtin: true,
		},
		{
			ID: "viewer", Name: "查看者", Description: "只读访问",
			Permissions: []string{
				string(PermReadFile), string(PermListDir), string(PermViewMetadata), string(PermRead),
			},
			Builtin: true,
		},
		{
			ID: "collaborator", Name: "协作者", Description: "读写和分享",
			Permissions: []string{
				string(PermReadFile), string(PermWriteFile), string(PermCreateDir), string(PermListDir),
				string(PermShareExternal), string(PermViewMetadata), string(PermBackup), string(PermRead), string(PermWrite), string(PermShare),
			},
			Builtin: true,
		},
		{
			ID: "admin", Name: "管理员", Description: "系统管理权限",
			Permissions: []string{
				string(PermReadFile), string(PermWriteFile), string(PermDeleteFile), string(PermCreateDir),
				string(PermListDir), string(PermShareExternal), string(PermManageACL), string(PermViewMetadata),
				string(PermModifyMetadata), string(PermExecuteScript), string(PermMountRemote),
				string(PermBackup), string(PermRestore), string(PermRead), string(PermWrite), string(PermExecute),
				string(PermAdmin), string(PermDelete), string(PermShare),
			},
			Builtin: true,
		},
	}
	for i := range builtinRoles {
		builtinRoles[i].CreatedAt = time.Now()
		rm.roles[builtinRoles[i].ID] = &builtinRoles[i]
	}
}

// CreateRole creates a custom role.
func (rm *RBACManager) CreateRole(role Role) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if role.ID == "" {
		return fmt.Errorf("role ID cannot be empty")
	}
	if _, exists := rm.roles[role.ID]; exists {
		return fmt.Errorf("role already exists: %s", role.ID)
	}
	role.Builtin = false
	role.CreatedAt = time.Now()
	rm.roles[role.ID] = &role
	return nil
}

// DeleteRole deletes a custom role (builtin roles cannot be deleted).
func (rm *RBACManager) DeleteRole(roleID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	role, exists := rm.roles[roleID]
	if !exists {
		return fmt.Errorf("role not found: %s", roleID)
	}
	if role.Builtin {
		return fmt.Errorf("cannot delete builtin role: %s", roleID)
	}
	// Remove all assignments for this role
	filtered := make([]*UserRoleAssignment, 0, len(rm.assignments))
	for _, a := range rm.assignments {
		if a.RoleID != roleID {
			filtered = append(filtered, a)
		}
	}
	rm.assignments = filtered
	delete(rm.roles, roleID)
	return nil
}

// AssignRole assigns a role to a user.
func (rm *RBACManager) AssignRole(assignment UserRoleAssignment) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.roles[assignment.RoleID]; !exists {
		return fmt.Errorf("role not found: %s", assignment.RoleID)
	}
	assignment.GrantedAt = time.Now()
	rm.assignments = append(rm.assignments, &assignment)
	return nil
}

// RevokeRole revokes a role assignment.
func (rm *RBACManager) RevokeRole(userID, roleID, path string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	filtered := make([]*UserRoleAssignment, 0, len(rm.assignments))
	found := false
	for _, a := range rm.assignments {
		if a.UserID == userID && a.RoleID == roleID && a.Path == path {
			found = true
			continue
		}
		filtered = append(filtered, a)
	}
	if !found {
		return fmt.Errorf("assignment not found")
	}
	rm.assignments = filtered
	return nil
}

// CheckPermission checks if a user has a specific permission on a resource.
func (rm *RBACManager) CheckPermission(userID, resource, permission string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	now := time.Now()

	// Check RBAC assignments
	for _, assignment := range rm.assignments {
		if assignment.UserID != userID {
			continue
		}
		// Check expiration
		if assignment.ExpiresAt != nil && assignment.ExpiresAt.Before(now) {
			continue
		}
		// Check path scope
		if assignment.Path != "" {
			if assignment.Recursive {
				if len(resource) < len(assignment.Path) || resource[:len(assignment.Path)] != assignment.Path {
					continue
				}
			} else if resource != assignment.Path {
				continue
			}
		}
		// Check role permissions
		role, exists := rm.roles[assignment.RoleID]
		if !exists {
			continue
		}
		for _, p := range role.Permissions {
			if p == permission || p == string(PermAdmin) {
				rm.logAudit("check", userID, resource, permission, "allowed")
				return true
			}
		}
	}

	// Fall back to ACL rules
	if rm.aclMgr != nil {
		resp := rm.aclMgr.CheckAccess(CheckAccessRequest{
			Subject:    userID,
			Path:       resource,
			Permission: Permission(permission),
		})
		if resp.Allowed {
			rm.logAudit("check", userID, resource, permission, "allowed")
			return true
		}
	}

	rm.logAudit("check", userID, resource, permission, "denied")
	return false
}

// GetUserRoles returns all roles assigned to a user.
func (rm *RBACManager) GetUserRoles(userID string) []UserRoleAssignment {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []UserRoleAssignment
	for _, a := range rm.assignments {
		if a.UserID == userID {
			result = append(result, *a)
		}
	}
	return result
}

// ListRoles returns all available roles.
func (rm *RBACManager) ListRoles() []Role {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]Role, 0, len(rm.roles))
	for _, r := range rm.roles {
		result = append(result, *r)
	}
	return result
}

// GetAuditLog returns recent audit entries.
func (rm *RBACManager) GetAuditLog(limit int) []RBACAuditEntry {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if limit <= 0 || limit > len(rm.auditLog) {
		limit = len(rm.auditLog)
	}
	start := len(rm.auditLog) - limit
	result := make([]RBACAuditEntry, limit)
	copy(result, rm.auditLog[start:])
	return result
}

func (rm *RBACManager) logAudit(action, userID, resource, detail, result string) {
	rm.auditLog = append(rm.auditLog, RBACAuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		UserID:    userID,
		Resource:  resource,
		Detail:    detail,
		Result:    result,
	})
	// Keep audit log bounded
	if len(rm.auditLog) > 10000 {
		rm.auditLog = rm.auditLog[len(rm.auditLog)-5000:]
	}
}
