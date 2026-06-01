// DRAID HTTP API 处理器
// 提供 DRAID 阵列管理的 RESTful API 接口
// 遵循项目统一的 API 风格和错误处理模式

package draid

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 提供 DRAID 管理的 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建新的 DRAID 处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 DRAID API 路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	draid := rg.Group("/draid")
	{
		// 阵列管理
		draid.GET("/arrays", h.listArrays)
		draid.POST("/arrays", h.createArray)
		draid.GET("/arrays/:name", h.getArray)
		draid.DELETE("/arrays/:name", h.deleteArray)
		draid.GET("/arrays/:name/status", h.getArrayStatus)

		// 分布式热备管理
		draid.GET("/arrays/:name/spares", h.listDistributedSpares)
		draid.POST("/arrays/:name/spares", h.addDistributedSpare)
		draid.DELETE("/arrays/:name/spares/:device", h.removeDistributedSpare)

		// 设备故障与重建
		draid.POST("/arrays/:name/failure/:device", h.reportDeviceFailure)
		draid.POST("/arrays/:name/rebuild", h.rebuildArray)
		draid.PUT("/arrays/:name/rebuild/progress", h.updateRebuildProgress)

		// 数据重分布
		draid.POST("/arrays/:name/reshare", h.reshareData)
		draid.PUT("/arrays/:name/reshare/progress", h.updateReshareProgress)

		// 性能监控
		draid.GET("/arrays/:name/metrics", h.getMetrics)
		draid.PUT("/arrays/:name/metrics", h.updateMetrics)
	}
}

// listArrays 列出所有 DRAID 阵列
func (h *Handlers) listArrays(c *gin.Context) {
	arrays := h.manager.ListArrays()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": arrays})
}

// createArray 创建 DRAID 阵列
func (h *Handlers) createArray(c *gin.Context) {
	var req struct {
		Name         string   `json:"name" binding:"required"`
		Level        string   `json:"level" binding:"required"`
		Devices      []string `json:"devices" binding:"required"`
		SpareDevices []string `json:"spare_devices"`
		GroupSize    int      `json:"group_size" binding:"required"`
		DataDisks    int      `json:"data_disks" binding:"required"`
		ChunkSize    string   `json:"chunk_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.CreateArray(req.Name, req.Level, req.Devices, req.SpareDevices, req.GroupSize, req.DataDisks, req.ChunkSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "DRAID 阵列已创建"})
}

// getArray 获取 DRAID 阵列信息
func (h *Handlers) getArray(c *gin.Context) {
	name := c.Param("name")
	arr, err := h.manager.GetArray(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": arr})
}

// deleteArray 删除 DRAID 阵列
func (h *Handlers) deleteArray(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeleteArray(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "DRAID 阵列已删除"})
}

// getArrayStatus 获取 DRAID 阵列详细状态
func (h *Handlers) getArrayStatus(c *gin.Context) {
	name := c.Param("name")
	status, err := h.manager.GetArrayStatus(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

// listDistributedSpares 列出分布式热备
func (h *Handlers) listDistributedSpares(c *gin.Context) {
	name := c.Param("name")
	spares, err := h.manager.ListDistributedSpares(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": spares})
}

// addDistributedSpare 添加分布式热备
func (h *Handlers) addDistributedSpare(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Device string `json:"device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.AddDistributedSpare(name, req.Device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "分布式热备已添加"})
}

// removeDistributedSpare 移除分布式热备
func (h *Handlers) removeDistributedSpare(c *gin.Context) {
	name := c.Param("name")
	device := c.Param("device")
	if err := h.manager.RemoveDistributedSpare(name, device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "分布式热备已移除"})
}

// reportDeviceFailure 报告设备故障
func (h *Handlers) reportDeviceFailure(c *gin.Context) {
	name := c.Param("name")
	device := c.Param("device")
	if err := h.manager.ReportDeviceFailure(name, device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "设备故障已报告"})
}

// rebuildArray 触发阵列重建
func (h *Handlers) rebuildArray(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.RebuildArray(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "阵列重建已开始"})
}

// updateRebuildProgress 更新重建进度
func (h *Handlers) updateRebuildProgress(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Progress float64 `json:"progress" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.UpdateRebuildProgress(name, req.Progress); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "重建进度已更新"})
}

// reshareData 触发数据重分布
func (h *Handlers) reshareData(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Devices []string `json:"devices" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.ReshareData(name, req.Devices); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "数据重分布已开始"})
}

// updateReshareProgress 更新数据重分布进度
func (h *Handlers) updateReshareProgress(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Progress float64  `json:"progress" binding:"required"`
		Devices  []string `json:"devices"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.UpdateReshareProgress(name, req.Progress, req.Devices); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "重分布进度已更新"})
}

// getMetrics 获取性能指标
func (h *Handlers) getMetrics(c *gin.Context) {
	name := c.Param("name")
	metrics, err := h.manager.GetMetrics(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": metrics})
}

// updateMetrics 更新性能指标
func (h *Handlers) updateMetrics(c *gin.Context) {
	name := c.Param("name")
	var req PerformanceMetrics
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.UpdateMetrics(name, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "性能指标已更新"})
}
