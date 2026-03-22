package media

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StreamConfig 流式传输配置
type StreamConfig struct {
	// 视频比特率
	VideoBitrate string `json:"videoBitrate"`
	// 音频比特率
	AudioBitrate string `json:"audioBitrate"`
	// 分辨率
	Resolution string `json:"resolution"`
	// 帧率
	Framerate int `json:"framerate"`
	// HLS 分片时长（秒）
	HLSSegmentDuration int `json:"hlsSegmentDuration"`
	// HLS 分片数量
	HLSListSize int `json:"hlsListSize"`
	// 硬件加速
	HWAccel string `json:"hwAccel"`
}

// StreamServer 流媒体服务器
type StreamServer struct {
	config      StreamConfig
	ffmpegPath  string
	ffprobePath string
	sessions    map[string]*StreamSession
	mu          sync.RWMutex
}

// StreamSession 流媒体会话
type StreamSession struct {
	ID          string    `json:"id"`
	SourcePath  string    `json:"sourcePath"`
	Type        string    `json:"type"`   // hls, dash, rtsp, rtmp
	Status      string    `json:"status"` // starting, running, stopped, error
	StartTime   time.Time `json:"startTime"`
	Viewers     int       `json:"viewers"`
	OutputDir   string    `json:"outputDir"`
	ManifestURL string    `json:"manifestUrl"`
	Error       string    `json:"error,omitempty"`

	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
	mu     sync.RWMutex
}

// NewStreamServer 创建流媒体服务器
func NewStreamServer(config StreamConfig, ffmpegPath, ffprobePath string) *StreamServer {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	return &StreamServer{
		config:      config,
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		sessions:    make(map[string]*StreamSession),
	}
}

// DefaultStreamConfig 默认流配置
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		VideoBitrate:       "2M",
		AudioBitrate:       "128k",
		Resolution:         "1280x720",
		Framerate:          30,
		HLSSegmentDuration: 6,
		HLSListSize:        10,
		HWAccel:            "none",
	}
}

// CreateHLSSession 创建 HLS 流会话
func (ss *StreamServer) CreateHLSSession(sourcePath, outputDir string) (*StreamSession, error) {
	// 检查源文件
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("源文件不存在: %s", sourcePath)
	}

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建会话
	session := &StreamSession{
		ID:         fmt.Sprintf("hls_%d", time.Now().UnixNano()),
		SourcePath: sourcePath,
		Type:       "hls",
		Status:     "starting",
		StartTime:  time.Now(),
		OutputDir:  outputDir,
	}

	session.ctx, session.cancel = context.WithCancel(context.Background())

	ss.mu.Lock()
	ss.sessions[session.ID] = session
	ss.mu.Unlock()

	// 启动转码
	go ss.runHLSStream(session)

	return session, nil
}

// runHLSStream 运行 HLS 流
func (ss *StreamServer) runHLSStream(session *StreamSession) {
	// 播放列表文件
	playlistPath := filepath.Join(session.OutputDir, "stream.m3u8")
	segmentPattern := filepath.Join(session.OutputDir, "segment_%03d.ts")

	// 构建 ffmpeg 命令
	args := ss.buildHLSArgs(session.SourcePath, playlistPath, segmentPattern)

	session.cmd = exec.CommandContext(session.ctx, ss.ffmpegPath, args...)

	// 捕获输出
	stderr, err := session.cmd.StderrPipe()
	if err != nil {
		session.mu.Lock()
		session.Status = "error"
		session.Error = err.Error()
		session.mu.Unlock()
		return
	}

	if err := session.cmd.Start(); err != nil {
		session.mu.Lock()
		session.Status = "error"
		session.Error = err.Error()
		session.mu.Unlock()
		return
	}

	session.mu.Lock()
	session.Status = "running"
	session.ManifestURL = "/stream/" + session.ID + "/stream.m3u8"
	session.mu.Unlock()

	// 读取输出日志
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// 可以记录日志
			_ = scanner.Text()
		}
	}()

	// 等待命令完成
	err = session.cmd.Wait()

	session.mu.Lock()
	if err != nil && session.ctx.Err() == nil {
		session.Status = "error"
		session.Error = err.Error()
	} else if session.ctx.Err() != nil {
		session.Status = "stopped"
	} else {
		session.Status = "completed"
	}
	session.mu.Unlock()
}

// buildHLSArgs 构建 HLS 转码参数
func (ss *StreamServer) buildHLSArgs(inputPath, playlistPath, segmentPattern string) []string {
	args := []string{"-i", inputPath}

	// 硬件加速
	switch strings.ToLower(ss.config.HWAccel) {
	case "cuda", "nvenc":
		args = append(args, "-c:v", "h264_nvenc")
	case "qsv":
		args = append(args, "-c:v", "h264_qsv")
	case "vaapi":
		args = append(args, "-c:v", "h264_vaapi")
	default:
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "fast")
	}

	// 视频参数
	if ss.config.VideoBitrate != "" {
		args = append(args, "-b:v", ss.config.VideoBitrate)
	}
	if ss.config.Resolution != "" {
		args = append(args, "-s", ss.config.Resolution)
	}
	if ss.config.Framerate > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", ss.config.Framerate))
	}

	// 音频编码
	args = append(args, "-c:a", "aac")
	if ss.config.AudioBitrate != "" {
		args = append(args, "-b:a", ss.config.AudioBitrate)
	}

	// HLS 参数
	hlsTime := ss.config.HLSSegmentDuration
	if hlsTime <= 0 {
		hlsTime = 6
	}
	hlsListSize := ss.config.HLSListSize
	if hlsListSize <= 0 {
		hlsListSize = 10
	}

	args = append(args,
		"-hls_time", fmt.Sprintf("%d", hlsTime),
		"-hls_list_size", fmt.Sprintf("%d", hlsListSize),
		"-hls_flags", "delete_segments",
		"-f", "hls",
		"-y", playlistPath,
	)

	return args
}

// StopSession 停止流会话
func (ss *StreamServer) StopSession(sessionID string) error {
	ss.mu.RLock()
	session, ok := ss.sessions[sessionID]
	ss.mu.RUnlock()

	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != "running" {
		return fmt.Errorf("会话未在运行")
	}

	if session.cancel != nil {
		session.cancel()
	}

	session.Status = "stopped"
	return nil
}

// GetSession 获取会话信息
func (ss *StreamServer) GetSession(sessionID string) *StreamSession {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.sessions[sessionID]
}

// ListSessions 列出所有会话
func (ss *StreamServer) ListSessions() []*StreamSession {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	sessions := make([]*StreamSession, 0, len(ss.sessions))
	for _, s := range ss.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// DeleteSession 删除会话
func (ss *StreamServer) DeleteSession(sessionID string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, ok := ss.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	session.mu.Lock()
	if session.Status == "running" {
		session.mu.Unlock()
		return fmt.Errorf("无法删除运行中的会话")
	}
	session.mu.Unlock()

	// 清理输出文件
	if session.OutputDir != "" {
		_ = os.RemoveAll(session.OutputDir)
	}

	delete(ss.sessions, sessionID)
	return nil
}

// StreamFile 直接流式传输文件（Range 支持）
func (ss *StreamServer) StreamFile(w http.ResponseWriter, r *http.Request, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	fileSize := stat.Size()
	fileName := filepath.Base(filePath)

	// 设置响应头
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", ss.getContentType(filePath))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileName))

	// 处理 Range 请求
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		ss.handleRangeRequest(w, r, file, fileSize, rangeHeader)
		return nil
	}

	// 完整文件传输
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
	http.ServeContent(w, r, fileName, stat.ModTime(), file)
	return nil
}

// handleRangeRequest 处理 Range 请求
func (ss *StreamServer) handleRangeRequest(w http.ResponseWriter, r *http.Request, file *os.File, fileSize int64, rangeHeader string) {
	// 解析 Range 头
	// 格式: bytes=start-end 或 bytes=start-
	var start, end int64
	_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
	if err != nil {
		// 可能只有 start
		_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
		if err != nil {
			http.Error(w, "Invalid Range", http.StatusBadRequest)
			return
		}
		end = fileSize - 1
	}

	// 验证范围
	if start >= fileSize || start > end {
		http.Error(w, "Invalid Range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if end >= fileSize {
		end = fileSize - 1
	}

	contentLength := end - start + 1

	// 设置响应头
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.WriteHeader(http.StatusPartialContent)

	// 定位并传输
	_, _ = file.Seek(start, 0)
	_, _ = io.CopyN(w, file, contentLength)
}

// getContentType 获取内容类型
func (ss *StreamServer) getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// TranscodeStream 实时转码流
func (ss *StreamServer) TranscodeStream(ctx context.Context, inputPath string, config StreamConfig, output io.Writer) error {
	args := ss.buildTranscodeArgs(inputPath, config)

	cmd := exec.CommandContext(ctx, ss.ffmpegPath, args...)

	// 设置输出
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// 复制输出
	_, err = io.Copy(output, pipe)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// buildTranscodeArgs 构建转码参数
func (ss *StreamServer) buildTranscodeArgs(inputPath string, config StreamConfig) []string {
	args := []string{"-i", inputPath}

	// 视频编码
	args = append(args, "-c:v", "libx264", "-preset", "fast")
	if config.VideoBitrate != "" {
		args = append(args, "-b:v", config.VideoBitrate)
	}
	if config.Resolution != "" {
		args = append(args, "-s", config.Resolution)
	}

	// 音频编码
	args = append(args, "-c:a", "aac")
	if config.AudioBitrate != "" {
		args = append(args, "-b:a", config.AudioBitrate)
	}

	// 输出格式
	args = append(args, "-f", "matroska", "-")

	return args
}

// CreateDASHSession 创建 DASH 流会话
func (ss *StreamServer) CreateDASHSession(sourcePath, outputDir string) (*StreamSession, error) {
	// 检查源文件
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("源文件不存在: %s", sourcePath)
	}

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建会话
	session := &StreamSession{
		ID:         fmt.Sprintf("dash_%d", time.Now().UnixNano()),
		SourcePath: sourcePath,
		Type:       "dash",
		Status:     "starting",
		StartTime:  time.Now(),
		OutputDir:  outputDir,
	}

	session.ctx, session.cancel = context.WithCancel(context.Background())

	ss.mu.Lock()
	ss.sessions[session.ID] = session
	ss.mu.Unlock()

	// 启动转码
	go ss.runDASHStream(session)

	return session, nil
}

// runDASHStream 运行 DASH 流
func (ss *StreamServer) runDASHStream(session *StreamSession) {
	manifestPath := filepath.Join(session.OutputDir, "stream.mpd")

	// 构建 ffmpeg 命令
	args := ss.buildDASHArgs(session.SourcePath, manifestPath, session.OutputDir)

	session.cmd = exec.CommandContext(session.ctx, ss.ffmpegPath, args...)

	if err := session.cmd.Start(); err != nil {
		session.mu.Lock()
		session.Status = "error"
		session.Error = err.Error()
		session.mu.Unlock()
		return
	}

	session.mu.Lock()
	session.Status = "running"
	session.ManifestURL = "/stream/" + session.ID + "/stream.mpd"
	session.mu.Unlock()

	err := session.cmd.Wait()

	session.mu.Lock()
	if err != nil && session.ctx.Err() == nil {
		session.Status = "error"
		session.Error = err.Error()
	} else if session.ctx.Err() != nil {
		session.Status = "stopped"
	} else {
		session.Status = "completed"
	}
	session.mu.Unlock()
}

// buildDASHArgs 构建 DASH 转码参数
func (ss *StreamServer) buildDASHArgs(inputPath, manifestPath, outputDir string) []string {
	args := []string{"-i", inputPath}

	// 视频编码
	args = append(args,
		"-c:v", "libx264",
		"-preset", "fast",
		"-b:v", ss.config.VideoBitrate,
	)

	// 音频编码
	args = append(args,
		"-c:a", "aac",
		"-b:a", ss.config.AudioBitrate,
	)

	// DASH 参数
	args = append(args,
		"-f", "dash",
		"-seg_duration", fmt.Sprintf("%d", ss.config.HLSSegmentDuration),
		"-y", manifestPath,
	)

	return args
}

// AdaptiveStream 自适应流配置
type AdaptiveStream struct {
	Quality    string `json:"quality"`
	Resolution string `json:"resolution"`
	Bitrate    string `json:"bitrate"`
}

// CreateAdaptiveHLS 创建自适应 HLS 流（多质量）
func (ss *StreamServer) CreateAdaptiveHLS(sourcePath, outputDir string, qualities []AdaptiveStream) (*StreamSession, error) {
	if len(qualities) == 0 {
		qualities = []AdaptiveStream{
			{Quality: "1080p", Resolution: "1920x1080", Bitrate: "5M"},
			{Quality: "720p", Resolution: "1280x720", Bitrate: "2M"},
			{Quality: "480p", Resolution: "854x480", Bitrate: "1M"},
		}
	}

	// 检查源文件
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("源文件不存在: %s", sourcePath)
	}

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 创建会话
	session := &StreamSession{
		ID:         fmt.Sprintf("ahls_%d", time.Now().UnixNano()),
		SourcePath: sourcePath,
		Type:       "adaptive-hls",
		Status:     "starting",
		StartTime:  time.Now(),
		OutputDir:  outputDir,
	}

	session.ctx, session.cancel = context.WithCancel(context.Background())

	ss.mu.Lock()
	ss.sessions[session.ID] = session
	ss.mu.Unlock()

	// 启动自适应转码
	go ss.runAdaptiveHLS(session, qualities)

	return session, nil
}

// runAdaptiveHLS 运行自适应 HLS 流
func (ss *StreamServer) runAdaptiveHLS(session *StreamSession, qualities []AdaptiveStream) {
	// 构建复杂的 ffmpeg 命令（多输出）
	args := []string{"-i", session.SourcePath}

	for _, q := range qualities {
		// 输出参数
		outputPath := filepath.Join(session.OutputDir, q.Quality, "stream.m3u8")
		_ = os.MkdirAll(filepath.Dir(outputPath), 0750)

		args = append(args,
			"-map", "0:v",
			"-map", "0:a",
			"-c:v", "libx264",
			"-b:v", q.Bitrate,
			"-s", q.Resolution,
			"-c:a", "aac",
			"-b:a", "128k",
			"-hls_time", fmt.Sprintf("%d", ss.config.HLSSegmentDuration),
			"-hls_list_size", fmt.Sprintf("%d", ss.config.HLSListSize),
			"-hls_flags", "delete_segments",
			"-f", "hls",
			"-y", outputPath,
		)
	}

	session.cmd = exec.CommandContext(session.ctx, ss.ffmpegPath, args...)

	if err := session.cmd.Start(); err != nil {
		session.mu.Lock()
		session.Status = "error"
		session.Error = err.Error()
		session.mu.Unlock()
		return
	}

	session.mu.Lock()
	session.Status = "running"
	session.mu.Unlock()

	err := session.cmd.Wait()

	session.mu.Lock()
	if err != nil && session.ctx.Err() == nil {
		session.Status = "error"
		session.Error = err.Error()
	} else if session.ctx.Err() != nil {
		session.Status = "stopped"
	} else {
		session.Status = "completed"

		// 创建主播放列表
		ss.createMasterPlaylist(session.OutputDir, qualities)
	}
	session.mu.Unlock()
}

// createMasterPlaylist 创建主播放列表
func (ss *StreamServer) createMasterPlaylist(outputDir string, qualities []AdaptiveStream) {
	playlistPath := filepath.Join(outputDir, "master.m3u8")
	content := "#EXTM3U\n#EXT-X-VERSION:3\n"

	for _, q := range qualities {
		bandwidth := ss.parseBitrate(q.Bitrate)
		content += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s\n",
			bandwidth, q.Resolution)
		content += fmt.Sprintf("%s/stream.m3u8\n", q.Quality)
	}

	_ = os.WriteFile(playlistPath, []byte(content), 0640)
}

// parseBitrate 解析比特率字符串
func (ss *StreamServer) parseBitrate(bitrate string) int {
	bitrate = strings.TrimSpace(bitrate)
	multiplier := 1

	if strings.HasSuffix(bitrate, "M") {
		multiplier = 1000000
		bitrate = strings.TrimSuffix(bitrate, "M")
	} else if strings.HasSuffix(bitrate, "K") || strings.HasSuffix(bitrate, "k") {
		multiplier = 1000
		bitrate = strings.TrimSuffix(bitrate, "K")
		bitrate = strings.TrimSuffix(bitrate, "k")
	}

	value, err := strconv.Atoi(bitrate)
	if err != nil {
		return 2000000 // 默认 2M
	}

	return value * multiplier
}

// IncrementViewer 增加观看者数量
func (ss *StreamServer) IncrementViewer(sessionID string) {
	ss.mu.RLock()
	session, ok := ss.sessions[sessionID]
	ss.mu.RUnlock()

	if ok {
		session.mu.Lock()
		session.Viewers++
		session.mu.Unlock()
	}
}

// DecrementViewer 减少观看者数量
func (ss *StreamServer) DecrementViewer(sessionID string) {
	ss.mu.RLock()
	session, ok := ss.sessions[sessionID]
	ss.mu.RUnlock()

	if ok {
		session.mu.Lock()
		if session.Viewers > 0 {
			session.Viewers--
		}
		session.mu.Unlock()
	}
}

// CleanupStaleSessions 清理过期会话
func (ss *StreamServer) CleanupStaleSessions(maxAge time.Duration) int {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	count := 0
	now := time.Now()

	for id, session := range ss.sessions {
		session.mu.RLock()
		status := session.Status
		startTime := session.StartTime
		session.mu.RUnlock()

		if status != "running" && now.Sub(startTime) > maxAge {
			// 清理输出文件
			if session.OutputDir != "" {
				_ = os.RemoveAll(session.OutputDir)
			}
			delete(ss.sessions, id)
			count++
		}
	}

	return count
}
