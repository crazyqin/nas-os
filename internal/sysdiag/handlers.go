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
