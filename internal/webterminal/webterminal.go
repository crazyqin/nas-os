// Package webterminal 提供 Web 终端（WebSocket SSH）功能
// Version: v1.0.0 - Web 终端
package webterminal

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// TerminalSession 终端会话
type TerminalSession struct {
	ID        string    `json:"id"`
	User      string    `json:"user"`
	StartedAt time.Time `json:"startedAt"`
	LastActive time.Time `json:"lastActive"`
	Status    string    `json:"status"` // active, closed
	Remote    string    `json:"remote"`
	Cols      int       `json:"cols"`
	Rows      int       `json:"rows"`
}

// TerminalConfig 终端配置
type TerminalConfig struct {
	MaxSessions   int           `json:"maxSessions"`
	IdleTimeout   time.Duration `json:"idleTimeout"`
	AllowRoot     bool          `json:"allowRoot"`
	DefaultShell  string        `json:"defaultShell"`
	AllowedUsers  []string      `json:"allowedUsers"`
	CommandWhitelist []string   `json:"commandWhitelist"`
}

// Manager 终端管理器
type Manager struct {
	logger    *zap.Logger
	mu        sync.RWMutex
	sessions  map[string]*TerminalSession
	config    TerminalConfig
	upgrader  websocket.Upgrader
}

// NewManager 创建终端管理器
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:   logger,
		sessions: make(map[string]*TerminalSession),
		config: TerminalConfig{
			MaxSessions:  10,
			IdleTimeout:  30 * time.Minute,
			DefaultShell: "/bin/bash",
		},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ListSessions 列出所有活跃会话
func (m *Manager) ListSessions() []TerminalSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []TerminalSession
	for _, s := range m.sessions {
		if s.Status == "active" {
			sessions = append(sessions, *s)
		}
	}
	return sessions
}

// CloseSession 关闭终端会话
func (m *Manager) CloseSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}
	session.Status = "closed"
	m.logger.Info("关闭终端会话", zap.String("id", id))
	return nil
}

// GetSession 获取会话信息
func (m *Manager) GetSession(id string) (*TerminalSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在", id)
	}
	return session, nil
}

// GetConfig 获取终端配置
func (m *Manager) GetConfig() TerminalConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新终端配置
func (m *Manager) UpdateConfig(config TerminalConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	m.logger.Info("更新终端配置")
}

// CleanupIdle 清理空闲会话
func (m *Manager) CleanupIdle() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleaned := 0
	now := time.Now()
	for id, s := range m.sessions {
		if s.Status == "active" && now.Sub(s.LastActive) > m.config.IdleTimeout {
			s.Status = "closed"
			cleaned++
			m.logger.Info("清理空闲终端会话", zap.String("id", id))
		}
	}
	return cleaned
}

// ActiveCount 活跃会话数
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, s := range m.sessions {
		if s.Status == "active" {
			count++
		}
	}
	return count
}

// HandleWebSocket 处理 WebSocket 终端连接
func (m *Manager) HandleWebSocket(c *gin.Context) {
	// 检查会话数限制
	if m.ActiveCount() >= m.config.MaxSessions {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": -1, "message": "终端会话数已达上限"})
		return
	}

	conn, err := m.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		m.logger.Error("WebSocket 升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	sessionID := fmt.Sprintf("term-%d", time.Now().UnixNano())
	session := &TerminalSession{
		ID:         sessionID,
		User:       c.GetString("user"),
		StartedAt:  time.Now(),
		LastActive: time.Now(),
		Status:     "active",
		Remote:     c.ClientIP(),
		Cols:       80,
		Rows:       24,
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	m.logger.Info("终端会话建立", zap.String("id", sessionID), zap.String("remote", session.Remote))

	// 确定 shell
	shell := m.config.DefaultShell
	if shell == "" {
		shell = "/bin/bash"
		if _, err := os.Stat(shell); err != nil {
			shell = "/bin/sh"
		}
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TERM=xterm-256color"),
		fmt.Sprintf("COLUMNS=%d", session.Cols),
		fmt.Sprintf("LINES=%d", session.Rows),
	)

	// 创建 PTY（简化实现，使用 stdin/stdout pipe）
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		m.logger.Error("启动 shell 失败", zap.Error(err))
		conn.WriteMessage(websocket.TextMessage, []byte("启动终端失败: "+err.Error()+"\r\n"))
		return
	}

	// stdout → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// stderr → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// WebSocket → stdin
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			session.LastActive = time.Now()

			// 处理 resize 命令
			if len(message) > 0 && message[0] == 0x1b {
				// 忽略转义序列
				continue
			}

			stdin.Write(message)
		}
	}()

	cmd.Wait()

	m.mu.Lock()
	session.Status = "closed"
	m.mu.Unlock()

	m.logger.Info("终端会话结束", zap.String("id", sessionID))
}

// Handlers HTTP 处理器
type Handlers struct {
	logger  *zap.Logger
	manager *Manager
}

// NewHandlers 创建终端 HTTP 处理器
func NewHandlers(logger *zap.Logger, manager *Manager) *Handlers {
	return &Handlers{logger: logger, manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	term := rg.Group("/terminal")
	{
		term.GET("/ws", h.manager.HandleWebSocket)
		term.GET("/sessions", h.listSessions)
		term.GET("/sessions/:id", h.getSession)
		term.DELETE("/sessions/:id", h.closeSession)
		term.GET("/config", h.getConfig)
		term.PUT("/config", h.updateConfig)
		term.POST("/cleanup", h.cleanup)
	}
}

func (h *Handlers) listSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": sessions, "active": h.manager.ActiveCount()})
}

func (h *Handlers) getSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": session})
}

func (h *Handlers) closeSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.CloseSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "会话已关闭"})
}

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": config})
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var config TerminalConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的配置参数"})
		return
	}
	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置已更新"})
}

func (h *Handlers) cleanup(c *gin.Context) {
	cleaned := h.manager.CleanupIdle()
	c.JSON(http.StatusOK, gin.H{"code": 0, "cleaned": cleaned})
}
