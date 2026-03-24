package search

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GlobalSearchResultType 全局搜索结果类型
type GlobalSearchResultType string

// 搜索结果类型常量
const (
	// ResultTypeFile 文件类型
	ResultTypeFile      GlobalSearchResultType = "file"
	ResultTypeSetting   GlobalSearchResultType = "setting"
	ResultTypeApp       GlobalSearchResultType = "app"
	ResultTypeContainer GlobalSearchResultType = "container"
)

// GlobalSearchResult 全局搜索结果
type GlobalSearchResult struct {
	Type        GlobalSearchResultType `json:"type"`
	Score       float64                `json:"score"`
	Title       string                 `json:"title"`       // 显示标题
	Description string                 `json:"description"` // 描述
	Path        string                 `json:"path"`        // 访问路径
	Icon        string                 `json:"icon"`        // 图标
	Category    string                 `json:"category"`    // 分类
	MatchType   string                 `json:"matchType"`   // 匹配类型
	MatchField  string                 `json:"matchField"`  // 匹配字段
	RawData     interface{}            `json:"rawData"`     // 原始数据
}

// GlobalSearchRequest 全局搜索请求
type GlobalSearchRequest struct {
	Query      string                   `json:"query"`                // 搜索查询
	Types      []GlobalSearchResultType `json:"types,omitempty"`      // 限制结果类型
	Limit      int                      `json:"limit,omitempty"`      // 每种类型最大结果数
	TotalLimit int                      `json:"totalLimit,omitempty"` // 总结果数限制
	MinScore   float64                  `json:"minScore,omitempty"`   // 最小分数阈值
	IncludeRaw bool                     `json:"includeRaw,omitempty"` // 是否包含原始数据
}

// GlobalSearchResponse 全局搜索响应
type GlobalSearchResponse struct {
	Query       string               `json:"query"`
	Took        time.Duration        `json:"took"` // 搜索耗时
	Total       int                  `json:"total"`
	Files       []GlobalSearchResult `json:"files,omitempty"`
	Settings    []GlobalSearchResult `json:"settings,omitempty"`
	Apps        []GlobalSearchResult `json:"apps,omitempty"`
	Containers  []GlobalSearchResult `json:"containers,omitempty"`
	Suggestions []string             `json:"suggestions,omitempty"` // 搜索建议
}

// GlobalSearchService 全局搜索服务
type GlobalSearchService struct {
	engine           *Engine
	settingsRegistry *SettingsRegistry
	appRegistry      *AppRegistry
	logger           *zap.Logger
}

// NewGlobalSearchService 创建全局搜索服务
func NewGlobalSearchService(
	engine *Engine,
	settingsRegistry *SettingsRegistry,
	appRegistry *AppRegistry,
	logger *zap.Logger,
) *GlobalSearchService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GlobalSearchService{
		engine:           engine,
		settingsRegistry: settingsRegistry,
		appRegistry:      appRegistry,
		logger:           logger,
	}
}

// GlobalSearch 执行全局搜索
func (s *GlobalSearchService) GlobalSearch(ctx context.Context, req GlobalSearchRequest) (*GlobalSearchResponse, error) {
	startTime := time.Now()

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.TotalLimit <= 0 {
		req.TotalLimit = 20
	}
	if req.MinScore <= 0 {
		req.MinScore = 0.3
	}

	// 过滤结果类型
	typeFilter := make(map[GlobalSearchResultType]bool)
	if len(req.Types) > 0 {
		for _, t := range req.Types {
			typeFilter[t] = true
		}
	} else {
		// 默认搜索所有类型
		typeFilter[ResultTypeFile] = true
		typeFilter[ResultTypeSetting] = true
		typeFilter[ResultTypeApp] = true
		typeFilter[ResultTypeContainer] = true
	}

	response := &GlobalSearchResponse{
		Query: req.Query,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// 并发搜索文件
	if typeFilter[ResultTypeFile] && s.engine != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := s.searchFiles(ctx, req)
			mu.Lock()
			response.Files = results
			mu.Unlock()
		}()
	}

	// 并发搜索设置
	if typeFilter[ResultTypeSetting] && s.settingsRegistry != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := s.searchSettings(req)
			mu.Lock()
			response.Settings = results
			mu.Unlock()
		}()
	}

	// 并发搜索应用和容器
	if (typeFilter[ResultTypeApp] || typeFilter[ResultTypeContainer]) && s.appRegistry != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			appResults, containerResults := s.searchApps(req, typeFilter)
			mu.Lock()
			if typeFilter[ResultTypeApp] {
				response.Apps = appResults
			}
			if typeFilter[ResultTypeContainer] {
				response.Containers = containerResults
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// 计算总数
	response.Total = len(response.Files) + len(response.Settings) +
		len(response.Apps) + len(response.Containers)

	// 生成搜索建议
	if response.Total == 0 {
		response.Suggestions = s.GenerateSuggestions(req.Query)
	}

	response.Took = time.Since(startTime)

	return response, nil
}

// searchFiles 搜索文件
func (s *GlobalSearchService) searchFiles(ctx context.Context, req GlobalSearchRequest) []GlobalSearchResult {
	results := make([]GlobalSearchResult, 0)

	if s.engine == nil {
		return results
	}

	// 使用现有的搜索引擎
	searchReq := Request{
		Query: req.Query,
		Limit: req.Limit,
	}

	resp, err := s.engine.Search(searchReq)
	if err != nil {
		s.logger.Debug("文件搜索失败", zap.Error(err))
		return results
	}

	for _, r := range resp.Results {
		if r.Score < req.MinScore {
			continue
		}

		result := GlobalSearchResult{
			Type:        ResultTypeFile,
			Score:       r.Score,
			Title:       r.Name,
			Description: r.Path,
			Path:        r.Path,
			Icon:        s.getFileIcon(r.Ext),
			Category:    "文件",
			MatchType:   "content",
			MatchField:  "name",
		}

		if req.IncludeRaw {
			result.RawData = r
		}

		results = append(results, result)
	}

	return results
}

// searchSettings 搜索设置
func (s *GlobalSearchService) searchSettings(req GlobalSearchRequest) []GlobalSearchResult {
	results := make([]GlobalSearchResult, 0)

	if s.settingsRegistry == nil {
		return results
	}

	settingsResults := s.settingsRegistry.SearchSettings(req.Query, req.Limit)

	for _, r := range settingsResults {
		if r.Score < req.MinScore {
			continue
		}

		result := GlobalSearchResult{
			Type:        ResultTypeSetting,
			Score:       r.Score,
			Title:       r.Setting.Name,
			Description: r.Setting.Description,
			Path:        r.Setting.Path,
			Icon:        r.Setting.Icon,
			Category:    r.Setting.Section,
			MatchType:   r.MatchType,
			MatchField:  r.MatchField,
		}

		if req.IncludeRaw {
			result.RawData = r.Setting
		}

		results = append(results, result)
	}

	return results
}

// searchApps 搜索应用和容器
func (s *GlobalSearchService) searchApps(req GlobalSearchRequest, typeFilter map[GlobalSearchResultType]bool) ([]GlobalSearchResult, []GlobalSearchResult) {
	appResults := make([]GlobalSearchResult, 0)
	containerResults := make([]GlobalSearchResult, 0)

	if s.appRegistry == nil {
		return appResults, containerResults
	}

	appSearchResults := s.appRegistry.SearchApps(req.Query, req.Limit*2)

	for _, r := range appSearchResults {
		if r.Score < req.MinScore {
			continue
		}

		result := GlobalSearchResult{
			Score:      r.Score,
			MatchType:  r.MatchType,
			MatchField: r.MatchField,
		}

		if r.Type == "app" {
			if !typeFilter[ResultTypeApp] {
				continue
			}
			app, ok := r.Item.(AppItem)
			if !ok {
				continue
			}
			result.Type = ResultTypeApp
			result.Title = app.DisplayName
			if result.Title == "" {
				result.Title = app.Name
			}
			result.Description = app.Description
			result.Path = app.Path
			result.Icon = app.Icon
			result.Category = app.Category

			if req.IncludeRaw {
				result.RawData = app
			}

			appResults = append(appResults, result)
		} else {
			if !typeFilter[ResultTypeContainer] {
				continue
			}
			container, ok := r.Item.(ContainerItem)
			if !ok {
				continue
			}
			result.Type = ResultTypeContainer
			result.Title = container.Name
			result.Description = container.Image
			result.Path = "/containers/" + container.ID
			result.Icon = "docker"
			result.Category = "容器"

			if req.IncludeRaw {
				result.RawData = container
			}

			containerResults = append(containerResults, result)
		}
	}

	// 限制数量
	if len(appResults) > req.Limit {
		appResults = appResults[:req.Limit]
	}
	if len(containerResults) > req.Limit {
		containerResults = containerResults[:req.Limit]
	}

	return appResults, containerResults
}

// getFileIcon 获取文件图标
func (s *GlobalSearchService) getFileIcon(ext string) string {
	iconMap := map[string]string{
		".txt":  "file-text",
		".md":   "file-alt",
		".pdf":  "file-pdf",
		".doc":  "file-word",
		".docx": "file-word",
		".xls":  "file-excel",
		".xlsx": "file-excel",
		".ppt":  "file-powerpoint",
		".pptx": "file-powerpoint",
		".jpg":  "file-image",
		".jpeg": "file-image",
		".png":  "file-image",
		".gif":  "file-image",
		".mp4":  "file-video",
		".mp3":  "file-audio",
		".zip":  "file-archive",
		".tar":  "file-archive",
		".gz":   "file-archive",
		".go":   "file-code",
		".py":   "file-code",
		".js":   "file-code",
		".ts":   "file-code",
		".sh":   "file-code",
		".json": "file-code",
		".yaml": "file-code",
		".yml":  "file-code",
		".xml":  "file-code",
		".html": "file-code",
		".css":  "file-code",
	}

	if icon, ok := iconMap[strings.ToLower(ext)]; ok {
		return icon
	}
	return "file"
}

// GenerateSuggestions 生成搜索建议
func (s *GlobalSearchService) GenerateSuggestions(query string) []string {
	suggestions := make([]string, 0)

	// 基于查询生成建议
	query = strings.ToLower(query)

	// 常见搜索建议
	commonSuggestions := map[string][]string{
		"存储": {"存储池", "数据集", "快照", "共享"},
		"网络": {"网络接口", "DNS", "防火墙"},
		"用户": {"用户管理", "用户组", "权限"},
		"备份": {"备份任务", "复制", "云同步"},
		"容器": {"Docker容器", "Docker Compose", "镜像"},
		"应用": {"应用商店", "已安装应用"},
		"监控": {"监控仪表板", "报表中心", "告警"},
		"安全": {"SSH设置", "SSL证书", "审计日志"},
	}

	for key, values := range commonSuggestions {
		if strings.Contains(key, query) {
			suggestions = append(suggestions, values...)
		}
		for _, v := range values {
			if strings.Contains(strings.ToLower(v), query) {
				suggestions = append(suggestions, v)
			}
		}
	}

	// 去重
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, s := range suggestions {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}

	if len(unique) > 5 {
		unique = unique[:5]
	}

	return unique
}

// QuickSearch 快速搜索（用于自动补全）
func (s *GlobalSearchService) QuickSearch(ctx context.Context, query string, limit int) (*GlobalSearchResponse, error) {
	if limit <= 0 {
		limit = 3
	}

	return s.GlobalSearch(ctx, GlobalSearchRequest{
		Query:      query,
		Limit:      limit,
		TotalLimit: limit * 4,
		MinScore:   0.5,
	})
}

// SearchByType 按类型搜索
func (s *GlobalSearchService) SearchByType(ctx context.Context, query string, resultType GlobalSearchResultType, limit int) (*GlobalSearchResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	return s.GlobalSearch(ctx, GlobalSearchRequest{
		Query: query,
		Types: []GlobalSearchResultType{resultType},
		Limit: limit,
	})
}

// GetSearchCategories 获取搜索分类
func (s *GlobalSearchService) GetSearchCategories() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type":  "file",
			"name":  "文件",
			"icon":  "file",
			"count": 0, // 可以动态获取
		},
		{
			"type":  "setting",
			"name":  "设置",
			"icon":  "cog",
			"count": 0,
		},
		{
			"type":  "app",
			"name":  "应用",
			"icon":  "box",
			"count": 0,
		},
		{
			"type":  "container",
			"name":  "容器",
			"icon":  "docker",
			"count": 0,
		},
	}
}

// GetPopularSearches 获取热门搜索
func (s *GlobalSearchService) GetPopularSearches() []string {
	return []string{
		"存储池",
		"用户管理",
		"Docker容器",
		"快照",
		"网络设置",
		"备份",
		"SSL证书",
		"监控",
	}
}

// GetRecentSearches 获取最近搜索（需要持久化存储）
func (s *GlobalSearchService) GetRecentSearches() []string {
	// TODO: 实现持久化存储
	return []string{}
}

// ClearRecentSearches 清除最近搜索
func (s *GlobalSearchService) ClearRecentSearches() error {
	// TODO: 实现持久化存储
	return nil
}
