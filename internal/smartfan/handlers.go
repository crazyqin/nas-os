// Package smartfan 提供智能风扇控制功能
// HTTP API handlers
package smartfan

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 智能风扇 HTTP 处理器
type Handler struct {
	controller *Controller
	logger     *zap.Logger
}

// NewHandler 创建 HTTP 处理器
func NewHandler(controller *Controller, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		controller: controller,
		logger:     logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	fan := rg.Group("/smartfan")
	{
		fan.GET("/fans", h.ListFans)
		fan.GET("/zones", h.ListZones)
		fan.GET("/profiles", h.ListProfiles)
		fan.POST("/profiles", h.CreateProfile)
		fan.PUT("/active", h.UpdateActive)
		fan.GET("/stats", h.GetStats)
		fan.GET("/alerts", h.GetAlerts)
	}
}

// ListFans handles GET /api/v1/smartfan/fans
// 列出所有风扇设备
func (h *Handler) ListFans(c *gin.Context) {
	fans := h.controller.GetFans()
	c.JSON(http.StatusOK, gin.H{
		"fans":  fans,
		"total": len(fans),
	})
}

// ListZones handles GET /api/v1/smartfan/zones
// 列出所有温度区域
func (h *Handler) ListZones(c *gin.Context) {
	zones := h.controller.GetZones()
	c.JSON(http.StatusOK, gin.H{
		"zones": zones,
		"total": len(zones),
	})
}

// ListProfiles handles GET /api/v1/smartfan/profiles
// 列出所有风扇配置
func (h *Handler) ListProfiles(c *gin.Context) {
	profiles := h.controller.GetProfiles()
	c.JSON(http.StatusOK, gin.H{
		"profiles": profiles,
		"total":    len(profiles),
	})
}

// CreateProfileRequest 创建配置请求
type CreateProfileRequest struct {
	Name  string       `json:"name" binding:"required"`
	Mode  FanMode      `json:"mode" binding:"required"`
	Curve []CurvePoint `json:"curve" binding:"required,min=2"`
}

// CreateProfile handles POST /api/v1/smartfan/profiles
// 创建自定义风扇配置
func (h *Handler) CreateProfile(c *gin.Context) {
	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证模式
	validModes := map[FanMode]bool{
		FanModeSilent:      true,
		FanModeBalanced:    true,
		FanModePerformance: true,
		FanModeCustom:      true,
	}
	if !validModes[req.Mode] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的风扇模式"})
		return
	}

	profile, err := h.controller.CreateProfile(req.Name, req.Mode, req.Curve)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[智能风扇 API] 创建配置",
		zap.String("id", profile.ID),
		zap.String("name", profile.Name))

	c.JSON(http.StatusCreated, profile)
}

// UpdateActiveRequest 切换活跃配置请求
type UpdateActiveRequest struct {
	ProfileID string `json:"profileId" binding:"required"`
}

// UpdateActive handles PUT /api/v1/smartfan/active
// 切换活跃配置
func (h *Handler) UpdateActive(c *gin.Context) {
	var req UpdateActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.controller.SetActiveProfile(req.ProfileID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("[智能风扇 API] 切换配置", zap.String("profileId", req.ProfileID))

	c.JSON(http.StatusOK, gin.H{
		"message":    "配置已切换",
		"profileId": req.ProfileID,
	})
}

// GetStats handles GET /api/v1/smartfan/stats
// 获取温度/转速统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.controller.GetStats()
	c.JSON(http.StatusOK, stats)
}

// GetAlerts handles GET /api/v1/smartfan/alerts
// 获取告警列表
func (h *Handler) GetAlerts(c *gin.Context) {
	// 解析 limit 参数
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	alerts := h.controller.GetAlerts(limit)
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}
