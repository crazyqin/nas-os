// Package search provides full-text search capabilities using Bleve.
// This file implements query parsing and building for advanced search operations.
package search

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// ================== 查询解析器 ==================

// QueryParser 查询解析器
// 支持高级搜索语法，类似 TrueNAS Electric Eel 的搜索功能.
type QueryParser struct {
	// 时间格式
	timeFormats []string
	// 默认搜索字段
	defaultField string
	// 模糊搜索距离
	fuzziness int
	// 是否启用前缀匹配
	prefixEnabled bool
}

// NewQueryParser 创建查询解析器.
func NewQueryParser() *QueryParser {
	return &QueryParser{
		timeFormats: []string{
			"2006-01-02",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006/01/02",
			time.RFC3339,
		},
		defaultField:   "content",
		fuzziness:      1,
		prefixEnabled:  true,
	}
}

// ParsedQuery 解析后的查询.
type ParsedQuery struct {
	// 原始查询字符串
	Raw string `json:"raw"`
	// 搜索关键词
	Terms []string `json:"terms"`
	// 必须包含的词（+前缀）
	MustTerms []string `json:"mustTerms"`
	// 排除词（-前缀）
	NotTerms []string `json:"notTerms"`
	// 字段过滤
	FieldFilters map[string][]string `json:"fieldFilters"`
	// 文件类型过滤
	FileTypes []string `json:"fileTypes"`
	// 路径过滤
	Paths []string `json:"paths"`
	// 大小过滤
	SizeRange *SizeRange `json:"sizeRange,omitempty"`
	// 时间过滤
	DateRange *DateRange `json:"dateRange,omitempty"`
	// 是否正则表达式
	IsRegex bool `json:"isRegex"`
	// 正则表达式模式
	RegexPattern string `json:"regexPattern,omitempty"`
	// 是否精确匹配
	ExactMatch bool `json:"exactMatch"`
	// 布尔操作符
	Operator QueryOperator `json:"operator"`
}

// QueryOperator 查询操作符.
type QueryOperator string

const (
	// OperatorAnd AND 操作符.
	OperatorAnd QueryOperator = "and"
	// OperatorOr OR 操作符.
	OperatorOr QueryOperator = "or"
)

// SizeRange 大小范围.
type SizeRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

// DateRange 时间范围.
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Parse 解析查询字符串.
// 支持以下语法：
//   - word: 普通搜索
//   - "exact phrase": 精确匹配
//   - +must: 必须包含
//   - -not: 排除词
//   - field:value: 字段过滤
//   - type:ext: 文件类型
//   - path:/path/to: 路径过滤
//   - size:>10MB: 大小过滤
//   - date:>2024-01-01: 时间过滤
//   - /regex/: 正则表达式
func (qp *QueryParser) Parse(q string) (*ParsedQuery, error) {
	pq := &ParsedQuery{
		Raw:          q,
		FieldFilters: make(map[string][]string),
		Operator:     OperatorOr,
	}

	// 空查询
	q = strings.TrimSpace(q)
	if q == "" {
		return pq, nil
	}

	// 检查正则表达式
	if strings.HasPrefix(q, "/") && strings.HasSuffix(q, "/") {
		pq.IsRegex = true
		pq.RegexPattern = strings.Trim(q, "/")
		return pq, nil
	}

	// 检查精确匹配
	if strings.HasPrefix(q, "\"") && strings.HasSuffix(q, "\"") {
		pq.ExactMatch = true
		pq.Terms = []string{strings.Trim(q, "\"")}
		return pq, nil
	}

	// 解析查询词
	terms := qp.tokenize(q)
	for _, term := range terms {
		if err := qp.parseTerm(term, pq); err != nil {
			return nil, err
		}
	}

	return pq, nil
}

// tokenize 分词.
func (qp *QueryParser) tokenize(q string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	escape := false

	for _, r := range q {
		if escape {
			current.WriteRune(r)
			escape = false
			continue
		}

		switch r {
		case '\\':
			escape = true
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseTerm 解析单个查询词.
func (qp *QueryParser) parseTerm(term string, pq *ParsedQuery) error {
	// 检查前缀
	switch {
	case strings.HasPrefix(term, "+"):
		// 必须包含
		pq.MustTerms = append(pq.MustTerms, strings.TrimPrefix(term, "+"))

	case strings.HasPrefix(term, "-"):
		// 排除
		pq.NotTerms = append(pq.NotTerms, strings.TrimPrefix(term, "-"))

	case strings.Contains(term, ":"):
		// 字段过滤
		parts := strings.SplitN(term, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("无效的字段过滤: %s", term)
		}
		field := strings.ToLower(parts[0])
		value := parts[1]

		switch field {
		case "type", "ext":
			// 文件类型
			if !strings.HasPrefix(value, ".") {
				value = "." + value
			}
			pq.FileTypes = append(pq.FileTypes, value)

		case "path", "dir", "folder":
			// 路径过滤
			pq.Paths = append(pq.Paths, value)

		case "size":
			// 大小过滤
			sizeRange, err := qp.parseSizeFilter(value)
			if err != nil {
				return err
			}
			pq.SizeRange = sizeRange

		case "date", "time", "modified", "mtime":
			// 时间过滤
			dateRange, err := qp.parseDateFilter(value)
			if err != nil {
				return err
			}
			pq.DateRange = dateRange

		case "name", "filename":
			// 文件名过滤
			pq.FieldFilters["name"] = append(pq.FieldFilters["name"], value)

		case "content", "text":
			// 内容过滤
			pq.FieldFilters["content"] = append(pq.FieldFilters["content"], value)

		case "mime", "mimetype":
			// MIME 类型过滤
			pq.FieldFilters["mimeType"] = append(pq.FieldFilters["mimeType"], value)

		default:
			// 通用字段过滤
			pq.FieldFilters[field] = append(pq.FieldFilters[field], value)
		}

	default:
		// 普通搜索词
		pq.Terms = append(pq.Terms, term)
	}

	return nil
}

// parseSizeFilter 解析大小过滤.
func (qp *QueryParser) parseSizeFilter(value string) (*SizeRange, error) {
	// 支持的格式: >10MB, <100KB, 10MB-100MB, 10MB..100MB
	sr := &SizeRange{}

	// 解析大小单位
	parseSize := func(s string) (int64, error) {
		s = strings.TrimSpace(strings.ToUpper(s))
		multiplier := int64(1)

		switch {
		case strings.HasSuffix(s, "GB"):
			multiplier = 1024 * 1024 * 1024
			s = strings.TrimSuffix(s, "GB")
		case strings.HasSuffix(s, "MB"):
			multiplier = 1024 * 1024
			s = strings.TrimSuffix(s, "MB")
		case strings.HasSuffix(s, "KB"):
			multiplier = 1024
			s = strings.TrimSuffix(s, "KB")
		case strings.HasSuffix(s, "B"):
			s = strings.TrimSuffix(s, "B")
		}

		val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return 0, err
		}
		return val * multiplier, nil
	}

	// 检查操作符
	switch {
	case strings.HasPrefix(value, ">="):
		size, err := parseSize(strings.TrimPrefix(value, ">="))
		if err != nil {
			return nil, err
		}
		sr.Min = size

	case strings.HasPrefix(value, ">"):
		size, err := parseSize(strings.TrimPrefix(value, ">"))
		if err != nil {
			return nil, err
		}
		sr.Min = size + 1

	case strings.HasPrefix(value, "<="):
		size, err := parseSize(strings.TrimPrefix(value, "<="))
		if err != nil {
			return nil, err
		}
		sr.Max = size

	case strings.HasPrefix(value, "<"):
		size, err := parseSize(strings.TrimPrefix(value, "<"))
		if err != nil {
			return nil, err
		}
		sr.Max = size - 1

	case strings.Contains(value, "-") || strings.Contains(value, ".."):
		// 范围
		sep := "-"
		if strings.Contains(value, "..") {
			sep = ".."
		}
		parts := strings.Split(value, sep)
		if len(parts) != 2 {
			return nil, fmt.Errorf("无效的大小范围: %s", value)
		}
		minSize, err := parseSize(parts[0])
		if err != nil {
			return nil, err
		}
		maxSize, err := parseSize(parts[1])
		if err != nil {
			return nil, err
		}
		sr.Min = minSize
		sr.Max = maxSize

	default:
		// 精确大小
		size, err := parseSize(value)
		if err != nil {
			return nil, err
		}
		sr.Min = size
		sr.Max = size
	}

	return sr, nil
}

// parseDateFilter 解析时间过滤.
func (qp *QueryParser) parseDateFilter(value string) (*DateRange, error) {
	dr := &DateRange{}

	// 解析时间
	parseTime := func(s string) (time.Time, error) {
		s = strings.TrimSpace(s)
		for _, format := range qp.timeFormats {
			t, err := time.Parse(format, s)
			if err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("无法解析时间: %s", s)
	}

	// 支持相对时间
	parseRelative := func(s string) (time.Time, error) {
		s = strings.ToLower(s)
		now := time.Now()

		switch {
		case s == "today":
			return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
		case s == "yesterday":
			yesterday := now.AddDate(0, 0, -1)
			return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location()), nil
		case s == "thisweek":
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			weekStart := now.AddDate(0, 0, -weekday+1)
			return time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location()), nil
		case s == "thismonth":
			return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), nil
		case s == "thisyear":
			return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()), nil
		case strings.HasSuffix(s, "d"):
			days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
			if err != nil {
				return time.Time{}, err
			}
			return now.AddDate(0, 0, -days), nil
		case strings.HasSuffix(s, "w"):
			weeks, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
			if err != nil {
				return time.Time{}, err
			}
			return now.AddDate(0, 0, -weeks*7), nil
		case strings.HasSuffix(s, "m"):
			months, err := strconv.Atoi(strings.TrimSuffix(s, "m"))
			if err != nil {
				return time.Time{}, err
			}
			return now.AddDate(0, -months, 0), nil
		case strings.HasSuffix(s, "y"):
			years, err := strconv.Atoi(strings.TrimSuffix(s, "y"))
			if err != nil {
				return time.Time{}, err
			}
			return now.AddDate(-years, 0, 0), nil
		}
		return time.Time{}, fmt.Errorf("无法解析相对时间: %s", s)
	}

	// 检查操作符
	switch {
	case strings.HasPrefix(value, ">="):
		t, err := parseTime(strings.TrimPrefix(value, ">="))
		if err != nil {
			t, err = parseRelative(strings.TrimPrefix(value, ">="))
			if err != nil {
				return nil, err
			}
		}
		dr.From = t

	case strings.HasPrefix(value, ">"):
		t, err := parseTime(strings.TrimPrefix(value, ">"))
		if err != nil {
			t, err = parseRelative(strings.TrimPrefix(value, ">"))
			if err != nil {
				return nil, err
			}
		}
		dr.From = t.Add(1 * time.Second)

	case strings.HasPrefix(value, "<="):
		t, err := parseTime(strings.TrimPrefix(value, "<="))
		if err != nil {
			t, err = parseRelative(strings.TrimPrefix(value, "<="))
			if err != nil {
				return nil, err
			}
		}
		dr.To = t

	case strings.HasPrefix(value, "<"):
		t, err := parseTime(strings.TrimPrefix(value, "<"))
		if err != nil {
			t, err = parseRelative(strings.TrimPrefix(value, "<"))
			if err != nil {
				return nil, err
			}
		}
		dr.To = t.Add(-1 * time.Second)

	case strings.Contains(value, "-") || strings.Contains(value, ".."):
		// 范围
		sep := "-"
		if strings.Contains(value, "..") {
			sep = ".."
		}
		parts := strings.Split(value, sep)
		if len(parts) != 2 {
			return nil, fmt.Errorf("无效的时间范围: %s", value)
		}
		fromTime, err := parseTime(parts[0])
		if err != nil {
			fromTime, err = parseRelative(parts[0])
			if err != nil {
				return nil, err
			}
		}
		toTime, err := parseTime(parts[1])
		if err != nil {
			toTime, err = parseRelative(parts[1])
			if err != nil {
				return nil, err
			}
		}
		dr.From = fromTime
		dr.To = toTime

	default:
		// 相对时间或精确时间
		t, err := parseTime(value)
		if err != nil {
			t, err = parseRelative(value)
			if err != nil {
				return nil, err
			}
		}
		dr.From = t
		dr.To = t.Add(24 * time.Hour)
	}

	return dr, nil
}

// BuildQuery 构建查询.
func (qp *QueryParser) BuildQuery(pq *ParsedQuery) query.Query {
	var queries []query.Query

	// 正则表达式查询
	if pq.IsRegex {
		regexQuery := bleve.NewRegexpQuery(pq.RegexPattern)
		regexQuery.SetField(qp.defaultField)
		return regexQuery
	}

	// 精确匹配查询
	if pq.ExactMatch && len(pq.Terms) > 0 {
		phraseQuery := bleve.NewPhraseQuery(strings.Fields(pq.Terms[0]), qp.defaultField)
		return phraseQuery
	}

	// 普通搜索词
	for _, term := range pq.Terms {
		matchQuery := bleve.NewMatchQuery(term)
		matchQuery.SetFuzziness(qp.fuzziness)
		if qp.prefixEnabled {
			matchQuery.SetPrefix(3)
		}
		queries = append(queries, matchQuery)
	}

	// 必须包含的词
	var mustQueries []query.Query
	for _, term := range pq.MustTerms {
		matchQuery := bleve.NewMatchQuery(term)
		matchQuery.SetFuzziness(qp.fuzziness)
		mustQueries = append(mustQueries, matchQuery)
	}

	// 排除词
	var notQueries []query.Query
	for _, term := range pq.NotTerms {
		matchQuery := bleve.NewMatchQuery(term)
		notQueries = append(notQueries, matchQuery)
	}

	// 字段过滤
	for field, values := range pq.FieldFilters {
		for _, value := range values {
			termQuery := bleve.NewTermQuery(value)
			termQuery.SetField(field)
			queries = append(queries, termQuery)
		}
	}

	// 文件类型过滤
	if len(pq.FileTypes) > 0 {
		var typeQueries []query.Query
		for _, ext := range pq.FileTypes {
			termQuery := bleve.NewTermQuery(ext)
			termQuery.SetField("ext")
			typeQueries = append(typeQueries, termQuery)
		}
		if len(typeQueries) == 1 {
			queries = append(queries, typeQueries[0])
		} else {
			queries = append(queries, bleve.NewDisjunctionQuery(typeQueries...))
		}
	}

	// 路径过滤
	if len(pq.Paths) > 0 {
		var pathQueries []query.Query
		for _, path := range pq.Paths {
			prefixQuery := bleve.NewPrefixQuery(path)
			prefixQuery.SetField("path")
			pathQueries = append(pathQueries, prefixQuery)
		}
		if len(pathQueries) == 1 {
			queries = append(queries, pathQueries[0])
		} else {
			queries = append(queries, bleve.NewDisjunctionQuery(pathQueries...))
		}
	}

	// 大小过滤
	if pq.SizeRange != nil {
		min := float64(pq.SizeRange.Min)
		max := float64(pq.SizeRange.Max)
		rangeQuery := bleve.NewNumericRangeQuery(&min, &max)
		rangeQuery.SetField("size")
		queries = append(queries, rangeQuery)
	}

	// 时间过滤
	if pq.DateRange != nil {
		rangeQuery := bleve.NewDateRangeQuery(pq.DateRange.From, pq.DateRange.To)
		rangeQuery.SetField("modTime")
		queries = append(queries, rangeQuery)
	}

	// 组合查询
	if len(mustQueries) > 0 {
		// 必须包含的查询
		mustConj := bleve.NewConjunctionQuery(mustQueries...)
		queries = append(queries, mustConj)
	}

	if len(notQueries) > 0 {
		// 排除查询 - 使用 BooleanQuery
		boolQuery := bleve.NewBooleanQuery()
		boolQuery.AddMustNot(notQueries...)
		queries = append(queries, boolQuery)
	}

	// 最终组合
	if len(queries) == 0 {
		// 返回匹配所有的查询
		return bleve.NewMatchAllQuery()
	}

	if len(queries) == 1 {
		return queries[0]
	}

	// 根据操作符组合
	if pq.Operator == OperatorAnd {
		return bleve.NewConjunctionQuery(queries...)
	}
	return bleve.NewDisjunctionQuery(queries...)
}

// ================== 高级查询构建器 ==================

// QueryBuilder 查询构建器
// 提供流畅的 API 构建复杂查询.
type QueryBuilder struct {
	parser *QueryParser
	query  *ParsedQuery
}

// NewQueryBuilder 创建查询构建器.
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		parser: NewQueryParser(),
		query: &ParsedQuery{
			FieldFilters: make(map[string][]string),
			Operator:     OperatorAnd,
		},
	}
}

// Query 添加搜索词.
func (qb *QueryBuilder) Query(q string) *QueryBuilder {
	parsed, err := qb.parser.Parse(q)
	if err != nil {
		return qb
	}
	qb.query.Terms = append(qb.query.Terms, parsed.Terms...)
	qb.query.MustTerms = append(qb.query.MustTerms, parsed.MustTerms...)
	qb.query.NotTerms = append(qb.query.NotTerms, parsed.NotTerms...)
	return qb
}

// Must 必须包含.
func (qb *QueryBuilder) Must(terms ...string) *QueryBuilder {
	qb.query.MustTerms = append(qb.query.MustTerms, terms...)
	return qb
}

// Not 排除词.
func (qb *QueryBuilder) Not(terms ...string) *QueryBuilder {
	qb.query.NotTerms = append(qb.query.NotTerms, terms...)
	return qb
}

// Type 文件类型过滤.
func (qb *QueryBuilder) Type(exts ...string) *QueryBuilder {
	for _, ext := range exts {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		qb.query.FileTypes = append(qb.query.FileTypes, ext)
	}
	return qb
}

// Path 路径过滤.
func (qb *QueryBuilder) Path(paths ...string) *QueryBuilder {
	qb.query.Paths = append(qb.query.Paths, paths...)
	return qb
}

// SizeRange 大小范围过滤.
func (qb *QueryBuilder) SizeRange(min, max int64) *QueryBuilder {
	qb.query.SizeRange = &SizeRange{Min: min, Max: max}
	return qb
}

// SizeMin 最小大小.
func (qb *QueryBuilder) SizeMin(min int64) *QueryBuilder {
	if qb.query.SizeRange == nil {
		qb.query.SizeRange = &SizeRange{}
	}
	qb.query.SizeRange.Min = min
	return qb
}

// SizeMax 最大大小.
func (qb *QueryBuilder) SizeMax(max int64) *QueryBuilder {
	if qb.query.SizeRange == nil {
		qb.query.SizeRange = &SizeRange{}
	}
	qb.query.SizeRange.Max = max
	return qb
}

// DateRange 时间范围过滤.
func (qb *QueryBuilder) DateRange(from, to time.Time) *QueryBuilder {
	qb.query.DateRange = &DateRange{From: from, To: to}
	return qb
}

// DateAfter 指定时间之后.
func (qb *QueryBuilder) DateAfter(t time.Time) *QueryBuilder {
	if qb.query.DateRange == nil {
		qb.query.DateRange = &DateRange{}
	}
	qb.query.DateRange.From = t
	return qb
}

// DateBefore 指定时间之前.
func (qb *QueryBuilder) DateBefore(t time.Time) *QueryBuilder {
	if qb.query.DateRange == nil {
		qb.query.DateRange = &DateRange{}
	}
	qb.query.DateRange.To = t
	return qb
}

// Field 字段过滤.
func (qb *QueryBuilder) Field(name, value string) *QueryBuilder {
	qb.query.FieldFilters[name] = append(qb.query.FieldFilters[name], value)
	return qb
}

// Name 文件名过滤.
func (qb *QueryBuilder) Name(name string) *QueryBuilder {
	return qb.Field("name", name)
}

// Content 内容过滤.
func (qb *QueryBuilder) Content(content string) *QueryBuilder {
	return qb.Field("content", content)
}

// MimeType MIME 类型过滤.
func (qb *QueryBuilder) MimeType(mimeType string) *QueryBuilder {
	return qb.Field("mimeType", mimeType)
}

// Regex 正则表达式查询.
func (qb *QueryBuilder) Regex(pattern string) *QueryBuilder {
	qb.query.IsRegex = true
	qb.query.RegexPattern = pattern
	return qb
}

// Exact 精确匹配.
func (qb *QueryBuilder) Exact(phrase string) *QueryBuilder {
	qb.query.ExactMatch = true
	qb.query.Terms = []string{phrase}
	return qb
}

// And 使用 AND 操作符.
func (qb *QueryBuilder) And() *QueryBuilder {
	qb.query.Operator = OperatorAnd
	return qb
}

// Or 使用 OR 操作符.
func (qb *QueryBuilder) Or() *QueryBuilder {
	qb.query.Operator = OperatorOr
	return qb
}

// Build 构建查询.
func (qb *QueryBuilder) Build() query.Query {
	return qb.parser.BuildQuery(qb.query)
}

// BuildParsed 构建解析后的查询.
func (qb *QueryBuilder) BuildParsed() *ParsedQuery {
	return qb.query
}

// ================== 快捷查询 ==================

// QuickSearch 快速搜索
// 简化的搜索接口，支持常用查询模式.
func QuickSearch(engine *Engine, q string, limit int) (*Response, error) {
	parser := NewQueryParser()
	parsed, err := parser.Parse(q)
	if err != nil {
		return nil, err
	}

	// 构建 Request
	req := Request{
		Query: strings.Join(parsed.Terms, " "),
		Limit: limit,
	}

	// 设置路径过滤
	if len(parsed.Paths) > 0 {
		req.Paths = parsed.Paths
	}

	// 设置文件类型
	if len(parsed.FileTypes) > 0 {
		req.Types = parsed.FileTypes
	}

	// 设置大小范围
	if parsed.SizeRange != nil {
		req.MinSize = parsed.SizeRange.Min
		req.MaxSize = parsed.SizeRange.Max
	}

	// 设置时间范围
	if parsed.DateRange != nil {
		req.FromDate = &parsed.DateRange.From
		req.ToDate = &parsed.DateRange.To
	}

	return engine.Search(req)
}

// SearchFiles 搜索文件
// 按文件名和内容搜索.
func SearchFiles(engine *Engine, query string, paths []string, exts []string, limit int) (*Response, error) {
	qb := NewQueryBuilder().Query(query).And()

	if len(paths) > 0 {
		qb.Path(paths...)
	}

	if len(exts) > 0 {
		qb.Type(exts...)
	}

	parsed := qb.BuildParsed()
	req := Request{
		Query: strings.Join(parsed.Terms, " "),
		Limit: limit,
	}

	if len(parsed.Paths) > 0 {
		req.Paths = parsed.Paths
	}
	if len(parsed.FileTypes) > 0 {
		req.Types = parsed.FileTypes
	}

	return engine.Search(req)
}

// SearchBySize 按大小搜索.
func SearchBySize(engine *Engine, minSize, maxSize int64, limit int) (*Response, error) {
	req := Request{
		Query:   "*",
		MinSize: minSize,
		MaxSize: maxSize,
		Limit:   limit,
	}
	return engine.Search(req)
}

// SearchByDate 按日期搜索.
func SearchByDate(engine *Engine, fromDate, toDate time.Time, limit int) (*Response, error) {
	req := Request{
		Query:    "*",
		FromDate: &fromDate,
		ToDate:   &toDate,
		Limit:    limit,
	}
	return engine.Search(req)
}

// SearchRecent 搜索最近修改的文件.
func SearchRecent(engine *Engine, days int, limit int) (*Response, error) {
	from := time.Now().AddDate(0, 0, -days)
	return SearchByDate(engine, from, time.Now(), limit)
}

// SearchLarge 搜索大文件.
func SearchLarge(engine *Engine, minSize int64, limit int) (*Response, error) {
	req := Request{
		Query:   "*",
		MinSize: minSize,
		Limit:   limit,
		SortBy:  "size",
		SortDesc: true,
	}
	return engine.Search(req)
}

// ================== 查询验证 ==================

// ValidateQuery 验证查询字符串.
func ValidateQuery(q string) error {
	if len(q) > 1000 {
		return fmt.Errorf("查询字符串过长（最大 1000 字符）")
	}

	// 检查正则表达式语法
	if strings.HasPrefix(q, "/") && strings.HasSuffix(q, "/") {
		pattern := strings.Trim(q, "/")
		_, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("无效的正则表达式: %w", err)
		}
	}

	return nil
}

// SanitizeQuery 清理查询字符串.
func SanitizeQuery(q string) string {
	// 移除控制字符
	var result strings.Builder
	for _, r := range q {
		if r >= 32 || r == '\t' || r == '\n' || r == '\r' {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// ExplainQuery 解释查询（调试用）.
func ExplainQuery(q string) string {
	parser := NewQueryParser()
	parsed, err := parser.Parse(q)
	if err != nil {
		return fmt.Sprintf("解析错误: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("查询分析:\n")
	sb.WriteString(fmt.Sprintf("  原始查询: %s\n", parsed.Raw))
	sb.WriteString(fmt.Sprintf("  操作符: %s\n", parsed.Operator))

	if len(parsed.Terms) > 0 {
		sb.WriteString(fmt.Sprintf("  搜索词: %v\n", parsed.Terms))
	}
	if len(parsed.MustTerms) > 0 {
		sb.WriteString(fmt.Sprintf("  必须包含: %v\n", parsed.MustTerms))
	}
	if len(parsed.NotTerms) > 0 {
		sb.WriteString(fmt.Sprintf("  排除词: %v\n", parsed.NotTerms))
	}
	if len(parsed.FileTypes) > 0 {
		sb.WriteString(fmt.Sprintf("  文件类型: %v\n", parsed.FileTypes))
	}
	if len(parsed.Paths) > 0 {
		sb.WriteString(fmt.Sprintf("  路径过滤: %v\n", parsed.Paths))
	}
	if parsed.SizeRange != nil {
		sb.WriteString(fmt.Sprintf("  大小范围: %d - %d\n", parsed.SizeRange.Min, parsed.SizeRange.Max))
	}
	if parsed.DateRange != nil {
		sb.WriteString(fmt.Sprintf("  时间范围: %s - %s\n",
			parsed.DateRange.From.Format("2006-01-02"),
			parsed.DateRange.To.Format("2006-01-02")))
	}
	if parsed.IsRegex {
		sb.WriteString(fmt.Sprintf("  正则表达式: %s\n", parsed.RegexPattern))
	}
	if parsed.ExactMatch {
		sb.WriteString("  精确匹配: 是\n")
	}

	return sb.String()
}