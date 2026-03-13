package webdav

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/studio-b12/gowebdav"
)

// Config WebDAV 服务器配置
type Config struct {
	Enabled    bool   `json:"enabled"`
	Port       int    `json:"port"`
	RootPath   string `json:"root_path"`
	AllowGuest bool   `json:"allow_guest"` // 允许匿名访问
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:    true,
		Port:       8081,
		RootPath:   "/data",
		AllowGuest: false,
	}
}

// Server WebDAV 服务器
type Server struct {
	mu       sync.RWMutex
	config   *Config
	server   *http.Server
	fs       *gowebdav.Client
	authFunc func(username, password string) bool // 认证函数
}

// NewServer 创建 WebDAV 服务器
func NewServer(config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Server{
		config: config,
	}

	return s, nil
}

// SetAuthFunc 设置认证函数
func (s *Server) SetAuthFunc(fn func(username, password string) bool) {
	s.authFunc = fn
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

	// 启动服务器
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("WebDAV 服务器错误：%v\n", err)
		}
	}()

	return nil
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
		if !s.config.AllowGuest {
			username, password, ok := r.BasicAuth()
			if !ok || !s.authenticate(username, password) {
				w.Header().Set("WWW-Authenticate", `Basic realm="NAS-OS WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// 处理 WebDAV 方法
		s.handleWebDAV(w, r)
	})

	return mux
}

// authenticate 认证用户
func (s *Server) authenticate(username, password string) bool {
	if s.authFunc != nil {
		return s.authFunc(username, password)
	}
	// 默认：没有认证函数时拒绝访问
	return false
}

// handleWebDAV 处理 WebDAV 请求
func (s *Server) handleWebDAV(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	fullPath := filepath.Join(s.config.RootPath, path)

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
		s.handlePut(w, r, fullPath)
	case "DELETE":
		s.handleDelete(w, r, fullPath)
	case "MKCOL":
		s.handleMkcol(w, r, fullPath)
	case "COPY":
		s.handleCopy(w, r, fullPath)
	case "MOVE":
		s.handleMove(w, r, fullPath)
	case "HEAD":
		s.handleHead(w, r, fullPath)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleOptions 处理 OPTIONS 请求
func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("DAV", "1, 2")
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, POST, PUT, DELETE, MKCOL, COPY, MOVE, PROPFIND, PROPPATCH")
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

	// 构建响应 XML
	xml := `<?xml version="1.0" encoding="utf-8"?>`
	xml += `<D:multistatus xmlns:D="DAV:">`
	xml += s.propfindResponse(fullPath, info, depth == "1" && info.IsDir())
	xml += `</D:multistatus>`

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	w.Write([]byte(xml))
}

// propfindResponse 生成单个项目的 PROPFIND 响应
func (s *Server) propfindResponse(path string, info os.FileInfo, recurse bool) string {
	xml := `<D:response>`
	xml += fmt.Sprintf(`<D:href>%s</D:href>`, path)
	xml += `<D:propstat>`
	xml += `<D:prop>`
	xml += fmt.Sprintf(`<D:displayname>%s</D:displayname>`, info.Name())
	xml += fmt.Sprintf(`<D:getlastmodified>%s</D:getlastmodified>`, info.ModTime().Format(http.TimeFormat))
	if !info.IsDir() {
		xml += fmt.Sprintf(`<D:getcontentlength>%d</D:getcontentlength>`, info.Size())
	}
	xml += fmt.Sprintf(`<D:resourcetype>%s</D:resourcetype>`, s.resourceType(info))
	xml += `</D:prop>`
	xml += `<D:status>HTTP/1.1 200 OK</D:status>`
	xml += `</D:propstat>`
	xml += `</D:response>`

	// 递归列出子目录
	if recurse && info.IsDir() {
		entries, err := os.ReadDir(path)
		if err == nil {
			for _, entry := range entries {
				entryInfo, err := entry.Info()
				if err == nil {
					xml += s.propfindResponse(filepath.Join(path, entry.Name()), entryInfo, false)
				}
			}
		}
	}

	return xml
}

// resourceType 获取资源类型 XML
func (s *Server) resourceType(info os.FileInfo) string {
	if info.IsDir() {
		return `<D:collection/>`
	}
	return ``
}

// handleProppatch 处理 PROPPATCH 请求（设置文件属性）
func (s *Server) handleProppatch(w http.ResponseWriter, r *http.Request, fullPath string) {
	// 简化实现：不支持属性修改
	w.WriteHeader(http.StatusNotImplemented)
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
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

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
	}
	w.WriteHeader(http.StatusOK)
}

// handlePut 处理 PUT 请求（上传文件）
func (s *Server) handlePut(w http.ResponseWriter, r *http.Request, fullPath string) {
	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 创建文件
	file, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 复制内容
	_, err = file.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleDelete 处理 DELETE 请求
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, fullPath string) {
	if err := os.RemoveAll(fullPath); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleMkcol 处理 MKCOL 请求（创建目录）
func (s *Server) handleMkcol(w http.ResponseWriter, r *http.Request, fullPath string) {
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleCopy 处理 COPY 请求
func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request, fullPath string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 简化实现：不支持远程复制
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

// handleMove 处理 MOVE 请求
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request, fullPath string) {
	dest := r.Header.Get("Destination")
	if dest == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 简化实现：不支持远程移动
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
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

// RegisterRoutes 注册 HTTP 路由（用于管理 API）
func (s *Server) RegisterRoutes(api *gin.RouterGroup) {
	webdav := api.Group("/webdav")
	{
		webdav.GET("/config", s.getConfigHandler)
		webdav.PUT("/config", s.updateConfigHandler)
		webdav.POST("/start", s.startHandler)
		webdav.POST("/stop", s.stopHandler)
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
