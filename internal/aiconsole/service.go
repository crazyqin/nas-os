package aiconsole

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service AI Console 业务服务.
type Service struct {
	store    *Store
	redactor *Redactor
	client   *http.Client
}

// NewService 创建服务实例.
func NewService(db *sql.DB) (*Service, error) {
	store, err := NewStore(db)
	if err != nil {
		return nil, err
	}

	redactor := NewRedactor()

	// 从数据库加载规则
	rules, err := store.ListEnabledRules()
	if err != nil {
		return nil, fmt.Errorf("加载脱敏规则失败: %w", err)
	}

	// 如果数据库没有规则，加载默认规则
	if len(rules) == 0 {
		redactor.SetDefaultRules()
		// 将默认规则写入数据库
		for _, r := range redactor.GetRules() {
			r.ID = uuid.New().String()
			r.CreatedAt = time.Now()
			r.UpdatedAt = time.Now()
			_ = store.CreateRule(r) //nolint:errcheck // 初始化失败不阻塞启动
		}
	} else {
		if err := redactor.LoadRules(rules); err != nil {
			return nil, fmt.Errorf("加载脱敏规则失败: %w", err)
		}
	}

	return &Service{
		store:    store,
		redactor: redactor,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// ==================== 模型管理 ====================

// CreateModel 创建 AI 模型配置.
func (s *Service) CreateModel(req CreateModelRequest) (*AIModel, error) {
	now := time.Now()
	m := &AIModel{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Provider:    req.Provider,
		Endpoint:    req.Endpoint,
		APIKey:      req.APIKey,
		ModelName:   req.ModelName,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Status:      ModelStatusActive,
		IsDefault:   req.IsDefault,
		Enabled:     true,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if m.MaxTokens == 0 {
		m.MaxTokens = 4096
	}
	if m.Temperature == 0 {
		m.Temperature = 0.7
	}

	// 如果设为默认，清除其他默认
	if m.IsDefault {
		if err := s.store.ClearDefault(); err != nil {
			return nil, fmt.Errorf("清除默认标记失败: %w", err)
		}
	}

	if err := s.store.CreateModel(m); err != nil {
		return nil, fmt.Errorf("创建模型失败: %w", err)
	}
	return m, nil
}

// ListModels 列出所有模型.
func (s *Service) ListModels() ([]*AIModel, error) {
	return s.store.ListModels()
}

// GetModel 获取单个模型.
func (s *Service) GetModel(id string) (*AIModel, error) {
	return s.store.GetModel(id)
}

// DeleteModel 删除模型.
func (s *Service) DeleteModel(id string) error {
	return s.store.DeleteModel(id)
}

// ==================== 脱敏规则 CRUD ====================

// CreateRule 创建脱敏规则.
func (s *Service) CreateRule(req CreateRuleRequest) (*RedactRule, error) {
	// 验证正则表达式合法性
	if _, err := validatePattern(req.Pattern); err != nil {
		return nil, fmt.Errorf("正则表达式不合法: %w", err)
	}

	now := time.Now()
	r := &RedactRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		PIIType:     req.PIIType,
		Pattern:     req.Pattern,
		Strategy:    req.Strategy,
		MaskChar:    req.MaskChar,
		ShowFirst:   req.ShowFirst,
		ShowLast:    req.ShowLast,
		Replacement: req.Replacement,
		Enabled:     req.Enabled,
		Priority:    req.Priority,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if r.MaskChar == "" {
		r.MaskChar = "*"
	}

	if err := s.store.CreateRule(r); err != nil {
		return nil, fmt.Errorf("创建规则失败: %w", err)
	}

	// 重新加载规则到脱敏引擎
	if err := s.reloadRules(); err != nil {
		return nil, fmt.Errorf("重新加载规则失败: %w", err)
	}

	return r, nil
}

// ListRules 列出所有脱敏规则.
func (s *Service) ListRules() ([]*RedactRule, error) {
	return s.store.ListRules()
}

// GetRule 获取单条规则.
func (s *Service) GetRule(id string) (*RedactRule, error) {
	return s.store.GetRule(id)
}

// UpdateRule 更新脱敏规则.
func (s *Service) UpdateRule(id string, req UpdateRuleRequest) (*RedactRule, error) {
	r, err := s.store.GetRule(id)
	if err != nil {
		return nil, fmt.Errorf("查询规则失败: %w", err)
	}
	if r == nil {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	if req.Name != nil {
		r.Name = *req.Name
	}
	if req.PIIType != nil {
		r.PIIType = *req.PIIType
	}
	if req.Pattern != nil {
		if _, err := validatePattern(*req.Pattern); err != nil {
			return nil, fmt.Errorf("正则表达式不合法: %w", err)
		}
		r.Pattern = *req.Pattern
	}
	if req.Strategy != nil {
		r.Strategy = *req.Strategy
	}
	if req.MaskChar != nil {
		r.MaskChar = *req.MaskChar
	}
	if req.ShowFirst != nil {
		r.ShowFirst = *req.ShowFirst
	}
	if req.ShowLast != nil {
		r.ShowLast = *req.ShowLast
	}
	if req.Replacement != nil {
		r.Replacement = *req.Replacement
	}
	if req.Enabled != nil {
		r.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		r.Priority = *req.Priority
	}
	if req.Description != nil {
		r.Description = *req.Description
	}
	r.UpdatedAt = time.Now()

	if err := s.store.UpdateRule(r); err != nil {
		return nil, fmt.Errorf("更新规则失败: %w", err)
	}

	if err := s.reloadRules(); err != nil {
		return nil, fmt.Errorf("重新加载规则失败: %w", err)
	}

	return r, nil
}

// DeleteRule 删除脱敏规则.
func (s *Service) DeleteRule(id string) error {
	if err := s.store.DeleteRule(id); err != nil {
		return err
	}
	return s.reloadRules()
}

// reloadRules 重新从数据库加载脱敏规则.
func (s *Service) reloadRules() error {
	rules, err := s.store.ListEnabledRules()
	if err != nil {
		return err
	}
	return s.redactor.LoadRules(rules)
}

// ==================== 聊天（自动脱敏） ====================

// Chat 发送聊天请求（自动脱敏）.
func (s *Service) Chat(ctx context.Context, req ChatRequest, userID, username, ip string) (*ChatResponse, *AuditEntry, error) {
	startTime := time.Now()

	// 查找模型
	var model *AIModel
	var err error

	if req.ModelID != "" {
		model, err = s.store.GetModel(req.ModelID)
	} else {
		model, err = s.store.GetDefaultModel()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("查询模型失败: %w", err)
	}
	if model == nil {
		return nil, nil, fmt.Errorf("未找到可用模型")
	}
	if !model.Enabled {
		return nil, nil, fmt.Errorf("模型已禁用: %s", model.Name)
	}

	// 对消息执行脱敏处理
	totalRedactCount := 0
	redacted := false
	processedMessages := make([]ChatMessage, len(req.Messages))
	copy(processedMessages, req.Messages)

	for i, msg := range processedMessages {
		result := s.redactor.Process(msg.Content)
		if result.HasRedaction {
			processedMessages[i].Content = result.Processed
			totalRedactCount += result.RedactCount
			redacted = true
		}
	}

	// 设置默认参数
	temperature := req.Temperature
	if temperature == 0 {
		temperature = model.Temperature
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = model.MaxTokens
	}

	// 调用远程 AI API
	chatResp, err := s.callRemoteAPI(ctx, model, processedMessages, temperature, maxTokens)
	duration := time.Since(startTime)

	// 构建审计条目
	audit := &AuditEntry{
		ID:           uuid.New().String(),
		Timestamp:    startTime,
		UserID:       userID,
		Username:     username,
		ModelID:      model.ID,
		ModelName:    model.Name,
		Action:       "chat",
		Redacted:     redacted,
		RedactCount:  totalRedactCount,
		DurationMs:   duration.Milliseconds(),
		IPAddress:    ip,
	}

	// 请求/响应摘要（截断前 200 字符）
	if len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1].Content
		if len(lastMsg) > 200 {
			lastMsg = lastMsg[:200] + "..."
		}
		audit.RequestSummary = lastMsg
	}

	if err != nil {
		audit.Success = false
		audit.ErrorMessage = err.Error()
		// 写审计日志（即使失败也要记录）
		_ = s.store.CreateAuditEntry(audit)
		return nil, audit, fmt.Errorf("AI API 调用失败: %w", err)
	}

	// 填充响应信息
	audit.Success = true
	audit.PromptTokens = chatResp.PromptTokens
	audit.CompletionTokens = chatResp.CompletionTokens
	audit.TotalTokens = chatResp.TotalTokens

	if len(chatResp.Content) > 200 {
		audit.ResponseSummary = chatResp.Content[:200] + "..."
	} else {
		audit.ResponseSummary = chatResp.Content
	}

	chatResp.Redacted = redacted
	chatResp.RedactCount = totalRedactCount

	// 写审计日志
	_ = s.store.CreateAuditEntry(audit)

	return chatResp, audit, nil
}

// callRemoteAPI 调用远程 AI API（OpenAI 兼容格式）.
func (s *Service) callRemoteAPI(ctx context.Context, model *AIModel, messages []ChatMessage, temperature float64, maxTokens int) (*ChatResponse, error) {
	// 构建 OpenAI 兼容请求体
	type apiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type apiRequest struct {
		Model       string       `json:"model"`
		Messages    []apiMessage `json:"messages"`
		Temperature float64      `json:"temperature"`
		MaxTokens   int          `json:"max_tokens,omitempty"`
	}

	apiMsgs := make([]apiMessage, len(messages))
	for i, m := range messages {
		apiMsgs[i] = apiMessage{Role: m.Role, Content: m.Content}
	}

	reqBody := apiRequest{
		Model:       model.ModelName,
		Messages:    apiMsgs,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 确定 endpoint
	endpoint := model.Endpoint
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	endpoint += "v1/chat/completions"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if model.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+model.APIKey)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求发送失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析 OpenAI 兼容响应
	type apiChoice struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}
	type usageInfo struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	type apiResponse struct {
		ID      string      `json:"id"`
		Choices []apiChoice `json:"choices"`
		Usage   usageInfo   `json:"usage"`
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	content := ""
	finishReason := ""
	if len(apiResp.Choices) > 0 {
		content = apiResp.Choices[0].Message.Content
		finishReason = apiResp.Choices[0].FinishReason
	}

	return &ChatResponse{
		ID:               apiResp.ID,
		ModelID:          model.ID,
		Content:          content,
		FinishReason:     finishReason,
		PromptTokens:     apiResp.Usage.PromptTokens,
		CompletionTokens: apiResp.Usage.CompletionTokens,
		TotalTokens:      apiResp.Usage.TotalTokens,
	}, nil
}

// ==================== 审计日志 ====================

// QueryAuditLogs 查询审计日志.
func (s *Service) QueryAuditLogs(filter AuditQueryFilter) ([]*AuditEntry, int64, error) {
	return s.store.QueryAuditLogs(filter)
}

// ==================== 工具函数 ====================

// validatePattern 验证正则表达式是否合法.
func validatePattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}
