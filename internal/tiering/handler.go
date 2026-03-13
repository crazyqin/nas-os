// Package tiering API 处理器
package tiering

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建 API 处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	tiering := r.Group("/tiering")
	{
		// 存储层配置
		tiering.GET("/tiers", h.ListTiers)
		tiering.GET("/tiers/:type", h.GetTier)
		tiering.PUT("/tiers/:type", h.UpdateTier)

		// 分层策略
		tiering.GET("/policies", h.ListPolicies)
		tiering.POST("/policies", h.CreatePolicy)
		tiering.GET("/policies/:id", h.GetPolicy)
		tiering.PUT("/policies/:id", h.UpdatePolicy)
		tiering.DELETE("/policies/:id", h.DeletePolicy)
		tiering.POST("/policies/:id/execute", h.ExecutePolicy)

		// 迁移操作
		tiering.POST("/migrate", h.Migrate)

		// 任务管理
		tiering.GET("/tasks", h.ListTasks)
		tiering.GET("/tasks/:id", h.GetTask)

		// 状态查询
		tiering.GET("/status", h.GetStatus)
	}
}

// ListTiers 列出所有存储层
func (h *Handler) ListTiers(c *gin.Context) {
	tiers := h.manager.ListTiers()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tiers,
	})
}

// GetTier 获取存储层配置
func (h *Handler) GetTier(c *gin.Context) {
	tierType := TierType(c.Param("type"))
	tier, err := h.manager.GetTier(tierType)
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
		"data":    tier,
	})
}

// UpdateTier 更新存储层配置
func (h *Handler) UpdateTier(c *gin.Context) {
	tierType := TierType(c.Param("type"))
	var config TierConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	config.Type = tierType
	if err := h.manager.UpdateTier(tierType, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "存储层配置已更新",
	})
}

// ListPolicies 列出所有策略
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policies,
	})
}

// CreatePolicy 创建策略
func (h *Handler) CreatePolicy(c *gin.Context) {
	var policy Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	created, err := h.manager.CreatePolicy(policy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略已创建",
		"data":    created,
	})
}

// GetPolicy 获取策略
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
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
		"data":    policy,
	})
}

// UpdatePolicy 更新策略
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var policy Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.UpdatePolicy(id, policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略已更新",
	})
}

// DeletePolicy 删除策略
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略已删除",
	})
}

// ExecutePolicy 执行策略
func (h *Handler) ExecutePolicy(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.ExecutePolicy(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略执行中",
		"data":    task,
	})
}

// Migrate 手动迁移
func (h *Handler) Migrate(c *gin.Context) {
	var req MigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	task, err := h.manager.Migrate(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "迁移任务已创建",
		"data":    task,
	})
}

// ListTasks 列出迁移任务
func (h *Handler) ListTasks(c *gin.Context) {
	tasks := h.manager.ListTasks(100)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

// GetTask 获取任务详情
func (h *Handler) GetTask(c *gin.Context) {
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

// GetStatus 获取分层状态
func (h *Handler) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}