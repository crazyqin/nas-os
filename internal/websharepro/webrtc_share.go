// Package websharepro - WebRTC P2P 直传模块
// 不经过服务器的 WebRTC 点对点文件传输
// 支持 STUN/TURN 配置、信令交换、直连传输
package websharepro

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// STUN/TURN 配置
// ---------------------------------------------------------------------------

// ICEServer ICE 服务器配置（STUN/TURN）
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
	// 服务器类型
	Type string `json:"type"` // stun, turn
}

// WebRTCConfig WebRTC 配置
type WebRTCConfig struct {
	// ICE 服务器列表
	ICEServers []ICEServer `json:"iceServers"`
	// 默认 STUN 服务器
	DefaultSTUN []string `json:"defaultStun"`
	// TURN 服务器（当 P2P 直连失败时使用）
	TURNServers []ICEServer `json:"turnServers"`
	// 传输配置
	MaxMessageSize  int64         `json:"maxMessageSize"`  // DataChannel 最大消息字节数
	TransferTimeout time.Duration `json:"transferTimeout"` // 传输超时
	IdleTimeout     time.Duration `json:"idleTimeout"`     // 空闲超时
	EnableRelay     bool          `json:"enableRelay"`     // 启用 TURN 中继
	MaxConnections  int           `json:"maxConnections"`  // 最大并发连接数
	BufferSize      int           `json:"bufferSize"`      // DataChannel 缓冲区大小
}

// DefaultWebRTCConfig 返回默认 WebRTC 配置
func DefaultWebRTCConfig() *WebRTCConfig {
	return &WebRTCConfig{
		DefaultSTUN: []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
		},
		MaxMessageSize:  64 * 1024, // 64KB
		TransferTimeout: 30 * time.Minute,
		IdleTimeout:     5 * time.Minute,
		EnableRelay:     true,
		MaxConnections:  50,
		BufferSize:      1024 * 1024, // 1MB
	}
}

// ---------------------------------------------------------------------------
// P2P 传输会话
// ---------------------------------------------------------------------------

// P2PSessionStatus P2P 会话状态
type P2PSessionStatus string

const (
	P2PWaiting    P2PSessionStatus = "waiting"    // 等待对方加入
	P2PConnecting P2PSessionStatus = "connecting" // 正在建立连接
	P2PConnected  P2PSessionStatus = "connected"  // 已连接，传输中
	P2PCompleted  P2PSessionStatus = "completed"  // 传输完成
	P2PFailed     P2PSessionStatus = "failed"     // 传输失败
	P2PCancelled  P2PSessionStatus = "cancelled"  // 已取消
)

// FileTransferInfo 文件传输信息
type FileTransferInfo struct {
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
	MimeType string `json:"mimeType,omitempty"`
	// 文件哈希（用于完整性验证）
	SHA256 string `json:"sha256,omitempty"`
}

// P2PShareSession WebRTC P2P 分享会话
type P2PShareSession struct {
	ID string `json:"id"`
	// 发起方
	InitiatorID   string `json:"initiatorId"`
	InitiatorName string `json:"initiatorName,omitempty"`
	// 接收方
	ReceiverID   string `json:"receiverId,omitempty"`
	ReceiverName string `json:"receiverName,omitempty"`
	// 传输文件信息
	Files []FileTransferInfo `json:"files"`
	// 状态
	Status P2PSessionStatus `json:"status"`
	// 信令数据
	OfferSDP  string `json:"offerSdp,omitempty"`
	AnswerSDP string `json:"answerSdp,omitempty"`
	// ICE 候选者
	InitiatorCandidates []ICECandidate `json:"initiatorCandidates,omitempty"`
	ReceiverCandidates  []ICECandidate `json:"receiverCandidates,omitempty"`
	// 密码保护
	Password string `json:"password,omitempty"`
	HasPwd   bool   `json:"hasPwd"`
	// 时间
	CreatedAt   time.Time  `json:"createdAt"`
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	// 传输统计
	BytesTransferred int64   `json:"bytesTransferred"`
	Speed            int64   `json:"speed"`    // bytes/sec
	Progress         float64 `json:"progress"` // 0-100
	// 使用的连接类型
	ConnectionType string `json:"connectionType,omitempty"` // direct, relay
}

// ICECandidate ICE 候选者
type ICECandidate struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid,omitempty"`
	SDPMLineIndex int    `json:"sdpMLineIndex,omitempty"`
}

// ---------------------------------------------------------------------------
// 信令消息
// ---------------------------------------------------------------------------

// SignalMessageType 信令消息类型
type SignalMessageType string

const (
	SignalOffer        SignalMessageType = "offer"
	SignalAnswer       SignalMessageType = "answer"
	SignalICECandidate SignalMessageType = "ice-candidate"
	SignalJoin         SignalMessageType = "join"
	SignalLeave        SignalMessageType = "leave"
	SignalError        SignalMessageType = "error"
)

// SignalMessage 信令消息
type SignalMessage struct {
	Type      SignalMessageType `json:"type"`
	SessionID string            `json:"sessionId"`
	SenderID  string            `json:"senderId"`
	Payload   json.RawMessage   `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// P2PShareManager P2P 传输管理器
// ---------------------------------------------------------------------------

// P2PShareManager WebRTC P2P 分享管理器
type P2PShareManager struct {
	mu       sync.RWMutex
	sessions map[string]*P2PShareSession // id -> session
	config   *WebRTCConfig
	logger   *zap.Logger
	// 信令通道（实际部署中使用 WebSocket）
	signalChan map[string]chan SignalMessage
}

// NewP2PShareManager 创建 P2P 分享管理器
func NewP2PShareManager(config *WebRTCConfig, logger *zap.Logger) *P2PShareManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultWebRTCConfig()
	}
	// 确保 ICE 服务器列表包含默认 STUN
	if len(config.ICEServers) == 0 && len(config.DefaultSTUN) > 0 {
		for _, url := range config.DefaultSTUN {
			config.ICEServers = append(config.ICEServers, ICEServer{
				URLs: []string{url},
				Type: "stun",
			})
		}
	}
	return &P2PShareManager{
		sessions:   make(map[string]*P2PShareSession),
		config:     config,
		logger:     logger,
		signalChan: make(map[string]chan SignalMessage),
	}
}

// CreateSession 创建 P2P 传输会话
func (pm *P2PShareManager) CreateSession(initiatorID, initiatorName, password string, files []FileTransferInfo) (*P2PShareSession, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 检查并发连接数
	activeCount := 0
	for _, s := range pm.sessions {
		if s.Status == P2PWaiting || s.Status == P2PConnecting || s.Status == P2PConnected {
			activeCount++
		}
	}
	if activeCount >= pm.config.MaxConnections {
		return nil, fmt.Errorf("max concurrent connections reached (%d)", pm.config.MaxConnections)
	}

	id := generateShareID()
	now := time.Now()
	expiresAt := now.Add(pm.config.TransferTimeout)

	session := &P2PShareSession{
		ID:                  id,
		InitiatorID:         initiatorID,
		InitiatorName:       initiatorName,
		Files:               files,
		Status:              P2PWaiting,
		Password:            password,
		HasPwd:              password != "",
		CreatedAt:           now,
		ExpiresAt:           &expiresAt,
		InitiatorCandidates: make([]ICECandidate, 0),
		ReceiverCandidates:  make([]ICECandidate, 0),
	}

	pm.sessions[id] = session
	// 创建信令通道
	pm.signalChan[id] = make(chan SignalMessage, 64)

	pm.logger.Info("P2P session created",
		zap.String("id", id),
		zap.String("initiatorId", initiatorID),
		zap.Int("files", len(files)),
	)
	return session, nil
}

// GetSession 获取会话
func (pm *P2PShareManager) GetSession(id string) (*P2PShareSession, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	session, ok := pm.sessions[id]
	if !ok {
		return nil, false
	}
	// 检查过期
	if session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now()) && session.Status == P2PWaiting {
		session.Status = P2PFailed
	}
	return session, true
}

// JoinSession 加入 P2P 会话（接收方）
func (pm *P2PShareManager) JoinSession(sessionID, receiverID, receiverName, password string) (*P2PShareSession, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return nil, ErrLinkNotFound
	}

	if session.Status != P2PWaiting {
		return nil, fmt.Errorf("session is not waiting for a receiver (status: %s)", session.Status)
	}

	// 密码验证
	if session.HasPwd && session.Password != password {
		return nil, fmt.Errorf("incorrect password")
	}

	session.ReceiverID = receiverID
	session.ReceiverName = receiverName
	session.Status = P2PConnecting

	// 发送信令通知
	pm.sendSignal(sessionID, SignalMessage{
		Type:      SignalJoin,
		SessionID: sessionID,
		SenderID:  receiverID,
		Timestamp: time.Now(),
	})

	pm.logger.Info("receiver joined P2P session",
		zap.String("sessionId", sessionID),
		zap.String("receiverId", receiverID),
	)
	return session, nil
}

// SetOfferSDP 设置发起方 SDP Offer
func (pm *P2PShareManager) SetOfferSDP(sessionID, sdp string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return ErrLinkNotFound
	}
	session.OfferSDP = sdp

	// 通知接收方
	pm.sendSignal(sessionID, SignalMessage{
		Type:      SignalOffer,
		SessionID: sessionID,
		SenderID:  session.InitiatorID,
		Payload:   json.RawMessage(fmt.Sprintf(`{"sdp":%q}`, sdp)),
		Timestamp: time.Now(),
	})

	return nil
}

// SetAnswerSDP 设置接收方 SDP Answer
func (pm *P2PShareManager) SetAnswerSDP(sessionID, sdp string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return ErrLinkNotFound
	}
	session.AnswerSDP = sdp
	now := time.Now()
	session.ConnectedAt = &now
	session.Status = P2PConnected

	// 通知发起方
	pm.sendSignal(sessionID, SignalMessage{
		Type:      SignalAnswer,
		SessionID: sessionID,
		SenderID:  session.ReceiverID,
		Payload:   json.RawMessage(fmt.Sprintf(`{"sdp":%q}`, sdp)),
		Timestamp: time.Now(),
	})

	pm.logger.Info("P2P session connected", zap.String("sessionId", sessionID))
	return nil
}

// AddICECandidate 添加 ICE 候选者
func (pm *P2PShareManager) AddICECandidate(sessionID, senderID string, candidate ICECandidate) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return ErrLinkNotFound
	}

	if senderID == session.InitiatorID {
		session.InitiatorCandidates = append(session.InitiatorCandidates, candidate)
	} else {
		session.ReceiverCandidates = append(session.ReceiverCandidates, candidate)
	}

	// 转发 ICE 候选者
	pm.sendSignal(sessionID, SignalMessage{
		Type:      SignalICECandidate,
		SessionID: sessionID,
		SenderID:  senderID,
		Payload:   json.RawMessage(fmt.Sprintf(`{"candidate":%q,"sdpMid":%q,"sdpMLineIndex":%d}`, candidate.Candidate, candidate.SDPMid, candidate.SDPMLineIndex)),
		Timestamp: time.Now(),
	})

	return nil
}

// UpdateTransferProgress 更新传输进度
func (pm *P2PShareManager) UpdateTransferProgress(sessionID string, bytesTransferred int64, speed int64) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return ErrLinkNotFound
	}

	session.BytesTransferred = bytesTransferred
	session.Speed = speed

	// 计算进度
	var totalSize int64
	for _, f := range session.Files {
		totalSize += f.FileSize
	}
	if totalSize > 0 {
		session.Progress = float64(bytesTransferred) / float64(totalSize) * 100
		if session.Progress > 100 {
			session.Progress = 100
		}
	}

	// 检查是否完成
	if session.Progress >= 100 {
		now := time.Now()
		session.CompletedAt = &now
		session.Status = P2PCompleted
		pm.logger.Info("P2P transfer completed",
			zap.String("sessionId", sessionID),
			zap.Int64("bytesTransferred", bytesTransferred),
		)
	}

	return nil
}

// CompleteSession 完成 P2P 会话
func (pm *P2PShareManager) CompleteSession(sessionID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return ErrLinkNotFound
	}

	now := time.Now()
	session.CompletedAt = &now
	session.Status = P2PCompleted
	session.Progress = 100

	pm.logger.Info("P2P session completed", zap.String("sessionId", sessionID))
	return nil
}

// CancelSession 取消 P2P 会话
func (pm *P2PShareManager) CancelSession(sessionID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return ErrLinkNotFound
	}

	session.Status = P2PCancelled

	// 通知对方
	pm.sendSignal(sessionID, SignalMessage{
		Type:      SignalLeave,
		SessionID: sessionID,
		Timestamp: time.Now(),
	})

	pm.logger.Info("P2P session cancelled", zap.String("sessionId", sessionID))
	return nil
}

// FailSession 标记会话失败
func (pm *P2PShareManager) FailSession(sessionID, reason string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	session, ok := pm.sessions[sessionID]
	if !ok {
		return ErrLinkNotFound
	}

	session.Status = P2PFailed

	pm.sendSignal(sessionID, SignalMessage{
		Type:      SignalError,
		SessionID: sessionID,
		Payload:   json.RawMessage(fmt.Sprintf(`{"reason":%q}`, reason)),
		Timestamp: time.Now(),
	})

	pm.logger.Error("P2P session failed",
		zap.String("sessionId", sessionID),
		zap.String("reason", reason),
	)
	return nil
}

// GetICEServers 获取 ICE 服务器配置（供客户端使用）
func (pm *P2PShareManager) GetICEServers() []ICEServer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	servers := make([]ICEServer, len(pm.config.ICEServers))
	copy(servers, pm.config.ICEServers)

	// 如果启用中继，添加 TURN 服务器
	if pm.config.EnableRelay {
		servers = append(servers, pm.config.TURNServers...)
	}

	return servers
}

// GetConfig 获取 WebRTC 配置
func (pm *P2PShareManager) GetConfig() *WebRTCConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.config
}

// ListSessions 列出会话
func (pm *P2PShareManager) ListSessions(userID string, status P2PSessionStatus) []*P2PShareSession {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*P2PShareSession
	for _, s := range pm.sessions {
		if userID != "" && s.InitiatorID != userID && s.ReceiverID != userID {
			continue
		}
		if status != "" && s.Status != status {
			continue
		}
		result = append(result, s)
	}
	return result
}

// Cleanup 清理过期/完成/失败的会话
func (pm *P2PShareManager) Cleanup() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	count := 0
	for id, session := range pm.sessions {
		shouldRemove := false
		switch session.Status {
		case P2PCompleted, P2PFailed, P2PCancelled:
			// 已结束的会话：30 分钟后清理
			var endTime *time.Time
			if session.CompletedAt != nil {
				endTime = session.CompletedAt
			}
			if endTime != nil && time.Since(*endTime) > 30*time.Minute {
				shouldRemove = true
			}
		case P2PWaiting:
			// 等待中的会话：超时后清理
			if session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now()) {
				session.Status = P2PFailed
				shouldRemove = true
			}
		}
		if shouldRemove {
			delete(pm.sessions, id)
			delete(pm.signalChan, id)
			count++
		}
	}
	return count
}

// ReceiveSignal 接收信令消息（阻塞式）
func (pm *P2PShareManager) ReceiveSignal(sessionID string) <-chan SignalMessage {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	ch, ok := pm.signalChan[sessionID]
	if !ok {
		return nil
	}
	return ch
}

// sendSignal 发送信令消息（内部）
func (pm *P2PShareManager) sendSignal(sessionID string, msg SignalMessage) {
	ch, ok := pm.signalChan[sessionID]
	if !ok {
		return
	}
	select {
	case ch <- msg:
	default:
		pm.logger.Warn("signal channel full, dropping message",
			zap.String("sessionId", sessionID),
			zap.String("type", string(msg.Type)),
		)
	}
}

// GenerateP2PID 生成 P2P 会话 ID
func GenerateP2PID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "p2p-" + hex.EncodeToString(b)
}
