package downloadstation

import (
	"fmt"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 下载站 API 处理器.
type Handlers struct {
	manager    *Manager
	rssManager *RSSManager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager, rssManager *RSSManager) *Handlers {
	return &Handlers{
		manager:    manager,
		rssManager: rssManager,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	download := r.Group("/download")
	{
		// 下载任务管理
		download.GET("/tasks", h.listTasks)
		download.POST("/tasks", h.createTask)
		download.GET("/tasks/:id", h.getTask)
		download.PUT("/tasks/:id", h.updateTask)
		download.DELETE("/tasks/:id", h.deleteTask)

		// 任务控制
		download.POST("/tasks/:id/start", h.startTask)
		download.POST("/tasks/:id/pause", h.pauseTask)
		download.POST("/tasks/:id/resume", h.resumeTask)
		download.POST("/tasks/:id/cancel", h.cancelTask)

		// 批量操作
		download.POST("/tasks/batch/start", h.batchStart)
		download.POST("/tasks/batch/pause", h.batchPause)
		download.POST("/tasks/batch/delete", h.batchDelete)

		// 统计和历史
		download.GET("/stats", h.getStats)
		download.GET("/history", h.getHistory)
		download.DELETE("/history", h.clearHistory)

		// 队列配置
		download.GET("/config", h.getConfig)
		download.PUT("/config", h.updateConfig)

		// 速度统计
		download.GET("/speed-stats", h.getSpeedStats)

		// RSS 订阅管理
		rss := download.Group("/rss")
		{
			rss.GET("/feeds", h.listFeeds)
			rss.POST("/feeds", h.addFeed)
			rss.GET("/feeds/:id", h.getFeed)
			rss.PUT("/feeds/:id", h.updateFeed)
			rss.DELETE("/feeds/:id", h.deleteFeed)
			rss.POST("/feeds/:id/enable", h.enableFeed)
			rss.POST("/feeds/:id/refresh", h.refreshFeed)
			rss.GET("/feeds/:id/items", h.getFeedItems)
		}

		// 健康检查
		download.GET("/health", h.healthCheck)
	}
}

// ========== 下载任务管理 ==========

// listTasks 列出下载任务
// @Summary 列出下载任务
// @Description 获取所有下载任务列表
// @Tags download
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]DownloadTask}
// @Router /download/tasks [get]
// @Security BearerAuth.
func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	api.OK(c, tasks)
}

// createTask 创建下载任务
// @Summary 创建下载任务
// @Description 创建新的下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "创建任务请求"
// @Success 200 {object} api.Response{data=DownloadTask}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks [post]
// @Security BearerAuth.
func (h *Handlers) createTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	task, err := h.manager.CreateTask(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "下载任务创建成功", task)
}

// getTask 获取下载任务
// @Summary 获取下载任务详情
// @Description 获取指定下载任务的详细信息
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response{data=DownloadTask}
// @Failure 404 {object} api.Response
// @Router /download/tasks/{id} [get]
// @Security BearerAuth.
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetTask(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, task)
}

// updateTask 更新下载任务
// @Summary 更新下载任务
// @Description 更新指定的下载任务配置
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Param request body UpdateTaskRequest true "更新请求"
// @Success 200 {object} api.Response{data=DownloadTask}
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/{id} [put]
// @Security BearerAuth.
func (h *Handlers) updateTask(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	task, err := h.manager.UpdateTask(id, req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务更新成功", task)
}

// deleteTask 删除下载任务
// @Summary 删除下载任务
// @Description 删除指定的下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/{id} [delete]
// @Security BearerAuth.
func (h *Handlers) deleteTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteTask(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已删除", nil)
}

// ========== 任务控制 ==========

// startTask 开始下载任务
// @Summary 开始下载任务
// @Description 开始执行指定的下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/{id}/start [post]
// @Security BearerAuth.
func (h *Handlers) startTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StartTask(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已开始", nil)
}

// pauseTask 暂停下载任务
// @Summary 暂停下载任务
// @Description 暂停指定的下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/{id}/pause [post]
// @Security BearerAuth.
func (h *Handlers) pauseTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.PauseTask(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已暂停", nil)
}

// resumeTask 恢复下载任务
// @Summary 恢复下载任务
// @Description 恢复执行指定的下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/{id}/resume [post]
// @Security BearerAuth.
func (h *Handlers) resumeTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResumeTask(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已恢复", nil)
}

// cancelTask 取消下载任务
// @Summary 取消下载任务
// @Description 取消指定的下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "任务 ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/{id}/cancel [post]
// @Security BearerAuth.
func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTask(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "任务已取消", nil)
}

// ========== 批量操作 ==========

// batchStart 批量开始任务
// @Summary 批量开始任务
// @Description 批量开始多个下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param request body BatchRequest true "批量请求"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/batch/start [post]
// @Security BearerAuth.
func (h *Handlers) batchStart(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	errors := h.manager.BatchStart(req.TaskIDs)
	if len(errors) > 0 {
		api.InternalError(c, fmt.Sprintf("部分任务启动失败: %v", errors))
		return
	}

	api.OKWithMessage(c, "批量操作成功", nil)
}

// batchPause 批量暂停任务
// @Summary 批量暂停任务
// @Description 批量暂停多个下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param request body BatchRequest true "批量请求"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/batch/pause [post]
// @Security BearerAuth.
func (h *Handlers) batchPause(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	errors := h.manager.BatchPause(req.TaskIDs)
	if len(errors) > 0 {
		api.InternalError(c, fmt.Sprintf("部分任务暂停失败: %v", errors))
		return
	}

	api.OKWithMessage(c, "批量操作成功", nil)
}

// batchDelete 批量删除任务
// @Summary 批量删除任务
// @Description 批量删除多个下载任务
// @Tags download
// @Accept json
// @Produce json
// @Param request body BatchRequest true "批量请求"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/tasks/batch/delete [post]
// @Security BearerAuth.
func (h *Handlers) batchDelete(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	errors := h.manager.BatchDelete(req.TaskIDs)
	if len(errors) > 0 {
		api.InternalError(c, fmt.Sprintf("部分任务删除失败: %v", errors))
		return
	}

	api.OKWithMessage(c, "批量操作成功", nil)
}

// ========== 统计和历史 ==========

// getStats 获取下载统计
// @Summary 获取下载统计
// @Description 获取下载系统的统计信息
// @Tags download
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=DownloadStats}
// @Router /download/stats [get]
// @Security BearerAuth.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	api.OK(c, stats)
}

// getHistory 获取下载历史
// @Summary 获取下载历史
// @Description 获取下载历史记录列表
// @Tags download
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]HistoryEntry}
// @Router /download/history [get]
// @Security BearerAuth.
func (h *Handlers) getHistory(c *gin.Context) {
	history := h.manager.GetHistory()
	api.OK(c, history)
}

// clearHistory 清空下载历史
// @Summary 清空下载历史
// @Description 清空所有下载历史记录
// @Tags download
// @Accept json
// @Produce json
// @Success 200 {object} api.Response
// @Router /download/history [delete]
// @Security BearerAuth.
func (h *Handlers) clearHistory(c *gin.Context) {
	h.manager.ClearHistory()
	api.OKWithMessage(c, "历史记录已清空", nil)
}

// ========== 队列配置 ==========

// getConfig 获取队列配置
// @Summary 获取队列配置
// @Description 获取下载队列配置
// @Tags download
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=QueueConfig}
// @Router /download/config [get]
// @Security BearerAuth.
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	api.OK(c, config)
}

// updateConfig 更新队列配置
// @Summary 更新队列配置
// @Description 更新下载队列配置
// @Tags download
// @Accept json
// @Produce json
// @Param config body QueueConfig true "队列配置"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/config [put]
// @Security BearerAuth.
func (h *Handlers) updateConfig(c *gin.Context) {
	var config QueueConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.manager.UpdateConfig(config)
	api.OKWithMessage(c, "配置更新成功", nil)
}

// ========== 速度统计 ==========

// getSpeedStats 获取速度统计
// @Summary 获取速度统计
// @Description 获取下载速度统计历史
// @Tags download
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(60)
// @Success 200 {object} api.Response{data=[]SpeedStats}
// @Router /download/speed-stats [get]
// @Security BearerAuth.
func (h *Handlers) getSpeedStats(c *gin.Context) {
	limit := 60 // 默认返回最近 60 条记录

	stats := h.manager.queue.GetSpeedStats(limit)
	api.OK(c, stats)
}

// ========== RSS 订阅管理 ==========

// listFeeds 列出 RSS 订阅
// @Summary 列出 RSS 订阅
// @Description 获取所有 RSS 订阅列表
// @Tags download
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]RSSFeed}
// @Router /download/rss/feeds [get]
// @Security BearerAuth.
func (h *Handlers) listFeeds(c *gin.Context) {
	feeds := h.rssManager.ListFeeds()
	api.OK(c, feeds)
}

// addFeed 添加 RSS 订阅
// @Summary 添加 RSS 订阅
// @Description 添加新的 RSS 订阅
// @Tags download
// @Accept json
// @Produce json
// @Param request body AddRSSRequest true "添加订阅请求"
// @Success 200 {object} api.Response{data=RSSFeed}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/rss/feeds [post]
// @Security BearerAuth.
func (h *Handlers) addFeed(c *gin.Context) {
	var req AddRSSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	feed, err := h.rssManager.AddFeed(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "RSS 订阅添加成功", feed)
}

// getFeed 获取 RSS 订阅
// @Summary 获取 RSS 订阅详情
// @Description 获取指定 RSS 订阅的详细信息
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "订阅 ID"
// @Success 200 {object} api.Response{data=RSSFeed}
// @Failure 404 {object} api.Response
// @Router /download/rss/feeds/{id} [get]
// @Security BearerAuth.
func (h *Handlers) getFeed(c *gin.Context) {
	id := c.Param("id")

	feed, err := h.rssManager.GetFeed(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, feed)
}

// updateFeed 更新 RSS 订阅
// @Summary 更新 RSS 订阅
// @Description 更新指定的 RSS 订阅配置
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "订阅 ID"
// @Param request body AddRSSRequest true "更新请求"
// @Success 200 {object} api.Response{data=RSSFeed}
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/rss/feeds/{id} [put]
// @Security BearerAuth.
func (h *Handlers) updateFeed(c *gin.Context) {
	id := c.Param("id")

	var req AddRSSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	feed, err := h.rssManager.UpdateFeed(id, req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "订阅更新成功", feed)
}

// deleteFeed 删除 RSS 订阅
// @Summary 删除 RSS 订阅
// @Description 删除指定的 RSS 订阅
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "订阅 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/rss/feeds/{id} [delete]
// @Security BearerAuth.
func (h *Handlers) deleteFeed(c *gin.Context) {
	id := c.Param("id")

	if err := h.rssManager.DeleteFeed(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "订阅已删除", nil)
}

// enableFeed 启用/禁用 RSS 订阅
// @Summary 启用/禁用 RSS 订阅
// @Description 启用或禁用指定的 RSS 订阅
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "订阅 ID"
// @Param request body object{enabled=bool} true "启用状态"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/rss/feeds/{id}/enable [post]
// @Security BearerAuth.
func (h *Handlers) enableFeed(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.rssManager.EnableFeed(id, req.Enabled); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "状态已更新", nil)
}

// refreshFeed 刷新 RSS 订阅
// @Summary 刷新 RSS 订阅
// @Description 立即刷新指定的 RSS 订阅
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "订阅 ID"
// @Success 200 {object} api.Response{data=[]RSSItem}
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /download/rss/feeds/{id}/refresh [post]
// @Security BearerAuth.
func (h *Handlers) refreshFeed(c *gin.Context) {
	id := c.Param("id")

	items, err := h.rssManager.RefreshFeed(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "订阅刷新成功", items)
}

// getFeedItems 获取 RSS 条目
// @Summary 获取 RSS 条目
// @Description 获取指定 RSS 订阅的条目列表
// @Tags download
// @Accept json
// @Produce json
// @Param id path string true "订阅 ID"
// @Success 200 {object} api.Response{data=[]RSSItem}
// @Failure 404 {object} api.Response
// @Router /download/rss/feeds/{id}/items [get]
// @Security BearerAuth.
func (h *Handlers) getFeedItems(c *gin.Context) {
	id := c.Param("id")

	items, err := h.rssManager.GetFeedItems(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, items)
}

// ========== 健康检查 ==========

// healthCheck 健康检查
// @Summary 下载站健康检查
// @Description 检查下载站的健康状态
// @Tags download
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=HealthCheckResult}
// @Failure 500 {object} api.Response
// @Router /download/health [get]
// @Security BearerAuth.
func (h *Handlers) healthCheck(c *gin.Context) {
	stats := h.manager.GetStats()

	result := HealthCheckResult{
		Status:    "healthy",
		ActiveTasks: stats.ActiveTasks,
		TotalTasks:  stats.TotalTasks,
		QueueSize:   h.manager.queue.Len(),
		CurrentSpeed: stats.CurrentSpeed,
	}

	// 检查是否有异常
	if stats.FailedTasks > stats.CompletedTasks && stats.TotalTasks > 10 {
		result.Status = "degraded"
	}

	api.OK(c, result)
}

// HealthCheckResult 健康检查结果.
type HealthCheckResult struct {
	Status       string `json:"status"`        // healthy, degraded, unhealthy
	ActiveTasks  int    `json:"activeTasks"`   // 活跃任务数
	TotalTasks   int    `json:"totalTasks"`    // 总任务数
	QueueSize    int    `json:"queueSize"`     // 队列大小
	CurrentSpeed int64  `json:"currentSpeed"`  // 当前速度
}
