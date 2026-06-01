// Package smartinsight 提供智能洞察功能，包括系统使用分析、智能推荐、异常检测、成本分析和报告生成。
package smartinsight

import "time"

// InsightCategory 洞察类别
type InsightCategory string

const (
	CategoryStorage  InsightCategory = "storage"
	CategoryCPU      InsightCategory = "cpu"
	CategoryMemory   InsightCategory = "memory"
	CategoryNetwork  InsightCategory = "network"
	CategorySecurity InsightCategory = "security"
)

// InsightSeverity 洞察严重程度
type InsightSeverity string

const (
	SeverityInfo     InsightSeverity = "info"
	SeverityWarning  InsightSeverity = "warning"
	SeverityCritical InsightSeverity = "critical"
)

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalyFileAccess    AnomalyType = "file_access"
	AnomalyResourceSpike AnomalyType = "resource_spike"
	AnomalyUnusualIO     AnomalyType = "unusual_io"
	AnomalySuspiciousNet AnomalyType = "suspicious_network"
	AnomalyDiskActivity  AnomalyType = "disk_activity"
)

// RecommendationType 推荐类型
type RecommendationType string

const (
	RecStorageOptimize RecommendationType = "storage_optimize"
	RecCacheTuning     RecommendationType = "cache_tuning"
	RecDeduplication   RecommendationType = "deduplication"
	RecTiering         RecommendationType = "tiering"
	RecCompression     RecommendationType = "compression"
	RecCleanup         RecommendationType = "cleanup"
)

// Insight 单条洞察
type Insight struct {
	ID        string          `json:"id"`
	Category  InsightCategory `json:"category"`
	Severity  InsightSeverity `json:"severity"`
	Title     string          `json:"title"`
	Summary   string          `json:"summary"`
	Detail    string          `json:"detail,omitempty"`
	Score     float64         `json:"score"`
	CreatedAt time.Time       `json:"created_at"`
}

// Recommendation 智能推荐
type Recommendation struct {
	ID          string             `json:"id"`
	Type        RecommendationType `json:"type"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Impact      string             `json:"impact"`
	Effort      string             `json:"effort"`
	Score       float64            `json:"score"`
	CreatedAt   time.Time          `json:"created_at"`
}

// UsageTrend 使用趋势
type UsageTrend struct {
	ID        string        `json:"id"`
	Category  string        `json:"category"`
	Current   float64       `json:"current"`
	Previous  float64       `json:"previous"`
	Unit      string        `json:"unit"`
	Direction string        `json:"direction"` // up, down, stable
	ChangePct float64       `json:"change_pct"`
	TrendData []TrendPoint  `json:"trend_data,omitempty"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// CostAnalysis 成本分析
type CostAnalysis struct {
	ID               string         `json:"id"`
	StorageUsedGB    float64        `json:"storage_used_gb"`
	StorageTotalGB   float64        `json:"storage_total_gb"`
	CostPerGB        float64        `json:"cost_per_gb"`
	CurrentCost      float64        `json:"current_cost"`
	ProjectedCost    float64        `json:"projected_cost"`
	SavingsPotential float64        `json:"savings_potential"`
	Breakdown        []CostItem     `json:"breakdown"`
	CreatedAt        time.Time      `json:"created_at"`
}

// CostItem 成本明细项
type CostItem struct {
	Category string  `json:"category"`
	Cost     float64 `json:"cost"`
	Percent  float64 `json:"percent"`
}

// Anomaly 异常事件
type Anomaly struct {
	ID          string      `json:"id"`
	Type        AnomalyType `json:"type"`
	Severity    string      `json:"severity"`
	Description string      `json:"description"`
	Resource    string      `json:"resource"`
	Value       float64     `json:"value"`
	Threshold   float64     `json:"threshold"`
	DetectedAt  time.Time   `json:"detected_at"`
}

// InsightReport 系统洞察报告
type InsightReport struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	Summary         string           `json:"summary"`
	HealthScore     float64          `json:"health_score"`
	Insights        []*Insight       `json:"insights"`
	Recommendations []*Recommendation `json:"recommendations"`
	Trends          []*UsageTrend    `json:"trends"`
	Anomalies       []*Anomaly       `json:"anomalies"`
	Cost            *CostAnalysis    `json:"cost,omitempty"`
	GeneratedAt     time.Time        `json:"generated_at"`
}

// StatsOverview 系统统计概览
type StatsOverview struct {
	TotalInsights        int     `json:"total_insights"`
	TotalRecommendations int     `json:"total_recommendations"`
	TotalAnomalies       int     `json:"total_anomalies"`
	HealthScore          float64 `json:"health_score"`
	LastReportTime       string  `json:"last_report_time"`
}

// AnalyzeRequest 分析请求参数
type AnalyzeRequest struct {
	Category string `json:"category,omitempty"`
	Period   string `json:"period,omitempty"` // 1h, 6h, 24h, 7d, 30d
}
