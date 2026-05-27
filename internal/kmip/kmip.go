// Package kmip KMIP密钥管理模块
// Key Management Interoperability Protocol 企业级密钥管理
// 对标TrueNAS KMIP支持
package kmip

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"
)

// KeyState 密钥状态
type KeyState string

const (
	KeyStatePreActive  KeyState = "pre_active"
	KeyStateActive     KeyState = "active"
	KeyStateDeactivated KeyState = "deactivated"
	KeyStateCompromised KeyState = "compromised"
	KeyStateDestroyed  KeyState = "destroyed"
)

// KeyAlgorithm 密钥算法
type KeyAlgorithm string

const (
	AlgorithmAES128 KeyAlgorithm = "AES-128"
	AlgorithmAES256 KeyAlgorithm = "AES-256"
	AlgorithmRSA2048 KeyAlgorithm = "RSA-2048"
	AlgorithmRSA4096 KeyAlgorithm = "RSA-4096"
)

// KMIPKey KMIP密钥
type KMIPKey struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Algorithm     KeyAlgorithm `json:"algorithm"`
	KeySize       int          `json:"key_size"`
	State         KeyState     `json:"state"`
	Usage         []string     `json:"usage"` // encrypt, decrypt, sign, verify
	CreatedAt     time.Time    `json:"created_at"`
	ActivatedAt   *time.Time   `json:"activated_at,omitempty"`
	ExpiresAt     *time.Time   `json:"expires_at,omitempty"`
	LastUsedAt    *time.Time   `json:"last_used_at,omitempty"`
	
	// KMIP特有属性
	UniqueIdentifier string            `json:"unique_identifier"`
	ObjectType       string            `json:"object_type"`
	CryptographicLength int            `json:"cryptographic_length"`
	CryptographicUsageMask int         `json:"cryptographic_usage_mask"`
	
	// 元数据
	Tags       map[string]string `json:"tags,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// KMIPConfig KMIP配置
type KMIPConfig struct {
	// 服务器配置
	Host       string `json:"host"`
	Port       int    `json:"port"`
	
	// TLS配置
	TLSEnabled  bool   `json:"tls_enabled"`
	CertFile    string `json:"cert_file,omitempty"`
	KeyFile     string `json:"key_file,omitempty"`
	CAFile      string `json:"ca_file,omitempty"`
	
	// 认证
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	
	// 连接配置
	Timeout     int    `json:"timeout"` // 秒
	MaxRetries  int    `json:"max_retries"`
	
	// 密钥轮换
	AutoRotate  bool   `json:"auto_rotate"`
	RotateDays  int    `json:"rotate_days"`
}

// RotatePolicy 轮换策略
type RotatePolicy struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	KeyAlgorithm    KeyAlgorithm `json:"key_algorithm"`
	RotateInterval  int          `json:"rotate_interval"` // 天
	NotifyBefore    int          `json:"notify_before"`   // 提前多少天通知
	AutoActivate    bool         `json:"auto_activate"`
	ArchiveOldKey   bool         `json:"archive_old_key"`
	Enabled         bool         `json:"enabled"`
}

// AuditEvent 审计事件
type AuditEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	KeyID     string    `json:"key_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	Details   string    `json:"details,omitempty"`
}

// Service KMIP服务
type Service struct {
	mu       sync.RWMutex
	config   *KMIPConfig
	keys     map[string]*KMIPKey
	policies map[string]*RotatePolicy
	auditLog []AuditEvent
	client   *tls.Conn
}

// NewService 创建KMIP服务
func NewService(config *KMIPConfig) *Service {
	if config == nil {
		config = &KMIPConfig{
			Port:       5696,
			TLSEnabled: true,
			Timeout:    30,
			MaxRetries: 3,
			AutoRotate: true,
			RotateDays: 90,
		}
	}
	
	return &Service{
		config:   config,
		keys:     make(map[string]*KMIPKey),
		policies: make(map[string]*RotatePolicy),
		auditLog: make([]AuditEvent, 0),
	}
}

// Connect 连接到KMIP服务器
func (s *Service) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.config.TLSEnabled {
		return fmt.Errorf("TLS is required for KMIP connection")
	}
	
	// 加载客户端证书
	cert, err := tls.LoadX509KeyPair(s.config.CertFile, s.config.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load client certificate: %w", err)
	}
	
	// 加载CA证书
	caCert, err := os.ReadFile(s.config.CAFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}
	
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	
	// 配置TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}
	
	// 连接服务器
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to KMIP server: %w", err)
	}
	
	s.client = conn
	
	// 添加审计日志
	s.addAuditEvent("connect", "", "success", "Connected to KMIP server")
	
	return nil
}

// Disconnect 断开连接
func (s *Service) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.client != nil {
		err := s.client.Close()
		s.client = nil
		s.addAuditEvent("disconnect", "", "success", "Disconnected from KMIP server")
		return err
	}
	
	return nil
}

// CreateKey 创建密钥
func (s *Service) CreateKey(ctx context.Context, key *KMIPKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if key.ID == "" {
		key.ID = generateKeyID()
	}
	
	if key.UniqueIdentifier == "" {
		key.UniqueIdentifier = key.ID
	}
	
	key.State = KeyStatePreActive
	key.CreatedAt = time.Now()
	key.ObjectType = "SymmetricKey"
	
	// 设置密钥大小
	switch key.Algorithm {
	case AlgorithmAES128:
		key.KeySize = 128
		key.CryptographicLength = 128
	case AlgorithmAES256:
		key.KeySize = 256
		key.CryptographicLength = 256
	case AlgorithmRSA2048:
		key.KeySize = 2048
		key.CryptographicLength = 2048
	case AlgorithmRSA4096:
		key.KeySize = 4096
		key.CryptographicLength = 4096
	}
	
	// 设置使用掩码
	key.CryptographicUsageMask = s.calculateUsageMask(key.Usage)
	
	s.keys[key.ID] = key
	s.addAuditEvent("create_key", key.ID, "success", fmt.Sprintf("Created key: %s", key.Name))
	
	return nil
}

// ActivateKey 激活密钥
func (s *Service) ActivateKey(ctx context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key, exists := s.keys[keyID]
	if !exists {
		return fmt.Errorf("key not found: %s", keyID)
	}
	
	if key.State != KeyStatePreActive {
		return fmt.Errorf("cannot activate key in state: %s", key.State)
	}
	
	key.State = KeyStateActive
	now := time.Now()
	key.ActivatedAt = &now
	
	s.addAuditEvent("activate_key", keyID, "success", "Key activated")
	
	return nil
}

// DeactivateKey 停用密钥
func (s *Service) DeactivateKey(ctx context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key, exists := s.keys[keyID]
	if !exists {
		return fmt.Errorf("key not found: %s", keyID)
	}
	
	if key.State != KeyStateActive {
		return fmt.Errorf("cannot deactivate key in state: %s", key.State)
	}
	
	key.State = KeyStateDeactivated
	s.addAuditEvent("deactivate_key", keyID, "success", "Key deactivated")
	
	return nil
}

// DestroyKey 销毁密钥
func (s *Service) DestroyKey(ctx context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key, exists := s.keys[keyID]
	if !exists {
		return fmt.Errorf("key not found: %s", keyID)
	}
	
	key.State = KeyStateDestroyed
	s.addAuditEvent("destroy_key", keyID, "success", "Key destroyed")
	
	return nil
}

// GetKey 获取密钥信息
func (s *Service) GetKey(ctx context.Context, keyID string) (*KMIPKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	key, exists := s.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	
	return key, nil
}

// ListKeys 列出密钥
func (s *Service) ListKeys(ctx context.Context, state KeyState) []*KMIPKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	keys := make([]*KMIPKey, 0)
	for _, key := range s.keys {
		if state == "" || key.State == state {
			keys = append(keys, key)
		}
	}
	
	return keys
}

// RotateKey 轮换密钥
func (s *Service) RotateKey(ctx context.Context, keyID string) (*KMIPKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	oldKey, exists := s.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	
	// 创建新密钥
	newKey := &KMIPKey{
		Name:      oldKey.Name + "_rotated",
		Algorithm: oldKey.Algorithm,
		Usage:     oldKey.Usage,
		Tags:      oldKey.Tags,
	}
	
	// 生成新ID
	newKey.ID = generateKeyID()
	newKey.UniqueIdentifier = newKey.ID
	newKey.State = KeyStatePreActive
	newKey.CreatedAt = time.Now()
	
	// 复制密钥大小
	newKey.KeySize = oldKey.KeySize
	newKey.CryptographicLength = oldKey.CryptographicLength
	newKey.CryptographicUsageMask = oldKey.CryptographicUsageMask
	
	// 停用旧密钥
	oldKey.State = KeyStateDeactivated
	
	// 保存新密钥
	s.keys[newKey.ID] = newKey
	
	s.addAuditEvent("rotate_key", keyID, "success", 
		fmt.Sprintf("Key rotated: %s -> %s", keyID, newKey.ID))
	
	return newKey, nil
}

// AddRotatePolicy 添加轮换策略
func (s *Service) AddRotatePolicy(ctx context.Context, policy *RotatePolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if policy.ID == "" {
		policy.ID = generatePolicyID()
	}
	
	s.policies[policy.ID] = policy
	return nil
}

// ListRotatePolicies 列出轮换策略
func (s *Service) ListRotatePolicies(ctx context.Context) []*RotatePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	policies := make([]*RotatePolicy, 0, len(s.policies))
	for _, policy := range s.policies {
		policies = append(policies, policy)
	}
	
	return policies
}

// CheckAndRotateKeys 检查并轮换需要轮换的密钥
func (s *Service) CheckAndRotateKeys(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	rotatedKeys := make([]string, 0)
	now := time.Now()
	
	for _, key := range s.keys {
		if key.State != KeyStateActive || key.ExpiresAt == nil {
			continue
		}
		
		// 检查是否需要轮换
		if now.After(*key.ExpiresAt) {
			// 轮换密钥
			newKey := &KMIPKey{
				Name:      key.Name,
				Algorithm: key.Algorithm,
				Usage:     key.Usage,
			}
			newKey.ID = generateKeyID()
			newKey.State = KeyStatePreActive
			newKey.CreatedAt = now
			
			key.State = KeyStateDeactivated
			s.keys[newKey.ID] = newKey
			
			rotatedKeys = append(rotatedKeys, key.ID)
			s.addAuditEvent("auto_rotate", key.ID, "success", 
				fmt.Sprintf("Auto-rotated to %s", newKey.ID))
		}
	}
	
	return rotatedKeys, nil
}

// GetAuditLog 获取审计日志
func (s *Service) GetAuditLog(ctx context.Context, limit int) []AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if limit <= 0 || limit > len(s.auditLog) {
		limit = len(s.auditLog)
	}
	
	// 返回最近的日志
	start := len(s.auditLog) - limit
	return s.auditLog[start:]
}

// 内部方法

func (s *Service) addAuditEvent(action, keyID, status, details string) {
	event := AuditEvent{
		ID:        generateAuditID(),
		Timestamp: time.Now(),
		Action:    action,
		KeyID:     keyID,
		Source:    "kmip_service",
		Status:    status,
		Details:   details,
	}
	
	s.auditLog = append(s.auditLog, event)
	
	// 限制日志数量
	if len(s.auditLog) > 10000 {
		s.auditLog = s.auditLog[1000:]
	}
}

func (s *Service) calculateUsageMask(usage []string) int {
	mask := 0
	for _, u := range usage {
		switch u {
		case "encrypt":
			mask |= 0x00000001
		case "decrypt":
			mask |= 0x00000002
		case "sign":
			mask |= 0x00000004
		case "verify":
			mask |= 0x00000008
		}
	}
	return mask
}

func generateKeyID() string {
	return fmt.Sprintf("key_%d", time.Now().UnixNano())
}

func generatePolicyID() string {
	return fmt.Sprintf("policy_%d", time.Now().UnixNano())
}

func generateAuditID() string {
	return fmt.Sprintf("audit_%d", time.Now().UnixNano())
}
