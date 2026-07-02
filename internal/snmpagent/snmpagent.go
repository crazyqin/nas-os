package snmpagent

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// SNMPConfig 定义 SNMP 代理配置.
type SNMPConfig struct {
	ListenAddr string `json:"listen_addr"`        // 监听地址，默认 ":161"
	Community  string `json:"community"`          // SNMP v2c community 字符串
	Version    string `json:"version"`            // SNMP 版本："2c" 或 "3"
	Username   string `json:"username,omitempty"` // SNMPv3 用户名
	AuthKey    string `json:"auth_key,omitempty"` // SNMPv3 认证密钥
	PrivKey    string `json:"priv_key,omitempty"` // SNMPv3 加密密钥
	Enabled    bool   `json:"enabled"`            // 是否启用
}

// SNMPMetric 定义 SNMP 指标结构.
type SNMPMetric struct {
	OID       string            `json:"oid"`        // OID 标识符
	Name      string            `json:"name"`       // 指标名称
	Value     interface{}       `json:"value"`      // 指标值
	Type      string            `json:"type"`       // 类型：gauge/counter/string
	Labels    map[string]string `json:"labels"`     // 标签
	UpdatedAt time.Time         `json:"updated_at"` // 更新时间
}

// Agent 是 SNMP 监控代理.
type Agent struct {
	mu      sync.RWMutex
	config  SNMPConfig
	metrics map[string]*SNMPMetric
	stopCh  chan struct{}
	running bool
}

// NewAgent 创建一个新的 SNMP 代理.
func NewAgent(cfg *SNMPConfig) *Agent {
	if cfg == nil {
		cfg = &SNMPConfig{
			ListenAddr: ":161",
			Community:  "public",
			Version:    "2c",
			Enabled:    true,
		}
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":161"
	}
	if cfg.Community == "" {
		cfg.Community = "public"
	}
	if cfg.Version == "" {
		cfg.Version = "2c"
	}

	a := &Agent{
		config:  *cfg,
		metrics: make(map[string]*SNMPMetric),
		stopCh:  make(chan struct{}),
	}

	a.registerDefaultMetrics()
	return a
}

// Start 启动 SNMP 代理（模拟模式）.
func (a *Agent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("SNMP agent is already running")
	}

	a.running = true
	a.stopCh = make(chan struct{})
	log.Printf("✅ SNMP agent started (listen: %s, version: %s)", a.config.ListenAddr, a.config.Version)
	return nil
}

// Stop 停止 SNMP 代理.
func (a *Agent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return fmt.Errorf("SNMP agent is not running")
	}

	close(a.stopCh)
	a.running = false
	log.Println("🛑 SNMP agent stopped")
	return nil
}

// RegisterMetric 注册一个新的 SNMP 指标.
func (a *Agent) RegisterMetric(oid, name string, value interface{}, metricType string, labels map[string]string) error {
	if oid == "" {
		return fmt.Errorf("OID cannot be empty")
	}
	if metricType != "gauge" && metricType != "counter" && metricType != "string" {
		return fmt.Errorf("invalid metric type: %s (must be gauge/counter/string)", metricType)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.metrics[oid]; exists {
		return fmt.Errorf("metric with OID %s already exists", oid)
	}

	a.metrics[oid] = &SNMPMetric{
		OID:       oid,
		Name:      name,
		Value:     value,
		Type:      metricType,
		Labels:    labels,
		UpdatedAt: time.Now(),
	}
	return nil
}

// UpdateMetric 更新指定 OID 的指标值.
func (a *Agent) UpdateMetric(oid string, value interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	m, ok := a.metrics[oid]
	if !ok {
		return fmt.Errorf("metric not found: %s", oid)
	}

	m.Value = value
	m.UpdatedAt = time.Now()
	return nil
}

// GetMetric 获取指定 OID 的指标.
func (a *Agent) GetMetric(oid string) (*SNMPMetric, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	m, ok := a.metrics[oid]
	if !ok {
		return nil, fmt.Errorf("metric not found: %s", oid)
	}

	cp := *m
	return &cp, nil
}

// ListMetrics 列出所有已注册的指标.
func (a *Agent) ListMetrics() []SNMPMetric {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]SNMPMetric, 0, len(a.metrics))
	for _, m := range a.metrics {
		result = append(result, *m)
	}
	return result
}

// UnregisterMetric 注销指定 OID 的指标.
func (a *Agent) UnregisterMetric(oid string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.metrics[oid]; !ok {
		return fmt.Errorf("metric not found: %s", oid)
	}

	delete(a.metrics, oid)
	return nil
}

// GetStatus 返回 SNMP 代理的运行状态.
func (a *Agent) GetStatus() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"running":      a.running,
		"listen_addr":  a.config.ListenAddr,
		"community":    a.config.Community,
		"version":      a.config.Version,
		"metric_count": len(a.metrics),
	}
}

// GetConfig 返回当前 SNMP 配置.
func (a *Agent) GetConfig() SNMPConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// UpdateConfig 更新 SNMP 配置.
func (a *Agent) UpdateConfig(cfg SNMPConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg
}

// registerDefaultMetrics 预注册常用的 NAS SNMP 指标.
func (a *Agent) registerDefaultMetrics() {
	defaults := []struct {
		oid    string
		name   string
		value  interface{}
		typ    string
		labels map[string]string
	}{
		// CPU 使用率
		{"1.3.6.1.4.1.2021.11.9.0", "cpu_user_percent", 5.0, "gauge", map[string]string{"component": "cpu"}},
		{"1.3.6.1.4.1.2021.11.10.0", "cpu_system_percent", 2.0, "gauge", map[string]string{"component": "cpu"}},
		{"1.3.6.1.4.1.2021.11.11.0", "cpu_idle_percent", 93.0, "gauge", map[string]string{"component": "cpu"}},

		// 内存
		{"1.3.6.1.4.1.2021.4.5.0", "mem_total_bytes", 8589934592, "gauge", map[string]string{"component": "memory"}},
		{"1.3.6.1.4.1.2021.4.6.0", "mem_avail_bytes", 4294967296, "gauge", map[string]string{"component": "memory"}},
		{"1.3.6.1.4.1.2021.4.14.0", "mem_buffer_bytes", 268435456, "gauge", map[string]string{"component": "memory"}},

		// 磁盘空间
		{"1.3.6.1.4.1.2021.9.1.7.1", "disk_total_bytes", 1099511627776, "gauge", map[string]string{"component": "disk", "device": "/dev/sda"}},
		{"1.3.6.1.4.1.2021.9.1.8.1", "disk_avail_bytes", 549755813888, "gauge", map[string]string{"component": "disk", "device": "/dev/sda"}},

		// 网络流量
		{"1.3.6.1.2.1.2.2.1.10.1", "if_in_octets", 0, "counter", map[string]string{"component": "network", "interface": "eth0"}},
		{"1.3.6.1.2.1.2.2.1.16.1", "if_out_octets", 0, "counter", map[string]string{"component": "network", "interface": "eth0"}},

		// 系统信息
		{"1.3.6.1.2.1.1.3.0", "sys_uptime_seconds", 0, "counter", map[string]string{"component": "system"}},
		{"1.3.6.1.2.1.1.5.0", "sys_hostname", "nas-os", "string", map[string]string{"component": "system"}},
	}

	for _, d := range defaults {
		a.metrics[d.oid] = &SNMPMetric{
			OID:       d.oid,
			Name:      d.name,
			Value:     d.value,
			Type:      d.typ,
			Labels:    d.labels,
			UpdatedAt: time.Now(),
		}
	}

	log.Printf("📋 已注册 %d 个默认 SNMP 指标", len(defaults))
}
