package storage

import (
	"errors"
	"fmt"
	"strings"
)

// ErrDeleteNotConfirmed is returned when a destructive volume delete lacks
// an explicit confirmation matching the volume name.
var ErrDeleteNotConfirmed = errors.New("volume delete not confirmed: confirm_name must exactly match the volume name")

// ErrWipeNotAllowed is returned when a device wipe is requested without
// the deliberate allow_wipe danger flag.
var ErrWipeNotAllowed = errors.New("device wipe not allowed: set allow_wipe=true to acknowledge irreversible wipefs")

// DeleteVolumeOptions gates destructive volume deletion (device wipe).
// Callers (HTTP handlers) must populate ConfirmName from the request body;
// force alone is not sufficient.
//
// Soft delete (default when allow_wipe=false): unmount + unregister only —
// devices keep filesystem signatures for recovery.
// Hard wipe (allow_wipe=true): wipefs on member devices after unmount.
type DeleteVolumeOptions struct {
	// ConfirmName must exactly equal the volume name being deleted.
	ConfirmName string `json:"confirm_name"`
	// AllowWipe must be true to run wipefs on member devices.
	// When false, only DetachVolume runs (no wipefs).
	AllowWipe bool `json:"allow_wipe"`
	// Force allows deleting volumes that still have subvolumes / ignore unmount errors.
	Force bool `json:"force"`
}

// ValidateDeleteConfirmation checks the confirmation payload before any
// destructive storage operation. Pure function — no disk I/O.
// allow_wipe is optional: false means soft-delete (detach); true means wipe.
func ValidateDeleteConfirmation(volumeName string, opts DeleteVolumeOptions) error {
	name := strings.TrimSpace(volumeName)
	if name == "" {
		return fmt.Errorf("volume name is required")
	}
	confirm := strings.TrimSpace(opts.ConfirmName)
	if confirm == "" || confirm != name {
		return ErrDeleteNotConfirmed
	}
	return nil
}

// DeleteVolumeConfirmed validates confirmation then either soft-detaches
// (no wipefs) or fully deletes with wipe when AllowWipe is set.
func (m *Manager) DeleteVolumeConfirmed(name string, opts DeleteVolumeOptions) error {
	if err := ValidateDeleteConfirmation(name, opts); err != nil {
		return err
	}
	if opts.AllowWipe {
		return m.DeleteVolume(name, opts.Force)
	}
	return m.DetachVolume(name, opts.Force)
}

// DetachVolume unmounts and unregisters a volume without wiping devices.
// Member disks retain their btrfs signatures for recovery.
func (m *Manager) DetachVolume(name string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol, exists := m.volumes[name]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", name)
	}

	if len(vol.Subvolumes) > 0 && !force {
		return fmt.Errorf("卷包含 %d 个子卷，请先删除子卷或使用强制删除", len(vol.Subvolumes))
	}

	if vol.MountPoint != "" && m.client != nil {
		if err := m.client.Unmount(vol.MountPoint); err != nil {
			if !force {
				return fmt.Errorf("卸载失败: %w", err)
			}
		}
	}

	// Soft delete: do NOT wipefs; only drop manager bookkeeping.
	delete(m.volumes, name)
	return nil
}
