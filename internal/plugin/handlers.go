package plugin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 插件 API 处理器.
type Handlers struct {
	manager *Manager
	market  *Market
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager, market *Market) *Handlers {
	return &Handlers{
		manager: manager,
		market:  market,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	plugins := rg.Group("/plugins")
	{
		// 已安装插件管理
		plugins.GET("", h.list)
		plugins.GET("/:id", h.get)
		plugins.POST("", h.install)
		plugins.DELETE("/:id", h.uninstall)

		// 插件操作
		plugins.POST("/:id/enable", h.enable)
		plugins.POST("/:id/disable", h.disable)
		plugins.POST("/:id/start", h.start)
		plugins.POST("/:id/stop", h.stop)
		plugins.POST("/:id/update", h.update)
		plugins.PUT("/:id/config", h.configure)

		// 插件市场
		market := plugins.Group("/market")
		{
			market.GET("", h.marketList)
			market.GET("/search", h.marketSearch)
			market.GET("/categories", h.marketCategories)
			market.GET("/:id", h.marketDetail)
			market.POST("/:id/rate", h.marketRate)
			market.GET("/:id/reviews", h.marketReviews)
		}

		// 发现本地插件
		plugins.GET("/discover", h.discover)
	}
}

// ========== 已安装插件 API ==========

// list 列出已安装插件.
func (h *Handlers) list(c *gin.Context) {
	states := h.manager.List()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    states,
	})
}

// get 获取插件详情.
func (h *Handlers) get(c *gin.Context) {
	pluginID := c.Param("id")

	state, err := h.manager.Get(pluginID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	// 获取更多信息
	inst, _ := h.manager.loader.GetInstance(pluginID)
	var info *Info
	if inst != nil {
		info = &inst.Info
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"state": state,
			"info":  info,
		},
	})
}

// install 安装插件.
func (h *Handlers) install(c *gin.Context) {
	var req struct {
		Source string `json:"source" binding:"required"` // URL, 本地路径, 或插件 ID
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误: " + err.Error(),
		})
		return
	}

	state, err := h.manager.Install(req.Source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "安装失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "安装成功",
		"data":    state,
	})
}

// uninstall 卸载插件.
func (h *Handlers) uninstall(c *gin.Context) {
	pluginID := c.Param("id")

	if err := h.manager.Uninstall(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "卸载失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "卸载成功",
	})
}

// enable 启用插件.
func (h *Handlers) enable(c *gin.Context) {
	pluginID := c.Param("id")

	if err := h.manager.Enable(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "启用失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "插件已启用",
	})
}

// disable 禁用插件.
func (h *Handlers) disable(c *gin.Context) {
	pluginID := c.Param("id")

	if err := h.manager.Disable(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "禁用失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "插件已禁用",
	})
}

// start 启动插件.
func (h *Handlers) start(c *gin.Context) {
	pluginID := c.Param("id")

	if err := h.manager.StartPlugin(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "启动失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "插件已启动",
	})
}

// stop 停止插件.
func (h *Handlers) stop(c *gin.Context) {
	pluginID := c.Param("id")

	if err := h.manager.StopPlugin(pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "停止失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "插件已停止",
	})
}

// update 更新插件.
func (h *Handlers) update(c *gin.Context) {
	pluginID := c.Param("id")

	state, err := h.manager.Update(pluginID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    state,
	})
}

// configure 配置插件.
func (h *Handlers) configure(c *gin.Context) {
	pluginID := c.Param("id")

	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误: " + err.Error(),
		})
		return
	}

	if err := h.manager.Configure(pluginID, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "配置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已保存",
	})
}

// discover 发现可用插件.
func (h *Handlers) discover(c *gin.Context) {
	plugins, err := h.manager.Discover()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "发现插件失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    plugins,
	})
}

// ========== 插件市场 API ==========

// marketList 市场插件列表.
func (h *Handlers) marketList(c *gin.Context) {
	category := c.Query("category")
	sort := c.DefaultQuery("sort", "popular") // popular, newest, rating
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil {
		pageSize = 20
	}

	if h.market == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"plugins":  []MarketInfo{},
				"total":    0,
				"page":     page,
				"pageSize": pageSize,
			},
		})
		return
	}

	plugins, total, err := h.market.List(category, sort, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"plugins":  plugins,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// marketSearch 搜索市场插件.
func (h *Handlers) marketSearch(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	if h.market == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"plugins":  []MarketInfo{},
				"total":    0,
				"page":     page,
				"pageSize": pageSize,
			},
		})
		return
	}

	plugins, total, err := h.market.Search(query, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"plugins":  plugins,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

// marketCategories 获取分类列表.
func (h *Handlers) marketCategories(c *gin.Context) {
	categories := []gin.H{
		{"id": "storage", "name": "存储管理", "icon": "hard-drive"},
		{"id": "file-manager", "name": "文件管理", "icon": "folder"},
		{"id": "network", "name": "网络工具", "icon": "network"},
		{"id": "system", "name": "系统工具", "icon": "settings"},
		{"id": "security", "name": "安全工具", "icon": "shield"},
		{"id": "media", "name": "多媒体", "icon": "play-circle"},
		{"id": "backup", "name": "备份同步", "icon": "cloud-upload"},
		{"id": "theme", "name": "主题外观", "icon": "palette"},
		{"id": "integration", "name": "第三方集成", "icon": "plug"},
		{"id": "developer", "name": "开发工具", "icon": "code"},
		{"id": "productivity", "name": "生产力", "icon": "briefcase"},
		{"id": "other", "name": "其他", "icon": "more-horizontal"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    categories,
	})
}

// marketDetail 获取市场插件详情.
func (h *Handlers) marketDetail(c *gin.Context) {
	pluginID := c.Param("id")

	if h.market == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    nil,
		})
		return
	}

	info, err := h.market.GetDetail(pluginID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "插件不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    info,
	})
}

// marketRate 插件评分.
func (h *Handlers) marketRate(c *gin.Context) {
	pluginID := c.Param("id")

	var req struct {
		Rating int    `json:"rating" binding:"required,min=1,max=5"`
		Review string `json:"review"`
		UserID string `json:"userId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求格式错误: " + err.Error(),
		})
		return
	}

	if h.market == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "评分成功",
		})
		return
	}

	if err := h.market.Rate(pluginID, req.UserID, req.Rating, req.Review); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "评分失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "评分成功",
	})
}

// marketReviews 获取插件评论.
func (h *Handlers) marketReviews(c *gin.Context) {
	pluginID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	if h.market == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"reviews":  []Review{},
				"total":    0,
				"page":     page,
				"pageSize": pageSize,
			},
		})
		return
	}

	reviews, total, err := h.market.GetReviews(pluginID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取评论失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"reviews":  reviews,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}
