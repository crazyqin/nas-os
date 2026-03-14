package ai_classify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Classifier AI 文件分类器
type Classifier struct {
	config     Config
	categories map[string]Category
	rules      []ClassificationRule
	tags       map[string]Tag
	hashCache  map[string]string // 文件路径 -> 哈希
	mu         sync.RWMutex
}

// NewClassifier 创建分类器
func NewClassifier(config Config) (*Classifier, error) {
	c := &Classifier{
		config:     config,
		categories: make(map[string]Category),
		rules:      make([]ClassificationRule, 0),
		tags:       make(map[string]Tag),
		hashCache:  make(map[string]string),
	}

	// 加载数据
	if err := c.load(); err != nil {
		// 如果加载失败，使用默认分类
		c.initDefaultCategories()
	}

	return c, nil
}

// initDefaultCategories 初始化默认分类
func (c *Classifier) initDefaultCategories() {
	defaultCats := []Category{
		{
			ID:          "documents",
			Name:        "文档",
			Description: "文档文件，包括 Office、PDF 等",
			Extensions:  []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".rtf", ".txt"},
			Color:       "#3498db",
			Icon:        "file-text",
		},
		{
			ID:          "images",
			Name:        "图片",
			Description: "图像文件",
			Extensions:  []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".ico", ".tiff", ".heic", ".heif"},
			Color:       "#e74c3c",
			Icon:        "image",
		},
		{
			ID:          "videos",
			Name:        "视频",
			Description: "视频文件",
			Extensions:  []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpeg", ".mpg"},
			Color:       "#9b59b6",
			Icon:        "video",
		},
		{
			ID:          "audio",
			Name:        "音频",
			Description: "音频文件",
			Extensions:  []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a", ".ape"},
			Color:       "#1abc9c",
			Icon:        "music",
		},
		{
			ID:          "code",
			Name:        "代码",
			Description: "源代码文件",
			Extensions:  []string{".js", ".ts", ".py", ".go", ".java", ".c", ".cpp", ".h", ".rs", ".rb", ".php", ".swift", ".kt"},
			Color:       "#f39c12",
			Icon:        "code",
		},
		{
			ID:          "archives",
			Name:        "压缩包",
			Description: "压缩文件",
			Extensions:  []string{".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz"},
			Color:       "#95a5a6",
			Icon:        "archive",
		},
		{
			ID:          "ebooks",
			Name:        "电子书",
			Description: "电子书文件",
			Extensions:  []string{".epub", ".mobi", ".azw", ".azw3", ".fb2"},
			Color:       "#2ecc71",
			Icon:        "book",
		},
		{
			ID:          "databases",
			Name:        "数据库",
			Description: "数据库文件",
			Extensions:  []string{".db", ".sqlite", ".sqlite3", ".mdb", ".accdb"},
			Color:       "#e67e22",
			Icon:        "database",
		},
		{
			ID:          "fonts",
			Name:        "字体",
			Description: "字体文件",
			Extensions:  []string{".ttf", ".otf", ".woff", ".woff2", ".eot"},
			Color:       "#34495e",
			Icon:        "font",
		},
		{
			ID:          "others",
			Name:        "其他",
			Description: "其他类型文件",
			Extensions:  []string{},
			Color:       "#7f8c8d",
			Icon:        "file",
		},
	}

	for _, cat := range defaultCats {
		cat.CreatedAt = time.Now()
		cat.UpdatedAt = time.Now()
		c.categories[cat.ID] = cat
	}

	// 添加子分类
	subCats := []Category{
		{
			ID:          "invoices",
			Name:        "发票",
			Description: "发票和收据",
			Keywords:    []string{"invoice", "发票", "收据", "receipt", "账单"},
			Patterns:    []string{`(?i)invoice|发票|收据|receipt`},
			ParentID:    "documents",
			Color:       "#27ae60",
			Icon:        "receipt",
		},
		{
			ID:          "contracts",
			Name:        "合同",
			Description: "合同和协议",
			Keywords:    []string{"contract", "合同", "agreement", "协议", "agreement"},
			Patterns:    []string{`(?i)contract|合同|协议|agreement`},
			ParentID:    "documents",
			Color:       "#8e44ad",
			Icon:        "file-contract",
		},
		{
			ID:          "resumes",
			Name:        "简历",
			Description: "简历和履历",
			Keywords:    []string{"resume", "简历", "cv", "curriculum", "履历"},
			Patterns:    []string{`(?i)resume|简历|cv|curriculum|履历`},
			ParentID:    "documents",
			Color:       "#16a085",
			Icon:        "user-tie",
		},
		{
			ID:          "screenshots",
			Name:        "截图",
			Description: "屏幕截图",
			Keywords:    []string{"screenshot", "截图", "capture"},
			Patterns:    []string{`(?i)screenshot|截图|capture`},
			ParentID:    "images",
			Color:       "#d35400",
			Icon:        "camera",
		},
		{
			ID:          "photos",
			Name:        "照片",
			Description: "照片",
			Keywords:    []string{"photo", "照片", "img", "image", "picture"},
			Patterns:    []string{`(?i)(^|[_-])(photo|img|image|picture|照片|pic)[_-]?\d*`},
			ParentID:    "images",
			Color:       "#c0392b",
			Icon:        "camera-retro",
		},
		{
			ID:          "movies",
			Name:        "电影",
			Description: "电影文件",
			Keywords:    []string{"movie", "电影", "film"},
			Patterns:    []string{`(?i)(19|20)\d{2}`}, // 年份模式
			ParentID:    "videos",
			Color:       "#8e44ad",
			Icon:        "film",
		},
		{
			ID:          "music",
			Name:        "音乐",
			Description: "音乐文件",
			Keywords:    []string{"music", "音乐", "song", "歌曲"},
			Patterns:    []string{},
			ParentID:    "audio",
			Color:       "#1abc9c",
			Icon:        "music",
		},
		{
			ID:          "podcasts",
			Name:        "播客",
			Description: "播客录音",
			Keywords:    []string{"podcast", "播客"},
			Patterns:    []string{`(?i)podcast|播客`},
			ParentID:    "audio",
			Color:       "#9b59b6",
			Icon:        "podcast",
		},
	}

	for _, cat := range subCats {
		cat.CreatedAt = time.Now()
		cat.UpdatedAt = time.Now()
		c.categories[cat.ID] = cat
	}

	// 初始化默认标签
	c.initDefaultTags()
}

// initDefaultTags 初始化默认标签
func (c *Classifier) initDefaultTags() {
	defaultTags := []Tag{
		{Name: "重要", Color: "#e74c3c"},
		{Name: "工作", Color: "#3498db"},
		{Name: "个人", Color: "#2ecc71"},
		{Name: "待处理", Color: "#f39c12"},
		{Name: "已完成", Color: "#27ae60"},
		{Name: "归档", Color: "#95a5a6"},
		{Name: "加密", Color: "#9b59b6"},
		{Name: "备份", Color: "#1abc9c"},
		{Name: "临时", Color: "#e67e22"},
		{Name: "收藏", Color: "#e91e63"},
	}

	for i, tag := range defaultTags {
		tag.ID = fmt.Sprintf("tag_%d", i+1)
		tag.CreatedAt = time.Now()
		c.tags[tag.ID] = tag
	}
}

// Classify 分类文件
func (c *Classifier) Classify(ctx context.Context, path string) (*FileClassification, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("无法访问文件: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("不支持分类目录")
	}

	// 提取特征
	features, err := c.extractFeatures(path)
	if err != nil {
		// 特征提取失败不阻止分类
		features = Features{}
	}

	// 1. 先应用规则
	ruleResult := c.applyRules(path, info)
	if ruleResult != nil {
		return ruleResult, nil
	}

	// 2. 基于文件名和扩展名分类
	category, confidence := c.classifyByPath(path)

	// 3. 基于内容分类（如果启用）
	if c.config.EnableTextFeat && confidence < c.config.ConfidenceThreshold {
		contentCategory, contentConf := c.classifyByContent(path, features)
		if contentConf > confidence {
			category = contentCategory
			confidence = contentConf
		}
	}

	// 4. 生成标签
	tags := c.generateTags(path, features, category)

	// 5. 计算内容哈希
	contentHash, _ := c.calculateHash(path)

	return &FileClassification{
		Path:        path,
		FileName:    filepath.Base(path),
		Extension:   strings.ToLower(filepath.Ext(path)),
		Category:    category,
		Tags:        tags,
		Confidence:  confidence,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		ContentHash: contentHash,
		Features:    features,
		CreatedAt:   time.Now(),
	}, nil
}

// classifyByPath 基于路径分类
func (c *Classifier) classifyByPath(path string) (Category, float64) {
	ext := strings.ToLower(filepath.Ext(path))
	fileName := strings.ToLower(filepath.Base(path))

	// 检查子分类（关键词和模式）
	for _, cat := range c.categories {
		if cat.ParentID == "" {
			continue
		}

		// 检查关键词
		for _, kw := range cat.Keywords {
			if strings.Contains(fileName, strings.ToLower(kw)) {
				return cat, 0.85
			}
		}

		// 检查模式
		for _, pattern := range cat.Patterns {
			if matched, _ := regexp.MatchString(pattern, fileName); matched {
				return cat, 0.80
			}
		}
	}

	// 检查扩展名
	for _, cat := range c.categories {
		if cat.ParentID != "" {
			continue
		}

		for _, catExt := range cat.Extensions {
			if ext == catExt {
				return cat, 0.90
			}
		}
	}

	// 返回其他分类
	return c.categories["others"], 0.5
}

// classifyByContent 基于内容分类
func (c *Classifier) classifyByContent(path string, features Features) (Category, float64) {
	ext := strings.ToLower(filepath.Ext(path))
	fileName := strings.ToLower(filepath.Base(path))

	// PDF 特殊处理
	if ext == ".pdf" {
		// 检查关键词
		text := strings.ToLower(strings.Join(features.Keywords, " "))
		for _, cat := range c.categories {
			if cat.ParentID != "documents" {
				continue
			}
			for _, kw := range cat.Keywords {
				if strings.Contains(text, strings.ToLower(kw)) {
					return cat, 0.75
				}
			}
		}
	}

	// 文本文件
	if len(features.Keywords) > 0 {
		text := strings.ToLower(strings.Join(features.Keywords, " "))
		for _, cat := range c.categories {
			for _, kw := range cat.Keywords {
				if strings.Contains(text, strings.ToLower(kw)) {
					return cat, 0.70
				}
			}
		}
	}

	// 图片文件名模式
	if c.isImageExt(ext) {
		// 检查是否是截图
		if strings.Contains(fileName, "screenshot") || strings.Contains(fileName, "截图") {
			if cat, ok := c.categories["screenshots"]; ok {
				return cat, 0.85
			}
		}
		// 检查是否是照片
		if matched, _ := regexp.MatchString(`(?i)(img|photo|pic|image|照片)[_-]?\d*`, fileName); matched {
			if cat, ok := c.categories["photos"]; ok {
				return cat, 0.80
			}
		}
	}

	return c.categories["others"], 0.4
}

// isImageExt 检查是否是图片扩展名
func (c *Classifier) isImageExt(ext string) bool {
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true, ".svg": true, ".ico": true,
	}
	return imageExts[ext]
}

// applyRules 应用分类规则
func (c *Classifier) applyRules(path string, info os.FileInfo) *FileClassification {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 按优先级排序规则
	sort.Slice(c.rules, func(i, j int) bool {
		return c.rules[i].Priority > c.rules[j].Priority
	})

	for _, rule := range c.rules {
		if !rule.Enabled {
			continue
		}

		if c.matchConditions(path, info, rule.Conditions) {
			// 规则匹配
			category := c.categories["others"]
			tags := make([]Tag, 0)

			for _, action := range rule.Actions {
				switch action.Type {
				case ActionClassify:
					if cat, ok := c.categories[action.CategoryID]; ok {
						category = cat
					}
				case ActionTag:
					for _, tagName := range action.TagNames {
						for _, t := range c.tags {
							if t.Name == tagName {
								tags = append(tags, t)
							}
						}
					}
				}
			}

			// 更新规则命中计数
			c.mu.RUnlock()
			c.mu.Lock()
			rule.HitCount++
			rule.LastHit = time.Now()
			c.mu.Unlock()
			c.mu.RLock()

			return &FileClassification{
				Path:       path,
				FileName:   filepath.Base(path),
				Extension:  strings.ToLower(filepath.Ext(path)),
				Category:   category,
				Tags:       tags,
				Confidence: 1.0, // 规则匹配置信度为 1
				Size:       info.Size(),
				ModTime:    info.ModTime(),
				CreatedAt:  time.Now(),
			}
		}
	}

	return nil
}

// matchConditions 匹配条件
func (c *Classifier) matchConditions(path string, info os.FileInfo, conditions []Condition) bool {
	for _, cond := range conditions {
		var value interface{}

		switch cond.Type {
		case CondTypeFileName:
			value = filepath.Base(path)
		case CondTypeExtension:
			value = strings.ToLower(filepath.Ext(path))
		case CondTypePath:
			value = path
		case CondTypeSize:
			value = info.Size()
		case CondTypeMIME:
			// 简化处理
			value = ""
		case CondTypeTime:
			value = info.ModTime()
		default:
			continue
		}

		if !c.matchOperator(value, cond.Operator, cond.Value) {
			return false
		}
	}
	return true
}

// matchOperator 匹配操作符
func (c *Classifier) matchOperator(value interface{}, op string, target interface{}) bool {
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
	case "gt", ">":
		return c.compareNumbers(value, target) > 0
	case "lt", "<":
		return c.compareNumbers(value, target) < 0
	case "gte", ">=":
		return c.compareNumbers(value, target) >= 0
	case "lte", "<=":
		return c.compareNumbers(value, target) <= 0
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

// compareNumbers 比较数字
func (c *Classifier) compareNumbers(a, b interface{}) int {
	aFloat := toFloat64(a)
	bFloat := toFloat64(b)
	if aFloat < bFloat {
		return -1
	} else if aFloat > bFloat {
		return 1
	}
	return 0
}

// toFloat64 转换为 float64
func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	case float32:
		return float64(n)
	}
	return 0
}

// extractFeatures 提取文件特征
func (c *Classifier) extractFeatures(path string) (Features, error) {
	ext := strings.ToLower(filepath.Ext(path))
	features := Features{}

	// 获取文件大小
	info, err := os.Stat(path)
	if err != nil {
		return features, err
	}

	// 根据文件类型提取特征
	if c.isTextFile(ext) {
		return c.extractTextFeatures(path, info)
	}

	if c.isImageExt(ext) && c.config.EnableImageFeat {
		return c.extractImageFeatures(path)
	}

	return features, nil
}

// isTextFile 检查是否是文本文件
func (c *Classifier) isTextFile(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".xml": true,
		".yaml": true, ".yml": true, ".csv": true, ".log": true,
		".js": true, ".ts": true, ".py": true, ".go": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".html": true, ".css": true, ".sql": true, ".sh": true,
	}
	return textExts[ext]
}

// extractTextFeatures 提取文本特征
func (c *Classifier) extractTextFeatures(path string, info os.FileInfo) (Features, error) {
	features := Features{}

	// 限制读取大小
	maxSize := c.config.MaxContentSize
	if info.Size() < maxSize {
		maxSize = info.Size()
	}

	file, err := os.Open(path)
	if err != nil {
		return features, err
	}
	defer func() { _ = file.Close() }()

	data := make([]byte, maxSize)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return features, err
	}
	content := string(data[:n])

	// 计算行数和词数
	lines := strings.Count(content, "\n") + 1
	words := len(strings.Fields(content))
	features.LineCount = lines
	features.WordCount = words

	// 提取关键词
	features.Keywords = c.extractKeywords(content)

	return features, nil
}

// extractKeywords 提取关键词
func (c *Classifier) extractKeywords(content string) []string {
	// 简单的关键词提取（词频统计）
	words := strings.Fields(strings.ToLower(content))
	wordCount := make(map[string]int)

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "under": true, "again": true,
		"further": true, "then": true, "once": true, "here": true, "there": true,
		"when": true, "where": true, "why": true, "how": true, "all": true,
		"each": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "nor": true, "not": true,
		"only": true, "own": true, "same": true, "so": true, "than": true,
		"too": true, "very": true, "just": true, "and": true, "but": true,
		"if": true, "or": true, "because": true, "until": true, "while": true,
		"this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "what": true, "which": true, "who": true,
	}

	for _, word := range words {
		// 清理标点
		word = strings.Trim(word, ".,!?;:\"'()[]{}<>")
		if len(word) < 3 || stopWords[word] {
			continue
		}
		wordCount[word]++
	}

	// 按频率排序
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

	// 返回前 10 个关键词
	var result []string
	for i := 0; i < 10 && i < len(sorted); i++ {
		result = append(result, sorted[i].key)
	}

	return result
}

// extractImageFeatures 提取图像特征
func (c *Classifier) extractImageFeatures(path string) (Features, error) {
	features := Features{}
	// 图像特征提取需要额外的库支持
	// 这里返回空特征，实际使用时可以集成图像处理库
	return features, nil
}

// calculateHash 计算文件哈希
func (c *Classifier) calculateHash(path string) (string, error) {
	// 检查缓存
	if c.config.EnableHashCache {
		c.mu.RLock()
		if hash, ok := c.hashCache[path]; ok {
			c.mu.RUnlock()
			return hash, nil
		}
		c.mu.RUnlock()
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	// 缓存哈希
	if c.config.EnableHashCache {
		c.mu.Lock()
		c.hashCache[path] = hash
		c.mu.Unlock()
	}

	return hash, nil
}

// ClassifyBatch 批量分类
func (c *Classifier) ClassifyBatch(ctx context.Context, paths []string, concurrency int) ([]*FileClassification, error) {
	if concurrency <= 0 {
		concurrency = 4
	}

	results := make([]*FileClassification, len(paths))
	errs := make([]error, len(paths))

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i, path := range paths {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := c.Classify(ctx, p)
			results[idx] = result
			errs[idx] = err
		}(i, path)
	}

	wg.Wait()

	// 检查错误
	for i, err := range errs {
		if err != nil {
			return results, fmt.Errorf("分类 %s 失败: %w", paths[i], err)
		}
	}

	return results, nil
}

// load 加载数据
func (c *Classifier) load() error {
	dataDir := c.config.DataDir

	// 加载分类
	categoriesFile := filepath.Join(dataDir, "categories.json")
	if data, err := os.ReadFile(categoriesFile); err == nil {
		var categories []Category
		if err := json.Unmarshal(data, &categories); err == nil {
			for _, cat := range categories {
				c.categories[cat.ID] = cat
			}
		}
	}

	// 加载规则
	rulesFile := filepath.Join(dataDir, "rules.json")
	if data, err := os.ReadFile(rulesFile); err == nil {
		var rules []ClassificationRule
		if err := json.Unmarshal(data, &rules); err == nil {
			c.rules = rules
		}
	}

	// 加载标签
	tagsFile := filepath.Join(dataDir, "tags.json")
	if data, err := os.ReadFile(tagsFile); err == nil {
		var tags []Tag
		if err := json.Unmarshal(data, &tags); err == nil {
			for _, tag := range tags {
				c.tags[tag.ID] = tag
			}
		}
	}

	return nil
}

// Save 保存数据
func (c *Classifier) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.saveLocked()
}

// saveLocked 保存数据（调用者已持有锁）
func (c *Classifier) saveLocked() error {
	dataDir := c.config.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	// 保存分类
	var categories []Category
	for _, cat := range c.categories {
		categories = append(categories, cat)
	}
	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal categories: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "categories.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write categories: %w", err)
	}

	// 保存规则
	data, err = json.MarshalIndent(c.rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "rules.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write rules: %w", err)
	}

	// 保存标签
	var tags []Tag
	for _, tag := range c.tags {
		tags = append(tags, tag)
	}
	data, err = json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "tags.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write tags: %w", err)
	}

	return nil
}

// AddCategory 添加分类
func (c *Classifier) AddCategory(category Category) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	category.ID = generateID("cat")
	category.CreatedAt = time.Now()
	category.UpdatedAt = time.Now()
	c.categories[category.ID] = category

	return c.saveLocked()
}

// AddRule 添加规则
func (c *Classifier) AddRule(rule ClassificationRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	rule.ID = generateID("rule")
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	c.rules = append(c.rules, rule)

	return c.saveLocked()
}

// GetCategories 获取所有分类
func (c *Classifier) GetCategories() []Category {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []Category
	for _, cat := range c.categories {
		result = append(result, cat)
	}
	return result
}

// GetRules 获取所有规则
func (c *Classifier) GetRules() []ClassificationRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.rules
}

// generateID 生成 ID
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// generateTags 生成标签
func (c *Classifier) generateTags(path string, features Features, category Category) []Tag {
	tags := make(map[string]Tag)
	fileName := strings.ToLower(filepath.Base(path))
	_ = strings.ToLower(filepath.Ext(path)) // ext available for future use

	// 基于分类添加标签
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

	// 基于文件名模式添加标签
	tagPatterns := []struct {
		pattern string
		tagName string
	}{
		{`(?i)(invoice|发票|收据|receipt)`, "发票"},
		{`(?i)(contract|合同|协议|agreement)`, "合同"},
		{`(?i)(resume|cv|简历|履历)`, "简历"},
		{`(?i)(report|报告|报表)`, "报告"},
		{`(?i)(财务|financial|账|accounting|budget|预算)`, "财务"},
		{`(?i)(legal|法律|law|律师|court|法院)`, "法律"},
		{`(?i)(project|项目|proj)`, "项目"},
		{`(?i)(meeting|会议|会议纪要|minutes)`, "会议"},
		{`(?i)(backup|备份|bak|old|copy|副本)`, "备份"},
		{`(?i)(temp|tmp|临时|未命名|untitled)`, "临时"},
		{`(?i)(confidential|机密|secret|私密|internal|内部)`, "机密"},
	}

	for _, tp := range tagPatterns {
		if matched, _ := regexp.MatchString(tp.pattern, fileName); matched {
			tags[tp.tagName] = Tag{
				ID:   "auto_" + tp.tagName,
				Name: tp.tagName,
			}
		}
	}

	// 基于路径添加标签
	pathLower := strings.ToLower(path)
	pathKeywords := map[string]string{
		"work":     "工作",
		"工作":       "工作",
		"personal": "个人",
		"个人":       "个人",
		"download": "下载",
		"下载":       "下载",
		"backup":   "备份",
		"备份":       "备份",
		"project":  "项目",
		"项目":       "项目",
	}

	for keyword, tagName := range pathKeywords {
		if strings.Contains(pathLower, keyword) {
			tags[tagName] = Tag{
				ID:   "path_" + keyword,
				Name: tagName,
			}
		}
	}

	// 基于特征添加标签
	if features.Width > 4000 || features.Height > 4000 {
		tags["高清"] = Tag{ID: "feat_hd", Name: "高清"}
	}
	if features.Duration > 3600 {
		tags["长视频"] = Tag{ID: "feat_long", Name: "长视频"}
	} else if features.Duration > 0 && features.Duration < 60 {
		tags["短视频"] = Tag{ID: "feat_short", Name: "短视频"}
	}
	if features.LineCount > 10000 {
		tags["大型项目"] = Tag{ID: "feat_large", Name: "大型项目"}
	} else if features.LineCount > 0 && features.LineCount < 100 {
		tags["小型脚本"] = Tag{ID: "feat_script", Name: "小型脚本"}
	}

	// 转换为切片并限制数量
	result := make([]Tag, 0, len(tags))
	for _, tag := range tags {
		result = append(result, tag)
		if len(result) >= c.config.MaxTags {
			break
		}
	}

	return result
}
