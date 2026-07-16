package users

import (
	"context"
	"fmt"
)

// Health verifies identity store is available and at least one admin account exists.
func (m *Manager) Health(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("users manager is nil")
	}
	_ = ctx
	admins := m.GetUsersByRole(RoleAdmin)
	if len(admins) == 0 {
		return fmt.Errorf("no admin user configured")
	}
	return nil
}

// MustChangePassword reports whether user must rotate the bootstrap password.
func (m *Manager) MustChangePassword(username string) bool {
	u, err := m.GetUser(username)
	if err != nil || u == nil {
		return false
	}
	return u.MustChangePassword
}
