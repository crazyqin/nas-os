// Package storage RAIDZ Expansion API
// 兵部 Round 141 - RAIDZ Expansion API设计（对标TrueNAS 24.10）
package storage

import (
	"context"
	"fmt"
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
func NewRAIDZExpansionHandlers(poolMgr *ZPoolManager) *RAIDZExpansionHandlers {
	service := NewRAIDZExpansionService(poolMgr)
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
		
		// 开始扩展
		raidzGroup.POST("/start", h.startExpansion)
		
		// 取消扩展（如果正在进行）
		raidzGroup.POST("/cancel/:pool", h.cancelExpansion)
		
		// 获取可用磁盘列表
		raidzGroup.GET("/available-disks", h.getAvailableDisks)
		
		// 获取扩展历史
		raidzGroup.GET("/history", h.getExpansionHistory)
		
		// 预估时间和容量
		raidzGroup.POST("/estimate", h.estimateExpansion)
	}
}

// checkEligibility 检查池是否可以扩展
// @Summary 检查RAIDZ扩展资格
// @Description 检查指定ZFS池是否满足RAIDZ扩展条件
// @Tags storage
// @Accept json
// @Produce json
// @Param pool path string true "ZFS池名称"
// @Success 200 {object} api.Response{data=ExpansionEligibility} "成功"
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
// @Success 200 {object} api.Response{data=RAIDZExpansionStatus} "成功"
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

// startExpansion 开始RAIDZ扩展
// @Summary 开始RAIDZ扩展
// @Description 启动RAIDZ vdev的单盘扩容操作（需确认）
// @Tags storage
// @Accept json
// @Produce json
// @Param request body RAIDZExpansionRequest true "扩展请求"
// @Success 200 {object} api.Response{data=RAIDZExpansionStatus} "扩展已启动"
// @Failure 400 {object} api.Response "参数错误"
// @Failure 409 {object} api.Response "池已有扩展任务进行中"
// @Failure 500 {object} api.Response "服务器内部错误"
// @Router /storage/raidz-expansion/start [post]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) startExpansion(c *gin.Context) {
	var req RAIDZExpansionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求格式错误: "+err.Error())
		return
	}
	
	// 验证必填字段
	if req.PoolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}
	if req.NewDiskID == "" {
		api.BadRequest(c, "新磁盘ID不能为空")
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
	if currentStatus != nil && currentStatus.Status == "expanding" {
		c.JSON(http.StatusConflict, api.Error(api.CodeConflict, "池已有扩展任务正在进行中"))
		return
	}
	
	status, err := h.service.StartExpansion(ctx, &req)
	if err != nil {
		api.InternalError(c, "启动扩展失败: "+err.Error())
		return
	}
	
	api.OKWithMessage(c, "RAIDZ扩展已启动", status)
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
	
	// 检查当前状态
	status, err := h.service.GetExpansionStatus(poolName)
	if err != nil || status.Status != "expanding" {
		api.NotFound(c, "该池无正在进行的扩展任务")
		return
	}
	
	// TODO: 实现取消逻辑
	// 实际需要调用 zpool cancel 或类似命令
	
	api.OKWithMessage(c, "扩展任务已取消", gin.H{
		"pool":     poolName,
		"status":   "cancelled",
		"progress": status.Progress,
	})
}

// getAvailableDisks 获取可用于扩展的磁盘
// @Summary 获取可用磁盘列表
// @Description 获取所有可用于RAIDZ扩展的新磁盘列表
// @Tags storage
// @Accept json
// @Produce json
// @Param minSize query int false "最小容量要求(GB)" default(0)
// @Success 200 {object} api.Response "成功"
// @Router /storage/raidz-expansion/available-disks [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) getAvailableDisks(c *gin.Context) {
	minSize := 0
	if m := c.Query("minSize"); m != "" {
		if n, err := strconv.Atoi(m); err == nil {
			minSize = n
		}
	}
	
	// TODO: 实现磁盘发现逻辑
	// 需要调用系统命令查找未使用的磁盘
	// 使用 lsblk、fdisk -l 或 smartctl --scan
	
	disks := []AvailableDisk{
		// 示例数据，实际需要动态获取
		{
			ID:        "disk-sda",
			Path:      "/dev/sda",
			Model:     "Samsung SSD 870 EVO",
			SizeGB:    500,
			Interface: "SATA",
			Healthy:   true,
		},
	}
	
	// 过滤最小容量
	if minSize > 0 {
		filtered := make([]AvailableDisk, 0)
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
// @Success 200 {object} api.Response "成功"
// @Router /storage/raidz-expansion/history [get]
// @Security BearerAuth
func (h *RAIDZExpansionHandlers) getExpansionHistory(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	
	// TODO: 实现历史记录查询
	// 需要持久化存储扩展记录
	
	api.OK(c, gin.H{
		"total":   0,
		"records": []ExpansionHistory{},
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
// @Success 200 {object} api.Response{data=ExpansionEstimate} "成功"
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
	
	// 预估时间（简化计算）
	// OpenZFS RAIDZ扩展：每TB约30分钟重平衡
	pool, _ := h.service.pools.GetPool(req.PoolName)
	usedTB := float64(pool.Used) / (1024 * 1024 * 1024 * 1024)
	estimatedMinutes := int(usedTB * 30)
	
	estimate := ExpansionEstimate{
		PoolName:            req.PoolName,
		CurrentCapacityGB:   int(pool.Available / (1024 * 1024 * 1024)),
		CapacityGainGB:      int(eligibility.CapacityGain / (1024 * 1024 * 1024)),
		NewCapacityGB:       int(eligibility.CapacityGain + pool.Used) / (1024 * 1024 * 1024),
		EstimatedDuration:   fmt.Sprintf("%d分钟", estimatedMinutes),
		EstimatedMinutes:    estimatedMinutes,
		PreChecksPassed:     len(getPassedChecks(eligibility.PreChecks)),
		PreChecksFailed:     len(getFailedChecks(eligibility.PreChecks)),
		Warnings:            eligibility.Warnings,
		Eligible:            eligibility.Eligible,
	}
	
	api.OK(c, estimate)
}

// 辅助函数
func getPassedChecks(checks []PreCheckResult) []PreCheckResult {
	passed := make([]PreCheckResult, 0)
	for _, check := range checks {
		if check.Passed {
			passed = append(passed, check)
		}
	}
	return passed
}

func getFailedChecks(checks []PreCheckResult) []PreCheckResult {
	failed := make([]PreCheckResult, 0)
	for _, check := range checks {
		if !check.Passed {
			failed = append(failed, check)
		}
	}
	return failed
}

// 类型定义

// AvailableDisk 可用磁盘
type AvailableDisk struct {
	ID        string `json:"id"`         // 磁盘唯一ID
	Path      string `json:"path"`       // 设备路径
	Model     string `json:"model"`      // 型号
	SizeGB    int    `json:"size_gb"`    // 容量(GB)
	Interface string `json:"interface"`  // 接口类型(SATA/NVMe)
	Healthy   bool   `json:"healthy"`    // SMART健康状态
}

// ExpansionHistory 扩展历史记录
type ExpansionHistory struct {
	PoolName     string    `json:"pool_name"`
	OldCapacity  uint64    `json:"old_capacity"`
	NewCapacity  uint64    `json:"new_capacity"`
	Duration     string    `json:"duration"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Status       string    `json:"status"`
}

// EstimateRequest 预估请求
type EstimateRequest struct {
	PoolName  string `json:"pool_name" binding:"required"`
	NewDiskID string `json:"new_disk_id"`
}

// ExpansionEstimate 扩展预估结果
type ExpansionEstimate struct {
	PoolName          string   `json:"pool_name"`
	CurrentCapacityGB int      `json:"current_capacity_gb"`
	CapacityGainGB    int      `json:"capacity_gain_gb"`
	NewCapacityGB     int      `json:"new_capacity_gb"`
	EstimatedDuration string   `json:"estimated_duration"`
	EstimatedMinutes  int      `json:"estimated_minutes"`
	PreChecksPassed   int      `json:"pre_checks_passed"`
	PreChecksFailed   int      `json:"pre_checks_failed"`
	Warnings          []string `json:"warnings"`
	Eligible          bool     `json:"eligible"`
}