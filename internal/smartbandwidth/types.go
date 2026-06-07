package smartbandwidth

import (
	"fmt"
	"sync"
	"time"
)

// TrafficClass 流量类型
type TrafficClass string

const (
	TrafficClassVideo        TrafficClass = "video"         // 视频流
	TrafficClassFileTransfer TrafficClass = "file_transfer" // 文件传输
	TrafficClassBackup       TrafficClass = "backup"        // 备份
	TrafficClassAIInference  TrafficClass = "ai_inference"  // AI推理
	TrafficClassWeb          TrafficClass = "web"           // 网页浏览
	TrafficClassVoIP         TrafficClass = "voip"          // 语音通话
	TrafficClassGaming       TrafficClass = "gaming"        // 游戏
	TrafficClassStreaming    TrafficClass = "streaming"     // 流媒体
	TrafficClassDownload     TrafficClass = "download"      // 下载
	TrafficClassOther        TrafficClass = "other"         // 其他
)

// QoSPolicy QoS策略
type QoSPolicy struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Priority  int       `json:"priority"` // 优先级 1-10，10最高
	MinMbps   int64     `json:"min_mbps"` // 最小带宽保证
	MaxMbps   int64     `json:"max_mbps"` // 最大带宽限制
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrafficProfile 流量配置文件
type TrafficProfile struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	TrafficClass TrafficClass `json:"traffic_class"`
	Priority     int          `json:"priority"` // 优先级 1-10
	MinMbps      int64        `json:"min_mbps"` // 最小带宽保证
	MaxMbps      int64        `json:"max_mbps"` // 最大带宽限制
	Description  string       `json:"description"`
	Enabled      bool         `json:"enabled"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// BandwidthRule 带宽规则
type BandwidthRule struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	TrafficClass TrafficClass `json:"traffic_class"`
	SourceIP     string       `json:"source_ip,omitempty"`
	DestIP       string       `json:"dest_ip,omitempty"`
	SourcePort   int          `json:"source_port,omitempty"`
	DestPort     int          `json:"dest_port,omitempty"`
	Protocol     string       `json:"protocol,omitempty"`
	Priority     int          `json:"priority"` // 优先级 1-10
	MinMbps      int64        `json:"min_mbps"` // 最小带宽保证
	MaxMbps      int64        `json:"max_mbps"` // 最大带宽限制
	Enabled      bool         `json:"enabled"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// BandwidthStats 带宽统计
type BandwidthStats struct {
	RuleID       string       `json:"rule_id"`
	TrafficClass TrafficClass `json:"traffic_class"`
	CurrentMbps  float64      `json:"current_mbps"` // 当前带宽
	TotalBytes   int64        `json:"total_bytes"`  // 总流量
	Packets      int64        `json:"packets"`      // 包数
	Priority     int          `json:"priority"`
	Utilization  float64      `json:"utilization"` // 带宽利用率
	LastUpdated  time.Time    `json:"last_updated"`
}

// SmartBandwidthManager 智能带宽管理器
type SmartBandwidthManager struct {
	mu           sync.RWMutex
	rules        map[string]*BandwidthRule
	policies     map[string]*QoSPolicy
	profiles     map[string]*TrafficProfile
	stats        map[string]*BandwidthStats
	config       *SmartBandwidthConfig
	trafficRules map[TrafficClass][]*BandwidthRule // 按流量类型索引的规则
}

// SmartBandwidthConfig 智能带宽配置
type SmartBandwidthConfig struct {
	TotalBandwidthMbps int64  `json:"total_bandwidth_mbps"` // 总带宽
	Enabled            bool   `json:"enabled"`
	Interface          string `json:"interface"`           // 网络接口
	AdjustInterval     int    `json:"adjust_interval_sec"` // 动态调整间隔（秒）
}

// NewSmartBandwidthManager 创建智能带宽管理器
func NewSmartBandwidthManager(config *SmartBandwidthConfig) *SmartBandwidthManager {
	if config == nil {
		config = &SmartBandwidthConfig{
			TotalBandwidthMbps: 1000,
			Enabled:            true,
			AdjustInterval:     30,
		}
	}
	return &SmartBandwidthManager{
		rules:        make(map[string]*BandwidthRule),
		policies:     make(map[string]*QoSPolicy),
		profiles:     make(map[string]*TrafficProfile),
		stats:        make(map[string]*BandwidthStats),
		config:       config,
		trafficRules: make(map[TrafficClass][]*BandwidthRule),
	}
}

// SetBandwidthLimit 设置带宽限制
func (m *SmartBandwidthManager) SetBandwidthLimit(rule *BandwidthRule) (*BandwidthRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}

	if rule.Priority < 1 || rule.Priority > 10 {
		return nil, fmt.Errorf("优先级必须在1-10之间")
	}

	if rule.MaxMbps <= 0 {
		return nil, fmt.Errorf("最大带宽必须大于0")
	}

	if rule.MinMbps < 0 {
		return nil, fmt.Errorf("最小带宽不能为负数")
	}

	if rule.MinMbps > rule.MaxMbps {
		return nil, fmt.Errorf("最小带宽不能大于最大带宽")
	}

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("bw_%d", time.Now().UnixNano())
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.Enabled = true

	m.rules[rule.ID] = rule

	// 添加到流量类型索引
	if rule.TrafficClass != "" {
		m.trafficRules[rule.TrafficClass] = append(m.trafficRules[rule.TrafficClass], rule)
	}

	// 初始化统计
	m.stats[rule.ID] = &BandwidthStats{
		RuleID:       rule.ID,
		TrafficClass: rule.TrafficClass,
		Priority:     rule.Priority,
		LastUpdated:  time.Now(),
	}

	return rule, nil
}

// ClassifyTraffic 分类流量
func (m *SmartBandwidthManager) ClassifyTraffic(srcIP, dstIP string, srcPort, dstPort int, protocol string) TrafficClass {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 根据端口和协议智能分类
	switch {
	case dstPort == 80 || dstPort == 443:
		return TrafficClassWeb
	case dstPort == 5060 || dstPort == 5061 || protocol == "udp" && (dstPort >= 10000 && dstPort <= 20000):
		return TrafficClassVoIP
	case dstPort == 22 || dstPort == 2222:
		return TrafficClassFileTransfer
	case dstPort == 8080 || dstPort == 8443 || dstPort == 5000 || dstPort == 5001:
		return TrafficClassAIInference
	case dstPort == 3389 || dstPort == 5900 || dstPort == 5901:
		return TrafficClassGaming
	case dstPort == 8096 || dstPort == 32400 || dstPort == 9096:
		return TrafficClassStreaming
	case dstPort == 21 || dstPort == 990 || dstPort == 6881:
		return TrafficClassDownload
	case dstPort == 873 || dstPort == 55413 || dstPort == 55414:
		return TrafficClassBackup
	}

	// 检查是否有匹配的规则
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if rule.SourceIP != "" && rule.SourceIP != srcIP {
			continue
		}
		if rule.DestIP != "" && rule.DestIP != dstIP {
			continue
		}
		if rule.SourcePort != 0 && rule.SourcePort != srcPort {
			continue
		}
		if rule.DestPort != 0 && rule.DestPort != dstPort {
			continue
		}
		if rule.Protocol != "" && rule.Protocol != protocol {
			continue
		}
		if rule.TrafficClass != "" {
			return rule.TrafficClass
		}
	}

	return TrafficClassOther
}

// GetBandwidthStats 获取带宽统计
func (m *SmartBandwidthManager) GetBandwidthStats(ruleID string) (*BandwidthStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, exists := m.stats[ruleID]
	if !exists {
		return nil, fmt.Errorf("统计不存在: %s", ruleID)
	}

	return stats, nil
}

// GetAllBandwidthStats 获取所有带宽统计
func (m *SmartBandwidthManager) GetAllBandwidthStats() map[string]*BandwidthStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*BandwidthStats)
	for k, v := range m.stats {
		result[k] = v
	}
	return result
}

// GetBandwidthStatsByClass 按流量类型获取带宽统计
func (m *SmartBandwidthManager) GetBandwidthStatsByClass(class TrafficClass) []*BandwidthStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*BandwidthStats
	for _, stats := range m.stats {
		if stats.TrafficClass == class {
			result = append(result, stats)
		}
	}
	return result
}

// ApplyQoSPolicy 应用QoS策略
func (m *SmartBandwidthManager) ApplyQoSPolicy(policy *QoSPolicy) (*QoSPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.Name == "" {
		return nil, fmt.Errorf("策略名称不能为空")
	}

	if policy.Priority < 1 || policy.Priority > 10 {
		return nil, fmt.Errorf("优先级必须在1-10之间")
	}

	if policy.MaxMbps <= 0 {
		return nil, fmt.Errorf("最大带宽必须大于0")
	}

	if policy.MinMbps < 0 {
		return nil, fmt.Errorf("最小带宽不能为负数")
	}

	if policy.MinMbps > policy.MaxMbps {
		return nil, fmt.Errorf("最小带宽不能大于最大带宽")
	}

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("qos_%d", time.Now().UnixNano())
	}

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	policy.Enabled = true

	m.policies[policy.ID] = policy

	return policy, nil
}

// AdjustDynamic 动态调整带宽分配
func (m *SmartBandwidthManager) AdjustDynamic() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return fmt.Errorf("智能带宽管理未启用")
	}

	totalUsed := int64(0)
	priorityClasses := make(map[int][]*BandwidthRule)

	// 计算已用带宽并按优先级分组
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		stats, exists := m.stats[rule.ID]
		if exists {
			totalUsed += int64(stats.CurrentMbps)
		}
		priorityClasses[rule.Priority] = append(priorityClasses[rule.Priority], rule)
	}

	// 剩余带宽
	remaining := m.config.TotalBandwidthMbps - totalUsed
	if remaining <= 0 {
		return nil
	}

	// 按优先级从高到低分配剩余带宽
	for priority := 10; priority >= 1; priority-- {
		rules, exists := priorityClasses[priority]
		if !exists || len(rules) == 0 {
			continue
		}

		// 计算该优先级组的总最小带宽需求
		totalMinRequired := int64(0)
		for _, rule := range rules {
			stats, exists := m.stats[rule.ID]
			if exists && stats.CurrentMbps < float64(rule.MinMbps) {
				totalMinRequired += rule.MinMbps - int64(stats.CurrentMbps)
			}
		}

		// 如果剩余带宽足够，优先满足最小带宽保证
		if remaining >= totalMinRequired {
			for _, rule := range rules {
				stats, exists := m.stats[rule.ID]
				if exists && stats.CurrentMbps < float64(rule.MinMbps) {
					deficit := rule.MinMbps - int64(stats.CurrentMbps)
					stats.CurrentMbps = float64(rule.MinMbps)
					remaining -= deficit
				}
			}
		}

		// 分配剩余带宽（不超过最大限制）
		if remaining > 0 {
			perRule := remaining / int64(len(rules))
			for _, rule := range rules {
				stats, exists := m.stats[rule.ID]
				if exists {
					newMbps := stats.CurrentMbps + float64(perRule)
					if newMbps > float64(rule.MaxMbps) {
						newMbps = float64(rule.MaxMbps)
					}
					stats.CurrentMbps = newMbps
					stats.LastUpdated = time.Now()
				}
			}
			remaining -= perRule * int64(len(rules))
		}
	}

	return nil
}

// GetTrafficProfiles 获取流量配置文件列表
func (m *SmartBandwidthManager) GetTrafficProfiles() []*TrafficProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]*TrafficProfile, 0, len(m.profiles))
	for _, profile := range m.profiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// GetTrafficProfile 获取流量配置文件
func (m *SmartBandwidthManager) GetTrafficProfile(id string) (*TrafficProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, exists := m.profiles[id]
	if !exists {
		return nil, fmt.Errorf("流量配置文件不存在: %s", id)
	}

	return profile, nil
}

// CreateTrafficProfile 创建流量配置文件
func (m *SmartBandwidthManager) CreateTrafficProfile(profile *TrafficProfile) (*TrafficProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if profile.Name == "" {
		return nil, fmt.Errorf("配置文件名称不能为空")
	}

	if profile.Priority < 1 || profile.Priority > 10 {
		return nil, fmt.Errorf("优先级必须在1-10之间")
	}

	if profile.MaxMbps <= 0 {
		return nil, fmt.Errorf("最大带宽必须大于0")
	}

	if profile.MinMbps < 0 {
		return nil, fmt.Errorf("最小带宽不能为负数")
	}

	if profile.MinMbps > profile.MaxMbps {
		return nil, fmt.Errorf("最小带宽不能大于最大带宽")
	}

	if profile.ID == "" {
		profile.ID = fmt.Sprintf("profile_%d", time.Now().UnixNano())
	}

	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()
	profile.Enabled = true

	m.profiles[profile.ID] = profile

	return profile, nil
}

// DeleteBandwidthRule 删除带宽规则
func (m *SmartBandwidthManager) DeleteBandwidthRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	// 从流量类型索引中删除
	if rule.TrafficClass != "" {
		rules := m.trafficRules[rule.TrafficClass]
		for i, r := range rules {
			if r.ID == id {
				m.trafficRules[rule.TrafficClass] = append(rules[:i], rules[i+1:]...)
				break
			}
		}
	}

	delete(m.rules, id)
	delete(m.stats, id)
	return nil
}

// GetBandwidthRule 获取带宽规则
func (m *SmartBandwidthManager) GetBandwidthRule(id string) (*BandwidthRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	return rule, nil
}

// ListBandwidthRules 列出所有带宽规则
func (m *SmartBandwidthManager) ListBandwidthRules() []*BandwidthRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*BandwidthRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

// UpdateBandwidthRule 更新带宽规则
func (m *SmartBandwidthManager) UpdateBandwidthRule(id string, update *BandwidthRule) (*BandwidthRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	if update.Name != "" {
		rule.Name = update.Name
	}
	if update.Priority > 0 && update.Priority <= 10 {
		rule.Priority = update.Priority
	}
	if update.MaxMbps > 0 {
		rule.MaxMbps = update.MaxMbps
	}
	if update.MinMbps >= 0 {
		rule.MinMbps = update.MinMbps
	}
	if update.TrafficClass != "" {
		// 从旧索引删除
		if rule.TrafficClass != "" {
			rules := m.trafficRules[rule.TrafficClass]
			for i, r := range rules {
				if r.ID == id {
					m.trafficRules[rule.TrafficClass] = append(rules[:i], rules[i+1:]...)
					break
				}
			}
		}
		rule.TrafficClass = update.TrafficClass
		// 添加到新索引
		m.trafficRules[rule.TrafficClass] = append(m.trafficRules[rule.TrafficClass], rule)
	}
	if update.SourceIP != "" {
		rule.SourceIP = update.SourceIP
	}
	if update.DestIP != "" {
		rule.DestIP = update.DestIP
	}
	if update.SourcePort != 0 {
		rule.SourcePort = update.SourcePort
	}
	if update.DestPort != 0 {
		rule.DestPort = update.DestPort
	}
	if update.Protocol != "" {
		rule.Protocol = update.Protocol
	}
	rule.UpdatedAt = time.Now()

	return rule, nil
}

// EnableBandwidthRule 启用带宽规则
func (m *SmartBandwidthManager) EnableBandwidthRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = true
	rule.UpdatedAt = time.Now()
	return nil
}

// DisableBandwidthRule 禁用带宽规则
func (m *SmartBandwidthManager) DisableBandwidthRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return fmt.Errorf("规则不存在: %s", id)
	}

	rule.Enabled = false
	rule.UpdatedAt = time.Now()
	return nil
}

// GetQoSPolicy 获取QoS策略
func (m *SmartBandwidthManager) GetQoSPolicy(id string) (*QoSPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}

	return policy, nil
}

// ListQoSPolicies 列出所有QoS策略
func (m *SmartBandwidthManager) ListQoSPolicies() []*QoSPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*QoSPolicy, 0, len(m.policies))
	for _, policy := range m.policies {
		policies = append(policies, policy)
	}
	return policies
}

// DeleteQoSPolicy 删除QoS策略
func (m *SmartBandwidthManager) DeleteQoSPolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在: %s", id)
	}

	delete(m.policies, id)
	return nil
}

// UpdateQoSPolicy 更新QoS策略
func (m *SmartBandwidthManager) UpdateQoSPolicy(id string, update *QoSPolicy) (*QoSPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}

	if update.Name != "" {
		policy.Name = update.Name
	}
	if update.Priority > 0 && update.Priority <= 10 {
		policy.Priority = update.Priority
	}
	if update.MaxMbps > 0 {
		policy.MaxMbps = update.MaxMbps
	}
	if update.MinMbps >= 0 {
		policy.MinMbps = update.MinMbps
	}
	policy.UpdatedAt = time.Now()

	return policy, nil
}
