package aiassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func TestDetectQueryType(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		query    string
		expected QueryType
	}{
		{"查看 CPU 使用率", QueryTypeCPU},
		{"内存使用情况", QueryTypeMemory},
		{"磁盘空间不足", QueryTypeDisk},
		{"系统状态", QueryTypeSystem},
		{"搜索文件 report.pdf", QueryTypeFile},
		{"系统故障排查", QueryTypeDiag},
		{"你好", QueryTypeGeneral},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := m.detectQueryType(tt.query)
			if result != tt.expected {
				t.Errorf("detectQueryType(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestProcessQuery(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// 测试系统查询
	resp, err := m.ProcessQuery(ctx, &QueryRequest{
		Query:     "查看系统状态",
		QueryType: QueryTypeSystem,
	})
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if resp.Status != QueryStatusCompleted {
		t.Errorf("expected status completed, got %v", resp.Status)
	}
	if resp.Answer == "" {
		t.Error("expected non-empty answer")
	}
	if resp.Data == nil {
		t.Error("expected non-nil data")
	}
}

func TestProcessQueryCPU(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	resp, err := m.ProcessQuery(ctx, &QueryRequest{
		Query:     "CPU 使用情况",
		QueryType: QueryTypeCPU,
	})
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected CPU data")
	}
}

func TestProcessQueryMemory(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	resp, err := m.ProcessQuery(ctx, &QueryRequest{
		Query:     "内存使用情况",
		QueryType: QueryTypeMemory,
	})
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected memory data")
	}
}

func TestProcessQueryDisk(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	resp, err := m.ProcessQuery(ctx, &QueryRequest{
		Query:     "磁盘使用情况",
		QueryType: QueryTypeDisk,
	})
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected disk data")
	}
}

func TestProcessQueryFile(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	resp, err := m.ProcessQuery(ctx, &QueryRequest{
		Query:     "搜索文件 test.txt",
		QueryType: QueryTypeFile,
	})
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if resp.Status != QueryStatusCompleted {
		t.Errorf("expected status completed, got %v", resp.Status)
	}
}

func TestProcessQueryDiagnosis(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	resp, err := m.ProcessQuery(ctx, &QueryRequest{
		Query:     "系统运行缓慢",
		QueryType: QueryTypeDiag,
	})
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected diagnosis data")
	}
	if len(resp.Suggestions) == 0 {
		t.Error("expected suggestions")
	}
}

func TestProcessQueryGeneral(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	resp, err := m.ProcessQuery(ctx, &QueryRequest{
		Query: "你好",
	})
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if resp.Answer == "" {
		t.Error("expected non-empty answer")
	}
}

func TestQueryCache(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	req := &QueryRequest{
		Query:     "系统状态",
		QueryType: QueryTypeSystem,
	}

	// 第一次查询
	resp1, err := m.ProcessQuery(ctx, req)
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}

	// 第二次查询应该命中缓存
	resp2, err := m.ProcessQuery(ctx, req)
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}

	if resp1.ID != resp2.ID {
		t.Error("expected same ID from cache")
	}
}

func TestQueryHistory(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	// 执行多个查询
	for i := 0; i < 5; i++ {
		m.ProcessQuery(ctx, &QueryRequest{
			Query:     fmt.Sprintf("test query %d", i),
			QueryType: QueryTypeGeneral,
		})
	}

	history := m.GetQueryHistory(3)
	if len(history) != 3 {
		t.Errorf("expected 3 history items, got %d", len(history))
	}
}

func TestConversation(t *testing.T) {
	m := setupTestManager(t)

	// 创建对话
	conv := m.CreateConversation()
	if conv.ID == "" {
		t.Error("expected non-empty conversation ID")
	}

	// 添加消息
	err := m.AddMessage(conv.ID, "user", "你好")
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	// 获取对话
	got, err := m.GetConversation(conv.ID)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(got.Messages))
	}

	// 不存在的对话
	_, err = m.GetConversation("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

func TestDisabledAssistant(t *testing.T) {
	cfg := DefaultAIConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	_, err := m.ProcessQuery(context.Background(), &QueryRequest{
		Query: "test",
	})
	if err == nil {
		t.Error("expected error when disabled")
	}
}

func TestHandler_Query(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"query":"系统状态","query_type":"system"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai-assistant/query", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_GetSystemStatus(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai-assistant/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_SearchFiles(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"query":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai-assistant/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Diagnose(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"problem":"系统运行缓慢"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai-assistant/diagnose", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Conversation(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 创建对话
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai-assistant/conversations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	convData, _ := json.Marshal(resp.Data)
	var conv Conversation
	json.Unmarshal(convData, &conv)

	// 获取对话
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ai-assistant/conversations/"+conv.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 添加消息
	msgBody := `{"role":"user","content":"你好"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/ai-assistant/conversations/"+conv.ID+"/messages", bytes.NewBufferString(msgBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_History(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai-assistant/history?limit=10", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Config(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 获取配置
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai-assistant/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 更新配置
	body := `{"enabled":true,"default_model":"gpt-4","max_tokens":4096}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/ai-assistant/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ClearCache(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai-assistant/cache/clear", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAnalyzeProblem(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		problem  string
		severity Severity
		category string
	}{
		{"系统运行缓慢", SeverityMedium, "performance"},
		{"磁盘空间不足", SeverityHigh, "storage"},
		{"网络连接问题", SeverityMedium, "network"},
		{"未知问题", SeverityLow, "general"},
	}

	for _, tt := range tests {
		t.Run(tt.problem, func(t *testing.T) {
			result := m.analyzeProblem(tt.problem)
			if result.Severity != tt.severity {
				t.Errorf("expected severity %v, got %v", tt.severity, result.Severity)
			}
			if result.Category != tt.category {
				t.Errorf("expected category %v, got %v", tt.category, result.Category)
			}
		})
	}
}

func TestExtractSearchTerm(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		query    string
		expected string
	}{
		{`搜索文件 "report.pdf"`, "report.pdf"},
		{"搜索 test", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := m.extractSearchTerm(tt.query)
			if result != tt.expected {
				t.Errorf("extractSearchTerm(%q) = %q, want %q", tt.query, result, tt.expected)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}
