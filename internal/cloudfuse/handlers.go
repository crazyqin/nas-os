// Package cloudfuse provides cloud storage mounting via FUSE
// HTTP API handlers
package cloudfuse

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建 HTTP 处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	mounts := r.Group("/mounts")
	{
		mounts.GET("", h.ListMounts)
		mounts.POST("", h.CreateMount)
		mounts.GET("/providers", h.ListProviders)
		mounts.GET("/:id", h.GetMount)
		mounts.PUT("/:id", h.UpdateMount)
		mounts.DELETE("/:id", h.DeleteMount)
		mounts.POST("/:id/mount", h.Mount)
		mounts.POST("/:id/unmount", h.Unmount)
		mounts.GET("/:id/stats", h.GetStats)
		mounts.POST("/test", h.TestMount)
	}
}

// ListProviders 列出支持的提供商
// @Summary 列出支持的网盘提供商
// @Description 返回所有支持的网盘类型及其功能特性
// @Tags cloudfuse
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cloudfuse/mounts/providers [get]
func (h *Handler) ListProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"providers": SupportedProviders(),
	})
}

// ListMounts 列出所有挂载
// @Summary 列出所有挂载点
// @Description 返回所有已配置的挂载点信息
// @Tags cloudfuse
// @Produce json
// @Success 200 {object} MountListResponse
// @Router /api/v1/cloudfuse/mounts [get]
func (h *Handler) ListMounts(c *gin.Context) {
	mounts := h.manager.ListMounts()

	c.JSON(http.StatusOK, MountListResponse{
		Total: int64(len(mounts)),
		Items: mounts,
	})
}

// CreateMount 创建挂载配置
// @Summary 创建挂载配置
// @Description 创建新的网盘挂载配置（不立即挂载）
// @Tags cloudfuse
// @Accept json
// @Produce json
// @Param request body MountRequest true "挂载请求"
// @Success 200 {object} MountInfo
// @Failure 400 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts [post]
func (h *Handler) CreateMount(c *gin.Context) {
	var req MountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := &MountConfig{
		ID:           req.Name + "-" + time.Now().Format("20060102150405"),
		Name:         req.Name,
		Type:         req.Type,
		MountPoint:   req.MountPoint,
		RemotePath:   req.RemotePath,
		Enabled:      true,
		AutoMount:    req.AutoMount,
		ReadOnly:     req.ReadOnly,
		AllowOther:   req.AllowOther,
		CacheEnabled: req.CacheEnabled,
		CacheDir:     req.CacheDir,
		CacheSize:    req.CacheSize,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),

		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		UserID:       req.UserID,
		DriveID:      req.DriveID,

		Endpoint:   req.Endpoint,
		Bucket:     req.Bucket,
		AccessKey:  req.AccessKey,
		SecretKey:  req.SecretKey,
		Region:     req.Region,
		PathStyle:  req.PathStyle,
		Insecure:   req.Insecure,
		ClientID:   req.ClientID,
		TenantID:   req.TenantID,
		RootFolder: req.RootFolder,
	}

	// 保存配置
	h.manager.AddMountConfig(cfg)

	c.JSON(http.StatusOK, MountInfo{
		ID:         cfg.ID,
		Name:       cfg.Name,
		Type:       cfg.Type,
		MountPoint: cfg.MountPoint,
		Status:     MountStatusIdle,
		CreatedAt:  cfg.CreatedAt,
	})
}

// GetMount 获取挂载详情
// @Summary 获取挂载详情
// @Description 获取指定挂载点的详细信息
// @Tags cloudfuse
// @Produce json
// @Param id path string true "挂载ID"
// @Success 200 {object} MountInfo
// @Failure 404 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts/{id} [get]
func (h *Handler) GetMount(c *gin.Context) {
	id := c.Param("id")

	info, err := h.manager.GetMount(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// UpdateMount 更新挂载配置
// @Summary 更新挂载配置
// @Description 更新指定挂载点的配置
// @Tags cloudfuse
// @Accept json
// @Produce json
// @Param id path string true "挂载ID"
// @Param request body MountRequest true "挂载配置"
// @Success 200 {object} MountInfo
// @Failure 400 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts/{id} [put]
func (h *Handler) UpdateMount(c *gin.Context) {
	id := c.Param("id")

	var req MountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := &MountConfig{
		ID:           id,
		Name:         req.Name,
		Type:         req.Type,
		MountPoint:   req.MountPoint,
		RemotePath:   req.RemotePath,
		AutoMount:    req.AutoMount,
		ReadOnly:     req.ReadOnly,
		AllowOther:   req.AllowOther,
		CacheEnabled: req.CacheEnabled,
		CacheDir:     req.CacheDir,
		CacheSize:    req.CacheSize,
		UpdatedAt:    time.Now(),

		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		UserID:       req.UserID,
		DriveID:      req.DriveID,

		Endpoint:   req.Endpoint,
		Bucket:     req.Bucket,
		AccessKey:  req.AccessKey,
		SecretKey:  req.SecretKey,
		Region:     req.Region,
		PathStyle:  req.PathStyle,
		Insecure:   req.Insecure,
		ClientID:   req.ClientID,
		TenantID:   req.TenantID,
		RootFolder: req.RootFolder,
	}

	if err := h.manager.UpdateMountConfig(id, cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	info, _ := h.manager.GetMount(id)
	c.JSON(http.StatusOK, info)
}

// DeleteMount 删除挂载配置
// @Summary 删除挂载配置
// @Description 删除指定挂载点的配置（如果已挂载会先卸载）
// @Tags cloudfuse
// @Param id path string true "挂载ID"
// @Success 200 {object} OperationResult
// @Failure 400 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts/{id} [delete]
func (h *Handler) DeleteMount(c *gin.Context) {
	id := c.Param("id")

	// 先卸载
	_ = h.manager.Unmount(id)

	// 删除配置
	if err := h.manager.RemoveMountConfig(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, OperationResult{
		Success: true,
		Message: "挂载配置已删除",
	})
}

// Mount 执行挂载
// @Summary 执行挂载
// @Description 挂载指定的网盘到本地目录
// @Tags cloudfuse
// @Param id path string true "挂载ID"
// @Success 200 {object} MountInfo
// @Failure 400 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts/{id}/mount [post]
func (h *Handler) Mount(c *gin.Context) {
	id := c.Param("id")

	// 获取配置
	info, err := h.manager.GetMount(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "挂载配置不存在"})
		return
	}

	// 从配置中获取完整配置
	var cfg *MountConfig
	for _, m := range h.manager.config.Mounts {
		if m.ID == id {
			cfg = &m
			break
		}
	}

	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "挂载配置不存在"})
		return
	}

	// 执行挂载
	mountInfo, err := h.manager.Mount(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"info":  info,
		})
		return
	}

	c.JSON(http.StatusOK, mountInfo)
}

// Unmount 卸载挂载
// @Summary 卸载挂载
// @Description 卸载指定的挂载点
// @Tags cloudfuse
// @Param id path string true "挂载ID"
// @Success 200 {object} OperationResult
// @Failure 400 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts/{id}/unmount [post]
func (h *Handler) Unmount(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.Unmount(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, OperationResult{
		Success: true,
		Message: "已卸载",
	})
}

// GetStats 获取挂载统计
// @Summary 获取挂载统计
// @Description 获取指定挂载点的运行统计信息
// @Tags cloudfuse
// @Produce json
// @Param id path string true "挂载ID"
// @Success 200 {object} MountStats
// @Failure 404 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts/{id}/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.manager.GetStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// TestMount 测试挂载配置
// @Summary 测试挂载配置
// @Description 测试挂载配置是否有效（不执行实际挂载）
// @Tags cloudfuse
// @Accept json
// @Produce json
// @Param request body MountRequest true "挂载配置"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/v1/cloudfuse/mounts/test [post]
func (h *Handler) TestMount(c *gin.Context) {
	var req MountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := &MountConfig{
		Type:         req.Type,
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		UserID:       req.UserID,
		DriveID:      req.DriveID,

		Endpoint:   req.Endpoint,
		Bucket:     req.Bucket,
		AccessKey:  req.AccessKey,
		SecretKey:  req.SecretKey,
		Region:     req.Region,
		PathStyle:  req.PathStyle,
		Insecure:   req.Insecure,
		ClientID:   req.ClientID,
		TenantID:   req.TenantID,
		RootFolder: req.RootFolder,
	}

	result, err := h.manager.TestMountConfig(cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   result.Success,
		"provider":  result.Provider,
		"endpoint":  result.Endpoint,
		"latencyMs": result.LatencyMs,
		"message":   result.Message,
		"error":     result.Error,
	})
}
