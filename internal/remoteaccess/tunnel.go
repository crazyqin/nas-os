// Package remoteaccess 提供安全隧道实现
// 使用 Noise_IK 协议 (类似 WireGuard) 进行密钥交换
// 然后使用 ChaCha20-Poly1305 进行数据加密
package remoteaccess

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// Noise 协议消息类型
const (
	NoiseMsgHandshake1 uint8 = 0x01
	NoiseMsgHandshake2 uint8 = 0x02
	NoiseMsgTransport  uint8 = 0x03
)

// SecureTunnelConfig 安全隧道配置
type SecureTunnelConfig struct {
	LocalPeerID   string
	RemotePeerID  string
	PrivateKey    []byte // 32 bytes Curve25519 私钥
	PublicKey     []byte // 32 bytes Curve25519 公钥
	RemotePubKey  []byte // 远程公钥
	Conn          net.Conn // 底层连接 (可以是 UDP 打洞连接或 TCP 中继连接)
	MTU           int
	KDFInterval   time.Duration // 密钥轮换间隔
}

// SecureTunnel 安全隧道
type SecureTunnel struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	config       SecureTunnelConfig
	conn         net.Conn
	sendCipher   cipher.AEAD
	recvCipher   cipher.AEAD
	sendNonce    uint64 // atomic
	recvNonce    uint64 // atomic
	bytesIn      int64  // atomic
	bytesOut     int64  // atomic
	handshakeDone int32 // atomic
	sessionKey   []byte
	createdAt    time.Time
	lastRekey    time.Time
	rekeyCount   int
	onData       func([]byte)
	done         chan struct{}
}

// NewSecureTunnel 创建安全隧道
func NewSecureTunnel(logger *zap.Logger, config SecureTunnelConfig) (*SecureTunnel, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config.MTU <= 0 {
		config.MTU = 1400
	}
	if config.KDFInterval <= 0 {
		config.KDFInterval = 10 * time.Minute
	}

	// 如果没有提供密钥对，生成一个
	if config.PrivateKey == nil || config.PublicKey == nil {
		privKey, pubKey, err := generateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("生成密钥对失败: %w", err)
		}
		config.PrivateKey = privKey
		config.PublicKey = pubKey
	}

	return &SecureTunnel{
		logger:    logger,
		config:    config,
		conn:      config.Conn,
		createdAt: time.Now(),
		done:      make(chan struct{}),
	}, nil
}

// Handshake 执行 Noise_IK 握手
func (t *SecureTunnel) Handshake() error {
	// 发起握手: e, es, s, ss
	ephPriv, ephPub, err := generateKeyPair()
	if err != nil {
		return err
	}

	// 发送第一握手消息: [ephemeral_pub(32)] + [encrypted_static(48)] + [encrypted_timestamp(24)]
	// hs = Hash(ephPub || remotePubKey)
	hs := sha256.New()
	hs.Write(ephPub)
	hs.Write(t.config.RemotePubKey)
	hash := hs.Sum(nil)

	// es = DH(ephPriv, remotePubKey)
	es, err := curve25519.X25519(ephPriv, t.config.RemotePubKey)
	if err != nil {
		return fmt.Errorf("DH 计算失败: %w", err)
	}

	// 派生密钥
	ck := hash // chaining key
	k := deriveKeyFromDH(ck, es)
	ck = hmacHash(ck, es)

	// 加密静态公钥: encrypt(k, 0, static_pub)
	encryptedStatic := encryptWithKey(k, t.config.PublicKey, uint64(0))

	// ss = DH(staticPriv, remotePubKey)
	ss, err := curve25519.X25519(t.config.PrivateKey, t.config.RemotePubKey)
	if err != nil {
		return err
	}

	k = deriveKeyFromDH(ck, ss)
	ck = hmacHash(ck, ss)

	// 加密时间戳
	timestamp := make([]byte, 8)
	// 使用当前时间的前 8 字节
	for i := 0; i < 8; i++ {
		timestamp[i] = byte(time.Now().UnixNano() >> (8 * (7 - i)))
	}
	encryptedTimestamp := encryptWithKey(k, timestamp, uint64(1))

	// 发送握手消息 1
	msg1 := make([]byte, 1+32+len(encryptedStatic)+len(encryptedTimestamp))
	msg1[0] = NoiseMsgHandshake1
	copy(msg1[1:33], ephPub)
	copy(msg1[33:33+len(encryptedStatic)], encryptedStatic)
	copy(msg1[33+len(encryptedStatic):], encryptedTimestamp)

	if _, err := t.conn.Write(msg1); err != nil {
		return fmt.Errorf("发送握手消息失败: %w", err)
	}

	// 接收握手响应: ee, se
	buf := make([]byte, 1024)
	n, err := t.conn.Read(buf)
	if err != nil {
		return fmt.Errorf("读取握手响应失败: %w", err)
	}
	if n < 1 || buf[0] != NoiseMsgHandshake2 {
		return fmt.Errorf("无效的握手响应")
	}

	// 解析响应中的临时公钥
	respEphPub := buf[1:33]

	// 计算 ee = DH(ephPriv, respEphPub)
	ee, err := curve25519.X25519(ephPriv, respEphPub)
	if err != nil {
		return err
	}

	k = deriveKeyFromDH(ck, ee)
	ck = hmacHash(ck, ee)

	// se = DH(staticPriv, respEphPub)
	se, err := curve25519.X25519(t.config.PrivateKey, respEphPub)
	if err != nil {
		return err
	}

	k = deriveKeyFromDH(ck, se)
	ck = hmacHash(ck, se)

	// 从 ck 派生会话密钥
	t.sessionKey = ck

	// 初始化加密器
	t.sendCipher, err = chacha20poly1305.New(k[:32])
	if err != nil {
		return err
	}
	t.recvCipher, err = chacha20poly1305.New(k[:32])
	if err != nil {
		return err
	}

	atomic.StoreInt32(&t.handshakeDone, 1)
	t.lastRekey = time.Now()

	t.logger.Info("安全隧道握手完成",
		zap.String("local", t.config.LocalPeerID),
		zap.String("remote", t.config.RemotePeerID),
	)

	return nil
}

// Write 加密并写入数据
func (t *SecureTunnel) Write(data []byte) (int, error) {
	if atomic.LoadInt32(&t.handshakeDone) != 1 {
		return 0, fmt.Errorf("握手未完成")
	}

	// 检查是否需要密钥轮换
	if time.Since(t.lastRekey) > t.config.KDFInterval {
		t.rekey()
	}

	// 分片写入
	totalWritten := 0
	for totalWritten < len(data) {
		chunkSize := len(data) - totalWritten
		if chunkSize > t.config.MTU-24 { // 24 bytes overhead
			chunkSize = t.config.MTU - 24
		}

		chunk := data[totalWritten : totalWritten+chunkSize]

		// 加密
		nonce := atomic.AddUint64(&t.sendNonce, 1) - 1
		nonceBytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			nonceBytes[7-i] = byte(nonce >> (8 * i))
		}
		// 扩展到 12 bytes
		fullNonce := make([]byte, 12)
		copy(fullNonce[4:], nonceBytes)

		encrypted := t.sendCipher.Seal(nil, fullNonce, chunk, nil)

		// 写入: [type(1)] + [encrypted_data]
		msg := make([]byte, 1+len(encrypted))
		msg[0] = NoiseMsgTransport
		copy(msg[1:], encrypted)

		if _, err := t.conn.Write(msg); err != nil {
			return totalWritten, err
		}

		totalWritten += chunkSize
		atomic.AddInt64(&t.bytesOut, int64(chunkSize))
	}

	return totalWritten, nil
}

// Read 读取并解密数据
func (t *SecureTunnel) Read(buf []byte) (int, error) {
	if atomic.LoadInt32(&t.handshakeDone) != 1 {
		return 0, fmt.Errorf("握手未完成")
	}

	// 读取消息
	msgBuf := make([]byte, t.config.MTU+24+1) // 1 for type + overhead
	n, err := t.conn.Read(msgBuf)
	if err != nil {
		return 0, err
	}
	if n < 1 || msgBuf[0] != NoiseMsgTransport {
		return 0, fmt.Errorf("无效的消息类型: %d", msgBuf[0])
	}

	// 解密
	nonce := atomic.AddUint64(&t.recvNonce, 1) - 1
	nonceBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		nonceBytes[7-i] = byte(nonce >> (8 * i))
	}
	fullNonce := make([]byte, 12)
	copy(fullNonce[4:], nonceBytes)

	decrypted, err := t.recvCipher.Open(nil, fullNonce, msgBuf[1:n], nil)
	if err != nil {
		return 0, fmt.Errorf("解密失败: %w", err)
	}

	copied := copy(buf, decrypted)
	atomic.AddInt64(&t.bytesIn, int64(copied))

	return copied, nil
}

// SetOnData 设置数据回调 (异步读取模式)
func (t *SecureTunnel) SetOnData(fn func([]byte)) {
	t.onData = fn
	go t.readLoop()
}

// readLoop 异步读取循环
func (t *SecureTunnel) readLoop() {
	buf := make([]byte, 65536)
	for {
		select {
		case <-t.done:
			return
		default:
		}

		n, err := t.Read(buf)
		if err != nil {
			t.logger.Debug("隧道读取错误", zap.Error(err))
			return
		}
		if t.onData != nil && n > 0 {
			t.onData(buf[:n])
		}
	}
}

// Close 关闭隧道
func (t *SecureTunnel) Close() error {
	select {
	case <-t.done:
		return nil
	default:
		close(t.done)
	}
	return t.conn.Close()
}

// GetStats 获取隧道统计
func (t *SecureTunnel) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"local_peer":  t.config.LocalPeerID,
		"remote_peer": t.config.RemotePeerID,
		"bytes_in":    atomic.LoadInt64(&t.bytesIn),
		"bytes_out":   atomic.LoadInt64(&t.bytesOut),
		"handshake":   atomic.LoadInt32(&t.handshakeDone) == 1,
		"rekey_count": t.rekeyCount,
		"created_at":  t.createdAt,
		"last_rekey":  t.lastRekey,
	}
}

// rekey 密钥轮换
func (t *SecureTunnel) rekey() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 使用 HKDF 风格的密钥轮换
	newKey := sha256.Sum256(append(t.sessionKey, byte(t.rekeyCount)))
	newCipher, err := chacha20poly1305.New(newKey[:32])
	if err != nil {
		t.logger.Warn("密钥轮换失败", zap.Error(err))
		return
	}

	t.sendCipher = newCipher
	t.recvCipher = newCipher
	atomic.StoreUint64(&t.sendNonce, 0)
	atomic.StoreUint64(&t.recvNonce, 0)
	t.lastRekey = time.Now()
	t.rekeyCount++
	t.sessionKey = newKey[:]

	t.logger.Debug("密钥轮换完成", zap.Int("count", t.rekeyCount))
}

// generateKeyPair 生成 Curve25519 密钥对
func generateKeyPair() (privKey, pubKey []byte, err error) {
	privKey = make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		return nil, nil, err
	}

	pubKey, err = curve25519.X25519(privKey, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}

	return privKey, pubKey, nil
}

// deriveKeyFromDH 从 DH 结果派生密钥
func deriveKeyFromDH(ck, dh []byte) []byte {
	h := sha256.New()
	h.Write(ck)
	h.Write(dh)
	return h.Sum(nil)
}

// hmacHash 简化的 HMAC-SHA256
func hmacHash(key, data []byte) []byte {
	h := sha256.New()
	h.Write(key)
	h.Write(data)
	return h.Sum(nil)
}

// encryptWithKey 使用指定密钥和 nonce 加密
func encryptWithKey(key []byte, plaintext []byte, nonce uint64) []byte {
	if len(key) < 32 {
		// Pad key
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	aead, err := chacha20poly1305.New(key[:32])
	if err != nil {
		return plaintext
	}

	nonceBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		nonceBytes[7-i] = byte(nonce >> (8 * i))
	}
	fullNonce := make([]byte, 12)
	copy(fullNonce[4:], nonceBytes)

	return aead.Seal(nil, fullNonce, plaintext, nil)
}

// KeyManager 密钥管理器
type KeyManager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	privateKey []byte
	publicKey  []byte
	peers      map[string]*PeerKeyInfo
}

// PeerKeyInfo 节点密钥信息
type PeerKeyInfo struct {
	PeerID    string    `json:"peer_id"`
	PublicKey string    `json:"public_key"`
	AddedAt   time.Time `json:"added_at"`
	Trusted   bool      `json:"trusted"`
}

// NewKeyManager 创建密钥管理器
func NewKeyManager(logger *zap.Logger) (*KeyManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	privKey, pubKey, err := generateKeyPair()
	if err != nil {
		return nil, err
	}

	return &KeyManager{
		logger:     logger,
		privateKey: privKey,
		publicKey:  pubKey,
		peers:      make(map[string]*PeerKeyInfo),
	}, nil
}

// GetPublicKey 获取本机公钥 (base64 编码)
func (km *KeyManager) GetPublicKey() string {
	return base64.StdEncoding.EncodeToString(km.publicKey)
}

// GetPrivateKey 获取本机私钥
func (km *KeyManager) GetPrivateKey() []byte {
	return km.privateKey
}

// AddTrustedPeer 添加受信任的节点
func (km *KeyManager) AddTrustedPeer(peerID, pubKeyBase64 string) error {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return fmt.Errorf("无效的公钥: %w", err)
	}
	if len(pubKeyBytes) != 32 {
		return fmt.Errorf("公钥长度错误: %d (需要 32)", len(pubKeyBytes))
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	km.peers[peerID] = &PeerKeyInfo{
		PeerID:    peerID,
		PublicKey: pubKeyBase64,
		AddedAt:   time.Now(),
		Trusted:   true,
	}

	km.logger.Info("添加受信任节点",
		zap.String("peer_id", peerID),
		zap.String("public_key", hex.EncodeToString(pubKeyBytes[:8])),
	)

	return nil
}

// GetPeerPublicKey 获取节点公钥
func (km *KeyManager) GetPeerPublicKey(peerID string) ([]byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	info, exists := km.peers[peerID]
	if !exists {
		return nil, fmt.Errorf("节点 %s 未注册", peerID)
	}
	if !info.Trusted {
		return nil, fmt.Errorf("节点 %s 未受信任", peerID)
	}

	return base64.StdEncoding.DecodeString(info.PublicKey)
}

// RemovePeer 移除节点
func (km *KeyManager) RemovePeer(peerID string) {
	km.mu.Lock()
	defer km.mu.Unlock()
	delete(km.peers, peerID)
}

// ListPeers 列出所有已知节点
func (km *KeyManager) ListPeers() []*PeerKeyInfo {
	km.mu.RLock()
	defer km.mu.RUnlock()

	peers := make([]*PeerKeyInfo, 0, len(km.peers))
	for _, p := range km.peers {
		peers = append(peers, p)
	}
	return peers
}
