// Package ai provides data desensitization and PII protection
// Supports multiple redaction strategies with reversible masking
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DesensitizationStrategy represents different strategies for data protection
type DesensitizationStrategy string

const (
	// StrategyMask replaces sensitive data with asterisks
	StrategyMask      DesensitizationStrategy = "mask"       // Replace with ****
	// StrategyHash applies one-way hash to sensitive data
	StrategyHash      DesensitizationStrategy = "hash"       // One-way hash
	StrategyTokenize  DesensitizationStrategy = "tokenize"   // Reversible tokenization
	StrategyRedact    DesensitizationStrategy = "redact"     // Complete removal
	StrategyPartial   DesensitizationStrategy = "partial"    // Show partial info
	StrategyEncrypt   DesensitizationStrategy = "encrypt"    // AES encryption
)

// PIIType represents types of personally identifiable information
type PIIType string

const (
	// PIIName represents personally identifiable name
	PIIName          PIIType = "name"
	// PIIEmail represents email address
	PIIEmail         PIIType = "email"
	PIIPhone         PIIType = "phone"
	PIIIDCard        PIIType = "id_card"
	PIIPassport      PIIType = "passport"
	PIICreditCard    PIIType = "credit_card"
	PIIBankAccount   PIIType = "bank_account"
	PIIAddress       PIIType = "address"
	PIIIPAddress     PIIType = "ip_address"
	PIIMACAddress    PIIType = "mac_address"
	PIILicensePlate  PIIType = "license_plate"
	PIISocialMedia   PIIType = "social_media"
	PIIDateOfBirth   PIIType = "date_of_birth"
	PIIMedicalRecord PIIType = "medical_record"
	PIICustom        PIIType = "custom"
)

// DesensitizationRule defines a rule for data protection
type DesensitizationRule struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Type         PIIType                 `json:"type"`
	Pattern      string                  `json:"pattern"`
	Strategy     DesensitizationStrategy `json:"strategy"`
	Replacement  string                  `json:"replacement,omitempty"`
	MaskChar     string                  `json:"maskChar,omitempty"`
	ShowFirst    int                     `json:"showFirst,omitempty"`
	ShowLast     int                     `json:"showLast,omitempty"`
	Enabled      bool                    `json:"enabled"`
	Priority     int                     `json:"priority"` // Higher priority rules processed first
	Description  string                  `json:"description,omitempty"`
}

// DesensitizationResult represents the result of desensitization
type DesensitizationResult struct {
	Original     string                  `json:"-"` // Never store original
	Processed    string                  `json:"processed"`
	RedactionCount int                   `json:"redactionCount"`
	Redactions   []RedactionDetail       `json:"redactions"`
	Strategy     DesensitizationStrategy `json:"strategy"`
	Timestamp    time.Time               `json:"timestamp"`
}

// RedactionDetail represents details of a single redaction
type RedactionDetail struct {
	Type       PIIType                 `json:"type"`
	Start      int                     `json:"start"`
	End        int                     `json:"end"`
	Original   string                  `json:"-"` // Never expose original
	Replaced   string                  `json:"replaced"`
	Strategy   DesensitizationStrategy `json:"strategy"`
	Token      string                  `json:"token,omitempty"` // For tokenization
}

// Desensitizer provides comprehensive data desensitization
type Desensitizer struct {
	rules       []DesensitizationRule
	tokenStore  *TokenStore
	mu          sync.RWMutex
}

// TokenStore stores tokens for reversible desensitization
type TokenStore struct {
	tokens map[string]TokenEntry
	mu     sync.RWMutex
}

// TokenEntry represents a stored token
type TokenEntry struct {
	Token       string    `json:"token"`
	Original    string    `json:"-"` // Never store in JSON
	ValueHash   string    `json:"valueHash"`
	Type        PIIType   `json:"type"`
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
	SessionID   string    `json:"sessionId,omitempty"`
}

// NewDesensitizer creates a new desensitizer
func NewDesensitizer() *Desensitizer {
	d := &Desensitizer{
		tokenStore: NewTokenStore(),
	}

	// Load default rules
	d.rules = DefaultDesensitizationRules()
	return d
}

// NewDesensitizerWithRules creates a desensitizer with custom rules
func NewDesensitizerWithRules(rules []DesensitizationRule) *Desensitizer {
	d := &Desensitizer{
		rules:      rules,
		tokenStore: NewTokenStore(),
	}
	return d
}

// DefaultDesensitizationRules returns default PII protection rules
// Ordered by priority (higher first), more specific patterns first
func DefaultDesensitizationRules() []DesensitizationRule {
	return []DesensitizationRule{
		// ID Card - 18 digits, highest priority
		{
			ID:          "id_card_cn",
			Name:        "中国身份证号",
			Type:        PIIIDCard,
			Pattern:     `\b[1-9]\d{5}(?:18|19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`,
			Strategy:    StrategyPartial,
			ShowFirst:   1,
			ShowLast:    1,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    100,
			Description: "中国大陆身份证号码（18位）",
		},
		// Credit Card - 16 digits
		{
			ID:          "credit_card",
			Name:        "信用卡号",
			Type:        PIICreditCard,
			Pattern:     `\b(?:\d{4}[-\s]?){3}\d{4}\b`,
			Strategy:    StrategyPartial,
			ShowFirst:   4,
			ShowLast:    4,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    95,
			Description: "信用卡号码",
		},
		// Bank Account
		{
			ID:          "bank_account",
			Name:        "银行账号",
			Type:        PIIBankAccount,
			Pattern:     `\b\d{16,19}\b`,
			Strategy:    StrategyPartial,
			ShowFirst:   4,
			ShowLast:    4,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    90,
			Description: "银行账户号码",
		},
		// Phone - 11 digits
		{
			ID:          "phone_cn",
			Name:        "手机号码",
			Type:        PIIPhone,
			Pattern:     `\b1[3-9]\d{9}\b`,
			Strategy:    StrategyPartial,
			ShowFirst:   3,
			ShowLast:    4,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    85,
			Description: "中国大陆手机号码",
		},
		// Email
		{
			ID:          "email",
			Name:        "电子邮箱",
			Type:        PIIEmail,
			Pattern:     `\b[\w.-]+@[\w.-]+\.\w{2,}\b`,
			Strategy:    StrategyPartial,
			ShowFirst:   2,
			ShowLast:    0,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    80,
			Description: "电子邮件地址",
		},
		// IP Address
		{
			ID:          "ip_address",
			Name:        "IP地址",
			Type:        PIIIPAddress,
			Pattern:     `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
			Strategy:    StrategyPartial,
			ShowFirst:   2,
			ShowLast:    0,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    75,
			Description: "IPv4地址",
		},
		// MAC Address
		{
			ID:          "mac_address",
			Name:        "MAC地址",
			Type:        PIIMACAddress,
			Pattern:     `\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b`,
			Strategy:    StrategyMask,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    70,
			Description: "MAC地址",
		},
		// License Plate
		{
			ID:          "license_plate",
			Name:        "车牌号",
			Type:        PIILicensePlate,
			Pattern:     `[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼使领][A-Z][A-HJ-NP-Z0-9]{4,5}[A-HJ-NP-Z0-9挂学警港澳]`,
			Strategy:    StrategyPartial,
			ShowFirst:   1,
			ShowLast:    1,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    65,
			Description: "中国车牌号码",
		},
		// Passport
		{
			ID:          "passport",
			Name:        "护照号码",
			Type:        PIIPassport,
			Pattern:     `\b[EG]\d{8}\b`,
			Strategy:    StrategyPartial,
			ShowFirst:   1,
			ShowLast:    2,
			MaskChar:    "*",
			Enabled:     true,
			Priority:    60,
			Description: "护照号码",
		},
	}
}

// Process processes text and desensitizes PII
func (d *Desensitizer) Process(text string) *DesensitizationResult {
	return d.ProcessWithStrategy(text, "")
}

// ProcessWithStrategy processes text with a specific strategy override
func (d *Desensitizer) ProcessWithStrategy(text string, strategyOverride DesensitizationStrategy) *DesensitizationResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := &DesensitizationResult{
		Processed:  text,
		Redactions: make([]RedactionDetail, 0),
		Timestamp:  time.Now(),
	}

	// Sort rules by priority (already sorted in default rules)
	// Process each rule
	for _, rule := range d.rules {
		if !rule.Enabled {
			continue
		}

		strategy := rule.Strategy
		if strategyOverride != "" {
			strategy = strategyOverride
		}

		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllStringIndex(result.Processed, -1)
		// Process in reverse order to maintain correct indices
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			start, end := match[0], match[1]
			original := result.Processed[start:end]

			replacement := d.applyStrategy(original, rule, strategy)

			// Record redaction
			result.Redactions = append(result.Redactions, RedactionDetail{
				Type:     rule.Type,
				Start:    start,
				End:      end,
				Replaced: replacement,
				Strategy: strategy,
			})

			// Apply replacement
			result.Processed = result.Processed[:start] + replacement + result.Processed[end:]
			result.RedactionCount++
		}
	}

	return result
}

// ProcessWithContext processes text with session context for reversible operations
func (d *Desensitizer) ProcessWithContext(ctx context.Context, text, sessionID string) *DesensitizationResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := &DesensitizationResult{
		Processed:  text,
		Redactions: make([]RedactionDetail, 0),
		Timestamp:  time.Now(),
	}

	for _, rule := range d.rules {
		if !rule.Enabled {
			continue
		}

		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllStringIndex(result.Processed, -1)
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			start, end := match[0], match[1]
			original := result.Processed[start:end]

			// Use tokenization for reversible masking
			strategy := StrategyTokenize
			token := d.generateToken(original, rule.Type, sessionID)

			// Store token
			d.tokenStore.Store(token, original, rule.Type, sessionID)

			result.Redactions = append(result.Redactions, RedactionDetail{
				Type:     rule.Type,
				Start:    start,
				End:      end,
				Replaced: token,
				Strategy: strategy,
				Token:    token,
			})

			result.Processed = result.Processed[:start] + token + result.Processed[end:]
			result.RedactionCount++
		}
	}

	return result
}

// Restore restores desensitized text using stored tokens
func (d *Desensitizer) Restore(text, sessionID string) string {
	return d.tokenStore.Restore(text, sessionID)
}

// applyStrategy applies the desensitization strategy to a value
func (d *Desensitizer) applyStrategy(value string, rule DesensitizationRule, strategy DesensitizationStrategy) string {
	switch strategy {
	case StrategyMask:
		maskLen := len([]rune(value))
		if maskLen <= 0 {
			return value
		}
		maskChar := rule.MaskChar
		if maskChar == "" {
			maskChar = "*"
		}
		return strings.Repeat(maskChar, maskLen)

	case StrategyPartial:
		return d.partialMask(value, rule)

	case StrategyHash:
		return d.hashValue(value)

	case StrategyRedact:
		return "[REDACTED]"

	case StrategyTokenize:
		return d.generateToken(value, rule.Type, "")

	case StrategyEncrypt:
		// For encryption, we'd need a key management system
		return d.hashValue(value) // Fallback to hash

	default:
		return d.partialMask(value, rule)
	}
}

// partialMask applies partial masking
func (d *Desensitizer) partialMask(value string, rule DesensitizationRule) string {
	runes := []rune(value)
	length := len(runes)

	if length == 0 {
		return value
	}

	showFirst := rule.ShowFirst
	showLast := rule.ShowLast
	maskChar := rule.MaskChar
	if maskChar == "" {
		maskChar = "*"
	}

	// Adjust if value is too short
	if showFirst+showLast >= length {
		showFirst = length / 3
		showLast = length / 3
	}

	var result strings.Builder

	// Show first characters
	for i := 0; i < showFirst && i < length; i++ {
		result.WriteRune(runes[i])
	}

	// Mask middle
	maskLen := length - showFirst - showLast
	if maskLen > 0 {
		result.WriteString(strings.Repeat(maskChar, maskLen))
	}

	// Show last characters
	for i := length - showLast; i < length && i >= 0; i++ {
		if i >= showFirst {
			result.WriteRune(runes[i])
		}
	}

	return result.String()
}

// hashValue creates a one-way hash of the value
func (d *Desensitizer) hashValue(value string) string {
	h := sha256.Sum256([]byte(value))
	return "[HASH:" + base64.StdEncoding.EncodeToString(h[:8]) + "]"
}

// generateToken generates a reversible token
func (d *Desensitizer) generateToken(value string, piiType PIIType, sessionID string) string {
	h := sha256.Sum256([]byte(value + string(piiType) + sessionID + time.Now().String()))
	tokenID := base64.URLEncoding.EncodeToString(h[:6])
	return fmt.Sprintf("[%s_TOKEN_%s]", strings.ToUpper(string(piiType)), tokenID)
}

// AddRule adds a custom desensitization rule
func (d *Desensitizer) AddRule(rule DesensitizationRule) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Insert rule at correct position based on priority
	inserted := false
	for i, r := range d.rules {
		if rule.Priority > r.Priority {
			d.rules = append(d.rules[:i], append([]DesensitizationRule{rule}, d.rules[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		d.rules = append(d.rules, rule)
	}
}

// RemoveRule removes a rule by ID
func (d *Desensitizer) RemoveRule(ruleID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, rule := range d.rules {
		if rule.ID == ruleID {
			d.rules = append(d.rules[:i], d.rules[i+1:]...)
			break
		}
	}
}

// GetRules returns all rules
func (d *Desensitizer) GetRules() []DesensitizationRule {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]DesensitizationRule, len(d.rules))
	copy(result, d.rules)
	return result
}

// SetRuleEnabled enables or disables a rule
func (d *Desensitizer) SetRuleEnabled(ruleID string, enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i := range d.rules {
		if d.rules[i].ID == ruleID {
			d.rules[i].Enabled = enabled
			break
		}
	}
}

// ClearSessionTokens clears all tokens for a session
func (d *Desensitizer) ClearSessionTokens(sessionID string) {
	d.tokenStore.ClearSession(sessionID)
}

// NewTokenStore creates a new token store
func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokens: make(map[string]TokenEntry),
	}
}

// Store stores a token
func (t *TokenStore) Store(token, original string, piiType PIIType, sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.tokens[token] = TokenEntry{
		Token:     token,
		Original:  original,
		ValueHash: hashValue(original),
		Type:      piiType,
		CreatedAt: time.Now(),
		SessionID: sessionID,
	}
}

// Get retrieves the original value for a token
func (t *TokenStore) Get(token string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, exists := t.tokens[token]
	if !exists {
		return "", false
	}
	return entry.Original, true
}

// Restore restores all tokens in text
func (t *TokenStore) Restore(text, sessionID string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := text
	for token, entry := range t.tokens {
		// Only restore tokens from the same session if sessionID is provided
		if sessionID != "" && entry.SessionID != sessionID {
			continue
		}
		result = strings.ReplaceAll(result, token, entry.Original)
	}
	return result
}

// ClearSession clears all tokens for a session
func (t *TokenStore) ClearSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for token, entry := range t.tokens {
		if entry.SessionID == sessionID {
			delete(t.tokens, token)
		}
	}
}

// Clear clears all tokens
func (t *TokenStore) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.tokens = make(map[string]TokenEntry)
}

// GetTokenCount returns the number of stored tokens
func (t *TokenStore) GetTokenCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.tokens)
}

// ExportConfig exports desensitization configuration
func (d *Desensitizer) ExportConfig() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	config := struct {
		Rules []DesensitizationRule `json:"rules"`
	}{
		Rules: d.rules,
	}

	return json.MarshalIndent(config, "", "  ")
}

// ImportConfig imports desensitization configuration
func (d *Desensitizer) ImportConfig(data []byte) error {
	var config struct {
		Rules []DesensitizationRule `json:"rules"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.rules = config.Rules
	return nil
}

// DesensitizationAPI provides API interface for external use
type DesensitizationAPI struct {
	desensitizer *Desensitizer
}

// NewDesensitizationAPI creates a new API wrapper
func NewDesensitizationAPI() *DesensitizationAPI {
	return &DesensitizationAPI{
		desensitizer: NewDesensitizer(),
	}
}

// DesensitizeRequest represents an API request
type DesensitizeRequest struct {
	Text       string                  `json:"text"`
	Strategy   DesensitizationStrategy `json:"strategy,omitempty"`
	SessionID  string                  `json:"sessionId,omitempty"`
	RuleIDs    []string                `json:"ruleIds,omitempty"` // Apply only specific rules
}

// DesensitizeResponse represents an API response
type DesensitizeResponse struct {
	Success       bool                   `json:"success"`
	Processed     string                 `json:"processed"`
	RedactionCount int                   `json:"redactionCount"`
	Redactions    []RedactionDetail      `json:"redactions,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// Desensitize handles API desensitization request
func (api *DesensitizationAPI) Desensitize(req *DesensitizeRequest) *DesensitizeResponse {
	var result *DesensitizationResult

	if req.SessionID != "" {
		result = api.desensitizer.ProcessWithContext(context.Background(), req.Text, req.SessionID)
	} else {
		result = api.desensitizer.ProcessWithStrategy(req.Text, req.Strategy)
	}

	return &DesensitizeResponse{
		Success:        true,
		Processed:      result.Processed,
		RedactionCount: result.RedactionCount,
		Redactions:     result.Redactions,
	}
}

// RestoreRequest represents a restore request
type RestoreRequest struct {
	Text      string `json:"text"`
	SessionID string `json:"sessionId"`
}

// RestoreResponse represents a restore response
type RestoreResponse struct {
	Success   bool   `json:"success"`
	Restored  string `json:"restored"`
	Error     string `json:"error,omitempty"`
}

// Restore handles API restore request
func (api *DesensitizationAPI) Restore(req *RestoreRequest) *RestoreResponse {
	restored := api.desensitizer.Restore(req.Text, req.SessionID)

	return &RestoreResponse{
		Success:  true,
		Restored: restored,
	}
}

// GetRules returns all configured rules
func (api *DesensitizationAPI) GetRules() []DesensitizationRule {
	return api.desensitizer.GetRules()
}

// AddRule adds a custom rule
func (api *DesensitizationAPI) AddRule(rule DesensitizationRule) {
	api.desensitizer.AddRule(rule)
}

func hashValue(value string) string {
	h := sha256.Sum256([]byte(value))
	return base64.URLEncoding.EncodeToString(h[:])
}