package selfserviceportal

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidStatus = errors.New("invalid status transition")
	ErrAlreadyExists = errors.New("already exists")
	ErrUnauthorized  = errors.New("unauthorized")
)

// NewPortal 创建新的自助门户实例
func NewPortal() *Portal {
	return &Portal{
		quotaRequests:   make(map[string]*QuotaRequest),
		permRequests:    make(map[string]*PermissionRequest),
		restoreRequests: make(map[string]*RestoreRequest),
		issueTickets:    make(map[string]*IssueTicket),
		approvals:       make(map[string][]*Approval),
		notifications:   make(map[string][]*Notification),
		restorePoints:   make(map[string][]*RestorePoint),
		userStats:       make(map[string]*UserStats),
		autoApprovalRules: []*AutoApprovalRule{
			{
				ID:                  "rule-1",
				Name:                "Auto approve small quota increase",
				Description:         "Auto approve if requested amount is less than 50% of current quota",
				MaxAutoApproveGB:    100,
				MaxPercentOfCurrent: 50,
				Enabled:             true,
			},
		},
		nextID: 1,
	}
}

func (p *Portal) generateID() string {
	id := fmt.Sprintf("id-%d", p.nextID)
	p.nextID++
	return id
}

// ========== 配额管理 ==========

// SubmitQuotaRequest 提交配额申请
func (p *Portal) SubmitQuotaRequest(userID string, currentGB, requestedGB int64, reason string) (*QuotaRequest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	req := &QuotaRequest{
		ID:          p.generateID(),
		UserID:      userID,
		CurrentGB:   currentGB,
		RequestedGB: requestedGB,
		Reason:      reason,
		Status:      TicketPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 检查自动审批规则
	if p.checkAutoApproval(currentGB, requestedGB) {
		req.Status = TicketApproved
	}

	p.quotaRequests[req.ID] = req

	// 更新用户统计
	p.updateUserStats(userID)

	// 创建通知
	p.createNotification(userID, "配额申请已提交", fmt.Sprintf("申请 %dGB 配额，状态: %s", requestedGB, req.Status))

	return req, nil
}

// checkAutoApproval 检查是否满足自动审批条件
func (p *Portal) checkAutoApproval(currentGB, requestedGB int64) bool {
	for _, rule := range p.autoApprovalRules {
		if !rule.Enabled {
			continue
		}
		if requestedGB <= rule.MaxAutoApproveGB {
			return true
		}
		if currentGB > 0 && float64(requestedGB) < float64(currentGB)*rule.MaxPercentOfCurrent/100 {
			return true
		}
	}
	return false
}

// GetQuotaRequest 获取配额申请详情
func (p *Portal) GetQuotaRequest(id string) (*QuotaRequest, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	req, ok := p.quotaRequests[id]
	if !ok {
		return nil, ErrNotFound
	}
	return req, nil
}

// ListQuotaRequests 获取用户的配额申请列表
func (p *Portal) ListQuotaRequests(userID string) []*QuotaRequest {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var requests []*QuotaRequest
	for _, req := range p.quotaRequests {
		if req.UserID == userID {
			requests = append(requests, req)
		}
	}
	return requests
}

// ApproveQuotaRequest 审批配额申请
func (p *Portal) ApproveQuotaRequest(id, approverID, comment string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, ok := p.quotaRequests[id]
	if !ok {
		return ErrNotFound
	}

	if req.Status != TicketPending {
		return ErrInvalidStatus
	}

	req.Status = TicketApproved
	req.UpdatedAt = time.Now()

	// 记录审批
	p.addApproval(id, TicketTypeQuota, approverID, ApprovalApproved, comment)

	// 通知用户
	p.createNotification(req.UserID, "配额申请已批准", fmt.Sprintf("您的 %dGB 配额申请已批准", req.RequestedGB))

	return nil
}

// RejectQuotaRequest 拒绝配额申请
func (p *Portal) RejectQuotaRequest(id, approverID, comment string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, ok := p.quotaRequests[id]
	if !ok {
		return ErrNotFound
	}

	if req.Status != TicketPending {
		return ErrInvalidStatus
	}

	req.Status = TicketRejected
	req.UpdatedAt = time.Now()

	p.addApproval(id, TicketTypeQuota, approverID, ApprovalRejected, comment)
	p.createNotification(req.UserID, "配额申请已拒绝", fmt.Sprintf("您的 %dGB 配额申请被拒绝，原因: %s", req.RequestedGB, comment))

	return nil
}

// ========== 权限管理 ==========

// SubmitPermissionRequest 提交权限申请
func (p *Portal) SubmitPermissionRequest(userID, sharePath string, permType PermissionType, temporary bool, expiresAt *time.Time, reason string) (*PermissionRequest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	req := &PermissionRequest{
		ID:        p.generateID(),
		UserID:    userID,
		SharePath: sharePath,
		PermType:  permType,
		Temporary: temporary,
		ExpiresAt: expiresAt,
		Reason:    reason,
		Status:    TicketPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	p.permRequests[req.ID] = req
	p.updateUserStats(userID)
	p.createNotification(userID, "权限申请已提交", fmt.Sprintf("申请 %s 权限到 %s", permType, sharePath))

	return req, nil
}

// GetPermissionRequest 获取权限申请详情
func (p *Portal) GetPermissionRequest(id string) (*PermissionRequest, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	req, ok := p.permRequests[id]
	if !ok {
		return nil, ErrNotFound
	}
	return req, nil
}

// ListPermissionRequests 获取用户的权限申请列表
func (p *Portal) ListPermissionRequests(userID string) []*PermissionRequest {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var requests []*PermissionRequest
	for _, req := range p.permRequests {
		if req.UserID == userID {
			requests = append(requests, req)
		}
	}
	return requests
}

// ApprovePermissionRequest 审批权限申请
func (p *Portal) ApprovePermissionRequest(id, approverID, comment string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, ok := p.permRequests[id]
	if !ok {
		return ErrNotFound
	}

	if req.Status != TicketPending {
		return ErrInvalidStatus
	}

	req.Status = TicketApproved
	req.UpdatedAt = time.Now()

	p.addApproval(id, TicketTypePerm, approverID, ApprovalApproved, comment)
	p.createNotification(req.UserID, "权限申请已批准", fmt.Sprintf("您的 %s 权限申请已批准", req.PermType))

	return nil
}

// RejectPermissionRequest 拒绝权限申请
func (p *Portal) RejectPermissionRequest(id, approverID, comment string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, ok := p.permRequests[id]
	if !ok {
		return ErrNotFound
	}

	if req.Status != TicketPending {
		return ErrInvalidStatus
	}

	req.Status = TicketRejected
	req.UpdatedAt = time.Now()

	p.addApproval(id, TicketTypePerm, approverID, ApprovalRejected, comment)
	p.createNotification(req.UserID, "权限申请已拒绝", fmt.Sprintf("您的 %s 权限申请被拒绝", req.PermType))

	return nil
}

// ========== 备份恢复 ==========

// CreateRestorePoint 创建恢复点
func (p *Portal) CreateRestorePoint(userID, filePath string, sizeBytes int64) *RestorePoint {
	p.mu.Lock()
	defer p.mu.Unlock()

	rp := &RestorePoint{
		ID:        p.generateID(),
		UserID:    userID,
		FilePath:  filePath,
		SizeBytes: sizeBytes,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour), // 30天有效期
	}

	p.restorePoints[userID] = append(p.restorePoints[userID], rp)
	p.updateUserStats(userID)

	return rp
}

// ListRestorePoints 获取用户的恢复点列表
func (p *Portal) ListRestorePoints(userID string) []*RestorePoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.restorePoints[userID]
}

// SubmitRestoreRequest 提交恢复请求
func (p *Portal) SubmitRestoreRequest(userID, restorePointID, targetPath string) (*RestoreRequest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 验证恢复点存在
	points := p.restorePoints[userID]
	found := false
	for _, rp := range points {
		if rp.ID == restorePointID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrNotFound
	}

	req := &RestoreRequest{
		ID:           p.generateID(),
		UserID:       userID,
		RestorePoint: restorePointID,
		TargetPath:   targetPath,
		Status:       TicketPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	p.restoreRequests[req.ID] = req
	p.createNotification(userID, "恢复请求已提交", fmt.Sprintf("恢复点 %s 的恢复请求已提交", restorePointID))

	return req, nil
}

// ApproveRestoreRequest 审批恢复请求
func (p *Portal) ApproveRestoreRequest(id, approverID, comment string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, ok := p.restoreRequests[id]
	if !ok {
		return ErrNotFound
	}

	if req.Status != TicketPending {
		return ErrInvalidStatus
	}

	req.Status = TicketApproved
	req.UpdatedAt = time.Now()

	p.addApproval(id, TicketTypeBackup, approverID, ApprovalApproved, comment)
	p.createNotification(req.UserID, "恢复请求已批准", "您的文件恢复请求已批准，正在处理中")

	return nil
}

// ========== 问题报告 ==========

// SubmitIssueTicket 提交问题工单
func (p *Portal) SubmitIssueTicket(userID, subject, description, category, priority string) *IssueTicket {
	p.mu.Lock()
	defer p.mu.Unlock()

	ticket := &IssueTicket{
		ID:          p.generateID(),
		UserID:      userID,
		Subject:     subject,
		Description: description,
		Category:    category,
		Priority:    priority,
		Status:      TicketPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	p.issueTickets[ticket.ID] = ticket
	p.updateUserStats(userID)
	p.createNotification(userID, "工单已创建", fmt.Sprintf("工单 %s: %s 已创建", ticket.ID, subject))

	return ticket
}

// GetIssueTicket 获取问题工单详情
func (p *Portal) GetIssueTicket(id string) (*IssueTicket, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ticket, ok := p.issueTickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	return ticket, nil
}

// ListIssueTickets 获取用户的问题工单列表
func (p *Portal) ListIssueTickets(userID string) []*IssueTicket {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var tickets []*IssueTicket
	for _, t := range p.issueTickets {
		if t.UserID == userID {
			tickets = append(tickets, t)
		}
	}
	return tickets
}

// AssignIssueTicket 分配工单
func (p *Portal) AssignIssueTicket(id, assigneeID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ticket, ok := p.issueTickets[id]
	if !ok {
		return ErrNotFound
	}

	ticket.AssignedTo = assigneeID
	ticket.UpdatedAt = time.Now()

	return nil
}

// ResolveIssueTicket 解决工单
func (p *Portal) ResolveIssueTicket(id, resolution string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ticket, ok := p.issueTickets[id]
	if !ok {
		return ErrNotFound
	}

	if ticket.Status == TicketResolved {
		return ErrInvalidStatus
	}

	ticket.Status = TicketResolved
	ticket.Resolution = resolution
	ticket.UpdatedAt = time.Now()

	p.createNotification(ticket.UserID, "工单已解决", fmt.Sprintf("工单 %s 已解决: %s", id, resolution))
	p.updateUserStats(ticket.UserID)

	return nil
}

// ========== 用户统计 ==========

// GetUserStats 获取用户统计信息
func (p *Portal) GetUserStats(userID string) *UserStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats, ok := p.userStats[userID]
	if !ok {
		stats = &UserStats{UserID: userID}
		p.userStats[userID] = stats
	}
	return stats
}

// updateUserStats 更新用户统计(需在写锁内调用)
func (p *Portal) updateUserStats(userID string) {
	stats, ok := p.userStats[userID]
	if !ok {
		stats = &UserStats{UserID: userID}
		p.userStats[userID] = stats
	}

	// 统计工单
	active := 0
	resolved := 0
	for _, t := range p.issueTickets {
		if t.UserID == userID {
			if t.Status == TicketResolved {
				resolved++
			} else {
				active++
			}
		}
	}
	for _, r := range p.quotaRequests {
		if r.UserID == userID {
			if r.Status == TicketResolved {
				resolved++
			} else {
				active++
			}
		}
	}
	for _, r := range p.permRequests {
		if r.UserID == userID {
			if r.Status == TicketResolved {
				resolved++
			} else {
				active++
			}
		}
	}

	stats.ActiveTickets = active
	stats.ResolvedTickets = resolved
	stats.RestorePoints = len(p.restorePoints[userID])

	// 计算使用百分比
	if stats.TotalQuotaGB > 0 {
		stats.UsagePercent = float64(stats.UsedQuotaGB) / float64(stats.TotalQuotaGB) * 100
	}
}

// SetUserQuota 设置用户配额
func (p *Portal) SetUserQuota(userID string, totalGB, usedGB int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats, ok := p.userStats[userID]
	if !ok {
		stats = &UserStats{UserID: userID}
		p.userStats[userID] = stats
	}

	stats.TotalQuotaGB = totalGB
	stats.UsedQuotaGB = usedGB
	if totalGB > 0 {
		stats.UsagePercent = float64(usedGB) / float64(totalGB) * 100
	}
}

// ========== 通知管理 ==========

// createNotification 创建通知(需在写锁内调用)
func (p *Portal) createNotification(userID, title, message string) {
	notif := &Notification{
		ID:        p.generateID(),
		UserID:    userID,
		Title:     title,
		Message:   message,
		Read:      false,
		CreatedAt: time.Now(),
	}
	p.notifications[userID] = append(p.notifications[userID], notif)
}

// ListNotifications 获取用户通知列表
func (p *Portal) ListNotifications(userID string) []*Notification {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.notifications[userID]
}

// MarkNotificationRead 标记通知已读
func (p *Portal) MarkNotificationRead(userID, notifID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	notifs, ok := p.notifications[userID]
	if !ok {
		return ErrNotFound
	}

	for _, n := range notifs {
		if n.ID == notifID {
			n.Read = true
			return nil
		}
	}

	return ErrNotFound
}

// ========== 审批记录 ==========

// addApproval 添加审批记录(需在写锁内调用)
func (p *Portal) addApproval(ticketID string, ticketType TicketType, approverID string, status ApprovalStatus, comment string) {
	approval := &Approval{
		ID:         p.generateID(),
		TicketID:   ticketID,
		TicketType: ticketType,
		ApproverID: approverID,
		Status:     status,
		Comment:    comment,
		CreatedAt:  time.Now(),
	}
	p.approvals[ticketID] = append(p.approvals[ticketID], approval)
}

// GetApprovals 获取工单的审批记录
func (p *Portal) GetApprovals(ticketID string) []*Approval {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.approvals[ticketID]
}

// ========== 自动审批规则 ==========

// AddAutoApprovalRule 添加自动审批规则
func (p *Portal) AddAutoApprovalRule(rule *AutoApprovalRule) {
	p.mu.Lock()
	defer p.mu.Unlock()

	rule.ID = p.generateID()
	p.autoApprovalRules = append(p.autoApprovalRules, rule)
}

// ListAutoApprovalRules 获取自动审批规则列表
func (p *Portal) ListAutoApprovalRules() []*AutoApprovalRule {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.autoApprovalRules
}
