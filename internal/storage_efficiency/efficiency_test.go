package storage_efficiency

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// newTestAnalyzer 创建用于测试的分析器.
func newTestAnalyzer(t *testing.T) (*Analyzer, string) {
	t.Helper()
	dir := t.TempDir()
	a := NewAnalyzer(dir)
	return a, dir
}

// createTestFiles 在指定目录创建测试文件.
func createTestFiles(t *testing.T, dir string) {
	t.Helper()

	// 文本文件（可压缩）
	for i := 0; i < 5; i++ {
		path := filepath.Join(dir, "text_"+string(rune('a'+i))+".txt")
		content := strings.Repeat("Hello NAS-OS storage efficiency test data! ", 1000)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	// JSON 文件（可压缩）
	for i := 0; i < 3; i++ {
		path := filepath.Join(dir, "data_"+string(rune('a'+i))+".json")
		content := `{"key": "value", "number": 12345, "nested": {"a": 1, "b": 2}}`
		if err := os.WriteFile(path, []byte(strings.Repeat(content, 500)), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	// 重复文件（用于去重测试）
	dupContent := strings.Repeat("duplicate content for dedup testing! ", 500)
	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, "dup_"+string(rune('a'+i))+".dat")
		if err := os.WriteFile(path, []byte(dupContent), 0644); err != nil {
			t.Fatalf("创建重复文件失败: %v", err)
		}
	}

	// 已压缩文件（jpg，不可再压缩）
	for i := 0; i < 2; i++ {
		path := filepath.Join(dir, "image_"+string(rune('a'+i))+".jpg")
		// 写入假的jpg数据
		data := make([]byte, 50000)
		data[0] = 0xFF
		data[1] = 0xD8 // JPEG magic bytes
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("创建jpg文件失败: %v", err)
		}
	}

	// 大文本文件
	path := filepath.Join(dir, "large_log.log")
	content := strings.Repeat("2024-01-01 INFO: Storage efficiency module test log entry\n", 10000)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建大日志文件失败: %v", err)
	}
}

// ========== 类型测试 ==========

func TestTypes_Constants(t *testing.T) {
	// 建议类型
	if SuggestTypeCompression != "compression" {
		t.Errorf("SuggestTypeCompression = %s, 期望 compression", SuggestTypeCompression)
	}
	if SuggestTypeDedup != "dedup" {
		t.Errorf("SuggestTypeDedup = %s, 期望 dedup", SuggestTypeDedup)
	}
	if SuggestTypeTiering != "tiering" {
		t.Errorf("SuggestTypeTiering = %s, 期望 tiering", SuggestTypeTiering)
	}

	// 优先级
	if PriorityHigh != "high" {
		t.Errorf("PriorityHigh = %s, 期望 high", PriorityHigh)
	}
	if PriorityMedium != "medium" {
		t.Errorf("PriorityMedium = %s, 期望 medium", PriorityMedium)
	}
	if PriorityLow != "low" {
		t.Errorf("PriorityLow = %s, 期望 low", PriorityLow)
	}

	// 任务状态
	if StatusRunning != "running" {
		t.Errorf("StatusRunning = %s, 期望 running", StatusRunning)
	}
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %s, 期望 completed", StatusCompleted)
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %s, 期望 failed", StatusFailed)
	}
}

// ========== 分析器测试 ==========

func TestNewAnalyzer(t *testing.T) {
	a, dir := newTestAnalyzer(t)
	if a == nil {
		t.Fatal("NewAnalyzer 返回 nil")
	}
	if a.dataDir != dir {
		t.Errorf("dataDir = %s, 期望 %s", a.dataDir, dir)
	}
	if a.sampleRate != 10 {
		t.Errorf("sampleRate = %d, 期望 10", a.sampleRate)
	}
	if a.history == nil {
		t.Fatal("history 未初始化")
	}
	if a.tasks == nil {
		t.Fatal("tasks 未初始化")
	}
}

func TestNewAnalyzer_DefaultDir(t *testing.T) {
	a := NewAnalyzer("")
	if a.dataDir != "/var/lib/nas-os/efficiency" {
		t.Errorf("默认 dataDir = %s, 期望 /var/lib/nas-os/efficiency", a.dataDir)
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	summary, err := a.Analyze(dir, 100, false)
	if err != nil {
		t.Fatalf("Analyze 失败: %v", err)
	}
	if summary == nil {
		t.Fatal("Analyze 返回 nil")
	}
	if summary.UpdatedAt.IsZero() {
		t.Error("UpdatedAt 不应为零值")
	}
}

func TestAnalyzer_Analyze_EmptyDir(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()

	summary, err := a.Analyze(dir, 100, false)
	if err != nil {
		t.Fatalf("空目录分析不应报错: %v", err)
	}
	if summary == nil {
		t.Fatal("空目录应返回非nil结果")
	}
	if summary.TotalLogicalSize != 0 {
		t.Errorf("空目录逻辑大小 = %d, 期望 0", summary.TotalLogicalSize)
	}
}

func TestAnalyzer_Analyze_DefaultPath(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	// 使用一个实际存在的临时目录作为默认路径
	dir := t.TempDir()

	summary, err := a.Analyze(dir, 100, false)
	if err != nil {
		t.Fatalf("默认路径分析不应报错: %v", err)
	}
	if summary == nil {
		t.Fatal("应返回非nil结果")
	}

	// 等待异步历史保存完成，避免TempDir清理竞争
	time.Sleep(300 * time.Millisecond)
}

func TestAnalyzer_Analyze_InvalidSampleRate(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	// 无效采样率应使用默认值
	summary, err := a.Analyze(dir, 0, false)
	if err != nil {
		t.Fatalf("无效采样率不应报错: %v", err)
	}
	if summary == nil {
		t.Fatal("应返回非nil结果")
	}

	// 超出范围的采样率
	summary, err = a.Analyze(dir, 200, false)
	if err != nil {
		t.Fatalf("超出范围采样率不应报错: %v", err)
	}
	if summary == nil {
		t.Fatal("应返回非nil结果")
	}
}

func TestAnalyzer_AnalyzeAsync(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	result := a.AnalyzeAsync(dir, 100, false)
	if result == nil {
		t.Fatal("AnalyzeAsync 返回 nil")
	}
	if result.TaskID == "" {
		t.Error("TaskID 不应为空")
	}
	if result.Status != StatusRunning {
		t.Errorf("初始状态 = %s, 期望 %s", result.Status, StatusRunning)
	}

	// 等待任务完成
	time.Sleep(500 * time.Millisecond)

	task := a.GetTask(result.TaskID)
	if task == nil {
		t.Fatal("GetTask 返回 nil")
	}
	if task.Status != StatusCompleted && task.Status != StatusFailed {
		t.Errorf("任务状态 = %s, 期望 completed 或 failed", task.Status)
	}
}

func TestAnalyzer_GetTask_NotFound(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	task := a.GetTask("nonexistent-task-id")
	if task != nil {
		t.Error("不存在的任务应返回 nil")
	}
}

func TestAnalyzer_GetCompressionStats(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	stats, err := a.GetCompressionStats(dir)
	if err != nil {
		t.Fatalf("GetCompressionStats 失败: %v", err)
	}
	if stats == nil {
		t.Fatal("GetCompressionStats 返回 nil")
	}
	// 应该有可压缩和不可压缩文件
	if stats.CompressedFiles+stats.UncompressedFiles == 0 {
		t.Error("应检测到文件")
	}
}

func TestAnalyzer_GetDedupStats(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	stats, err := a.GetDedupStats(dir)
	if err != nil {
		t.Fatalf("GetDedupStats 失败: %v", err)
	}
	if stats == nil {
		t.Fatal("GetDedupStats 返回 nil")
	}
	if stats.TotalFiles == 0 {
		t.Error("应检测到文件")
	}
}

func TestAnalyzer_GetTrends(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	// 先执行分析产生历史记录
	_, _ = a.Analyze(dir, 100, false)
	_, _ = a.Analyze(dir, 100, false)

	trends := a.GetTrends(30)
	if trends == nil {
		t.Fatal("GetTrends 返回 nil")
	}
	if trends.Days != 30 {
		t.Errorf("Days = %d, 期望 30", trends.Days)
	}
	if len(trends.Points) == 0 {
		t.Error("应有至少1个趋势数据点")
	}
}

func TestAnalyzer_GetTrends_DefaultDays(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	trends := a.GetTrends(0)
	if trends == nil {
		t.Fatal("GetTrends 返回 nil")
	}
	if trends.Days != 30 {
		t.Errorf("默认天数 = %d, 期望 30", trends.Days)
	}
}

// ========== 分析算法测试 ==========

func TestIsCompressible(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"file.txt", true},
		{"file.log", true},
		{"file.csv", true},
		{"file.json", true},
		{"file.xml", true},
		{"file.yaml", true},
		{"file.go", true},
		{"file.py", true},
		{"file.html", true},
		{"file.css", true},
		{"file.js", true},
		{"file.pdf", true},
		{"file.doc", true},
		{"file.xlsx", true},
		{"file.sql", true},
		{"file.jpg", false},
		{"file.png", false},
		{"file.gif", false},
		{"file.webp", false},
		{"file.mp4", false},
		{"file.mkv", false},
		{"file.mp3", false},
		{"file.flac", false},
		{"file.zip", false},
		{"file.rar", false},
		{"file.7z", false},
		{"file.gz", false},
		{"file.unknown", false},
	}

	for _, tt := range tests {
		result := isCompressible(tt.path)
		if result != tt.expected {
			t.Errorf("isCompressible(%s) = %v, 期望 %v", tt.path, result, tt.expected)
		}
	}
}

func TestEstimateCompressedSize(t *testing.T) {
	tests := []struct {
		path     string
		size     int64
		maxRatio float64 // 最大压缩比（压缩后/压缩前）
	}{
		{"test.txt", 10000, 0.35},  // 文本约30%
		{"test.log", 10000, 0.15},  // 日志约10%
		{"test.json", 10000, 0.25}, // JSON约20%
		{"test.pdf", 10000, 0.85},  // PDF约80%
		{"test.docx", 10000, 0.55}, // DOCX约50%
	}

	for _, tt := range tests {
		fi := fileInfo{
			path: tt.path,
			size: tt.size,
		}
		result := estimateCompressedSize(fi)
		if result <= 0 {
			t.Errorf("estimateCompressedSize(%s) 应 > 0, 实际 %d", tt.path, result)
		}
		if float64(result)/float64(tt.size) > tt.maxRatio {
			t.Errorf("estimateCompressedSize(%s) 压缩率 %.2f > 期望上限 %.2f",
				tt.path, float64(result)/float64(tt.size), tt.maxRatio)
		}
	}
}

func TestFileHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hash_test.txt")
	content := "test content for hash"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	hash1, err := fileHash(path)
	if err != nil {
		t.Fatalf("fileHash 失败: %v", err)
	}
	if hash1 == "" {
		t.Error("哈希值不应为空")
	}

	// 同一文件应产生相同哈希
	hash2, err := fileHash(path)
	if err != nil {
		t.Fatalf("fileHash 第二次失败: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("同一文件哈希不同: %s != %s", hash1, hash2)
	}

	// 不同文件应产生不同哈希
	path2 := filepath.Join(dir, "hash_test2.txt")
	if err := os.WriteFile(path2, []byte("different content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}
	hash3, err := fileHash(path2)
	if err != nil {
		t.Fatalf("fileHash 失败: %v", err)
	}
	if hash1 == hash3 {
		t.Error("不同文件不应有相同哈希")
	}
}

func TestFileHash_Nonexistent(t *testing.T) {
	_, err := fileHash("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("不存在的文件应返回错误")
	}
}

// ========== 优化器测试 ==========

func TestNewOptimizer(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	o := NewOptimizer(a)
	if o == nil {
		t.Fatal("NewOptimizer 返回 nil")
	}
	if o.analyzer != a {
		t.Error("analyzer 引用不正确")
	}
}

func TestOptimizer_GenerateSuggestions(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	o := NewOptimizer(a)
	suggestions, err := o.GenerateSuggestions(dir)
	if err != nil {
		t.Fatalf("GenerateSuggestions 失败: %v", err)
	}
	if suggestions == nil {
		t.Fatal("GenerateSuggestions 返回 nil")
	}
	// 建议列表可能为空（正常情况）
}

func TestOptimizer_GenerateSuggestions_EmptyDir(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()

	o := NewOptimizer(a)
	suggestions, err := o.GenerateSuggestions(dir)
	if err != nil {
		t.Fatalf("空目录建议生成不应报错: %v", err)
	}
	if suggestions == nil {
		t.Fatal("应返回非nil切片")
	}
}

func TestOptimizer_GenerateSuggestions_DefaultPath(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	o := NewOptimizer(a)

	suggestions, err := o.GenerateSuggestions("")
	if err != nil {
		t.Fatalf("默认路径建议生成不应报错: %v", err)
	}
	if suggestions == nil {
		t.Fatal("应返回非nil切片")
	}
}

func TestSortSuggestions(t *testing.T) {
	suggestions := []Suggestion{
		{ID: "low1", Priority: PriorityLow},
		{ID: "high1", Priority: PriorityHigh},
		{ID: "med1", Priority: PriorityMedium},
		{ID: "high2", Priority: PriorityHigh},
		{ID: "low2", Priority: PriorityLow},
	}

	sortSuggestions(suggestions)

	// 验证排序：high在前，medium中间，low在后
	expectedOrder := []string{PriorityHigh, PriorityHigh, PriorityMedium, PriorityLow, PriorityLow}
	for i, s := range suggestions {
		if s.Priority != expectedOrder[i] {
			t.Errorf("位置 %d: 优先级 = %s, 期望 %s", i, s.Priority, expectedOrder[i])
		}
	}
}

func TestSortSuggestions_Empty(t *testing.T) {
	suggestions := []Suggestion{}
	sortSuggestions(suggestions)
	if len(suggestions) != 0 {
		t.Error("空切片排序后应仍为空")
	}
}

func TestSortSuggestions_Single(t *testing.T) {
	suggestions := []Suggestion{
		{ID: "only", Priority: PriorityMedium},
	}
	sortSuggestions(suggestions)
	if len(suggestions) != 1 {
		t.Error("单元素切片排序后应仍为1个元素")
	}
	if suggestions[0].Priority != PriorityMedium {
		t.Error("单元素排序不应改变内容")
	}
}

// ========== 处理器测试 ==========

func setupTestRouter(t *testing.T) (*gin.Engine, *Handlers) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	a, _ := newTestAnalyzer(t)
	o := NewOptimizer(a)
	h := NewHandlers(a, o)

	r := gin.New()
	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1)

	return r, h
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	r, _ := setupTestRouter(t)
	routes := r.Routes()

	expectedPaths := map[string]string{
		"/api/v1/storage/efficiency/summary":     "GET",
		"/api/v1/storage/efficiency/compression":  "GET",
		"/api/v1/storage/efficiency/dedup":        "GET",
		"/api/v1/storage/efficiency/suggestions":  "GET",
		"/api/v1/storage/efficiency/analyze":      "POST",
		"/api/v1/storage/efficiency/trends":       "GET",
	}

	for path, method := range expectedPaths {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == method {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("未注册路由: %s %s", method, path)
		}
	}
}

func TestHandlers_GetSummary(t *testing.T) {
	r, _ := setupTestRouter(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	req := httptest.NewRequest("GET", "/api/v1/storage/efficiency/summary?path="+dir+"&sampleRate=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("code = %v, 期望 0", resp["code"])
	}
	if resp["data"] == nil {
		t.Error("data 不应为 nil")
	}
}

func TestHandlers_GetCompression(t *testing.T) {
	r, _ := setupTestRouter(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	req := httptest.NewRequest("GET", "/api/v1/storage/efficiency/compression?path="+dir+"&sampleRate=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["data"] == nil {
		t.Error("data 不应为 nil")
	}
}

func TestHandlers_GetDedup(t *testing.T) {
	r, _ := setupTestRouter(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	req := httptest.NewRequest("GET", "/api/v1/storage/efficiency/dedup?path="+dir+"&sampleRate=100", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["data"] == nil {
		t.Error("data 不应为 nil")
	}
}

func TestHandlers_GetSuggestions(t *testing.T) {
	r, _ := setupTestRouter(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	req := httptest.NewRequest("GET", "/api/v1/storage/efficiency/suggestions?path="+dir, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["data"] == nil {
		t.Error("data 不应为 nil")
	}
}

func TestHandlers_TriggerAnalyze(t *testing.T) {
	r, _ := setupTestRouter(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	body := `{"path":"` + dir + `","sampleRate":100}`
	req := httptest.NewRequest("POST", "/api/v1/storage/efficiency/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("状态码 = %d, 期望 %d", w.Code, http.StatusAccepted)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if data["taskId"] == nil || data["taskId"] == "" {
		t.Error("taskId 不应为空")
	}
	if data["status"] != StatusRunning {
		t.Errorf("初始状态 = %v, 期望 %s", data["status"], StatusRunning)
	}
}

func TestHandlers_TriggerAnalyze_EmptyBody(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest("POST", "/api/v1/storage/efficiency/analyze", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 空请求体应该能正常处理
	if w.Code != http.StatusAccepted {
		t.Errorf("空请求体状态码 = %d, 期望 %d", w.Code, http.StatusAccepted)
	}
}

func TestHandlers_GetTrends(t *testing.T) {
	r, h := setupTestRouter(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	// 先执行分析产生历史数据
	_, _ = h.analyzer.Analyze(dir, 100, false)

	req := httptest.NewRequest("GET", "/api/v1/storage/efficiency/trends?days=30", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if data["days"].(float64) != 30 {
		t.Errorf("days = %v, 期望 30", data["days"])
	}
}

func TestHandlers_GetTrends_InvalidDays(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/v1/storage/efficiency/trends?days=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("无效天数应返回200，实际 %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if data["days"].(float64) != 30 {
		t.Errorf("无效天数应默认30, 实际 %v", data["days"])
	}
}

func TestParseSampleRate(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"10", 10},
		{"50", 50},
		{"100", 100},
		{"0", 10},   // 无效值应返回默认值
		{"-1", 10},  // 负值应返回默认值
		{"200", 10}, // 超出范围应返回默认值
		{"abc", 10}, // 非数字应返回默认值
		{"", 10},    // 空字符串应返回默认值
	}

	for _, tt := range tests {
		result := parseSampleRate(tt.input)
		if result != tt.expected {
			t.Errorf("parseSampleRate(%q) = %d, 期望 %d", tt.input, result, tt.expected)
		}
	}
}

// ========== 历史记录持久化测试 ==========

func TestAnalyzer_HistoryPersistence(t *testing.T) {
	dir := t.TempDir()
	a := NewAnalyzer(dir)
	testDir := t.TempDir()
	createTestFiles(t, testDir)

	// 执行分析产生历史
	_, _ = a.Analyze(testDir, 100, false)

	// 创建新分析器加载同一目录
	a2 := NewAnalyzer(dir)
	if len(a2.history) == 0 {
		// 历史可能还在异步写入中，等待一下
		time.Sleep(200 * time.Millisecond)
		a2 = NewAnalyzer(dir)
	}
	if len(a2.history) == 0 {
		t.Error("新分析器应加载历史记录")
	}
}

// ========== JSON 序列化测试 ==========

func TestEfficiencySummary_JSON(t *testing.T) {
	summary := &EfficiencySummary{
		TotalLogicalSize:  1024 * 1024 * 1024,
		TotalPhysicalSize: 512 * 1024 * 1024,
		CompressionRatio:  2.0,
		DedupRatio:        0.5,
		SpaceSaved:        512 * 1024 * 1024,
		SpaceSavedPercent: 50.0,
		UpdatedAt:         time.Now(),
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var result EfficiencySummary
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if result.TotalLogicalSize != summary.TotalLogicalSize {
		t.Errorf("TotalLogicalSize = %d, 期望 %d", result.TotalLogicalSize, summary.TotalLogicalSize)
	}
	if result.CompressionRatio != summary.CompressionRatio {
		t.Errorf("CompressionRatio = %f, 期望 %f", result.CompressionRatio, summary.CompressionRatio)
	}
}

func TestCompressionStats_JSON(t *testing.T) {
	stats := &CompressionStats{
		CompressedFiles:     100,
		UncompressedFiles:   50,
		AverageRatio:        2.5,
		BestRatio:           5.0,
		WorstRatio:          1.2,
		TotalOriginalSize:   1024 * 1024,
		TotalCompressedSize: 512 * 1024,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var result CompressionStats
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if result.CompressedFiles != stats.CompressedFiles {
		t.Errorf("CompressedFiles = %d, 期望 %d", result.CompressedFiles, stats.CompressedFiles)
	}
}

func TestSuggestion_JSON(t *testing.T) {
	s := Suggestion{
		ID:          "test-id",
		Type:        SuggestTypeCompression,
		Priority:    PriorityHigh,
		Title:       "测试建议",
		Description: "这是测试建议描述",
		PotentialMB: 1024,
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var result Suggestion
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if result.ID != s.ID {
		t.Errorf("ID = %s, 期望 %s", result.ID, s.ID)
	}
	if result.Type != s.Type {
		t.Errorf("Type = %s, 期望 %s", result.Type, s.Type)
	}
	if result.Priority != s.Priority {
		t.Errorf("Priority = %s, 期望 %s", result.Priority, s.Priority)
	}
	if result.PotentialMB != s.PotentialMB {
		t.Errorf("PotentialMB = %d, 期望 %d", result.PotentialMB, s.PotentialMB)
	}
}

// ========== 边界条件测试 ==========

func TestAnalyzer_Analyze_DeepScan(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	summary, err := a.Analyze(dir, 100, true)
	if err != nil {
		t.Fatalf("深度扫描失败: %v", err)
	}
	if summary == nil {
		t.Fatal("深度扫描应返回非nil结果")
	}
}

func TestAnalyzer_Analyze_LargeSampleRate(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	// 100% 采样率应该扫描所有文件
	summary, err := a.Analyze(dir, 100, false)
	if err != nil {
		t.Fatalf("分析失败: %v", err)
	}
	if summary == nil {
		t.Fatal("应返回非nil结果")
	}
}

func TestAnalyzer_Analyze_SmallSampleRate(t *testing.T) {
	a, _ := newTestAnalyzer(t)
	dir := t.TempDir()
	createTestFiles(t, dir)

	// 1% 采样率
	summary, err := a.Analyze(dir, 1, false)
	if err != nil {
		t.Fatalf("低采样率分析失败: %v", err)
	}
	if summary == nil {
		t.Fatal("应返回非nil结果")
	}
}
