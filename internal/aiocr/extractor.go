// extractor.go - 结构化信息提取（发票、合同、身份证等）
package aiocr

import (
	"log"
	"regexp"
	"strings"
)

// Extractor 结构化信息提取器.
type Extractor struct {
	config    *Config
	templates map[DocumentCategory]*DocumentTemplate
}

// NewExtractor 创建提取器.
func NewExtractor(cfg *Config) *Extractor {
	e := &Extractor{
		config:    cfg,
		templates: make(map[DocumentCategory]*DocumentTemplate),
	}

	// 初始化文档模板
	e.initTemplates()

	return e
}

// initTemplates 初始化文档模板.
func (e *Extractor) initTemplates() {
	// 发票模板
	e.templates[CategoryInvoice] = &DocumentTemplate{
		ID:          "tpl_invoice",
		Name:        "发票模板",
		Type:        "invoice",
		Description: "增值税发票、普通发票识别",
		Fields: []*TemplateField{
			{Name: "invoice_code", Type: "string", Label: "发票代码", Pattern: `\d{10,12}`, Required: true},
			{Name: "invoice_number", Type: "string", Label: "发票号码", Pattern: `\d{8}`, Required: true},
			{Name: "invoice_date", Type: "date", Label: "开票日期", Pattern: `\d{4}年\d{1,2}月\d{1,2}日`, Required: true},
			{Name: "amount", Type: "amount", Label: "金额", Pattern: `¥?\d+\.\d{2}`, Required: true},
			{Name: "tax", Type: "amount", Label: "税额", Pattern: `¥?\d+\.\d{2}`, Required: true},
			{Name: "total", Type: "amount", Label: "价税合计", Pattern: `¥?\d+\.\d{2}`, Required: true},
			{Name: "buyer_name", Type: "string", Label: "购买方名称", Required: true},
			{Name: "seller_name", Type: "string", Label: "销售方名称", Required: true},
			{Name: "tax_number", Type: "string", Label: "纳税人识别号", Pattern: `[A-Za-z0-9]{15,20}`, Sensitive: true},
		},
		Keywords: []string{"发票", "增值税", "税额", "价税合计", "购买方", "销售方"},
		Enabled:  true,
	}

	// 合同模板
	e.templates[CategoryContract] = &DocumentTemplate{
		ID:          "tpl_contract",
		Name:        "合同模板",
		Type:        "contract",
		Description: "各类合同协议识别",
		Fields: []*TemplateField{
			{Name: "contract_number", Type: "string", Label: "合同编号"},
			{Name: "party_a", Type: "string", Label: "甲方", Required: true},
			{Name: "party_b", Type: "string", Label: "乙方", Required: true},
			{Name: "sign_date", Type: "date", Label: "签订日期"},
			{Name: "start_date", Type: "date", Label: "开始日期"},
			{Name: "end_date", Type: "date", Label: "结束日期"},
			{Name: "amount", Type: "amount", Label: "合同金额"},
		},
		Keywords: []string{"合同", "协议", "甲方", "乙方", "签订", "盖章"},
		Enabled:  true,
	}

	// 身份证模板
	e.templates[CategoryIDCard] = &DocumentTemplate{
		ID:          "tpl_idcard",
		Name:        "身份证模板",
		Type:        "id_card",
		Description: "身份证正反面识别",
		Fields: []*TemplateField{
			{Name: "name", Type: "string", Label: "姓名", Required: true},
			{Name: "gender", Type: "string", Label: "性别"},
			{Name: "ethnicity", Type: "string", Label: "民族"},
			{Name: "birth_date", Type: "date", Label: "出生日期"},
			{Name: "address", Type: "string", Label: "住址"},
			{Name: "id_number", Type: "string", Label: "身份证号", Pattern: `\d{17}[\dXx]`, Required: true, Sensitive: true},
		},
		Keywords: []string{"居民身份证", "姓名", "性别", "民族", "住址", "公民身份号码"},
		Enabled:  true,
	}

	// 营业执照模板
	e.templates[CategoryBusiness] = &DocumentTemplate{
		ID:          "tpl_business",
		Name:        "营业执照模板",
		Type:        "business_license",
		Description: "营业执照识别",
		Fields: []*TemplateField{
			{Name: "company_name", Type: "string", Label: "公司名称", Required: true},
			{Name: "legal_representative", Type: "string", Label: "法定代表人", Required: true},
			{Name: "registered_capital", Type: "string", Label: "注册资本"},
			{Name: "establishment_date", Type: "date", Label: "成立日期"},
			{Name: "business_scope", Type: "string", Label: "经营范围"},
			{Name: "credit_code", Type: "string", Label: "统一社会信用代码", Pattern: `[A-Za-z0-9]{18}`, Sensitive: true},
		},
		Keywords: []string{"营业执照", "统一社会信用代码", "法定代表人", "注册资本", "经营范围"},
		Enabled:  true,
	}

	// 银行卡模板
	e.templates[CategoryBankCard] = &DocumentTemplate{
		ID:          "tpl_bankcard",
		Name:        "银行卡模板",
		Type:        "bank_card",
		Description: "银行卡识别",
		Fields: []*TemplateField{
			{Name: "bank_name", Type: "string", Label: "银行名称", Required: true},
			{Name: "card_number", Type: "string", Label: "卡号", Pattern: `\d{16,19}`, Required: true, Sensitive: true},
			{Name: "cardholder", Type: "string", Label: "持卡人"},
			{Name: "expiry_date", Type: "string", Label: "有效期", Pattern: `\d{2}/\d{2}`},
		},
		Keywords: []string{"银行", "借记卡", "信用卡", "储蓄卡"},
		Enabled:  true,
	}

	log.Printf("✅ 已初始化 %d 个文档模板", len(e.templates))
}

// Extract 提取结构化信息.
func (e *Extractor) Extract(text string, pages []*PageResult, category DocumentCategory) *StructuredData {
	log.Printf("📋 提取结构化信息，分类: %s", category)

	data := &StructuredData{
		DocumentType: string(category),
		Fields:       make(map[string]interface{}),
		IsValid:      true,
		Errors:       make([]string, 0),
	}

	// 获取对应模板
	tmpl, exists := e.templates[category]
	if !exists {
		// 尝试自动匹配
		tmpl = e.matchTemplate(text)
		if tmpl == nil {
			log.Println("⚠️ 未找到匹配的文档模板")
			return data
		}
		data.Template = tmpl.Name
	}

	data.Template = tmpl.Name

	// 提取字段
	for _, field := range tmpl.Fields {
		value := e.extractField(text, field)
		if value != "" {
			data.Fields[field.Name] = value
		} else if field.Required {
			data.Errors = append(data.Errors, "缺少必需字段: "+field.Label)
			data.IsValid = false
		}
	}

	// 计算置信度
	data.Confidence = e.calculateExtractionConfidence(data, tmpl)

	log.Printf("✅ 结构化信息提取完成，字段: %d, 置信度: %.2f",
		len(data.Fields), data.Confidence)

	return data
}

// matchTemplate 自动匹配模板.
func (e *Extractor) matchTemplate(text string) *DocumentTemplate {
	bestMatch := 0
	var bestTemplate *DocumentTemplate

	lowerText := strings.ToLower(text)

	for _, tmpl := range e.templates {
		if !tmpl.Enabled {
			continue
		}

		score := 0
		for _, keyword := range tmpl.Keywords {
			if strings.Contains(lowerText, strings.ToLower(keyword)) {
				score++
			}
		}

		if score > bestMatch {
			bestMatch = score
			bestTemplate = tmpl
		}
	}

	if bestMatch >= 2 {
		return bestTemplate
	}

	return nil
}

// extractField 提取单个字段.
func (e *Extractor) extractField(text string, field *TemplateField) string {
	if field.Pattern == "" {
		// 没有模式，尝试根据标签提取
		return e.extractByLabel(text, field.Label)
	}

	// 使用正则表达式提取
	re, err := regexp.Compile(field.Pattern)
	if err != nil {
		log.Printf("⚠️ 正则表达式编译失败: %s, %v", field.Pattern, err)
		return ""
	}

	matches := re.FindString(text)
	return matches
}

// extractByLabel 根据标签提取.
func (e *Extractor) extractByLabel(text, label string) string {
	// 查找标签后面的值
	// 例如："姓名: 张三" -> "张三"
	patterns := []string{
		label + `[：:]\s*(.+)`,
		label + `\s*[：:]\s*(.+)`,
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// calculateExtractionConfidence 计算提取置信度.
func (e *Extractor) calculateExtractionConfidence(data *StructuredData, tmpl *DocumentTemplate) float64 {
	if len(tmpl.Fields) == 0 {
		return 0
	}

	found := 0
	requiredFound := 0
	requiredTotal := 0

	for _, field := range tmpl.Fields {
		_, exists := data.Fields[field.Name]
		if exists {
			found++
			if field.Required {
				requiredFound++
			}
		}
		if field.Required {
			requiredTotal++
		}
	}

	// 基础置信度：找到的字段比例
	baseConf := float64(found) / float64(len(tmpl.Fields))

	// 必需字段置信度
	requiredConf := 1.0
	if requiredTotal > 0 {
		requiredConf = float64(requiredFound) / float64(requiredTotal)
	}

	// 综合置信度
	return baseConf*0.4 + requiredConf*0.6
}

// GetTemplate 获取模板.
func (e *Extractor) GetTemplate(category DocumentCategory) *DocumentTemplate {
	return e.templates[category]
}

// GetTemplates 获取所有模板.
func (e *Extractor) GetTemplates() map[DocumentCategory]*DocumentTemplate {
	return e.templates
}

// AddTemplate 添加模板.
func (e *Extractor) AddTemplate(category DocumentCategory, tmpl *DocumentTemplate) {
	e.templates[category] = tmpl
	log.Printf("✅ 已添加文档模板: %s (%s)", tmpl.Name, category)
}

// RemoveTemplate 移除模板.
func (e *Extractor) RemoveTemplate(category DocumentCategory) {
	delete(e.templates, category)
	log.Printf("✅ 已移除文档模板: %s", category)
}
