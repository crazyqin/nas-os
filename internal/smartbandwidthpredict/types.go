package smartbandwidthpredict

import (
	"fmt"
	"sync"
	"time"
)

// TrendType 趋势类型
type TrendType string

const (
	TrendRising  TrendType = "rising"  // 上升趋势
	TrendFalling TrendType = "falling" // 下降趋势
	TrendStable  TrendType = "stable"  // 稳定
)

// TrafficSample 流量采样
type TrafficSample struct {
	Timestamp    time.Time `json:"timestamp"`     // 采样时间戳
	InboundMbps  float64   `json:"inbound_mbps"`  // 入向带宽 (Mbps)
	OutboundMbps float64   `json:"outbound_mbps"` // 出向带宽 (Mbps)
	LatencyMs    float64   `json:"latency_ms"`    // 延迟 (ms)
	PacketLoss   float64   `json:"packet_loss"`   // 丢包率 (0-100%)
	Interface    string    `json:"interface"`     // 网络接口
}

// BandwidthPrediction 带宽预测结果
type BandwidthPrediction struct {
	Timestamp      time.Time `json:"timestamp"`       // 预测时间点
	PredictedMbps  float64   `json:"predicted_mbps"`  // 预测带宽值
	LowerBound     float64   `json:"lower_bound"`     // 置信区间下界
	UpperBound     float64   `json:"upper_bound"`     // 置信区间上界
	Confidence     float64   `json:"confidence"`      // 置信度 (0-1)
	Trend          TrendType `json:"trend"`           // 趋势
	HorizonMinutes int       `json:"horizon_minutes"` // 预测时长（分钟）
}

// ScheduleTask 调度任务
type ScheduleTask struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Priority     int           `json:"priority"`      // 优先级 1-10
	RequiredMbps float64       `json:"required_mbps"` // 所需带宽
	Duration     time.Duration `json:"duration"`      // 预计持续时间
	ScheduledAt  time.Time     `json:"scheduled_at"`  // 计划开始时间
	Deadline     time.Time     `json:"deadline"`      // 截止时间
}

// SchedulePlan 调度计划
type SchedulePlan struct {
	ID        string          `json:"id"`
	Tasks     []*ScheduleTask `json:"tasks"`
	StartTime time.Time       `json:"start_time"`
	EndTime   time.Time       `json:"end_time"`
	TotalMbps float64         `json:"total_mbps"` // 总分配带宽
	CreatedAt time.Time       `json:"created_at"`
}

// QoSPolicy QoS策略
type QoSPolicy struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MinMbps   float64   `json:"min_mbps"` // 最小带宽保证
	MaxMbps   float64   `json:"max_mbps"` // 最大带宽限制
	Priority  int       `json:"priority"` // 优先级 1-10
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Config 智能带宽预测配置
type Config struct {
	TotalBandwidthMbps float64       `json:"total_bandwidth_mbps"` // 总带宽
	CollectInterval    time.Duration `json:"collect_interval"`     // 采集间隔
	PredictionWindow   int           `json:"prediction_window"`    // 预测窗口（采样点数）
	PredictionHorizon  int           `json:"prediction_horizon"`   // 预测时长（分钟）
	AnomalyThreshold   float64       `json:"anomaly_threshold"`    // 异常检测阈值（标准差倍数）
	SmoothingAlpha     float64       `json:"smoothing_alpha"`      // 指数平滑系数
	MaxSamples         int           `json:"max_samples"`          // 最大采样数
	Interfaces         []string      `json:"interfaces"`           // 监控的网络接口
	Enabled            bool          `json:"enabled"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		TotalBandwidthMbps: 1000,
		CollectInterval:    30 * time.Second,
		PredictionWindow:   100,
		PredictionHorizon:  30,
		AnomalyThreshold:   2.0,
		SmoothingAlpha:     0.3,
		MaxSamples:         10000,
		Interfaces:         []string{"eth0"},
		Enabled:            true,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.TotalBandwidthMbps <= 0 {
		return fmt.Errorf("总带宽必须大于0")
	}
	if c.CollectInterval <= 0 {
		return fmt.Errorf("采集间隔必须大于0")
	}
	if c.PredictionWindow < 10 {
		return fmt.Errorf("预测窗口不能小于10")
	}
	if c.PredictionHorizon <= 0 {
		return fmt.Errorf("预测时长必须大于0")
	}
	if c.AnomalyThreshold <= 0 {
		return fmt.Errorf("异常检测阈值必须大于0")
	}
	if c.SmoothingAlpha <= 0 || c.SmoothingAlpha >= 1 {
		return fmt.Errorf("平滑系数必须在(0,1)之间")
	}
	if c.MaxSamples < 100 {
		return fmt.Errorf("最大采样数不能小于100")
	}
	if len(c.Interfaces) == 0 {
		return fmt.Errorf("至少需要一个网络接口")
	}
	return nil
}

// PredictionEngine 预测引擎
type PredictionEngine struct {
	mu          sync.RWMutex
	samples     []*TrafficSample
	predictions []*BandwidthPrediction
	config      *Config
	running     bool
	stopCh      chan struct{}
}

// NewPredictionEngine 创建预测引擎
func NewPredictionEngine(config *Config) *PredictionEngine {
	if config == nil {
		config = DefaultConfig()
	}
	return &PredictionEngine{
		samples:     make([]*TrafficSample, 0, config.MaxSamples),
		predictions: make([]*BandwidthPrediction, 0),
		config:      config,
		stopCh:      make(chan struct{}),
	}
}

// IsRunning 检查引擎是否运行中
func (pe *PredictionEngine) IsRunning() bool {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.running
}

// GetSampleCount 获取采样数量
func (pe *PredictionEngine) GetSampleCount() int {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return len(pe.samples)
}

// GetPredictionCount 获取预测数量
func (pe *PredictionEngine) GetPredictionCount() int {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return len(pe.predictions)
}
