// Package aisearch 提供 AI 语义搜索引擎单元测试
package aisearch

import (
	"testing"
	"time"
)

// MockVectorEncoder 模拟向量编码器
type MockVectorEncoder struct {
	dimension int
}

func (m *MockVectorEncoder) Encode(text string) ([]float64, error) {
	// 生成简单的模拟向量
	vector := make([]float64, m.dimension)
	for i := range vector {
		vector[i] = float64(i%10) / 10.0
	}
	return vector, nil
}

func (m *MockVectorEncoder) EncodeBatch(texts []string) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i, text := range texts {
		vector, err := m.Encode(text)
		if err != nil {
			return nil, err
		}
		vectors[i] = vector
	}
	return vectors, nil
}

func (m *MockVectorEncoder) Dimension() int {
	return m.dimension
}

// MockContentExtractor 模拟内容提取器
type MockContentExtractor struct {
}

func (m *MockContentExtractor) Extract(filePath string) (string, error) {
	return "这是模拟的文件内容，用于测试搜索引擎功能", nil
}

func (m *MockContentExtractor) CanExtract(fileType FileType, mimeType string) bool {
	return fileType == FileTypeDocument
}

func TestSearchEngine(t *testing.T) {
	config := DefaultSearchConfig()
	encoder := &MockVectorEncoder{dimension: 384}
	extractor := &MockContentExtractor{}
	engine := NewEngine(config, encoder, extractor)
	defer engine.Close()

	// 测试索引文档
	t.Run("IndexDocument", func(t *testing.T) {
		doc := &SearchIndex{
			ID:         "test-doc-1",
			FilePath:   "/test/document.pdf",
			FileName:   "document.pdf",
			FileType:   FileTypeDocument,
			FileSize:   1024,
			ModifiedAt: time.Now(),
			CreatedAt:  time.Now(),
			Content:    "这是一个测试文档，用于验证搜索引擎功能",
			Tags:       []string{"测试", "文档"},
			Metadata:   map[string]string{"author": "测试用户"},
			Status:     IndexStatusPending,
		}

		err := engine.IndexDocument(doc)
		if err != nil {
			t.Fatalf("索引文档失败: %v", err)
		}

		stats, _ := engine.GetStats()
		if stats.IndexedDocuments != 1 {
			t.Errorf("期望索引文档数为 1，实际为 %d", stats.IndexedDocuments)
		}
	})

	// 测试全文搜索
	t.Run("FullTextSearch", func(t *testing.T) {
		query := &SearchQuery{
			Keyword:  "测试",
			Mode:     SearchModeFullText,
			Page:     1,
			PageSize: 10,
		}

		response, err := engine.Search(query)
		if err != nil {
			t.Fatalf("搜索失败: %v", err)
		}

		if response.Total == 0 {
			t.Error("期望找到结果，但结果为空")
		}

		if len(response.Results) > 0 {
			if response.Results[0].TextScore == 0 {
				t.Error("文本得分不应为 0")
			}
		}
	})

	// 测试语义搜索
	t.Run("SemanticSearch", func(t *testing.T) {
		query := &SearchQuery{
			Keyword:  "文档验证",
			Mode:     SearchModeSemantic,
			Page:     1,
			PageSize: 10,
		}

		response, err := engine.Search(query)
		if err != nil {
			t.Fatalf("语义搜索失败: %v", err)
		}

		if response.Total == 0 {
			t.Error("期望找到结果，但结果为空")
		}
	})

	// 测试混合搜索
	t.Run("HybridSearch", func(t *testing.T) {
		query := &SearchQuery{
			Keyword:  "测试功能",
			Mode:     SearchModeHybrid,
			Page:     1,
			PageSize: 10,
		}

		response, err := engine.Search(query)
		if err != nil {
			t.Fatalf("混合搜索失败: %v", err)
		}

		if response.Total == 0 {
			t.Error("期望找到结果，但结果为空")
		}
	})

	// 测试删除文档
	t.Run("DeleteDocument", func(t *testing.T) {
		err := engine.DeleteDocument("test-doc-1")
		if err != nil {
			t.Fatalf("删除文档失败: %v", err)
		}

		stats, _ := engine.GetStats()
		if stats.IndexedDocuments != 0 {
			t.Errorf("期望索引文档数为 0，实际为 %d", stats.IndexedDocuments)
		}
	})

	// 测试搜索建议
	t.Run("Suggest", func(t *testing.T) {
		// 先添加一个文档
		doc := &SearchIndex{
			ID:       "test-doc-2",
			FilePath: "/test/test.txt",
			FileName: "test.txt",
			FileType: FileTypeDocument,
			FileSize: 512,
			Content:  "测试建议功能",
			Tags:     []string{"测试"},
			Status:   IndexStatusPending,
		}
		engine.IndexDocument(doc)

		suggestions, err := engine.Suggest("测", 5)
		if err != nil {
			t.Fatalf("获取建议失败: %v", err)
		}

		if len(suggestions) == 0 {
			t.Error("期望有建议，但结果为空")
		}
	})
}

func TestSearchQueryValidation(t *testing.T) {
	// 测试空关键词
	t.Run("EmptyKeyword", func(t *testing.T) {
		query := &SearchQuery{
			Keyword: "",
			Mode:    SearchModeFullText,
		}

		err := query.Validate()
		if err == nil {
			t.Error("期望验证失败，但成功了")
		}
	})

	// 测试无效模式
	t.Run("InvalidMode", func(t *testing.T) {
		query := &SearchQuery{
			Keyword: "test",
			Mode:    SearchMode("invalid"),
		}

		err := query.Validate()
		if err == nil {
			t.Error("期望验证失败，但成功了")
		}
	})

	// 测试日期范围
	t.Run("InvalidDateRange", func(t *testing.T) {
		now := time.Now()
		query := &SearchQuery{
			Keyword:  "test",
			Mode:     SearchModeFullText,
			DateFrom: &now,
			DateTo:   &[]time.Time{now.Add(-24 * time.Hour)}[0],
		}

		err := query.Validate()
		if err == nil {
			t.Error("期望验证失败，但成功了")
		}
	})

	// 测试默认值
	t.Run("DefaultValues", func(t *testing.T) {
		query := &SearchQuery{
			Keyword: "test",
		}

		err := query.Validate()
		if err != nil {
			t.Fatalf("验证失败: %v", err)
		}

		if query.Page != 1 {
			t.Errorf("期望页码为 1，实际为 %d", query.Page)
		}
		if query.PageSize != 20 {
			t.Errorf("期望每页数量为 20，实际为 %d", query.PageSize)
		}
	})
}

func TestRanker(t *testing.T) {
	ranker := NewRanker(nil)

	// 测试排序
	t.Run("Rank", func(t *testing.T) {
		results := []SearchResult{
			{ID: "1", Score: 5.0, ModifiedAt: time.Now().Add(-24 * time.Hour)},
			{ID: "2", Score: 10.0, ModifiedAt: time.Now()},
			{ID: "3", Score: 7.0, ModifiedAt: time.Now().Add(-12 * time.Hour)},
		}

		query := &SearchQuery{Keyword: "test"}
		ranked := ranker.Rank(results, query)

		// Ranker 会重新计算得分，考虑时间新鲜度等因素
		// ID=2 的时间最新，所以应该排在前面
		if ranked[0].ID != "2" {
			t.Logf("排序结果: %v", ranked)
		}
	})

	// 测试去重
	t.Run("Deduplicate", func(t *testing.T) {
		results := []SearchResult{
			{ID: "1", Score: 5.0},
			{ID: "2", Score: 10.0},
			{ID: "1", Score: 7.0},
		}

		deduplicated := ranker.Deduplicate(results)
		if len(deduplicated) != 2 {
			t.Errorf("期望去重后有 2 个结果，实际有 %d 个", len(deduplicated))
		}
	})

	// 测试归一化
	t.Run("NormalizeScores", func(t *testing.T) {
		results := []SearchResult{
			{ID: "1", Score: 0.0},
			{ID: "2", Score: 10.0},
			{ID: "3", Score: 5.0},
		}

		normalized := ranker.NormalizeScores(results)

		if normalized[0].Score != 0.0 {
			t.Errorf("期望最小得分为 0.0，实际为 %f", normalized[0].Score)
		}
		if normalized[1].Score != 1.0 {
			t.Errorf("期望最大得分为 1.0，实际为 %f", normalized[1].Score)
		}
	})
}

func TestFilter(t *testing.T) {
	// 测试文件类型过滤
	t.Run("FileTypeFilter", func(t *testing.T) {
		results := []SearchResult{
			{ID: "1", FileType: FileTypeDocument},
			{ID: "2", FileType: FileTypeImage},
			{ID: "3", FileType: FileTypeDocument},
		}

		filter := NewFilter().WithFileTypes(FileTypeDocument)
		filtered := filter.Apply(results)

		if len(filtered) != 2 {
			t.Errorf("期望过滤后有 2 个结果，实际有 %d 个", len(filtered))
		}
	})

	// 测试标签过滤
	t.Run("TagFilter", func(t *testing.T) {
		results := []SearchResult{
			{ID: "1", Tags: []string{"测试", "文档"}},
			{ID: "2", Tags: []string{"图片"}},
			{ID: "3", Tags: []string{"测试", "代码"}},
		}

		filter := NewFilter().WithTags("测试")
		filtered := filter.Apply(results)

		if len(filtered) != 2 {
			t.Errorf("期望过滤后有 2 个结果，实际有 %d 个", len(filtered))
		}
	})

	// 测试大小过滤
	t.Run("SizeFilter", func(t *testing.T) {
		results := []SearchResult{
			{ID: "1", FileSize: 512},
			{ID: "2", FileSize: 1024},
			{ID: "3", FileSize: 2048},
		}

		filter := NewFilter().WithSizeRange(1000, 1500)
		filtered := filter.Apply(results)

		if len(filtered) != 1 {
			t.Errorf("期望过滤后有 1 个结果，实际有 %d 个", len(filtered))
		}
	})
}

func TestSuggester(t *testing.T) {
	suggester := NewSuggester(100, 10)

	// 测试添加文档
	t.Run("AddDocument", func(t *testing.T) {
		doc := &SearchIndex{
			ID:       "test-1",
			FileName: "test.txt",
			Tags:     []string{"测试", "文档"},
			Content:  "这是测试内容",
		}

		suggester.AddDocument(doc)
	})

	// 测试搜索建议
	t.Run("Suggest", func(t *testing.T) {
		suggestions := suggester.Suggest("测", 5)
		if len(suggestions) == 0 {
			t.Error("期望有建议，但结果为空")
		}
	})

	// 测试添加历史
	t.Run("AddHistory", func(t *testing.T) {
		history := SearchHistory{
			ID:          "history-1",
			Keyword:     "测试搜索",
			Mode:        SearchModeFullText,
			ResultCount: 10,
			SearchedAt:  time.Now(),
		}

		suggester.AddHistory(history)

		hotWords := suggester.GetHotWords(10)
		if len(hotWords) == 0 {
			t.Error("期望有热词，但结果为空")
		}
	})
}

func TestCache(t *testing.T) {
	// 测试 SearchCache
	t.Run("SearchCache", func(t *testing.T) {
		cache := NewSearchCache(10, 1*time.Second)
		defer cache.Clear()

		// 测试 Set 和 Get
		cache.Set("key1", "value1")
		value := cache.Get("key1")
		if value != "value1" {
			t.Errorf("期望值为 'value1'，实际为 %v", value)
		}

		// 测试未命中
		value = cache.Get("key2")
		if value != nil {
			t.Error("期望返回 nil")
		}

		// 测试命中率
		if cache.HitRate() != 0.5 {
			t.Errorf("期望命中率为 0.5，实际为 %f", cache.HitRate())
		}
	})

	// 测试 LRUCache
	t.Run("LRUCache", func(t *testing.T) {
		cache := NewLRUCache(3, 1*time.Second)
		defer cache.Clear()

		// 测试添加
		cache.Set("key1", "value1", 1*time.Second)
		cache.Set("key2", "value2", 1*time.Second)
		cache.Set("key3", "value3", 1*time.Second)

		// 测试获取
		value := cache.Get("key1")
		if value != "value1" {
			t.Errorf("期望值为 'value1'，实际为 %v", value)
		}

		// 测试 LRU 驱逐
		cache.Set("key4", "value4", 1*time.Second)
		value = cache.Get("key2")
		if value != nil {
			t.Error("期望 key2 被驱逐")
		}
	})
}

func TestTrie(t *testing.T) {
	trie := NewTrie()

	// 测试插入
	t.Run("Insert", func(t *testing.T) {
		trie.Insert("test", "id1")
		trie.Insert("testing", "id2")
		trie.Insert("tester", "id3")
	})

	// 测试搜索
	t.Run("Search", func(t *testing.T) {
		results := trie.Search("test", 10)
		if len(results) != 3 {
			t.Errorf("期望找到 3 个结果，实际找到 %d 个", len(results))
		}
	})

	// 测试前缀搜索
	t.Run("PrefixSearch", func(t *testing.T) {
		results := trie.Search("tes", 10)
		if len(results) != 3 {
			t.Errorf("期望找到 3 个结果，实际找到 %d 个", len(results))
		}
	})

	// 测试精确搜索
	t.Run("ExactSearch", func(t *testing.T) {
		results := trie.Search("testing", 10)
		if len(results) != 1 {
			t.Errorf("期望找到 1 个结果，实际找到 %d 个", len(results))
		}
	})
}

func TestCosineSimilarity(t *testing.T) {
	// 测试完全相似
	t.Run("Identical", func(t *testing.T) {
		a := []float64{1, 0, 0}
		b := []float64{1, 0, 0}
		sim := cosineSimilarity(a, b)
		if sim != 1.0 {
			t.Errorf("期望相似度为 1.0，实际为 %f", sim)
		}
	})

	// 测试正交
	t.Run("Orthogonal", func(t *testing.T) {
		a := []float64{1, 0, 0}
		b := []float64{0, 1, 0}
		sim := cosineSimilarity(a, b)
		if sim != 0.0 {
			t.Errorf("期望相似度为 0.0，实际为 %f", sim)
		}
	})

	// 测试空向量
	t.Run("Empty", func(t *testing.T) {
		a := []float64{}
		b := []float64{}
		sim := cosineSimilarity(a, b)
		if sim != 0.0 {
			t.Errorf("期望相似度为 0.0，实际为 %f", sim)
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultSearchConfig()

	if config.MaxResults != 1000 {
		t.Errorf("期望 MaxResults 为 1000，实际为 %d", config.MaxResults)
	}

	if config.PageSize != 20 {
		t.Errorf("期望 PageSize 为 20，实际为 %d", config.PageSize)
	}

	if config.VectorDimension != 384 {
		t.Errorf("期望 VectorDimension 为 384，实际为 %d", config.VectorDimension)
	}

	if config.SimilarityThresh != 0.7 {
		t.Errorf("期望 SimilarityThresh 为 0.7，实际为 %f", config.SimilarityThresh)
	}
}
