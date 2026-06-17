// Package ailocalinfer provides local AI inference engine for NAS-OS
// infer.go - Main inference engine implementation
package ailocalinfer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// engine implements the InferenceEngine interface
type engine struct {
	config      *InferenceEngineConfig
	models      map[string]*Model
	modelCache  *ModelCache
	gpuInfo     *GPUInfo
	metrics     *MetricsCollector
	workerPool  chan struct{}
	mu          sync.RWMutex
	initialized bool
	closed      bool
}

// NewInferenceEngine creates a new local inference engine
func NewInferenceEngine(cfg *InferenceEngineConfig) (InferenceEngine, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Ensure model directory exists
	if err := os.MkdirAll(cfg.ModelDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

	// Set defaults
	if cfg.MaxModels <= 0 {
		cfg.MaxModels = 10
	}
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = 32
	}
	if cfg.CacheSize <= 0 {
		cfg.CacheSize = 5
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}

	e := &engine{
		config:     cfg,
		models:     make(map[string]*Model),
		modelCache: NewModelCache(int64(cfg.CacheSize)),
		metrics:    NewMetricsCollector(),
		workerPool: make(chan struct{}, cfg.WorkerCount),
	}

	// Initialize worker pool
	for i := 0; i < cfg.WorkerCount; i++ {
		e.workerPool <- struct{}{}
	}

	// Detect GPU
	if cfg.EnableGPU {
		e.gpuInfo = e.detectGPU()
	}

	// Scan existing models
	if err := e.scanModels(); err != nil {
		log.Printf("Warning: failed to scan models: %v", err)
	}

	e.initialized = true
	return e, nil
}

// LoadModel loads a model into memory
func (e *engine) LoadModel(ctx context.Context, modelName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	model, exists := e.models[modelName]
	if !exists {
		return fmt.Errorf("model %s not found", modelName)
	}

	if model.Status == ModelStatusReady {
		return nil // Already loaded
	}

	if model.Status == ModelStatusLoading {
		return fmt.Errorf("model %s is currently loading", modelName)
	}

	// Check if we need to unload another model due to cache limits
	if err := e.ensureCacheSpace(model); err != nil {
		return fmt.Errorf("failed to ensure cache space: %w", err)
	}

	model.Status = ModelStatusLoading
	model.LastError = ""

	// Simulate model loading (in real implementation, this would load the actual model)
	go func() {
		if err := e.doLoadModel(ctx, model); err != nil {
			e.mu.Lock()
			model.Status = ModelStatusError
			model.LastError = err.Error()
			e.mu.Unlock()
			log.Printf("Failed to load model %s: %v", modelName, err)
			return
		}

		e.mu.Lock()
		model.Status = ModelStatusReady
		now := time.Now()
		model.LoadedAt = &now
		e.mu.Unlock()

		log.Printf("Model %s loaded successfully", modelName)
	}()

	return nil
}

// doLoadModel performs the actual model loading
func (e *engine) doLoadModel(ctx context.Context, model *Model) error {
	// Check GPU requirements
	if model.GPURequired && (e.gpuInfo == nil || !e.gpuInfo.Available) {
		return fmt.Errorf("model %s requires GPU but no GPU available", model.Name)
	}

	// Check if model file exists
	if _, err := os.Stat(model.Path); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", model.Path)
	}

	// Simulate loading time
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
		// Loading complete
	}

	return nil
}

// UnloadModel unloads a model from memory
func (e *engine) UnloadModel(ctx context.Context, modelName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("engine is closed")
	}

	model, exists := e.models[modelName]
	if !exists {
		return fmt.Errorf("model %s not found", modelName)
	}

	if model.Status != ModelStatusReady {
		return fmt.Errorf("model %s is not loaded", modelName)
	}

	model.Status = ModelStatusUnloading

	// Simulate unloading
	go func() {
		// In real implementation, this would release model resources
		time.Sleep(100 * time.Millisecond)

		e.mu.Lock()
		model.Status = ModelStatusUnloaded
		model.LoadedAt = nil
		e.mu.Unlock()

		// Remove from cache
		e.modelCache.Remove(modelName)

		log.Printf("Model %s unloaded", modelName)
	}()

	return nil
}

// ListModels lists all available models
func (e *engine) ListModels(ctx context.Context) ([]ModelInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return nil, fmt.Errorf("engine is closed")
	}

	models := make([]ModelInfo, 0, len(e.models))
	for _, m := range e.models {
		models = append(models, ModelInfo{
			Name:         m.Name,
			Type:         m.Type,
			Size:         m.Size,
			Parameters:   m.Parameters,
			Quantization: m.Quantization,
			GPURequired:  m.GPURequired,
			GPUMemoryMB:  m.GPUMemoryMB,
			Capabilities: e.getModelCapabilities(m),
			Metadata:     m.Metadata,
		})
	}

	return models, nil
}

// getModelCapabilities returns capabilities for a model
func (e *engine) getModelCapabilities(model *Model) []string {
	capabilities := []string{}
	switch model.Type {
	case ModelTypeTextGeneration:
		capabilities = append(capabilities, "text_generation", "chat")
	case ModelTypeImageRecognition:
		capabilities = append(capabilities, "image_recognition", "object_detection")
	case ModelTypeSpeechRecognition:
		capabilities = append(capabilities, "speech_recognition", "transcription")
	case ModelTypeEmbedding:
		capabilities = append(capabilities, "embedding")
	}
	return capabilities
}

// GetModelStatus gets the status of a model
func (e *engine) GetModelStatus(ctx context.Context, modelName string) (*Model, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return nil, fmt.Errorf("engine is closed")
	}

	model, exists := e.models[modelName]
	if !exists {
		return nil, fmt.Errorf("model %s not found", modelName)
	}

	// Return a copy
	copy := *model
	return &copy, nil
}

// Inference runs inference on a single request
func (e *engine) Inference(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	if e.closed {
		return nil, fmt.Errorf("engine is closed")
	}

	// Validate request
	if err := e.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Generate request ID if not provided
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}

	// Get worker from pool
	select {
	case <-e.workerPool:
		defer func() { e.workerPool <- struct{}{} }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Record start time
	start := time.Now()

	// Run inference
	resp, err := e.doInference(ctx, req)

	// Record metrics
	latency := time.Since(start)
	if e.config.EnableMetrics {
		e.metrics.Record(latency, err == nil)
	}

	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	resp.LatencyMS = latency.Milliseconds()
	return resp, nil
}

// doInference performs the actual inference
func (e *engine) doInference(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	e.mu.RLock()
	model, exists := e.models[req.ModelName]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model %s not found", req.ModelName)
	}

	if model.Status != ModelStatusReady {
		return nil, fmt.Errorf("model %s is not ready (status: %s)", req.ModelName, model.Status)
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, e.config.RequestTimeout)
	defer cancel()

	// Route to appropriate handler based on model type
	var output interface{}
	var err error

	switch model.Type {
	case ModelTypeTextGeneration:
		output, err = e.handleTextGeneration(timeoutCtx, model, req)
	case ModelTypeImageRecognition:
		output, err = e.handleImageRecognition(timeoutCtx, model, req)
	case ModelTypeSpeechRecognition:
		output, err = e.handleSpeechRecognition(timeoutCtx, model, req)
	default:
		return nil, fmt.Errorf("unsupported model type: %s", model.Type)
	}

	if err != nil {
		return nil, err
	}

	return &InferenceResponse{
		RequestID: req.RequestID,
		ModelName: req.ModelName,
		Output:    output,
		Metadata:  req.Metadata,
	}, nil
}

// handleTextGeneration handles text generation requests
func (e *engine) handleTextGeneration(ctx context.Context, model *Model, req *InferenceRequest) (*TextGenerationOutput, error) {
	input, ok := req.Input.(*TextGenerationInput)
	if !ok {
		// Try to convert from map
		inputData, err := json.Marshal(req.Input)
		if err != nil {
			return nil, fmt.Errorf("invalid input format")
		}
		input = &TextGenerationInput{}
		if err := json.Unmarshal(inputData, input); err != nil {
			return nil, fmt.Errorf("invalid input format: %w", err)
		}
	}

	if input.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// In real implementation, this would call the model backend
	// For now, simulate inference
	maxTokens := req.Options.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 512
	}

	// Simulate processing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	// Generate simulated response
	text := fmt.Sprintf("Generated response for prompt: %s", input.Prompt)
	if len(text) > maxTokens*4 { // Approximate 4 chars per token
		text = text[:maxTokens*4]
	}

	return &TextGenerationOutput{
		Text:         text,
		FinishReason: "stop",
	}, nil
}

// handleImageRecognition handles image recognition requests
func (e *engine) handleImageRecognition(ctx context.Context, model *Model, req *InferenceRequest) (*ImageRecognitionOutput, error) {
	input, ok := req.Input.(*ImageRecognitionInput)
	if !ok {
		// Try to convert from map
		inputData, err := json.Marshal(req.Input)
		if err != nil {
			return nil, fmt.Errorf("invalid input format")
		}
		input = &ImageRecognitionInput{}
		if err := json.Unmarshal(inputData, input); err != nil {
			return nil, fmt.Errorf("invalid input format: %w", err)
		}
	}

	if len(input.ImageData) == 0 && input.ImageURL == "" {
		return nil, fmt.Errorf("image data or URL is required")
	}

	// Simulate processing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}

	return &ImageRecognitionOutput{
		Description: "A sample image description",
		Labels: []LabelPrediction{
			{Name: "object", Confidence: 0.95},
			{Name: "scene", Confidence: 0.85},
		},
		Objects: []ObjectDetection{
			{
				Name:       "object",
				Confidence: 0.95,
				BBox: BoundingBox{
					X:      10,
					Y:      10,
					Width:  100,
					Height: 100,
				},
			},
		},
	}, nil
}

// handleSpeechRecognition handles speech recognition requests
func (e *engine) handleSpeechRecognition(ctx context.Context, model *Model, req *InferenceRequest) (*SpeechRecognitionOutput, error) {
	input, ok := req.Input.(*SpeechRecognitionInput)
	if !ok {
		// Try to convert from map
		inputData, err := json.Marshal(req.Input)
		if err != nil {
			return nil, fmt.Errorf("invalid input format")
		}
		input = &SpeechRecognitionInput{}
		if err := json.Unmarshal(inputData, input); err != nil {
			return nil, fmt.Errorf("invalid input format: %w", err)
		}
	}

	if len(input.AudioData) == 0 && input.AudioURL == "" {
		return nil, fmt.Errorf("audio data or URL is required")
	}

	// Simulate processing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(300 * time.Millisecond):
	}

	return &SpeechRecognitionOutput{
		Text:       "This is a sample transcription",
		Language:   input.Language,
		Confidence: 0.95,
		Segments: []Segment{
			{
				Text:       "This is a sample transcription",
				Start:      0.0,
				End:        5.0,
				Confidence: 0.95,
			},
		},
	}, nil
}

// BatchInference runs inference on a batch of requests
func (e *engine) BatchInference(ctx context.Context, req *BatchRequest) (*BatchResponse, error) {
	if e.closed {
		return nil, fmt.Errorf("engine is closed")
	}

	if len(req.Requests) == 0 {
		return nil, fmt.Errorf("batch request is empty")
	}

	if len(req.Requests) > e.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(req.Requests), e.config.MaxBatchSize)
	}

	// Generate batch ID if not provided
	if req.BatchID == "" {
		req.BatchID = uuid.New().String()
	}

	start := time.Now()

	// Set concurrency limit
	maxConcurrency := req.Options.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = e.config.WorkerCount
	}
	semaphore := make(chan struct{}, maxConcurrency)

	// Process requests concurrently
	responses := make([]InferenceResponse, len(req.Requests))
	errors := make([]BatchError, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, inferenceReq := range req.Requests {
		wg.Add(1)
		go func(idx int, r InferenceRequest) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			resp, err := e.Inference(ctx, &r)
			if err != nil {
				mu.Lock()
				errors = append(errors, BatchError{
					Index:     idx,
					RequestID: r.RequestID,
					Error:     err.Error(),
				})
				mu.Unlock()

				if req.Options.FailOnFirst {
					return
				}
			} else {
				responses[idx] = *resp
			}
		}(i, inferenceReq)
	}

	wg.Wait()

	return &BatchResponse{
		BatchID:   req.BatchID,
		Responses: responses,
		Errors:    errors,
		TotalMS:   time.Since(start).Milliseconds(),
	}, nil
}

// GetGPUInfo gets GPU information
func (e *engine) GetGPUInfo(ctx context.Context) (*GPUInfo, error) {
	if e.gpuInfo == nil {
		return &GPUInfo{Available: false}, nil
	}

	// Return a copy
	copy := *e.gpuInfo
	return &copy, nil
}

// GetMetrics gets performance metrics
func (e *engine) GetMetrics(ctx context.Context) (*PerformanceMetrics, error) {
	return e.metrics.GetMetrics(), nil
}

// Close closes the engine and releases resources
func (e *engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	e.closed = true

	// Unload all models
	for name, model := range e.models {
		if model.Status == ModelStatusReady {
			model.Status = ModelStatusUnloaded
			log.Printf("Unloaded model %s during shutdown", name)
		}
	}

	// Clear cache
	e.modelCache.Clear()

	return nil
}

// validateRequest validates an inference request
func (e *engine) validateRequest(req *InferenceRequest) error {
	if req.ModelName == "" {
		return fmt.Errorf("model name is required")
	}
	if req.Input == nil {
		return fmt.Errorf("input is required")
	}
	return nil
}

// ensureCacheSpace ensures there's enough cache space for a model
func (e *engine) ensureCacheSpace(model *Model) error {
	if e.modelCache.Size()+1 <= e.config.CacheSize {
		return nil
	}

	// Evict least recently used model
	evicted := e.modelCache.Evict()
	if evicted != "" {
		if m, exists := e.models[evicted]; exists {
			m.Status = ModelStatusUnloaded
			m.LoadedAt = nil
			log.Printf("Evicted model %s from cache", evicted)
		}
	}

	return nil
}

// scanModels scans the model directory for existing models
func (e *engine) scanModels() error {
	entries, err := os.ReadDir(e.config.ModelDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check for model files
		name := entry.Name()
		if e.isModelFile(name) {
			info, _ := entry.Info()
			modelType := e.detectModelType(name)
			model := &Model{
				Name:   strings.TrimSuffix(name, filepath.Ext(name)),
				Type:   modelType,
				Path:   filepath.Join(e.config.ModelDir, name),
				Size:   info.Size(),
				Status: ModelStatusUnloaded,
				GPURequired: e.isGPURequired(modelType),
			}
			e.models[model.Name] = model
		}
	}

	return nil
}

// isModelFile checks if a file is a model file
func (e *engine) isModelFile(name string) bool {
	extensions := []string{".gguf", ".ggml", ".bin", ".onnx", ".pt", ".pth", ".safetensors"}
	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(name), ext) {
			return true
		}
	}
	return false
}

// detectModelType detects model type from filename
func (e *engine) detectModelType(name string) ModelType {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "whisper") || strings.Contains(lower, "asr"):
		return ModelTypeSpeechRecognition
	case strings.Contains(lower, "clip") || strings.Contains(lower, "vit"):
		return ModelTypeImageRecognition
	case strings.Contains(lower, "embed") || strings.Contains(lower, "bge"):
		return ModelTypeEmbedding
	default:
		return ModelTypeTextGeneration
	}
}

// isGPURequired checks if a model type typically requires GPU
func (e *engine) isGPURequired(modelType ModelType) bool {
	switch modelType {
	case ModelTypeImageRecognition:
		return true
	default:
		return false
	}
}

// detectGPU detects available GPU
func (e *engine) detectGPU() *GPUInfo {
	// Try nvidia-smi for NVIDIA GPUs
	if output, err := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,memory.used,driver_version", "--format=csv,noheader,nounits").Output(); err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), ",")
		if len(parts) >= 4 {
			memTotal := parseInt(strings.TrimSpace(parts[1]))
			memUsed := parseInt(strings.TrimSpace(parts[2]))
			return &GPUInfo{
				Available:    true,
				Type:         "nvidia",
				Name:         strings.TrimSpace(parts[0]),
				MemoryMB:     memTotal,
				MemoryUsedMB: memUsed,
				Driver:       strings.TrimSpace(parts[3]),
			}
		}
	}

	// Try rocm-smi for AMD GPUs
	if output, err := exec.Command("rocm-smi", "--showmeminfo", "vram").Output(); err == nil {
		if strings.Contains(string(output), "GPU") {
			return &GPUInfo{
				Available: true,
				Type:      "amd",
				Name:      "AMD GPU",
			}
		}
	}

	// Try Intel GPU
	if output, err := exec.Command("intel_gpu_top", "-l").Output(); err == nil {
		if strings.Contains(string(output), "Intel") {
			return &GPUInfo{
				Available: true,
				Type:      "intel",
				Name:      "Intel GPU",
			}
		}
	}

	return &GPUInfo{Available: false}
}

// parseInt parses a string to int, returns 0 on error
func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

// ==================== LRU Cache Implementation ====================

// NewLRUCache creates a new LRU cache
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		items:    make(map[string]*CacheItem),
		order:    NewDoublyLinkedList(),
	}
}

// Get gets an item from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Move to front (most recently used)
	c.order.MoveToFront(item.Element)
	item.AccessAt = time.Now()
	item.Frequency++

	return item.Value, true
}

// Put puts an item into the cache
func (c *LRUCache) Put(key string, value interface{}, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if item, exists := c.items[key]; exists {
		item.Value = value
		item.Size = size
		item.AccessAt = time.Now()
		item.Frequency++
		c.order.MoveToFront(item.Element)
		return
	}

	// Evict if at capacity
	if c.order.Size() >= c.capacity {
		c.evict()
	}

	// Add new item
	element := c.order.PushFront(key)
	c.items[key] = &CacheItem{
		Key:       key,
		Value:     value,
		Size:      size,
		AccessAt:  time.Now(),
		Frequency: 1,
		Element:   element,
	}
}

// Remove removes an item from the cache
func (c *LRUCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists {
		c.order.Remove(item.Element)
		delete(c.items, key)
	}
}

// evict removes the least recently used item
func (c *LRUCache) evict() {
	back := c.order.Back()
	if back == nil {
		return
	}

	key := back.key
	if item, exists := c.items[key]; exists {
		c.order.Remove(item.Element)
		delete(c.items, key)
	}
}

// Size returns the number of items in the cache
func (c *LRUCache) Size() int {
	return c.order.Size()
}

// NewDoublyLinkedList creates a new doubly linked list
func NewDoublyLinkedList() *DoublyLinkedList {
	head := &ListNode{}
	tail := &ListNode{}
	head.next = tail
	tail.prev = head

	return &DoublyLinkedList{
		head: head,
		tail: tail,
	}
}

// PushFront pushes an item to the front
func (l *DoublyLinkedList) PushFront(key string) *ListNode {
	node := &ListNode{key: key}
	node.next = l.head.next
	node.prev = l.head
	l.head.next.prev = node
	l.head.next = node
	l.size++
	return node
}

// Remove removes a node
func (l *DoublyLinkedList) Remove(node *ListNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
	l.size--
}

// MoveToFront moves a node to the front
func (l *DoublyLinkedList) MoveToFront(node *ListNode) {
	l.Remove(node)
	l.PushFront(node.key)
}

// Back returns the last node
func (l *DoublyLinkedList) Back() *ListNode {
	if l.size == 0 {
		return nil
	}
	return l.tail.prev
}

// Size returns the size of the list
func (l *DoublyLinkedList) Size() int {
	return l.size
}

// ==================== Model Cache Implementation ====================

// NewModelCache creates a new model cache
func NewModelCache(maxSize int64) *ModelCache {
	return &ModelCache{
		cache:   NewLRUCache(100), // Max 100 models in cache
		maxSize: maxSize,
	}
}

// Get gets a model from cache
func (c *ModelCache) Get(modelName string) (*Model, bool) {
	val, ok := c.cache.Get(modelName)
	if !ok {
		return nil, false
	}
	return val.(*Model), true
}

// Put puts a model into cache
func (c *ModelCache) Put(model *Model) {
	c.cache.Put(model.Name, model, model.Size)
}

// Remove removes a model from cache
func (c *ModelCache) Remove(modelName string) {
	c.cache.Remove(modelName)
}

// Evict evicts the least recently used model
func (c *ModelCache) Evict() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	back := c.cache.order.Back()
	if back == nil {
		return ""
	}
	return back.key
}

// Size returns the number of cached models
func (c *ModelCache) Size() int {
	return c.cache.Size()
}

// Clear clears the cache
func (c *ModelCache) Clear() {
	c.cache = NewLRUCache(100)
}

// ==================== Metrics Collector Implementation ====================

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requests:  make([]LatencyRecord, 0, 10000),
		startTime: time.Now(),
	}
}

// Record records a latency
func (m *MetricsCollector) Record(latency time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalRequests++
	if success {
		m.successCount++
	} else {
		m.failCount++
	}

	m.requests = append(m.requests, LatencyRecord{
		LatencyMS: latency.Milliseconds(),
		Timestamp: time.Now(),
	})

	now := time.Now()
	m.lastRequest = &now
}

// GetMetrics returns current metrics
func (m *MetricsCollector) GetMetrics() *PerformanceMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := &PerformanceMetrics{
		TotalRequests:      m.totalRequests,
		SuccessfulRequests: m.successCount,
		FailedRequests:     m.failCount,
		StartTime:          m.startTime,
		LastRequestTime:    m.lastRequest,
		Uptime:             time.Since(m.startTime),
	}

	if len(m.requests) > 0 {
		// Calculate latency statistics
		latencies := make([]int64, len(m.requests))
		var total int64
		for i, r := range m.requests {
			latencies[i] = r.LatencyMS
			total += r.LatencyMS
		}

		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		metrics.AvgLatencyMS = float64(total) / float64(len(latencies))
		metrics.P50LatencyMS = percentile(latencies, 50)
		metrics.P95LatencyMS = percentile(latencies, 95)
		metrics.P99LatencyMS = percentile(latencies, 99)

		// Calculate throughput (requests per second)
		if len(m.requests) > 1 {
			duration := m.requests[len(m.requests)-1].Timestamp.Sub(m.requests[0].Timestamp)
			if duration > 0 {
				metrics.ThroughputRPS = float64(len(m.requests)) / duration.Seconds()
			}
		}
	}

	return metrics
}

// percentile calculates percentile
func percentile(sorted []int64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index])
}

// ==================== Utility Functions ====================

// RegisterModel registers a new model in the engine
func (e *engine) RegisterModel(model *Model) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.models[model.Name]; exists {
		return fmt.Errorf("model %s already registered", model.Name)
	}

	if model.Status == "" {
		model.Status = ModelStatusUnloaded
	}

	e.models[model.Name] = model
	return nil
}

// UnregisterModel unregisters a model from the engine
func (e *engine) UnregisterModel(modelName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, exists := e.models[modelName]
	if !exists {
		return fmt.Errorf("model %s not found", modelName)
	}

	if model.Status == ModelStatusReady {
		return fmt.Errorf("cannot unregister loaded model %s, unload it first", modelName)
	}

	delete(e.models, modelName)
	e.modelCache.Remove(modelName)

	return nil
}
