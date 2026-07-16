// Package selfheal 提供 REST API 处理器
package selfheal

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 自愈模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sh := r.Group("/self-heal")
	{
		sh.GET("/status", h.getStatus)
		sh.GET("/checks", h.listChecks)
		sh.POST("/run", h.runChecks)
		sh.PUT("/config", h.updateConfig)
		sh.GET("/config", h.getConfig)
		sh.GET("/history", h.getHistory)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getStatus 获取整体健康状态.
func (h *Handlers) getStatus(c *gin.Context) {
	status := h.manager.GetLastStatus()
	if status == nil {
		// 未执行过检查，执行一次
		status = h.manager.RunAll(c.Request.Context())
	}

	httpStatus := http.StatusOK
	switch status.Status {
	case StatusUnhealthy:
		httpStatus = http.StatusServiceUnavailable
	case StatusDegraded:
		httpStatus = http.StatusOK // 降级仍返回 200
	}

	c.JSON(httpStatus, response{
		Code:    0,
		Message: string(status.Status),
		Data:    status,
	})
}

// listChecks 列出所有检查项.
func (h *Handlers) listChecks(c *gin.Context) {
	checks := h.manager.ListCheckers()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(checks),
			"checks": checks,
		},
	})
}

// RunRequest 手动执行请求.
type RunRequest struct {
	Name string `json:"name"` // 为空时执行全部
}

// runChecks 手动执行检查.
func (h *Handlers) runChecks(c *gin.Context) {
	var req RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 无 body 时执行全部检查
		req.Name = ""
	}

	if req.Name == "" {
		// 执行所有检查
		status := h.manager.RunAll(c.Request.Context())
		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    status,
		})
		return
	}

	// 执行单个检查
	result, err := h.manager.RunSingle(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// updateConfig 更新自愈策略配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg StrategyConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid config: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
		Data:    h.manager.GetConfig(),
	})
}

// getConfig 获取当前配置.
func (h *Handlers) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    h.manager.GetConfig(),
	})
}

// getHistory 获取检查历史.
func (h *Handlers) getHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	records, err := h.manager.GetHistory(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(records),
			"records": records,
		},
	})
}
