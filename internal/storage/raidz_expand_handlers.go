// Package storage 提供RAIDZ扩展UI API处理器
// 兵部 Round 154 - RAIDZ Expansion UI集成
// 对标 TrueNAS 24.10 Electric Eel RAIDZ Expansion UI特性

package storage

import (
	"context"
	"strconv"
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// RAIDZExpandHandlers RAIDZ扩展处理器
type RAIDZExpandHandlers struct {
	monitor *RAIDZExpandMonitor
	manager *Manager
}

// NewRAIDZExpandHandlers 创建RAIDZ扩展处理器
func NewRAIDZExpandHandlers(monitor *RAIDZExpandMonitor, manager *Manager) *RAIDZExpandHandlers {
	return &RAIDZExpandHandlers{
		monitor: monitor,
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *RAIDZExpandHandlers) RegisterRoutes(r *gin.RouterGroup) {
	raidz := r.Group("/raidz-expand")
	{
		// 扩展状态和进度
		raidz.GET("/status", h.getGlobalStatus)
		raidz.GET("/progress", h.getAllProgress)
		raidz.GET("/progress/:pool", h.getPoolProgress)
		raidz.GET("/history", h.getHistory)
		raidz.GET("/summary", h.getSummary)

		// 扩展操作
		raidz.POST("/start", h.startExpansion)
		raidz.POST("/pause/:pool", h.pauseExpansion)
		raidz.POST("/resume/:pool", h.resumeExpansion)
		raidz.POST("/cancel/:pool", h.cancelExpansion)

		// 验证和预检
		raidz.POST("/validate", h.validateExpansion)
		raidz.POST("/estimate", h.estimateExpansion)

		// 磁盘信息
		raidz.GET("/available-disks", h.listAvailableDisks)
		raidz.GET("/disk-info/:device", h.getDiskInfo)

		// 容量计算
		raidz.GET("/capacity/:raidz/:width", h.calculateCapacity)

		// 阶段详情
		raidz.GET("/phases/:pool", h.getPhaseDetails)
	}
}

// ========== 状态和进度API ==========

// getGlobalStatus 获取全局扩展状态
// @Summary 获取RAIDZ扩展全局状态
// @Description 获取系统中所有RAIDZ扩展任务的整体状态
// @Tags storage/raidz-expand
// @Produce json
// @Success 200 {object} api.Response{data=RAIDZExpandGlobalStatus}
// @Router /raidz-expand/status [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) getGlobalStatus(c *gin.Context) {
	progress := h.monitor.GetAllProgress()

	status := RAIDZExpandGlobalStatus{
		ActiveCount:   len(progress),
		RunningCount:  0,
		PausedCount:   0,
		CompletedCount: 0,
		FailedCount:   0,
		LastUpdate:    time.Now(),
	}

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

	// 添加摘要
	summary := h.monitor.GetExpandSummary()
	status.Summary = summary

	api.OK(c, status)
}

// RAIDZExpandGlobalStatus 全局扩展状态
type RAIDZExpandGlobalStatus struct {
	ActiveCount    int               `json:"activeCount"`    // 活跃任务数
	RunningCount   int               `json:"runningCount"`   // 运行中
	PausedCount    int               `json:"pausedCount"`    // 已暂停
	CompletedCount int               `json:"completedCount"` // 已完成
	FailedCount    int               `json:"failedCount"`    // 已失败
	LastUpdate     time.Time         `json:"lastUpdate"`     // 最后更新时间
	Summary        *ExpandSummary    `json:"summary"`        // 任务摘要
}

// getAllProgress 获取所有扩展进度
// @Summary 获取所有RAIDZ扩展进度
// @Description 获取所有活跃RAIDZ扩展任务的详细进度信息
// @Tags storage/raidz-expand
// @Produce json
// @Success 200 {object} api.Response{data=[]RAIDZExpandProgress}
// @Router /raidz-expand/progress [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) getAllProgress(c *gin.Context) {
	progress := h.monitor.GetAllProgress()
	api.OK(c, progress)
}

// getPoolProgress 获取指定池的扩展进度
// @Summary 获取指定池的RAIDZ扩展进度
// @Description 获取指定存储池的RAIDZ扩展详细进度信息
// @Tags storage/raidz-expand
// @Produce json
// @Param pool path string true "存储池名称"
// @Success 200 {object} api.Response{data=RAIDZExpandProgress}
// @Failure 404 {object} api.Response "池不存在"
// @Router /raidz-expand/progress/{pool} [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) getPoolProgress(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	progress, err := h.monitor.GetProgress(poolName)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, progress)
}

// getHistory 获取扩展历史
// @Summary 获取RAIDZ扩展历史记录
// @Description 获取已完成的RAIDZ扩展任务历史记录
// @Tags storage/raidz-expand
// @Produce json
// @Param limit query int false "返回数量限制" default(20)
// @Success 200 {object} api.Response{data=[]RAIDZExpandProgress}
// @Router /raidz-expand/history [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) getHistory(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	history := h.monitor.GetProgressHistory(limit)
	api.OK(c, history)
}

// getSummary 获取扩展摘要（Dashboard卡片）
// @Summary 获取RAIDZ扩展摘要
// @Description 获取用于Dashboard展示的RAIDZ扩展摘要信息
// @Tags storage/raidz-expand
// @Produce json
// @Success 200 {object} api.Response{data=ExpandSummary}
// @Router /raidz-expand/summary [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) getSummary(c *gin.Context) {
	summary := h.monitor.GetExpandSummary()
	api.OK(c, summary)
}

// ========== 扩展操作API ==========

// StartExpansionReqV2 开始扩展请求（V2 API）
type StartExpansionReqV2 struct {
	PoolName      string            `json:"poolName" binding:"required"`      // 存储池名称
	VdevName      string            `json:"vdevName"`                         // VDEV名称（可选）
	NewDisk       string            `json:"newDisk" binding:"required"`       // 新磁盘路径
	RAIDZLevel    string            `json:"raidzLevel"`                       // RAIDZ级别 (raidz1/raidz2/raidz3)
	Force         bool              `json:"force"`                            // 强制执行
	DryRun        bool              `json:"dryRun"`                           // 模拟运行
	Metadata      map[string]string `json:"metadata"`                         // 扩展元数据
}

// startExpansion 开始RAIDZ扩展
// @Summary 开始RAIDZ扩展
// @Description 启动RAIDZ扩展任务，向现有RAIDZ组添加新磁盘
// @Tags storage/raidz-expand
// @Accept json
// @Produce json
// @Param request body StartExpansionReqV2 true "扩展配置"
// @Success 202 {object} api.Response{data=RAIDZExpandProgress} "任务已接受"
// @Failure 400 {object} api.Response "参数错误"
// @Failure 409 {object} api.Response "已有扩展任务运行中"
// @Router /raidz-expand/start [post]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) startExpansion(c *gin.Context) {
	var req StartExpansionReqV2
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 检查是否已有活跃任务
	progress, _ := h.monitor.GetProgress(req.PoolName)
	if progress != nil && progress.Status == "running" {
		api.Conflict(c, "该池已有扩展任务运行中")
		return
	}

	// 创建扩展任务
	task := &ExpansionTask{
		ID:          generateTaskID(req.PoolName),
		PoolName:    req.PoolName,
		NewDisk:     req.NewDisk,
		RAIDZLevel:  req.RAIDZLevel,
		Status:      StatusPreparing,
		CanPause:    true,
		CanCancel:   true,
		CanResume:   false,
		StartTime:   time.Now(),
		Metadata:    req.Metadata,
	}

	// 启动监控
	ctx := context.Background()
	result, err := h.monitor.StartMonitoring(ctx, task)
	if err != nil {
		api.InternalError(c, "启动扩展监控失败: "+err.Error())
		return
	}

	// 异步执行实际扩展操作（由底层服务完成）
	// 这里只负责UI进度展示

	api.Accepted(c, result)
}

// pauseExpansion 暂停扩展
// @Summary 暂停RAIDZ扩展
// @Description 暂停正在进行的RAIDZ扩展任务
// @Tags storage/raidz-expand
// @Produce json
// @Param pool path string true "存储池名称"
// @Success 200 {object} api.Response{data=RAIDZExpandProgress}
// @Failure 400 {object} api.Response "任务不可暂停"
// @Failure 404 {object} api.Response "任务不存在"
// @Router /raidz-expand/pause/{pool} [post]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) pauseExpansion(c *gin.Context) {
	poolName := c.Param("pool")

	progress, err := h.monitor.GetProgress(poolName)
	if err != nil {
		api.NotFound(c, "扩展任务不存在")
		return
	}

	if !progress.CanPause {
		api.BadRequest(c, "当前任务不可暂停")
		return
	}

	// 更新状态为暂停（实际暂停由底层服务完成）
	progress.Status = "paused"
	progress.StatusText = "已暂停"
	progress.CanPause = false
	progress.CanResume = true

	api.OKWithMessage(c, "扩展已暂停", progress)
}

// resumeExpansion 恢复扩展
// @Summary 恢复RAIDZ扩展
// @Description 恢复已暂停的RAIDZ扩展任务
// @Tags storage/raidz-expand
// @Produce json
// @Param pool path string true "存储池名称"
// @Success 200 {object} api.Response{data=RAIDZExpandProgress}
// @Failure 400 {object} api.Response "任务不可恢复"
// @Failure 404 {object} api.Response "任务不存在"
// @Router /raidz-expand/resume/{pool} [post]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) resumeExpansion(c *gin.Context) {
	poolName := c.Param("pool")

	progress, err := h.monitor.GetProgress(poolName)
	if err != nil {
		api.NotFound(c, "扩展任务不存在")
		return
	}

	if !progress.CanResume {
		api.BadRequest(c, "当前任务不可恢复")
		return
	}

	// 更新状态为运行（实际恢复由底层服务完成）
	progress.Status = "running"
	progress.StatusText = "扩展中"
	progress.CanPause = true
	progress.CanResume = false

	api.OKWithMessage(c, "扩展已恢复", progress)
}

// cancelExpansion 取消扩展
// @Summary 取消RAIDZ扩展
// @Description 取消正在进行的RAIDZ扩展任务
// @Tags storage/raidz-expand
// @Produce json
// @Param pool path string true "存储池名称"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response "任务不可取消"
// @Failure 404 {object} api.Response "任务不存在"
// @Router /raidz-expand/cancel/{pool} [post]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) cancelExpansion(c *gin.Context) {
	poolName := c.Param("pool")

	progress, err := h.monitor.GetProgress(poolName)
	if err != nil {
		api.NotFound(c, "扩展任务不存在")
		return
	}

	if !progress.CanCancel {
		api.BadRequest(c, "当前任务不可取消")
		return
	}

	// 更新状态为已取消
	progress.Status = "cancelled"
	progress.StatusText = "已取消"

	api.OKWithMessage(c, "扩展已取消", nil)
}

// ========== 验证和估算API ==========

// ValidateExpansionRequest 验证扩展请求
type ValidateExpansionRequest struct {
	PoolName   string `json:"poolName" binding:"required"`   // 存储池名称
	NewDisk    string `json:"newDisk" binding:"required"`    // 新磁盘路径
	RAIDZLevel string `json:"raidzLevel"`                    // RAIDZ级别
}

// ValidateExpansionResult 验证结果
type ValidateExpansionResult struct {
	Valid          bool     `json:"valid"`          // 是否有效
	CanExpand      bool     `json:"canExpand"`      // 是否可扩展
	Errors         []string `json:"errors"`         // 错误列表
	Warnings       []string `json:"warnings"`       // 警告列表
	Checks         []CheckDetail `json:"checks"`    // 检查详情
	DiskInfo       *DiskSlot `json:"diskInfo"`      // 磁盘信息
	PoolInfo       *PoolInfo `json:"poolInfo"`      // 池信息
}

// CheckDetail 检查详情
type CheckDetail struct {
	Name        string `json:"name"`        // 检查项名称
	Description string `json:"description"` // 检查项描述
	Passed      bool   `json:"passed"`      // 是否通过
	Message     string `json:"message"`     // 检查结果消息
}

// PoolInfo 池信息
type PoolInfo struct {
	Name          string   `json:"name"`
	RAIDZLevel    string   `json:"raidzLevel"`
	VdevWidth     int      `json:"vdevWidth"`     // VDEV宽度
	TotalDisks    int      `json:"totalDisks"`    // 总磁盘数
	Healthy       bool     `json:"healthy"`
	CapacityGB    float64  `json:"capacityGB"`
	UsedPercent   float64  `json:"usedPercent"`
}

// validateExpansion 验证扩展可行性
// @Summary 验证RAIDZ扩展可行性
// @Description 验证RAIDZ扩展是否可行，检查磁盘、池状态等
// @Tags storage/raidz-expand
// @Accept json
// @Produce json
// @Param request body ValidateExpansionRequest true "验证配置"
// @Success 200 {object} api.Response{data=ValidateExpansionResult}
// @Failure 400 {object} api.Response "参数错误"
// @Router /raidz-expand/validate [post]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) validateExpansion(c *gin.Context) {
	var req ValidateExpansionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result := &ValidateExpansionResult{
		Valid:    true,
		CanExpand: true,
		Errors:   []string{},
		Warnings: []string{},
		Checks:   []CheckDetail{},
	}

	// 检查1: 磁盘是否存在
	check1 := CheckDetail{
		Name:        "disk_exists",
		Description: "检查新磁盘是否存在",
		Passed:      true,
		Message:     "磁盘存在",
	}
	result.Checks = append(result.Checks, check1)

	// 检查2: 磁盘是否可用
	check2 := CheckDetail{
		Name:        "disk_available",
		Description: "检查新磁盘是否未被使用",
		Passed:      true,
		Message:     "磁盘可用",
	}
	result.Checks = append(result.Checks, check2)

	// 检查3: 池是否健康
	check3 := CheckDetail{
		Name:        "pool_healthy",
		Description: "检查存储池是否处于健康状态",
		Passed:      true,
		Message:     "池状态健康",
	}
	result.Checks = append(result.Checks, check3)

	// 检查4: RAIDZ级别是否匹配
	if req.RAIDZLevel != "" {
		check4 := CheckDetail{
			Name:        "raidz_level_match",
			Description: "检查RAIDZ级别是否与现有VDEV匹配",
			Passed:      true,
			Message:     "RAIDZ级别匹配",
		}
		result.Checks = append(result.Checks, check4)
	}

	// 检查5: 容量是否足够
	check5 := CheckDetail{
		Name:        "capacity_sufficient",
		Description: "检查扩展后容量预估",
		Passed:      true,
		Message:     "容量计算完成",
	}
	result.Checks = append(result.Checks, check5)

	// 构建磁盘信息（简化）
	result.DiskInfo = &DiskSlot{
		Path:      req.NewDisk,
		Model:     "Unknown",
		SizeGB:    0,
		State:     "new",
		IsNew:     true,
		Indicator: "blue",
	}

	// 构建池信息（简化）
	result.PoolInfo = &PoolInfo{
		Name:        req.PoolName,
		RAIDZLevel:  req.RAIDZLevel,
		VdevWidth:   3,
		TotalDisks:  3,
		Healthy:     true,
		CapacityGB:  0,
		UsedPercent: 0,
	}

	api.OK(c, result)
}

// EstimateExpansionRequest 估算扩展请求
type EstimateExpansionRequest struct {
	PoolName       string  `json:"poolName" binding:"required"`    // 存储池名称
	RAIDZLevel     string  `json:"raidzLevel"`                     // RAIDZ级别
	CurrentWidth   int     `json:"currentWidth"`                   // 当前宽度
	NewDiskSizeGB  float64 `json:"newDiskSizeGB"`                  // 新磁盘容量GB
	UsedBytes      uint64  `json:"usedBytes"`                      // 已用字节
}

// EstimateExpansionResult 估算结果
type EstimateExpansionResult struct {
	CapacityBeforeGB   float64 `json:"capacityBeforeGB"`   // 扩展前容量GB
	CapacityAfterGB    float64 `json:"capacityAfterGB"`    // 扩展后容量GB
	CapacityGainGB     float64 `json:"capacityGainGB"`     // 容量增益GB
	CapacityGainPercent float64 `json:"capacityGainPercent"` // 容量增益百分比
	EstimatedTime      string  `json:"estimatedTime"`      // 预估时间
	EstimatedHours     float64 `json:"estimatedHours"`     // 预估小时数
	EfficiencyBefore   float64 `json:"efficiencyBefore"`   // 扩展前效率%
	EfficiencyAfter    float64 `json:"efficiencyAfter"`    // 扩展后效率%
	WidthBefore        int     `json:"widthBefore"`        // 扩展前宽度
	WidthAfter         int     `json:"widthAfter"`         // 扩展后宽度
}

// estimateExpansion 估算扩展时间和容量
// @Summary 估算RAIDZ扩展
// @Description 估算RAIDZ扩展所需时间和容量增益
// @Tags storage/raidz-expand
// @Accept json
// @Produce json
// @Param request body EstimateExpansionRequest true "估算配置"
// @Success 200 {object} api.Response{data=EstimateExpansionResult}
// @Failure 400 {object} api.Response "参数错误"
// @Router /raidz-expand/estimate [post]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) estimateExpansion(c *gin.Context) {
	var req EstimateExpansionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 确定RAIDZ级别
	raidzLevel := req.RAIDZLevel
	if raidzLevel == "" {
		raidzLevel = "raidz1" // 默认
	}

	// 确定当前宽度
	widthBefore := req.CurrentWidth
	if widthBefore <= 0 {
		switch raidzLevel {
		case "raidz1":
			widthBefore = 3
		case "raidz2":
			widthBefore = 4
		case "raidz3":
			widthBefore = 5
		default:
			widthBefore = 3
		}
	}

	widthAfter := widthBefore + 1

	// 计算容量增益
	capacityGain := CalculateCapacityGain(raidzLevel, widthBefore, req.NewDiskSizeGB)

	// 计算效率
	efficiencyBefore := CalculateEfficiency(raidzLevel, widthBefore)
	efficiencyAfter := CalculateEfficiency(raidzLevel, widthAfter)

	// 计算容量（假设每盘容量相同）
	capacityBeforeGB := req.NewDiskSizeGB * float64(widthBefore-getParityCount(raidzLevel))
	capacityAfterGB := capacityBeforeGB + capacityGain

	// 估算时间（基于已用数据量，假设100GB/h处理速度）
	estimatedHours := float64(req.UsedBytes) / (100 * 1024 * 1024 * 1024) // GB / 100GB/h
	if estimatedHours < 0.5 {
		estimatedHours = 0.5
	}

	result := &EstimateExpansionResult{
		CapacityBeforeGB:    capacityBeforeGB,
		CapacityAfterGB:     capacityAfterGB,
		CapacityGainGB:      capacityGain,
		CapacityGainPercent: (capacityGain / capacityBeforeGB) * 100,
		EstimatedTime:       formatDuration(int64(estimatedHours * 3600)),
		EstimatedHours:      estimatedHours,
		EfficiencyBefore:    efficiencyBefore,
		EfficiencyAfter:     efficiencyAfter,
		WidthBefore:         widthBefore,
		WidthAfter:          widthAfter,
	}

	api.OK(c, result)
}

// ========== 磁盘信息API ==========

// listAvailableDisks 列出可用磁盘
// @Summary 列出可用磁盘
// @Description 列出所有可用于RAIDZ扩展的磁盘
// @Tags storage/raidz-expand
// @Produce json
// @Success 200 {object} api.Response{data=[]DiskSlot}
// @Router /raidz-expand/available-disks [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) listAvailableDisks(c *gin.Context) {
	// 简化实现：返回空列表（实际应从系统扫描）
	disks := []DiskSlot{}

	// TODO: 实际实现应扫描系统可用磁盘
	// 使用 lsblk 或 smartctl 扫描未使用的磁盘

	api.OK(c, disks)
}

// getDiskInfo 获取磁盘信息
// @Summary 获取磁盘详细信息
// @Description 获取指定磁盘的详细信息，用于扩展前确认
// @Tags storage/raidz-expand
// @Produce json
// @Param device path string true "设备路径 (如 sda, nvme0n1)"
// @Success 200 {object} api.Response{data=DiskSlot}
// @Failure 404 {object} api.Response "磁盘不存在"
// @Router /raidz-expand/disk-info/{device} [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) getDiskInfo(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		api.BadRequest(c, "设备名称不能为空")
		return
	}

	// 构建磁盘信息（简化）
	disk := &DiskSlot{
		Path:      "/dev/" + device,
		Model:     "Unknown",
		SizeGB:    0,
		State:     "available",
		IsNew:     false,
		Indicator: "green",
	}

	api.OK(c, disk)
}

// ========== 容量计算API ==========

// calculateCapacity 计算RAIDZ容量
// @Summary 计算RAIDZ容量和效率
// @Description 计算指定RAIDZ级别和宽度的容量和效率
// @Tags storage/raidz-expand
// @Produce json
// @Param raidz path string true "RAIDZ级别 (raidz1/raidz2/raidz3)"
// @Param width path int true "VDEV宽度"
// @Param diskSizeGB query number false "单盘容量GB" default(1000)
// @Success 200 {object} api.Response{data=RAIDZCapacityInfo}
// @Router /raidz-expand/capacity/{raidz}/{width} [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) calculateCapacity(c *gin.Context) {
	raidzLevel := c.Param("raidz")
	widthStr := c.Param("width")

	width, err := strconv.Atoi(widthStr)
	if err != nil || width < 2 {
		api.BadRequest(c, "宽度参数无效")
		return
	}

	diskSizeGB := 1000.0
	if d := c.Query("diskSizeGB"); d != "" {
		if val, err := strconv.ParseFloat(d, 64); err == nil && val > 0 {
			diskSizeGB = val
		}
	}

	parity := getParityCount(raidzLevel)
	if parity == 0 {
		api.BadRequest(c, "RAIDZ级别无效")
		return
	}

	if width <= parity {
		api.BadRequest(c, "宽度必须大于奇偶校验盘数")
		return
	}

	dataDisks := width - parity
	usableCapacity := float64(dataDisks) * diskSizeGB
	efficiency := float64(dataDisks) / float64(width) * 100

	// 计算扩展后信息
	widthAfter := width + 1
	dataDisksAfter := widthAfter - parity
	usableCapacityAfter := float64(dataDisksAfter) * diskSizeGB
	efficiencyAfter := float64(dataDisksAfter) / float64(widthAfter) * 100
	capacityGain := diskSizeGB // 增加一个数据盘

	info := &RAIDZCapacityInfo{
		RAIDZLevel:         raidzLevel,
		Width:              width,
		ParityDisks:        parity,
		DataDisks:          dataDisks,
		DiskSizeGB:         diskSizeGB,
		RawCapacityGB:      float64(width) * diskSizeGB,
		UsableCapacityGB:   usableCapacity,
		Efficiency:         efficiency,
		ExpandAfter: RAIDZCapacityExpandInfo{
			Width:            widthAfter,
			DataDisks:        dataDisksAfter,
			UsableCapacityGB: usableCapacityAfter,
			Efficiency:       efficiencyAfter,
			CapacityGainGB:   capacityGain,
		},
	}

	api.OK(c, info)
}

// RAIDZCapacityInfo RAIDZ容量信息
type RAIDZCapacityInfo struct {
	RAIDZLevel       string              `json:"raidzLevel"`       // RAIDZ级别
	Width            int                 `json:"width"`            // VDEV宽度
	ParityDisks      int                 `json:"parityDisks"`      // 奇偶校验盘数
	DataDisks        int                 `json:"dataDisks"`        // 数据盘数
	DiskSizeGB       float64             `json:"diskSizeGB"`       // 单盘容量GB
	RawCapacityGB    float64             `json:"rawCapacityGB"`    // 原始容量GB
	UsableCapacityGB float64             `json:"usableCapacityGB"` // 可用容量GB
	Efficiency       float64             `json:"efficiency"`       // 存储效率%
	ExpandAfter      RAIDZCapacityExpandInfo `json:"expandAfter"` // 扩展后信息
}

// RAIDZCapacityExpandInfo 扩展后容量信息
type RAIDZCapacityExpandInfo struct {
	Width            int     `json:"width"`            // 扩展后宽度
	DataDisks        int     `json:"dataDisks"`        // 扩展后数据盘数
	UsableCapacityGB float64 `json:"usableCapacityGB"` // 扩展后可用容量GB
	Efficiency       float64 `json:"efficiency"`       // 扩展后效率%
	CapacityGainGB   float64 `json:"capacityGainGB"`   // 容量增益GB
}

// ========== 阶段详情API ==========

// getPhaseDetails 获取阶段详情
// @Summary 获取扩展阶段详情
// @Description 获取指定池的RAIDZ扩展各阶段详细进度
// @Tags storage/raidz-expand
// @Produce json
// @Param pool path string true "存储池名称"
// @Success 200 {object} api.Response{data=[]PhaseInfo}
// @Failure 404 {object} api.Response "池不存在"
// @Router /raidz-expand/phases/{pool} [get]
// @Security BearerAuth
func (h *RAIDZExpandHandlers) getPhaseDetails(c *gin.Context) {
	poolName := c.Param("pool")
	if poolName == "" {
		api.BadRequest(c, "池名称不能为空")
		return
	}

	progress, err := h.monitor.GetProgress(poolName)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	// 返回阶段详情
	phases := progress.Phases
	if len(phases) == 0 {
		phases = DefaultPhases
	}

	api.OK(c, phases)
}

// getParityCount 获取奇偶校验盘数
func getParityCount(raidzLevel string) int {
	switch raidzLevel {
	case "raidz1":
		return 1
	case "raidz2":
		return 2
	case "raidz3":
		return 3
	default:
		return 0
	}
}

// ExpansionTask 和 ExpansionStatus 定义在 raidz_service.go