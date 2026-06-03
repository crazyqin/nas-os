// Package ransomshield - AI 驱动的高级检测器
// 熵分析、行为模式识别、文件系统监控
package ransomshield

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 高级检测器
// ============================================================

// Detector AI 驱动的高级勒索软件检测器
type Detector struct {
	mu sync.RWMutex

	// policies 防护策略列表
	policies map[string]*ShieldPolicy

	// rules 威胁规则列表
	rules map[string]*ThreatRule

	// patterns 已知攻击模式
	patterns map[string]*AttackPattern

	// threatEvents 威胁事件历史
	threatEvents []ThreatEvent

	// stats 统计信息
	stats ShieldStats

	// running 运行状态
	running bool

	// startTime 启动时间
	startTime time.Time

	// stopChan 停止信号
	stopChan chan struct{}

	// eventChan 文件事件通道
	eventChan chan FileEvent

	// threatChan 威胁事件通道
	threatChan chan ThreatEvent
}

// FileEvent 文件事件（内部使用）
type FileEvent struct {
	Path        string    `json:"path"`
	OldPath     string    `json:"old_path,omitempty"`
	Size        int64     `json:"size"`
	Extension   string    `json:"extension"`
	EventType   string    `json:"event_type"` // create, modify, delete, rename
	ProcessName string    `json:"process_name"`
	ProcessID   int       `json:"process_id"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewDetector 创建高级检测器
func NewDetector() *Detector {
	d := &Detector{
		policies:     make(map[string]*ShieldPolicy),
		rules:        make(map[string]*ThreatRule),
		patterns:     make(map[string]*AttackPattern),
		threatEvents: make([]ThreatEvent, 0, 1000),
		stopChan:     make(chan struct{}),
		eventChan:    make(chan FileEvent, 500),
		threatChan:   make(chan ThreatEvent, 50),
	}

	// 加载内置规则和模式
	d.loadBuiltinRules()
	d.loadBuiltinPatterns()

	return d
}

// Start 启动检测器
func (d *Detector) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return nil
	}
	d.running = true
	d.startTime = time.Now()
	d.mu.Unlock()

	go d.eventLoop(ctx)
	go d.analysisLoop(ctx)
	go d.threatHandlerLoop(ctx)

	log.Println("[RansomShield] 高级检测器已启动")
	return nil
}

// Stop 停止检测器
func (d *Detector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	close(d.stopChan)
	d.running = false
	log.Println("[RansomShield] 高级检测器已停止")
}

// GetStatus 获取防护状态
func (d *Detector) GetStatus() ShieldStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var uptime int64
	if d.running {
		uptime = int64(time.Since(d.startTime).Seconds())
	}

	activePolicies := 0
	for _, p := range d.policies {
		if p.Enabled {
			activePolicies++
		}
	}
	activeRules := 0
	for _, r := range d.rules {
		if r.Enabled {
			activeRules++
		}
	}

	var lastThreat *ThreatEvent
	if len(d.threatEvents) > 0 {
		last := d.threatEvents[len(d.threatEvents)-1]
		lastThreat = &last
	}

	return ShieldStatus{
		Running:          d.running,
		Uptime:           uptime,
		PoliciesActive:   activePolicies,
		RulesActive:      activeRules,
		ThreatsDetected:  d.stats.AnomaliesDetected,
		ThreatsBlocked:   d.stats.BlocksTriggered,
		SnapshotsCreated: 0, // 由 protector 统计
		RecoveryPoints:   0, // 由 protector 管理
		LastThreat:       lastThreat,
		Stats:            d.stats,
	}
}

// RecordEvent 记录文件事件
func (d *Detector) RecordEvent(event FileEvent) {
	select {
	case d.eventChan <- event:
	default:
		log.Println("[RansomShield] 事件通道满，丢弃事件")
	}
}

// GetThreatEvents 获取威胁事件列表
func (d *Detector) GetThreatEvents(page, perPage int) ([]ThreatEvent, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	total := len(d.threatEvents)
	start := (page - 1) * perPage
	if start >= total {
		return nil, total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	result := make([]ThreatEvent, end-start)
	copy(result, d.threatEvents[start:end])
	return result, total
}

// AddPolicy 添加防护策略
func (d *Detector) AddPolicy(policy *ShieldPolicy) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.policies[policy.ID] = policy
}

// RemovePolicy 移除防护策略
func (d *Detector) RemovePolicy(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.policies, id)
}

// GetPolicies 获取所有策略
func (d *Detector) GetPolicies() []ShieldPolicy {
	d.mu.RLock()
	defer d.mu.RUnlock()

	policies := make([]ShieldPolicy, 0, len(d.policies))
	for _, p := range d.policies {
		policies = append(policies, *p)
	}
	return policies
}

// GetPolicy 获取指定策略
func (d *Detector) GetPolicy(id string) (*ShieldPolicy, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	p, ok := d.policies[id]
	if !ok {
		return nil, false
	}
	result := *p
	return &result, true
}

// AddRule 添加威胁规则
func (d *Detector) AddRule(rule *ThreatRule) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rules[rule.ID] = rule
}

// GetRules 获取所有规则
func (d *Detector) GetRules() []ThreatRule {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rules := make([]ThreatRule, 0, len(d.rules))
	for _, r := range d.rules {
		rules = append(rules, *r)
	}
	return rules
}

// ============================================================
// 事件处理循环
// ============================================================

// eventLoop 文件事件处理循环
func (d *Detector) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case event := <-d.eventChan:
			d.processFileEvent(event)
		}
	}
}

// processFileEvent 处理单个文件事件
func (d *Detector) processFileEvent(event FileEvent) {
	d.mu.Lock()
	d.stats.TotalFilesScanned++
	d.mu.Unlock()

	// 检查是否在监控路径内
	if !d.isMonitored(event.Path) {
		return
	}

	// 熵分析
	entropy := d.analyzeEntropy(event)

	// 行为模式匹配
	threats := d.matchPatterns(event, entropy)

	for _, threat := range threats {
		select {
		case d.threatChan <- threat:
		default:
			log.Printf("[RansomShield] 威胁通道满，丢弃事件: %s", threat.ID)
		}
	}
}

// threatHandlerLoop 威胁事件处理循环
func (d *Detector) threatHandlerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case threat := <-d.threatChan:
			d.handleThreat(threat)
		}
	}
}

// handleThreat 处理威胁事件
func (d *Detector) handleThreat(threat ThreatEvent) {
	d.mu.Lock()
	d.threatEvents = append(d.threatEvents, threat)
	d.stats.AnomaliesDetected++

	// 保持事件历史在合理范围
	if len(d.threatEvents) > 10000 {
		d.threatEvents = d.threatEvents[1:]
	}
	d.mu.Unlock()

	log.Printf("[RansomShield] 威胁检测: ID=%s, Level=%d, Score=%d, Path=%s",
		threat.ID, threat.Level, threat.Score, threat.SourcePath)
}

// ============================================================
// 熵分析
// ============================================================

// analyzeEntropy 对文件进行熵分析
func (d *Detector) analyzeEntropy(event FileEvent) float64 {
	// 只对修改和创建事件做熵分析
	if event.EventType != "modify" && event.EventType != "create" {
		return 0
	}

	// 读取文件数据进行熵计算
	data, err := os.ReadFile(event.Path)
	if err != nil {
		return 0
	}

	if len(data) == 0 {
		return 0
	}

	// 采样分析（大文件只分析前 64KB）
	sampleSize := 65536
	if len(data) < sampleSize {
		sampleSize = len(data)
	}

	entropy := calculateEntropy(data[:sampleSize])

	// 统计高熵文件
	d.mu.Lock()
	if entropy > 7.5 {
		d.stats.HighEntropyFiles++
	}
	d.mu.Unlock()

	return entropy
}

// calculateEntropy 计算字节数据的香农熵
func calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}

	total := float64(len(data))
	var entropy float64

	for _, count := range freq {
		p := float64(count) / total
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// ============================================================
// 行为模式匹配
// ============================================================

// matchPatterns 匹配攻击模式，返回匹配到的威胁事件
func (d *Detector) matchPatterns(event FileEvent, entropy float64) []ThreatEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var threats []ThreatEvent

	for _, rule := range d.rules {
		if !rule.Enabled {
			continue
		}

		matched, score := d.evaluateRule(rule, event, entropy)
		if matched {
			threat := ThreatEvent{
				ID:          uuid.New().String(),
				RuleID:      rule.ID,
				Level:       rule.Level,
				Phase:       rule.Phase,
				Score:       score,
				Confidence:  float64(rule.Weight) / 100.0,
				SourcePath:  event.Path,
				ProcessName: event.ProcessName,
				ProcessID:   event.ProcessID,
				Indicators:  []string{fmt.Sprintf("rule:%s", rule.ID)},
				CreatedAt:   time.Now(),
			}
			threats = append(threats, threat)
		}
	}

	return threats
}

// evaluateRule 评估单个规则
func (d *Detector) evaluateRule(rule *ThreatRule, event FileEvent, entropy float64) (bool, int) {
	score := 0
	matched := 0

	for _, cond := range rule.Conditions {
		hit, condScore := d.evaluateCondition(cond, event, entropy)
		if hit {
			matched++
			score += condScore
		}
	}

	// 所有条件都命中才触发（AND 逻辑）
	if matched == len(rule.Conditions) {
		return true, score * rule.Weight / 100
	}

	return false, 0
}

// evaluateCondition 评估单个条件
func (d *Detector) evaluateCondition(cond Condition, event FileEvent, entropy float64) (bool, int) {
	switch cond.Type {
	case ConditionEntropy:
		threshold, ok := cond.Value.(float64)
		if !ok {
			return false, 0
		}
		if compareFloat(entropy, threshold, cond.Operator) {
			return true, int(entropy * 10)
		}

	case ConditionWriteFreq:
		// 基于时间窗口的写入频率
		threshold, ok := toFloat64(cond.Value)
		if !ok {
			return false, 0
		}
		// 简化：使用事件频率估算
		if event.EventType == "modify" && threshold > 0 {
			return true, int(threshold)
		}

	case ConditionRenameFreq:
		threshold, ok := toFloat64(cond.Value)
		if !ok {
			return false, 0
		}
		if event.EventType == "rename" && threshold > 0 {
			return true, int(threshold)
		}

	case ConditionDeleteFreq:
		threshold, ok := toFloat64(cond.Value)
		if !ok {
			return false, 0
		}
		if event.EventType == "delete" && threshold > 0 {
			return true, int(threshold)
		}

	case ConditionExtChange:
		pattern, ok := cond.Value.(string)
		if !ok {
			return false, 0
		}
		if event.EventType == "rename" && event.OldPath != "" {
			oldExt := strings.ToLower(filepath.Ext(event.OldPath))
			newExt := strings.ToLower(filepath.Ext(event.Path))
			if oldExt != newExt {
				matched, _ := regexp.MatchString(pattern, newExt)
				if matched {
					return true, 50
				}
			}
		}

	case ConditionFileSizeChange:
		// 文件大小突变检测
		threshold, ok := toFloat64(cond.Value)
		if !ok {
			return false, 0
		}
		if event.EventType == "modify" && event.Size > int64(threshold) {
			return true, 30
		}

	case ConditionHoneypotAccess:
		// 蜜罐访问检测
		if d.isHoneypotPath(event.Path) {
			return true, 100
		}

	case ConditionProcessAnomaly:
		// 进程异常行为检测
		suspiciousProcs := []string{
			"encrypt", "crypto", "ransom", "locky", "wanna",
			"cerber", "petya", "notpetya", "ryuk", "maze",
		}
		procLower := strings.ToLower(event.ProcessName)
		for _, sp := range suspiciousProcs {
			if strings.Contains(procLower, sp) {
				return true, 80
			}
		}
	}

	return false, 0
}

// ============================================================
// 分析循环
// ============================================================

// analysisLoop 定时分析循环
func (d *Detector) analysisLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.runPeriodicScan()
		}
	}
}

// runPeriodicScan 执行周期性扫描
func (d *Detector) runPeriodicScan() {
	d.mu.RLock()
	policies := make([]*ShieldPolicy, 0)
	for _, p := range d.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	d.mu.RUnlock()

	if len(policies) == 0 {
		return
	}

	start := time.Now()

	for _, policy := range policies {
		d.scanPaths(policy)
	}

	d.mu.Lock()
	d.stats.TotalScans++
	d.stats.LastScanTime = time.Now()
	d.stats.ScanDurationMs = time.Since(start).Milliseconds()
	d.mu.Unlock()
}

// scanPaths 扫描策略中的路径
func (d *Detector) scanPaths(policy *ShieldPolicy) {
	for _, watchPath := range policy.WatchPaths {
		err := filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}

			// 检查是否在排除路径
			for _, excludePath := range policy.ExcludePaths {
				if strings.HasPrefix(path, excludePath) {
					return nil
				}
			}

			// 熵分析
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			if len(data) > 0 {
				sampleSize := 65536
				if len(data) < sampleSize {
					sampleSize = len(data)
				}
				entropy := calculateEntropy(data[:sampleSize])

				if entropy > policy.EntropyThreshold {
					// 高熵文件，记录
					d.mu.Lock()
					d.stats.HighEntropyFiles++
					d.mu.Unlock()
				}
			}

			return nil
		})

		if err != nil {
			log.Printf("[RansomShield] 扫描路径失败: %s, error: %v", watchPath, err)
		}
	}
}

// ============================================================
// 内置规则和模式
// ============================================================

// loadBuiltinRules 加载内置威胁规则
func (d *Detector) loadBuiltinRules() {
	d.rules = map[string]*ThreatRule{
		"rapid-encryption": {
			ID:          "rapid-encryption",
			Name:        "快速加密检测",
			Description: "短时间内大量文件被高熵写入，疑似勒索加密",
			Enabled:     true,
			Level:       ThreatLevelCritical,
			Phase:       AttackPhaseEncrypt,
			Weight:      95,
			Conditions: []Condition{
				{Type: ConditionEntropy, Operator: "gte", Value: 7.5},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		"bulk-rename": {
			ID:          "bulk-rename",
			Name:        "批量重命名检测",
			Description: "大量文件扩展名被修改，疑似勒索软件重命名",
			Enabled:     true,
			Level:       ThreatLevelHigh,
			Phase:       AttackPhaseEncrypt,
			Weight:      85,
			Conditions: []Condition{
				{Type: ConditionRenameFreq, Operator: "gte", Value: 10, TimeWindowSec: 60},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		"ext-change-suspicious": {
			ID:          "ext-change-suspicious",
			Name:        "可疑扩展名变更",
			Description: "文件扩展名被改为已知勒索软件扩展名",
			Enabled:     true,
			Level:       ThreatLevelCritical,
			Phase:       AttackPhaseEncrypt,
			Weight:      90,
			Conditions: []Condition{
				{Type: ConditionExtChange, Operator: "regex", Value: `\.(encrypted|locked|crypto|crypt|locky|cerber|wncry)`},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		"rapid-delete": {
			ID:          "rapid-delete",
			Name:        "快速删除检测",
			Description: "短时间内大量文件被删除",
			Enabled:     true,
			Level:       ThreatLevelHigh,
			Phase:       AttackPhaseExecute,
			Weight:      80,
			Conditions: []Condition{
				{Type: ConditionDeleteFreq, Operator: "gte", Value: 50, TimeWindowSec: 30},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		"honeypot-trigger": {
			ID:          "honeypot-trigger",
			Name:        "蜜罐触发检测",
			Description: "蜜罐文件被访问或修改，高度疑似勒索攻击",
			Enabled:     true,
			Level:       ThreatLevelCritical,
			Phase:       AttackPhaseRecon,
			Weight:      100,
			Conditions: []Condition{
				{Type: ConditionHoneypotAccess, Operator: "eq", Value: true},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		"suspicious-process": {
			ID:          "suspicious-process",
			Name:        "可疑进程检测",
			Description: "检测到已知勒索软件进程名",
			Enabled:     true,
			Level:       ThreatLevelCritical,
			Phase:       AttackPhaseExecute,
			Weight:      95,
			Conditions: []Condition{
				{Type: ConditionProcessAnomaly, Operator: "eq", Value: true},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// loadBuiltinPatterns 加载内置攻击模式
func (d *Detector) loadBuiltinPatterns() {
	d.patterns = map[string]*AttackPattern{
		"pattern-encrypt-burst": {
			ID:          "pattern-encrypt-burst",
			Name:        "加密突发模式",
			Description: "短时间内大量文件被加密，典型勒索软件行为",
			Phase:       AttackPhaseEncrypt,
			Severity:    ThreatLevelCritical,
			Confidence:  0.95,
			Indicators: []Indicator{
				{Type: "entropy", Weight: 0.4, Threshold: 7.5, WindowSize: 30},
				{Type: "write_freq", Weight: 0.3, Threshold: 100, WindowSize: 60},
				{Type: "ext_change", Weight: 0.3, Threshold: 50, WindowSize: 60},
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		"pattern-recon-honeypot": {
			ID:          "pattern-recon-honeypot",
			Name:        "蜜罐侦察模式",
			Description: "访问蜜罐文件，表明攻击者正在扫描文件系统",
			Phase:       AttackPhaseRecon,
			Severity:    ThreatLevelHigh,
			Confidence:  0.85,
			Indicators: []Indicator{
				{Type: "honeypot_access", Weight: 1.0, Threshold: 1, WindowSize: 300},
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		"pattern-data-exfil": {
			ID:          "pattern-data-exfil",
			Name:        "数据窃取模式",
			Description: "大量文件被读取后发送到外部，疑似数据窃取",
			Phase:       AttackPhaseExfil,
			Severity:    ThreatLevelHigh,
			Confidence:  0.70,
			Indicators: []Indicator{
				{Type: "read_freq", Weight: 0.5, Threshold: 500, WindowSize: 300},
				{Type: "network_anomaly", Weight: 0.5, Threshold: 1, WindowSize: 300},
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
	}
}

// ============================================================
// 辅助方法
// ============================================================

// isMonitored 检查路径是否被监控
func (d *Detector) isMonitored(path string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 如果没有策略，默认监控所有
	if len(d.policies) == 0 {
		return true
	}

	for _, policy := range d.policies {
		if !policy.Enabled {
			continue
		}

		// 检查排除路径
		for _, excludePath := range policy.ExcludePaths {
			if strings.HasPrefix(path, excludePath) {
				return false
			}
		}

		// 检查监控路径
		for _, watchPath := range policy.WatchPaths {
			if strings.HasPrefix(path, watchPath) {
				return true
			}
		}
	}

	return false
}

// isHoneypotPath 检查是否为蜜罐路径
func (d *Detector) isHoneypotPath(path string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, policy := range d.policies {
		if !policy.HoneypotEnabled {
			continue
		}
		for _, hp := range policy.HoneypotPaths {
			if strings.HasPrefix(path, hp) {
				return true
			}
		}
	}
	return false
}

// compareFloat 浮点数比较
func compareFloat(a, b float64, op string) bool {
	switch op {
	case "gt":
		return a > b
	case "gte":
		return a >= b
	case "lt":
		return a < b
	case "lte":
		return a <= b
	case "eq":
		return math.Abs(a-b) < 1e-9
	default:
		return false
	}
}

// toFloat64 安全转换为 float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float32:
		return float64(val), true
	default:
		return 0, false
	}
}
