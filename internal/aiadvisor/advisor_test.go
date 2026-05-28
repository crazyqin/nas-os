package aiadvisor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== Mock Ollama Server ==========

// mockOllamaServer 创建模拟 Ollama 服务器.
func mockOllamaServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// defaultOllamaHandler 默认 Ollama 聊天响应处理器.
func defaultOllamaHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/tags" {
		// Health check
		json.NewEncoder(w).Encode(ollamaTagsResponse{
			Models: []ollamaModel{{Name: "qwen2.5:7b"}},
		})
		return
	}

	if r.URL.Path == "/api/chat" {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := ollamaResponse{
			Model: req.Model,
			Message: Message{
				Role:    "assistant",
				Content: "这是一个测试回复。系统状态良好，CPU 使用率正常。",
			},
			Done: true,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	http.NotFound(w, r)
}

// diagnoseOllamaHandler 诊断场景的 Ollama 响应.
func diagnoseOllamaHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/tags" {
		json.NewEncoder(w).Encode(ollamaTagsResponse{
			Models: []ollamaModel{{Name: "qwen2.5:7b"}},
		})
		return
	}

	if r.URL.Path == "/api/chat" {
		resp := ollamaResponse{
			Model: "qwen2.5:7b",
			Message: Message{
				Role:    "assistant",
				Content: `{"problem": "磁盘空间不足", "cause": "日志文件过大，未配置轮转", "solutions": ["清理旧日志文件", "配置 logrotate 自动轮转", "扩容磁盘"], "severity": "warning"}`,
			},
			Done: true,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	http.NotFound(w, r)
}

// errorOllamaHandler 返回错误的 Ollama 响应.
func errorOllamaHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/tags" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("internal error"))
}

// ========== 测试辅助函数 ==========

// newTestAdvisor 创建测试用顾问实例.
func newTestAdvisor(serverURL string) *Advisor {
	cfg := DefaultConfig()
	cfg.OllamaEndpoint = serverURL
	cfg.RequestTimeout = 5 * time.Second
	return NewAdvisor(cfg, zap.NewNop())
}

// ========== Advisor 核心测试 ==========

func TestNewAdvisor(t *testing.T) {
	t.Run("默认配置", func(t *testing.T) {
		a := NewAdvisor(nil, nil)
		require.NotNil(t, a)
		assert.NotNil(t, a.config)
		assert.Equal(t, "http://localhost:11434", a.config.OllamaEndpoint)
		assert.Equal(t, "qwen2.5:7b", a.config.Model)
		assert.NotNil(t, a.sessions)
	})

	t.Run("自定义配置", func(t *testing.T) {
		cfg := &Config{
			OllamaEndpoint:     "http://custom:9999",
			Model:              "llama3:8b",
			Temperature:        0.5,
			MaxHistoryMessages: 50,
			RequestTimeout:     30 * time.Second,
			MaxTokens:          4096,
		}
		a := NewAdvisor(cfg, zap.NewNop())
		require.NotNil(t, a)
		assert.Equal(t, "http://custom:9999", a.config.OllamaEndpoint)
		assert.Equal(t, "llama3:8b", a.config.Model)
	})

	t.Run("nil logger 使用 nop", func(t *testing.T) {
		a := NewAdvisor(DefaultConfig(), nil)
		assert.NotNil(t, a.logger)
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "http://localhost:11434", cfg.OllamaEndpoint)
	assert.Equal(t, "qwen2.5:7b", cfg.Model)
	assert.Equal(t, 0.7, cfg.Temperature)
	assert.Equal(t, 20, cfg.MaxHistoryMessages)
	assert.Equal(t, 60*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 2048, cfg.MaxTokens)
}

func TestChat(t *testing.T) {
	t.Run("正常聊天", func(t *testing.T) {
		server := mockOllamaServer(defaultOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		resp, err := a.Chat(context.Background(), &ChatRequest{
			Message: "系统状态怎么样？",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Answer)
		assert.NotEmpty(t, resp.SessionID)
		assert.False(t, resp.Timestamp.IsZero())
	})

	t.Run("指定会话ID", func(t *testing.T) {
		server := mockOllamaServer(defaultOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		resp, err := a.Chat(context.Background(), &ChatRequest{
			Message:   "你好",
			SessionID: "my-session",
		})

		require.NoError(t, err)
		assert.Equal(t, "my-session", resp.SessionID)
	})

	t.Run("空消息", func(t *testing.T) {
		server := mockOllamaServer(defaultOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		_, err := a.Chat(context.Background(), &ChatRequest{
			Message: "",
		})

		assert.ErrorIs(t, err, ErrEmptyMessage)
	})

	t.Run("多轮对话历史", func(t *testing.T) {
		server := mockOllamaServer(defaultOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		sessionID := "multi-turn-test"

		// 第一轮
		resp1, err := a.Chat(context.Background(), &ChatRequest{
			Message:   "你好",
			SessionID: sessionID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp1.Answer)

		// 验证历史已保存
		a.mu.RLock()
		hist := a.sessions[sessionID]
		a.mu.RUnlock()
		assert.Equal(t, 2, len(hist)) // user + assistant
		assert.Equal(t, "user", hist[0].Role)
		assert.Equal(t, "你好", hist[0].Content)
		assert.Equal(t, "assistant", hist[1].Role)

		// 第二轮
		resp2, err := a.Chat(context.Background(), &ChatRequest{
			Message:   "内存使用率多少？",
			SessionID: sessionID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp2.Answer)

		a.mu.RLock()
		hist = a.sessions[sessionID]
		a.mu.RUnlock()
		assert.Equal(t, 4, len(hist)) // 两轮对话
	})

	t.Run("Ollama 不可用", func(t *testing.T) {
		a := newTestAdvisor("http://127.0.0.1:1") // 不存在的端口
		_, err := a.Chat(context.Background(), &ChatRequest{
			Message: "你好",
		})

		assert.Error(t, err)
	})

	t.Run("Ollama 返回错误", func(t *testing.T) {
		server := mockOllamaServer(errorOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		_, err := a.Chat(context.Background(), &ChatRequest{
			Message: "你好",
		})

		assert.Error(t, err)
	})

	t.Run("上下文取消", func(t *testing.T) {
		server := mockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				json.NewEncoder(w).Encode(ollamaTagsResponse{})
				return
			}
			// 模拟慢响应
			time.Sleep(5 * time.Second)
		})
		defer server.Close()

		a := newTestAdvisor(server.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := a.Chat(ctx, &ChatRequest{
			Message: "你好",
		})

		assert.Error(t, err)
	})
}

func TestGetSuggestions(t *testing.T) {
	server := mockOllamaServer(defaultOllamaHandler)
	defer server.Close()

	a := newTestAdvisor(server.URL)
	suggestions, err := a.GetSuggestions(context.Background())

	require.NoError(t, err)
	assert.NotEmpty(t, suggestions)

	// 至少有备份和安全建议（通用建议）
	categories := make(map[string]bool)
	for _, s := range suggestions {
		categories[s.Category] = true
		// 验证字段完整性
		assert.NotEmpty(t, s.ID)
		assert.NotEmpty(t, s.Title)
		assert.NotEmpty(t, s.Description)
		assert.GreaterOrEqual(t, s.Priority, 1)
		assert.LessOrEqual(t, s.Priority, 3)
		assert.NotEmpty(t, s.Action)
	}

	// 应包含 backup 和 security 建议
	assert.True(t, categories["backup"], "应包含备份建议")
	assert.True(t, categories["security"], "应包含安全建议")
}

func TestDiagnose(t *testing.T) {
	t.Run("正常诊断", func(t *testing.T) {
		server := mockOllamaServer(diagnoseOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		resp, err := a.Diagnose(context.Background(), &DiagnoseRequest{
			LogContent: "ERROR: disk full on /dev/sda1\nNo space left on device",
			Service:    "system",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Problem)
		assert.NotEmpty(t, resp.Cause)
		assert.NotEmpty(t, resp.Solutions)
		assert.NotEmpty(t, resp.Severity)
		assert.False(t, resp.Timestamp.IsZero())

		// 验证 JSON 解析成功
		assert.Equal(t, "磁盘空间不足", resp.Problem)
		assert.Equal(t, "warning", resp.Severity)
		assert.Len(t, resp.Solutions, 3)
	})

	t.Run("空日志内容", func(t *testing.T) {
		server := mockOllamaServer(defaultOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		_, err := a.Diagnose(context.Background(), &DiagnoseRequest{
			LogContent: "",
		})

		assert.ErrorIs(t, err, ErrEmptyDiagnosis)
	})

	t.Run("AI 非 JSON 回复时的回退处理", func(t *testing.T) {
		server := mockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				json.NewEncoder(w).Encode(ollamaTagsResponse{})
				return
			}
			resp := ollamaResponse{
				Model: "qwen2.5:7b",
				Message: Message{
					Role:    "assistant",
					Content: "这是一个错误日志分析。根据日志内容来看，可能是磁盘空间不足导致的问题。",
				},
				Done: true,
			}
			json.NewEncoder(w).Encode(resp)
		})
		defer server.Close()

		a := newTestAdvisor(server.URL)
		resp, err := a.Diagnose(context.Background(), &DiagnoseRequest{
			LogContent: "panic: runtime error: out of memory",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Problem)
		assert.Equal(t, "critical", resp.Severity) // panic 关键词 -> critical
	})

	t.Run("Ollama 不可用", func(t *testing.T) {
		a := newTestAdvisor("http://127.0.0.1:1")
		_, err := a.Diagnose(context.Background(), &DiagnoseRequest{
			LogContent: "some error log",
		})

		assert.Error(t, err)
	})
}

func TestDetectSeverity(t *testing.T) {
	a := newTestAdvisor("http://localhost:11434")

	tests := []struct {
		log      string
		expected string
	}{
		{"kernel panic - not syncing: Fatal exception", "critical"},
		{"OOM killed process 1234", "critical"},
		{"segfault at 0x00000000", "critical"},
		{"ERROR: connection refused", "warning"},
		{"timeout waiting for response", "warning"},
		{"access denied for user root", "warning"},
		{"INFO: service started successfully", "info"},
		{"normal log entry without issues", "info"},
		{"", "info"},
	}

	for _, tt := range tests {
		result := a.detectSeverity(tt.log)
		assert.Equal(t, tt.expected, result, "detectSeverity(%q)", tt.log)
	}
}

func TestHealthCheck(t *testing.T) {
	t.Run("服务可用", func(t *testing.T) {
		server := mockOllamaServer(defaultOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		err := a.HealthCheck(context.Background())
		assert.NoError(t, err)
	})

	t.Run("服务不可用", func(t *testing.T) {
		server := mockOllamaServer(errorOllamaHandler)
		defer server.Close()

		a := newTestAdvisor(server.URL)
		err := a.HealthCheck(context.Background())
		assert.ErrorIs(t, err, ErrOllamaUnavail)
	})

	t.Run("连接失败", func(t *testing.T) {
		a := newTestAdvisor("http://127.0.0.1:1")
		err := a.HealthCheck(context.Background())
		assert.ErrorIs(t, err, ErrOllamaUnavail)
	})
}

func TestClearSession(t *testing.T) {
	server := mockOllamaServer(defaultOllamaHandler)
	defer server.Close()

	a := newTestAdvisor(server.URL)

	// 创建会话
	_, err := a.Chat(context.Background(), &ChatRequest{
		Message:   "测试",
		SessionID: "test-session",
	})
	require.NoError(t, err)

	// 验证会话存在
	a.mu.RLock()
	_, exists := a.sessions["test-session"]
	a.mu.RUnlock()
	assert.True(t, exists)

	// 清除会话
	a.ClearSession("test-session")

	a.mu.RLock()
	_, exists = a.sessions["test-session"]
	a.mu.RUnlock()
	assert.False(t, exists)
}

func TestGetSystemContext(t *testing.T) {
	a := newTestAdvisor("http://localhost:11434")
	ctx := a.GetSystemContext()

	require.NotNil(t, ctx)
	assert.NotEmpty(t, ctx.Hostname)
	assert.NotEmpty(t, ctx.Arch)
	assert.NotEmpty(t, ctx.OS)
	assert.Greater(t, ctx.CPUCores, 0)
	assert.Greater(t, ctx.MemTotal, uint64(0))
	assert.False(t, ctx.Timestamp.IsZero())
}

func TestParseDiagnosis(t *testing.T) {
	a := newTestAdvisor("http://localhost:11434")

	t.Run("有效JSON响应", func(t *testing.T) {
		answer := `{"problem": "内存溢出", "cause": "内存泄漏", "solutions": ["重启服务", "检查代码"], "severity": "critical"}`
		resp := a.parseDiagnosis(answer, "oom killed")

		assert.Equal(t, "内存溢出", resp.Problem)
		assert.Equal(t, "内存泄漏", resp.Cause)
		assert.Len(t, resp.Solutions, 2)
		assert.Equal(t, "critical", resp.Severity)
	})

	t.Run("带有前后文字的JSON", func(t *testing.T) {
		answer := `根据分析，诊断结果如下：
{"problem": "磁盘故障", "cause": "SMART错误", "solutions": ["备份数据"], "severity": "warning"}
请尽快处理。`
		resp := a.parseDiagnosis(answer, "smart error")

		assert.Equal(t, "磁盘故障", resp.Problem)
		assert.Equal(t, "warning", resp.Severity)
	})

	t.Run("非JSON响应", func(t *testing.T) {
		answer := "这是一个纯文本回复，不包含 JSON"
		resp := a.parseDiagnosis(answer, "some log content")

		assert.Equal(t, "系统日志异常", resp.Problem)
		assert.Equal(t, answer, resp.Cause)
		assert.Equal(t, "info", resp.Severity) // 无关键词 -> info
	})
}

func TestBuildChatMessages(t *testing.T) {
	a := newTestAdvisor("http://localhost:11434")
	sysCtx := &SystemContext{
		Hostname: "test-host",
		OS:       "linux",
		CPUUsage: 50.0,
	}

	t.Run("无历史消息", func(t *testing.T) {
		msgs := a.buildChatMessages("test-session", "你好", sysCtx)
		assert.GreaterOrEqual(t, len(msgs), 3) // system + context + user
		assert.Equal(t, "system", msgs[0].Role)
		assert.Equal(t, "user", msgs[len(msgs)-1].Role)
		assert.Equal(t, "你好", msgs[len(msgs)-1].Content)
	})

	t.Run("有历史消息", func(t *testing.T) {
		sessionID := "hist-test"
		a.sessions[sessionID] = []Message{
			{Role: "user", Content: "之前的问题"},
			{Role: "assistant", Content: "之前的回答"},
		}

		msgs := a.buildChatMessages(sessionID, "新问题", sysCtx)
		assert.GreaterOrEqual(t, len(msgs), 5) // system + context + 2 history + user
	})
}

// ========== 辅助函数测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{1536 * 1024 * 1024, "1.50 GB"},
	}
	for _, tt := range tests {
		result := formatBytes(tt.input)
		assert.Equal(t, tt.expected, result, "formatBytes(%d)", tt.input)
	}
}

func TestRound2(t *testing.T) {
	assert.Equal(t, 3.14, round2(3.14159))
	assert.Equal(t, 10.0, round2(10.0))
	assert.Equal(t, 0.01, round2(0.005))
}

// ========== Handlers 测试 ==========

func TestHandlers(t *testing.T) {
	server := mockOllamaServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			json.NewEncoder(w).Encode(ollamaTagsResponse{
				Models: []ollamaModel{{Name: "qwen2.5:7b"}},
			})
			return
		}
		if r.URL.Path == "/api/chat" {
			var req ollamaRequest
			json.NewDecoder(r.Body).Decode(&req)

			// 根据消息内容返回不同响应
			var answer string
			lastMsg := req.Messages[len(req.Messages)-1].Content
			if strings.Contains(lastMsg, "诊断") {
				answer = `{"problem": "磁盘满", "cause": "日志过大", "solutions": ["清理日志"], "severity": "warning"}`
			} else {
				answer = "系统状态良好，CPU 使用率 25%，内存使用率 40%。"
			}

			json.NewEncoder(w).Encode(ollamaResponse{
				Model:   "qwen2.5:7b",
				Message: Message{Role: "assistant", Content: answer},
				Done:    true,
			})
			return
		}
		http.NotFound(w, r)
	})
	defer server.Close()

	a := newTestAdvisor(server.URL)
	h := NewHandlers(a, zap.NewNop())
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	t.Run("POST /chat - 正常请求", func(t *testing.T) {
		body := `{"message": "系统状态怎么样？"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp ChatResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Answer)
		assert.NotEmpty(t, resp.SessionID)
	})

	t.Run("POST /chat - 指定会话ID", func(t *testing.T) {
		body := `{"message": "你好", "session_id": "user-123"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp ChatResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "user-123", resp.SessionID)
	})

	t.Run("POST /chat - 空消息", func(t *testing.T) {
		body := `{"message": ""}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST /chat - 无效 JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/chat", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GET /suggestions", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai/advisor/suggestions", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Suggestions []Suggestion `json:"suggestions"`
			Total       int          `json:"total"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Greater(t, resp.Total, 0)
		assert.Len(t, resp.Suggestions, resp.Total)
	})

	t.Run("POST /diagnose - 正常请求", func(t *testing.T) {
		body := `{"log_content": "ERROR: disk full\nNo space left on device", "service": "system"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/diagnose", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp DiagnoseResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Problem)
	})

	t.Run("POST /diagnose - 空日志", func(t *testing.T) {
		body := `{"log_content": ""}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/diagnose", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST /diagnose - 无效 JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/diagnose", strings.NewReader("bad"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GET /system-context", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai/advisor/system-context", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp SystemContext
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Hostname)
	})

	t.Run("DELETE /session/:id", func(t *testing.T) {
		// 先创建会话
		chatBody := `{"message": "测试会话", "session_id": "del-test"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/chat", strings.NewReader(chatBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// 删除会话
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("DELETE", "/api/v1/ai/advisor/session/del-test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "会话已清除")
	})

	t.Run("GET /health - 可用", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai/advisor/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"status":"ok"`)
	})
}

func TestHandlersOllamaUnavailable(t *testing.T) {
	server := mockOllamaServer(errorOllamaHandler)
	defer server.Close()

	a := newTestAdvisor(server.URL)
	h := NewHandlers(a, zap.NewNop())
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	t.Run("chat - Ollama 不可用", func(t *testing.T) {
		body := `{"message": "你好"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("diagnose - Ollama 不可用", func(t *testing.T) {
		body := `{"log_content": "some error"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai/advisor/diagnose", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("health - Ollama 不可用", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai/advisor/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

func TestHandlersRouteRegistration(t *testing.T) {
	a := newTestAdvisor("http://localhost:11434")
	h := NewHandlers(a, nil) // nil logger -> nop
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	// 验证路由已注册 - 通过405/404等状态码间接验证
	routes := router.Routes()
	registeredPaths := make(map[string]bool)
	for _, r := range routes {
		registeredPaths[r.Path] = true
	}

	assert.True(t, registeredPaths["/api/v1/ai/advisor/chat"])
	assert.True(t, registeredPaths["/api/v1/ai/advisor/suggestions"])
	assert.True(t, registeredPaths["/api/v1/ai/advisor/diagnose"])
	assert.True(t, registeredPaths["/api/v1/ai/advisor/system-context"])
	assert.True(t, registeredPaths["/api/v1/ai/advisor/session/:session_id"])
	assert.True(t, registeredPaths["/api/v1/ai/advisor/health"])
}

// ========== 并发安全测试 ==========

func TestConcurrentChat(t *testing.T) {
	server := mockOllamaServer(defaultOllamaHandler)
	defer server.Close()

	a := newTestAdvisor(server.URL)
	const goroutines = 10

	done := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			_, err := a.Chat(context.Background(), &ChatRequest{
				Message:   "并发测试",
				SessionID: "concurrent-session",
			})
			done <- err
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		err := <-done
		assert.NoError(t, err)
	}

	// 验证历史记录数量正确
	a.mu.RLock()
	hist := a.sessions["concurrent-session"]
	a.mu.RUnlock()
	assert.Equal(t, goroutines*2, len(hist)) // 每轮 user + assistant
}
