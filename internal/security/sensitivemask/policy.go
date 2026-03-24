package sensitivemask

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PolicyManager manages data protection policies
type PolicyManager struct {
	policies map[string]*Policy
	active   string // Active policy ID
	mu       sync.RWMutex
	storage  string // Storage path for policies
}

// NewPolicyManager creates a new policy manager
func NewPolicyManager(storagePath string) *PolicyManager {
	pm := &PolicyManager{
		policies: make(map[string]*Policy),
		storage:  storagePath,
	}
	pm.loadPolicies()
	return pm
}

// CreatePolicy creates a new policy
func (pm *PolicyManager) CreatePolicy(name, description string, detector DetectorConfig, masker MaskerConfig, actions PolicyActions) (*Policy, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	id := uuid.New().String()
	policy := &Policy{
		ID:          id,
		Name:        name,
		Description: description,
		Detector:    detector,
		Masker:      masker,
		Actions:     actions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pm.policies[id] = policy
	pm.savePolicies()
	return policy, nil
}

// GetPolicy retrieves a policy by ID
func (pm *PolicyManager) GetPolicy(id string) (*Policy, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	policy, ok := pm.policies[id]
	return policy, ok
}

// UpdatePolicy updates an existing policy
func (pm *PolicyManager) UpdatePolicy(id string, updates PolicyUpdate) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	policy, ok := pm.policies[id]
	if !ok {
		return fmt.Errorf("policy not found: %s", id)
	}

	if updates.Name != nil {
		policy.Name = *updates.Name
	}
	if updates.Description != nil {
		policy.Description = *updates.Description
	}
	if updates.Detector != nil {
		policy.Detector = *updates.Detector
	}
	if updates.Masker != nil {
		policy.Masker = *updates.Masker
	}
	if updates.Actions != nil {
		policy.Actions = *updates.Actions
	}
	policy.UpdatedAt = time.Now()

	pm.savePolicies()
	return nil
}

// DeletePolicy deletes a policy
func (pm *PolicyManager) DeletePolicy(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}

	if pm.active == id {
		return fmt.Errorf("cannot delete active policy")
	}

	delete(pm.policies, id)
	pm.savePolicies()
	return nil
}

// SetActivePolicy sets the active policy
func (pm *PolicyManager) SetActivePolicy(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}

	pm.active = id
	pm.savePolicies()
	return nil
}

// GetActivePolicy returns the currently active policy
func (pm *PolicyManager) GetActivePolicy() (*Policy, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.active == "" {
		return nil, fmt.Errorf("no active policy set")
	}

	policy, ok := pm.policies[pm.active]
	if !ok {
		return nil, fmt.Errorf("active policy not found: %s", pm.active)
	}

	return policy, nil
}

// ListPolicies lists all policies
func (pm *PolicyManager) ListPolicies() []*Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policies := make([]*Policy, 0, len(pm.policies))
	for _, p := range pm.policies {
		policies = append(policies, p)
	}
	return policies
}

// loadPolicies loads policies from storage
func (pm *PolicyManager) loadPolicies() {
	if pm.storage == "" {
		return
	}

	data, err := os.ReadFile(filepath.Join(pm.storage, "policies.json"))
	if err != nil {
		return
	}

	var state struct {
		Policies map[string]*Policy `json:"policies"`
		Active   string             `json:"active"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}

	pm.policies = state.Policies
	pm.active = state.Active
}

// savePolicies saves policies to storage
func (pm *PolicyManager) savePolicies() {
	if pm.storage == "" {
		return
	}

	state := struct {
		Policies map[string]*Policy `json:"policies"`
		Active   string             `json:"active"`
	}{
		Policies: pm.policies,
		Active:   pm.active,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}

	if err := os.MkdirAll(pm.storage, 0755); err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(pm.storage, "policies.json"), data, 0644); err != nil {
		return
	}
}

// PolicyUpdate represents updates to a policy
type PolicyUpdate struct {
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Detector    *DetectorConfig  `json:"detector,omitempty"`
	Masker      *MaskerConfig    `json:"masker,omitempty"`
	Actions     *PolicyActions   `json:"actions,omitempty"`
}

// AuditLogger handles audit logging for sensitive data operations
type AuditLogger struct {
	logs     []AuditLog
	maxLogs  int
	mu       sync.RWMutex
	storage  string
	enabled  bool
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(storagePath string, maxLogs int) *AuditLogger {
	al := &AuditLogger{
		logs:    make([]AuditLog, 0),
		maxLogs: maxLogs,
		storage: storagePath,
		enabled: true,
	}
	al.loadLogs()
	return al
}

// Log records an audit log entry
func (al *AuditLogger) Log(ctx context.Context, entry AuditLog) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if !al.enabled {
		return nil
	}

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	al.logs = append(al.logs, entry)

	// Trim old logs if exceeding max
	if len(al.logs) > al.maxLogs {
		al.logs = al.logs[len(al.logs)-al.maxLogs:]
	}

	al.saveLogs()
	return nil
}

// GetLogs retrieves logs with optional filtering
func (al *AuditLogger) GetLogs(filter AuditFilter) []AuditLog {
	al.mu.RLock()
	defer al.mu.RUnlock()

	result := make([]AuditLog, 0)
	for _, log := range al.logs {
		if filter.StartTime != nil && log.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && log.Timestamp.After(*filter.EndTime) {
			continue
		}
		if filter.UserID != "" && log.UserID != filter.UserID {
			continue
		}
		if filter.ServiceName != "" && log.ServiceName != filter.ServiceName {
			continue
		}
		if filter.PolicyID != "" && log.PolicyID != filter.PolicyID {
			continue
		}
		if filter.OnlyBlocked && !log.Blocked {
			continue
		}
		result = append(result, log)
	}

	// Sort by timestamp descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Timestamp.Before(result[j].Timestamp) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// Clear clears all logs
func (al *AuditLogger) Clear() {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.logs = make([]AuditLog, 0)
	al.saveLogs()
}

// Enable enables or disables logging
func (al *AuditLogger) Enable(enabled bool) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.enabled = enabled
}

// loadLogs loads logs from storage
func (al *AuditLogger) loadLogs() {
	if al.storage == "" {
		return
	}

	data, err := os.ReadFile(filepath.Join(al.storage, "audit_logs.json"))
	if err != nil {
		return
	}

	if err := json.Unmarshal(data, &al.logs); err != nil {
		return
	}
}

// saveLogs saves logs to storage
func (al *AuditLogger) saveLogs() {
	if al.storage == "" {
		return
	}

	data, err := json.MarshalIndent(al.logs, "", "  ")
	if err != nil {
		return
	}

	if err := os.MkdirAll(al.storage, 0755); err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(al.storage, "audit_logs.json"), data, 0644); err != nil {
		return
	}
}

// AuditFilter represents filters for audit log queries
type AuditFilter struct {
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	UserID      string     `json:"user_id,omitempty"`
	ServiceName string     `json:"service_name,omitempty"`
	PolicyID    string     `json:"policy_id,omitempty"`
	OnlyBlocked bool       `json:"only_blocked,omitempty"`
	Limit       int        `json:"limit,omitempty"`
}