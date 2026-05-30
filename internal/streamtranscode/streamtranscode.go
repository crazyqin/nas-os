package streamtranscode

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// 分辨率常量
const (
	Resolution4K   = "3840x2160"
	Resolution1080p = "1920x1080"
	Resolution720p  = "1280x720"
	Resolution480p  = "854x480"
)

// 视频编码常量
const (
	CodecH264 = "H264"
	CodecH265 = "H265"
	CodecVP9  = "VP9"
	CodecAV1  = "AV1"
)

// 音频编码常量
const (
	AudioCodecAAC  = "AAC"
	AudioCodecMP3  = "MP3"
	AudioCodecOpus = "Opus"
	AudioCodecFLAC = "FLAC"
)

// 任务状态
const (
	TaskStatusPending    = "pending"
	TaskStatusProcessing = "processing"
	TaskStatusCompleted  = "completed"
	TaskStatusFailed     = "failed"
	TaskStatusCancelled  = "cancelled"
)

// 优先级
const (
	PriorityLow    = 1
	PriorityNormal = 5
	PriorityHigh   = 10
)

var (
	ErrTaskNotFound     = errors.New("转码任务不存在")
	ErrPresetNotFound   = errors.New("转码预设不存在")
	ErrInvalidConfig    = errors.New("无效的转码配置")
	ErrTaskNotCancellable = errors.New("任务无法取消")
	ErrQueueFull        = errors.New("转码队列已满")
	ErrDuplicatePreset  = errors.New("预设名称已存在")
)

// TranscodeConfig 转码配置
type TranscodeConfig struct {
	Resolution   string `json:"resolution"`    // 分辨率
	VideoCodec   string `json:"video_codec"`   // 视频编码
	AudioCodec   string `json:"audio_codec"`   // 音频编码
	VideoBitrate int    `json:"video_bitrate"`  // 视频码率 (kbps)
	AudioBitrate int    `json:"audio_bitrate"`  // 音频码率 (kbps)
	FPS          int    `json:"fps"`            // 帧率
}

// TranscodeTask 转码任务
type TranscodeTask struct {
	ID         string          `json:"id"`
	InputFile  string          `json:"input_file"`
	OutputFile string          `json:"output_file"`
	Config     TranscodeConfig `json:"config"`
	Preset     string          `json:"preset,omitempty"` // 使用的预设名称
	Status     string          `json:"status"`
	Priority   int             `json:"priority"`
	Progress   float64         `json:"progress"` // 0-100
	CreatedAt  time.Time       `json:"created_at"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// TranscodePreset 转码预设
type TranscodePreset struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Config      TranscodeConfig `json:"config"`
	BuiltIn     bool            `json:"built_in"` // 是否内置预设
}

// TranscodeStats 转码统计
type TranscodeStats struct {
	TotalTasks     int            `json:"total_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	FailedTasks    int            `json:"failed_tasks"`
	CancelledTasks int            `json:"cancelled_tasks"`
	CompletionRate float64        `json:"completion_rate"`
	AverageDuration float64       `json:"average_duration_ms"` // 平均耗时(毫秒)
	FormatDist     map[string]int `json:"format_distribution"` // 格式分布
}

// ThumbnailInfo 缩略图信息
type ThumbnailInfo struct {
	TaskID    string    `json:"task_id"`
	FilePath  string    `json:"file_path"`
	Timestamp float64   `json:"timestamp"` // 视频中的时间点(秒)
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

// StreamTranscodeEngine 流媒体转码引擎
type StreamTranscodeEngine struct {
	mu              sync.RWMutex
	tasks           map[string]*TranscodeTask
	presets         map[string]*TranscodePreset
	thumbnails      map[string][]*ThumbnailInfo // key: task ID
	maxConcurrency  int
	activeWorkers   int
	maxQueueSize    int
	taskOrder       []string // 按优先级排序的任务ID队列
}

// NewEngine 创建转码引擎
func NewEngine(maxConcurrency, maxQueueSize int) *StreamTranscodeEngine {
	if maxConcurrency <= 0 {
		maxConcurrency = 2
	}
	if maxQueueSize <= 0 {
		maxQueueSize = 100
	}

	engine := &StreamTranscodeEngine{
		tasks:          make(map[string]*TranscodeTask),
		presets:        make(map[string]*TranscodePreset),
		thumbnails:     make(map[string][]*ThumbnailInfo),
		maxConcurrency: maxConcurrency,
		maxQueueSize:   maxQueueSize,
	}

	// 注册内置预设
	engine.registerBuiltInPresets()

	return engine
}

// registerBuiltInPresets 注册内置预设
func (e *StreamTranscodeEngine) registerBuiltInPresets() {
	builtInPresets := []*TranscodePreset{
		{
			Name:        "4K-高品质",
			Description: "4K分辨率，H265编码，高品质输出",
			Config: TranscodeConfig{
				Resolution:   Resolution4K,
				VideoCodec:   CodecH265,
				AudioCodec:   AudioCodecAAC,
				VideoBitrate: 20000,
				AudioBitrate: 320,
				FPS:          30,
			},
			BuiltIn: true,
		},
		{
			Name:        "1080p-标准",
			Description: "1080p分辨率，H264编码，通用兼容",
			Config: TranscodeConfig{
				Resolution:   Resolution1080p,
				VideoCodec:   CodecH264,
				AudioCodec:   AudioCodecAAC,
				VideoBitrate: 8000,
				AudioBitrate: 192,
				FPS:          30,
			},
			BuiltIn: true,
		},
		{
			Name:        "720p-省空间",
			Description: "720p分辨率，H264编码，节省存储空间",
			Config: TranscodeConfig{
				Resolution:   Resolution720p,
				VideoCodec:   CodecH264,
				AudioCodec:   AudioCodecAAC,
				VideoBitrate: 4000,
				AudioBitrate: 128,
				FPS:          30,
			},
			BuiltIn: true,
		},
		{
			Name:        "480p-移动端",
			Description: "480p分辨率，适合移动设备",
			Config: TranscodeConfig{
				Resolution:   Resolution480p,
				VideoCodec:   CodecH264,
				AudioCodec:   AudioCodecAAC,
				VideoBitrate: 2000,
				AudioBitrate: 128,
				FPS:          24,
			},
			BuiltIn: true,
		},
		{
			Name:        "AV1-高效",
			Description: "AV1编码，最新高效压缩",
			Config: TranscodeConfig{
				Resolution:   Resolution1080p,
				VideoCodec:   CodecAV1,
				AudioCodec:   AudioCodecOpus,
				VideoBitrate: 5000,
				AudioBitrate: 128,
				FPS:          30,
			},
			BuiltIn: true,
		},
	}

	for _, p := range builtInPresets {
		e.presets[p.Name] = p
	}
}

// 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// CreateTask 创建转码任务
func (e *StreamTranscodeEngine) CreateTask(inputFile, outputFile string, config TranscodeConfig, priority int) (*TranscodeTask, error) {
	if err := e.validateConfig(config); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查队列是否已满
	pendingCount := 0
	for _, t := range e.tasks {
		if t.Status == TaskStatusPending || t.Status == TaskStatusProcessing {
			pendingCount++
		}
	}
	if pendingCount >= e.maxQueueSize {
		return nil, ErrQueueFull
	}

	if priority < PriorityLow {
		priority = PriorityLow
	}
	if priority > PriorityHigh {
		priority = PriorityHigh
	}

	task := &TranscodeTask{
		ID:         generateTaskID(),
		InputFile:  inputFile,
		OutputFile: outputFile,
		Config:     config,
		Status:     TaskStatusPending,
		Priority:   priority,
		Progress:   0,
		CreatedAt:  time.Now(),
	}

	e.tasks[task.ID] = task
	e.insertTaskOrder(task.ID, task.Priority)

	return task, nil
}

// CreateTaskFromPreset 使用预设创建转码任务
func (e *StreamTranscodeEngine) CreateTaskFromPreset(inputFile, outputFile, presetName string, priority int) (*TranscodeTask, error) {
	e.mu.RLock()
	preset, exists := e.presets[presetName]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrPresetNotFound
	}

	task, err := e.CreateTask(inputFile, outputFile, preset.Config, priority)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	task.Preset = presetName
	e.mu.Unlock()

	return task, nil
}

// insertTaskOrder 按优先级插入任务到队列
func (e *StreamTranscodeEngine) insertTaskOrder(taskID string, priority int) {
	// 找到插入位置（优先级高的在前）
	insertIdx := len(e.taskOrder)
	for i, id := range e.taskOrder {
		if t, ok := e.tasks[id]; ok {
			if priority > t.Priority {
				insertIdx = i
				break
			}
		}
	}

	// 插入
	e.taskOrder = append(e.taskOrder, "")
	copy(e.taskOrder[insertIdx+1:], e.taskOrder[insertIdx:])
	e.taskOrder[insertIdx] = taskID
}

// GetTask 获取转码任务
func (e *StreamTranscodeEngine) GetTask(taskID string) (*TranscodeTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	// 返回副本
	taskCopy := *task
	return &taskCopy, nil
}

// CancelTask 取消转码任务
func (e *StreamTranscodeEngine) CancelTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusPending && task.Status != TaskStatusProcessing {
		return ErrTaskNotCancellable
	}

	task.Status = TaskStatusCancelled
	task.Error = "用户取消"

	// 从队列中移除
	e.removeFromOrder(taskID)

	return nil
}

// removeFromOrder 从队列中移除任务
func (e *StreamTranscodeEngine) removeFromOrder(taskID string) {
	for i, id := range e.taskOrder {
		if id == taskID {
			e.taskOrder = append(e.taskOrder[:i], e.taskOrder[i+1:]...)
			break
		}
	}
}

// ListTasks 列出所有转码任务
func (e *StreamTranscodeEngine) ListTasks(statusFilter string) []*TranscodeTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*TranscodeTask
	for _, task := range e.tasks {
		if statusFilter == "" || task.Status == statusFilter {
			taskCopy := *task
			result = append(result, &taskCopy)
		}
	}

	// 按创建时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// GetNextTask 获取下一个待处理任务
func (e *StreamTranscodeEngine) GetNextTask() *TranscodeTask {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.activeWorkers >= e.maxConcurrency {
		return nil
	}

	for _, taskID := range e.taskOrder {
		if task, ok := e.tasks[taskID]; ok && task.Status == TaskStatusPending {
			task.Status = TaskStatusProcessing
			now := time.Now()
			task.StartedAt = &now
			e.activeWorkers++
			e.removeFromOrder(taskID)
			taskCopy := *task
			return &taskCopy
		}
	}

	return nil
}

// UpdateTaskProgress 更新任务进度
func (e *StreamTranscodeEngine) UpdateTaskProgress(taskID string, progress float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusProcessing {
		return errors.New("只能更新处理中的任务进度")
	}

	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	task.Progress = progress
	return nil
}

// CompleteTask 完成转码任务
func (e *StreamTranscodeEngine) CompleteTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusProcessing {
		return errors.New("只能完成处理中的任务")
	}

	task.Status = TaskStatusCompleted
	task.Progress = 100
	now := time.Now()
	task.CompletedAt = &now
	e.activeWorkers--

	return nil
}

// FailTask 标记任务失败
func (e *StreamTranscodeEngine) FailTask(taskID string, errMsg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, exists := e.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != TaskStatusProcessing {
		return errors.New("只能标记处理中的任务为失败")
	}

	task.Status = TaskStatusFailed
	task.Error = errMsg
	now := time.Now()
	task.CompletedAt = &now
	e.activeWorkers--

	return nil
}

// AddPreset 添加自定义预设
func (e *StreamTranscodeEngine) AddPreset(preset *TranscodePreset) error {
	if err := e.validateConfig(preset.Config); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.presets[preset.Name]; exists {
		return ErrDuplicatePreset
	}

	preset.BuiltIn = false
	e.presets[preset.Name] = preset
	return nil
}

// GetPreset 获取预设
func (e *StreamTranscodeEngine) GetPreset(name string) (*TranscodePreset, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	preset, exists := e.presets[name]
	if !exists {
		return nil, ErrPresetNotFound
	}

	presetCopy := *preset
	return &presetCopy, nil
}

// ListPresets 列出所有预设
func (e *StreamTranscodeEngine) ListPresets() []*TranscodePreset {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*TranscodePreset
	for _, preset := range e.presets {
		presetCopy := *preset
		result = append(result, &presetCopy)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// DeletePreset 删除自定义预设
func (e *StreamTranscodeEngine) DeletePreset(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	preset, exists := e.presets[name]
	if !exists {
		return ErrPresetNotFound
	}

	if preset.BuiltIn {
		return errors.New("不能删除内置预设")
	}

	delete(e.presets, name)
	return nil
}

// GenerateThumbnail 生成缩略图
func (e *StreamTranscodeEngine) GenerateThumbnail(taskID, filePath string, timestamp float64, width, height int) (*ThumbnailInfo, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查任务是否存在
	task, exists := e.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	// 只能为处理中或已完成的任务生成缩略图
	if task.Status != TaskStatusProcessing && task.Status != TaskStatusCompleted {
		return nil, errors.New("只能为处理中或已完成的任务生成缩略图")
	}

	if width <= 0 {
		width = 320
	}
	if height <= 0 {
		height = 240
	}

	thumbnail := &ThumbnailInfo{
		TaskID:    taskID,
		FilePath:  filePath,
		Timestamp: timestamp,
		Width:     width,
		Height:    height,
		CreatedAt: time.Now(),
	}

	e.thumbnails[taskID] = append(e.thumbnails[taskID], thumbnail)

	return thumbnail, nil
}

// GetThumbnails 获取任务的所有缩略图
func (e *StreamTranscodeEngine) GetThumbnails(taskID string) ([]*ThumbnailInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, exists := e.tasks[taskID]; !exists {
		return nil, ErrTaskNotFound
	}

	thumbnails := e.thumbnails[taskID]
	if thumbnails == nil {
		return []*ThumbnailInfo{}, nil
	}

	// 返回副本
	result := make([]*ThumbnailInfo, len(thumbnails))
	for i, t := range thumbnails {
		tc := *t
		result[i] = &tc
	}

	return result, nil
}

// GetStats 获取转码统计
func (e *StreamTranscodeEngine) GetStats() *TranscodeStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &TranscodeStats{
		FormatDist: make(map[string]int),
	}

	var totalDuration time.Duration
	completedCount := 0

	for _, task := range e.tasks {
		stats.TotalTasks++

		switch task.Status {
		case TaskStatusCompleted:
			stats.CompletedTasks++
			if task.StartedAt != nil && task.CompletedAt != nil {
				totalDuration += task.CompletedAt.Sub(*task.StartedAt)
				completedCount++
			}
			// 统计编码格式分布
			stats.FormatDist[task.Config.VideoCodec]++
		case TaskStatusFailed:
			stats.FailedTasks++
		case TaskStatusCancelled:
			stats.CancelledTasks++
		}
	}

	// 计算完成率
	if stats.TotalTasks > 0 {
		stats.CompletionRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	// 计算平均耗时
	if completedCount > 0 {
		stats.AverageDuration = float64(totalDuration.Milliseconds()) / float64(completedCount)
	}

	return stats
}

// GetStatsJSON 获取统计信息的JSON格式
func (e *StreamTranscodeEngine) GetStatsJSON() (string, error) {
	stats := e.GetStats()
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetQueueStatus 获取队列状态
func (e *StreamTranscodeEngine) GetQueueStatus() map[string]int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := map[string]int{
		"pending":     0,
		"processing":  0,
		"completed":   0,
		"failed":      0,
		"cancelled":   0,
		"total":       0,
		"queue_depth": len(e.taskOrder),
		"max_concurrency": e.maxConcurrency,
		"active_workers":  e.activeWorkers,
	}

	for _, task := range e.tasks {
		status[task.Status]++
		status["total"]++
	}

	return status
}

// validateConfig 验证转码配置
func (e *StreamTranscodeEngine) validateConfig(config TranscodeConfig) error {
	// 验证分辨率
	validResolutions := map[string]bool{
		Resolution4K: true, Resolution1080p: true,
		Resolution720p: true, Resolution480p: true,
	}
	if !validResolutions[config.Resolution] {
		return fmt.Errorf("%w: 不支持的分辨率 %s", ErrInvalidConfig, config.Resolution)
	}

	// 验证视频编码
	validVideoCodecs := map[string]bool{
		CodecH264: true, CodecH265: true, CodecVP9: true, CodecAV1: true,
	}
	if !validVideoCodecs[config.VideoCodec] {
		return fmt.Errorf("%w: 不支持的视频编码 %s", ErrInvalidConfig, config.VideoCodec)
	}

	// 验证音频编码
	validAudioCodecs := map[string]bool{
		AudioCodecAAC: true, AudioCodecMP3: true, AudioCodecOpus: true, AudioCodecFLAC: true,
	}
	if !validAudioCodecs[config.AudioCodec] {
		return fmt.Errorf("%w: 不支持的音频编码 %s", ErrInvalidConfig, config.AudioCodec)
	}

	// 验证码率
	if config.VideoBitrate <= 0 {
		return fmt.Errorf("%w: 视频码率必须大于0", ErrInvalidConfig)
	}
	if config.AudioBitrate <= 0 {
		return fmt.Errorf("%w: 音频码率必须大于0", ErrInvalidConfig)
	}

	// 验证帧率
	if config.FPS <= 0 || config.FPS > 120 {
		return fmt.Errorf("%w: 帧率必须在1-120之间", ErrInvalidConfig)
	}

	return nil
}

// EstimateFileSize 估算输出文件大小（MB）
func (e *StreamTranscodeEngine) EstimateFileSize(config TranscodeConfig, durationSeconds float64) float64 {
	// 视频码率 (kbps) * 时长(s) / 8 / 1024 = MB
	videoSize := float64(config.VideoBitrate) * durationSeconds / 8.0 / 1024.0
	audioSize := float64(config.AudioBitrate) * durationSeconds / 8.0 / 1024.0
	return math.Round((videoSize+audioSize)*100) / 100
}

// GetResolutionLabel 获取分辨率标签
func GetResolutionLabel(resolution string) string {
	labels := map[string]string{
		Resolution4K:    "4K (2160p)",
		Resolution1080p: "Full HD (1080p)",
		Resolution720p:  "HD (720p)",
		Resolution480p:  "SD (480p)",
	}
	if label, ok := labels[resolution]; ok {
		return label
	}
	return resolution
}

// GetCodecInfo 获取编码信息
func GetCodecInfo(codec string) string {
	info := map[string]string{
		CodecH264: "H.264/AVC - 通用兼容",
		CodecH265: "H.265/HEVC - 高压缩率",
		CodecVP9:  "VP9 - 开源高效",
		CodecAV1:  "AV1 - 最新一代编码",
	}
	if desc, ok := info[codec]; ok {
		return desc
	}
	return codec
}

// FormatDuration 格式化时长
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f秒", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1f分钟", d.Minutes())
	}
	return fmt.Sprintf("%.1f小时", d.Hours())
}

// GetTaskDuration 获取任务耗时
func GetTaskDuration(task *TranscodeTask) time.Duration {
	if task.StartedAt == nil {
		return 0
	}
	end := time.Now()
	if task.CompletedAt != nil {
		end = *task.CompletedAt
	}
	return end.Sub(*task.StartedAt)
}

// String 方法实现

func (t *TranscodeTask) String() string {
	return fmt.Sprintf("Task[%s] %s -> %s (%s, %.1f%%)",
		t.ID, t.InputFile, t.OutputFile, t.Status, t.Progress)
}

func (p *TranscodePreset) String() string {
	builtin := ""
	if p.BuiltIn {
		builtin = " [内置]"
	}
	return fmt.Sprintf("Preset[%s]%s - %s", p.Name, builtin, p.Description)
}

func (c *TranscodeConfig) String() string {
	return fmt.Sprintf("%s/%s/%s %dkbps",
		GetResolutionLabel(c.Resolution), c.VideoCodec, c.AudioCodec, c.VideoBitrate)
}

// ToJSON 序列化为JSON
func (t *TranscodeTask) ToJSON() (string, error) {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSON 序列化为JSON
func (p *TranscodePreset) ToJSON() (string, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseConfigFromJSON 从JSON解析配置
func ParseConfigFromJSON(jsonStr string) (*TranscodeConfig, error) {
	var config TranscodeConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &config, nil
}

// GetSupportedResolutions 获取支持的分辨率列表
func GetSupportedResolutions() []string {
	return []string{Resolution4K, Resolution1080p, Resolution720p, Resolution480p}
}

// GetSupportedVideoCodecs 获取支持的视频编码列表
func GetSupportedVideoCodecs() []string {
	return []string{CodecH264, CodecH265, CodecVP9, CodecAV1}
}

// GetSupportedAudioCodecs 获取支持的音频编码列表
func GetSupportedAudioCodecs() []string {
	return []string{AudioCodecAAC, AudioCodecMP3, AudioCodecOpus, AudioCodecFLAC}
}

// MatchPreset 根据配置匹配最佳预设
func (e *StreamTranscodeEngine) MatchPreset(config TranscodeConfig) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for name, preset := range e.presets {
		if preset.Config.Resolution == config.Resolution &&
			preset.Config.VideoCodec == config.VideoCodec &&
			preset.Config.AudioCodec == config.AudioCodec {
			return name
		}
	}
	return ""
}

// CleanCompletedTasks 清理已完成的任务
func (e *StreamTranscodeEngine) CleanCompletedTasks(before time.Time) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	removed := 0
	for id, task := range e.tasks {
		if (task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled) &&
			task.CompletedAt != nil && task.CompletedAt.Before(before) {
			delete(e.tasks, id)
			delete(e.thumbnails, id)
			removed++
		}
	}

	return removed
}

// ValidateConfig 公开的配置验证方法
func ValidateConfig(config TranscodeConfig) error {
	engine := &StreamTranscodeEngine{}
	return engine.validateConfig(config)
}

// CodecSupported 检查编码是否支持
func CodecSupported(codec string) bool {
	codec = strings.ToUpper(codec)
	supported := map[string]bool{
		CodecH264: true, CodecH265: true, CodecVP9: true, CodecAV1: true,
		AudioCodecAAC: true, AudioCodecMP3: true, AudioCodecOpus: true, AudioCodecFLAC: true,
	}
	return supported[codec]
}
