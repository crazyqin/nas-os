// Package projectkanban 提供项目看板管理功能
//
// 项目看板模块支持：
// - 看板项目管理（创建/归档项目）
// - 任务卡片拖拽（待办/进行中/已完成）
// - 任务标签、优先级、截止日期
// - 文件关联（任务关联 NAS 文件）
// - 项目统计（完成率、延期任务、成员工作量）
package projectkanban

import "github.com/gin-gonic/gin"

// Module 项目看板模块
type Module struct {
	manager  *Manager
	handlers *Handlers
}

// NewModule 创建模块实例
func NewModule() *Module {
	manager := NewManager()
	handlers := NewHandlers(manager)

	return &Module{
		manager:  manager,
		handlers: handlers,
	}
}

// RegisterRoutes 注册路由到 gin 引擎
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	m.handlers.RegisterRoutes(r)
}

// GetManager 获取管理器实例
func (m *Module) GetManager() *Manager {
	return m.manager
}
