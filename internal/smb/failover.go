package smb

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// FailoverConfig 故障转移配置
type FailoverConfig struct {
	Enabled             bool   `json:"enabled"`               // 是否启用故障转移
	Mode                string `json:"mode"`                   // "primary_standby" | "load_balance"
	HeartbeatIntervalMs int    `json:"heartbeat_interval_ms"`  // 心跳间隔(毫秒)
	HeartbeatTimeoutMs  int    `json:"heartbeat_timeout_ms"`   // 心跳超时(毫秒)
	MaxRetries          int    `json:"max_retries"`           // 最大重试次数
	RetryIntervalMs     int    `json:"retry_interval_ms"`      // 重试间隔(毫秒)
	StateSyncIntervalMs int    `json:"state_sync_interval_ms"` // 状态同步间隔(毫秒)
	PreferredNode       string `json:"preferred_node"`         // 偏好节点ID
	ClusterIP           string `json:"cluster_ip"`            // 集群虚拟IP
	StateFilePath       string `json:"state_file_path"`       // 会话状态文件路径
}

// NodeState 节点状态
type NodeState struct {
	NodeID        string    `json:"node_id"`
	Host          string    `json:"host"`
	Role          string    `json:"role"`    // "primary" | "standby" | "unknown"
	Status        string    `json:"status"`  // "active" | "unhealthy" | "offline"
	Priority      int       `json:"priority"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	HealthScore   int       `json:"health_score"` // 0-100
	IsLocal       bool      `json:"is_local"`
}

// SMBSession SMB会话信息（用于状态追踪）
type SMBSession struct {
	SessionID     string            `json:"session_id"`
	ClientIP      string            `json:"client_ip"`
	ClientName    string            `json:"client_name"`
	Username      string            `json:"username"`
	ShareName     string            `json:"share_name"`
	Protocol      string            `json:"protocol"`
	TemporalFile  string            `json:"temporal_file,omitempty"` // 临时文件路径
	OpenedFiles   []string          `json:"opened_files,omitempty"`  // 打开的文件列表
	Locks         []FileLock        `json:"locks,omitempty"`         // 文件锁
	OplockLevel   string            `json:"oplock_level,omitempty"` // oplock级别
	ConnectedAt   time.Time         `json:"connected_at"`
	LastActiveAt  time.Time         `json:"last_active_at"`
	ExpiresAt     time.Time         `json:"expires_at,omitempty"` // 会话过期时间
	Authenticated bool              `json:"authenticated"`
	Encrypted     bool              `json:"encrypted"`
	Metadata      map[string]string `json:"metadata,omitempty"` // 额外元数据
}

// FileLock 文件锁信息
type FileLock struct {
	FilePath  string    `json:"file_path"`
	PID       int       `json:"pid"`
	Mode      string    `json:"mode"` // "read" | "write" | "read_write"
	Acquired  time.Time `json:"acquired"`
}

// SessionRegistry 会话注册表
type SessionRegistry struct {
	mu           sync.RWMutex
	sessions     map[string]*SMBSession       // key: session_id
	indexByIP    map[string][]string          // key: client_ip -> session_ids
	indexByUser  map[string][]string          // key: username -> session_ids
	indexByShare map[string][]string          // key: share_name -> session_ids
}

// FailoverState 故障转移状态管理器
type FailoverState struct {
	mu                sync.RWMutex
	config            *FailoverConfig
	localNode         *NodeState
	clusterNodes      map[string]*NodeState // node_id -> NodeState
	sessionRegistry   *SessionRegistry
	activeSessions    int
	failoverCount     int
	healthyCount      int
	lastFailover      time.Time
	lastStateSync     time.Time
	isPrimary         bool
	isRunning         bool
	stopChan          chan struct{}
	heartbeatChan     chan heartbeatMsg
	stateSyncChan     chan stateSyncMsg
}

// heartbeatMsg 心跳消息
type heartbeatMsg struct {
	FromNodeID  string
	Timestamp   time.Time
	HealthScore int
	Status      string
}

// stateSyncMsg 状态同步消息
type stateSyncMsg struct {
	Type      string      `json:"type"` // "session_update" | "node_update" | "full_sync"
	Session   *SMBSession `json:"session,omitempty"`
	NodeState *NodeState `json:"node_state,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// FailoverStatus 故障转移状态（用于API查询）
type FailoverStatus struct {
	Enabled        bool             `json:"enabled"`
	Mode           string           `json:"mode"`
	IsPrimary      bool             `json:"is_primary"`
	LocalNode      *NodeState       `json:"local_node"`
	ClusterNodes   []*NodeState     `json:"cluster_nodes"`
	ActiveSessions int              `json:"active_sessions"`
	FailoverCount  int              `json:"failover_count"`
	HealthyCount   int              `json:"healthy_count"`
	LastFailover   *time.Time       `json:"last_failover,omitempty"`
	LastStateSync  *time.Time       `json:"last_state_sync,omitempty"`
	IsRunning      bool             `json:"is_running"`
	Config         *FailoverConfig  `json:"config"`
}

// FailoverEvent 故障转移事件（用于事件通知）
type FailoverEvent struct {
	Type      string      `json:"type"` // "heartbeat_timeout" | "failover_start" | "failover_complete" | "session_recovery"
	NodeID    string      `json:"node_id,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Details   string      `json:"details,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// -------------------- 会话注册表 --------------------

// NewSessionRegistry 创建会话注册表
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions:     make(map[string]*SMBSession),
		indexByIP:    make(map[string][]string),
		indexByUser:  make(map[string][]string),
		indexByShare: make(map[string][]string),
	}
}

// Add 添加会话到注册表
func (r *SessionRegistry) Add(session *SMBSession) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[session.SessionID] = session

	if session.ClientIP != "" {
		r.indexByIP[session.ClientIP] = append(r.indexByIP[session.ClientIP], session.SessionID)
	}
	if session.Username != "" {
		r.indexByUser[session.Username] = append(r.indexByUser[session.Username], session.SessionID)
	}
	if session.ShareName != "" {
		r.indexByShare[session.ShareName] = append(r.indexByShare[session.ShareName], session.SessionID)
	}
}

// Get 根据会话ID获取会话
func (r *SessionRegistry) Get(sessionID string) (*SMBSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}
	return session, nil
}

// Remove 根据会话ID移除会话
func (r *SessionRegistry) Remove(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return
	}

	delete(r.sessions, sessionID)

	if session.ClientIP != "" {
		r.indexByIP[session.ClientIP] = filterStrings(r.indexByIP[session.ClientIP], sessionID)
	}
	if session.Username != "" {
		r.indexByUser[session.Username] = filterStrings(r.indexByUser[session.Username], sessionID)
	}
	if session.ShareName != "" {
		r.indexByShare[session.ShareName] = filterStrings(r.indexByShare[session.ShareName], sessionID)
	}
}

// ListAll 返回所有会话
func (r *SessionRegistry) ListAll() []*SMBSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SMBSession, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s)
	}
	return result
}

// GetByClient 按客户端IP查询
func (r *SessionRegistry) GetByClient(clientIP string) []*SMBSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.indexByIP[clientIP]
	return r.getByIDsLocked(ids)
}

// GetByUser 按用户名查询
func (r *SessionRegistry) GetByUser(username string) []*SMBSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.indexByUser[username]
	return r.getByIDsLocked(ids)
}

// GetByShare 按共享名查询
func (r *SessionRegistry) GetByShare(shareName string) []*SMBSession {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.indexByShare[shareName]
	return r.getByIDsLocked(ids)
}

func (r *SessionRegistry) getByIDsLocked(ids []string) []*SMBSession {
	result := make([]*SMBSession, 0, len(ids))
	for _, id := range ids {
		if s, ok := r.sessions[id]; ok {
			result = append(result, s)
		}
	}
	return result
}

// Size 返回会话总数
func (r *SessionRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// -------------------- 故障转移核心 --------------------

// NewFailoverState 创建故障转移状态管理器
func NewFailoverState(config *FailoverConfig) (*FailoverState, error) {
	if config == nil {
		config = DefaultFailoverConfig()
	}
	if err := ValidateFailoverConfig(config); err != nil {
		return nil, fmt.Errorf("验证故障转移配置失败: %w", err)
	}

	nodeID := getLocalNodeID()
	hostname, _ := os.Hostname()

	state := &FailoverState{
		config:          config,
		localNode: &NodeState{
			NodeID:        nodeID,
			Host:          hostname,
			Role:          "primary",
			Status:        "active",
			Priority:      100,
			LastHeartbeat: time.Now(),
			HealthScore:   100,
			IsLocal:       true,
		},
		clusterNodes:    make(map[string]*NodeState),
		sessionRegistry: NewSessionRegistry(),
		stopChan:        make(chan struct{}),
		heartbeatChan:   make(chan heartbeatMsg, 100),
		stateSyncChan:   make(chan stateSyncMsg, 100),
		isPrimary:       true,
		isRunning:       false,
	}

	if config.Enabled {
		if err := state.Start(); err != nil {
			return nil, fmt.Errorf("启动故障转移失败: %w", err)
		}
	}

	logInfo("故障转移状态管理器已创建", "node_id", nodeID, "enabled", config.Enabled)
	return state, nil
}

// DefaultFailoverConfig 返回默认配置
func DefaultFailoverConfig() *FailoverConfig {
	return &FailoverConfig{
		Enabled:             false,
		Mode:                "primary_standby",
		HeartbeatIntervalMs: 1000,
		HeartbeatTimeoutMs:  3000,
		MaxRetries:          3,
		RetryIntervalMs:     500,
		StateSyncIntervalMs: 5000,
		PreferredNode:       "",
		ClusterIP:           "",
		StateFilePath:       "/var/lib/samba/.failover_state.json",
	}
}

// ValidateFailoverConfig 验证故障转移配置
func ValidateFailoverConfig(config *FailoverConfig) error {
	if config.HeartbeatIntervalMs < 100 {
		return fmt.Errorf("心跳间隔至少100ms")
	}
	if config.HeartbeatIntervalMs > 60000 {
		return fmt.Errorf("心跳间隔最多60秒")
	}
	if config.HeartbeatTimeoutMs <= config.HeartbeatIntervalMs {
		return fmt.Errorf("心跳超时必须大于心跳间隔")
	}
	if config.MaxRetries < 1 {
		return fmt.Errorf("最大重试次数至少1")
	}
	if config.Mode != "primary_standby" && config.Mode != "load_balance" {
		return fmt.Errorf("模式必须是 primary_standby 或 load_balance")
	}
	return nil
}

// Start 启动故障转移管理
func (s *FailoverState) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}

	// 加载持久化的会话状态
	_ = s.loadState()

	// 启动心跳循环
	go s.heartbeatLoop()

	// 启动状态同步循环
	go s.stateSyncLoop()

	// 启动会话清理循环
	go s.sessionCleanupLoop()

	s.isRunning = true
	logInfo("故障转移管理已启动", "node_id", s.localNode.NodeID, "mode", s.config.Mode)
	return nil
}

// Stop 停止故障转移管理
func (s *FailoverState) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}

	close(s.stopChan)
	s.stopChan = make(chan struct{})

	// 保存会话状态
	_ = s.saveState()

	s.isRunning = false
	logInfo("故障转移管理已停止", "node_id", s.localNode.NodeID)
	return nil
}

// RegisterSession 注册一个SMB会话
func (s *FailoverState) RegisterSession(session *SMBSession) error {
	if session == nil {
		return fmt.Errorf("会话不能为空")
	}
	if session.SessionID == "" {
		return fmt.Errorf("会话ID不能为空")
	}

	s.sessionRegistry.Add(session)

	s.mu.Lock()
	s.activeSessions++
	s.mu.Unlock()

	// 触发状态同步
	select {
	case s.stateSyncChan <- stateSyncMsg{Type: "session_update", Session: session, Timestamp: time.Now()}:
	default:
	}

	logInfo("SMB会话已注册", "session_id", session.SessionID, "client_ip", session.ClientIP)
	return nil
}

// UnregisterSession 注销一个SMB会话
func (s *FailoverState) UnregisterSession(sessionID string) error {
	session, err := s.sessionRegistry.Get(sessionID)
	if err != nil {
		return err
	}

	s.sessionRegistry.Remove(sessionID)

	s.mu.Lock()
	if s.activeSessions > 0 {
		s.activeSessions--
	}
	s.mu.Unlock()

	select {
	case s.stateSyncChan <- stateSyncMsg{Type: "session_update", Session: session, Timestamp: time.Now()}:
	default:
	}

	logInfo("SMB会话已注销", "session_id", sessionID)
	return nil
}

// GetSession 获取会话信息
func (s *FailoverState) GetSession(sessionID string) (*SMBSession, error) {
	return s.sessionRegistry.Get(sessionID)
}

// ListSessions 列出所有会话
func (s *FailoverState) ListSessions() []*SMBSession {
	return s.sessionRegistry.ListAll()
}

// GetSessionsByClient 获取客户端的所有会话
func (s *FailoverState) GetSessionsByClient(clientIP string) []*SMBSession {
	return s.sessionRegistry.GetByClient(clientIP)
}

// GetSessionsByUser 获取用户的所有会话
func (s *FailoverState) GetSessionsByUser(username string) []*SMBSession {
	return s.sessionRegistry.GetByUser(username)
}

// GetSessionsByShare 获取共享的所有会话
func (s *FailoverState) GetSessionsByShare(shareName string) []*SMBSession {
	return s.sessionRegistry.GetByShare(shareName)
}

// UpdateSession 更新会话信息
func (s *FailoverState) UpdateSession(sessionID string, updater func(*SMBSession) error) error {
	session, err := s.sessionRegistry.Get(sessionID)
	if err != nil {
		return err
	}
	if err := updater(session); err != nil {
		return err
	}
	session.LastActiveAt = time.Now()

	select {
	case s.stateSyncChan <- stateSyncMsg{Type: "session_update", Session: session, Timestamp: time.Now()}:
	default:
	}
	return nil
}

// UpdateNodeHealth 更新节点健康状态
func (s *FailoverState) UpdateNodeHealth(nodeID string, healthScore int, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node, ok := s.clusterNodes[nodeID]; ok {
		node.HealthScore = healthScore
		node.Status = status
		node.LastHeartbeat = time.Now()
	} else if nodeID == s.localNode.NodeID {
		s.localNode.HealthScore = healthScore
		s.localNode.Status = status
		s.localNode.LastHeartbeat = time.Now()
	}
}

// GetStatus 获取故障转移状态（API查询用）
func (s *FailoverState) GetStatus() *FailoverStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]*NodeState, 0, len(s.clusterNodes)+1)
	nodes = append(nodes, s.localNode)
	for _, n := range s.clusterNodes {
		nodes = append(nodes, n)
	}

	var lastFailover, lastStateSync *time.Time
	if !s.lastFailover.IsZero() {
		t := s.lastFailover
		lastFailover = &t
	}
	if !s.lastStateSync.IsZero() {
		t := s.lastStateSync
		lastStateSync = &t
	}

	return &FailoverStatus{
		Enabled:        s.config.Enabled,
		Mode:           s.config.Mode,
		IsPrimary:      s.isPrimary,
		LocalNode:      s.localNode,
		ClusterNodes:   nodes,
		ActiveSessions: s.activeSessions,
		FailoverCount:  s.failoverCount,
		HealthyCount:   s.healthyCount,
		LastFailover:   lastFailover,
		LastStateSync:  lastStateSync,
		IsRunning:      s.isRunning,
		Config:         s.config,
	}
}

// heartbeatLoop 心跳循环
func (s *FailoverState) heartbeatLoop() {
	interval := time.Duration(s.config.HeartbeatIntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.performHeartbeat()
		}
	}
}

// performHeartbeat 执行一次心跳检测
func (s *FailoverState) performHeartbeat() {
	s.mu.Lock()
	s.localNode.LastHeartbeat = time.Now()
	s.localNode.HealthScore = s.checkLocalHealth()
	s.mu.Unlock()

	// 检查其他节点的心跳超时
	s.mu.RLock()
	for nodeID, node := range s.clusterNodes {
		elapsed := time.Since(node.LastHeartbeat)
		timeout := time.Duration(s.config.HeartbeatTimeoutMs) * time.Millisecond
		if elapsed > timeout {
			s.mu.RUnlock()
			s.handleHeartbeatTimeout(nodeID)
			s.mu.RLock()
		}
	}
	s.mu.RUnlock()

	s.broadcastHeartbeat()
}

// checkLocalHealth 检查本地健康状态，返回 0-100
func (s *FailoverState) checkLocalHealth() int {
	score := 100

	// 检查SMB服务
	if !s.isSMBServiceHealthy() {
		score -= 50
	}

	// 检查磁盘空间
	if ds := s.checkDiskSpace(); ds < score {
		score = ds
	}

	// 检查内存
	if ms := s.checkMemoryPressure(); ms < score {
		score = ms
	}

	return max(0, score)
}

func (s *FailoverState) isSMBServiceHealthy() bool {
	cmd := exec.CommandContext(context.Background(), "systemctl", "is-active", "smbd")
	return cmd.Run() == nil
}

func (s *FailoverState) checkDiskSpace() int {
	cmd := exec.CommandContext(context.Background(), "df", "-B1", "/")
	out, err := cmd.Output()
	if err != nil {
		return 100
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 100
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return 100
	}
	usage := strings.TrimSuffix(fields[4], "%")
	var percent int
	if _, err := fmt.Sscanf(usage, "%d", &percent); err != nil {
		return 100
	}
	if percent >= 95 {
		return 0
	}
	if percent >= 90 {
		return 20
	}
	if percent >= 80 {
		return 60
	}
	if percent >= 70 {
		return 80
	}
	return 100
}

func (s *FailoverState) checkMemoryPressure() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 100
	}
	var memTotal, memAvailable int64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		var val int64
		fmt.Sscanf(fields[1], "%d", &val)
		switch fields[0] {
		case "MemTotal:":
			memTotal = val * 1024
		case "MemAvailable:":
			memAvailable = val * 1024
		}
	}
	if memTotal == 0 {
		return 100
	}
	availPct := float64(memAvailable) / float64(memTotal) * 100
	if availPct < 5 {
		return 0
	}
	if availPct < 10 {
		return 20
	}
	if availPct < 20 {
		return 60
	}
	if availPct < 30 {
		return 80
	}
	return 100
}

// handleHeartbeatTimeout 处理心跳超时
func (s *FailoverState) handleHeartbeatTimeout(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.clusterNodes[nodeID]
	if !ok {
		return
	}
	node.Status = "unhealthy"
	logInfo("检测到节点心跳超时", "node_id", nodeID)

	if node.Role == "primary" {
		s.triggerFailoverLocked(nodeID)
	}
}

// triggerFailoverLocked 触发故障转移（调用者须持有锁）
func (s *FailoverState) triggerFailoverLocked(failedNodeID string) {
	if s.isPrimary {
		return
	}

	logInfo("触发故障转移", "failed_node", failedNodeID, "taking_over", s.localNode.NodeID)

	s.stateSyncChan <- stateSyncMsg{
		Type: "node_update",
		NodeState: &NodeState{
			NodeID:  s.localNode.NodeID,
			Role:    "primary",
			Status:  "active",
			IsLocal: true,
		},
		Timestamp: time.Now(),
	}

	// 执行会话恢复
	go func() {
		if err := s.recoverSessions(); err != nil {
			logError("会话恢复失败", err)
		}
	}()

	s.isPrimary = true
	s.localNode.Role = "primary"
	s.failoverCount++
	s.lastFailover = time.Now()
	logInfo("故障转移完成", "new_primary", s.localNode.NodeID)
}

// triggerFailover 触发故障转移（公开方法）
func (s *FailoverState) triggerFailover(failedNodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggerFailoverLocked(failedNodeID)
}

// recoverSessions 恢复会话（Stateful reconnect）
func (s *FailoverState) recoverSessions() error {
	sessions := s.sessionRegistry.ListAll()
	recovered, failed := 0, 0

	for _, session := range sessions {
		if err := s.recoverSession(session); err != nil {
			logError("恢复会话失败", err, "session_id", session.SessionID)
			failed++
		} else {
			recovered++
		}
	}

	logInfo("会话恢复完成", "recovered", recovered, "failed", failed)
	if failed > 0 && recovered == 0 {
		return fmt.Errorf("所有会话恢复失败")
	}
	return nil
}

// recoverSession 恢复单个会话
func (s *FailoverState) recoverSession(session *SMBSession) error {
	// 1. 验证客户端是否仍然连接
	if !s.isClientConnected(session.ClientIP) {
		s.sessionRegistry.Remove(session.SessionID)
		return fmt.Errorf("客户端已断开: %s", session.ClientIP)
	}

	// 2. 验证文件句柄是否仍然有效
	for _, filePath := range session.OpenedFiles {
		if filePath != "" {
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				logInfo("文件不再可访问", "session_id", session.SessionID, "file", filePath)
			}
		}
	}

	// 3. 重新建立锁状态
	for _, lock := range session.Locks {
		if err := s.reacquireLock(lock); err != nil {
			logError("重新获取锁失败", err, "session_id", session.SessionID)
		}
	}

	session.LastActiveAt = time.Now()
	session.ConnectedAt = time.Now()
	logInfo("会话已恢复", "session_id", session.SessionID, "client_ip", session.ClientIP)
	return nil
}

// isClientConnected 检查客户端是否仍然活跃
func (s *FailoverState) isClientConnected(clientIP string) bool {
	cmd := exec.CommandContext(context.Background(), "ss", "-tnp")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, clientIP) && strings.Contains(line, ":445") {
			return true
		}
	}
	return false
}

// reacquireLock 重新获取文件锁
func (s *FailoverState) reacquireLock(lock FileLock) error {
	// 通知smbd重新获取锁
	cmd := exec.CommandContext(context.Background(), "smbcontrol", "smbd", "debug", "reacquire-lock")
	_ = cmd.Run()
	return nil
}

// stateSyncLoop 状态同步循环
func (s *FailoverState) stateSyncLoop() {
	interval := time.Duration(s.config.StateSyncIntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.performStateSync()
		case msg := <-s.stateSyncChan:
			s.handleStateSync(msg)
		}
	}
}

// performStateSync 执行状态同步
func (s *FailoverState) performStateSync() {
	s.mu.Lock()
	s.lastStateSync = time.Now()
	s.mu.Unlock()

	if err := s.saveState(); err != nil {
		logError("状态同步保存失败", err)
	}
}

// handleStateSync 处理状态同步消息
func (s *FailoverState) handleStateSync(msg stateSyncMsg) {
	switch msg.Type {
	case "session_update":
		if msg.Session != nil {
			s.sessionRegistry.Add(msg.Session)
		}
	case "node_update":
		if msg.NodeState != nil && !msg.NodeState.IsLocal {
			s.mu.Lock()
			s.clusterNodes[msg.NodeState.NodeID] = msg.NodeState
			s.mu.Unlock()
		}
	case "full_sync":
		s.mu.Lock()
		s.clusterNodes = make(map[string]*NodeState)
		s.mu.Unlock()
	}
}

// broadcastHeartbeat 向集群广播心跳
func (s *FailoverState) broadcastHeartbeat() {
	msg := heartbeatMsg{
		FromNodeID:  s.localNode.NodeID,
		Timestamp:   time.Now(),
		HealthScore: s.localNode.HealthScore,
		Status:      s.localNode.Status,
	}
	select {
	case s.heartbeatChan <- msg:
	default:
		// channel满，跳过
	}
}

// sessionCleanupLoop 会话清理循环
func (s *FailoverState) sessionCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions 清理过期的会话
func (s *FailoverState) cleanupExpiredSessions() {
	sessions := s.sessionRegistry.ListAll()
	now := time.Now()
	cleaned := 0

	for _, session := range sessions {
		if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
			s.sessionRegistry.Remove(session.SessionID)
			cleaned++
			continue
		}
		if now.Sub(session.LastActiveAt) > 30*time.Minute {
			if !s.isClientConnected(session.ClientIP) {
				s.sessionRegistry.Remove(session.SessionID)
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		logInfo("清理过期会话", "count", cleaned)
	}
}

// saveState 保存状态到文件
func (s *FailoverState) saveState() error {
	if s.config.StateFilePath == "" {
		return nil
	}

	type persistState struct {
		Sessions      []*SMBSession       `json:"sessions"`
		LocalNode     *NodeState          `json:"local_node"`
		ClusterNodes  map[string]*NodeState `json:"cluster_nodes"`
		FailoverCount int                 `json:"failover_count"`
		SavedAt       time.Time           `json:"saved_at"`
	}

	s.mu.RLock()
	state := &persistState{
		Sessions:      s.sessionRegistry.ListAll(),
		LocalNode:     s.localNode,
		ClusterNodes:  s.clusterNodes,
		FailoverCount: s.failoverCount,
		SavedAt:       time.Now(),
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}

	dir := s.config.StateFilePath
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' {
			dir = dir[:i]
			break
		}
	}
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0750)
	}

	if err := os.WriteFile(s.config.StateFilePath, data, 0600); err != nil {
		return fmt.Errorf("写入状态文件失败: %w", err)
	}

	return nil
}

// loadState 从文件加载状态
func (s *FailoverState) loadState() error {
	if s.config.StateFilePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.config.StateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取状态文件失败: %w", err)
	}

	type persistState struct {
		Sessions      []*SMBSession        `json:"sessions"`
		LocalNode     *NodeState           `json:"local_node"`
		ClusterNodes  map[string]*NodeState `json:"cluster_nodes"`
		FailoverCount int                 `json:"failover_count"`
		SavedAt       time.Time           `json:"saved_at"`
	}

	var state persistState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("解析状态文件失败: %w", err)
	}

	for _, session := range state.Sessions {
		session.LastActiveAt = time.Now()
		s.sessionRegistry.Add(session)
	}

	s.mu.Lock()
	if state.ClusterNodes != nil {
		s.clusterNodes = state.ClusterNodes
	}
	s.failoverCount = state.FailoverCount
	s.mu.Unlock()

	logInfo("已加载故障转移状态", "sessions", len(state.Sessions), "saved_at", state.SavedAt)
	return nil
}

// UpdateConfig 更新故障转移配置
func (s *FailoverState) UpdateConfig(config *FailoverConfig) error {
	if err := ValidateFailoverConfig(config); err != nil {
		return err
	}

	s.mu.Lock()
	wasEnabled := s.config.Enabled
	s.config = config
	s.mu.Unlock()

	if config.Enabled && !wasEnabled {
		return s.Start()
	}
	if !config.Enabled && wasEnabled {
		return s.Stop()
	}
	return nil
}

// IsPrimary 检查本地节点是否为主节点
func (s *FailoverState) IsPrimary() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isPrimary
}

// GetActiveSessions 返回当前活跃会话数
func (s *FailoverState) GetActiveSessions() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeSessions
}

// GetFailoverCount 返回累计故障转移次数
func (s *FailoverState) GetFailoverCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failoverCount
}

// -------------------- 工具函数 --------------------

// getLocalNodeID 获取本地节点唯一ID
func getLocalNodeID() string {
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Sprintf("node-%d", time.Now().UnixNano())
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) > 0 {
			return fmt.Sprintf("node-%s", iface.HardwareAddr.String())
		}
	}
	return fmt.Sprintf("node-%d", time.Now().UnixNano())
}

// filterStrings 从字符串切片中移除指定元素
func filterStrings(src []string, target string) []string {
	result := make([]string, 0, len(src))
	for _, s := range src {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}

// max returns the larger of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
