package acl

import (
	"fmt"
	"log"
	"sync"
)

// Permission types
const (
	PermRead    = "read"
	PermWrite   = "write"
	PermExecute = "execute"
	PermAdmin   = "admin"
	PermDelete  = "delete"
	PermShare   = "share"
)

// ACLRule represents a fine-grained access control rule.
type ACLRule struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`        // file/folder path
	Subject     string   `json:"subject"`      // user or group name
	SubjectType string   `json:"subject_type"` // "user" or "group"
	Permissions []string `json:"permissions"`  // read, write, execute, delete, share, admin
	Recursive   bool     `json:"recursive"`    // apply to sub-paths
	Priority    int      `json:"priority"`     // higher = evaluated first
	Enabled     bool     `json:"enabled"`
}

// Manager manages ACL rules.
type Manager struct {
	mu    sync.RWMutex
	rules map[string]*ACLRule // id -> rule
}

// NewManager creates a new ACL manager.
func NewManager() *Manager {
	return &Manager{
		rules: make(map[string]*ACLRule),
	}
}

// AddRule adds an ACL rule.
func (m *Manager) AddRule(rule ACLRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule.Enabled = true
	m.rules[rule.ID] = &rule
	log.Printf("ACL规则已添加: %s -> %s (%s)", rule.Subject, rule.Path, rule.Permissions)
}

// RemoveRule removes an ACL rule.
func (m *Manager) RemoveRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, id)
}

// UpdateRule updates an existing ACL rule.
func (m *Manager) UpdateRule(rule ACLRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[rule.ID]; !ok {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}
	m.rules[rule.ID] = &rule
	return nil
}

// ListRules returns all ACL rules.
func (m *Manager) ListRules() []ACLRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ACLRule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, *r)
	}
	return result
}

// CheckAccess checks if a subject has a specific permission on a path.
func (m *Manager) CheckAccess(subject, path, permission string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect matching rules
	type scored struct {
		rule  *ACLRule
		score int
	}
	var matches []scored

	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Subject != subject {
			continue
		}
		// Path matching
		if rule.Recursive {
			if len(path) >= len(rule.Path) && path[:len(rule.Path)] == rule.Path {
				matches = append(matches, scored{rule, rule.Priority + len(rule.Path)})
			}
		} else if path == rule.Path {
			matches = append(matches, scored{rule, rule.Priority + len(rule.Path)})
		}
	}

	if len(matches) == 0 {
		return false // default deny
	}

	// Highest priority wins
	best := matches[0]
	for _, m := range matches[1:] {
		if m.score > best.score {
			best = m
		}
	}

	// Check permission
	for _, p := range best.rule.Permissions {
		if p == permission || p == PermAdmin {
			return true
		}
	}
	return false
}

// GetEffectivePermissions returns all effective permissions for a subject on a path.
func (m *Manager) GetEffectivePermissions(subject, path string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	permSet := make(map[string]bool)
	for _, rule := range m.rules {
		if !rule.Enabled || rule.Subject != subject {
			continue
		}
		match := false
		if rule.Recursive && len(path) >= len(rule.Path) && path[:len(rule.Path)] == rule.Path {
			match = true
		} else if path == rule.Path {
			match = true
		}
		if match {
			for _, p := range rule.Permissions {
				permSet[p] = true
			}
		}
	}

	result := make([]string, 0, len(permSet))
	for p := range permSet {
		result = append(result, p)
	}
	return result
}
