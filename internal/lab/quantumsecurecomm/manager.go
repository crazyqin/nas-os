// Package quantumsecurecomm 提供量子安全通信管理
package quantumsecurecomm

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// Manager 量子安全通信管理器.
type Manager struct {
	mu              sync.RWMutex
	channels        map[string]*SecureChannel
	handshakes      map[string]*HandshakeSession
	keyPairs        map[string]*KeyPair
	auditLog        []SecurityAudit
	defaultAlgo     AlgorithmType
	defaultSecurity SecurityLevel
	stopChan        chan struct{}
	running         bool
	startedAt       time.Time
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	DefaultAlgorithm AlgorithmType
	DefaultSecurity  SecurityLevel
	MaxChannels      int
	MaxHandshakes    int
	AuditLogSize     int
}

// NewManager 创建量子安全通信管理器.
func NewManager(cfg *ManagerConfig) *Manager {
	if cfg == nil {
		cfg = &ManagerConfig{}
	}

	if cfg.DefaultAlgorithm == "" {
		cfg.DefaultAlgorithm = AlgorithmKyber
	}
	if cfg.DefaultSecurity == "" {
		cfg.DefaultSecurity = SecurityLevel1
	}
	if cfg.MaxChannels == 0 {
		cfg.MaxChannels = 1000
	}
	if cfg.MaxHandshakes == 0 {
		cfg.MaxHandshakes = 100
	}
	if cfg.AuditLogSize == 0 {
		cfg.AuditLogSize = 10000
	}

	return &Manager{
		channels:        make(map[string]*SecureChannel),
		handshakes:      make(map[string]*HandshakeSession),
		keyPairs:        make(map[string]*KeyPair),
		auditLog:        make([]SecurityAudit, 0, cfg.AuditLogSize),
		defaultAlgo:     cfg.DefaultAlgorithm,
		defaultSecurity: cfg.DefaultSecurity,
		stopChan:        make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.startedAt = time.Now()
	m.mu.Unlock()
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		close(m.stopChan)
		m.running = false
	}
}

// GenerateKeyPair 生成密钥对.
func (m *Manager) GenerateKeyPair(algorithm AlgorithmType, level SecurityLevel) (*KeyPair, error) {
	if algorithm == "" {
		algorithm = m.defaultAlgo
	}
	if level == "" {
		level = m.defaultSecurity
	}

	// 模拟密钥生成
	pubKeySize := getPublicKeySize(algorithm, level)
	privKeySize := getPrivateKeySize(algorithm, level)

	pubKey := make([]byte, pubKeySize)
	privKey := make([]byte, privKeySize)

	if _, err := rand.Read(pubKey); err != nil {
		return nil, fmt.Errorf("生成公钥失败: %w", err)
	}
	if _, err := rand.Read(privKey); err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	now := time.Now()
	kp := &KeyPair{
		PublicKey:     pubKey,
		PrivateKey:    privKey,
		Algorithm:     algorithm,
		SecurityLevel: level,
		CreatedAt:     now,
		ExpiresAt:     now.Add(365 * 24 * time.Hour), // 1年有效期
	}

	keyID := fmt.Sprintf("kp-%d", now.UnixNano())
	m.mu.Lock()
	m.keyPairs[keyID] = kp
	m.addAuditLog("key_generated", "", algorithm, fmt.Sprintf("生成 %s 密钥对", algorithm), "info")
	m.mu.Unlock()

	return kp, nil
}

// InitiateHandshake 发起握手.
func (m *Manager) InitiateHandshake(algorithm AlgorithmType, level SecurityLevel) (*HandshakeSession, error) {
	if algorithm == "" {
		algorithm = m.defaultAlgo
	}
	if level == "" {
		level = m.defaultSecurity
	}

	// 生成本地密钥对
	keyPair, err := m.GenerateKeyPair(algorithm, level)
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	now := time.Now()
	session := &HandshakeSession{
		ID:            fmt.Sprintf("hs-%d", now.UnixNano()),
		State:         HandshakeStateInit,
		Algorithm:     algorithm,
		SecurityLevel: level,
		LocalKeyPair:  keyPair,
		StartedAt:     now,
	}

	m.mu.Lock()
	m.handshakes[session.ID] = session
	m.addAuditLog("handshake_initiated", session.ID, algorithm, "握手已发起", "info")
	m.mu.Unlock()

	return session, nil
}

// CompleteHandshake 完成握手.
func (m *Manager) CompleteHandshake(sessionID string, remotePubKey []byte) (*SecureChannel, error) {
	m.mu.Lock()
	session, ok := m.handshakes[sessionID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("握手会话不存在: %s", sessionID)
	}

	session.RemotePubKey = remotePubKey
	session.State = HandshakeStateKeyExchange

	// 模拟密钥封装
	ciphertext := make([]byte, getCiphertextSize(session.Algorithm))
	rand.Read(ciphertext)

	// 模拟生成共享密钥
	sharedSecret := make([]byte, 32)
	rand.Read(sharedSecret)

	session.SharedSecret = sharedSecret
	now := time.Now()
	session.State = HandshakeStateComplete
	session.CompletedAt = &now

	// 创建安全通道
	channel := &SecureChannel{
		ID:            fmt.Sprintf("ch-%d", now.UnixNano()),
		State:         ChannelStateEstablished,
		Algorithm:     session.Algorithm,
		SecurityLevel: session.SecurityLevel,
		SessionID:     sessionID,
		SharedSecret:  sharedSecret,
		CreatedAt:     now,
		LastActiveAt:  now,
	}

	m.channels[channel.ID] = channel
	m.addAuditLog("handshake_completed", sessionID, session.Algorithm, "握手完成，安全通道已建立", "info")
	m.mu.Unlock()

	return channel, nil
}

// EncryptMessage 加密消息.
func (m *Manager) EncryptMessage(channelID string, plaintext []byte) (*EncryptedMessage, error) {
	m.mu.RLock()
	channel, ok := m.channels[channelID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("通道不存在: %s", channelID)
	}

	if channel.State != ChannelStateEstablished {
		m.mu.RUnlock()
		return nil, fmt.Errorf("通道状态异常: %s", channel.State)
	}
	m.mu.RUnlock()

	// 模拟加密
	nonce := make([]byte, 12)
	rand.Read(nonce)

	ciphertext := make([]byte, len(plaintext))
	copy(ciphertext, plaintext) // 简化：实际应使用 AEAD 加密

	tag := make([]byte, 16)
	rand.Read(tag)

	msg := &EncryptedMessage{
		ChannelID:  channelID,
		Sequence:   channel.MessageCount + 1,
		Nonce:      nonce,
		Ciphertext: ciphertext,
		Tag:        tag,
		Timestamp:  time.Now(),
	}

	m.mu.Lock()
	channel.MessageCount++
	channel.BytesSent += uint64(len(plaintext))
	channel.LastActiveAt = time.Now()
	m.mu.Unlock()

	return msg, nil
}

// DecryptMessage 解密消息.
func (m *Manager) DecryptMessage(msg *EncryptedMessage) ([]byte, error) {
	m.mu.RLock()
	channel, ok := m.channels[msg.ChannelID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("通道不存在: %s", msg.ChannelID)
	}

	if channel.State != ChannelStateEstablished {
		m.mu.RUnlock()
		return nil, fmt.Errorf("通道状态异常: %s", channel.State)
	}
	m.mu.RUnlock()

	// 模拟解密
	plaintext := make([]byte, len(msg.Ciphertext))
	copy(plaintext, msg.Ciphertext) // 简化：实际应使用 AEAD 解密

	m.mu.Lock()
	channel.BytesReceived += uint64(len(plaintext))
	channel.LastActiveAt = time.Now()
	m.mu.Unlock()

	return plaintext, nil
}

// SignMessage 签名消息.
func (m *Manager) SignMessage(keyID string, message []byte) (*DigitalSignature, error) {
	m.mu.RLock()
	keyPair, ok := m.keyPairs[keyID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("密钥对不存在: %s", keyID)
	}
	m.mu.RUnlock()

	// 模拟签名
	sigSize := getSignatureSize(keyPair.Algorithm)
	signature := make([]byte, sigSize)
	rand.Read(signature)

	return &DigitalSignature{
		Signature: signature,
		PublicKey: keyPair.PublicKey,
		Message:   message,
		Algorithm: keyPair.Algorithm,
		CreatedAt: time.Now(),
	}, nil
}

// VerifySignature 验证签名.
func (m *Manager) VerifySignature(sig *DigitalSignature) (bool, error) {
	if sig == nil || len(sig.Signature) == 0 || len(sig.PublicKey) == 0 {
		return false, fmt.Errorf("无效签名数据")
	}

	// 模拟验证：检查签名大小是否正确
	expectedSize := getSignatureSize(sig.Algorithm)
	if len(sig.Signature) != expectedSize {
		return false, nil
	}

	return true, nil
}

// CloseChannel 关闭安全通道.
func (m *Manager) CloseChannel(channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, ok := m.channels[channelID]
	if !ok {
		return fmt.Errorf("通道不存在: %s", channelID)
	}

	channel.State = ChannelStateClosed
	m.addAuditLog("channel_closed", channelID, channel.Algorithm, "安全通道已关闭", "info")
	return nil
}

// GetChannel 获取安全通道.
func (m *Manager) GetChannel(channelID string) (*SecureChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, ok := m.channels[channelID]
	if !ok {
		return nil, fmt.Errorf("通道不存在: %s", channelID)
	}
	return channel, nil
}

// ListChannels 列出所有通道.
func (m *Manager) ListChannels() []*SecureChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]*SecureChannel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	return channels
}

// GetHandshake 获取握手会话.
func (m *Manager) GetHandshake(sessionID string) (*HandshakeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.handshakes[sessionID]
	if !ok {
		return nil, fmt.Errorf("握手会话不存在: %s", sessionID)
	}
	return session, nil
}

// GetSupportedAlgorithms 获取支持的算法列表.
func (m *Manager) GetSupportedAlgorithms() []AlgorithmInfo {
	return []AlgorithmInfo{
		{
			Type:           AlgorithmKyber,
			Name:           "CRYSTALS-Kyber",
			Description:    "NIST 标准密钥封装机制",
			SecurityLevels: []SecurityLevel{SecurityLevel1, SecurityLevel3, SecurityLevel5},
			PublicKeySize:  1184,
			PrivateKeySize: 2400,
			CiphertextSize: 1088,
			SharedKeySize:  32,
			IsNISTStandard: true,
		},
		{
			Type:           AlgorithmDilithium,
			Name:           "CRYSTALS-Dilithium",
			Description:    "NIST 标准数字签名算法",
			SecurityLevels: []SecurityLevel{SecurityLevel1, SecurityLevel3, SecurityLevel5},
			PublicKeySize:  1312,
			PrivateKeySize: 2528,
			SignatureSize:  2420,
			SharedKeySize:  0,
			IsNISTStandard: true,
		},
		{
			Type:           AlgorithmSPHINCSPlus,
			Name:           "SPHINCS+",
			Description:    "无状态哈希签名方案",
			SecurityLevels: []SecurityLevel{SecurityLevel1, SecurityLevel3, SecurityLevel5},
			PublicKeySize:  32,
			PrivateKeySize: 64,
			SignatureSize:  7856,
			SharedKeySize:  0,
			IsNISTStandard: true,
		},
		{
			Type:           AlgorithmFalcon,
			Name:           "FALCON",
			Description:    "基于格的签名算法",
			SecurityLevels: []SecurityLevel{SecurityLevel1, SecurityLevel5},
			PublicKeySize:  897,
			PrivateKeySize: 1281,
			SignatureSize:  666,
			SharedKeySize:  0,
			IsNISTStandard: false,
		},
		{
			Type:           AlgorithmNTRU,
			Name:           "NTRU",
			Description:    "基于格的密钥封装机制",
			SecurityLevels: []SecurityLevel{SecurityLevel1, SecurityLevel3, SecurityLevel5},
			PublicKeySize:  1230,
			PrivateKeySize: 1590,
			CiphertextSize: 1230,
			SharedKeySize:  32,
			IsNISTStandard: false,
		},
		{
			Type:           AlgorithmClassicMcEliece,
			Name:           "Classic McEliece",
			Description:    "基于编码的密钥封装机制",
			SecurityLevels: []SecurityLevel{SecurityLevel1, SecurityLevel3, SecurityLevel5},
			PublicKeySize:  261120,
			PrivateKeySize: 6492,
			CiphertextSize: 128,
			SharedKeySize:  32,
			IsNISTStandard: true,
		},
	}
}

// GetState 获取管理器状态.
func (m *Manager) GetState() *ManagerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeChannels := 0
	for _, ch := range m.channels {
		if ch.State == ChannelStateEstablished {
			activeChannels++
		}
	}

	activeHandshakes := 0
	for _, hs := range m.handshakes {
		if hs.State != HandshakeStateComplete && hs.State != HandshakeStateFailed {
			activeHandshakes++
		}
	}

	return &ManagerState{
		ActiveChannels:   activeChannels,
		ActiveHandshakes: activeHandshakes,
		TotalKeyPairs:    len(m.keyPairs),
		DefaultAlgorithm: m.defaultAlgo,
		DefaultSecurity:  m.defaultSecurity,
		Uptime:           time.Since(m.startedAt),
		StartedAt:        m.startedAt,
	}
}

// GetAuditLog 获取审计日志.
func (m *Manager) GetAuditLog(limit int) []SecurityAudit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	// 返回最新的日志
	start := len(m.auditLog) - limit
	result := make([]SecurityAudit, limit)
	copy(result, m.auditLog[start:])
	return result
}

// RunWithContext 带 Context 运行.
func (m *Manager) RunWithContext(ctx context.Context) {
	m.Start()
	<-ctx.Done()
	m.Stop()
}

// addAuditLog 添加审计日志（内部使用，需持有锁）.
func (m *Manager) addAuditLog(eventType, channelID string, algorithm AlgorithmType, details, severity string) {
	entry := SecurityAudit{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		ChannelID: channelID,
		EventType: eventType,
		Algorithm: algorithm,
		Details:   details,
		Severity:  severity,
		Timestamp: time.Now(),
	}

	// 保持日志大小限制
	if len(m.auditLog) >= cap(m.auditLog) {
		// 移除最旧的条目
		m.auditLog = m.auditLog[1:]
	}
	m.auditLog = append(m.auditLog, entry)
}

// 辅助函数：获取算法参数大小.
func getPublicKeySize(algo AlgorithmType, level SecurityLevel) int {
	switch algo {
	case AlgorithmKyber:
		switch level {
		case SecurityLevel1:
			return 800
		case SecurityLevel3:
			return 1184
		case SecurityLevel5:
			return 1568
		}
	case AlgorithmDilithium:
		switch level {
		case SecurityLevel1:
			return 1312
		case SecurityLevel3:
			return 1952
		case SecurityLevel5:
			return 2592
		}
	case AlgorithmSPHINCSPlus:
		return 32
	case AlgorithmFalcon:
		return 897
	case AlgorithmNTRU:
		return 1230
	case AlgorithmClassicMcEliece:
		return 261120
	}
	return 1024
}

func getPrivateKeySize(algo AlgorithmType, level SecurityLevel) int {
	switch algo {
	case AlgorithmKyber:
		switch level {
		case SecurityLevel1:
			return 1632
		case SecurityLevel3:
			return 2400
		case SecurityLevel5:
			return 3168
		}
	case AlgorithmDilithium:
		switch level {
		case SecurityLevel1:
			return 2528
		case SecurityLevel3:
			return 4000
		case SecurityLevel5:
			return 4864
		}
	case AlgorithmSPHINCSPlus:
		return 64
	case AlgorithmFalcon:
		return 1281
	case AlgorithmNTRU:
		return 1590
	case AlgorithmClassicMcEliece:
		return 6492
	}
	return 2048
}

func getCiphertextSize(algo AlgorithmType) int {
	switch algo {
	case AlgorithmKyber:
		return 768
	case AlgorithmNTRU:
		return 1230
	case AlgorithmClassicMcEliece:
		return 128
	}
	return 512
}

func getSignatureSize(algo AlgorithmType) int {
	switch algo {
	case AlgorithmDilithium:
		return 2420
	case AlgorithmSPHINCSPlus:
		return 7856
	case AlgorithmFalcon:
		return 666
	}
	return 256
}
