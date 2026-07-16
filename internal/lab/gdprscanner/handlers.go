// Package gdprscanner 提供 REST API 处理器
package gdprscanner

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 隐私合规扫描 API 处理器.
type Handlers struct {
	manager *ScannerManager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *ScannerManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	gdpr := r.Group("/security/gdpr")
	{
		gdpr.POST("/scan", h.ScanFiles)
		gdpr.POST("/scan/paths", h.ScanPaths)
		gdpr.GET("/reports", h.ListReports)
		gdpr.GET("/reports/:id", h.GetReport)
		gdpr.GET("/patterns", h.GetPatterns)
		gdpr.GET("/masking-suggestions", h.GetMaskingSuggestions)
	}
}

// ScanFiles 扫描指定路径.
func (h *Handlers) ScanFiles(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	report, err := h.manager.ScanFiles(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "scan completed", Data: report})
}

// ScanPaths 扫描多个路径.
func (h *Handlers) ScanPaths(c *gin.Context) {
	var req ScanPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	allResults := make([]*ScanResult, 0)
	for _, path := range req.Paths {
		scanReq := ScanRequest{
			Path:       path,
			Extensions: req.Extensions,
		}
		report, err := h.manager.ScanFiles(scanReq)
		if err != nil {
			allResults = append(allResults, &ScanResult{
				FilePath: path,
				Error:    err.Error(),
			})
			continue
		}
		allResults = append(allResults, report.Results...)
	}

	report := h.manager.GenerateComplianceReport(allResults)
	c.JSON(http.StatusOK, response{Code: 0, Message: "scan completed", Data: report})
}

// ListReports 列出所有报告.
func (h *Handlers) ListReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(reports),
			"reports": reports,
		},
	})
}

// GetReport 获取单个报告.
func (h *Handlers) GetReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

// GetPatterns 获取 PII 匹配模式.
func (h *Handlers) GetPatterns(c *gin.Context) {
	patterns := h.manager.GetPatterns()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(patterns),
			"patterns": patterns,
		},
	})
}

// GetMaskingSuggestions 获取脱敏建议.
func (h *Handlers) GetMaskingSuggestions(c *gin.Context) {
	suggestions := h.manager.SuggestMasking()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":       len(suggestions),
			"suggestions": suggestions,
		},
	})
}
