package nascopilot

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager NAS Copilot 核心管理器
type Manager struct {
	conversations map[string]*Conversation
	messages      map[string][]*Message
	knowledge     map[string]*KnowledgeEntry
	tasks         map[string]*ScheduledTask
	preferences   map[string]*UserPreference
	auditLog      []*AuditEntry
	mu            sync.RWMutex
	executedCmds  int
	successCmds   int
}

// NewManager 创建新的管理器实例
func NewManager() *Manager {
	return &Manager{
		conversations: make(map[string]*Conversation),
		messages:      make(map[string][]*Message),
		knowledge:     make(map[string]*KnowledgeEntry),
		tasks:         make(map[string]*ScheduledTask),
		preferences:   make(map[string]*UserPreference),
		auditLog:      make([]*AuditEntry, 0),
	}
}

// CreateConversation 创建新对话
func (m *Manager) CreateConversation(userID, title string) *Conversation {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv := &Conversation{
		ID:         uuid.New().String(),
		UserID:     userID,
		Title:      title,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		Status:     ConverStatusActive,
	}
	m.conversations[conv.ID] = conv
	m.messages[conv.ID] = make([]*Message, 0)
	return conv
}

// SendMessage 发送消息并获取响应
func (m *Manager) SendMessage(conversationID, content, userID string) (*ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conv, ok := m.conversations[conversationID]
	if !ok {
		return nil, fmt.Errorf("conversation %s not found", conversationID)
	}

	// 添加用户消息
	userMsg := &Message{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           RoleUser,
		Content:        content,
		Timestamp:      time.Now(),
	}
	m.messages[conversationID] = append(m.messages[conversationID], userMsg)
	conv.MessageCount++
	conv.LastActive = time.Now()

	// 解析意图
	intent := m.parseIntent(content)

	// 生成回复
	responseText := m.generateResponse(intent, content)

	// 如果是命令类型，尝试执行
	var cmdResult *ActionResult
	if intent.Type == IntentAction {
		cmd := m.parseCommand(content)
		if cmd != nil {
			result := m.executeCommand(cmd)
			cmdResult = &result
			responseText = fmt.Sprintf("命令执行完成: %s", result.Message)
		}
	}

	// 添加助手回复
	assistantMsg := &Message{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           RoleAssistant,
		Content:        responseText,
		Timestamp:      time.Now(),
		CommandResult:  cmdResult,
	}
	m.messages[conversationID] = append(m.messages[conversationID], assistantMsg)
	conv.MessageCount++

	return &ChatResponse{
		ConversationID: conversationID,
		Message:        *assistantMsg,
		Intent:         intent,
	}, nil
}

// ParseIntent 解析用户意图
func (m *Manager) ParseIntent(text string) *Intent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.parseIntent(text)
}

// parseIntent 内部意图解析（无锁）
func (m *Manager) parseIntent(text string) *Intent {
	lower := strings.ToLower(text)
	intent := &Intent{
		Parameters: make(map[string]string),
	}

	// 按关键词匹配意图
	switch {
	case containsAny(lower, "存储", "硬盘", "磁盘", "空间", "容量", "storage", "disk"):
		intent.Type = IntentStorage
		intent.Confidence = 0.9
	case containsAny(lower, "备份", "恢复", "快照", "backup", "restore", "snapshot"):
		intent.Type = IntentBackup
		intent.Confidence = 0.9
	case containsAny(lower, "网络", "ip", "dns", "网卡", "network", "wifi", "防火墙"):
		intent.Type = IntentNetwork
		intent.Confidence = 0.85
	case containsAny(lower, "docker", "容器", "镜像", "container", "image"):
		intent.Type = IntentDocker
		intent.Confidence = 0.9
	case containsAny(lower, "用户", "权限", "密码", "user", "permission", "password"):
		intent.Type = IntentUser
		intent.Confidence = 0.85
	case containsAny(lower, "系统", "进程", "服务", "重启", "关机", "system", "service", "process"):
		intent.Type = IntentSystem
		intent.Confidence = 0.8
	case containsAny(lower, "查看", "查询", "显示", "多少", "状态", "list", "show", "status", "info"):
		intent.Type = IntentQuery
		intent.Confidence = 0.7
	case containsAny(lower, "创建", "新建", "删除", "更新", "修改", "执行", "运行", "create", "delete", "update", "run", "execute"):
		intent.Type = IntentAction
		intent.Confidence = 0.8
	default:
		intent.Type = IntentQuery
		intent.Confidence = 0.5
	}

	return intent
}

// generateResponse 根据意图生成回复
func (m *Manager) generateResponse(intent *Intent, text string) string {
	switch intent.Type {
	case IntentStorage:
		return "存储管理：当前 NAS 存储状态正常。如需查看详细磁盘使用情况，请使用「查看磁盘」命令。"
	case IntentBackup:
		return "备份管理：建议定期进行数据备份。您可以使用快照功能保护重要数据。"
	case IntentNetwork:
		return "网络配置：NAS 网络状态正常。如需修改网络设置，请告知具体需求。"
	case IntentDocker:
		return "Docker 管理：可以帮您管理容器和镜像。请告诉您想执行的具体操作。"
	case IntentUser:
		return "用户管理：可以帮您管理用户账户和权限设置。"
	case IntentSystem:
		return "系统管理：NAS 系统运行正常。如需执行系统操作，请确认具体需求。"
	case IntentAction:
		return "已收到操作指令，正在处理..."
	default:
		return "您好！我是 NAS 智能助手，可以帮您管理存储、备份、网络、Docker 等。请问有什么可以帮您的？"
	}
}

// parseCommand 解析命令
func (m *Manager) parseCommand(text string) *Command {
	lower := strings.ToLower(text)
	cmd := &Command{
		Parameters: make(map[string]string),
		Status:     CommandStatusPending,
	}

	switch {
	case containsAny(lower, "创建", "新建", "create"):
		cmd.Verb = CommandCreate
	case containsAny(lower, "删除", "delete", "remove"):
		cmd.Verb = CommandDelete
	case containsAny(lower, "更新", "修改", "update", "modify"):
		cmd.Verb = CommandUpdate
	case containsAny(lower, "查看", "列表", "list", "show"):
		cmd.Verb = CommandList
	case containsAny(lower, "备份", "backup"):
		cmd.Verb = CommandBackup
	case containsAny(lower, "恢复", "restore"):
		cmd.Verb = CommandRestore
	default:
		return nil
	}

	// 识别资源类型
	switch {
	case containsAny(lower, "用户", "user"):
		cmd.ResourceType = "user"
	case containsAny(lower, "容器", "docker", "container"):
		cmd.ResourceType = "container"
	case containsAny(lower, "共享", "share"):
		cmd.ResourceType = "share"
	case containsAny(lower, "备份", "backup"):
		cmd.ResourceType = "backup"
	default:
		cmd.ResourceType = "unknown"
	}

	return cmd
}

// executeCommand 执行命令
func (m *Manager) executeCommand(cmd *Command) ActionResult {
	m.executedCmds++
	cmd.Status = CommandStatusRunning

	// 模拟命令执行
	result := ActionResult{
		Success: true,
		Message: fmt.Sprintf("成功执行 %s 操作，资源类型: %s", cmd.Verb, cmd.ResourceType),
		Scope:   cmd.ResourceType,
	}

	m.successCmds++
	cmd.Status = CommandStatusSuccess
	return result
}

// ExecuteCommand 公开的命令执行方法
func (m *Manager) ExecuteCommand(cmd Command) ActionResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.executeCommand(&cmd)
}

// ListConversations 获取用户对话列表
func (m *Manager) ListConversations(userID string) []*Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Conversation
	for _, conv := range m.conversations {
		if userID == "" || conv.UserID == userID {
			result = append(result, conv)
		}
	}
	return result
}

// GetConversation 获取对话详情
func (m *Manager) GetConversation(id string) (*Conversation, []*Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, ok := m.conversations[id]
	if !ok {
		return nil, nil, fmt.Errorf("conversation %s not found", id)
	}
	return conv, m.messages[id], nil
}

// DeleteConversation 删除对话
func (m *Manager) DeleteConversation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.conversations[id]; !ok {
		return fmt.Errorf("conversation %s not found", id)
	}
	delete(m.conversations, id)
	delete(m.messages, id)
	return nil
}

// AddKnowledge 添加知识条目
func (m *Manager) AddKnowledge(entry KnowledgeEntry) *KnowledgeEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry.ID = uuid.New().String()
	m.knowledge[entry.ID] = &entry
	return &entry
}

// ListKnowledge 列出知识条目
func (m *Manager) ListKnowledge() []*KnowledgeEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*KnowledgeEntry, 0, len(m.knowledge))
	for _, entry := range m.knowledge {
		result = append(result, entry)
	}
	return result
}

// SearchKnowledge 搜索知识库
func (m *Manager) SearchKnowledge(query string) []*KnowledgeEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lower := strings.ToLower(query)
	var result []*KnowledgeEntry
	for _, entry := range m.knowledge {
		if strings.Contains(strings.ToLower(entry.Title), lower) ||
			strings.Contains(strings.ToLower(entry.Content), lower) {
			result = append(result, entry)
			continue
		}
		for _, tag := range entry.Tags {
			if strings.Contains(strings.ToLower(tag), lower) {
				result = append(result, entry)
				break
			}
		}
	}
	return result
}

// CreateScheduledTask 创建定时任务
func (m *Manager) CreateScheduledTask(desc, cronExpr, command string, enabled bool) *ScheduledTask {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &ScheduledTask{
		ID:          uuid.New().String(),
		Description: desc,
		CronExpr:    cronExpr,
		Command:     command,
		Enabled:     enabled,
		NextRun:     time.Now().Add(time.Hour), // 模拟下次运行
	}
	m.tasks[task.ID] = task
	return task
}

// ListScheduledTasks 列出定时任务
func (m *Manager) ListScheduledTasks() []*ScheduledTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ScheduledTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		result = append(result, task)
	}
	return result
}

// UpdateScheduledTask 更新定时任务
func (m *Manager) UpdateScheduledTask(id string, req UpdateTaskRequest) (*ScheduledTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}

	if req.Description != "" {
		task.Description = req.Description
	}
	if req.CronExpr != "" {
		task.CronExpr = req.CronExpr
	}
	if req.Command != "" {
		task.Command = req.Command
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	return task, nil
}

// DeleteScheduledTask 删除定时任务
func (m *Manager) DeleteScheduledTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return fmt.Errorf("task %s not found", id)
	}
	delete(m.tasks, id)
	return nil
}

// GetUserPreference 获取用户偏好
func (m *Manager) GetUserPreference(userID string) *UserPreference {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pref, ok := m.preferences[userID]
	if !ok {
		return &UserPreference{
			UserID:       userID,
			Language:     "zh-CN",
			ConfirmLevel: ConfirmDangerous,
			OutputFormat: "text",
		}
	}
	return pref
}

// UpdateUserPreference 更新用户偏好
func (m *Manager) UpdateUserPreference(userID string, pref UserPreference) *UserPreference {
	m.mu.Lock()
	defer m.mu.Unlock()

	pref.UserID = userID
	m.preferences[userID] = &pref
	return &pref
}

// GetStats 获取统计数据
func (m *Manager) GetStats() CopilotStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalMessages := 0
	for _, msgs := range m.messages {
		totalMessages += len(msgs)
	}

	var successRate float64
	if m.executedCmds > 0 {
		successRate = float64(m.successCmds) / float64(m.executedCmds) * 100
	}

	return CopilotStats{
		TotalConversations: len(m.conversations),
		TotalMessages:      totalMessages,
		ExecutedCommands:   m.executedCmds,
		SuccessRate:        successRate,
		AvgResponseTime:    120.5, // 模拟值
	}
}

// AddAuditEntry 添加审计日志
func (m *Manager) AddAuditEntry(userID, operation, command, result, ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &AuditEntry{
		ID:        uuid.New().String(),
		UserID:    userID,
		Operation: operation,
		Command:   command,
		Result:    result,
		Timestamp: time.Now(),
		IPAddress: ip,
	}
	m.auditLog = append(m.auditLog, entry)
}

// ListAuditEntries 获取审计日志
func (m *Manager) ListAuditEntries() []*AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AuditEntry, len(m.auditLog))
	copy(result, m.auditLog)
	return result
}

// containsAny 检查文本是否包含任意关键词
func containsAny(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
