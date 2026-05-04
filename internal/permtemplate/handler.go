package permtemplate

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 权限模板HTTP处理器
type Handler struct {
	manager *TemplateManager
}

// NewHandler 创建处理器
func NewHandler(manager *TemplateManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	pt := rg.Group("/perm-templates")
	{
		pt.GET("", h.List)
		pt.GET("/:id", h.Get)
		pt.POST("", h.Create)
		pt.PUT("/:id", h.Update)
		pt.DELETE("/:id", h.Delete)
		pt.POST("/:id/apply", h.Apply)
		pt.GET("/applications", h.GetApplications)
	}
}

// List 列出模板
func (h *Handler) List(c *gin.Context) {
	category := c.Query("category")
	templates := h.manager.List(category)
	c.JSON(http.StatusOK, templates)
}

// Get 获取模板
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	tmpl, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, tmpl)
}

// Create 创建模板
func (h *Handler) Create(c *gin.Context) {
	var tmpl PermissionTemplate
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.Create(&tmpl); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tmpl)
}

// Update 更新模板
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var tmpl PermissionTemplate
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tmpl.ID = id
	if err := h.manager.Update(&tmpl); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tmpl)
}

// Delete 删除模板
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Delete(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// SingleApplyRequest 单用户应用请求
type SingleApplyRequest struct {
	UserID string `json:"userId" binding:"required"`
}

// Apply 应用模板
func (h *Handler) Apply(c *gin.Context) {
	id := c.Param("id")
	var req SingleApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.manager.Apply(id, req.UserID, "admin"); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "template applied", "userId": req.UserID})
}

// GetApplications 获取应用记录
func (h *Handler) GetApplications(c *gin.Context) {
	userID := c.Query("userId")
	apps := h.manager.GetApplications(userID)
	c.JSON(http.StatusOK, apps)
}
