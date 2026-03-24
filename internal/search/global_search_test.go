// Package search 提供全局搜索服务测试
package search

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGlobalSearchService_New(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	assert.NotNil(t, service)
	assert.NotNil(t, service.metadataIndex)
	assert.NotNil(t, service.history)
	assert.Equal(t, 100, service.maxHistory)
}

func TestGlobalSearchService_GlobalSearch(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, NewSettingsRegistry(), nil, logger)

	ctx := context.Background()
	req := GlobalSearchRequest{
		Query:    "存储",
		Limit:    5,
		MinScore: 0.1,
	}

	resp, err := service.GlobalSearch(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "存储", resp.Query)
	assert.True(t, resp.Took >= 0)
	assert.NotNil(t, resp.Facets)
}

func TestGlobalSearchService_SearchMetadata(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	// 添加元数据
	service.AddMetadata(MetadataItem{
		ID:          "photo1",
		Type:        "photo",
		Title:       "海滩日落",
		Description: "美丽的海滩日落照片",
		Tags:        []string{"风景", "日落", "海滩"},
		Path:        "/photos/beach-sunset.jpg",
	})

	service.AddMetadata(MetadataItem{
		ID:          "video1",
		Type:        "video",
		Title:       "旅行视频",
		Description: "欧洲旅行记录",
		Tags:        []string{"旅行", "欧洲"},
		Path:        "/videos/travel.mp4",
	})

	req := GlobalSearchRequest{
		Query:      "日落",
		Limit:      10,
		MinScore:   0.1,
		IncludeRaw: true,
	}

	results := service.searchMetadata(req)

	assert.NotEmpty(t, results)
	// 检查结果包含正确的内容
	assert.Equal(t, ResultTypeMetadata, results[0].Type)
	assert.Contains(t, results[0].Title, "日落")
}

func TestGlobalSearchService_SearchHistory(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	// 记录搜索历史
	service.recordHistory("存储池")
	service.recordHistory("用户管理")
	service.recordHistory("存储池") // 重复搜索

	history := service.GetRecentSearches()
	assert.Len(t, history, 2) // 去重后应该是2个
	// 验证历史记录包含正确的内容
	assert.Contains(t, history, "存储池")
	assert.Contains(t, history, "用户管理")
	// 验证搜索次数
	assert.Equal(t, 2, service.history[0].Count)
}

func TestGlobalSearchService_PopularSearches(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	// 记录多次搜索
	service.recordHistory("Docker")
	service.recordHistory("Docker")
	service.recordHistory("Docker")
	service.recordHistory("存储")
	service.recordHistory("存储")
	service.recordHistory("用户")

	popular := service.GetPopularSearches()
	assert.NotEmpty(t, popular)
	// Docker 搜索次数最多，应该排在第一位
	assert.Equal(t, "Docker", popular[0])
}

func TestGlobalSearchService_GenerateSuggestions(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	tests := []struct {
		query    string
		expected int
	}{
		{"存", 2}, // 存储、存储池
		{"网", 2}, // 网络、网络接口
		{"容", 2}, // 容器、Docker容器
		{"xyz", 0}, // 无匹配
	}

	for _, tt := range tests {
		suggestions := service.GenerateSuggestions(tt.query)
		assert.GreaterOrEqual(t, len(suggestions), tt.expected, "query: %s", tt.query)
	}
}

func TestGlobalSearchService_QuickSearch(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, NewSettingsRegistry(), nil, logger)

	ctx := context.Background()
	resp, err := service.QuickSearch(ctx, "存储", 3)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGlobalSearchService_SearchByType(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, NewSettingsRegistry(), nil, logger)

	ctx := context.Background()
	resp, err := service.SearchByType(ctx, "网络", ResultTypeSetting, 10)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Settings)
	assert.Empty(t, resp.Files) // 只搜索设置
}

func TestGlobalSearchService_GetStats(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, NewSettingsRegistry(), nil, logger)

	ctx := context.Background()

	// 执行几次搜索
	for i := 0; i < 5; i++ {
		_, _ = service.GlobalSearch(ctx, GlobalSearchRequest{Query: "test"})
	}

	stats := service.GetStats()
	assert.Equal(t, int64(5), stats.TotalSearches)
	assert.True(t, stats.AverageLatency > 0)
}

func TestGlobalSearchService_ClearHistory(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	service.recordHistory("test1")
	service.recordHistory("test2")

	err := service.ClearRecentSearches()
	assert.NoError(t, err)

	history := service.GetRecentSearches()
	assert.Empty(t, history)
}

func TestGlobalSearchService_MetadataScore(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	item := MetadataItem{
		Title:       "测试文档",
		Description: "这是一个测试文档",
		Tags:        []string{"测试", "文档"},
		Attributes: map[string]interface{}{
			"author": "testuser",
		},
	}

	tests := []struct {
		query     string
		minScore  float64
		expectHit bool
	}{
		{"测试", 0.3, true},     // 标题匹配
		{"文档", 0.2, true},     // 标题和描述匹配
		{"testuser", 0.1, true}, // 属性匹配
		{"xyz", 0.1, false},     // 无匹配
	}

	for _, tt := range tests {
		score := service.calculateMetadataScore(item, tt.query)
		if tt.expectHit {
			assert.GreaterOrEqual(t, score, tt.minScore, "query: %s", tt.query)
		} else {
			assert.Less(t, score, tt.minScore, "query: %s", tt.query)
		}
	}
}

func TestGlobalSearchService_IndexMetadata(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, nil, nil, logger)

	items := []MetadataItem{
		{ID: "1", Type: "photo", Title: "照片1"},
		{ID: "2", Type: "photo", Title: "照片2"},
		{ID: "3", Type: "video", Title: "视频1"},
	}

	service.IndexMetadata(items)

	photos := service.GetMetadataByType("photo")
	assert.Len(t, photos, 2)

	videos := service.GetMetadataByType("video")
	assert.Len(t, videos, 1)
}

func TestGlobalSearchService_Concurrent(t *testing.T) {
	logger := zap.NewNop()
	service := NewGlobalSearchService(nil, NewSettingsRegistry(), nil, logger)

	ctx := context.Background()
	var wg sync.WaitGroup

	// 并发搜索
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = service.GlobalSearch(ctx, GlobalSearchRequest{Query: "测试"})
		}()
	}

	wg.Wait()
	stats := service.GetStats()
	assert.Equal(t, int64(10), stats.TotalSearches)
}

