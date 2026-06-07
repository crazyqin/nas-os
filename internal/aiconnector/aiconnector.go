package aiconnector

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AIConnector AI助手连接器
type AIConnector struct {
	mu            sync.RWMutex
	providers     map[string]*Provider
	conversations map[string]*Conversation
	config        *Config
	embedder      *Embedder
}

// Provider AI提供商
type Provider struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // openai, anthropic, local, ollama
	Endpoint    string    `json:"endpoint"`
	APIKey      string    `json:"api_key"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	IsEnabled   bool      `json:"is_enabled"`
	LastUsed    time.Time `json:"last_used"`
	UsageCount  int64     `json:"usage_count"`
}

// Conversation 对话
type Conversation struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Messages   []*Message `json:"messages"`
	ProviderID string     `json:"provider_id"`
	Model      string     `json:"model"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Message 消息
type Message struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"` // user, assistant, system
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// Embedding 嵌入向量
type Embedding struct {
	ID        string                 `json:"id"`
	Text      string                 `json:"text"`
	Vector    []float64              `json:"vector"`
	FilePath  string                 `json:"file_path"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
}

// Config 配置
type Config struct {
	DefaultProvider string        `json:"default_provider"`
	DefaultModel    string        `json:"default_model"`
	CacheEnabled    bool          `json:"cache_enabled"`
	CacheTTL        time.Duration `json:"cache_ttl"`
	MaxHistory      int           `json:"max_history"`
	EmbeddingModel  string        `json:"embedding_model"`
	VectorDBPath    string        `json:"vector_db_path"`
}

// NewAIConnector 创建AI连接器
func NewAIConnector(config *Config) *AIConnector {
	return &AIConnector{
		providers:     make(map[string]*Provider),
		conversations: make(map[string]*Conversation),
		config:        config,
		embedder:      NewEmbedder(config.EmbeddingModel),
	}
}

// AddProvider 添加AI提供商
func (ac *AIConnector) AddProvider(ctx context.Context, provider *Provider) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.providers[provider.ID] = provider
	return nil
}

// GetProvider 获取提供商
func (ac *AIConnector) GetProvider(ctx context.Context, id string) (*Provider, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	provider, exists := ac.providers[id]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return provider, nil
}

// ListProviders 列出所有提供商
func (ac *AIConnector) ListProviders(ctx context.Context) []*Provider {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	var providers []*Provider
	for _, provider := range ac.providers {
		providers = append(providers, provider)
	}
	return providers
}

// CreateConversation 创建对话
func (ac *AIConnector) CreateConversation(ctx context.Context, title, providerID string) (*Conversation, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	provider, exists := ac.providers[providerID]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	conversation := &Conversation{
		ID:         generateID(),
		Title:      title,
		Messages:   make([]*Message, 0),
		ProviderID: providerID,
		Model:      provider.Model,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ac.conversations[conversation.ID] = conversation
	return conversation, nil
}

// SendMessage 发送消息
func (ac *AIConnector) SendMessage(ctx context.Context, conversationID, content string) (*Message, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	conversation, exists := ac.conversations[conversationID]
	if !exists {
		return nil, fmt.Errorf("conversation not found: %s", conversationID)
	}

	// 添加用户消息
	userMessage := &Message{
		ID:        generateID(),
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	conversation.Messages = append(conversation.Messages, userMessage)

	// 调用AI生成回复
	response, err := ac.callAI(ctx, conversation)
	if err != nil {
		return nil, err
	}

	// 添加助手消息
	assistantMessage := &Message{
		ID:        generateID(),
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	}
	conversation.Messages = append(conversation.Messages, assistantMessage)
	conversation.UpdatedAt = time.Now()

	return assistantMessage, nil
}

// SearchFiles 智能文件搜索
func (ac *AIConnector) SearchFiles(ctx context.Context, query string, limit int) ([]*SearchResult, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	// 生成查询向量
	queryVector, err := ac.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// 向量搜索
	results, err := ac.vectorSearch(ctx, queryVector, limit)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// IndexFile 索引文件
func (ac *AIConnector) IndexFile(ctx context.Context, filePath, content string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// 生成嵌入向量
	vector, err := ac.embedder.Embed(ctx, content)
	if err != nil {
		return err
	}

	// 存储到向量数据库
	embedding := &Embedding{
		ID:        generateID(),
		Text:      content,
		Vector:    vector,
		FilePath:  filePath,
		CreatedAt: time.Now(),
	}

	return ac.storeEmbedding(ctx, embedding)
}

// GetRecommendations 获取智能推荐
func (ac *AIConnector) GetRecommendations(ctx context.Context, userID string, limit int) ([]*Recommendation, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	// 分析用户行为
	behavior, err := ac.analyzeBehavior(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 生成推荐
	recommendations, err := ac.generateRecommendations(ctx, behavior, limit)
	if err != nil {
		return nil, err
	}

	return recommendations, nil
}

// SummarizeDocument 文档摘要
func (ac *AIConnector) SummarizeDocument(ctx context.Context, content string, maxLength int) (string, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	prompt := fmt.Sprintf("请为以下文档生成一个简洁的摘要，不超过%d字：\n\n%s", maxLength, content)

	response, err := ac.callAIWithPrompt(ctx, prompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

// TranslateText 翻译文本
func (ac *AIConnector) TranslateText(ctx context.Context, text, targetLang string) (string, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	prompt := fmt.Sprintf("请将以下文本翻译成%s：\n\n%s", targetLang, text)

	response, err := ac.callAIWithPrompt(ctx, prompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

// GetConversation 获取对话
func (ac *AIConnector) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	conversation, exists := ac.conversations[id]
	if !exists {
		return nil, fmt.Errorf("conversation not found: %s", id)
	}
	return conversation, nil
}

// ListConversations 列出对话
func (ac *AIConnector) ListConversations(ctx context.Context, limit int) []*Conversation {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	var conversations []*Conversation
	for _, conv := range ac.conversations {
		conversations = append(conversations, conv)
	}

	// 按更新时间排序
	sortConversationsByUpdateTime(conversations)

	if len(conversations) > limit {
		return conversations[:limit]
	}
	return conversations
}

// DeleteConversation 删除对话
func (ac *AIConnector) DeleteConversation(ctx context.Context, id string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	delete(ac.conversations, id)
	return nil
}

// GetStats 获取统计信息
func (ac *AIConnector) GetStats(ctx context.Context) map[string]interface{} {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	totalMessages := 0
	for _, conv := range ac.conversations {
		totalMessages += len(conv.Messages)
	}

	return map[string]interface{}{
		"total_providers":     len(ac.providers),
		"total_conversations": len(ac.conversations),
		"total_messages":      totalMessages,
	}
}

// 内部方法
func (ac *AIConnector) callAI(ctx context.Context, conversation *Conversation) (string, error) {
	provider, exists := ac.providers[conversation.ProviderID]
	if !exists {
		return "", fmt.Errorf("provider not found: %s", conversation.ProviderID)
	}

	// 构建请求
	request := &AIRequest{
		Model:       conversation.Model,
		Messages:    conversation.Messages,
		MaxTokens:   provider.MaxTokens,
		Temperature: provider.Temperature,
	}

	// 调用AI API
	response, err := ac.callProviderAPI(ctx, provider, request)
	if err != nil {
		return "", err
	}

	// 更新使用统计
	provider.LastUsed = time.Now()
	provider.UsageCount++

	return response, nil
}

func (ac *AIConnector) callAIWithPrompt(ctx context.Context, prompt string) (string, error) {
	providerID := ac.config.DefaultProvider
	provider, exists := ac.providers[providerID]
	if !exists {
		return "", fmt.Errorf("default provider not found: %s", providerID)
	}

	messages := []*Message{
		{
			ID:        generateID(),
			Role:      "user",
			Content:   prompt,
			Timestamp: time.Now(),
		},
	}

	request := &AIRequest{
		Model:       provider.Model,
		Messages:    messages,
		MaxTokens:   provider.MaxTokens,
		Temperature: provider.Temperature,
	}

	response, err := ac.callProviderAPI(ctx, provider, request)
	if err != nil {
		return "", err
	}

	provider.LastUsed = time.Now()
	provider.UsageCount++

	return response, nil
}

func (ac *AIConnector) callProviderAPI(ctx context.Context, provider *Provider, request *AIRequest) (string, error) {
	// 根据提供商类型调用不同的API
	switch provider.Type {
	case "openai":
		return ac.callOpenAI(ctx, provider, request)
	case "anthropic":
		return ac.callAnthropic(ctx, provider, request)
	case "ollama":
		return ac.callOllama(ctx, provider, request)
	default:
		return "", fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

func (ac *AIConnector) callOpenAI(ctx context.Context, provider *Provider, request *AIRequest) (string, error) {
	// 实现OpenAI API调用
	return "OpenAI response placeholder", nil
}

func (ac *AIConnector) callAnthropic(ctx context.Context, provider *Provider, request *AIRequest) (string, error) {
	// 实现Anthropic API调用
	return "Anthropic response placeholder", nil
}

func (ac *AIConnector) callOllama(ctx context.Context, provider *Provider, request *AIRequest) (string, error) {
	// 实现Ollama API调用
	return "Ollama response placeholder", nil
}

func (ac *AIConnector) vectorSearch(ctx context.Context, queryVector []float64, limit int) ([]*SearchResult, error) {
	// 实现向量搜索
	return nil, nil
}

func (ac *AIConnector) storeEmbedding(ctx context.Context, embedding *Embedding) error {
	// 存储嵌入向量
	return nil
}

func (ac *AIConnector) analyzeBehavior(ctx context.Context, userID string) (*UserBehavior, error) {
	// 分析用户行为
	return nil, nil
}

func (ac *AIConnector) generateRecommendations(ctx context.Context, behavior *UserBehavior, limit int) ([]*Recommendation, error) {
	// 生成推荐
	return nil, nil
}

// 辅助类型
type AIRequest struct {
	Model       string     `json:"model"`
	Messages    []*Message `json:"messages"`
	MaxTokens   int        `json:"max_tokens"`
	Temperature float64    `json:"temperature"`
}

type SearchResult struct {
	FilePath string                 `json:"file_path"`
	Score    float64                `json:"score"`
	Snippet  string                 `json:"snippet"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Recommendation struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Title  string  `json:"title"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

type UserBehavior struct {
	RecentFiles    []string               `json:"recent_files"`
	FrequentTopics []string               `json:"frequent_topics"`
	Preferences    map[string]interface{} `json:"preferences"`
}

// 辅助函数
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func sortConversationsByUpdateTime(conversations []*Conversation) {
	for i := 0; i < len(conversations); i++ {
		for j := i + 1; j < len(conversations); j++ {
			if conversations[i].UpdatedAt.Before(conversations[j].UpdatedAt) {
				conversations[i], conversations[j] = conversations[j], conversations[i]
			}
		}
	}
}
