// Package resmetering 提供资源计量HTTP接口
package resmetering

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 资源计量HTTP处理器
type Handler struct {
	service *Service
}

// NewHandler 创建HTTP处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/resmetering")
	{
		g.GET("/summary", h.Summary)
		g.GET("/by-user", h.ByUser)
		g.GET("/by-container", h.ByContainer)
		g.POST("/record", h.Record)
	}
}

// parseTimeRange 解析时间范围参数
// period: hourly/daily/monthly
// 返回: from, to, period
func parseTimeRange(c *gin.Context) (time.Time, time.Time, AggregationPeriod) {
	now := time.Now()
	period := AggregationPeriod(c.DefaultQuery("period", "daily"))

	var from time.Time
	switch period {
	case PeriodHourly:
		from = now.Add(-1 * time.Hour)
	case PeriodDaily:
		from = now.AddDate(0, 0, -1)
	case PeriodMonthly:
		from = now.AddDate(0, -1, 0)
	default:
		period = PeriodDaily
		from = now.AddDate(0, 0, -1)
	}

	// 支持自定义时间范围
	if fromStr := c.Query("from"); fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = parsed
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			now = parsed
		}
	}

	return from, now, period
}

// Summary 获取资源使用汇总
// GET /api/v1/resmetering/summary?period=daily&from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z
func (h *Handler) Summary(c *gin.Context) {
	from, to, period := parseTimeRange(c)
	summary := h.service.GetSummary(period, from, to)
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

// ByUser 按用户维度获取资源使用报告
// GET /api/v1/resmetering/by-user?period=daily
func (h *Handler) ByUser(c *gin.Context) {
	from, to, period := parseTimeRange(c)
	report := h.service.GetByUser(period, from, to)
	c.JSON(http.StatusOK, gin.H{"data": report})
}

// ByContainer 按容器维度获取资源使用报告
// GET /api/v1/resmetering/by-container?period=hourly
func (h *Handler) ByContainer(c *gin.Context) {
	from, to, period := parseTimeRange(c)
	report := h.service.GetByContainer(period, from, to)
	c.JSON(http.StatusOK, gin.H{"data": report})
}

// Record 提交一条资源采样数据
// POST /api/v1/resmetering/record
func (h *Handler) Record(c *gin.Context) {
	var sample Sample
	if err := c.ShouldBindJSON(&sample); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if sample.Timestamp.IsZero() {
		sample.Timestamp = time.Now()
	}

	h.service.Record(sample)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// parsePeriod 辅助函数：将字符串转为AggregationPeriod
// 用于测试中快速构造
func parsePeriod(s string) AggregationPeriod {
	return AggregationPeriod(s)
}

// dummy reference to keep strconv imported (used in future pagination)
var _ = strconv.Itoa
