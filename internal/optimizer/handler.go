package optimizer

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// EngineHandlers 优化引擎 HTTP 处理器
type EngineHandlers struct {
	logger *zap.Logger
	engine *OptimizationEngine
}

// NewEngineHandlers 创建处理器
func NewEngineHandlers(logger *zap.Logger, engine *OptimizationEngine) *EngineHandlers {
	return &EngineHandlers{
		logger: logger,
		engine: engine,
	}
}

// RegisterRoutes 注册路由
func (h *EngineHandlers) RegisterRoutes(api *gin.RouterGroup) {
	engine := api.Group("/optimization-engine")
	{
		// 引擎状态
		engine.GET("/status", h.getStatus)
		engine.GET("/stats", h.getStats)
		engine.GET("/config", h.getConfig)
		engine.PUT("/config", h.updateConfig)

		// 引擎控制
		engine.POST("/start", h.startEngine)
		engine.POST("/stop", h.stopEngine)

		// 性能指标
		engine.GET("/metrics", h.getMetrics)

		// 瓶颈检测
		engine.GET("/bottlenecks", h.getBottlenecks)

		// 资源预测
		engine.GET("/predictions", h.getPredictions)

		// 优化建议
		engine.GET("/suggestions", h.getSuggestions)

		// 优化历史
		engine.GET("/history", h.getHistory)
		engine.DELETE("/history", h.clearHistory)

		// 定时任务
		engine.GET("/scheduled-tasks", h.getScheduledTasks)
		engine.POST("/scheduled-tasks", h.addScheduledTask)
		engine.PUT("/scheduled-tasks/:id", h.updateScheduledTask)
		engine.DELETE("/scheduled-tasks/:id", h.removeScheduledTask)

		// 手动优化
		engine.POST("/optimize/cpu", h.optimizeCPU)
		engine.POST("/optimize/memory", h.optimizeMemory)
		engine.POST("/optimize/io", h.optimizeIO)
	}
}

// EngineStatusResponse 引擎状态响应
type EngineStatusResponse struct {
	Running bool            `json:"running"`
	Uptime  time.Duration   `json:"uptime"`
	Stats   *EngineStats    `json:"stats"`
	Config  *AutoTuneConfig `json:"config"`
}

// getStatus 获取引擎状态
func (h *EngineHandlers) getStatus(c *gin.Context) {
	stats := h.engine.GetStats()
	config := h.engine.GetConfig()

	response := EngineStatusResponse{
		Running: h.engine.IsRunning(),
		Uptime:  stats.Uptime,
		Stats:   stats,
		Config:  config,
	}

	c.JSON(http.StatusOK, success(response))
}

// getStats 获取引擎统计
func (h *EngineHandlers) getStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, success(stats))
}

// getConfig 获取配置
func (h *EngineHandlers) getConfig(c *gin.Context) {
	config := h.engine.GetConfig()
	c.JSON(http.StatusOK, success(config))
}

// UpdateEngineConfigRequest 更新引擎配置请求
type UpdateEngineConfigRequest struct {
	Enabled       *bool    `json:"enabled"`
	CPUThreshold  *float64 `json:"cpu_threshold"`
	MemThreshold  *float64 `json:"mem_threshold"`
	IOThreshold   *float64 `json:"io_threshold"`
	TuneInterval  *int     `json:"tune_interval"`
	MaxConcurrent *int     `json:"max_concurrent"`
	DryRun        *bool    `json:"dry_run"`
	AutoApply     *bool    `json:"auto_apply"`
	NotifyOnTune  *bool    `json:"notify_on_tune"`
}

// updateConfig 更新配置
func (h *EngineHandlers) updateConfig(c *gin.Context) {
	var req UpdateEngineConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	config := h.engine.GetConfig()

	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.CPUThreshold != nil {
		config.CPUThreshold = *req.CPUThreshold
	}
	if req.MemThreshold != nil {
		config.MemThreshold = *req.MemThreshold
	}
	if req.IOThreshold != nil {
		config.IOThreshold = *req.IOThreshold
	}
	if req.TuneInterval != nil {
		config.TuneInterval = *req.TuneInterval
	}
	if req.MaxConcurrent != nil {
		config.MaxConcurrent = *req.MaxConcurrent
	}
	if req.DryRun != nil {
		config.DryRun = *req.DryRun
	}
	if req.AutoApply != nil {
		config.AutoApply = *req.AutoApply
	}
	if req.NotifyOnTune != nil {
		config.NotifyOnTune = *req.NotifyOnTune
	}

	h.engine.UpdateConfig(config)

	c.JSON(http.StatusOK, success(config))
}

// startEngine 启动引擎
func (h *EngineHandlers) startEngine(c *gin.Context) {
	if h.engine.IsRunning() {
		c.JSON(http.StatusOK, success(map[string]string{
			"status": "already running",
		}))
		return
	}

	if err := h.engine.Start(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, apiError(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(map[string]string{
		"status": "started",
	}))
}

// stopEngine 停止引擎
func (h *EngineHandlers) stopEngine(c *gin.Context) {
	if !h.engine.IsRunning() {
		c.JSON(http.StatusOK, success(map[string]string{
			"status": "already stopped",
		}))
		return
	}

	h.engine.Stop()

	c.JSON(http.StatusOK, success(map[string]string{
		"status": "stopped",
	}))
}

// getMetrics 获取当前指标
func (h *EngineHandlers) getMetrics(c *gin.Context) {
	metrics := h.engine.metrics.Collect()
	c.JSON(http.StatusOK, success(metrics))
}

// getBottlenecks 获取瓶颈
func (h *EngineHandlers) getBottlenecks(c *gin.Context) {
	bottlenecks := h.engine.GetBottlenecks()
	c.JSON(http.StatusOK, success(bottlenecks))
}

// getPredictions 获取预测
func (h *EngineHandlers) getPredictions(c *gin.Context) {
	predictions := h.engine.GetPredictions()
	c.JSON(http.StatusOK, success(predictions))
}

// getSuggestions 获取建议
func (h *EngineHandlers) getSuggestions(c *gin.Context) {
	suggestions := h.engine.GetSuggestions()
	c.JSON(http.StatusOK, success(suggestions))
}

// getHistory 获取历史
func (h *EngineHandlers) getHistory(c *gin.Context) {
	history := h.engine.GetHistory()
	c.JSON(http.StatusOK, success(history))
}

// clearHistory 清空历史
func (h *EngineHandlers) clearHistory(c *gin.Context) {
	h.engine.history.Clear()
	c.JSON(http.StatusOK, success(map[string]string{
		"status": "history cleared",
	}))
}

// getScheduledTasks 获取定时任务
func (h *EngineHandlers) getScheduledTasks(c *gin.Context) {
	tasks := h.engine.GetScheduler().GetTasks()
	c.JSON(http.StatusOK, success(tasks))
}

// AddScheduledTaskRequest 添加定时任务请求
type AddScheduledTaskRequest struct {
	Name     string   `json:"name" binding:"required"`
	CronExpr string   `json:"cron_expr" binding:"required"`
	Category string   `json:"category" binding:"required"`
	Actions  []string `json:"actions" binding:"required"`
	Enabled  bool     `json:"enabled"`
}

// addScheduledTask 添加定时任务
func (h *EngineHandlers) addScheduledTask(c *gin.Context) {
	var req AddScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	task := &ScheduledTask{
		ID:        generateID(),
		Name:      req.Name,
		CronExpr:  req.CronExpr,
		Category:  req.Category,
		Actions:   req.Actions,
		Enabled:   req.Enabled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	h.engine.GetScheduler().AddTask(task)

	c.JSON(http.StatusOK, success(task))
}

// UpdateScheduledTaskRequest 更新定时任务请求
type UpdateScheduledTaskRequest struct {
	Name     *string  `json:"name"`
	CronExpr *string  `json:"cron_expr"`
	Category *string  `json:"category"`
	Actions  []string `json:"actions"`
	Enabled  *bool    `json:"enabled"`
}

// updateScheduledTask 更新定时任务
func (h *EngineHandlers) updateScheduledTask(c *gin.Context) {
	taskID := c.Param("id")

	var req UpdateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 获取现有任务
	tasks := h.engine.GetScheduler().GetTasks()
	var task *ScheduledTask
	for _, t := range tasks {
		if t.ID == taskID {
			task = t
			break
		}
	}

	if task == nil {
		c.JSON(http.StatusNotFound, apiError(404, "task not found"))
		return
	}

	// 更新字段
	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.CronExpr != nil {
		task.CronExpr = *req.CronExpr
	}
	if req.Category != nil {
		task.Category = *req.Category
	}
	if req.Actions != nil {
		task.Actions = req.Actions
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	task.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, success(task))
}

// removeScheduledTask 移除定时任务
func (h *EngineHandlers) removeScheduledTask(c *gin.Context) {
	taskID := c.Param("id")
	h.engine.GetScheduler().RemoveTask(taskID)

	c.JSON(http.StatusOK, success(map[string]string{
		"status": "task removed",
	}))
}

// optimizeCPU 手动 CPU 优化
func (h *EngineHandlers) optimizeCPU(c *gin.Context) {
	metrics := h.engine.metrics.Collect()
	record := h.engine.autoTuner.tuneCPU(c.Request.Context(), metrics)

	if record != nil {
		h.engine.history.Add(record)
		h.engine.mu.Lock()
		h.engine.stats.TotalOptimizations++
		h.engine.stats.SuccessfulTunes++
		h.engine.stats.TotalImprovement += record.Improvement
		h.engine.mu.Unlock()
	}

	c.JSON(http.StatusOK, success(record))
}

// optimizeMemory 手动内存优化
func (h *EngineHandlers) optimizeMemory(c *gin.Context) {
	metrics := h.engine.metrics.Collect()
	record := h.engine.autoTuner.tuneMemory(c.Request.Context(), metrics)

	if record != nil {
		h.engine.history.Add(record)
		h.engine.mu.Lock()
		h.engine.stats.TotalOptimizations++
		h.engine.stats.SuccessfulTunes++
		h.engine.stats.TotalImprovement += record.Improvement
		h.engine.mu.Unlock()
	}

	c.JSON(http.StatusOK, success(record))
}

// optimizeIO 手动 IO 优化
func (h *EngineHandlers) optimizeIO(c *gin.Context) {
	metrics := h.engine.metrics.Collect()
	record := h.engine.autoTuner.tuneIO(c.Request.Context(), metrics)

	if record != nil {
		h.engine.history.Add(record)
		h.engine.mu.Lock()
		h.engine.stats.TotalOptimizations++
		h.engine.stats.SuccessfulTunes++
		h.engine.stats.TotalImprovement += record.Improvement
		h.engine.mu.Unlock()
	}

	c.JSON(http.StatusOK, success(record))
}

// generateID 生成唯一 ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
