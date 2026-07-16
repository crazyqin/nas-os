// Package netmonitor 单元测试
package netmonitor

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Manager Tests ==========

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.NotNil(t, m.interfaces)
	assert.NotNil(t, m.trafficLog)
	assert.NotNil(t, m.connections)
	assert.NotNil(t, m.alertRules)
	assert.NotNil(t, m.alertEvents)
	assert.NotNil(t, m.portMonitors)
}

func TestManager_GetInterfaces(t *testing.T) {
	m := NewManager()

	// 手动添加接口
	m.interfaces["eth0"] = &NetworkInterface{
		Name:   "eth0",
		Status: InterfaceStatusUp,
		Speed:  1000,
	}

	ifaces := m.GetInterfaces()
	assert.Len(t, ifaces, 1)
	assert.Equal(t, "eth0", ifaces[0].Name)
}

func TestManager_GetInterface(t *testing.T) {
	m := NewManager()

	m.interfaces["eth0"] = &NetworkInterface{
		Name:   "eth0",
		Status: InterfaceStatusUp,
	}

	// 存在的接口
	iface, err := m.GetInterface("eth0")
	require.NoError(t, err)
	assert.Equal(t, "eth0", iface.Name)

	// 不存在的接口
	_, err = m.GetInterface("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetTrafficStats(t *testing.T) {
	m := NewManager()

	// 添加流量数据
	m.trafficLog["eth0"] = []*TrafficStats{
		{Interface: "eth0", RxBytesSec: 1024, TxBytesSec: 512},
	}

	// 获取指定接口
	stats := m.GetTrafficStats("eth0")
	assert.Len(t, stats, 1)

	// 获取所有接口
	allStats := m.GetTrafficStats("")
	assert.Len(t, allStats, 1)
}

func TestManager_GetConnections(t *testing.T) {
	m := NewManager()

	m.connections = []*ConnectionInfo{
		{Protocol: "tcp", State: "ESTABLISHED"},
		{Protocol: "tcp", State: "LISTEN"},
		{Protocol: "udp"},
	}

	stats := m.GetConnections()
	assert.Equal(t, 3, stats.TotalConns)
	assert.Equal(t, 2, stats.TCPConns)
	assert.Equal(t, 1, stats.UDPConns)
	assert.Equal(t, 1, stats.Established)
	assert.Equal(t, 1, stats.Listening)
}

func TestManager_AlertRules(t *testing.T) {
	m := NewManager()

	// 添加规则
	rule := &AlertRule{
		Name:      "high_bandwidth",
		Interface: "eth0",
		Type:      "bandwidth",
		Threshold: 80.0,
		Level:     AlertLevelWarning,
		Enabled:   true,
	}

	err := m.AddAlertRule(rule)
	require.NoError(t, err)
	assert.NotEmpty(t, rule.ID)

	// 获取规则列表
	rules := m.GetAlertRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, "high_bandwidth", rules[0].Name)

	// 删除规则
	err = m.RemoveAlertRule(rule.ID)
	require.NoError(t, err)

	rules = m.GetAlertRules()
	assert.Len(t, rules, 0)
}

func TestManager_RemoveAlertRule_NotFound(t *testing.T) {
	m := NewManager()

	err := m.RemoveAlertRule("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetAlertEvents(t *testing.T) {
	m := NewManager()

	// 添加事件
	m.alertEvents = []*AlertEvent{
		{ID: "1", Message: "event1"},
		{ID: "2", Message: "event2"},
		{ID: "3", Message: "event3"},
	}

	events := m.GetAlertEvents(2)
	assert.Len(t, events, 2)
	assert.Equal(t, "2", events[0].ID)
	assert.Equal(t, "3", events[1].ID)
}

func TestManager_DiscoverTopology(t *testing.T) {
	m := NewManager()

	topology := m.DiscoverTopology()
	require.NotNil(t, topology)
	assert.NotEmpty(t, topology.Nodes)
	assert.NotEmpty(t, topology.Links)
	assert.False(t, topology.Discovered.IsZero())
}

func TestManager_GetTopology(t *testing.T) {
	m := NewManager()

	// 未发现时为空
	assert.Nil(t, m.GetTopology())

	// 发现后有数据
	m.DiscoverTopology()
	topology := m.GetTopology()
	assert.NotNil(t, topology)
}

func TestManager_PortMonitors(t *testing.T) {
	m := NewManager()

	config := &PortMonitorConfig{
		Host:     "192.168.1.1",
		Ports:    []int{22, 80, 443},
		Interval: 60,
	}

	err := m.AddPortMonitor(config)
	require.NoError(t, err)

	configs := m.GetPortMonitors()
	assert.Len(t, configs, 1)
	assert.Equal(t, "192.168.1.1", configs[0].Host)
}

func TestManager_CheckPorts(t *testing.T) {
	m := NewManager()

	results := m.CheckPorts("192.168.1.1", []int{22, 80, 443})
	assert.Len(t, results, 3)

	for _, r := range results {
		assert.Equal(t, "tcp", r.Protocol)
		assert.Equal(t, "open", r.State)
		assert.NotEmpty(t, r.Service)
	}
}

// ========== Handlers Tests ==========

func setupHandlers(t *testing.T) (*gin.Engine, *Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	m := NewManager()
	h := NewHandlers(m)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	return r, m
}

func TestHandlers_GetInterfaces(t *testing.T) {
	r, m := setupHandlers(t)

	// 添加接口
	m.interfaces["eth0"] = &NetworkInterface{
		Name:   "eth0",
		Status: InterfaceStatusUp,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/interfaces", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "eth0")
}

func TestHandlers_GetInterface(t *testing.T) {
	r, m := setupHandlers(t)

	m.interfaces["eth0"] = &NetworkInterface{
		Name:   "eth0",
		Status: InterfaceStatusUp,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/interfaces/eth0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "eth0")
}

func TestHandlers_GetInterface_NotFound(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/interfaces/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetTraffic(t *testing.T) {
	r, m := setupHandlers(t)

	m.trafficLog["eth0"] = []*TrafficStats{
		{Interface: "eth0", RxBytesSec: 1024},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/traffic?interface=eth0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "eth0")
}

func TestHandlers_GetConnections(t *testing.T) {
	r, m := setupHandlers(t)

	m.connections = []*ConnectionInfo{
		{Protocol: "tcp", State: "ESTABLISHED"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/connections", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tcp_conns")
}

func TestHandlers_AddAlertRule(t *testing.T) {
	r, _ := setupHandlers(t)

	body := `{"name":"test_rule","type":"bandwidth","threshold":80,"level":"warning","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/netmonitor/alerts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "alert rule added")
}

func TestHandlers_GetAlertRules(t *testing.T) {
	r, m := setupHandlers(t)

	m.AddAlertRule(&AlertRule{
		Name:    "test_rule",
		Type:    "bandwidth",
		Enabled: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/alerts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test_rule")
}

func TestHandlers_RemoveAlertRule(t *testing.T) {
	r, m := setupHandlers(t)

	rule := &AlertRule{
		Name:    "to_remove",
		Type:    "errors",
		Enabled: true,
	}
	m.AddAlertRule(rule)

	req := httptest.NewRequest(http.MethodDelete, "/api/netmonitor/alerts/"+rule.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "alert rule removed")
}

func TestHandlers_RemoveAlertRule_NotFound(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/netmonitor/alerts/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetAlertEvents(t *testing.T) {
	r, m := setupHandlers(t)

	m.alertEvents = []*AlertEvent{
		{ID: "1", Message: "test event"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/alerts/events?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test event")
}

func TestHandlers_GetTopology_Initial(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/topology", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 未发现时返回 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_DiscoverTopology(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/netmonitor/topology/discover", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "topology discovered")
	assert.Contains(t, w.Body.String(), "NAS")
}

func TestHandlers_GetTopology_AfterDiscovery(t *testing.T) {
	r, m := setupHandlers(t)

	// 先发现拓扑
	m.DiscoverTopology()

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/topology", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "NAS")
}

func TestHandlers_CheckPorts(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/ports?host=192.168.1.1&ports=22&ports=80", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "192.168.1.1")
	assert.Contains(t, w.Body.String(), "ssh")
}

func TestHandlers_CheckPorts_MissingHost(t *testing.T) {
	r, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/netmonitor/ports?ports=22", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== Types Tests ==========

func TestTypes_Constants(t *testing.T) {
	assert.Equal(t, InterfaceStatus("up"), InterfaceStatusUp)
	assert.Equal(t, InterfaceStatus("down"), InterfaceStatusDown)
	assert.Equal(t, InterfaceStatus("unknown"), InterfaceStatusUnknown)

	assert.Equal(t, AlertLevel("info"), AlertLevelInfo)
	assert.Equal(t, AlertLevel("warning"), AlertLevelWarning)
	assert.Equal(t, AlertLevel("critical"), AlertLevelCritical)
}

// ========== Integration Test ==========

func TestIntegration_FullFlow(t *testing.T) {
	m := NewManager()

	// 添加告警规则
	rule := &AlertRule{
		Name:      "high_errors",
		Interface: "eth0",
		Type:      "errors",
		Threshold: 10,
		Level:     AlertLevelWarning,
		Enabled:   true,
	}
	m.AddAlertRule(rule)

	// 发现拓扑
	topology := m.DiscoverTopology()
	assert.NotNil(t, topology)
	assert.NotEmpty(t, topology.Nodes)

	// 检查端口
	ports := m.CheckPorts("192.168.1.1", []int{22, 80})
	assert.Len(t, ports, 2)

	// 获取连接统计
	m.connections = []*ConnectionInfo{
		{Protocol: "tcp", State: "ESTABLISHED"},
		{Protocol: "tcp", State: "LISTEN"},
	}
	stats := m.GetConnections()
	assert.Equal(t, 2, stats.TotalConns)

	// 获取接口
	m.interfaces["eth0"] = &NetworkInterface{
		Name:   "eth0",
		Status: InterfaceStatusUp,
	}
	ifaces := m.GetInterfaces()
	assert.Len(t, ifaces, 1)
}
