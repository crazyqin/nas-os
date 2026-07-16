package domainsync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SyncEngine 同步引擎.
type SyncEngine struct {
	mu         sync.Mutex
	config     SyncConfig
	ouSyncer   *OUDiscoverer
	running    bool
	cancel     context.CancelFunc
	lastResult *SyncResult
	status     SyncStatus
	progress   int
}

// NewSyncEngine 创建同步引擎.
func NewSyncEngine(config SyncConfig) *SyncEngine {
	return &SyncEngine{
		config:   config,
		ouSyncer: NewOUDiscoverer(config.DCConfig),
		status:   SyncStatusIdle,
	}
}

// SyncOnce 执行一次同步.
func (e *SyncEngine) SyncOnce(ctx context.Context) (*SyncResult, error) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil, ErrSyncInProgress
	}
	// 复制配置到局部变量，避免后续访问 e.config 的数据竞争
	cfg := e.config
	e.running = true
	e.status = SyncStatusRunning
	e.progress = 0
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()

	result := &SyncResult{
		ID:        uuid.New().String(),
		StartTime: time.Now(),
		Status:    SyncStatusRunning,
		Strategy:  cfg.Strategy,
	}

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	ouSyncer := NewOUDiscoverer(cfg.DCConfig)

	// 步骤1: 发现 OU
	if err := syncOUs(ctx, ouSyncer, cfg, result); err != nil {
		result.Status = SyncStatusFailed
		result.Message = fmt.Sprintf("OU 同步失败: %v", err)
		e.mu.Lock()
		e.status = SyncStatusFailed
		e.lastResult = result
		e.mu.Unlock()
		return result, err
	}

	e.mu.Lock()
	e.progress = 30
	e.mu.Unlock()

	// 步骤2: 同步用户（如果启用）
	if cfg.SyncUsers {
		if err := syncUsers(ctx, cfg, result); err != nil {
			result.Errors = append(result.Errors, SyncError{
				Type:    "user",
				Message: err.Error(),
			})
		}
	}

	e.mu.Lock()
	e.progress = 65
	e.mu.Unlock()

	// 步骤3: 同步组（如果启用）
	if cfg.SyncGroups {
		if err := syncGroups(ctx, cfg, result); err != nil {
			result.Errors = append(result.Errors, SyncError{
				Type:    "group",
				Message: err.Error(),
			})
		}
	}

	e.mu.Lock()
	e.progress = 100
	e.status = SyncStatusCompleted
	e.lastResult = result
	e.mu.Unlock()

	result.Status = SyncStatusCompleted
	result.Success = true
	result.Message = "同步完成"

	return result, nil
}

// syncOUs 同步组织单元（包级函数，避免访问 e 的字段）.
func syncOUs(ctx context.Context, discoverer *OUDiscoverer, cfg SyncConfig, result *SyncResult) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	ous, err := discoverer.Discover()
	if err != nil {
		return fmt.Errorf("OU 发现失败: %w", err)
	}

	// 如果配置了选择性同步，过滤 OU
	if len(cfg.SelectedOUs) > 0 {
		filtered := make([]*OU, 0)
		selectedSet := make(map[string]bool, len(cfg.SelectedOUs))
		for _, dn := range cfg.SelectedOUs {
			selectedSet[dn] = true
		}
		for _, ou := range ous {
			if selectedSet[ou.DN] {
				filtered = append(filtered, ou)
			}
		}
		ous = filtered
	}

	result.OUSynced = len(ous)
	return nil
}

// syncUsers 同步用户.
func syncUsers(_ context.Context, cfg SyncConfig, result *SyncResult) error {
	switch cfg.Strategy {
	case SyncStrategyFull:
		result.UsersSynced = 0
		result.UsersCreated = 0
		result.UsersUpdated = 0
	case SyncStrategyIncremental:
		result.UsersSynced = 0
		result.UsersUpdated = 0
	case SyncStrategyScheduled:
		result.UsersSynced = 0
		result.UsersUpdated = 0
	}
	return nil
}

// syncGroups 同步组.
func syncGroups(_ context.Context, cfg SyncConfig, result *SyncResult) error {
	switch cfg.Strategy {
	case SyncStrategyFull:
		result.GroupsSynced = 0
		result.GroupsCreated = 0
		result.GroupsUpdated = 0
	case SyncStrategyIncremental, SyncStrategyScheduled:
		result.GroupsSynced = 0
		result.GroupsUpdated = 0
	}
	return nil
}

// StartScheduled 启动定时同步.
func (e *SyncEngine) StartScheduled(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return ErrSyncInProgress
	}

	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.running = true
	e.status = SyncStatusRunning

	go e.scheduleLoop(ctx)

	return nil
}

// Stop 停止同步引擎.
func (e *SyncEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	e.running = false
	e.status = SyncStatusIdle
}

// scheduleLoop 定时同步循环.
func (e *SyncEngine) scheduleLoop(ctx context.Context) {
	e.mu.Lock()
	interval := e.config.ScheduleInterval
	e.mu.Unlock()

	if interval <= 0 {
		interval = 1 * time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即执行一次
	result, err := e.SyncOnce(ctx)
	if err != nil {
		log.Printf("域同步失败: %v", err)
	}
	e.mu.Lock()
	e.lastResult = result
	e.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := e.SyncOnce(ctx)
			if err != nil {
				log.Printf("域同步失败: %v", err)
			}
			e.mu.Lock()
			e.lastResult = result
			e.mu.Unlock()
		}
	}
}

// IsRunning 检查是否正在运行.
func (e *SyncEngine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// GetStatus 获取当前状态.
func (e *SyncEngine) GetStatus() SyncStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

// GetProgress 获取同步进度.
func (e *SyncEngine) GetProgress() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.progress
}

// GetLastResult 获取上次同步结果.
func (e *SyncEngine) GetLastResult() *SyncResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastResult
}

// UpdateConfig 更新配置.
func (e *SyncEngine) UpdateConfig(config SyncConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
	e.ouSyncer = NewOUDiscoverer(config.DCConfig)
}

// GetConfig 获取当前配置.
func (e *SyncEngine) GetConfig() SyncConfig {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.config
}

// TestConnection 测试域控制器连接.
func (e *SyncEngine) TestConnection() (bool, error) {
	e.mu.Lock()
	cfg := e.config.DCConfig
	e.mu.Unlock()
	discoverer := NewOUDiscoverer(cfg)
	_, err := discoverer.Discover()
	if err != nil {
		return false, err
	}
	return true, nil
}
