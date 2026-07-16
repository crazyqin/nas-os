package acme

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages ACME certificates.
type Manager struct {
	mu     sync.RWMutex
	certs  map[string]Certificate
	config ACMEConfig
}

// NewManager creates a new ACME manager with mock data.
func NewManager() *Manager {
	m := &Manager{
		certs: make(map[string]Certificate),
		config: ACMEConfig{
			Email:       "admin@example.com",
			CAProvider:  "letsencrypt",
			DNSProvider: "cloudflare",
			RenewalDays: 30,
		},
	}

	// Add some mock certificates
	m.addMockCerts()

	return m
}

func (m *Manager) addMockCerts() {
	mockCerts := []struct {
		domain          string
		daysUntilExpiry int
	}{
		{"app.example.com", 90},
		{"api.example.com", 25}, // Expiring soon
		{"admin.example.com", 180},
	}

	for _, mc := range mockCerts {
		now := time.Now()
		cert := Certificate{
			ID:        uuid.New().String(),
			Domain:    mc.domain,
			Issuer:    "Let's Encrypt",
			NotBefore: now.Add(-90 * 24 * time.Hour),
			NotAfter:  now.Add(time.Duration(mc.daysUntilExpiry) * 24 * time.Hour),
			Status:    "active",
			AutoRenew: true,
			CertPath:  "/etc/ssl/certs/" + mc.domain + ".pem",
			KeyPath:   "/etc/ssl/private/" + mc.domain + ".key",
			CreatedAt: now.Add(-90 * 24 * time.Hour),
		}

		if mc.daysUntilExpiry <= 0 {
			cert.Status = "expired"
		}

		m.certs[cert.ID] = cert
	}
}

// RequestCertificate requests a new certificate for a domain.
func (m *Manager) RequestCertificate(domain string) (*Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for existing certificate
	for _, c := range m.certs {
		if c.Domain == domain && c.Status != "expired" && c.Status != "revoked" {
			return nil, fmt.Errorf("certificate already exists for domain '%s'", domain)
		}
	}

	now := time.Now()
	cert := Certificate{
		ID:        uuid.New().String(),
		Domain:    domain,
		Issuer:    m.getIssuer(),
		NotBefore: now,
		NotAfter:  now.Add(90 * 24 * time.Hour), // 90 days
		Status:    "active",
		AutoRenew: true,
		CertPath:  "/etc/ssl/certs/" + domain + ".pem",
		KeyPath:   "/etc/ssl/private/" + domain + ".key",
		CreatedAt: now,
	}

	m.certs[cert.ID] = cert
	return &cert, nil
}

// RenewCertificate renews an existing certificate.
func (m *Manager) RenewCertificate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cert, ok := m.certs[id]
	if !ok {
		return fmt.Errorf("certificate not found: %s", id)
	}

	if cert.Status == "revoked" {
		return fmt.Errorf("cannot renew revoked certificate")
	}

	now := time.Now()
	cert.NotBefore = now
	cert.NotAfter = now.Add(90 * 24 * time.Hour)
	cert.Status = "active"
	cert.RenewedAt = now

	m.certs[id] = cert
	return nil
}

// RevokeCertificate revokes a certificate.
func (m *Manager) RevokeCertificate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cert, ok := m.certs[id]
	if !ok {
		return fmt.Errorf("certificate not found: %s", id)
	}

	if cert.Status == "revoked" {
		return fmt.Errorf("certificate already revoked")
	}

	cert.Status = "revoked"
	m.certs[id] = cert

	return nil
}

// ListCertificates returns all certificates.
func (m *Manager) ListCertificates() []Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	certs := make([]Certificate, 0, len(m.certs))
	for _, c := range m.certs {
		certs = append(certs, c)
	}
	return certs
}

// GetCertByDomain returns a certificate by domain.
func (m *Manager) GetCertByDomain(domain string) (*Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, c := range m.certs {
		if c.Domain == domain && c.Status != "revoked" {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("certificate not found for domain: %s", domain)
}

// CheckExpiry returns certificates expiring within 30 days.
func (m *Manager) CheckExpiry() []Certificate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expiring []Certificate
	threshold := time.Now().Add(30 * 24 * time.Hour)

	for _, c := range m.certs {
		if c.Status == "active" && c.NotAfter.Before(threshold) {
			expiring = append(expiring, c)
		}
	}
	return expiring
}

// AutoRenew automatically renews certificates that are expiring soon.
func (m *Manager) AutoRenew() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	threshold := time.Now().Add(time.Duration(m.config.RenewalDays) * 24 * time.Hour)
	now := time.Now()

	for id, c := range m.certs {
		if c.AutoRenew && c.Status == "active" && c.NotAfter.Before(threshold) {
			c.NotBefore = now
			c.NotAfter = now.Add(90 * 24 * time.Hour)
			c.RenewedAt = now
			m.certs[id] = c
		}
	}

	return nil
}

// ConfigureDNS configures the DNS provider for ACME challenges.
func (m *Manager) ConfigureDNS(provider, credentials string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	supportedProviders := map[string]bool{
		"cloudflare": true,
		"route53":    true,
		"aliyun":     true,
		"godaddy":    true,
	}

	if !supportedProviders[provider] {
		return fmt.Errorf("unsupported DNS provider: %s", provider)
	}

	m.config.DNSProvider = provider
	return nil
}

// GetConfig returns the current ACME configuration.
func (m *Manager) GetConfig() ACMEConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig updates the ACME configuration.
func (m *Manager) UpdateConfig(config ACMEConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Email == "" {
		return fmt.Errorf("email is required")
	}

	validProviders := map[string]bool{
		"letsencrypt": true,
		"zerossl":     true,
		"googledns":   true,
	}

	if !validProviders[config.CAProvider] {
		return fmt.Errorf("invalid CA provider: %s", config.CAProvider)
	}

	if config.RenewalDays < 1 || config.RenewalDays > 90 {
		return fmt.Errorf("renewal_days must be between 1 and 90")
	}

	m.config = config
	return nil
}

func (m *Manager) getIssuer() string {
	switch m.config.CAProvider {
	case "zerossl":
		return "ZeroSSL"
	case "googledns":
		return "Google Trust Services"
	default:
		return "Let's Encrypt"
	}
}
