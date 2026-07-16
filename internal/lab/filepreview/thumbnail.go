package filepreview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// ThumbnailGenerator 缩略图生成器.
type ThumbnailGenerator struct {
	config    *PreviewConfig
	semaphore chan struct{}
	// rawConverter RAW 转换器（使用 dcraw 或 libraw）.
	rawConverter string
	mu           sync.RWMutex
}

// NewThumbnailGenerator 创建缩略图生成器.
func NewThumbnailGenerator(config *PreviewConfig) *ThumbnailGenerator {
	if config == nil {
		config = DefaultPreviewConfig()
	}
	return &ThumbnailGenerator{
		config:       config,
		semaphore:    make(chan struct{}, config.MaxConcurrent),
		rawConverter: "dcraw",
	}
}

// Generate 生成缩略图.
func (g *ThumbnailGenerator) Generate(ctx context.Context, req *PreviewRequest) (*PreviewResult, error) {
	// 检查文件是否存在.
	info, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	// 检查文件大小.
	if info.Size() > g.config.MaxFileSize {
		return nil, fmt.Errorf("%w: 文件大小 %d 超过限制 %d", ErrInvalidSize, info.Size(), g.config.MaxFileSize)
	}

	// 检测图片格式.
	format := DetectImageFormat(req.FilePath)
	if format == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(req.FilePath))
	}

	// 获取输出尺寸.
	width, height := g.getOutputSize(req)

	// 限制并发.
	select {
	case g.semaphore <- struct{}{}:
		defer func() { <-g.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 根据格式选择生成方式.
	switch format {
	case FormatRAW:
		return g.generateFromRAW(ctx, req.FilePath, width, height, req.Quality)
	case FormatHEIC:
		return g.generateFromHEIC(ctx, req.FilePath, width, height, req.Quality)
	default:
		return g.generateFromStandard(ctx, req.FilePath, format, width, height, req.Quality)
	}
}

// GenerateAllSizes 生成所有预设尺寸的缩略图.
func (g *ThumbnailGenerator) GenerateAllSizes(ctx context.Context, filePath string, quality int) (map[ThumbnailSize]*PreviewResult, error) {
	results := make(map[ThumbnailSize]*PreviewResult)

	for _, size := range g.config.Cache.ThumbnailSizes {
		width, height := GetThumbnailDimensions(size)
		req := &PreviewRequest{
			FilePath: filePath,
			Width:    width,
			Height:   height,
			Quality:  quality,
		}

		result, err := g.Generate(ctx, req)
		if err != nil {
			return results, fmt.Errorf("生成 %s 缩略图失败: %w", size, err)
		}
		results[size] = result
	}

	return results, nil
}

// GetImageInfo 获取图片信息.
func (g *ThumbnailGenerator) GetImageInfo(ctx context.Context, filePath string) (*ImageInfo, error) {
	format := DetectImageFormat(filePath)
	if format == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(filePath))
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	// 解码图片获取尺寸.
	reader, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var imgConfig image.Config
	switch format {
	case FormatJPEG:
		imgConfig, err = jpeg.DecodeConfig(reader)
	case FormatPNG:
		imgConfig, err = png.DecodeConfig(reader)
	case FormatWebP:
		imgConfig, err = webp.DecodeConfig(reader)
	default:
		// 使用通用方法.
		imgConfig, _, err = image.DecodeConfig(reader)
	}
	if err != nil {
		return nil, fmt.Errorf("解码图片配置失败: %w", err)
	}

	return &ImageInfo{
		FilePath:   filePath,
		Format:     format,
		Width:      imgConfig.Width,
		Height:     imgConfig.Height,
		FileSize:   info.Size(),
		ColorSpace: "RGB", // 默认色彩空间
		HasAlpha:   hasAlphaChannel(imgConfig.ColorModel),
	}, nil
}

// generateFromStandard 从标准格式生成缩略图.
func (g *ThumbnailGenerator) generateFromStandard(ctx context.Context, filePath string, format ImageFormat, width, height, quality int) (*PreviewResult, error) {
	// 打开源文件.
	srcFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer srcFile.Close()

	// 解码图片.
	var src image.Image
	switch format {
	case FormatWebP:
		src, err = webp.Decode(srcFile)
	default:
		src, _, err = image.Decode(srcFile)
	}
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	// 生成缩略图.
	thumbnail := g.resize(src, width, height)

	// 保存缩略图.
	outputPath, err := g.saveThumbnail(thumbnail, filePath, width, height, quality)
	if err != nil {
		return nil, err
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    filePath,
		FileType:    FileTypeImage,
		PreviewPath: outputPath,
		ContentType: getContentType(format),
		Width:       width,
		Height:      height,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
	}, nil
}

// generateFromRAW 从 RAW 格式生成缩略图.
func (g *ThumbnailGenerator) generateFromRAW(ctx context.Context, filePath string, width, height, quality int) (*PreviewResult, error) {
	// 首先尝试提取嵌入的缩略图.
	embeddedPath := filePath + ".thumb.jpg"
	cmd := exec.CommandContext(ctx, g.rawConverter, "-e", "-o", embeddedPath, filePath)
	if err := cmd.Run(); err == nil {
		if _, statErr := os.Stat(embeddedPath); statErr == nil {
			// 成功提取嵌入缩略图，进行缩放.
			result, genErr := g.generateFromStandard(ctx, embeddedPath, FormatJPEG, width, height, quality)
			os.Remove(embeddedPath) // 清理临时文件
			if genErr == nil {
				return result, nil
			}
		}
	}

	// 使用 dcraw 转换为 PPM 后处理.
	tmpFile, err := os.CreateTemp("", "raw-*.ppm")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd = exec.CommandContext(ctx, g.rawConverter, "-c", "-w", "-h", filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("RAW 转换失败: %w", err)
	}

	// 解码 PPM.
	src, _, err := image.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("解码 RAW 输出失败: %w", err)
	}

	// 缩放.
	thumbnail := g.resize(src, width, height)

	// 保存.
	outputPath, err := g.saveThumbnail(thumbnail, filePath, width, height, quality)
	if err != nil {
		return nil, err
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    filePath,
		FileType:    FileTypeImage,
		PreviewPath: outputPath,
		ContentType: "image/jpeg",
		Width:       width,
		Height:      height,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
	}, nil
}

// generateFromHEIC 从 HEIC 格式生成缩略图.
func (g *ThumbnailGenerator) generateFromHEIC(ctx context.Context, filePath string, width, height, quality int) (*PreviewResult, error) {
	// 使用 heif-convert 或 ImageMagick.
	tmpFile, err := os.CreateTemp("", "heic-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// 尝试 heif-convert.
	cmd := exec.CommandContext(ctx, "heif-convert", filePath, tmpFile.Name())
	if err := cmd.Run(); err != nil {
		// 回退到 ImageMagick.
		cmd = exec.CommandContext(ctx, "convert", filePath, tmpFile.Name())
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("HEIC 转换失败: %w", err)
		}
	}

	// 从转换后的 JPEG 生成缩略图.
	return g.generateFromStandard(ctx, tmpFile.Name(), FormatJPEG, width, height, quality)
}

// resize 缩放图片，保持宽高比.
func (g *ThumbnailGenerator) resize(src image.Image, maxWidth, maxHeight int) image.Image {
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	// 计算缩放比例.
	ratio := float64(maxWidth) / float64(srcWidth)
	if ratio2 := float64(maxHeight) / float64(srcHeight); ratio2 < ratio {
		ratio = ratio2
	}

	// 如果图片已经足够小，直接返回.
	if ratio >= 1.0 {
		return src
	}

	newWidth := int(float64(srcWidth) * ratio)
	newHeight := int(float64(srcHeight) * ratio)

	// 创建目标图片.
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// 使用高质量缩放算法.
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)

	return dst
}

// saveThumbnail 保存缩略图到文件.
func (g *ThumbnailGenerator) saveThumbnail(img image.Image, sourcePath string, width, height, quality int) (string, error) {
	// 生成输出路径.
	hash := fmt.Sprintf("%s_%dx%d", filepath.Base(sourcePath), width, height)
	ext := ".jpg"
	if quality == 100 {
		ext = ".png"
	}

	outputDir := filepath.Join(g.config.Cache.CacheDir, "thumbnails")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("创建缓存目录失败: %w", err)
	}

	outputPath := filepath.Join(outputDir, hash+ext)

	// 创建输出文件.
	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	// 编码保存.
	if ext == ".png" {
		return outputPath, png.Encode(outFile, img)
	}

	return outputPath, jpeg.Encode(outFile, img, &jpeg.Options{Quality: quality})
}

// getOutputSize 获取输出尺寸.
func (g *ThumbnailGenerator) getOutputSize(req *PreviewRequest) (width, height int) {
	width = req.Width
	height = req.Height

	if width <= 0 && height <= 0 {
		width = g.config.DefaultThumbnailWidth
		height = g.config.DefaultThumbnailHeight
	} else if width <= 0 {
		width = height
	} else if height <= 0 {
		height = width
	}

	return width, height
}

// getContentType 获取 MIME 类型.
func getContentType(format ImageFormat) string {
	switch format {
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	case FormatGIF:
		return "image/gif"
	case FormatWebP:
		return "image/webp"
	case FormatHEIC:
		return "image/heic"
	case FormatBMP:
		return "image/bmp"
	case FormatTIFF:
		return "image/tiff"
	default:
		return "image/jpeg"
	}
}

// hasAlphaChannel 检查色彩模型是否包含透明通道.
func hasAlphaChannel(model color.Model) bool {
	// 检查常见带透明度的色彩模型.
	switch model {
	case color.RGBAModel, color.NRGBAModel, color.AlphaModel, color.Alpha16Model,
		color.NRGBA64Model, color.RGBA64Model:
		return true
	default:
		return false
	}
}

// SetRawConverter 设置 RAW 转换器路径.
func (g *ThumbnailGenerator) SetRawConverter(path string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rawConverter = path
}

// IsRawConverterAvailable 检查 RAW 转换器是否可用.
func (g *ThumbnailGenerator) IsRawConverterAvailable() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, err := exec.LookPath(g.rawConverter)
	return err == nil
}
