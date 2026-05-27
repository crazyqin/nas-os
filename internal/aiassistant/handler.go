// Package aiassistant 提供 REST API 处理器
package aiassistant

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers AI 助手 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ai := r.Group("/ai-assistant")
	{
		// 查询接口
		ai.POST("/query", h.query)
		ai.POST("/query/stream", h.queryStream) // 流式响应（预留）

		// 系统状态快速查询
		ai.GET("/status", h.getSystemStatus)
		ai.GET("/status/cpu", h.getCPUStatus)
		ai.GET("/status/memory", h.getMemoryStatus)
		ai.GET("/status/disks", h.getDiskStatus)

		// 文件搜索
		ai.POST("/search", h.searchFiles)

		// 故障诊断
		ai.POST("/diagnose", h.diagnose)

		// 对话管理
		ai.POST("/conversations", h.createConversation)
		ai.GET("/conversations/:id", h.getConversation)
		ai.POST("/conversations/:id/messages", h.addMessage)

		// 历史和配置
		ai.GET("/history", h.getHistory)
		ai.GET("/config", h.getConfig)
		ai.PUT("/config", h.updateConfig)
		ai.POST("/cache/clear", h.clearCache)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// query 处理自然语言查询
func (h *Handlers) query(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.manager.ProcessQuery(c.Request.Context(), &req)
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
		Data:    resp,
	})
}

// queryStream 流式查询（预留）
func (h *Handlers) queryStream(c *gin.Context) {
	// TODO: 实现 SSE 或 WebSocket 流式响应
	c.JSON(http.StatusNotImplemented, response{
		Code:    1,
		Message: "streaming not yet implemented",
	})
}

// getSystemStatus 获取系统状态
func (h *Handlers) getSystemStatus(c *gin.Context) {
	req := &QueryRequest{Query: "系统状态概览", QueryType: QueryTypeSystem}
	resp, err := h.manager.ProcessQuery(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: resp.Data})
}

// getCPUStatus 获取 CPU 状态
func (h *Handlers) getCPUStatus(c *gin.Context) {
	req := &QueryRequest{Query: "CPU 使用情况", QueryType: QueryTypeCPU}
	resp, err := h.manager.ProcessQuery(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: resp.Data})
}

// getMemoryStatus 获取内存状态
func (h *Handlers) getMemoryStatus(c *gin.Context) {
	req := &QueryRequest{Query: "内存使用情况", QueryType: QueryTypeMemory}
	resp, err := h.manager.ProcessQuery(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: resp.Data})
}

// getDiskStatus 获取磁盘状态
func (h *Handlers) getDiskStatus(c *gin.Context) {
	req := &QueryRequest{Query: "磁盘使用情况", QueryType: QueryTypeDisk}
	resp, err := h.manager.ProcessQuery(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: resp.Data})
}

// searchFiles 文件搜索
func (h *Handlers) searchFiles(c *gin.Context) {
	var req FileSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	queryReq := &QueryRequest{
		Query:     req.Query,
		QueryType: QueryTypeFile,
	}
	resp, err := h.manager.ProcessQuery(c.Request.Context(), queryReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: resp.Data})
}

// diagnose 故障诊断
func (h *Handlers) diagnose(c *gin.Context) {
	var req DiagnosisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	queryReq := &QueryRequest{
		Query:     req.Problem,
		QueryType: QueryTypeDiag,
	}
	resp, err := h.manager.ProcessQuery(c.Request.Context(), queryReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: resp.Data})
}

// createConversation 创建对话
func (h *Handlers) createConversation(c *gin.Context) {
	conv := h.manager.CreateConversation()
	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "conversation created",
		Data:    conv,
	})
}

// getConversation 获取对话
func (h *Handlers) getConversation(c *gin.Context) {
	id := c.Param("id")
	conv, err := h.manager.GetConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: conv})
}

// addMessage 添加消息
func (h *Handlers) addMessage(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Role    string `json:"role" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	if err := h.manager.AddMessage(id, req.Role, req.Content); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "message added"})
}

// getHistory 获取查询历史
func (h *Handlers) getHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetQueryHistory(limit)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: history})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg AIConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{Code: 0, Message: "config updated"})
}

// clearCache 清除缓存
func (h *Handlers) clearCache(c *gin.Context) {
	h.manager.ClearCache()
	c.JSON(http.StatusOK, response{Code: 0, Message: "cache cleared"})
}
