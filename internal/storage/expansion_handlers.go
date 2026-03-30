// Package storage 提供存储扩展 API 处理器
// RAID扩展功能API端点
package storage

import (
	"context"

	"nas-os/internal/api"
	btrfs "nas-os/pkg/storage/btrfs"

	"github.com/gin-gonic/gin"
)

// ========== 扩展管理器集成 ==========

// RAIDExpansionHandlers RAID扩展处理器
type RAIDExpansionHandlers struct {
	expansionManager *btrfs.RAIDExpansionManager
	manager          *Manager
}

// NewRAIDExpansionHandlers 创建RAID扩展处理器
func NewRAIDExpansionHandlers(expansionManager *btrfs.RAIDExpansionManager, manager *Manager) *RAIDExpansionHandlers {
	return &RAIDExpansionHandlers{
		expansionManager: expansionManager,
		manager:          manager,
	}
}

// RegisterExpansionRoutes 注册扩展路由
func (h *Handlers) RegisterExpansionRoutes(r *gin.RouterGroup, expansionManager *btrfs.RAIDExpansionManager) {
	if expansionManager == nil {
		return
	}

	expansionHandlers := NewRAIDExpansionHandlers(expansionManager, h.manager)

	expansion := r.Group("/expansion")
	{
		// 扩展状态
		expansion.GET("/status", expansionHandlers.getExpansionStatus)
		expansion.GET("/history", expansionHandlers.getExpansionHistory)

		// 可用磁盘
		expansion.GET("/available-disks", expansionHandlers.listAvailableDisks)

		// 设备验证
		expansion.POST("/validate/device", expansionHandlers.validateDevice)

		// 卷验证
		expansion.POST("/validate/volume", expansionHandlers.validateVolume)

		// 扩展操作
		expansion.POST("/start", expansionHandlers.startExpansion)
		expansion.POST("/pause", expansionHandlers.pauseExpansion)
		expansion.POST("/resume", expansionHandlers.resumeExpansion)
		expansion.POST("/cancel", expansionHandlers.cancelExpansion)

		// 估算
		expansion.POST("/estimate/time", expansionHandlers.estimateExpansionTime)
		expansion.POST("/estimate/capacity", expansionHandlers.estimateCapacityGain)

		// RAID配置信息
		expansion.GET("/raid-configs", expansionHandlers.getRAIDExpansionConfigs)
	}

	// 卷级别的扩展操作
	volumes := r.Group("/volumes")
	{
		volumes.POST("/:name/expand", expansionHandlers.expandVolume)
		volumes.GET("/:name/expand/status", expansionHandlers.getVolumeExpansionStatus)
		volumes.GET("/:name/expand/estimate", expansionHandlers.estimateVolumeExpansion)
	}
}

// ========== API处理器 ==========

// getExpansionStatus 获取当前扩展状态
// @Summary 获取扩展状态
// @Description 获取当前正在进行的RAID扩展状态
// @Tags storage/expansion
// @Produce json
// @Success 200 {object} api.Response{data=btrfs.ExpansionStatus}
// @Router /expansion/status [get]
func (h *RAIDExpansionHandlers) getExpansionStatus(c *gin.Context) {
	status := h.expansionManager.GetExpansionStatus()
	api.OK(c, status)
}

// getExpansionHistory 获取扩展历史
// @Summary 获取扩展历史
// @Description 获取RAID扩展历史记录
// @Tags storage/expansion
// @Produce json
// @Success 200 {object} api.Response{data=[]btrfs.ExpansionStatus}
// @Router /expansion/history [get]
func (h *RAIDExpansionHandlers) getExpansionHistory(c *gin.Context) {
	history := h.expansionManager.GetExpansionHistory()
	api.OK(c, history)
}

// listAvailableDisks 列出可用磁盘
// @Summary 列出可用磁盘
// @Description 列出所有可用于扩展的磁盘
// @Tags storage/expansion
// @Produce json
// @Success 200 {object} api.Response{data=[]btrfs.DeviceValidationResult}
// @Router /expansion/available-disks [get]
func (h *RAIDExpansionHandlers) listAvailableDisks(c *gin.Context) {
	ctx := context.Background()
	disks, err := h.expansionManager.ListAvailableDisks(ctx)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, disks)
}

// validateDeviceRequest 验证设备请求
type validateDeviceRequest struct {
	Device string `json:"device" binding:"required"`
}

// validateDevice 验证设备
// @Summary 验证设备
// @Description 验证设备是否可用于RAID扩展
// @Tags storage/expansion
// @Accept json
// @Produce json
// @Param request body validateDeviceRequest true "设备路径"
// @Success 200 {object} api.Response{data=btrfs.DeviceValidationResult}
// @Router /expansion/validate/device [post]
func (h *RAIDExpansionHandlers) validateDevice(c *gin.Context) {
	var req validateDeviceRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	result, err := h.expansionManager.ValidateDevice(ctx, req.Device)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, result)
}

// validateVolumeRequest 验证卷请求
type validateVolumeRequest struct {
	MountPoint string `json:"mountPoint" binding:"required"`
}

// validateVolume 验证卷
// @Summary 验证卷
// @Description 验证卷是否可进行RAID扩展
// @Tags storage/expansion
// @Accept json
// @Produce json
// @Param request body validateVolumeRequest true "挂载点"
// @Success 200 {object} api.Response{data=btrfs.VolumeValidationResult}
// @Router /expansion/validate/volume [post]
func (h *RAIDExpansionHandlers) validateVolume(c *gin.Context) {
	var req validateVolumeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	result, err := h.expansionManager.ValidateVolume(ctx, req.MountPoint)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, result)
}

// startExpansionRequest 开始扩展请求
type startExpansionRequest struct {
	VolumeName    string `json:"volumeName" binding:"required"`
	MountPoint    string `json:"mountPoint" binding:"required"`
	NewDevice     string `json:"newDevice" binding:"required"`
	TargetProfile string `json:"targetProfile"`
	AutoBalance   bool   `json:"autoBalance"`
	Force         bool   `json:"force"`
	DryRun        bool   `json:"dryRun"`
}

// startExpansion 开始扩展
// @Summary 开始RAID扩展
// @Description 启动RAID扩展任务
// @Tags storage/expansion
// @Accept json
// @Produce json
// @Param request body startExpansionRequest true "扩展配置"
// @Success 200 {object} api.Response{data=btrfs.ExpansionStatus}
// @Router /expansion/start [post]
func (h *RAIDExpansionHandlers) startExpansion(c *gin.Context) {
	var req startExpansionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	config := btrfs.ExpansionConfig{
		VolumeName:    req.VolumeName,
		MountPoint:    req.MountPoint,
		NewDevice:     req.NewDevice,
		TargetProfile: req.TargetProfile,
		AutoBalance:   req.AutoBalance,
		Force:         req.Force,
		DryRun:        req.DryRun,
	}

	// 默认启用自动平衡
	if config.AutoBalance == false && config.TargetProfile == "" {
		config.AutoBalance = true
	}

	status, err := h.expansionManager.StartExpansion(ctx, config)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, status)
}

// pauseExpansion 暂停扩展
// @Summary 暂停扩展
// @Description 暂停正在进行的RAID扩展
// @Tags storage/expansion
// @Success 200 {object} api.Response
// @Router /expansion/pause [post]
func (h *RAIDExpansionHandlers) pauseExpansion(c *gin.Context) {
	if err := h.expansionManager.PauseExpansion(); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "扩展已暂停", nil)
}

// resumeExpansionRequest 恢复扩展请求
type resumeExpansionRequest struct {
	VolumeName    string `json:"volumeName" binding:"required"`
	MountPoint    string `json:"mountPoint" binding:"required"`
	NewDevice     string `json:"newDevice"`
	TargetProfile string `json:"targetProfile"`
}

// resumeExpansion 恢复扩展
// @Summary 恢复扩展
// @Description 恢复已暂停的RAID扩展
// @Tags storage/expansion
// @Accept json
// @Produce json
// @Param request body resumeExpansionRequest true "恢复配置"
// @Success 200 {object} api.Response{data=btrfs.ExpansionStatus}
// @Router /expansion/resume [post]
func (h *RAIDExpansionHandlers) resumeExpansion(c *gin.Context) {
	var req resumeExpansionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	config := btrfs.ExpansionConfig{
		VolumeName:    req.VolumeName,
		MountPoint:    req.MountPoint,
		NewDevice:     req.NewDevice,
		TargetProfile: req.TargetProfile,
		AutoBalance:   true,
	}

	status, err := h.expansionManager.ResumeExpansion(ctx, config)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, status)
}

// cancelExpansion 取消扩展
// @Summary 取消扩展
// @Description 取消正在进行的RAID扩展
// @Tags storage/expansion
// @Success 200 {object} api.Response
// @Router /expansion/cancel [post]
func (h *RAIDExpansionHandlers) cancelExpansion(c *gin.Context) {
	if err := h.expansionManager.CancelExpansion(); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "扩展已取消", nil)
}

// estimateTimeRequest 估算时间请求
type estimateTimeRequest struct {
	MountPoint string `json:"mountPoint" binding:"required"`
}

// estimateExpansionTime 估算扩展时间
// @Summary 估算扩展时间
// @Description 估算RAID扩展所需时间
// @Tags storage/expansion
// @Accept json
// @Produce json
// @Param request body estimateTimeRequest true "挂载点"
// @Success 200 {object} api.Response{data=map[string]string}
// @Router /expansion/estimate/time [post]
func (h *RAIDExpansionHandlers) estimateExpansionTime(c *gin.Context) {
	var req estimateTimeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	duration, err := h.expansionManager.EstimateExpansionTime(ctx, req.MountPoint)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, map[string]string{
		"estimatedTime": duration.String(),
		"seconds":       string(rune(duration.Seconds())),
	})
}

// estimateCapacityRequest 估算容量请求
type estimateCapacityRequest struct {
	MountPoint     string `json:"mountPoint" binding:"required"`
	NewDeviceSize  uint64 `json:"newDeviceSize"`
	CurrentProfile string `json:"currentProfile"`
}

// estimateCapacityGain 估算容量增益
// @Summary 估算容量增益
// @Description 估算RAID扩展后的容量增益
// @Tags storage/expansion
// @Accept json
// @Produce json
// @Param request body estimateCapacityRequest true "估算参数"
// @Success 200 {object} api.Response{data=btrfs.CapacityEstimate}
// @Router /expansion/estimate/capacity [post]
func (h *RAIDExpansionHandlers) estimateCapacityGain(c *gin.Context) {
	var req estimateCapacityRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	estimate, err := h.expansionManager.EstimateCapacityGain(ctx, req.MountPoint, req.NewDeviceSize, req.CurrentProfile)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, estimate)
}

// getRAIDExpansionConfigs 获取RAID扩展配置
// @Summary 获取RAID扩展配置
// @Description 获取所有支持的RAID扩展配置
// @Tags storage/expansion
// @Produce json
// @Success 200 {object} api.Response{data=map[string]btrfs.BtrfsRAIDConfig}
// @Router /expansion/raid-configs [get]
func (h *RAIDExpansionHandlers) getRAIDExpansionConfigs(c *gin.Context) {
	api.OK(c, btrfs.PredefinedRAIDConfigs)
}

// expandVolumeRequest 扩展卷请求
type expandVolumeRequest struct {
	NewDevice     string `json:"newDevice" binding:"required"`
	TargetProfile string `json:"targetProfile"`
	AutoBalance   bool   `json:"autoBalance"`
	Force         bool   `json:"force"`
	DryRun        bool   `json:"dryRun"`
}

// expandVolume 扩展卷
// @Summary 扩展卷
// @Description 向指定卷添加设备进行扩容
// @Tags storage/volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body expandVolumeRequest true "扩展配置"
// @Success 200 {object} api.Response{data=btrfs.ExpansionStatus}
// @Router /volumes/{name}/expand [post]
func (h *RAIDExpansionHandlers) expandVolume(c *gin.Context) {
	volumeName := c.Param("name")

	var req expandVolumeRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 获取卷信息
	vol := h.manager.GetVolume(volumeName)
	if vol == nil {
		api.NotFound(c, "卷不存在: "+volumeName)
		return
	}

	if vol.MountPoint == "" {
		api.BadRequest(c, "卷未挂载")
		return
	}

	ctx := context.Background()
	config := btrfs.ExpansionConfig{
		VolumeName:    volumeName,
		MountPoint:    vol.MountPoint,
		NewDevice:     req.NewDevice,
		TargetProfile: req.TargetProfile,
		AutoBalance:   req.AutoBalance,
		Force:         req.Force,
		DryRun:        req.DryRun,
	}

	// 默认启用自动平衡
	if config.AutoBalance == false && config.TargetProfile == "" {
		config.AutoBalance = true
	}

	status, err := h.expansionManager.StartExpansion(ctx, config)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, status)
}

// getVolumeExpansionStatus 获取卷扩展状态
// @Summary 获取卷扩展状态
// @Description 获取指定卷的扩展状态
// @Tags storage/volumes
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response{data=btrfs.ExpansionStatus}
// @Router /volumes/{name}/expand/status [get]
func (h *RAIDExpansionHandlers) getVolumeExpansionStatus(c *gin.Context) {
	status := h.expansionManager.GetExpansionStatus()
	api.OK(c, status)
}

// estimateVolumeExpansionRequest 估算卷扩展请求
type estimateVolumeExpansionRequest struct {
	NewDeviceSize uint64 `json:"newDeviceSize"`
}

// estimateVolumeExpansion 估算卷扩展
// @Summary 估算卷扩展
// @Description 估算指定卷扩展后的容量增益
// @Tags storage/volumes
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body estimateVolumeExpansionRequest true "估算参数"
// @Success 200 {object} api.Response{data=btrfs.CapacityEstimate}
// @Router /volumes/{name}/expand/estimate [post]
func (h *RAIDExpansionHandlers) estimateVolumeExpansion(c *gin.Context) {
	volumeName := c.Param("name")

	var req estimateVolumeExpansionRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 获取卷信息
	vol := h.manager.GetVolume(volumeName)
	if vol == nil {
		api.NotFound(c, "卷不存在: "+volumeName)
		return
	}

	if vol.MountPoint == "" {
		api.BadRequest(c, "卷未挂载")
		return
	}

	ctx := context.Background()
	estimate, err := h.expansionManager.EstimateCapacityGain(ctx, vol.MountPoint, req.NewDeviceSize, vol.DataProfile)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, estimate)
}