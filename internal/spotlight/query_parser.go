package spotlight

import (
	"fmt"
	"strings"
	"time"
)

// QueryParser 查询解析器
// 支持 Spotlight 语法、布尔查询和过滤条件
type QueryParser struct {
	// 属性映射（Spotlight 属性名 -> 内部字段名）
	attributeMap map[string]string
}

// ParsedQuery 解析后的查询
type ParsedQuery struct {
	Raw       string     `json:"raw"`
	Terms     []string   `json:"terms"`
	Keywords  []string   `json:"keywords"`
	Paths     []string   `json:"paths,omitempty"`
	FileTypes []string   `json:"fileTypes,omitempty"`
	SizeRange *SizeRange `json:"sizeRange,omitempty"`
	DateRange *DateRange `json:"dateRange,omitempty"`
	Operators []Operator `json:"operators,omitempty"`
}

// SizeRange 大小范围
type SizeRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// DateRange 日期范围
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Operator 布尔运算符
type Operator struct {
	Type  string `json:"type"` // AND, OR, NOT
	Left  string `json:"left"`
	Right string `json:"right"`
}

// NewQueryParser 创建查询解析器
func NewQueryParser() *QueryParser {
	return &QueryParser{
		attributeMap: map[string]string{
			"kMDItemDisplayName":         "name",
			"kMDItemPath":                "path",
			"kMDItemFSSize":              "size",
			"kMDItemFSCreationDate":      "createTime",
			"kMDItemFSContentChangeDate": "modTime",
			"kMDItemContentType":         "mimeType",
			"kMDItemKind":                "type",
			"kMDItemTextContent":         "content",
			"kMDItemKeywords":            "keywords",
		},
	}
}

// Parse 解析查询字符串
func (p *QueryParser) Parse(query string) (*ParsedQuery, error) {
	if query == "" {
		return nil, fmt.Errorf("查询不能为空")
	}

	parsed := &ParsedQuery{
		Raw:   query,
		Terms: make([]string, 0),
	}

	// 预处理查询
	query = strings.TrimSpace(query)

	// 解析属性查询（Spotlight 语法）
	query = p.parseAttributes(query, parsed)

	// 解析布尔运算符
	query = p.parseOperators(query, parsed)

	// 解析文件类型过滤
	query = p.parseFileTypes(query, parsed)

	// 解析大小范围
	query = p.parseSizeRange(query, parsed)

	// 解析日期范围
	query = p.parseDateRange(query, parsed)

	// 解析路径限制
	query = p.parsePaths(query, parsed)

	// 剩余部分作为搜索词
	terms := strings.Fields(query)
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term != "" {
			parsed.Terms = append(parsed.Terms, term)
			parsed.Keywords = append(parsed.Keywords, term)
		}
	}

	return parsed, nil
}

// parseAttributes 解析 Spotlight 属性查询
// 格式: kMDItemDisplayName == "value"
func (p *QueryParser) parseAttributes(query string, parsed *ParsedQuery) string {
	// 查找属性查询模式
	for attr, field := range p.attributeMap {
		pattern := attr + " =="
		if strings.Contains(query, pattern) {
			// 提取值
			start := strings.Index(query, pattern)
			if start < 0 {
				continue
			}
			valueStart := start + len(pattern)
			value := extractQuotedValue(query[valueStart:])

			if value != "" {
				// 添加到查询
				switch field {
				case "path":
					parsed.Paths = append(parsed.Paths, value)
				case "mimeType":
					parsed.FileTypes = append(parsed.FileTypes, value)
				default:
					parsed.Terms = append(parsed.Terms, value)
				}

				// 从原始查询中移除
				query = strings.Replace(query, attr+" == "+`"`+value+`"`, "", 1)
				query = strings.Replace(query, attr+" == "+value, "", 1)
			}
		}
	}

	return strings.TrimSpace(query)
}

// parseOperators 解析布尔运算符
func (p *QueryParser) parseOperators(query string, parsed *ParsedQuery) string {
	// 处理 AND
	if strings.Contains(query, " AND ") {
		parts := strings.Split(query, " AND ")
		if len(parts) == 2 {
			parsed.Operators = append(parsed.Operators, Operator{
				Type:  "AND",
				Left:  strings.TrimSpace(parts[0]),
				Right: strings.TrimSpace(parts[1]),
			})
			return ""
		}
	}

	// 处理 OR
	if strings.Contains(query, " OR ") {
		parts := strings.Split(query, " OR ")
		if len(parts) == 2 {
			parsed.Operators = append(parsed.Operators, Operator{
				Type:  "OR",
				Left:  strings.TrimSpace(parts[0]),
				Right: strings.TrimSpace(parts[1]),
			})
			return ""
		}
	}

	// 处理 NOT
	if strings.Contains(query, " NOT ") {
		parts := strings.Split(query, " NOT ")
		if len(parts) == 2 {
			parsed.Operators = append(parsed.Operators, Operator{
				Type:  "NOT",
				Left:  strings.TrimSpace(parts[0]),
				Right: strings.TrimSpace(parts[1]),
			})
			return ""
		}
	}

	return query
}

// parseFileTypes 解析文件类型过滤
// 格式: type:pdf 或 filetype:pdf
func (p *QueryParser) parseFileTypes(query string, parsed *ParsedQuery) string {
	terms := strings.Fields(query)
	remaining := make([]string, 0)

	for _, term := range terms {
		lower := strings.ToLower(term)
		if strings.HasPrefix(lower, "type:") || strings.HasPrefix(lower, "filetype:") {
			ext := strings.TrimPrefix(lower, "type:")
			ext = strings.TrimPrefix(ext, "filetype:")
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			parsed.FileTypes = append(parsed.FileTypes, ext)
		} else {
			remaining = append(remaining, term)
		}
	}

	return strings.Join(remaining, " ")
}

// parseSizeRange 解析大小范围
// 格式: size:>1MB 或 size:<100KB
func (p *QueryParser) parseSizeRange(query string, parsed *ParsedQuery) string {
	terms := strings.Fields(query)
	remaining := make([]string, 0)

	for _, term := range terms {
		lower := strings.ToLower(term)
		if strings.HasPrefix(lower, "size:") {
			sizeStr := strings.TrimPrefix(lower, "size:")
			size := parseSize(sizeStr)

			if strings.HasPrefix(sizeStr, ">") {
				if parsed.SizeRange == nil {
					parsed.SizeRange = &SizeRange{}
				}
				parsed.SizeRange.Min = size
			} else if strings.HasPrefix(sizeStr, "<") {
				if parsed.SizeRange == nil {
					parsed.SizeRange = &SizeRange{}
				}
				parsed.SizeRange.Max = size
			} else {
				// 精确大小，设置为范围
				parsed.SizeRange = &SizeRange{Min: size, Max: size}
			}
		} else {
			remaining = append(remaining, term)
		}
	}

	return strings.Join(remaining, " ")
}

// parseDateRange 解析日期范围
// 格式: date:>2024-01-01 或 date:<2024-12-31
func (p *QueryParser) parseDateRange(query string, parsed *ParsedQuery) string {
	terms := strings.Fields(query)
	remaining := make([]string, 0)

	for _, term := range terms {
		lower := strings.ToLower(term)
		if strings.HasPrefix(lower, "date:") {
			dateStr := strings.TrimPrefix(lower, "date:")
			date, err := parseDate(dateStr)
			if err != nil {
				remaining = append(remaining, term)
				continue
			}

			if parsed.DateRange == nil {
				parsed.DateRange = &DateRange{}
			}

			if strings.HasPrefix(dateStr, ">") {
				parsed.DateRange.From = date
			} else if strings.HasPrefix(dateStr, "<") {
				parsed.DateRange.To = date
			} else {
				// 精确日期，设置为整天
				parsed.DateRange.From = date
				parsed.DateRange.To = date.Add(24 * time.Hour)
			}
		} else {
			remaining = append(remaining, term)
		}
	}

	return strings.Join(remaining, " ")
}

// parsePaths 解析路径限制
// 格式: path:/Volumes/share 或 in:/path
func (p *QueryParser) parsePaths(query string, parsed *ParsedQuery) string {
	terms := strings.Fields(query)
	remaining := make([]string, 0)

	for _, term := range terms {
		lower := strings.ToLower(term)
		if strings.HasPrefix(lower, "path:") {
			path := strings.TrimPrefix(term, "path:")
			parsed.Paths = append(parsed.Paths, path)
		} else if strings.HasPrefix(lower, "in:") {
			path := strings.TrimPrefix(term, "in:")
			parsed.Paths = append(parsed.Paths, path)
		} else {
			remaining = append(remaining, term)
		}
	}

	return strings.Join(remaining, " ")
}

// extractQuotedValue 提取引号内的值
func extractQuotedValue(s string) string {
	s = strings.TrimSpace(s)

	// 查找引号
	start := strings.Index(s, `"`)
	if start < 0 {
		// 没有引号，取第一个空格前的值
		end := strings.Index(s, " ")
		if end < 0 {
			return s
		}
		return s[:end]
	}

	end := strings.Index(s[start+1:], `"`)
	if end < 0 {
		return s[start+1:]
	}

	return s[start+1 : start+1+end]
}

// parseSize 解析大小字符串
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, ">")
	s = strings.TrimPrefix(s, "<")
	s = strings.ToLower(s)

	multiplier := int64(1)
	if strings.HasSuffix(s, "kb") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "kb")
	} else if strings.HasSuffix(s, "mb") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "mb")
	} else if strings.HasSuffix(s, "gb") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "gb")
	} else if strings.HasSuffix(s, "b") {
		s = strings.TrimSuffix(s, "b")
	}

	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		}
	}
	return result * multiplier
}

// parseDate 解析日期字符串
func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, ">")
	s = strings.TrimPrefix(s, "<")

	// 尝试多种日期格式
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析日期: %s", s)
}
