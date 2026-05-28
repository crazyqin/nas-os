package quantumsafe

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager is the core business logic for quantum-safe operations.
//
// It manages KEM and signature key pairs, handles key rotation,
// hybrid encryption/decryption, and migration from classical to
// post-quantum algorithms.
type Manager struct {
	config         Config
	kemKeys        map[string]*KEMKeyPair       // keyID -> key
	signKeys       map[string]*SignatureKeyPair  // keyID -> key
	rotationLog    []KeyRotationRecord
	migrationJobs  map[string]*MigrationJob
	logger         *zap.Logger
	mu             sync.RWMutex
}

// NewManager creates a new quantumsafe manager with the given config.
func NewManager(cfg Config, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		config:        cfg,
		kemKeys:       make(map[string]*KEMKeyPair),
		signKeys:      make(map[string]*SignatureKeyPair),
		rotationLog:   make([]KeyRotationRecord, 0),
		migrationJobs: make(map[string]*MigrationJob),
		logger:        logger,
	}
}

// NewDefaultManager creates a manager with default configuration.
func NewDefaultManager() *Manager {
	return NewManager(DefaultConfig(), nil)
}

// ========== KEM Key Management ==========

// GenerateKEMKeyPair generates a new KEM key pair using the configured default algorithm.
func (m *Manager) GenerateKEMKeyPair() (*KEMKeyPair, error) {
	return m.GenerateKEMKeyPairWithAlgorithm(m.config.DefaultKEMAlgorithm)
}

// GenerateKEMKeyPairWithAlgorithm generates a KEM key pair with a specific algorithm.
func (m *Manager) GenerateKEMKeyPairWithAlgorithm(algo Algorithm) (*KEMKeyPair, error) {
	scheme, err := GetKEMScheme(algo)
	if err != nil {
		return nil, err
	}

	pub, priv, err := scheme.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate kem keypair: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := &KEMKeyPair{
		ID:        uuid.New().String(),
		Algorithm: algo,
		PublicKey: pub,
		PrivateKey: priv,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.config.KeyRotationPeriod),
		State:     KeyStateActive,
		Version:   m.nextKEMVersion(algo),
	}

	m.kemKeys[key.ID] = key
	m.logger.Info("KEM key pair generated",
		zap.String("id", key.ID),
		zap.String("algorithm", string(algo)),
		zap.Int("version", key.Version),
	)
	return key, nil
}

// GetKEMKeyPair returns a KEM key pair by ID.
// Private key is zeroed in the returned copy.
func (m *Manager) GetKEMKeyPair(id string) (*KEMKeyPair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.kemKeys[id]
	if !ok {
		return nil, fmt.Errorf("KEM key not found: %s", id)
	}
	cp := *key
	cp.PrivateKey = nil // never expose private key
	return &cp, nil
}

// ListKEMKeyPairs returns all KEM key pairs with private keys redacted.
func (m *Manager) ListKEMKeyPairs() []*KEMKeyPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*KEMKeyPair, 0, len(m.kemKeys))
	for _, k := range m.kemKeys {
		cp := *k
		cp.PrivateKey = nil
		result = append(result, &cp)
	}
	return result
}

// DestroyKEMKeyPair securely destroys a KEM key pair.
func (m *Manager) DestroyKEMKeyPair(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.kemKeys[id]
	if !ok {
		return fmt.Errorf("KEM key not found: %s", id)
	}
	zeroBytes(key.PrivateKey)
	key.PrivateKey = nil
	key.State = KeyStateDestroyed
	m.logger.Info("KEM key destroyed", zap.String("id", id))
	return nil
}

// ========== Signature Key Management ==========

// GenerateSignatureKeyPair generates a new signature key pair.
func (m *Manager) GenerateSignatureKeyPair() (*SignatureKeyPair, error) {
	return m.GenerateSignatureKeyPairWithAlgorithm(m.config.DefaultSignatureAlgorithm)
}

// GenerateSignatureKeyPairWithAlgorithm generates a signature key pair with a specific algorithm.
func (m *Manager) GenerateSignatureKeyPairWithAlgorithm(algo Algorithm) (*SignatureKeyPair, error) {
	scheme, err := GetSignatureScheme(algo)
	if err != nil {
		return nil, err
	}

	pub, priv, err := scheme.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate sign keypair: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := &SignatureKeyPair{
		ID:        uuid.New().String(),
		Algorithm: algo,
		PublicKey: pub,
		PrivateKey: priv,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.config.KeyRotationPeriod),
		State:     KeyStateActive,
		Version:   m.nextSignVersion(algo),
	}

	m.signKeys[key.ID] = key
	m.logger.Info("Signature key pair generated",
		zap.String("id", key.ID),
		zap.String("algorithm", string(algo)),
		zap.Int("version", key.Version),
	)
	return key, nil
}

// GetSignatureKeyPair returns a signature key pair by ID (private key redacted).
func (m *Manager) GetSignatureKeyPair(id string) (*SignatureKeyPair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.signKeys[id]
	if !ok {
		return nil, fmt.Errorf("signature key not found: %s", id)
	}
	cp := *key
	cp.PrivateKey = nil
	return &cp, nil
}

// ListSignatureKeyPairs returns all signature key pairs (private keys redacted).
func (m *Manager) ListSignatureKeyPairs() []*SignatureKeyPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SignatureKeyPair, 0, len(m.signKeys))
	for _, k := range m.signKeys {
		cp := *k
		cp.PrivateKey = nil
		result = append(result, &cp)
	}
	return result
}

// DestroySignatureKeyPair securely destroys a signature key pair.
func (m *Manager) DestroySignatureKeyPair(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.signKeys[id]
	if !ok {
		return fmt.Errorf("signature key not found: %s", id)
	}
	zeroBytes(key.PrivateKey)
	key.PrivateKey = nil
	key.State = KeyStateDestroyed
	m.logger.Info("Signature key destroyed", zap.String("id", id))
	return nil
}

// ========== Encapsulation / Decapsulation ==========

// Encapsulate generates a shared secret encapsulated to the given KEM key.
func (m *Manager) Encapsulate(keyID string) (*EncapsulatedKey, error) {
	m.mu.RLock()
	key, ok := m.kemKeys[keyID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("KEM key not found: %s", keyID)
	}
	if key.State == KeyStateDestroyed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("KEM key is destroyed: %s", keyID)
	}
	pubKey := make([]byte, len(key.PublicKey))
	copy(pubKey, key.PublicKey)
	algo := key.Algorithm
	m.mu.RUnlock()

	scheme, err := GetKEMScheme(algo)
	if err != nil {
		return nil, err
	}

	ct, ss, err := scheme.Encapsulate(pubKey)
	if err != nil {
		return nil, fmt.Errorf("encapsulate: %w", err)
	}

	return &EncapsulatedKey{
		Ciphertext:   ct,
		SharedSecret: ss,
		KeyID:        keyID,
		Algorithm:    algo,
		CreatedAt:    time.Now(),
	}, nil
}

// Decapsulate recovers the shared secret from an encapsulated ciphertext.
func (m *Manager) Decapsulate(keyID string, ciphertext []byte) ([]byte, error) {
	m.mu.RLock()
	key, ok := m.kemKeys[keyID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("KEM key not found: %s", keyID)
	}
	if key.State == KeyStateDestroyed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("KEM key is destroyed: %s", keyID)
	}
	privKey := make([]byte, len(key.PrivateKey))
	copy(privKey, key.PrivateKey)
	algo := key.Algorithm
	m.mu.RUnlock()

	if len(privKey) == 0 {
		return nil, fmt.Errorf("KEM private key unavailable: %s", keyID)
	}

	scheme, err := GetKEMScheme(algo)
	if err != nil {
		return nil, err
	}

	ss, err := scheme.Decapsulate(ciphertext, privKey)
	if err != nil {
		return nil, fmt.Errorf("decapsulate: %w", err)
	}
	return ss, nil
}

// ========== Sign / Verify ==========

// Sign signs a message using the specified signature key.
func (m *Manager) Sign(keyID string, message []byte) (*SignedMessage, error) {
	m.mu.RLock()
	key, ok := m.signKeys[keyID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("signature key not found: %s", keyID)
	}
	if key.State == KeyStateDestroyed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("signature key is destroyed: %s", keyID)
	}
	privKey := make([]byte, len(key.PrivateKey))
	copy(privKey, key.PrivateKey)
	algo := key.Algorithm
	m.mu.RUnlock()

	if len(privKey) == 0 {
		return nil, fmt.Errorf("signature private key unavailable: %s", keyID)
	}

	scheme, err := GetSignatureScheme(algo)
	if err != nil {
		return nil, err
	}

	sig, err := scheme.Sign(message, privKey)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	return &SignedMessage{
		Message:   message,
		Signature: sig,
		KeyID:     keyID,
		Algorithm: algo,
		SignedAt:  time.Now(),
	}, nil
}

// Verify verifies a signed message.
func (m *Manager) Verify(keyID string, message, signature []byte) (bool, error) {
	m.mu.RLock()
	key, ok := m.signKeys[keyID]
	if !ok {
		m.mu.RUnlock()
		return false, fmt.Errorf("signature key not found: %s", keyID)
	}
	pubKey := make([]byte, len(key.PublicKey))
	copy(pubKey, key.PublicKey)
	algo := key.Algorithm
	m.mu.RUnlock()

	scheme, err := GetSignatureScheme(algo)
	if err != nil {
		return false, err
	}

	return scheme.Verify(message, signature, pubKey)
}

// ========== Hybrid Encryption / Decryption ==========

// HybridEncrypt encrypts data using hybrid mode: KEM + AES-256-GCM.
//
// Flow:
//  1. Encapsulate a shared secret to the KEM key
//  2. Derive an AES-256 key from the shared secret
//  3. Encrypt the plaintext with AES-256-GCM
func (m *Manager) HybridEncrypt(kemKeyID string, plaintext []byte) (*HybridCiphertext, error) {
	// Step 1: KEM encapsulation
	encKey, err := m.Encapsulate(kemKeyID)
	if err != nil {
		return nil, fmt.Errorf("kem encapsulate: %w", err)
	}

	// Step 2: Derive AES key
	aesKey, err := hkdfDerive(encKey.SharedSecret, nil, []byte("hybrid-encrypt"), 32)
	if err != nil {
		return nil, fmt.Errorf("derive aes key: %w", err)
	}

	// Step 3: Encrypt with AES-256-GCM
	nonce, ct, err := EncryptAESGCM(aesKey, plaintext, encKey.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("aes-gcm encrypt: %w", err)
	}

	m.mu.RLock()
	algo := m.kemKeys[kemKeyID].Algorithm
	m.mu.RUnlock()

	return &HybridCiphertext{
		KEMCiphertext: encKey.Ciphertext,
		EncryptedData: ct,
		Nonce:         nonce,
		KEMKeyID:      kemKeyID,
		Algorithm:     algo,
		CreatedAt:     time.Now(),
	}, nil
}

// HybridDecrypt decrypts data that was encrypted with HybridEncrypt.
func (m *Manager) HybridDecrypt(kemKeyID string, hc *HybridCiphertext) ([]byte, error) {
	// Step 1: KEM decapsulation
	ss, err := m.Decapsulate(kemKeyID, hc.KEMCiphertext)
	if err != nil {
		return nil, fmt.Errorf("kem decapsulate: %w", err)
	}

	// Step 2: Derive AES key
	aesKey, err := hkdfDerive(ss, nil, []byte("hybrid-encrypt"), 32)
	if err != nil {
		return nil, fmt.Errorf("derive aes key: %w", err)
	}

	// Step 3: Decrypt
	plaintext, err := DecryptAESGCM(aesKey, hc.Nonce, hc.EncryptedData, hc.KEMCiphertext)
	if err != nil {
		return nil, fmt.Errorf("aes-gcm decrypt: %w", err)
	}
	return plaintext, nil
}

// ========== Key Rotation ==========

// RotateKEMKey rotates a KEM key: generates a new key and deprecates the old one.
func (m *Manager) RotateKEMKey(oldKeyID string) (*KEMKeyPair, error) {
	m.mu.Lock()
	oldKey, ok := m.kemKeys[oldKeyID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("KEM key not found: %s", oldKeyID)
	}
	algo := oldKey.Algorithm
	oldKey.State = KeyStateDeprecated
	m.mu.Unlock()

	newKey, err := m.GenerateKEMKeyPairWithAlgorithm(algo)
	if err != nil {
		return nil, fmt.Errorf("generate new KEM key: %w", err)
	}

	m.mu.Lock()
	m.rotationLog = append(m.rotationLog, KeyRotationRecord{
		ID:            uuid.New().String(),
		KeyType:       "kem",
		OldKeyID:      oldKeyID,
		NewKeyID:      newKey.ID,
		Algorithm:     algo,
		RotatedAt:     time.Now(),
		TriggerReason: "manual",
	})
	m.mu.Unlock()

	m.logger.Info("KEM key rotated",
		zap.String("old_id", oldKeyID),
		zap.String("new_id", newKey.ID),
		zap.String("algorithm", string(algo)),
	)
	return newKey, nil
}

// RotateSignatureKey rotates a signature key.
func (m *Manager) RotateSignatureKey(oldKeyID string) (*SignatureKeyPair, error) {
	m.mu.Lock()
	oldKey, ok := m.signKeys[oldKeyID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("signature key not found: %s", oldKeyID)
	}
	algo := oldKey.Algorithm
	oldKey.State = KeyStateDeprecated
	m.mu.Unlock()

	newKey, err := m.GenerateSignatureKeyPairWithAlgorithm(algo)
	if err != nil {
		return nil, fmt.Errorf("generate new sign key: %w", err)
	}

	m.mu.Lock()
	m.rotationLog = append(m.rotationLog, KeyRotationRecord{
		ID:            uuid.New().String(),
		KeyType:       "signature",
		OldKeyID:      oldKeyID,
		NewKeyID:      newKey.ID,
		Algorithm:     algo,
		RotatedAt:     time.Now(),
		TriggerReason: "manual",
	})
	m.mu.Unlock()

	m.logger.Info("Signature key rotated",
		zap.String("old_id", oldKeyID),
		zap.String("new_id", newKey.ID),
		zap.String("algorithm", string(algo)),
	)
	return newKey, nil
}

// AutoRotateKeys checks for expired keys and rotates them.
// Returns the number of keys rotated.
func (m *Manager) AutoRotateKeys() (int, error) {
	m.mu.RLock()
	var expiredKEM []string
	var expiredSign []string
	now := time.Now()

	for id, k := range m.kemKeys {
		if k.State == KeyStateActive && now.After(k.ExpiresAt) {
			expiredKEM = append(expiredKEM, id)
		}
	}
	for id, k := range m.signKeys {
		if k.State == KeyStateActive && now.After(k.ExpiresAt) {
			expiredSign = append(expiredSign, id)
		}
	}
	m.mu.RUnlock()

	rotated := 0
	for _, id := range expiredKEM {
		if _, err := m.RotateKEMKey(id); err != nil {
			m.logger.Error("auto-rotate KEM key failed",
				zap.String("id", id),
				zap.Error(err),
			)
			continue
		}
		rotated++
	}
	for _, id := range expiredSign {
		if _, err := m.RotateSignatureKey(id); err != nil {
			m.logger.Error("auto-rotate sign key failed",
				zap.String("id", id),
				zap.Error(err),
			)
			continue
		}
		rotated++
	}

	if rotated > 0 {
		m.logger.Info("auto-rotation completed", zap.Int("rotated", rotated))
	}
	return rotated, nil
}

// GetRotationLog returns the key rotation history.
func (m *Manager) GetRotationLog() []KeyRotationRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]KeyRotationRecord, len(m.rotationLog))
	copy(result, m.rotationLog)
	return result
}

// ========== Migration ==========

// StartMigration creates a migration job from classical to quantum-safe encryption.
func (m *Manager) StartMigration(name string, sourceAlgo, targetAlgo Algorithm, totalItems int64) (*MigrationJob, error) {
	// Validate algorithms
	if _, err := GetKEMScheme(targetAlgo); err != nil {
		return nil, fmt.Errorf("invalid target algorithm: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job := &MigrationJob{
		ID:              uuid.New().String(),
		Name:            name,
		SourceAlgorithm: sourceAlgo,
		TargetAlgorithm: targetAlgo,
		Status:          MigrationPending,
		TotalItems:      totalItems,
		ProcessedItems:  0,
		FailedItems:     0,
		Errors:          make([]string, 0),
	}

	m.migrationJobs[job.ID] = job
	m.logger.Info("Migration job created",
		zap.String("id", job.ID),
		zap.String("name", name),
		zap.String("source", string(sourceAlgo)),
		zap.String("target", string(targetAlgo)),
	)
	return job, nil
}

// UpdateMigrationProgress updates the progress of a migration job.
func (m *Manager) UpdateMigrationProgress(jobID string, processed, failed int64, errors []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.migrationJobs[jobID]
	if !ok {
		return fmt.Errorf("migration job not found: %s", jobID)
	}

	job.ProcessedItems += processed
	job.FailedItems += failed
	job.Errors = append(job.Errors, errors...)

	if job.Status == MigrationPending {
		now := time.Now()
		job.StartedAt = &now
		job.Status = MigrationInProgress
	}

	if job.ProcessedItems+job.FailedItems >= job.TotalItems {
		now := time.Now()
		job.CompletedAt = &now
		if job.FailedItems > 0 {
			job.Status = MigrationFailed
		} else {
			job.Status = MigrationCompleted
		}
		m.logger.Info("Migration job completed",
			zap.String("id", jobID),
			zap.Int64("processed", job.ProcessedItems),
			zap.Int64("failed", job.FailedItems),
		)
	}
	return nil
}

// GetMigrationJob returns the status of a migration job.
func (m *Manager) GetMigrationJob(jobID string) (*MigrationJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.migrationJobs[jobID]
	if !ok {
		return nil, fmt.Errorf("migration job not found: %s", jobID)
	}
	cp := *job
	return &cp, nil
}

// ListMigrationJobs returns all migration jobs.
func (m *Manager) ListMigrationJobs() []*MigrationJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MigrationJob, 0, len(m.migrationJobs))
	for _, j := range m.migrationJobs {
		cp := *j
		result = append(result, &cp)
	}
	return result
}

// ========== Status ==========

// Status returns the current module status.
type Status struct {
	TotalKEMKeys        int   `json:"total_kem_keys"`
	ActiveKEMKeys       int   `json:"active_kem_keys"`
	TotalSignKeys       int   `json:"total_sign_keys"`
	ActiveSignKeys      int   `json:"active_sign_keys"`
	TotalRotations      int   `json:"total_rotations"`
	TotalMigrations     int   `json:"total_migrations"`
	DefaultKEMAlgorithm string  `json:"default_kem_algorithm"`
	DefaultSignAlgorithm string `json:"default_sign_algorithm"`
	HybridModeEnabled   bool   `json:"hybrid_mode_enabled"`
}

// GetStatus returns the module status.
func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeKEM := 0
	for _, k := range m.kemKeys {
		if k.State == KeyStateActive {
			activeKEM++
		}
	}
	activeSign := 0
	for _, k := range m.signKeys {
		if k.State == KeyStateActive {
			activeSign++
		}
	}

	return Status{
		TotalKEMKeys:         len(m.kemKeys),
		ActiveKEMKeys:        activeKEM,
		TotalSignKeys:        len(m.signKeys),
		ActiveSignKeys:       activeSign,
		TotalRotations:       len(m.rotationLog),
		TotalMigrations:      len(m.migrationJobs),
		DefaultKEMAlgorithm:  string(m.config.DefaultKEMAlgorithm),
		DefaultSignAlgorithm: string(m.config.DefaultSignatureAlgorithm),
		HybridModeEnabled:    m.config.EnableHybridMode,
	}
}

// GetConfig returns a copy of the current configuration.
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig updates the manager configuration.
func (m *Manager) UpdateConfig(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	m.logger.Info("Configuration updated")
}

// GetPublicKeyHex returns the hex-encoded public key for a KEM key.
func (m *Manager) GetPublicKeyHex(keyID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.kemKeys[keyID]
	if !ok {
		return "", fmt.Errorf("KEM key not found: %s", keyID)
	}
	return hex.EncodeToString(key.PublicKey), nil
}

// GetSignPublicKeyHex returns the hex-encoded public key for a signature key.
func (m *Manager) GetSignPublicKeyHex(keyID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.signKeys[keyID]
	if !ok {
		return "", fmt.Errorf("signature key not found: %s", keyID)
	}
	return hex.EncodeToString(key.PublicKey), nil
}

// ========== Internal Helpers ==========

func (m *Manager) nextKEMVersion(algo Algorithm) int {
	maxVer := 0
	for _, k := range m.kemKeys {
		if k.Algorithm == algo && k.Version > maxVer {
			maxVer = k.Version
		}
	}
	return maxVer + 1
}

func (m *Manager) nextSignVersion(algo Algorithm) int {
	maxVer := 0
	for _, k := range m.signKeys {
		if k.Algorithm == algo && k.Version > maxVer {
			maxVer = k.Version
		}
	}
	return maxVer + 1
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
