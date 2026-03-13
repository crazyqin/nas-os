package tiering

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Integration 存储分层集成器
// 负责与 NAS-OS 其他模块的集成
type Integration struct {
	mu sync.RWMutex

	// 分层管理器
	manager *Manager

	// 监控指标
	metrics *Metrics

	// 日志
	logger *zap.Logger

	// 存储回调
	storageCallbacks StorageCallbacks

	// 文件系统监控
	fsWatcher *FSWatcher

	// 运行状态
	running bool
	stopCh  chan struct{}
}

// StorageCallbacks 存储模块回调接口
type StorageCallbacks interface {
	// 获取存储池信息
	GetPoolInfo(poolName string) (*PoolInfo, error)

	// 获取卷信息
	GetVolumeInfo(volumeName string) (*VolumeInfo, error)

	// 获取文件访问统计
	GetFileAccessStats(path string) (*FileAccessStats, error)

	// 移动文件
	MoveFile(ctx context.Context, src, dst string) error

	// 复制文件
	CopyFile(ctx context.Context, src, dst string) error

	// 删除文件
	DeleteFile(ctx context.Context, path string) error

	// 检查文件是否存在
	FileExists(path string) bool

	// 获取文件大小
	GetFileSize(path string) (int64, error)
}

// PoolInfo 存储池信息
type PoolInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // btrfs, zfs, etc.
	TotalBytes  int64  `json:"totalBytes"`
	UsedBytes   int64  `json:"usedBytes"`
	FreeBytes   int64  `json:"freeBytes"`
	Health      string `json:"health"`
	MountPoint  string `json:"mountPoint"`
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	SizeBytes  int64    `json:"sizeBytes"`
	UsedBytes  int64    `json:"usedBytes"`
	TierType   TierType `json:"tierType"`
	IsReadOnly bool     `json:"isReadOnly"`
}

// FileAccessStats 文件访问统计
type FileAccessStats struct {
	Path          string    `json:"path"`
	LastAccess    time.Time `json:"lastAccess"`
	AccessCount   int64     `json:"accessCount"`
	ReadBytes     int64     `json:"readBytes"`
	WriteBytes    int64     `json:"writeBytes"`
	IsHot         bool      `json:"isHot"`
	IsCold        bool      `json:"isCold"`
}

// FSWatcher 文件系统监控器
type FSWatcher struct {
	watchPaths []string
	events     chan FSEvent
	stopCh     chan struct{}
}

// FSEvent 文件系统事件
type FSEvent struct {
	Type      FSEventType
	Path      string
	Timestamp time.Time
}

// FSEventType 文件系统事件类型
type FSEventType string

const (
	FSEventCreate FSEventType = "create"
	FSEventModify FSEventType = "modify"
	FSEventDelete FSEventType = "delete"
	FSEventAccess FSEventType = "access"
)

// IntegrationConfig 集成配置
type IntegrationConfig struct {
	// 配置文件路径
	ConfigPath string `json:"configPath"`

	// 是否启用自动分层
	EnableAutoTier bool `json:"enableAutoTier"`

	// 监控路径
	WatchPaths []string `json:"watchPaths"`

	// 日志级别
	LogLevel string `json:"logLevel"`

	// 同步间隔
	SyncInterval time.Duration `json:"syncInterval"`
}

// DefaultIntegrationConfig 默认集成配置
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		ConfigPath:    "/etc/nas-os/tiering/config.json",
		EnableAutoTier: true,
		WatchPaths:    []string{"/mnt/ssd", "/mnt/hdd"},
		LogLevel:      "info",
		SyncInterval:  5 * time.Minute,
	}
}

// NewIntegration 创建集成器
func NewIntegration(config IntegrationConfig, logger *zap.Logger) *Integration {
	engineConfig := DefaultPolicyEngineConfig()
	engineConfig.EnableAutoTier = config.EnableAutoTier

	manager := NewManager(config.ConfigPath, engineConfig)
	metrics := NewMetrics()

	return &Integration{
		manager:  manager,
		metrics:  metrics,
		logger:   logger,
		stopCh:   make(chan struct{}),
		fsWatcher: &FSWatcher{
			events: make(chan FSEvent, 1000),
			stopCh: make(chan struct{}),
		},
	}
}

// SetStorageCallbacks 设置存储回调
func (i *Integration) SetStorageCallbacks(callbacks StorageCallbacks) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.storageCallbacks = callbacks
}

// Initialize 初始化集成器
func (i *Integration) Initialize(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// 初始化分层管理器
	if err := i.manager.Initialize(); err != nil {
		return fmt.Errorf("初始化分层管理器失败: %w", err)
	}

	// 同步存储层配置
	if err := i.syncTierConfig(ctx); err != nil {
		i.logger.Warn("同步存储层配置失败", zap.Error(err))
	}

	i.running = true

	// 启动同步任务
	go i.runSyncTask(ctx)

	i.logger.Info("存储分层集成器初始化完成")

	return nil
}

// syncTierConfig 同步存储层配置
func (i *Integration) syncTierConfig(ctx context.Context) error {
	if i.storageCallbacks == nil {
		return nil
	}

	// 获取各个存储池的信息并更新配置
	pools := []string{"ssd", "hdd", "cloud"}

	for _, poolName := range pools {
		poolInfo, err := i.storageCallbacks.GetPoolInfo(poolName)
		if err != nil {
			continue
		}

		tierType := poolToTierType(poolName)
		if tierType == "" {
			continue
		}

		tier, err := i.manager.GetTier(tierType)
		if err != nil {
			continue
		}

		// 更新容量信息
		if tier != nil && poolInfo != nil {
			tier.Capacity = poolInfo.TotalBytes
			tier.Used = poolInfo.UsedBytes
			tier.Path = poolInfo.MountPoint

			// 更新指标
			stats := &TierStats{
				Type:     tierType,
				Name:     tier.Name,
				Capacity: tier.Capacity,
				Used:     tier.Used,
				Available: poolInfo.FreeBytes,
				UsagePercent: float64(tier.Used) / float64(max(tier.Capacity, 1)) * 100,
			}
			i.metrics.UpdateTierMetrics(tierType, stats)
		}
	}

	return nil
}

// runSyncTask 运行同步任务
func (i *Integration) runSyncTask(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-i.stopCh:
			return
		case <-ticker.C:
			if err := i.syncTierConfig(ctx); err != nil {
				i.logger.Debug("同步存储层配置", zap.Error(err))
			}
		}
	}
}

// Start 启动集成器
func (i *Integration) Start(ctx context.Context) error {
	return i.Initialize(ctx)
}

// Stop 停止集成器
func (i *Integration) Stop() {
	i.mu.Lock()
	i.running = false
	i.mu.Unlock()

	close(i.stopCh)
	i.manager.Stop()

	i.logger.Info("存储分层集成器已停止")
}

// ==================== 文件操作集成 ====================

// OnFileAccess 文件访问回调
func (i *Integration) OnFileAccess(ctx context.Context, path string, tierType TierType, readBytes, writeBytes int64) error {
	// 记录访问
	if err := i.manager.tracker.RecordAccess(path, tierType, readBytes, writeBytes); err != nil {
		i.logger.Debug("记录文件访问失败", zap.String("path", path), zap.Error(err))
	}

	// 更新指标
	i.metrics.RecordTierIO(tierType, readBytes, writeBytes, 1, 0)

	return nil
}

// OnFileCreate 文件创建回调
func (i *Integration) OnFileCreate(ctx context.Context, path string, size int64, tierType TierType) error {
	// 创建访问记录
	i.manager.tracker.RecordAccess(path, tierType, 0, size)

	i.logger.Debug("新文件创建", zap.String("path", path), zap.String("tier", string(tierType)))

	return nil
}

// OnFileDelete 文件删除回调
func (i *Integration) OnFileDelete(ctx context.Context, path string, tierType TierType) error {
	// 移除访问记录
	i.manager.tracker.RemoveRecord(path)

	i.logger.Debug("文件删除", zap.String("path", path))

	return nil
}

// OnFileMove 文件移动回调
func (i *Integration) OnFileMove(ctx context.Context, srcPath, dstPath string, srcTier, dstTier TierType) error {
	// 更新访问记录的存储层
	i.manager.tracker.RemoveRecord(srcPath)
	i.manager.tracker.RecordAccess(dstPath, dstTier, 0, 0)

	// 更新迁移指标
	i.metrics.RecordTierMigration(srcTier, 0, 1, 0, 0)
	i.metrics.RecordTierMigration(dstTier, 1, 0, 0, 0)

	i.logger.Debug("文件移动",
		zap.String("src", srcPath),
		zap.String("dst", dstPath),
		zap.String("srcTier", string(srcTier)),
		zap.String("dstTier", string(dstTier)),
	)

	return nil
}

// ==================== 存储层管理集成 ====================

// GetTierInfo 获取存储层信息
func (i *Integration) GetTierInfo(tierType TierType) (*TierInfo, error) {
	tier, err := i.manager.GetTier(tierType)
	if err != nil {
		return nil, err
	}

	stats, _ := i.manager.GetTierStats(tierType)
	metrics := i.metrics.GetTierMetrics(tierType)

	info := &TierInfo{
		Type:       tierType,
		Name:       tier.Name,
		Path:       tier.Path,
		Capacity:   tier.Capacity,
		Used:       tier.Used,
		Enabled:    tier.Enabled,
		Priority:   tier.Priority,
		Threshold:  tier.Threshold,
		ProviderID: tier.ProviderID,
	}

	if stats != nil {
		info.TotalFiles = stats.TotalFiles
		info.HotFiles = stats.HotFiles
		info.WarmFiles = stats.WarmFiles
		info.ColdFiles = stats.ColdFiles
	}

	if metrics != nil {
		info.ReadBytes = metrics.ReadBytes
		info.WriteBytes = metrics.WriteBytes
		info.FilesMigratedIn = metrics.FilesMigratedIn
		info.FilesMigratedOut = metrics.FilesMigratedOut
	}

	return info, nil
}

// TierInfo 存储层详细信息
type TierInfo struct {
	Type       TierType `json:"type"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Capacity   int64    `json:"capacity"`
	Used       int64    `json:"used"`
	Enabled    bool     `json:"enabled"`
	Priority   int      `json:"priority"`
	Threshold  int      `json:"threshold"`
	ProviderID string   `json:"providerId,omitempty"`

	// 文件统计
	TotalFiles int64 `json:"totalFiles"`
	HotFiles   int64 `json:"hotFiles"`
	WarmFiles  int64 `json:"warmFiles"`
	ColdFiles  int64 `json:"coldFiles"`

	// I/O 统计
	ReadBytes  int64 `json:"readBytes"`
	WriteBytes int64 `json:"writeBytes"`

	// 迁移统计
	FilesMigratedIn  int64 `json:"filesMigratedIn"`
	FilesMigratedOut int64 `json:"filesMigratedOut"`
}

// ==================== 策略执行集成 ====================

// ExecutePolicyWithCallback 执行策略并回调
func (i *Integration) ExecutePolicyWithCallback(ctx context.Context, policyID string, preMigrate func(path string) bool) (*MigrateTask, error) {
	task, err := i.manager.ExecutePolicy(policyID)
	if err != nil {
		return nil, err
	}

	// 记录指标
	i.metrics.RecordMigrationStart()

	return task, nil
}

// ==================== 监控指标集成 ====================

// GetMetrics 获取监控指标
func (i *Integration) GetMetrics() *Metrics {
	return i.metrics
}

// GetMetricsSummary 获取指标汇总
func (i *Integration) GetMetricsSummary() *MetricsSummary {
	return i.metrics.GetSummary()
}

// ExportPrometheusMetrics 导出 Prometheus 格式指标
func (i *Integration) ExportPrometheusMetrics() string {
	return i.metrics.ExportPrometheus()
}

// ==================== 健康检查 ====================

// HealthCheck 健康检查
func (i *Integration) HealthCheck(ctx context.Context) *HealthStatus {
	i.mu.RLock()
	defer i.mu.RUnlock()

	status := &HealthStatus{
		Healthy:   true,
		Timestamp: time.Now(),
		Checks:    make(map[string]HealthCheckResult),
	}

	// 检查存储层
	for tierType := range i.manager.tiers {
		tier, err := i.manager.GetTier(tierType)
		if err != nil {
			status.Checks[string(tierType)] = HealthCheckResult{
				Status:  "error",
				Message: err.Error(),
			}
			status.Healthy = false
			continue
		}

		if !tier.Enabled {
			status.Checks[string(tierType)] = HealthCheckResult{
				Status:  "disabled",
				Message: "存储层已禁用",
			}
			continue
		}

		// 检查容量
		if tier.Capacity > 0 {
			usagePercent := float64(tier.Used) / float64(tier.Capacity) * 100
			if usagePercent > float64(tier.Threshold) {
				status.Checks[string(tierType)] = HealthCheckResult{
					Status:  "warning",
					Message: fmt.Sprintf("存储层使用率 %.1f%% 超过阈值 %d%%", usagePercent, tier.Threshold),
				}
			} else {
				status.Checks[string(tierType)] = HealthCheckResult{
					Status:  "healthy",
					Message: fmt.Sprintf("使用率 %.1f%%", usagePercent),
				}
			}
		}
	}

	// 检查运行中的任务
	metrics := i.metrics.GetMigrationMetrics()
	if metrics.RunningTasks > 10 {
		status.Checks["migration"] = HealthCheckResult{
			Status:  "warning",
			Message: fmt.Sprintf("有 %d 个迁移任务正在运行", metrics.RunningTasks),
		}
	} else {
		status.Checks["migration"] = HealthCheckResult{
			Status:  "healthy",
			Message: fmt.Sprintf("%d 个任务运行中", metrics.RunningTasks),
		}
	}

	return status
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy   bool                       `json:"healthy"`
	Timestamp time.Time                  `json:"timestamp"`
	Checks    map[string]HealthCheckResult `json:"checks"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status  string `json:"status"` // healthy, warning, error, disabled
	Message string `json:"message"`
}

// ==================== 辅助函数 ====================

func poolToTierType(poolName string) TierType {
	switch poolName {
	case "ssd", "ssd-cache", "nvme":
		return TierTypeSSD
	case "hdd", "hdd-storage":
		return TierTypeHDD
	case "cloud", "archive":
		return TierTypeCloud
	default:
		return ""
	}
}

// ==================== API 处理器集成 ====================

// IntegrationHandler 集成 API 处理器
type IntegrationHandler struct {
	integration *Integration
	handler     *Handler
}

// NewIntegrationHandler 创建集成处理器
func NewIntegrationHandler(integration *Integration) *IntegrationHandler {
	return &IntegrationHandler{
		integration: integration,
		handler:     NewHandler(integration.manager),
	}
}

// RegisterRoutes 注册路由
func (h *IntegrationHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 注册基础路由
	h.handler.RegisterRoutes(r)

	// 注册集成路由
	integration := r.Group("/tiering")
	{
		integration.GET("/metrics", h.GetMetrics)
		integration.GET("/metrics/prometheus", h.GetPrometheusMetrics)
		integration.GET("/health", h.GetHealth)
	}
}

// GetMetrics 获取指标
func (h *IntegrationHandler) GetMetrics(c *gin.Context) {
	summary := h.integration.GetMetricsSummary()
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    summary,
	})
}

// GetPrometheusMetrics 获取 Prometheus 格式指标
func (h *IntegrationHandler) GetPrometheusMetrics(c *gin.Context) {
	metrics := h.integration.ExportPrometheusMetrics()
	c.Data(200, "text/plain; charset=utf-8", []byte(metrics))
}

// GetHealth 获取健康状态
func (h *IntegrationHandler) GetHealth(c *gin.Context) {
	status := h.integration.HealthCheck(c.Request.Context())
	if !status.Healthy {
		c.JSON(503, status)
		return
	}
	c.JSON(200, status)
}