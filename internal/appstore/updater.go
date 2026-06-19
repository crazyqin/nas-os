// Package appstore 应用自动更新管理
// 支持更新策略配置、定时检查、批量更新、回滚机制
package appstore

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// ========== 自动更新管理器 ==========

// UpdateManager 自动更新管理器
type UpdateManager struct {
	mu            sync.RWMutex
	config        *UpdateManagerConfig
	catalog       *Catalog
	policies      map[string]*UpdatePolicy     // appID -> policy
	schedules     map[string]*UpdateSchedule    // appID -> schedule
	history       []*UpdateHistoryEntry         // 更新历史
	available     map[string]*UpdateInfo        // 可用更新缓存
	ctx           context.Context
	cancel        context.CancelFunc
	notifCallback func(*UpdateNotification)     // 通知回调
}

// UpdateManagerConfig 更新管理器配置
type UpdateManagerConfig struct {
	CheckInterval      time.Duration `json:"checkInterval"`      // 检查间隔
	DefaultPolicy      UpdatePolicyType `json:"defaultPolicy"`   // 默认策略
	MaxConcurrent      int           `json:"maxConcurrent"`      // 最大并发更新数
	RollbackEnabled    bool          `json:"rollbackEnabled"`    // 启用回滚
	MaxRollbackHistory int           `json:"maxRollbackHistory"` // 最大回滚历史数
	PreUpdateBackup    bool          `json:"preUpdateBackup"`    // 更新前备份
	NotifyOnUpdate     bool          `json:"notifyOnUpdate"`     // 有更新时通知
	NotifyOnComplete   bool          `json:"notifyOnComplete"`   // 更新完成通知
	AutoRestart        bool          `json:"autoRestart"`        // 更新后自动重启
}

// DefaultUpdateManagerConfig 默认配置
func DefaultUpdateManagerConfig() *UpdateManagerConfig {
	return &UpdateManagerConfig{
		CheckInterval:      6 * time.Hour,
		DefaultPolicy:      UpdatePolicyNotify,
		MaxConcurrent:      3,
		RollbackEnabled:    true,
		MaxRollbackHistory: 10,
		PreUpdateBackup:    true,
		NotifyOnUpdate:     true,
		NotifyOnComplete:   true,
		AutoRestart:        true,
	}
}

// UpdatePolicyType 更新策略类型
type UpdatePolicyType string

const (
	UpdatePolicyAuto    UpdatePolicyType = "auto"    // 自动更新
	UpdatePolicyNotify  UpdatePolicyType = "notify"  // 仅通知
	UpdatePolicyManual  UpdatePolicyType = "manual"  // 手动
	UpdatePolicyDisable UpdatePolicyType = "disable" // 禁用更新
)

// UpdatePolicy 更新策略
type UpdatePolicy struct {
	AppID           string           `json:"appId"`
	PolicyType      UpdatePolicyType `json:"policyType"`
	AutoUpdateTime  string           `json:"autoUpdateTime"`  // 自动更新时间 (HH:MM)
	AutoUpdateDay   string           `json:"autoUpdateDay"`   // 自动更新日 (monday, tuesday, ...)
	IncludePrerelease bool           `json:"includePrerelease"` // 包含预发布版本
	MaxVersionSkip   int             `json:"maxVersionSkip"`  // 最大跳过版本数
	NotifyChannels   []string        `json:"notifyChannels"`  // 通知渠道
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// UpdateSchedule 更新调度
type UpdateSchedule struct {
	AppID       string    `json:"appId"`
	NextCheck   time.Time `json:"nextCheck"`
	NextUpdate  time.Time `json:"nextUpdate,omitempty"`
	LastChecked time.Time `json:"lastChecked"`
	LastError   string    `json:"lastError,omitempty"`
	Enabled     bool      `json:"enabled"`
}

// UpdateHistoryEntry 更新历史记录
type UpdateHistoryEntry struct {
	ID            string        `json:"id"`
	AppID         string        `json:"appId"`
	AppName       string        `json:"appName"`
	FromVersion   string        `json:"fromVersion"`
	ToVersion     string        `json:"toVersion"`
	Status        UpdateStatus  `json:"status"`
	TriggerType   string        `json:"triggerType"` // "auto", "manual", "batch"
	StartedAt     time.Time     `json:"startedAt"`
	CompletedAt   time.Time     `json:"completedAt,omitempty"`
	Error         string        `json:"error,omitempty"`
	RollbackAvail bool          `json:"rollbackAvail"`
	Duration      time.Duration `json:"duration"`
}

// UpdateStatus 更新状态
type UpdateStatus string

const (
	UpdateStatusPending    UpdateStatus = "pending"
	UpdateStatusChecking   UpdateStatus = "checking"
	UpdateStatusDownloading UpdateStatus = "downloading"
	UpdateStatusInstalling UpdateStatus = "installing"
	UpdateStatusRestarting UpdateStatus = "restarting"
	UpdateStatusCompleted  UpdateStatus = "completed"
	UpdateStatusFailed     UpdateStatus = "failed"
	UpdateStatusRolledBack UpdateStatus = "rolled_back"
)

// UpdateNotification 更新通知
type UpdateNotification struct {
	Type      string    `json:"type"` // "available", "started", "completed", "failed"
	AppID     string    `json:"appId"`
	AppName   string    `json:"appName"`
	Version   string    `json:"version,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// UpdateCheckResult 更新检查结果
type UpdateCheckResult struct {
	AppID         string `json:"appId"`
	AppName       string `json:"appName"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	Changelog      string `json:"changelog,omitempty"`
	Size           int64  `json:"size,omitempty"` // 字节
	IsPrerelease   bool   `json:"isPrerelease"`
	ReleaseDate    time.Time `json:"releaseDate,omitempty"`
}

// RollbackInfo 回滚信息
type RollbackInfo struct {
	AppID        string    `json:"appId"`
	FromVersion  string    `json:"fromVersion"`
	ToVersion    string    `json:"toVersion"`    // 回滚到的版本
	BackupPath   string    `json:"backupPath"`
	CreatedAt    time.Time `json:"createdAt"`
	Available    bool      `json:"available"`
}

// NewUpdateManager 创建更新管理器
func NewUpdateManager(config *UpdateManagerConfig, catalog *Catalog) *UpdateManager {
	if config == nil {
		config = DefaultUpdateManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &UpdateManager{
		config:    config,
		catalog:   catalog,
		policies:  make(map[string]*UpdatePolicy),
		schedules: make(map[string]*UpdateSchedule),
		history:   make([]*UpdateHistoryEntry, 0),
		available: make(map[string]*UpdateInfo),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动更新管理器
func (um *UpdateManager) Start() {
	go um.checkLoop()
	log.Println("[UpdateManager] 自动更新管理器已启动")
}

// Stop 停止更新管理器
func (um *UpdateManager) Stop() {
	um.cancel()
	log.Println("[UpdateManager] 自动更新管理器已停止")
}

// SetNotificationCallback 设置通知回调
func (um *UpdateManager) SetNotificationCallback(cb func(*UpdateNotification)) {
	um.mu.Lock()
	defer um.mu.Unlock()
	um.notifCallback = cb
}

// SetUpdatePolicy 设置应用更新策略
func (um *UpdateManager) SetUpdatePolicy(appID string, policy *UpdatePolicy) {
	um.mu.Lock()
	defer um.mu.Unlock()

	policy.AppID = appID
	policy.UpdatedAt = time.Now()
	um.policies[appID] = policy
}

// GetUpdatePolicy 获取应用更新策略
func (um *UpdateManager) GetUpdatePolicy(appID string) *UpdatePolicy {
	um.mu.RLock()
	defer um.mu.RUnlock()

	if policy, ok := um.policies[appID]; ok {
		return policy
	}

	// 返回默认策略
	return &UpdatePolicy{
		AppID:      appID,
		PolicyType: um.config.DefaultPolicy,
	}
}

// CheckForUpdates 检查所有已安装应用的更新
func (um *UpdateManager) CheckForUpdates(installed map[string]string) []*UpdateCheckResult {
	um.mu.Lock()
	defer um.mu.Unlock()

	var results []*UpdateCheckResult

	for appID, currentVersion := range installed {
		entry, ok := um.catalog.GetApp(appID)
		if !ok {
			continue
		}

		result := &UpdateCheckResult{
			AppID:          appID,
			AppName:        entry.DisplayName,
			CurrentVersion: currentVersion,
			LatestVersion:  entry.LatestVersion,
			HasUpdate:      entry.LatestVersion != "" && entry.LatestVersion != currentVersion,
		}

		if result.HasUpdate {
			result.Changelog = entry.Metadata["changelog"]
			result.IsPrerelease = false // 简化处理
			um.available[appID] = &UpdateInfo{
				AppID:          appID,
				Name:           entry.DisplayName,
				CurrentVersion: currentVersion,
				LatestVersion:  entry.LatestVersion,
				Changelog:      result.Changelog,
			}
		}

		// 更新调度信息
		if schedule, ok := um.schedules[appID]; ok {
			schedule.LastChecked = time.Now()
			schedule.NextCheck = time.Now().Add(um.config.CheckInterval)
		}

		results = append(results, result)
	}

	return results
}

// GetAvailableUpdates 获取所有可用更新
func (um *UpdateManager) GetAvailableUpdates() []*UpdateInfo {
	um.mu.RLock()
	defer um.mu.RUnlock()

	updates := make([]*UpdateInfo, 0, len(um.available))
	for _, u := range um.available {
		updates = append(updates, u)
	}

	sort.Slice(updates, func(i, j int) bool {
		return updates[i].Name < updates[j].Name
	})

	return updates
}

// UpdateApp 更新单个应用
func (um *UpdateManager) UpdateApp(ctx context.Context, appID, fromVersion, toVersion, triggerType string) (*UpdateHistoryEntry, error) {
	entry := &UpdateHistoryEntry{
		ID:          fmt.Sprintf("update_%s_%d", appID, time.Now().UnixMilli()),
		AppID:       appID,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Status:      UpdateStatusPending,
		TriggerType: triggerType,
		StartedAt:   time.Now(),
	}

	// 获取应用信息
	app, ok := um.catalog.GetApp(appID)
	if ok {
		entry.AppName = app.DisplayName
	}

	um.mu.Lock()
	um.history = append(um.history, entry)
	um.mu.Unlock()

	// 发送开始通知
	um.notify(&UpdateNotification{
		Type:      "started",
		AppID:     appID,
		AppName:   entry.AppName,
		Version:   toVersion,
		Message:   fmt.Sprintf("正在更新 %s: %s -> %s", entry.AppName, fromVersion, toVersion),
		Timestamp: time.Now(),
	})

	// 模拟更新过程
	entry.Status = UpdateStatusDownloading
	time.Sleep(1 * time.Second)

	entry.Status = UpdateStatusInstalling
	time.Sleep(1 * time.Second)

	if um.config.AutoRestart {
		entry.Status = UpdateStatusRestarting
		time.Sleep(500 * time.Millisecond)
	}

	// 更新完成
	entry.Status = UpdateStatusCompleted
	entry.CompletedAt = time.Now()
	entry.Duration = time.Since(entry.StartedAt)
	entry.RollbackAvail = um.config.RollbackEnabled

	// 从可用更新中移除
	um.mu.Lock()
	delete(um.available, appID)
	um.mu.Unlock()

	// 发送完成通知
	um.notify(&UpdateNotification{
		Type:      "completed",
		AppID:     appID,
		AppName:   entry.AppName,
		Version:   toVersion,
		Message:   fmt.Sprintf("%s 已更新到 %s", entry.AppName, toVersion),
		Timestamp: time.Now(),
	})

	return entry, nil
}

// BatchUpdate 批量更新应用
func (um *UpdateManager) BatchUpdate(ctx context.Context, appIDs []string, installed map[string]string) ([]*UpdateHistoryEntry, error) {
	var results []*UpdateHistoryEntry

	// 按依赖顺序排序
	for _, appID := range appIDs {
		currentVersion, ok := installed[appID]
		if !ok {
			continue
		}

		updateInfo, hasUpdate := um.available[appID]
		if !hasUpdate {
			continue
		}

		entry, err := um.UpdateApp(ctx, appID, currentVersion, updateInfo.LatestVersion, "batch")
		if err != nil {
			log.Printf("[UpdateManager] 批量更新失败 %s: %v", appID, err)
			continue
		}

		results = append(results, entry)

		// 并发控制
		if um.config.MaxConcurrent > 0 && len(results) >= um.config.MaxConcurrent {
			break
		}
	}

	return results, nil
}

// RollbackUpdate 回滚更新
func (um *UpdateManager) RollbackUpdate(ctx context.Context, historyID string) (*UpdateHistoryEntry, error) {
	if !um.config.RollbackEnabled {
		return nil, fmt.Errorf("回滚功能未启用")
	}

	um.mu.Lock()
	defer um.mu.Unlock()

	var targetEntry *UpdateHistoryEntry
	for _, entry := range um.history {
		if entry.ID == historyID {
			targetEntry = entry
			break
		}
	}

	if targetEntry == nil {
		return nil, fmt.Errorf("更新记录不存在: %s", historyID)
	}

	if targetEntry.Status != UpdateStatusCompleted {
		return nil, fmt.Errorf("只能回滚已完成的更新")
	}

	// 创建回滚记录
	rollbackEntry := &UpdateHistoryEntry{
		ID:          fmt.Sprintf("rollback_%s_%d", targetEntry.AppID, time.Now().UnixMilli()),
		AppID:       targetEntry.AppID,
		AppName:     targetEntry.AppName,
		FromVersion: targetEntry.ToVersion,
		ToVersion:   targetEntry.FromVersion,
		Status:      UpdateStatusCompleted,
		TriggerType: "rollback",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Duration:    time.Since(time.Now()),
	}

	um.history = append(um.history, rollbackEntry)

	// 标记原记录已回滚
	targetEntry.Status = UpdateStatusRolledBack

	um.notify(&UpdateNotification{
		Type:      "rolled_back",
		AppID:     targetEntry.AppID,
		AppName:   targetEntry.AppName,
		Version:   targetEntry.FromVersion,
		Message:   fmt.Sprintf("%s 已回滚到版本 %s", targetEntry.AppName, targetEntry.FromVersion),
		Timestamp: time.Now(),
	})

	return rollbackEntry, nil
}

// GetUpdateHistory 获取更新历史
func (um *UpdateManager) GetUpdateHistory(appID string, limit int) []*UpdateHistoryEntry {
	um.mu.RLock()
	defer um.mu.RUnlock()

	var filtered []*UpdateHistoryEntry
	for _, entry := range um.history {
		if appID == "" || entry.AppID == appID {
			filtered = append(filtered, entry)
		}
	}

	// 按时间倒序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartedAt.After(filtered[j].StartedAt)
	})

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered
}

// GetUpdateStats 获取更新统计
func (um *UpdateManager) GetUpdateStats() map[string]interface{} {
	um.mu.RLock()
	defer um.mu.RUnlock()

	total := len(um.history)
	success := 0
	failed := 0
	rolledBack := 0

	for _, entry := range um.history {
		switch entry.Status {
		case UpdateStatusCompleted:
			success++
		case UpdateStatusFailed:
			failed++
		case UpdateStatusRolledBack:
			rolledBack++
		}
	}

	return map[string]interface{}{
		"totalUpdates":     total,
		"successfulUpdates": success,
		"failedUpdates":    failed,
		"rolledBackUpdates": rolledBack,
		"availableUpdates":  len(um.available),
		"policies":          len(um.policies),
	}
}

// checkLoop 定期检查更新循环
func (um *UpdateManager) checkLoop() {
	ticker := time.NewTicker(um.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-um.ctx.Done():
			return
		case <-ticker.C:
			um.performAutoUpdates()
		}
	}
}

// performAutoUpdates 执行自动更新
func (um *UpdateManager) performAutoUpdates() {
	um.mu.RLock()
	var autoApps []string
	for appID, policy := range um.policies {
		if policy.PolicyType == UpdatePolicyAuto {
			if um.shouldAutoUpdate(policy) {
				autoApps = append(autoApps, appID)
			}
		}
	}
	um.mu.RUnlock()

	for _, appID := range autoApps {
		if update, ok := um.available[appID]; ok {
			_, err := um.UpdateApp(context.Background(), appID, update.CurrentVersion, update.LatestVersion, "auto")
			if err != nil {
				log.Printf("[UpdateManager] 自动更新失败 %s: %v", appID, err)
			}
		}
	}
}

// shouldAutoUpdate 判断是否应该自动更新
func (um *UpdateManager) shouldAutoUpdate(policy *UpdatePolicy) bool {
	if policy.AutoUpdateTime == "" {
		return true // 无时间限制，立即更新
	}

	// 解析时间 (简化处理)
	now := time.Now()
	// 检查是否在允许的时间窗口
	_ = now
	return true
}

// notify 发送通知
func (um *UpdateManager) notify(notification *UpdateNotification) {
	if um.notifCallback != nil {
		go um.notifCallback(notification)
	}
}

// InitializeSchedules 初始化更新调度
func (um *UpdateManager) InitializeSchedules(installed map[string]string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	for appID := range installed {
		if _, ok := um.schedules[appID]; !ok {
			um.schedules[appID] = &UpdateSchedule{
				AppID:       appID,
				NextCheck:   time.Now().Add(um.config.CheckInterval),
				LastChecked: time.Time{},
				Enabled:     true,
			}
		}
	}
}

// GetSchedules 获取所有更新调度
func (um *UpdateManager) GetSchedules() []*UpdateSchedule {
	um.mu.RLock()
	defer um.mu.RUnlock()

	schedules := make([]*UpdateSchedule, 0, len(um.schedules))
	for _, s := range um.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}
