package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityAuditLogger_Log(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)

	// 测试基本日志记录
	entry := SecurityAuditEntry{
		Category: "auth",
		Event:    "login",
		UserID:   "user1",
		Username: "testuser",
		IP:       "192.168.1.1",
		Status:   "success",
	}

	logger.Log(entry)

	// 验证日志文件已创建
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("日志文件未创建")
	}

	// 验证日志内容
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件为空")
	}
}

func TestSecurityAuditLogger_LogLogin(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)

	// 测试成功登录
	logger.LogLogin("user1", "testuser", "192.168.1.1", "Mozilla/5.0", "success", "")

	// 测试失败登录
	logger.LogLoginAttempt("testuser", "192.168.1.1", "Mozilla/5.0", "failure", "密码错误")

	// 验证日志记录
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件为空")
	}
}

func TestSecurityAuditLogger_LogMFA(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)

	// 测试 MFA 设置
	logger.LogMFASetup("user1", "testuser", "192.168.1.1", "totp", "success")

	// 测试 MFA 验证成功
	logger.LogMFAVerify("user1", "testuser", "192.168.1.1", "totp", "success", "")

	// 测试 MFA 验证失败
	logger.LogMFAVerify("user1", "testuser", "192.168.1.1", "totp", "failure", "验证码无效")

	// 验证日志记录
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件为空")
	}
}

func TestSecurityAuditLogger_LogPassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)

	// 测试密码变更
	logger.LogPasswordChange("user1", "testuser", "192.168.1.1", "success", "")

	// 测试密码重置
	logger.LogPasswordReset("user1", "testuser", "192.168.1.1", "success", "")

	// 验证日志记录
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件为空")
	}
}

func TestSecurityAuditLogger_LogSession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)

	// 测试会话创建
	logger.LogSessionCreate("user1", "testuser", "192.168.1.1", "Mozilla/5.0", "device-123")

	// 测试会话失效
	logger.LogSessionInvalidate("user1", "testuser", "192.168.1.1", "用户主动登出")

	// 验证日志记录
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件为空")
	}
}

func TestSecurityAuditLogger_LogAccountLock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)

	// 测试账户锁定
	logger.LogAccountLock("testuser", "192.168.1.1", "连续5次登录失败")

	// 测试账户解锁
	logger.LogAccountUnlock("testuser", "admin", "管理员手动解锁")

	// 验证日志记录
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件为空")
	}
}

func TestSecurityAuditLogger_Disabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)
	logger.SetEnabled(false)

	// 禁用后不应记录日志
	logger.LogLogin("user1", "testuser", "192.168.1.1", "Mozilla/5.0", "success", "")

	// 验证日志文件不存在或为空
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		data, _ := os.ReadFile(logPath)
		if len(data) > 0 {
			t.Error("禁用状态不应记录日志")
		}
	}
}

func TestSecurityAuditLogger_MultipleEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "security-audit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security-audit.log")
	logger := NewSecurityAuditLogger(logPath)

	// 记录多条日志
	for i := 0; i < 100; i++ {
		logger.LogLogin("user1", "testuser", "192.168.1.1", "Mozilla/5.0", "success", "")
	}

	// 验证日志记录
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件为空")
	}
}

func TestAuditLogGlobals(t *testing.T) {
	// 测试全局函数（使用默认记录器）
	// 由于默认记录器写入 /var/log/nas-os/，我们只验证不会崩溃
	AuditLogin("user1", "testuser", "192.168.1.1", "Mozilla/5.0", "success", "")
	AuditMFASetup("user1", "testuser", "192.168.1.1", "totp", "success")
	AuditPasswordChange("user1", "testuser", "192.168.1.1", "success", "")
}