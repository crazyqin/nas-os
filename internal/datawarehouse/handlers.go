package datawarehouse

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API处理器.
type Handler struct {
	warehouse *Warehouse
}

// NewHandler 创建Handler.
func NewHandler(warehouse *Warehouse) *Handler {
	return &Handler{warehouse: warehouse}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	dw := rg.Group("/datawarehouse")
	{
		dw.POST("/ingest", h.Ingest)
		dw.POST("/query", h.Query)
		dw.POST("/rollup", h.Rollup)
		dw.POST("/drilldown", h.DrillDown)
		dw.POST("/pivot", h.Pivot)
		dw.GET("/stats", h.Stats)
		dw.GET("/metrics", h.Metrics)
		dw.GET("/timeseries/:metric", h.TimeSeries)
	}
}

// IngestRequest 数据采集请求.
type IngestRequest struct {
	Source     DataSource         `json:"source" binding:"required"`
	Timestamp  time.Time          `json:"timestamp" binding:"required"`
	Dimensions map[string]string  `json:"dimensions"`
	Measures   map[string]float64 `json:"measures" binding:"required"`
}

// Ingest 数据采集.
func (h *Handler) Ingest(c *gin.Context) {
	var req IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dp := DataPoint{
		Timestamp:  req.Timestamp,
		Source:     req.Source,
		Dimensions: req.Dimensions,
		Measures:   req.Measures,
	}

	h.warehouse.Ingest(dp)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Query 执行查询.
func (h *Handler) Query(c *gin.Context) {
	var req Query
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.warehouse.Query(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RollupRequest 时间聚合请求.
type RollupRequestHTTP struct {
	Source    DataSource        `json:"source" binding:"required"`
	StartTime time.Time         `json:"start_time" binding:"required"`
	EndTime   time.Time         `json:"end_time" binding:"required"`
	Interval  Duration          `json:"interval" binding:"required"`
	Measures  []Measure         `json:"measures" binding:"required"`
	Filters   map[string]string `json:"filters"`
}

// Duration 自定义Duration类型支持JSON解析.
type Duration struct {
	time.Duration
}

// UnmarshalJSON 实现JSON反序列化.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = duration
	return nil
}

// Rollup 时间聚合.
func (h *Handler) Rollup(c *gin.Context) {
	var req RollupRequestHTTP
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rollupReq := RollupRequest{
		Source:    req.Source,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Interval:  req.Interval.Duration,
		Measures:  req.Measures,
		Filters:   req.Filters,
	}

	result, err := h.warehouse.Rollup(rollupReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DrillDownRequestHTTP 钻取请求.
type DrillDownRequestHTTP struct {
	Source    DataSource        `json:"source" binding:"required"`
	StartTime time.Time         `json:"start_time" binding:"required"`
	EndTime   time.Time         `json:"end_time" binding:"required"`
	Dimension string            `json:"dimension" binding:"required"`
	Value     string            `json:"value" binding:"required"`
	DrillDims []string          `json:"drill_dimensions" binding:"required"`
	Measures  []Measure         `json:"measures" binding:"required"`
	Filters   map[string]string `json:"filters"`
}

// DrillDown 钻取查询.
func (h *Handler) DrillDown(c *gin.Context) {
	var req DrillDownRequestHTTP
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	drillReq := DrillDownRequest(req)

	result, err := h.warehouse.DrillDown(drillReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// PivotRequestHTTP 旋转请求.
type PivotRequestHTTP struct {
	Source    DataSource        `json:"source" binding:"required"`
	StartTime time.Time         `json:"start_time" binding:"required"`
	EndTime   time.Time         `json:"end_time" binding:"required"`
	RowDims   []string          `json:"row_dimensions" binding:"required"`
	ColDims   []string          `json:"col_dimensions" binding:"required"`
	Measures  []Measure         `json:"measures" binding:"required"`
	Filters   map[string]string `json:"filters"`
}

// Pivot 旋转查询.
func (h *Handler) Pivot(c *gin.Context) {
	var req PivotRequestHTTP
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pivotReq := PivotRequest(req)

	result, err := h.warehouse.Pivot(pivotReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Stats 统计信息.
func (h *Handler) Stats(c *gin.Context) {
	stats := h.warehouse.Stats()
	c.JSON(http.StatusOK, stats)
}

// Metrics 获取所有指标.
func (h *Handler) Metrics(c *gin.Context) {
	metrics := h.warehouse.timeSeries.Metrics()
	c.JSON(http.StatusOK, gin.H{"metrics": metrics})
}

// TimeSeries 查询时间序列.
func (h *Handler) TimeSeries(c *gin.Context) {
	metric := c.Param("metric")
	startStr := c.Query("start")
	endStr := c.Query("end")

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start time"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end time"})
		return
	}

	points := h.warehouse.timeSeries.Query(metric, start, end)
	c.JSON(http.StatusOK, gin.H{"metric": metric, "points": points})
}
