// manager.go - 远程会话管理
package remoteassist

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 远程会话管理器.
type Manager struct {
	config     *Config
	sessions   map[string]*Session
	screens    map[string]*ScreenShare
	terminals  map[string]*TerminalSession
	transfers  map[string]*FileTransfer
	recordings map[string]*Recording
	chatMsgs   map[string][]*ChatMessage
	audits     []*AuditEvent
	auth       *AuthManager
	recorder   *Recorder
	chat       *ChatService
	audit      *AuditService
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewManager 创建会话管理器.
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	mgr := &Manager{
		config:     cfg,
		sessions:   make(map[string]*Session),
		screens:    make(map[string]*ScreenShare),
		terminals:  make(map[string]*TerminalSession),
		transfers:  make(map[string]*FileTransfer),
		recordings: make(map[string]*Recording),
		chatMsgs:   make(map[string][]*ChatMessage),
		audits:     make([]*AuditEvent, 0),
		ctx:        ctx,
		cancel:     cancel,
	}

	// 初始化子服务
	mgr.auth = NewAuthManager(cfg.Security)
	mgr.recorder = NewRecorder(cfg.Recording)
	mgr.chat = NewChatService()
	mgr.audit = NewAuditService()

	// 启动清理任务
	go mgr.cleanupTask()

	log.Println("✅ 远程协助管理器已启动")
	return mgr, nil
}

// cleanupTask 清理任务.
func (m *Manager) cleanupTask() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpired()
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanupExpired 清理过期会话.
func (m *Manager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expired := make([]string, 0)

	for id, session := range m.sessions {
		if session.Status == StatusActive && now.After(session.ExpiresAt) {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		m.endSession(id, "expired")
		log.Printf("🧹 清理过期会话: %s", id)
	}
}

// CreateSession 创建远程会话.
func (m *Manager) CreateSession(req *AssistRequest) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查会话数限制
	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("会话数已达上限: %d", m.config.MaxSessions)
	}

	// 验证请求
	if err := m.validateRequest(req); err != nil {
		return nil, err
	}

	session := &Session{
		ID:           uuid.New().String(),
		Name:         fmt.Sprintf("远程协助-%s", time.Now().Format("20060102-150405")),
		Type:         req.Type,
		Status:       StatusPending,
		HostID:       req.HostID,
		GuestID:      req.GuestID,
		Permission:   req.Permission,
		Token:        m.generateToken(),
		ExpiresAt:    time.Now().Add(time.Duration(req.ExpiresIn) * time.Second),
		Tags:         make([]string, 0),
		Metadata:     make(map[string]string),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.sessions[session.ID] = session

	// 记录审计事件
	m.audit.LogEvent(&AuditEvent{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		UserID:    req.GuestID,
		Action:    "session_created",
		Resource:  session.ID,
		Status:    "success",
		Timestamp: time.Now(),
		RiskLevel: "low",
	})

	log.Printf("✅ 创建远程会话: %s, 类型: %s", session.ID, session.Type)
	return session, nil
}

// validateRequest 验证请求.
func (m *Manager) validateRequest(req *AssistRequest) error {
	if req.HostID == "" {
		return fmt.Errorf("主机ID不能为空")
	}
	if req.GuestID == "" {
		return fmt.Errorf("访客ID不能为空")
	}
	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 3600 // 默认1小时
	}
	if req.ExpiresIn > m.config.MaxDuration {
		return fmt.Errorf("过期时间超过最大限制: %d", m.config.MaxDuration)
	}
	return nil
}

// generateToken 生成连接令牌.
func (m *Manager) generateToken() string {
	return uuid.New().String()
}

// GetSession 获取会话.
func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", id)
	}
	return session, nil
}

// ListSessions 列出会话.
func (m *Manager) ListSessions(userID string, status SessionStatus) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0)
	for _, session := range m.sessions {
		if userID != "" && session.HostID != userID && session.GuestID != userID {
			continue
		}
		if status != "" && session.Status != status {
			continue
		}
		result = append(result, session)
	}
	return result
}

// ActivateSession 激活会话.
func (m *Manager) ActivateSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	if session.Status != StatusPending {
		return fmt.Errorf("会话状态不允许激活: %s", session.Status)
	}

	now := time.Now()
	session.Status = StatusActive
	session.StartedAt = &now
	session.UpdatedAt = now

	// 记录审计事件
	m.audit.LogEvent(&AuditEvent{
		ID:        uuid.New().String(),
		SessionID: id,
		Action:    "session_activated",
		Status:    "success",
		Timestamp: time.Now(),
		RiskLevel: "low",
	})

	log.Printf("✅ 激活会话: %s", id)
	return nil
}

// PauseSession 暂停会话.
func (m *Manager) PauseSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	if session.Status != StatusActive {
		return fmt.Errorf("会话状态不允许暂停: %s", session.Status)
	}

	session.Status = StatusPaused
	session.UpdatedAt = time.Now()

	log.Printf("⏸️ 暂停会话: %s", id)
	return nil
}

// ResumeSession 恢复会话.
func (m *Manager) ResumeSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	if session.Status != StatusPaused {
		return fmt.Errorf("会话状态不允许恢复: %s", session.Status)
	}

	session.Status = StatusActive
	session.UpdatedAt = time.Now()

	log.Printf("▶️ 恢复会话: %s", id)
	return nil
}

// EndSession 结束会话.
func (m *Manager) EndSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.endSession(id, "manual")
}

// endSession 内部结束会话.
func (m *Manager) endSession(id, reason string) error {
	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	now := time.Now()
	session.Status = StatusCompleted
	session.EndedAt = &now
	session.UpdatedAt = now

	if session.StartedAt != nil {
		session.Duration = int64(now.Sub(*session.StartedAt).Seconds())
	}

	// 停止录制
	if session.IsRecording {
		m.recorder.StopRecording(id)
	}

	// 记录审计事件
	m.audit.LogEvent(&AuditEvent{
		ID:        uuid.New().String(),
		SessionID: id,
		Action:    "session_ended",
		Details:   map[string]interface{}{"reason": reason},
		Status:    "success",
		Timestamp: time.Now(),
		RiskLevel: "low",
	})

	log.Printf("⏹️ 结束会话: %s, 原因: %s", id, reason)
	return nil
}

// DeleteSession 删除会话.
func (m *Manager) DeleteSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("会话不存在: %s", id)
	}

	if session.Status == StatusActive {
		m.endSession(id, "delete")
	}

	delete(m.sessions, id)
	delete(m.chatMsgs, id)

	log.Printf("🗑️ 删除会话: %s", id)
	return nil
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalSessions: len(m.sessions),
	}

	userSet := make(map[string]bool)
	for _, session := range m.sessions {
		if session.Status == StatusActive {
			stats.ActiveSessions++
		}
		userSet[session.HostID] = true
		userSet[session.GuestID] = true
	}

	stats.TotalUsers = len(userSet)
	stats.TotalRecordings = len(m.recordings)
	stats.TotalTransfers = len(m.transfers)

	return stats
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:     true,
		BindAddress: "0.0.0.0",
		BindPort:    8443,
		MaxSessions: 100,
		MaxDuration: 86400, // 24小时
		TokenExpiry: 3600,  // 1小时
		Recording: &RecordingConfig{
			Enabled:       true,
			AutoRecord:    false,
			Format:        "webm",
			Resolution:    "1080p",
			MaxSize:       1024 * 1024 * 1024, // 1GB
			RetentionDays: 30,
			StoragePath:   "/var/nas-os/remoteassist/recordings",
		},
		Security: &SecurityConfig{
			Encryption:  true,
			TLS:         true,
			MaxAttempts: 5,
			LockoutTime: 300,
		},
		RateLimit: &RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 60,
			BurstSize:      10,
		},
		Storage: &StorageConfig{
			Type: "local",
			Path: "/var/nas-os/remoteassist",
		},
	}
}

// GetSessionHistory 获取会话历史.
func (m *Manager) GetSessionHistory(id string) ([]*AuditEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", id)
	}

	return m.audit.GetEventsBySession(id), nil
}

// Close 关闭管理器.
func (m *Manager) Close() {
	m.cancel()

	// 结束所有活跃会话
	m.mu.Lock()
	for id, session := range m.sessions {
		if session.Status == StatusActive || session.Status == StatusPaused {
			m.endSession(id, "shutdown")
		}
	}
	m.mu.Unlock()

	log.Println("🛑 远程协助管理器已关闭")
}
