// Package compress 提供透明压缩存储功能
package compress

import (
	"log"

	"github.com/gin-gonic/gin"
)

// Service 压缩服务
type Service struct {
	Manager  *Manager
	FS       *FileSystem
	Handlers *Handlers
	rootPath string
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	RootPath string        `json:"root_path"`
	Config   *Config       `json:"config"`
}

// DefaultServiceConfig 默认服务配置
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		RootPath: "/data",
		Config:   DefaultConfig(),
	}
}

// NewService 创建压缩服务
func NewService(config ServiceConfig) (*Service, error) {
	// 创建管理器
	manager, err := NewManager(config.Config)
	if err != nil {
		return nil, err
	}

	// 创建文件系统
	fs, err := NewFileSystem(config.RootPath, manager)
	if err != nil {
		return nil, err
	}

	// 创建处理器
	handlers := NewHandlers(manager, fs)

	return &Service{
		Manager:  manager,
		FS:       fs,
		Handlers: handlers,
		rootPath: config.RootPath,
	}, nil
}

// Start 启动服务
func (s *Service) Start() error {
	log.Println("✅ 压缩服务已启动")
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	log.Println("压缩服务已停止")
}

// RegisterRoutes 注册路由
func (s *Service) RegisterRoutes(r *gin.RouterGroup) {
	s.Handlers.RegisterRoutes(r)
}

// InitializeService 初始化压缩服务（便捷函数）
func InitializeService(rootPath string) *Service {
	config := ServiceConfig{
		RootPath: rootPath,
		Config:   DefaultConfig(),
	}

	svc, err := NewService(config)
	if err != nil {
		log.Printf("⚠️ 压缩服务初始化失败: %v", err)
		return nil
	}

	if err := svc.Start(); err != nil {
		log.Printf("⚠️ 压缩服务启动失败: %v", err)
	}

	return svc
}

// GetCompressedSize 获取压缩后大小
func (s *Service) GetCompressedSize(originalSize int64) int64 {
	if !s.Manager.config.Enabled {
		return originalSize
	}

	// 使用平均压缩率估算
	stats := s.Manager.GetStats()
	if stats.TotalFiles == 0 {
		// 默认估算压缩率 50%
		return originalSize / 2
	}

	return int64(float64(originalSize) * stats.AvgRatio)
}

// GetStorageSavings 获取存储节省
func (s *Service) GetStorageSavings() int64 {
	return s.Manager.GetStats().SavedBytes
}

// GetCompressionRatio 获取平均压缩率
func (s *Service) GetCompressionRatio() float64 {
	return s.Manager.GetStats().AvgRatio
}

// IsEnabled 检查是否启用
func (s *Service) IsEnabled() bool {
	return s.Manager.config.Enabled
}

// SetEnabled 设置启用状态
func (s *Service) SetEnabled(enabled bool) {
	s.Manager.mu.Lock()
	defer s.Manager.mu.Unlock()
	s.Manager.config.Enabled = enabled
}

// GetAlgorithm 获取当前算法
func (s *Service) GetAlgorithm() Algorithm {
	return s.Manager.config.DefaultAlgorithm
}

// SetAlgorithm 设置压缩算法
func (s *Service) SetAlgorithm(algorithm Algorithm) {
	s.Manager.mu.Lock()
	defer s.Manager.mu.Unlock()
	s.Manager.config.DefaultAlgorithm = algorithm
}