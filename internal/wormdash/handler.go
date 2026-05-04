package wormdash

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 合规仪表盘HTTP处理器
type Handler struct {
	dashboard *Dashboard
	logger    *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(dashboard *Dashboard, logger *zap.Logger) *Handler {
	return &Handler{dashboard: dashboard, logger: logger}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	wd := rg.Group("/wormdash")
	{
		wd.GET("/overview", h.Overview)
		wd.GET("/policies", h.ListPolicies)
		wd.POST("/policies", h.CreatePolicy)
		wd.GET("/policies/:id", h.GetPolicy)
		wd.PUT("/policies/:id", h.UpdatePolicy)
		wd.DELETE("/policies/:id", h.DeletePolicy)
		wd.POST("/report", h.GenerateReport)
		wd.GET("/reports", h.ListReports)
		wd.GET("/alerts", h.ListAlerts)
		wd.POST("/alerts/bypass", h.ReportBypass)
		wd.POST("/alerts/:id/resolve", h.ResolveAlert)
		wd.GET("/retention", h.ListRetention)
		wd.POST("/retention", h.AddRetention)
		wd.POST("/retention/:id/extend", h.ExtendRetention)
		wd.GET("/audit", h.ListAudit)
	}
}

// Overview 获取合规概览
func (h *Handler) Overview(c *gin.Context) {
	overview := h.dashboard.Overview()
	c.JSON(http.StatusOK, overview)
}

// ListPolicies 列出策略
func (h *Handler) ListPolicies(c *gin.Context) {
	status := PolicyStatus(c.Query("status"))
	policies := h.dashboard.ListPolicies(status)
	c.JSON(http.StatusOK, policies)
}

// CreatePolicy 创建策略
func (h *Handler) CreatePolicy(c *gin.Context) {
	var req struct {
		Name          string `json:"name" binding:"required"`
		Scope         string `json:"scope" binding:"required"`
		Target        string `json:"target" binding:"required"`
		RetentionDays int    `json:"retentionDays"`
		Description   string `json:"description"`
		CreatedBy     string `json:"createdBy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	policy := h.dashboard.AddPolicy(req.Name, PolicyScope(req.Scope), req.Target, req.RetentionDays, req.Description, req.CreatedBy)
	h.logger.Info("WORM策略已创建",
		zap.String("policy_id", policy.ID),
		zap.String("name", policy.Name),
		zap.String("actor", req.CreatedBy),
	)
	c.JSON(http.StatusCreated, policy)
}

// GetPolicy 获取策略
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, ok := h.dashboard.GetPolicy(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// UpdatePolicy 更新策略
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name          *string `json:"name"`
		RetentionDays *int    `json:"retentionDays"`
		Status        *string `json:"status"`
		Actor         string  `json:"actor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var status *PolicyStatus
	if req.Status != nil {
		s := PolicyStatus(*req.Status)
		status = &s
	}
	policy, err := h.dashboard.UpdatePolicy(id, req.Name, req.RetentionDays, status, req.Actor)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// DeletePolicy 删除策略
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	actor := c.Query("actor")
	if err := h.dashboard.DeletePolicy(id, actor); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// GenerateReport 生成合规报告
func (h *Handler) GenerateReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report, err := h.dashboard.GenerateReport(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.logger.Info("合规报告已生成",
		zap.String("report_id", report.ID),
		zap.String("type", report.ReportType),
		zap.String("actor", req.GeneratedBy),
	)
	c.JSON(http.StatusCreated, report)
}

// ListReports 列出报告
func (h *Handler) ListReports(c *gin.Context) {
	reports := h.dashboard.ListReports()
	c.JSON(http.StatusOK, reports)
}

// ListAlerts 列出告警
func (h *Handler) ListAlerts(c *gin.Context) {
	var resolved *bool
	if v := c.Query("resolved"); v != "" {
		b := v == "true"
		resolved = &b
	}
	alerts := h.dashboard.ListAlerts(resolved)
	c.JSON(http.StatusOK, alerts)
}

// ReportBypass 报告绕过尝试
func (h *Handler) ReportBypass(c *gin.Context) {
	var req struct {
		SourcePath  string `json:"sourcePath" binding:"required"`
		SourceIP    string `json:"sourceIp"`
		UserID      string `json:"userId"`
		Description string `json:"description" binding:"required"`
		Severity    string `json:"severity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	alert := h.dashboard.ReportBypassAttempt(req.SourcePath, req.SourceIP, req.UserID, req.Description, AlertSeverity(req.Severity))
	h.logger.Warn("检测到WORM绕过尝试",
		zap.String("alert_id", alert.ID),
		zap.String("path", req.SourcePath),
		zap.String("severity", req.Severity),
	)
	c.JSON(http.StatusCreated, alert)
}

// ResolveAlert 解决告警
func (h *Handler) ResolveAlert(c *gin.Context) {
	id := c.Param("id")
	actor := c.Query("actor")
	if err := h.dashboard.ResolveAlert(id, actor); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"resolved": id})
}

// ListRetention 列出保留记录
func (h *Handler) ListRetention(c *gin.Context) {
	entries := h.dashboard.ListRetention()
	c.JSON(http.StatusOK, entries)
}

// AddRetention 添加保留记录
func (h *Handler) AddRetention(c *gin.Context) {
	var req struct {
		FileID        string `json:"fileId" binding:"required"`
		FilePath      string `json:"filePath" binding:"required"`
		RetentionDays int    `json:"retentionDays"`
		Actor         string `json:"actor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry := h.dashboard.AddRetention(req.FileID, req.FilePath, req.RetentionDays, req.Actor)
	c.JSON(http.StatusCreated, entry)
}

// ExtendRetention 延长保留期
func (h *Handler) ExtendRetention(c *gin.Context) {
	id := c.Param("id")
	extraDays, err := strconv.Atoi(c.Query("extraDays"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid extraDays"})
		return
	}
	actor := c.Query("actor")
	entry, err := h.dashboard.ExtendRetention(id, extraDays, actor)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// ListAudit 列出审计日志
func (h *Handler) ListAudit(c *gin.Context) {
	action := AuditAction(c.Query("action"))
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	entries := h.dashboard.ListAudit(action, limit)
	c.JSON(http.StatusOK, entries)
}
