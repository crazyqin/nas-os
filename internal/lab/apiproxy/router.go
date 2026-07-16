package apiproxy

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ========== 智能路由管理器 ==========

// Router 智能路由管理器
// 根据模型名路由到对应 provider，支持负载均衡、故障转移、本地模型优先.
type Router struct {
	mu        sync.RWMutex
	providers map[string]*AIProvider     // providerID -> provider
	health    map[string]*ProviderHealth // providerID -> health
	modelMap  map[string][]string        // modelName -> providerID 列表（按优先级排序）
	rrCounter uint64                     // 轮询计数器
}

// NewRouter 创建路由管理器.
func NewRouter() *Router {
	return &Router{
		providers: make(map[string]*AIProvider),
		health:    make(map[string]*ProviderHealth),
		modelMap:  make(map[string][]string),
	}
}

// RegisterProvider 注册一个 AI provider.
func (r *Router) RegisterProvider(p *AIProvider) error {
	if p == nil {
		return fmt.Errorf("provider 不能为空")
	}
	if p.ID == "" {
		return fmt.Errorf("provider ID 不能为空")
	}
	if p.Endpoint == "" {
		return fmt.Errorf("provider endpoint 不能为空")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 设置默认值
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()

	r.providers[p.ID] = p

	// 初始化健康状态
	r.health[p.ID] = &ProviderHealth{
		ProviderID:  p.ID,
		Healthy:     true,
		LastChecked: time.Now(),
	}

	// 更新模型映射
	for _, model := range p.Models {
		r.modelMap[model] = append(r.modelMap[model], p.ID)
	}

	// 按优先级排序：数值小的在前
	r.sortModelProviders()

	return nil
}

// UnregisterProvider 注销 provider.
func (r *Router) UnregisterProvider(providerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.providers[providerID]
	if !ok {
		return fmt.Errorf("provider %s 不存在", providerID)
	}

	// 从模型映射中移除
	for _, model := range p.Models {
		ids := r.modelMap[model]
		for i, id := range ids {
			if id == providerID {
				r.modelMap[model] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
		if len(r.modelMap[model]) == 0 {
			delete(r.modelMap, model)
		}
	}

	delete(r.providers, providerID)
	delete(r.health, providerID)

	return nil
}

// Route 根据模型名路由到合适的 provider
// 策略：本地模型优先 → 按优先级 → 负载均衡（轮询）→ 故障转移.
func (r *Router) Route(model string) (*AIProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerIDs, ok := r.modelMap[model]
	if !ok || len(providerIDs) == 0 {
		return nil, fmt.Errorf("没有找到支持模型 %s 的 provider", model)
	}

	// 过滤出健康且启用的 provider
	var candidates []*AIProvider
	for _, pid := range providerIDs {
		p := r.providers[pid]
		if !p.Enabled {
			continue
		}
		h := r.health[pid]
		if h != nil && !h.Healthy && h.ConsecutiveFailures >= 3 {
			// 连续失败 3 次以上，跳过
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		// 所有 provider 都不健康，尝试故障转移：返回第一个未启用的也行
		// 或者返回错误
		return nil, fmt.Errorf("模型 %s 的所有 provider 均不可用", model)
	}

	// 本地模型优先
	localFirst := make([]*AIProvider, 0, len(candidates))
	remoteFirst := make([]*AIProvider, 0, len(candidates))
	for _, p := range candidates {
		if p.IsLocal {
			localFirst = append(localFirst, p)
		} else {
			remoteFirst = append(remoteFirst, p)
		}
	}

	// 优先使用本地模型
	if len(localFirst) > 0 {
		// 在本地模型中按优先级 + 轮询
		return r.selectByRoundRobin(localFirst), nil
	}

	// 没有本地模型，使用远程模型
	return r.selectByRoundRobin(remoteFirst), nil
}

// RouteWithFallback 带故障转移的路由
// 如果首选 provider 请求失败，自动切换到下一个可用 provider.
func (r *Router) RouteWithFallback(model string, excludeIDs ...string) (*AIProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerIDs, ok := r.modelMap[model]
	if !ok || len(providerIDs) == 0 {
		return nil, fmt.Errorf("没有找到支持模型 %s 的 provider", model)
	}

	// 构建排除集合
	exclude := make(map[string]bool)
	for _, id := range excludeIDs {
		exclude[id] = true
	}

	var candidates []*AIProvider
	for _, pid := range providerIDs {
		if exclude[pid] {
			continue
		}
		p := r.providers[pid]
		if !p.Enabled {
			continue
		}
		h := r.health[pid]
		if h != nil && !h.Healthy && h.ConsecutiveFailures >= 3 {
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("模型 %s 没有更多可用的 fallback provider", model)
	}

	// 本地优先
	localFirst := make([]*AIProvider, 0, len(candidates))
	remoteFirst := make([]*AIProvider, 0, len(candidates))
	for _, p := range candidates {
		if p.IsLocal {
			localFirst = append(localFirst, p)
		} else {
			remoteFirst = append(remoteFirst, p)
		}
	}

	if len(localFirst) > 0 {
		return r.selectByRoundRobin(localFirst), nil
	}
	return r.selectByRoundRobin(remoteFirst), nil
}

// selectByRoundRobin 轮询选择（线程安全）.
func (r *Router) selectByRoundRobin(candidates []*AIProvider) *AIProvider {
	if len(candidates) == 1 {
		return candidates[0]
	}
	idx := atomic.AddUint64(&r.rrCounter, 1)
	return candidates[idx%uint64(len(candidates))]
}

// MarkProviderHealth 更新 provider 健康状态.
func (r *Router) MarkProviderHealth(providerID string, healthy bool, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	h, ok := r.health[providerID]
	if !ok {
		return
	}

	h.LastChecked = time.Now()
	if healthy {
		h.Healthy = true
		h.ConsecutiveFailures = 0
		h.LastError = ""
	} else {
		h.Healthy = false
		h.LastError = errMsg
		h.ConsecutiveFailures++
	}
}

// GetProvider 获取 provider 信息.
func (r *Router) GetProvider(providerID string) (*AIProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	return p, ok
}

// ListProviders 列出所有 provider.
func (r *Router) ListProviders() []*AIProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*AIProvider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// GetProviderHealth 获取 provider 健康状态.
func (r *Router) GetProviderHealth(providerID string) (*ProviderHealth, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.health[providerID]
	return h, ok
}

// ListModels 列出所有可用模型.
func (r *Router) ListModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	models := make([]string, 0, len(r.modelMap))
	for m := range r.modelMap {
		models = append(models, m)
	}
	return models
}

// sortModelProviders 按优先级排序每个模型的 provider 列表.
func (r *Router) sortModelProviders() {
	for model, ids := range r.modelMap {
		// 简单冒泡排序，按 priority 升序
		sorted := make([]string, len(ids))
		copy(sorted, ids)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				pi := r.providers[sorted[i]]
				pj := r.providers[sorted[j]]
				if pi != nil && pj != nil && pi.Priority > pj.Priority {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		r.modelMap[model] = sorted
	}
}

// ResetHealth 重置所有 provider 健康状态为健康.
func (r *Router) ResetHealth() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.health {
		h.Healthy = true
		h.ConsecutiveFailures = 0
		h.LastError = ""
		h.LastChecked = time.Now()
	}
}

// SetSeed 设置随机种子（测试用）.
func (r *Router) SetSeed(seed int64) {
	atomic.StoreUint64(&r.rrCounter, uint64(seed))
}

// randIntn 辅助函数（避免引入 math/rand 全局状态）.
var randIntn = func(n int) int {
	return rand.Intn(n)
}
