// Package usbmount 提供 USB 设备自动挂载管理功能
package usbmount

import (
	"log"

	"github.com/gin-gonic/gin"
)

// Service USB 挂载服务
type Service struct {
	Manager  *Manager
	Handlers *Handlers
}

// NewService 创建 USB 挂载服务
func NewService(configPath string) *Service {
	manager := NewManager(configPath)
	handlers := NewHandlers(manager)

	return &Service{
		Manager:  manager,
		Handlers: handlers,
	}
}

// Initialize 初始化服务
func (s *Service) Initialize() error {
	if err := s.Manager.Start(); err != nil {
		return err
	}

	log.Println("✅ USB 自动挂载服务就绪")
	return nil
}

// Close 关闭服务
func (s *Service) Close() {
	s.Manager.Stop()
}

// RegisterRoutes 注册路由
func (s *Service) RegisterRoutes(r *gin.RouterGroup) {
	s.Handlers.RegisterRoutes(r)
}

// DefaultConfigPath 默认配置路径
const DefaultConfigPath = "/var/lib/nas-os/usb-mount-config.json"

// InitializeService 初始化 USB 挂载服务（便捷函数）
func InitializeService() *Service {
	svc := NewService(DefaultConfigPath)
	if err := svc.Initialize(); err != nil {
		log.Printf("⚠️ USB 挂载服务初始化警告: %v", err)
	}
	return svc
}
