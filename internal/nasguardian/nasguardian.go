// Package nasguardian - NAS安全卫士
// 提供实时威胁检测、入侵防御、安全加固、漏洞扫描、安全评分功能
package nasguardian

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ========== 常量定义 ==========

// ThreatLevel 威胁等级
type ThreatLevel string

const (
	ThreatLevelLow      ThreatLevel = "low"      // 低风险
	ThreatLevelMedium   ThreatLevel = "medium"   // 中风险
	ThreatLevelHigh     ThreatLevel = "high"     // 高风险
	ThreatLevelCritical ThreatLevel = "critical" // 严重
)

// ThreatStatus 威胁状态
type ThreatStatus string

const (
	ThreatStatusActive    ThreatStatus = "active"    // 活跃
	ThreatStatusMitigated ThreatStatus = "mitigated" // 已缓解
	ThreatStatusResolved  ThreatStatus = "resolved"  // 已解决
)

// VulnSeverity 漏洞严重程度
type VulnSeverity string

const (
	VulnSeverityLow      VulnSeverity = "low"
	VulnSeverityMedium   VulnSeverity = "medium"
	VulnSeverityHigh     VulnSeverity = "high"
	VulnSeverityCritical VulnSeverity = "critical"
)

// HardeningCategory 加固类别
type HardeningCategory string

const (
	HardeningNetwork    HardeningCategory = "network"    // 网络加固
	HardeningAuth       HardeningCategory = "auth"       // 认证加固
	HardeningEncryption HardeningCategory = "encryption" // 加密加固
	HardeningSystem     HardeningCategory = "system"     // 系统加固
)

// ========== 错误定义 ==========

var (
	ErrGuardianNotRunning = errors.New("guardian is not running")
	ErrGuardianRunning    = errors.New("guardian is already running")
	ErrInvalidIP          = errors.New("invalid IP address")
	ErrIPNotBlocked       = errors.New("IP is not blocked")
	ErrRuleNotFound       = errors.New("security rule not found")
	ErrInvalidConfig      = errors.New("invalid configuration")
)

// ========== 数据结构 ==========

// Config NAS安全卫士配置
type Config struct {
	ScanInterval       time.Duration `json:"scan_interval"`        // 扫描间隔
	AlertThreshold     int           `json:"alert_threshold"`      // 告警阈值（每小时最大告警数）
	AutoRepair         bool          `json:"auto_repair"`          // 自动修复
	MaxBlockedIPs      int           `json:"max_blocked_ips"`      // 最大封锁IP数
	BlockDuration      time.Duration `json:"block_duration"`       // 默认封锁时长
	MaxThreatHistory   int           `json:"max_threat_history"`   // 最大威胁历史记录数
	VulnScanEnabled    bool          `json:"vuln_scan_enabled"`    // 漏洞扫描开关
	RealtimeMonitoring bool          `json:"realtime_monitoring"`  // 实时监控
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		ScanInterval:       30 * time.Minute,
		AlertThreshold:     100,
		AutoRepair:         true,
		MaxBlockedIPs:      1000,
		BlockDuration:      24 * time.Hour,
		MaxThreatHistory:   10000,
		VulnScanEnabled:    true,
		RealtimeMonitoring: true,
	}
}

// Threat 威胁信息
type Threat struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"`        // 威胁类型
	Level       ThreatLevel  `json:"level"`       // 威胁等级
	Status      ThreatStatus `json:"status"`      // 威胁状态
	Source      string       `json:"source"`      // 来源IP/路径
	Description string       `json:"description"` // 描述
	DetectedAt  time.Time    `json:"detected_at"` // 检测时间
	ResolvedAt  *time.Time   `json:"resolved_at,omitempty"`
	Details     string       `json:"details,omitempty"`
}

// Vulnerability 漏洞信息
type Vulnerability struct {
	ID          string       `json:"id"`
	CVE         string       `json:"cve"`         // CVE编号
	Severity    VulnSeverity `json:"severity"`    // 严重程度
	Title       string       `json:"title"`       // 标题
	Description string       `json:"description"` // 描述
	Affected    string       `json:"affected"`    // 受影响组件
	FixVersion  string       `json:"fix_version"` // 修复版本
	DetectedAt  time.Time    `json:"detected_at"` // 检测时间
	Fixed       bool         `json:"fixed"`       // 是否已修复
}

// SecurityScore 安全评分
type SecurityScore struct {
	Overall     int       `json:"overall"`      // 总分 0-100
	Network     int       `json:"network"`      // 网络安全分
	Auth        int       `json:"auth"`         // 认证安全分
	Encryption  int       `json:"encryption"`   // 加密安全分
	System      int       `json:"system"`       // 系统安全分
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
	ThreatCount int       `json:"threat_count"` // 活跃威胁数
	VulnCount   int       `json:"vuln_count"`   // 未修复漏洞数
}

// SecurityRule 安全规则
type SecurityRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    HardeningCategory `json:"category"`
	Enabled     bool              `json:"enabled"`
	Description string            `json:"description"`
	Condition   string            `json:"condition"` // 规则条件
	Action      string            `json:"action"`    // 触发动作
	Severity    ThreatLevel       `json:"severity"`
	CreatedAt   time.Time         `json:"created_at"`
}

// HardeningTask 加固任务
type HardeningTask struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    HardeningCategory `json:"category"`
	Description string            `json:"description"`
	Applied     bool              `json:"applied"`
	AppliedAt   *time.Time        `json:"applied_at,omitempty"`
	Rollback    bool              `json:"rollback"` // 是否可回滚
}

// BlockedIP 被封锁的IP信息
type BlockedIP struct {
	IP        string    `json:"ip"`
	Reason    string    `json:"reason"`
	BlockedAt time.Time `json:"blocked_at"`
	ExpiresAt time.Time `json:"expires_at"`
	HitCount  int       `json:"hit_count"` // 命中次数
}

// SecurityReport 安全报告
type SecurityReport struct {
	GeneratedAt     time.Time         `json:"generated_at"`
	Score           SecurityScore     `json:"score"`
	ActiveThreats   int               `json:"active_threats"`
	TotalThreats    int               `json:"total_threats"`
	OpenVulns       int               `json:"open_vulns"`
	BlockedIPs      int               `json:"blocked_ips"`
	ActiveRules     int               `json:"active_rules"`
	AppliedHardenings int             `json:"applied_hardenings"`
	RecentThreats   []Threat          `json:"recent_threats"`
	TopVulns        []Vulnerability   `json:"top_vulns"`
}

// ========== NASGuardian 主结构体 ==========

// NASGuardian NAS安全卫士
type NASGuardian struct {
	mu             sync.RWMutex
	config         Config
	running        bool
	cancel         context.CancelFunc

	threats        map[string]*Threat
	vulns          map[string]*Vulnerability
	rules          map[string]*SecurityRule
	hardeningTasks map[string]*HardeningTask
	blockedIPs     map[string]*BlockedIP
	threatHistory  []*Threat

	threatCounter  int
	vulnCounter    int
	ruleCounter    int
	taskCounter    int

	score          SecurityScore
}

// New 创建NAS安全卫士实例
func New(config Config) *NASGuardian {
	if config.ScanInterval == 0 {
		config = DefaultConfig()
	}
	if config.MaxBlockedIPs == 0 {
		config.MaxBlockedIPs = 1000
	}
	if config.BlockDuration == 0 {
		config.BlockDuration = 24 * time.Hour
	}
	if config.MaxThreatHistory == 0 {
		config.MaxThreatHistory = 10000
	}

	return &NASGuardian{
		config:         config,
		threats:        make(map[string]*Threat),
		vulns:          make(map[string]*Vulnerability),
		rules:          make(map[string]*SecurityRule),
		hardeningTasks: make(map[string]*HardeningTask),
		blockedIPs:     make(map[string]*BlockedIP),
		threatHistory:  make([]*Threat, 0),
		score: SecurityScore{
			Overall:    100,
			Network:    100,
			Auth:       100,
			Encryption: 100,
			System:     100,
			UpdatedAt:  time.Now(),
		},
	}
}

// Start 启动NAS安全卫士
func (g *NASGuardian) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return ErrGuardianRunning
	}

	ctx, g.cancel = context.WithCancel(ctx)
	g.running = true

	// 启动后台扫描
	if g.config.RealtimeMonitoring {
		go g.monitorLoop(ctx)
	}

	return nil
}

// Stop 停止NAS安全卫士
func (g *NASGuardian) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return ErrGuardianNotRunning
	}

	if g.cancel != nil {
		g.cancel()
	}
	g.running = false

	return nil
}

// IsRunning 是否运行中
func (g *NASGuardian) IsRunning() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.running
}

// monitorLoop 监控循环
func (g *NASGuardian) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(g.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.ScanThreats(ctx)
			if g.config.VulnScanEnabled {
				g.ScanVulnerabilities(ctx)
			}
			g.EvaluateRules(ctx)
			g.updateScore()
		}
	}
}

// ========== 威胁检测 ==========

// ScanThreats 扫描威胁
func (g *NASGuardian) ScanThreats(ctx context.Context) ([]Threat, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return nil, ErrGuardianNotRunning
	}

	// 模拟威胁检测 - 检查异常登录、可疑连接等
	detected := make([]Threat, 0)

	// 检查封锁IP的异常访问
	for _, bip := range g.blockedIPs {
		bip.HitCount++
	}

	return detected, nil
}

// AddThreat 添加威胁记录
func (g *NASGuardian) AddThreat(threat Threat) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.threatCounter++
	threat.ID = fmt.Sprintf("threat-%d", g.threatCounter)
	if threat.DetectedAt.IsZero() {
		threat.DetectedAt = time.Now()
	}
	if threat.Status == "" {
		threat.Status = ThreatStatusActive
	}

	g.threats[threat.ID] = &threat
	g.threatHistory = append(g.threatHistory, &threat)

	// 限制历史记录大小
	if len(g.threatHistory) > g.config.MaxThreatHistory {
		g.threatHistory = g.threatHistory[len(g.threatHistory)-g.config.MaxThreatHistory:]
	}

	g.updateScoreUnsafe()
	return threat.ID
}

// ResolveThreat 解决威胁
func (g *NASGuardian) ResolveThreat(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	threat, ok := g.threats[id]
	if !ok {
		return fmt.Errorf("threat %s not found", id)
	}

	now := time.Now()
	threat.Status = ThreatStatusResolved
	threat.ResolvedAt = &now
	g.updateScoreUnsafe()

	return nil
}

// GetThreat 获取威胁详情
func (g *NASGuardian) GetThreat(id string) (*Threat, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	threat, ok := g.threats[id]
	if !ok {
		return nil, fmt.Errorf("threat %s not found", id)
	}
	return threat, nil
}

// GetThreatHistory 获取威胁历史
func (g *NASGuardian) GetThreatHistory(limit int) []Threat {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limit <= 0 || limit > len(g.threatHistory) {
		limit = len(g.threatHistory)
	}

	// 返回最新的记录
	start := len(g.threatHistory) - limit
	result := make([]Threat, limit)
	for i, t := range g.threatHistory[start:] {
		result[i] = *t
	}
	return result
}

// ========== 漏洞扫描 ==========

// ScanVulnerabilities 扫描漏洞
func (g *NASGuardian) ScanVulnerabilities(ctx context.Context) ([]Vulnerability, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return nil, ErrGuardianNotRunning
	}

	// 模拟漏洞扫描
	detected := make([]Vulnerability, 0)

	return detected, nil
}

// AddVulnerability 添加漏洞
func (g *NASGuardian) AddVulnerability(vuln Vulnerability) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.vulnCounter++
	vuln.ID = fmt.Sprintf("vuln-%d", g.vulnCounter)
	if vuln.DetectedAt.IsZero() {
		vuln.DetectedAt = time.Now()
	}

	g.vulns[vuln.ID] = &vuln
	g.updateScoreUnsafe()
	return vuln.ID
}

// FixVulnerability 修复漏洞
func (g *NASGuardian) FixVulnerability(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	vuln, ok := g.vulns[id]
	if !ok {
		return fmt.Errorf("vulnerability %s not found", id)
	}

	vuln.Fixed = true
	g.updateScoreUnsafe()
	return nil
}

// GetVulnerabilities 获取所有漏洞
func (g *NASGuardian) GetVulnerabilities() []Vulnerability {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Vulnerability, 0, len(g.vulns))
	for _, v := range g.vulns {
		result = append(result, *v)
	}
	return result
}

// ========== 安全评分 ==========

// GetSecurityScore 获取安全评分
func (g *NASGuardian) GetSecurityScore() SecurityScore {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.score
}

// updateScore 更新安全评分（需要外部锁）
func (g *NASGuardian) updateScore() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.updateScoreUnsafe()
}

// updateScoreUnsafe 更新安全评分（不加锁，调用者需持锁）
func (g *NASGuardian) updateScoreUnsafe() {
	activeThreats := 0
	for _, t := range g.threats {
		if t.Status == ThreatStatusActive {
			activeThreats++
		}
	}

	openVulns := 0
	for _, v := range g.vulns {
		if !v.Fixed {
			openVulns++
		}
	}

	// 计算各维度得分
	networkScore := 100 - activeThreats*10 - len(g.blockedIPs)*2
	authScore := 100
	encryptionScore := 100
	systemScore := 100 - openVulns*15

	// 钳制分数范围
	g.score.Network = clampScore(networkScore)
	g.score.Auth = clampScore(authScore)
	g.score.Encryption = clampScore(encryptionScore)
	g.score.System = clampScore(systemScore)
	g.score.Overall = (g.score.Network + g.score.Auth + g.score.Encryption + g.score.System) / 4
	g.score.UpdatedAt = time.Now()
	g.score.ThreatCount = activeThreats
	g.score.VulnCount = openVulns
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// ========== 安全加固 ==========

// ApplyHardening 应用安全加固
func (g *NASGuardian) ApplyHardening(ctx context.Context, task HardeningTask) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return ErrGuardianNotRunning
	}

	g.taskCounter++
	task.ID = fmt.Sprintf("task-%d", g.taskCounter)
	now := time.Now()
	task.Applied = true
	task.AppliedAt = &now

	g.hardeningTasks[task.ID] = &task
	g.updateScoreUnsafe()
	return nil
}

// GetHardeningTasks 获取加固任务列表
func (g *NASGuardian) GetHardeningTasks() []HardeningTask {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]HardeningTask, 0, len(g.hardeningTasks))
	for _, t := range g.hardeningTasks {
		result = append(result, *t)
	}
	return result
}

// ========== 安全规则 ==========

// AddRule 添加安全规则
func (g *NASGuardian) AddRule(rule SecurityRule) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.ruleCounter++
	rule.ID = fmt.Sprintf("rule-%d", g.ruleCounter)
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	if rule.Enabled {
		rule.Enabled = true
	}

	g.rules[rule.ID] = &rule
	return rule.ID
}

// RemoveRule 移除安全规则
func (g *NASGuardian) RemoveRule(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.rules[id]; !ok {
		return ErrRuleNotFound
	}
	delete(g.rules, id)
	return nil
}

// GetRules 获取所有安全规则
func (g *NASGuardian) GetRules() []SecurityRule {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]SecurityRule, 0, len(g.rules))
	for _, r := range g.rules {
		result = append(result, *r)
	}
	return result
}

// EvaluateRules 评估安全规则
func (g *NASGuardian) EvaluateRules(ctx context.Context) ([]SecurityRule, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.running {
		return nil, ErrGuardianNotRunning
	}

	triggered := make([]SecurityRule, 0)
	for _, rule := range g.rules {
		if !rule.Enabled {
			continue
		}
		// 评估规则条件（简化实现）
		if g.evaluateRuleCondition(rule) {
			triggered = append(triggered, *rule)
		}
	}

	return triggered, nil
}

// evaluateRuleCondition 评估规则条件（内部方法）
func (g *NASGuardian) evaluateRuleCondition(rule *SecurityRule) bool {
	// 简化实现：基于规则类别进行基本评估
	switch rule.Category {
	case HardeningNetwork:
		// 网络规则：检查封锁IP数量
		return len(g.blockedIPs) > g.config.MaxBlockedIPs/2
	case HardeningAuth:
		// 认证规则：检查活跃威胁数
		activeThreats := 0
		for _, t := range g.threats {
			if t.Status == ThreatStatusActive {
				activeThreats++
			}
		}
		return activeThreats > 0
	default:
		return false
	}
}

// ========== IP封锁 ==========

// BlockIP 封锁IP
func (g *NASGuardian) BlockIP(ctx context.Context, ip string, duration time.Duration) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return ErrGuardianNotRunning
	}

	// 验证IP格式
	if net.ParseIP(ip) == nil {
		return ErrInvalidIP
	}

	// 检查封锁数量限制
	if _, exists := g.blockedIPs[ip]; !exists && len(g.blockedIPs) >= g.config.MaxBlockedIPs {
		return fmt.Errorf("blocked IP limit reached (%d)", g.config.MaxBlockedIPs)
	}

	if duration == 0 {
		duration = g.config.BlockDuration
	}

	now := time.Now()
	g.blockedIPs[ip] = &BlockedIP{
		IP:        ip,
		Reason:    "Manual block",
		BlockedAt: now,
		ExpiresAt: now.Add(duration),
	}

	return nil
}

// UnblockIP 解除IP封锁
func (g *NASGuardian) UnblockIP(ip string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.blockedIPs[ip]; !ok {
		return ErrIPNotBlocked
	}
	delete(g.blockedIPs, ip)
	return nil
}

// GetBlockedIPs 获取封锁IP列表
func (g *NASGuardian) GetBlockedIPs() []BlockedIP {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]BlockedIP, 0, len(g.blockedIPs))
	for _, bip := range g.blockedIPs {
		result = append(result, *bip)
	}
	return result
}

// IsIPBlocked 检查IP是否被封锁
func (g *NASGuardian) IsIPBlocked(ip string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	bip, ok := g.blockedIPs[ip]
	if !ok {
		return false
	}

	// 检查是否过期
	if time.Now().After(bip.ExpiresAt) {
		delete(g.blockedIPs, ip)
		return false
	}
	return true
}

// CleanupExpiredBlocks 清理过期的IP封锁
func (g *NASGuardian) CleanupExpiredBlocks() int {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	count := 0
	for ip, bip := range g.blockedIPs {
		if now.After(bip.ExpiresAt) {
			delete(g.blockedIPs, ip)
			count++
		}
	}
	return count
}

// ========== 安全报告 ==========

// GenerateSecurityReport 生成安全报告
func (g *NASGuardian) GenerateSecurityReport() SecurityReport {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 统计活跃威胁
	activeThreats := 0
	for _, t := range g.threats {
		if t.Status == ThreatStatusActive {
			activeThreats++
		}
	}

	// 统计未修复漏洞
	openVulns := 0
	for _, v := range g.vulns {
		if !v.Fixed {
			openVulns++
		}
	}

	// 统计活跃规则
	activeRules := 0
	for _, r := range g.rules {
		if r.Enabled {
			activeRules++
		}
	}

	// 统计已应用的加固任务
	appliedHardenings := 0
	for _, t := range g.hardeningTasks {
		if t.Applied {
			appliedHardenings++
		}
	}

	// 最近威胁（最多5条）
	recentThreats := make([]Threat, 0, 5)
	threatList := make([]*Threat, 0, len(g.threats))
	for _, t := range g.threats {
		threatList = append(threatList, t)
	}
	// 简单排序：按检测时间倒序
	for i := 0; i < len(threatList) && i < 5; i++ {
		recentThreats = append(recentThreats, *threatList[i])
	}

	// 高危漏洞（最多5条）
	topVulns := make([]Vulnerability, 0, 5)
	for _, v := range g.vulns {
		if !v.Fixed && (v.Severity == VulnSeverityCritical || v.Severity == VulnSeverityHigh) {
			topVulns = append(topVulns, *v)
			if len(topVulns) >= 5 {
				break
			}
		}
	}

	return SecurityReport{
		GeneratedAt:       time.Now(),
		Score:             g.score,
		ActiveThreats:     activeThreats,
		TotalThreats:      len(g.threats),
		OpenVulns:         openVulns,
		BlockedIPs:        len(g.blockedIPs),
		ActiveRules:       activeRules,
		AppliedHardenings: appliedHardenings,
		RecentThreats:     recentThreats,
		TopVulns:          topVulns,
	}
}

// GetConfig 获取配置
func (g *NASGuardian) GetConfig() Config {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config
}
