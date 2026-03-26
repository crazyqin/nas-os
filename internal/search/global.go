// Package search 提供全局搜索服务
// 参考 TrueNAS Electric Eel 的全局搜索功能实现
package search

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GlobalSearchResultType 全局搜索结果类型.
type GlobalSearchResultType string

// 搜索结果类型常量.
const (
	// ResultTypeFile 文件类型.
	ResultTypeFile      GlobalSearchResultType = "file"
	ResultTypeSetting   GlobalSearchResultType = "setting"
	ResultTypeApp       GlobalSearchResultType = "app"
	ResultTypeContainer GlobalSearchResultType = "container"
	ResultTypeMetadata  GlobalSearchResultType = "metadata" // 元数据搜索
	ResultTypeAPI       GlobalSearchResultType = "api"     // API端点搜索
	ResultTypeDoc       GlobalSearchResultType = "doc"     // 文档搜索
	ResultTypeLog       GlobalSearchResultType = "log"     // 日志搜索
)

// GlobalSearchResult 全局搜索结果.
type GlobalSearchResult struct {
	Type        GlobalSearchResultType `json:"type"`
	Score       float64                `json:"score"`
	Title       string                 `json:"title"`              // 显示标题
	Description string                 `json:"description"`        // 描述
	Path        string                 `json:"path"`               // 访问路径
	Icon        string                 `json:"icon"`               // 图标
	Category    string                 `json:"category"`           // 分类
	MatchType   string                 `json:"matchType"`          // 匹配类型
	MatchField  string                 `json:"matchField"`         // 匹配字段
	RawData     interface{}            `json:"rawData"`            // 原始数据
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // 元数据信息
}

// GlobalSearchRequest 全局搜索请求.
type GlobalSearchRequest struct {
	Query      string                   `json:"query"`                // 搜索查询
	Types      []GlobalSearchResultType `json:"types,omitempty"`      // 限制结果类型
	Limit      int                      `json:"limit,omitempty"`      // 每种类型最大结果数
	TotalLimit int                      `json:"totalLimit,omitempty"` // 总结果数限制
	MinScore   float64                  `json:"minScore,omitempty"`   // 最小分数阈值
	IncludeRaw bool                     `json:"includeRaw,omitempty"` // 是否包含原始数据
	Fuzzy      bool                     `json:"fuzzy,omitempty"`      // 是否模糊搜索
	Locale     string                   `json:"locale,omitempty"`     // 语言环境
}

// GlobalSearchResponse 全局搜索响应.
type GlobalSearchResponse struct {
	Query       string               `json:"query"`
	Took        time.Duration        `json:"took"` // 搜索耗时
	Total       int                  `json:"total"`
	Files       []GlobalSearchResult `json:"files,omitempty"`
	Settings    []GlobalSearchResult `json:"settings,omitempty"`
	Apps        []GlobalSearchResult `json:"apps,omitempty"`
	Containers  []GlobalSearchResult `json:"containers,omitempty"`
	Metadata    []GlobalSearchResult `json:"metadata,omitempty"`
	APIs        []GlobalSearchResult `json:"apis,omitempty"`
	Docs        []GlobalSearchResult `json:"docs,omitempty"`
	Logs        []GlobalSearchResult `json:"logs,omitempty"`
	Suggestions []string             `json:"suggestions,omitempty"` // 搜索建议
	Facets      map[string]int       `json:"facets,omitempty"`      // 分面统计
}

// SearchHistory 搜索历史记录.
type SearchHistory struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"` // 搜索次数
}

// GlobalSearchService 全局搜索服务.
type GlobalSearchService struct {
	engine           *Engine
	settingsRegistry *SettingsRegistry
	appRegistry      *AppRegistry
	metadataIndex    *MetadataIndex // 元数据索引
	apiRegistry      *APIRegistry   // API端点注册表
	docRegistry      *DocRegistry   // 文档注册表
	logRegistry      *LogRegistry   // 日志注册表
	logger           *zap.Logger

	// 搜索历史
	history    []SearchHistory
	historyMu  sync.RWMutex
	maxHistory int

	// 性能指标
	stats   *SearchStats
	statsMu sync.RWMutex
}

// SearchStats 搜索统计.
type SearchStats struct {
	TotalSearches  int64         `json:"totalSearches"`
	AverageLatency time.Duration `json:"averageLatency"`
	CacheHits      int64         `json:"cacheHits"`
	CacheMisses    int64         `json:"cacheMisses"`
	LastUpdated    time.Time     `json:"lastUpdated"`
}

// MetadataIndex 元数据索引.
type MetadataIndex struct {
	items map[string][]MetadataItem
	mu    sync.RWMutex
}

// MetadataItem 元数据项.
type MetadataItem struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // photo, video, music, document
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	Attributes  map[string]interface{} `json:"attributes"`
	Path        string                 `json:"path"`
	IndexedAt   time.Time              `json:"indexedAt"`
}

// NewGlobalSearchService 创建全局搜索服务.
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
		metadataIndex:    NewMetadataIndex(),
		apiRegistry:      NewAPIRegistry(),
		docRegistry:      NewDocRegistry(),
		logRegistry:      NewLogRegistry(nil),
		logger:           logger,
		history:          make([]SearchHistory, 0),
		maxHistory:       100,
		stats:            &SearchStats{},
	}
}

// NewMetadataIndex 创建元数据索引.
func NewMetadataIndex() *MetadataIndex {
	return &MetadataIndex{
		items: make(map[string][]MetadataItem),
	}
}

// GlobalSearch 执行全局搜索.
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
		typeFilter[ResultTypeMetadata] = true
		typeFilter[ResultTypeAPI] = true
		typeFilter[ResultTypeDoc] = true
		typeFilter[ResultTypeLog] = true
	}

	response := &GlobalSearchResponse{
		Query:  req.Query,
		Facets: make(map[string]int),
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
			response.Facets["file"] = len(results)
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
			response.Facets["setting"] = len(results)
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
				response.Facets["app"] = len(appResults)
			}
			if typeFilter[ResultTypeContainer] {
				response.Containers = containerResults
				response.Facets["container"] = len(containerResults)
			}
			mu.Unlock()
		}()
	}

	// 并发搜索元数据
	if typeFilter[ResultTypeMetadata] && s.metadataIndex != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := s.searchMetadata(req)
			mu.Lock()
			response.Metadata = results
			response.Facets["metadata"] = len(results)
			mu.Unlock()
		}()
	}

	// 并发搜索API端点
	if typeFilter[ResultTypeAPI] && s.apiRegistry != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := s.searchAPIs(req)
			mu.Lock()
			response.APIs = results
			response.Facets["api"] = len(results)
			mu.Unlock()
		}()
	}

	// 并发搜索文档
	if typeFilter[ResultTypeDoc] && s.docRegistry != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := s.searchDocs(req)
			mu.Lock()
			response.Docs = results
			response.Facets["doc"] = len(results)
			mu.Unlock()
		}()
	}

	// 并发搜索日志
	if typeFilter[ResultTypeLog] && s.logRegistry != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := s.searchLogs(req)
			mu.Lock()
			response.Logs = results
			response.Facets["log"] = len(results)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// 计算总数
	response.Total = len(response.Files) + len(response.Settings) +
		len(response.Apps) + len(response.Containers) + len(response.Metadata) +
		len(response.APIs) + len(response.Docs) + len(response.Logs)

	// 生成搜索建议
	if response.Total == 0 {
		response.Suggestions = s.GenerateSuggestions(req.Query)
	}

	response.Took = time.Since(startTime)

	// 记录搜索历史
	s.recordHistory(req.Query)

	// 更新统计
	s.updateStats(response.Took)

	return response, nil
}

// searchMetadata 搜索元数据.
func (s *GlobalSearchService) searchMetadata(req GlobalSearchRequest) []GlobalSearchResult {
	results := make([]GlobalSearchResult, 0)

	if s.metadataIndex == nil {
		return results
	}

	s.metadataIndex.mu.RLock()
	defer s.metadataIndex.mu.RUnlock()

	query := strings.ToLower(req.Query)

	for _, items := range s.metadataIndex.items {
		for _, item := range items {
			score := s.calculateMetadataScore(item, query)
			if score < req.MinScore {
				continue
			}

			result := GlobalSearchResult{
				Type:        ResultTypeMetadata,
				Score:       score,
				Title:       item.Title,
				Description: item.Description,
				Path:        item.Path,
				Icon:        s.getMetadataIcon(item.Type),
				Category:    item.Type,
				MatchType:   "metadata",
				Metadata:    item.Attributes,
			}

			if req.IncludeRaw {
				result.RawData = item
			}

			results = append(results, result)

			if len(results) >= req.Limit {
				return results
			}
		}
	}

	return results
}

// calculateMetadataScore 计算元数据匹配分数.
func (s *GlobalSearchService) calculateMetadataScore(item MetadataItem, query string) float64 {
	score := 0.0

	// 标题匹配
	if strings.Contains(strings.ToLower(item.Title), query) {
		score += 0.5
	}

	// 描述匹配
	if strings.Contains(strings.ToLower(item.Description), query) {
		score += 0.3
	}

	// 标签匹配
	for _, tag := range item.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			score += 0.2
			break
		}
	}

	// 属性匹配
	for k, v := range item.Attributes {
		if strings.Contains(strings.ToLower(k), query) {
			score += 0.1
		}
		if strVal, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(strVal), query) {
				score += 0.1
			}
		}
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// getMetadataIcon 获取元数据图标.
func (s *GlobalSearchService) getMetadataIcon(metaType string) string {
	iconMap := map[string]string{
		"photo":    "image",
		"video":    "video",
		"music":    "music",
		"document": "file-alt",
	}
	if icon, ok := iconMap[metaType]; ok {
		return icon
	}
	return "file"
}

// searchFiles 搜索文件.
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

// searchSettings 搜索设置.
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

// searchApps 搜索应用和容器.
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

// getFileIcon 获取文件图标.
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

// recordHistory 记录搜索历史.
func (s *GlobalSearchService) recordHistory(query string) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	// 查找是否已存在
	for i := range s.history {
		if s.history[i].Query == query {
			s.history[i].Count++
			s.history[i].Timestamp = time.Now()
			return
		}
	}

	// 添加新记录
	s.history = append(s.history, SearchHistory{
		Query:     query,
		Timestamp: time.Now(),
		Count:     1,
	})

	// 限制历史记录数量
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
}

// updateStats 更新统计信息.
func (s *GlobalSearchService) updateStats(latency time.Duration) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.stats.TotalSearches++
	// 计算移动平均延迟
	if s.stats.TotalSearches == 1 {
		s.stats.AverageLatency = latency
	} else {
		// 指数移动平均
		alpha := 0.1
		s.stats.AverageLatency = time.Duration(
			float64(s.stats.AverageLatency)*(1-alpha) + float64(latency)*alpha,
		)
	}
	s.stats.LastUpdated = time.Now()
}

// GetStats 获取统计信息.
func (s *GlobalSearchService) GetStats() *SearchStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.stats
}

// GenerateSuggestions 生成搜索建议.
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
		"穿透": {"内网穿透", "远程访问", "隧道"},
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
	for _, sug := range suggestions {
		if !seen[sug] {
			seen[sug] = true
			unique = append(unique, sug)
		}
	}

	if len(unique) > 5 {
		unique = unique[:5]
	}

	return unique
}

// QuickSearch 快速搜索（用于自动补全）.
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

// SearchByType 按类型搜索.
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

// AddMetadata 添加元数据项.
func (s *GlobalSearchService) AddMetadata(item MetadataItem) {
	s.metadataIndex.mu.Lock()
	defer s.metadataIndex.mu.Unlock()

	item.IndexedAt = time.Now()
	s.metadataIndex.items[item.Type] = append(s.metadataIndex.items[item.Type], item)
}

// GetSearchCategories 获取搜索分类.
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
		{
			"type":  "metadata",
			"name":  "元数据",
			"icon":  "tags",
			"count": 0,
		},
	}
}

// GetPopularSearches 获取热门搜索.
func (s *GlobalSearchService) GetPopularSearches() []string {
	// 基于历史记录排序
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	// 复制并按次数排序
	history := make([]SearchHistory, len(s.history))
	copy(history, s.history)

	// 简单排序（实际应用中可使用更高效的算法）
	for i := 0; i < len(history); i++ {
		for j := i + 1; j < len(history); j++ {
			if history[j].Count > history[i].Count {
				history[i], history[j] = history[j], history[i]
			}
		}
	}

	result := make([]string, 0, 8)
	for i := 0; i < len(history) && i < 8; i++ {
		result = append(result, history[i].Query)
	}

	// 如果历史记录不足，返回默认热门搜索
	if len(result) < 5 {
		defaultPopular := []string{
			"存储池",
			"用户管理",
			"Docker容器",
			"快照",
			"网络设置",
			"备份",
			"SSL证书",
			"监控",
		}
		for _, q := range defaultPopular {
			if len(result) >= 8 {
				break
			}
			found := false
			for _, r := range result {
				if r == q {
					found = true
					break
				}
			}
			if !found {
				result = append(result, q)
			}
		}
	}

	return result
}

// GetRecentSearches 获取最近搜索.
func (s *GlobalSearchService) GetRecentSearches() []string {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	result := make([]string, 0, 10)
	for i := len(s.history) - 1; i >= 0 && len(result) < 10; i-- {
		result = append(result, s.history[i].Query)
	}
	return result
}

// ClearRecentSearches 清除最近搜索.
func (s *GlobalSearchService) ClearRecentSearches() error {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	s.history = make([]SearchHistory, 0)
	return nil
}

// IndexMetadata 批量索引元数据.
func (s *GlobalSearchService) IndexMetadata(items []MetadataItem) {
	for _, item := range items {
		s.AddMetadata(item)
	}
}

// GetMetadataByType 按类型获取元数据.
func (s *GlobalSearchService) GetMetadataByType(metaType string) []MetadataItem {
	s.metadataIndex.mu.RLock()
	defer s.metadataIndex.mu.RUnlock()
	return s.metadataIndex.items[metaType]
}

// searchAPIs 搜索API端点.
func (s *GlobalSearchService) searchAPIs(req GlobalSearchRequest) []GlobalSearchResult {
	results := make([]GlobalSearchResult, 0)

	if s.apiRegistry == nil {
		return results
	}

	apiResults := s.apiRegistry.SearchAPIs(req.Query, req.Limit)

	for _, r := range apiResults {
		if r.Score < req.MinScore {
			continue
		}

		result := GlobalSearchResult{
			Type:        ResultTypeAPI,
			Score:       r.Score,
			Title:       r.Endpoint.Summary,
			Description: fmt.Sprintf("%s %s - %s", r.Endpoint.Method, r.Endpoint.Path, r.Endpoint.Description),
			Path:        r.Endpoint.Path,
			Icon:        getMethodIcon(r.Endpoint.Method),
			Category:    strings.Join(r.Endpoint.Tags, ", "),
			MatchType:   r.MatchType,
			MatchField:  r.MatchField,
		}

		if req.IncludeRaw {
			result.RawData = r.Endpoint
		}

		results = append(results, result)
	}

	return results
}

// searchDocs 搜索文档.
func (s *GlobalSearchService) searchDocs(req GlobalSearchRequest) []GlobalSearchResult {
	results := make([]GlobalSearchResult, 0)

	if s.docRegistry == nil {
		return results
	}

	docResults := s.docRegistry.SearchDocs(req.Query, req.Limit)

	for _, r := range docResults {
		if r.Score < req.MinScore {
			continue
		}

		result := GlobalSearchResult{
			Type:        ResultTypeDoc,
			Score:       r.Score,
			Title:       r.Doc.Title,
			Description: r.Doc.Content,
			Path:        r.Doc.Path,
			Icon:        r.Doc.Icon,
			Category:    r.Doc.Section,
			MatchType:   r.MatchType,
			MatchField:  r.MatchField,
		}

		if req.IncludeRaw {
			result.RawData = r.Doc
		}

		results = append(results, result)
	}

	return results
}

// searchLogs 搜索日志.
func (s *GlobalSearchService) searchLogs(req GlobalSearchRequest) []GlobalSearchResult {
	results := make([]GlobalSearchResult, 0)

	if s.logRegistry == nil {
		return results
	}

	logReq := LogSearchRequest{
		Query:      req.Query,
		Limit:      req.Limit,
		IgnoreCase: true,
	}

	logResp, err := s.logRegistry.SearchLogs(logReq)
	if err != nil {
		s.logger.Debug("日志搜索失败", zap.Error(err))
		return results
	}

	for _, r := range logResp.Results {
		if r.Score < req.MinScore {
			continue
		}

		result := GlobalSearchResult{
			Type:        ResultTypeLog,
			Score:       r.Score,
			Title:       fmt.Sprintf("[%s] %s", r.Entry.Level, truncateText(r.Entry.Message, 50)),
			Description: r.Entry.RawLine,
			Path:        r.Entry.File,
			Icon:        getLevelIcon(r.Entry.Level),
			Category:    r.Entry.Source,
			MatchType:   r.MatchField,
			MatchField:  r.MatchField,
		}

		if req.IncludeRaw {
			result.RawData = r.Entry
		}

		results = append(results, result)
	}

	return results
}

// getMethodIcon 根据HTTP方法获取图标.
func getMethodIcon(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "arrow-down"
	case "POST":
		return "plus"
	case "PUT":
		return "edit"
	case "DELETE":
		return "trash"
	case "PATCH":
		return "patch"
	default:
		return "code"
	}
}

// getLevelIcon 根据日志级别获取图标.
func getLevelIcon(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "bug"
	case LogLevelInfo:
		return "info-circle"
	case LogLevelWarning:
		return "exclamation-triangle"
	case LogLevelError:
		return "times-circle"
	case LogLevelFatal:
		return "skull-crossbones"
	default:
		return "file-alt"
	}
}

// GetSearchCategories 获取搜索分类（更新版）.
func (s *GlobalSearchService) GetSearchCategories() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type":  "file",
			"name":  "文件",
			"icon":  "file",
			"count": 0,
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
		{
			"type":  "metadata",
			"name":  "元数据",
			"icon":  "tags",
			"count": 0,
		},
		{
			"type":  "api",
			"name":  "API端点",
			"icon":  "code",
			"count": 0,
		},
		{
			"type":  "doc",
			"name":  "文档",
			"icon":  "book",
			"count": 0,
		},
		{
			"type":  "log",
			"name":  "日志",
			"icon":  "file-alt",
			"count": 0,
		},
	}
}

// RegisterAPI 注册API端点（供外部调用）.
func (s *GlobalSearchService) RegisterAPI(endpoints ...APIEndpoint) {
	if s.apiRegistry != nil {
		s.apiRegistry.Register(endpoints...)
	}
}

// RegisterDoc 注册文档（供外部调用）.
func (s *GlobalSearchService) RegisterDoc(docs ...DocumentItem) {
	if s.docRegistry != nil {
		s.docRegistry.Register(docs...)
	}
}

// GetAPIRegistry 获取API注册表.
func (s *GlobalSearchService) GetAPIRegistry() *APIRegistry {
	return s.apiRegistry
}

// GetDocRegistry 获取文档注册表.
func (s *GlobalSearchService) GetDocRegistry() *DocRegistry {
	return s.docRegistry
}

// GetLogRegistry 获取日志注册表.
func (s *GlobalSearchService) GetLogRegistry() *LogRegistry {
	return s.logRegistry
}
