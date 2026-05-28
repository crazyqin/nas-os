package benchmarkpro

import (
	"time"
)

// BenchTestType 基准测试类型
type BenchTestType string

const (
	TestTypeCPU        BenchTestType = "cpu"
	TestTypeMemory     BenchTestType = "memory"
	TestTypeDiskIO     BenchTestType = "disk_io"
	TestTypeNetwork    BenchTestType = "network"
	TestTypeComprehensive BenchTestType = "comprehensive"
)

// BenchStatus 测试状态
type BenchStatus string

const (
	StatusPending   BenchStatus = "pending"
	StatusRunning   BenchStatus = "running"
	StatusCompleted BenchStatus = "completed"
	StatusFailed    BenchStatus = "failed"
)

// SeverityLevel 瓶颈严重程度
type SeverityLevel string

const (
	SeverityInfo     SeverityLevel = "info"
	SeverityWarning  SeverityLevel = "warning"
	SeverityCritical SeverityLevel = "critical"
)

// Config 基准测试配置
type Config struct {
	Enabled      bool   `json:"enabled"`
	TmpDir       string `json:"tmp_dir"`
	MaxFileSizeMB int   `json:"max_file_size_mb"`
	NetworkTarget string `json:"network_target"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:       true,
		TmpDir:        "/tmp/nas-benchmarkpro",
		MaxFileSizeMB: 4096,
		NetworkTarget: "",
	}
}

// BenchRequest 基准测试请求
type BenchRequest struct {
	TestType     BenchTestType `json:"test_type" binding:"required"`
	TargetPath   string        `json:"target_path,omitempty"`
	FileSizeMB   int           `json:"file_size_mb,omitempty"`
	DurationSec  int           `json:"duration_sec,omitempty"`
	Concurrency  int           `json:"concurrency,omitempty"`
	BlockSizeKB  int           `json:"block_size_kb,omitempty"`
	NetworkTarget string       `json:"network_target,omitempty"`
}

// BenchResult 测试结果
type BenchResult struct {
	ID           string        `json:"id"`
	TestType     BenchTestType `json:"test_type"`
	Status       BenchStatus   `json:"status"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Duration     time.Duration `json:"duration_ms"`
	ErrorMsg     string        `json:"error_msg,omitempty"`

	// CPU 测试结果
	CPUScore      float64 `json:"cpu_score,omitempty"`
	CPUGFLOPS     float64 `json:"cpu_gflops,omitempty"`
	CPUMultiScore float64 `json:"cpu_multi_score,omitempty"`

	// 内存测试结果
	MemBandwidthGBps float64 `json:"mem_bandwidth_gbps,omitempty"`
	MemLatencyNs     float64 `json:"mem_latency_ns,omitempty"`
	MemScore         float64 `json:"mem_score,omitempty"`

	// 磁盘 IO 测试结果
	SeqReadMBps     float64       `json:"seq_read_mbps,omitempty"`
	SeqWriteMBps    float64       `json:"seq_write_mbps,omitempty"`
	RandomReadIOPS  float64       `json:"random_read_iops,omitempty"`
	RandomWriteIOPS float64       `json:"random_write_iops,omitempty"`
	IOLatencyAvg    time.Duration `json:"io_latency_avg,omitempty"`
	IOLatencyP99    time.Duration `json:"io_latency_p99,omitempty"`

	// 网络测试结果
	NetThroughputMbps float64 `json:"net_throughput_mbps,omitempty"`
	NetLatencyMs      float64 `json:"net_latency_ms,omitempty"`
	NetPacketLoss     float64 `json:"net_packet_loss,omitempty"`
	NetMaxConns       int     `json:"net_max_conns,omitempty"`

	// 综合评分
	OverallScore float64 `json:"overall_score,omitempty"`
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	TestType     string    `json:"test_type"`
	Score        float64   `json:"score"`
	CPUScore     float64   `json:"cpu_score,omitempty"`
	MemScore     float64   `json:"mem_score,omitempty"`
	DiskScore    float64   `json:"disk_score,omitempty"`
	NetScore     float64   `json:"net_score,omitempty"`
	OverallScore float64   `json:"overall_score"`
}

// TrendAnalysis 趋势分析结果
type TrendAnalysis struct {
	TestType  string        `json:"test_type"`
	Points    []TrendPoint  `json:"points"`
	Trend     string        `json:"trend"` // "improving", "stable", "degrading"
	ChangePct float64       `json:"change_pct"`
	AvgScore  float64       `json:"avg_score"`
	MinScore  float64       `json:"min_score"`
	MaxScore  float64       `json:"max_score"`
}

// Bottleneck 性能瓶颈
type Bottleneck struct {
	Resource    string        `json:"resource"`
	Severity    SeverityLevel `json:"severity"`
	Description string        `json:"description"`
	Value       float64       `json:"value"`
	Threshold   float64       `json:"threshold"`
	Suggestion  string        `json:"suggestion"`
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	Category    string `json:"category"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
}

// CompetitorEntry 竞品数据条目
type CompetitorEntry struct {
	Name         string  `json:"name"`
	CPUScore     float64 `json:"cpu_score"`
	MemScore     float64 `json:"mem_score"`
	DiskScore    float64 `json:"disk_score"`
	NetScore     float64 `json:"net_score"`
	OverallScore float64 `json:"overall_score"`
}

// CompetitorComparison 竞品对比结果
type CompetitorComparison struct {
	Local      *BenchResult        `json:"local"`
	Competitor *CompetitorEntry    `json:"competitor"`
	Diff       *CompetitorDiff     `json:"diff"`
}

// CompetitorDiff 竞品差异
type CompetitorDiff struct {
	CPUDiff     float64 `json:"cpu_diff"`
	MemDiff     float64 `json:"mem_diff"`
	DiskDiff    float64 `json:"disk_diff"`
	NetDiff     float64 `json:"net_diff"`
	OverallDiff float64 `json:"overall_diff"`
}

// BenchmarkReport 基准测试报告
type BenchmarkReport struct {
	GeneratedAt      time.Time                `json:"generated_at"`
	LatestResult     *BenchResult             `json:"latest_result"`
	TrendAnalysis    *TrendAnalysis           `json:"trend_analysis,omitempty"`
	Bottlenecks      []*Bottleneck            `json:"bottlenecks,omitempty"`
	Suggestions      []*OptimizationSuggestion `json:"suggestions,omitempty"`
	CompetitorData   []*CompetitorEntry       `json:"competitor_data,omitempty"`
	History          []*BenchResult           `json:"history,omitempty"`
}
