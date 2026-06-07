// Package storage RAIDZ Expansion API
// 兵部 Round 142 - RAIDZ Expansion API设计（对标TrueNAS 24.10）
package storage

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// RAIDZExpansionHandlers RAIDZ扩展API处理器
type RAIDZExpansionHandlers struct {
	service *RAIDZExpansionService
}

// NewRAIDZExpansionHandlers 创建处理器
func NewRAIDZExpansionHandlers(service *RAIDZExpansionService) *RAIDZExpansionHandlers {
	return &RAIDZExpansionHandlers{
		service: service,
	}
}

// RegisterRoutes 注册路由
func (h *RAIDZExpansionHandlers) RegisterRoutes(r *gin.RouterGroup) {
	raidzGroup := r.Group("/storage/raidz-expansion")
	{
		// 检查扩展资格
		raidzGroup.GET("/eligibility/:pool", h.checkEligibility)

		// 获取扩展状态
		raidzGroup.GET("/status/:pool", h.getExpansionStatus)

		// 获取所有活跃任务
		raidzGroup.GET("/tasks", h.getAllActiveTasks)

		// 开始扩展
		raidzGroup.POST("/start", h.startExpansion)

		// 暂停扩展
		raidzGroup.POST("/pause/:pool", h.pauseExpansion)

		// 恢复扩展
		raidzGroup.POST("/resume/:pool", h.resumeExpansion)

		// 取消扩展
		raidzGroup.POST("/cancel/:pool", h.cancelExpansion)

		// 获取可用磁盘列表
		raidzGroup.GET("/available-disks", h.getAvailableDisks)

		// 获取扩展历史
		raidzGroup.GET("/history", h.getExpansionHistory)

		// 预估时间和容量
		raidzGroup.POST("/estimate", h.estimateExpansion)

		// 检查服务状态
		raidzGroup.GET("/service-status", h.getServiceStatus)
	}
}

// checkEligibility 检查池是否可以扩展
// @Summary 检查RAIDZ扩展资格
// @Description 检查指定ZFS池是否满足RAIDZ扩展条件
// @Tags storage
// @Accept json
// @Produce json
// @Param pool path string true "ZFS池名称"
// @Success 200 {object} api.Response{data=ExpansionEligibilityResult} "成功"
// @Failure 400 {object} api.Response "池不存在"
// @Failure 500 {object} api.Response "服务器内部错误"
// @Router /storage/raidz-expansion/eligibility/{pool} [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) checkEligibility(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	eligibility, err := h.service.CheckExpansionEligibility(ctx, poolName)
	if err != nil {
		api.NotFound(c, "池不存在或无法访问: "+err.Error())
		return
	}

	api.OK(c, eligibility)
}

// getExpansionStatus 获取扩展进度状态
// @Summary 获取RAIDZ扩展状态
// @Description 获取指定池的RAIDZ扩展进度和状态
// @Tags storage
// @Accept json
// @Produce json
// @Param pool path string true "ZFS池名称"
// @Success 200 {object} api.Response{data=ExpansionTask} "成功"
// @Router /storage/raidz-expansion/status/{pool} [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) getExpansionStatus(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	status, err := h.service.GetExpansionStatus(poolName)
	if err != nil {
		api.InternalError(c, "获取扩展状态失败: "+err.Error())
		return
	}

	api.OK(c, status)
}

// getAllActiveTasks 获取所有活跃任务
// @Summary 获取所有活跃扩展任务
// @Description 获取所有正在进行的RAIDZ扩展任务列表
// @Tags storage
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]ExpansionTask} "成功"
// @Router /storage/raidz-expansion/tasks [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) getAllActiveTasks(c *gin.Context) {
	tasks := h.service.GetAllActiveTasks()
	api.OK(c, tasks)
}

// StartExpansionReq 开始扩展请求（旧版API）
type StartExpansionReq struct {
	PoolName string `json:"pool_name" binding:"required"` // ZFS池名称
	NewDisk  string `json:"new_disk" binding:"required"`  // 新磁盘路径
	Force    bool   `json:"force"`                        // 强制执行
	Confirm  bool   `json:"confirm"`                      // 确认执行
}

// startExpansion 开始RAIDZ扩展
// @Summary 开始RAIDZ扩展
// @Description 启动RAIDZ vdev的单盘扩容操作（需确认）
// @Tags storage
// @Accept json
// @Produce json
// @Param request body StartExpansionReq true "扩展请求"
// @Success 200 {object} api.Response{data=ExpansionTask} "扩展已启动"
// @Failure 400 {object} api.Response "参数错误"
// @Failure 409 {object} api.Response "池已有扩展任务进行中"
// @Failure 500 {object} api.Response "服务器内部错误"
// @Router /storage/raidz-expansion/start [post]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) startExpansion(c *gin.Context) {
	var req StartExpansionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求格式错误: "+err.Error())
		return
	}

	// 验证必填字段
	if req.PoolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}
	if req.NewDisk == "" {
		api.BadRequest(c, "新磁盘路径不能为空")
		return
	}
	if !req.Confirm {
		api.BadRequest(c, "扩展操作需要明确确认（confirm=true）")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 检查是否有正在进行的扩展
	currentStatus, _ := h.service.GetExpansionStatus(req.PoolName)
	if currentStatus != nil && currentStatus.Status == StatusRunning {
		c.JSON(http.StatusConflict, api.Error(api.CodeConflict, "池已有扩展任务正在进行中"))
		return
	}

	task, err := h.service.StartExpansion(ctx, req.PoolName, req.NewDisk, req.Force)
	if err != nil {
		api.InternalError(c, "启动扩展失败: "+err.Error())
		return
	}

	api.OKWithMessage(c, "RAIDZ扩展已启动", task)
}

// pauseExpansion 暂停扩展
// @Summary 暂停RAIDZ扩展
// @Description 暂停指定池正在进行的RAIDZ扩展操作
// @Tags storage
// @Accept json
// @Produce json
// @Param pool path string true "ZFS池名称"
// @Success 200 {object} api.Response "扩展已暂停"
// @Failure 400 {object} api.Response "池名称为空"
// @Failure 404 {object} api.Response "无正在进行的扩展"
// @Router /storage/raidz-expansion/pause/{pool} [post]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) pauseExpansion(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	if err := h.service.PauseExpansion(poolName); err != nil {
		api.BadRequest(c, "暂停失败: "+err.Error())
		return
	}

	status, _ := h.service.GetExpansionStatus(poolName)
	api.OKWithMessage(c, "扩展任务已暂停", status)
}

// resumeExpansion 恢复扩展
// @Summary 恢复RAIDZ扩展
// @Description 恢复指定池已暂停的RAIDZ扩展操作
// @Tags storage
// @Accept json
// @Produce json
// @Param pool path string true "ZFS池名称"
// @Success 200 {object} api.Response{data=ExpansionTask} "扩展已恢复"
// @Failure 400 {object} api.Response "池名称为空"
// @Failure 404 {object} api.Response "无暂停的扩展"
// @Router /storage/raidz-expansion/resume/{pool} [post]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) resumeExpansion(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	if err := h.service.ResumeExpansion(poolName); err != nil {
		api.BadRequest(c, "恢复失败: "+err.Error())
		return
	}

	status, _ := h.service.GetExpansionStatus(poolName)
	api.OKWithMessage(c, "扩展任务已恢复", status)
}

// cancelExpansion 取消正在进行的扩展
// @Summary 取消RAIDZ扩展
// @Description 取消指定池正在进行的RAIDZ扩展操作
// @Tags storage
// @Accept json
// @Produce json
// @Param pool path string true "ZFS池名称"
// @Success 200 {object} api.Response "扩展已取消"
// @Failure 400 {object} api.Response "池名称为空"
// @Failure 404 {object} api.Response "无正在进行的扩展"
// @Router /storage/raidz-expansion/cancel/{pool} [post]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) cancelExpansion(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	if err := h.service.CancelExpansion(poolName); err != nil {
		api.BadRequest(c, "取消失败: "+err.Error())
		return
	}

	api.OKWithMessage(c, "扩展任务已取消", gin.H{
		"pool":   poolName,
		"status": StatusCancelled,
	})
}

// getAvailableDisks 获取可用于扩展的磁盘
// @Summary 获取可用磁盘列表
// @Description 获取所有可用于RAIDZ扩展的新磁盘列表
// @Tags storage
// @Accept json
// @Produce json
// @Param minSize query int false "最小容量要求(GB)" default(0)
// @Success 200 {object} api.Response{data=[]AvailableDiskInfo} "成功"
// @Router /storage/raidz-expansion/available-disks [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) getAvailableDisks(c *gin.Context) {
	minSize := 0
	if m := c.Query("minSize"); m != "" {
		if n, err := strconv.Atoi(m); err == nil {
			minSize = n
		}
	}

	ctx := c.Request.Context()

	disks, err := h.service.ListAvailableDisks(ctx)
	if err != nil {
		api.InternalError(c, "获取磁盘列表失败: "+err.Error())
		return
	}

	// 过滤最小容量
	if minSize > 0 {
		filtered := make([]AvailableDiskInfo, 0)
		for _, disk := range disks {
			if disk.SizeGB >= minSize {
				filtered = append(filtered, disk)
			}
		}
		disks = filtered
	}

	api.OK(c, disks)
}

// getExpansionHistory 获取扩展历史记录
// @Summary 获取RAIDZ扩展历史
// @Description 获取所有RAIDZ扩展操作的历史记录
// @Tags storage
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(20)
// @Success 200 {object} api.Response{data=[]ExpansionTask} "成功"
// @Router /storage/raidz-expansion/history [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) getExpansionHistory(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	history := h.service.GetTaskHistory(limit)
	api.OK(c, gin.H{
		"total":   len(history),
		"records": history,
		"limit":   limit,
	})
}

// estimateExpansion 预估扩展时间和容量增益
// @Summary 预估RAIDZ扩展效果
// @Description 预估扩展所需时间和最终容量增益
// @Tags storage
// @Accept json
// @Produce json
// @Param request body EstimateRequest true "预估请求"
// @Success 200 {object} api.Response{data=ExpansionEstimateResult} "成功"
// @Router /storage/raidz-expansion/estimate [post]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) estimateExpansion(c *gin.Context) {
	var req EstimateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求格式错误: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 检查资格获取容量信息
	eligibility, err := h.service.CheckExpansionEligibility(ctx, req.PoolName)
	if err != nil {
		api.BadRequest(c, "池信息获取失败: "+err.Error())
		return
	}

	// 预估时间
	estimatedTime, err := h.service.EstimateExpansionTime(ctx, req.PoolName)
	if err != nil {
		estimatedTime = 0 // 使用默认估算
	}

	estimate := ExpansionEstimateResult{
		PoolName:          req.PoolName,
		CurrentCapacityGB: int(eligibility.CurrentCapacity / (1024 * 1024 * 1024)),
		CapacityGainGB:    int(eligibility.CapacityGain / (1024 * 1024 * 1024)),
		NewCapacityGB:     int(eligibility.NewCapacity / (1024 * 1024 * 1024)),
		EstimatedTime:     estimatedTime.String(),
		EstimatedMinutes:  int(estimatedTime.Minutes()),
		RAIDZLevel:        eligibility.RAIDZLevel,
		CurrentWidth:      eligibility.CurrentWidth,
		NewWidth:          eligibility.NewWidth,
		PreChecksPassed:   countPassedChecks(eligibility.PreChecks),
		PreChecksFailed:   countFailedChecks(eligibility.PreChecks),
		Warnings:          eligibility.Warnings,
		Eligible:          eligibility.Eligible,
		DiskRequirements:  eligibility.DiskRequirements,
	}

	api.OK(c, estimate)
}

// getServiceStatus 获取服务状态
// @Summary 获取RAIDZ扩展服务状态
// @Description 检查RAIDZ扩展服务是否可用
// @Tags storage
// @Accept json
// @Produce json
// @Success 200 {object} api.Response "成功"
// @Router /storage/raidz-expansion/service-status [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) getServiceStatus(c *gin.Context) {
	available := h.service.IsAvailable()

	activeCount := len(h.service.GetAllActiveTasks())

	api.OK(c, gin.H{
		"available":      available,
		"active_tasks":   activeCount,
		"service_status": "running",
	})
}

// 辅助函数
func countPassedChecks(checks []PreCheckResult) int {
	count := 0
	for _, check := range checks {
		if check.Passed {
			count++
		}
	}
	return count
}

func countFailedChecks(checks []PreCheckResult) int {
	count := 0
	for _, check := range checks {
		if !check.Passed {
			count++
		}
	}
	return count
}

// 类型定义（补充 handlers 专用类型）

// EstimateRequest 预估请求
type EstimateRequest struct {
	PoolName  string `json:"pool_name" binding:"required"`
	NewDiskID string `json:"new_disk_id"`
}

// ExpansionEstimateResult 扩展预估结果
type ExpansionEstimateResult struct {
	PoolName          string           `json:"pool_name"`
	CurrentCapacityGB int              `json:"current_capacity_gb"`
	CapacityGainGB    int              `json:"capacity_gain_gb"`
	NewCapacityGB     int              `json:"new_capacity_gb"`
	EstimatedTime     string           `json:"estimated_time"`
	EstimatedMinutes  int              `json:"estimated_minutes"`
	RAIDZLevel        string           `json:"raidz_level"`
	CurrentWidth      int              `json:"current_width"`
	NewWidth          int              `json:"new_width"`
	PreChecksPassed   int              `json:"pre_checks_passed"`
	PreChecksFailed   int              `json:"pre_checks_failed"`
	Warnings          []string         `json:"warnings"`
	Eligible          bool             `json:"eligible"`
	DiskRequirements  DiskRequirements `json:"disk_requirements"`
}
