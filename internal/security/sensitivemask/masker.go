package sensitivemask

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
)

// Masker handles sensitive data masking operations.
type Masker struct {
	config MaskerConfig
	mu     sync.RWMutex
}

// NewMasker creates a new masker with the given configuration.
func NewMasker(config MaskerConfig) *Masker {
	return &Masker{
		config: config,
	}
}

// Mask masks sensitive data based on the configured strategy.
func (m *Masker) Mask(ctx context.Context, text string, matches []SensitiveMatch) (string, error) {
	if len(matches) == 0 {
		return text, nil
	}

	// Build result from end to start to preserve positions
	result := text
	for i := len(matches) - 1; i >= 0; i-- {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		match := matches[i]
		maskedValue := m.maskValue(match.Value, match.Type)

		// Replace in text
		result = result[:match.StartPos] + maskedValue + result[match.EndPos:]
	}

	return result, nil
}

// maskValue masks a single value based on its type.
func (m *Masker) maskValue(value string, sType SensitiveType) string {
	strategy, ok := m.config.Strategies[sType]
	if !ok {
		strategy = MaskStrategyFull
	}

	switch strategy {
	case MaskStrategyNone:
		return value

	case MaskStrategyPartial:
		return m.maskPartial(value, sType)

	case MaskStrategyFull:
		return m.maskFull(value)

	case MaskStrategyHash:
		return m.maskHash(value)

	case MaskStrategyRemove:
		return "[REMOVED]"

	default:
		return m.maskFull(value)
	}
}

// maskPartial performs partial masking, keeping some characters visible.
func (m *Masker) maskPartial(value string, sType SensitiveType) string {
	keepLen, ok := m.config.PartialKeepLen[sType]
	if !ok {
		keepLen = 2
	}

	maskChar := m.config.DefaultMask

	switch sType {
	case TypePhoneNumber:
		// 手机号：保留前3和后4位
		if len(value) >= 11 {
			cleaned := strings.TrimPrefix(value, "+86")
			cleaned = strings.TrimPrefix(cleaned, "0086")
			if len(cleaned) >= 11 {
				return cleaned[:3] + strings.Repeat(maskChar, 4) + cleaned[7:]
			}
		}
		return value[:3] + strings.Repeat(maskChar, len(value)-7) + value[len(value)-4:]

	case TypeIDCard:
		// 身份证：显示前6位（地区码）和后4位
		if len(value) == 18 {
			return value[:6] + strings.Repeat(maskChar, 8) + value[14:]
		}
		return value[:keepLen] + strings.Repeat(maskChar, len(value)-keepLen*2) + value[len(value)-keepLen:]

	case TypeEmail:
		// 邮箱：保留用户名前3字符和域名
		parts := strings.Split(value, "@")
		if len(parts) == 2 {
			user := parts[0]
			domain := parts[1]
			if len(user) <= 3 {
				return strings.Repeat(maskChar, len(user)) + "@" + domain
			}
			return user[:3] + strings.Repeat(maskChar, len(user)-3) + "@" + domain
		}
		return value

	case TypeBankCard, TypeCreditCard:
		// 银行卡/信用卡：保留后4位
		cleaned := strings.ReplaceAll(value, " ", "")
		cleaned = strings.ReplaceAll(cleaned, "-", "")
		if len(cleaned) > 4 {
			return strings.Repeat(maskChar, len(cleaned)-4) + cleaned[len(cleaned)-4:]
		}
		return strings.Repeat(maskChar, len(cleaned))

	case TypeIPv4:
		// IP地址：保留最后一段
		parts := strings.Split(value, ".")
		if len(parts) == 4 {
			return strings.Repeat(maskChar, 3) + "." +
				strings.Repeat(maskChar, 3) + "." +
				strings.Repeat(maskChar, 3) + "." +
				parts[3]
		}
		return value

	default:
		if len(value) <= keepLen*2 {
			return strings.Repeat(maskChar, len(value))
		}
		return value[:keepLen] + strings.Repeat(maskChar, len(value)-keepLen*2) + value[len(value)-keepLen:]
	}
}

// maskFull performs full masking.
func (m *Masker) maskFull(value string) string {
	return strings.Repeat(m.config.DefaultMask, len(value))
}

// maskHash performs hash-based masking for verification purposes.
func (m *Masker) maskHash(value string) string {
	h := sha256.New()
	h.Write([]byte(value))
	h.Write([]byte(m.config.HashSalt))
	hash := hex.EncodeToString(h.Sum(nil))
	return "HASH:" + hash[:16]
}

// MaskWithType masks using a specific strategy.
func (m *Masker) MaskWithType(value string, sType SensitiveType, strategy MaskStrategy) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	originalStrategy := m.config.Strategies[sType]
	m.config.Strategies[sType] = strategy
	result := m.maskValue(value, sType)
	m.config.Strategies[sType] = originalStrategy
	return result
}

// UpdateConfig updates the masker configuration.
func (m *Masker) UpdateConfig(config MaskerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig returns current configuration.
func (m *Masker) GetConfig() MaskerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetStrategy sets the masking strategy for a specific type.
func (m *Masker) SetStrategy(sType SensitiveType, strategy MaskStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Strategies[sType] = strategy
}

// SetDefaultMask sets the default masking character.
func (m *Masker) SetDefaultMask(maskChar string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.DefaultMask = maskChar
}

// QuickMask provides a quick one-liner for masking sensitive data.
func QuickMask(text string) (string, []SensitiveMatch) {
	detector := NewDetector(DefaultDetectorConfig)
	masker := NewMasker(DefaultMaskerConfig)

	result, _ := detector.Detect(context.Background(), text)
	if !result.HasSensitive {
		return text, nil
	}

	masked, _ := masker.Mask(context.Background(), text, result.Matches)
	return masked, result.Matches
}

// QuickDetect provides a quick one-liner for detecting sensitive data.
func QuickDetect(text string) []SensitiveMatch {
	detector := NewDetector(DefaultDetectorConfig)
	result, _ := detector.Detect(context.Background(), text)
	return result.Matches
}

// HasSensitive checks if text contains any sensitive information.
func HasSensitive(text string) bool {
	detector := NewDetector(DefaultDetectorConfig)
	result, _ := detector.Detect(context.Background(), text)
	return result.HasSensitive
}
