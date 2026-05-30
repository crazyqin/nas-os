package smartdatatier

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 智能数据分层HTTP处理器
type Handler struct {
	manager *TierManager
}

// NewHandler 创建处理器
func NewHandler(manager *TierManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/smartdatatier")
	{
		group.GET("/files", h.ListFiles)
		group.GET("/files/:id", h.GetFile)
		group.POST("/files", h.RegisterFile)
		group.POST("/files/:id/access", h.RecordAccess)
		group.GET("/files/:id/recommend", h.RecommendTier)
		group.GET("/migration-plan", h.GetMigrationPlan)
		group.POST("/migrate/:taskId", h.ExecuteMigration)
		group.GET("/stats", h.GetStats)
	}
}

// RegisterFileRequest 注册文件请求
type RegisterFileRequest struct {
	ID   string `json:"id" binding:"required"`
	Path string `json:"path" binding:"required"`
	Size int64  `json:"size"`
}

// ListFiles 列出文件
func (h *Handler) ListFiles(c *gin.Context) {
	tier := TierLevel(-1)
	if t := c.Query("tier"); t != "" {
		switch t {
		case "hot":
			tier = TierHot
		case "warm":
			tier = TierWarm
		case "cold":
			tier = TierCold
		case "archive":
			tier = TierArchive
		}
	}

	files := h.manager.ListFiles(tier)
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// GetFile 获取文件详情
func (h *Handler) GetFile(c *gin.Context) {
	id := c.Param("id")
	file, ok := h.manager.GetFile(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.JSON(http.StatusOK, file)
}

// RegisterFile 注册文件
func (h *Handler) RegisterFile(c *gin.Context) {
	var req RegisterFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	file := &DataFile{
		ID:   req.ID,
		Path: req.Path,
		Size: req.Size,
	}

	h.manager.RegisterFile(file)
	c.JSON(http.StatusCreated, gin.H{"message": "file registered", "id": req.ID})
}

// RecordAccess 记录访问
func (h *Handler) RecordAccess(c *gin.Context) {
	id := c.Param("id")
	h.manager.RecordAccess(id)
	c.JSON(http.StatusOK, gin.H{"message": "access recorded"})
}

// RecommendTier 推荐层级
func (h *Handler) RecommendTier(c *gin.Context) {
	id := c.Param("id")
	tier := h.manager.RecommendTier(id)

	tierName := "unknown"
	switch tier {
	case TierHot:
		tierName = "hot"
	case TierWarm:
		tierName = "warm"
	case TierCold:
		tierName = "cold"
	case TierArchive:
		tierName = "archive"
	}

	c.JSON(http.StatusOK, gin.H{
		"fileId":     id,
		"recommended": tierName,
	})
}

// GetMigrationPlan 获取迁移计划
func (h *Handler) GetMigrationPlan(c *gin.Context) {
	plan := h.manager.GetMigrationPlan()
	c.JSON(http.StatusOK, gin.H{
		"plan":  plan,
		"total": len(plan),
	})
}

// ExecuteMigration 执行迁移
func (h *Handler) ExecuteMigration(c *gin.Context) {
	taskID := c.Param("taskId")
	if err := h.manager.ExecuteMigration(taskID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "migration completed"})
}

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
