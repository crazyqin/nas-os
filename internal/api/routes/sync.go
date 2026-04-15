// Package routes 提供 Cloud Drive Sync REST API 端点
// v2.384.0 - 户部实现
package routes

import (
	"encoding/json"
	"time"

	"nas-os/internal/api"
	"nas-os/internal/db/models"

	"github.com/gin-gonic/gin"
)

// SyncHandlers 同步任务 API 处理器.
type SyncHandlers struct {
	store *models.SyncStore
}

// NewSyncHandlers 创建同步 API 处理器.
func NewSyncHandlers(store *models.SyncStore) *SyncHandlers {
	return &SyncHandlers{store: store}
}

// RegisterRoutes 注册同步相关路由.
func (h *SyncHandlers) RegisterRoutes(r *gin.RouterGroup) {
	sync := r.Group("/sync")
	{
		// 同步任务 CRUD
		sync.POST("/tasks", h.CreateSyncTask)
		sync.GET("/tasks", h.ListSyncTasks)
		sync.GET("/tasks/:id", h.GetSyncTask)
		sync.PATCH("/tasks/:id", h.UpdateSyncTask)
		sync.DELETE("/tasks/:id", h.DeleteSyncTask)

		// 同步操作
		sync.POST("/tasks/:id/trigger", h.TriggerSync)

		// 同步历史
		sync.GET("/tasks/:id/history", h.GetSyncHistory)
	}
}

// ==================== 同步任务 CRUD ====================

// CreateSyncTask 创建同步任务
// @Summary 创建同步任务
// @Description 创建一个新的云盘同步任务
// @Tags sync
// @Accept json
// @Produce json
// @Param request body models.CreateSyncTaskRequest true "同步任务参数"
// @Success 201 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /api/sync/tasks [post].
func (h *SyncHandlers) CreateSyncTask(c *gin.Context) {
	var req models.CreateSyncTaskRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	task := &models.SyncTask{
		Name:             req.Name,
		ProviderID:       req.ProviderID,
		ProviderType:     req.ProviderType,
		Status:           models.SyncTaskStatusActive,
		LocalPath:        req.LocalPath,
		RemotePath:       req.RemotePath,
		Direction:        req.Direction,
		ConflictStrategy: req.ConflictStrategy,
		ScheduleType:     req.ScheduleType,
		ScheduleExpr:     req.ScheduleExpr,
		IncludePatterns:  stringsToJSON(req.IncludePatterns),
		ExcludePatterns:  stringsToJSON(req.ExcludePatterns),
		MaxFileSize:      req.MaxFileSize,
		DeleteRemote:     req.DeleteRemote,
		PreserveModTime:  req.PreserveModTime,
		ChecksumVerify:   req.ChecksumVerify,
		EncryptEnabled:   req.EncryptEnabled,
		BandwidthLimit:   req.BandwidthLimit,
		MaxVersions:      req.MaxVersions,
	}

	// 从上下文获取用户ID（如果存在）
	if userID, exists := c.Get("userID"); exists {
		if uid, ok := userID.(string); ok {
			task.UserID = uid
		}
	}

	if err := h.store.CreateSyncTask(task); err != nil {
		api.InternalError(c, "创建同步任务失败: "+err.Error())
		return
	}

	api.Created(c, task)
}

// ListSyncTasks 列出同步任务
// @Summary 列出同步任务
// @Description 分页列出所有同步任务
// @Tags sync
// @Produce json
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param status query string false "状态筛选"
// @Param sortBy query string false "排序字段" default(created_at)
// @Param sortDir query string false "排序方向" default(desc)
// @Success 200 {object} api.Response
// @Router /api/sync/tasks [get].
func (h *SyncHandlers) ListSyncTasks(c *gin.Context) {
	var query models.ListSyncTasksQuery
	if err := api.BindQueryAndValidate(c, &query); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	tasks, total, err := h.store.ListSyncTasks(query)
	if err != nil {
		api.InternalError(c, "查询同步任务失败: "+err.Error())
		return
	}

	api.Page(c, tasks, total, query.Page, query.PageSize)
}

// GetSyncTask 获取同步任务详情
// @Summary 获取同步任务详情
// @Description 根据ID获取同步任务详细信息
// @Tags sync
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/sync/tasks/{id} [get].
func (h *SyncHandlers) GetSyncTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "任务ID不能为空")
		return
	}

	task, err := h.store.GetSyncTask(id)
	if err != nil {
		api.NotFound(c, "同步任务不存在")
		return
	}

	api.OK(c, task)
}

// UpdateSyncTask 更新同步任务
// @Summary 更新同步任务
// @Description 部分更新同步任务配置
// @Tags sync
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body models.UpdateSyncTaskRequest true "更新参数"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/sync/tasks/{id} [patch].
func (h *SyncHandlers) UpdateSyncTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "任务ID不能为空")
		return
	}

	var req models.UpdateSyncTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "无效的请求参数: "+err.Error())
		return
	}

	if err := h.store.UpdateSyncTask(id, &req); err != nil {
		if err.Error() == "没有需要更新的字段" {
			api.BadRequest(c, err.Error())
			return
		}
		api.NotFound(c, err.Error())
		return
	}

	// 返回更新后的任务
	task, err := h.store.GetSyncTask(id)
	if err != nil {
		api.OKWithMessage(c, "同步任务更新成功", nil)
		return
	}

	api.OKWithMessage(c, "同步任务更新成功", task)
}

// DeleteSyncTask 删除同步任务
// @Summary 删除同步任务
// @Description 软删除指定同步任务
// @Tags sync
// @Param id path string true "任务ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/sync/tasks/{id} [delete].
func (h *SyncHandlers) DeleteSyncTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "任务ID不能为空")
		return
	}

	if err := h.store.DeleteSyncTask(id); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OKWithMessage(c, "同步任务已删除", nil)
}

// ==================== 同步操作 ====================

// TriggerSync 手动触发同步
// @Summary 手动触发同步
// @Description 立即触发指定同步任务的执行
// @Tags sync
// @Produce json
// @Param id path string true "任务ID"
// @Success 202 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 409 {object} api.Response
// @Router /api/sync/tasks/{id}/trigger [post].
func (h *SyncHandlers) TriggerSync(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "任务ID不能为空")
		return
	}

	task, err := h.store.GetSyncTask(id)
	if err != nil {
		api.NotFound(c, "同步任务不存在")
		return
	}

	// 检查任务状态
	if task.Status == models.SyncTaskStatusPaused {
		api.Conflict(c, "任务已暂停，请先恢复任务")
		return
	}
	if task.Status == models.SyncTaskStatusDisabled {
		api.Conflict(c, "任务已禁用，请先启用任务")
		return
	}

	// 创建同步历史记录
	history := &models.SyncHistory{
		TaskID:      id,
		Status:      models.HistoryStatusRunning,
		StartedAt:   time.Now(),
		TriggerType: "manual",
	}
	if err := h.store.CreateSyncHistory(history); err != nil {
		api.InternalError(c, "创建同步记录失败: "+err.Error())
		return
	}

	// TODO: 实际的同步引擎调用 —— 这里由同步引擎异步处理
	// go syncEngine.Run(task, history)

	api.Accepted(c, gin.H{
		"taskId":    id,
		"historyId": history.ID,
		"status":    "triggered",
		"message":   "同步任务已触发",
	})
}

// ==================== 同步历史 ====================

// GetSyncHistory 获取同步历史记录
// @Summary 获取同步历史记录
// @Description 分页列出指定同步任务的执行历史
// @Tags sync
// @Produce json
// @Param id path string true "任务ID"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param status query string false "状态筛选"
// @Param sortBy query string false "排序字段" default(started_at)
// @Param sortDir query string false "排序方向" default(desc)
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/sync/tasks/{id}/history [get].
func (h *SyncHandlers) GetSyncHistory(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		api.BadRequest(c, "任务ID不能为空")
		return
	}

	// 验证任务存在
	if _, err := h.store.GetSyncTask(taskID); err != nil {
		api.NotFound(c, "同步任务不存在")
		return
	}

	var query models.ListSyncHistoryQuery
	if err := api.BindQueryAndValidate(c, &query); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	histories, total, err := h.store.ListSyncHistory(taskID, query)
	if err != nil {
		api.InternalError(c, "查询同步历史失败: "+err.Error())
		return
	}

	api.Page(c, histories, total, query.Page, query.PageSize)
}

// ==================== 辅助函数 ====================

// stringsToJSON 将字符串切片转为 JSON 数组字符串.
func stringsToJSON(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(ss)
	return string(b)
}
