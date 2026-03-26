package search

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GlobalSearchHandlers 全局搜索处理器.
type GlobalSearchHandlers struct {
	service *GlobalSearchService
}

// NewGlobalSearchHandlers 创建全局搜索处理器.
func NewGlobalSearchHandlers(service *GlobalSearchService) *GlobalSearchHandlers {
	return &GlobalSearchHandlers{service: service}
}

// RegisterRoutes 注册路由.
func (h *GlobalSearchHandlers) RegisterRoutes(r *gin.RouterGroup) {
	search := r.Group("/search")
	{
		// 全局搜索
		search.POST("/global", h.globalSearch)
		search.GET("/quick", h.quickSearch)
		search.GET("/suggestions", h.suggestions)

		// 分类搜索
		search.POST("/files", h.searchFiles)
		search.POST("/settings", h.searchSettings)
		search.POST("/apps", h.searchApps)
		search.POST("/containers", h.searchContainers)
		search.POST("/apis", h.searchAPIs)
		search.POST("/docs", h.searchDocs)
		search.POST("/logs", h.searchLogs)

		// 搜索历史
		search.GET("/history/recent", h.getRecentSearches)
		search.GET("/history/popular", h.getPopularSearches)
		search.DELETE("/history", h.clearHistory)

		// 分类和统计
		search.GET("/categories", h.getCategories)
		search.GET("/stats", h.getStats)
	}
}

// globalSearch 全局搜索
// @Summary 全局搜索
// @Description 执行全局搜索，支持文件、设置、应用、API、文档、日志等多种类型
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body GlobalSearchRequest true "搜索请求"
// @Success 200 {object} GlobalSearchResponse
// @Failure 400 {object} map[string]interface{}
// @Router /search/global [post].
func (h *GlobalSearchHandlers) globalSearch(c *gin.Context) {
	var req GlobalSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 执行搜索
	result, err := h.service.GlobalSearch(c.Request.Context(), req)
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

// quickSearch 快速搜索
// @Summary 快速搜索
// @Description 执行快速搜索，用于自动补全
// @Tags 搜索
// @Produce json
// @Param q query string true "搜索关键词"
// @Param limit query int false "结果数量限制" default(5)
// @Success 200 {object} GlobalSearchResponse
// @Router /search/quick [get].
func (h *GlobalSearchHandlers) quickSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "搜索关键词不能为空",
		})
		return
	}

	limit := 5
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil {
			limit = parsed
		}
	}

	result, err := h.service.QuickSearch(c.Request.Context(), query, limit)
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

// suggestions 获取搜索建议
// @Summary 获取搜索建议
// @Description 基于输入获取搜索建议
// @Tags 搜索
// @Produce json
// @Param q query string true "搜索关键词"
// @Success 200 {object} map[string]interface{}
// @Router /search/suggestions [get].
func (h *GlobalSearchHandlers) suggestions(c *gin.Context) {
	query := c.Query("q")
	suggestions := h.service.GenerateSuggestions(query)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":       query,
			"suggestions": suggestions,
		},
	})
}

// searchFiles 搜索文件
// @Summary 搜索文件
// @Description 搜索文件系统中的文件
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body Request true "搜索请求"
// @Success 200 {object} Response
// @Router /search/files [post].
func (h *GlobalSearchHandlers) searchFiles(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	engine := h.service.engine
	if engine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索引擎未初始化",
		})
		return
	}

	result, err := engine.Search(req)
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

// searchSettings 搜索设置
// @Summary 搜索设置
// @Description 搜索系统设置项
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "搜索请求 {query: string, limit: int}"
// @Success 200 {object} map[string]interface{}
// @Router /search/settings [post].
func (h *GlobalSearchHandlers) searchSettings(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}

	registry := h.service.settingsRegistry
	if registry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "设置注册表未初始化",
		})
		return
	}

	results := registry.SearchSettings(req.Query, req.Limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  req.Query,
			"total":  len(results),
			"result": results,
		},
	})
}

// searchApps 搜索应用
// @Summary 搜索应用
// @Description 搜索已安装的应用
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "搜索请求 {query: string, limit: int}"
// @Success 200 {object} map[string]interface{}
// @Router /search/apps [post].
func (h *GlobalSearchHandlers) searchApps(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}

	registry := h.service.appRegistry
	if registry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "应用注册表未初始化",
		})
		return
	}

	results := registry.SearchApps(req.Query, req.Limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  req.Query,
			"total":  len(results),
			"result": results,
		},
	})
}

// searchContainers 搜索容器
// @Summary 搜索容器
// @Description 搜索Docker容器
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "搜索请求 {query: string, status: string}"
// @Success 200 {object} map[string]interface{}
// @Router /search/containers [post].
func (h *GlobalSearchHandlers) searchContainers(c *gin.Context) {
	var req struct {
		Query  string `json:"query"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	registry := h.service.appRegistry
	if registry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "容器注册表未初始化",
		})
		return
	}

	var results []ContainerItem
	if req.Status != "" {
		results = registry.SearchContainersByStatus(req.Status)
	} else if req.Query != "" {
		appResults := registry.SearchApps(req.Query, 100)
		for _, r := range appResults {
			if r.Type == "container" {
				if container, ok := r.Item.(ContainerItem); ok {
					results = append(results, container)
				}
			}
		}
	} else {
		results = registry.GetAllContainers()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  req.Query,
			"total":  len(results),
			"result": results,
		},
	})
}

// searchAPIs 搜索API端点
// @Summary 搜索API端点
// @Description 搜索系统API端点
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "搜索请求 {query: string, limit: int}"
// @Success 200 {object} map[string]interface{}
// @Router /search/apis [post].
func (h *GlobalSearchHandlers) searchAPIs(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}

	registry := h.service.apiRegistry
	if registry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "API注册表未初始化",
		})
		return
	}

	results := registry.SearchAPIs(req.Query, req.Limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  req.Query,
			"total":  len(results),
			"result": results,
		},
	})
}

// searchDocs 搜索文档
// @Summary 搜索文档
// @Description 搜索系统文档
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "搜索请求 {query: string, limit: int, type: string}"
// @Success 200 {object} map[string]interface{}
// @Router /search/docs [post].
func (h *GlobalSearchHandlers) searchDocs(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
		Type  string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}

	registry := h.service.docRegistry
	if registry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "文档注册表未初始化",
		})
		return
	}

	results := registry.SearchDocs(req.Query, req.Limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  req.Query,
			"total":  len(results),
			"result": results,
		},
	})
}

// searchLogs 搜索日志
// @Summary 搜索日志
// @Description 搜索系统日志
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body LogSearchRequest true "搜索请求"
// @Success 200 {object} LogSearchResponse
// @Router /search/logs [post].
func (h *GlobalSearchHandlers) searchLogs(c *gin.Context) {
	var req LogSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	registry := h.service.logRegistry
	if registry == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "日志注册表未初始化",
		})
		return
	}

	result, err := registry.SearchLogs(req)
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

// getRecentSearches 获取最近搜索
// @Summary 获取最近搜索
// @Description 获取用户的最近搜索记录
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/history/recent [get].
func (h *GlobalSearchHandlers) getRecentSearches(c *gin.Context) {
	searches := h.service.GetRecentSearches()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"searches": searches,
		},
	})
}

// getPopularSearches 获取热门搜索
// @Summary 获取热门搜索
// @Description 获取热门搜索关键词
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/history/popular [get].
func (h *GlobalSearchHandlers) getPopularSearches(c *gin.Context) {
	searches := h.service.GetPopularSearches()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"searches": searches,
		},
	})
}

// clearHistory 清除搜索历史
// @Summary 清除搜索历史
// @Description 清除用户的搜索历史记录
// @Tags 搜索
// @Success 200 {object} map[string]interface{}
// @Router /search/history [delete].
func (h *GlobalSearchHandlers) clearHistory(c *gin.Context) {
	err := h.service.ClearRecentSearches()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "清除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getCategories 获取搜索分类
// @Summary 获取搜索分类
// @Description 获取所有可搜索的分类
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/categories [get].
func (h *GlobalSearchHandlers) getCategories(c *gin.Context) {
	categories := h.service.GetSearchCategories()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"categories": categories,
		},
	})
}

// getStats 获取搜索统计
// @Summary 获取搜索统计
// @Description 获取搜索引擎的统计信息
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/stats [get].
func (h *GlobalSearchHandlers) getStats(c *gin.Context) {
	stats := h.service.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// parseInt 解析整数.
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
