package aiconsoledatamask

import (
	"sync"
	"time"
)

// MaskType 脱敏类型
type MaskType string

const (
	MaskTypeEmail    MaskType = "email"
	MaskTypePhone    MaskType = "phone"
	MaskTypeIDCard   MaskType = "idcard"
	MaskTypeCreditCard MaskType = "creditcard"
	MaskTypeName     MaskType = "name"
	MaskTypeAddress  MaskType = "address"
	MaskTypeCustom   MaskType = "custom"
)

// MaskStrategy 脱敏策略
type MaskStrategy string

const (
	StrategyPartial  MaskStrategy = "partial"  // 部分遮挡
	StrategyReplace  MaskStrategy = "replace"  // 替换为*
	StrategyHash     MaskStrategy = "hash"     // 哈希处理
	StrategyEncrypt  MaskStrategy = "encrypt"  // 加密
	StrategyRemove   MaskStrategy = "remove"   // 移除
)

// SensitivePattern 敏感信息模式
type SensitivePattern struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      MaskType  `json:"type"`
	Pattern   string    `json:"pattern"`   // 正则表达式
	Strategy  MaskStrategy `json:"strategy"`
	Enabled   bool      `json:"enabled"`
	Priority  int       `json:"priority"`  // 优先级
	CreatedAt time.Time `json:"createdAt"`
}

// MaskRule 脱敏规则
type MaskRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Patterns    []string      `json:"patterns"`    // 模式ID列表
	ApplyTo     []string      `json:"applyTo"`     // 应用场景
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// MaskResult 脱敏结果
type MaskResult struct {
	OriginalLength  int       `json:"originalLength"`
	MaskedLength    int       `json:"maskedLength"`
	MaskedText      string    `json:"maskedText"`
	DetectedTypes   []MaskType `json:"detectedTypes"`
	MaskedCount     int       `json:"maskedCount"`
	ProcessingTime  time.Duration `json:"processingTime"`
}

// MaskStats 脱敏统计
type MaskStats struct {
	mu              sync.RWMutex
	TotalRequests   int64     `json:"totalRequests"`
	TotalMasked     int64     `json:"totalMasked"`
	ByType          map[MaskType]int64 `json:"byType"`
	ByStrategy      map[MaskStrategy]int64 `json:"byStrategy"`
	AvgProcessTime  time.Duration `json:"avgProcessTime"`
	LastUpdated     time.Time `json:"lastUpdated"`
}

// DataMaskConfig 脱敏配置
type DataMaskConfig struct {
	Enabled         bool          `json:"enabled"`
	DefaultStrategy MaskStrategy  `json:"defaultStrategy"`
	MaxInputLength  int           `json:"maxInputLength"`
	CacheEnabled    bool          `json:"cacheEnabled"`
	CacheTTL        time.Duration `json:"cacheTTL"`
	LogLevel        string        `json:"logLevel"`
}

// DefaultDataMaskConfig 默认配置
func DefaultDataMaskConfig() *DataMaskConfig {
	return &DataMaskConfig{
		Enabled:         true,
		DefaultStrategy: StrategyPartial,
		MaxInputLength:  10000,
		CacheEnabled:    true,
		CacheTTL:        5 * time.Minute,
		LogLevel:        "info",
	}
}

// DataMaskEngine 脱敏引擎
type DataMaskEngine struct {
	mu        sync.RWMutex
	config    *DataMaskConfig
	patterns  map[string]*SensitivePattern
	rules     map[string]*MaskRule
	stats     *MaskStats
	cache     map[string]*MaskResult
	cacheMu   sync.RWMutex
	running   bool
	stopCh    chan struct{}
}

// MaskRequest 脱敏请求
type MaskRequest struct {
	Text      string   `json:"text"`
	Context   string   `json:"context"`   // 上下文（如API名称）
	RuleIDs   []string `json:"ruleIds"`   // 指定规则
	Types     []MaskType `json:"types"`   // 指定类型
	Strategy  MaskStrategy `json:"strategy"` // 指定策略
}

// BatchMaskRequest 批量脱敏请求
type BatchMaskRequest struct {
	Requests []MaskRequest `json:"requests"`
}

// BatchMaskResult 批量脱敏结果
type BatchMaskResult struct {
	Results []MaskResult `json:"results"`
	Total   int          `json:"total"`
	Success int          `json:"success"`
	Failed  int          `json:"failed"`
}
