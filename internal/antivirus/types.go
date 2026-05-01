// Package antivirus 提供病毒扫描功能
// 对标群晖 Antivirus Essential，集成 ClamAV 引擎
package antivirus

import (
	"errors"
	"sync"
	"time"
)

// ========== 常量定义 ==========

// ScanType 扫描类型
type ScanType string

const (
	ScanTypeFull     ScanType = "full"     // 全盘扫描
	ScanTypeQuick    ScanType = "quick"    // 快速扫描
	ScanTypeCustom   ScanType = "custom"   // 自定义路径扫描
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	ScanStatusPending  ScanStatus = "pending"  // 等待中
	ScanStatusRunning  ScanStatus = "running"  // 扫描中
	ScanStatusPaused   ScanStatus = "paused"   // 已暂停
	ScanStatusDone     ScanStatus = "done"     // 完成
	ScanStatusFailed   ScanStatus = "failed"   // 失败
	ScanStatusCanceled ScanStatus = "canceled" // 已取消
)

// ThreatAction 对感染文件的处理方式
type ThreatAction string

const (
	ThreatActionQuarantine ThreatAction = "quarantine" // 隔离
	ThreatActionDelete     ThreatAction = "delete"     // 删除
	ThreatActionIgnore     ThreatAction = "ignore"     // 忽略
)

// ClamAV 连接方式
type ClamAVTransport string

const (
	TransportSocket ClamAVTransport = "socket" // Unix socket
	TransportTCP    ClamAVTransport = "tcp"    // TCP 连接
)

// ========== 错误定义 ==========

var (
	ErrScanNotFound     = errors.New("scan task not found")
	ErrScanRunning      = errors.New("scan task is already running")
	ErrClamAVNotReady   = errors.New("clamd is not reachable")
	ErrInvalidPath      = errors.New("invalid scan path")
	ErrQuarantineFailed = errors.New("quarantine file failed")
	ErrNotInfected      = errors.New("file is not infected")
)

// ========== ClamAV 配置 ==========

// ClamAVConfig ClamAV 连接配置
type ClamAVConfig struct {
	Transport ClamAVTransport `json:"transport"`           // socket 或 tcp
	Socket    string          `json:"socket,omitempty"`    // Unix socket 路径
	Host      string          `json:"host,omitempty"`      // TCP 主机
	Port      int             `json:"port,omitempty"`      // TCP 端口
	Timeout   time.Duration   `json:"timeout"`             // 超时时间
	MaxStream int             `json:"max_stream_size"`     // 最大流大小（字节）
}

// DefaultClamAVConfig 默认 ClamAV 配置
func DefaultClamAVConfig() ClamAVConfig {
	return ClamAVConfig{
		Transport: TransportSocket,
		Socket:    "/var/run/clamav/clamd.ctl",
		Host:      "127.0.0.1",
		Port:      3310,
		Timeout:   120 * time.Second,
		MaxStream: 25 * 1024 * 1024, // 25MB
	}
}

// ClamAVVersion ClamAV 版本信息
type ClamAVVersion struct {
	Version    string    `json:"version"`     // clamd 版本
	DBVersion  string    `json:"db_version"`  // 病毒库版本
	DBDate     time.Time `json:"db_date"`     // 病毒库日期
	Signature  int       `json:"signature_count"` // 签名数量
	Engine     string    `json:"engine"`      // 引擎版本
}

// VirusDBUpdateStatus 病毒库更新状态
type VirusDBUpdateStatus struct {
	Status    string    `json:"status"`     // idle, updating, success, failed
	LastCheck time.Time `json:"last_check"` // 上次检查时间
	LastUpdate time.Time `json:"last_update"` // 上次更新时间
	DBVersion string    `json:"db_version"` // 当前版本
	Error     string    `json:"error,omitempty"`
}

// ========== 扫描任务 ==========

// ScanTask 扫描任务
type ScanTask struct {
	mu sync.RWMutex `json:"-"`

	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        ScanType  `json:"type"`
	Status      ScanStatus `json:"status"`
	Paths       []string  `json:"paths"`         // 扫描路径列表
	Recursive   bool      `json:"recursive"`     // 是否递归扫描
	ScanArchives bool     `json:"scan_archives"` // 是否扫描压缩文件
	ThreatAction ThreatAction `json:"threat_action"` // 发现感染时的默认处理

	// 进度
	TotalFiles   int64  `json:"total_files"`   // 总文件数
	ScannedFiles int64  `json:"scanned_files"` // 已扫描文件数
	InfectedFiles int64 `json:"infected_files"` // 感染文件数
	CurrentPath  string `json:"current_path"`  // 当前扫描路径

	// 时间
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Duration    int64      `json:"duration_sec"` // 耗时（秒）

	// 结果
	Results []ScanResult `json:"results,omitempty"`

	// 错误
	Error string `json:"error,omitempty"`
}

// ScanResult 单个文件扫描结果
type ScanResult struct {
	FilePath     string       `json:"file_path"`
	FileSize     int64        `json:"file_size"`
	ThreatName   string       `json:"threat_name"`   // 病毒名称
	IsInfected   bool         `json:"is_infected"`
	Action       ThreatAction `json:"action"`         // 采取的动作
	Quarantined  bool         `json:"quarantined"`    // 是否已隔离
	QuarantinePath string     `json:"quarantine_path,omitempty"`
	ScannedAt    time.Time    `json:"scanned_at"`
}

// SetProgress 更新进度（线程安全）
func (t *ScanTask) SetProgress(scanned, total int64, currentPath string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ScannedFiles = scanned
	t.TotalFiles = total
	t.CurrentPath = currentPath
}

// SetStatus 设置状态（线程安全）
func (t *ScanTask) SetStatus(status ScanStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
}

// AddResult 添加扫描结果（线程安全）
func (t *ScanTask) AddResult(r ScanResult) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Results = append(t.Results, r)
	if r.IsInfected {
		t.InfectedFiles++
	}
}

// GetProgress 获取当前进度快照（线程安全）
func (t *ScanTask) GetProgress() (scanned, total, infected int64, current string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ScannedFiles, t.TotalFiles, t.InfectedFiles, t.CurrentPath
}

// ========== 扫描策略（定时任务） ==========

// ScanSchedule 定时扫描策略
type ScanSchedule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Enabled     bool        `json:"enabled"`
	CronExpr    string      `json:"cron_expr"`    // cron 表达式
	ScanType    ScanType    `json:"scan_type"`
	Paths       []string    `json:"paths"`        // 自定义路径（custom 时使用）
	ThreatAction ThreatAction `json:"threat_action"`
	FileFilter  FileFilter  `json:"file_filter"`  // 文件类型过滤
	NotifyOnComplete bool   `json:"notify_on_complete"` // 完成后通知
	LastRun     *time.Time  `json:"last_run,omitempty"`
	NextRun     *time.Time  `json:"next_run,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// FileFilter 文件过滤规则
type FileFilter struct {
	IncludeExtensions []string `json:"include_extensions,omitempty"` // 仅扫描这些扩展名，为空则全部
	ExcludeExtensions []string `json:"exclude_extensions,omitempty"` // 排除的扩展名
	ExcludePaths      []string `json:"exclude_paths,omitempty"`     // 排除的路径
	MaxFileSize       int64    `json:"max_file_size,omitempty"`     // 最大文件大小（字节），0 表示不限
}

// ========== 隔离区 ==========

// QuarantineEntry 隔离区条目
type QuarantineEntry struct {
	ID             string       `json:"id"`
	OriginalPath   string       `json:"original_path"`   // 原始路径
	QuarantinePath string       `json:"quarantine_path"` // 隔离路径
	ThreatName     string       `json:"threat_name"`
	FileSize       int64        `json:"file_size"`
	FileHash       string       `json:"file_hash"`      // SHA256
	QuarantinedAt  time.Time    `json:"quarantined_at"`
	ScanTaskID     string       `json:"scan_task_id"`   // 关联的扫描任务
	Restored       bool         `json:"restored"`       // 是否已恢复
	RestoredAt     *time.Time   `json:"restored_at,omitempty"`
	Deleted        bool         `json:"deleted"`        // 是否已删除
	DeletedAt      *time.Time   `json:"deleted_at,omitempty"`
}

// ========== 白名单 ==========

// WhitelistEntry 白名单条目
type WhitelistEntry struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`      // 路径（支持通配符）
	Hash      string    `json:"hash"`      // 文件哈希（精确匹配时使用）
	Reason    string    `json:"reason"`    // 添加原因
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`
}

// ========== 实时监控 ==========

// RealtimeMonitorConfig 实时监控配置
type RealtimeMonitorConfig struct {
	Enabled       bool     `json:"enabled"`
	WatchPaths    []string `json:"watch_paths"`    // 监控路径
	ThreatAction  ThreatAction `json:"threat_action"` // 发现威胁时的动作
	Recursive     bool     `json:"recursive"`
	ExcludePaths  []string `json:"exclude_paths"`  // 排除路径
}

// ========== 扫描报告 ==========

// ScanReport 扫描报告
type ScanReport struct {
	TaskID        string        `json:"task_id"`
	TaskName      string        `json:"task_name"`
	ScanType      ScanType      `json:"scan_type"`
	Status        ScanStatus    `json:"status"`
	StartedAt     *time.Time    `json:"started_at"`
	FinishedAt    *time.Time    `json:"finished_at"`
	Duration      int64         `json:"duration_sec"`
	TotalFiles    int64         `json:"total_files"`
	ScannedFiles  int64         `json:"scanned_files"`
	InfectedFiles int64         `json:"infected_files"`
	SkippedFiles  int64         `json:"skipped_files"`
	ScanSpeed     float64       `json:"scan_speed"`    // 文件/秒
	InfectedList  []ScanResult  `json:"infected_list"` // 感染文件详情
	ThreatSummary map[string]int `json:"threat_summary"` // 威胁类型统计
}

// ScanStats 总体扫描统计
type ScanStats struct {
	TotalScans      int   `json:"total_scans"`
	TotalFiles      int64 `json:"total_files_scanned"`
	TotalInfected   int64 `json:"total_infected_files"`
	TotalQuarantine int   `json:"total_quarantined"`
	LastScanTime    *time.Time `json:"last_scan_time,omitempty"`
	DBVersion       string `json:"db_version"`
	DBDate          time.Time `json:"db_date"`
}

// ========== API 请求体 ==========

// CreateScanRequest 创建扫描请求
type CreateScanRequest struct {
	Name         string       `json:"name"`
	Type         ScanType     `json:"type" binding:"required"`
	Paths        []string     `json:"paths"`
	Recursive    bool         `json:"recursive"`
	ScanArchives bool         `json:"scan_archives"`
	ThreatAction ThreatAction `json:"threat_action"`
}

// UpdateScheduleRequest 更新定时扫描请求
type UpdateScheduleRequest struct {
	Name          *string        `json:"name,omitempty"`
	Enabled       *bool          `json:"enabled,omitempty"`
	CronExpr      *string        `json:"cron_expr,omitempty"`
	ScanType      *ScanType      `json:"scan_type,omitempty"`
	Paths         []string       `json:"paths,omitempty"`
	ThreatAction  *ThreatAction  `json:"threat_action,omitempty"`
	FileFilter    *FileFilter    `json:"file_filter,omitempty"`
	NotifyOnComplete *bool       `json:"notify_on_complete,omitempty"`
}

// WhitelistAddRequest 添加白名单请求
type WhitelistAddRequest struct {
	Path   string `json:"path" binding:"required"`
	Hash   string `json:"hash"`
	Reason string `json:"reason"`
}

// UpdateMonitorConfigRequest 更新实时监控配置请求
type UpdateMonitorConfigRequest struct {
	Enabled      *bool         `json:"enabled,omitempty"`
	WatchPaths   []string      `json:"watch_paths,omitempty"`
	ThreatAction *ThreatAction `json:"threat_action,omitempty"`
	Recursive    *bool         `json:"recursive,omitempty"`
	ExcludePaths []string      `json:"exclude_paths,omitempty"`
}
