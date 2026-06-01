// Package diagnostics HTTP API 处理器
package diagnostics

import (
	"net/http"
	"strconv"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 诊断HTTP处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建诊断处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册诊断路由
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	diag := apiGroup.Group("/diagnostics")
	{
		// 诊断操作
		diag.POST("/run", h.runDiagnostic)
		diag.GET("/latest", h.getLatestReport)

		// 历史记录
		diag.GET("/history", h.getHistory)
		diag.GET("/trend", h.getTrend)

		// 单项诊断
		diag.GET("/cpu", h.diagnoseCPU)
		diag.GET("/memory", h.diagnoseMemory)
		diag.GET("/disk", h.diagnoseDisk)
		diag.GET("/network", h.diagnoseNetwork)

		// 系统状态
		diag.GET("/status", h.getSystemStatus)
	}
}

// runDiagnostic 执行一键诊断
// @Summary 执行系统诊断
// @Description 对CPU、内存、磁盘、网络进行全面诊断
// @Tags diagnostics
// @Produce json
// @Success 200 {object} api.Response{data=DiagnosticReport}
// @Failure 500 {object} api.Response
// @Router /diagnostics/run [post]
func (h *Handlers) runDiagnostic(c *gin.Context) {
	report, err := h.manager.RunDiagnostic()
	if err != nil {
		api.InternalError(c, "诊断执行失败: "+err.Error())
		return
	}
	api.OK(c, report)
}

// getLatestReport 获取最新诊断报告
// @Summary 获取最新报告
// @Description 获取最近一次诊断的结果
// @Tags diagnostics
// @Produce json
// @Success 200 {object} api.Response{data=DiagnosticReport}
// @Failure 404 {object} api.Response
// @Router /diagnostics/latest [get]
func (h *Handlers) getLatestReport(c *gin.Context) {
	report := h.manager.GetLatestReport()
	if report == nil {
		api.NotFound(c, "暂无诊断记录")
		return
	}
	api.OK(c, report)
}

// getHistory 获取诊断历史
// @Summary 获取历史记录
// @Description 获取诊断历史记录列表
// @Tags diagnostics
// @Produce json
// @Param limit query int false "返回数量" default(10)
// @Success 200 {object} api.Response{data=[]DiagnosticReport}
// @Router /diagnostics/history [get]
func (h *Handlers) getHistory(c *gin.Context) {
	limit := 10
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	history := h.manager.GetHistory(limit)
	api.OK(c, history)
}

// getTrend 获取趋势数据
// @Summary 获取趋势数据
// @Description 获取指定时间范围内的趋势数据
// @Tags diagnostics
// @Produce json
// @Param hours query int false "时间范围(小时)" default(24)
// @Success 200 {object} api.Response{data=[]TrendPoint}
// @Router /diagnostics/trend [get]
func (h *Handlers) getTrend(c *gin.Context) {
	hours := 24
	if h, err := strconv.Atoi(c.Query("hours")); err == nil && h > 0 {
		hours = h
	}

	trend := h.manager.GetTrend(hours)
	api.OK(c, trend)
}

// diagnoseCPU 诊断CPU
// @Summary CPU诊断
// @Description 获取CPU诊断信息
// @Tags diagnostics
// @Produce json
// @Success 200 {object} api.Response{data=CPUDiag}
// @Router /diagnostics/cpu [get]
func (h *Handlers) diagnoseCPU(c *gin.Context) {
	report, err := h.manager.RunDiagnostic()
	if err != nil {
		api.InternalError(c, "CPU诊断失败: "+err.Error())
		return
	}
	api.OK(c, report.CPU)
}

// diagnoseMemory 诊断内存
// @Summary 内存诊断
// @Description 获取内存诊断信息
// @Tags diagnostics
// @Produce json
// @Success 200 {object} api.Response{data=MemoryDiag}
// @Router /diagnostics/memory [get]
func (h *Handlers) diagnoseMemory(c *gin.Context) {
	report, err := h.manager.RunDiagnostic()
	if err != nil {
		api.InternalError(c, "内存诊断失败: "+err.Error())
		return
	}
	api.OK(c, report.Memory)
}

// diagnoseDisk 诊断磁盘
// @Summary 磁盘诊断
// @Description 获取磁盘诊断信息
// @Tags diagnostics
// @Produce json
// @Success 200 {object} api.Response{data=DiskDiag}
// @Router /diagnostics/disk [get]
func (h *Handlers) diagnoseDisk(c *gin.Context) {
	report, err := h.manager.RunDiagnostic()
	if err != nil {
		api.InternalError(c, "磁盘诊断失败: "+err.Error())
		return
	}
	api.OK(c, report.Disk)
}

// diagnoseNetwork 诊断网络
// @Summary 网络诊断
// @Description 获取网络诊断信息
// @Tags diagnostics
// @Produce json
// @Success 200 {object} api.Response{data=NetworkDiag}
// @Router /diagnostics/network [get]
func (h *Handlers) diagnoseNetwork(c *gin.Context) {
	report, err := h.manager.RunDiagnostic()
	if err != nil {
		api.InternalError(c, "网络诊断失败: "+err.Error())
		return
	}
	api.OK(c, report.Network)
}

// getSystemStatus 获取系统状态概览
// @Summary 系统状态
// @Description 获取系统整体状态概览
// @Tags diagnostics
// @Produce json
// @Success 200 {object} api.Response
// @Router /diagnostics/status [get]
func (h *Handlers) getSystemStatus(c *gin.Context) {
	report := h.manager.GetLatestReport()
	if report == nil {
		// 如果没有历史记录，执行一次诊断
		var err error
		report, err = h.manager.RunDiagnostic()
		if err != nil {
			api.InternalError(c, "获取系统状态失败: "+err.Error())
			return
		}
	}

	status := gin.H{
		"score":       report.Score,
		"status":      report.Status,
		"summary":     report.Summary,
		"lastCheck":   report.Timestamp,
		"problemCount": len(report.Problems),
		"suggestionCount": len(report.Suggestions),
		"cpu": gin.H{
			"usage":  report.CPU.Usage,
			"score":  report.CPU.Score,
			"status": report.CPU.Status,
		},
		"memory": gin.H{
			"usedPercent": report.Memory.UsedPercent,
			"score":       report.Memory.Score,
			"status":      report.Memory.Status,
		},
		"disk": gin.H{
			"usedPercent": report.Disk.UsedPercent,
			"score":       report.Disk.Score,
			"status":      report.Disk.Status,
		},
		"network": gin.H{
			"connectivity": report.Network.Connectivity,
			"latency":      report.Network.Latency,
			"score":        report.Network.Score,
			"status":       report.Network.Status,
		},
	}

	api.OK(c, status)
}

// APIResponse 用于Swagger文档的响应类型
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// HTTPStatusFromCode 根据业务码返回HTTP状态码
func HTTPStatusFromCode(code int) int {
	switch code {
	case http.StatusOK:
		return http.StatusOK
	case http.StatusBadRequest:
		return http.StatusBadRequest
	case http.StatusUnauthorized:
		return http.StatusUnauthorized
	case http.StatusForbidden:
		return http.StatusForbidden
	case http.StatusNotFound:
		return http.StatusNotFound
	case http.StatusInternalServerError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
