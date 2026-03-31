// Package media provides transcoding and streaming support
package media

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TranscodeManager 转码管理器 - 支持实时转码播放
type TranscodeManager struct {
	transcoder   *Transcoder
	sessions     map[string]*PlaybackSession
	sessionQueue chan *PlaybackSession
	maxSessions  int
	config       TranscodeManagerConfig
	mu           sync.RWMutex
}

// TranscodeManagerConfig 转码管理器配置
type TranscodeManagerConfig struct {
	// FFmpeg路径
	FFmpegPath  string
	FFprobePath string

	// 并发限制
	MaxConcurrentSessions int

	// 硬件加速
	HWAccel       string // none, cuda, nvenc, qsv, vaapi
	HWAccelDevice string

	// 预设转码配置
	DefaultConfig TranscodeConfig

	// 输出目录
	OutputDir string

	// 缓存配置
	EnableCache bool
	CacheDir    string
	CacheMaxAge time.Duration

	// HLS配置
	HLSSegmentDuration int    // 秒
	HLSSegmentPrefix   string
}

// DefaultTranscodeManagerConfig 默认配置
func DefaultTranscodeManagerConfig() TranscodeManagerConfig {
	return TranscodeManagerConfig{
		FFmpegPath:            "ffmpeg",
		FFprobePath:           "ffprobe",
		MaxConcurrentSessions: 3,
		HWAccel:               "none",
		DefaultConfig:         DefaultTranscodeConfig(),
		OutputDir:             "/tmp/transcode",
		EnableCache:           true,
		CacheDir:              "/tmp/transcode_cache",
		CacheMaxAge:           24 * time.Hour,
		HLSSegmentDuration:    6,
		HLSSegmentPrefix:      "segment",
	}
}

// NewTranscodeManager 创建转码管理器
func NewTranscodeManager(config TranscodeManagerConfig) *TranscodeManager {
	if config.MaxConcurrentSessions <= 0 {
		config.MaxConcurrentSessions = 3
	}

	// 确保输出目录存在
	_ = os.MkdirAll(config.OutputDir, 0755)
	if config.EnableCache {
		_ = os.MkdirAll(config.CacheDir, 0755)
	}

	t := NewTranscoder(config.FFmpegPath, config.FFprobePath, config.MaxConcurrentSessions)

	return &TranscodeManager{
		transcoder:   t,
		sessions:     make(map[string]*PlaybackSession),
		sessionQueue: make(chan *PlaybackSession, config.MaxConcurrentSessions),
		maxSessions:  config.MaxConcurrentSessions,
		config:       config,
	}
}

// PlaybackSession 播放会话
type PlaybackSession struct {
	ID            string
	SourcePath    string
	OutputDir     string
	Format        string // hls, dash, mp4
	Status        string // preparing, ready, playing, ended, error
	Progress      float64
	HLSManifest   string
	DASHManifest  string
	SegmentCount  int
	Duration      float64
	StartTime     time.Time
	EndTime       *time.Time
	LastError     string
	ClientIP      string
	Quality       string
	TranscodeJob  *TranscodeJob
	Bitrate       int64
	Resolution    string

	ctx    context.Context
	cancel context.CancelFunc
}

// CreatePlaybackSession 创建播放会话
func (m *TranscodeManager) CreatePlaybackSession(ctx context.Context, sourcePath, format string, quality string) (*PlaybackSession, error) {
	// 检查源文件
	if _, err := os.Stat(sourcePath); err != nil {
		return nil, fmt.Errorf("源文件不存在: %s", sourcePath)
	}

	// 获取视频信息
	info, err := m.transcoder.GetVideoInfo(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}

	// 检查是否需要转码（如果格式兼容，直接使用原始文件）
	if m.shouldTranscode(info, format, quality) {
		return m.createTranscodeSession(ctx, sourcePath, format, quality, info)
	}

	// 不需要转码，直接创建播放会话
	session := &PlaybackSession{
		ID:         generateSessionID(),
		SourcePath: sourcePath,
		OutputDir:  "",
		Format:     format,
		Status:     "ready",
		Progress:   100,
		Duration:   info.Duration,
		StartTime:  time.Now(),
		Bitrate:    info.Bitrate,
		Resolution: fmt.Sprintf("%dx%d", info.Width, info.Height),
		Quality:    quality,
	}

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	return session, nil
}

// shouldTranscode 判断是否需要转码
func (m *TranscodeManager) shouldTranscode(info *VideoInfo, format, quality string) bool {
	// HLS需要转码
	if format == "hls" {
		return true
	}

	// DASH需要转码
	if format == "dash" {
		return true
	}

	// 指定质量且与原视频不同
	if quality != "original" {
		targetRes := getTargetResolution(quality)
		if targetRes.Height != info.Height || targetRes.Width != info.Width {
			return true
		}
	}

	// 编码不兼容常见播放器
	if info.VideoCodec != "h264" && info.VideoCodec != "hevc" {
		return true
	}

	// 音频编码不兼容
	if info.AudioCodec != "aac" && info.AudioCodec != "ac3" {
		return true
	}

	return false
}

// createTranscodeSession 创建转码会话
func (m *TranscodeManager) createTranscodeSession(ctx context.Context, sourcePath, format, quality string, info *VideoInfo) (*PlaybackSession, error) {
	sessionCtx, cancel := context.WithCancel(ctx)

	sessionID := generateSessionID()
	outputDir := filepath.Join(m.config.OutputDir, sessionID)
	_ = os.MkdirAll(outputDir, 0755)

	session := &PlaybackSession{
		ID:         sessionID,
		SourcePath: sourcePath,
		OutputDir:  outputDir,
		Format:     format,
		Status:     "preparing",
		StartTime:  time.Now(),
		Duration:   info.Duration,
		Quality:    quality,
		ctx:        sessionCtx,
		cancel:     cancel,
	}

	// 创建转码配置
	config := m.createTranscodeConfig(info, format, quality)

	// 根据格式启动不同类型的转码
	switch format {
	case "hls":
		go m.startHLSTranscode(session, config)
	case "dash":
		go m.startDASHTranscode(session, config)
	case "mp4":
		go m.startMP4Transcode(session, config)
	default:
		go m.startHLSTranscode(session, config) // 默认HLS
	}

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	return session, nil
}

// createTranscodeConfig 创建转码配置
func (m *TranscodeManager) createTranscodeConfig(info *VideoInfo, format, quality string) TranscodeConfig {
	config := m.config.DefaultConfig

	// 根据质量调整参数
	targetRes := getTargetResolution(quality)
	config.Resolution = fmt.Sprintf("%dx%d", targetRes.Width, targetRes.Height)

	// 根据分辨率设置比特率
	bitrate := getTargetBitrate(quality)
	config.VideoBitrate = bitrate

	// 根据格式调整编码器
	switch format {
	case "hls":
		config.OutputFormat = "hls"
		config.VideoCodec = "libx264"
		config.AudioCodec = "aac"
	case "dash":
		config.OutputFormat = "dash"
		config.VideoCodec = "libx264"
		config.AudioCodec = "aac"
	case "mp4":
		config.OutputFormat = "mp4"
	}

	// 硬件加速
	if m.config.HWAccel != "none" {
		config.HWAccel = m.config.HWAccel
	}

	return config
}

// startHLSTranscode 启动HLS转码
func (m *TranscodeManager) startHLSTranscode(session *PlaybackSession, config TranscodeConfig) {
	defer func() {
		now := time.Now()
		session.EndTime = &now
		session.cancel()
	}()

	// HLS输出路径
	manifestPath := filepath.Join(session.OutputDir, "playlist.m3u8")

	// 构建FFmpeg命令
	args := m.buildHLSCommand(session.SourcePath, manifestPath, session.OutputDir, config, session.Duration)

	cmd := exec.CommandContext(session.ctx, m.config.FFmpegPath, args...)

	// 启动转码
	session.Status = "transcoding"
	session.HLSManifest = manifestPath

	err := cmd.Run()
	if err != nil {
		session.Status = "error"
		session.LastError = err.Error()
		return
	}

	session.Status = "ready"
	session.Progress = 100
}

// buildHLSCommand 构建HLS转码命令
func (m *TranscodeManager) buildHLSCommand(sourcePath, manifestPath, outputDir string, config TranscodeConfig, duration float64) []string {
	args := []string{
		"-i", sourcePath,
	}

	// 硬件加速
	if config.HWAccel != "none" {
		switch config.HWAccel {
		case "cuda", "nvenc":
			args = append(args, "-hwaccel", "cuda")
			args = append(args, "-c:v", "h264_nvenc")
		case "qsv":
			args = append(args, "-hwaccel", "qsv")
			args = append(args, "-c:v", "h264_qsv")
		case "vaapi":
			args = append(args, "-hwaccel", "vaapi")
			args = append(args, "-c:v", "h264_vaapi")
		default:
			args = append(args, "-c:v", config.VideoCodec)
		}
	} else {
		args = append(args, "-c:v", config.VideoCodec)
	}

	// 视频参数
	if config.VideoBitrate != "" {
		args = append(args, "-b:v", config.VideoBitrate)
	}
	if config.Resolution != "" {
		args = append(args, "-s", config.Resolution)
	}
	if config.Preset != "" {
		args = append(args, "-preset", config.Preset)
	}

	// 音频参数
	args = append(args, "-c:a", config.AudioCodec)
	if config.AudioBitrate != "" {
		args = append(args, "-b:a", config.AudioBitrate)
	}

	// HLS参数
	args = append(args,
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", m.config.HLSSegmentDuration),
		"-hls_list_size", "0", // 不限制列表大小
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(outputDir, m.config.HLSSegmentPrefix+"_%05d.ts"),
		manifestPath,
	)

	return args
}

// startDASHTranscode 启动DASH转码
func (m *TranscodeManager) startDASHTranscode(session *PlaybackSession, config TranscodeConfig) {
	defer func() {
		now := time.Now()
		session.EndTime = &now
		session.cancel()
	}()

	manifestPath := filepath.Join(session.OutputDir, "playlist.mpd")

	args := m.buildDASHCommand(session.SourcePath, manifestPath, session.OutputDir, config)

	cmd := exec.CommandContext(session.ctx, m.config.FFmpegPath, args...)

	session.Status = "transcoding"
	session.DASHManifest = manifestPath

	err := cmd.Run()
	if err != nil {
		session.Status = "error"
		session.LastError = err.Error()
		return
	}

	session.Status = "ready"
	session.Progress = 100
}

// buildDASHCommand 构建DASH转码命令
func (m *TranscodeManager) buildDASHCommand(sourcePath, manifestPath, outputDir string, config TranscodeConfig) []string {
	args := []string{
		"-i", sourcePath,
		"-c:v", config.VideoCodec,
		"-c:a", config.AudioCodec,
	}

	if config.VideoBitrate != "" {
		args = append(args, "-b:v", config.VideoBitrate)
	}
	if config.Resolution != "" {
		args = append(args, "-s", config.Resolution)
	}

	// DASH参数
	args = append(args,
		"-f", "dash",
		"-seg_duration", fmt.Sprintf("%d", m.config.HLSSegmentDuration),
		"-dash_segment_type", "mp4",
		"-hls_playlist", "0",
		manifestPath,
	)

	return args
}

// startMP4Transcode 启动MP4转码
func (m *TranscodeManager) startMP4Transcode(session *PlaybackSession, config TranscodeConfig) {
	defer func() {
		now := time.Now()
		session.EndTime = &now
		session.cancel()
	}()

	outputPath := filepath.Join(session.OutputDir, "output.mp4")

	args := []string{
		"-i", session.SourcePath,
		"-c:v", config.VideoCodec,
		"-c:a", config.AudioCodec,
	}

	if config.VideoBitrate != "" {
		args = append(args, "-b:v", config.VideoBitrate)
	}
	if config.Resolution != "" {
		args = append(args, "-s", config.Resolution)
	}

	args = append(args, "-movflags", "faststart", outputPath)

	cmd := exec.CommandContext(session.ctx, m.config.FFmpegPath, args...)

	session.Status = "transcoding"

	err := cmd.Run()
	if err != nil {
		session.Status = "error"
		session.LastError = err.Error()
		return
	}

	session.Status = "ready"
	session.Progress = 100
}

// GetSession 获取会话
func (m *TranscodeManager) GetSession(sessionID string) (*PlaybackSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}
	return session, nil
}

// EndSession 结束会话
func (m *TranscodeManager) EndSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	if session.cancel != nil {
		session.cancel()
	}

	// 清理输出文件
	_ = os.RemoveAll(session.OutputDir)

	now := time.Now()
	session.EndTime = &now
	session.Status = "ended"

	delete(m.sessions, sessionID)
	return nil
}

// ListSessions 列出所有会话
func (m *TranscodeManager) ListSessions() []*PlaybackSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*PlaybackSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// CleanupOldSessions 清理过期会话
func (m *TranscodeManager) CleanupOldSessions(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for id, session := range m.sessions {
		if session.EndTime != nil && now.Sub(*session.EndTime) > maxAge {
			_ = os.RemoveAll(session.OutputDir)
			delete(m.sessions, id)
			cleaned++
		}
	}

	return cleaned
}

// ====== 流媒体服务器 ======

// StreamingServer 流媒体服务器
type StreamingServer struct {
	manager *TranscodeManager
	config  StreamingServerConfig
}

// StreamingServerConfig 流媒体服务器配置
type StreamingServerConfig struct {
	// 监听地址
	Addr string

	// 基础URL（用于生成播放URL）
	BaseURL string

	// 最大会话数
	MaxSessions int

	// 会话超时
	SessionTimeout time.Duration

	// 预缓冲时间（秒）
	PreBuffer int
}

// NewStreamingServer 创建流媒体服务器
func NewStreamingServer(manager *TranscodeManager, config StreamingServerConfig) *StreamingServer {
	return &StreamingServer{
		manager: manager,
		config:  config,
	}
}

// ServeHTTP 处理流媒体请求
func (s *StreamingServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 解析请求
	// /stream/{sessionID}/playlist.m3u8
	// /stream/{sessionID}/segment_00001.ts

	path := r.URL.Path
	parts := strings.Split(path, "/")

	if len(parts) < 3 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	sessionID := parts[2]
	session, err := s.manager.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 更新会话状态
	if session.Status == "ready" {
		session.Status = "playing"
	}

	// 处理manifest请求
	if len(parts) >= 4 {
		fileName := parts[3]
		filePath := filepath.Join(session.OutputDir, fileName)

		// 检查文件是否存在
		if _, err := os.Stat(filePath); err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		// 根据文件类型设置Content-Type
		ext := filepath.Ext(fileName)
		switch ext {
		case ".m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		case ".ts":
			w.Header().Set("Content-Type", "video/mp2t")
		case ".mpd":
			w.Header().Set("Content-Type", "application/dash+xml")
		case ".m4s":
			w.Header().Set("Content-Type", "video/mp4")
		}

		http.ServeFile(w, r, filePath)
	}
}

// ====== 辅助函数 ======

func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

type Resolution struct {
	Width  int
	Height int
}

func getTargetResolution(quality string) Resolution {
	switch quality {
	case "8k":
		return Resolution{Width: 7680, Height: 4320}
	case "4k", "uhd":
		return Resolution{Width: 3840, Height: 2160}
	case "fhd", "1080p":
		return Resolution{Width: 1920, Height: 1080}
	case "hd", "720p":
		return Resolution{Width: 1280, Height: 720}
	case "sd", "480p":
		return Resolution{Width: 640, Height: 480}
	default:
		return Resolution{Width: 1920, Height: 1080} // 默认1080p
	}
}

func getTargetBitrate(quality string) string {
	switch quality {
	case "8k":
		return "50M"
	case "4k", "uhd":
		return "20M"
	case "fhd", "1080p":
		return "5M"
	case "hd", "720p":
		return "2M"
	case "sd", "480p":
		return "1M"
	default:
		return "5M"
	}
}