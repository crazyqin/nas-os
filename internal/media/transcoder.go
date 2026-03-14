package media

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TranscodeConfig 转码配置
type TranscodeConfig struct {
	// 视频编码器 (libx264, libx265, libvpx, etc.)
	VideoCodec string `json:"videoCodec"`
	// 音频编码器 (aac, mp3, opus, etc.)
	AudioCodec string `json:"audioCodec"`
	// 视频比特率 (e.g., "2M", "5000k")
	VideoBitrate string `json:"videoBitrate"`
	// 音频比特率 (e.g., "128k", "256k")
	AudioBitrate string `json:"audioBitrate"`
	// 分辨率 (e.g., "1920x1080", "1280x720")
	Resolution string `json:"resolution"`
	// 帧率 (e.g., 30, 60)
	Framerate int `json:"framerate"`
	// 输出格式 (mp4, mkv, webm, etc.)
	OutputFormat string `json:"outputFormat"`
	// 质量预设 (ultrafast, fast, medium, slow)
	Preset string `json:"preset"`
	// CRF 质量 (0-51, 越小质量越好)
	CRF int `json:"crf"`
	// 硬件加速 (none, cuda, nvenc, qsv, vaapi)
	HWAccel string `json:"hwAccel"`
	// 是否复制视频流（不转码）
	CopyVideo bool `json:"copyVideo"`
	// 是否复制音频流（不转码）
	CopyAudio bool `json:"copyAudio"`
}

// TranscodeJob 转码任务
type TranscodeJob struct {
	ID           string          `json:"id"`
	InputPath    string          `json:"inputPath"`
	OutputPath   string          `json:"outputPath"`
	Config       TranscodeConfig `json:"config"`
	Status       string          `json:"status"` // pending, running, completed, failed, cancelled
	Progress     float64         `json:"progress"`
	CurrentFrame int             `json:"currentFrame"`
	TotalFrames  int             `json:"totalFrames"`
	Speed        string          `json:"speed"`
	ETA          string          `json:"eta"`
	StartTime    *time.Time      `json:"startTime,omitempty"`
	EndTime      *time.Time      `json:"endTime,omitempty"`
	Error        string          `json:"error,omitempty"`
	Log          []string        `json:"log,omitempty"`

	// 内部状态
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
	mu     sync.RWMutex
}

// Transcoder 转码器
type Transcoder struct {
	ffmpegPath  string
	ffprobePath string
	jobs        map[string]*TranscodeJob
	maxJobs     int
	mu          sync.RWMutex
}

// NewTranscoder 创建转码器
func NewTranscoder(ffmpegPath, ffprobePath string, maxJobs int) *Transcoder {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	if maxJobs <= 0 {
		maxJobs = 2
	}

	return &Transcoder{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		jobs:        make(map[string]*TranscodeJob),
		maxJobs:     maxJobs,
	}
}

// DefaultTranscodeConfig 默认转码配置
func DefaultTranscodeConfig() TranscodeConfig {
	return TranscodeConfig{
		VideoCodec:   "libx264",
		AudioCodec:   "aac",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
		Preset:       "medium",
		CRF:          23,
		OutputFormat: "mp4",
		HWAccel:      "none",
	}
}

// GetVideoInfo 获取视频信息
func (t *Transcoder) GetVideoInfo(path string) (*VideoInfo, error) {
	cmd := exec.Command(t.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe 执行失败: %w", err)
	}

	return parseVideoInfo(output)
}

// VideoInfo 视频信息
type VideoInfo struct {
	Duration   float64          `json:"duration"`
	Width      int              `json:"width"`
	Height     int              `json:"height"`
	Framerate  float64          `json:"framerate"`
	VideoCodec string           `json:"videoCodec"`
	AudioCodec string           `json:"audioCodec"`
	Bitrate    int64            `json:"bitrate"`
	Size       int64            `json:"size"`
	Streams    []StreamInfo     `json:"streams"`
	Format     FormatInfo       `json:"format"`
}

// StreamInfo 流信息
type StreamInfo struct {
	Index     int    `json:"index"`
	Type      string `json:"codec_type"`
	Codec     string `json:"codec_name"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Duration  string `json:"duration,omitempty"`
	BitRate   string `json:"bit_rate,omitempty"`
	Language  string `json:"language,omitempty"`
	FrameRate string `json:"r_frame_rate,omitempty"`
}

// FormatInfo 格式信息
type FormatInfo struct {
	Filename   string `json:"filename"`
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
	Size       string `json:"size"`
}

// parseVideoInfo 解析视频信息
func parseVideoInfo(data []byte) (*VideoInfo, error) {
	var result struct {
		Format struct {
			Filename   string `json:"filename"`
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
			Size       string `json:"size"`
		} `json:"format"`
		Streams []struct {
			Index     int    `json:"index"`
			Type      string `json:"codec_type"`
			Codec     string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
			BitRate   string `json:"bit_rate"`
			Language  string `json:"language"`
			FrameRate string `json:"r_frame_rate"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	info := &VideoInfo{}

	// 解析格式信息
	info.Format = FormatInfo{
		Filename:   result.Format.Filename,
		FormatName: result.Format.FormatName,
		Duration:   result.Format.Duration,
		BitRate:    result.Format.BitRate,
		Size:       result.Format.Size,
	}

	if dur, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		info.Duration = dur
	}
	if br, err := strconv.ParseInt(result.Format.BitRate, 10, 64); err == nil {
		info.Bitrate = br
	}
	if sz, err := strconv.ParseInt(result.Format.Size, 10, 64); err == nil {
		info.Size = sz
	}

	// 解析流信息
	for _, s := range result.Streams {
		stream := StreamInfo{
			Index:     s.Index,
			Type:      s.Type,
			Codec:     s.Codec,
			Width:     s.Width,
			Height:    s.Height,
			Duration:  s.Duration,
			BitRate:   s.BitRate,
			Language:  s.Language,
			FrameRate: s.FrameRate,
		}
		info.Streams = append(info.Streams, stream)

		if s.Type == "video" {
			info.VideoCodec = s.Codec
			info.Width = s.Width
			info.Height = s.Height
			if s.FrameRate != "" {
				// 解析帧率如 "30/1"
				parts := strings.Split(s.FrameRate, "/")
				if len(parts) == 2 {
					num, _ := strconv.ParseFloat(parts[0], 64)
					den, _ := strconv.ParseFloat(parts[1], 64)
					if den != 0 {
						info.Framerate = num / den
					}
				}
			}
		} else if s.Type == "audio" {
			info.AudioCodec = s.Codec
		}
	}

	return info, nil
}

// CreateJob 创建转码任务
func (t *Transcoder) CreateJob(inputPath, outputPath string, config TranscodeConfig) (*TranscodeJob, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 检查输入文件
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("输入文件不存在: %s", inputPath)
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	id := fmt.Sprintf("transcode_%d", time.Now().UnixNano())
	job := &TranscodeJob{
		ID:         id,
		InputPath:  inputPath,
		OutputPath: outputPath,
		Config:     config,
		Status:     "pending",
		Log:        make([]string, 0),
	}

	t.jobs[id] = job
	return job, nil
}

// StartJob 启动转码任务
func (t *Transcoder) StartJob(jobID string) error {
	t.mu.RLock()
	job, ok := t.jobs[jobID]
	t.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	if job.Status == "running" {
		return fmt.Errorf("任务已在运行")
	}

	// 获取视频信息以计算总帧数
	info, err := t.GetVideoInfo(job.InputPath)
	if err != nil {
		job.Status = "failed"
		job.Error = fmt.Sprintf("获取视频信息失败: %v", err)
		return err
	}

	job.TotalFrames = int(info.Duration * info.Framerate)
	job.Status = "running"
	now := time.Now()
	job.StartTime = &now

	// 创建上下文
	job.ctx, job.cancel = context.WithCancel(context.Background())

	// 构建 ffmpeg 命令
	args := t.buildFFmpegArgs(job)

	// 启动 ffmpeg
	job.cmd = exec.CommandContext(job.ctx, t.ffmpegPath, args...)

	// 创建管道读取输出
	stderr, err := job.cmd.StderrPipe()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return err
	}

	if err := job.cmd.Start(); err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return err
	}

	// 启动 goroutine 解析进度
	go t.parseProgress(job, stderr)

	// 启动 goroutine 等待完成
	go t.waitForCompletion(job)

	return nil
}

// buildFFmpegArgs 构建 ffmpeg 参数
func (t *Transcoder) buildFFmpegArgs(job *TranscodeJob) []string {
	args := []string{"-i", job.InputPath}

	// 硬件加速
	switch strings.ToLower(job.Config.HWAccel) {
	case "cuda", "nvenc":
		args = append(args, "-hwaccel", "cuda")
	case "qsv":
		args = append(args, "-hwaccel", "qsv")
	case "vaapi":
		args = append(args, "-hwaccel", "vaapi")
	}

	// 视频编码
	if job.Config.CopyVideo {
		args = append(args, "-c:v", "copy")
	} else if job.Config.VideoCodec != "" {
		args = append(args, "-c:v", job.Config.VideoCodec)

		// 视频比特率
		if job.Config.VideoBitrate != "" {
			args = append(args, "-b:v", job.Config.VideoBitrate)
		}

		// CRF
		if job.Config.CRF > 0 && job.Config.VideoCodec == "libx264" {
			args = append(args, "-crf", fmt.Sprintf("%d", job.Config.CRF))
		}

		// 预设
		if job.Config.Preset != "" {
			args = append(args, "-preset", job.Config.Preset)
		}

		// 分辨率
		if job.Config.Resolution != "" {
			args = append(args, "-s", job.Config.Resolution)
		}

		// 帧率
		if job.Config.Framerate > 0 {
			args = append(args, "-r", fmt.Sprintf("%d", job.Config.Framerate))
		}
	}

	// 音频编码
	if job.Config.CopyAudio {
		args = append(args, "-c:a", "copy")
	} else if job.Config.AudioCodec != "" {
		args = append(args, "-c:a", job.Config.AudioCodec)

		if job.Config.AudioBitrate != "" {
			args = append(args, "-b:a", job.Config.AudioBitrate)
		}
	}

	// 输出格式
	if job.Config.OutputFormat != "" {
		args = append(args, "-f", job.Config.OutputFormat)
	}

	// 覆盖输出
	args = append(args, "-y", job.OutputPath)

	return args
}

// parseProgress 解析转码进度
func (t *Transcoder) parseProgress(job *TranscodeJob, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	var currentFrame int
	var _ float64 // fps ignored
	var speed string

	for scanner.Scan() {
		line := scanner.Text()
		job.mu.Lock()
		job.Log = append(job.Log, line)

		// 解析进度信息
		// frame= 123 fps=30 speed=1.5x
		if strings.Contains(line, "frame=") {
			// 解析 frame
			if f, err := parseFFmpegValue(line, "frame"); err == nil {
				currentFrame = f
				job.CurrentFrame = currentFrame
				if job.TotalFrames > 0 {
					job.Progress = float64(currentFrame) / float64(job.TotalFrames) * 100
					if job.Progress > 100 {
						job.Progress = 100
					}
				}
			}
			// 解析 fps
			if v, err := parseFFmpegFloatValue(line, "fps"); err == nil {
				_ = v
			}
			// 解析 speed
			if s := parseFFmpegSpeed(line); s != "" {
				speed = s
				job.Speed = speed
			}
		}
		job.mu.Unlock()
	}
}

// waitForCompletion 等待转码完成
func (t *Transcoder) waitForCompletion(job *TranscodeJob) {
	err := job.cmd.Wait()
	now := time.Now()

	job.mu.Lock()
	defer job.mu.Unlock()

	job.EndTime = &now

	if err != nil {
		if job.ctx.Err() == context.Canceled {
			job.Status = "cancelled"
		} else {
			job.Status = "failed"
			job.Error = err.Error()
		}
	} else {
		job.Status = "completed"
		job.Progress = 100
	}
}

// CancelJob 取消转码任务
func (t *Transcoder) CancelJob(jobID string) error {
	t.mu.RLock()
	job, ok := t.jobs[jobID]
	t.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	if job.Status != "running" {
		return fmt.Errorf("任务未在运行")
	}

	if job.cancel != nil {
		job.cancel()
	}

	job.Status = "cancelled"
	return nil
}

// GetJob 获取任务状态
func (t *Transcoder) GetJob(jobID string) *TranscodeJob {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.jobs[jobID]
}

// ListJobs 列出所有任务
func (t *Transcoder) ListJobs() []*TranscodeJob {
	t.mu.RLock()
	defer t.mu.RUnlock()

	jobs := make([]*TranscodeJob, 0, len(t.jobs))
	for _, job := range t.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// DeleteJob 删除任务
func (t *Transcoder) DeleteJob(jobID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	job, ok := t.jobs[jobID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	if job.Status == "running" {
		return fmt.Errorf("无法删除运行中的任务")
	}

	delete(t.jobs, jobID)
	return nil
}

// QuickConvert 快速转换（使用默认配置）
func (t *Transcoder) QuickConvert(inputPath, outputPath, format string) (*TranscodeJob, error) {
	config := DefaultTranscodeConfig()
	config.OutputFormat = format

	// 根据输出格式调整编码器
	switch format {
	case "webm":
		config.VideoCodec = "libvpx-vp9"
		config.AudioCodec = "libopus"
	case "mp4":
		config.VideoCodec = "libx264"
		config.AudioCodec = "aac"
	case "mkv":
		config.VideoCodec = "libx264"
		config.AudioCodec = "aac"
	}

	return t.CreateJob(inputPath, outputPath, config)
}

// OptimizeForWeb 优化为 Web 播放格式
func (t *Transcoder) OptimizeForWeb(inputPath, outputPath string) (*TranscodeJob, error) {
	config := TranscodeConfig{
		VideoCodec:   "libx264",
		AudioCodec:   "aac",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
		Preset:       "fast",
		CRF:          23,
		OutputFormat: "mp4",
	}

	return t.CreateJob(inputPath, outputPath, config)
}

// Helper functions

func parseFFmpegValue(line, key string) (int, error) {
	// 格式: key=value 或 key= value
	pattern := key + "="
	idx := strings.Index(line, pattern)
	if idx == -1 {
		return 0, fmt.Errorf("not found")
	}

	rest := line[idx+len(pattern):]
	rest = strings.TrimSpace(rest)

	// 找到第一个空格
	end := strings.Index(rest, " ")
	if end == -1 {
		end = len(rest)
	}

	value := rest[:end]
	return strconv.Atoi(value)
}

func parseFFmpegFloatValue(line, key string) (float64, error) {
	pattern := key + "="
	idx := strings.Index(line, pattern)
	if idx == -1 {
		return 0, fmt.Errorf("not found")
	}

	rest := line[idx+len(pattern):]
	rest = strings.TrimSpace(rest)

	end := strings.Index(rest, " ")
	if end == -1 {
		end = len(rest)
	}

	value := rest[:end]
	return strconv.ParseFloat(value, 64)
}

func parseFFmpegSpeed(line string) string {
	pattern := "speed="
	idx := strings.Index(line, pattern)
	if idx == -1 {
		return ""
	}

	rest := line[idx+len(pattern):]
	rest = strings.TrimSpace(rest)

	end := strings.Index(rest, "x")
	if end == -1 {
		return ""
	}

	return rest[:end+1] + "x"
}