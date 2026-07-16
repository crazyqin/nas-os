package nfs

import (
	"context"
	"fmt"
)

// Health verifies the NFS manager can list exports.
func (m *Manager) Health(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("nfs manager is nil")
	}
	_ = ctx
	if _, err := m.ListExports(); err != nil {
		return fmt.Errorf("nfs list exports: %w", err)
	}
	return nil
}
