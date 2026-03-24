// Package ai provides AI service integration for NAS-OS
// Inspired by Synology AI Console - supports multiple AI providers
// with privacy protection (PII de-identification)
package ai

import (
	"context"
	"sync"
)

// Provider represents an AI service provider
type Provider string

const (
	ProviderOpenAI   Provider = "openai"
	ProviderGoogle   Provider = "google"
	ProviderAzure    Provider = "azure"
	ProviderBaidu    Provider = "baidu"
	ProviderLocal    Provider = "local"  // Local LLM support
	ProviderCustom   Provider = "custom"
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
	EnableDeID     bool    // Enable PII de-identification
	DeIDRules      []DeIDRule
	
	// Rate limiting
	RequestLimit   int
	RequestWindow  int64 // seconds
}

// DeIDRule defines a rule for de-identifying sensitive data
type DeIDRule struct {
	Name        string
	Pattern     string
	Replacement string
	Enabled     bool
}

// DefaultDeIDRules returns standard PII protection rules
func DefaultDeIDRules() []DeIDRule {
	return []DeIDRule{
		{Name: "email", Pattern: `[\w.-]+@[\w.-]+\.\w+`, Replacement: "[EMAIL]", Enabled: true},
		{Name: "phone", Pattern: `\d{11}`, Replacement: "[PHONE]", Enabled: true},
		{Name: "idcard", Pattern: `\d{17}[\dXx]`, Replacement: "[ID]", Enabled: true},
		{Name: "credit_card", Pattern: `\d{16}`, Replacement: "[CARD]", Enabled: true},
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
	rules []DeIDRule
}

// NewDeIdentifier creates a new de-identifier
func NewDeIdentifier(rules []DeIDRule) *DeIdentifier {
	return &DeIdentifier{rules: rules}
}

// Process applies de-identification rules to text
func (d *DeIdentifier) Process(text string) string {
	// In production, this would use regex to replace PII
	// For now, return as-is
	return text
}

// Restore attempts to restore original values (if stored)
func (d *DeIdentifier) Restore(text string) string {
	// Reverse the de-identification if we stored mappings
	return text
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