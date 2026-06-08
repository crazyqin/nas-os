package storageanalytics

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP API 处理器.
type Handler struct {
	collector *Collector
	analyzer  *Analyzer
	reporter  *Reporter
	logger    *zap.Logger
	analyzing int32 // 原子标志，防止并发分析
}

// NewHandler 创建HTTP处理器.
func NewHandler(collector *Collector, analyzer *Analyzer, reporter *Reporter, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		collector: collector,
		analyzer:  analyzer,
		reporter:  reporter,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	storageGroup := api.Group("/storage-analytics")
	{
		storageGroup.POST("/analyze", h.analyze)
		storageGroup.GET("/overview", h.overview)
		storageGroup.GET("/breakdown", h.breakdown)
		storageGroup.GET("/trends", h.trends)
		storageGroup.GET("/insights", h.insights)
		storageGroup.GET("/report", h.fullReport)
		// 存储成本分析
		storageGroup.GET("/cost/overview", h.costOverview)
		storageGroup.GET("/cost/forecast", h.costForecast)
		storageGroup.GET("/cost/tiers", h.costTiers)
		storageGroup.GET("/cost/cloud-comparison", h.costCloudComparison)
		// 存储优化建议
		storageGroup.GET("/optimization/recommendations", h.optimizationRecommendations)
		storageGroup.GET("/optimization/savings", h.optimizationSavings)
	}
}

// analyze 启动存储分析.
func (h *Handler) analyze(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不能为空"})
		return
	}

	// 防止并发分析
	if !atomic.CompareAndSwapInt32(&h.analyzing, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"error": ErrAnalysisRunning.Error()})
		return
	}
	defer atomic.StoreInt32(&h.analyzing, 0)

	h.logger.Info("开始分析", zap.String("path", req.Path))

	// 采集
	result, err := h.collector.Collect(req.Path, req.MaxDepth, req.TopN)
	if err != nil {
		h.logger.Error("采集失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "采集失败: " + err.Error()})
		return
	}

	// 分析
	report := h.analyzer.Analyze(result)

	c.JSON(http.StatusOK, gin.H{
		"message":     "分析完成",
		"scan_path":   report.ScanPath,
		"total_size":  report.Summary.TotalSize,
		"total_files": report.Summary.TotalFiles,
		"total_dirs":  report.Summary.TotalDirs,
	})
}

// overview 获取存储概览.
func (h *Handler) overview(c *gin.Context) {
	report, err := h.analyzer.GetLastReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scan_path":    report.ScanPath,
		"generated_at": report.GeneratedAt,
		"summary":      report.Summary,
		"health":       report.Health,
	})
}

// breakdown 获取文件类型/目录分布.
func (h *Handler) breakdown(c *gin.Context) {
	report, err := h.analyzer.GetLastReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_type_stats":     report.FileTypeStats,
		"top_directories":     report.TopDirectories,
		"size_distribution":   report.SizeDist,
		"age_distribution":    report.AgeDist,
		"access_distribution": report.AccessDist,
	})
}

// trends 获取增长趋势.
func (h *Handler) trends(c *gin.Context) {
	report, err := h.analyzer.GetLastReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report.Trends)
}

// insights 获取智能洞察.
func (h *Handler) insights(c *gin.Context) {
	report, err := h.analyzer.GetLastReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report.Insights)
}

// fullReport 获取完整报告.
func (h *Handler) fullReport(c *gin.Context) {
	report, err := h.analyzer.GetLastReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 根据 Accept 头决定格式
	format := c.DefaultQuery("format", "json")

	switch format {
	case "markdown", "md":
		costReport, _ := h.analyzer.GetLastCostReport()
		var markdown string
		if costReport != nil {
			markdown = h.reporter.ToMarkdownWithCost(report, costReport)
		} else {
			markdown = h.reporter.ToMarkdown(report)
		}
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, markdown)
	default:
		data, err := h.reporter.ToJSON(report)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成报告失败: " + err.Error()})
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	}
}

// costOverview 获取存储成本概览.
func (h *Handler) costOverview(c *gin.Context) {
	costReport, err := h.analyzer.GetLastCostReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"generated_at":       costReport.GeneratedAt,
		"total_monthly_cost": costReport.TotalMonthlyCost,
		"total_yearly_cost":  costReport.TotalYearlyCost,
		"cost_per_tb_avg":    costReport.CostPerTBAvg,
		"tier_breakdown":     costReport.TierBreakdown,
	})
}

// costForecast 获取成本预测.
func (h *Handler) costForecast(c *gin.Context) {
	costReport, err := h.analyzer.GetLastCostReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if costReport.Forecast == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "暂无预测数据"})
		return
	}

	c.JSON(http.StatusOK, costReport.Forecast)
}

// costTiers 获取各存储层级成本详情.
func (h *Handler) costTiers(c *gin.Context) {
	costReport, err := h.analyzer.GetLastCostReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tiers": costReport.TierBreakdown,
	})
}

// costCloudComparison 获取云存储成本对比.
func (h *Handler) costCloudComparison(c *gin.Context) {
	costReport, err := h.analyzer.GetLastCostReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if costReport.ComparisonWithCloud == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "暂无云存储对比数据"})
		return
	}

	c.JSON(http.StatusOK, costReport.ComparisonWithCloud)
}

// optimizationRecommendations 获取优化建议.
func (h *Handler) optimizationRecommendations(c *gin.Context) {
	costReport, err := h.analyzer.GetLastCostReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recommendations": costReport.Recommendations,
		"count":           len(costReport.Recommendations),
	})
}

// optimizationSavings 获取优化节省汇总.
func (h *Handler) optimizationSavings(c *gin.Context) {
	costReport, err := h.analyzer.GetLastCostReport()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	totalSavingBytes := int64(0)
	totalSavingCost := 0.0
	byCategory := make(map[string]int64)

	for _, rec := range costReport.Recommendations {
		totalSavingBytes += rec.SavingBytes
		totalSavingCost += rec.SavingCost
		byCategory[rec.Category] += rec.SavingBytes
	}

	c.JSON(http.StatusOK, gin.H{
		"total_saving_bytes": totalSavingBytes,
		"total_saving_cost":  totalSavingCost,
		"by_category":        byCategory,
		"recommendation_count": len(costReport.Recommendations),
	})
}
