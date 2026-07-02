package apiproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ========== OpenAI 兼容网关 ==========

// Gateway OpenAI 兼容 API 网关
// 实现 /v1/chat/completions 兼容接口，支持流式和非流式请求转发.
type Gateway struct {
	router *Router
	keyMgr *KeyManager
	client *http.Client
}

// NewGateway 创建网关实例.
func NewGateway(router *Router, keyMgr *KeyManager) *Gateway {
	return &Gateway{
		router: router,
		keyMgr: keyMgr,
		client: &http.Client{
			Timeout: 120 * time.Second, // AI 请求可能较慢
		},
	}
}

// ChatCompletions 处理 /v1/chat/completions 请求
// 认证 → 路由 → 转发 → 返回结果（支持流式和非流式）.
func (g *Gateway) ChatCompletions(ctx context.Context, apiKey string, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// 1. 验证 API Key
	keyCfg, err := g.keyMgr.Validate(apiKey)
	if err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}

	// 2. 检查模型权限
	if len(keyCfg.AllowedModels) > 0 {
		allowed := false
		for _, m := range keyCfg.AllowedModels {
			if m == req.Model {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("API Key 无权使用模型 %s", req.Model)
		}
	}

	// 3. 检查配额
	if err := g.keyMgr.CheckQuota(keyCfg.KeyID); err != nil {
		return nil, fmt.Errorf("配额检查失败: %w", err)
	}

	// 4. 路由到 provider
	provider, err := g.router.Route(req.Model)
	if err != nil {
		return nil, fmt.Errorf("路由失败: %w", err)
	}

	// 5. 转发请求
	resp, err := g.forward(ctx, provider, req)
	if err != nil {
		// 标记 provider 不健康
		g.router.MarkProviderHealth(provider.ID, false, err.Error())
		// 尝试故障转移
		resp, err = g.fallbackForward(ctx, req, provider.ID)
		if err != nil {
			return nil, fmt.Errorf("请求失败且无可用 fallback: %w", err)
		}
	} else {
		// 标记健康
		g.router.MarkProviderHealth(provider.ID, true, "")
	}

	// 6. 记录使用量
	if resp != nil {
		g.keyMgr.RecordUsage(keyCfg.KeyID, resp.Usage.TotalTokens)
	}

	// 7. 更新 Key 使用时间
	g.keyMgr.TouchKey(keyCfg.KeyID)

	return resp, nil
}

// ChatCompletionsStream 处理流式 /v1/chat/completions 请求
// 返回一个 channel，持续推送 StreamChunk.
func (g *Gateway) ChatCompletionsStream(ctx context.Context, apiKey string, req *ChatCompletionRequest) (<-chan StreamChunk, error) {
	// 1. 验证 API Key
	keyCfg, err := g.keyMgr.Validate(apiKey)
	if err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}

	// 2. 检查模型权限
	if len(keyCfg.AllowedModels) > 0 {
		allowed := false
		for _, m := range keyCfg.AllowedModels {
			if m == req.Model {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("API Key 无权使用模型 %s", req.Model)
		}
	}

	// 3. 检查配额
	if err := g.keyMgr.CheckQuota(keyCfg.KeyID); err != nil {
		return nil, fmt.Errorf("配额检查失败: %w", err)
	}

	// 4. 路由
	provider, err := g.router.Route(req.Model)
	if err != nil {
		return nil, fmt.Errorf("路由失败: %w", err)
	}

	// 5. 流式转发
	ch, totalTokens, err := g.forwardStream(ctx, provider, req)
	if err != nil {
		// 标记不健康并尝试 fallback
		g.router.MarkProviderHealth(provider.ID, false, err.Error())
		ch, totalTokens, err = g.fallbackForwardStream(ctx, req, provider.ID)
		if err != nil {
			return nil, fmt.Errorf("流式请求失败且无可用 fallback: %w", err)
		}
	} else {
		g.router.MarkProviderHealth(provider.ID, true, "")
	}

	// 6. 异步记录使用量
	go func() {
		if totalTokens > 0 {
			g.keyMgr.RecordUsage(keyCfg.KeyID, totalTokens)
		}
		g.keyMgr.TouchKey(keyCfg.KeyID)
	}()

	return ch, nil
}

// forward 转发非流式请求到 provider.
func (g *Gateway) forward(ctx context.Context, provider *AIProvider, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	url := strings.TrimSuffix(provider.Endpoint, "/") + "/v1/chat/completions"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 provider 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider 返回错误 %d: %s", resp.StatusCode, string(errBody))
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}

// forwardStream 转发流式请求到 provider
// 返回 StreamChunk channel 和估算的 token 总量.
func (g *Gateway) forwardStream(ctx context.Context, provider *AIProvider, req *ChatCompletionRequest) (<-chan StreamChunk, int, error) {
	// 强制设置 stream=true
	streamReq := *req
	streamReq.Stream = true

	url := strings.TrimSuffix(provider.Endpoint, "/") + "/v1/chat/completions"

	body, err := json.Marshal(&streamReq)
	if err != nil {
		return nil, 0, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("请求 provider 失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, 0, fmt.Errorf("provider 返回错误 %d: %s", resp.StatusCode, string(errBody))
	}

	ch := make(chan StreamChunk, 64)
	totalTokens := 0

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 256*1024), 256*1024) // 增大缓冲区

		for scanner.Scan() {
			line := scanner.Text()
			// SSE 格式: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			// 粗略统计 token（按字符数估算）
			for _, choice := range chunk.Choices {
				totalTokens += len(choice.Delta.Content) / 4
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, totalTokens, nil
}

// fallbackForward 故障转移转发（非流式）.
func (g *Gateway) fallbackForward(ctx context.Context, req *ChatCompletionRequest, excludeID string) (*ChatCompletionResponse, error) {
	provider, err := g.router.RouteWithFallback(req.Model, excludeID)
	if err != nil {
		return nil, err
	}
	return g.forward(ctx, provider, req)
}

// fallbackForwardStream 故障转移流式转发.
func (g *Gateway) fallbackForwardStream(ctx context.Context, req *ChatCompletionRequest, excludeID string) (<-chan StreamChunk, int, error) {
	provider, err := g.router.RouteWithFallback(req.Model, excludeID)
	if err != nil {
		return nil, 0, err
	}
	return g.forwardStream(ctx, provider, req)
}

// HandleHTTP 处理 HTTP 请求（可作为 http.Handler 使用）
// 路径: POST /v1/chat/completions.
func (g *Gateway) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST 请求")
		return
	}

	// 提取 API Key
	apiKey := g.extractAPIKey(r)
	if apiKey == "" {
		g.writeError(w, http.StatusUnauthorized, "invalid_api_key", "缺少 API Key")
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, "invalid_request", "请求体格式错误")
		return
	}

	if req.Stream {
		// 流式响应
		g.handleStreamHTTP(w, r, apiKey, &req)
		return
	}

	// 非流式响应
	resp, err := g.ChatCompletions(r.Context(), apiKey, &req)
	if err != nil {
		g.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleStreamHTTP 处理流式 HTTP 请求.
func (g *Gateway) handleStreamHTTP(w http.ResponseWriter, r *http.Request, apiKey string, req *ChatCompletionRequest) {
	ch, err := g.ChatCompletionsStream(r.Context(), apiKey, req)
	if err != nil {
		g.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		g.writeError(w, http.StatusInternalServerError, "internal_error", "不支持流式响应")
		return
	}

	for chunk := range ch {
		data, err := json.Marshal(chunk)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// 发送结束标记
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// extractAPIKey 从请求中提取 API Key
// 优先从 Authorization Header 提取，其次从 x-api-key Header.
func (g *Gateway) extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.Header.Get("x-api-key")
}

// writeError 写入错误响应（OpenAI 兼容格式）.
func (g *Gateway) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorBody{
			Message: message,
			Type:    "invalid_request_error",
			Code:    code,
		},
	})
}

// Close 关闭网关，释放资源.
func (g *Gateway) Close() {
	// 目前 http.Client 无需显式关闭
}
