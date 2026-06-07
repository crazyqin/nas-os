package acmemanager

import (
	"fmt"
	"sync"
	"time"
)

// ACMEManager ACME证书管理器
type ACMEManager struct {
	mu       sync.RWMutex
	certs    map[string]*Certificate
	accounts map[string]*Account
	config   *ACMEConfig
}

// ACMEConfig ACME配置
type ACMEConfig struct {
	Directory   string `json:"directory"`
	Email       string `json:"email"`
	CAURL       string `json:"ca_url"`
	AutoRenew   bool   `json:"auto_renew"`
	RenewBefore int    `json:"renew_before_days"`
}

// Account ACME账户
type Account struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Directory string    `json:"directory"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Certificate 证书
type Certificate struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	Status    string    `json:"status"`
	Issuer    string    `json:"issuer"`
	Serial    string    `json:"serial"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	CertPEM   string    `json:"cert_pem,omitempty"`
	KeyPEM    string    `json:"key_pem,omitempty"`
	ChainPEM  string    `json:"chain_pem,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	RenewedAt time.Time `json:"renewed_at,omitempty"`
	AutoRenew bool      `json:"auto_renew"`
}

// NewACMEManager 创建ACME管理器
func NewACMEManager(config *ACMEConfig) *ACMEManager {
	if config == nil {
		config = &ACMEConfig{
			Directory:   "https://acme-v02.api.letsencrypt.org/directory",
			AutoRenew:   true,
			RenewBefore: 30,
		}
	}
	return &ACMEManager{
		certs:    make(map[string]*Certificate),
		accounts: make(map[string]*Account),
		config:   config,
	}
}

// CreateAccount 创建账户
func (m *ACMEManager) CreateAccount(email string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	account := &Account{
		ID:        fmt.Sprintf("account_%d", time.Now().UnixNano()),
		Email:     email,
		Directory: m.config.Directory,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	m.accounts[account.ID] = account
	return account, nil
}

// RequestCertificate 请求证书
func (m *ACMEManager) RequestCertificate(domain string) (*Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	// 检查是否已有证书
	for _, cert := range m.certs {
		if cert.Domain == domain && cert.Status == "valid" {
			return cert, nil
		}
	}

	cert := &Certificate{
		ID:        fmt.Sprintf("cert_%d", time.Now().UnixNano()),
		Domain:    domain,
		Status:    "valid",
		Issuer:    "Let's Encrypt",
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(90 * 24 * time.Hour),
		Serial:    fmt.Sprintf("%x", time.Now().UnixNano()),
		CreatedAt: time.Now(),
		AutoRenew: m.config.AutoRenew,
	}

	m.certs[cert.ID] = cert

	return cert, nil
}

// GetCertificate 获取证书
func (m *ACMEManager) GetCertificate(id string) (*Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cert, exists := m.certs[id]
	if !exists {
		return nil, fmt.Errorf("certificate not found: %s", id)
	}
	return cert, nil
}

// ListCertificates 列出所有证书
func (m *ACMEManager) ListCertificates() []*Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	certs := make([]*Certificate, 0, len(m.certs))
	for _, cert := range m.certs {
		certs = append(certs, cert)
	}
	return certs
}

// RevokeCertificate 吊销证书
func (m *ACMEManager) RevokeCertificate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cert, exists := m.certs[id]
	if !exists {
		return fmt.Errorf("certificate not found: %s", id)
	}

	cert.Status = "revoked"
	return nil
}

// RenewCertificate 续期证书
func (m *ACMEManager) RenewCertificate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cert, exists := m.certs[id]
	if !exists {
		return fmt.Errorf("certificate not found: %s", id)
	}

	// 模拟续期过程
	cert.Status = "renewing"
	go func() {
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		cert.Status = "valid"
		cert.NotBefore = time.Now()
		cert.NotAfter = time.Now().Add(90 * 24 * time.Hour)
		cert.RenewedAt = time.Now()
	}()

	return nil
}

// CheckExpiring 检查即将过期的证书
func (m *ACMEManager) CheckExpiring(days int) []*Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expiring []*Certificate
	threshold := time.Now().AddDate(0, 0, days)

	for _, cert := range m.certs {
		if cert.Status == "valid" && cert.NotAfter.Before(threshold) {
			expiring = append(expiring, cert)
		}
	}

	return expiring
}

// AutoRenewCertificates 自动续期证书
func (m *ACMEManager) AutoRenewCertificates() {
	expiring := m.CheckExpiring(m.config.RenewBefore)
	for _, cert := range expiring {
		m.RenewCertificate(cert.ID)
	}
}

// GetCertificateByDomain 根据域名获取证书
func (m *ACMEManager) GetCertificateByDomain(domain string) (*Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cert := range m.certs {
		if cert.Domain == domain && cert.Status == "valid" {
			return cert, nil
		}
	}

	return nil, fmt.Errorf("no valid certificate found for domain: %s", domain)
}

// GetAccount 获取账户
func (m *ACMEManager) GetAccount(id string) (*Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	account, exists := m.accounts[id]
	if !exists {
		return nil, fmt.Errorf("account not found: %s", id)
	}
	return account, nil
}

// ListAccounts 列出所有账户
func (m *ACMEManager) ListAccounts() []*Account {
	m.mu.RLock()
	defer m.mu.RUnlock()

	accounts := make([]*Account, 0, len(m.accounts))
	for _, account := range m.accounts {
		accounts = append(accounts, account)
	}
	return accounts
}

// GetStats 获取统计信息
func (m *ACMEManager) GetStats() *ACMEStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ACMEStats{
		TotalCerts:    len(m.certs),
		TotalAccounts: len(m.accounts),
	}

	for _, cert := range m.certs {
		switch cert.Status {
		case "valid":
			stats.ValidCerts++
		case "pending":
			stats.PendingCerts++
		case "expired":
			stats.ExpiredCerts++
		case "revoked":
			stats.RevokedCerts++
		}
	}

	return stats
}

// ACMEStats ACME统计
type ACMEStats struct {
	TotalCerts    int `json:"total_certs"`
	ValidCerts    int `json:"valid_certs"`
	PendingCerts  int `json:"pending_certs"`
	ExpiredCerts  int `json:"expired_certs"`
	RevokedCerts  int `json:"revoked_certs"`
	TotalAccounts int `json:"total_accounts"`
}
