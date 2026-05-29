// Package healthdashboard provides storage health monitoring and alerting.
package healthdashboard

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers provides HTTP handlers for the health dashboard.
type Handlers struct {
	collector *Collector
	alerts    []*AlertEvent
	rules     map[string]*HealthAlertRule
	mu        sync.RWMutex
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(collector *Collector) *Handlers {
	return &Handlers{
		collector: collector,
		alerts:    make([]*AlertEvent, 0),
		rules:     make(map[string]*HealthAlertRule),
	}
}

// RegisterRoutes registers all health dashboard routes.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1/healthdashboard")
	{
		api.GET("/overview", h.getOverview)
		api.GET("/scores", h.getScores)
		api.GET("/metrics", h.getMetrics)
		api.GET("/trends", h.getTrends)
		api.GET("/alerts", h.getAlerts)
		api.POST("/alerts", h.createAlertRule)
	}
}

// getOverview returns the dashboard overview.
func (h *Handlers) getOverview(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	score := h.collector.GetHealthScore()
	metrics := h.collector.GetRealTimeMetrics()

	// Get active alerts
	activeAlerts := make([]*AlertEvent, 0)
	for _, alert := range h.alerts {
		if time.Since(alert.TriggeredAt) < 24*time.Hour {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data: OverviewResponse{
			Score:      score,
			Metrics:    metrics,
			Alerts:     activeAlerts,
			AlertCount: len(activeAlerts),
			UpdatedAt:  time.Now(),
		},
	})
}

// getScores returns the health scores.
func (h *Handlers) getScores(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	score := h.collector.GetHealthScore()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    score,
	})
}

// getMetrics returns real-time health metrics.
func (h *Handlers) getMetrics(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	metrics := h.collector.GetRealTimeMetrics()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    metrics,
	})
}

// getTrends returns historical trend data.
func (h *Handlers) getTrends(c *gin.Context) {
	period := c.DefaultQuery("period", "7d")

	h.mu.RLock()
	defer h.mu.RUnlock()

	trends := h.collector.GetTrends(period)

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    trends,
	})
}

// getAlerts returns alert rules and recent alerts.
func (h *Handlers) getAlerts(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rules := make([]*HealthAlertRule, 0)
	for _, rule := range h.rules {
		rules = append(rules, rule)
	}

	recentAlerts := make([]*AlertEvent, 0)
	for _, alert := range h.alerts {
		if time.Since(alert.TriggeredAt) < 7*24*time.Hour {
			recentAlerts = append(recentAlerts, alert)
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"rules":  rules,
			"alerts": recentAlerts,
		},
	})
}

// createAlertRule creates a new alert rule.
func (h *Handlers) createAlertRule(c *gin.Context) {
	var rule HealthAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    1,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate operator
	validOps := map[string]bool{"gt": true, "lt": true, "eq": true}
	if !validOps[rule.Operator] {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    2,
			Message: "Invalid operator. Must be gt, lt, or eq",
		})
		return
	}

	// Validate severity
	validSeverities := map[string]bool{"info": true, "warning": true, "critical": true}
	if !validSeverities[rule.Severity] {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    3,
			Message: "Invalid severity. Must be info, warning, or critical",
		})
		return
	}

	// Generate ID if empty
	if rule.ID == "" {
		rule.ID = generateRuleID(rule.Metric, rule.Operator)
	}

	h.mu.Lock()
	h.rules[rule.ID] = &rule
	h.mu.Unlock()

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "Alert rule created",
		Data:    rule,
	})
}

// CheckAlerts evaluates all rules against current metrics.
func (h *Handlers) CheckAlerts() {
	metrics := h.collector.GetRealTimeMetrics()

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, rule := range h.rules {
		if !rule.Enabled {
			continue
		}

		for _, metric := range metrics {
			if metric.Name != rule.Metric {
				continue
			}

			triggered := false
			switch rule.Operator {
			case "gt":
				triggered = metric.Value > rule.Threshold
			case "lt":
				triggered = metric.Value < rule.Threshold
			case "eq":
				triggered = metric.Value == rule.Threshold
			}

			if triggered {
				alert := &AlertEvent{
					RuleID:      rule.ID,
					Metric:      metric.Name,
					Value:       metric.Value,
					Threshold:   rule.Threshold,
					Severity:    rule.Severity,
					Message:     formatAlertMessage(rule, metric),
					TriggeredAt: time.Now(),
				}
				h.alerts = append(h.alerts, alert)
			}
		}
	}

	// Keep only last 1000 alerts
	if len(h.alerts) > 1000 {
		h.alerts = h.alerts[len(h.alerts)-1000:]
	}
}

func generateRuleID(metric, operator string) string {
	return metric + "_" + operator + "_" + time.Now().Format("20060102150405")
}

func formatAlertMessage(rule *HealthAlertRule, metric *HealthMetric) string {
	return metric.Name + " is " + metric.Status + ": " +
		formatFloat(metric.Value) + " " + metric.Unit +
		" (threshold: " + formatFloat(rule.Threshold) + ")"
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
