// Package smartfilecollect - 智能文件收集模块入口
// 基于群晖 File Request 的增强版，通过链接安全收集文件
// 支持自动分类、去重、病毒扫描
package smartfilecollect

import (
	"net/http"
)

// SmartFileCollectModule 智能文件收集模块.
type SmartFileCollectModule struct {
	manager *CollectManager
	handler *CollectHandler
}

// NewSmartFileCollectModule 创建智能文件收集模块.
func NewSmartFileCollectModule(config *CollectConfig) *SmartFileCollectModule {
	manager := NewCollectManager(config)
	handler := NewCollectHandler(manager)

	return &SmartFileCollectModule{
		manager: manager,
		handler: handler,
	}
}

// GetManager 获取管理器.
func (m *SmartFileCollectModule) GetManager() *CollectManager {
	return m.manager
}

// GetHandler 获取处理器.
func (m *SmartFileCollectModule) GetHandler() *CollectHandler {
	return m.handler
}

// RegisterRoutes 注册路由.
func (m *SmartFileCollectModule) RegisterRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}

// 便捷函数

// QuickCreateCollect 快速创建收集请求.
func QuickCreateCollect(title, description, targetPath string, expireDays int) (*CollectRequest, error) {
	manager := NewCollectManager(nil)
	return manager.CreateCollectRequest(&CreateCollectRequest{
		Title:       title,
		Description: description,
		TargetPath:  targetPath,
		ExpiresIn:   expireDays,
	}, "system", "System")
}

// ValidateCollectLink 验证收集链接.
func ValidateCollectLink(link string) bool {
	manager := NewCollectManager(nil)
	_, err := manager.GetCollectRequestByLink(link)
	return err == nil
}
