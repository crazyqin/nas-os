package filetagger

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrTagNotFound      = errors.New("tag not found")
	ErrRuleNotFound     = errors.New("rule not found")
	ErrTagExists        = errors.New("tag already exists")
	ErrCircularParent   = errors.New("circular parent reference")
	ErrInvalidCondition = errors.New("invalid condition")
	ErrFileNotFound     = errors.New("file not found")
	ErrNotConfirmed     = errors.New("operation not confirmed")
)

// ========== 引擎创建 ==========

// NewEngine 创建标签引擎实例.
func NewEngine(config Config) *Engine {
	return &Engine{
		config:      config,
		tags:        make(map[string]*Tag),
		rules:       make(map[string]*compiledRule),
		fileTags:    make(map[string][]FileTag),
		tagChildren: make(map[string][]string),
	}
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ========== 标签管理 ==========

// CreateTag 创建标签.
func (e *Engine) CreateTag(name string, category FileCategory, parentID, color, icon string) (*Tag, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查同名标签
	for _, t := range e.tags {
		if t.Name == name && t.ParentID == parentID {
			return nil, ErrTagExists
		}
	}

	// 验证父标签
	if parentID != "" {
		if _, ok := e.tags[parentID]; !ok {
			return nil, ErrTagNotFound
		}
	}

	now := time.Now()
	tag := &Tag{
		ID:        generateID(),
		Name:      name,
		Category:  category,
		ParentID:  parentID,
		Color:     color,
		Icon:      icon,
		CreatedAt: now,
		UpdatedAt: now,
	}

	e.tags[tag.ID] = tag
	if parentID != "" {
		e.tagChildren[parentID] = append(e.tagChildren[parentID], tag.ID)
	}

	return tag, nil
}

// GetTag 获取标签.
func (e *Engine) GetTag(id string) (*Tag, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tag, ok := e.tags[id]
	if !ok {
		return nil, ErrTagNotFound
	}
	return tag, nil
}

// UpdateTag 更新标签.
func (e *Engine) UpdateTag(id, name, parentID, color, icon string) (*Tag, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tag, ok := e.tags[id]
	if !ok {
		return nil, ErrTagNotFound
	}

	// 检查循环引用
	if parentID != "" && parentID != tag.ParentID {
		if err := e.checkCircularRef(id, parentID); err != nil {
			return nil, err
		}
	}

	// 从旧父标签移除
	if tag.ParentID != "" && tag.ParentID != parentID {
		e.removeFromParent(tag.ParentID, id)
	}

	if name != "" {
		tag.Name = name
	}
	if parentID != tag.ParentID {
		tag.ParentID = parentID
		if parentID != "" {
			e.tagChildren[parentID] = append(e.tagChildren[parentID], id)
		}
	}
	if color != "" {
		tag.Color = color
	}
	if icon != "" {
		tag.Icon = icon
	}
	tag.UpdatedAt = time.Now()

	return tag, nil
}

// DeleteTag 删除标签及其子标签.
func (e *Engine) DeleteTag(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	tag, ok := e.tags[id]
	if !ok {
		return ErrTagNotFound
	}

	// 递归删除子标签
	for _, childID := range e.tagChildren[id] {
		_ = e.deleteTagRecursive(childID)
	}

	// 从父标签移除
	if tag.ParentID != "" {
		e.removeFromParent(tag.ParentID, id)
	}

	// 删除文件关联
	for path, tags := range e.fileTags {
		filtered := make([]FileTag, 0, len(tags))
		for _, ft := range tags {
			if ft.TagID != id {
				filtered = append(filtered, ft)
			}
		}
		e.fileTags[path] = filtered
	}

	delete(e.tagChildren, id)
	delete(e.tags, id)
	return nil
}

func (e *Engine) deleteTagRecursive(id string) error {
	for _, childID := range e.tagChildren[id] {
		_ = e.deleteTagRecursive(childID)
	}
	delete(e.tagChildren, id)
	delete(e.tags, id)
	return nil
}

func (e *Engine) removeFromParent(parentID, childID string) {
	children := e.tagChildren[parentID]
	for i, c := range children {
		if c == childID {
			e.tagChildren[parentID] = append(children[:i], children[i+1:]...)
			return
		}
	}
}

func (e *Engine) checkCircularRef(tagID, newParentID string) error {
	visited := make(map[string]bool)
	current := newParentID
	for current != "" {
		if current == tagID {
			return ErrCircularParent
		}
		if visited[current] {
			break
		}
		visited[current] = true
		tag, ok := e.tags[current]
		if !ok {
			break
		}
		current = tag.ParentID
	}
	return nil
}

// ListTags 列出所有标签.
func (e *Engine) ListTags() []Tag {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tags := make([]Tag, 0, len(e.tags))
	for _, t := range e.tags {
		tags = append(tags, *t)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	return tags
}

// GetTagTree 获取标签树（带层级关系）.
func (e *Engine) GetTagTree() []TagWithChildren {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 找到所有根标签
	var roots []string
	for _, t := range e.tags {
		if t.ParentID == "" {
			roots = append(roots, t.ID)
		}
	}
	sort.Strings(roots)

	var buildTree func(id string) TagWithChildren
	buildTree = func(id string) TagWithChildren {
		tag := e.tags[id]
		node := TagWithChildren{Tag: *tag}
		for _, childID := range e.tagChildren[id] {
			node.Children = append(node.Children, buildTree(childID))
		}
		return node
	}

	var result []TagWithChildren
	for _, id := range roots {
		result = append(result, buildTree(id))
	}
	return result
}

// GetTagAncestors 获取标签的所有祖先ID.
func (e *Engine) GetTagAncestors(tagID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var ancestors []string
	current := tagID
	for {
		tag, ok := e.tags[current]
		if !ok || tag.ParentID == "" {
			break
		}
		ancestors = append(ancestors, tag.ParentID)
		current = tag.ParentID
	}
	return ancestors
}

// ========== 文件标签操作 ==========

// AddFileTag 添加文件标签.
func (e *Engine) AddFileTag(filePath, tagID string, isAuto bool, ruleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.tags[tagID]; !ok {
		return ErrTagNotFound
	}

	tags := e.fileTags[filePath]
	// 检查是否已存在
	for _, ft := range tags {
		if ft.TagID == tagID {
			return nil // 已存在，跳过
		}
	}

	now := time.Now()
	ft := FileTag{
		FilePath:  filePath,
		TagID:     tagID,
		TagName:   e.tags[tagID].Name,
		IsAuto:    isAuto,
		RuleID:    ruleID,
		AppliedAt: now,
	}
	e.fileTags[filePath] = append(e.fileTags[filePath], ft)
	return nil
}

// RemoveFileTag 移除文件标签.
func (e *Engine) RemoveFileTag(filePath, tagID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	tags := e.fileTags[filePath]
	for i, ft := range tags {
		if ft.TagID == tagID {
			e.fileTags[filePath] = append(tags[:i], tags[i+1:]...)
			return nil
		}
	}
	return ErrTagNotFound
}

// GetFileTags 获取文件的所有标签.
func (e *Engine) GetFileTags(filePath string) FileTags {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tags := e.fileTags[filePath]
	result := FileTags{
		FilePath: filePath,
		Tags:     tags,
	}
	for _, ft := range tags {
		if ft.IsAuto {
			result.AutoTags = append(result.AutoTags, ft)
		} else {
			result.ManualTags = append(result.ManualTags, ft)
		}
	}
	return result
}

// ========== 规则引擎 ==========

// CreateRule 创建自动标签规则.
func (e *Engine) CreateRule(rule AutoRule) (*AutoRule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}

	// 编译规则
	compiled, err := e.compileRule(rule)
	if err != nil {
		return nil, err
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = rule.CreatedAt
	e.rules[rule.ID] = compiled

	return &rule, nil
}

// UpdateRule 更新规则.
func (e *Engine) UpdateRule(rule AutoRule) (*AutoRule, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[rule.ID]; !ok {
		return nil, ErrRuleNotFound
	}

	compiled, err := e.compileRule(rule)
	if err != nil {
		return nil, err
	}

	rule.UpdatedAt = time.Now()
	e.rules[rule.ID] = compiled

	return &rule, nil
}

// DeleteRule 删除规则.
func (e *Engine) DeleteRule(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rules[id]; !ok {
		return ErrRuleNotFound
	}

	delete(e.rules, id)
	return nil
}

// GetRule 获取规则.
func (e *Engine) GetRule(id string) (*AutoRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	compiled, ok := e.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	rule := compiled.rule
	return &rule, nil
}

// ListRules 列出所有规则.
func (e *Engine) ListRules() []AutoRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]AutoRule, 0, len(e.rules))
	for _, c := range e.rules {
		rules = append(rules, c.rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})
	return rules
}

// compileRule 编译规则.
func (e *Engine) compileRule(rule AutoRule) (*compiledRule, error) {
	cr := &compiledRule{rule: rule}

	if rule.Conditions.PathRegex != "" {
		re, err := regexp.Compile(rule.Conditions.PathRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid path regex: %w", err)
		}
		cr.pathRegex = re
	}

	if rule.Conditions.NameRegex != "" {
		re, err := regexp.Compile(rule.Conditions.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name regex: %w", err)
		}
		cr.nameRegex = re
	}

	for _, pattern := range rule.Conditions.PathPatterns {
		re, err := globToRegex(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		cr.pathGlobs = append(cr.pathGlobs, globPattern{pattern: pattern, regex: re})
	}

	return cr, nil
}

// MatchFile 对文件匹配所有规则，返回匹配的标签ID列表.
func (e *Engine) MatchFile(filePath string, fileSize int64, fileMIME string, contentHeader []byte) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info := fileInfo{
		path:    filePath,
		name:    filepath.Base(filePath),
		ext:     strings.ToLower(filepath.Ext(filePath)),
		size:    fileSize,
		mime:    fileMIME,
		content: contentHeader,
	}

	var matchedTagIDs []string
	// 按优先级排序执行
	rules := e.sortedRules()
	for _, cr := range rules {
		if !cr.rule.Enabled {
			continue
		}
		if e.matchCondition(&info, cr.rule.Conditions, cr) {
			matchedTagIDs = append(matchedTagIDs, cr.rule.TagIDs...)
		}
	}

	return matchedTagIDs
}

type fileInfo struct {
	path    string
	name    string
	ext     string
	size    int64
	mime    string
	content []byte
}

func (e *Engine) sortedRules() []*compiledRule {
	rules := make([]*compiledRule, 0, len(e.rules))
	for _, cr := range e.rules {
		rules = append(rules, cr)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].rule.Priority > rules[j].rule.Priority
	})
	return rules
}

func (e *Engine) matchCondition(info *fileInfo, cond Condition, cr *compiledRule) bool {
	// 组合条件: And
	if len(cond.And) > 0 {
		for _, sub := range cond.And {
			if !e.matchCondition(info, sub, cr) {
				return false
			}
		}
		return true
	}

	// 组合条件: Or
	if len(cond.Or) > 0 {
		for _, sub := range cond.Or {
			if e.matchCondition(info, sub, cr) {
				return true
			}
		}
		return false
	}

	// 组合条件: Not
	if cond.Not != nil {
		return !e.matchCondition(info, *cond.Not, cr)
	}

	matched := false

	// 扩展名匹配
	if len(cond.Extensions) > 0 {
		for _, ext := range cond.Extensions {
			if strings.EqualFold(info.ext, ext) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// MIME类型匹配
	if len(cond.MIMETypes) > 0 {
		matched = false
		for _, m := range cond.MIMETypes {
			if info.mime == m {
				matched = true
				break
			}
			// 支持通配符如 image/*
			if strings.HasSuffix(m, "/*") {
				prefix := strings.TrimSuffix(m, "/*")
				if strings.HasPrefix(info.mime, prefix+"/") {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}

	// 路径glob匹配
	if len(cr.pathGlobs) > 0 {
		matched = false
		for _, gp := range cr.pathGlobs {
			if gp.regex.MatchString(info.path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 路径正则匹配
	if cr.pathRegex != nil {
		if !cr.pathRegex.MatchString(info.path) {
			return false
		}
	}

	// 文件名正则匹配
	if cr.nameRegex != nil {
		if !cr.nameRegex.MatchString(info.name) {
			return false
		}
	}

	// 文件大小匹配
	if cond.SizeOp != "" {
		if !matchSize(info.size, cond.SizeOp, cond.SizeValue, cond.SizeValue2) {
			return false
		}
	}

	// 内容魔术字节匹配
	if len(cond.ContentMagic) > 0 {
		matched = false
		for _, magic := range cond.ContentMagic {
			if matchContentMagic(info.content, magic) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func matchSize(fileSize int64, op Operator, val, val2 int64) bool {
	switch op {
	case OpEquals:
		return fileSize == val
	case OpNotEqual:
		return fileSize != val
	case OpGreater:
		return fileSize > val
	case OpLess:
		return fileSize < val
	case OpBetween:
		return fileSize >= val && fileSize <= val2
	default:
		return false
	}
}

func matchContentMagic(content []byte, hexMagic string) bool {
	magic, err := hex.DecodeString(hexMagic)
	if err != nil || len(magic) == 0 {
		return false
	}
	if len(content) < len(magic) {
		return false
	}
	for i := range magic {
		if content[i] != magic[i] {
			return false
		}
	}
	return true
}

// ========== 智能分类 ==========

// ClassifyFile 根据扩展名和MIME分类文件.
func ClassifyFile(ext, mimeType string) FileCategory {
	ext = strings.ToLower(ext)
	mimeType = strings.ToLower(mimeType)

	// 文档
	docExts := map[string]bool{
		".doc": true, ".docx": true, ".pdf": true, ".txt": true,
		".rtf": true, ".odt": true, ".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true, ".csv": true, ".md": true,
		".epub": true, ".pages": true, ".numbers": true, ".key": true,
	}
	if docExts[ext] || strings.HasPrefix(mimeType, "application/pdf") {
		return CategoryDocument
	}

	// 图片
	imgExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".svg": true, ".webp": true, ".ico": true,
		".tiff": true, ".tif": true, ".heic": true, ".heif": true,
		".raw": true, ".cr2": true, ".nef": true, ".arw": true,
	}
	if imgExts[ext] || strings.HasPrefix(mimeType, "image/") {
		return CategoryImage
	}

	// 视频
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".ts": true, ".mpg": true, ".mpeg": true, ".3gp": true,
	}
	if videoExts[ext] || strings.HasPrefix(mimeType, "video/") {
		return CategoryVideo
	}

	// 音频
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".aac": true,
		".ogg": true, ".wma": true, ".m4a": true, ".opus": true,
		".aiff": true, ".ape": true, ".alac": true,
	}
	if audioExts[ext] || strings.HasPrefix(mimeType, "audio/") {
		return CategoryAudio
	}

	// 代码
	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".rs": true, ".rb": true, ".php": true, ".swift": true,
		".kt": true, ".scala": true, ".sh": true, ".bash": true,
		".html": true, ".css": true, ".scss": true, ".less": true,
		".sql": true, ".r": true, ".lua": true, ".perl": true,
		".yml": true, ".yaml": true, ".json": true, ".xml": true,
		".toml": true, ".ini": true, ".cfg": true, ".conf": true,
		".vue": true, ".jsx": true, ".tsx": true, ".svelte": true,
	}
	if codeExts[ext] {
		return CategoryCode
	}

	// 压缩包
	archiveExts := map[string]bool{
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".xz": true, ".7z": true, ".rar": true, ".tgz": true,
		".tar.gz": true, ".tar.bz2": true, ".tar.xz": true,
		".zst": true, ".lz4": true,
	}
	if archiveExts[ext] || strings.HasPrefix(mimeType, "application/zip") ||
		strings.HasPrefix(mimeType, "application/x-tar") ||
		strings.HasPrefix(mimeType, "application/gzip") {
		return CategoryArchive
	}

	// 数据文件
	dataExts := map[string]bool{
		".db": true, ".sqlite": true, ".sqlite3": true,
		".mdb": true, ".accdb": true, ".parquet": true,
		".avro": true, ".proto": true,
	}
	if dataExts[ext] || strings.HasPrefix(mimeType, "application/x-sqlite") {
		return CategoryData
	}

	// 字体
	fontExts := map[string]bool{
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
		".eot": true,
	}
	if fontExts[ext] || strings.HasPrefix(mimeType, "font/") {
		return CategoryFont
	}

	return CategoryOther
}

// DetectMIME 检测文件MIME类型.
func DetectMIME(filePath string) string {
	ext := filepath.Ext(filePath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType
}

// ReadContentHeader 读取文件头部字节用于内容特征匹配.
func ReadContentHeader(filePath string, readKB int) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, readKB*1024)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, err
	}
	return buf[:n], nil
}

// ========== 批量操作 ==========

// BatchApplyTags 批量给文件添加标签.
func (e *Engine) BatchApplyTags(files []string, tagID string, isAuto bool, ruleID string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.tags[tagID]; !ok {
		return 0, ErrTagNotFound
	}

	count := 0
	for _, fp := range files {
		tags := e.fileTags[fp]
		exists := false
		for _, ft := range tags {
			if ft.TagID == tagID {
				exists = true
				break
			}
		}
		if !exists {
			e.fileTags[fp] = append(e.fileTags[fp], FileTag{
				FilePath:  fp,
				TagID:     tagID,
				TagName:   e.tags[tagID].Name,
				IsAuto:    isAuto,
				RuleID:    ruleID,
				AppliedAt: time.Now(),
			})
			count++
		}
	}
	return count, nil
}

// ========== 标签统计 ==========

// GetStats 获取总体统计.
func (e *Engine) GetStats() OverallStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := OverallStats{
		TotalTags:  int64(len(e.tags)),
		TotalRules: int64(len(e.rules)),
	}

	catMap := make(map[FileCategory]*CategoryStat)
	tagFileCount := make(map[string]int64)
	tagTotalSize := make(map[string]int64)
	fileSet := make(map[string]bool)

	for path, tags := range e.fileTags {
		fileSet[path] = true
		for _, ft := range tags {
			tagFileCount[ft.TagID]++
			tag, ok := e.tags[ft.TagID]
			if ok {
				cat := tag.Category
				if _, exists := catMap[cat]; !exists {
					catMap[cat] = &CategoryStat{Category: cat}
				}
			}
		}
	}

	stats.TotalFiles = int64(len(fileSet))

	for cat, cs := range catMap {
		_ = cat
		stats.ByCategory = append(stats.ByCategory, *cs)
	}

	// Top tags
	type tagCount struct {
		id    string
		count int64
	}
	var tcList []tagCount
	for id, count := range tagFileCount {
		tcList = append(tcList, tagCount{id: id, count: count})
	}
	sort.Slice(tcList, func(i, j int) bool {
		return tcList[i].count > tcList[j].count
	})

	limit := 10
	if len(tcList) < limit {
		limit = len(tcList)
	}
	for i := 0; i < limit; i++ {
		tag, ok := e.tags[tcList[i].id]
		if ok {
			stats.TopTags = append(stats.TopTags, TagStat{
				TagID:     tag.ID,
				TagName:   tag.Name,
				Category:  tag.Category,
				FileCount: tcList[i].count,
				TotalSize: tagTotalSize[tag.ID],
			})
		}
	}

	return stats
}

// GetTagStats 获取指定标签的统计.
func (e *Engine) GetTagStats(tagID string) (*TagStat, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tag, ok := e.tags[tagID]
	if !ok {
		return nil, ErrTagNotFound
	}

	var fileCount int64
	for _, tags := range e.fileTags {
		for _, ft := range tags {
			if ft.TagID == tagID {
				fileCount++
				break
			}
		}
	}

	return &TagStat{
		TagID:     tag.ID,
		TagName:   tag.Name,
		Category:  tag.Category,
		FileCount: fileCount,
	}, nil
}

// ========== 搜索 ==========

// Search 按标签搜索文件.
func (e *Engine) Search(query SearchQuery) SearchResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}

	var results []FileTags
	for path, tags := range e.fileTags {
		ft := FileTags{
			FilePath: path,
			Tags:     tags,
		}

		if !matchSearchQuery(&ft, query) {
			continue
		}

		for _, t := range tags {
			if t.IsAuto {
				ft.AutoTags = append(ft.AutoTags, t)
			} else {
				ft.ManualTags = append(ft.ManualTags, t)
			}
		}

		results = append(results, ft)
	}

	// 排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].FilePath < results[j].FilePath
	})

	total := int64(len(results))
	start := (query.Page - 1) * query.PageSize
	if start >= len(results) {
		return SearchResult{
			Files:    []FileTags{},
			Total:    total,
			Page:     query.Page,
			PageSize: query.PageSize,
		}
	}

	end := start + query.PageSize
	if end > len(results) {
		end = len(results)
	}

	return SearchResult{
		Files:    results[start:end],
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}
}

func matchSearchQuery(ft *FileTags, q SearchQuery) bool {
	tagIDs := make(map[string]bool)
	for _, t := range ft.Tags {
		tagIDs[t.TagID] = true
	}

	// AND 逻辑的标签
	for _, id := range q.Tags {
		if !tagIDs[id] {
			return false
		}
	}

	// OR 逻辑的标签
	if len(q.AnyTags) > 0 {
		found := false
		for _, id := range q.AnyTags {
			if tagIDs[id] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 排除标签
	for _, id := range q.ExcludeTags {
		if tagIDs[id] {
			return false
		}
	}

	// 自动/手动标签过滤
	if q.IsAuto != nil {
		hasAuto := false
		hasManual := false
		for _, t := range ft.Tags {
			if t.IsAuto {
				hasAuto = true
			} else {
				hasManual = true
			}
		}
		if *q.IsAuto && !hasAuto {
			return false
		}
		if !*q.IsAuto && !hasManual {
			return false
		}
	}

	return true
}

// ========== 导入导出 ==========

// Export 导出所有标签和规则.
func (e *Engine) Export() ExportData {
	e.mu.RLock()
	defer e.mu.RUnlock()

	data := ExportData{
		Version:    "1.0",
		ExportedAt: time.Now(),
	}

	for _, t := range e.tags {
		data.Tags = append(data.Tags, *t)
	}
	for _, cr := range e.rules {
		data.Rules = append(data.Rules, cr.rule)
	}
	for _, tags := range e.fileTags {
		data.FileTags = append(data.FileTags, tags...)
	}

	return data
}

// Import 导入标签和规则.
func (e *Engine) Import(data ExportData, overwrite bool) (tagsImported, rulesImported, fileTagsImported int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 导入标签
	for _, tag := range data.Tags {
		if _, exists := e.tags[tag.ID]; exists && !overwrite {
			continue
		}
		t := tag
		e.tags[tag.ID] = &t
		if tag.ParentID != "" {
			e.tagChildren[tag.ParentID] = append(e.tagChildren[tag.ParentID], tag.ID)
		}
		tagsImported++
	}

	// 导入规则
	for _, rule := range data.Rules {
		if _, exists := e.rules[rule.ID]; exists && !overwrite {
			continue
		}
		compiled, compErr := e.compileRule(rule)
		if compErr != nil {
			continue
		}
		e.rules[rule.ID] = compiled
		rulesImported++
	}

	// 导入文件标签
	for _, ft := range data.FileTags {
		tags := e.fileTags[ft.FilePath]
		exists := false
		for _, existing := range tags {
			if existing.TagID == ft.TagID {
				exists = true
				break
			}
		}
		if !exists {
			e.fileTags[ft.FilePath] = append(e.fileTags[ft.FilePath], ft)
			fileTagsImported++
		}
	}

	return tagsImported, rulesImported, fileTagsImported, nil
}

// ExportJSON 导出为JSON.
func (e *Engine) ExportJSON() ([]byte, error) {
	data := e.Export()
	return json.MarshalIndent(data, "", "  ")
}

// ImportJSON 从JSON导入.
func (e *Engine) ImportJSON(jsonData []byte, overwrite bool) (int, int, int, error) {
	var data ExportData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid JSON: %w", err)
	}
	return e.Import(data, overwrite)
}

// ========== glob工具函数 ==========

// globToRegex 将glob模式转换为正则表达式.
func globToRegex(pattern string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("^")

	i := 0
	for i < len(pattern) {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** 匹配任意路径
				sb.WriteString(".*")
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					i += 3 // 跳过 **/
				} else {
					i += 2 // 跳过 **
				}
			} else {
				// * 匹配单层
				sb.WriteString("[^/]*")
				i++
			}
		case '?':
			sb.WriteString("[^/]")
			i++
		case '[':
			sb.WriteString("[")
			i++
		case ']':
			sb.WriteString("]")
			i++
		default:
			sb.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}

	sb.WriteString("$")
	return regexp.Compile(sb.String())
}
