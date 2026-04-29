package secureboot

import (
	"testing"
)

func TestDefaultSecureBootConfig(t *testing.T) {
	cfg := DefaultSecureBootConfig()

	if !cfg.Enabled {
		t.Error("默认配置应启用 Secure Boot")
	}
	if cfg.Mode != ModeStandard {
		t.Errorf("默认模式应为 standard，实际为 %s", cfg.Mode)
	}
	if cfg.SecureBootState != StateEnabled {
		t.Errorf("默认状态应为 enabled，实际为 %s", cfg.SecureBootState)
	}
	if cfg.TPMEnabled {
		t.Error("默认不应启用 TPM")
	}
}

func TestDefaultSecureBootPolicy(t *testing.T) {
	p := DefaultSecureBootPolicy()

	if !p.EnforceKernelSignature {
		t.Error("默认应强制内核签名验证")
	}
	if !p.EnforceModuleSignature {
		t.Error("默认应强制模块签名验证")
	}
	if p.AllowCustomKeys {
		t.Error("默认不应允许自定义密钥")
	}
	if p.AuditMode {
		t.Error("默认不应为审计模式")
	}
}

func TestKeyStoreOperations(t *testing.T) {
	ks := newKeyStore()

	// 添加条目
	entry := &KeyEntry{
		Type:    SigTypeX509,
		KeyType: KeyTypeDB,
		Hash:    [32]byte{1, 2, 3},
	}
	ks.add(KeyTypeDB, entry)

	// 列出
	entries := ks.list(KeyTypeDB)
	if len(entries) != 1 {
		t.Fatalf("期望 1 个 db 条目，实际 %d", len(entries))
	}
	if entries[0].Hash != entry.Hash {
		t.Error("哈希不匹配")
	}

	// 计数
	if ks.count() != 1 {
		t.Errorf("期望总数 1，实际 %d", ks.count())
	}

	// 移除
	removed := ks.remove(KeyTypeDB, entry.Hash)
	if !removed {
		t.Error("应成功移除条目")
	}
	if ks.count() != 0 {
		t.Error("移除后应为 0")
	}

	// 移除不存在的
	removed = ks.remove(KeyTypeDB, [32]byte{9, 9, 9})
	if removed {
		t.Error("不应移除不存在的条目")
	}
}

func TestKeyStoreRevoked(t *testing.T) {
	ks := newKeyStore()
	hash := [32]byte{0xaa, 0xbb}

	// 未添加时不应被标记为吊销
	if ks.isRevoked(hash) {
		t.Error("未添加时不应被吊销")
	}

	// 添加到 dbx
	now := testTime()
	ks.add(KeyTypeDBX, &KeyEntry{
		KeyType:   KeyTypeDBX,
		Hash:      hash,
		RevokedAt: &now,
	})

	if !ks.isRevoked(hash) {
		t.Error("应被标记为吊销")
	}
}

func TestKeyStoreConcurrency(t *testing.T) {
	ks := newKeyStore()
	done := make(chan bool, 20)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(n int) {
			hash := [32]byte{byte(n)}
			ks.add(KeyTypeDB, &KeyEntry{
				KeyType: KeyTypeDB,
				Hash:    hash,
			})
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_ = ks.list(KeyTypeDB)
			_ = ks.count()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	if ks.count() != 10 {
		t.Errorf("期望 10 个条目，实际 %d", ks.count())
	}
}

func TestHashCertificate(t *testing.T) {
	cert := generateTestCert(t, "Test Cert")
	h1 := hashCertificate(cert)
	h2 := hashCertificate(cert)

	if h1 != h2 {
		t.Error("同一证书的哈希应相同")
	}

	cert2 := generateTestCert(t, "Other Cert")
	h3 := hashCertificate(cert2)

	if h1 == h3 {
		t.Error("不同证书的哈希应不同")
	}
}

func TestHashData(t *testing.T) {
	data := []byte("test data")
	h1 := hashData(data)
	h2 := hashData(data)

	if h1 != h2 {
		t.Error("相同数据的哈希应相同")
	}

	h3 := hashData([]byte("other data"))
	if h1 == h3 {
		t.Error("不同数据的哈希应不同")
	}
}

func TestComputeDigest(t *testing.T) {
	data := []byte("hello world")

	d256 := ComputeDigest(data, HashSHA256)
	if d256.Algorithm != HashSHA256 {
		t.Error("算法应为 sha256")
	}
	if len(d256.Value) != 32 {
		t.Errorf("SHA256 摘要长度应为 32，实际 %d", len(d256.Value))
	}

	d384 := ComputeDigest(data, HashSHA384)
	if d384.Algorithm != HashSHA384 {
		t.Error("算法应为 sha384")
	}
	if len(d384.Value) != 48 {
		t.Errorf("SHA384 摘要长度应为 48，实际 %d", len(d384.Value))
	}

	d512 := ComputeDigest(data, HashSHA512)
	if d512.Algorithm != HashSHA512 {
		t.Error("算法应为 sha512")
	}
	if len(d512.Value) != 64 {
		t.Errorf("SHA512 摘要长度应为 64，实际 %d", len(d512.Value))
	}

	// 未知算法应默认 SHA256
	dUnknown := ComputeDigest(data, "unknown")
	if dUnknown.Algorithm != HashSHA256 {
		t.Error("未知算法应默认 sha256")
	}
}

func TestValidationError(t *testing.T) {
	e := &ValidationError{
		Code:    "ERR_TEST",
		Message: "测试错误",
	}

	expected := "[ERR_TEST] 测试错误"
	if e.Error() != expected {
		t.Errorf("错误信息不匹配：期望 %q，实际 %q", expected, e.Error())
	}
}

func TestKeyTypeConstants(t *testing.T) {
	tests := []struct {
		keyType KeyType
		want    string
	}{
		{KeyTypePK, "PK"},
		{KeyTypeKEK, "KEK"},
		{KeyTypeDB, "db"},
		{KeyTypeDBX, "dbx"},
	}
	for _, tt := range tests {
		if string(tt.keyType) != tt.want {
			t.Errorf("KeyType %v 应为 %s", tt.keyType, tt.want)
		}
	}
}

func TestModeConstants(t *testing.T) {
	if ModeStrict != "strict" {
		t.Error("ModeStrict 应为 strict")
	}
	if ModeStandard != "standard" {
		t.Error("ModeStandard 应为 standard")
	}
	if ModeAudit != "audit" {
		t.Error("ModeAudit 应为 audit")
	}
}
