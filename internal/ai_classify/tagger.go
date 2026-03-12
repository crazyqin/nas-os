package ai_classify

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Tagger 标签生成器
type Tagger struct {
	config     Config
	tagRules   []TagRule
	customTags map[string]Tag
	mu         sync.RWMutex
}

// TagRule 标签规则
type TagRule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	TagName    string      `json:"tagName"`
	Conditions []Condition `json:"conditions"`
	Priority   int         `json:"priority"`
	Enabled    bool        `json:"enabled"`
	HitCount   int         `json:"hitCount"`
	LastHit    time.Time   `json:"lastHit"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// NewTagger 创建标签生成器
func NewTagger(config Config) *Tagger {
	t := &Tagger{
		config:     config,
		tagRules:   make([]TagRule, 0),
		customTags: make(map[string]Tag),
	}

	t.initDefaultRules()
	return t
}

// initDefaultRules 初始化默认标签规则
func (t *Tagger) initDefaultRules() {
	// 文件名模式规则
	t.tagRules = []TagRule{
		{
			ID:      "tag_invoice",
			Name:    "发票识别",
			TagName: "发票",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(invoice|发票|收据|receipt)`},
			},
			Priority:  100,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_contract",
			Name:    "合同识别",
			TagName: "合同",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(contract|合同|协议|agreement)`},
			},
			Priority:  100,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_resume",
			Name:    "简历识别",
			TagName: "简历",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(resume|cv|简历|履历)`},
			},
			Priority:  100,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_report",
			Name:    "报告识别",
			TagName: "报告",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(report|报告|报表)`},
			},
			Priority:  100,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_financial",
			Name:    "财务文件识别",
			TagName: "财务",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(财务|financial|账|accounting|budget|预算)`},
			},
			Priority:  90,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_legal",
			Name:    "法律文件识别",
			TagName: "法律",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(legal|法律|law|attorney|律师|court|法院)`},
			},
			Priority:  90,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_project",
			Name:    "项目文件识别",
			TagName: "项目",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(project|项目|proj)`},
			},
			Priority:  80,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_meeting",
			Name:    "会议文件识别",
			TagName: "会议",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(meeting|会议|会议纪要|minutes|议程)`},
			},
			Priority:  80,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_backup",
			Name:    "备份文件识别",
			TagName: "备份",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(backup|备份|bak|old|copy|副本)`},
			},
			Priority:  70,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_temp",
			Name:    "临时文件识别",
			TagName: "临时",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(temp|tmp|临时|未命名|untitled)`},
			},
			Priority:  70,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_confidential",
			Name:    "机密文件识别",
			TagName: "机密",
			Conditions: []Condition{
				{Type: CondTypeFileName, Operator: "matches", Value: `(?i)(confidential|机密|secret|私密|internal|内部)`},
			},
			Priority:  95,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:      "tag_archive",
			Name:    "归档文件识别",
			TagName: "归档",
			Conditions: []Condition{
				{Type: CondTypeExtension, Operator: "in", Value: []interface{}{".zip", ".rar", ".7z", ".tar", ".gz"}},
			},
			Priority:  60,
			Enabled:   true,
			CreatedAt: time.Now(),
		},
	}
}

// GenerateTags 为文件生成标签
func (t *Tagger) GenerateTags(ctx context.Context, path string, features Features, category Category) []Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := make(map[string]Tag)
	fileName := strings.ToLower(path)
	ext := strings.ToLower(getExt(path))

	// 按优先级排序规则
	sort.Slice(t.tagRules, func(i, j int) bool {
		return t.tagRules[i].Priority > t.tagRules[j].Priority
	})

	// 应用标签规则
	for _, rule := range t.tagRules {
		if !rule.Enabled {
			continue
		}

		if t.matchRule(path, fileName, ext, features, rule) {
			tag := Tag{
				ID:       rule.ID,
				Name:     rule.TagName,
				Category: category.ID,
			}
			tags[rule.TagName] = tag

			// 更新命中计数
			t.mu.RUnlock()
			t.mu.Lock()
			rule.HitCount++
			rule.LastHit = time.Now()
			t.mu.Unlock()
			t.mu.RLock()
		}
	}

	// 基于分类自动添加标签
	t.addCategoryTags(category, tags)

	// 基于特征添加标签
	t.addFeatureTags(features, tags)

	// 基于路径添加标签
	t.addPathTags(path, tags)

	// 限制标签数量
	result := make([]Tag, 0, len(tags))
	for _, tag := range tags {
		result = append(result, tag)
		if len(result) >= t.config.MaxTags {
			break
		}
	}

	return result
}

// matchRule 匹配标签规则
func (t *Tagger) matchRule(path, fileName, ext string, features Features, rule TagRule) bool {
	for _, cond := range rule.Conditions {
		var value interface{}

		switch cond.Type {
		case CondTypeFileName:
			value = fileName
		case CondTypeExtension:
			value = ext
		case CondTypePath:
			value = path
		case CondTypeContent:
			// 从特征中获取内容关键词
			if len(features.Keywords) > 0 {
				value = strings.ToLower(strings.Join(features.Keywords, " "))
			}
		case CondTypeMetadata:
			// 可以扩展为从元数据获取值
			value = ""
		}

		if !t.matchOperator(value, cond.Operator, cond.Value) {
			return false
		}
	}
	return true
}

// matchOperator 匹配操作符
func (t *Tagger) matchOperator(value interface{}, op string, target interface{}) bool {
	switch op {
	case "eq", "==":
		return value == target
	case "ne", "!=":
		return value != target
	case "contains":
		if str, ok := value.(string); ok {
			if targetStr, ok := target.(string); ok {
				return strings.Contains(strings.ToLower(str), strings.ToLower(targetStr))
			}
		}
	case "matches":
		if str, ok := value.(string); ok {
			if pattern, ok := target.(string); ok {
				matched, _ := regexp.MatchString(pattern, str)
				return matched
			}
		}
	case "in":
		if arr, ok := target.([]interface{}); ok {
			for _, item := range arr {
				if value == item {
					return true
				}
			}
		}
		return false
	}
	return false
}

// addCategoryTags 基于分类添加标签
func (t *Tagger) addCategoryTags(category Category, tags map[string]Tag) {
	categoryTags := map[string]string{
		"documents": "文档",
		"images":    "图片",
		"videos":    "视频",
		"audio":     "音频",
		"code":      "代码",
		"archives":  "压缩包",
		"ebooks":    "电子书",
	}

	if tagName, ok := categoryTags[category.ID]; ok {
		tags[tagName] = Tag{
			ID:       "cat_" + category.ID,
			Name:     tagName,
			Category: category.ID,
		}
	}
}

// addFeatureTags 基于特征添加标签
func (t *Tagger) addFeatureTags(features Features, tags map[string]Tag) {
	// 根据文件大小添加标签
	if features.Width > 0 && features.Height > 0 {
		if features.Width > 4000 || features.Height > 4000 {
			tags["高清"] = Tag{ID: "feat_hd", Name: "高清"}
		}
		if features.Width > 8000 || features.Height > 8000 {
			tags["8K"] = Tag{ID: "feat_8k", Name: "8K"}
		}
	}

	// 根据时长添加标签
	if features.Duration > 0 {
		if features.Duration > 3600 {
			tags["长视频"] = Tag{ID: "feat_long", Name: "长视频"}
		} else if features.Duration < 60 {
			tags["短视频"] = Tag{ID: "feat_short", Name: "短视频"}
		}
	}

	// 根据代码特征添加标签
	if features.LineCount > 0 {
		if features.LineCount > 10000 {
			tags["大型项目"] = Tag{ID: "feat_large", Name: "大型项目"}
		} else if features.LineCount < 100 {
			tags["小型脚本"] = Tag{ID: "feat_script", Name: "小型脚本"}
		}
	}
}

// addPathTags 基于路径添加标签
func (t *Tagger) addPathTags(path string, tags map[string]Tag) {
	pathLower := strings.ToLower(path)

	// 路径关键词标签
	pathKeywords := map[string]string{
		"work":      "工作",
		"工作":        "工作",
		"personal":  "个人",
		"个人":        "个人",
		"download":  "下载",
		"下载":        "下载",
		"desktop":   "桌面",
		"桌面":        "桌面",
		"documents": "文档",
		"文档":        "文档",
		"photos":    "照片",
		"照片":        "照片",
		"backup":    "备份",
		"备份":        "备份",
		"project":   "项目",
		"项目":        "项目",
	}

	for keyword, tagName := range pathKeywords {
		if strings.Contains(pathLower, keyword) {
			tags[tagName] = Tag{
				ID:   "path_" + keyword,
				Name: tagName,
			}
		}
	}

	// 年份标签
	yearPattern := regexp.MustCompile(`[^\d](20\d{2})[^\d]`)
	if matches := yearPattern.FindStringSubmatch(path); len(matches) > 1 {
		tags[matches[1]+"年"] = Tag{
			ID:   "year_" + matches[1],
			Name: matches[1] + "年",
		}
	}

	// 月份标签
	monthPattern := regexp.MustCompile(`[^\d](20\d{2}[-_]?0?[1-9]|1[0-2])`)
	if matches := monthPattern.FindStringSubmatch(path); len(matches) > 1 {
		tags["月份归档"] = Tag{
			ID:   "monthly",
			Name: "月份归档",
		}
	}
}

// getExt 获取扩展名
func getExt(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx == -1 {
		return ""
	}
	return strings.ToLower(path[idx:])
}

// AddTagRule 添加标签规则
func (t *Tagger) AddTagRule(rule TagRule) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	rule.ID = generateID("tag_rule")
	rule.CreatedAt = time.Now()
	t.tagRules = append(t.tagRules, rule)

	return nil
}

// GetTagRules 获取所有标签规则
func (t *Tagger) GetTagRules() []TagRule {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.tagRules
}

// AddCustomTag 添加自定义标签
func (t *Tagger) AddCustomTag(tag Tag) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if tag.ID == "" {
		tag.ID = generateID("custom_tag")
	}
	tag.CreatedAt = time.Now()
	t.customTags[tag.ID] = tag
}

// GetCustomTags 获取自定义标签
func (t *Tagger) GetCustomTags() []Tag {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var tags []Tag
	for _, tag := range t.customTags {
		tags = append(tags, tag)
	}
	return tags
}

// ExtractKeywordsFromContent 从内容提取关键词作为标签
func (t *Tagger) ExtractKeywordsFromContent(content string) []Tag {
	keywords := extractKeywordsSimple(content)
	tags := make([]Tag, 0, len(keywords))

	for i, kw := range keywords {
		if i >= 5 { // 最多 5 个关键词标签
			break
		}
		tags = append(tags, Tag{
			ID:   "kw_" + kw,
			Name: kw,
		})
	}

	return tags
}

// extractKeywordsSimple 简单关键词提取
func extractKeywordsSimple(content string) []string {
	words := strings.Fields(strings.ToLower(content))
	wordCount := make(map[string]int)

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "can": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "under": true,
	}

	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}<>")
		if len(word) < 3 || stopWords[word] {
			continue
		}
		wordCount[word]++
	}

	type kv struct {
		key   string
		value int
	}

	var sorted []kv
	for k, v := range wordCount {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].value > sorted[j].value
	})

	var result []string
	for i := 0; i < 10 && i < len(sorted); i++ {
		result = append(result, sorted[i].key)
	}

	return result
}
