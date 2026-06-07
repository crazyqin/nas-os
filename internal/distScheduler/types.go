// Package distScheduler 提供分布式任务调度功能，支持Cron表达式、资源感知调度、
// 节点故障自动转移和任务依赖图。
package distScheduler

import "time"

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusScheduled TaskStatus = "scheduled"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusRetrying  TaskStatus = "retrying"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusBusy    NodeStatus = "busy"
	NodeStatusDrain   NodeStatus = "drain" // 排空模式，不接受新任务
)

// Strategy 调度策略
type Strategy string

const (
	StrategyRoundRobin Strategy = "round_robin" // 轮询
	StrategyLeastLoad  Strategy = "least_load"  // 最小负载
	StrategyResource   Strategy = "resource"    // 资源感知
	StrategyAffinity   Strategy = "affinity"    // 亲和性
	StrategyRandom     Strategy = "random"      // 随机
)

// ResourceType 资源类型
type ResourceType string

const (
	ResourceCPU    ResourceType = "cpu"
	ResourceMemory ResourceType = "memory"
	ResourceGPU    ResourceType = "gpu"
	ResourceDisk   ResourceType = "disk"
	ResourceIO     ResourceType = "io"
)

// Config 分布式调度器配置
type Config struct {
	Enabled          bool     `json:"enabled"`
	Strategy         Strategy `json:"strategy"`
	MaxRetries       int      `json:"max_retries"`       // 最大重试次数
	RetryDelay       int      `json:"retry_delay"`       // 重试延迟（秒）
	HeartbeatTimeout int      `json:"heartbeat_timeout"` // 心跳超时（秒）
	TaskTimeout      int      `json:"task_timeout"`      // 默认任务超时（秒）
	MaxConcurrent    int      `json:"max_concurrent"`    // 节点最大并发
	SchedulerWorkers int      `json:"scheduler_workers"` // 调度器工作线程
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		Strategy:         StrategyLeastLoad,
		MaxRetries:       3,
		RetryDelay:       30,
		HeartbeatTimeout: 60,
		TaskTimeout:      300,
		MaxConcurrent:    10,
		SchedulerWorkers: 4,
	}
}

// Node 调度节点
type Node struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Address    string            `json:"address"`
	Status     NodeStatus        `json:"status"`
	Resources  *NodeResources    `json:"resources"`
	Tags       map[string]string `json:"tags,omitempty"`
	LastHB     time.Time         `json:"last_heartbeat"`
	TaskCount  int               `json:"task_count"`  // 当前任务数
	TotalTasks int64             `json:"total_tasks"` // 累计完成任务数
	CreatedAt  time.Time         `json:"created_at"`
}

// NodeResources 节点资源
type NodeResources struct {
	CPU    ResourceInfo `json:"cpu"`
	Memory ResourceInfo `json:"memory"`
	GPU    ResourceInfo `json:"gpu,omitempty"`
	Disk   ResourceInfo `json:"disk"`
}

// ResourceInfo 资源信息
type ResourceInfo struct {
	Total     float64 `json:"total"`     // 总量
	Used      float64 `json:"used"`      // 已用量
	Available float64 `json:"available"` // 可用量
	Unit      string  `json:"unit"`      // 单位
}

// Task 调度任务
type Task struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Status       TaskStatus        `json:"status"`
	Priority     int               `json:"priority"`     // 0=低, 1=正常, 2=高, 3=紧急
	NodeID       string            `json:"node_id"`      // 分配的节点
	Dependencies []string          `json:"dependencies"` // 依赖任务 ID 列表
	CronExpr     string            `json:"cron_expr"`    // Cron 表达式（可选）
	Requirements *ResourceReq      `json:"requirements"` // 资源需求
	Payload      interface{}       `json:"payload"`      // 任务负载
	Result       interface{}       `json:"result"`       // 任务结果
	Error        string            `json:"error,omitempty"`
	Attempts     int               `json:"attempts"`
	MaxAttempts  int               `json:"max_attempts"`
	Tags         map[string]string `json:"tags,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	ScheduledAt  *time.Time        `json:"scheduled_at,omitempty"` // 下次执行时间
	Timeout      int               `json:"timeout"`                // 任务超时（秒）
}

// ResourceReq 资源需求
type ResourceReq struct {
	CPU    float64 `json:"cpu"`    // CPU 核数
	Memory int64   `json:"memory"` // 内存（MB）
	GPU    int     `json:"gpu"`    // GPU 数量
	Disk   int64   `json:"disk"`   // 磁盘（MB）
}

// ScheduleResult 调度结果
type ScheduleResult struct {
	TaskID  string `json:"task_id"`
	NodeID  string `json:"node_id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// CronEntry Cron 调度项
type CronEntry struct {
	TaskID   string     `json:"task_id"`
	CronExpr string     `json:"cron_expr"`
	NextRun  time.Time  `json:"next_run"`
	LastRun  *time.Time `json:"last_run,omitempty"`
	Enabled  bool       `json:"enabled"`
}

// TaskGraph 任务依赖图
type TaskGraph struct {
	Tasks map[string]*Task    `json:"tasks"` // taskID -> Task
	Edges map[string][]string `json:"edges"` // taskID -> 依赖的 taskID 列表
}
