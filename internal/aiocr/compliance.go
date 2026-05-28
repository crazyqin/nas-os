// compliance.go - 敏感信息脱敏（身份证号、银行卡号等）
package aiocr

import (
	"log"
	"regexp"
	"strings"
)

// Compliance 合规处理器.
type Compliance struct {
	config    *DesensitizeConfig
	patterns  map[string]*regexp.Regexp
}

// NewCompliance 创建合规处理器.
func NewCompliance(cfg *DesensitizeConfig) *Compliance {
	c := &Compliance{
		config:   cfg,
		patterns: make(map[string]*regexp.Regexp),
	}

	if cfg == nil {
		c.config = DefaultDesensitizeConfig()
	}

	// 初始化正则模式
	c.initPatterns()

	return c
}

// DefaultDesensitizeConfig 默认脱敏配置.
func DefaultDesensitizeConfig() *DesensitizeConfig {
	return &DesensitizeConfig{
		Enabled:         true,
		IDCardPattern:   `\d{17}[\dXx]`,
		BankCardPattern: `\d{16,19}`,
		PhonePattern:    `1[3-9]\d{9}`,
		EmailPattern:    `[\w.]+@[\w.]+\.\w+`,
		LicensePattern:  `[A-Za-z0-9]{18}`,
		MaskChar:        "*",
		KeepPrefix:      4,
		KeepSuffix:      4,
	}
}

// initPatterns 初始化正则模式.
func (c *Compliance) initPatterns() {
	patterns := map[string]string{
		"id_card":   c.config.IDCardPattern,
		"bank_card": c.config.BankCardPattern,
		"phone":     c.config.PhonePattern,
		"email":     c.config.EmailPattern,
		"license":   c.config.LicensePattern,
	}

	for name, pattern := range patterns {
		if pattern != "" {
			re, err := regexp.Compile(pattern)
			if err != nil {
				log.Printf("⚠️ 正则编译失败: %s, %v", name, err)
				continue
			}
			c.patterns[name] = re
		}
	}

	// 添加自定义模式
	for i, pattern := range c.config.CustomPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("⚠️ 自定义正则编译失败: %d, %v", i, err)
			continue
		}
		c.patterns["custom_"+string(rune('a'+i))] = re
	}

	log.Printf("✅ 已初始化 %d 个脱敏模式", len(c.patterns))
}

// DesensitizeText 文本脱敏.
func (c *Compliance) DesensitizeText(text string) string {
	if !c.config.Enabled {
		return text
	}

	result := text

	// 身份证号脱敏
	if re, ok := c.patterns["id_card"]; ok {
		result = c.maskPattern(result, re, "id_card")
	}

	// 银行卡号脱敏
	if re, ok := c.patterns["bank_card"]; ok {
		result = c.maskPattern(result, re, "bank_card")
	}

	// 手机号脱敏
	if re, ok := c.patterns["phone"]; ok {
		result = c.maskPattern(result, re, "phone")
	}

	// 邮箱脱敏
	if re, ok := c.patterns["email"]; ok {
		result = c.maskPattern(result, re, "email")
	}

	// 营业执照脱敏
	if re, ok := c.patterns["license"]; ok {
		result = c.maskPattern(result, re, "license")
	}

	// 自定义模式脱敏
	for name, re := range c.patterns {
		if strings.HasPrefix(name, "custom_") {
			result = c.maskPattern(result, re, name)
		}
	}

	return result
}

// maskPattern 脱敏匹配.
func (c *Compliance) maskPattern(text string, re *regexp.Regexp, patternType string) string {
	return re.ReplaceAllStringFunc(text, func(match string) string {
		return c.mask(match, patternType)
	})
}

// mask 脱敏处理.
func (c *Compliance) mask(value, patternType string) string {
	length := len(value)
	if length <= c.config.KeepPrefix+c.config.KeepSuffix {
		return strings.Repeat(c.config.MaskChar, length)
	}

	// 保留前缀和后缀，中间用脱敏字符替换
	prefix := value[:c.config.KeepPrefix]
	suffix := value[length-c.config.KeepSuffix:]
	maskLength := length - c.config.KeepPrefix - c.config.KeepSuffix
	mask := strings.Repeat(c.config.MaskChar, maskLength)

	return prefix + mask + suffix
}

// DesensitizeResult 脱敏识别结果.
func (c *Compliance) DesensitizeResult(result *OCRResult, fields []string) {
	if !c.config.Enabled {
		return
	}

	log.Printf("🔒 开始敏感信息脱敏: %s", result.ID)

	// 脱敏全文
	result.FullText = c.DesensitizeText(result.FullText)

	// 脱敏页面文本
	for _, page := range result.Pages {
		page.Text = c.DesensitizeText(page.Text)
		for _, block := range page.Blocks {
			block.Text = c.DesensitizeText(block.Text)
		}
	}

	// 脱敏结构化数据
	if result.Structured != nil {
		c.desensitizeStructuredData(result.Structured, fields)
	}

	// 脱敏表格
	for _, page := range result.Pages {
		for _, table := range page.Tables {
			c.desensitizeTable(table)
		}
	}

	result.Desensitized = true
	log.Printf("✅ 敏感信息脱敏完成: %s", result.ID)
}

// desensitizeStructuredData 脱敏结构化数据.
func (c *Compliance) desensitizeStructuredData(data *StructuredData, fields []string) {
	if data.Fields == nil {
		return
	}

	// 如果指定了字段，只脱敏指定字段
	if len(fields) > 0 {
		for _, field := range fields {
			if value, exists := data.Fields[field]; exists {
				if strValue, ok := value.(string); ok {
					data.Fields[field] = c.DesensitizeText(strValue)
				}
			}
		}
		return
	}

	// 否则脱敏所有敏感字段
	sensitiveFields := []string{
		"id_number", "card_number", "phone", "email",
		"tax_number", "credit_code", "bank_account",
	}

	for _, field := range sensitiveFields {
		if value, exists := data.Fields[field]; exists {
			if strValue, ok := value.(string); ok {
				data.Fields[field] = c.DesensitizeText(strValue)
			}
		}
	}
}

// desensitizeTable 脱敏表格.
func (c *Compliance) desensitizeTable(table *Table) {
	for i, row := range table.Cells {
		for j, cell := range row {
			table.Cells[i][j] = c.DesensitizeText(cell)
		}
	}
}

// CheckCompliance 检查合规性.
func (c *Compliance) CheckCompliance(text string) *ComplianceCheck {
	log.Printf("🔍 检查合规性")

	check := &ComplianceCheck{
		IsValid:     true,
		Violations:  make([]Violation, 0),
		Suggestions: make([]string, 0),
	}

	// 检查是否包含敏感信息
	for name, re := range c.patterns {
		matches := re.FindAllString(text, -1)
		if len(matches) > 0 {
			check.Violations = append(check.Violations, Violation{
				Type:     name,
				Count:    len(matches),
				Examples: c.getExamples(matches, 3),
			})
			check.IsValid = false
		}
	}

	// 生成建议
	if !check.IsValid {
		check.Suggestions = append(check.Suggestions, "建议进行敏感信息脱敏处理")
		check.Suggestions = append(check.Suggestions, "使用 DesensitizeText 方法进行脱敏")
	}

	log.Printf("✅ 合规性检查完成，有效: %v, 违规数: %d",
		check.IsValid, len(check.Violations))

	return check
}

// getExamples 获取示例（脱敏后）.
func (c *Compliance) getExamples(matches []string, limit int) []string {
	examples := make([]string, 0, limit)
	for i, match := range matches {
		if i >= limit {
			break
		}
		examples = append(examples, c.mask(match, "example"))
	}
	return examples
}

// ComplianceCheck 合规性检查结果.
type ComplianceCheck struct {
	IsValid     bool        `json:"is_valid"`    // 是否合规
	Violations  []Violation `json:"violations"`  // 违规项
	Suggestions []string    `json:"suggestions"` // 建议
}

// Violation 违规项.
type Violation struct {
	Type     string   `json:"type"`     // 类型
	Count    int      `json:"count"`    // 数量
	Examples []string `json:"examples"` // 示例
}

// AddCustomPattern 添加自定义模式.
func (c *Compliance) AddCustomPattern(name, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	c.patterns[name] = re
	log.Printf("✅ 已添加自定义脱敏模式: %s", name)
	return nil
}

// RemovePattern 移除模式.
func (c *Compliance) RemovePattern(name string) {
	delete(c.patterns, name)
	log.Printf("✅ 已移除脱敏模式: %s", name)
}

// GetPatterns 获取所有模式.
func (c *Compliance) GetPatterns() map[string]string {
	result := make(map[string]string)
	for name := range c.patterns {
		result[name] = c.patterns[name].String()
	}
	return result
}

// MaskIDCard 脱敏身份证号.
func (c *Compliance) MaskIDCard(id string) string {
	if len(id) != 18 {
		return strings.Repeat(c.config.MaskChar, len(id))
	}
	return id[:4] + strings.Repeat(c.config.MaskChar, 10) + id[14:]
}

// MaskBankCard 脱敏银行卡号.
func (c *Compliance) MaskBankCard(card string) string {
	length := len(card)
	if length < 8 {
		return strings.Repeat(c.config.MaskChar, length)
	}
	return card[:4] + strings.Repeat(c.config.MaskChar, length-8) + card[length-4:]
}

// MaskPhone 脱敏手机号.
func (c *Compliance) MaskPhone(phone string) string {
	if len(phone) != 11 {
		return strings.Repeat(c.config.MaskChar, len(phone))
	}
	return phone[:3] + "****" + phone[7:]
}

// MaskEmail 脱敏邮箱.
func (c *Compliance) MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return strings.Repeat(c.config.MaskChar, len(email))
	}

	username := parts[0]
	domain := parts[1]

	if len(username) <= 2 {
		return strings.Repeat(c.config.MaskChar, len(username)) + "@" + domain
	}

	return username[:2] + strings.Repeat(c.config.MaskChar, len(username)-2) + "@" + domain
}
