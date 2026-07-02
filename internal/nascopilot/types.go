// Package nascopilot 提供 NAS 智能助手/对话式管理功能
// 自然语言理解、命令解析、操作执行、知识问答、定时任务、审计日志
package nascopilot

import (
	"time"
)

// ========== 对话系统 ==========

// Conversation 对话会话.
type Conversation struct {
	ID           string       `json:"id"`           // 唯一标识
	UserID       string       `json:"userId"`       // 用户 ID
	Title        string       `json:"title"`        // 对话标题
	CreatedAt    time.Time    `json:"createdAt"`    // 创建时间
	LastActive   time.Time    `json:"lastActive"`   // 最后活跃时间
	MessageCount int          `json:"messageCount"` // 消息数
	Status       ConverStatus `json:"status"`       // 状态
}

// ConverStatus 对话状态.
type ConverStatus string

const (
	ConverStatusActive   ConverStatus = "active"   // 活跃
	ConverStatusArchived ConverStatus = "archived" // 已归档
)

// Message 消息.
type Message struct {
	ID             string        `json:"id"`             // 唯一标识
	ConversationID string        `json:"conversationId"` // 对话 ID
	Role           MessageRole   `json:"role"`           // 角色
	Content        string        `json:"content"`        // 内容
	Timestamp      time.Time     `json:"timestamp"`      // 时间戳
	CommandResult  *ActionResult `json:"commandResult"`  // 操作结果 (仅命令执行时)
}

// MessageRole 消息角色.
type MessageRole string

const (
	RoleUser      MessageRole = "user"      // 用户
	RoleAssistant MessageRole = "assistant" // 助手
	RoleSystem    MessageRole = "system"    // 系统
)

// ========== 意图识别 ==========

// Intent 识别出的用户意图.
type Intent struct {
	Type       IntentType        `json:"type"`       // 意图类型
	Confidence float64           `json:"confidence"` // 置信度 (0-1)
	Parameters map[string]string `json:"parameters"` // 提取的参数
}

// IntentType 意图类型.
type IntentType string

const (
	IntentStorage IntentType = "storage" // 存储管理
	IntentBackup  IntentType = "backup"  // 备份
	IntentNetwork IntentType = "network" // 网络配置
	IntentDocker  IntentType = "docker"  // Docker 管理
	IntentUser    IntentType = "user"    // 用户管理
	IntentSystem  IntentType = "system"  // 系统管理
	IntentQuery   IntentType = "query"   // 查询/问答
	IntentAction  IntentType = "action"  // 执行操作
)

// ========== 命令系统 ==========

// Command 解析出的命令.
type Command struct {
	Verb         CommandType       `json:"verb"`         // 动词
	ResourceType string            `json:"resourceType"` // 资源类型
	Parameters   map[string]string `json:"parameters"`   // 参数
	Status       CommandStatus     `json:"status"`       // 状态
}

// CommandType 命令动词类型.
type CommandType string

const (
	CommandCreate  CommandType = "create"  // 创建
	CommandDelete  CommandType = "delete"  // 删除
	CommandUpdate  CommandType = "update"  // 更新
	CommandList    CommandType = "list"    // 列表
	CommandShow    CommandType = "show"    // 查看详情
	CommandBackup  CommandType = "backup"  // 备份
	CommandRestore CommandType = "restore" // 恢复
)

// CommandStatus 命令状态.
type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"   // 待执行
	CommandStatusRunning   CommandStatus = "running"   // 执行中
	CommandStatusSuccess   CommandStatus = "success"   // 成功
	CommandStatusFailed    CommandStatus = "failed"    // 失败
	CommandStatusCancelled CommandStatus = "cancelled" // 已取消
)

// ========== 操作结果 ==========

// ActionResult 操作执行结果.
type ActionResult struct {
	Success  bool   `json:"success"`  // 是否成功
	Message  string `json:"message"`  // 输出信息
	Scope    string `json:"scope"`    // 影响范围
	Rollback string `json:"rollback"` // 回滚信息
}

// ========== 知识库 ==========

// KnowledgeEntry 知识条目.
type KnowledgeEntry struct {
	ID       string       `json:"id"`       // 唯一标识
	Category KnowledgeCat `json:"category"` // 分类
	Title    string       `json:"title"`    // 标题
	Content  string       `json:"content"`  // 内容
	Tags     []string     `json:"tags"`     // 标签
}

// KnowledgeCat 知识分类.
type KnowledgeCat string

const (
	KnowledgeSecurity    KnowledgeCat = "security"    // 安全
	KnowledgePerformance KnowledgeCat = "performance" // 性能
	KnowledgeBackup      KnowledgeCat = "backup"      // 备份
	KnowledgeNetwork     KnowledgeCat = "network"     // 网络
)

// ========== 定时任务 ==========

// ScheduledTask 定时任务.
type ScheduledTask struct {
	ID          string    `json:"id"`          // 唯一标识
	Description string    `json:"description"` // 描述
	CronExpr    string    `json:"cronExpr"`    // Cron 表达式
	Command     string    `json:"command"`     // 要执行的命令
	Enabled     bool      `json:"enabled"`     // 是否启用
	NextRun     time.Time `json:"nextRun"`     // 下次运行时间
	LastRun     time.Time `json:"lastRun"`     // 上次运行时间
}

// ========== 用户偏好 ==========

// UserPreference 用户偏好设置.
type UserPreference struct {
	UserID       string       `json:"userId"`       // 用户 ID
	Language     string       `json:"language"`     // 语言
	ConfirmLevel ConfirmLevel `json:"confirmLevel"` // 确认级别
	OutputFormat string       `json:"outputFormat"` // 输出格式
}

// ConfirmLevel 操作确认级别.
type ConfirmLevel string

const (
	ConfirmAlways    ConfirmLevel = "always"    // 所有操作都确认
	ConfirmDangerous ConfirmLevel = "dangerous" // 仅危险操作确认
	ConfirmNever     ConfirmLevel = "never"     // 从不确认
)

// ========== 审计日志 ==========

// AuditEntry 审计日志条目.
type AuditEntry struct {
	ID        string    `json:"id"`        // 唯一标识
	UserID    string    `json:"userId"`    // 用户 ID
	Operation string    `json:"operation"` // 操作
	Command   string    `json:"command"`   // 命令
	Result    string    `json:"result"`    // 结果
	Timestamp time.Time `json:"timestamp"` // 时间戳
	IPAddress string    `json:"ipAddress"` // IP 地址
}

// ========== 统计数据 ==========

// CopilotStats 助手统计.
type CopilotStats struct {
	TotalConversations int     `json:"totalConversations"` // 总会话数
	TotalMessages      int     `json:"totalMessages"`      // 总消息数
	ExecutedCommands   int     `json:"executedCommands"`   // 执行操作数
	SuccessRate        float64 `json:"successRate"`        // 成功率
	AvgResponseTime    float64 `json:"avgResponseTime"`    // 平均响应时间 (ms)
}

// ========== 请求/响应结构 ==========

// ChatRequest 聊天请求.
type ChatRequest struct {
	ConversationID string `json:"conversationId"`             // 对话 ID (为空则自动创建)
	Message        string `json:"message" binding:"required"` // 消息内容
	UserID         string `json:"userId"`                     // 用户 ID
}

// ChatResponse 聊天响应.
type ChatResponse struct {
	ConversationID string  `json:"conversationId"` // 对话 ID
	Message        Message `json:"message"`        // 助手回复
	Intent         *Intent `json:"intent"`         // 识别的意图
}

// ParseRequest 解析请求.
type ParseRequest struct {
	Text string `json:"text" binding:"required"`
}

// ParseResponse 解析响应.
type ParseResponse struct {
	Intent  Intent   `json:"intent"`  // 识别的意图
	Command *Command `json:"command"` // 解析的命令
}

// ExecuteRequest 执行请求.
type ExecuteRequest struct {
	Command Command `json:"command" binding:"required"`
}

// CreateTaskRequest 创建定时任务请求.
type CreateTaskRequest struct {
	Description string `json:"description" binding:"required"`
	CronExpr    string `json:"cronExpr" binding:"required"`
	Command     string `json:"command" binding:"required"`
	Enabled     *bool  `json:"enabled"` // 指针以区分零值和未设置
}

// UpdateTaskRequest 更新定时任务请求.
type UpdateTaskRequest struct {
	Description string `json:"description"`
	CronExpr    string `json:"cronExpr"`
	Command     string `json:"command"`
	Enabled     *bool  `json:"enabled"`
}

// AddKnowledgeRequest 添加知识条目请求.
type AddKnowledgeRequest struct {
	Category KnowledgeCat `json:"category" binding:"required"`
	Title    string       `json:"title" binding:"required"`
	Content  string       `json:"content" binding:"required"`
	Tags     []string     `json:"tags"`
}
