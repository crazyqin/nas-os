// Package dlpengine 提供 REST API 处理器
package dlpengine

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers DLP引擎 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	dlp := r.Group("/dlp")
	{
		// 内容扫描
		dlp.POST("/scan", h.scanContent)
		dlp.POST("/scan/file", h.scanFile)

		// 策略管理
		dlp.GET("/policies", h.listPolicies)
		dlp.POST("/policies", h.createPolicy)
		dlp.GET("/policies/:id", h.getPolicy)
		dlp.PUT("/policies/:id", h.updatePolicy)
		dlp.DELETE("/policies/:id", h.deletePolicy)

		// 模式管理
		dlp.GET("/patterns", h.listPatterns)
		dlp.POST("/patterns", h.createPattern)
		dlp.GET("/patterns/:id", h.getPattern)
		dlp.PUT("/patterns/:id", h.updatePattern)
		dlp.DELETE("/patterns/:id", h.deletePattern)

		// 违规记录
		dlp.GET("/violations", h.getViolations)

		// 传输阻断
		dlp.POST("/block", h.blockTransfer)

		// 统计信息
		dlp.GET("/stats", h.getStats)

		// 配置
		dlp.GET("/config", h.getConfig)
		dlp.PUT("/config", h.updateConfig)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// scanContent 扫描内容
func (h *Handlers) scanContent(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.ScanContent(&req)
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
		Data:    result,
	})
}

// scanFile 扫描文件
func (h *Handlers) scanFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid file: " + err.Error(),
		})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: "failed to read file: " + err.Error(),
		})
		return
	}

	req := &ScanRequest{
		Content:     content,
		Resource:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		UserID:      c.Query("user_id"),
	}

	result, err := h.manager.ScanContent(req)
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
		Data:    result,
	})
}

// listPolicies 列出策略
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policies,
	})
}

// createPolicy 创建策略
func (h *Handlers) createPolicy(c *gin.Context) {
	var req DLPPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	policy, err := h.manager.SetPolicy(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "policy created",
		Data:    policy,
	})
}

// getPolicy 获取策略
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
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
		Data:    policy,
	})
}

// updatePolicy 更新策略
func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req DLPPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	policy, err := h.manager.UpdatePolicy(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy updated",
		Data:    policy,
	})
}

// deletePolicy 删除策略
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy deleted",
	})
}

// listPatterns 列出模式
func (h *Handlers) listPatterns(c *gin.Context) {
	patterns := h.manager.ListPatterns()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    patterns,
	})
}

// createPattern 创建模式
func (h *Handlers) createPattern(c *gin.Context) {
	var req SensitivePattern
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pattern, err := h.manager.CreatePattern(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "pattern created",
		Data:    pattern,
	})
}

// getPattern 获取模式
func (h *Handlers) getPattern(c *gin.Context) {
	id := c.Param("id")
	pattern, err := h.manager.GetPattern(id)
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
		Data:    pattern,
	})
}

// updatePattern 更新模式
func (h *Handlers) updatePattern(c *gin.Context) {
	id := c.Param("id")
	var req SensitivePattern
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pattern, err := h.manager.UpdatePattern(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "pattern updated",
		Data:    pattern,
	})
}

// deletePattern 删除模式
func (h *Handlers) deletePattern(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePattern(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "pattern deleted",
	})
}

// getViolations 获取违规记录
func (h *Handlers) getViolations(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	level := SensitivityLevel(c.Query("level"))
	userID := c.Query("user_id")

	violations := h.manager.GetViolations(limit, level, userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    violations,
	})
}

// blockTransfer 阻断传输
func (h *Handlers) blockTransfer(c *gin.Context) {
	var req struct {
		Resource string `json:"resource" binding:"required"`
		UserID   string `json:"user_id" binding:"required"`
		Reason   string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.BlockTransfer(req.Resource, req.UserID, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "transfer blocked",
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg DLPConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}
