// Package security provides KMIP key management protocol support.
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// KMIPObjectType represents KMIP object types.
type KMIPObjectType string

const (
	KMIPTypeSymmetricKey KMIPObjectType = "symmetric_key"
	KMIPTypePublicKey    KMIPObjectType = "public_key"
	KMIPTypePrivateKey   KMIPObjectType = "private_key"
	KMIPTypeCertificate  KMIPObjectType = "certificate"
	KMIPTypeSecretData   KMIPObjectType = "secret_data"
)

// KMIPKeyState represents key lifecycle state.
type KMIPKeyState string

const (
	KMIPStatePreActive            KMIPKeyState = "pre_active"
	KMIPStateActive               KMIPKeyState = "active"
	KMIPStateDeactivated          KMIPKeyState = "deactivated"
	KMIPStateCompromised          KMIPKeyState = "compromised"
	KMIPStateDestroyed            KMIPKeyState = "destroyed"
	KMIPStateDestroyedCompromised KMIPKeyState = "destroyed_compromised"
)

// KMIPKey represents a managed key object.
type KMIPKey struct {
	ID               string         `json:"id"`
	UniqueIdentifier string         `json:"unique_identifier"`
	Name             string         `json:"name"`
	ObjectType       KMIPObjectType `json:"object_type"`
	KeyState         KMIPKeyState   `json:"key_state"`
	KeyAlgorithm     string         `json:"key_algorithm"` // AES, RSA, etc.
	KeyLength        int            `json:"key_length"`    // Bits
	KeyUsage         []string       `json:"key_usage"`     // encrypt, decrypt, sign, verify
	CreatedAt        time.Time      `json:"created_at"`
	ActivatedAt      time.Time      `json:"activated_at,omitempty"`
	DeactivatedAt    time.Time      `json:"deactivated_at,omitempty"`
	ExpiresAt        time.Time      `json:"expires_at,omitempty"`
	CreatedBy        string         `json:"created_by"`   // User or service
	KMSProvider      string         `json:"kms_provider"` // External KMS name if applicable
	Tags             []string       `json:"tags"`
}

// KMIPClientConfig represents KMIP client configuration.
type KMIPClientConfig struct {
	ServerAddress      string `json:"server_address"`
	ServerPort         int    `json:"server_port"` // Default 5696
	UseTLS             bool   `json:"use_tls"`
	CertificatePath    string `json:"certificate_path"`
	KeyPath            string `json:"key_path"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	ConnectionPoolSize int    `json:"connection_pool_size"`
	KMSName            string `json:"kms_name"` // External KMS identifier
}

// KMIPManager manages KMIP 2.0 key lifecycle operations.
type KMIPManager struct {
	mu           sync.RWMutex
	keys         map[string]*KMIPKey
	clientConfig *KMIPClientConfig
	connections  map[string]net.Conn // Connection pool
	logger       *zap.Logger
	configPath   string
}

// NewKMIPManager creates a new KMIP manager.
func NewKMIPManager(configPath string, logger *zap.Logger) (*KMIPManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &KMIPClientConfig{
		ServerAddress:      "localhost",
		ServerPort:         5696,
		UseTLS:             true,
		TimeoutSeconds:     30,
		ConnectionPoolSize: 10,
		KMSName:            "default",
	}

	m := &KMIPManager{
		keys:         make(map[string]*KMIPKey),
		clientConfig: config,
		connections:  make(map[string]net.Conn),
		logger:       logger,
		configPath:   configPath,
	}

	if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return m, nil
}

// CreateKey creates a new key object.
func (m *KMIPManager) CreateKey(ctx context.Context, name string, objectType KMIPObjectType, algorithm string, length int, usage []string, createdBy string) (*KMIPKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate name
	for _, key := range m.keys {
		if key.Name == name {
			return nil, fmt.Errorf("key with name %s already exists", name)
		}
	}

	keyID := uuid.New().String()
	uniqueID := uuid.New().String()

	key := &KMIPKey{
		ID:               keyID,
		UniqueIdentifier: uniqueID,
		Name:             name,
		ObjectType:       objectType,
		KeyState:         KMIPStatePreActive,
		KeyAlgorithm:     algorithm,
		KeyLength:        length,
		KeyUsage:         usage,
		CreatedAt:        time.Now(),
		CreatedBy:        createdBy,
		KMSProvider:      m.clientConfig.KMSName,
		Tags:             []string{},
	}

	m.keys[keyID] = key
	m.logger.Info("Created KMIP key",
		zap.String("key_id", keyID),
		zap.String("name", name),
		zap.String("type", string(objectType)))

	// TODO: Send Create operation to external KMS if configured

	return key, m.saveConfig()
}

// RegisterKey registers an existing key from external KMS.
func (m *KMIPManager) RegisterKey(ctx context.Context, uniqueIdentifier string, name string, objectType KMIPObjectType, kmsProvider string) (*KMIPKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyID := uuid.New().String()

	key := &KMIPKey{
		ID:               keyID,
		UniqueIdentifier: uniqueIdentifier,
		Name:             name,
		ObjectType:       objectType,
		KeyState:         KMIPStateActive,
		CreatedAt:        time.Now(),
		ActivatedAt:      time.Now(),
		KMSProvider:      kmsProvider,
		Tags:             []string{},
	}

	m.keys[keyID] = key
	m.logger.Info("Registered external KMIP key",
		zap.String("key_id", keyID),
		zap.String("unique_identifier", uniqueIdentifier),
		zap.String("kms", kmsProvider))

	return key, m.saveConfig()
}

// ActivateKey activates a key for use.
func (m *KMIPManager) ActivateKey(ctx context.Context, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return fmt.Errorf("key %s not found", keyID)
	}

	if key.KeyState != KMIPStatePreActive {
		return fmt.Errorf("key must be in pre-active state to activate")
	}

	key.KeyState = KMIPStateActive
	key.ActivatedAt = time.Now()

	m.logger.Info("Activated KMIP key", zap.String("key_id", keyID))

	// TODO: Send Activate operation to external KMS

	return m.saveConfig()
}

// DeactivateKey deactivates a key.
func (m *KMIPManager) DeactivateKey(ctx context.Context, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return fmt.Errorf("key %s not found", keyID)
	}

	if key.KeyState != KMIPStateActive {
		return fmt.Errorf("key must be in active state to deactivate")
	}

	key.KeyState = KMIPStateDeactivated
	key.DeactivatedAt = time.Now()

	m.logger.Info("Deactivated KMIP key", zap.String("key_id", keyID))

	return m.saveConfig()
}

// DestroyKey destroys a key (permanent removal).
func (m *KMIPManager) DestroyKey(ctx context.Context, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return fmt.Errorf("key %s not found", keyID)
	}

	key.KeyState = KMIPStateDestroyed
	key.DeactivatedAt = time.Now()

	// Remove from local registry but mark as destroyed
	m.logger.Info("Destroyed KMIP key", zap.String("key_id", keyID))

	// TODO: Send Destroy operation to external KMS

	return m.saveConfig()
}

// RevokeKey marks a key as compromised.
func (m *KMIPManager) RevokeKey(ctx context.Context, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return fmt.Errorf("key %s not found", keyID)
	}

	key.KeyState = KMIPStateCompromised

	m.logger.Warn("Key marked as compromised",
		zap.String("key_id", keyID),
		zap.String("name", key.Name))

	return m.saveConfig()
}

// GetKey retrieves a key by ID.
func (m *KMIPManager) GetKey(ctx context.Context, keyID string) (*KMIPKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key %s not found", keyID)
	}

	return key, nil
}

// GetKeyByName retrieves a key by name.
func (m *KMIPManager) GetKeyByName(ctx context.Context, name string) (*KMIPKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, key := range m.keys {
		if key.Name == name {
			return key, nil
		}
	}

	return nil, fmt.Errorf("key with name %s not found", name)
}

// ListKeys lists all keys.
func (m *KMIPManager) ListKeys(ctx context.Context) []*KMIPKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*KMIPKey, 0, len(m.keys))
	for _, key := range m.keys {
		result = append(result, key)
	}
	return result
}

// ListActiveKeys lists only active keys.
func (m *KMIPManager) ListActiveKeys(ctx context.Context) []*KMIPKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*KMIPKey{}
	for _, key := range m.keys {
		if key.KeyState == KMIPStateActive {
			result = append(result, key)
		}
	}
	return result
}

// SetClientConfig updates KMIP client configuration.
func (m *KMIPManager) SetClientConfig(ctx context.Context, config *KMIPClientConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clientConfig = config
	m.logger.Info("Updated KMIP client config",
		zap.String("server", config.ServerAddress),
		zap.Int("port", config.ServerPort))

	return m.saveConfig()
}

// GetKeyStats returns key management statistics.
func (m *KMIPManager) GetKeyStats(ctx context.Context) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]int{
		"total":       len(m.keys),
		"pre_active":  0,
		"active":      0,
		"deactivated": 0,
		"compromised": 0,
		"destroyed":   0,
	}

	byType := map[string]int{}
	byAlgorithm := map[string]int{}

	for _, key := range m.keys {
		stats[string(key.KeyState)]++
		byType[string(key.ObjectType)]++
		byAlgorithm[key.KeyAlgorithm]++
	}

	return map[string]interface{}{
		"key_stats":      stats,
		"by_type":        byType,
		"by_algorithm":   byAlgorithm,
		"external_kms":   m.clientConfig.KMSName,
		"server_address": m.clientConfig.ServerAddress,
	}
}

// loadConfig loads KMIP configuration.
func (m *KMIPManager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		ClientConfig *KMIPClientConfig   `json:"client_config"`
		Keys         map[string]*KMIPKey `json:"keys"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.ClientConfig != nil {
		m.clientConfig = cfg.ClientConfig
	}
	m.keys = cfg.Keys

	return nil
}

// saveConfig saves KMIP configuration.
func (m *KMIPManager) saveConfig() error {
	cfg := struct {
		ClientConfig *KMIPClientConfig   `json:"client_config"`
		Keys         map[string]*KMIPKey `json:"keys"`
	}{
		ClientConfig: m.clientConfig,
		Keys:         m.keys,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}
