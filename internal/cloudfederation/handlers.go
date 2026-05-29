// Package cloudfederation handlers - HTTP API
package cloudfederation

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/cloudfederation")
	{
		// 云提供商管理
		g.GET("/providers", h.ListProviders)
		g.POST("/providers", h.RegisterProvider)
		g.GET("/providers/:id", h.GetProvider)
		g.PUT("/providers/:id", h.UpdateProvider)
		g.DELETE("/providers/:id", h.DeleteProvider)
		g.POST("/providers/:id/health", h.CheckProviderHealth)

		// 命名空间管理
		g.GET("/namespaces", h.ListNamespaces)
		g.POST("/namespaces", h.CreateNamespace)
		g.GET("/namespaces/:id", h.GetNamespace)
		g.DELETE("/namespaces/:id", h.DeleteNamespace)

		// 对象管理
		g.GET("/namespaces/:id/objects", h.ListObjects)
		g.POST("/namespaces/:id/objects", h.PlaceObject)
		g.GET("/namespaces/:id/objects/:key", h.GetObject)
		g.DELETE("/namespaces/:id/objects/:key", h.DeleteObject)

		// 同步任务
		g.GET("/syncs", h.ListSyncTasks)
		g.POST("/syncs", h.CreateSyncTask)
		g.GET("/syncs/:id", h.GetSyncTask)

		// 迁移任务
		g.GET("/migrations", h.ListMigrationTasks)
		g.POST("/migrations", h.CreateMigrationTask)
		g.GET("/migrations/:id", h.GetMigrationTask)
		g.POST("/migrations/:id/cancel", h.CancelMigrationTask)

		// 成本分析
		g.GET("/costs", h.AnalyzeCosts)

		// 统计
		g.GET("/stats", h.GetFederationStats)
	}
}

// ListProviders 列出云提供商
func (h *Handlers) ListProviders(c *gin.Context) {
	providerType := CloudProvider(c.Query("type"))
	providers := h.mgr.ListProviders(providerType)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": providers, "total": len(providers)})
}

// RegisterProvider 注册云提供商
func (h *Handlers) RegisterProvider(c *gin.Context) {
	var cfg CloudProviderConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.RegisterProvider(&cfg); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

// GetProvider 获取云提供商
func (h *Handlers) GetProvider(c *gin.Context) {
	provider, err := h.mgr.GetProvider(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": provider})
}

// UpdateProvider 更新云提供商
func (h *Handlers) UpdateProvider(c *gin.Context) {
	var cfg CloudProviderConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.UpdateProvider(c.Param("id"), &cfg); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "updated"})
}

// DeleteProvider 删除云提供商
func (h *Handlers) DeleteProvider(c *gin.Context) {
	if err := h.mgr.DeleteProvider(c.Param("id")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// CheckProviderHealth 检查提供商健康状态
func (h *Handlers) CheckProviderHealth(c *gin.Context) {
	status, err := h.mgr.CheckProviderHealth(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"status": status}})
}

// ListNamespaces 列出命名空间
func (h *Handlers) ListNamespaces(c *gin.Context) {
	namespaces := h.mgr.ListNamespaces()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": namespaces, "total": len(namespaces)})
}

// CreateNamespace 创建命名空间
func (h *Handlers) CreateNamespace(c *gin.Context) {
	var ns Namespace
	if err := c.ShouldBindJSON(&ns); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateNamespace(&ns); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ns})
}

// GetNamespace 获取命名空间
func (h *Handlers) GetNamespace(c *gin.Context) {
	ns, err := h.mgr.GetNamespace(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ns})
}

// DeleteNamespace 删除命名空间
func (h *Handlers) DeleteNamespace(c *gin.Context) {
	if err := h.mgr.DeleteNamespace(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// ListObjects 列出对象
func (h *Handlers) ListObjects(c *gin.Context) {
	prefix := c.Query("prefix")
	limit := 100
	objects, err := h.mgr.ListObjects(c.Param("id"), prefix, limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": objects, "total": len(objects)})
}

// PlaceObject 放置对象
func (h *Handlers) PlaceObject(c *gin.Context) {
	var obj StorageObject
	if err := c.ShouldBindJSON(&obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	providerID, err := h.mgr.PlaceObject(c.Param("id"), &obj)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": obj, "provider": providerID})
}

// GetObject 获取对象
func (h *Handlers) GetObject(c *gin.Context) {
	obj, err := h.mgr.GetObject(c.Param("id"), c.Param("key"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": obj})
}

// DeleteObject 删除对象
func (h *Handlers) DeleteObject(c *gin.Context) {
	if err := h.mgr.DeleteObject(c.Param("id"), c.Param("key")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// ListSyncTasks 列出同步任务
func (h *Handlers) ListSyncTasks(c *gin.Context) {
	status := SyncStatus(c.Query("status"))
	tasks := h.mgr.ListSyncTasks(status)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks, "total": len(tasks)})
}

// CreateSyncTask 创建同步任务
func (h *Handlers) CreateSyncTask(c *gin.Context) {
	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateSyncTask(&task); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// GetSyncTask 获取同步任务
func (h *Handlers) GetSyncTask(c *gin.Context) {
	task, err := h.mgr.GetSyncTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// ListMigrationTasks 列出迁移任务
func (h *Handlers) ListMigrationTasks(c *gin.Context) {
	status := MigrationStatus(c.Query("status"))
	tasks := h.mgr.ListMigrationTasks(status)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks, "total": len(tasks)})
}

// CreateMigrationTask 创建迁移任务
func (h *Handlers) CreateMigrationTask(c *gin.Context) {
	var task MigrationTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateMigrationTask(&task); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// GetMigrationTask 获取迁移任务
func (h *Handlers) GetMigrationTask(c *gin.Context) {
	task, err := h.mgr.GetMigrationTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// CancelMigrationTask 取消迁移任务
func (h *Handlers) CancelMigrationTask(c *gin.Context) {
	if err := h.mgr.CancelMigrationTask(c.Param("id")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "cancelled"})
}

// AnalyzeCosts 成本分析
func (h *Handlers) AnalyzeCosts(c *gin.Context) {
	period := c.DefaultQuery("period", "monthly")
	analysis, err := h.mgr.AnalyzeCosts(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": analysis})
}

// GetFederationStats 获取联邦统计
func (h *Handlers) GetFederationStats(c *gin.Context) {
	stats := h.mgr.GetFederationStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
