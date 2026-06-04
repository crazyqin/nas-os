package filepreview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// VideoPreviewer 视频预览器.
type VideoPreviewer struct {
	config    *PreviewConfig
	semaphore chan struct{}
	mu        sync.RWMutex
}

// NewVideoPreviewer 创建视频预览器.
func NewVideoPreviewer(config *PreviewConfig) *VideoPreviewer {
	if config == nil {
		config = DefaultPreviewConfig()
	}
	return &VideoPreviewer{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrent),
	}
}

// Generate 生成视频预览（缩略图）.
func (p *VideoPreviewer) Generate(ctx context.Context, req *PreviewRequest) (*PreviewResult, error) {
	// 检查文件是否存在.
	info, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	// 检查文件大小.
	if info.Size() > p.config.MaxFileSize {
		return nil, fmt.Errorf("%w: 文件大小 %d 超过限制 %d", ErrInvalidSize, info.Size(), p.config.MaxFileSize)
	}

	// 检测视频格式.
	format := DetectVideoFormat(req.FilePath)
	if format == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(req.FilePath))
	}

	// 限制并发.
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 获取视频信息.
	videoInfo, err := p.GetVideoInfo(ctx, req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}

	// 确定时间戳.
	timestamp := req.Timestamp
	if timestamp <= 0 {
		// 默认取 10% 位置，但至少 1 秒.
		timestamp = videoInfo.Duration * 0.1
		if timestamp < 1 {
			timestamp = 1
		}
	}

	// 确保时间戳不超过视频时长.
	if timestamp > videoInfo.Duration {
		timestamp = videoInfo.Duration * 0.9
	}

	// 获取输出尺寸.
	width, height := p.getOutputSize(req, videoInfo)

	// 生成缩略图.
	outputPath, err := p.extractFrame(ctx, req.FilePath, timestamp, width, height)
	if err != nil {
		return nil, err
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    req.FilePath,
		FileType:    FileTypeVideo,
		PreviewPath: outputPath,
		ContentType: "image/jpeg",
		Width:       width,
		Height:      height,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		Duration:    videoInfo.Duration,
		Metadata: map[string]string{
			"video_codec": videoInfo.Codec,
			"audio_codec": videoInfo.AudioCodec,
			"fps":         fmt.Sprintf("%.2f", videoInfo.FPS),
			"bitrate":     fmt.Sprintf("%d", videoInfo.Bitrate),
		},
	}, nil
}

// GetVideoInfo 获取视频信息.
func (p *VideoPreviewer) GetVideoInfo(ctx context.Context, filePath string) (*VideoInfo, error) {
	format := DetectVideoFormat(filePath)
	if format == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(filePath))
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	// 使用 ffprobe 获取视频信息.
	cmd := exec.CommandContext(ctx, p.config.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe 执行失败: %w", err)
	}

	// 解析 JSON 输出.
	var probeData struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
			Duration   string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Bitrate  string `json:"bit_rate"`
			Size     string `json:"size"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		return nil, fmt.Errorf("解析 ffprobe 输出失败: %w", err)
	}

	videoInfo := &VideoInfo{
		FilePath: filePath,
		Format:   format,
		FileSize: info.Size(),
	}

	// 解析时长.
	if probeData.Format.Duration != "" {
		videoInfo.Duration, _ = strconv.ParseFloat(probeData.Format.Duration, 64)
	}

	// 解析比特率.
	if probeData.Format.Bitrate != "" {
		videoInfo.Bitrate, _ = strconv.ParseInt(probeData.Format.Bitrate, 10, 64)
	}

	// 解析流信息.
	for _, stream := range probeData.Streams {
		switch stream.CodecType {
		case "video":
			videoInfo.Width = stream.Width
			videoInfo.Height = stream.Height
			videoInfo.Codec = stream.CodecName
			videoInfo.FPS = parseFrameRate(stream.RFrameRate)
			if stream.Duration != "" && videoInfo.Duration == 0 {
				videoInfo.Duration, _ = strconv.ParseFloat(stream.Duration, 64)
			}
		case "audio":
			videoInfo.AudioCodec = stream.CodecName
		}
	}

	return videoInfo, nil
}

// ExtractKeyFrames 提取关键帧.
func (p *VideoPreviewer) ExtractKeyFrames(ctx context.Context, filePath string, count int, width, height int) ([]KeyFrame, error) {
	// 检查文件.
	if _, err := os.Stat(filePath); err != nil {
		return nil, ErrFileNotFound
	}

	// 获取视频信息.
	videoInfo, err := p.GetVideoInfo(ctx, filePath)
	if err != nil {
		return nil, err
	}

	if count <= 0 {
		count = 10 // 默认提取 10 帧
	}

	// 限制并发.
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 创建临时目录.
	tmpDir, err := os.MkdirTemp("", "keyframes-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 使用 ffmpeg 提取关键帧.
	outputPattern := filepath.Join(tmpDir, "frame_%04d.jpg")
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-vf", fmt.Sprintf("select='gt(scene,0.3)',scale=%d:%d", width, height),
		"-vsync", "vfr",
		"-frames:v", fmt.Sprintf("%d", count*2), // 多提取一些以供选择
		"-q:v", "2",
		outputPattern,
	)
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		// 回退到均匀采样.
		return p.extractUniformFrames(ctx, filePath, videoInfo, count, width, height)
	}

	// 收集生成的帧.
	frames, err := p.collectFrames(tmpDir, videoInfo)
	if err != nil {
		return nil, err
	}

	// 如果帧数不足，补充均匀采样帧.
	if len(frames) < count {
		uniformFrames, err := p.extractUniformFrames(ctx, filePath, videoInfo, count-len(frames), width, height)
		if err == nil {
			frames = append(frames, uniformFrames...)
		}
	}

	// 限制到请求数量.
	if len(frames) > count {
		frames = frames[:count]
	}

	return frames, nil
}

// ExtractFrameAtTime 在指定时间提取帧.
func (p *VideoPreviewer) ExtractFrameAtTime(ctx context.Context, filePath string, timestamp float64, width, height int) (*PreviewResult, error) {
	req := &PreviewRequest{
		FilePath:  filePath,
		Timestamp: timestamp,
		Width:     width,
		Height:    height,
	}
	return p.Generate(ctx, req)
}

// GenerateSpriteSheet 生成视频雪碧图（用于进度条预览）.
func (p *VideoPreviewer) GenerateSpriteSheet(ctx context.Context, filePath string, cols, rows int, thumbWidth, thumbHeight int) (*PreviewResult, error) {
	videoInfo, err := p.GetVideoInfo(ctx, filePath)
	if err != nil {
		return nil, err
	}

	totalFrames := cols * rows
	interval := videoInfo.Duration / float64(totalFrames+1)

	// 创建临时目录.
	tmpDir, err := os.MkdirTemp("", "sprite-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 提取帧.
	outputPattern := filepath.Join(tmpDir, "frame_%04d.jpg")
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-vf", fmt.Sprintf("fps=1/%.2f,scale=%d:%d", interval, thumbWidth, thumbHeight),
		"-frames:v", fmt.Sprintf("%d", totalFrames),
		"-q:v", "3",
		outputPattern,
	)
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("提取雪碧图帧失败: %w", err)
	}

	// 使用 ImageMagick 合并雪碧图.
	spritePath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + "_sprite.jpg"
	cmd = exec.CommandContext(ctx, "montage",
		filepath.Join(tmpDir, "frame_*.jpg"),
		"-tile", fmt.Sprintf("%dx%d", cols, rows),
		"-geometry", fmt.Sprintf("%dx%d+0+0", thumbWidth, thumbHeight),
		spritePath,
	)
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("生成雪碧图失败: %w", err)
	}

	stat, _ := os.Stat(spritePath)
	return &PreviewResult{
		FilePath:    filePath,
		FileType:    FileTypeVideo,
		PreviewPath: spritePath,
		ContentType: "image/jpeg",
		Width:       cols * thumbWidth,
		Height:      rows * thumbHeight,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		Duration:    videoInfo.Duration,
		Metadata: map[string]string{
			"type":  "sprite_sheet",
			"cols":  fmt.Sprintf("%d", cols),
			"rows":  fmt.Sprintf("%d", rows),
			"total": fmt.Sprintf("%d", totalFrames),
		},
	}, nil
}

// extractFrame 提取单帧.
func (p *VideoPreviewer) extractFrame(ctx context.Context, filePath string, timestamp float64, width, height int) (string, error) {
	outputPath := fmt.Sprintf("%s_thumb_%dx%d.jpg", filePath, width, height)

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", filePath,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2", width, height, width, height),
		"-q:v", "2",
		"-y",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: 提取帧失败: %s", ErrGenerationFailed, stderr.String())
	}

	if _, err := os.Stat(outputPath); err != nil {
		return "", fmt.Errorf("%w: 输出文件未生成", ErrGenerationFailed)
	}

	return outputPath, nil
}

// extractUniformFrames 均匀采样提取帧.
func (p *VideoPreviewer) extractUniformFrames(ctx context.Context, filePath string, videoInfo *VideoInfo, count, width, height int) ([]KeyFrame, error) {
	tmpDir, err := os.MkdirTemp("", "uniform-frames-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	interval := videoInfo.Duration / float64(count+1)
	outputPattern := filepath.Join(tmpDir, "frame_%04d.jpg")

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-vf", fmt.Sprintf("fps=1/%.2f,scale=%d:%d", interval, width, height),
		"-frames:v", fmt.Sprintf("%d", count),
		"-q:v", "3",
		outputPattern,
	)
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("均匀采样失败: %w", err)
	}

	return p.collectFrames(tmpDir, videoInfo)
}

// collectFrames 收集生成的帧.
func (p *VideoPreviewer) collectFrames(tmpDir string, videoInfo *VideoInfo) ([]KeyFrame, error) {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}

	var frames []KeyFrame
	for i, entry := range entries {
		if entry.IsDir() {
			continue
		}

		framePath := filepath.Join(tmpDir, entry.Name())
		stat, err := os.Stat(framePath)
		if err != nil {
			continue
		}

		frame := KeyFrame{
			Timestamp:   float64(i) * videoInfo.Duration / float64(len(entries)+1),
			PreviewPath: framePath,
			IsKeyFrame:  true,
		}

		// 从文件获取尺寸信息（简化处理）.
		frame.Width = 320
		frame.Height = 180

		frames = append(frames, frame)
		_ = stat
	}

	return frames, nil
}

// getOutputSize 获取输出尺寸.
func (p *VideoPreviewer) getOutputSize(req *PreviewRequest, videoInfo *VideoInfo) (width, height int) {
	width = req.Width
	height = req.Height

	if width <= 0 && height <= 0 {
		width = 640
		height = 360
	} else if width <= 0 {
		// 根据高度按比例计算宽度.
		ratio := float64(videoInfo.Width) / float64(videoInfo.Height)
		width = int(float64(height) * ratio)
	} else if height <= 0 {
		// 根据宽度按比例计算高度.
		ratio := float64(videoInfo.Height) / float64(videoInfo.Width)
		height = int(float64(width) * ratio)
	}

	return width, height
}

// parseFrameRate 解析帧率.
func parseFrameRate(fps string) float64 {
	if fps == "" {
		return 0
	}

	parts := strings.Split(fps, "/")
	if len(parts) != 2 {
		val, _ := strconv.ParseFloat(fps, 64)
		return val
	}

	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}

	return num / den
}
