package fipsvault

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:         true,
		FIPSLevel:       FIPSLevel140_3,
		DefaultCipher:   CipherAES256GCM,
		MinTLSVersion:   "1.3",
		KeyRotationDays: 90,
		AuditEnabled:    true,
	}
	m := NewManager(config)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config.FIPSLevel != FIPSLevel140_3 {
		t.Errorf("expected FIPS 140-3, got %s", m.config.FIPSLevel)
	}
}

func TestGenerateKey(t *testing.T) {
	m := NewManager(&Config{
		Enabled:       true,
		FIPSLevel:     FIPSLevel140_3,
		AuditEnabled:  true,
		DefaultCipher: CipherAES256GCM,
	})

	key, err := m.GenerateKey("测试密钥", CipherAES256GCM, 256)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if key.ID == "" {
		t.Error("key ID should be generated")
	}
	if key.Status != KeyStatusActive {
		t.Errorf("expected active status, got %s", key.Status)
	}
	if key.KeySize != 256 {
		t.Errorf("expected 256 bit key, got %d", key.KeySize)
	}
	if key.Version != 1 {
		t.Errorf("expected version 1, got %d", key.Version)
	}
}

func TestRotateKey(t *testing.T) {
	m := NewManager(&Config{
		Enabled:         true,
		FIPSLevel:       FIPSLevel140_3,
		KeyRotationDays: 90,
		AuditEnabled:    true,
	})

	key, _ := m.GenerateKey("轮换测试", CipherAES256GCM, 256)
	newKey, err := m.RotateKey(key.ID)
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}

	if newKey.Version != 2 {
		t.Errorf("expected version 2 after rotation, got %d", newKey.Version)
	}
	if newKey.Status != KeyStatusActive {
		t.Errorf("new key should be active, got %s", newKey.Status)
	}

	// 旧密钥应为退役状态
	keys := m.ListKeys()
	for _, k := range keys {
		if k.Version == 1 {
			if k.Status != KeyStatusRetired {
				t.Errorf("old key should be retired, got %s", k.Status)
			}
		}
	}
}

func TestCreateEncryptedShare(t *testing.T) {
	m := NewManager(&Config{
		Enabled:       true,
		FIPSLevel:     FIPSLevel140_3,
		DefaultCipher: CipherAES256GCM,
		MinTLSVersion: "1.3",
		AuditEnabled:  true,
	})

	key, _ := m.GenerateKey("共享密钥", CipherAES256GCM, 256)

	share := &EncryptedShare{
		Name:     "安全共享",
		Path:     "/secure/docs",
		Protocol: ProtocolHTTPS,
		KeyID:    key.ID,
	}

	if err := m.CreateEncryptedShare(share); err != nil {
		t.Fatalf("CreateEncryptedShare failed: %v", err)
	}
	if share.ID == "" {
		t.Error("share ID should be generated")
	}
	if share.TLSVersion != "1.3" {
		t.Errorf("expected TLS 1.3, got %s", share.TLSVersion)
	}
}

func TestComplianceCheck(t *testing.T) {
	m := NewManager(&Config{
		Enabled:       true,
		FIPSLevel:     FIPSLevel140_3,
		DefaultCipher: CipherAES256GCM,
		MinTLSVersion: "1.3",
		AuditEnabled:  true,
	})

	// 创建合规共享
	key, _ := m.GenerateKey("合规密钥", CipherAES256GCM, 256)
	m.CreateEncryptedShare(&EncryptedShare{
		Name:        "合规共享",
		Path:        "/compliant",
		Protocol:    ProtocolHTTPS,
		KeyID:       key.ID,
		FIPSLevel:   FIPSLevel140_3,
		TLSVersion:  "1.3",
		CipherSuite: CipherAES256GCM,
	})

	report := m.RunComplianceCheck()
	if report.OverallStatus != "compliant" {
		t.Errorf("expected compliant status, got %s", report.OverallStatus)
	}
	if report.TotalShares != 1 {
		t.Errorf("expected 1 share, got %d", report.TotalShares)
	}
	if report.EncryptedShares != 1 {
		t.Errorf("expected 1 encrypted share, got %d", report.EncryptedShares)
	}
}

func TestComplianceCheckWithIssues(t *testing.T) {
	m := NewManager(&Config{
		Enabled:       true,
		FIPSLevel:     FIPSLevel140_3,
		DefaultCipher: CipherAES256GCM,
		MinTLSVersion: "1.3",
		AuditEnabled:  true,
	})

	// 创建不合规共享（旧 TLS 版本）
	m.CreateEncryptedShare(&EncryptedShare{
		Name:        "不安全共享",
		Path:        "/insecure",
		Protocol:    ProtocolHTTPS,
		FIPSLevel:   FIPSLevel140_2, // 不匹配
		TLSVersion:  "1.0",          // 不安全
		CipherSuite: CipherAES256CBC,
	})

	report := m.RunComplianceCheck()
	if report.OverallStatus == "compliant" {
		t.Error("should not be compliant with TLS 1.0")
	}
	if len(report.Issues) == 0 {
		t.Error("expected compliance issues")
	}
}

func TestAuditLog(t *testing.T) {
	m := NewManager(&Config{
		Enabled:       true,
		FIPSLevel:     FIPSLevel140_3,
		AuditEnabled:  true,
		MaxAuditEntries: 1000,
	})

	// 执行一些操作
	m.GenerateKey("审计测试", CipherAES256GCM, 256)
	m.GenerateKey("审计测试2", CipherAES256GCM, 256)

	logs := m.GetAuditLog(10)
	if len(logs) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(logs))
	}

	// 验证审计条目格式
	for _, entry := range logs {
		if entry.ID == "" {
			t.Error("audit entry should have ID")
		}
		if entry.Timestamp.IsZero() {
			t.Error("audit entry should have timestamp")
		}
		if entry.EventType == "" {
			t.Error("audit entry should have event type")
		}
	}
}

func TestListKeys(t *testing.T) {
	m := NewManager(&Config{
		Enabled:       true,
		AuditEnabled:  true,
		DefaultCipher: CipherAES256GCM,
	})

	for i := 0; i < 5; i++ {
		m.GenerateKey("密钥", CipherAES256GCM, 256)
	}

	keys := m.ListKeys()
	if len(keys) != 5 {
		t.Errorf("expected 5 keys, got %d", len(keys))
	}
}

func TestListShares(t *testing.T) {
	m := NewManager(&Config{
		Enabled:       true,
		FIPSLevel:     FIPSLevel140_3,
		DefaultCipher: CipherAES256GCM,
		AuditEnabled:  true,
	})

	for i := 0; i < 3; i++ {
		m.CreateEncryptedShare(&EncryptedShare{
			Name:     "共享",
			Path:     "/share",
			Protocol: ProtocolHTTPS,
		})
	}

	shares := m.ListShares()
	if len(shares) != 3 {
		t.Errorf("expected 3 shares, got %d", len(shares))
	}
}
