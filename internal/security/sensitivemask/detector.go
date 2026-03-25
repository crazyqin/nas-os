package sensitivemask

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Detector detects sensitive information in text.
type Detector struct {
	config    DetectorConfig
	patterns  map[SensitiveType]*regexp.Regexp
	validator map[SensitiveType]func(string) (bool, float64)
	mu        sync.RWMutex
}

// NewDetector creates a new sensitive information detector.
func NewDetector(config DetectorConfig) *Detector {
	d := &Detector{
		config:    config,
		patterns:  make(map[SensitiveType]*regexp.Regexp),
		validator: make(map[SensitiveType]func(string) (bool, float64)),
	}

	d.initPatterns()
	d.initValidators()

	// Add custom patterns
	for _, cp := range config.CustomPatterns {
		if re, err := regexp.Compile(cp.Pattern); err == nil {
			d.patterns[cp.Type] = re
		}
	}

	return d
}

// initPatterns initializes regex patterns for different sensitive types.
func (d *Detector) initPatterns() {
	// 中国手机号 - 支持 +86、0086 前缀
	// 注意：0086前缀会匹配到部分，需要在validator中处理
	d.patterns[TypePhoneNumber] = regexp.MustCompile(`(?i)(?:\+?86)?1[3-9]\d{9}|001?861[3-9]\d{9}`)

	// 中国身份证号 - 18位，支持X结尾
	d.patterns[TypeIDCard] = regexp.MustCompile(`[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`)

	// 银行卡号 - 16-19位数字
	// 注意：不使用\b边界，因为中文标点不是单词边界
	d.patterns[TypeBankCard] = regexp.MustCompile(`\d{16,19}`)

	// 邮箱
	d.patterns[TypeEmail] = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	// 护照号码
	d.patterns[TypePassport] = regexp.MustCompile(`\b(?:[EG]\d{8})|(?:[PS]\d{7})|(?:[DG]\d{8})|(?:[A-Z]{1,2}\d{6,9})\b`)

	// 信用卡号 - 必须带分隔符的格式（纯数字16位由银行卡模式处理）
	d.patterns[TypeCreditCard] = regexp.MustCompile(`\d{4}[-\s]\d{4}[-\s]\d{4}[-\s]\d{4}`)

	// API Key - 常见格式
	d.patterns[TypeAPIKey] = regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|secret|token|bearer|auth)[_-]?[\w\-]{16,}|(?:sk-[a-zA-Z0-9]{20,})|(?:xox[baprs]-[a-zA-Z0-9\-]{10,})`)

	// IPv4 地址
	d.patterns[TypeIPv4] = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`)

	// 密码相关模式
	d.patterns[TypePassword] = regexp.MustCompile(`(?i)(?:password|passwd|pwd|pass)\s*[=:：]\s*\S+`)
}

// initValidators initializes validators with checksum verification.
func (d *Detector) initValidators() {
	// 手机号验证器
	d.validator[TypePhoneNumber] = func(s string) (bool, float64) {
		// 去除前缀
		s = strings.TrimPrefix(s, "+86")
		s = strings.TrimPrefix(s, "0086")
		if len(s) != 11 {
			return false, 0
		}
		// 中国手机号第二位必须是 3-9
		if s[0] != '1' || s[1] < '3' || s[1] > '9' {
			return false, 0.3
		}
		// 检查是否全数字
		for _, c := range s {
			if !unicode.IsDigit(c) {
				return false, 0
			}
		}
		return true, 0.95
	}

	// 身份证验证器（含校验和）
	d.validator[TypeIDCard] = func(s string) (bool, float64) {
		s = strings.ToUpper(s)
		if len(s) != 18 {
			return false, 0
		}
		// 前17位必须为数字
		for i := 0; i < 17; i++ {
			if !unicode.IsDigit(rune(s[i])) {
				return false, 0
			}
		}
		// 第18位可以是数字或X
		if !unicode.IsDigit(rune(s[17])) && s[17] != 'X' {
			return false, 0
		}
		// 校验省份码
		provinceCode := s[0:2]
		validProvinces := map[string]bool{
			"11": true, "12": true, "13": true, "14": true, "15": true,
			"21": true, "22": true, "23": true, "31": true, "32": true,
			"33": true, "34": true, "35": true, "36": true, "37": true,
			"41": true, "42": true, "43": true, "44": true, "45": true,
			"46": true, "50": true, "51": true, "52": true, "53": true,
			"54": true, "61": true, "62": true, "63": true, "64": true,
			"65": true, "71": true, "81": true, "82": true,
		}
		if !validProvinces[provinceCode] {
			return false, 0.5
		}
		// 校验和计算
		if d.config.CheckChecksum {
			weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
			checkCodes := "10X98765432"
			sum := 0
			for i := 0; i < 17; i++ {
				sum += int(s[i]-'0') * weights[i]
			}
			if checkCodes[sum%11] != s[17] {
				return false, 0.7
			}
		}
		return true, 0.98
	}

	// 银行卡验证器（Luhn算法）
	d.validator[TypeBankCard] = func(s string) (bool, float64) {
		// 移除空格和横线
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "-", "")
		if len(s) < 16 || len(s) > 19 {
			return false, 0
		}
		// 全数字检查
		for _, c := range s {
			if !unicode.IsDigit(c) {
				return false, 0
			}
		}
		// Luhn算法验证
		if d.config.CheckChecksum && !luhnCheck(s) {
			return false, 0.5
		}
		return true, 0.9
	}

	// 邮箱验证器
	d.validator[TypeEmail] = func(s string) (bool, float64) {
		if !strings.Contains(s, "@") || !strings.Contains(s, ".") {
			return false, 0
		}
		parts := strings.Split(s, "@")
		if len(parts) != 2 {
			return false, 0
		}
		if len(parts[0]) == 0 || len(parts[1]) < 3 {
			return false, 0
		}
		// 检查域名部分
		if !strings.Contains(parts[1], ".") {
			return false, 0
		}
		return true, 0.95
	}

	// IPv4验证器
	d.validator[TypeIPv4] = func(s string) (bool, float64) {
		parts := strings.Split(s, ".")
		if len(parts) != 4 {
			return false, 0
		}
		for _, p := range parts {
			var num int
			for _, c := range p {
				if !unicode.IsDigit(c) {
					return false, 0
				}
				num = num*10 + int(c-'0')
			}
			if num > 255 {
				return false, 0
			}
		}
		return true, 0.95
	}
}

// luhnCheck implements the Luhn algorithm for bank card validation.
func luhnCheck(number string) bool {
	digits := make([]int, 0, len(number))
	for _, c := range number {
		if c < '0' || c > '9' {
			return false
		}
		digits = append(digits, int(c-'0'))
	}

	if len(digits) == 0 {
		return false
	}

	sum := 0
	parity := len(digits) % 2
	for i, d := range digits {
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// Detect detects all sensitive information in the given text.
func (d *Detector) Detect(ctx context.Context, text string) (*DetectionResult, error) {
	start := time.Now()
	result := &DetectionResult{
		Matches: make([]SensitiveMatch, 0),
	}

	// Priority order: longer/more specific patterns first
	priorityOrder := []SensitiveType{
		TypeIDCard,      // 18 chars, very specific pattern
		TypePassport,    // Specific format
		TypeCreditCard,  // Specific format with separators
		TypeEmail,       // Has @ symbol
		TypeAPIKey,      // Specific prefixes
		TypePassword,    // Specific pattern
		TypePhoneNumber, // 11 chars
		TypeBankCard,    // 16-19 digits
		TypeIPv4,        // IP addresses
	}

	// Track positions that have been matched
	matchedPositions := make(map[int]SensitiveType)

	for _, t := range priorityOrder {
		if !d.config.EnabledTypes[t] {
			continue
		}

		pattern, ok := d.patterns[t]
		if !ok {
			continue
		}

		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}

			startPos, endPos := match[0], match[1]

			// Check if this position overlaps with an already matched position
			overlaps := false
			for pos := startPos; pos < endPos; pos++ {
				if _, exists := matchedPositions[pos]; exists {
					overlaps = true
					break
				}
			}
			if overlaps {
				continue
			}

			value := text[startPos:endPos]

			// Validate and get confidence
			confidence := 1.0
			if validator, ok := d.validator[t]; ok && d.config.CheckChecksum {
				valid, conf := validator(value)
				if !valid && d.config.StrictMode {
					continue
				}
				confidence = conf
			}

			// Skip low confidence matches
			if confidence < d.config.MinConfidence {
				continue
			}

			// Mark positions as matched
			for pos := startPos; pos < endPos; pos++ {
				matchedPositions[pos] = t
			}

			// Get context
			contextStart := max(0, startPos-d.config.ContextLength)
			contextEnd := min(len(text), endPos+d.config.ContextLength)
			context := text[contextStart:contextEnd]

			// Determine risk level
			riskLevel := d.getRiskLevel(t, confidence)

			result.Matches = append(result.Matches, SensitiveMatch{
				Type:       t,
				Value:      value,
				StartPos:   startPos,
				EndPos:     endPos,
				RiskLevel:  riskLevel,
				Confidence: confidence,
				Context:    context,
			})
		}
	}

	// Sort matches by position
	sortMatches(result.Matches)

	result.TotalCount = len(result.Matches)
	for _, m := range result.Matches {
		if m.RiskLevel >= RiskLevelHigh {
			result.HighRiskCount++
		}
	}
	result.HasSensitive = result.TotalCount > 0
	result.ProcessingTime = time.Since(start)

	return result, nil
}

// getRiskLevel determines risk level based on type and confidence.
func (d *Detector) getRiskLevel(t SensitiveType, confidence float64) RiskLevel {
	switch t {
	case TypeIDCard, TypeCreditCard, TypeAPIKey, TypePassword:
		if confidence >= 0.9 {
			return RiskLevelCritical
		}
		return RiskLevelHigh
	case TypeBankCard, TypePassport:
		return RiskLevelHigh
	case TypePhoneNumber, TypeEmail:
		if confidence >= 0.95 {
			return RiskLevelMedium
		}
		return RiskLevelLow
	case TypeIPv4:
		return RiskLevelLow
	default:
		return RiskLevelMedium
	}
}

// DetectWithType detects only specific types of sensitive information.
func (d *Detector) DetectWithType(ctx context.Context, text string, types []SensitiveType) (*DetectionResult, error) {
	original := d.config.EnabledTypes
	d.config.EnabledTypes = make(map[SensitiveType]bool)
	for _, t := range types {
		d.config.EnabledTypes[t] = true
	}
	result, err := d.Detect(ctx, text)
	d.config.EnabledTypes = original
	return result, err
}

// UpdateConfig updates the detector configuration.
func (d *Detector) UpdateConfig(config DetectorConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = config

	// Add new custom patterns
	for _, cp := range config.CustomPatterns {
		if re, err := regexp.Compile(cp.Pattern); err == nil {
			d.patterns[cp.Type] = re
		}
	}
}

// GetConfig returns current configuration.
func (d *Detector) GetConfig() DetectorConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// sortMatches sorts matches by start position.
func sortMatches(matches []SensitiveMatch) {
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].StartPos > matches[j].StartPos {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
