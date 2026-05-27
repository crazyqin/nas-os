package optimizer

import (
	"sync"
	"time"
)

// ResourceMetrics 资源使用指标
type ResourceMetrics struct {
	Timestamp    time.Time `json:"timestamp"`
	CPUPercent   float64   `json:"cpu_percent"`
	MemPercent   float64   `json:"mem_percent"`
	MemUsedMB    float64   `json:"mem_used_mb"`
	MemTotalMB   float64   `json:"mem_total_mb"`
	DiskReadKB   float64   `json:"disk_read_kb"`
	DiskWriteKB  float64   `json:"disk_write_kb"`
	NetworkInKB  float64   `json:"network_in_kb"`
	NetworkOutKB float64   `json:"network_out_kb"`
	IOPSRead     int64     `json:"iops_read"`
	IOPSWrite    int64     `json:"iops_write"`
	LoadAvg1     float64   `json:"load_avg_1"`
	LoadAvg5     float64   `json:"load_avg_5"`
	LoadAvg15    float64   `json:"load_avg_15"`
}

// Bottleneck 瓶颈信息
type Bottleneck struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`        // cpu, memory, io, network
	Severity    string    `json:"severity"`    // critical, warning, info
	Description string    `json:"description"`
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	DetectedAt  time.Time `json:"detected_at"`
	Suggestions []string  `json:"suggestions"`
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	ID              string        `json:"id"`
	Category        string        `json:"category"`        // cpu, memory, io, network, general
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Impact          string        `json:"impact"`          // high, medium, low
	EstimatedGain   float64       `json:"estimated_gain"`  // 预计提升百分比
	Risk            string        `json:"risk"`            // high, medium, low
	Implementation  string        `json:"implementation"`  // 实施步骤
	AutoApplicable  bool          `json:"auto_applicable"` // 是否可自动应用
	CreatedAt       time.Time     `json:"created_at"`
	AppliedAt       *time.Time    `json:"applied_at,omitempty"`
	ActualGain      *float64      `json:"actual_gain,omitempty"`
}

// OptimizationRecord 优化记录
type OptimizationRecord struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`          // auto, manual, scheduled
	Category      string                 `json:"category"`      // cpu, memory, io, network
	Action        string                 `json:"action"`
	Parameters    map[string]interface{} `json:"parameters"`
	BeforeMetrics *ResourceMetrics       `json:"before_metrics"`
	AfterMetrics  *ResourceMetrics       `json:"after_metrics"`
	Improvement   float64                `json:"improvement"` // 性能提升百分比
	Duration      time.Duration          `json:"duration"`
	Status        string                 `json:"status"` // success, failed, partial
	Error         string                 `json:"error,omitempty"`
	ExecutedAt    time.Time              `json:"executed_at"`
	ExecutedBy    string                 `json:"executed_by"` // system, user, scheduler
}

// ScheduledTask 定时优化任务
type ScheduledTask struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	CronExpr     string        `json:"cron_expr"`     // cron 表达式
	Category     string        `json:"category"`       // cpu, memory, io, network, all
	Actions      []string      `json:"actions"`        // 要执行的优化动作
	Enabled      bool          `json:"enabled"`
	LastRun      *time.Time    `json:"last_run,omitempty"`
	NextRun      *time.Time    `json:"next_run,omitempty"`
	RunCount     int           `json:"run_count"`
	FailCount    int           `json:"fail_count"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// PredictionResult 预测结果
type PredictionResult struct {
	Resource    string        `json:"resource"`     // cpu, memory, disk, network
	CurrentValue float64      `json:"current_value"`
	Predicted1H  float64      `json:"predicted_1h"`
	Predicted6H  float64      `json:"predicted_6h"`
	Predicted24H float64      `json:"predicted_24h"`
	Trend        string        `json:"trend"`        // increasing, decreasing, stable
	Confidence   float64      `json:"confidence"`   // 0-100 置信度
	Warning      bool          `json:"warning"`
	WarningMsg   string        `json:"warning_msg,omitempty"`
}

// AutoTuneConfig 自动调优配置
type AutoTuneConfig struct {
	Enabled         bool    `json:"enabled"`
	CPUThreshold    float64 `json:"cpu_threshold"`     // CPU 使用率阈值
	MemThreshold    float64 `json:"mem_threshold"`     // 内存使用率阈值
	IOThreshold     float64 `json:"io_threshold"`      // IO 使用率阈值
	TuneInterval    int     `json:"tune_interval"`     // 调优间隔（秒）
	MaxConcurrent   int     `json:"max_concurrent"`    // 最大并发调优数
	DryRun          bool    `json:"dry_run"`           // 仅模拟，不实际执行
	AutoApply       bool    `json:"auto_apply"`        // 自动应用优化建议
	NotifyOnTune    bool    `json:"notify_on_tune"`    // 调优时发送通知
}

// DefaultAutoTuneConfig 默认自动调优配置
func DefaultAutoTuneConfig() AutoTuneConfig {
	return AutoTuneConfig{
		Enabled:       true,
		CPUThreshold:  80.0,
		MemThreshold:  85.0,
		IOThreshold:   70.0,
		TuneInterval:  300, // 5 分钟
		MaxConcurrent: 3,
		DryRun:        false,
		AutoApply:     true,
		NotifyOnTune:  true,
	}
}

// EngineStats 引擎统计
type EngineStats struct {
	TotalOptimizations  int64     `json:"total_optimizations"`
	SuccessfulTunes     int64     `json:"successful_tunes"`
	FailedTunes         int64     `json:"failed_tunes"`
	TotalImprovement    float64   `json:"total_improvement"`    // 总体提升百分比
	AvgImprovement      float64   `json:"avg_improvement"`      // 平均提升百分比
	LastOptimization    time.Time `json:"last_optimization"`
	Uptime              time.Duration `json:"uptime"`
	BottlenecksDetected int       `json:"bottlenecks_detected"`
	PredictionsMade     int       `json:"predictions_made"`
	ScheduledRuns       int       `json:"scheduled_runs"`
}

// OptimizationHistory 优化历史管理器
type OptimizationHistory struct {
	mu       sync.RWMutex
	records  []*OptimizationRecord
	maxSize  int
}

// NewOptimizationHistory 创建优化历史管理器
func NewOptimizationHistory(maxSize int) *OptimizationHistory {
	return &OptimizationHistory{
		records: make([]*OptimizationRecord, 0),
		maxSize: maxSize,
	}
}

// Add 添加记录
func (h *OptimizationHistory) Add(record *OptimizationRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, record)

	// 超过最大容量时移除最旧的记录
	if len(h.records) > h.maxSize {
		h.records = h.records[len(h.records)-h.maxSize:]
	}
}

// GetAll 获取所有记录
func (h *OptimizationHistory) GetAll() []*OptimizationRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*OptimizationRecord, len(h.records))
	copy(result, h.records)
	return result
}

// GetByType 按类型获取记录
func (h *OptimizationHistory) GetByType(recordType string) []*OptimizationRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*OptimizationRecord
	for _, r := range h.records {
		if r.Type == recordType {
			result = append(result, r)
		}
	}
	return result
}

// GetByCategory 按类别获取记录
func (h *OptimizationHistory) GetByCategory(category string) []*OptimizationRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*OptimizationRecord
	for _, r := range h.records {
		if r.Category == category {
			result = append(result, r)
		}
	}
	return result
}

// GetRecent 获取最近的记录
func (h *OptimizationHistory) GetRecent(limit int) []*OptimizationRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if limit <= 0 || limit > len(h.records) {
		limit = len(h.records)
	}

	start := len(h.records) - limit
	result := make([]*OptimizationRecord, limit)
	copy(result, h.records[start:])
	return result
}

// Clear 清空历史
func (h *OptimizationHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = make([]*OptimizationRecord, 0)
}

// Count 记录数量
func (h *OptimizationHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.records)
}
