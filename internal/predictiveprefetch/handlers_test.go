// Package predictiveprefetch provides tests for predictive prefetching.
package predictiveprefetch

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandler() (*Handler, *gin.Engine) {
	config := DefaultConfig()
	prefetch := NewPredictivePrefetch(config)
	handler := NewHandler(prefetch)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return handler, router
}

func TestRecordAccess(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("记录访问成功", func(t *testing.T) {
		body := `{
			"user_id": "user1",
			"file_path": "/data/file1.txt",
			"duration": 1.5
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/predictive-prefetch/access", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("缺少必填字段", func(t *testing.T) {
		body := `{"user_id": "user1"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/predictive-prefetch/access", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestPredict(t *testing.T) {
	handler, router := setupTestHandler()

	// Record some access patterns
	handler.prefetch.RecordAccess(nil, "user1", "/data/file1.txt", 1.0)
	handler.prefetch.RecordAccess(nil, "user1", "/data/file2.txt", 1.0)
	handler.prefetch.RecordAccess(nil, "user1", "/data/file3.txt", 1.0)

	t.Run("预测下一个文件", func(t *testing.T) {
		body := `{
			"user_id": "user1",
			"current_file": "/data/file1.txt"
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/predictive-prefetch/predict", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["predictions"])
	})
}

func TestPrefetch(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("预取文件", func(t *testing.T) {
		body := `{
			"candidates": [
				{
					"file_path": "/data/file1.txt",
					"score": 0.8,
					"reason": "sequential",
					"size": 1024,
					"priority": 1
				}
			]
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/predictive-prefetch/prefetch", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetCached(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("获取缓存列表", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/predictive-prefetch/cache", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["cached"])
		assert.NotNil(t, response["count"])
		assert.NotNil(t, response["size"])
	})
}

func TestClearCache(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("清除缓存", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/predictive-prefetch/cache", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetStats(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("获取统计信息", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/predictive-prefetch/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats PrefetchStats
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)
		assert.NotNil(t, stats)
	})
}

func TestGetConfig(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("获取配置", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/predictive-prefetch/config", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var config PrefetchConfig
		err := json.Unmarshal(w.Body.Bytes(), &config)
		require.NoError(t, err)
		assert.Equal(t, int64(1024*1024*1024), config.MaxCacheSize)
	})
}

func TestUpdateConfig(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("更新配置", func(t *testing.T) {
		body := `{
			"max_cache_size": 2147483648,
			"max_entries": 2000,
			"prefetch_threshold": 0.8
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/predictive-prefetch/config", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestEnableDisable(t *testing.T) {
	_, router := setupTestHandler()

	t.Run("启用预取", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/predictive-prefetch/enable", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("禁用预取", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/predictive-prefetch/disable", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSequentialPrediction(t *testing.T) {
	prefetch := NewPredictivePrefetch(DefaultConfig())

	// Record sequential access
	prefetch.RecordAccess(nil, "user1", "/data/a.txt", 1.0)
	prefetch.RecordAccess(nil, "user1", "/data/b.txt", 1.0)
	prefetch.RecordAccess(nil, "user1", "/data/a.txt", 1.0)
	prefetch.RecordAccess(nil, "user1", "/data/b.txt", 1.0)

	// Predict from a.txt
	candidates := prefetch.Predict(nil, "user1", "/data/a.txt")

	// Should predict b.txt
	found := false
	for _, c := range candidates {
		if c.FilePath == "/data/b.txt" {
			found = true
			assert.Greater(t, c.Score, 0.5)
			break
		}
	}
	assert.True(t, found, "Should predict b.txt after a.txt")
}

func TestCacheLRU(t *testing.T) {
	config := DefaultConfig()
	config.MaxCacheSize = 2048
	config.MaxEntries = 2
	prefetch := NewPredictivePrefetch(config)

	// Add entries
	prefetch.Prefetch(nil, []PrefetchCandidate{
		{FilePath: "/data/file1.txt", Size: 1024, Score: 0.9},
	})
	prefetch.Prefetch(nil, []PrefetchCandidate{
		{FilePath: "/data/file2.txt", Size: 1024, Score: 0.8},
	})

	assert.Equal(t, 2, prefetch.GetCacheCount())
	assert.Equal(t, int64(2048), prefetch.GetCacheSize())

	// Add third entry (should evict first)
	prefetch.Prefetch(nil, []PrefetchCandidate{
		{FilePath: "/data/file3.txt", Size: 1024, Score: 0.7},
	})

	assert.Equal(t, 2, prefetch.GetCacheCount())
	assert.Equal(t, int64(2048), prefetch.GetCacheSize())
}

func TestTouch(t *testing.T) {
	prefetch := NewPredictivePrefetch(DefaultConfig())

	// Add entry
	prefetch.Prefetch(nil, []PrefetchCandidate{
		{FilePath: "/data/file1.txt", Size: 1024, Score: 0.9},
	})

	// Touch it
	prefetch.Touch("/data/file1.txt")

	stats := prefetch.GetStats()
	assert.Equal(t, 1, stats.CacheHits)

	// Touch non-existent
	prefetch.Touch("/data/nonexistent.txt")

	stats = prefetch.GetStats()
	assert.Equal(t, 1, stats.CacheMisses)
}
