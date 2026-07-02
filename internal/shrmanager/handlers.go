package shrmanager

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 提供 SHR 管理的 HTTP 处理器.
type Handlers struct {
	manager *SHRManager
}

// NewHandlers 创建新的 SHR 处理器.
func NewHandlers(manager *SHRManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 SHR API 路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	shr := rg.Group("/shr")
	{
		// 存储池管理
		shr.GET("/pools", h.listPools)
		shr.POST("/pools", h.createPool)
		shr.GET("/pools/:name", h.getPool)
		shr.DELETE("/pools/:name", h.deletePool)
		shr.GET("/pools/:name/status", h.getPoolStatus)
		shr.POST("/pools/:name/optimize", h.optimizeLayout)

		// 在线扩容
		shr.POST("/pools/:name/disks", h.addDisk)

		// 热备盘管理
		shr.POST("/pools/:name/spare", h.addSpareDisk)
		shr.DELETE("/pools/:name/spare/:device", h.removeSpareDisk)

		// 硬盘管理
		shr.GET("/pools/:name/disks/:device/replace", h.replaceFailedDisk)

		// 冗余迁移
		shr.POST("/pools/:name/migrate", h.migrateRedundancy)
		shr.GET("/migrations", h.listMigrations)
		shr.GET("/migrations/:id", h.getMigration)

		// 硬盘注册
		shr.GET("/disks", h.listDisks)
		shr.POST("/disks", h.registerDisk)
		shr.GET("/disks/available", h.listAvailableDisks)
		shr.GET("/disks/:device", h.getDisk)
		shr.DELETE("/disks/:device", h.unregisterDisk)

		// 冗余计算
		shr.POST("/calculate-redundancy", h.calculateRedundancy)

		// 配置
		shr.GET("/config", h.getConfig)
		shr.PUT("/config", h.updateConfig)
	}
}

func (h *Handlers) listPools(c *gin.Context) {
	pools := h.manager.ListPools()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": pools})
}

func (h *Handlers) createPool(c *gin.Context) {
	var req struct {
		Name       string   `json:"name" binding:"required"`
		Redundancy string   `json:"redundancy" binding:"required"`
		Devices    []string `json:"devices" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.CreatePool(req.Name, req.Redundancy, req.Devices); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "存储池已创建"})
}

func (h *Handlers) getPool(c *gin.Context) {
	name := c.Param("name")
	pool, err := h.manager.GetPool(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": pool})
}

func (h *Handlers) deletePool(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeletePool(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "存储池已删除"})
}

func (h *Handlers) getPoolStatus(c *gin.Context) {
	name := c.Param("name")
	status, err := h.manager.GetPoolStatus(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
}

func (h *Handlers) optimizeLayout(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.OptimizeLayout(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "布局已优化"})
}

func (h *Handlers) addDisk(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Device string `json:"device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.AddDisk(name, req.Device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "硬盘已添加，扩容中"})
}

func (h *Handlers) addSpareDisk(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Device string `json:"device" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.AddSpareDisk(name, req.Device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "热备盘已添加"})
}

func (h *Handlers) removeSpareDisk(c *gin.Context) {
	name := c.Param("name")
	device := c.Param("device")
	if err := h.manager.RemoveSpareDisk(name, device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "热备盘已移除"})
}

func (h *Handlers) replaceFailedDisk(c *gin.Context) {
	name := c.Param("name")
	device := c.Param("device")
	if err := h.manager.ReplaceFailedDisk(name, device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "故障盘已替换"})
}

func (h *Handlers) migrateRedundancy(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		TargetRedundancy string `json:"target_redundancy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	task, err := h.manager.MigrateRedundancy(name, req.TargetRedundancy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

func (h *Handlers) listMigrations(c *gin.Context) {
	tasks := h.manager.ListMigrations()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks})
}

func (h *Handlers) getMigration(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetMigration(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

func (h *Handlers) listDisks(c *gin.Context) {
	disks := h.manager.ListDisks()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": disks})
}

func (h *Handlers) registerDisk(c *gin.Context) {
	var req struct {
		Device   string `json:"device" binding:"required"`
		Model    string `json:"model"`
		Serial   string `json:"serial"`
		Capacity int64  `json:"capacity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.RegisterDisk(req.Device, req.Model, req.Serial, req.Capacity); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "硬盘已注册"})
}

func (h *Handlers) listAvailableDisks(c *gin.Context) {
	disks := h.manager.ListAvailableDisks()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": disks})
}

func (h *Handlers) getDisk(c *gin.Context) {
	device := c.Param("device")
	disk, err := h.manager.GetDisk(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": disk})
}

func (h *Handlers) unregisterDisk(c *gin.Context) {
	device := c.Param("device")
	if err := h.manager.UnregisterDisk(device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "硬盘已注销"})
}

func (h *Handlers) calculateRedundancy(c *gin.Context) {
	var req struct {
		Devices    []string `json:"devices" binding:"required"`
		Redundancy string   `json:"redundancy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	tolerance, err := h.manager.CalculateRedundancy(req.Devices, req.Redundancy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"max_fault_tolerance": tolerance}})
}

func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg SHRConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.UpdateConfig(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}
