package filepreview

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// AudioPreviewer 音频预览器.
type AudioPreviewer struct {
	config    *PreviewConfig
	semaphore chan struct{}
	mu        sync.RWMutex
}

// NewAudioPreviewer 创建音频预览器.
func NewAudioPreviewer(config *PreviewConfig) *AudioPreviewer {
	if config == nil {
		config = DefaultPreviewConfig()
	}
	return &AudioPreviewer{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrent),
	}
}

// Generate 生成音频波形预览.
func (p *AudioPreviewer) Generate(ctx context.Context, req *PreviewRequest) (*PreviewResult, error) {
	// 检查文件.
	info, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	if info.Size() > p.config.MaxFileSize {
		return nil, fmt.Errorf("%w: 文件大小 %d 超过限制 %d", ErrInvalidSize, info.Size(), p.config.MaxFileSize)
	}

	// 检测音频格式.
	format := DetectAudioFormat(req.FilePath)
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

	// 获取音频信息.
	audioInfo, err := p.GetAudioInfo(ctx, req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("获取音频信息失败: %w", err)
	}

	// 生成波形图.
	width := req.Width
	if width <= 0 {
		width = 1200
	}
	height := req.Height
	if height <= 0 {
		height = 200
	}

	outputPath, err := p.generateWaveformImage(ctx, req.FilePath, width, height)
	if err != nil {
		return nil, err
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    req.FilePath,
		FileType:    FileTypeAudio,
		PreviewPath: outputPath,
		ContentType: "image/png",
		Width:       width,
		Height:      height,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		Duration:    audioInfo.Duration,
		Metadata: map[string]string{
			"codec":      audioInfo.Codec,
			"sample_rate": fmt.Sprintf("%d", audioInfo.SampleRate),
			"channels":   fmt.Sprintf("%d", audioInfo.Channels),
			"bitrate":    fmt.Sprintf("%d", audioInfo.Bitrate),
		},
	}, nil
}

// GetAudioInfo 获取音频信息.
func (p *AudioPreviewer) GetAudioInfo(ctx context.Context, filePath string) (*AudioInfo, error) {
	format := DetectAudioFormat(filePath)
	if format == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(filePath))
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	// 使用 ffprobe 获取音频信息.
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

	// 解析 JSON.
	var probeData struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
			Duration   string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Bitrate  string `json:"bit_rate"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		return nil, fmt.Errorf("解析 ffprobe 输出失败: %w", err)
	}

	audioInfo := &AudioInfo{
		FilePath: filePath,
		Format:   format,
		FileSize: info.Size(),
	}

	// 解析时长.
	if probeData.Format.Duration != "" {
		audioInfo.Duration, _ = strconv.ParseFloat(probeData.Format.Duration, 64)
	}

	// 解析比特率.
	if probeData.Format.Bitrate != "" {
		audioInfo.Bitrate, _ = strconv.ParseInt(probeData.Format.Bitrate, 10, 64)
	}

	// 解析流信息.
	for _, stream := range probeData.Streams {
		if stream.CodecType == "audio" {
			audioInfo.Codec = stream.CodecName
			audioInfo.Channels = stream.Channels
			if stream.SampleRate != "" {
				audioInfo.SampleRate, _ = strconv.Atoi(stream.SampleRate)
			}
			if stream.Duration != "" && audioInfo.Duration == 0 {
				audioInfo.Duration, _ = strconv.ParseFloat(stream.Duration, 64)
			}
			break
		}
	}

	return audioInfo, nil
}

// ExtractWaveformData 提取波形数据.
func (p *AudioPreviewer) ExtractWaveformData(ctx context.Context, filePath string, samples int) (*WaveformData, error) {
	// 检查文件.
	if _, err := os.Stat(filePath); err != nil {
		return nil, ErrFileNotFound
	}

	// 获取音频信息.
	audioInfo, err := p.GetAudioInfo(ctx, filePath)
	if err != nil {
		return nil, err
	}

	if samples <= 0 {
		samples = 1000
	}

	// 限制并发.
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 使用 ffmpeg 提取 PCM 数据.
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-ac", "1", // 单声道
		"-ar", "8000", // 低采样率
		"-f", "s16le", // 16-bit PCM
		"-acodec", "pcm_s16le",
		"pipe:1",
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("提取 PCM 数据失败: %s", stderr.String())
	}

	// 解析 PCM 数据.
	pcmData := stdout.Bytes()
	totalSamples := len(pcmData) / 2 // 16-bit = 2 bytes per sample

	if totalSamples == 0 {
		return nil, fmt.Errorf("无音频数据")
	}

	// 转换为 float64 并归一化.
	waveformSamples := make([]float64, totalSamples)
	for i := 0; i < totalSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(pcmData[i*2 : i*2+2]))
		waveformSamples[i] = float64(sample) / 32768.0
	}

	// 下采样到请求数量.
	downsampled := p.downsample(waveformSamples, samples)

	// 计算波峰.
	peaks := p.calculatePeaks(waveformSamples, samples*2)

	return &WaveformData{
		Samples:    downsampled,
		Duration:   audioInfo.Duration,
		SampleRate: 8000,
		Channels:   1,
		Peaks:      peaks,
	}, nil
}

// GenerateSpectrogram 生成频谱图.
func (p *AudioPreviewer) GenerateSpectrogram(ctx context.Context, filePath string, width, height int) (*PreviewResult, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, ErrFileNotFound
	}

	if width <= 0 {
		width = 1200
	}
	if height <= 0 {
		height = 400
	}

	// 限制并发.
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	outputPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + "_spectrogram.png"

	// 使用 ffmpeg 生成频谱图.
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-lavfi", fmt.Sprintf("showspectrumpic=s=%dx%d:mode=combined:color=intensity:scale=log", width, height),
		"-y",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("生成频谱图失败: %s", stderr.String())
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    filePath,
		FileType:    FileTypeAudio,
		PreviewPath: outputPath,
		ContentType: "image/png",
		Width:       width,
		Height:      height,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		Metadata: map[string]string{
			"type": "spectrogram",
		},
	}, nil
}

// ExtractAudioCover 提取音频封面.
func (p *AudioPreviewer) ExtractAudioCover(ctx context.Context, filePath string) (*PreviewResult, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, ErrFileNotFound
	}

	outputPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + "_cover.jpg"

	// 使用 ffmpeg 提取封面.
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-an", // 不处理音频
		"-vcodec", "mjpeg",
		"-vframes", "1",
		"-y",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("提取封面失败: %s", stderr.String())
	}

	// 检查是否提取成功.
	if _, err := os.Stat(outputPath); err != nil {
		return nil, fmt.Errorf("音频无封面图片")
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    filePath,
		FileType:    FileTypeAudio,
		PreviewPath: outputPath,
		ContentType: "image/jpeg",
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		Metadata: map[string]string{
			"type": "cover_art",
		},
	}, nil
}

// generateWaveformImage 生成波形图.
func (p *AudioPreviewer) generateWaveformImage(ctx context.Context, filePath string, width, height int) (string, error) {
	outputPath := fmt.Sprintf("%s_waveform_%dx%d.png", filePath, width, height)

	// 使用 ffmpeg 的 showwaves 滤镜.
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-filter_complex", fmt.Sprintf(
			"[0:a]showwaves=s=%dx%d:mode=cline:rate=25:colors=0x00AAFF|0xFF6600:scale=sqrt[v]",
			width, height,
		),
		"-map", "[v]",
		"-frames:v", "1",
		"-y",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// 回退到简单波形.
		return p.generateSimpleWaveform(ctx, filePath, width, height)
	}

	return outputPath, nil
}

// generateSimpleWaveform 生成简单波形图.
func (p *AudioPreviewer) generateSimpleWaveform(ctx context.Context, filePath string, width, height int) (string, error) {
	outputPath := fmt.Sprintf("%s_waveform_%dx%d.png", filePath, width, height)

	// 使用 showwavespic 生成静态波形图.
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath,
		"-i", filePath,
		"-filter_complex", fmt.Sprintf(
			"showwavespic=s=%dx%d:colors=0x00AAFF:split_channels=0",
			width, height,
		),
		"-frames:v", "1",
		"-y",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("生成波形图失败: %s", stderr.String())
	}

	return outputPath, nil
}

// downsample 下采样数据.
func (p *AudioPreviewer) downsample(data []float64, targetLen int) []float64 {
	if len(data) == 0 {
		return data
	}

	if targetLen <= 0 {
		return data
	}

	if len(data) <= targetLen {
		return data
	}

	result := make([]float64, targetLen)
	step := float64(len(data)) / float64(targetLen)

	for i := 0; i < targetLen; i++ {
		start := int(float64(i) * step)
		end := int(float64(i+1) * step)
		if end > len(data) {
			end = len(data)
		}

		// 计算该区间的 RMS 值.
		var sum float64
		for j := start; j < end; j++ {
			sum += data[j] * data[j]
		}
		if end > start {
			result[i] = math.Sqrt(sum / float64(end-start))
		}
	}

	return result
}

// calculatePeaks 计算波峰数据.
func (p *AudioPreviewer) calculatePeaks(data []float64, targetLen int) []float64 {
	if len(data) <= targetLen {
		return data
	}

	result := make([]float64, targetLen)
	step := float64(len(data)) / float64(targetLen)

	for i := 0; i < targetLen; i++ {
		start := int(float64(i) * step)
		end := int(float64(i+1) * step)
		if end > len(data) {
			end = len(data)
		}

		// 找最大绝对值.
		maxVal := 0.0
		for j := start; j < end; j++ {
			absVal := math.Abs(data[j])
			if absVal > maxVal {
				maxVal = absVal
			}
		}
		result[i] = maxVal
	}

	return result
}

// GetAudioMetadata 获取音频元数据.
func (p *AudioPreviewer) GetAudioMetadata(ctx context.Context, filePath string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, p.config.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_entries", "format_tags",
		filePath,
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var data struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		return nil, err
	}

	return data.Format.Tags, nil
}
