package auth

import (
	"errors"
	"fmt"
	"regexp"
	"unicode"
)

// PasswordPolicy 密码策略配置.
type PasswordPolicy struct {
	MinLength        int  `json:"min_length"`        // 最小长度
	MaxLength        int  `json:"max_length"`        // 最大长度
	RequireUppercase bool `json:"require_uppercase"` // 需要大写字母
	RequireLowercase bool `json:"require_lowercase"` // 需要小写字母
	RequireDigit     bool `json:"require_digit"`     // 需要数字
	RequireSpecial   bool `json:"require_special"`   // 需要特殊字符
	MinSpecialCount  int  `json:"min_special_count"` // 最少特殊字符数
	PreventCommon    bool `json:"prevent_common"`    // 阻止常见弱密码
	PreventUserInfo  bool `json:"prevent_user_info"` // 阻止包含用户信息的密码
	HistoryCount     int  `json:"history_count"`     // 密码历史记录数量
	MaxAge           int  `json:"max_age"`           // 密码最大有效期（天），0 表示不限制
}

// DefaultPasswordPolicy 默认密码策略.
var DefaultPasswordPolicy = PasswordPolicy{
	MinLength:        8,
	MaxLength:        128,
	RequireUppercase: true,
	RequireLowercase: true,
	RequireDigit:     true,
	RequireSpecial:   true,
	MinSpecialCount:  1,
	PreventCommon:    true,
	PreventUserInfo:  true,
	HistoryCount:     5,
	MaxAge:           90, // 90 天
}

// PasswordValidationResult 密码验证结果.
type PasswordValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Score    int      `json:"score"` // 0-100 密码强度评分
}

// PasswordValidator 密码验证器.
type PasswordValidator struct {
	policy          PasswordPolicy
	commonPasswords map[string]bool
}

// NewPasswordValidator 创建密码验证器.
func NewPasswordValidator(policy PasswordPolicy) *PasswordValidator {
	v := &PasswordValidator{
		policy:          policy,
		commonPasswords: make(map[string]bool),
	}

	// 加载常见弱密码列表
	v.loadCommonPasswords()

	return v
}

// loadCommonPasswords 加载常见弱密码.
func (v *PasswordValidator) loadCommonPasswords() {
	// 常见弱密码列表（部分）
	common := []string{
		"password", "password1", "password123", "123456", "12345678",
		"qwerty", "abc123", "monkey", "master", "dragon",
		"letmein", "login", "admin", "welcome", "hello",
		"sunshine", "princess", "football", "baseball", "soccer",
		"iloveyou", "trustno1", "shadow", "ashley", "michael",
		"password!", "Password1", "Password1!", "P@ssw0rd", "Passw0rd!",
		"qwer1234", "asdf1234", "zxcv1234", "1qaz2wsx",
		"密码", "密码123", "管理员", "admin123", "root123",
	}

	for _, p := range common {
		v.commonPasswords[p] = true
		v.commonPasswords[p] = true // 小写版本
	}
}

// Validate 验证密码.
func (v *PasswordValidator) Validate(password string, userInfo ...string) PasswordValidationResult {
	result := PasswordValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Score:    0,
	}

	// 长度检查
	if len(password) < v.policy.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("密码长度至少需要 %d 个字符", v.policy.MinLength))
	}

	if v.policy.MaxLength > 0 && len(password) > v.policy.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("密码长度不能超过 %d 个字符", v.policy.MaxLength))
	}

	// 字符类型检查
	var hasUpper, hasLower, hasDigit bool
	var specialCount int

	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			specialCount++
		}
	}

	if v.policy.RequireUppercase && !hasUpper {
		result.Valid = false
		result.Errors = append(result.Errors, "密码需要包含至少一个大写字母")
	}

	if v.policy.RequireLowercase && !hasLower {
		result.Valid = false
		result.Errors = append(result.Errors, "密码需要包含至少一个小写字母")
	}

	if v.policy.RequireDigit && !hasDigit {
		result.Valid = false
		result.Errors = append(result.Errors, "密码需要包含至少一个数字")
	}

	if v.policy.RequireSpecial && specialCount < v.policy.MinSpecialCount {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("密码需要包含至少 %d 个特殊字符", v.policy.MinSpecialCount))
	}

	// 常见弱密码检查
	if v.policy.PreventCommon {
		if v.commonPasswords[password] || v.commonPasswords[lowercase(password)] {
			result.Valid = false
			result.Errors = append(result.Errors, "密码过于常见，请使用更复杂的密码")
		}
	}

	// 用户信息检查
	if v.policy.PreventUserInfo && len(userInfo) > 0 {
		for _, info := range userInfo {
			if info != "" && len(info) >= 3 {
				if containsIgnoreCase(password, info) {
					result.Valid = false
					result.Errors = append(result.Errors, "密码不能包含用户信息")
					break
				}
			}
		}
	}

	// 计算密码强度评分
	result.Score = v.calculateScore(password, hasUpper, hasLower, hasDigit, specialCount)

	// 添加警告
	if result.Score < 50 {
		result.Warnings = append(result.Warnings, "密码强度较弱，建议使用更复杂的密码")
	}

	return result
}

// ValidateWithConfirm 验证密码并确认.
func (v *PasswordValidator) ValidateWithConfirm(password, confirmPassword string, userInfo ...string) PasswordValidationResult {
	result := v.Validate(password, userInfo...)

	if password != confirmPassword {
		result.Valid = false
		result.Errors = append(result.Errors, "两次输入的密码不一致")
	}

	return result
}

// calculateScore 计算密码强度评分.
func (v *PasswordValidator) calculateScore(password string, hasUpper, hasLower, hasDigit bool, specialCount int) int {
	score := 0

	// 基础长度分
	length := len(password)
	switch {
	case length >= 16:
		score += 25
	case length >= 12:
		score += 20
	case length >= 8:
		score += 15
	case length >= 6:
		score += 10
	}

	// 字符多样性分
	if hasUpper {
		score += 10
	}
	if hasLower {
		score += 10
	}
	if hasDigit {
		score += 10
	}
	if specialCount > 0 {
		score += 15
	}
	if specialCount >= 2 {
		score += 10
	}

	// 额外长度奖励
	if length >= 20 {
		score += 10
	}

	// 检查是否有连续字符或重复字符
	if hasSequentialChars(password) {
		score -= 10
	}
	if hasRepeatingChars(password) {
		score -= 10
	}

	// 确保分数在 0-100 范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// GetStrengthLevel 获取密码强度等级.
func GetStrengthLevel(score int) string {
	switch {
	case score >= 80:
		return "强"
	case score >= 60:
		return "中"
	case score >= 40:
		return "弱"
	default:
		return "很弱"
	}
}

// 辅助函数

// lowercase 将字符串转换为小写.
func lowercase(s string) string {
	result := make([]rune, len(s))
	for i, c := range s {
		result[i] = unicode.ToLower(c)
	}
	return string(result)
}

// containsIgnoreCase 不区分大小写检查子字符串.
func containsIgnoreCase(s, substr string) bool {
	return regexp.MustCompile("(?i)" + regexp.QuoteMeta(substr)).MatchString(s)
}

// hasSequentialChars 检查是否有连续字符（如 abc, 123, cba, 321）.
func hasSequentialChars(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i]+1 == s[i+1] && s[i+1]+1 == s[i+2] {
			return true
		}
		if s[i]-1 == s[i+1] && s[i+1]-1 == s[i+2] {
			return true
		}
	}
	return false
}

// hasRepeatingChars 检查是否有重复字符（如 aaa, 111）.
func hasRepeatingChars(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] == s[i+1] && s[i+1] == s[i+2] {
			return true
		}
	}
	return false
}

// PasswordHistory 密码历史记录.
type PasswordHistory struct {
	Hashes   []string `json:"hashes"`
	MaxCount int      `json:"max_count"`
}

// NewPasswordHistory 创建密码历史记录.
func NewPasswordHistory(maxCount int) *PasswordHistory {
	if maxCount <= 0 {
		maxCount = 5
	}
	return &PasswordHistory{
		Hashes:   make([]string, 0),
		MaxCount: maxCount,
	}
}

// Add 添加密码到历史记录.
func (h *PasswordHistory) Add(hash string) {
	h.Hashes = append(h.Hashes, hash)
	if len(h.Hashes) > h.MaxCount {
		h.Hashes = h.Hashes[1:]
	}
}

// Contains 检查密码是否在历史记录中.
func (h *PasswordHistory) Contains(hash string) bool {
	for _, existingHash := range h.Hashes {
		if existingHash == hash {
			return true
		}
	}
	return false
}

// 错误定义.
var (
	ErrPasswordTooShort       = errors.New("密码长度不足")
	ErrPasswordTooLong        = errors.New("密码过长")
	ErrPasswordMissingUpper   = errors.New("密码需要包含大写字母")
	ErrPasswordMissingLower   = errors.New("密码需要包含小写字母")
	ErrPasswordMissingDigit   = errors.New("密码需要包含数字")
	ErrPasswordMissingSpecial = errors.New("密码需要包含特殊字符")
	ErrPasswordTooCommon      = errors.New("密码过于常见")
	ErrPasswordContainsInfo   = errors.New("密码包含用户信息")
	ErrPasswordInHistory      = errors.New("不能使用最近使用过的密码")
)
