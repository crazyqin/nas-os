package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/internal/monitor"
)

// Manager 仪表板管理器
type Manager struct {
	mu             sync.RWMutex
	registry       *WidgetRegistry
	dashboards     map[string]*Dashboard
	states         map[string]*DashboardState
	monitorManager *monitor.Manager
	dataDir        string
	refreshTicker  *time.Ticker
	stopChan       chan struct{}
	running        bool
	subscribers    []chan *DashboardEvent
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	DataDir        string
	RefreshRate    time.Duration
	MonitorManager *monitor.Manager
}

// NewManager 创建仪表板管理器
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	if cfg.RefreshRate == 0 {
		cfg.RefreshRate = 5 * time.Second
	}

	m := &Manager{
		dashboards:     make(map[string]*Dashboard),
		states:         make(map[string]*DashboardState),
		monitorManager: cfg.MonitorManager,
		dataDir:        cfg.DataDir,
		stopChan:       make(chan struct{}),
		subscribers:    make([]chan *DashboardEvent, 0),
	}

	// 创建小组件注册表
	if m.monitorManager != nil {
		m.registry = DefaultWidgetRegistry(m.monitorManager)
	} else {
		m.registry = NewWidgetRegistry()
	}

	// 确保数据目录存在
	if m.dataDir != "" {
		if err := os.MkdirAll(m.dataDir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
		// 加载已保存的仪表板
		if err := m.loadDashboards(); err != nil {
			// 非致命错误，记录即可
			fmt.Printf("加载仪表板失败: %v\n", err)
		}
	}

	return m, nil
}

// Start 启动管理器
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.refreshTicker = time.NewTicker(5 * time.Second)
	m.mu.Unlock()

	go m.refreshLoop()
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		close(m.stopChan)
		if m.refreshTicker != nil {
			m.refreshTicker.Stop()
		}
		m.running = false

		// 保存仪表板
		_ = m.saveDashboards()
	}
}

// refreshLoop 刷新循环
func (m *Manager) refreshLoop() {
	for {
		select {
		case <-m.stopChan:
			return
		case <-m.refreshTicker.C:
			m.refreshAllWidgets()
		}
	}
}

// refreshAllWidgets 刷新所有小组件
func (m *Manager) refreshAllWidgets() {
	m.mu.RLock()
	dashboards := make([]*Dashboard, 0, len(m.dashboards))
	for _, d := range m.dashboards {
		dashboards = append(dashboards, d)
	}
	m.mu.RUnlock()

	for _, dashboard := range dashboards {
		m.refreshDashboardWidgets(dashboard)
	}
}

// refreshDashboardWidgets 刷新仪表板小组件
func (m *Manager) refreshDashboardWidgets(dashboard *Dashboard) {
	now := time.Now()

	state := &DashboardState{
		DashboardID: dashboard.ID,
		LastUpdate:  now,
		WidgetData:  make(map[string]*WidgetData),
		Status:      "healthy",
	}

	var healthScores []float64

	for _, widget := range dashboard.Widgets {
		if !widget.Enabled {
			continue
		}

		data, err := m.getWidgetData(widget)
		if err != nil {
			data = &WidgetData{
				WidgetID:  widget.ID,
				Type:      widget.Type,
				Timestamp: now,
				Error:     err.Error(),
			}
		}

		state.WidgetData[widget.ID] = data

		// 计算健康评分
		if data.Error == "" {
			status := GetWidgetStatus(data, widget.Config)
			switch status {
			case "critical":
				healthScores = append(healthScores, 0)
				state.Status = "critical"
			case "warning":
				healthScores = append(healthScores, 50)
				if state.Status == "healthy" {
					state.Status = "warning"
				}
			case "healthy":
				healthScores = append(healthScores, 100)
			}
		}
	}

	// 计算平均健康评分
	if len(healthScores) > 0 {
		var total float64
		for _, s := range healthScores {
			total += s
		}
		state.HealthScore = total / float64(len(healthScores))
	}

	m.mu.Lock()
	m.states[dashboard.ID] = state
	m.mu.Unlock()

	// 发布事件
	m.publishEvent(&DashboardEvent{
		Type:        "refresh",
		DashboardID: dashboard.ID,
		Timestamp:   now,
		Data:        state,
	})
}

// getWidgetData 获取小组件数据
func (m *Manager) getWidgetData(widget *Widget) (*WidgetData, error) {
	provider, ok := m.registry.Get(widget.Type)
	if !ok {
		return nil, fmt.Errorf("未找到小组件类型: %s", widget.Type)
	}

	return provider.GetData(widget)
}

// CreateDashboard 创建仪表板
func (m *Manager) CreateDashboard(name, description string) (*Dashboard, error) {
	var dashboard *Dashboard
	var subscribers []chan *DashboardEvent

	m.mu.Lock()
	now := time.Now()
	dashboard = &Dashboard{
		ID:          generateID(),
		Name:        name,
		Description: description,
		Widgets:     make([]*Widget, 0),
		Layout: Layout{
			Columns: 2,
			Rows:    2,
			Gap:     10,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.dashboards[dashboard.ID] = dashboard
	_ = m.saveDashboards()

	// 在锁内复制订阅者列表
	subscribers = m.getSubscribersInternal()
	m.mu.Unlock()

	// 在锁外发布事件
	event := &DashboardEvent{
		Type:        "create",
		DashboardID: dashboard.ID,
		Timestamp:   now,
		Data:        dashboard,
	}
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
			// channel 满，跳过
		}
	}

	return dashboard, nil
}

// GetDashboard 获取仪表板
func (m *Manager) GetDashboard(id string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, ok := m.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("仪表板不存在: %s", id)
	}

	return dashboard, nil
}

// ListDashboards 列出仪表板
func (m *Manager) ListDashboards() []*Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Dashboard, 0, len(m.dashboards))
	for _, d := range m.dashboards {
		result = append(result, d)
	}
	return result
}

// UpdateDashboard 更新仪表板
func (m *Manager) UpdateDashboard(dashboard *Dashboard) error {
	var subscribers []chan *DashboardEvent

	m.mu.Lock()
	if _, ok := m.dashboards[dashboard.ID]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("仪表板不存在: %s", dashboard.ID)
	}

	dashboard.UpdatedAt = time.Now()
	m.dashboards[dashboard.ID] = dashboard
	_ = m.saveDashboards()
	subscribers = m.getSubscribersInternal()
	m.mu.Unlock()

	// 在锁外发布事件
	event := &DashboardEvent{
		Type:        "update",
		DashboardID: dashboard.ID,
		Timestamp:   time.Now(),
		Data:        dashboard,
	}
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}

	return nil
}

// DeleteDashboard 删除仪表板
func (m *Manager) DeleteDashboard(id string) error {
	var subscribers []chan *DashboardEvent

	m.mu.Lock()
	if _, ok := m.dashboards[id]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("仪表板不存在: %s", id)
	}

	delete(m.dashboards, id)
	delete(m.states, id)
	_ = m.saveDashboards()
	subscribers = m.getSubscribersInternal()
	m.mu.Unlock()

	// 在锁外发布事件
	event := &DashboardEvent{
		Type:        "delete",
		DashboardID: id,
		Timestamp:   time.Now(),
	}
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}

	return nil
}

// AddWidget 添加小组件
func (m *Manager) AddWidget(dashboardID string, widget *Widget) error {
	var subscribers []chan *DashboardEvent
	var now time.Time

	m.mu.Lock()
	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("仪表板不存在: %s", dashboardID)
	}

	now = time.Now()
	widget.CreatedAt = now
	widget.UpdatedAt = now

	if widget.ID == "" {
		widget.ID = generateID()
	}

	dashboard.Widgets = append(dashboard.Widgets, widget)
	dashboard.UpdatedAt = now
	_ = m.saveDashboards()
	subscribers = m.getSubscribersInternal()
	m.mu.Unlock()

	// 在锁外发布事件
	event := &DashboardEvent{
		Type:        "widget_add",
		DashboardID: dashboardID,
		WidgetID:    widget.ID,
		Timestamp:   now,
		Data:        widget,
	}
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}

	return nil
}

// RemoveWidget 移除小组件
func (m *Manager) RemoveWidget(dashboardID, widgetID string) error {
	var subscribers []chan *DashboardEvent

	m.mu.Lock()
	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("仪表板不存在: %s", dashboardID)
	}

	for i, w := range dashboard.Widgets {
		if w.ID == widgetID {
			dashboard.Widgets = append(dashboard.Widgets[:i], dashboard.Widgets[i+1:]...)
			dashboard.UpdatedAt = time.Now()
			_ = m.saveDashboards()
			subscribers = m.getSubscribersInternal()
			m.mu.Unlock()

			// 在锁外发布事件
			event := &DashboardEvent{
				Type:        "widget_remove",
				DashboardID: dashboardID,
				WidgetID:    widgetID,
				Timestamp:   time.Now(),
			}
			for _, ch := range subscribers {
				select {
				case ch <- event:
				default:
				}
			}

			return nil
		}
	}
	m.mu.Unlock()
	return fmt.Errorf("小组件不存在: %s", widgetID)
}

// UpdateWidget 更新小组件
func (m *Manager) UpdateWidget(dashboardID string, widget *Widget) error {
	var subscribers []chan *DashboardEvent

	m.mu.Lock()
	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("仪表板不存在: %s", dashboardID)
	}

	for i, w := range dashboard.Widgets {
		if w.ID == widget.ID {
			widget.UpdatedAt = time.Now()
			dashboard.Widgets[i] = widget
			dashboard.UpdatedAt = time.Now()
			_ = m.saveDashboards()
			subscribers = m.getSubscribersInternal()
			m.mu.Unlock()

			// 在锁外发布事件
			event := &DashboardEvent{
				Type:        "widget_update",
				DashboardID: dashboardID,
				WidgetID:    widget.ID,
				Timestamp:   time.Now(),
				Data:        widget,
			}
			for _, ch := range subscribers {
				select {
				case ch <- event:
				default:
				}
			}

			return nil
		}
	}
	m.mu.Unlock()
	return fmt.Errorf("小组件不存在: %s", widget.ID)
}

// GetWidgetData 获取小组件数据
func (m *Manager) GetWidgetData(dashboardID, widgetID string) (*WidgetData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.states[dashboardID]
	if !ok {
		return nil, fmt.Errorf("仪表板状态不存在: %s", dashboardID)
	}

	data, ok := state.WidgetData[widgetID]
	if !ok {
		return nil, fmt.Errorf("小组件数据不存在: %s", widgetID)
	}

	return data, nil
}

// GetDashboardState 获取仪表板状态
func (m *Manager) GetDashboardState(dashboardID string) (*DashboardState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.states[dashboardID]
	if !ok {
		return nil, fmt.Errorf("仪表板状态不存在: %s", dashboardID)
	}

	return state, nil
}

// RefreshDashboard 刷新仪表板
func (m *Manager) RefreshDashboard(dashboardID string) error {
	m.mu.RLock()
	dashboard, ok := m.dashboards[dashboardID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("仪表板不存在: %s", dashboardID)
	}

	m.refreshDashboardWidgets(dashboard)
	return nil
}

// CreateDefaultDashboard 创建默认仪表板
func (m *Manager) CreateDefaultDashboard() (*Dashboard, error) {
	dashboard, err := m.CreateDashboard("系统监控", "默认系统监控仪表板")
	if err != nil {
		return nil, err
	}

	// 添加默认小组件
	widgets := CreateDefaultWidgets()
	for _, widget := range widgets {
		if err := m.AddWidget(dashboard.ID, widget); err != nil {
			fmt.Printf("添加小组件失败: %v\n", err)
		}
	}

	dashboard.IsDefault = true
	_ = m.UpdateDashboard(dashboard)

	return dashboard, nil
}

// Subscribe 订阅事件
func (m *Manager) Subscribe() chan *DashboardEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *DashboardEvent, 100)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

// Unsubscribe 取消订阅
func (m *Manager) Unsubscribe(ch chan *DashboardEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, sub := range m.subscribers {
		if sub == ch {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// publishEvent 发布事件（调用时不应持有锁）
func (m *Manager) publishEvent(event *DashboardEvent) {
	subscribers := m.getSubscribers()
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
			// channel 满，跳过
		}
	}
}

// getSubscribers 获取订阅者列表副本（线程安全）
func (m *Manager) getSubscribers() []chan *DashboardEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	subscribers := make([]chan *DashboardEvent, len(m.subscribers))
	copy(subscribers, m.subscribers)
	return subscribers
}

// getSubscribersInternal 获取订阅者列表副本（内部使用，不获取锁）
func (m *Manager) getSubscribersInternal() []chan *DashboardEvent {
	subscribers := make([]chan *DashboardEvent, len(m.subscribers))
	copy(subscribers, m.subscribers)
	return subscribers
}

// saveDashboards 保存仪表板
func (m *Manager) saveDashboards() error {
	if m.dataDir == "" {
		return nil
	}

	data, err := json.MarshalIndent(m.dashboards, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化仪表板失败: %w", err)
	}

	path := filepath.Join(m.dataDir, "dashboards.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("保存仪表板失败: %w", err)
	}

	return nil
}

// loadDashboards 加载仪表板
func (m *Manager) loadDashboards() error {
	if m.dataDir == "" {
		return nil
	}

	path := filepath.Join(m.dataDir, "dashboards.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取仪表板失败: %w", err)
	}

	if err := json.Unmarshal(data, &m.dashboards); err != nil {
		return fmt.Errorf("解析仪表板失败: %w", err)
	}

	return nil
}

// ExportDashboard 导出仪表板
func (m *Manager) ExportDashboard(dashboardID string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("仪表板不存在: %s", dashboardID)
	}

	return json.MarshalIndent(dashboard, "", "  ")
}

// ImportDashboard 导入仪表板
func (m *Manager) ImportDashboard(data []byte) (*Dashboard, error) {
	var dashboard Dashboard
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return nil, fmt.Errorf("解析仪表板失败: %w", err)
	}

	// 生成新 ID
	dashboard.ID = generateID()
	dashboard.CreatedAt = time.Now()
	dashboard.UpdatedAt = time.Now()

	m.mu.Lock()
	m.dashboards[dashboard.ID] = &dashboard
	_ = m.saveDashboards()
	m.mu.Unlock()

	m.publishEvent(&DashboardEvent{
		Type:        "import",
		DashboardID: dashboard.ID,
		Timestamp:   time.Now(),
		Data:        &dashboard,
	})

	return &dashboard, nil
}

// CloneDashboard 克隆仪表板
func (m *Manager) CloneDashboard(dashboardID string, name string) (*Dashboard, error) {
	m.mu.RLock()
	original, ok := m.dashboards[dashboardID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("仪表板不存在: %s", dashboardID)
	}

	now := time.Now()
	clone := &Dashboard{
		ID:          generateID(),
		Name:        name,
		Description: original.Description,
		Widgets:     make([]*Widget, 0, len(original.Widgets)),
		Layout:      original.Layout,
		IsDefault:   false,
		OwnerID:     original.OwnerID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 克隆小组件
	for _, w := range original.Widgets {
		widget := *w
		widget.ID = generateID()
		widget.CreatedAt = now
		widget.UpdatedAt = now
		clone.Widgets = append(clone.Widgets, &widget)
	}

	m.mu.Lock()
	m.dashboards[clone.ID] = clone
	_ = m.saveDashboards()
	m.mu.Unlock()

	m.publishEvent(&DashboardEvent{
		Type:        "clone",
		DashboardID: clone.ID,
		Timestamp:   now,
		Data:        clone,
	})

	return clone, nil
}

// GetWidgetTypes 获取可用小组件类型
func (m *Manager) GetWidgetTypes() []WidgetType {
	return m.registry.GetAvailableTypes()
}

// RunWithContext 带 Context 运行
func (m *Manager) RunWithContext(ctx context.Context) {
	m.Start()

	<-ctx.Done()
	m.Stop()
}

// generateID 生成 ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
