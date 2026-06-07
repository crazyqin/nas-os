// Package trafficclassifier 提供流量分类核心逻辑
package trafficclassifier

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 流量分类管理器
type Manager struct {
	mu                sync.RWMutex
	logger            *zap.Logger
	config            *ClassifierConfig
	flows             map[string]*TrafficFlow
	stats             *TrafficStats
	anomalies         map[string]*AnomalyAlert
	rules             map[string]*ClassificationRule
	bandwidthPolicies map[string]*BandwidthPolicy
	mirrorConfigs     map[string]*MirrorConfig
	qosRules          map[string]*QoSRule
	dpiSignatures     []DPISignature
	reports           []*TrafficReport
	stopChan          chan struct{}
	running           bool
}

// NewManager 创建流量分类管理器
func NewManager(logger *zap.Logger, config *ClassifierConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultClassifierConfig()
	}
	m := &Manager{
		logger:            logger,
		config:            config,
		flows:             make(map[string]*TrafficFlow),
		stats:             &TrafficStats{FlowsByType: make(map[TrafficType]int64), BytesByType: make(map[TrafficType]int64), ProtocolBreakdown: make(map[string]int64)},
		anomalies:         make(map[string]*AnomalyAlert),
		rules:             make(map[string]*ClassificationRule),
		bandwidthPolicies: make(map[string]*BandwidthPolicy),
		mirrorConfigs:     make(map[string]*MirrorConfig),
		qosRules:          make(map[string]*QoSRule),
		reports:           make([]*TrafficReport, 0),
		stopChan:          make(chan struct{}),
	}
	m.initDPISignatures()
	m.initDefaultPolicies()
	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDPISignatures 初始化 DPI 签名库
func (m *Manager) initDPISignatures() {
	m.dpiSignatures = []DPISignature{
		{Name: "HTTP", Pattern: "^(GET|POST|PUT|DELETE|HEAD|OPTIONS|PATCH) ", TrafficType: TrafficTypeOffice, Port: 80, Protocol: "tcp"},
		{Name: "HTTPS", Pattern: "\x16\x03", TrafficType: TrafficTypeOffice, Port: 443, Protocol: "tcp"},
		{Name: "RTSP", Pattern: "^RTSP/", TrafficType: TrafficTypeVideo, Port: 554, Protocol: "tcp"},
		{Name: "RTMP", Pattern: "\x03", TrafficType: TrafficTypeVideo, Port: 1935, Protocol: "tcp"},
		{Name: "HLS", Pattern: "\\.m3u8", TrafficType: TrafficTypeVideo, Port: 80, Protocol: "tcp"},
		{Name: "RTCP", Pattern: "\x80\xc8", TrafficType: TrafficTypeAudio, Port: 5005, Protocol: "udp"},
		{Name: "SIP", Pattern: "^INVITE sip:|^REGISTER sip:", TrafficType: TrafficTypeAudio, Port: 5060, Protocol: "tcp"},
		{Name: "RTP", Pattern: "\x80", TrafficType: TrafficTypeAudio, Port: 5004, Protocol: "udp"},
		{Name: "Steam", Pattern: "steam", TrafficType: TrafficTypeGame, Port: 27015, Protocol: "udp"},
		{Name: "Xbox", Pattern: "xbox", TrafficType: TrafficTypeGame, Port: 3074, Protocol: "udp"},
		{Name: "PSN", Pattern: "psn", TrafficType: TrafficTypeGame, Port: 9293, Protocol: "tcp"},
		{Name: "BitTorrent", Pattern: "\x13BitTorrent protocol", TrafficType: TrafficTypeDownload, Port: 6881, Protocol: "tcp"},
		{Name: "FTP", Pattern: "^220 .* FTP", TrafficType: TrafficTypeDownload, Port: 21, Protocol: "tcp"},
		{Name: "SSH", Pattern: "^SSH-", TrafficType: TrafficTypeOffice, Port: 22, Protocol: "tcp"},
		{Name: "MQTT", Pattern: "\x10", TrafficType: TrafficTypeIoT, Port: 1883, Protocol: "tcp"},
		{Name: "CoAP", Pattern: "\x40", TrafficType: TrafficTypeIoT, Port: 5683, Protocol: "udp"},
		{Name: "DNS", Pattern: "", TrafficType: TrafficTypeOffice, Port: 53, Protocol: "udp"},
	}
}

// initDefaultPolicies 初始化默认带宽策略
func (m *Manager) initDefaultPolicies() {
	now := time.Now()
	defaults := []BandwidthPolicy{
		{ID: "policy-video", Name: "视频流量", TrafficType: TrafficTypeVideo, MinMbps: 10, MaxMbps: 100, Priority: 3, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "policy-audio", Name: "音频流量", TrafficType: TrafficTypeAudio, MinMbps: 2, MaxMbps: 20, Priority: 2, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "policy-game", Name: "游戏流量", TrafficType: TrafficTypeGame, MinMbps: 5, MaxMbps: 50, Priority: 1, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "policy-download", Name: "下载流量", TrafficType: TrafficTypeDownload, MinMbps: 5, MaxMbps: 200, Priority: 7, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "policy-office", Name: "办公流量", TrafficType: TrafficTypeOffice, MinMbps: 5, MaxMbps: 100, Priority: 4, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "policy-iot", Name: "IoT 流量", TrafficType: TrafficTypeIoT, MinMbps: 1, MaxMbps: 10, Priority: 5, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "policy-unknown", Name: "未知流量", TrafficType: TrafficTypeUnknown, MinMbps: 1, MaxMbps: 50, Priority: 10, Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	for i := range defaults {
		m.bandwidthPolicies[defaults[i].ID] = &defaults[i]
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("manager already running")
	}
	m.running = true
	m.logger.Info("traffic classifier started")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
	m.logger.Info("traffic classifier stopped")
}

// IsRunning 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// AnalyzeFlows 分析流量
func (m *Manager) AnalyzeFlows(req *AnalyzeRequest) (*AnalyzeResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("traffic classifier is disabled")
	}

	start := time.Now()
	m.logger.Info("analyzing flows", zap.Int("count", len(req.Flows)))

	results := make([]ClassificationResult, 0, len(req.Flows))
	alerts := make([]AnomalyAlert, 0)

	for i := range req.Flows {
		flow := &req.Flows[i]

		// 提取特征
		features := m.extractFeatures(flow)

		// 分类
		result := m.classifyFlow(flow, features, req.WithDPI)
		results = append(results, *result)

		// 更新流量类型
		flow.TrafficType = result.TrafficType
		flow.Confidence = result.Confidence

		// 异常检测
		if alert := m.detectAnomaly(flow, features); alert != nil {
			flow.IsAnomaly = true
			flow.AnomalyType = alert.AnomalyType
			alerts = append(alerts, *alert)
			m.mu.Lock()
			m.anomalies[alert.ID] = alert
			m.mu.Unlock()
		}

		// 存储流
		m.mu.Lock()
		m.flows[flow.ID] = flow
		m.mu.Unlock()
	}

	// 更新统计
	m.updateStats(req.Flows)

	processMs := time.Since(start).Milliseconds()

	return &AnalyzeResponse{
		Results:   results,
		Stats:     m.GetStats(),
		Alerts:    alerts,
		Status:    AnalysisStatusCompleted,
		ProcessMs: processMs,
	}, nil
}

// extractFeatures 提取流量特征
func (m *Manager) extractFeatures(flow *TrafficFlow) *TrafficFeature {
	totalPackets := flow.PacketsIn + flow.PacketsOut
	totalBytes := flow.BytesIn + flow.BytesOut

	avgPktSize := float64(0)
	if totalPackets > 0 {
		avgPktSize = float64(totalBytes) / float64(totalPackets)
	}

	duration := flow.EndTime.Sub(flow.StartTime).Seconds()
	if duration <= 0 {
		duration = 0.001
	}

	pktRate := float64(totalPackets) / duration
	byteRate := float64(totalBytes) / duration

	return &TrafficFeature{
		AvgPacketSize: avgPktSize,
		StdPacketSize: avgPktSize * 0.3, // 简化估计
		PacketRate:    pktRate,
		ByteRate:      byteRate,
		Protocol:      flow.Protocol,
		DstPort:       flow.DstPort,
		FlowDuration:  duration,
		PushFlagRatio: 0.5, // 简化
		SynFlagRatio:  0.1, // 简化
		BurstCount:    estimateBursts(totalPackets, duration),
	}
}

// estimateBursts 估计突发数
func estimateBursts(packets int64, duration float64) int {
	if duration <= 0 {
		return 0
	}
	rate := float64(packets) / duration
	if rate > 1000 {
		return int(rate / 100)
	}
	return int(rate / 10)
}

// classifyFlow 分类流量流
func (m *Manager) classifyFlow(flow *TrafficFlow, features *TrafficFeature, withDPI bool) *ClassificationResult {
	result := &ClassificationResult{
		FlowID:    flow.ID,
		Features:  features,
		Timestamp: time.Now(),
	}

	// 1. 自定义规则匹配
	m.mu.RLock()
	rules := make([]*ClassificationRule, 0, len(m.rules))
	for _, r := range m.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	m.mu.RUnlock()

	// 按优先级排序
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	for _, rule := range rules {
		if m.matchRule(rule, flow) {
			result.TrafficType = rule.TrafficType
			result.Confidence = 0.95
			result.RuleName = rule.Name
			return result
		}
	}

	// 2. DPI 检测
	if withDPI {
		if dpiType, name := m.dpiInspect(flow); dpiType != TrafficTypeUnknown {
			result.TrafficType = dpiType
			result.Confidence = 0.90
			result.ModelName = "dpi-" + name
			return result
		}
	}

	// 3. 端口启发式
	portType := m.classifyByPort(flow.DstPort)
	if portType != TrafficTypeUnknown {
		result.TrafficType = portType
		result.Confidence = 0.70
		result.ModelName = "port-heuristic"
		return result
	}

	// 4. 基于特征的简单启发式
	result.TrafficType = m.classifyByFeatures(features)
	result.Confidence = 0.50
	result.ModelName = "feature-heuristic"
	return result
}

// matchRule 匹配自定义规则
func (m *Manager) matchRule(rule *ClassificationRule, flow *TrafficFlow) bool {
	if rule.SrcIPPattern != "" {
		if matched, _ := matchPattern(rule.SrcIPPattern, flow.SrcIP); !matched {
			return false
		}
	}
	if rule.DstIPPattern != "" {
		if matched, _ := matchPattern(rule.DstIPPattern, flow.DstIP); !matched {
			return false
		}
	}
	if len(rule.Ports) > 0 {
		found := false
		for _, p := range rule.Ports {
			if p == flow.DstPort || p == flow.SrcPort {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if rule.Protocol != "" && !strings.EqualFold(rule.Protocol, flow.Protocol) {
		return false
	}
	if rule.MinPacketSize > 0 {
		avgSize := (flow.BytesIn + flow.BytesOut) / max64(flow.PacketsIn+flow.PacketsOut, 1)
		if avgSize < int64(rule.MinPacketSize) {
			return false
		}
	}
	if rule.MaxPacketSize > 0 {
		avgSize := (flow.BytesIn + flow.BytesOut) / max64(flow.PacketsIn+flow.PacketsOut, 1)
		if avgSize > int64(rule.MaxPacketSize) {
			return false
		}
	}
	return true
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// matchPattern 匹配 IP 模式(CIDR 或正则)
func matchPattern(pattern, ip string) (bool, error) {
	// 尝试 CIDR
	if strings.Contains(pattern, "/") {
		_, cidr, err := net.ParseCIDR(pattern)
		if err != nil {
			return false, err
		}
		return cidr.Contains(net.ParseIP(ip)), nil
	}
	// 精确匹配
	if pattern == ip {
		return true, nil
	}
	// 正则匹配
	return regexp.MatchString(pattern, ip)
}

// classifyByPort 基于端口分类
func (m *Manager) classifyByPort(port int) TrafficType {
	switch {
	case port == 80 || port == 443 || port == 8080 || port == 8443:
		return TrafficTypeOffice
	case port == 22 || port == 3389 || port == 5900:
		return TrafficTypeOffice
	case port == 554 || port == 1935 || port == 8554:
		return TrafficTypeVideo
	case port == 5060 || port == 5061 || port == 5004:
		return TrafficTypeAudio
	case port == 27015 || port == 27016 || port == 3074 || port == 9293 || port == 3478:
		return TrafficTypeGame
	case port == 21 || port == 6881 || port == 6882 || port == 6969:
		return TrafficTypeDownload
	case port == 1883 || port == 8883 || port == 5683 || port == 5684:
		return TrafficTypeIoT
	case port == 53:
		return TrafficTypeOffice
	default:
		return TrafficTypeUnknown
	}
}

// classifyByFeatures 基于特征分类
func (m *Manager) classifyByFeatures(f *TrafficFeature) TrafficType {
	// 高带宽 + 长持续时间 → 视频或下载
	if f.ByteRate > 5000000 { // >5Mbps
		if f.FlowDuration > 60 {
			return TrafficTypeVideo
		}
		return TrafficTypeDownload
	}

	// 低带宽 + 低包率 → IoT
	if f.ByteRate < 10000 && f.PacketRate < 10 {
		return TrafficTypeIoT
	}

	// 中等带宽 + 高包率 → 游戏
	if f.PacketRate > 30 && f.AvgPacketSize < 200 {
		return TrafficTypeGame
	}

	// 低带宽 + 中等包率 → 音频
	if f.ByteRate < 200000 && f.PacketRate > 20 && f.PacketRate < 100 {
		return TrafficTypeAudio
	}

	return TrafficTypeUnknown
}

// dpiInspect 深度包检测
func (m *Manager) dpiInspect(flow *TrafficFlow) (TrafficType, string) {
	// 基于端口和协议匹配签名
	for _, sig := range m.dpiSignatures {
		if sig.Port > 0 && (sig.Port == flow.DstPort || sig.Port == flow.SrcPort) {
			if sig.Protocol == "" || strings.EqualFold(sig.Protocol, flow.Protocol) {
				return sig.TrafficType, sig.Name
			}
		}
	}
	return TrafficTypeUnknown, ""
}

// detectAnomaly 检测异常流量
func (m *Manager) detectAnomaly(flow *TrafficFlow, features *TrafficFeature) *AnomalyAlert {
	// DDoS 检测：高包率 + 小包
	if features.PacketRate > 10000 && features.AvgPacketSize < 100 {
		return m.createAlert(AnomalyTypeDDoS, AlertSeverityCritical, flow, "检测到疑似 DDoS 攻击：高包率小包流量")
	}

	// 流量泛洪检测
	if features.PacketRate > 5000 && flow.PacketsIn > 100000 {
		return m.createAlert(AnomalyTypeFlood, AlertSeverityHigh, flow, "检测到流量泛洪异常")
	}

	// 挖矿检测：长连接 + 稳定小包
	if features.FlowDuration > 3600 && features.AvgPacketSize < 200 && features.PacketRate > 1 && features.PacketRate < 10 {
		return m.createAlert(AnomalyTypeMining, AlertSeverityHigh, flow, "检测到疑似挖矿行为：长时间稳定小包连接")
	}

	// 数据泄露检测：大出站流量
	if flow.BytesOut > 1073741824 && float64(flow.BytesOut)/float64(flow.BytesIn) > 10 { // >1GB 出站，出/入比>10
		return m.createAlert(AnomalyTypeDataLeak, AlertSeverityCritical, flow, "检测到疑似数据泄露：异常大出站流量")
	}

	// 端口扫描检测：多端口短连接
	if features.FlowDuration < 1 && features.SynFlagRatio > 0.8 {
		return m.createAlert(AnomalyTypeScan, AlertSeverityMedium, flow, "检测到疑似端口扫描")
	}

	return nil
}

// createAlert 创建告警
func (m *Manager) createAlert(atype AnomalyType, severity AlertSeverity, flow *TrafficFlow, desc string) *AnomalyAlert {
	return &AnomalyAlert{
		ID:          generateID(),
		AnomalyType: atype,
		Severity:    severity,
		SourceIP:    flow.SrcIP,
		DestIP:      flow.DstIP,
		Description: desc,
		FlowIDs:     []string{flow.ID},
		BytesTotal:  flow.BytesIn + flow.BytesOut,
		FirstSeen:   flow.StartTime,
		LastSeen:    flow.EndTime,
		IsResolved:  false,
	}
}

// updateStats 更新统计信息
func (m *Manager) updateStats(flows []TrafficFlow) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, flow := range flows {
		m.stats.TotalBytes += flow.BytesIn + flow.BytesOut
		m.stats.TotalPackets += flow.PacketsIn + flow.PacketsOut
		m.stats.FlowsByType[flow.TrafficType]++
		m.stats.BytesByType[flow.TrafficType] += flow.BytesIn + flow.BytesOut
		m.stats.ProtocolBreakdown[flow.Protocol]++
		if flow.IsAnomaly {
			m.stats.AnomalyCount++
		}
	}
	m.stats.ActiveFlows = len(m.flows)
	m.stats.Timestamp = time.Now()

	// 更新 top talkers
	m.updateTopTalkers(flows)
}

// updateTopTalkers 更新 Top Talkers
func (m *Manager) updateTopTalkers(flows []TrafficFlow) {
	talkerMap := make(map[string]*EndpointStats)
	for _, flow := range flows {
		if ts, ok := talkerMap[flow.SrcIP]; ok {
			ts.BytesOut += flow.BytesOut
			ts.FlowCount++
		} else {
			talkerMap[flow.SrcIP] = &EndpointStats{IP: flow.SrcIP, BytesOut: flow.BytesOut, FlowCount: 1}
		}
		if ts, ok := talkerMap[flow.DstIP]; ok {
			ts.BytesIn += flow.BytesIn
		} else {
			talkerMap[flow.DstIP] = &EndpointStats{IP: flow.DstIP, BytesIn: flow.BytesIn}
		}
	}
	talkers := make([]EndpointStats, 0, len(talkerMap))
	for _, ts := range talkerMap {
		talkers = append(talkers, *ts)
	}
	sort.Slice(talkers, func(i, j int) bool {
		return (talkers[i].BytesIn + talkers[i].BytesOut) > (talkers[j].BytesIn + talkers[j].BytesOut)
	})
	if len(talkers) > 10 {
		talkers = talkers[:10]
	}
	m.stats.TopTalkers = talkers
}

// GetStats 获取流量统计
func (m *Manager) GetStats() *TrafficStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := *m.stats
	stats.FlowsByType = make(map[TrafficType]int64)
	for k, v := range m.stats.FlowsByType {
		stats.FlowsByType[k] = v
	}
	stats.BytesByType = make(map[TrafficType]int64)
	for k, v := range m.stats.BytesByType {
		stats.BytesByType[k] = v
	}
	stats.ProtocolBreakdown = make(map[string]int64)
	for k, v := range m.stats.ProtocolBreakdown {
		stats.ProtocolBreakdown[k] = v
	}
	return &stats
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *ClassifierConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *ClassifierConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// --- 分类规则管理 ---

// AddRule 添加分类规则
func (m *Manager) AddRule(rule *ClassificationRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	m.logger.Info("rule added", zap.String("id", rule.ID), zap.String("name", rule.Name))
}

// UpdateRule 更新分类规则
func (m *Manager) UpdateRule(rule *ClassificationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[rule.ID]; !ok {
		return fmt.Errorf("rule %s not found", rule.ID)
	}
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除分类规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule %s not found", id)
	}
	delete(m.rules, id)
	return nil
}

// ListRules 列出所有规则
func (m *Manager) ListRules() []*ClassificationRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := make([]*ClassificationRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})
	return rules
}

// GetRule 获取单条规则
func (m *Manager) GetRule(id string) (*ClassificationRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rule, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule %s not found", id)
	}
	return rule, nil
}

// --- 带宽策略管理 ---

// AddBandwidthPolicy 添加带宽策略
func (m *Manager) AddBandwidthPolicy(policy *BandwidthPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if policy.ID == "" {
		policy.ID = generateID()
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	m.bandwidthPolicies[policy.ID] = policy
}

// ListBandwidthPolicies 列出带宽策略
func (m *Manager) ListBandwidthPolicies() []*BandwidthPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	policies := make([]*BandwidthPolicy, 0, len(m.bandwidthPolicies))
	for _, p := range m.bandwidthPolicies {
		policies = append(policies, p)
	}
	return policies
}

// GetBandwidthPolicy 获取带宽策略
func (m *Manager) GetBandwidthPolicy(id string) (*BandwidthPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.bandwidthPolicies[id]
	if !ok {
		return nil, fmt.Errorf("bandwidth policy %s not found", id)
	}
	return p, nil
}

// DeleteBandwidthPolicy 删除带宽策略
func (m *Manager) DeleteBandwidthPolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.bandwidthPolicies[id]; !ok {
		return fmt.Errorf("bandwidth policy %s not found", id)
	}
	delete(m.bandwidthPolicies, id)
	return nil
}

// --- 镜像管理 ---

// AddMirrorConfig 添加镜像配置
func (m *Manager) AddMirrorConfig(cfg *MirrorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg.ID == "" {
		cfg.ID = generateID()
	}
	cfg.CreatedAt = time.Now()
	m.mirrorConfigs[cfg.ID] = cfg
}

// ListMirrorConfigs 列出镜像配置
func (m *Manager) ListMirrorConfigs() []*MirrorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	configs := make([]*MirrorConfig, 0, len(m.mirrorConfigs))
	for _, c := range m.mirrorConfigs {
		configs = append(configs, c)
	}
	return configs
}

// DeleteMirrorConfig 删除镜像配置
func (m *Manager) DeleteMirrorConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.mirrorConfigs[id]; !ok {
		return fmt.Errorf("mirror config %s not found", id)
	}
	delete(m.mirrorConfigs, id)
	return nil
}

// --- QoS 规则管理 ---

// AddQoSRule 添加 QoS 规则
func (m *Manager) AddQoSRule(rule *QoSRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.qosRules[rule.ID] = rule
}

// ListQoSRules 列出 QoS 规则
func (m *Manager) ListQoSRules() []*QoSRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := make([]*QoSRule, 0, len(m.qosRules))
	for _, r := range m.qosRules {
		rules = append(rules, r)
	}
	return rules
}

// DeleteQoSRule 删除 QoS 规则
func (m *Manager) DeleteQoSRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.qosRules[id]; !ok {
		return fmt.Errorf("qos rule %s not found", id)
	}
	delete(m.qosRules, id)
	return nil
}

// --- 异常告警管理 ---

// ListAlerts 列出告警
func (m *Manager) ListAlerts(resolved bool) []*AnomalyAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alerts := make([]*AnomalyAlert, 0)
	for _, a := range m.mirrorConfigs {
		_ = a
	}
	for _, a := range m.anomalies {
		if a.IsResolved == resolved {
			alerts = append(alerts, a)
		}
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].FirstSeen.After(alerts[j].FirstSeen)
	})
	return alerts
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	alert, ok := m.anomalies[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	now := time.Now()
	alert.IsResolved = true
	alert.ResolvedAt = &now
	return nil
}

// --- 报告生成 ---

// GenerateReport 生成流量报告
func (m *Manager) GenerateReport(req *ReportRequest) *TrafficReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &TrafficReport{
		ID:             generateID(),
		Title:          req.Title,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		Stats:          m.GetStats(),
		BandwidthUsage: make(map[TrafficType]float64),
		GeneratedAt:    time.Now(),
	}

	if report.Title == "" {
		report.Title = fmt.Sprintf("流量报告 %s - %s",
			req.StartTime.Format("2006-01-02 15:04"),
			req.EndTime.Format("2006-01-02 15:04"))
	}

	// 计算带宽使用
	duration := req.EndTime.Sub(req.StartTime).Seconds()
	if duration > 0 {
		for ttype, bytes := range m.stats.BytesByType {
			mbps := float64(bytes) * 8 / duration / 1000000
			report.BandwidthUsage[ttype] = mbps
		}
	}

	// 获取异常
	alerts := make([]AnomalyAlert, 0)
	for _, a := range m.anomalies {
		if !a.FirstSeen.Before(req.StartTime) && !a.LastSeen.After(req.EndTime) {
			alerts = append(alerts, *a)
		}
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].FirstSeen.After(alerts[j].FirstSeen)
	})
	if len(alerts) > 10 {
		alerts = alerts[:10]
	}
	report.TopAnomalies = alerts

	// 生成摘要
	report.Summary = fmt.Sprintf("分析期间共 %d 条流量，总计 %.2f MB，检测到 %d 个异常",
		m.stats.ActiveFlows,
		float64(m.stats.TotalBytes)/1048576,
		m.stats.AnomalyCount)

	m.reports = append(m.reports, report)
	return report
}

// GetReport 获取报告
func (m *Manager) GetReport(id string) (*TrafficReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.reports {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("report %s not found", id)
}

// ListReports 列出报告
func (m *Manager) ListReports() []*TrafficReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	reports := make([]*TrafficReport, len(m.reports))
	copy(reports, m.reports)
	return reports
}
