package cloudsync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sync Health Endpoints — /healthz/sync
// ─────────────────────────────────────────────────────────────────────────────

// HealthStatus 同步队列健康状态.
type HealthStatus struct {
	QueueDepth       int64 `json:"queue_depth"`
	ActiveTasks     int64 `json:"active_tasks"`
	PendingTasks    int64 `json:"pending_tasks"`
	FailedTasks     int64 `json:"failed_tasks"`
	TotalProviders  int64 `json:"total_providers"`
	HealthyProviders int64 `json:"healthy_providers"`
	UptimeSeconds   int64 `json:"uptime_seconds"`
}

// syncMetrics 同步 Prometheus 指标.
type syncMetrics struct {
	TasksTotal        atomic.Int64
	TasksCompleted    atomic.Int64
	TasksFailed       atomic.Int64
	BytesTransferred  atomic.Int64
	BytesUploaded     atomic.Int64
	BytesDownloaded   atomic.Int64
	ErrorsTotal       atomic.Int64
	QueueSize         atomic.Int64
	ActiveWorkers     atomic.Int64
	StartTime         int64 // Unix 秒，启动时间
}

// globalMetrics 全局指标实例（daemon 启动时初始化）.
var globalMetrics syncMetrics

// InitMetrics 初始化指标（daemon 启动时调用）.
func InitMetrics() {
	globalMetrics.StartTime = atomic.LoadInt64(&globalMetrics.StartTime)
}

// SyncHealthHandler 处理 /healthz/sync 请求.
func SyncHealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := HealthStatus{
		QueueDepth:    globalMetrics.QueueSize.Load(),
		ActiveTasks:   globalMetrics.ActiveWorkers.Load(),
		PendingTasks:  globalMetrics.TasksTotal.Load() - globalMetrics.TasksCompleted.Load() - globalMetrics.TasksFailed.Load(),
		FailedTasks:   globalMetrics.TasksFailed.Load(),
		UptimeSeconds: globalMetrics.StartTime,
	}

	// 健康判断
	httpCode := http.StatusOK
	message := "ok"
	if status.FailedTasks > 10 || status.QueueDepth > 1000 {
		httpCode = http.StatusServiceUnavailable
		message = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": message,
		"data":   status,
	})
}

// RecordTaskStarted 记录任务开始.
func RecordTaskStarted(sizeBytes int64) {
	globalMetrics.TasksTotal.Add(1)
	globalMetrics.QueueSize.Add(1)
	if sizeBytes > 0 {
		globalMetrics.BytesTransferred.Add(sizeBytes)
	}
}

// RecordTaskCompleted 记录任务完成.
func RecordTaskCompleted(bytesTransferred int64) {
	globalMetrics.TasksCompleted.Add(1)
	globalMetrics.QueueSize.Add(-1)
	if bytesTransferred > 0 {
		globalMetrics.BytesTransferred.Add(bytesTransferred)
	}
}

// RecordTaskFailed 记录任务失败.
func RecordTaskFailed() {
	globalMetrics.TasksFailed.Add(1)
	globalMetrics.QueueSize.Add(-1)
}

// RecordSyncError 记录同步错误.
func RecordSyncError() {
	globalMetrics.ErrorsTotal.Add(1)
}

// ─────────────────────────────────────────────────────────────────────────────
// Prometheus Metrics Endpoints — /metrics
// ─────────────────────────────────────────────────────────────────────────────

// MetricsHandler 处理 Prometheus metrics 请求.
// 暴露指标: sync_tasks_total, sync_bytes_transferred, sync_errors_total
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	metrics := fmt.Sprintf(`# HELP nas_sync_tasks_total Total number of sync tasks processed
# TYPE nas_sync_tasks_total counter
nas_sync_tasks_total{status="completed"} %d
nas_sync_tasks_total{status="failed"} %d

# HELP nas_sync_bytes_transferred Total bytes transferred (uploads + downloads)
# TYPE nas_sync_bytes_transferred counter
nas_sync_bytes_transferred %d

# HELP nas_sync_errors_total Total number of sync errors
# TYPE nas_sync_errors_total counter
nas_sync_errors_total %d

# HELP nas_sync_queue_size Current number of tasks waiting in queue
# TYPE nas_sync_queue_size gauge
nas_sync_queue_size %d

# HELP nas_sync_active_workers Number of currently running sync workers
# TYPE nas_sync_active_workers gauge
nas_sync_active_workers %d
`,
		globalMetrics.TasksCompleted.Load(),
		globalMetrics.TasksFailed.Load(),
		globalMetrics.BytesTransferred.Load(),
		globalMetrics.ErrorsTotal.Load(),
		globalMetrics.QueueSize.Load(),
		globalMetrics.ActiveWorkers.Load(),
	)
	w.Write([]byte(metrics))
}
