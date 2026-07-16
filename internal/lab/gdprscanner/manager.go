// Package gdprscanner 提供隐私合规扫描核心业务逻辑
package gdprscanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ScannerManager 隐私合规扫描管理器.
type ScannerManager struct {
	patterns []*PIIPattern
	reports  map[string]*ComplianceReport
	mu       sync.RWMutex
}

// NewScannerManager 创建扫描管理器.
func NewScannerManager() *ScannerManager {
	m := &ScannerManager{
		patterns: getDefaultPatterns(),
		reports:  make(map[string]*ComplianceReport),
	}
	return m
}

// getDefaultPatterns 获取默认 PII 匹配模式.
func getDefaultPatterns() []*PIIPattern {
	return []*PIIPattern{
		{
			Name:        "中国身份证号",
			Category:    CategoryIDCard,
			Pattern:     `\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`,
			Sensitivity: SensitivityHigh,
			Description: "18位中国居民身份证号码",
		},
		{
			Name:        "手机号码",
			Category:    CategoryPhone,
			Pattern:     `\b1[3-9]\d{9}\b`,
			Sensitivity: SensitivityMedium,
			Description: "中国大陆手机号码",
		},
		{
			Name:        "电子邮箱",
			Category:    CategoryEmail,
			Pattern:     `\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`,
			Sensitivity: SensitivityLow,
			Description: "电子邮箱地址",
		},
		{
			Name:        "银行卡号",
			Category:    CategoryBankCard,
			Pattern:     `\b(?:6[0-9]{15,18}|4[0-9]{12,15}|5[1-5][0-9]{14}|3[47][0-9]{13})\b`,
			Sensitivity: SensitivityHigh,
			Description: "银行卡号（含 Visa/MasterCard/银联）",
		},
		{
			Name:        "护照号码",
			Category:    CategoryPassport,
			Pattern:     `\b[A-Z][0-9]{8}\b`,
			Sensitivity: SensitivityHigh,
			Description: "中国护照号码",
		},
		{
			Name:        "车牌号",
			Category:    CategoryLicensePlate,
			Pattern:     `\b[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤川青藏琼宁][A-Z][A-HJ-NP-Z0-9]{4,5}[A-HJ-NP-Z0-9挂学警港澳]\b`,
			Sensitivity: SensitivityMedium,
			Description: "中国车牌号",
		},
		{
			Name:        "IP 地址",
			Category:    CategoryIPAddress,
			Pattern:     `\b(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`,
			Sensitivity: SensitivityLow,
			Description: "IPv4 地址",
		},
	}
}

// ScanFiles 扫描指定路径下的文件.
func (m *ScannerManager) ScanFiles(req ScanRequest) (*ComplianceReport, error) {
	if req.Path == "" {
		return nil, fmt.Errorf("scan path is required")
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// 默认文件扩展名
	extensions := req.Extensions
	if len(extensions) == 0 {
		extensions = []string{".txt", ".csv", ".json", ".xml", ".log", ".md", ".html", ".yml", ".yaml", ".conf", ".cfg", ".ini"}
	}

	// 默认最大深度
	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	// 收集需要扫描的文件
	files := make([]string, 0)
	if info.IsDir() {
		err = filepath.Walk(req.Path, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无法访问的文件
			}

			// 检查目录深度
			relPath, _ := filepath.Rel(req.Path, path)
			depth := strings.Count(relPath, string(os.PathSeparator))
			if depth > maxDepth {
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// 检查是否在排除目录中
			for _, dir := range req.ExcludeDirs {
				if strings.Contains(path, dir) {
					if fi.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if fi.IsDir() {
				return nil
			}

			// 检查文件扩展名
			ext := filepath.Ext(path)
			for _, e := range extensions {
				if strings.EqualFold(ext, e) {
					files = append(files, path)
					break
				}
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk path failed: %w", err)
		}
	} else {
		files = append(files, req.Path)
	}

	// 扫描文件
	results := make([]*ScanResult, 0, len(files))
	for _, f := range files {
		result := m.scanSingleFile(f)
		results = append(results, result)
	}

	// 生成报告
	report := m.GenerateComplianceReport(results)

	// 保存报告
	m.mu.Lock()
	m.reports[report.ID] = report
	m.mu.Unlock()

	return report, nil
}

// scanSingleFile 扫描单个文件.
func (m *ScannerManager) scanSingleFile(filePath string) *ScanResult {
	result := &ScanResult{
		FilePath: filePath,
		ScanTime: time.Now(),
		Matches:  make([]PIIMatch, 0),
	}

	file, err := os.Open(filePath)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer file.Close()

	// 编译所有正则
	regexps := make([]*regexp.Regexp, len(m.patterns))
	for i, p := range m.patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			continue
		}
		regexps[i] = re
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for i, re := range regexps {
			if re == nil {
				continue
			}
			locs := re.FindAllStringIndex(line, -1)
			for _, loc := range locs {
				value := line[loc[0]:loc[1]]
				// 提取上下文
				ctxStart := loc[0] - 20
				if ctxStart < 0 {
					ctxStart = 0
				}
				ctxEnd := loc[1] + 20
				if ctxEnd > len(line) {
					ctxEnd = len(line)
				}
				context := line[ctxStart:ctxEnd]

				match := PIIMatch{
					Category:    m.patterns[i].Category,
					Value:       value,
					Line:        lineNum,
					Column:      loc[0] + 1,
					Sensitivity: m.patterns[i].Sensitivity,
					Context:     context,
				}
				result.Matches = append(result.Matches, match)
			}
		}
	}

	result.TotalMatch = len(result.Matches)
	return result
}

// ClassifyPII 对 PII 匹配结果进行分类统计.
func (m *ScannerManager) ClassifyPII(matches []PIIMatch) CategorySummary {
	summary := CategorySummary{}
	for _, match := range matches {
		switch match.Category {
		case CategoryIDCard:
			summary.IDCardCount++
		case CategoryPhone:
			summary.PhoneCount++
		case CategoryEmail:
			summary.EmailCount++
		case CategoryBankCard:
			summary.BankCardCount++
		case CategoryPassport:
			summary.PassportCount++
		case CategoryLicensePlate:
			summary.LicensePlateCount++
		case CategoryIPAddress:
			summary.IPAddressCount++
		}
	}
	return summary
}

// GenerateComplianceReport 生成合规报告.
func (m *ScannerManager) GenerateComplianceReport(results []*ScanResult) *ComplianceReport {
	totalMatches := 0
	allMatches := make([]PIIMatch, 0)
	for _, r := range results {
		totalMatches += r.TotalMatch
		allMatches = append(allMatches, r.Matches...)
	}

	summary := m.ClassifyPII(allMatches)
	riskLevel := m.calculateRiskLevel(summary, totalMatches)
	suggestions := m.generateSuggestions(summary, totalMatches)

	report := &ComplianceReport{
		ID:           uuid.New().String(),
		ScanTime:     time.Now(),
		TotalFiles:   len(results),
		ScannedFiles: len(results),
		TotalMatches: totalMatches,
		Results:      results,
		Summary:      summary,
		RiskLevel:    riskLevel,
		Suggestions:  suggestions,
	}

	return report
}

// calculateRiskLevel 计算风险等级.
func (m *ScannerManager) calculateRiskLevel(summary CategorySummary, totalMatches int) string {
	// 高敏感数据存在即为高风险
	highSensitivityCount := summary.IDCardCount + summary.BankCardCount + summary.PassportCount
	if highSensitivityCount > 0 {
		return "高"
	}

	// 中敏感数据超过阈值为中风险
	mediumSensitivityCount := summary.PhoneCount + summary.LicensePlateCount
	if mediumSensitivityCount > 10 {
		return "中"
	}

	// 低敏感数据大量存在为低风险
	if totalMatches > 50 {
		return "低"
	}

	return "安全"
}

// generateSuggestions 生成脱敏建议.
func (m *ScannerManager) generateSuggestions(summary CategorySummary, totalMatches int) []string {
	suggestions := make([]string, 0)

	if summary.IDCardCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 处身份证号，建议使用部分遮掩脱敏（如 110***********1234）", summary.IDCardCount))
	}
	if summary.BankCardCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 处银行卡号，建议保留前6后4位（如 622588******1234）", summary.BankCardCount))
	}
	if summary.PhoneCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 处手机号，建议中间4位脱敏（如 138****5678）", summary.PhoneCount))
	}
	if summary.EmailCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 处邮箱地址，建议用户名部分脱敏（如 t***@example.com）", summary.EmailCount))
	}
	if summary.PassportCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 处护照号，建议完全脱敏或加密存储", summary.PassportCount))
	}
	if summary.LicensePlateCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 处车牌号，建议部分遮掩（如 京A***89）", summary.LicensePlateCount))
	}
	if summary.IPAddressCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 处IP地址，建议最后一段置零（如 192.168.1.0）", summary.IPAddressCount))
	}

	if totalMatches == 0 {
		suggestions = append(suggestions, "未发现个人隐私信息，数据合规状态良好")
	}

	suggestions = append(suggestions, "建议定期进行隐私合规扫描，确保数据安全")
	suggestions = append(suggestions, "根据 GDPR 第25条，应实施数据保护设计和默认数据保护措施")

	return suggestions
}

// SuggestMasking 获取脱敏建议.
func (m *ScannerManager) SuggestMasking() []MaskingSuggestion {
	return []MaskingSuggestion{
		{
			Category:    CategoryIDCard,
			Strategy:    "部分遮掩",
			Example:     "110***********1234",
			Description: "保留前3位和后4位，中间用*替代",
		},
		{
			Category:    CategoryPhone,
			Strategy:    "中间脱敏",
			Example:     "138****5678",
			Description: "保留前3位和后4位，中间4位用*替代",
		},
		{
			Category:    CategoryEmail,
			Strategy:    "用户名脱敏",
			Example:     "t***@example.com",
			Description: "保留首字母和@后域名，其余用*替代",
		},
		{
			Category:    CategoryBankCard,
			Strategy:    "前6后4",
			Example:     "622588******1234",
			Description: "保留前6位BIN码和后4位，中间用*替代",
		},
		{
			Category:    CategoryPassport,
			Strategy:    "完全脱敏",
			Example:     "*********",
			Description: "护照号建议完全脱敏或加密存储",
		},
		{
			Category:    CategoryLicensePlate,
			Strategy:    "部分遮掩",
			Example:     "京A***89",
			Description: "保留省份和字母，数字部分用*替代",
		},
		{
			Category:    CategoryIPAddress,
			Strategy:    "末段置零",
			Example:     "192.168.1.0",
			Description: "将最后一段替换为0",
		},
	}
}

// GetReport 获取指定报告.
func (m *ScannerManager) GetReport(id string) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report %q not found", id)
	}
	return report, nil
}

// ListReports 列出所有报告.
func (m *ScannerManager) ListReports() []*ComplianceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*ComplianceReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}
	return reports
}

// GetPatterns 获取所有 PII 匹配模式.
func (m *ScannerManager) GetPatterns() []*PIIPattern {
	patterns := make([]*PIIPattern, len(m.patterns))
	copy(patterns, m.patterns)
	return patterns
}
