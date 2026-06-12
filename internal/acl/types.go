package acl

import (
	"fmt"
	"strings"
	"time"
)

// Permission constants for 13 fine-grained permissions
const (
	PermRead         Permission = "read"          // 读取
	PermWrite        Permission = "write"         // 写入
	PermDelete       Permission = "delete"        // 删除
	PermExecute      Permission = "execute"       // 执行
	PermCreate       Permission = "create"        // 创建
	PermRename       Permission = "rename"        // 重命名
	PermMove         Permission = "move"          // 移动
	PermCopy         Permission = "copy"          // 复制
	PermViewAttr     Permission = "view_attr"     // 查看属性
	PermModifyAttr   Permission = "modify_attr"   // 修改属性
	PermChangePerm   Permission = "change_perm"   // 更改权限
	PermTakeOwner    Permission = "take_owner"    // 获取所有权
	PermTraverse     Permission = "traverse"      // 遍历文件夹
	PermAdmin        Permission = "admin"         // 管理员权限
	PermShare        Permission = "share"         // 分享权限
)

// Permission represents a single permission type
type Permission string

// PermissionGroup groups related permissions
type PermissionGroup struct {
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
}

// Standard permission groups for convenience
var (
	// ReadOnly - minimal read access
	ReadOnly = []Permission{PermRead, PermViewAttr, PermTraverse}
	
	// ReadWrite - standard read/write access
	ReadWrite = []Permission{PermRead, PermWrite, PermCreate, PermRename, PermCopy, PermViewAttr, PermTraverse}
	
	// FullControl - all permissions
	FullControl = []Permission{
		PermRead, PermWrite, PermDelete, PermExecute,
		PermCreate, PermRename, PermMove, PermCopy,
		PermViewAttr, PermModifyAttr, PermChangePerm,
		PermTakeOwner, PermTraverse,
	}
	
	// Modify - can modify content but not change permissions
	Modify = []Permission{
		PermRead, PermWrite, PermDelete, PermCreate,
		PermRename, PermMove, PermCopy,
		PermViewAttr, PermTraverse,
	}
)

// SubjectType represents the type of subject (user or group)
type SubjectType string

const (
	SubjectUser  SubjectType = "user"
	SubjectGroup SubjectType = "group"
)

// AccessType represents how access was granted
type AccessType string

const (
	AccessExplicit    AccessType = "explicit"    // Directly assigned to this path
	AccessInherited   AccessType = "inherited"   // Inherited from parent
	AccessDefault     AccessType = "default"     // Default permission
)

// InheritanceType controls how permissions propagate
type InheritanceType string

const (
	InheritFull        InheritanceType = "full"         // Inherit all permissions to children
	InheritSelective   InheritanceType = "selective"    // Inherit only specified permissions
	InheritNone        InheritanceType = "none"         // No inheritance (break inheritance chain)
	InheritContainer   InheritanceType = "container"    // Inherit to containers only (folders)
	InheritObject      InheritanceType = "object"       // Inherit to objects only (files)
)

// EntryType represents a filesystem entry
type EntryType string

const (
	EntryFile      EntryType = "file"
	EntryDirectory EntryType = "directory"
	EntrySymlink   EntryType = "symlink"
)

// ACE represents a single Access Control Entry
type ACE struct {
	ID              string          `json:"id"`
	Subject         string          `json:"subject"`          // User or group name
	SubjectType     SubjectType     `json:"subject_type"`     // "user" or "group"
	Permissions     []Permission    `json:"permissions"`      // List of granted permissions
	AccessType      AccessType      `json:"access_type"`      // explicit, inherited, default
	Allowed         bool            `json:"allowed"`          // true=allow, false=deny
	AppliesTo       EntryType       `json:"applies_to"`       // file, directory, symlink, or empty for all
	InheritFlags    []InheritanceType `json:"inherit_flags"`  // How this ACE propagates
	EffectiveFrom   string          `json:"effective_from"`   // Path this ACE is inherited from (empty if explicit)
}

// ACL represents an Access Control List for a path
type ACL struct {
	Path                string    `json:"path"`
	EntryType           EntryType `json:"entry_type"`
	Owner               string    `json:"owner"`
	Group               string    `json:"group"`
	ACES                []ACE     `json:"aces"`
	InheritEnabled      bool      `json:"inherit_enabled"`      // Whether to inherit from parent
	InheritPermissions  bool      `json:"inherit_permissions"`  // Whether to inherit permissions
	Protected           bool      `json:"protected"`            // Protected from inheritance changes
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ACLRule is kept for backward compatibility
type ACLRule struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Subject     string   `json:"subject"`
	SubjectType string   `json:"subject_type"`
	Permissions []string `json:"permissions"`
	Recursive   bool     `json:"recursive"`
	Priority    int      `json:"priority"`
	Enabled     bool     `json:"enabled"`
}

// CheckAccessRequest represents a request to check access
type CheckAccessRequest struct {
	Subject    string     `json:"subject" binding:"required"`
	Path       string     `json:"path" binding:"required"`
	Permission Permission `json:"permission" binding:"required"`
}

// CheckAccessResponse represents the result of an access check
type CheckAccessResponse struct {
	Allowed     bool       `json:"allowed"`
	Subject     string     `json:"subject"`
	Path        string     `json:"path"`
	Permission  Permission `json:"permission"`
	Reason      string     `json:"reason"`
	Source      AccessType `json:"source"`      // Where the permission came from
	SourcePath  string     `json:"source_path"` // Path that granted the permission
}

// EffectivePermissionsResponse represents effective permissions for a subject
type EffectivePermissionsResponse struct {
	Subject         string       `json:"subject"`
	Path            string       `json:"path"`
	Permissions     []Permission `json:"permissions"`
	AccessEntries   []ACE        `json:"access_entries"`
}

// CreateACLRequest represents a request to create an ACL
type CreateACLRequest struct {
	Path               string          `json:"path" binding:"required"`
	EntryType          EntryType       `json:"entry_type" binding:"required"`
	Owner              string          `json:"owner" binding:"required"`
	Group              string          `json:"group"`
	InheritEnabled     bool            `json:"inherit_enabled"`
	InheritPermissions bool            `json:"inherit_permissions"`
}

// UpdateACLRequest represents a request to update an ACL
type UpdateACLRequest struct {
	Owner              string `json:"owner"`
	Group              string `json:"group"`
	InheritEnabled     *bool  `json:"inherit_enabled"`
	InheritPermissions *bool  `json:"inherit_permissions"`
	Protected          *bool  `json:"protected"`
}

// AddACERequest represents a request to add an ACE
type AddACERequest struct {
	Subject      string            `json:"subject" binding:"required"`
	SubjectType  SubjectType       `json:"subject_type" binding:"required"`
	Permissions  []Permission      `json:"permissions" binding:"required,min=1"`
	Allowed      bool              `json:"allowed"`
	AppliesTo    EntryType         `json:"applies_to"`
	InheritFlags []InheritanceType `json:"inherit_flags"`
}

// UpdateACERequest represents a request to update an ACE
type UpdateACERequest struct {
	Permissions  []Permission      `json:"permissions"`
	Allowed      *bool             `json:"allowed"`
	AppliesTo    EntryType         `json:"applies_to"`
	InheritFlags []InheritanceType `json:"inherit_flags"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate validates a Permission
func (p Permission) Validate() error {
	validPerms := map[Permission]bool{
		PermRead: true, PermWrite: true, PermDelete: true, PermExecute: true,
		PermCreate: true, PermRename: true, PermMove: true, PermCopy: true,
		PermViewAttr: true, PermModifyAttr: true, PermChangePerm: true,
		PermTakeOwner: true, PermTraverse: true, PermAdmin: true, PermShare: true,
	}
	if !validPerms[p] {
		return fmt.Errorf("invalid permission: %s", p)
	}
	return nil
}

// Validate validates a SubjectType
func (st SubjectType) Validate() error {
	if st != SubjectUser && st != SubjectGroup {
		return fmt.Errorf("invalid subject type: %s", st)
	}
	return nil
}

// Validate validates an InheritanceType
func (it InheritanceType) Validate() error {
	validTypes := map[InheritanceType]bool{
		InheritFull: true, InheritSelective: true, InheritNone: true,
		InheritContainer: true, InheritObject: true,
	}
	if !validTypes[it] {
		return fmt.Errorf("invalid inheritance type: %s", it)
	}
	return nil
}

// PathParent returns the parent path
func PathParent(path string) string {
	// Clean and trim path
	path = strings.TrimSuffix(path, "/")
	if path == "" || path == "/" {
		return ""
	}
	
	// Find last separator
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

// IsPathAncestor checks if ancestor is an ancestor of descendant
func IsPathAncestor(ancestor, descendant string) bool {
	ancestor = strings.TrimSuffix(ancestor, "/")
	descendant = strings.TrimSuffix(descendant, "/")
	
	if ancestor == "" || ancestor == "/" {
		return true // Root is ancestor of everything
	}
	return strings.HasPrefix(descendant, ancestor+"/") || ancestor == descendant
}

// NormalizePath normalizes a file path
func NormalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Remove trailing slash except for root
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}
