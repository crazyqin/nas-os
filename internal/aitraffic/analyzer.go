// Package aitraffic 提供 AI 驱动的网络流量分析与异常检测
// 对标群晖 Traffic Control + TrueNAS NetFlow，增加 AI 异常检测
package aitraffic

import (
	"context"
	"math"
	"sync"
	"time"
)

// FlowRecord 流量记录.
type FlowRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	SrcIP       string    `json:"src_ip"`
	DstIP       string    `json:"dst_ip"`
	SrcPort     int       `json:"src_port"`
	DstPort     int       `json:"dst_port"`
	Protocol    string    `json:"protocol"`
	BytesIn     int64     `json:"bytes_in"`
	BytesOut    int64     `json:"bytes_out"`
	PacketsIn   int64     `json:"packets_in"`
	PacketsOut  int64     `json:"packets_out"`
	Duration    int       `json:"duration_ms"`
	Application string    `json:"application,omitempty"`
}

// TrafficStats 流量统计.
type TrafficStats struct {
	TotalBytesIn   int64            `json:"total_bytes_in"`
	TotalBytesOut  int64            `json:"total_bytes_out"`
	AvgBandwidth   float64          `json:"avg_bandwidth_mbps"`
	PeakBandwidth  float64          `json:"peak_bandwidth_mbps"`
	ProtocolDist   map[string]int64 `json:"protocol_dist"`
	TopApps        []AppTraffic     `json:"top_apps"`
	TopConnections []ConnTraffic    `json:"top_connections"`
	Anomalies      []Anomaly        `json:"anomalies"`
	TrendDirection string           `json:"trend_direction"` // up/down/stable
	TrendPct       float64          `json:"trend_pct"`
}

// AppTraffic 应用流量.
type AppTraffic struct {
	App      string  `json:"app"`
	BytesIn  int64   `json:"bytes_in"`
	BytesOut int64   `json:"bytes_out"`
	Total    int64   `json:"total"`
	Pct      float64 `json:"pct"`
}

// ConnTraffic 连接流量.
type ConnTraffic struct {
	SrcIP    string `json:"src_ip"`
	DstIP    string `json:"dst_ip"`
	Total    int64  `json:"total"`
	Duration int    `json:"duration_ms"`
}

// Anomaly 异常.
type Anomaly struct {
	Timestamp     time.Time `json:"timestamp"`
	Type          string    `json:"type"`     // spike/ddos/portscan/exfiltration
	Severity      string    `json:"severity"` // low/medium/high/critical
	Source        string    `json:"source"`
	Target        string    `json:"target"`
	Score         float64   `json:"score"` // 异常分数 0-1
	Details       string    `json:"details"`
	BytesAffected int64     `json:"bytes_affected"`
}

// AnalyzerConfig 分析器配置.
type AnalyzerConfig struct {
	WindowSize     time.Duration `json:"window_size"`
	BaselineWindow time.Duration `json:"baseline_window"`
	SpikeThreshold float64       `json:"spike_threshold"` // 标准差倍数
	DDoSThreshold  int64         `json:"ddos_threshold"`  // 连接数/秒
	PortScanThresh int           `json:"portscan_thresh"` // 不同端口数
	ExfilThreshold int64         `json:"exfil_threshold"` // 异常出站字节数
	MaxFlows       int           `json:"max_flows"`
}

// DefaultAnalyzerConfig 默认配置.
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		WindowSize:     5 * time.Minute,
		BaselineWindow: 24 * time.Hour,
		SpikeThreshold: 3.0,
		DDoSThreshold:  1000,
		PortScanThresh: 100,
		ExfilThreshold: 10 * 1024 * 1024 * 1024, // 10GB
		MaxFlows:       100000,
	}
}

// Analyzer 分析器.
type Analyzer struct {
	config    *AnalyzerConfig
	flows     []FlowRecord
	baseline  *TrafficStats
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	anomalyCh chan Anomaly
}

// NewAnalyzer 创建分析器.
func NewAnalyzer(config *AnalyzerConfig) *Analyzer {
	if config == nil {
		config = DefaultAnalyzerConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Analyzer{
		config:    config,
		flows:     make([]FlowRecord, 0, config.MaxFlows),
		ctx:       ctx,
		cancel:    cancel,
		anomalyCh: make(chan Anomaly, 100),
	}
}

// Start 启动分析器.
func (a *Analyzer) Start() {
	go a.analysisLoop()
}

// Stop 停止分析器.
func (a *Analyzer) Stop() {
	a.cancel()
	close(a.anomalyCh)
}

// IngestFlow 导入流量记录.
func (a *Analyzer) IngestFlow(record FlowRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.flows) >= a.config.MaxFlows {
		a.flows = a.flows[1:]
	}
	a.flows = append(a.flows, record)

	// 实时检测
	a.detectAnomaly(record)
}

// GetStats 获取流量统计.
func (a *Analyzer) GetStats() *TrafficStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := &TrafficStats{
		ProtocolDist: make(map[string]int64),
	}

	appMap := make(map[string]*AppTraffic)
	connMap := make(map[string]*ConnTraffic)

	for _, f := range a.flows {
		stats.TotalBytesIn += f.BytesIn
		stats.TotalBytesOut += f.BytesOut

		stats.ProtocolDist[f.Protocol] += f.BytesIn + f.BytesOut

		key := f.Application
		if key == "" {
			key = "unknown"
		}
		if _, ok := appMap[key]; !ok {
			appMap[key] = &AppTraffic{App: key}
		}
		appMap[key].BytesIn += f.BytesIn
		appMap[key].BytesOut += f.BytesOut
		appMap[key].Total += f.BytesIn + f.BytesOut

		connKey := f.SrcIP + "->" + f.DstIP
		if _, ok := connMap[connKey]; !ok {
			connMap[connKey] = &ConnTraffic{SrcIP: f.SrcIP, DstIP: f.DstIP}
		}
		connMap[connKey].Total += f.BytesIn + f.BytesOut
		connMap[connKey].Duration += f.Duration
	}

	// 计算带宽
	if len(a.flows) > 0 {
		duration := a.flows[len(a.flows)-1].Timestamp.Sub(a.flows[0].Timestamp)
		if duration > 0 {
			stats.AvgBandwidth = float64(stats.TotalBytesIn+stats.TotalBytesOut) / duration.Seconds() / 1024 / 1024 * 8
		}
	}

	// Top apps
	totalAll := stats.TotalBytesIn + stats.TotalBytesOut
	for _, app := range appMap {
		if totalAll > 0 {
			app.Pct = float64(app.Total) / float64(totalAll) * 100
		}
		stats.TopApps = append(stats.TopApps, *app)
	}

	// Top connections
	for _, conn := range connMap {
		stats.TopConnections = append(stats.TopConnections, *conn)
	}

	// 趋势分析
	stats.TrendDirection, stats.TrendPct = a.calculateTrend()

	return stats
}

// GetAnomalies 获取异常列表.
func (a *Analyzer) GetAnomalies() []Anomaly {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 从 channel 收集
	var anomalies []Anomaly
	for {
		select {
		case anom := <-a.anomalyCh:
			anomalies = append(anomalies, anom)
		default:
			return anomalies
		}
	}
}

// detectAnomaly 实时异常检测.
func (a *Analyzer) detectAnomaly(f FlowRecord) {
	// DDoS 检测
	if f.PacketsIn > a.config.DDoSThreshold {
		a.anomalyCh <- Anomaly{
			Timestamp:     f.Timestamp,
			Type:          "ddos",
			Severity:      "high",
			Source:        f.SrcIP,
			Target:        f.DstIP,
			Score:         0.8,
			Details:       "高频率连接检测",
			BytesAffected: f.BytesIn,
		}
	}

	// 异常出站检测
	if f.BytesOut > a.config.ExfilThreshold {
		a.anomalyCh <- Anomaly{
			Timestamp:     f.Timestamp,
			Type:          "exfiltration",
			Severity:      "critical",
			Source:        f.SrcIP,
			Target:        f.DstIP,
			Score:         0.9,
			Details:       "大量数据外传检测",
			BytesAffected: f.BytesOut,
		}
	}

	// 流量突增检测
	if a.baseline != nil && a.baseline.AvgBandwidth > 0 {
		currentBW := float64(f.BytesIn+f.BytesOut) / 1024 / 1024 * 8
		if currentBW > a.baseline.AvgBandwidth*a.config.SpikeThreshold {
			a.anomalyCh <- Anomaly{
				Timestamp:     f.Timestamp,
				Type:          "spike",
				Severity:      "medium",
				Source:        f.SrcIP,
				Target:        f.DstIP,
				Score:         0.6,
				Details:       "流量突增检测",
				BytesAffected: f.BytesIn + f.BytesOut,
			}
		}
	}
}

// calculateTrend 计算流量趋势.
func (a *Analyzer) calculateTrend() (string, float64) {
	if len(a.flows) < 2 {
		return "stable", 0
	}

	mid := len(a.flows) / 2
	var firstHalf, secondHalf int64

	for i, f := range a.flows {
		total := f.BytesIn + f.BytesOut
		if i < mid {
			firstHalf += total
		} else {
			secondHalf += total
		}
	}

	if firstHalf == 0 {
		return "up", 100
	}

	change := float64(secondHalf-firstHalf) / float64(firstHalf) * 100
	change = math.Round(change*10) / 10

	if change > 10 {
		return "up", change
	} else if change < -10 {
		return "down", math.Abs(change)
	}
	return "stable", math.Abs(change)
}

// analysisLoop 分析循环.
func (a *Analyzer) analysisLoop() {
	ticker := time.NewTicker(a.config.WindowSize)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.updateBaseline()
		}
	}
}

// updateBaseline 更新基线.
func (a *Analyzer) updateBaseline() {
	a.mu.Lock()
	defer a.mu.Unlock()

	stats := &TrafficStats{
		ProtocolDist: make(map[string]int64),
	}

	for _, f := range a.flows {
		stats.TotalBytesIn += f.BytesIn
		stats.TotalBytesOut += f.BytesOut
	}

	if len(a.flows) > 0 {
		duration := a.flows[len(a.flows)-1].Timestamp.Sub(a.flows[0].Timestamp)
		if duration > 0 {
			stats.AvgBandwidth = float64(stats.TotalBytesIn+stats.TotalBytesOut) / duration.Seconds() / 1024 / 1024 * 8
		}
	}

	a.baseline = stats
}
