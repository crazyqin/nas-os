// Package edgeai 提供推理引擎核心功能
package edgeai

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine 推理引擎实现
type Engine struct {
	config    *EngineConfig
	models    map[string]*Model
	loaders   map[ModelFormat]ModelLoader
	pipeline  InferPipeline
	scheduler *TaskScheduler
	monitor   ResourceMonitor
	cache     *InferCache
	stats     *InferStats
	mu        sync.RWMutex
	stopCh    chan struct{}
	startTime time.Time
}

// NewEngine 创建推理引擎
func NewEngine(config *EngineConfig, pipeline InferPipeline, monitor ResourceMonitor) *Engine {
	if config == nil {
		config = DefaultEngineConfig()
	}

	engine := &Engine{
		config:    config,
		models:    make(map[string]*Model),
		loaders:   make(map[ModelFormat]ModelLoader),
		pipeline:  pipeline,
		scheduler: NewTaskScheduler(config.MaxQueueSize, config.MaxConcurrent),
		monitor:   monitor,
		cache:     NewInferCache(config.CacheSize, config.CacheTTL),
		stats:     &InferStats{},
		stopCh:    make(chan struct{}),
		startTime: time.Now(),
	}

	// 启动监控
	if monitor != nil && config.MonitorInterval > 0 {
		monitor.Start(config.MonitorInterval)
	}

	// 启动调度器
	engine.scheduler.Start()

	return engine
}

// RegisterLoader 注册模型加载器
func (e *Engine) RegisterLoader(format ModelFormat, loader ModelLoader) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaders[format] = loader
}

// LoadModel 加载模型
func (e *Engine) LoadModel(model *Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查模型是否已加载
	if existing, ok := e.models[model.ID]; ok {
		if existing.Status == ModelStatusReady || existing.Status == ModelStatusRunning {
			return fmt.Errorf("模型 %s 已加载", model.ID)
		}
	}

	// 查找合适的加载器
	loader, ok := e.loaders[model.Format]
	if !ok {
		return fmt.Errorf("不支持的模型格式: %s", model.Format)
	}

	// 更新状态为加载中
	model.Status = ModelStatusLoading
	model.UpdatedAt = time.Now()
	e.models[model.ID] = model

	// 异步加载模型
	go func() {
		loadedModel, err := loader.Load(model.FilePath, model.Config)
		if err != nil {
			e.mu.Lock()
			model.Status = ModelStatusError
			model.UpdatedAt = time.Now()
			e.mu.Unlock()
			log.Printf("加载模型 %s 失败: %v", model.ID, err)
			return
		}

		e.mu.Lock()
		model.Status = ModelStatusReady
		now := time.Now()
		model.LoadedAt = &now
		model.UpdatedAt = now
		e.models[model.ID] = model
		e.stats.ModelsLoaded++
		e.mu.Unlock()

		// 存储加载的模型实例
		e.cache.SetModel(model.ID, loadedModel)

		log.Printf("模型 %s 加载成功", model.ID)
	}()

	return nil
}

// UnloadModel 卸载模型
func (e *Engine) UnloadModel(modelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, ok := e.models[modelID]
	if !ok {
		return fmt.Errorf("模型 %s 不存在", modelID)
	}

	// 查找加载器
	loader, ok := e.loaders[model.Format]
	if !ok {
		return fmt.Errorf("不支持的模型格式: %s", model.Format)
	}

	// 获取加载的模型实例
	loadedModel := e.cache.GetModel(modelID)
	if loadedModel != nil {
		if err := loader.Unload(loadedModel); err != nil {
			log.Printf("卸载模型 %s 失败: %v", modelID, err)
		}
	}

	// 更新状态
	model.Status = ModelStatusUnloaded
	model.UpdatedAt = time.Now()
	e.stats.ModelsLoaded--

	// 清理缓存
	e.cache.RemoveModel(modelID)

	return nil
}

// Infer 执行推理
func (e *Engine) Infer(request *InferenceRequest) (*InferenceResult, error) {
	start := time.Now()

	// 校验请求
	if err := request.Validate(); err != nil {
		return nil, err
	}

	// 检查缓存
	cacheKey := e.buildCacheKey(request)
	if cached := e.cache.GetResult(cacheKey); cached != nil {
		return cached.(*InferenceResult), nil
	}

	// 获取模型
	e.mu.RLock()
	model, ok := e.models[request.ModelID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("模型 %s 不存在", request.ModelID)
	}

	if model.Status != ModelStatusReady {
		return nil, fmt.Errorf("模型 %s 未就绪，当前状态: %s", request.ModelID, model.Status)
	}

	// 更新模型状态
	e.mu.Lock()
	model.Status = ModelStatusRunning
	model.UpdatedAt = time.Now()
	e.mu.Unlock()

	// 执行推理
	result, err := e.pipeline.Process(request, model)
	if err != nil {
		e.mu.Lock()
		model.Status = ModelStatusReady
		model.UpdatedAt = time.Now()
		e.stats.FailedRequests++
		e.mu.Unlock()

		return &InferenceResult{
			ID:          uuid.New().String(),
			RequestID:   request.ID,
			ModelID:     request.ModelID,
			TaskType:    request.TaskType,
			Status:      TaskStatusFailed,
			Error:       err.Error(),
			Latency:     time.Since(start),
			CompletedAt: time.Now(),
		}, nil
	}

	// 更新模型状态和统计
	e.mu.Lock()
	model.Status = ModelStatusReady
	model.UpdatedAt = time.Now()
	model.InferCount++
	latency := time.Since(start)
	model.AvgLatency = (model.AvgLatency*float64(model.InferCount-1) + float64(latency.Milliseconds())) / float64(model.InferCount)
	e.stats.TotalRequests++
	e.stats.SuccessRequests++
	e.stats.LastInferTime = time.Now()
	e.mu.Unlock()

	result.Latency = latency
	result.Device = model.Device
	result.CompletedAt = time.Now()

	// 缓存结果
	e.cache.SetResult(cacheKey, result)

	return result, nil
}

// InferAsync 异步推理
func (e *Engine) InferAsync(request *InferenceRequest) (string, error) {
	// 校验请求
	if err := request.Validate(); err != nil {
		return "", err
	}

	// 提交到调度器
	requestID, err := e.scheduler.Submit(request)
	if err != nil {
		return "", fmt.Errorf("提交任务失败: %w", err)
	}

	// 异步执行
	go func() {
		result, err := e.Infer(request)
		if err != nil {
			log.Printf("异步推理失败: %v", err)
			return
		}

		// 存储结果
		e.cache.SetAsyncResult(requestID, result)

		// 触发回调
		if request.Callback != "" {
			e.triggerCallback(request.Callback, result)
		}
	}()

	return requestID, nil
}

// GetResult 获取推理结果
func (e *Engine) GetResult(requestID string) (*InferenceResult, error) {
	result := e.cache.GetAsyncResult(requestID)
	if result == nil {
		return nil, fmt.Errorf("结果 %s 不存在或尚未完成", requestID)
	}
	return result.(*InferenceResult), nil
}

// GetModel 获取模型信息
func (e *Engine) GetModel(modelID string) (*Model, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	model, ok := e.models[modelID]
	if !ok {
		return nil, fmt.Errorf("模型 %s 不存在", modelID)
	}

	return model, nil
}

// ListModels 列出所有模型
func (e *Engine) ListModels() ([]*Model, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	models := make([]*Model, 0, len(e.models))
	for _, model := range e.models {
		models = append(models, model)
	}

	return models, nil
}

// GetStats 获取推理统计
func (e *Engine) GetStats() (*InferStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	stats.Uptime = time.Since(e.startTime)
	stats.QueuedRequests = int64(e.scheduler.QueueLength())

	return &stats, nil
}

// GetResourceUsage 获取资源使用情况
func (e *Engine) GetResourceUsage() (*ResourceUsage, error) {
	if e.monitor == nil {
		return nil, fmt.Errorf("资源监控器未初始化")
	}
	return e.monitor.GetUsage()
}

// Close 关闭引擎
func (e *Engine) Close() error {
	close(e.stopCh)

	// 停止监控
	if e.monitor != nil {
		e.monitor.Stop()
	}

	// 停止调度器
	e.scheduler.Stop()

	// 收集要卸载的模型 ID
	e.mu.RLock()
	modelIDs := make([]string, 0, len(e.models))
	for id := range e.models {
		modelIDs = append(modelIDs, id)
	}
	e.mu.RUnlock()

	// 卸载所有模型
	for _, id := range modelIDs {
		e.UnloadModel(id)
	}

	// 清理缓存
	e.cache.Clear()

	return nil
}

// buildCacheKey 构建缓存键
func (e *Engine) buildCacheKey(request *InferenceRequest) string {
	return fmt.Sprintf("%s:%s:%v", request.ModelID, request.TaskType, request.Input)
}

// triggerCallback 触发回调
func (e *Engine) triggerCallback(callbackURL string, result *InferenceResult) {
	// 简化实现：记录日志
	log.Printf("触发回调: %s, 结果: %v", callbackURL, result.ID)
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(maxQueue, maxConcurrent int) *TaskScheduler {
	return &TaskScheduler{
		queue:         make([]*InferenceRequest, 0),
		processing:    make(map[string]*InferenceRequest),
		maxQueue:      maxQueue,
		maxConcurrent: maxConcurrent,
		priorities:    make(map[TaskPriority]int),
		stats:         &SchedulerStats{},
	}
}

// Start 启动调度器
func (s *TaskScheduler) Start() {
	// 启动调度循环
	go s.scheduleLoop()
}

// Stop 停止调度器
func (s *TaskScheduler) Stop() {
	// 清理队列
	s.mu.Lock()
	s.queue = make([]*InferenceRequest, 0)
	s.processing = make(map[string]*InferenceRequest)
	s.mu.Unlock()
}

// Submit 提交任务
func (s *TaskScheduler) Submit(request *InferenceRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) >= s.maxQueue {
		return "", fmt.Errorf("队列已满，当前长度: %d", len(s.queue))
	}

	requestID := uuid.New().String()
	request.ID = requestID
	request.CreatedAt = time.Now()

	// 按优先级插入
	inserted := false
	for i, req := range s.queue {
		if request.Priority > req.Priority {
			s.queue = append(s.queue[:i], append([]*InferenceRequest{request}, s.queue[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		s.queue = append(s.queue, request)
	}

	s.stats.TotalQueued++
	s.stats.QueueLength = len(s.queue)

	return requestID, nil
}

// QueueLength 获取队列长度
func (s *TaskScheduler) QueueLength() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.queue)
}

// scheduleLoop 调度循环
func (s *TaskScheduler) scheduleLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()

		// 检查是否有空闲槽位
		if len(s.processing) >= s.maxConcurrent || len(s.queue) == 0 {
			s.mu.Unlock()
			continue
		}

		// 取出最高优先级的任务
		request := s.queue[0]
		s.queue = s.queue[1:]
		s.processing[request.ID] = request
		s.stats.QueueLength = len(s.queue)
		s.stats.Processing = len(s.processing)

		s.mu.Unlock()

		// 计算等待时间
		waitTime := time.Since(request.CreatedAt)
		s.mu.Lock()
		s.stats.TotalProcessed++
		s.stats.AvgWaitTime = (s.stats.AvgWaitTime*float64(s.stats.TotalProcessed-1) + float64(waitTime.Milliseconds())) / float64(s.stats.TotalProcessed)
		if float64(waitTime.Milliseconds()) > s.stats.MaxWaitTime {
			s.stats.MaxWaitTime = float64(waitTime.Milliseconds())
		}
		s.mu.Unlock()
	}
}

// CompleteTask 完成任务
func (s *TaskScheduler) CompleteTask(requestID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.processing, requestID)
	s.stats.Processing = len(s.processing)
}

// InferCache 推理缓存
type InferCache struct {
	mu          sync.RWMutex
	results     map[string]interface{}
	models      map[string]interface{}
	asyncResults map[string]interface{}
	maxSize     int
	ttl         time.Duration
}

// NewInferCache 创建推理缓存
func NewInferCache(maxSize int, ttl time.Duration) *InferCache {
	cache := &InferCache{
		results:      make(map[string]interface{}),
		models:       make(map[string]interface{}),
		asyncResults: make(map[string]interface{}),
		maxSize:      maxSize,
		ttl:          ttl,
	}

	go cache.cleanup()

	return cache
}

// GetResult 获取推理结果缓存
func (c *InferCache) GetResult(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.results[key]
}

// SetResult 设置推理结果缓存
func (c *InferCache) SetResult(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[key] = value
}

// GetAsyncResult 获取异步推理结果
func (c *InferCache) GetAsyncResult(requestID string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.asyncResults[requestID]
}

// SetAsyncResult 设置异步推理结果
func (c *InferCache) SetAsyncResult(requestID string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asyncResults[requestID] = value
}

// GetModel 获取模型缓存
func (c *InferCache) GetModel(modelID string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.models[modelID]
}

// SetModel 设置模型缓存
func (c *InferCache) SetModel(modelID string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.models[modelID] = value
}

// RemoveModel 移除模型缓存
func (c *InferCache) RemoveModel(modelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.models, modelID)
}

// Clear 清空缓存
func (c *InferCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = make(map[string]interface{})
	c.models = make(map[string]interface{})
	c.asyncResults = make(map[string]interface{})
}

// cleanup 定期清理过期缓存
func (c *InferCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		// 简化实现：清理所有结果缓存
		c.results = make(map[string]interface{})
		c.asyncResults = make(map[string]interface{})
		c.mu.Unlock()
	}
}

// Cancel 取消任务
func (s *TaskScheduler) Cancel(requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从队列中移除
	for i, req := range s.queue {
		if req.ID == requestID {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			s.stats.QueueLength = len(s.queue)
			return nil
		}
	}

	// 从处理中移除
	if _, ok := s.processing[requestID]; ok {
		delete(s.processing, requestID)
		s.stats.Processing = len(s.processing)
		return nil
	}

	return fmt.Errorf("任务 %s 不存在", requestID)
}

// GetSchedulerStats 获取调度器统计
func (s *TaskScheduler) GetSchedulerStats() *SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := *s.stats
	return &stats
}
