package aitraffic

import (
	"testing"
	"time"
)

func TestAnalyzerIngestAndStats(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	now := time.Now()
	flows := []FlowRecord{
		{Timestamp: now, SrcIP: "192.168.1.1", DstIP: "10.0.0.1", Protocol: "tcp", BytesIn: 1000, BytesOut: 500, Application: "web"},
		{Timestamp: now.Add(time.Second), SrcIP: "192.168.1.2", DstIP: "10.0.0.1", Protocol: "tcp", BytesIn: 2000, BytesOut: 1000, Application: "ssh"},
		{Timestamp: now.Add(2 * time.Second), SrcIP: "192.168.1.1", DstIP: "10.0.0.2", Protocol: "udp", BytesIn: 500, BytesOut: 200, Application: "dns"},
	}

	for _, f := range flows {
		analyzer.IngestFlow(f)
	}

	stats := analyzer.GetStats()
	if stats.TotalBytesIn != 3500 {
		t.Errorf("TotalBytesIn = %d, want 3500", stats.TotalBytesIn)
	}
	if stats.TotalBytesOut != 1700 {
		t.Errorf("TotalBytesOut = %d, want 1700", stats.TotalBytesOut)
	}
}

func TestProtocolDistribution(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	now := time.Now()

	analyzer.IngestFlow(FlowRecord{Timestamp: now, Protocol: "tcp", BytesIn: 1000, BytesOut: 500})
	analyzer.IngestFlow(FlowRecord{Timestamp: now, Protocol: "tcp", BytesIn: 2000, BytesOut: 1000})
	analyzer.IngestFlow(FlowRecord{Timestamp: now, Protocol: "udp", BytesIn: 500, BytesOut: 200})

	stats := analyzer.GetStats()
	if stats.ProtocolDist["tcp"] != 4500 {
		t.Errorf("tcp total = %d, want 4500", stats.ProtocolDist["tcp"])
	}
	if stats.ProtocolDist["udp"] != 700 {
		t.Errorf("udp total = %d, want 700", stats.ProtocolDist["udp"])
	}
}

func TestAnomalyDetection(t *testing.T) {
	analyzer := NewAnalyzer(&AnalyzerConfig{
		DDoSThreshold:  100,
		ExfilThreshold: 1000,
		SpikeThreshold: 2.0,
		MaxFlows:       1000,
	})

	// 测试 DDoS 检测
	analyzer.IngestFlow(FlowRecord{
		Timestamp: time.Now(),
		SrcIP:     "192.168.1.100",
		DstIP:     "10.0.0.1",
		PacketsIn: 200,
		BytesIn:   1000,
	})

	anomalies := analyzer.GetAnomalies()
	found := false
	for _, a := range anomalies {
		if a.Type == "ddos" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected DDoS anomaly to be detected")
	}
}

func TestExfiltrationDetection(t *testing.T) {
	analyzer := NewAnalyzer(&AnalyzerConfig{
		DDoSThreshold:  1000,
		ExfilThreshold: 500,
		SpikeThreshold: 3.0,
		MaxFlows:       1000,
	})

	analyzer.IngestFlow(FlowRecord{
		Timestamp: time.Now(),
		SrcIP:     "10.0.0.1",
		DstIP:     "192.168.1.100",
		BytesOut:  1000,
	})

	anomalies := analyzer.GetAnomalies()
	found := false
	for _, a := range anomalies {
		if a.Type == "exfiltration" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected exfiltration anomaly to be detected")
	}
}

func TestTopApps(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	now := time.Now()

	analyzer.IngestFlow(FlowRecord{Timestamp: now, Application: "web", BytesIn: 5000, BytesOut: 1000})
	analyzer.IngestFlow(FlowRecord{Timestamp: now, Application: "ssh", BytesIn: 1000, BytesOut: 500})
	analyzer.IngestFlow(FlowRecord{Timestamp: now, Application: "web", BytesIn: 3000, BytesOut: 800})

	stats := analyzer.GetStats()
	if len(stats.TopApps) < 2 {
		t.Fatalf("expected at least 2 apps, got %d", len(stats.TopApps))
	}
}

func TestAnalyzerStartStop(t *testing.T) {
	analyzer := NewAnalyzer(&AnalyzerConfig{
		WindowSize: 100 * time.Millisecond,
	})
	analyzer.Start()
	time.Sleep(50 * time.Millisecond)
	analyzer.Stop()
}

func TestMaxFlowsLimit(t *testing.T) {
	config := DefaultAnalyzerConfig()
	config.MaxFlows = 5
	analyzer := NewAnalyzer(config)

	now := time.Now()
	for i := 0; i < 10; i++ {
		analyzer.IngestFlow(FlowRecord{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			BytesIn:   100,
		})
	}

	stats := analyzer.GetStats()
	// 应该只保留最后 5 条
	if stats.TotalBytesIn != 500 {
		t.Errorf("TotalBytesIn = %d, want 500", stats.TotalBytesIn)
	}
}
