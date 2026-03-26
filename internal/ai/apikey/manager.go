// Package apikey provides secure API key management
// manager.go - Core key management operations
package apikey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages API keys with encryption, RBAC, and auditing
type Manager struct {
	keys       map[string]*APIKey
	keyManager *KeyManager
	auditLog   AuditLogger
	rbac       RBACChecker
	config     ManagerConfig
	mu         sync.RWMutex
}

// ManagerConfig holds manager configuration
type ManagerConfig struct {
	StorePath        string  `json:"store_path"`
	AutoExpire       bool    `json:"auto_expire"`
	DefaultLimit     int64   `json:"default_limit"`
	DefaultCostLimit float64 `json:"default_cost_limit"`
	EnableRateLimit  bool    `json:"enable_rate_limit"`
	StrictValidation bool    `json:"strict_validation"`
}

// DefaultManagerConfig returns default configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		AutoExpire:       true,
		DefaultLimit:     1000000, // 1M requests/month
		DefaultCostLimit: 1000.0,  // $1000/month
		EnableRateLimit:  true,
		StrictValidation: true,
	}
}

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	LogKeyEvent(ctx context.Context, event *KeyAuditLog) error
	QueryLogs(ctx context.Context, filter AuditFilter) ([]*KeyAuditLog, error)
}

// AuditFilter defines filter for audit queries
type AuditFilter struct {
	KeyID     *string         `json:"key_id,omitempty"`
	EventType *AuditEventType `json:"event_type,omitempty"`
	UserID    *string         `json:"user_id,omitempty"`
	Provider  *ProviderType   `json:"provider,omitempty"`
	StartTime *time.Time      `json:"start_time,omitempty"`
	EndTime   *time.Time      `json:"end_time,omitempty"`
	Result    *string         `json:"result,omitempty"`
	Limit     int             `json:"limit,omitempty"`
}

// RBACChecker defines the interface for permission checking
type RBACChecker interface {
	CheckPermission(userID, resource, action string) (bool, error)
	CheckKeyAccess(userID string, key *APIKey, action string) (bool, error)
}

// NewManager creates a new API key manager
func NewManager(config ManagerConfig, keyManager *KeyManager, auditLog AuditLogger, rbac RBACChecker) (*Manager, error) {
	m := &Manager{
		keys:       make(map[string]*APIKey),
		keyManager: keyManager,
		auditLog:   auditLog,
		rbac:       rbac,
		config:     config,
	}

	// Load existing keys if store path is set
	if config.StorePath != "" {
		if err := m.load(); err != nil {
			return nil, fmt.Errorf("failed to load keys: %w", err)
		}
	}

	return m, nil
}

// CreateKey creates a new API key with encryption
func (m *Manager) CreateKey(ctx context.Context, req *KeyCreateRequest, userID, username string) (*APIKey, error) {
	// Validate provider
	providerInfo, ok := DefaultProviders[req.Provider]
	if !ok && m.config.StrictValidation {
		return nil, errors.New(ErrProviderNotFound)
	}

	// Validate key format if pattern exists
	if providerInfo.KeyPattern != "" && m.config.StrictValidation {
		matched, err := regexp.MatchString(providerInfo.KeyPattern, req.APIKey)
		if err != nil || !matched {
			return nil, errors.New(ErrInvalidKeyFormat)
		}
	}

	// Check permission
	if m.rbac != nil {
		allowed, err := m.rbac.CheckPermission(userID, string(PermKeyCreate), "create")
		if err != nil {
			return nil, fmt.Errorf("permission check failed: %w", err)
		}
		if !allowed {
			return nil, errors.New(ErrAccessDenied)
		}
	}

	// Encrypt the key
	encryptedKey, err := m.keyManager.Encrypt(req.APIKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrEncryptionFailed, err)
	}

	// Create key record
	now := time.Now()
	key := &APIKey{
		ID:            uuid.New().String(),
		Provider:      req.Provider,
		KeyType:       req.KeyType,
		Name:          req.Name,
		Description:   req.Description,
		EncryptedKey:  encryptedKey,
		KeyHash:       m.keyManager.HashKey(req.APIKey),
		KeyPreview:    GetKeyPreview(req.APIKey),
		Status:        KeyStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     req.ExpiresAt,
		UsageLimit:    req.UsageLimit,
		CostLimit:     req.CostLimit,
		AllowedUsers:  req.AllowedUsers,
		AllowedGroups: req.AllowedGroups,
		AllowedModels: req.AllowedModels,
		RateLimit:     req.RateLimit,
		RotationDays:  req.RotationDays,
		OwnerID:       userID,
		OwnerType:     "user",
		Endpoint:      req.Endpoint,
		Headers:       req.Headers,
		Metadata:      req.Metadata,
		CreatedBy:     userID,
		Version:       1,
	}

	// Set defaults
	if key.UsageLimit == 0 {
		key.UsageLimit = m.config.DefaultLimit
	}
	if key.CostLimit == 0 {
		key.CostLimit = m.config.DefaultCostLimit
	}
	if key.KeyType == "" {
		key.KeyType = KeyTypeAPIKey
	}
	if key.Endpoint == "" && providerInfo.APIEndpoint != "" {
		key.Endpoint = providerInfo.APIEndpoint
	}

	// Store key
	m.mu.Lock()
	m.keys[key.ID] = key
	m.mu.Unlock()

	// Audit log
	if m.auditLog != nil {
		_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
			ID:        uuid.New().String(),
			Timestamp: now,
			EventType: AuditEventKeyCreate,
			KeyID:     key.ID,
			KeyName:   key.Name,
			Provider:  key.Provider,
			UserID:    userID,
			Username:  username,
			Action:    "create",
			Result:    "success",
			NewValue: map[string]any{
				"provider":    key.Provider,
				"name":        key.Name,
				"key_type":    key.KeyType,
				"usage_limit": key.UsageLimit,
				"cost_limit":  key.CostLimit,
			},
		})
	}

	// Save to disk
	if m.config.StorePath != "" {
		_ = m.save()
	}

	return key, nil
}

// GetKey retrieves a key by ID (without decrypting)
func (m *Manager) GetKey(ctx context.Context, keyID, userID string) (*APIKey, error) {
	m.mu.RLock()
	key, exists := m.keys[keyID]
	m.mu.RUnlock()

	if !exists {
		return nil, errors.New(ErrKeyNotFound)
	}

	// Check access
	if m.rbac != nil {
		allowed, err := m.rbac.CheckKeyAccess(userID, key, "read")
		if err != nil || !allowed {
			return nil, errors.New(ErrAccessDenied)
		}
	}

	// Audit log
	if m.auditLog != nil {
		_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			EventType: AuditEventKeyAccess,
			KeyID:     key.ID,
			KeyName:   key.Name,
			Provider:  key.Provider,
			UserID:    userID,
			Action:    "read",
			Result:    "success",
		})
	}

	return key, nil
}

// UseKey retrieves and validates a key for use
func (m *Manager) UseKey(ctx context.Context, req *KeyUseRequest) (*KeyUseResult, error) {
	// Find key
	var key *APIKey

	m.mu.RLock()
	if req.KeyID != "" {
		key = m.keys[req.KeyID]
	} else if req.Provider != "" {
		// Find key by provider that user can access
		for _, k := range m.keys {
			if k.Provider == req.Provider && k.Status == KeyStatusActive {
				// Check if user can use this key
				if m.rbac != nil {
					if allowed, _ := m.rbac.CheckKeyAccess(req.UserID, k, "use"); allowed {
						key = k
						break
					}
				} else if k.OwnerID == req.UserID {
					key = k
					break
				}
			}
		}
	}
	m.mu.RUnlock()

	if key == nil {
		return nil, errors.New(ErrKeyNotFound)
	}

	result := &KeyUseResult{
		KeyID:    key.ID,
		Provider: key.Provider,
		Allowed:  false,
	}

	// Check status
	switch key.Status {
	case KeyStatusDisabled:
		result.Reason = ErrKeyDisabled
		return result, errors.New(ErrKeyDisabled)
	case KeyStatusExpired:
		result.Reason = ErrKeyExpired
		return result, errors.New(ErrKeyExpired)
	case KeyStatusRevoked:
		result.Reason = ErrKeyRevoked
		return result, errors.New(ErrKeyRevoked)
	}

	// Check expiration
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		m.mu.Lock()
		key.Status = KeyStatusExpired
		m.mu.Unlock()

		result.Reason = ErrKeyExpired

		// Audit log
		if m.auditLog != nil {
			_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
				ID:        uuid.New().String(),
				Timestamp: time.Now(),
				EventType: AuditEventKeyExpired,
				KeyID:     key.ID,
				Provider:  key.Provider,
				UserID:    req.UserID,
				Action:    "use",
				Result:    "denied",
				Reason:    "key expired",
			})
		}

		return result, errors.New(ErrKeyExpired)
	}

	// Check access permission
	if m.rbac != nil {
		allowed, err := m.rbac.CheckKeyAccess(req.UserID, key, "use")
		if err != nil || !allowed {
			result.Reason = ErrAccessDenied

			// Audit log
			if m.auditLog != nil {
				_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
					ID:        uuid.New().String(),
					Timestamp: time.Now(),
					EventType: AuditEventKeyDeny,
					KeyID:     key.ID,
					Provider:  key.Provider,
					UserID:    req.UserID,
					Action:    "use",
					Result:    "denied",
					Reason:    ErrAccessDenied,
				})
			}

			return result, errors.New(ErrAccessDenied)
		}
	} else if key.OwnerID != req.UserID && !containsUser(key.AllowedUsers, req.UserID) {
		result.Reason = ErrAccessDenied
		return result, errors.New(ErrAccessDenied)
	}

	// Check model restriction
	if len(key.AllowedModels) > 0 && req.Model != "" {
		if !containsModel(key.AllowedModels, req.Model) {
			result.Reason = fmt.Sprintf("model %s not allowed for this key", req.Model)
			return result, errors.New(result.Reason)
		}
	}

	// Check usage limit
	if key.UsageLimit > 0 && key.CurrentUsage >= key.UsageLimit {
		result.Reason = ErrKeyLimitReached

		// Audit log
		if m.auditLog != nil {
			_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
				ID:        uuid.New().String(),
				Timestamp: time.Now(),
				EventType: AuditEventLimitReached,
				KeyID:     key.ID,
				Provider:  key.Provider,
				UserID:    req.UserID,
				Action:    "use",
				Result:    "denied",
				Reason:    "usage limit reached",
			})
		}

		return result, errors.New(ErrKeyLimitReached)
	}

	// Check cost limit
	if key.CostLimit > 0 && key.CurrentCost >= key.CostLimit {
		result.Reason = ErrKeyCostLimit
		return result, errors.New(ErrKeyCostLimit)
	}

	// Decrypt key
	decryptedKey, err := m.keyManager.Decrypt(key.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrDecryptionFailed, err)
	}

	// Update usage stats
	m.mu.Lock()
	key.UsageCount++
	key.CurrentUsage++
	now := time.Now()
	key.LastUsedAt = &now
	m.mu.Unlock()

	// Build result
	result.DecryptedKey = decryptedKey
	result.Endpoint = key.Endpoint
	result.Headers = key.Headers
	result.Allowed = true

	// Audit log
	if m.auditLog != nil {
		_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
			ID:        uuid.New().String(),
			Timestamp: now,
			EventType: AuditEventKeyUse,
			KeyID:     key.ID,
			Provider:  key.Provider,
			UserID:    req.UserID,
			Action:    "use",
			Result:    "success",
			Resource:  req.Model,
		})
	}

	// Save
	if m.config.StorePath != "" {
		_ = m.save()
	}

	return result, nil
}

// UpdateKey updates key metadata
func (m *Manager) UpdateKey(ctx context.Context, keyID, userID, username string, req *KeyUpdateRequest) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return nil, errors.New(ErrKeyNotFound)
	}

	// Check permission
	if m.rbac != nil {
		allowed, err := m.rbac.CheckKeyAccess(userID, key, "update")
		if err != nil || !allowed {
			return nil, errors.New(ErrAccessDenied)
		}
	}

	// Track old values for audit
	oldValues := make(map[string]any)

	// Apply updates
	if req.Name != nil {
		oldValues["name"] = key.Name
		key.Name = *req.Name
	}
	if req.Description != nil {
		oldValues["description"] = key.Description
		key.Description = *req.Description
	}
	if req.Status != nil {
		oldValues["status"] = key.Status
		key.Status = *req.Status
	}
	if req.UsageLimit != nil {
		oldValues["usage_limit"] = key.UsageLimit
		key.UsageLimit = *req.UsageLimit
	}
	if req.CostLimit != nil {
		oldValues["cost_limit"] = key.CostLimit
		key.CostLimit = *req.CostLimit
	}
	if req.AllowedUsers != nil {
		oldValues["allowed_users"] = key.AllowedUsers
		key.AllowedUsers = *req.AllowedUsers
	}
	if req.AllowedGroups != nil {
		oldValues["allowed_groups"] = key.AllowedGroups
		key.AllowedGroups = *req.AllowedGroups
	}
	if req.AllowedModels != nil {
		oldValues["allowed_models"] = key.AllowedModels
		key.AllowedModels = *req.AllowedModels
	}
	if req.RateLimit != nil {
		oldValues["rate_limit"] = key.RateLimit
		key.RateLimit = *req.RateLimit
	}
	if req.RotationDays != nil {
		oldValues["rotation_days"] = key.RotationDays
		key.RotationDays = *req.RotationDays
	}
	if req.Headers != nil {
		oldValues["headers"] = key.Headers
		key.Headers = *req.Headers
	}
	if req.Metadata != nil {
		oldValues["metadata"] = key.Metadata
		key.Metadata = *req.Metadata
	}

	key.UpdatedAt = time.Now()

	// Audit log
	if m.auditLog != nil {
		_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
			ID:        uuid.New().String(),
			Timestamp: key.UpdatedAt,
			EventType: AuditEventKeyUpdate,
			KeyID:     key.ID,
			KeyName:   key.Name,
			Provider:  key.Provider,
			UserID:    userID,
			Username:  username,
			Action:    "update",
			Result:    "success",
			OldValue:  oldValues,
			NewValue:  req,
		})
	}

	// Save
	if m.config.StorePath != "" {
		_ = m.save()
	}

	return key, nil
}

// DeleteKey deletes a key
func (m *Manager) DeleteKey(ctx context.Context, keyID, userID, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return errors.New(ErrKeyNotFound)
	}

	// Check permission
	if m.rbac != nil {
		allowed, err := m.rbac.CheckKeyAccess(userID, key, "delete")
		if err != nil || !allowed {
			return errors.New(ErrAccessDenied)
		}
	}

	// Delete key
	delete(m.keys, keyID)

	// Audit log
	if m.auditLog != nil {
		_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			EventType: AuditEventKeyDelete,
			KeyID:     key.ID,
			KeyName:   key.Name,
			Provider:  key.Provider,
			UserID:    userID,
			Username:  username,
			Action:    "delete",
			Result:    "success",
		})
	}

	// Save
	if m.config.StorePath != "" {
		_ = m.save()
	}

	return nil
}

// RotateKey rotates a key
func (m *Manager) RotateKey(ctx context.Context, req *KeyRotationRequest, userID, username string) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[req.KeyID]
	if !exists {
		return nil, errors.New(ErrKeyNotFound)
	}

	// Check permission
	if m.rbac != nil {
		allowed, err := m.rbac.CheckKeyAccess(userID, key, "rotate")
		if err != nil || !allowed {
			return nil, errors.New(ErrAccessDenied)
		}
	}

	// Validate new key
	providerInfo, ok := DefaultProviders[key.Provider]
	if ok && providerInfo.KeyPattern != "" && m.config.StrictValidation {
		matched, err := regexp.MatchString(providerInfo.KeyPattern, req.NewAPIKey)
		if err != nil || !matched {
			return nil, errors.New(ErrInvalidKeyFormat)
		}
	}

	// Encrypt new key
	encryptedKey, err := m.keyManager.Encrypt(req.NewAPIKey)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrEncryptionFailed, err)
	}

	oldStatus := key.Status
	oldEncryptedKey := key.EncryptedKey

	// Update key
	key.EncryptedKey = encryptedKey
	key.KeyHash = m.keyManager.HashKey(req.NewAPIKey)
	key.KeyPreview = GetKeyPreview(req.NewAPIKey)
	key.Version++
	key.UpdatedAt = time.Now()
	now := time.Now()
	key.LastRotatedAt = &now

	if req.RevokeOld {
		key.Status = KeyStatusActive
	}

	// Audit log
	if m.auditLog != nil {
		_ = m.auditLog.LogKeyEvent(ctx, &KeyAuditLog{
			ID:        uuid.New().String(),
			Timestamp: now,
			EventType: AuditEventKeyRotate,
			KeyID:     key.ID,
			KeyName:   key.Name,
			Provider:  key.Provider,
			UserID:    userID,
			Username:  username,
			Action:    "rotate",
			Result:    "success",
			OldValue:  map[string]any{"status": oldStatus, "version": key.Version - 1},
			NewValue:  map[string]any{"status": key.Status, "version": key.Version},
		})
	}

	// If not revoking old immediately, store old key for grace period
	if !req.RevokeOld {
		// Store old encrypted key in metadata for rollback
		if key.Metadata == nil {
			key.Metadata = make(map[string]any)
		}
		key.Metadata["previous_encrypted_key"] = oldEncryptedKey
		key.Metadata["previous_key_version"] = key.Version - 1
		key.Status = KeyStatusRotating
	}

	// Save
	if m.config.StorePath != "" {
		_ = m.save()
	}

	return key, nil
}

// ListKeys lists keys with optional filtering
func (m *Manager) ListKeys(ctx context.Context, userID string, filter *KeyFilter) (*KeyListResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check permission
	if m.rbac != nil {
		allowed, _ := m.rbac.CheckPermission(userID, string(PermKeyRead), "read")
		if !allowed {
			// Only return user's own keys
			filter.OwnerID = &userID
		}
	}

	// Filter keys
	var keys []*APIKey
	for _, key := range m.keys {
		if filter.Matches(key) {
			// Redact sensitive fields
			keyCopy := *key
			keyCopy.EncryptedKey = nil
			keys = append(keys, &keyCopy)
		}
	}

	// Sort by created date (newest first)
	sortKeysByCreated(keys, true)

	// Convert to []APIKey for the result
	resultKeys := make([]APIKey, len(keys))
	for i, k := range keys {
		resultKeys[i] = *k
	}

	return &KeyListResult{
		Keys:     resultKeys,
		Total:    len(keys),
		Page:     1,
		PageSize: len(keys),
	}, nil
}

// GetProviders returns available providers
func (m *Manager) GetProviders() map[ProviderType]ProviderInfo {
	return DefaultProviders
}

// load loads keys from the store file
func (m *Manager) load() error {
	if m.config.StorePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.config.StorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing file is OK
		}
		return fmt.Errorf("failed to read key store: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var keys []*APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("failed to unmarshal keys: %w", err)
	}

	m.keys = make(map[string]*APIKey)
	for _, key := range keys {
		m.keys[key.ID] = key
	}

	return nil
}

// save persists keys to the store file
func (m *Manager) save() error {
	if m.config.StorePath == "" {
		return nil
	}

	m.mu.RLock()
	keys := make([]*APIKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, key)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keys: %w", err)
	}

	if err := os.WriteFile(m.config.StorePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write key store: %w", err)
	}

	return nil
}

// Helper functions

func containsUser(users []string, userID string) bool {
	for _, u := range users {
		if u == userID || strings.EqualFold(u, "all") {
			return true
		}
	}
	return false
}

func containsModel(models []string, model string) bool {
	for _, m := range models {
		if m == model || m == "*" {
			return true
		}
		// Support prefix match
		if strings.HasSuffix(m, "*") && strings.HasPrefix(model, strings.TrimSuffix(m, "*")) {
			return true
		}
	}
	return false
}

func sortKeysByCreated(keys []*APIKey, desc bool) {
	// Simple insertion sort
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0; j-- {
			shouldSwap := keys[j].CreatedAt.After(keys[j-1].CreatedAt)
			if !desc {
				shouldSwap = keys[j].CreatedAt.Before(keys[j-1].CreatedAt)
			}
			if shouldSwap {
				keys[j], keys[j-1] = keys[j-1], keys[j]
			} else {
				break
			}
		}
	}
}

// Matches checks if a key matches the filter
func (f *KeyFilter) Matches(key *APIKey) bool {
	if f == nil {
		return true
	}

	if f.Provider != nil && key.Provider != *f.Provider {
		return false
	}
	if f.Status != nil && key.Status != *f.Status {
		return false
	}
	if f.KeyType != nil && key.KeyType != *f.KeyType {
		return false
	}
	if f.OwnerID != nil && key.OwnerID != *f.OwnerID {
		return false
	}
	if f.OwnerType != nil && key.OwnerType != *f.OwnerType {
		return false
	}
	if f.Model != nil && len(key.AllowedModels) > 0 {
		if !containsModel(key.AllowedModels, *f.Model) {
			return false
		}
	}
	if f.SearchQuery != nil && *f.SearchQuery != "" {
		query := strings.ToLower(*f.SearchQuery)
		if !strings.Contains(strings.ToLower(key.Name), query) &&
			!strings.Contains(strings.ToLower(key.Description), query) {
			return false
		}
	}

	return true
}
