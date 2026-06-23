package storage_efficiency

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 存储效率 API 处理器.
type Handlers struct {
	analyzer  *Analyzer
	optimizer *Optimizer
}

// NewHandlers 创建处理器实例.
func NewHandlers(analyzer *Analyzer, optimizer *Optimizer) *Handlers {
	return &Handlers{
		analyzer:  analyzer,
		optimizer: optimizer,
	}
}

// RegisterRoutes 注册存储效率相关路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/storage/efficiency")
	{
		group.GET("/summary", h.getSummary)
		group.GET("/compression", h.getCompression)
		group.GET("/dedup", h.getDedup)
		group.GET("/suggestions", h.getSuggestions)
		group.POST("/analyze", h.triggerAnalyze)
		group.GET("/trends", h.getTrends)

		// 新增：去重检测和清理建议
		group.POST("/detect-duplicates", h.detectDuplicates)
		group.GET("/duplicate-groups", h.getDuplicateGroups)

		// 新增：存储空间使用分析
		group.GET("/usage-analysis", h.getUsageAnalysis)

		// 新增：存储成本估算
		group.POST("/cost-estimate", h.getCostEstimate)
	}
}

// getSummary 获取存储效率概览.
func (h *Handlers) getSummary(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	sampleRate := parseSampleRate(c.DefaultQuery("sampleRate", "10"))

	summary, err := h.analyzer.Analyze(path, sampleRate, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "分析失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    summary,
	})
}

// getCompression 获取压缩统计详情.
func (h *Handlers) getCompression(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	stats, err := h.analyzer.GetCompressionStats(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取压缩统计失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getDedup 获取去重统计详情.
func (h *Handlers) getDedup(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	stats, err := h.analyzer.GetDedupStats(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取去重统计失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getSuggestions 获取优化建议列表.
func (h *Handlers) getSuggestions(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	suggestions, err := h.optimizer.GenerateSuggestions(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "生成建议失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    suggestions,
	})
}

// triggerAnalyze 触发异步分析任务.
func (h *Handlers) triggerAnalyze(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = AnalyzeRequest{}
	}

	if req.Path == "" {
		req.Path = "/"
	}
	if req.SampleRate <= 0 || req.SampleRate > 100 {
		req.SampleRate = 10
	}

	result := h.analyzer.AnalyzeAsync(req.Path, req.SampleRate, req.DeepScan)

	c.JSON(http.StatusAccepted, gin.H{
		"code":    0,
		"message": "分析任务已启动",
		"data":    result,
	})
}

// getTrends 获取效率趋势数据.
func (h *Handlers) getTrends(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 30
	}

	trends := h.analyzer.GetTrends(days)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    trends,
	})
}

// detectDuplicates 检测重复文件.
func (h *Handlers) detectDuplicates(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	result, err := h.analyzer.DetectDuplicates(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "检测重复文件失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// getDuplicateGroups 获取重复文件组.
func (h *Handlers) getDuplicateGroups(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	minCountStr := c.DefaultQuery("minCount", "2")
	minCount, err := strconv.Atoi(minCountStr)
	if err != nil || minCount < 2 {
		minCount = 2
	}

	result, err := h.analyzer.DetectDuplicates(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取重复文件组失败",
			"message": err.Error(),
		})
		return
	}

	// 过滤掉小于 minCount 的组
	filtered := make([]DuplicateGroup, 0)
	for _, g := range result.Groups {
		if g.Count >= minCount {
			filtered = append(filtered, g)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalGroups": len(filtered),
			"groups":      filtered,
		},
	})
}

// getUsageAnalysis 获取存储空间使用分析.
func (h *Handlers) getUsageAnalysis(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	groupBy := c.DefaultQuery("groupBy", "all")

	result, err := h.analyzer.AnalyzeUsage(path, groupBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "分析存储使用失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// getCostEstimate 获取存储成本估算.
func (h *Handlers) getCostEstimate(c *gin.Context) {
	var req CostEstimateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = CostEstimateRequest{}
	}

	if req.Path == "" {
		req.Path = "/"
	}

	result, err := h.analyzer.EstimateCost(req.Path, req.TierCosts, req.Currency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "估算存储成本失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// parseSampleRate 解析采样率参数.
func parseSampleRate(s string) int {
	rate, err := strconv.Atoi(s)
	if err != nil || rate <= 0 || rate > 100 {
		return 10
	}
	return rate
}
