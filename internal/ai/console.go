// Package ai provides AI Console for NAS-OS
// Inspired by Synology AI Console - provides unified AI capabilities with privacy protection
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ConsoleConfig represents AI Console configuration
type ConsoleConfig struct {
	// Default provider
	DefaultProvider Provider `json:"defaultProvider"`
	// Enable PII protection
	EnablePIIProtection bool `json:"enablePIIProtection"`
	// Enable audit logging
	EnableAuditLog bool `json:"enableAuditLog"`
	// Max concurrent requests
	MaxConcurrent int `json:"maxConcurrent"`
	// Request timeout
	Timeout time.Duration `json:"timeout"`
	// Enable usage tracking
	EnableUsageTracking bool `json:"enableUsageTracking"`
	// Retention period for logs
	LogRetentionDays int `json:"logRetentionDays"`
}

// DefaultConsoleConfig returns default configuration
func DefaultConsoleConfig() ConsoleConfig {
	return ConsoleConfig{
		DefaultProvider:     ProviderOpenAI,
		EnablePIIProtection: true,
		EnableAuditLog:      true,
		MaxConcurrent:       5,
		Timeout:             60 * time.Second,
		EnableUsageTracking: true,
		LogRetentionDays:    30,
	}
}

// AICapability represents available AI capabilities
type AICapability string

const (
	CapabilityChat          AICapability = "chat"
	CapabilityEmbedding     AICapability = "embedding"
	CapabilityImageGen      AICapability = "image-generation"
	CapabilityImageAnalysis AICapability = "image-analysis"
	CapabilityCodeGen       AICapability = "code-generation"
	CapabilityTranslation   AICapability = "translation"
	CapabilitySummary       AICapability = "summarization"
	CapabilitySentiment     AICapability = "sentiment-analysis"
	CapabilityOCR           AICapability = "ocr"
)

// TaskType represents predefined AI task types
type TaskType string

const (
	TaskPhotoSearch       TaskType = "photo-search"       // Smart photo search
	TaskPhotoCaption      TaskType = "photo-caption"      // Generate photo captions
	TaskDocumentSummary   TaskType = "document-summary"   // Summarize documents
	TaskVideoAnalysis     TaskType = "video-analysis"     // Analyze video content
	TaskSpeechToText      TaskType = "speech-to-text"     // Speech recognition
	TaskTextToSpeech      TaskType = "text-to-speech"     // TTS synthesis
	TaskSmartAlbum        TaskType = "smart-album"        // Create smart albums
	TaskContentModeration TaskType = "content-moderation" // Content safety check
	TaskLanguageDetect    TaskType = "language-detect"    // Detect language
)

// ConsoleRequest represents a request to AI Console
type ConsoleRequest struct {
	ID         string                 `json:"id"`
	TaskType   TaskType               `json:"taskType"`
	Capability AICapability           `json:"capability"`
	Provider   Provider               `json:"provider,omitempty"` // Override default
	Model      string                 `json:"model,omitempty"`
	Prompt     string                 `json:"prompt"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Files      []FileReference        `json:"files,omitempty"`
	Options    RequestOptions         `json:"options,omitempty"`
	UserID     string                 `json:"userId,omitempty"`
	SessionID  string                 `json:"sessionId,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// FileReference represents a file to be processed
type FileReference struct {
	Path     string `json:"path"`
	Type     string `json:"type"` // image, video, audio, document
	Encoding string `json:"encoding,omitempty"`
}

// RequestOptions represents additional options for AI requests
type RequestOptions struct {
	Temperature   float64  `json:"temperature,omitempty"`
	MaxTokens     int      `json:"maxTokens,omitempty"`
	TopP          float64  `json:"topP,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
	Language      string   `json:"language,omitempty"`
	OutputFormat  string   `json:"outputFormat,omitempty"`
}

// ConsoleResponse represents a response from AI Console
type ConsoleResponse struct {
	ID          string                 `json:"id"`
	RequestID   string                 `json:"requestId"`
	Provider    Provider               `json:"provider"`
	Model       string                 `json:"model"`
	Result      interface{}            `json:"result"`
	Confidence  float64                `json:"confidence,omitempty"`
	TokensUsed  TokenUsage             `json:"tokensUsed"`
	Duration    time.Duration          `json:"duration"`
	Warnings    []string               `json:"warnings,omitempty"`
	PIIRedacted bool                   `json:"piiRedacted"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           string                 `json:"id"`
	RequestID    string                 `json:"requestId"`
	UserID       string                 `json:"userId"`
	TaskType     TaskType               `json:"taskType"`
	Provider     Provider               `json:"provider"`
	Model        string                 `json:"model"`
	PromptHash   string                 `json:"promptHash"` // Hashed, not stored
	TokensUsed   TokenUsage             `json:"tokensUsed"`
	Duration     time.Duration          `json:"duration"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"errorMessage,omitempty"`
	PIIRedacted  bool                   `json:"piiRedacted"`
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// UsageStats represents usage statistics
type UsageStats struct {
	TotalRequests      int64              `json:"totalRequests"`
	SuccessfulRequests int64              `json:"successfulRequests"`
	FailedRequests     int64              `json:"failedRequests"`
	TotalTokens        int64              `json:"totalTokens"`
	PromptTokens       int64              `json:"promptTokens"`
	CompletionTokens   int64              `json:"completionTokens"`
	ByProvider         map[Provider]int64 `json:"byProvider"`
	ByTaskType         map[TaskType]int64 `json:"byTaskType"`
	ByUser             map[string]int64   `json:"byUser"`
	StartTime          time.Time          `json:"startTime"`
	LastUpdated        time.Time          `json:"lastUpdated"`
}

// Console is the main AI Console interface
type Console struct {
	config    ConsoleConfig
	manager   *Manager
	deider    *DeIdentifier
	audit     *AuditLogger
	usage     *UsageTracker
	providers map[Provider]ProviderInfo
	mu        sync.RWMutex
}

// ProviderInfo represents provider information
type ProviderInfo struct {
	Name         string         `json:"name"`
	Provider     Provider       `json:"provider"`
	Models       []ModelInfo    `json:"models"`
	Capabilities []AICapability `json:"capabilities"`
	Status       string         `json:"status"`
	LastChecked  time.Time      `json:"lastChecked"`
}

// ModelInfo represents model information
type ModelInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Provider    Provider       `json:"provider"`
	ContextSize int            `json:"contextSize"`
	MaxTokens   int            `json:"maxTokens"`
	Supports    []AICapability `json:"supports"`
	InputPrice  float64        `json:"inputPrice"`  // per 1K tokens
	OutputPrice float64        `json:"outputPrice"` // per 1K tokens
}

// NewConsole creates a new AI Console
func NewConsole(config ConsoleConfig) *Console {
	console := &Console{
		config:    config,
		manager:   NewManager(),
		deider:    NewDeIdentifier(DefaultDeIDRules()),
		providers: make(map[Provider]ProviderInfo),
	}

	if config.EnableAuditLog {
		console.audit = NewAuditLogger(config.LogRetentionDays)
	}

	if config.EnableUsageTracking {
		console.usage = NewUsageTracker()
	}

	return console
}

// RegisterProvider registers an AI provider
func (c *Console) RegisterProvider(info ProviderInfo, service Service, config *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.providers[info.Provider] = info
	c.manager.RegisterProvider(info.Provider, service, config)
}

// Process processes an AI request with PII protection
func (c *Console) Process(ctx context.Context, req *ConsoleRequest) (*ConsoleResponse, error) {
	startTime := time.Now()

	// Generate request ID if not provided
	if req.ID == "" {
		req.ID = generateRequestID()
	}
	req.Timestamp = startTime

	// Select provider
	provider := req.Provider
	if provider == "" {
		provider = c.config.DefaultProvider
	}

	// Apply PII protection
	var piiRedacted bool
	originalPrompt := req.Prompt
	if c.config.EnablePIIProtection {
		req.Prompt = c.deider.Process(req.Prompt)
		piiRedacted = req.Prompt != originalPrompt
	}

	// Build AI request
	aiReq := &Request{
		Messages: []Message{
			{Role: "user", Content: req.Prompt},
		},
		Model:       req.Model,
		MaxTokens:   req.Options.MaxTokens,
		Temperature: req.Options.Temperature,
	}

	// Send to provider
	resp, err := c.manager.Chat(ctx, provider, aiReq)

	// Create response
	result := &ConsoleResponse{
		ID:          generateRequestID(),
		RequestID:   req.ID,
		Provider:    provider,
		PIIRedacted: piiRedacted,
		Duration:    time.Since(startTime),
	}

	if err != nil {
		// Log failure
		if c.audit != nil {
			c.audit.Log(&AuditLog{
				ID:           generateRequestID(),
				RequestID:    req.ID,
				UserID:       req.UserID,
				TaskType:     req.TaskType,
				Provider:     provider,
				Model:        req.Model,
				Duration:     result.Duration,
				Success:      false,
				ErrorMessage: err.Error(),
				PIIRedacted:  piiRedacted,
				Timestamp:    startTime,
			})
		}
		return nil, err
	}

	// Restore PII if needed
	content := resp.Content
	if piiRedacted {
		content = c.deider.Restore(content)
	}

	result.Result = content
	result.Model = resp.Model
	result.TokensUsed = TokenUsage{
		TotalTokens: resp.TokensUsed,
	}

	// Log success
	if c.audit != nil {
		c.audit.Log(&AuditLog{
			ID:          generateRequestID(),
			RequestID:   req.ID,
			UserID:      req.UserID,
			TaskType:    req.TaskType,
			Provider:    provider,
			Model:       resp.Model,
			TokensUsed:  result.TokensUsed,
			Duration:    result.Duration,
			Success:     true,
			PIIRedacted: piiRedacted,
			Timestamp:   startTime,
		})
	}

	// Track usage
	if c.usage != nil {
		c.usage.Track(req.UserID, provider, req.TaskType, result.TokensUsed)
	}

	return result, nil
}

// QuickChat sends a simple chat message
func (c *Console) QuickChat(ctx context.Context, prompt string) (string, error) {
	resp, err := c.Process(ctx, &ConsoleRequest{
		TaskType:   TaskType(TaskPhotoSearch), // Default task
		Capability: CapabilityChat,
		Prompt:     prompt,
	})
	if err != nil {
		return "", err
	}

	if s, ok := resp.Result.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", resp.Result), nil
}

// Summarize summarizes text content
func (c *Console) Summarize(ctx context.Context, content string, maxLength int) (string, error) {
	prompt := fmt.Sprintf("Please summarize the following content concisely (max %d words):\n\n%s", maxLength, content)

	resp, err := c.Process(ctx, &ConsoleRequest{
		TaskType:   TaskDocumentSummary,
		Capability: CapabilitySummary,
		Prompt:     prompt,
		Options: RequestOptions{
			MaxTokens: maxLength * 2, // Approximate
		},
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", resp.Result), nil
}

// Translate translates text
func (c *Console) Translate(ctx context.Context, text, targetLang string) (string, error) {
	prompt := fmt.Sprintf("Translate the following text to %s:\n\n%s", targetLang, text)

	resp, err := c.Process(ctx, &ConsoleRequest{
		TaskType:   TaskLanguageDetect,
		Capability: CapabilityTranslation,
		Prompt:     prompt,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", resp.Result), nil
}

// AnalyzeSentiment analyzes text sentiment
func (c *Console) AnalyzeSentiment(ctx context.Context, text string) (SentimentResult, error) {
	prompt := fmt.Sprintf("Analyze the sentiment of the following text. Return JSON with 'sentiment' (positive/negative/neutral), 'confidence' (0-1), and 'keywords' (array):\n\n%s", text)

	resp, err := c.Process(ctx, &ConsoleRequest{
		TaskType:   TaskContentModeration,
		Capability: CapabilitySentiment,
		Prompt:     prompt,
	})
	if err != nil {
		return SentimentResult{}, err
	}

	var result SentimentResult
	if s, ok := resp.Result.(string); ok {
		if err := json.Unmarshal([]byte(s), &result); err != nil {
			return SentimentResult{}, fmt.Errorf("failed to parse sentiment result: %w", err)
		}
	}
	return result, nil
}

// SentimentResult represents sentiment analysis result
type SentimentResult struct {
	Sentiment  string   `json:"sentiment"`
	Confidence float64  `json:"confidence"`
	Keywords   []string `json:"keywords"`
}

// GetAvailableProviders returns available AI providers
func (c *Console) GetAvailableProviders() []ProviderInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]ProviderInfo, 0, len(c.providers))
	for _, info := range c.providers {
		result = append(result, info)
	}
	return result
}

// GetUsageStats returns usage statistics
func (c *Console) GetUsageStats() *UsageStats {
	if c.usage == nil {
		return nil
	}
	return c.usage.GetStats()
}

// GetAuditLogs returns audit logs
func (c *Console) GetAuditLogs(userID string, since time.Time, limit int) []AuditLog {
	if c.audit == nil {
		return nil
	}
	return c.audit.GetLogs(userID, since, limit)
}

// SetDeIDRules sets custom PII de-identification rules
func (c *Console) SetDeIDRules(rules []DeIDRule) {
	c.deider = NewDeIdentifier(rules)
}

// AddCustomRule adds a custom de-identification rule
func (c *Console) AddCustomRule(rule DeIDRule) {
	c.deider.AddRule(rule)
}

// ProcessPII processes text with PII protection only (no AI)
func (c *Console) ProcessPII(text string) (processed string, redactions []PIIRedaction) {
	processed = c.deider.Process(text)

	// Find redactions
	mappings := c.deider.GetMappings()
	for placeholder, original := range mappings {
		if strings.Contains(processed, placeholder) {
			redactions = append(redactions, PIIRedaction{
				Placeholder: placeholder,
				Type:        detectPIIType(original),
			})
		}
	}

	return processed, redactions
}

// PIIRedaction represents a PII redaction
type PIIRedaction struct {
	Placeholder string `json:"placeholder"`
	Type        string `json:"type"`
	Count       int    `json:"count,omitempty"`
}

// detectPIIType detects the type of PII
func detectPIIType(value string) string {
	if len(value) == 18 && isAllDigitsOrX(value) {
		return "id_card"
	}
	if len(value) == 11 && isAllDigits(value) {
		return "phone"
	}
	if len(value) == 16 && isAllDigits(value) {
		return "credit_card"
	}
	if strings.Contains(value, "@") && strings.Contains(value, ".") {
		return "email"
	}
	if isIPAddress(value) {
		return "ip_address"
	}
	return "unknown"
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isAllDigitsOrX(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && c != 'X' && c != 'x' {
			return false
		}
	}
	return true
}

func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		n := 0
		for _, c := range p {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else {
				return false
			}
		}
		if n > 255 {
			return false
		}
	}
	return true
}

// AuditLogger handles audit logging
type AuditLogger struct {
	logs      []AuditLog
	retention int
	mu        sync.RWMutex
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(retentionDays int) *AuditLogger {
	return &AuditLogger{
		logs:      make([]AuditLog, 0),
		retention: retentionDays,
	}
}

// Log adds an audit log entry
func (a *AuditLogger) Log(entry *AuditLog) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logs = append(a.logs, *entry)

	// Cleanup old logs
	cutoff := time.Now().AddDate(0, 0, -a.retention)
	var cleaned []AuditLog
	for _, log := range a.logs {
		if log.Timestamp.After(cutoff) {
			cleaned = append(cleaned, log)
		}
	}
	a.logs = cleaned
}

// GetLogs returns audit logs
func (a *AuditLogger) GetLogs(userID string, since time.Time, limit int) []AuditLog {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditLog
	for i := len(a.logs) - 1; i >= 0 && len(result) < limit; i-- {
		log := a.logs[i]
		if (userID == "" || log.UserID == userID) && log.Timestamp.After(since) {
			result = append(result, log)
		}
	}
	return result
}

// UsageTracker tracks usage statistics
type UsageTracker struct {
	stats UsageStats
	mu    sync.RWMutex
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		stats: UsageStats{
			ByProvider: make(map[Provider]int64),
			ByTaskType: make(map[TaskType]int64),
			ByUser:     make(map[string]int64),
			StartTime:  time.Now(),
		},
	}
}

// Track tracks usage
func (u *UsageTracker) Track(userID string, provider Provider, taskType TaskType, tokens TokenUsage) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.stats.TotalRequests++
	u.stats.TotalTokens += int64(tokens.TotalTokens)
	u.stats.PromptTokens += int64(tokens.PromptTokens)
	u.stats.CompletionTokens += int64(tokens.CompletionTokens)
	u.stats.ByProvider[provider]++
	u.stats.ByTaskType[taskType]++
	if userID != "" {
		u.stats.ByUser[userID]++
	}
	u.stats.LastUpdated = time.Now()
}

// GetStats returns usage statistics
func (u *UsageTracker) GetStats() *UsageStats {
	u.mu.RLock()
	defer u.mu.RUnlock()

	result := u.stats
	result.ByProvider = make(map[Provider]int64)
	result.ByTaskType = make(map[TaskType]int64)
	result.ByUser = make(map[string]int64)

	for k, v := range u.stats.ByProvider {
		result.ByProvider[k] = v
	}
	for k, v := range u.stats.ByTaskType {
		result.ByTaskType[k] = v
	}
	for k, v := range u.stats.ByUser {
		result.ByUser[k] = v
	}

	return &result
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// AddRule adds a rule to the de-identifier
func (d *DeIdentifier) AddRule(rule DeIDRule) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rules = append(d.rules, rule)
}
