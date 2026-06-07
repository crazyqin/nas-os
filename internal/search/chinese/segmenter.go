// Package chinese 中文分词支持
// 提供基于jieba的中文分词功能，增强中文搜索体验
package chinese

import (
	"strings"
	"sync"
)

// Segmenter 中文分词器
// 实现基于词典的中文分词，支持正向最大匹配和逆向最大匹配算法
type Segmenter struct {
	dict       *Dictionary
	maxWordLen int
	logger     interface{} // 可注入zap.Logger
	mu         sync.RWMutex
}

// Dictionary 分词词典
type Dictionary struct {
	words    map[string]bool
	weights  map[string]float64  // 词权重
	synonyms map[string][]string // 同义词
}

// NewSegmenter 创建分词器
func NewSegmenter() *Segmenter {
	s := &Segmenter{
		dict:       NewDefaultDictionary(),
		maxWordLen: 10, // 最大词长度
	}

	return s
}

// NewDefaultDictionary 创建默认词典
func NewDefaultDictionary() *Dictionary {
	dict := &Dictionary{
		words:    make(map[string]bool),
		weights:  make(map[string]float64),
		synonyms: make(map[string][]string),
	}

	// 加载常用中文词汇
	dict.loadDefaultWords()

	return dict
}

// loadDefaultWords 加载默认词汇
func (d *Dictionary) loadDefaultWords() {
	// 常用中文词汇
	commonWords := []string{
		"文件", "文件夹", "目录", "文档", "图片", "照片", "视频", "音频", "音乐",
		"压缩", "备份", "配置", "设置", "系统", "网络", "存储", "用户", "权限",
		"安全", "加密", "日志", "报告", "数据", "数据库", "代码", "程序", "应用",
		"下载", "上传", "分享", "同步", "复制", "移动", "删除", "重命名", "搜索",
		"查找", "排序", "过滤", "分类", "标签", "备注", "描述", "标题", "作者",
		"创建", "修改", "更新", "保存", "打开", "关闭", "启动", "停止", "重启",
		"安装", "卸载", "更新", "升级", "版本", "状态", "进度", "大小", "格式",
		"类型", "扩展", "名称", "路径", "位置", "时间", "日期", "今天", "昨天",
		"明天", "本周", "本月", "本年", "最近", "最新", "历史", "旧版", "新版",
		"重要", "紧急", "临时", "永久", "公开", "私有", "共享", "加密", "解密",
		// 新增常用词
		"管理", "处理器", "服务", "功能", "界面", "操作", "执行", "运行",
		"监控", "检测", "分析", "处理", "结果", "输出", "输入", "转换",
		"支持", "兼容", "优化", "性能", "稳定", "可靠", "安全", "高效",
		// 文件类型相关
		"文本", "表格", "演示", "幻灯片", "PDF", "Word", "Excel", "PPT",
		"图片", "JPEG", "PNG", "GIF", "BMP", "TIFF", "RAW", "SVG", "WEBP",
		"视频", "MP4", "AVI", "MKV", "MOV", "WMV", "FLV", "WEBM", "MPEG",
		"音频", "MP3", "WAV", "FLAC", "AAC", "OGG", "WMA", "APE", "M4A",
		"压缩", "ZIP", "RAR", "7Z", "TAR", "GZ", "BZ2", "XZ", "ISO",
		"代码", "GO", "Python", "JavaScript", "Java", "C", "CPP", "Rust", "Ruby",
		// 技术相关
		"服务器", "客户端", "接口", "API", "HTTP", "HTTPS", "TCP", "UDP", "DNS",
		"防火墙", "代理", "虚拟", "容器", "Docker", "K8s", "Kubernetes", "镜像",
		"编译", "运行", "调试", "测试", "部署", "发布", "回滚", "监控", "告警",
		// 业务相关
		"财务", "报表", "发票", "合同", "协议", "订单", "客户", "供应商", "项目",
		"任务", "计划", "进度", "里程碑", "风险", "问题", "解决", "方案", "策略",
	}

	for _, word := range commonWords {
		d.words[word] = true
		d.weights[word] = 1.0
	}

	// 同义词映射
	d.synonyms["图片"] = []string{"照片", "图像", "photo", "image", "picture"}
	d.synonyms["文档"] = []string{"文件", "document", "file", "doc"}
	d.synonyms["视频"] = []string{"影片", "movie", "video", "film"}
	d.synonyms["音乐"] = []string{"音频", "歌曲", "music", "audio", "song"}
	d.synonyms["代码"] = []string{"源码", "程序", "code", "source", "program"}
	d.synonyms["备份"] = []string{"拷贝", "副本", "backup", "copy", "bak"}
	d.synonyms["配置"] = []string{"设置", "选项", "config", "settings", "conf"}
	d.synonyms["压缩"] = []string{"打包", "archive", "zip", "package"}
	d.synonyms["搜索"] = []string{"查找", "检索", "search", "find", "query"}
	d.synonyms["文件夹"] = []string{"目录", "folder", "directory", "dir"}
	d.synonyms["报告"] = []string{"报表", "report", "报表"}
	d.synonyms["日志"] = []string{"记录", "log", "record", "logs"}
	d.synonyms["数据"] = []string{"资料", "data", "database", "db"}
	d.synonyms["用户"] = []string{"账号", "user", "account", "users"}
	d.synonyms["权限"] = []string{"授权", "permission", "auth", "access"}
	d.synonyms["安全"] = []string{"防护", "security", "safe", "protect"}
	d.synonyms["网络"] = []string{"互联网", "network", "net", "internet"}
	d.synonyms["存储"] = []string{"硬盘", "磁盘", "storage", "disk", "drive"}
	d.synonyms["系统"] = []string{"平台", "system", "platform", "sys"}
	d.synonyms["项目"] = []string{"工程", "project", "proj"}
	d.synonyms["任务"] = []string{"作业", "task", "job", "work"}
}

// AddWord 添加自定义词汇
func (d *Dictionary) AddWord(word string, weight float64) {
	d.words[word] = true
	if weight > 0 {
		d.weights[word] = weight
	} else {
		d.weights[word] = 1.0
	}
}

// AddSynonym 添加同义词
func (d *Dictionary) AddSynonym(word string, synonyms []string) {
	d.synonyms[word] = synonyms
}

// HasWord 检查词汇是否存在
func (d *Dictionary) HasWord(word string) bool {
	return d.words[word]
}

// GetWeight 获取词汇权重
func (d *Dictionary) GetWeight(word string) float64 {
	if w, ok := d.weights[word]; ok {
		return w
	}
	return 0.5 // 默认权重
}

// GetSynonyms 获取同义词
func (d *Dictionary) GetSynonyms(word string) []string {
	if syns, ok := d.synonyms[word]; ok {
		return syns
	}
	return nil
}

// Segment 分词 - 正向最大匹配算法
func (s *Segmenter) Segment(text string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if text == "" {
		return nil
	}

	// 混合处理：中文和英文分开处理
	var result []string
	var chineseBuffer string
	var englishBuffer string

	// 使用rune处理UTF-8
	runes := []rune(text)

	for _, r := range runes {
		// 检查是否为中文字符
		if isChineseRune(r) {
			// 先处理之前积累的英文
			if englishBuffer != "" {
				result = append(result, englishBuffer)
				englishBuffer = ""
			}
			chineseBuffer += string(r)
		} else {
			// 先处理之前积累的中文
			if chineseBuffer != "" {
				words := s.segmentChinese(chineseBuffer)
				result = append(result, words...)
				chineseBuffer = ""
			}

			// 英文/数字/符号处理
			if r == ' ' || r == '\t' || r == '\n' {
				if englishBuffer != "" {
					result = append(result, englishBuffer)
					englishBuffer = ""
				}
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				englishBuffer += string(r)
			} else {
				// 特殊符号
				if englishBuffer != "" {
					result = append(result, englishBuffer)
					englishBuffer = ""
				}
				result = append(result, string(r))
			}
		}
	}

	// 处理剩余缓冲
	if chineseBuffer != "" {
		words := s.segmentChinese(chineseBuffer)
		result = append(result, words...)
	}
	if englishBuffer != "" {
		result = append(result, englishBuffer)
	}

	return result
}

// segmentChinese 中文分词 - 正向最大匹配
func (s *Segmenter) segmentChinese(text string) []string {
	var result []string
	runes := []rune(text)
	pos := 0

	for pos < len(runes) {
		// 尝试匹配最长词汇
		maxMatch := ""
		matchLen := 0

		// 从最大词长度开始递减尝试
		for l := s.maxWordLen; l >= 2; l-- {
			if pos+l > len(runes) {
				continue
			}

			sub := string(runes[pos : pos+l])
			if s.dict.HasWord(sub) {
				maxMatch = sub
				matchLen = l
				break
			}
		}

		if matchLen > 0 {
			result = append(result, maxMatch)
			pos += matchLen
		} else {
			// 单字切分
			result = append(result, string(runes[pos]))
			pos++
		}
	}

	return result
}

// SegmentReverse 逆向最大匹配分词
func (s *Segmenter) SegmentReverse(text string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if text == "" {
		return nil
	}

	var result []string
	runes := []rune(text)
	pos := len(runes)

	for pos > 0 {
		maxMatch := ""
		matchLen := 0

		for l := s.maxWordLen; l >= 2; l-- {
			if pos-l < 0 {
				continue
			}

			sub := string(runes[pos-l : pos])
			if s.dict.HasWord(sub) {
				maxMatch = sub
				matchLen = l
				break
			}
		}

		if matchLen > 0 {
			result = append([]string{maxMatch}, result...)
			pos -= matchLen
		} else {
			result = append([]string{string(runes[pos-1])}, result...)
			pos--
		}
	}

	return result
}

// SegmentBidirectional 双向最大匹配分词
// 结合正向和逆向匹配，选择切分数量较少的结果
func (s *Segmenter) SegmentBidirectional(text string) []string {
	forward := s.Segment(text)
	reverse := s.SegmentReverse(text)

	// 选择词数较少的结果（切分粒度更大）
	if len(forward) <= len(reverse) {
		return forward
	}
	return reverse
}

// SegmentWithPosition 分词并返回位置信息
func (s *Segmenter) SegmentWithPosition(text string) []Token {
	words := s.Segment(text)
	tokens := make([]Token, 0, len(words))

	pos := 0
	for _, word := range words {
		token := Token{
			Text:   word,
			Start:  pos,
			End:    pos + len(word),
			Weight: s.dict.GetWeight(word),
		}
		tokens = append(tokens, token)
		pos += len(word)
	}

	return tokens
}

// Token 分词结果
type Token struct {
	Text   string  `json:"text"`
	Start  int     `json:"start"`
	End    int     `json:"end"`
	Weight float64 `json:"weight"`
	Type   string  `json:"type"` // word, number, symbol, unknown
}

// ExpandQuery 查询扩展
// 根据同义词扩展搜索词，提高召回率
func (s *Segmenter) ExpandQuery(query string) []string {
	words := s.Segment(query)
	var expanded []string

	// 原始查询词
	expanded = append(expanded, query)

	// 分词后的词
	expanded = append(expanded, words...)

	// 同义词扩展
	for _, word := range words {
		synonyms := s.dict.GetSynonyms(word)
		if len(synonyms) > 0 {
			expanded = append(expanded, synonyms...)
		}
	}

	// 去重
	return uniqueStrings(expanded)
}

// uniqueStrings 字符串去重
func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}

	return result
}

// isChineseChar 判断是否为中文字符
func isChineseChar(c byte) bool {
	// UTF-8中文字符的范围
	// 中文字符的第一个字节在 0x81-0xFE 范围内
	// 简化判断：检查是否在高位范围
	return c >= 0x80
}

// isChineseRune 判断rune是否为中文字符
func isChineseRune(r rune) bool {
	// CJK统一汉字范围
	// 0x4E00-0x9FFF: CJK统一汉字
	// 0x3400-0x4DBF: CJK扩展A
	// 0x20000-0x2A6DF: CJK扩展B (需要特殊处理)
	return r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF
}

// NormalizeText 文本规范化
func (s *Segmenter) NormalizeText(text string) string {
	// 转小写
	text = strings.ToLower(text)

	// 去除多余空格
	text = strings.TrimSpace(text)

	// 替换特殊字符
	replacer := strings.NewReplacer(
		"　", " ", // 全角空格
		"—", "-", // 全角横线
		"－", "-", // 全角减号
		"：", ":", // 全角冒号
		"，", ",", // 全角逗号
		"。", ".", // 全角句号
		"？", "?", // 全角问号
		"！", "!", // 全角感叹号
		"（", "(", // 全角左括号
		"）", ")", // 全角右括号
		"【", "[", // 全角左中括号
		"】", "]", // 全角右中括号
		"《", "<", // 全角左书名号
		"》", ">", // 全角右书名号
	)
	text = replacer.Replace(text)

	return text
}

// ExtractKeywords 关键词提取
// 从文本中提取关键词，用于索引
func (s *Segmenter) ExtractKeywords(text string, topN int) []Keyword {
	tokens := s.SegmentWithPosition(text)

	// 统计词频
	freqMap := make(map[string]int)
	weightMap := make(map[string]float64)

	for _, token := range tokens {
		freqMap[token.Text]++
		weightMap[token.Text] = token.Weight
	}

	// 构建关键词列表
	keywords := make([]Keyword, 0, len(freqMap))
	for word, freq := range freqMap {
		keywords = append(keywords, Keyword{
			Text:   word,
			Count:  freq,
			Weight: weightMap[word] * float64(freq),
		})
	}

	// 按权重排序
	sortKeywords(keywords)

	// 返回topN
	if len(keywords) > topN {
		keywords = keywords[:topN]
	}

	return keywords
}

// Keyword 关键词
type Keyword struct {
	Text   string  `json:"text"`
	Count  int     `json:"count"`
	Weight float64 `json:"weight"`
}

// sortKeywords 关键词排序
func sortKeywords(keywords []Keyword) {
	// 按权重降序排序
	for i := 0; i < len(keywords)-1; i++ {
		for j := i + 1; j < len(keywords); j++ {
			if keywords[j].Weight > keywords[i].Weight {
				keywords[i], keywords[j] = keywords[j], keywords[i]
			}
		}
	}
}

// LoadCustomDictionary 加载自定义词典
func (s *Segmenter) LoadCustomDictionary(words []string, weights map[string]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, word := range words {
		s.dict.AddWord(word, weights[word])
	}
}

// LoadSynonyms 加载同义词词典
func (s *Segmenter) LoadSynonyms(synonyms map[string][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for word, syns := range synonyms {
		s.dict.AddSynonym(word, syns)
	}
}

// GetDictionary 获取词典
func (s *Segmenter) GetDictionary() *Dictionary {
	return s.dict
}

// SetMaxWordLen 设置最大词长度
func (s *Segmenter) SetMaxWordLen(len int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxWordLen = len
}

// GetMaxWordLen 获取最大词长度
func (s *Segmenter) GetMaxWordLen() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxWordLen
}
