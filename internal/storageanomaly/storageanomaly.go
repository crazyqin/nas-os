// Package storageanomaly - 存储异常检测模块入口
// 基于 AI 的存储异常检测，检测异常访问模式、潜在数据损坏、异常容量增长等
package storageanomaly

import (
	"net/http"
)

// StorageAnomalyModule 存储异常检测模块
type StorageAnomalyModule struct {
	manager *AnomalyManager
	handler *AnomalyHandler
}

// NewStorageAnomalyModule 创建存储异常检测模块
func NewStorageAnomalyModule(config *AnomalyConfig) *StorageAnomalyModule {
	manager := NewAnomalyManager(config)
	handler := NewAnomalyHandler(manager)

	return &StorageAnomalyModule{
		manager: manager,
		handler: handler,
	}
}

// GetManager 获取管理器
func (m *StorageAnomalyModule) GetManager() *AnomalyManager {
	return m.manager
}

// GetHandler 获取处理器
func (m *StorageAnomalyModule) GetHandler() *AnomalyHandler {
	return m.handler
}

// RegisterRoutes 注册路由
func (m *StorageAnomalyModule) RegisterRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}

// 便捷函数

// QuickDetect 快速检测指定设备的异常
func QuickDetect(deviceID string) (*DetectionResult, error) {
	manager := NewAnomalyManager(nil)
	return manager.DetectAnomalies(deviceID)
}

// QuickCollectMetrics 快速收集指标
func QuickCollectMetrics(metrics *StorageMetrics) error {
	manager := NewAnomalyManager(nil)
	return manager.CollectMetrics(metrics)
}
