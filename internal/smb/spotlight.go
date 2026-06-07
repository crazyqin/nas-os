// Package smb SMB Spotlight Search 模块
// 对标 TrueNAS 26 SMB Spotlight - macOS Spotlight 集成
//
// 功能特性：
//   - macOS Spotlight 可直接搜索 SMB 共享内容
//   - 支持文件名、内容、类型等多维度搜索
//   - 每共享可独立配置 Spotlight 启用状态
//   - 完整的 kMDItem* 属性映射
//   - 中文分词支持
//   - 内容全文索引
//
// 架构设计：
//
//	spotlight.go          - RPC 接口定义（本文件）
//	spotlight_integration.go - 核心实现（索引器、搜索器）
//	spotlight_api.go      - HTTP API handlers
//
// RPC 接口：
//
//	SpotlightService.Search(query) -> results
//	SpotlightService.GetIndexStatus() -> status
//	SpotlightService.EnableForShare(shareName) -> void
//	SpotlightService.DisableForShare(shareName) -> void
//	SpotlightService.RebuildIndex(path) -> void
package smb

import (
	"context"
	"time"
)

// ================== RPC 接口定义 ==================

// SpotlightServiceRPC Spotlight RPC 服务接口
// 定义所有 Spotlight 相关的 RPC 方法
type SpotlightServiceRPC interface {
	// Search 执行 Spotlight 搜索
	// 参数：
	//   - query: Spotlight 查询语法 (kMDItemDisplayName == "xxx")
	//   - scope: 搜索范围路径列表
	//   - limit: 结果数量限制
	// 返回：搜索结果列表
	Search(ctx context.Context, req *SpotlightSearchRequest) (*SpotlightSearchResponse, error)

	// GetIndexStatus 获取索引状态
	// 返回：当前索引统计信息
	GetIndexStatus(ctx context.Context) (*SpotlightIndexStatus, error)

	// EnableForShare 为指定共享启用 Spotlight
	// 参数：shareName - SMB 共享名称
	EnableForShare(ctx context.Context, shareName string) error

	// DisableForShare 为指定共享禁用 Spotlight
	// 参数：shareName - SMB 共享名称
	DisableForShare(ctx context.Context, shareName string) error

	// RebuildIndex 重建指定路径的索引
	// 参数：path - 需要重建索引的路径
	RebuildIndex(ctx context.Context, path string) error

	// GetShareSpotlightConfig 获取共享的 Spotlight 配置
	// 参数：shareName - SMB 共享名称
	GetShareSpotlightConfig(ctx context.Context, shareName string) (*ShareSpotlightConfig, error)

	// UpdateShareSpotlightConfig 更新共享的 Spotlight 配置
	// 参数：shareName + config
	UpdateShareSpotlightConfig(ctx context.Context, shareName string, config *ShareSpotlightConfig) error

	// ListSpotlightShares 列出所有启用了 Spotlight 的共享
	ListSpotlightShares(ctx context.Context) ([]*SpotlightShareInfo, error)

	// ClearIndex 清除指定路径的索引
	ClearIndex(ctx context.Context, path string) error

	// PauseIndexing 暂停索引任务
	PauseIndexing(ctx context.Context) error

	// ResumeIndexing 恢复索引任务
	ResumeIndexing(ctx context.Context) error
}

// ================== RPC 请求/响应类型 ==================

// SpotlightSearchRequest Spotlight 搜索请求
type SpotlightSearchRequest struct {
	// Query Spotlight 查询语法
	// 支持格式：
	//   - 简单关键词: "keyword"
	//   - 属性查询: kMDItemDisplayName == "filename"
	//   - 类型过滤: kMDItemContentType == "public.image"
	//   - 组合查询: (kMDItemDisplayName == "doc") AND (kMDItemContentType == "public.plain-text")
	Query string `json:"query"`

	// Scope 搜索范围路径
	// 指定搜索的 SMB 共享路径，可多个
	Scope []string `json:"scope"`

	// Attributes 请求返回的 Spotlight 属性
	// 如: kMDItemDisplayName, kMDItemFSSize, kMDItemContentType
	Attributes []string `json:"attributes"`

	// Limit 结果数量限制
	// 默认 100，最大 1000
	Limit int `json:"limit"`

	// Offset 结果偏移（分页）
	Offset int `json:"offset"`

	// SortBy 排序字段
	// 可选: score, name, size, modified, type
	SortBy string `json:"sortBy"`

	// SortDesc 降序排序
	SortDesc bool `json:"sortDesc"`

	// OnlyFiles 仅返回文件（排除目录）
	OnlyFiles bool `json:"onlyFiles"`

	// OnlyDirs 仅返回目录
	OnlyDirs bool `json:"onlyDirs"`

	// FuzzyMatch 启用模糊匹配
	FuzzyMatch bool `json:"fuzzyMatch"`

	// ContentSearch 内容全文搜索
	ContentSearch bool `json:"contentSearch"`

	// FileType 文件类型过滤
	// 如: image, video, audio, document, archive
	FileType string `json:"fileType"`

	// MinSize 最小文件大小（字节）
	MinSize int64 `json:"minSize"`

	// MaxSize 最大文件大小（字节）
	MaxSize int64 `json:"maxSize"`

	// ModifiedAfter 修改时间筛选（之后）
	ModifiedAfter *time.Time `json:"modifiedAfter,omitempty"`

	// ModifiedBefore 修改时间筛选（之前）
	ModifiedBefore *time.Time `json:"modifiedBefore,omitempty"`

	// Extensions 扩展名过滤
	// 如: .pdf, .doc, .jpg
	Extensions []string `json:"extensions"`
}

// SpotlightSearchResponse Spotlight 搜索响应
type SpotlightSearchResponse struct {
	// Query 原始查询
	Query string `json:"query"`

	// Results 搜索结果列表
	Results []*SpotlightFileResult `json:"results"`

	// Total 总结果数（不受 limit 影响）
	Total int `json:"total"`

	// Took 查询耗时（毫秒）
	Took int64 `json:"took"`

	// Scope 实际搜索范围
	Scope []string `json:"scope"`

	// Attributes 返回的属性列表
	Attributes []string `json:"attributes"`

	// HasMore 是否有更多结果
	HasMore bool `json:"hasMore"`

	// Error 错误信息（如有）
	Error string `json:"error,omitempty"`
}

// SpotlightFileResult Spotlight 文件搜索结果
// 对应 macOS Spotlight 的 kMDItem 属性
type SpotlightFileResult struct {
	// Path 文件完整路径
	Path string `json:"path"`

	// RelativePath 相对于搜索范围的路径
	RelativePath string `json:"relativePath"`

	// Name 文件名
	Name string `json:"name"`

	// Size 文件大小（字节）
	Size int64 `json:"size"`

	// ModTime 修改时间
	ModTime time.Time `json:"modTime"`

	// Type kMDItemContentType
	// 如: public.jpeg, com.adobe.pdf, public.plain-text
	Type string `json:"type"`

	// Kind kMDItemKind（本地化类型描述）
	// 如: "JPEG图像", "PDF文档"
	Kind string `json:"kind"`

	// Extension 文件扩展名
	Extension string `json:"extension"`

	// IsDirectory 是否为目录
	IsDirectory bool `json:"isDirectory"`

	// Score 搜索相关性评分 (0-100)
	Score float64 `json:"score"`

	// Snippet 内容摘要（内容搜索时）
	Snippet string `json:"snippet,omitempty"`

	// Attributes kMDItem 属性映射
	// 如: kMDItemDisplayName -> 文件名
	Attributes map[string]string `json:"attributes"`

	// Thumbnail 缩略图路径（图像/视频）
	Thumbnail string `json:"thumbnail,omitempty"`

	// Width 图像/视频宽度
	Width int `json:"width,omitempty"`

	// Height 图像/视频高度
	Height int `json:"height,omitempty"`

	// Duration 视频时长（秒）
	Duration float64 `json:"duration,omitempty"`

	// Author 文件作者（文档类型）
	Author string `json:"author,omitempty"`

	// Title 文件标题（文档类型）
	Title string `json:"title,omitempty"`
}

// SpotlightIndexStatus Spotlight 索引状态
type SpotlightIndexStatus struct {
	// Enabled Spotlight 是否启用
	Enabled bool `json:"enabled"`

	// Status 索引器状态
	// 可选: idle, building, ready, error, paused
	Status string `json:"status"`

	// TotalFiles 总文件数
	TotalFiles int64 `json:"totalFiles"`

	// IndexedFiles 已索引文件数
	IndexedFiles int64 `json:"indexedFiles"`

	// IndexedSize 已索引总大小（字节）
	IndexedSize int64 `json:"indexedSize"`

	// Progress 索引进度 (0-100)
	Progress float64 `json:"progress"`

	// LastUpdate 上次更新时间
	LastUpdate time.Time `json:"lastUpdate"`

	// SharePaths 索引的共享路径
	SharePaths []string `json:"sharePaths"`

	// ContentIndexed 是否启用内容索引
	ContentIndexed bool `json:"contentIndexed"`

	// EstimatedTimeRemaining 预估剩余时间（秒）
	EstimatedTimeRemaining int64 `json:"estimatedTimeRemaining,omitempty"`

	// Error 错误信息
	Error string `json:"error,omitempty"`
}

// ShareSpotlightConfig 共享 Spotlight 配置
type ShareSpotlightConfig struct {
	// ShareName SMB 共享名称
	ShareName string `json:"shareName"`

	// Enabled 是否启用 Spotlight
	Enabled bool `json:"enabled"`

	// IndexPath 索引路径
	IndexPath string `json:"indexPath"`

	// ContentIndex 启用内容索引
	ContentIndex bool `json:"contentIndex"`

	// ChineseSegment 启用中文分词
	ChineseSegment bool `json:"chineseSegment"`

	// ExcludedPaths 排除索引的路径
	ExcludedPaths []string `json:"excludedPaths"`

	// MaxIndexSizeMB 最大索引大小（MB）
	MaxIndexSizeMB int64 `json:"maxIndexSizeMB"`

	// UpdateInterval 更新间隔（秒）
	UpdateInterval int `json:"updateInterval"`

	// CacheSize 搜索缓存大小
	CacheSize int `json:"cacheSize"`
}

// SpotlightShareInfo Spotlight 共享信息
type SpotlightShareInfo struct {
	// ShareName SMB 共享名称
	ShareName string `json:"shareName"`

	// Path 共享路径
	Path string `json:"path"`

	// Enabled Spotlight 是否启用
	Enabled bool `json:"enabled"`

	// IndexedFiles 已索引文件数
	IndexedFiles int64 `json:"indexedFiles"`

	// IndexedSize 已索引大小
	IndexedSize int64 `json:"indexedSize"`

	// Status 索引状态
	Status string `json:"status"`

	// LastIndexed 上次索引时间
	LastIndexed time.Time `json:"lastIndexed"`
}

// ================== Spotlight 服务实现骨架 ==================

// SpotlightService Spotlight 服务
// 实现 SpotlightServiceRPC 接口
type SpotlightService struct {
	rpc         SpotlightServiceRPC
	integration *SpotlightIntegration
	manager     *Manager
}

// NewSpotlightService 创建 Spotlight 服务
func NewSpotlightService(integration *SpotlightIntegration, manager *Manager) *SpotlightService {
	return &SpotlightService{
		integration: integration,
		manager:     manager,
	}
}

// Search 执行 Spotlight 搜索
func (s *SpotlightService) Search(ctx context.Context, req *SpotlightSearchRequest) (*SpotlightSearchResponse, error) {
	// 转换为内部查询格式
	query := SpotlightQuery{
		Query:         req.Query,
		Attributes:    req.Attributes,
		Scope:         req.Scope,
		Limit:         req.Limit,
		SortBy:        req.SortBy,
		SortDesc:      req.SortDesc,
		OnlyFiles:     req.OnlyFiles,
		OnlyDirs:      req.OnlyDirs,
		FuzzyMatch:    req.FuzzyMatch,
		ContentSearch: req.ContentSearch,
	}

	// 调用集成层搜索
	resp, err := s.integration.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	// 转换为 RPC 响应格式
	results := make([]*SpotlightFileResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = &SpotlightFileResult{
			Path:        r.Path,
			Name:        r.Name,
			Size:        r.Size,
			ModTime:     r.ModTime,
			Type:        r.Type,
			Kind:        r.Kind,
			Extension:   filepathExt(r.Path),
			IsDirectory: false,
			Score:       r.Score,
			Snippet:     r.Snippet,
			Attributes:  r.Attributes,
		}
	}

	return &SpotlightSearchResponse{
		Query:      resp.Query,
		Results:    results,
		Total:      resp.Total,
		Took:       resp.Took,
		Scope:      resp.Scope,
		Attributes: resp.Attributes,
		HasMore:    resp.Total > req.Limit,
	}, nil
}

// GetIndexStatus 获取索引状态
func (s *SpotlightService) GetIndexStatus(ctx context.Context) (*SpotlightIndexStatus, error) {
	stats := s.integration.GetIndexStatus()

	return &SpotlightIndexStatus{
		Enabled:        s.integration.config.Enabled,
		Status:         stats.Status,
		TotalFiles:     stats.TotalFiles,
		IndexedFiles:   stats.IndexedFiles,
		IndexedSize:    stats.IndexedSize,
		Progress:       stats.Progress,
		LastUpdate:     stats.LastUpdate,
		SharePaths:     s.integration.config.SharePaths,
		ContentIndexed: s.integration.config.EnableContentIdx,
	}, nil
}

// EnableForShare 为共享启用 Spotlight
func (s *SpotlightService) EnableForShare(ctx context.Context, shareName string) error {
	share, err := s.manager.GetShare(shareName)
	if err != nil {
		return err
	}

	return s.integration.EnableForShare(share.Path)
}

// DisableForShare 为共享禁用 Spotlight
func (s *SpotlightService) DisableForShare(ctx context.Context, shareName string) error {
	share, err := s.manager.GetShare(shareName)
	if err != nil {
		return err
	}

	return s.integration.DisableForShare(share.Path)
}

// RebuildIndex 重建索引
func (s *SpotlightService) RebuildIndex(ctx context.Context, path string) error {
	return s.integration.RebuildIndex(ctx, path)
}

// GetShareSpotlightConfig 获取共享 Spotlight 配置
func (s *SpotlightService) GetShareSpotlightConfig(ctx context.Context, shareName string) (*ShareSpotlightConfig, error) {
	share, err := s.manager.GetShare(shareName)
	if err != nil {
		return nil, err
	}

	// 检查是否在 Spotlight 路径列表中
	enabled := contains(s.integration.config.SharePaths, share.Path)

	return &ShareSpotlightConfig{
		ShareName:      shareName,
		Enabled:        enabled,
		IndexPath:      share.Path,
		ContentIndex:   s.integration.config.EnableContentIdx,
		ChineseSegment: s.integration.config.EnableChineseSeg,
		ExcludedPaths:  s.integration.config.ExcludedPaths,
		MaxIndexSizeMB: s.integration.config.MaxIndexSize,
		UpdateInterval: s.integration.config.UpdateInterval,
		CacheSize:      s.integration.config.CacheSize,
	}, nil
}

// UpdateShareSpotlightConfig 更新共享 Spotlight 配置
func (s *SpotlightService) UpdateShareSpotlightConfig(ctx context.Context, shareName string, config *ShareSpotlightConfig) error {
	if config.Enabled {
		return s.EnableForShare(ctx, shareName)
	}
	return s.DisableForShare(ctx, shareName)
}

// ListSpotlightShares 列出 Spotlight 共享
func (s *SpotlightService) ListSpotlightShares(ctx context.Context) ([]*SpotlightShareInfo, error) {
	shares, err := s.manager.ListShares()
	if err != nil {
		return nil, err
	}

	result := make([]*SpotlightShareInfo, 0)
	stats := s.integration.GetIndexStatus()

	for _, share := range shares {
		enabled := contains(s.integration.config.SharePaths, share.Path)

		info := &SpotlightShareInfo{
			ShareName:   share.Name,
			Path:        share.Path,
			Enabled:     enabled,
			Status:      stats.Status,
			LastIndexed: stats.LastUpdate,
		}

		if enabled {
			// 计算该共享的索引文件数（简化）
			info.IndexedFiles = stats.IndexedFiles
			info.IndexedSize = stats.IndexedSize
		}

		result = append(result, info)
	}

	return result, nil
}

// ClearIndex 清除索引
func (s *SpotlightService) ClearIndex(ctx context.Context, path string) error {
	s.integration.indexer.ClearIndex(path)
	return nil
}

// PauseIndexing 暂停索引
func (s *SpotlightService) PauseIndexing(ctx context.Context) error {
	s.integration.mu.Lock()
	s.integration.indexer.mu.Lock()
	s.integration.indexer.running = false
	s.integration.indexer.mu.Unlock()
	s.integration.mu.Unlock()
	return nil
}

// ResumeIndexing 恢复索引
func (s *SpotlightService) ResumeIndexing(ctx context.Context) error {
	s.integration.mu.Lock()
	s.integration.indexer.mu.Lock()
	s.integration.indexer.running = true
	s.integration.indexer.mu.Unlock()
	s.integration.mu.Unlock()
	return nil
}

// ================== 辅助函数 ==================

// filepathExt 获取文件扩展名
func filepathExt(path string) string {
	if path == "" {
		return ""
	}
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}
