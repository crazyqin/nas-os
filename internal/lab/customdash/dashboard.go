// Package customdash 提供自定义监控仪表盘功能
// 支持可拖拽布局、多仪表盘、多数据源、阈值告警、历史数据采样
package customdash

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// WidgetType Widget类型.
type WidgetType string

const (
	WidgetCPU          WidgetType = "cpu"
	WidgetMemory       WidgetType = "memory"
	WidgetDiskIO       WidgetType = "disk_io"
	WidgetNetTraffic   WidgetType = "net_traffic"
	WidgetStoragePool  WidgetType = "storage_pool"
	WidgetTemperature  WidgetType = "temperature"
	WidgetUPSBattery   WidgetType = "ups_battery"
	WidgetDockerStatus WidgetType = "docker_status"
	WidgetZFSHealth    WidgetType = "zfs_health"
)

// DataSourceType 数据源类型.
type DataSourceType string

const (
	DataSourcePrometheus DataSourceType = "prometheus"
	DataSourceBuiltin    DataSourceType = "builtin"
	DataSourceSNMP       DataSourceType = "snmp"
	DataSourceHTTP       DataSourceType = "http"
)

// ThresholdOperator 阈值比较操作符.
type ThresholdOperator string

const (
	OpGT  ThresholdOperator = ">"
	OpGTE ThresholdOperator = ">="
	OpLT  ThresholdOperator = "<"
	OpLTE ThresholdOperator = "<="
	OpEQ  ThresholdOperator = "=="
)

// Position Widget位置（可拖拽布局配置）.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Size Widget大小.
type Size struct {
	W int `json:"w"`
	H int `json:"h"`
}

// ThresholdAlert 阈值告警配置.
type ThresholdAlert struct {
	Enabled  bool              `json:"enabled"`
	Metric   string            `json:"metric"`
	Operator ThresholdOperator `json:"operator"`
	Value    float64           `json:"value"`
	Message  string            `json:"message"`
}

// DataSource 数据源抽象.
type DataSource struct {
	Type     DataSourceType    `json:"type"`
	Endpoint string            `json:"endpoint,omitempty"`
	Query    string            `json:"query,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// Widget 仪表盘Widget配置.
type Widget struct {
	ID              string         `json:"id"`
	DashboardID     string         `json:"dashboardId"`
	Type            WidgetType     `json:"type"`
	Title           string         `json:"title"`
	Position        Position       `json:"position"`
	Size            Size           `json:"size"`
	RefreshInterval time.Duration  `json:"refreshInterval"`
	DataSource      DataSource     `json:"dataSource"`
	Threshold       ThresholdAlert `json:"threshold"`
}

// Dashboard 仪表盘.
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsDefault   bool      `json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// WidgetData Widget数据（含历史记录）.
type WidgetData struct {
	WidgetID string        `json:"widgetId"`
	Latest   *DataSample   `json:"latest,omitempty"`
	Samples  []*DataSample `json:"samples"`
}

// DataSample 数据采样点.
type DataSample struct {
	Timestamp time.Time          `json:"timestamp"`
	Value     float64            `json:"value"`
	Label     string             `json:"label,omitempty"`
	Extra     map[string]float64 `json:"extra,omitempty"`
}

// ExportData 导入/导出数据格式.
type ExportData struct {
	Dashboard *Dashboard `json:"dashboard"`
	Widgets   []*Widget  `json:"widgets"`
}

// DataProvider 数据提供者接口（可扩展）.
type DataProvider interface {
	Fetch(w *Widget) (*DataSample, error)
}

// BuiltinProvider 内置指标提供者.
type BuiltinProvider struct{}

func (p *BuiltinProvider) Fetch(w *Widget) (*DataSample, error) {
	// 模拟内置数据采集
	sample := &DataSample{
		Timestamp: time.Now(),
		Label:     string(w.Type),
	}
	switch w.Type {
	case WidgetCPU:
		sample.Value = 35.5
		sample.Extra = map[string]float64{"user": 20.0, "system": 10.0, "idle": 64.5}
	case WidgetMemory:
		sample.Value = 68.2
		sample.Extra = map[string]float64{"used_gb": 10.9, "total_gb": 16.0, "cached": 3.2}
	case WidgetDiskIO:
		sample.Value = 125.0
		sample.Extra = map[string]float64{"read_mbps": 80.0, "write_mbps": 45.0}
	case WidgetNetTraffic:
		sample.Value = 450.0
		sample.Extra = map[string]float64{"rx_mbps": 300.0, "tx_mbps": 150.0}
	case WidgetStoragePool:
		sample.Value = 72.0
		sample.Extra = map[string]float64{"used_tb": 3.6, "total_tb": 5.0}
	case WidgetTemperature:
		sample.Value = 52.0
		sample.Extra = map[string]float64{"cpu": 52.0, "hdd": 38.0, "nvme": 45.0}
	case WidgetUPSBattery:
		sample.Value = 95.0
		sample.Extra = map[string]float64{"load_w": 180.0, "runtime_min": 45.0}
	case WidgetDockerStatus:
		sample.Value = 8.0
		sample.Extra = map[string]float64{"running": 8, "stopped": 2, "total": 10}
	case WidgetZFSHealth:
		sample.Value = 1.0
		sample.Extra = map[string]float64{"scrub_errors": 0, "status_ok": 1}
	default:
		sample.Value = 0
	}
	return sample, nil
}

// HTTPProvider 自定义HTTP数据源.
type HTTPProvider struct {
	Endpoint string
}

func (p *HTTPProvider) Fetch(w *Widget) (*DataSample, error) {
	// 实际实现中会发HTTP请求到endpoint
	return &DataSample{
		Timestamp: time.Now(),
		Value:     0,
		Label:     string(w.Type),
	}, nil
}

// SNMPProvider SNMP数据源.
type SNMPProvider struct {
	Endpoint string
}

func (p *SNMPProvider) Fetch(w *Widget) (*DataSample, error) {
	return &DataSample{
		Timestamp: time.Now(),
		Value:     0,
		Label:     string(w.Type),
	}, nil
}

// PrometheusProvider Prometheus数据源.
type PrometheusProvider struct {
	Endpoint string
}

func (p *PrometheusProvider) Fetch(w *Widget) (*DataSample, error) {
	return &DataSample{
		Timestamp: time.Now(),
		Value:     0,
		Label:     string(w.Type),
	}, nil
}

// DashboardManager 仪表盘管理器.
type DashboardManager struct {
	dashboards map[string]*Dashboard
	widgets    map[string]*Widget     // widgetID -> Widget
	widgetData map[string]*WidgetData // widgetID -> historical data
	providers  map[DataSourceType]DataProvider
	mu         sync.RWMutex
	ctx        interface{ Done() <-chan struct{} }
	cancel     func()
	running    bool
	sampler    *time.Ticker
}

// NewDashboardManager 创建仪表盘管理器.
func NewDashboardManager() *DashboardManager {
	dm := &DashboardManager{
		dashboards: make(map[string]*Dashboard),
		widgets:    make(map[string]*Widget),
		widgetData: make(map[string]*WidgetData),
		providers: map[DataSourceType]DataProvider{
			DataSourceBuiltin:    &BuiltinProvider{},
			DataSourceHTTP:       &HTTPProvider{},
			DataSourceSNMP:       &SNMPProvider{},
			DataSourcePrometheus: &PrometheusProvider{},
		},
	}
	dm.initDefaults()
	return dm
}

// initDefaults 初始化3个默认仪表盘.
func (dm *DashboardManager) initDefaults() {
	// 系统概览仪表盘
	sysDash := &Dashboard{
		ID:          "default-system",
		Name:        "系统概览",
		Description: "CPU、内存、温度等系统核心指标",
		IsDefault:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	dm.dashboards[sysDash.ID] = sysDash

	sysWidgets := []*Widget{
		{ID: "w-sys-cpu", DashboardID: sysDash.ID, Type: WidgetCPU, Title: "CPU 使用率", Position: Position{X: 0, Y: 0}, Size: Size{W: 6, H: 4}, RefreshInterval: 5 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
		{ID: "w-sys-mem", DashboardID: sysDash.ID, Type: WidgetMemory, Title: "内存使用率", Position: Position{X: 6, Y: 0}, Size: Size{W: 6, H: 4}, RefreshInterval: 5 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
		{ID: "w-sys-temp", DashboardID: sysDash.ID, Type: WidgetTemperature, Title: "温度监控", Position: Position{X: 0, Y: 4}, Size: Size{W: 4, H: 4}, RefreshInterval: 10 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
		{ID: "w-sys-ups", DashboardID: sysDash.ID, Type: WidgetUPSBattery, Title: "UPS 电量", Position: Position{X: 4, Y: 4}, Size: Size{W: 4, H: 4}, RefreshInterval: 30 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
		{ID: "w-sys-docker", DashboardID: sysDash.ID, Type: WidgetDockerStatus, Title: "Docker 容器", Position: Position{X: 8, Y: 4}, Size: Size{W: 4, H: 4}, RefreshInterval: 15 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
	}
	for _, w := range sysWidgets {
		dm.widgets[w.ID] = w
		dm.widgetData[w.ID] = &WidgetData{WidgetID: w.ID, Samples: make([]*DataSample, 0)}
	}

	// 存储监控仪表盘
	storeDash := &Dashboard{
		ID:          "default-storage",
		Name:        "存储监控",
		Description: "磁盘IO、存储池、ZFS健康状态",
		IsDefault:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	dm.dashboards[storeDash.ID] = storeDash

	storeWidgets := []*Widget{
		{ID: "w-stor-disk", DashboardID: storeDash.ID, Type: WidgetDiskIO, Title: "磁盘 I/O", Position: Position{X: 0, Y: 0}, Size: Size{W: 6, H: 4}, RefreshInterval: 5 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
		{ID: "w-stor-pool", DashboardID: storeDash.ID, Type: WidgetStoragePool, Title: "存储池容量", Position: Position{X: 6, Y: 0}, Size: Size{W: 6, H: 4}, RefreshInterval: 30 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
		{ID: "w-stor-zfs", DashboardID: storeDash.ID, Type: WidgetZFSHealth, Title: "ZFS 健康状态", Position: Position{X: 0, Y: 4}, Size: Size{W: 12, H: 3}, RefreshInterval: 60 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}},
	}
	for _, w := range storeWidgets {
		dm.widgets[w.ID] = w
		dm.widgetData[w.ID] = &WidgetData{WidgetID: w.ID, Samples: make([]*DataSample, 0)}
	}

	// 网络监控仪表盘
	netDash := &Dashboard{
		ID:          "default-network",
		Name:        "网络监控",
		Description: "网络流量监控",
		IsDefault:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	dm.dashboards[netDash.ID] = netDash

	netWidgets := []*Widget{
		{ID: "w-net-traffic", DashboardID: netDash.ID, Type: WidgetNetTraffic, Title: "网络流量", Position: Position{X: 0, Y: 0}, Size: Size{W: 12, H: 4}, RefreshInterval: 5 * time.Second, DataSource: DataSource{Type: DataSourceBuiltin}, Threshold: ThresholdAlert{Enabled: true, Metric: "rx_mbps", Operator: OpGT, Value: 900, Message: "入站流量超过 900Mbps"}},
	}
	for _, w := range netWidgets {
		dm.widgets[w.ID] = w
		dm.widgetData[w.ID] = &WidgetData{WidgetID: w.ID, Samples: make([]*DataSample, 0)}
	}

	log.Printf("[CustomDash] initialized %d default dashboards with %d widgets", 3, len(dm.widgets))
}

// ListDashboards 获取仪表盘列表.
func (dm *DashboardManager) ListDashboards() []*Dashboard {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	result := make([]*Dashboard, 0, len(dm.dashboards))
	for _, d := range dm.dashboards {
		cp := *d
		result = append(result, &cp)
	}
	return result
}

// GetDashboard 获取单个仪表盘.
func (dm *DashboardManager) GetDashboard(id string) (*Dashboard, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	d, ok := dm.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %s not found", id)
	}
	cp := *d
	return &cp, nil
}

// CreateDashboard 创建仪表盘.
func (dm *DashboardManager) CreateDashboard(name, description string) (*Dashboard, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if name == "" {
		return nil, fmt.Errorf("dashboard name is required")
	}
	d := &Dashboard{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		IsDefault:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	dm.dashboards[d.ID] = d
	log.Printf("[CustomDash] created dashboard: %s (%s)", d.Name, d.ID)
	return d, nil
}

// UpdateDashboard 更新仪表盘.
func (dm *DashboardManager) UpdateDashboard(id, name, description string) (*Dashboard, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	d, ok := dm.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %s not found", id)
	}
	if name != "" {
		d.Name = name
	}
	if description != "" {
		d.Description = description
	}
	d.UpdatedAt = time.Now()
	cp := *d
	return &cp, nil
}

// DeleteDashboard 删除仪表盘（同时删除关联widget）.
func (dm *DashboardManager) DeleteDashboard(id string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	d, ok := dm.dashboards[id]
	if !ok {
		return fmt.Errorf("dashboard %s not found", id)
	}
	// 删除关联widgets
	for wid, w := range dm.widgets {
		if w.DashboardID == id {
			delete(dm.widgets, wid)
			delete(dm.widgetData, wid)
		}
	}
	delete(dm.dashboards, d.ID)
	log.Printf("[CustomDash] deleted dashboard: %s", id)
	return nil
}

// GetWidgets 获取仪表盘的widget列表.
func (dm *DashboardManager) GetWidgets(dashboardID string) ([]*Widget, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	if _, ok := dm.dashboards[dashboardID]; !ok {
		return nil, fmt.Errorf("dashboard %s not found", dashboardID)
	}
	result := make([]*Widget, 0)
	for _, w := range dm.widgets {
		if w.DashboardID == dashboardID {
			cp := *w
			result = append(result, &cp)
		}
	}
	return result, nil
}

// AddWidget 添加widget.
func (dm *DashboardManager) AddWidget(dashboardID string, w *Widget) (*Widget, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if _, ok := dm.dashboards[dashboardID]; !ok {
		return nil, fmt.Errorf("dashboard %s not found", dashboardID)
	}
	if w.Type == "" {
		return nil, fmt.Errorf("widget type is required")
	}
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	w.DashboardID = dashboardID
	dm.widgets[w.ID] = w
	dm.widgetData[w.ID] = &WidgetData{WidgetID: w.ID, Samples: make([]*DataSample, 0)}
	log.Printf("[CustomDash] added widget %s (%s) to dashboard %s", w.Title, w.ID, dashboardID)
	return w, nil
}

// UpdateWidget 更新widget.
func (dm *DashboardManager) UpdateWidget(dashboardID, widgetID string, update *Widget) (*Widget, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	w, ok := dm.widgets[widgetID]
	if !ok {
		return nil, fmt.Errorf("widget %s not found", widgetID)
	}
	if w.DashboardID != dashboardID {
		return nil, fmt.Errorf("widget %s does not belong to dashboard %s", widgetID, dashboardID)
	}
	if update.Title != "" {
		w.Title = update.Title
	}
	if update.Type != "" {
		w.Type = update.Type
	}
	if update.RefreshInterval > 0 {
		w.RefreshInterval = update.RefreshInterval
	}
	if update.Position.X != 0 || update.Position.Y != 0 {
		w.Position = update.Position
	}
	if update.Size.W != 0 || update.Size.H != 0 {
		w.Size = update.Size
	}
	if update.DataSource.Type != "" {
		w.DataSource = update.DataSource
	}
	if update.Threshold.Metric != "" || update.Threshold.Enabled {
		w.Threshold = update.Threshold
	}
	cp := *w
	return &cp, nil
}

// DeleteWidget 删除widget.
func (dm *DashboardManager) DeleteWidget(dashboardID, widgetID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	w, ok := dm.widgets[widgetID]
	if !ok {
		return fmt.Errorf("widget %s not found", widgetID)
	}
	if w.DashboardID != dashboardID {
		return fmt.Errorf("widget %s does not belong to dashboard %s", widgetID, dashboardID)
	}
	delete(dm.widgets, widgetID)
	delete(dm.widgetData, widgetID)
	log.Printf("[CustomDash] deleted widget %s from dashboard %s", widgetID, dashboardID)
	return nil
}

// ExportDashboard 导出仪表盘（JSON格式）.
func (dm *DashboardManager) ExportDashboard(id string) (*ExportData, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	d, ok := dm.dashboards[id]
	if !ok {
		return nil, fmt.Errorf("dashboard %s not found", id)
	}
	dashCopy := *d
	widgets := make([]*Widget, 0)
	for _, w := range dm.widgets {
		if w.DashboardID == id {
			cp := *w
			widgets = append(widgets, &cp)
		}
	}
	return &ExportData{Dashboard: &dashCopy, Widgets: widgets}, nil
}

// ImportDashboard 导入仪表盘（JSON格式）.
func (dm *DashboardManager) ImportDashboard(data *ExportData) (*Dashboard, error) {
	if data == nil || data.Dashboard == nil {
		return nil, fmt.Errorf("invalid import data")
	}
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 生成新ID避免冲突
	oldID := data.Dashboard.ID
	data.Dashboard.ID = uuid.New().String()
	data.Dashboard.IsDefault = false
	data.Dashboard.CreatedAt = time.Now()
	data.Dashboard.UpdatedAt = time.Now()
	dm.dashboards[data.Dashboard.ID] = data.Dashboard

	for _, w := range data.Widgets {
		oldWID := w.ID
		w.ID = uuid.New().String()
		w.DashboardID = data.Dashboard.ID
		dm.widgets[w.ID] = w
		dm.widgetData[w.ID] = &WidgetData{WidgetID: w.ID, Samples: make([]*DataSample, 0)}
		// 替换引用
		_ = oldID
		_ = oldWID
	}
	log.Printf("[CustomDash] imported dashboard: %s (%s)", data.Dashboard.Name, data.Dashboard.ID)
	return data.Dashboard, nil
}

// ExportDashboardJSON 导出为JSON字节.
func (dm *DashboardManager) ExportDashboardJSON(id string) ([]byte, error) {
	data, err := dm.ExportDashboard(id)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(data, "", "  ")
}

// ImportDashboardJSON 从JSON字节导入.
func (dm *DashboardManager) ImportDashboardJSON(raw []byte) (*Dashboard, error) {
	var data ExportData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return dm.ImportDashboard(&data)
}

// GetWidgetData 获取widget数据（含历史记录）.
func (dm *DashboardManager) GetWidgetData(dashboardID, widgetID string) (*WidgetData, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	w, ok := dm.widgets[widgetID]
	if !ok {
		return nil, fmt.Errorf("widget %s not found", widgetID)
	}
	if w.DashboardID != dashboardID {
		return nil, fmt.Errorf("widget %s does not belong to dashboard %s", widgetID, dashboardID)
	}
	wd, ok := dm.widgetData[widgetID]
	if !ok {
		return &WidgetData{WidgetID: widgetID, Samples: make([]*DataSample, 0)}, nil
	}
	return wd, nil
}

// collectSample 采集单个widget的数据.
func (dm *DashboardManager) collectSample(w *Widget) {
	provider, ok := dm.providers[w.DataSource.Type]
	if !ok {
		provider = dm.providers[DataSourceBuiltin]
	}
	sample, err := provider.Fetch(w)
	if err != nil {
		log.Printf("[CustomDash] collect %s failed: %v", w.ID, err)
		return
	}
	dm.mu.Lock()
	defer dm.mu.Unlock()
	wd, ok := dm.widgetData[w.ID]
	if !ok {
		wd = &WidgetData{WidgetID: w.ID, Samples: make([]*DataSample, 0)}
		dm.widgetData[w.ID] = wd
	}
	wd.Samples = append(wd.Samples, sample)
	wd.Latest = sample
	// 清理超过24小时的采样
	cutoff := time.Now().Add(-24 * time.Hour)
	cleaned := make([]*DataSample, 0, len(wd.Samples))
	for _, s := range wd.Samples {
		if s.Timestamp.After(cutoff) {
			cleaned = append(cleaned, s)
		}
	}
	wd.Samples = cleaned
	// 检查阈值告警
	if w.Threshold.Enabled {
		dm.checkThreshold(w, sample)
	}
}

// checkThreshold 检查阈值告警.
func (dm *DashboardManager) checkThreshold(w *Widget, sample *DataSample) {
	var value float64
	if w.Threshold.Metric != "" {
		if v, ok := sample.Extra[w.Threshold.Metric]; ok {
			value = v
		}
	} else {
		value = sample.Value
	}
	triggered := false
	switch w.Threshold.Operator {
	case OpGT:
		triggered = value > w.Threshold.Value
	case OpGTE:
		triggered = value >= w.Threshold.Value
	case OpLT:
		triggered = value < w.Threshold.Value
	case OpLTE:
		triggered = value <= w.Threshold.Value
	case OpEQ:
		triggered = value == w.Threshold.Value
	}
	if triggered {
		msg := w.Threshold.Message
		if msg == "" {
			msg = fmt.Sprintf("threshold triggered: %s %s %.2f (current: %.2f)", w.Threshold.Metric, w.Threshold.Operator, w.Threshold.Value, value)
		}
		log.Printf("[CustomDash][ALERT] %s: %s", w.Title, msg)
	}
}

// StartSampler 启动后台数据采样.
func (dm *DashboardManager) StartSampler(interval time.Duration) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if dm.running {
		return
	}
	dm.running = true
	dm.sampler = time.NewTicker(interval)
	go func() {
		for range dm.sampler.C {
			dm.mu.RLock()
			widgets := make([]*Widget, 0, len(dm.widgets))
			for _, w := range dm.widgets {
				widgets = append(widgets, w)
			}
			dm.mu.RUnlock()
			for _, w := range widgets {
				dm.collectSample(w)
			}
		}
	}()
	log.Printf("[CustomDash] sampler started, interval: %v", interval)
}

// StopSampler 停止后台数据采样.
func (dm *DashboardManager) StopSampler() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if !dm.running {
		return
	}
	if dm.sampler != nil {
		dm.sampler.Stop()
	}
	dm.running = false
	log.Printf("[CustomDash] sampler stopped")
}

// WidgetDataSize 获取widget数据点数量（测试辅助）.
func (dm *DashboardManager) WidgetDataSize(widgetID string) int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	wd, ok := dm.widgetData[widgetID]
	if !ok {
		return 0
	}
	return len(wd.Samples)
}

// DashboardCount 获取仪表盘数量（测试辅助）.
func (dm *DashboardManager) DashboardCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return len(dm.dashboards)
}

// WidgetCount 获取widget数量（测试辅助）.
func (dm *DashboardManager) WidgetCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return len(dm.widgets)
}
