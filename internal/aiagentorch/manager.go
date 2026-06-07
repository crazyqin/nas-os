// Package aiagentorch - 代理编排管理器实现
package aiagentorch

import (
	"fmt"
	"sync"
	"time"
)

// Manager 代理编排管理器.
type Manager struct {
	mu        sync.RWMutex
	agents    map[string]*Agent
	tasks     map[string]*AgentTask
	logs      []*ExecutionLog
	messages  []*AgentMessage
	nameIndex map[string]string // name -> agentID
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		agents:    make(map[string]*Agent),
		tasks:     make(map[string]*AgentTask),
		nameIndex: make(map[string]string),
	}
}

// ========== 代理 CRUD ==========

// CreateAgent 创建代理.
func (m *Manager) CreateAgent(agent *Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if agent.Name == "" || agent.Type == "" {
		return ErrInvalidConfig
	}
	if _, exists := m.nameIndex[agent.Name]; exists {
		return ErrAgentNameExists
	}

	agent.ID = fmt.Sprintf("agent-%d", time.Now().UnixNano())
	agent.Status = AgentStatusInactive
	if agent.Config.Parameters == nil {
		agent.Config.Parameters = make(map[string]string)
	}
	if agent.Tags == nil {
		agent.Tags = []string{}
	}
	if agent.EventTriggers == nil {
		agent.EventTriggers = []EventTrigger{}
	}
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	m.agents[agent.ID] = agent
	m.nameIndex[agent.Name] = agent.ID
	return nil
}

// GetAgent 获取代理.
func (m *Manager) GetAgent(id string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agent, ok := m.agents[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

// ListAgents 列出代理.
func (m *Manager) ListAgents(agentType AgentType, status AgentStatus, page, pageSize int) ([]Agent, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Agent
	for _, agent := range m.agents {
		if agentType != "" && agent.Type != agentType {
			continue
		}
		if status != "" && agent.Status != status {
			continue
		}
		result = append(result, *agent)
	}

	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// UpdateAgent 更新代理.
func (m *Manager) UpdateAgent(id string, update *Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.agents[id]
	if !ok {
		return ErrAgentNotFound
	}

	// 检查名称冲突
	if update.Name != "" && update.Name != existing.Name {
		if _, exists := m.nameIndex[update.Name]; exists {
			return ErrAgentNameExists
		}
		delete(m.nameIndex, existing.Name)
		existing.Name = update.Name
		m.nameIndex[existing.Name] = id
	}

	if update.Description != "" {
		existing.Description = update.Description
	}
	if update.Type != "" {
		existing.Type = update.Type
	}
	if update.Config.ModelName != "" {
		existing.Config.ModelName = update.Config.ModelName
	}
	if update.Config.ModelEndpoint != "" {
		existing.Config.ModelEndpoint = update.Config.ModelEndpoint
	}
	if update.Config.Temperature > 0 {
		existing.Config.Temperature = update.Config.Temperature
	}
	if update.Config.MaxTokens > 0 {
		existing.Config.MaxTokens = update.Config.MaxTokens
	}
	if update.Config.SystemPrompt != "" {
		existing.Config.SystemPrompt = update.Config.SystemPrompt
	}
	if update.Config.Parameters != nil {
		existing.Config.Parameters = update.Config.Parameters
	}
	if update.Permission.AllowedPaths != nil {
		existing.Permission.AllowedPaths = update.Permission.AllowedPaths
	}
	if update.Permission.DeniedPaths != nil {
		existing.Permission.DeniedPaths = update.Permission.DeniedPaths
	}
	if update.CronExpr != "" {
		existing.CronExpr = update.CronExpr
	}
	if update.TriggerType != "" {
		existing.TriggerType = update.TriggerType
	}
	if update.EventTriggers != nil {
		existing.EventTriggers = update.EventTriggers
	}
	if update.Tags != nil {
		existing.Tags = update.Tags
	}

	existing.Enabled = update.Enabled
	existing.Permission.ReadOnly = update.Permission.ReadOnly
	if update.Permission.MaxConcurrent > 0 {
		existing.Permission.MaxConcurrent = update.Permission.MaxConcurrent
	}
	if update.Permission.RateLimitPerMin > 0 {
		existing.Permission.RateLimitPerMin = update.Permission.RateLimitPerMin
	}
	existing.UpdatedAt = time.Now()
	return nil
}

// DeleteAgent 删除代理.
func (m *Manager) DeleteAgent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[id]
	if !ok {
		return ErrAgentNotFound
	}

	// 删除关联任务
	for tid, task := range m.tasks {
		if task.AgentID == id {
			delete(m.tasks, tid)
		}
	}

	delete(m.nameIndex, agent.Name)
	delete(m.agents, id)
	return nil
}

// EnableAgent 启用代理.
func (m *Manager) EnableAgent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[id]
	if !ok {
		return ErrAgentNotFound
	}
	agent.Enabled = true
	agent.Status = AgentStatusActive
	agent.UpdatedAt = time.Now()
	return nil
}

// DisableAgent 停用代理.
func (m *Manager) DisableAgent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[id]
	if !ok {
		return ErrAgentNotFound
	}
	agent.Enabled = false
	agent.Status = AgentStatusInactive
	agent.UpdatedAt = time.Now()
	return nil
}

// ========== 任务管理 ==========

// CreateTask 创建任务.
func (m *Manager) CreateTask(task *AgentTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.agents[task.AgentID]; !ok {
		return ErrAgentNotFound
	}
	if task.Name == "" {
		return ErrInvalidConfig
	}

	task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	task.RunCount = 0
	task.CreatedAt = time.Now()
	m.tasks[task.ID] = task
	return nil
}

// GetTask 获取任务.
func (m *Manager) GetTask(id string) (*AgentTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出任务.
func (m *Manager) ListTasks(agentID string) []AgentTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []AgentTask
	for _, task := range m.tasks {
		if agentID == "" || task.AgentID == agentID {
			result = append(result, *task)
		}
	}
	return result
}

// DeleteTask 删除任务.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return ErrTaskNotFound
	}
	delete(m.tasks, id)
	return nil
}

// ========== 执行管理 ==========

// ExecuteAgent 手动执行代理.
func (m *Manager) ExecuteAgent(agentID string) (*ExecutionLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return nil, ErrAgentNotFound
	}
	if !agent.Enabled {
		return nil, ErrAgentNotActive
	}

	agent.Status = AgentStatusRunning
	now := time.Now()
	agent.LastRun = &now

	log := &ExecutionLog{
		ID:          fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		AgentID:     agentID,
		AgentName:   agent.Name,
		TriggerType: TriggerManual,
		Status:      ExecRunning,
		StartTime:   now,
		Details:     make(map[string]string),
	}
	m.logs = append(m.logs, log)

	// 模拟执行完成
	endTime := time.Now()
	log.EndTime = &endTime
	log.DurationMs = endTime.Sub(log.StartTime).Milliseconds()
	log.Status = ExecSuccess
	log.Result = "execution completed"
	agent.Status = AgentStatusActive
	agent.RunCount++

	return log, nil
}

// ExecuteTask 执行指定任务.
func (m *Manager) ExecuteTask(taskID string) (*ExecutionLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	agent, ok := m.agents[task.AgentID]
	if !ok {
		return nil, ErrAgentNotFound
	}
	if !agent.Enabled {
		return nil, ErrAgentNotActive
	}

	agent.Status = AgentStatusRunning
	now := time.Now()
	agent.LastRun = &now
	task.LastRun = &now
	task.RunCount++

	log := &ExecutionLog{
		ID:          fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		AgentID:     agent.ID,
		AgentName:   agent.Name,
		TaskID:      taskID,
		TriggerType: task.TriggerType,
		Status:      ExecRunning,
		StartTime:   now,
		Details:     make(map[string]string),
	}
	m.logs = append(m.logs, log)

	// 模拟执行完成
	endTime := time.Now()
	log.EndTime = &endTime
	log.DurationMs = endTime.Sub(log.StartTime).Milliseconds()
	log.Status = ExecSuccess
	log.Result = fmt.Sprintf("task '%s' completed", task.Name)
	agent.Status = AgentStatusActive
	agent.RunCount++

	return log, nil
}

// CancelExecution 取消执行.
func (m *Manager) CancelExecution(logID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, log := range m.logs {
		if log.ID == logID {
			if log.Status == ExecRunning {
				log.Status = ExecCancelled
				endTime := time.Now()
				log.EndTime = &endTime
				log.DurationMs = endTime.Sub(log.StartTime).Milliseconds()
				return nil
			}
			return fmt.Errorf("execution is not running")
		}
	}
	return fmt.Errorf("execution log not found")
}

// ========== 日志查询 ==========

// GetExecutionLogs 获取执行日志.
func (m *Manager) GetExecutionLogs(agentID string, status ExecutionStatus, page, pageSize int) ([]ExecutionLog, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ExecutionLog
	for _, log := range m.logs {
		if agentID != "" && log.AgentID != agentID {
			continue
		}
		if status != "" && log.Status != status {
			continue
		}
		result = append(result, *log)
	}

	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// GetExecutionLog 获取单条执行日志.
func (m *Manager) GetExecutionLog(id string) (*ExecutionLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, log := range m.logs {
		if log.ID == id {
			return log, nil
		}
	}
	return nil, fmt.Errorf("execution log not found")
}

// ========== 消息传递 ==========

// SendMessage 发送代理间消息.
func (m *Manager) SendMessage(fromID, toID, msgType, content string, priority MessagePriority) (*AgentMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.agents[fromID]; !ok {
		return nil, ErrAgentNotFound
	}
	if _, ok := m.agents[toID]; !ok {
		return nil, ErrAgentNotFound
	}

	msg := &AgentMessage{
		ID:          fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		FromAgentID: fromID,
		ToAgentID:   toID,
		MessageType: msgType,
		Content:     content,
		Priority:    priority,
		Read:        false,
		CreatedAt:   time.Now(),
	}
	m.messages = append(m.messages, msg)
	return msg, nil
}

// GetMessages 获取代理消息.
func (m *Manager) GetMessages(agentID string, unreadOnly bool, page, pageSize int) ([]AgentMessage, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AgentMessage
	for _, msg := range m.messages {
		if agentID != "" && msg.ToAgentID != agentID && msg.FromAgentID != agentID {
			continue
		}
		if unreadOnly && msg.Read {
			continue
		}
		result = append(result, *msg)
	}

	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// MarkMessageRead 标记消息已读.
func (m *Manager) MarkMessageRead(msgID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.messages {
		if msg.ID == msgID {
			msg.Read = true
			return nil
		}
	}
	return ErrMessageNotFound
}

// ========== 统计 ==========

// GetStats 获取编排器统计.
func (m *Manager) GetStats() OrchStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := OrchStats{
		AgentsByType: make(map[AgentType]int),
	}

	for _, agent := range m.agents {
		stats.TotalAgents++
		stats.AgentsByType[agent.Type]++
		switch agent.Status {
		case AgentStatusActive:
			stats.ActiveAgents++
		case AgentStatusRunning:
			stats.RunningAgents++
		case AgentStatusError:
			stats.ErrorAgents++
		}
	}

	stats.TotalTasks = len(m.tasks)

	for _, log := range m.logs {
		stats.TotalExecutions++
		switch log.Status {
		case ExecSuccess:
			stats.SuccessExecutions++
		case ExecFailed:
			stats.FailedExecutions++
		}
	}

	for _, msg := range m.messages {
		if !msg.Read {
			stats.UnreadMessages++
		}
	}

	return stats
}

// GetAgentCount 获取代理数量.
func (m *Manager) GetAgentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agents)
}

// GetTaskCount 获取任务数量.
func (m *Manager) GetTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tasks)
}
