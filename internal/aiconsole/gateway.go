package aiconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Gateway 统一 API 网关.
type Gateway struct {
	mu           sync.RWMutex
	service      *Service
	providerMgr  *ProviderManager
	healthTicker *time.Ticker
	stopCh       chan struct{}
}

// NewGateway 创建 API 网关.
func NewGateway(service *Service) *Gateway {
	gw := &Gateway{
		service:     service,
		providerMgr: NewProviderManager(),
		stopCh:      make(chan struct{}),
	}
	return gw
}

// Start 启动网关（包含健康检查）.
func (gw *Gateway) Start(ctx context.Context) error {
	// 启动健康检查定时器
	gw.healthTicker = time.NewTicker(60 * time.Second)
	go gw.healthCheckLoop(ctx)
	return nil
}

// Stop 停止网关.
func (gw *Gateway) Stop() {
	if gw.healthTicker != nil {
		gw.healthTicker.Stop()
	}
	close(gw.stopCh)
}

// healthCheckLoop 健康检查循环.
func (gw *Gateway) healthCheckLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-gw.stopCh:
			return
		case <-gw.healthTicker.C:
			gw.runHealthChecks(ctx)
		}
	}
}

// runHealthChecks 执行所有提供者健康检查.
func (gw *Gateway) runHealthChecks(ctx context.Context) {
	models, err := gw.service.ListModels()
	if err != nil {
		return
	}

	for _, model := range models {
		if !model.Enabled {
			continue
		}

		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := gw.checkModelHealth(checkCtx, model)
		cancel()

		newStatus := ModelStatusActive
		if err != nil {
			newStatus = ModelStatusError
		}

		if model.Status != newStatus {
			model.Status = newStatus
			_ = gw.service.UpdateModelStatus(model.ID, newStatus)
		}
	}
}

// checkModelHealth 检查单个模型健康状态.
func (gw *Gateway) checkModelHealth(ctx context.Context, model *AIModel) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, model.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	if model.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+model.APIKey)
	}
	resp, err := gw.service.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("服务端错误: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Chat 统一聊天入口（兼容 OpenAI API 格式）.
func (gw *Gateway) Chat(ctx context.Context, req *GatewayChatRequest) (*GatewayChatResponse, error) {
	// 查找模型
	model, err := gw.service.GetModel(req.ModelID)
	if err != nil {
		return nil, fmt.Errorf("模型不存在: %w", err)
	}

	if !model.Enabled {
		return nil, fmt.Errorf("模型已禁用")
	}

	if model.Status == ModelStatusError {
		// 尝试故障转移
		altModel, ferr := gw.findAlternativeModel(model)
		if ferr != nil {
			return nil, fmt.Errorf("模型不可用且无备选: %w", err)
		}
		model = altModel
	}

	// 转换请求格式
	chatReq := &ChatRequest{
		ModelID:     model.ID,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}

	// 调用 Service 处理
	chatResp, _, err := gw.service.Chat(ctx, *chatReq, req.UserID, req.Username, req.IP)
	if err != nil {
		return nil, err
	}

	return &GatewayChatResponse{
		ID:               chatResp.ID,
		Model:            model.ModelName,
		Content:          chatResp.Content,
		FinishReason:     chatResp.FinishReason,
		PromptTokens:     chatResp.PromptTokens,
		CompletionTokens: chatResp.CompletionTokens,
		TotalTokens:      chatResp.TotalTokens,
	}, nil
}

// findAlternativeModel 查找备选模型.
func (gw *Gateway) findAlternativeModel(failed *AIModel) (*AIModel, error) {
	models, err := gw.service.ListModels()
	if err != nil {
		return nil, err
	}

	// 优先找同 provider 的备选
	for _, m := range models {
		if m.ID == failed.ID || !m.Enabled || m.Status != ModelStatusActive {
			continue
		}
		if m.Provider == failed.Provider {
			return m, nil
		}
	}

	// 再找任意可用模型
	for _, m := range models {
		if m.ID == failed.ID || !m.Enabled || m.Status != ModelStatusActive {
			continue
		}
		return m, nil
	}

	return nil, fmt.Errorf("无可用备选模型")
}

// ProxyOpenAI 兼容 OpenAI API 的代理端点.
func (gw *Gateway) ProxyOpenAI(w http.ResponseWriter, r *http.Request) {
	// 解析 OpenAI 格式请求
	var oaiReq struct {
		Model       string        `json:"model"`
		Messages    []ChatMessage `json:"messages"`
		Temperature float64       `json:"temperature"`
		MaxTokens   int           `json:"max_tokens,omitempty"`
		Stream      bool          `json:"stream,omitempty"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求失败", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &oaiReq); err != nil {
		http.Error(w, "解析请求失败", http.StatusBadRequest)
		return
	}

	// 通过模型名查找配置
	models, err := gw.service.ListModels()
	if err != nil {
		http.Error(w, "服务错误", http.StatusInternalServerError)
		return
	}

	var targetModel *AIModel
	for _, m := range models {
		if m.ModelName == oaiReq.Model && m.Enabled && m.Status == ModelStatusActive {
			targetModel = m
			break
		}
	}

	if targetModel == nil {
		// 尝试使用默认模型
		for _, m := range models {
			if m.IsDefault && m.Enabled && m.Status == ModelStatusActive {
				targetModel = m
				break
			}
		}
	}

	if targetModel == nil {
		http.Error(w, "无可用模型", http.StatusServiceUnavailable)
		return
	}

	// 调用聊天
	chatReq := &ChatRequest{
		ModelID:     targetModel.ID,
		Messages:    oaiReq.Messages,
		Temperature: oaiReq.Temperature,
		MaxTokens:   oaiReq.MaxTokens,
	}

	userID := r.Header.Get("X-User-ID")
	username := r.Header.Get("X-Username")
	ip := r.RemoteAddr

	ctx := r.Context()
	chatResp, _, err := gw.service.Chat(ctx, *chatReq, userID, username, ip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回 OpenAI 格式响应
	oaiResp := map[string]interface{}{
		"id":      chatResp.ID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   targetModel.ModelName,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": chatResp.Content,
				},
				"finish_reason": chatResp.FinishReason,
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     chatResp.PromptTokens,
			"completion_tokens": chatResp.CompletionTokens,
			"total_tokens":      chatResp.TotalTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(oaiResp); err != nil {
		// log error but response already started
		_ = err
	}
}

// GatewayChatRequest 网关聊天请求.
type GatewayChatRequest struct {
	ModelID     string        `json:"modelId"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"maxTokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	UserID      string        `json:"-"`
	Username    string        `json:"-"`
	IP          string        `json:"-"`
}

// GatewayChatResponse 网关聊天响应.
type GatewayChatResponse struct {
	ID               string `json:"id"`
	Model            string `json:"model"`
	Content          string `json:"content"`
	FinishReason     string `json:"finishReason,omitempty"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
}

// GetProviderManager 获取提供者管理器.
func (gw *Gateway) GetProviderManager() *ProviderManager {
	return gw.providerMgr
}

// RouteToProvider 路由请求到指定提供者.
func (gw *Gateway) RouteToProvider(ctx context.Context, provider ModelProvider, req *ProviderChatRequest) (*ProviderChatResponse, error) {
	p, err := gw.providerMgr.Get(provider)
	if err != nil {
		return nil, err
	}
	return p.Chat(ctx, req)
}

// LoadBalance 负载均衡选择模型.
func (gw *Gateway) LoadBalance(provider ModelProvider) (*AIModel, error) {
	models, err := gw.service.ListModels()
	if err != nil {
		return nil, err
	}

	var candidates []*AIModel
	for _, m := range models {
		if m.Provider == provider && m.Enabled && m.Status == ModelStatusActive {
			candidates = append(candidates, m)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("提供者 %s 无可用模型", provider)
	}

	// 简单轮询策略
	idx := time.Now().UnixNano() % int64(len(candidates))
	return candidates[idx], nil
}
