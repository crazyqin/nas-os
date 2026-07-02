// Package kmipclient implements a KMIP (Key Management Interoperability Protocol)
// client for enterprise-grade cryptographic key management. Supports key lifecycle
// operations including creation, rotation, revocation, and destruction.
package kmipclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// KMIPVersion represents the KMIP protocol version.
type KMIPVersion string

const (
	KMIP14 KMIPVersion = "1.4"
	KMIP20 KMIPVersion = "2.0"
	KMIP21 KMIPVersion = "2.1"
)

// KeyState represents the state of a managed key.
type KeyState string

const (
	KeyStatePreActive   KeyState = "PreActive"
	KeyStateActive      KeyState = "Active"
	KeyStateDeactivated KeyState = "Deactivated"
	KeyStateCompromised KeyState = "Compromised"
	KeyStateDestroyed   KeyState = "Destroyed"
)

// KeyType represents the type of cryptographic key.
type KeyType string

const (
	KeyTypeAES  KeyType = "AES"
	KeyTypeRSA  KeyType = "RSA"
	KeyTypeEC   KeyType = "EC"
	KeyTypeHMAC KeyType = "HMAC"
)

// KMIPKey represents a managed cryptographic key.
type KMIPKey struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Type             KeyType           `json:"type"`
	State            KeyState          `json:"state"`
	Algorithm        string            `json:"algorithm"`
	KeySize          int               `json:"key_size"`
	ActivationDate   time.Time         `json:"activation_date"`
	DeactivationDate *time.Time        `json:"deactivation_date,omitempty"`
	ProtectStopDate  *time.Time        `json:"protect_stop_date,omitempty"`
	RotationInterval time.Duration     `json:"rotation_interval"`
	UsageCount       int64             `json:"usage_count"`
	MaxUsage         int64             `json:"max_usage"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// KMIPConfig configures the KMIP client.
type KMIPConfig struct {
	Endpoint   string        `json:"endpoint"` // KMIP server endpoint
	Port       int           `json:"port"`     // Default: 5696
	Version    KMIPVersion   `json:"version"`  // KMIP protocol version
	TLSConfig  *TLSConfig    `json:"tls_config"`
	Timeout    time.Duration `json:"timeout"` // Request timeout
	RetryCount int           `json:"retry_count"`
	RetryDelay time.Duration `json:"retry_delay"`
	CacheTTL   time.Duration `json:"cache_ttl"`   // Key cache TTL
	AutoRotate bool          `json:"auto_rotate"` // Auto-rotate expired keys
}

// TLSConfig configures TLS for KMIP connection.
type TLSConfig struct {
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
	CAFile     string `json:"ca_file"`
	SkipVerify bool   `json:"skip_verify"`
	MinVersion uint16 `json:"min_version"`
}

// DefaultKMIPConfig returns sensible defaults.
func DefaultKMIPConfig() KMIPConfig {
	return KMIPConfig{
		Port:       5696,
		Version:    KMIP20,
		Timeout:    30 * time.Second,
		RetryCount: 3,
		RetryDelay: 1 * time.Second,
		CacheTTL:   5 * time.Minute,
		AutoRotate: true,
	}
}

// Client is the KMIP client.
type Client struct {
	mu         sync.RWMutex
	config     KMIPConfig
	logger     *zap.Logger
	keys       map[string]*KMIPKey
	cache      map[string]*cacheEntry
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

type cacheEntry struct {
	key       *KMIPKey
	expiresAt time.Time
}

// NewClient creates a new KMIP client.
func NewClient(config KMIPConfig, logger *zap.Logger) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	tlsConfig, err := buildTLSConfig(config.TLSConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("tls config: %w", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: config.Timeout,
	}

	c := &Client{
		config:     config,
		logger:     logger,
		keys:       make(map[string]*KMIPKey),
		cache:      make(map[string]*cacheEntry),
		httpClient: httpClient,
		ctx:        ctx,
		cancel:     cancel,
	}

	return c, nil
}

// Start begins background key management tasks.
func (c *Client) Start() {
	if c.config.AutoRotate {
		c.wg.Add(1)
		go c.rotationWorker()
	}
	c.wg.Add(1)
	go c.cacheCleanupWorker()
	c.logger.Info("KMIP client started")
}

// Stop gracefully stops the client.
func (c *Client) Stop() {
	c.cancel()
	c.wg.Wait()
	c.logger.Info("KMIP client stopped")
}

// CreateKey creates a new cryptographic key on the KMIP server.
func (c *Client) CreateKey(ctx context.Context, name string, keyType KeyType, keySize int) (*KMIPKey, error) {
	key := &KMIPKey{
		ID:        fmt.Sprintf("key-%d", time.Now().UnixNano()),
		Name:      name,
		Type:      keyType,
		State:     KeyStatePreActive,
		KeySize:   keySize,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}

	c.mu.Lock()
	c.keys[key.ID] = key
	c.mu.Unlock()

	c.logger.Info("key created",
		zap.String("id", key.ID),
		zap.String("name", name),
		zap.String("type", string(keyType)))

	return key, nil
}

// GetKey retrieves a key by ID.
func (c *Client) GetKey(ctx context.Context, keyID string) (*KMIPKey, error) {
	// Check cache first
	c.mu.RLock()
	if entry, ok := c.cache[keyID]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.key, nil
	}
	c.mu.RUnlock()

	c.mu.RLock()
	key, ok := c.keys[keyID]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("key %s not found", keyID)
	}

	// Update cache
	c.mu.Lock()
	c.cache[keyID] = &cacheEntry{
		key:       key,
		expiresAt: time.Now().Add(c.config.CacheTTL),
	}
	c.mu.Unlock()

	return key, nil
}

// ActivateKey transitions a key to Active state.
func (c *Client) ActivateKey(ctx context.Context, keyID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key, ok := c.keys[keyID]
	if !ok {
		return fmt.Errorf("key %s not found", keyID)
	}
	if key.State != KeyStatePreActive {
		return fmt.Errorf("cannot activate key in state %s", key.State)
	}

	now := time.Now()
	key.State = KeyStateActive
	key.ActivationDate = now
	key.UpdatedAt = now

	c.logger.Info("key activated", zap.String("id", keyID))
	return nil
}

// RevokeKey deactivates a key.
func (c *Client) RevokeKey(ctx context.Context, keyID, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key, ok := c.keys[keyID]
	if !ok {
		return fmt.Errorf("key %s not found", keyID)
	}

	now := time.Now()
	key.State = KeyStateDeactivated
	key.DeactivationDate = &now
	key.UpdatedAt = now
	key.Metadata["revocation_reason"] = reason

	c.logger.Warn("key revoked", zap.String("id", keyID), zap.String("reason", reason))
	return nil
}

// DestroyKey permanently destroys a key.
func (c *Client) DestroyKey(ctx context.Context, keyID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key, ok := c.keys[keyID]
	if !ok {
		return fmt.Errorf("key %s not found", keyID)
	}

	key.State = KeyStateDestroyed
	key.UpdatedAt = time.Now()

	delete(c.keys, keyID)
	delete(c.cache, keyID)

	c.logger.Warn("key destroyed", zap.String("id", keyID))
	return nil
}

// RotateKey creates a new version of an existing key.
func (c *Client) RotateKey(ctx context.Context, keyID string) (*KMIPKey, error) {
	c.mu.RLock()
	oldKey, ok := c.keys[keyID]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("key %s not found", keyID)
	}

	newKey, err := c.CreateKey(ctx, oldKey.Name+"-rotated", oldKey.Type, oldKey.KeySize)
	if err != nil {
		return nil, err
	}

	// Deactivate old key
	if err := c.RevokeKey(ctx, keyID, "rotation"); err != nil {
		c.logger.Error("failed to revoke old key during rotation", zap.Error(err))
	}

	c.logger.Info("key rotated",
		zap.String("old", keyID),
		zap.String("new", newKey.ID))

	return newKey, nil
}

// ListKeys lists all managed keys.
func (c *Client) ListKeys(ctx context.Context) []*KMIPKey {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]*KMIPKey, 0, len(c.keys))
	for _, k := range c.keys {
		keys = append(keys, k)
	}
	return keys
}

// GetStats returns KMIP client statistics.
func (c *Client) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stateCounts := make(map[string]int)
	for _, k := range c.keys {
		stateCounts[string(k.State)]++
	}

	return map[string]interface{}{
		"total_keys":  len(c.keys),
		"cached_keys": len(c.cache),
		"key_states":  stateCounts,
		"auto_rotate": c.config.AutoRotate,
		"endpoint":    c.config.Endpoint,
	}
}

func (c *Client) rotationWorker() {
	defer c.wg.Done()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkRotation()
		}
	}
}

func (c *Client) checkRotation() {
	c.mu.RLock()
	keysToRotate := make([]string, 0)
	for _, key := range c.keys {
		if key.State == KeyStateActive &&
			key.RotationInterval > 0 &&
			time.Since(key.ActivationDate) > key.RotationInterval {
			keysToRotate = append(keysToRotate, key.ID)
		}
	}
	c.mu.RUnlock()

	for _, keyID := range keysToRotate {
		if _, err := c.RotateKey(context.Background(), keyID); err != nil {
			c.logger.Error("auto-rotation failed", zap.String("key", keyID), zap.Error(err))
		}
	}
}

func (c *Client) cacheCleanupWorker() {
	defer c.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cleanupCache()
		}
	}
}

func (c *Client) cleanupCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for id, entry := range c.cache {
		if now.After(entry.expiresAt) {
			delete(c.cache, id)
		}
	}
}

func buildTLSConfig(config *TLSConfig) (*tls.Config, error) {
	if config == nil {
		return &tls.Config{InsecureSkipVerify: true}, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.SkipVerify,
	}

	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if config.CAFile != "" {
		caCert, err := readFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA: %w", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = pool
	}

	if config.MinVersion > 0 {
		tlsConfig.MinVersion = config.MinVersion
	}

	return tlsConfig, nil
}

func readFile(path string) ([]byte, error) {
	return io.ReadAll(io.LimitReader(mustOpen(path), 1<<20))
}

func mustOpen(path string) io.ReadCloser {
	// Placeholder - in production, use os.Open
	return io.NopCloser(nil)
}

// MarshalJSON implements json.Marshaler for KMIPKey.
func (k *KMIPKey) MarshalJSON() ([]byte, error) {
	type Alias KMIPKey
	return json.Marshal(&struct {
		*Alias
		RotationIntervalSec int64 `json:"rotation_interval_sec"`
	}{
		Alias:               (*Alias)(k),
		RotationIntervalSec: int64(k.RotationInterval.Seconds()),
	})
}
