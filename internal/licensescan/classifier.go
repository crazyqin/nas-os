package licensescan

import "strings"

// knownLicenseCategories 已知许可证分类映射.
var knownLicenseCategories = map[string]Category{
	// 宽松许可证
	"MIT":              CategoryPermissive,
	"MIT-0":            CategoryPermissive,
	"Apache-2.0":       CategoryPermissive,
	"BSD-2-Clause":     CategoryPermissive,
	"BSD-3-Clause":     CategoryPermissive,
	"ISC":              CategoryPermissive,
	"Zlib":             CategoryPermissive,
	"Unlicense":        CategoryPermissive,
	"CC0-1.0":          CategoryPermissive,
	"0BSD":             CategoryPermissive,
	"BlueOak-1.0.0":    CategoryPermissive,
	"PostgreSQL":       CategoryPermissive,
	"PSF-2.0":          CategoryPermissive,
	"Python-2.0":       CategoryPermissive,
	"WTFPL":            CategoryPermissive,
	"CC-BY-4.0":        CategoryPermissive,
	"CC-BY-3.0":        CategoryPermissive,

	// 弱传染许可证
	"LGPL-2.0":         CategoryWeakCopyleft,
	"LGPL-2.1":         CategoryWeakCopyleft,
	"LGPL-3.0":         CategoryWeakCopyleft,
	"LGPL-2.0-only":    CategoryWeakCopyleft,
	"LGPL-2.1-only":    CategoryWeakCopyleft,
	"LGPL-3.0-only":    CategoryWeakCopyleft,
	"LGPL-2.0-or-later": CategoryWeakCopyleft,
	"LGPL-2.1-or-later": CategoryWeakCopyleft,
	"LGPL-3.0-or-later": CategoryWeakCopyleft,
	"MPL-2.0":          CategoryWeakCopyleft,
	"EPL-1.0":          CategoryWeakCopyleft,
	"EPL-2.0":          CategoryWeakCopyleft,
	"CDDL-1.0":         CategoryWeakCopyleft,
	"CDDL-1.1":         CategoryWeakCopyleft,

	// 强传染许可证
	"GPL-2.0":          CategoryStrongCopyleft,
	"GPL-3.0":          CategoryStrongCopyleft,
	"GPL-2.0-only":     CategoryStrongCopyleft,
	"GPL-3.0-only":     CategoryStrongCopyleft,
	"GPL-2.0-or-later": CategoryStrongCopyleft,
	"GPL-3.0-or-later": CategoryStrongCopyleft,
	"AGPL-3.0":         CategoryStrongCopyleft,
	"AGPL-3.0-only":    CategoryStrongCopyleft,
	"AGPL-3.0-or-later": CategoryStrongCopyleft,
	"SSPL-1.0":         CategoryStrongCopyleft,
	"EUPL-1.1":         CategoryStrongCopyleft,
	"EUPL-1.2":         CategoryStrongCopyleft,
	"OSL-3.0":          CategoryStrongCopyleft,
}

// ClassifyLicense 根据许可证SPDX标识或名称分类.
// 返回许可证类别，未知返回 CategoryUnknown.
func ClassifyLicense(license string) Category {
	if license == "" {
		return CategoryUnknown
	}

	// 精确匹配SPDX标识
	if cat, ok := knownLicenseCategories[license]; ok {
		return cat
	}

	// 大小写不敏感匹配
	upper := strings.ToUpper(license)
	for spdx, cat := range knownLicenseCategories {
		if strings.ToUpper(spdx) == upper {
			return cat
		}
	}

	// 模糊匹配
	lower := strings.ToLower(license)
	if strings.Contains(lower, "gpl") || strings.Contains(lower, "agpl") {
		if strings.Contains(lower, "lgpl") {
			return CategoryWeakCopyleft
		}
		return CategoryStrongCopyleft
	}
	if strings.Contains(lower, "mit") {
		return CategoryPermissive
	}
	if strings.Contains(lower, "bsd") {
		return CategoryPermissive
	}
	if strings.Contains(lower, "apache") {
		return CategoryPermissive
	}
	if strings.Contains(lower, "lgpl") {
		return CategoryWeakCopyleft
	}
	if strings.Contains(lower, "mpl") || strings.Contains(lower, "mozilla") {
		return CategoryWeakCopyleft
	}

	return CategoryCustom
}

// GetComplianceStatus 根据许可证类别和策略判断合规状态.
// 先检查白名单/黑名单/灰名单，再根据类别做默认判断.
func GetComplianceStatus(license string, policy *Policy) Compliance {
	if policy == nil {
		return defaultCompliance(ClassifyLicense(license))
	}

	normalizedLicense := strings.TrimSpace(license)

	// 检查黑名单（最高优先级）
	for _, l := range policy.Blacklist {
		if matchLicense(normalizedLicense, l) {
			return ComplianceDenied
		}
	}

	// 检查白名单
	for _, l := range policy.Whitelist {
		if matchLicense(normalizedLicense, l) {
			return ComplianceAllowed
		}
	}

	// 检查灰名单
	for _, l := range policy.Graylist {
		if matchLicense(normalizedLicense, l) {
			return ComplianceReview
		}
	}

	// 根据DefaultList处理未匹配的
	switch policy.DefaultList {
	case ListWhitelist:
		return ComplianceAllowed
	case ListBlacklist:
		return ComplianceDenied
	case ListGraylist:
		return ComplianceReview
	default:
		return defaultCompliance(ClassifyLicense(license))
	}
}

// defaultCompliance 根据许可证类别返回默认合规状态.
func defaultCompliance(cat Category) Compliance {
	switch cat {
	case CategoryPermissive:
		return ComplianceAllowed
	case CategoryWeakCopyleft:
		return ComplianceReview
	case CategoryStrongCopyleft:
		return ComplianceDenied
	default:
		return ComplianceUnknown
	}
}

// matchLicense 匹配许可证名称（大小写不敏感）.
func matchLicense(name, pattern string) bool {
	return strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(pattern))
}
