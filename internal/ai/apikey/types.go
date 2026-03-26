// Package apikey provides secure API key management for AI services
// Implements encryption at rest, access auditing, and RBAC integration
package apikey

import (
	"time"
)

// ProviderType represents an AI service provider
type ProviderType string

const (
	// ProviderOpenAI - OpenAI API
	ProviderOpenAI ProviderType = "openai"
	// ProviderAzure - Azure OpenAI
	ProviderAzure ProviderType = "azure"
	// ProviderAnthropic - Anthropic Claude
	ProviderAnthropic ProviderType = "anthropic"
	// ProviderGoogle - Google AI
	ProviderGoogle ProviderType = "google"
	// ProviderCohere - Cohere
	ProviderCohere ProviderType = "cohere"
	// ProviderMistral - Mistral AI
	ProviderMistral ProviderType = "mistral"
	// ProviderDeepSeek - DeepSeek
	ProviderDeepSeek ProviderType = "deepseek"
	// ProviderMoonshot - Moonshot Kimi
	ProviderMoonshot ProviderType = "moonshot"
	// ProviderZhipu - 智谱AI
	ProviderZhipu ProviderType = "zhipu"
	// ProviderBaidu - 百度文心
	ProviderBaidu ProviderType = "baidu"
	// ProviderAlibaba - 阿里通义
	ProviderAlibaba ProviderType = "alibaba"
	// ProviderTencent - 腾讯混元
	ProviderTencent ProviderType = "tencent"
	// ProviderCustom - 自定义服务
	ProviderCustom ProviderType = "custom"
	// ProviderLocal - 本地服务 (Ollama等)
	ProviderLocal ProviderType = "local"
)

// ProviderInfo contains provider metadata
type ProviderInfo struct {
	Type          ProviderType `json:"type"`
	Name          string       `json:"name"`
	APIEndpoint   string       `json:"api_endpoint"`
	AuthHeader    string       `json:"auth_header"` // Authorization header format
	KeyPrefix     string       `json:"key_prefix"`  // API key prefix for validation
	KeyPattern    string       `json:"key_pattern"` // Regex pattern for key validation
	RequiresKey   bool         `json:"requires_key"`
	SupportsOAuth bool         `json:"supports_oauth"`
	RegionBased   bool         `json:"region_based"` // e.g., Azure requires region
}

// DefaultProviders returns predefined provider configurations
var DefaultProviders = map[ProviderType]ProviderInfo{
	ProviderOpenAI: {
		Type:        ProviderOpenAI,
		Name:        "OpenAI",
		APIEndpoint: "https://api.openai.com/v1",
		AuthHeader:  "Bearer",
		KeyPrefix:   "sk-",
		KeyPattern:  `^sk-[a-zA-Z0-9]{20,}$`,
		RequiresKey: true,
	},
	ProviderAzure: {
		Type:        ProviderAzure,
		Name:        "Azure OpenAI",
		APIEndpoint: "https://YOUR_RESOURCE.openai.azure.com",
		AuthHeader:  "api-key",
		KeyPrefix:   "",
		KeyPattern:  `^[a-f0-9]{32}$`,
		RequiresKey: true,
		RegionBased: true,
	},
	ProviderAnthropic: {
		Type:        ProviderAnthropic,
		Name:        "Anthropic",
		APIEndpoint: "https://api.anthropic.com/v1",
		AuthHeader:  "x-api-key",
		KeyPrefix:   "sk-ant-",
		KeyPattern:  `^sk-ant-[a-zA-Z0-9-]{20,}$`,
		RequiresKey: true,
	},
	ProviderGoogle: {
		Type:          ProviderGoogle,
		Name:          "Google AI",
		APIEndpoint:   "https://generativelanguage.googleapis.com/v1",
		AuthHeader:    "Bearer",
		RequiresKey:   true,
		SupportsOAuth: true,
	},
	ProviderCohere: {
		Type:        ProviderCohere,
		Name:        "Cohere",
		APIEndpoint: "https://api.cohere.ai/v1",
		AuthHeader:  "Bearer",
		KeyPrefix:   "",
		RequiresKey: true,
	},
	ProviderMistral: {
		Type:        ProviderMistral,
		Name:        "Mistral AI",
		APIEndpoint: "https://api.mistral.ai/v1",
		AuthHeader:  "Bearer",
		KeyPrefix:   "",
		RequiresKey: true,
	},
	ProviderDeepSeek: {
		Type:        ProviderDeepSeek,
		Name:        "DeepSeek",
		APIEndpoint: "https://api.deepseek.com/v1",
		AuthHeader:  "Bearer",
		KeyPrefix:   "sk-",
		KeyPattern:  `^sk-[a-zA-Z0-9]{20,}$`,
		RequiresKey: true,
	},
	ProviderMoonshot: {
		Type:        ProviderMoonshot,
		Name:        "Moonshot AI",
		APIEndpoint: "https://api.moonshot.cn/v1",
		AuthHeader:  "Bearer",
		KeyPrefix:   "sk-",
		RequiresKey: true,
	},
	ProviderZhipu: {
		Type:        ProviderZhipu,
		Name:        "Zhipu AI",
		APIEndpoint: "https://open.bigmodel.cn/api/paas/v4",
		AuthHeader:  "Bearer",
		RequiresKey: true,
	},
	ProviderBaidu: {
		Type:        ProviderBaidu,
		Name:        "Baidu AI",
		APIEndpoint: "https://aip.baidubce.com/rpc/2.0/ai_custom/v1",
		AuthHeader:  "Bearer",
		RequiresKey: true,
	},
	ProviderAlibaba: {
		Type:        ProviderAlibaba,
		Name:        "Alibaba Qwen",
		APIEndpoint: "https://dashscope.aliyuncs.com/api/v1",
		AuthHeader:  "Bearer",
		RequiresKey: true,
	},
	ProviderTencent: {
		Type:        ProviderTencent,
		Name:        "Tencent AI",
		APIEndpoint: "https://api.hunyuan.cloud.tencent.com/v1",
		AuthHeader:  "Bearer",
		RequiresKey: true,
	},
	ProviderLocal: {
		Type:        ProviderLocal,
		Name:        "Local AI Service",
		APIEndpoint: "http://localhost:11434",
		AuthHeader:  "",
		RequiresKey: false,
	},
}

// KeyStatus represents the status of an API key
type KeyStatus string

const (
	// KeyStatusActive - 密钥可用
	KeyStatusActive KeyStatus = "active"
	// KeyStatusDisabled - 密钥已禁用
	KeyStatusDisabled KeyStatus = "disabled"
	// KeyStatusExpired - 密钥已过期
	KeyStatusExpired KeyStatus = "expired"
	// KeyStatusRevoked - 密钥已吊销
	KeyStatusRevoked KeyStatus = "revoked"
	// KeyStatusRotating - 密钥轮换中
	KeyStatusRotating KeyStatus = "rotating"
)

// KeyType represents the type of API key
type KeyType string

const (
	// KeyTypeAPIKey - 标准API密钥
	KeyTypeAPIKey KeyType = "api_key"
	// KeyTypeOAuth - OAuth令牌
	KeyTypeOAuth KeyType = "oauth"
	// KeyTypeService - 服务账户密钥
	KeyTypeService KeyType = "service"
	// KeyTypeProject - 项目专用密钥
	KeyTypeProject KeyType = "project"
	// KeyTypeTemporary - 临时/过期密钥
	KeyTypeTemporary KeyType = "temporary"
)

// APIKey represents a stored API key with metadata
type APIKey struct {
	ID          string       `json:"id"`
	Provider    ProviderType `json:"provider"`
	KeyType     KeyType      `json:"key_type"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`

	// Encrypted key storage
	EncryptedKey []byte `json:"encrypted_key"`
	KeyHash      string `json:"key_hash"`    // SHA256 hash for verification
	KeyPreview   string `json:"key_preview"` // Last 4 chars for display

	// Status and validity
	Status     KeyStatus  `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`

	// Usage tracking
	UsageCount   int64   `json:"usage_count"`
	UsageLimit   int64   `json:"usage_limit,omitempty"` // Monthly usage limit
	CurrentUsage int64   `json:"current_usage"`         // Current month usage
	CostLimit    float64 `json:"cost_limit,omitempty"`  // Cost limit in USD
	CurrentCost  float64 `json:"current_cost"`          // Current month cost

	// Access control
	OwnerID       string   `json:"owner_id"`                 // User who owns this key
	OwnerType     string   `json:"owner_type"`               // "user" or "service"
	AllowedUsers  []string `json:"allowed_users,omitempty"`  // Users allowed to use this key
	AllowedGroups []string `json:"allowed_groups,omitempty"` // Groups allowed to use this key

	// Scope restrictions
	AllowedModels    []string `json:"allowed_models,omitempty"`    // Models this key can access
	AllowedEndpoints []string `json:"allowed_endpoints,omitempty"` // API endpoints allowed

	// Rate limiting
	RateLimit  int `json:"rate_limit,omitempty"`  // Requests per minute
	BurstLimit int `json:"burst_limit,omitempty"` // Burst allowance

	// Custom configuration
	Endpoint string            `json:"endpoint,omitempty"` // Custom endpoint override
	Headers  map[string]string `json:"headers,omitempty"`  // Additional headers
	Metadata map[string]any    `json:"metadata,omitempty"` // Custom metadata

	// Audit and security
	CreatedBy     string     `json:"created_by"`
	RotationDays  int        `json:"rotation_days,omitempty"` // Auto-rotation interval
	LastRotatedAt *time.Time `json:"last_rotated_at,omitempty"`
	Version       int        `json:"version"` // Key version for rotation
}

// KeyCreateRequest represents a request to create a new API key
type KeyCreateRequest struct {
	Provider      ProviderType      `json:"provider" binding:"required"`
	KeyType       KeyType           `json:"key_type"`
	Name          string            `json:"name" binding:"required"`
	Description   string            `json:"description,omitempty"`
	APIKey        string            `json:"api_key" binding:"required"` // Plain text, will be encrypted
	Endpoint      string            `json:"endpoint,omitempty"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	UsageLimit    int64             `json:"usage_limit,omitempty"`
	CostLimit     float64           `json:"cost_limit,omitempty"`
	AllowedUsers  []string          `json:"allowed_users,omitempty"`
	AllowedGroups []string          `json:"allowed_groups,omitempty"`
	AllowedModels []string          `json:"allowed_models,omitempty"`
	RateLimit     int               `json:"rate_limit,omitempty"`
	RotationDays  int               `json:"rotation_days,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// KeyUpdateRequest represents a request to update an API key
type KeyUpdateRequest struct {
	Name          *string            `json:"name,omitempty"`
	Description   *string            `json:"description,omitempty"`
	Status        *KeyStatus         `json:"status,omitempty"`
	UsageLimit    *int64             `json:"usage_limit,omitempty"`
	CostLimit     *float64           `json:"cost_limit,omitempty"`
	AllowedUsers  *[]string          `json:"allowed_users,omitempty"`
	AllowedGroups *[]string          `json:"allowed_groups,omitempty"`
	AllowedModels *[]string          `json:"allowed_models,omitempty"`
	RateLimit     *int               `json:"rate_limit,omitempty"`
	RotationDays  *int               `json:"rotation_days,omitempty"`
	Headers       *map[string]string `json:"headers,omitempty"`
	Metadata      *map[string]any    `json:"metadata,omitempty"`
}

// KeyUseRequest represents a request to use an API key
type KeyUseRequest struct {
	KeyID       string       `json:"key_id,omitempty"`
	Provider    ProviderType `json:"provider,omitempty"`
	UserID      string       `json:"user_id"`
	Model       string       `json:"model,omitempty"`
	Endpoint    string       `json:"endpoint,omitempty"`
	RequestSize int          `json:"request_size,omitempty"` // For rate limiting
}

// KeyUseResult represents the result of a key use request
type KeyUseResult struct {
	KeyID        string            `json:"key_id"`
	Provider     ProviderType      `json:"provider"`
	DecryptedKey string            `json:"decrypted_key"` // Only returned to authorized callers
	Endpoint     string            `json:"endpoint"`
	Headers      map[string]string `json:"headers,omitempty"`
	Allowed      bool              `json:"allowed"`
	Reason       string            `json:"reason,omitempty"`
}

// KeyRotationRequest represents a request to rotate an API key
type KeyRotationRequest struct {
	KeyID     string `json:"key_id" binding:"required"`
	NewAPIKey string `json:"new_api_key" binding:"required"`
	RevokeOld bool   `json:"revoke_old"` // Immediately revoke old key
}

// KeyFilter represents filter options for listing keys
type KeyFilter struct {
	Provider    *ProviderType `json:"provider,omitempty"`
	Status      *KeyStatus    `json:"status,omitempty"`
	KeyType     *KeyType      `json:"key_type,omitempty"`
	OwnerID     *string       `json:"owner_id,omitempty"`
	OwnerType   *string       `json:"owner_type,omitempty"`
	Model       *string       `json:"model,omitempty"`        // Keys that can access this model
	SearchQuery *string       `json:"search_query,omitempty"` // Search in name/description
}

// KeyListResult represents a paginated list of keys
type KeyListResult struct {
	Keys       []APIKey `json:"keys"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	TotalPages int      `json:"total_pages"`
}

// UsageStats represents usage statistics for an API key
type UsageStats struct {
	KeyID         string           `json:"key_id"`
	Provider      ProviderType     `json:"provider"`
	Period        string           `json:"period"` // e.g., "2024-01"
	TotalRequests int64            `json:"total_requests"`
	TotalTokens   int64            `json:"total_tokens"`
	TotalCost     float64          `json:"total_cost"`
	AvgLatencyMs  float64          `json:"avg_latency_ms"`
	ErrorCount    int64            `json:"error_count"`
	ByModel       map[string]int64 `json:"by_model"`
	ByEndpoint    map[string]int64 `json:"by_endpoint"`
	ByUser        map[string]int64 `json:"by_user"`
	DailyUsage    []DailyUsage     `json:"daily_usage"`
}

// DailyUsage represents daily usage data
type DailyUsage struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// AuditEventType represents types of key-related audit events
type AuditEventType string

const (
	// AuditEventKeyCreate - 密钥创建
	AuditEventKeyCreate AuditEventType = "key_create"
	// AuditEventKeyUpdate - 密钥更新
	AuditEventKeyUpdate AuditEventType = "key_update"
	// AuditEventKeyDelete - 密钥删除
	AuditEventKeyDelete AuditEventType = "key_delete"
	// AuditEventKeyAccess - 密钥访问
	AuditEventKeyAccess AuditEventType = "key_access"
	// AuditEventKeyRotate - 密钥轮换
	AuditEventKeyRotate AuditEventType = "key_rotate"
	// AuditEventKeyRevoke - 密钥吊销
	AuditEventKeyRevoke AuditEventType = "key_revoke"
	// AuditEventKeyUse - 密钥使用
	AuditEventKeyUse AuditEventType = "key_use"
	// AuditEventKeyDeny - 密钥拒绝
	AuditEventKeyDeny AuditEventType = "key_deny"
	// AuditEventLimitReached - 限额达到
	AuditEventLimitReached AuditEventType = "limit_reached"
	// AuditEventKeyExpired - 密钥过期
	AuditEventKeyExpired AuditEventType = "key_expired"
)

// KeyAuditLog represents an audit log entry for API key operations
type KeyAuditLog struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	EventType AuditEventType `json:"event_type"`
	KeyID     string         `json:"key_id"`
	KeyName   string         `json:"key_name,omitempty"`
	Provider  ProviderType   `json:"provider"`
	UserID    string         `json:"user_id"`
	Username  string         `json:"username,omitempty"`
	Action    string         `json:"action"`
	Resource  string         `json:"resource,omitempty"`
	Result    string         `json:"result"` // success, denied, error
	Reason    string         `json:"reason,omitempty"`
	OldValue  any            `json:"old_value,omitempty"`
	NewValue  any            `json:"new_value,omitempty"`
	ClientIP  string         `json:"client_ip,omitempty"`
	UserAgent string         `json:"user_agent,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// KeyPermission represents permissions for API key operations
type KeyPermission string

const (
	// PermKeyCreate - 创建密钥权限
	PermKeyCreate KeyPermission = "ai:key:create"
	// PermKeyRead - 读取密钥权限
	PermKeyRead KeyPermission = "ai:key:read"
	// PermKeyUpdate - 更新密钥权限
	PermKeyUpdate KeyPermission = "ai:key:update"
	// PermKeyDelete - 删除密钥权限
	PermKeyDelete KeyPermission = "ai:key:delete"
	// PermKeyUse - 使用密钥权限
	PermKeyUse KeyPermission = "ai:key:use"
	// PermKeyRotate - 轮换密钥权限
	PermKeyRotate KeyPermission = "ai:key:rotate"
	// PermKeyAudit - 审计密钥权限
	PermKeyAudit KeyPermission = "ai:key:audit"
	// PermKeyAdmin - 完全管理权限
	PermKeyAdmin KeyPermission = "ai:key:admin"
	// PermKeyViewSecret - 查看解密密钥权限
	PermKeyViewSecret KeyPermission = "ai:key:view_secret"
)

// AllKeyPermissions returns all key-related permissions
func AllKeyPermissions() []KeyPermission {
	return []KeyPermission{
		PermKeyCreate,
		PermKeyRead,
		PermKeyUpdate,
		PermKeyDelete,
		PermKeyUse,
		PermKeyRotate,
		PermKeyAudit,
		PermKeyAdmin,
		PermKeyViewSecret,
	}
}

// Errors
var (
	ErrKeyNotFound       = "API key not found"
	ErrKeyExists         = "API key already exists"
	ErrKeyInvalid        = "Invalid API key format"
	ErrKeyExpired        = "API key has expired"
	ErrKeyDisabled       = "API key is disabled"
	ErrKeyRevoked        = "API key has been revoked"
	ErrKeyLimitReached   = "Usage limit reached"
	ErrKeyCostLimit      = "Cost limit reached"
	ErrAccessDenied      = "Access denied to API key"
	ErrProviderNotFound  = "Provider not found"
	ErrEncryptionFailed  = "Failed to encrypt key"
	ErrDecryptionFailed  = "Failed to decrypt key"
	ErrRateLimitExceeded = "Rate limit exceeded"
	ErrInvalidKeyFormat  = "Invalid key format for provider"
)
