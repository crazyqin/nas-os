// Package securityv2 提供安全模块 v2 版本
package securityv2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSafeFileManager(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	if manager.baseDir != filepath.Clean(tmpDir) {
		t.Error("Base directory should be cleaned")
	}

	if len(manager.allowedExt) == 0 {
		t.Error("Should have default allowed extensions")
	}
}

func TestSafeFileManager_SafePath(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "simple filename",
			input:     "test.txt",
			expectErr: false,
		},
		{
			name:      "nested path",
			input:     "subdir/test.txt",
			expectErr: false,
		},
		{
			name:      "path traversal attempt",
			input:     "../../../etc/passwd",
			expectErr: true,
		},
		{
			name:      "absolute path attempt",
			input:     "/etc/passwd",
			expectErr: true,
		},
		{
			name:      "parent directory traversal",
			input:     "../test.txt",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.SafePath(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("SafePath(%s) error = %v, expectErr %v", tt.input, err, tt.expectErr)
			}
		})
	}
}

func TestSafeFileManager_SafeRead(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 测试正常读取
	data, err := manager.SafeRead("test.txt")
	if err != nil {
		t.Fatalf("SafeRead failed: %v", err)
	}

	if string(data) != "test content" {
		t.Errorf("Expected 'test content', got %s", string(data))
	}

	// 测试不允许的扩展名
	execFile := filepath.Join(tmpDir, "test.exe")
	if err := os.WriteFile(execFile, []byte("executable"), 0644); err != nil {
		t.Fatalf("Failed to create exec file: %v", err)
	}

	_, err = manager.SafeRead("test.exe")
	if err == nil {
		t.Error("Should not allow reading .exe files")
	}
}

func TestSafeFileManager_SafeWrite(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	// 测试正常写入
	err := manager.SafeWrite("test.txt", []byte("new content"))
	if err != nil {
		t.Fatalf("SafeWrite failed: %v", err)
	}

	// 验证文件内容
	data, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != "new content" {
		t.Errorf("Expected 'new content', got %s", string(data))
	}

	// 测试不允许的扩展名
	err = manager.SafeWrite("test.exe", []byte("executable"))
	if err == nil {
		t.Error("Should not allow writing .exe files")
	}
}

func TestSafeFileManager_SafeDelete(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "delete.txt")
	if err := os.WriteFile(testFile, []byte("delete me"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 测试正常删除
	if err := manager.SafeDelete("delete.txt"); err != nil {
		t.Fatalf("SafeDelete failed: %v", err)
	}

	// 验证文件已删除
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should be deleted")
	}
}

func TestSafeFileManager_SafeWalk(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	// 创建目录结构
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file2.txt"), []byte("2"), 0644)

	var files []string
	err := manager.SafeWalk(".", func(path string, info os.FileInfo) error {
		if !info.IsDir() {
			files = append(files, filepath.Base(path))
		}
		return nil
	})

	if err != nil {
		t.Fatalf("SafeWalk failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
}

func TestSafeFileManager_ExtensionManagement(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	// 添加新扩展名
	manager.AddAllowedExtension(".newext")

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.newext")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 应该可以读取
	_, err := manager.SafeRead("test.newext")
	if err != nil {
		t.Errorf("Should allow .newext files: %v", err)
	}

	// 移除扩展名
	manager.RemoveAllowedExtension(".newext")

	// 应该不能读取
	_, err = manager.SafeRead("test.newext")
	if err == nil {
		t.Error("Should not allow .newext files after removal")
	}
}

func TestHashCalculator_CalculateFileHash(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "hash_test.txt")
	os.WriteFile(tmpFile, []byte("test content"), 0644)

	calc := &HashCalculator{}
	result, err := calc.CalculateFileHash(tmpFile)
	if err != nil {
		t.Fatalf("CalculateFileHash failed: %v", err)
	}

	if result.Hash == "" {
		t.Error("Hash should not be empty")
	}

	if len(result.Hash) != 64 { // SHA-256 hex length
		t.Errorf("Expected hash length 64, got %d", len(result.Hash))
	}

	if result.Size != 12 {
		t.Errorf("Expected size 12, got %d", result.Size)
	}
}

func TestHashCalculator_VerifyFileIntegrity(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "integrity_test.txt")
	os.WriteFile(tmpFile, []byte("test content"), 0644)

	calc := &HashCalculator{}

	// 计算正确的哈希
	result, _ := calc.CalculateFileHash(tmpFile)

	// 验证正确的哈希
	valid, err := calc.VerifyFileIntegrity(tmpFile, result.Hash)
	if err != nil {
		t.Fatalf("VerifyFileIntegrity failed: %v", err)
	}

	if !valid {
		t.Error("Should verify correct hash")
	}

	// 验证错误的哈希
	valid, _ = calc.VerifyFileIntegrity(tmpFile, "wronghash")
	if valid {
		t.Error("Should not verify wrong hash")
	}
}

func TestSecurityAuditor_AuditPath(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)
	auditor := NewSecurityAuditor(manager)

	// 测试安全路径
	result, err := auditor.AuditPath(nil, "test.txt")
	if err != nil {
		t.Fatalf("AuditPath failed: %v", err)
	}

	if result == nil {
		t.Fatal("AuditPath returned nil result")
	}

	// 测试敏感文件名
	result, _ = auditor.AuditPath(nil, "password.txt")
	if result.Severity != "medium" {
		t.Errorf("Expected medium severity for sensitive filename, got %s", result.Severity)
	}
}
