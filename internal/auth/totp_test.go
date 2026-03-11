package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTPSecret(t *testing.T) {
	// 使用 SetupTOTP 生成密钥（因为 GenerateTOTPSecret 需要 issuer）
	setup, err := SetupTOTP("TestIssuer", "test@example.com")
	if err != nil {
		t.Fatalf("生成 TOTP 密钥失败：%v", err)
	}

	if len(setup.Secret) == 0 {
		t.Error("生成的密钥为空")
	}

	t.Logf("生成的密钥：%s", setup.Secret)
}

func TestGenerateTOTPURI(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	uri := GenerateTOTPURI(secret, "TestIssuer", "test@example.com")

	expected := "otpauth://totp/TestIssuer:test@example.com?secret=JBSWY3DPEHPK3PXP&issuer=TestIssuer&algorithm=SHA1&digits=6&period=30"
	if uri != expected {
		t.Errorf("URI 不匹配:\n期望：%s\n实际：%s", expected, uri)
	}
}

func TestVerifyTOTP(t *testing.T) {
	// 使用 SetupTOTP 生成密钥
	setup, err := SetupTOTP("TestIssuer", "test@example.com")
	if err != nil {
		t.Fatalf("生成密钥失败：%v", err)
	}

	// 生成当前时间的验证码
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatalf("生成验证码失败：%v", err)
	}

	// 验证
	if !VerifyTOTP(setup.Secret, code) {
		t.Error("验证码验证失败")
	}

	// 测试错误验证码
	if VerifyTOTP(setup.Secret, "000000") {
		t.Error("错误验证码验证通过")
	}
}

func TestSetupTOTP(t *testing.T) {
	setup, err := SetupTOTP("TestIssuer", "test@example.com")
	if err != nil {
		t.Fatalf("设置 TOTP 失败：%v", err)
	}

	if setup.Secret == "" {
		t.Error("密钥为空")
	}

	if setup.URI == "" {
		t.Error("URI 为空")
	}

	if setup.QRCode == "" {
		t.Error("QR 码为空")
	}

	if setup.Issuer != "TestIssuer" {
		t.Errorf("发行者不匹配：%s", setup.Issuer)
	}

	if setup.AccountName != "test@example.com" {
		t.Errorf("账户名不匹配：%s", setup.AccountName)
	}

	t.Logf("TOTP 设置成功：密钥=%s", setup.Secret)
}
