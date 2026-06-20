// Package smartcacheprefetch 智能缓存预取模块
// 基于ML的读取模式预测，NVMe高速缓存预热，多级缓存管理
// 对标TrueNAS智能缓存预取策略
package smartcacheprefetch

import (
	"errors"
	"time"
)

// CacheLayerType 缓存层类型
type CacheLayerType int

const (
	CacheLayerNVMe CacheLayerType = iota // L1 NVMe缓存
	CacheLayerSSD                        // L2 SSD缓存
	CacheLayerHDD                        // L3 HDD缓存
)

// String returns human-readable layer type
func (t CacheLayerType) String() string {
	switch t {
	case CacheLayerNVMe:
		return "NVMe"
	case CacheLayerSSD:
		return "SSD"
	case CacheLayerHDD:
		return "HDD"
	default:
		return "Unknown"
	}
}

// EvictionPolicy 缓存淘汰策略
type EvictionPolicy int

const (
	EvictionLRU     EvictionPolicy = iota // 最近最少使用
	EvictionLFU                           // 最不经常使用
	EvictionARC                           // 自适应替换缓存
	EvictionMLAware                       // ML感知淘汰
)

// String returns human-readable eviction policy
func (p EvictionPolicy) String() string {
	switch p {
	case EvictionLRU:
		return "LRU"
	case EvictionLFU:
		return "LFU"
	case EvictionARC:
		return "ARC"
	case EvictionMLAware:
		return "ML_AWARE"
	default:
		return "Unknown"
	}
}

// PrefetchStrategy 预取策略
type PrefetchStrategy int

const (
	PrefetchSequential  PrefetchStrategy = iota // 顺序预取
	PrefetchPredictive                          // 预测性预取
	PrefetchAdaptive                            // 自适应预取
)

// String returns human-readable prefetch strategy
func (s PrefetchStrategy) String() string {
	switch s {
	case PrefetchSequential:
		return "Sequential"
	case PrefetchPredictive:
		return "Predictive"
	case PrefetchAdaptive:
		return "Adaptive"
	default:
		return "Unknown"
	}
}

// TaskStatus 预取任务状态
type TaskStatus int

const (
	TaskPending   TaskStatus = iota // 等待执行
	TaskRunning                     // 执行中
	TaskCompleted                   // 已完成
	TaskFailed                      // 失败
)

// String returns human-readable task status
func (s TaskStatus) String() string {
	switch s {
	case TaskPending:
		return "Pending"
	case TaskRunning:
		return "Running"
	case TaskCompleted:
		return "Completed"
	case TaskFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// CacheLayer 缓存层
type CacheLayer struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Type       CacheLayerType `json:"type"`
	Capacity   int64          `json:"capacity"` // 字节
	Used       int64          `json:"used"`
	Entries    map[string]*CacheEntry `json:"entries"`
	Policy     EvictionPolicy `json:"policy"`
	HitRate    float64        `json:"hit_rate"`
	AvgLatency time.Duration  `json:"avg_latency"`
	CreatedAt  time.Time      `json:"created_at"`
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key         string        `json:"key"`
	Path        string        `json:"path"`
	Size        int64         `json:"size"`
	Layer       string        `json:"layer"`
	AccessCount int64         `json:"access_count"`
	LastAccess  time.Time     `json:"last_access"`
	CreatedAt   time.Time     `json:"created_at"`
	Priority    float64       `json:"priority"` // 基于ML的优先级评分
	TTL         time.Duration `json:"ttl"`
	Checksum    string        `json:"checksum"`
}

// AccessPattern 访问模式
type AccessPattern struct {
	FileID      string      `json:"file_id"`
	AccessTimes []time.Time `json:"access_times"`
	ReadSizes   []int64     `json:"read_sizes"`
	Sequential  float64     `json:"sequential"`   // 顺序读取概率
	Random      float64     `json:"random"`       // 随机读取概率
	Periodic    float64     `json:"periodic"`     // 周期性访问概率
	Frequency   float64     `json:"frequency"`    // 访问频率
	LastUpdated time.Time   `json:"last_updated"`
}

// Prediction 预取预测
type Prediction struct {
	FileID       string          `json:"file_id"`
	Probability  float64         `json:"probability"`
	ExpectedTime time.Time       `json:"expected_time"`
	ExpectedSize int64           `json:"expected_size"`
	Confidence   float64         `json:"confidence"`
	Strategy     PrefetchStrategy `json:"strategy"`
}

// PrefetchTask 预取任务
type PrefetchTask struct {
	ID          string        `json:"id"`
	SourcePath  string        `json:"source_path"`
	TargetLayer string        `json:"target_layer"`
	Size        int64         `json:"size"`
	Priority    float64       `json:"priority"`
	Deadline    time.Time     `json:"deadline"`
	Status      TaskStatus    `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
}

// AccessPredictor ML访问预测器
type AccessPredictor struct {
	weights     map[string]float64 `json:"weights"`
	features    []string           `json:"features"`
	accuracy    float64            `json:"accuracy"`
	trainedAt   time.Time          `json:"trained_at"`
	predictions int64              `json:"predictions"`
	hits        int64              `json:"hits"`
}

// PrefetchMetrics 预取指标
type PrefetchMetrics struct {
	TotalPrefetches int64                    `json:"total_prefetches"`
	SuccessfulHits  int64                    `json:"successful_hits"`
	Misses          int64                    `json:"misses"`
	HitRate         float64                  `json:"hit_rate"`
	BytesSaved      int64                    `json:"bytes_saved"`
	AvgPrefetchTime time.Duration            `json:"avg_prefetch_time"`
	CacheEfficiency float64                  `json:"cache_efficiency"`
	LayerStats      map[string]*LayerStats   `json:"layer_stats"`
}

// LayerStats 缓存层统计
type LayerStats struct {
	HitCount   int64         `json:"hit_count"`
	MissCount  int64         `json:"miss_count"`
	HitRate    float64       `json:"hit_rate"`
	AvgLatency time.Duration `json:"avg_latency"`
	Evictions  int64         `json:"evictions"`
}

// PrefetchConfig 预取配置
type PrefetchConfig struct {
	MaxCacheSize       int64         `json:"max_cache_size"`
	PrefetchWindow     time.Duration `json:"prefetch_window"`
	MaxQueueSize       int           `json:"max_queue_size"`
	MLModelEnabled     bool          `json:"ml_model_enabled"`
	AggressivePrefetch bool          `json:"aggressive_prefetch"`
	LayerConfigs       []LayerConfig `json:"layer_configs"`
}

// LayerConfig 缓存层配置
type LayerConfig struct {
	ID       string         `json:"id"`
	Type     CacheLayerType `json:"type"`
	Capacity int64          `json:"capacity"`
	Policy   EvictionPolicy `json:"policy"`
}

// DefaultPrefetchConfig returns default prefetch configuration
func DefaultPrefetchConfig() PrefetchConfig {
	return PrefetchConfig{
		MaxCacheSize:       1024 * 1024 * 1024 * 100, // 100GB
		PrefetchWindow:     30 * time.Second,
		MaxQueueSize:       1024,
		MLModelEnabled:     true,
		AggressivePrefetch: false,
		LayerConfigs: []LayerConfig{
			{ID: "l1-nvme", Type: CacheLayerNVMe, Capacity: 1024 * 1024 * 1024 * 10, Policy: EvictionMLAware},
			{ID: "l2-ssd", Type: CacheLayerSSD, Capacity: 1024 * 1024 * 1024 * 50, Policy: EvictionARC},
			{ID: "l3-hdd", Type: CacheLayerHDD, Capacity: 1024 * 1024 * 1024 * 40, Policy: EvictionLFU},
		},
	}
}

// 预定义错误
var (
	ErrCacheLayerNotFound   = errors.New("cache layer not found")
	ErrCacheLayerExists     = errors.New("cache layer already exists")
	ErrCacheFull            = errors.New("cache layer is full")
	ErrEntryNotFound        = errors.New("cache entry not found")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrEngineNotRunning     = errors.New("prefetch engine is not running")
	ErrEngineAlreadyRunning = errors.New("prefetch engine is already running")
	ErrQueueFull            = errors.New("prefetch queue is full")
	ErrPredictionFailed     = errors.New("prediction failed")
	ErrMLModelNotTrained    = errors.New("ML model not trained")
	ErrPrefetchFailed       = errors.New("prefetch operation failed")
	ErrFileNotFound         = errors.New("file not found")
)
