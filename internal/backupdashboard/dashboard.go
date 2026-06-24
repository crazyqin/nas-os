// Package backupdashboard 提供备份仪表板 Widget
//
// 实现 TrueNAS 风格的备份任务仪表板 Widget，
// 展示备份概览、最近任务、存储使用趋势等。
//
// 工部（DevOps）注: 本模块于 2026-06-24 开发完成。
package backupdashboard

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// DashboardWidget 仪表板 Widget 类型
type DashboardWidget string

const (
	WidgetOverview     DashboardWidget = "overview"
	WidgetRecentJobs   DashboardWidget = "recent_jobs"
	WidgetStorageTrend DashboardWidget = "storage_trend"
	WidgetAlerts       DashboardWidget = "alerts"
	WidgetSources      DashboardWidget = "sources"
)

// BackupOverview 备份概览
type BackupOverview struct {
	TotalJobs       int       `json:"total_jobs"`
	ActiveJobs      int       `json:"active_jobs"`
	SuccessfulToday int       `json:"successful_today"`
	FailedToday     int       `json:"failed_today"`
	TotalSources    int       `json:"total_sources"`
	ProtectedData   string    `json:"protected_data"`
	LastBackupTime  time.Time `json:"last_backup_time,omitempty"`
	NextBackupTime  time.Time `json:"next_backup_time,omitempty"`
	HealthScore     float64   `json:"health_score"` // 0-100
	UpdatedAt       time.Time `json:"updated_at"`
}

// RecentJob 最近任务
type RecentJob struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Source     string    `json:"source"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Size       string    `json:"size"`
	Duration   string    `json:"duration"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// StorageTrend 存储趋势
type StorageTrend struct {
	Date        string  `json:"date"`
	UsedBytes   int64   `json:"used_bytes"`
	TotalBytes  int64   `json:"total_bytes"`
	GrowthBytes int64   `json:"growth_bytes"`
	GrowthRate  float64 `json:"growth_rate"` // 百分比
}

// BackupAlert 备份告警
type BackupAlert struct {
	ID        string    `json:"id"`
	Level     string    `json:"level"` // "info", "warning", "critical"
	Message   string    `json:"message"`
	JobID     string    `json:"job_id,omitempty"`
	Source    string    `json:"source,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

// SourceStatus 备份源状态
type SourceStatus struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	Status      string    `json:"status"` // "online", "offline", "warning"
	LastBackup  time.Time `json:"last_backup,omitempty"`
	DataSize    string    `json:"data_size"`
	AgentVersion string   `json:"agent_version"`
	Uptime      string    `json:"uptime"`
}

// DashboardData 完整仪表板数据
type DashboardData struct {
	Overview     BackupOverview  `json:"overview"`
	RecentJobs   []RecentJob     `json:"recent_jobs"`
	Trends       []StorageTrend  `json:"trends"`
	Alerts       []BackupAlert   `json:"alerts"`
	Sources      []SourceStatus  `json:"sources"`
	GeneratedAt  time.Time       `json:"generated_at"`
}

// Dashboard 仪表板服务
type Dashboard struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	data     DashboardData
	refreshInterval time.Duration
}

// NewDashboard 创建仪表板服务
func NewDashboard(logger *slog.Logger) *Dashboard {
	if logger == nil {
		logger = slog.Default()
	}
	d := &Dashboard{
		logger:          logger,
		refreshInterval: 5 * time.Minute,
	}
	d.initSampleData()
	return d
}

// initSampleData 初始化示例数据
func (d *Dashboard) initSampleData() {
	now := time.Now()

	d.data = DashboardData{
		Overview: BackupOverview{
			TotalJobs:       12,
			ActiveJobs:      3,
			SuccessfulToday: 8,
			FailedToday:     1,
			TotalSources:    5,
			ProtectedData:   "2.4 TB",
			LastBackupTime:  now.Add(-30 * time.Minute),
			NextBackupTime:  now.Add(2 * time.Hour),
			HealthScore:     92.5,
			UpdatedAt:       now,
		},
		RecentJobs: []RecentJob{
			{
				ID: "job-001", Name: "Web服务器全量备份", Source: "web-server-01",
				Type: "full", Status: "done", Size: "120 GB", Duration: "45m",
				StartedAt: now.Add(-2 * time.Hour), FinishedAt: now.Add(-75 * time.Minute),
			},
			{
				ID: "job-002", Name: "数据库增量备份", Source: "db-master",
				Type: "incremental", Status: "done", Size: "8.5 GB", Duration: "12m",
				StartedAt: now.Add(-90 * time.Minute), FinishedAt: now.Add(-78 * time.Minute),
			},
			{
				ID: "job-003", Name: "文件服务器差异备份", Source: "file-server-01",
				Type: "differential", Status: "running", Size: "—", Duration: "—",
				StartedAt: now.Add(-15 * time.Minute),
			},
			{
				ID: "job-004", Name: "邮件服务器全量备份", Source: "mail-server",
				Type: "full", Status: "failed", Size: "—", Duration: "5m",
				StartedAt: now.Add(-3 * time.Hour), FinishedAt: now.Add(-175 * time.Minute),
			},
		},
		Trends: generateTrends(7),
		Alerts: []BackupAlert{
			{
				ID: "alert-001", Level: "warning", Message: "邮件服务器备份失败：磁盘空间不足",
				JobID: "job-004", Source: "mail-server", Timestamp: now.Add(-3 * time.Hour),
			},
			{
				ID: "alert-002", Level: "info", Message: "Web服务器备份完成，压缩率 68%",
				JobID: "job-001", Source: "web-server-01", Timestamp: now.Add(-75 * time.Minute),
			},
		},
		Sources: []SourceStatus{
			{ID: "src-001", Name: "web-server-01", Platform: "Linux", Status: "online", LastBackup: now.Add(-75 * time.Minute), DataSize: "450 GB", AgentVersion: "2.1.0", Uptime: "45d 12h"},
			{ID: "src-002", Name: "db-master", Platform: "Linux", Status: "online", LastBackup: now.Add(-78 * time.Minute), DataSize: "120 GB", AgentVersion: "2.1.0", Uptime: "30d 8h"},
			{ID: "src-003", Name: "file-server-01", Platform: "Windows", Status: "online", LastBackup: now.Add(-15 * time.Minute), DataSize: "1.2 TB", AgentVersion: "2.0.5", Uptime: "15d 4h"},
			{ID: "src-004", Name: "mail-server", Platform: "Linux", Status: "warning", LastBackup: now.Add(-3 * time.Hour), DataSize: "80 GB", AgentVersion: "2.1.0", Uptime: "60d"},
			{ID: "src-005", Name: "vm-cluster", Platform: "VMware", Status: "online", LastBackup: now.Add(-6 * time.Hour), DataSize: "600 GB", AgentVersion: "2.0.8", Uptime: "90d"},
		},
		GeneratedAt: now,
	}
}

// generateTrends 生成趋势数据
func generateTrends(days int) []StorageTrend {
	trends := make([]StorageTrend, days)
	now := time.Now()
	baseUsed := int64(2000 * 1024 * 1024 * 1024) // 2TB
	totalBytes := int64(4 * 1024 * 1024 * 1024 * 1024) // 4TB

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -(days - 1 - i))
		growth := int64(i * 50 * 1024 * 1024 * 1024) // 每天增长 50GB
		used := baseUsed + growth

		trends[i] = StorageTrend{
			Date:        date.Format("2006-01-02"),
			UsedBytes:   used,
			TotalBytes:  totalBytes,
			GrowthBytes: 50 * 1024 * 1024 * 1024,
			GrowthRate:  float64(50*1024*1024*1024) / float64(totalBytes) * 100,
		}
	}
	return trends
}

// GetData 获取完整仪表板数据
func (d *Dashboard) GetData() DashboardData {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.data
}

// GetOverview 获取概览数据
func (d *Dashboard) GetOverview() BackupOverview {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.data.Overview
}

// GetRecentJobs 获取最近任务
func (d *Dashboard) GetRecentJobs(limit int) []RecentJob {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.data.RecentJobs) {
		limit = len(d.data.RecentJobs)
	}
	result := make([]RecentJob, limit)
	copy(result, d.data.RecentJobs[:limit])
	return result
}

// GetAlerts 获取告警
func (d *Dashboard) GetAlerts(includeResolved bool) []BackupAlert {
	d.mu.RLock()
	defer d.mu.RUnlock()

	alerts := make([]BackupAlert, 0)
	for _, alert := range d.data.Alerts {
		if !includeResolved && alert.Resolved {
			continue
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// RegisterRoutes 注册 HTTP 路由
func (d *Dashboard) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/backup/dashboard", d.handleDashboard)
	mux.HandleFunc("/api/v1/backup/dashboard/overview", d.handleOverview)
	mux.HandleFunc("/api/v1/backup/dashboard/jobs", d.handleRecentJobs)
	mux.HandleFunc("/api/v1/backup/dashboard/trends", d.handleTrends)
	mux.HandleFunc("/api/v1/backup/dashboard/alerts", d.handleAlerts)
	mux.HandleFunc("/api/v1/backup/dashboard/sources", d.handleSources)
}

func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := d.GetData()
	writeDashboardJSON(w, data)
}

func (d *Dashboard) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeDashboardJSON(w, d.GetOverview())
}

func (d *Dashboard) handleRecentJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeDashboardJSON(w, d.GetRecentJobs(10))
}

func (d *Dashboard) handleTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d.mu.RLock()
	data := d.data.Trends
	d.mu.RUnlock()
	writeDashboardJSON(w, data)
}

func (d *Dashboard) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	includeResolved := r.URL.Query().Get("include_resolved") == "true"
	writeDashboardJSON(w, d.GetAlerts(includeResolved))
}

func (d *Dashboard) handleSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	d.mu.RLock()
	data := d.data.Sources
	d.mu.RUnlock()
	writeDashboardJSON(w, data)
}

func writeDashboardJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
