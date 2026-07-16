package raid

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 提供 RAID 管理的 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建新的 RAID 处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 RAID API 路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	raid := rg.Group("/raid")
	{
		raid.GET("/arrays", h.listArrays)
		raid.POST("/arrays", h.createArray)
		raid.GET("/arrays/:name", h.getArray)
		raid.DELETE("/arrays/:name", h.deleteArray)
		raid.POST("/arrays/:name/spare", h.addSpare)
		raid.DELETE("/arrays/:name/spare/:device", h.removeSpare)
		raid.POST("/arrays/:name/rebuild", h.rebuildArray)
		raid.POST("/arrays/:name/expand", h.expandArray)
		raid.GET("/arrays/:name/status", h.getArrayStatus)
	}
}

func (h *Handlers) listArrays(c *gin.Context) {
	arrays := h.manager.ListArrays()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": arrays})
}

func (h *Handlers) createArray(c *gin.Context) {
	var req struct {
		Name         string   `json:"name" binding:"required"`
		Level        string   `json:"level" binding:"required"`
		Devices      []string `json:"devices" binding:"required"`
		SpareDevices []string `json:"spare_devices"`
		ChunkSize    string   `json:"chunk_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.CreateArray(req.Name, req.Level, req.Devices, req.SpareDevices, req.ChunkSize); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "阵列已创建"})
}

func (h *Handlers) getArray(c *gin.Context) {
	name := c.Param("name")
	arr, err := h.manager.GetArray(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": arr})
}

func (h *Handlers) deleteArray(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeleteArray(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "阵列已删除"})
}

func (h *Handlers) addSpare(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Device string `json:"device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.AddSpare(name, req.Device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "备用设备已添加"})
}

func (h *Handlers) removeSpare(c *gin.Context) {
	name := c.Param("name")
	device := c.Param("device")
	if err := h.manager.RemoveSpare(name, device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "备用设备已移除"})
}

func (h *Handlers) rebuildArray(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.RebuildArray(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "阵列重建已开始"})
}

func (h *Handlers) expandArray(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Devices []string `json:"devices" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.ExpandArray(name, req.Devices); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "阵列已扩展"})
}

func (h *Handlers) getArrayStatus(c *gin.Context) {
	name := c.Param("name")
	status, err := h.manager.GetArrayStatus(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}
