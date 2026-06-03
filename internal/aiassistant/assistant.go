package aiassistant

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// AIProvider AI提供商
type AIProvider string

const (
	ProviderLocal   AIProvider = "LOCAL"
	ProviderOpenAI  AIProvider = "OPENAI"
	ProviderClaude  AIProvider = "CLAUDE"
	ProviderCustom  AIProvider = "CUSTOM"
)

// TaskType 任务类型
type TaskType string

const (
	TaskSummarize    TaskType = "SUMMARIZE"
	TaskWrite        TaskType = "WRITE"
	TaskTranslate    TaskType = "TRANSLATE"
	TaskCodeReview   TaskType = "CODE_REVIEW"
	TaskDataAnalysis TaskType = "DATA_ANALYSIS"
	TaskEmailDraft   TaskType = "EMAIL_DRAFT"
	TaskDocSearch    TaskType = "DOC_SEARCH"
)

// AIMessage AI消息
type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIRequest AI请求
type AIRequest struct {
	Task     TaskType     `json:"task"`
	Provider AIProvider   `json:"provider"`
	Messages []AIMessage  `json:"messages"`
	Context  string       `json:"context"`
	Model    string       `json:"model,omitempty"`
	MaxTokens int        `json:"max_tokens,omitempty"`
}

// AIResponse AI响应
type AIResponse struct {
	Content    string        `json:"content"`
	TokensUsed int           `json:"tokens_used"`
	Duration   time.Duration `json:"duration"`
	Provider   AIProvider    `json:"provider"`
	Model      string        `json:"model"`
	Cached     bool          `json:"cached"`
}

// Document 文档信息
type Document struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	Summary  string    `json:"summary,omitempty"`
	Tags     []string  `json:"tags,omitempty"`
}

// AIAssistant AI办公助手
type AIAssistant struct {
	providers  map[AIProvider]*ProviderConfig
	documents  map[string]*Document
	cache      map[string]*AIResponse
	mu         sync.RWMutex
	dataPath   string
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	Provider AIProvider `json:"provider"`
	Endpoint string     `json:"endpoint"`
	APIKey   string     `json:"api_key,omitempty"`
	Model    string     `json:"model"`
	Enabled  bool       `json:"enabled"`
}

// NewAIAssistant 创建AI助手
func NewAIAssistant(dataPath string) *AIAssistant {
	os.MkdirAll(dataPath, 0755)
	a := &AIAssistant{
		providers: make(map[AIProvider]*ProviderConfig),
		documents: make(map[string]*Document),
		cache:     make(map[string]*AIResponse),
		dataPath:  dataPath,
	}
	a.loadState()
	return a
}

// ConfigureProvider 配置AI提供商
func (a *AIAssistant) ConfigureProvider(config *ProviderConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers[config.Provider] = config
	a.saveState()
}

// ProcessRequest 处理AI请求
func (a *AIAssistant) ProcessRequest(req *AIRequest) (*AIResponse, error) {
	a.mu.RLock()
	config, ok := a.providers[req.Provider]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", req.Provider)
	}
	if !config.Enabled {
		return nil, fmt.Errorf("provider %s is disabled", req.Provider)
	}
	start := time.Now()
	response := &AIResponse{
		Provider: req.Provider,
		Model:    config.Model,
		Duration: time.Since(start),
	}
	switch req.Task {
	case TaskSummarize:
		response.Content = fmt.Sprintf("[AI摘要] 基于%d条消息生成的摘要内容", len(req.Messages))
	case TaskWrite:
		response.Content = fmt.Sprintf("[AI写作] 基于上下文生成的文档内容")
	case TaskTranslate:
		response.Content = fmt.Sprintf("[AI翻译] 翻译结果")
	case TaskCodeReview:
		response.Content = fmt.Sprintf("[AI代码审查] 代码审查意见")
	case TaskDataAnalysis:
		response.Content = fmt.Sprintf("[AI数据分析] 数据分析报告")
	case TaskEmailDraft:
		response.Content = fmt.Sprintf("[AI邮件] 邮件草稿")
	case TaskDocSearch:
		response.Content = fmt.Sprintf("[AI文档搜索] 搜索结果")
	}
	return response, nil
}

// SummarizeDocument 文档摘要
func (a *AIAssistant) SummarizeDocument(docPath string, provider AIProvider) (string, error) {
	req := &AIRequest{
		Task:     TaskSummarize,
		Provider: provider,
		Messages: []AIMessage{{Role: "user", Content: fmt.Sprintf("请总结文档: %s", docPath)}},
	}
	resp, err := a.ProcessRequest(req)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	if doc, ok := a.documents[docPath]; ok {
		doc.Summary = resp.Content
	}
	a.mu.Unlock()
	return resp.Content, nil
}

// SearchDocuments 语义搜索文档
func (a *AIAssistant) SearchDocuments(query string, limit int) []*Document {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var results []*Document
	for _, doc := range a.documents {
		if len(results) >= limit {
			break
		}
		results = append(results, doc)
	}
	return results
}

// RegisterDocument 注册文档
func (a *AIAssistant) RegisterDocument(doc *Document) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.documents[doc.ID] = doc
	a.saveState()
}

// GetProviders 获取提供商列表
func (a *AIAssistant) GetProviders() []*ProviderConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var configs []*ProviderConfig
	for _, c := range a.providers {
		configs = append(configs, c)
	}
	return configs
}

// GetDocuments 获取文档列表
func (a *AIAssistant) GetDocuments() []*Document {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var docs []*Document
	for _, d := range a.documents {
		docs = append(docs, d)
	}
	return docs
}

func (a *AIAssistant) saveState() {
	state := struct {
		Providers map[AIProvider]*ProviderConfig `json:"providers"`
		Documents map[string]*Document          `json:"documents"`
	}{a.providers, a.documents}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(a.dataPath+"/state.json", data, 0644)
}

func (a *AIAssistant) loadState() {
	data, err := os.ReadFile(a.dataPath + "/state.json")
	if err != nil {
		return
	}
	var state struct {
		Providers map[AIProvider]*ProviderConfig `json:"providers"`
		Documents map[string]*Document          `json:"documents"`
	}
	json.Unmarshal(data, &state)
	if state.Providers != nil {
		a.providers = state.Providers
	}
	if state.Documents != nil {
		a.documents = state.Documents
	}
}
