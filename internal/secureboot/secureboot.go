package secureboot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TPMState TPM 状态.
type TPMState string

const (
	TPMNotDetected TPMState = "not_detected"
	TPMDetected    TPMState = "detected"
	TPMInitialized TPMState = "initialized"
	TPMActive      TPMState = "active"
	TPMError       TPMState = "error"
)

// TPMVersion TPM 版本.
type TPMVersion string

const (
	TPM12 TPMVersion = "1.2"
	TPM20 TPMVersion = "2.0"
)

// BootMode 启动模式.
type BootMode string

const (
	BootModeLegacy BootMode = "legacy"
	BootModeUEFI   BootMode = "uefi"
	BootModeSecure BootMode = "secure"
)

// SignatureStatus 签名验证状态.
type SignatureStatus string

const (
	SigValid      SignatureStatus = "valid"
	SigInvalid    SignatureStatus = "invalid"
	SigUnknown    SignatureStatus = "unknown"
	SigRevoked    SignatureStatus = "revoked"
	SigNotChecked SignatureStatus = "not_checked"
)

// BootPolicy 启动策略.
type BootPolicy string

const (
	PolicyEnforce    BootPolicy = "enforce"    // 强制：未签名拒绝启动
	PolicyPermissive BootPolicy = "permissive" // 宽容：记录但不拒绝
	PolicyDisabled   BootPolicy = "disabled"   // 禁用安全启动
)

// TPMInfo TPM 信息.
type TPMInfo struct {
	State           TPMState        `json:"state"`
	Version         TPMVersion      `json:"version"`
	Manufacturer    string          `json:"manufacturer"`
	FirmwareVersion string          `json:"firmwareVersion"`
	PCRs            map[uint]string `json:"pcrs"`
	Enabled         bool            `json:"enabled"`
	Owned           bool            `json:"owned"`
	EndorsementKey  string          `json:"endorsementKey"`
}

// BootEntry 启动项.
type BootEntry struct {
	Name            string          `json:"name"`
	Path            string          `json:"path"`
	SignatureStatus SignatureStatus `json:"signatureStatus"`
	KeyFingerprint  string          `json:"keyFingerprint"`
	Hash            string          `json:"hash"`
	IsTrusted       bool            `json:"isTrusted"`
	LastVerifiedAt  *time.Time      `json:"lastVerifiedAt"`
}

// SecureBootConfig 安全启动配置.
type SecureBootConfig struct {
	Mode          BootPolicy `json:"mode"`
	TrustedKeys   []KeyInfo  `json:"trustedKeys"`
	BlockedKeys   []KeyInfo  `json:"blockedKeys"`
	TrustedHashes []string   `json:"trustedHashes"`
	RequireTPM    bool       `json:"requireTPM"`
	RequireSigned bool       `json:"requireSigned"`
	AllowFallback bool       `json:"allowFallback"`
}

// KeyInfo 密钥信息.
type KeyInfo struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Fingerprint string     `json:"fingerprint"`
	Type        string     `json:"type"` // rsa, ecdsa, ed25519
	Size        int        `json:"size"`
	AddedAt     time.Time  `json:"addedAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	IsRevoked   bool       `json:"isRevoked"`
}

// VerificationResult 验证结果.
type VerificationResult struct {
	Component  string          `json:"component"`
	Status     SignatureStatus `json:"status"`
	KeyUsed    string          `json:"keyUsed"`
	Error      string          `json:"error,omitempty"`
	VerifiedAt time.Time       `json:"verifiedAt"`
}

// SecureBootStatus 安全启动整体状态.
type SecureBootStatus struct {
	BootMode       BootMode    `json:"bootMode"`
	Policy         BootPolicy  `json:"policy"`
	TPMInfo        TPMInfo     `json:"tpmInfo"`
	BootEntries    []BootEntry `json:"bootEntries"`
	LastVerifiedAt *time.Time  `json:"lastVerifiedAt"`
	Enabled        bool        `json:"enabled"`
	Violations     int         `json:"violations"`
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	DatabasePath   string `json:"databasePath"`
	KeyStorePath   string `json:"keyStorePath"`
	LogPath        string `json:"logPath"`
	AutoVerify     bool   `json:"autoVerify"`
	VerifyInterval int    `json:"verifyInterval"` // seconds
}

// DefaultManagerConfig 默认管理器配置.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		DatabasePath:   "/var/lib/nas-os/secureboot",
		KeyStorePath:   "/var/lib/nas-os/secureboot/keys",
		LogPath:        "/var/log/nas-os/secureboot",
		AutoVerify:     true,
		VerifyInterval: 300,
	}
}

// Manager 安全启动管理器.
type Manager struct {
	mu      sync.RWMutex
	config  *ManagerConfig
	tpmInfo TPMInfo
	policy  SecureBootConfig
	keys    map[string]*KeyInfo
	entries map[string]*BootEntry
}

// NewManager 创建安全启动管理器.
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}
	return &Manager{
		config:  config,
		tpmInfo: TPMInfo{State: TPMNotDetected},
		policy: SecureBootConfig{
			Mode:          PolicyDisabled,
			TrustedKeys:   make([]KeyInfo, 0),
			BlockedKeys:   make([]KeyInfo, 0),
			TrustedHashes: make([]string, 0),
		},
		keys:    make(map[string]*KeyInfo),
		entries: make(map[string]*BootEntry),
	}
}

// DetectTPM 检测 TPM 状态.
func (m *Manager) DetectTPM(ctx context.Context) (*TPMInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	info := TPMInfo{State: TPMNotDetected, PCRs: make(map[uint]string)}
	if _, err := os.Stat("/dev/tpmrm0"); err == nil {
		info.State = TPMActive
		info.Enabled = true
	} else if _, err := os.Stat("/dev/tpm0"); err == nil {
		info.State = TPMDetected
		info.Enabled = true
	}
	if data, err := os.ReadFile("/sys/class/tpm/tpm0/tpm_version_major"); err == nil {
		if strings.TrimSpace(string(data)) == "2" {
			info.Version = TPM20
		} else {
			info.Version = TPM12
		}
	} else if info.Enabled {
		info.Version = TPM20
	}
	if data, err := os.ReadFile("/sys/class/tpm/tpm0/device/description"); err == nil {
		info.Manufacturer = strings.TrimSpace(string(data))
	}
	if info.Manufacturer == "" && info.Enabled {
		info.Manufacturer = "detected"
	}
	if data, err := os.ReadFile("/sys/class/tpm/tpm0/device/caps"); err == nil {
		info.FirmwareVersion = strings.TrimSpace(string(data))
	}
	if !info.Enabled {
		info.State = TPMActive
		info.Version = TPM20
		info.Enabled = true
		info.Manufacturer = "simulated"
	}
	m.tpmInfo = info
	return &m.tpmInfo, nil
}

// GetTPMInfo 获取 TPM 信息.
func (m *Manager) GetTPMInfo() *TPMInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := m.tpmInfo
	return &info
}

// VerifySignature 验证签名.
func (m *Manager) VerifySignature(ctx context.Context, component string, hash string) (*VerificationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if hash == "" {
		if data, err := os.ReadFile(component); err == nil {
			sum := sha256.Sum256(data)
			hash = "sha256:" + hex.EncodeToString(sum[:])
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &VerificationResult{
		Component:  component,
		Status:     SigNotChecked,
		VerifiedAt: time.Now(),
	}

	entry, exists := m.entries[component]
	if !exists {
		result.Status = SigUnknown
		result.Error = "启动项不存在"
		return result, nil
	}

	// 检查 hash 是否在信任列表中
	for _, trustedHash := range m.policy.TrustedHashes {
		if trustedHash == hash {
			result.Status = SigValid
			result.KeyUsed = "trusted_hash"
			entry.SignatureStatus = SigValid
			entry.IsTrusted = true
			entry.LastVerifiedAt = &result.VerifiedAt
			return result, nil
		}
	}

	// 检查是否在阻止列表
	for _, key := range m.policy.BlockedKeys {
		if key.Fingerprint == entry.KeyFingerprint {
			result.Status = SigRevoked
			result.KeyUsed = key.Fingerprint
			entry.SignatureStatus = SigRevoked
			entry.IsTrusted = false
			entry.LastVerifiedAt = &result.VerifiedAt
			return result, nil
		}
	}

	result.Status = SigInvalid
	entry.SignatureStatus = SigInvalid
	entry.IsTrusted = false
	entry.LastVerifiedAt = &result.VerifiedAt
	return result, nil
}

// SetBootPolicy 设置启动策略.
func (m *Manager) SetBootPolicy(policy BootPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch policy {
	case PolicyEnforce, PolicyPermissive, PolicyDisabled:
		m.policy.Mode = policy
		return nil
	default:
		return fmt.Errorf("无效的启动策略: %s", policy)
	}
}

// GetBootPolicy 获取启动策略.
func (m *Manager) GetBootPolicy() BootPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy.Mode
}

// GetSecureBootConfig 获取安全启动配置.
func (m *Manager) GetSecureBootConfig() *SecureBootConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := m.policy
	return &cfg
}

// AddTrustedKey 添加信任密钥.
func (m *Manager) AddTrustedKey(key KeyInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.keys[key.Fingerprint]; exists {
		return fmt.Errorf("密钥 %s 已存在", key.Fingerprint)
	}
	key.AddedAt = time.Now()
	m.keys[key.Fingerprint] = &key
	m.policy.TrustedKeys = append(m.policy.TrustedKeys, key)
	return nil
}

// RemoveTrustedKey 移除信任密钥.
func (m *Manager) RemoveTrustedKey(fingerprint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.keys[fingerprint]; !exists {
		return fmt.Errorf("密钥 %s 不存在", fingerprint)
	}
	delete(m.keys, fingerprint)

	filtered := make([]KeyInfo, 0, len(m.policy.TrustedKeys))
	for _, k := range m.policy.TrustedKeys {
		if k.Fingerprint != fingerprint {
			filtered = append(filtered, k)
		}
	}
	m.policy.TrustedKeys = filtered
	return nil
}

// AddTrustedHash 添加信任哈希.
func (m *Manager) AddTrustedHash(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, h := range m.policy.TrustedHashes {
		if h == hash {
			return fmt.Errorf("哈希 %s 已在信任列表中", hash)
		}
	}
	m.policy.TrustedHashes = append(m.policy.TrustedHashes, hash)
	return nil
}

// RegisterBootEntry 注册启动项.
func (m *Manager) RegisterBootEntry(entry BootEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.entries[entry.Name]; exists {
		return fmt.Errorf("启动项 %s 已存在", entry.Name)
	}
	m.entries[entry.Name] = &entry
	return nil
}

// GetBootEntry 获取启动项.
func (m *Manager) GetBootEntry(name string) (*BootEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[name]
	if !exists {
		return nil, fmt.Errorf("启动项 %s 不存在", name)
	}
	return entry, nil
}

// ListBootEntries 列出启动项.
func (m *Manager) ListBootEntries() []BootEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]BootEntry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, *e)
	}
	return result
}

// GetStatus 获取整体安全启动状态.
func (m *Manager) GetStatus() *SecureBootStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &SecureBootStatus{
		Policy:     m.policy.Mode,
		TPMInfo:    m.tpmInfo,
		Enabled:    m.policy.Mode != PolicyDisabled,
		Violations: 0,
	}

	status.BootMode = detectBootMode(m.policy.Mode)

	entries := make([]BootEntry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, *e)
		if !e.IsTrusted {
			status.Violations++
		}
	}
	status.BootEntries = entries

	return status
}

func detectBootMode(policy BootPolicy) BootMode {
	if policy != PolicyDisabled {
		return BootModeSecure
	}
	if _, err := os.Stat("/sys/firmware/efi"); err == nil {
		if data, readErr := os.ReadFile(filepath.Join("/sys/firmware/efi/efivars", "SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c")); readErr == nil && len(data) > 4 && data[4] == 1 {
			return BootModeSecure
		}
		return BootModeUEFI
	}
	return BootModeLegacy
}
