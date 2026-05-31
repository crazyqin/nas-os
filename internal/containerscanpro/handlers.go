// Package containerscanpro 提供容器安全扫描增强功能，包括 CVE 漏洞检测、
// 合规策略检查、运行时异常检测、自动修复建议、扫描报告生成等。
package containerscanpro

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 提供容器安全扫描的 HTTP API handlers
type Handler struct {
	manager *Manager
}

// NewHandler 创建新的 HTTP handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册 API 路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	csp := rg.Group("/containerscanpro")
	{
		csp.POST("/scan", h.handleScan)
		csp.GET("/scans", h.handleListScans)
		csp.GET("/scans/:id", h.handleGetScan)
		csp.GET("/vulnerabilities", h.handleListVulnerabilities)
		csp.POST("/policies", h.handleCreatePolicy)
		csp.GET("/policies", h.handleListPolicies)
		csp.POST("/autofix", h.handleAutoFix)
		csp.GET("/compliance", h.handleComplianceReport)
		csp.GET("/runtime", h.handleRuntimeMonitors)
	}
}

// handleScan POST /containerscanpro/scan - 扫描镜像
func (h *Handler) handleScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	result, err := h.manager.ScanImage(req.ImageName, req.PolicyID)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: result})
}

// handleListScans GET /containerscanpro/scans - 扫描历史
func (h *Handler) handleListScans(c *gin.Context) {
	results := h.manager.ListScanResults()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: results})
}

// handleGetScan GET /containerscanpro/scans/:id - 扫描详情
func (h *Handler) handleGetScan(c *gin.Context) {
	scanID := c.Param("id")
	result, err := h.manager.GetScanResult(scanID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: result})
}

// handleListVulnerabilities GET /containerscanpro/vulnerabilities - 漏洞列表
func (h *Handler) handleListVulnerabilities(c *gin.Context) {
	severity := c.Query("severity")
	vulns := h.manager.ListVulnerabilities(severity)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: vulns})
}

// handleCreatePolicy POST /containerscanpro/policies - 创建策略
func (h *Handler) handleCreatePolicy(c *gin.Context) {
	var req PolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	policy, err := h.manager.CreatePolicy(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policy})
}

// handleListPolicies GET /containerscanpro/policies - 策略列表
func (h *Handler) handleListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: policies})
}

// handleAutoFix POST /containerscanpro/autofix - 自动修复
func (h *Handler) handleAutoFix(c *gin.Context) {
	var req AutoFixRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	fixes, err := h.manager.AutoFix(req.ScanID, req.VulnIDs, req.FixAction)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: fixes})
}

// handleComplianceReport GET /containerscanpro/compliance - 合规报告
func (h *Handler) handleComplianceReport(c *gin.Context) {
	standard := ComplianceStandard(c.Query("standard"))
	report := h.manager.GenerateComplianceReport(standard)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: report})
}

// handleRuntimeMonitors GET /containerscanpro/runtime - 运行时监控状态
func (h *Handler) handleRuntimeMonitors(c *gin.Context) {
	containerID := c.Query("container_id")
	if containerID != "" {
		mon, err := h.manager.GetRuntimeMonitor(containerID)
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: mon})
		return
	}
	monitors := h.manager.ListRuntimeMonitors()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: monitors})
}
