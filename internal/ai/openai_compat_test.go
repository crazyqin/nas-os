// Package ai provides AI service integration for NAS-OS
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== OpenAICompatibleClient Tests ====================

func TestNewOpenAICompatibleClient(t *testing.T) {
	tests := []struct {
		name    string
		config  *OpenAICompatibleConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &OpenAICompatibleConfig{
				APIKey:   "test-key",
				Endpoint: "https://api.example.com/v1",
				Model:    "gpt-4",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &OpenAICompatibleConfig{
				Endpoint: "https://api.example.com/v1",
				Model:    "gpt-4",
			},
			wantErr: true,
		},
		{
			name: "default values",
			config: &OpenAICompatibleConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewOpenAICompatibleClient(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestOpenAICompatibleClient_Chat(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		// 返回模拟响应
		resp := OpenAIChatResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 创建客户端
	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      "test-key",
		Endpoint:    server.URL,
		Model:       "gpt-4",
		Provider:    ProviderOpenAI,
		MaxTokens:   100,
		Temperature: 0.7,
	})
	require.NoError(t, err)

	// 发送请求
	req := &Request{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := client.Chat(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "Hello! How can I help you?", resp.Content)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Equal(t, ProviderOpenAI, resp.Provider)
	assert.Equal(t, 30, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
}

func TestOpenAICompatibleClient_Chat_Error(t *testing.T) {
	// 创建返回错误的测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := OpenAIErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "invalid-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	req := &Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err = client.Chat(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
}

func TestOpenAICompatibleClient_Embed(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)

		resp := OpenAIEmbedResponse{
			Object: "list",
			Data: []OpenAIEmbedData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
				},
			},
			Model: "text-embedding-ada-002",
			Usage: OpenAIUsage{
				TotalTokens: 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "text-embedding-ada-002",
	})
	require.NoError(t, err)

	embedding, err := client.Embed(context.Background(), "Hello world")
	require.NoError(t, err)

	assert.Len(t, embedding, 5)
	assert.Equal(t, []float32{0.1, 0.2, 0.3, 0.4, 0.5}, embedding)
}

func TestOpenAICompatibleClient_GetProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		expected Provider
	}{
		{
			name:     "custom provider",
			provider: ProviderCustom,
			expected: ProviderCustom,
		},
		{
			name:     "empty provider defaults to OpenAI",
			provider: "",
			expected: ProviderOpenAI,
		},
		{
			name:     "local provider",
			provider: ProviderLocal,
			expected: ProviderLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
				APIKey:   "test-key",
				Provider: tt.provider,
			})
			require.NoError(t, err)

			assert.Equal(t, tt.expected, client.GetProvider())
		})
	}
}

// ==================== SSE Reader Tests ====================

func TestSSEReader_ReadEvent(t *testing.T) {
	sseData := "event: message\ndata: {\"content\": \"Hello\"}\n\nevent: message\ndata: {\"content\": \"World\"}\n\ndata: [DONE]\n\n"

	reader := NewSSEReader(strings.NewReader(sseData))

	// 读取第一个事件
	event, err := reader.ReadEvent()
	require.NoError(t, err)
	assert.Equal(t, "message", event.Event)
	assert.Equal(t, `{"content": "Hello"}`, event.Data)

	// 读取第二个事件
	event, err = reader.ReadEvent()
	require.NoError(t, err)
	assert.Equal(t, "message", event.Event)
	assert.Equal(t, `{"content": "World"}`, event.Data)

	// 读取结束标记
	event, err = reader.ReadEvent()
	require.NoError(t, err)
	assert.Equal(t, "[DONE]", event.Data)
}

// ==================== Predefined Provider Tests ====================

func TestNewOpenAIClient(t *testing.T) {
	client, err := NewOpenAIClient("test-key", "gpt-4")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, OpenAIEndpoint, client.config.Endpoint)
	assert.Equal(t, "gpt-4", client.config.Model)
	assert.Equal(t, ProviderOpenAI, client.config.Provider)
}

func TestNewDeepSeekClient(t *testing.T) {
	client, err := NewDeepSeekClient("test-key", "")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, DeepSeekEndpoint, client.config.Endpoint)
	assert.Equal(t, "deepseek-chat", client.config.Model) // default model
}

func TestNewZhipuAIClient(t *testing.T) {
	client, err := NewZhipuAIClient("test-key", "glm-4-flash")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, ZhipuAIEndpoint, client.config.Endpoint)
	assert.Equal(t, "glm-4-flash", client.config.Model)
}

func TestNewQwenClient(t *testing.T) {
	client, err := NewQwenClient("test-key", "")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, QwenEndpoint, client.config.Endpoint)
	assert.Equal(t, "qwen-turbo", client.config.Model) // default model
}

func TestNewMoonshotClient(t *testing.T) {
	client, err := NewMoonshotClient("test-key", "moonshot-v1-32k")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, MoonshotEndpoint, client.config.Endpoint)
	assert.Equal(t, "moonshot-v1-32k", client.config.Model)
}

func TestNewLocalLLMClient(t *testing.T) {
	client, err := NewLocalLLMClient("http://localhost:11434/v1", "")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:11434/v1", client.config.Endpoint)
	assert.Equal(t, "llama2", client.config.Model) // default model
	assert.Equal(t, ProviderLocal, client.config.Provider)
}

// ==================== Provider List Tests ====================

func TestGetOpenAICompatibleProviders(t *testing.T) {
	providers := GetOpenAICompatibleProviders()

	assert.NotEmpty(t, providers)
	assert.GreaterOrEqual(t, len(providers), 5)

	// 验证每个提供商都有必要字段
	for _, p := range providers {
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Endpoint)
		assert.NotEmpty(t, p.Description)
		assert.NotEmpty(t, p.Models)
	}
}

// ==================== OpenAI API Types Tests ====================

func TestOpenAIChatRequest_Marshal(t *testing.T) {
	req := OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	// 验证序列化结果
	assert.Contains(t, string(data), `"model":"gpt-4"`)
	assert.Contains(t, string(data), `"max_tokens":100`)
	assert.Contains(t, string(data), `"temperature":0.7`)
}

func TestOpenAIChatResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Hello there!"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15
		}
	}`

	var resp OpenAIChatResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	require.NoError(t, err)

	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "Hello there!", resp.Choices[0].Message.Content)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestOpenAIErrorResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"error": {
			"message": "Rate limit exceeded",
			"type": "rate_limit_error",
			"code": "rate_limit_exceeded"
		}
	}`

	var resp OpenAIErrorResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Rate limit exceeded", resp.Error.Message)
	assert.Equal(t, "rate_limit_error", resp.Error.Type)
	assert.Equal(t, "rate_limit_exceeded", resp.Error.Code)
}

// ==================== Edge Cases ====================

func TestOpenAICompatibleClient_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-4",
			Choices: []OpenAIChoice{}, // Empty choices
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	req := &Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err = client.Chat(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no response choices")
}

func TestOpenAICompatibleClient_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证自定义请求头
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))

		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-4",
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "OK"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
		Headers:  map[string]string{"X-Custom-Header": "custom-value"},
	})
	require.NoError(t, err)

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	_, err = client.Chat(context.Background(), req)
	require.NoError(t, err)
}

func TestOpenAICompatibleClient_OverrideModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// 验证使用了请求中的模型，而不是配置中的
		assert.Equal(t, "gpt-4-turbo", req.Model)

		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-4-turbo",
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "OK"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-3.5-turbo", // 配置中的模型
	})
	require.NoError(t, err)

	req := &Request{
		Model:    "gpt-4-turbo", // 请求中覆盖模型
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	_, err = client.Chat(context.Background(), req)
	require.NoError(t, err)
}

// ==================== StreamChat Tests ====================

func TestOpenAICompatibleClient_StreamChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// 使用 Flusher 确保数据立即发送
		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "ResponseWriter should support Flusher")

		// 发送第一个事件
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":\"\"}]}\n\n")
		flusher.Flush()

		// 小延迟确保客户端有时间处理
		time.Sleep(10 * time.Millisecond)

		// 发送第二个事件
		fmt.Fprintf(w, "data: {\"id\":\"2\",\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" World\"},\"finish_reason\":\"\"}]}\n\n")
		flusher.Flush()

		// 小延迟
		time.Sleep(10 * time.Millisecond)

		// 发送结束标记
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	var chunks []string
	req := &Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	err = client.StreamChat(context.Background(), req, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Hello", " World"}, chunks)
}

func TestOpenAICompatibleClient_StreamChat_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": {"message": "Server error"}}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	err = client.StreamChat(context.Background(), req, func(chunk string) error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestOpenAICompatibleClient_StreamChat_CallbackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send SSE event with proper format (ends with double newline)
		fmt.Fprint(w, "data: {\"id\":\"test\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	expectedErr := errors.New("callback error")

	err = client.StreamChat(context.Background(), req, func(chunk string) error {
		return expectedErr
	})
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// ==================== Context Cancellation Tests ====================

func TestOpenAICompatibleClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // 模拟慢响应
		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-4",
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "OK"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	_, err = client.Chat(ctx, req)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context"))
}

// ==================== Embed Error Tests ====================

func TestOpenAICompatibleClient_EmptyEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OpenAIEmbedResponse{
			Object: "list",
			Data:   []OpenAIEmbedData{}, // Empty
			Model:  "text-embedding-ada-002",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "text-embedding-ada-002",
	})
	require.NoError(t, err)

	_, err = client.Embed(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no embedding data")
}

func TestOpenAICompatibleClient_Embed_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "text-embedding-ada-002",
	})
	require.NoError(t, err)

	_, err = client.Embed(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Rate limit")
}

// ==================== Temperature & MaxTokens Override Tests ====================

func TestOpenAICompatibleClient_RequestOverrides(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// 验证请求覆盖了配置中的值
		assert.Equal(t, 2000, req.MaxTokens)
		assert.Equal(t, 0.5, req.Temperature)

		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-4",
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "OK"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:      "test-key",
		Endpoint:    server.URL,
		Model:       "gpt-4",
		MaxTokens:   100, // 配置值
		Temperature: 0.9, // 配置值
	})
	require.NoError(t, err)

	req := &Request{
		Messages:    []Message{{Role: "user", Content: "test"}},
		MaxTokens:   2000, // 覆盖
		Temperature: 0.5,  // 覆盖
	}
	_, err = client.Chat(context.Background(), req)
	require.NoError(t, err)
}

// ==================== Network Error Tests ====================

func TestOpenAICompatibleClient_NetworkError(t *testing.T) {
	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: "http://127.0.0.1:1", // 无效端口
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	_, err = client.Chat(context.Background(), req)
	assert.Error(t, err)
}

func TestOpenAICompatibleClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	_, err = client.Chat(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

// ==================== Concurrent Safety Tests ====================

func TestOpenAICompatibleClient_ConcurrentChat(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-4",
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "OK"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	// 并发发送 10 个请求
	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
			_, err := client.Chat(context.Background(), req)
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		assert.NoError(t, err)
	}

	assert.Equal(t, int32(10), atomic.LoadInt32(&requestCount))
}

// ==================== SSE Reader Edge Cases ====================

func TestSSEReader_MultipleDataLines(t *testing.T) {
	sseData := `event: message
data: line1
data: line2

`
	reader := NewSSEReader(strings.NewReader(sseData))

	event, err := reader.ReadEvent()
	require.NoError(t, err)
	assert.Equal(t, "message", event.Event)
	assert.Equal(t, "line1\nline2", event.Data)
}

func TestSSEReader_EmptyEvent(t *testing.T) {
	sseData := `data: test


`
	reader := NewSSEReader(strings.NewReader(sseData))

	event, err := reader.ReadEvent()
	require.NoError(t, err)
	assert.Equal(t, "test", event.Data)
}

func TestSSEReader_EOF(t *testing.T) {
	reader := NewSSEReader(strings.NewReader(""))

	_, err := reader.ReadEvent()
	assert.Error(t, err)
}

// ==================== Endpoint Trailing Slash Tests ====================

func TestOpenAICompatibleClient_EndpointTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-4",
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "OK"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 测试带尾部斜杠的端点
	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL + "/", // 带尾部斜杠
		Model:    "gpt-4",
	})
	require.NoError(t, err)

	// 验证端点已去除尾部斜杠
	assert.Equal(t, server.URL, client.config.Endpoint)

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	_, err = client.Chat(context.Background(), req)
	require.NoError(t, err)
}

// ==================== Default Model Tests ====================

func TestOpenAICompatibleClient_DefaultModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req OpenAIChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// 验证使用了默认模型
		assert.Equal(t, "gpt-3.5-turbo", req.Model)

		resp := OpenAIChatResponse{
			ID:      "test-id",
			Model:   "gpt-3.5-turbo",
			Choices: []OpenAIChoice{{Message: OpenAIMessage{Content: "OK"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
		APIKey:   "test-key",
		Endpoint: server.URL,
		// 不指定 Model，应使用默认值
	})
	require.NoError(t, err)
	assert.Equal(t, "gpt-3.5-turbo", client.config.Model)

	req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
	_, err = client.Chat(context.Background(), req)
	require.NoError(t, err)
}

// ==================== HTTP Status Error Tests ====================

func TestOpenAICompatibleClient_HTTPStatusError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"400 Bad Request", http.StatusBadRequest, true},
		{"401 Unauthorized", http.StatusUnauthorized, true},
		{"403 Forbidden", http.StatusForbidden, true},
		{"404 Not Found", http.StatusNotFound, true},
		{"429 Rate Limited", http.StatusTooManyRequests, true},
		{"500 Internal Error", http.StatusInternalServerError, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"error": {"message": "test error"}}`))
			}))
			defer server.Close()

			client, err := NewOpenAICompatibleClient(&OpenAICompatibleConfig{
				APIKey:   "test-key",
				Endpoint: server.URL,
				Model:    "gpt-4",
			})
			require.NoError(t, err)

			req := &Request{Messages: []Message{{Role: "user", Content: "test"}}}
			_, err = client.Chat(context.Background(), req)
			assert.Error(t, err)
		})
	}
}
