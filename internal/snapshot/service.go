package snapshot

import (
	"log"

	"github.com/gin-gonic/gin"
	"nas-os/internal/storage"
)

// Service 快照服务
type Service struct {
	PolicyManager *PolicyManager
	Handlers      *Handlers
}

// NewService 创建快照服务
func NewService(configPath string, storageMgr *storage.Manager) *Service {
	// 创建策略管理器
	pm := NewPolicyManagerWithStorage(configPath, storageMgr)

	// 创建处理器
	handlers := NewHandlers(pm)

	return &Service{
		PolicyManager: pm,
		Handlers:      handlers,
	}
}

// Initialize 初始化服务
func (s *Service) Initialize() error {
	if err := s.PolicyManager.Initialize(); err != nil {
		return err
	}

	log.Println("✅ 快照策略服务就绪")
	return nil
}

// Close 关闭服务
func (s *Service) Close() {
	s.PolicyManager.Close()
}

// RegisterRoutes 注册路由
func (s *Service) RegisterRoutes(r *gin.RouterGroup) {
	s.Handlers.RegisterRoutes(r)
}

// DefaultConfigPath 默认配置路径
const DefaultConfigPath = "/var/lib/nas-os/snapshot-policies.json"

// InitializeService 初始化快照服务（便捷函数）
func InitializeService(storageMgr *storage.Manager) *Service {
	svc := NewService(DefaultConfigPath, storageMgr)
	if err := svc.Initialize(); err != nil {
		log.Printf("⚠️ 快照服务初始化警告: %v", err)
	}
	return svc
}
