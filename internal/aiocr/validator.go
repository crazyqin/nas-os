// validator.go - 识别结果校验和纠错
package aiocr

import (
	"log"
	"regexp"
	"strings"
)

// Validator 结果校验器.
type Validator struct {
	config      *Config
	rules       []*ValidationRule
	corrections map[string]string
}

// ValidationRule 校验规则.
type ValidationRule struct {
	Name      string `json:"name"`      // 规则名称
	Field     string `json:"field"`     // 字段名
	Pattern   string `json:"pattern"`   // 校验模式
	Message   string `json:"message"`   // 错误消息
	Required  bool   `json:"required"`  // 是否必需
	Corrector string `json:"corrector"` // 纠错规则
}

// ValidationResult 校验结果.
type ValidationResult struct {
	IsValid  bool     `json:"is_valid"` // 是否有效
	Errors   []string `json:"errors"`   // 错误列表
	Warnings []string `json:"warnings"` // 警告列表
	Fixed    []string `json:"fixed"`    // 已修复项
}

// NewValidator 创建校验器.
func NewValidator(cfg *Config) *Validator {
	v := &Validator{
		config:      cfg,
		rules:       make([]*ValidationRule, 0),
		corrections: make(map[string]string),
	}

	// 初始化校验规则
	v.initRules()

	// 初始化纠错规则
	v.initCorrections()

	return v
}

// initRules 初始化校验规则.
func (v *Validator) initRules() {
	// 身份证号校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "身份证号格式",
		Field:    "id_number",
		Pattern:  `^\d{17}[\dXx]$`,
		Message:  "身份证号格式不正确",
		Required: true,
	})

	// 手机号校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "手机号格式",
		Field:    "phone",
		Pattern:  `^1[3-9]\d{9}$`,
		Message:  "手机号格式不正确",
		Required: false,
	})

	// 邮箱校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "邮箱格式",
		Field:    "email",
		Pattern:  `^[\w.]+@[\w.]+\.\w+$`,
		Message:  "邮箱格式不正确",
		Required: false,
	})

	// 日期格式校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "日期格式",
		Field:    "date",
		Pattern:  `^\d{4}[-年/]\d{1,2}[-月/]\d{1,2}日?$`,
		Message:  "日期格式不正确",
		Required: false,
	})

	// 金额格式校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "金额格式",
		Field:    "amount",
		Pattern:  `^¥?\d+(\.\d{1,2})?$`,
		Message:  "金额格式不正确",
		Required: false,
	})

	// 发票代码校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "发票代码",
		Field:    "invoice_code",
		Pattern:  `^\d{10,12}$`,
		Message:  "发票代码格式不正确",
		Required: true,
	})

	// 发票号码校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "发票号码",
		Field:    "invoice_number",
		Pattern:  `^\d{8}$`,
		Message:  "发票号码格式不正确",
		Required: true,
	})

	// 统一社会信用代码校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "统一社会信用代码",
		Field:    "credit_code",
		Pattern:  `^[A-Za-z0-9]{18}$`,
		Message:  "统一社会信用代码格式不正确",
		Required: true,
	})

	// 银行卡号校验
	v.rules = append(v.rules, &ValidationRule{
		Name:     "银行卡号",
		Field:    "card_number",
		Pattern:  `^\d{16,19}$`,
		Message:  "银行卡号格式不正确",
		Required: true,
	})

	log.Printf("✅ 已初始化 %d 条校验规则", len(v.rules))
}

// initCorrections 初始化纠错规则.
func (v *Validator) initCorrections() {
	// 常见 OCR 错误纠正
	v.corrections["0"] = "O" // 数字0 -> 字母O
	v.corrections["O"] = "0" // 字母O -> 数字0
	v.corrections["l"] = "1" // 小写L -> 数字1
	v.corrections["I"] = "1" // 大写I -> 数字1
	v.corrections["S"] = "5" // 字母S -> 数字5
	v.corrections["B"] = "8" // 字母B -> 数字8

	// 常见中文错误
	v.corrections["己"] = "已"
	v.corrections["末"] = "未"
	v.corrections["午"] = "牛"
}

// Validate 校验识别结果.
func (v *Validator) Validate(result *OCRResult) {
	log.Printf("🔍 校验识别结果: %s", result.ID)

	if result.Structured == nil {
		return
	}

	validation := &ValidationResult{
		IsValid:  true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Fixed:    make([]string, 0),
	}

	// 校验结构化数据
	for _, rule := range v.rules {
		value, exists := result.Structured.Fields[rule.Field]
		if !exists {
			if rule.Required {
				validation.Errors = append(validation.Errors, rule.Message+": 字段缺失")
				validation.IsValid = false
			}
			continue
		}

		strValue, ok := value.(string)
		if !ok {
			continue
		}

		// 尝试纠错
		fixed := v.tryCorrect(strValue, rule)
		if fixed != strValue {
			result.Structured.Fields[rule.Field] = fixed
			validation.Fixed = append(validation.Fixed, rule.Field+": "+strValue+" -> "+fixed)
		}

		// 校验格式
		if !v.validatePattern(fixed, rule.Pattern) {
			if rule.Required {
				validation.Errors = append(validation.Errors, rule.Message+": "+fixed)
				validation.IsValid = false
			} else {
				validation.Warnings = append(validation.Warnings, rule.Message+": "+fixed)
			}
		}
	}

	// 校验身份证号校验位
	if idNumber, exists := result.Structured.Fields["id_number"]; exists {
		if strID, ok := idNumber.(string); ok {
			if !v.validateIDChecksum(strID) {
				validation.Errors = append(validation.Errors, "身份证号校验位不正确")
				validation.IsValid = false
			}
		}
	}

	// 校验银行卡号 Luhn 算法
	if cardNumber, exists := result.Structured.Fields["card_number"]; exists {
		if strCard, ok := cardNumber.(string); ok {
			if !v.validateLuhn(strCard) {
				validation.Warnings = append(validation.Warnings, "银行卡号校验失败")
			}
		}
	}

	// 更新结构化数据状态
	result.Structured.IsValid = validation.IsValid
	result.Structured.Errors = validation.Errors

	if len(validation.Fixed) > 0 {
		log.Printf("🔧 自动纠错: %v", validation.Fixed)
	}
	if len(validation.Errors) > 0 {
		log.Printf("❌ 校验错误: %v", validation.Errors)
	}
	if len(validation.Warnings) > 0 {
		log.Printf("⚠️ 校验警告: %v", validation.Warnings)
	}

	log.Printf("✅ 校验完成，有效: %v", validation.IsValid)
}

// validatePattern 校验模式.
func (v *Validator) validatePattern(value, pattern string) bool {
	if pattern == "" {
		return true
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("⚠️ 正则表达式编译失败: %s", pattern)
		return true
	}

	return re.MatchString(value)
}

// tryCorrect 尝试纠错.
func (v *Validator) tryCorrect(value string, rule *ValidationRule) string {
	// 根据字段类型进行智能纠错
	switch rule.Field {
	case "id_number":
		return v.correctIDNumber(value)
	case "card_number":
		return v.correctCardNumber(value)
	case "amount":
		return v.correctAmount(value)
	default:
		return value
	}
}

// correctIDNumber 纠错身份证号.
func (v *Validator) correctIDNumber(id string) string {
	if len(id) != 18 {
		return id
	}

	// 常见错误：最后一位 X 应该是大写
	if strings.HasSuffix(id, "x") {
		return id[:17] + "X"
	}

	return id
}

// correctCardNumber 纠错银行卡号.
func (v *Validator) correctCardNumber(card string) string {
	// 移除空格和连字符
	card = strings.ReplaceAll(card, " ", "")
	card = strings.ReplaceAll(card, "-", "")
	return card
}

// correctAmount 纠错金额.
func (v *Validator) correctAmount(amount string) string {
	// 移除多余的空格
	amount = strings.TrimSpace(amount)

	// 统一货币符号
	amount = strings.ReplaceAll(amount, "￥", "¥")
	amount = strings.ReplaceAll(amount, "元", "")

	return amount
}

// validateIDChecksum 校验身份证号校验位.
func (v *Validator) validateIDChecksum(id string) bool {
	if len(id) != 18 {
		return false
	}

	// 加权因子
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	// 校验码对应值
	checkCodes := []byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

	sum := 0
	for i := 0; i < 17; i++ {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
		sum += int(id[i]-'0') * weights[i]
	}

	expected := checkCodes[sum%11]
	actual := id[17]
	if actual >= 'a' && actual <= 'z' {
		actual = actual - 32 // 转大写
	}

	return actual == expected
}

// validateLuhn Luhn 算法校验银行卡号.
func (v *Validator) validateLuhn(card string) bool {
	if len(card) < 13 || len(card) > 19 {
		return false
	}

	sum := 0
	alternate := false

	for i := len(card) - 1; i >= 0; i-- {
		if card[i] < '0' || card[i] > '9' {
			return false
		}

		n := int(card[i] - '0')
		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alternate = !alternate
	}

	return sum%10 == 0
}

// AddRule 添加校验规则.
func (v *Validator) AddRule(rule *ValidationRule) {
	v.rules = append(v.rules, rule)
	log.Printf("✅ 已添加校验规则: %s", rule.Name)
}

// RemoveRule 移除校验规则.
func (v *Validator) RemoveRule(name string) {
	for i, rule := range v.rules {
		if rule.Name == name {
			v.rules = append(v.rules[:i], v.rules[i+1:]...)
			log.Printf("✅ 已移除校验规则: %s", name)
			return
		}
	}
}

// GetRules 获取所有规则.
func (v *Validator) GetRules() []*ValidationRule {
	return v.rules
}

// AddCorrection 添加纠错规则.
func (v *Validator) AddCorrection(from, to string) {
	v.corrections[from] = to
	log.Printf("✅ 已添加纠错规则: %s -> %s", from, to)
}

// GetCorrections 获取所有纠错规则.
func (v *Validator) GetCorrections() map[string]string {
	return v.corrections
}
