package appcenter

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 应用商店HTTP处理器.
type Handler struct {
	store  *AppStore
	logger *zap.Logger
}

// NewHandler 创建应用商店处理器.
func NewHandler(store *AppStore, logger *zap.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	apps := r.Group("/appcenter")
	{
		// 应用管理
		apps.GET("/apps", h.ListApps)
		apps.POST("/apps", h.RegisterApp)
		apps.GET("/apps/:id", h.GetApp)
		apps.PUT("/apps/:id", h.UpdateApp)
		apps.DELETE("/apps/:id", h.RemoveApp)

		// 应用操作
		apps.POST("/apps/:id/install", h.InstallApp)
		apps.POST("/apps/:id/uninstall", h.UninstallApp)
		apps.POST("/apps/:id/start", h.StartApp)
		apps.POST("/apps/:id/stop", h.StopApp)
		apps.POST("/apps/:id/enable", h.EnableApp)
		apps.POST("/apps/:id/disable", h.DisableApp)
		apps.POST("/apps/:id/update", h.UpdateAppVersion)
		apps.GET("/apps/:id/lifecycle-plan", h.GetLifecyclePlan)

		// 配置
		apps.PUT("/apps/:id/config", h.SetAppConfig)

		// 分类
		apps.GET("/categories", h.GetCategories)

		// 评价
		apps.GET("/apps/:id/reviews", h.GetReviews)
		apps.POST("/apps/:id/reviews", h.AddReview)

		// 搜索
		apps.GET("/search", h.SearchApps)

		// 已安装
		apps.GET("/installed", h.GetInstalledApps)

		// 更新检查
		apps.GET("/updates", h.CheckUpdates)

		// 日志
		apps.GET("/logs", h.GetInstallLogs)
	}
}

func (h *Handler) ListApps(c *gin.Context) {
	category := c.Query("category")
	installedOnly := c.Query("installed") == "true"
	apps := h.store.ListApps(c.Request.Context(), category, installedOnly)
	c.JSON(http.StatusOK, gin.H{"apps": apps, "total": len(apps)})
}

func (h *Handler) RegisterApp(c *gin.Context) {
	var app App
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.RegisterApp(c.Request.Context(), &app); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, app)
}

func (h *Handler) GetApp(c *gin.Context) {
	id := c.Param("id")
	app, err := h.store.GetApp(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

func (h *Handler) UpdateApp(c *gin.Context) {
	id := c.Param("id")
	var app App
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	app.ID = id
	if err := h.store.UpdateApp(c.Request.Context(), &app); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, app)
}

func (h *Handler) RemoveApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.RemoveApp(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app removed"})
}

func (h *Handler) InstallApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.InstallApp(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app installed"})
}

func (h *Handler) UninstallApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.UninstallApp(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app uninstalled"})
}

func (h *Handler) StartApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.StartApp(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app started"})
}

func (h *Handler) StopApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.StopApp(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app stopped"})
}

func (h *Handler) EnableApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.EnableApp(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app enabled"})
}

func (h *Handler) DisableApp(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DisableApp(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app disabled"})
}

func (h *Handler) UpdateAppVersion(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.UpdateAppVersion(c.Request.Context(), id, req.Version); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "app updated"})
}

func (h *Handler) GetLifecyclePlan(c *gin.Context) {
	id := c.Param("id")
	action := LifecycleAction(c.DefaultQuery("action", string(LifecycleActionInstall)))
	plan, err := h.store.PlanLifecycle(c.Request.Context(), id, action)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *Handler) SetAppConfig(c *gin.Context) {
	id := c.Param("id")
	var config map[string]string
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.SetAppConfig(c.Request.Context(), id, config); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

func (h *Handler) GetCategories(c *gin.Context) {
	categories := h.store.GetCategories(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func (h *Handler) GetReviews(c *gin.Context) {
	id := c.Param("id")
	reviews := h.store.GetReviews(c.Request.Context(), id)
	c.JSON(http.StatusOK, gin.H{"reviews": reviews, "total": len(reviews)})
}

func (h *Handler) AddReview(c *gin.Context) {
	id := c.Param("id")
	var review AppReview
	if err := c.ShouldBindJSON(&review); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	review.AppID = id
	if err := h.store.AddReview(c.Request.Context(), &review); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, review)
}

func (h *Handler) SearchApps(c *gin.Context) {
	query := c.Query("q")
	apps := h.store.SearchApps(c.Request.Context(), query)
	c.JSON(http.StatusOK, gin.H{"apps": apps, "total": len(apps)})
}

func (h *Handler) GetInstalledApps(c *gin.Context) {
	apps := h.store.GetInstalledApps(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"apps": apps, "total": len(apps)})
}

func (h *Handler) CheckUpdates(c *gin.Context) {
	updates := h.store.CheckUpdates(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"updates": updates, "total": len(updates)})
}

func (h *Handler) GetInstallLogs(c *gin.Context) {
	appID := c.Query("app_id")
	logs := h.store.GetInstallLogs(c.Request.Context(), appID)
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": len(logs)})
}
