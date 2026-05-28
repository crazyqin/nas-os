package benchmarkpro

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(mgr *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	h := NewHandlers(mgr)
	h.RegisterRoutes(api)
	return r
}

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
}

func TestManagerStartStop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	m.Stop()
}

func TestRunTestCPU(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	req := &BenchRequest{
		TestType:    TestTypeCPU,
		DurationSec: 1,
	}
	result, err := m.RunTest(req)
	if err != nil {
		t.Fatalf("RunTest 失败: %v", err)
	}
	if result == nil {
		t.Fatal("结果不应为 nil")
	}
	if result.Status != StatusPending && result.Status != StatusRunning {
		t.Logf("测试状态: %s", result.Status)
	}
}

func TestRunTestInvalidType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	req := &BenchRequest{
		TestType:    BenchTestType("invalid"),
		DurationSec: 1,
	}
	result, _ := m.RunTest(req)
	// 等待测试完成
	if result != nil {
		// 异步执行，最终应为 failed 状态
		t.Logf("测试 ID: %s, 状态: %s", result.ID, result.Status)
	}
}

func TestGetResultNotFound(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	_, err := m.GetResult("non-existent-id")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestListResultsEmpty(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	results := m.ListResults()
	if results == nil {
		t.Fatal("结果列表不应为 nil")
	}
	if len(results) != 0 {
		t.Fatalf("期望空列表，得到 %d 条", len(results))
	}
}

func TestDiagnoseBottlenecks(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	// 构造一个有瓶颈的结果
	result := &BenchResult{
		CPUScore:         100,
		MemBandwidthGBps: 5,
		SeqReadMBps:      50,
		RandomReadIOPS:   500,
		NetLatencyMs:     15,
		NetPacketLoss:    1.0,
	}

	bottlenecks := m.DiagnoseBottlenecks(result)
	if len(bottlenecks) == 0 {
		t.Fatal("应检测到瓶颈")
	}

	// 验证关键瓶颈
	foundCPU := false
	foundNet := false
	for _, b := range bottlenecks {
		if b.Resource == "cpu" {
			foundCPU = true
		}
		if b.Resource == "network" && b.Severity == SeverityCritical {
			foundNet = true
		}
	}
	if !foundCPU {
		t.Error("应检测到 CPU 瓶颈")
	}
	if !foundNet {
		t.Error("应检测到网络严重瓶颈（丢包率过高）")
	}
}

func TestGenerateSuggestions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	// 无瓶颈时应生成默认建议
	result := &BenchResult{
		OverallScore: 800,
	}
	suggestions := m.GenerateSuggestions(result, nil)
	if len(suggestions) == 0 {
		t.Fatal("应生成至少一条建议")
	}

	// 有瓶颈时应生成针对性建议
	bottlenecks := []*Bottleneck{
		{Resource: "cpu", Severity: SeverityWarning, Suggestion: "升级 CPU"},
	}
	suggestions = m.GenerateSuggestions(result, bottlenecks)
	if len(suggestions) == 0 {
		t.Fatal("应生成至少一条建议")
	}
	found := false
	for _, s := range suggestions {
		if s.Category == "cpu" {
			found = true
		}
	}
	if !found {
		t.Error("应包含 CPU 优化建议")
	}
}

func TestCompetitorComparison(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	// 添加竞品数据
	m.AddCompetitor(&CompetitorEntry{
		Name:         "竞品A",
		CPUScore:     500,
		MemScore:     600,
		DiskScore:    400,
		NetScore:     700,
		OverallScore: 550,
	})

	competitors := m.ListCompetitors()
	if len(competitors) != 1 {
		t.Fatalf("期望 1 个竞品，得到 %d 个", len(competitors))
	}
	if competitors[0].Name != "竞品A" {
		t.Errorf("期望竞品名称为 竞品A，得到 %s", competitors[0].Name)
	}
}

func TestTrendAnalysis(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	// 空趋势
	analysis := m.AnalyzeTrend("")
	if analysis == nil {
		t.Fatal("趋势分析不应为 nil")
	}
	if analysis.Trend != "stable" {
		t.Errorf("期望趋势为 stable，得到 %s", analysis.Trend)
	}
}

func TestHandlerRunTest(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	r := setupTestRouter(m)

	body := `{"test_type":"cpu","duration_sec":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/benchmarkpro/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusAccepted, w.Code)
	}

	var result BenchResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if result.ID == "" {
		t.Error("结果 ID 不应为空")
	}
}

func TestHandlerListResults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	r := setupTestRouter(m)

	req := httptest.NewRequest(http.MethodGet, "/api/benchmarkpro/results", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusOK, w.Code)
	}

	var results []*BenchResult
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
}

func TestHandlerGetResultNotFound(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	r := setupTestRouter(m)

	req := httptest.NewRequest(http.MethodGet, "/api/benchmarkpro/results/not-exist", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusNotFound, w.Code)
	}
}

func TestHandlerAddCompetitor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	r := setupTestRouter(m)

	body := `{"name":"测试竞品","cpu_score":500,"mem_score":600,"disk_score":400,"net_score":700,"overall_score":550}`
	req := httptest.NewRequest(http.MethodPost, "/api/benchmarkpro/competitors", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusCreated, w.Code)
	}
}

func TestHandlerAddCompetitorMissingName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	r := setupTestRouter(m)

	body := `{"cpu_score":500}`
	req := httptest.NewRequest(http.MethodPost, "/api/benchmarkpro/competitors", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlerGetTrend(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	r := setupTestRouter(m)

	req := httptest.NewRequest(http.MethodGet, "/api/benchmarkpro/trend?type=cpu", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d，得到 %d", http.StatusOK, w.Code)
	}
}

func TestCalculateOverallScore(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TmpDir = t.TempDir()
	m := NewManager(cfg)
	defer m.Stop()

	// 全部为 0
	result := &BenchResult{}
	score := m.calculateOverallScore(result)
	if score != 0 {
		t.Errorf("期望评分为 0，得到 %f", score)
	}

	// 有 CPU 评分
	result.CPUScore = 500
	score = m.calculateOverallScore(result)
	if score != 500 {
		t.Errorf("期望评分为 500，得到 %f", score)
	}

	// 多项评分
	result.MemScore = 600
	result.NetThroughputMbps = 1000
	score = m.calculateOverallScore(result)
	if score <= 0 {
		t.Error("综合评分应大于 0")
	}
}
