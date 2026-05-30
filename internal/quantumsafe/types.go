// Package quantumsafe 提供抗量子加密模块功能，为存储和传输提供后量子密码学保护。
// 支持混合加密方案、密钥迁移和加密审计。
package quantumsafe

import "time"

// Algorithm 算法类型（与 algorithms.go 兼容）
type Algorithm string

const (
	// Post-quantum KEM algorithms
	Kyber768  Algorithm = "kyber768"
	Kyber1024 Algorithm = "kyber1024"

	// Post-quantum signature algorithms
	Dilithium3 Algorithm = "dilithium3"
	Dilithium5 Algorithm = "dilithium5"

	// Hybrid algorithms
	HybridKyber768    Algorithm = "hybrid-kyber768"
	HybridDilithium3  Algorithm = "hybrid-dilithium3"

	// Classical algorithms
	AlgorithmClassic Algorithm = "aes-256-gcm"
	AlgorithmX25519  Algorithm = "x25519"
	AlgorithmEd25519 Algorithm = "ed25519"
)

// KeyStatus 密钥状态
type KeyStatus string

const (
	KeyStatusActive     KeyStatus = "active"
	KeyStatusRotating   KeyStatus = "rotating"
	KeyStatusDeprecated KeyStatus = "deprecated"
	KeyStatusRevoked    KeyStatus = "revoked"
	KeyStatusArchived   KeyStatus = "archived"
)

// CipherMode 加密模式
type CipherMode string

const (
	ModeHybrid      CipherMode = "hybrid"
	ModePostQuantum CipherMode = "post_quantum"
	ModeClassical   CipherMode = "classical"
)

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	MigrationPending    MigrationStatus = "pending"
	MigrationInProgress MigrationStatus = "in_progress"
	MigrationCompleted  MigrationStatus = "completed"
	MigrationFailed     MigrationStatus = "failed"
	MigrationRolledBack MigrationStatus = "rolled_back"
)

// AuditAction 审计动作
type AuditAction string

const (
	AuditKeyGenerate  AuditAction = "key_generate"
	AuditKeyRotate    AuditAction = "key_rotate"
	AuditKeyRevoke    AuditAction = "key_revoke"
	AuditEncrypt      AuditAction = "encrypt"
	AuditDecrypt      AuditAction = "decrypt"
	AuditSign         AuditAction = "sign"
	AuditVerify       AuditAction = "verify"
	AuditMigrate      AuditAction = "migrate"
	AuditExport       AuditAction = "export"
	AuditImport       AuditAction = "import"
)

// SecurityLevel 安全等级
type SecurityLevel int

const (
	SecurityLevel1 SecurityLevel = 1 // 128-bit classical security
	SecurityLevel3 SecurityLevel = 3 // 192-bit classical security
	SecurityLevel5 SecurityLevel = 5 // 256-bit classical security
)

// QuantumKey 量子安全密钥
type QuantumKey struct {
	ID            string         `json:"id"`
	Name          string         `json:"name" binding:"required"`
	Algorithm     Algorithm      `json:"algorithm" binding:"required"`
	SecurityLevel SecurityLevel  `json:"security_level"`
	Status        KeyStatus      `json:"status"`
	PublicKey     []byte         `json:"public_key,omitempty"`
	PrivateKey    []byte         `json:"private_key,omitempty"`
	KeySize       int            `json:"key_size"`
	IsHybrid      bool           `json:"is_hybrid"`
	AlgorithmPair *AlgorithmPair `json:"algorithm_pair,omitempty"`
	ExpiresAt     time.Time      `json:"expires_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	RotatedFrom   string         `json:"rotated_from,omitempty"`
	UsageCount    int64          `json:"usage_count"`
	MaxUsage      int64          `json:"max_usage,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// AlgorithmPair 算法对（用于混合加密）
type AlgorithmPair struct {
	PostQuantum Algorithm `json:"post_quantum"`
	Classical   Algorithm `json:"classical"`
}

// HybridCipher 混合加密器
type HybridCipher struct {
	ID              string        `json:"id"`
	Name            string        `json:"name" binding:"required"`
	Mode            CipherMode    `json:"mode" binding:"required"`
	Algorithm       Algorithm     `json:"algorithm" binding:"required"`
	ClassicalAlgo   Algorithm     `json:"classical_algo,omitempty"`
	KeyID           string        `json:"key_id" binding:"required"`
	KeySize         int           `json:"key_size"`
	BlockSize       int           `json:"block_size"`
	IsAuthenticated bool          `json:"is_authenticated"`
	IsActive        bool          `json:"is_active"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	Config          *CipherConfig `json:"config,omitempty"`
}

// CipherConfig 加密配置
type CipherConfig struct {
	IVSize        int    `json:"iv_size"`
	TagSize       int    `json:"tag_size"`
	HashAlgorithm string `json:"hash_algorithm"`
	KDFAlgorithm  string `json:"kdf_algorithm"`
	Iterations    int    `json:"iterations"`
}

// MigrationPlan 迁移计划
type MigrationPlan struct {
	ID              string            `json:"id"`
	Name            string            `json:"name" binding:"required"`
	Description     string            `json:"description,omitempty"`
	Status          MigrationStatus   `json:"status"`
	SourceAlgorithm Algorithm         `json:"source_algorithm"`
	TargetAlgorithm Algorithm         `json:"target_algorithm"`
	SourceKeyID     string            `json:"source_key_id"`
	TargetKeyID     string            `json:"target_key_id,omitempty"`
	TotalResources  int               `json:"total_resources"`
	MigratedCount   int               `json:"migrated_count"`
	FailedCount     int               `json:"failed_count"`
	Progress        float64           `json:"progress"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	EstimatedEnd    *time.Time        `json:"estimated_end,omitempty"`
	Resources       []MigrationResource `json:"resources,omitempty"`
	Errors          []MigrationError  `json:"errors,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// MigrationResource 迁移资源
type MigrationResource struct {
	ResourceID   string          `json:"resource_id"`
	ResourceType string          `json:"resource_type"`
	Status       MigrationStatus `json:"status"`
	Error        string          `json:"error,omitempty"`
	MigratedAt   *time.Time      `json:"migrated_at,omitempty"`
}

// MigrationError 迁移错误
type MigrationError struct {
	ResourceID string    `json:"resource_id"`
	Error      string    `json:"error"`
	Timestamp  time.Time `json:"timestamp"`
}

// CryptoAudit 加密审计
type CryptoAudit struct {
	ID        string      `json:"id"`
	Action    AuditAction `json:"action"`
	KeyID     string      `json:"key_id,omitempty"`
	Algorithm Algorithm   `json:"algorithm,omitempty"`
	Resource  string      `json:"resource,omitempty"`
	UserID    string      `json:"user_id,omitempty"`
	SourceIP  string      `json:"source_ip,omitempty"`
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// EncryptRequest 加密请求
type EncryptRequest struct {
	Plaintext []byte    `json:"plaintext" binding:"required"`
	KeyID     string    `json:"key_id" binding:"required"`
	Algorithm Algorithm `json:"algorithm,omitempty"`
	Mode      CipherMode `json:"mode,omitempty"`
	AAD       []byte    `json:"aad,omitempty"` // Additional Authenticated Data
}

// EncryptResponse 加密响应
type EncryptResponse struct {
	Ciphertext []byte    `json:"ciphertext"`
	IV         []byte    `json:"iv,omitempty"`
	Tag        []byte    `json:"tag,omitempty"`
	KeyID      string    `json:"key_id"`
	Algorithm  Algorithm `json:"algorithm"`
	Mode       CipherMode `json:"mode"`
}

// DecryptRequest 解密请求
type DecryptRequest struct {
	Ciphertext []byte `json:"ciphertext" binding:"required"`
	KeyID      string `json:"key_id" binding:"required"`
	IV         []byte `json:"iv,omitempty"`
	Tag        []byte `json:"tag,omitempty"`
	AAD        []byte `json:"aad,omitempty"`
}

// DecryptResponse 解密响应
type DecryptResponse struct {
	Plaintext []byte    `json:"plaintext"`
	KeyID     string    `json:"key_id"`
	Algorithm Algorithm `json:"algorithm"`
}

// SignRequest 签名请求
type SignRequest struct {
	Message   []byte    `json:"message" binding:"required"`
	KeyID     string    `json:"key_id" binding:"required"`
	Algorithm Algorithm `json:"algorithm,omitempty"`
}

// SignResponse 签名响应
type SignResponse struct {
	Signature []byte    `json:"signature"`
	KeyID     string    `json:"key_id"`
	Algorithm Algorithm `json:"algorithm"`
}

// VerifyRequest 验证请求
type VerifyRequest struct {
	Message   []byte `json:"message" binding:"required"`
	Signature []byte `json:"signature" binding:"required"`
	KeyID     string `json:"key_id" binding:"required"`
}

// VerifyResponse 验证响应
type VerifyResponse struct {
	Valid     bool      `json:"valid"`
	KeyID     string    `json:"key_id"`
	Algorithm Algorithm `json:"algorithm"`
}

// KeyRotationRequest 密钥轮换请求
type KeyRotationRequest struct {
	KeyID          string    `json:"key_id" binding:"required"`
	NewAlgorithm   Algorithm `json:"new_algorithm,omitempty"`
	RetainOldKey   bool      `json:"retain_old_key"`
	MigrateData    bool      `json:"migrate_data"`
	ExpirationDays int       `json:"expiration_days,omitempty"`
}

// CryptoStats 加密统计
type CryptoStats struct {
	TotalKeys       int64            `json:"total_keys"`
	ActiveKeys      int64            `json:"active_keys"`
	ExpiredKeys     int64            `json:"expired_keys"`
	TotalOperations int64            `json:"total_operations"`
	EncryptOps      int64            `json:"encrypt_ops"`
	DecryptOps      int64            `json:"decrypt_ops"`
	SignOps         int64            `json:"sign_ops"`
	VerifyOps       int64            `json:"verify_ops"`
	ByAlgorithm     map[Algorithm]int64 `json:"by_algorithm"`
	ByMode          map[CipherMode]int64 `json:"by_mode"`
	MigrationsTotal  int64           `json:"migrations_total"`
	MigrationsActive int64           `json:"migrations_active"`
}

// QuantumSafeConfig 量子安全模块配置
type QuantumSafeConfig struct {
	Enabled              bool           `json:"enabled"`
	DefaultAlgorithm     Algorithm      `json:"default_algorithm"`
	DefaultSecurityLevel SecurityLevel  `json:"default_security_level"`
	HybridMode           bool           `json:"hybrid_mode"`
	ClassicalAlgorithm   Algorithm      `json:"classical_algorithm"`
	KeyRotationDays      int            `json:"key_rotation_days"`
	MaxKeyUsage          int64          `json:"max_key_usage"`
	AuditEnabled         bool           `json:"audit_enabled"`
	MigrationEnabled     bool           `json:"migration_enabled"`
	AutoMigrate          bool           `json:"auto_migrate"`
	BackupKeys           bool           `json:"backup_keys"`
	BackupPath           string         `json:"backup_path"`
}

// DefaultQuantumSafeConfig 默认配置
func DefaultQuantumSafeConfig() *QuantumSafeConfig {
	return &QuantumSafeConfig{
		Enabled:              true,
		DefaultAlgorithm:     Kyber768,
		DefaultSecurityLevel: SecurityLevel3,
		HybridMode:           true,
		ClassicalAlgorithm:   AlgorithmClassic,
		KeyRotationDays:      90,
		MaxKeyUsage:          1000000,
		AuditEnabled:         true,
		MigrationEnabled:     true,
		AutoMigrate:          false,
		BackupKeys:           true,
		BackupPath:           "/var/backup/quantum-keys",
	}
}

// SupportedAlgorithms 获取支持的算法列表
func SupportedAlgorithms() []Algorithm {
	return []Algorithm{
		Kyber768,
		Kyber1024,
		Dilithium3,
		Dilithium5,
		HybridKyber768,
		HybridDilithium3,
		AlgorithmClassic,
		AlgorithmX25519,
		AlgorithmEd25519,
	}
}

// AlgorithmInfo 算法信息
type AlgorithmInfo struct {
	Algorithm     Algorithm     `json:"algorithm"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Type          string        `json:"type"` // KEM, Signature, Symmetric
	SecurityLevel SecurityLevel `json:"security_level"`
	KeySize       int           `json:"key_size"`
	IsQuantumSafe bool          `json:"is_quantum_safe"`
	IsHybridReady bool          `json:"is_hybrid_ready"`
}

// GetAlgorithmInfo 获取算法信息
func GetAlgorithmInfo(algo Algorithm) *AlgorithmInfo {
	algorithms := map[Algorithm]*AlgorithmInfo{
		Kyber768: {
			Algorithm:     Kyber768,
			Name:          "CRYSTALS-Kyber-768",
			Description:   "基于格的密钥封装机制 (KEM)，NIST Level 3",
			Type:          "KEM",
			SecurityLevel: SecurityLevel3,
			KeySize:       3168,
			IsQuantumSafe: true,
			IsHybridReady: true,
		},
		Kyber1024: {
			Algorithm:     Kyber1024,
			Name:          "CRYSTALS-Kyber-1024",
			Description:   "基于格的密钥封装机制 (KEM)，NIST Level 5",
			Type:          "KEM",
			SecurityLevel: SecurityLevel5,
			KeySize:       4224,
			IsQuantumSafe: true,
			IsHybridReady: true,
		},
		Dilithium3: {
			Algorithm:     Dilithium3,
			Name:          "CRYSTALS-Dilithium-3",
			Description:   "基于格的数字签名算法，NIST Level 3",
			Type:          "Signature",
			SecurityLevel: SecurityLevel3,
			KeySize:       1952,
			IsQuantumSafe: true,
			IsHybridReady: true,
		},
		Dilithium5: {
			Algorithm:     Dilithium5,
			Name:          "CRYSTALS-Dilithium-5",
			Description:   "基于格的数字签名算法，NIST Level 5",
			Type:          "Signature",
			SecurityLevel: SecurityLevel5,
			KeySize:       2592,
			IsQuantumSafe: true,
			IsHybridReady: true,
		},
		HybridKyber768: {
			Algorithm:     HybridKyber768,
			Name:          "Hybrid X25519+Kyber-768",
			Description:   "混合密钥封装：X25519 + Kyber-768",
			Type:          "KEM",
			SecurityLevel: SecurityLevel3,
			KeySize:       3200,
			IsQuantumSafe: true,
			IsHybridReady: true,
		},
		HybridDilithium3: {
			Algorithm:     HybridDilithium3,
			Name:          "Hybrid Ed25519+Dilithium-3",
			Description:   "混合签名：Ed25519 + Dilithium-3",
			Type:          "Signature",
			SecurityLevel: SecurityLevel3,
			KeySize:       2016,
			IsQuantumSafe: true,
			IsHybridReady: true,
		},
		AlgorithmClassic: {
			Algorithm:     AlgorithmClassic,
			Name:          "AES-256-GCM",
			Description:   "经典对称加密算法",
			Type:          "Symmetric",
			SecurityLevel: SecurityLevel5,
			KeySize:       32,
			IsQuantumSafe: false,
			IsHybridReady: true,
		},
		AlgorithmX25519: {
			Algorithm:     AlgorithmX25519,
			Name:          "X25519",
			Description:   "椭圆曲线 Diffie-Hellman 密钥交换",
			Type:          "KEM",
			SecurityLevel: SecurityLevel1,
			KeySize:       32,
			IsQuantumSafe: false,
			IsHybridReady: true,
		},
		AlgorithmEd25519: {
			Algorithm:     AlgorithmEd25519,
			Name:          "Ed25519",
			Description:   "椭圆曲线数字签名算法",
			Type:          "Signature",
			SecurityLevel: SecurityLevel1,
			KeySize:       32,
			IsQuantumSafe: false,
			IsHybridReady: true,
		},
	}

	if info, ok := algorithms[algo]; ok {
		return info
	}
	return &AlgorithmInfo{
		Algorithm: algo,
		Name:      string(algo),
	}
}
