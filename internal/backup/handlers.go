package backup

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 备份 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	backup := r.Group("/backup")
	{
		// 备份配置
		backup.GET("/configs", h.listConfigs)
		backup.POST("/configs", h.createConfig)
		backup.GET("/configs/:id", h.getConfig)
		backup.PUT("/configs/:id", h.updateConfig)
		backup.DELETE("/configs/:id", h.deleteConfig)
		backup.POST("/configs/:id/enable", h.enableConfig)

		// 备份操作
		backup.POST("/run/:id", h.runBackup)
		backup.POST("/restore", h.restore)

		// 任务管理
		backup.GET("/tasks", h.listTasks)
		backup.GET("/tasks/:id", h.getTask)
		backup.DELETE("/tasks/:id", h.cancelTask)

		// 历史记录
		backup.GET("/history/:configId", h.getHistory)
	}
}

// ========== 配置管理 ==========

func (h *Handlers) listConfigs(c *gin.Context) {
	configs := h.manager.ListConfigs()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

func (h *Handlers) getConfig(c *gin.Context) {
	id := c.Param("id")
	config, err := h.manager.GetConfig(id)
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
		"data":    config,
	})
}

func (h *Handlers) createConfig(c *gin.Context) {
	var config BackupConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.CreateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "备份配置创建成功",
		"data":    config,
	})
}

func (h *Handlers) updateConfig(c *gin.Context) {
	id := c.Param("id")

	var config BackupConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(id, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "备份配置更新成功",
	})
}

func (h *Handlers) deleteConfig(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteConfig(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "备份配置已删除",
	})
}

func (h *Handlers) enableConfig(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.EnableConfig(id, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "状态已更新",
	})
}

// ========== 备份操作 ==========

func (h *Handlers) runBackup(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.RunBackup(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "备份任务已启动",
		"data":    task,
	})
}

func (h *Handlers) restore(c *gin.Context) {
	var options RestoreOptions
	if err := c.ShouldBindJSON(&options); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	task, err := h.manager.Restore(options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "恢复任务已启动",
		"data":    task,
	})
}

// ========== 任务管理 ==========

func (h *Handlers) listTasks(c *gin.Context) {
	tasks := h.manager.ListTasks()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetTask(id)
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

func (h *Handlers) cancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "任务已取消",
	})
}

// ========== 历史记录 ==========

func (h *Handlers) getHistory(c *gin.Context) {
	configId := c.Param("configId")

	history, err := h.manager.GetHistory(configId)
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
		"data":    history,
	})
}