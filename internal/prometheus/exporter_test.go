// Package prometheus 测试
package prometheus

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock Provider
// ---------------------------------------------------------------------------

// MockProvider 模拟指标数据源.
type MockProvider struct {
	CPUUsage       float64
	MemoryUsage    float64
	DiskMetrics    []DiskMetric
	NetworkMetrics []NetworkMetric
	StoragePools   []StoragePoolMetric
	DiskTemps      []DiskTempMetric
	ActiveConn     float64
	SMBSessions    float64
	ErrCPU         error
	ErrMemory      error
	ErrDisk        error
	ErrNetwork     error
	ErrStorage     error
	ErrTemp        error
	ErrConn        error
	ErrSMB         error
}

func (m *MockProvider) GetCPUUsage() (float64, error) {
	return m.CPUUsage, m.ErrCPU
}

func (m *MockProvider) GetMemoryUsage() (float64, error) {
	return m.MemoryUsage, m.ErrMemory
}

func (m *MockProvider) GetDiskStats() ([]DiskMetric, error) {
	return m.DiskMetrics, m.ErrDisk
}

func (m *MockProvider) GetNetworkStats() ([]NetworkMetric, error) {
	return m.NetworkMetrics, m.ErrNetwork
}

func (m *MockProvider) GetStoragePoolStatus() ([]StoragePoolMetric, error) {
	return m.StoragePools, m.ErrStorage
}

func (m *MockProvider) GetDiskTemperatures() ([]DiskTempMetric, error) {
	return m.DiskTemps, m.ErrTemp
}

func (m *MockProvider) GetActiveConnections() (float64, error) {
	return m.ActiveConn, m.ErrConn
}

func (m *MockProvider) GetSMBSessionCount() (float64, error) {
	return m.SMBSessions, m.ErrSMB
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestExporter_Describe(t *testing.T) {
	provider := &MockProvider{}
	exporter := NewExporter(provider)

	ch := make(chan *promclient.Desc, 20)
	exporter.Describe(ch)
	close(ch)

	var descs []*promclient.Desc
	for d := range ch {
		descs = append(descs, d)
	}

	assert.Len(t, descs, 10, "应导出 10 个指标描述符")

	// 验证指标名称
	names := make(map[string]bool)
	for _, d := range descs {
		// 从 Desc 的 FQDN 中提取指标名
		s := d.String()
		for _, name := range []string{
			"nas_os_cpu_usage",
			"nas_os_memory_usage",
			"nas_os_disk_usage_bytes",
			"nas_os_disk_total_bytes",
			"nas_os_network_rx_bytes",
			"nas_os_network_tx_bytes",
			"nas_os_storage_pool_status",
			"nas_os_disk_temperature",
			"nas_os_active_connections",
			"nas_os_smb_sessions",
		} {
			if strings.Contains(s, name) {
				names[name] = true
			}
		}
	}
	assert.Len(t, names, 10, "应包含全部 10 个指标名称")
}

func TestExporter_Collect_AllMetrics(t *testing.T) {
	provider := &MockProvider{
		CPUUsage:    45.5,
		MemoryUsage: 72.3,
		DiskMetrics: []DiskMetric{
			{MountPoint: "/", Used: 50000000000, Total: 100000000000},
			{MountPoint: "/data", Used: 200000000000, Total: 500000000000},
		},
		NetworkMetrics: []NetworkMetric{
			{Interface: "eth0", RXBytes: 1000000, TXBytes: 500000},
		},
		StoragePools: []StoragePoolMetric{
			{Pool: "tank", Status: 0},
			{Pool: "backup", Status: 1},
		},
		DiskTemps: []DiskTempMetric{
			{Device: "/dev/sda", Temperature: 38},
			{Device: "/dev/sdb", Temperature: 42},
		},
		ActiveConn:  15,
		SMBSessions: 3,
	}

	exporter := NewExporter(provider)
	registry := promclient.NewRegistry()
	require.NoError(t, registry.Register(exporter))

	metrics, err := registry.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]bool)
	for _, mf := range metrics {
		metricNames[mf.GetName()] = true
	}

	assert.True(t, metricNames["nas_os_cpu_usage"], "应包含 nas_os_cpu_usage")
	assert.True(t, metricNames["nas_os_memory_usage"], "应包含 nas_os_memory_usage")
	assert.True(t, metricNames["nas_os_disk_usage_bytes"], "应包含 nas_os_disk_usage_bytes")
	assert.True(t, metricNames["nas_os_disk_total_bytes"], "应包含 nas_os_disk_total_bytes")
	assert.True(t, metricNames["nas_os_network_rx_bytes"], "应包含 nas_os_network_rx_bytes")
	assert.True(t, metricNames["nas_os_network_tx_bytes"], "应包含 nas_os_network_tx_bytes")
	assert.True(t, metricNames["nas_os_storage_pool_status"], "应包含 nas_os_storage_pool_status")
	assert.True(t, metricNames["nas_os_disk_temperature"], "应包含 nas_os_disk_temperature")
	assert.True(t, metricNames["nas_os_active_connections"], "应包含 nas_os_active_connections")
	assert.True(t, metricNames["nas_os_smb_sessions"], "应包含 nas_os_smb_sessions")
}

func TestExporter_Collect_Values(t *testing.T) {
	provider := &MockProvider{
		CPUUsage:    50.0,
		MemoryUsage: 80.0,
		ActiveConn:  42,
		SMBSessions: 7,
		DiskMetrics: []DiskMetric{
			{MountPoint: "/data", Used: 1024, Total: 2048},
		},
		NetworkMetrics: []NetworkMetric{
			{Interface: "eth0", RXBytes: 100, TXBytes: 200},
		},
	}

	exporter := NewExporter(provider)
	registry := promclient.NewRegistry()
	require.NoError(t, registry.Register(exporter))

	metrics, err := registry.Gather()
	require.NoError(t, err)

	for _, mf := range metrics {
		switch mf.GetName() {
		case "nas_os_cpu_usage":
			assert.Equal(t, 50.0, mf.GetMetric()[0].GetGauge().GetValue())
		case "nas_os_memory_usage":
			assert.Equal(t, 80.0, mf.GetMetric()[0].GetGauge().GetValue())
		case "nas_os_active_connections":
			assert.Equal(t, 42.0, mf.GetMetric()[0].GetGauge().GetValue())
		case "nas_os_smb_sessions":
			assert.Equal(t, 7.0, mf.GetMetric()[0].GetGauge().GetValue())
		case "nas_os_disk_usage_bytes":
			assert.Equal(t, 1024.0, mf.GetMetric()[0].GetGauge().GetValue())
			// 验证 label
			assert.Equal(t, "/data", mf.GetMetric()[0].GetLabel()[0].GetValue())
		case "nas_os_network_rx_bytes":
			assert.Equal(t, 100.0, mf.GetMetric()[0].GetGauge().GetValue())
		case "nas_os_network_tx_bytes":
			assert.Equal(t, 200.0, mf.GetMetric()[0].GetGauge().GetValue())
		}
	}
}

func TestExporter_Collect_ErrorGraceful(t *testing.T) {
	// 模拟所有数据源出错，Collect 不应 panic
	provider := &MockProvider{
		ErrCPU:     errors.New("cpu error"),
		ErrMemory:  errors.New("mem error"),
		ErrDisk:    errors.New("disk error"),
		ErrNetwork: errors.New("net error"),
		ErrStorage: errors.New("pool error"),
		ErrTemp:    errors.New("temp error"),
		ErrConn:    errors.New("conn error"),
		ErrSMB:     errors.New("smb error"),
	}

	exporter := NewExporter(provider)
	registry := promclient.NewRegistry()
	require.NoError(t, registry.Register(exporter))

	// 不应 panic，且不应返回错误指标
	assert.NotPanics(t, func() {
		_, _ = registry.Gather()
	})
}

func TestExporter_Collect_EmptyData(t *testing.T) {
	provider := &MockProvider{
		CPUUsage:       0,
		MemoryUsage:    0,
		DiskMetrics:    nil,
		NetworkMetrics: nil,
		StoragePools:   nil,
		DiskTemps:      nil,
		ActiveConn:     0,
		SMBSessions:    0,
	}

	exporter := NewExporter(provider)
	registry := promclient.NewRegistry()
	require.NoError(t, registry.Register(exporter))

	metrics, err := registry.Gather()
	require.NoError(t, err)

	// 应有 CPU/内存/连接/SMB 指标（即使值为 0），但无 disk/network/pool/temp 指标
	metricNames := make(map[string]bool)
	for _, mf := range metrics {
		metricNames[mf.GetName()] = true
	}

	assert.True(t, metricNames["nas_os_cpu_usage"])
	assert.True(t, metricNames["nas_os_memory_usage"])
	assert.True(t, metricNames["nas_os_active_connections"])
	assert.True(t, metricNames["nas_os_smb_sessions"])
	assert.False(t, metricNames["nas_os_disk_usage_bytes"], "无磁盘时不应有磁盘指标")
	assert.False(t, metricNames["nas_os_network_rx_bytes"], "无网卡时不应有网络指标")
}

func TestHandler_HTTP(t *testing.T) {
	provider := &MockProvider{
		CPUUsage:    33.3,
		MemoryUsage: 66.6,
		ActiveConn:  10,
		SMBSessions: 2,
	}

	handler := NewHandler(provider)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, "nas_os_cpu_usage", "响应应包含 nas_os_cpu_usage")
	assert.Contains(t, body, "nas_os_memory_usage", "响应应包含 nas_os_memory_usage")
	assert.Contains(t, body, "nas_os_active_connections", "响应应包含 nas_os_active_connections")
	assert.Contains(t, body, "nas_os_smb_sessions", "响应应包含 nas_os_smb_sessions")
}

func TestHandler_MetricsHandler(t *testing.T) {
	provider := &MockProvider{
		CPUUsage: 10.0,
	}

	handler := NewHandler(provider)
	metricsHandler := handler.MetricsHandler()

	assert.NotNil(t, metricsHandler, "MetricsHandler 不应返回 nil")
}

func TestHandler_Registry(t *testing.T) {
	provider := &MockProvider{}
	handler := NewHandler(provider)

	assert.NotNil(t, handler.Registry(), "Registry 不应返回 nil")
}

func TestParsePoolHealth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"ONLINE", 0},
		{"DEGRADED", 1},
		{"FAULTED", 2},
		{"UNAVAIL", 2},
		{"REMOVED", 2},
		{"UNKNOWN", 2},
		{"online", 0}, // 大小写不敏感
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, parsePoolHealth(tt.input))
		})
	}
}
