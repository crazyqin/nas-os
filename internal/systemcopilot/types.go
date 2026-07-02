package systemcopilot

import "time"

// CommandType represents the type of system command.
type CommandType string

const (
	CommandTypeService  CommandType = "service"
	CommandTypeNetwork  CommandType = "network"
	CommandTypeStorage  CommandType = "storage"
	CommandTypeSystem   CommandType = "system"
	CommandTypeDocker   CommandType = "docker"
	CommandTypeUser     CommandType = "user"
	CommandTypeFirewall CommandType = "firewall"
	CommandTypeMonitor  CommandType = "monitor"
	CommandTypeBackup   CommandType = "backup"
	CommandTypeUnknown  CommandType = "unknown"
)

// CommandStatus represents the execution status of a command.
type CommandStatus string

const (
	StatusPending   CommandStatus = "pending"
	StatusExecuting CommandStatus = "executing"
	StatusSuccess   CommandStatus = "success"
	StatusFailed    CommandStatus = "failed"
	StatusDenied    CommandStatus = "denied"
	StatusCancelled CommandStatus = "cancelled"
)

// SensitivityLevel represents how sensitive an operation is.
type SensitivityLevel string

const (
	SensitivityLow      SensitivityLevel = "low"
	SensitivityMedium   SensitivityLevel = "medium"
	SensitivityHigh     SensitivityLevel = "high"
	SensitivityCritical SensitivityLevel = "critical"
)

// Command represents a parsed user command.
type Command struct {
	ID           string            `json:"id"`
	RawInput     string            `json:"raw_input"`
	Type         CommandType       `json:"type"`
	Action       string            `json:"action"`
	Target       string            `json:"target,omitempty"`
	Parameters   map[string]string `json:"parameters,omitempty"`
	Sensitivity  SensitivityLevel  `json:"sensitivity"`
	NeedsConfirm bool              `json:"needs_confirm"`
	CreatedAt    time.Time         `json:"created_at"`
}

// CommandResult represents the result of executing a command.
type CommandResult struct {
	CommandID  string        `json:"command_id"`
	Status     CommandStatus `json:"status"`
	Output     string        `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
	ExecutedAt time.Time     `json:"executed_at"`
	Duration   time.Duration `json:"duration_ms"`
}

// Suggestion represents an AI-generated suggestion.
type Suggestion struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Priority    int       `json:"priority"` // 1-5, 5 being highest
	Action      *Command  `json:"action,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CopilotSession represents a conversation session with the copilot.
type CopilotSession struct {
	ID        string           `json:"id"`
	Commands  []*Command       `json:"commands"`
	Results   []*CommandResult `json:"results"`
	StartedAt time.Time        `json:"started_at"`
	EndedAt   *time.Time       `json:"ended_at,omitempty"`
}

// CopilotStats contains statistics about copilot usage.
type CopilotStats struct {
	TotalCommands int                 `json:"total_commands"`
	SuccessCount  int                 `json:"success_count"`
	FailedCount   int                 `json:"failed_count"`
	DeniedCount   int                 `json:"denied_count"`
	CommandByType map[CommandType]int `json:"command_by_type"`
	AvgDurationMs float64             `json:"avg_duration_ms"`
	LastCommandAt *time.Time          `json:"last_command_at,omitempty"`
	UptimeHours   float64             `json:"uptime_hours"`
}

// ProcessRequest is the HTTP request body for processing a command.
type ProcessRequest struct {
	Input         string `json:"input"`
	ConfirmAction bool   `json:"confirm_action,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
}

// ProcessResponse is the HTTP response for a processed command.
type ProcessResponse struct {
	Command     *Command       `json:"command"`
	Result      *CommandResult `json:"result,omitempty"`
	Message     string         `json:"message"`
	NeedConfirm bool           `json:"need_confirm,omitempty"`
}

// HistoryResponse is the HTTP response for command history.
type HistoryResponse struct {
	Commands []*Command       `json:"commands"`
	Results  []*CommandResult `json:"results"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
