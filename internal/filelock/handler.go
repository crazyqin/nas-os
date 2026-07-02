// Package filelock 提供文件锁定功能。
// HTTP handler 层：提供 REST API 用于锁定/解锁文件及查看锁定列表。
package filelock

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 文件锁 HTTP 处理器.
type Handler struct {
	service *Service
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
// 路由组路径建议: /api/v1/filelock.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/filelock")
	{
		// 锁定文件
		group.POST("/lock", h.lock)
		// 解锁文件
		group.POST("/unlock", h.unlock)
		// 查看锁定列表
		group.GET("/list", h.list)
		// 获取锁详情
		group.GET("/locks/:id", h.getLock)
		// 检查文件是否被锁定
		group.GET("/check/*filepath", h.checkLock)
		// 获取统计信息
		group.GET("/stats", h.getStats)
		// 获取/更新配置
		group.GET("/config", h.getConfig)
		group.PUT("/config", h.updateConfig)
	}
}

// response 标准响应.
type response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// lock 锁定文件
// POST /api/v1/filelock/lock.
func (h *Handler) lock(c *gin.Context) {
	var req LockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	lock, err := h.service.Lock(&req)
	if err != nil {
		c.JSON(http.StatusConflict, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "文件锁定成功",
		Data: &LockResponse{
			Lock:    lock,
			Success: true,
		},
	})
}

// unlock 解锁文件
// POST /api/v1/filelock/unlock.
func (h *Handler) unlock(c *gin.Context) {
	var req UnlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	count, err := h.service.Unlock(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "文件解锁成功",
		Data: &UnlockResponse{
			ReleasedCount: count,
			Success:       true,
		},
	})
}

// list 查看锁定列表
// GET /api/v1/filelock/list.
func (h *Handler) list(c *gin.Context) {
	// 支持可选过滤参数
	userID := c.Query("user_id")
	filePath := c.Query("file_path")

	var data any
	if userID != "" {
		data = &ListLocksResponse{
			Locks: h.service.ListByUser(userID),
			Total: len(h.service.ListByUser(userID)),
		}
	} else if filePath != "" {
		data = &ListLocksResponse{
			Locks: h.service.ListByFile(filePath),
			Total: len(h.service.ListByFile(filePath)),
		}
	} else {
		data = h.service.List()
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// getLock 获取锁详情
// GET /api/v1/filelock/locks/:id.
func (h *Handler) getLock(c *gin.Context) {
	id := c.Param("id")
	lock, err := h.service.GetLock(id)
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
		Data:    lock,
	})
}

// checkLock 检查文件是否被锁定
// GET /api/v1/filelock/check/*filepath.
func (h *Handler) checkLock(c *gin.Context) {
	filePath := c.Param("filepath")
	locked := h.service.IsFileLocked(filePath)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]any{
			"file_path": filePath,
			"locked":    locked,
		},
	})
}

// getStats 获取统计信息
// GET /api/v1/filelock/stats.
func (h *Handler) getStats(c *gin.Context) {
	stats := h.service.GetStats()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getConfig 获取配置
// GET /api/v1/filelock/config.
func (h *Handler) getConfig(c *gin.Context) {
	cfg := h.service.GetConfig()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
// PUT /api/v1/filelock/config.
func (h *Handler) updateConfig(c *gin.Context) {
	var cfg Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	h.service.UpdateConfig(&cfg)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "配置已更新",
	})
}
