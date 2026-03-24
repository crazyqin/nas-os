package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestSettingsRegistry_Search(t *testing.T) {
	registry := NewSettingsRegistry()

	tests := []struct {
		name     string
		query    string
		minCount int
	}{
		{"搜索存储", "存储", 3},
		{"搜索网络", "网络", 3},
		{"搜索用户", "用户", 2},
		{"搜索Docker", "Docker", 2},
		{"搜索备份", "备份", 2},
		{"搜索SSH", "SSH", 1},
		{"搜索监控", "监控", 1},
		{"英文搜索 storage", "storage", 1},
		{"英文搜索 network", "network", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.SearchSettings(tt.query, 10)
			if len(results) < tt.minCount {
				t.Errorf("期望至少 %d 个结果，实际 %d", tt.minCount, len(results))
			}

			t.Logf("查询 '%s' 找到 %d 个结果:", tt.query, len(results))
			for _, r := range results {
				t.Logf("  - %s (score: %.2f, type: %s)", r.Setting.Name, r.Score, r.MatchType)
			}
		})
	}
}

func TestSettingsRegistry_GetByCategory(t *testing.T) {
	registry := NewSettingsRegistry()

	tests := []struct {
		category string
		minCount int
	}{
		{"storage", 4},
		{"network", 5},
		{"system", 6},
		{"security", 5},
		{"services", 5},
		{"containers", 4},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			items := registry.GetByCategory(tt.category)
			if len(items) < tt.minCount {
				t.Errorf("分类 %s 期望至少 %d 个设置项，实际 %d", tt.category, tt.minCount, len(items))
			}
		})
	}
}

func TestSettingsRegistry_GetCategories(t *testing.T) {
	registry := NewSettingsRegistry()

	categories := registry.GetCategories()
	if len(categories) == 0 {
		t.Error("期望获取到分类列表")
	}

	t.Logf("找到 %d 个分类: %v", len(categories), categories)
}

func TestSettingsRegistry_GetByPath(t *testing.T) {
	registry := NewSettingsRegistry()

	item := registry.GetByPath("/storage/pools")
	if item == nil {
		t.Error("期望找到存储池设置项")
		return
	}

	if item.Name != "存储池管理" {
		t.Errorf("期望名称为 '存储池管理'，实际 '%s'", item.Name)
	}
}

func TestAppRegistry_Search(t *testing.T) {
	registry := NewAppRegistry()

	// 注册测试应用
	registry.RegisterApp([]AppItem{
		{
			ID:          "jellyfin",
			Name:        "jellyfin",
			DisplayName: "Jellyfin",
			Description: "开源媒体服务器",
			Category:    "media",
			Version:     "10.8.0",
			Status:      "running",
			Keywords:    []string{"媒体", "视频", "音乐", "media", "streaming"},
			Type:        "app",
		},
		{
			ID:          "nextcloud",
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "私有云存储和协作平台",
			Category:    "productivity",
			Version:     "27.0.0",
			Status:      "running",
			Keywords:    []string{"云", "存储", "文件", "cloud", "storage"},
			Type:        "app",
		},
		{
			ID:          "homeassistant",
			Name:        "homeassistant",
			DisplayName: "Home Assistant",
			Description: "智能家居自动化平台",
			Category:    "automation",
			Version:     "2023.1",
			Status:      "stopped",
			Keywords:    []string{"智能家居", "自动化", "IoT", "home", "smart"},
			Type:        "app",
		},
	}...)

	// 注册测试容器
	registry.RegisterContainer([]ContainerItem{
		{
			ID:     "abc123",
			Name:   "nginx-proxy",
			Image:  "nginx:latest",
			Status: "running",
			Ports:  []string{"80:80", "443:443"},
			Keywords: []string{"proxy", "web", "http"},
		},
		{
			ID:     "def456",
			Name:   "redis-cache",
			Image:  "redis:7-alpine",
			Status: "running",
			Keywords: []string{"cache", "redis"},
		},
	}...)

	tests := []struct {
		name     string
		query    string
		minCount int
	}{
		{"搜索媒体", "媒体", 1},
		{"搜索云", "云", 1},
		{"搜索nginx", "nginx", 1},
		{"搜索redis", "redis", 1},
		{"搜索running", "running", 3},
		{"搜索Jellyfin", "Jellyfin", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.SearchApps(tt.query, 10)
			if len(results) < tt.minCount {
				t.Errorf("期望至少 %d 个结果，实际 %d", tt.minCount, len(results))
			}

			t.Logf("查询 '%s' 找到 %d 个结果:", tt.query, len(results))
			for _, r := range results {
				t.Logf("  - type: %s, score: %.2f, match: %s", r.Type, r.Score, r.MatchField)
			}
		})
	}
}

func TestAppRegistry_GetStats(t *testing.T) {
	registry := NewAppRegistry()

	registry.RegisterApp([]AppItem{
		{ID: "app1", Name: "app1", Status: "running"},
		{ID: "app2", Name: "app2", Status: "running"},
		{ID: "app3", Name: "app3", Status: "stopped"},
	}...)

	registry.RegisterContainer([]ContainerItem{
		{ID: "c1", Name: "c1", Status: "running"},
		{ID: "c2", Name: "c2", Status: "stopped"},
	}...)

	stats := registry.GetAppStats()

	appStats := stats["apps"].(map[string]int)
	if appStats["total"] != 3 {
		t.Errorf("期望 3 个应用，实际 %d", appStats["total"])
	}
	if appStats["running"] != 2 {
		t.Errorf("期望 2 个运行中应用，实际 %d", appStats["running"])
	}

	containerStats := stats["containers"].(map[string]int)
	if containerStats["total"] != 2 {
		t.Errorf("期望 2 个容器，实际 %d", containerStats["total"])
	}
}

func TestGlobalSearchService_GlobalSearch(t *testing.T) {
	// 创建临时目录和索引
	tmpDir, err := os.MkdirTemp("", "global-search-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFiles := map[string]string{
		"test.txt":     "这是一个测试文件，用于测试全局搜索功能",
		"readme.md":    "# 全局搜索\n\n这是一个全局搜索组件的实现",
		"config.json":  `{"name": "test", "type": "config"}`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 创建搜索引擎
	indexDir, err := os.MkdirTemp("", "global-search-index-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(indexDir)

	config := IndexConfig{
		IndexPath:    filepath.Join(indexDir, "test.bleve"),
		MaxFileSize:  1024 * 1024,
		Workers:      2,
		IndexContent: true,
		BatchSize:    10,
		TextExts:     []string{".txt", ".md", ".json"},
		ExcludeDirs:  []string{},
		ExcludeFiles: []string{},
	}

	logger := zap.NewNop()
	engine, err := NewEngine(config, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 索引测试文件
	if err := engine.IndexDirectory(tmpDir); err != nil {
		t.Fatal(err)
	}

	// 创建注册表
	settingsRegistry := NewSettingsRegistry()
	appRegistry := NewAppRegistry()

	// 注册测试应用
	appRegistry.RegisterApp([]AppItem{
		{
			ID:          "test-app",
			Name:        "test-app",
			DisplayName: "测试应用",
			Description: "用于测试全局搜索",
			Status:      "running",
			Keywords:    []string{"测试", "search"},
		},
	}...)

	// 创建全局搜索服务
	globalSearch := NewGlobalSearchService(engine, settingsRegistry, appRegistry, logger)

	// 执行搜索
	tests := []struct {
		name     string
		query    string
		minTotal int
	}{
		{"搜索测试", "测试", 1},
		{"搜索存储", "存储", 1},
		{"搜索网络", "网络", 1},
		{"搜索用户", "用户", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := globalSearch.GlobalSearch(context.Background(), GlobalSearchRequest{
				Query:      tt.query,
				Limit:      5,
				TotalLimit: 20,
				MinScore:   0.1,
			})
			if err != nil {
				t.Fatal(err)
			}

			if result.Total < tt.minTotal {
				t.Errorf("期望至少 %d 个结果，实际 %d", tt.minTotal, result.Total)
			}

			t.Logf("查询 '%s' 找到 %d 个结果 (耗时: %v):", tt.query, result.Total, result.Took)
			for _, r := range result.Files {
				t.Logf("  [文件] %s (score: %.2f)", r.Title, r.Score)
			}
			for _, r := range result.Settings {
				t.Logf("  [设置] %s (score: %.2f)", r.Title, r.Score)
			}
			for _, r := range result.Apps {
				t.Logf("  [应用] %s (score: %.2f)", r.Title, r.Score)
			}
		})
	}
}

func TestGlobalSearchService_QuickSearch(t *testing.T) {
	logger := zap.NewNop()
	settingsRegistry := NewSettingsRegistry()
	appRegistry := NewAppRegistry()
	globalSearch := NewGlobalSearchService(nil, settingsRegistry, appRegistry, logger)

	result, err := globalSearch.QuickSearch(context.Background(), "存储", 3)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("快速搜索 '存储' 找到 %d 个设置结果", len(result.Settings))
}

func TestGlobalSearchService_GetFileIcon(t *testing.T) {
	logger := zap.NewNop()
	globalSearch := NewGlobalSearchService(nil, nil, nil, logger)

	tests := []struct {
		ext      string
		expected string
	}{
		{".txt", "file-text"},
		{".pdf", "file-pdf"},
		{".jpg", "file-image"},
		{".mp4", "file-video"},
		{".go", "file-code"},
		{".unknown", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			icon := globalSearch.getFileIcon(tt.ext)
			if icon != tt.expected {
				t.Errorf("期望图标 '%s'，实际 '%s'", tt.expected, icon)
			}
		})
	}
}

func TestGlobalSearchService_GenerateSuggestions(t *testing.T) {
	logger := zap.NewNop()
	settingsRegistry := NewSettingsRegistry()
	appRegistry := NewAppRegistry()
	globalSearch := NewGlobalSearchService(nil, settingsRegistry, appRegistry, logger)

	suggestions := globalSearch.GenerateSuggestions("存")
	if len(suggestions) == 0 {
		t.Error("期望生成搜索建议")
	}

	t.Logf("建议: %v", suggestions)
}

func TestGlobalSearchService_SearchByType(t *testing.T) {
	logger := zap.NewNop()
	settingsRegistry := NewSettingsRegistry()
	appRegistry := NewAppRegistry()
	globalSearch := NewGlobalSearchService(nil, settingsRegistry, appRegistry, logger)

	// 只搜索设置
	result, err := globalSearch.SearchByType(context.Background(), "存储", ResultTypeSetting, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Settings) == 0 {
		t.Error("期望找到设置结果")
	}

	if len(result.Files) > 0 || len(result.Apps) > 0 || len(result.Containers) > 0 {
		t.Error("期望只有设置结果")
	}

	t.Logf("按类型搜索找到 %d 个设置结果", len(result.Settings))
}

func TestGlobalSearchService_GetSearchCategories(t *testing.T) {
	logger := zap.NewNop()
	globalSearch := NewGlobalSearchService(nil, nil, nil, logger)

	categories := globalSearch.GetSearchCategories()
	if len(categories) != 4 {
		t.Errorf("期望 4 个分类，实际 %d", len(categories))
	}

	t.Logf("分类: %v", categories)
}

func TestGlobalSearchService_GetPopularSearches(t *testing.T) {
	logger := zap.NewNop()
	globalSearch := NewGlobalSearchService(nil, nil, nil, logger)

	popular := globalSearch.GetPopularSearches()
	if len(popular) == 0 {
		t.Error("期望获取热门搜索")
	}

	t.Logf("热门搜索: %v", popular)
}