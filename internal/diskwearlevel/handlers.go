// Package diskwearlevel - HTTP API 处理器
package diskwearlevel

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 磁盘磨损均衡 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	dwl := rg.Group("/disk-wear")
	{
		// 磁盘管理
		dwl.GET("/disks", h.listDisks)
		dwl.GET("/disks/:id", h.getDisk)
		dwl.POST("/disks", h.registerDisk)

		// SMART 数据
		dwl.POST("/smart", h.updateSMART)
		dwl.GET("/smart/:diskId", h.getSMART)

		// 策略管理
		dwl.GET("/policies", h.listPolicies)
		dwl.POST("/policies", h.createPolicy)

		// 均衡计划
		dwl.POST("/rebalance/generate", h.generatePlan)
		dwl.GET("/rebalance/plans", h.listPlans)

		// 统计
		dwl.GET("/stats", h.getStats)
	}
}

func (h *Handlers) listDisks(c *gin.Context) {
	disks := h.manager.ListDisks()
	c.JSON(http.StatusOK, gin.H{"disks": disks})
}

func (h *Handlers) getDisk(c *gin.Context) {
	disk, err := h.manager.GetDisk(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, disk)
}

func (h *Handlers) registerDisk(c *gin.Context) {
	var disk DiskInfo
	if err := c.ShouldBindJSON(&disk); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.RegisterDisk(&disk); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, disk)
}

func (h *Handlers) updateSMART(c *gin.Context) {
	var data SMARTData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.UpdateSMARTData(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handlers) getSMART(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"diskId": c.Param("diskId"), "message": "use /api/v1/disks/:id for current data"})
}

func (h *Handlers) listPolicies(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"policies": h.manager.policies})
}

func (h *Handlers) createPolicy(c *gin.Context) {
	var policy WearPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.CreatePolicy(&policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, policy)
}

func (h *Handlers) generatePlan(c *gin.Context) {
	plan := h.manager.GenerateRebalancePlan()
	if plan == nil {
		c.JSON(http.StatusOK, gin.H{"message": "no rebalance needed"})
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *Handlers) listPlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"plans": h.manager.plans})
}

func (h *Handlers) getStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.manager.GetStats())
}
