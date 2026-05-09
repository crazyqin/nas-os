package auditexport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers HTTP API 处理器
type Handlers struct {
	logger   *zap.Logger
	exporter *Exporter
}

// NewHandlers 创建处理器
func NewHandlers(logger *zap.Logger, exporter *Exporter) *Handlers {
	return &Handlers{
		logger:   logger,
		exporter: exporter,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	audit := api.Group("/audit")
	{
		audit.POST("/export", h.handleExport)
		audit.POST("/report", h.handleReport)
		audit.GET("/export/download", h.handleDownload)
	}
}

// handleExport 导出审计日志
func (h *Handlers) handleExport(c *gin.Context) {
	var req struct {
		Filter ExportFilter `json:"filter"`
		Format ExportFormat `json:"format"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	// 默认 JSON 格式
	if req.Format == "" {
		req.Format = FormatJSON
	}

	var data []byte
	var err error

	switch req.Format {
	case FormatCSV:
		data, err = h.exporter.ExportCSV(req.Filter)
	case FormatJSON:
		data, err = h.exporter.ExportJSON(req.Filter)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的导出格式",
		})
		return
	}

	if err != nil {
		h.logger.Error("导出失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "导出失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "导出成功",
		"data": gin.H{
			"format": req.Format,
			"size":   len(data),
			"content": string(data),
		},
	})
}

// handleReport 生成合规报告
func (h *Handlers) handleReport(c *gin.Context) {
	var req struct {
		StartTime *time.Time `json:"start_time"`
		EndTime   *time.Time `json:"end_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数",
		})
		return
	}

	// 默认最近 7 天
	now := time.Now()
	if req.EndTime == nil {
		req.EndTime = &now
	}
	if req.StartTime == nil {
		start := now.AddDate(0, 0, -7)
		req.StartTime = &start
	}

	report := h.exporter.GenerateReport(*req.StartTime, *req.EndTime)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "报告生成成功",
		"data":    report,
	})
}

// handleDownload 下载导出文件
func (h *Handlers) handleDownload(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	filter := ExportFilter{}

	if startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err == nil {
			filter.StartTime = &t
		}
	}
	if endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err == nil {
			filter.EndTime = &t
		}
	}

	var data []byte
	var err error
	var contentType string
	var filename string

	switch ExportFormat(format) {
	case FormatCSV:
		data, err = h.exporter.ExportCSV(filter)
		contentType = "text/csv; charset=utf-8"
		filename = "audit_export.csv"
	case FormatJSON:
		data, err = h.exporter.ExportJSON(filter)
		contentType = "application/json"
		filename = "audit_export.json"
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的导出格式",
		})
		return
	}

	if err != nil {
		h.logger.Error("导出失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "导出失败",
		})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}
