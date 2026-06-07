// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

// TagUpdateRequest 标签更新请求.
type TagUpdateRequest struct {
	Title       string `json:"title,omitempty"`        // 标题
	Artist      string `json:"artist,omitempty"`       // 艺术家
	Album       string `json:"album,omitempty"`        // 专辑
	AlbumArtist string `json:"album_artist,omitempty"` // 专辑艺术家
	Genre       string `json:"genre,omitempty"`        // 流派
	Year        int    `json:"year,omitempty"`         // 年份
	TrackNum    int    `json:"track_num,omitempty"`    // 曲目号
	DiscNum     int    `json:"disc_num,omitempty"`     // 碟片号
}

// TagEditor 标签编辑器.
type TagEditor struct {
	manager *Manager
}

// NewTagEditor 创建标签编辑器.
func NewTagEditor(mgr *Manager) *TagEditor {
	return &TagEditor{manager: mgr}
}

// UpdateTrackTag 更新曲目标签（内存索引 + 文件元数据）.
func (te *TagEditor) UpdateTrackTag(trackID string, req TagUpdateRequest) (*Track, error) {
	// 获取曲目
	te.manager.mu.Lock()
	defer te.manager.mu.Unlock()

	track, exists := te.manager.tracks[trackID]
	if !exists {
		return nil, ErrTrackNotFound
	}

	// 更新内存中的标签
	if req.Title != "" {
		track.Title = req.Title
	}
	if req.Artist != "" {
		track.Artist = req.Artist
	}
	if req.Album != "" {
		track.Album = req.Album
	}
	if req.AlbumArtist != "" {
		track.AlbumArtist = req.AlbumArtist
	}
	if req.Genre != "" {
		track.Genre = req.Genre
	}
	if req.Year > 0 {
		track.Year = req.Year
	}
	if req.TrackNum > 0 {
		track.TrackNum = req.TrackNum
	}
	if req.DiscNum > 0 {
		track.DiscNum = req.DiscNum
	}

	track.UpdatedAt = time.Now()

	// 尝试写入文件元数据
	ext := strings.ToLower(string(track.Format))
	switch ext {
	case "mp3":
		if err := te.writeMP3Tag(track.FilePath, req); err != nil {
			// 写入文件失败不影响内存更新，记录警告
			fmt.Printf("警告: 写入 MP3 标签失败: %v\n", err)
		}
	case "flac":
		if err := te.writeFLACTag(track.FilePath, req); err != nil {
			fmt.Printf("警告: 写入 FLAC 标签失败: %v\n", err)
		}
	}

	// 重建索引
	te.manager.rebuildIndex()

	_ = te.manager.saveConfig()
	return track, nil
}

// BatchUpdateTags 批量更新标签.
func (te *TagEditor) BatchUpdateTags(trackIDs []string, req TagUpdateRequest) ([]*Track, error) {
	tracks := make([]*Track, 0, len(trackIDs))
	for _, id := range trackIDs {
		track, err := te.UpdateTrackTag(id, req)
		if err != nil {
			return tracks, fmt.Errorf("更新曲目 %s 失败: %w", id, err)
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

// writeMP3Tag 写入 MP3 ID3v2 标签.
func (te *TagEditor) writeMP3Tag(filePath string, req TagUpdateRequest) error {
	// 读取原文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 检查是否有 ID3v2 头
	if len(data) < 10 || string(data[:3]) != "ID3" {
		// 没有 ID3v2 标签，创建新的
		return te.createID3v2Tag(filePath, data, req)
	}

	// 解析现有标签大小
	oldSize := decodeSyncsafe(data[6:10])

	// 构建新的标签帧
	frames := te.buildID3v2Frames(req)
	newTag := te.buildID3v2Tag(data[3], frames)

	// 替换文件内容
	newData := make([]byte, 0, len(newTag)+len(data)-10-oldSize)
	newData = append(newData, newTag...)
	newData = append(newData, data[10+oldSize:]...)

	return os.WriteFile(filePath, newData, 0644)
}

// writeFLACTag 写入 FLAC Vorbis Comment 标签.
func (te *TagEditor) writeFLACTag(filePath string, req TagUpdateRequest) error {
	// 读取原文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	if len(data) < 4 || string(data[:4]) != "fLaC" {
		return fmt.Errorf("不是有效的 FLAC 文件")
	}

	// 构建新的 Vorbis Comment 块
	newCommentBlock := te.buildVorbisComment(req)

	// 查找并替换 Vorbis Comment 块（类型 4）
	offset := 4
	var newFileData []byte
	newFileData = append(newFileData, data[:4]...) // fLaC 标识
	foundComment := false

	for offset+4 <= len(data) {
		blockHeader := data[offset]
		isLast := (blockHeader & 0x80) != 0
		blockType := blockHeader & 0x7F
		blockSize := int(binary.BigEndian.Uint32(append([]byte{0}, data[offset+1:offset+4]...)))

		if offset+4+blockSize > len(data) {
			break
		}

		if blockType == 4 {
			// 替换 Vorbis Comment 块
			foundComment = true
			// 设置块头（非最后块）
			commentHeader := make([]byte, 4)
			commentHeader[0] = 4
			binary.BigEndian.PutUint32(commentHeader[1:], uint32(len(newCommentBlock)))
			newFileData = append(newFileData, commentHeader...)
			newFileData = append(newFileData, newCommentBlock...)
		} else {
			// 保留其他块
			blockData := data[offset : offset+4+blockSize]
			if isLast && foundComment {
				// 原来是最后块，现在不是了（后面还要加新 comment 块）
				blockData[0] = blockData[0] & 0x7F // 清除最后标志
			}
			newFileData = append(newFileData, blockData...)
		}

		offset += 4 + blockSize
		if isLast && !foundComment {
			break
		}
	}

	if !foundComment {
		// 没有找到 comment 块，追加到末尾
		// 先清除原来最后块的 last 标志
		if len(newFileData) > 4 {
			// 查找原来的最后块并清除 last 标志
			tempOffset := 4
			for tempOffset+4 <= len(newFileData) {
				blockHeader := newFileData[tempOffset]
				isLast := (blockHeader & 0x80) != 0
				blockSize := int(binary.BigEndian.Uint32(append([]byte{0}, newFileData[tempOffset+1:tempOffset+4]...)))
				if isLast {
					newFileData[tempOffset] = blockHeader & 0x7F
					break
				}
				tempOffset += 4 + blockSize
			}
		}

		commentHeader := make([]byte, 4)
		commentHeader[0] = 4 | 0x80 // 设置为最后块
		binary.BigEndian.PutUint32(commentHeader[1:], uint32(len(newCommentBlock)))
		newFileData = append(newFileData, commentHeader...)
		newFileData = append(newFileData, newCommentBlock...)
	}

	// 追加音频数据（从第一个音频帧开始）
	// 简化处理：查找音频数据的起始位置
	audioStart := offset
	if audioStart < len(data) {
		newFileData = append(newFileData, data[audioStart:]...)
	}

	return os.WriteFile(filePath, newFileData, 0644)
}

// buildID3v2Frames 构建 ID3v2 帧数据.
func (te *TagEditor) buildID3v2Frames(req TagUpdateRequest) []byte {
	var frames []byte

	addFrame := func(id, value string) {
		if value == "" {
			return
		}
		text := []byte{0} // 编码: ISO-8859-1
		text = append(text, []byte(value)...)

		// 帧头
		frameHeader := make([]byte, 10)
		copy(frameHeader[:4], id)
		binary.BigEndian.PutUint32(frameHeader[4:8], uint32(len(text)))
		// flags = 0
		frames = append(frames, frameHeader...)
		frames = append(frames, text...)
	}

	if req.Title != "" {
		addFrame("TIT2", req.Title)
	}
	if req.Artist != "" {
		addFrame("TPE1", req.Artist)
	}
	if req.Album != "" {
		addFrame("TALB", req.Album)
	}
	if req.AlbumArtist != "" {
		addFrame("TPE2", req.AlbumArtist)
	}
	if req.Genre != "" {
		addFrame("TCON", req.Genre)
	}
	if req.Year > 0 {
		addFrame("TYER", fmt.Sprintf("%d", req.Year))
	}
	if req.TrackNum > 0 {
		addFrame("TRCK", fmt.Sprintf("%d", req.TrackNum))
	}
	if req.DiscNum > 0 {
		addFrame("TPOS", fmt.Sprintf("%d", req.DiscNum))
	}

	return frames
}

// buildID3v2Tag 构建完整的 ID3v2 标签.
func (te *TagEditor) buildID3v2Tag(version byte, frames []byte) []byte {
	// ID3v2 头部 (10 bytes)
	tag := make([]byte, 10)
	tag[0] = 'I'
	tag[1] = 'D'
	tag[2] = '3'
	tag[3] = version
	tag[4] = 0 // revision
	tag[5] = 0 // flags

	// 编码帧数据大小为同步安全整数
	size := len(frames)
	tag[6] = byte(size>>21) & 0x7F
	tag[7] = byte(size>>14) & 0x7F
	tag[8] = byte(size>>7) & 0x7F
	tag[9] = byte(size) & 0x7F

	tag = append(tag, frames...)
	return tag
}

// createID3v2Tag 创建新的 ID3v2 标签.
func (te *TagEditor) createID3v2Tag(filePath string, originalData []byte, req TagUpdateRequest) error {
	frames := te.buildID3v2Frames(req)
	if len(frames) == 0 {
		return nil
	}

	tag := te.buildID3v2Tag(3, frames) // ID3v2.3
	tag = append(tag, originalData...)

	return os.WriteFile(filePath, tag, 0644)
}

// buildVorbisComment 构建 Vorbis Comment 块数据.
func (te *TagEditor) buildVorbisComment(req TagUpdateRequest) []byte {
	var comments []string

	addComment := func(key, value string) {
		if value != "" {
			comments = append(comments, key+"="+value)
		}
	}

	addComment("TITLE", req.Title)
	addComment("ARTIST", req.Artist)
	addComment("ALBUM", req.Album)
	addComment("ALBUMARTIST", req.AlbumArtist)
	addComment("GENRE", req.Genre)
	if req.Year > 0 {
		addComment("DATE", fmt.Sprintf("%d", req.Year))
	}
	if req.TrackNum > 0 {
		addComment("TRACKNUMBER", fmt.Sprintf("%d", req.TrackNum))
	}
	if req.DiscNum > 0 {
		addComment("DISCNUMBER", fmt.Sprintf("%d", req.DiscNum))
	}

	// 构建块数据
	vendor := "nas-os/audiostation"
	var block []byte

	// vendor length (little-endian)
	vendorLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vendorLen, uint32(len(vendor)))
	block = append(block, vendorLen...)
	block = append(block, []byte(vendor)...)

	// comment count (little-endian)
	commentCount := make([]byte, 4)
	binary.LittleEndian.PutUint32(commentCount, uint32(len(comments)))
	block = append(block, commentCount...)

	// comments
	for _, comment := range comments {
		commentLen := make([]byte, 4)
		binary.LittleEndian.PutUint32(commentLen, uint32(len(comment)))
		block = append(block, commentLen...)
		block = append(block, []byte(comment)...)
	}

	return block
}
