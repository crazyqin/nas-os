package secureboot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.GetKeyManager() == nil {
		t.Error("KeyManager 应已初始化")
	}
	if m.GetSignatureVerifier() == nil {
		t.Error("SignatureVerifier 应已初始化")
	}
	if m.GetFirmwareVerifier() == nil {
		t.Error("FirmwareVerifier 应已初始化")
	}
	if m.GetVariableStore() == nil {
		t.Error("VariableStore 应已初始化")
	}
}

func TestManagerDefaults(t *testing.T) {
	m := NewManager(nil)

	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("默认配置应启用")
	}
	if cfg.Mode != ModeStandard {
		t.Errorf("默认模式应为 standard，实际 %s", cfg.Mode)
	}

	p := m.GetPolicy()
	if !p.EnforceKernelSignature {
		t.Error("默认应强制内核签名")
	}
}

func TestManagerWithConfig(t *testing.T) {
	customCfg := SecureBootConfig{
		Enabled:         true,
		Mode:            ModeStrict,
		SecureBootState: StateEnabled,
		TPMEnabled:      true,
	}

	m := NewManager(nil, WithConfig(customCfg))
	cfg := m.GetConfig()

	if cfg.Mode != ModeStrict {
		t.Errorf("模式应为 strict，实际 %s", cfg.Mode)
	}
	if !cfg.TPMEnabled {
		t.Error("TPM 应启用")
	}
}

func TestManagerWithPolicy(t *testing.T) {
	policy := SecureBootPolicy{
		EnforceKernelSignature: false,
		AuditMode:              true,
	}

	m := NewManager(nil, WithPolicy(policy))
	p := m.GetPolicy()

	if p.EnforceKernelSignature {
		t.Error("内核签名验证应禁用")
	}
	if !p.AuditMode {
		t.Error("审计模式应启用")
	}
}

func TestManagerInitialize(t *testing.T) {
	m := NewManager(nil)

	if m.IsInitialized() {
		t.Error("初始化前不应标记为已初始化")
	}

	err := m.Initialize()
	if err != nil {
		t.Fatalf("Initialize 失败：%v", err)
	}

	if !m.IsInitialized() {
		t.Error("初始化后应标记为已初始化")
	}

	// 重复初始化应失败
	err = m.Initialize()
	if err == nil {
		t.Error("重复初始化应返回错误")
	}
}

func TestManagerStatus(t *testing.T) {
	m := NewManager(nil)
	_ = m.Initialize()

	status := m.Status()

	if !status.Enabled {
		t.Error("应为启用状态")
	}
	if status.State != "enabled" {
		t.Errorf("状态应为 enabled，实际 %s", status.State)
	}
	if status.KeyCount < 3 {
		t.Errorf("密钥数应至少 3，实际 %d", status.KeyCount)
	}
}

func TestManagerIsEnabled(t *testing.T) {
	m := NewManager(nil, WithConfig(SecureBootConfig{Enabled: true}))
	if !m.IsEnabled() {
		t.Error("应为启用")
	}

	m2 := NewManager(nil, WithConfig(SecureBootConfig{Enabled: false}))
	if m2.IsEnabled() {
		t.Error("应为禁用")
	}
}

func TestManagerUpdateConfig(t *testing.T) {
	m := NewManager(nil)
	_ = m.Initialize()

	newCfg := SecureBootConfig{
		Enabled:         true,
		Mode:            ModeAudit,
		SecureBootState: StateEnabled,
	}

	err := m.UpdateConfig(newCfg)
	if err != nil {
		t.Fatalf("UpdateConfig 失败：%v", err)
	}

	cfg := m.GetConfig()
	if cfg.Mode != ModeAudit {
		t.Errorf("模式应为 audit，实际 %s", cfg.Mode)
	}
}

func TestManagerUpdatePolicy(t *testing.T) {
	m := NewManager(nil)

	policy := SecureBootPolicy{
		EnforceKernelSignature: false,
		EnforceModuleSignature: false,
		AllowCustomKeys:        true,
		AuditMode:              true,
	}

	m.UpdatePolicy(policy)

	p := m.GetPolicy()
	if p.EnforceKernelSignature {
		t.Error("内核签名验证应禁用")
	}
	if !p.AllowCustomKeys {
		t.Error("应允许自定义密钥")
	}
	if !p.AuditMode {
		t.Error("审计模式应启用")
	}
}

func TestManagerPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secureboot-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败：%v", err)
	}
	defer os.RemoveAll(tmpDir)

	stateDir := filepath.Join(tmpDir, "state")

	// 创建并初始化管理器
	m1 := NewManager(nil, WithStateDir(stateDir))
	err = m1.Initialize()
	if err != nil {
		t.Fatalf("Initialize 失败：%v", err)
	}

	// 更新配置
	newCfg := m1.GetConfig()
	newCfg.Mode = ModeAudit
	_ = m1.UpdateConfig(newCfg)

	// 创建新管理器（模拟重启）
	m2 := NewManager(nil, WithStateDir(stateDir))
	err = m2.Initialize()
	if err != nil {
		t.Fatalf("恢复 Initialize 失败：%v", err)
	}

	// 验证配置恢复
	cfg := m2.GetConfig()
	if cfg.Mode != ModeAudit {
		t.Errorf("恢复后模式应为 audit，实际 %s", cfg.Mode)
	}
}

func TestManagerVariableStoreIntegration(t *testing.T) {
	m := NewManager(nil)
	_ = m.Initialize()

	store := m.GetVariableStore()

	// 检查 SecureBoot 变量
	sbVar, err := store.GetVariable(UEFIVarSecureBoot, UEFIGlobalGUID)
	if err != nil {
		t.Fatalf("获取 SecureBoot 变量失败：%v", err)
	}
	if len(sbVar.Data) != 1 || sbVar.Data[0] != 1 {
		t.Error("SecureBoot 变量应为启用 (1)")
	}

	// 检查 SetupMode 变量
	smVar, err := store.GetVariable(UEFIVarSetupMode, UEFIGlobalGUID)
	if err != nil {
		t.Fatalf("获取 SetupMode 变量失败：%v", err)
	}
	if len(smVar.Data) != 1 || smVar.Data[0] != 0 {
		t.Error("SetupMode 变量应为 UserMode (0)")
	}
}

func TestManagerDisabledConfig(t *testing.T) {
	cfg := SecureBootConfig{
		Enabled:         false,
		Mode:            ModeAudit,
		SecureBootState: StateDisabled,
	}

	m := NewManager(nil, WithConfig(cfg))
	_ = m.Initialize()

	store := m.GetVariableStore()
	sbVar, err := store.GetVariable(UEFIVarSecureBoot, UEFIGlobalGUID)
	if err != nil {
		t.Fatalf("获取 SecureBoot 变量失败：%v", err)
	}
	if len(sbVar.Data) != 1 || sbVar.Data[0] != 0 {
		t.Error("禁用时 SecureBoot 变量应为 0")
	}
}
