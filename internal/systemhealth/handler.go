package systemhealth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API 处理器
type Handler struct {
	engine *HealthEngine
}

// NewHandler 创建处理器
func NewHandler(engine *HealthEngine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	health := rg.Group("/systemhealth")
	{
		health.GET("/status", h.GetStatus)
		health.GET("/report", h.GetReport)
		health.GET("/alerts", h.GetAlerts)
		health.POST("/check", h.TriggerCheck)
		health.GET("/system-info", h.GetSystemInfo)
	}
}

// GetStatus 获取健康状态概览
func (h *Handler) GetStatus(c *gin.Context) {
	report := h.engine.GetLastReport()
	if report == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "unknown",
			"message": "尚未执行健康检查",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    report.Overall,
		"score":     report.Score,
		"timestamp": report.Timestamp,
		"alerts":    len(report.Alerts),
	})
}

// GetReport 获取完整健康报告
func (h *Handler) GetReport(c *gin.Context) {
	report := h.engine.GetLastReport()
	if report == nil {
		// 执行一次检查
		var err error
		report, err = h.engine.Check(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, report)
}

// GetAlerts 获取告警列表
func (h *Handler) GetAlerts(c *gin.Context) {
	resolved := c.Query("resolved") == "true"
	alerts := h.engine.GetAlerts(resolved)
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// TriggerCheck 手动触发健康检查
func (h *Handler) TriggerCheck(c *gin.Context) {
	report, err := h.engine.Check(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "健康检查完成",
		"status":    report.Overall,
		"score":     report.Score,
		"timestamp": report.Timestamp,
	})
}

// GetSystemInfo 获取系统信息
func (h *Handler) GetSystemInfo(c *gin.Context) {
	report := h.engine.GetLastReport()
	if report == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "请先执行健康检查",
		})
		return
	}

	c.JSON(http.StatusOK, report.SystemInfo)
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string    `json:"status"`
	Score     int       `json:"score"`
	Timestamp time.Time `json:"timestamp"`
	Components []ComponentHealth `json:"components"`
}
