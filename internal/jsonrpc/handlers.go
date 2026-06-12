// Package jsonrpc 提供JSON-RPC 2.0 API的HTTP处理器
package jsonrpc

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers JSON-RPC HTTP处理器
type Handlers struct {
	server *Server
}

// NewHandlers 创建处理器
func NewHandlers(server *Server) *Handlers {
	return &Handlers{server: server}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	rpcGroup := api.Group("/jsonrpc")
	{
		// JSON-RPC端点
		rpcGroup.POST("", h.handleRPC)
		rpcGroup.POST("/", h.handleRPC)

		// API密钥管理
		rpcGroup.POST("/api-keys", h.createAPIKey)
		rpcGroup.GET("/api-keys", h.listAPIKeys)
		rpcGroup.GET("/api-keys/:id", h.getAPIKey)
		rpcGroup.DELETE("/api-keys/:id", h.revokeAPIKey)

		// 方法列表
		rpcGroup.GET("/methods", h.listMethods)

		// 版本信息
		rpcGroup.GET("/versions", h.getVersions)

		// 统计
		rpcGroup.GET("/stats", h.getStats)
		rpcGroup.GET("/health", h.healthCheck)
	}
}

// handleRPC 处理JSON-RPC请求
func (h *Handlers) handleRPC(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, &Response{
			JSONRPC: "2.0",
			Error: &Error{
				Code:    ErrorCodeParseError,
				Message: "读取请求体失败",
			},
		})
		return
	}

	// 检查是否为批量请求
	if len(body) > 0 && body[0] == '[' {
		var batch BatchRequest
		if err := json.Unmarshal(body, &batch); err != nil {
			c.JSON(http.StatusBadRequest, &Response{
				JSONRPC: "2.0",
				Error: &Error{
					Code:    ErrorCodeParseError,
					Message: "JSON解析失败",
				},
			})
			return
		}

		responses := h.server.HandleBatchRequest(batch)
		c.JSON(http.StatusOK, responses)
		return
	}

	// 单个请求
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, &Response{
			JSONRPC: "2.0",
			Error: &Error{
				Code:    ErrorCodeParseError,
				Message: "JSON解析失败",
			},
		})
		return
	}

	resp := h.server.HandleRequest(&req)
	c.JSON(http.StatusOK, resp)
}

// createAPIKey 创建API密钥
func (h *Handlers) createAPIKey(c *gin.Context) {
	var key APIKey
	if err := c.ShouldBindJSON(&key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.server.CreateAPIKey(&key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "API密钥创建成功",
		"api_key": key,
	})
}

// listAPIKeys 列出API密钥
func (h *Handlers) listAPIKeys(c *gin.Context) {
	userID := c.Query("user_id")
	keys := h.server.ListAPIKeys(userID)
	c.JSON(http.StatusOK, gin.H{
		"api_keys": keys,
		"total":    len(keys),
	})
}

// getAPIKey 获取API密钥
func (h *Handlers) getAPIKey(c *gin.Context) {
	id := c.Param("id")
	key, err := h.server.GetAPIKey(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, key)
}

// revokeAPIKey 撤销API密钥
func (h *Handlers) revokeAPIKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.server.RevokeAPIKey(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "API密钥已撤销"})
}

// listMethods 列出方法
func (h *Handlers) listMethods(c *gin.Context) {
	stats := h.server.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"methods": stats["methods"],
		"version": stats["version"],
	})
}

// getVersions 获取版本信息
func (h *Handlers) getVersions(c *gin.Context) {
	versions := h.server.GetVersions()
	c.JSON(http.StatusOK, gin.H{
		"versions": versions,
		"current":  h.server.currentVersion,
	})
}

// getStats 获取统计
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.server.GetStats()
	c.JSON(http.StatusOK, stats)
}

// healthCheck 健康检查
func (h *Handlers) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"api":    "JSON-RPC 2.0",
		"version": h.server.currentVersion,
	})
}
