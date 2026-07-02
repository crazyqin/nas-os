package dashboardwidget

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WidgetType 组件类型.
type WidgetType string

const (
	WidgetTypeChart    WidgetType = "chart"
	WidgetTypeGauge    WidgetType = "gauge"
	WidgetTypeList     WidgetType = "list"
	WidgetTypeStat     WidgetType = "stat"
	WidgetTypeTimeline WidgetType = "timeline"
	WidgetTypeMap      WidgetType = "map"
)

// Widget 组件.
type Widget struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       WidgetType        `json:"type"`
	Position   Position          `json:"position"`
	Size       Size              `json:"size"`
	Config     map[string]string `json:"config"`
	DataSource string            `json:"data_source"`
	RefreshSec int               `json:"refresh_sec"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Position 位置.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Size 尺寸.
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Dashboard 仪表盘.
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Widgets     []string  `json:"widgets"`
	IsDefault   bool      `json:"is_default"`
	Layout      string    `json:"layout"` // grid, free
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Manager 仪表盘管理器.
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	widgets    map[string]*Widget
	dashboards map[string]*Dashboard
	dataPath   string
}

// NewManager 创建管理器.
func NewManager(logger *zap.Logger, dataPath string) *Manager {
	m := &Manager{
		logger:     logger,
		widgets:    make(map[string]*Widget),
		dashboards: make(map[string]*Dashboard),
		dataPath:   dataPath,
	}
	_ = m.loadData()
	return m
}

// CreateWidget 创建组件.
func (m *Manager) CreateWidget(name string, wType WidgetType, config map[string]string) *Widget {
	m.mu.Lock()
	defer m.mu.Unlock()
	widget := &Widget{
		ID:         genID(),
		Name:       name,
		Type:       wType,
		Size:       Size{Width: 4, Height: 3},
		Config:     config,
		RefreshSec: 30,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.widgets[widget.ID] = widget
	_ = m.saveData()
	return widget
}

// UpdateWidget 更新组件.
func (m *Manager) UpdateWidget(id string, pos *Position, size *Size) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.widgets[id]
	if !ok {
		return fmt.Errorf("widget not found: %s", id)
	}
	if pos != nil {
		w.Position = *pos
	}
	if size != nil {
		w.Size = *size
	}
	w.UpdatedAt = time.Now()
	_ = m.saveData()
	return nil
}

// DeleteWidget 删除组件.
func (m *Manager) DeleteWidget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.widgets[id]; !ok {
		return fmt.Errorf("widget not found: %s", id)
	}
	delete(m.widgets, id)
	for _, d := range m.dashboards {
		filtered := d.Widgets[:0]
		for _, wid := range d.Widgets {
			if wid != id {
				filtered = append(filtered, wid)
			}
		}
		d.Widgets = filtered
	}
	_ = m.saveData()
	return nil
}

// CreateDashboard 创建仪表盘.
func (m *Manager) CreateDashboard(name, desc, layout string) *Dashboard {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := &Dashboard{
		ID:          genID(),
		Name:        name,
		Description: desc,
		Layout:      layout,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if layout == "" {
		d.Layout = "grid"
	}
	m.dashboards[d.ID] = d
	_ = m.saveData()
	return d
}

// AddWidgetToDashboard 添加组件到仪表盘.
func (m *Manager) AddWidgetToDashboard(dashboardID, widgetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard not found: %s", dashboardID)
	}
	if _, ok := m.widgets[widgetID]; !ok {
		return fmt.Errorf("widget not found: %s", widgetID)
	}
	for _, wid := range d.Widgets {
		if wid == widgetID {
			return nil
		}
	}
	d.Widgets = append(d.Widgets, widgetID)
	d.UpdatedAt = time.Now()
	_ = m.saveData()
	return nil
}

// ListDashboards 列出仪表盘.
func (m *Manager) ListDashboards() []*Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Dashboard
	for _, d := range m.dashboards {
		result = append(result, d)
	}
	return result
}

// GetDashboard 获取仪表盘详情.
func (m *Manager) GetDashboard(id string) (*Dashboard, map[string]*Widget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.dashboards[id]
	if !ok {
		return nil, nil, fmt.Errorf("dashboard not found")
	}
	widgets := make(map[string]*Widget)
	for _, wid := range d.Widgets {
		if w, ok := m.widgets[wid]; ok {
			widgets[wid] = w
		}
	}
	return d, widgets, nil
}

// GetSystemWidgets 获取系统预置组件数据.
func (m *Manager) GetSystemWidgets() []map[string]interface{} {
	return []map[string]interface{}{
		{"name": "CPU使用率", "type": "gauge", "data_source": "system.cpu"},
		{"name": "内存使用", "type": "gauge", "data_source": "system.memory"},
		{"name": "磁盘空间", "type": "chart", "data_source": "system.disk"},
		{"name": "网络流量", "type": "timeline", "data_source": "system.network"},
		{"name": "最近文件", "type": "list", "data_source": "files.recent"},
		{"name": "容器状态", "type": "stat", "data_source": "docker.status"},
	}
}

func (m *Manager) loadData() error {
	if m.dataPath == "" {
		return nil
	}
	dataFile := filepath.Join(m.dataPath, "dashboard.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Widgets    map[string]*Widget    `json:"widgets"`
		Dashboards map[string]*Dashboard `json:"dashboards"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Widgets != nil {
		m.widgets = stored.Widgets
	}
	if stored.Dashboards != nil {
		m.dashboards = stored.Dashboards
	}
	return nil
}

func (m *Manager) saveData() error {
	if m.dataPath == "" {
		return nil
	}
	_ = os.MkdirAll(m.dataPath, 0o755)
	stored := struct {
		Widgets    map[string]*Widget    `json:"widgets"`
		Dashboards map[string]*Dashboard `json:"dashboards"`
	}{Widgets: m.widgets, Dashboards: m.dashboards}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dataPath, "dashboard.json"), data, 0o644)
}

func genID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// Handlers API.
type Handlers struct {
	mgr *Manager
}

func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	dash := rg.Group("/dashboard")
	{
		dash.GET("/dashboards", h.listDashboards)
		dash.POST("/dashboards", h.createDashboard)
		dash.GET("/dashboards/:id", h.getDashboard)
		dash.POST("/dashboards/:id/widgets", h.addWidget)
		dash.POST("/widgets", h.createWidget)
		dash.PUT("/widgets/:id", h.updateWidget)
		dash.DELETE("/widgets/:id", h.deleteWidget)
		dash.GET("/system-widgets", h.getSystemWidgets)
	}
}

func (h *Handlers) listDashboards(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"dashboards": h.mgr.ListDashboards()})
}

func (h *Handlers) createDashboard(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Layout      string `json:"layout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d := h.mgr.CreateDashboard(req.Name, req.Description, req.Layout)
	c.JSON(http.StatusCreated, d)
}

func (h *Handlers) getDashboard(c *gin.Context) {
	id := c.Param("id")
	d, widgets, err := h.mgr.GetDashboard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"dashboard": d, "widgets": widgets})
}

func (h *Handlers) addWidget(c *gin.Context) {
	dashID := c.Param("id")
	var req struct {
		WidgetID string `json:"widget_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.mgr.AddWidgetToDashboard(dashID, req.WidgetID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) createWidget(c *gin.Context) {
	var req struct {
		Name       string            `json:"name" binding:"required"`
		Type       WidgetType        `json:"type" binding:"required"`
		Config     map[string]string `json:"config"`
		DataSource string            `json:"data_source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	w := h.mgr.CreateWidget(req.Name, req.Type, req.Config)
	if req.DataSource != "" {
		w.DataSource = req.DataSource
	}
	c.JSON(http.StatusCreated, w)
}

func (h *Handlers) updateWidget(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Position *Position `json:"position"`
		Size     *Size     `json:"size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.mgr.UpdateWidget(id, req.Position, req.Size); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) deleteWidget(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteWidget(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) getSystemWidgets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"widgets": h.mgr.GetSystemWidgets()})
}
