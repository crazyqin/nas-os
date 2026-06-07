// Package storagesimulator HTTP API 处理器
package storagesimulator

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/storage-simulator")
	{
		group.GET("/usage", h.GetUsage)
		group.POST("/usage", h.PostUsage)
		group.GET("/forecast", h.GetForecast)
		group.GET("/scenarios", h.GetScenarios)
		group.GET("/alerts", h.GetAlerts)
		group.POST("/config", h.PostConfig)
	}
}

// GetUsage 获取当前存储使用量
func (h *Handler) GetUsage(c *gin.Context) {
	history := h.manager.GetUsageHistory()

	var current *StorageUsage
	if len(history) > 0 {
		current = &history[len(history)-1]
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"current": current,
			"history": history,
			"count":   len(history),
		},
	})
}

// PostUsage 记录存储使用量快照
func (h *Handler) PostUsage(c *gin.Context) {
	var usage StorageUsage
	if err := c.ShouldBindJSON(&usage); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if usage.Timestamp.IsZero() {
		usage.Timestamp = time.Now()
	}

	h.manager.AddUsageRecord(usage)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    usage,
	})
}

// GetForecast 获取存储预测
func (h *Handler) GetForecast(c *gin.Context) {
	period := ForecastPeriod(c.DefaultQuery("period", "daily"))
	duration := 30
	if d, err := strconv.Atoi(c.Query("duration")); err == nil && d > 0 {
		duration = d
	}
	scenario := GrowthScenario(c.DefaultQuery("scenario", "medium"))

	result, err := h.manager.Forecast(period, duration, scenario)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// GetScenarios 获取不同增长场景对比
func (h *Handler) GetScenarios(c *gin.Context) {
	scenarioID := c.Query("id")

	// 如果指定了场景ID，返回单个场景模拟结果
	if scenarioID != "" {
		result, err := h.manager.SimulateScenario(scenarioID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
		return
	}

	// 否则返回所有场景列表
	scenarios := h.manager.ListScenarios()

	// 如果请求参数包含 simulate=true，同时返回各场景的模拟结果
	if c.Query("simulate") == "true" {
		results := make([]ScenarioResult, 0)
		for _, s := range scenarios {
			if result, err := h.manager.SimulateScenario(s.ID); err == nil {
				results = append(results, *result)
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"scenarios":   scenarios,
				"simulations": results,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": scenarios})
}

// GetAlerts 获取容量告警
func (h *Handler) GetAlerts(c *gin.Context) {
	alerts := h.manager.ListAlerts()

	// 筛选触发的告警
	triggeredOnly := c.Query("triggered") == "true"
	if triggeredOnly {
		triggered := make([]AlertStatus, 0)
		for _, a := range alerts {
			if a.Triggered {
				triggered = append(triggered, a)
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"alerts":   triggered,
				"count":    len(triggered),
				"filtered": true,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"alerts": alerts,
			"count":  len(alerts),
		},
	})
}

// ConfigRequest 配置请求
type ConfigRequest struct {
	// 告警配置
	Alert *CapacityAlert `json:"alert,omitempty"`
	// 成本配置
	CostPerGB *float64 `json:"cost_per_gb,omitempty"`
	Currency  string   `json:"currency,omitempty"`
	// 总容量配置
	TotalCapacity *int64 `json:"total_capacity,omitempty"`
}

// PostConfig 配置告警阈值和系统参数
func (h *Handler) PostConfig(c *gin.Context) {
	var req ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	results := make(map[string]interface{})

	// 配置告警
	if req.Alert != nil {
		if req.Alert.ID == "" {
			req.Alert.ID = "alert_" + time.Now().Format("20060102150405")
		}

		// 尝试更新，如果不存在则创建
		if err := h.manager.UpdateAlert(req.Alert); err != nil {
			if err := h.manager.AddAlert(req.Alert); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
				return
			}
			results["alert"] = gin.H{"action": "created", "data": req.Alert}
		} else {
			results["alert"] = gin.H{"action": "updated", "data": req.Alert}
		}
	}

	// 配置成本
	if req.CostPerGB != nil {
		currency := req.Currency
		if currency == "" {
			currency = "USD"
		}
		h.manager.SetCostConfig(*req.CostPerGB, currency)
		results["cost"] = gin.H{
			"cost_per_gb": *req.CostPerGB,
			"currency":    currency,
		}
	}

	// 配置总容量
	if req.TotalCapacity != nil {
		h.manager.SetTotalCapacity(*req.TotalCapacity)
		results["capacity"] = gin.H{
			"total_bytes": *req.TotalCapacity,
		}
	}

	if len(results) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "未提供有效配置项",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}
