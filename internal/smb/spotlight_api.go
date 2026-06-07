// Package smb SMB Spotlight HTTP API
// 提供 Spotlight 搜索的 HTTP REST API 接口
package smb

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SpotlightAPIHandler Spotlight API 处理器
type SpotlightAPIHandler struct {
	service *SpotlightService
	logger  *zap.Logger
}

// NewSpotlightAPIHandler 创建 Spotlight API 处理器
func NewSpotlightAPIHandler(service *SpotlightService, logger *zap.Logger) *SpotlightAPIHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SpotlightAPIHandler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes 注册 Spotlight API 路由
func (h *SpotlightAPIHandler) RegisterRoutes(api *gin.RouterGroup) {
	spotlight := api.Group("/spotlight")
	{
		// 搜索接口
		spotlight.POST("/search", h.Search)
		spotlight.GET("/search", h.SearchGet)

		// 索引状态
		spotlight.GET("/status", h.GetStatus)
		spotlight.GET("/stats", h.GetStats)

		// 索引管理
		spotlight.POST("/rebuild", h.RebuildIndex)
		spotlight.POST("/clear", h.ClearIndex)
		spotlight.POST("/pause", h.PauseIndexing)
		spotlight.POST("/resume", h.ResumeIndexing)

		// 共享配置
		spotlight.GET("/shares", h.ListSpotlightShares)
		spotlight.GET("/shares/:name", h.GetShareConfig)
		spotlight.PUT("/shares/:name", h.UpdateShareConfig)
		spotlight.POST("/shares/:name/enable", h.EnableForShare)
		spotlight.POST("/shares/:name/disable", h.DisableForShare)

		// 配置管理
		spotlight.GET("/config", h.GetConfig)
		spotlight.PUT("/config", h.UpdateConfig)
	}
}

// ================== 搜索接口 ==================

// Search POST 搜索接口
// Body: SpotlightSearchRequest JSON
func (h *SpotlightAPIHandler) Search(c *gin.Context) {
	var req SpotlightSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求格式: " + err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	resp, err := h.service.Search(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Spotlight搜索失败", zap.Error(err), zap.String("query", req.Query))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SearchGet GET 搜索接口
// Query: q, scope, limit, offset, sort, type
func (h *SpotlightAPIHandler) SearchGet(c *gin.Context) {
	req := &SpotlightSearchRequest{
		Query:         c.Query("q"),
		Scope:         c.QueryArray("scope"),
		Attributes:    c.QueryArray("attr"),
		SortBy:        c.Query("sort"),
		FileType:      c.Query("type"),
		FuzzyMatch:    c.Query("fuzzy") == "true",
		ContentSearch: c.Query("content") == "true",
		Extensions:    c.QueryArray("ext"),
	}

	// 解析数值参数
	if limit, err := strconv.Atoi(c.Query("limit")); err == nil && limit > 0 {
		req.Limit = limit
	} else {
		req.Limit = 100
	}

	if offset, err := strconv.Atoi(c.Query("offset")); err == nil && offset >= 0 {
		req.Offset = offset
	}

	if minSize, err := strconv.ParseInt(c.Query("minSize"), 10, 64); err == nil {
		req.MinSize = minSize
	}

	if maxSize, err := strconv.ParseInt(c.Query("maxSize"), 10, 64); err == nil {
		req.MaxSize = maxSize
	}

	req.SortDesc = c.Query("order") == "desc"
	req.OnlyFiles = c.Query("onlyFiles") == "true"
	req.OnlyDirs = c.Query("onlyDirs") == "true"

	// 解析时间参数
	if after := c.Query("after"); after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err == nil {
			req.ModifiedAfter = &t
		}
	}

	if before := c.Query("before"); before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err == nil {
			req.ModifiedBefore = &t
		}
	}

	resp, err := h.service.Search(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ================== 状态接口 ==================

// GetStatus 获取索引状态
func (h *SpotlightAPIHandler) GetStatus(c *gin.Context) {
	status, err := h.service.GetIndexStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetStats 获取详细统计（兼容旧接口）
func (h *SpotlightAPIHandler) GetStats(c *gin.Context) {
	status, err := h.service.GetIndexStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 添加更多统计信息
	stats := gin.H{
		"enabled":        status.Enabled,
		"status":         status.Status,
		"totalFiles":     status.TotalFiles,
		"indexedFiles":   status.IndexedFiles,
		"indexedSize":    status.IndexedSize,
		"progress":       status.Progress,
		"lastUpdate":     status.LastUpdate,
		"sharePaths":     status.SharePaths,
		"contentIndexed": status.ContentIndexed,
		"performance": gin.H{
			"cacheSize":           h.service.integration.config.CacheSize,
			"cacheTTLSeconds":     h.service.integration.config.CacheTTLSeconds,
			"maxConcurrentSearch": h.service.integration.config.MaxConcurrentSearch,
			"indexBatchSize":      h.service.integration.config.IndexBatchSize,
		},
	}

	c.JSON(http.StatusOK, stats)
}

// ================== 索引管理接口 ==================

// RebuildIndex 重建索引
// Body: { "path": "/data/share" }
func (h *SpotlightAPIHandler) RebuildIndex(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 path 参数"})
		return
	}

	err := h.service.RebuildIndex(c.Request.Context(), req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Spotlight索引重建请求", zap.String("path", req.Path))
	c.JSON(http.StatusOK, gin.H{
		"message": "索引重建已启动",
		"path":    req.Path,
	})
}

// ClearIndex 清除索引
// Body: { "path": "/data/share" }
func (h *SpotlightAPIHandler) ClearIndex(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 path 参数"})
		return
	}

	err := h.service.ClearIndex(c.Request.Context(), req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Spotlight索引清除", zap.String("path", req.Path))
	c.JSON(http.StatusOK, gin.H{
		"message": "索引已清除",
		"path":    req.Path,
	})
}

// PauseIndexing 暂停索引
func (h *SpotlightAPIHandler) PauseIndexing(c *gin.Context) {
	err := h.service.PauseIndexing(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "索引已暂停"})
}

// ResumeIndexing 恢复索引
func (h *SpotlightAPIHandler) ResumeIndexing(c *gin.Context) {
	err := h.service.ResumeIndexing(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "索引已恢复"})
}

// ================== 共享配置接口 ==================

// ListSpotlightShares 列出所有共享的 Spotlight 状态
func (h *SpotlightAPIHandler) ListSpotlightShares(c *gin.Context) {
	shares, err := h.service.ListSpotlightShares(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"shares": shares,
		"total":  len(shares),
	})
}

// GetShareConfig 获取指定共享的 Spotlight 配置
func (h *SpotlightAPIHandler) GetShareConfig(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少共享名称"})
		return
	}

	config, err := h.service.GetShareSpotlightConfig(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateShareConfig 更新共享 Spotlight 配置
// Body: ShareSpotlightConfig JSON
func (h *SpotlightAPIHandler) UpdateShareConfig(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少共享名称"})
		return
	}

	var config ShareSpotlightConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置格式"})
		return
	}

	err := h.service.UpdateShareSpotlightConfig(c.Request.Context(), name, &config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Spotlight共享配置更新",
		zap.String("share", name),
		zap.Bool("enabled", config.Enabled))

	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
		"share":   name,
	})
}

// EnableForShare 启用共享的 Spotlight
func (h *SpotlightAPIHandler) EnableForShare(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少共享名称"})
		return
	}

	err := h.service.EnableForShare(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("为共享启用Spotlight", zap.String("share", name))
	c.JSON(http.StatusOK, gin.H{
		"message": "Spotlight已启用",
		"share":   name,
	})
}

// DisableForShare 禁用共享的 Spotlight
func (h *SpotlightAPIHandler) DisableForShare(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少共享名称"})
		return
	}

	err := h.service.DisableForShare(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("为共享禁用Spotlight", zap.String("share", name))
	c.JSON(http.StatusOK, gin.H{
		"message": "Spotlight已禁用",
		"share":   name,
	})
}

// ================== 配置接口 ==================

// SpotlightGlobalConfig 全局配置响应
type SpotlightGlobalConfig struct {
	Enabled             bool     `json:"enabled"`
	SharePaths          []string `json:"sharePaths"`
	ExcludedPaths       []string `json:"excludedPaths"`
	MaxIndexSizeMB      int64    `json:"maxIndexSizeMB"`
	UpdateInterval      int      `json:"updateInterval"`
	EnableContentIdx    bool     `json:"enableContentIdx"`
	EnableChineseSeg    bool     `json:"enableChineseSeg"`
	IndexerWorkers      int      `json:"indexerWorkers"`
	CacheSize           int      `json:"cacheSize"`
	CacheTTLSeconds     int      `json:"cacheTTLSeconds"`
	MaxConcurrentSearch int      `json:"maxConcurrentSearch"`
	IndexBatchSize      int      `json:"indexBatchSize"`
	EnableResultCache   bool     `json:"enableResultCache"`
	EnableParallelIndex bool     `json:"enableParallelIndex"`
	FuzzyThreshold      float64  `json:"fuzzyThreshold"`
}

// GetConfig 获取全局 Spotlight 配置
func (h *SpotlightAPIHandler) GetConfig(c *gin.Context) {
	cfg := h.service.integration.config

	config := SpotlightGlobalConfig{
		Enabled:             cfg.Enabled,
		SharePaths:          cfg.SharePaths,
		ExcludedPaths:       cfg.ExcludedPaths,
		MaxIndexSizeMB:      cfg.MaxIndexSize,
		UpdateInterval:      cfg.UpdateInterval,
		EnableContentIdx:    cfg.EnableContentIdx,
		EnableChineseSeg:    cfg.EnableChineseSeg,
		IndexerWorkers:      cfg.IndexerWorkers,
		CacheSize:           cfg.CacheSize,
		CacheTTLSeconds:     cfg.CacheTTLSeconds,
		MaxConcurrentSearch: cfg.MaxConcurrentSearch,
		IndexBatchSize:      cfg.IndexBatchSize,
		EnableResultCache:   cfg.EnableResultCache,
		EnableParallelIndex: cfg.EnableParallelIndex,
		FuzzyThreshold:      cfg.FuzzyThreshold,
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig 更新全局 Spotlight 配置
// Body: SpotlightGlobalConfig JSON
func (h *SpotlightAPIHandler) UpdateConfig(c *gin.Context) {
	var req SpotlightGlobalConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的配置格式"})
		return
	}

	// 更新配置
	integration := h.service.integration
	integration.mu.Lock()
	integration.config.Enabled = req.Enabled
	integration.config.MaxIndexSize = req.MaxIndexSizeMB
	integration.config.UpdateInterval = req.UpdateInterval
	integration.config.EnableContentIdx = req.EnableContentIdx
	integration.config.EnableChineseSeg = req.EnableChineseSeg
	integration.config.IndexerWorkers = req.IndexerWorkers
	integration.config.CacheSize = req.CacheSize
	integration.config.CacheTTLSeconds = req.CacheTTLSeconds
	integration.config.MaxConcurrentSearch = req.MaxConcurrentSearch
	integration.config.IndexBatchSize = req.IndexBatchSize
	integration.config.EnableResultCache = req.EnableResultCache
	integration.config.EnableParallelIndex = req.EnableParallelIndex
	integration.config.FuzzyThreshold = req.FuzzyThreshold
	integration.mu.Unlock()

	h.logger.Info("Spotlight全局配置更新", zap.Bool("enabled", req.Enabled))
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// ================== SMB.conf 配置生成 ==================

// GenerateSMBConfSpotlight 生成 SMB.conf 的 Spotlight 配置段
// 用于应用到 SMB 配置文件
func GenerateSMBConfSpotlight(enabled bool, indexPaths []string, excludedPaths []string) string {
	if !enabled {
		return ""
	}

	config := "    # Spotlight Configuration (TrueNAS 26 Compatible)\n"
	config += "    spotlight = yes\n"
	config += "    spotlight indexing = yes\n"

	if len(indexPaths) > 0 {
		config += "    spotlight indexing paths = "
		for i, path := range indexPaths {
			if i > 0 {
				config += ", "
			}
			config += path
		}
		config += "\n"
	}

	if len(excludedPaths) > 0 {
		config += "    spotlight exclude paths = "
		for i, path := range excludedPaths {
			if i > 0 {
				config += ", "
			}
			config += path
		}
		config += "\n"
	}

	return config
}

// ================== macOS Spotlight 协议兼容 ==================

// SpotlightMDQueryRequest macOS mdfind 请求格式
// 用于支持 macOS Spotlight 原生搜索协议
type SpotlightMDQueryRequest struct {
	// Query mdfind 查询语法
	// 格式: 'kMDItemDisplayName == "*.pdf" && kMDItemFSSize > 1000000'
	Query string `json:"query"`

	// OnlyIn 搜索范围
	OnlyIn string `json:"onlyIn"`

	// Live 实时搜索模式
	Live bool `json:"live"`

	// Interpret 解释模式（更友好的搜索）
	Interpret bool `json:"interpret"`

	// Attributes 返回的属性
	Attributes []string `json:"attributes"`
}

// SpotlightMDQueryResponse macOS mdfind 响应格式
type SpotlightMDQueryResponse struct {
	// Results 文件路径列表（原生 mdfind 格式）
	Results []string `json:"results"`

	// AttributesResults 属性结果（如果请求了属性）
	AttributesResults []map[string]string `json:"attributesResults,omitempty"`

	// QueryID 查询ID（用于 live 模式）
	QueryID string `json:"queryId,omitempty"`

	// Count 结果数量
	Count int `json:"count"`
}

// HandleMDQuery 处理 macOS mdfind 格式请求
// POST /spotlight/mdfind
func (h *SpotlightAPIHandler) HandleMDQuery(c *gin.Context) {
	var req SpotlightMDQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 mdfind 请求"})
		return
	}

	// 转换为内部搜索格式
	internalReq := &SpotlightSearchRequest{
		Query:      req.Query,
		Scope:      []string{req.OnlyIn},
		Attributes: req.Attributes,
		Limit:      1000,
		FuzzyMatch: req.Interpret,
	}

	resp, err := h.service.Search(c.Request.Context(), internalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为 mdfind 格式响应
	mdResp := SpotlightMDQueryResponse{
		Results: make([]string, len(resp.Results)),
		Count:   resp.Total,
	}

	for i, r := range resp.Results {
		mdResp.Results[i] = r.Path
	}

	if len(req.Attributes) > 0 {
		mdResp.AttributesResults = make([]map[string]string, len(resp.Results))
		for i, r := range resp.Results {
			mdResp.AttributesResults[i] = r.Attributes
		}
	}

	c.JSON(http.StatusOK, mdResp)
}
