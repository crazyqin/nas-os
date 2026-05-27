package acmemanager

import (
	"testing"
)

func TestNewACMEManager(t *testing.T) {
	manager := NewACMEManager(nil)
	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config == nil {
		t.Fatal("Expected default config to be set")
	}

	if manager.config.Directory != "https://acme-v02.api.letsencrypt.org/directory" {
		t.Errorf("Expected default directory, got %s", manager.config.Directory)
	}
}

func TestCreateAccount(t *testing.T) {
	manager := NewACMEManager(nil)

	account, err := manager.CreateAccount("test@example.com")
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	if account.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", account.Email)
	}

	if account.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", account.Status)
	}

	// 测试空邮箱
	_, err = manager.CreateAccount("")
	if err == nil {
		t.Error("Expected error for empty email")
	}
}

func TestRequestCertificate(t *testing.T) {
	manager := NewACMEManager(nil)

	cert, err := manager.RequestCertificate("example.com")
	if err != nil {
		t.Fatalf("Failed to request certificate: %v", err)
	}

	if cert.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", cert.Domain)
	}

	if cert.Status != "valid" {
		t.Errorf("Expected status 'valid', got '%s'", cert.Status)
	}

	// 测试空域名
	_, err = manager.RequestCertificate("")
	if err == nil {
		t.Error("Expected error for empty domain")
	}
}

func TestGetCertificate(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建证书
	cert, _ := manager.RequestCertificate("example.com")

	// 获取证书
	fetchedCert, err := manager.GetCertificate(cert.ID)
	if err != nil {
		t.Fatalf("Failed to get certificate: %v", err)
	}

	if fetchedCert.ID != cert.ID {
		t.Errorf("Expected cert ID '%s', got '%s'", cert.ID, fetchedCert.ID)
	}

	// 测试获取不存在的证书
	_, err = manager.GetCertificate("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent certificate")
	}
}

func TestListCertificates(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建一些证书
	manager.RequestCertificate("example1.com")
	manager.RequestCertificate("example2.com")

	certs := manager.ListCertificates()
	if len(certs) != 2 {
		t.Errorf("Expected 2 certificates, got %d", len(certs))
	}
}

func TestRevokeCertificate(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建证书
	cert, _ := manager.RequestCertificate("example.com")

	// 吊销证书
	err := manager.RevokeCertificate(cert.ID)
	if err != nil {
		t.Fatalf("Failed to revoke certificate: %v", err)
	}

	// 验证吊销
	fetchedCert, _ := manager.GetCertificate(cert.ID)
	if fetchedCert.Status != "revoked" {
		t.Errorf("Expected status 'revoked', got '%s'", fetchedCert.Status)
	}
}

func TestRenewCertificate(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建证书
	cert, _ := manager.RequestCertificate("example.com")

	// 续期证书
	err := manager.RenewCertificate(cert.ID)
	if err != nil {
		t.Fatalf("Failed to renew certificate: %v", err)
	}

	// 测试续期不存在的证书
	err = manager.RenewCertificate("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent certificate")
	}
}

func TestCheckExpiring(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建证书
	manager.RequestCertificate("example.com")

	// 检查即将过期的证书
	certs := manager.CheckExpiring(30)
	// 新创建的证书不应该在30天内过期
	if len(certs) != 0 {
		t.Errorf("Expected 0 expiring certificates, got %d", len(certs))
	}
}

func TestGetCertificateByDomain(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建证书
	manager.RequestCertificate("example.com")

	// 根据域名获取证书
	cert, err := manager.GetCertificateByDomain("example.com")
	if err != nil {
		t.Fatalf("Failed to get certificate by domain: %v", err)
	}

	if cert.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", cert.Domain)
	}

	// 测试不存在的域名
	_, err = manager.GetCertificateByDomain("nonexistent.com")
	if err == nil {
		t.Error("Expected error for nonexistent domain")
	}
}

func TestGetAccount(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建账户
	account, _ := manager.CreateAccount("test@example.com")

	// 获取账户
	fetchedAccount, err := manager.GetAccount(account.ID)
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}

	if fetchedAccount.ID != account.ID {
		t.Errorf("Expected account ID '%s', got '%s'", account.ID, fetchedAccount.ID)
	}

	// 测试获取不存在的账户
	_, err = manager.GetAccount("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent account")
	}
}

func TestListAccounts(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建一些账户
	manager.CreateAccount("test1@example.com")
	manager.CreateAccount("test2@example.com")

	accounts := manager.ListAccounts()
	if len(accounts) != 2 {
		t.Errorf("Expected 2 accounts, got %d", len(accounts))
	}
}

func TestGetStats(t *testing.T) {
	manager := NewACMEManager(nil)

	// 创建一些证书和账户
	manager.RequestCertificate("example1.com")
	manager.RequestCertificate("example2.com")
	manager.CreateAccount("test@example.com")

	stats := manager.GetStats()
	if stats.TotalCerts != 2 {
		t.Errorf("Expected 2 total certs, got %d", stats.TotalCerts)
	}

	if stats.TotalAccounts != 1 {
		t.Errorf("Expected 1 total accounts, got %d", stats.TotalAccounts)
	}
}
