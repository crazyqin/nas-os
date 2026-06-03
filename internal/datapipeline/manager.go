package datapipeline

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// PipelineManager 流水线管理器
type PipelineManager struct {
	engine     *PipelineEngine
	pipelines  sync.Map // map[string]*Pipeline
	executions sync.Map // map[string]*Execution
	stats      sync.Map // map[string]*PipelineStats
	mu         sync.RWMutex
	logger     *log.Logger
}

// NewPipelineManager 创建流水线管理器
func NewPipelineManager(logger *log.Logger) *PipelineManager {
	if logger == nil {
		logger = log.Default()
	}

	return &PipelineManager{
		engine: NewPipelineEngine(logger),
		logger: logger,
	}
}

// CreatePipeline 创建流水线
func (m *PipelineManager) CreatePipeline(pipeline *Pipeline) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline cannot be nil")
	}

	if err := pipeline.Validate(); err != nil {
		return fmt.Errorf("invalid pipeline: %w", err)
	}

	// 检查是否已存在
	if _, loaded := m.pipelines.LoadOrStore(pipeline.ID, pipeline); loaded {
		return fmt.Errorf("pipeline %s already exists", pipeline.ID)
	}

	// 初始化统计
	m.stats.Store(pipeline.ID, &PipelineStats{
		PipelineID: pipeline.ID,
	})

	m.logger.Printf("Created pipeline: %s (%s)", pipeline.ID, pipeline.Name)
	return nil
}

// UpdatePipeline 更新流水线
func (m *PipelineManager) UpdatePipeline(pipeline *Pipeline) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline cannot be nil")
	}

	if err := pipeline.Validate(); err != nil {
		return fmt.Errorf("invalid pipeline: %w", err)
	}

	// 检查是否存在
	if _, ok := m.pipelines.Load(pipeline.ID); !ok {
		return fmt.Errorf("pipeline %s not found", pipeline.ID)
	}

	// 检查是否正在运行
	if m.engine.IsRunning(pipeline.ID) {
		return fmt.Errorf("cannot update pipeline %s while it is running", pipeline.ID)
	}

	// 更新版本
	pipeline.Version++
	pipeline.UpdatedAt = time.Now()

	m.pipelines.Store(pipeline.ID, pipeline)
	m.logger.Printf("Updated pipeline: %s (version %d)", pipeline.ID, pipeline.Version)
	return nil
}

// DeletePipeline 删除流水线
func (m *PipelineManager) DeletePipeline(pipelineID string) error {
	// 检查是否正在运行
	if m.engine.IsRunning(pipelineID) {
		return fmt.Errorf("cannot delete pipeline %s while it is running", pipelineID)
	}

	m.pipelines.Delete(pipelineID)
	m.stats.Delete(pipelineID)
	m.logger.Printf("Deleted pipeline: %s", pipelineID)
	return nil
}

// GetPipeline 获取流水线
func (m *PipelineManager) GetPipeline(pipelineID string) (*Pipeline, error) {
	value, ok := m.pipelines.Load(pipelineID)
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", pipelineID)
	}
	return value.(*Pipeline), nil
}

// ListPipelines 列出所有流水线
func (m *PipelineManager) ListPipelines() []*Pipeline {
	pipelines := make([]*Pipeline, 0)
	m.pipelines.Range(func(key, value interface{}) bool {
		pipelines = append(pipelines, value.(*Pipeline))
		return true
	})
	return pipelines
}

// ExecutePipeline 执行流水线
func (m *PipelineManager) ExecutePipeline(ctx context.Context, pipelineID string, inputData map[string]interface{}) (*Execution, error) {
	// 获取流水线
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return nil, err
	}

	// 检查流水线状态
	if pipeline.Status != PipelineStatusActive && pipeline.Status != PipelineStatusDraft {
		return nil, fmt.Errorf("pipeline %s is not in active status", pipelineID)
	}

	// 执行
	execution, err := m.engine.Execute(ctx, pipeline, inputData)
	if err != nil {
		// 更新统计
		m.updateStatsOnFailure(pipelineID, err)
		return nil, err
	}

	// 存储执行记录
	m.executions.Store(execution.ID, execution)

	// 异步等待执行完成并更新统计
	go func() {
		// 简单轮询等待完成
		for {
			time.Sleep(100 * time.Millisecond)
			if execution.Status == ExecutionStatusSuccess ||
				execution.Status == ExecutionStatusFailed ||
				execution.Status == ExecutionStatusCancelled {
				break
			}
		}
		m.updateStatsOnCompletion(pipelineID, execution)
	}()

	return execution, nil
}

// StopPipeline 停止流水线执行
func (m *PipelineManager) StopPipeline(pipelineID string) error {
	return m.engine.StopPipeline(pipelineID)
}

// GetExecution 获取执行记录
func (m *PipelineManager) GetExecution(executionID string) (*Execution, error) {
	value, ok := m.executions.Load(executionID)
	if !ok {
		return nil, fmt.Errorf("execution %s not found", executionID)
	}
	return value.(*Execution), nil
}

// ListExecutions 列出流水线的执行记录
func (m *PipelineManager) ListExecutions(pipelineID string) []*Execution {
	executions := make([]*Execution, 0)
	m.executions.Range(func(key, value interface{}) bool {
		exec := value.(*Execution)
		if exec.PipelineID == pipelineID {
			executions = append(executions, exec)
		}
		return true
	})
	return executions
}

// GetPipelineStats 获取流水线统计
func (m *PipelineManager) GetPipelineStats(pipelineID string) (*PipelineStats, error) {
	value, ok := m.stats.Load(pipelineID)
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", pipelineID)
	}
	return value.(*PipelineStats), nil
}

// EnablePipeline 启用流水线
func (m *PipelineManager) EnablePipeline(pipelineID string) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	pipeline.Status = PipelineStatusActive
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	m.logger.Printf("Enabled pipeline: %s", pipelineID)
	return nil
}

// DisablePipeline 禁用流水线
func (m *PipelineManager) DisablePipeline(pipelineID string) error {
	// 检查是否正在运行
	if m.engine.IsRunning(pipelineID) {
		return fmt.Errorf("cannot disable pipeline %s while it is running", pipelineID)
	}

	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	pipeline.Status = PipelineStatusStopped
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	m.logger.Printf("Disabled pipeline: %s", pipelineID)
	return nil
}

// AddDataSource 添加数据源到流水线
func (m *PipelineManager) AddDataSource(pipelineID string, ds DataSource) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	if err := ds.Validate(); err != nil {
		return fmt.Errorf("invalid data source: %w", err)
	}

	pipeline.DataSources = append(pipeline.DataSources, ds)
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	return nil
}

// RemoveDataSource 从流水线移除数据源
func (m *PipelineManager) RemoveDataSource(pipelineID, dsID string) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	for i, ds := range pipeline.DataSources {
		if ds.ID == dsID {
			pipeline.DataSources = append(pipeline.DataSources[:i], pipeline.DataSources[i+1:]...)
			pipeline.UpdatedAt = time.Now()
			m.pipelines.Store(pipelineID, pipeline)
			return nil
		}
	}

	return fmt.Errorf("data source %s not found in pipeline", dsID)
}

// AddProcessor 添加处理器到流水线
func (m *PipelineManager) AddProcessor(pipelineID string, proc ProcessorNode) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	if err := proc.Validate(); err != nil {
		return fmt.Errorf("invalid processor: %w", err)
	}

	pipeline.Processors = append(pipeline.Processors, proc)
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	return nil
}

// RemoveProcessor 从流水线移除处理器
func (m *PipelineManager) RemoveProcessor(pipelineID, procID string) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	for i, proc := range pipeline.Processors {
		if proc.ID == procID {
			pipeline.Processors = append(pipeline.Processors[:i], pipeline.Processors[i+1:]...)
			pipeline.UpdatedAt = time.Now()
			m.pipelines.Store(pipelineID, pipeline)
			return nil
		}
	}

	return fmt.Errorf("processor %s not found in pipeline", procID)
}

// AddOutput 添加输出节点到流水线
func (m *PipelineManager) AddOutput(pipelineID string, out OutputNode) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	if err := out.Validate(); err != nil {
		return fmt.Errorf("invalid output: %w", err)
	}

	pipeline.Outputs = append(pipeline.Outputs, out)
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	return nil
}

// RemoveOutput 从流水线移除输出节点
func (m *PipelineManager) RemoveOutput(pipelineID, outID string) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	for i, out := range pipeline.Outputs {
		if out.ID == outID {
			pipeline.Outputs = append(pipeline.Outputs[:i], pipeline.Outputs[i+1:]...)
			pipeline.UpdatedAt = time.Now()
			m.pipelines.Store(pipelineID, pipeline)
			return nil
		}
	}

	return fmt.Errorf("output %s not found in pipeline", outID)
}

// AddDAGEdge 添加 DAG 边
func (m *PipelineManager) AddDAGEdge(pipelineID string, edge DAGEdge) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	// 检查节点是否存在
	nodeExists := false
	for _, proc := range pipeline.Processors {
		if proc.ID == edge.From || proc.ID == edge.To {
			nodeExists = true
			break
		}
	}
	for _, out := range pipeline.Outputs {
		if out.ID == edge.From || out.ID == edge.To {
			nodeExists = true
			break
		}
	}

	if !nodeExists {
		return fmt.Errorf("node not found in pipeline")
	}

	pipeline.DAG = append(pipeline.DAG, edge)
	pipeline.UpdatedAt = time.Now()

	// 验证 DAG
	if err := pipeline.validateDAG(); err != nil {
		// 回滚
		pipeline.DAG = pipeline.DAG[:len(pipeline.DAG)-1]
		return fmt.Errorf("invalid DAG edge: %w", err)
	}

	m.pipelines.Store(pipelineID, pipeline)
	return nil
}

// RemoveDAGEdge 移除 DAG 边
func (m *PipelineManager) RemoveDAGEdge(pipelineID, from, to string) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	for i, edge := range pipeline.DAG {
		if edge.From == from && edge.To == to {
			pipeline.DAG = append(pipeline.DAG[:i], pipeline.DAG[i+1:]...)
			pipeline.UpdatedAt = time.Now()
			m.pipelines.Store(pipelineID, pipeline)
			return nil
		}
	}

	return fmt.Errorf("DAG edge from %s to %s not found", from, to)
}

// AddTrigger 添加触发器
func (m *PipelineManager) AddTrigger(pipelineID string, trigger Trigger) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	pipeline.Triggers = append(pipeline.Triggers, trigger)
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	return nil
}

// RemoveTrigger 移除触发器
func (m *PipelineManager) RemoveTrigger(pipelineID, triggerID string) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	for i, trigger := range pipeline.Triggers {
		if trigger.ID == triggerID {
			pipeline.Triggers = append(pipeline.Triggers[:i], pipeline.Triggers[i+1:]...)
			pipeline.UpdatedAt = time.Now()
			m.pipelines.Store(pipelineID, pipeline)
			return nil
		}
	}

	return fmt.Errorf("trigger %s not found in pipeline", triggerID)
}

// SetRetryPolicy 设置重试策略
func (m *PipelineManager) SetRetryPolicy(pipelineID string, policy *RetryPolicy) error {
	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	pipeline.RetryPolicy = policy
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	return nil
}

// SetConcurrency 设置并发数
func (m *PipelineManager) SetConcurrency(pipelineID string, concurrency int) error {
	if concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}

	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	pipeline.Concurrency = concurrency
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	return nil
}

// SetTimeout 设置执行超时
func (m *PipelineManager) SetTimeout(pipelineID string, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	pipeline, err := m.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	pipeline.Timeout = timeout
	pipeline.UpdatedAt = time.Now()
	m.pipelines.Store(pipelineID, pipeline)

	return nil
}

// ClonePipeline 克隆流水线
func (m *PipelineManager) ClonePipeline(sourceID, newID, newName string) (*Pipeline, error) {
	source, err := m.GetPipeline(sourceID)
	if err != nil {
		return nil, err
	}

	// 深拷贝
	clone := &Pipeline{
		ID:          newID,
		Name:        newName,
		Description: source.Description,
		Status:      PipelineStatusDraft,
		DataSources: make([]DataSource, len(source.DataSources)),
		Processors:  make([]ProcessorNode, len(source.Processors)),
		Outputs:     make([]OutputNode, len(source.Outputs)),
		DAG:         make([]DAGEdge, len(source.DAG)),
		Triggers:    make([]Trigger, len(source.Triggers)),
		Concurrency: source.Concurrency,
		Timeout:     source.Timeout,
		Variables:   make(map[string]interface{}),
		Tags:        make([]string, len(source.Tags)),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Version:     1,
	}

	copy(clone.DataSources, source.DataSources)
	copy(clone.Processors, source.Processors)
	copy(clone.Outputs, source.Outputs)
	copy(clone.DAG, source.DAG)
	copy(clone.Triggers, source.Triggers)
	copy(clone.Tags, source.Tags)

	for k, v := range source.Variables {
		clone.Variables[k] = v
	}

	if source.RetryPolicy != nil {
		clone.RetryPolicy = &RetryPolicy{
			MaxRetries:      source.RetryPolicy.MaxRetries,
			InitialInterval: source.RetryPolicy.InitialInterval,
			MaxInterval:     source.RetryPolicy.MaxInterval,
			Multiplier:      source.RetryPolicy.Multiplier,
		}
	}

	if err := m.CreatePipeline(clone); err != nil {
		return nil, err
	}

	return clone, nil
}

// updateStatsOnCompletion 执行完成时更新统计
func (m *PipelineManager) updateStatsOnCompletion(pipelineID string, execution *Execution) {
	value, ok := m.stats.Load(pipelineID)
	if !ok {
		return
	}

	stats := value.(*PipelineStats)
	stats.TotalExecutions++
	stats.LastExecution = execution.CompletedAt
	stats.TotalRecordsProcessed += execution.Metrics.ProcessedRecords

	if execution.Status == ExecutionStatusSuccess {
		stats.SuccessExecutions++
		stats.LastSuccess = execution.CompletedAt
	} else if execution.Status == ExecutionStatusFailed {
		stats.FailedExecutions++
		stats.LastFailure = execution.CompletedAt
	}

	// 计算平均执行时长
	if stats.TotalExecutions > 0 {
		totalDuration := stats.AverageDuration * time.Duration(stats.TotalExecutions-1)
		stats.AverageDuration = (totalDuration + execution.Duration) / time.Duration(stats.TotalExecutions)
	}

	m.stats.Store(pipelineID, stats)
}

// updateStatsOnFailure 执行失败时更新统计
func (m *PipelineManager) updateStatsOnFailure(pipelineID string, err error) {
	value, ok := m.stats.Load(pipelineID)
	if !ok {
		return
	}

	stats := value.(*PipelineStats)
	stats.TotalExecutions++
	stats.FailedExecutions++
	now := time.Now()
	stats.LastExecution = &now
	stats.LastFailure = &now

	m.stats.Store(pipelineID, stats)
}

// GetEngine 获取执行引擎（用于测试）
func (m *PipelineManager) GetEngine() *PipelineEngine {
	return m.engine
}
