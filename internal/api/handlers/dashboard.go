// Package handlers 提供 Dashboard API 端点
// v2.56.0 - 兵部实现
package handlers

import (
	"net/http"
	"time"

	"nas-os/internal/dashboard"
	"nas-os/internal/monitor"

	"github.com/gin-gonic/gin"
)

// CreateDashboardRequest 创建仪表板请求
type CreateDashboardRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateDashboardRequest 更新仪表板请求
type UpdateDashboardRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Layout      *dashboard.Layout `json:"layout"`
}

// CloneDashboardRequest 克隆仪表板请求
type CloneDashboardRequest struct {
	Name string `json:"name" binding:"required"`
}

// AddWidgetRequest 添加小组件请求
type AddWidgetRequest struct {
	Type        dashboard.WidgetType     `json:"type" binding:"required"`
	Title       string                   `json:"title"`
	Size        dashboard.WidgetSize     `json:"size"`
	Position    dashboard.WidgetPosition `json:"position"`
	Config      dashboard.WidgetConfig   `json:"config"`
	RefreshRate int                      `json:"refreshRate"`
}

// UpdateWidgetRequest 更新小组件请求
type UpdateWidgetRequest struct {
	Title       string                    `json:"title"`
	Size        dashboard.WidgetSize      `json:"size"`
	Position    *dashboard.WidgetPosition `json:"position"`
	Config      *dashboard.WidgetConfig   `json:"config"`
	Enabled     *bool                     `json:"enabled"`
	RefreshRate int                       `json:"refreshRate"`
}

// UpdatePositionRequest 更新位置请求
type UpdatePositionRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// LayoutRequest 布局请求
type LayoutRequest struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

// CreateFromTemplateRequest 从模板创建请求
type CreateFromTemplateRequest struct {
	TemplateID string `json:"templateId" binding:"required"`
	Name       string `json:"name"`
}

// DashboardHandlers Dashboard API 处理器
type DashboardHandlers struct {
	manager    *dashboard.Manager
	monitorMgr *monitor.Manager
	wsHub      interface {
		Broadcast(msgType string, data interface{}) error
	}
}

// NewDashboardHandlers 创建 Dashboard 处理器
func NewDashboardHandlers(mgr *dashboard.Manager, monitorMgr *monitor.Manager, wsHub interface {
	Broadcast(msgType string, data interface{}) error
}) *DashboardHandlers {
	return &DashboardHandlers{
		manager:    mgr,
		monitorMgr: monitorMgr,
		wsHub:      wsHub,
	}
}

// RegisterRoutes 注册路由
func (h *DashboardHandlers) RegisterRoutes(r *gin.RouterGroup) {
	dashboards := r.Group("/dashboards")
	{
		// 仪表板 CRUD
		dashboards.GET("", h.ListDashboards)
		dashboards.POST("", h.CreateDashboard)
		dashboards.GET("/:id", h.GetDashboard)
		dashboards.PUT("/:id", h.UpdateDashboard)
		dashboards.DELETE("/:id", h.DeleteDashboard)
		dashboards.POST("/:id/clone", h.CloneDashboard)

		// 小组件管理
		dashboards.GET("/:id/widgets", h.GetWidgets)
		dashboards.POST("/:id/widgets", h.AddWidget)
		dashboards.PUT("/:id/widgets/:widgetId", h.UpdateWidget)
		dashboards.DELETE("/:id/widgets/:widgetId", h.RemoveWidget)
		dashboards.PUT("/:id/widgets/:widgetId/position", h.UpdateWidgetPosition)

		// 布局管理
		dashboards.GET("/:id/layout", h.GetLayout)
		dashboards.PUT("/:id/layout", h.UpdateLayout)
		dashboards.POST("/:id/layout/reset", h.ResetLayout)

		// 数据获取
		dashboards.GET("/:id/state", h.GetDashboardState)
		dashboards.GET("/:id/widgets/:widgetId/data", h.GetWidgetData)
		dashboards.POST("/:id/refresh", h.RefreshDashboard)

		// 导入导出
		dashboards.GET("/:id/export", h.ExportDashboard)
		dashboards.POST("/import", h.ImportDashboard)

		// 模板
		dashboards.GET("/templates", h.GetTemplates)
		dashboards.POST("/from-template", h.CreateFromTemplate)
	}

	// 默认仪表板
	r.GET("/dashboard/default", h.GetDefaultDashboard)
	r.GET("/dashboard/stats", h.GetDashboardStats)
	r.GET("/dashboard/widgets/types", h.GetWidgetTypes)

	// 实时推送
	r.GET("/dashboard/ws", h.HandleDashboardWebSocket)
}

// ListDashboards 列出所有仪表板
// @Summary 列出所有仪表板
// @Description 获取用户所有仪表板列表
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /dashboards [get]
func (h *DashboardHandlers) ListDashboards(c *gin.Context) {
	dashboards := h.manager.ListDashboards()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    dashboards,
	})
}

// CreateDashboard 创建仪表板
// @Summary 创建新仪表板
// @Description 创建一个新的自定义仪表板
// @Tags dashboard
// @Accept json
// @Produce json
// @Param request body CreateDashboardRequest true "仪表板参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards [post]
func (h *DashboardHandlers) CreateDashboard(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	d, err := h.manager.CreateDashboard(req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "仪表板创建成功",
		"data":    d,
	})
}

// GetDashboard 获取仪表板详情
// @Summary 获取仪表板详情
// @Description 根据ID获取仪表板详细信息
// @Tags dashboard
// @Produce json
// @Param id path string true "仪表板ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id} [get]
func (h *DashboardHandlers) GetDashboard(c *gin.Context) {
	id := c.Param("id")

	d, err := h.manager.GetDashboard(id)
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
		"data":    d,
	})
}

// UpdateDashboard 更新仪表板
// @Summary 更新仪表板
// @Description 更新仪表板信息和布局
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "仪表板ID"
// @Param request body UpdateDashboardRequest true "更新参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id} [put]
func (h *DashboardHandlers) UpdateDashboard(c *gin.Context) {
	id := c.Param("id")

	d, err := h.manager.GetDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Layout      *dashboard.Layout `json:"layout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Name != "" {
		d.Name = req.Name
	}
	if req.Description != "" {
		d.Description = req.Description
	}
	if req.Layout != nil {
		d.Layout = *req.Layout
	}

	if err := h.manager.UpdateDashboard(d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "仪表板更新成功",
		"data":    d,
	})
}

// DeleteDashboard 删除仪表板
// @Summary 删除仪表板
// @Description 删除指定仪表板
// @Tags dashboard
// @Param id path string true "仪表板ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id} [delete]
func (h *DashboardHandlers) DeleteDashboard(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteDashboard(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "仪表板已删除",
	})
}

// CloneDashboard 克隆仪表板
// @Summary 克隆仪表板
// @Description 复制现有仪表板
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "仪表板ID"
// @Param request body CloneDashboardRequest true "克隆参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/clone [post]
func (h *DashboardHandlers) CloneDashboard(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	d, err := h.manager.CloneDashboard(id, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "仪表板克隆成功",
		"data":    d,
	})
}

// GetWidgets 获取小组件列表
// @Summary 获取仪表板小组件
// @Description 获取仪表板的所有小组件
// @Tags dashboard
// @Produce json
// @Param id path string true "仪表板ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/widgets [get]
func (h *DashboardHandlers) GetWidgets(c *gin.Context) {
	id := c.Param("id")

	d, err := h.manager.GetDashboard(id)
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
		"data":    d.Widgets,
	})
}

// AddWidget 添加小组件
// @Summary 添加小组件
// @Description 向仪表板添加新的小组件
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "仪表板ID"
// @Param request body AddWidgetRequest true "小组件参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/widgets [post]
func (h *DashboardHandlers) AddWidget(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Type        dashboard.WidgetType     `json:"type" binding:"required"`
		Title       string                   `json:"title"`
		Size        dashboard.WidgetSize     `json:"size"`
		Position    dashboard.WidgetPosition `json:"position"`
		Config      dashboard.WidgetConfig   `json:"config"`
		RefreshRate int                      `json:"refreshRate"` // 秒
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	widget := &dashboard.Widget{
		Type:     req.Type,
		Title:    req.Title,
		Size:     req.Size,
		Position: req.Position,
		Config:   req.Config,
		Enabled:  true,
	}

	if req.RefreshRate > 0 {
		widget.RefreshRate = time.Duration(req.RefreshRate) * time.Second
	} else {
		widget.RefreshRate = 5 * time.Second
	}

	if widget.Size == "" {
		widget.Size = dashboard.WidgetSizeMedium
	}

	if err := h.manager.AddWidget(id, widget); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "小组件添加成功",
		"data":    widget,
	})
}

// UpdateWidget 更新小组件
// @Summary 更新小组件
// @Description 更新小组件配置
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "仪表板ID"
// @Param widgetId path string true "小组件ID"
// @Param request body UpdateWidgetRequest true "更新参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/widgets/{widgetId} [put]
func (h *DashboardHandlers) UpdateWidget(c *gin.Context) {
	dashboardID := c.Param("id")
	widgetID := c.Param("widgetId")

	d, err := h.manager.GetDashboard(dashboardID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	var existingWidget *dashboard.Widget
	for _, w := range d.Widgets {
		if w.ID == widgetID {
			existingWidget = w
			break
		}
	}

	if existingWidget == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "小组件不存在",
		})
		return
	}

	var req struct {
		Title       string                    `json:"title"`
		Size        dashboard.WidgetSize      `json:"size"`
		Position    *dashboard.WidgetPosition `json:"position"`
		Config      *dashboard.WidgetConfig   `json:"config"`
		Enabled     *bool                     `json:"enabled"`
		RefreshRate int                       `json:"refreshRate"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Title != "" {
		existingWidget.Title = req.Title
	}
	if req.Size != "" {
		existingWidget.Size = req.Size
	}
	if req.Position != nil {
		existingWidget.Position = *req.Position
	}
	if req.Config != nil {
		existingWidget.Config = *req.Config
	}
	if req.Enabled != nil {
		existingWidget.Enabled = *req.Enabled
	}
	if req.RefreshRate > 0 {
		existingWidget.RefreshRate = time.Duration(req.RefreshRate) * time.Second
	}

	if err := h.manager.UpdateWidget(dashboardID, existingWidget); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "小组件更新成功",
		"data":    existingWidget,
	})
}

// RemoveWidget 删除小组件
// @Summary 删除小组件
// @Description 从仪表板删除小组件
// @Tags dashboard
// @Param id path string true "仪表板ID"
// @Param widgetId path string true "小组件ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/widgets/{widgetId} [delete]
func (h *DashboardHandlers) RemoveWidget(c *gin.Context) {
	dashboardID := c.Param("id")
	widgetID := c.Param("widgetId")

	if err := h.manager.RemoveWidget(dashboardID, widgetID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "小组件已删除",
	})
}

// UpdateWidgetPosition 更新小组件位置
// @Summary 更新小组件位置
// @Description 更新小组件在仪表板中的位置
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "仪表板ID"
// @Param widgetId path string true "小组件ID"
// @Param request body UpdatePositionRequest true "位置参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/widgets/{widgetId}/position [put]
func (h *DashboardHandlers) UpdateWidgetPosition(c *gin.Context) {
	dashboardID := c.Param("id")
	widgetID := c.Param("widgetId")

	d, err := h.manager.GetDashboard(dashboardID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	var existingWidget *dashboard.Widget
	for _, w := range d.Widgets {
		if w.ID == widgetID {
			existingWidget = w
			break
		}
	}

	if existingWidget == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "小组件不存在",
		})
		return
	}

	var req struct {
		X int `json:"x"`
		Y int `json:"y"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	existingWidget.Position = dashboard.WidgetPosition{X: req.X, Y: req.Y}

	if err := h.manager.UpdateWidget(dashboardID, existingWidget); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "位置更新成功",
	})
}

// GetLayout 获取布局
// @Summary 获取仪表板布局
// @Description 获取仪表板的布局配置
// @Tags dashboard
// @Produce json
// @Param id path string true "仪表板ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/layout [get]
func (h *DashboardHandlers) GetLayout(c *gin.Context) {
	id := c.Param("id")

	d, err := h.manager.GetDashboard(id)
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
		"data":    d.Layout,
	})
}

// UpdateLayout 更新布局
// @Summary 更新仪表板布局
// @Description 更新仪表板的整体布局配置
// @Tags dashboard
// @Accept json
// @Produce json
// @Param id path string true "仪表板ID"
// @Param request body LayoutRequest true "布局参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/layout [put]
func (h *DashboardHandlers) UpdateLayout(c *gin.Context) {
	id := c.Param("id")

	d, err := h.manager.GetDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	var req struct {
		Columns int `json:"columns"`
		Rows    int `json:"rows"`
		Gap     int `json:"gap"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Columns > 0 {
		d.Layout.Columns = req.Columns
	}
	if req.Rows > 0 {
		d.Layout.Rows = req.Rows
	}
	if req.Gap >= 0 {
		d.Layout.Gap = req.Gap
	}

	if err := h.manager.UpdateDashboard(d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "布局更新成功",
	})
}

// ResetLayout 重置布局
// @Summary 重置布局
// @Description 重置仪表板布局为默认
// @Tags dashboard
// @Param id path string true "仪表板ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/layout/reset [post]
func (h *DashboardHandlers) ResetLayout(c *gin.Context) {
	id := c.Param("id")

	d, err := h.manager.GetDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	// 重置布局
	d.Layout = dashboard.Layout{
		Columns: 2,
		Rows:    2,
		Gap:     10,
	}

	// 重置小组件位置
	for i, w := range d.Widgets {
		w.Position = dashboard.WidgetPosition{
			X: i % d.Layout.Columns,
			Y: i / d.Layout.Columns,
		}
		d.Widgets[i] = w
	}

	if err := h.manager.UpdateDashboard(d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "布局已重置",
		"data":    d,
	})
}

// GetDashboardState 获取仪表板状态
// @Summary 获取仪表板状态
// @Description 获取仪表板实时状态和所有小组件数据
// @Tags dashboard
// @Produce json
// @Param id path string true "仪表板ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/state [get]
func (h *DashboardHandlers) GetDashboardState(c *gin.Context) {
	id := c.Param("id")

	state, err := h.manager.GetDashboardState(id)
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
		"data":    state,
	})
}

// GetWidgetData 获取小组件数据
// @Summary 获取小组件数据
// @Description 获取指定小组件的实时数据
// @Tags dashboard
// @Produce json
// @Param id path string true "仪表板ID"
// @Param widgetId path string true "小组件ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/widgets/{widgetId}/data [get]
func (h *DashboardHandlers) GetWidgetData(c *gin.Context) {
	dashboardID := c.Param("id")
	widgetID := c.Param("widgetId")

	data, err := h.manager.GetWidgetData(dashboardID, widgetID)
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
		"data":    data,
	})
}

// RefreshDashboard 刷新仪表板
// @Summary 刷新仪表板
// @Description 手动刷新仪表板数据
// @Tags dashboard
// @Param id path string true "仪表板ID"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/{id}/refresh [post]
func (h *DashboardHandlers) RefreshDashboard(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RefreshDashboard(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	state, _ := h.manager.GetDashboardState(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "刷新成功",
		"data":    state,
	})
}

// ExportDashboard 导出仪表板
// @Summary 导出仪表板
// @Description 导出仪表板配置为JSON
// @Tags dashboard
// @Produce json
// @Param id path string true "仪表板ID"
// @Success 200 {string} string "JSON配置"
// @Router /dashboards/{id}/export [get]
func (h *DashboardHandlers) ExportDashboard(c *gin.Context) {
	id := c.Param("id")

	data, err := h.manager.ExportDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

// ImportDashboard 导入仪表板
// @Summary 导入仪表板
// @Description 从JSON配置导入仪表板
// @Tags dashboard
// @Accept json
// @Produce json
// @Param request body string true "仪表板JSON配置"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/import [post]
func (h *DashboardHandlers) ImportDashboard(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无法读取请求体",
		})
		return
	}

	d, err := h.manager.ImportDashboard(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "仪表板导入成功",
		"data":    d,
	})
}

// GetTemplates 获取仪表板模板
// @Summary 获取仪表板模板
// @Description 获取可用的仪表板模板列表
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/templates [get]
func (h *DashboardHandlers) GetTemplates(c *gin.Context) {
	templates := []map[string]interface{}{
		{
			"id":          "system-monitor",
			"name":        "系统监控",
			"description": "CPU、内存、磁盘、网络基础监控",
			"category":    "monitoring",
		},
		{
			"id":          "storage-overview",
			"name":        "存储概览",
			"description": "存储使用、磁盘健康、IO性能",
			"category":    "storage",
		},
		{
			"id":          "network-traffic",
			"name":        "网络流量",
			"description": "网络接口流量、连接状态",
			"category":    "network",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    templates,
	})
}

// CreateFromTemplate 从模板创建仪表板
// @Summary 从模板创建仪表板
// @Description 使用预定义模板创建仪表板
// @Tags dashboard
// @Accept json
// @Produce json
// @Param request body CreateFromTemplateRequest true "模板参数"
// @Success 200 {object} map[string]interface{}
// @Router /dashboards/from-template [post]
func (h *DashboardHandlers) CreateFromTemplate(c *gin.Context) {
	var req struct {
		TemplateID string `json:"templateId" binding:"required"`
		Name       string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	d, err := h.manager.CreateDashboard(req.Name, "从模板创建")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 根据模板添加小组件
	var widgets []*dashboard.Widget
	switch req.TemplateID {
	case "system-monitor":
		widgets = dashboard.CreateDefaultWidgets()
	case "storage-overview":
		widgets = []*dashboard.Widget{
			{
				Type: dashboard.WidgetTypeDisk, Title: "磁盘使用", Size: dashboard.WidgetSizeLarge,
				Position: dashboard.WidgetPosition{X: 0, Y: 0}, Enabled: true, RefreshRate: 30 * time.Second,
			},
		}
	case "network-traffic":
		widgets = []*dashboard.Widget{
			{
				Type: dashboard.WidgetTypeNetwork, Title: "网络流量", Size: dashboard.WidgetSizeLarge,
				Position: dashboard.WidgetPosition{X: 0, Y: 0}, Enabled: true, RefreshRate: 5 * time.Second,
			},
		}
	default:
		widgets = dashboard.CreateDefaultWidgets()
	}

	for _, w := range widgets {
		_ = h.manager.AddWidget(d.ID, w)
	}

	d, _ = h.manager.GetDashboard(d.ID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "仪表板创建成功",
		"data":    d,
	})
}

// GetDefaultDashboard 获取默认仪表板
// @Summary 获取默认仪表板
// @Description 获取系统默认仪表板
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /dashboard/default [get]
func (h *DashboardHandlers) GetDefaultDashboard(c *gin.Context) {
	dashboards := h.manager.ListDashboards()

	for _, d := range dashboards {
		if d.IsDefault {
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "success",
				"data":    d,
			})
			return
		}
	}

	// 创建默认仪表板
	d, err := h.manager.CreateDefaultDashboard()
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
		"data":    d,
	})
}

// GetDashboardStats 获取仪表板统计
// @Summary 获取仪表板统计
// @Description 获取仪表板统计信息
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /dashboard/stats [get]
func (h *DashboardHandlers) GetDashboardStats(c *gin.Context) {
	dashboards := h.manager.ListDashboards()

	totalWidgets := 0
	for _, d := range dashboards {
		totalWidgets += len(d.Widgets)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalDashboards": len(dashboards),
			"totalWidgets":    totalWidgets,
		},
	})
}

// GetWidgetTypes 获取小组件类型
// @Summary 获取小组件类型
// @Description 获取所有可用的小组件类型
// @Tags dashboard
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /dashboard/widgets/types [get]
func (h *DashboardHandlers) GetWidgetTypes(c *gin.Context) {
	types := h.manager.GetWidgetTypes()

	typeInfo := make([]map[string]interface{}, 0, len(types))
	for _, t := range types {
		info := map[string]interface{}{
			"type": t,
			"name": getWidgetTypeName(t),
		}
		typeInfo = append(typeInfo, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    typeInfo,
	})
}

// HandleDashboardWebSocket WebSocket 处理
// @Summary Dashboard WebSocket
// @Description WebSocket 实时数据推送
// @Tags dashboard
// @Router /dashboard/ws [get]
func (h *DashboardHandlers) HandleDashboardWebSocket(c *gin.Context) {
	// 这里应该使用现有的 WebSocket Hub
	// 暂时返回提示
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "WebSocket 端点请使用 /api/v1/system/ws",
	})
}

func getWidgetTypeName(t dashboard.WidgetType) string {
	switch t {
	case dashboard.WidgetTypeCPU:
		return "CPU 监控"
	case dashboard.WidgetTypeMemory:
		return "内存监控"
	case dashboard.WidgetTypeDisk:
		return "磁盘监控"
	case dashboard.WidgetTypeNetwork:
		return "网络监控"
	case dashboard.WidgetTypeCustom:
		return "自定义组件"
	default:
		return string(t)
	}
}
