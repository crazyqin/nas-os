package aiphoto

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// QueueState 队列状态
type QueueState string

const (
	QueueStateIdle     QueueState = "idle"
	QueueStateRunning  QueueState = "running"
	QueueStatePaused   QueueState = "paused"
	QueueStateStopping QueueState = "stopping"
)

// QueueConfig 队列配置
type QueueConfig struct {
	MaxConcurrent int           `json:"maxConcurrent"` // 最大并发数，默认 2
	MaxRetries    int           `json:"maxRetries"`    // 最大重试次数，默认 3
	RetryDelay    time.Duration `json:"retryDelay"`    // 重试延迟
	TaskTimeout   time.Duration `json:"taskTimeout"`   // 单任务超时
	OutputDir     string        `json:"outputDir"`     // 输出目录
	SaveProgress  bool          `json:"saveProgress"`  // 是否保存进度
}

// DefaultQueueConfig 默认队列配置
func DefaultQueueConfig() *QueueConfig {
	return &QueueConfig{
		MaxConcurrent: 2,
		MaxRetries:    3,
		RetryDelay:    time.Second * 5,
		TaskTimeout:   time.Minute * 30,
		OutputDir:     "output",
		SaveProgress:  true,
	}
}

// TaskProcessor 任务处理函数
type TaskProcessor func(ctx context.Context, task *PhotoTask) (*ProcessResult, error)

// Queue 批量处理队列
type Queue struct {
	mu         sync.RWMutex
	tasks      map[string]*PhotoTask
	queue      []*PhotoTask // 等待队列（按优先级排序）
	state      QueueState
	config     *QueueConfig
	processors map[TaskType]TaskProcessor
	dataDir    string
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	notifyChan chan *PhotoTask
	onComplete func(task *PhotoTask, result *ProcessResult)
	onProgress func(task *PhotoTask, progress float64)
}

// NewQueue 创建批量处理队列
func NewQueue(dataDir string, config *QueueConfig) *Queue {
	if config == nil {
		config = DefaultQueueConfig()
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 2
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.OutputDir == "" {
		config.OutputDir = "output"
	}

	ctx, cancel := context.WithCancel(context.Background())

	q := &Queue{
		tasks:      make(map[string]*PhotoTask),
		queue:      make([]*PhotoTask, 0),
		state:      QueueStateIdle,
		config:     config,
		processors: make(map[TaskType]TaskProcessor),
		dataDir:    dataDir,
		ctx:        ctx,
		cancel:     cancel,
		notifyChan: make(chan *PhotoTask, 100),
	}

	// 注册默认处理器
	q.RegisterProcessor(TaskTypeDenoise, q.defaultDenoiseProcessor)
	q.RegisterProcessor(TaskTypeUpscale, q.defaultUpscaleProcessor)
	q.RegisterProcessor(TaskTypeRestore, q.defaultRestoreProcessor)
	q.RegisterProcessor(TaskTypeSmartCrop, q.defaultSmartCropProcessor)

	// 加载保存的任务
	if config.SaveProgress {
		q.loadTasks()
	}

	return q
}

// RegisterProcessor 注册任务处理器
func (q *Queue) RegisterProcessor(taskType TaskType, processor TaskProcessor) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.processors[taskType] = processor
}

// SetOnComplete 设置完成回调
func (q *Queue) SetOnComplete(fn func(task *PhotoTask, result *ProcessResult)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onComplete = fn
}

// SetOnProgress 设置进度回调
func (q *Queue) SetOnProgress(fn func(task *PhotoTask, progress float64)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onProgress = fn
}

// Submit 提交任务
func (q *Queue) Submit(taskType TaskType, inputPath string, options interface{}) (*PhotoTask, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 验证输入文件
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("输入文件不存在：%s", inputPath)
	}

	// 生成任务 ID
	taskID := fmt.Sprintf("%s_%d", taskType, time.Now().UnixNano())

	// 生成输出路径
	ext := filepath.Ext(inputPath)
	baseName := inputPath[:len(inputPath)-len(ext)]
	outputPath := filepath.Join(q.config.OutputDir, fmt.Sprintf("%s_%s%s", filepath.Base(baseName), taskType, ext))

	task := &PhotoTask{
		ID:         taskID,
		Type:       taskType,
		Status:     TaskStatusPending,
		InputPath:  inputPath,
		OutputPath: outputPath,
		Options:    options,
		Progress:   0,
		CreatedAt:  time.Now(),
	}

	q.tasks[taskID] = task
	q.queue = append(q.queue, task)

	// 通知队列
	select {
	case q.notifyChan <- task:
	default:
	}

	// 保存任务状态
	if q.config.SaveProgress {
		q.saveTasks()
	}

	return task, nil
}

// Start 启动队列处理
func (q *Queue) Start() {
	q.mu.Lock()
	if q.state == QueueStateRunning {
		q.mu.Unlock()
		return
	}
	q.state = QueueStateRunning
	q.mu.Unlock()

	// 启动工作协程
	for i := 0; i < q.config.MaxConcurrent; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

// Stop 停止队列处理
func (q *Queue) Stop(wait bool) {
	q.mu.Lock()
	q.state = QueueStateStopping
	q.mu.Unlock()

	q.cancel()

	if wait {
		q.wg.Wait()
	}

	q.mu.Lock()
	q.state = QueueStateIdle
	q.mu.Unlock()
}

// Pause 暂停队列
func (q *Queue) Pause() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.state == QueueStateRunning {
		q.state = QueueStatePaused
	}
}

// Resume 恢复队列
func (q *Queue) Resume() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.state == QueueStatePaused {
		q.state = QueueStateRunning
		// 通知所有等待的工作协程
		for i := 0; i < q.config.MaxConcurrent; i++ {
			select {
			case q.notifyChan <- nil:
			default:
			}
		}
	}
}

// GetTask 获取任务状态
func (q *Queue) GetTask(taskID string) (*PhotoTask, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在：%s", taskID)
	}

	taskCopy := *task
	return &taskCopy, nil
}

// ListTasks 列出任务
func (q *Queue) ListTasks(status TaskStatus) []*PhotoTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*PhotoTask, 0)
	for _, task := range q.tasks {
		if status != "" && task.Status != status {
			continue
		}
		taskCopy := *task
		result = append(result, &taskCopy)
	}

	return result
}

// CancelTask 取消任务
func (q *Queue) CancelTask(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在：%s", taskID)
	}

	if task.Status == TaskStatusPending {
		task.Status = TaskStatusCancelled
		// 从队列中移除
		for i, t := range q.queue {
			if t.ID == taskID {
				q.queue = append(q.queue[:i], q.queue[i+1:]...)
				break
			}
		}
	}

	if q.config.SaveProgress {
		q.saveTasks()
	}

	return nil
}

// RetryTask 重试失败的任务
func (q *Queue) RetryTask(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在：%s", taskID)
	}

	if task.Status != TaskStatusFailed {
		return fmt.Errorf("只能重试失败的任务")
	}

	task.Status = TaskStatusPending
	task.Error = ""
	task.Progress = 0
	q.queue = append(q.queue, task)

	// 通知队列
	select {
	case q.notifyChan <- task:
	default:
	}

	if q.config.SaveProgress {
		q.saveTasks()
	}

	return nil
}

// GetState 获取队列状态
func (q *Queue) GetState() QueueState {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.state
}

// GetStats 获取队列统计
func (q *Queue) GetStats() map[string]int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := map[string]int{
		"total":      len(q.tasks),
		"pending":    0,
		"processing": 0,
		"completed":  0,
		"failed":     0,
		"cancelled":  0,
	}

	for _, task := range q.tasks {
		switch task.Status {
		case TaskStatusPending:
			stats["pending"]++
		case TaskStatusProcessing:
			stats["processing"]++
		case TaskStatusCompleted:
			stats["completed"]++
		case TaskStatusFailed:
			stats["failed"]++
		case TaskStatusCancelled:
			stats["cancelled"]++
		}
	}

	return stats
}

// worker 工作协程
func (q *Queue) worker(id int) {
	defer q.wg.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.notifyChan:
			// 检查队列状态
			q.mu.RLock()
			state := q.state
			q.mu.RUnlock()

			if state != QueueStateRunning {
				if state == QueueStateStopping {
					return
				}
				// 暂停状态，等待恢复
				time.Sleep(time.Second)
				continue
			}

			// 获取下一个任务
			task := q.getNextTask()
			if task == nil {
				continue
			}

			// 处理任务
			q.processTask(task)
		}
	}
}

// getNextTask 获取下一个待处理任务
func (q *Queue) getNextTask() *PhotoTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return nil
	}

	task := q.queue[0]
	q.queue = q.queue[1:]

	task.Status = TaskStatusProcessing
	now := time.Now()
	task.StartedAt = &now

	return task
}

// processTask 处理任务
func (q *Queue) processTask(task *PhotoTask) {
	q.mu.RLock()
	processor, exists := q.processors[task.Type]
	onComplete := q.onComplete
	onProgress := q.onProgress
	q.mu.RUnlock()

	if !exists {
		q.failTask(task, fmt.Errorf("未注册的任务处理器：%s", task.Type))
		return
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(q.ctx, q.config.TaskTimeout)
	defer cancel()

	// 设置进度回调
	progressCallback := func(progress float64) {
		q.mu.Lock()
		task.Progress = progress
		q.mu.Unlock()

		if onProgress != nil {
			onProgress(task, progress)
		}
	}

	// 包装处理器以支持进度报告
	wrappedProcessor := func(ctx context.Context, t *PhotoTask) (*ProcessResult, error) {
		progressCallback(0)
		result, err := processor(ctx, t)
		if err == nil {
			progressCallback(100)
		}
		return result, err
	}

	// 执行处理
	result, err := wrappedProcessor(ctx, task)

	now := time.Now()
	task.CompletedAt = &now
	task.Duration = now.Sub(*task.StartedAt)

	if err != nil {
		q.failTask(task, err)
		return
	}

	// 成功
	q.mu.Lock()
	task.Status = TaskStatusCompleted
	task.Progress = 100
	q.mu.Unlock()

	if q.config.SaveProgress {
		q.saveTasks()
	}

	if onComplete != nil {
		onComplete(task, result)
	}
}

// failTask 标记任务失败
func (q *Queue) failTask(task *PhotoTask, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.Status = TaskStatusFailed
	task.Error = err.Error()

	if q.config.SaveProgress {
		q.saveTasks()
	}
}

// saveTasks 保存任务状态
func (q *Queue) saveTasks() error {
	path := filepath.Join(q.dataDir, "aiphoto-tasks.json")
	tasks := make([]*PhotoTask, 0, len(q.tasks))
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// loadTasks 加载任务状态
func (q *Queue) loadTasks() error {
	path := filepath.Join(q.dataDir, "aiphoto-tasks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tasks []*PhotoTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	for _, task := range tasks {
		q.tasks[task.ID] = task
		if task.Status == TaskStatusPending {
			q.queue = append(q.queue, task)
		}
	}

	return nil
}

// 默认处理器

func (q *Queue) defaultDenoiseProcessor(ctx context.Context, task *PhotoTask) (*ProcessResult, error) {
	opts := DefaultDenoiseOptions()
	if task.Options != nil {
		if o, ok := task.Options.(*DenoiseOptions); ok {
			opts = o
		}
	}

	denoiser := NewDenoiser(opts)
	return q.processWithImage(ctx, task, func(img interface{}) (interface{}, error) {
		return denoiser.Denoise(ctx, img.(image.Image))
	})
}

func (q *Queue) defaultUpscaleProcessor(ctx context.Context, task *PhotoTask) (*ProcessResult, error) {
	opts := DefaultUpscaleOptions()
	if task.Options != nil {
		if o, ok := task.Options.(*UpscaleOptions); ok {
			opts = o
		}
	}

	upscaler := NewUpscaler(opts)
	return q.processWithImage(ctx, task, func(img interface{}) (interface{}, error) {
		return upscaler.Upscale(ctx, img.(image.Image))
	})
}

func (q *Queue) defaultRestoreProcessor(ctx context.Context, task *PhotoTask) (*ProcessResult, error) {
	opts := DefaultRestoreOptions()
	if task.Options != nil {
		if o, ok := task.Options.(*RestoreOptions); ok {
			opts = o
		}
	}

	restorer := NewRestorer(opts)
	return q.processWithImage(ctx, task, func(img interface{}) (interface{}, error) {
		return restorer.Restore(ctx, img.(image.Image))
	})
}

func (q *Queue) defaultSmartCropProcessor(ctx context.Context, task *PhotoTask) (*ProcessResult, error) {
	opts := DefaultSmartCropOptions()
	if task.Options != nil {
		if o, ok := task.Options.(*SmartCropOptions); ok {
			opts = o
		}
	}

	cropper := NewSmartCropper(opts)
	return q.processWithImage(ctx, task, func(img interface{}) (interface{}, error) {
		result, err := cropper.SmartCrop(ctx, img.(image.Image))
		if err != nil {
			return nil, err
		}
		return result.Image, nil
	})
}

// processWithImage 通用图像处理流程
func (q *Queue) processWithImage(ctx context.Context, task *PhotoTask, processor func(interface{}) (interface{}, error)) (*ProcessResult, error) {
	// 这里是占位实现，实际需要加载/保存图片
	return &ProcessResult{
		TaskID:     task.ID,
		Success:    true,
		OutputPath: task.OutputPath,
	}, nil
}
