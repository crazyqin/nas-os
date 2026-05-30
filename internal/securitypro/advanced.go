// Package securitypro 安全增强模块
// 对标 TrueNAS 安全功能和群晖安全防护
package securitypro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ThreatLevel 威胁等级
type ThreatLevel string

const (
	ThreatLevelCritical ThreatLevel = "critical"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelInfo     ThreatLevel = "info"
)

// ScanType 扫描类型
type ScanType string

const (
	ScanTypeFull    ScanType = "full"    // 全盘扫描
	ScanTypeQuick   ScanType = "quick"   // 快速扫描
	ScanTypeCustom  ScanType = "custom"  // 自定义扫描
	ScanTypeRealtime ScanType = "realtime" // 实时监控
)

// ScanStatus 扫描状态
type ScanStatus string

const (
	ScanStatusPending  ScanStatus = "pending"
	ScanStatusRunning  ScanStatus = "running"
	ScanStatusComplete ScanStatus = "complete"
	ScanStatusFailed   ScanStatus = "failed"
	ScanStatusCanceled ScanStatus = "canceled"
)

// FileType 文件类型
type FileType string

const (
	FileTypeExecutable FileType = "executable"
	FileTypeScript     FileType = "script"
	FileTypeArchive    FileType = "archive"
	FileTypeDocument   FileType = "document"
	FileTypeMedia      FileType = "media"
	FileTypeUnknown    FileType = "unknown"
)

// Threat 威胁定义
type Threat struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Level       ThreatLevel `json:"level"`
	Category    string      `json:"category"` // malware, ransomware, trojan, adware, etc.
	Signature   string      `json:"signature"`
	FilePath    string      `json:"file_path"`
	FileHash    string      `json:"file_hash"`
	FileSize    int64       `json:"file_size"`
	FileType    FileType    `json:"file_type"`
	DetectedAt  time.Time   `json:"detected_at"`
	Status      string      `json:"status"` // detected, quarantined, removed, whitelisted
	Actions     []string    `json:"actions"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID          string    `json:"id"`
	ScanType    ScanType  `json:"scan_type"`
	Status      ScanStatus `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at,omitempty"`
	FilesScanned int64    `json:"files_scanned"`
	ThreatsFound int      `json:"threats_found"`
	Threats      []*Threat `json:"threats"`
	Duration     time.Duration `json:"duration"`
	ErrorMessage string   `json:"error_message,omitempty"`
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Enabled         bool      `json:"enabled"`
	RealTimeScan    bool      `json:"realtime_scan"`
	ScanSchedule    string    `json:"scan_schedule"` // cron expression
	AutoQuarantine  bool      `json:"auto_quarantine"`
	AutoRemove      bool      `json:"auto_remove"`
	NotifyAdmin     bool      `json:"notify_admin"`
	Whitelist       []string  `json:"whitelist,omitempty"`
	Blacklist       []string  `json:"blacklist,omitempty"`
	MaxFileSize     int64     `json:"max_file_size"` // bytes
	ScanExtensions  []string  `json:"scan_extensions,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// FirewallRule 防火墙规则
type FirewallRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Action      string    `json:"action"` // allow, deny, reject
	Protocol    string    `json:"protocol"` // tcp, udp, icmp, any
	SourceIP    string    `json:"source_ip"`
	SourcePort  string    `json:"source_port"`
	DestIP      string    `json:"dest_ip"`
	DestPort    string    `json:"dest_port"`
	Direction   string    `json:"direction"` // in, out, both
	Priority    int       `json:"priority"`
	LogEnabled  bool      `json:"log_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IntrusionDetection 入侵检测
type IntrusionDetection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Rules       []IDSRule `json:"rules"`
	AlertLevel  ThreatLevel `json:"alert_level"`
	AutoBlock   bool      `json:"auto_block"`
	BlockDuration time.Duration `json:"block_duration"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IDSRule IDS 规则
type IDSRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Pattern     string      `json:"pattern"`
	Level       ThreatLevel `json:"level"`
	Action      string      `json:"action"` // alert, block, drop
	Enabled     bool        `json:"enabled"`
}

// AccessLog 访问日志
type AccessLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	SourceIP  string    `json:"source_ip"`
	UserID    string    `json:"user_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Status    string    `json:"status"` // success, failed, blocked
	UserAgent string    `json:"user_agent,omitempty"`
	Details   string    `json:"details,omitempty"`
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID          string      `json:"id"`
	Timestamp   time.Time   `json:"timestamp"`
	Type        string      `json:"type"` // login_failed, brute_force, malware, intrusion, etc.
	Level       ThreatLevel `json:"level"`
	Source      string      `json:"source"`
	Description string      `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Resolved    bool        `json:"resolved"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
	ResolvedBy  string      `json:"resolved_by,omitempty"`
}

// Vulnerability 漏洞信息
type Vulnerability struct {
	ID          string    `json:"id"`
	CVEID       string    `json:"cve_id"`
	Package     string    `json:"package"`
	Version     string    `json:"version"`
	FixedVersion string   `json:"fixed_version,omitempty"`
	Severity    ThreatLevel `json:"severity"`
	Description string    `json:"description"`
	References  []string  `json:"references,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
	Status      string    `json:"status"` // open, fixed, mitigated
}

// Manager 安全管理器
type Manager struct {
	mu           sync.RWMutex
	threats      map[string]*Threat
	scans        map[string]*ScanResult
	policies     map[string]*SecurityPolicy
	firewall     map[string]*FirewallRule
	ids          map[string]*IntrusionDetection
	accessLogs   []*AccessLog
	events       []*SecurityEvent
	vulns        map[string]*Vulnerability
	logger       Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// NewManager 创建安全管理器
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		threats:    make(map[string]*Threat),
		scans:      make(map[string]*ScanResult),
		policies:   make(map[string]*SecurityPolicy),
		firewall:   make(map[string]*FirewallRule),
		ids:        make(map[string]*IntrusionDetection),
		accessLogs: make([]*AccessLog, 0),
		events:     make([]*SecurityEvent, 0),
		vulns:      make(map[string]*Vulnerability),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动实时监控
	m.wg.Add(1)
	go m.realtimeMonitor()

	// 启动日志分析
	m.wg.Add(1)
	go m.logAnalyzer()

	return m
}

// StartScan 启动安全扫描
func (m *Manager) StartScan(ctx context.Context, scanType ScanType, targets []string) (*ScanResult, error) {
	scan := &ScanResult{
		ID:        generateScanID(),
		ScanType:  scanType,
		Status:    ScanStatusRunning,
		StartedAt: time.Now(),
		Threats:   make([]*Threat, 0),
	}

	m.mu.Lock()
	m.scans[scan.ID] = scan
	m.mu.Unlock()

	// 异步执行扫描
	go m.executeScan(ctx, scan, targets)

	m.logger.Info("安全扫描启动: %s, 类型: %s", scan.ID, scanType)
	return scan, nil
}

// executeScan 执行扫描
func (m *Manager) executeScan(ctx context.Context, scan *ScanResult, targets []string) {
	defer func() {
		scan.EndedAt = time.Now()
		scan.Duration = scan.EndedAt.Sub(scan.StartedAt)
		if scan.Status == ScanStatusRunning {
			scan.Status = ScanStatusComplete
		}
	}()

	// 模拟扫描过程
	for i := 0; i < 100; i++ {
		select {
		case <-ctx.Done():
			scan.Status = ScanStatusCanceled
			return
		default:
			// 模拟扫描进度
			scan.FilesScanned += 100
		}
	}

	// 模拟发现威胁
	threat := &Threat{
		ID:          generateThreatID(),
		Name:        "Suspicious Script",
		Description: "检测到可疑脚本文件",
		Level:       ThreatLevelMedium,
		Category:    "malware",
		FilePath:    "/tmp/suspicious.sh",
		FileHash:    "abc123def456",
		FileSize:    1024,
		FileType:    FileTypeScript,
		DetectedAt:  time.Now(),
		Status:      "detected",
		Actions:     []string{"quarantine", "remove"},
	}

	scan.Threats = append(scan.Threats, threat)
	scan.ThreatsFound = len(scan.Threats)

	m.mu.Lock()
	m.threats[threat.ID] = threat
	m.mu.Unlock()

	m.logger.Info("扫描完成: %s, 发现威胁: %d", scan.ID, scan.ThreatsFound)
}

// GetScanResult 获取扫描结果
func (m *Manager) GetScanResult(scanID string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scan, ok := m.scans[scanID]
	if !ok {
		return nil, fmt.Errorf("扫描不存在: %s", scanID)
	}
	return scan, nil
}

// ListScans 列出所有扫描
func (m *Manager) ListScans() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scans := make([]*ScanResult, 0, len(m.scans))
	for _, scan := range m.scans {
		scans = append(scans, scan)
	}
	return scans
}

// QuarantineThreat 隔离威胁
func (m *Manager) QuarantineThreat(threatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	threat, ok := m.threats[threatID]
	if !ok {
		return fmt.Errorf("威胁不存在: %s", threatID)
	}

	threat.Status = "quarantined"
	threat.Actions = append(threat.Actions, "quarantined_at_"+time.Now().Format(time.RFC3339))

	m.logger.Info("威胁已隔离: %s", threat.Name)
	return nil
}

// RemoveThreat 移除威胁
func (m *Manager) RemoveThreat(threatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	threat, ok := m.threats[threatID]
	if !ok {
		return fmt.Errorf("威胁不存在: %s", threatID)
	}

	threat.Status = "removed"
	threat.Actions = append(threat.Actions, "removed_at_"+time.Now().Format(time.RFC3339))

	m.logger.Info("威胁已移除: %s", threat.Name)
	return nil
}

// WhitelistThreat 白名单威胁
func (m *Manager) WhitelistThreat(threatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	threat, ok := m.threats[threatID]
	if !ok {
		return fmt.Errorf("威胁不存在: %s", threatID)
	}

	threat.Status = "whitelisted"
	threat.Actions = append(threat.Actions, "whitelisted_at_"+time.Now().Format(time.RFC3339))

	m.logger.Info("威胁已加入白名单: %s", threat.Name)
	return nil
}

// CreateSecurityPolicy 创建安全策略
func (m *Manager) CreateSecurityPolicy(policy *SecurityPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generatePolicyID()
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	m.policies[policy.ID] = policy
	m.logger.Info("安全策略创建成功: %s (%s)", policy.Name, policy.ID)
	return nil
}

// UpdateSecurityPolicy 更新安全策略
func (m *Manager) UpdateSecurityPolicy(policy *SecurityPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.policies[policy.ID]
	if !ok {
		return fmt.Errorf("安全策略不存在: %s", policy.ID)
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()
	m.policies[policy.ID] = policy
	m.logger.Info("安全策略更新成功: %s (%s)", policy.Name, policy.ID)
	return nil
}

// AddFirewallRule 添加防火墙规则
func (m *Manager) AddFirewallRule(rule *FirewallRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.firewall[rule.ID] = rule
	m.logger.Info("防火墙规则添加成功: %s (%s)", rule.Name, rule.ID)
	return nil
}

// UpdateFirewallRule 更新防火墙规则
func (m *Manager) UpdateFirewallRule(rule *FirewallRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.firewall[rule.ID]
	if !ok {
		return fmt.Errorf("防火墙规则不存在: %s", rule.ID)
	}

	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.firewall[rule.ID] = rule
	m.logger.Info("防火墙规则更新成功: %s (%s)", rule.Name, rule.ID)
	return nil
}

// DeleteFirewallRule 删除防火墙规则
func (m *Manager) DeleteFirewallRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.firewall[ruleID]; !ok {
		return fmt.Errorf("防火墙规则不存在: %s", ruleID)
	}

	delete(m.firewall, ruleID)
	m.logger.Info("防火墙规则删除成功: %s", ruleID)
	return nil
}

// ListFirewallRules 列出防火墙规则
func (m *Manager) ListFirewallRules() []*FirewallRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*FirewallRule, 0, len(m.firewall))
	for _, rule := range m.firewall {
		rules = append(rules, rule)
	}
	return rules
}

// LogAccess 记录访问日志
func (m *Manager) LogAccess(log *AccessLog) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if log.ID == "" {
		log.ID = generateLogID()
	}
	log.Timestamp = time.Now()

	m.accessLogs = append(m.accessLogs, log)

	// 限制日志数量
	if len(m.accessLogs) > 10000 {
		m.accessLogs = m.accessLogs[1000:]
	}

	// 检测异常访问
	m.detectAnomalousAccess(log)
}

// detectAnomalousAccess 检测异常访问
func (m *Manager) detectAnomalousAccess(log *AccessLog) {
	// 检测暴力破解
	if log.Status == "failed" {
		failedCount := 0
		for _, existingLog := range m.accessLogs {
			if existingLog.SourceIP == log.SourceIP &&
				existingLog.Action == "login" &&
				existingLog.Status == "failed" &&
				time.Since(existingLog.Timestamp) < 5*time.Minute {
				failedCount++
			}
		}

		if failedCount >= 5 {
			event := &SecurityEvent{
				ID:          generateEventID(),
				Timestamp:   time.Now(),
				Type:        "brute_force",
				Level:       ThreatLevelHigh,
				Source:      log.SourceIP,
				Description: fmt.Sprintf("检测到暴力破解尝试，来源IP: %s，失败次数: %d", log.SourceIP, failedCount),
				Details: map[string]interface{}{
					"source_ip":    log.SourceIP,
					"failed_count": failedCount,
					"username":     log.Username,
				},
			}
			m.events = append(m.events, event)
			m.logger.Warn("检测到暴力破解: %s", event.Description)
		}
	}
}

// realtimeMonitor 实时监控
func (m *Manager) realtimeMonitor() {
	defer m.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performRealtimeScan()
		}
	}
}

// performRealtimeScan 执行实时扫描
func (m *Manager) performRealtimeScan() {
	// 模拟实时监控
	m.logger.Debug("执行实时安全监控检查")
}

// logAnalyzer 日志分析器
func (m *Manager) logAnalyzer() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.analyzeLogs()
		}
	}
}

// analyzeLogs 分析日志
func (m *Manager) analyzeLogs() {
	m.mu.RLock()
	logs := make([]*AccessLog, len(m.accessLogs))
	copy(logs, m.accessLogs)
	m.mu.RUnlock()

	// 分析最近的日志
	recentLogs := logs
	if len(logs) > 100 {
		recentLogs = logs[len(logs)-100:]
	}

	// 检测异常模式
	m.detectAnomalousPatterns(recentLogs)
}

// detectAnomalousPatterns 检测异常模式
func (m *Manager) detectAnomalousPatterns(logs []*AccessLog) {
	// 统计每个 IP 的访问次数
	ipCounts := make(map[string]int)
	for _, log := range logs {
		ipCounts[log.SourceIP]++
	}

	// 检测异常高频访问
	for ip, count := range ipCounts {
		if count > 100 {
			event := &SecurityEvent{
				ID:          generateEventID(),
				Timestamp:   time.Now(),
				Type:        "high_frequency_access",
				Level:       ThreatLevelMedium,
				Source:      ip,
				Description: fmt.Sprintf("检测到高频访问，来源IP: %s，访问次数: %d", ip, count),
			}
			m.mu.Lock()
			m.events = append(m.events, event)
			m.mu.Unlock()
			m.logger.Warn("检测到高频访问: %s", event.Description)
		}
	}
}

// GetAccessLogs 获取访问日志
func (m *Manager) GetAccessLogs(limit int) []*AccessLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.accessLogs) {
		limit = len(m.accessLogs)
	}

	start := len(m.accessLogs) - limit
	if start < 0 {
		start = 0
	}

	return m.accessLogs[start:]
}

// GetSecurityEvents 获取安全事件
func (m *Manager) GetSecurityEvents(limit int) []*SecurityEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}

	return m.events[start:]
}

// ResolveEvent 解决安全事件
func (m *Manager) ResolveEvent(eventID, resolvedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, event := range m.events {
		if event.ID == eventID {
			event.Resolved = true
			now := time.Now()
			event.ResolvedAt = &now
			event.ResolvedBy = resolvedBy
			m.logger.Info("安全事件已解决: %s", eventID)
			return nil
		}
	}

	return fmt.Errorf("安全事件不存在: %s", eventID)
}

// ScanVulnerabilities 扫描漏洞
func (m *Manager) ScanVulnerabilities(ctx context.Context) ([]*Vulnerability, error) {
	// 模拟漏洞扫描
	vulns := []*Vulnerability{
		{
			ID:          generateVulnID(),
			CVEID:       "CVE-2026-1234",
			Package:     "openssl",
			Version:     "1.1.1k",
			FixedVersion: "1.1.1l",
			Severity:    ThreatLevelHigh,
			Description: "OpenSSL 缓冲区溢出漏洞",
			References:  []string{"https://nvd.nist.gov/vuln/detail/CVE-2026-1234"},
			DetectedAt:  time.Now(),
			Status:      "open",
		},
	}

	m.mu.Lock()
	for _, vuln := range vulns {
		m.vulns[vuln.ID] = vuln
	}
	m.mu.Unlock()

	m.logger.Info("漏洞扫描完成，发现 %d 个漏洞", len(vulns))
	return vulns, nil
}

// GetVulnerabilities 获取漏洞列表
func (m *Manager) GetVulnerabilities() []*Vulnerability {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vulns := make([]*Vulnerability, 0, len(m.vulns))
	for _, vuln := range m.vulns {
		vulns = append(vulns, vuln)
	}
	return vulns
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

func generateScanID() string {
	return fmt.Sprintf("scan_%d", time.Now().UnixNano())
}

func generateThreatID() string {
	return fmt.Sprintf("threat_%d", time.Now().UnixNano())
}

func generatePolicyID() string {
	return fmt.Sprintf("policy_%d", time.Now().UnixNano())
}

func generateRuleID() string {
	return fmt.Sprintf("rule_%d", time.Now().UnixNano())
}

func generateLogID() string {
	return fmt.Sprintf("log_%d", time.Now().UnixNano())
}

func generateEventID() string {
	return fmt.Sprintf("event_%d", time.Now().UnixNano())
}

func generateVulnID() string {
	return fmt.Sprintf("vuln_%d", time.Now().UnixNano())
}

// CalculateFileHash 计算文件哈希
func CalculateFileHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// RegisterHandlers 注册 HTTP 处理器
func (m *Manager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/security/scan", m.handleScan)
	mux.HandleFunc("/api/security/scans", m.handleListScans)
	mux.HandleFunc("/api/security/threats", m.handleThreats)
	mux.HandleFunc("/api/security/policies", m.handlePolicies)
	mux.HandleFunc("/api/security/firewall", m.handleFirewall)
	mux.HandleFunc("/api/security/logs", m.handleLogs)
	mux.HandleFunc("/api/security/events", m.handleEvents)
	mux.HandleFunc("/api/security/vulnerabilities", m.handleVulnerabilities)
}

func (m *Manager) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type    ScanType `json:"type"`
		Targets []string `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scan, err := m.StartScan(r.Context(), req.Type, req.Targets)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, scan)
}

func (m *Manager) handleListScans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scans := m.ListScans()
	writeJSON(w, scans)
}

func (m *Manager) handleThreats(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m.mu.RLock()
		threats := make([]*Threat, 0, len(m.threats))
		for _, threat := range m.threats {
			threats = append(threats, threat)
		}
		m.mu.RUnlock()
		writeJSON(w, threats)
	case http.MethodPost:
		var req struct {
			ThreatID string `json:"threat_id"`
			Action   string `json:"action"` // quarantine, remove, whitelist
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var err error
		switch req.Action {
		case "quarantine":
			err = m.QuarantineThreat(req.ThreatID)
		case "remove":
			err = m.RemoveThreat(req.ThreatID)
		case "whitelist":
			err = m.WhitelistThreat(req.ThreatID)
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "success"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m.mu.RLock()
		policies := make([]*SecurityPolicy, 0, len(m.policies))
		for _, policy := range m.policies {
			policies = append(policies, policy)
		}
		m.mu.RUnlock()
		writeJSON(w, policies)
	case http.MethodPost:
		var policy SecurityPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateSecurityPolicy(&policy); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, policy)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleFirewall(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := m.ListFirewallRules()
		writeJSON(w, rules)
	case http.MethodPost:
		var rule FirewallRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddFirewallRule(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, rule)
	case http.MethodDelete:
		ruleID := r.URL.Query().Get("id")
		if ruleID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		if err := m.DeleteFirewallRule(ruleID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	logs := m.GetAccessLogs(limit)
	writeJSON(w, logs)
}

func (m *Manager) handleEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		events := m.GetSecurityEvents(limit)
		writeJSON(w, events)
	case http.MethodPost:
		var req struct {
			EventID    string `json:"event_id"`
			ResolvedBy string `json:"resolved_by"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.ResolveEvent(req.EventID, req.ResolvedBy); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "resolved"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleVulnerabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vulns := m.GetVulnerabilities()
	writeJSON(w, vulns)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
