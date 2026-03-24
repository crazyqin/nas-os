package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"nas-os/internal/search"
)

// SearchHandler 搜索处理器
type SearchHandler struct {
	globalSearch *search.GlobalSearchService
	engine       *search.Engine
	settings     *search.SettingsRegistry
	apps         *search.AppRegistry
	logger       *zap.Logger
}

// NewSearchHandler 创建搜索处理器
func NewSearchHandler(
	globalSearch *search.GlobalSearchService,
	engine *search.Engine,
	settings *search.SettingsRegistry,
	apps *search.AppRegistry,
	logger *zap.Logger,
) *SearchHandler {
	return &SearchHandler{
		globalSearch: globalSearch,
		engine:       engine,
		settings:     settings,
		apps:         apps,
		logger:       logger,
	}
}

// RegisterRoutes 注册路由
func (h *SearchHandler) RegisterRoutes(r *gin.RouterGroup) {
	searchGroup := r.Group("/search")
	{
		// 全局搜索
		searchGroup.POST("/global", h.GlobalSearch)
		searchGroup.GET("/quick", h.QuickSearch)
		searchGroup.GET("/suggestions", h.GetSuggestions)
		searchGroup.GET("/categories", h.GetCategories)
		searchGroup.GET("/popular", h.GetPopularSearches)

		// 文件搜索
		searchGroup.POST("/files", h.SearchFiles)
		searchGroup.POST("/files/index", h.IndexFiles)
		searchGroup.POST("/files/index/dir", h.IndexDirectory)
		searchGroup.DELETE("/files/index", h.DeleteFromIndex)
		searchGroup.GET("/files/stats", h.GetFileStats)

		// 设置搜索
		searchGroup.GET("/settings", h.SearchSettings)
		searchGroup.GET("/settings/categories", h.GetSettingCategories)
		searchGroup.GET("/settings/all", h.GetAllSettings)

		// 应用搜索
		searchGroup.GET("/apps", h.SearchApps)
		searchGroup.GET("/apps/stats", h.GetAppStats)
		searchGroup.GET("/containers", h.SearchContainers)
	}
}

// GlobalSearchRequest 全局搜索请求
type GlobalSearchRequest struct {
	Query      string   `json:"query" binding:"required"`
	Types      []string `json:"types,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	TotalLimit int      `json:"totalLimit,omitempty"`
	MinScore   float64  `json:"minScore,omitempty"`
	IncludeRaw bool     `json:"includeRaw,omitempty"`
}

// GlobalSearch 全局搜索
// @Summary 全局搜索
// @Description 搜索文件、设置、应用和容器
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body GlobalSearchRequest true "搜索请求"
// @Success 200 {object} search.GlobalSearchResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /search/global [post]
func (h *SearchHandler) GlobalSearch(c *gin.Context) {
	var req GlobalSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 转换类型
	var types []search.GlobalSearchResultType
	for _, t := range req.Types {
		types = append(types, search.GlobalSearchResultType(t))
	}

	// 执行搜索
	result, err := h.globalSearch.GlobalSearch(c.Request.Context(), search.GlobalSearchRequest{
		Query:      req.Query,
		Types:      types,
		Limit:      req.Limit,
		TotalLimit: req.TotalLimit,
		MinScore:   req.MinScore,
		IncludeRaw: req.IncludeRaw,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// QuickSearchRequest 快速搜索请求
type QuickSearchRequest struct {
	Query string `form:"query" binding:"required"`
	Limit int    `form:"limit"`
}

// QuickSearch 快速搜索
// @Summary 快速搜索
// @Description 快速搜索用于自动补全
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索查询"
// @Param limit query int false "结果数量限制"
// @Success 200 {object} search.GlobalSearchResponse
// @Failure 400 {object} map[string]interface{}
// @Router /search/quick [get]
func (h *SearchHandler) QuickSearch(c *gin.Context) {
	var req QuickSearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	result, err := h.globalSearch.QuickSearch(c.Request.Context(), req.Query, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GetSuggestions 获取搜索建议
// @Summary 获取搜索建议
// @Description 根据输入获取搜索建议
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索查询"
// @Success 200 {object} map[string]interface{}
// @Router /search/suggestions [get]
func (h *SearchHandler) GetSuggestions(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"suggestions": []string{},
			},
		})
		return
	}

	// 使用引擎的建议功能
	if h.engine != nil {
		result, err := h.engine.GetSuggestions(query)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "success",
				"data":    result,
			})
			return
		}
	}

	// 回退到全局搜索建议
	suggestions := h.globalSearch.GenerateSuggestions(query)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":       query,
			"suggestions": suggestions,
		},
	})
}

// GetCategories 获取搜索分类
// @Summary 获取搜索分类
// @Description 获取可搜索的分类列表
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/categories [get]
func (h *SearchHandler) GetCategories(c *gin.Context) {
	categories := h.globalSearch.GetSearchCategories()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    categories,
	})
}

// GetPopularSearches 获取热门搜索
// @Summary 获取热门搜索
// @Description 获取热门搜索词
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/popular [get]
func (h *SearchHandler) GetPopularSearches(c *gin.Context) {
	popular := h.globalSearch.GetPopularSearches()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"searches": popular,
		},
	})
}

// SearchFiles 文件搜索
// @Summary 文件搜索
// @Description 搜索文件系统
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body search.Request true "搜索请求"
// @Success 200 {object} search.Response
// @Failure 400 {object} map[string]interface{}
// @Router /search/files [post]
func (h *SearchHandler) SearchFiles(c *gin.Context) {
	var req search.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	result, err := h.engine.Search(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// IndexFilesRequest 索引文件请求
type IndexFilesRequest struct {
	Paths []string `json:"paths" binding:"required"`
}

// IndexFiles 索引文件
// @Summary 索引文件
// @Description 将文件添加到搜索索引
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body IndexFilesRequest true "索引请求"
// @Success 200 {object} map[string]interface{}
// @Router /search/files/index [post]
func (h *SearchHandler) IndexFiles(c *gin.Context) {
	var req IndexFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	success := 0
	failed := 0
	errors := []string{}

	for _, path := range req.Paths {
		if err := h.engine.IndexFile(path); err != nil {
			failed++
			errors = append(errors, path+": "+err.Error())
		} else {
			success++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "索引完成",
		"data": gin.H{
			"success": success,
			"failed":  failed,
			"errors":  errors,
		},
	})
}

// IndexDirectoryRequest 索引目录请求
type IndexDirectoryRequest struct {
	Path string `json:"path" binding:"required"`
}

// IndexDirectory 索引目录
// @Summary 索引目录
// @Description 递归索引目录
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body IndexDirectoryRequest true "索引请求"
// @Success 200 {object} map[string]interface{}
// @Router /search/files/index/dir [post]
func (h *SearchHandler) IndexDirectory(c *gin.Context) {
	var req IndexDirectoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 异步索引
	go func() {
		if err := h.engine.IndexDirectory(req.Path); err != nil {
			h.logger.Error("索引目录失败", zap.Error(err), zap.String("path", req.Path))
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "索引任务已启动",
		"data": gin.H{
			"path":      req.Path,
			"startTime": time.Now(),
		},
	})
}

// DeleteFromIndexRequest 删除索引请求
type DeleteFromIndexRequest struct {
	Paths []string `json:"paths" binding:"required"`
}

// DeleteFromIndex 从索引删除
// @Summary 从索引删除
// @Description 从搜索索引中删除文件
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body DeleteFromIndexRequest true "删除请求"
// @Success 200 {object} map[string]interface{}
// @Router /search/files/index [delete]
func (h *SearchHandler) DeleteFromIndex(c *gin.Context) {
	var req DeleteFromIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.engine.DeleteBatch(req.Paths); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
		"data": gin.H{
			"count": len(req.Paths),
		},
	})
}

// GetFileStats 获取文件索引统计
// @Summary 获取文件索引统计
// @Description 获取搜索索引的统计信息
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/files/stats [get]
func (h *SearchHandler) GetFileStats(c *gin.Context) {
	stats := h.engine.Stats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// SearchSettings 搜索设置
// @Summary 搜索设置
// @Description 搜索系统设置项
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索查询"
// @Param limit query int false "结果数量限制"
// @Success 200 {object} map[string]interface{}
// @Router /search/settings [get]
func (h *SearchHandler) SearchSettings(c *gin.Context) {
	query := c.Query("query")
	limit := 10
	if l := c.Query("limit"); l != "" {
		if _, err := time.ParseDuration(l + "s"); err == nil {
			// ignore, just check if it's a number
		}
	}

	results := h.settings.SearchSettings(query, limit)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":   query,
			"total":   len(results),
			"results": results,
		},
	})
}

// GetSettingCategories 获取设置分类
// @Summary 获取设置分类
// @Description 获取所有设置分类
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/settings/categories [get]
func (h *SearchHandler) GetSettingCategories(c *gin.Context) {
	categories := h.settings.GetCategories()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    categories,
	})
}

// GetAllSettings 获取所有设置
// @Summary 获取所有设置
// @Description 获取所有注册的设置项
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/settings/all [get]
func (h *SearchHandler) GetAllSettings(c *gin.Context) {
	settings := h.settings.GetAll()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total": len(settings),
			"items": settings,
		},
	})
}

// SearchApps 搜索应用
// @Summary 搜索应用
// @Description 搜索已安装的应用
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索查询"
// @Param limit query int false "结果数量限制"
// @Success 200 {object} map[string]interface{}
// @Router /search/apps [get]
func (h *SearchHandler) SearchApps(c *gin.Context) {
	query := c.Query("query")
	limit := 10

	results := h.apps.SearchApps(query, limit)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":   query,
			"total":   len(results),
			"results": results,
		},
	})
}

// GetAppStats 获取应用统计
// @Summary 获取应用统计
// @Description 获取应用和容器的统计信息
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/apps/stats [get]
func (h *SearchHandler) GetAppStats(c *gin.Context) {
	stats := h.apps.GetAppStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// SearchContainers 搜索容器
// @Summary 搜索容器
// @Description 搜索Docker容器
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索查询"
// @Param status query string false "状态过滤"
// @Success 200 {object} map[string]interface{}
// @Router /search/containers [get]
func (h *SearchHandler) SearchContainers(c *gin.Context) {
	query := c.Query("query")
	status := c.Query("status")

	var results []search.ContainerItem
	if status != "" {
		results = h.apps.SearchContainersByStatus(status)
	} else if query != "" {
		appResults := h.apps.SearchApps(query, 20)
		for _, r := range appResults {
			if r.Type == "container" {
				if container, ok := r.Item.(search.ContainerItem); ok {
					results = append(results, container)
				}
			}
		}
	} else {
		results = h.apps.GetAllContainers()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":   len(results),
			"results": results,
		},
	})
}
