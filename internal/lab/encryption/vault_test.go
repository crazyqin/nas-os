// Package encryption Vault 测试
// 刑部 Round 241 - Vault Password 加密卷
package encryption

import (
	"encoding/base64"
	"sync"
	"testing"
)

// ========== VaultManager 核心测试 ==========

func TestVaultManager_CreateVault(t *testing.T) {
	vm := NewVaultManager()

	// 测试正常创建
	vault, err := vm.CreateVault("test-vault", "password123")
	if err != nil {
		t.Fatalf("创建 vault 失败: %v", err)
	}
	if vault.ID == "" {
		t.Error("vault ID 不应为空")
	}
	if vault.Name != "test-vault" {
		t.Errorf("vault 名称应为 'test-vault'，实际为 '%s'", vault.Name)
	}
	if vault.State != VaultLocked {
		t.Errorf("新建 vault 状态应为 locked，实际为 %s", vault.State)
	}
	if vault.Algorithm != AlgorithmAES256GCM {
		t.Errorf("加密算法应为 AES-256-GCM，实际为 %s", vault.Algorithm)
	}

	// 验证加密数据已存储
	if vault.EncryptedKey == "" {
		t.Error("EncryptedKey 不应为空")
	}
	if vault.Salt == "" {
		t.Error("Salt 不应为空")
	}
	// 验证是合法的 Base64
	if _, err := base64.StdEncoding.DecodeString(vault.EncryptedKey); err != nil {
		t.Errorf("EncryptedKey 不是合法 Base64: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(vault.Salt); err != nil {
		t.Errorf("Salt 不是合法 Base64: %v", err)
	}
}

func TestVaultManager_CreateVault_Validation(t *testing.T) {
	vm := NewVaultManager()

	// 空名称
	_, err := vm.CreateVault("", "password123")
	if err == nil {
		t.Error("空名称应返回错误")
	}

	// 空密码
	_, err = vm.CreateVault("test", "")
	if err == nil {
		t.Error("空密码应返回错误")
	}

	// 密码太短
	_, err = vm.CreateVault("test", "short")
	if err == nil {
		t.Error("短密码应返回错误")
	}

	// 重复名称
	_, _ = vm.CreateVault("dup", "password123")
	_, err = vm.CreateVault("dup", "password456")
	if err == nil {
		t.Error("重复名称应返回错误")
	}
}

func TestVaultManager_UnlockAndLockVault(t *testing.T) {
	vm := NewVaultManager()

	vault, err := vm.CreateVault("lock-test", "password123")
	if err != nil {
		t.Fatalf("创建 vault 失败: %v", err)
	}

	// 解锁 - 正确密码
	dv, err := vm.UnlockVault(vault.ID, "password123")
	if err != nil {
		t.Fatalf("解锁 vault 失败: %v", err)
	}
	if dv == nil {
		t.Fatal("解密后的 vault 不应为 nil")
	}
	if len(dv.DecodedKey) == 0 {
		t.Error("解密后的密钥不应为空")
	}
	if !vm.IsUnlocked(vault.ID) {
		t.Error("vault 应处于解锁状态")
	}

	// 重复解锁应返回缓存
	dv2, err := vm.UnlockVault(vault.ID, "password123")
	if err != nil {
		t.Fatalf("重复解锁失败: %v", err)
	}
	if dv2 != dv {
		t.Error("重复解锁应返回相同的解密实例")
	}

	// 锁定
	err = vm.LockVault(vault.ID)
	if err != nil {
		t.Fatalf("锁定 vault 失败: %v", err)
	}
	if vm.IsUnlocked(vault.ID) {
		t.Error("vault 应处于锁定状态")
	}

	// 锁定后解密数据应被清除
	if _, exists := vm.decrypted[vault.ID]; exists {
		t.Error("锁定后解密数据应被清除")
	}
}

func TestVaultManager_UnlockVault_WrongPassword(t *testing.T) {
	vm := NewVaultManager()

	vault, err := vm.CreateVault("pwd-test", "correct-password")
	if err != nil {
		t.Fatalf("创建 vault 失败: %v", err)
	}

	// 错误密码
	_, err = vm.UnlockVault(vault.ID, "wrong-password")
	if err == nil {
		t.Error("错误密码应返回错误")
	}

	// vault 应仍为锁定状态
	if vm.IsUnlocked(vault.ID) {
		t.Error("密码错误后 vault 应仍为锁定状态")
	}
}

func TestVaultManager_UnlockVault_NotFound(t *testing.T) {
	vm := NewVaultManager()

	_, err := vm.UnlockVault("non-existent-id", "password")
	if err == nil {
		t.Error("不存在的 vault ID 应返回错误")
	}
}

func TestVaultManager_DeleteVault(t *testing.T) {
	vm := NewVaultManager()

	vault, err := vm.CreateVault("to-delete", "password123")
	if err != nil {
		t.Fatalf("创建 vault 失败: %v", err)
	}

	// 先解锁
	_, _ = vm.UnlockVault(vault.ID, "password123")

	// 删除
	err = vm.DeleteVault(vault.ID)
	if err != nil {
		t.Fatalf("删除 vault 失败: %v", err)
	}

	// 验证已删除
	_, err = vm.GetVault(vault.ID)
	if err == nil {
		t.Error("已删除的 vault 应返回错误")
	}

	// 验证解密数据已清除
	if _, exists := vm.decrypted[vault.ID]; exists {
		t.Error("删除后解密数据应被清除")
	}

	// 删除不存在的 vault
	err = vm.DeleteVault("non-existent")
	if err == nil {
		t.Error("删除不存在的 vault 应返回错误")
	}
}

func TestVaultManager_ListVaults(t *testing.T) {
	vm := NewVaultManager()

	// 空列表
	vaults := vm.ListVaults()
	if len(vaults) != 0 {
		t.Errorf("空管理器应返回空列表，实际 %d 个", len(vaults))
	}

	// 创建多个 vault
	_, _ = vm.CreateVault("vault-1", "password123")
	_, _ = vm.CreateVault("vault-2", "password456")
	_, _ = vm.CreateVault("vault-3", "password789")

	vaults = vm.ListVaults()
	if len(vaults) != 3 {
		t.Errorf("应有 3 个 vault，实际 %d 个", len(vaults))
	}

	// 验证敏感字段被隐藏
	for _, v := range vaults {
		if v.EncryptedKey != "***" {
			t.Error("列表中 EncryptedKey 应被隐藏")
		}
		if v.Salt != "***" {
			t.Error("列表中 Salt 应被隐藏")
		}
	}
}

func TestVaultManager_AutoLockExpired(t *testing.T) {
	vm := NewVaultManager()
	vm.SetConfig(VaultConfig{
		Algorithm:    AlgorithmAES256GCM,
		Iterations:   PBKDF2Iterations,
		AutoLockMins: 0, // 立即过期（测试用）
	})

	vault, _ := vm.CreateVault("auto-lock", "password123")
	_, _ = vm.UnlockVault(vault.ID, "password123")

	if !vm.IsUnlocked(vault.ID) {
		t.Fatal("vault 应为解锁状态")
	}

	// AutoLockMins=0 不应锁定
	locked := vm.AutoLockExpired()
	// 0 表示禁用，不应锁定
	if locked != 0 {
		t.Errorf("AutoLockMins=0 时不应锁定，实际锁定 %d 个", locked)
	}

	// 设置 AutoLockMins=1（1分钟）
	vm.config.AutoLockMins = 1

	// 由于 LastAccessed 就是当前时间，1 分钟内不应过期
	locked = vm.AutoLockExpired()
	if locked != 0 {
		t.Errorf("未超时不应锁定，实际锁定 %d 个", locked)
	}
}

func TestVaultManager_Concurrent(t *testing.T) {
	vm := NewVaultManager()

	// 并发创建
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := "concurrent-" + string(rune('A'+id))
			_, _ = vm.CreateVault(name, "password123456")
		}(i)
	}
	wg.Wait()

	vaults := vm.ListVaults()
	if len(vaults) != 10 {
		t.Errorf("并发创建应有 10 个 vault，实际 %d 个", len(vaults))
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vm.ListVaults()
		}()
	}
	wg.Wait()
}

func TestVaultManager_GetVault(t *testing.T) {
	vm := NewVaultManager()

	vault, _ := vm.CreateVault("get-test", "password123")

	// 获取存在的 vault
	info, err := vm.GetVault(vault.ID)
	if err != nil {
		t.Fatalf("获取 vault 失败: %v", err)
	}
	if info.ID != vault.ID {
		t.Errorf("vault ID 不匹配: %s vs %s", info.ID, vault.ID)
	}
	if info.EncryptedKey != "***" {
		t.Error("GetVault 应隐藏 EncryptedKey")
	}

	// 获取不存在的 vault
	_, err = vm.GetVault("non-existent")
	if err == nil {
		t.Error("获取不存在的 vault 应返回错误")
	}
}

// ========== 加密原语测试 ==========

func TestEncryptDecryptAESGCM(t *testing.T) {
	key := make([]byte, KeyLength)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("NAS-OS Vault 测试数据 🔐")

	// 加密
	ciphertext, err := encryptAESGCM(plaintext, key)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 密文不应等于明文
	if string(ciphertext) == string(plaintext) {
		t.Error("密文不应等于明文")
	}

	// 解密
	decrypted, err := decryptAESGCM(ciphertext, key)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("解密结果不匹配: '%s' vs '%s'", decrypted, plaintext)
	}
}

func TestDecryptAESGCM_WrongKey(t *testing.T) {
	key := make([]byte, KeyLength)
	wrongKey := make([]byte, KeyLength)
	for i := range key {
		key[i] = byte(i)
		wrongKey[i] = byte(i + 1)
	}

	plaintext := []byte("secret data")
	ciphertext, _ := encryptAESGCM(plaintext, key)

	_, err := decryptAESGCM(ciphertext, wrongKey)
	if err == nil {
		t.Error("错误密钥应返回解密失败")
	}
}

func TestZeroBytes(t *testing.T) {
	data := []byte{0xFF, 0xAB, 0x12, 0x00, 0x99}
	zeroBytes(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("索引 %d 应为 0，实际为 %d", i, b)
		}
	}
}
