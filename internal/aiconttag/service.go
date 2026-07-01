// Package aiconttag 提供AI内容自动标签服务层
package aiconttag

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Service AI 内容自动标签服务
type Service struct {
	mu          sync.RWMutex
	config      *AutoTagConfig
	tags        map[string]*ContentTag          // tagID -> ContentTag
	rules       map[string]*TagRule             // ruleID -> TagRule
	fileTags    map[string][]*FileContentTag    // filePath -> []*FileContentTag
	tagFiles    map[string][]*FileContentTag     // tagID -> []*FileContentTag
}

// NewService 创建 AI 内容自动标签服务
func NewService(cfg *AutoTagConfig) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{
		config:   cfg,
		tags:     make(map[string]*ContentTag),
		rules:    make(map[string]*TagRule),
		fileTags: make(map[string][]*FileContentTag),
		tagFiles: make(map[string][]*FileContentTag),
	}
}

// ========== 标签管理 ==========

// CreateTag 创建标签
func (s *Service) CreateTag(ctx context.Context, req *CreateTagRequest) (*ContentTag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查同名标签
	for _, t := range s.tags {
		if t.Name == req.Name && t.Category == req.Category {
			return nil, fmt.Errorf("标签 %q (分类 %q) 已存在", req.Name, req.Category)
		}
	}

	tag := newContentTag(req)
	s.tags[tag.ID] = tag
	return tag, nil
}

// GetTag 获取标签详情
func (s *Service) GetTag(ctx context.Context, tagID string) (*ContentTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tag, ok := s.tags[tagID]
	if !ok {
		return nil, fmt.Errorf("标签不存在: %s", tagID)
	}
	return tag, nil
}

// ListTags 列出标签
func (s *Service) ListTags(ctx context.Context, category TagCategory, source TagSource) ([]*ContentTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ContentTag
	for _, tag := range s.tags {
		if category != "" && tag.Category != category {
			continue
		}
		if source != "" && tag.Source != source {
			continue
		}
		result = append(result, tag)
	}

	// 按名称排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// UpdateTag 更新标签
func (s *Service) UpdateTag(ctx context.Context, tagID, name, description, color string, synonyms []string) (*ContentTag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, ok := s.tags[tagID]
	if !ok {
		return nil, fmt.Errorf("标签不存在: %s", tagID)
	}

	if name != "" {
		// 检查新名称是否冲突
		for _, t := range s.tags {
			if t.ID != tagID && t.Name == name && t.Category == tag.Category {
				return nil, fmt.Errorf("标签 %q 已存在", name)
			}
		}
		tag.Name = name
	}
	if description != "" {
		tag.Description = description
	}
	if color != "" {
		tag.Color = color
	}
	if synonyms != nil {
		tag.Synonyms = synonyms
	}
	tag.UpdatedAt = time.Now()

	return tag, nil
}

// DeleteTag 删除标签
func (s *Service) DeleteTag(ctx context.Context, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tags[tagID]; !ok {
		return fmt.Errorf("标签不存在: %s", tagID)
	}

	// 清理文件关联
	for filePath, fts := range s.fileTags {
		var newFTs []*FileContentTag
		for _, ft := range fts {
			if ft.TagID != tagID {
				newFTs = append(newFTs, ft)
			}
		}
		s.fileTags[filePath] = newFTs
	}

	delete(s.tagFiles, tagID)
	delete(s.tags, tagID)
	return nil
}

// MergeTags 合并标签
func (s *Service) MergeTags(ctx context.Context, sourceTagID, targetTagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sourceTag, ok := s.tags[sourceTagID]
	if !ok {
		return fmt.Errorf("源标签不存在: %s", sourceTagID)
	}
	targetTag, ok := s.tags[targetTagID]
	if !ok {
		return fmt.Errorf("目标标签不存在: %s", targetTagID)
	}

	// 将源标签的文件关联转移到目标标签
	for _, ft := range s.tagFiles[sourceTagID] {
		ft.TagID = targetTagID
		ft.TagName = targetTag.Name
		s.tagFiles[targetTagID] = append(s.tagFiles[targetTagID], ft)
	}
	targetTag.FileCount += sourceTag.FileCount

	// 更新文件标签列表中的引用
	for _, fts := range s.fileTags {
		for _, ft := range fts {
			if ft.TagID == sourceTagID {
				ft.TagID = targetTagID
				ft.TagName = targetTag.Name
			}
		}
	}

	sourceTag.Status = TagStatusMerged
	delete(s.tagFiles, sourceTagID)
	return nil
}

// ========== 规则管理 ==========

// CreateRule 创建标签规则
func (s *Service) CreateRule(ctx context.Context, req *CreateRuleRequest) (*TagRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查规则名是否重复
	for _, r := range s.rules {
		if r.Name == req.Name {
			return nil, fmt.Errorf("规则 %q 已存在", req.Name)
		}
	}

	rule := newTagRule(req)
	if rule.MinConfidence == 0 {
		rule.MinConfidence = s.config.MinConfidenceThreshold
	}
	if !rule.Enabled {
		rule.Enabled = true // 默认启用
	}

	s.rules[rule.ID] = rule
	return rule, nil
}

// GetRule 获取规则
func (s *Service) GetRule(ctx context.Context, ruleID string) (*TagRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rule, ok := s.rules[ruleID]
	if !ok {
		return nil, fmt.Errorf("规则不存在: %s", ruleID)
	}
	return rule, nil
}

// ListRules 列出规则
func (s *Service) ListRules(ctx context.Context) ([]*TagRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TagRule
	for _, rule := range s.rules {
		result = append(result, rule)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority // 高优先级在前
	})
	return result, nil
}

// DeleteRule 删除规则
func (s *Service) DeleteRule(ctx context.Context, ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rules[ruleID]; !ok {
		return fmt.Errorf("规则不存在: %s", ruleID)
	}
	delete(s.rules, ruleID)
	return nil
}

// UpdateRule 更新规则
func (s *Service) UpdateRule(ctx context.Context, ruleID string, req *CreateRuleRequest) (*TagRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, ok := s.rules[ruleID]
	if !ok {
		return nil, fmt.Errorf("规则不存在: %s", ruleID)
	}

	rule.Name = req.Name
	rule.Keywords = req.Keywords
	rule.RegexPatterns = req.RegexPatterns
	rule.ApplyTagIDs = req.ApplyTagIDs
	if req.MinConfidence > 0 {
		rule.MinConfidence = req.MinConfidence
	}
	if req.Priority != 0 {
		rule.Priority = req.Priority
	}
	rule.Enabled = req.Enabled
	rule.UpdatedAt = time.Now()

	return rule, nil
}

// ========== AI 分析 ==========

// AnalyzeContent 分析文件内容并生成标签（模拟 AI 分析）
func (s *Service) AnalyzeContent(ctx context.Context, req *AnalyzeRequest) (*ContentAnalysisResult, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("AI 自动标签功能未启用")
	}

	fileType := req.FileType
	if fileType == "" {
		fileType = detectFileType(req.FilePath)
	}

	if !isSupportedFileType(fileType, s.config) {
		return nil, fmt.Errorf("不支持的文件类型: %s", fileType)
	}

	// 模拟 AI 分析
	now := time.Now()
	result := &ContentAnalysisResult{
		FilePath:      req.FilePath,
		FileType:      fileType,
		Tags:          s.simulateAnalysis(req.FilePath, fileType),
		ModelName:     s.config.ModelName,
		AnalyzedAt:    now,
	}
	result.AnalysisTimeMs = time.Since(now).Milliseconds()

	// 自动应用检测到的标签
	err := s.applyDetectedTags(ctx, req.FilePath, result.Tags)
	if err != nil {
		return result, fmt.Errorf("标签应用失败: %w", err)
	}

	return result, nil
}

// BatchAnalyzeContent 批量分析内容
func (s *Service) BatchAnalyzeContent(ctx context.Context, filePaths []string) (*BatchAnalyzeResponse, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("AI 自动标签功能未启用")
	}

	response := &BatchAnalyzeResponse{
		Total: len(filePaths),
	}

	for _, fp := range filePaths {
		req := &AnalyzeRequest{FilePath: fp}
		result, err := s.AnalyzeContent(ctx, req)
		if err != nil {
			response.Failed++
			continue
		}
		response.Results = append(response.Results, *result)
		response.Success++
	}

	return response, nil
}

// simulateAnalysis 模拟 AI 内容分析（实际应调用 AI 模型）
func (s *Service) simulateAnalysis(filePath string, fileType string) []TagDetection {
	// 根据文件类型生成不同标签检测
	var detections []TagDetection

	switch {
	case isImageType(fileType):
		detections = append(detections, TagDetection{
			Name: "图片", Category: CategoryContent, Confidence: 0.98,
		})
		// 模拟场景检测
		if strings.Contains(strings.ToLower(filePath), "sunset") || strings.Contains(strings.ToLower(filePath), "beach") {
			detections = append(detections, TagDetection{
				Name: "自然风光", Category: CategoryScene, Confidence: 0.92,
			})
		}
		if strings.Contains(strings.ToLower(filePath), "portrait") || strings.Contains(strings.ToLower(filePath), "face") {
			detections = append(detections, TagDetection{
				Name: "人物", Category: CategoryObject, Confidence: 0.95,
			})
		}
		detections = append(detections, TagDetection{
			Name: "高分辨率", Category: CategoryQuality, Confidence: 0.88,
		})

	case isTextType(fileType):
		detections = append(detections, TagDetection{
			Name: "文档", Category: CategoryContent, Confidence: 0.99,
		})
		if strings.Contains(strings.ToLower(filePath), "report") {
			detections = append(detections, TagDetection{
				Name: "报告", Category: CategoryTopic, Confidence: 0.91,
			})
		}
		detections = append(detections, TagDetection{
			Name: "可编辑", Category: CategoryQuality, Confidence: 0.85,
		})

	case isVideoType(fileType):
		detections = append(detections, TagDetection{
			Name: "视频", Category: CategoryContent, Confidence: 0.99,
		})
		detections = append(detections, TagDetection{
			Name: "动态影像", Category: CategoryScene, Confidence: 0.87,
		})

	case isAudioType(fileType):
		detections = append(detections, TagDetection{
			Name: "音频", Category: CategoryContent, Confidence: 0.98,
		})
		detections = append(detections, TagDetection{
			Name: "声音内容", Category: CategoryTopic, Confidence: 0.80,
		})
	}

	// 应用规则引擎
	if s.config.RuleEngineEnabled {
		ruleDetections := s.applyRules(filePath)
		detections = append(detections, ruleDetections...)
	}

	// 限制标签数量
	if len(detections) > s.config.MaxTagsPerFile {
		detections = detections[:s.config.MaxTagsPerFile]
	}

	return detections
}

// applyDetectedTags 将检测到的标签应用到文件
func (s *Service) applyDetectedTags(ctx context.Context, filePath string, detections []TagDetection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for _, det := range detections {
		// 检查置信度阈值
		if !meetsThreshold(det.Confidence, s.config) {
			continue
		}

		// 查找或创建标签
		tag := s.findOrCreateTagByName(det.Name, det.Category, det.Confidence)

		// 检查是否已关联
		alreadyTagged := false
		for _, ft := range s.fileTags[filePath] {
			if ft.TagID == tag.ID {
				alreadyTagged = true
				break
			}
		}
		if alreadyTagged {
			continue
		}

		autoConfirm := shouldAutoConfirm(det.Confidence, s.config)

		fct := &FileContentTag{
			FilePath:    filePath,
			TagID:       tag.ID,
			TagName:      tag.Name,
			TagCategory:  tag.Category,
			Confidence:   det.Confidence,
			Source:       SourceAI,
			AnalyzedAt:   now,
			ModelName:    s.config.ModelName,
			Confirmed:    autoConfirm,
		}

		s.fileTags[filePath] = append(s.fileTags[filePath], fct)
		s.tagFiles[tag.ID] = append(s.tagFiles[tag.ID], fct)
		tag.FileCount++

		// 自动确认则更新标签状态
		if autoConfirm {
			tag.Status = TagStatusActive
		} else {
			tag.Status = TagStatusPending
		}
	}

	return nil
}

// findOrCreateTagByName 查找或创建标签（必须在持锁状态下调用）
func (s *Service) findOrCreateTagByName(name string, category TagCategory, confidence float64) *ContentTag {
	// 检查同名同分类标签
	for _, t := range s.tags {
		if t.Name == name && t.Category == category {
			return t
		}
	}

	// 创建新标签
	now := time.Now()
	tag := &ContentTag{
		ID:              "tag_" + now.Format("010203") + name[:min(4, len(name))],
		Name:            name,
		Category:        category,
		Source:          SourceAI,
		Confidence:      confidence,
		ConfidenceLevel: confidenceLevel(confidence),
		ModelName:       s.config.ModelName,
		Status:          TagStatusPending,
		CreatedBy:       "ai-system",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.tags[tag.ID] = tag
	return tag
}

// min 返回较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// applyRules 应用规则引擎匹配
func (s *Service) applyRules(filePath string) []TagDetection {
	var detections []TagDetection

	// 获取按优先级排序的规则
	var rules []*TagRule
	for _, r := range s.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	lowerPath := strings.ToLower(filePath)
	for _, rule := range rules {
		matched := false
		// 关键词匹配
		for _, kw := range rule.Keywords {
			if strings.Contains(lowerPath, strings.ToLower(kw)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		// 应用规则关联的标签
		for _, tagID := range rule.ApplyTagIDs {
			if tag, ok := s.tags[tagID]; ok {
				detections = append(detections, TagDetection{
					Name:       tag.Name,
					Category:   tag.Category,
					Confidence: rule.MinConfidence,
				})
			}
		}
	}

	return detections
}

// ========== 手动标签操作 ==========

// ManualTag 手动为文件打标签
func (s *Service) ManualTag(ctx context.Context, req *ManualTagRequest) ([]*FileContentTag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []*FileContentTag
	now := time.Now()

	for _, tagName := range req.TagNames {
		// 查找或创建标签
		tag := s.findOrCreateTagByName(tagName, CategoryCustom, 1.0)
		tag.Source = SourceHybrid // 手动修改了 AI 标签或新建
		tag.Status = TagStatusActive

		// 检查是否已关联
		alreadyTagged := false
		for _, ft := range s.fileTags[req.FilePath] {
			if ft.TagID == tag.ID {
				alreadyTagged = true
				break
			}
		}
		if alreadyTagged {
			continue
		}

		fct := &FileContentTag{
			FilePath:    req.FilePath,
			TagID:       tag.ID,
			TagName:      tag.Name,
			TagCategory:  tag.Category,
			Confidence:   1.0,
			Source:       SourceManual,
			AnalyzedAt:   now,
			Confirmed:    true,
		}

		s.fileTags[req.FilePath] = append(s.fileTags[req.FilePath], fct)
		s.tagFiles[tag.ID] = append(s.tagFiles[tag.ID], fct)
		tag.FileCount++

		result = append(result, fct)
	}

	return result, nil
}

// ConfirmTag 确认 AI 生成的标签
func (s *Service) ConfirmTag(ctx context.Context, req *ConfirmTagRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ft := range s.fileTags[req.FilePath] {
		if ft.TagID == req.TagID {
			ft.Confirmed = true
			// 更新标签状态
			if tag, ok := s.tags[req.TagID]; ok {
				tag.Status = TagStatusActive
			}
			return nil
		}
	}
	return fmt.Errorf("未找到文件 %s 上的标签 %s", req.FilePath, req.TagID)
}

// RejectTag 拒绝 AI 生成的标签
func (s *Service) RejectTag(ctx context.Context, filePath, tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从文件标签列表中移除
	fts := s.fileTags[filePath]
	for i, ft := range fts {
		if ft.TagID == tagID {
			s.fileTags[filePath] = append(fts[:i], fts[i+1:]...)
			break
		}
	}

	// 从标签文件索引中移除
	tfs := s.tagFiles[tagID]
	for i, ft := range tfs {
		if ft.FilePath == filePath {
			s.tagFiles[tagID] = append(tfs[:i], tfs[i+1:]...)
			break
		}
	}

	// 更新标签状态
	if tag, ok := s.tags[tagID]; ok {
		tag.FileCount--
		if tag.FileCount <= 0 && tag.Source == SourceAI {
			tag.Status = TagStatusRejected
		}
	}

	return nil
}

// GetFileTags 获取文件的所有标签
func (s *Service) GetFileTags(ctx context.Context, filePath string) ([]*FileContentTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.fileTags[filePath], nil
}

// GetTagFiles 获取标签关联的所有文件
func (s *Service) GetTagFiles(ctx context.Context, tagID string) ([]*FileContentTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.tags[tagID]; !ok {
		return nil, fmt.Errorf("标签不存在: %s", tagID)
	}
	return s.tagFiles[tagID], nil
}

// ========== 搜索 ==========

// SearchByTags 按标签搜索文件
func (s *Service) SearchByTags(ctx context.Context, req *SearchByTagRequest) (*TagSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 收集匹配的标签 ID
	tagIDSet := make(map[string]bool)
	for _, tagID := range req.TagIDs {
		tagIDSet[tagID] = true
	}
	for _, tagName := range req.TagNames {
		for _, tag := range s.tags {
			if tag.Name == tagName {
				tagIDSet[tag.ID] = true
				break
			}
			// 检查同义词
			for _, syn := range tag.Synonyms {
				if syn == tagName {
					tagIDSet[tag.ID] = true
					break
				}
			}
		}
	}

	// 按分类过滤
	if req.Category != "" {
		for tid, tag := range s.tags {
			if tag.Category != req.Category {
				delete(tagIDSet, tid)
			}
		}
	}

	// 收集匹配的文件
	seen := make(map[string]bool)
	var files []FileContentTag

	for tagID := range tagIDSet {
		for _, ft := range s.tagFiles[tagID] {
			if seen[ft.FilePath] {
				continue
			}
			// 检查最小置信度
			if req.MinConfidence > 0 && ft.Confidence < req.MinConfidence {
				continue
			}
			seen[ft.FilePath] = true
			files = append(files, *ft)
		}
	}

	return &TagSearchResult{
		Files: files,
		Total: len(files),
	}, nil
}

// ========== 统计 ==========

// GetStats 获取标签统计
func (s *Service) GetStats(ctx context.Context) (*TagStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &TagStats{
		TagsByCategory:   make(map[string]int),
		TagsBySource:     make(map[string]int),
		TagsByConfidence: make(map[string]int),
	}

	uniqueFiles := make(map[string]bool)

	for _, tag := range s.tags {
		stats.TotalTags++
		stats.TagsByCategory[string(tag.Category)]++
		stats.TagsBySource[string(tag.Source)]++
		stats.TagsByConfidence[string(tag.ConfidenceLevel)]++

		if tag.Status == TagStatusPending {
			stats.PendingReview++
		}
		if tag.Status == TagStatusActive {
			stats.AutoConfirmed++
		}
	}

	for filePath := range s.fileTags {
		uniqueFiles[filePath] = true
	}
	stats.TotalFiles = len(uniqueFiles)

	stats.RuleCount = len(s.rules)
	for _, rule := range s.rules {
		if rule.Enabled {
			stats.EnabledRules++
		}
	}

	return stats, nil
}

// ========== 配置管理 ==========

// GetConfig 获取配置
func (s *Service) GetConfig() *AutoTagConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := *s.config
	return &cfg
}

// UpdateConfig 更新配置
func (s *Service) UpdateConfig(cfg *AutoTagConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// ========== 文件类型检测辅助 ==========

// detectFileType 从文件路径推断文件类型
func detectFileType(filePath string) string {
	idx := strings.LastIndex(filePath, ".")
	if idx < 0 {
		return "unknown"
	}
	return strings.ToLower(filePath[idx+1:])
}

func isImageType(ft string) bool {
	switch ft {
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff":
		return true
	}
	return false
}

func isTextType(ft string) bool {
	switch ft {
	case "txt", "pdf", "md", "doc", "docx", "rtf", "csv", "json", "xml", "html", "log":
		return true
	}
	return false
}

func isVideoType(ft string) bool {
	switch ft {
	case "mp4", "mov", "avi", "mkv", "wmv", "flv":
		return true
	}
	return false
}

func isAudioType(ft string) bool {
	switch ft {
	case "mp3", "wav", "flac", "aac", "ogg", "m4a":
		return true
	}
	return false
}