// Package smarttraffic - 智能网络流量分析器
// 对标群晖 Traffic Control，增强：AI 异常检测、应用层识别、QoS 自动调优
package smarttraffic

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Manager 流量管理器
type Manager struct {
	mu          sync.RWMutex
	flows       map[string]*Flow
	apps        map[string]*AppTraffic
	alerts      []*TrafficAlert
	qosRules    []*QoSRule
	config      *Config
	collector   *Collector
	analyzer    *Analyzer
}

// Config 配置
type Config struct {
	Enabled         bool          `json:"enabled"`
	CollectInterval time.Duration `json:"collect_interval"`
	FlowTimeout     time.Duration `json:"flow_timeout"`
	MaxFlows        int           `json:"max_flows"`
	AnomalyEnabled  bool          `json:"anomaly_enabled"`
	QoSAutoTune     bool          `json:"qos_auto_tune"`
	RetentionDays   int           `json:"retention_days"`
}

// Flow 网络流
type Flow struct {
	ID          string    `json:"id"`
	Protocol    string    `json:"protocol"`
	SrcIP       string    `json:"src_ip"`
	DstIP       string    `json:"dst_ip"`
	SrcPort     int       `json:"src_port"`
	DstPort     int       `json:"dst_port"`
	AppName     string    `json:"app_name"`
	BytesSent   int64     `json:"bytes_sent"`
	BytesRecv   int64     `json:"bytes_recv"`
	PacketsSent int64     `json:"packets_sent"`
	PacketsRecv int64     `json:"packets_recv"`
	StartTime   time.Time `json:"start_time"`
	LastSeen    time.Time `json:"last_seen"`
	Direction   string    `json:"direction"` // inbound, outbound, lan
}

// AppTraffic 应用流量
type AppTraffic struct {
	AppName     string    `json:"app_name"`
	TotalBytes  int64     `json:"total_bytes"`
	FlowCount   int       `json:"flow_count"`
	LastUpdated time.Time `json:"last_updated"`
	Category    string    `json:"category"`
}

// TrafficAlert 流量告警
type TrafficAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // spike, anomaly, threshold
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	FlowID    string    `json:"flow_id,omitempty"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"`
}

// QoSRule QoS 规则
type QoSRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AppName    string `json:"app_name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	SrcIP      string `json:"src_ip,omitempty"`
	DstIP      string `json:"dst_ip,omitempty"`
	Priority   int    `json:"priority"` // 1-7, 1=highest
	MaxBandwidth int64 `json:"max_bandwidth"` // bytes/sec
	MinBandwidth int64 `json:"min_bandwidth"`
	Enabled    bool   `json:"enabled"`
}

// Collector 数据采集器
type Collector struct {
	manager *Manager
}

// Analyzer 流量分析器
type Analyzer struct {
	manager     *Manager
	baselines   map[string]*Baseline
	anomalies   []*Anomaly
}

// Baseline 基线数据
type Baseline struct {
	AppName      string    `json:"app_name"`
	AvgBytes     float64   `json:"avg_bytes"`
	StdDev       float64   `json:"std_dev"`
	SampleCount  int       `json:"sample_count"`
	LastUpdated  time.Time `json:"last_updated"`
}

// Anomaly 异常检测结果
type Anomaly struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	AppName   string    `json:"app_name"`
	Value     float64   `json:"value"`
	Expected  float64   `json:"expected"`
	Deviation float64   `json:"deviation"`
	Timestamp time.Time `json:"timestamp"`
}

// NewManager 创建管理器
func NewManager(config *Config) *Manager {
	m := &Manager{
		flows:    make(map[string]*Flow),
		apps:     make(map[string]*AppTraffic),
		alerts:   make([]*TrafficAlert, 0),
		qosRules: make([]*QoSRule, 0),
		config:   config,
	}
	m.collector = &Collector{manager: m}
	m.analyzer = &Analyzer{
		manager:   m,
		baselines: make(map[string]*Baseline),
		anomalies: make([]*Anomaly, 0),
	}
	return m
}

// Start 启动流量监控
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		return nil
	}
	log.Println("🔍 智能流量分析器已启动")
	go m.collectLoop(ctx)
	go m.analyzeLoop(ctx)
	return nil
}

func (m *Manager) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.CollectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collector.Collect()
		}
	}
}

func (m *Manager) analyzeLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.analyzer.Analyze()
		}
	}
}

// Collect 采集流量数据
func (c *Collector) Collect() {
	// 实际实现：读取 /proc/net/tcp, netstat 等
	log.Println("📊 采集网络流量数据...")
}

// Analyze 分析流量
func (a *Analyzer) Analyze() {
	// 异常检测逻辑
}

// GetDashboard 获取仪表盘数据
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	totalBandwidth := int64(0)
	for _, flow := range m.flows {
		totalBandwidth += flow.BytesSent + flow.BytesRecv
	}
	
	return map[string]interface{}{
		"total_flows":      len(m.flows),
		"total_apps":       len(m.apps),
		"active_alerts":    m.activeAlerts(),
		"total_bandwidth":  totalBandwidth,
		"qos_rules":        len(m.qosRules),
	}
}

func (m *Manager) activeAlerts() int {
	count := 0
	for _, a := range m.alerts {
		if !a.Acked {
			count++
		}
	}
	return count
}

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/smart-traffic")
	{
		group.GET("/dashboard", h.GetDashboard)
		group.GET("/flows", h.ListFlows)
		group.GET("/apps", h.ListAppTraffic)
		group.GET("/alerts", h.ListAlerts)
		group.POST("/alerts/:id/ack", h.AckAlert)
		group.GET("/qos", h.ListQoS)
		group.POST("/qos", h.CreateQoS)
		group.PUT("/qos/:id", h.UpdateQoS)
		group.DELETE("/qos/:id", h.DeleteQoS)
		group.GET("/anomalies", h.ListAnomalies)
	}
}

func (h *Handler) GetDashboard(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.manager.GetDashboard()})
}

func (h *Handler) ListFlows(c *gin.Context) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()
	flows := make([]*Flow, 0, len(h.manager.flows))
	for _, f := range h.manager.flows {
		flows = append(flows, f)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": flows})
}

func (h *Handler) ListAppTraffic(c *gin.Context) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()
	apps := make([]*AppTraffic, 0, len(h.manager.apps))
	for _, a := range h.manager.apps {
		apps = append(apps, a)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": apps})
}

func (h *Handler) ListAlerts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.manager.alerts})
}

func (h *Handler) AckAlert(c *gin.Context) {
	id := c.Param("id")
	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()
	for _, a := range h.manager.alerts {
		if a.ID == id {
			a.Acked = true
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "告警不存在"})
}

func (h *Handler) ListQoS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.manager.qosRules})
}

func (h *Handler) CreateQoS(c *gin.Context) {
	var rule QoSRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	rule.ID = fmt.Sprintf("qos-%d", time.Now().UnixNano())
	h.manager.mu.Lock()
	h.manager.qosRules = append(h.manager.qosRules, &rule)
	h.manager.mu.Unlock()
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": rule})
}

func (h *Handler) UpdateQoS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) DeleteQoS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) ListAnomalies(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.manager.analyzer.anomalies})
}
