package acl

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages ACLs and handles inheritance
type Manager struct {
	mu      sync.RWMutex
	acls    map[string]*ACL        // path -> ACL
	rules   map[string]*ACLRule    // id -> rule (backward compat)
	auditLog []AuditEntry
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	User        string    `json:"user"`
	Action      string    `json:"action"`
	Path        string    `json:"path"`
	Subject     string    `json:"subject"`
	Permission  string    `json:"permission"`
	Allowed     bool      `json:"allowed"`
	Source      string    `json:"source"`
	Details     string    `json:"details"`
}

// NewManager creates a new ACL manager
func NewManager() *Manager {
	return &Manager{
		acls:  make(map[string]*ACL),
		rules: make(map[string]*ACLRule),
	}
}

// CreateACL creates a new ACL for a path
func (m *Manager) CreateACL(req CreateACLRequest) (*ACL, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := NormalizePath(req.Path)
	
	if _, exists := m.acls[path]; exists {
		return nil, fmt.Errorf("ACL already exists for path: %s", path)
	}

	now := time.Now()
	acl := &ACL{
		Path:               path,
		EntryType:          req.EntryType,
		Owner:              req.Owner,
		Group:              req.Group,
		ACES:               []ACE{},
		InheritEnabled:     req.InheritEnabled,
		InheritPermissions: req.InheritPermissions,
		Protected:          false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	m.acls[path] = acl
	log.Printf("ACL已创建: %s (owner: %s)", path, req.Owner)
	
	m.addAuditEntry("system", "create_acl", path, "", "", true, "system", fmt.Sprintf("ACL created for %s", path))
	
	return acl, nil
}

// GetACL returns the ACL for a path
func (m *Manager) GetACL(path string) (*ACL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path = NormalizePath(path)
	acl, exists := m.acls[path]
	if !exists {
		return nil, fmt.Errorf("ACL not found for path: %s", path)
	}

	return acl, nil
}

// UpdateACL updates an existing ACL
func (m *Manager) UpdateACL(path string, req UpdateACLRequest) (*ACL, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = NormalizePath(path)
	acl, exists := m.acls[path]
	if !exists {
		return nil, fmt.Errorf("ACL not found for path: %s", path)
	}

	if req.Owner != "" {
		acl.Owner = req.Owner
	}
	if req.Group != "" {
		acl.Group = req.Group
	}
	if req.InheritEnabled != nil {
		acl.InheritEnabled = *req.InheritEnabled
	}
	if req.InheritPermissions != nil {
		acl.InheritPermissions = *req.InheritPermissions
	}
	if req.Protected != nil {
		acl.Protected = *req.Protected
	}
	acl.UpdatedAt = time.Now()

	log.Printf("ACL已更新: %s", path)
	m.addAuditEntry("system", "update_acl", path, "", "", true, "system", "ACL updated")
	
	return acl, nil
}

// DeleteACL deletes an ACL
func (m *Manager) DeleteACL(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = NormalizePath(path)
	if _, exists := m.acls[path]; !exists {
		return fmt.Errorf("ACL not found for path: %s", path)
	}

	delete(m.acls, path)
	log.Printf("ACL已删除: %s", path)
	m.addAuditEntry("system", "delete_acl", path, "", "", true, "system", "ACL deleted")
	
	return nil
}

// ListACLs returns all ACLs
func (m *Manager) ListACLs() []*ACL {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ACL, 0, len(m.acls))
	for _, acl := range m.acls {
		result = append(result, acl)
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	
	return result
}

// AddACE adds an Access Control Entry to an ACL
func (m *Manager) AddACE(path string, req AddACERequest) (*ACE, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = NormalizePath(path)
	acl, exists := m.acls[path]
	if !exists {
		return nil, fmt.Errorf("ACL not found for path: %s", path)
	}

	// Validate permissions
	for _, perm := range req.Permissions {
		if err := perm.Validate(); err != nil {
			return nil, err
		}
	}

	// Validate subject type
	if err := req.SubjectType.Validate(); err != nil {
		return nil, err
	}

	// Validate inherit flags
	for _, flag := range req.InheritFlags {
		if err := flag.Validate(); err != nil {
			return nil, err
		}
	}

	// Check for duplicate
	for _, ace := range acl.ACES {
		if ace.Subject == req.Subject && ace.SubjectType == req.SubjectType {
			return nil, fmt.Errorf("ACE already exists for subject %s on path %s", req.Subject, path)
		}
	}

	ace := ACE{
		ID:              uuid.New().String(),
		Subject:         req.Subject,
		SubjectType:     req.SubjectType,
		Permissions:     req.Permissions,
		AccessType:      AccessExplicit,
		Allowed:         req.Allowed,
		AppliesTo:       req.AppliesTo,
		InheritFlags:    req.InheritFlags,
		EffectiveFrom:   "",
	}

	acl.ACES = append(acl.ACES, ace)
	acl.UpdatedAt = time.Now()

	log.Printf("ACE已添加: %s -> %s (%s)", req.Subject, path, req.Permissions)
	m.addAuditEntry("system", "add_ace", path, req.Subject, string(req.Permissions[0]), true, "system", 
		fmt.Sprintf("ACE added for %s with permissions %v", req.Subject, req.Permissions))

	return &ace, nil
}

// UpdateACE updates an existing ACE
func (m *Manager) UpdateACE(path, aceID string, req UpdateACERequest) (*ACE, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = NormalizePath(path)
	acl, exists := m.acls[path]
	if !exists {
		return nil, fmt.Errorf("ACL not found for path: %s", path)
	}

	for i, ace := range acl.ACES {
		if ace.ID == aceID {
			if req.Permissions != nil {
				for _, perm := range req.Permissions {
					if err := perm.Validate(); err != nil {
						return nil, err
					}
				}
				acl.ACES[i].Permissions = req.Permissions
			}
			if req.Allowed != nil {
				acl.ACES[i].Allowed = *req.Allowed
			}
			if req.AppliesTo != "" {
				acl.ACES[i].AppliesTo = req.AppliesTo
			}
			if req.InheritFlags != nil {
				for _, flag := range req.InheritFlags {
					if err := flag.Validate(); err != nil {
						return nil, err
					}
				}
				acl.ACES[i].InheritFlags = req.InheritFlags
			}
			acl.UpdatedAt = time.Now()
			
			log.Printf("ACE已更新: %s", aceID)
			m.addAuditEntry("system", "update_ace", path, ace.Subject, "", true, "system", "ACE updated")
			
			return &acl.ACES[i], nil
		}
	}

	return nil, fmt.Errorf("ACE not found: %s", aceID)
}

// RemoveACE removes an ACE from an ACL
func (m *Manager) RemoveACE(path, aceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = NormalizePath(path)
	acl, exists := m.acls[path]
	if !exists {
		return fmt.Errorf("ACL not found for path: %s", path)
	}

	for i, ace := range acl.ACES {
		if ace.ID == aceID {
			acl.ACES = append(acl.ACES[:i], acl.ACES[i+1:]...)
			acl.UpdatedAt = time.Now()
			
			log.Printf("ACE已删除: %s", aceID)
			m.addAuditEntry("system", "remove_ace", path, ace.Subject, "", true, "system", "ACE removed")
			
			return nil
		}
	}

	return fmt.Errorf("ACE not found: %s", aceID)
}

// CheckAccess checks if a subject has a specific permission on a path
func (m *Manager) CheckAccess(req CheckAccessRequest) *CheckAccessResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path := NormalizePath(req.Path)
	subject := req.Subject
	permission := req.Permission

	// Validate permission
	if err := permission.Validate(); err != nil {
		return &CheckAccessResponse{
			Allowed:    false,
			Subject:    subject,
			Path:       path,
			Permission: permission,
			Reason:     fmt.Sprintf("Invalid permission: %s", err),
		}
	}

	// Collect all applicable ACEs (explicit + inherited)
	aces := m.collectEffectiveACEs(subject, path)

	// Sort by specificity: explicit > inherited
	sort.Slice(aces, func(i, j int) bool {
		if aces[i].AccessType != aces[j].AccessType {
			return aces[i].AccessType == AccessExplicit
		}
		return len(aces[i].EffectiveFrom) < len(aces[j].EffectiveFrom)
	})

	// Check for deny ACEs first (deny takes precedence)
	for _, ace := range aces {
		if !ace.Allowed {
			for _, p := range ace.Permissions {
				if p == permission {
					return &CheckAccessResponse{
						Allowed:    false,
						Subject:    subject,
						Path:       path,
						Permission: permission,
						Reason:     "Explicitly denied",
						Source:     ace.AccessType,
						SourcePath: ace.EffectiveFrom,
					}
				}
			}
		}
	}

	// Check for allow ACEs
	for _, ace := range aces {
		if ace.Allowed {
			for _, p := range ace.Permissions {
				if p == permission {
					return &CheckAccessResponse{
						Allowed:    true,
						Subject:    subject,
						Path:       path,
						Permission: permission,
						Reason:     "Permission granted",
						Source:     ace.AccessType,
						SourcePath: ace.EffectiveFrom,
					}
				}
			}
		}
	}

	// Default deny
	return &CheckAccessResponse{
		Allowed:    false,
		Subject:    subject,
		Path:       path,
		Permission: permission,
		Reason:     "No matching ACE found (default deny)",
	}
}

// GetEffectivePermissions returns all effective permissions for a subject on a path
func (m *Manager) GetEffectivePermissions(subject, path string) *EffectivePermissionsResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path = NormalizePath(path)
	aces := m.collectEffectiveACEs(subject, path)

	permSet := make(map[Permission]bool)
	for _, ace := range aces {
		if ace.Allowed {
			for _, p := range ace.Permissions {
				permSet[p] = true
			}
		}
	}

	permissions := make([]Permission, 0, len(permSet))
	for p := range permSet {
		permissions = append(permissions, p)
	}
	sort.Slice(permissions, func(i, j int) bool {
		return string(permissions[i]) < string(permissions[j])
	})

	return &EffectivePermissionsResponse{
		Subject:       subject,
		Path:          path,
		Permissions:   permissions,
		AccessEntries: aces,
	}
}

// collectEffectiveACEs collects all effective ACEs for a subject on a path
func (m *Manager) collectEffectiveACEs(subject, path string) []ACE {
	var result []ACE
	
	// Start from the path and walk up to root
	currentPath := path
	for {
		acl, exists := m.acls[currentPath]
		if exists {
			// Check direct ACEs
			for _, ace := range acl.ACES {
				if ace.Subject == subject {
					newACE := ace
					if currentPath != path {
						newACE.AccessType = AccessInherited
						newACE.EffectiveFrom = currentPath
					}
					result = append(result, newACE)
				}
			}
			
			// If inheritance is disabled, stop here
			if !acl.InheritEnabled {
				break
			}
		}
		
		// Move to parent
		parent := PathParent(currentPath)
		if parent == "" || parent == currentPath {
			break
		}
		currentPath = parent
	}
	
	return result
}

// PropagateInheritance propagates inherited permissions to child paths
func (m *Manager) PropagateInheritance(parentPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	parentPath = NormalizePath(parentPath)
	parentACL, exists := m.acls[parentPath]
	if !exists {
		return fmt.Errorf("ACL not found for path: %s", parentPath)
	}

	if !parentACL.InheritEnabled {
		return nil // Nothing to propagate
	}

	// Find all child paths
	childPaths := m.findChildPaths(parentPath)

	for _, childPath := range childPaths {
		childACL, exists := m.acls[childPath]
		if !exists {
			// Create child ACL with inheritance
			childACL = &ACL{
				Path:               childPath,
				EntryType:          EntryDirectory, // Assume directory for now
				Owner:              parentACL.Owner,
				Group:              parentACL.Group,
				ACES:               []ACE{},
				InheritEnabled:     true,
				InheritPermissions: true,
				Protected:          false,
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			}
			m.acls[childPath] = childACL
		}

		if childACL.Protected {
			continue // Skip protected ACLs
		}

		// Apply inherited ACEs
		for _, parentACE := range parentACL.ACES {
			if !m.shouldInherit(parentACE, childACL.EntryType) {
				continue
			}

			// Check if ACE already exists for this subject
			aceExists := false
			for _, childACE := range childACL.ACES {
				if childACE.Subject == parentACE.Subject && 
				   childACE.SubjectType == parentACE.SubjectType {
					aceExists = true
					break
				}
			}

			if !aceExists {
				inheritedACE := ACE{
					ID:            uuid.New().String(),
					Subject:       parentACE.Subject,
					SubjectType:   parentACE.SubjectType,
					Permissions:   parentACE.Permissions,
					AccessType:    AccessInherited,
					Allowed:       parentACE.Allowed,
					AppliesTo:     parentACE.AppliesTo,
					InheritFlags:  parentACE.InheritFlags,
					EffectiveFrom: parentPath,
				}
				childACL.ACES = append(childACL.ACES, inheritedACE)
			}
		}
		childACL.UpdatedAt = time.Now()
	}

	return nil
}

// shouldInherit checks if an ACE should be inherited to a child entry type
func (m *Manager) shouldInherit(ace ACE, childType EntryType) bool {
	// Check inherit flags
	for _, flag := range ace.InheritFlags {
		switch flag {
		case InheritNone:
			return false
		case InheritContainer:
			if childType != EntryDirectory {
				return false
			}
		case InheritObject:
			if childType != EntryFile && childType != EntrySymlink {
				return false
			}
		case InheritFull:
			return true
		case InheritSelective:
			// Selective inheritance - check AppliesTo
			if ace.AppliesTo != "" && ace.AppliesTo != childType {
				return false
			}
			return true
		}
	}

	// Default: inherit to all
	return true
}

// findChildPaths finds all direct child paths in the ACL store
func (m *Manager) findChildPaths(parentPath string) []string {
	var children []string
	parentPath = strings.TrimSuffix(parentPath, "/")
	
	for path := range m.acls {
		if strings.HasPrefix(path, parentPath+"/") {
			// Check if it's a direct child (no intermediate /)
			relative := strings.TrimPrefix(path, parentPath+"/")
			if !strings.Contains(relative, "/") {
				children = append(children, path)
			}
		}
	}
	
	return children
}

// SetOwner sets the owner of an ACL
func (m *Manager) SetOwner(path, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = NormalizePath(path)
	acl, exists := m.acls[path]
	if !exists {
		return fmt.Errorf("ACL not found for path: %s", path)
	}

	acl.Owner = owner
	acl.UpdatedAt = time.Now()
	
	log.Printf("所有者已设置: %s -> %s", path, owner)
	m.addAuditEntry("system", "set_owner", path, owner, "", true, "system", fmt.Sprintf("Owner set to %s", owner))
	
	return nil
}

// SetGroup sets the group of an ACL
func (m *Manager) SetGroup(path, group string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = NormalizePath(path)
	acl, exists := m.acls[path]
	if !exists {
		return fmt.Errorf("ACL not found for path: %s", path)
	}

	acl.Group = group
	acl.UpdatedAt = time.Now()
	
	log.Printf("组已设置: %s -> %s", path, group)
	m.addAuditEntry("system", "set_group", path, group, "", true, "system", fmt.Sprintf("Group set to %s", group))
	
	return nil
}

// GetAuditLog returns the audit log
func (m *Manager) GetAuditLog(limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}
	
	// Return most recent entries
	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}
	
	return m.auditLog[start:]
}

// addAuditEntry adds an entry to the audit log
func (m *Manager) addAuditEntry(user, action, path, subject, permission string, allowed bool, source, details string) {
	entry := AuditEntry{
		Timestamp:  time.Now(),
		User:       user,
		Action:     action,
		Path:       path,
		Subject:    subject,
		Permission: permission,
		Allowed:    allowed,
		Source:     source,
		Details:    details,
	}
	m.auditLog = append(m.auditLog, entry)
}

// AddRule adds an ACL rule (backward compatibility)
func (m *Manager) AddRule(rule ACLRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule.Enabled = true
	m.rules[rule.ID] = &rule
	log.Printf("ACL规则已添加: %s -> %s (%s)", rule.Subject, rule.Path, rule.Permissions)
}

// RemoveRule removes an ACL rule (backward compatibility)
func (m *Manager) RemoveRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, id)
}

// UpdateRule updates an existing ACL rule (backward compatibility)
func (m *Manager) UpdateRule(rule ACLRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[rule.ID]; !ok {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}
	m.rules[rule.ID] = &rule
	return nil
}

// ListRules returns all ACL rules (backward compatibility)
func (m *Manager) ListRules() []ACLRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ACLRule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, *r)
	}
	return result
}

// GetPermissionGroups returns predefined permission groups
func GetPermissionGroups() []PermissionGroup {
	return []PermissionGroup{
		{
			Name:        "ReadOnly",
			Permissions: ReadOnly,
		},
		{
			Name:        "ReadWrite",
			Permissions: ReadWrite,
		},
		{
			Name:        "Modify",
			Permissions: Modify,
		},
		{
			Name:        "FullControl",
			Permissions: FullControl,
		},
	}
}
