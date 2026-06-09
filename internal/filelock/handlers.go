// Package filelock 提供 REST API 处理器
package filelock

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 文件锁 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	locks := r.Group("/locks")
	{
		// 获取锁列表
		locks.GET("", h.listLocks)
		// 获取锁详情
		locks.GET("/:id", h.getLock)
		// 获取锁
		locks.POST("/acquire", h.acquireLock)
		// 释放锁
		locks.POST("/release", h.releaseLock)
		// 续期锁
		locks.POST("/renew", h.renewLock)
		// 管理员强制释放
		locks.POST("/force-release", h.forceReleaseLock)
		// 检查文件是否锁定
		locks.GET("/check/*filepath", h.checkFileLock)
		// 获取文件锁详情
		locks.GET("/file/*filepath", h.getFileLocks)
		// 获取统计信息
		locks.GET("/stats", h.getStats)
		// 获取历史记录
		locks.GET("/history", h.getHistory)
		// 获取策略
		locks.GET("/policy", h.getPolicy)
		// 更新策略
		locks.PUT("/policy", h.updatePolicy)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listLocks 列出锁
func (h *Handlers) listLocks(c *gin.Context) {
	req := &ListLocksRequest{
		FilePath: c.Query("file_path"),
		UserID:   c.Query("user_id"),
		Status:   LockStatus(c.Query("status")),
		LockType: LockType(c.Query("lock_type")),
	}

	if page, err := strconv.Atoi(c.Query("page")); err == nil {
		req.Page = page
	}
	if pageSize, err := strconv.Atoi(c.Query("page_size")); err == nil {
		req.PageSize = pageSize
	}

	locks, total := h.manager.ListLocks(req)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"locks": locks,
			"total": total,
			"page":  req.Page,
		},
	})
}

// getLock 获取锁详情
func (h *Handlers) getLock(c *gin.Context) {
	id := c.Param("id")
	lock, err := h.manager.GetLock(id)
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

// acquireLock 获取锁
func (h *Handlers) acquireLock(c *gin.Context) {
	var req AcquireRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	lock, err := h.manager.AcquireLock(&req)
	if err != nil {
		c.JSON(http.StatusConflict, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "锁已获取",
		Data:    lock,
	})
}

// releaseLock 释放锁
func (h *Handlers) releaseLock(c *gin.Context) {
	var req ReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.ReleaseLock(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "锁已释放",
	})
}

// renewLock 续期锁
func (h *Handlers) renewLock(c *gin.Context) {
	var req RenewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	lock, err := h.manager.RenewLock(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "锁已续期",
		Data:    lock,
	})
}

// forceReleaseLock 管理员强制释放锁
func (h *Handlers) forceReleaseLock(c *gin.Context) {
	var req ForceReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.ForceReleaseLock(&req); err != nil {
		c.JSON(http.StatusForbidden, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "锁已强制释放",
	})
}

// checkFileLock 检查文件是否锁定
func (h *Handlers) checkFileLock(c *gin.Context) {
	filePath := c.Param("filepath")
	locked := h.manager.IsFileLocked(filePath)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"file_path": filePath,
			"locked":    locked,
		},
	})
}

// getFileLocks 获取文件锁详情
func (h *Handlers) getFileLocks(c *gin.Context) {
	filePath := c.Param("filepath")
	locks := h.manager.GetFileLocks(filePath)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    locks,
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

// getHistory 获取历史记录
func (h *Handlers) getHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetHistory(limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// getPolicy 获取策略
func (h *Handlers) getPolicy(c *gin.Context) {
	policy := h.manager.GetPolicy()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// updatePolicy 更新策略
func (h *Handlers) updatePolicy(c *gin.Context) {
	var policy LockPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdatePolicy(&policy)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "策略已更新",
	})
}
