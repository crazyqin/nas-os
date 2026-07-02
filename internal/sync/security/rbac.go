package security

import (
	"fmt"
	"os"
	"strings"
)

// UserRole represents user privilege level.
type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

// Permission represents a sync-related permission.
type Permission string

const (
	PermSyncCreate Permission = "sync:create"
	PermSyncDelete Permission = "sync:delete"
	PermSyncRun    Permission = "sync:run"
	PermSyncView   Permission = "sync:view"
	PermSyncAdmin  Permission = "sync:admin"
)

// rolePermissions maps roles to their allowed permissions.
var rolePermissions = map[UserRole][]Permission{
	RoleAdmin:    {PermSyncCreate, PermSyncDelete, PermSyncRun, PermSyncView, PermSyncAdmin},
	RoleOperator: {PermSyncCreate, PermSyncRun, PermSyncView},
	RoleViewer:   {PermSyncView},
}

// CheckPermission verifies a user has the required permission.
func CheckPermission(role UserRole, perm Permission) error {
	perms, ok := rolePermissions[role]
	if !ok {
		return fmt.Errorf("unknown role: %s", role)
	}
	for _, p := range perms {
		if p == perm {
			return nil
		}
	}
	return fmt.Errorf("permission denied: role %s lacks %s", role, perm)
}

// AllowedSyncBasePaths returns directories a user can sync from/to.
func AllowedSyncBasePaths(role UserRole) []string {
	homeDir, _ := os.UserHomeDir()
	sharedBase := "/mnt"

	switch role {
	case RoleAdmin:
		return []string{sharedBase, "/media", homeDir}
	case RoleOperator:
		return []string{sharedBase, homeDir}
	case RoleViewer:
		return []string{homeDir}
	default:
		return []string{}
	}
}

// ValidateSyncPaths checks both source and destination paths.
func ValidateSyncPaths(src, dst string, role UserRole) error {
	allowedBases := AllowedSyncBasePaths(role)
	if len(allowedBases) == 0 {
		return fmt.Errorf("no allowed sync paths for role: %s", role)
	}

	// Check blocked paths
	if IsBlockedPath(src) {
		return fmt.Errorf("source path is blocked: %s", src)
	}
	if IsBlockedPath(dst) {
		return fmt.Errorf("destination path is blocked: %s", dst)
	}

	// Check path is within allowed bases
	if !isWithinAny(src, allowedBases) {
		return fmt.Errorf("source path outside allowed directories: %s", src)
	}
	if !isWithinAny(dst, allowedBases) {
		return fmt.Errorf("destination path outside allowed directories: %s", dst)
	}

	// Check traversal
	for _, base := range allowedBases {
		if err := CheckPathTraversal(src, base); err == nil {
			break
		}
	}

	return nil
}

func isWithinAny(path string, bases []string) bool {
	for _, base := range bases {
		if strings.HasPrefix(path, base) {
			return true
		}
	}
	return false
}
