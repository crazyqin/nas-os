// Package sanitization 提供AI数据去敏感化功能
// 支持敏感信息自动识别和本地化处理，不上传云端
package sanitization

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ========== 核心错误定义 ==========

var (
	// ErrSensitiveDataFound 发现敏感数据
	ErrSensitiveDataFound = errors.New("sensitive data found")
	// ErrSanitizationFailed 去敏感化失败
	ErrSanitizationFailed = errors.New("sanitization failed")
	// ErrInvalidPattern 无效模式
	ErrInvalidPattern = errors.New("invalid pattern")
	// ErrEncryptionFailed 加密失败
	ErrEncryptionFailed = errors.New("encryption failed")
	// ErrDecryptionFailed 解密失败
	ErrDecryptionFailed = errors.New("decryption failed")
	// ErrKeyNotFound 密钥未找到
	ErrKeyNotFound = errors.New("encryption key not found")
	// ErrUnauthorizedAccess 未授权访问
	ErrUnauthorizedAccess = errors.New("unauthorized access")
)

// ========== 敏感信息类型 ==========

// SensitiveType 敏感信息类型
type SensitiveType string

const (
	SensitiveTypeCreditCard    SensitiveType = "credit_card"
	SensitiveTypeSSN           SensitiveType = "ssn"
	SensitiveTypePassport      SensitiveType = "passport"
	SensitiveTypePhone         SensitiveType = "phone"
	SensitiveTypeEmail         SensitiveType = "email"
	SensitiveTypeIP            SensitiveType = "ip_address"
	SensitiveTypePassword      SensitiveType = "password"
	SensitiveTypeAPIKey        SensitiveType = "api_key"
	SensitiveTypeBankAccount   SensitiveType = "bank_account"
	SensitiveTypeName          SensitiveType = "name"
	SensitiveTypeAddress       SensitiveType = "address"
	SensitiveTypeIDCard        SensitiveType = "id_card"
	SensitiveTypeCustom        SensitiveType = "custom"
)

// SensitiveData 敏感数据
type SensitiveData struct {
	Type        SensitiveType `json:"type"`
	Value       string        `json:"value"`
	StartPos    int           `json:"start_pos"`
	EndPos      int           `json:"end_pos"`
	Confidence  float64       `json:"confidence"`
	Context     string        `json:"context,omitempty"`
	Replacement string        `json:"replacement,omitempty"`
}

// SanitizationResult 去敏感化结果
type SanitizationResult struct {
	OriginalText     string           `json:"original_text,omitempty"`
	SanitizedText    string           `json:"sanitized_text"`
	DetectedItems    []SensitiveData  `json:"detected_items"`
	Stats            SanitizationStats `json:"stats"`
	Timestamp        time.Time        `json:"timestamp"`
	ProcessingTimeMS int64            `json:"processing_time_ms"`
}

// SanitizationStats 统计信息
type SanitizationStats struct {
	TotalScanned      int `json:"total_scanned"`
	TotalDetected     int `json:"total_detected"`
	TotalReplaced     int `json:"total_replaced"`
	TotalRedacted     int `json:"total_redacted"`
	TotalEncrypted    int `json:"total_encrypted"`
	ByType            map[SensitiveType]int `json:"by_type"`
}

// SanitizationMode 去敏感化模式
type SanitizationMode string

const (
	// ModeRedact 完全移除敏感信息
	ModeRedact SanitizationMode = "redact"
	// ModeMask 部分遮蔽
	ModeMask SanitizationMode = "mask"
	// ModeReplace 替换为占位符
	ModeReplace SanitizationMode = "replace"
	// ModeEncrypt 加密存储
	ModeEncrypt SanitizationMode = "encrypt"
	// ModeHash 哈希替换
	ModeHash SanitizationMode = "hash"
	// ModeTokenize 令牌化
	ModeTokenize SanitizationMode = "tokenize"
)

// SanitizationConfig 去敏感化配置
type SanitizationConfig struct {
	// 默认模式
	DefaultMode SanitizationMode `json:"default_mode"`
	
	// 各类型的模式配置
	TypeModes map[SensitiveType]SanitizationMode `json:"type_modes"`
	
	// 遮蔽字符
	MaskChar string `json:"mask_char"`
	
	// 遮蔽比例（0-1，保留多少比例的原始字符）
	MaskRatio float64 `json:"mask_ratio"`
	
	// 替换模板
	ReplaceTemplate string `json:"replace_template"`
	
	// 是否保留上下文
	PreserveContext bool `json:"preserve_context"`
	
	// 上下文窗口大小
	ContextWindowSize int `json:"context_window_size"`
	
	// 最小置信度阈值
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	
	// 是否启用加密
	EnableEncryption bool `json:"enable_encryption"`
	
	// 加密密钥路径
	EncryptionKeyPath string `json:"encryption_key_path"`
	
	// 是否生成报告
	GenerateReport bool `json:"generate_report"`
	
	// 自定义模式
	CustomPatterns map[string]*regexp.Regexp `json:"-"`
	
	// 敏感词列表
	SensitiveWords []string `json:"sensitive_words"`
	
	// 排除模式（不处理的模式）
	ExcludePatterns []string `json:"exclude_patterns"`
}

// DefaultSanitizationConfig 默认配置
func DefaultSanitizationConfig() *SanitizationConfig {
	return &SanitizationConfig{
		DefaultMode:         ModeMask,
		TypeModes:           make(map[SensitiveType]SanitizationMode),
		MaskChar:           "*",
		MaskRatio:          0.3,
		ReplaceTemplate:    "[REDACTED_%s]",
		PreserveContext:    true,
		ContextWindowSize:  50,
		ConfidenceThreshold: 0.7,
		EnableEncryption:   false,
		GenerateReport:     true,
		CustomPatterns:     make(map[string]*regexp.Regexp),
		SensitiveWords:     []string{},
		ExcludePatterns:    []string{},
	}
}

// AISanitizer AI去敏感化器
type AISanitizer struct {
	config     *SanitizationConfig
	patterns   map[SensitiveType]*regexp.Regexp
	encryptKey []byte
	tokenStore *TokenStore
	mu         sync.RWMutex
}

// NewAISanitizer 创建AI去敏感化器
func NewAISanitizer(config *SanitizationConfig) (*AISanitizer, error) {
	if config == nil {
		config = DefaultSanitizationConfig()
	}

	s := &AISanitizer{
		config:     config,
		patterns:   make(map[SensitiveType]*regexp.Regexp),
		tokenStore: NewTokenStore(),
	}

	// 初始化内置模式
	s.initPatterns()

	// 加载加密密钥
	if config.EnableEncryption && config.EncryptionKeyPath != "" {
		if err := s.loadEncryptionKey(config.EncryptionKeyPath); err != nil {
			return nil, fmt.Errorf("failed to load encryption key: %w", err)
		}
	}

	return s, nil
}

// initPatterns 初始化内置模式
func (s *AISanitizer) initPatterns() {
	// 信用卡号
	s.patterns[SensitiveTypeCreditCard] = regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`)
	
	// 社会安全号（中国身份证）
	s.patterns[SensitiveTypeSSN] = regexp.MustCompile(`\b\d{6}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)
	
	// 护照号码
	s.patterns[SensitiveTypePassport] = regexp.MustCompile(`\b[EGP][A-Z]?\d{8}\b`)
	
	// 手机号
	s.patterns[SensitiveTypePhone] = regexp.MustCompile(`\b(?:\+?86)?1[3-9]\d{9}\b`)
	
	// 邮箱
	s.patterns[SensitiveTypeEmail] = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	
	// IP地址
	s.patterns[SensitiveTypeIP] = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	
	// 密码模式（常见密码字段）
	s.patterns[SensitiveTypePassword] = regexp.MustCompile(`(?i)(?:password|passwd|pwd|密码)\s*[=:：]\s*\S+`)
	
	// API Key模式
	s.patterns[SensitiveTypeAPIKey] = regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|secret[_-]?key|token)\s*[=:：]\s*[A-Za-z0-9_-]{16,}`)
	
	// 银行账号
	s.patterns[SensitiveTypeBankAccount] = regexp.MustCompile(`\b\d{16,19}\b`)
	
	// 身份证号
	s.patterns[SensitiveTypeIDCard] = regexp.MustCompile(`\b\d{17}[\dXx]\b`)
}

// loadEncryptionKey 加载加密密钥
func (s *AISanitizer) loadEncryptionKey(keyPath string) error {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		// 如果密钥文件不存在，生成新密钥
		if os.IsNotExist(err) {
			key := make([]byte, 32) // AES-256
			if _, err := rand.Read(key); err != nil {
				return err
			}
			s.encryptKey = key
			
			// 保存密钥
			if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
				return err
			}
			return os.WriteFile(keyPath, key, 0600)
		}
		return err
	}
	
	s.encryptKey = data
	return nil
}

// Sanitize 执行去敏感化
func (s *AISanitizer) Sanitize(ctx context.Context, text string) (*SanitizationResult, error) {
	startTime := time.Now()
	
	result := &SanitizationResult{
		OriginalText:  text,
		SanitizedText: text,
		DetectedItems: []SensitiveData{},
		Stats: SanitizationStats{
			ByType: make(map[SensitiveType]int),
		},
		Timestamp: time.Now(),
	}

	// 检测敏感信息
	detected := s.detect(text)
	result.DetectedItems = detected
	result.Stats.TotalDetected = len(detected)
	result.Stats.TotalScanned = len(text)

	// 统计各类型数量
	for _, item := range detected {
		result.Stats.ByType[item.Type]++
	}

	// 根据模式处理
	sanitizedText := text
	offset := 0
	
	for _, item := range detected {
		// 获取该类型的处理模式
		mode := s.getMode(item.Type)
		
		// 计算实际位置（考虑之前替换的偏移）
		actualStart := item.StartPos + offset
		actualEnd := item.EndPos + offset
		
		var replacement string
		var err error
		
		replacement, err = s.processItem(item, mode)
		if err != nil {
			return nil, fmt.Errorf("failed to process item: %w", err)
		}
		
		// 替换文本
		before := sanitizedText[:actualStart]
		after := sanitizedText[actualEnd:]
		sanitizedText = before + replacement + after
		
		// 计算偏移量变化
		offset += len(replacement) - (item.EndPos - item.StartPos)
		
		// 更新统计
		switch mode {
		case ModeRedact:
			result.Stats.TotalRedacted++
		case ModeEncrypt:
			result.Stats.TotalEncrypted++
		default:
			result.Stats.TotalReplaced++
		}
	}

	result.SanitizedText = sanitizedText
	result.ProcessingTimeMS = time.Since(startTime).Milliseconds()

	return result, nil
}

// detect 检测敏感信息
func (s *AISanitizer) detect(text string) []SensitiveData {
	var results []SensitiveData
	
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 使用内置模式检测
	for sensitiveType, pattern := range s.patterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			value := text[start:end]
			
			// 检查是否在排除列表
			if s.isExcluded(value) {
				continue
			}
			
			// 获取上下文
			context := s.getContext(text, start, end)
			
			results = append(results, SensitiveData{
				Type:       sensitiveType,
				Value:      value,
				StartPos:   start,
				EndPos:     end,
				Confidence: s.calculateConfidence(sensitiveType, value),
				Context:    context,
			})
		}
	}

	// 使用自定义模式检测
	for _, pattern := range s.config.CustomPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			value := text[start:end]
			
			if s.isExcluded(value) {
				continue
			}
			
			context := s.getContext(text, start, end)
			
			results = append(results, SensitiveData{
				Type:       SensitiveTypeCustom,
				Value:      value,
				StartPos:   start,
				EndPos:     end,
				Confidence: 0.9, // 自定义模式默认高置信度
				Context:    context,
			})
		}
	}

	// 检测敏感词
	for _, word := range s.config.SensitiveWords {
		if strings.Contains(text, word) {
			start := strings.Index(text, word)
			end := start + len(word)
			results = append(results, SensitiveData{
				Type:       SensitiveTypeCustom,
				Value:      word,
				StartPos:   start,
				EndPos:     end,
				Confidence: 1.0,
			})
		}
	}

	// 按置信度过滤
	filtered := results[:0]
	for _, item := range results {
		if item.Confidence >= s.config.ConfidenceThreshold {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// getMode 获取处理模式
func (s *AISanitizer) getMode(t SensitiveType) SanitizationMode {
	if mode, ok := s.config.TypeModes[t]; ok {
		return mode
	}
	return s.config.DefaultMode
}

// processItem 处理单个敏感项
func (s *AISanitizer) processItem(item SensitiveData, mode SanitizationMode) (string, error) {
	switch mode {
	case ModeRedact:
		return s.redact(item)
	case ModeMask:
		return s.mask(item)
	case ModeReplace:
		return s.replace(item)
	case ModeEncrypt:
		return s.encrypt(item)
	case ModeHash:
		return s.hash(item)
	case ModeTokenize:
		return s.tokenize(item)
	default:
		return s.mask(item)
	}
}

// redact 完全移除
func (s *AISanitizer) redact(item SensitiveData) (string, error) {
	return "", nil
}

// mask 部分遮蔽
func (s *AISanitizer) mask(item SensitiveData) (string, error) {
	value := item.Value
	length := len(value)
	
	if length <= 4 {
		return strings.Repeat(s.config.MaskChar, length), nil
	}
	
	// 根据遮蔽比例计算保留的字符数
	keepChars := int(float64(length) * s.config.MaskRatio)
	if keepChars < 2 {
		keepChars = 2
	}
	
	// 保留首尾，中间遮蔽
	start := value[:keepChars/2]
	end := value[length-keepChars/2:]
	middle := strings.Repeat(s.config.MaskChar, length-keepChars)
	
	return start + middle + end, nil
}

// replace 替换为占位符
func (s *AISanitizer) replace(item SensitiveData) (string, error) {
	template := s.config.ReplaceTemplate
	if template == "" {
		template = "[REDACTED_%s]"
	}
	return fmt.Sprintf(template, strings.ToUpper(string(item.Type))), nil
}

// encrypt 加密
func (s *AISanitizer) encrypt(item SensitiveData) (string, error) {
	if s.encryptKey == nil {
		return "", ErrKeyNotFound
	}
	
	block, err := aes.NewCipher(s.encryptKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	
	encrypted := gcm.Seal(nonce, nonce, []byte(item.Value), nil)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// hash 哈希替换
func (s *AISanitizer) hash(item SensitiveData) (string, error) {
	h := sha256.New()
	h.Write([]byte(item.Value))
	return fmt.Sprintf("HASH_%x", h.Sum(nil)[:8]), nil
}

// tokenize 令牌化
func (s *AISanitizer) tokenize(item SensitiveData) (string, error) {
	token := s.tokenStore.CreateToken(item.Value, item.Type)
	return token, nil
}

// Detokenize 反令牌化
func (s *AISanitizer) Detokenize(token string) (string, error) {
	value, ok := s.tokenStore.GetValue(token)
	if !ok {
		return "", ErrKeyNotFound
	}
	return value, nil
}

// Decrypt 解密
func (s *AISanitizer) Decrypt(encryptedBase64 string) (string, error) {
	if s.encryptKey == nil {
		return "", ErrKeyNotFound
	}
	
	encrypted, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("%w: invalid base64", ErrDecryptionFailed)
	}
	
	block, err := aes.NewCipher(s.encryptKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	
	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return "", fmt.Errorf("%w: ciphertext too short", ErrDecryptionFailed)
	}
	
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	
	return string(plaintext), nil
}

// getContext 获取上下文
func (s *AISanitizer) getContext(text string, start, end int) string {
	if !s.config.PreserveContext {
		return ""
	}
	
	windowSize := s.config.ContextWindowSize / 2
	
	contextStart := start - windowSize
	if contextStart < 0 {
		contextStart = 0
	}
	
	contextEnd := end + windowSize
	if contextEnd > len(text) {
		contextEnd = len(text)
	}
	
	return text[contextStart:contextEnd]
}

// calculateConfidence 计算置信度
func (s *AISanitizer) calculateConfidence(t SensitiveType, value string) float64 {
	// 基于类型和值计算置信度
	switch t {
	case SensitiveTypeCreditCard:
		// Luhn 校验
		if s.luhnCheck(value) {
			return 0.95
		}
		return 0.7
	case SensitiveTypeSSN, SensitiveTypeIDCard:
		// 校验位验证
		if s.idCardCheck(value) {
			return 0.95
		}
		return 0.6
	case SensitiveTypePhone:
		// 手机号格式验证
		if len(strings.ReplaceAll(strings.ReplaceAll(value, "+", ""), " ", "")) == 11 {
			return 0.9
		}
		return 0.7
	case SensitiveTypeEmail:
		// 邮箱格式相对稳定
		return 0.9
	case SensitiveTypeIP:
		// IP地址格式验证
		parts := strings.Split(value, ".")
		valid := true
		for _, part := range parts {
			var num int
			_, _ = fmt.Sscanf(part, "%d", &num)
			if num < 0 || num > 255 {
				valid = false
				break
			}
		}
		if valid {
			return 0.85
		}
		return 0.6
	default:
		return 0.8
	}
}

// luhnCheck Luhn 算法校验信用卡号
func (s *AISanitizer) luhnCheck(number string) bool {
	// 移除非数字字符
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)
	
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	
	sum := 0
	alt := false
	
	for i := len(digits) - 1; i >= 0; i-- {
		digit := int(digits[i] - '0')
		
		if alt {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		
		sum += digit
		alt = !alt
	}
	
	return sum%10 == 0
}

// idCardCheck 身份证校验
func (s *AISanitizer) idCardCheck(id string) bool {
	if len(id) != 18 {
		return false
	}
	
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkCodes := "10X98765432"

	sum := 0
	for i := 0; i < 17; i++ {
		var digit int
		_, _ = fmt.Sscanf(string(id[i]), "%d", &digit)
		sum += digit * weights[i]
	}
	
	expectedCheck := checkCodes[sum%11]
	return strings.ToUpper(string(id[17])) == string(expectedCheck)
}

// isExcluded 检查是否在排除列表
func (s *AISanitizer) isExcluded(value string) bool {
	for _, pattern := range s.config.ExcludePatterns {
		if matched, _ := regexp.MatchString(pattern, value); matched {
			return true
		}
	}
	return false
}

// AddCustomPattern 添加自定义模式
func (s *AISanitizer) AddCustomPattern(name string, pattern *regexp.Regexp) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.CustomPatterns[name] = pattern
}

// RemoveCustomPattern 移除自定义模式
func (s *AISanitizer) RemoveCustomPattern(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.config.CustomPatterns, name)
}

// AddSensitiveWord 添加敏感词
func (s *AISanitizer) AddSensitiveWord(word string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.SensitiveWords = append(s.config.SensitiveWords, word)
}

// SetTypeMode 设置特定类型的处理模式
func (s *AISanitizer) SetTypeMode(t SensitiveType, mode SanitizationMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.TypeModes[t] = mode
}

// GenerateReport 生成报告
func (s *AISanitizer) GenerateReport(result *SanitizationResult) string {
	var sb strings.Builder
	
	sb.WriteString("=== 数据去敏感化报告 ===\n\n")
	sb.WriteString(fmt.Sprintf("处理时间: %s\n", result.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("处理耗时: %d ms\n\n", result.ProcessingTimeMS))
	
	sb.WriteString("=== 统计信息 ===\n")
	sb.WriteString(fmt.Sprintf("扫描字符数: %d\n", result.Stats.TotalScanned))
	sb.WriteString(fmt.Sprintf("检测到敏感信息: %d\n", result.Stats.TotalDetected))
	sb.WriteString(fmt.Sprintf("已处理: %d\n", result.Stats.TotalReplaced+result.Stats.TotalRedacted+result.Stats.TotalEncrypted))
	sb.WriteString(fmt.Sprintf("  - 移除: %d\n", result.Stats.TotalRedacted))
	sb.WriteString(fmt.Sprintf("  - 替换: %d\n", result.Stats.TotalReplaced))
	sb.WriteString(fmt.Sprintf("  - 加密: %d\n", result.Stats.TotalEncrypted))
	
	sb.WriteString("\n=== 按类型统计 ===\n")
	for t, count := range result.Stats.ByType {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", t, count))
	}
	
	sb.WriteString("\n=== 检测详情 ===\n")
	for i, item := range result.DetectedItems {
		sb.WriteString(fmt.Sprintf("%d. 类型: %s, 置信度: %.2f%%\n", i+1, item.Type, item.Confidence*100))
		if item.Context != "" {
			sb.WriteString(fmt.Sprintf("   上下文: %s\n", item.Context))
		}
	}
	
	return sb.String()
}

// ========== TokenStore 令牌存储 ==========

// TokenStore 令牌存储
type TokenStore struct {
	tokens map[string]*TokenInfo
	mu     sync.RWMutex
}

// TokenInfo 令牌信息
type TokenInfo struct {
	Token     string        `json:"token"`
	Value     string        `json:"value"`
	Type      SensitiveType `json:"type"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
}

// NewTokenStore 创建令牌存储
func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokens: make(map[string]*TokenInfo),
	}
}

// CreateToken 创建令牌
func (ts *TokenStore) CreateToken(value string, t SensitiveType) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	// 生成唯一令牌
	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := fmt.Sprintf("TKN_%s", base64.URLEncoding.EncodeToString(tokenBytes))
	
	ts.tokens[token] = &TokenInfo{
		Token:     token,
		Value:     value,
		Type:      t,
		CreatedAt: time.Now(),
	}
	
	return token
}

// GetValue 获取值
func (ts *TokenStore) GetValue(token string) (string, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	info, ok := ts.tokens[token]
	if !ok {
		return "", false
	}
	
	// 检查过期
	if info.ExpiresAt != nil && info.ExpiresAt.Before(time.Now()) {
		return "", false
	}
	
	return info.Value, true
}

// SetExpiry 设置过期时间
func (ts *TokenStore) SetExpiry(token string, expiry time.Time) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	info, ok := ts.tokens[token]
	if !ok {
		return false
	}
	
	info.ExpiresAt = &expiry
	return true
}

// DeleteToken 删除令牌
func (ts *TokenStore) DeleteToken(token string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.tokens, token)
}

// SaveToFile 保存到文件
func (ts *TokenStore) SaveToFile(path string) error {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	data, err := json.MarshalIndent(ts.tokens, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0600)
}

// LoadFromFile 从文件加载
func (ts *TokenStore) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, &ts.tokens)
}

// ========== 批量处理 ==========

// BatchSanitize 批量去敏感化
func (s *AISanitizer) BatchSanitize(ctx context.Context, texts []string) ([]*SanitizationResult, error) {
	results := make([]*SanitizationResult, len(texts))
	
	for i, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			result, err := s.Sanitize(ctx, text)
			if err != nil {
				return nil, fmt.Errorf("failed to sanitize text %d: %w", i, err)
			}
			results[i] = result
		}
	}
	
	return results, nil
}

// SanitizeFile 文件去敏感化
func (s *AISanitizer) SanitizeFile(ctx context.Context, inputPath, outputPath string) (*SanitizationResult, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	result, err := s.Sanitize(ctx, string(data))
	if err != nil {
		return nil, err
	}
	
	if err := os.WriteFile(outputPath, []byte(result.SanitizedText), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	
	// 不保留原始文本在结果中
	result.OriginalText = ""
	
	return result, nil
}

// ========== 辅助函数 ==========

// ValidateConfig 验证配置
func ValidateConfig(config *SanitizationConfig) error {
	if config.MaskRatio < 0 || config.MaskRatio > 1 {
		return fmt.Errorf("mask_ratio must be between 0 and 1, got %f", config.MaskRatio)
	}
	
	if config.ConfidenceThreshold < 0 || config.ConfidenceThreshold > 1 {
		return fmt.Errorf("confidence_threshold must be between 0 and 1, got %f", config.ConfidenceThreshold)
	}
	
	return nil
}