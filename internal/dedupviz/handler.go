// Package dedupviz 提供 REST API 处理器
package dedupviz

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 去重可视化 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	dedup := r.Group("/dedup-viz")
	{
		// 扫描操作
		dedup.POST("/scan", h.startScan)
		dedup.GET("/scan/:id", h.getScanResult)
		dedup.GET("/scans", h.listScanResults)
		dedup.GET("/scan/latest", h.getLatestScan)

		// 可视化数据
		dedup.GET("/visualization/:id", h.getVisualization)
		dedup.GET("/visualization/latest", h.getLatestVisualization)

		// 重复文件查询
		dedup.GET("/duplicates/:id", h.getDuplicates)
		dedup.GET("/duplicates/:id/by-type", h.getDuplicatesByType)
		dedup.GET("/duplicates/:id/by-size", h.getDuplicatesBySize)

		// 删除操作
		dedup.POST("/delete", h.deleteDuplicates)
		dedup.POST("/delete/batch", h.batchDelete)

		// 导出
		dedup.GET("/export/:id", h.exportResult)

		// 配置
		dedup.GET("/config", h.getConfig)
		dedup.PUT("/config", h.updateConfig)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// startScan 启动扫描
func (h *Handlers) startScan(c *gin.Context) {
	var req struct {
		Paths      []string    `json:"paths"`
		ScanConfig *ScanConfig `json:"scan_config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	// 使用默认路径
	if len(req.Paths) == 0 {
		req.Paths = h.manager.GetConfig().DefaultPaths
	}

	if len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "no paths specified"})
		return
	}

	result, err := h.manager.ScanDirectory(req.Paths, req.ScanConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, response{Code: 0, Message: "scan started", Data: result})
}

// getScanResult 获取扫描结果
func (h *Handlers) getScanResult(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetScanResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// listScanResults 列出扫描结果
func (h *Handlers) listScanResults(c *gin.Context) {
	results := h.manager.ListScanResults()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: results})
}

// getLatestScan 获取最新扫描
func (h *Handlers) getLatestScan(c *gin.Context) {
	result := h.manager.GetLastScanResult()
	if result == nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: "no scan results available"})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// getVisualization 获取可视化数据
func (h *Handlers) getVisualization(c *gin.Context) {
	id := c.Param("id")
	data, err := h.manager.GetVisualizationData(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: data})
}

// getLatestVisualization 获取最新可视化数据
func (h *Handlers) getLatestVisualization(c *gin.Context) {
	data, err := h.manager.GetLastVisualizationData()
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: data})
}

// getDuplicates 获取重复文件列表
func (h *Handlers) getDuplicates(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetScanResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"scan_id":          id,
			"duplicate_groups": result.DuplicateGroups,
			"duplicate_files":  result.DuplicateFiles,
			"wasted_size":      result.WastedSize,
			"groups":           result.Groups,
		},
	})
}

// getDuplicatesByType 按类型获取重复文件
func (h *Handlers) getDuplicatesByType(c *gin.Context) {
	fileType := FileType(c.Query("type"))
	if fileType == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "type parameter required"})
		return
	}

	groups, err := h.manager.GetDuplicatesByType(fileType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: groups})
}

// getDuplicatesBySize 按大小获取重复文件
func (h *Handlers) getDuplicatesBySize(c *gin.Context) {
	minSize := int64(0)
	maxSize := int64(0)

	// 简化实现，实际应解析查询参数
	groups, err := h.manager.GetDuplicatesBySizeRange(minSize, maxSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: groups})
}

// deleteDuplicates 删除重复文件
func (h *Handlers) deleteDuplicates(c *gin.Context) {
	var req DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	result, err := h.manager.DeleteDuplicates(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// batchDelete 批量删除
func (h *Handlers) batchDelete(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	result, err := h.manager.BatchDeleteDuplicates(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// exportResult 导出结果
func (h *Handlers) exportResult(c *gin.Context) {
	id := c.Param("id")
	data, err := h.manager.ExportScanResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=dedup-report-"+id+".json")
	c.Data(http.StatusOK, "application/json", data)
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg DedupvizConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}
	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{Code: 0, Message: "config updated"})
}
