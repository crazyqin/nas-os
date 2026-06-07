package permaudit

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// Handlers 权限审计 HTTP 处理器
type Handlers struct {
	auditor      *Auditor
	mu           sync.RWMutex
	latestReport *AuditReport
}

// NewHandlers 创建权限审计处理器
func NewHandlers(auditor *Auditor) *Handlers {
	return &Handlers{auditor: auditor}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	perm := api.Group("/permaudit")
	{
		perm.POST("/scan", h.postScan)
		perm.GET("/report", h.getReport)
		perm.GET("/issues", h.getIssues)
	}
}

// postScan 提交用户列表并扫描
func (h *Handlers) postScan(c *gin.Context) {
	var users []UserPerm
	if err := c.ShouldBindJSON(&users); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体: " + err.Error()})
		return
	}
	report := h.auditor.ScanUsers(users)
	h.mu.Lock()
	h.latestReport = &report
	h.mu.Unlock()
	c.JSON(http.StatusOK, report)
}

// getReport 获取最新报告
func (h *Handlers) getReport(c *gin.Context) {
	h.mu.RLock()
	report := h.latestReport
	h.mu.RUnlock()
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "暂无审计报告，请先执行扫描"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// getIssues 仅获取问题列表
func (h *Handlers) getIssues(c *gin.Context) {
	h.mu.RLock()
	report := h.latestReport
	h.mu.RUnlock()
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "暂无审计报告，请先执行扫描"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"issues":           report.Issues,
		"issue_summary":    report.IssueSummary,
		"severity_summary": report.SeveritySummary,
	})
}
