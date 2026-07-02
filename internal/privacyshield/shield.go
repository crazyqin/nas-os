package privacyshield

import (
	"crypto/sha256"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewShield 创建新的隐私保护盾实例.
func NewShield() *Shield {
	return &Shield{
		patterns: DefaultPatterns(),
		rules:    DefaultMaskRules(),
	}
}

// NewShieldWithPatterns 使用自定义模式创建隐私保护盾.
func NewShieldWithPatterns(patterns []SensitivePattern, rules map[string]MaskRule) *Shield {
	if rules == nil {
		rules = DefaultMaskRules()
	}
	return &Shield{
		patterns: patterns,
		rules:    rules,
	}
}

// GetPatterns 获取当前所有敏感数据模式.
func (s *Shield) GetPatterns() []SensitivePattern {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.patterns
}

// AddPattern 添加新的敏感数据模式.
func (s *Shield) AddPattern(pattern SensitivePattern) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patterns = append(s.patterns, pattern)
}

// RemovePattern 根据名称移除敏感数据模式.
func (s *Shield) RemovePattern(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.patterns {
		if p.Name == name {
			s.patterns = append(s.patterns[:i], s.patterns[i+1:]...)
			return true
		}
	}
	return false
}

// ScanContent 扫描内容中的敏感数据.
func (s *Shield) ScanContent(content string, categories ...string) (*ScanResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := &ScanResult{
		Matches:    []SensitiveMatch{},
		Categories: make(map[string]int),
		ScannedAt:  time.Now(),
	}

	// 按行处理内容
	lines := strings.Split(content, "\n")
	currentPos := 0

	for lineNum, line := range lines {
		for _, pattern := range s.patterns {
			// 如果指定了分类，只扫描指定分类
			if len(categories) > 0 {
				found := false
				for _, cat := range categories {
					if cat == pattern.Category {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			re, err := regexp.Compile(pattern.Pattern)
			if err != nil {
				continue // 跳过无效的正则
			}

			matches := re.FindAllStringIndex(line, -1)
			for _, match := range matches {
				matchValue := line[match[0]:match[1]]
				result.Matches = append(result.Matches, SensitiveMatch{
					Pattern:   pattern,
					Value:     matchValue,
					Start:     currentPos + match[0],
					End:       currentPos + match[1],
					LineNum:   lineNum + 1,
					RiskLevel: pattern.Severity,
				})
				result.Categories[pattern.Category]++
			}
		}
		currentPos += len(line) + 1 // +1 for newline
	}

	result.TotalMatches = len(result.Matches)
	result.RiskScore = s.calculateScanRiskScore(result)

	return result, nil
}

// MaskContent 对内容进行脱敏处理.
func (s *Shield) MaskContent(content string, strategy string, options *MaskOptions) (*MaskResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &MaskResponse{
		Original: content,
		Masked:   content,
		Strategy: strategy,
	}

	maskedContent := content
	matchCount := 0

	for _, pattern := range s.patterns {
		re, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllString(maskedContent, -1)
		matchCount += len(matches)

		// 获取对应的脱敏规则
		rule, exists := s.rules[pattern.Category]
		if !exists {
			// 使用默认策略
			rule = MaskRule{
				Strategy:   strategy,
				PrefixKeep: 0,
				SuffixKeep: 0,
				MaskChar:   "*",
			}
		}

		// 如果指定了策略，使用指定的策略
		if strategy != "" {
			rule.Strategy = strategy
		}
		if options != nil {
			if options.PrefixKeep > 0 {
				rule.PrefixKeep = options.PrefixKeep
			}
			if options.SuffixKeep > 0 {
				rule.SuffixKeep = options.SuffixKeep
			}
			if options.MaskChar != "" {
				rule.MaskChar = options.MaskChar
			}
		}

		maskedContent = re.ReplaceAllStringFunc(maskedContent, func(match string) string {
			return s.applyMaskRule(match, rule)
		})
	}

	response.Masked = maskedContent
	response.MatchCount = matchCount

	return response, nil
}

// applyMaskRule 应用脱敏规则.
func (s *Shield) applyMaskRule(value string, rule MaskRule) string {
	runes := []rune(value)
	length := len(runes)

	switch rule.Strategy {
	case "mask":
		// 完全掩码
		return strings.Repeat(rule.MaskChar, length)

	case "partial":
		// 保留首尾，中间掩码
		if length <= rule.PrefixKeep+rule.SuffixKeep {
			return strings.Repeat(rule.MaskChar, length)
		}
		prefix := string(runes[:rule.PrefixKeep])
		suffix := ""
		if rule.SuffixKeep > 0 {
			suffix = string(runes[length-rule.SuffixKeep:])
		}
		maskLen := length - rule.PrefixKeep - rule.SuffixKeep
		return prefix + strings.Repeat(rule.MaskChar, maskLen) + suffix

	case "hash":
		// SHA-256 哈希截断
		hash := sha256.Sum256([]byte(value))
		hashStr := fmt.Sprintf("%x", hash)
		if len(hashStr) > 16 {
			hashStr = hashStr[:16]
		}
		return "[HASH:" + hashStr + "]"

	case "redact":
		// 完全删除
		return "[REDACTED]"

	default:
		// 默认使用 partial 策略
		return s.applyMaskRule(value, MaskRule{
			Strategy:   "partial",
			PrefixKeep: 3,
			SuffixKeep: 4,
			MaskChar:   "*",
		})
	}
}

// GenerateComplianceReport 生成合规检查报告.
func (s *Shield) GenerateComplianceReport(content string, framework string) (*ComplianceReport, error) {
	scanResult, err := s.ScanContent(content)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}

	report := &ComplianceReport{
		ID:          uuid.New().String(),
		GeneratedAt: time.Now(),
		Framework:   framework,
		TotalItems:  len(scanResult.Matches),
		Issues:      []ComplianceIssue{},
	}

	// 根据框架进行检查
	switch framework {
	case "GDPR":
		report.Issues = s.checkGDPRCompliance(scanResult)
	case "PIPL":
		report.Issues = s.checkPIPLCompliance(scanResult)
	case "ALL":
		report.Issues = append(report.Issues, s.checkGDPRCompliance(scanResult)...)
		report.Issues = append(report.Issues, s.checkPIPLCompliance(scanResult)...)
	default:
		report.Issues = s.checkGDPRCompliance(scanResult)
	}

	// 计算合规分数
	report.NonCompliantItems = len(report.Issues)
	report.CompliantItems = report.TotalItems - report.NonCompliantItems
	if report.TotalItems > 0 {
		report.Score = float64(report.CompliantItems) / float64(report.TotalItems) * 100
	} else {
		report.Score = 100
	}

	// 确定状态
	if report.Score >= 90 {
		report.Status = "compliant"
	} else if report.Score >= 70 {
		report.Status = "warning"
	} else {
		report.Status = "non-compliant"
	}

	return report, nil
}

// checkGDPRCompliance 检查 GDPR 合规性.
func (s *Shield) checkGDPRCompliance(scanResult *ScanResult) []ComplianceIssue {
	issues := []ComplianceIssue{}

	// 检查是否包含高敏感度数据
	highSeverityCategories := []string{"id_card", "bank_card"}
	for _, category := range highSeverityCategories {
		if count, exists := scanResult.Categories[category]; exists && count > 0 {
			issues = append(issues, ComplianceIssue{
				Type:        "high_sensitivity_data",
				Severity:    "critical",
				Description: fmt.Sprintf("检测到高敏感度数据类型: %s，数量: %d", category, count),
				Items:       s.getMatchValuesByCategory(scanResult, category),
				Remediation: "建议对高敏感度数据进行加密存储和脱敏处理",
			})
		}
	}

	// 检查是否包含个人身份信息
	if count, exists := scanResult.Categories["id_card"]; exists && count > 0 {
		issues = append(issues, ComplianceIssue{
			Type:        "pii_detected",
			Severity:    "high",
			Description: "检测到个人身份信息（PII）",
			Items:       s.getMatchValuesByCategory(scanResult, "id_card"),
			Remediation: "根据GDPR第9条，处理个人身份信息需要明确同意或合法基础",
		})
	}

	// 检查数据密度
	if scanResult.TotalMatches > 10 {
		issues = append(issues, ComplianceIssue{
			Type:        "high_data_density",
			Severity:    "medium",
			Description: fmt.Sprintf("数据密度过高，检测到 %d 个敏感数据", scanResult.TotalMatches),
			Items:       []string{},
			Remediation: "建议对数据进行分类分级，降低敏感数据密度",
		})
	}

	return issues
}

// checkPIPLCompliance 检查个人信息保护法合规性.
func (s *Shield) checkPIPLCompliance(scanResult *ScanResult) []ComplianceIssue {
	issues := []ComplianceIssue{}

	// 检查身份证信息
	if count, exists := scanResult.Categories["id_card"]; exists && count > 0 {
		issues = append(issues, ComplianceIssue{
			Type:        "identity_data",
			Severity:    "critical",
			Description: "检测到身份证信息，属于敏感个人信息",
			Items:       s.getMatchValuesByCategory(scanResult, "id_card"),
			Remediation: "根据《个人信息保护法》第28条，处理敏感个人信息应当取得个人的单独同意",
		})
	}

	// 检查手机号码
	if count, exists := scanResult.Categories["phone"]; exists && count > 0 {
		issues = append(issues, ComplianceIssue{
			Type:        "contact_data",
			Severity:    "high",
			Description: fmt.Sprintf("检测到 %d 个手机号码", count),
			Items:       s.getMatchValuesByCategory(scanResult, "phone"),
			Remediation: "处理手机号码需要告知处理目的并取得同意",
		})
	}

	// 检查银行卡信息
	if count, exists := scanResult.Categories["bank_card"]; exists && count > 0 {
		issues = append(issues, ComplianceIssue{
			Type:        "financial_data",
			Severity:    "critical",
			Description: "检测到银行卡信息，属于金融敏感信息",
			Items:       s.getMatchValuesByCategory(scanResult, "bank_card"),
			Remediation: "金融信息需要特别保护，建议加密存储并限制访问权限",
		})
	}

	return issues
}

// getMatchValuesByCategory 获取指定分类的匹配值.
func (s *Shield) getMatchValuesByCategory(scanResult *ScanResult, category string) []string {
	values := []string{}
	for _, match := range scanResult.Matches {
		if match.Pattern.Category == category {
			// 脱敏后展示
			masked := s.applyMaskRule(match.Value, MaskRule{
				Strategy:   "partial",
				PrefixKeep: 2,
				SuffixKeep: 2,
				MaskChar:   "*",
			})
			values = append(values, masked)
		}
	}
	return values
}

// calculateScanRiskScore 计算扫描风险分数.
func (s *Shield) calculateScanRiskScore(scanResult *ScanResult) float64 {
	if len(scanResult.Matches) == 0 {
		return 0
	}

	// 计算敏感数据密度 (0-1)
	density := math.Min(float64(scanResult.TotalMatches)/100.0, 1.0)

	// 计算平均风险等级 (0-1)
	totalRisk := 0
	for _, match := range scanResult.Matches {
		totalRisk += match.RiskLevel
	}
	avgRisk := float64(totalRisk) / float64(len(scanResult.Matches)) / 10.0

	// 综合评分 = 敏感数据密度 * 0.4 + 访问范围 * 0.3 + 加密状态 * 0.3
	// 这里简化处理，访问范围和加密状态默认为中等风险
	accessScope := 0.5
	encrypted := 0.5

	score := density*0.4 + accessScope*0.3 + encrypted*0.3
	_ = avgRisk                     // used for potential risk level calculation
	return math.Min(score*100, 100) // 归一化到 0-100
}

// AssessRisk 进行风险评估.
func (s *Shield) AssessRisk(content string, encrypted bool, accessLevel string) (*RiskScore, error) {
	scanResult, err := s.ScanContent(content)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}

	riskScore := &RiskScore{
		Breakdown:  make(map[string]float64),
		AssessedAt: time.Now(),
	}

	// 计算敏感数据密度 (0-1)
	if len(content) > 0 {
		riskScore.Density = math.Min(float64(scanResult.TotalMatches)/float64(len(content))*1000, 1.0)
	}

	// 访问范围评分 (0-1, 1表示高风险)
	switch accessLevel {
	case "public":
		riskScore.AccessScope = 1.0
	case "internal":
		riskScore.AccessScope = 0.7
	case "private":
		riskScore.AccessScope = 0.4
	case "restricted":
		riskScore.AccessScope = 0.2
	default:
		riskScore.AccessScope = 0.5
	}

	// 加密状态评分 (0-1, 1表示高风险，即未加密)
	if encrypted {
		riskScore.Encrypted = 0.2
	} else {
		riskScore.Encrypted = 1.0
	}

	// 各分类风险明细
	for category, count := range scanResult.Categories {
		categoryRisk := float64(count) / 10.0
		if categoryRisk > 1.0 {
			categoryRisk = 1.0
		}
		riskScore.Breakdown[category] = categoryRisk * 100
	}

	// 综合评分 = 密度 * 0.4 + 访问范围 * 0.3 + 加密状态 * 0.3
	riskScore.Overall = (riskScore.Density*0.4 + riskScore.AccessScope*0.3 + riskScore.Encrypted*0.3) * 100
	riskScore.Overall = math.Min(riskScore.Overall, 100)

	// 确定风险等级
	if riskScore.Overall >= 80 {
		riskScore.RiskLevel = "critical"
	} else if riskScore.Overall >= 60 {
		riskScore.RiskLevel = "high"
	} else if riskScore.Overall >= 40 {
		riskScore.RiskLevel = "medium"
	} else {
		riskScore.RiskLevel = "low"
	}

	return riskScore, nil
}
