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

	if manager.accessLogger == nil {
		t.Error("Should have access logger initialized")
	}

	if manager.permChecker == nil {
		t.Error("Should have permission checker initialized")
	}

	if manager.sensitiveDetector == nil {
		t.Error("Should have sensitive detector initialized")
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
	if result.Severity != "medium" && result.Severity != "high" {
		t.Errorf("Expected medium or high severity for sensitive filename, got %s", result.Severity)
	}
}

// ========== 新增测试：文件权限检查 ==========

func TestPermissionChecker_CheckFilePermission(t *testing.T) {
	tmpDir := t.TempDir()
	checker := NewPermissionChecker()

	// 测试安全的文件权限
	safeFile := filepath.Join(tmpDir, "safe.txt")
	os.WriteFile(safeFile, []byte("test"), 0644)

	issue, err := checker.CheckFilePermission(safeFile)
	if err != nil {
		t.Fatalf("CheckFilePermission failed: %v", err)
	}
	if issue != nil {
		t.Errorf("Safe file should not have permission issues, got: %s", issue.Message)
	}

	// 测试过于开放的文件权限
	openFile := filepath.Join(tmpDir, "open.txt")
	os.WriteFile(openFile, []byte("test"), 0777)

	issue, err = checker.CheckFilePermission(openFile)
	if err != nil {
		t.Fatalf("CheckFilePermission failed: %v", err)
	}
	if issue == nil {
		t.Error("Overly permissive file should have issues")
	}
	if issue != nil && issue.Severity != "high" {
		t.Errorf("Overly permissive file should have high severity, got %s", issue.Severity)
	}

	// 测试目录权限
	safeDir := filepath.Join(tmpDir, "safedir")
	os.Mkdir(safeDir, 0755)

	issue, err = checker.CheckFilePermission(safeDir)
	if err != nil {
		t.Fatalf("CheckFilePermission for dir failed: %v", err)
	}
	if issue != nil {
		t.Errorf("Safe directory should not have permission issues, got: %s", issue.Message)
	}
}

func TestPermissionChecker_FixPermission(t *testing.T) {
	tmpDir := t.TempDir()
	checker := NewPermissionChecker()

	// 创建权限过于开放的文件
	openFile := filepath.Join(tmpDir, "open.txt")
	os.WriteFile(openFile, []byte("test"), 0777)

	// 修复权限
	err := checker.FixPermission(openFile)
	if err != nil {
		t.Fatalf("FixPermission failed: %v", err)
	}

	// 验证权限已修复
	info, _ := os.Stat(openFile)
	if info.Mode().Perm() != 0644 {
		t.Errorf("Expected 0644, got %o", info.Mode().Perm())
	}
}

// ========== 新增测试：敏感文件检测 ==========

func TestSensitiveFileDetector_Detect(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewSensitiveFileDetector()

	// 测试普通文件名
	normalFile := filepath.Join(tmpDir, "normal.txt")
	os.WriteFile(normalFile, []byte("normal content"), 0644)

	result, err := detector.Detect(normalFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result != nil {
		t.Errorf("Normal file should not be detected as sensitive, got: %+v", result)
	}

	// 测试敏感文件名 - password
	pwFile := filepath.Join(tmpDir, "password.txt")
	os.WriteFile(pwFile, []byte("some password"), 0644)

	result, err = detector.Detect(pwFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result == nil {
		t.Error("Password file should be detected as sensitive")
	}
	if result != nil && result.Severity != "high" {
		t.Errorf("Password file should have high severity, got %s", result.Severity)
	}

	// 测试敏感文件名 - .key
	keyFile := filepath.Join(tmpDir, "private.key")
	os.WriteFile(keyFile, []byte("private key"), 0644)

	result, err = detector.Detect(keyFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result == nil {
		t.Error("Key file should be detected as sensitive")
	}
}

func TestSensitiveFileDetector_DetectContent(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewSensitiveFileDetector()

	// 测试包含敏感内容的文件
	sensitiveContent := `password = "my-secret-password"
api_key = "sk-1234567890"
`
	sensitiveFile := filepath.Join(tmpDir, "config.txt")
	os.WriteFile(sensitiveFile, []byte(sensitiveContent), 0644)

	result, err := detector.Detect(sensitiveFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result == nil {
		t.Fatal("File with sensitive content should be detected")
	}
	if result.Severity != "critical" {
		t.Errorf("File with sensitive content should have critical severity, got %s", result.Severity)
	}
	if len(result.ContentMatch) == 0 {
		t.Error("Should have content matches")
	}
}

func TestSensitiveFileDetector_DetectSSHKey(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewSensitiveFileDetector()

	// 测试 SSH 私钥格式
	sshKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAlwAAAAdzc2gtcn
-----END OPENSSH PRIVATE KEY-----`
	keyFile := filepath.Join(tmpDir, "id_rsa.txt")
	os.WriteFile(keyFile, []byte(sshKey), 0644)

	result, err := detector.Detect(keyFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result == nil {
		t.Fatal("SSH key file should be detected")
	}
	if result.Severity != "high" && result.Severity != "critical" {
		t.Errorf("SSH key should have high or critical severity, got %s", result.Severity)
	}
}

// ========== 新增测试：访问日志 ==========

func TestAccessLogger_Log(t *testing.T) {
	logger := NewAccessLogger("", 100)

	// 记录成功的读取操作
	logger.Log(AccessLog{
		Operation: "read",
		Path:      "/test/file.txt",
		Success:   true,
		FileSize:  100,
	})

	// 记录失败的写入操作
	logger.Log(AccessLog{
		Operation: "write",
		Path:      "/test/secret.key",
		Success:   false,
		Error:     "file extension .key not allowed",
	})

	// 记录带安全风险的删除操作
	logger.Log(AccessLog{
		Operation:    "delete",
		Path:         "/test/password.txt",
		Success:      true,
		SecurityRisk: "删除敏感文件: password",
	})

	logs := logger.GetLogs(0)
	if len(logs) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(logs))
	}

	// 验证时间戳已设置
	for _, log := range logs {
		if log.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	}
}

func TestAccessLogger_GetRecentFailures(t *testing.T) {
	logger := NewAccessLogger("", 100)

	// 添加一些日志
	logger.Log(AccessLog{Operation: "read", Path: "a.txt", Success: true})
	logger.Log(AccessLog{Operation: "read", Path: "b.txt", Success: false, Error: "not found"})
	logger.Log(AccessLog{Operation: "write", Path: "c.txt", Success: true})
	logger.Log(AccessLog{Operation: "read", Path: "d.txt", Success: false, Error: "permission denied"})

	failures := logger.GetRecentFailures(10)
	if len(failures) != 2 {
		t.Errorf("Expected 2 failures, got %d", len(failures))
	}
}

func TestAccessLogger_GetSecurityRisks(t *testing.T) {
	logger := NewAccessLogger("", 100)

	// 添加一些日志
	logger.Log(AccessLog{Operation: "read", Path: "a.txt", Success: true})
	logger.Log(AccessLog{Operation: "read", Path: "password.txt", Success: true, SecurityRisk: "敏感文件"})
	logger.Log(AccessLog{Operation: "write", Path: "c.txt", Success: true})
	logger.Log(AccessLog{Operation: "delete", Path: "secret.key", Success: true, SecurityRisk: "敏感文件删除"})

	risks := logger.GetSecurityRisks()
	if len(risks) != 2 {
		t.Errorf("Expected 2 security risks, got %d", len(risks))
	}
}

func TestAccessLogger_MaxLogs(t *testing.T) {
	logger := NewAccessLogger("", 10)

	// 添加超过限制的日志
	for i := 0; i < 20; i++ {
		logger.Log(AccessLog{
			Operation: "read",
			Path:      "test.txt",
			Success:   true,
		})
	}

	logs := logger.GetLogs(0)
	if len(logs) > 10 {
		t.Errorf("Should limit logs to maxLogs, got %d", len(logs))
	}
}

func TestAccessLogger_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "access.log")
	logger := NewAccessLogger(logFile, 100)

	// 记录日志
	logger.Log(AccessLog{
		Operation: "read",
		Path:      "test.txt",
		Success:   true,
	})

	// 验证文件已创建
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should be created")
	}
}

// ========== 新增测试：增强的 SafeFileManager ==========

func TestSafeFileManager_AccessLogging(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)
	logger := NewAccessLogger("", 100)
	manager.SetAccessLogger(logger)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	// 读取文件
	_, err := manager.SafeRead("test.txt")
	if err != nil {
		t.Fatalf("SafeRead failed: %v", err)
	}

	// 验证日志已记录
	logs := logger.GetLogs(0)
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(logs))
	}

	if logs[0].Operation != "read" {
		t.Errorf("Expected operation 'read', got '%s'", logs[0].Operation)
	}
	if !logs[0].Success {
		t.Error("Log should show success")
	}
	if logs[0].FileSize != 7 {
		t.Errorf("Expected file size 7, got %d", logs[0].FileSize)
	}
	if logs[0].Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestSafeFileManager_WriteWithSecurePermission(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)

	// 写入文件
	err := manager.SafeWrite("test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("SafeWrite failed: %v", err)
	}

	// 验证文件权限
	info, err := os.Stat(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	// 应该是 0644
	if info.Mode().Perm() != 0644 {
		t.Errorf("Expected 0644 permission, got %o", info.Mode().Perm())
	}
}

func TestSafeFileManager_DeleteWithLogging(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)
	logger := NewAccessLogger("", 100)
	manager.SetAccessLogger(logger)

	// 创建并删除文件
	testFile := filepath.Join(tmpDir, "delete.txt")
	os.WriteFile(testFile, []byte("delete me"), 0644)

	err := manager.SafeDelete("delete.txt")
	if err != nil {
		t.Fatalf("SafeDelete failed: %v", err)
	}

	// 验证日志
	logs := logger.GetLogs(0)
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(logs))
	}

	if logs[0].Operation != "delete" {
		t.Errorf("Expected operation 'delete', got '%s'", logs[0].Operation)
	}
}

func TestSafeFileManager_SensitiveFileRead(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)
	logger := NewAccessLogger("", 100)
	manager.SetAccessLogger(logger)

	// 创建密码文件
	pwFile := filepath.Join(tmpDir, "password.txt")
	os.WriteFile(pwFile, []byte("secret"), 0644)
	manager.AddAllowedExtension(".txt")

	// 读取应该成功但记录安全风险
	data, err := manager.SafeRead("password.txt")
	if err != nil {
		t.Fatalf("SafeRead should succeed: %v", err)
	}

	if string(data) != "secret" {
		t.Errorf("Expected 'secret', got '%s'", string(data))
	}

	// 验证安全风险已记录
	logs := logger.GetLogs(0)
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(logs))
	}

	if logs[0].SecurityRisk == "" {
		t.Error("Should have security risk recorded")
	}
}

// ========== 新增测试：增强的安全审计 ==========

func TestSecurityAuditor_AuditDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)
	auditor := NewSecurityAuditor(manager)

	// 创建测试文件结构
	os.WriteFile(filepath.Join(tmpDir, "normal.txt"), []byte("normal"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "password.txt"), []byte("pw"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "open.txt"), []byte("open"), 0777)

	// 审计目录
	results, err := auditor.AuditDirectory(nil, ".")
	if err != nil {
		t.Fatalf("AuditDirectory failed: %v", err)
	}

	// 应该检测到问题
	if len(results) == 0 {
		t.Error("Should detect security issues in directory")
	}

	// 检查是否检测到了敏感文件名和开放权限
	var foundSensitive, foundPerm bool
	for _, r := range results {
		if r.SensitiveFileInfo != nil {
			foundSensitive = true
		}
		if r.PermissionIssue != nil {
			foundPerm = true
		}
	}

	if !foundSensitive {
		t.Error("Should detect sensitive filename")
	}
	if !foundPerm {
		t.Error("Should detect permission issue")
	}
}

func TestSecurityAuditor_FixPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)
	auditor := NewSecurityAuditor(manager)

	// 创建权限过于开放的文件
	openFile := filepath.Join(tmpDir, "open.txt")
	os.WriteFile(openFile, []byte("open"), 0777)

	// 修复权限
	fixed, errs := auditor.FixPermissions(nil, ".")
	if len(errs) > 0 {
		t.Logf("Some errors occurred: %v", errs)
	}

	if fixed == 0 {
		t.Error("Should have fixed at least one file")
	}

	// 验证权限已修复
	info, _ := os.Stat(openFile)
	if info.Mode().Perm() != 0644 {
		t.Errorf("Expected 0644, got %o", info.Mode().Perm())
	}
}

func TestSecurityAuditor_SymlinkDetection(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewSafeFileManager(tmpDir)
	auditor := NewSecurityAuditor(manager)

	// 创建文件和符号链接
	targetFile := filepath.Join(tmpDir, "target.txt")
	os.WriteFile(targetFile, []byte("target"), 0644)

	linkFile := filepath.Join(tmpDir, "link.txt")
	os.Symlink(targetFile, linkFile)

	// 审计符号链接
	result, err := auditor.AuditPath(nil, "link.txt")
	if err != nil {
		t.Fatalf("AuditPath failed: %v", err)
	}

	// 审计应该成功（符号链接指向内部目录）
	if result == nil {
		t.Fatal("AuditPath returned nil result")
	}

	// 验证路径可以安全访问
	safePath, err := manager.SafePath("link.txt")
	if err != nil {
		t.Errorf("SafePath should allow symlink to internal file: %v", err)
	}
	if safePath != linkFile {
		t.Errorf("SafePath returned unexpected path: %s", safePath)
	}
}
