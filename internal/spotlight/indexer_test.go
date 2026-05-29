package spotlight

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.index)
	assert.NotNil(t, mgr.tokenizer)
	assert.NotNil(t, mgr.config)
}

func TestIndexDocument(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	doc := &Document{
		ID:        "test-1",
		Path:      "/test/file.txt",
		Name:      "file.txt",
		Extension: ".txt",
		Size:      1024,
		FileType:  FileTypeDocument,
		Content:   "This is a test document with some content",
		Tags:      []string{"test", "document"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := mgr.IndexDocument(doc)
	require.NoError(t, err)

	// 验证文档已索引
	retrieved, exists := mgr.GetDocument("test-1")
	assert.True(t, exists)
	assert.Equal(t, "test-1", retrieved.ID)
	assert.Equal(t, "/test/file.txt", retrieved.Path)
}

func TestSearch(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	// 索引测试文档
	docs := []*Document{
		{
			ID:        "doc-1",
			Path:      "/docs/readme.md",
			Name:      "readme.md",
			Extension: ".md",
			Size:      2048,
			FileType:  FileTypeDocument,
			Content:   "Welcome to our project. This is the main readme file.",
			Tags:      []string{"readme", "project"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc-2",
			Path:      "/docs/guide.pdf",
			Name:      "guide.pdf",
			Extension: ".pdf",
			Size:      10240,
			FileType:  FileTypeDocument,
			Content:   "This guide explains how to use the project features.",
			Tags:      []string{"guide", "tutorial"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "img-1",
			Path:      "/images/logo.png",
			Name:      "logo.png",
			Extension: ".png",
			Size:      51200,
			FileType:  FileTypeImage,
			Tags:      []string{"logo", "brand"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, doc := range docs {
		err := mgr.IndexDocument(doc)
		require.NoError(t, err)
	}

	// 测试基本搜索
	t.Run("BasicSearch", func(t *testing.T) {
		req := &SearchRequest{
			Query:    "readme",
			Page:     1,
			PageSize: 10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.Total, 0)
		assert.Contains(t, result.Documents[0].Name, "readme")
	})

	// 测试内容搜索
	t.Run("ContentSearch", func(t *testing.T) {
		req := &SearchRequest{
			Query:    "project",
			Page:     1,
			PageSize: 10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)
		assert.Greater(t, result.Total, 0)
	})

	// 测试文件类型过滤
	t.Run("FileTypeFilter", func(t *testing.T) {
		req := &SearchRequest{
			Query:    "project",
			FileType: FileTypeDocument,
			Page:     1,
			PageSize: 10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)

		for _, doc := range result.Documents {
			assert.Equal(t, FileTypeDocument, doc.FileType)
		}
	})

	// 测试扩展名过滤
	t.Run("ExtensionFilter", func(t *testing.T) {
		req := &SearchRequest{
			Query:      "project",
			Extensions: []string{"md"},
			Page:       1,
			PageSize:   10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)

		for _, doc := range result.Documents {
			assert.Equal(t, ".md", doc.Extension)
		}
	})

	// 测试标签过滤
	t.Run("TagFilter", func(t *testing.T) {
		req := &SearchRequest{
			Query:    "project",
			Tags:     []string{"readme"},
			Page:     1,
			PageSize: 10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)

		for _, doc := range result.Documents {
			assert.Contains(t, doc.Tags, "readme")
		}
	})

	// 测试空查询
	t.Run("EmptyQuery", func(t *testing.T) {
		req := &SearchRequest{
			Query:    "",
			Page:     1,
			PageSize: 10,
		}

		_, err := mgr.Search(req)
		assert.Error(t, err)
	})

	// 测试无结果
	t.Run("NoResults", func(t *testing.T) {
		req := &SearchRequest{
			Query:    "nonexistent",
			Page:     1,
			PageSize: 10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)
		assert.Equal(t, 0, result.Total)
	})
}

func TestSuggest(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	// 索引测试文档
	docs := []*Document{
		{
			ID:       "doc-1",
			Path:     "/test/readme.md",
			Name:     "readme.md",
			Content:  "readme documentation",
			Tags:     []string{"readme"},
			FileType: FileTypeDocument,
		},
		{
			ID:       "doc-2",
			Path:     "/test/report.pdf",
			Name:     "report.pdf",
			Content:  "annual report",
			Tags:     []string{"report"},
			FileType: FileTypeDocument,
		},
		{
			ID:       "doc-3",
			Path:     "/test/resource.txt",
			Name:     "resource.txt",
			Content:  "resource management",
			Tags:     []string{"resource"},
			FileType: FileTypeDocument,
		},
	}

	for _, doc := range docs {
		err := mgr.IndexDocument(doc)
		require.NoError(t, err)
	}

	// 测试前缀搜索
	t.Run("PrefixSearch", func(t *testing.T) {
		req := &SuggestRequest{
			Query: "re",
			Limit: 10,
		}

		result, err := mgr.Suggest(req)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Suggestions)
	})

	// 测试空查询
	t.Run("EmptyQuery", func(t *testing.T) {
		req := &SuggestRequest{
			Query: "",
			Limit: 10,
		}

		_, err := mgr.Suggest(req)
		assert.Error(t, err)
	})
}

func TestRemoveDocument(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	doc := &Document{
		ID:        "test-remove",
		Path:      "/test/remove.txt",
		Name:      "remove.txt",
		Content:   "test content",
		FileType:  FileTypeDocument,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := mgr.IndexDocument(doc)
	require.NoError(t, err)

	// 确认文档存在
	_, exists := mgr.GetDocument("test-remove")
	assert.True(t, exists)

	// 删除文档
	err = mgr.RemoveDocument("test-remove")
	require.NoError(t, err)

	// 确认文档已删除
	_, exists = mgr.GetDocument("test-remove")
	assert.False(t, exists)

	// 搜索应该找不到
	req := &SearchRequest{
		Query:    "test",
		Page:     1,
		PageSize: 10,
	}

	result, err := mgr.Search(req)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
}

func TestBatchIndex(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	docs := []*Document{
		{
			ID:       "batch-1",
			Path:     "/test/batch1.txt",
			Name:     "batch1.txt",
			Content:  "first batch document",
			FileType: FileTypeDocument,
		},
		{
			ID:       "batch-2",
			Path:     "/test/batch2.txt",
			Name:     "batch2.txt",
			Content:  "second batch document",
			FileType: FileTypeDocument,
		},
	}

	indexed, err := mgr.IndexDocuments(docs)
	require.NoError(t, err)
	assert.Equal(t, 2, indexed)

	// 搜索验证
	req := &SearchRequest{
		Query:    "batch",
		Page:     1,
		PageSize: 10,
	}

	result, err := mgr.Search(req)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
}

func TestGetStats(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	doc := &Document{
		ID:       "stats-test",
		Path:     "/test/stats.txt",
		Name:     "stats.txt",
		Size:     1024,
		Content:  "stats test content",
		FileType: FileTypeDocument,
	}

	err := mgr.IndexDocument(doc)
	require.NoError(t, err)

	stats := mgr.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.TotalDocuments)
	assert.Equal(t, int64(1024), stats.IndexSize)
	assert.Equal(t, 1, stats.DocumentsByType[FileTypeDocument])
}

func TestSearchWithPagination(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	// 创建多个文档
	for i := 0; i < 25; i++ {
		doc := &Document{
			ID:       "page-" + string(rune('A'+i%26)),
			Path:     "/test/page" + string(rune('A'+i%26)) + ".txt",
			Name:     "page" + string(rune('A'+i%26)) + ".txt",
			Content:  "pagination test content",
			FileType: FileTypeDocument,
		}
		err := mgr.IndexDocument(doc)
		require.NoError(t, err)
	}

	// 测试分页
	req := &SearchRequest{
		Query:    "pagination",
		Page:     1,
		PageSize: 10,
	}

	result, err := mgr.Search(req)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Documents), 10)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.PageSize)
}

func TestSearchSorting(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	docs := []*Document{
		{
			ID:        "sort-1",
			Path:      "/test/a.txt",
			Name:      "a.txt",
			Size:      100,
			Content:   "sorting test",
			FileType:  FileTypeDocument,
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:        "sort-2",
			Path:      "/test/b.txt",
			Name:      "b.txt",
			Size:      200,
			Content:   "sorting test",
			FileType:  FileTypeDocument,
			UpdatedAt: time.Now(),
		},
	}

	for _, doc := range docs {
		err := mgr.IndexDocument(doc)
		require.NoError(t, err)
	}

	// 测试按日期排序
	t.Run("SortByDate", func(t *testing.T) {
		req := &SearchRequest{
			Query:     "sorting",
			SortBy:    "date",
			SortOrder: "desc",
			Page:      1,
			PageSize:  10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)
		assert.Equal(t, 2, len(result.Documents))
		assert.True(t, result.Documents[0].UpdatedAt.After(result.Documents[1].UpdatedAt))
	})

	// 测试按大小排序
	t.Run("SortBySize", func(t *testing.T) {
		req := &SearchRequest{
			Query:     "sorting",
			SortBy:    "size",
			SortOrder: "desc",
			Page:      1,
			PageSize:  10,
		}

		result, err := mgr.Search(req)
		require.NoError(t, err)
		assert.Equal(t, 2, len(result.Documents))
		assert.True(t, result.Documents[0].Size > result.Documents[1].Size)
	})
}

func TestClassifyFileType(t *testing.T) {
	tests := []struct {
		ext      string
		expected FileType
	}{
		{"pdf", FileTypeDocument},
		{"doc", FileTypeDocument},
		{"jpg", FileTypeImage},
		{"png", FileTypeImage},
		{"mp4", FileTypeVideo},
		{"avi", FileTypeVideo},
		{"mp3", FileTypeAudio},
		{"wav", FileTypeAudio},
		{"zip", FileTypeArchive},
		{"tar", FileTypeArchive},
		{"go", FileTypeCode},
		{"py", FileTypeCode},
		{"xyz", FileTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := classifyFileType(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			result := editDistance(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCJKTokenization(t *testing.T) {
	// Test pure CJK tokenization
	result := tokenizeCJK("你好世界")
	assert.ElementsMatch(t, []string{"你", "好", "世", "界", "你好", "好世", "世界"}, result)

	// Test mixed language via the tokenizer
	tok := newTokenizer()
	result = tok.tokenize("hello你好")
	// After language-aware splitting: "hello" → tokenizeEnglish → ["hello"], "你好" → tokenizeCJK → ["你","好","你好"]
	// Note: single CJK chars "你" and "好" are stop words, filtered out
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "你好")
}

func TestTrieSearch(t *testing.T) {
	root := newTrieNode()

	root.insert("test")
	root.insert("testing")
	root.insert("tester")
	root.insert("other")

	suggestions := root.search("test", 10)
	assert.NotEmpty(t, suggestions)

	for _, s := range suggestions {
		assert.True(t, len(s.Text) >= 4)
		assert.Equal(t, "completion", s.Type)
	}
}

func TestSearchByPath(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	docs := []*Document{
		{
			ID:       "path-1",
			Path:     "/documents/file1.txt",
			Name:     "file1.txt",
			FileType: FileTypeDocument,
		},
		{
			ID:       "path-2",
			Path:     "/documents/file2.txt",
			Name:     "file2.txt",
			FileType: FileTypeDocument,
		},
		{
			ID:       "path-3",
			Path:     "/images/photo.jpg",
			Name:     "photo.jpg",
			FileType: FileTypeImage,
		},
	}

	for _, doc := range docs {
		err := mgr.IndexDocument(doc)
		require.NoError(t, err)
	}

	results := mgr.SearchByPath("/documents", 10)
	assert.Equal(t, 2, len(results))

	for _, doc := range results {
		assert.Contains(t, doc.Path, "/documents")
	}
}

func TestSearchByTags(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	docs := []*Document{
		{
			ID:       "tag-1",
			Path:     "/test/tag1.txt",
			Name:     "tag1.txt",
			Tags:     []string{"important", "work"},
			FileType: FileTypeDocument,
		},
		{
			ID:       "tag-2",
			Path:     "/test/tag2.txt",
			Name:     "tag2.txt",
			Tags:     []string{"important", "personal"},
			FileType: FileTypeDocument,
		},
		{
			ID:       "tag-3",
			Path:     "/test/tag3.txt",
			Name:     "tag3.txt",
			Tags:     []string{"other"},
			FileType: FileTypeDocument,
		},
	}

	for _, doc := range docs {
		err := mgr.IndexDocument(doc)
		require.NoError(t, err)
	}

	results := mgr.SearchByTags([]string{"important"}, 10)
	assert.Equal(t, 2, len(results))

	for _, doc := range results {
		assert.Contains(t, doc.Tags, "important")
	}
}

func TestRebuildIndex(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	doc := &Document{
		ID:       "rebuild-test",
		Path:     "/test/rebuild.txt",
		Name:     "rebuild.txt",
		Content:  "rebuild test content",
		FileType: FileTypeDocument,
	}

	err := mgr.IndexDocument(doc)
	require.NoError(t, err)

	// 重建索引
	err = mgr.RebuildIndex()
	require.NoError(t, err)

	// 搜索应该仍然有效
	req := &SearchRequest{
		Query:    "rebuild",
		Page:     1,
		PageSize: 10,
	}

	result, err := mgr.Search(req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
}

func TestGetPopularSearches(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)
	defer mgr.Close()

	docs := []*Document{
		{
			ID:       "popular-1",
			Path:     "/test/popular1.txt",
			Name:     "popular1.txt",
			Content:  "popular term test",
			FileType: FileTypeDocument,
		},
		{
			ID:       "popular-2",
			Path:     "/test/popular2.txt",
			Name:     "popular2.txt",
			Content:  "popular term another",
			FileType: FileTypeDocument,
		},
	}

	for _, doc := range docs {
		err := mgr.IndexDocument(doc)
		require.NoError(t, err)
	}

	popular := mgr.GetPopularSearches(10)
	assert.NotNil(t, popular)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 1000000, config.MaxIndexSize)
	assert.Equal(t, 2, config.MinTermLength)
	assert.Equal(t, 100, config.MaxTermLength)
	assert.True(t, config.EnableCJK)
	assert.True(t, config.EnableStemming)
	assert.Equal(t, 10, config.SuggestionLimit)
}
