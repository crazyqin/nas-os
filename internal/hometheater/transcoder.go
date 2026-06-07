// Package hometheater 提供媒体转码功能
package hometheater

import (
	"fmt"
	"sync"
	"time"
)

// Transcoder 转码器.
type Transcoder struct {
	mu             sync.RWMutex
	engine         *Engine
	jobs           map[string]*TranscodeJob
	maxConcurrent  int
	activeJobs     int
	hwAccel        HardwareAccel
	defaultProfile *TranscodeProfile
	progressChan   chan *TranscodeProgress
}

// TranscodeProgress 转码进度.
type TranscodeProgress struct {
	JobID    string          `json:"job_id"`
	Status   TranscodeStatus `json:"status"`
	Progress float64         `json:"progress"`
	Speed    float64         `json:"speed"`
	FPS      float64         `json:"fps"`
	Bitrate  int             `json:"bitrate"`
	ETA      time.Duration   `json:"eta"`
	Error    string          `json:"error,omitempty"`
}

// TranscodeRequest 转码请求.
type TranscodeRequest struct {
	MediaID    string            `json:"media_id"`
	ProfileID  string            `json:"profile_id"`
	InputPath  string            `json:"input_path"`
	OutputPath string            `json:"output_path"`
	Priority   int               `json:"priority"`
	Options    *TranscodeOptions `json:"options,omitempty"`
}

// TranscodeOptions 转码选项.
type TranscodeOptions struct {
	StartTime   float64 `json:"start_time"`   // 开始时间（秒）
	Duration    float64 `json:"duration"`     // 转码时长（秒）
	ScaleWidth  int     `json:"scale_width"`  // 缩放宽度
	ScaleHeight int     `json:"scale_height"` // 缩放高度
	Deinterlace bool    `json:"deinterlace"`  // 去隔行
	Denoise     bool    `json:"denoise"`      // 降噪
	Sharpen     bool    `json:"sharpen"`      // 锐化
}

// NewTranscoder 创建转码器.
func NewTranscoder(engine *Engine) *Transcoder {
	return &Transcoder{
		engine:        engine,
		jobs:          make(map[string]*TranscodeJob),
		maxConcurrent: 2,
		hwAccel:       AccelNone,
		progressChan:  make(chan *TranscodeProgress, 100),
	}
}

// SetMaxConcurrent 设置最大并发转码数.
func (t *Transcoder) SetMaxConcurrent(max int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if max > 0 {
		t.maxConcurrent = max
	}
}

// SetHardwareAccel 设置硬件加速.
func (t *Transcoder) SetHardwareAccel(accel HardwareAccel) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hwAccel = accel
}

// SetDefaultProfile 设置默认转码配置.
func (t *Transcoder) SetDefaultProfile(profile *TranscodeProfile) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.defaultProfile = profile
}

// GetProgressChannel 获取进度通道.
func (t *Transcoder) GetProgressChannel() <-chan *TranscodeProgress {
	return t.progressChan
}

// SubmitJob 提交转码任务.
func (t *Transcoder) SubmitJob(req *TranscodeRequest) (*TranscodeJob, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.activeJobs >= t.maxConcurrent {
		return nil, fmt.Errorf("转码队列已满，当前活跃任务: %d/%d", t.activeJobs, t.maxConcurrent)
	}

	// 获取转码配置
	var profile *TranscodeProfile
	if req.ProfileID != "" {
		p, err := t.engine.GetTranscodeProfile(req.ProfileID)
		if err != nil {
			return nil, fmt.Errorf("无效的转码配置: %s", req.ProfileID)
		}
		profile = p
	} else if t.defaultProfile != nil {
		profile = t.defaultProfile
	} else {
		return nil, ErrInvalidProfile
	}

	job := &TranscodeJob{
		ID:         fmt.Sprintf("tc_%d", time.Now().UnixNano()),
		MediaID:    req.MediaID,
		ProfileID:  profile.ID,
		Status:     TranscodePending,
		InputPath:  req.InputPath,
		OutputPath: req.OutputPath,
		StartTime:  time.Now(),
	}

	t.jobs[job.ID] = job
	t.activeJobs++

	// 启动异步转码
	go t.processJob(job, profile, req.Options)

	return job, nil
}

// GetJob 获取转码任务.
func (t *Transcoder) GetJob(jobID string) (*TranscodeJob, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	job, exists := t.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("转码任务不存在: %s", jobID)
	}
	return job, nil
}

// ListJobs 列出所有转码任务.
func (t *Transcoder) ListJobs(status TranscodeStatus) []*TranscodeJob {
	t.mu.RLock()
	defer t.mu.RUnlock()

	jobs := make([]*TranscodeJob, 0)
	for _, job := range t.jobs {
		if status == "" || job.Status == status {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// CancelJob 取消转码任务.
func (t *Transcoder) CancelJob(jobID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	job, exists := t.jobs[jobID]
	if !exists {
		return fmt.Errorf("转码任务不存在: %s", jobID)
	}

	if job.Status == TranscodeCompleted || job.Status == TranscodeFailed {
		return fmt.Errorf("转码任务已完成或失败: %s", jobID)
	}

	job.Status = TranscodeCancelled
	now := time.Now()
	job.EndTime = &now
	t.activeJobs--

	return nil
}

// GetActiveCount 获取活跃任务数.
func (t *Transcoder) GetActiveCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.activeJobs
}

// processJob 处理转码任务.
func (t *Transcoder) processJob(job *TranscodeJob, profile *TranscodeProfile, options *TranscodeOptions) {
	t.mu.Lock()
	job.Status = TranscodeRunning
	t.mu.Unlock()

	// 模拟转码过程
	totalDuration := 7200.0 // 假设2小时电影
	if options != nil && options.Duration > 0 {
		totalDuration = options.Duration
	}

	for progress := 0.0; progress <= 100.0; progress += 5.0 {
		// 检查是否取消
		t.mu.RLock()
		if job.Status == TranscodeCancelled {
			t.mu.RUnlock()
			return
		}
		t.mu.RUnlock()

		// 更新进度
		t.mu.Lock()
		job.Progress = progress
		job.Stats = &TranscodeStats{
			Duration:      totalDuration * progress / 100,
			TotalDuration: totalDuration,
			Speed:         2.5,
			FPS:           30.0,
			Bitrate:       profile.VideoBitrate,
			Size:          int64(float64(totalDuration) * float64(profile.VideoBitrate) * 125 * progress / 100),
			FrameCount:    int64(totalDuration * 30 * progress / 100),
		}
		t.mu.Unlock()

		// 发送进度通知
		t.progressChan <- &TranscodeProgress{
			JobID:    job.ID,
			Status:   TranscodeRunning,
			Progress: progress,
			Speed:    2.5,
			FPS:      30.0,
			Bitrate:  profile.VideoBitrate,
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 完成
	t.mu.Lock()
	job.Status = TranscodeCompleted
	job.Progress = 100.0
	now := time.Now()
	job.EndTime = &now
	t.activeJobs--
	t.mu.Unlock()

	t.progressChan <- &TranscodeProgress{
		JobID:    job.ID,
		Status:   TranscodeCompleted,
		Progress: 100.0,
	}
}

// GetOptimalProfile 获取最优转码配置.
func (t *Transcoder) GetOptimalProfile(videoInfo *VideoInfo, maxWidth, maxHeight int) *TranscodeProfile {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 如果不需要转码
	if videoInfo.Width <= maxWidth && videoInfo.Height <= maxHeight {
		if videoInfo.Codec == CodecH264 || videoInfo.Codec == CodecH265 {
			return nil // 直接播放
		}
	}

	// 选择合适的分辨率
	targetWidth, targetHeight := calculateScale(videoInfo.Width, videoInfo.Height, maxWidth, maxHeight)

	// 根据硬件加速选择编码器
	videoCodec := CodecH264
	if t.hwAccel != AccelNone {
		videoCodec = CodecH265
	}

	return &TranscodeProfile{
		ID:            "auto",
		Name:          "自动",
		VideoCodec:    videoCodec,
		AudioCodec:    AudioCodecAAC,
		Width:         targetWidth,
		Height:        targetHeight,
		VideoBitrate:  calculateBitrate(targetWidth, targetHeight),
		AudioBitrate:  128,
		FrameRate:     videoInfo.FrameRate,
		Preset:        "fast",
		HardwareAccel: t.hwAccel,
	}
}

// calculateScale 计算缩放尺寸.
func calculateScale(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= maxWidth && height <= maxHeight {
		return width, height
	}

	ratioW := float64(maxWidth) / float64(width)
	ratioH := float64(maxHeight) / float64(height)
	ratio := ratioW
	if ratioH < ratioW {
		ratio = ratioH
	}

	newWidth := int(float64(width) * ratio)
	newHeight := int(float64(height) * ratio)

	// 确保宽高为偶数
	newWidth = newWidth & ^1
	newHeight = newHeight & ^1

	return newWidth, newHeight
}

// calculateBitrate 根据分辨率计算推荐码率.
func calculateBitrate(width, height int) int {
	pixels := width * height

	switch {
	case pixels >= 3840*2160: // 4K
		return 20000
	case pixels >= 1920*1080: // 1080p
		return 5000
	case pixels >= 1280*720: // 720p
		return 2500
	default:
		return 1000
	}
}

// GetSupportedCodecs 获取支持的编码格式.
func GetSupportedCodecs() map[string][]string {
	return map[string][]string{
		"video": {
			string(CodecH264),
			string(CodecH265),
			string(CodecVP9),
			string(CodecAV1),
		},
		"audio": {
			string(AudioCodecAAC),
			string(AudioCodecAC3),
			string(AudioCodecDTS),
			string(AudioCodecEAC3),
			string(AudioCodecOpus),
		},
	}
}

// GetHWAccelCapabilities 获取硬件加速能力.
func GetHWAccelCapabilities() map[HardwareAccel]bool {
	return map[HardwareAccel]bool{
		AccelVAAPI: true,
		AccelNVENC: true,
		AccelQSV:   true,
		AccelRKMPP: true,
		AccelV4L2:  true,
	}
}
