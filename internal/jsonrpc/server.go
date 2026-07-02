// Package jsonrpc 提供JSON-RPC 2.0 API框架
package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// 错误定义.
var (
	ErrMethodNotFound    = errors.New("方法不存在")
	ErrInvalidParams     = errors.New("无效参数")
	ErrInternalError     = errors.New("内部错误")
	ErrInvalidRequest    = errors.New("无效请求")
	ErrParseError        = errors.New("解析错误")
	ErrAPIKeyNotFound    = errors.New("API密钥不存在")
	ErrAPIKeyExpired     = errors.New("API密钥已过期")
	ErrAPIKeyRevoked     = errors.New("API密钥已撤销")
	ErrPermissionDenied  = errors.New("权限被拒绝")
	ErrRateLimitExceeded = errors.New("请求频率超限")
)

// JSON-RPC 2.0 错误码.
const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603
)

// APIKeyStatus API密钥状态.
type APIKeyStatus string

const (
	APIKeyStatusActive  APIKeyStatus = "active"  // 活跃
	APIKeyStatusExpired APIKeyStatus = "expired" // 已过期
	APIKeyStatusRevoked APIKeyStatus = "revoked" // 已撤销
)

// Permission 权限类型.
type Permission string

const (
	PermRead    Permission = "read"    // 读取
	PermWrite   Permission = "write"   // 写入
	PermDelete  Permission = "delete"  // 删除
	PermAdmin   Permission = "admin"   // 管理
	PermExecute Permission = "execute" // 执行
)

// Request JSON-RPC 2.0 请求.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

// Response JSON-RPC 2.0 响应.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// Error JSON-RPC 2.0 错误.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// BatchRequest 批量请求.
type BatchRequest []Request

// APIKey API密钥.
type APIKey struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Key         string            `json:"key"`
	Secret      string            `json:"secret,omitempty"`
	UserID      string            `json:"user_id"`
	UserName    string            `json:"user_name,omitempty"`
	Permissions []Permission      `json:"permissions"`
	Status      APIKeyStatus      `json:"status"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	UsageCount  int64             `json:"usage_count"`
	RateLimit   int               `json:"rate_limit"` // 每分钟请求限制
	IPWhitelist []string          `json:"ip_whitelist,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// APIVersion API版本.
type APIVersion struct {
	Version      string     `json:"version"`
	ReleaseDate  string     `json:"release_date"`
	Status       string     `json:"status"` // current, deprecated, sunset
	DeprecatedAt *time.Time `json:"deprecated_at,omitempty"`
	SunsetAt     *time.Time `json:"sunset_at,omitempty"`
	Changes      []string   `json:"changes,omitempty"`
}

// Method API方法定义.
type Method struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Handler     MethodHandler `json:"-"`
	Params      []ParamDef    `json:"params,omitempty"`
	Permissions []Permission  `json:"required_permissions,omitempty"`
	Deprecated  bool          `json:"deprecated,omitempty"`
	Version     string        `json:"version,omitempty"`
}

// ParamDef 参数定义.
type ParamDef struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Description string      `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// MethodHandler 方法处理器.
type MethodHandler func(params json.RawMessage) (interface{}, *Error)

// RateLimitEntry 频率限制记录.
type RateLimitEntry struct {
	Count   int
	ResetAt time.Time
}

// Server JSON-RPC 2.0 服务器.
type Server struct {
	mu             sync.RWMutex
	methods        map[string]*Method
	apiKeys        map[string]*APIKey
	versions       []*APIVersion
	currentVersion string
	rateLimits     map[string]*RateLimitEntry
	maxBatchSize   int
	startTime      time.Time
	totalRequests  int64
}

// NewServer 创建JSON-RPC服务器.
func NewServer(version string) *Server {
	s := &Server{
		methods:        make(map[string]*Method),
		apiKeys:        make(map[string]*APIKey),
		rateLimits:     make(map[string]*RateLimitEntry),
		maxBatchSize:   100,
		startTime:      time.Now(),
		currentVersion: version,
	}

	// 注册默认版本
	s.versions = []*APIVersion{
		{
			Version:     version,
			ReleaseDate: time.Now().Format("2006-01-02"),
			Status:      "current",
		},
	}

	// 注册内置方法
	s.registerBuiltinMethods()

	return s
}

// registerBuiltinMethods 注册内置方法.
func (s *Server) registerBuiltinMethods() {
	s.RegisterMethod(&Method{
		Name:        "system.version",
		Description: "获取API版本",
		Handler: func(params json.RawMessage) (interface{}, *Error) {
			return map[string]interface{}{
				"version": s.currentVersion,
				"methods": len(s.methods),
			}, nil
		},
	})

	s.RegisterMethod(&Method{
		Name:        "system.methods",
		Description: "列出所有可用方法",
		Handler: func(params json.RawMessage) (interface{}, *Error) {
			methods := make([]map[string]interface{}, 0)
			for _, m := range s.methods {
				methods = append(methods, map[string]interface{}{
					"name":        m.Name,
					"description": m.Description,
					"deprecated":  m.Deprecated,
				})
			}
			return methods, nil
		},
	})

	s.RegisterMethod(&Method{
		Name:        "system.health",
		Description: "系统健康检查",
		Handler: func(params json.RawMessage) (interface{}, *Error) {
			return map[string]interface{}{
				"status":   "healthy",
				"uptime":   int64(time.Since(s.startTime).Seconds()),
				"requests": s.totalRequests,
				"methods":  len(s.methods),
				"api_keys": len(s.apiKeys),
			}, nil
		},
	})
}

// RegisterMethod 注册API方法.
func (s *Server) RegisterMethod(method *Method) error {
	if method == nil || method.Name == "" {
		return ErrInvalidParams
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.methods[method.Name] = method
	return nil
}

// UnregisterMethod 注销API方法.
func (s *Server) UnregisterMethod(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.methods[name]; !exists {
		return ErrMethodNotFound
	}

	delete(s.methods, name)
	return nil
}

// CreateAPIKey 创建API密钥.
func (s *Server) CreateAPIKey(key *APIKey) error {
	if key == nil || key.ID == "" || key.Key == "" || key.UserID == "" {
		return ErrInvalidParams
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key.Status = APIKeyStatusActive
	key.CreatedAt = time.Now()
	key.UpdatedAt = time.Now()
	s.apiKeys[key.ID] = key

	return nil
}

// RevokeAPIKey 撤销API密钥.
func (s *Server) RevokeAPIKey(keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, exists := s.apiKeys[keyID]
	if !exists {
		return ErrAPIKeyNotFound
	}

	key.Status = APIKeyStatusRevoked
	key.UpdatedAt = time.Now()

	return nil
}

// GetAPIKey 获取API密钥.
func (s *Server) GetAPIKey(keyID string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, exists := s.apiKeys[keyID]
	if !exists {
		return nil, ErrAPIKeyNotFound
	}

	return key, nil
}

// ListAPIKeys 列出用户API密钥.
func (s *Server) ListAPIKeys(userID string) []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*APIKey, 0)
	for _, key := range s.apiKeys {
		if userID == "" || key.UserID == userID {
			result = append(result, key)
		}
	}
	return result
}

// ValidateAPIKey 验证API密钥.
func (s *Server) ValidateAPIKey(keyStr string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, key := range s.apiKeys {
		if key.Key == keyStr {
			if key.Status == APIKeyStatusRevoked {
				return nil, ErrAPIKeyRevoked
			}
			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				key.Status = APIKeyStatusExpired
				return nil, ErrAPIKeyExpired
			}
			return key, nil
		}
	}

	return nil, ErrAPIKeyNotFound
}

// HandleRequest 处理单个请求.
func (s *Server) HandleRequest(req *Request) *Response {
	s.mu.Lock()
	s.totalRequests++
	s.mu.Unlock()

	if req.JSONRPC != "2.0" {
		return &Response{
			JSONRPC: "2.0",
			Error: &Error{
				Code:    ErrorCodeInvalidRequest,
				Message: "无效的JSON-RPC版本",
			},
			ID: req.ID,
		}
	}

	s.mu.RLock()
	method, exists := s.methods[req.Method]
	s.mu.RUnlock()

	if !exists {
		return &Response{
			JSONRPC: "2.0",
			Error: &Error{
				Code:    ErrorCodeMethodNotFound,
				Message: fmt.Sprintf("方法 %s 不存在", req.Method),
			},
			ID: req.ID,
		}
	}

	result, rpcErr := method.Handler(req.Params)
	if rpcErr != nil {
		return &Response{
			JSONRPC: "2.0",
			Error:   rpcErr,
			ID:      req.ID,
		}
	}

	return &Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}
}

// HandleBatchRequest 处理批量请求.
func (s *Server) HandleBatchRequest(batch BatchRequest) []*Response {
	if len(batch) > s.maxBatchSize {
		return []*Response{
			{
				JSONRPC: "2.0",
				Error: &Error{
					Code:    ErrorCodeInvalidRequest,
					Message: fmt.Sprintf("批量请求大小超过限制 %d", s.maxBatchSize),
				},
			},
		}
	}

	responses := make([]*Response, 0, len(batch))
	for _, req := range batch {
		responses = append(responses, s.HandleRequest(&req))
	}

	return responses
}

// GetStats 获取服务器统计.
func (s *Server) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"version":        s.currentVersion,
		"methods":        len(s.methods),
		"api_keys":       len(s.apiKeys),
		"total_requests": s.totalRequests,
		"uptime_seconds": int64(time.Since(s.startTime).Seconds()),
	}
}

// GetVersions 获取API版本历史.
func (s *Server) GetVersions() []*APIVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.versions
}

// CheckRateLimit 检查频率限制.
func (s *Server) CheckRateLimit(keyID string, limit int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.rateLimits[keyID]
	if !exists {
		s.rateLimits[keyID] = &RateLimitEntry{
			Count:   1,
			ResetAt: time.Now().Add(time.Minute),
		}
		return true
	}

	if time.Now().After(entry.ResetAt) {
		entry.Count = 1
		entry.ResetAt = time.Now().Add(time.Minute)
		return true
	}

	if entry.Count >= limit {
		return false
	}

	entry.Count++
	return true
}

// UpdateAPIKeyUsage 更新API密钥使用统计.
func (s *Server) UpdateAPIKeyUsage(keyID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key, exists := s.apiKeys[keyID]; exists {
		now := time.Now()
		key.UsageCount++
		key.LastUsedAt = &now
		key.UpdatedAt = now
	}
}
