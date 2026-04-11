// Package app 应用池迁移功能
// 对标 TrueNAS 25.10 新特性：自动迁移应用池
// 提供应用在不同存储池之间的迁移能力，包含进度跟踪、状态管理和回滚机制
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	// MigrationStatusPending 待迁移
	MigrationStatusPending MigrationStatus = "pending"
	// MigrationStatusPreparing 准备中（验证源池、目标池）
	MigrationStatusPreparing MigrationStatus = "preparing"
	// MigrationStatusMigrating 迁移中
	MigrationStatusMigrating MigrationStatus = "migrating"
	// MigrationStatusVerifying 验证中
	MigrationStatusVerifying MigrationStatus = "verifying"
	// MigrationStatusCompleted 已完成
	MigrationStatusCompleted MigrationStatus = "completed"
	// MigrationStatusFailed 失败
	MigrationStatusFailed MigrationStatus = "failed"
	// MigrationStatusRollingBack 回滚中
	MigrationStatusRollingBack MigrationStatus = "rolling_back"
	// MigrationStatusRolledBack 已回滚
	MigrationStatusRolledBack MigrationStatus = "rolled_back"
)

// MigrationPhase 迁移阶段
type MigrationPhase string

const (
	// MigrationPhaseInit 初始化阶段
	MigrationPhaseInit MigrationPhase = "init"
	// MigrationPhaseValidate 验证阶段
	MigrationPhaseValidate MigrationPhase = "validate"
	// MigrationPhaseStopApps 停止应用阶段
	MigrationPhaseStopApps MigrationPhase = "stop_apps"
	// MigrationPhaseTransferData 数据传输阶段
	MigrationPhaseTransferData MigrationPhase = "transfer_data"
	// MigrationPhaseUpdateConfig 更新配置阶段
	MigrationPhaseUpdateConfig MigrationPhase = "update_config"
	// MigrationPhaseStartApps 启动应用阶段
	MigrationPhaseStartApps MigrationPhase = "start_apps"
	// MigrationPhaseVerify 验证阶段
	MigrationPhaseVerify MigrationPhase = "verify"
	// MigrationPhaseCleanup 清理阶段
	MigrationPhaseCleanup MigrationPhase = "cleanup"
)

// MigrationType 迁移类型
type MigrationType string

const (
	// MigrationTypeOnline 在线迁移（应用保持运行，最小停机时间）
	MigrationTypeOnline MigrationType = "online"
	// MigrationTypeOffline 离线迁移（应用停止，完整数据复制）
	MigrationTypeOffline MigrationType = "offline"
	// MigrationTypeLive 实时迁移（零停机，适用于支持的应用）
	MigrationTypeLive MigrationType = "live"
)

// MigrationError 迁移错误类型
var (
	ErrMigrationInProgress   = errors.New("migration already in progress")
	ErrInvalidSourcePool      = errors.New("invalid source pool")
	ErrInvalidTargetPool      = errors.New("invalid target pool")
	ErrInsufficientSpace      = errors.New("insufficient space on target pool")
	ErrAppNotSupported        = errors.New("app does not support migration")
	ErrRollbackFailed         = errors.New("rollback failed")
	ErrVerificationFailed     = errors.New("migration verification failed")
	ErrTargetPoolNotHealthy   = errors.New("target pool not healthy")
	ErrSourcePoolNotHealthy   = errors.New("source pool not healthy")
	ErrAppRunningDependencies = errors.New("app has running dependencies")
)

// MigrationProgress 迁移进度
type MigrationProgress struct {
	// 整体进度 (0-100)
	Percent float64 `json:"percent"`
	
	// 当前阶段
	Phase MigrationPhase `json:"phase"`
	
	// 已传输字节数
	BytesTransferred uint64 `json:"bytesTransferred"`
	
	// 总字节数
	BytesTotal uint64 `json:"bytesTotal"`
	
	// 传输速度 (bytes/s)
	TransferSpeed uint64 `json:"transferSpeed"`
	
	// 预计剩余时间（秒）
	EstimatedTimeRemaining int64 `json:"estimatedTimeRemaining"`
	
	// 已迁移应用数
	AppsMigrated int `json:"appsMigrated"`
	
	// 总应用数
	AppsTotal int `json:"appsTotal"`
	
	// 已跳过应用数
	AppsSkipped int `json:"appsSkipped"`
	
	// 错误信息
	Errors []MigrationErrorInfo `json:"errors,omitempty"`
	
	// 警告信息
	Warnings []string `json:"warnings,omitempty"`
}

// MigrationErrorInfo 迁移错误详情
type MigrationErrorInfo struct {
	Time    time.Time `json:"time"`
	AppID   string    `json:"appId,omitempty"`
	Message string    `json:"message"`
	Code    string    `json:"code"`
}

// MigrationCheckpoint 迁移检查点（用于恢复和回滚）
type MigrationCheckpoint struct {
	ID           string                 `json:"id"`
	MigrationID  string                 `json:"migrationId"`
	Phase        MigrationPhase         `json:"phase"`
	Timestamp    time.Time              `json:"timestamp"`
	AppStates    map[string]AppState    `json:"appStates"`
	VolumeStates map[string]VolumeState `json:"volumeStates"`
	ConfigBackup []byte                 `json:"configBackup"`
	Data         map[string]interface{} `json:"data,omitempty"`
}

// AppState 应用状态快照
type AppState struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	PoolPath string `json:"poolPath"`
}

// VolumeState 卷状态快照
type VolumeState struct {
	Name     string `json:"name"`
	PoolPath string `json:"poolPath"`
	Size     uint64 `json:"size"`
	BindMount bool  `json:"bindMount"`
}

// MigrationConfig 迁移配置
type MigrationConfig struct {
	// 源存储池ID
	SourcePoolID string `json:"sourcePoolId"`
	
	// 目标存储池ID
	TargetPoolID string `json:"targetPoolId"`
	
	// 要迁移的应用ID列表（空表示迁移全部）
	AppIDs []string `json:"appIds,omitempty"`
	
	// 迁移类型
	Type MigrationType `json:"type"`
	
	// 是否自动回滚失败的应用
	AutoRollback bool `json:"autoRollback"`
	
	// 是否保留源池数据（安全备份）
	KeepSourceData bool `json:"keepSourceData"`
	
	// 是否并行迁移
	ParallelMigration bool `json:"parallelMigration"`
	
	// 最大并行数
	MaxParallel int `json:"maxParallel"`
	
	// 迁移超时（秒）
	Timeout int `json:"timeout"`
	
	// 是否验证数据完整性
	VerifyIntegrity bool `json:"verifyIntegrity"`
	
	// 压缩传输
	CompressTransfer bool `json:"compressTransfer"`
	
	// 网络带宽限制 (MB/s, 0=无限制)
	BandwidthLimit int `json:"bandwidthLimit"`
}

// MigrationRecord 迁移记录
type MigrationRecord struct {
	ID           string           `json:"id"`
	Config       MigrationConfig  `json:"config"`
	Status       MigrationStatus  `json:"status"`
	Progress     MigrationProgress `json:"progress"`
	StartTime    time.Time        `json:"startTime"`
	EndTime      *time.Time       `json:"endTime,omitempty"`
	Duration     int64            `json:"duration"` // 秒
	Checkpoints  []MigrationCheckpoint `json:"checkpoints"`
	InitiatedBy string           `json:"initiatedBy"`
	CancelReason string          `json:"cancelReason,omitempty"`
}

// PoolMigrationManager 应用池迁移管理器
type PoolMigrationManager struct {
	mu              sync.RWMutex
	activeMigration *MigrationRecord
	history         []*MigrationRecord
	checkpoints      map[string]*MigrationCheckpoint
	eventHandlers   []MigrationEventHandler
	logger          MigrationLogger
	storage         MigrationStorage
	
	// 健康检查和验证器
	healthChecker   PoolHealthChecker
	appValidator    AppMigrationValidator
	appController   AppController
	
	// 进度追踪
	progressTracker *ProgressTracker
}

// PoolHealthChecker 存储池健康检查器接口
type PoolHealthChecker interface {
	CheckPoolHealth(poolID string) (*PoolHealthStatus, error)
	GetPoolCapacity(poolID string) (*PoolCapacityInfo, error)
	IsPoolAvailable(poolID string) bool
}

// PoolHealthStatus 存储池健康状态
type PoolHealthStatus struct {
	PoolID       string    `json:"poolId"`
	Healthy      bool      `json:"healthy"`
	Status       string    `json:"status"`
	HealthScore  float64   `json:"healthScore"`  // 0-100
	Errors       []string  `json:"errors"`
	Warnings     []string  `json:"warnings"`
	LastChecked  time.Time `json:"lastChecked"`
}

// PoolCapacityInfo 存储池容量信息
type PoolCapacityInfo struct {
	PoolID        string `json:"poolId"`
	TotalBytes    uint64 `json:"totalBytes"`
	UsedBytes     uint64 `json:"usedBytes"`
	AvailableBytes uint64 `json:"availableBytes"`
	UsedPercent   float64 `json:"usedPercent"`
}

// AppMigrationValidator 应用迁移验证器接口
type AppMigrationValidator interface {
	ValidateAppMigration(appID string, sourcePoolID, targetPoolID string) (*AppMigrationValidation, error)
	CheckAppDependencies(appID string) ([]string, error)
	GetAppStorageUsage(appID string) (*AppStorageUsage, error)
}

// AppMigrationValidation 应用迁移验证结果
type AppMigrationValidation struct {
	AppID           string `json:"appId"`
	CanMigrate      bool   `json:"canMigrate"`
	Reason          string `json:"reason,omitempty"`
	RequiresStop    bool   `json:"requiresStop"`
	EstimatedTime   int64  `json:"estimatedTime"` // 秒
	DataSize        uint64 `json:"dataSize"`
	VolumeCount     int    `json:"volumeCount"`
}

// AppStorageUsage 应用存储使用情况
type AppStorageUsage struct {
	AppID       string            `json:"appId"`
	Volumes     []VolumeUsage     `json:"volumes"`
	TotalSize   uint64            `json:"totalSize"`
	ConfigSize  uint64            `json:"configSize"`
}

// VolumeUsage 卷使用情况
type VolumeUsage struct {
	VolumeName  string `json:"volumeName"`
	Path        string `json:"path"`
	SizeBytes   uint64 `json:"sizeBytes"`
	BindMount   bool   `json:"bindMount"`
}

// AppController 应用控制器接口
type AppController interface {
	StopApp(appID string) error
	StartApp(appID string) error
	GetAppStatus(appID string) (string, error)
	GetAppPool(appID string) (string, error)
	UpdateAppPool(appID string, poolID string) error
	ListAppsOnPool(poolID string) ([]string, error)
}

// ProgressTracker 进度追踪器
type ProgressTracker struct {
	transferred    uint64
	speed          uint64
	startTime      time.Time
	lastUpdate     time.Time
	bytesPerUpdate uint64
}

// MigrationEventHandler 迁移事件处理器
type MigrationEventHandler interface {
	OnMigrationStart(record *MigrationRecord)
	OnMigrationProgress(record *MigrationRecord, progress *MigrationProgress)
	OnMigrationComplete(record *MigrationRecord)
	OnMigrationFailed(record *MigrationRecord, err error)
	OnRollbackStart(record *MigrationRecord)
	OnRollbackComplete(record *MigrationRecord)
}

// MigrationLogger 迁移日志接口
type MigrationLogger interface {
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// MigrationStorage 迁移存储接口
type MigrationStorage interface {
	SaveCheckpoint(checkpoint *MigrationCheckpoint) error
	LoadCheckpoint(migrationID string) (*MigrationCheckpoint, error)
	DeleteCheckpoint(migrationID string) error
	SaveMigrationRecord(record *MigrationRecord) error
	LoadMigrationRecord(id string) (*MigrationRecord, error)
	ListMigrationRecords(limit int) ([]*MigrationRecord, error)
}

// NewPoolMigrationManager 创建应用池迁移管理器
func NewPoolMigrationManager(logger MigrationLogger, storage MigrationStorage) *PoolMigrationManager {
	return &PoolMigrationManager{
		checkpoints:    make(map[string]*MigrationCheckpoint),
		eventHandlers:  make([]MigrationEventHandler, 0),
		logger:         logger,
		storage:        storage,
		progressTracker: &ProgressTracker{},
	}
}

// SetHealthChecker 设置健康检查器
func (m *PoolMigrationManager) SetHealthChecker(checker PoolHealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthChecker = checker
}

// SetAppValidator 设置应用验证器
func (m *PoolMigrationManager) SetAppValidator(validator AppMigrationValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appValidator = validator
}

// SetAppController 设置应用控制器
func (m *PoolMigrationManager) SetAppController(controller AppController) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appController = controller
}

// getHealthChecker 获取健康检查器
func (m *PoolMigrationManager) getHealthChecker() PoolHealthChecker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthChecker
}

// getAppValidator 获取应用验证器
func (m *PoolMigrationManager) getAppValidator() AppMigrationValidator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.appValidator
}

// getAppController 获取应用控制器
func (m *PoolMigrationManager) getAppController() AppController {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.appController
}

// listAppsOnPool 获取池上的应用列表
func (m *PoolMigrationManager) listAppsOnPool(poolID string) ([]string, error) {
	controller := m.getAppController()
	if controller == nil {
		return nil, errors.New("app controller not configured")
	}
	return controller.ListAppsOnPool(poolID)
}

// isAppOnPool 检查应用是否在指定池上
func (m *PoolMigrationManager) isAppOnPool(appID string, poolID string) bool {
	controller := m.getAppController()
	if controller == nil {
		return false
	}
	appPool, err := controller.GetAppPool(appID)
	if err != nil {
		return false
	}
	return appPool == poolID
}

// RegisterEventHandler 注册事件处理器
func (m *PoolMigrationManager) RegisterEventHandler(handler MigrationEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)
}

// StartMigration 启动迁移任务
func (m *PoolMigrationManager) StartMigration(ctx context.Context, config *MigrationConfig, initiatedBy string) (*MigrationRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 检查是否有正在进行的迁移
	if m.activeMigration != nil && 
		m.activeMigration.Status != MigrationStatusCompleted &&
		m.activeMigration.Status != MigrationStatusFailed &&
		m.activeMigration.Status != MigrationStatusRolledBack {
		return nil, ErrMigrationInProgress
	}
	
	// 验证配置
	if err := m.validateConfig(config); err != nil {
		return nil, err
	}
	
	// 创建迁移记录
	record := &MigrationRecord{
		ID:           generateMigrationID(),
		Config:       *config,
		Status:       MigrationStatusPending,
		Progress:     MigrationProgress{Phase: MigrationPhaseInit},
		StartTime:    time.Now(),
		InitiatedBy:  initiatedBy,
		Checkpoints:  make([]MigrationCheckpoint, 0),
	}
	
	// 保存初始检查点
	checkpoint := m.createCheckpoint(record, MigrationPhaseInit)
	if err := m.saveCheckpoint(checkpoint); err != nil {
		return nil, fmt.Errorf("failed to save initial checkpoint: %w", err)
	}
	record.Checkpoints = append(record.Checkpoints, *checkpoint)
	
	// 设置为活跃迁移
	m.activeMigration = record
	
	// 触发事件
	m.emitMigrationStart(record)
	
	// 异步执行迁移
	go m.executeMigration(config, record)
	
	return record, nil
}

// GetMigrationStatus 获取迁移状态
func (m *PoolMigrationManager) GetMigrationStatus(id string) (*MigrationRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 检查是否是当前活跃迁移
	if m.activeMigration != nil && m.activeMigration.ID == id {
		return m.activeMigration, nil
	}
	
	// 从存储加载
	return m.storage.LoadMigrationRecord(id)
}

// GetActiveMigration 获取当前活跃迁移
func (m *PoolMigrationManager) GetActiveMigration() *MigrationRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeMigration
}

// CancelMigration 取消迁移
func (m *PoolMigrationManager) CancelMigration(ctx context.Context, id string, rollback bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.activeMigration == nil || m.activeMigration.ID != id {
		return errors.New("no active migration with given id")
	}
	
	// 更新状态
	if rollback {
		m.activeMigration.Status = MigrationStatusRollingBack
		m.activeMigration.CancelReason = "user_cancelled"
		go m.performRollback(m.activeMigration)
	} else {
		now := time.Now()
		m.activeMigration.Status = MigrationStatusFailed
		m.activeMigration.EndTime = &now
		m.activeMigration.CancelReason = "user_cancelled"
		m.activeMigration = nil
	}
	
	return nil
}

// RollbackMigration 回滚迁移
func (m *PoolMigrationManager) RollbackMigration(ctx context.Context, id string) error {
	m.mu.Lock()
	
	if m.activeMigration == nil || m.activeMigration.ID != id {
		m.mu.Unlock()
		return errors.New("no active migration with given id")
	}
	
	m.activeMigration.Status = MigrationStatusRollingBack
	m.mu.Unlock()
	
	return m.performRollback(m.activeMigration)
}

// ListMigrationHistory 列出迁移历史
func (m *PoolMigrationManager) ListMigrationHistory(limit int) ([]*MigrationRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if limit <= 0 {
		limit = 100
	}
	
	return m.storage.ListMigrationRecords(limit)
}

// ========== 内部方法 ==========

// validateConfig 验证迁移配置
func (m *PoolMigrationManager) validateConfig(config *MigrationConfig) error {
	if config.SourcePoolID == "" {
		return ErrInvalidSourcePool
	}
	if config.TargetPoolID == "" {
		return ErrInvalidTargetPool
	}
	if config.SourcePoolID == config.TargetPoolID {
		return errors.New("source and target pool cannot be the same")
	}
	if config.MaxParallel < 1 {
		config.MaxParallel = 1
	}
	if config.Timeout < 0 {
		config.Timeout = 0 // 无限等待
	}
	return nil
}

// createCheckpoint 创建检查点
func (m *PoolMigrationManager) createCheckpoint(record *MigrationRecord, phase MigrationPhase) *MigrationCheckpoint {
	return &MigrationCheckpoint{
		ID:          generateCheckpointID(),
		MigrationID: record.ID,
		Phase:       phase,
		Timestamp:   time.Now(),
		AppStates:   make(map[string]AppState),
		VolumeStates: make(map[string]VolumeState),
	}
}

// saveCheckpoint 保存检查点
func (m *PoolMigrationManager) saveCheckpoint(checkpoint *MigrationCheckpoint) error {
	m.mu.Lock()
	m.checkpoints[checkpoint.MigrationID] = checkpoint
	m.mu.Unlock()
	return m.storage.SaveCheckpoint(checkpoint)
}

// executeMigration 执行迁移流程
func (m *PoolMigrationManager) executeMigration(config *MigrationConfig, record *MigrationRecord) {
	defer func() {
		m.mu.Lock()
		if m.activeMigration == record {
			m.activeMigration = nil
		}
		m.mu.Unlock()
	}()
	
	// 阶段1: 准备和验证
	if err := m.phasePrepareAndValidate(config, record); err != nil {
		m.handleMigrationFailure(record, err, config.AutoRollback)
		return
	}
	
	// 阶段2: 停止应用
	if err := m.phaseStopApps(config, record); err != nil {
		m.handleMigrationFailure(record, err, config.AutoRollback)
		return
	}
	
	// 阶段3: 数据传输
	if err := m.phaseTransferData(config, record); err != nil {
		m.handleMigrationFailure(record, err, config.AutoRollback)
		return
	}
	
	// 阶段4: 更新配置
	if err := m.phaseUpdateConfig(config, record); err != nil {
		m.handleMigrationFailure(record, err, config.AutoRollback)
		return
	}
	
	// 阶段5: 启动应用
	if err := m.phaseStartApps(config, record); err != nil {
		m.handleMigrationFailure(record, err, config.AutoRollback)
		return
	}
	
	// 阶段6: 验证
	if err := m.phaseVerify(config, record); err != nil {
		m.handleMigrationFailure(record, err, config.AutoRollback)
		return
	}
	
	// 阶段7: 清理
	m.phaseCleanup(config, record)
	
	// 完成
	now := time.Now()
	m.mu.Lock()
	record.Status = MigrationStatusCompleted
	record.EndTime = &now
	record.Duration = int64(now.Sub(record.StartTime).Seconds())
	record.Progress.Percent = 100
	record.Progress.Phase = MigrationPhaseCleanup
	m.mu.Unlock()
	
	m.saveMigrationRecord(record)
	m.emitMigrationComplete(record)
}

// PoolHealthChecker 存储池健康检查器接口
type PoolHealthChecker interface {
	CheckPoolHealth(poolID string) (*PoolHealthStatus, error)
	GetPoolCapacity(poolID string) (*PoolCapacityInfo, error)
	IsPoolAvailable(poolID string) bool
}

// PoolHealthStatus 存储池健康状态
type PoolHealthStatus struct {
	PoolID       string    `json:"poolId"`
	Healthy      bool      `json:"healthy"`
	Status       string    `json:"status"`
	HealthScore  float64   `json:"healthScore"`  // 0-100
	Errors       []string  `json:"errors"`
	Warnings     []string  `json:"warnings"`
	LastChecked  time.Time `json:"lastChecked"`
}

// PoolCapacityInfo 存储池容量信息
type PoolCapacityInfo struct {
	PoolID        string `json:"poolId"`
	TotalBytes    uint64 `json:"totalBytes"`
	UsedBytes     uint64 `json:"usedBytes"`
	AvailableBytes uint64 `json:"availableBytes"`
	UsedPercent   float64 `json:"usedPercent"`
}

// AppMigrationValidator 应用迁移验证器接口
type AppMigrationValidator interface {
	ValidateAppMigration(appID string, sourcePoolID, targetPoolID string) (*AppMigrationValidation, error)
	CheckAppDependencies(appID string) ([]string, error)
	GetAppStorageUsage(appID string) (*AppStorageUsage, error)
}

// AppMigrationValidation 应用迁移验证结果
type AppMigrationValidation struct {
	AppID           string `json:"appId"`
	CanMigrate      bool   `json:"canMigrate"`
	Reason          string `json:"reason,omitempty"`
	RequiresStop    bool   `json:"requiresStop"`
	EstimatedTime   int64  `json:"estimatedTime"` // 秒
	DataSize        uint64 `json:"dataSize"`
	VolumeCount     int    `json:"volumeCount"`
}

// AppStorageUsage 应用存储使用情况
type AppStorageUsage struct {
	AppID       string            `json:"appId"`
	Volumes     []VolumeUsage     `json:"volumes"`
	TotalSize   uint64            `json:"totalSize"`
	ConfigSize  uint64            `json:"configSize"`
}

// VolumeUsage 卷使用情况
type VolumeUsage struct {
	VolumeName  string `json:"volumeName"`
	Path        string `json:"path"`
	SizeBytes   uint64 `json:"sizeBytes"`
	BindMount   bool   `json:"bindMount"`
}

// phasePrepareAndValidate 准备和验证阶段
func (m *PoolMigrationManager) phasePrepareAndValidate(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseValidate, 0)
	
	// 获取健康检查器和验证器
	healthChecker := m.getHealthChecker()
	appValidator := m.getAppValidator()
	
	if healthChecker == nil || appValidator == nil {
		return errors.New("health checker or app validator not configured")
	}
	
	// 1. 验证源存储池状态和健康度
	sourceHealth, err := healthChecker.CheckPoolHealth(config.SourcePoolID)
	if err != nil {
		return fmt.Errorf("failed to check source pool health: %w", err)
	}
	if !sourceHealth.Healthy || sourceHealth.HealthScore < 50 {
		record.Progress.Warnings = append(record.Progress.Warnings,
			fmt.Sprintf("Source pool health degraded: score %.1f", sourceHealth.HealthScore))
		if sourceHealth.HealthScore < 30 {
			return ErrSourcePoolNotHealthy
		}
	}
	m.updateProgress(record, MigrationPhaseValidate, 1)
	
	// 2. 验证目标存储池状态和健康度
	targetHealth, err := healthChecker.CheckPoolHealth(config.TargetPoolID)
	if err != nil {
		return fmt.Errorf("failed to check target pool health: %w", err)
	}
	if !targetHealth.Healthy || targetHealth.HealthScore < 50 {
		return ErrTargetPoolNotHealthy
	}
	if !healthChecker.IsPoolAvailable(config.TargetPoolID) {
		return errors.New("target pool is not available for migration")
	}
	m.updateProgress(record, MigrationPhaseValidate, 2)
	
	// 3. 检查目标存储池空间是否足够
	targetCapacity, err := healthChecker.GetPoolCapacity(config.TargetPoolID)
	if err != nil {
		return fmt.Errorf("failed to get target pool capacity: %w", err)
	}
	
	// 计算需要迁移的总数据量
	var totalDataSize uint64
	var appsToMigrate []string
	
	if len(config.AppIDs) == 0 {
		// 迁移池上所有应用
		appsToMigrate, err = m.listAppsOnPool(config.SourcePoolID)
		if err != nil {
			return fmt.Errorf("failed to list apps on source pool: %w", err)
		}
	} else {
		appsToMigrate = config.AppIDs
	}
	
	for _, appID := range appsToMigrate {
		usage, err := appValidator.GetAppStorageUsage(appID)
		if err != nil {
			record.Progress.Warnings = append(record.Progress.Warnings,
				fmt.Sprintf("Failed to get storage usage for app %s: %v", appID, err))
			continue
		}
		totalDataSize += usage.TotalSize
	}
	
	// 检查空间是否足够（预留 10% 缓冲）
	requiredSpace := totalDataSize + uint64(float64(totalDataSize)*0.1)
	if targetCapacity.AvailableBytes < requiredSpace {
		return ErrInsufficientSpace
	}
	
	record.Progress.BytesTotal = totalDataSize
	record.Progress.AppsTotal = len(appsToMigrate)
	m.updateProgress(record, MigrationPhaseValidate, 3)
	
	// 4. 验证应用是否支持迁移
	var unsupportedApps []string
	for _, appID := range appsToMigrate {
		validation, err := appValidator.ValidateAppMigration(appID, config.SourcePoolID, config.TargetPoolID)
		if err != nil {
			record.Progress.Warnings = append(record.Progress.Warnings,
				fmt.Sprintf("Failed to validate app %s: %v", appID, err))
			continue
		}
		if !validation.CanMigrate {
			unsupportedApps = append(unsupportedApps, appID)
			record.Progress.Errors = append(record.Progress.Errors, MigrationErrorInfo{
				Time:    time.Now(),
				AppID:   appID,
				Message: validation.Reason,
				Code:    "APP_NOT_SUPPORTED",
			})
		}
	}
	
	if len(unsupportedApps) > 0 && len(unsupportedApps) == len(appsToMigrate) {
		return ErrAppNotSupported
	}
	
	// 跳过不支持的应用
	record.Progress.AppsSkipped = len(unsupportedApps)
	m.updateProgress(record, MigrationPhaseValidate, 4)
	
	// 5. 检查应用依赖关系
	for _, appID := range appsToMigrate {
		deps, err := appValidator.CheckAppDependencies(appID)
		if err != nil {
			continue
		}
		
		// 检查依赖是否在同一池上
		for _, depID := range deps {
			if !m.isAppOnPool(depID, config.SourcePoolID) {
				record.Progress.Warnings = append(record.Progress.Warnings,
					fmt.Sprintf("App %s depends on %s which is not on source pool", appID, depID))
			}
		}
	}
	
	m.updateProgress(record, MigrationPhaseValidate, 5)
	
	// 创建检查点
	checkpoint := m.createCheckpoint(record, MigrationPhaseValidate)
	checkpoint.Data["appsToMigrate"] = appsToMigrate
	checkpoint.Data["totalDataSize"] = totalDataSize
	checkpoint.Data["sourceHealth"] = sourceHealth
	checkpoint.Data["targetHealth"] = targetHealth
	m.saveCheckpoint(checkpoint)
	
	m.logger.Info("Validation phase completed",
		"appsToMigrate", len(appsToMigrate),
		"appsSkipped", len(unsupportedApps),
		"totalDataSize", totalDataSize,
	)
	
	return nil
}

// phaseStopApps 停止应用阶段
func (m *PoolMigrationManager) phaseStopApps(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseStopApps, 10)
	
	// TODO: 实现应用停止逻辑
	// 1. 按依赖顺序停止应用
	// 2. 等待应用完全停止
	// 3. 记录应用状态
	
	checkpoint := m.createCheckpoint(record, MigrationPhaseStopApps)
	m.saveCheckpoint(checkpoint)
	
	return nil
}

// phaseTransferData 数据传输阶段
func (m *PoolMigrationManager) phaseTransferData(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseTransferData, 20)
	
	// TODO: 实现数据传输逻辑
	// 1. 创建目标卷
	// 2. 复制数据（支持压缩、限速）
	// 3. 更新传输进度
	// 4. 验证数据完整性（如果配置）
	
	// 模拟进度更新
	for i := 20; i <= 80; i += 10 {
		m.updateProgress(record, MigrationPhaseTransferData, float64(i))
		time.Sleep(100 * time.Millisecond) // 实际实现中移除
	}
	
	return nil
}

// phaseUpdateConfig 更新配置阶段
func (m *PoolMigrationManager) phaseUpdateConfig(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseUpdateConfig, 85)
	
	// TODO: 实现配置更新逻辑
	// 1. 更新应用配置中的池路径
	// 2. 更新 Docker/容器配置
	// 3. 更新卷挂载点
	
	checkpoint := m.createCheckpoint(record, MigrationPhaseUpdateConfig)
	m.saveCheckpoint(checkpoint)
	
	return nil
}

// phaseStartApps 启动应用阶段
func (m *PoolMigrationManager) phaseStartApps(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseStartApps, 90)
	
	// TODO: 实现应用启动逻辑
	// 1. 按依赖顺序启动应用
	// 2. 验证应用健康状态
	
	return nil
}

// phaseVerify 验证阶段
func (m *PoolMigrationManager) phaseVerify(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseVerify, 95)
	
	// TODO: 实现验证逻辑
	// 1. 验证应用运行状态
	// 2. 验证数据完整性（如果配置）
	// 3. 验证网络连接
	// 4. 验证存储挂载
	
	return nil
}

// phaseCleanup 清理阶段
func (m *PoolMigrationManager) phaseCleanup(config *MigrationConfig, record *MigrationRecord) {
	m.updateProgress(record, MigrationPhaseCleanup, 98)
	
	// TODO: 实现清理逻辑
	// 1. 如果不保留源数据，删除源池数据
	// 2. 清理临时文件
	// 3. 归档迁移记录
}

// performRollback 执行回滚
func (m *PoolMigrationManager) performRollback(record *MigrationRecord) error {
	m.emitRollbackStart(record)
	
	// 获取最近的检查点
	var lastCheckpoint *MigrationCheckpoint
	m.mu.RLock()
	if cp, exists := m.checkpoints[record.ID]; exists {
		lastCheckpoint = cp
	}
	m.mu.RUnlock()
	
	if lastCheckpoint == nil && len(record.Checkpoints) > 0 {
		cp := record.Checkpoints[len(record.Checkpoints)-1]
		lastCheckpoint = &cp
	}
	
	// TODO: 实现回滚逻辑
	// 1. 恢复应用配置
	// 2. 删除目标池数据
	// 3. 恢复源池数据（如果有修改）
	// 4. 恢复应用运行状态
	
	now := time.Now()
	m.mu.Lock()
	record.Status = MigrationStatusRolledBack
	record.EndTime = &now
	record.Duration = int64(now.Sub(record.StartTime).Seconds())
	m.activeMigration = nil
	m.mu.Unlock()
	
	m.saveMigrationRecord(record)
	m.emitRollbackComplete(record)
	
	return nil
}

// handleMigrationFailure 处理迁移失败
func (m *PoolMigrationManager) handleMigrationFailure(record *MigrationRecord, err error, autoRollback bool) {
	m.mu.Lock()
	record.Status = MigrationStatusFailed
	now := time.Now()
	record.EndTime = &now
	record.Duration = int64(now.Sub(record.StartTime).Seconds())
	record.Progress.Errors = append(record.Progress.Errors, MigrationErrorInfo{
		Time:    time.Now(),
		Message: err.Error(),
		Code:    "MIGRATION_FAILED",
	})
	m.mu.Unlock()
	
	m.saveMigrationRecord(record)
	m.emitMigrationFailed(record, err)
	
	if autoRollback {
		m.performRollback(record)
	}
}

// updateProgress 更新进度
func (m *PoolMigrationManager) updateProgress(record *MigrationRecord, phase MigrationPhase, percent float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record.Progress.Phase = phase
	record.Progress.Percent = percent
	m.emitMigrationProgress(record, &record.Progress)
}

// saveMigrationRecord 保存迁移记录
func (m *PoolMigrationManager) saveMigrationRecord(record *MigrationRecord) error {
	return m.storage.SaveMigrationRecord(record)
}

// ========== 事件发射方法 ==========

func (m *PoolMigrationManager) emitMigrationStart(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnMigrationStart(record)
	}
}

func (m *PoolMigrationManager) emitMigrationProgress(record *MigrationRecord, progress *MigrationProgress) {
	for _, h := range m.eventHandlers {
		h.OnMigrationProgress(record, progress)
	}
}

func (m *PoolMigrationManager) emitMigrationComplete(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnMigrationComplete(record)
	}
}

func (m *PoolMigrationManager) emitMigrationFailed(record *MigrationRecord, err error) {
	for _, h := range m.eventHandlers {
		h.OnMigrationFailed(record, err)
	}
}

func (m *PoolMigrationManager) emitRollbackStart(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnRollbackStart(record)
	}
}

func (m *PoolMigrationManager) emitRollbackComplete(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnRollbackComplete(record)
	}
}

// ========== 辅助函数 ==========

// generateMigrationID 生成迁移ID
func generateMigrationID() string {
	return fmt.Sprintf("migration-%d", time.Now().UnixNano())
}

// generateCheckpointID 生成检查点ID
func generateCheckpointID() string {
	return fmt.Sprintf("checkpoint-%d", time.Now().UnixNano())
}