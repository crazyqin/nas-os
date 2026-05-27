package storageanalyzer

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 存储深度分析 HTTP 处理器.
type Handlers struct {
	analyzer *Analyzer
}

// NewHandlers 创建处理器.
func NewHandlers(analyzer *Analyzer) *Handlers {
	return &Handlers{analyzer: analyzer}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	storageGroup := api.Group("/storage-analyzer")
	{
		// 分析任务
		storageGroup.POST("/analyze", h.startAnalysis)
		storageGroup.GET("/jobs", h.listJobs)
		storageGroup.GET("/jobs/:id", h.getJob)

		// 报告查询
		storageGroup.GET("/reports", h.listReports)
		storageGroup.GET("/reports/:id", h.getReport)

		// 详细数据查询
		storageGroup.GET("/reports/:id/directories", h.getDirectoryUsage)
		storageGroup.GET("/reports/:id/file-types", h.getFileTypeUsage)
		storageGroup.GET("/reports/:id/users", h.getUserUsage)
		storageGroup.GET("/reports/:id/time-usage", h.getTimeUsage)
		storageGroup.GET("/reports/:id/duplicates", h.getDuplicates)
		storageGroup.GET("/reports/:id/big-files", h.getBigFiles)
		storageGroup.GET("/reports/:id/suggestions", h.getSuggestions)
		storageGroup.GET("/reports/:id/heatmap", h.getHeatmap)
		storageGroup.GET("/reports/:id/snapshots", h.getSnapshotUsage)

		// 趋势和历史
		storageGroup.GET("/growth-trend", h.getGrowthTrend)
		storageGroup.GET("/history", h.getHistory)

		// 实时查询
		storageGroup.GET("/current", h.getCurrentUsage)
		storageGroup.GET("/health", h.getStorageHealth)
	}
}

// startAnalysis 启动存储分析.
func (h *Handlers) startAnalysis(c *gin.Context) {
	go func() {
		_, err := h.analyzer.RunAnalysis(c.Request.Context())
		if err != nil {
			// Log error but don't block the response
			return
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "存储分析任务已启动",
		"status":   "running",
		"check_at": "/api/v1/storage-analyzer/jobs",
	})
}

// listJobs 列出所有分析任务.
func (h *Handlers) listJobs(c *gin.Context) {
	// Note: Jobs are stored internally, returning basic info
	c.JSON(http.StatusOK, gin.H{
		"message": "使用 GET /api/v1/storage-analyzer/jobs/:id 查询具体任务状态",
	})
}

// getJob 获取任务状态.
func (h *Handlers) getJob(c *gin.Context) {
	jobID := c.Param("id")
	job, ok := h.analyzer.GetJob(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// listReports 列出所有报告.
func (h *Handlers) listReports(c *gin.Context) {
	reports := h.analyzer.GetReports()
	// Return summary only
	summaries := make([]gin.H, 0, len(reports))
	for _, r := range reports {
		summaries = append(summaries, gin.H{
			"id":           r.ID,
			"generated_at": r.GeneratedAt,
			"used_space":   r.UsedSpace,
			"total_space":  r.TotalSpace,
			"usage_percent": r.UsagePercent,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"reports": summaries,
		"total":   len(summaries),
	})
}

// getReport 获取完整报告.
func (h *Handlers) getReport(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// getDirectoryUsage 获取目录使用情况.
func (h *Handlers) getDirectoryUsage(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	limit := 20
	if l, err := parseIntQuery(c, "limit"); err == nil && l > 0 {
		limit = l
	}

	dirs := report.ByDirectory
	if len(dirs) > limit {
		dirs = dirs[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"directories": dirs,
		"total":       len(report.ByDirectory),
	})
}

// getFileTypeUsage 获取文件类型使用情况.
func (h *Handlers) getFileTypeUsage(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	category := c.Query("category")
	if category != "" {
		filtered := make([]FileTypeUsage, 0)
		for _, ft := range report.ByFileType {
			if ft.Category == category {
				filtered = append(filtered, ft)
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"file_types": filtered,
			"category":   category,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file_types": report.ByFileType,
		"total":      len(report.ByFileType),
	})
}

// getUserUsage 获取用户使用情况.
func (h *Handlers) getUserUsage(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": report.ByUser,
		"total": len(report.ByUser),
	})
}

// getTimeUsage 获取时间维度使用情况.
func (h *Handlers) getTimeUsage(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"time_usage": report.ByTime,
	})
}

// getDuplicates 获取重复文件.
func (h *Handlers) getDuplicates(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	var totalWasted int64
	for _, d := range report.Duplicates {
		totalWasted += d.Wasted
	}

	c.JSON(http.StatusOK, gin.H{
		"duplicates":   report.Duplicates,
		"total_groups": len(report.Duplicates),
		"total_wasted": totalWasted,
	})
}

// getBigFiles 获取大文件列表.
func (h *Handlers) getBigFiles(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	limit := 20
	if l, err := parseIntQuery(c, "limit"); err == nil && l > 0 {
		limit = l
	}

	files := report.BigFiles
	if len(files) > limit {
		files = files[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"big_files": files,
		"total":     len(report.BigFiles),
	})
}

// getSuggestions 获取清理建议.
func (h *Handlers) getSuggestions(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	var totalRecoverable int64
	for _, s := range report.Suggestions {
		totalRecoverable += s.Size
	}

	c.JSON(http.StatusOK, gin.H{
		"suggestions":        report.Suggestions,
		"total":              len(report.Suggestions),
		"total_recoverable":  totalRecoverable,
		"recoverable_human":  formatBytes(totalRecoverable),
	})
}

// getHeatmap 获取存储热力图.
func (h *Handlers) getHeatmap(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"heatmap": report.Heatmap,
		"total":   len(report.Heatmap),
	})
}

// getSnapshotUsage 获取快照使用情况.
func (h *Handlers) getSnapshotUsage(c *gin.Context) {
	reportID := c.Param("id")
	report, ok := h.analyzer.GetReport(reportID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
		return
	}

	c.JSON(http.StatusOK, report.Snapshots)
}

// getGrowthTrend 获取增长趋势.
func (h *Handlers) getGrowthTrend(c *gin.Context) {
	history := h.analyzer.GetHistory()
	if len(history) < 2 {
		c.JSON(http.StatusOK, gin.H{
			"message": "数据不足，需要至少两次快照才能计算趋势",
			"history": history,
		})
		return
	}

	// Calculate growth
	var totalGrowth int64
	var days float64
	for i := 1; i < len(history); i++ {
		growth := history[i].UsedSpace - history[i-1].UsedSpace
		duration := history[i].Timestamp.Sub(history[i-1].Timestamp).Hours() / 24
		if duration > 0 {
			totalGrowth += growth
			days += duration
		}
	}

	dailyAvg := int64(0)
	if days > 0 {
		dailyAvg = int64(float64(totalGrowth) / days)
	}

	c.JSON(http.StatusOK, gin.H{
		"daily_avg":   dailyAvg,
		"weekly_avg":  dailyAvg * 7,
		"monthly_avg": dailyAvg * 30,
		"history":     history,
		"data_points": len(history),
	})
}

// getHistory 获取存储历史.
func (h *Handlers) getHistory(c *gin.Context) {
	history := h.analyzer.GetHistory()
	c.JSON(http.StatusOK, gin.H{
		"history":     history,
		"data_points": len(history),
	})
}

// getCurrentUsage 获取当前存储使用情况.
func (h *Handlers) getCurrentUsage(c *gin.Context) {
	total, free, err := h.analyzer.getFSStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取存储信息: " + err.Error()})
		return
	}

	used := total - free
	usagePercent := float64(used) / float64(total) * 100

	c.JSON(http.StatusOK, gin.H{
		"total_space":   total,
		"total_human":   formatBytes(total),
		"used_space":    used,
		"used_human":    formatBytes(used),
		"free_space":    free,
		"free_human":    formatBytes(free),
		"usage_percent": usagePercent,
		"timestamp":     time.Now(),
	})
}

// getStorageHealth 获取存储健康状态.
func (h *Handlers) getStorageHealth(c *gin.Context) {
	total, free, err := h.analyzer.getFSStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取存储信息: " + err.Error()})
		return
	}

	usagePercent := float64(total-free) / float64(total) * 100

	status := "healthy"
	message := "存储状态良好"
	if usagePercent > 90 {
		status = "critical"
		message = "存储空间严重不足，建议立即清理"
	} else if usagePercent > 80 {
		status = "warning"
		message = "存储空间紧张，建议清理"
	} else if usagePercent > 70 {
		status = "attention"
		message = "存储空间使用率较高"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        status,
		"message":       message,
		"usage_percent": usagePercent,
		"free_space":    free,
		"free_human":    formatBytes(free),
		"timestamp":     time.Now(),
	})
}

// parseIntQuery parses an integer query parameter.
func parseIntQuery(c *gin.Context, key string) (int, error) {
	val := c.Query(key)
	if val == "" {
		return 0, nil
	}
	var result int
	_, err := fmt.Sscanf(val, "%d", &result)
	return result, err
}

// formatBytes formats bytes to human readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
