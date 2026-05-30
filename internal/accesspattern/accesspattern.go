// Package accesspattern - 访问模式分析模块
// 分析文件访问模式，识别冷热数据，为智能分层提供依据
package accesspattern

import (
	"net/http"
)

// Module 访问模式分析模块
type Module struct {
	manager *AccessPatternManager
	handler *AccessPatternHandler
}

// NewModule 创建模块实例
func NewModule(config *AccessPatternConfig) *Module {
	manager := NewAccessPatternManager(config)
	handler := NewAccessPatternHandler(manager)

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
func (m *Module) GetManager() *AccessPatternManager {
	return m.manager
}

// RecordAccess 记录文件访问（便捷方法）
func (m *Module) RecordAccess(filePath string, fileSize int64, userID string) (*AccessRecord, error) {
	return m.manager.RecordAccess(&RecordAccessRequest{
		FilePath:   filePath,
		FileSize:   fileSize,
		AccessMode: "read",
		UserID:     userID,
	})
}

// AnalyzeFile 分析文件（便捷方法）
func (m *Module) AnalyzeFile(filePath string) (*PatternAnalysis, error) {
	return m.manager.AnalyzeFile(filePath)
}
