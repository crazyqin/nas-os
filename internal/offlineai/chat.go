// Package offlineai 对话引擎，支持多轮对话、历史管理和流式响应
package offlineai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ChatEngine 对话引擎
type ChatEngine struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	engine        *Engine
	conversations map[string]*Conversation
	maxHistory    int
}

// NewChatEngine 创建对话引擎
func NewChatEngine(logger *zap.Logger, engine *Engine, maxHistory int) *ChatEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &ChatEngine{
		logger:        logger,
		engine:        engine,
		conversations: make(map[string]*Conversation),
		maxHistory:    maxHistory,
	}
}

// generateMsgID 生成消息 ID
func generateMsgID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateConversation 创建新对话
func (ce *ChatEngine) CreateConversation(modelName string) *Conversation {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	conv := &Conversation{
		ID:        generateMsgID(),
		Messages:  make([]Message, 0),
		ModelName: modelName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	ce.conversations[conv.ID] = conv

	ce.logger.Debug("conversation created", zap.String("id", conv.ID))
	return conv
}

// GetConversation 获取对话
func (ce *ChatEngine) GetConversation(id string) (*Conversation, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	conv, exists := ce.conversations[id]
	if !exists {
		return nil, fmt.Errorf("conversation %s not found", id)
	}
	return conv, nil
}

// DeleteConversation 删除对话
func (ce *ChatEngine) DeleteConversation(id string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if _, exists := ce.conversations[id]; !exists {
		return fmt.Errorf("conversation %s not found", id)
	}
	delete(ce.conversations, id)
	return nil
}

// ListConversations 列出所有对话
func (ce *ChatEngine) ListConversations() []*Conversation {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	result := make([]*Conversation, 0, len(ce.conversations))
	for _, conv := range ce.conversations {
		result = append(result, conv)
	}
	return result
}

// SendMessage 发送消息并获取回复
func (ce *ChatEngine) SendMessage(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// 获取或创建对话
	var conv *Conversation
	var err error

	if req.ConversationID != "" {
		conv, err = ce.GetConversation(req.ConversationID)
		if err != nil {
			return nil, err
		}
	} else {
		conv = ce.CreateConversation(req.ModelName)
	}

	// 添加用户消息
	userMsg := Message{
		ID:        generateMsgID(),
		Role:      RoleUser,
		Content:   req.Message,
		Tokens:    estimateTokens(req.Message),
		Timestamp: time.Now(),
	}

	ce.mu.Lock()
	conv.Messages = append(conv.Messages, userMsg)
	conv.TotalTokens += userMsg.Tokens
	conv.UpdatedAt = time.Now()
	ce.mu.Unlock()

	// 构建推理提示（包含对话历史）
	prompt := ce.buildPrompt(conv)

	// 执行推理
	inferReq := &InferRequest{
		Prompt:    prompt,
		ModelName: req.ModelName,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	resp, err := ce.engine.Infer(ctx, inferReq)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// 添加助手回复
	assistantMsg := Message{
		ID:        generateMsgID(),
		Role:      RoleAssistant,
		Content:   resp.Text,
		Tokens:    resp.TokensUsed,
		Timestamp: time.Now(),
	}

	ce.mu.Lock()
	conv.Messages = append(conv.Messages, assistantMsg)
	conv.TotalTokens += resp.TokensUsed
	conv.UpdatedAt = time.Now()

	// 裁剪历史
	ce.trimHistory(conv)
	ce.mu.Unlock()

	return &ChatResponse{
		ConversationID: conv.ID,
		Reply:          resp.Text,
		TokensUsed:     resp.TokensUsed,
		Duration:       time.Since(start),
	}, nil
}

// buildPrompt 构建包含历史的推理提示
func (ce *ChatEngine) buildPrompt(conv *Conversation) string {
	var sb strings.Builder

	// 添加系统提示
	sb.WriteString("You are a helpful AI assistant running on NAS device.\n\n")

	// 添加历史消息
	for _, msg := range conv.Messages {
		switch msg.Role {
		case RoleSystem:
			sb.WriteString(fmt.Sprintf("System: %s\n", msg.Content))
		case RoleUser:
			sb.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
		case RoleAssistant:
			sb.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content))
		}
	}

	sb.WriteString("Assistant: ")
	return sb.String()
}

// trimHistory 裁剪对话历史，保留最新消息
func (ce *ChatEngine) trimHistory(conv *Conversation) {
	if len(conv.Messages) > ce.maxHistory*2 {
		conv.Messages = conv.Messages[len(conv.Messages)-ce.maxHistory*2:]
	}
}

// StreamChat 流式对话（返回 channel）
func (ce *ChatEngine) StreamChat(ctx context.Context, req *ChatRequest) (<-chan *StreamChunk, error) {
	req.Stream = true

	ch := make(chan *StreamChunk, 64)

	go func() {
		defer close(ch)

		resp, err := ce.SendMessage(ctx, req)
		if err != nil {
			ch <- &StreamChunk{Text: fmt.Sprintf("Error: %v", err), Done: true}
			return
		}

		// 模拟流式输出：逐字输出
		runes := []rune(resp.Reply)
		for i, r := range runes {
			select {
			case <-ctx.Done():
				return
			case ch <- &StreamChunk{
				Text: string(r),
				Done: i == len(runes)-1,
			}:
			}
		}
	}()

	return ch, nil
}
