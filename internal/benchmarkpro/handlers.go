package benchmarkpro

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 基准测试 HTTP 处理器
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	bench := api.Group("/benchmarkpro")
	{
		bench.POST("/run", h.RunTest)
		bench.GET("/results", h.ListResults)
		bench.GET("/results/:id", h.GetResult)
		bench.GET("/results/:id/bottlenecks", h.GetBottlenecks)
		bench.GET("/results/:id/suggestions", h.GetSuggestions)
		bench.GET("/results/:id/report", h.GetReport)
		bench.GET("/results/:id/report/export", h.ExportReport)
		bench.GET("/trend", h.GetTrend)
		bench.POST("/competitors", h.AddCompetitor)
		bench.GET("/competitors", h.ListCompetitors)
		bench.GET("/competitors/compare/:id/:name", h.CompareCompetitor)
	}
}

// RunTest 启动基准测试
func (h *Handlers) RunTest(c *gin.Context) {
	var req BenchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.mgr.RunTest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, result)
}

// ListResults 列出所有测试结果
func (h *Handlers) ListResults(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.ListResults())
}

// GetResult 获取单个测试结果
func (h *Handlers) GetResult(c *gin.Context) {
	id := c.Param("id")
	result, err := h.mgr.GetResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetBottlenecks 获取性能瓶颈诊断
func (h *Handlers) GetBottlenecks(c *gin.Context) {
	id := c.Param("id")
	result, err := h.mgr.GetResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	bottlenecks := h.mgr.DiagnoseBottlenecks(result)
	c.JSON(http.StatusOK, gin.H{
		"result_id":  id,
		"bottlenecks": bottlenecks,
		"count":      len(bottlenecks),
	})
}

// GetSuggestions 获取优化建议
func (h *Handlers) GetSuggestions(c *gin.Context) {
	id := c.Param("id")
	result, err := h.mgr.GetResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	bottlenecks := h.mgr.DiagnoseBottlenecks(result)
	suggestions := h.mgr.GenerateSuggestions(result, bottlenecks)
	c.JSON(http.StatusOK, gin.H{
		"result_id":   id,
		"suggestions": suggestions,
		"count":       len(suggestions),
	})
}

// GetReport 获取测试报告
func (h *Handlers) GetReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.mgr.GenerateReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// ExportReport 导出测试报告
func (h *Handlers) ExportReport(c *gin.Context) {
	id := c.Param("id")
	data, err := h.mgr.ExportReportJSON(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=benchmark-report.json")
	c.Data(http.StatusOK, "application/json", data)
}

// GetTrend 获取趋势分析
func (h *Handlers) GetTrend(c *gin.Context) {
	testType := c.Query("type")
	analysis := h.mgr.AnalyzeTrend(testType)
	c.JSON(http.StatusOK, analysis)
}

// AddCompetitor 添加竞品数据
func (h *Handlers) AddCompetitor(c *gin.Context) {
	var entry CompetitorEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if entry.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "竞品名称不能为空"})
		return
	}

	h.mgr.AddCompetitor(&entry)
	c.JSON(http.StatusCreated, gin.H{"message": "竞品数据已添加", "name": entry.Name})
}

// ListCompetitors 列出竞品数据
func (h *Handlers) ListCompetitors(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.ListCompetitors())
}

// CompareCompetitor 竞品对比
func (h *Handlers) CompareCompetitor(c *gin.Context) {
	id := c.Param("id")
	name := c.Param("name")

	comparison, err := h.mgr.CompareWithCompetitor(id, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, comparison)
}
