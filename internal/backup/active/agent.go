// Package active 远程备份代理通信模块
// 支持 Windows/Linux/Mac 客户端代理注册、心跳和数据传输
package active

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// AgentPlatform 代理平台类型.
type AgentPlatform string

const (
	PlatformWindows AgentPlatform = "windows"
	PlatformLinux   AgentPlatform = "linux"
	PlatformMac     AgentPlatform = "macos"
)

// AgentStatus 代理状态.
type AgentStatus string

const (
	AgentStatusOnline  AgentStatus = "online"
	AgentStatusOffline AgentStatus = "offline"
	AgentStatusBusy    AgentStatus = "busy"
	AgentStatusError   AgentStatus = "error"
)

// AgentInfo 远程代理信息.
type AgentInfo struct {
	ID            string            `json:"id"`
	Hostname      string            `json:"hostname"`
	Platform      AgentPlatform     `json:"platform"`
	OSVersion     string            `json:"os_version"`
	AgentVersion  string            `json:"agent_version"`
	IPAddress     string            `json:"ip_address"`
	Capabilities  []string          `json:"capabilities"` // "file_backup", "disk_image", "database", "vm"
	Status        AgentStatus       `json:"status"`
	LastSeen      time.Time         `json:"last_seen"`
	ConnectedAt   time.Time         `json:"connected_at"`
	Labels        map[string]string `json:"labels"`
	BytesSent     int64             `json:"bytes_sent"`
	BytesReceived int64             `json:"bytes_received"`
	ActiveJobs    int               `json:"active_jobs"`
}

// AgentMessage 代理通信消息.
type AgentMessage struct {
	Type      string          `json:"type"` // "register", "heartbeat", "backup_data", "restore_request", "status_update"
	ID        string          `json:"id"`
	AgentID   string          `json:"agent_id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// RegisterPayload 注册请求.
type RegisterPayload struct {
	Hostname     string            `json:"hostname"`
	Platform     AgentPlatform     `json:"platform"`
	OSVersion    string            `json:"os_version"`
	AgentVersion string            `json:"agent_version"`
	Capabilities []string          `json:"capabilities"`
	Labels       map[string]string `json:"labels"`
}

// HeartbeatPayload 心跳请求.
type HeartbeatPayload struct {
	Status      AgentStatus `json:"status"`
	ActiveJobs  int         `json:"active_jobs"`
	CPUUsage    float64     `json:"cpu_usage"`
	MemoryUsage float64     `json:"memory_usage"`
	DiskFree    int64       `json:"disk_free"`
}

// BackupDataPayload 备份数据传输.
type BackupDataPayload struct {
	JobID      string `json:"job_id"`
	SnapshotID string `json:"snapshot_id"`
	ChunkIndex int    `json:"chunk_index"`
	ChunkTotal int    `json:"chunk_total"`
	DataOffset int64  `json:"data_offset"`
	DataSize   int64  `json:"data_size"`
	Checksum   string `json:"checksum"`
	IsLast     bool   `json:"is_last"`
}

// RestoreRequestPayload 恢复请求.
type RestoreRequestPayload struct {
	JobID       string         `json:"job_id"`
	SnapshotID  string         `json:"snapshot_id"`
	TargetPaths []string       `json:"target_paths"`
	Options     RestoreOptions `json:"options"`
}

// RestoreOptions 恢复选项.
type RestoreOptions struct {
	OverwriteExisting bool   `json:"overwrite_existing"`
	RestoreACL        bool   `json:"restore_acl"`
	RestoreTimestamps bool   `json:"restore_timestamps"`
	TargetPath        string `json:"target_path"`
}

// AgentConnection 代理 WebSocket 连接.
type AgentConnection struct {
	Agent     *AgentInfo
	Conn      *websocket.Conn
	SendCh    chan []byte
	mu        sync.Mutex
	closeOnce sync.Once
}

// AgentRegistry 代理注册表.
type AgentRegistry struct {
	mu          sync.RWMutex
	agents      map[string]*AgentInfo       // agentID -> info
	connections map[string]*AgentConnection // agentID -> connection
	logger      *zap.Logger
}

// NewAgentRegistry 创建代理注册表.
func NewAgentRegistry(logger *zap.Logger) *AgentRegistry {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentRegistry{
		agents:      make(map[string]*AgentInfo),
		connections: make(map[string]*AgentConnection),
		logger:      logger,
	}
}

// Register 注册代理.
func (r *AgentRegistry) Register(payload *RegisterPayload, conn *websocket.Conn, ip string) (*AgentInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	agentID := uuid.New().String()
	agent := &AgentInfo{
		ID:           agentID,
		Hostname:     payload.Hostname,
		Platform:     payload.Platform,
		OSVersion:    payload.OSVersion,
		AgentVersion: payload.AgentVersion,
		IPAddress:    ip,
		Capabilities: payload.Capabilities,
		Status:       AgentStatusOnline,
		LastSeen:     time.Now(),
		ConnectedAt:  time.Now(),
		Labels:       payload.Labels,
	}
	if agent.Labels == nil {
		agent.Labels = make(map[string]string)
	}
	if agent.Capabilities == nil {
		agent.Capabilities = make([]string, 0)
	}

	r.agents[agentID] = agent

	if conn != nil {
		r.connections[agentID] = &AgentConnection{
			Agent:  agent,
			Conn:   conn,
			SendCh: make(chan []byte, 256),
		}
	}

	r.logger.Info("代理注册成功",
		zap.String("agent_id", agentID),
		zap.String("hostname", payload.Hostname),
		zap.String("platform", string(payload.Platform)))

	return agent, nil
}

// Unregister 注销代理.
func (r *AgentRegistry) Unregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, ok := r.connections[agentID]; ok {
		conn.closeOnce.Do(func() {
			close(conn.SendCh)
			conn.Conn.Close()
		})
		delete(r.connections, agentID)
	}

	if agent, ok := r.agents[agentID]; ok {
		agent.Status = AgentStatusOffline
	}

	r.logger.Info("代理已注销", zap.String("agent_id", agentID))
}

// GetAgent 获取代理信息.
func (r *AgentRegistry) GetAgent(agentID string) (*AgentInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("代理 %s 不存在", agentID)
	}
	return agent, nil
}

// ListAgents 列出所有代理.
func (r *AgentRegistry) ListAgents(status AgentStatus) []*AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*AgentInfo, 0, len(r.agents))
	for _, agent := range r.agents {
		if status == "" || agent.Status == status {
			result = append(result, agent)
		}
	}
	return result
}

// Count 返回在线代理数量.
func (r *AgentRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, a := range r.agents {
		if a.Status == AgentStatusOnline || a.Status == AgentStatusBusy {
			count++
		}
	}
	return count
}

// Heartbeat 处理代理心跳.
func (r *AgentRegistry) Heartbeat(agentID string, payload *HeartbeatPayload) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("代理 %s 不存在", agentID)
	}

	agent.LastSeen = time.Now()
	agent.Status = payload.Status
	agent.ActiveJobs = payload.ActiveJobs

	return nil
}

// SendMessage 向代理发送消息.
func (r *AgentRegistry) SendMessage(agentID string, msg *AgentMessage) error {
	r.mu.RLock()
	conn, exists := r.connections[agentID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("代理 %s 无连接", agentID)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	select {
	case conn.SendCh <- data:
		return nil
	default:
		return fmt.Errorf("代理 %s 发送缓冲区已满", agentID)
	}
}

// CleanupStale 清理超时代理.
func (r *AgentRegistry) CleanupStale(timeout time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-timeout)
	removed := 0

	for id, agent := range r.agents {
		if agent.LastSeen.Before(cutoff) && agent.Status != AgentStatusOffline {
			agent.Status = AgentStatusOffline
			if conn, ok := r.connections[id]; ok {
				conn.closeOnce.Do(func() {
					close(conn.SendCh)
					conn.Conn.Close()
				})
				delete(r.connections, id)
			}
			removed++
			r.logger.Info("清理超时代理", zap.String("agent_id", id))
		}
	}

	return removed
}

// AgentHandler 代理 HTTP API 处理器.
type AgentHandler struct {
	registry *AgentRegistry
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

// NewAgentHandler 创建代理 API 处理器.
func NewAgentHandler(registry *AgentRegistry, logger *zap.Logger) *AgentHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentHandler{
		registry: registry,
		logger:   logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// RegisterRoutes 注册代理 API 路由到 gin 路由组.
func (h *AgentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	agents := rg.Group("/agents")
	{
		agents.GET("", h.listAgents)
		agents.GET("/:agentId", h.getAgent)
		agents.DELETE("/:agentId", h.deleteAgent)
		agents.GET("/:agentId/ws", h.handleWebSocket)
		agents.POST("/:agentId/heartbeat", h.handleHeartbeat)
	}
}

func (h *AgentHandler) listAgents(c *gin.Context) {
	var status AgentStatus
	if s := c.Query("status"); s != "" {
		status = AgentStatus(s)
	}
	agents := h.registry.ListAgents(status)
	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"total":  len(agents),
	})
}

func (h *AgentHandler) getAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	agent, err := h.registry.GetAgent(agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *AgentHandler) deleteAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	h.registry.Unregister(agentID)
	c.JSON(http.StatusOK, gin.H{"status": "unregistered"})
}

func (h *AgentHandler) handleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket 升级失败", zap.Error(err))
		return
	}

	var agentID string
	defer func() {
		if agentID != "" {
			h.registry.Unregister(agentID)
		}
		conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Error("WebSocket 读取错误", zap.Error(err))
			}
			break
		}

		var msg AgentMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			h.logger.Error("解析消息失败", zap.Error(err))
			continue
		}

		switch msg.Type {
		case "register":
			var payload RegisterPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				h.logger.Error("解析注册载荷失败", zap.Error(err))
				continue
			}
			ip := c.ClientIP()
			agent, err := h.registry.Register(&payload, conn, ip)
			if err != nil {
				h.logger.Error("代理注册失败", zap.Error(err))
				continue
			}
			agentID = agent.ID

			resp, _ := json.Marshal(AgentMessage{
				Type:      "register_ack",
				ID:        uuid.New().String(),
				AgentID:   agent.ID,
				Timestamp: time.Now(),
			})
			conn.WriteMessage(websocket.TextMessage, resp)

		case "heartbeat":
			var payload HeartbeatPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				continue
			}
			h.registry.Heartbeat(msg.AgentID, &payload)

		case "backup_data":
			h.logger.Debug("收到备份数据", zap.String("agent_id", msg.AgentID))

		case "status_update":
			h.logger.Debug("代理状态更新", zap.String("agent_id", msg.AgentID))
		}
	}
}

func (h *AgentHandler) handleHeartbeat(c *gin.Context) {
	agentID := c.Param("agentId")
	var payload HeartbeatPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求体"})
		return
	}
	if err := h.registry.Heartbeat(agentID, &payload); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
