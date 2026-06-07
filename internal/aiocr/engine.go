// engine.go - OCR 引擎核心
package aiocr

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine OCR 引擎.
type Engine struct {
	config        *Config
	preprocessor  *Preprocessor
	recognizer    *Recognizer
	extractor     *Extractor
	classifier    *Classifier
	validator     *Validator
	archiver      *Archiver
	compliance    *Compliance
	queue         chan *OCRRequest
	results       map[string]*OCRResult
	tasks         map[string]*BatchTask
	workers       int
	activeWorkers int
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	stats         *Stats
}

// NewEngine 创建 OCR 引擎.
func NewEngine(cfg *Config) (*Engine, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &Engine{
		config:       cfg,
		preprocessor: NewPreprocessor(cfg),
		recognizer:   NewRecognizer(cfg),
		extractor:    NewExtractor(cfg),
		classifier:   NewClassifier(cfg),
		validator:    NewValidator(cfg),
		archiver:     NewArchiver(cfg),
		compliance:   NewCompliance(cfg.Desensitize),
		queue:        make(chan *OCRRequest, cfg.QueueSize),
		results:      make(map[string]*OCRResult),
		tasks:        make(map[string]*BatchTask),
		workers:      cfg.Workers,
		ctx:          ctx,
		cancel:       cancel,
		stats:        &Stats{},
	}

	// 启动工作线程
	engine.startWorkers()

	log.Printf("✅ OCR 引擎已启动，工作线程: %d", cfg.Workers)
	return engine, nil
}

// startWorkers 启动工作线程.
func (e *Engine) startWorkers() {
	for i := 0; i < e.workers; i++ {
		go e.worker(i)
	}
}

// worker 工作线程.
func (e *Engine) worker(id int) {
	log.Printf("🔧 OCR 工作线程 %d 启动", id)
	for {
		select {
		case req := <-e.queue:
			e.processRequest(req)
		case <-e.ctx.Done():
			log.Printf("🔧 OCR 工作线程 %d 停止", id)
			return
		}
	}
}

// processRequest 处理 OCR 请求.
func (e *Engine) processRequest(req *OCRRequest) {
	start := time.Now()
	e.mu.Lock()
	e.activeWorkers++
	e.stats.TotalRequests++
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.activeWorkers--
		e.mu.Unlock()
	}()

	log.Printf("📄 处理 OCR 请求: %s, 文件: %s", req.ID, req.FilePath)

	result := &OCRResult{
		ID:        uuid.New().String(),
		RequestID: req.ID,
		FileID:    req.FileID,
		FileName:  req.FilePath,
		Language:  req.Language,
		CreatedAt: time.Now(),
		Metadata:  req.Metadata,
	}

	// 1. 图像预处理
	processed, err := e.preprocessor.Process(req.FilePath, req.Options)
	if err != nil {
		e.handleResultError(result, err)
		return
	}

	// 2. 文字识别
	pages, err := e.recognizer.Recognize(processed, req.Language, req.Options)
	if err != nil {
		e.handleResultError(result, err)
		return
	}
	result.Pages = pages

	// 3. 提取全文
	fullText := ""
	for _, page := range pages {
		fullText += page.Text + "\n"
	}
	result.FullText = fullText

	// 4. 文档分类
	classification := e.classifier.Classify(fullText, pages)
	result.Template = string(classification.Category)

	// 5. 结构化信息提取
	if req.Template != "" || classification.Confidence > 0.7 {
		structured := e.extractor.Extract(fullText, pages, classification.Category)
		result.Structured = structured
	}

	// 6. 结果校验
	e.validator.Validate(result)

	// 7. 敏感信息脱敏
	if req.Options != nil && req.Options.Desensitize {
		e.compliance.DesensitizeResult(result, req.Options.DesensitizeFields)
		result.Desensitized = true
	}

	// 8. 计算整体置信度
	totalConf := 0.0
	for _, page := range pages {
		totalConf += page.Confidence
	}
	if len(pages) > 0 {
		result.Confidence = totalConf / float64(len(pages))
	}

	// 9. 归档和索引
	if e.config.IndexEnabled {
		if err := e.archiver.Archive(result); err != nil {
			log.Printf("⚠️ 归档失败: %v", err)
		}
	}

	// 记录处理时间
	result.ProcessingMs = time.Since(start).Milliseconds()

	// 保存结果
	e.mu.Lock()
	e.results[result.ID] = result
	e.stats.SuccessRequests++
	e.stats.TotalPages += int64(len(pages))
	e.mu.Unlock()

	log.Printf("✅ OCR 完成: %s, 耗时: %dms, 置信度: %.2f",
		result.ID, result.ProcessingMs, result.Confidence)
}

// handleResultError 处理结果错误.
func (e *Engine) handleResultError(result *OCRResult, err error) {
	result.Confidence = 0
	result.ProcessingMs = 0

	e.mu.Lock()
	e.results[result.ID] = result
	e.stats.FailedRequests++
	e.mu.Unlock()

	log.Printf("❌ OCR 失败: %s, 错误: %v", result.ID, err)
}

// Submit 提交 OCR 请求.
func (e *Engine) Submit(req *OCRRequest) (string, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}

	select {
	case e.queue <- req:
		log.Printf("📥 OCR 请求已入队: %s", req.ID)
		return req.ID, nil
	default:
		return "", fmt.Errorf("队列已满，请稍后重试")
	}
}

// SubmitBatch 提交批量 OCR 请求.
func (e *Engine) SubmitBatch(files []string, options *OCROptions) (string, error) {
	task := &BatchTask{
		ID:         uuid.New().String(),
		Status:     BatchStatusPending,
		TotalFiles: len(files),
		Options:    options,
		Results:    make([]*OCRResult, 0, len(files)),
		CreatedAt:  time.Now(),
	}

	e.mu.Lock()
	e.tasks[task.ID] = task
	e.mu.Unlock()

	// 异步处理批量任务
	go e.processBatch(task, files)

	log.Printf("📥 批量任务已创建: %s, 文件数: %d", task.ID, len(files))
	return task.ID, nil
}

// processBatch 处理批量任务.
func (e *Engine) processBatch(task *BatchTask, files []string) {
	now := time.Now()
	task.StartedAt = &now
	task.Status = BatchStatusProcessing

	for i, file := range files {
		req := &OCRRequest{
			ID:        uuid.New().String(),
			FilePath:  file,
			Options:   task.Options,
			BatchID:   task.ID,
			Priority:  1,
			CreatedAt: time.Now(),
		}

		// 同步处理
		e.processRequest(req)

		e.mu.Lock()
		result, exists := e.results[req.ID]
		if exists {
			task.Results = append(task.Results, result)
			task.Processed++
		} else {
			task.Failed++
		}
		e.mu.Unlock()

		log.Printf("📊 批量进度: %d/%d", i+1, len(files))
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt
	task.Status = BatchStatusCompleted

	if task.Failed > 0 {
		task.Status = BatchStatusCompleted
		task.Errors = append(task.Errors, fmt.Sprintf("%d 个文件处理失败", task.Failed))
	}

	log.Printf("✅ 批量任务完成: %s, 成功: %d, 失败: %d",
		task.ID, task.Processed, task.Failed)
}

// GetResult 获取识别结果.
func (e *Engine) GetResult(id string) (*OCRResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result, exists := e.results[id]
	if !exists {
		return nil, fmt.Errorf("结果不存在: %s", id)
	}
	return result, nil
}

// GetBatchTask 获取批量任务.
func (e *Engine) GetBatchTask(id string) (*BatchTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, exists := e.tasks[id]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}
	return task, nil
}

// GetStats 获取统计信息.
func (e *Engine) GetStats() *Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	stats.QueueLength = len(e.queue)
	stats.ActiveWorkers = e.activeWorkers
	return &stats
}

// Recognize 同步识别（单个文件）.
func (e *Engine) Recognize(filePath string, options *OCROptions) (*OCRResult, error) {
	req := &OCRRequest{
		ID:        uuid.New().String(),
		FilePath:  filePath,
		Options:   options,
		Priority:  10,
		CreatedAt: time.Now(),
	}

	// 同步处理
	e.processRequest(req)

	// 查找结果（通过 RequestID）
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, result := range e.results {
		if result.RequestID == req.ID {
			return result, nil
		}
	}
	return nil, fmt.Errorf("识别失败: 结果未找到")
}

// Close 关闭引擎.
func (e *Engine) Close() {
	e.cancel()
	log.Println("🛑 OCR 引擎已关闭")
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		Engine:          "builtin",
		DefaultLanguage: "chi_sim+eng",
		Languages:       []string{"chi_sim", "chi_tra", "eng", "jpn", "kor"},
		MaxFileSize:     50 * 1024 * 1024, // 50MB
		MaxPages:        100,
		Workers:         4,
		QueueSize:       100,
		Desensitize: &DesensitizeConfig{
			Enabled:         true,
			IDCardPattern:   `\d{17}[\dXx]`,
			BankCardPattern: `\d{16,19}`,
			PhonePattern:    `1[3-9]\d{9}`,
			EmailPattern:    `[\w.]+@[\w.]+\.\w+`,
			MaskChar:        "*",
			KeepPrefix:      4,
			KeepSuffix:      4,
		},
		ArchivePath:   "/var/nas-os/ocr/archive",
		IndexEnabled:  true,
		RetentionDays: 90,
	}
}
