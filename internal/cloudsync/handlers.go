package cloudsync

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 云同步 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cloudsync := r.Group("/cloudsync")
	{
		// 提供商管理
		cloudsync.POST("/providers", h.createProvider)
		cloudsync.GET("/providers", h.listProviders)
		cloudsync.GET("/providers/:id", h.getProvider)
		cloudsync.PUT("/providers/:id", h.updateProvider)
		cloudsync.DELETE("/providers/:id", h.deleteProvider)
		cloudsync.POST("/providers/:id/test", h.testProvider)

		// 同步任务管理
		cloudsync.POST("/tasks", h.createSyncTask)
		cloudsync.GET("/tasks", h.listSyncTasks)
		cloudsync.GET("/tasks/:id", h.getSyncTask)
		cloudsync.PUT("/tasks/:id", h.updateSyncTask)
		cloudsync.DELETE("/tasks/:id", h.deleteSyncTask)

		// 同步操作
		cloudsync.POST("/tasks/:id/run", h.runSyncTask)
		cloudsync.POST("/tasks/:id/pause", h.pauseSyncTask)
		cloudsync.POST("/tasks/:id/resume", h.resumeSyncTask)
		cloudsync.POST("/tasks/:id/cancel", h.cancelSyncTask)
		cloudsync.GET("/tasks/:id/status", h.getSyncStatus)

		// 全局状态
		cloudsync.GET("/statuses", h.getAllStatuses)
		cloudsync.GET("/stats", h.getStats)
		cloudsync.GET("/providers-info", h.getProvidersInfo)
	}
}

// ==================== 提供商管理 ====================

func (h *Handlers) createProvider(c *gin.Context) {
	var config ProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	provider, err := h.manager.CreateProvider(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "提供商创建成功",
		"data":    provider,
	})
}

func (h *Handlers) getProvider(c *gin.Context) {
	id := c.Param("id")

	provider, err := h.manager.GetProvider(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    provider,
	})
}

func (h *Handlers) listProviders(c *gin.Context) {
	providers := h.manager.ListProviders()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    providers,
	})
}

func (h *Handlers) updateProvider(c *gin.Context) {
	id := c.Param("id")

	var config ProviderConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.UpdateProvider(id, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "提供商更新成功",
	})
}

func (h *Handlers) deleteProvider(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteProvider(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "提供商已删除",
	})
}

func (h *Handlers) testProvider(c *gin.Context) {
	id := c.Param("id")

	result, err := h.manager.TestProvider(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// ==================== 同步任务管理 ====================

func (h *Handlers) createSyncTask(c *gin.Context) {
	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	createdTask, err := h.manager.CreateSyncTask(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务创建成功",
		"data":    createdTask,
	})
}

func (h *Handlers) getSyncTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetSyncTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

func (h *Handlers) listSyncTasks(c *gin.Context) {
	tasks := h.manager.ListSyncTasks()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

func (h *Handlers) updateSyncTask(c *gin.Context) {
	id := c.Param("id")

	var task SyncTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.UpdateSyncTask(id, task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务更新成功",
	})
}

func (h *Handlers) deleteSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已删除",
	})
}

// ==================== 同步操作 ====================

func (h *Handlers) runSyncTask(c *gin.Context) {
	id := c.Param("id")

	status, err := h.manager.RunSyncTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已启动",
		"data":    status,
	})
}

func (h *Handlers) pauseSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.PauseSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已暂停",
	})
}

func (h *Handlers) resumeSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResumeSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已恢复",
	})
}

func (h *Handlers) cancelSyncTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelSyncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步任务已取消",
	})
}

func (h *Handlers) getSyncStatus(c *gin.Context) {
	id := c.Param("id")

	status, err := h.manager.GetSyncStatus(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// ==================== 全局状态 ====================

func (h *Handlers) getAllStatuses(c *gin.Context) {
	statuses := h.manager.GetAllSyncStatuses()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    statuses,
	})
}

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

func (h *Handlers) getProvidersInfo(c *gin.Context) {
	providers := SupportedProviders()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    providers,
	})
}