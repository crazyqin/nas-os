// handlers.go - 智能温控管理 HTTP 接口
package thermalmanager

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 温控管理 HTTP 处理器
type Handler struct {
	logger  *zap.Logger
	manager *Manager
}

// NewHandler 创建温控管理处理器
func NewHandler(logger *zap.Logger, manager *Manager) *Handler {
	return &Handler{
		logger:  logger,
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	thermal := rg.Group("/thermal-manager")
	{
		thermal.GET("/overview", h.getOverview)
		thermal.GET("/zones", h.getZones)
		thermal.GET("/fans", h.getFans)
		thermal.GET("/profile", h.getProfile)
		thermal.PUT("/profile", h.updateProfile)
		thermal.GET("/alerts", h.getAlerts)
		thermal.DELETE("/alerts", h.clearAlerts)
		thermal.GET("/history", h.getHistory)
		thermal.GET("/stats", h.getStats)
		thermal.POST("/mode", h.setCoolingMode)
		thermal.PUT("/fan/:id/mode", h.setFanMode)
		thermal.PUT("/fan/:id/pwm", h.setFanPWM)
	}
}

// getOverview 获取温度总览
func (h *Handler) getOverview(c *gin.Context) {
	overview := h.manager.GetOverview()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": overview,
	})
}

// getZones 获取温度区域列表
func (h *Handler) getZones(c *gin.Context) {
	zones := h.manager.GetZones()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": zones,
	})
}

// getFans 获取风扇信息
func (h *Handler) getFans(c *gin.Context) {
	fans := h.manager.GetFans()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": fans,
	})
}

// getProfile 获取当前散热配置
func (h *Handler) getProfile(c *gin.Context) {
	profile := h.manager.GetProfile()
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": profile,
	})
}

// updateProfileRequest 更新配置请求
type updateProfileRequest struct {
	Name       string         `json:"name"`
	Mode       CoolingMode    `json:"mode"`
	WarmThresh float64        `json:"warmThresh"`
	HotThresh  float64        `json:"hotThresh"`
	CritThresh float64        `json:"critThresh"`
	Curves     []ThermalCurve `json:"curves"`
}

// updateProfile 更新散热配置
func (h *Handler) updateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	profile := CoolingProfile{
		Name:       req.Name,
		Mode:       req.Mode,
		WarmThresh: req.WarmThresh,
		HotThresh:  req.HotThresh,
		CritThresh: req.CritThresh,
		Curves:     req.Curves,
	}

	if profile.Name == "" {
		profile.Name = "custom"
	}
	if profile.Mode == "" {
		profile.Mode = CoolingBalanced
	}

	h.manager.SetProfile(profile)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": profile,
	})
}

// getAlerts 获取告警列表
func (h *Handler) getAlerts(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	alerts := h.manager.GetAlerts(limit)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": alerts,
	})
}

// clearAlerts 清空告警
func (h *Handler) clearAlerts(c *gin.Context) {
	h.manager.ClearAlerts()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "告警已清空",
	})
}

// getHistory 获取历史数据
func (h *Handler) getHistory(c *gin.Context) {
	minutes := 60
	if m, err := strconv.Atoi(c.Query("minutes")); err == nil && m > 0 {
		minutes = m
	}

	history := h.manager.GetHistory(minutes)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": history,
	})
}

// getStats 获取温度统计
func (h *Handler) getStats(c *gin.Context) {
	zone := c.Query("zone")
	if zone == "" {
		zone = "CPU"
	}

	minutes := 60
	if m, err := strconv.Atoi(c.Query("minutes")); err == nil && m > 0 {
		minutes = m
	}

	stats := h.manager.GetStats(zone, minutes)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": stats,
	})
}

// setCoolingModeRequest 设置散热模式请求
type setCoolingModeRequest struct {
	Mode CoolingMode `json:"mode" binding:"required"`
}

// setCoolingMode 设置散热模式
func (h *Handler) setCoolingMode(c *gin.Context) {
	var req setCoolingModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	switch req.Mode {
	case CoolingSilent, CoolingBalanced, CoolingPerformance:
		h.manager.SetCoolingMode(req.Mode)
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "散热模式已更新",
			"data":    h.manager.GetProfile(),
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的散热模式，可选: silent, balanced, performance",
		})
	}
}

// setFanModeRequest 设置风扇模式请求
type setFanModeRequest struct {
	Mode FanMode `json:"mode" binding:"required"`
}

// setFanMode 设置风扇模式
func (h *Handler) setFanMode(c *gin.Context) {
	fanID := c.Param("id")
	if fanID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "风扇 ID 不能为空",
		})
		return
	}

	var req setFanModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	switch req.Mode {
	case FanPWM, FanManual, FanAuto:
		if err := h.manager.SetFanMode(fanID, req.Mode); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "风扇模式已更新",
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的风扇模式，可选: pwm, manual, auto",
		})
	}
}

// setFanPWMRequest 设置风扇 PWM 请求
type setFanPWMRequest struct {
	PWM int `json:"pwm" binding:"required,min=0,max=255"`
}

// setFanPWM 设置风扇 PWM
func (h *Handler) setFanPWM(c *gin.Context) {
	fanID := c.Param("id")
	if fanID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "风扇 ID 不能为空",
		})
		return
	}

	var req setFanPWMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	if err := h.manager.SetFanPWM(fanID, req.PWM); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "风扇 PWM 已更新",
	})
}
