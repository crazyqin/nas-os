// Package datapipeline 数据管道管理器 - ETL管道定义、执行和监控
package datapipeline

import (
	"errors"
	"time"
)

// DataSourceType 数据源类型
type DataSourceType string

const (
	SourceFile  DataSourceType = "file"
	SourceDB    DataSourceType = "database"
	SourceAPI   DataSourceType = "api"
	SourceS3    DataSourceType = "s3"
	SourceKafka DataSourceType = "kafka"
	SourceStdin DataSourceType = "stdin"
)

// TransformType 转换类型
type TransformType string

const (
	TransformFilter    TransformType = "filter"
	TransformMap       TransformType = "map"
	TransformAggregate TransformType = "aggregate"
	TransformJoin      TransformType = "join"
	TransformWindow    TransformType = "window"
	TransformFlatten   TransformType = "flatten"
	TransformDedup     TransformType = "dedup"
)

// ScheduleType 调度类型
type ScheduleType string

const (
	ScheduleCron     ScheduleType = "cron"
	ScheduleRealtime ScheduleType = "realtime"
	ScheduleManual   ScheduleType = "manual"
	ScheduleOnce     ScheduleType = "once"
)

// PipelineStatus 管道状态
type PipelineStatus string

const (
	PipelineIdle      PipelineStatus = "idle"
	PipelineRunning   PipelineStatus = "running"
	PipelinePaused    PipelineStatus = "paused"
	PipelineError     PipelineStatus = "error"
	PipelineCompleted PipelineStatus = "completed"
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecPending  ExecutionStatus = "pending"
	ExecRunning  ExecutionStatus = "running"
	ExecSuccess  ExecutionStatus = "success"
	ExecFailed   ExecutionStatus = "failed"
	ExecRetrying ExecutionStatus = "retrying"
	ExecDead     ExecutionStatus = "dead_letter"
)

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
}

// DeadLetterConfig 死信队列配置
type DeadLetterConfig struct {
	Enabled   bool   `json:"enabled"`
	MaxSize   int    `json:"max_size"`
	Retention string `json:"retention"`
}

// DataSource 数据源配置
type DataSource struct {
	ID        string            `json:"id"`
	Type      DataSourceType    `json:"type"`
	Name      string            `json:"name"`
	Config    map[string]string `json:"config"`
	BatchSize int               `json:"batch_size,omitempty"`
	Interval  string            `json:"interval,omitempty"`
}

// TransformStep 转换步骤
type TransformStep struct {
	ID      string            `json:"id"`
	Type    TransformType     `json:"type"`
	Name    string            `json:"name"`
	Config  map[string]string `json:"config"`
	Enabled bool              `json:"enabled"`
}

// DataSink 数据输出目标
type DataSink struct {
	ID     string            `json:"id"`
	Type   DataSourceType    `json:"type"`
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

// Schedule 调度配置
type Schedule struct {
	Type    ScheduleType `json:"type"`
	Cron    string       `json:"cron,omitempty"`
	Enabled bool         `json:"enabled"`
	NextRun *time.Time   `json:"next_run,omitempty"`
	LastRun *time.Time   `json:"last_run,omitempty"`
}

// Pipeline 数据管道定义
type Pipeline struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      PipelineStatus   `json:"status"`
	Source      DataSource       `json:"source"`
	Transforms  []TransformStep  `json:"transforms"`
	Sink        DataSink         `json:"sink"`
	Schedule    Schedule         `json:"schedule"`
	Retry       RetryPolicy      `json:"retry"`
	DeadLetter  DeadLetterConfig `json:"dead_letter"`
	Tags        []string         `json:"tags,omitempty"`
	RunCount    int64            `json:"run_count"`
	LastSuccess *time.Time       `json:"last_success,omitempty"`
	LastError   string           `json:"last_error,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// Execution 执行记录
type Execution struct {
	ID         string          `json:"id"`
	PipelineID string          `json:"pipeline_id"`
	Status     ExecutionStatus `json:"status"`
	StartTime  time.Time       `json:"start_time"`
	EndTime    *time.Time      `json:"end_time,omitempty"`
	Duration   int64           `json:"duration_ms"`
	RecordsIn  int64           `json:"records_in"`
	RecordsOut int64           `json:"records_out"`
	ErrorMsg   string          `json:"error_msg,omitempty"`
	RetryCount int             `json:"retry_count"`
	Trigger    string          `json:"trigger"`
}

// DLQEntry 死信队列条目
type DLQEntry struct {
	ID          string    `json:"id"`
	PipelineID  string    `json:"pipeline_id"`
	ExecutionID string    `json:"execution_id"`
	Data        string    `json:"data"`
	Error       string    `json:"error"`
	RetryCount  int       `json:"retry_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// PipelineStats 管道统计
type PipelineStats struct {
	TotalPipelines   int   `json:"total_pipelines"`
	RunningPipelines int   `json:"running_pipelines"`
	TotalExecutions  int64 `json:"total_executions"`
	SuccessCount     int64 `json:"success_count"`
	FailedCount      int64 `json:"failed_count"`
	DLQCount         int   `json:"dlq_count"`
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxPipelines  int `json:"max_pipelines"`
	MaxExecutions int `json:"max_executions_per_pipeline"`
	MaxDLQSize    int `json:"max_dlq_size"`
	WorkerCount   int `json:"worker_count"`
}

var (
	ErrPipelineNotFound   = errors.New("pipeline not found")
	ErrPipelineExists     = errors.New("pipeline already exists")
	ErrPipelineRunning    = errors.New("pipeline is running")
	ErrPipelineNotRunning = errors.New("pipeline is not running")
	ErrMaxPipelines       = errors.New("max pipelines reached")
	ErrInvalidSource      = errors.New("invalid data source type")
	ErrInvalidTransform   = errors.New("invalid transform type")
	ErrInvalidSchedule    = errors.New("invalid schedule type")
	ErrDLQFull            = errors.New("dead letter queue is full")
	ErrExecutionNotFound  = errors.New("execution not found")
)
