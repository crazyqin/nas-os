// Package quantumsecurecomm 提供量子安全通信功能
// 后量子密码学，抗量子攻击通信
package quantumsecurecomm

import (
	"time"
)

// AlgorithmType 后量子密码算法类型.
type AlgorithmType string

const (
	// AlgorithmKyber Kyber 密钥封装机制 (NIST 标准).
	AlgorithmKyber AlgorithmType = "kyber"
	// AlgorithmDilithium Dilithium 数字签名算法 (NIST 标准).
	AlgorithmDilithium AlgorithmType = "dilithium"
	// AlgorithmSPHINCSPlus SPHINCS+ 无状态哈希签名.
	AlgorithmSPHINCSPlus AlgorithmType = "sphincs_plus"
	// AlgorithmFalcon Falcon 数字签名算法.
	AlgorithmFalcon AlgorithmType = "falcon"
	// AlgorithmNTRU NTRU 密钥封装机制.
	AlgorithmNTRU AlgorithmType = "ntru"
	// AlgorithmClassicMcEliece Classic McEliece 密钥封装机制.
	AlgorithmClassicMcEliece AlgorithmType = "classic_mceliece"
)

// SecurityLevel 安全等级.
type SecurityLevel string

const (
	// SecurityLevel1 NIST Level 1: 等效 AES-128.
	SecurityLevel1 SecurityLevel = "level1"
	// SecurityLevel3 NIST Level 3: 等效 AES-192.
	SecurityLevel3 SecurityLevel = "level3"
	// SecurityLevel5 NIST Level 5: 等效 AES-256.
	SecurityLevel5 SecurityLevel = "level5"
)

// KeyEncapsulation 密钥封装结果.
type KeyEncapsulation struct {
	PublicKey    []byte        `json:"publicKey"`
	SharedSecret []byte        `json:"sharedSecret"`
	Ciphertext   []byte        `json:"ciphertext"`
	Algorithm    AlgorithmType `json:"algorithm"`
	CreatedAt    time.Time     `json:"createdAt"`
}

// DigitalSignature 数字签名.
type DigitalSignature struct {
	Signature []byte        `json:"signature"`
	PublicKey []byte        `json:"publicKey"`
	Message   []byte        `json:"message"`
	Algorithm AlgorithmType `json:"algorithm"`
	CreatedAt time.Time     `json:"createdAt"`
}

// KeyPair 密钥对.
type KeyPair struct {
	PublicKey     []byte        `json:"publicKey"`
	PrivateKey    []byte        `json:"privateKey"`
	Algorithm     AlgorithmType `json:"algorithm"`
	SecurityLevel SecurityLevel `json:"securityLevel"`
	CreatedAt     time.Time     `json:"createdAt"`
	ExpiresAt     time.Time     `json:"expiresAt"`
}

// HandshakeState 握手状态.
type HandshakeState string

const (
	// HandshakeStateInit 初始化.
	HandshakeStateInit HandshakeState = "init"
	// HandshakeStateKeyExchange 密钥交换中.
	HandshakeStateKeyExchange HandshakeState = "key_exchange"
	// HandshakeStateAuthentication 认证中.
	HandshakeStateAuthentication HandshakeState = "authentication"
	// HandshakeStateComplete 完成.
	HandshakeStateComplete HandshakeState = "complete"
	// HandshakeStateFailed 失败.
	HandshakeStateFailed HandshakeState = "failed"
)

// HandshakeSession 握手会话.
type HandshakeSession struct {
	ID            string         `json:"id"`
	State         HandshakeState `json:"state"`
	Algorithm     AlgorithmType  `json:"algorithm"`
	SecurityLevel SecurityLevel  `json:"securityLevel"`
	LocalKeyPair  *KeyPair       `json:"localKeyPair"`
	RemotePubKey  []byte         `json:"remotePubKey,omitempty"`
	SharedSecret  []byte         `json:"sharedSecret,omitempty"`
	StartedAt     time.Time      `json:"startedAt"`
	CompletedAt   *time.Time     `json:"completedAt,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// ChannelState 通道状态.
type ChannelState string

const (
	// ChannelStateEstablished 已建立.
	ChannelStateEstablished ChannelState = "established"
	// ChannelStateRekeying 密钥更新中.
	ChannelStateRekeying ChannelState = "rekeying"
	// ChannelStateClosed 已关闭.
	ChannelStateClosed ChannelState = "closed"
)

// SecureChannel 安全通道.
type SecureChannel struct {
	ID            string        `json:"id"`
	State         ChannelState  `json:"state"`
	Algorithm     AlgorithmType `json:"algorithm"`
	SecurityLevel SecurityLevel `json:"securityLevel"`
	SessionID     string        `json:"sessionId"`
	SharedSecret  []byte        `json:"sharedSecret"`
	CreatedAt     time.Time     `json:"createdAt"`
	LastActiveAt  time.Time     `json:"lastActiveAt"`
	BytesSent     uint64        `json:"bytesSent"`
	BytesReceived uint64        `json:"bytesReceived"`
	MessageCount  uint64        `json:"messageCount"`
}

// EncryptedMessage 加密消息.
type EncryptedMessage struct {
	ChannelID  string    `json:"channelId"`
	Sequence   uint64    `json:"sequence"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
	Tag        []byte    `json:"tag"`
	Timestamp  time.Time `json:"timestamp"`
}

// AlgorithmInfo 算法信息.
type AlgorithmInfo struct {
	Type           AlgorithmType   `json:"type"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	SecurityLevels []SecurityLevel `json:"securityLevels"`
	PublicKeySize  int             `json:"publicKeySize"`
	PrivateKeySize int             `json:"privateKeySize"`
	SignatureSize  int             `json:"signatureSize,omitempty"`
	CiphertextSize int             `json:"ciphertextSize,omitempty"`
	SharedKeySize  int             `json:"sharedKeySize"`
	IsNISTStandard bool            `json:"isNISTStandard"`
}

// SecurityAudit 安全审计记录.
type SecurityAudit struct {
	ID        string        `json:"id"`
	ChannelID string        `json:"channelId"`
	EventType string        `json:"eventType"`
	Algorithm AlgorithmType `json:"algorithm"`
	Details   string        `json:"details"`
	Severity  string        `json:"severity"`
	Timestamp time.Time     `json:"timestamp"`
}

// ManagerState 管理器状态.
type ManagerState struct {
	ActiveChannels   int           `json:"activeChannels"`
	ActiveHandshakes int           `json:"activeHandshakes"`
	TotalKeyPairs    int           `json:"totalKeyPairs"`
	DefaultAlgorithm AlgorithmType `json:"defaultAlgorithm"`
	DefaultSecurity  SecurityLevel `json:"defaultSecurity"`
	Uptime           time.Duration `json:"uptime"`
	StartedAt        time.Time     `json:"startedAt"`
}
