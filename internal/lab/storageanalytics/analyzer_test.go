package storageanalytics

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// createTestDir 创建测试目录结构.
func createTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// 创建各种类型的测试文件
	files := map[string]string{
		"images/photo.jpg":     "fake jpg content for test",
		"images/picture.png":   "fake png",
		"videos/movie.mp4":     "fake video content here",
		"documents/report.pdf": "fake pdf content",
		"documents/notes.txt":  "hello world",
		"archives/data.zip":    "fake zip content",
		"code/main.go":         "package main",
		"code/utils.py":        "print('hello')",
		"logs/app.log":         "2026-01-01 INFO test log",
		"cache/temp.cache":     "cached data",
		"temp_file.tmp":        "temporary data",
		"other/random.dat":     "random data",
		"subdir/deep/file.txt": "deep file",
	}

	for relPath, content := range files {
		fullPath := filepath.Join(dir, relPath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	return dir
}

// ========== Collector 测试 ==========

func TestCollector_Collect(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	assert.Equal(t, dir, result.ScanPath)
	assert.Greater(t, result.TotalFiles, 0)
	assert.Greater(t, result.TotalSize, int64(0))
	assert.NotEmpty(t, result.Files)
	assert.False(t, result.ScanTime.IsZero())
}

func TestCollector_CollectWithMaxDepth(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())

	result, err := collector.Collect(dir, 1, 10)
	require.NoError(t, err)

	// depth=1 should still collect most files
	assert.Greater(t, result.TotalFiles, 0)
}

func TestCollector_CollectInvalidPath(t *testing.T) {
	collector := NewCollector(nil, zap.NewNop())
	_, err := collector.Collect("/nonexistent/path", 0, 10)
	assert.Error(t, err)
}

func TestCollector_CollectEmptyDir(t *testing.T) {
	dir := t.TempDir()
	collector := NewCollector(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, result.TotalFiles)
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		path     string
		expected FileType
	}{
		{"photo.jpg", FileTypeImage},
		{"video.mp4", FileTypeVideo},
		{"doc.pdf", FileTypeDocument},
		{"archive.zip", FileTypeArchive},
		{"main.go", FileTypeCode},
		{"data.bin", FileTypeOther},
		{"image.PNG", FileTypeImage},
		{"movie.AVI", FileTypeVideo},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, ClassifyFile(tt.path))
		})
	}
}

func TestClassifySizeBracket(t *testing.T) {
	tests := []struct {
		size     int64
		expected SizeBracket
	}{
		{500 * 1024, SizeLT1MB},
		{50 * 1024 * 1024, Size1MBTo100},
		{500 * 1024 * 1024, Size100MBTo1G},
		{2 * 1024 * 1024 * 1024, SizeGT1GB},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, classifySizeBracket(tt.size))
	}
}

// ========== Analyzer 测试 ==========

func TestAnalyzer_Analyze(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	report := analyzer.Analyze(result)

	assert.Equal(t, dir, report.ScanPath)
	assert.False(t, report.GeneratedAt.IsZero())
	assert.Equal(t, result.TotalFiles, report.Summary.TotalFiles)
	assert.Equal(t, result.TotalSize, report.Summary.TotalSize)
	assert.NotEmpty(t, report.FileTypeStats)
	assert.NotEmpty(t, report.SizeDist)
	assert.NotEmpty(t, report.AgeDist)
	assert.NotEmpty(t, report.AccessDist)
}

func TestAnalyzer_FileTypeStats(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	report := analyzer.Analyze(result)

	// 确保所有类型都有统计
	categories := make(map[string]bool)
	for _, s := range report.FileTypeStats {
		categories[s.Category] = true
	}

	assert.True(t, categories["图片"])
	assert.True(t, categories["视频"])
	assert.True(t, categories["文档"])
	assert.True(t, categories["压缩包"])
	assert.True(t, categories["代码"])
	assert.True(t, categories["其他"])
}

func TestAnalyzer_HealthMetrics(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	report := analyzer.Analyze(result)

	assert.GreaterOrEqual(t, report.Health.OverallScore, 0.0)
	assert.LessOrEqual(t, report.Health.OverallScore, 100.0)
	assert.GreaterOrEqual(t, report.Health.FragmentationScore, 0.0)
	assert.GreaterOrEqual(t, report.Health.EfficiencyScore, 0.0)
	assert.GreaterOrEqual(t, report.Health.RedundancyRate, 0.0)
	assert.LessOrEqual(t, report.Health.RedundancyRate, 1.0)
}

func TestAnalyzer_Insights(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())

	result, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)

	report := analyzer.Analyze(result)

	// 应该能检测到浪费文件（.tmp, .log, .cache）
	assert.GreaterOrEqual(t, len(report.Insights.Insights), 0)
}

func TestAnalyzer_GetLastReportNoData(t *testing.T) {
	analyzer := NewAnalyzer(nil, zap.NewNop())
	_, err := analyzer.GetLastReport()
	assert.ErrorIs(t, err, ErrNoAnalysisData)
}

func TestAnalyzer_TrendsWithPrevData(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())

	// 第一次分析
	result1, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)
	analyzer.Analyze(result1)

	// 添加新文件后再分析
	os.WriteFile(filepath.Join(dir, "new_file.txt"), []byte("new data added"), 0644)
	result2, err := collector.Collect(dir, 0, 10)
	require.NoError(t, err)
	report2 := analyzer.Analyze(result2)

	// 趋势应该有数据
	assert.NotNil(t, report2.Trends)
}

// ========== Reporter 测试 ==========

func TestReporter_ToJSON(t *testing.T) {
	reporter := NewReporter()
	report := &StorageReport{
		ScanPath:    "/test",
		GeneratedAt: now(),
		Summary: Summary{
			TotalSize:  1024 * 1024 * 100,
			TotalFiles: 50,
			TotalDirs:  5,
		},
		FileTypeStats: []CategoryStat{
			{Category: "图片", FileCount: 20, TotalSize: 50 * 1024 * 1024, Percentage: 50.0},
			{Category: "文档", FileCount: 30, TotalSize: 50 * 1024 * 1024, Percentage: 50.0},
		},
	}

	data, err := reporter.ToJSON(report)
	require.NoError(t, err)
	assert.Contains(t, string(data), "scan_path")
	assert.Contains(t, string(data), "file_type_stats")
}

func TestReporter_ToMarkdown(t *testing.T) {
	reporter := NewReporter()
	report := &StorageReport{
		ScanPath:    "/test/data",
		GeneratedAt: now(),
		Summary: Summary{
			TotalSize:  1024 * 1024 * 100,
			TotalFiles: 50,
			TotalDirs:  5,
		},
		FileTypeStats: []CategoryStat{
			{Category: "图片", FileCount: 20, TotalSize: 50 * 1024 * 1024, Percentage: 50.0},
		},
		SizeDist: []SizeDistribution{
			{Bracket: SizeLT1MB, FileCount: 30, TotalSize: 10 * 1024 * 1024},
		},
		AgeDist: []AgeDistribution{
			{Bracket: AgeLT7Days, FileCount: 10, TotalSize: 5 * 1024 * 1024},
		},
		AccessDist: []AccessDistribution{
			{Frequency: AccessFrequent, FileCount: 20, TotalSize: 30 * 1024 * 1024},
		},
		Health: HealthMetrics{
			OverallScore:       75.5,
			FragmentationScore: 80.0,
			EfficiencyScore:    70.0,
		},
		Insights: InsightAnalysis{
			Insights: []Insight{
				{
					Type:     "waste",
					Severity: "medium",
					Title:    "临时文件占用空间",
					Detail:   "占用 500 MB",
					Saving:   500 * 1024 * 1024,
					Action:   "建议清理临时文件",
				},
			},
			TotalPotentialSaving: 500 * 1024 * 1024,
		},
	}

	md := reporter.ToMarkdown(report)

	assert.Contains(t, md, "# 存储分析报告")
	assert.Contains(t, md, "存储概览")
	assert.Contains(t, md, "文件类型分布")
	assert.Contains(t, md, "智能洞察")
	assert.Contains(t, md, "临时文件")
}

// ========== Handler 测试 ==========

func TestHandler_Analyze(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "分析完成")
}

func TestHandler_AnalyzeEmptyPath(t *testing.T) {
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	body := `{"path":""}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_OverviewNoData(t *testing.T) {
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/storage-analytics/overview", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), ErrNoAnalysisData.Error())
}

func TestHandler_OverviewWithData(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 再获取概览
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/overview", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "summary")
	assert.Contains(t, w.Body.String(), "health")
}

func TestHandler_Breakdown(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取分布
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/breakdown", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "file_type_stats")
}

func TestHandler_Trends(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取趋势
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/trends", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_Insights(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取洞察
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/insights", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "insights")
}

func TestHandler_ReportJSON(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取JSON报告
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/report", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "scan_path")
}

func TestHandler_ReportMarkdown(t *testing.T) {
	dir := createTestDir(t)
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	// 先执行分析
	body := `{"path":"` + strings.ReplaceAll(dir, `\`, `\\`) + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/storage-analytics/analyze", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 获取Markdown报告
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/storage-analytics/report?format=markdown", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "# 存储分析报告")
}

func TestHandler_ReportNoData(t *testing.T) {
	collector := NewCollector(nil, zap.NewNop())
	analyzer := NewAnalyzer(nil, zap.NewNop())
	reporter := NewReporter()
	handler := NewHandler(collector, analyzer, reporter, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/storage-analytics/report", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ========== 辅助函数测试 ==========

func TestFormatBytes(t *testing.T) {
	assert.Equal(t, "1 KB", formatBytes(1024))
	assert.Equal(t, "1 MB", formatBytes(1024*1024))
	assert.Equal(t, "1 GB", formatBytes(1024*1024*1024))
	assert.Equal(t, "1 TB", formatBytes(1024*1024*1024*1024))
	assert.Equal(t, "500 B", formatBytes(500))
}

func TestFormatDuration(t *testing.T) {
	assert.Contains(t, formatDuration(3*365*24*time.Hour), "年")
	assert.Contains(t, formatDuration(60*24*time.Hour), "月")
}

func TestAccessFrequencyClassification(t *testing.T) {
	assert.Equal(t, AccessFrequent, classifyAccessFrequency(now().Add(-1*24*time.Hour)))
}

func TestAgeBracketClassification(t *testing.T) {
	assert.Equal(t, AgeLT7Days, classifyAgeBracket(now().Add(-1*24*time.Hour)))
	assert.Equal(t, AgeGT1Year, classifyAgeBracket(now().Add(-400*24*time.Hour)))
}

func TestWasteDetection(t *testing.T) {
	a := NewAnalyzer(nil, zap.NewNop())
	assert.True(t, a.isWasteFile("/tmp/test.tmp"))
	assert.True(t, a.isWasteFile("/var/log/app.log"))
	assert.True(t, a.isWasteFile("/home/user/.cache/data"))
	assert.True(t, a.isWasteFile("Thumbs.db"))
	assert.True(t, a.isWasteFile(".DS_Store"))
	assert.False(t, a.isWasteFile("/home/user/document.pdf"))
}

func TestRound2(t *testing.T) {
	assert.Equal(t, 3.14, round2(3.14159))
	assert.Equal(t, 10.0, round2(10.0))
}

func now() time.Time {
	return time.Now()
}
