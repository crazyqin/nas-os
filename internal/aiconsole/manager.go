package aiconsole

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Manager AI Console 管理器.
type Manager struct {
	mu         sync.RWMutex
	service    *Service
	gateway    *Gateway
	dashboard  *Dashboard
	providerMgr *ProviderManager
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewManager 创建管理器实例.
func NewManager(db interface{}) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 类型断言获取 *sql.DB
	// 这里简化处理，实际需要传入 *sql.DB
	service := &Service{
		client: &http.Client{Timeout: 120 * time.Second},
	}

	dashboard := &Dashboard{
		startTime: time.Now(),
	}

	gateway := &Gateway{
		service:     service,
		providerMgr: NewProviderManager(),
		stopCh:      make(chan struct{}),
	}

	return &Manager{
		service:     service,
		gateway:     gateway,
		dashboard:   dashboard,
		providerMgr: NewProviderManager(),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start 启动管理器.
func (m *Manager) Start() error {
	return m.gateway.Start(m.ctx)
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.cancel()
	m.gateway.Stop()
}

// GetService 获取服务实例.
func (m *Manager) GetService() *Service {
	return m.service
}

// GetGateway 获取网关实例.
func (m *Manager) GetGateway() *Gateway {
	return m.gateway
}

// GetDashboard 获取仪表盘实例.
func (m *Manager) GetDashboard() *Dashboard {
	return m.dashboard
}

// GetProviderManager 获取提供者管理器.
func (m *Manager) GetProviderManager() *ProviderManager {
	return m.providerMgr
}

// ==================== 模型故障转移 ====================

// FailoverConfig 故障转移配置.
type FailoverConfig struct {
	MaxRetries     int           `json:"maxRetries"`
	RetryDelay     time.Duration `json:"retryDelay"`
	HealthCheckTTL time.Duration `json:"healthCheckTTL"`
}

// DefaultFailoverConfig 默认故障转移配置.
func DefaultFailoverConfig() *FailoverConfig {
	return &FailoverConfig{
		MaxRetries:     3,
		RetryDelay:     time.Second,
		HealthCheckTTL: 60 * time.Second,
	}
}

// FailoverManager 故障转移管理器.
type FailoverManager struct {
	mu       sync.RWMutex
	service  *Service
	config   *FailoverConfig
	health   map[string]*ModelHealth
}

// ModelHealth 模型健康状态.
type ModelHealth struct {
	ModelID      string    `json:"modelId"`
	Healthy      bool      `json:"healthy"`
	LastCheck    time.Time `json:"lastCheck"`
	SuccessCount int64     `json:"successCount"`
	FailCount    int64     `json:"failCount"`
	LastError    string    `json:"lastError,omitempty"`
	AvgLatencyMs float64   `json:"avgLatencyMs"`
}

// NewFailoverManager 创建故障转移管理器.
func NewFailoverManager(service *Service, config *FailoverConfig) *FailoverManager {
	if config == nil {
		config = DefaultFailoverConfig()
	}
	return &FailoverManager{
		service: service,
		config:  config,
		health:  make(map[string]*ModelHealth),
	}
}

// RecordSuccess 记录成功调用.
func (fm *FailoverManager) RecordSuccess(modelID string, latencyMs float64) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	h, ok := fm.health[modelID]
	if !ok {
		h = &ModelHealth{ModelID: modelID}
		fm.health[modelID] = h
	}

	h.Healthy = true
	h.LastCheck = time.Now()
	h.SuccessCount++
	h.AvgLatencyMs = (h.AvgLatencyMs*float64(h.SuccessCount-1) + latencyMs) / float64(h.SuccessCount)
}

// RecordFailure 记录失败调用.
func (fm *FailoverManager) RecordFailure(modelID string, err error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	h, ok := fm.health[modelID]
	if !ok {
		h = &ModelHealth{ModelID: modelID}
		fm.health[modelID] = h
	}

	h.LastCheck = time.Now()
	h.FailCount++
	h.LastError = err.Error()

	// 连续失败 3 次标记为不健康
	if h.FailCount >= 3 && h.SuccessCount == 0 {
		h.Healthy = false
	}
}

// IsHealthy 检查模型是否健康.
func (fm *FailoverManager) IsHealthy(modelID string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	h, ok := fm.health[modelID]
	if !ok {
		return true // 未知模型默认健康
	}
	return h.Healthy
}

// GetHealth 获取模型健康状态.
func (fm *FailoverManager) GetHealth(modelID string) *ModelHealth {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.health[modelID]
}

// GetAllHealth 获取所有模型健康状态.
func (fm *FailoverManager) GetAllHealth() map[string]*ModelHealth {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make(map[string]*ModelHealth, len(fm.health))
	for k, v := range fm.health {
		cp := *v
		result[k] = &cp
	}
	return result
}

// ResetHealth 重置模型健康状态.
func (fm *FailoverManager) ResetHealth(modelID string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.health[modelID] = &ModelHealth{
		ModelID:   modelID,
		Healthy:   true,
		LastCheck: time.Now(),
	}
}

// SelectBestModel 选择最佳可用模型.
func (fm *FailoverManager) SelectBestModel(models []*AIModel) (*AIModel, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var best *AIModel
	var bestScore float64 = -1

	for _, m := range models {
		if !m.Enabled || m.Status != ModelStatusActive {
			continue
		}

		h, ok := fm.health[m.ID]
		if !ok {
			// 未记录的模型给中等分
			score := 50.0
			if best == nil || score > bestScore {
				best = m
				bestScore = score
			}
			continue
		}

		if !h.Healthy {
			continue
		}

		// 评分：成功率权重 70%，延迟权重 30%
		var successRate float64
		total := h.SuccessCount + h.FailCount
		if total > 0 {
			successRate = float64(h.SuccessCount) / float64(total) * 100
		} else {
			successRate = 100
		}

		latencyScore := 100.0
		if h.AvgLatencyMs > 0 {
			latencyScore = 100.0 / (1 + h.AvgLatencyMs/1000)
		}

		score := successRate*0.7 + latencyScore*0.3
		if score > bestScore {
			best = m
			bestScore = score
		}
	}

	if best == nil {
		return nil, fmt.Errorf("无可用健康模型")
	}
	return best, nil
}

// ==================== 访问控制 ====================

// AccessPolicy 访问策略.
type AccessPolicy struct {
	UserID       string   `json:"userId"`
	GroupID      string   `json:"groupId,omitempty"`
	AllowedModels []string `json:"allowedModels"`
	DeniedModels  []string `json:"deniedModels"`
	MaxTokensPerDay int   `json:"maxTokensPerDay"`
	MaxRequestsPerDay int `json:"maxRequestsPerDay"`
}

// AccessControl 访问控制.
type AccessControl struct {
	mu       sync.RWMutex
	policies map[string]*AccessPolicy // userID -> policy
	groupPolicies map[string]*AccessPolicy // groupID -> policy
}

// NewAccessControl 创建访问控制.
func NewAccessControl() *AccessControl {
	return &AccessControl{
		policies:      make(map[string]*AccessPolicy),
		groupPolicies: make(map[string]*AccessPolicy),
	}
}

// SetUserPolicy 设置用户策略.
func (ac *AccessControl) SetUserPolicy(policy *AccessPolicy) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.policies[policy.UserID] = policy
}

// SetGroupPolicy 设置组策略.
func (ac *AccessControl) SetGroupPolicy(policy *AccessPolicy) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.groupPolicies[policy.GroupID] = policy
}

// CheckAccess 检查访问权限.
func (ac *AccessControl) CheckAccess(userID, modelID string) error {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	// 检查用户策略
	if p, ok := ac.policies[userID]; ok {
		// 检查拒绝列表
		for _, denied := range p.DeniedModels {
			if denied == modelID {
				return fmt.Errorf("用户 %s 无权访问模型 %s", userID, modelID)
			}
		}
		// 如果有允许列表，检查是否在列表中
		if len(p.AllowedModels) > 0 {
			allowed := false
			for _, a := range p.AllowedModels {
				if a == modelID {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("用户 %s 无权访问模型 %s", userID, modelID)
			}
		}
	}

	return nil
}

// GetUserPolicy 获取用户策略.
func (ac *AccessControl) GetUserPolicy(userID string) *AccessPolicy {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.policies[userID]
}
