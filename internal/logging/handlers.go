// Package logging 提供结构化日志功能
package logging

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 日志 API 处理器.
type Handlers struct {
	searcher *LogSearcher
	manager  *LogManager
	path     string
}

// NewHandlers 创建日志处理器.
func NewHandlers(logPath string, manager *LogManager) *Handlers {
	return &Handlers{
		searcher: NewLogSearcher(logPath),
		manager:  manager,
		path:     logPath,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1")
	{
		// 日志搜索
		api.GET("/logs", h.searchLogs)
		api.GET("/logs/stream", h.streamLogs)
		api.GET("/logs/stats", h.getStats)
		api.GET("/logs/files", h.listFiles)
		api.GET("/logs/files/:name", h.getFileContent)

		// 日志管理
		api.GET("/loggers", h.listLoggers)
		api.GET("/loggers/:name", h.getLoggerInfo)
		api.PUT("/loggers/:name/level", h.setLoggerLevel)

		// 轮转管理
		api.POST("/logs/rotate", h.forceRotate)
		api.GET("/logs/rotator/status", h.getRotatorStatus)
	}
}

// Response 通用响应.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// searchLogs 搜索日志
// @Summary 搜索日志
// @Description 根据条件搜索日志条目
// @Tags logs
// @Produce json
// @Param level query string false "日志级别 (DEBUG, INFO, WARN, ERROR, FATAL)"
// @Param keyword query string false "搜索关键词"
// @Param source query string false "日志来源"
// @Param request_id query string false "请求 ID"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param limit query int false "结果限制" default(100)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} Response
// @Router /api/v1/logs [get].
func (h *Handlers) searchLogs(c *gin.Context) {
	config := &SearchConfig{
		Path:      h.path,
		Level:     c.Query("level"),
		Keyword:   c.Query("keyword"),
		Source:    c.Query("source"),
		RequestID: c.Query("request_id"),
	}

	// 解析时间参数
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			config.StartTime = t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			config.EndTime = t
		}
	}

	// 解析分页参数
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			config.Limit = l
		}
	}
	if config.Limit == 0 || config.Limit > 1000 {
		config.Limit = 100
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			config.Offset = o
		}
	}

	result, err := h.searcher.Search(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// streamLogs 流式日志
// @Summary 流式日志
// @Description 实时获取日志流
// @Tags logs
// @Produce json
// @Param level query string false "日志级别过滤"
// @Param keyword query string false "关键词过滤"
// @Success 200
// @Router /api/v1/logs/stream [get].
func (h *Handlers) streamLogs(c *gin.Context) {
	config := &SearchConfig{
		Path:    h.path,
		Level:   c.Query("level"),
		Keyword: c.Query("keyword"),
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	ch, err := h.searcher.Stream(ctx, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: "streaming not supported",
		})
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			// 发送 SSE 事件
			c.SSEvent("log", entry)
			flusher.Flush()
		}
	}
}

// getStats 获取日志统计
// @Summary 获取日志统计
// @Description 获取日志目录的统计信息
// @Tags logs
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/logs/stats [get].
func (h *Handlers) getStats(c *gin.Context) {
	stats, err := h.searcher.GetStats(h.path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// listFiles 列出日志文件
// @Summary 列出日志文件
// @Description 获取日志目录下的所有文件列表
// @Tags logs
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/logs/files [get].
func (h *Handlers) listFiles(c *gin.Context) {
	files := make([]gin.H, 0)

	// 获取主日志文件
	if info, err := h.searcher.GetStats(h.path); err == nil {
		files = append(files, gin.H{
			"name":    "current",
			"path":    h.path,
			"size":    info["size"],
			"size_mb": info["size_mb"],
		})
	}

	// 获取备份文件
	if h.manager != nil && h.manager.GetRotator() != nil {
		backups, err := h.manager.GetRotator().ListBackups()
		if err == nil {
			for _, backup := range backups {
				info, err := h.searcher.GetStats(backup)
				if err != nil {
					continue
				}
				files = append(files, gin.H{
					"name":    backup,
					"path":    backup,
					"size":    info["size"],
					"size_mb": info["size_mb"],
				})
			}
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(files),
			"files": files,
		},
	})
}

// getFileContent 获取文件内容
// @Summary 获取日志文件内容
// @Description 获取指定日志文件的内容
// @Tags logs
// @Produce json
// @Param name path string true "文件名"
// @Param lines query int false "行数限制" default(100)
// @Success 200 {object} Response
// @Router /api/v1/logs/files/{name} [get].
func (h *Handlers) getFileContent(c *gin.Context) {
	name := c.Param("name")
	lines := 100
	if l := c.Query("lines"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			lines = parsed
		}
	}

	// 构建文件路径
	filePath := h.path
	if name != "current" {
		filePath = name
	}

	// 搜索该文件
	config := &SearchConfig{
		Path:  filePath,
		Limit: lines,
	}

	result, err := h.searcher.Search(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// listLoggers 列出日志记录器
// @Summary 列出日志记录器
// @Description 获取所有已注册的日志记录器列表
// @Tags logs
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/loggers [get].
func (h *Handlers) listLoggers(c *gin.Context) {
	if h.manager == nil {
		c.JSON(http.StatusOK, Response{
			Code:    0,
			Message: "success",
			Data: gin.H{
				"loggers": []string{},
			},
		})
		return
	}

	names := h.manager.ListLoggers()
	loggers := make([]gin.H, 0, len(names))
	for _, name := range names {
		logger := h.manager.GetLogger(name)
		loggers = append(loggers, gin.H{
			"name":  name,
			"level": logger.GetLevel().String(),
		})
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(loggers),
			"loggers": loggers,
		},
	})
}

// getLoggerInfo 获取日志记录器信息
// @Summary 获取日志记录器信息
// @Description 获取指定日志记录器的详细信息
// @Tags logs
// @Produce json
// @Param name path string true "日志记录器名称"
// @Success 200 {object} Response
// @Router /api/v1/loggers/{name} [get].
func (h *Handlers) getLoggerInfo(c *gin.Context) {
	name := c.Param("name")

	if h.manager == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    1,
			Message: "logger not found",
		})
		return
	}

	logger := h.manager.GetLogger(name)
	if logger == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    1,
			Message: "logger not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"name":  name,
			"level": logger.GetLevel().String(),
		},
	})
}

// setLoggerLevel 设置日志级别
// @Summary 设置日志级别
// @Description 设置指定日志记录器的级别
// @Tags logs
// @Accept json
// @Produce json
// @Param name path string true "日志记录器名称"
// @Param body body setLevelRequest true "日志级别"
// @Success 200 {object} Response
// @Router /api/v1/loggers/{name}/level [put].
func (h *Handlers) setLoggerLevel(c *gin.Context) {
	name := c.Param("name")

	var req setLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if h.manager == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    1,
			Message: "logger not found",
		})
		return
	}

	logger := h.manager.GetLogger(name)
	if logger == nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    1,
			Message: "logger not found",
		})
		return
	}

	level := ParseLevel(req.Level)
	logger.SetLevel(level)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"name":  name,
			"level": level.String(),
		},
	})
}

type setLevelRequest struct {
	Level string `json:"level" binding:"required"`
}

// forceRotate 强制轮转日志
// @Summary 强制轮转日志
// @Description 手动触发日志轮转
// @Tags logs
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/logs/rotate [post].
func (h *Handlers) forceRotate(c *gin.Context) {
	if h.manager == nil || h.manager.GetRotator() == nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1,
			Message: "no rotator configured",
		})
		return
	}

	err := h.manager.GetRotator().ForceRotate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "log rotated successfully",
	})
}

// getRotatorStatus 获取轮转器状态
// @Summary 获取轮转器状态
// @Description 获取日志轮转器的当前状态
// @Tags logs
// @Produce json
// @Success 200 {object} Response
// @Router /api/v1/logs/rotator/status [get].
func (h *Handlers) getRotatorStatus(c *gin.Context) {
	if h.manager == nil || h.manager.GetRotator() == nil {
		c.JSON(http.StatusOK, Response{
			Code:    0,
			Message: "no rotator configured",
			Data: gin.H{
				"enabled": false,
			},
		})
		return
	}

	rotator := h.manager.GetRotator()
	backups, _ := rotator.ListBackups()

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"enabled":      true,
			"path":         rotator.GetPath(),
			"current_size": rotator.GetCurrentSize(),
			"backups":      len(backups),
		},
	})
}
