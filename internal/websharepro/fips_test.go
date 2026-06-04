package websharepro

import (
	"bytes"
	"testing"
	"time"
)

func TestNewFIPSTransport(t *testing.T) {
	transport := NewFIPSTransport(nil)
	if transport == nil {
		t.Fatal("NewFIPSTransport returned nil")
	}
	if !transport.config.Enabled {
		t.Error("expected FIPS to be enabled by default")
	}
	if transport.config.ComplianceLevel != FIPSLevel140_3 {
		t.Errorf("expected compliance level 140-3, got %d", transport.config.ComplianceLevel)
	}
}

func TestFIPSCompliant(t *testing.T) {
	transport := NewFIPSTransport(&FIPSConfig{
		Enabled:         true,
		ComplianceLevel: FIPSLevel140_3,
		CipherSuite:     SuiteAES256GCM,
	})
	if !transport.IsCompliant() {
		t.Error("expected transport to be FIPS compliant")
	}
}

func TestFIPSNotCompliant(t *testing.T) {
	transport := NewFIPSTransport(&FIPSConfig{
		Enabled: false,
	})
	if transport.IsCompliant() {
		t.Error("expected transport to be non-compliant when disabled")
	}
}

func TestFIPSEncryptDecryptGCM(t *testing.T) {
	transport := NewFIPSTransport(&FIPSConfig{
		Enabled:         true,
		CipherSuite:     SuiteAES256GCM,
		KeyRotationDays: 90,
		AuditEnabled:    true,
	})

	plaintext := []byte("Hello, FIPS 140-2/3 compliant encryption!")

	result, err := transport.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if result.Ciphertext == nil {
		t.Fatal("expected ciphertext")
	}
	if result.IV == nil {
		t.Fatal("expected IV")
	}
	if result.Tag == nil {
		t.Fatal("expected tag")
	}
	if result.Algorithm != string(SuiteAES256GCM) {
		t.Errorf("expected algorithm AES-256-GCM, got %s", result.Algorithm)
	}

	decrypted, err := transport.Decrypt(result)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("decrypted text doesn't match: got %s", string(decrypted))
	}
}

func TestFIPSEncryptDecryptCBC(t *testing.T) {
	transport := NewFIPSTransport(&FIPSConfig{
		Enabled:         true,
		CipherSuite:     SuiteAES256CBC,
		KeyRotationDays: 90,
		AuditEnabled:    true,
	})

	plaintext := []byte("Testing AES-CBC mode with HMAC integrity")

	result, err := transport.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	decrypted, err := transport.Decrypt(result)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("decrypted text doesn't match")
	}
}

func TestFIPSDisabledEncrypt(t *testing.T) {
	transport := NewFIPSTransport(&FIPSConfig{
		Enabled: false,
	})

	_, err := transport.Encrypt([]byte("test"))
	if err == nil {
		t.Error("expected error when FIPS disabled")
	}
}

func TestFIPSSignVerify(t *testing.T) {
	transport := NewFIPSTransport(nil)

	data := []byte("data to sign")
	sig, err := transport.Sign(data)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	if !transport.Verify(data, sig) {
		t.Error("signature verification failed")
	}

	// 验证错误数据
	if transport.Verify([]byte("wrong data"), sig) {
		t.Error("expected verification to fail for wrong data")
	}
}

func TestFIPSHMAC(t *testing.T) {
	transport := NewFIPSTransport(nil)

	data := []byte("data to HMAC")
	mac := transport.GenerateHMAC(data)

	if !transport.VerifyHMAC(data, mac) {
		t.Error("HMAC verification failed")
	}

	if transport.VerifyHMAC([]byte("wrong data"), mac) {
		t.Error("expected HMAC verification to fail for wrong data")
	}
}

func TestFIPSKeyRotation(t *testing.T) {
	config := &FIPSConfig{
		Enabled:         true,
		CipherSuite:     SuiteAES256GCM,
		KeyRotationDays: 1, // 短周期用于测试
		AuditEnabled:    true,
	}

	transport := NewFIPSTransport(config)

	// 加密一些数据
	result, err := transport.Encrypt([]byte("test"))
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// 手动设置密钥过期
	transport.mu.Lock()
	for _, k := range transport.keys {
		k.ExpiresAt = time.Now().Add(-1 * time.Hour)
	}
	transport.mu.Unlock()

	// 轮换密钥
	rotated := transport.RotateKeys()
	if rotated == 0 {
		t.Error("expected at least one key rotation")
	}

	// 验证旧数据仍可解密（使用旧密钥）
	decrypted, err := transport.Decrypt(result)
	if err != nil {
		t.Fatalf("decrypt after rotation failed: %v", err)
	}
	if string(decrypted) != "test" {
		t.Errorf("decrypted text doesn't match")
	}
}

func TestFIPSAuditLog(t *testing.T) {
	transport := NewFIPSTransport(&FIPSConfig{
		Enabled:         true,
		AuditEnabled:    true,
		KeyRotationDays: 90,
	})

	// 执行一些操作
	transport.Encrypt([]byte("test"))
	transport.Sign([]byte("test"))

	log := transport.GetAuditLog()
	if len(log) == 0 {
		t.Error("expected audit log entries")
	}

	// 检查日志内容
	foundEncrypt := false
	for _, entry := range log {
		if entry.Operation == "encrypt-gcm" && entry.Success {
			foundEncrypt = true
		}
	}
	if !foundEncrypt {
		t.Error("expected encrypt-gcm audit entry")
	}
}

func TestFIPSGetKeyInfo(t *testing.T) {
	transport := NewFIPSTransport(nil)

	keys := transport.GetKeyInfo()
	if len(keys) == 0 {
		t.Error("expected at least one key")
	}

	for _, key := range keys {
		if _, exists := key["id"]; !exists {
			t.Error("expected key to have id")
		}
		if _, exists := key["algorithm"]; !exists {
			t.Error("expected key to have algorithm")
		}
		// 确保密钥数据不泄露
		if _, exists := key["keyData"]; exists {
			t.Error("key data should not be exposed")
		}
	}
}

func TestFIPSHash(t *testing.T) {
	transport := NewFIPSTransport(nil)

	data := []byte("test data")

	// SHA-256
	hash256 := transport.Hash(data, "SHA-256")
	if len(hash256) != 32 {
		t.Errorf("expected 32 bytes for SHA-256, got %d", len(hash256))
	}

	// SHA-384
	hash384 := transport.Hash(data, "SHA-384")
	if len(hash384) != 48 {
		t.Errorf("expected 48 bytes for SHA-384, got %d", len(hash384))
	}

	// SHA-512
	hash512 := transport.Hash(data, "SHA-512")
	if len(hash512) != 64 {
		t.Errorf("expected 64 bytes for SHA-512, got %d", len(hash512))
	}
}

func TestFIPSRandomKeyGeneration(t *testing.T) {
	key1, err := GenerateRandomKey(32)
	if err != nil {
		t.Fatalf("generate random key failed: %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(key1))
	}

	key2, err := GenerateRandomKey(32)
	if err != nil {
		t.Fatalf("generate random key failed: %v", err)
	}

	// 两次生成的密钥应该不同
	if bytes.Equal(key1, key2) {
		t.Error("expected different random keys")
	}
}

func TestFIPSHexEncodeDecode(t *testing.T) {
	data := []byte("test data")
	encoded := HexEncode(data)

	decoded, err := HexDecode(encoded)
	if err != nil {
		t.Fatalf("hex decode failed: %v", err)
	}

	if !bytes.Equal(data, decoded) {
		t.Error("hex encode/decode roundtrip failed")
	}
}

func TestFIPSCipherSuites(t *testing.T) {
	suites := []FIPSCipherSuite{
		SuiteAES256GCM,
		SuiteAES128GCM,
		SuiteAES256CBC,
		SuiteHMACSHA256,
		SuiteECDSAP256,
	}

	for _, suite := range suites {
		transport := NewFIPSTransport(&FIPSConfig{
			Enabled:     true,
			CipherSuite: suite,
		})
		if !transport.IsCompliant() {
			t.Errorf("expected %s to be compliant", suite)
		}
	}
}
