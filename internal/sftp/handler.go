package sftp

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SFTPHandler SFTP 文件处理器（完整实现）
type SFTPHandler struct {
	rootDir      string
	readOnly     bool
	username     string
	sessionID    string
	clientIP     string
	logger       *TransferLogger
	bandwidthLim *BandwidthLimiter
}

// BandwidthLimiter 带宽限制器
type BandwidthLimiter struct {
	DownloadKBps int64
	UploadKBps   int64
	Enabled      bool
}

// NewSFTPHandler 创建 SFTP 处理器
func NewSFTPHandler(rootDir, username, sessionID, clientIP string, logger *TransferLogger, bwLimit *BandwidthLimiter) *SFTPHandler {
	return &SFTPHandler{
		rootDir:      rootDir,
		readOnly:     false,
		username:     username,
		sessionID:    sessionID,
		clientIP:     clientIP,
		logger:       logger,
		bandwidthLim: bwLimit,
	}
}

// resolvePath 解析路径（确保在 chroot 内）
func (h *SFTPHandler) resolvePath(name string) (string, error) {
	cleanPath := filepath.Clean(name)

	if strings.Contains(cleanPath, "..") {
		return "", os.ErrPermission
	}

	fullPath := filepath.Join(h.rootDir, cleanPath)

	if !strings.HasPrefix(fullPath, h.rootDir) {
		return "", os.ErrPermission
	}

	return fullPath, nil
}

// FileOp 文件操作接口实现

// Fileread 读取文件
func (h *SFTPHandler) Fileread(r *Request) (io.ReadCloser, error) {
	if h.readOnly {
		return nil, os.ErrPermission
	}

	fullPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}

	// 获取文件大小用于日志
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	// 创建传输日志
	var transferLog *TransferLog
	if h.logger != nil {
		transferLog = h.logger.StartTransfer(h.username, h.clientIP, h.sessionID, "download", r.Filepath, info.Size())
	}

	// 包装读取器以跟踪进度
	return &trackedReader{
		ReadCloser:   file,
		handler:      h,
		transferLog:  transferLog,
		startTime:    time.Now(),
		bytesRead:    0,
		totalSize:    info.Size(),
		bandwidthLim: h.bandwidthLim,
	}, nil
}

// Filewrite 写入文件
func (h *SFTPHandler) Filewrite(r *Request) (io.WriteCloser, error) {
	if h.readOnly {
		return nil, os.ErrPermission
	}

	fullPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return nil, err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return nil, err
	}

	// 创建传输日志
	var transferLog *TransferLog
	if h.logger != nil {
		transferLog = h.logger.StartTransfer(h.username, h.clientIP, h.sessionID, "upload", r.Filepath, 0)
	}

	return &trackedWriter{
		WriteCloser:  file,
		handler:      h,
		transferLog:  transferLog,
		startTime:    time.Now(),
		bytesWritten: 0,
		bandwidthLim: h.bandwidthLim,
	}, nil
}

// Filecmd 文件命令操作
func (h *SFTPHandler) Filecmd(r *Request) error {
	if h.readOnly && (r.Method == "Setstat" || r.Method == "Rename" || r.Method == "Rmdir" || r.Method == "Remove" || r.Method == "Mkdir") {
		return os.ErrPermission
	}

	switch r.Method {
	case "Setstat":
		return h.handleSetstat(r)
	case "Rename":
		return h.handleRename(r)
	case "Rmdir":
		return h.handleRmdir(r)
	case "Mkdir":
		return h.handleMkdir(r)
	case "Remove":
		return h.handleRemove(r)
	case "Symlink":
		return h.handleSymlink(r)
	default:
		return errors.New("unsupported command")
	}
}

// Filelist 列出文件
func (h *SFTPHandler) Filelist(r *Request) (ListerAt, error) {
	fullPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, os.ErrInvalid
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	fileInfos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, info)
	}

	return sliceListerAt(fileInfos), nil
}

// handleSetstat 处理属性设置
func (h *SFTPHandler) handleSetstat(r *Request) error {
	fullPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return err
	}

	if r.Mode != 0 {
		if err := os.Chmod(fullPath, r.Mode); err != nil {
			return err
		}
	}

	if !r.Mtime.IsZero() {
		if err := os.Chtimes(fullPath, r.Atime, r.Mtime); err != nil {
			return err
		}
	}

	return nil
}

// handleRename 处理重命名
func (h *SFTPHandler) handleRename(r *Request) error {
	oldPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return err
	}

	newPath, err := h.resolvePath(r.Target)
	if err != nil {
		return err
	}

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}

	return os.Rename(oldPath, newPath)
}

// handleRmdir 处理删除目录
func (h *SFTPHandler) handleRmdir(r *Request) error {
	fullPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return err
	}
	return os.RemoveAll(fullPath)
}

// handleMkdir 处理创建目录
func (h *SFTPHandler) handleMkdir(r *Request) error {
	fullPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return err
	}
	return os.MkdirAll(fullPath, 0755)
}

// handleRemove 处理删除文件
func (h *SFTPHandler) handleRemove(r *Request) error {
	fullPath, err := h.resolvePath(r.Filepath)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}

// handleSymlink 处理符号链接
func (h *SFTPHandler) handleSymlink(r *Request) error {
	// 安全考虑：禁用符号链接
	return os.ErrPermission
}

// trackedReader 跟踪读取进度的读取器
type trackedReader struct {
	io.ReadCloser
	handler      *SFTPHandler
	transferLog  *TransferLog
	startTime    time.Time
	bytesRead    int64
	totalSize    int64
	bandwidthLim *BandwidthLimiter
}

func (r *trackedReader) Read(p []byte) (n int, err error) {
	if r.bandwidthLim != nil && r.bandwidthLim.Enabled && r.bandwidthLim.DownloadKBps > 0 {
		// 限速读取
		start := time.Now()
		n, err = r.ReadCloser.Read(p)
		elapsed := time.Since(start)

		expected := time.Duration(int64(n)*1000/r.bandwidthLim.DownloadKBps) * time.Microsecond
		if elapsed < expected {
			time.Sleep(expected - elapsed)
		}
	} else {
		n, err = r.ReadCloser.Read(p)
	}

	r.bytesRead += int64(n)
	return n, err
}

func (r *trackedReader) Close() error {
	if r.transferLog != nil && r.handler.logger != nil {
		r.handler.logger.CompleteTransfer(
			r.transferLog,
			r.bytesRead,
			time.Since(r.startTime),
			true,
			"",
		)
	}
	return r.ReadCloser.Close()
}

// trackedWriter 跟踪写入进度的写入器
type trackedWriter struct {
	io.WriteCloser
	handler      *SFTPHandler
	transferLog  *TransferLog
	startTime    time.Time
	bytesWritten int64
	bandwidthLim *BandwidthLimiter
}

func (w *trackedWriter) Write(p []byte) (n int, err error) {
	if w.bandwidthLim != nil && w.bandwidthLim.Enabled && w.bandwidthLim.UploadKBps > 0 {
		// 限速写入
		start := time.Now()
		n, err = w.WriteCloser.Write(p)
		elapsed := time.Since(start)

		expected := time.Duration(int64(n)*1000/w.bandwidthLim.UploadKBps) * time.Microsecond
		if elapsed < expected {
			time.Sleep(expected - elapsed)
		}
	} else {
		n, err = w.WriteCloser.Write(p)
	}

	w.bytesWritten += int64(n)
	return n, err
}

func (w *trackedWriter) Close() error {
	if w.transferLog != nil && w.handler.logger != nil {
		w.transferLog.FileSize = w.bytesWritten
		w.handler.logger.CompleteTransfer(
			w.transferLog,
			w.bytesWritten,
			time.Since(w.startTime),
			true,
			"",
		)
	}
	return w.WriteCloser.Close()
}

// Request 简化的请求结构
type Request struct {
	Method   string
	Filepath string
	Target   string
	Mode     os.FileMode
	Atime    time.Time
	Mtime    time.Time
}

// ListerAt 列表接口
type ListerAt interface {
	ListAt([]os.FileInfo, int64) (int, error)
}

// sliceListerAt 切片实现的 ListerAt
type sliceListerAt []os.FileInfo

func (s sliceListerAt) ListAt(f []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(s)) {
		return 0, io.EOF
	}

	n := copy(f, s[offset:])
	if n < len(f) {
		return n, io.EOF
	}
	return n, nil
}

// HandshakeHandler SSH 握手处理器
type HandshakeHandler struct {
	server       *Server
	getUserHome  func(username string) string
	getUserPerms func(username string) *UserPermissions
}

// UserPermissions 用户权限
type UserPermissions struct {
	Read      bool
	Write     bool
	Delete    bool
	Admin     bool
	HomeDir   string
	ChrootDir string
}

// NewHandshakeHandler 创建握手处理器
func NewHandshakeHandler(server *Server) *HandshakeHandler {
	return &HandshakeHandler{
		server: server,
	}
}

// SetGetUserHome 设置获取用户主目录函数
func (h *HandshakeHandler) SetGetUserHome(fn func(username string) string) {
	h.getUserHome = fn
}

// SetGetUserPerms 设置获取用户权限函数
func (h *HandshakeHandler) SetGetUserPerms(fn func(username string) *UserPermissions) {
	h.getUserPerms = fn
}

// HandleConnection 处理 SSH 连接
func (h *HandshakeHandler) HandleConnection(conn net.Conn) {
	// 使用 server 的配置处理连接
	// 具体实现在 server.go 中
}
