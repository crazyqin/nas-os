package smartmigrate

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能迁移 HTTP 处理器
type Handlers struct {
	mgr *SmartMigrateManager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *SmartMigrateManager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	migrate := api.Group("/migrate")
	{
		migrate.POST("/tasks", h.CreateTask)
		migrate.GET("/tasks", h.ListTasks)
		migrate.GET("/tasks/:id", h.GetTask)
		migrate.POST("/tasks/:id/start", h.StartTask)
		migrate.POST("/tasks/:id/pause", h.PauseTask)
		migrate.POST("/tasks/:id/cancel", h.CancelTask)
		migrate.GET("/history", h.GetHistory)
	}
}

func (h *Handlers) CreateTask(c *gin.Context) {
	var req struct {
		Name       string        `json:"name" binding:"required"`
		SourcePath string        `json:"source_path" binding:"required"`
		DestPath   string        `json:"dest_path" binding:"required"`
		Type       MigrateType   `json:"type"`
		Options    *MigrateOptions `json:"options"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Type == "" {
		req.Type = TypeCopy
	}
	task, err := h.mgr.CreateTask(req.Name, req.SourcePath, req.DestPath, req.Type, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) ListTasks(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.ListTasks())
}

func (h *Handlers) GetTask(c *gin.Context) {
	task, err := h.mgr.GetTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handlers) StartTask(c *gin.Context) {
	if err := h.mgr.StartTask(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (h *Handlers) PauseTask(c *gin.Context) {
	if err := h.mgr.PauseTask(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

func (h *Handlers) CancelTask(c *gin.Context) {
	if err := h.mgr.CancelTask(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

func (h *Handlers) GetHistory(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetHistory())
}
