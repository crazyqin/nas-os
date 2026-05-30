// Package datamigration - 数据迁移向导
// 引导式数据迁移，支持多种源和目标
package datamigration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Migration 迁移任务
type Migration struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Status      string            `json:"status"` // pending, running, paused, completed, failed, cancelled
	SourceType  string            `json:"source_type"`
	Source      Source            `json:"source"`
	TargetType  string            `json:"target_type"`
	Target      Target            `json:"target"`
	Options     MigrationOptions  `json:"options"`
	Progress    Progress          `json:"progress"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Source 迁移源
type Source struct {
	Type     string            `json:"type"` // local, nfs, smb, s3, rsync, synology, truenas
	Host     string            `json:"host,omitempty"`
	Port     int               `json:"port,omitempty"`
	Path     string            `json:"path"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

// Target 迁移目标
type Target struct {
	Type     string            `json:"type"` // local, nfs, smb, s3
	Host     string            `json:"host,omitempty"`
	Port     int               `json:"port,omitempty"`
	Path     string            `json:"path"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
}

// MigrationOptions 迁移选项
type MigrationOptions struct {
	Bandwidth     int  `json:"bandwidth"` // MB/s, 0=无限制
	Parallel      int  `json:"parallel"` // 并发数
	Verify        bool `json:"verify"` // 校验
	Sync          bool `json:"sync"` // 同步模式
	DeleteOrphan  bool `json:"delete_orphan"` // 删除目标多余文件
	Compress      bool `json:"compress"` // 压缩传输
	Incremental   bool `json:"incremental"` // 增量迁移
	Exclude       []string `json:"exclude,omitempty"` // 排除模式
	Include       []string `json:"include,omitempty"` // 包含模式
	RetryCount    int  `json:"retry_count"`
	RetryDelay    int  `json:"retry_delay"` // 秒
}

// Progress 进度
type Progress struct {
	TotalFiles     int64   `json:"total_files"`
	CompletedFiles int64   `json:"completed_files"`
	FailedFiles    int64   `json:"failed_files"`
	SkippedFiles   int64   `json:"skipped_files"`
	TotalBytes     int64   `json:"total_bytes"`
	CompletedBytes int64   `json:"completed_bytes"`
	Speed          float64 `json:"speed"` // bytes/sec
	ETA            int     `json:"eta"` // 秒
	Percent        float64 `json:"percent"`
	CurrentFile    string  `json:"current_file,omitempty"`
}

// Plan 迁移计划
type Plan struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	SourceType  string        `json:"source_type"`
	TargetType  string        `json:"target_type"`
	Steps       []Step        `json:"steps"`
	Estimate    Estimate      `json:"estimate"`
	CreatedAt   time.Time     `json:"created_at"`
}

// Step 迁移步骤
type Step struct {
	Order       int    `json:"order"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"` // pending, running, completed, failed
	Action      string `json:"action"`
}

// Estimate 估算
type Estimate struct {
	TotalFiles int64 `json:"total_files"`
	TotalBytes int64 `json:"total_bytes"`
	Duration   int   `json:"duration"` // 秒
}

// CreateMigrationRequest 创建迁移请求
type CreateMigrationRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	SourceType  string           `json:"source_type"`
	Source      Source           `json:"source"`
	TargetType  string           `json:"target_type"`
	Target      Target           `json:"target"`
	Options     MigrationOptions `json:"options"`
}

// CreatePlanRequest 创建计划请求
type CreatePlanRequest struct {
	Name       string `json:"name"`
	SourceType string `json:"source_type"`
	TargetType string `json:"target_type"`
	Source     Source `json:"source"`
	Target     Target `json:"target"`
}

// Manager 管理器
type Manager struct {
	mu         sync.RWMutex
	migrations map[string]*Migration
	plans      map[string]*Plan
	config     *Config
	dataFile   string
}

// Config 配置
type Config struct {
	MaxMigrations   int    `json:"max_migrations"`
	MaxBandwidth    int    `json:"max_bandwidth"` // MB/s
	DefaultParallel int    `json:"default_parallel"`
	RetentionDays   int    `json:"retention_days"`
	AllowRemote     bool   `json:"allow_remote"`
}

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		migrations: make(map[string]*Migration),
		plans:      make(map[string]*Plan),
		config: &Config{
			MaxMigrations:   10,
			MaxBandwidth:    100,
			DefaultParallel: 4,
			RetentionDays:   30,
			AllowRemote:     true,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	m.loadDefaultPlans()
	return m.load()
}

func (m *Manager) loadDefaultPlans() {
	m.plans["nas-to-local"] = &Plan{
		ID:          "nas-to-local",
		Name:        "NAS迁移到本地",
		Description: "从旧NAS迁移到本地存储",
		SourceType:  "nfs",
		TargetType:  "local",
		Steps: []Step{
			{Order: 1, Name: "扫描源目录", Action: "scan"},
			{Order: 2, Name: "估算数据量", Action: "estimate"},
			{Order: 3, Name: "创建目标目录", Action: "prepare"},
			{Order: 4, Name: "传输文件", Action: "transfer"},
			{Order: 5, Name: "校验数据", Action: "verify"},
			{Order: 6, Name: "清理临时文件", Action: "cleanup"},
		},
	}

	m.plans["synology-migration"] = &Plan{
		ID:          "synology-migration",
		Name:        "群晖数据迁移",
		Description: "从群晖NAS迁移数据",
		SourceType:  "synology",
		TargetType:  "local",
		Steps: []Step{
			{Order: 1, Name: "连接群晖NAS", Action: "connect"},
			{Order: 2, Name: "扫描共享文件夹", Action: "scan"},
			{Order: 3, Name: "选择迁移内容", Action: "select"},
			{Order: 4, Name: "传输文件", Action: "transfer"},
			{Order: 5, Name: "校验数据", Action: "verify"},
		},
	}

	m.plans["cloud-migration"] = &Plan{
		ID:          "cloud-migration",
		Name:        "云存储迁移",
		Description: "从云存储迁移到本地",
		SourceType:  "s3",
		TargetType:  "local",
		Steps: []Step{
			{Order: 1, Name: "配置云存储凭证", Action: "configure"},
			{Order: 2, Name: "扫描桶/容器", Action: "scan"},
			{Order: 3, Name: "下载文件", Action: "transfer"},
			{Order: 4, Name: "校验数据", Action: "verify"},
		},
	}
}

// CreateMigration 创建迁移任务
func (m *Manager) CreateMigration(req CreateMigrationRequest) (*Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.migrations) >= m.config.MaxMigrations {
		return nil, fmt.Errorf("已达到最大迁移任务数限制 (%d)", m.config.MaxMigrations)
	}

	if !m.config.AllowRemote && (req.SourceType != "local" || req.TargetType != "local") {
		return nil, fmt.Errorf("管理员已禁用远程迁移")
	}

	id := fmt.Sprintf("migration_%d", time.Now().UnixNano())
	migration := &Migration{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      "pending",
		SourceType:  req.SourceType,
		Source:      req.Source,
		TargetType:  req.TargetType,
		Target:      req.Target,
		Options:     req.Options,
		Progress:    Progress{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if migration.Options.Parallel <= 0 {
		migration.Options.Parallel = m.config.DefaultParallel
	}

	m.migrations[id] = migration
	return migration, m.save()
}

// StartMigration 开始迁移
func (m *Manager) StartMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("迁移任务 '%s' 不存在", id)
	}

	if migration.Status != "pending" {
		return fmt.Errorf("迁移任务不在待处理状态")
	}

	now := time.Now()
	migration.Status = "running"
	migration.StartedAt = &now
	migration.UpdatedAt = now

	return m.save()
}

// PauseMigration 暂停迁移
func (m *Manager) PauseMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("迁移任务 '%s' 不存在", id)
	}

	if migration.Status != "running" {
		return fmt.Errorf("迁移任务不在运行状态")
	}

	migration.Status = "paused"
	migration.UpdatedAt = time.Now()
	return m.save()
}

// ResumeMigration 恢复迁移
func (m *Manager) ResumeMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("迁移任务 '%s' 不存在", id)
	}

	if migration.Status != "paused" {
		return fmt.Errorf("迁移任务不在暂停状态")
	}

	migration.Status = "running"
	migration.UpdatedAt = time.Now()
	return m.save()
}

// CancelMigration 取消迁移
func (m *Manager) CancelMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("迁移任务 '%s' 不存在", id)
	}

	if migration.Status == "completed" || migration.Status == "cancelled" {
		return fmt.Errorf("迁移任务已完成或已取消")
	}

	migration.Status = "cancelled"
	migration.UpdatedAt = time.Now()
	return m.save()
}

// GetMigration 获取迁移任务
func (m *Manager) GetMigration(id string) (*Migration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	migration, ok := m.migrations[id]
	if !ok {
		return nil, fmt.Errorf("迁移任务 '%s' 不存在", id)
	}
	return migration, nil
}

// ListMigrations 列出迁移任务
func (m *Manager) ListMigrations(status string) []*Migration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Migration
	for _, mig := range m.migrations {
		if status == "" || mig.Status == status {
			result = append(result, mig)
		}
	}
	return result
}

// UpdateProgress 更新进度
func (m *Manager) UpdateProgress(id string, progress Progress) {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return
	}

	migration.Progress = progress
	migration.UpdatedAt = time.Now()

	if progress.TotalBytes > 0 {
		migration.Progress.Percent = float64(progress.CompletedBytes) / float64(progress.TotalBytes) * 100
	}
	if progress.Speed > 0 && progress.TotalBytes > progress.CompletedBytes {
		remaining := progress.TotalBytes - progress.CompletedBytes
		migration.Progress.ETA = int(float64(remaining) / progress.Speed)
	}
}

// CompleteMigration 完成迁移
func (m *Manager) CompleteMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("迁移任务 '%s' 不存在", id)
	}

	now := time.Now()
	migration.Status = "completed"
	migration.CompletedAt = &now
	migration.Progress.Percent = 100
	migration.UpdatedAt = now

	return m.save()
}

// FailMigration 迁移失败
func (m *Manager) FailMigration(id, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	migration, ok := m.migrations[id]
	if !ok {
		return fmt.Errorf("迁移任务 '%s' 不存在", id)
	}

	migration.Status = "failed"
	migration.Error = errMsg
	migration.UpdatedAt = time.Now()

	return m.save()
}

// CreatePlan 创建迁移计划
func (m *Manager) CreatePlan(req CreatePlanRequest) (*Plan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("plan_%d", time.Now().UnixNano())
	plan := &Plan{
		ID:         id,
		Name:       req.Name,
		SourceType: req.SourceType,
		TargetType: req.TargetType,
		Steps: []Step{
			{Order: 1, Name: "扫描源", Action: "scan"},
			{Order: 2, Name: "估算数据量", Action: "estimate"},
			{Order: 3, Name: "执行迁移", Action: "transfer"},
			{Order: 4, Name: "校验数据", Action: "verify"},
		},
		CreatedAt: time.Now(),
	}

	m.plans[id] = plan
	return plan, m.save()
}

// GetPlan 获取迁移计划
func (m *Manager) GetPlan(id string) (*Plan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("迁移计划 '%s' 不存在", id)
	}
	return plan, nil
}

// ListPlans 列出迁移计划
func (m *Manager) ListPlans() []*Plan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Plan
	for _, p := range m.plans {
		result = append(result, p)
	}
	return result
}

func (m *Manager) load() error {
	return nil
}

func (m *Manager) save() error {
	return nil
}

// RegisterHandlers 注册HTTP处理器
func (m *Manager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/migration/tasks", m.handleMigrations)
	mux.HandleFunc("/api/v1/migration/tasks/", m.handleMigrationByID)
	mux.HandleFunc("/api/v1/migration/plans", m.handlePlans)
	mux.HandleFunc("/api/v1/migration/plans/", m.handlePlanByID)
}

func (m *Manager) handleMigrations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := r.URL.Query().Get("status")
		migrations := m.ListMigrations(status)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(migrations)
	case http.MethodPost:
		var req CreateMigrationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		migration, err := m.CreateMigration(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(migration)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleMigrationByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/migration/tasks/"):]
	if id == "" {
		http.Error(w, "Missing migration ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		migration, err := m.GetMigration(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(migration)
	case http.MethodPut:
		action := r.URL.Query().Get("action")
		var err error
		switch action {
		case "start":
			err = m.StartMigration(id)
		case "pause":
			err = m.PauseMigration(id)
		case "resume":
			err = m.ResumeMigration(id)
		case "cancel":
			err = m.CancelMigration(id)
		case "complete":
			err = m.CompleteMigration(id)
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handlePlans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		plans := m.ListPlans()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(plans)
	case http.MethodPost:
		var req CreatePlanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		plan, err := m.CreatePlan(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(plan)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handlePlanByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/migration/plans/"):]
	if id == "" {
		http.Error(w, "Missing plan ID", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	plan, err := m.GetPlan(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}
