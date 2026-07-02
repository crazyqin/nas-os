// Package selectiveadsync - 选择性AD同步模块入口
// 从 Active Directory 选择性同步指定的 OU（组织单元），而非全量同步
// 参考群晖 DSM 7.3 的 "Smarter domain control: Sync only selected OUs to reduce load, minimize exposure, and enforce least-privilege policies"
package selectiveadsync

import (
	"net/http"
	"sync"
)

// Module 选择性AD同步模块.
type Module struct {
	mu      sync.RWMutex
	manager *SelectiveADSyncManager
	handler *SelectiveADSyncHandler
	enabled bool
}

// NewModule 创建模块实例.
func NewModule() *Module {
	manager := NewSelectiveADSyncManager()
	handler := NewSelectiveADSyncHandler(manager)

	return &Module{
		manager: manager,
		handler: handler,
		enabled: true,
	}
}

// Name 模块名称.
func (m *Module) Name() string {
	return "selectiveadsync"
}

// Description 模块描述.
func (m *Module) Description() string {
	return "选择性AD同步 - 从 Active Directory 选择性同步指定的 OU"
}

// Enable 启用模块.
func (m *Module) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable 禁用模块.
func (m *Module) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// IsEnabled 是否启用.
func (m *Module) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// RegisterRoutes 注册HTTP路由.
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}

// GetManager 获取管理器.
func (m *Module) GetManager() *SelectiveADSyncManager {
	return m.manager
}

// GetHandler 获取处理器.
func (m *Module) GetHandler() *SelectiveADSyncHandler {
	return m.handler
}
