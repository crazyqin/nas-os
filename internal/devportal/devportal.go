// Package devportal 开发者门户核心实现
package devportal

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Manager 开发者门户管理器
type Manager struct {
	mu           sync.RWMutex
	apiKeys      map[string]*APIKey
	webhooks     map[string]*WebhookEndpoint
	deliveries   []*WebhookDelivery
	apps         map[string]*DeveloperApp
	tokens       map[string]*OAuthToken
	usage        map[string][]*UsageRecord // ownerID -> records
	spec         *OpenAPISpec
	config       *DevPortalConfig
	dataFile     string
}

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		apiKeys:    make(map[string]*APIKey),
		webhooks:   make(map[string]*WebhookEndpoint),
		deliveries: make([]*WebhookDelivery, 0),
		apps:       make(map[string]*DeveloperApp),
		tokens:     make(map[string]*OAuthToken),
		usage:      make(map[string][]*UsageRecord),
		config: &DevPortalConfig{
			Quota: QuotaConfig{
				DefaultRateLimit:  60,
				DefaultDailyQuota: 10000,
				MaxRateLimit:      1000,
				MaxDailyQuota:     100000,
				MaxWebhooks:       50,
				MaxAPIKeys:        20,
			},
			WebhookMaxRetries: 3,
			WebhookTimeout:    30,
			TokenExpiry:       3600,
			RefreshExpiry:     86400 * 30,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	m.initDefaultAPISpec()
	return m.load()
}

// initDefaultAPISpec 初始化默认API文档
func (m *Manager) initDefaultAPISpec() {
	m.spec = &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: map[string]interface{}{
			"title":       "NAS-OS API",
			"version":     "1.0.0",
			"description": "NAS-OS 家庭服务器操作系统 API",
		},
		Paths: map[string]interface{}{
			"/api/v1/services": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "列出所有服务",
					"operationId": "listServices",
					"tags":        []string{"services"},
					"parameters": []map[string]interface{}{
						{"name": "type", "in": "query", "schema": map[string]string{"type": "string"}},
						{"name": "status", "in": "query", "schema": map[string]string{"type": "string"}},
					},
				},
				"post": map[string]interface{}{
					"summary":     "创建服务",
					"operationId": "createService",
					"tags":        []string{"services"},
				},
			},
			"/api/v1/services/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "获取服务详情",
					"operationId": "getService",
					"tags":        []string{"services"},
				},
				"delete": map[string]interface{}{
					"summary":     "删除服务",
					"operationId": "deleteService",
					"tags":        []string{"services"},
				},
			},
			"/api/v1/services/{id}/start": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "启动服务",
					"operationId": "startService",
					"tags":        []string{"services"},
				},
			},
			"/api/v1/services/{id}/stop": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "停止服务",
					"operationId": "stopService",
					"tags":        []string{"services"},
				},
			},
			"/api/v1/storage/files": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "列出文件",
					"operationId": "listFiles",
					"tags":        []string{"storage"},
				},
			},
			"/api/v1/backup/jobs": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "列出备份任务",
					"operationId": "listBackupJobs",
					"tags":        []string{"backup"},
				},
			},
		},
		Components: map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"ApiKeyAuth": map[string]interface{}{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-Key",
				},
				"BearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
		Servers: []map[string]string{
			{"url": "http://localhost:8080", "description": "本地开发"},
		},
	}
}

// ==================== API密钥管理 ====================

// CreateAPIKey 创建API密钥
func (m *Manager) CreateAPIKey(name, ownerID string, scopes []APIScope, rateLimit, dailyQuota int) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, k := range m.apiKeys {
		if k.OwnerID == ownerID && k.Status == KeyActive {
			count++
		}
	}
	if count >= m.config.Quota.MaxAPIKeys {
		return nil, ErrQuotaExceeded
	}

	if rateLimit <= 0 {
		rateLimit = m.config.Quota.DefaultRateLimit
	}
	if dailyQuota <= 0 {
		dailyQuota = m.config.Quota.DefaultDailyQuota
	}
	if rateLimit > m.config.Quota.MaxRateLimit {
		rateLimit = m.config.Quota.MaxRateLimit
	}
	if dailyQuota > m.config.Quota.MaxDailyQuota {
		dailyQuota = m.config.Quota.MaxDailyQuota
	}

	key, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	secret, err := generateRandomString(48)
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	apiKey := &APIKey{
		ID:         fmt.Sprintf("key-%d", time.Now().UnixNano()),
		Name:       name,
		Key:        key,
		Secret:     secret,
		OwnerID:    ownerID,
		Status:     KeyActive,
		Scopes:     scopes,
		RateLimit:  rateLimit,
		DailyQuota: dailyQuota,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.apiKeys[apiKey.ID] = apiKey
	return apiKey, m.save()
}

// RevokeAPIKey 吊销API密钥
func (m *Manager) RevokeAPIKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.apiKeys[id]
	if !ok {
		return ErrAPIKeyNotFound
	}
	key.Status = KeyRevoked
	key.UpdatedAt = time.Now()
	return m.save()
}

// GetAPIKey 获取API密钥
func (m *Manager) GetAPIKey(id string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.apiKeys[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return key, nil
}

// ListAPIKeys 列出API密钥
func (m *Manager) ListAPIKeys(ownerID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*APIKey
	for _, k := range m.apiKeys {
		if ownerID != "" && k.OwnerID != ownerID {
			continue
		}
		result = append(result, k)
	}
	return result
}

// ValidateAPIKey 验证API密钥并记录使用
func (m *Manager) ValidateAPIKey(keyStr string) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, k := range m.apiKeys {
		if k.Key == keyStr {
			if k.Status == KeyRevoked {
				return nil, ErrAPIKeyRevoked
			}
			if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
				k.Status = KeyExpired
				return nil, ErrAPIKeyExpired
			}
			if k.UsedToday >= k.DailyQuota {
				return nil, ErrQuotaExceeded
			}
			now := time.Now()
			k.LastUsedAt = &now
			k.TotalCalls++
			k.UsedToday++
			return k, nil
		}
	}
	return nil, ErrAPIKeyNotFound
}

// ResetDailyUsage 重置每日使用量
func (m *Manager) ResetDailyUsage() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.apiKeys {
		k.UsedToday = 0
	}
}

// ==================== Webhook管理 ====================

// RegisterWebhook 注册Webhook
func (m *Manager) RegisterWebhook(name, url, ownerID string, events []WebhookEvent) (*WebhookEndpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, w := range m.webhooks {
		if w.OwnerID == ownerID && w.Status == WebhookActive {
			count++
		}
	}
	if count >= m.config.Quota.MaxWebhooks {
		return nil, ErrQuotaExceeded
	}

	secret, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	wh := &WebhookEndpoint{
		ID:         fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Name:       name,
		URL:        url,
		OwnerID:    ownerID,
		Status:     WebhookActive,
		Events:     events,
		Secret:     secret,
		Headers:    make(map[string]string),
		MaxRetries: m.config.WebhookMaxRetries,
		Timeout:    m.config.WebhookTimeout,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.webhooks[wh.ID] = wh
	return wh, m.save()
}

// UpdateWebhook 更新Webhook
func (m *Manager) UpdateWebhook(id string, name, url string, events []WebhookEvent) (*WebhookEndpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wh, ok := m.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}
	if name != "" {
		wh.Name = name
	}
	if url != "" {
		wh.URL = url
	}
	if events != nil {
		wh.Events = events
	}
	wh.UpdatedAt = time.Now()
	return wh, m.save()
}

// DeleteWebhook 删除Webhook
func (m *Manager) DeleteWebhook(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.webhooks[id]; !ok {
		return ErrWebhookNotFound
	}
	delete(m.webhooks, id)
	return m.save()
}

// GetWebhook 获取Webhook
func (m *Manager) GetWebhook(id string) (*WebhookEndpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wh, ok := m.webhooks[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}
	return wh, nil
}

// ListWebhooks 列出Webhook
func (m *Manager) ListWebhooks(ownerID string) []*WebhookEndpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*WebhookEndpoint
	for _, w := range m.webhooks {
		if ownerID != "" && w.OwnerID != ownerID {
			continue
		}
		result = append(result, w)
	}
	return result
}

// TriggerWebhook 触发Webhook事件
func (m *Manager) TriggerWebhook(event WebhookEvent, payload interface{}) {
	m.mu.RLock()
	var targets []*WebhookEndpoint
	for _, wh := range m.webhooks {
		if wh.Status != WebhookActive {
			continue
		}
		for _, e := range wh.Events {
			if e == event {
				targets = append(targets, wh)
				break
			}
		}
	}
	m.mu.RUnlock()

	for _, wh := range targets {
		go m.deliverWebhook(wh, event, payload)
	}
}

// deliverWebhook 投递Webhook
func (m *Manager) deliverWebhook(wh *WebhookEndpoint, event WebhookEvent, payload interface{}) {
	body, _ := json.Marshal(map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      payload,
	})

	signature := m.computeHMAC(body, wh.Secret)

	delivery := &WebhookDelivery{
		ID:        fmt.Sprintf("del-%d", time.Now().UnixNano()),
		WebhookID: wh.ID,
		Event:     event,
		URL:       wh.URL,
		Status:    DeliveryPending,
		CreatedAt: time.Now(),
	}

	client := &http.Client{Timeout: time.Duration(wh.Timeout) * time.Second}
	var lastErr string
	var lastStatus int

	for attempt := 1; attempt <= wh.MaxRetries; attempt++ {
		delivery.Attempts = attempt
		start := time.Now()

		req, err := http.NewRequest(http.MethodPost, wh.URL, strings.NewReader(string(body)))
		if err != nil {
			lastErr = err.Error()
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Event", string(event))
		req.Header.Set("X-Webhook-Signature", signature)
		for k, v := range wh.Headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		delivery.Duration = time.Since(start).Milliseconds()

		if err != nil {
			lastErr = err.Error()
			delivery.Status = DeliveryRetrying
			if attempt < wh.MaxRetries {
				time.Sleep(time.Duration(attempt*attempt) * time.Second)
			}
			continue
		}
		resp.Body.Close()
		lastStatus = resp.StatusCode

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			delivery.StatusCode = resp.StatusCode
			delivery.Status = DeliverySuccess
			break
		}
		lastErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
		if attempt < wh.MaxRetries {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}
	}

	if delivery.Status != DeliverySuccess {
		delivery.Status = DeliveryFailed
		delivery.Error = lastErr
		delivery.StatusCode = lastStatus
	}

	m.mu.Lock()
	wh.TotalSent++
	if delivery.Status == DeliveryFailed {
		wh.TotalFailed++
	} else {
		now := time.Now()
		wh.LastDeliveredAt = &now
	}
	m.deliveries = append(m.deliveries, delivery)
	if len(m.deliveries) > 10000 {
		m.deliveries = m.deliveries[len(m.deliveries)-10000:]
	}
	m.mu.Unlock()
}

// RetryDelivery 重试投递
func (m *Manager) RetryDelivery(deliveryID string) error {
	m.mu.RLock()
	var delivery *WebhookDelivery
	for _, d := range m.deliveries {
		if d.ID == deliveryID {
			delivery = d
			break
		}
	}
	if delivery == nil {
		m.mu.RUnlock()
		return ErrDeliveryNotFound
	}
	wh, ok := m.webhooks[delivery.WebhookID]
	m.mu.RUnlock()
	if !ok {
		return ErrWebhookNotFound
	}

	go m.deliverWebhook(wh, delivery.Event, nil)
	return nil
}

// ListDeliveries 列出投递记录
func (m *Manager) ListDeliveries(webhookID string, limit int) []*WebhookDelivery {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*WebhookDelivery
	for i := len(m.deliveries) - 1; i >= 0; i-- {
		if webhookID != "" && m.deliveries[i].WebhookID != webhookID {
			continue
		}
		result = append(result, m.deliveries[i])
		if len(result) >= limit {
			break
		}
	}
	return result
}

// ==================== 使用量统计 ====================

// RecordUsage 记录API使用量
func (m *Manager) RecordUsage(ownerID string, success bool, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	records := m.usage[ownerID]
	if len(records) > 0 && records[len(records)-1].Date == today {
		last := records[len(records)-1]
		last.Total++
		if success {
			last.Success++
		} else {
			last.Failed++
		}
		last.AvgLatency = (last.AvgLatency*int64(last.Total-1) + latencyMs) / int64(last.Total)
		return
	}
	m.usage[ownerID] = append(records, &UsageRecord{
		Date:       today,
		Total:      1,
		Success:    boolToInt(success),
		Failed:     boolToInt(!success),
		AvgLatency: latencyMs,
	})
}

// GetUsageStats 获取使用量统计
func (m *Manager) GetUsageStats(ownerID string, days int) []*UsageRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if days <= 0 {
		days = 30
	}
	records := m.usage[ownerID]
	if len(records) > days {
		return records[len(records)-days:]
	}
	return records
}

// ==================== 开发者应用 ====================

// RegisterApp 注册开发者应用
func (m *Manager) RegisterApp(name, ownerID, description string, redirectURIs []string, grantTypes []OAuthGrantType, scopes []APIScope) (*DeveloperApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientID, err := generateRandomString(24)
	if err != nil {
		return nil, fmt.Errorf("generate client_id: %w", err)
	}
	clientSecret, err := generateRandomString(48)
	if err != nil {
		return nil, fmt.Errorf("generate client_secret: %w", err)
	}

	app := &DeveloperApp{
		ID:           fmt.Sprintf("app-%d", time.Now().UnixNano()),
		Name:         name,
		Description:  description,
		OwnerID:      ownerID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: redirectURIs,
		GrantTypes:   grantTypes,
		Scopes:       scopes,
		Status:       AppApproved,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.apps[app.ID] = app
	return app, m.save()
}

// GetApp 获取开发者应用
func (m *Manager) GetApp(id string) (*DeveloperApp, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, ok := m.apps[id]
	if !ok {
		return nil, ErrAppNotFound
	}
	return app, nil
}

// ListApps 列出开发者应用
func (m *Manager) ListApps(ownerID string) []*DeveloperApp {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DeveloperApp
	for _, app := range m.apps {
		if ownerID != "" && app.OwnerID != ownerID {
			continue
		}
		result = append(result, app)
	}
	return result
}

// DeleteApp 删除开发者应用
func (m *Manager) DeleteApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.apps[id]; !ok {
		return ErrAppNotFound
	}
	delete(m.apps, id)
	return m.save()
}

// IssueToken 签发OAuth2令牌
func (m *Manager) IssueToken(clientID, clientSecret string, grantType OAuthGrantType, scopes []APIScope) (*OAuthToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var app *DeveloperApp
	for _, a := range m.apps {
		if a.ClientID == clientID && a.ClientSecret == clientSecret {
			app = a
			break
		}
	}
	if app == nil {
		return nil, ErrAppNotFound
	}
	if app.Status != AppApproved {
		return nil, fmt.Errorf("app not approved: %s", app.Status)
	}

	validGrant := false
	for _, gt := range app.GrantTypes {
		if gt == grantType {
			validGrant = true
			break
		}
	}
	if !validGrant {
		return nil, ErrInvalidGrantType
	}

	accessToken, _ := generateRandomString(64)
	refreshToken, _ := generateRandomString(64)

	scopeStrs := make([]string, len(scopes))
	for i, s := range scopes {
		scopeStrs[i] = string(s)
	}

	token := &OAuthToken{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    m.config.TokenExpiry,
		RefreshToken: refreshToken,
		Scope:        strings.Join(scopeStrs, " "),
		AppID:        app.ID,
		CreatedAt:    time.Now(),
	}
	m.tokens[accessToken] = token
	return token, nil
}

// ValidateToken 验证OAuth2令牌
func (m *Manager) ValidateToken(accessToken string) (*OAuthToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.tokens[accessToken]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}
	if time.Since(token.CreatedAt) > time.Duration(token.ExpiresIn)*time.Second {
		return nil, fmt.Errorf("token expired")
	}
	return token, nil
}

// ==================== API文档 ====================

// GetAPISpec 获取OpenAPI规范
func (m *Manager) GetAPISpec() *OpenAPISpec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.spec
}

// UpdateAPISpec 更新OpenAPI规范
func (m *Manager) UpdateAPISpec(spec *OpenAPISpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spec = spec
}

// ==================== SDK代码生成 ====================

// GenerateSDK 生成SDK示例代码
func (m *Manager) GenerateSDK(lang SDKLanguage) (string, error) {
	switch lang {
	case SDKPython:
		return m.genPythonSDK(), nil
	case SDKGo:
		return m.genGoSDK(), nil
	case SDKJavaScript:
		return m.genJavaScriptSDK(), nil
	default:
		return "", fmt.Errorf("unsupported language: %s", lang)
	}
}

func (m *Manager) genPythonSDK() string {
	return `"""NAS-OS Python SDK"""
import requests
from typing import Optional, Dict, Any


class NasOSClient:
    """NAS-OS API客户端"""

    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()
        self.session.headers.update({
            "X-API-Key": api_key,
            "Content-Type": "application/json",
        })

    def list_services(self, service_type: str = None, status: str = None) -> Dict[str, Any]:
        """列出所有服务"""
        params = {}
        if service_type:
            params["type"] = service_type
        if status:
            params["status"] = status
        resp = self.session.get(f"{self.base_url}/api/v1/services", params=params)
        resp.raise_for_status()
        return resp.json()

    def get_service(self, service_id: str) -> Dict[str, Any]:
        """获取服务详情"""
        resp = self.session.get(f"{self.base_url}/api/v1/services/{service_id}")
        resp.raise_for_status()
        return resp.json()

    def create_service(self, name: str, service_type: str, image: str, **kwargs) -> Dict[str, Any]:
        """创建服务"""
        data = {"name": name, "type": service_type, "image": image, **kwargs}
        resp = self.session.post(f"{self.base_url}/api/v1/services", json=data)
        resp.raise_for_status()
        return resp.json()

    def start_service(self, service_id: str) -> Dict[str, Any]:
        """启动服务"""
        resp = self.session.post(f"{self.base_url}/api/v1/services/{service_id}/start")
        resp.raise_for_status()
        return resp.json()

    def stop_service(self, service_id: str) -> Dict[str, Any]:
        """停止服务"""
        resp = self.session.post(f"{self.base_url}/api/v1/services/{service_id}/stop")
        resp.raise_for_status()
        return resp.json()

    def list_files(self, path: str = "/") -> Dict[str, Any]:
        """列出文件"""
        resp = self.session.get(f"{self.base_url}/api/v1/storage/files", params={"path": path})
        resp.raise_for_status()
        return resp.json()


# 使用示例
if __name__ == "__main__":
    client = NasOSClient("http://localhost:8080", "your-api-key")
    services = client.list_services()
    print(services)
`
}

func (m *Manager) genGoSDK() string {
	return `// Package nasos NAS-OS Go SDK
package nasos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client NAS-OS API客户端
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient 创建客户端
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

// Service 服务定义
type Service struct {
	ID     string ` + "`json:\"id\"`" + `
	Name   string ` + "`json:\"name\"`" + `
	Type   string ` + "`json:\"type\"`" + `
	Status string ` + "`json:\"status\"`" + `
	Image  string ` + "`json:\"image\"`" + `
}

// APIResponse API响应
type APIResponse struct {
	Code    int             ` + "`json:\"code\"`" + `
	Data    json.RawMessage ` + "`json:\"data\"`" + `
	Message string          ` + "`json:\"message,omitempty\"`" + `
	Total   int             ` + "`json:\"total,omitempty\"`" + `
}

// ListServices 列出服务
func (c *Client) ListServices() ([]Service, error) {
	resp, err := c.doRequest("GET", "/api/v1/services", nil)
	if err != nil {
		return nil, err
	}
	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, err
	}
	var services []Service
	if err := json.Unmarshal(apiResp.Data, &services); err != nil {
		return nil, err
	}
	return services, nil
}

// GetService 获取服务
func (c *Client) GetService(id string) (*Service, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/v1/services/%s", id), nil)
	if err != nil {
		return nil, err
	}
	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, err
	}
	var svc Service
	if err := json.Unmarshal(apiResp.Data, &svc); err != nil {
		return nil, err
	}
	return &svc, nil
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
`
}

func (m *Manager) genJavaScriptSDK() string {
	return `/**
 * NAS-OS JavaScript SDK
 */
class NasOSClient {
  /**
   * @param {string} baseURL - API基础URL
   * @param {string} apiKey - API密钥
   */
  constructor(baseURL, apiKey) {
    this.baseURL = baseURL.replace(/\\/$/, '');
    this.apiKey = apiKey;
  }

  async request(method, path, body = null) {
    const options = {
      method,
      headers: {
        'X-API-Key': this.apiKey,
        'Content-Type': 'application/json',
      },
    };
    if (body) {
      options.body = JSON.stringify(body);
    }
    const resp = await fetch(this.baseURL + path, options);
    if (!resp.ok) {
      throw new Error('API error: ' + resp.status);
    }
    return resp.json();
  }

  /** 列出服务 */
  async listServices(params = {}) {
    const query = new URLSearchParams(params).toString();
    const path = '/api/v1/services' + (query ? '?' + query : '');
    return this.request('GET', path);
  }

  /** 获取服务详情 */
  async getService(id) {
    return this.request('GET', '/api/v1/services/' + id);
  }

  /** 创建服务 */
  async createService(data) {
    return this.request('POST', '/api/v1/services', data);
  }

  /** 启动服务 */
  async startService(id) {
    return this.request('POST', '/api/v1/services/' + id + '/start');
  }

  /** 停止服务 */
  async stopService(id) {
    return this.request('POST', '/api/v1/services/' + id + '/stop');
  }

  /** 列出文件 */
  async listFiles(path = '/') {
    return this.request('GET', '/api/v1/storage/files?path=' + encodeURIComponent(path));
  }
}

// 使用示例
// const client = new NasOSClient('http://localhost:8080', 'your-api-key');
// const services = await client.listServices();
// console.log(services);

module.exports = { NasOSClient };
`
}

// ==================== 统计 ====================

// GetStats 获取门户统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeKeys := 0
	totalCalls := int64(0)
	for _, k := range m.apiKeys {
		if k.Status == KeyActive {
			activeKeys++
		}
		totalCalls += k.TotalCalls
	}

	activeWebhooks := 0
	for _, w := range m.webhooks {
		if w.Status == WebhookActive {
			activeWebhooks++
		}
	}

	return map[string]interface{}{
		"total_api_keys":     len(m.apiKeys),
		"active_api_keys":    activeKeys,
		"total_api_calls":    totalCalls,
		"total_webhooks":     len(m.webhooks),
		"active_webhooks":    activeWebhooks,
		"total_deliveries":   len(m.deliveries),
		"total_apps":         len(m.apps),
		"total_tokens":       len(m.tokens),
	}
}

// ==================== 持久化 ====================

type storedData struct {
	APIKeys    map[string]*APIKey         `json:"api_keys"`
	Webhooks   map[string]*WebhookEndpoint `json:"webhooks"`
	Deliveries []*WebhookDelivery         `json:"deliveries"`
	Apps       map[string]*DeveloperApp   `json:"apps"`
	Tokens     map[string]*OAuthToken     `json:"tokens"`
	Usage      map[string][]*UsageRecord  `json:"usage"`
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored storedData
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.APIKeys != nil {
		m.apiKeys = stored.APIKeys
	}
	if stored.Webhooks != nil {
		m.webhooks = stored.Webhooks
	}
	if stored.Deliveries != nil {
		m.deliveries = stored.Deliveries
	}
	if stored.Apps != nil {
		m.apps = stored.Apps
	}
	if stored.Tokens != nil {
		m.tokens = stored.Tokens
	}
	if stored.Usage != nil {
		m.usage = stored.Usage
	}
	return nil
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(storedData{
		APIKeys:    m.apiKeys,
		Webhooks:   m.webhooks,
		Deliveries: m.deliveries,
		Apps:       m.apps,
		Tokens:     m.tokens,
		Usage:      m.usage,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}

// ==================== 工具函数 ====================

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

func (m *Manager) computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
