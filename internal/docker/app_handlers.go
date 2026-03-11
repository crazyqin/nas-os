package docker

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// AppHandlers 应用商店处理器
type AppHandlers struct {
	store *AppStore
	mu    sync.RWMutex
}

// NewAppHandlers 创建应用商店处理器
func NewAppHandlers(store *AppStore) *AppHandlers {
	return &AppHandlers{
		store: store,
	}
}

// RegisterRoutes 注册路由
func (h *AppHandlers) RegisterRoutes(r *gin.RouterGroup) {
	apps := r.Group("/apps")
	{
		// 应用目录
		apps.GET("/catalog", h.listCatalog)
		apps.GET("/catalog/:id", h.getTemplate)

		// 已安装应用
		apps.GET("/installed", h.listInstalled)
		apps.GET("/installed/:id", h.getInstalled)
		apps.GET("/installed/:id/stats", h.getAppStats)

		// 应用操作
		apps.POST("/install/:id", h.installApp)
		apps.DELETE("/installed/:id", h.uninstallApp)
		apps.POST("/installed/:id/start", h.startApp)
		apps.POST("/installed/:id/stop", h.stopApp)
		apps.POST("/installed/:id/restart", h.restartApp)
		apps.POST("/installed/:id/update", h.updateApp)
	}
}

// listCatalog 列出应用目录
func (h *AppHandlers) listCatalog(c *gin.Context) {
	category := c.Query("category")

	templates := h.store.ListTemplates()
	if category != "" {
		filtered := make([]*AppTemplate, 0)
		for _, t := range templates {
			if t.Category == category {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
	}

	// 按类别分组
	categories := make(map[string][]*AppTemplate)
	for _, t := range templates {
		categories[t.Category] = append(categories[t.Category], t)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"templates":  templates,
			"categories": categories,
		},
	})
}

// getTemplate 获取模板详情
func (h *AppHandlers) getTemplate(c *gin.Context) {
	id := c.Param("id")

	template := h.store.GetTemplate(id)
	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "模板不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    template,
	})
}

// listInstalled 列出已安装应用
func (h *AppHandlers) listInstalled(c *gin.Context) {
	apps := h.store.ListInstalled()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    apps,
	})
}

// getInstalled 获取已安装应用详情
func (h *AppHandlers) getInstalled(c *gin.Context) {
	id := c.Param("id")

	app := h.store.GetInstalled(id)
	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "应用未安装",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    app,
	})
}

// installApp 安装应用
func (h *AppHandlers) installApp(c *gin.Context) {
	templateID := c.Param("id")

	var req struct {
		Ports       map[string]interface{} `json:"ports"`
		Volumes     map[string]interface{} `json:"volumes"`
		Environment map[string]interface{} `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空请求，使用默认配置
	}

	// 构建配置
	config := make(map[string]interface{})
	for k, v := range req.Ports {
		config[k] = v
	}
	for k, v := range req.Volumes {
		config[k] = v
	}
	for k, v := range req.Environment {
		config[k] = v
	}

	app, err := h.store.InstallApp(templateID, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用安装成功",
		"data":    app,
	})
}

// uninstallApp 卸载应用
func (h *AppHandlers) uninstallApp(c *gin.Context) {
	id := c.Param("id")
	removeData := c.Query("removeData") == "true"

	if err := h.store.UninstallApp(id, removeData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用已卸载",
	})
}

// startApp 启动应用
func (h *AppHandlers) startApp(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.StartApp(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用已启动",
	})
}

// stopApp 停止应用
func (h *AppHandlers) stopApp(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.StopApp(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用已停止",
	})
}

// restartApp 重启应用
func (h *AppHandlers) restartApp(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.RestartApp(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用已重启",
	})
}

// updateApp 更新应用
func (h *AppHandlers) updateApp(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.UpdateApp(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用已更新",
	})
}

// getAppStats 获取应用统计
func (h *AppHandlers) getAppStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.store.GetAppStats(id)
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
		"data":    stats,
	})
}