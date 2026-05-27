package acme

import (
	"time"
)

// Certificate represents an SSL/TLS certificate
type Certificate struct {
	ID         string    `json:"id"`
	Domain     string    `json:"domain"`
	Issuer     string    `json:"issuer"`
	NotBefore  time.Time `json:"not_before"`
	NotAfter   time.Time `json:"not_after"`
	Status     string    `json:"status"` // active, expired, revoked, pending
	AutoRenew  bool      `json:"auto_renew"`
	CertPath   string    `json:"cert_path"`
	KeyPath    string    `json:"key_path"`
	CreatedAt  time.Time `json:"created_at"`
	RenewedAt  time.Time `json:"renewed_at,omitempty"`
}

// ACMEConfig represents ACME configuration
type ACMEConfig struct {
	Email        string `json:"email"`
	CAProvider   string `json:"ca_provider"`   // letsencrypt, zerossl, googledns
	DNSProvider  string `json:"dns_provider"`  // cloudflare, route53, aliyun, etc.
	RenewalDays  int    `json:"renewal_days"`   // Days before expiry to renew
}

// CreateCertRequest represents a request to create a new certificate
type CreateCertRequest struct {
	Domain    string `json:"domain" validate:"required"`
	AutoRenew bool   `json:"auto_renew,omitempty"`
}

// RenewCertRequest represents a request to renew a certificate
type RenewCertRequest struct {
	ID string `json:"id" validate:"required"`
}

// CertStats represents aggregated certificate statistics
type CertStats struct {
	TotalCerts      int `json:"total_certs"`
	ActiveCerts     int `json:"active_certs"`
	ExpiringSoon    int `json:"expiring_soon"` // Within 30 days
	AutoRenewEnabled int `json:"auto_renew_enabled"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a generic API success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}
