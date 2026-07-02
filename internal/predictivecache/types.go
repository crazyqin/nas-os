package predictivecache

import "time"

// CacheRequest 缓存请求.
type CacheRequest struct {
	FilePath   string     `json:"file_path" binding:"required"`
	SizeBytes  int64      `json:"size_bytes"`
	CacheLevel CacheLevel `json:"cache_level"`
	Pin        bool       `json:"pin"`
}

// AccessRecordRequest 访问记录请求.
type AccessRecordRequest struct {
	FilePath  string `json:"file_path" binding:"required"`
	UserID    string `json:"user_id"`
	Operation string `json:"operation"`
	SizeBytes int64  `json:"size_bytes"`
	Duration  int    `json:"duration_ms"`
}

// WarmingRequest 预热请求.
type WarmingRequest struct {
	FilePath   string     `json:"file_path" binding:"required"`
	CacheLevel CacheLevel `json:"cache_level"`
}

// PredictionRequest 预测请求.
type PredictionRequest struct {
	FilePath string `json:"file_path" binding:"required"`
}

// PredictionResponse 预测响应.
type PredictionResponse struct {
	FilePath    string               `json:"file_path"`
	ShouldCache bool                 `json:"should_cache"`
	CacheLevel  CacheLevel           `json:"cache_level,omitempty"`
	Priority    int                  `json:"priority"`
	NextAccess  time.Time            `json:"next_access,omitempty"`
	Confidence  PredictionConfidence `json:"confidence"`
	Pattern     AccessPattern        `json:"pattern,omitempty"`
	Reason      string               `json:"reason"`
}

// CacheStatsResponse 缓存统计响应.
type CacheStatsResponse struct {
	TotalEntries int                   `json:"total_entries"`
	TotalHits    int64                 `json:"total_hits"`
	TotalMisses  int64                 `json:"total_misses"`
	HitRate      float64               `json:"hit_rate"`
	WarmingTasks int                   `json:"warming_tasks"`
	Levels       map[string]LevelStats `json:"levels"`
}

// LevelStats 层级统计.
type LevelStats struct {
	Count int   `json:"count"`
	Size  int64 `json:"size_bytes"`
}

// PolicyUpdateRequest 策略更新请求.
type PolicyUpdateRequest struct {
	MaxL1SizeGB     float64 `json:"max_l1_size_gb"`
	MaxL2SizeGB     float64 `json:"max_l2_size_gb"`
	MaxL3SizeGB     float64 `json:"max_l3_size_gb"`
	EvictionPolicy  string  `json:"eviction_policy"`
	TTLHours        int     `json:"ttl_hours"`
	AutoWarming     bool    `json:"auto_warming"`
	WarmingSchedule string  `json:"warming_schedule"`
}

// ModelUpdateRequest 模型更新请求.
type ModelUpdateRequest struct {
	WindowSize          int     `json:"window_size"`
	MinSamples          int     `json:"min_samples"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	DecayFactor         float64 `json:"decay_factor"`
	TrendWeight         float64 `json:"trend_weight"`
	SeasonWeight        float64 `json:"season_weight"`
}

// BulkWarmRequest 批量预热请求.
type BulkWarmRequest struct {
	FilePaths  []string   `json:"file_paths" binding:"required"`
	CacheLevel CacheLevel `json:"cache_level"`
}

// CacheEntryResponse 缓存条目响应.
type CacheEntryResponse struct {
	ID         string     `json:"id"`
	FilePath   string     `json:"file_path"`
	CacheLevel CacheLevel `json:"cache_level"`
	SizeBytes  int64      `json:"size_bytes"`
	Priority   int        `json:"priority"`
	HitCount   int        `json:"hit_count"`
	HitRate    float64    `json:"hit_rate"`
	LoadedAt   time.Time  `json:"loaded_at"`
	LastAccess time.Time  `json:"last_access"`
	ExpiresAt  time.Time  `json:"expires_at"`
	Pinned     bool       `json:"pinned"`
}
