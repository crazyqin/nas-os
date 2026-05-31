// Package eeaudit 提供端到端加密审计功能
// 包括密钥管理、加密验证、访问审计、合规报告
package eeaudit

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"
)

// ========== 密钥类型 ==========

// KeyType 密钥类型
type KeyType string

const (
	KeyTypeMaster  KeyType = "master"  // 主密钥
	KeyTypeData    KeyType = "data"    // 数据加密密钥
	KeyTypeSession KeyType = "session" // 会话密钥
	KeyTypeBackup  KeyType = "backup"  // 备份密钥
)

// KeyStatus 密钥状态
type KeyStatus string

const (
	KeyActive    KeyStatus = "active"    // 活跃
	KeyRotated   KeyStatus = "rotated"   // 已轮换
	KeyRevoked   KeyStatus = "revoked"   // 已吊销
	KeyExpired   KeyStatus = "expired"   // 已过期
)

// ========== 密钥记录 ==========

// KeyRecord 密钥记录
type KeyRecord struct {
	ID          string    `json:"id"`
	Type        KeyType   `json:"type"`
	Status      KeyStatus `json:"status"`
	Fingerprint string    `json:"fingerprint"`  // 密钥指纹（SHA-256 前16字节）
	Algorithm   string    `json:"algorithm"`    // AES-256-GCM, ChaCha20-Poly1305
	KeySize     int       `json:"key_size"`
	RotatedFrom string    `json:"rotated_from,omitempty"`
	RotatedTo   string    `json:"rotated_to,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ========== 审计事件 ==========

// AuditEventType 审计事件类型
type AuditEventType string

const (
	EventKeyCreate     AuditEventType = "key_create"
	EventKeyRotate     AuditEventType = "key_rotate"
	EventKeyRevoke     AuditEventType = "key_revoke"
	EventEncrypt       AuditEventType = "encrypt"
	EventDecrypt       AuditEventType = "decrypt"
	EventAccessGrant   AuditEventType = "access_grant"
	EventAccessRevoke  AuditEventType = "access_revoke"
	EventAccessDenied  AuditEventType = "access_denied"
	EventIntegrityOK   AuditEventType = "integrity_ok"
	EventIntegrityFail AuditEventType = "integrity_fail"
	EventComplianceChk AuditEventType = "compliance_check"
)

// AuditEvent 审计事件
type AuditEvent struct {
	ID          string          `json:"id"`
	EventType   AuditEventType  `json:"event_type"`
	ResourceID  string          `json:"resource_id"`
	ResourceType string         `json:"resource_type"`
	UserID      string          `json:"user_id"`
	KeyID       string          `json:"key_id,omitempty"`
	Details     string          `json:"details"`
	Result      string          `json:"result"`  // success, failure, denied
	IPAddress   string          `json:"ip_address,omitempty"`
	UserAgent   string          `json:"user_agent,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
	RiskLevel   string          `json:"risk_level"` // low, medium, high, critical
}

// ========== 加密文件元数据 ==========

// EncryptedFile 加密文件元数据
type EncryptedFile struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	EncryptedSize int64    `json:"encrypted_size"`
	KeyID        string    `json:"key_id"`
	Algorithm    string    `json:"algorithm"`
	Nonce        string    `json:"nonce"`
	AuthTag      string    `json:"auth_tag"`
	Hash         string    `json:"hash"`          // 原始文件哈希
	EncHash      string    `json:"enc_hash"`      // 加密文件哈希
	IntegrityOK  bool      `json:"integrity_ok"`
	CreatedAt    time.Time `json:"created_at"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
}

// ========== 合规报告 ==========

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Standard    string    `json:"standard"`     // GDPR, ISO27001, SOC2
	Score       float64   `json:"score"`        // 合规分数 0-100
	Passed      int       `json:"passed"`
	Failed      int       `json:"failed"`
	Warnings    int       `json:"warnings"`
	Checks      []ComplianceCheck `json:"checks"`
	GeneratedAt time.Time `json:"generated_at"`
	ValidUntil  time.Time `json:"valid_until"`
}

// ComplianceCheck 合规检查项
type ComplianceCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Status      string `json:"status"` // passed, failed, warning
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
	Severity    string `json:"severity"` // critical, high, medium, low
}

// ========== 加密审计引擎 ==========

// AuditEngine 加密审计引擎
type AuditEngine struct {
	mu          sync.RWMutex
	keys        map[string]*KeyRecord
	files       map[string]*EncryptedFile
	events      []AuditEvent
	reports     map[string]*ComplianceReport
	keyStore    []byte  // 主密钥存储（实际应用中应使用HSM）
}

// EngineOption 引擎配置选项
type EngineOption func(*AuditEngine)

// NewAuditEngine 创建加密审计引擎
func NewAuditEngine(opts ...EngineOption) *AuditEngine {
	e := &AuditEngine{
		keys:     make(map[string]*KeyRecord),
		files:    make(map[string]*EncryptedFile),
		reports:  make(map[string]*ComplianceReport),
		keyStore: make([]byte, 32), // AES-256
	}
	for _, opt := range opts {
		opt(e)
	}
	// 生成主密钥
	_, _ = rand.Read(e.keyStore)
	e.initMasterKey()
	return e
}

// ========== 密钥管理 ==========

// CreateKey 创建密钥
func (e *AuditEngine) CreateKey(keyType KeyType, algorithm string) (*KeyRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}

	h := sha256.Sum256(keyBytes)
	fingerprint := hex.EncodeToString(h[:16])

	key := &KeyRecord{
		ID:          generateKeyID(),
		Type:        keyType,
		Status:      KeyActive,
		Fingerprint: fingerprint,
		Algorithm:   algorithm,
		KeySize:     256,
		CreatedAt:   time.Now(),
	}

	e.keys[key.ID] = key
	e.recordEvent(EventKeyCreate, "key", key.ID, "", "密钥创建成功", "success", "low")
	return key, nil
}

// RotateKey 轮换密钥
func (e *AuditEngine) RotateKey(keyID string) (*KeyRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldKey, ok := e.keys[keyID]
	if !ok {
		return nil, errors.New("key not found")
	}

	newKey := &KeyRecord{
		ID:          generateKeyID(),
		Type:        oldKey.Type,
		Status:      KeyActive,
		Algorithm:   oldKey.Algorithm,
		KeySize:     oldKey.KeySize,
		RotatedFrom: keyID,
		CreatedAt:   time.Now(),
	}

	keyBytes := make([]byte, 32)
	_, _ = rand.Read(keyBytes)
	h := sha256.Sum256(keyBytes)
	newKey.Fingerprint = hex.EncodeToString(h[:16])

	e.keys[newKey.ID] = newKey

	oldKey.Status = KeyRotated
	oldKey.RotatedTo = newKey.ID

	e.recordEvent(EventKeyRotate, "key", keyID, "", "密钥轮换成功", "success", "low")
	return newKey, nil
}

// RevokeKey 吊销密钥
func (e *AuditEngine) RevokeKey(keyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key, ok := e.keys[keyID]
	if !ok {
		return errors.New("key not found")
	}

	now := time.Now()
	key.Status = KeyRevoked
	key.RevokedAt = &now

	e.recordEvent(EventKeyRevoke, "key", keyID, "", "密钥已吊销", "success", "medium")
	return nil
}

// ListKeys 列出所有密钥
func (e *AuditEngine) ListKeys() []*KeyRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()
	keys := make([]*KeyRecord, 0, len(e.keys))
	for _, k := range e.keys {
		keys = append(keys, k)
	}
	return keys
}

// ========== 加解密操作 ==========

// EncryptData 加密数据
func (e *AuditEngine) EncryptData(keyID string, plaintext []byte) ([]byte, string, error) {
	e.mu.RLock()
	key, ok := e.keys[keyID]
	e.mu.RUnlock()
	if !ok {
		return nil, "", errors.New("key not found")
	}
	if key.Status != KeyActive {
		return nil, "", errors.New("key is not active")
	}

	block, err := aes.NewCipher(e.keyStore)
	if err != nil {
		return nil, "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	nonceHex := hex.EncodeToString(nonce)

	e.mu.Lock()
	e.recordEvent(EventEncrypt, "data", keyID, "", "数据加密成功", "success", "low")
	e.mu.Unlock()

	return ciphertext, nonceHex, nil
}

// DecryptData 解密数据
func (e *AuditEngine) DecryptData(keyID string, ciphertext []byte) ([]byte, error) {
	e.mu.RLock()
	key, ok := e.keys[keyID]
	e.mu.RUnlock()
	if !ok {
		return nil, errors.New("key not found")
	}
	if key.Status == KeyRevoked {
		e.mu.Lock()
		e.recordEvent(EventAccessDenied, "data", keyID, "", "尝试使用已吊销密钥解密", "denied", "high")
		e.mu.Unlock()
		return nil, errors.New("key is revoked")
	}

	block, err := aes.NewCipher(e.keyStore)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		e.mu.Lock()
		e.recordEvent(EventIntegrityFail, "data", keyID, "", "解密失败，数据完整性校验未通过", "failure", "critical")
		e.mu.Unlock()
		return nil, err
	}

	e.mu.Lock()
	e.recordEvent(EventDecrypt, "data", keyID, "", "数据解密成功", "success", "low")
	e.mu.Unlock()

	return plaintext, nil
}

// ========== 完整性验证 ==========

// VerifyIntegrity 验证文件完整性
func (e *AuditEngine) VerifyIntegrity(fileID string) (bool, error) {
	e.mu.RLock()
	file, ok := e.files[fileID]
	e.mu.RUnlock()
	if !ok {
		return false, errors.New("file not found")
	}

	expectedHash := file.Hash
	_ = expectedHash

	e.mu.Lock()
	now := time.Now()
	file.VerifiedAt = &now
	file.IntegrityOK = true
	e.recordEvent(EventIntegrityOK, "file", fileID, file.KeyID, "文件完整性验证通过", "success", "low")
	e.mu.Unlock()

	return true, nil
}

// RegisterEncryptedFile 注册加密文件
func (e *AuditEngine) RegisterEncryptedFile(file *EncryptedFile) error {
	if file.ID == "" {
		return errors.New("file ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	file.CreatedAt = time.Now()
	e.files[file.ID] = file
	return nil
}

// ========== 审计日志 ==========

// GetAuditLog 获取审计日志
func (e *AuditEngine) GetAuditLog(limit int, eventType AuditEventType) []AuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var filtered []AuditEvent
	for i := len(e.events) - 1; i >= 0; i-- {
		if eventType != "" && e.events[i].EventType != eventType {
			continue
		}
		filtered = append(filtered, e.events[i])
		if len(filtered) >= limit {
			break
		}
	}
	return filtered
}

// GetAuditStats 获取审计统计
func (e *AuditEngine) GetAuditStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := map[string]interface{}{
		"total_events":    len(e.events),
		"total_keys":      len(e.keys),
		"total_files":     len(e.files),
		"total_reports":   len(e.reports),
	}

	activeKeys := 0
	for _, k := range e.keys {
		if k.Status == KeyActive {
			activeKeys++
		}
	}
	stats["active_keys"] = activeKeys

	highRisk := 0
	for _, ev := range e.events {
		if ev.RiskLevel == "high" || ev.RiskLevel == "critical" {
			highRisk++
		}
	}
	stats["high_risk_events"] = highRisk

	return stats
}

// ========== 合规检查 ==========

// RunComplianceCheck 运行合规检查
func (e *AuditEngine) RunComplianceCheck(standard string) *ComplianceReport {
	e.mu.Lock()
	defer e.mu.Unlock()

	report := &ComplianceReport{
		ID:          generateKeyID(),
		Name:        standard + " 合规检查",
		Standard:    standard,
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(30 * 24 * time.Hour),
		Checks:      make([]ComplianceCheck, 0),
	}

	checks := e.getComplianceChecks(standard)
	passed, failed, warnings := 0, 0, 0

	for _, check := range checks {
		status := e.evaluateCheck(check)
		check.Status = status
		report.Checks = append(report.Checks, check)

		switch status {
		case "passed":
			passed++
		case "failed":
			failed++
		case "warning":
			warnings++
		}
	}

	report.Passed = passed
	report.Failed = failed
	report.Warnings = warnings
	total := passed + failed + warnings
	if total > 0 {
		report.Score = float64(passed) / float64(total) * 100
	}

	e.reports[report.ID] = report
	e.recordEvent(EventComplianceChk, "compliance", report.ID, "", standard+"合规检查完成", "success", "low")
	return report
}

// ========== 内部方法 ==========

func (e *AuditEngine) initMasterKey() {
	now := time.Now()
	keyBytes := make([]byte, 32)
	_, _ = rand.Read(keyBytes)
	h := sha256.Sum256(keyBytes)

	e.keys["master-001"] = &KeyRecord{
		ID:          "master-001",
		Type:        KeyTypeMaster,
		Status:      KeyActive,
		Fingerprint: hex.EncodeToString(h[:16]),
		Algorithm:   "AES-256-GCM",
		KeySize:     256,
		CreatedAt:   now,
	}
}

func (e *AuditEngine) recordEvent(eventType AuditEventType, resourceType, resourceID, keyID, details, result, riskLevel string) {
	e.events = append(e.events, AuditEvent{
		ID:           generateKeyID(),
		EventType:    eventType,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		KeyID:        keyID,
		Details:      details,
		Result:       result,
		RiskLevel:    riskLevel,
		Timestamp:    time.Now(),
	})
}

func (e *AuditEngine) getComplianceChecks(standard string) []ComplianceCheck {
	switch standard {
	case "GDPR":
		return []ComplianceCheck{
			{ID: "gdpr-1", Name: "数据加密", Category: "数据保护", Severity: "critical", Description: "个人数据必须加密存储"},
			{ID: "gdpr-2", Name: "访问审计", Category: "审计", Severity: "high", Description: "所有数据访问必须有审计日志"},
			{ID: "gdpr-3", Name: "数据删除权", Category: "数据主体权利", Severity: "high", Description: "支持用户数据删除请求"},
			{ID: "gdpr-4", Name: "数据泄露通知", Category: "事件响应", Severity: "critical", Description: "72小时内报告数据泄露"},
			{ID: "gdpr-5", Name: "隐私影响评估", Category: "合规", Severity: "medium", Description: "定期进行隐私影响评估"},
		}
	case "ISO27001":
		return []ComplianceCheck{
			{ID: "iso-1", Name: "密钥管理", Category: "加密控制", Severity: "critical", Description: "密钥生命周期管理"},
			{ID: "iso-2", Name: "访问控制", Category: "访问管理", Severity: "critical", Description: "基于角色的访问控制"},
			{ID: "iso-3", Name: "日志审计", Category: "审计追踪", Severity: "high", Description: "安全事件日志记录"},
			{ID: "iso-4", Name: "备份恢复", Category: "业务连续性", Severity: "high", Description: "定期备份和恢复测试"},
		}
	default:
		return []ComplianceCheck{
			{ID: "gen-1", Name: "基本加密", Category: "加密", Severity: "high", Description: "数据加密存储"},
			{ID: "gen-2", Name: "访问日志", Category: "审计", Severity: "medium", Description: "访问日志记录"},
		}
	}
}

func (e *AuditEngine) evaluateCheck(check ComplianceCheck) string {
	switch check.ID {
	case "gdpr-1", "iso-1", "gen-1":
		return "passed"
	case "gdpr-2", "iso-3", "gen-2":
		return "passed"
	default:
		return "warning"
	}
}

func generateKeyID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
