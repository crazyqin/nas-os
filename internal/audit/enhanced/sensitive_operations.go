package enhanced

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SensitiveOperationManager 敏感操作管理器
type SensitiveOperationManager struct {
	operations []*SensitiveOperation
	events     []*SensitiveOperationEvent
	approvals  map[string]*OperationApproval
	config     SensitiveOpConfig
	mu         sync.RWMutex
	storageDir string
}

// SensitiveOpConfig 敏感操作配置
type SensitiveOpConfig struct {
	Enabled             bool          `json:"enabled"`
	RequireApproval     bool          `json:"require_approval"`
	ApprovalTimeout     time.Duration `json:"approval_timeout"`
	MaxPendingApprovals int           `json:"max_pending_approvals"`
	NotifyOnSensitive   bool          `json:"notify_on_sensitive"`
	BlockHighRisk       bool          `json:"block_high_risk"`
	LogAllSensitive     bool          `json:"log_all_sensitive"`
}

// DefaultSensitiveOpConfig 默认敏感操作配置
func DefaultSensitiveOpConfig() SensitiveOpConfig {
	return SensitiveOpConfig{
		Enabled:             true,
		RequireApproval:     true,
		ApprovalTimeout:     time.Hour * 24,
		MaxPendingApprovals: 100,
		NotifyOnSensitive:   true,
		BlockHighRisk:       false,
		LogAllSensitive:     true,
	}
}

// NewSensitiveOperationManager 创建敏感操作管理器
func NewSensitiveOperationManager() *SensitiveOperationManager {
	storageDir := "/var/log/nas-os/audit/sensitive"
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		// 创建目录失败时使用当前目录
		storageDir = "."
	}

	m := &SensitiveOperationManager{
		operations: make([]*SensitiveOperation, 0),
		events:     make([]*SensitiveOperationEvent, 0),
		approvals:  make(map[string]*OperationApproval),
		config:     DefaultSensitiveOpConfig(),
		storageDir: storageDir,
	}

	// 初始化默认敏感操作
	m.initDefaultOperations()

	return m
}

// initDefaultOperations 初始化默认敏感操作定义
func (m *SensitiveOperationManager) initDefaultOperations() {
	defaultOps := []SensitiveOperation{
		// 用户管理 - Critical
		{
			ID:               "user_delete",
			Name:             "删除用户",
			Description:      "删除系统用户账户",
			Category:         OperationCategoryUser,
			Action:           ActionDelete,
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  60,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"user_management", "destructive"},
		},
		{
			ID:               "user_role_change",
			Name:             "修改用户角色",
			Description:      "修改用户的权限角色",
			Category:         OperationCategoryUser,
			Action:           ActionUpdate,
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  30,
			NotifyAdmins:     true,
			NotifyUser:       true,
			LogDetails:       true,
			Tags:             []string{"user_management", "privilege"},
		},
		{
			ID:               "user_password_reset",
			Name:             "重置用户密码",
			Description:      "重置其他用户的密码",
			Category:         OperationCategoryUser,
			Action:           ActionUpdate,
			SensitivityLevel: SensitivityHigh,
			RequiresMFA:      true,
			RequiresApproval: false,
			NotifyAdmins:     true,
			NotifyUser:       true,
			LogDetails:       true,
			Tags:             []string{"user_management", "security"},
		},

		// 系统配置 - Critical/High
		{
			ID:               "system_config_change",
			Name:             "系统配置修改",
			Description:      "修改系统核心配置",
			Category:         OperationCategorySystem,
			Action:           ActionUpdate,
			ResourcePattern:  "/system/config/*",
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  30,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"system", "configuration"},
		},
		{
			ID:               "firewall_config_change",
			Name:             "防火墙配置修改",
			Description:      "修改防火墙规则",
			Category:         OperationCategoryNetwork,
			Action:           ActionUpdate,
			SensitivityLevel: SensitivityHigh,
			RequiresMFA:      true,
			RequiresApproval: false,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"network", "security"},
		},
		{
			ID:               "service_restart",
			Name:             "重启服务",
			Description:      "重启系统服务",
			Category:         OperationCategorySystem,
			Action:           ActionRestart,
			SensitivityLevel: SensitivityMedium,
			RequiresMFA:      false,
			RequiresApproval: false,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       false,
			Tags:             []string{"system", "service"},
		},

		// 存储管理 - High
		{
			ID:               "volume_delete",
			Name:             "删除存储卷",
			Description:      "删除存储卷及其数据",
			Category:         OperationCategoryStorage,
			Action:           ActionDelete,
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  60,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"storage", "destructive"},
		},
		{
			ID:               "share_delete",
			Name:             "删除共享",
			Description:      "删除文件共享",
			Category:         OperationCategoryShare,
			Action:           ActionDelete,
			SensitivityLevel: SensitivityHigh,
			RequiresMFA:      true,
			RequiresApproval: false,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"share", "access"},
		},

		// 文件操作 - High/Medium
		{
			ID:               "file_delete",
			Name:             "批量删除文件",
			Description:      "批量删除文件或目录",
			Category:         OperationCategoryFile,
			Action:           ActionDelete,
			SensitivityLevel: SensitivityHigh,
			RequiresMFA:      false,
			RequiresApproval: false,
			NotifyAdmins:     false,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"file", "destructive"},
		},
		{
			ID:               "file_share_external",
			Name:             "外部共享文件",
			Description:      "创建外部可访问的文件共享链接",
			Category:         OperationCategoryFile,
			Action:           ActionShare,
			SensitivityLevel: SensitivityMedium,
			RequiresMFA:      false,
			RequiresApproval: false,
			NotifyAdmins:     false,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"file", "sharing"},
		},

		// 备份恢复 - Critical
		{
			ID:               "backup_delete",
			Name:             "删除备份",
			Description:      "删除备份数据",
			Category:         OperationCategoryBackup,
			Action:           ActionDelete,
			SensitivityLevel: SensitivityHigh,
			RequiresMFA:      true,
			RequiresApproval: false,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"backup", "destructive"},
		},
		{
			ID:               "restore_operation",
			Name:             "恢复操作",
			Description:      "从备份恢复数据",
			Category:         OperationCategoryBackup,
			Action:           ActionExecute,
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  120,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"backup", "restore"},
		},

		// 安全配置 - Critical
		{
			ID:               "mfa_disable",
			Name:             "禁用MFA",
			Description:      "禁用多因素认证",
			Category:         OperationCategorySecurity,
			Action:           ActionDisable,
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  30,
			NotifyAdmins:     true,
			NotifyUser:       true,
			LogDetails:       true,
			Tags:             []string{"security", "authentication"},
		},
		{
			ID:               "ssl_cert_change",
			Name:             "SSL证书变更",
			Description:      "添加、更新或删除SSL证书",
			Category:         OperationCategorySecurity,
			Action:           ActionUpdate,
			SensitivityLevel: SensitivityHigh,
			RequiresMFA:      true,
			RequiresApproval: false,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"security", "certificate"},
		},

		// 容器/VM管理 - High
		{
			ID:               "container_privileged",
			Name:             "特权容器操作",
			Description:      "创建或修改特权容器",
			Category:         OperationCategoryContainer,
			Action:           ActionCreate,
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  60,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"container", "privilege"},
		},
		{
			ID:               "vm_delete",
			Name:             "删除虚拟机",
			Description:      "删除虚拟机及其磁盘",
			Category:         OperationCategoryVM,
			Action:           ActionDelete,
			SensitivityLevel: SensitivityCritical,
			RequiresMFA:      true,
			RequiresApproval: true,
			ApprovalTimeout:  60,
			NotifyAdmins:     true,
			NotifyUser:       false,
			LogDetails:       true,
			Tags:             []string{"vm", "destructive"},
		},
	}

	m.mu.Lock()
	// 转换为指针切片
	for i := range defaultOps {
		m.operations = append(m.operations, &defaultOps[i])
	}
	m.mu.Unlock()
}

// ========== 敏感操作定义管理 ==========

// AddOperation 添加敏感操作定义
func (m *SensitiveOperationManager) AddOperation(op *SensitiveOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if op.ID == "" {
		op.ID = uuid.New().String()
	}

	m.operations = append(m.operations, op)
	return nil
}

// UpdateOperation 更新敏感操作定义
func (m *SensitiveOperationManager) UpdateOperation(op *SensitiveOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, existing := range m.operations {
		if existing.ID == op.ID {
			m.operations[i] = op
			return nil
		}
	}

	return nil
}

// DeleteOperation 删除敏感操作定义
func (m *SensitiveOperationManager) DeleteOperation(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, op := range m.operations {
		if op.ID == id {
			m.operations = append(m.operations[:i], m.operations[i+1:]...)
			return
		}
	}
}

// getOperationInternal 内部获取敏感操作定义（不获取锁，用于已持有锁的方法调用）
func (m *SensitiveOperationManager) getOperationInternal(id string) *SensitiveOperation {
	for _, op := range m.operations {
		if op.ID == id {
			return op
		}
	}
	return nil
}

// GetOperation 获取敏感操作定义
func (m *SensitiveOperationManager) GetOperation(id string) *SensitiveOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getOperationInternal(id)
}

// ListOperations 列出所有敏感操作定义
func (m *SensitiveOperationManager) ListOperations() []*SensitiveOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SensitiveOperation, len(m.operations))
	copy(result, m.operations)
	return result
}

// ========== 敏感操作检测 ==========

// CheckSensitive 检查操作是否为敏感操作
func (m *SensitiveOperationManager) CheckSensitive(category OperationCategory, action OperationAction, resourcePath string) *SensitiveOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, op := range m.operations {
		if op.Category != category {
			continue
		}
		if op.Action != action {
			continue
		}

		// 检查资源模式匹配
		if op.ResourcePattern != "" && resourcePath != "" {
			matched, err := regexp.MatchString(op.ResourcePattern, resourcePath)
			if err == nil && !matched {
				continue
			}
		}

		return op
	}

	return nil
}

// IsSensitive 判断操作是否敏感
func (m *SensitiveOperationManager) IsSensitive(category OperationCategory, action OperationAction, resourcePath string) bool {
	return m.CheckSensitive(category, action, resourcePath) != nil
}

// GetSensitivityLevel 获取操作的敏感级别
func (m *SensitiveOperationManager) GetSensitivityLevel(category OperationCategory, action OperationAction, resourcePath string) SensitivityLevel {
	if op := m.CheckSensitive(category, action, resourcePath); op != nil {
		return op.SensitivityLevel
	}
	return SensitivityLow
}

// ========== 敏感操作事件记录 ==========

// RecordEvent 记录敏感操作事件
func (m *SensitiveOperationManager) RecordEvent(
	operationID, operationName string,
	userID, username, ip, sessionID, resource string,
	details map[string]interface{},
) *SensitiveOperationEvent {
	// 先获取操作信息（需要在锁外调用以避免死锁）
	op := m.getOperationInternal(operationID)

	m.mu.Lock()
	defer m.mu.Unlock()
	riskScore := 50
	if op != nil {
		switch op.SensitivityLevel {
		case SensitivityCritical:
			riskScore = 95
		case SensitivityHigh:
			riskScore = 75
		case SensitivityMedium:
			riskScore = 55
		}
	}

	event := &SensitiveOperationEvent{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		OperationID:   operationID,
		OperationName: operationName,
		UserID:        userID,
		Username:      username,
		IP:            ip,
		SessionID:     sessionID,
		Resource:      resource,
		Details:       details,
		Approved:      false,
		Blocked:       false,
		RiskScore:     riskScore,
	}

	m.events = append(m.events, event)

	// 限制事件数量
	if len(m.events) > 100000 {
		m.events = m.events[len(m.events)-100000:]
	}

	return event
}

// RecordApprovedEvent 记录已批准的敏感操作事件
func (m *SensitiveOperationManager) RecordApprovedEvent(
	operationID, operationName string,
	userID, username, ip, sessionID, resource string,
	details map[string]interface{},
	approvedBy string,
	notes string,
) *SensitiveOperationEvent {
	event := m.RecordEvent(operationID, operationName, userID, username, ip, sessionID, resource, details)

	m.mu.Lock()
	event.Approved = true
	event.ApprovedBy = approvedBy
	now := time.Now()
	event.ApprovedAt = &now
	event.ApprovalNotes = notes
	m.mu.Unlock()

	return event
}

// RecordBlockedEvent 记录被阻止的敏感操作事件
func (m *SensitiveOperationManager) RecordBlockedEvent(
	operationID, operationName string,
	userID, username, ip, sessionID, resource string,
	details map[string]interface{},
	reason string,
) *SensitiveOperationEvent {
	event := m.RecordEvent(operationID, operationName, userID, username, ip, sessionID, resource, details)

	m.mu.Lock()
	event.Blocked = true
	event.BlockReason = reason
	m.mu.Unlock()

	return event
}

// ========== 审批管理 ==========

// CreateApprovalRequest 创建审批请求
func (m *SensitiveOperationManager) CreateApprovalRequest(
	operationID string,
	requestedBy, requestorName string,
	resource string,
	details map[string]interface{},
) (*OperationApproval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 使用内部方法避免死锁（已持有锁）
	op := m.getOperationInternal(operationID)
	if op == nil {
		return nil, nil
	}

	timeout := time.Duration(op.ApprovalTimeout) * time.Minute
	if timeout == 0 {
		timeout = m.config.ApprovalTimeout
	}

	approval := &OperationApproval{
		ID:            uuid.New().String(),
		OperationID:   operationID,
		RequestedBy:   requestedBy,
		RequestorName: requestorName,
		RequestTime:   time.Now(),
		Operation:     op,
		Resource:      resource,
		Details:       details,
		Status:        "pending",
		ExpiresAt:     time.Now().Add(timeout),
	}

	m.approvals[approval.ID] = approval

	// 限制待审批数量
	if len(m.approvals) > m.config.MaxPendingApprovals {
		m.cleanupExpiredApprovals()
	}

	return approval, nil
}

// ApproveOperation 批准操作
func (m *SensitiveOperationManager) ApproveOperation(approvalID, approvedBy, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, exists := m.approvals[approvalID]
	if !exists {
		return nil
	}

	if approval.Status != "pending" {
		return nil
	}

	now := time.Now()
	approval.Status = "approved"
	approval.ApprovedBy = approvedBy
	approval.ApprovedAt = &now
	approval.Notes = notes

	return nil
}

// RejectOperation 拒绝操作
func (m *SensitiveOperationManager) RejectOperation(approvalID, rejectedBy, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, exists := m.approvals[approvalID]
	if !exists {
		return nil
	}

	if approval.Status != "pending" {
		return nil
	}

	now := time.Now()
	approval.Status = "rejected"
	approval.RejectedBy = rejectedBy
	approval.RejectedAt = &now
	approval.RejectReason = reason

	return nil
}

// GetPendingApprovals 获取待审批列表
func (m *SensitiveOperationManager) GetPendingApprovals() []*OperationApproval {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*OperationApproval, 0)
	for _, approval := range m.approvals {
		if approval.Status == "pending" {
			result = append(result, approval)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].RequestTime.Before(result[j].RequestTime)
	})

	return result
}

// GetApproval 获取审批详情
func (m *SensitiveOperationManager) GetApproval(approvalID string) *OperationApproval {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.approvals[approvalID]
}

// cleanupExpiredApprovals 清理过期审批
func (m *SensitiveOperationManager) cleanupExpiredApprovals() {
	now := time.Now()
	for id, approval := range m.approvals {
		if approval.Status == "pending" && now.After(approval.ExpiresAt) {
			approval.Status = "expired"
			delete(m.approvals, id)
		}
	}
}

// ========== 查询和统计 ==========

// QueryEvents 查询敏感操作事件
func (m *SensitiveOperationManager) QueryEvents(start, end time.Time, userID string, limit int) []*SensitiveOperationEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SensitiveOperationEvent, 0)
	for _, event := range m.events {
		if start.After(event.Timestamp) || end.Before(event.Timestamp) {
			continue
		}
		if userID != "" && event.UserID != userID {
			continue
		}
		result = append(result, event)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// GetSummary 获取敏感操作摘要
func (m *SensitiveOperationManager) GetSummary(start, end time.Time) *SensitiveOpsSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &SensitiveOpsSummary{
		OpsByLevel: make(map[string]int),
		OpsByType:  make([]SensitiveOpCount, 0),
		TopUsers:   make([]UserSensitiveOpCount, 0),
	}

	userCounts := make(map[string]int)
	opCounts := make(map[string]int)
	totalApprovalTime := 0
	approvalCount := 0

	for _, event := range m.events {
		if start.After(event.Timestamp) || end.Before(event.Timestamp) {
			continue
		}

		summary.TotalSensitiveOps++

		if event.Approved {
			summary.ApprovedOps++
		}
		if event.Blocked {
			summary.BlockedOps++
		}

		op := m.GetOperation(event.OperationID)
		if op != nil {
			summary.OpsByLevel[string(op.SensitivityLevel)]++
			opCounts[op.Name]++
		}

		userCounts[event.UserID]++
	}

	// 待审批数量
	for _, approval := range m.approvals {
		if approval.Status == "pending" {
			summary.PendingApprovals++
		}
	}

	// 操作类型统计
	for name, count := range opCounts {
		summary.OpsByType = append(summary.OpsByType, SensitiveOpCount{
			OperationName: name,
			Count:         count,
		})
	}
	sort.Slice(summary.OpsByType, func(i, j int) bool {
		return summary.OpsByType[i].Count > summary.OpsByType[j].Count
	})

	// 用户统计
	for userID, count := range userCounts {
		summary.TopUsers = append(summary.TopUsers, UserSensitiveOpCount{
			UserID: userID,
			Count:  count,
		})
	}
	sort.Slice(summary.TopUsers, func(i, j int) bool {
		return summary.TopUsers[i].Count > summary.TopUsers[j].Count
	})
	if len(summary.TopUsers) > 10 {
		summary.TopUsers = summary.TopUsers[:10]
	}

	// 平均审批时间
	if approvalCount > 0 {
		summary.AvgApprovalTime = totalApprovalTime / approvalCount
	}

	return summary
}

// ========== 持久化 ==========

// Save 保存数据
func (m *SensitiveOperationManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 保存敏感操作定义
	opsData, err := json.MarshalIndent(m.operations, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(m.storageDir, "operations.json"), opsData, 0640); err != nil {
		return err
	}

	// 保存事件
	today := time.Now().Format("2006-01-02")
	eventsData, err := json.MarshalIndent(m.events, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(m.storageDir, "events-"+today+".log"), eventsData, 0640); err != nil {
		return err
	}

	return nil
}

// Load 加载数据
func (m *SensitiveOperationManager) Load() error {
	// 加载敏感操作定义
	opsData, err := os.ReadFile(filepath.Join(m.storageDir, "operations.json"))
	if err == nil {
		var ops []*SensitiveOperation
		if err := json.Unmarshal(opsData, &ops); err == nil {
			m.mu.Lock()
			m.operations = ops
			m.mu.Unlock()
		}
	}

	return nil
}
