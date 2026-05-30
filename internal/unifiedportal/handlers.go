// Package unifiedportal 提供 REST API 处理器
package unifiedportal

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 统一门户 API 处理器
type Handlers struct {
	manager *PortalManager
}

// NewHandlers 创建处理器
func NewHandlers(manager *PortalManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	portal := r.Group("/portal")
	{
		// 仪表盘管理
		portal.GET("/dashboards", h.listDashboards)
		portal.GET("/dashboards/search", h.searchDashboards)
		portal.POST("/dashboards", h.createDashboard)
		portal.GET("/dashboards/:id", h.getDashboard)
		portal.PUT("/dashboards/:id", h.updateDashboard)
		portal.DELETE("/dashboards/:id", h.deleteDashboard)
		portal.POST("/dashboards/:id/export", h.exportDashboard)
		portal.POST("/dashboards/import", h.importDashboard)
		portal.POST("/dashboards/from-template/:template_id", h.createFromTemplate)
		portal.POST("/dashboards/:id/duplicate", h.duplicateDashboard)

		// Widget管理
		portal.POST("/dashboards/:id/widgets", h.addWidget)
		portal.PUT("/widgets/:id", h.updateWidget)
		portal.PATCH("/widgets/:id/move", h.moveWidget)
		portal.PATCH("/widgets/:id/visibility", h.toggleWidgetVisibility)
		portal.DELETE("/widgets/:id", h.deleteWidget)

		// 主题管理
		portal.GET("/themes", h.getThemes)
		portal.GET("/themes/active", h.getActiveTheme)
		portal.PUT("/theme", h.switchTheme)

		// 数据源管理
		portal.GET("/datasources", h.listDataSources)
		portal.POST("/datasources", h.registerDataSource)
		portal.DELETE("/datasources/:id", h.deleteDataSource)

		// 指标聚合
		portal.GET("/metrics", h.getMetrics)

		// 系统健康检查
		portal.GET("/health", h.healthCheck)
		portal.GET("/stats", h.getDashboardStats)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listDashboards 列出仪表盘
func (h *Handlers) listDashboards(c *gin.Context) {
	owner := c.Query("owner")
	dashboards := h.manager.ListDashboards(owner)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    dashboards,
	})
}

// createDashboard 创建仪表盘
func (h *Handlers) createDashboard(c *gin.Context) {
	var req DashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	owner := c.GetString("user_id") // 从认证中间件获取
	dashboard, err := h.manager.CreateDashboard(&req, owner)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "dashboard created",
		Data:    dashboard,
	})
}

// getDashboard 获取仪表盘
func (h *Handlers) getDashboard(c *gin.Context) {
	id := c.Param("id")
	dashboard, err := h.manager.GetDashboard(id)
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
		Data:    dashboard,
	})
}

// updateDashboard 更新仪表盘
func (h *Handlers) updateDashboard(c *gin.Context) {
	id := c.Param("id")
	var req DashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	dashboard, err := h.manager.UpdateDashboard(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "dashboard updated",
		Data:    dashboard,
	})
}

// deleteDashboard 删除仪表盘
func (h *Handlers) deleteDashboard(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDashboard(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "dashboard deleted",
	})
}

// exportDashboard 导出仪表盘
func (h *Handlers) exportDashboard(c *gin.Context) {
	id := c.Param("id")
	export, err := h.manager.ExportDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "dashboard exported",
		Data:    export,
	})
}

// importDashboard 导入仪表盘
func (h *Handlers) importDashboard(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "failed to read request body",
		})
		return
	}

	// 尝试解析JSON格式的导出数据
	var export DashboardExport
	if err := json.Unmarshal(body, &export); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid import data: " + err.Error(),
		})
		return
	}

	owner := c.GetString("user_id")
	dashboard, err := h.manager.ImportDashboard(body, owner)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "dashboard imported",
		Data:    dashboard,
	})
}

// createFromTemplate 从模板创建仪表盘
func (h *Handlers) createFromTemplate(c *gin.Context) {
	templateID := c.Param("template_id")
	name := c.Query("name")
	if name == "" {
		name = "新建仪表盘"
	}

	owner := c.GetString("user_id")
	dashboard, err := h.manager.CreateDashboardFromTemplate(templateID, name, owner)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "dashboard created from template",
		Data:    dashboard,
	})
}

// addWidget 添加Widget
func (h *Handlers) addWidget(c *gin.Context) {
	dashboardID := c.Param("id")
	var req WidgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	widget, err := h.manager.AddWidget(dashboardID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "widget added",
		Data:    widget,
	})
}

// updateWidget 更新Widget
func (h *Handlers) updateWidget(c *gin.Context) {
	id := c.Param("id")
	var req WidgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	widget, err := h.manager.UpdateWidget(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "widget updated",
		Data:    widget,
	})
}

// moveWidget 移动Widget
func (h *Handlers) moveWidget(c *gin.Context) {
	id := c.Param("id")
	var req WidgetMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	widget, err := h.manager.MoveWidget(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "widget moved",
		Data:    widget,
	})
}

// deleteWidget 删除Widget
func (h *Handlers) deleteWidget(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteWidget(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "widget deleted",
	})
}

// getThemes 获取主题列表
func (h *Handlers) getThemes(c *gin.Context) {
	themes := h.manager.GetThemes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    themes,
	})
}

// getActiveTheme 获取当前激活主题
func (h *Handlers) getActiveTheme(c *gin.Context) {
	theme, err := h.manager.GetActiveTheme()
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
		Data:    theme,
	})
}

// switchTheme 切换主题
func (h *Handlers) switchTheme(c *gin.Context) {
	var req ThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	theme, err := h.manager.SwitchTheme(req.ThemeID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "theme switched",
		Data:    theme,
	})
}

// listDataSources 列出数据源
func (h *Handlers) listDataSources(c *gin.Context) {
	sources := h.manager.ListDataSources()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    sources,
	})
}

// registerDataSource 注册数据源
func (h *Handlers) registerDataSource(c *gin.Context) {
	var ds DataSource
	if err := c.ShouldBindJSON(&ds); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result := h.manager.RegisterDataSource(&ds)
	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "data source registered",
		Data:    result,
	})
}

// deleteDataSource 删除数据源
func (h *Handlers) deleteDataSource(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDataSource(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "data source deleted",
	})
}

// getMetrics 获取聚合指标
func (h *Handlers) getMetrics(c *gin.Context) {
	metrics := h.manager.AggregateMetrics()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    metrics,
	})
}

// searchDashboards 搜索仪表盘
func (h *Handlers) searchDashboards(c *gin.Context) {
	keyword := c.Query("q")
	tag := c.Query("tag")
	onlyTemplates := c.Query("only_templates") == "true"

	dashboards := h.manager.SearchDashboards(keyword, tag, onlyTemplates)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    dashboards,
	})
}

// duplicateDashboard 复制仪表盘
func (h *Handlers) duplicateDashboard(c *gin.Context) {
	id := c.Param("id")
	name := c.Query("name")

	onner := c.GetString("user_id")
	dashboard, err := h.manager.DuplicateDashboard(id, name, onner)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "dashboard duplicated",
		Data:    dashboard,
	})
}

// toggleWidgetVisibility 切换Widget可见性
func (h *Handlers) toggleWidgetVisibility(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		IsVisible bool `json:"is_visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	widget, err := h.manager.ToggleWidgetVisibility(id, req.IsVisible)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "widget visibility toggled",
		Data:    widget,
	})
}

// healthCheck 系统健康检查
func (h *Handlers) healthCheck(c *gin.Context) {
	health := h.manager.HealthCheck()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "healthy",
		Data:    health,
	})
}

// getDashboardStats 获取仪表盘统计信息
func (h *Handlers) getDashboardStats(c *gin.Context) {
	stats := h.manager.GetDashboardStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
