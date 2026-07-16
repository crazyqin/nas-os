package hybridpool

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler API 处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建 API 处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	pool := r.Group("/hybridpool")
	{
		// 存储池管理
		pool.POST("/pools", h.CreatePool)
		pool.GET("/pools", h.ListPools)
		pool.GET("/pools/:id", h.GetPool)
		pool.DELETE("/pools/:id", h.DeletePool)

		// 存储层管理
		pool.POST("/pools/:id/tiers", h.AddTier)

		// 迁移策略
		pool.PUT("/pools/:id/migration-policy", h.SetMigrationPolicy)

		// 统计和监控
		pool.GET("/pools/:id/stats", h.GetPoolStats)
		pool.GET("/pools/:id/performance", h.GetPerformanceMetrics)

		// 数据迁移
		pool.POST("/pools/:id/migrate", h.MigrateData)
		pool.GET("/tasks", h.ListMigrationTasks)
		pool.GET("/tasks/:id", h.GetMigrationTask)

		// 容量预测
		pool.GET("/pools/:id/capacity-prediction", h.PredictCapacity)
	}
}

// CreatePool 创建存储池.
func (h *Handler) CreatePool(c *gin.Context) {
	var req CreatePoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	pool, err := h.manager.CreatePool(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "存储池创建成功",
		"data":    pool,
	})
}

// ListPools 列出存储池.
func (h *Handler) ListPools(c *gin.Context) {
	pools := h.manager.ListPools()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    pools,
	})
}

// GetPool 获取存储池.
func (h *Handler) GetPool(c *gin.Context) {
	poolID := c.Param("id")
	pool, err := h.manager.GetPool(poolID)
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
		"data":    pool,
	})
}

// DeletePool 删除存储池.
func (h *Handler) DeletePool(c *gin.Context) {
	poolID := c.Param("id")
	if err := h.manager.DeletePool(poolID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "存储池删除成功",
	})
}

// AddTier 添加存储层.
func (h *Handler) AddTier(c *gin.Context) {
	poolID := c.Param("id")
	var req struct {
		TierType TierType   `json:"tierType" binding:"required"`
		Config   TierConfig `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddTier(poolID, req.TierType, &req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "存储层添加成功",
	})
}

// SetMigrationPolicy 设置迁移策略.
func (h *Handler) SetMigrationPolicy(c *gin.Context) {
	poolID := c.Param("id")
	var policy MigrationPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.manager.SetMigrationPolicy(poolID, &policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "迁移策略设置成功",
	})
}

// GetPoolStats 获取存储池统计.
func (h *Handler) GetPoolStats(c *gin.Context) {
	poolID := c.Param("id")
	stats, err := h.manager.GetPoolStats(poolID)
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
		"data":    stats,
	})
}

// GetPerformanceMetrics 获取性能指标.
func (h *Handler) GetPerformanceMetrics(c *gin.Context) {
	poolID := c.Param("id")
	pool, err := h.manager.GetPool(poolID)
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
		"data":    pool.Performance,
	})
}

// MigrateData 触发数据迁移.
func (h *Handler) MigrateData(c *gin.Context) {
	poolID := c.Param("id")
	var req DataMigrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	task, err := h.manager.MigrateData(poolID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"code":    0,
		"message": "迁移任务已创建",
		"data":    task,
	})
}

// ListMigrationTasks 列出迁移任务.
func (h *Handler) ListMigrationTasks(c *gin.Context) {
	poolID := c.Query("poolId")
	tasks := h.manager.ListMigrationTasks(poolID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

// GetMigrationTask 获取迁移任务.
func (h *Handler) GetMigrationTask(c *gin.Context) {
	taskID := c.Param("id")
	task, err := h.manager.GetMigrationTask(taskID)
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

// PredictCapacity 容量预测.
func (h *Handler) PredictCapacity(c *gin.Context) {
	poolID := c.Param("id")
	prediction, err := h.manager.PredictCapacity(poolID)
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
		"data":    prediction,
	})
}
