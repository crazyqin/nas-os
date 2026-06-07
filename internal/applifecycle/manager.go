// Package applifecycle 应用生命周期管理模块
// 管理Docker应用的完整生命周期：安装、配置、备份、升级、回滚、卸载
package applifecycle

import (
	"fmt"
	"sync"
	"time"
)

// AppState 应用状态
type AppState string

const (
	StateInstalling AppState = "installing"
	StateRunning    AppState = "running"
	StateStopped    AppState = "stopped"
	StateUpgrading  AppState = "upgrading"
	StateBacking    AppState = "backing_up"
	StateRolling    AppState = "rolling_back"
	StateError      AppState = "error"
	StateRemoved    AppState = "removed"
)

// App 应用信息
type App struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Version       string            `json:"version"`
	State         AppState          `json:"state"`
	Ports         map[string]string `json:"ports"`
	Volumes       []string          `json:"volumes"`
	Env           map[string]string `json:"env"`
	Labels        map[string]string `json:"labels"`
	HealthCheck   string            `json:"health_check"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	ErrorMsg      string            `json:"error_msg,omitempty"`
	RestartCount  int               `json:"restart_count"`
	Backups       []BackupRecord    `json:"backups"`
	ConfigHistory []ConfigSnapshot  `json:"config_history"`
}

// BackupRecord 备份记录
type BackupRecord struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	Version   string    `json:"version"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// ConfigSnapshot 配置快照
type ConfigSnapshot struct {
	ID        string            `json:"id"`
	AppID     string            `json:"app_id"`
	Version   int               `json:"version"`
	Env       map[string]string `json:"env"`
	Volumes   []string          `json:"volumes"`
	Ports     map[string]string `json:"ports"`
	CreatedAt time.Time         `json:"created_at"`
	Comment   string            `json:"comment"`
}

// Manager 应用生命周期管理器
type Manager struct {
	mu      sync.RWMutex
	apps    map[string]*App
	storage StorageBackend
}

// StorageBackend 存储后端接口
type StorageBackend interface {
	Save(app *App) error
	Load(id string) (*App, error)
	Delete(id string) error
	List() ([]*App, error)
	SaveBackup(record BackupRecord) error
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		apps: make(map[string]*App),
	}
}

// Install 安装应用
func (m *Manager) Install(name, image, version string, opts InstallOptions) (*App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	for _, app := range m.apps {
		if app.Name == name && app.State != StateRemoved {
			return nil, fmt.Errorf("应用 %s 已存在", name)
		}
	}

	app := &App{
		ID:        generateID(),
		Name:      name,
		Image:     image,
		Version:   version,
		State:     StateInstalling,
		Ports:     opts.Ports,
		Volumes:   opts.Volumes,
		Env:       opts.Env,
		Labels:    opts.Labels,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.apps[app.ID] = app

	// 模拟安装过程
	go m.performInstall(app)

	return app, nil
}

// InstallOptions 安装选项
type InstallOptions struct {
	Ports   map[string]string `json:"ports"`
	Volumes []string          `json:"volumes"`
	Env     map[string]string `json:"env"`
	Labels  map[string]string `json:"labels"`
}

// performInstall 执行安装
func (m *Manager) performInstall(app *App) {
	time.Sleep(2 * time.Second) // 模拟安装时间

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	app.State = StateRunning
	app.StartedAt = &now
	app.UpdatedAt = now

	// 保存初始配置快照
	snapshot := ConfigSnapshot{
		ID:        generateID(),
		AppID:     app.ID,
		Version:   1,
		Env:       copyMap(app.Env),
		Volumes:   copySlice(app.Volumes),
		Ports:     copyMap(app.Ports),
		CreatedAt: now,
		Comment:   "初始安装配置",
	}
	app.ConfigHistory = append(app.ConfigHistory, snapshot)
}

// Stop 停止应用
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("应用 %s 不存在", id)
	}

	if app.State != StateRunning {
		return fmt.Errorf("应用 %s 当前状态 %s 无法停止", id, app.State)
	}

	app.State = StateStopped
	app.UpdatedAt = time.Now()
	return nil
}

// Start 启动应用
func (m *Manager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("应用 %s 不存在", id)
	}

	if app.State != StateStopped && app.State != StateError {
		return fmt.Errorf("应用 %s 当前状态 %s 无法启动", id, app.State)
	}

	now := time.Now()
	app.State = StateRunning
	app.StartedAt = &now
	app.UpdatedAt = now
	app.ErrorMsg = ""
	return nil
}

// Upgrade 升级应用
func (m *Manager) Upgrade(id, newVersion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("应用 %s 不存在", id)
	}

	if app.State != StateRunning && app.State != StateStopped {
		return fmt.Errorf("应用 %s 当前状态 %s 无法升级", id, app.State)
	}

	// 自动备份当前版本
	backup := BackupRecord{
		ID:        generateID(),
		AppID:     app.ID,
		Version:   app.Version,
		CreatedAt: time.Now(),
	}
	app.Backups = append(app.Backups, backup)

	oldVersion := app.Version
	app.State = StateUpgrading
	app.UpdatedAt = time.Now()

	// 模拟升级
	go m.performUpgrade(app, oldVersion, newVersion)

	return nil
}

// performUpgrade 执行升级
func (m *Manager) performUpgrade(app *App, oldVersion, newVersion string) {
	time.Sleep(3 * time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	app.Version = newVersion
	app.State = StateRunning
	app.UpdatedAt = now

	// 保存升级后的配置快照
	snapshot := ConfigSnapshot{
		ID:        generateID(),
		AppID:     app.ID,
		Version:   len(app.ConfigHistory) + 1,
		Env:       copyMap(app.Env),
		Volumes:   copySlice(app.Volumes),
		Ports:     copyMap(app.Ports),
		CreatedAt: now,
		Comment:   fmt.Sprintf("从 %s 升级到 %s", oldVersion, newVersion),
	}
	app.ConfigHistory = append(app.ConfigHistory, snapshot)
}

// Rollback 回滚到指定版本
func (m *Manager) Rollback(id, targetVersion string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("应用 %s 不存在", id)
	}

	// 查找目标版本的备份
	var targetBackup *BackupRecord
	for i := len(app.Backups) - 1; i >= 0; i-- {
		if app.Backups[i].Version == targetVersion {
			targetBackup = &app.Backups[i]
			break
		}
	}

	if targetBackup == nil {
		return fmt.Errorf("未找到版本 %s 的备份", targetVersion)
	}

	app.State = StateRolling
	app.UpdatedAt = time.Now()

	// 模拟回滚
	go m.performRollback(app, targetVersion)

	return nil
}

// performRollback 执行回滚
func (m *Manager) performRollback(app *App, targetVersion string) {
	time.Sleep(2 * time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	app.Version = targetVersion
	app.State = StateRunning
	app.UpdatedAt = now
	app.RestartCount++

	snapshot := ConfigSnapshot{
		ID:        generateID(),
		AppID:     app.ID,
		Version:   len(app.ConfigHistory) + 1,
		Env:       copyMap(app.Env),
		Volumes:   copySlice(app.Volumes),
		Ports:     copyMap(app.Ports),
		CreatedAt: now,
		Comment:   fmt.Sprintf("回滚到版本 %s", targetVersion),
	}
	app.ConfigHistory = append(app.ConfigHistory, snapshot)
}

// Backup 手动备份应用
func (m *Manager) Backup(id string) (*BackupRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", id)
	}

	app.State = StateBacking
	app.UpdatedAt = time.Now()

	backup := BackupRecord{
		ID:        generateID(),
		AppID:     app.ID,
		Version:   app.Version,
		CreatedAt: time.Now(),
	}

	app.Backups = append(app.Backups, backup)

	// 模拟备份完成
	go func() {
		time.Sleep(1 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		if app.State == StateBacking {
			app.State = StateRunning
			app.UpdatedAt = time.Now()
		}
	}()

	return &backup, nil
}

// Uninstall 卸载应用
func (m *Manager) Uninstall(id string, keepData bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("应用 %s 不存在", id)
	}

	app.State = StateRemoved
	app.UpdatedAt = time.Now()

	if !keepData {
		delete(m.apps, id)
	}

	return nil
}

// GetApp 获取应用信息
func (m *Manager) GetApp(id string) (*App, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, ok := m.apps[id]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", id)
	}

	return app, nil
}

// ListApps 列出所有应用
func (m *Manager) ListApps(stateFilter AppState) []*App {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*App
	for _, app := range m.apps {
		if stateFilter == "" || app.State == stateFilter {
			result = append(result, app)
		}
	}

	return result
}

// GetConfigHistory 获取配置历史
func (m *Manager) GetConfigHistory(id string) ([]ConfigSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, ok := m.apps[id]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", id)
	}

	return app.ConfigHistory, nil
}

// RestoreConfig 恢复到指定配置版本
func (m *Manager) RestoreConfig(id string, version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("应用 %s 不存在", id)
	}

	for _, snap := range app.ConfigHistory {
		if snap.Version == version {
			app.Env = copyMap(snap.Env)
			app.Volumes = copySlice(snap.Volumes)
			app.Ports = copyMap(snap.Ports)
			app.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("配置版本 %d 不存在", version)
}

// 辅助函数
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func copyMap(m map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = v
	}
	return result
}

func copySlice(s []string) []string {
	result := make([]string, len(s))
	copy(result, s)
	return result
}
