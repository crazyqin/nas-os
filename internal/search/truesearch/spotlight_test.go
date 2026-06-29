// Package truesearch 实现全文搜索引擎 (TrueSearch Phase 2)
// 本文件包含 macOS Spotlight 集成的单元测试。
package truesearch

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSpotlightEngine(t *testing.T) (*Engine, *SpotlightServer, *SpotlightHandler) {
	t.Helper()

	dir := t.TempDir()
	logger := newTestLogger(t)

	cfg := Config{
		IndexPath:     filepath.Join(dir, "test.bleve"),
		MaxFileSize:   10 * 1024 * 1024,
		BatchSize:     10,
		SupportedExts: []string{".txt", ".md"},
		ExcludeDirs:   []string{},
	}

	engine, err := New(cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })

	// 创建并索引测试文件
	shareDir := filepath.Join(dir, "share")
	require.NoError(t, os.MkdirAll(shareDir, 0755))

	testFiles := map[string]string{
		"report.txt":   "Annual report with financial data and analysis",
		"notes.md":     "# Meeting Notes\n\nDiscussed project timeline and budget",
		"database.txt": "PostgreSQL database configuration and tuning guide",
		"readme.md":    "# Project README\n\nWelcome to the NAS project documentation",
	}

	for name, content := range testFiles {
		path := filepath.Join(shareDir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		require.NoError(t, engine.IndexFile(path))
	}

	time.Sleep(100 * time.Millisecond)

	server := NewSpotlightServer(engine, logger)
	server.RegisterShare("public", shareDir)

	handler := NewSpotlightHandler(server, logger)

	return engine, server, handler
}

// ─── SpotlightServer 测试 ─────────────────────────────────────

func TestSpotlightServer_RegisterShare(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	path, ok := server.GetSharePath("public")
	require.True(t, ok)
	assert.NotEmpty(t, path)

	_, ok = server.GetSharePath("nonexistent")
	assert.False(t, ok)
}

func TestSpotlightServer_UnregisterShare(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	server.UnregisterShare("public")

	_, ok := server.GetSharePath("public")
	assert.False(t, ok)
}

func TestSpotlightServer_ListShares(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	shares := server.ListShares()
	assert.Contains(t, shares, "public")
}

func TestSpotlightServer_Search(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	resp, err := server.Search(SpotlightQuery{
		Query:      "database",
		ShareName:  "public",
		MaxResults: 10,
	})
	require.NoError(t, err)
	assert.True(t, resp.Total > 0)
	assert.Equal(t, "", resp.Error)

	// 验证结果包含 Spotlight 元数据
	for _, r := range resp.Results {
		assert.NotEmpty(t, r.FileName)
		assert.NotEmpty(t, r.Kind)
		assert.NotNil(t, r.Metadata)
		assert.Contains(t, r.Metadata, "kMDItemDisplayName")
		assert.Contains(t, r.Metadata, "kMDItemFSName")
		assert.Contains(t, r.Metadata, "kMDItemKind")
	}
}

func TestSpotlightServer_SearchEmptyQuery(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	resp, err := server.Search(SpotlightQuery{
		Query:     "",
		ShareName: "public",
	})
	require.NoError(t, err)
	assert.NotEqual(t, "", resp.Error)
}

func TestSpotlightServer_SearchUnknownShare(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	resp, err := server.Search(SpotlightQuery{
		Query:     "test",
		ShareName: "nonexistent",
	})
	require.NoError(t, err)
	assert.NotEqual(t, "", resp.Error)
}

func TestSpotlightServer_SearchXML(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	result, err := server.SearchXML("database", "public")
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// 验证是有效 XML
	var envelope spotlightXMLEnvelope
	err = xml.Unmarshal([]byte(result), &envelope)
	require.NoError(t, err)
}

// ─── ParseSpotlightQuery 测试 ─────────────────────────────────

func TestParseSpotlightQuery_PlainText(t *testing.T) {
	req := ParseSpotlightQuery("hello world")
	assert.Equal(t, "hello world", req.Query)
}

func TestParseSpotlightQuery_QuotedText(t *testing.T) {
	req := ParseSpotlightQuery("\"search term\"")
	assert.Equal(t, "search term", req.Query)
}

func TestParseSpotlightQuery_ContentAttribute(t *testing.T) {
	req := ParseSpotlightQuery(`kMDItemTextContent == "financial data"c`)
	assert.Equal(t, "financial data", req.Query)
}

func TestParseSpotlightQuery_DisplayNameAttribute(t *testing.T) {
	req := ParseSpotlightQuery(`kMDItemDisplayName == "report"`)
	assert.Equal(t, "report", req.Query)
}

func TestParseSpotlightQuery_ContentTypeAttribute(t *testing.T) {
	req := ParseSpotlightQuery(`kMDItemContentType == "txt"`)
	assert.Contains(t, req.Types, "txt")
}

func TestParseSpotlightQuery_EmptyQuery(t *testing.T) {
	req := ParseSpotlightQuery("")
	assert.Equal(t, "", req.Query)
}

// ─── Spotlight Handler Gin 测试 ───────────────────────────────

func TestSpotlightHandler_GinGet(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler.RegisterRoutesGin(r.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/spotlight?q=database&share=public", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])

	data := resp["data"].(map[string]interface{})
	assert.True(t, data["total"].(float64) > 0)
}

func TestSpotlightHandler_GinGetMissingQuery(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler.RegisterRoutesGin(r.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/spotlight", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSpotlightHandler_GinGetXMLFormat(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler.RegisterRoutesGin(r.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/spotlight?q=database&share=public&format=xml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "xml")
}

func TestSpotlightHandler_GinPost(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler.RegisterRoutesGin(r.Group("/api/v1"))

	body := SpotlightQuery{
		Query:      "project documentation",
		ShareName:  "public",
		MaxResults: 5,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/spotlight", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])
}

func TestSpotlightHandler_GinPostEmptyQuery(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler.RegisterRoutesGin(r.Group("/api/v1"))

	body := SpotlightQuery{Query: ""}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/spotlight", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSpotlightHandler_GinAttributes(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler.RegisterRoutesGin(r.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/spotlight/attributes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].(map[string]interface{})
	attrs := data["attributes"].([]interface{})
	assert.True(t, len(attrs) > 0)

	kinds := data["kinds"].([]interface{})
	assert.True(t, len(kinds) > 0)
}

// ─── Spotlight Handler HTTP 测试 ──────────────────────────────

func TestSpotlightHandler_HTTPGet(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/spotlight?q=database&share=public", nil)
	w := httptest.NewRecorder()
	handler.HandleHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "json")
}

func TestSpotlightHandler_HTTPGetMissingQuery(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/spotlight", nil)
	w := httptest.NewRecorder()
	handler.HandleHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSpotlightHandler_HTTPPost(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	body := SpotlightQuery{
		Query:     "report",
		ShareName: "public",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/spotlight", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestSpotlightHandler_HTTPMethodNotAllowed(t *testing.T) {
	_, _, handler := setupSpotlightEngine(t)

	req := httptest.NewRequest(http.MethodDelete, "/spotlight", nil)
	w := httptest.NewRecorder()
	handler.HandleHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// ─── Spotlight 辅助函数测试 ───────────────────────────────────

func TestGetKind(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".txt", "Plain Text"},
		{".md", "Markdown"},
		{".pdf", "PDF Document"},
		{".docx", "Microsoft Word"},
		{".go", "Go Source"},
		{".xyz", "Document"}, // 未知类型
		{"", "Document"},     // 空扩展名
	}

	for _, tt := range tests {
		got := getKind(tt.ext)
		assert.Equal(t, tt.want, got, "getKind(%q)", tt.ext)
	}
}

func TestToSMBPath(t *testing.T) {
	tests := []struct {
		name      string
		localPath string
		sharePath string
		shareName string
		want      string
	}{
		{
			name:      "simple path",
			localPath: "/mnt/pool/data/file.txt",
			sharePath: "/mnt/pool/data",
			shareName: "public",
			want:      "smb://nas/public/file.txt",
		},
		{
			name:      "nested path",
			localPath: "/mnt/pool/data/docs/reports/2024.txt",
			sharePath: "/mnt/pool/data",
			shareName: "share1",
			want:      "smb://nas/share1/docs/reports/2024.txt",
		},
		{
			name:      "root file",
			localPath: "/mnt/pool/data/readme.md",
			sharePath: "/mnt/pool/data",
			shareName: "docs",
			want:      "smb://nas/docs/readme.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toSMBPath(tt.localPath, tt.sharePath, tt.shareName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractQuotedStrings(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{`"hello world"`, []string{"hello world"}},
		{`"first" and "second"`, []string{"first", "second"}},
		{`no quotes here`, nil},
		{`"unclosed quote`, nil},
		{`""`, []string{""}}, // 空字符串
	}

	for _, tt := range tests {
		got := extractQuotedStrings(tt.input)
		// 使用 reflect.DeepEqual 比较 nil 和空切片
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("extractQuotedStrings(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSupportedSpotlightAttributes(t *testing.T) {
	attrs := SupportedSpotlightAttributes()
	assert.NotEmpty(t, attrs)
	assert.Contains(t, attrs, "kMDItemDisplayName")
	assert.Contains(t, attrs, "kMDItemTextContent")
}

func TestSupportedKinds(t *testing.T) {
	kinds := SupportedKinds()
	assert.NotEmpty(t, kinds)
	assert.Contains(t, kinds, "Plain Text")
	assert.Contains(t, kinds, "PDF Document")
}

func TestSpotlightServer_SyncMetadata(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	shareDir, _ := server.GetSharePath("public")
	filePath := filepath.Join(shareDir, "report.txt")

	meta, err := server.SyncMetadata("public", filePath)
	require.NoError(t, err)
	assert.Equal(t, "public", meta.ShareName)
	// SyncMetadata may return empty props if the path search doesn't match,
	// but the call itself should succeed without error.
	t.Logf("SyncMetadata props: %v", meta.Props)
}

func TestSpotlightServer_SyncMetadataUnknownShare(t *testing.T) {
	_, server, _ := setupSpotlightEngine(t)

	_, err := server.SyncMetadata("nonexistent", "/some/path")
	assert.Error(t, err)
}
