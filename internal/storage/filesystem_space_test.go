package storage

import (
	"path/filepath"
	"testing"
)

func TestFilesystemSpace(t *testing.T) {
	total, available, err := filesystemSpace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if total <= 0 || available < 0 || available > total {
		t.Fatalf("invalid filesystem capacity: total=%d available=%d", total, available)
	}
}

func TestFilesystemSpaceMissingPath(t *testing.T) {
	if _, _, err := filesystemSpace(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing path must fail")
	}
}
