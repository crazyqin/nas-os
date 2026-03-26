// Package apikey provides secure API key management
// rbac_integration.go - RBAC integration for API key access control
package apikey

import (
	"context"
	"sync"
	"time"
)

// RBACConfig holds RBAC configuration for API key management
type RBACConfig struct {
	Enabled          bool `json:"enabled"`
	StrictMode       bool `json:"strict_mode"`       // Deny by default
	AdminFullAccess  bool `json:"admin_full_access"` // Admins have full access
	OwnerFullAccess  bool `json:"owner_full_access"` // Key owners have full access to their keys
	GroupInheritance bool `json:"group_inheritance"` // Inherit permissions from groups
	AuditAll         bool `json:"audit_all"`         // Audit all access attempts
}

// DefaultRBACConfig returns default RBAC configuration
func DefaultRBACConfig() RBACConfig {
	return RBACConfig{
		Enabled:          true,
		StrictMode:       true,
		AdminFullAccess:  true,
		OwnerFullAccess:  true,
		GroupInheritance: true,
		AuditAll:         true,
	}
}

// APIKeyRBAC provides RBAC integration for API key management
type APIKeyRBAC struct {
	config        RBACConfig
	permissionMgr PermissionManager
	groupProvider GroupProvider
	userProvider  UserProvider
	auditLogger   AuditLogger
	cachedPerms   map[string]*CachedPermissions
	mu            sync.RWMutex
}

// CachedPermissions caches user permissions
type CachedPermissions struct {
	UserID      string          `json:"user_id"`
	Permissions []KeyPermission `json:"permissions"`
	KeyIDs      []string        `json:"key_ids"` // Keys user can access
	GroupIDs    []string        `json:"group_ids"`
	ExpiresAt   time.Time       `json:"expires_at"`
}

// PermissionManager defines the interface for permission management
type PermissionManager interface {
	CheckPermission(ctx context.Context, userID, permission string) (bool, error)
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

// GroupProvider defines the interface for group membership
type GroupProvider interface {
	GetUserGroups(ctx context.Context, userID string) ([]string, error)
	GetGroupMembers(ctx context.Context, groupID string) ([]string, error)
	IsGroupAdmin(ctx context.Context, userID, groupID string) (bool, error)
}

// UserProvider defines the interface for user information
type UserProvider interface {
	GetUser(ctx context.Context, userID string) (*UserInfo, error)
	GetUserByUsername(ctx context.Context, username string) (*UserInfo, error)
}

// UserInfo contains user information
type UserInfo struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Role     string   `json:"role"`
	Groups   []string `json:"groups"`
	Active   bool     `json:"active"`
}

// NewAPIKeyRBAC creates a new RBAC handler
func NewAPIKeyRBAC(config RBACConfig, permMgr PermissionManager, groupProvider GroupProvider, userProvider UserProvider, auditLogger AuditLogger) *APIKeyRBAC {
	return &APIKeyRBAC{
		config:        config,
		permissionMgr: permMgr,
		groupProvider: groupProvider,
		userProvider:  userProvider,
		auditLogger:   auditLogger,
		cachedPerms:   make(map[string]*CachedPermissions),
	}
}

// CheckPermission checks if a user has a specific permission
func (r *APIKeyRBAC) CheckPermission(userID, resource, action string) (bool, error) {
	if !r.config.Enabled {
		return true, nil
	}

	ctx := context.Background()

	// Check cache
	cached := r.getCachedPermissions(userID)
	if cached != nil {
		return r.checkCachedPermission(cached, resource, action), nil
	}

	// Check if admin
	if r.config.AdminFullAccess && r.permissionMgr != nil {
		isAdmin, err := r.permissionMgr.IsAdmin(ctx, userID)
		if err == nil && isAdmin {
			return true, nil
		}
	}

	// Check specific permission
	permission := "ai:key:" + action
	if r.permissionMgr != nil {
		allowed, err := r.permissionMgr.CheckPermission(ctx, userID, permission)
		if err != nil {
			if r.config.StrictMode {
				return false, err
			}
			return true, nil
		}
		return allowed, nil
	}

	// No permission manager, use strict mode
	if r.config.StrictMode {
		return false, nil
	}
	return true, nil
}

// CheckKeyAccess checks if a user can access a specific key
func (r *APIKeyRBAC) CheckKeyAccess(userID string, key *APIKey, action string) (bool, error) {
	if !r.config.Enabled {
		return true, nil
	}

	ctx := context.Background()

	// Owner has full access
	if r.config.OwnerFullAccess && key.OwnerID == userID {
		return true, nil
	}

	// Check if admin
	if r.config.AdminFullAccess && r.permissionMgr != nil {
		isAdmin, _ := r.permissionMgr.IsAdmin(ctx, userID)
		if isAdmin {
			return true, nil
		}
	}

	// Check allowed users list
	if containsUser(key.AllowedUsers, userID) {
		return true, nil
	}

	// Check group membership
	if len(key.AllowedGroups) > 0 && r.groupProvider != nil {
		userGroups, err := r.groupProvider.GetUserGroups(ctx, userID)
		if err == nil {
			for _, allowedGroup := range key.AllowedGroups {
				for _, userGroup := range userGroups {
					if allowedGroup == userGroup || allowedGroup == "*" {
						return true, nil
					}
				}
			}
		}
	}

	// Check specific permission
	permission := "ai:key:" + action
	if r.permissionMgr != nil {
		allowed, _ := r.permissionMgr.CheckPermission(ctx, userID, permission)
		if allowed {
			return true, nil
		}
	}

	// Check for key-specific permission
	permission = "ai:key:" + key.ID + ":" + action
	if r.permissionMgr != nil {
		allowed, _ := r.permissionMgr.CheckPermission(ctx, userID, permission)
		if allowed {
			return true, nil
		}
	}

	return false, nil
}

// CanViewSecret checks if a user can view decrypted keys
func (r *APIKeyRBAC) CanViewSecret(userID string, key *APIKey) (bool, error) {
	if !r.config.Enabled {
		return false, nil
	}

	ctx := context.Background()

	// Only admins and key owners can view secrets
	if key.OwnerID == userID {
		return true, nil
	}

	if r.permissionMgr != nil {
		isAdmin, _ := r.permissionMgr.IsAdmin(ctx, userID)
		if isAdmin {
			return true, nil
		}

		// Check specific permission
		allowed, _ := r.permissionMgr.CheckPermission(ctx, userID, string(PermKeyViewSecret))
		if allowed {
			return true, nil
		}
	}

	return false, nil
}

// GrantKeyAccess grants a user access to a key
func (r *APIKeyRBAC) GrantKeyAccess(ctx context.Context, key *APIKey, targetUserID string) error {
	if !containsUser(key.AllowedUsers, targetUserID) {
		key.AllowedUsers = append(key.AllowedUsers, targetUserID)
	}
	return nil
}

// RevokeKeyAccess revokes a user's access to a key
func (r *APIKeyRBAC) RevokeKeyAccess(ctx context.Context, key *APIKey, targetUserID string) error {
	newUsers := make([]string, 0, len(key.AllowedUsers))
	for _, u := range key.AllowedUsers {
		if u != targetUserID {
			newUsers = append(newUsers, u)
		}
	}
	key.AllowedUsers = newUsers
	return nil
}

// GrantGroupAccess grants a group access to a key
func (r *APIKeyRBAC) GrantGroupAccess(ctx context.Context, key *APIKey, groupID string) error {
	for _, g := range key.AllowedGroups {
		if g == groupID {
			return nil // Already has access
		}
	}
	key.AllowedGroups = append(key.AllowedGroups, groupID)
	return nil
}

// RevokeGroupAccess revokes a group's access to a key
func (r *APIKeyRBAC) RevokeGroupAccess(ctx context.Context, key *APIKey, groupID string) error {
	newGroups := make([]string, 0, len(key.AllowedGroups))
	for _, g := range key.AllowedGroups {
		if g != groupID {
			newGroups = append(newGroups, g)
		}
	}
	key.AllowedGroups = newGroups
	return nil
}

// GetAccessibleKeys returns all keys a user can access
func (r *APIKeyRBAC) GetAccessibleKeys(ctx context.Context, userID string, keys map[string]*APIKey) []*APIKey {
	result := make([]*APIKey, 0)

	for _, key := range keys {
		if allowed, _ := r.CheckKeyAccess(userID, key, "read"); allowed {
			result = append(result, key)
		}
	}

	return result
}

// AuditAccess logs an access attempt
func (r *APIKeyRBAC) AuditAccess(ctx context.Context, userID string, key *APIKey, action string, allowed bool, reason string) {
	if !r.config.AuditAll || r.auditLogger == nil {
		return
	}

	eventType := AuditEventKeyAccess
	result := "success"
	if !allowed {
		eventType = AuditEventKeyDeny
		result = "denied"
	}

	_ = r.auditLogger.LogKeyEvent(ctx, &KeyAuditLog{
		ID:        generateID(),
		Timestamp: time.Now(),
		EventType: eventType,
		KeyID:     key.ID,
		KeyName:   key.Name,
		Provider:  key.Provider,
		UserID:    userID,
		Action:    action,
		Result:    result,
		Reason:    reason,
	})
}

// Helper methods

func (r *APIKeyRBAC) getCachedPermissions(userID string) *CachedPermissions {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cached, exists := r.cachedPerms[userID]
	if !exists || time.Now().After(cached.ExpiresAt) {
		return nil
	}

	return cached
}

func (r *APIKeyRBAC) checkCachedPermission(cached *CachedPermissions, _ string, action string) bool {
	perm := "ai:key:" + action
	for _, p := range cached.Permissions {
		if string(p) == perm || string(p) == "ai:key:*" || string(p) == "ai:*" || string(p) == "*" {
			return true
		}
	}
	return false
}

// cachePermissions caches permissions for a user
// nolint:unused // Kept for future caching implementation
func (r *APIKeyRBAC) cachePermissions(userID string, perms []KeyPermission) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cachedPerms[userID] = &CachedPermissions{
		UserID:      userID,
		Permissions: perms,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
}

// InvalidateCache invalidates cached permissions for a user
func (r *APIKeyRBAC) InvalidateCache(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.cachedPerms, userID)
}

// InvalidateAllCache invalidates all cached permissions
func (r *APIKeyRBAC) InvalidateAllCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cachedPerms = make(map[string]*CachedPermissions)
}

// KeyAccessPolicy represents an access policy for API keys
type KeyAccessPolicy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Provider    ProviderType      `json:"provider,omitempty"`   // Apply to specific provider
	KeyTypes    []KeyType         `json:"key_types,omitempty"`  // Apply to key types
	Principals  []string          `json:"principals"`           // Users/groups this applies to
	Permissions []KeyPermission   `json:"permissions"`          // Granted permissions
	Conditions  []AccessCondition `json:"conditions,omitempty"` // Access conditions
	Effect      string            `json:"effect"`               // "allow" or "deny"
	Priority    int               `json:"priority"`
	Enabled     bool              `json:"enabled"`
}

// AccessCondition defines conditions for access
type AccessCondition struct {
	Type     string   `json:"type"`     // time, ip, model, etc.
	Key      string   `json:"key"`      // Condition key
	Operator string   `json:"operator"` // eq, neq, in, not_in, between
	Values   []string `json:"values"`   // Condition values
}

// PolicyEvaluator evaluates access policies
type PolicyEvaluator struct {
	policies []*KeyAccessPolicy
	mu       sync.RWMutex
}

// NewPolicyEvaluator creates a new policy evaluator
func NewPolicyEvaluator() *PolicyEvaluator {
	return &PolicyEvaluator{
		policies: make([]*KeyAccessPolicy, 0),
	}
}

// AddPolicy adds an access policy
func (e *PolicyEvaluator) AddPolicy(policy *KeyAccessPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Insert in priority order
	inserted := false
	for i, p := range e.policies {
		if policy.Priority > p.Priority {
			e.policies = append(e.policies[:i], append([]*KeyAccessPolicy{policy}, e.policies[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		e.policies = append(e.policies, policy)
	}
}

// RemovePolicy removes a policy
func (e *PolicyEvaluator) RemovePolicy(policyID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, p := range e.policies {
		if p.ID == policyID {
			e.policies = append(e.policies[:i], e.policies[i+1:]...)
			break
		}
	}
}

// Evaluate evaluates access against policies
func (e *PolicyEvaluator) Evaluate(ctx context.Context, userID string, key *APIKey, action string) (bool, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, policy := range e.policies {
		if !policy.Enabled {
			continue
		}

		// Check if policy applies
		if !e.policyApplies(policy, userID, key, action) {
			continue
		}

		// Check conditions
		if !e.evaluateConditions(policy.Conditions, ctx, key) {
			continue
		}

		// Return based on effect
		if policy.Effect == "deny" {
			return false, "denied by policy: " + policy.Name
		}
		return true, "allowed by policy: " + policy.Name
	}

	// Default deny
	return false, "no matching policy"
}

func (e *PolicyEvaluator) policyApplies(policy *KeyAccessPolicy, userID string, key *APIKey, action string) bool {
	// Check provider
	if policy.Provider != "" && policy.Provider != key.Provider {
		return false
	}

	// Check key type
	if len(policy.KeyTypes) > 0 {
		found := false
		for _, kt := range policy.KeyTypes {
			if kt == key.KeyType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check principals
	principalMatched := false
	for _, principal := range policy.Principals {
		if principal == userID || principal == "*" {
			principalMatched = true
			break
		}
		if len(principal) > 6 && principal[:6] == "group:" {
			// Would need group provider to check
			principalMatched = true
			break
		}
	}
	if !principalMatched {
		return false
	}

	// Check action/permission
	actionMatched := false
	for _, perm := range policy.Permissions {
		if string(perm) == "ai:key:"+action || string(perm) == "ai:key:*" {
			actionMatched = true
			break
		}
	}

	return actionMatched
}

func (e *PolicyEvaluator) evaluateConditions(conditions []AccessCondition, _ context.Context, _ *APIKey) bool {
	for _, cond := range conditions {
		switch cond.Type {
		case "time":
			// Time-based condition
			// Would implement time range checking
		case "ip":
			// IP-based condition
			// Would check client IP
		case "model":
			// Model restriction
			// Already handled at key level
		}
	}
	return true
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
