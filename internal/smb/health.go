package smb

import (
	"context"
	"fmt"
)

// Health verifies the SMB manager can list shares (config store available).
func (m *Manager) Health(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("smb manager is nil")
	}
	_ = ctx
	if _, err := m.ListShares(); err != nil {
		return fmt.Errorf("smb list shares: %w", err)
	}
	return nil
}
