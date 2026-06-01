package systemcopilot

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages the SystemCopilot business logic
type Manager struct {
	mu            sync.RWMutex
	commands      map[string]*Command
	results       map[string]*CommandResult
	sessions      map[string]*CopilotSession
	startTime     time.Time
	ruleEngine    *RuleEngine
}

// RuleEngine handles keyword matching and command parsing
type RuleEngine struct {
	patterns []commandPattern
}

type commandPattern struct {
	keywords    []string
	commandType CommandType
	action      string
	sensitivity SensitivityLevel
	needsConfirm bool
}

// NewManager creates a new SystemCopilot manager
func NewManager() *Manager {
	return &Manager{
		commands:  make(map[string]*Command),
		results:   make(map[string]*CommandResult),
		sessions:  make(map[string]*CopilotSession),
		startTime: time.Now(),
		ruleEngine: newRuleEngine(),
	}
}

func newRuleEngine() *RuleEngine {
	return &RuleEngine{
		patterns: []commandPattern{
			// Service management
			{keywords: []string{"重启", "restart", "服务", "service"}, commandType: CommandTypeService, action: "restart", sensitivity: SensitivityMedium, needsConfirm: true},
			{keywords: []string{"启动", "start", "开启"}, commandType: CommandTypeService, action: "start", sensitivity: SensitivityLow, needsConfirm: false},
			{keywords: []string{"停止", "stop", "关闭"}, commandType: CommandTypeService, action: "stop", sensitivity: SensitivityMedium, needsConfirm: true},
			{keywords: []string{"状态", "status", "查看"}, commandType: CommandTypeService, action: "status", sensitivity: SensitivityLow, needsConfirm: false},

			// Network
			{keywords: []string{"网络", "network", "ip", "接口"}, commandType: CommandTypeNetwork, action: "info", sensitivity: SensitivityLow, needsConfirm: false},
			{keywords: []string{"防火墙", "firewall", "iptables", "端口", "port"}, commandType: CommandTypeFirewall, action: "manage", sensitivity: SensitivityHigh, needsConfirm: true},

			// Storage
			{keywords: []string{"磁盘", "disk", "存储", "storage", "空间"}, commandType: CommandTypeStorage, action: "info", sensitivity: SensitivityLow, needsConfirm: false},
			{keywords: []string{"清理", "clean", "cache", "缓存"}, commandType: CommandTypeStorage, action: "clean", sensitivity: SensitivityMedium, needsConfirm: true},

			// Docker
			{keywords: []string{"docker", "容器", "container"}, commandType: CommandTypeDocker, action: "manage", sensitivity: SensitivityMedium, needsConfirm: true},
			{keywords: []string{"docker", "ps", "列表", "list"}, commandType: CommandTypeDocker, action: "list", sensitivity: SensitivityLow, needsConfirm: false},

			// System
			{keywords: []string{"系统", "system", "信息", "info", "概览"}, commandType: CommandTypeSystem, action: "info", sensitivity: SensitivityLow, needsConfirm: false},
			{keywords: []string{"重启", "reboot", "shutdown", "关机"}, commandType: CommandTypeSystem, action: "reboot", sensitivity: SensitivityCritical, needsConfirm: true},
			{keywords: []string{"更新", "update", "upgrade", "升级"}, commandType: CommandTypeSystem, action: "update", sensitivity: SensitivityHigh, needsConfirm: true},
			{keywords: []string{"日志", "log", "查看"}, commandType: CommandTypeSystem, action: "logs", sensitivity: SensitivityLow, needsConfirm: false},

			// User management
			{keywords: []string{"用户", "user", "账号"}, commandType: CommandTypeUser, action: "manage", sensitivity: SensitivityHigh, needsConfirm: true},

			// Backup
			{keywords: []string{"备份", "backup", "恢复", "restore"}, commandType: CommandTypeBackup, action: "manage", sensitivity: SensitivityHigh, needsConfirm: true},

			// Monitor
			{keywords: []string{"监控", "monitor", "性能", "performance"}, commandType: CommandTypeMonitor, action: "info", sensitivity: SensitivityLow, needsConfirm: false},
		},
	}
}

// ProcessCommand parses user input and creates a command
func (m *Manager) ProcessCommand(input string, sessionID string) (*Command, *CommandResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse the input
	cmd := m.ruleEngine.ParseInput(input)

	// Generate ID
	cmd.ID = uuid.New().String()
	cmd.CreatedAt = time.Now()

	// Store command
	m.commands[cmd.ID] = cmd

	// Add to session
	if sessionID != "" {
		session, exists := m.sessions[sessionID]
		if !exists {
			session = &CopilotSession{
				ID:        sessionID,
				Commands:  make([]*Command, 0),
				Results:   make([]*CommandResult, 0),
				StartedAt: time.Now(),
			}
			m.sessions[sessionID] = session
		}
		session.Commands = append(session.Commands, cmd)
	}

	// If needs confirmation, return without executing
	if cmd.NeedsConfirm {
		return cmd, nil, nil
	}

	// Execute the command
	result := m.executeCommand(cmd)
	m.results[cmd.ID] = result

	if sessionID != "" {
		if session, exists := m.sessions[sessionID]; exists {
			session.Results = append(session.Results, result)
		}
	}

	return cmd, result, nil
}

// ConfirmCommand confirms and executes a previously pending command
func (m *Manager) ConfirmCommand(commandID string) (*CommandResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd, exists := m.commands[commandID]
	if !exists {
		return nil, fmt.Errorf("command not found: %s", commandID)
	}

	if !cmd.NeedsConfirm {
		return nil, fmt.Errorf("command does not need confirmation")
	}

	// Execute the command
	result := m.executeCommand(cmd)
	m.results[cmd.ID] = result

	return result, nil
}

// GetSuggestions generates suggestions based on system state
func (m *Manager) GetSuggestions() []*Suggestion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suggestions := make([]*Suggestion, 0)

	// Generate suggestions based on patterns
	suggestions = append(suggestions, &Suggestion{
		ID:          uuid.New().String(),
		Title:       "检查系统状态",
		Description: "查看CPU、内存、磁盘使用情况",
		Category:    "monitoring",
		Priority:    3,
		CreatedAt:   time.Now(),
		Action: &Command{
			Type:   CommandTypeSystem,
			Action: "info",
		},
	})

	suggestions = append(suggestions, &Suggestion{
		ID:          uuid.New().String(),
		Title:       "检查Docker容器",
		Description: "列出所有运行中的Docker容器状态",
		Category:    "docker",
		Priority:    2,
		CreatedAt:   time.Now(),
		Action: &Command{
			Type:   CommandTypeDocker,
			Action: "list",
		},
	})

	suggestions = append(suggestions, &Suggestion{
		ID:          uuid.New().String(),
		Title:       "查看磁盘空间",
		Description: "检查存储空间使用情况，及时清理",
		Category:    "storage",
		Priority:    2,
		CreatedAt:   time.Now(),
		Action: &Command{
			Type:   CommandTypeStorage,
			Action: "info",
		},
	})

	return suggestions
}

// GetHistory returns command history with pagination
func (m *Manager) GetHistory(page, pageSize int) ([]*Command, []*CommandResult, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	commands := make([]*Command, 0, len(m.commands))
	for _, cmd := range m.commands {
		commands = append(commands, cmd)
	}

	// Sort by creation time (newest first)
	for i := 0; i < len(commands); i++ {
		for j := i + 1; j < len(commands); j++ {
			if commands[j].CreatedAt.After(commands[i].CreatedAt) {
				commands[i], commands[j] = commands[j], commands[i]
			}
		}
	}

	total := len(commands)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return nil, nil, total
	}
	if end > total {
		end = total
	}

	pagedCommands := commands[start:end]
	results := make([]*CommandResult, 0, len(pagedCommands))
	for _, cmd := range pagedCommands {
		if result, exists := m.results[cmd.ID]; exists {
			results = append(results, result)
		}
	}

	return pagedCommands, results, total
}

// GetStats returns copilot usage statistics
func (m *Manager) GetStats() *CopilotStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &CopilotStats{
		CommandByType: make(map[CommandType]int),
	}

	var totalDuration time.Duration
	var lastCmd *Command

	for _, cmd := range m.commands {
		stats.TotalCommands++
		stats.CommandByType[cmd.Type]++

		if lastCmd == nil || cmd.CreatedAt.After(lastCmd.CreatedAt) {
			lastCmd = cmd
		}
	}

	for _, result := range m.results {
		switch result.Status {
		case StatusSuccess:
			stats.SuccessCount++
		case StatusFailed:
			stats.FailedCount++
		case StatusDenied:
			stats.DeniedCount++
		}
		totalDuration += result.Duration
	}

	if stats.TotalCommands > 0 {
		stats.AvgDurationMs = float64(totalDuration.Milliseconds()) / float64(stats.TotalCommands)
	}

	if lastCmd != nil {
		stats.LastCommandAt = &lastCmd.CreatedAt
	}

	stats.UptimeHours = time.Since(m.startTime).Hours()

	return stats
}

// GetSession returns a copilot session by ID
func (m *Manager) GetSession(sessionID string) (*CopilotSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ParseInput parses natural language input into a command
func (re *RuleEngine) ParseInput(input string) *Command {
	inputLower := strings.ToLower(input)

	cmd := &Command{
		RawInput:   input,
		Parameters: make(map[string]string),
		CreatedAt:  time.Now(),
	}

	// Find matching pattern
	bestMatch := -1
	bestScore := 0

	for i, pattern := range re.patterns {
		score := 0
		for _, keyword := range pattern.keywords {
			if strings.Contains(inputLower, strings.ToLower(keyword)) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestMatch = i
		}
	}

	if bestMatch >= 0 && bestScore > 0 {
		pattern := re.patterns[bestMatch]
		cmd.Type = pattern.commandType
		cmd.Action = pattern.action
		cmd.Sensitivity = pattern.sensitivity
		cmd.NeedsConfirm = pattern.needsConfirm
	} else {
		cmd.Type = CommandTypeUnknown
		cmd.Action = "query"
		cmd.Sensitivity = SensitivityLow
		cmd.NeedsConfirm = false
	}

	// Extract target (simple extraction)
	words := strings.Fields(input)
	for _, word := range words {
		wordLower := strings.ToLower(word)
		// Skip common words
		if len(wordLower) > 2 && !isStopWord(wordLower) {
			cmd.Target = word
			break
		}
	}

	return cmd
}

func isStopWord(word string) bool {
	stopWords := []string{"的", "了", "在", "是", "我", "有", "和", "就", "不", "人", "都", "一", "一个", "上", "也", "很", "到", "说", "要", "去", "你", "会", "着", "没有", "看", "好", "自己", "这"}
	for _, sw := range stopWords {
		if word == sw {
			return true
		}
	}
	return false
}

// executeCommand simulates command execution
func (m *Manager) executeCommand(cmd *Command) *CommandResult {
	start := time.Now()

	result := &CommandResult{
		CommandID:  cmd.ID,
		ExecutedAt: start,
	}

	// Simulate execution based on command type
	switch cmd.Type {
	case CommandTypeSystem:
		if cmd.Action == "info" {
			result.Status = StatusSuccess
			result.Output = "系统运行正常\nCPU: 45%\n内存: 6.2GB/16GB\n磁盘: 234GB/500GB\n运行时间: 15天"
		} else if cmd.Action == "reboot" {
			result.Status = StatusSuccess
			result.Output = "系统重启命令已发送"
		} else if cmd.Action == "logs" {
			result.Status = StatusSuccess
			result.Output = "最近日志:\n[INFO] 系统正常运行\n[INFO] 服务启动完成"
		} else {
			result.Status = StatusSuccess
			result.Output = fmt.Sprintf("系统操作 %s 已执行", cmd.Action)
		}

	case CommandTypeService:
		if cmd.Action == "status" {
			result.Status = StatusSuccess
			result.Output = "服务状态:\nnginx: running\nmysql: running\nredis: running"
		} else {
			result.Status = StatusSuccess
			result.Output = fmt.Sprintf("服务 %s 操作已完成", cmd.Action)
		}

	case CommandTypeDocker:
		if cmd.Action == "list" {
			result.Status = StatusSuccess
			result.Output = "运行中的容器:\n- nas-api (Up 3 days)\n- nas-web (Up 3 days)\n- postgres (Up 1 week)"
		} else {
			result.Status = StatusSuccess
			result.Output = fmt.Sprintf("Docker操作 %s 已执行", cmd.Action)
		}

	case CommandTypeNetwork:
		result.Status = StatusSuccess
		result.Output = "网络信息:\nIP: 192.168.1.100\n网关: 192.168.1.1\nDNS: 8.8.8.8"

	case CommandTypeStorage:
		if cmd.Action == "info" {
			result.Status = StatusSuccess
			result.Output = "存储信息:\n/ : 45% (234GB/500GB)\n/data: 67% (1.3TB/2TB)"
		} else {
			result.Status = StatusSuccess
			result.Output = fmt.Sprintf("存储操作 %s 已执行", cmd.Action)
		}

	case CommandTypeFirewall:
		result.Status = StatusSuccess
		result.Output = "防火墙规则已更新"

	case CommandTypeMonitor:
		result.Status = StatusSuccess
		result.Output = "监控数据:\nCPU使用率: 45%\n内存使用率: 38%\n磁盘IO: 正常\n网络流量: 12MB/s"

	case CommandTypeBackup:
		result.Status = StatusSuccess
		result.Output = fmt.Sprintf("备份操作 %s 已执行", cmd.Action)

	case CommandTypeUser:
		result.Status = StatusSuccess
		result.Output = fmt.Sprintf("用户管理操作 %s 已执行", cmd.Action)

	default:
		result.Status = StatusSuccess
		result.Output = fmt.Sprintf("已处理查询: %s", cmd.RawInput)
	}

	result.Duration = time.Since(start)
	return result
}
