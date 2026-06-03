package datapipeline

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// PipelineEngine 流水线执行引擎
type PipelineEngine struct {
	processorFactory *ProcessorFactory
	running          sync.Map // map[string]context.CancelFunc
	mu               sync.RWMutex
	logger           *log.Logger
}

// NewPipelineEngine 创建流水线执行引擎
func NewPipelineEngine(logger *log.Logger) *PipelineEngine {
	if logger == nil {
		logger = log.Default()
	}

	return &PipelineEngine{
		processorFactory: NewProcessorFactory(),
		logger:           logger,
	}
}

// Execute 执行流水线
func (e *PipelineEngine) Execute(ctx context.Context, pipeline *Pipeline, inputData map[string]interface{}) (*Execution, error) {
	if err := pipeline.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pipeline: %w", err)
	}

	// 创建执行记录
	execution := &Execution{
		ID:          generateID(),
		PipelineID:  pipeline.ID,
		Status:      ExecutionStatusRunning,
		TriggerType: TriggerTypeManual,
		StartedAt:   time.Now(),
		InputData:   inputData,
		NodeResults: make([]NodeResult, 0),
		Logs:        make([]ExecutionLog, 0),
		Metrics:     &ExecutionMetrics{},
	}

	// 添加日志
	e.addLog(execution, "info", fmt.Sprintf("Starting pipeline execution: %s", pipeline.Name), "")

	// 检查是否已在运行
	if _, loaded := e.running.LoadOrStore(pipeline.ID, nil); loaded {
		execution.Status = ExecutionStatusFailed
		execution.Error = "pipeline is already running"
		execution.CompletedAt = timePtr(time.Now())
		execution.Duration = time.Since(execution.StartedAt)
		return execution, fmt.Errorf("pipeline %s is already running", pipeline.ID)
	}

	// 创建可取消的 context
	pipelineCtx, cancel := context.WithCancel(ctx)
	e.running.Store(pipeline.ID, cancel)

	// 异步执行
	go func() {
		defer func() {
			e.running.Delete(pipeline.ID)
			execution.CompletedAt = timePtr(time.Now())
			execution.Duration = time.Since(execution.StartedAt)
		}()

		if err := e.executeDAG(pipelineCtx, pipeline, execution, inputData); err != nil {
			execution.Status = ExecutionStatusFailed
			execution.Error = err.Error()
			e.addLog(execution, "error", fmt.Sprintf("Pipeline execution failed: %v", err), "")
		} else {
			execution.Status = ExecutionStatusSuccess
			e.addLog(execution, "info", "Pipeline execution completed successfully", "")
		}
	}()

	return execution, nil
}

// executeDAG 执行 DAG
func (e *PipelineEngine) executeDAG(ctx context.Context, pipeline *Pipeline, execution *Execution, inputData map[string]interface{}) error {
	// 获取拓扑排序
	order, err := pipeline.GetTopologicalOrder()
	if err != nil {
		return fmt.Errorf("failed to get topological order: %w", err)
	}

	e.addLog(execution, "info", fmt.Sprintf("DAG execution order: %v", order), "")

	// 节点结果缓存
	nodeOutputs := make(map[string][]map[string]interface{})
	nodeOutputs["_input"] = convertToSlice(inputData)

	// 并发控制
	concurrency := pipeline.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	// 按层级执行（支持并发）
	levels := e.calculateLevels(pipeline, order)
	e.addLog(execution, "info", fmt.Sprintf("Execution levels: %d", len(levels)), "")

	for levelIdx, level := range levels {
		e.addLog(execution, "info", fmt.Sprintf("Executing level %d with %d nodes", levelIdx, len(level)), "")

		// 并发执行同层节点
		if len(level) > 1 && concurrency > 1 {
			if err := e.executeLevelConcurrently(ctx, pipeline, execution, level, nodeOutputs, concurrency); err != nil {
				return err
			}
		} else {
			for _, nodeID := range level {
				if err := e.executeNode(ctx, pipeline, execution, nodeID, nodeOutputs); err != nil {
					return err
				}
			}
		}
	}

	// 计算最终指标
	e.calculateMetrics(execution, nodeOutputs)

	return nil
}

// calculateLevels 计算执行层级
func (e *PipelineEngine) calculateLevels(pipeline *Pipeline, order []string) [][]string {
	// 构建入度表
	inDegree := make(map[string]int)
	adjList := make(map[string][]string)

	for _, nodeID := range order {
		inDegree[nodeID] = 0
	}

	for _, edge := range pipeline.DAG {
		adjList[edge.From] = append(adjList[edge.From], edge.To)
		inDegree[edge.To]++
	}

	// BFS 计算层级
	levels := make([][]string, 0)
	queue := make([]string, 0)

	// 第一层：入度为 0 的节点
	for _, nodeID := range order {
		if inDegree[nodeID] == 0 {
			queue = append(queue, nodeID)
		}
	}

	for len(queue) > 0 {
		currentLevel := make([]string, 0, len(queue))
		currentLevel = append(currentLevel, queue...)
		levels = append(levels, currentLevel)

		nextQueue := make([]string, 0)
		for _, nodeID := range queue {
			for _, neighbor := range adjList[nodeID] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextQueue = append(nextQueue, neighbor)
				}
			}
		}
		queue = nextQueue
	}

	return levels
}

// executeLevelConcurrently 并发执行同层节点
func (e *PipelineEngine) executeLevelConcurrently(ctx context.Context, pipeline *Pipeline, execution *Execution, level []string, nodeOutputs map[string][]map[string]interface{}, concurrency int) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(level))
	semaphore := make(chan struct{}, concurrency)

	for _, nodeID := range level {
		wg.Add(1)
		go func(nID string) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := e.executeNode(ctx, pipeline, execution, nID, nodeOutputs); err != nil {
				errChan <- err
			}
		}(nodeID)
	}

	// 等待所有节点完成
	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// executeNode 执行单个节点
func (e *PipelineEngine) executeNode(ctx context.Context, pipeline *Pipeline, execution *Execution, nodeID string, nodeOutputs map[string][]map[string]interface{}) error {
	// 确定节点类型和配置
	nodeType, nodeConfig := e.findNodeConfig(pipeline, nodeID)

	// 重试配置
	maxRetries := 0
	if pipeline.RetryPolicy != nil {
		maxRetries = pipeline.RetryPolicy.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 检查 context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 创建节点结果
		nodeResult := NodeResult{
			NodeID:    nodeID,
			StartedAt: time.Now(),
			Status:    NodeStatusRunning,
			NodeType:  nodeType,
			Retries:   attempt,
		}

		if attempt > 0 {
			e.addLog(execution, "info", fmt.Sprintf("Retrying node %s, attempt %d/%d", nodeID, attempt, maxRetries), nodeID)
		} else {
			e.addLog(execution, "info", fmt.Sprintf("Executing node: %s (%s)", nodeID, nodeType), nodeID)
		}

		// 收集输入数据
		inputData := e.collectNodeInputs(pipeline, nodeID, nodeOutputs)
		nodeResult.Input = map[string]interface{}{
			"count": len(inputData),
		}

		// 执行节点
		var outputData []map[string]interface{}
		var err error

		switch nodeType {
		case "processor":
			outputData, err = e.executeProcessorNode(ctx, nodeConfig, inputData)
		case "output":
			err = e.executeOutputNode(ctx, nodeConfig, inputData)
			outputData = inputData // 输出节点不改变数据
		default:
			err = fmt.Errorf("unknown node type: %s", nodeType)
		}

		// 更新节点结果
		completedAt := timePtr(time.Now())
		nodeResult.CompletedAt = completedAt
		nodeResult.Duration = time.Since(nodeResult.StartedAt)
		nodeResult.Output = map[string]interface{}{
			"count": len(outputData),
		}

		if err != nil {
			nodeResult.Status = NodeStatusFailed
			nodeResult.Error = err.Error()
			lastErr = err
			e.addLog(execution, "warning", fmt.Sprintf("Node %s failed (attempt %d/%d): %v", nodeID, attempt+1, maxRetries+1, err), nodeID)

			// 等待重试间隔
			if attempt < maxRetries {
				interval := e.calculateRetryInterval(pipeline.RetryPolicy, attempt)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(interval):
				}
			}
			continue
		}

		// 成功
		nodeResult.Status = NodeStatusSuccess
		execution.NodeResults = append(execution.NodeResults, nodeResult)
		nodeOutputs[nodeID] = outputData
		e.addLog(execution, "info", fmt.Sprintf("Node %s completed successfully, output %d records", nodeID, len(outputData)), nodeID)
		return nil
	}

	// 所有重试都失败了
	return fmt.Errorf("node %s failed after %d attempts: %w", nodeID, maxRetries+1, lastErr)
}

// calculateRetryInterval 计算重试间隔
func (e *PipelineEngine) calculateRetryInterval(policy *RetryPolicy, attempt int) time.Duration {
	if policy == nil {
		return 1 * time.Second
	}

	interval := policy.InitialInterval
	if interval == 0 {
		interval = 1 * time.Second
	}

	for i := 0; i < attempt; i++ {
		interval = time.Duration(float64(interval) * policy.Multiplier)
		if policy.MaxInterval > 0 && interval > policy.MaxInterval {
			interval = policy.MaxInterval
		}
	}

	return interval
}

// findNodeConfig 查找节点配置
func (e *PipelineEngine) findNodeConfig(pipeline *Pipeline, nodeID string) (string, interface{}) {
	for _, proc := range pipeline.Processors {
		if proc.ID == nodeID {
			return "processor", proc
		}
	}
	for _, out := range pipeline.Outputs {
		if out.ID == nodeID {
			return "output", out
		}
	}
	return "unknown", nil
}

// collectNodeInputs 收集节点输入
func (e *PipelineEngine) collectNodeInputs(pipeline *Pipeline, nodeID string, nodeOutputs map[string][]map[string]interface{}) []map[string]interface{} {
	// 获取依赖节点
	deps := pipeline.GetNodeDependencies(nodeID)

	if len(deps) == 0 {
		// 无依赖，使用初始输入
		if input, ok := nodeOutputs["_input"]; ok {
			return input
		}
		return []map[string]interface{}{}
	}

	// 合并所有依赖的输出
	var result []map[string]interface{}
	for _, dep := range deps {
		if output, ok := nodeOutputs[dep]; ok {
			result = append(result, output...)
		}
	}

	return result
}

// executeProcessorNode 执行处理器节点
func (e *PipelineEngine) executeProcessorNode(ctx context.Context, nodeConfig interface{}, input []map[string]interface{}) ([]map[string]interface{}, error) {
	procConfig, ok := nodeConfig.(ProcessorNode)
	if !ok {
		return nil, fmt.Errorf("invalid processor node config")
	}

	if !procConfig.Enabled {
		return input, nil // 跳过禁用的节点
	}

	// 创建处理器
	processor, err := e.processorFactory.Create(procConfig.Type, procConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create processor: %w", err)
	}

	// 执行处理
	return processor.Process(ctx, input)
}

// executeOutputNode 执行输出节点
func (e *PipelineEngine) executeOutputNode(ctx context.Context, nodeConfig interface{}, input []map[string]interface{}) error {
	outConfig, ok := nodeConfig.(OutputNode)
	if !ok {
		return fmt.Errorf("invalid output node config")
	}

	if !outConfig.Enabled {
		return nil // 跳过禁用的节点
	}

	// 创建输出连接器
	connector, err := NewOutputConnector(outConfig)
	if err != nil {
		return fmt.Errorf("failed to create output connector: %w", err)
	}

	// 连接
	if err := connector.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect output: %w", err)
	}
	defer connector.Disconnect()

	// 写入数据
	return connector.Write(ctx, input)
}

// calculateMetrics 计算执行指标
func (e *PipelineEngine) calculateMetrics(execution *Execution, nodeOutputs map[string][]map[string]interface{}) {
	metrics := execution.Metrics

	// 计算总处理记录数
	for _, output := range nodeOutputs {
		if int64(len(output)) > metrics.ProcessedRecords {
			metrics.ProcessedRecords = int64(len(output))
		}
	}

	metrics.TotalRecords = metrics.ProcessedRecords

	// 计算吞吐量
	if execution.Duration > 0 {
		metrics.Throughput = float64(metrics.ProcessedRecords) / execution.Duration.Seconds()
	}

	execution.Metrics = metrics
}

// addLog 添加日志
func (e *PipelineEngine) addLog(execution *Execution, level, message, nodeID string) {
	logEntry := ExecutionLog{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		NodeID:    nodeID,
	}
	execution.Logs = append(execution.Logs, logEntry)
	e.logger.Printf("[%s] %s: %s", level, nodeID, message)
}

// StopPipeline 停止流水线执行
func (e *PipelineEngine) StopPipeline(pipelineID string) error {
	if cancel, ok := e.running.Load(pipelineID); ok {
		if cancelFunc, ok := cancel.(context.CancelFunc); ok && cancelFunc != nil {
			cancelFunc()
			return nil
		}
	}
	return fmt.Errorf("pipeline %s is not running", pipelineID)
}

// IsRunning 检查流水线是否在运行
func (e *PipelineEngine) IsRunning(pipelineID string) bool {
	_, ok := e.running.Load(pipelineID)
	return ok
}

// 辅助函数

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func convertToSlice(m map[string]interface{}) []map[string]interface{} {
	if m == nil {
		return []map[string]interface{}{}
	}
	return []map[string]interface{}{m}
}
