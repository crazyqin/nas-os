// Package storageanomaly 提供 AI 存储异常检测功能
package storageanomaly

import (
	"time"
)

// AnomalyEvent 异常事件.
type AnomalyEvent struct {
	ID          string    `json:"id"`
	EventType   string    `json:"event_type"`   // write_spike, size_anomaly, access_pattern, data_leak, hw_failure, malware
	Severity    string    `json:"severity"`      // low, medium, high, critical
	Path        string    `json:"path"`          // 关联路径
	Description string    `json:"description"`
	Metric      float64   `json:"metric"`        // 异常指标值
	Baseline    float64   `json:"baseline"`      // 基线值
	Deviation   float64   `json:"deviation"`     // 偏差倍数
	Source      string    `json:"source"`        // 检测来源（规则名或算法名）
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Response    string    `json:"response"`      // 自动响应动作
}

// AnomalyRule 异常检测规则.
type AnomalyRule struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	EventType   string  `json:"event_type"`
	Enabled     bool    `json:"enabled"`
	Threshold   float64 `json:"threshold"`    // 偏差倍数阈值
	MinSamples  int     `json:"min_samples"`  // 最小样本数
	Description string  `json:"description"`
}

// StorageBaseline 存储基线数据.
type StorageBaseline struct {
	Path           string    `json:"path"`
	AvgWriteBytes  float64   `json:"avg_write_bytes"`  // 平均每次采样写入字节
	AvgReadBytes   float64   `json:"avg_read_bytes"`   // 平均每次采样读取字节
	AvgFileSize    float64   `json:"avg_file_size"`    // 平均文件数量
	AvgAccessFreq  float64   `json:"avg_access_freq"`  // 平均访问操作数
	StdWriteBytes  float64   `json:"std_write_bytes"`  // 写入字节标准差
	StdReadBytes   float64   `json:"std_read_bytes"`   // 读取字节标准差
	StdFileSize    float64   `json:"std_file_size"`    // 文件数量标准差
	StdAccessFreq  float64   `json:"std_access_freq"`  // 访问操作数标准差
	SampleCount    int       `json:"sample_count"`
	LastUpdated    time.Time `json:"last_updated"`
}

// DetectionConfig 检测配置.
type DetectionConfig struct {
	Enabled          bool    `json:"enabled"`
	ScanInterval     int     `json:"scan_interval"`     // seconds
	DeviationFactor  float64 `json:"deviation_factor"`  // 偏差倍数（默认 3.0 即 3σ）
	MinBaselineAge   int     `json:"min_baseline_age"`  // hours, 基线最短建立时间
	AutoRespond      bool    `json:"auto_respond"`      // 是否启用自动响应
	AlertThreshold   string  `json:"alert_threshold"`   // 触发告警的最低级别: low/medium/high/critical
	MaxEventsPerHour int     `json:"max_events_per_hour"`
}

// SampleDataPoint 采样数据点.
type SampleDataPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	WriteBytes int64     `json:"write_bytes"`
	ReadBytes  int64     `json:"read_bytes"`
	FileCount  int       `json:"file_count"`
	AccessOps  int       `json:"access_ops"`
}

// AnomalyStats 异常统计.
type AnomalyStats struct {
	TotalEvents    int            `json:"total_events"`
	BySeverity     map[string]int `json:"by_severity"`
	ByType         map[string]int `json:"by_type"`
	Unresolved     int            `json:"unresolved"`
	LastEventTime  *time.Time     `json:"last_event_time,omitempty"`
}

// AddRuleRequest 添加规则请求.
type AddRuleRequest struct {
	Name        string  `json:"name" binding:"required"`
	EventType   string  `json:"event_type" binding:"required"`
	Threshold   float64 `json:"threshold"`
	MinSamples  int     `json:"min_samples"`
	Description string  `json:"description"`
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	Enabled          *bool    `json:"enabled,omitempty"`
	ScanInterval     *int     `json:"scan_interval,omitempty"`
	DeviationFactor  *float64 `json:"deviation_factor,omitempty"`
	MinBaselineAge   *int     `json:"min_baseline_age,omitempty"`
	AutoRespond      *bool    `json:"auto_respond,omitempty"`
	AlertThreshold   *string  `json:"alert_threshold,omitempty"`
	MaxEventsPerHour *int     `json:"max_events_per_hour,omitempty"`
}

// IngestSampleRequest 导入采样数据请求.
type IngestSampleRequest struct {
	Path        string `json:"path" binding:"required"`
	WriteBytes  int64  `json:"write_bytes"`
	ReadBytes   int64  `json:"read_bytes"`
	FileCount   int    `json:"file_count"`
	AccessOps   int    `json:"access_ops"`
}
