// Package chinese 中文分词支持
// 提供 Bleve 兼容的中文分析器，支持中英文混合搜索
package chinese

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/registry"
)

const (
	// AnalyzerName 中文分析器名称.
	AnalyzerName = "chinese"
	// TokenizerName 中文分词器名称.
	TokenizerName = "chinese_tokenizer"
	// LowercaseFilterName 小写过滤器名称.
	LowercaseFilterName = "chinese_lowercase"
)

// init 注册 Bleve 分析器组件.
func init() {
	registry.RegisterTokenizer(TokenizerName, tokenizerConstructor)
	registry.RegisterTokenFilter(LowercaseFilterName, lowercaseFilterConstructor)
	registry.RegisterAnalyzer(AnalyzerName, analyzerConstructor)
}

// ================== 中文分词器 ==================

// ChineseTokenizer 中文分词器
// 实现 analysis.Tokenizer 接口，支持中英文混合分词.
type ChineseTokenizer struct {
	segmenter *Segmenter
}

// tokenizerConstructor 分词器构造函数.
func tokenizerConstructor(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
	return NewChineseTokenizer(), nil
}

// NewChineseTokenizer 创建中文分词器.
func NewChineseTokenizer() *ChineseTokenizer {
	return &ChineseTokenizer{
		segmenter: NewSegmenter(),
	}
}

// Tokenize 实现 analysis.Tokenizer 接口
// 对输入文本进行中英文混合分词.
func (t *ChineseTokenizer) Tokenize(input []byte) analysis.TokenStream {
	if len(input) == 0 {
		return analysis.TokenStream{}
	}

	text := string(input)
	words := t.segmenter.Segment(text)

	tokens := make(analysis.TokenStream, 0, len(words))
	position := 0
	bytePos := 0

	for _, word := range words {
		if word == "" {
			continue
		}

		wordBytes := []byte(word)
		start := bytePos
		end := bytePos + len(wordBytes)

		token := &analysis.Token{
			Term:     wordBytes,
			Start:    start,
			End:      end,
			Position: position,
			Type:     detectTokenType(word),
		}

		tokens = append(tokens, token)
		position++
		bytePos = end
	}

	return tokens
}

// detectTokenType 检测词元类型.
func detectTokenType(word string) analysis.TokenType {
	if word == "" {
		return analysis.Ideographic
	}

	r, _ := utf8.DecodeRuneInString(word)
	if r == utf8.RuneError {
		return analysis.Ideographic
	}

	// 中文字符
	if isChineseRune(r) {
		return analysis.Ideographic
	}

	// 数字
	if unicode.IsDigit(r) {
		return analysis.Numeric
	}

	// 字母
	if unicode.IsLetter(r) {
		return analysis.AlphaNumeric
	}

	return analysis.Ideographic
}

// ================== 小写过滤器 ==================

// ChineseLowercaseFilter 中文小写过滤器
// 对英文词元进行小写转换，中文保持不变.
type ChineseLowercaseFilter struct{}

// lowercaseFilterConstructor 过滤器构造函数.
func lowercaseFilterConstructor(config map[string]interface{}, cache *registry.Cache) (analysis.TokenFilter, error) {
	return NewChineseLowercaseFilter(), nil
}

// NewChineseLowercaseFilter 创建小写过滤器.
func NewChineseLowercaseFilter() *ChineseLowercaseFilter {
	return &ChineseLowercaseFilter{}
}

// Filter 实现 analysis.TokenFilter 接口.
func (f *ChineseLowercaseFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	output := make(analysis.TokenStream, 0, len(input))

	for _, token := range input {
		// 检查是否为需要小写化的英文词元
		if token.Type == analysis.AlphaNumeric {
			term := string(token.Term)
			lower := strings.ToLower(term)
			if lower != term {
				token.Term = []byte(lower)
				token.End = token.Start + len(token.Term)
			}
		}
		output = append(output, token)
	}

	return output
}

// ================== 中文分析器 ==================

// ChineseAnalyzer 中文分析器
// 实现 analysis.Analyzer 接口，组合分词器和过滤器.
type ChineseAnalyzer struct {
	tokenizer analysis.Tokenizer
	filters   []analysis.TokenFilter
}

// analyzerConstructor 分析器构造函数.
func analyzerConstructor(config map[string]interface{}, cache *registry.Cache) (analysis.Analyzer, error) {
	tokenizer, err := cache.DefineTokenizer(TokenizerName, config)
	if err != nil {
		return nil, err
	}

	filter, err := cache.DefineTokenFilter(LowercaseFilterName, config)
	if err != nil {
		return nil, err
	}

	return &ChineseAnalyzer{
		tokenizer: tokenizer,
		filters: []analysis.TokenFilter{
			filter,
		},
	}, nil
}

// NewChineseAnalyzer 创建中文分析器.
func NewChineseAnalyzer() *ChineseAnalyzer {
	return &ChineseAnalyzer{
		tokenizer: NewChineseTokenizer(),
		filters: []analysis.TokenFilter{
			NewChineseLowercaseFilter(),
		},
	}
}

// Analyze 实现 analysis.Analyzer 接口
// 先分词，再依次应用过滤器.
func (a *ChineseAnalyzer) Analyze(input []byte) analysis.TokenStream {
	if len(input) == 0 {
		return analysis.TokenStream{}
	}

	// 分词
	tokens := a.tokenizer.Tokenize(input)

	// 依次应用过滤器
	for _, filter := range a.filters {
		tokens = filter.Filter(tokens)
	}

	return tokens
}

// ================== 同义词过滤器 ==================

// SynonymFilter 同义词过滤器
// 基于分词器的同义词词典扩展词元.
type SynonymFilter struct {
	segmenter *Segmenter
}

// NewSynonymFilter 创建同义词过滤器.
func NewSynonymFilter(segmenter *Segmenter) *SynonymFilter {
	return &SynonymFilter{segmenter: segmenter}
}

// Filter 实现 analysis.TokenFilter 接口
// 将同义词追加到词元流中.
func (f *SynonymFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	output := make(analysis.TokenStream, 0, len(input)*2)
	output = append(output, input...)

	for _, token := range input {
		term := string(token.Term)
		synonyms := f.segmenter.GetDictionary().GetSynonyms(term)
		for _, syn := range synonyms {
			synToken := &analysis.Token{
				Term:     []byte(syn),
				Start:    token.Start,
				End:      token.End,
				Position: token.Position,
				Type:     token.Type,
			}
			output = append(output, synToken)
		}
	}

	return output
}

// ================== 工厂函数 ==================

// MustNewAnalyzer 创建分析器，失败时 panic
// 适用于初始化阶段的快速创建.
func MustNewAnalyzer() *ChineseAnalyzer {
	return NewChineseAnalyzer()
}
