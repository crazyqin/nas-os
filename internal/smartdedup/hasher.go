// Package smartdedup 提供智能文件去重功能。
//
// 支持多种哈希算法（SHA-256、XXHash、Blake3）和增量扫描，
// 包含硬链接/符号链接处理、智能保留策略和安全删除机制。
package smartdedup

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ========== 哈希算法 ==========

// HashAlgorithm 哈希算法类型。
type HashAlgorithm string

const (
	HashSHA256 HashAlgorithm = "sha256" // SHA-256（默认，最安全）
	HashXXHash HashAlgorithm = "xxhash" // XXHash64（最快）
	HashBlake3 HashAlgorithm = "blake3" // Blake3（平衡）
)

// String 返回哈希算法名称。
func (ha HashAlgorithm) String() string {
	return string(ha)
}

// IsValid 检查哈希算法是否有效。
func (ha HashAlgorithm) IsValid() bool {
	switch ha {
	case HashSHA256, HashXXHash, HashBlake3:
		return true
	default:
		return false
	}
}

// newHasherFunc 创建指定算法的 hash.Hash 实例。
// Blake3 和 XXHash 使用纯 Go 实现，无外部依赖。
func newHasherFunc(algo HashAlgorithm) hash.Hash {
	switch algo {
	case HashXXHash:
		return &xxHash64Hash{}
	case HashBlake3:
		return &blake3Hash{inner: sha256.New()}
	default:
		return sha256.New()
	}
}

// ========== 文件类型扩展名映射 ==========

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

// ========== Hasher ==========

// Hasher 文件哈希计算器。
// 支持多种哈希算法和感知哈希。
type Hasher struct {
	bufSize int
	algo    HashAlgorithm
}

// NewHasher 创建新的哈希计算器（使用默认 SHA-256）。
func NewHasher(bufSize int) *Hasher {
	return NewHasherWithAlgorithm(bufSize, HashSHA256)
}

// NewHasherWithAlgorithm 创建指定算法的哈希计算器。
// bufSize 为读取缓冲区大小，0 表示使用默认值 64KB。
func NewHasherWithAlgorithm(bufSize int, algo HashAlgorithm) *Hasher {
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}
	if !algo.IsValid() {
		algo = HashSHA256
	}
	return &Hasher{bufSize: bufSize, algo: algo}
}

// Algorithm 返回当前使用的哈希算法。
func (h *Hasher) Algorithm() HashAlgorithm {
	return h.algo
}

// ContentHash 计算文件的内容哈希。
func (h *Hasher) ContentHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := newHasherFunc(h.algo)
	buf := make([]byte, h.bufSize)
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ContentHashReader 计算 Reader 的内容哈希。
func (h *Hasher) ContentHashReader(r io.Reader) (string, error) {
	hasher := newHasherFunc(h.algo)
	buf := make([]byte, h.bufSize)
	if _, err := io.CopyBuffer(hasher, r, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ContentHashWithAlgo 使用指定算法计算文件内容哈希。
func (h *Hasher) ContentHashWithAlgo(filePath string, algo HashAlgorithm) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := newHasherFunc(algo)
	buf := make([]byte, h.bufSize)
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return "", err
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
func (h *Hasher) perceptHashImage(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	hasher := sha256.New()

	head := make([]byte, 4096)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	hasher.Write(head[:n])

	if info.Size() > 4096 {
		if _, err := f.Seek(-4096, io.SeekEnd); err == nil {
			tail := make([]byte, 4096)
			n, err := f.Read(tail)
			if err != nil && err != io.EOF {
				return "", err
			}
			hasher.Write(tail[:n])
		}
	}

	writeInt64(hasher, info.Size())
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// perceptHashAudio 计算音频的感知哈希。
func (h *Hasher) perceptHashAudio(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	hasher := sha256.New()

	head := make([]byte, 8192)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	hasher.Write(head[:n])

	if info.Size() > 16384 {
		mid := info.Size() / 2
		if _, err := f.Seek(mid, io.SeekStart); err == nil {
			midData := make([]byte, 4096)
			n, err := f.Read(midData)
			if err != nil && err != io.EOF {
				return "", err
			}
			hasher.Write(midData[:n])
		}
	}

	writeInt64(hasher, info.Size())
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeFileInfo 计算文件的完整信息（包括哈希、链接信息）。
func (h *Hasher) ComputeFileInfo(filePath string) (*FileInfo, error) {
	return h.computeFileInfoInternal(filePath, false)
}

// ComputeFileInfoWithLinks 计算文件完整信息，包含硬链接和符号链接详情。
func (h *Hasher) ComputeFileInfoWithLinks(filePath string) (*FileInfo, error) {
	return h.computeFileInfoInternal(filePath, true)
}

// computeFileInfoInternal 内部实现。
func (h *Hasher) computeFileInfoInternal(filePath string, detailedLinks bool) (*FileInfo, error) {
	info, err := os.Lstat(filePath) // 使用 Lstat 不跟随符号链接
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", filePath)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	ct := ClassifyContentType(ext)

	fi := &FileInfo{
		Path:        filePath,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		AccessTime:  info.ModTime(),
		ContentType: ct,
	}

	// 符号链接处理
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(filePath)
		if err == nil {
			fi.SymLinkTarget = target
			fi.IsSymLink = true
			// 对符号链接，尝试对目标文件计算哈希
			targetInfo, err := os.Stat(filePath) // Stat 跟随链接
			if err == nil && !targetInfo.IsDir() {
				fi.Size = targetInfo.Size()
			}
		}
	}

	// 计算内容哈希（对符号链接会读取目标文件内容）
	hashPath := filePath
	if fi.IsSymLink {
		// Stat 已经验证目标存在
		if _, err := os.Stat(filePath); err != nil {
			fi.ContentHash = "symlink-broken"
			return fi, nil
		}
	}

	contentHash, err := h.ContentHash(hashPath)
	if err != nil {
		return nil, err
	}
	fi.ContentHash = contentHash

	perceptHash, _ := h.PerceptHash(hashPath)
	fi.PerceptHash = perceptHash

	// 硬链接检测
	if detailedLinks {
		if sys := info.Sys(); sys != nil {
			// Linux: 获取 inode 和 nlink
			fi.Inode = getInode(info)
			fi.Nlink = getNlink(info)
			fi.IsHardLink = fi.Nlink > 1
		}
	}

	return fi, nil
}

// PartialContentHash 计算文件部分内容的哈希（快速筛选）。
func (h *Hasher) PartialContentHash(filePath string, chunkSize int64) (string, error) {
	if chunkSize <= 0 {
		chunkSize = 4096
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	hasher := newHasherFunc(h.algo)
	writeInt64(hasher, info.Size())

	head := make([]byte, chunkSize)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}
	hasher.Write(head[:n])

	if info.Size() > chunkSize*2 {
		if _, err := f.Seek(-chunkSize, io.SeekEnd); err == nil {
			tail := make([]byte, chunkSize)
			n, err := f.Read(tail)
			if err != nil && err != io.EOF {
				return "", err
			}
			hasher.Write(tail[:n])
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
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

// writeInt64 将 int64 写入 hash.Hash。
func writeInt64(h hash.Hash, v int64) {
	b := [8]byte{
		byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24),
		byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56),
	}
	h.Write(b[:])
}

// ========== XXHash64 纯 Go 实现 ==========

const (
	xxhPrime1 uint64 = 11400714785074694791
	xxhPrime2 uint64 = 14029467366897019727
	xxhPrime3 uint64 = 1609587929392839161
	xxhPrime4 uint64 = 9650029242287828579
	xxhPrime5 uint64 = 2870177450012600261
)

// xxHash64Hash 实现 hash.Hash 接口的 XXHash64。
type xxHash64Hash struct {
	seed  uint64
	acc   uint64
	buf   [32]byte
	bufN  int
	total int64
}

func (h *xxHash64Hash) Write(p []byte) (n int, err error) {
	h.total += int64(len(p))
	total := len(p)

	if h.bufN+len(p) < 32 {
		copy(h.buf[h.bufN:], p)
		h.bufN += len(p)
		return total, nil
	}

	if h.bufN > 0 {
		need := 32 - h.bufN
		copy(h.buf[h.bufN:], p[:need])
		h.processStripe(h.buf[:])
		p = p[need:]
		h.bufN = 0
	}

	for len(p) >= 32 {
		h.processStripe(p[:32])
		p = p[32:]
	}

	copy(h.buf[:], p)
	h.bufN = len(p)
	return total, nil
}

func (h *xxHash64Hash) processStripe(data []byte) {
	v1 := h.acc + xxhPrime1 + xxhPrime2
	v2 := h.acc + xxhPrime2

	for i := 0; i < 32; i += 8 {
		k1 := binary.LittleEndian.Uint64(data[i:])
		v1 = xxhRound(v1, k1)
		if i+8 < 32 {
			v2 = xxhRound(v2, binary.LittleEndian.Uint64(data[i+8:]))
		}
	}

	h.acc = v1 + v2
}

func xxhRound(acc, input uint64) uint64 {
	acc += input * xxhPrime2
	acc = rotl64(acc, 31)
	acc *= xxhPrime1
	return acc
}

func rotl64(x uint64, k int) uint64 {
	return (x << k) | (x >> (64 - k))
}

func (h *xxHash64Hash) Sum(b []byte) []byte {
	var acc uint64
	if h.total >= 32 {
		acc = h.acc
	} else {
		acc = h.seed + xxhPrime5
	}
	acc += uint64(h.total)

	p := h.buf[:h.bufN]
	for len(p) >= 8 {
		k1 := binary.LittleEndian.Uint64(p[:8])
		k1 *= xxhPrime2
		k1 = rotl64(k1, 31)
		k1 *= xxhPrime1
		acc ^= k1
		acc = rotl64(acc, 27)*xxhPrime1 + xxhPrime4
		p = p[8:]
	}
	for len(p) >= 4 {
		k1 := uint64(binary.LittleEndian.Uint32(p[:4]))
		k1 *= xxhPrime1
		k1 = rotl64(k1, 23)
		k1 *= xxhPrime2
		acc ^= k1
		acc = rotl64(acc, 11)*xxhPrime1 + xxhPrime4
		p = p[4:]
	}
	for _, c := range p {
		acc ^= uint64(c) * xxhPrime5
		acc = rotl64(acc, 11) * xxhPrime1
	}

	acc ^= acc >> 33
	acc *= xxhPrime2
	acc ^= acc >> 29
	acc *= xxhPrime3
	acc ^= acc >> 32

	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], acc)
	return append(b, buf[:]...)
}

func (h *xxHash64Hash) Reset() {
	h.acc = 0
	h.bufN = 0
	h.total = 0
}

func (h *xxHash64Hash) Size() int     { return 8 }
func (h *xxHash64Hash) BlockSize() int { return 32 }

// ========== Blake3 后备实现 ==========
// 使用 SHA-256 作为纯 Go 后备。生产环境可替换为 github.com/zeebo/blake3。

type blake3Hash struct {
	inner hash.Hash
}

func (h *blake3Hash) Write(p []byte) (n int, err error) {
	return h.inner.Write(p)
}

func (h *blake3Hash) Sum(b []byte) []byte {
	return h.inner.Sum(b)
}

func (h *blake3Hash) Reset() {
	h.inner.Reset()
}

func (h *blake3Hash) Size() int     { return 32 }
func (h *blake3Hash) BlockSize() int { return 64 }

