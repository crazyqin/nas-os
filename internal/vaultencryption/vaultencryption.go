// Package vaultencryption - 保险库加密模块入口
// 使用保险库密码解锁加密卷，提供灵活安全的数据访问
// 参考群晖 DSM 7.3 的 "Convenient encryption: Unlock encrypted volumes with a vault password"
package vaultencryption

import (
	"net/http"
	"sync"
)

// Module 保险库加密模块
type Module struct {
	mu      sync.RWMutex
	manager *VaultEncryptionManager
	handler *VaultEncryptionHandler
	enabled bool
}

// NewModule 创建模块实例
func NewModule() *Module {
	manager := NewVaultEncryptionManager()
	handler := NewVaultEncryptionHandler(manager)

	return &Module{
		manager: manager,
		handler: handler,
		enabled: true,
	}
}

// Name 模块名称
func (m *Module) Name() string {
	return "vaultencryption"
}

// Description 模块描述
func (m *Module) Description() string {
	return "保险库加密 - 使用保险库密码解锁加密卷"
}

// Enable 启用模块
func (m *Module) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable 禁用模块
func (m *Module) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// IsEnabled 是否启用
func (m *Module) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// RegisterRoutes 注册HTTP路由
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}

// GetManager 获取管理器
func (m *Module) GetManager() *VaultEncryptionManager {
	return m.manager
}

// GetHandler 获取处理器
func (m *Module) GetHandler() *VaultEncryptionHandler {
	return m.handler
}
