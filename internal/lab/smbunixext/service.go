// Package smbunixext 提供 SMB3 Unix Extensions 支持。
package smbunixext

import (
	"fmt"
	"sync"
	"time"
)

// ExtensionManager SMB3 Unix Extensions 管理器.
type ExtensionManager struct {
	mu      sync.RWMutex
	configs map[string]*UnixExtensionConfig
}

// NewExtensionManager 创建 Unix Extensions 管理器.
func NewExtensionManager() *ExtensionManager {
	return &ExtensionManager{
		configs: make(map[string]*UnixExtensionConfig),
	}
}

// SetExtension 设置共享的 Unix Extensions 配置.
func (m *ExtensionManager) SetExtension(req *SetExtensionRequest) (*UnixExtensionConfig, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.ShareName == "" {
		return nil, fmt.Errorf("share name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := &UnixExtensionConfig{
		ShareName:    req.ShareName,
		Enabled:      req.Enabled,
		Protocol:     ProtocolMulti,
		UpdatedAt:    time.Now(),
		Capabilities: CapabilityDefaults,
	}
	if req.Enabled {
		cfg.IsMultiProtocol = true
	}
	m.configs[req.ShareName] = cfg
	return cfg, nil
}

// GetExtension 获取共享的 Unix Extensions 配置.
func (m *ExtensionManager) GetExtension(shareName string) (*UnixExtensionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[shareName]
	if !ok {
		return nil, fmt.Errorf("config not found for share: %s", shareName)
	}
	return cfg, nil
}

// GetExtensionStatus 获取扩展状态.
func (m *ExtensionManager) GetExtensionStatus(shareName string) (*ExtensionStatusResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[shareName]
	if !ok {
		return nil, fmt.Errorf("config not found for share: %s", shareName)
	}

	status := ExtensionStatusDisabled
	if cfg.Enabled {
		status = ExtensionStatusEnabled
	}

	return &ExtensionStatusResponse{
		ShareName:       cfg.ShareName,
		Enabled:         cfg.Enabled,
		Protocol:        cfg.Protocol,
		IsMultiProtocol: cfg.IsMultiProtocol,
		Status:          status,
		Capabilities:    cfg.Capabilities,
	}, nil
}

// ListExtensions 列出所有扩展配置.
func (m *ExtensionManager) ListExtensions() []*UnixExtensionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*UnixExtensionConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	return result
}

// RemoveExtension 移除共享的扩展配置.
func (m *ExtensionManager) RemoveExtension(shareName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.configs, shareName)
}

// IsMultiProtocol 检查共享是否为 multi-protocol 模式.
func (m *ExtensionManager) IsMultiProtocol(shareName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[shareName]
	if !ok {
		return false
	}
	return cfg.IsMultiProtocol
}

// CanEnableUnixExtensions 检查是否可以启用 Unix Extensions
// 只有 Multi-Protocol 模式的共享才支持.
func (m *ExtensionManager) CanEnableUnixExtensions(shareName string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[shareName]
	if !ok {
		return false, fmt.Errorf("config not found for share: %s", shareName)
	}
	if !cfg.IsMultiProtocol {
		return false, nil
	}
	return true, nil
}

// SaveToDB 保存配置到数据库（模拟）.
func (m *ExtensionManager) SaveToDB(shareName string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.configs[shareName]
	if !ok {
		return fmt.Errorf("config not found for share: %s", shareName)
	}
	// 模拟数据库保存
	return nil
}

// LoadFromDB 从数据库加载配置（模拟）.
func (m *ExtensionManager) LoadFromDB(shareName string) (*UnixExtensionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[shareName]
	if !ok {
		return nil, fmt.Errorf("config not found for share: %s", shareName)
	}
	return cfg, nil
}
