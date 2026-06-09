package netdatawidget

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// MetricType 指标类型
type MetricType string

const (
	MetricCPU     MetricType = "CPU"
	MetricMemory  MetricType = "MEMORY"
	MetricDisk    MetricType = "DISK"
	MetricNetwork MetricType = "NETWORK"
	MetricIO      MetricType = "IO"
	MetricTemp    MetricType = "TEMPERATURE"
	MetricCustom  MetricType = "CUSTOM"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "INFO"
	AlertWarning  AlertLevel = "WARNING"
	AlertCritical AlertLevel = "CRITICAL"
)

// TimeRange 时间范围
type TimeRange string

const (
	Range1H  TimeRange = "1H"
	Range6H  TimeRange = "6H"
	Range24H TimeRange = "24H"
	Range7D  TimeRange = "7D"
	Range30D TimeRange = "30D"
)

// MetricPoint 指标点
type MetricPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// MetricSeries 指标序列
type MetricSeries struct {
	Name    string        `json:"name"`
	Type    MetricType    `json:"type"`
	Unit    string        `json:"unit"`
	Points  []MetricPoint `json:"points"`
	Min     float64       `json:"min"`
	Max     float64       `json:"max"`
	Avg     float64       `json:"avg"`
	Current float64       `json:"current"`
}

// DashboardWidget 仪表盘组件
type DashboardWidget struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Type        string      `json:"type"` // gauge, chart, table, heatmap
	Metrics     []string    `json:"metrics"`
	Position    Position    `json:"position"`
	Size        Size        `json:"size"`
	RefreshRate int         `json:"refresh_rate_sec"`
	Thresholds  []Threshold `json:"thresholds,omitempty"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Size struct {
	W int `json:"w"`
	H int `json:"h"`
}

type Threshold struct {
	Value float64    `json:"value"`
	Level AlertLevel `json:"level"`
	Color string     `json:"color"`
}

// SystemAlert 系统告警
type SystemAlert struct {
	ID        string     `json:"id"`
	Level     AlertLevel `json:"level"`
	Metric    string     `json:"metric"`
	Message   string     `json:"message"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
	Timestamp time.Time  `json:"timestamp"`
	Acked     bool       `json:"acked"`
}

// NetdataWidget Netdata集成组件
type NetdataWidget struct {
	metrics   map[string]*MetricSeries
	widgets   map[string]*DashboardWidget
	alerts    []*SystemAlert
	dataPath  string
	mu        sync.RWMutex
	maxPoints int
}

// NewNetdataWidget 创建Netdata组件
func NewNetdataWidget(dataPath string, maxPoints int) *NetdataWidget {
	os.MkdirAll(dataPath, 0755)
	if maxPoints == 0 {
		maxPoints = 1000
	}
	w := &NetdataWidget{
		metrics:   make(map[string]*MetricSeries),
		widgets:   make(map[string]*DashboardWidget),
		dataPath:  dataPath,
		maxPoints: maxPoints,
	}
	w.loadState()
	w.initDefaultWidgets()
	return w
}

// RecordMetric 记录指标
func (w *NetdataWidget) RecordMetric(name string, mType MetricType, unit string, value float64, labels map[string]string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	series, ok := w.metrics[name]
	if !ok {
		series = &MetricSeries{
			Name: name,
			Type: mType,
			Unit: unit,
			Min:  value,
			Max:  value,
		}
		w.metrics[name] = series
	}
	point := MetricPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	}
	series.Points = append(series.Points, point)
	if len(series.Points) > w.maxPoints {
		series.Points = series.Points[len(series.Points)-w.maxPoints:]
	}
	series.Current = value
	if value < series.Min {
		series.Min = value
	}
	if value > series.Max {
		series.Max = value
	}
	total := 0.0
	for _, p := range series.Points {
		total += p.Value
	}
	series.Avg = total / float64(len(series.Points))
	w.checkThresholds(name, value)
}

// GetMetric 获取指标
func (w *NetdataWidget) GetMetric(name string) (*MetricSeries, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	m, ok := w.metrics[name]
	return m, ok
}

// GetMetrics 获取所有指标
func (w *NetdataWidget) GetMetrics(mType *MetricType) []*MetricSeries {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var result []*MetricSeries
	for _, m := range w.metrics {
		if mType != nil && m.Type != *mType {
			continue
		}
		result = append(result, m)
	}
	return result
}

// AddWidget 添加组件
func (w *NetdataWidget) AddWidget(widget *DashboardWidget) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.widgets[widget.ID] = widget
	w.saveState()
}

// RemoveWidget 移除组件
func (w *NetdataWidget) RemoveWidget(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.widgets, id)
	w.saveState()
}

// GetWidgets 获取所有组件
func (w *NetdataWidget) GetWidgets() []*DashboardWidget {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var result []*DashboardWidget
	for _, widget := range w.widgets {
		result = append(result, widget)
	}
	return result
}

// GetAlerts 获取告警
func (w *NetdataWidget) GetAlerts(unackedOnly bool) []*SystemAlert {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var result []*SystemAlert
	for _, a := range w.alerts {
		if unackedOnly && a.Acked {
			continue
		}
		result = append(result, a)
	}
	return result
}

// AcknowledgeAlert 确认告警
func (w *NetdataWidget) AcknowledgeAlert(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, a := range w.alerts {
		if a.ID == id {
			a.Acked = true
			break
		}
	}
}

// GetSystemOverview 系统概览
func (w *NetdataWidget) GetSystemOverview() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()
	overview := map[string]interface{}{
		"total_metrics": len(w.metrics),
		"total_widgets": len(w.widgets),
		"total_alerts":  len(w.alerts),
	}
	cpu, ok := w.metrics["system.cpu"]
	if ok {
		overview["cpu_usage"] = cpu.Current
	}
	mem, ok := w.metrics["system.memory"]
	if ok {
		overview["memory_usage"] = mem.Current
	}
	return overview
}

func (w *NetdataWidget) checkThresholds(name string, value float64) {
	for _, widget := range w.widgets {
		// 仅检查包含该指标的组件
		matched := false
		for _, m := range widget.Metrics {
			if m == name {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		for _, t := range widget.Thresholds {
			if value >= t.Value {
				alert := &SystemAlert{
					ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
					Level:     t.Level,
					Metric:    name,
					Message:   fmt.Sprintf("%s 超过阈值: %.2f >= %.2f", name, value, t.Value),
					Value:     value,
					Threshold: t.Value,
					Timestamp: time.Now(),
				}
				w.alerts = append(w.alerts, alert)
			}
		}
	}
}

func (w *NetdataWidget) initDefaultWidgets() {
	if len(w.widgets) > 0 {
		return
	}
	w.widgets = map[string]*DashboardWidget{
		"cpu": {
			ID: "cpu", Title: "CPU使用率", Type: "gauge",
			Metrics: []string{"system.cpu"}, RefreshRate: 5,
			Thresholds: []Threshold{{Value: 80, Level: AlertWarning, Color: "#ff9800"}, {Value: 95, Level: AlertCritical, Color: "#f44336"}},
		},
		"memory": {
			ID: "memory", Title: "内存使用率", Type: "gauge",
			Metrics: []string{"system.memory"}, RefreshRate: 5,
			Thresholds: []Threshold{{Value: 85, Level: AlertWarning, Color: "#ff9800"}, {Value: 95, Level: AlertCritical, Color: "#f44336"}},
		},
		"disk": {
			ID: "disk", Title: "磁盘使用率", Type: "chart",
			Metrics: []string{"system.disk"}, RefreshRate: 30,
		},
		"network": {
			ID: "network", Title: "网络流量", Type: "chart",
			Metrics: []string{"system.net.in", "system.net.out"}, RefreshRate: 5,
		},
	}
}

func (w *NetdataWidget) saveState() {
	state := struct {
		Widgets map[string]*DashboardWidget `json:"widgets"`
	}{w.widgets}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(w.dataPath+"/widgets.json", data, 0644)
}

func (w *NetdataWidget) loadState() {
	data, err := os.ReadFile(w.dataPath + "/widgets.json")
	if err != nil {
		return
	}
	var state struct {
		Widgets map[string]*DashboardWidget `json:"widgets"`
	}
	json.Unmarshal(data, &state)
	if state.Widgets != nil {
		w.widgets = state.Widgets
	}
}
