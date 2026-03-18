// Package rbac 提供审计日志集成
// 记录权限相关操作
package rbac

import (
	"time"
)

// AuditLogger 审计日志记录器接口
type AuditLogger interface {
	LogPermissionGrant(userID, username, targetUserID, permission string)
	LogPermissionRevoke(userID, username, targetUserID, permission string)
	LogRoleChange(userID, username, targetUserID, oldRole, newRole string)
	LogPolicyCreate(userID, username, policyName string)
	LogPolicyUpdate(userID, username, policyID string, changes map[string]interface{})
	LogPolicyDelete(userID, username, policyID string)
	LogAccessCheck(userID, resource, action string, allowed bool, reason string)
	LogGroupPermissionChange(userID, username, groupID string, permissions []string)
	LogShareACLChange(userID, username, shareName string, changes map[string]interface{})
}

// AuditLevel 审计级别
type AuditLevel string

const (
	AuditLevelInfo    AuditLevel = "info"
	AuditLevelWarning AuditLevel = "warning"
	AuditLevelError   AuditLevel = "error"
)

// AuditCategory 审计分类
type AuditCategory string

const (
	AuditCategoryPermission AuditCategory = "permission"
	AuditCategoryRole       AuditCategory = "role"
	AuditCategoryPolicy     AuditCategory = "policy"
	AuditCategoryAccess     AuditCategory = "access"
	AuditCategoryGroup      AuditCategory = "group"
	AuditCategoryShare      AuditCategory = "share"
)

// AuditEvent 审计事件
type AuditEvent struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      AuditLevel             `json:"level"`
	Category   AuditCategory          `json:"category"`
	Event      string                 `json:"event"`
	UserID     string                 `json:"user_id,omitempty"`
	Username   string                 `json:"username,omitempty"`
	TargetID   string                 `json:"target_id,omitempty"`
	TargetName string                 `json:"target_name,omitempty"`
	Resource   string                 `json:"resource,omitempty"`
	Action     string                 `json:"action,omitempty"`
	Permission string                 `json:"permission,omitempty"`
	OldValue   interface{}            `json:"old_value,omitempty"`
	NewValue   interface{}            `json:"new_value,omitempty"`
	Result     string                 `json:"result,omitempty"` // success, failure
	Reason     string                 `json:"reason,omitempty"`
	IP         string                 `json:"ip,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// RBACAuditLogger RBAC 审计日志记录器
type RBACAuditLogger struct {
	logChan chan *AuditEvent
	handler func(event *AuditEvent)
}

// NewRBACAuditLogger 创建 RBAC 审计日志记录器
func NewRBACAuditLogger(bufferSize int, handler func(event *AuditEvent)) *RBACAuditLogger {
	logger := &RBACAuditLogger{
		logChan: make(chan *AuditEvent, bufferSize),
		handler: handler,
	}

	// 启动处理协程
	go logger.processLoop()

	return logger
}

func (l *RBACAuditLogger) processLoop() {
	for event := range l.logChan {
		if l.handler != nil {
			l.handler(event)
		}
	}
}

// Log 异步记录审计事件
func (l *RBACAuditLogger) Log(event *AuditEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case l.logChan <- event:
	default:
		// 缓冲区满，丢弃事件（防止阻塞）
	}
}

// LogSync 同步记录审计事件
func (l *RBACAuditLogger) LogSync(event *AuditEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if l.handler != nil {
		l.handler(event)
	}
}

// Close 关闭日志记录器
func (l *RBACAuditLogger) Close() {
	close(l.logChan)
}

// ========== 审计方法实现 ==========

// LogPermissionGrant 记录权限授予
func (l *RBACAuditLogger) LogPermissionGrant(operatorID, operatorName, targetUserID, permission string) {
	l.Log(&AuditEvent{
		Timestamp:  time.Now(),
		Level:      AuditLevelInfo,
		Category:   AuditCategoryPermission,
		Event:      "permission_grant",
		UserID:     operatorID,
		Username:   operatorName,
		TargetID:   targetUserID,
		Permission: permission,
		Result:     "success",
	})
}

// LogPermissionRevoke 记录权限撤销
func (l *RBACAuditLogger) LogPermissionRevoke(operatorID, operatorName, targetUserID, permission string) {
	l.Log(&AuditEvent{
		Timestamp:  time.Now(),
		Level:      AuditLevelInfo,
		Category:   AuditCategoryPermission,
		Event:      "permission_revoke",
		UserID:     operatorID,
		Username:   operatorName,
		TargetID:   targetUserID,
		Permission: permission,
		Result:     "success",
	})
}

// LogRoleChange 记录角色变更
func (l *RBACAuditLogger) LogRoleChange(operatorID, operatorName, targetUserID, oldRole, newRole string) {
	level := AuditLevelInfo
	if newRole == string(RoleAdmin) || oldRole == string(RoleAdmin) {
		level = AuditLevelWarning // 管理员角色变更为警告级别
	}

	l.Log(&AuditEvent{
		Timestamp: time.Now(),
		Level:     level,
		Category:  AuditCategoryRole,
		Event:     "role_change",
		UserID:    operatorID,
		Username:  operatorName,
		TargetID:  targetUserID,
		OldValue:  oldRole,
		NewValue:  newRole,
		Result:    "success",
	})
}

// LogPolicyCreate 记录策略创建
func (l *RBACAuditLogger) LogPolicyCreate(operatorID, operatorName, policyName string) {
	l.Log(&AuditEvent{
		Timestamp:  time.Now(),
		Level:      AuditLevelInfo,
		Category:   AuditCategoryPolicy,
		Event:      "policy_create",
		UserID:     operatorID,
		Username:   operatorName,
		TargetName: policyName,
		Result:     "success",
	})
}

// LogPolicyUpdate 记录策略更新
func (l *RBACAuditLogger) LogPolicyUpdate(operatorID, operatorName, policyID string, changes map[string]interface{}) {
	l.Log(&AuditEvent{
		Timestamp: time.Now(),
		Level:     AuditLevelInfo,
		Category:  AuditCategoryPolicy,
		Event:     "policy_update",
		UserID:    operatorID,
		Username:  operatorName,
		TargetID:  policyID,
		NewValue:  changes,
		Result:    "success",
	})
}

// LogPolicyDelete 记录策略删除
func (l *RBACAuditLogger) LogPolicyDelete(operatorID, operatorName, policyID string) {
	l.Log(&AuditEvent{
		Timestamp: time.Now(),
		Level:     AuditLevelWarning, // 策略删除为警告级别
		Category:  AuditCategoryPolicy,
		Event:     "policy_delete",
		UserID:    operatorID,
		Username:  operatorName,
		TargetID:  policyID,
		Result:    "success",
	})
}

// LogAccessCheck 记录访问检查（可选，通常只在失败时记录）
func (l *RBACAuditLogger) LogAccessCheck(userID, resource, action string, allowed bool, reason string) {
	level := AuditLevelInfo
	if !allowed {
		level = AuditLevelWarning
	}

	result := "success"
	if !allowed {
		result = "denied"
	}

	l.Log(&AuditEvent{
		Timestamp: time.Now(),
		Level:     level,
		Category:  AuditCategoryAccess,
		Event:     "access_check",
		UserID:    userID,
		Resource:  resource,
		Action:    action,
		Result:    result,
		Reason:    reason,
	})
}

// LogGroupPermissionChange 记录组权限变更
func (l *RBACAuditLogger) LogGroupPermissionChange(operatorID, operatorName, groupID string, permissions []string) {
	l.Log(&AuditEvent{
		Timestamp: time.Now(),
		Level:     AuditLevelInfo,
		Category:  AuditCategoryGroup,
		Event:     "group_permission_change",
		UserID:    operatorID,
		Username:  operatorName,
		TargetID:  groupID,
		NewValue:  permissions,
		Result:    "success",
	})
}

// LogShareACLChange 记录共享 ACL 变更
func (l *RBACAuditLogger) LogShareACLChange(operatorID, operatorName, shareName string, changes map[string]interface{}) {
	l.Log(&AuditEvent{
		Timestamp:  time.Now(),
		Level:      AuditLevelInfo,
		Category:   AuditCategoryShare,
		Event:      "share_acl_change",
		UserID:     operatorID,
		Username:   operatorName,
		TargetName: shareName,
		NewValue:   changes,
		Result:     "success",
	})
}

// ========== 审计中间件 ==========

// AuditMiddleware 审计中间件
type AuditMiddleware struct {
	logger *RBACAuditLogger
}

// NewAuditMiddleware 创建审计中间件
func NewAuditMiddleware(logger *RBACAuditLogger) *AuditMiddleware {
	return &AuditMiddleware{
		logger: logger,
	}
}

// WrapManager 包装 RBAC 管理器，添加审计日志
func (am *AuditMiddleware) WrapManager(m *Manager) *AuditedManager {
	return &AuditedManager{
		Manager: m,
		logger:  am.logger,
	}
}

// AuditedManager 带审计日志的 RBAC 管理器
type AuditedManager struct {
	*Manager
	logger *RBACAuditLogger
}

// SetUserRole 设置用户角色（带审计）
func (am *AuditedManager) SetUserRole(operatorID, operatorName, userID, username string, role Role) error {
	// 获取旧角色
	oldRole := RoleGuest
	if up, err := am.GetUserPermissions(userID); err == nil {
		oldRole = up.Role
	}

	err := am.Manager.SetUserRole(userID, username, role)
	if err != nil {
		return err
	}

	am.logger.LogRoleChange(operatorID, operatorName, userID, string(oldRole), string(role))
	return nil
}

// GrantPermission 授予权限（带审计）
func (am *AuditedManager) GrantPermissionWithAudit(operatorID, operatorName, userID, username, permission string) error {
	err := am.GrantPermission(userID, username, permission)
	if err != nil {
		return err
	}

	am.logger.LogPermissionGrant(operatorID, operatorName, userID, permission)
	return nil
}

// RevokePermission 撤销权限（带审计）
func (am *AuditedManager) RevokePermissionWithAudit(operatorID, operatorName, userID, permission string) error {
	err := am.RevokePermission(userID, permission)
	if err != nil {
		return err
	}

	am.logger.LogPermissionRevoke(operatorID, operatorName, userID, permission)
	return nil
}

// CreatePolicyWithAudit 创建策略（带审计）
func (am *AuditedManager) CreatePolicyWithAudit(operatorID, operatorName, name, description string, effect PolicyEffect, principals, resources, actions []string, priority int) (*Policy, error) {
	policy, err := am.CreatePolicy(name, description, effect, principals, resources, actions, priority)
	if err != nil {
		return nil, err
	}

	am.logger.LogPolicyCreate(operatorID, operatorName, name)
	return policy, nil
}

// DeletePolicyWithAudit 删除策略（带审计）
func (am *AuditedManager) DeletePolicyWithAudit(operatorID, operatorName, policyID string) error {
	err := am.DeletePolicy(policyID)
	if err != nil {
		return err
	}

	am.logger.LogPolicyDelete(operatorID, operatorName, policyID)
	return nil
}

// ========== 审计统计 ==========

// AuditStats 审计统计
type AuditStats struct {
	TotalEvents       int            `json:"total_events"`
	PermissionGrants  int            `json:"permission_grants"`
	PermissionRevokes int            `json:"permission_revokes"`
	RoleChanges       int            `json:"role_changes"`
	PolicyChanges     int            `json:"policy_changes"`
	AccessDenials     int            `json:"access_denials"`
	ByCategory        map[string]int `json:"by_category"`
	ByLevel           map[string]int `json:"by_level"`
	TopOperators      []OperatorStat `json:"top_operators"`
	RecentEvents      []*AuditEvent  `json:"recent_events"`
}

// OperatorStat 操作者统计
type OperatorStat struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Count    int    `json:"count"`
}

// AuditStatsCollector 审计统计收集器
type AuditStatsCollector struct {
	events        []*AuditEvent
	maxEvents     int
	operatorCount map[string]*OperatorStat
	categoryCount map[string]int
	levelCount    map[string]int
}

// NewAuditStatsCollector 创建审计统计收集器
func NewAuditStatsCollector(maxEvents int) *AuditStatsCollector {
	return &AuditStatsCollector{
		events:        make([]*AuditEvent, 0, maxEvents),
		maxEvents:     maxEvents,
		operatorCount: make(map[string]*OperatorStat),
		categoryCount: make(map[string]int),
		levelCount:    make(map[string]int),
	}
}

// Record 记录事件
func (c *AuditStatsCollector) Record(event *AuditEvent) {
	// 添加到事件列表
	c.events = append(c.events, event)
	if len(c.events) > c.maxEvents {
		c.events = c.events[len(c.events)-c.maxEvents:]
	}

	// 更新统计
	c.categoryCount[string(event.Category)]++
	c.levelCount[string(event.Level)]++

	if event.UserID != "" {
		if stat, exists := c.operatorCount[event.UserID]; exists {
			stat.Count++
		} else {
			c.operatorCount[event.UserID] = &OperatorStat{
				UserID:   event.UserID,
				Username: event.Username,
				Count:    1,
			}
		}
	}
}

// GetStats 获取统计
func (c *AuditStatsCollector) GetStats() *AuditStats {
	stats := &AuditStats{
		TotalEvents:  len(c.events),
		ByCategory:   make(map[string]int),
		ByLevel:      make(map[string]int),
		TopOperators: make([]OperatorStat, 0),
		RecentEvents: c.events,
	}

	// 复制分类和级别统计
	for k, v := range c.categoryCount {
		stats.ByCategory[k] = v
	}
	for k, v := range c.levelCount {
		stats.ByLevel[k] = v
	}

	// 计算各类事件数量
	for _, event := range c.events {
		switch event.Event {
		case "permission_grant":
			stats.PermissionGrants++
		case "permission_revoke":
			stats.PermissionRevokes++
		case "role_change":
			stats.RoleChanges++
		case "policy_create", "policy_update", "policy_delete":
			stats.PolicyChanges++
		}

		if event.Result == "denied" {
			stats.AccessDenials++
		}
	}

	// 转换操作者统计
	for _, stat := range c.operatorCount {
		stats.TopOperators = append(stats.TopOperators, *stat)
	}

	return stats
}
