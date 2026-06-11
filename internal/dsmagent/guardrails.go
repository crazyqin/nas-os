package dsmagent

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Guardrails 安全护栏，限制危险操作和资源使用
// 提供操作审批、资源限制、审计日志和角色访问控制
type Guardrails struct {
	mu            sync.RWMutex
	config        GuardrailsConfig
	auditLog      []*AuditEntry           // 审计日志
	approvals     map[string]*ApprovalRequest // 待审批请求
	rateLimiters  map[string]*RateLimiter  // 速率限制器
}

// GuardrailsConfig 安全护栏配置
type GuardrailsConfig struct {
	// 资源限制
	MaxCPUUsage    float64 `json:"max_cpu_usage"`     // CPU使用率上限（百分比）
	MaxMemoryUsage float64 `json:"max_memory_usage"`  // 内存使用率上限（百分比）
	MaxDiskUsage   float64 `json:"max_disk_usage"`    // 磁盘使用率上限（百分比）
	MaxNetworkIO   int64   `json:"max_network_io"`    // 网络IO上限（字节/秒）

	// 操作限制
	RequireApproval   bool     `json:"require_approval"`    // 危险操作是否需要审批
	BlockedOperations []string `json:"blocked_operations"`  // 禁止的操作列表
	DangerousPatterns []string `json:"dangerous_patterns"`  // 危险命令模式

	// 速率限制
	RateLimitWindow time.Duration `json:"rate_limit_window"` // 速率限制时间窗口
	MaxOpsPerWindow int           `json:"max_ops_per_window"` // 窗口内最大操作数

	// 审计设置
	AuditEnabled    bool `json:"audit_enabled"`    // 是否启用审计
	MaxAuditEntries int  `json:"max_audit_entries"` // 最大审计日志条数
}

// AuditEntry 审计日志条目
type AuditEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Actor     string                 `json:"actor"`      // 操作者
	Action    string                 `json:"action"`     // 操作类型
	Resource  string                 `json:"resource"`   // 操作对象
	Details   map[string]interface{} `json:"details,omitempty"`
	Allowed   bool                   `json:"allowed"`    // 是否被允许
	Reason    string                 `json:"reason"`     // 决策原因
}

// ApprovalRequest 审批请求
type ApprovalRequest struct {
	ID          string                 `json:"id"`
	RequestedBy string                 `json:"requested_by"`
	Action      string                 `json:"action"`
	Resource    string                 `json:"resource"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Reason      string                 `json:"reason"`
	Status      ApprovalStatus         `json:"status"`
	RequestedAt time.Time              `json:"requested_at"`
	ReviewedAt  *time.Time             `json:"reviewed_at,omitempty"`
	ReviewedBy  string                 `json:"reviewed_by,omitempty"`
	Comment     string                 `json:"comment,omitempty"`
}

// ApprovalStatus 审批状态
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalExpired  ApprovalStatus = "expired"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	window    time.Duration
	maxOps    int
	operation []time.Time
}

// NewGuardrails 创建安全护栏实例
func NewGuardrails(config GuardrailsConfig) *Guardrails {
	// 设置默认值
	if config.MaxCPUUsage <= 0 {
		config.MaxCPUUsage = 95.0
	}
	if config.MaxMemoryUsage <= 0 {
		config.MaxMemoryUsage = 95.0
	}
	if config.MaxDiskUsage <= 0 {
		config.MaxDiskUsage = 95.0
	}
	if config.RateLimitWindow <= 0 {
		config.RateLimitWindow = 1 * time.Minute
	}
	if config.MaxOpsPerWindow <= 0 {
		config.MaxOpsPerWindow = 100
	}
	if config.MaxAuditEntries <= 0 {
		config.MaxAuditEntries = 10000
	}

	// 初始化默认危险操作列表
	if len(config.BlockedOperations) == 0 {
		config.BlockedOperations = []string{
			"rm -rf /",
			"mkfs",
			"dd if=/dev/zero",
			"shutdown",
			"reboot",
			"halt",
		}
	}

	if len(config.DangerousPatterns) == 0 {
		config.DangerousPatterns = []string{
			"rm -rf",
			"chmod 777",
			"eval",
			"exec",
		}
	}

	guard := &Guardrails{
		config:       config,
		auditLog:     make([]*AuditEntry, 0),
		approvals:    make(map[string]*ApprovalRequest),
		rateLimiters: make(map[string]*RateLimiter),
	}

	log.Printf("[Guardrails] 安全护栏已初始化 (审计: %v, 审批: %v)", config.AuditEnabled, config.RequireApproval)
	return guard
}

// CheckOperation 检查操作是否被允许
func (g *Guardrails) CheckOperation(actor, action, resource string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 检查是否在禁止列表中
	for _, blocked := range g.config.BlockedOperations {
		if action == blocked {
			g.addAuditEntry(actor, action, resource, false, "操作被禁止")
			return fmt.Errorf("操作被安全护栏禁止: %s", action)
		}
	}

	// 检查危险模式
	for _, pattern := range g.config.DangerousPatterns {
		if containsPattern(action, pattern) {
			if g.config.RequireApproval {
				return fmt.Errorf("操作包含危险模式，需要审批: %s", pattern)
			}
			g.addAuditEntry(actor, action, resource, false, "匹配危险模式")
			return fmt.Errorf("操作匹配危险模式: %s", pattern)
		}
	}

	// 检查速率限制
	if !g.checkRateLimit(actor) {
		g.addAuditEntry(actor, action, resource, false, "超出速率限制")
		return fmt.Errorf("操作超出速率限制")
	}

	// 记录允许的操作
	g.addAuditEntry(actor, action, resource, true, "操作被允许")
	return nil
}

// CheckResourceLimits 检查资源使用是否超限
func (g *Guardrails) CheckResourceLimits(health *SystemHealth) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var warnings []string

	if health.CPUUsage > g.config.MaxCPUUsage {
		warnings = append(warnings, fmt.Sprintf("CPU使用率超限: %.1f%% > %.1f%%", health.CPUUsage, g.config.MaxCPUUsage))
	}
	if health.MemoryUsage > g.config.MaxMemoryUsage {
		warnings = append(warnings, fmt.Sprintf("内存使用率超限: %.1f%% > %.1f%%", health.MemoryUsage, g.config.MaxMemoryUsage))
	}
	if health.DiskUsage > g.config.MaxDiskUsage {
		warnings = append(warnings, fmt.Sprintf("磁盘使用率超限: %.1f%% > %.1f%%", health.DiskUsage, g.config.MaxDiskUsage))
	}

	return warnings
}

// CheckWorkflowExecution 检查工作流是否允许执行
func (g *Guardrails) CheckWorkflowExecution(tmpl *WorkflowTemplate) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 检查是否有被禁止的步骤
	for _, step := range tmpl.Steps {
		for _, blocked := range g.config.BlockedOperations {
			if step.Action == blocked {
				return fmt.Errorf("工作流包含被禁止的操作: %s (步骤: %s)", step.Action, step.Name)
			}
		}
	}

	return nil
}

// RequestApproval 请求操作审批
func (g *Guardrails) RequestApproval(requestedBy, action, resource, reason string) (*ApprovalRequest, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	request := &ApprovalRequest{
		ID:          fmt.Sprintf("approval_%d", time.Now().UnixNano()),
		RequestedBy: requestedBy,
		Action:      action,
		Resource:    resource,
		Reason:      reason,
		Status:      ApprovalPending,
		RequestedAt: time.Now(),
	}

	g.approvals[request.ID] = request
	log.Printf("[Guardrails] 收到审批请求: %s (操作: %s)", request.ID, action)

	return request, nil
}

// ApproveRequest 批准审批请求
func (g *Guardrails) ApproveRequest(requestID, reviewedBy, comment string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	request, exists := g.approvals[requestID]
	if !exists {
		return fmt.Errorf("审批请求不存在: %s", requestID)
	}

	if request.Status != ApprovalPending {
		return fmt.Errorf("审批请求状态异常: %s", request.Status)
	}

	now := time.Now()
	request.Status = ApprovalApproved
	request.ReviewedAt = &now
	request.ReviewedBy = reviewedBy
	request.Comment = comment

	g.addAuditEntry(reviewedBy, "approve", requestID, true, comment)
	log.Printf("[Guardrails] 审批请求已批准: %s (审批人: %s)", requestID, reviewedBy)

	return nil
}

// RejectRequest 拒绝审批请求
func (g *Guardrails) RejectRequest(requestID, reviewedBy, comment string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	request, exists := g.approvals[requestID]
	if !exists {
		return fmt.Errorf("审批请求不存在: %s", requestID)
	}

	if request.Status != ApprovalPending {
		return fmt.Errorf("审批请求状态异常: %s", request.Status)
	}

	now := time.Now()
	request.Status = ApprovalRejected
	request.ReviewedAt = &now
	request.ReviewedBy = reviewedBy
	request.Comment = comment

	g.addAuditEntry(reviewedBy, "reject", requestID, true, comment)
	log.Printf("[Guardrails] 审批请求已拒绝: %s (审批人: %s)", requestID, reviewedBy)

	return nil
}

// GetApprovalRequest 获取审批请求详情
func (g *Guardrails) GetApprovalRequest(requestID string) (*ApprovalRequest, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	request, exists := g.approvals[requestID]
	if !exists {
		return nil, fmt.Errorf("审批请求不存在: %s", requestID)
	}
	return request, nil
}

// ListPendingApprovals 列出待审批请求
func (g *Guardrails) ListPendingApprovals() []*ApprovalRequest {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var pending []*ApprovalRequest
	for _, request := range g.approvals {
		if request.Status == ApprovalPending {
			pending = append(pending, request)
		}
	}
	return pending
}

// GetAuditLog 获取审计日志
func (g *Guardrails) GetAuditLog(limit int) []*AuditEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limit <= 0 || limit > len(g.auditLog) {
		limit = len(g.auditLog)
	}

	// 返回最近的日志
	start := len(g.auditLog) - limit
	if start < 0 {
		start = 0
	}
	return g.auditLog[start:]
}

// addAuditEntry 添加审计日志条目
func (g *Guardrails) addAuditEntry(actor, action, resource string, allowed bool, reason string) {
	if !g.config.AuditEnabled {
		return
	}

	entry := &AuditEntry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Allowed:   allowed,
		Reason:    reason,
	}

	g.auditLog = append(g.auditLog, entry)

	// 超过最大条数时清理旧日志
	if len(g.auditLog) > g.config.MaxAuditEntries {
		g.auditLog = g.auditLog[len(g.auditLog)-g.config.MaxAuditEntries:]
	}
}

// checkRateLimit 检查速率限制
func (g *Guardrails) checkRateLimit(actor string) bool {
	limiter, exists := g.rateLimiters[actor]
	if !exists {
		limiter = &RateLimiter{
			window: g.config.RateLimitWindow,
			maxOps: g.config.MaxOpsPerWindow,
		}
		g.rateLimiters[actor] = limiter
	}

	now := time.Now()
	windowStart := now.Add(-limiter.window)

	// 清理过期的记录
	validOps := make([]time.Time, 0)
	for _, t := range limiter.operation {
		if t.After(windowStart) {
			validOps = append(validOps, t)
		}
	}
	limiter.operation = validOps

	// 检查是否超出限制
	if len(validOps) >= limiter.maxOps {
		return false
	}

	// 记录本次操作
	limiter.operation = append(limiter.operation, now)
	return true
}

// containsPattern 检查字符串是否包含指定模式
func containsPattern(s, pattern string) bool {
	return len(s) >= len(pattern) && (s == pattern || len(s) > 0 && len(pattern) > 0 && findSubstring(s, pattern))
}

// findSubstring 简单的子串查找
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetConfig 获取安全护栏配置
func (g *Guardrails) GetConfig() GuardrailsConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config
}

// UpdateConfig 更新安全护栏配置
func (g *Guardrails) UpdateConfig(config GuardrailsConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.config = config
	log.Printf("[Guardrails] 安全护栏配置已更新")
}
