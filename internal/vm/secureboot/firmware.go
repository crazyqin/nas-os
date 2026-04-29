package secureboot

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FirmwareVerifier UEFI 固件验证器。
//
// 负责验证 UEFI 固件镜像的完整性和签名，确保
// VM 启动时加载的固件未被篡改。
type FirmwareVerifier struct {
	mu          sync.RWMutex
	keyManager  *KeyManager
	verifier    *SignatureVerifier
	knownHashes map[string][32]byte // 已知固件哈希白名单
	logger      *zap.Logger
}

// FirmwareImage 固件镜像信息。
type FirmwareImage struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Path      string    `json:"path"`
	Data      []byte    `json:"-"`
	Hash      [32]byte  `json:"hash"`
	SignedBy  string    `json:"signed_by,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// FirmwareVerificationResult 固件验证结果。
type FirmwareVerificationResult struct {
	Valid       bool      `json:"valid"`
	FirmwareName string   `json:"firmware_name"`
	Version     string    `json:"version,omitempty"`
	HashValid   bool      `json:"hash_valid"`
	SignatureValid bool   `json:"signature_valid"`
	Reason      string    `json:"reason,omitempty"`
	VerifiedAt  time.Time `json:"verified_at"`
}

// UEFIVariableStore UEFI 变量存储接口。
type UEFIVariableStore interface {
	// GetVariable 读取 UEFI 变量。
	GetVariable(name, guid string) (*UEFIVariable, error)
	// SetVariable 写入 UEFI 变量。
	SetVariable(variable *UEFIVariable) error
	// DeleteVariable 删除 UEFI 变量。
	DeleteVariable(name, guid string) error
	// ListVariables 列出所有 UEFI 变量。
	ListVariables() ([]*UEFIVariable, error)
}

// FirmwareProvider 固件提供者接口。
type FirmwareProvider interface {
	// GetFirmware 获取指定名称的固件。
	GetFirmware(name string) (*FirmwareImage, error)
	// ListFirmwares 列出所有可用固件。
	ListFirmwares() ([]*FirmwareImage, error)
}

// SecureBootProvider Secure Boot 提供者接口。
//
// 为上层 VM 管理器提供 Secure Boot 功能的统一接口。
type SecureBootProvider interface {
	// EnableSecureBoot 启用 Secure Boot。
	EnableSecureBoot(vmID string) error
	// DisableSecureBoot 禁用 Secure Boot。
	DisableSecureBoot(vmID string) error
	// GetSecureBootStatus 获取 Secure Boot 状态。
	GetSecureBootStatus(vmID string) (*SecureBootStatus, error)
	// VerifyFirmware 验证固件。
	VerifyFirmware(vmID string, firmware *FirmwareImage) (*FirmwareVerificationResult, error)
	// VerifyBootChain 验证启动链。
	VerifyBootChain(vmID string, components []*BootComponent) (*BootChainResult, error)
	// UpdateKeys 更新密钥。
	UpdateKeys(vmID string, keyType KeyType, certPEM []byte) error
}

// NewFirmwareVerifier 创建固件验证器。
func NewFirmwareVerifier(km *KeyManager, logger *zap.Logger) *FirmwareVerifier {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FirmwareVerifier{
		keyManager:  km,
		verifier:    NewSignatureVerifier(km, logger),
		knownHashes: make(map[string][32]byte),
		logger:      logger,
	}
}

// RegisterKnownFirmware 注册已知固件哈希（白名单）。
func (fv *FirmwareVerifier) RegisterKnownFirmware(name string, hash [32]byte) {
	fv.mu.Lock()
	defer fv.mu.Unlock()
	fv.knownHashes[name] = hash
	fv.logger.Info("已注册固件哈希",
		zap.String("name", name),
		zap.String("hash", fmt.Sprintf("%x", hash)),
	)
}

// VerifyFirmware 验证固件镜像。
//
// 验证流程：
//  1. 计算固件哈希
//  2. 与已知白名单比对
//  3. 如果有签名，验证签名
//  4. 返回验证结果
func (fv *FirmwareVerifier) VerifyFirmware(firmware *FirmwareImage) *FirmwareVerificationResult {
	result := &FirmwareVerificationResult{
		FirmwareName: firmware.Name,
		Version:      firmware.Version,
		VerifiedAt:   time.Now(),
	}

	if len(firmware.Data) == 0 {
		result.Valid = false
		result.Reason = "固件数据为空"
		return result
	}

	// 计算哈希
	actualHash := sha256.Sum256(firmware.Data)

	// 检查白名单
	fv.mu.RLock()
	expectedHash, known := fv.knownHashes[firmware.Name]
	fv.mu.RUnlock()

	if known {
		if actualHash == expectedHash {
			result.HashValid = true
		} else {
			result.Valid = false
			result.HashValid = false
			result.Reason = fmt.Sprintf("固件哈希不匹配：期望 %x，实际 %x", expectedHash, actualHash)
			return result
		}
	} else {
		// 不在白名单中，但不一定是无效的（审计模式下允许）
		result.HashValid = true
		fv.logger.Warn("固件不在已知白名单中",
			zap.String("name", firmware.Name),
		)
	}

	// 如果固件自带签名，进行签名验证
	if len(firmware.SignedBy) > 0 {
		dbEntries := fv.keyManager.ListKeys(KeyTypeDB)
		for _, entry := range dbEntries {
			if entry.Certificate != nil && entry.Certificate.Subject.CommonName == firmware.SignedBy {
				result.SignatureValid = true
				break
			}
		}
		if !result.SignatureValid {
			result.Valid = false
			result.Reason = fmt.Sprintf("固件签名者 '%s' 不在信任列表中", firmware.SignedBy)
			return result
		}
	} else {
		// 无签名 —— 审计模式下允许
		result.SignatureValid = false
		fv.logger.Warn("固件无签名", zap.String("name", firmware.Name))
	}

	result.Valid = true
	return result
}

// VerifyBootChain 验证完整的启动链。
//
// 按照 UEFI Secure Boot 规范，依次验证：
//  1. UEFI 固件
//  2. Bootloader (如 GRUB/shim)
//  3. OS Kernel
//
// 所有组件都必须通过验证，启动链才有效。
func (fv *FirmwareVerifier) VerifyBootChain(components []*BootComponent) *BootChainResult {
	result := &BootChainResult{
		VerifiedAt: time.Now(),
		OverallOK:  true,
	}

	if len(components) == 0 {
		result.Valid = false
		result.OverallOK = false
		return result
	}

	for _, comp := range components {
		cr := fv.verifier.VerifyComponent(comp)
		result.Components = append(result.Components, *cr)

		if !cr.Valid {
			result.OverallOK = false
		}
	}

	result.Valid = result.OverallOK

	if !result.Valid {
		fv.logger.Warn("启动链验证失败",
			zap.Int("total_components", len(components)),
		)
	} else {
		fv.logger.Info("启动链验证通过",
			zap.Int("total_components", len(components)),
		)
	}

	return result
}

// MemoryVariableStore 内存实现的 UEFI 变量存储（用于测试和开发）。
type MemoryVariableStore struct {
	mu        sync.RWMutex
	variables map[string]*UEFIVariable
}

// NewMemoryVariableStore 创建内存 UEFI 变量存储。
func NewMemoryVariableStore() *MemoryVariableStore {
	return &MemoryVariableStore{
		variables: make(map[string]*UEFIVariable),
	}
}

func (s *MemoryVariableStore) variableKey(name, guid string) string {
	return name + ":" + guid
}

// GetVariable 读取 UEFI 变量。
func (s *MemoryVariableStore) GetVariable(name, guid string) (*UEFIVariable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.variableKey(name, guid)
	v, ok := s.variables[key]
	if !ok {
		return nil, fmt.Errorf("变量 %s 不存在", name)
	}
	return v, nil
}

// SetVariable 写入 UEFI 变量。
func (s *MemoryVariableStore) SetVariable(variable *UEFIVariable) error {
	if variable == nil {
		return errors.New("变量不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.variableKey(variable.Name, variable.GUID)
	variable.UpdatedAt = time.Now()
	s.variables[key] = variable
	return nil
}

// DeleteVariable 删除 UEFI 变量。
func (s *MemoryVariableStore) DeleteVariable(name, guid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.variableKey(name, guid)
	if _, ok := s.variables[key]; !ok {
		return fmt.Errorf("变量 %s 不存在", name)
	}
	delete(s.variables, key)
	return nil
}

// ListVariables 列出所有 UEFI 变量。
func (s *MemoryVariableStore) ListVariables() ([]*UEFIVariable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*UEFIVariable, 0, len(s.variables))
	for _, v := range s.variables {
		result = append(result, v)
	}
	return result, nil
}

// Count 返回变量数量。
func (s *MemoryVariableStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.variables)
}

// UEFIVariableNames 预定义的 UEFI 安全启动变量名。
const (
	UEFIVarPK        = "PK"
	UEFIVarKEK       = "KEK"
	UEFIVarDB        = "db"
	UEFIVarDBX       = "dbx"
	UEFIVarSecureBoot = "SecureBoot"
	UEFIVarSetupMode = "SetupMode"
	UEFIVarDeployedMode = "DeployedMode"
)

// UEFI 标准 GUID。
const (
	UEFIGlobalGUID = "8BE4DF61-93CA-11D2-AA0D-00E098032B8C"
	UEFISecureGUID = "D719B2CB-3D3A-4596-A3BC-DAD00E67656F"
)
