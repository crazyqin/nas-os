package storageqos

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// QoSLevel 优先级级别
type QoSLevel string

const (
	// QoSLevelPlatinum 白金级别 - 最高优先级
	QoSLevelPlatinum QoSLevel = "platinum"
	// QoSLevelGold 黄金级别 - 高优先级
	QoSLevelGold QoSLevel = "gold"
	// QoSLevelSilver 白银级别 - 中等优先级
	QoSLevelSilver QoSLevel = "silver"
	// QoSLevelBronze 青铜级别 - 最低优先级
	QoSLevelBronze QoSLevel = "bronze"
)

// QoSPolicy QoS策略
type QoSPolicy struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Level        QoSLevel  `json:"level"`
	TargetType   string    `json:"target_type"` // volume, share, container
	TargetID     string    `json:"target_id"`
	MinIOPS      int64     `json:"min_iops"`      // IOPS下限
	MaxIOPS      int64     `json:"max_iops"`      // IOPS上限
	MinBandwidth int64     `json:"min_bandwidth"` // 带宽下限 (MB/s)
	MaxBandwidth int64     `json:"max_bandwidth"` // 带宽上限 (MB/s)
	LatencyMax   int64     `json:"latency_max"`   // 最大延迟阈值 (ms)
	Enabled      bool      `json:"enabled"`
	Adaptive     bool      `json:"adaptive"` // 自适应QoS
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// QoSTarget QoS目标对象
type QoSTarget struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // volume, share, container
	Name       string `json:"name"`
	Path       string `json:"path"`
	DevicePath string `json:"device_path"` // 设备路径，如 /dev/sda1
	CGroupPath string `json:"cgroup_path"` // cgroup路径
}

// QoSMetrics 实时指标
type QoSMetrics struct {
	TargetID    string    `json:"target_id"`
	IOPS        int64     `json:"iops"`        // 当前IOPS
	ReadIOPS    int64     `json:"read_iops"`   // 读IOPS
	WriteIOPS   int64     `json:"write_iops"`  // 写IOPS
	Bandwidth   int64     `json:"bandwidth"`   // 当前带宽 (MB/s)
	ReadBW      int64     `json:"read_bw"`     // 读带宽
	WriteBW     int64     `json:"write_bw"`    // 写带宽
	Latency     int64     `json:"latency"`     // 当前延迟 (ms)
	QueueDepth  int64     `json:"queue_depth"` // 队列深度
	Utilization float64   `json:"utilization"` // 设备利用率 (%)
	Timestamp   time.Time `json:"timestamp"`
}

// QoSViolation 违规记录
type QoSViolation struct {
	ID         string     `json:"id"`
	PolicyID   string     `json:"policy_id"`
	PolicyName string     `json:"policy_name"`
	TargetID   string     `json:"target_id"`
	Type       string     `json:"type"` // iops_exceeded, bandwidth_exceeded, latency_exceeded
	Threshold  int64      `json:"threshold"`
	Actual     int64      `json:"actual"`
	Message    string     `json:"message"`
	Severity   string     `json:"severity"` // warning, critical
	Resolved   bool       `json:"resolved"`
	Timestamp  time.Time  `json:"timestamp"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// QoSManager 存储QoS管理器
// 负责QoS策略的生命周期管理，包括创建、更新、删除、查询等操作
// 支持策略模板、批量操作、策略冲突检测等高级功能
type QoSManager struct {
	mu        sync.RWMutex
	policies  map[string]*QoSPolicy
	templates map[string]*QoSPolicyTemplate
	config    *QoSConfig
	stopCh    chan struct{}
}

// QoSConfig QoS配置
type QoSConfig struct {
	Enabled          bool `json:"enabled"`
	MetricsInterval  int  `json:"metrics_interval"`  // 指标采集间隔（秒）
	ViolationHistory int  `json:"violation_history"` // 违规历史保留数量
	AdaptiveEnabled  bool `json:"adaptive_enabled"`  // 启用自适应QoS
	AlertEnabled     bool `json:"alert_enabled"`     // 启用告警
	MaxPolicies      int  `json:"max_policies"`      // 最大策略数量
}

// QoSPolicyTemplate 策略模板
// 用于快速创建标准化的QoS策略
// 模板包含预定义的配置，用户可以基于模板快速创建策略
type QoSPolicyTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Level       QoSLevel `json:"level"`
	MinIOPS     int64    `json:"min_iops"`
	MaxIOPS     int64    `json:"max_iops"`
	MinBW       int64    `json:"min_bandwidth"`
	MaxBW       int64    `json:"max_bandwidth"`
	LatencyMax  int64    `json:"latency_max"`
	Adaptive    bool     `json:"adaptive"`
	Tags        []string `json:"tags,omitempty"`
}

// NewQoSManager 创建QoS管理器
// config: QoS配置，如果为nil则使用默认配置
func NewQoSManager(config *QoSConfig) *QoSManager {
	if config == nil {
		config = &QoSConfig{
			Enabled:          true,
			MetricsInterval:  10,
			ViolationHistory: 1000,
			AdaptiveEnabled:  true,
			AlertEnabled:     true,
			MaxPolicies:      1000,
		}
	}
	m := &QoSManager{
		policies:  make(map[string]*QoSPolicy),
		templates: make(map[string]*QoSPolicyTemplate),
		config:    config,
		stopCh:    make(chan struct{}),
	}
	// 初始化默认模板
	m.initDefaultTemplates()
	return m
}

// initDefaultTemplates 初始化默认策略模板
func (m *QoSManager) initDefaultTemplates() {
	defaultTemplates := []*QoSPolicyTemplate{
		{
			ID:          "tpl_high_performance",
			Name:        "高性能存储",
			Description: "适用于数据库、虚拟机等高性能场景",
			Level:       QoSLevelPlatinum,
			MinIOPS:     5000,
			MaxIOPS:     50000,
			MinBW:       500,
			MaxBW:       5000,
			LatencyMax:  1,
			Adaptive:    true,
			Tags:        []string{"database", "vm", "ssd"},
		},
		{
			ID:          "tpl_standard",
			Name:        "标准存储",
			Description: "适用于文件共享、一般应用",
			Level:       QoSLevelGold,
			MinIOPS:     1000,
			MaxIOPS:     10000,
			MinBW:       100,
			MaxBW:       1000,
			LatencyMax:  5,
			Adaptive:    false,
			Tags:        []string{"fileshare", "general"},
		},
		{
			ID:          "tpl_archive",
			Name:        "归档存储",
			Description: "适用于备份、日志等低频访问场景",
			Level:       QoSLevelBronze,
			MinIOPS:     100,
			MaxIOPS:     1000,
			MinBW:       10,
			MaxBW:       100,
			LatencyMax:  50,
			Adaptive:    false,
			Tags:        []string{"backup", "archive", "cold"},
		},
	}

	for _, tpl := range defaultTemplates {
		m.templates[tpl.ID] = tpl
	}
}

// CreatePolicy 创建QoS策略
func (m *QoSManager) CreatePolicy(policy *QoSPolicy) (*QoSPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证策略名称
	if policy.Name == "" {
		return nil, fmt.Errorf("策略名称不能为空")
	}

	// 验证优先级级别
	if !isValidLevel(policy.Level) {
		return nil, fmt.Errorf("无效的优先级级别: %s", policy.Level)
	}

	// 验证IOPS限制
	if policy.MinIOPS < 0 || policy.MaxIOPS < 0 {
		return nil, fmt.Errorf("IOPS不能为负数")
	}
	if policy.MinIOPS > 0 && policy.MaxIOPS > 0 && policy.MinIOPS > policy.MaxIOPS {
		return nil, fmt.Errorf("IOPS下限不能大于上限")
	}

	// 验证带宽限制
	if policy.MinBandwidth < 0 || policy.MaxBandwidth < 0 {
		return nil, fmt.Errorf("带宽不能为负数")
	}
	if policy.MinBandwidth > 0 && policy.MaxBandwidth > 0 && policy.MinBandwidth > policy.MaxBandwidth {
		return nil, fmt.Errorf("带宽下限不能大于上限")
	}

	// 验证延迟阈值
	if policy.LatencyMax < 0 {
		return nil, fmt.Errorf("延迟阈值不能为负数")
	}

	// 验证目标类型
	if policy.TargetType != "" && policy.TargetType != "volume" && policy.TargetType != "share" && policy.TargetType != "container" {
		return nil, fmt.Errorf("无效的目标类型: %s", policy.TargetType)
	}

	// 生成ID
	policy.ID = fmt.Sprintf("qos_%d", time.Now().UnixNano())
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	policy.Enabled = true

	m.policies[policy.ID] = policy

	return policy, nil
}

// UpdatePolicy 更新QoS策略
func (m *QoSManager) UpdatePolicy(id string, update *QoSPolicy) (*QoSPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}

	// 更新字段
	if update.Name != "" {
		policy.Name = update.Name
	}
	if update.Description != "" {
		policy.Description = update.Description
	}
	if update.Level != "" && isValidLevel(update.Level) {
		policy.Level = update.Level
	}
	if update.MinIOPS >= 0 {
		policy.MinIOPS = update.MinIOPS
	}
	if update.MaxIOPS >= 0 {
		policy.MaxIOPS = update.MaxIOPS
	}
	if update.MinBandwidth >= 0 {
		policy.MinBandwidth = update.MinBandwidth
	}
	if update.MaxBandwidth >= 0 {
		policy.MaxBandwidth = update.MaxBandwidth
	}
	if update.LatencyMax >= 0 {
		policy.LatencyMax = update.LatencyMax
	}
	if update.Adaptive {
		policy.Adaptive = update.Adaptive
	}

	policy.UpdatedAt = time.Now()

	// 验证更新后的配置
	if policy.MinIOPS > 0 && policy.MaxIOPS > 0 && policy.MinIOPS > policy.MaxIOPS {
		return nil, fmt.Errorf("IOPS下限不能大于上限")
	}
	if policy.MinBandwidth > 0 && policy.MaxBandwidth > 0 && policy.MinBandwidth > policy.MaxBandwidth {
		return nil, fmt.Errorf("带宽下限不能大于上限")
	}

	return policy, nil
}

// DeletePolicy 删除QoS策略
func (m *QoSManager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.policies[id]; !exists {
		return fmt.Errorf("策略不存在: %s", id)
	}

	delete(m.policies, id)
	return nil
}

// GetPolicy 获取QoS策略
func (m *QoSManager) GetPolicy(id string) (*QoSPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.policies[id]
	if !exists {
		return nil, fmt.Errorf("策略不存在: %s", id)
	}

	return policy, nil
}

// ListPolicies 列出所有QoS策略
func (m *QoSManager) ListPolicies() []*QoSPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*QoSPolicy, 0, len(m.policies))
	for _, policy := range m.policies {
		policies = append(policies, policy)
	}
	return policies
}

// EnablePolicy 启用策略
func (m *QoSManager) EnablePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return fmt.Errorf("策略不存在: %s", id)
	}

	policy.Enabled = true
	policy.UpdatedAt = time.Now()
	return nil
}

// DisablePolicy 禁用策略
func (m *QoSManager) DisablePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.policies[id]
	if !exists {
		return fmt.Errorf("策略不存在: %s", id)
	}

	policy.Enabled = false
	policy.UpdatedAt = time.Now()
	return nil
}

// isValidLevel 验证优先级级别
func isValidLevel(level QoSLevel) bool {
	switch level {
	case QoSLevelPlatinum, QoSLevelGold, QoSLevelSilver, QoSLevelBronze:
		return true
	default:
		return false
	}
}

// GetLevelWeight 获取优先级权重
func GetLevelWeight(level QoSLevel) int {
	switch level {
	case QoSLevelPlatinum:
		return 100
	case QoSLevelGold:
		return 75
	case QoSLevelSilver:
		return 50
	case QoSLevelBronze:
		return 25
	default:
		return 0
	}
}

// GetConfig 获取配置
func (m *QoSManager) GetConfig() *QoSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *QoSManager) UpdateConfig(config *QoSConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	m.config = config
	return nil
}

// GetPoliciesByLevel 按优先级级别获取策略
func (m *QoSManager) GetPoliciesByLevel(level QoSLevel) []*QoSPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var policies []*QoSPolicy
	for _, policy := range m.policies {
		if policy.Level == level {
			policies = append(policies, policy)
		}
	}
	return policies
}

// GetEnabledPolicies 获取启用的策略
func (m *QoSManager) GetEnabledPolicies() []*QoSPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var policies []*QoSPolicy
	for _, policy := range m.policies {
		if policy.Enabled {
			policies = append(policies, policy)
		}
	}
	return policies
}

// GetPoliciesByTarget 按目标获取策略
func (m *QoSManager) GetPoliciesByTarget(targetType, targetID string) []*QoSPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var policies []*QoSPolicy
	for _, policy := range m.policies {
		if policy.TargetType == targetType && policy.TargetID == targetID {
			policies = append(policies, policy)
		}
	}
	return policies
}

// CreatePolicyFromTemplate 基于模板创建策略
// templateID: 模板ID
// targetType: 目标类型 (volume, share, container)
// targetID: 目标ID
// customName: 自定义名称，如果为空则使用模板名称
func (m *QoSManager) CreatePolicyFromTemplate(templateID, targetType, targetID, customName string) (*QoSPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	template, exists := m.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	// 验证目标类型
	if targetType != "volume" && targetType != "share" && targetType != "container" {
		return nil, fmt.Errorf("无效的目标类型: %s", targetType)
	}

	// 检查是否已存在相同目标的策略
	for _, p := range m.policies {
		if p.TargetType == targetType && p.TargetID == targetID {
			return nil, fmt.Errorf("目标 %s/%s 已存在QoS策略: %s", targetType, targetID, p.ID)
		}
	}

	name := customName
	if name == "" {
		name = template.Name
	}

	policy := &QoSPolicy{
		ID:           fmt.Sprintf("qos_%d", time.Now().UnixNano()),
		Name:         name,
		Description:  fmt.Sprintf("基于模板 '%s' 创建", template.Name),
		Level:        template.Level,
		TargetType:   targetType,
		TargetID:     targetID,
		MinIOPS:      template.MinIOPS,
		MaxIOPS:      template.MaxIOPS,
		MinBandwidth: template.MinBW,
		MaxBandwidth: template.MaxBW,
		LatencyMax:   template.LatencyMax,
		Adaptive:     template.Adaptive,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.policies[policy.ID] = policy
	return policy, nil
}

// ListTemplates 列出所有模板
func (m *QoSManager) ListTemplates() []*QoSPolicyTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*QoSPolicyTemplate, 0, len(m.templates))
	for _, tpl := range m.templates {
		templates = append(templates, tpl)
	}
	return templates
}

// GetTemplate 获取模板
func (m *QoSManager) GetTemplate(id string) (*QoSPolicyTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, exists := m.templates[id]
	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", id)
	}
	return tpl, nil
}

// BatchCreatePolicies 批量创建策略
// policies: 策略列表
// 返回成功创建的策略列表和错误列表
func (m *QoSManager) BatchCreatePolicies(policies []*QoSPolicy) ([]*QoSPolicy, []error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var created []*QoSPolicy
	var errors []error

	for _, policy := range policies {
		// 验证策略
		if policy.Name == "" {
			errors = append(errors, fmt.Errorf("策略名称不能为空"))
			continue
		}
		if !isValidLevel(policy.Level) {
			errors = append(errors, fmt.Errorf("无效的优先级级别: %s", policy.Level))
			continue
		}

		// 检查策略数量限制
		if m.config.MaxPolicies > 0 && len(m.policies) >= m.config.MaxPolicies {
			errors = append(errors, fmt.Errorf("已达到最大策略数量限制: %d", m.config.MaxPolicies))
			break
		}

		// 生成ID并设置时间
		policy.ID = fmt.Sprintf("qos_%d", time.Now().UnixNano())
		policy.CreatedAt = time.Now()
		policy.UpdatedAt = time.Now()
		policy.Enabled = true

		m.policies[policy.ID] = policy
		created = append(created, policy)
	}

	return created, errors
}

// BatchDeletePolicies 批量删除策略
// ids: 策略ID列表
// 返回成功删除的数量和错误列表
func (m *QoSManager) BatchDeletePolicies(ids []string) (int, []error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var deleted int
	var errors []error

	for _, id := range ids {
		if _, exists := m.policies[id]; !exists {
			errors = append(errors, fmt.Errorf("策略不存在: %s", id))
			continue
		}
		delete(m.policies, id)
		deleted++
	}

	return deleted, errors
}

// SearchPolicies 搜索策略
// keyword: 搜索关键词（匹配名称和描述）
func (m *QoSManager) SearchPolicies(keyword string) []*QoSPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*QoSPolicy
	keyword = strings.ToLower(keyword)

	for _, policy := range m.policies {
		if strings.Contains(strings.ToLower(policy.Name), keyword) ||
			strings.Contains(strings.ToLower(policy.Description), keyword) {
			result = append(result, policy)
		}
	}
	return result
}

// GetPolicyStats 获取策略统计信息
func (m *QoSManager) GetPolicyStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total":    len(m.policies),
		"enabled":  0,
		"disabled": 0,
		"by_level": map[QoSLevel]int{},
		"by_type":  map[string]int{},
		"adaptive": 0,
	}

	for _, policy := range m.policies {
		if policy.Enabled {
			stats["enabled"] = stats["enabled"].(int) + 1
		} else {
			stats["disabled"] = stats["disabled"].(int) + 1
		}
		if policy.Adaptive {
			stats["adaptive"] = stats["adaptive"].(int) + 1
		}

		// 按级别统计
		byLevel := stats["by_level"].(map[QoSLevel]int)
		byLevel[policy.Level]++

		// 按类型统计
		if policy.TargetType != "" {
			byType := stats["by_type"].(map[string]int)
			byType[policy.TargetType]++
		}
	}

	return stats
}

// DetectPolicyConflicts 检测策略冲突
// 检测同一目标是否有多个策略
func (m *QoSManager) DetectPolicyConflicts() [][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 按目标分组
	targetPolicies := make(map[string][]string)
	for _, policy := range m.policies {
		if policy.TargetType != "" && policy.TargetID != "" {
			key := fmt.Sprintf("%s/%s", policy.TargetType, policy.TargetID)
			targetPolicies[key] = append(targetPolicies[key], policy.ID)
		}
	}

	// 找出有冲突的策略
	var conflicts [][]string
	for _, policyIDs := range targetPolicies {
		if len(policyIDs) > 1 {
			conflicts = append(conflicts, policyIDs)
		}
	}

	return conflicts
}

// ExportPolicies 导出策略配置
func (m *QoSManager) ExportPolicies() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export := map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now(),
		"policies":    m.ListPolicies(),
		"config":      m.config,
	}

	return json.Marshal(export)
}

// ImportPolicies 导入策略配置
func (m *QoSManager) ImportPolicies(data []byte, overwrite bool) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var importData struct {
		Policies []*QoSPolicy `json:"policies"`
	}

	if err := json.Unmarshal(data, &importData); err != nil {
		return 0, fmt.Errorf("解析导入数据失败: %w", err)
	}

	imported := 0
	for _, policy := range importData.Policies {
		if policy.ID == "" {
			policy.ID = fmt.Sprintf("qos_%d", time.Now().UnixNano())
		}

		// 检查是否已存在
		if _, exists := m.policies[policy.ID]; exists && !overwrite {
			continue
		}

		policy.UpdatedAt = time.Now()
		m.policies[policy.ID] = policy
		imported++
	}

	return imported, nil
}
