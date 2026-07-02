// Package logcenter REST API 处理器
package logcenter

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Handlers 日志中心 API 处理器.
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(logger *zap.Logger, manager *Manager) *Handlers {
	return &Handlers{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	lg := r.Group("/logcenter")
	{
		// 查询日志
		lg.GET("/logs", h.queryLogs)

		// 获取统计
		lg.GET("/stats", h.getStats)

		// 添加日志
		lg.POST("/logs", h.addLog)

		// 清空日志
		lg.DELETE("/logs", h.clearLogs)

		// 获取配置
		lg.GET("/config", h.getConfig)

		// 更新配置
		lg.PUT("/config", h.updateConfig)

		// 获取来源列表
		lg.GET("/sources", h.getSources)

		// 获取分类列表
		lg.GET("/categories", h.getCategories)

		// 导出日志
		lg.GET("/export", h.exportLogs)

		// 实时日志流 (WebSocket)
		lg.GET("/stream", h.streamLogs)
	}
}

// queryLogs 查询日志.
func (h *Handlers) queryLogs(c *gin.Context) {
	var query LogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 解析时间参数
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			query.StartTime = t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			query.EndTime = t
		}
	}

	result := h.manager.Query(query)
	c.JSON(http.StatusOK, result)
}

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// addLog 添加日志.
func (h *Handlers) addLog(c *gin.Context) {
	var entry LogEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	h.manager.Add(entry)
	c.JSON(http.StatusCreated, gin.H{"message": "日志已添加"})
}

// clearLogs 清空日志.
func (h *Handlers) clearLogs(c *gin.Context) {
	h.manager.Clear()
	c.JSON(http.StatusOK, gin.H{"message": "日志已清空"})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, config)
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var config LogConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// getSources 获取来源列表.
func (h *Handlers) getSources(c *gin.Context) {
	sources := h.manager.GetSources()
	c.JSON(http.StatusOK, gin.H{"sources": sources})
}

// getCategories 获取分类列表.
func (h *Handlers) getCategories(c *gin.Context) {
	categories := h.manager.GetCategories()
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// exportLogs 导出日志.
func (h *Handlers) exportLogs(c *gin.Context) {
	var query LogQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	format := c.DefaultQuery("format", "json")

	data, err := h.manager.Export(query, format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=logs.csv")
	default:
		c.Header("Content-Type", "application/json")
	}

	c.Data(http.StatusOK, c.GetHeader("Content-Type"), data)
}

// streamLogs 实时日志流 (WebSocket).
func (h *Handlers) streamLogs(c *gin.Context) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket 升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	// 订阅日志流
	ch := h.manager.Subscribe()
	defer h.manager.Unsubscribe(ch)

	// 发送初始统计
	stats := h.manager.GetStats()
	initMsg := LogStreamMessage{Type: "stats", Stats: &stats}
	if err := conn.WriteJSON(initMsg); err != nil {
		h.logger.Error("发送初始统计失败", zap.Error(err))
		return
	}

	// 监听日志和连接关闭
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(msg); err != nil {
				h.logger.Error("发送日志失败", zap.Error(err))
				return
			}
		}
	}
}

// parseQueryParams 解析查询参数.
func parseQueryParams(c *gin.Context) LogQuery {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	return LogQuery{
		Keywords: c.Query("keywords"),
		Level:    LogLevel(c.Query("level")),
		Source:   LogSource(c.Query("source")),
		Category: c.Query("category"),
		Hostname: c.Query("hostname"),
		Service:  c.Query("service"),
		Page:     page,
		PageSize: pageSize,
		SortBy:   c.DefaultQuery("sort_by", "timestamp"),
		SortDesc: c.DefaultQuery("sort_desc", "true") == "true",
	}
}
