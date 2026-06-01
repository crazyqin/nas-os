// Package sysdiag 提供系统诊断 HTTP API
package sysdiag

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 系统诊断 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	diag := r.Group("/sysdiag")
	{
		diag.POST("/run", h.runDiagnostics)
		diag.GET("/results", h.getResults)
		diag.GET("/health", h.getHealth)
		diag.GET("/report", h.getReport)
	}

	// 系统诊断增强路由
	diagV1 := r.Group("/diag")
	{
		diagV1.POST("/full", h.runFullDiag)
		diagV1.GET("/network", h.diagnoseNetwork)
		diagV1.GET("/storage", h.diagnoseStorage)
		diagV1.GET("/bottleneck", h.analyzeBottleneck)
		diagV1.POST("/autofix", h.autoFixIssue)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// runDiagnostics 运行诊断.
func (h *Handlers) runDiagnostics(c *gin.Context) {
	task := h.manager.RunDiagnostics()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "diagnostics completed",
		Data:    task,
	})
}

// getResults 获取诊断结果.
func (h *Handlers) getResults(c *gin.Context) {
	task := h.manager.GetLastTask()

	if task == nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "no diagnostics have been run yet",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    task,
	})
}

// getHealth 获取系统健康状态.
func (h *Handlers) getHealth(c *gin.Context) {
	health := h.manager.GetHealthStatus()

	// 确定整体健康状态
	overallStatus := DiagStatusPass
	for _, item := range health {
		if item.Status == DiagStatusFail {
			overallStatus = DiagStatusFail
			break
		}
		if item.Status == DiagStatusWarn {
			overallStatus = DiagStatusWarn
		}
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: string(overallStatus),
		Data: gin.H{
			"status": overallStatus,
			"items":  health,
		},
	})
}

// getReport 获取诊断报告.
func (h *Handlers) getReport(c *gin.Context) {
	report := h.manager.GetLastReport()

	if report == nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "no report available, run diagnostics first",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// ========== 系统诊断增强 Handlers ==========

func (h *Handlers) runFullDiag(c *gin.Context) {
	task := h.manager.RunFullDiag()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "full diagnostics completed",
		Data:    task,
	})
}

func (h *Handlers) diagnoseNetwork(c *gin.Context) {
	diag := h.manager.DiagnoseNetwork()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "network diagnostics completed",
		Data:    diag,
	})
}

func (h *Handlers) diagnoseStorage(c *gin.Context) {
	diag := h.manager.DiagnoseStorage()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "storage diagnostics completed",
		Data:    diag,
	})
}

func (h *Handlers) analyzeBottleneck(c *gin.Context) {
	bottlenecks := h.manager.AnalyzeBottleneck()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "bottleneck analysis completed",
		Data: gin.H{
			"total":       len(bottlenecks),
			"bottlenecks": bottlenecks,
		},
	})
}

func (h *Handlers) autoFixIssue(c *gin.Context) {
	var req AutoFixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	fix := h.manager.AutoFixIssue(req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "auto fix completed",
		Data:    fix,
	})
}
