// Package filetimemachine2 提供 HTTP API 处理器
package filetimemachine2

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 时间机器 API 处理器
type Handlers struct {
	engine *TimeMachineEngine
}

// NewHandlers 创建处理器
func NewHandlers(engine *TimeMachineEngine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	tm := r.Group("/filetimemachine2")
	{
		// 快照管理
		tm.GET("/snapshots", h.listSnapshots)
		tm.POST("/snapshots", h.createSnapshot)
		tm.GET("/snapshots/:id", h.getSnapshot)
		tm.DELETE("/snapshots/:id", h.deleteSnapshot)

		// 快照浏览
		tm.GET("/snapshots/:id/browse", h.browseSnapshot)
		tm.GET("/snapshots/:id/files/*path", h.getFileContent)

		// 快照恢复
		tm.POST("/snapshots/:id/restore", h.restoreSnapshot)

		// 差异对比
		tm.GET("/diff", h.diffSnapshots)

		// 时间线
		tm.GET("/timeline", h.getTimeline)

		// 搜索
		tm.GET("/search", h.searchFiles)

		// 存储统计
		tm.GET("/stats", h.getStorageStats)

		// 保留策略
		tm.GET("/retention", h.getRetentionConfig)
		tm.PUT("/retention", h.updateRetentionConfig)
		tm.POST("/retention/cleanup", h.cleanupExpired)

		// 标签管理
		tm.POST("/snapshots/:id/tag", h.addTags)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// successResponse 成功响应
func successResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// errorResponse 错误响应
func errorResponse(c *gin.Context, code int, msg string) {
	c.JSON(code, response{
		Code:    -1,
		Message: msg,
	})
}

// listSnapshots 获取快照列表
func (h *Handlers) listSnapshots(c *gin.Context) {
	snapshots := h.engine.ListSnapshots()
	successResponse(c, snapshots)
}

// createSnapshot 创建快照
func (h *Handlers) createSnapshot(c *gin.Context) {
	var req CreateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	snapshot, err := h.engine.CreateSnapshot(req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "创建快照失败: "+err.Error())
		return
	}

	successResponse(c, snapshot)
}

// getSnapshot 获取快照详情
func (h *Handlers) getSnapshot(c *gin.Context) {
	id := c.Param("id")
	snapshot, err := h.engine.GetSnapshot(id)
	if err != nil {
		errorResponse(c, http.StatusNotFound, err.Error())
		return
	}
	successResponse(c, snapshot)
}

// deleteSnapshot 删除快照
func (h *Handlers) deleteSnapshot(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.DeleteSnapshot(id); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(c, gin.H{"deleted": id})
}

// browseSnapshot 浏览快照内容
func (h *Handlers) browseSnapshot(c *gin.Context) {
	id := c.Param("id")
	subPath := c.Query("path")

	content, err := h.engine.BrowseSnapshot(id, subPath)
	if err != nil {
		errorResponse(c, http.StatusNotFound, err.Error())
		return
	}
	successResponse(c, content)
}

// getFileContent 获取文件内容
func (h *Handlers) getFileContent(c *gin.Context) {
	id := c.Param("id")
	filePath := c.Param("path")

	data, err := h.engine.GetFileContent(id, filePath)
	if err != nil {
		errorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	c.Data(http.StatusOK, "application/octet-stream", data)
}

// restoreSnapshot 恢复快照
func (h *Handlers) restoreSnapshot(c *gin.Context) {
	id := c.Param("id")
	var req RestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.engine.RestoreSnapshot(id, req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "恢复失败: "+err.Error())
		return
	}

	successResponse(c, result)
}

// diffSnapshots 对比两个快照
func (h *Handlers) diffSnapshots(c *gin.Context) {
	snapshotA := c.Query("snapshot_a")
	snapshotB := c.Query("snapshot_b")

	if snapshotA == "" || snapshotB == "" {
		errorResponse(c, http.StatusBadRequest, "请提供 snapshot_a 和 snapshot_b 参数")
		return
	}

	result, err := h.engine.DiffSnapshots(snapshotA, snapshotB)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	successResponse(c, result)
}

// getTimeline 获取时间线数据
func (h *Handlers) getTimeline(c *gin.Context) {
	granularity := AggregationGranularity(c.Query("granularity"))
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	if startTimeStr != "" {
		startTime, _ = time.Parse(time.RFC3339, startTimeStr)
	}
	if endTimeStr != "" {
		endTime, _ = time.Parse(time.RFC3339, endTimeStr)
	}

	data, err := h.engine.GetTimeline(granularity, startTime, endTime)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, data)
}

// searchFiles 搜索文件版本
func (h *Handlers) searchFiles(c *gin.Context) {
	req := SearchRequest{
		FileName:  c.Query("file_name"),
		StartTime: c.Query("start_time"),
		EndTime:   c.Query("end_time"),
		Tag:       c.Query("tag"),
	}

	if v := c.Query("min_size"); v != "" {
		req.MinSize, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := c.Query("max_size"); v != "" {
		req.MaxSize, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := c.Query("limit"); v != "" {
		req.Limit, _ = strconv.Atoi(v)
	}

	result, err := h.engine.SearchFiles(req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, result)
}

// getStorageStats 获取存储统计
func (h *Handlers) getStorageStats(c *gin.Context) {
	stats := h.engine.GetStorageStats()
	successResponse(c, stats)
}

// getRetentionConfig 获取保留策略配置
func (h *Handlers) getRetentionConfig(c *gin.Context) {
	config := h.engine.GetRetentionConfig()
	successResponse(c, config)
}

// updateRetentionConfig 更新保留策略配置
func (h *Handlers) updateRetentionConfig(c *gin.Context) {
	var req UpdateRetentionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	h.engine.UpdateRetentionConfig(req)
	successResponse(c, h.engine.GetRetentionConfig())
}

// cleanupExpired 清理过期快照
func (h *Handlers) cleanupExpired(c *gin.Context) {
	result, err := h.engine.CleanupExpired()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(c, result)
}

// addTags 添加标签
func (h *Handlers) addTags(c *gin.Context) {
	id := c.Param("id")
	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	if err := h.engine.AddTags(id, req.Tags); err != nil {
		errorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	snapshot, _ := h.engine.GetSnapshot(id)
	successResponse(c, snapshot)
}
