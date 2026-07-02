// Package filededup 存储去重 HTTP API 处理器
package filededup

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP API 处理器.
type Handlers struct {
	manager *ExtendedManager
}

// NewHandlers 创建处理器实例.
func NewHandlers(manager *ExtendedManager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	dedup := r.Group("/dedup")
	{
		dedup.POST("/scans", h.startScan)
		dedup.GET("/scans", h.listScans)
		dedup.GET("/scans/:id", h.getScanResult)
		dedup.GET("/scans/:id/groups", h.getScanGroups)
		dedup.DELETE("/files", h.deleteDuplicate)
		dedup.GET("/recommendations", h.getRecommendations)
	}
}

// startScan 启动扫描
// @Summary 启动文件去重扫描
// @Description 扫描指定路径查找重复文件
// @Tags dedup
// @Accept json
// @Produce json
// @Param request body ScanConfig true "扫描配置"
// @Success 200 {object} ScanResult "扫描完成"
// @Failure 400 {object} map[string]string "请求参数错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /dedup/scans [post].
func (h *Handlers) startScan(c *gin.Context) {
	var config ScanConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.manager.StartScan(&config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// listScans 列出扫描任务
// @Summary 列出所有扫描任务
// @Description 获取所有扫描任务的列表
// @Tags dedup
// @Produce json
// @Success 200 {array} ScanResult "扫描任务列表"
// @Router /dedup/scans [get].
func (h *Handlers) listScans(c *gin.Context) {
	scans := h.manager.ListScans()
	c.JSON(http.StatusOK, scans)
}

// getScanResult 获取扫描结果
// @Summary 获取扫描结果
// @Description 根据ID获取扫描结果详情
// @Tags dedup
// @Produce json
// @Param id path string true "扫描任务ID"
// @Success 200 {object} ScanResult "扫描结果"
// @Failure 404 {object} map[string]string "未找到"
// @Router /dedup/scans/{id} [get].
func (h *Handlers) getScanResult(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetScanResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// getScanGroups 获取重复文件组
// @Summary 获取重复文件组
// @Description 根据扫描ID获取重复文件组列表
// @Tags dedup
// @Produce json
// @Param id path string true "扫描任务ID"
// @Success 200 {array} DuplicateGroup "重复文件组列表"
// @Failure 404 {object} map[string]string "未找到"
// @Router /dedup/scans/{id}/groups [get].
func (h *Handlers) getScanGroups(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetScanResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result.Groups)
}

// deleteDuplicate 删除重复文件
// @Summary 删除重复文件
// @Description 删除指定重复文件组中的重复文件（保留第一个）
// @Tags dedup
// @Produce json
// @Param groupId query string true "重复文件组ID"
// @Param keepIndex query int false "保留文件的索引（默认为0）" default(0)
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /dedup/files [delete].
func (h *Handlers) deleteDuplicate(c *gin.Context) {
	groupID := c.Query("groupId")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "groupId 参数必填"})
		return
	}

	keepIndex := 0 // 默认保留第一个

	if err := h.manager.DeleteDuplicate(groupID, keepIndex); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// getRecommendations 获取清理建议
// @Summary 获取清理建议
// @Description 获取重复文件清理建议列表
// @Tags dedup
// @Produce json
// @Success 200 {array} Recommendation "清理建议列表"
// @Router /dedup/recommendations [get].
func (h *Handlers) getRecommendations(c *gin.Context) {
	recommendations := h.manager.GetRecommendations()
	c.JSON(http.StatusOK, recommendations)
}
