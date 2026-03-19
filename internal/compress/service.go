// Package compress 提供透明压缩存储功能
// v2.6.0 增强版本：并行压缩、进度追踪、失败恢复
package compress

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Service 压缩服务
type Service struct {
	Manager  *Manager
	FS       *FileSystem
	Handlers *Handlers
	rootPath string

	// v2.6.0 并行压缩
	parallelCompressor *ParallelCompressor
	stateDir           string

	// 活跃任务
	mu    sync.RWMutex
	tasks map[string]*Task
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	RootPath string  `json:"root_path"`
	Config   *Config `json:"config"`
	StateDir string  `json:"state_dir"` // 状态存储目录（用于恢复）
}

// DefaultServiceConfig 默认服务配置
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		RootPath: "/data",
		Config:   DefaultConfig(),
		StateDir: "/var/lib/nas-os/compress-state",
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

	svc := &Service{
		Manager:  manager,
		FS:       fs,
		Handlers: handlers,
		rootPath: config.RootPath,
		stateDir: config.StateDir,
		tasks:    make(map[string]*Task),
	}

	// 创建并行压缩器
	if config.StateDir != "" {
		// 从 Config 转换为 ParallelConfig
		parallelCfg := &ParallelConfig{
			Algorithm:       config.Config.DefaultAlgorithm,
			Level:           config.Config.CompressionLevel,
			Workers:         4,
			DeleteOriginal:  false,
			Overwrite:       false,
			ContinueOnError: true,
			DryRun:          false,
			VerifyAfter:     true,
			MaxRetries:      3,
			MinSize:         config.Config.MinSize,
		}
		pc, err := NewParallelCompressor(parallelCfg, config.StateDir)
		if err != nil {
			log.Printf("⚠️ 并行压缩器初始化失败: %v", err)
		} else {
			svc.parallelCompressor = pc
		}
	}

	return svc, nil
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

// ========== v2.6.0 并行压缩接口 ==========

// Task 压缩任务
type Task struct {
	ID        string
	Status    string // pending, running, completed, failed, paused
	Progress  *CompressionProgress
	Result    *ParallelCompressResult
	Error     error
	Cancel    context.CancelFunc
	StartedAt time.Time
}

// CompressParallel 并行压缩文件
func (s *Service) CompressParallel(ctx context.Context, paths []string, config *ParallelConfig) (*ParallelCompressResult, error) {
	if s.parallelCompressor == nil {
		return nil, ErrParallelNotAvailable
	}

	// 创建任务
	taskID := s.parallelCompressor.progress.StartTime.Format("20060102-150405")
	taskCtx, cancel := context.WithCancel(ctx)

	task := &Task{
		ID:        taskID,
		Status:    "running",
		Progress:  s.parallelCompressor.GetProgress(),
		StartedAt: time.Now(),
		Cancel:    cancel,
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	// 执行压缩
	result, err := s.parallelCompressor.CompressParallel(taskCtx, paths, config)

	s.mu.Lock()
	if err != nil {
		task.Status = "failed"
		task.Error = err
	} else {
		task.Status = "completed"
		task.Result = result
	}
	s.mu.Unlock()

	return result, err
}

// CompressParallelWithProgress 带进度回调的并行压缩
func (s *Service) CompressParallelWithProgress(ctx context.Context, paths []string, config *ParallelConfig, callback ProgressCallback) (*ParallelCompressResult, error) {
	if s.parallelCompressor == nil {
		return nil, ErrParallelNotAvailable
	}

	// 注册进度回调
	s.parallelCompressor.progress.OnProgress(callback)

	return s.CompressParallel(ctx, paths, config)
}

// GetTaskProgress 获取任务进度
func (s *Service) GetTaskProgress(taskID string) (*CompressionProgress, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, false
	}
	return task.Progress, true
}

// GetTaskResult 获取任务结果
func (s *Service) GetTaskResult(taskID string) (*ParallelCompressResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return nil, false
	}
	return task.Result, true
}

// CancelTask 取消任务
func (s *Service) CancelTask(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok || task.Cancel == nil {
		return false
	}

	task.Cancel()
	task.Status = "cancelled"
	return true
}

// ListPendingTasks 列出待恢复的任务
func (s *Service) ListPendingTasks() []*CompressionState {
	if s.parallelCompressor == nil {
		return nil
	}
	return s.parallelCompressor.ListPendingTasks()
}

// ResumeTask 恢复中断的任务
func (s *Service) ResumeTask(ctx context.Context, taskID string) (*ParallelCompressResult, error) {
	if s.parallelCompressor == nil {
		return nil, ErrParallelNotAvailable
	}
	return s.parallelCompressor.Resume(ctx, taskID)
}

// GetGlobalProgress 获取全局进度
func (s *Service) GetGlobalProgress() *CompressionProgress {
	if s.parallelCompressor == nil {
		return nil
	}
	return s.parallelCompressor.GetProgress()
}

// 错误定义
var (
	ErrParallelNotAvailable = &Error{Code: "PARALLEL_NOT_AVAILABLE", Message: "并行压缩器未初始化"}
	ErrTaskNotFound         = &Error{Code: "TASK_NOT_FOUND", Message: "任务不存在"}
	ErrTaskAlreadyRunning   = &Error{Code: "TASK_RUNNING", Message: "任务正在运行"}
)

// Error 压缩错误
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}



func (e *Error) Error() string {
	return e.Message
}
