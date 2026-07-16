package netflow

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestHandler(t *testing.T) (*Handler, *Collector, *Analyzer, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	config := DefaultCollectorConfig()
	config.BufferSize = 1000
	config.FlushIntervalSec = 3600 // 长间隔，避免测试中触发

	collector := NewCollector(config, zap.NewNop())
	analyzer := NewAnalyzer(collector, zap.NewNop())
	handler := NewHandler(collector, analyzer, zap.NewNop())

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, collector, analyzer, router
}

func ingestTestFlows(collector *Collector, count int) {
	now := time.Now()
	for i := 0; i < count; i++ {
		collector.IngestFlow(FlowRecord{
			SrcIP:     "192.168.1.100",
			DstIP:     "10.0.0.1",
			SrcPort:   uint16(10000 + i),
			DstPort:   80,
			Protocol:  ProtocolHTTP,
			Bytes:     int64(1024 * (i + 1)),
			Packets:   int64(i + 1),
			Direction: DirectionOutbound,
			Interface: "eth0",
			Timestamp: now.Add(-time.Duration(count-i) * time.Second),
		})
	}
}

func TestStartStopCollector(t *testing.T) {
	_, collector, _, router := setupTestHandler(t)

	// 启动
	req := httptest.NewRequest(http.MethodPost, "/api/v1/netflow/collector/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("start: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !collector.IsRunning() {
		t.Error("collector should be running after start")
	}

	// 重复启动
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("duplicate start: expected 200, got %d", w2.Code)
	}

	// 停止
	stopReq := httptest.NewRequest(http.MethodPost, "/api/v1/netflow/collector/stop", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, stopReq)

	if w3.Code != http.StatusOK {
		t.Fatalf("stop: expected 200, got %d", w3.Code)
	}
	if collector.IsRunning() {
		t.Error("collector should not be running after stop")
	}
}

func TestGetCollectorStatus(t *testing.T) {
	_, _, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/collector/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["running"]; !ok {
		t.Error("expected 'running' field in response")
	}
	if _, ok := resp["config"]; !ok {
		t.Error("expected 'config' field in response")
	}
}

func TestGetTrafficStats(t *testing.T) {
	_, collector, _, router := setupTestHandler(t)

	ingestTestFlows(collector, 5)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp TrafficStatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Stats.TotalBytesOut <= 0 {
		t.Errorf("expected total_bytes_out > 0, got %d", resp.Stats.TotalBytesOut)
	}
}

func TestGetProtocolStats(t *testing.T) {
	_, collector, _, router := setupTestHandler(t)

	ingestTestFlows(collector, 3)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/stats/protocols", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Protocols []ProtocolStats `json:"protocols"`
		Total     int             `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total < 1 {
		t.Errorf("expected at least 1 protocol, got %d", resp.Total)
	}
}

func TestGetTopHosts(t *testing.T) {
	_, collector, _, router := setupTestHandler(t)

	ingestTestFlows(collector, 5)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/stats/hosts?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Hosts []HostTraffic `json:"hosts"`
		Total int           `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total < 1 {
		t.Errorf("expected at least 1 host, got %d", resp.Total)
	}
}

func TestGetBandwidthHistory(t *testing.T) {
	_, _, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/stats/bandwidth", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		History []BandwidthUsage `json:"history"`
		Total   int              `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// 初始状态应该为空
	if resp.Total != 0 {
		t.Errorf("expected 0 history entries initially, got %d", resp.Total)
	}
}

func TestTopHosts(t *testing.T) {
	_, collector, analyzer, router := setupTestHandler(t)

	ingestTestFlows(collector, 10)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/top/hosts?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result TopNResult
	json.Unmarshal(w.Body.Bytes(), &result)

	if result.Category != "hosts" {
		t.Errorf("expected category hosts, got %s", result.Category)
	}

	_ = analyzer // 保持引用
}

func TestTopProtocols(t *testing.T) {
	_, collector, _, router := setupTestHandler(t)

	// 注入不同协议的流量
	now := time.Now()
	collector.IngestFlow(FlowRecord{
		SrcIP: "1.1.1.1", DstIP: "2.2.2.2", Protocol: ProtocolHTTP,
		Bytes: 5000, Direction: DirectionInbound, Timestamp: now,
	})
	collector.IngestFlow(FlowRecord{
		SrcIP: "1.1.1.1", DstIP: "2.2.2.2", Protocol: ProtocolDNS,
		Bytes: 200, Direction: DirectionOutbound, Timestamp: now,
	})
	collector.IngestFlow(FlowRecord{
		SrcIP: "1.1.1.1", DstIP: "2.2.2.2", Protocol: ProtocolHTTPS,
		Bytes: 8000, Direction: DirectionInbound, Timestamp: now,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/top/protocols?limit=3", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result TopNResult
	json.Unmarshal(w.Body.Bytes(), &result)

	if result.Category != "protocols" {
		t.Errorf("expected category protocols, got %s", result.Category)
	}
	if len(result.Entries) < 1 {
		t.Error("expected at least 1 protocol entry")
	}
}

func TestTopConversations(t *testing.T) {
	_, collector, _, router := setupTestHandler(t)

	ingestTestFlows(collector, 5)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/top/conversations?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result TopNResult
	json.Unmarshal(w.Body.Bytes(), &result)

	if result.Category != "conversations" {
		t.Errorf("expected category conversations, got %s", result.Category)
	}
}

func TestRunAnalysis(t *testing.T) {
	_, _, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/netflow/analyze", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		NewAlerts []AnomalyAlert `json:"new_alerts"`
		Total     int            `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// 无数据时不应有告警
	if resp.Total != 0 {
		t.Errorf("expected 0 alerts with no data, got %d", resp.Total)
	}
}

func TestGetAlerts(t *testing.T) {
	_, _, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/alerts?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp AlertListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 0 {
		t.Errorf("expected 0 alerts initially, got %d", resp.Total)
	}
}

func TestGetAlertStats(t *testing.T) {
	_, _, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netflow/alerts/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &stats)

	if stats["total_alerts"] != float64(0) {
		t.Errorf("expected 0 total alerts, got %v", stats["total_alerts"])
	}
}

func TestResolveAlert(t *testing.T) {
	_, _, analyzer, router := setupTestHandler(t)

	// 创建一个告警用于测试
	alert := AnomalyAlert{
		ID:       "test-alert-001",
		Type:     AnomalyTrafficSpike,
		Severity: "warning",
	}
	analyzer.mu.Lock()
	analyzer.alerts = append(analyzer.alerts, alert)
	analyzer.mu.Unlock()

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/netflow/alerts/test-alert-001/resolve", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveAlertNotFound(t *testing.T) {
	_, _, _, router := setupTestHandler(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/netflow/alerts/nonexistent/resolve", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateThresholds(t *testing.T) {
	_, _, analyzer, router := setupTestHandler(t)

	body := `{
		"spike_threshold_mbps": 200,
		"port_scan_threshold": 50,
		"dns_flood_threshold": 2000,
		"high_conn_rate_threshold": 1000
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/netflow/config/thresholds", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证阈值已更新
	analyzer.mu.RLock()
	if analyzer.spikeThresholdMBPS != 200 {
		t.Errorf("expected spike threshold 200, got %f", analyzer.spikeThresholdMBPS)
	}
	if analyzer.portScanThreshold != 50 {
		t.Errorf("expected port scan threshold 50, got %d", analyzer.portScanThreshold)
	}
	analyzer.mu.RUnlock()
}

func TestCollectorIngestFlow(t *testing.T) {
	_, collector, _, _ := setupTestHandler(t)

	record := FlowRecord{
		SrcIP:     "192.168.1.1",
		DstIP:     "10.0.0.1",
		SrcPort:   12345,
		DstPort:   443,
		Protocol:  ProtocolHTTPS,
		Bytes:     4096,
		Packets:   8,
		Direction: DirectionOutbound,
		Interface: "eth0",
		Timestamp: time.Now(),
	}

	collector.IngestFlow(record)

	records := collector.GetRecentRecords(10)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Protocol != ProtocolHTTPS {
		t.Errorf("expected HTTPS protocol, got %s", records[0].Protocol)
	}
}

func TestCollectorIngestBatch(t *testing.T) {
	_, collector, _, _ := setupTestHandler(t)

	records := make([]FlowRecord, 100)
	for i := range records {
		records[i] = FlowRecord{
			SrcIP:     "192.168.1.1",
			DstIP:     "10.0.0.1",
			Protocol:  ProtocolTCP,
			Bytes:     1024,
			Direction: DirectionInbound,
			Timestamp: time.Now(),
		}
	}

	collector.IngestBatch(records)

	recent := collector.GetRecentRecords(200)
	if len(recent) != 100 {
		t.Errorf("expected 100 records, got %d", len(recent))
	}
}

func TestCollectorClear(t *testing.T) {
	_, collector, _, _ := setupTestHandler(t)

	ingestTestFlows(collector, 10)

	if len(collector.GetRecentRecords(100)) == 0 {
		t.Fatal("expected records after ingest")
	}

	collector.Clear()

	if len(collector.GetRecentRecords(100)) != 0 {
		t.Error("expected 0 records after clear")
	}
}

func TestCollectorBufferSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := DefaultCollectorConfig()
	config.BufferSize = 5 // 小缓冲区
	config.FlushIntervalSec = 3600

	collector := NewCollector(config, zap.NewNop())

	// 注入超过缓冲区大小的记录
	for i := 0; i < 10; i++ {
		collector.IngestFlow(FlowRecord{
			SrcIP: "1.1.1.1", DstIP: "2.2.2.2",
			Protocol: ProtocolTCP, Bytes: 100,
			Direction: DirectionInbound, Timestamp: time.Now(),
		})
	}

	records := collector.GetRecentRecords(100)
	if len(records) > 5 {
		t.Errorf("expected at most 5 records (buffer size), got %d", len(records))
	}
}
