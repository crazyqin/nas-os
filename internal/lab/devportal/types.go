// Package devportal 开发者门户 - API文档/密钥/Webhook/SDK/OAuth2管理
package devportal

import (
	"errors"
	"time"
)

// APIKeyStatus API密钥状态.
type APIKeyStatus string

const (
	KeyActive   APIKeyStatus = "active"
	KeyRevoked  APIKeyStatus = "revoked"
	KeyExpired  APIKeyStatus = "expired"
	KeyDisabled APIKeyStatus = "disabled"
)

// WebhookStatus Webhook状态.
type WebhookStatus string

const (
	WebhookActive   WebhookStatus = "active"
	WebhookInactive WebhookStatus = "inactive"
	WebhookFailed   WebhookStatus = "failed"
)

// WebhookEvent Webhook事件类型.
type WebhookEvent string

const (
	EventServiceStart   WebhookEvent = "service.start"
	EventServiceStop    WebhookEvent = "service.stop"
	EventServiceError   WebhookEvent = "service.error"
	EventAlertTrigger   WebhookEvent = "alert.trigger"
	EventUserCreated    WebhookEvent = "user.created"
	EventDataUploaded   WebhookEvent = "data.uploaded"
	EventBackupComplete WebhookEvent = "backup.complete"
)

// DeliveryStatus 投递状态.
type DeliveryStatus string

const (
	DeliveryPending  DeliveryStatus = "pending"
	DeliverySuccess  DeliveryStatus = "success"
	DeliveryFailed   DeliveryStatus = "failed"
	DeliveryRetrying DeliveryStatus = "retrying"
)

// AppStatus 应用状态.
type AppStatus string

const (
	AppPending  AppStatus = "pending"
	AppApproved AppStatus = "approved"
	AppRejected AppStatus = "rejected"
	AppDisabled AppStatus = "disabled"
)

// OAuthGrantType OAuth2授权类型.
type OAuthGrantType string

const (
	GrantAuthCode   OAuthGrantType = "authorization_code"
	GrantClientCred OAuthGrantType = "client_credentials"
	GrantRefresh    OAuthGrantType = "refresh_token"
)

// SDKLanguage SDK语言.
type SDKLanguage string

const (
	SDKPython     SDKLanguage = "python"
	SDKGo         SDKLanguage = "go"
	SDKJavaScript SDKLanguage = "javascript"
)

// APIScope API权限范围.
type APIScope string

const (
	ScopeRead      APIScope = "read"
	ScopeWrite     APIScope = "write"
	ScopeAdmin     APIScope = "admin"
	ScopeDelete    APIScope = "delete"
	ScopeWebhook   APIScope = "webhook"
	ScopeDevPortal APIScope = "devportal"
)

// APIKey API密钥.
type APIKey struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Key         string       `json:"key"`
	Secret      string       `json:"secret,omitempty"`
	OwnerID     string       `json:"owner_id"`
	Status      APIKeyStatus `json:"status"`
	Scopes      []APIScope   `json:"scopes"`
	RateLimit   int          `json:"rate_limit"`  // 每分钟请求数
	DailyQuota  int          `json:"daily_quota"` // 每日配额
	UsedToday   int          `json:"used_today"`
	TotalCalls  int64        `json:"total_calls"`
	LastUsedAt  *time.Time   `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	Description string       `json:"description,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// WebhookEndpoint Webhook端点.
type WebhookEndpoint struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	OwnerID         string            `json:"owner_id"`
	Status          WebhookStatus     `json:"status"`
	Events          []WebhookEvent    `json:"events"`
	Secret          string            `json:"secret"`
	Headers         map[string]string `json:"headers,omitempty"`
	RetryCount      int               `json:"retry_count"`
	MaxRetries      int               `json:"max_retries"`
	Timeout         int               `json:"timeout"` // 秒
	TotalSent       int64             `json:"total_sent"`
	TotalFailed     int64             `json:"total_failed"`
	LastDeliveredAt *time.Time        `json:"last_delivered_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// WebhookDelivery Webhook投递记录.
type WebhookDelivery struct {
	ID           string         `json:"id"`
	WebhookID    string         `json:"webhook_id"`
	Event        WebhookEvent   `json:"event"`
	URL          string         `json:"url"`
	StatusCode   int            `json:"status_code"`
	RequestBody  string         `json:"request_body,omitempty"`
	ResponseBody string         `json:"response_body,omitempty"`
	Attempts     int            `json:"attempts"`
	Status       DeliveryStatus `json:"status"`
	Error        string         `json:"error,omitempty"`
	Duration     int64          `json:"duration_ms"`
	CreatedAt    time.Time      `json:"created_at"`
	NextRetryAt  *time.Time     `json:"next_retry_at,omitempty"`
}

// DeveloperApp 开发者应用.
type DeveloperApp struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	OwnerID      string           `json:"owner_id"`
	ClientID     string           `json:"client_id"`
	ClientSecret string           `json:"client_secret"`
	RedirectURIs []string         `json:"redirect_uris"`
	GrantTypes   []OAuthGrantType `json:"grant_types"`
	Scopes       []APIScope       `json:"scopes"`
	Status       AppStatus        `json:"status"`
	Website      string           `json:"website,omitempty"`
	LogoURL      string           `json:"logo_url,omitempty"`
	APIKeyID     string           `json:"api_key_id,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// OAuthToken OAuth2令牌.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope"`
	AppID        string    `json:"app_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// UsageRecord 使用量记录.
type UsageRecord struct {
	Date       string `json:"date"` // YYYY-MM-DD
	Total      int    `json:"total"`
	Success    int    `json:"success"`
	Failed     int    `json:"failed"`
	AvgLatency int64  `json:"avg_latency_ms"`
}

// QuotaConfig 配额配置.
type QuotaConfig struct {
	DefaultRateLimit  int `json:"default_rate_limit"`  // 默认每分钟请求
	DefaultDailyQuota int `json:"default_daily_quota"` // 默认每日配额
	MaxRateLimit      int `json:"max_rate_limit"`      // 最大每分钟请求
	MaxDailyQuota     int `json:"max_daily_quota"`     // 最大每日配额
	MaxWebhooks       int `json:"max_webhooks"`        // 最大Webhook数
	MaxAPIKeys        int `json:"max_api_keys"`        // 最大API密钥数
}

// OpenAPISpec 简化的OpenAPI规范.
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       map[string]interface{} `json:"info"`
	Paths      map[string]interface{} `json:"paths"`
	Components map[string]interface{} `json:"components,omitempty"`
	Servers    []map[string]string    `json:"servers,omitempty"`
}

// DevPortalConfig 开发者门户配置.
type DevPortalConfig struct {
	Quota             QuotaConfig `json:"quota"`
	WebhookMaxRetries int         `json:"webhook_max_retries"`
	WebhookTimeout    int         `json:"webhook_timeout"`
	TokenExpiry       int         `json:"token_expiry"`   // 秒
	RefreshExpiry     int         `json:"refresh_expiry"` // 秒
}

var (
	ErrAPIKeyNotFound     = errors.New("api key not found")
	ErrAPIKeyExists       = errors.New("api key already exists")
	ErrAPIKeyRevoked      = errors.New("api key is revoked")
	ErrAPIKeyExpired      = errors.New("api key is expired")
	ErrWebhookNotFound    = errors.New("webhook not found")
	ErrWebhookExists      = errors.New("webhook already exists")
	ErrAppNotFound        = errors.New("developer app not found")
	ErrAppExists          = errors.New("developer app already exists")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
	ErrInvalidScope       = errors.New("invalid scope")
	ErrInvalidGrantType   = errors.New("invalid grant type")
	ErrDeliveryNotFound   = errors.New("delivery not found")
	ErrInvalidRedirectURI = errors.New("invalid redirect uri")
)
