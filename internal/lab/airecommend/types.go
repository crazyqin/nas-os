package airecommend

import (
	"sync"
	"time"
)

// Config 推荐引擎配置.
type Config struct {
	CacheTTL    time.Duration `json:"cache_ttl"`
	MaxResults  int           `json:"max_results"`
	MinAccesses int           `json:"min_accesses"`
	DecayFactor float64       `json:"decay_factor"` // 时间衰减因子
	Weights     Weights       `json:"weights"`
}

// Weights 推荐算法权重.
type Weights struct {
	TimeDecay     float64 `json:"time_decay"`    // 时间衰减权重 0.3
	Frequency     float64 `json:"frequency"`     // 频率权重 0.3
	Collaborative float64 `json:"collaborative"` // 协同过滤权重 0.2
	Content       float64 `json:"content"`       // 内容相似度权重 0.2
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		CacheTTL:    30 * time.Minute,
		MaxResults:  20,
		MinAccesses: 3,
		DecayFactor: 0.95,
		Weights: Weights{
			TimeDecay:     0.3,
			Frequency:     0.3,
			Collaborative: 0.2,
			Content:       0.2,
		},
	}
}

// UserProfile 用户画像.
type UserProfile struct {
	UserID        string             `json:"user_id"`
	AccessHistory []AccessRecord     `json:"access_history"`
	Preferences   map[string]float64 `json:"preferences"` // 文件类型 -> 偏好分数
	LastActive    time.Time          `json:"last_active"`
}

// FileItem 文件信息.
type FileItem struct {
	FileID    string            `json:"file_id"`
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Type      string            `json:"type"` // 文件类型
	Size      int64             `json:"size"`
	Tags      []string          `json:"tags"`     // 文件标签
	Metadata  map[string]string `json:"metadata"` // 元数据
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// AccessRecord 访问记录.
type AccessRecord struct {
	UserID    string    `json:"user_id"`
	FileID    string    `json:"file_id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // view, edit, download
}

// Recommendation 推荐结果.
type Recommendation struct {
	FileID    string    `json:"file_id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Score     float64   `json:"score"`
	Reason    string    `json:"reason"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CacheEntry 缓存条目.
type CacheEntry struct {
	Recommendations []Recommendation `json:"recommendations"`
	ExpiresAt       time.Time        `json:"expires_at"`
}

// Engine 推荐引擎.
type Engine struct {
	config    *Config
	mu        sync.RWMutex
	users     map[string]*UserProfile // 用户ID -> 用户画像
	files     map[string]*FileItem    // 文件ID -> 文件信息
	cache     map[string]*CacheEntry  // 用户ID -> 缓存
	accessLog []AccessRecord          // 访问历史
}

// NewEngine 创建推荐引擎.
func NewEngine(config *Config) *Engine {
	if config == nil {
		config = DefaultConfig()
	}
	return &Engine{
		config: config,
		users:  make(map[string]*UserProfile),
		files:  make(map[string]*FileItem),
		cache:  make(map[string]*CacheEntry),
	}
}

// GetUserRecommendationsRequest 获取推荐请求.
type GetUserRecommendationsRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Limit  int    `json:"limit"`
}

// GetUserRecommendationsResponse 推荐响应.
type GetUserRecommendationsResponse struct {
	UserID          string           `json:"user_id"`
	Recommendations []Recommendation `json:"recommendations"`
	CachedAt        time.Time        `json:"cached_at"`
	ExpiresAt       time.Time        `json:"expires_at"`
}

// AddAccessRecordRequest 添加访问记录请求.
type AddAccessRecordRequest struct {
	UserID string `json:"user_id" binding:"required"`
	FileID string `json:"file_id" binding:"required"`
	Action string `json:"action" binding:"required"`
}

// AddFileRequest 添加文件请求.
type AddFileRequest struct {
	FileID   string            `json:"file_id" binding:"required"`
	Name     string            `json:"name" binding:"required"`
	Path     string            `json:"path"`
	Type     string            `json:"type"`
	Size     int64             `json:"size"`
	Tags     []string          `json:"tags"`
	Metadata map[string]string `json:"metadata"`
}

// AddUserRequest 添加用户请求.
type AddUserRequest struct {
	UserID string `json:"user_id" binding:"required"`
}
