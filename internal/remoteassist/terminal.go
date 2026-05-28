// terminal.go - 安全终端接入
package remoteassist

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TerminalManager 终端管理器.
type TerminalManager struct {
	sessions map[string]*TerminalSession
	commands map[string][]*TerminalCommand
	mu       sync.RWMutex
}

// NewTerminalManager 创建终端管理器.
func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*TerminalSession),
		commands: make(map[string][]*TerminalCommand),
	}
}

// CreateTerminal 创建终端会话.
func (m *TerminalManager) CreateTerminal(sessionID string, options *TerminalOptions) (*TerminalSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有终端
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("会话已有终端: %s", sessionID)
	}

	shell := options.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	term := &TerminalSession{
		ID:          uuid.New().String(),
		SessionID:   sessionID,
		Shell:       shell,
		Rows:        options.Rows,
		Cols:        options.Cols,
		WorkingDir:  options.WorkingDir,
		Env:         options.Env,
		Status:      "active",
		StartedAt:   time.Now(),
		LastInputAt: time.Now(),
	}

	m.sessions[sessionID] = term
	m.commands[sessionID] = make([]*TerminalCommand, 0)

	log.Printf("💻 创建终端会话: %s, Shell: %s", term.ID, shell)
	return term, nil
}

// TerminalOptions 终端选项.
type TerminalOptions struct {
	Shell      string            `json:"shell"`       // Shell类型
	Rows       int               `json:"rows"`        // 行数
	Cols       int               `json:"cols"`        // 列数
	WorkingDir string            `json:"working_dir"` // 工作目录
	Env        map[string]string `json:"env"`         // 环境变量
}

// CloseTerminal 关闭终端会话.
func (m *TerminalManager) CloseTerminal(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	term, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("终端会话不存在: %s", sessionID)
	}

	term.Status = "closed"
	delete(m.sessions, sessionID)

	log.Printf("💻 关闭终端会话: %s", term.ID)
	return nil
}

// GetTerminal 获取终端会话.
func (m *TerminalManager) GetTerminal(sessionID string) (*TerminalSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	term, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("终端会话不存在: %s", sessionID)
	}
	return term, nil
}

// ExecuteCommand 执行命令.
func (m *TerminalManager) ExecuteCommand(sessionID string, command string) (*TerminalCommand, error) {
	m.mu.RLock()
	term, exists := m.sessions[sessionID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("终端会话不存在: %s", sessionID)
	}
	m.mu.RUnlock()

	if term.Status != "active" {
		return nil, fmt.Errorf("终端会话未激活: %s", term.Status)
	}

	start := time.Now()

	// 执行命令（简化实现）
	output, exitCode := executeShellCommand(term.Shell, command)

	cmd := &TerminalCommand{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Command:   command,
		Output:    output,
		ExitCode:  exitCode,
		StartedAt: start,
		EndedAt:   time.Now(),
		Duration:  time.Since(start).Milliseconds(),
	}

	m.mu.Lock()
	m.commands[sessionID] = append(m.commands[sessionID], cmd)
	term.LastInputAt = time.Now()
	m.mu.Unlock()

	log.Printf("💻 执行命令: %s, 退出码: %d", command, exitCode)
	return cmd, nil
}

// executeShellCommand 执行Shell命令.
func executeShellCommand(shell, command string) (string, int) {
	// 这里简化实现，实际应该使用 os/exec
	// 返回模拟输出
	return fmt.Sprintf("命令已执行: %s", command), 0
}

// GetCommandHistory 获取命令历史.
func (m *TerminalManager) GetCommandHistory(sessionID string, limit int) ([]*TerminalCommand, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	commands, exists := m.commands[sessionID]
	if !exists {
		return nil, fmt.Errorf("终端会话不存在: %s", sessionID)
	}

	if limit <= 0 || limit > len(commands) {
		limit = len(commands)
	}

	return commands[len(commands)-limit:], nil
}

// ResizeTerminal 调整终端大小.
func (m *TerminalManager) ResizeTerminal(sessionID string, rows, cols int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	term, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("终端会话不存在: %s", sessionID)
	}

	term.Rows = rows
	term.Cols = cols

	log.Printf("💻 调整终端大小: %s -> %dx%d", sessionID, cols, rows)
	return nil
}

// SendInput 发送输入.
func (m *TerminalManager) SendInput(sessionID string, input string) error {
	m.mu.RLock()
	term, exists := m.sessions[sessionID]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("终端会话不存在: %s", sessionID)
	}
	m.mu.RUnlock()

	if term.Status != "active" {
		return fmt.Errorf("终端会话未激活: %s", term.Status)
	}

	// 处理输入
	log.Printf("💻 发送输入: %s, 长度: %d", sessionID, len(input))

	m.mu.Lock()
	term.LastInputAt = time.Now()
	m.mu.Unlock()

	return nil
}

// GetOutput 获取输出.
func (m *TerminalManager) GetOutput(sessionID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.sessions[sessionID]
	if !exists {
		return "", fmt.Errorf("终端会话不存在: %s", sessionID)
	}

	// 返回最近的输出
	commands := m.commands[sessionID]
	if len(commands) > 0 {
		return commands[len(commands)-1].Output, nil
	}

	return "", nil
}

// ListTerminals 列出所有终端.
func (m *TerminalManager) ListTerminals() []*TerminalSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TerminalSession, 0, len(m.sessions))
	for _, term := range m.sessions {
		result = append(result, term)
	}
	return result
}

// GetTerminalStats 获取终端统计.
func (m *TerminalManager) GetTerminalStats(sessionID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	term, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("终端会话不存在: %s", sessionID)
	}

	commands := m.commands[sessionID]
	totalDuration := int64(0)
	for _, cmd := range commands {
		totalDuration += cmd.Duration
	}

	stats := map[string]interface{}{
		"session_id":      sessionID,
		"terminal_id":     term.ID,
		"shell":           term.Shell,
		"status":          term.Status,
		"rows":            term.Rows,
		"cols":            term.Cols,
		"working_dir":     term.WorkingDir,
		"command_count":   len(commands),
		"total_duration":  totalDuration,
		"uptime":          time.Since(term.StartedAt).Seconds(),
		"last_input_ago":  time.Since(term.LastInputAt).Seconds(),
	}

	return stats, nil
}

// SetEnvironment 设置环境变量.
func (m *TerminalManager) SetEnvironment(sessionID string, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	term, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("终端会话不存在: %s", sessionID)
	}

	if term.Env == nil {
		term.Env = make(map[string]string)
	}
	term.Env[key] = value

	log.Printf("💻 设置环境变量: %s, %s=%s", sessionID, key, value)
	return nil
}

// ChangeDirectory 切换目录.
func (m *TerminalManager) ChangeDirectory(sessionID string, dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	term, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("终端会话不存在: %s", sessionID)
	}

	term.WorkingDir = dir

	log.Printf("💻 切换目录: %s -> %s", sessionID, dir)
	return nil
}
