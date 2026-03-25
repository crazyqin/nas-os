package media

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ThumbnailConfig 缩略图配置
type ThumbnailConfig struct {
	// 宽度
	Width int `json:"width"`
	// 高度
	Height int `json:"height"`
	// 输出格式 (jpg, png, webp)
	Format string `json:"format"`
	// 质量 (1-100)
	Quality int `json:"quality"`
	// 时间点（秒）
	Timestamp float64 `json:"timestamp"`
	// 是否保持宽高比
	KeepAspectRatio bool `json:"keepAspectRatio"`
}

// ThumbnailGenerator 缩略图生成器
type ThumbnailGenerator struct {
	ffmpegPath  string
	ffprobePath string
}

// NewThumbnailGenerator 创建缩略图生成器
func NewThumbnailGenerator(ffmpegPath, ffprobePath string) *ThumbnailGenerator {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	return &ThumbnailGenerator{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
	}
}

// DefaultThumbnailConfig 默认缩略图配置
func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{
		Width:           320,
		Height:          180,
		Format:          "jpg",
		Quality:         85,
		Timestamp:       0,
		KeepAspectRatio: true,
	}
}

// GenerateFromVideo 从视频生成缩略图
func (tg *ThumbnailGenerator) GenerateFromVideo(videoPath, outputPath string, config ThumbnailConfig) error {
	// 检查输入文件
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("视频文件不存在: %s", videoPath)
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 构建 ffmpeg 命令
	args := tg.buildThumbnailArgs(videoPath, outputPath, config)

	cmd := exec.CommandContext(context.Background(), tg.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("生成缩略图失败: %s", string(output))
	}

	return nil
}

// buildThumbnailArgs 构建 ffmpeg 缩略图参数
func (tg *ThumbnailGenerator) buildThumbnailArgs(inputPath, outputPath string, config ThumbnailConfig) []string {
	args := []string{}

	// 时间点
	if config.Timestamp > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", config.Timestamp))
	}

	// 输入文件
	args = append(args, "-i", inputPath)

	// 只处理一帧
	args = append(args, "-vframes", "1")

	// 缩放
	if config.Width > 0 && config.Height > 0 {
		if config.KeepAspectRatio {
			// 保持宽高比缩放
			args = append(args, "-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", config.Width, config.Height))
		} else {
			args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", config.Width, config.Height))
		}
	} else if config.Width > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:-1", config.Width))
	} else if config.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=-1:%d", config.Height))
	}

	// 输出格式和质量
	switch strings.ToLower(config.Format) {
	case "png":
		args = append(args, "-f", "image2", "-c:v", "png")
	case "webp":
		args = append(args, "-f", "image2", "-c:v", "libwebp")
		if config.Quality > 0 {
			args = append(args, "-quality", fmt.Sprintf("%d", config.Quality))
		}
	default: // jpg
		args = append(args, "-f", "image2", "-c:v", "mjpeg")
		if config.Quality > 0 && config.Quality <= 100 {
			args = append(args, "-q:v", fmt.Sprintf("%d", 31-config.Quality*31/100))
		}
	}

	// 覆盖输出
	args = append(args, "-y", outputPath)

	return args
}

// GenerateMultiple 从视频生成多个缩略图（均匀分布）
func (tg *ThumbnailGenerator) GenerateMultiple(videoPath, outputDir string, count int, config ThumbnailConfig) ([]string, error) {
	// 获取视频时长
	duration, err := tg.getVideoDuration(videoPath)
	if err != nil {
		return nil, fmt.Errorf("获取视频时长失败: %w", err)
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 计算时间间隔
	interval := duration / float64(count+1)
	timestamps := make([]float64, count)
	for i := 0; i < count; i++ {
		timestamps[i] = interval * float64(i+1)
	}

	// 生成缩略图
	paths := make([]string, 0, count)
	ext := "." + config.Format
	if ext == "." {
		ext = ".jpg"
	}

	for i, ts := range timestamps {
		outputPath := filepath.Join(outputDir, fmt.Sprintf("thumb_%03d%s", i+1, ext))
		cfg := config
		cfg.Timestamp = ts

		if err := tg.GenerateFromVideo(videoPath, outputPath, cfg); err != nil {
			// 记录错误但继续
			continue
		}
		paths = append(paths, outputPath)
	}

	return paths, nil
}

// GenerateSprite 生成缩略图精灵图（用于视频预览条）
func (tg *ThumbnailGenerator) GenerateSprite(videoPath, outputPath string, cols, rows int, config ThumbnailConfig) (*SpriteInfo, error) {
	// 获取视频时长
	duration, err := tg.getVideoDuration(videoPath)
	if err != nil {
		return nil, fmt.Errorf("获取视频时长失败: %w", err)
	}

	// 计算需要的缩略图数量
	totalThumbs := cols * rows

	// 计算时间间隔
	interval := duration / float64(totalThumbs+1)

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "thumbs_*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// 生成所有缩略图
	thumbs := make([]string, 0, totalThumbs)
	for i := 0; i < totalThumbs; i++ {
		ts := interval * float64(i+1)
		thumbPath := filepath.Join(tempDir, fmt.Sprintf("thumb_%03d.jpg", i))
		cfg := config
		cfg.Timestamp = ts
		cfg.Format = "jpg"

		if err := tg.GenerateFromVideo(videoPath, thumbPath, cfg); err != nil {
			continue
		}
		thumbs = append(thumbs, thumbPath)
	}

	// 使用 ffmpeg 拼接成精灵图
	spriteInfo, err := tg.createSprite(thumbs, outputPath, cols, rows, config)
	if err != nil {
		return nil, err
	}

	return spriteInfo, nil
}

// SpriteInfo 精灵图信息
type SpriteInfo struct {
	Path       string    `json:"path"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Cols       int       `json:"cols"`
	Rows       int       `json:"rows"`
	Timestamps []float64 `json:"timestamps"`
	Duration   float64   `json:"duration"`
}

// createSprite 创建精灵图
func (tg *ThumbnailGenerator) createSprite(thumbs []string, outputPath string, cols, rows int, config ThumbnailConfig) (*SpriteInfo, error) {
	if len(thumbs) == 0 {
		return nil, fmt.Errorf("没有可用的缩略图")
	}

	// 读取第一个缩略图获取尺寸
	firstThumb, err := os.Open(thumbs[0])
	if err != nil {
		return nil, err
	}
	defer func() { _ = firstThumb.Close() }()

	img, _, err := image.Decode(firstThumb)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	thumbWidth := img.Bounds().Dx()
	thumbHeight := img.Bounds().Dy()

	// 创建精灵图画布
	spriteWidth := thumbWidth * cols
	spriteHeight := thumbHeight * rows

	// 使用 ffmpeg 拼接
	// 创建文件列表
	listFile := filepath.Join(filepath.Dir(thumbs[0]), "list.txt")
	listContent := strings.Builder{}
	for _, t := range thumbs {
		fmt.Fprintf(&listContent, "file '%s'\n", t)
	}
	if err := os.WriteFile(listFile, []byte(listContent.String()), 0640); err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(listFile) }()

	// 使用 ffmpeg 的 tile 滤镜
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-vf", fmt.Sprintf("tile=%dx%d", cols, rows),
		"-frames:v", "1",
		"-y", outputPath,
	}

	cmd := exec.CommandContext(context.Background(), tg.ffmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("创建精灵图失败: %s", string(output))
	}

	return &SpriteInfo{
		Path:       outputPath,
		Width:      spriteWidth,
		Height:     spriteHeight,
		Cols:       cols,
		Rows:       rows,
		Timestamps: make([]float64, 0),
	}, nil
}

// GenerateFromImage 从图片生成缩略图
func (tg *ThumbnailGenerator) GenerateFromImage(inputPath, outputPath string, config ThumbnailConfig) error {
	// 打开输入图片
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("打开图片失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	// 解码图片
	img, format, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("解码图片失败: %w", err)
	}

	// 计算缩放后的尺寸
	bounds := img.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	dstWidth := config.Width
	dstHeight := config.Height

	if config.KeepAspectRatio && dstWidth > 0 && dstHeight > 0 {
		// 计算保持比例的尺寸
		srcRatio := float64(srcWidth) / float64(srcHeight)
		dstRatio := float64(dstWidth) / float64(dstHeight)

		if srcRatio > dstRatio {
			dstHeight = int(float64(dstWidth) / srcRatio)
		} else {
			dstWidth = int(float64(dstHeight) * srcRatio)
		}
	}

	// 缩放图片（使用简单缩放，实际项目可用更好的库）
	resized := tg.resizeImage(img, dstWidth, dstHeight)

	// 创建输出文件
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	// 编码输出
	outputFormat := config.Format
	if outputFormat == "" {
		outputFormat = format
	}

	switch strings.ToLower(outputFormat) {
	case "png":
		return png.Encode(outFile, resized)
	default:
		return jpeg.Encode(outFile, resized, &jpeg.Options{Quality: config.Quality})
	}
}

// resizeImage 简单的图片缩放
func (tg *ThumbnailGenerator) resizeImage(img image.Image, width, height int) image.Image {
	if width <= 0 || height <= 0 {
		return img
	}

	// 创建目标图像
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	// 简单的最近邻插值
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := x * srcW / width
			srcY := y * srcH / height
			dst.Set(x, y, img.At(srcX+srcBounds.Min.X, srcY+srcBounds.Min.Y))
		}
	}

	return dst
}

// getVideoDuration 获取视频时长
func (tg *ThumbnailGenerator) getVideoDuration(videoPath string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, tg.ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

// GeneratePreviewGif 生成预览 GIF
func (tg *ThumbnailGenerator) GeneratePreviewGif(videoPath, outputPath string, duration float64, config ThumbnailConfig) error {
	// 获取视频总时长
	videoDuration, err := tg.getVideoDuration(videoPath)
	if err != nil {
		return fmt.Errorf("获取视频时长失败: %w", err)
	}

	if duration <= 0 || duration > videoDuration {
		duration = videoDuration
		if duration > 10 {
			duration = 10 // 默认最多 10 秒
		}
	}

	// 构建 ffmpeg 命令
	args := []string{
		"-t", fmt.Sprintf("%.2f", duration),
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=10,scale=%d:%d:force_original_aspect_ratio=decrease,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse", config.Width, config.Height),
		"-loop", "0",
		"-y", outputPath,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, tg.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("生成 GIF 失败: %s", string(output))
	}

	return nil
}

// BatchGenerate 批量生成缩略图
func (tg *ThumbnailGenerator) BatchGenerate(videos []string, outputDir string, config ThumbnailConfig, concurrency int) map[string]error {
	results := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 限制并发
	if concurrency <= 0 {
		concurrency = 2
	}
	sem := make(chan struct{}, concurrency)

	for _, video := range videos {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 生成输出路径
			baseName := strings.TrimSuffix(filepath.Base(v), filepath.Ext(v))
			ext := "." + config.Format
			if ext == "." {
				ext = ".jpg"
			}
			outputPath := filepath.Join(outputDir, baseName+ext)

			err := tg.GenerateFromVideo(v, outputPath, config)

			mu.Lock()
			results[v] = err
			mu.Unlock()
		}(video)
	}

	wg.Wait()
	return results
}

// GenerateWithProgress 带进度的缩略图生成
func (tg *ThumbnailGenerator) GenerateWithProgress(ctx context.Context, videoPath, outputPath string, config ThumbnailConfig, progress chan<- float64) error {
	defer close(progress)

	err := tg.GenerateFromVideo(videoPath, outputPath, config)
	if err != nil {
		return err
	}

	// 发送完成进度
	select {
	case progress <- 100:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// GetThumbnailInfo 获取缩略图信息
func (tg *ThumbnailGenerator) GetThumbnailInfo(path string) (*ThumbnailInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	img, format, err := image.DecodeConfig(file)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &ThumbnailInfo{
		Path:      path,
		Width:     img.Width,
		Height:    img.Height,
		Format:    format,
		Size:      stat.Size(),
		CreatedAt: stat.ModTime(),
	}, nil
}

// ThumbnailInfo 缩略图信息
type ThumbnailInfo struct {
	Path      string    `json:"path"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Format    string    `json:"format"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}
