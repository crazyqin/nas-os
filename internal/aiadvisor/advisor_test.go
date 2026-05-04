package aiadvisor

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

// newTestAdvisor 创建测试用顾问实例.
func newTestAdvisor() *Advisor {
	return NewAdvisor(zap.NewNop(), nil)
}

// createTestDir 创建测试目录结构.
func createTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// 创建一些测试文件
	files := map[string]string{
		"small.txt":    "hello world",
		"medium.log":   strings.Repeat("log entry\n", 1000),
		"data.csv":     "col1,col2\n" + strings.Repeat("a,b\n", 500),
		"config.json":  `{"key": "value"}`,
		"sub/note.txt": "subdirectory file",
	}

	// 创建大文件（2MB）
	bigContent := make([]byte, 2*1024*1024)
	files["bigfile.bin"] = string(bigContent)

	// 创建重复文件
	dupContent := "duplicate content for testing hash dedup"
	files["dup_a.txt"] = dupContent
	files["subdir/dup_b.txt"] = dupContent

	for name, content := range files {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0755)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	return dir
}

// ========== 类型测试 ==========

func TestDefaultScanConfig(t *testing.T) {
	cfg := DefaultScanConfig()
	assert.Equal(t, "/", cfg.RootPath)
	assert.Equal(t, 100, cfg.LargeFileThresholdMB)
	assert.Equal(t, 90, cfg.StaleDays)
	assert.Equal(t, 10, cfg.MaxDepth)
	assert.True(t, cfg.EnableDedupCheck)
}

// ========== 辅助函数测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
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

func TestFileHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("test content for hashing"), 0644)

	hash1, err := fileHash(path)
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)

	// 相同文件应产生相同哈希
	hash2, err := fileHash(path)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2)
}

func TestGetAccessTime(t *testing.T) {
	info, err := os.Stat("/dev/null")
	if err != nil {
		t.Skip("无法获取/dev/null信息")
	}
	atime := getAccessTime(info)
	assert.False(t, atime.IsZero())
}

func TestCalculateCapacityGrowthRate(t *testing.T) {
	t.Run("正常增长", func(t *testing.T) {
		base := time.Now().AddDate(0, -3, 0)
		history := []CapacityDataPoint{
			{Timestamp: base, UsedBytes: 100 * 1024 * 1024 * 1024, TotalBytes: 500 * 1024 * 1024 * 1024},
			{Timestamp: base.AddDate(0, 1, 0), UsedBytes: 110 * 1024 * 1024 * 1024, TotalBytes: 500 * 1024 * 1024 * 1024},
			{Timestamp: base.AddDate(0, 2, 0), UsedBytes: 120 * 1024 * 1024 * 1024, TotalBytes: 500 * 1024 * 1024 * 1024},
			{Timestamp: base.AddDate(0, 3, 0), UsedBytes: 130 * 1024 * 1024 * 1024, TotalBytes: 500 * 1024 * 1024 * 1024},
		}
		gb, pct := calculateCapacityGrowthRate(history)
		assert.Greater(t, gb, 0.0)
		assert.Greater(t, pct, 0.0)
	})

	t.Run("数据不足", func(t *testing.T) {
		history := []CapacityDataPoint{
			{Timestamp: time.Now(), UsedBytes: 100 * 1024 * 1024 * 1024, TotalBytes: 500 * 1024 * 1024 * 1024},
		}
		gb, pct := calculateCapacityGrowthRate(history)
		assert.Equal(t, 0.0, gb)
		assert.Equal(t, 0.0, pct)
	})
}

// ========== Advisor 核心测试 ==========

func TestNewAdvisor(t *testing.T) {
	t.Run("默认配置", func(t *testing.T) {
		a := NewAdvisor(nil, nil)
		assert.NotNil(t, a)
		assert.NotNil(t, a.logger)
		assert.NotNil(t, a.config)
	})

	t.Run("自定义配置", func(t *testing.T) {
		cfg := &ScanConfig{
			RootPath:             "/tmp",
			LargeFileThresholdMB: 200,
			StaleDays:            30,
			MaxDepth:             5,
			EnableDedupCheck:     false,
		}
		a := NewAdvisor(zap.NewNop(), cfg)
		assert.NotNil(t, a)
		assert.Equal(t, "/tmp", a.config.RootPath)
		assert.Equal(t, 200, a.config.LargeFileThresholdMB)
	})
}

func TestScan(t *testing.T) {
	a := newTestAdvisor()
	dir := createTestDir(t)

	t.Run("正常扫描", func(t *testing.T) {
		cfg := &ScanConfig{
			RootPath:             dir,
			LargeFileThresholdMB: 1, // 1MB阈值
			StaleDays:            0, // 所有文件都算陈旧
			MaxDepth:             10,
			EnableDedupCheck:     true,
		}
		result, err := a.Scan(cfg)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.TotalFiles, 0)
		assert.Greater(t, result.TotalSizeBytes, int64(0))
		assert.NotZero(t, result.ScanStartedAt)
		assert.NotZero(t, result.ScanFinishedAt)
		assert.Greater(t, result.DurationSeconds, 0.0)

		// 应该检测到大文件（阈值1MB，bigfile.bin是2MB）
		assert.Greater(t, len(result.LargeFiles), 0, "应检测到大文件")

		// 应该检测到重复文件
		assert.Greater(t, len(result.DuplicateGroups), 0, "应检测到重复文件组")
		if len(result.DuplicateGroups) > 0 {
			assert.Equal(t, 2, result.DuplicateGroups[0].Count)
			assert.Greater(t, result.DuplicateGroups[0].WastedBytes, int64(0))
		}

		// 扩展名统计应有数据
		assert.NotEmpty(t, result.ExtensionSummary)

		// 扫描后应有建议
		recs, err := a.GetRecommendations()
		require.NoError(t, err)
		assert.Greater(t, len(recs), 0, "扫描后应生成优化建议")
	})

	t.Run("无效路径", func(t *testing.T) {
		cfg := &ScanConfig{
			RootPath: "/nonexistent/path/that/does/not/exist",
		}
		_, err := a.Scan(cfg)
		assert.ErrorIs(t, err, ErrInvalidPath)
	})
}

func TestGetRecommendations(t *testing.T) {
	a := newTestAdvisor()

	t.Run("无扫描数据", func(t *testing.T) {
		_, err := a.GetRecommendations()
		assert.ErrorIs(t, err, ErrNoScanData)
	})

	t.Run("有扫描数据", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := &ScanConfig{
			RootPath:             dir,
			LargeFileThresholdMB: 1,
			StaleDays:            0,
			MaxDepth:             10,
			EnableDedupCheck:     true,
		}
		_, err := a.Scan(cfg)
		require.NoError(t, err)

		recs, err := a.GetRecommendations()
		require.NoError(t, err)
		assert.NotEmpty(t, recs)

		// 检查建议字段完整性
		for _, r := range recs {
			assert.NotEmpty(t, r.ID)
			assert.NotEmpty(t, r.Type)
			assert.NotEmpty(t, r.Title)
			assert.NotEmpty(t, r.Description)
			assert.GreaterOrEqual(t, r.Priority, 1)
			assert.LessOrEqual(t, r.Priority, 3)
			assert.False(t, r.Applied)
		}
	})
}

func TestGetReport(t *testing.T) {
	a := newTestAdvisor()

	t.Run("无扫描数据", func(t *testing.T) {
		_, err := a.GetReport()
		assert.ErrorIs(t, err, ErrNoScanData)
	})

	t.Run("有扫描数据", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := &ScanConfig{
			RootPath:             dir,
			LargeFileThresholdMB: 1,
			StaleDays:            0,
			MaxDepth:             10,
			EnableDedupCheck:     true,
		}
		_, err := a.Scan(cfg)
		require.NoError(t, err)

		report, err := a.GetReport()
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.Greater(t, report.ScanSummary.TotalFiles, 0)
		assert.NotEmpty(t, report.Recommendations)
		assert.GreaterOrEqual(t, report.TotalEstimatedSaving, int64(0))
		assert.False(t, report.GeneratedAt.IsZero())
	})
}

func TestCapacityForecast(t *testing.T) {
	a := newTestAdvisor()

	t.Run("数据不足", func(t *testing.T) {
		_, err := a.GetCapacityForecast(12)
		assert.ErrorIs(t, err, ErrInsufficientHistory)
	})

	t.Run("有足够数据", func(t *testing.T) {
		base := time.Now().AddDate(0, -6, 0)
		for i := 0; i < 6; i++ {
			used := int64(float64(100*1024*1024*1024) * (1 + float64(i)*0.1))
			a.AddCapacityData(CapacityDataPoint{
				Timestamp:  base.AddDate(0, i, 0),
				UsedBytes:  used,
				TotalBytes: 500 * 1024 * 1024 * 1024,
			})
		}

		forecast, err := a.GetCapacityForecast(6)
		require.NoError(t, err)
		assert.NotNil(t, forecast)
		assert.Greater(t, forecast.CurrentUsedBytes, int64(0))
		assert.Greater(t, forecast.CurrentTotalBytes, int64(0))
		assert.Len(t, forecast.Predictions, 6)
		assert.NotEmpty(t, forecast.UrgencyLevel)
		assert.False(t, forecast.GeneratedAt.IsZero())
	})

	t.Run("默认预测月数", func(t *testing.T) {
		forecast, err := a.GetCapacityForecast(0)
		require.NoError(t, err)
		assert.Len(t, forecast.Predictions, 12)
	})
}

func TestApplyRecommendation(t *testing.T) {
	a := newTestAdvisor()
	dir := createTestDir(t)

	cfg := &ScanConfig{
		RootPath:             dir,
		LargeFileThresholdMB: 1,
		StaleDays:            0,
		MaxDepth:             10,
		EnableDedupCheck:     true,
	}
	_, err := a.Scan(cfg)
	require.NoError(t, err)

	recs, err := a.GetRecommendations()
	require.NoError(t, err)
	require.NotEmpty(t, recs)

	t.Run("正常应用", func(t *testing.T) {
		rec, err := a.ApplyRecommendation(recs[0].ID)
		require.NoError(t, err)
		assert.True(t, rec.Applied)
		assert.NotNil(t, rec.AppliedAt)
	})

	t.Run("重复应用", func(t *testing.T) {
		rec, err := a.ApplyRecommendation(recs[0].ID)
		require.NoError(t, err)
		assert.True(t, rec.Applied)
	})

	t.Run("不存在的ID", func(t *testing.T) {
		_, err := a.ApplyRecommendation("nonexistent-id")
		assert.ErrorIs(t, err, ErrRecommendationNotFound)
	})
}

func TestAddCapacityData(t *testing.T) {
	a := newTestAdvisor()
	a.AddCapacityData(CapacityDataPoint{
		Timestamp:  time.Now(),
		UsedBytes:  100 * 1024 * 1024 * 1024,
		TotalBytes: 500 * 1024 * 1024 * 1024,
	})
	assert.Len(t, a.capacityHistory, 1)
}

// ========== Handlers 测试 ==========

func TestHandlers(t *testing.T) {
	a := newTestAdvisor()
	dir := createTestDir(t)
	h := NewHandlers(a, zap.NewNop())
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	// 先扫描
	t.Run("启动扫描", func(t *testing.T) {
		body := `{"root_path": "` + strings.ReplaceAll(dir, `\`, `\\`) + `", "large_file_threshold_mb": 1, "stale_days": 0, "max_depth": 10, "enable_dedup_check": true}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai-advisor/scan", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "存储扫描完成")
	})

	t.Run("获取建议列表", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai-advisor/recommendations", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "recommendations")
	})

	t.Run("获取报告", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai-advisor/report", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "scan_summary")
	})

	t.Run("容量预测-数据不足", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai-advisor/capacity-forecast?months=6", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("应用建议", func(t *testing.T) {
		// 先获取建议列表
		recs, _ := a.GetRecommendations()
		if len(recs) > 0 {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/ai-advisor/apply/"+recs[0].ID, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), "建议已应用")
		}
	})

	t.Run("应用不存在的建议", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/ai-advisor/apply/nonexistent", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandlersNoScanData(t *testing.T) {
	a := newTestAdvisor()
	h := NewHandlers(a, zap.NewNop())
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	t.Run("无扫描数据-建议", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai-advisor/recommendations", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("无扫描数据-报告", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai-advisor/report", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandlersCapacityForecastWithData(t *testing.T) {
	a := newTestAdvisor()
	h := NewHandlers(a, zap.NewNop())
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	// 添加容量数据
	base := time.Now().AddDate(0, -6, 0)
	for i := 0; i < 6; i++ {
		used := int64(float64(100*1024*1024*1024) * (1 + float64(i)*0.1))
		a.AddCapacityData(CapacityDataPoint{
			Timestamp:  base.AddDate(0, i, 0),
			UsedBytes:  used,
			TotalBytes: 500 * 1024 * 1024 * 1024,
		})
	}

	t.Run("容量预测-有数据", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai-advisor/capacity-forecast?months=6", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "predictions")
	})

	t.Run("容量预测-默认月数", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ai-advisor/capacity-forecast", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
