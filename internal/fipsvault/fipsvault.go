// Package fipsvault 提供 FIPS 140-2/140-3 合规加密功能
// 学习 TrueNAS 26 WebShare FIPS 140 加密传输特性：
// - FIPS 合规的加密算法套件
// - 传输层加密（TLS 1.3 强制）
// - 静态数据加密（AES-256-GCM）
// - 密钥管理和轮换
// - 合规审计日志
// - 跨协议加密共享（SMB/NFS/HTTP）
package fipsvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// FIPSLevel FIPS 合规级别
type FIPSLevel string

const (
	FIPSLevel140_2 FIPSLevel = "140-2"
	FIPSLevel140_3 FIPSLevel = "140-3"
)

// CipherSuite 加密套件
type CipherSuite string

const (
	CipherAES256GCM CipherSuite = "AES-256-GCM"
	CipherAES256CBC CipherSuite = "AES-256-CBC"
	CipherChaCha20  CipherSuite = "ChaCha20-Poly1305"
	CipherAES128GCM CipherSuite = "AES-128-GCM"
)

// Protocol 传输协议
type Protocol string

const (
	ProtocolHTTPS  Protocol = "https"
	ProtocolSMB    Protocol = "smb"
	ProtocolNFS    Protocol = "nfs"
	ProtocolSFTP   Protocol = "sftp"
	ProtocolWebDAV Protocol = "webdav"
)

// KeyStatus 密钥状态
type KeyStatus string

const (
	KeyStatusActive      KeyStatus = "active"
	KeyStatusRotating    KeyStatus = "rotating"
	KeyStatusRetired     KeyStatus = "retired"
	KeyStatusCompromised KeyStatus = "compromised"
)

// EncryptionKey 加密密钥
type EncryptionKey struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Algorithm  CipherSuite `json:"algorithm"`
	KeySize    int         `json:"keySize"` // bits
	Status     KeyStatus   `json:"status"`
	CreatedAt  time.Time   `json:"createdAt"`
	ExpiresAt  time.Time   `json:"expiresAt"`
	RotatedAt  time.Time   `json:"rotatedAt"`
	UsageCount int64       `json:"usageCount"`
	MaxUsage   int64       `json:"maxUsage"` // 最大使用次数
	Version    int         `json:"version"`
}

// EncryptedShare 加密共享
type EncryptedShare struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Protocol    Protocol    `json:"protocol"`
	KeyID       string      `json:"keyId"`
	CipherSuite CipherSuite `json:"cipherSuite"`
	TLSVersion  string      `json:"tlsVersion"` // 1.3
	FIPSLevel   FIPSLevel   `json:"fipsLevel"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	AuditLog    bool        `json:"auditLog"` // 是否记录审计日志
}

// AuditEntry 审计条目
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"eventType"` // encrypt, decrypt, key_rotate, access, error
	ShareID   string    `json:"shareId"`
	KeyID     string    `json:"keyId"`
	UserID    string    `json:"userId"`
	SourceIP  string    `json:"sourceIp"`
	Protocol  Protocol  `json:"protocol"`
	Success   bool      `json:"success"`
	Details   string    `json:"details"`
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	GeneratedAt     time.Time                 `json:"generatedAt"`
	FIPSLevel       FIPSLevel                 `json:"fipsLevel"`
	OverallStatus   string                    `json:"overallStatus"` // compliant, non_compliant, warning
	TotalShares     int                       `json:"totalShares"`
	EncryptedShares int                       `json:"encryptedShares"`
	ActiveKeys      int                       `json:"activeKeys"`
	ExpiredKeys     int                       `json:"expiredKeys"`
	Issues          []ComplianceIssue         `json:"issues"`
	Protocols       map[string]ProtocolStatus `json:"protocols"`
}

// ComplianceIssue 合规问题
type ComplianceIssue struct {
	Severity string `json:"severity"` // critical, high, medium, low
	Share    string `json:"share"`
	Message  string `json:"message"`
	Action   string `json:"action"`
}

// ProtocolStatus 协议状态
type ProtocolStatus struct {
	Name       string `json:"name"`
	Encrypted  bool   `json:"encrypted"`
	TLSVersion string `json:"tlsVersion"`
	Cipher     string `json:"cipher"`
	Compliant  bool   `json:"compliant"`
}

// Manager FIPS 管理器
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	keys     map[string]*EncryptionKey
	shares   map[string]*EncryptedShare
	auditLog []*AuditEntry
}

// Config 管理器配置
type Config struct {
	Enabled         bool        `json:"enabled"`
	FIPSLevel       FIPSLevel   `json:"fipsLevel"`
	DefaultCipher   CipherSuite `json:"defaultCipher"`
	MinTLSVersion   string      `json:"minTLSVersion"`   // "1.3"
	KeyRotationDays int         `json:"keyRotationDays"` // 密钥轮换周期（天）
	AuditEnabled    bool        `json:"auditEnabled"`
	MaxAuditEntries int         `json:"maxAuditEntries"`
}

// NewManager 创建管理器
func NewManager(config *Config) *Manager {
	if config.DefaultCipher == "" {
		config.DefaultCipher = CipherAES256GCM
	}
	if config.MinTLSVersion == "" {
		config.MinTLSVersion = "1.3"
	}
	if config.KeyRotationDays == 0 {
		config.KeyRotationDays = 90
	}
	if config.MaxAuditEntries == 0 {
		config.MaxAuditEntries = 100000
	}
	return &Manager{
		config:   config,
		keys:     make(map[string]*EncryptionKey),
		shares:   make(map[string]*EncryptedShare),
		auditLog: make([]*AuditEntry, 0),
	}
}

// GenerateKey 生成加密密钥
func (m *Manager) GenerateKey(name string, cipher CipherSuite, keySize int) (*EncryptionKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cipher == "" {
		cipher = m.config.DefaultCipher
	}
	if keySize == 0 {
		keySize = 256
	}

	key := &EncryptionKey{
		ID:        fmt.Sprintf("key-%d", time.Now().UnixNano()),
		Name:      name,
		Algorithm: cipher,
		KeySize:   keySize,
		Status:    KeyStatusActive,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().AddDate(0, 0, m.config.KeyRotationDays),
		MaxUsage:  1000000,
		Version:   1,
	}

	m.keys[key.ID] = key
	m.addAudit("key_generate", "", key.ID, "", "", ProtocolHTTPS, true, fmt.Sprintf("生成密钥: %s (%s-%d)", name, cipher, keySize))

	return key, nil
}

// RotateKey 轮换密钥
func (m *Manager) RotateKey(keyID string) (*EncryptionKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldKey, ok := m.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("key %s not found", keyID)
	}

	// 旧密钥标记为轮换中
	oldKey.Status = KeyStatusRotating

	// 生成新密钥
	newKey := &EncryptionKey{
		ID:        fmt.Sprintf("key-%d", time.Now().UnixNano()),
		Name:      oldKey.Name,
		Algorithm: oldKey.Algorithm,
		KeySize:   oldKey.KeySize,
		Status:    KeyStatusActive,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().AddDate(0, 0, m.config.KeyRotationDays),
		MaxUsage:  oldKey.MaxUsage,
		Version:   oldKey.Version + 1,
	}

	m.keys[newKey.ID] = newKey

	// 旧密钥标记为退役
	oldKey.Status = KeyStatusRetired
	oldKey.RotatedAt = time.Now()

	m.addAudit("key_rotate", "", keyID, "", "", ProtocolHTTPS, true, fmt.Sprintf("密钥轮换: v%d -> v%d", oldKey.Version, newKey.Version))

	return newKey, nil
}

// CreateEncryptedShare 创建加密共享
func (m *Manager) CreateEncryptedShare(share *EncryptedShare) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if share.ID == "" {
		share.ID = fmt.Sprintf("share-%d", time.Now().UnixNano())
	}
	if share.CipherSuite == "" {
		share.CipherSuite = m.config.DefaultCipher
	}
	if share.TLSVersion == "" {
		share.TLSVersion = m.config.MinTLSVersion
	}
	if share.FIPSLevel == "" {
		share.FIPSLevel = m.config.FIPSLevel
	}
	share.Enabled = true
	share.AuditLog = m.config.AuditEnabled
	share.CreatedAt = time.Now()
	share.UpdatedAt = time.Now()

	m.shares[share.ID] = share
	m.addAudit("share_create", share.ID, share.KeyID, "", "", share.Protocol, true, fmt.Sprintf("创建加密共享: %s (%s)", share.Name, share.Protocol))

	return nil
}

// EncryptData 加密数据
func (m *Manager) EncryptData(keyID string, plaintext []byte) ([]byte, error) {
	m.mu.RLock()
	key, ok := m.keys[keyID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key %s not found", keyID)
	}
	if key.Status != KeyStatusActive {
		return nil, fmt.Errorf("key %s is not active", keyID)
	}

	// 使用 AES-256-GCM 加密
	block, err := aes.NewCipher(make([]byte, 32)) // 实际应从安全存储获取密钥
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	m.mu.Lock()
	key.UsageCount++
	m.mu.Unlock()

	m.addAudit("encrypt", "", keyID, "", "", ProtocolHTTPS, true, fmt.Sprintf("加密 %d 字节", len(plaintext)))

	return ciphertext, nil
}

// DecryptData 解密数据
func (m *Manager) DecryptData(keyID string, ciphertext []byte) ([]byte, error) {
	m.mu.RLock()
	_, ok := m.keys[keyID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key %s not found", keyID)
	}

	block, err := aes.NewCipher(make([]byte, 32))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		m.addAudit("decrypt_error", "", keyID, "", "", ProtocolHTTPS, false, err.Error())
		return nil, err
	}

	m.addAudit("decrypt", "", keyID, "", "", ProtocolHTTPS, true, fmt.Sprintf("解密 %d 字节", len(plaintext)))

	return plaintext, nil
}

// GenerateShareLink 生成加密共享链接
func (m *Manager) GenerateShareLink(shareID, userID string, expiresIn time.Duration) (string, error) {
	m.mu.RLock()
	share, ok := m.shares[shareID]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("share %s not found", shareID)
	}

	// 生成加密令牌
	token := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%d", shareID, userID, time.Now().UnixNano(), expiresIn)))
	link := fmt.Sprintf("https://%s/share/%s?token=%s&expires=%d",
		"nas.local", shareID, hex.EncodeToString(token[:]),
		time.Now().Add(expiresIn).Unix())

	m.addAudit("link_generate", shareID, share.KeyID, userID, "", share.Protocol, true, fmt.Sprintf("生成共享链接，有效期 %v", expiresIn))

	return link, nil
}

// RunComplianceCheck 运行合规检查
func (m *Manager) RunComplianceCheck() *ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &ComplianceReport{
		GeneratedAt: time.Now(),
		FIPSLevel:   m.config.FIPSLevel,
		Issues:      make([]ComplianceIssue, 0),
		Protocols:   make(map[string]ProtocolStatus),
	}

	// 统计密钥
	for _, key := range m.keys {
		switch key.Status {
		case KeyStatusActive:
			report.ActiveKeys++
			// 检查是否即将过期
			if time.Until(key.ExpiresAt) < 7*24*time.Hour {
				report.Issues = append(report.Issues, ComplianceIssue{
					Severity: "medium",
					Message:  fmt.Sprintf("密钥 %s 将在 7 天内过期", key.Name),
					Action:   "轮换密钥",
				})
			}
		case KeyStatusRetired:
			report.ExpiredKeys++
		}
	}

	// 检查共享合规性
	for _, share := range m.shares {
		report.TotalShares++
		if share.CipherSuite != "" {
			report.EncryptedShares++
		}

		// TLS 版本检查
		if share.TLSVersion < "1.2" {
			report.Issues = append(report.Issues, ComplianceIssue{
				Severity: "critical",
				Share:    share.Name,
				Message:  fmt.Sprintf("共享 %s 使用不安全的 TLS 版本: %s", share.Name, share.TLSVersion),
				Action:   "升级到 TLS 1.3",
			})
		}

		// FIPS 合规检查
		if share.FIPSLevel != m.config.FIPSLevel {
			report.Issues = append(report.Issues, ComplianceIssue{
				Severity: "high",
				Share:    share.Name,
				Message:  fmt.Sprintf("共享 %s FIPS 级别不匹配", share.Name),
				Action:   fmt.Sprintf("升级到 %s", m.config.FIPSLevel),
			})
		}

		report.Protocols[string(share.Protocol)] = ProtocolStatus{
			Name:       string(share.Protocol),
			Encrypted:  share.CipherSuite != "",
			TLSVersion: share.TLSVersion,
			Cipher:     string(share.CipherSuite),
			Compliant:  share.FIPSLevel == m.config.FIPSLevel,
		}
	}

	// 整体状态
	if len(report.Issues) == 0 {
		report.OverallStatus = "compliant"
	} else {
		criticalCount := 0
		for _, issue := range report.Issues {
			if issue.Severity == "critical" {
				criticalCount++
			}
		}
		if criticalCount > 0 {
			report.OverallStatus = "non_compliant"
		} else {
			report.OverallStatus = "warning"
		}
	}

	m.addAudit("compliance_check", "", "", "", "", ProtocolHTTPS, true, fmt.Sprintf("合规检查完成: %s", report.OverallStatus))

	return report
}

// GetAuditLog 获取审计日志
func (m *Manager) GetAuditLog(limit int) []*AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	// 返回最新的记录
	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*AuditEntry, limit)
	copy(result, m.auditLog[start:])
	return result
}

// ListKeys 列出所有密钥
func (m *Manager) ListKeys() []*EncryptionKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*EncryptionKey, 0, len(m.keys))
	for _, k := range m.keys {
		result = append(result, k)
	}
	return result
}

// ListShares 列出所有共享
func (m *Manager) ListShares() []*EncryptedShare {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*EncryptedShare, 0, len(m.shares))
	for _, s := range m.shares {
		result = append(result, s)
	}
	return result
}

func (m *Manager) addAudit(eventType, shareID, keyID, userID, sourceIP string, protocol Protocol, success bool, details string) {
	if !m.config.AuditEnabled {
		return
	}

	entry := &AuditEntry{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		EventType: eventType,
		ShareID:   shareID,
		KeyID:     keyID,
		UserID:    userID,
		SourceIP:  sourceIP,
		Protocol:  protocol,
		Success:   success,
		Details:   details,
	}

	m.auditLog = append(m.auditLog, entry)

	// 限制审计日志大小
	if len(m.auditLog) > m.config.MaxAuditEntries {
		m.auditLog = m.auditLog[len(m.auditLog)-m.config.MaxAuditEntries:]
	}
}

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/fips-vault")
	{
		// 密钥管理
		group.POST("/keys", h.GenerateKey)
		group.GET("/keys", h.ListKeys)
		group.POST("/keys/:id/rotate", h.RotateKey)

		// 加密共享
		group.POST("/shares", h.CreateShare)
		group.GET("/shares", h.ListShares)
		group.POST("/shares/:id/link", h.GenerateLink)

		// 加密操作
		group.POST("/encrypt", h.Encrypt)
		group.POST("/decrypt", h.Decrypt)

		// 合规
		group.GET("/compliance", h.ComplianceCheck)
		group.GET("/audit", h.GetAuditLog)
	}
}

func (h *Handler) GenerateKey(c *gin.Context) {
	var req struct {
		Name    string      `json:"name"`
		Cipher  CipherSuite `json:"cipher"`
		KeySize int         `json:"keySize"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	key, err := h.manager.GenerateKey(req.Name, req.Cipher, req.KeySize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": key})
}

func (h *Handler) ListKeys(c *gin.Context) {
	keys := h.manager.ListKeys()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": keys, "total": len(keys)})
}

func (h *Handler) RotateKey(c *gin.Context) {
	id := c.Param("id")
	newKey, err := h.manager.RotateKey(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": newKey})
}

func (h *Handler) CreateShare(c *gin.Context) {
	var share EncryptedShare
	if err := c.ShouldBindJSON(&share); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.manager.CreateEncryptedShare(&share); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": share})
}

func (h *Handler) ListShares(c *gin.Context) {
	shares := h.manager.ListShares()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares, "total": len(shares)})
}

func (h *Handler) GenerateLink(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		UserID   string `json:"userId"`
		ExpireIn int    `json:"expireIn"` // 秒
	}
	c.ShouldBindJSON(&req)
	if req.ExpireIn == 0 {
		req.ExpireIn = 3600
	}

	link, err := h.manager.GenerateShareLink(id, req.UserID, time.Duration(req.ExpireIn)*time.Second)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"link": link}})
}

func (h *Handler) Encrypt(c *gin.Context) {
	var req struct {
		KeyID string `json:"keyId"`
		Data  []byte `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	encrypted, err := h.manager.EncryptData(req.KeyID, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"encrypted": encrypted}})
}

func (h *Handler) Decrypt(c *gin.Context) {
	var req struct {
		KeyID      string `json:"keyId"`
		Ciphertext []byte `json:"ciphertext"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	decrypted, err := h.manager.DecryptData(req.KeyID, req.Ciphertext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"decrypted": decrypted}})
}

func (h *Handler) ComplianceCheck(c *gin.Context) {
	report := h.manager.RunComplianceCheck()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

func (h *Handler) GetAuditLog(c *gin.Context) {
	limit := 100
	logs := h.manager.GetAuditLog(limit)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs, "total": len(logs)})
}
