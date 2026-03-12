package auth

import (
	"testing"
	"time"
)

func TestSMSManager_SendCode(t *testing.T) {
	provider := &MockSMSProvider{}
	mgr := NewSMSManager(provider)

	phone := "+8613800138000"
	err := mgr.SendCode(phone)
	if err != nil {
		t.Fatalf("发送验证码失败：%v", err)
	}

	// 检查验证码是否存储
	if _, ok := provider.Codes[phone]; !ok {
		t.Error("验证码未存储")
	}

	t.Logf("发送验证码到 %s: %s", phone, provider.Codes[phone])
}

func TestSMSManager_VerifyCode(t *testing.T) {
	provider := &MockSMSProvider{}
	mgr := NewSMSManager(provider)

	phone := "+8613800138000"

	// 发送验证码
	_ = mgr.SendCode(phone)
	code := provider.Codes[phone]

	// 验证正确验证码
	if err := mgr.VerifyCode(phone, code); err != nil {
		t.Errorf("验证正确验证码失败：%v", err)
	}

	// 再次发送验证码
	_ = mgr.SendCode(phone)
	newCode := provider.Codes[phone]

	// 验证错误验证码
	if err := mgr.VerifyCode(phone, "000000"); err == nil {
		t.Error("错误验证码验证通过")
	}

	// 验证正确验证码
	if err := mgr.VerifyCode(phone, newCode); err != nil {
		t.Errorf("验证正确验证码失败：%v", err)
	}
}

func TestSMSManager_MaxAttempts(t *testing.T) {
	provider := &MockSMSProvider{}
	mgr := NewSMSManager(provider)
	mgr.maxAttempts = 3

	phone := "+8613800138000"
	_ = mgr.SendCode(phone)

	// 尝试 3 次错误验证码
	for i := 0; i < 3; i++ {
		err := mgr.VerifyCode(phone, "000000")
		if err == nil && i < 2 {
			t.Error("错误验证码验证通过")
		}
	}

	// 第 4 次应该失败（超过最大尝试次数）
	if err := mgr.VerifyCode(phone, provider.Codes[phone]); err == nil {
		t.Error("超过最大尝试次数后仍可验证")
	}
}

func TestSMSManager_Expiration(t *testing.T) {
	provider := &MockSMSProvider{}
	mgr := NewSMSManager(provider)
	mgr.validity = 1 * time.Second // 1 秒过期

	phone := "+8613800138000"
	mgr.SendCode(phone)
	code := provider.Codes[phone]

	// 等待过期
	time.Sleep(2 * time.Second)

	// 验证应该失败（已过期）
	if err := mgr.VerifyCode(phone, code); err == nil {
		t.Error("过期验证码验证通过")
	}
}

func TestSMSManager_SendRateLimit(t *testing.T) {
	provider := &MockSMSProvider{}
	mgr := NewSMSManager(provider)

	phone := "+8613800138000"

	// 第一次发送
	if err := mgr.SendCode(phone); err != nil {
		t.Fatalf("第一次发送失败：%v", err)
	}

	// 立即再次发送（应该失败）
	if err := mgr.SendCode(phone); err == nil {
		t.Error("30 秒内重复发送成功")
	}
}
