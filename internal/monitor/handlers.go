package monitor

import (
	"net/http"
	"sync"
	"time"

	"nas-os/internal/notify"

	"github.com/gin-gonic/gin"
)

// Handlers 监控处理器
type Handlers struct {
	manager    *Manager
	alerts     []*Alert
	alertRules []*AlertRule
	notifyMgr  *notify.Manager
	mu         sync.RWMutex
}

// NewHandlers 创建监控处理器
func NewHandlers(mgr *Manager, notifyMgr *notify.Manager) *Handlers {
	h := &Handlers{
		manager:   mgr,
		alerts:    make([]*Alert, 0),
		notifyMgr: notifyMgr,
		alertRules: []*AlertRule{
			{Name: "cpu-warning", Type: "cpu", Threshold: 80, Level: "warning", Enabled: true},
			{Name: "cpu-critical", Type: "cpu", Threshold: 95, Level: "critical", Enabled: true},
			{Name: "memory-warning", Type: "memory", Threshold: 85, Level: "warning", Enabled: true},
			{Name: "memory-critical", Type: "memory", Threshold: 95, Level: "critical", Enabled: true},
			{Name: "disk-warning", Type: "disk", Threshold: 85, Level: "warning", Enabled: true},
			{Name: "disk-critical", Type: "disk", Threshold: 95, Level: "critical", Enabled: true},
		},
	}
	go h.startAlertChecker()
	return h
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	monitor := r.Group("/monitor")
	{
		monitor.GET("/system", h.getSystemStats)
		monitor.GET("/disks", h.getDiskStats)
		monitor.GET("/network", h.getNetworkStats)
		monitor.GET("/smart/:device", h.getSMARTInfo)
		monitor.POST("/smart/check", h.checkAllDisks)

		// 告警管理
		monitor.GET("/alerts", h.getAlerts)
		monitor.POST("/alerts/:id/acknowledge", h.acknowledgeAlert)
		monitor.DELETE("/alerts/:id", h.deleteAlert)
		monitor.GET("/alerts/rules", h.getAlertRules)
		monitor.PUT("/alerts/rules/:name", h.updateAlertRule)
	}
}

// getSystemStats 获取系统统计信息
func (h *Handlers) getSystemStats(c *gin.Context) {
	stats, err := h.manager.GetSystemStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getDiskStats 获取磁盘统计信息
func (h *Handlers) getDiskStats(c *gin.Context) {
	stats, err := h.manager.GetDiskStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getNetworkStats 获取网络统计信息
func (h *Handlers) getNetworkStats(c *gin.Context) {
	stats, err := h.manager.GetNetworkStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getSMARTInfo 获取 SMART 信息
func (h *Handlers) getSMARTInfo(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备名称不能为空",
		})
		return
	}

	info, err := h.manager.GetSMARTInfo(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    info,
	})
}

// checkAllDisks 检查所有磁盘
func (h *Handlers) checkAllDisks(c *gin.Context) {
	results, err := h.manager.CheckDisks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    results,
	})
}

// getAlerts 获取告警列表
func (h *Handlers) getAlerts(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 过滤参数
	level := c.Query("level")
	alertType := c.Query("type")

	alerts := make([]*Alert, 0)
	for _, alert := range h.alerts {
		if level != "" && alert.Level != level {
			continue
		}
		if alertType != "" && alert.Type != alertType {
			continue
		}
		alerts = append(alerts, alert)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alerts,
	})
}

// acknowledgeAlert 确认告警
func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	id := c.Param("id")

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, alert := range h.alerts {
		if alert.ID == id {
			alert.Acknowledged = true
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "告警已确认",
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "告警不存在",
	})
}

// deleteAlert 删除告警
func (h *Handlers) deleteAlert(c *gin.Context) {
	id := c.Param("id")

	h.mu.Lock()
	defer h.mu.Unlock()

	for i, alert := range h.alerts {
		if alert.ID == id {
			h.alerts = append(h.alerts[:i], h.alerts[i+1:]...)
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "告警已删除",
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "告警不存在",
	})
}

// getAlertRules 获取告警规则
func (h *Handlers) getAlertRules(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.alertRules,
	})
}

// updateAlertRule 更新告警规则
func (h *Handlers) updateAlertRule(c *gin.Context) {
	name := c.Param("name")

	var req struct {
		Threshold float64 `json:"threshold"`
		Enabled   bool    `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, rule := range h.alertRules {
		if rule.Name == name {
			rule.Threshold = req.Threshold
			rule.Enabled = req.Enabled
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "规则已更新",
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "规则不存在",
	})
}

// startAlertChecker 启动告警检查器
func (h *Handlers) startAlertChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.checkAlerts()
	}
}

// checkAlerts 检查告警
func (h *Handlers) checkAlerts() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 检查 CPU
	if stats, err := h.manager.GetSystemStats(); err == nil {
		for _, rule := range h.alertRules {
			if rule.Type == "cpu" && rule.Enabled && stats.CPUUsage >= rule.Threshold {
				h.addAlert(&Alert{
					ID:        generateAlertID(),
					Type:      "cpu",
					Level:     rule.Level,
					Message:   "CPU 使用率过高",
					Source:    "system",
					Timestamp: time.Now(),
				})
			}

			if rule.Type == "memory" && rule.Enabled && stats.MemoryUsage >= rule.Threshold {
				h.addAlert(&Alert{
					ID:        generateAlertID(),
					Type:      "memory",
					Level:     rule.Level,
					Message:   "内存使用率过高",
					Source:    "system",
					Timestamp: time.Now(),
				})
			}
		}
	}

	// 检查磁盘
	if disks, err := h.manager.GetDiskStats(); err == nil {
		for _, disk := range disks {
			for _, rule := range h.alertRules {
				if rule.Type == "disk" && rule.Enabled && disk.UsagePercent >= rule.Threshold {
					h.addAlert(&Alert{
						ID:        generateAlertID(),
						Type:      "disk",
						Level:     rule.Level,
						Message:   "磁盘空间不足",
						Source:    disk.Device,
						Timestamp: time.Now(),
					})
				}
			}
		}
	}
}

// addAlert 添加告警
func (h *Handlers) addAlert(alert *Alert) {
	// 检查是否已存在相同的未确认告警
	for _, a := range h.alerts {
		if a.ID == alert.ID && !a.Acknowledged {
			return
		}
	}

	h.alerts = append(h.alerts, alert)

	// 保留最近 100 条告警
	if len(h.alerts) > 100 {
		h.alerts = h.alerts[len(h.alerts)-100:]
	}

	// 发送通知
	if h.notifyMgr != nil {
		var level notify.AlertLevel
		switch alert.Level {
		case "warning":
			level = notify.LevelWarning
		case "critical":
			level = notify.LevelCritical
		default:
			level = notify.LevelInfo
		}

		notif := &notify.Notification{
			Title:   alert.Message,
			Message: h.getAlertDescription(alert),
			Level:   level,
			Source:  "NAS-OS 监控",
		}
		_ = h.notifyMgr.Send(notif)
	}
}

// getAlertDescription 获取告警详细描述
func (h *Handlers) getAlertDescription(alert *Alert) string {
	desc := alert.Message
	if alert.Source != "" {
		desc += " - 来源：" + alert.Source
	}
	desc += "\n级别：" + alert.Level
	return desc
}
