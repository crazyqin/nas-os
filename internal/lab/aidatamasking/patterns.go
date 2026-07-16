package aidatamasking

import "regexp"

// SensitivePattern 敏感数据模式.
type SensitivePattern struct {
	DataType SensitiveDataType
	Pattern  *regexp.Regexp
	Name     string
}

// compiledPatterns 编译后的模式列表.
var compiledPatterns []*SensitivePattern

func init() {
	compiledPatterns = compilePatterns()
}

// compilePatterns 编译所有敏感数据模式.
func compilePatterns() []*SensitivePattern {
	patterns := []struct {
		dataType SensitiveDataType
		pattern  string
		name     string
	}{
		// 身份证号：18位，最后一位可能是X
		{
			DataTypeIDCard,
			`[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`,
			"中国居民身份证号",
		},
		// 手机号：1开头11位
		{
			DataTypePhone,
			`1[3-9]\d{9}`,
			"中国大陆手机号",
		},
		// 银行卡号：16-19位数字
		{
			DataTypeBankCard,
			`[1-9]\d{15,18}`,
			"银行卡号",
		},
		// 邮箱
		{
			DataTypeEmail,
			`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
			"电子邮箱",
		},
		// IPv4地址
		{
			DataTypeIPAddress,
			`(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)`,
			"IPv4地址",
		},
		// 车牌号
		{
			DataTypeLicensePlate,
			`[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤川青藏琼宁][A-HJ-NP-Z][A-HJ-NP-Z0-9]{4,5}[A-HJ-NP-Z0-9挂学警港澳]`,
			"中国车牌号",
		},
		// 护照号
		{
			DataTypePassport,
			`[A-Z]\d{8}`,
			"护照号码",
		},
	}

	var result []*SensitivePattern
	for _, p := range patterns {
		compiled, err := regexp.Compile(p.pattern)
		if err != nil {
			continue
		}
		result = append(result, &SensitivePattern{
			DataType: p.dataType,
			Pattern:  compiled,
			Name:     p.name,
		})
	}
	return result
}

// GetDefaultPatterns 获取默认的敏感数据模式.
func GetDefaultPatterns() []*SensitivePattern {
	return compiledPatterns
}

// GetPatternsByType 获取指定类型的模式.
func GetPatternsByType(dataType SensitiveDataType) []*SensitivePattern {
	var result []*SensitivePattern
	for _, p := range compiledPatterns {
		if p.DataType == dataType {
			result = append(result, p)
		}
	}
	return result
}

// AddCustomPattern 添加自定义模式.
func AddCustomPattern(dataType SensitiveDataType, pattern string, name string) (*SensitivePattern, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	sp := &SensitivePattern{
		DataType: dataType,
		Pattern:  compiled,
		Name:     name,
	}
	compiledPatterns = append(compiledPatterns, sp)
	return sp, nil
}

// GetPatternStats 获取模式统计信息.
func GetPatternStats() map[SensitiveDataType]int {
	stats := make(map[SensitiveDataType]int)
	for _, p := range compiledPatterns {
		stats[p.DataType]++
	}
	return stats
}
