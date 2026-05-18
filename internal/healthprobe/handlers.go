// Package healthprobe - HTTP API 处理器
package healthprobe

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 健康探针 HTTP 处理器
type Handlers struct {
	manager *ProbeManager
	version string
}

// NewHandlers 创建健康探针处理器
func NewHandlers(manager *ProbeManager, version string) *Handlers {
	return &Handlers{
		manager: manager,
		version: version,
	}
}

// RegisterRoutes 注册路由到 API 组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1")
	{
		api.GET("/health", h.getHealth)
		api.GET("/health/detailed", h.getDetailedHealth)
		api.GET("/health/probes", h.listProbes)
		api.GET("/health/probes/:name", h.getProbeStatus)
		api.GET("/health/history", h.getHistory)
		api.GET("/health/alerts", h.getAlerts)
		api.POST("/health/alerts/:id/resolve", h.resolveAlert)
		api.GET("/health/rules", h.getRules)
		api.POST("/health/rules", h.addRule)
		api.DELETE("/health/rules/:name", h.removeRule)
		api.POST("/health/check", h.triggerCheck)
	}
}

// RegisterSimpleRoutes 注册简单路由（用于直接挂载到 Engine）
func (h *Handlers) RegisterSimpleRoutes(r *gin.Engine) {
	r.GET("/health", h.getHealth)
	r.GET("/health/detailed", h.getDetailedHealth)
	r.GET("/healthz", h.healthz)
	r.GET("/readyz", h.readyz)
}

// APIResponse API 响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HealthResponse 健康端点简化响应
type HealthResponse struct {
	Status    string  `json:"status"`
	Score     float64 `json:"score"`
	Timestamp string  `json:"timestamp"`
	Version   string  `json:"version,omitempty"`
	Uptime    string  `json:"uptime"`
}

// getHealth /health - 简化健康状态
func (h *Handlers) getHealth(c *gin.Context) {
	status := h.manager.GetStatus()
	if status == nil {
		// 尚未执行检查，触发一次
		status = h.manager.Check(c.Request.Context())
	}

	httpStatus := http.StatusOK
	switch status.Level {
	case LevelCritical:
		httpStatus = http.StatusServiceUnavailable
	case LevelDegraded:
		httpStatus = http.StatusOK // 降级仍返回 200
	}

	resp := HealthResponse{
		Status:    string(status.Level),
		Score:     status.Score,
		Timestamp: status.Timestamp.Format(time.RFC3339),
		Version:   h.version,
		Uptime:    status.Uptime.String(),
	}

	c.JSON(httpStatus, APIResponse{
		Code:    0,
		Message: string(status.Level),
		Data:    resp,
	})
}

// getDetailedHealth /health/detailed - 详细健康状态
func (h *Handlers) getDetailedHealth(c *gin.Context) {
	status := h.manager.GetStatus()
	if status == nil {
		status = h.manager.Check(c.Request.Context())
	}

	httpStatus := http.StatusOK
	switch status.Level {
	case LevelCritical:
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, APIResponse{
		Code:    0,
		Message: string(status.Level),
		Data:    status,
	})
}

// healthz /healthz - Kubernetes 存活探针
func (h *Handlers) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// readyz /readyz - Kubernetes 就绪探针
func (h *Handlers) readyz(c *gin.Context) {
	status := h.manager.GetStatus()
	if status == nil {
		status = h.manager.Check(c.Request.Context())
	}

	if status.Level == LevelCritical {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"level":  string(status.Level),
			"score":  status.Score,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"level":  string(status.Level),
		"score":  status.Score,
	})
}

// listProbes /health/probes - 列出所有探针
func (h *Handlers) listProbes(c *gin.Context) {
	probes := h.manager.GetProbes()
	status := h.manager.GetStatus()

	probeList := make([]gin.H, 0, len(probes))
	for _, name := range probes {
		item := gin.H{"name": name}
		if status != nil {
			if result, ok := status.Probes[name]; ok {
				item["level"] = result.Level
				item["value"] = result.Value
				item["unit"] = result.Unit
				item["message"] = result.Message
				item["lastCheck"] = result.Timestamp.Format(time.RFC3339)
			}
		}
		probeList = append(probeList, item)
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(probes),
			"probes": probeList,
		},
	})
}

// getProbeStatus /health/probes/:name - 获取单个探针状态
func (h *Handlers) getProbeStatus(c *gin.Context) {
	name := c.Param("name")
	status := h.manager.GetStatus()

	if status == nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    1,
			Message: "健康状态尚未初始化",
		})
		return
	}

	result, exists := status.Probes[name]
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    1,
			Message: "探针 " + name + " 未找到",
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// getHistory /health/history - 获取历史记录
func (h *Handlers) getHistory(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	history := h.manager.GetHistory(limit)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// getAlerts /health/alerts - 获取告警列表
func (h *Handlers) getAlerts(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	includeResolved := c.Query("resolved") == "true"
	alerts := h.manager.GetAlerts(limit, includeResolved)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

// resolveAlert /health/alerts/:id/resolve - 解决告警
func (h *Handlers) resolveAlert(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "告警已解决",
	})
}

// getRules /health/rules - 获取所有规则
func (h *Handlers) getRules(c *gin.Context) {
	rules := h.manager.GetRules()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

// addRule /health/rules - 添加规则
func (h *Handlers) addRule(c *gin.Context) {
	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    1,
			Message: "无效的规则参数: " + err.Error(),
		})
		return
	}

	if rule.Name == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    1,
			Message: "规则名称不能为空",
		})
		return
	}

	rule.Enabled = true
	h.manager.AddRule(&rule)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "规则已添加",
		Data:    rule,
	})
}

// removeRule /health/rules/:name - 移除规则
func (h *Handlers) removeRule(c *gin.Context) {
	name := c.Param("name")
	h.manager.RemoveRule(name)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "规则已移除",
	})
}

// triggerCheck /health/check - 手动触发检查
func (h *Handlers) triggerCheck(c *gin.Context) {
	status := h.manager.Check(c.Request.Context())

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "检查完成",
		Data:    status,
	})
}
