// Package nvmeof provides NVMe/TCP TLS transport encryption support.
// Implements mutual TLS (mTLS) for NVMe-oF connections.
package nvmeof

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// TLSConfig holds TLS configuration for NVMe-oF.
type TLSConfig struct {
	Enabled    bool   `json:"enabled"`
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
	CAFile     string `json:"ca_file"`
	RequireMTLS bool  `json:"require_mtls"`
	MinVersion uint16 `json:"min_version"`
}

// TLSManager manages TLS certificates and connections for NVMe-oF.
type TLSManager struct {
	config   *TLSConfig
	logger   *zap.Logger
	tlsCfg   *tls.Config
	certPath string
}

// NewTLSManager creates a new TLS manager for NVMe-oF.
func NewTLSManager(config *TLSConfig, dataDir string, logger *zap.Logger) (*TLSManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	tm := &TLSManager{
		config:   config,
		logger:   logger,
		certPath: filepath.Join(dataDir, "tls"),
	}

	if config.Enabled {
		if err := tm.loadCertificates(); err != nil {
			return nil, fmt.Errorf("failed to load TLS certificates: %w", err)
		}
	}

	return tm, nil
}

// EnsureCertificates generates self-signed certificates if none exist.
func (tm *TLSManager) EnsureCertificates() error {
	if tm.config.CertFile != "" && tm.config.KeyFile != "" {
		// Check if files exist
		if _, err := os.Stat(tm.config.CertFile); err == nil {
			return nil // Certificates already exist
		}
	}

	// Generate self-signed certificate for NVMe-oF
	if err := tm.generateSelfSignedCert(); err != nil {
		return fmt.Errorf("failed to generate self-signed certificate: %w", err)
	}

	tm.logger.Info("Generated self-signed TLS certificate for NVMe-oF",
		zap.String("cert", tm.config.CertFile),
		zap.String("key", tm.config.KeyFile))

	return nil
}

// GetTLSConfig returns the TLS configuration for listeners.
func (tm *TLSManager) GetTLSConfig() *tls.Config {
	if !tm.config.Enabled || tm.tlsCfg == nil {
		return nil
	}
	return tm.tlsCfg
}

// WrapListener wraps a net.Listener with TLS.
func (tm *TLSManager) WrapListener(listener net.Listener) (net.Listener, error) {
	if !tm.config.Enabled || tm.tlsCfg == nil {
		return listener, nil
	}
	return tls.NewListener(listener, tm.tlsCfg), nil
}

// loadCertificates loads TLS certificates from disk.
func (tm *TLSManager) loadCertificates() error {
	cert, err := tls.LoadX509KeyPair(tm.config.CertFile, tm.config.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tm.config.MinVersion,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// Load CA for mTLS if required
	if tm.config.RequireMTLS && tm.config.CAFile != "" {
		caCert, err := os.ReadFile(tm.config.CAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA file: %w", err)
		}

		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate")
		}

		tlsCfg.ClientCAs = caPool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if tm.config.MinVersion == 0 {
		tlsCfg.MinVersion = tls.VersionTLS13
	}

	tm.tlsCfg = tlsCfg
	return nil
}

// generateSelfSignedCert generates a self-signed ECDSA certificate.
func (tm *TLSManager) generateSelfSignedCert() error {
	// Generate ECDSA P-256 key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NAS-OS"},
			CommonName:   "NAS-OS NVMe-oF",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Add SANs
	hostname, _ := os.Hostname()
	template.DNSNames = []string{hostname, "localhost"}
	template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(tm.certPath, 0700); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Write certificate
	certFile := filepath.Join(tm.certPath, "nvmeof.crt")
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key
	keyFile := filepath.Join(tm.certPath, "nvmeof.key")
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	tm.config.CertFile = certFile
	tm.config.KeyFile = keyFile
	tm.config.Enabled = true

	return tm.loadCertificates()
}
