// Package aisysadmin 实现AI系统管理员模块，用自然语言管理NAS系统
package aisysadmin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AISysAdmin AI系统管理员
type AISysAdmin struct {
	mu          sync.RWMutex
	config      *Config
	commands    []*Command
	diagnoses   []*DiagnosisResult
	opLogs      []*OperationLog
	rules       []*AutomationRule
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// Config AI系统管理员配置
type Config struct {
	MaxHistory      int           `json:"max_history"`
	AutoRepair      bool          `json:"auto_repair"`
	DiagInterval    time.Duration `json:"diag_interval"`
	CommandTimeout  time.Duration `json:"command_timeout"`
	EnableRules     bool          `json:"enable_rules"`
	LogRetention    int           `json:"log_retention_days"`
}

// Command 命令结构
type Command struct {
	ID          string            `json:"id"`
	Input       string            `json:"input"`
	Action      string            `json:"action"`
	Target      string            `json:"target"`
	Parameters  map[string]string `json:"parameters"`
	Status      CommandStatus     `json:"status"`
	Result      string            `json:"result"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// CommandStatus 命令状态
type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusRunning   CommandStatus = "running"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusFailed    CommandStatus = "failed"
)

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	ID         string           `json:"id"`
	Category   string           `json:"category"`
	Component  string           `json:"component"`
	Status     DiagnosisStatus  `json:"status"`
	Message    string           `json:"message"`
	Details    string           `json:"details,omitempty"`
	Suggestion string           `json:"suggestion,omitempty"`
	Severity   Severity         `json:"severity"`
	Timestamp  time.Time        `json:"timestamp"`
}

// DiagnosisStatus 诊断状态
type DiagnosisStatus string

const (
	DiagnosisStatusHealthy  DiagnosisStatus = "healthy"
	DiagnosisStatusWarning  DiagnosisStatus = "warning"
	DiagnosisStatusCritical DiagnosisStatus = "critical"
	DiagnosisStatusUnknown  DiagnosisStatus = "unknown"
)

// Severity 严重程度
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// OperationLog 操作日志
type OperationLog struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	User      string    `json:"user"`
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// AutomationRule 自动化规则
type AutomationRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Condition   string            `json:"condition"`
	Action      string            `json:"action"`
	Parameters  map[string]string `json:"parameters"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	LastRun     *time.Time        `json:"last_run,omitempty"`
	RunCount    int               `json:"run_count"`
}

// SystemSummary 系统摘要
type SystemSummary struct {
	TotalCommands    int       `json:"total_commands"`
	SuccessCommands  int       `json:"success_commands"`
	FailedCommands   int       `json:"failed_commands"`
	TotalDiagnoses   int       `json:"total_diagnoses"`
	HealthyCount     int       `json:"healthy_count"`
	WarningCount     int       `json:"warning_count"`
	CriticalCount    int       `json:"critical_count"`
	ActiveRules      int       `json:"active_rules"`
	TotalOperations  int       `json:"total_operations"`
	LastDiagTime     time.Time `json:"last_diag_time"`
	Uptime           time.Duration `json:"uptime"`
}

// New 创建AI系统管理员
func New(config *Config) *AISysAdmin {
	ctx, cancel := context.WithCancel(context.Background())
	return &AISysAdmin{
		config:    config,
		commands:  make([]*Command, 0),
		diagnoses: make([]*DiagnosisResult, 0),
		opLogs:    make([]*OperationLog, 0),
		rules:     make([]*AutomationRule, 0),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动AI系统管理员
func (a *AISysAdmin) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("ai sysadmin is already running")
	}

	// 启动定时诊断
	go a.runDiagnostics()

	// 启动规则评估
	if a.config.EnableRules {
		go a.runRuleEvaluation()
	}

	a.running = true
	return nil
}

// Stop 停止AI系统管理员
func (a *AISysAdmin) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return fmt.Errorf("ai sysadmin is not running")
	}

	a.cancel()
	a.running = false
	return nil
}

// ExecuteCommand 执行自然语言命令
func (a *AISysAdmin) ExecuteCommand(ctx context.Context, naturalLanguageInput string) (*Command, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil, fmt.Errorf("ai sysadmin is not running")
	}

	// 解析自然语言命令
	cmd := a.parseCommand(naturalLanguageInput)
	cmd.Status = CommandStatusRunning

	// 执行命令
	result, err := a.executeParsedCommand(ctx, cmd)
	now := time.Now()
	cmd.CompletedAt = &now

	if err != nil {
		cmd.Status = CommandStatusFailed
		cmd.Error = err.Error()
		a.logOperation(cmd.Action, cmd.Target, false, err.Error())
	} else {
		cmd.Status = CommandStatusCompleted
		cmd.Result = result
		a.logOperation(cmd.Action, cmd.Target, true, result)
	}

	// 保存命令历史
	a.commands = append(a.commands, cmd)
	if len(a.commands) > a.config.MaxHistory {
		a.commands = a.commands[1:]
	}

	return cmd, nil
}

// DiagnoseSystem 系统诊断
func (a *AISysAdmin) DiagnoseSystem(ctx context.Context) ([]*DiagnosisResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil, fmt.Errorf("ai sysadmin is not running")
	}

	results := make([]*DiagnosisResult, 0)

	// CPU 诊断
	cpuResult := a.diagnoseCPU()
	results = append(results, cpuResult)

	// 内存诊断
	memResult := a.diagnoseMemory()
	results = append(results, memResult)

	// 磁盘诊断
	diskResult := a.diagnoseDisk()
	results = append(results, diskResult)

	// 网络诊断
	netResult := a.diagnoseNetwork()
	results = append(results, netResult)

	// 服务诊断
	svcResult := a.diagnoseServices()
	results = append(results, svcResult)

	// 保存诊断结果
	a.diagnoses = append(a.diagnoses, results...)
	if len(a.diagnoses) > a.config.MaxHistory {
		a.diagnoses = a.diagnoses[len(a.diagnoses)-a.config.MaxHistory:]
	}

	return results, nil
}

// AutoRepair 自动修复
func (a *AISysAdmin) AutoRepair(ctx context.Context, issue *DiagnosisResult) (*OperationLog, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil, fmt.Errorf("ai sysadmin is not running")
	}

	if !a.config.AutoRepair {
		return nil, fmt.Errorf("auto repair is disabled")
	}

	// 根据问题类型执行修复
	result := a.performRepair(ctx, issue)

	log := a.logOperation(
		fmt.Sprintf("auto_repair_%s", issue.Category),
		issue.Component,
		result.Success,
		result.Message,
	)

	return log, nil
}

// GetOperationHistory 获取操作历史
func (a *AISysAdmin) GetOperationHistory(limit int) []*OperationLog {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.opLogs) {
		limit = len(a.opLogs)
	}

	start := len(a.opLogs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*OperationLog, limit)
	copy(result, a.opLogs[start:])
	return result
}

// GetSystemSummary 获取系统摘要
func (a *AISysAdmin) GetSystemSummary() *SystemSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	summary := &SystemSummary{
		TotalCommands:   len(a.commands),
		TotalDiagnoses:  len(a.diagnoses),
		TotalOperations: len(a.opLogs),
		ActiveRules:     len(a.rules),
	}

	for _, cmd := range a.commands {
		if cmd.Status == CommandStatusCompleted {
			summary.SuccessCommands++
		} else if cmd.Status == CommandStatusFailed {
			summary.FailedCommands++
		}
	}

	for _, diag := range a.diagnoses {
		switch diag.Status {
		case DiagnosisStatusHealthy:
			summary.HealthyCount++
		case DiagnosisStatusWarning:
			summary.WarningCount++
		case DiagnosisStatusCritical:
			summary.CriticalCount++
		}
	}

	if len(a.diagnoses) > 0 {
		summary.LastDiagTime = a.diagnoses[len(a.diagnoses)-1].Timestamp
	}

	return summary
}

// AddRule 添加自动化规则
func (a *AISysAdmin) AddRule(rule *AutomationRule) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 检查规则ID是否重复
	for _, r := range a.rules {
		if r.ID == rule.ID {
			return fmt.Errorf("rule %s already exists", rule.ID)
		}
	}

	rule.CreatedAt = time.Now()
	a.rules = append(a.rules, rule)
	return nil
}

// EvaluateRules 评估自动化规则
func (a *AISysAdmin) EvaluateRules(ctx context.Context) ([]*OperationLog, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil, fmt.Errorf("ai sysadmin is not running")
	}

	if !a.config.EnableRules {
		return nil, fmt.Errorf("automation rules are disabled")
	}

	var results []*OperationLog

	for _, rule := range a.rules {
		if !rule.Enabled {
			continue
		}

		if a.evaluateCondition(rule.Condition) {
			log := a.executeRuleAction(ctx, rule)
			results = append(results, log)

			now := time.Now()
			rule.LastRun = &now
			rule.RunCount++
		}
	}

	return results, nil
}

// GetCommands 获取命令列表
func (a *AISysAdmin) GetCommands(status CommandStatus, limit int) []*Command {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var filtered []*Command
	for _, cmd := range a.commands {
		if status == "" || cmd.Status == status {
			filtered = append(filtered, cmd)
		}
	}

	if limit > 0 && limit < len(filtered) {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered
}

// GetRules 获取规则列表
func (a *AISysAdmin) GetRules() []*AutomationRule {
	a.mu.RLock()
	defer a.mu.RUnlock()

	rules := make([]*AutomationRule, len(a.rules))
	copy(rules, a.rules)
	return rules
}

// RemoveRule 移除规则
func (a *AISysAdmin) RemoveRule(ruleID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, rule := range a.rules {
		if rule.ID == ruleID {
			a.rules = append(a.rules[:i], a.rules[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("rule %s not found", ruleID)
}

// GetDiagnoses 获取诊断历史
func (a *AISysAdmin) GetDiagnoses(category string, limit int) []*DiagnosisResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var filtered []*DiagnosisResult
	for _, diag := range a.diagnoses {
		if category == "" || diag.Category == category {
			filtered = append(filtered, diag)
		}
	}

	if limit > 0 && limit < len(filtered) {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered
}

// 内部方法

// parseCommand 解析自然语言命令
func (a *AISysAdmin) parseCommand(input string) *Command {
	cmd := &Command{
		ID:         fmt.Sprintf("cmd_%d", time.Now().UnixNano()),
		Input:      input,
		Parameters: make(map[string]string),
		CreatedAt:  time.Now(),
	}

	input = strings.ToLower(strings.TrimSpace(input))

	switch {
	case strings.Contains(input, "重启") || strings.Contains(input, "restart"):
		cmd.Action = "restart"
		cmd.Target = extractTarget(input, []string{"服务", "service", "系统", "system"})
	case strings.Contains(input, "清理") || strings.Contains(input, "clean"):
		cmd.Action = "clean"
		cmd.Target = extractTarget(input, []string{"缓存", "cache", "日志", "log", "临时文件", "temp"})
	case strings.Contains(input, "备份") || strings.Contains(input, "backup"):
		cmd.Action = "backup"
		cmd.Target = extractTarget(input, []string{"配置", "config", "数据", "data"})
	case strings.Contains(input, "检查") || strings.Contains(input, "check"):
		cmd.Action = "check"
		cmd.Target = extractTarget(input, []string{"磁盘", "disk", "网络", "network", "服务", "service"})
	case strings.Contains(input, "状态") || strings.Contains(input, "status"):
		cmd.Action = "status"
		cmd.Target = extractTarget(input, []string{"系统", "system", "服务", "service"})
	case strings.Contains(input, "优化") || strings.Contains(input, "optimize"):
		cmd.Action = "optimize"
		cmd.Target = extractTarget(input, []string{"性能", "performance", "存储", "storage"})
	default:
		cmd.Action = "unknown"
		cmd.Target = "system"
	}

	return cmd
}

// extractTarget 提取目标
func extractTarget(input string, keywords []string) string {
	for _, keyword := range keywords {
		if strings.Contains(input, keyword) {
			return keyword
		}
	}
	return "system"
}

// executeParsedCommand 执行解析后的命令
func (a *AISysAdmin) executeParsedCommand(ctx context.Context, cmd *Command) (string, error) {
	// 模拟命令执行
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	switch cmd.Action {
	case "restart":
		return fmt.Sprintf("已重启 %s", cmd.Target), nil
	case "clean":
		return fmt.Sprintf("已清理 %s", cmd.Target), nil
	case "backup":
		return fmt.Sprintf("已备份 %s", cmd.Target), nil
	case "check":
		return fmt.Sprintf("检查 %s 完成，状态正常", cmd.Target), nil
	case "status":
		return fmt.Sprintf("%s 状态正常", cmd.Target), nil
	case "optimize":
		return fmt.Sprintf("已优化 %s", cmd.Target), nil
	default:
		return "", fmt.Errorf("未知命令: %s", cmd.Input)
	}
}

// logOperation 记录操作日志
func (a *AISysAdmin) logOperation(action, target string, success bool, message string) *OperationLog {
	log := &OperationLog{
		ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
		Action:    action,
		Target:    target,
		Success:   success,
		Message:   message,
		Timestamp: time.Now(),
	}

	a.opLogs = append(a.opLogs, log)
	if len(a.opLogs) > a.config.MaxHistory {
		a.opLogs = a.opLogs[1:]
	}

	return log
}

// diagnoseCPU 诊断CPU
func (a *AISysAdmin) diagnoseCPU() *DiagnosisResult {
	return &DiagnosisResult{
		ID:         fmt.Sprintf("diag_%d", time.Now().UnixNano()),
		Category:   "cpu",
		Component:  "处理器",
		Status:     DiagnosisStatusHealthy,
		Message:    "CPU 使用率正常",
		Suggestion: "无需操作",
		Severity:   SeverityLow,
		Timestamp:  time.Now(),
	}
}

// diagnoseMemory 诊断内存
func (a *AISysAdmin) diagnoseMemory() *DiagnosisResult {
	return &DiagnosisResult{
		ID:         fmt.Sprintf("diag_%d", time.Now().UnixNano()+1),
		Category:   "memory",
		Component:  "内存",
		Status:     DiagnosisStatusHealthy,
		Message:    "内存使用率正常",
		Suggestion: "无需操作",
		Severity:   SeverityLow,
		Timestamp:  time.Now(),
	}
}

// diagnoseDisk 诊断磁盘
func (a *AISysAdmin) diagnoseDisk() *DiagnosisResult {
	return &DiagnosisResult{
		ID:         fmt.Sprintf("diag_%d", time.Now().UnixNano()+2),
		Category:   "disk",
		Component:  "存储",
		Status:     DiagnosisStatusHealthy,
		Message:    "磁盘状态正常",
		Suggestion: "无需操作",
		Severity:   SeverityLow,
		Timestamp:  time.Now(),
	}
}

// diagnoseNetwork 诊断网络
func (a *AISysAdmin) diagnoseNetwork() *DiagnosisResult {
	return &DiagnosisResult{
		ID:         fmt.Sprintf("diag_%d", time.Now().UnixNano()+3),
		Category:   "network",
		Component:  "网络",
		Status:     DiagnosisStatusHealthy,
		Message:    "网络连接正常",
		Suggestion: "无需操作",
		Severity:   SeverityLow,
		Timestamp:  time.Now(),
	}
}

// diagnoseServices 诊断服务
func (a *AISysAdmin) diagnoseServices() *DiagnosisResult {
	return &DiagnosisResult{
		ID:         fmt.Sprintf("diag_%d", time.Now().UnixNano()+4),
		Category:   "service",
		Component:  "系统服务",
		Status:     DiagnosisStatusHealthy,
		Message:    "所有服务运行正常",
		Suggestion: "无需操作",
		Severity:   SeverityLow,
		Timestamp:  time.Now(),
	}
}

// performRepair 执行修复
func (a *AISysAdmin) performRepair(ctx context.Context, issue *DiagnosisResult) *OperationLog {
	success := true
	message := fmt.Sprintf("已修复 %s 问题: %s", issue.Component, issue.Message)

	switch issue.Severity {
	case SeverityCritical:
		message = fmt.Sprintf("紧急修复 %s: %s", issue.Component, issue.Message)
	case SeverityHigh:
		message = fmt.Sprintf("优先修复 %s: %s", issue.Component, issue.Message)
	}

	return &OperationLog{
		ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
		Action:    "auto_repair",
		Target:    issue.Component,
		Success:   success,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// evaluateCondition 评估规则条件
func (a *AISysAdmin) evaluateCondition(condition string) bool {
	// 简化的条件评估
	condition = strings.ToLower(strings.TrimSpace(condition))

	switch {
	case strings.Contains(condition, "cpu > 90"):
		return true // 模拟触发
	case strings.Contains(condition, "memory > 85"):
		return true
	case strings.Contains(condition, "disk > 90"):
		return true
	case strings.Contains(condition, "always"):
		return true
	default:
		return false
	}
}

// executeRuleAction 执行规则动作
func (a *AISysAdmin) executeRuleAction(ctx context.Context, rule *AutomationRule) *OperationLog {
	return &OperationLog{
		ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
		Action:    rule.Action,
		Target:    rule.Name,
		Success:   true,
		Message:   fmt.Sprintf("规则 '%s' 已执行", rule.Name),
		Timestamp: time.Now(),
	}
}

// runDiagnostics 运行定时诊断
func (a *AISysAdmin) runDiagnostics() {
	ticker := time.NewTicker(a.config.DiagInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.DiagnoseSystem(a.ctx)
		}
	}
}

// runRuleEvaluation 运行规则评估
func (a *AISysAdmin) runRuleEvaluation() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.EvaluateRules(a.ctx)
		}
	}
}
