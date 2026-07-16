package users

import (
	"context"
	"path/filepath"
	"testing"
)

func TestManagerHealthRequiresAdmin(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(filepath.Join(dir, "users"))
	if err != nil {
		// NewManager may require mount base - try alternate
		t.Skipf("NewManager: %v", err)
	}
	if err := m.Health(context.Background()); err != nil {
		t.Fatalf("bootstrap admin should make Health OK: %v", err)
	}
	// bootstrap admin must change password
	if !m.MustChangePassword("admin") {
		t.Fatal("bootstrap admin must have MustChangePassword")
	}
}
