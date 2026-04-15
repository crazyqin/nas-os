package sync

import (
	"hash/fnv"
	"os"
	"path/filepath"

	"github.com/cespare/xxhash/v2"
)

const (
	// DefaultChunkSize 块大小（512KB）.
	DefaultChunkSize int64 = 512 * 1024
	// ChunkSizeLarge 大块大小（4MB，用于大文件）.
	ChunkSizeLarge int64 = 4 * 1024 * 1024
	// LargeFileThreshold 大文件阈值（50MB）.
	LargeFileThreshold int64 = 50 * 1024 * 1024
)

// Checksummer 计算文件校验和.
type Checksummer struct {
	chunkSize int64
}

// NewChecksummer 创建校验和计算器.
func NewChecksummer() *Checksummer {
	return &Checksummer{chunkSize: DefaultChunkSize}
}

// FileChecksum 计算整个文件的 xxhash64（快速，非加密）.
func (c *Checksummer) FileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := xxhash.New()
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return checksumHex(h.Sum(nil)), nil
}

// FileChunks 将文件分块并返回每块校验和，用于 delta-sync 比较.
func (c *Checksummer) FileChunks(path string) ([]ChunkInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// 选择块大小
	chunkSize := c.chunkSize
	if info.Size() > LargeFileThreshold {
		chunkSize = ChunkSizeLarge
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var chunks []ChunkInfo
	buf := make([]byte, chunkSize)
	var offset int64

	for {
		n, err := f.Read(buf)
		if n > 0 {
			h := xxhash.New()
			_, _ = h.Write(buf[:n])
			chunks = append(chunks, ChunkInfo{
				Index:  int64(len(chunks)),
				Hash:   checksumHex(h.Sum(nil)),
				Offset: offset,
				Length: int64(n),
			})
			offset += int64(n)
		}
		if err != nil {
			break
		}
	}

	// 空文件至少返回一个块
	if len(chunks) == 0 {
		chunks = append(chunks, ChunkInfo{
			Index:  0,
			Hash:   checksumHex([]byte{0}),
			Offset: 0,
			Length: 0,
		})
	}

	return chunks, nil
}

// DiffChunks 比较两份 chunk 列表，返回需要在 source 端更新的块索引.
// 典型用法：远程有 chunksRemote，本地有 chunksLocal，
// 返回本地有但远程不同的块的 index 列表，即需要传输的块.
func DiffChunks(source, target []ChunkInfo) []int64 {
	sourceMap := make(map[int64]ChunkInfo, len(source))
	for _, c := range source {
		sourceMap[c.Index] = c
	}

	var missing []int64
	for _, sc := range source {
		if sc.Index >= int64(len(target)) || target[sc.Index].Hash != sc.Hash {
			missing = append(missing, sc.Index)
		}
	}
	return missing
}

// QuickHash 极速文件指纹（仅取 size + modtime + 首块 hash），用于快速比较.
func QuickHash(path string, info os.FileInfo) (uint64, error) {
	h := fnv.New64a()
	// 写入 size
	_, _ = h.Write([]byte{
		byte(info.Size()),
		byte(info.Size() >> 8),
		byte(info.Size() >> 16),
		byte(info.Size() >> 24),
		byte(info.Size() >> 32),
		byte(info.Size() >> 40),
		byte(info.Size() >> 48),
		byte(info.Size() >> 56),
	})
	// 写入 modtime unix
	mt := info.ModTime().UnixNano()
	_, _ = h.Write([]byte{
		byte(mt),
		byte(mt >> 8),
		byte(mt >> 16),
		byte(mt >> 24),
		byte(mt >> 32),
		byte(mt >> 40),
		byte(mt >> 48),
		byte(mt >> 56),
	})

	// 首块 hash
	if info.Size() > 0 {
		f, err := os.Open(path)
		if err != nil {
			return h.Sum64(), nil
		}
		defer f.Close()
		buf := make([]byte, 4096)
		n, _ := f.Read(buf)
		if n > 0 {
			xh := xxhash.New()
			_, _ = xh.Write(buf[:n])
			fh := xh.Sum64()
			_, _ = h.Write([]byte{
				byte(fh), byte(fh >> 8), byte(fh >> 16), byte(fh >> 24),
				byte(fh >> 32), byte(fh >> 40), byte(fh >> 48), byte(fh >> 56),
			})
		}
	}
	return h.Sum64(), nil
}

// checksumHex 将 xxhash 的字节输出转为十六进制字符串.
func checksumHex(b []byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexDigits[v>>4]
		out[i*2+1] = hexDigits[v&0x0f]
	}
	return string(out)
}

// ShouldSkipFile 判断文件是否应该跳过（临时文件、系统文件等）.
func ShouldSkipFile(name string) bool {
	// 跳过隐藏文件（.开头）
	if len(name) > 0 && name[0] == '.' {
		// 保留 .xxx 但跳过 .~tmp、.DS_Store 等
		if name == ".DS_Store" || name == ".localized" {
			return true
		}
	}
	// 跳过临时文件
	if filepath.Ext(name) == ".tmp" || filepath.Ext(name) == ".temp" {
		return true
	}
	// 跳过 Office 临时锁定文件
	if len(name) > 0 && name[0] == '~' && name[1] == '$' {
		return true
	}
	return false
}
