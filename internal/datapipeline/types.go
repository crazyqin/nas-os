// Package datapipeline 提供智能数据流水线功能
// 支持可视化数据处理流水线编排、多数据源连接、数据处理和输出
// 对标群晖和 TrueNAS 的自动化能力
package datapipeline

import (
	"fmt"
	"time"
)

// PipelineStatus 流水线状态
type PipelineStatus string

const (
	// PipelineStatusDraft 草稿状态
	PipelineStatusDraft PipelineStatus = "draft"
	// PipelineStatusActive 激活状态
	PipelineStatusActive PipelineStatus = "active"
	// PipelineStatusRunning 运行中
	PipelineStatusRunning PipelineStatus = "running"
	// PipelineStatusStopped 已停止
	PipelineStatusStopped PipelineStatus = "stopped"
	// PipelineStatusError 错误状态
	PipelineStatusError PipelineStatus = "error"
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	// ExecutionStatusPending 等待执行
	ExecutionStatusPending ExecutionStatus = "pending"
	// ExecutionStatusRunning 正在执行
	ExecutionStatusRunning ExecutionStatus = "running"
	// ExecutionStatusSuccess 执行成功
	ExecutionStatusSuccess ExecutionStatus = "success"
	// ExecutionStatusFailed 执行失败
	ExecutionStatusFailed ExecutionStatus = "failed"
	// ExecutionStatusCancelled 执行已取消
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
	// ExecutionStatusRetrying 重试中
	ExecutionStatusRetrying ExecutionStatus = "retrying"
)

// NodeStatus 节点执行状态
type NodeStatus string

const (
	// NodeStatusPending 等待执行
	NodeStatusPending NodeStatus = "pending"
	// NodeStatusRunning 正在执行
	NodeStatusRunning NodeStatus = "running"
	// NodeStatusSuccess 执行成功
	NodeStatusSuccess NodeStatus = "success"
	// NodeStatusFailed 执行失败
	NodeStatusFailed NodeStatus = "failed"
	// NodeStatusSkipped 已跳过
	NodeStatusSkipped NodeStatus = "skipped"
	// NodeStatusCancelled 已取消
	NodeStatusCancelled NodeStatus = "cancelled"
)

// DataSourceType 数据源类型
type DataSourceType string

const (
	// DataSourceFileSystem 本地文件系统
	DataSourceFileSystem DataSourceType = "filesystem"
	// DataSourceSMB SMB/CIFS 共享
	DataSourceSMB DataSourceType = "smb"
	// DataSourceNFS NFS 共享
	DataSourceNFS DataSourceType = "nfs"
	// DataSourceS3 S3 兼容存储
	DataSourceS3 DataSourceType = "s3"
	// DataSourceDatabase 数据库
	DataSourceDatabase DataSourceType = "database"
	// DataSourceHTTP HTTP API
	DataSourceHTTP DataSourceType = "http"
	// DataSourceFTP FTP 服务器
	DataSourceFTP DataSourceType = "ftp"
)

// ProcessorType 处理器类型
type ProcessorType string

const (
	// ProcessorTypeFilter 过滤器
	ProcessorTypeFilter ProcessorType = "filter"
	// ProcessorTypeTransform 转换器
	ProcessorTypeTransform ProcessorType = "transform"
	// ProcessorTypeAggregate 聚合器
	ProcessorTypeAggregate ProcessorType = "aggregate"
	// ProcessorTypeEnrichment 富化器
	ProcessorTypeEnrichment ProcessorType = "enrichment"
	// ProcessorTypeValidator 验证器
	ProcessorTypeValidator ProcessorType = "validator"
	// ProcessorTypeDeduplicator 去重器
	ProcessorTypeDeduplicator ProcessorType = "deduplicator"
	// ProcessorTypeRouter 路由器
	ProcessorTypeRouter ProcessorType = "router"
)

// OutputType 输出类型
type OutputType string

const (
	// OutputTypeFile 文件输出
	OutputTypeFile OutputType = "file"
	// OutputTypeNotification 通知输出
	OutputTypeNotification OutputType = "notification"
	// OutputTypeWebhook Webhook 输出
	OutputTypeWebhook OutputType = "webhook"
	// OutputTypeDatabase 数据库输出
	OutputTypeDatabase OutputType = "database"
	// OutputTypeS3 S3 输出
	OutputTypeS3 OutputType = "s3"
	// OutputTypeEmail 邮件输出
	OutputTypeEmail OutputType = "email"
)

// TriggerType 触发器类型
type TriggerType string

const (
	// TriggerTypeSchedule 定时触发
	TriggerTypeSchedule TriggerType = "schedule"
	// TriggerTypeEvent 事件触发
	TriggerTypeEvent TriggerType = "event"
	// TriggerTypeManual 手动触发
	TriggerTypeManual TriggerType = "manual"
	// TriggerTypeWebhook Webhook 触发
	TriggerTypeWebhook TriggerType = "webhook"
	// TriggerTypeFileChange 文件变更触发
	TriggerTypeFileChange TriggerType = "file_change"
)

// RetryPolicy 重试策略
type RetryPolicy struct {
	// MaxRetries 最大重试次数
	MaxRetries int `json:"maxRetries"`
	// InitialInterval 初始重试间隔
	InitialInterval time.Duration `json:"initialInterval"`
	// MaxInterval 最大重试间隔
	MaxInterval time.Duration `json:"maxInterval"`
	// Multiplier 重试间隔倍数
	Multiplier float64 `json:"multiplier"`
	// RetryableErrors 可重试的错误类型
	RetryableErrors []string `json:"retryableErrors,omitempty"`
}

// DataSource 数据源配置
type DataSource struct {
	// ID 数据源唯一标识符
	ID string `json:"id"`
	// Name 数据源名称
	Name string `json:"name"`
	// Type 数据源类型
	Type DataSourceType `json:"type"`
	// Connection 连接配置
	Connection map[string]interface{} `json:"connection"`
	// Options 额外选项
	Options map[string]interface{} `json:"options,omitempty"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// ProcessorNode 处理器节点配置
type ProcessorNode struct {
	// ID 节点唯一标识符
	ID string `json:"id"`
	// Name 节点名称
	Name string `json:"name"`
	// Type 处理器类型
	Type ProcessorType `json:"type"`
	// Config 处理器配置
	Config map[string]interface{} `json:"config"`
	// InputKeys 输入数据键
	InputKeys []string `json:"inputKeys,omitempty"`
	// OutputKeys 输出数据键
	OutputKeys []string `json:"outputKeys,omitempty"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// Order 执行顺序（用于非 DAG 模式）
	Order int `json:"order,omitempty"`
}

// OutputNode 输出节点配置
type OutputNode struct {
	// ID 节点唯一标识符
	ID string `json:"id"`
	// Name 节点名称
	Name string `json:"name"`
	// Type 输出类型
	Type OutputType `json:"type"`
	// Config 输出配置
	Config map[string]interface{} `json:"config"`
	// InputKeys 输入数据键
	InputKeys []string `json:"inputKeys,omitempty"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
}

// Trigger 触发器配置
type Trigger struct {
	// ID 触发器唯一标识符
	ID string `json:"id"`
	// Type 触发器类型
	Type TriggerType `json:"type"`
	// Config 触发器配置
	Config map[string]interface{} `json:"config"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
}

// DAGEdge DAG 边（节点依赖关系）
type DAGEdge struct {
	// From 源节点 ID
	From string `json:"from"`
	// To 目标节点 ID
	To string `json:"to"`
	// Condition 条件表达式（可选）
	Condition string `json:"condition,omitempty"`
}

// Pipeline 数据处理流水线
type Pipeline struct {
	// ID 流水线唯一标识符
	ID string `json:"id"`
	// Name 流水线名称
	Name string `json:"name"`
	// Description 流水线描述
	Description string `json:"description,omitempty"`
	// Status 流水线状态
	Status PipelineStatus `json:"status"`
	// DataSources 数据源列表
	DataSources []DataSource `json:"dataSources"`
	// Processors 处理器节点列表
	Processors []ProcessorNode `json:"processors"`
	// Outputs 输出节点列表
	Outputs []OutputNode `json:"outputs"`
	// DAG DAG 定义（节点间依赖关系）
	DAG []DAGEdge `json:"dag,omitempty"`
	// Triggers 触发器列表
	Triggers []Trigger `json:"triggers,omitempty"`
	// RetryPolicy 重试策略
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
	// Concurrency 并发数
	Concurrency int `json:"concurrency,omitempty"`
	// Timeout 执行超时时间
	Timeout time.Duration `json:"timeout,omitempty"`
	// Variables 流水线变量
	Variables map[string]interface{} `json:"variables,omitempty"`
	// Tags 标签
	Tags []string `json:"tags,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy 创建者
	CreatedBy string `json:"createdBy,omitempty"`
	// Version 版本号
	Version int `json:"version"`
}

// Execution 执行记录
type Execution struct {
	// ID 执行唯一标识符
	ID string `json:"id"`
	// PipelineID 流水线 ID
	PipelineID string `json:"pipelineId"`
	// Status 执行状态
	Status ExecutionStatus `json:"status"`
	// TriggerType 触发类型
	TriggerType TriggerType `json:"triggerType"`
	// StartedAt 开始时间
	StartedAt time.Time `json:"startedAt"`
	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	// Duration 执行耗时
	Duration time.Duration `json:"duration"`
	// InputData 输入数据
	InputData map[string]interface{} `json:"inputData,omitempty"`
	// OutputData 输出数据
	OutputData map[string]interface{} `json:"outputData,omitempty"`
	// NodeResults 节点执行结果
	NodeResults []NodeResult `json:"nodeResults,omitempty"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// Logs 执行日志
	Logs []ExecutionLog `json:"logs,omitempty"`
	// Retries 重试次数
	Retries int `json:"retries"`
	// Metrics 执行指标
	Metrics *ExecutionMetrics `json:"metrics,omitempty"`
}

// NodeResult 节点执行结果
type NodeResult struct {
	// NodeID 节点 ID
	NodeID string `json:"nodeId"`
	// NodeType 节点类型 (processor/output)
	NodeType string `json:"nodeType"`
	// Status 节点状态
	Status NodeStatus `json:"status"`
	// StartedAt 开始时间
	StartedAt time.Time `json:"startedAt"`
	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	// Duration 执行耗时
	Duration time.Duration `json:"duration"`
	// Input 输入数据
	Input map[string]interface{} `json:"input,omitempty"`
	// Output 输出数据
	Output map[string]interface{} `json:"output,omitempty"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// Retries 重试次数
	Retries int `json:"retries"`
}

// ExecutionLog 执行日志
type ExecutionLog struct {
	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`
	// Level 日志级别
	Level string `json:"level"`
	// Message 日志消息
	Message string `json:"message"`
	// NodeID 相关节点 ID
	NodeID string `json:"nodeId,omitempty"`
	// Data 附加数据
	Data map[string]interface{} `json:"data,omitempty"`
}

// ExecutionMetrics 执行指标
type ExecutionMetrics struct {
	// TotalRecords 总记录数
	TotalRecords int64 `json:"totalRecords"`
	// ProcessedRecords 处理记录数
	ProcessedRecords int64 `json:"processedRecords"`
	// FilteredRecords 过滤记录数
	FilteredRecords int64 `json:"filteredRecords"`
	// ErrorRecords 错误记录数
	ErrorRecords int64 `json:"errorRecords"`
	// BytesProcessed 处理字节数
	BytesProcessed int64 `json:"bytesProcessed"`
	// BytesOutput 输出字节数
	BytesOutput int64 `json:"bytesOutput"`
	// Throughput 吞吐量（记录/秒）
	Throughput float64 `json:"throughput"`
}

// PipelineStats 流水线统计
type PipelineStats struct {
	// PipelineID 流水线 ID
	PipelineID string `json:"pipelineId"`
	// TotalExecutions 总执行次数
	TotalExecutions int64 `json:"totalExecutions"`
	// SuccessExecutions 成功执行次数
	SuccessExecutions int64 `json:"successExecutions"`
	// FailedExecutions 失败执行次数
	FailedExecutions int64 `json:"failedExecutions"`
	// AverageDuration 平均执行时长
	AverageDuration time.Duration `json:"averageDuration"`
	// LastExecution 最后执行时间
	LastExecution *time.Time `json:"lastExecution,omitempty"`
	// LastSuccess 最后成功时间
	LastSuccess *time.Time `json:"lastSuccess,omitempty"`
	// LastFailure 最后失败时间
	LastFailure *time.Time `json:"lastFailure,omitempty"`
	// TotalRecordsProcessed 总处理记录数
	TotalRecordsProcessed int64 `json:"totalRecordsProcessed"`
}

// Validate 验证流水线配置
func (p *Pipeline) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("pipeline ID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	if len(p.DataSources) == 0 {
		return fmt.Errorf("at least one data source is required")
	}
	if len(p.Processors) == 0 && len(p.Outputs) == 0 {
		return fmt.Errorf("at least one processor or output is required")
	}

	// 验证数据源
	for _, ds := range p.DataSources {
		if err := ds.Validate(); err != nil {
			return fmt.Errorf("invalid data source %s: %w", ds.ID, err)
		}
	}

	// 验证处理器节点
	for _, proc := range p.Processors {
		if err := proc.Validate(); err != nil {
			return fmt.Errorf("invalid processor %s: %w", proc.ID, err)
		}
	}

	// 验证输出节点
	for _, out := range p.Outputs {
		if err := out.Validate(); err != nil {
			return fmt.Errorf("invalid output %s: %w", out.ID, err)
		}
	}

	// 验证 DAG（如果存在）
	if len(p.DAG) > 0 {
		if err := p.validateDAG(); err != nil {
			return fmt.Errorf("invalid DAG: %w", err)
		}
	}

	return nil
}

// Validate 验证数据源配置
func (ds *DataSource) Validate() error {
	if ds.ID == "" {
		return fmt.Errorf("data source ID is required")
	}
	if ds.Name == "" {
		return fmt.Errorf("data source name is required")
	}
	if ds.Type == "" {
		return fmt.Errorf("data source type is required")
	}
	if len(ds.Connection) == 0 {
		return fmt.Errorf("data source connection is required")
	}
	return nil
}

// Validate 验证处理器节点配置
func (pn *ProcessorNode) Validate() error {
	if pn.ID == "" {
		return fmt.Errorf("processor ID is required")
	}
	if pn.Name == "" {
		return fmt.Errorf("processor name is required")
	}
	if pn.Type == "" {
		return fmt.Errorf("processor type is required")
	}
	return nil
}

// Validate 验证输出节点配置
func (on *OutputNode) Validate() error {
	if on.ID == "" {
		return fmt.Errorf("output ID is required")
	}
	if on.Name == "" {
		return fmt.Errorf("output name is required")
	}
	if on.Type == "" {
		return fmt.Errorf("output type is required")
	}
	return nil
}

// validateDAG 验证 DAG 是否有效（无环）
func (p *Pipeline) validateDAG() error {
	// 构建邻接表
	graph := make(map[string][]string)
	allNodes := make(map[string]bool)

	// 收集所有节点
	for _, proc := range p.Processors {
		allNodes[proc.ID] = true
	}
	for _, out := range p.Outputs {
		allNodes[out.ID] = true
	}

	// 构建图
	for _, edge := range p.DAG {
		if !allNodes[edge.From] {
			return fmt.Errorf("edge references unknown node: %s", edge.From)
		}
		if !allNodes[edge.To] {
			return fmt.Errorf("edge references unknown node: %s", edge.To)
		}
		graph[edge.From] = append(graph[edge.From], edge.To)
	}

	// 检测环（使用 DFS）
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(node string) bool
	hasCycle = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range graph[node] {
			if !visited[neighbor] {
				if hasCycle(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for node := range allNodes {
		if !visited[node] {
			if hasCycle(node) {
				return fmt.Errorf("DAG contains a cycle")
			}
		}
	}

	return nil
}

// GetTopologicalOrder 获取 DAG 拓扑排序
func (p *Pipeline) GetTopologicalOrder() ([]string, error) {
	if len(p.DAG) == 0 {
		// 无 DAG 定义时，按处理器顺序执行
		order := make([]string, 0, len(p.Processors)+len(p.Outputs))
		for _, proc := range p.Processors {
			order = append(order, proc.ID)
		}
		for _, out := range p.Outputs {
			order = append(order, out.ID)
		}
		return order, nil
	}

	// 构建邻接表和入度表
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	allNodes := make(map[string]bool)

	// 收集所有节点
	for _, proc := range p.Processors {
		allNodes[proc.ID] = true
		inDegree[proc.ID] = 0
	}
	for _, out := range p.Outputs {
		allNodes[out.ID] = true
		inDegree[out.ID] = 0
	}

	// 构建图
	for _, edge := range p.DAG {
		graph[edge.From] = append(graph[edge.From], edge.To)
		inDegree[edge.To]++
	}

	// Kahn's algorithm
	queue := make([]string, 0)
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	order := make([]string, 0, len(allNodes))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(allNodes) {
		return nil, fmt.Errorf("DAG contains a cycle")
	}

	return order, nil
}

// GetNodeDependencies 获取节点的直接依赖
func (p *Pipeline) GetNodeDependencies(nodeID string) []string {
	deps := make([]string, 0)
	for _, edge := range p.DAG {
		if edge.To == nodeID {
			deps = append(deps, edge.From)
		}
	}
	return deps
}

// GetNodeDependents 获取依赖此节点的节点
func (p *Pipeline) GetNodeDependents(nodeID string) []string {
	deps := make([]string, 0)
	for _, edge := range p.DAG {
		if edge.From == nodeID {
			deps = append(deps, edge.To)
		}
	}
	return deps
}
