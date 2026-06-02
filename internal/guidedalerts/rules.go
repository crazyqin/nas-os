package guidedalerts

import (
	"fmt"
	"sync"
	"time"
)

// AlertRule 告警规则定义
type AlertRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Severity    AlertSeverity     `json:"severity"`
	Category    AlertCategory     `json:"category"`
	Labels      map[string]string `json:"labels,omitempty"`

	// 条件配置
	Condition   RuleCondition     `json:"condition"`

	// 引导修复
	Guidance    *Guidance         `json:"guidance,omitempty"`
	MenuHint    *MenuHint         `json:"menuHint,omitempty"`
	AutoFix     *AutoFix          `json:"autoFix,omitempty"`

	// 升级策略
	Escalation  *EscalationConfig `json:"escalation,omitempty"`

	// 聚合与去重
	GroupBy     []string          `json:"groupBy,omitempty"`    // 按标签分组
	RepeatWait  time.Duration     `json:"repeatWait"`          // 重复告警等待时间

	// 静默与抑制
	SilenceMatchers []LabelMatcher `json:"silenceMatchers,omitempty"`

	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	Type       ConditionType     `json:"type"`       // threshold, range, status, custom
	Metric     string            `json:"metric"`     // 指标名
	Operator   string            `json:"operator"`   // gt, lt, gte, lte, eq, ne
	Threshold  float64           `json:"threshold"`  // 阈值
	Duration   time.Duration     `json:"duration"`   // 持续时间
	For        time.Duration     `json:"for"`        // 触发前持续满足条件的时间
}

// ConditionType 条件类型
type ConditionType string

const (
	ConditionThreshold ConditionType = "threshold"
	ConditionRange     ConditionType = "range"
	ConditionStatus    ConditionType = "status"
	ConditionCustom    ConditionType = "custom"
)

// EscalationConfig 升级配置
type EscalationConfig struct {
	Enabled     bool              `json:"enabled"`
	Timeout     time.Duration     `json:"timeout"`
	MaxLevel    int               `json:"maxLevel"`
	Targets     []EscalationTarget `json:"targets"`
}

// EscalationTarget 升级目标
type EscalationTarget struct {
	Level  int      `json:"level"`
	Notify []string `json:"notify"` // 通知目标
}

// RuleEngine 规则引擎
type RuleEngine struct {
	mu        sync.RWMutex
	rules     map[string]*AlertRule
	evalFuncs map[ConditionType]EvalFunc
}

// EvalFunc 条件评估函数
type EvalFunc func(condition *RuleCondition, metrics map[string]float64) bool

// NewRuleEngine 创建规则引擎
func NewRuleEngine() *RuleEngine {
	engine := &RuleEngine{
		rules:     make(map[string]*AlertRule),
		evalFuncs: make(map[ConditionType]EvalFunc),
	}

	// 注册内置评估函数
	engine.evalFuncs[ConditionThreshold] = evalThreshold
	engine.evalFuncs[ConditionRange] = evalRange
	engine.evalFuncs[ConditionStatus] = evalStatus

	return engine
}

// RegisterEvalFunc 注册自定义评估函数
func (re *RuleEngine) RegisterEvalFunc(condType ConditionType, fn EvalFunc) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.evalFuncs[condType] = fn
}

// AddRule 添加规则
func (re *RuleEngine) AddRule(rule *AlertRule) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if _, exists := re.rules[rule.ID]; exists {
		return fmt.Errorf("rule %s already exists", rule.ID)
	}

	now := time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now

	re.rules[rule.ID] = rule
	return nil
}

// UpdateRule 更新规则
func (re *RuleEngine) UpdateRule(rule *AlertRule) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, exists := re.rules[rule.ID]; !exists {
		return fmt.Errorf("rule %s not found", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	re.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除规则
func (re *RuleEngine) DeleteRule(id string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if _, exists := re.rules[id]; !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	delete(re.rules, id)
	return nil
}

// GetRule 获取规则
func (re *RuleEngine) GetRule(id string) (*AlertRule, bool) {
	re.mu.RLock()
	defer re.mu.RUnlock()
	rule, ok := re.rules[id]
	return rule, ok
}

// ListRules 列出所有规则
func (re *RuleEngine) ListRules(category AlertCategory, enabledOnly bool) []*AlertRule {
	re.mu.RLock()
	defer re.mu.RUnlock()

	var result []*AlertRule
	for _, rule := range re.rules {
		if category != "" && rule.Category != category {
			continue
		}
		if enabledOnly && !rule.Enabled {
			continue
		}
		result = append(result, rule)
	}
	return result
}

// Evaluate 评估规则
func (re *RuleEngine) Evaluate(ruleID string, metrics map[string]float64) (bool, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	rule, ok := re.rules[ruleID]
	if !ok {
		return false, fmt.Errorf("rule %s not found", ruleID)
	}
	if !rule.Enabled {
		return false, nil
	}

	evalFn, ok := re.evalFuncs[rule.Condition.Type]
	if !ok {
		return false, fmt.Errorf("unsupported condition type: %s", rule.Condition.Type)
	}

	return evalFn(&rule.Condition, metrics), nil
}

// EvaluateAll 评估所有启用的规则
func (re *RuleEngine) EvaluateAll(metrics map[string]float64) map[string]bool {
	re.mu.RLock()
	defer re.mu.RUnlock()

	results := make(map[string]bool)
	for id, rule := range re.rules {
		if !rule.Enabled {
			continue
		}
		evalFn, ok := re.evalFuncs[rule.Condition.Type]
		if !ok {
			continue
		}
		results[id] = evalFn(&rule.Condition, metrics)
	}
	return results
}

// 内置评估函数

func evalThreshold(condition *RuleCondition, metrics map[string]float64) bool {
	value, ok := metrics[condition.Metric]
	if !ok {
		return false
	}

	switch condition.Operator {
	case "gt":
		return value > condition.Threshold
	case "lt":
		return value < condition.Threshold
	case "gte":
		return value >= condition.Threshold
	case "lte":
		return value <= condition.Threshold
	case "eq":
		return value == condition.Threshold
	case "ne":
		return value != condition.Threshold
	default:
		return false
	}
}

func evalRange(condition *RuleCondition, metrics map[string]float64) bool {
	// 简化实现：检查值是否在阈值范围内（阈值作为下限，阈值+10%作为上限）
	value, ok := metrics[condition.Metric]
	if !ok {
		return false
	}
	upper := condition.Threshold * 1.1
	return value >= condition.Threshold && value <= upper
}

func evalStatus(condition *RuleCondition, metrics map[string]float64) bool {
	// 简化实现：检查状态值是否等于阈值（0=正常，非0=异常）
	value, ok := metrics[condition.Metric]
	if !ok {
		return false
	}
	if condition.Operator == "eq" {
		return value == condition.Threshold
	}
	return value != 0
}

// GetBuiltinRules 获取内置规则模板
func GetBuiltinRules() []*AlertRule {
	return []*AlertRule{
		{
			ID:          "high-cpu-usage",
			Name:        "CPU 使用率过高",
			Description: "CPU 使用率持续超过阈值",
			Enabled:     true,
			Severity:    SeverityWarning,
			Category:    CategoryCPU,
			Condition: RuleCondition{
				Type:      ConditionThreshold,
				Metric:    "cpu_usage",
				Operator:  "gt",
				Threshold: 80,
				Duration:  5 * time.Minute,
			},
			Guidance: &Guidance{
				Steps: []RepairStep{
					{Order: 1, Title: "检查高 CPU 进程", Description: "运行 top 或 htop 查看占用 CPU 的进程", Command: "top -bn1 | head -20"},
					{Order: 2, Title: "分析进程原因", Description: "检查是否有异常进程或服务"},
					{Order: 3, Title: "优化或重启服务", Description: "根据分析结果优化配置或重启问题服务"},
				},
				Difficulty:   "easy",
				EstimatedMin: 10,
			},
			Escalation: &EscalationConfig{
				Enabled:  true,
				Timeout:  30 * time.Minute,
				MaxLevel: 2,
				Targets: []EscalationTarget{
					{Level: 1, Notify: []string{"admin"}},
					{Level: 2, Notify: []string{"manager"}},
				},
			},
			RepeatWait: 10 * time.Minute,
		},
		{
			ID:          "high-memory-usage",
			Name:        "内存使用率过高",
			Description: "内存使用率持续超过阈值",
			Enabled:     true,
			Severity:    SeverityWarning,
			Category:    CategoryMemory,
			Condition: RuleCondition{
				Type:      ConditionThreshold,
				Metric:    "memory_usage",
				Operator:  "gt",
				Threshold: 85,
				Duration:  5 * time.Minute,
			},
			Guidance: &Guidance{
				Steps: []RepairStep{
					{Order: 1, Title: "检查内存使用", Description: "运行 free -h 查看内存使用情况", Command: "free -h"},
					{Order: 2, Title: "识别内存大户", Description: "运行 ps aux --sort=-%mem | head 查看占用内存最多的进程", Command: "ps aux --sort=-%mem | head -10"},
					{Order: 3, Title: "清理缓存", Description: "如果缓存占用过高，可手动清理", Command: "sync && echo 3 > /proc/sys/vm/drop_caches", NeedsRoot: true},
				},
				Difficulty:   "easy",
				EstimatedMin: 5,
			},
			RepeatWait: 15 * time.Minute,
		},
		{
			ID:          "disk-space-low",
			Name:        "磁盘空间不足",
			Description: "磁盘使用率超过阈值",
			Enabled:     true,
			Severity:    SeverityCritical,
			Category:    CategoryDisk,
			Condition: RuleCondition{
				Type:      ConditionThreshold,
				Metric:    "disk_usage",
				Operator:  "gt",
				Threshold: 90,
			},
			Guidance: &Guidance{
				Steps: []RepairStep{
					{Order: 1, Title: "检查磁盘使用", Description: "运行 df -h 查看各分区使用情况", Command: "df -h"},
					{Order: 2, Title: "查找大文件", Description: "运行 du -sh /* 查找占用空间大的目录", Command: "du -sh /* | sort -rh | head -10"},
					{Order: 3, Title: "清理日志", Description: "清理过期日志文件释放空间", Command: "find /var/log -name '*.gz' -mtime +30 -delete", NeedsRoot: true},
					{Order: 4, Title: "扩展存储", Description: "考虑添加新硬盘或扩展现有存储"},
				},
				Difficulty:   "medium",
				EstimatedMin: 20,
			},
			AutoFix: &AutoFix{
				Available: true,
				Commands: []string{
					"find /tmp -type f -mtime +7 -delete",
					"journalctl --vacuum-time=7d",
				},
				NeedsRoot: true,
				RiskLevel: "low",
			},
			Escalation: &EscalationConfig{
				Enabled:  true,
				Timeout:  15 * time.Minute,
				MaxLevel: 3,
				Targets: []EscalationTarget{
					{Level: 1, Notify: []string{"admin"}},
					{Level: 2, Notify: []string{"manager"}},
					{Level: 3, Notify: []string{"director"}},
				},
			},
			RepeatWait: 5 * time.Minute,
		},
		{
			ID:          "network-high-latency",
			Name:        "网络延迟过高",
			Description: "网络延迟超过阈值",
			Enabled:     true,
			Severity:    SeverityWarning,
			Category:    CategoryNetwork,
			Condition: RuleCondition{
				Type:      ConditionThreshold,
				Metric:    "network_latency",
				Operator:  "gt",
				Threshold: 100, // 毫秒
				Duration:  3 * time.Minute,
			},
			Guidance: &Guidance{
				Steps: []RepairStep{
					{Order: 1, Title: "检查网络连接", Description: "运行 ping 检测网络延迟", Command: "ping -c 5 8.8.8.8"},
					{Order: 2, Title: "检查 DNS", Description: "确认 DNS 解析正常", Command: "nslookup example.com"},
					{Order: 3, Title: "检查网络接口", Description: "查看网络接口状态和错误统计", Command: "ip -s link"},
				},
				Difficulty:   "easy",
				EstimatedMin: 10,
			},
			RepeatWait: 10 * time.Minute,
		},
		{
			ID:          "service-down",
			Name:        "服务停止运行",
			Description: "关键服务停止响应",
			Enabled:     true,
			Severity:    SeverityCritical,
			Category:    CategoryService,
			Condition: RuleCondition{
				Type:      ConditionStatus,
				Metric:    "service_status",
				Operator:  "ne",
				Threshold: 1, // 1=运行中，0=停止
			},
			Guidance: &Guidance{
				Steps: []RepairStep{
					{Order: 1, Title: "检查服务状态", Description: "查看服务详细状态", Command: "systemctl status <service>"},
					{Order: 2, Title: "查看服务日志", Description: "检查服务日志查找错误原因", Command: "journalctl -u <service> -n 50"},
					{Order: 3, Title: "重启服务", Description: "尝试重启服务", Command: "systemctl restart <service>", NeedsRoot: true},
					{Order: 4, Title: "检查依赖", Description: "确认服务依赖的端口和资源正常"},
				},
				Difficulty:   "medium",
				EstimatedMin: 15,
			},
			AutoFix: &AutoFix{
				Available: true,
				Commands:   []string{"systemctl restart <service>"},
				NeedsRoot:  true,
				RiskLevel:  "medium",
			},
			Escalation: &EscalationConfig{
				Enabled:  true,
				Timeout:  5 * time.Minute,
				MaxLevel: 2,
				Targets: []EscalationTarget{
					{Level: 1, Notify: []string{"admin"}},
					{Level: 2, Notify: []string{"oncall"}},
				},
			},
			RepeatWait: 2 * time.Minute,
		},
	}
}
