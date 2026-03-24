// Package ai provides AI Console tests
package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Console Core Tests ====================

func TestNewConsole(t *testing.T) {
	config := DefaultConsoleConfig()
	console := NewConsole(config)

	require.NotNil(t, console)
	assert.NotNil(t, console.manager)
	assert.NotNil(t, console.deider)
	assert.NotNil(t, console.audit)
	assert.NotNil(t, console.usage)
}

func TestNewConsole_NoAudit(t *testing.T) {
	config := DefaultConsoleConfig()
	config.EnableAuditLog = false
	console := NewConsole(config)

	require.NotNil(t, console)
	assert.Nil(t, console.audit)
}

func TestNewConsole_NoUsageTracking(t *testing.T) {
	config := DefaultConsoleConfig()
	config.EnableUsageTracking = false
	console := NewConsole(config)

	require.NotNil(t, console)
	assert.Nil(t, console.usage)
}

// ==================== Default Config Tests ====================

func TestDefaultConsoleConfig(t *testing.T) {
	config := DefaultConsoleConfig()

	assert.Equal(t, ProviderOpenAI, config.DefaultProvider)
	assert.True(t, config.EnablePIIProtection)
	assert.True(t, config.EnableAuditLog)
	assert.Equal(t, 5, config.MaxConcurrent)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.True(t, config.EnableUsageTracking)
	assert.Equal(t, 30, config.LogRetentionDays)
}

// ==================== Provider Registration Tests ====================

func TestConsole_RegisterProvider(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	info := ProviderInfo{
		Name:     "Test Provider",
		Provider: ProviderOpenAI,
		Models: []ModelInfo{
			{
				ID:          "gpt-4",
				Name:        "GPT-4",
				Provider:    ProviderOpenAI,
				ContextSize: 8192,
				MaxTokens:   4096,
			},
		},
		Capabilities: []AICapability{CapabilityChat, CapabilityEmbedding},
		Status:       "active",
	}

	// Register with mock service
	mockService := &mockAIService{}
	console.RegisterProvider(info, mockService, &Config{APIKey: "test-key"})

	providers := console.GetAvailableProviders()
	assert.Len(t, providers, 1)
	assert.Equal(t, "Test Provider", providers[0].Name)
}

// ==================== PII Protection Tests ====================

func TestConsole_ProcessPII(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	processed, redactions := console.ProcessPII("手机号13812345678和邮箱test@example.com")

	assert.NotContains(t, processed, "13812345678")
	assert.NotContains(t, processed, "test@example.com")
	assert.NotEmpty(t, redactions)
}

func TestConsole_ProcessPII_Disabled(t *testing.T) {
	config := DefaultConsoleConfig()
	config.EnablePIIProtection = false
	console := NewConsole(config)

	// Even with protection disabled, ProcessPII should still work
	processed, redactions := console.ProcessPII("手机号13812345678")

	assert.NotContains(t, processed, "13812345678")
	assert.NotEmpty(t, redactions)
}

func TestConsole_AddCustomRule(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	customRule := DeIDRule{
		Name:        "Custom ID",
		Pattern:     `CUSTOM-\d+`,
		Replacement: "[CUSTOM_ID]",
		Enabled:     true,
	}

	console.AddCustomRule(customRule)

	processed, _ := console.ProcessPII("Found CUSTOM-12345")
	assert.Contains(t, processed, "[CUSTOM_ID]")
}

// ==================== Audit Logger Tests ====================

func TestAuditLogger_Log(t *testing.T) {
	logger := NewAuditLogger(30)

	entry := &AuditLog{
		ID:         "log1",
		RequestID:  "req1",
		UserID:     "user1",
		TaskType:   TaskPhotoSearch,
		Provider:   ProviderOpenAI,
		Model:      "gpt-4",
		TokensUsed: TokenUsage{TotalTokens: 100},
		Duration:   500 * time.Millisecond,
		Success:    true,
		Timestamp:  time.Now(),
	}

	logger.Log(entry)

	logs := logger.GetLogs("", time.Time{}, 10)
	assert.Len(t, logs, 1)
	assert.Equal(t, "req1", logs[0].RequestID)
}

func TestAuditLogger_GetLogs_FilterByUser(t *testing.T) {
	logger := NewAuditLogger(30)

	logger.Log(&AuditLog{ID: "1", RequestID: "r1", UserID: "user1", Timestamp: time.Now()})
	logger.Log(&AuditLog{ID: "2", RequestID: "r2", UserID: "user2", Timestamp: time.Now()})
	logger.Log(&AuditLog{ID: "3", RequestID: "r3", UserID: "user1", Timestamp: time.Now()})

	logs := logger.GetLogs("user1", time.Time{}, 10)
	assert.Len(t, logs, 2)
}

func TestAuditLogger_GetLogs_FilterByTime(t *testing.T) {
	logger := NewAuditLogger(30)

	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now().Add(-30 * time.Minute)

	logger.Log(&AuditLog{ID: "1", RequestID: "r1", Timestamp: oldTime})
	logger.Log(&AuditLog{ID: "2", RequestID: "r2", Timestamp: recentTime})

	logs := logger.GetLogs("", time.Now().Add(-1*time.Hour), 10)
	assert.Len(t, logs, 1)
	assert.Equal(t, "r2", logs[0].RequestID)
}

func TestAuditLogger_Retention(t *testing.T) {
	logger := NewAuditLogger(1) // 1 day retention

	// Log old entry
	oldEntry := &AuditLog{
		ID:        "old",
		RequestID: "r1",
		Timestamp:  time.Now().Add(-2 * 24 * time.Hour),
	}
	logger.Log(oldEntry)

	// Log new entry
	newEntry := &AuditLog{
		ID:        "new",
		RequestID: "r2",
		Timestamp:  time.Now(),
	}
	logger.Log(newEntry)

	// Old entry should be cleaned up
	logs := logger.GetLogs("", time.Time{}, 10)
	assert.Len(t, logs, 1)
	assert.Equal(t, "r2", logs[0].RequestID)
}

// ==================== Usage Tracker Tests ====================

func TestUsageTracker_Track(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{
		PromptTokens:     50,
		CompletionTokens: 100,
		TotalTokens:      150,
	})

	stats := tracker.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(150), stats.TotalTokens)
	assert.Equal(t, int64(50), stats.PromptTokens)
	assert.Equal(t, int64(100), stats.CompletionTokens)
}

func TestUsageTracker_TrackByProvider(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 100})
	tracker.Track("user1", ProviderGoogle, TaskDocumentSummary, TokenUsage{TotalTokens: 200})
	tracker.Track("user2", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 50})

	stats := tracker.GetStats()
	assert.Equal(t, int64(2), stats.ByProvider[ProviderOpenAI])
	assert.Equal(t, int64(1), stats.ByProvider[ProviderGoogle])
}

func TestUsageTracker_TrackByTaskType(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 100})
	tracker.Track("user1", ProviderOpenAI, TaskDocumentSummary, TokenUsage{TotalTokens: 200})
	tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 50})

	stats := tracker.GetStats()
	assert.Equal(t, int64(2), stats.ByTaskType[TaskPhotoSearch])
	assert.Equal(t, int64(1), stats.ByTaskType[TaskDocumentSummary])
}

func TestUsageTracker_TrackByUser(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 100})
	tracker.Track("user2", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 200})
	tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 50})

	stats := tracker.GetStats()
	assert.Equal(t, int64(2), stats.ByUser["user1"])
	assert.Equal(t, int64(1), stats.ByUser["user2"])
}

// ==================== Token Usage Tests ====================

func TestTokenUsage(t *testing.T) {
	usage := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	assert.Equal(t, 100, usage.PromptTokens)
	assert.Equal(t, 50, usage.CompletionTokens)
	assert.Equal(t, 150, usage.TotalTokens)
}

// ==================== Request/Response Tests ====================

func TestConsoleRequest_IDGeneration(t *testing.T) {
	req := &ConsoleRequest{
		TaskType:   TaskPhotoSearch,
		Capability: CapabilityChat,
		Prompt:     "Test prompt",
	}

	assert.Empty(t, req.ID)
	// ID would be generated during Process()
}

func TestConsoleResponse(t *testing.T) {
	resp := &ConsoleResponse{
		ID:          "resp1",
		RequestID:   "req1",
		Provider:    ProviderOpenAI,
		Model:       "gpt-4",
		Result:      "Test result",
		TokensUsed:  TokenUsage{TotalTokens: 100},
		Duration:    500 * time.Millisecond,
		PIIRedacted: true,
	}

	assert.Equal(t, "resp1", resp.ID)
	assert.True(t, resp.PIIRedacted)
}

// ==================== Sentiment Analysis Tests ====================

func TestSentimentResult(t *testing.T) {
	result := SentimentResult{
		Sentiment:  "positive",
		Confidence: 0.85,
		Keywords:   []string{"great", "excellent"},
	}

	assert.Equal(t, "positive", result.Sentiment)
	assert.Equal(t, 0.85, result.Confidence)
	assert.Len(t, result.Keywords, 2)
}

// ==================== Capability Tests ====================

func TestAICapabilities(t *testing.T) {
	capabilities := []AICapability{
		CapabilityChat,
		CapabilityEmbedding,
		CapabilityImageGen,
		CapabilityImageAnalysis,
		CapabilityCodeGen,
		CapabilityTranslation,
		CapabilitySummary,
		CapabilitySentiment,
		CapabilityOCR,
	}

	for _, cap := range capabilities {
		assert.NotEmpty(t, string(cap))
	}
}

// ==================== Task Type Tests ====================

func TestTaskTypes(t *testing.T) {
	taskTypes := []TaskType{
		TaskPhotoSearch,
		TaskPhotoCaption,
		TaskDocumentSummary,
		TaskVideoAnalysis,
		TaskSpeechToText,
		TaskTextToSpeech,
		TaskSmartAlbum,
		TaskContentModeration,
		TaskLanguageDetect,
	}

	for _, tt := range taskTypes {
		assert.NotEmpty(t, string(tt))
	}
}

// ==================== PII Type Detection Tests ====================

func TestDetectPIIType(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"id_card", "110101199001011234", "id_card"},
		{"phone", "13812345678", "phone"},
		{"credit_card", "6222021234567890", "credit_card"},
		{"email", "test@example.com", "email"},
		{"ip_address", "192.168.1.1", "ip_address"},
		{"unknown", "random text", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectPIIType(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ==================== Helper Function Tests ====================

func TestIsAllDigits(t *testing.T) {
	assert.True(t, isAllDigits("1234567890"))
	assert.True(t, isAllDigits("0"))
	assert.False(t, isAllDigits("123abc"))
	assert.False(t, isAllDigits("abc"))
}

func TestIsAllDigitsOrX(t *testing.T) {
	assert.True(t, isAllDigitsOrX("1234567890"))
	assert.True(t, isAllDigitsOrX("12345X"))
	assert.True(t, isAllDigitsOrX("12345x"))
	assert.False(t, isAllDigitsOrX("123abc"))
}

func TestIsIPAddress(t *testing.T) {
	assert.True(t, isIPAddress("192.168.1.1"))
	assert.True(t, isIPAddress("0.0.0.0"))
	assert.True(t, isIPAddress("255.255.255.255"))
	assert.False(t, isIPAddress("256.1.1.1"))
	assert.False(t, isIPAddress("192.168.1"))
	assert.False(t, isIPAddress("abc.def.ghi.jkl"))
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2, "Request IDs should be unique")
	assert.Contains(t, id1, "req_")
}

// ==================== Provider Info Tests ====================

func TestProviderInfo(t *testing.T) {
	info := ProviderInfo{
		Name:     "Test Provider",
		Provider: ProviderOpenAI,
		Models: []ModelInfo{
			{
				ID:          "gpt-4",
				Name:        "GPT-4",
				Provider:    ProviderOpenAI,
				ContextSize: 8192,
				MaxTokens:   4096,
				Supports:    []AICapability{CapabilityChat, CapabilityEmbedding},
				InputPrice:  0.03,
				OutputPrice: 0.06,
			},
		},
		Capabilities: []AICapability{CapabilityChat},
		Status:       "active",
	}

	assert.Equal(t, "Test Provider", info.Name)
	assert.Len(t, info.Models, 1)
	assert.Equal(t, "gpt-4", info.Models[0].ID)
}

// ==================== Mock Service for Testing ====================

type mockAIService struct{}

func (m *mockAIService) Chat(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		Content:    "Mock response",
		Model:      req.Model,
		TokensUsed: 100,
	}, nil
}

func (m *mockAIService) Complete(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		Content:    "Mock completion",
		Model:      req.Model,
		TokensUsed: 50,
	}, nil
}

func (m *mockAIService) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockAIService) StreamChat(ctx context.Context, req *Request, callback func(chunk string) error) error {
	return nil
}

func (m *mockAIService) GetProvider() Provider {
	return ProviderOpenAI
}

// ==================== Console Process Tests ====================

func TestConsole_Process(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	// Register mock provider
	console.RegisterProvider(ProviderInfo{
		Name:     "Mock",
		Provider: ProviderOpenAI,
		Status:   "active",
	}, &mockAIService{}, &Config{APIKey: "test"})

	req := &ConsoleRequest{
		TaskType:   TaskPhotoSearch,
		Capability: CapabilityChat,
		Prompt:     "Test prompt",
		Model:      "gpt-4",
	}

	resp, err := console.Process(context.Background(), req)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.NotEmpty(t, resp.RequestID)
	assert.Equal(t, ProviderOpenAI, resp.Provider)
}

func TestConsole_Process_WithPII(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	console.RegisterProvider(ProviderInfo{
		Name:     "Mock",
		Provider: ProviderOpenAI,
		Status:   "active",
	}, &mockAIService{}, &Config{APIKey: "test"})

	req := &ConsoleRequest{
		TaskType:   TaskPhotoSearch,
		Capability: CapabilityChat,
		Prompt:     "我的手机号是13812345678",
		Model:      "gpt-4",
	}

	resp, err := console.Process(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, resp.PIIRedacted)
}

// ==================== Console Methods Tests ====================

func TestConsole_GetUsageStats(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	// Initially nil
	stats := console.GetUsageStats()
	assert.Nil(t, stats)
}

func TestConsole_GetAuditLogs(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	logs := console.GetAuditLogs("", time.Time{}, 10)
	assert.Nil(t, logs) // Nil when audit is disabled
}

func TestConsole_GetAvailableProviders(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	providers := console.GetAvailableProviders()
	assert.Empty(t, providers)

	console.RegisterProvider(ProviderInfo{
		Name:     "Mock",
		Provider: ProviderOpenAI,
		Status:   "active",
	}, &mockAIService{}, &Config{})

	providers = console.GetAvailableProviders()
	assert.Len(t, providers, 1)
}

// ==================== File Reference Tests ====================

func TestFileReference(t *testing.T) {
	ref := FileReference{
		Path:     "/path/to/file.jpg",
		Type:     "image",
		Encoding: "base64",
	}

	assert.Equal(t, "/path/to/file.jpg", ref.Path)
	assert.Equal(t, "image", ref.Type)
}

// ==================== Request Options Tests ====================

func TestRequestOptions(t *testing.T) {
	opts := RequestOptions{
		Temperature:   0.7,
		MaxTokens:     1000,
		TopP:          0.9,
		StopSequences: []string{"END", "STOP"},
		Language:      "zh",
		OutputFormat:  "json",
	}

	assert.Equal(t, 0.7, opts.Temperature)
	assert.Equal(t, 1000, opts.MaxTokens)
	assert.Len(t, opts.StopSequences, 2)
}

// ==================== Concurrent Safety Tests ====================

func TestConsole_ConcurrentAccess(t *testing.T) {
	console := NewConsole(DefaultConsoleConfig())

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				console.GetAvailableProviders()
				console.GetUsageStats()
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				console.AddCustomRule(DeIDRule{
					Name:        "test",
					Pattern:     `test`,
					Replacement: "[TEST]",
					Enabled:     true,
				})
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestUsageTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewUsageTracker()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 100})
				tracker.GetStats()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestAuditLogger_ConcurrentAccess(t *testing.T) {
	logger := NewAuditLogger(30)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				logger.Log(&AuditLog{
					ID:        generateRequestID(),
					RequestID: generateRequestID(),
					Timestamp: time.Now(),
				})
				logger.GetLogs("", time.Time{}, 10)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ==================== Benchmarks ====================

func BenchmarkNewConsole(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewConsole(DefaultConsoleConfig())
	}
}

func BenchmarkProcessPII(b *testing.B) {
	console := NewConsole(DefaultConsoleConfig())
	text := "手机号13812345678和邮箱test@example.com以及身份证110101199001011234"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		console.ProcessPII(text)
	}
}

func BenchmarkAuditLog(b *testing.B) {
	logger := NewAuditLogger(30)
	entry := &AuditLog{
		ID:        "test",
		RequestID: "req1",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Log(entry)
	}
}

func BenchmarkUsageTrack(b *testing.B) {
	tracker := NewUsageTracker()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.Track("user1", ProviderOpenAI, TaskPhotoSearch, TokenUsage{TotalTokens: 100})
	}
}