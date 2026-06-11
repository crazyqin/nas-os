package acl

import (
	"log"
	"sync"
)

// ACLSystem is the main entry point for the ACL system
type ACLSystem struct {
	Manager  *Manager
	Handlers *Handlers
}

// NewACLSystem creates a new ACL system
func NewACLSystem() *ACLSystem {
	manager := NewManager()
	handlers := NewHandlers(manager)
	
	log.Println("企业级ACL权限系统已初始化")
	
	return &ACLSystem{
		Manager:  manager,
		Handlers: handlers,
	}
}

// Init initializes the ACL system with default configurations
func (s *ACLSystem) Init() {
	log.Println("ACL系统正在初始化...")
	
	// Create default ACL for root path
	s.Manager.CreateACL(CreateACLRequest{
		Path:               "/",
		EntryType:          EntryDirectory,
		Owner:              "admin",
		Group:              "administrators",
		InheritEnabled:     true,
		InheritPermissions: true,
	})
	
	log.Println("ACL系统初始化完成")
}

// CheckAccess is a convenience method to check access
func (s *ACLSystem) CheckAccess(subject, path string, permission Permission) bool {
	result := s.Manager.CheckAccess(CheckAccessRequest{
		Subject:    subject,
		Path:       path,
		Permission: permission,
	})
	return result.Allowed
}

// GrantPermissions grants permissions to a subject on a path
func (s *ACLSystem) GrantPermissions(subject string, subjectType SubjectType, path string, permissions []Permission) error {
	_, err := s.Manager.AddACE(path, AddACERequest{
		Subject:     subject,
		SubjectType: subjectType,
		Permissions: permissions,
		Allowed:     true,
	})
	return err
}

// DenyPermissions denies permissions to a subject on a path
func (s *ACLSystem) DenyPermissions(subject string, subjectType SubjectType, path string, permissions []Permission) error {
	_, err := s.Manager.AddACE(path, AddACERequest{
		Subject:     subject,
		SubjectType: subjectType,
		Permissions: permissions,
		Allowed:     false,
	})
	return err
}

// RevokePermissions revokes permissions from a subject on a path
func (s *ACLSystem) RevokePermissions(subject, path string) error {
	acl, err := s.Manager.GetACL(path)
	if err != nil {
		return err
	}
	
	for _, ace := range acl.ACES {
		if ace.Subject == subject {
			if err := s.Manager.RemoveACE(path, ace.ID); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// SetOwner sets the owner of a path
func (s *ACLSystem) SetOwner(path, owner string) error {
	return s.Manager.SetOwner(path, owner)
}

// SetGroup sets the group of a path
func (s *ACLSystem) SetGroup(path, group string) error {
	return s.Manager.SetGroup(path, group)
}

// EnableInheritance enables inheritance for a path
func (s *ACLSystem) EnableInheritance(path string) error {
	inheritEnabled := true
	_, err := s.Manager.UpdateACL(path, UpdateACLRequest{
		InheritEnabled: &inheritEnabled,
	})
	return err
}

// DisableInheritance disables inheritance for a path
func (s *ACLSystem) DisableInheritance(path string) error {
	inheritEnabled := false
	_, err := s.Manager.UpdateACL(path, UpdateACLRequest{
		InheritEnabled: &inheritEnabled,
	})
	return err
}

// PropagateInheritance propagates inheritance from a parent to children
func (s *ACLSystem) PropagateInheritance(parentPath string) error {
	return s.Manager.PropagateInheritance(parentPath)
}

// GetAuditLog returns the audit log
func (s *ACLSystem) GetAuditLog(limit int) []AuditEntry {
	return s.Manager.GetAuditLog(limit)
}

// GetPermissionGroups returns predefined permission groups
func (s *ACLSystem) GetPermissionGroups() []PermissionGroup {
	return GetPermissionGroups()
}

// GetAllPermissions returns all available permissions
func (s *ACLSystem) GetAllPermissions() []Permission {
	return []Permission{
		PermRead, PermWrite, PermDelete, PermExecute,
		PermCreate, PermRename, PermMove, PermCopy,
		PermViewAttr, PermModifyAttr, PermChangePerm,
		PermTakeOwner, PermTraverse,
	}
}

// Sync is a helper for concurrent access
type Sync struct {
	mu sync.RWMutex
}

// NewSync creates a new Sync helper
func NewSync() *Sync {
	return &Sync{}
}

// Lock acquires a write lock
func (s *Sync) Lock() {
	s.mu.Lock()
}

// Unlock releases a write lock
func (s *Sync) Unlock() {
	s.mu.Unlock()
}

// RLock acquires a read lock
func (s *Sync) RLock() {
	s.mu.RLock()
}

// RUnlock releases a read lock
func (s *Sync) RUnlock() {
	s.mu.RUnlock()
}
