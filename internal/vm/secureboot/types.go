// Package secureboot 实现 UEFI Secure Boot 核心模块。
//
// 提供 Secure Boot 密钥管理 (PK/KEK/db/dbx)、证书链验证、
// UEFI 固件验证接口等功能，对标 TrueNAS 26 Secure Boot 特性。
package secureboot

import (
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"sync"
	"time"
)

// SecureBootMode 安全启动模式。
type SecureBootMode string

const (
	// ModeStrict 严格模式：所有启动组件必须通过签名验证。
	ModeStrict SecureBootMode = "strict"
	// ModeStandard 标准模式：使用默认策略验证。
	ModeStandard SecureBootMode = "standard"
	// ModeAudit 审计模式：仅记录验证失败，不阻止启动。
	ModeAudit SecureBootMode = "audit"
)

// SecureBootState 安全启动状态。
type SecureBootState string

const (
	StateEnabled  SecureBootState = "enabled"
	StateDisabled SecureBootState = "disabled"
	StateSetup    SecureBootState = "setup"
)

// KeyType UEFI 密钥类型。
type KeyType string

const (
	// KeyTypePK 平台密钥 (Platform Key) — 根信任锚。
	KeyTypePK KeyType = "PK"
	// KeyTypeKEK 密钥交换密钥 (Key Exchange Key) — 签名 db/dbx 更新。
	KeyTypeKEK KeyType = "KEK"
	// KeyTypeDB 签名数据库 (Signature Database) — 允许的签名。
	KeyTypeDB KeyType = "db"
	// KeyTypeDBX 撤销数据库 (Forbidden Signatures Database) — 禁止的签名。
	KeyTypeDBX KeyType = "dbx"
)

// SignatureType 签名类型。
type SignatureType string

const (
	// SigTypeX509 X.509 证书签名。
	SigTypeX509 SignatureType = "x509"
	// SigTypeSHA256 SHA256 哈希签名。
	SigTypeSHA256 SignatureType = "sha256"
	// SigTypePKCS7 PKCS#7 签名。
	SigTypePKCS7 SignatureType = "pkcs7"
)

// SecureBootConfig 安全启动配置。
type SecureBootConfig struct {
	Enabled         bool            `json:"enabled"`
	Mode            SecureBootMode  `json:"mode"`
	AllowedKeys     []string        `json:"allowed_keys"`
	SecureBootState SecureBootState `json:"secure_boot_state"`
	TPMEnabled      bool            `json:"tpm_enabled"`
}

// SecureBootPolicy 安全启动策略。
type SecureBootPolicy struct {
	EnforceKernelSignature bool `json:"enforce_kernel_signature"`
	EnforceModuleSignature bool `json:"enforce_module_signature"`
	AllowCustomKeys        bool `json:"allow_custom_keys"`
	AuditMode              bool `json:"audit_mode"`
}

// SecureBootStatus 安全启动状态（API 响应）。
type SecureBootStatus struct {
	Enabled        bool   `json:"enabled"`
	State          string `json:"state"`
	LastBootSecure bool   `json:"last_boot_secure"`
	TPMPresent     bool   `json:"tpm_present"`
	KeyCount       int    `json:"key_count"`
}

// UEFIVariable UEFI 安全变量。
type UEFIVariable struct {
	Name       string    `json:"name"`
	GUID       string    `json:"guid"`
	Attributes uint32    `json:"attributes"`
	Data       []byte    `json:"data"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// KeyEntry 密钥条目。
type KeyEntry struct {
	Type        SignatureType     `json:"type"`
	KeyType     KeyType           `json:"key_type"`
	Certificate *x509.Certificate `json:"-"`
	Hash        [32]byte          `json:"hash"`
	Description string            `json:"description"`
	OwnerGUID   string            `json:"owner_guid"`
	AddedAt     time.Time         `json:"added_at"`
	RevokedAt   *time.Time        `json:"revoked_at,omitempty"`
}

// SignatureEntry 签名数据库条目。
type SignatureEntry struct {
	Type      SignatureType     `json:"type"`
	OwnerGUID string            `json:"owner_guid"`
	Cert      *x509.Certificate `json:"-"`
	Hash      [32]byte          `json:"hash"`
}

// VerificationResult 验证结果。
type VerificationResult struct {
	Valid       bool      `json:"valid"`
	Reason      string    `json:"reason,omitempty"`
	Chain       []string  `json:"chain,omitempty"`
	VerifiedAt  time.Time `json:"verified_at"`
	SignerCN    string    `json:"signer_cn,omitempty"`
	TrustedRoot bool      `json:"trusted_root"`
}

// BootComponent 启动组件。
type BootComponent struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Data      []byte   `json:"-"`
	Signature []byte   `json:"-"`
	Hash      [32]byte `json:"hash"`
}

// BootChainResult 启动链验证结果。
type BootChainResult struct {
	Valid      bool              `json:"valid"`
	Components []ComponentResult `json:"components"`
	VerifiedAt time.Time         `json:"verified_at"`
	OverallOK  bool              `json:"overall_ok"`
}

// ComponentResult 单个组件验证结果。
type ComponentResult struct {
	Name   string `json:"name"`
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

// ValidationError 验证错误。
type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// DefaultSecureBootConfig 返回默认安全启动配置。
func DefaultSecureBootConfig() SecureBootConfig {
	return SecureBootConfig{
		Enabled:         true,
		Mode:            ModeStandard,
		AllowedKeys:     nil,
		SecureBootState: StateEnabled,
		TPMEnabled:      false,
	}
}

// DefaultSecureBootPolicy 返回默认安全启动策略。
func DefaultSecureBootPolicy() SecureBootPolicy {
	return SecureBootPolicy{
		EnforceKernelSignature: true,
		EnforceModuleSignature: true,
		AllowCustomKeys:        false,
		AuditMode:              false,
	}
}

// keyStore 线程安全的密钥存储。
type keyStore struct {
	mu   sync.RWMutex
	keys map[KeyType][]*KeyEntry
}

func newKeyStore() *keyStore {
	return &keyStore{
		keys: make(map[KeyType][]*KeyEntry),
	}
}

func (ks *keyStore) add(keyType KeyType, entry *KeyEntry) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.keys[keyType] = append(ks.keys[keyType], entry)
}

func (ks *keyStore) list(keyType KeyType) []*KeyEntry {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	entries := ks.keys[keyType]
	result := make([]*KeyEntry, len(entries))
	copy(result, entries)
	return result
}

func (ks *keyStore) remove(keyType KeyType, hash [32]byte) bool {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	entries := ks.keys[keyType]
	for i, e := range entries {
		if e.Hash == hash {
			ks.keys[keyType] = append(entries[:i], entries[i+1:]...)
			return true
		}
	}
	return false
}

func (ks *keyStore) count() int {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	total := 0
	for _, entries := range ks.keys {
		total += len(entries)
	}
	return total
}

func (ks *keyStore) isRevoked(hash [32]byte) bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	for _, entry := range ks.keys[KeyTypeDBX] {
		if entry.Hash == hash && entry.RevokedAt != nil {
			return true
		}
	}
	return false
}

// hashCertificate 计算证书 SHA256 哈希。
func hashCertificate(cert *x509.Certificate) [32]byte {
	return sha256.Sum256(cert.Raw)
}

// hashData 计算数据 SHA256 哈希。
func hashData(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// HashAlgorithm 哈希算法类型。
type HashAlgorithm string

const (
	HashSHA256 HashAlgorithm = "sha256"
	HashSHA384 HashAlgorithm = "sha384"
	HashSHA512 HashAlgorithm = "sha512"
)

// Digest 摘要信息。
type Digest struct {
	Algorithm HashAlgorithm `json:"algorithm"`
	Value     []byte        `json:"value"`
}

// ComputeDigest 计算数据摘要。
func ComputeDigest(data []byte, algo HashAlgorithm) Digest {
	switch algo {
	case HashSHA256:
		h := sha256.Sum256(data)
		return Digest{Algorithm: algo, Value: h[:]}
	case HashSHA384:
		h := crypto.SHA384.New()
		h.Write(data)
		return Digest{Algorithm: algo, Value: h.Sum(nil)}
	case HashSHA512:
		h := crypto.SHA512.New()
		h.Write(data)
		return Digest{Algorithm: algo, Value: h.Sum(nil)}
	default:
		h := sha256.Sum256(data)
		return Digest{Algorithm: HashSHA256, Value: h[:]}
	}
}
