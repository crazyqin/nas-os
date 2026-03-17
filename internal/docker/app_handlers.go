package docker

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AppHandlers 应用商店处理器
type AppHandlers struct {
	store             *AppStore
	ratingManager     *RatingManager
	discovery         *AppDiscovery
	customTemplateMgr *CustomTemplateManager
	versionManager    *VersionManager
	// mu                 sync.RWMutex - 保留用于未来需要并发控制的场景
}

// NewAppHandlers 创建应用商店处理器
func NewAppHandlers(store *AppStore) *AppHandlers {
	return &AppHandlers{
		store: store,
	}
}

// SetRatingManager 设置评分管理器
func (h *AppHandlers) SetRatingManager(rm *RatingManager) {
	h.ratingManager = rm
}

// SetDiscovery 设置应用发现器
func (h *AppHandlers) SetDiscovery(ad *AppDiscovery) {
	h.discovery = ad
}

// SetCustomTemplateManager 设置自定义模板管理器
func (h *AppHandlers) SetCustomTemplateManager(ctm *CustomTemplateManager) {
	h.customTemplateMgr = ctm
}

// SetVersionManager 设置版本管理器
func (h *AppHandlers) SetVersionManager(vm *VersionManager) {
	h.versionManager = vm
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

		// === 评分和评论 ===
		apps.GET("/ratings/:templateId", h.getRatings)
		apps.GET("/ratings/:templateId/stats", h.getRatingStats)
		apps.POST("/ratings/:templateId", h.addRating)
		apps.DELETE("/ratings/:templateId/:ratingId", h.deleteRating)
		apps.POST("/ratings/:templateId/:ratingId/helpful", h.markHelpful)

		// === 应用发现 ===
		apps.GET("/discover", h.getDiscovered)
		apps.POST("/discover/refresh", h.refreshDiscovery)

		// === 自定义模板 ===
		apps.GET("/custom", h.listCustomTemplates)
		apps.GET("/custom/:id", h.getCustomTemplate)
		apps.POST("/custom", h.createCustomTemplate)
		apps.POST("/custom/url", h.createCustomFromURL)
		apps.POST("/custom/github", h.createCustomFromGitHub)
		apps.PUT("/custom/:id", h.updateCustomTemplate)
		apps.DELETE("/custom/:id", h.deleteCustomTemplate)
		apps.POST("/custom/:id/install", h.installCustomTemplate)

		// === 版本管理 ===
		apps.GET("/versions/:templateId", h.getAvailableVersions)
		apps.POST("/versions/check", h.checkUpdates)
		apps.GET("/notifications", h.getNotifications)
		apps.POST("/notifications/:id/read", h.markNotificationRead)
		apps.POST("/notifications/:id/dismiss", h.dismissNotification)
		apps.POST("/notifications/read-all", h.markAllNotificationsRead)
		apps.GET("/notifications/unread-count", h.getUnreadCount)
		apps.POST("/installed/:id/update-version", h.updateAppVersion)
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

	// 添加评分统计
	type TemplateWithStats struct {
		*AppTemplate
		RatingStats *RatingStats `json:"ratingStats"`
	}

	result := make([]*TemplateWithStats, 0, len(templates))
	for _, t := range templates {
		ts := &TemplateWithStats{AppTemplate: t}
		if h.ratingManager != nil {
			ts.RatingStats = h.ratingManager.GetStats(t.ID)
		}
		result = append(result, ts)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"templates":  result,
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

	// 添加评分统计
	type TemplateWithStats struct {
		*AppTemplate
		RatingStats *RatingStats `json:"ratingStats"`
	}

	result := &TemplateWithStats{AppTemplate: template}
	if h.ratingManager != nil {
		result.RatingStats = h.ratingManager.GetStats(id)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// listInstalled 列出已安装应用
func (h *AppHandlers) listInstalled(c *gin.Context) {
	apps := h.store.ListInstalled()

	// 添加更新通知信息
	type InstalledWithUpdates struct {
		*InstalledApp
		HasUpdate     bool   `json:"hasUpdate"`
		LatestVersion string `json:"latestVersion,omitempty"`
	}

	result := make([]*InstalledWithUpdates, 0, len(apps))
	for _, app := range apps {
		iwu := &InstalledWithUpdates{InstalledApp: app}
		if h.versionManager != nil {
			// 检查是否有更新
			notifications := h.versionManager.GetNotifications(false)
			for _, n := range notifications {
				if n.AppID == app.ID && !n.Dismissed {
					iwu.HasUpdate = true
					iwu.LatestVersion = n.LatestVer
					break
				}
			}
		}
		result = append(result, iwu)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
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
		// 不返回错误，继续处理
		_ = err // 明确忽略错误，避免 staticcheck 警告
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

	// 验证评分购买
	if h.ratingManager != nil {
		userID := c.GetString("userID")
		if userID != "" {
			if err := h.ratingManager.VerifyPurchase(templateID, userID); err != nil {
				// 记录验证失败，但不阻断安装流程
				log.Printf("验证购买失败: %v", err)
			}
		}
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

// === 评分和评论 ===

// getRatings 获取评分列表
func (h *AppHandlers) getRatings(c *gin.Context) {
	templateID := c.Param("templateId")

	if h.ratingManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*AppRating{},
		})
		return
	}

	sortby := c.DefaultQuery("sort", "recent")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ratings := h.ratingManager.GetRatings(templateID, sortby, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ratings,
	})
}

// getRatingStats 获取评分统计
func (h *AppHandlers) getRatingStats(c *gin.Context) {
	templateID := c.Param("templateId")

	if h.ratingManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    &RatingStats{TemplateID: templateID},
		})
		return
	}

	stats := h.ratingManager.GetStats(templateID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// addRating 添加评分
func (h *AppHandlers) addRating(c *gin.Context) {
	templateID := c.Param("templateId")

	var req struct {
		Rating  int    `json:"rating"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误",
		})
		return
	}

	if h.ratingManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "评分系统未初始化",
		})
		return
	}

	// 获取用户信息
	userID := c.GetString("userID")
	if userID == "" {
		userID = "anonymous"
	}
	userName := c.GetString("userName")
	if userName == "" {
		userName = "匿名用户"
	}

	rating, err := h.ratingManager.AddRating(templateID, userID, userName, req.Rating, req.Title, req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 验证购买
	if err := h.ratingManager.VerifyPurchase(templateID, userID); err != nil {
		// 记录验证失败，但不阻断评分流程
		log.Printf("验证购买失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "评分成功",
		"data":    rating,
	})
}

// deleteRating 删除评分
func (h *AppHandlers) deleteRating(c *gin.Context) {
	templateID := c.Param("templateId")
	ratingID := c.Param("ratingId")

	if h.ratingManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "评分系统未初始化",
		})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		userID = "anonymous"
	}

	if err := h.ratingManager.DeleteRating(templateID, ratingID, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "评分已删除",
	})
}

// markHelpful 标记有用
func (h *AppHandlers) markHelpful(c *gin.Context) {
	templateID := c.Param("templateId")
	ratingID := c.Param("ratingId")

	if h.ratingManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "评分系统未初始化",
		})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		userID = "anonymous"
	}

	if err := h.ratingManager.MarkHelpful(templateID, ratingID, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已标记",
	})
}

// === 应用发现 ===

// getDiscovered 获取发现的应用
func (h *AppHandlers) getDiscovered(c *gin.Context) {
	if h.discovery == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*DiscoveredApp{},
		})
		return
	}

	source := c.Query("source")
	category := c.Query("category")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	apps := h.discovery.GetDiscoveredApps(source, category, limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"apps":       apps,
			"lastUpdate": h.discovery.GetLastUpdateTime(),
			"cacheValid": h.discovery.IsCacheValid(),
		},
	})
}

// refreshDiscovery 刷新发现
func (h *AppHandlers) refreshDiscovery(c *gin.Context) {
	if h.discovery == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "应用发现未初始化",
		})
		return
	}

	if err := h.discovery.RefreshDiscovery(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "刷新成功",
	})
}

// === 自定义模板 ===

// listCustomTemplates 列出自定义模板
func (h *AppHandlers) listCustomTemplates(c *gin.Context) {
	if h.customTemplateMgr == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*CustomTemplate{},
		})
		return
	}

	templates := h.customTemplateMgr.ListTemplates()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    templates,
	})
}

// getCustomTemplate 获取自定义模板
func (h *AppHandlers) getCustomTemplate(c *gin.Context) {
	id := c.Param("id")

	if h.customTemplateMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自定义模板系统未初始化",
		})
		return
	}

	template, err := h.customTemplateMgr.GetTemplate(id)
	if err != nil {
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

// createCustomTemplate 创建自定义模板
func (h *AppHandlers) createCustomTemplate(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Compose     string `json:"compose"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误",
		})
		return
	}

	if h.customTemplateMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自定义模板系统未初始化",
		})
		return
	}

	template, err := h.customTemplateMgr.CreateFromCompose(req.Name, req.DisplayName, req.Description, req.Category, req.Compose)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板创建成功",
		"data":    template,
	})
}

// createCustomFromURL 从 URL 创建模板
func (h *AppHandlers) createCustomFromURL(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Category    string `json:"category"`
		URL         string `json:"url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误",
		})
		return
	}

	if h.customTemplateMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自定义模板系统未初始化",
		})
		return
	}

	template, err := h.customTemplateMgr.CreateFromURL(req.Name, req.DisplayName, req.Description, req.Category, req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板创建成功",
		"data":    template,
	})
}

// createCustomFromGitHub 从 GitHub 创建模板
func (h *AppHandlers) createCustomFromGitHub(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Category    string `json:"category"`
		GitHubURL   string `json:"githubUrl"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误",
		})
		return
	}

	if h.customTemplateMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自定义模板系统未初始化",
		})
		return
	}

	// 解析 GitHub URL: owner/repo/path@ref
	var owner, repo, path, ref string
	parts := strings.Split(req.GitHubURL, "/")
	if len(parts) >= 2 {
		owner = parts[0]
		repo = parts[1]
	}
	ref = "main"
	path = "docker-compose.yml"

	template, err := h.customTemplateMgr.ImportFromGitHub(owner, repo, path, ref, req.Name, req.DisplayName, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板创建成功",
		"data":    template,
	})
}

// updateCustomTemplate 更新自定义模板
func (h *AppHandlers) updateCustomTemplate(c *gin.Context) {
	id := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误",
		})
		return
	}

	if h.customTemplateMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自定义模板系统未初始化",
		})
		return
	}

	template, err := h.customTemplateMgr.UpdateTemplate(id, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板更新成功",
		"data":    template,
	})
}

// deleteCustomTemplate 删除自定义模板
func (h *AppHandlers) deleteCustomTemplate(c *gin.Context) {
	id := c.Param("id")

	if h.customTemplateMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自定义模板系统未初始化",
		})
		return
	}

	if err := h.customTemplateMgr.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "模板已删除",
	})
}

// installCustomTemplate 安装自定义模板
func (h *AppHandlers) installCustomTemplate(c *gin.Context) {
	id := c.Param("id")

	if h.customTemplateMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自定义模板系统未初始化",
		})
		return
	}

	template, err := h.customTemplateMgr.GetTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "模板不存在",
		})
		return
	}

	var req struct {
		Ports       map[string]interface{} `json:"ports"`
		Volumes     map[string]interface{} `json:"volumes"`
		Environment map[string]interface{} `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空请求，使用默认配置
		_ = err // 明确忽略错误，避免 staticcheck 警告
	}

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

	// 使用模板 ID 安装
	app, err := h.store.InstallApp(id, config)
	if err != nil {
		// 如果安装失败，尝试直接使用自定义模板
		app, err = h.store.InstallApp(template.Name, config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": err.Error(),
			})
			return
		}
	}

	h.customTemplateMgr.IncrementDownloads(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用安装成功",
		"data":    app,
	})
}

// === 版本管理 ===

// getAvailableVersions 获取可用版本
func (h *AppHandlers) getAvailableVersions(c *gin.Context) {
	templateID := c.Param("templateId")

	if h.versionManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*AppVersion{},
		})
		return
	}

	versions, err := h.versionManager.GetAvailableVersions(templateID)
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
		"data":    versions,
	})
}

// checkUpdates 检查更新
func (h *AppHandlers) checkUpdates(c *gin.Context) {
	if h.versionManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*UpdateNotification{},
		})
		return
	}

	updates, err := h.versionManager.CheckForUpdates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "检查完成",
		"data":    updates,
	})
}

// getNotifications 获取通知列表
func (h *AppHandlers) getNotifications(c *gin.Context) {
	if h.versionManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    []*UpdateNotification{},
		})
		return
	}

	unreadOnly := c.Query("unread") == "true"
	notifications := h.versionManager.GetNotifications(unreadOnly)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    notifications,
	})
}

// markNotificationRead 标记通知已读
func (h *AppHandlers) markNotificationRead(c *gin.Context) {
	id := c.Param("id")

	if h.versionManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "版本管理系统未初始化",
		})
		return
	}

	if err := h.versionManager.MarkNotificationRead(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已标记已读",
	})
}

// dismissNotification 忽略通知
func (h *AppHandlers) dismissNotification(c *gin.Context) {
	id := c.Param("id")

	if h.versionManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "版本管理系统未初始化",
		})
		return
	}

	if err := h.versionManager.DismissNotification(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已忽略",
	})
}

// markAllNotificationsRead 标记所有通知已读
func (h *AppHandlers) markAllNotificationsRead(c *gin.Context) {
	if h.versionManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "版本管理系统未初始化",
		})
		return
	}

	if err := h.versionManager.MarkAllNotificationsRead(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已全部标记已读",
	})
}

// getUnreadCount 获取未读通知数量
func (h *AppHandlers) getUnreadCount(c *gin.Context) {
	if h.versionManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"count": 0,
			},
		})
		return
	}

	count := h.versionManager.GetUnreadCount()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"count": count,
		},
	})
}

// updateAppVersion 更新应用版本
func (h *AppHandlers) updateAppVersion(c *gin.Context) {
	appID := c.Param("id")

	var req struct {
		Version string `json:"version"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误",
		})
		return
	}

	if h.versionManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "版本管理系统未初始化",
		})
		return
	}

	if err := h.versionManager.UpdateAppVersion(appID, req.Version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "版本更新成功",
	})
}
