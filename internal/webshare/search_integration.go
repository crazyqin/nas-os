// Package webshare 搜索集成模块
// 将 WebShare 内容搜索与全局搜索模块集成
// 参考: TrueNAS TrueSearch 全局搜索能力
package webshare

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WebShareSearchIntegration WebShare 搜索集成
// 提供与全局搜索模块的统一接口
type WebShareSearchIntegration struct {
	contentSearch *ContentSearchService
	searchIndex   *SearchIndex
	manager       *Manager
	logger        *zap.Logger
	mu            sync.RWMutex
	running       bool
	ctx           context.Context
	cancel        context.CancelFunc
}

// IntegrationConfig 集成配置
type IntegrationConfig struct {
	EnableContentSearch bool          `json:"enableContentSearch"` // 启用内容搜索
	EnableFileNameIndex bool          `json:"enableFileNameIndex"` // 启用文件名索引
	IndexInterval       time.Duration `json:"indexInterval"`       // 索引刷新间隔
	MaxConcurrentSearch int           `json:"maxConcurrentSearch"` // 最大并发搜索
}

// DefaultIntegrationConfig 默认配置
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		EnableContentSearch: true,
		EnableFileNameIndex: true,
		IndexInterval:       5 * time.Minute,
		MaxConcurrentSearch: 10,
	}
}

// NewWebShareSearchIntegration 创建搜索集成
func NewWebShareSearchIntegration(
	manager *Manager,
	config WebShareConfig,
	logger *zap.Logger,
) *WebShareSearchIntegration {
	ctx, cancel := context.WithCancel(context.Background())

	integration := &WebShareSearchIntegration{
		contentSearch: NewContentSearchService(config, logger),
		searchIndex:   NewSearchIndex(config),
		manager:       manager,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}

	return integration
}

// Start 启动集成服务
func (wsi *WebShareSearchIntegration) Start() error {
	wsi.mu.Lock()
	wsi.running = true
	wsi.mu.Unlock()

	// 启动内容搜索服务
	wsi.contentSearch.Start()

	// 构建初始索引
	if err := wsi.BuildIndex(); err != nil {
		wsi.logger.Warn("初始索引构建失败", zap.Error(err))
	}

	// 启动后台索引刷新
	go wsi.backgroundRefresh()

	wsi.logger.Info("WebShare搜索集成启动")
	return nil
}

// Stop 停止集成服务
func (wsi *WebShareSearchIntegration) Stop() {
	wsi.cancel()
	wsi.contentSearch.Stop()
	wsi.mu.Lock()
	wsi.running = false
	wsi.mu.Unlock()
	wsi.logger.Info("WebShare搜索集成停止")
}

// backgroundRefresh 后台索引刷新
func (wsi *WebShareSearchIntegration) backgroundRefresh() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-wsi.ctx.Done():
			return
		case <-ticker.C:
			wsi.RefreshIndex()
		}
	}
}

// BuildIndex 构建完整索引
func (wsi *WebShareSearchIntegration) BuildIndex() error {
	basePath := wsi.manager.config.BaseDir
	if basePath == "" {
		basePath = "/"
	}

	wsi.logger.Info("开始构建搜索索引", zap.String("path", basePath))

	// 构建文件名索引
	if err := wsi.searchIndex.BuildIndex(basePath); err != nil {
		wsi.logger.Error("文件名索引构建失败", zap.Error(err))
		return err
	}

	// 构建内容索引
	if err := wsi.contentSearch.BuildIndex(basePath); err != nil {
		wsi.logger.Error("内容索引构建失败", zap.Error(err))
		return err
	}

	wsi.logger.Info("搜索索引构建完成")
	return nil
}

// RefreshIndex 刷新索引
func (wsi *WebShareSearchIntegration) RefreshIndex() {
	wsi.searchIndex.ClearIndex()
	wsi.contentSearch.refreshIndex()

	basePath := wsi.manager.config.BaseDir
	if basePath == "" {
		basePath = "/"
	}

	if err := wsi.searchIndex.BuildIndex(basePath); err != nil {
		wsi.logger.Warn("索引刷新失败", zap.Error(err))
	}
}

// UnifiedSearchRequest 统一搜索请求
type UnifiedSearchRequest struct {
	Query       string    `json:"query"`       // 搜索关键词
	Paths       []string  `json:"paths"`       // 搜索路径限制
	Extensions  []string  `json:"extensions"`  // 文件扩展名过滤
	FileType    string    `json:"fileType"`    // 文件类型 (image, video, audio, document, code, archive)
	MinSize     int64     `json:"minSize"`     // 最小文件大小
	MaxSize     int64     `json:"maxSize"`     // 最大文件大小
	FromDate    *time.Time `json:"fromDate"`   // 修改时间起始
	ToDate      *time.Time `json:"toDate"`     // 修改时间结束
	Content     bool      `json:"content"`     // 是否搜索内容
	MaxResults  int       `json:"maxResults"`  // 最大结果数
	Fuzzy       bool      `json:"fuzzy"`       // 模糊搜索
	Highlight   bool      `json:"highlight"`   // 高亮匹配
	ExactMatch  bool      `json:"exactMatch"`  // 精确匹配
	CaseSense   bool      `json:"caseSense"`   // 大小写敏感
	WithContext bool      `json:"withContext"` // 返回上下文
}

// UnifiedSearchResult 统一搜索结果
type UnifiedSearchResult struct {
	Path        string         `json:"path"`
	Name        string         `json:"name"`
	Ext         string         `json:"ext"`
	Type        string         `json:"type"`
	Size        int64          `json:"size"`
	ModTime     time.Time      `json:"modTime"`
	IsDir       bool           `json:"isDir"`
	Score       float64        `json:"score"`
	MatchCount  int            `json:"matchCount"`
	Excerpt     string         `json:"excerpt,omitempty"`
	Highlights  []Highlight    `json:"highlights,omitempty"`
	Contexts    []MatchContext `json:"contexts,omitempty"`
	ContentType string         `json:"contentType,omitempty"`
	Thumbnail   string         `json:"thumbnail,omitempty"`
}

// UnifiedSearchResponse 统一搜索响应
type UnifiedSearchResponse struct {
	Query       string              `json:"query"`
	Took        time.Duration       `json:"took"`
	Total       int                 `json:"total"`
	Results     []UnifiedSearchResult `json:"results"`
	Truncated   bool                `json:"truncated"`
	Suggestions []string            `json:"suggestions"`
	Facets      map[string]int      `json:"facets"`
	Stats       SearchStats         `json:"stats"`
}

// SearchStats 搜索统计
type SearchStats struct {
	FilesScanned   int     `json:"filesScanned"`
	BytesScanned   int64   `json:"bytesScanned"`
	IndexHitRatio  float64 `json:"indexHitRatio"`
	AverageScore   float64 `json:"averageScore"`
	ContentSearch  bool    `json:"contentSearch"`
	NameSearch     bool    `json:"nameSearch"`
}

// Search 执行统一搜索
// 同时搜索文件名和文件内容
func (wsi *WebShareSearchIntegration) Search(ctx context.Context, req UnifiedSearchRequest) (*UnifiedSearchResponse, error) {
	startTime := time.Now()

	response := &UnifiedSearchResponse{
		Query:     req.Query,
		Facets:    make(map[string]int),
		Results:   make([]UnifiedSearchResult, 0),
		Suggestions: make([]string, 0),
	}

	if req.MaxResults == 0 {
		req.MaxResults = 50
	}

	var stats SearchStats
	var allResults []UnifiedSearchResult

	// 1. 文件名搜索
	nameResults, err := wsi.searchIndex.Search(req.Query, "", req.FileType, req.MinSize, req.MaxSize)
	if err == nil {
		stats.NameSearch = true
		for _, item := range nameResults {
			result := UnifiedSearchResult{
				Path:       item.Path,
				Name:       item.Name,
				Ext:        item.Extension,
				Type:       item.Type,
				Size:       item.Size,
				ModTime:    item.ModTime,
				IsDir:      item.IsDir,
				Score:      10.0, // 文件名匹配基础分
				Thumbnail:  item.Thumbnail,
				MatchCount: 1,
			}
			allResults = append(allResults, result)
			stats.FilesScanned++
			response.Facets[item.Type]++
		}
	}

	// 2. 内容搜索（如果启用）
	if req.Content {
		contentReq := ContentSearchRequest{
			Query:       req.Query,
			Paths:       req.Paths,
			Extensions:  req.Extensions,
			MinSize:     req.MinSize,
			MaxSize:     req.MaxSize,
			FromDate:    req.FromDate,
			ToDate:      req.ToDate,
			MaxResults:  req.MaxResults,
			Fuzzy:       req.Fuzzy,
			Highlight:   req.Highlight,
			ExactMatch:  req.ExactMatch,
			CaseSense:   req.CaseSense,
			WithContext: req.WithContext,
			ContextSize: 100,
		}

		contentResp, err := wsi.contentSearch.Search(ctx, contentReq)
		if err == nil {
			stats.ContentSearch = true
			stats.BytesScanned = contentResp.Stats.BytesScanned

			for _, cr := range contentResp.Results {
				// 检查是否已存在于结果中
				found := false
				for i, r := range allResults {
					if r.Path == cr.Path {
						// 合并分数
						allResults[i].Score += cr.Score
						allResults[i].MatchCount += cr.MatchCount
						allResults[i].Excerpt = cr.Excerpt
						allResults[i].Highlights = cr.Highlights
						allResults[i].Contexts = cr.Contexts
						allResults[i].ContentType = cr.ContentType
						found = true
						break
					}
				}

				if !found {
					result := UnifiedSearchResult{
						Path:        cr.Path,
						Name:        cr.Name,
						Ext:         cr.Ext,
						Type:        "file",
						Size:        cr.Size,
						ModTime:     cr.ModTime,
						IsDir:       false,
						Score:       cr.Score,
						MatchCount:  cr.MatchCount,
						Excerpt:     cr.Excerpt,
						Highlights:  cr.Highlights,
						Contexts:    cr.Contexts,
						ContentType: cr.ContentType,
					}
					allResults = append(allResults, result)
					stats.FilesScanned++
					response.Facets[cr.ContentType]++
				}
			}

			// 合并建议
			for _, s := range contentResp.Suggestions {
				response.Suggestions = append(response.Suggestions, s)
			}
		}
	}

	// 3. 排序（按分数降序）
	wsi.sortResults(allResults)

	// 4. 截断
	if len(allResults) > req.MaxResults {
		allResults = allResults[:req.MaxResults]
		response.Truncated = true
	}

	response.Results = allResults
	response.Total = len(allResults)
	response.Took = time.Since(startTime)
	response.Stats = stats

	// 计算平均分数
	if len(allResults) > 0 {
		var totalScore float64
		for _, r := range allResults {
			totalScore += r.Score
		}
		stats.AverageScore = totalScore / float64(len(allResults))
	}

	return response, nil
}

// sortResults 排序结果
func (wsi *WebShareSearchIntegration) sortResults(results []UnifiedSearchResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// QuickSearch 快速搜索
// 仅搜索文件名，适合即时响应
func (wsi *WebShareSearchIntegration) QuickSearch(query string, path string, limit int) ([]UnifiedSearchResult, error) {
	if limit == 0 {
		limit = 20
	}

	results := make([]UnifiedSearchResult, 0)

	items, err := wsi.searchIndex.Search(query, path, "", 0, 0)
	if err != nil {
		return results, err
	}

	for i, item := range items {
		if i >= limit {
			break
		}
		result := UnifiedSearchResult{
			Path:      item.Path,
			Name:      item.Name,
			Ext:       item.Extension,
			Type:      item.Type,
			Size:      item.Size,
			ModTime:   item.ModTime,
			IsDir:     item.IsDir,
			Score:     10.0,
			Thumbnail: item.Thumbnail,
		}
		results = append(results, result)
	}

	return results, nil
}

// GetIndexStats 获取索引统计
func (wsi *WebShareSearchIntegration) GetIndexStats() map[string]interface{} {
	nameStats := wsi.searchIndex.GetStats()
	contentStats := wsi.contentSearch.GetIndexStats()

	return map[string]interface{}{
		"fileNameIndex": nameStats,
		"contentIndex":  contentStats,
		"running":       wsi.running,
	}
}

// UpdateFileIndex 更新单个文件索引
func (wsi *WebShareSearchIntegration) UpdateFileIndex(path string) error {
	// 更新文件名索引
	if err := wsi.searchIndex.UpdateIndex(path); err != nil {
		return err
	}

	// 内容索引会在后台刷新中更新
	return nil
}

// RemoveFileIndex 移除文件索引
func (wsi *WebShareSearchIntegration) RemoveFileIndex(path string) {
	wsi.contentSearch.clearIndex(path)
	// searchIndex 的 UpdateIndex 会处理删除情况
	_ = wsi.searchIndex.UpdateIndex(path)
}