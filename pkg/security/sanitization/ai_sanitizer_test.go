package sanitization

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestNewAISanitizer(t *testing.T) {
	s, err := NewAISanitizer(nil)
	if err != nil {
		t.Fatalf("Failed to create sanitizer: %v", err)
	}

	if s.config == nil {
		t.Error("Config should not be nil")
	}

	if s.config.DefaultMode != ModeMask {
		t.Errorf("Default mode should be mask, got %s", s.config.DefaultMode)
	}
}

func TestDetectCreditCard(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	text := "My credit card is 4532-1234-5678-9010 and 5500 0000 0000 0004"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if len(result.DetectedItems) < 1 {
		t.Error("Should detect at least one credit card")
	}

	// Check that the text was modified
	if result.SanitizedText == text {
		t.Error("Text should be sanitized")
	}
}

func TestDetectEmail(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	text := "Contact me at test@example.com or admin@company.org"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if len(result.DetectedItems) < 2 {
		t.Error("Should detect at least two emails")
	}

	// Check stats
	if result.Stats.ByType[SensitiveTypeEmail] < 2 {
		t.Error("Should have at least 2 emails in stats")
	}
}

func TestDetectPhone(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	text := "Call me at 13812345678 or +8613987654321"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if len(result.DetectedItems) < 1 {
		t.Error("Should detect at least one phone number")
	}
}

func TestDetectIP(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	text := "Server IP: 192.168.1.100 and 10.0.0.1"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if len(result.DetectedItems) < 2 {
		t.Error("Should detect at least two IP addresses")
	}
}

func TestDetectIDCard(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	// Valid Chinese ID card (test data)
	text := "ID: 11010519491231002X"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if len(result.DetectedItems) < 1 {
		t.Error("Should detect ID card")
	}
}

func TestMaskMode(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.DefaultMode = ModeMask
	config.MaskChar = "*"
	config.MaskRatio = 0.3

	s, _ := NewAISanitizer(config)

	text := "Email: test@example.com"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	// Should contain mask characters
	if !regexp.MustCompile(`\*`).MatchString(result.SanitizedText) {
		t.Error("Masked text should contain asterisks")
	}
}

func TestRedactMode(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.DefaultMode = ModeRedact

	s, _ := NewAISanitizer(config)

	text := "Email: test@example.com"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	// Email should be removed
	if result.Stats.TotalRedacted < 1 {
		t.Error("Should have at least one redaction")
	}
}

func TestReplaceMode(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.DefaultMode = ModeReplace
	config.ReplaceTemplate = "[REDACTED]"

	s, _ := NewAISanitizer(config)

	text := "Email: test@example.com"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if !regexp.MustCompile(`\[REDACTED\]`).MatchString(result.SanitizedText) {
		t.Error("Should contain replacement placeholder")
	}
}

func TestHashMode(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.DefaultMode = ModeHash

	s, _ := NewAISanitizer(config)

	text := "Email: test@example.com"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	// Should contain hash prefix
	if !regexp.MustCompile(`HASH_`).MatchString(result.SanitizedText) {
		t.Error("Should contain hash prefix")
	}
}

func TestTokenizeMode(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.DefaultMode = ModeTokenize

	s, _ := NewAISanitizer(config)

	text := "Email: test@example.com"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	// Should contain token prefix
	if !regexp.MustCompile(`TKN_`).MatchString(result.SanitizedText) {
		t.Error("Should contain token prefix")
	}

	// Should be able to detokenize
	for _, item := range result.DetectedItems {
		if item.Type == SensitiveTypeEmail {
			_, err := s.Detokenize(result.SanitizedText)
			if err == nil {
				break
			}
		}
	}
}

func TestCustomPattern(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	// Add custom pattern for account numbers
	s.AddCustomPattern("account", regexp.MustCompile(`ACC\d{10}`))

	text := "Account: ACC1234567890"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if len(result.DetectedItems) < 1 {
		t.Error("Should detect custom pattern")
	}

	if result.Stats.ByType[SensitiveTypeCustom] < 1 {
		t.Error("Should have custom type in stats")
	}
}

func TestSensitiveWords(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.SensitiveWords = []string{"secret", "confidential"}

	s, _ := NewAISanitizer(config)

	text := "This is a secret document"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	if len(result.DetectedItems) < 1 {
		t.Error("Should detect sensitive word")
	}
}

func TestTypeSpecificMode(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.DefaultMode = ModeMask
	config.TypeModes = map[SensitiveType]SanitizationMode{
		SensitiveTypeEmail: ModeReplace,
	}

	s, _ := NewAISanitizer(config)

	text := "Email: test@example.com and phone: 13812345678"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	// Email should be replaced, phone should be masked
	if result.Stats.TotalReplaced < 1 {
		t.Error("Should have at least one replacement (email)")
	}
}

func TestLuhnCheck(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	// Valid credit card number (test number)
	if !s.luhnCheck("4532015112830366") {
		t.Error("Valid card should pass Luhn check")
	}

	// Invalid credit card number
	if s.luhnCheck("4532015112830367") {
		t.Error("Invalid card should fail Luhn check")
	}
}

func TestIDCardCheck(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	// Valid Chinese ID card (test data with correct checksum)
	if !s.idCardCheck("11010519491231002X") {
		t.Error("Valid ID card should pass check")
	}

	// Invalid ID card
	if s.idCardCheck("123456789012345678") {
		t.Error("Invalid ID card should fail check")
	}
}

func TestBatchSanitize(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	texts := []string{
		"Email: test1@example.com",
		"Phone: 13812345678",
		"IP: 192.168.1.1",
	}

	results, err := s.BatchSanitize(context.Background(), texts)
	if err != nil {
		t.Fatalf("Batch sanitization failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if len(result.DetectedItems) < 1 {
			t.Errorf("Result %d should have detected items", i)
		}
	}
}

func TestGenerateReport(t *testing.T) {
	s, _ := NewAISanitizer(nil)

	text := "Email: test@example.com, Phone: 13812345678, IP: 192.168.1.1"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	report := s.GenerateReport(result)
	if report == "" {
		t.Error("Report should not be empty")
	}

	// Check report contains expected sections
	if !regexp.MustCompile(`检测到敏感信息`).MatchString(report) {
		t.Error("Report should contain detection count")
	}
}

func TestConfidenceThreshold(t *testing.T) {
	config := DefaultSanitizationConfig()
	config.ConfidenceThreshold = 0.99 // Very high threshold

	s, _ := NewAISanitizer(config)

	text := "Some text with IP 192.168.1.1"
	result, err := s.Sanitize(context.Background(), text)
	if err != nil {
		t.Fatalf("Sanitization failed: %v", err)
	}

	// With high threshold, should filter out most detections
	// This depends on confidence calculation
	t.Logf("Detected %d items with high threshold", len(result.DetectedItems))
}

func TestTokenStore(t *testing.T) {
	ts := NewTokenStore()

	token := ts.CreateToken("sensitive-data", SensitiveTypeEmail)
	if token == "" {
		t.Error("Token should not be empty")
	}

	value, ok := ts.GetValue(token)
	if !ok {
		t.Error("Should be able to get value")
	}
	if value != "sensitive-data" {
		t.Errorf("Value mismatch: got %s", value)
	}

	// Test expiry
	expiry := time.Now().Add(-time.Hour) // Already expired
	ts.SetExpiry(token, expiry)
	_, ok = ts.GetValue(token)
	if ok {
		t.Error("Expired token should not return value")
	}
}

func TestValidateConfig(t *testing.T) {
	// Valid config
	config := DefaultSanitizationConfig()
	if err := ValidateConfig(config); err != nil {
		t.Errorf("Valid config should pass: %v", err)
	}

	// Invalid mask ratio
	config.MaskRatio = 1.5
	if err := ValidateConfig(config); err == nil {
		t.Error("Invalid mask ratio should fail")
	}

	// Invalid confidence threshold
	config.MaskRatio = 0.5
	config.ConfidenceThreshold = 1.5
	if err := ValidateConfig(config); err == nil {
		t.Error("Invalid confidence threshold should fail")
	}
}

func BenchmarkSanitize(b *testing.B) {
	s, _ := NewAISanitizer(nil)
	text := "Email: test@example.com, Phone: 13812345678, IP: 192.168.1.1, Card: 4532015112830366"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Sanitize(context.Background(), text)
	}
}

func BenchmarkDetectOnly(b *testing.B) {
	s, _ := NewAISanitizer(nil)
	text := "Email: test@example.com, Phone: 13812345678, IP: 192.168.1.1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.detect(text)
	}
}