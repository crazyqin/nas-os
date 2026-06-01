// Package backupvault 提供 REST API 处理器
package backupvault

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 备份保险库 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	vault := r.Group("/vault")
	{
		// 保险库管理
		vault.GET("", h.listVaults)
		vault.GET("/:id", h.getVault)

		// 备份任务
		vault.POST("/jobs", h.createJob)
		vault.GET("/jobs", h.listJobs)
		vault.GET("/jobs/:id", h.getJob)

		// 去重统计
		vault.GET("/dedup/:vault_id", h.getDedupStats)

		// 恢复演练
		vault.POST("/restore-test", h.runRestoreTest)
		vault.GET("/restore-test", h.listTests)
		vault.GET("/restore-test/:id", h.getTest)

		// SLA 策略
		vault.POST("/sla", h.setSLA)
		vault.GET("/sla", h.listSLAs)
		vault.GET("/sla/:id", h.getSLA)

		// 配置
		vault.GET("/config", h.getConfig)
		vault.PUT("/config", h.updateConfig)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listVaults 列出保险库
func (h *Handlers) listVaults(c *gin.Context) {
	vaults := h.manager.ListVaults()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    vaults,
	})
}

// getVault 获取保险库
func (h *Handlers) getVault(c *gin.Context) {
	id := c.Param("id")
	vault, err := h.manager.GetVault(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    vault,
	})
}

// createJob 创建备份任务
func (h *Handlers) createJob(c *gin.Context) {
	var job BackupJob
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CreateJob(&job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "job created",
		Data:    result,
	})
}

// listJobs 列出备份任务
func (h *Handlers) listJobs(c *gin.Context) {
	jobs := h.manager.ListJobs()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    jobs,
	})
}

// getJob 获取备份任务
func (h *Handlers) getJob(c *gin.Context) {
	id := c.Param("id")
	job, err := h.manager.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    job,
	})
}

// getDedupStats 获取去重统计
func (h *Handlers) getDedupStats(c *gin.Context) {
	vaultID := c.Param("vault_id")
	stats, err := h.manager.GetDedupStats(vaultID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// runRestoreTest 运行恢复演练
func (h *Handlers) runRestoreTest(c *gin.Context) {
	var test RestoreTest
	if err := c.ShouldBindJSON(&test); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.RunRestoreTest(&test)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "restore test started",
		Data:    result,
	})
}

// listTests 列出恢复演练
func (h *Handlers) listTests(c *gin.Context) {
	tests := h.manager.ListTests()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tests,
	})
}

// getTest 获取恢复演练
func (h *Handlers) getTest(c *gin.Context) {
	id := c.Param("id")
	test, err := h.manager.GetTest(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    test,
	})
}

// setSLA 设置 SLA 策略
func (h *Handlers) setSLA(c *gin.Context) {
	var policy SLAPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.SetSLA(&policy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "SLA policy set",
		Data:    result,
	})
}

// listSLAs 列出 SLA 策略
func (h *Handlers) listSLAs(c *gin.Context) {
	policies := h.manager.ListSLAs()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policies,
	})
}

// getSLA 获取 SLA 策略
func (h *Handlers) getSLA(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetSLA(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg BackupVaultConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}
