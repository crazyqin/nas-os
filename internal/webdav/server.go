package webdav

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Config WebDAV 服务器配置
type Config struct {
	Enabled       bool   `json:"enabled"`
	Port          int    `json:"port"`
	RootPath      string `json:"root_path"`
	AllowGuest    bool   `json:"allow_guest"`     // 允许匿名访问
	MaxUploadSize int64  `json:"max_upload_size"` // 最大上传大小 (字节)，0 表示不限制
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:       true,
		Port:          8081,
		RootPath:      "/data",
		AllowGuest:    false,
		MaxUploadSize: 0, // 不限制
	}
}

// Server WebDAV 服务器
type Server struct {
	mu            sync.RWMutex
	config        *Config
	server        *http.Server
	authFunc      func(username, password string) bool // 认证函数
	getUserHome   func(username string) string         // 获取用户主目录
	lockManager   *LockManager
	quotaProvider QuotaProvider
	userSessions  map[string]string // 连接 -> 用户名
}

// NewServer 创建 WebDAV 服务器
func NewServer(config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Server{
		config:        config,
		lockManager:   NewLockManager(),
		quotaProvider: &NoOpQuotaProvider{},
		userSessions:  make(map[string]string),
	}

	return s, nil
}

// SetAuthFunc 设置认证函数
func (s *Server) SetAuthFunc(fn func(username, password string) bool) {
	s.authFunc = fn
}

// SetGetUserHome 设置获取用户主目录函数
func (s *Server) SetGetUserHome(fn func(username string) string) {
	s.getUserHome = fn
}

// SetQuotaProvider 设置配额提供者
func (s *Server) SetQuotaProvider(provider QuotaProvider) {
	s.quotaProvider = provider
}

// Start 启动 WebDAV 服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.config.Enabled {
		return nil
	}

	// 确保根目录存在
	if err := os.MkdirAll(s.config.RootPath, 0755); err != nil {
		return fmt.Errorf("创建根目录失败：%w", err)
	}

	// 创建 WebDAV 处理器
	handler := s.createHandler()

	// 创建 HTTP 服务器
	addr := fmt.Sprintf(":%d", s.config.Port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// 启动定期清理过期锁
	go s.startLockCleanup()

	// 启动服务器
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("WebDAV 服务器错误：%v\n", err)
		}
	}()

	return nil
}

// startLockCleanup 定期清理过期锁
func (s *Server) startLockCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.lockManager.CleanupExpired()
	}
}

// Stop 停止 WebDAV 服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}

	return nil
}

// createHandler 创建 WebDAV HTTP 处理器
func (s *Server) createHandler() http.Handler {
	mux := http.NewServeMux()

	// WebDAV 端点
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 认证检查
		username := ""
		if !s.config.AllowGuest {
			var ok bool
			username, ok = s.authenticateRequest(w, r)
			if !ok {
				return
			}
		}

		// 保存用户会话
		s.mu.Lock()
		s.userSessions[r.RemoteAddr] = username
		s.mu.Unlock()

		// 处理 WebDAV 方法
		s.handleWebDAV(w, r, username)
	})

	return mux
}

// authenticateRequest 认证请求
func (s *Server) authenticateRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	username, password, ok := r.BasicAuth()
	if !ok || !s.authenticate(username, password) {
		w.Header().Set("WWW-Authenticate", `Basic realm="NAS-OS WebDAV"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return username, true
}

// authenticate 认证用户
func (s *Server) authenticate(username, password string) bool {
	if s.authFunc != nil {
		return s.authFunc(username, password)
	}
	// 默认：没有认证函数时拒绝访问
	return false
}

// resolvePath 解析路径
func (s *Server) resolvePath(r *http.Request, requestPath string) (string, error) {
	// 解码 URL
	decodedPath, err := url.PathUnescape(requestPath)
	if err != nil {
		return "", err
	}

	// 清理路径
	cleanPath := filepath.Clean("/" + decodedPath)

	// 防止路径遍历攻击
	if strings.Contains(cleanPath, "..") {
		return "", ErrPathTraversal
	}

	// 获取用户主目录
	username := ""
	s.mu.RLock()
	if u, ok := s.userSessions[r.RemoteAddr]; ok {
		username = u
	}
	s.mu.RUnlock()

	basePath := s.config.RootPath
	if s.getUserHome != nil && username != "" {
		if home := s.getUserHome(username); home != "" {
			basePath = home
		}
	}

	return filepath.Join(basePath, cleanPath), nil
}

// handleWebDAV 处理 WebDAV 请求
func (s *Server) handleWebDAV(w http.ResponseWriter, r *http.Request, username string) {
	path := r.URL.Path
	fullPath, err := s.resolvePath(r, path)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "OPTIONS":
		s.handleOptions(w, r)
	case "PROPFIND":
		s.handlePropfind(w, r, fullPath)
	case "PROPPATCH":
		s.handleProppatch(w, r, fullPath)
	case "GET":
		s.handleGet(w, r, fullPath)
	case "PUT":
		s.handlePut(w, r, fullPath, username)
	case "DELETE":
		s.handleDelete(w, r, fullPath, username)
	case "MKCOL":
		s.handleMkcol(w, r, fullPath)
	case "COPY":
		s.handleCopy(w, r, fullPath, username)
	case "MOVE":
		s.handleMove(w, r, fullPath, username)
	case "HEAD":
		s.handleHead(w, r, fullPath)
	case "LOCK":
		s.handleLock(w, r, fullPath, username)
	case "UNLOCK":
		s.handleUnlock(w, r, fullPath)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleOptions 处理 OPTIONS 请求
func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 2, 3")
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, POST, PUT, DELETE, MKCOL, COPY, MOVE, PROPFIND, PROPPATCH, LOCK, UNLOCK")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusNoContent)
}

// handlePropfind 处理 PROPFIND 请求（列出目录/获取文件属性）
func (s *Server) handlePropfind(w http.ResponseWriter, r *http.Request, fullPath string) {
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "0"
	}

	// 解析请求体中的属性
	propFind := s.parsePropfind(r.Body)

	// 构建响应
	response := &Multistatus{
		XMLName: xml.Name{Space: "DAV:", Local: "multistatus"},
		Responses: []PropfindResponse{
			s.buildPropfindResponse(r.URL.Path, fullPath, info, depth == "1" && info.IsDir(), propFind),
		},
	}

	output, err := xml.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>`))
	_, _ = w.Write(output)
}

// parsePropfind 解析 PROPFIND 请求
func (s *Server) parsePropfind(body io.Reader) *PropFind {
	propFind := &PropFind{}
	if body == nil {
		return propFind
	}

	data, err := io.ReadAll(io.LimitReader(body, 1024*1024))
	if err != nil {
		return propFind
	}

	if len(data) == 0 {
		return propFind
	}

	_ = xml.Unmarshal(data, propFind)
	return propFind
}

// buildPropfindResponse 构建 PROPFIND 响应
func (s *Server) buildPropfindResponse(href, fullPath string, info os.FileInfo, recurse bool, propFind *PropFind) PropfindResponse {
	response := PropfindResponse{
		Href: href,
		PropStat: []PropStat{
			{
				Prop: Prop{
					Displayname:     info.Name(),
					GetLastModified: info.ModTime().UTC().Format(http.TimeFormat),
					ResourceType:    s.resourceTypeXML(info),
				},
				Status: "HTTP/1.1 200 OK",
			},
		},
	}

	if !info.IsDir() {
		response.PropStat[0].Prop.GetContentLength = info.Size()
		response.PropStat[0].Prop.GetContentType = "application/octet-stream"
		response.PropStat[0].Prop.GetETag = fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size())
	}

	// 添加锁信息
	if lock, exists := s.lockManager.GetLockByPath(href); exists {
		response.PropStat[0].Prop.LockDiscovery = &LockDiscovery{
			ActiveLock: &ActiveLock{
				LockType:  &LockType{},
				LockScope: &LockScope{},
				Depth:     lock.Depth,
				Owner:     &Owner{Href: lock.Owner},
				Timeout:   formatTimeout(lock.Timeout),
				LockToken: &LockToken{Href: lock.Token},
				LockRoot:  &LockRoot{Href: lock.Path},
			},
		}
		if lock.Scope == "exclusive" {
			response.PropStat[0].Prop.LockDiscovery.ActiveLock.LockScope.Exclusive = &struct{}{}
		} else {
			response.PropStat[0].Prop.LockDiscovery.ActiveLock.LockScope.Shared = &struct{}{}
		}
	}

	return response
}

// resourceTypeXML 获取资源类型 XML
func (s *Server) resourceTypeXML(info os.FileInfo) *ResourceType {
	if info.IsDir() {
		return &ResourceType{Collection: &struct{}{}}
	}
	return &ResourceType{}
}

// formatTimeout 格式化超时时间
func formatTimeout(t time.Time) string {
	if t.IsZero() {
		return "Infinite"
	}
	remaining := time.Until(t)
	if remaining <= 0 {
		return "Second-0"
	}
	return fmt.Sprintf("Second-%d", int(remaining.Seconds()))
}

// handleProppatch 处理 PROPPATCH 请求（设置文件属性）
func (s *Server) handleProppatch(w http.ResponseWriter, r *http.Request, fullPath string) {
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// 简化实现：读取请求但不实际修改属性
	// 大多数属性（如修改时间）需要操作系统级别的支持

	response := &Multistatus{
		XMLName: xml.Name{Space: "DAV:", Local: "multistatus"},
		Responses: []PropfindResponse{
			{
				Href: r.URL.Path,
				PropStat: []PropStat{
					{
						Prop: Prop{
							Displayname:     info.Name(),
							GetLastModified: info.ModTime().UTC().Format(http.TimeFormat),
						},
						Status: "HTTP/1.1 200 OK",
					},
				},
			},
		},
	}

	output, err := xml.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>`))
	_, _ = w.Write(output)
}

// handleGet 处理 GET 请求（下载文件）
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, fullPath string) {
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if info.IsDir() {
		// 对于目录，返回目录列表
		s.handlePropfind(w, r, fullPath)
		return
	}

	// 设置 Content-Disposition
	filename := filepath.Base(fullPath)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, fullPath)
}

// handleHead 处理 HEAD 请求
func (s *Server) handleHead(w http.ResponseWriter, r *http.Request, fullPath string) {
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if !info.IsDir() {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size()))
	}
	w.WriteHeader(http.StatusOK)
}

// handlePut 处理 PUT 请求（上传文件）
func (s *Server) handlePut(w http.ResponseWriter, r *http.Request, fullPath, username string) {
	// 检查锁
	if _, exists := s.lockManager.GetLockByPath(r.URL.Path); exists {
		ifToken := r.Header.Get("If")
		if ifToken == "" || !s.lockManager.ValidateToken(r.URL.Path, strings.Trim(ifToken, "<>")) {
			http.Error(w, "Locked", http.StatusLocked)
			return
		}
	}

	// 检查上传大小限制
	if s.config.MaxUploadSize > 0 {
		if r.ContentLength > s.config.MaxUploadSize {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
			return
		}
	}

	// 检查配额
	if s.quotaProvider != nil && username != "" && r.ContentLength > 0 {
		available, err := s.quotaProvider.CheckQuota(username)
		if err == nil && available >= 0 && r.ContentLength > available {
			http.Error(w, "Insufficient Storage", http.StatusInsufficientStorage)
			return
		}
	}

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 创建临时文件
	tmpPath := fullPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 复制内容
	written, err := io.Copy(file, r.Body)
	if err != nil {
		os.Remove(tmpPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 确保内容写入磁盘
	if err := file.Sync(); err != nil {
		os.Remove(tmpPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 重命名临时文件
	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 更新配额
	if s.quotaProvider != nil && username != "" && written > 0 {
		_ = s.quotaProvider.ConsumeQuota(username, written)
	}

	w.WriteHeader(http.StatusCreated)
}

// handleDelete 处理 DELETE 请求
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, fullPath, username string) {
	// 检查锁
	if _, exists := s.lockManager.GetLockByPath(r.URL.Path); exists {
		ifToken := r.Header.Get("If")
		if ifToken == "" || !s.lockManager.ValidateToken(r.URL.Path, strings.Trim(ifToken, "<>")) {
			http.Error(w, "Locked", http.StatusLocked)
			return
		}
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// 计算释放的空间
	var size int64
	if !info.IsDir() {
		size = info.Size()
	}

	if err := os.RemoveAll(fullPath); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 释放配额
	if s.quotaProvider != nil && username != "" && size > 0 {
		_ = s.quotaProvider.ReleaseQuota(username, size)
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleMkcol 处理 MKCOL 请求（创建目录）
func (s *Server) handleMkcol(w http.ResponseWriter, r *http.Request, fullPath string) {
	// 检查是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleCopy 处理 COPY 请求
func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request, fullPath, username string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 解析目标 URL
	destURL, err := url.Parse(dest)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	destPath, err := s.resolvePath(r, destURL.Path)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 检查源是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// 检查目标是否已存在
	overwrite := r.Header.Get("Overwrite") != "F"
	if _, err := os.Stat(destPath); err == nil {
		if !overwrite {
			http.Error(w, "Precondition Failed", http.StatusPreconditionFailed)
			return
		}
		// 删除现有目标
		os.RemoveAll(destPath)
	}

	// 执行复制
	if info.IsDir() {
		err = s.copyDir(fullPath, destPath)
	} else {
		err = s.copyFile(fullPath, destPath)
	}

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 返回状态
	if _, err := os.Stat(destPath); err == nil && overwrite {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

// copyFile 复制文件
func (s *Server) copyFile(src, dst string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 复制内容
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyDir 复制目录
func (s *Server) copyDir(src, dst string) error {
	// 创建目标目录
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// 读取源目录
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// 复制每个条目
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := s.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := s.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// handleMove 处理 MOVE 请求
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request, fullPath, username string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 解析目标 URL
	destURL, err := url.Parse(dest)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	destPath, err := s.resolvePath(r, destURL.Path)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 检查源是否存在
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// 检查目标是否已存在
	overwrite := r.Header.Get("Overwrite") != "F"
	if _, err := os.Stat(destPath); err == nil {
		if !overwrite {
			http.Error(w, "Precondition Failed", http.StatusPreconditionFailed)
			return
		}
		// 删除现有目标
		os.RemoveAll(destPath)
	}

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 执行移动
	if err := os.Rename(fullPath, destPath); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 返回状态
	if overwrite {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

// handleLock 处理 LOCK 请求
func (s *Server) handleLock(w http.ResponseWriter, r *http.Request, fullPath, username string) {
	// 解析锁请求
	var lockInfo struct {
		Owner struct {
			Href string `xml:"href"`
		} `xml:"owner"`
		LockScope struct {
			Exclusive *struct{} `xml:"exclusive"`
			Shared    *struct{} `xml:"shared"`
		} `xml:"lockscope"`
		LockType struct {
			Write *struct{} `xml:"write"`
		} `xml:"locktype"`
	}

	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err == nil && len(body) > 0 {
			_ = xml.Unmarshal(body, &lockInfo)
		}
	}

	// 确定锁类型
	scope := "exclusive"
	if lockInfo.LockScope.Shared != nil {
		scope = "shared"
	}

	// 解析超时
	timeout := parseTimeoutHeader(r.Header.Get("Timeout"))

	// 创建锁
	lock, err := s.lockManager.CreateLock(r.URL.Path, lockInfo.Owner.Href, 0, scope, timeout)
	if err != nil {
		if err == ErrLocked {
			// 返回现有的锁信息
			if existingLock, exists := s.lockManager.GetLockByPath(r.URL.Path); exists {
				s.writeLockResponse(w, existingLock, http.StatusLocked)
				return
			}
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 如果资源不存在，创建空文件
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err == nil {
			os.WriteFile(fullPath, []byte{}, 0644)
		}
	}

	s.writeLockResponse(w, lock, http.StatusOK)
}

// handleUnlock 处理 UNLOCK 请求
func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request, fullPath string) {
	// 从 Lock-Token 头获取令牌
	token := r.Header.Get("Lock-Token")
	if token == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 去除尖括号
	token = strings.Trim(token, "<>")

	// 验证并移除锁
	if err := s.lockManager.RemoveLock(token); err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeLockResponse 写入锁响应
func (s *Server) writeLockResponse(w http.ResponseWriter, lock *Lock, statusCode int) {
	activeLock := &ActiveLock{
		LockType:  &LockType{},
		Depth:     lock.Depth,
		Owner:     &Owner{Href: lock.Owner},
		Timeout:   formatTimeout(lock.Timeout),
		LockToken: &LockToken{Href: lock.Token},
		LockRoot:  &LockRoot{Href: lock.Path},
	}

	if lock.Scope == "exclusive" {
		activeLock.LockScope = &LockScope{Exclusive: &struct{}{}}
	} else {
		activeLock.LockScope = &LockScope{Shared: &struct{}{}}
	}

	response := &Prop{
		LockDiscovery: &LockDiscovery{ActiveLock: activeLock},
	}

	output, err := xml.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Lock-Token", "<"+lock.Token+">")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>`))
	_, _ = w.Write(output)
}

// parseTimeoutHeader 解析 Timeout 头
func parseTimeoutHeader(header string) int {
	if header == "" || header == "Infinite" {
		return 0 // 无限
	}

	// 格式: Second-3600
	if strings.HasPrefix(header, "Second-") {
		var seconds int
		_, _ = fmt.Sscanf(header, "Second-%d", &seconds)
		return seconds
	}

	return 60 // 默认 60 秒
}

// GetConfig 获取配置
func (s *Server) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig 更新配置
func (s *Server) UpdateConfig(config *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果端口变化，需要重启服务器
	if s.config.Port != config.Port && s.server != nil {
		s.server.Shutdown(context.Background())
		s.server = nil
	}

	s.config = config

	// 如果启用且服务器未运行，启动它
	if config.Enabled && s.server == nil {
		go func() {
			s.Start()
		}()
	}

	return nil
}

// GetStatus 获取服务器状态
func (s *Server) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"enabled":    s.config.Enabled,
		"port":       s.config.Port,
		"running":    s.server != nil,
		"root_path":  s.config.RootPath,
		"lock_count": len(s.lockManager.locks),
	}
}

// RegisterRoutes 注册 HTTP 路由（用于管理 API）
func (s *Server) RegisterRoutes(api *gin.RouterGroup) {
	webdav := api.Group("/webdav")
	{
		webdav.GET("/config", s.getConfigHandler)
		webdav.PUT("/config", s.updateConfigHandler)
		webdav.POST("/start", s.startHandler)
		webdav.POST("/stop", s.stopHandler)
		webdav.GET("/status", s.statusHandler)
		webdav.GET("/locks", s.getLocksHandler)
		webdav.DELETE("/locks/:token", s.deleteLockHandler)
	}
}

func (s *Server) getConfigHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    s.GetConfig(),
	})
}

func (s *Server) updateConfigHandler(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := s.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    s.GetConfig(),
	})
}

func (s *Server) startHandler(c *gin.Context) {
	config := s.GetConfig()
	config.Enabled = true
	if err := s.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "WebDAV 服务器已启动",
	})
}

func (s *Server) stopHandler(c *gin.Context) {
	config := s.GetConfig()
	config.Enabled = false
	if err := s.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	if err := s.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "WebDAV 服务器已停止",
	})
}

func (s *Server) statusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    s.GetStatus(),
	})
}

func (s *Server) getLocksHandler(c *gin.Context) {
	s.lockManager.mu.RLock()
	locks := make([]*Lock, 0, len(s.lockManager.locks))
	for _, lock := range s.lockManager.locks {
		locks = append(locks, lock)
	}
	s.lockManager.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    locks,
	})
}

func (s *Server) deleteLockHandler(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少锁令牌",
		})
		return
	}

	if err := s.lockManager.RemoveLock(token); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "锁已删除",
	})
}
