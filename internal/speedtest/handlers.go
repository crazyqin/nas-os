// Package speedtest 提供 REST API 处理器
package speedtest

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

// Handlers 网络测速 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	st := r.Group("/network/speedtest")
	{
		st.POST("/run", h.RunTest)
		st.POST("/run/download", h.RunDownloadTest)
		st.POST("/run/upload", h.RunUploadTest)
		st.POST("/run/latency", h.RunLatencyTest)
		st.GET("/servers", h.ListServers)
		st.POST("/servers", h.AddServer)
		st.DELETE("/servers/:id", h.RemoveServer)
		st.GET("/history", h.GetHistory)
		st.GET("/stats", h.GetStats)
		st.DELETE("/history", h.ClearHistory)
	}
}

// ========== 测试接口 ==========

// RunTest 运行完整测速.
func (h *Handlers) RunTest(c *gin.Context) {
	var req RunTestRequest
	// 请求体是可选的
	_ = c.ShouldBindJSON(&req)

	result, err := h.manager.RunTest(req.ServerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "test completed", Data: result})
}

// RunDownloadTest 仅测试下载.
func (h *Handlers) RunDownloadTest(c *gin.Context) {
	var req RunTestRequest
	_ = c.ShouldBindJSON(&req)

	result, err := h.manager.RunDownloadTest(req.ServerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "download test completed", Data: result})
}

// RunUploadTest 仅测试上传.
func (h *Handlers) RunUploadTest(c *gin.Context) {
	var req RunTestRequest
	_ = c.ShouldBindJSON(&req)

	result, err := h.manager.RunUploadTest(req.ServerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "upload test completed", Data: result})
}

// RunLatencyTest 仅测试延迟.
func (h *Handlers) RunLatencyTest(c *gin.Context) {
	var req RunTestRequest
	_ = c.ShouldBindJSON(&req)

	result, err := h.manager.RunLatencyTest(req.ServerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "latency test completed", Data: result})
}

// ========== 服务器接口 ==========

// ListServers 列出服务器.
func (h *Handlers) ListServers(c *gin.Context) {
	servers := h.manager.ListServers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(servers),
			"servers": servers,
		},
	})
}

// AddServer 添加服务器.
func (h *Handlers) AddServer(c *gin.Context) {
	var req AddServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	server := h.manager.AddServer(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: server})
}

// RemoveServer 移除服务器.
func (h *Handlers) RemoveServer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveServer(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "removed"})
}

// ========== 历史接口 ==========

// GetHistory 获取历史记录.
func (h *Handlers) GetHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetHistory(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// GetStats 获取统计数据.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ClearHistory 清除历史记录.
func (h *Handlers) ClearHistory(c *gin.Context) {
	h.manager.ClearHistory()
	c.JSON(http.StatusOK, response{Code: 0, Message: "history cleared"})
}
