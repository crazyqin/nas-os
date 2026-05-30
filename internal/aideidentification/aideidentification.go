// Package aideidentification - AI隐私脱敏模块
// 可定制的 PII（个人身份信息）脱敏规则，在 AI 处理前自动脱敏
package aideidentification

import (
	"net/http"
)

// Module AI隐私脱敏模块
type Module struct {
	manager *DeidentificationManager
	handler *DeidentificationHandler
}

// NewModule 创建模块实例
func NewModule(config *DeidentificationConfig) *Module {
	manager := NewDeidentificationManager(config)
	handler := NewDeidentificationHandler(manager)

	return &Module{
		manager: manager,
		handler: handler,
	}
}

// RegisterRoutes 注册HTTP路由
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}

// GetManager 获取管理器实例（供内部模块调用）
func (m *Module) GetManager() *DeidentificationManager {
	return m.manager
}

// Deidentify 执行脱敏（便捷方法）
func (m *Module) Deidentify(text string) (*DeidentificationResult, error) {
	return m.manager.Deidentify(text, "")
}

// DeidentifyWithRule 使用指定规则执行脱敏
func (m *Module) DeidentifyWithRule(text string, ruleID string) (*DeidentificationResult, error) {
	return m.manager.Deidentify(text, ruleID)
}
