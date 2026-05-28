package snapshottimeline

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 快照时间线 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	snapshots := r.Group("/snapshots")
	{
		// 时间线
		snapshots.GET("/timeline", h.getTimeline)

		// 统计信息
		snapshots.GET("/stats", h.getStats)

		// CRUD 操作
		snapshots.POST("", h.createSnapshot)
		snapshots.GET("/:id", h.getSnapshot)
		snapshots.DELETE("/:id", h.deleteSnapshot)

		// 恢复操作
		snapshots.POST("/:id/restore", h.restoreSnapshot)

		// 对比操作
		snapshots.GET("/:id/compare/:id2", h.compareSnapshots)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// createSnapshot 创建快照
func (h *Handlers) createSnapshot(c *gin.Context) {
	var req struct {
		PoolID      string   `json:"pool_id" binding:"required"`
		Dataset     string   `json:"dataset" binding:"required"`
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	entry, err := h.manager.CreateSnapshot(req.PoolID, req.Dataset, req.Name, req.Description, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "snapshot created",
		Data:    entry,
	})
}

// getSnapshot 获取快照详情
func (h *Handlers) getSnapshot(c *gin.Context) {
	id := c.Param("id")

	entry, err := h.manager.GetSnapshot(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code: 0,
		Data: entry,
	})
}

// deleteSnapshot 删除快照
func (h *Handlers) deleteSnapshot(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteSnapshot(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "snapshot deleted",
	})
}

// restoreSnapshot 恢复快照
func (h *Handlers) restoreSnapshot(c *gin.Context) {
	id := c.Param("id")

	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	req.SnapshotID = id

	result, err := h.manager.RestoreSnapshot(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: result.Message,
		Data:    result,
	})
}

// getTimeline 获取快照时间线
func (h *Handlers) getTimeline(c *gin.Context) {
	dataset := c.Query("dataset")
	sinceStr := c.Query("since")
	untilStr := c.Query("until")

	var since, until time.Time
	var err error

	if sinceStr != "" {
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{
				Code:    1,
				Message: "invalid since parameter: " + err.Error(),
			})
			return
		}
	}

	if untilStr != "" {
		until, err = time.Parse(time.RFC3339, untilStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{
				Code:    1,
				Message: "invalid until parameter: " + err.Error(),
			})
			return
		}
	}

	entries, err := h.manager.GetTimeline(dataset, since, until)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code: 0,
		Data: entries,
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, response{
		Code: 0,
		Data: stats,
	})
}

// compareSnapshots 对比快照
func (h *Handlers) compareSnapshots(c *gin.Context) {
	id1 := c.Param("id")
	id2 := c.Param("id2")

	diff, err := h.manager.CompareSnapshots(id1, id2)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code: 0,
		Data: diff,
	})
}

// ListSnapshotsRequest 列表请求参数
type ListSnapshotsRequest struct {
	Dataset string        `form:"dataset"`
	PoolID  string        `form:"pool_id"`
	Since   string        `form:"since"`
	Until   string        `form:"until"`
	Tags    string        `form:"tags"`
	State   SnapshotState `form:"state"`
	Limit   int           `form:"limit"`
	Offset  int           `form:"offset"`
}

// listSnapshots 列出快照 (可选实现，供注册)
func (h *Handlers) listSnapshots(c *gin.Context) {
	var req ListSnapshotsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	filter := TimelineFilter{
		Dataset: req.Dataset,
		PoolID:  req.PoolID,
		State:   req.State,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}

	if req.Since != "" {
		since, err := time.Parse(time.RFC3339, req.Since)
		if err == nil {
			filter.Since = since
		}
	}

	if req.Until != "" {
		until, err := time.Parse(time.RFC3339, req.Until)
		if err == nil {
			filter.Until = until
		}
	}

	if req.Tags != "" {
		filter.Tags = splitTags(req.Tags)
	}

	entries, err := h.manager.ListSnapshots(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code: 0,
		Data: entries,
	})
}

// splitTags 分割标签字符串
func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}

	var result []string
	for _, tag := range splitString(tags, ",") {
		trimmed := trimSpace(tag)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// splitString 分割字符串
func splitString(s string, sep string) []string {
	if s == "" {
		return nil
	}

	var result []string
	start := 0

	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}

	result = append(result, s[start:])
	return result
}

// trimSpace 去除空格
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// parseInt 解析整数
func parseInt(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}
