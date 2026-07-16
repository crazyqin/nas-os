package network

import (
	"context"
	"fmt"
)

// Health verifies network manager is initialized and config is accessible.
func (m *Manager) Health(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("network manager is nil")
	}
	_ = ctx
	// ListDDNS exercises in-memory config without requiring live NIC state.
	_ = m.ListDDNS()
	return nil
}
