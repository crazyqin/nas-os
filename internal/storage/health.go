package storage

import (
	"context"
	"fmt"
)

// Health reports whether storage manager can enumerate volumes and none are marked unhealthy.
// Empty volume list is healthy (fresh install). Unhealthy volume status fails the check.
func (m *Manager) Health(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("storage manager is nil")
	}
	_ = ctx
	vols := m.ListVolumes()
	for _, v := range vols {
		if v != nil && !v.Status.Healthy {
			return fmt.Errorf("volume %q is unhealthy", v.Name)
		}
	}
	return nil
}
