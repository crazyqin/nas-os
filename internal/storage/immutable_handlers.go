// Package storage 提供 WriteOnce 不可变存储 API 处理器
package storage

import (
	"net/http"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ImmutableHandlers 不可变存储 API 处理器.
type ImmutableHandlers struct {
	manager *ImmutableManager
}

// NewImmutableHandlers 创建处理器.
func NewImmutableHandlers(manager *ImmutableManager) *ImmutableHandlers {
	return &ImmutableHandlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *ImmutableHandlers) RegisterRoutes(r *gin.RouterGroup) {
	immutable := r.Group("/immutable")
	{
		// 记录管理
		immutable.GET("", h.listRecords)
		immutable.GET("/:id", h.getRecord)
		immutable.POST("", h.lockPath)
		immutable.DELETE("/:id", h.unlockPath)
		immutable.POST("/:id/extend", h.extendLock)
		immutable.POST("/:id/restore", h.restoreSnapshot)

		// 状态查询
		immutable.GET("/status", h.getPathStatus)
		immutable.GET("/statistics", h.getStatistics)
		immutable.POST("/check-ransomware", h.checkRansomwareProtection)

		// 快捷操作
		immutable.POST("/quick-lock", h.quickLock)
		immutable.POST("/batch-lock", h.batchLock)
	}
}

// ========== 记录管理 ==========

// listRecordsRequest 列表请求.
type listRecordsRequest struct {
	Status      string `form:"status"`
	VolumeName  string `form:"volumeName"`
	PathContain string `form:"pathContains"`
	CreatedBy   string `form:"createdBy"`
}

// listRecords 列出不可变记录
// @Summary 列出不可变记录
// @Description 获取所有不可变存储记录列表
// @Tags immutable
// @Produce json
// @Param status query string false "状态过滤 (active/expired/unlocked)"
// @Param volumeName query string false "卷名过滤"
// @Param pathContains query string false "路径包含过滤"
// @Param createdBy query string false "创建者过滤"
// @Success 200 {object} api.Response{data=[]ImmutableRecord}
// @Router /immutable [get].
func (h *ImmutableHandlers) listRecords(c *gin.Context) {
	var req listRecordsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		api.BadRequest(c, "无效的请求参数")
		return
	}

	var filter *RecordFilter
	if req.Status != "" || req.VolumeName != "" || req.PathContain != "" || req.CreatedBy != "" {
		filter = &RecordFilter{
			VolumeName:   req.VolumeName,
			PathContains: req.PathContain,
			CreatedBy:    req.CreatedBy,
		}
		if req.Status != "" {
			status := ImmutableStatus(req.Status)
			filter.Status = &status
		}
	}

	records := h.manager.ListRecords(filter)
	api.OK(c, records)
}

// getRecord 获取记录详情
// @Summary 获取不可变记录详情
// @Description 获取指定不可变存储记录的详细信息
// @Tags immutable
// @Produce json
// @Param id path string true "记录 ID"
// @Success 200 {object} api.Response{data=ImmutableRecord}
// @Failure 404 {object} api.Response
// @Router /immutable/{id} [get].
func (h *ImmutableHandlers) getRecord(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "记录 ID 不能为空")
		return
	}

	record, err := h.manager.GetRecord(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, record)
}

// lockPathRequest 锁定请求.
type lockPathRequest struct {
	Path                  string   `json:"path" binding:"required"`
	Duration              string   `json:"duration" binding:"required"` // 7d, 30d, permanent
	Description           string   `json:"description"`
	Tags                  []string `json:"tags"`
	CreatedBy             string   `json:"createdBy"`
	ProtectFromRansomware bool     `json:"protectFromRansomware"`
}

// lockPathResponse 锁定响应.
type lockPathResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	LockedAt     string `json:"lockedAt"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
	SnapshotPath string `json:"snapshotPath"`
}

// lockPath 锁定路径
// @Summary 锁定路径
// @Description 创建不可变快照，锁定指定路径
// @Tags immutable
// @Accept json
// @Produce json
// @Param request body lockPathRequest true "锁定请求"
// @Success 201 {object} api.Response{data=lockPathResponse}
// @Failure 400 {object} api.Response
// @Router /immutable [post].
func (h *ImmutableHandlers) lockPath(c *gin.Context) {
	var req lockPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}

	record, err := h.manager.Lock(LockRequest{
		Path:                  req.Path,
		Duration:              LockDuration(req.Duration),
		Description:           req.Description,
		Tags:                  req.Tags,
		CreatedBy:             req.CreatedBy,
		ProtectFromRansomware: req.ProtectFromRansomware,
	})
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusCreated, api.Response{
		Code:    0,
		Message: "锁定成功",
		Data: lockPathResponse{
			ID:           record.ID,
			Status:       string(record.Status),
			LockedAt:     record.LockedAt.Format("2006-01-02 15:04:05"),
			ExpiresAt:    record.ExpiresAt.Format("2006-01-02 15:04:05"),
			SnapshotPath: record.SnapshotPath,
		},
	})
}

// unlockPathRequest 解锁请求.
type unlockPathRequest struct {
	Force bool `json:"force"` // 强制解锁（管理授权）
}

// unlockPath 解锁路径
// @Summary 解锁路径
// @Description 解锁（删除）不可变快照
// @Tags immutable
// @Accept json
// @Produce json
// @Param id path string true "记录 ID"
// @Param request body unlockPathRequest false "解锁请求"
// @Success 200 {object} api.Response{data=ImmutableRecord}
// @Failure 400 {object} api.Response
// @Router /immutable/{id} [delete].
func (h *ImmutableHandlers) unlockPath(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "记录 ID 不能为空")
		return
	}

	var req unlockPathRequest
	_ = c.ShouldBindJSON(&req) // 可选参数

	record, err := h.manager.Unlock(id, req.Force)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, record)
}

// extendLockRequest 延长锁定请求.
type extendLockRequest struct {
	Duration string `json:"duration" binding:"required"` // 7d, 30d, permanent
}

// extendLock 延长锁定时间
// @Summary 延长锁定时间
// @Description 延长不可变存储的锁定时间
// @Tags immutable
// @Accept json
// @Produce json
// @Param id path string true "记录 ID"
// @Param request body extendLockRequest true "延长请求"
// @Success 200 {object} api.Response{data=ImmutableRecord}
// @Failure 400 {object} api.Response
// @Router /immutable/{id}/extend [post].
func (h *ImmutableHandlers) extendLock(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "记录 ID 不能为空")
		return
	}

	var req extendLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数")
		return
	}

	record, err := h.manager.ExtendLock(id, LockDuration(req.Duration))
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, record)
}

// restoreSnapshotRequest 恢复请求.
type restoreSnapshotRequest struct {
	TargetPath string `json:"targetPath"` // 恢复目标路径（可选）
}

// restoreSnapshot 从不可变快照恢复
// @Summary 从不可变快照恢复
// @Description 从不可变快照创建可写副本
// @Tags immutable
// @Accept json
// @Produce json
// @Param id path string true "记录 ID"
// @Param request body restoreSnapshotRequest false "恢复请求"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /immutable/{id}/restore [post].
func (h *ImmutableHandlers) restoreSnapshot(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "记录 ID 不能为空")
		return
	}

	var req restoreSnapshotRequest
	_ = c.ShouldBindJSON(&req) // 可选参数

	if err := h.manager.Restore(id, req.TargetPath); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{"message": "恢复成功"})
}

// ========== 状态查询 ==========

// getPathStatusRequest 状态查询请求.
type getPathStatusRequest struct {
	Path string `form:"path" binding:"required"`
}

// getPathStatus 获取路径锁定状态
// @Summary 获取路径锁定状态
// @Description 查询指定路径的不可变锁定状态
// @Tags immutable
// @Produce json
// @Param path query string true "路径"
// @Success 200 {object} api.Response{data=ImmutableRecord}
// @Router /immutable/status [get].
func (h *ImmutableHandlers) getPathStatus(c *gin.Context) {
	var req getPathStatusRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		api.BadRequest(c, "路径不能为空")
		return
	}

	record, err := h.manager.GetStatus(req.Path)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	if record == nil {
		api.OK(c, gin.H{
			"locked":  false,
			"message": "路径未被锁定",
		})
		return
	}

	api.OK(c, gin.H{
		"locked": true,
		"record": record,
	})
}

// getStatistics 获取统计信息
// @Summary 获取不可变存储统计
// @Description 获取不可变存储的统计信息
// @Tags immutable
// @Produce json
// @Success 200 {object} api.Response{data=ImmutableStatistics}
// @Router /immutable/statistics [get].
func (h *ImmutableHandlers) getStatistics(c *gin.Context) {
	stats := h.manager.GetStatistics()
	api.OK(c, stats)
}

// checkRansomwareProtectionRequest 防勒索检查请求.
type checkRansomwareProtectionRequest struct {
	Path string `json:"path" binding:"required"`
}

// checkRansomwareProtection 检查防勒索保护状态
// @Summary 检查防勒索保护状态
// @Description 检查指定路径是否受防勒索保护
// @Tags immutable
// @Accept json
// @Produce json
// @Param request body checkRansomwareProtectionRequest true "检查请求"
// @Success 200 {object} api.Response{data=RansomwareProtectionStatus}
// @Router /immutable/check-ransomware [post].
func (h *ImmutableHandlers) checkRansomwareProtection(c *gin.Context) {
	var req checkRansomwareProtectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "路径不能为空")
		return
	}

	status, err := h.manager.CheckRansomwareProtection(req.Path)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, status)
}

// ========== 快捷操作 ==========

// quickLockRequest 快速锁定请求.
type quickLockRequest struct {
	Path     string `json:"path" binding:"required"`
	Duration string `json:"duration" binding:"required"` // 7d, 30d, permanent
}

// quickLock 快速锁定
// @Summary 快速锁定
// @Description 使用默认配置快速锁定路径（启用防勒索保护）
// @Tags immutable
// @Accept json
// @Produce json
// @Param request body quickLockRequest true "快速锁定请求"
// @Success 201 {object} api.Response{data=ImmutableRecord}
// @Router /immutable/quick-lock [post].
func (h *ImmutableHandlers) quickLock(c *gin.Context) {
	var req quickLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数")
		return
	}

	record, err := h.manager.QuickLock(req.Path, LockDuration(req.Duration))
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusCreated, api.Response{
		Code:    0,
		Message: "快速锁定成功",
		Data:    record,
	})
}

// batchLockRequest 批量锁定请求.
type batchLockRequest struct {
	Paths       []string `json:"paths" binding:"required"`
	Duration    string   `json:"duration" binding:"required"`
	Description string   `json:"description"`
}

// batchLockResponse 批量锁定响应.
type batchLockResponse struct {
	SuccessCount int                `json:"successCount"`
	FailedCount  int                `json:"failedCount"`
	Records      []*ImmutableRecord `json:"records"`
	Errors       []string           `json:"errors,omitempty"`
}

// batchLock 批量锁定
// @Summary 批量锁定
// @Description 批量锁定多个路径
// @Tags immutable
// @Accept json
// @Produce json
// @Param request body batchLockRequest true "批量锁定请求"
// @Success 200 {object} api.Response{data=batchLockResponse}
// @Router /immutable/batch-lock [post].
func (h *ImmutableHandlers) batchLock(c *gin.Context) {
	var req batchLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数")
		return
	}

	if len(req.Paths) == 0 {
		api.BadRequest(c, "路径列表不能为空")
		return
	}

	if len(req.Paths) > 100 {
		api.BadRequest(c, "单次批量操作最多 100 个路径")
		return
	}

	records, errs := h.manager.BatchLock(req.Paths, LockDuration(req.Duration), req.Description)

	errMsgs := make([]string, 0, len(errs))
	for _, e := range errs {
		errMsgs = append(errMsgs, e.Error())
	}

	api.OK(c, batchLockResponse{
		SuccessCount: len(records),
		FailedCount:  len(errs),
		Records:      records,
		Errors:       errMsgs,
	})
}
