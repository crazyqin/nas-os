// Package fileintegrity 提供 REST API 处理器
package fileintegrity

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 文件完整性监控 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	fim := r.Group("/file-integrity")
	{
		// 状态和配置
		fim.GET("/status", h.getStatus)
		fim.GET("/config", h.getConfig)
		fim.PUT("/config", h.updateConfig)

		// 基线管理
		fim.POST("/baselines", h.createBaseline)
		fim.GET("/baselines", h.listBaselines)
		fim.GET("/baselines/:id", h.getBaseline)
		fim.DELETE("/baselines/:id", h.deleteBaseline)
		fim.POST("/baselines/:id/report", h.generateReport)

		// 规则管理
		fim.POST("/rules", h.addRule)
		fim.GET("/rules", h.listRules)
		fim.GET("/rules/:id", h.getRule)
		fim.PUT("/rules/:id", h.updateRule)
		fim.DELETE("/rules/:id", h.deleteRule)

		// 扫描
		fim.POST("/scan", h.runScan)
		fim.GET("/scans", h.getScanResults)

		// 变更管理
		fim.GET("/changes", h.listChanges)
		fim.POST("/changes/:id/ack", h.acknowledgeChange)
		fim.GET("/changes/:id/suggestions", h.getRepairSuggestions)

		// 告警
		fim.GET("/alerts", h.getAlerts)

		// 审计日志
		fim.GET("/audit-log/export", h.exportAuditLog)

		// 控制
		fim.POST("/start", h.start)
		fim.POST("/stop", h.stop)
	}
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *Handlers) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"running": h.manager.IsRunning(),
			"status":  h.manager.GetStatus(),
		},
	})
}

func (h *Handlers) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: h.manager.GetConfig()})
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg MonitorConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{Code: 0, Message: "config updated"})
}

func (h *Handlers) createBaseline(c *gin.Context) {
	var req struct {
		Name        string        `json:"name" binding:"required"`
		Description string        `json:"description"`
		Paths       []string      `json:"paths" binding:"required"`
		Algorithm   HashAlgorithm `json:"algorithm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	baseline, err := h.manager.CreateBaseline(c.Request.Context(), req.Name, req.Description, req.Paths, req.Algorithm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response{Code: 0, Message: "baseline created", Data: baseline})
}

func (h *Handlers) listBaselines(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: h.manager.ListBaselines(page, pageSize)})
}

func (h *Handlers) getBaseline(c *gin.Context) {
	id := c.Param("id")
	baseline, err := h.manager.GetBaseline(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: baseline})
}

func (h *Handlers) deleteBaseline(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteBaseline(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "baseline deleted"})
}

func (h *Handlers) generateReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GenerateReport(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "report generated", Data: report})
}

func (h *Handlers) addRule(c *gin.Context) {
	var rule MonitorRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	if err := h.manager.AddRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response{Code: 0, Message: "rule added", Data: rule})
}

func (h *Handlers) listRules(c *gin.Context) {
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: h.manager.ListRules()})
}

func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.manager.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: rule})
}

func (h *Handlers) updateRule(c *gin.Context) {
	var rule MonitorRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	rule.ID = c.Param("id")
	if err := h.manager.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "rule updated", Data: rule})
}

func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "rule deleted"})
}

func (h *Handlers) runScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	if req.Mode == "" {
		req.Mode = ScanModeFull
	}

	result, err := h.manager.RunScan(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "scan completed", Data: result})
}

func (h *Handlers) getScanResults(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: h.manager.GetScanResults(limit)})
}

func (h *Handlers) listChanges(c *gin.Context) {
	req := &ListChangesRequest{}
	req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	req.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	req.Level = AlertLevel(c.Query("level"))

	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			req.Since = &t
		}
	}
	if untilStr := c.Query("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			req.Until = &t
		}
	}
	if ackedStr := c.Query("acked"); ackedStr != "" {
		acked := ackedStr == "true"
		req.Acked = &acked
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: h.manager.ListChanges(req)})
}

func (h *Handlers) acknowledgeChange(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Notes string `json:"notes"`
	}
	c.ShouldBindJSON(&req)

	if err := h.manager.AcknowledgeChange(id, req.Notes); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "change acknowledged"})
}

func (h *Handlers) getRepairSuggestions(c *gin.Context) {
	id := c.Param("id")
	suggestions, err := h.manager.GetRepairSuggestions(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: suggestions})
}

func (h *Handlers) getAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: h.manager.GetAlerts(limit)})
}

func (h *Handlers) exportAuditLog(c *gin.Context) {
	req := &ExportAuditLogRequest{
		Format: c.DefaultQuery("format", "json"),
	}
	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			req.Since = &t
		}
	}
	if untilStr := c.Query("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			req.Until = &t
		}
	}
	req.Action = c.Query("action")

	data, err := h.manager.ExportAuditLog(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	if req.Format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=audit_log.csv")
		c.Data(http.StatusOK, "text/csv", data)
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: json.RawMessage(data)})
}

func (h *Handlers) start(c *gin.Context) {
	if err := h.manager.Start(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "FIM manager started"})
}

func (h *Handlers) stop(c *gin.Context) {
	h.manager.Stop()
	c.JSON(http.StatusOK, response{Code: 0, Message: "FIM manager stopped"})
}
