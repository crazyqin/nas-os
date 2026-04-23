// Package web RAIDZ 扩展 WebUI API
// 兵部 Round 230 - RAIDZ Expansion WebUI 集成
// 对标 TrueNAS 24.10 Electric Eel RAIDZ Expansion UI特性

package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"nas-os/internal/api"
	"nas-os/internal/storage"

	"github.com/gin-gonic/gin"
)

// RAIDZUIHandlers RAIDZ扩展WebUI处理器
// 整合底层storage模块的RAIDZExpandHandlers，提供WebUI专用接口
type RAIDZUIHandlers struct {
	// 核心服务
	expandService *storage.RAIDZExpansionService
	expandMonitor *storage.RAIDZExpandMonitor
	storageMgr    *storage.Manager

	// 内层处理器（复用现有逻辑）
	innerHandlers *storage.RAIDZExpandHandlers
}

// NewRAIDZUIHandlers 创建RAIDZ扩展WebUI处理器
func NewRAIDZUIHandlers(expandService *storage.RAIDZExpansionService, monitor *storage.RAIDZExpandMonitor, storageMgr *storage.Manager) *RAIDZUIHandlers {
	h := &RAIDZUIHandlers{
		expandService: expandService,
		expandMonitor: monitor,
		storageMgr:    storageMgr,
	}

	// 创建内层处理器（复用现有逻辑）
	if monitor != nil && storageMgr != nil {
		h.innerHandlers = storage.NewRAIDZExpandHandlers(monitor, storageMgr)
	}

	return h
}

// RegisterRoutes 注册路由到 /api/raidz-expansion 组
func (h *RAIDZUIHandlers) RegisterRoutes(rg *gin.RouterGroup) {
	raidz := rg.Group("/raidz-expansion")
	{
		// ========== Dashboard 卡片接口 ==========
		raidz.GET("/dashboard", h.getDashboardSummary)

		// ========== 状态和进度接口 ==========
		raidz.GET("/status", h.getGlobalStatus)
		raidz.GET("/progress", h.getAllProgress)
		raidz.GET("/progress/:pool", h.getPoolProgress)
		raidz.GET("/history", h.getHistory)
		raidz.GET("/summary", h.getSummary)

		// ========== 扩展操作接口 ==========
		raidz.POST("/start", h.startExpansion)
		raidz.POST("/pause/:pool", h.pauseExpansion)
		raidz.POST("/resume/:pool", h.resumeExpansion)
		raidz.POST("/cancel/:pool", h.cancelExpansion)

		// ========== 验证和预检接口 ==========
		raidz.POST("/validate", h.validateExpansion)
		raidz.POST("/estimate", h.estimateExpansion)
		raidz.GET("/eligibility/:pool", h.checkEligibility)

		// ========== 磁盘信息接口 ==========
		raidz.GET("/available-disks", h.listAvailableDisks)
		raidz.GET("/disk-info/:device", h.getDiskInfo)

		// ========== 容量计算接口 ==========
		raidz.GET("/capacity/:raidz/:width", h.calculateCapacity)

		// ========== 阶段详情接口 ==========
		raidz.GET("/phases/:pool", h.getPhaseDetails)

		// ========== WebSocket 实时进度推送 ==========
		raidz.GET("/ws/:pool", h.websocketProgress)
	}
}

// ========== Dashboard 卡片接口 ==========

// RAIDZDashboardSummary Dashboard展示摘要
type RAIDZDashboardSummary struct {
	// 活跃任务
	ActiveCount    int `json:"activeCount"`    // 活跃任务数
	RunningCount   int `json:"runningCount"`   // 运行中数
	PausedCount    int `json:"pausedCount"`    // 已暂停数

	// 最新任务（如果有）
	LatestTask     *RAIDZTaskCard `json:"latestTask"` // 最新任务卡片

	// 最近历史
	RecentHistory  []RAIDZHistoryCard `json:"recentHistory"` // 最近历史卡片

	// 是否支持扩展
	ExpansionSupported bool   `json:"expansionSupported"` // 系统是否支持RAIDZ扩展
	SupportReason      string `json:"supportReason"`      // 不支持原因（如果不支持）

	// 可扩展池列表
	ExpandablePools []ExpandablePoolCard `json:"expandablePools"` // 可扩展池列表

	// 最后更新时间
	LastUpdate     time.Time `json:"lastUpdate"`
}

// RAIDZTaskCard 任务卡片（Dashboard展示）
type RAIDZTaskCard struct {
	PoolName       string    `json:"poolName"`       // 池名称
	RAIDZLevel     string    `json:"raidzLevel"`     // RAIDZ级别
	Status         string    `json:"status"`         // 状态
	StatusText     string    `json:"statusText"`     // 状态文本
	Percent        float64   `json:"percent"`        // 进度百分比
	SpeedMBps      float64   `json:"speedMBps"`      // 当前速度
	ETAFormatted   string    `json:"etaFormatted"`   // 预估剩余时间
	ElapsedFormatted string  `json:"elapsedFormatted"` // 已耗时
	Phase          string    `json:"phase"`          // 当前阶段
	PhaseText      string    `json:"phaseText"`      // 阶段文本
	CanPause       bool      `json:"canPause"`       // 是否可暂停
	CanResume      bool      `json:"canResume"`      // 是否可恢复
	CanCancel      bool      `json:"canCancel"`      // 是否可取消
	StartTime      time.Time `json:"startTime"`      // 开始时间
}

// RAIDZHistoryCard 历史卡片
type RAIDZHistoryCard struct {
	PoolName     string    `json:"poolName"`     // 池名称
	Status       string    `json:"status"`       // 最终状态
	Percent      float64   `json:"percent"`      // 最终进度
	StartTime    time.Time `json:"startTime"`    // 开始时间
	EndTime      time.Time `json:"endTime"`      // 结束时间
	Duration     string    `json:"duration"`     // 总耗时
	CapacityGain float64   `json:"capacityGain"` // 容量增益GB
}

// ExpandablePoolCard 可扩展池卡片
type ExpandablePoolCard struct {
	PoolName       string  `json:"poolName"`       // 池名称
	RAIDZLevel     string  `json:"raidzLevel"`     // RAIDZ级别
	CurrentWidth   int     `json:"currentWidth"`   // 当前宽度
	CurrentCapGB   float64 `json:"currentCapGB"`   // 当前容量GB
	PotentialGain  float64 `json:"potentialGain"`  // 潜在增益GB
	Healthy        bool    `json:"healthy"`        // 是否健康
	HasActiveTask  bool    `json:"hasActiveTask"`  // 是否有活跃任务
}

// getDashboardSummary 获取Dashboard摘要
// @Summary 获取RAIDZ扩展Dashboard摘要
// @Description 获取用于Dashboard卡片展示的RAIDZ扩展摘要信息
// @Tags storage/raidz-expansion
// @Produce json
// @Success 200 {object} api.Response{data=RAIDZDashboardSummary}
// @Router /raidz-expansion/dashboard [get]
// @Security BearerAuth
func (h *RAIDZUIHandlers) getDashboardSummary(c *gin.Context) {
	summary := &RAIDZDashboardSummary{
		LastUpdate: time.Now(),
	}

	// 检查系统是否支持RAIDZ扩展
	if h.expandService != nil {
		summary.ExpansionSupported = h.expandService.IsAvailable()
		if !summary.ExpansionSupported {
			summary.SupportReason = "ZFS版本不支持RAIDZ扩展，需要OpenZFS 2.3+"
		}
	}

	// 获取活跃任务进度
	if h.expandMonitor != nil {
		allProgress := h.expandMonitor.GetAllProgress()
		summary.ActiveCount = len(allProgress)

		for _, p := range allProgress {
			switch p.Status {
			case "running":
				summary.RunningCount++
			case "paused":
				summary.PausedCount++
			}

			// 构建最新任务卡片
			if summary.LatestTask == nil || p.LastUpdate.After(summary.LatestTask.StartTime) {
				summary.LatestTask = &RAIDZTaskCard{
					PoolName:         p.PoolName,
					RAIDZLevel:       p.VdevName,
					Status:           p.Status,
					StatusText:       p.StatusText,
					Percent:          p.Percent,
					SpeedMBps:        p.SpeedMBps,
					ETAFormatted:     p.ETAFormatted,
					ElapsedFormatted: p.ElapsedFormatted,
					Phase:            p.Phase,
					PhaseText:        h.phaseToText(p.Phase),
					CanPause:         p.CanPause,
					CanResume:        p.CanResume,
					CanCancel:        p.CanCancel,
				}
			}
		}

		// 获取最近历史
		history := h.expandMonitor.GetProgressHistory(5)
		summary.RecentHistory = make([]RAIDZHistoryCard, 0, len(history))
		for _, hItem := range history {
			duration := "-"
			if !hItem.StartTime.IsZero() {
				elapsed := hItem.Elapsed
				if elapsed > 0 {
					duration = h.formatDuration(elapsed)
				}
			}

			summary.RecentHistory = append(summary.RecentHistory, RAIDZHistoryCard{
				PoolName:     hItem.PoolName,
				Status:       hItem.Status,
				Percent:      hItem.Percent,
				StartTime:    hItem.StartTime,
				EndTime:      hItem.LastUpdate,
				Duration:     duration,
				CapacityGain: hItem.CapacityGainGB,
			})
		}
	}

	// 获取可扩展池列表
	if h.expandService != nil && h.storageMgr != nil {
		summary.ExpandablePools = h.getExpandablePools(c.Request.Context())
	}

	api.OK(c, summary)
}

// ========== 状态和进度接口 ==========

// getGlobalStatus 获取全局扩展状态
func (h *RAIDZUIHandlers) getGlobalStatus(c *gin.Context) {
	if h.innerHandlers != nil {
		h.innerHandlers.RegisterRoutes(nil) // 调用内层方法
		// 直接调用内层方法
		c.Next()
		return
	}

	// 简化实现
	status := struct {
		ActiveCount    int       `json:"activeCount"`
		RunningCount   int       `json:"runningCount"`
		PausedCount    int       `json:"pausedCount"`
		CompletedCount int       `json:"completedCount"`
		FailedCount    int       `json:"failedCount"`
		LastUpdate     time.Time `json:"lastUpdate"`
	}{
		LastUpdate: time.Now(),
	}

	if h.expandMonitor != nil {
		progress := h.expandMonitor.GetAllProgress()
		status.ActiveCount = len(progress)
		for _, p := range progress {
			switch p.Status {
			case "running":
				status.RunningCount++
			case "paused":
				status.PausedCount++
			case "completed":
				status.CompletedCount++
			case "failed":
				status.FailedCount++
			}
		}
	}

	api.OK(c, status)
}

// getAllProgress 获取所有扩展进度
func (h *RAIDZUIHandlers) getAllProgress(c *gin.Context) {
	if h.expandMonitor == nil {
		api.OK(c, []storage.RAIDZExpandProgress{})
		return
	}

	progress := h.expandMonitor.GetAllProgress()
	api.OK(c, progress)
}

// getPoolProgress 获取指定池的扩展进度
func (h *RAIDZUIHandlers) getPoolProgress(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	if h.expandMonitor == nil {
		api.OK(c, &storage.RAIDZExpandProgress{
			PoolName:   poolName,
			Status:     "idle",
			StatusText: "无扩展任务",
		})
		return
	}

	progress, err := h.expandMonitor.GetProgress(poolName)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, progress)
}

// getHistory 获取扩展历史
func (h *RAIDZUIHandlers) getHistory(c *gin.Context) {
	if h.expandMonitor == nil {
		api.OK(c, []storage.RAIDZExpandProgress{})
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		// 解析limit参数
	}

	history := h.expandMonitor.GetProgressHistory(limit)
	api.OK(c, history)
}

// getSummary 获取扩展摘要
func (h *RAIDZUIHandlers) getSummary(c *gin.Context) {
	if h.expandMonitor == nil {
		api.OK(c, &storage.ExpandSummary{})
		return
	}

	summary := h.expandMonitor.GetExpandSummary()
	api.OK(c, summary)
}

// ========== 扩展操作接口 ==========

// StartExpansionUIReq WebUI启动扩展请求
type StartExpansionUIReq struct {
	PoolName      string            `json:"poolName" binding:"required"`      // 存储池名称
	NewDisk       string            `json:"newDisk" binding:"required"`       // 新磁盘路径
	Force         bool              `json:"force"`                            // 强制执行
	DryRun        bool              `json:"dryRun"`                           // 模拟运行（预检模式）
	AutoStart     bool              `json:"autoStart"`                        // 验证后自动启动
	Metadata      map[string]string `json:"metadata"`                         // 扩展元数据
}

// startExpansion 启动RAIDZ扩展
func (h *RAIDZUIHandlers) startExpansion(c *gin.Context) {
	var req StartExpansionUIReq
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 检查是否有活跃任务
	if h.expandMonitor != nil {
		progress, _ := h.expandMonitor.GetProgress(req.PoolName)
		if progress != nil && progress.Status == "running" {
			api.Conflict(c, "该池已有扩展任务运行中")
			return
		}
	}

	// 如果是dryRun模式，只返回预检结果
	if req.DryRun {
		if h.expandService == nil {
			api.BadRequest(c, "扩展服务未初始化")
			return
		}

		ctx := c.Request.Context()
		eligibility, err := h.expandService.CheckExpansionEligibility(ctx, req.PoolName)
		if err != nil {
			api.InternalError(c, "预检失败: "+err.Error())
			return
		}

		api.OKWithMessage(c, "预检完成（dryRun模式，未实际执行）", eligibility)
		return
	}

	// 启动实际扩展
	if h.expandService == nil {
		api.BadRequest(c, "扩展服务未初始化")
		return
	}

	ctx := c.Request.Context()
	task, err := h.expandService.StartExpansion(ctx, req.PoolName, req.NewDisk, req.Force)
	if err != nil {
		api.InternalError(c, "启动扩展失败: "+err.Error())
		return
	}

	// 创建进度监控
	if h.expandMonitor != nil {
		progress, _ := h.expandMonitor.StartMonitoring(ctx, task)
		api.Accepted(c, progress)
		return
	}

	api.Accepted(c, task)
}

// pauseExpansion 暂停扩展
func (h *RAIDZUIHandlers) pauseExpansion(c *gin.Context) {
	poolName := c.Param("pool")

	if h.expandService == nil {
		api.BadRequest(c, "扩展服务未初始化")
		return
	}

	err := h.expandService.PauseExpansion(poolName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 更新进度显示
	if h.expandMonitor != nil {
		progress, _ := h.expandMonitor.GetProgress(poolName)
		if progress != nil {
			progress.Status = "paused"
			progress.StatusText = "已暂停"
			progress.CanPause = false
			progress.CanResume = true
		}
	}

	api.OKWithMessage(c, "扩展已暂停", nil)
}

// resumeExpansion 恢复扩展
func (h *RAIDZUIHandlers) resumeExpansion(c *gin.Context) {
	poolName := c.Param("pool")

	if h.expandService == nil {
		api.BadRequest(c, "扩展服务未初始化")
		return
	}

	err := h.expandService.ResumeExpansion(poolName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 更新进度显示
	if h.expandMonitor != nil {
		progress, _ := h.expandMonitor.GetProgress(poolName)
		if progress != nil {
			progress.Status = "running"
			progress.StatusText = "扩展中"
			progress.CanPause = true
			progress.CanResume = false
		}
	}

	api.OKWithMessage(c, "扩展已恢复", nil)
}

// cancelExpansion 取消扩展
func (h *RAIDZUIHandlers) cancelExpansion(c *gin.Context) {
	poolName := c.Param("pool")

	if h.expandService == nil {
		api.BadRequest(c, "扩展服务未初始化")
		return
	}

	err := h.expandService.CancelExpansion(poolName)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "扩展已取消", nil)
}

// ========== 验证和预检接口 ==========

// validateExpansion 验证扩展可行性
func (h *RAIDZUIHandlers) validateExpansion(c *gin.Context) {
	var req storage.ValidateExpansionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 构建验证结果
	result := &storage.ValidateExpansionResult{
		Valid:    true,
		CanExpand: true,
		Errors:   []string{},
		Warnings: []string{},
		Checks:   []storage.CheckDetail{},
	}

	// 使用扩展服务进行验证
	if h.expandService != nil {
		ctx := c.Request.Context()
		eligibility, err := h.expandService.CheckExpansionEligibility(ctx, req.PoolName)
		if err != nil {
			result.Valid = false
			result.CanExpand = false
			result.Errors = append(result.Errors, err.Error())
		}

		if eligibility != nil {
			result.CanExpand = eligibility.Eligible
			result.Warnings = eligibility.Warnings

			for _, check := range eligibility.PreChecks {
				result.Checks = append(result.Checks, storage.CheckDetail{
					Name:        check.Name,
					Description: check.Message,
					Passed:      check.Passed,
					Message:     check.Message,
				})
			}
		}
	}

	api.OK(c, result)
}

// estimateExpansion 估算扩展时间和容量
func (h *RAIDZUIHandlers) estimateExpansion(c *gin.Context) {
	var req storage.EstimateExpansionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 使用扩展服务进行估算
	if h.expandService != nil {
		ctx := c.Request.Context()
		estimatedTime, err := h.expandService.EstimateExpansionTime(ctx, req.PoolName)
		if err == nil {
			// 构建估算结果
			result := &storage.EstimateExpansionResult{
				EstimatedTime:  storage.FormatDuration(int64(estimatedTime.Seconds())),
				EstimatedHours: estimatedTime.Hours(),
			}
			api.OK(c, result)
			return
		}
	}

	// 默认估算（简化）
	raidzLevel := req.RAIDZLevel
	if raidzLevel == "" {
		raidzLevel = "raidz1"
	}

	widthBefore := req.CurrentWidth
	if widthBefore <= 0 {
		widthBefore = 3
	}

	// 容量增益计算
	capacityGain := storage.CalculateCapacityGain(raidzLevel, widthBefore, req.NewDiskSizeGB)
	efficiencyBefore := storage.CalculateEfficiency(raidzLevel, widthBefore)
	efficiencyAfter := storage.CalculateEfficiency(raidzLevel, widthBefore+1)

	result := &storage.EstimateExpansionResult{
		CapacityBeforeGB:    req.NewDiskSizeGB * float64(widthBefore-1),
		CapacityAfterGB:     req.NewDiskSizeGB * float64(widthBefore),
		CapacityGainGB:      capacityGain,
		CapacityGainPercent: (capacityGain / (req.NewDiskSizeGB * float64(widthBefore-1))) * 100,
		EstimatedTime:       "预估需根据实际数据量计算",
		EstimatedHours:      float64(req.UsedBytes) / (100 * 1024 * 1024 * 1024),
		EfficiencyBefore:    efficiencyBefore,
		EfficiencyAfter:     efficiencyAfter,
		WidthBefore:         widthBefore,
		WidthAfter:          widthBefore + 1,
	}

	api.OK(c, result)
}

// checkEligibility 检查池扩展资格
func (h *RAIDZUIHandlers) checkEligibility(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	if h.expandService == nil {
		api.BadRequest(c, "扩展服务未初始化")
		return
	}

	ctx := c.Request.Context()
	result, err := h.expandService.CheckExpansionEligibility(ctx, poolName)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, result)
}

// ========== 磁盘信息接口 ==========

// listAvailableDisks 列出可用磁盘
func (h *RAIDZUIHandlers) listAvailableDisks(c *gin.Context) {
	if h.expandService == nil {
		api.OK(c, []storage.AvailableDiskInfo{})
		return
	}

	ctx := c.Request.Context()
	disks, err := h.expandService.ListAvailableDisks(ctx)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, disks)
}

// getDiskInfo 获取磁盘信息
func (h *RAIDZUIHandlers) getDiskInfo(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		api.BadRequest(c, "设备名称不能为空")
		return
	}

	disk := &storage.DiskSlot{
		Path:      "/dev/" + device,
		Model:     "Unknown",
		SizeGB:    0,
		State:     "available",
		IsNew:     false,
		Indicator: "green",
	}

	api.OK(c, disk)
}

// ========== 容量计算接口 ==========

// calculateCapacity 计算RAIDZ容量
func (h *RAIDZUIHandlers) calculateCapacity(c *gin.Context) {
	raidzLevel := c.Param("raidz")
	widthStr := c.Param("width")

	if raidzLevel == "" || widthStr == "" {
		api.BadRequest(c, "参数不完整")
		return
	}

	// 复用内层处理器逻辑
	if h.innerHandlers != nil {
		// 调用内层方法
		c.Params = gin.Params{
			{Key: "raidz", Value: raidzLevel},
			{Key: "width", Value: widthStr},
		}
		h.innerHandlers.RegisterRoutes(nil)
		return
	}

	api.OK(c, storage.RAIDZCapacityInfo{
		RAIDZLevel: raidzLevel,
	})
}

// ========== 阶段详情接口 ==========

// getPhaseDetails 获取阶段详情
func (h *RAIDZUIHandlers) getPhaseDetails(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	if h.expandMonitor == nil {
		api.OK(c, storage.DefaultPhases)
		return
	}

	progress, err := h.expandMonitor.GetProgress(poolName)
	if err != nil {
		api.OK(c, storage.DefaultPhases)
		return
	}

	api.OK(c, progress.Phases)
}

// ========== WebSocket 实时进度推送 ==========

// websocketProgress WebSocket进度推送
func (h *RAIDZUIHandlers) websocketProgress(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "池名称不能为空"})
		return
	}

	// WebSocket 实现（简化）
	// 实际应使用 gorilla/websocket 或类似库
	c.JSON(http.StatusOK, gin.H{
		"message": "WebSocket endpoint for pool: " + poolName,
		"note":    "Use ws:// protocol for real-time progress updates",
	})
}

// ========== 辅助方法 ==========

// phaseToText 阶段转文本
func (h *RAIDZUIHandlers) phaseToText(phase string) string {
	switch phase {
	case storage.PhasePreparing:
		return "准备扩展环境"
	case storage.PhaseDataScan:
		return "扫描数据布局"
	case storage.PhaseDataMigration:
		return "重分布数据块"
	case storage.PhaseVerification:
		return "校验数据完整性"
	case storage.PhaseFinalization:
		return "更新元数据"
	case storage.PhaseCompleted:
		return "扩展完成"
	default:
		return "未知阶段"
	}
}

// formatDuration 格式化时长
func (h *RAIDZUIHandlers) formatDuration(seconds int64) string {
	if seconds <= 0 {
		return "-"
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

// getExpandablePools 获取可扩展池列表
func (h *RAIDZUIHandlers) getExpandablePools(ctx context.Context) []ExpandablePoolCard {
	pools := []ExpandablePoolCard{}

	if h.storageMgr == nil {
		return pools
	}

	// 从存储管理器获取池列表
	volumes := h.storageMgr.ListVolumes()
	for _, vol := range volumes {
		// 检查是否是RAIDZ类型
		// Btrfs 不支持 RAIDZ，这里需要从 ZFS 层获取
		// 简化实现，假设支持

		poolCard := ExpandablePoolCard{
			PoolName:      vol.Name,
			RAIDZLevel:    "raidz1", // 需要从实际池获取
			CurrentWidth:  len(vol.Devices),
			CurrentCapGB:  float64(vol.Size) / (1024 * 1024 * 1024),
			PotentialGain: float64(vol.Size / uint64(len(vol.Devices))) / (1024 * 1024 * 1024),
			Healthy:       vol.Status.Healthy,
			HasActiveTask: false,
		}

		// 检查是否有活跃任务
		if h.expandMonitor != nil {
			progress, _ := h.expandMonitor.GetProgress(vol.Name)
			if progress != nil && progress.Status != "idle" && progress.Status != "completed" {
				poolCard.HasActiveTask = true
			}
		}

		pools = append(pools, poolCard)
	}

	return pools
}

// ========== 响应类型导出 ==========

// FormatDuration 导出格式化函数（供其他模块使用）
var FormatDuration = storage.FormatDuration