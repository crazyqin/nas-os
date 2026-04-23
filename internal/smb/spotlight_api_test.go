// Package smb SMB Spotlight API 单元测试
// 测试 Spotlight 搜索功能的 macOS 集成
package smb

import (
	"context"
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

// ========== SpotlightSearchRequest 测试 ==========

func TestSpotlightSearchRequestDefaults(t *testing.T) {
	req := &SpotlightSearchRequest{}

	// 验证默认值（由API handler设置）
	assert.Equal(t, 0, req.Limit) // 默认会在handler中设置为100
	assert.Empty(t, req.Query)
	assert.Empty(t, req.Scope)
}

func TestSpotlightSearchRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     SpotlightSearchRequest
		wantErr bool
	}{
		{
			name: "empty query",
			req:  SpotlightSearchRequest{Query: ""},
			wantErr: false, // 允许空查询
		},
		{
			name: "simple keyword",
			req:  SpotlightSearchRequest{Query: "testfile"},
			wantErr: false,
		},
		{
			name: "spotlight syntax",
			req:  SpotlightSearchRequest{Query: "kMDItemDisplayName == 'document.pdf'"},
			wantErr: false,
		},
		{
			name: "complex query",
			req:  SpotlightSearchRequest{
				Query:    "test",
				Scope:    []string{"/data/share"},
				Limit:    50,
				OnlyFiles: true,
				FuzzyMatch: true,
			},
			wantErr: false,
		},
		{
			name: "limit too large",
			req:  SpotlightSearchRequest{Query: "test", Limit: 5000},
			wantErr: false, // handler会限制到1000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 基本验证逻辑
			if tt.wantErr {
				assert.NotNil(t, tt.req)
			}
		})
	}
}

// ========== SpotlightFileResult 测试 ==========

func TestSpotlightFileResultFields(t *testing.T) {
	now := time.Now()
	result := &SpotlightFileResult{
		Path:        "/data/share/document.pdf",
		RelativePath: "document.pdf",
		Name:        "document.pdf",
		Size:        1024000,
		ModTime:     now,
		Type:        "com.adobe.pdf",
		Kind:        "PDF文档",
		Extension:   ".pdf",
		IsDirectory: false,
		Score:       85.5,
		Attributes: map[string]string{
			"kMDItemDisplayName": "document.pdf",
			"kMDItemFSSize":      "1024000",
		},
	}

	assert.Equal(t, "/data/share/document.pdf", result.Path)
	assert.Equal(t, "document.pdf", result.Name)
	assert.Equal(t, int64(1024000), result.Size)
	assert.Equal(t, "com.adobe.pdf", result.Type)
	assert.Equal(t, ".pdf", result.Extension)
	assert.False(t, result.IsDirectory)
	assert.Equal(t, 85.5, result.Score)
	assert.Contains(t, result.Attributes, "kMDItemDisplayName")
}

// ========== filepathExt 辅助函数测试 ==========

func TestFilepathExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/data/share/document.pdf", ".pdf"},
		{"/data/share/image.jpg", ".jpg"},
		{"/data/share/archive.tar.gz", ".gz"},
		{"noextension", ""},
		{"", ""},
		{"hidden/.gitignore", ".gitignore"},
		{"multiple.dots.in.name.txt", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ext := filepathExt(tt.path)
			assert.Equal(t, tt.want, ext)
		})
	}
}

// ========== SpotlightIndexStatus 测试 ==========

func TestSpotlightIndexStatusFields(t *testing.T) {
	status := &SpotlightIndexStatus{
		Enabled:      true,
		Status:       "ready",
		TotalFiles:   10000,
		IndexedFiles: 8500,
		IndexedSize:  524288000,
		Progress:     85.0,
		LastUpdate:   time.Now(),
		SharePaths:   []string{"/data/share1", "/data/share2"},
		ContentIndexed: true,
	}

	assert.True(t, status.Enabled)
	assert.Equal(t, "ready", status.Status)
	assert.Equal(t, int64(10000), status.TotalFiles)
	assert.Equal(t, int64(8500), status.IndexedFiles)
	assert.GreaterOrEqual(t, status.Progress, 0.0)
	assert.LessOrEqual(t, status.Progress, 100.0)
	assert.Len(t, status.SharePaths, 2)
}

// ========== ShareSpotlightConfig 测试 ==========

func TestShareSpotlightConfigDefaults(t *testing.T) {
	config := &ShareSpotlightConfig{
		ShareName:      "testshare",
		Enabled:        true,
		IndexPath:      "/data/testshare",
		ContentIndex:   true,
		ChineseSegment: true,
		MaxIndexSizeMB: 500,
		UpdateInterval: 300,
		CacheSize:      100,
	}

	assert.Equal(t, "testshare", config.ShareName)
	assert.True(t, config.Enabled)
	assert.Equal(t, "/data/testshare", config.IndexPath)
	assert.True(t, config.ContentIndex)
	assert.True(t, config.ChineseSegment)
}

// ========== SpotlightConfig 测试 ==========

func TestSpotlightConfigDefaults(t *testing.T) {
	config := SpotlightConfig{
		Enabled:          true,
		SharePaths:       []string{"/data/share"},
		MaxIndexSize:     500,
		UpdateInterval:   300,
		EnableContentIdx: true,
		EnableChineseSeg: true,
		IndexerWorkers:   4,
		CacheSize:        100,
	}

	assert.True(t, config.Enabled)
	assert.Len(t, config.SharePaths, 1)
	assert.Equal(t, int64(500), config.MaxIndexSize)
	assert.Equal(t, 300, config.UpdateInterval)
}

// ========== Indexer 测试 ==========

func TestNewIndexer(t *testing.T) {
	config := SpotlightConfig{
		CacheSize:        50,
		CacheTTLSeconds:  60,
		MaxConcurrentSearch: 4,
		IndexBatchSize:   500,
	}

	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	assert.NotNil(t, indexer)
	assert.NotNil(t, indexer.fileIndex)
	assert.NotNil(t, indexer.contentIdx)
	assert.NotNil(t, indexer.wordIndex)
	assert.NotNil(t, indexer.searchCache)
	assert.NotNil(t, indexer.searchSemaphore)
}

func TestIndexerContentType(t *testing.T) {
	config := SpotlightConfig{}
	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	tests := []struct {
		ext  string
		want string
	}{
		{".pdf", "com.adobe.pdf"},
		{".txt", "public.plain-text"},
		{".jpg", "public.jpeg"},
		{".png", "public.png"},
		{".mp4", "public.mpeg-4"},
		{".mp3", "public.mp3"},
		{".zip", "public.zip-archive"},
		{".unknown", "public.item"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			contentType := indexer.getContentType(tt.ext)
			assert.Equal(t, tt.want, contentType)
		})
	}
}

func TestIndexerGetKind(t *testing.T) {
	config := SpotlightConfig{}
	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	tests := []struct {
		ext  string
		want string
	}{
		{".pdf", "PDF文档"},
		{".txt", "文本"},
		{".jpg", "JPEG图像"},
		{".png", "PNG图像"},
		{".mp4", "MP4视频"},
		{".mp3", "MP3音频"},
		{".go", "Go源代码"},
		{".py", "Python源代码"},
		{".js", "JavaScript源代码"},
		{".unknown", "文件"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			kind := indexer.getKind(tt.ext)
			assert.Equal(t, tt.want, kind)
		})
	}
}

func TestIndexerIsTextFile(t *testing.T) {
	config := SpotlightConfig{}
	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	textFiles := []string{".txt", ".md", ".json", ".yaml", ".go", ".py", ".js", ".html", ".css", ".xml"}
	for _, ext := range textFiles {
		assert.True(t, indexer.isTextFile(ext), ext+" should be text file")
	}

	nonTextFiles := []string{".jpg", ".png", ".pdf", ".mp4", ".zip"}
	for _, ext := range nonTextFiles {
		assert.False(t, indexer.isTextFile(ext), ext+" should not be text file")
	}
}

// ========== SearchCache 测试 ==========

func TestNewSearchCache(t *testing.T) {
	cache := NewSearchCache(10, 30*time.Second)

	assert.NotNil(t, cache)
	assert.Equal(t, 10, cache.maxSize)
	assert.Equal(t, 30*time.Second, cache.ttl)
}

// ========== MDQueryHandler 测试 ==========

func TestMDQueryHandlerParseQuery(t *testing.T) {
	logger := zap.NewNop()
	handler := NewMDQueryHandler(logger)

	tests := []struct {
		query    string
		expected map[string]interface{}
	}{
		{
			query: "testfile",
			expected: map[string]interface{}{"name": "testfile"},
		},
		{
			query: "kMDItemDisplayName == \"document.pdf\"",
			expected: map[string]interface{}{"name": "document.pdf"},
		},
		{
			query: "kMDItemContentType == \"public.image\"",
			expected: map[string]interface{}{"type": "public.image"},
		},
		{
			query: "kMDItemDisplayName == \"file\" OR kMDItemContentType == \"public.pdf\"",
			expected: map[string]interface{}{"name": "file", "type": "public.pdf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := handler.ParseQuery(tt.query)
			for key, val := range tt.expected {
				assert.Equal(t, val, result[key], "key: "+key)
			}
		})
	}
}

func TestMDQueryHandlerMapSpotlightAttr(t *testing.T) {
	logger := zap.NewNop()
	handler := NewMDQueryHandler(logger)

	tests := []struct {
		attr     string
		expected string
	}{
		{"kMDItemDisplayName", "name"},
		{"kMDItemFSName", "name"},
		{"kMDItemPath", "path"},
		{"kMDItemFSSize", "size"},
		{"kMDItemContentType", "type"},
		{"kMDItemKind", "kind"},
		{"unknownAttr", ""},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			result := handler.mapSpotlightAttr(tt.attr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMDQueryHandlerMapToSpotlightAttributes(t *testing.T) {
	logger := zap.NewNop()
	handler := NewMDQueryHandler(logger)

	internalAttrs := map[string]string{
		"name":     "test.pdf",
		"size":     "1024",
		"type":     "com.adobe.pdf",
		"kind":     "PDF文档",
		"modified": "2024-01-01T00:00:00Z",
	}

	result := handler.MapToSpotlightAttributes(internalAttrs)

	assert.Equal(t, "test.pdf", result["kMDItemDisplayName"])
	assert.Equal(t, "1024", result["kMDItemFSSize"])
	assert.Equal(t, "com.adobe.pdf", result["kMDItemContentType"])
}

// ========== SpotlightIntegration 测试 ==========

func TestNewSpotlightIntegration(t *testing.T) {
	config := SpotlightConfig{
		Enabled:        true,
		SharePaths:     []string{"/data/share"},
		IndexerWorkers: 4,
	}

	logger := zap.NewNop()
	si := NewSpotlightIntegration(config, logger)

	assert.NotNil(t, si)
	assert.NotNil(t, si.indexer)
	assert.NotNil(t, si.mdquery)
	assert.True(t, si.config.Enabled)
}

func TestSpotlightIntegrationGetIndexStatus(t *testing.T) {
	config := SpotlightConfig{
		Enabled:    true,
		SharePaths: []string{},
	}

	logger := zap.NewNop()
	si := NewSpotlightIntegration(config, logger)

	status := si.GetIndexStatus()

	assert.NotNil(t, status)
	assert.Equal(t, "", status.Status) // 初始状态为空
}

// ========== SpotlightService 测试 ==========

func TestNewSpotlightService(t *testing.T) {
	config := SpotlightConfig{Enabled: true}
	logger := zap.NewNop()
	integration := NewSpotlightIntegration(config, logger)

	service := NewSpotlightService(integration, nil) // manager为nil测试

	assert.NotNil(t, service)
	assert.NotNil(t, service.integration)
}

func TestSpotlightServiceGetIndexStatus(t *testing.T) {
	config := SpotlightConfig{
		Enabled:        true,
		SharePaths:     []string{"/data/share"},
		EnableContentIdx: true,
	}

	logger := zap.NewNop()
	integration := NewSpotlightIntegration(config, logger)
	service := NewSpotlightService(integration, nil)

	ctx := context.Background()
	status, err := service.GetIndexStatus(ctx)

	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.True(t, status.Enabled)
	assert.Equal(t, []string{"/data/share"}, status.SharePaths)
}

// ========== SpotlightAPIHandler 测试 ==========

func TestNewSpotlightAPIHandler(t *testing.T) {
	config := SpotlightConfig{Enabled: true}
	logger := zap.NewNop()
	integration := NewSpotlightIntegration(config, logger)
	service := NewSpotlightService(integration, nil)

	handler := NewSpotlightAPIHandler(service, logger)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.service)
}

func TestSpotlightAPIHandlerGetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := SpotlightConfig{
		Enabled:      true,
		SharePaths:   []string{"/data/share"},
		EnableContentIdx: true,
	}

	logger := zap.NewNop()
	integration := NewSpotlightIntegration(config, logger)
	service := NewSpotlightService(integration, nil)
	handler := NewSpotlightAPIHandler(service, logger)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api"))

	// 测试 GET /api/spotlight/status
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/spotlight/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "enabled")
}

func TestSpotlightAPIHandlerGetStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := SpotlightConfig{
		Enabled:        true,
		SharePaths:     []string{"/data/share"},
		EnableContentIdx: true,
		CacheSize:      100,
	}

	logger := zap.NewNop()
	integration := NewSpotlightIntegration(config, logger)
	service := NewSpotlightService(integration, nil)
	handler := NewSpotlightAPIHandler(service, logger)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api"))

	// 测试 GET /api/spotlight/stats
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/spotlight/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "performance")
}

func TestSpotlightAPIHandlerSearchGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := SpotlightConfig{Enabled: false} // 禁用状态下测试
	logger := zap.NewNop()
	integration := NewSpotlightIntegration(config, logger)
	service := NewSpotlightService(integration, nil)
	handler := NewSpotlightAPIHandler(service, logger)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api"))

	// 测试 GET /api/spotlight/search?q=test
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/spotlight/search?q=test&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// 禁用状态下返回空结果（不报错）
	assert.Contains(t, w.Body.String(), "total")
}

func TestSpotlightAPIHandlerGetConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := SpotlightConfig{
		Enabled:             true,
		SharePaths:          []string{"/data/share"},
		MaxIndexSize:        500,
		UpdateInterval:      300,
		EnableContentIdx:    true,
		EnableChineseSeg:    true,
		IndexerWorkers:      4,
		CacheSize:           100,
		CacheTTLSeconds:     300,
		MaxConcurrentSearch: 8,
		IndexBatchSize:      1000,
	}

	logger := zap.NewNop()
	integration := NewSpotlightIntegration(config, logger)
	service := NewSpotlightService(integration, nil)
	handler := NewSpotlightAPIHandler(service, logger)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api"))

	// 测试 GET /api/spotlight/config
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/spotlight/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "enabled")
	assert.Contains(t, w.Body.String(), "sharePaths")
}

// ========== GenerateSMBConfSpotlight 测试 ==========

func TestGenerateSMBConfSpotlight(t *testing.T) {
	// 启用状态
	enabled := GenerateSMBConfSpotlight(true, []string{"/data/share"}, nil)
	assert.Contains(t, enabled, "spotlight = yes")
	assert.Contains(t, enabled, "spotlight indexing = yes")

	// 禁用状态
	disabled := GenerateSMBConfSpotlight(false, nil, nil)
	assert.Empty(t, disabled)

	// 包含排除路径
	withExcluded := GenerateSMBConfSpotlight(true, 
		[]string{"/data/share"}, 
		[]string{"/data/share/tmp", "/data/share/cache"})
	assert.Contains(t, withExcluded, "spotlight exclude paths")
}

// ========== GenerateSMBSpotlightConfig 测试 ==========

func TestGenerateSMBSpotlightConfig(t *testing.T) {
	// 启用状态
	enabled := GenerateSMBSpotlightConfig(true, []string{"/data/share1", "/data/share2"})
	assert.Contains(t, enabled, "spotlight = yes")
	assert.Contains(t, enabled, "/data/share1")
	assert.Contains(t, enabled, "/data/share2")

	// 禁用状态
	disabled := GenerateSMBSpotlightConfig(false, nil)
	assert.Empty(t, disabled)
}

// ========== 辅助函数测试 ==========

func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	assert.True(t, contains(slice, "apple"))
	assert.True(t, contains(slice, "banana"))
	assert.False(t, contains(slice, "orange"))
	assert.False(t, contains(slice, ""))
}

func TestRemoveFromSlice(t *testing.T) {
	slice := []string{"a", "b", "c", "d"}

	result := removeFromSlice(slice, "b")
	assert.Equal(t, []string{"a", "c", "d"}, result)

	result2 := removeFromSlice(slice, "x")
	assert.Equal(t, slice, result2) // 不存在时不改变
}

func TestIndexerDetectLanguage(t *testing.T) {
	config := SpotlightConfig{}
	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	// 中文文本
	chineseText := "这是一个中文测试文本，用于检测语言类型。"
	assert.Equal(t, "zh", indexer.detectLanguage(chineseText))

	// 英文文本
	englishText := "This is an English test text for language detection."
	assert.Equal(t, "en", indexer.detectLanguage(englishText))

	// 混合文本
	mixedText := "Hello world 你好世界"
	assert.Equal(t, "zh", indexer.detectLanguage(mixedText))
}

func TestIndexerExtractWords(t *testing.T) {
	config := SpotlightConfig{}
	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	text := "Hello, World! This is a test."
	words := indexer.extractWords(text)

	assert.GreaterOrEqual(t, len(words), 2)
	assert.Contains(t, words, "hello")
	assert.Contains(t, words, "world")
	assert.Contains(t, words, "test")
}

func TestIndexerMakeExcerpt(t *testing.T) {
	config := SpotlightConfig{}
	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	shortText := "Short text"
	assert.Equal(t, "Short text", indexer.makeExcerpt(shortText, 200))

	longText := "This is a very long text that should be truncated when creating an excerpt for display purposes."
	excerpt := indexer.makeExcerpt(longText, 50)
	assert.LessOrEqual(t, len(excerpt), 53) // 50 + "..."
	assert.True(t, strings.HasSuffix(excerpt, "..."))
}

func TestIndexerExtractKeywords(t *testing.T) {
	config := SpotlightConfig{}
	logger := zap.NewNop()
	indexer := NewIndexer(config, logger)

	words := []string{"test", "test", "document", "file", "file", "file", "system"}
	keywords := indexer.extractKeywords(words)

	// "file"出现3次，应该是关键词
	assert.Contains(t, keywords, "file")
	// "test"出现2次，可能也是关键词
	assert.Contains(t, keywords, "test")
}

// ========== SpotlightQuery 测试 ==========

func TestSpotlightQuery(t *testing.T) {
	query := SpotlightQuery{
		Query:        "kMDItemDisplayName == 'test.pdf'",
		Attributes:   []string{"kMDItemDisplayName", "kMDItemFSSize"},
		Scope:        []string{"/data/share"},
		Limit:        50,
		SortBy:       "score",
		SortDesc:     true,
		OnlyFiles:    true,
		FuzzyMatch:   true,
		ContentSearch: false,
	}

	assert.Equal(t, "kMDItemDisplayName == 'test.pdf'", query.Query)
	assert.Len(t, query.Attributes, 2)
	assert.Len(t, query.Scope, 1)
	assert.Equal(t, 50, query.Limit)
	assert.True(t, query.SortDesc)
	assert.True(t, query.OnlyFiles)
}

// ========== SpotlightResult 测试 ==========

func TestSpotlightResult(t *testing.T) {
	result := SpotlightResult{
		Path:    "/data/share/file.pdf",
		Name:    "file.pdf",
		Size:    102400,
		ModTime: time.Now(),
		Type:    "com.adobe.pdf",
		Kind:    "PDF文档",
		Score:   90.5,
		Snippet: "This is a PDF document...",
		Attributes: map[string]string{
			"kMDItemDisplayName": "file.pdf",
		},
	}

	assert.Equal(t, "/data/share/file.pdf", result.Path)
	assert.Equal(t, "file.pdf", result.Name)
	assert.Equal(t, int64(102400), result.Size)
	assert.Equal(t, 90.5, result.Score)
	assert.NotEmpty(t, result.Snippet)
}

// ========== SpotlightResponse 测试 ==========

func TestSpotlightResponse(t *testing.T) {
	response := SpotlightResponse{
		Query:   "test",
		Results: []SpotlightResult{},
		Total:   0,
		Took:    15,
		Scope:   []string{"/data/share"},
	}

	assert.Equal(t, "test", response.Query)
	assert.Equal(t, 0, response.Total)
	assert.Equal(t, int64(15), response.Took)
}

// ========== IndexStats 测试 ==========

func TestIndexStats(t *testing.T) {
	stats := IndexStats{
		TotalFiles:   10000,
		IndexedFiles: 8500,
		IndexedSize:  524288000,
		Status:       "ready",
		Progress:     85.0,
		LastUpdate:   time.Now(),
	}

	assert.Equal(t, int64(10000), stats.TotalFiles)
	assert.Equal(t, int64(8500), stats.IndexedFiles)
	assert.Equal(t, "ready", stats.Status)
	assert.Equal(t, 85.0, stats.Progress)
}

// ========== SpotlightMDQueryRequest/Response 测试 ==========

func TestSpotlightMDQueryRequest(t *testing.T) {
	req := SpotlightMDQueryRequest{
		Query:     "kMDItemDisplayName == '*.pdf'",
		OnlyIn:    "/data/share",
		Live:      false,
		Interpret: true,
		Attributes: []string{"kMDItemDisplayName", "kMDItemFSSize"},
	}

	assert.Equal(t, "kMDItemDisplayName == '*.pdf'", req.Query)
	assert.Equal(t, "/data/share", req.OnlyIn)
	assert.True(t, req.Interpret)
}

func TestSpotlightMDQueryResponse(t *testing.T) {
	resp := SpotlightMDQueryResponse{
		Results: []string{"file1.pdf", "file2.pdf"},
		Count:   2,
		QueryID: "query-123",
	}

	assert.Len(t, resp.Results, 2)
	assert.Equal(t, 2, resp.Count)
	assert.Equal(t, "query-123", resp.QueryID)
}

// ========== SpotlightGlobalConfig 测试 ==========

func TestSpotlightGlobalConfig(t *testing.T) {
	config := SpotlightGlobalConfig{
		Enabled:             true,
		SharePaths:          []string{"/data/share"},
		MaxIndexSizeMB:      500,
		UpdateInterval:      300,
		EnableContentIdx:    true,
		EnableChineseSeg:    true,
		IndexerWorkers:      4,
		CacheSize:           100,
		CacheTTLSeconds:     300,
		MaxConcurrentSearch: 8,
		IndexBatchSize:      1000,
		EnableResultCache:   true,
		EnableParallelIndex: true,
		FuzzyThreshold:      0.7,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, int64(500), config.MaxIndexSizeMB)
	assert.Equal(t, 0.7, config.FuzzyThreshold)
}

// ========== SpotlightShareInfo 测试 ==========

func TestSpotlightShareInfo(t *testing.T) {
	info := SpotlightShareInfo{
		ShareName:    "documents",
		Path:         "/data/documents",
		Enabled:      true,
		IndexedFiles: 5000,
		IndexedSize:  256000000,
		Status:       "ready",
		LastIndexed:  time.Now(),
	}

	assert.Equal(t, "documents", info.ShareName)
	assert.True(t, info.Enabled)
	assert.Equal(t, int64(5000), info.IndexedFiles)
}