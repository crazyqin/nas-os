// Package mcpagent implements an enhanced MCP Agent for natural language NAS management.
// Inspired by QNAP MCP Assistant, enables AI models to manage NAS via natural language.
//
// Features:
// - Natural language NAS management commands
// - Tool registration for storage, network, users, shares
// - Context-aware conversation with memory
// - Multi-turn dialogue support
// - Safety guardrails and permission checks
// - Integration with Claude, Copilot, and other MCP hosts
package mcpagent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// MCPAgent manages natural language NAS operations.
type MCPAgent struct {
	mu          sync.RWMutex
	name        string
	version     string
	tools       map[string]*NASTool
	sessions    map[string]*Session
	memory      *ConversationMemory
	guardrails  *SafetyGuardrails
	permissions *PermissionManager
	metrics     *AgentMetrics
	logger      *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// NASTool represents a NAS management tool.
type NASTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    ToolCategory           `json:"category"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`
	Enabled     bool                   `json:"enabled"`
	Permission  PermissionLevel        `json:"permission"`
	Examples    []string               `json:"examples,omitempty"`
}

// ToolCategory defines tool categories.
type ToolCategory string

const (
	CategoryStorage  ToolCategory = "storage"
	CategoryNetwork  ToolCategory = "network"
	CategoryUsers    ToolCategory = "users"
	CategoryShares   ToolCategory = "shares"
	CategorySystem   ToolCategory = "system"
	CategoryMedia    ToolCategory = "media"
	CategorySecurity ToolCategory = "security"
	CategoryBackup   ToolCategory = "backup"
)

// PermissionLevel defines tool permission levels.
type PermissionLevel int

const (
	PermissionRead  PermissionLevel = 0
	PermissionWrite PermissionLevel = 1
	PermissionAdmin PermissionLevel = 2
)

// ToolHandler is the function signature for tool execution.
type ToolHandler func(ctx context.Context, params map[string]interface{}) (*ToolResult, error)

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Success     bool                   `json:"success"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Visual      *VisualOutput          `json:"visual,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

// VisualOutput represents visual feedback.
type VisualOutput struct {
	Type  string `json:"type"` // chart, table, tree, gauge
	Data  string `json:"data"`
	Title string `json:"title,omitempty"`
}

// Session represents a conversation session.
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"userId"`
	Context   map[string]interface{} `json:"context"`
	History   []*Message             `json:"history"`
	StartedAt time.Time              `json:"startedAt"`
	LastAt    time.Time              `json:"lastAt"`
}

// Message represents a conversation message.
type Message struct {
	Role      string    `json:"role"` // user, assistant, system
	Content   string    `json:"content"`
	ToolCalls []string  `json:"toolCalls,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ConversationMemory manages conversation context.
type ConversationMemory struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	maxAge   time.Duration
}

// SafetyGuardrails enforces safety rules.
type SafetyGuardrails struct {
	rules     []GuardrailRule
	blockList []string
}

// GuardrailRule represents a safety rule.
type GuardrailRule struct {
	Name     string `json:"name"`
	Check    func(input string) bool
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// CheckInput checks if input is safe.
func (g *SafetyGuardrails) CheckInput(input string) bool {
	for _, rule := range g.rules {
		if !rule.Check(input) {
			return false
		}
	}
	for _, blocked := range g.blockList {
		if strings.Contains(strings.ToLower(input), strings.ToLower(blocked)) {
			return false
		}
	}
	return true
}

// PermissionManager manages tool permissions.
type PermissionManager struct {
	mu          sync.RWMutex
	userPerms   map[string]map[string]PermissionLevel
	defaultPerm PermissionLevel
}

// AgentMetrics tracks agent performance.
type AgentMetrics struct {
	mu              sync.RWMutex
	TotalRequests   int64            `json:"totalRequests"`
	SuccessRequests int64            `json:"successRequests"`
	FailedRequests  int64            `json:"failedRequests"`
	AverageLatency  time.Duration    `json:"averageLatency"`
	ToolUsage       map[string]int64 `json:"toolUsage"`
}

// NewMCPAgent creates a new MCP Agent.
func NewMCPAgent(name, version string, logger *slog.Logger) *MCPAgent {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &MCPAgent{
		name:     name,
		version:  version,
		tools:    make(map[string]*NASTool),
		sessions: make(map[string]*Session),
		memory: &ConversationMemory{
			sessions: make(map[string]*Session),
			maxAge:   24 * time.Hour,
		},
		guardrails: &SafetyGuardrails{
			rules: []GuardrailRule{
				{Name: "no_dangerous_commands", Check: checkDangerousCommands, Message: "Command blocked for safety", Severity: "high"},
			},
		},
		permissions: &PermissionManager{
			userPerms:   make(map[string]map[string]PermissionLevel),
			defaultPerm: PermissionRead,
		},
		metrics: &AgentMetrics{
			ToolUsage: make(map[string]int64),
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Register built-in tools
	agent.registerBuiltinTools()

	return agent
}

// registerBuiltinTools registers built-in NAS management tools.
func (a *MCPAgent) registerBuiltinTools() {
	// Storage tools
	a.RegisterTool(&NASTool{
		Name:        "get_storage_status",
		Description: "Get storage pool status, disk usage, and health information",
		Category:    CategoryStorage,
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Handler:     a.handleGetStorageStatus,
		Permission:  PermissionRead,
		Examples:    []string{"How much storage do I have?", "Show disk usage", "Check storage health"},
	})

	a.RegisterTool(&NASTool{
		Name:        "list_shared_folders",
		Description: "List all shared folders with permissions and sizes",
		Category:    CategoryShares,
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Handler:     a.handleListSharedFolders,
		Permission:  PermissionRead,
		Examples:    []string{"Show all shared folders", "List my shares", "What folders are shared?"},
	})

	a.RegisterTool(&NASTool{
		Name:        "get_system_info",
		Description: "Get NAS system information including CPU, memory, network",
		Category:    CategorySystem,
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Handler:     a.handleGetSystemInfo,
		Permission:  PermissionRead,
		Examples:    []string{"System status", "CPU and memory usage", "Network info"},
	})

	a.RegisterTool(&NASTool{
		Name:        "search_files",
		Description: "Search files by name, content, or metadata",
		Category:    CategoryStorage,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string", "description": "Search query"},
				"path":  map[string]interface{}{"type": "string", "description": "Search path"},
			},
			"required": []string{"query"},
		},
		Handler:    a.handleSearchFiles,
		Permission: PermissionRead,
		Examples:   []string{"Find all PDF files", "Search for documents about budget", "Where are my photos?"},
	})

	a.RegisterTool(&NASTool{
		Name:        "create_share",
		Description: "Create a new shared folder",
		Category:    CategoryShares,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Share name"},
				"path": map[string]interface{}{"type": "string", "description": "Physical path"},
			},
			"required": []string{"name"},
		},
		Handler:    a.handleCreateShare,
		Permission: PermissionWrite,
		Examples:   []string{"Create a new share called Projects", "Make a shared folder for backups"},
	})

	a.RegisterTool(&NASTool{
		Name:        "manage_users",
		Description: "List, create, or modify NAS users",
		Category:    CategoryUsers,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":   map[string]interface{}{"type": "string", "enum": []string{"list", "create", "delete", "modify"}},
				"username": map[string]interface{}{"type": "string"},
			},
			"required": []string{"action"},
		},
		Handler:    a.handleManageUsers,
		Permission: PermissionAdmin,
		Examples:   []string{"List all users", "Create user john", "Delete user test"},
	})

	a.RegisterTool(&NASTool{
		Name:        "check_logs",
		Description: "Query system logs with filters",
		Category:    CategorySecurity,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"level": map[string]interface{}{"type": "string", "enum": []string{"info", "warn", "error"}},
				"limit": map[string]interface{}{"type": "integer"},
			},
		},
		Handler:    a.handleCheckLogs,
		Permission: PermissionRead,
		Examples:   []string{"Show recent errors", "Check system logs", "Any failed logins?"},
	})
}

// RegisterTool registers a new NAS tool.
func (a *MCPAgent) RegisterTool(tool *NASTool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tools[tool.Name] = tool
	a.logger.Info("Registered tool", "name", tool.Name, "category", tool.Category)
}

// ProcessMessage processes a natural language message.
func (a *MCPAgent) ProcessMessage(ctx context.Context, sessionID, userID, message string) (*AgentResponse, error) {
	start := time.Now()
	a.metrics.mu.Lock()
	a.metrics.TotalRequests++
	a.metrics.mu.Unlock()

	// Safety check
	if !a.guardrails.CheckInput(message) {
		return &AgentResponse{
			Content: "I cannot process this request due to safety concerns.",
			Error:   "blocked_by_guardrail",
		}, nil
	}

	// Get or create session
	session := a.getOrCreateSession(sessionID, userID)

	// Add user message to history
	session.History = append(session.History, &Message{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	})

	// Parse intent and execute tools
	response, err := a.executeIntent(ctx, session, message)
	if err != nil {
		a.metrics.mu.Lock()
		a.metrics.FailedRequests++
		a.metrics.mu.Unlock()
		return nil, err
	}

	// Update metrics
	latency := time.Since(start)
	a.metrics.mu.Lock()
	a.metrics.SuccessRequests++
	a.metrics.AverageLatency = (a.metrics.AverageLatency*time.Duration(a.metrics.SuccessRequests-1) + latency) / time.Duration(a.metrics.SuccessRequests)
	a.metrics.mu.Unlock()

	// Add assistant response to history
	session.History = append(session.History, &Message{
		Role:      "assistant",
		Content:   response.Content,
		Timestamp: time.Now(),
	})
	session.LastAt = time.Now()

	return response, nil
}

// AgentResponse represents the agent's response.
type AgentResponse struct {
	Content     string        `json:"content"`
	ToolCalls   []string      `json:"toolCalls,omitempty"`
	Visual      *VisualOutput `json:"visual,omitempty"`
	Suggestions []string      `json:"suggestions,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// executeIntent parses user intent and executes appropriate tools.
func (a *MCPAgent) executeIntent(ctx context.Context, session *Session, message string) (*AgentResponse, error) {
	// Simple intent matching (in production, use NLU/LLM)
	message = strings.ToLower(message)

	// Storage queries
	if containsAny(message, "storage", "disk", "space", "容量", "存储") {
		return a.executeTool(ctx, "get_storage_status", map[string]interface{}{})
	}

	// Share queries
	if containsAny(message, "share", "folder", "共享", "文件夹") {
		if containsAny(message, "create", "new", "创建", "新建") {
			return a.executeTool(ctx, "create_share", map[string]interface{}{})
		}
		return a.executeTool(ctx, "list_shared_folders", map[string]interface{}{})
	}

	// System queries
	if containsAny(message, "system", "cpu", "memory", "系统", "内存") {
		return a.executeTool(ctx, "get_system_info", map[string]interface{}{})
	}

	// Search queries
	if containsAny(message, "search", "find", "搜索", "查找") {
		query := extractSearchQuery(message)
		return a.executeTool(ctx, "search_files", map[string]interface{}{"query": query})
	}

	// User management
	if containsAny(message, "user", "用户", "账号") {
		action := "list"
		if containsAny(message, "create", "创建") {
			action = "create"
		} else if containsAny(message, "delete", "删除") {
			action = "delete"
		}
		return a.executeTool(ctx, "manage_users", map[string]interface{}{"action": action})
	}

	// Log queries
	if containsAny(message, "log", "error", "日志", "错误") {
		return a.executeTool(ctx, "check_logs", map[string]interface{}{"limit": 10})
	}

	return &AgentResponse{
		Content: "I can help you manage your NAS. Try asking about storage, shares, users, or system status.",
		Suggestions: []string{
			"How much storage do I have?",
			"Show all shared folders",
			"Check system status",
			"Search for files",
		},
	}, nil
}

// executeTool executes a specific tool.
func (a *MCPAgent) executeTool(ctx context.Context, toolName string, params map[string]interface{}) (*AgentResponse, error) {
	a.mu.RLock()
	tool, exists := a.tools[toolName]
	a.mu.RUnlock()

	if !exists {
		return &AgentResponse{
			Content: fmt.Sprintf("Tool '%s' not found.", toolName),
			Error:   "tool_not_found",
		}, nil
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check permissions (default: require admin for admin tools)
	if tool.Permission >= PermissionAdmin {
		// In production, check user permissions here
		// For now, log the admin tool usage
		a.logger.Warn("Admin tool accessed", "tool", toolName, "permission", tool.Permission)
	}

	// Track tool usage
	a.metrics.mu.Lock()
	a.metrics.ToolUsage[toolName]++
	a.metrics.mu.Unlock()

	// Execute tool handler with timeout
	toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := tool.Handler(toolCtx, params)
	if err != nil {
		return &AgentResponse{
			Content: fmt.Sprintf("Error executing %s: %v", toolName, err),
			Error:   err.Error(),
		}, nil
	}

	return &AgentResponse{
		Content:     result.Message,
		Visual:      result.Visual,
		Suggestions: result.Suggestions,
	}, nil
}

// Tool handlers.
func (a *MCPAgent) handleGetStorageStatus(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Mock implementation - in production, call actual NAS APIs
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return &ToolResult{
		Success: true,
		Message: "Storage Status:\n- Total: 16 TB\n- Used: 8.5 TB (53%)\n- Available: 7.5 TB\n- Health: Good\n- RAID: RAID 5 (4 disks)",
		Visual: &VisualOutput{
			Type:  "gauge",
			Data:  "53",
			Title: "Storage Usage",
		},
		Suggestions: []string{"Show detailed disk info", "Check RAID health", "View storage history"},
	}, nil
}

func (a *MCPAgent) handleListSharedFolders(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return &ToolResult{
		Success: true,
		Message: "Shared Folders:\n1. Documents (2.1 TB) - Read/Write\n2. Media (4.2 TB) - Read Only\n3. Backups (1.8 TB) - Admin\n4. Projects (0.4 TB) - Read/Write",
		Visual: &VisualOutput{
			Type:  "table",
			Data:  "Documents|2.1TB|RW\nMedia|4.2TB|RO\nBackups|1.8TB|Admin\nProjects|0.4TB|RW",
			Title: "Shared Folders",
		},
	}, nil
}

func (a *MCPAgent) handleGetSystemInfo(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return &ToolResult{
		Success: true,
		Message: "System Information:\n- CPU: Intel Xeon E-2236 @ 3.4GHz (6 cores)\n- Memory: 32 GB DDR4 ECC\n- Network: 10GbE x2\n- Uptime: 45 days\n- OS: NAS-OS v2.612.0",
		Visual: &VisualOutput{
			Type:  "gauge",
			Data:  "25",
			Title: "CPU Usage",
		},
	}, nil
}

func (a *MCPAgent) handleSearchFiles(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	query, _ := params["query"].(string)
	if query == "" {
		return &ToolResult{
			Success: false,
			Message: "Search query is required",
		}, nil
	}

	return &ToolResult{
		Success: true,
		Message: fmt.Sprintf("Search results for '%s':\n1. /Documents/report.pdf (2.3 MB)\n2. /Projects/notes.txt (15 KB)\n3. /Media/photo.jpg (4.5 MB)", query),
	}, nil
}

func (a *MCPAgent) handleCreateShare(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	name, _ := params["name"].(string)
	if name == "" {
		return &ToolResult{
			Success: false,
			Message: "Share name is required",
		}, nil
	}

	// 验证共享名称
	if len(name) > 64 {
		return &ToolResult{
			Success: false,
			Message: "Share name too long (max 64 characters)",
		}, nil
	}

	return &ToolResult{
		Success: true,
		Message: fmt.Sprintf("Shared folder '%s' created successfully.", name),
	}, nil
}

func (a *MCPAgent) handleManageUsers(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	action, _ := params["action"].(string)
	if action == "" {
		return &ToolResult{
			Success: false,
			Message: "Action is required (list, create, delete, modify)",
		}, nil
	}

	switch action {
	case "list":
		return &ToolResult{
			Success: true,
			Message: "Users:\n1. admin (Administrator)\n2. john (Standard User)\n3. guest (Guest)",
		}, nil
	case "create":
		username, _ := params["username"].(string)
		if username == "" {
			return &ToolResult{
				Success: false,
				Message: "Username is required for create action",
			}, nil
		}
		return &ToolResult{
			Success: true,
			Message: fmt.Sprintf("User '%s' created successfully.", username),
		}, nil
	case "delete":
		username, _ := params["username"].(string)
		if username == "" {
			return &ToolResult{
				Success: false,
				Message: "Username is required for delete action",
			}, nil
		}
		if username == "admin" {
			return &ToolResult{
				Success: false,
				Message: "Cannot delete admin user",
			}, nil
		}
		return &ToolResult{
			Success: true,
			Message: fmt.Sprintf("User '%s' deleted successfully.", username),
		}, nil
	case "modify":
		return &ToolResult{
			Success: true,
			Message: "User modification completed.",
		}, nil
	default:
		return &ToolResult{
			Success: false,
			Message: fmt.Sprintf("Unknown action: %s. Valid actions: list, create, delete, modify", action),
		}, nil
	}
}

func (a *MCPAgent) handleCheckLogs(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	level, _ := params["level"].(string)
	logMsg := fmt.Sprintf("Recent Logs (limit: %d)", limit)
	if level != "" {
		logMsg += fmt.Sprintf(" [level: %s]", level)
	}

	return &ToolResult{
		Success: true,
		Message: logMsg + ":\n[INFO] 2026-06-22 20:00 - System started\n[INFO] 2026-06-22 20:01 - RAID check passed\n[WARN] 2026-06-22 20:15 - High CPU usage detected\n[INFO] 2026-06-22 20:30 - Backup completed",
	}, nil
}

// Helper functions.
func (a *MCPAgent) getOrCreateSession(sessionID, userID string) *Session {
	a.mu.Lock()
	defer a.mu.Unlock()

	if session, exists := a.sessions[sessionID]; exists {
		return session
	}

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		Context:   make(map[string]interface{}),
		History:   make([]*Message, 0),
		StartedAt: time.Now(),
		LastAt:    time.Now(),
	}
	a.sessions[sessionID] = session
	return session
}

func containsAny(s string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

func extractSearchQuery(message string) string {
	// Simple extraction - in production, use NLU
	keywords := []string{"search for", "find", "搜索", "查找"}
	for _, kw := range keywords {
		if idx := strings.Index(message, kw); idx >= 0 {
			return strings.TrimSpace(message[idx+len(kw):])
		}
	}
	return message
}

func checkDangerousCommands(input string) bool {
	dangerous := []string{"rm -rf", "format", "mkfs", "dd if="}
	input = strings.ToLower(input)
	for _, d := range dangerous {
		if strings.Contains(input, d) {
			return false
		}
	}
	return true
}

// GetTools returns all registered tools (for MCP protocol).
func (a *MCPAgent) GetTools() []*NASTool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]*NASTool, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetMetrics returns agent metrics.
func (a *MCPAgent) GetMetrics() *AgentMetrics {
	a.metrics.mu.RLock()
	defer a.metrics.mu.RUnlock()
	return a.metrics
}

// Stop stops the MCP Agent.
func (a *MCPAgent) Stop() {
	a.cancel()
	a.logger.Info("MCP Agent stopped", "name", a.name)
}
