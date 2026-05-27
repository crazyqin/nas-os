// Package resourceoptimizer 提供系统资源综合优化功能
package resourceoptimizer

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrInsufficientData 数据不足.
	ErrInsufficientData = errors.New("历史数据不足，至少需要3个数据点")
	// ErrAnalysisInProgress 分析正在进行中.
	ErrAnalysisInProgress = errors.New("资源分析正在进行中")
	// ErrInvalidResourceType 无效资源类型.
	ErrInvalidResourceType = errors.New("无效的资源类型")
	// ErrRecommendationNotFound 建议不存在.
	ErrRecommendationNotFound = errors.New("优化建议不存在")
)

// ========== 资源类型 ==========

// ResourceType 资源类型.
type ResourceType string

const (
	ResourceCPU     ResourceType = "cpu"
	ResourceMemory  ResourceType = "memory"
	ResourceDisk    ResourceType = "disk"
	ResourceNetwork ResourceType = "network"
)

// ========== 优先级 ==========

// Priority 优先级等级.
type Priority int

const (
	PriorityLow Priority = iota + 1
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

// String 返回优先级字符串.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ========== 资源使用数据 ==========

// ResourceUsage 资源使用数据点.
type ResourceUsage struct {
	Timestamp    time.Time    `json:"timestamp"`
	ResourceType ResourceType `json:"resource_type"`
	Used         float64      `json:"used"`          // 已使用量
	Total        float64      `json:"total"`         // 总量
	UsagePercent float64      `json:"usage_percent"` // 使用率百分比
	Unit         string       `json:"unit"`          // 单位 (%, MB, MB/s 等)
	ProcessName  string       `json:"process_name"`  // 相关进程名
	Details      string       `json:"details"`       // 附加详情
}

// ResourceSnapshot 资源快照.
type ResourceSnapshot struct {
	Timestamp  time.Time               `json:"timestamp"`
	CPU        *CPUMetrics             `json:"cpu"`
	Memory     *MemoryMetrics          `json:"memory"`
	Disk       *DiskMetrics            `json:"disk"`
	Network    *NetworkMetrics         `json:"network"`
	Processes  []ProcessMetrics        `json:"processes,omitempty"`
}

// CPUMetrics CPU 指标.
type CPUMetrics struct {
	UsagePercent   float64   `json:"usage_percent"`
	CoreUsage      []float64 `json:"core_usage"`
	LoadAvg1       float64   `json:"load_avg_1"`
	LoadAvg5       float64   `json:"load_avg_5"`
	LoadAvg15      float64   `json:"load_avg_15"`
	ContextSwitches uint64   `json:"context_switches"`
}

// MemoryMetrics 内存指标.
type MemoryMetrics struct {
	TotalMB        float64 `json:"total_mb"`
	UsedMB         float64 `json:"used_mb"`
	FreeMB         float64 `json:"free_mb"`
	CachedMB       float64 `json:"cached_mb"`
	BuffersMB      float64 `json:"buffers_mb"`
	SwapTotalMB    float64 `json:"swap_total_mb"`
	SwapUsedMB     float64 `json:"swap_used_mb"`
	UsagePercent   float64 `json:"usage_percent"`
}

// DiskMetrics 磁盘指标.
type DiskMetrics struct {
	Device         string  `json:"device"`
	MountPoint     string  `json:"mount_point"`
	TotalGB        float64 `json:"total_gb"`
	UsedGB         float64 `json:"used_gb"`
	FreeGB         float64 `json:"free_gb"`
	UsagePercent   float64 `json:"usage_percent"`
	ReadMBps       float64 `json:"read_mbps"`
	WriteMBps      float64 `json:"write_mbps"`
	IOUtilization  float64 `json:"io_utilization"`
}

// NetworkMetrics 网络指标.
type NetworkMetrics struct {
	Interface      string  `json:"interface"`
	RxMBps         float64 `json:"rx_mbps"`
	TxMBps         float64 `json:"tx_mbps"`
	TotalRxMB      float64 `json:"total_rx_mb"`
	TotalTxMB      float64 `json:"total_tx_mb"`
	Connections    int     `json:"connections"`
	Errors         int     `json:"errors"`
	Dropped        int     `json:"dropped"`
}

// ProcessMetrics 进程指标.
type ProcessMetrics struct {
	PID            int     `json:"pid"`
	Name           string  `json:"name"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryMB       float64 `json:"memory_mb"`
	MemoryPercent  float64 `json:"memory_percent"`
	Status         string  `json:"status"`
}

// ========== 优化建议 ==========

// Recommendation 优化建议.
type Recommendation struct {
	ID             string       `json:"id"`
	ResourceType   ResourceType `json:"resource_type"`
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	CurrentValue   string       `json:"current_value"`
	ExpectedValue  string       `json:"expected_value"`
	Priority       Priority     `json:"priority"`
	EstimatedSaving string      `json:"estimated_saving"` // 预估节省
	Action         string       `json:"action"`           // 建议操作
	Category       string       `json:"category"`         // 分类
	CreatedAt      time.Time    `json:"created_at"`
}

// RecommendationList 建议列表.
type RecommendationList struct {
	Recommendations []Recommendation `json:"recommendations"`
	Total           int              `json:"total"`
	GeneratedAt     time.Time        `json:"generated_at"`
}

// ========== 趋势预测 ==========

// TrendPoint 趋势数据点.
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	IsPredicted bool    `json:"is_predicted"`
}

// TrendPrediction 趋势预测结果.
type TrendPrediction struct {
	ResourceType     ResourceType `json:"resource_type"`
	CurrentUsage     float64      `json:"current_usage"`
	PredictedUsage7d float64      `json:"predicted_usage_7d"`  // 7天后预测
	PredictedUsage30d float64     `json:"predicted_usage_30d"` // 30天后预测
	TrendDirection   string       `json:"trend_direction"`     // rising, falling, stable
	Confidence       float64      `json:"confidence"`          // 置信度 0-1
	DataPoints       []TrendPoint `json:"data_points"`
	Warnings         []string     `json:"warnings,omitempty"`
}

// ========== 成本优化 ==========

// CostOptimization 成本优化建议.
type CostOptimization struct {
	ResourceType    ResourceType `json:"resource_type"`
	CurrentCost     float64      `json:"current_cost"`      // 当前月成本估算
	OptimizedCost   float64      `json:"optimized_cost"`    // 优化后月成本估算
	SavingAmount    float64      `json:"saving_amount"`     // 节省金额
	SavingPercent   float64      `json:"saving_percent"`    // 节省百分比
	Recommendations []string     `json:"recommendations"`
}

// ========== 综合分析 ==========

// AnalysisResult 综合分析结果.
type AnalysisResult struct {
	AnalysisID      string               `json:"analysis_id"`
	StartedAt       time.Time            `json:"started_at"`
	FinishedAt      time.Time            `json:"finished_at"`
	DurationSeconds float64              `json:"duration_seconds"`
	Snapshot        *ResourceSnapshot    `json:"snapshot"`
	Trends          []TrendPrediction    `json:"trends"`
	Recommendations []Recommendation     `json:"recommendations"`
	CostOptimizations []CostOptimization `json:"cost_optimizations"`
	OverallScore    float64              `json:"overall_score"` // 0-100 健康评分
}

// ========== 请求/响应 ==========

// AnalyzeRequest 分析请求.
type AnalyzeRequest struct {
	ResourceTypes []ResourceType `json:"resource_types,omitempty"` // 空则分析全部
	IncludeTrend  bool           `json:"include_trend"`
	IncludeCost   bool           `json:"include_cost"`
}

// DefaultAnalyzeRequest 默认分析请求.
func DefaultAnalyzeRequest() *AnalyzeRequest {
	return &AnalyzeRequest{
		ResourceTypes: []ResourceType{ResourceCPU, ResourceMemory, ResourceDisk, ResourceNetwork},
		IncludeTrend:  true,
		IncludeCost:   true,
	}
}
