// classifier.go - 文档自动分类
package aiocr

import (
	"log"
	"strings"
)

// Classifier 文档分类器.
type Classifier struct {
	config   *Config
	rules    []*ClassificationRule
	keywords map[DocumentCategory][]string
}

// ClassificationRule 分类规则.
type ClassificationRule struct {
	Name     string           `json:"name"`     // 规则名称
	Category DocumentCategory `json:"category"` // 分类
	Keywords []string         `json:"keywords"` // 关键词
	Patterns []string         `json:"patterns"` // 正则模式
	Priority int              `json:"priority"` // 优先级
	Weight   float64          `json:"weight"`   // 权重
	Enabled  bool             `json:"enabled"`  // 是否启用
}

// NewClassifier 创建分类器.
func NewClassifier(cfg *Config) *Classifier {
	c := &Classifier{
		config:   cfg,
		rules:    make([]*ClassificationRule, 0),
		keywords: make(map[DocumentCategory][]string),
	}

	// 初始化分类规则
	c.initRules()

	return c
}

// initRules 初始化分类规则.
func (c *Classifier) initRules() {
	// 发票分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "发票识别",
		Category: CategoryInvoice,
		Keywords: []string{
			"发票", "增值税", "普通发票", "专用发票", "电子发票",
			"发票代码", "发票号码", "开票日期", "价税合计",
			"购买方", "销售方", "纳税人识别号",
		},
		Priority: 10,
		Weight:   1.0,
		Enabled:  true,
	})

	// 合同分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "合同识别",
		Category: CategoryContract,
		Keywords: []string{
			"合同", "协议", "甲方", "乙方", "签订",
			"盖章", "签字", "生效", "终止", "违约",
		},
		Priority: 9,
		Weight:   1.0,
		Enabled:  true,
	})

	// 身份证分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "身份证识别",
		Category: CategoryIDCard,
		Keywords: []string{
			"居民身份证", "身份证", "姓名", "性别", "民族",
			"出生", "住址", "公民身份号码", "签发机关", "有效期限",
		},
		Priority: 10,
		Weight:   1.0,
		Enabled:  true,
	})

	// 营业执照分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "营业执照识别",
		Category: CategoryBusiness,
		Keywords: []string{
			"营业执照", "统一社会信用代码", "法定代表人",
			"注册资本", "成立日期", "经营范围", "企业类型",
		},
		Priority: 10,
		Weight:   1.0,
		Enabled:  true,
	})

	// 银行卡分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "银行卡识别",
		Category: CategoryBankCard,
		Keywords: []string{
			"银行", "借记卡", "信用卡", "储蓄卡", "卡号",
			"有效期", "持卡人", "银联", "VISA", "MasterCard",
		},
		Priority: 8,
		Weight:   1.0,
		Enabled:  true,
	})

	// 收据分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "收据识别",
		Category: CategoryReceipt,
		Keywords: []string{
			"收据", "收款", "收到", "金额", "日期",
			"经手人", "盖章",
		},
		Priority: 7,
		Weight:   1.0,
		Enabled:  true,
	})

	// 报告分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "报告识别",
		Category: CategoryReport,
		Keywords: []string{
			"报告", "分析", "研究", "调查", "统计",
			"数据", "结论", "建议",
		},
		Priority: 5,
		Weight:   1.0,
		Enabled:  true,
	})

	// 表单分类规则
	c.rules = append(c.rules, &ClassificationRule{
		Name:     "表单识别",
		Category: CategoryForm,
		Keywords: []string{
			"申请表", "登记表", "表格", "填写", "表格",
			"项目", "内容", "备注",
		},
		Priority: 6,
		Weight:   1.0,
		Enabled:  true,
	})

	log.Printf("✅ 已初始化 %d 条分类规则", len(c.rules))
}

// Classify 文档分类.
func (c *Classifier) Classify(text string, pages []*PageResult) *ClassificationResult {
	log.Printf("🏷️ 开始文档分类")

	// 统计关键词匹配
	scores := make(map[DocumentCategory]float64)

	for _, rule := range c.rules {
		if !rule.Enabled {
			continue
		}

		score := c.calculateScore(text, rule)
		scores[rule.Category] += score * rule.Weight
	}

	// 找到最高分
	var bestCategory DocumentCategory
	bestScore := 0.0

	for category, score := range scores {
		if score > bestScore {
			bestScore = score
			bestCategory = category
		}
	}

	// 计算置信度
	confidence := c.calculateConfidence(bestScore, scores)

	result := &ClassificationResult{
		Category:    bestCategory,
		Confidence:  confidence,
		Labels:      c.generateLabels(text, bestCategory),
		Suggestions: c.generateSuggestions(bestCategory),
	}

	log.Printf("✅ 文档分类完成: %s, 置信度: %.2f", bestCategory, confidence)
	return result
}

// calculateScore 计算匹配分数.
func (c *Classifier) calculateScore(text string, rule *ClassificationRule) float64 {
	if len(rule.Keywords) == 0 {
		return 0
	}

	lowerText := strings.ToLower(text)
	matches := 0

	for _, keyword := range rule.Keywords {
		if strings.Contains(lowerText, strings.ToLower(keyword)) {
			matches++
		}
	}

	// 返回匹配比例
	return float64(matches) / float64(len(rule.Keywords))
}

// calculateConfidence 计算置信度.
func (c *Classifier) calculateConfidence(bestScore float64, allScores map[DocumentCategory]float64) float64 {
	if bestScore == 0 {
		return 0
	}

	// 计算相对置信度
	totalScore := 0.0
	for _, score := range allScores {
		totalScore += score
	}

	if totalScore == 0 {
		return 0
	}

	// 相对置信度
	relativeConf := bestScore / totalScore

	// 绝对置信度
	absoluteConf := bestScore

	// 综合置信度
	return relativeConf*0.6 + absoluteConf*0.4
}

// generateLabels 生成标签.
func (c *Classifier) generateLabels(text string, category DocumentCategory) []string {
	labels := make([]string, 0)

	// 添加分类标签
	labels = append(labels, string(category))

	// 根据内容添加更多标签
	lowerText := strings.ToLower(text)

	labelPatterns := map[string][]string{
		"important": {"重要", "紧急", "机密", "保密"},
		"financial": {"财务", "会计", "报销", "付款"},
		"legal":     {"法律", "法规", "合规", "条款"},
		"official":  {"官方", "正式", "公章", "印章"},
	}

	for label, keywords := range labelPatterns {
		for _, keyword := range keywords {
			if strings.Contains(lowerText, keyword) {
				labels = append(labels, label)
				break
			}
		}
	}

	return labels
}

// generateSuggestions 生成建议.
func (c *Classifier) generateSuggestions(category DocumentCategory) []string {
	suggestions := make([]string, 0)

	switch category {
	case CategoryInvoice:
		suggestions = append(suggestions, "建议提取发票信息并归档")
		suggestions = append(suggestions, "可用于财务报销和税务申报")
	case CategoryContract:
		suggestions = append(suggestions, "建议提取合同关键条款")
		suggestions = append(suggestions, "设置合同到期提醒")
	case CategoryIDCard:
		suggestions = append(suggestions, "建议进行敏感信息脱敏")
		suggestions = append(suggestions, "妥善保管避免信息泄露")
	case CategoryBusiness:
		suggestions = append(suggestions, "建议提取企业信息用于备案")
	case CategoryBankCard:
		suggestions = append(suggestions, "建议进行卡号脱敏处理")
		suggestions = append(suggestions, "注意信息安全保护")
	}

	return suggestions
}

// AddRule 添加分类规则.
func (c *Classifier) AddRule(rule *ClassificationRule) {
	c.rules = append(c.rules, rule)
	log.Printf("✅ 已添加分类规则: %s", rule.Name)
}

// RemoveRule 移除分类规则.
func (c *Classifier) RemoveRule(name string) {
	for i, rule := range c.rules {
		if rule.Name == name {
			c.rules = append(c.rules[:i], c.rules[i+1:]...)
			log.Printf("✅ 已移除分类规则: %s", name)
			return
		}
	}
}

// GetRules 获取所有规则.
func (c *Classifier) GetRules() []*ClassificationRule {
	return c.rules
}

// UpdateRule 更新规则.
func (c *Classifier) UpdateRule(name string, rule *ClassificationRule) {
	for i, r := range c.rules {
		if r.Name == name {
			c.rules[i] = rule
			log.Printf("✅ 已更新分类规则: %s", name)
			return
		}
	}
}
