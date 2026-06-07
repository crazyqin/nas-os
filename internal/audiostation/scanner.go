// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scanner 音乐文件扫描器.
type Scanner struct {
	manager *Manager
}

// NewScanner 创建扫描器.
func NewScanner(mgr *Manager) *Scanner {
	return &Scanner{manager: mgr}
}

// Scan 扫描指定路径的音乐文件.
func (s *Scanner) Scan(paths []string, recursive, force bool) {
	s.manager.mu.Lock()
	s.manager.scanStatus = &ScanStatus{
		IsRunning: true,
		StartedAt: time.Now(),
	}
	s.manager.mu.Unlock()

	defer func() {
		s.manager.mu.Lock()
		now := time.Now()
		s.manager.scanStatus.IsRunning = false
		s.manager.scanStatus.CompletedAt = &now
		if s.manager.scanStatus.TotalFiles > 0 {
			s.manager.scanStatus.Progress = 100.0
		}
		s.manager.mu.Unlock()
		_ = s.manager.saveConfig()
	}()

	// 收集所有音乐文件
	var files []string
	for _, path := range paths {
		collected := s.collectFiles(path, recursive)
		files = append(files, collected...)
	}

	s.manager.mu.Lock()
	s.manager.scanStatus.TotalFiles = len(files)
	s.manager.mu.Unlock()

	// 扫描每个文件
	for i, filePath := range files {
		s.scanFile(filePath, force)

		s.manager.mu.Lock()
		s.manager.scanStatus.ScannedFiles = i + 1
		if s.manager.scanStatus.TotalFiles > 0 {
			s.manager.scanStatus.Progress = float64(i+1) / float64(s.manager.scanStatus.TotalFiles) * 100
		}
		s.manager.mu.Unlock()
	}
}

// collectFiles 收集目录下的音乐文件.
func (s *Scanner) collectFiles(dirPath string, recursive bool) []string {
	var files []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())
		if entry.IsDir() {
			if recursive {
				files = append(files, s.collectFiles(fullPath, true)...)
			}
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if isSupportedFormat(ext) {
			files = append(files, fullPath)
		}
	}

	return files
}

// scanFile 扫描单个音乐文件.
func (s *Scanner) scanFile(filePath string, force bool) {
	// 检查是否已存在
	s.manager.mu.RLock()
	existing := false
	for _, t := range s.manager.tracks {
		if t.FilePath == filePath {
			existing = true
			break
		}
	}
	s.manager.mu.RUnlock()

	if existing && !force {
		return
	}

	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		s.manager.mu.Lock()
		s.manager.scanStatus.ErrorFiles++
		s.manager.mu.Unlock()
		return
	}

	// 提取元数据
	metadata := s.extractMetadata(filePath)

	// 生成确定性ID
	trackID := generateID("track", filePath)

	s.manager.mu.Lock()
	if existing {
		// 更新已有曲目
		for _, t := range s.manager.tracks {
			if t.FilePath == filePath {
				t.Title = metadata.Title
				t.Artist = metadata.Artist
				t.Album = metadata.Album
				t.AlbumArtist = metadata.AlbumArtist
				t.Genre = metadata.Genre
				t.Year = metadata.Year
				t.TrackNum = metadata.TrackNum
				t.DiscNum = metadata.DiscNum
				t.Duration = metadata.Duration
				t.Bitrate = metadata.Bitrate
				t.SampleRate = metadata.SampleRate
				t.Channels = metadata.Channels
				t.FileSize = fileInfo.Size()
				t.UpdatedAt = time.Now()
				break
			}
		}
	} else {
		// 创建新曲目
		track := &Track{
			ID:          trackID,
			Title:       metadata.Title,
			Artist:      metadata.Artist,
			Album:       metadata.Album,
			AlbumArtist: metadata.AlbumArtist,
			Genre:       metadata.Genre,
			Year:        metadata.Year,
			TrackNum:    metadata.TrackNum,
			DiscNum:     metadata.DiscNum,
			Duration:    metadata.Duration,
			Bitrate:     metadata.Bitrate,
			SampleRate:  metadata.SampleRate,
			Channels:    metadata.Channels,
			Format:      AudioFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), ".")),
			FileSize:    fileInfo.Size(),
			FilePath:    filePath,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		s.manager.addTrackToIndex(track)
		s.manager.scanStatus.NewFiles++
	}
	s.manager.mu.Unlock()
}

// TrackMetadata 曲目元数据.
type TrackMetadata struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	Year        int
	TrackNum    int
	DiscNum     int
	Duration    int // 秒
	Bitrate     int // kbps
	SampleRate  int // Hz
	Channels    int
}

// extractMetadata 提取音频文件元数据.
func (s *Scanner) extractMetadata(filePath string) *TrackMetadata {
	ext := strings.ToLower(filepath.Ext(filePath))
	meta := &TrackMetadata{
		Title:  strings.TrimSuffix(filepath.Base(filePath), ext),
		Artist: "未知艺术家",
		Album:  "未知专辑",
	}

	switch ext {
	case ".mp3":
		s.extractMP3Metadata(filePath, meta)
	case ".flac":
		s.extractFLACMetadata(filePath, meta)
	default:
		// 其他格式使用基本文件信息
		s.extractBasicMetadata(filePath, meta)
	}

	// 确保标题不为空
	if meta.Title == "" {
		meta.Title = strings.TrimSuffix(filepath.Base(filePath), ext)
	}

	return meta
}

// extractMP3Metadata 提取 MP3 ID3 标签.
func (s *Scanner) extractMP3Metadata(filePath string, meta *TrackMetadata) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	// 读取 ID3v2 头部
	header := make([]byte, 10)
	if _, err := io.ReadFull(file, header); err != nil {
		return
	}

	// 检查 ID3v2 标识
	if string(header[:3]) != "ID3" {
		// 尝试从文件名提取信息
		s.parseFilename(filePath, meta)
		return
	}

	// 解析 ID3v2 标签大小
	size := decodeSyncsafe(header[6:10])
	if size == 0 || size > 10*1024*1024 { // 限制 10MB
		return
	}

	// 读取标签数据
	tagData := make([]byte, size)
	if _, err := io.ReadFull(file, tagData); err != nil {
		return
	}

	// 解析帧
	s.parseID3v2Frames(tagData, header[3], meta)

	// 获取文件大小估算时长（简化处理）
	if fileInfo, err := os.Stat(filePath); err == nil && meta.Duration == 0 {
		// 估算时长：文件大小 / 码率
		if meta.Bitrate > 0 {
			meta.Duration = int(fileInfo.Size()) / (meta.Bitrate * 128) // 粗略估算
		}
	}
}

// extractFLACMetadata 提取 FLAC 元数据.
func (s *Scanner) extractFLACMetadata(filePath string, meta *TrackMetadata) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	// 检查 FLAC 标识
	marker := make([]byte, 4)
	if _, err := io.ReadFull(file, marker); err != nil {
		return
	}
	if string(marker) != "fLaC" {
		s.parseFilename(filePath, meta)
		return
	}

	// 读取元数据块
	for {
		blockHeader := make([]byte, 4)
		if _, err := io.ReadFull(file, blockHeader); err != nil {
			break
		}

		blockType := blockHeader[0] & 0x7F
		isLast := (blockHeader[0] & 0x80) != 0
		blockSize := int(binary.BigEndian.Uint32(append([]byte{0}, blockHeader[1:]...)))

		if blockSize <= 0 || blockSize > 10*1024*1024 {
			break
		}

		blockData := make([]byte, blockSize)
		if _, err := io.ReadFull(file, blockData); err != nil {
			break
		}

		// STREAMINFO 块
		if blockType == 0 && len(blockData) >= 18 {
			sampleRate := int(binary.BigEndian.Uint32(append([]byte{0}, blockData[10:13]...))) >> 4
			meta.SampleRate = sampleRate
			meta.Channels = int((blockData[12]>>1)&0x07) + 1

			totalSamples := int64(binary.BigEndian.Uint64(blockData[14:22])) >> 4 // 36位
			if sampleRate > 0 {
				meta.Duration = int(totalSamples) / sampleRate
			}
		}

		// VORBIS_COMMENT 块
		if blockType == 4 {
			s.parseVorbisComment(blockData, meta)
		}

		if isLast {
			break
		}
	}
}

// extractBasicMetadata 提取基本元数据.
func (s *Scanner) extractBasicMetadata(filePath string, meta *TrackMetadata) {
	s.parseFilename(filePath, meta)
}

// parseFilename 从文件名推断信息.
func (s *Scanner) parseFilename(filePath string, meta *TrackMetadata) {
	name := filepath.Base(filePath)
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)

	// 尝试解析 "艺术家 - 标题" 格式
	if parts := strings.SplitN(name, " - ", 2); len(parts) == 2 {
		meta.Artist = strings.TrimSpace(parts[0])
		meta.Title = strings.TrimSpace(parts[1])
	} else {
		meta.Title = name
	}
}

// parseID3v2Frames 解析 ID3v2 帧.
func (s *Scanner) parseID3v2Frames(data []byte, version byte, meta *TrackMetadata) {
	offset := 0
	frameHeaderSize := 10
	if version < 3 {
		frameHeaderSize = 6
	}

	for offset+frameHeaderSize <= len(data) {
		var frameID string
		var frameSize int

		if version >= 3 {
			frameID = string(data[offset : offset+4])
			frameSize = int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		} else {
			frameID = string(data[offset : offset+3])
			frameSize = int(data[offset+3])<<16 | int(data[offset+4])<<8 | int(data[offset+5])
		}

		if frameSize <= 0 || offset+frameHeaderSize+frameSize > len(data) {
			break
		}

		frameData := data[offset+frameHeaderSize : offset+frameHeaderSize+frameSize]

		// 解析文本帧
		if len(frameData) > 1 {
			encoding := frameData[0]
			text := decodeText(frameData[1:], encoding)

			switch frameID {
			case "TIT2", "TT2":
				meta.Title = text
			case "TPE1", "TP1":
				meta.Artist = text
			case "TALB", "TAL":
				meta.Album = text
			case "TPE2", "TP2":
				meta.AlbumArtist = text
			case "TCON", "TCO":
				meta.Genre = text
			case "TYER", "TDRC", "TDOR":
				if year, err := parseYear(text); err == nil {
					meta.Year = year
				}
			case "TRCK", "TRK":
				if num, err := parseTrackNum(text); err == nil {
					meta.TrackNum = num
				}
			case "TPOS", "TPA":
				if num, err := parseTrackNum(text); err == nil {
					meta.DiscNum = num
				}
			}
		}

		offset += frameHeaderSize + frameSize
	}
}

// parseVorbisComment 解析 Vorbis 注释.
func (s *Scanner) parseVorbisComment(data []byte, meta *TrackMetadata) {
	if len(data) < 8 {
		return
	}

	offset := 0

	// 读取 vendor length
	vendorLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4 + vendorLen

	if offset+4 > len(data) {
		return
	}

	// 读取评论数量
	commentCount := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	for i := 0; i < commentCount && offset+4 <= len(data); i++ {
		commentLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+commentLen > len(data) {
			break
		}

		comment := string(data[offset : offset+commentLen])
		offset += commentLen

		if parts := strings.SplitN(comment, "=", 2); len(parts) == 2 {
			key := strings.ToUpper(parts[0])
			value := parts[1]

			switch key {
			case "TITLE":
				meta.Title = value
			case "ARTIST":
				meta.Artist = value
			case "ALBUM":
				meta.Album = value
			case "ALBUMARTIST":
				meta.AlbumArtist = value
			case "GENRE":
				meta.Genre = value
			case "DATE", "YEAR":
				if year, err := parseYear(value); err == nil {
					meta.Year = year
				}
			case "TRACKNUMBER":
				if num, err := parseTrackNum(value); err == nil {
					meta.TrackNum = num
				}
			case "DISCNUMBER":
				if num, err := parseTrackNum(value); err == nil {
					meta.DiscNum = num
				}
			}
		}
	}
}

// ========== 辅助函数 ==========

// isSupportedFormat 检查是否为支持的音频格式.
func isSupportedFormat(ext string) bool {
	format := AudioFormat(strings.TrimPrefix(ext, "."))
	for _, f := range SupportedFormats {
		if f == format {
			return true
		}
	}
	return false
}

// decodeSyncsafe 解码同步安全整数.
func decodeSyncsafe(data []byte) int {
	if len(data) < 4 {
		return 0
	}
	return int(data[0])<<21 | int(data[1])<<14 | int(data[2])<<7 | int(data[3])
}

// decodeText 解码文本帧.
func decodeText(data []byte, encoding byte) string {
	switch encoding {
	case 0: // ISO-8859-1
		return string(data)
	case 1: // UTF-16 with BOM
		return decodeUTF16(data)
	case 2: // UTF-16BE without BOM
		return decodeUTF16BE(data)
	case 3: // UTF-8
		return string(data)
	default:
		return string(data)
	}
}

// decodeUTF16 解码 UTF-16 文本.
func decodeUTF16(data []byte) string {
	if len(data) < 2 {
		return string(data)
	}

	// 检查 BOM
	if data[0] == 0xFF && data[1] == 0xFE {
		return decodeUTF16LE(data[2:])
	} else if data[0] == 0xFE && data[1] == 0xFF {
		return decodeUTF16BE(data[2:])
	}

	// 默认 UTF-16LE
	return decodeUTF16LE(data)
}

// decodeUTF16LE 解码 UTF-16LE.
func decodeUTF16LE(data []byte) string {
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		r := rune(binary.LittleEndian.Uint16(data[i : i+2]))
		runes = append(runes, r)
	}
	return string(runes)
}

// decodeUTF16BE 解码 UTF-16BE.
func decodeUTF16BE(data []byte) string {
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		r := rune(binary.BigEndian.Uint16(data[i : i+2]))
		runes = append(runes, r)
	}
	return string(runes)
}

// parseYear 解析年份.
func parseYear(s string) (int, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 4 {
		var year int
		_, err := fmt.Sscanf(s[:4], "%d", &year)
		return year, err
	}
	return 0, fmt.Errorf("invalid year: %s", s)
}

// parseTrackNum 解析曲目号.
func parseTrackNum(s string) (int, error) {
	s = strings.TrimSpace(s)
	// 处理 "1/12" 格式
	if parts := strings.SplitN(s, "/", 2); len(parts) >= 1 {
		var num int
		_, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &num)
		return num, err
	}
	return 0, fmt.Errorf("invalid track number: %s", s)
}
