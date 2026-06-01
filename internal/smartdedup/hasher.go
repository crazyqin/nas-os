package smartdedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// imageExtensions 图片文件扩展名。
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".tiff": true, ".tif": true, ".webp": true,
	".svg": true, ".ico": true, ".heic": true, ".heif": true,
	".raw": true, ".cr2": true, ".nef": true, ".arw": true,
}

// audioExtensions 音频文件扩展名。
var audioExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".flac": true, ".aac": true,
	".ogg": true, ".wma": true, ".m4a": true, ".opus": true,
	".ape": true, ".alac": true, ".aiff": true,
}

// videoExtensions 视频文件扩展名。
var videoExtensions = map[string]bool{
	".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
	".ts": true, ".rmvb": true, ".rm": true,
}

// documentExtensions 文档文件扩展名。
var documentExtensions = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true,
	".xlsx": true, ".ppt": true, ".pptx": true, ".txt": true,
	".md": true, ".csv": true, ".json": true, ".xml": true,
	".yaml": true, ".yml": true, ".html": true, ".htm": true,
}

// archiveExtensions 压缩文件扩展名。
var archiveExtensions = map[string]bool{
	".zip": true, ".tar": true, ".gz": true, ".bz2": true,
	".xz": true, ".7z": true, ".rar": true,
}

// Hasher 文件哈希计算器。
// 支持内容哈希（SHA-256）和感知哈希（简化版差异哈希）。
type Hasher struct {
	bufSize int
}

// NewHasher 创建新的哈希计算器。
// bufSize 为读取缓冲区大小，0 表示使用默认值 64KB。
func NewHasher(bufSize int) *Hasher {
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}
	return &Hasher{bufSize: bufSize}
}

// ContentHash 计算文件的内容哈希（SHA-256）。
func (h *Hasher) ContentHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer f.Close()

	hasher := sha256.New()
	buf := make([]byte, h.bufSize)
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return "", fmt.Errorf("hash file %s: %w", filePath, err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ContentHashReader 计算 Reader 的内容哈希。
func (h *Hasher) ContentHashReader(r io.Reader) (string, error) {
	hasher := sha256.New()
	buf := make([]byte, h.bufSize)
	if _, err := io.CopyBuffer(hasher, r, buf); err != nil {
		return "", fmt.Errorf("hash reader: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// PerceptHash 计算文件的感知哈希。
// 对图片和音频文件使用基于文件特征的感知哈希，其他类型返回空。
func (h *Hasher) PerceptHash(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	ct := ClassifyContentType(ext)

	switch ct {
	case ContentTypeImage:
		return h.perceptHashImage(filePath)
	case ContentTypeAudio:
		return h.perceptHashAudio(filePath)
	default:
		return "", nil
	}
}

// perceptHashImage 计算图片的感知哈希。
// 使用文件头部、尾部和大小作为特征生成哈希。
// 真实场景应解码图片并缩放到 8x8 灰度图后生成 dHash。
func (h *Hasher) perceptHashImage(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open image %s: %w", filePath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat image %s: %w", filePath, err)
	}

	hasher := sha256.New()

	// 读取头部 4KB
	head := make([]byte, 4096)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read image header: %w", err)
	}
	hasher.Write(head[:n])

	// 读取尾部 4KB
	if info.Size() > 4096 {
		if _, err := f.Seek(-4096, io.SeekEnd); err == nil {
			tail := make([]byte, 4096)
			n, err := f.Read(tail)
			if err != nil && err != io.EOF {
				return "", fmt.Errorf("read image tail: %w", err)
			}
			hasher.Write(tail[:n])
		}
	}

	// 加入文件大小作为特征
	writeInt64(hasher, info.Size())

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// perceptHashAudio 计算音频的感知哈希。
// 读取文件头部和中部数据生成特征哈希。
func (h *Hasher) perceptHashAudio(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open audio %s: %w", filePath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat audio %s: %w", filePath, err)
	}

	hasher := sha256.New()

	// 读取头部 8KB
	head := make([]byte, 8192)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read audio header: %w", err)
	}
	hasher.Write(head[:n])

	// 采样中部数据
	if info.Size() > 16384 {
		mid := info.Size() / 2
		if _, err := f.Seek(mid, io.SeekStart); err == nil {
			midData := make([]byte, 4096)
			n, err := f.Read(midData)
			if err != nil && err != io.EOF {
				return "", fmt.Errorf("read audio mid: %w", err)
			}
			hasher.Write(midData[:n])
		}
	}

	writeInt64(hasher, info.Size())

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeFileInfo 计算文件的完整信息（包括哈希）。
func (h *Hasher) ComputeFileInfo(filePath string) (*FileInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file %s: %w", filePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", filePath)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	ct := ClassifyContentType(ext)

	contentHash, err := h.ContentHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("content hash: %w", err)
	}

	perceptHash, _ := h.PerceptHash(filePath)

	return &FileInfo{
		Path:        filePath,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		AccessTime:  info.ModTime(), // 标准库无直接 atime
		ContentHash: contentHash,
		PerceptHash: perceptHash,
		ContentType: ct,
	}, nil
}

// ClassifyContentType 根据文件扩展名分类内容类型。
func ClassifyContentType(ext string) ContentType {
	ext = strings.ToLower(ext)
	switch {
	case imageExtensions[ext]:
		return ContentTypeImage
	case audioExtensions[ext]:
		return ContentTypeAudio
	case videoExtensions[ext]:
		return ContentTypeVideo
	case documentExtensions[ext]:
		return ContentTypeDocument
	case archiveExtensions[ext]:
		return ContentTypeArchive
	default:
		return ContentTypeBinary
	}
}

// PartialContentHash 计算文件部分内容的哈希（快速筛选）。
func (h *Hasher) PartialContentHash(filePath string, chunkSize int64) (string, error) {
	if chunkSize <= 0 {
		chunkSize = 4096
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file %s: %w", filePath, err)
	}

	hasher := sha256.New()
	writeInt64(hasher, info.Size())

	// 读取头部
	head := make([]byte, chunkSize)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read head: %w", err)
	}
	hasher.Write(head[:n])

	// 读取尾部
	if info.Size() > chunkSize*2 {
		if _, err := f.Seek(-chunkSize, io.SeekEnd); err == nil {
			tail := make([]byte, chunkSize)
			n, err := f.Read(tail)
			if err != nil && err != io.EOF {
				return "", fmt.Errorf("read tail: %w", err)
			}
			hasher.Write(tail[:n])
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// writeInt64 将 int64 写入 hash.Hash。
func writeInt64(h hash.Hash, v int64) {
	b := [8]byte{
		byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24),
		byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56),
	}
	h.Write(b[:])
}
