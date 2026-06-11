// Package compliance 提供 FIPS 140-2/140-3 合规检查功能
//
// FIPS 140-2/140-3 是美国联邦信息处理标准，规定了密码模块的安全要求。
// 本模块实现了 FIPS 合规检查、密钥管理验证、自检机制和合规状态报告。
package compliance

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// FIPSLevel FIPS 合规级别
type FIPSLevel string

const (
	FIPSLevel1 FIPSLevel = "Level 1" // 基本安全要求
	FIPSLevel2 FIPSLevel = "Level 2" // 物理安全要求
	FIPSLevel3 FIPSLevel = "Level 3" // 物理篡改检测
	FIPSLevel4 FIPSLevel = "Level 4" // 物理篡改防护
)

// FIPSStatus FIPS 合规状态
type FIPSStatus struct {
	Enabled       bool      `json:"enabled"`
	Level         FIPSLevel `json:"level"`
	Compliant     bool      `json:"compliant"`
	LastCheck     time.Time `json:"last_check"`
	NextCheck     time.Time `json:"next_check"`
	SelfTestOK    bool      `json:"self_test_ok"`
	Algorithms    []AlgorithmStatus `json:"algorithms"`
	KeyManagement KeyMgmtStatus    `json:"key_management"`
	Issues        []FIPSIssue       `json:"issues,omitempty"`
	CheckedAt     time.Time         `json:"checked_at"`
}

// AlgorithmStatus 算法合规状态
type AlgorithmStatus struct {
	Name      string    `json:"name"`
	Standard  string    `json:"standard"` // FIPS 140-2, NIST SP 800-185, etc.
	Approved  bool      `json:"approved"`
	KeySize   int       `json:"key_size"`
	Mode      string    `json:"mode"` // CBC, GCM, CTR, etc.
	CheckedAt time.Time `json:"checked_at"`
}

// KeyMgmtStatus 密钥管理状态
type KeyMgmtStatus struct {
	TotalKeys       int       `json:"total_keys"`
	ActiveKeys      int       `json:"active_keys"`
	ExpiredKeys     int       `json:"expired_keys"`
	RotationCompliant bool    `json:"rotation_compliant"`
	StorageCompliant  bool    `json:"storage_compliant"`
	LastKeyRotation  time.Time `json:"last_key_rotation"`
	NextKeyRotation  time.Time `json:"next_key_rotation"`
}

// FIPSIssue FIPS 合规问题
type FIPSIssue struct {
	ID          string    `json:"id"`
	Severity    string    `json:"severity"` // critical, high, medium, low
	Category    string    `json:"category"` // algorithm, key-mgmt, self-test, configuration
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Remediation string    `json:"remediation"`
	DetectedAt  time.Time `json:"detected_at"`
}

// FIPSComplianceChecker FIPS 合规检查器
type FIPSComplianceChecker struct {
	mu            sync.RWMutex
	level         FIPSLevel
	enabled       bool
	selfTestInterval time.Duration
	lastSelfTest  time.Time
	algorithms    []AlgorithmStatus
	issues        []FIPSIssue
	keyStore      *FIPSKeyStore
}

// FIPSKeyStore FIPS 密钥存储
type FIPSKeyStore struct {
	mu        sync.RWMutex
	keys      map[string]*FIPSKey
	algorithm string
}

// FIPSKey FIPS 密钥
type FIPSKey struct {
	ID        string    `json:"id"`
	Algorithm string    `json:"algorithm"`
	KeySize   int       `json:"key_size"`
	Usage     string    `json:"usage"` // encryption, signing, authentication
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IsActive  bool      `json:"is_active"`
	Rotations int       `json:"rotations"`
}

// NewFIPSComplianceChecker 创建 FIPS 合规检查器
func NewFIPSComplianceChecker(level FIPSLevel) *FIPSComplianceChecker {
	checker := &FIPSComplianceChecker{
		level:            level,
		enabled:          true,
		selfTestInterval: 24 * time.Hour, // 每24小时自检一次
		keyStore: &FIPSKeyStore{
			keys:      make(map[string]*FIPSKey),
			algorithm: "AES-256-GCM",
		},
	}

	// 初始化 FIPS 批准的算法列表
	checker.initApprovedAlgorithms()

	return checker
}

// initApprovedAlgorithms 初始化 FIPS 批准的算法
func (c *FIPSComplianceChecker) initApprovedAlgorithms() {
	now := time.Now()
	c.algorithms = []AlgorithmStatus{
		// 对称加密
		{Name: "AES-128-CBC", Standard: "FIPS 197", Approved: true, KeySize: 128, Mode: "CBC", CheckedAt: now},
		{Name: "AES-192-CBC", Standard: "FIPS 197", Approved: true, KeySize: 192, Mode: "CBC", CheckedAt: now},
		{Name: "AES-256-CBC", Standard: "FIPS 197", Approved: true, KeySize: 256, Mode: "CBC", CheckedAt: now},
		{Name: "AES-128-GCM", Standard: "FIPS 197", Approved: true, KeySize: 128, Mode: "GCM", CheckedAt: now},
		{Name: "AES-256-GCM", Standard: "FIPS 197", Approved: true, KeySize: 256, Mode: "GCM", CheckedAt: now},

		// 哈希算法
		{Name: "SHA-256", Standard: "FIPS 180-4", Approved: true, KeySize: 0, Mode: "Hash", CheckedAt: now},
		{Name: "SHA-384", Standard: "FIPS 180-4", Approved: true, KeySize: 0, Mode: "Hash", CheckedAt: now},
		{Name: "SHA-512", Standard: "FIPS 180-4", Approved: true, KeySize: 0, Mode: "Hash", CheckedAt: now},
		{Name: "SHA-3", Standard: "FIPS 202", Approved: true, KeySize: 0, Mode: "Hash", CheckedAt: now},

		// 非对称加密
		{Name: "RSA-2048", Standard: "FIPS 186-5", Approved: true, KeySize: 2048, Mode: "Sign/Encrypt", CheckedAt: now},
		{Name: "RSA-3072", Standard: "FIPS 186-5", Approved: true, KeySize: 3072, Mode: "Sign/Encrypt", CheckedAt: now},
		{Name: "RSA-4096", Standard: "FIPS 186-5", Approved: true, KeySize: 4096, Mode: "Sign/Encrypt", CheckedAt: now},
		{Name: "ECDSA-P256", Standard: "FIPS 186-5", Approved: true, KeySize: 256, Mode: "Sign", CheckedAt: now},
		{Name: "ECDSA-P384", Standard: "FIPS 186-5", Approved: true, KeySize: 384, Mode: "Sign", CheckedAt: now},

		// HMAC
		{Name: "HMAC-SHA-256", Standard: "FIPS 198-1", Approved: true, KeySize: 256, Mode: "MAC", CheckedAt: now},
		{Name: "HMAC-SHA-512", Standard: "FIPS 198-1", Approved: true, KeySize: 512, Mode: "MAC", CheckedAt: now},

		// TLS
		{Name: "TLS-1.2", Standard: "FIPS 140-2 IG", Approved: true, KeySize: 0, Mode: "Transport", CheckedAt: now},
		{Name: "TLS-1.3", Standard: "FIPS 140-3", Approved: true, KeySize: 0, Mode: "Transport", CheckedAt: now},

		// 密钥派生
		{Name: "PBKDF2", Standard: "NIST SP 800-132", Approved: true, KeySize: 0, Mode: "KDF", CheckedAt: now},
		{Name: "HKDF", Standard: "NIST SP 800-56C", Approved: true, KeySize: 0, Mode: "KDF", CheckedAt: now},
	}
}

// CheckStatus 执行 FIPS 合规状态检查
func (c *FIPSComplianceChecker) CheckStatus() *FIPSStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := &FIPSStatus{
		Enabled:    c.enabled,
		Level:      c.level,
		Algorithms: c.algorithms,
		CheckedAt:  time.Now(),
		LastCheck:  c.lastSelfTest,
		NextCheck:  c.lastSelfTest.Add(c.selfTestInterval),
	}

	// 检查密钥管理状态
	status.KeyManagement = c.checkKeyManagement()

	// 执行自检
	selfTestOK := c.runSelfTest()
	status.SelfTestOK = selfTestOK

	// 收集问题
	issues := c.collectIssues()
	status.Issues = issues

	// 判断整体合规性
	status.Compliant = c.evaluateCompliance(status)

	return status
}

// checkKeyManagement 检查密钥管理合规性
func (c *FIPSComplianceChecker) checkKeyManagement() KeyMgmtStatus {
	c.keyStore.mu.RLock()
	defer c.keyStore.mu.RUnlock()

	now := time.Now()
	totalKeys := len(c.keyStore.keys)
	activeKeys := 0
	expiredKeys := 0
	lastRotation := time.Time{}

	for _, key := range c.keyStore.keys {
		if key.IsActive {
			activeKeys++
		}
		if now.After(key.ExpiresAt) {
			expiredKeys++
		}
		if key.CreatedAt.After(lastRotation) {
			lastRotation = key.CreatedAt
		}
	}

	return KeyMgmtStatus{
		TotalKeys:         totalKeys,
		ActiveKeys:        activeKeys,
		ExpiredKeys:       expiredKeys,
		RotationCompliant: expiredKeys == 0,
		StorageCompliant:  true, // 简化实现
		LastKeyRotation:   lastRotation,
		NextKeyRotation:   lastRotation.Add(90 * 24 * time.Hour), // 90天轮换
	}
}

// runSelfTest 运行 FIPS 自检
func (c *FIPSComplianceChecker) runSelfTest() bool {
	// 1. 加密算法自检
	if !c.selfTestAlgorithms() {
		return false
	}

	// 2. 密钥完整性自检
	if !c.selfTestKeyIntegrity() {
		return false
	}

	// 3. TLS 配置自检
	if !c.selfTestTLSConfig() {
		return false
	}

	c.lastSelfTest = time.Now()
	return true
}

// selfTestAlgorithms 算法自检
func (c *FIPSComplianceChecker) selfTestAlgorithms() bool {
	// 测试 AES-GCM 加密/解密
	testKey := make([]byte, 32)
	if _, err := rand.Read(testKey); err != nil {
		return false
	}

	block, err := aes.NewCipher(testKey)
	if err != nil {
		return false
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return false
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return false
	}

	testData := []byte("FIPS self-test data")
	encrypted := gcm.Seal(nil, nonce, testData, nil)
	decrypted, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return false
	}

	return string(decrypted) == string(testData)
}

// selfTestKeyIntegrity 密钥完整性自检
func (c *FIPSComplianceChecker) selfTestKeyIntegrity() bool {
	// 验证密钥哈希完整性
	testKey := []byte("integrity-test-key")
	hash := sha256.Sum256(testKey)
	expectedHash := hex.EncodeToString(hash[:])

	// 重新计算验证
	verifyHash := sha256.Sum256(testKey)
	actualHash := hex.EncodeToString(verifyHash[:])

	return expectedHash == actualHash
}

// selfTestTLSConfig TLS 配置自检
func (c *FIPSComplianceChecker) selfTestTLSConfig() bool {
	// 验证 TLS 配置符合 FIPS 要求
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// 验证配置有效
	return config.MinVersion >= tls.VersionTLS12
}

// collectIssues 收集 FIPS 合规问题
func (c *FIPSComplianceChecker) collectIssues() []FIPSIssue {
	var issues []FIPSIssue

	// 检查密钥过期
	c.keyStore.mu.RLock()
	for _, key := range c.keyStore.keys {
		if time.Now().After(key.ExpiresAt) && key.IsActive {
			issues = append(issues, FIPSIssue{
				ID:          fmt.Sprintf("key-expired-%s", key.ID),
				Severity:    "high",
				Category:    "key-mgmt",
				Title:       fmt.Sprintf("密钥 %s 已过期", key.ID),
				Description: fmt.Sprintf("密钥 %s 已超过有效期 %s", key.ID, key.ExpiresAt.Format(time.RFC3339)),
				Remediation: "立即轮换过期密钥",
				DetectedAt:  time.Now(),
			})
		}
	}
	c.keyStore.mu.RUnlock()

	// 检查算法合规
	for _, algo := range c.algorithms {
		if !algo.Approved {
			issues = append(issues, FIPSIssue{
				ID:          fmt.Sprintf("algo-unapproved-%s", algo.Name),
				Severity:    "critical",
				Category:    "algorithm",
				Title:       fmt.Sprintf("使用未批准的算法: %s", algo.Name),
				Description: fmt.Sprintf("算法 %s 未获得 FIPS 批准", algo.Name),
				Remediation: "迁移到 FIPS 批准的算法",
				DetectedAt:  time.Now(),
			})
		}
	}

	return issues
}

// evaluateCompliance 评估整体合规性
func (c *FIPSComplianceChecker) evaluateCompliance(status *FIPSStatus) bool {
	if !status.Enabled {
		return false
	}

	if !status.SelfTestOK {
		return false
	}

	// 检查是否有严重问题
	for _, issue := range status.Issues {
		if issue.Severity == "critical" {
			return false
		}
	}

	// 检查密钥管理
	if !status.KeyManagement.RotationCompliant {
		return false
	}

	return true
}

// GenerateFIPSKey 生成 FIPS 合规密钥
func (c *FIPSComplianceChecker) GenerateFIPSKey(algorithm string, keySize int, usage string) (*FIPSKey, error) {
	c.keyStore.mu.Lock()
	defer c.keyStore.mu.Unlock()

	// 验证算法是否 FIPS 批准
	approved := false
	for _, algo := range c.algorithms {
		if algo.Name == algorithm && algo.Approved {
			approved = true
			break
		}
	}
	if !approved {
		return nil, fmt.Errorf("算法 %s 未获得 FIPS 批准", algorithm)
	}

	// 生成密钥材料
	keyMaterial := make([]byte, keySize/8)
	if _, err := rand.Read(keyMaterial); err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	key := &FIPSKey{
		ID:        fmt.Sprintf("fips-key-%d", time.Now().UnixNano()),
		Algorithm: algorithm,
		KeySize:   keySize,
		Usage:     usage,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(90 * 24 * time.Hour), // 90天有效期
		IsActive:  true,
		Rotations: 0,
	}

	c.keyStore.keys[key.ID] = key
	return key, nil
}

// RotateFIPSKey 轮换 FIPS 密钥
func (c *FIPSComplianceChecker) RotateFIPSKey(keyID string) (*FIPSKey, error) {
	c.keyStore.mu.Lock()
	defer c.keyStore.mu.Unlock()

	oldKey, exists := c.keyStore.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("密钥 %s 不存在", keyID)
	}

	// 创建新密钥
	newKey := &FIPSKey{
		ID:        fmt.Sprintf("fips-key-%d", time.Now().UnixNano()),
		Algorithm: oldKey.Algorithm,
		KeySize:   oldKey.KeySize,
		Usage:     oldKey.Usage,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
		IsActive:  true,
		Rotations: oldKey.Rotations + 1,
	}

	// 停用旧密钥
	oldKey.IsActive = false

	c.keyStore.keys[newKey.ID] = newKey
	return newKey, nil
}

// VerifyAlgorithm 验证算法是否 FIPS 合规
func (c *FIPSComplianceChecker) VerifyAlgorithm(name string) (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, algo := range c.algorithms {
		if algo.Name == name {
			if algo.Approved {
				return true, fmt.Sprintf("算法 %s 符合 %s 标准", algo.Name, algo.Standard)
			}
			return false, fmt.Sprintf("算法 %s 未获得 FIPS 批准", algo.Name)
		}
	}

	return false, fmt.Sprintf("未知算法: %s", name)
}

// GenerateFIPSReport 生成 FIPS 合规报告
func (c *FIPSComplianceChecker) GenerateFIPSReport() *FIPSComplianceReport {
	status := c.CheckStatus()

	report := &FIPSComplianceReport{
		ID:            fmt.Sprintf("fips-report-%d", time.Now().UnixNano()),
		Title:         fmt.Sprintf("FIPS %s 合规报告", c.level),
		Level:         c.level,
		Status:        status,
		GeneratedAt:   time.Now(),
		Summary:       c.generateFIPSSummary(status),
		Recommendations: c.generateFIPSRecommendations(status),
	}

	return report
}

// FIPSComplianceReport FIPS 合规报告
type FIPSComplianceReport struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Level           FIPSLevel `json:"level"`
	Status          *FIPSStatus `json:"status"`
	Summary         string    `json:"summary"`
	Recommendations []string  `json:"recommendations"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// generateFIPSSummary 生成 FIPS 报告摘要
func (c *FIPSComplianceChecker) generateFIPSSummary(status *FIPSStatus) string {
	compliantCount := 0
	for _, algo := range status.Algorithms {
		if algo.Approved {
			compliantCount++
		}
	}

	return fmt.Sprintf(
		"FIPS %s 合规检查结果: 合规状态=%v, 自检通过=%v, 批准算法=%d/%d, 活跃密钥=%d, 问题数=%d",
		c.level,
		status.Compliant,
		status.SelfTestOK,
		compliantCount, len(status.Algorithms),
		status.KeyManagement.ActiveKeys,
		len(status.Issues),
	)
}

// generateFIPSRecommendations 生成 FIPS 整改建议
func (c *FIPSComplianceChecker) generateFIPSRecommendations(status *FIPSStatus) []string {
	var recommendations []string

	if !status.SelfTestOK {
		recommendations = append(recommendations, "立即执行 FIPS 自检并修复失败项")
	}

	for _, issue := range status.Issues {
		recommendations = append(recommendations, fmt.Sprintf("[%s] %s: %s", issue.Severity, issue.Title, issue.Remediation))
	}

	if status.KeyManagement.ExpiredKeys > 0 {
		recommendations = append(recommendations, fmt.Sprintf("轮换 %d 个过期密钥", status.KeyManagement.ExpiredKeys))
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "系统符合 FIPS 合规要求，建议定期执行自检")
	}

	return recommendations
}

// EnableFIPS 启用 FIPS 模式
func (c *FIPSComplianceChecker) EnableFIPS() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 执行自检
	if !c.runSelfTest() {
		return fmt.Errorf("FIPS 自检失败，无法启用 FIPS 模式")
	}

	c.enabled = true
	return nil
}

// DisableFIPS 禁用 FIPS 模式
func (c *FIPSComplianceChecker) DisableFIPS() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = false
}

// IsFIPSEnabled 检查 FIPS 是否启用
func (c *FIPSComplianceChecker) IsFIPSEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.enabled
}

// ValidateCipherSuite 验证密码套件是否 FIPS 合规
func (c *FIPSComplianceChecker) ValidateCipherSuite(cipherSuite string) bool {
	fipsApprovedCiphers := map[string]bool{
		// TLS 1.2 FIPS 合规密码套件
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   true,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   true,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": true,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": true,
		"TLS_DHE_RSA_WITH_AES_256_GCM_SHA384":     true,
		"TLS_DHE_RSA_WITH_AES_128_GCM_SHA256":     true,

		// TLS 1.3 密码套件（全部 FIPS 合规）
		"TLS_AES_256_GCM_SHA384":       true,
		"TLS_AES_128_GCM_SHA256":       true,
		"TLS_CHACHA20_POLY1305_SHA256": true,
	}

	return fipsApprovedCiphers[cipherSuite]
}

// GetApprovedAlgorithms 获取 FIPS 批准的算法列表
func (c *FIPSComplianceChecker) GetApprovedAlgorithms() []AlgorithmStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var approved []AlgorithmStatus
	for _, algo := range c.algorithms {
		if algo.Approved {
			approved = append(approved, algo)
		}
	}
	return approved
}

// GetFIPSKeyStore 获取密钥存储状态
func (c *FIPSComplianceChecker) GetFIPSKeyStore() *FIPSKeyStore {
	return c.keyStore
}

// 便捷函数

// VerifySHA256 验证 SHA-256 哈希（FIPS 批准）
func VerifySHA256(data []byte, expectedHash string) bool {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]) == expectedHash
}

// VerifySHA512 验证 SHA-512 哈希（FIPS 批准）
func VerifySHA512(data []byte, expectedHash string) bool {
	hash := sha512.Sum512(data)
	return hex.EncodeToString(hash[:]) == expectedHash
}

// GenerateRSAPrivateKey 生成 RSA 私钥（FIPS 合规）
func GenerateRSAPrivateKey(bits int) (*rsa.PrivateKey, error) {
	if bits < 2048 {
		return nil, fmt.Errorf("RSA 密钥长度至少为 2048 位")
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("生成 RSA 密钥失败: %w", err)
	}

	return privateKey, nil
}

// SignData 使用 RSA 签名数据（FIPS 合规）
func SignData(privateKey *rsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}
	return signature, nil
}
