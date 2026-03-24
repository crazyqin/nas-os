// Package ai provides AI service integration for NAS-OS
package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== DeIdentifier Tests ====================

func TestDeIdentifier_Process(t *testing.T) {
	rules := DefaultDeIDRules()
	deid := NewDeIdentifier(rules)

	tests := []struct {
		name     string
		input    string
		contains string // Should contain this placeholder
		notHas   string // Should not contain this original value
	}{
		{
			name:     "email de-identification",
			input:    "My email is test@example.com",
			contains: "[EMAIL]",
			notHas:   "test@example.com",
		},
		{
			name:     "phone de-identification",
			input:    "Call me at 13812345678",
			contains: "[PHONE]",
			notHas:   "13812345678",
		},
		{
			name:     "ID card de-identification",
			input:    "ID: 110101199001011234",
			contains: "[ID]",
			notHas:   "110101199001011234",
		},
		{
			name:     "IP address de-identification",
			input:    "Server IP: 192.168.1.100",
			contains: "[IP]",
			notHas:   "192.168.1.100",
		},
		{
			name:     "multiple sensitive data",
			input:    "Email: user@example.com, Phone: 13987654321",
			contains: "[EMAIL]",
			notHas:   "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deid.Process(tt.input)
			assert.Contains(t, result, tt.contains)
			assert.NotContains(t, result, tt.notHas)
		})
	}
}

func TestDeIdentifier_Restore(t *testing.T) {
	rules := DefaultDeIDRules()
	deid := NewDeIdentifier(rules)

	original := "My email is test@example.com and phone is 13812345678"
	processed := deid.Process(original)

	// Verify processing happened
	assert.NotEqual(t, original, processed)
	assert.Contains(t, processed, "[EMAIL]")
	assert.Contains(t, processed, "[PHONE]")

	// Restore should return original
	restored := deid.Restore(processed)
	assert.Equal(t, original, restored)
}

func TestDeIdentifier_DisabledRule(t *testing.T) {
	rules := []DeIDRule{
		{Name: "email", Pattern: `[\w.-]+@[\w.-]+\.\w+`, Replacement: "[EMAIL]", Enabled: false},
		{Name: "phone", Pattern: `\d{11}`, Replacement: "[PHONE]", Enabled: true},
	}
	deid := NewDeIdentifier(rules)

	input := "Email: test@example.com, Phone: 13812345678"
	result := deid.Process(input)

	// Email should not be de-identified (disabled)
	assert.Contains(t, result, "test@example.com")
	// Phone should be de-identified (enabled)
	assert.NotContains(t, result, "13812345678")
	assert.Contains(t, result, "[PHONE]")
}

func TestDeIdentifier_ClearMappings(t *testing.T) {
	deid := NewDeIdentifier(DefaultDeIDRules())

	// Process some data
	_ = deid.Process("test@example.com")

	// Verify mappings exist
	mappings := deid.GetMappings()
	assert.NotEmpty(t, mappings)

	// Clear mappings
	deid.ClearMappings()

	// Verify mappings cleared
	mappings = deid.GetMappings()
	assert.Empty(t, mappings)
}

func TestDeIdentifier_MultipleMatches(t *testing.T) {
	deid := NewDeIdentifier(DefaultDeIDRules())

	input := "Emails: a@example.com, b@example.com, c@example.com"
	result := deid.Process(input)

	// All emails should be replaced
	assert.NotContains(t, result, "a@example.com")
	assert.NotContains(t, result, "b@example.com")
	assert.NotContains(t, result, "c@example.com")
	assert.Contains(t, result, "[EMAIL]")
}

func TestDeIdentifier_NoSensitiveData(t *testing.T) {
	deid := NewDeIdentifier(DefaultDeIDRules())

	input := "This is a normal text without sensitive data"
	result := deid.Process(input)

	assert.Equal(t, input, result)
}

func TestDeIdentifier_InvalidPattern(t *testing.T) {
	rules := []DeIDRule{
		{Name: "invalid", Pattern: `[`, Replacement: "[INVALID]", Enabled: true}, // Invalid regex
	}
	deid := NewDeIdentifier(rules)

	// Should not panic, just return original text
	result := deid.Process("some text")
	assert.Equal(t, "some text", result)
}

// ==================== DefaultDeIDRules Tests ====================

func TestDefaultDeIDRules(t *testing.T) {
	rules := DefaultDeIDRules()

	assert.NotEmpty(t, rules)
	assert.Len(t, rules, 5) // email, phone, idcard, credit_card, ip_address

	// Verify all rules are enabled by default
	for _, rule := range rules {
		assert.True(t, rule.Enabled)
		assert.NotEmpty(t, rule.Name)
		assert.NotEmpty(t, rule.Pattern)
		assert.NotEmpty(t, rule.Replacement)
	}
}

// ==================== Manager Tests ====================

func TestNewManager(t *testing.T) {
	m := NewManager()

	assert.NotNil(t, m)
	assert.NotNil(t, m.providers)
	assert.NotNil(t, m.configs)
	assert.NotNil(t, m.deider)
}

func TestManager_RegisterProvider(t *testing.T) {
	m := NewManager()

	mockService := &MockService{}
	config := &Config{
		Provider:   ProviderOpenAI,
		APIKey:     "test-key",
		Model:      "gpt-4",
		EnableDeID: true,
	}

	m.RegisterProvider(ProviderOpenAI, mockService, config)

	providers := m.GetAvailableProviders()
	assert.Len(t, providers, 1)
	assert.Contains(t, providers, ProviderOpenAI)
}

func TestManager_Chat_ProviderNotFound(t *testing.T) {
	m := NewManager()

	req := &Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err := m.Chat(context.Background(), ProviderOpenAI, req)
	assert.Error(t, err)
	assert.Equal(t, ErrProviderNotFound, err)
}

func TestManager_Chat_WithDeIdentification(t *testing.T) {
	m := NewManager()

	mockService := &MockService{Response: "AI response"}
	config := &Config{
		Provider:   ProviderOpenAI,
		EnableDeID: true,
	}

	m.RegisterProvider(ProviderOpenAI, mockService, config)

	req := &Request{
		Messages: []Message{
			{Role: "user", Content: "My email is test@example.com"},
		},
	}

	_, err := m.Chat(context.Background(), ProviderOpenAI, req)
	require.NoError(t, err)

	// Verify the message was de-identified before sending to service
	assert.NotContains(t, mockService.LastRequest.Messages[0].Content, "test@example.com")
	assert.Contains(t, mockService.LastRequest.Messages[0].Content, "[EMAIL]")
}

func TestManager_Chat_WithoutDeIdentification(t *testing.T) {
	m := NewManager()

	mockService := &MockService{Response: "AI response"}
	config := &Config{
		Provider:   ProviderOpenAI,
		EnableDeID: false,
	}

	m.RegisterProvider(ProviderOpenAI, mockService, config)

	req := &Request{
		Messages: []Message{
			{Role: "user", Content: "My email is test@example.com"},
		},
	}

	_, err := m.Chat(context.Background(), ProviderOpenAI, req)
	require.NoError(t, err)

	// Verify the message was NOT de-identified
	assert.Contains(t, mockService.LastRequest.Messages[0].Content, "test@example.com")
}

func TestManager_GetAvailableProviders(t *testing.T) {
	m := NewManager()

	// Empty initially
	providers := m.GetAvailableProviders()
	assert.Empty(t, providers)

	// Add providers
	m.RegisterProvider(ProviderOpenAI, &MockService{}, &Config{})
	m.RegisterProvider(ProviderGoogle, &MockService{}, &Config{})

	providers = m.GetAvailableProviders()
	assert.Len(t, providers, 2)
}

// ==================== AIError Tests ====================

func TestAIError_Error(t *testing.T) {
	err := &AIError{Code: "test_error", Message: "Test error message"}
	assert.Equal(t, "Test error message", err.Error())
}

// ==================== Mock Service ====================

type MockService struct {
	Response    string
	LastRequest *Request
}

func (m *MockService) Chat(ctx context.Context, req *Request) (*Response, error) {
	m.LastRequest = req
	return &Response{
		Content:  m.Response,
		Provider: ProviderOpenAI,
	}, nil
}

func (m *MockService) StreamChat(ctx context.Context, req *Request, callback func(chunk string) error) error {
	m.LastRequest = req
	return callback(m.Response)
}

func (m *MockService) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockService) GetProvider() Provider {
	return ProviderOpenAI
}

// ==================== Config Tests ====================

func TestConfig_Defaults(t *testing.T) {
	config := &Config{
		Provider: ProviderOpenAI,
	}

	assert.Equal(t, ProviderOpenAI, config.Provider)
	assert.False(t, config.EnableDeID)
	assert.Empty(t, config.DeIDRules)
}

// ==================== Request/Response Tests ====================

func TestRequest(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
		Model:       "gpt-4",
		MaxTokens:   1000,
		Temperature: 0.7,
		Stream:      false,
	}

	assert.Len(t, req.Messages, 2)
	assert.Equal(t, "gpt-4", req.Model)
	assert.Equal(t, 1000, req.MaxTokens)
	assert.Equal(t, 0.7, req.Temperature)
	assert.False(t, req.Stream)
}

func TestResponse(t *testing.T) {
	resp := &Response{
		Content:      "Hello, how can I help?",
		Model:        "gpt-4",
		Provider:     ProviderOpenAI,
		TokensUsed:   50,
		FinishReason: "stop",
	}

	assert.Equal(t, "Hello, how can I help?", resp.Content)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Equal(t, ProviderOpenAI, resp.Provider)
	assert.Equal(t, 50, resp.TokensUsed)
}

// ==================== DeIDRule Tests ====================

func TestDeIDRule(t *testing.T) {
	rule := DeIDRule{
		Name:        "test_rule",
		Pattern:     `\d+`,
		Replacement: "[NUM]",
		Enabled:     true,
	}

	assert.Equal(t, "test_rule", rule.Name)
	assert.Equal(t, `\d+`, rule.Pattern)
	assert.Equal(t, "[NUM]", rule.Replacement)
	assert.True(t, rule.Enabled)
}

// ==================== Edge Cases ====================

func TestDeIdentifier_EmptyInput(t *testing.T) {
	deid := NewDeIdentifier(DefaultDeIDRules())

	result := deid.Process("")
	assert.Equal(t, "", result)
}

func TestDeIdentifier_ConcurrentAccess(t *testing.T) {
	deid := NewDeIdentifier(DefaultDeIDRules())

	// Run concurrent processing
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = deid.Process("test@example.com")
			_ = deid.Restore("test")
			_ = deid.GetMappings()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDeIdentifier_SpecialCharacters(t *testing.T) {
	rules := []DeIDRule{
		{Name: "special", Pattern: `[!@#$%^&*()]`, Replacement: "[SPECIAL]", Enabled: true},
	}
	deid := NewDeIdentifier(rules)

	input := "Special chars: !@#$%"
	result := deid.Process(input)

	assert.Contains(t, result, "[SPECIAL]")
}
