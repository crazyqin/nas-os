// Package smartfileorganizer provides tests for the smart file organizer.
package smartfileorganizer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandler(t *testing.T) (*Handler, *gin.Engine, string) {
	t.Helper()
	tmpDir := t.TempDir()
	organizer := NewOrganizer(tmpDir)
	handler := NewHandler(organizer)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return handler, router, tmpDir
}

func createTestFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, content, 0644)
	require.NoError(t, err)
	return path
}

func TestScan(t *testing.T) {
	_, router, tmpDir := setupTestHandler(t)

	// Create test files
	createTestFile(t, tmpDir, "test.pdf", []byte("PDF content"))
	createTestFile(t, tmpDir, "photo.jpg", []byte("JPEG content"))
	createTestFile(t, tmpDir, "music.mp3", []byte("MP3 content"))

	t.Run("扫描目录", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/smart-organize/scan", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var report OrganizationReport
		err := json.Unmarshal(w.Body.Bytes(), &report)
		require.NoError(t, err)
		assert.Equal(t, 3, report.ScannedFiles)
		assert.Equal(t, 1, report.CategoryCounts[CategoryDocument])
		assert.Equal(t, 1, report.CategoryCounts[CategoryImage])
		assert.Equal(t, 1, report.CategoryCounts[CategoryAudio])
	})
}

func TestRules(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	var ruleID string

	t.Run("添加规则", func(t *testing.T) {
		body := `{
			"id": "test-rule-1",
			"name": "整理文档",
			"enabled": true,
			"category": "document",
			"targetDir": "/organized/documents"
		}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/smart-organize/rules", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var rule OrganizationRule
		err := json.Unmarshal(w.Body.Bytes(), &rule)
		require.NoError(t, err)
		ruleID = rule.ID
		assert.Equal(t, "整理文档", rule.Name)
	})

	t.Run("获取规则列表", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/smart-organize/rules", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, int(response["total"].(float64)), 1)
	})

	t.Run("删除规则", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/smart-organize/rules/"+ruleID, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("删除不存在的规则", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/smart-organize/rules/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestOrganize(t *testing.T) {
	_, router, tmpDir := setupTestHandler(t)

	// Create test files
	createTestFile(t, tmpDir, "doc.pdf", []byte("PDF content"))
	createTestFile(t, tmpDir, "photo.jpg", []byte("JPEG content"))

	// Scan first
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/smart-organize/scan", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	t.Run("试运行整理", func(t *testing.T) {
		body := `{"dry_run": true}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/smart-organize/organize", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestFindDuplicates(t *testing.T) {
	_, router, tmpDir := setupTestHandler(t)

	// Create duplicate files
	createTestFile(t, tmpDir, "file1.txt", []byte("same content"))
	createTestFile(t, tmpDir, "file2.txt", []byte("same content"))

	// Scan first
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/smart-organize/scan", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	t.Run("查找重复文件", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/smart-organize/duplicates", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["duplicates"])
	})
}

func TestGetByCategory(t *testing.T) {
	_, router, tmpDir := setupTestHandler(t)

	// Create test files
	createTestFile(t, tmpDir, "doc.pdf", []byte("PDF content"))
	createTestFile(t, tmpDir, "photo.jpg", []byte("JPEG content"))

	// Scan first
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/smart-organize/scan", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	t.Run("获取文档类文件", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/smart-organize/category/document", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(1), response["total"])
	})

	t.Run("获取图片类文件", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/smart-organize/category/image", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(1), response["total"])
	})
}

func TestGetStats(t *testing.T) {
	_, router, tmpDir := setupTestHandler(t)

	// Create test files
	createTestFile(t, tmpDir, "test.pdf", []byte("PDF content"))

	// Scan first
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/smart-organize/scan", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	t.Run("获取统计信息", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/smart-organize/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)
		assert.Equal(t, float64(1), stats["totalFiles"])
	})
}

func TestCategoryDetection(t *testing.T) {
	tests := []struct {
		ext      string
		expected FileCategory
	}{
		{".pdf", CategoryDocument},
		{".jpg", CategoryImage},
		{".mp4", CategoryVideo},
		{".mp3", CategoryAudio},
		{".zip", CategoryArchive},
		{".go", CategoryCode},
		{".exe", CategoryExec},
		{".json", CategoryData},
		{".xyz", CategoryOther},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := categorizeByExt(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}
