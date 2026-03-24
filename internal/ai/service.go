// Package ai provides AI service integration for NAS-OS
// Inspired by Synology AI Console - supports multiple AI providers
// with privacy protection (PII de-identification)
package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Provider represents an AI service provider
type Provider string

// AI provider constants
const (
	ProviderOpenAI Provider = "openai"
	ProviderGoogle Provider = "google"
	ProviderAzure  Provider = "azure"
	ProviderBaidu  Provider = "baidu"
	ProviderLocal  Provider = "local" // Local LLM support
	ProviderCustom Provider = "custom"
)

// Config holds AI service configuration
type Config struct {
	Provider    Provider
	APIKey      string
	Endpoint    string
	Model       string
	MaxTokens   int
	Temperature float64

	// Privacy settings
	EnableDeID bool // Enable PII de-identification
	DeIDRules  []DeIDRule

	// Rate limiting
	RequestLimit  int
	RequestWindow int64 // seconds
}

// DeIDRule defines a rule for de-identifying sensitive data
type DeIDRule struct {
	Name        string
	Pattern     string
	Replacement string
	Enabled     bool
}

// DefaultDeIDRules returns standard PII protection rules
// 注意：规则顺序很重要，更长/更精确的模式应该放在前面
func DefaultDeIDRules() []DeIDRule {
	return []DeIDRule{
		{Name: "idcard", Pattern: `\d{17}[\dXx]`, Replacement: "[ID]", Enabled: true},       // 身份证 18 位，放在最前面
		{Name: "credit_card", Pattern: `\d{16}`, Replacement: "[CARD]", Enabled: true},     // 信用卡 16 位
		{Name: "phone", Pattern: `\d{11}`, Replacement: "[PHONE]", Enabled: true},          // 手机号 11 位
		{Name: "email", Pattern: `[\w.-]+@[\w.-]+\.\w+`, Replacement: "[EMAIL]", Enabled: true},
		{Name: "ip_address", Pattern: `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`, Replacement: "[IP]", Enabled: true},
	}
}

// Message represents a chat message
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// Request represents an AI request
type Request struct {
	Messages    []Message
	Model       string
	MaxTokens   int
	Temperature float64
	Stream      bool
}

// Response represents an AI response
type Response struct {
	Content      string
	Model        string
	Provider     Provider
	TokensUsed   int
	FinishReason string
}

// Service defines the AI service interface
type Service interface {
	// Chat sends a chat request and returns the response
	Chat(ctx context.Context, req *Request) (*Response, error)

	// StreamChat streams the response
	StreamChat(ctx context.Context, req *Request, callback func(chunk string) error) error

	// Embed generates embeddings for text
	Embed(ctx context.Context, text string) ([]float32, error)

	// GetProvider returns the provider name
	GetProvider() Provider
}

// Manager manages multiple AI providers
type Manager struct {
	mu        sync.RWMutex
	providers map[Provider]Service
	configs   map[Provider]*Config
	deider    *DeIdentifier
}

// NewManager creates a new AI manager
func NewManager() *Manager {
	return &Manager{
		providers: make(map[Provider]Service),
		configs:   make(map[Provider]*Config),
		deider:    NewDeIdentifier(DefaultDeIDRules()),
	}
}

// RegisterProvider adds an AI provider
func (m *Manager) RegisterProvider(provider Provider, service Service, config *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[provider] = service
	m.configs[provider] = config
}

// Chat sends a request to the specified provider
func (m *Manager) Chat(ctx context.Context, provider Provider, req *Request) (*Response, error) {
	m.mu.RLock()
	service, exists := m.providers[provider]
	config := m.configs[provider]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrProviderNotFound
	}

	// Apply de-identification if enabled
	if config != nil && config.EnableDeID {
		for i, msg := range req.Messages {
			req.Messages[i].Content = m.deider.Process(msg.Content)
		}
	}

	return service.Chat(ctx, req)
}

// GetAvailableProviders returns list of available providers
func (m *Manager) GetAvailableProviders() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]Provider, 0, len(m.providers))
	for p := range m.providers {
		providers = append(providers, p)
	}
	return providers
}

// DeIdentifier handles PII de-identification
type DeIdentifier struct {
	rules    []DeIDRule
	mappings map[string]string // placeholder -> original value
	mu       sync.RWMutex
}

// NewDeIdentifier creates a new de-identifier
func NewDeIdentifier(rules []DeIDRule) *DeIdentifier {
	return &DeIdentifier{
		rules:    rules,
		mappings: make(map[string]string),
	}
}

// Process applies de-identification rules to text
// Replaces sensitive information with placeholders and stores mappings for restoration
func (d *DeIdentifier) Process(text string) string {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := text
	for _, rule := range d.rules {
		if !rule.Enabled {
			continue
		}
		result = d.applyRule(result, rule)
	}
	return result
}

// applyRule applies a single de-identification rule
func (d *DeIdentifier) applyRule(text string, rule DeIDRule) string {
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return text
	}

	matches := re.FindAllString(text, -1)
	for i, match := range matches {
		if match == "" {
			continue
		}
		// Generate unique placeholder
		placeholder := fmt.Sprintf("%s_%d", rule.Replacement, i)
		// Store mapping for potential restoration
		d.mappings[placeholder] = match
		// Replace first occurrence only to avoid replacing wrong content
		text = strings.Replace(text, match, placeholder, 1)
	}
	return text
}

// Restore attempts to restore original values using stored mappings
func (d *DeIdentifier) Restore(text string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := text
	for placeholder, original := range d.mappings {
		result = strings.ReplaceAll(result, placeholder, original)
	}
	return result
}

// ClearMappings clears stored mappings (call after processing is complete)
func (d *DeIdentifier) ClearMappings() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mappings = make(map[string]string)
}

// GetMappings returns current mappings (for debugging/logging)
func (d *DeIdentifier) GetMappings() map[string]string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make(map[string]string, len(d.mappings))
	for k, v := range d.mappings {
		result[k] = v
	}
	return result
}

// Errors
var (
	ErrProviderNotFound = &AIError{Code: "provider_not_found", Message: "AI provider not found"}
	ErrRequestFailed    = &AIError{Code: "request_failed", Message: "AI request failed"}
	ErrRateLimited      = &AIError{Code: "rate_limited", Message: "Rate limit exceeded"}
)

// AIError represents an AI service error
type AIError struct {
	Code    string
	Message string
}

func (e *AIError) Error() string {
	return e.Message
}
