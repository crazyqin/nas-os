package lock

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 锁API处理器
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建API处理器
func NewHandlers(manager *Manager, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	locks := rg.Group("/locks")
	{
		// 锁操作
		locks.POST("", h.acquireLock)       // 获取锁
		locks.DELETE("/:id", h.releaseLock) // 释放锁
		locks.PUT("/:id/extend", h.extendLock) // 延长锁

		// 锁查询
		locks.GET("/:id", h.getLock)        // 获取锁详情
		locks.GET("/path/*path", h.getLockByPath) // 通过路径获取锁
		locks.GET("", h.listLocks)          // 列出所有锁

		// 锁检查
		locks.GET("/check/*path", h.checkLock) // 检查文件锁定状态

		// 管理操作
		locks.DELETE("/:id/force", h.forceReleaseLock) // 强制释放锁

		// 统计
		locks.GET("/stats", h.getStats) // 获取统计信息

		// 用户锁列表
		locks.GET("/owner/:owner", h.listLocksByOwner) // 获取用户的所有锁
	}
}

// acquireLockRequest 获取锁请求
type acquireLockRequest struct {
	FilePath  string            `json:"filePath" binding:"required"`
	LockType  string            `json:"lockType"` // "shared" or "exclusive"
	Owner     string            `json:"owner" binding:"required"`
	OwnerName string            `json:"ownerName"`
	ClientID  string            `json:"clientId"`
	Protocol  string            `json:"protocol"`
	Timeout   int               `json:"timeout"` // 秒
	Metadata  map[string]string `json:"metadata"`
}

// extendLockRequest 延长锁请求
type extendLockRequest struct {
	Duration int `json:"duration"` // 秒
}

// acquireLock 获取锁
// @Summary 获取文件锁
// @Description 对指定文件获取共享锁或独占锁
// @Tags locks
// @Accept json
// @Produce json
// @Param request body acquireLockRequest true "锁请求参数"
// @Success 200 {object} LockResponse "成功获取锁"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 409 {object} LockConflictResponse "锁冲突"
// @Router /locks [post]
func (h *Handlers) acquireLock(c *gin.Context) {
	var req acquireLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 解析锁类型
	var lockType LockType
	switch req.LockType {
	case "exclusive", "Exclusive":
		lockType = LockTypeExclusive
	case "shared", "Shared", "":
		lockType = LockTypeShared
	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    400,
			Message: "invalid lock type, must be 'shared' or 'exclusive'",
		})
		return
	}

	lockReq := &LockRequest{
		FilePath:  req.FilePath,
		LockType:  lockType,
		Owner:     req.Owner,
		OwnerName: req.OwnerName,
		ClientID:  req.ClientID,
		Protocol:  req.Protocol,
		Timeout:   req.Timeout,
		Metadata:  req.Metadata,
	}

	lock, conflict, err := h.manager.Lock(lockReq)
	if err != nil {
		if err == ErrLockConflict && conflict != nil {
			c.JSON(http.StatusConflict, LockConflictResponse{
				Code:    409,
				Message: conflict.Message,
				Data:    conflict.ExistingLock,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LockResponse{
		Code:    0,
		Message: "lock acquired",
		Data:    lock.ToInfo(),
	})
}

// releaseLock 释放锁
// @Summary 释放文件锁
// @Description 释放指定的文件锁
// @Tags locks
// @Accept json
// @Produce json
// @Param id path string true "锁ID"
// @Param owner query string true "锁持有者"
// @Success 200 {object} SuccessResponse "成功释放锁"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "锁不存在"
// @Failure 403 {object} ErrorResponse "非锁持有者"
// @Router /locks/{id} [delete]
func (h *Handlers) releaseLock(c *gin.Context) {
	lockID := c.Param("id")
	owner := c.Query("owner")

	if owner == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    400,
			Message: "owner is required",
		})
		return
	}

	err := h.manager.Unlock(lockID, owner)
	if err != nil {
		switch err {
		case ErrLockNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:    404,
				Message: "lock not found",
			})
		case ErrNotLockOwner:
			c.JSON(http.StatusForbidden, ErrorResponse{
				Code:    403,
				Message: "not lock owner",
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Code:    500,
				Message: err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Code:    0,
		Message: "lock released",
	})
}

// forceReleaseLock 强制释放锁
// @Summary 强制释放锁
// @Description 管理员强制释放指定的锁
// @Tags locks
// @Accept json
// @Produce json
// @Param id path string true "锁ID"
// @Success 200 {object} SuccessResponse "成功释放锁"
// @Failure 404 {object} ErrorResponse "锁不存在"
// @Router /locks/{id}/force [delete]
func (h *Handlers) forceReleaseLock(c *gin.Context) {
	lockID := c.Param("id")

	err := h.manager.ForceUnlock(lockID)
	if err != nil {
		if err == ErrLockNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:    404,
				Message: "lock not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Code:    0,
		Message: "lock force released",
	})
}

// extendLock 延长锁
// @Summary 延长锁有效期
// @Description 延长指定锁的有效期
// @Tags locks
// @Accept json
// @Produce json
// @Param id path string true "锁ID"
// @Param request body extendLockRequest true "延长时间参数"
// @Success 200 {object} LockResponse "成功延长"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "锁不存在"
// @Failure 403 {object} ErrorResponse "非锁持有者"
// @Router /locks/{id}/extend [put]
func (h *Handlers) extendLock(c *gin.Context) {
	lockID := c.Param("id")
	owner := c.Query("owner")

	var req extendLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if req.Duration <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    400,
			Message: "duration must be positive",
		})
		return
	}

	duration := time.Duration(req.Duration) * time.Second

	err := h.manager.ExtendLock(lockID, owner, duration)
	if err != nil {
		switch err {
		case ErrLockNotFound:
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:    404,
				Message: "lock not found",
			})
		case ErrNotLockOwner:
			c.JSON(http.StatusForbidden, ErrorResponse{
				Code:    403,
				Message: "not lock owner",
			})
		case ErrLockExpired:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:    400,
				Message: "lock has expired",
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Code:    500,
				Message: err.Error(),
			})
		}
		return
	}

	// 获取更新后的锁信息
	info, _ := h.manager.GetLock(lockID)

	c.JSON(http.StatusOK, LockResponse{
		Code:    0,
		Message: "lock extended",
		Data:    info,
	})
}

// getLock 获取锁详情
// @Summary 获取锁详情
// @Description 根据锁ID获取锁的详细信息
// @Tags locks
// @Accept json
// @Produce json
// @Param id path string true "锁ID"
// @Success 200 {object} LockResponse "锁详情"
// @Failure 404 {object} ErrorResponse "锁不存在"
// @Router /locks/{id} [get]
func (h *Handlers) getLock(c *gin.Context) {
	lockID := c.Param("id")

	info, err := h.manager.GetLock(lockID)
	if err != nil {
		if err == ErrLockNotFound || err == ErrLockExpired {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:    404,
				Message: "lock not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LockResponse{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// getLockByPath 通过路径获取锁
// @Summary 通过路径获取锁
// @Description 根据文件路径获取锁的详细信息
// @Tags locks
// @Accept json
// @Produce json
// @Param path path string true "文件路径"
// @Success 200 {object} LockResponse "锁详情"
// @Failure 404 {object} ErrorResponse "锁不存在"
// @Router /locks/path/{path} [get]
func (h *Handlers) getLockByPath(c *gin.Context) {
	filePath := c.Param("path")

	// 确保路径以 / 开头
	if len(filePath) > 0 && filePath[0] != '/' {
		filePath = "/" + filePath
	}

	info, err := h.manager.GetLockByPath(filePath)
	if err != nil {
		if err == ErrLockNotFound || err == ErrLockExpired {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:    404,
				Message: "lock not found for path: " + filePath,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LockResponse{
		Code:    0,
		Message: "success",
		Data:    info,
	})
}

// listLocks 列出所有锁
// @Summary 列出所有锁
// @Description 获取所有活跃锁的列表
// @Tags locks
// @Accept json
// @Produce json
// @Param owner query string false "按持有者过滤"
// @Param type query string false "按锁类型过滤 (shared/exclusive)"
// @Param protocol query string false "按协议过滤"
// @Success 200 {object} LockListResponse "锁列表"
// @Router /locks [get]
func (h *Handlers) listLocks(c *gin.Context) {
	filter := &LockFilter{
		Owner:   c.Query("owner"),
		Protocol: c.Query("protocol"),
	}

	// 解析锁类型
	if lockType := c.Query("type"); lockType != "" {
		switch lockType {
		case "shared", "Shared":
			filter.LockType = LockTypeShared
		case "exclusive", "Exclusive":
			filter.LockType = LockTypeExclusive
		}
	}

	locks := h.manager.ListLocks(filter)

	c.JSON(http.StatusOK, LockListResponse{
		Code:    0,
		Message: "success",
		Data:    locks,
		Total:   len(locks),
	})
}

// checkLock 检查锁定状态
// @Summary 检查文件锁定状态
// @Description 检查指定文件是否被锁定
// @Tags locks
// @Accept json
// @Produce json
// @Param path path string true "文件路径"
// @Success 200 {object} LockStatusResponse "锁定状态"
// @Router /locks/check/{path} [get]
func (h *Handlers) checkLock(c *gin.Context) {
	filePath := c.Param("path")

	// 确保路径以 / 开头
	if len(filePath) > 0 && filePath[0] != '/' {
		filePath = "/" + filePath
	}

	locked := h.manager.IsLocked(filePath)

	response := LockStatusResponse{
		Code:    0,
		Message: "success",
		Data: LockStatusData{
			FilePath: filePath,
			IsLocked: locked,
		},
	}

	// 如果被锁定，获取锁信息
	if locked {
		info, err := h.manager.GetLockByPath(filePath)
		if err == nil {
			response.Data.Lock = info
		}
	}

	c.JSON(http.StatusOK, response)
}

// listLocksByOwner 获取用户的所有锁
// @Summary 获取用户的所有锁
// @Description 获取指定用户持有的所有锁
// @Tags locks
// @Accept json
// @Produce json
// @Param owner path string true "用户标识"
// @Success 200 {object} LockListResponse "锁列表"
// @Router /locks/owner/{owner} [get]
func (h *Handlers) listLocksByOwner(c *gin.Context) {
	owner := c.Param("owner")

	locks := h.manager.ListLocksByOwner(owner)

	c.JSON(http.StatusOK, LockListResponse{
		Code:    0,
		Message: "success",
		Data:    locks,
		Total:   len(locks),
	})
}

// getStats 获取统计信息
// @Summary 获取锁统计信息
// @Description 获取锁管理器的统计信息
// @Tags locks
// @Accept json
// @Produce json
// @Success 200 {object} StatsResponse "统计信息"
// @Router /locks/stats [get]
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.Stats()

	c.JSON(http.StatusOK, StatsResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// 响应类型定义

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LockResponse 锁响应
type LockResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    *LockInfo `json:"data,omitempty"`
}

// LockListResponse 锁列表响应
type LockListResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    []*LockInfo `json:"data"`
	Total   int         `json:"total"`
}

// LockConflictResponse 锁冲突响应
type LockConflictResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    *LockInfo `json:"data"`
}

// LockStatusResponse 锁状态响应
type LockStatusResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    LockStatusData `json:"data"`
}

// LockStatusData 锁状态数据
type LockStatusData struct {
	FilePath string    `json:"filePath"`
	IsLocked bool      `json:"isLocked"`
	Lock     *LockInfo `json:"lock,omitempty"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    ManagerStats `json:"data"`
}