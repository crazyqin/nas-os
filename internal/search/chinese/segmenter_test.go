package chinese

import (
	"testing"
)

func TestNewSegmenter(t *testing.T) {
	s := NewSegmenter()
	if s == nil {
		t.Fatal("Segmenter should not be nil")
	}
	if s.dict == nil {
		t.Fatal("Dictionary should not be nil")
	}
	if s.maxWordLen <= 0 {
		t.Fatal("MaxWordLen should be positive")
	}
}

func TestSegment(t *testing.T) {
	s := NewSegmenter()

	tests := []struct {
		input    string
		minWords int // 最少应该有这么多词
		maxWords int // 最多应该有这么多词
	}{
		{
			input:    "文件管理",
			minWords: 2,
			maxWords: 4,
		},
		{
			input:    "搜索图片",
			minWords: 2,
			maxWords: 4,
		},
		{
			input:    "备份视频文件",
			minWords: 3,
			maxWords: 6,
		},
		{
			input:    "test文件",
			minWords: 2,
			maxWords: 3,
		},
		{
			input:    "文件test",
			minWords: 2,
			maxWords: 3,
		},
	}

	for _, tt := range tests {
		result := s.Segment(tt.input)
		if len(result) < tt.minWords {
			t.Errorf("Segment(%s): got %d words, expected at least %d words", tt.input, len(result), tt.minWords)
		}
		if len(result) > tt.maxWords {
			t.Errorf("Segment(%s): got %d words, expected at most %d words", tt.input, len(result), tt.maxWords)
		}
	}
}

func TestSegmentReverse(t *testing.T) {
	s := NewSegmenter()

	text := "文件管理"
	result := s.SegmentReverse(text)

	if len(result) < 1 {
		t.Errorf("SegmentReverse should return at least one word")
	}
}

func TestSegmentBidirectional(t *testing.T) {
	s := NewSegmenter()

	text := "搜索图片文件"
	result := s.SegmentBidirectional(text)

	if len(result) < 1 {
		t.Errorf("SegmentBidirectional should return at least one word")
	}

	// 双向匹配应该返回合理数量的词
	if len(result) > 10 {
		t.Errorf("SegmentBidirectional returned too many words: %d", len(result))
	}
}

func TestExpandQuery(t *testing.T) {
	s := NewSegmenter()

	tests := []struct {
		query    string
		minCount int // 最少应该返回多少词
	}{
		{"图片", 5}, // 图片 + 同义词
		{"文档", 4},
		{"视频", 4},
		{"搜索文件", 2}, // 至少包含分词结果
	}

	for _, tt := range tests {
		result := s.ExpandQuery(tt.query)
		if len(result) < tt.minCount {
			t.Errorf("ExpandQuery(%s): got %d words, expected at least %d", tt.query, len(result), tt.minCount)
		}
	}
}

func TestNormalizeText(t *testing.T) {
	s := NewSegmenter()

	tests := []struct {
		input    string
		expected string
	}{
		{"文件　管理", "文件 管理"},
		{"搜索：图片", "搜索:图片"},
		{"文件（副本）", "文件(副本)"},
		{"TEST", "test"},
	}

	for _, tt := range tests {
		result := s.NormalizeText(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeText(%s): got %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestExtractKeywords(t *testing.T) {
	s := NewSegmenter()

	text := "搜索图片文件，图片文件管理，文件搜索功能"
	keywords := s.ExtractKeywords(text, 5)

	if len(keywords) > 5 {
		t.Errorf("ExtractKeywords should return at most %d keywords", 5)
	}

	// 检查关键词权重排序
	for i := 1; i < len(keywords); i++ {
		if keywords[i].Weight > keywords[i-1].Weight {
			t.Errorf("Keywords should be sorted by weight descending")
		}
	}
}

func TestDictionary(t *testing.T) {
	dict := NewDefaultDictionary()

	// 测试常用词汇
	commonWords := []string{"文件", "图片", "视频", "文档", "备份"}
	for _, word := range commonWords {
		if !dict.HasWord(word) {
			t.Errorf("Dictionary should contain common word: %s", word)
		}
	}

	// 测试权重
	if dict.GetWeight("文件") <= 0 {
		t.Errorf("Word weight should be positive")
	}

	// 测试同义词
	synonyms := dict.GetSynonyms("图片")
	if len(synonyms) < 3 {
		t.Errorf("图片 should have at least 3 synonyms")
	}
}

func TestAddWord(t *testing.T) {
	s := NewSegmenter()

	// 添加自定义词汇
	customWord := "自定义词"
	s.dict.AddWord(customWord, 1.5)

	if !s.dict.HasWord(customWord) {
		t.Errorf("Dictionary should contain added word: %s", customWord)
	}

	if s.dict.GetWeight(customWord) != 1.5 {
		t.Errorf("Word weight should be 1.5")
	}
}

func TestAddSynonym(t *testing.T) {
	s := NewSegmenter()

	// 添加自定义同义词
	word := "自定义"
	synonyms := []string{"custom", "customized"}
	s.dict.AddSynonym(word, synonyms)

	result := s.dict.GetSynonyms(word)
	if len(result) != len(synonyms) {
		t.Errorf("Synonym count mismatch")
	}
}

func TestSegmentWithPosition(t *testing.T) {
	s := NewSegmenter()

	text := "文件管理"
	tokens := s.SegmentWithPosition(text)

	if len(tokens) < 2 {
		t.Errorf("Should return at least 2 tokens")
	}

	for i, token := range tokens {
		if token.Text == "" {
			t.Errorf("Token %d should not have empty text", i)
		}
		if token.Start < 0 || token.End < 0 {
			t.Errorf("Token %d should have valid positions", i)
		}
		if token.Start >= token.End {
			t.Errorf("Token %d: Start should be less than End", i)
		}
	}
}

func TestMixedContent(t *testing.T) {
	s := NewSegmenter()

	// 测试中英文混合内容
	texts := []string{
		"test文件管理",
		"file文件search搜索",
		"PDF文档report报告",
	}

	for _, text := range texts {
		result := s.Segment(text)
		if len(result) == 0 {
			t.Errorf("Segment(%s) should return at least one word", text)
		}
	}
}

func TestEmptyInput(t *testing.T) {
	s := NewSegmenter()

	result := s.Segment("")
	if result != nil {
		t.Errorf("Segment empty string should return nil")
	}

	result = s.SegmentReverse("")
	if result != nil {
		t.Errorf("SegmentReverse empty string should return nil")
	}
}

func TestCustomDictionary(t *testing.T) {
	s := NewSegmenter()

	// 加载自定义词典
	customWords := []string{"专有词汇", "自定义术语"}
	weights := map[string]float64{
		"专有词汇":   2.0,
		"自定义术语": 1.5,
	}

	s.LoadCustomDictionary(customWords, weights)

	for _, word := range customWords {
		if !s.dict.HasWord(word) {
			t.Errorf("Should have custom word: %s", word)
		}
	}
}

func TestSynonymsLoading(t *testing.T) {
	s := NewSegmenter()

	// 加载同义词
	synonyms := map[string][]string{
		"新词": {"new", "novel", "fresh"},
	}

	s.LoadSynonyms(synonyms)

	result := s.dict.GetSynonyms("新词")
	if len(result) != 3 {
		t.Errorf("Should have 3 synonyms")
	}
}

func TestMaxWordLen(t *testing.T) {
	s := NewSegmenter()

	// 设置最大词长度
	s.SetMaxWordLen(5)

	if s.maxWordLen != 5 {
		t.Errorf("MaxWordLen should be 5")
	}
}

func BenchmarkSegment(b *testing.B) {
	s := NewSegmenter()
	text := "搜索图片文件管理备份视频音频文档"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Segment(text)
	}
}

func BenchmarkExpandQuery(b *testing.B) {
	s := NewSegmenter()
	query := "图片文件"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExpandQuery(query)
	}
}

func BenchmarkExtractKeywords(b *testing.B) {
	s := NewSegmenter()
	text := "搜索图片文件管理备份视频音频文档报告数据系统网络存储用户权限安全加密日志"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExtractKeywords(text, 10)
	}
}