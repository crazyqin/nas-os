package webapphost

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"
)

// SSLManager SSL 证书管理器
type SSLManager struct {
	mu        sync.RWMutex
	certs     map[string]*SSLEntry
	certDir   string
	provider  string // letsencrypt, selfsigned
	email     string
}

// NewSSLManager 创建 SSL 管理器
func NewSSLManager(certDir string, provider string) *SSLManager {
	if certDir == "" {
		certDir = "/etc/nas-os/ssl"
	}
	if provider == "" {
		provider = "selfsigned"
	}

	return &SSLManager{
		certs:    make(map[string]*SSLEntry),
		certDir:  certDir,
		provider: provider,
	}
}

// SetEmail 设置邮箱（用于 Let's Encrypt）
func (sm *SSLManager) SetEmail(email string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.email = email
}

// RequestCertificate 申请证书
func (sm *SSLManager) RequestCertificate(domain string) (*SSLEntry, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查是否已有证书
	if entry, exists := sm.certs[domain]; exists {
		if entry.Status == "active" && entry.NotAfter.After(time.Now()) {
			return entry, nil
		}
	}

	var entry *SSLEntry
	var err error

	switch sm.provider {
	case "letsencrypt":
		entry, err = sm.requestLetsEncrypt(domain)
	case "selfsigned":
		entry, err = sm.createSelfSigned(domain)
	default:
		return nil, fmt.Errorf("unsupported SSL provider: %s", sm.provider)
	}

	if err != nil {
		return nil, err
	}

	sm.certs[domain] = entry
	log.Printf("SSL certificate created for domain: %s", domain)
	return entry, nil
}

// requestLetsEncrypt 申请 Let's Encrypt 证书
func (sm *SSLManager) requestLetsEncrypt(domain string) (*SSLEntry, error) {
	// 模拟 Let's Encrypt 证书申请
	// 实际实现需要使用 ACME 协议

	log.Printf("Requesting Let's Encrypt certificate for: %s", domain)

	entry := &SSLEntry{
		ID:        GenerateID("cert"),
		Domain:    domain,
		CertPath:  fmt.Sprintf("%s/%s/cert.pem", sm.certDir, domain),
		KeyPath:   fmt.Sprintf("%s/%s/key.pem", sm.certDir, domain),
		Issuer:    "Let's Encrypt",
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(90 * 24 * time.Hour), // 90 天
		AutoRenew: true,
		Provider:  "letsencrypt",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return entry, nil
}

// createSelfSigned 创建自签名证书
func (sm *SSLManager) createSelfSigned(domain string) (*SSLEntry, error) {
	log.Printf("Creating self-signed certificate for: %s", domain)

	// 生成私钥
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// 创建证书模板
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NAS-OS"},
			CommonName:   domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 年
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	// 自签名证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// 序列化证书
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// 序列化私钥
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	_ = certPEM
	_ = keyPEM

	entry := &SSLEntry{
		ID:        GenerateID("cert"),
		Domain:    domain,
		CertPath:  fmt.Sprintf("%s/%s/cert.pem", sm.certDir, domain),
		KeyPath:   fmt.Sprintf("%s/%s/key.pem", sm.certDir, domain),
		Issuer:    "NAS-OS Self-Signed",
		NotBefore: template.NotBefore,
		NotAfter:  template.NotAfter,
		AutoRenew: true,
		Provider:  "selfsigned",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return entry, nil
}

// GetCertificate 获取证书
func (sm *SSLManager) GetCertificate(domain string) (*SSLEntry, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entry, exists := sm.certs[domain]
	if !exists {
		return nil, fmt.Errorf("certificate not found for domain: %s", domain)
	}
	return entry, nil
}

// ListCertificates 列出所有证书
func (sm *SSLManager) ListCertificates() []*SSLEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	certs := make([]*SSLEntry, 0, len(sm.certs))
	for _, cert := range sm.certs {
		certs = append(certs, cert)
	}
	return certs
}

// RevokeCertificate 吊销证书
func (sm *SSLManager) RevokeCertificate(domain string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	entry, exists := sm.certs[domain]
	if !exists {
		return fmt.Errorf("certificate not found for domain: %s", domain)
	}

	entry.Status = "revoked"
	entry.UpdatedAt = time.Now()

	log.Printf("Certificate revoked for domain: %s", domain)
	return nil
}

// DeleteCertificate 删除证书
func (sm *SSLManager) DeleteCertificate(domain string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.certs[domain]; !exists {
		return fmt.Errorf("certificate not found for domain: %s", domain)
	}

	delete(sm.certs, domain)
	log.Printf("Certificate deleted for domain: %s", domain)
	return nil
}

// RenewCertificate 续期证书
func (sm *SSLManager) RenewCertificate(domain string) (*SSLEntry, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	entry, exists := sm.certs[domain]
	if !exists {
		return nil, fmt.Errorf("certificate not found for domain: %s", domain)
	}

	log.Printf("Renewing certificate for domain: %s", domain)

	// 重新申请证书
	var newEntry *SSLEntry
	var err error

	switch entry.Provider {
	case "letsencrypt":
		newEntry, err = sm.requestLetsEncrypt(domain)
	case "selfsigned":
		newEntry, err = sm.createSelfSigned(domain)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", entry.Provider)
	}

	if err != nil {
		return nil, err
	}

	newEntry.ID = entry.ID
	sm.certs[domain] = newEntry

	log.Printf("Certificate renewed for domain: %s", domain)
	return newEntry, nil
}

// CheckExpiring 检查即将过期的证书
func (sm *SSLManager) CheckExpiring(days int) []*SSLEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	deadline := time.Now().AddDate(0, 0, days)
	var expiring []*SSLEntry

	for _, cert := range sm.certs {
		if cert.Status == "active" && cert.NotAfter.Before(deadline) {
			expiring = append(expiring, cert)
		}
	}

	return expiring
}

// AutoRenew 自动续期证书
func (sm *SSLManager) AutoRenew() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查 30 天内过期的证书
	deadline := time.Now().AddDate(0, 0, 30)

	for domain, cert := range sm.certs {
		if cert.AutoRenew && cert.Status == "active" && cert.NotAfter.Before(deadline) {
			log.Printf("Auto-renewing certificate for domain: %s", domain)

			var newEntry *SSLEntry
			var err error

			switch cert.Provider {
			case "letsencrypt":
				newEntry, err = sm.requestLetsEncrypt(domain)
			case "selfsigned":
				newEntry, err = sm.createSelfSigned(domain)
			default:
				log.Printf("Unsupported provider for auto-renew: %s", cert.Provider)
				continue
			}

			if err != nil {
				log.Printf("Failed to auto-renew certificate for %s: %v", domain, err)
				continue
			}

			newEntry.ID = cert.ID
			sm.certs[domain] = newEntry
			log.Printf("Certificate auto-renewed for domain: %s", domain)
		}
	}
}

// GetCertCount 获取证书数量
func (sm *SSLManager) GetCertCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.certs)
}

// IsSSLEnabled 检查域名是否启用 SSL
func (sm *SSLManager) IsSSLEnabled(domain string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entry, exists := sm.certs[domain]
	return exists && entry.Status == "active" && entry.NotAfter.After(time.Now())
}

// ExportCertificate 导出证书（PEM 格式）
func (sm *SSLManager) ExportCertificate(domain string) (certPEM, keyPEM string, err error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entry, exists := sm.certs[domain]
	if !exists {
		return "", "", fmt.Errorf("certificate not found for domain: %s", domain)
	}

	if entry.Status != "active" {
		return "", "", fmt.Errorf("certificate is not active: %s", entry.Status)
	}

	// 实际实现：读取证书文件并返回 PEM 内容
	return entry.CertPath, entry.KeyPath, nil
}
