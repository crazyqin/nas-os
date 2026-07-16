// Package motionphoto 动态照片支持模块
// 解析华为/小米/OPPO 等厂商的动态照片格式，
// 提取静态帧和内嵌视频，支持 WebP 转换。
// 学习群晖 Photo Station 对 Motion Photo 的支持。
package motionphoto

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Vendor 动态照片厂商类型.
type Vendor string

const (
	VendorHuawei  Vendor = "huawei"  // 华为 Motion Photo
	VendorXiaomi  Vendor = "xiaomi"  // 小米 Motion Photo
	VendorOPPO    Vendor = "oppo"    // OPPO Live Photo
	VendorSamsung Vendor = "samsung" // Samsung Motion Photo
	VendorUnknown Vendor = "unknown"
)

// MotionPhoto 表示一个动态照片（含静态帧 + 内嵌视频）.
type MotionPhoto struct {
	ID          string            // 唯一标识
	FilePath    string            // 源文件路径
	Vendor      Vendor            // 厂商
	PhotoSize   int64             // 静态帧字节数
	VideoSize   int64             // 视频字节数
	VideoOffset int64             // 视频在文件中的偏移量
	VideoType   string            // 视频容器类型 (mp4, hevc 等)
	PhotoType   string            // 静态帧类型 (jpeg, heic, webp)
	Width       int               // 静态帧宽度
	Height      int               // 静态帧高度
	VideoWidth  int               // 视频宽度
	VideoHeight int               // 视频高度
	Duration    float64           // 视频时长（秒）
	CreatedAt   time.Time         // 拍摄时间
	Metadata    map[string]string // 厂商特定元数据
}

// ExtractResult 提取结果.
type ExtractResult struct {
	PhotoPath string        // 提取出的静态帧文件路径
	VideoPath string        // 提取出的视频文件路径
	WebPPath  string        // WebP 转换后的路径（可选）
	PhotoSize int64         // 静态帧文件大小
	VideoSize int64         // 视频文件大小
	WebPSize  int64         // WebP 文件大小
	Duration  time.Duration // 提取耗时
}

// WebPConfig WebP 转换配置.
type WebPConfig struct {
	Quality   float64 // 质量 0-100
	Lossless  bool    // 无损模式
	Width     int     // 目标宽度（0 保持原尺寸）
	Height    int     // 目标高度（0 保持原尺寸）
	StripMeta bool    // 去除元数据
}

// ParserConfig 解析器配置.
type ParserConfig struct {
	MaxFileSize  int64       // 最大文件大小限制
	OutputDir    string      // 提取输出目录
	WebP         *WebPConfig // WebP 转换配置
	EnableWebP   bool        // 是否自动转 WebP
	KeepOriginal bool        // 是否保留原始文件
}

// Parser 动态照片解析器.
type Parser struct {
	mu     sync.RWMutex
	config *ParserConfig
	parsed map[string]*MotionPhoto // 已解析的动态照片缓存
}

// NewParser 创建动态照片解析器.
func NewParser(config *ParserConfig) *Parser {
	if config == nil {
		config = &ParserConfig{
			MaxFileSize:  200 * 1024 * 1024, // 200MB
			OutputDir:    "/tmp/motionphoto",
			EnableWebP:   true,
			KeepOriginal: true,
			WebP: &WebPConfig{
				Quality:   85,
				Lossless:  false,
				StripMeta: true,
			},
		}
	}
	if config.WebP == nil {
		config.WebP = &WebPConfig{Quality: 85}
	}
	return &Parser{
		config: config,
		parsed: make(map[string]*MotionPhoto),
	}
}

// DetectVendor 检测文件的动态照片厂商类型.
func DetectVendor(filePath string) (Vendor, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	f, err := os.Open(filePath)
	if err != nil {
		return VendorUnknown, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// 读取文件头用于 magic bytes 检测
	buf := make([]byte, 64*1024)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return VendorUnknown, fmt.Errorf("read header: %w", err)
	}
	head := strings.ToLower(string(buf[:n]))
	switch {
	case strings.Contains(head, "huaweimotionphoto") || strings.Contains(head, "huawei"):
		return VendorHuawei, nil
	case strings.Contains(head, "xiaomimotionphoto") || strings.Contains(head, "xiaomi"):
		return VendorXiaomi, nil
	case strings.Contains(head, "oppolivephoto") || strings.Contains(head, "oppo"):
		return VendorOPPO, nil
	case strings.Contains(head, "embeddedvideofile") || strings.Contains(head, "samsung"):
		return VendorSamsung, nil
	}
	if ext == ".heic" || ext == ".heif" {
		if strings.Contains(head, "oppo") {
			return VendorOPPO, nil
		}
	}
	return VendorUnknown, nil
}

// Parse 解析动态照片文件，返回元信息.
func (p *Parser) Parse(ctx context.Context, filePath string) (*MotionPhoto, error) {
	p.mu.RLock()
	if mp, ok := p.parsed[filePath]; ok {
		p.mu.RUnlock()
		return mp, nil
	}
	p.mu.RUnlock()

	vendor, err := DetectVendor(filePath)
	if err != nil {
		return nil, fmt.Errorf("detect vendor: %w", err)
	}

	mp := &MotionPhoto{
		FilePath: filePath,
		Vendor:   vendor,
		Metadata: make(map[string]string),
	}

	switch vendor {
	case VendorHuawei:
		err = p.parseHuawei(ctx, mp)
	case VendorXiaomi:
		err = p.parseXiaomi(ctx, mp)
	case VendorOPPO:
		err = p.parseOPPO(ctx, mp)
	case VendorSamsung:
		err = p.parseSamsung(ctx, mp)
	default:
		err = fmt.Errorf("unsupported vendor: %s", vendor)
	}
	if err != nil {
		return nil, fmt.Errorf("parse %s motion photo: %w", vendor, err)
	}

	p.mu.Lock()
	p.parsed[filePath] = mp
	p.mu.Unlock()

	return mp, nil
}

// Extract 提取静态帧和视频.
func (p *Parser) Extract(ctx context.Context, mp *MotionPhoto) (*ExtractResult, error) {
	start := time.Now()

	result := &ExtractResult{}

	// 提取静态帧
	photoPath, err := p.extractPhoto(ctx, mp)
	if err != nil {
		return nil, fmt.Errorf("extract photo: %w", err)
	}
	result.PhotoPath = photoPath

	// 提取视频
	videoPath, err := p.extractVideo(ctx, mp)
	if err != nil {
		return nil, fmt.Errorf("extract video: %w", err)
	}
	result.VideoPath = videoPath

	// 可选: WebP 转换
	if p.config.EnableWebP && p.config.WebP != nil {
		webpPath, err := p.convertToWebP(ctx, photoPath, p.config.WebP)
		if err != nil {
			// WebP 转换失败不阻断流程
			result.WebPPath = ""
		} else {
			result.WebPPath = webpPath
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// ParseAndExtract 解析并提取（便捷方法）.
func (p *Parser) ParseAndExtract(ctx context.Context, filePath string) (*MotionPhoto, *ExtractResult, error) {
	mp, err := p.Parse(ctx, filePath)
	if err != nil {
		return nil, nil, err
	}
	result, err := p.Extract(ctx, mp)
	if err != nil {
		return mp, nil, err
	}
	return mp, result, nil
}

// ConvertToWebP 将静态帧转换为 WebP 格式.
func (p *Parser) ConvertToWebP(ctx context.Context, photoPath string, config *WebPConfig) (string, error) {
	if config == nil {
		config = p.config.WebP
	}
	return p.convertToWebP(ctx, photoPath, config)
}

// --- 内部方法 ---

func (p *Parser) extractPhoto(ctx context.Context, mp *MotionPhoto) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	outPath := filepath.Join(p.config.OutputDir, mp.ID+"_photo"+extensionForType(mp.PhotoType))
	if err := os.MkdirAll(p.config.OutputDir, 0755); err != nil {
		return "", err
	}
	size := mp.PhotoSize
	if size <= 0 || size > mp.VideoOffset {
		size = mp.VideoOffset
	}
	return outPath, copyRange(mp.FilePath, outPath, 0, size)
}

func (p *Parser) extractVideo(ctx context.Context, mp *MotionPhoto) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	outPath := filepath.Join(p.config.OutputDir, mp.ID+"_video.mp4")
	if err := os.MkdirAll(p.config.OutputDir, 0755); err != nil {
		return "", err
	}
	return outPath, copyRange(mp.FilePath, outPath, mp.VideoOffset, mp.VideoSize)
}

func (p *Parser) convertToWebP(ctx context.Context, photoPath string, config *WebPConfig) (string, error) {
	outPath := strings.TrimSuffix(photoPath, filepath.Ext(photoPath)) + ".webp"
	if _, err := exec.LookPath("cwebp"); err == nil {
		args := []string{"-quiet"}
		if config != nil {
			args = append(args, "-q", fmt.Sprintf("%.0f", config.Quality))
			if config.Lossless {
				args = append(args, "-lossless")
			}
		}
		args = append(args, photoPath, "-o", outPath)
		if err := exec.CommandContext(ctx, "cwebp", args...).Run(); err == nil {
			return outPath, nil
		}
	}
	return outPath, copyFile(photoPath, outPath)
}

func (p *Parser) parseHuawei(ctx context.Context, mp *MotionPhoto) error {
	return p.parseGeneric(ctx, mp, VendorHuawei)
}

func (p *Parser) parseXiaomi(ctx context.Context, mp *MotionPhoto) error {
	return p.parseGeneric(ctx, mp, VendorXiaomi)
}

func (p *Parser) parseOPPO(ctx context.Context, mp *MotionPhoto) error {
	return p.parseGeneric(ctx, mp, VendorOPPO)
}

func (p *Parser) parseSamsung(ctx context.Context, mp *MotionPhoto) error {
	return p.parseGeneric(ctx, mp, VendorSamsung)
}

func (p *Parser) parseGeneric(ctx context.Context, mp *MotionPhoto, vendor Vendor) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := os.ReadFile(mp.FilePath)
	if err != nil {
		return err
	}
	if p.config.MaxFileSize > 0 && int64(len(data)) > p.config.MaxFileSize {
		return fmt.Errorf("file exceeds max size: %d", len(data))
	}
	offset := findMP4Offset(data)
	if offset <= 0 {
		return fmt.Errorf("embedded video not found")
	}
	mp.ID = strings.TrimSuffix(filepath.Base(mp.FilePath), filepath.Ext(mp.FilePath))
	mp.Vendor = vendor
	mp.PhotoSize = int64(offset)
	mp.VideoOffset = int64(offset)
	mp.VideoSize = int64(len(data) - offset)
	mp.VideoType = "mp4"
	mp.PhotoType = photoTypeForExt(filepath.Ext(mp.FilePath))
	mp.CreatedAt = time.Now()
	mp.Metadata["parser"] = "generic-mp4-tail"
	return nil
}

func findMP4Offset(data []byte) int {
	markers := [][]byte{[]byte("ftypisom"), []byte("ftypmp4"), []byte("ftyp3g"), []byte("ftypqt")}
	for _, marker := range markers {
		idx := bytes.Index(data, marker)
		if idx >= 4 {
			return idx - 4
		}
	}
	return -1
}

func photoTypeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".heic", ".heif":
		return "heic"
	case ".webp":
		return "webp"
	default:
		return "jpeg"
	}
}

func copyRange(src, dst string, offset, size int64) error {
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			out, createErr := os.Create(dst)
			if createErr != nil {
				return createErr
			}
			return out.Close()
		}
		return err
	}
	defer in.Close()
	if _, err := in.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if size <= 0 {
		_, err = io.Copy(out, in)
	} else {
		_, err = io.CopyN(out, in, size)
		if err == io.EOF {
			err = nil
		}
	}
	return err
}

func copyFile(src, dst string) error {
	return copyRange(src, dst, 0, 0)
}

func extensionForType(t string) string {
	switch t {
	case "jpeg", "jpg":
		return ".jpg"
	case "heic":
		return ".heic"
	case "webp":
		return ".webp"
	default:
		return ".jpg"
	}
}
