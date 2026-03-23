// Package drive 提供类似 Synology Drive 的文件同步服务
// 支持多端同步、版本控制、文件锁定、审计日志
package drive

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Service Drive 服务核心
type Service struct {
	mu       sync.RWMutex
	config   *Config
	syncer   *SyncEngine
	locker   *FileLocker
	auditor  *AuditLogger
	version  *VersionManager
	shutdown chan struct{}
}

// Config Drive 服务配置
type Config struct {
	// 同步根目录
	RootPath string `json:"rootPath"`
	// 同步间隔 (秒)
	SyncInterval int `json:"syncInterval"`
	// 最大版本数
	MaxVersions int `json:"maxVersions"`
	// 文件锁定超时 (秒)
	LockTimeout int `json:"lockTimeout"`
	// 启用审计日志
	EnableAudit bool `json:"enableAudit"`
	// 启用加密
	EnableEncryption bool `json:"enableEncryption"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		SyncInterval:      300,  // 5分钟
		MaxVersions:       32,   // 保留32个版本
		LockTimeout:       3600, // 1小时
		EnableAudit:       true,
		EnableEncryption:  false,
	}
}

// NewService 创建 Drive 服务
func NewService(cfg *Config) (*Service, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.RootPath == "" {
		return nil, fmt.Errorf("rootPath 不能为空")
	}

	s := &Service{
		config:   cfg,
		shutdown: make(chan struct{}),
	}

	// 初始化子模块
	s.syncer = NewSyncEngine(cfg.RootPath)
	s.locker = NewFileLocker(time.Duration(cfg.LockTimeout) * time.Second)
	s.version = NewVersionManager(cfg.MaxVersions)
	s.auditor = NewAuditLogger(cfg.EnableAudit)

	return s, nil
}

// Start 启动 Drive 服务
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 启动同步引擎
	if err := s.syncer.Start(ctx); err != nil {
		return fmt.Errorf("启动同步引擎失败: %w", err)
	}

	// 启动后台任务
	go s.backgroundWorker(ctx)

	return nil
}

// Stop 停止 Drive 服务
func (s *Service) Stop() error {
	close(s.shutdown)
	return s.syncer.Stop()
}

// backgroundWorker 后台任务
func (s *Service) backgroundWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.config.SyncInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			// 执行定期同步检查
			s.syncer.CheckChanges(ctx)
			// 清理过期锁定
			s.locker.CleanupExpired()
			// 清理旧版本
			s.version.CleanupOldVersions()
		}
	}
}

// SyncFile 同步单个文件
func (s *Service) SyncFile(ctx context.Context, path string) error {
	s.auditor.Log(AuditActionSync, path, "sync file")
	return s.syncer.SyncFile(ctx, path)
}

// LockFile 锁定文件
func (s *Service) LockFile(ctx context.Context, path, userID string) error {
	s.auditor.Log(AuditActionLock, path, "lock file by "+userID)
	return s.locker.Lock(path, userID)
}

// UnlockFile 解锁文件
func (s *Service) UnlockFile(ctx context.Context, path, userID string) error {
	s.auditor.Log(AuditActionUnlock, path, "unlock file by "+userID)
	return s.locker.Unlock(path, userID)
}

// GetVersion 获取文件版本
func (s *Service) GetVersion(ctx context.Context, path string, version int) ([]byte, error) {
	return s.version.GetVersion(path, version)
}

// ListVersions 列出文件所有版本
func (s *Service) ListVersions(ctx context.Context, path string) ([]FileVersion, error) {
	return s.version.ListVersions(path)
}

// GetAuditLogs 获取审计日志
func (s *Service) GetAuditLogs(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	return s.auditor.Query(filter)
}