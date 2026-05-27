// Package datascanner 提供 REST API 处理器
package datascanner

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 隐私数据扫描模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/data-scanner 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ds := r.Group("/data-scanner")
	{
		// 扫描任务 CRUD
		ds.POST("/tasks", h.createTask)
		ds.GET("/tasks", h.listTasks)
		ds.GET("/tasks/:id", h.getTask)
		ds.DELETE("/tasks/:id", h.deleteTask)

		// 任务控制
		ds.POST("/tasks/:id/start", h.startTask)
		ds.POST("/tasks/:id/pause", h.pauseTask)
		ds.POST("/tasks/:id/cancel", h.cancelTask)

		// 扫描结果
		ds.GET("/tasks/:id/results", h.getResults)
		ds.GET("/results/:resultId", h.getResult)

		// 内容扫描（直接提交文本）
		ds.POST("/scan-content", h.scanContent)

		// 报告
		ds.POST("/reports", h.generateReport)
		ds.GET("/reports/:id", h.getReport)
		ds.GET("/tasks/:id/reports", h.listReports)

		// 统计
		ds.GET("/tasks/:id/stats", h.getStats)

		// 白名单
		ds.POST("/whitelists", h.createWhitelist)
		ds.GET("/whitelists", h.listWhitelists)
		ds.GET("/whitelists/:id", h.getWhitelist)
		ds.PUT("/whitelists/:id", h.updateWhitelist)
		ds.DELETE("/whitelists/:id", h.deleteWhitelist)

		// 脱敏策略
		ds.GET("/desensitize-strategies", h.getDesensitizeStrategies)
	}
}

// ========== 扫描任务 Handlers ==========

// createTask 创建扫描任务.
func (h *Handlers) createTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "scan path is required"})
		return
	}

	task := h.manager.CreateTask(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: task})
}

// getTask 获取扫描任务.
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: task})
}

// listTasks 列出所有扫描任务.
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(tasks),
			"tasks": tasks,
		},
	})
}

// deleteTask 删除扫描任务.
func (h *Handlers) deleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// startTask 启动扫描任务.
func (h *Handlers) startTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.StartTask(id)
	if err != nil {
		if err == ErrTaskRunning {
			c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "started", Data: task})
}

// pauseTask 暂停扫描任务.
func (h *Handlers) pauseTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.PauseTask(id)
	if err != nil {
		if err == ErrTaskNotRunning {
			c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "paused", Data: task})
}

// cancelTask 取消扫描任务.
func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.CancelTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "canceled", Data: task})
}

// ========== 扫描结果 Handlers ==========

// getResults 获取任务扫描结果.
func (h *Handlers) getResults(c *gin.Context) {
	taskID := c.Param("id")
	riskLevel := c.Query("risk_level")
	piiType := c.Query("pii_type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 {
		limit = 50
	}

	results, total, err := h.manager.GetResults(taskID, riskLevel, piiType, limit, offset)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   total,
			"limit":   limit,
			"offset":  offset,
			"results": results,
		},
	})
}

// getResult 获取单条扫描结果.
func (h *Handlers) getResult(c *gin.Context) {
	resultID := c.Param("resultId")
	result, err := h.manager.GetResult(resultID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// scanContent 直接扫描文本内容.
func (h *Handlers) scanContent(c *gin.Context) {
	var req struct {
		Content  string    `json:"content" binding:"required"`
		FilePath string    `json:"file_path"`
		PIITypes []PIIType `json:"pii_types"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	// 默认检测所有类型
	if len(req.PIITypes) == 0 {
		req.PIITypes = []PIIType{
			PIIIDCard, PIIPhone, PIIBankCard, PIIEmail, PIIAddress, PIIName,
			PIICreditCode, PIIPassport, PIIMilitaryID, PIILicensePlate,
		}
	}

	results := h.manager.ScanContent(req.Content, req.FilePath, req.PIITypes)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(results),
			"results": results,
		},
	})
}

// ========== 报告 Handlers ==========

// generateReport 生成扫描报告.
func (h *Handlers) generateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	report, err := h.manager.GenerateReport(req.TaskID, req.Format)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "report generated", Data: report})
}

// getReport 获取报告.
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

// listReports 列出任务报告.
func (h *Handlers) listReports(c *gin.Context) {
	taskID := c.Param("id")
	reports := h.manager.ListReports(taskID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(reports),
			"reports": reports,
		},
	})
}

// ========== 统计 Handlers ==========

// getStats 获取任务扫描统计.
func (h *Handlers) getStats(c *gin.Context) {
	taskID := c.Param("id")
	stats, err := h.manager.GetStats(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 白名单 Handlers ==========

// createWhitelist 创建白名单.
func (h *Handlers) createWhitelist(c *gin.Context) {
	var req CreateWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule := h.manager.CreateWhitelist(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: rule})
}

// listWhitelists 列出白名单.
func (h *Handlers) listWhitelists(c *gin.Context) {
	rules := h.manager.ListWhitelists()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(rules),
			"whitelists": rules,
		},
	})
}

// getWhitelist 获取白名单.
func (h *Handlers) getWhitelist(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetWhitelist(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: rule})
}

// updateWhitelist 更新白名单.
func (h *Handlers) updateWhitelist(c *gin.Context) {
	id := c.Param("id")
	var req UpdateWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule, err := h.manager.UpdateWhitelist(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: rule})
}

// deleteWhitelist 删除白名单.
func (h *Handlers) deleteWhitelist(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteWhitelist(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 脱敏策略 Handlers ==========

// getDesensitizeStrategies 获取脱敏策略建议.
func (h *Handlers) getDesensitizeStrategies(c *gin.Context) {
	strategies := h.manager.GetDesensitizeStrategies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(strategies),
			"strategies": strategies,
		},
	})
}
