package secureboot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

// Manager Secure Boot 管理器。
//
// 统一管理 Secure Boot 的所有组件：
//   - 密钥管理 (KeyManager)
//   - 签名验证 (SignatureVerifier)
//   - 固件验证 (FirmwareVerifier)
//   - UEFI 变量存储
//   - 安全策略
type Manager struct {
	mu                sync.RWMutex
	config            SecureBootConfig
	policy            SecureBootPolicy
	keyManager        *KeyManager
	signatureVerifier *SignatureVerifier
	firmwareVerifier  *FirmwareVerifier
	varStore          UEFIVariableStore
	stateDir          string
	logger            *zap.Logger
	initialized       bool
}

// ManagerOption 管理器选项。
type ManagerOption func(*Manager)

// WithStateDir 设置状态持久化目录。
func WithStateDir(dir string) ManagerOption {
	return func(m *Manager) {
		m.stateDir = dir
	}
}

// WithConfig 设置初始配置。
func WithConfig(config SecureBootConfig) ManagerOption {
	return func(m *Manager) {
		m.config = config
	}
}

// WithPolicy 设置安全策略。
func WithPolicy(policy SecureBootPolicy) ManagerOption {
	return func(m *Manager) {
		m.policy = policy
	}
}

// WithVariableStore 设置 UEFI 变量存储。
func WithVariableStore(store UEFIVariableStore) ManagerOption {
	return func(m *Manager) {
		m.varStore = store
	}
}

// NewManager 创建 Secure Boot 管理器。
func NewManager(logger *zap.Logger, opts ...ManagerOption) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		config:   DefaultSecureBootConfig(),
		policy:   DefaultSecureBootPolicy(),
		varStore: NewMemoryVariableStore(),
		logger:   logger,
	}

	for _, opt := range opts {
		opt(m)
	}

	km := NewKeyManager(logger)
	m.keyManager = km
	m.signatureVerifier = NewSignatureVerifier(km, logger)
	m.firmwareVerifier = NewFirmwareVerifier(km, logger)

	return m
}

// Initialize 初始化 Secure Boot 管理器。
//
// 执行以下操作：
//  1. 生成平台 CA（如不存在）
//  2. 初始化默认密钥 (PK/KEK/db)
//  3. 持久化状态到磁盘
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return errors.New("管理器已初始化")
	}

	// 尝试从磁盘恢复状态
	if m.stateDir != "" {
		if err := m.loadState(); err != nil {
			m.logger.Info("未找到持久化状态，使用默认初始化", zap.Error(err))
		} else {
			m.initialized = true
			m.logger.Info("从持久化状态恢复 Secure Boot 管理器")
			return nil
		}
	}

	// 初始化默认密钥
	if err := m.keyManager.InitDefaultKeys(); err != nil {
		return fmt.Errorf("初始化默认密钥失败：%w", err)
	}

	// 初始化 UEFI 变量
	if err := m.initUEFIVariables(); err != nil {
		return fmt.Errorf("初始化 UEFI 变量失败：%w", err)
	}

	m.initialized = true

	// 持久化
	if m.stateDir != "" {
		if err := m.saveState(); err != nil {
			m.logger.Warn("持久化状态失败", zap.Error(err))
		}
	}

	m.logger.Info("Secure Boot 管理器已初始化",
		zap.Bool("enabled", m.config.Enabled),
		zap.String("mode", string(m.config.Mode)),
		zap.Int("key_count", m.keyManager.KeyCount()),
	)

	return nil
}

// GetKeyManager 获取密钥管理器。
func (m *Manager) GetKeyManager() *KeyManager {
	return m.keyManager
}

// GetSignatureVerifier 获取签名验证器。
func (m *Manager) GetSignatureVerifier() *SignatureVerifier {
	return m.signatureVerifier
}

// GetFirmwareVerifier 获取固件验证器。
func (m *Manager) GetFirmwareVerifier() *FirmwareVerifier {
	return m.firmwareVerifier
}

// GetVariableStore 获取 UEFI 变量存储。
func (m *Manager) GetVariableStore() UEFIVariableStore {
	return m.varStore
}

// GetConfig 获取当前配置。
func (m *Manager) GetConfig() SecureBootConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置。
func (m *Manager) UpdateConfig(config SecureBootConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config

	if m.stateDir != "" {
		if err := m.saveState(); err != nil {
			return fmt.Errorf("保存状态失败：%w", err)
		}
	}

	m.logger.Info("Secure Boot 配置已更新",
		zap.Bool("enabled", config.Enabled),
		zap.String("mode", string(config.Mode)),
	)
	return nil
}

// GetPolicy 获取当前策略。
func (m *Manager) GetPolicy() SecureBootPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// UpdatePolicy 更新安全策略。
func (m *Manager) UpdatePolicy(policy SecureBootPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.logger.Info("安全策略已更新",
		zap.Bool("enforce_kernel", policy.EnforceKernelSignature),
		zap.Bool("enforce_module", policy.EnforceModuleSignature),
		zap.Bool("audit_mode", policy.AuditMode),
	)
}

// Status 获取 Secure Boot 状态。
func (m *Manager) Status() SecureBootStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.keyManager.Status(m.config)
}

// IsEnabled 是否启用。
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Enabled
}

// IsInitialized 是否已初始化。
func (m *Manager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// initUEFIVariables 初始化 UEFI 安全变量。
func (m *Manager) initUEFIVariables() error {
	// 设置 SecureBoot 变量
	secureBootVal := byte(0)
	if m.config.Enabled {
		secureBootVal = 1
	}

	if err := m.varStore.SetVariable(&UEFIVariable{
		Name:       UEFIVarSecureBoot,
		GUID:       UEFIGlobalGUID,
		Attributes: 0x07, // EFI_VARIABLE_NON_VOLATILE | EFI_VARIABLE_BOOTSERVICE_ACCESS | EFI_VARIABLE_RUNTIME_ACCESS
		Data:       []byte{secureBootVal},
	}); err != nil {
		return fmt.Errorf("设置 SecureBoot 变量失败：%w", err)
	}

	// 设置 SetupMode 变量
	setupModeVal := byte(0) // UserMode
	if err := m.varStore.SetVariable(&UEFIVariable{
		Name:       UEFIVarSetupMode,
		GUID:       UEFIGlobalGUID,
		Attributes: 0x07,
		Data:       []byte{setupModeVal},
	}); err != nil {
		return fmt.Errorf("设置 SetupMode 变量失败：%w", err)
	}

	return nil
}

// stateData 持久化状态数据。
type stateData struct {
	Config SecureBootConfig `json:"config"`
	Policy SecureBootPolicy `json:"policy"`
}

// saveState 持久化状态到磁盘。
func (m *Manager) saveState() error {
	if m.stateDir == "" {
		return nil
	}

	if err := os.MkdirAll(m.stateDir, 0750); err != nil {
		return fmt.Errorf("创建状态目录失败：%w", err)
	}

	data := stateData{
		Config: m.config,
		Policy: m.policy,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败：%w", err)
	}

	path := filepath.Join(m.stateDir, "secureboot-state.json")
	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return fmt.Errorf("写入状态文件失败：%w", err)
	}

	return nil
}

// loadState 从磁盘恢复状态。
func (m *Manager) loadState() error {
	if m.stateDir == "" {
		return errors.New("未设置状态目录")
	}

	path := filepath.Join(m.stateDir, "secureboot-state.json")
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取状态文件失败：%w", err)
	}

	var data stateData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("解析状态文件失败：%w", err)
	}

	m.config = data.Config
	m.policy = data.Policy

	return nil
}
