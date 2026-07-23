// Package storage 提供存储管理 API 处理器
package storage

import (
	"fmt"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== 空间分析 ==========

// AnalyzeSpaceRequest 空间分析请求.
type AnalyzeSpaceRequest struct {
	Path               string `json:"path"`               // 分析路径（可选）
	IncludeHidden      bool   `json:"includeHidden"`      // 包含隐藏文件
	LargeFileThreshold uint64 `json:"largeFileThreshold"` // 大文件阈值（字节）
	TopDirCount        int    `json:"topDirCount"`        // 返回前N个目录
	TopFileTypes       int    `json:"topFileTypes"`       // 返回前N个文件类型
	AnalyzeDepth       int    `json:"analyzeDepth"`       // 分析深度
	EnableTrend        bool   `json:"enableTrend"`        // 启用趋势预测
}

// analyzeSpace 执行空间分析
// @Summary 执行空间分析
// @Description 对指定卷执行全面的存储空间分析
// @Tags storage
// @Accept json
// @Param volume path string true "卷名称"
// @Param request body AnalyzeSpaceRequest false "分析选项"
// @Success 200 {object} api.Response{data=AnalyzeResult}
// @Failure 400,404 {object} api.Response
// @Router /space/analyze/{volume} [get].
func (h *Handlers) analyzeSpace(c *gin.Context) {
	volumeName := c.Param("volume")

	// 从查询参数或请求体获取选项
	opts := DefaultAnalyzeOptions

	// 尝试从查询参数解析
	if path := c.Query("path"); path != "" {
		opts.Path = path
	}
	if c.Query("includeHidden") == "true" {
		opts.IncludeHidden = true
	}
	if threshold := c.Query("largeFileThreshold"); threshold != "" {
		_, _ = fmt.Sscanf(threshold, "%d", &opts.LargeFileThreshold)
	}
	if topDir := c.Query("topDirCount"); topDir != "" {
		_, _ = fmt.Sscanf(topDir, "%d", &opts.TopDirCount)
	}
	if topTypes := c.Query("topFileTypes"); topTypes != "" {
		_, _ = fmt.Sscanf(topTypes, "%d", &opts.TopFileTypes)
	}
	if depth := c.Query("analyzeDepth"); depth != "" {
		_, _ = fmt.Sscanf(depth, "%d", &opts.AnalyzeDepth)
	}
	if c.Query("enableTrend") == "false" {
		opts.EnableTrend = false
	}

	result, err := h.spaceAnalyzer.Analyze(volumeName, opts)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, result)
}

// getSpaceHistory 获取空间使用历史
// @Summary 获取空间使用历史
// @Description 获取指定卷的空间使用历史记录
// @Tags storage
// @Param volume path string true "卷名称"
// @Param days query int false "查询天数" default(30)
// @Success 200 {object} api.Response{data=[]SpaceRecord}
// @Failure 400,404 {object} api.Response
// @Router /space/history/{volume} [get].
func (h *Handlers) getSpaceHistory(c *gin.Context) {
	volumeName := c.Param("volume")

	days := 30
	if d := c.Query("days"); d != "" {
		_, _ = fmt.Sscanf(d, "%d", &days)
	}

	records, err := h.spaceAnalyzer.GetHistory(volumeName, days)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, records)
}

// getSpaceTrend 获取空间趋势预测
// @Summary 获取空间趋势预测
// @Description 获取指定卷的空间使用趋势和预测
// @Tags storage
// @Param volume path string true "卷名称"
// @Success 200 {object} api.Response{data=SpaceTrend}
// @Failure 400,404 {object} api.Response
// @Router /space/trend/{volume} [get].
func (h *Handlers) getSpaceTrend(c *gin.Context) {
	volumeName := c.Param("volume")

	// 获取卷信息
	vol := h.manager.GetVolume(volumeName)
	if vol == nil {
		api.NotFound(c, "卷不存在: "+volumeName)
		return
	}

	trend := h.spaceAnalyzer.predictTrend(volumeName, vol)
	api.OK(c, trend)
}

