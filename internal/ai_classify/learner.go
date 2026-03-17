package ai_classify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Learner 分类规则学习器
type Learner struct {
	config       Config
	classifier   *Classifier
	tagger       *Tagger
	learningData []LearningData
	ruleStats    map[string]RuleStats // 规则统计
	mu           sync.RWMutex
}

// RuleStats 规则统计
type RuleStats struct {
	RuleID      string    `json:"ruleId"`
	TotalHits   int       `json:"totalHits"`
	CorrectHits int       `json:"correctHits"`
	WrongHits   int       `json:"wrongHits"`
	Accuracy    float64   `json:"accuracy"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// NewLearner 创建学习器
func NewLearner(config Config, classifier *Classifier, tagger *Tagger) *Learner {
	return &Learner{
		config:       config,
		classifier:   classifier,
		tagger:       tagger,
		learningData: make([]LearningData, 0),
		ruleStats:    make(map[string]RuleStats),
	}
}

// LearnFromFeedback 从用户反馈学习
func (l *Learner) LearnFromFeedback(ctx context.Context, feedback UserFeedback) error {
	if !l.config.EnableLearning {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 记录学习数据
	data := LearningData{
		ID:         generateID("learn"),
		FilePath:   feedback.FilePath,
		CategoryID: feedback.CorrectCategoryID,
		TagIDs:     feedback.CorrectTagIDs,
		UserAction: feedback.Action,
		Corrected:  feedback.OriginalCategoryID != feedback.CorrectCategoryID,
		Feedback:   feedback.Comment,
		CreatedAt:  time.Now(),
	}
	l.learningData = append(l.learningData, data)

	// 分析反馈，生成新规则
	if feedback.Action == "correct" && feedback.OriginalCategoryID != feedback.CorrectCategoryID {
		if err := l.learnFromCorrection(ctx, feedback); err != nil {
			return err
		}
	}

	// 保存学习数据
	return l.saveLearningData()
}

// UserFeedback 用户反馈
type UserFeedback struct {
	FilePath           string   `json:"filePath"`
	OriginalCategoryID string   `json:"originalCategoryId"`
	CorrectCategoryID  string   `json:"correctCategoryId"`
	OriginalTagIDs     []string `json:"originalTagIds"`
	CorrectTagIDs      []string `json:"correctTagIds"`
	Action             string   `json:"action"` // correct, confirm, ignore
	Comment            string   `json:"comment,omitempty"`
}

// learnFromCorrection 从修正中学习
func (l *Learner) learnFromCorrection(ctx context.Context, feedback UserFeedback) error {
	fileName := filepath.Base(feedback.FilePath)
	ext := strings.ToLower(filepath.Ext(feedback.FilePath))

	// 分析文件名模式
	patterns := l.extractPatterns(fileName)

	// 检查是否已有类似规则
	existingRule := l.findSimilarRule(patterns, ext)

	if existingRule != nil {
		// 更新现有规则统计
		l.updateRuleStats(existingRule.ID, feedback.CorrectCategoryID == existingRule.ID)
	} else {
		// 创建新规则
		rule := ClassificationRule{
			Name:        fmt.Sprintf("从修正学习: %s", fileName),
			Description: fmt.Sprintf("用户将 %s 从 %s 修正为 %s", fileName, feedback.OriginalCategoryID, feedback.CorrectCategoryID),
			Priority:    50, // 学习规则优先级中等
			Enabled:     true,
			Conditions:  l.buildConditions(patterns, ext),
			Actions: []Action{
				{
					Type:       ActionClassify,
					CategoryID: feedback.CorrectCategoryID,
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// 添加标签动作
		if len(feedback.CorrectTagIDs) > 0 {
			rule.Actions = append(rule.Actions, Action{
				Type:     ActionTag,
				TagNames: feedback.CorrectTagIDs,
			})
		}

		if err := l.classifier.AddRule(rule); err != nil {
			return err
		}
	}

	return nil
}

// extractPatterns 提取文件名模式
func (l *Learner) extractPatterns(fileName string) []string {
	var patterns []string

	// 提取数字模式
	if containsDigits(fileName) {
		patterns = append(patterns, `\d+`)
	}

	// 提取日期模式
	datePatterns := []string{
		`\d{4}-\d{2}-\d{2}`,
		`\d{4}\d{2}\d{2}`,
		`\d{2}-\d{2}-\d{4}`,
	}
	for _, p := range datePatterns {
		if matched, _ := filepath.Match(p, fileName); matched {
			patterns = append(patterns, p)
		}
	}

	// 提取关键词
	keywords := extractKeywordsSimple(fileName)
	patterns = append(patterns, keywords...)

	return patterns
}

// buildConditions 构建规则条件
func (l *Learner) buildConditions(patterns []string, ext string) []Condition {
	var conditions []Condition

	// 扩展名条件
	if ext != "" {
		conditions = append(conditions, Condition{
			Type:     CondTypeExtension,
			Operator: "eq",
			Value:    ext,
		})
	}

	// 模式条件
	for _, pattern := range patterns {
		if isRegexPattern(pattern) {
			conditions = append(conditions, Condition{
				Type:     CondTypeFileName,
				Operator: "matches",
				Value:    pattern,
			})
		} else {
			conditions = append(conditions, Condition{
				Type:     CondTypeFileName,
				Operator: "contains",
				Value:    pattern,
			})
		}
	}

	return conditions
}

// findSimilarRule 查找相似规则
func (l *Learner) findSimilarRule(patterns []string, ext string) *ClassificationRule {
	rules := l.classifier.GetRules()

	for _, rule := range rules {
		// 检查是否有相似的扩展名条件
		hasExtCond := false
		for _, cond := range rule.Conditions {
			if cond.Type == CondTypeExtension && cond.Value == ext {
				hasExtCond = true
				break
			}
		}

		if hasExtCond {
			return &rule
		}
	}

	return nil
}

// updateRuleStats 更新规则统计
func (l *Learner) updateRuleStats(ruleID string, correct bool) {
	stats := l.ruleStats[ruleID]
	stats.RuleID = ruleID
	stats.TotalHits++
	if correct {
		stats.CorrectHits++
	} else {
		stats.WrongHits++
	}
	stats.Accuracy = float64(stats.CorrectHits) / float64(stats.TotalHits)
	stats.LastUpdated = time.Now()
	l.ruleStats[ruleID] = stats
}

// LearnFromDirectory 从目录结构学习
func (l *Learner) LearnFromDirectory(ctx context.Context, dir string, categoryID string) error {
	if !l.config.EnableLearning {
		return nil
	}

	// 分析目录名
	dirName := filepath.Base(dir)
	keywords := extractKeywordsSimple(dirName)

	// 创建基于目录的规则
	rule := ClassificationRule{
		Name:        fmt.Sprintf("目录规则: %s", dirName),
		Description: fmt.Sprintf("从目录 %s 学习的分类规则", dir),
		Priority:    30, // 目录规则优先级较低
		Enabled:     true,
		Conditions: []Condition{
			{
				Type:     CondTypePath,
				Operator: "contains",
				Value:    dir,
			},
		},
		Actions: []Action{
			{
				Type:       ActionClassify,
				CategoryID: categoryID,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 添加关键词条件
	for _, kw := range keywords {
		rule.Conditions = append(rule.Conditions, Condition{
			Type:     CondTypeFileName,
			Operator: "contains",
			Value:    kw,
		})
	}

	return l.classifier.AddRule(rule)
}

// LearnFromBatch 从批量分类学习
func (l *Learner) LearnFromBatch(ctx context.Context, results []*FileClassification, feedbacks []UserFeedback) error {
	if !l.config.EnableLearning || len(feedbacks) == 0 {
		return nil
	}

	// 分析修正模式
	corrections := make(map[string][]UserFeedback) // categoryID -> corrections
	for _, fb := range feedbacks {
		if fb.Action == "correct" {
			corrections[fb.CorrectCategoryID] = append(corrections[fb.CorrectCategoryID], fb)
		}
	}

	// 为每个有足够修正的分类生成规则
	for categoryID, fbs := range corrections {
		if len(fbs) < 3 {
			// 需要至少 3 次修正才生成规则
			continue
		}

		// 分析共同模式
		patterns := l.analyzeCorrectionPatterns(fbs)
		if len(patterns) == 0 {
			continue
		}

		// 创建规则
		rule := ClassificationRule{
			Name:        fmt.Sprintf("批量学习规则: %s", categoryID),
			Description: fmt.Sprintf("从 %d 次用户修正中学习", len(fbs)),
			Priority:    40,
			Enabled:     true,
			Conditions:  l.buildConditionsFromPatterns(patterns),
			Actions: []Action{
				{
					Type:       ActionClassify,
					CategoryID: categoryID,
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := l.classifier.AddRule(rule); err != nil {
			return err
		}
	}

	return nil
}

// analyzeCorrectionPatterns 分析修正模式
func (l *Learner) analyzeCorrectionPatterns(feedbacks []UserFeedback) []string {
	var patterns []string
	extCount := make(map[string]int)
	keywordCount := make(map[string]int)

	for _, fb := range feedbacks {
		// 统计扩展名
		ext := strings.ToLower(filepath.Ext(fb.FilePath))
		if ext != "" {
			extCount[ext]++
		}

		// 统计关键词
		fileName := filepath.Base(fb.FilePath)
		keywords := extractKeywordsSimple(fileName)
		for _, kw := range keywords {
			keywordCount[strings.ToLower(kw)]++
		}
	}

	// 找出最常见的扩展名
	for ext, count := range extCount {
		if count >= len(feedbacks)/2 {
			patterns = append(patterns, "ext:"+ext)
		}
	}

	// 找出最常见的关键词
	for kw, count := range keywordCount {
		if count >= len(feedbacks)/3 {
			patterns = append(patterns, kw)
		}
	}

	return patterns
}

// buildConditionsFromPatterns 从模式构建条件
func (l *Learner) buildConditionsFromPatterns(patterns []string) []Condition {
	var conditions []Condition

	for _, p := range patterns {
		if strings.HasPrefix(p, "ext:") {
			conditions = append(conditions, Condition{
				Type:     CondTypeExtension,
				Operator: "eq",
				Value:    strings.TrimPrefix(p, "ext:"),
			})
		} else {
			conditions = append(conditions, Condition{
				Type:     CondTypeFileName,
				Operator: "contains",
				Value:    p,
			})
		}
	}

	return conditions
}

// GetLearningStats 获取学习统计
func (l *Learner) GetLearningStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	totalCorrections := 0
	totalConfirmations := 0

	for _, data := range l.learningData {
		if data.Corrected {
			totalCorrections++
		} else {
			totalConfirmations++
		}
	}

	// 规则准确率统计
	var ruleAccuracies []map[string]interface{}
	for ruleID, stats := range l.ruleStats {
		ruleAccuracies = append(ruleAccuracies, map[string]interface{}{
			"ruleId":   ruleID,
			"accuracy": stats.Accuracy,
			"hits":     stats.TotalHits,
		})
	}

	sort.Slice(ruleAccuracies, func(i, j int) bool {
		accI, okI := ruleAccuracies[i]["accuracy"].(float64)
		accJ, okJ := ruleAccuracies[j]["accuracy"].(float64)
		if !okI {
			accI = 0
		}
		if !okJ {
			accJ = 0
		}
		return accI < accJ
	})

	return map[string]interface{}{
		"totalLearningSamples": len(l.learningData),
		"totalCorrections":     totalCorrections,
		"totalConfirmations":   totalConfirmations,
		"ruleStats":            ruleAccuracies,
	}
}

// SuggestRules 建议新规则
func (l *Learner) SuggestRules(ctx context.Context) ([]ClassificationRuleSuggestion, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var suggestions []ClassificationRuleSuggestion

	// 分析未分类或低置信度分类
	correctionsByCategory := make(map[string][]LearningData)
	for _, data := range l.learningData {
		if data.Corrected {
			correctionsByCategory[data.CategoryID] = append(correctionsByCategory[data.CategoryID], data)
		}
	}

	// 为每个分类生成建议
	for categoryID, dataList := range correctionsByCategory {
		if len(dataList) < 3 {
			continue
		}

		// 分析模式
		patterns := l.analyzeCorrectionDataPatterns(dataList)
		if len(patterns) == 0 {
			continue
		}

		suggestions = append(suggestions, ClassificationRuleSuggestion{
			CategoryID:  categoryID,
			Patterns:    patterns,
			SampleCount: len(dataList),
			Confidence:  float64(len(dataList)) / float64(len(l.learningData)),
			Description: fmt.Sprintf("基于 %d 次修正的建议", len(dataList)),
		})
	}

	// 按置信度排序
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Confidence > suggestions[j].Confidence
	})

	return suggestions, nil
}

// ClassificationRuleSuggestion 分类规则建议
type ClassificationRuleSuggestion struct {
	CategoryID  string   `json:"categoryId"`
	Patterns    []string `json:"patterns"`
	SampleCount int      `json:"sampleCount"`
	Confidence  float64  `json:"confidence"`
	Description string   `json:"description"`
}

// analyzeCorrectionDataPatterns 分析修正数据模式
func (l *Learner) analyzeCorrectionDataPatterns(dataList []LearningData) []string {
	var patterns []string
	extCount := make(map[string]int)
	keywordCount := make(map[string]int)

	for _, data := range dataList {
		ext := strings.ToLower(filepath.Ext(data.FilePath))
		if ext != "" {
			extCount[ext]++
		}

		fileName := filepath.Base(data.FilePath)
		keywords := extractKeywordsSimple(fileName)
		for _, kw := range keywords {
			keywordCount[strings.ToLower(kw)]++
		}
	}

	// 收集常见模式
	for ext, count := range extCount {
		if count >= len(dataList)/2 {
			patterns = append(patterns, "ext:"+ext)
		}
	}

	for kw, count := range keywordCount {
		if count >= len(dataList)/3 {
			patterns = append(patterns, kw)
		}
	}

	return patterns
}

// saveLearningData 保存学习数据
func (l *Learner) saveLearningData() error {
	dataDir := l.config.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	dataFile := filepath.Join(dataDir, "learning_data.json")
	jsonData, err := json.MarshalIndent(l.learningData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dataFile, jsonData, 0644)
}

// loadLearningData 加载学习数据
func (l *Learner) LoadLearningData() error {
	dataDir := l.config.DataDir
	dataFile := filepath.Join(dataDir, "learning_data.json")

	jsonData, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(jsonData, &l.learningData)
}

// OptimizeRules 优化规则
func (l *Learner) OptimizeRules(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	rules := l.classifier.GetRules()
	var optimized []ClassificationRule

	for _, rule := range rules {
		stats, hasStats := l.ruleStats[rule.ID]

		if hasStats && stats.TotalHits > 10 {
			// 如果准确率低于 50%，禁用规则
			if stats.Accuracy < 0.5 {
				rule.Enabled = false
			}
			// 如果准确率高于 90%，提高优先级
			if stats.Accuracy > 0.9 {
				rule.Priority += 10
			}
		}

		optimized = append(optimized, rule)
	}

	// 保存优化后的规则
	l.classifier.rules = optimized
	return l.classifier.Save()
}

// containsDigits 检查是否包含数字
func containsDigits(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

// isRegexPattern 检查是否是正则模式
func isRegexPattern(s string) bool {
	regexChars := []string{`\`, `(`, `)`, `[`, `]`, `{`, `}`, `|`, `^`, `$`, `*`, `+`, `?`}
	for _, c := range regexChars {
		if strings.Contains(s, c) {
			return true
		}
	}
	return false
}
