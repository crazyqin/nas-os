// Package health 提供系统健康检查功能
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 健康检查 API 处理器
type Handlers struct {
	manager *HealthManager
	version string
}

// NewHandlers 创建健康检查处理器
func NewHandlers(manager *HealthManager, version string) *Handlers {
	return &Handlers{
		manager: manager,
		version: version,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1")
	{
		// 健康检查
		api.GET("/health", h.getHealth)
		api.GET("/health/live", h.getLiveness)
		api.GET("/health/ready", h.getReadiness)
		api.GET("/health/report", h.getReport)
		api.GET("/health/checks", h.listChecks)
		api.POST("/health/checks/:name/run", h.runCheck)
		api.GET("/health/checks/:name", h.getCheckResult)
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getHealth 获取整体健康状态
// @Summary 获取系统健康状态
// @Description 返回系统整体健康状态和各组件检查结果
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /api/v1/health [get]
func (h *Handlers) getHealth(c *gin.Context) {
	report := h.manager.GenerateReport(c.Request.Context(), h.version)

	// 根据状态设置 HTTP 状态码
	httpStatus := http.StatusOK
	if report.Status == StatusUnhealthy {
		httpStatus = http.StatusServiceUnavailable
	} else if report.Status == StatusDegraded {
		httpStatus = http.StatusOK // 降级状态仍然返回 200
	}

	c.JSON(httpStatus, HealthResponse{
		Code:    0,
		Message: string(report.Status),
		Data:    report,
	})
}

// getLiveness 存活探针
// @Summary 存活探针
// @Description Kubernetes 存活探针，检查服务是否运行
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /api/v1/health/live [get]
func (h *Handlers) getLiveness(c *gin.Context) {
	// 存活探针只检查服务是否响应
	c.JSON(http.StatusOK, HealthResponse{
		Code:    0,
		Message: "alive",
		Data: gin.H{
			"status": "ok",
		},
	})
}

// getReadiness 就绪探针
// @Summary 就绪探针
// @Description Kubernetes 就绪探针，检查服务是否准备好接收流量
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /api/v1/health/ready [get]
func (h *Handlers) getReadiness(c *gin.Context) {
	// 就绪探针检查关键依赖是否可用
	report := h.manager.RunAllChecks(c.Request.Context())

	// 只有状态为健康时才返回 200
	if report.Status == StatusHealthy {
		c.JSON(http.StatusOK, HealthResponse{
			Code:    0,
			Message: "ready",
			Data: gin.H{
				"status":    "ok",
				"checks":    report.Summary.Total,
				"healthy":   report.Summary.Healthy,
				"unhealthy": report.Summary.Unhealthy,
				"degraded":  report.Summary.Degraded,
			},
		})
		return
	}

	// 不健康或降级状态返回 503
	c.JSON(http.StatusServiceUnavailable, HealthResponse{
		Code:    1,
		Message: "not ready",
		Data: gin.H{
			"status":    report.Status,
			"checks":    report.Summary.Total,
			"healthy":   report.Summary.Healthy,
			"unhealthy": report.Summary.Unhealthy,
			"degraded":  report.Summary.Degraded,
		},
	})
}

// getReport 获取详细健康报告
// @Summary 获取详细健康报告
// @Description 返回包含所有检查详情的健康报告
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /api/v1/health/report [get]
func (h *Handlers) getReport(c *gin.Context) {
	report := h.manager.GenerateReport(c.Request.Context(), h.version)

	c.JSON(http.StatusOK, HealthResponse{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// listChecks 列出所有检查器
// @Summary 列出所有健康检查器
// @Description 返回已注册的所有健康检查器列表
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /api/v1/health/checks [get]
func (h *Handlers) listChecks(c *gin.Context) {
	checkers := h.manager.ListCheckers()
	results := h.manager.GetAllLastResults()

	checks := make([]gin.H, 0, len(checkers))
	for _, name := range checkers {
		check := gin.H{
			"name": name,
		}
		if result, exists := results[name]; exists {
			check["status"] = result.Status
			check["message"] = result.Message
			check["last_check"] = result.Timestamp.Format("2006-01-02T15:04:05Z07:00")
		}
		checks = append(checks, check)
	}

	c.JSON(http.StatusOK, HealthResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(checkers),
			"checks": checks,
		},
	})
}

// runCheck 执行单个检查
// @Summary 执行单个健康检查
// @Description 执行指定名称的健康检查并返回结果
// @Tags health
// @Produce json
// @Param name path string true "检查器名称"
// @Success 200 {object} HealthResponse
// @Failure 404 {object} HealthResponse
// @Router /api/v1/health/checks/{name}/run [post]
func (h *Handlers) runCheck(c *gin.Context) {
	name := c.Param("name")

	result, err := h.manager.RunCheck(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, HealthResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	// 根据状态设置 HTTP 状态码
	httpStatus := http.StatusOK
	if result.Status == StatusUnhealthy {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, HealthResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// getCheckResult 获取最近检查结果
// @Summary 获取最近检查结果
// @Description 返回指定检查器的最近一次检查结果
// @Tags health
// @Produce json
// @Param name path string true "检查器名称"
// @Success 200 {object} HealthResponse
// @Failure 404 {object} HealthResponse
// @Router /api/v1/health/checks/{name} [get]
func (h *Handlers) getCheckResult(c *gin.Context) {
	name := c.Param("name")

	result, exists := h.manager.GetLastResult(name)
	if !exists {
		c.JSON(http.StatusNotFound, HealthResponse{
			Code:    1,
			Message: "check result not found",
		})
		return
	}

	c.JSON(http.StatusOK, HealthResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// RegisterSimpleRoutes 注册简化路由（用于内部服务）
func (h *Handlers) RegisterSimpleRoutes(r *gin.Engine) {
	r.GET("/health", h.getHealth)
	r.GET("/healthz", h.getLiveness)
	r.GET("/readyz", h.getReadiness)
}
