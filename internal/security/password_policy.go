// Package security provides password policy enforcement.
package security

import (
	"errors"
	"regexp"
	"sync"
)

// PasswordPolicy defines password security requirements.
type PasswordPolicy struct {
	Enabled             bool   `json:"enabled"`
	MinLength           int    `json:"min_length"`
	MaxLength           int    `json:"max_length"`
	RequireLowercase    bool   `json:"require_lowercase"`
	RequireUppercase    bool   `json:"require_uppercase"`
	RequireDigit        bool   `json:"require_digit"`
	RequireSpecialChar  bool   `json:"require_special_char"`
	MinSpecialChars     int    `json:"min_special_chars"`
	PreventCommonPasswords bool `json:"prevent_common_passwords"`
	PreventUserInfo     bool   `json:"prevent_user_info"`
	MaxAge              int    `json:"max_age"`           // days
	HistoryCount        int    `json:"history_count"`     // prevent reuse
	MaxAttempts         int    `json:"max_attempts"`      // before lockout
	LockoutDuration     int    `json:"lockout_duration"`  // minutes
}

// PasswordValidator validates passwords against policy.
type PasswordValidator struct {
	policy           *PasswordPolicy
	commonPasswords  map[string]bool
	historyStore     *PasswordHistoryStore
	attemptsTracker  *LoginAttemptsTracker
	mu               sync.RWMutex
}

// NewPasswordValidator creates a new password validator.
func NewPasswordValidator(policy *PasswordPolicy) *PasswordValidator {
	if policy == nil {
		policy = DefaultPasswordPolicy()
	}
	return &PasswordValidator{
		policy:          policy,
		commonPasswords: loadCommonPasswords(),
		historyStore:    NewPasswordHistoryStore(policy.HistoryCount),
		attemptsTracker: NewLoginAttemptsTracker(policy.MaxAttempts, policy.LockoutDuration),
	}
}

// Validate validates a password against the policy.
func (pv *PasswordValidator) Validate(password string, userInfo map[string]string) error {
	if !pv.policy.Enabled {
		return nil
	}

	// Check length
	if len(password) < pv.policy.MinLength {
		return errors.New("密码长度不足")
	}
	if pv.policy.MaxLength > 0 && len(password) > pv.policy.MaxLength {
		return errors.New("密码长度过长")
	}

	// Check lowercase
	if pv.policy.RequireLowercase {
		if !regexp.MustCompile(`[a-z]`).MatchString(password) {
			return errors.New("密码必须包含小写字母")
		}
	}

	// Check uppercase
	if pv.policy.RequireUppercase {
		if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
			return errors.New("密码必须包含大写字母")
		}
	}

	// Check digit
	if pv.policy.RequireDigit {
		if !regexp.MustCompile(`[0-9]`).MatchString(password) {
			return errors.New("密码必须包含数字")
		}
	}

	// Check special character
	if pv.policy.RequireSpecialChar {
		specialCount := countSpecialChars(password)
		if specialCount < pv.policy.MinSpecialChars {
			return errors.New("密码必须包含特殊字符")
		}
	}

	// Check common passwords
	if pv.policy.PreventCommonPasswords {
		if pv.commonPasswords[password] {
			return errors.New("密码过于常见，请更换")
		}
	}

	// Check user info leakage
	if pv.policy.PreventUserInfo && userInfo != nil {
		for _, info := range userInfo {
			if len(info) >= 3 && contains(password, info) {
				return errors.New("密码不能包含用户信息")
			}
		}
	}

	return nil
}

// ValidateForUser validates password for specific user (includes history check).
func (pv *PasswordValidator) ValidateForUser(password string, userID string, userInfo map[string]string) error {
	if err := pv.Validate(password, userInfo); err != nil {
		return err
	}

	// Check history
	if pv.policy.HistoryCount > 0 {
		if pv.historyStore.IsInHistory(userID, password) {
			return errors.New("不能使用最近使用过的密码")
		}
	}

	return nil
}

// RecordPassword records a password in user's history.
func (pv *PasswordValidator) RecordPassword(userID string, password string) error {
	if pv.policy.HistoryCount > 0 {
		pv.historyStore.Add(userID, password)
	}
	return nil
}

// CheckLoginAttempt checks if login is allowed (attempts tracking).
func (pv *PasswordValidator) CheckLoginAttempt(userID string) error {
	if pv.policy.MaxAttempts <= 0 {
		return nil
	}

	attempts, locked := pv.attemptsTracker.GetStatus(userID)
	if locked {
		return errors.New("账户已被锁定，请稍后再试")
	}

	if attempts >= pv.policy.MaxAttempts {
		pv.attemptsTracker.Lock(userID)
		return errors.New("登录失败次数过多，账户已被锁定")
	}

	return nil
}

// RecordLoginSuccess records successful login.
func (pv *PasswordValidator) RecordLoginSuccess(userID string) {
	pv.attemptsTracker.Reset(userID)
}

// RecordLoginFailure records failed login attempt.
func (pv *PasswordValidator) RecordLoginFailure(userID string) {
	pv.attemptsTracker.AddAttempt(userID)
}

// GetPasswordAge returns password age in days.
func (pv *PasswordValidator) GetPasswordAge(userID string) int {
	return pv.historyStore.GetAge(userID)
}

// IsPasswordExpired checks if password is expired.
func (pv *PasswordValidator) IsPasswordExpired(userID string) bool {
	if pv.policy.MaxAge <= 0 {
		return false
	}
	age := pv.GetPasswordAge(userID)
	return age >= pv.policy.MaxAge
}

// GetPolicy returns current password policy.
func (pv *PasswordValidator) GetPolicy() *PasswordPolicy {
	return pv.policy
}

// SetPolicy sets password policy.
func (pv *PasswordValidator) SetPolicy(policy *PasswordPolicy) {
	pv.mu.Lock()
	defer pv.mu.Unlock()
	pv.policy = policy
}

// DefaultPasswordPolicy returns default password policy.
func DefaultPasswordPolicy() *PasswordPolicy {
	return &PasswordPolicy{
		Enabled:              true,
		MinLength:            8,
		MaxLength:            128,
		RequireLowercase:     true,
		RequireUppercase:     true,
		RequireDigit:         true,
		RequireSpecialChar:   true,
		MinSpecialChars:      1,
		PreventCommonPasswords: true,
		PreventUserInfo:      true,
		MaxAge:              90,    // 90 days
		HistoryCount:        5,     // remember 5 passwords
		MaxAttempts:         5,     // 5 attempts
		LockoutDuration:     15,    // 15 minutes
	}
}

// WeakPasswordPolicy returns a weaker policy for easier testing.
func WeakPasswordPolicy() *PasswordPolicy {
	return &PasswordPolicy{
		Enabled:        true,
		MinLength:      6,
		MaxLength:      128,
		MaxAge:         0,  // no expiration
		HistoryCount:   0,  // no history
		MaxAttempts:    10,
		LockoutDuration: 5,
	}
}

// countSpecialChars counts special characters in password.
func countSpecialChars(s string) int {
	count := 0
	for _, c := range s {
		if isSpecialChar(c) {
			count++
		}
	}
	return count
}

func isSpecialChar(c rune) bool {
	special := `!@#$%^&*()_+-=[]{}|;:',.<>?/"~`
	for _, sc := range special {
		if c == sc {
			return true
		}
	}
	return false
}

func contains(password, info string) bool {
	return len(info) >= 3 && (password == info ||
		regexp.MustCompile(regexp.QuoteMeta(info)).MatchString(password))
}

// loadCommonPasswords loads common passwords to prevent.
func loadCommonPasswords() map[string]bool {
	common := []string{
		"password", "123456", "12345678", "qwerty", "abc123",
		"monkey", "master", "dragon", "111111", "baseball",
		"iloveyou", "trustno1", "sunshine", "princess", "admin",
		"welcome", "shadow", "superman", "michael", "password1",
		"123123", "football", "whatever", "letmein", "login",
	}
	result := make(map[string]bool)
	for _, p := range common {
		result[p] = true
	}
	return result
}