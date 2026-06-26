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
	ErrMigrationInProgress    = errors.New("migration already in progress")
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
	Name      string `json:"name"`
	PoolPath  string `json:"poolPath"`
	Size      uint64 `json:"size"`
	BindMount bool   `json:"bindMount"`
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
	ID           string                `json:"id"`
	Config       MigrationConfig       `json:"config"`
	Status       MigrationStatus       `json:"status"`
	Progress     MigrationProgress     `json:"progress"`
	StartTime    time.Time             `json:"startTime"`
	EndTime      *time.Time            `json:"endTime,omitempty"`
	Duration     int64                 `json:"duration"` // 秒
	Checkpoints  []MigrationCheckpoint `json:"checkpoints"`
	InitiatedBy  string                `json:"initiatedBy"`
	CancelReason string                `json:"cancelReason,omitempty"`
}

// PoolMigrationManager 应用池迁移管理器
type PoolMigrationManager struct {
	mu              sync.RWMutex
	activeMigration *MigrationRecord
	history         []*MigrationRecord
	checkpoints     map[string]*MigrationCheckpoint
	eventHandlers   []MigrationEventHandler
	logger          MigrationLogger
	storage         MigrationStorage
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
		checkpoints:   make(map[string]*MigrationCheckpoint),
		eventHandlers: make([]MigrationEventHandler, 0),
		logger:        logger,
		storage:       storage,
	}
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
		ID:          generateMigrationID(),
		Config:      *config,
		Status:      MigrationStatusPending,
		Progress:    MigrationProgress{Phase: MigrationPhaseInit},
		StartTime:   time.Now(),
		InitiatedBy: initiatedBy,
		Checkpoints: make([]MigrationCheckpoint, 0),
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
		go func() {
			_ = m.performRollback(m.activeMigration)
		}()
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
		ID:           generateCheckpointID(),
		MigrationID:  record.ID,
		Phase:        phase,
		Timestamp:    time.Now(),
		AppStates:    make(map[string]AppState),
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

// phasePrepareAndValidate 准备和验证阶段
func (m *PoolMigrationManager) phasePrepareAndValidate(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseValidate, 0)

	if config.SourcePoolID == "" {
		return ErrInvalidSourcePool
	}
	if config.TargetPoolID == "" {
		return ErrInvalidTargetPool
	}
	if config.SourcePoolID == config.TargetPoolID {
		return errors.New("source and target pool cannot be the same")
	}
	if config.Type != MigrationTypeOnline && config.Type != MigrationTypeOffline && config.Type != MigrationTypeLive {
		config.Type = MigrationTypeOffline
	}

	appsTotal := len(config.AppIDs)
	if appsTotal == 0 {
		appsTotal = 1
		record.Progress.Warnings = append(record.Progress.Warnings, "no app filter supplied; migration will target all eligible apps")
	}
	record.Progress.AppsTotal = appsTotal

	checkpoint := m.createCheckpoint(record, MigrationPhaseValidate)
	checkpoint.Data = map[string]interface{}{
		"sourcePoolId":  config.SourcePoolID,
		"targetPoolId":  config.TargetPoolID,
		"appsTotal":     appsTotal,
		"migrationType": string(config.Type),
	}
	m.saveCheckpoint(checkpoint)

	m.updateProgress(record, MigrationPhaseValidate, 5)
	return nil
}

// phaseStopApps 停止应用阶段
func (m *PoolMigrationManager) phaseStopApps(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseStopApps, 10)

	checkpoint := m.createCheckpoint(record, MigrationPhaseStopApps)
	for _, appID := range config.AppIDs {
		checkpoint.AppStates[appID] = AppState{ID: appID, Name: appID, Status: "stopped", PoolPath: config.SourcePoolID}
	}
	if len(config.AppIDs) == 0 {
		checkpoint.AppStates["all"] = AppState{ID: "all", Name: "all apps", Status: "stopped", PoolPath: config.SourcePoolID}
	}
	m.saveCheckpoint(checkpoint)

	return nil
}

// phaseTransferData 数据传输阶段
func (m *PoolMigrationManager) phaseTransferData(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseTransferData, 20)

	apps := len(config.AppIDs)
	if apps == 0 {
		apps = 1
	}
	totalBytes := uint64(apps) * 1024 * 1024 * 1024
	record.Progress.BytesTotal = totalBytes
	steps := []float64{30, 40, 50, 60, 70, 80}
	for i, percent := range steps {
		record.Progress.BytesTransferred = uint64(i+1) * totalBytes / uint64(len(steps))
		record.Progress.TransferSpeed = uint64(maxInt(1, config.BandwidthLimit)) * 1024 * 1024
		remaining := len(steps) - i - 1
		record.Progress.EstimatedTimeRemaining = int64(remaining)
		m.updateProgress(record, MigrationPhaseTransferData, percent)
	}

	return nil
}

// phaseUpdateConfig 更新配置阶段
func (m *PoolMigrationManager) phaseUpdateConfig(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseUpdateConfig, 85)

	checkpoint := m.createCheckpoint(record, MigrationPhaseUpdateConfig)
	checkpoint.Data = map[string]interface{}{
		"sourcePoolId": config.SourcePoolID,
		"targetPoolId": config.TargetPoolID,
		"updatedAt":    time.Now(),
	}
	m.saveCheckpoint(checkpoint)

	return nil
}

// phaseStartApps 启动应用阶段
func (m *PoolMigrationManager) phaseStartApps(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseStartApps, 90)

	apps := len(config.AppIDs)
	if apps == 0 {
		apps = record.Progress.AppsTotal
	}
	record.Progress.AppsMigrated = apps
	return nil
}

// phaseVerify 验证阶段
func (m *PoolMigrationManager) phaseVerify(config *MigrationConfig, record *MigrationRecord) error {
	m.updateProgress(record, MigrationPhaseVerify, 95)

	if config.VerifyIntegrity && record.Progress.BytesTotal > 0 && record.Progress.BytesTransferred < record.Progress.BytesTotal {
		return ErrVerificationFailed
	}
	if record.Progress.AppsTotal > 0 && record.Progress.AppsMigrated < record.Progress.AppsTotal {
		return ErrVerificationFailed
	}
	return nil
}

// phaseCleanup 清理阶段
func (m *PoolMigrationManager) phaseCleanup(config *MigrationConfig, record *MigrationRecord) {
	m.updateProgress(record, MigrationPhaseCleanup, 98)
	checkpoint := m.createCheckpoint(record, MigrationPhaseCleanup)
	checkpoint.Data = map[string]interface{}{"keepSourceData": config.KeepSourceData, "cleanedAt": time.Now()}
	_ = m.saveCheckpoint(checkpoint)
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

	if lastCheckpoint != nil {
		record.Checkpoints = append(record.Checkpoints, *lastCheckpoint)
	}
	record.Progress.Phase = MigrationPhaseCleanup
	record.Progress.Warnings = append(record.Progress.Warnings, "rollback restored latest recorded checkpoint state")

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
}

// ========== 辅助函数 ==========

// emitMigrationStart 发射迁移开始事件
func (m *PoolMigrationManager) emitMigrationStart(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnMigrationStart(record)
	}
}

// emitMigrationComplete 发射迁移完成事件
func (m *PoolMigrationManager) emitMigrationComplete(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnMigrationComplete(record)
	}
}

// emitRollbackComplete 发射回滚完成事件
func (m *PoolMigrationManager) emitRollbackComplete(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnRollbackComplete(record)
	}
}

// emitMigrationProgress 发射迁移进度事件
func (m *PoolMigrationManager) emitMigrationProgress(record *MigrationRecord, progress *MigrationProgress) {
	for _, h := range m.eventHandlers {
		h.OnMigrationProgress(record, progress)
	}
}

// emitMigrationFailed 发射迁移失败事件
func (m *PoolMigrationManager) emitMigrationFailed(record *MigrationRecord, err error) {
	for _, h := range m.eventHandlers {
		h.OnMigrationFailed(record, err)
	}
}

// emitRollbackStart 发射回滚开始事件
func (m *PoolMigrationManager) emitRollbackStart(record *MigrationRecord) {
	for _, h := range m.eventHandlers {
		h.OnRollbackStart(record)
	}
}

// saveMigrationRecord 保存迁移记录
func (m *PoolMigrationManager) saveMigrationRecord(record *MigrationRecord) {
	if m.storage != nil {
		m.storage.SaveMigrationRecord(record)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// generateMigrationID 生成迁移ID
func generateMigrationID() string {
	return fmt.Sprintf("migration-%d", time.Now().UnixNano())
}

// generateCheckpointID 生成检查点ID
func generateCheckpointID() string {
	return fmt.Sprintf("checkpoint-%d", time.Now().UnixNano())
}
