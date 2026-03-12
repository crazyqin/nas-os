package auth

import (
	"testing"
)

func TestGenerateBackupCodes(t *testing.T) {
	mgr := NewBackupCodeManager()

	codes, err := mgr.GenerateBackupCodes("user123", 10)
	if err != nil {
		t.Fatalf("生成备份码失败：%v", err)
	}

	if len(codes) != 10 {
		t.Errorf("期望生成 10 个备份码，实际：%d", len(codes))
	}

	// 检查格式 XXXX-XXXX
	for i, code := range codes {
		if len(code) != 17 { // 8 + 1 + 8
			t.Errorf("备份码 %d 长度不正确：%s", i, code)
		}
		if code[8] != '-' {
			t.Errorf("备份码 %d 格式不正确：%s", i, code)
		}
	}

	t.Logf("生成的备份码：%v", codes)
}

func TestVerifyBackupCode(t *testing.T) {
	mgr := NewBackupCodeManager()

	// 生成备份码
	codes, err := mgr.GenerateBackupCodes("user123", 5)
	if err != nil {
		t.Fatalf("生成备份码失败：%v", err)
	}

	// 验证第一个备份码
	if err := mgr.VerifyBackupCode("user123", codes[0]); err != nil {
		t.Errorf("验证备份码失败：%v", err)
	}

	// 再次验证同一个备份码（应该失败）
	if err := mgr.VerifyBackupCode("user123", codes[0]); err == nil {
		t.Error("已使用的备份码验证通过")
	}

	// 验证其他备份码
	for i := 1; i < len(codes); i++ {
		if err := mgr.VerifyBackupCode("user123", codes[i]); err != nil {
			t.Errorf("验证备份码 %d 失败：%v", i, err)
		}
	}
}

func TestGetUnusedCount(t *testing.T) {
	mgr := NewBackupCodeManager()

	// 初始为 0
	if count := mgr.GetUnusedCount("user123"); count != 0 {
		t.Errorf("初始备份码数量应为 0，实际：%d", count)
	}

	// 生成 10 个
	codes, _ := mgr.GenerateBackupCodes("user123", 10)
	if count := mgr.GetUnusedCount("user123"); count != 10 {
		t.Errorf("期望 10 个未使用备份码，实际：%d", count)
	}

	// 使用 3 个
	for i := 0; i < 3; i++ {
		_ = mgr.VerifyBackupCode("user123", codes[i])
	}

	if count := mgr.GetUnusedCount("user123"); count != 7 {
		t.Errorf("期望 7 个未使用备份码，实际：%d", count)
	}
}

func TestInvalidateAll(t *testing.T) {
	mgr := NewBackupCodeManager()

	// 生成备份码
	codes, _ := mgr.GenerateBackupCodes("user123", 5)

	// 使所有失效
	mgr.InvalidateAll("user123")

	// 验证应该失败
	for _, code := range codes {
		if err := mgr.VerifyBackupCode("user123", code); err == nil {
			t.Error("已失效的备份码验证通过")
		}
	}
}
