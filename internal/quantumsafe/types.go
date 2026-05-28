// Package quantumsafe provides post-quantum cryptographic operations.
//
// Implements NIST-standardized post-quantum algorithms:
//   - CRYSTALS-Kyber for key encapsulation mechanism (KEM)
//   - CRYSTALS-Dilithium for digital signatures
//   - Hybrid encryption mode (classical + post-quantum)
//   - Key rotation management
//   - Migration tools from classical to quantum-safe encryption
//
// Current implementation uses Go standard crypto as the underlying primitive,
// with an abstraction layer designed for drop-in replacement with native PQC
// libraries (e.g., cloudflare/circl) when available.
package quantumsafe

import (
	"time"
)

// ========== Algorithm Identifiers ==========

// Algorithm represents a cryptographic algorithm identifier.
type Algorithm string

const (
	// Kyber768 NIST Level 3 KEM (192-bit security).
	Kyber768 Algorithm = "KYBER-768"
	// Kyber1024 NIST Level 5 KEM (256-bit security).
	Kyber1024 Algorithm = "KYBER-1024"
	// Dilithium3 NIST Level 3 signature scheme.
	Dilithium3 Algorithm = "DILITHIUM-3"
	// Dilithium5 NIST Level 5 signature scheme.
	Dilithium5 Algorithm = "DILITHIUM-5"
	// HybridKyber768 hybrid KEM: X25519 + Kyber768.
	HybridKyber768 Algorithm = "HYBRID-KYBER-768"
	// HybridDilithium3 hybrid signature: Ed25519 + Dilithium3.
	HybridDilithium3 Algorithm = "HYBRID-DILITHIUM-3"
)

// ========== Key Types ==========

// KeyState represents the lifecycle state of a cryptographic key.
type KeyState string

const (
	// KeyStateActive key is active and usable.
	KeyStateActive KeyState = "active"
	// KeyStateRotating key is in rotation process.
	KeyStateRotating KeyState = "rotating"
	// KeyStateDeprecated key is deprecated, only for verification/decryption.
	KeyStateDeprecated KeyState = "deprecated"
	// KeyStateDestroyed key material has been securely erased.
	KeyStateDestroyed KeyState = "destroyed"
)

// KEMKeyPair holds a KEM (key encapsulation mechanism) key pair.
type KEMKeyPair struct {
	ID             string    `json:"id"`
	Algorithm      Algorithm `json:"algorithm"`
	PublicKey      []byte    `json:"public_key"`
	PrivateKey     []byte    `json:"private_key,omitempty"` // omitted in responses
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	State          KeyState  `json:"state"`
	RotationGroup  int       `json:"rotation_group"`
	Version        int       `json:"version"`
}

// SignatureKeyPair holds a digital signature key pair.
type SignatureKeyPair struct {
	ID             string    `json:"id"`
	Algorithm      Algorithm `json:"algorithm"`
	PublicKey      []byte    `json:"public_key"`
	PrivateKey     []byte    `json:"private_key,omitempty"` // omitted in responses
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	State          KeyState  `json:"state"`
	RotationGroup  int       `json:"rotation_group"`
	Version        int       `json:"version"`
}

// EncapsulatedKey holds the output of a KEM encapsulation operation.
type EncapsulatedKey struct {
	Ciphertext   []byte    `json:"ciphertext"`
	SharedSecret []byte    `json:"shared_secret,omitempty"` // omitted in responses
	KeyID        string    `json:"key_id"`
	Algorithm    Algorithm `json:"algorithm"`
	CreatedAt    time.Time `json:"created_at"`
}

// SignedMessage holds a signed message with metadata.
type SignedMessage struct {
	Message   []byte    `json:"message"`
	Signature []byte    `json:"signature"`
	KeyID     string    `json:"key_id"`
	Algorithm Algorithm `json:"algorithm"`
	SignedAt  time.Time `json:"signed_at"`
}

// HybridCiphertext holds hybrid-encrypted data.
type HybridCiphertext struct {
	KEMCiphertext    []byte    `json:"kem_ciphertext"`    // KEM encapsulated key
	EncryptedData    []byte    `json:"encrypted_data"`    // AES-256-GCM encrypted payload
	Nonce            []byte    `json:"nonce"`
	KEMKeyID         string    `json:"kem_key_id"`
	Algorithm        Algorithm `json:"algorithm"`
	CreatedAt        time.Time `json:"created_at"`
}

// ========== Key Rotation Types ==========

// RotationPolicy defines key rotation rules.
type RotationPolicy struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Algorithm       Algorithm     `json:"algorithm"`
	RotationPeriod  time.Duration `json:"rotation_period"`
	OverlapPeriod   time.Duration `json:"overlap_period"` // period where old+new keys are both valid
	MaxVersions     int           `json:"max_versions"`   // max historical versions to keep
	AutoRotate      bool          `json:"auto_rotate"`
}

// KeyRotationRecord records a key rotation event.
type KeyRotationRecord struct {
	ID            string    `json:"id"`
	KeyType       string    `json:"key_type"` // "kem" or "signature"
	OldKeyID      string    `json:"old_key_id"`
	NewKeyID      string    `json:"new_key_id"`
	Algorithm     Algorithm `json:"algorithm"`
	RotatedAt     time.Time `json:"rotated_at"`
	TriggerReason string    `json:"trigger_reason"`
}

// ========== Migration Types ==========

// MigrationStatus represents the state of a crypto migration.
type MigrationStatus string

const (
	// MigrationPending migration not started.
	MigrationPending MigrationStatus = "pending"
	// MigrationInProgress migration is running.
	MigrationInProgress MigrationStatus = "in_progress"
	// MigrationCompleted migration finished successfully.
	MigrationCompleted MigrationStatus = "completed"
	// MigrationFailed migration encountered errors.
	MigrationFailed MigrationStatus = "failed"
)

// MigrationJob tracks a migration from classical to quantum-safe encryption.
type MigrationJob struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	SourceAlgorithm Algorithm       `json:"source_algorithm"`
	TargetAlgorithm Algorithm       `json:"target_algorithm"`
	Status          MigrationStatus `json:"status"`
	TotalItems      int64           `json:"total_items"`
	ProcessedItems  int64           `json:"processed_items"`
	FailedItems     int64           `json:"failed_items"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	Errors          []string        `json:"errors,omitempty"`
}

// ========== Manager Config ==========

// Config holds the quantumsafe module configuration.
type Config struct {
	DefaultKEMAlgorithm       Algorithm       `json:"default_kem_algorithm"`
	DefaultSignatureAlgorithm Algorithm       `json:"default_signature_algorithm"`
	KeyRotationPeriod         time.Duration   `json:"key_rotation_period"`
	OverlapPeriod             time.Duration   `json:"overlap_period"`
	MaxKeyVersions            int             `json:"max_key_versions"`
	EnableHybridMode          bool            `json:"enable_hybrid_mode"`
	RotationPolicies          []RotationPolicy `json:"rotation_policies,omitempty"`
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		DefaultKEMAlgorithm:       HybridKyber768,
		DefaultSignatureAlgorithm: HybridDilithium3,
		KeyRotationPeriod:         24 * time.Hour * 30, // 30 days
		OverlapPeriod:             24 * time.Hour * 7,  // 7 days overlap
		MaxKeyVersions:            5,
		EnableHybridMode:          true,
	}
}
