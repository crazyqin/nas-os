// Package monitor 提供构建指标 API
// build_metrics_api.go - CI/CD 构建指标 HTTP 端点
//
// v2.89.0 工部创建

package monitor

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// BuildMetricsHandler 构建指标处理器
type BuildMetricsHandler struct {
	metrics *BuildMetrics
}

// NewBuildMetricsHandler 创建构建指标处理器
func NewBuildMetricsHandler(metrics *BuildMetrics) *BuildMetricsHandler {
	return &BuildMetricsHandler{
		metrics: metrics,
	}
}

// RegisterRoutes 注册路由
func (h *BuildMetricsHandler) RegisterRoutes(r *gin.RouterGroup) {
	build := r.Group("/build")
	{
		build.GET("/metrics", h.getMetrics)
		build.GET("/stats", h.getStats)
		build.GET("/health", h.getBuildHealth)
		build.POST("/record", h.recordBuild)
	}

	// Prometheus 指标端点
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// BuildMetricsResponse 构建指标响应
type BuildMetricsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// BuildStatsResponse 构建统计响应
type BuildStatsResponse struct {
	TotalBuilds      int64   `json:"totalBuilds"`
	SuccessfulBuilds int64   `json:"successfulBuilds"`
	FailedBuilds     int64   `json:"failedBuilds"`
	SuccessRate      float64 `json:"successRate"`
	AvgBuildTime     float64 `json:"avgBuildTime"`
	LastBuildTime    string  `json:"lastBuildTime"`
	LastBuildStatus  string  `json:"lastBuildStatus"`
	CacheHitCount    int64   `json:"cacheHitCount"`
	CacheMissCount   int64   `json:"cacheMissCount"`
	CacheHitRate     float64 `json:"cacheHitRate"`
}

// BuildHealthResponse 构建健康响应
type BuildHealthResponse struct {
	HealthScore     float64  `json:"healthScore"`
	Status          string   `json:"status"`
	BuildStreak     int64    `json:"buildStreak"`
	DeployStreak    int64    `json:"deployStreak"`
	LastFailure     string   `json:"lastFailure,omitempty"`
	LastBuildStatus string   `json:"lastBuildStatus"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// RecordBuildRequest 记录构建请求
type RecordBuildRequest struct {
	Job      string  `json:"job" binding:"required"`
	Platform string  `json:"platform"`
	Branch   string  `json:"branch"`
	Duration float64 `json:"duration"`
	Success  bool    `json:"success"`
}

// getMetrics 获取 Prometheus 指标
func (h *BuildMetricsHandler) getMetrics(c *gin.Context) {
	// 由 promhttp.Handler() 处理
	c.JSON(http.StatusOK, BuildMetricsResponse{
		Code:    0,
		Message: "Use /metrics endpoint for Prometheus metrics",
	})
}

// getStats 获取构建统计
func (h *BuildMetricsHandler) getStats(c *gin.Context) {
	stats := h.metrics.GetStats()
	successRate := h.metrics.CalculateBuildSuccessRate()

	var cacheHitRate float64
	total := stats.CacheHitCount + stats.CacheMissCount
	if total > 0 {
		cacheHitRate = float64(stats.CacheHitCount) / float64(total)
	}

	var lastBuildTime string
	if !stats.LastBuildTime.IsZero() {
		lastBuildTime = stats.LastBuildTime.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, BuildMetricsResponse{
		Code:    0,
		Message: "success",
		Data: BuildStatsResponse{
			TotalBuilds:      stats.TotalBuilds,
			SuccessfulBuilds: stats.SuccessfulBuilds,
			FailedBuilds:     stats.FailedBuilds,
			SuccessRate:      successRate,
			AvgBuildTime:     stats.AvgBuildTime,
			LastBuildTime:    lastBuildTime,
			LastBuildStatus:  stats.LastBuildStatus,
			CacheHitCount:    stats.CacheHitCount,
			CacheMissCount:   stats.CacheMissCount,
			CacheHitRate:     cacheHitRate,
		},
	})
}

// getBuildHealth 获取构建健康状态
func (h *BuildMetricsHandler) getBuildHealth(c *gin.Context) {
	stats := h.metrics.GetStats()
	successRate := h.metrics.CalculateBuildSuccessRate()

	// 计算健康评分 (0-100)
	healthScore := successRate

	// 确定状态
	status := "healthy"
	if healthScore < 50 {
		status = "critical"
	} else if healthScore < 80 {
		status = "degraded"
	}

	// 生成建议
	var recommendations []string
	if successRate < 80 {
		recommendations = append(recommendations, "构建成功率偏低，建议检查 CI 配置")
	}
	if stats.CacheHitCount+stats.CacheMissCount > 0 {
		cacheRate := float64(stats.CacheHitCount) / float64(stats.CacheHitCount+stats.CacheMissCount)
		if cacheRate < 0.5 {
			recommendations = append(recommendations, "缓存命中率偏低，建议优化缓存策略")
		}
	}

	c.JSON(http.StatusOK, BuildMetricsResponse{
		Code:    0,
		Message: "success",
		Data: BuildHealthResponse{
			HealthScore:     healthScore,
			Status:          status,
			BuildStreak:     stats.SuccessfulBuilds,
			LastBuildStatus: stats.LastBuildStatus,
			Recommendations: recommendations,
		},
	})
}

// recordBuild 记录构建（供 CI 系统调用）
func (h *BuildMetricsHandler) recordBuild(c *gin.Context) {
	var req RecordBuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, BuildMetricsResponse{
			Code:    400,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	platform := req.Platform
	if platform == "" {
		platform = "linux-amd64"
	}

	branch := req.Branch
	if branch == "" {
		branch = "unknown"
	}

	h.metrics.RecordBuild(req.Job, platform, branch, req.Duration, req.Success)

	c.JSON(http.StatusOK, BuildMetricsResponse{
		Code:    0,
		Message: "Build recorded successfully",
		Data: map[string]interface{}{
			"job":      req.Job,
			"platform": platform,
			"branch":   branch,
			"success":  req.Success,
		},
	})
}
