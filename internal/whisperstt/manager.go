// Package whisperstt 提供 Whisper 本地语音转文字服务
// 模型管理、转录队列、结果存储
package whisperstt

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager Whisper STT 管理器
type Manager struct {
	mu             sync.RWMutex
	logger         *zap.Logger
	models         map[string]*WhisperModel
	jobs           map[string]*TranscriptionJob
	results        map[string]*TranscriptionResult
	currentModelID string
	preprocess     AudioPreprocessConfig
	startTime      time.Time
	running        bool

	// 统计
	totalTranscriptions int
	totalAudioDuration  time.Duration
	totalProcessTime    time.Duration
	languageStats       map[string]int
	dailyStats          map[string]*DailyStat
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:        logger,
		models:        make(map[string]*WhisperModel),
		jobs:          make(map[string]*TranscriptionJob),
		results:       make(map[string]*TranscriptionResult),
		preprocess:    DefaultPreprocessConfig(),
		startTime:     time.Now(),
		languageStats: make(map[string]int),
		dailyStats:    make(map[string]*DailyStat),
	}

	// 初始化默认模型
	m.initDefaultModels()

	return m
}

// initDefaultModels 初始化默认模型
func (m *Manager) initDefaultModels() {
	models := []struct {
		id        string
		name      string
		size      int64
		gpu       bool
		languages []string
	}{
		{"tiny", "tiny", 39 * 1024 * 1024, true, []string{"en", "zh", "ja", "ko", "fr", "de", "es", "ru"}},
		{"base", "base", 74 * 1024 * 1024, true, []string{"en", "zh", "ja", "ko", "fr", "de", "es", "ru"}},
		{"small", "small", 244 * 1024 * 1024, true, []string{"en", "zh", "ja", "ko", "fr", "de", "es", "ru", "pt", "it", "nl", "sv", "pl"}},
		{"medium", "medium", 769 * 1024 * 1024, true, []string{"en", "zh", "ja", "ko", "fr", "de", "es", "ru", "pt", "it", "nl", "sv", "pl", "ar", "hi", "th"}},
		{"large", "large", 1550 * 1024 * 1024, true, []string{"en", "zh", "ja", "ko", "fr", "de", "es", "ru", "pt", "it", "nl", "sv", "pl", "ar", "hi", "th", "vi", "tr", "he", "id"}},
	}

	for _, mDef := range models {
		m.models[mDef.id] = &WhisperModel{
			ID:           mDef.id,
			Name:         mDef.name,
			Size:         mDef.size,
			Languages:    mDef.languages,
			IsLoaded:     false,
			GPUSupported: mDef.gpu,
		}
	}
}

// Start 启动管理器
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = true
	m.startTime = time.Now()
	m.logger.Info("[Whisper STT] 管理器已启动")
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	m.logger.Info("[Whisper STT] 管理器已停止")
}

// ========== 模型管理 ==========

// ListModels 列出所有模型
func (m *Manager) ListModels() []WhisperModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	models := make([]WhisperModel, 0, len(m.models))
	for _, model := range m.models {
		models = append(models, *model)
	}
	return models
}

// GetModel 获取模型
func (m *Manager) GetModel(id string) (*WhisperModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	model, ok := m.models[id]
	if !ok {
		return nil, fmt.Errorf("模型不存在: %s", id)
	}
	return model, nil
}

// LoadModel 加载模型
func (m *Manager) LoadModel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, ok := m.models[id]
	if !ok {
		return fmt.Errorf("模型不存在: %s", id)
	}

	if model.IsLoaded {
		return fmt.Errorf("模型已加载: %s", id)
	}

	// 卸载当前模型（如果有）
	if m.currentModelID != "" {
		if current, ok := m.models[m.currentModelID]; ok {
			current.IsLoaded = false
		}
	}

	model.IsLoaded = true
	model.LoadTime = time.Now()
	m.currentModelID = id

	m.logger.Info("[Whisper STT] 模型已加载", zap.String("model", id))
	return nil
}

// UnloadModel 卸载模型
func (m *Manager) UnloadModel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, ok := m.models[id]
	if !ok {
		return fmt.Errorf("模型不存在: %s", id)
	}

	if !model.IsLoaded {
		return fmt.Errorf("模型未加载: %s", id)
	}

	model.IsLoaded = false
	if m.currentModelID == id {
		m.currentModelID = ""
	}

	m.logger.Info("[Whisper STT] 模型已卸载", zap.String("model", id))
	return nil
}

// GetCurrentModel 获取当前模型
func (m *Manager) GetCurrentModel() *WhisperModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentModelID == "" {
		return nil
	}
	model, ok := m.models[m.currentModelID]
	if !ok {
		return nil
	}
	return model
}

// ========== 转录任务 ==========

// CreateJob 创建转录任务
func (m *Manager) CreateJob(filePath string, fileName string, options TranscriptionOptions, priority int) *TranscriptionJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobID := generateID()
	job := &TranscriptionJob{
		ID:        jobID,
		FilePath:  filePath,
		FileName:  fileName,
		Status:    JobStatusQueued,
		Progress:  0,
		Priority:  priority,
		Options:   options,
		CreatedAt: time.Now(),
	}

	m.jobs[jobID] = job
	m.logger.Info("[Whisper STT] 创建转录任务", zap.String("jobId", jobID), zap.String("file", fileName))
	return job
}

// GetJob 获取任务
func (m *Manager) GetJob(id string) (*TranscriptionJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", id)
	}
	return job, nil
}

// ListJobs 列出任务
func (m *Manager) ListJobs(statusFilter string) []TranscriptionJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]TranscriptionJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		if statusFilter == "" || string(job.Status) == statusFilter {
			jobs = append(jobs, *job)
		}
	}
	return jobs
}

// CancelJob 取消任务
func (m *Manager) CancelJob(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("任务不存在: %s", id)
	}

	if job.Status != JobStatusQueued && job.Status != JobStatusProcessing {
		return fmt.Errorf("任务无法取消，当前状态: %s", job.Status)
	}

	job.Status = JobStatusCancelled
	m.logger.Info("[Whisper STT] 任务已取消", zap.String("jobId", id))
	return nil
}

// UpdateJobStatus 更新任务状态
func (m *Manager) UpdateJobStatus(id string, status JobStatus, progress float64, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("任务不存在: %s", id)
	}

	job.Status = status
	job.Progress = progress
	job.ErrorMsg = errMsg

	now := time.Now()
	switch status {
	case JobStatusProcessing:
		job.StartedAt = &now
	case JobStatusCompleted, JobStatusFailed:
		job.CompletedAt = &now
	}

	return nil
}

// ========== 转录结果 ==========

// SaveResult 保存转录结果
func (m *Manager) SaveResult(result *TranscriptionResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.results[result.ID] = result
	m.totalTranscriptions++
	m.totalAudioDuration += time.Duration(result.Duration * float64(time.Second))
	m.languageStats[result.Language]++

	// 更新每日统计
	dateKey := result.ProcessedAt.Format("2006-01-02")
	if _, ok := m.dailyStats[dateKey]; !ok {
		m.dailyStats[dateKey] = &DailyStat{Date: dateKey}
	}
	m.dailyStats[dateKey].Transcriptions++
	m.dailyStats[dateKey].AudioDuration += time.Duration(result.Duration * float64(time.Second))

	m.logger.Info("[Whisper STT] 转录结果已保存", zap.String("resultId", result.ID))
}

// GetResult 获取结果
func (m *Manager) GetResult(id string) (*TranscriptionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.results[id]
	if !ok {
		return nil, fmt.Errorf("结果不存在: %s", id)
	}
	return result, nil
}

// ListResults 列出结果
func (m *Manager) ListResults() []TranscriptionResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]TranscriptionResult, 0, len(m.results))
	for _, result := range m.results {
		results = append(results, *result)
	}
	return results
}

// UpdateResult 更新结果
func (m *Manager) UpdateResult(id string, req EditResultRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	result, ok := m.results[id]
	if !ok {
		return fmt.Errorf("结果不存在: %s", id)
	}

	if req.Text != "" {
		result.Text = req.Text
	}
	if req.Segments != nil {
		result.Segments = req.Segments
	}

	return nil
}

// DeleteResult 删除结果
func (m *Manager) DeleteResult(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.results[id]; !ok {
		return fmt.Errorf("结果不存在: %s", id)
	}

	delete(m.results, id)
	return nil
}

// ========== 预处理配置 ==========

// GetPreprocessConfig 获取预处理配置
func (m *Manager) GetPreprocessConfig() AudioPreprocessConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.preprocess
}

// UpdatePreprocessConfig 更新预处理配置
func (m *Manager) UpdatePreprocessConfig(config AudioPreprocessConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preprocess = config
	m.logger.Info("[Whisper STT] 预处理配置已更新")
}

// ========== 队列管理 ==========

// GetQueueStats 获取队列统计
func (m *Manager) GetQueueStats() QueueStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := QueueStats{
		LastUpdated: time.Now(),
	}

	for _, job := range m.jobs {
		switch job.Status {
		case JobStatusQueued:
			stats.QueueLength++
		case JobStatusProcessing:
			stats.Processing++
		case JobStatusCompleted:
			stats.Completed++
		case JobStatusFailed:
			stats.Failed++
		}
	}

	return stats
}

// ClearQueue 清空队列
func (m *Manager) ClearQueue() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, job := range m.jobs {
		if job.Status == JobStatusQueued {
			delete(m.jobs, id)
			count++
		}
	}

	m.logger.Info("[Whisper STT] 队列已清空", zap.Int("count", count))
	return count
}

// ========== GPU 状态 ==========

// GetGPUStatus 获取 GPU 状态
func (m *Manager) GetGPUStatus() GPUStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 模拟 GPU 状态
	return GPUStatus{
		Available:   true,
		DeviceName:  "NVIDIA GeForce RTX 3080",
		CUDAVersion: "11.8",
		Memory: GPUMemory{
			Total:       10 * 1024 * 1024 * 1024,
			Used:        2 * 1024 * 1024 * 1024,
			Available:   8 * 1024 * 1024 * 1024,
			ModelUsage:  1550 * 1024 * 1024,
			LastUpdated: time.Now(),
		},
		Temperature: 65.0,
		Utilization: 45.0,
	}
}

// ========== 语言检测 ==========

// DetectLanguage 检测音频语言
func (m *Manager) DetectLanguage(filePath string) ([]LanguageDetect, error) {
	// 模拟语言检测结果
	return []LanguageDetect{
		{Code: "zh", Name: "Chinese", Confidence: 0.85},
		{Code: "en", Name: "English", Confidence: 0.10},
		{Code: "ja", Name: "Japanese", Confidence: 0.05},
	}, nil
}

// ========== 服务状态和统计 ==========

// GetStatus 获取服务状态
func (m *Manager) GetStatus() ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ServiceStatus{
		Status:       "running",
		Uptime:       time.Since(m.startTime),
		CurrentModel: m.currentModelID,
		ModelsLoaded: m.countLoadedModels(),
		TotalJobs:    len(m.jobs),
		ActiveJobs:   m.countActiveJobs(),
		GPU:          m.GetGPUStatus(),
		Queue:        m.getQueueStatsInternal(),
		StartTime:    m.startTime,
	}
}

// GetStats 获取统计信息
func (m *Manager) GetStats() ServiceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ServiceStats{
		TotalTranscriptions:  m.totalTranscriptions,
		TotalAudioDuration:   m.totalAudioDuration,
		TotalProcessTime:     m.totalProcessTime,
		LanguageDistribution: m.languageStats,
	}

	if m.totalTranscriptions > 0 {
		stats.AvgProcessTime = m.totalProcessTime / time.Duration(m.totalTranscriptions)
		if m.totalAudioDuration > 0 {
			stats.AvgRealTimeFactor = float64(m.totalProcessTime) / float64(m.totalAudioDuration)
		}
	}

	// 转换每日统计
	for _, ds := range m.dailyStats {
		stats.DailyStats = append(stats.DailyStats, *ds)
	}

	return stats
}

// ========== 辅助方法 ==========

func (m *Manager) countLoadedModels() int {
	count := 0
	for _, model := range m.models {
		if model.IsLoaded {
			count++
		}
	}
	return count
}

func (m *Manager) countActiveJobs() int {
	count := 0
	for _, job := range m.jobs {
		if job.Status == JobStatusQueued || job.Status == JobStatusProcessing {
			count++
		}
	}
	return count
}

func (m *Manager) getQueueStatsInternal() QueueStats {
	stats := QueueStats{
		LastUpdated: time.Now(),
	}
	for _, job := range m.jobs {
		switch job.Status {
		case JobStatusQueued:
			stats.QueueLength++
		case JobStatusProcessing:
			stats.Processing++
		case JobStatusCompleted:
			stats.Completed++
		case JobStatusFailed:
			stats.Failed++
		}
	}
	return stats
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
