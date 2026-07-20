package storage

import (
	"errors"
	"testing"
)

func TestValidateDeleteConfirmation_RejectsMissingConfirm(t *testing.T) {
	err := ValidateDeleteConfirmation("tank", DeleteVolumeOptions{
		ConfirmName: "",
		AllowWipe:   true,
		Force:       true,
	})
	if !errors.Is(err, ErrDeleteNotConfirmed) {
		t.Fatalf("want ErrDeleteNotConfirmed, got %v", err)
	}
}

func TestValidateDeleteConfirmation_RejectsMismatchedName(t *testing.T) {
	err := ValidateDeleteConfirmation("tank", DeleteVolumeOptions{
		ConfirmName: "other",
		AllowWipe:   true,
	})
	if !errors.Is(err, ErrDeleteNotConfirmed) {
		t.Fatalf("want ErrDeleteNotConfirmed, got %v", err)
	}
}

func TestValidateDeleteConfirmation_SoftDeleteWithoutWipeOK(t *testing.T) {
	// allow_wipe=false is valid: soft detach, no wipefs
	err := ValidateDeleteConfirmation("tank", DeleteVolumeOptions{
		ConfirmName: "tank",
		AllowWipe:   false,
		Force:       true,
	})
	if err != nil {
		t.Fatalf("soft delete confirm should pass, got %v", err)
	}
}

func TestValidateDeleteConfirmation_AcceptsExplicitDangerPath(t *testing.T) {
	err := ValidateDeleteConfirmation("tank", DeleteVolumeOptions{
		ConfirmName: "tank",
		AllowWipe:   true,
		Force:       true,
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestDeleteVolumeConfirmed_RejectsBeforeTouchingManager(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{}}
	err := m.DeleteVolumeConfirmed("missing-vol", DeleteVolumeOptions{
		ConfirmName: "",
		AllowWipe:   true,
	})
	if !errors.Is(err, ErrDeleteNotConfirmed) {
		t.Fatalf("want not confirmed, got %v", err)
	}

	// Confirmed soft-delete but volume does not exist → manager error.
	err = m.DeleteVolumeConfirmed("missing-vol", DeleteVolumeOptions{
		ConfirmName: "missing-vol",
		AllowWipe:   false,
	})
	if err == nil {
		t.Fatal("expected volume-not-found style error")
	}
	if errors.Is(err, ErrDeleteNotConfirmed) {
		t.Fatalf("should have passed gate and hit manager: %v", err)
	}
}

func TestDetachVolume_NoWipeKeepsOtherVolumes(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{
		"tank": {Name: "tank", MountPoint: ""},
		"data": {Name: "data"},
	}}
	if err := m.DetachVolume("tank", false); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.volumes["tank"]; ok {
		t.Fatal("tank should be detached")
	}
	if _, ok := m.volumes["data"]; !ok {
		t.Fatal("data should remain")
	}
}
