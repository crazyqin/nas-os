// Package active 备份监控仪表板数据接口
// 提供备份状态汇总、存储使用趋势、恢复点列表，WebSocket 实时推送备份进度
package active

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// DashboardHandler 仪表板数据处理器.
type DashboardHandler struct {
	mu        sync.RWMutex
	engine    *Engine
	manager   *BackupManager
	restore   *RestoreManager
	logger    *zap.Logger
	clients   map[*websocket.Conn]bool
	broadcast chan DashboardEvent
	upgrader  websocket.Upgrader
}

// DashboardEvent 仪表板事件.
type DashboardEvent struct {
	Type      string      `json:"type"` // "progress", "task_update", "storage_update", "alert"
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// BackupSummary 备份状态汇总.
type BackupSummary struct {
	TotalJobs      int              `json:"total_jobs"`
	RunningJobs    int              `json:"running_jobs"`
	CompletedJobs  int              `json:"completed_jobs"`
	FailedJobs     int              `json:"failed_jobs"`
	TotalSnapshots int              `json:"total_snapshots"`
	TotalStorage   int64            `json:"total_storage"` // 总存储使用（字节）
	StorageSaved   int64            `json:"storage_saved"` // 去重节省（字节）
	DedupRatio     float64          `json:"dedup_ratio"`   // 去重率
	OnlineAgents   int              `json:"online_agents"`
	LastBackupTime *time.Time       `json:"last_backup_time"`
	NextBackupTime *time.Time       `json:"next_backup_time"`
	EngineState    string           `json:"engine_state"`
	JobsByType     map[string]int   `json:"jobs_by_type"`   // 备份类型分布
	StorageByJob   []JobStorageInfo `json:"storage_by_job"` // 每个任务的存储使用
}

// JobStorageInfo 任务存储信息.
type JobStorageInfo struct {
	JobID         string `json:"job_id"`
	JobName       string `json:"job_name"`
	SnapshotCount int    `json:"snapshot_count"`
	TotalSize     int64  `json:"total_size"`
	LastBackup    string `json:"last_backup"`
}

// StorageTrend 存储使用趋势.
type StorageTrend struct {
	Points []StorageTrendPoint `json:"points"`
}

// StorageTrendPoint 存储趋势数据点.
type StorageTrendPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	TotalSize     int64     `json:"total_size"`
	UniqueSize    int64     `json:"unique_size"`
	SavedSize     int64     `json:"saved_size"`
	SnapshotCount int       `json:"snapshot_count"`
}

// BackupProgressEvent 备份进度事件（WebSocket 推送用）.
type BackupProgressEvent struct {
	JobID      string  `json:"job_id"`
	TaskRunID  string  `json:"task_run_id"`
	BackupType string  `json:"backup_type"`
	Status     string  `json:"status"`
	Progress   float64 `json:"progress"`
	FilesDone  int     `json:"files_done"`
	FilesTotal int     `json:"files_total"`
	BytesDone  int64   `json:"bytes_done"`
	BytesTotal int64   `json:"bytes_total"`
	Speed      float64 `json:"speed"` // MB/s
	ETA        int     `json:"eta"`   // 预计剩余秒数
	Message    string  `json:"message"`
}

// NewDashboardHandler 创建仪表板处理器.
func NewDashboardHandler(engine *Engine, manager *BackupManager, restore *RestoreManager, logger *zap.Logger) *DashboardHandler {
	if logger == nil {
		logger = zap.NewNop()
	}

	dh := &DashboardHandler{
		engine:    engine,
		manager:   manager,
		restore:   restore,
		logger:    logger,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan DashboardEvent, 256),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	// 注册引擎事件回调
	if engine != nil {
		engine.SetEventCallback(dh.onEngineEvent)
	}

	// 启动广播协程
	go dh.broadcastLoop()

	return dh
}

// GetSummary 获取备份状态汇总.
func (dh *DashboardHandler) GetSummary(c *gin.Context) {
	summary := dh.buildSummary()
	c.JSON(http.StatusOK, summary)
}

// GetStorageTrend 获取存储使用趋势.
func (dh *DashboardHandler) GetStorageTrend(c *gin.Context) {
	trend := dh.buildStorageTrend()
	c.JSON(http.StatusOK, trend)
}

// GetRestorePoints 获取恢复点列表.
func (dh *DashboardHandler) GetRestorePoints(c *gin.Context) {
	jobID := c.Query("job_id")
	points := dh.restore.ListRestorePoints(jobID)
	c.JSON(http.StatusOK, gin.H{
		"restore_points": points,
		"total":          len(points),
	})
}

// HandleWebSocket 处理 WebSocket 连接.
func (dh *DashboardHandler) HandleWebSocket(c *gin.Context) {
	conn, err := dh.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		dh.logger.Error("WebSocket 升级失败", zap.Error(err))
		return
	}

	dh.mu.Lock()
	dh.clients[conn] = true
	dh.mu.Unlock()

	dh.logger.Info("仪表板 WebSocket 客户端连接",
		zap.String("remote", conn.RemoteAddr().String()))

	// 发送初始状态
	summary := dh.buildSummary()
	initialEvent := DashboardEvent{
		Type:      "init",
		Timestamp: time.Now(),
		Payload:   summary,
	}
	data, _ := json.Marshal(initialEvent)
	conn.WriteMessage(websocket.TextMessage, data)

	// 保持连接并处理客户端消息
	defer func() {
		dh.mu.Lock()
		delete(dh.clients, conn)
		dh.mu.Unlock()
		conn.Close()
		dh.logger.Info("仪表板 WebSocket 客户端断开")
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				dh.logger.Warn("WebSocket 读取错误", zap.Error(err))
			}
			break
		}
	}
}

// BroadcastProgress 广播备份进度.
func (dh *DashboardHandler) BroadcastProgress(progress BackupProgressEvent) {
	dh.broadcast <- DashboardEvent{
		Type:      "progress",
		Timestamp: time.Now(),
		Payload:   progress,
	}
}

// onEngineEvent 引擎事件回调.
func (dh *DashboardHandler) onEngineEvent(event EngineEvent) {
	dh.broadcast <- DashboardEvent{
		Type:      "task_update",
		Timestamp: event.Timestamp,
		Payload:   event,
	}
}

// broadcastLoop 广播循环.
func (dh *DashboardHandler) broadcastLoop() {
	for event := range dh.broadcast {
		data, err := json.Marshal(event)
		if err != nil {
			dh.logger.Error("序列化广播事件失败", zap.Error(err))
			continue
		}

		dh.mu.RLock()
		clients := make([]*websocket.Conn, 0, len(dh.clients))
		for conn := range dh.clients {
			clients = append(clients, conn)
		}
		dh.mu.RUnlock()

		for _, conn := range clients {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				dh.logger.Warn("WebSocket 写入失败", zap.Error(err))
				dh.mu.Lock()
				delete(dh.clients, conn)
				dh.mu.Unlock()
				conn.Close()
			}
		}
	}
}

// buildSummary 构建备份状态汇总.
func (dh *DashboardHandler) buildSummary() *BackupSummary {
	jobs := dh.manager.ListJobs()
	snapshots := dh.manager.ListSnapshots("")

	summary := &BackupSummary{
		TotalJobs:      len(jobs),
		TotalSnapshots: len(snapshots),
		JobsByType:     make(map[string]int),
		StorageByJob:   make([]JobStorageInfo, 0),
	}

	if dh.engine != nil {
		summary.EngineState = string(dh.engine.GetState())
		summary.OnlineAgents = dh.engine.GetAgentRegistry().Count()
	}

	var totalStorage int64
	var lastBackup *time.Time
	var nextBackup *time.Time

	jobStorageMap := make(map[string]*JobStorageInfo)

	for _, job := range jobs {
		switch job.Status {
		case BackupStatusRunning:
			summary.RunningJobs++
		case BackupStatusCompleted:
			summary.CompletedJobs++
		case BackupStatusFailed:
			summary.FailedJobs++
		}

		summary.JobsByType[string(job.Policy.Type)]++

		if job.LastRun != nil {
			if lastBackup == nil || job.LastRun.After(*lastBackup) {
				lastBackup = job.LastRun
			}
		}
		if job.NextRun != nil {
			if nextBackup == nil || job.NextRun.Before(*nextBackup) {
				nextBackup = job.NextRun
			}
		}

		// 按任务统计存储
		jobStorageMap[job.ID] = &JobStorageInfo{
			JobID:   job.ID,
			JobName: job.Name,
			LastBackup: func() string {
				if job.LastRun != nil {
					return job.LastRun.Format(time.RFC3339)
				}
				return ""
			}(),
		}
	}

	for _, snap := range snapshots {
		totalStorage += snap.Size
		if info, ok := jobStorageMap[snap.JobID]; ok {
			info.TotalSize += snap.Size
			info.SnapshotCount++
		}
	}

	for _, info := range jobStorageMap {
		summary.StorageByJob = append(summary.StorageByJob, *info)
	}

	summary.TotalStorage = totalStorage
	summary.LastBackupTime = lastBackup
	summary.NextBackupTime = nextBackup

	// 计算去重统计
	if dh.engine != nil {
		dedupStats := dh.engine.GetDedupEngine().GetStats()
		summary.StorageSaved = dedupStats.SavedBytes
		summary.DedupRatio = dedupStats.DedupRatio
	}

	return summary
}

// buildStorageTrend 构建存储使用趋势.
func (dh *DashboardHandler) buildStorageTrend() *StorageTrend {
	snapshots := dh.manager.ListSnapshots("")

	// 按天聚合
	type dayStat struct {
		TotalSize     int64
		UniqueSize    int64
		SavedSize     int64
		SnapshotCount int
	}
	dayMap := make(map[string]*dayStat)

	for _, snap := range snapshots {
		day := snap.CreatedAt.Format("2006-01-02")
		if _, ok := dayMap[day]; !ok {
			dayMap[day] = &dayStat{}
		}
		dayMap[day].TotalSize += snap.Size
		dayMap[day].SnapshotCount++
	}

	// 如果有去重引擎，添加去重数据
	if dh.engine != nil {
		stats := dh.engine.GetDedupEngine().GetStats()
		// 简单分配到最近一天
		if len(dayMap) > 0 {
			for _, ds := range dayMap {
				ds.SavedSize = stats.SavedBytes / int64(len(dayMap))
				ds.UniqueSize = ds.TotalSize - ds.SavedSize
				break
			}
		}
	}

	trend := &StorageTrend{
		Points: make([]StorageTrendPoint, 0, len(dayMap)),
	}

	for day, stat := range dayMap {
		ts, _ := time.Parse("2006-01-02", day)
		trend.Points = append(trend.Points, StorageTrendPoint{
			Timestamp:     ts,
			TotalSize:     stat.TotalSize,
			UniqueSize:    stat.UniqueSize,
			SavedSize:     stat.SavedSize,
			SnapshotCount: stat.SnapshotCount,
		})
	}

	return trend
}
