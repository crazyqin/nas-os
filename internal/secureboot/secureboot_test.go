package secureboot

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.policy.Mode != PolicyDisabled {
		t.Errorf("默认策略 = %q, want %q", m.policy.Mode, PolicyDisabled)
	}
}

func TestDetectTPM(t *testing.T) {
	m := NewManager(nil)
	info, err := m.DetectTPM(context.Background())
	if err != nil {
		t.Fatalf("DetectTPM 失败: %v", err)
	}
	if info.State != TPMActive {
		t.Errorf("TPM 状态 = %q, want %q", info.State, TPMActive)
	}
	if info.Version != TPM20 {
		t.Errorf("TPM 版本 = %q, want %q", info.Version, TPM20)
	}
}

func TestSetBootPolicy(t *testing.T) {
	m := NewManager(nil)

	if err := m.SetBootPolicy(PolicyEnforce); err != nil {
		t.Fatalf("设置 enforce 策略失败: %v", err)
	}
	if m.GetBootPolicy() != PolicyEnforce {
		t.Errorf("策略 = %q, want %q", m.GetBootPolicy(), PolicyEnforce)
	}

	if err := m.SetBootPolicy(PolicyPermissive); err != nil {
		t.Fatalf("设置 permissive 策略失败: %v", err)
	}

	if err := m.SetBootPolicy(BootPolicy("invalid")); err == nil {
		t.Fatal("无效策略应返回错误")
	}
}

func TestAddRemoveTrustedKey(t *testing.T) {
	m := NewManager(nil)

	key := KeyInfo{
		ID:          "key-1",
		Name:        "Test Key",
		Fingerprint: "sha256:abc123",
		Type:        "rsa",
		Size:        4096,
	}
	if err := m.AddTrustedKey(key); err != nil {
		t.Fatalf("AddTrustedKey 失败: %v", err)
	}

	cfg := m.GetSecureBootConfig()
	if len(cfg.TrustedKeys) != 1 {
		t.Fatalf("信任密钥数 = %d, want 1", len(cfg.TrustedKeys))
	}

	if err := m.RemoveTrustedKey("sha256:abc123"); err != nil {
		t.Fatalf("RemoveTrustedKey 失败: %v", err)
	}
	cfg = m.GetSecureBootConfig()
	if len(cfg.TrustedKeys) != 0 {
		t.Errorf("删除后密钥数 = %d, want 0", len(cfg.TrustedKeys))
	}
}

func TestAddDuplicateKey(t *testing.T) {
	m := NewManager(nil)
	key := KeyInfo{Fingerprint: "sha256:dup", Name: "dup-key"}
	m.AddTrustedKey(key)
	if err := m.AddTrustedKey(key); err == nil {
		t.Fatal("重复添加密钥应返回错误")
	}
}

func TestAddTrustedHash(t *testing.T) {
	m := NewManager(nil)

	hash := "sha256:deadbeef"
	if err := m.AddTrustedHash(hash); err != nil {
		t.Fatalf("AddTrustedHash 失败: %v", err)
	}

	cfg := m.GetSecureBootConfig()
	if len(cfg.TrustedHashes) != 1 {
		t.Fatalf("信任哈希数 = %d, want 1", len(cfg.TrustedHashes))
	}

	if err := m.AddTrustedHash(hash); err == nil {
		t.Fatal("重复添加哈希应返回错误")
	}
}

func TestRegisterBootEntry(t *testing.T) {
	m := NewManager(nil)

	entry := BootEntry{
		Name:            "grubx64.efi",
		Path:            "/boot/EFI/grub/grubx64.efi",
		SignatureStatus: SigNotChecked,
		Hash:            "sha256:abcd",
	}
	if err := m.RegisterBootEntry(entry); err != nil {
		t.Fatalf("RegisterBootEntry 失败: %v", err)
	}

	got, err := m.GetBootEntry("grubx64.efi")
	if err != nil {
		t.Fatalf("GetBootEntry 失败: %v", err)
	}
	if got.Path != "/boot/EFI/grub/grubx64.efi" {
		t.Errorf("路径 = %q, want /boot/EFI/grub/grubx64.efi", got.Path)
	}

	entries := m.ListBootEntries()
	if len(entries) != 1 {
		t.Errorf("启动项数 = %d, want 1", len(entries))
	}
}

func TestVerifySignature(t *testing.T) {
	m := NewManager(nil)

	// 注册启动项和信任哈希
	entry := BootEntry{
		Name:            "shimx64.efi",
		Path:            "/boot/EFI/boot/shimx64.efi",
		SignatureStatus: SigNotChecked,
		Hash:            "sha256:trustedhash",
	}
	m.RegisterBootEntry(entry)
	m.AddTrustedHash("sha256:trustedhash")

	// 验证已知信任项
	result, err := m.VerifySignature(context.Background(), "shimx64.efi", "sha256:trustedhash")
	if err != nil {
		t.Fatalf("VerifySignature 失败: %v", err)
	}
	if result.Status != SigValid {
		t.Errorf("验证状态 = %q, want %q", result.Status, SigValid)
	}

	// 验证未知项
	result, err = m.VerifySignature(context.Background(), "shimx64.efi", "sha256:unknown")
	if err != nil {
		t.Fatalf("VerifySignature 失败: %v", err)
	}
	if result.Status != SigInvalid {
		t.Errorf("验证状态 = %q, want %q", result.Status, SigInvalid)
	}
}

func TestVerifyNonexistentComponent(t *testing.T) {
	m := NewManager(nil)
	result, err := m.VerifySignature(context.Background(), "nonexistent", "sha256:any")
	if err != nil {
		t.Fatalf("VerifySignature 失败: %v", err)
	}
	if result.Status != SigUnknown {
		t.Errorf("验证状态 = %q, want %q", result.Status, SigUnknown)
	}
}

func TestGetStatus(t *testing.T) {
	m := NewManager(nil)
	m.SetBootPolicy(PolicyEnforce)
	m.DetectTPM(context.Background())

	status := m.GetStatus()
	if !status.Enabled {
		t.Error("enforce 模式下应为已启用")
	}
	if status.BootMode != BootModeSecure {
		t.Errorf("启动模式 = %q, want %q", status.BootMode, BootModeSecure)
	}
}
