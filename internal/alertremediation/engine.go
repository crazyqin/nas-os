package alertremediation

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RemediationEngine 引导式告警修复引擎.
// 负责注册修复规则、分析告警、生成修复方案并执行修复操作.
type RemediationEngine struct {
	rules   map[string]*RemediationRule
	plans   map[string]*RemediationPlan
	running map[string]bool // 标记正在执行的动作
	logger  *zap.Logger
	mu      sync.RWMutex
}

// NewEngine 创建修复引擎.
func NewEngine(logger *zap.Logger) *RemediationEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	e := &RemediationEngine{
		rules:   make(map[string]*RemediationRule),
		plans:   make(map[string]*RemediationPlan),
		running: make(map[string]bool),
		logger:  logger,
	}
	e.registerBuiltinRules()
	return e
}

// RegisterRule 注册告警修复规则.
func (e *RemediationEngine) RegisterRule(rule *RemediationRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules[rule.ID] = rule
	e.logger.Info("registered remediation rule",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name),
	)
}

// UnregisterRule 注销告警修复规则.
func (e *RemediationEngine) UnregisterRule(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.rules, id)
}

// GetRule 获取已注册的规则.
func (e *RemediationEngine) GetRule(id string) (*RemediationRule, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.rules[id]
	return r, ok
}

// ListRules 列出所有已注册规则.
func (e *RemediationEngine) ListRules() []*RemediationRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rules := make([]*RemediationRule, 0, len(e.rules))
	for _, r := range e.rules {
		rules = append(rules, r)
	}
	return rules
}

// Analyze 分析告警并返回修复方案.
// 遍历已注册规则，匹配后生成包含排查步骤、修复动作和根因分析的修复方案.
func (e *RemediationEngine) Analyze(alert *Alert) *RemediationPlan {
	e.mu.RLock()
	// 收集匹配的规则
	var matched []*RemediationRule
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		if rule.MatchFunc != nil && rule.MatchFunc(alert) {
			matched = append(matched, rule)
		}
	}
	e.mu.RUnlock()

	if len(matched) == 0 {
		// 无匹配规则，生成通用方案
		return e.buildGenericPlan(alert)
	}

	// 选择优先级最高的规则
	best := matched[0]
	for _, r := range matched[1:] {
		if r.Priority < best.Priority {
			best = r
		}
	}

	return e.buildPlan(alert, best)
}

// GetPlan 获取已生成的修复方案.
func (e *RemediationEngine) GetPlan(id string) (*RemediationPlan, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.plans[id]
	return p, ok
}

// ListPlans 列出所有修复方案.
func (e *RemediationEngine) ListPlans() []*RemediationPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	plans := make([]*RemediationPlan, 0, len(e.plans))
	for _, p := range e.plans {
		plans = append(plans, p)
	}
	return plans
}

// ExecuteAction 执行修复方案中的指定动作.
func (e *RemediationEngine) ExecuteAction(ctx context.Context, planID, actionID string) *ActionResult {
	e.mu.RLock()
	plan, ok := e.plans[planID]
	if !ok {
		e.mu.RUnlock()
		return &ActionResult{
			ActionID:  actionID,
			Success:   false,
			Error:     fmt.Sprintf("remediation plan %q not found", planID),
			Timestamp: time.Now(),
		}
	}

	var action *RemediationAction
	for i := range plan.Actions {
		if plan.Actions[i].ID == actionID {
			action = &plan.Actions[i]
			break
		}
	}
	if action == nil {
		e.mu.RUnlock()
		return &ActionResult{
			ActionID:  actionID,
			Success:   false,
			Error:     fmt.Sprintf("action %q not found in plan %q", actionID, planID),
			Timestamp: time.Now(),
		}
	}
	e.mu.RUnlock()

	// 防止并发执行同一动作
	key := planID + ":" + actionID
	e.mu.Lock()
	if e.running[key] {
		e.mu.Unlock()
		return &ActionResult{
			ActionID:  actionID,
			Success:   false,
			Error:     "action is already running",
			Timestamp: time.Now(),
		}
	}
	e.running[key] = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.running, key)
		e.mu.Unlock()
	}()

	start := time.Now()
	result := e.executeAction(ctx, action)
	result.Duration = time.Since(start)

	// 更新动作状态
	e.mu.Lock()
	now := time.Now()
	action.ExecutedAt = &now
	if result.Success {
		action.Status = StatusCompleted
	} else {
		action.Status = StatusFailed
	}
	action.Result = result.Message

	// 检查是否所有动作都已完成
	allDone := true
	for _, a := range plan.Actions {
		if a.Status != StatusCompleted && a.Status != StatusFailed && a.Status != StatusSkipped {
			allDone = false
			break
		}
	}
	if allDone {
		plan.Status = StatusCompleted
		nowStr := time.Now()
		plan.CompletedAt = &nowStr
	}
	plan.UpdatedAt = time.Now()
	e.mu.Unlock()

	return result
}

// CompleteStep 标记排查步骤为已完成.
func (e *RemediationEngine) CompleteStep(planID string, stepOrder int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan, ok := e.plans[planID]
	if !ok {
		return fmt.Errorf("plan %q not found", planID)
	}

	for i := range plan.Steps {
		if plan.Steps[i].Order == stepOrder {
			plan.Steps[i].Completed = true
			plan.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("step %d not found in plan %q", stepOrder, planID)
}

// RemovePlan 删除修复方案.
func (e *RemediationEngine) RemovePlan(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.plans, id)
}

// buildPlan 根据告警和规则构建修复方案.
func (e *RemediationEngine) buildPlan(alert *Alert, rule *RemediationRule) *RemediationPlan {
	planID := uuid.New().String()

	// 构建排查步骤
	steps := make([]RemediationStep, 0, len(rule.Steps))
	for _, tmpl := range rule.Steps {
		steps = append(steps, RemediationStep{
			Order:       tmpl.Order,
			Title:       tmpl.Title,
			Description: tmpl.Description,
			Command:     tmpl.Command,
			Completed:   false,
		})
	}

	// 构建修复动作
	actions := make([]RemediationAction, 0, len(rule.Actions))
	for _, tmpl := range rule.Actions {
		actions = append(actions, RemediationAction{
			ID:              tmpl.ID,
			Name:            tmpl.Name,
			Description:     tmpl.Description,
			Type:            tmpl.Type,
			Command:         tmpl.Command,
			Parameters:      tmpl.Parameters,
			Destructive:     tmpl.Destructive,
			RequiresConfirm: tmpl.RequiresConfirm,
			Status:          StatusPending,
		})
	}

	plan := &RemediationPlan{
		ID:      planID,
		AlertID: alert.ID,
		RuleID:  rule.ID,
		Alert:   alert,
		RootCause: RootCauseAnalysis{
			Summary:        rule.RootCause,
			Description:    fmt.Sprintf("基于规则 %q 的根因分析: %s", rule.Name, rule.RootCause),
			PossibleCauses: []string{rule.RootCause},
			Impact:         fmt.Sprintf("影响类别: %s, 严重级别: %s", rule.Category, rule.Severity),
		},
		Steps:     steps,
		Actions:   actions,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	e.mu.Lock()
	e.plans[planID] = plan
	e.mu.Unlock()

	e.logger.Info("remediation plan created",
		zap.String("plan_id", planID),
		zap.String("alert_id", alert.ID),
		zap.String("rule_id", rule.ID),
	)

	return plan
}

// buildGenericPlan 当无匹配规则时生成通用方案.
func (e *RemediationEngine) buildGenericPlan(alert *Alert) *RemediationPlan {
	planID := uuid.New().String()

	plan := &RemediationPlan{
		ID:      planID,
		AlertID: alert.ID,
		RuleID:  "",
		Alert:   alert,
		RootCause: RootCauseAnalysis{
			Summary:     "未找到匹配的根因规则",
			Description: fmt.Sprintf("告警 [%s] %s 未匹配到已注册的修复规则，建议人工排查。", alert.Severity, alert.Title),
			PossibleCauses: []string{
				"未知的告警触发条件",
				"可能是新引入的告警源",
				"需要人工确认具体原因",
			},
			Impact: "影响范围需人工评估",
		},
		Steps: []RemediationStep{
			{
				Order:       1,
				Title:       "查看告警详情",
				Description: fmt.Sprintf("告警消息: %s\n来源: %s", alert.Message, alert.Source),
				Completed:   false,
			},
			{
				Order:       2,
				Title:       "检查系统日志",
				Description: "查看相关系统日志获取更多上下文信息",
				Command:     fmt.Sprintf("journalctl -u '%s' --since '-1h' --no-pager", alert.Source),
				Completed:   false,
			},
			{
				Order:       3,
				Title:       "联系管理员",
				Description: "如无法自行解决，请联系系统管理员并提供告警ID: " + alert.ID,
				Completed:   false,
			},
		},
		Actions:   []RemediationAction{},
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	e.mu.Lock()
	e.plans[planID] = plan
	e.mu.Unlock()

	return plan
}

// executeAction 执行具体的修复动作.
func (e *RemediationEngine) executeAction(ctx context.Context, action *RemediationAction) *ActionResult {
	result := &ActionResult{
		ActionID:  action.ID,
		Timestamp: time.Now(),
	}

	e.logger.Info("executing remediation action",
		zap.String("action_id", action.ID),
		zap.String("type", string(action.Type)),
		zap.String("name", action.Name),
	)

	switch action.Type {
	case ActionServiceRestart:
		return e.execServiceRestart(ctx, action, result)
	case ActionDiskCleanup:
		return e.execDiskCleanup(ctx, action, result)
	case ActionCommand:
		return e.execCommand(ctx, action, result)
	case ActionScript:
		return e.execCommand(ctx, action, result)
	case ActionZFSRepair:
		return e.execCommand(ctx, action, result)
	case ActionNetworkReset:
		return e.execCommand(ctx, action, result)
	case ActionLogRotation:
		return e.execLogRotation(ctx, action, result)
	case ActionConfigChange:
		return e.execConfigChange(ctx, action, result)
	case ActionNotifyUser:
		result.Success = true
		result.Message = action.Description
		return result
	default:
		result.Error = fmt.Sprintf("unsupported action type: %s", action.Type)
		return result
	}
}

// execServiceRestart 重启服务.
func (e *RemediationEngine) execServiceRestart(ctx context.Context, action *RemediationAction, result *ActionResult) *ActionResult {
	serviceName := action.Parameters["service"]
	if serviceName == "" {
		result.Error = "missing 'service' parameter"
		return result
	}

	cmd := exec.CommandContext(ctx, "systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	if err != nil {
		result.Error = err.Error()
		result.Message = fmt.Sprintf("重启服务 %s 失败: %s", serviceName, err)
		return result
	}
	result.Success = true
	result.Message = fmt.Sprintf("服务 %s 已成功重启", serviceName)
	return result
}

// execDiskCleanup 磁盘清理.
func (e *RemediationEngine) execDiskCleanup(ctx context.Context, action *RemediationAction, result *ActionResult) *ActionResult {
	target := action.Parameters["target"]
	if target == "" {
		target = "/tmp"
	}

	var outputs []string
	// 清理 tmp
	cmd := exec.CommandContext(ctx, "find", target, "-type", "f", "-mtime", "+7", "-delete")
	out, err := cmd.CombinedOutput()
	outputs = append(outputs, string(out))
	if err != nil {
		// 非致命错误，记录后继续
		e.logger.Warn("disk cleanup partial failure", zap.String("target", target), zap.Error(err))
	}

	// 清理 apt 缓存（如果存在）
	cmd2 := exec.CommandContext(ctx, "apt-get", "clean")
	out2, _ := cmd2.CombinedOutput()
	outputs = append(outputs, string(out2))

	result.Output = strings.Join(outputs, "\n")
	result.Success = true
	result.Message = fmt.Sprintf("磁盘清理完成，目标目录: %s", target)
	return result
}

// execCommand 通用命令执行.
func (e *RemediationEngine) execCommand(ctx context.Context, action *RemediationAction, result *ActionResult) *ActionResult {
	command := action.Command
	if command == "" {
		result.Error = "no command specified"
		return result
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	if err != nil {
		result.Error = err.Error()
		result.Message = fmt.Sprintf("命令执行失败: %s", err)
		return result
	}
	result.Success = true
	result.Message = "命令执行成功"
	return result
}

// execLogRotation 日志轮转.
func (e *RemediationEngine) execLogRotation(ctx context.Context, action *RemediationAction, result *ActionResult) *ActionResult {
	logPath := action.Parameters["log_path"]
	if logPath == "" {
		logPath = "/var/log/syslog"
	}

	// 执行 logrotate
	cmd := exec.CommandContext(ctx, "logrotate", "--force", "/etc/logrotate.d/rsyslog")
	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	if err != nil {
		result.Error = err.Error()
		result.Message = fmt.Sprintf("日志轮转失败: %s", err)
		return result
	}
	result.Success = true
	result.Message = fmt.Sprintf("日志轮转完成: %s", logPath)
	return result
}

// execConfigChange 配置变更.
func (e *RemediationEngine) execConfigChange(ctx context.Context, action *RemediationAction, result *ActionResult) *ActionResult {
	// 配置变更需要具体的脚本命令
	command := action.Command
	if command == "" {
		result.Error = "no config change command specified"
		return result
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	if err != nil {
		result.Error = err.Error()
		result.Message = fmt.Sprintf("配置变更失败: %s", err)
		return result
	}
	result.Success = true
	result.Message = "配置变更完成"
	return result
}

// registerBuiltinRules 注册内置告警修复规则.
func (e *RemediationEngine) registerBuiltinRules() {
	// 磁盘空间不足
	e.rules["disk-space-critical"] = &RemediationRule{
		ID:          "disk-space-critical",
		Name:        "磁盘空间不足",
		Description: "存储池或系统盘空间使用率超过阈值",
		Category:    CatStorage,
		Severity:    SeverityCritical,
		MatchFunc: func(alert *Alert) bool {
			title := strings.ToLower(alert.Title)
			msg := strings.ToLower(alert.Message)
			return (strings.Contains(title, "disk") || strings.Contains(title, "磁盘") ||
				strings.Contains(title, "space") || strings.Contains(title, "空间")) &&
				(strings.Contains(msg, "full") || strings.Contains(msg, "满") ||
					strings.Contains(msg, "usage") || strings.Contains(msg, "使用率"))
		},
		RootCause: "存储空间使用率过高，可能是日志堆积、临时文件过多或数据增长导致",
		Steps: []StepTemplate{
			{Order: 1, Title: "查看磁盘使用情况", Description: "确认各分区的空间占用", Command: "df -h"},
			{Order: 2, Title: "定位大文件/目录", Description: "查找占用空间最多的目录", Command: "du -sh /* 2>/dev/null | sort -rh | head -20"},
			{Order: 3, Title: "检查日志文件", Description: "检查是否有过大的日志文件", Command: "find /var/log -type f -size +100M -exec ls -lh {} \\;"},
			{Order: 4, Title: "检查临时文件", Description: "清理 /tmp 和其他临时目录", Command: "du -sh /tmp /var/tmp"},
		},
		Actions: []ActionTemplate{
			{
				ID:          "cleanup-tmp",
				Name:        "清理临时文件",
				Description: "删除 7 天前的临时文件",
				Type:        ActionDiskCleanup,
				Parameters:  map[string]string{"target": "/tmp"},
				Destructive: false,
			},
			{
				ID:              "log-rotation",
				Name:            "强制日志轮转",
				Description:     "强制执行日志轮转，压缩旧日志",
				Type:            ActionLogRotation,
				Parameters:      map[string]string{"log_path": "/var/log"},
				Destructive:     false,
				RequiresConfirm: false,
			},
		},
		Enabled:  true,
		Priority: 10,
	}

	// 内存 OOM
	e.rules["memory-oom"] = &RemediationRule{
		ID:          "memory-oom",
		Name:        "内存不足 (OOM)",
		Description: "系统内存不足，可能发生 OOM Kill",
		Category:    CatSystem,
		Severity:    SeverityCritical,
		MatchFunc: func(alert *Alert) bool {
			title := strings.ToLower(alert.Title)
			msg := strings.ToLower(alert.Message)
			return strings.Contains(title, "oom") || strings.Contains(title, "memory") ||
				strings.Contains(title, "内存") ||
				strings.Contains(msg, "out of memory") || strings.Contains(msg, "oom-kill")
		},
		RootCause: "系统物理内存不足，可能是某进程内存泄漏或工作负载超出内存容量",
		Steps: []StepTemplate{
			{Order: 1, Title: "查看内存使用情况", Description: "确认当前内存和交换分区状态", Command: "free -h"},
			{Order: 2, Title: "定位高内存进程", Description: "查找占用内存最多的进程", Command: "ps aux --sort=-%mem | head -20"},
			{Order: 3, Title: "检查 OOM 日志", Description: "查看内核 OOM 日志", Command: "dmesg | grep -i 'oom\\|out of memory' | tail -20"},
		},
		Actions: []ActionTemplate{
			{
				ID:              "restart-heavy-service",
				Name:            "重启高内存服务",
				Description:     "重启内存占用过高的服务进程",
				Type:            ActionServiceRestart,
				Parameters:      map[string]string{"service": ""},
				Destructive:     false,
				RequiresConfirm: true,
			},
		},
		Enabled:  true,
		Priority: 20,
	}

	// 网络不通
	e.rules["network-unreachable"] = &RemediationRule{
		ID:          "network-unreachable",
		Name:        "网络连接异常",
		Description: "网络连接断开或无法到达目标",
		Category:    CatNetwork,
		Severity:    SeverityWarning,
		MatchFunc: func(alert *Alert) bool {
			title := strings.ToLower(alert.Title)
			msg := strings.ToLower(alert.Message)
			return strings.Contains(title, "network") || strings.Contains(title, "网络") ||
				strings.Contains(title, "connect") || strings.Contains(title, "连接") ||
				strings.Contains(msg, "unreachable") || strings.Contains(msg, "timeout") ||
				strings.Contains(msg, "unreachable")
		},
		RootCause: "网络接口故障、DNS 解析失败、网关不可达或物理链路中断",
		Steps: []StepTemplate{
			{Order: 1, Title: "检查网络接口状态", Description: "确认网卡是否正常工作", Command: "ip addr show"},
			{Order: 2, Title: "测试本地连接", Description: "测试 loopback 和局域网连通性", Command: "ping -c 3 127.0.0.1 && ping -c 3 $(ip route | grep default | awk '{print $3}')"},
			{Order: 3, Title: "检查 DNS 解析", Description: "确认 DNS 是否正常工作", Command: "nslookup google.com 2>&1 || dig google.com"},
			{Order: 4, Title: "检查防火墙规则", Description: "确认防火墙没有阻止连接", Command: "iptables -L -n 2>/dev/null || nft list ruleset 2>/dev/null"},
		},
		Actions: []ActionTemplate{
			{
				ID:              "restart-network",
				Name:            "重启网络服务",
				Description:     "重启系统网络管理服务",
				Type:            ActionNetworkReset,
				Command:         "systemctl restart systemd-networkd || systemctl restart NetworkManager",
				Destructive:     false,
				RequiresConfirm: true,
			},
		},
		Enabled:  true,
		Priority: 30,
	}

	// 服务宕机
	e.rules["service-down"] = &RemediationRule{
		ID:          "service-down",
		Name:        "服务宕机",
		Description: "关键系统服务停止运行",
		Category:    CatService,
		Severity:    SeverityCritical,
		MatchFunc: func(alert *Alert) bool {
			title := strings.ToLower(alert.Title)
			msg := strings.ToLower(alert.Message)
			return (strings.Contains(title, "service") || strings.Contains(title, "服务")) &&
				(strings.Contains(msg, "down") || strings.Contains(msg, "stopped") ||
					strings.Contains(msg, "inactive") || strings.Contains(msg, "宕机") ||
					strings.Contains(msg, "failed"))
		},
		RootCause: "服务进程崩溃、被意外终止或配置错误导致启动失败",
		Steps: []StepTemplate{
			{Order: 1, Title: "检查服务状态", Description: "确认服务当前的运行状态", Command: "systemctl status {{service}}"},
			{Order: 2, Title: "查看服务日志", Description: "检查服务的错误日志", Command: "journalctl -u {{service}} --since '-1h' --no-pager"},
			{Order: 3, Title: "检查配置文件", Description: "验证服务配置是否正确", Command: "{{service}} -t 2>/dev/null || echo 'no config test available'"},
		},
		Actions: []ActionTemplate{
			{
				ID:              "restart-service",
				Name:            "重启服务",
				Description:     "尝试重启已停止的服务",
				Type:            ActionServiceRestart,
				Parameters:      map[string]string{"service": ""},
				Destructive:     false,
				RequiresConfirm: false,
			},
		},
		Enabled:  true,
		Priority: 15,
	}

	// ZFS 错误
	e.rules["zfs-error"] = &RemediationRule{
		ID:          "zfs-error",
		Name:        "ZFS 存储错误",
		Description: "ZFS 存储池或数据集出现错误",
		Category:    CatStorage,
		Severity:    SeverityCritical,
		MatchFunc: func(alert *Alert) bool {
			title := strings.ToLower(alert.Title)
			msg := strings.ToLower(alert.Message)
			return strings.Contains(title, "zfs") || strings.Contains(title, "pool") ||
				strings.Contains(title, "存储池") ||
				strings.Contains(msg, "zfs") || strings.Contains(msg, "checksum") ||
				strings.Contains(msg, "scrub") || strings.Contains(msg, "degraded") ||
				strings.Contains(msg, "faulted")
		},
		RootCause: "ZFS 存储池出现数据完整性问题、磁盘故障或配置异常",
		Steps: []StepTemplate{
			{Order: 1, Title: "查看存储池状态", Description: "检查 ZFS 存储池健康状态", Command: "zpool status"},
			{Order: 2, Title: "查看存储池错误", Description: "检查存储池错误计数", Command: "zpool status -x"},
			{Order: 3, Title: "检查数据集", Description: "列出所有数据集及使用情况", Command: "zfs list"},
			{Order: 4, Title: "查看最近事件", Description: "检查 ZFS 最近的事件日志", Command: "zpool events -f | tail -20"},
		},
		Actions: []ActionTemplate{
			{
				ID:              "zfs-scrub",
				Name:            "启动 ZFS Scrub",
				Description:     "启动存储池数据完整性校验",
				Type:            ActionZFSRepair,
				Command:         "zpool scrub $(zpool list -Ho name | head -1)",
				Destructive:     false,
				RequiresConfirm: true,
			},
			{
				ID:              "zfs-clear",
				Name:            "清除错误计数",
				Description:     "清除存储池的错误计数器",
				Type:            ActionZFSRepair,
				Command:         "zpool clear $(zpool list -Ho name | head -1)",
				Destructive:     false,
				RequiresConfirm: true,
			},
		},
		Enabled:  true,
		Priority: 5,
	}

	// SMART 告警
	e.rules["smart-alert"] = &RemediationRule{
		ID:          "smart-alert",
		Name:        "磁盘 SMART 告警",
		Description: "磁盘 SMART 健康检测发现潜在故障",
		Category:    CatStorage,
		Severity:    SeverityWarning,
		MatchFunc: func(alert *Alert) bool {
			title := strings.ToLower(alert.Title)
			msg := strings.ToLower(alert.Message)
			return strings.Contains(title, "smart") ||
				strings.Contains(title, "磁盘健康") ||
				strings.Contains(msg, "smart") || strings.Contains(msg, "reallocated") ||
				strings.Contains(msg, "pending") || strings.Contains(msg, "uncorrectable") ||
				strings.Contains(msg, "predict")
		},
		RootCause: "磁盘出现坏道、重分配扇区增加或健康指标下降，可能即将发生物理故障",
		Steps: []StepTemplate{
			{Order: 1, Title: "查看磁盘 SMART 信息", Description: "获取磁盘详细 SMART 数据", Command: "smartctl -a /dev/sda"},
			{Order: 2, Title: "检查健康评估", Description: "查看 SMART 健康评估结果", Command: "smartctl -H /dev/sda"},
			{Order: 3, Title: "查看错误日志", Description: "检查磁盘错误日志", Command: "smartctl -l error /dev/sda"},
		},
		Actions: []ActionTemplate{
			{
				ID:              "smart-selftest",
				Name:            "启动磁盘自检",
				Description:     "启动磁盘短时自检（Short Self-Test）",
				Type:            ActionCommand,
				Command:         "smartctl -t short /dev/sda",
				Destructive:     false,
				RequiresConfirm: false,
			},
			{
				ID:              "notify-disk-failure",
				Name:            "通知磁盘故障风险",
				Description:     "发送磁盘故障预警通知，建议备份数据并更换磁盘",
				Type:            ActionNotifyUser,
				Destructive:     false,
				RequiresConfirm: false,
			},
		},
		Enabled:  true,
		Priority: 25,
	}

	e.logger.Info("builtin remediation rules registered", zap.Int("count", len(e.rules)))
}
