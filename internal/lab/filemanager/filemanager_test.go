package filemanager

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func testBrowser(t *testing.T) (*Browser, string) {
	t.Helper()
	tmpDir := t.TempDir()
	b := NewBrowser(tmpDir, zap.NewNop())
	return b, tmpDir
}

func TestNewBrowser(t *testing.T) {
	b, _ := testBrowser(t)
	if b == nil {
		t.Fatal("NewBrowser returned nil")
	}
}

func TestListDirectory(t *testing.T) {
	b, root := testBrowser(t)
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0644)
	os.Mkdir(filepath.Join(root, "subdir"), 0755)

	listing, err := b.ListDirectory(root, false)
	if err != nil {
		t.Fatalf("ListDirectory failed: %v", err)
	}
	if listing == nil {
		t.Fatal("listing is nil")
	}
}

func TestListDirectoryShowHidden(t *testing.T) {
	b, root := testBrowser(t)
	os.WriteFile(filepath.Join(root, ".hidden"), []byte("secret"), 0644)
	os.WriteFile(filepath.Join(root, "visible.txt"), []byte("show"), 0644)

	listing, err := b.ListDirectory(root, true)
	if err != nil {
		t.Fatalf("ListDirectory(hidden) failed: %v", err)
	}
	if listing == nil {
		t.Fatal("listing is nil")
	}
}

func TestGetFileNode(t *testing.T) {
	b, root := testBrowser(t)
	testFile := filepath.Join(root, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	node, err := b.GetFileNode(testFile)
	if err != nil {
		t.Fatalf("GetFileNode failed: %v", err)
	}
	if node == nil {
		t.Fatal("node is nil")
	}
}

func TestGetFileAttributes(t *testing.T) {
	b, root := testBrowser(t)
	testFile := filepath.Join(root, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	attrs, err := b.GetFileAttributes(testFile)
	if err != nil {
		t.Fatalf("GetFileAttributes failed: %v", err)
	}
	if attrs == nil {
		t.Fatal("attrs is nil")
	}
}

func TestListDirectoryNotExist(t *testing.T) {
	b, _ := testBrowser(t)
	_, err := b.ListDirectory("/nonexistent/path", false)
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestOperationsDelete(t *testing.T) {
	tmpDir := t.TempDir()
	ops := NewOperations(tmpDir, "/tmp/test-ops", zap.NewNop())
	testFile := filepath.Join(tmpDir, "delete_me.txt")
	os.WriteFile(testFile, []byte("bye"), 0644)

	op, err := ops.Delete([]string{testFile}, "test-user")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if op == nil {
		t.Fatal("operation is nil")
	}
}

func TestOperationsRename(t *testing.T) {
	tmpDir := t.TempDir()
	ops := NewOperations(tmpDir, "/tmp/test-ops", zap.NewNop())
	oldFile := filepath.Join(tmpDir, "old.txt")
	os.WriteFile(oldFile, []byte("data"), 0644)

	op, err := ops.Rename(oldFile, "new.txt", "test-user")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if op == nil {
		t.Fatal("operation is nil")
	}
}

func TestOperationsCopy(t *testing.T) {
	tmpDir := t.TempDir()
	ops := NewOperations(tmpDir, "/tmp/test-ops", zap.NewNop())
	srcFile := filepath.Join(tmpDir, "src.txt")
	os.WriteFile(srcFile, []byte("copy me"), 0644)

	op, err := ops.Copy([]string{srcFile}, tmpDir, false, "test-user")
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if op == nil {
		t.Fatal("operation is nil")
	}
}
