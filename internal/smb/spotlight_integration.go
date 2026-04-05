// Package smb SMB Spotlight集成
// 对标TrueNAS SMB Spotlight功能，支持macOS Spotlight搜索
// v2.403.0: macOS兼容增强版
package smb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SpotlightIntegration SMB Spotlight集成服务
// 提供macOS Spotlight搜索兼容性
type SpotlightIntegration struct {
	config      SpotlightConfig
	logger      *zap.Logger
	indexer     *Indexer
	mdquery     *MDQueryHandler
	running     bool
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// SpotlightConfig Spotlight配置
type SpotlightConfig struct {
	Enabled          bool     `json:"enabled"`           // 启用Spotlight
	SharePaths       []string `json:"sharePaths"`        // SMB共享路径
	ExcludedPaths    []string `json:"excludedPaths"`     // 排除路径
	MaxIndexSize     int64    `json:"maxIndexSize"`      // 最大索引大小(MB)
	UpdateInterval   int      `json:"updateInterval"`    // 更新间隔(秒)
	EnableContentIdx bool     `json:"enableContentIdx"`  // 内容索引
	EnableChineseSeg bool     `json:"enableChineseSeg"`  // 中文分词
	IndexerWorkers   int      `json:"indexerWorkers"`    // 索引工作线程数
	CacheSize        int      `json:"cacheSize"`         // 搜索缓存大小
}

// Indexer Spotlight索引器
type Indexer struct {
	config    SpotlightConfig
	logger    *zap.Logger
	fileIndex map[string]*FileInfo
	contentIdx map[string]*ContentInfo
	wordIndex map[string][]string // word -> paths
	mu        sync.RWMutex
	running   bool
	stats     IndexStats
}

// FileInfo 文件信息索引
type FileInfo struct {
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	ModTime      time.Time         `json:"modTime"`
	Type         string            `json:"type"`         // kMDItemContentType
	Kind         string            `json:"kind"`         // kMDItemKind
	Extension    string            `json:"extension"`
	IsDirectory  bool              `json:"isDirectory"`
	Attributes   map[string]string `json:"attributes"`   // Spotlight属性
	IndexedAt    time.Time         `json:"indexedAt"`
	Score        float64           `json:"score"`        // 搜索相关性评分
}

// ContentInfo 内容索引
type ContentInfo struct {
	Path        string   `json:"path"`
	TextContent string   `json:"textContent"`
	Keywords    []string `json:"keywords"`
	WordCount   int      `json:"wordCount"`
	Excerpt     string   `json:"excerpt"`
	Language    string   `json:"language"`
}

// IndexStats 索引统计
type IndexStats struct {
	TotalFiles    int64     `json:"totalFiles"`
	IndexedFiles  int64     `json:"indexedFiles"`
	IndexedSize   int64     `json:"indexedSize"`
	LastUpdate    time.Time `json:"lastUpdate"`
	Status        string    `json:"status"`
	Progress      float64   `json:"progress"`
}

// MDQueryHandler macOS mdquery兼容处理器
type MDQueryHandler struct {
	logger *zap.Logger
}

// SpotlightQuery Spotlight查询请求
type SpotlightQuery struct {
	Query       string            `json:"query"`       // Spotlight查询语法
	Attributes  []string          `json:"attributes"`  // 请求的属性
	Scope       []string          `json:"scope"`       // 搜索范围路径
	Limit       int               `json:"limit"`       // 结果限制
	SortBy      string            `json:"sortBy"`      // 排序字段
	SortDesc    bool              `json:"sortDesc"`    // 降序排序
	OnlyFiles   bool              `json:"onlyFiles"`   // 仅返回文件
	OnlyDirs    bool              `json:"onlyDirs"`    // 仅返回目录
	FuzzyMatch  bool              `json:"fuzzyMatch"`  // 模糊匹配
	ContentSearch bool            `json:"contentSearch"` // 内容搜索
}

// SpotlightResult Spotlight搜索结果
type SpotlightResult struct {
	Path       string            `json:"path"`
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	ModTime    time.Time         `json:"modTime"`
	Type       string            `json:"type"`
	Kind       string            `json:"kind"`
	Attributes map[string]string `json:"attributes"` // kMDItem*属性
	Score      float64           `json:"score"`
	Snippet    string            `json:"snippet"`    // 内容摘要
}

// SpotlightResponse Spotlight搜索响应
type SpotlightResponse struct {
	Query      string            `json:"query"`
	Results    []SpotlightResult `json:"results"`
	Total      int               `json:"total"`
	Took       int64             `json:"took"`       // 查询耗时(ms)
	Scope      []string          `json:"scope"`
	Attributes []string          `json:"attributes"`
	Error      string            `json:"error,omitempty"`
}

// NewSpotlightIntegration 创建Spotlight集成服务
func NewSpotlightIntegration(config SpotlightConfig, logger *zap.Logger) *SpotlightIntegration {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 设置默认值
	if config.IndexerWorkers <= 0 {
		config.IndexerWorkers = 4
	}
	if config.MaxIndexSize <= 0 {
		config.MaxIndexSize = 500 // 500MB
	}
	if config.UpdateInterval <= 0 {
		config.UpdateInterval = 300 // 5分钟
	}
	if config.CacheSize <= 0 {
		config.CacheSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SpotlightIntegration{
		config:  config,
		logger:  logger,
		indexer: NewIndexer(config, logger),
		mdquery: NewMDQueryHandler(logger),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// NewIndexer 创建索引器
func NewIndexer(config SpotlightConfig, logger *zap.Logger) *Indexer {
	return &Indexer{
		config:     config,
		logger:     logger,
		fileIndex:  make(map[string]*FileInfo),
		contentIdx: make(map[string]*ContentInfo),
		wordIndex:  make(map[string][]string),
	}
}

// NewMDQueryHandler 创建mdquery处理器
func NewMDQueryHandler(logger *zap.Logger) *MDQueryHandler {
	return &MDQueryHandler{logger: logger}
}

// Start 启动Spotlight服务
func (si *SpotlightIntegration) Start(ctx context.Context) error {
	if !si.config.Enabled {
		si.logger.Info("Spotlight集成已禁用")
		return nil
	}

	si.mu.Lock()
	si.running = true
	si.mu.Unlock()

	// 启动索引器
	if err := si.indexer.Start(ctx, si.config.SharePaths); err != nil {
		return fmt.Errorf("启动索引器失败: %w", err)
	}

	// 启动后台更新
	go si.runBackgroundUpdate(ctx)

	si.logger.Info("SMB Spotlight服务已启动",
		zap.Int("sharePaths", len(si.config.SharePaths)),
		zap.Bool("contentIndex", si.config.EnableContentIdx))

	return nil
}

// Stop 停止Spotlight服务
func (si *SpotlightIntegration) Stop() error {
	si.cancel()
	si.mu.Lock()
	si.running = false
	si.mu.Unlock()

	si.indexer.Stop()

	si.logger.Info("SMB Spotlight服务已停止")
	return nil
}

// Search 执行Spotlight搜索
func (si *SpotlightIntegration) Search(ctx context.Context, query SpotlightQuery) (*SpotlightResponse, error) {
	startTime := time.Now()

	response := &SpotlightResponse{
		Query:   query.Query,
		Results: make([]SpotlightResult, 0),
		Scope:   query.Scope,
	}

	if !si.config.Enabled {
		response.Error = "Spotlight未启用"
		return response, nil
	}

	// 解析Spotlight查询语法
	parsedQuery := si.mdquery.ParseQuery(query.Query)

	// 执行文件搜索
	files, err := si.indexer.SearchFiles(ctx, parsedQuery, query)
	if err != nil {
		response.Error = err.Error()
		return response, err
	}

	// 转换为Spotlight结果格式
	for _, file := range files {
		result := SpotlightResult{
			Path:       file.Path,
			Name:       file.Name,
			Size:       file.Size,
			ModTime:    file.ModTime,
			Type:       file.Type,
			Kind:       file.Kind,
			Attributes: si.mdquery.MapToSpotlightAttributes(file.Attributes),
			Score:      file.Score,
		}

		// 添加内容摘要
		if si.config.EnableContentIdx {
			if content, ok := si.indexer.contentIdx[file.Path]; ok {
				result.Snippet = content.Excerpt
			}
		}

		response.Results = append(response.Results, result)
	}

	response.Total = len(response.Results)
	response.Took = time.Since(startTime).Milliseconds()
	response.Attributes = query.Attributes

	si.logger.Debug("Spotlight搜索完成",
		zap.String("query", query.Query),
		zap.Int("results", response.Total),
		zap.Int64("tookMs", response.Took))

	return response, nil
}

// GetIndexStatus 获取索引状态
func (si *SpotlightIntegration) GetIndexStatus() *IndexStats {
	return si.indexer.GetStats()
}

// RebuildIndex 重建索引
func (si *SpotlightIntegration) RebuildIndex(ctx context.Context, path string) error {
	return si.indexer.BuildIndex(ctx, path)
}

// EnableForShare 为SMB共享启用Spotlight
func (si *SpotlightIntegration) EnableForShare(sharePath string) error {
	si.mu.Lock()
	defer si.mu.Unlock()

	// 添加到索引路径
	if !contains(si.config.SharePaths, sharePath) {
		si.config.SharePaths = append(si.config.SharePaths, sharePath)
	}

	si.logger.Info("已为SMB共享启用Spotlight", zap.String("path", sharePath))
	return nil
}

// DisableForShare 为SMB共享禁用Spotlight
func (si *SpotlightIntegration) DisableForShare(sharePath string) error {
	si.mu.Lock()
	defer si.mu.Unlock()

	// 从索引路径移除
	si.config.SharePaths = removeFromSlice(si.config.SharePaths, sharePath)

	// 清除该路径的索引
	si.indexer.ClearIndex(sharePath)

	si.logger.Info("已为SMB共享禁用Spotlight", zap.String("path", sharePath))
	return nil
}

// runBackgroundUpdate 后台更新任务
func (si *SpotlightIntegration) runBackgroundUpdate(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(si.config.UpdateInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			si.indexer.RefreshIndex(ctx)
		}
	}
}

// ========== 索引器方法 ==========

// Start 启动索引器
func (idx *Indexer) Start(ctx context.Context, paths []string) error {
	idx.mu.Lock()
	idx.running = true
	idx.mu.Unlock()

	// 构建初始索引
	for _, path := range paths {
		if err := idx.BuildIndex(ctx, path); err != nil {
			idx.logger.Warn("索引构建失败", zap.String("path", path), zap.Error(err))
		}
	}

	idx.stats.Status = "running"
	idx.stats.LastUpdate = time.Now()

	return nil
}

// Stop 停止索引器
func (idx *Indexer) Stop() {
	idx.mu.Lock()
	idx.running = false
	idx.mu.Unlock()
}

// BuildIndex 构建索引
func (idx *Indexer) BuildIndex(ctx context.Context, basePath string) error {
	if basePath == "" {
		return fmt.Errorf("索引路径不能为空")
	}

	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return err
	}

	idx.logger.Info("开始构建索引", zap.String("path", absPath))
	idx.stats.Status = "building"

	var filesIndexed int64
	var sizeIndexed int64

	err = idx.walkAndIndex(absPath, &filesIndexed, &sizeIndexed)
	if err != nil {
		idx.stats.Status = "error"
		return err
	}

	idx.stats.TotalFiles = filesIndexed
	idx.stats.IndexedFiles = filesIndexed
	idx.stats.IndexedSize = sizeIndexed
	idx.stats.Status = "ready"
	idx.stats.LastUpdate = time.Now()

	idx.logger.Info("索引构建完成",
		zap.Int64("files", filesIndexed),
		zap.Int64("size", sizeIndexed))

	return nil
}

// walkAndIndex 遍历并索引
func (idx *Indexer) walkAndIndex(path string, filesIndexed, sizeIndexed *int64) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil // 跳过无法访问的目录
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(path, name)

		// 跳过隐藏文件
		if strings.HasPrefix(name, ".") {
			continue
		}

		// 跳过排除路径
		for _, excluded := range idx.config.ExcludedPaths {
			if strings.HasPrefix(fullPath, excluded) {
				continue
			}
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if entry.IsDir() {
			idx.walkAndIndex(fullPath, filesIndexed, sizeIndexed)
		} else {
			// 索引文件
			idx.indexFile(fullPath, info)
			*filesIndexed++
			*sizeIndexed += info.Size()
		}
	}

	return nil
}

// indexFile 索引单个文件
func (idx *Indexer) indexFile(path string, info os.FileInfo) {
	ext := strings.ToLower(filepath.Ext(path))

	fileInfo := &FileInfo{
		Path:        path,
		Name:        info.Name(),
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		Type:        idx.getContentType(ext),
		Kind:        idx.getKind(ext),
		Extension:   ext,
		IsDirectory: false,
		Attributes:  idx.extractAttributes(path, info),
		IndexedAt:   time.Now(),
	}

	idx.mu.Lock()
	idx.fileIndex[path] = fileInfo
	idx.mu.Unlock()

	// 内容索引（可选）
	if idx.config.EnableContentIdx && idx.isTextFile(ext) {
		go idx.indexContent(path)
	}
}

// indexContent 索引文件内容
func (idx *Indexer) indexContent(path string) {
	// 限制文件大小
	maxSize := idx.config.MaxIndexSize * 1024 * 1024
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxSize {
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	text := string(content)
	words := idx.extractWords(text)

	contentInfo := &ContentInfo{
		Path:        path,
		TextContent: text,
		Keywords:    idx.extractKeywords(words),
		WordCount:   len(words),
		Excerpt:     idx.makeExcerpt(text, 200),
		Language:    idx.detectLanguage(text),
	}

	idx.mu.Lock()
	idx.contentIdx[path] = contentInfo

	// 更新词索引
	for _, word := range words {
		if len(word) >= 2 {
			idx.wordIndex[word] = append(idx.wordIndex[word], path)
		}
	}
	idx.mu.Unlock()
}

// SearchFiles 搜索文件
func (idx *Indexer) SearchFiles(ctx context.Context, query map[string]interface{}, req SpotlightQuery) ([]FileInfo, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results := make([]FileInfo, 0)

	// 查询条件
	nameFilter := ""
	if name, ok := query["name"].(string); ok {
		nameFilter = strings.ToLower(name)
	}

	typeFilter := ""
	if t, ok := query["type"].(string); ok {
		typeFilter = t
	}

	// 遍历索引
	for path, file := range idx.fileIndex {
		// 路径范围过滤
		if len(req.Scope) > 0 {
			matched := false
			for _, scope := range req.Scope {
				if strings.HasPrefix(path, scope) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 名称过滤
		if nameFilter != "" {
			if req.FuzzyMatch {
				if !strings.Contains(strings.ToLower(file.Name), nameFilter) {
					continue
				}
			} else {
				if !strings.EqualFold(file.Name, nameFilter) {
					continue
				}
			}
		}

		// 类型过滤
		if typeFilter != "" && file.Type != typeFilter {
			continue
		}

		// 仅文件/仅目录过滤
		if req.OnlyFiles && file.IsDirectory {
			continue
		}
		if req.OnlyDirs && !file.IsDirectory {
			continue
		}

		// 计算分数
		file.Score = idx.calculateScore(query, file)

		results = append(results, *file)
	}

	// 排序
	if req.SortBy != "" {
		idx.sortResults(results, req.SortBy, req.SortDesc)
	} else {
		// 默认按分数排序
		idx.sortResults(results, "score", true)
	}

	// 限制结果数
	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

// ClearIndex 清除索引
func (idx *Indexer) ClearIndex(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 清除该路径下的所有索引
	for p := range idx.fileIndex {
		if strings.HasPrefix(p, path) {
			delete(idx.fileIndex, p)
			delete(idx.contentIdx, p)
		}
	}

	// 清除词索引中的相关路径
	for word, paths := range idx.wordIndex {
		newPaths := make([]string, 0)
		for _, p := range paths {
			if !strings.HasPrefix(p, path) {
				newPaths = append(newPaths, p)
			}
		}
		idx.wordIndex[word] = newPaths
	}
}

// RefreshIndex 刷新索引
func (idx *Indexer) RefreshIndex(ctx context.Context) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	for path, file := range idx.fileIndex {
		// 检查文件是否存在
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// 文件已删除，清除索引
			go idx.ClearIndex(path)
			continue
		}

		// 检查是否需要更新
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if info.ModTime().After(file.IndexedAt) {
			// 文件已更新，重新索引
			go idx.indexFile(path, info)
		}
	}
}

// GetStats 获取统计信息
func (idx *Indexer) GetStats() *IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return &idx.stats
}

// ========== MDQuery处理器方法 ==========

// ParseQuery 解析Spotlight查询语法
// 支持: kMDItemDisplayName == "xxx", kMDItemContentType == "yyy"
func (mq *MDQueryHandler) ParseQuery(query string) map[string]interface{} {
	result := make(map[string]interface{})

	// Spotlight查询语法解析
	// 格式: attribute == value OR attribute == value

	// 简化解析：提取关键词
	parts := strings.Split(query, "OR")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// 解析属性查询
		if strings.Contains(part, "==") {
			kv := strings.SplitN(part, "==", 2)
			if len(kv) == 2 {
				attr := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])

				// 去除引号
				val = strings.Trim(val, "\"'")

				// 映射Spotlight属性到内部字段
				internalAttr := mq.mapSpotlightAttr(attr)
				if internalAttr != "" {
					result[internalAttr] = val
				}
			}
		} else {
			// 简单关键词搜索
			result["name"] = strings.Trim(part, "\"'")
		}
	}

	return result
}

// mapSpotlightAttr 映射Spotlight属性到内部字段
func (mq *MDQueryHandler) mapSpotlightAttr(attr string) string {
	attrMap := map[string]string{
		"kMDItemDisplayName":          "name",
		"kMDItemFSName":               "name",
		"kMDItemPath":                 "path",
		"kMDItemFSSize":               "size",
		"kMDItemFSCreationDate":       "created",
		"kMDItemFSContentChangeDate":  "modified",
		"kMDItemContentType":          "type",
		"kMDItemKind":                 "kind",
		"kMDItemKeywords":             "keywords",
		"kMDItemTitle":                "title",
		"kMDItemAuthors":              "author",
		"kMDItemPixelWidth":           "width",
		"kMDItemPixelHeight":          "height",
		"kMDItemDurationSeconds":      "duration",
	}

	if mapped, ok := attrMap[attr]; ok {
		return mapped
	}

	// 尝试去除前缀
	if strings.HasPrefix(attr, "kMDItem") {
		return strings.TrimPrefix(attr, "kMDItem")
	}

	return ""
}

// MapToSpotlightAttributes 将内部属性映射到Spotlight格式
func (mq *MDQueryHandler) MapToSpotlightAttributes(attrs map[string]string) map[string]string {
	result := make(map[string]string)

	reverseMap := map[string]string{
		"name":     "kMDItemDisplayName",
		"path":     "kMDItemPath",
		"size":     "kMDItemFSSize",
		"created":  "kMDItemFSCreationDate",
		"modified": "kMDItemFSContentChangeDate",
		"type":     "kMDItemContentType",
		"kind":     "kMDItemKind",
	}

	for internal, val := range attrs {
		if spot, ok := reverseMap[internal]; ok {
			result[spot] = val
		}
	}

	return result
}

// ========== 辅助方法 ==========

func (idx *Indexer) getContentType(ext string) string {
	contentTypes := map[string]string{
		".txt":  "public.plain-text",
		".md":   "public.markdown",
		".pdf":  "com.adobe.pdf",
		".doc":  "com.microsoft.word.doc",
		".docx": "org.openxmlformats.wordprocessingml.document",
		".xls":  "com.microsoft.excel.xls",
		".xlsx": "org.openxmlformats.spreadsheetml.sheet",
		".ppt":  "com.microsoft.powerpoint.ppt",
		".pptx": "org.openxmlformats.presentationml.presentation",
		".jpg":  "public.jpeg",
		".jpeg": "public.jpeg",
		".png":  "public.png",
		".gif":  "com.compuserve.gif",
		".mp4":  "public.mpeg-4",
		".mov":  "com.apple.quicktime-movie",
		".mp3":  "public.mp3",
		".zip":  "public.zip-archive",
		".tar":  "public.tar-archive",
	}

	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	return "public.item"
}

func (idx *Indexer) getKind(ext string) string {
	kinds := map[string]string{
		".txt":  "文本",
		".md":   "Markdown文档",
		".pdf":  "PDF文档",
		".doc":  "Word文档",
		".docx": "Word文档",
		".xls":  "Excel表格",
		".xlsx": "Excel表格",
		".ppt":  "PowerPoint演示",
		".pptx": "PowerPoint演示",
		".jpg":  "JPEG图像",
		".png":  "PNG图像",
		".mp4":  "MP4视频",
		".mp3":  "MP3音频",
		".zip":  "ZIP压缩文件",
		".go":   "Go源代码",
		".py":   "Python源代码",
		".js":   "JavaScript源代码",
	}

	if kind, ok := kinds[ext]; ok {
		return kind
	}
	return "文件"
}

func (idx *Indexer) extractAttributes(path string, info os.FileInfo) map[string]string {
	attrs := map[string]string{
		"name":     info.Name(),
		"size":     fmt.Sprintf("%d", info.Size()),
		"modified": info.ModTime().Format(time.RFC3339),
	}

	// 可扩展：添加更多属性提取

	return attrs
}

func (idx *Indexer) isTextFile(ext string) bool {
	textExts := []string{
		".txt", ".md", ".json", ".yaml", ".yml", ".xml",
		".html", ".css", ".js", ".ts", ".go", ".py",
		".java", ".c", ".cpp", ".sh", ".sql", ".log",
	}

	for _, te := range textExts {
		if ext == te {
			return true
		}
	}
	return false
}

func (idx *Indexer) extractWords(text string) []string {
	text = strings.ToLower(text)
	// 简化分词
	words := strings.Fields(text)

	result := make([]string, 0)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) >= 2 {
			result = append(result, word)
		}
	}
	return result
}

func (idx *Indexer) extractKeywords(words []string) []string {
	wordCount := make(map[string]int)
	for _, w := range words {
		wordCount[w]++
	}

	keywords := make([]string, 0)
	for w, c := range wordCount {
		if c >= 2 && len(w) >= 3 {
			keywords = append(keywords, w)
		}
		if len(keywords) >= 20 {
			break
		}
	}
	return keywords
}

func (idx *Indexer) makeExcerpt(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func (idx *Indexer) detectLanguage(text string) string {
	chineseCount := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		}
	}
	if chineseCount > len(text)/10 {
		return "zh"
	}
	return "en"
}

func (idx *Indexer) calculateScore(query map[string]interface{}, file *FileInfo) float64 {
	score := 0.0

	// 名称匹配
	if name, ok := query["name"].(string); ok {
		if strings.Contains(strings.ToLower(file.Name), strings.ToLower(name)) {
			score += 50
		}
		if strings.EqualFold(file.Name, name) {
			score += 30 // 精确匹配加分
		}
	}

	// 类型匹配
	if t, ok := query["type"].(string); ok {
		if file.Type == t {
			score += 20
		}
	}

	return score
}

func (idx *Indexer) sortResults(results []FileInfo, sortBy string, desc bool) {
	// 简化排序
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			var less bool
			switch sortBy {
			case "score":
				less = results[i].Score < results[j].Score
			case "name":
				less = results[i].Name < results[j].Name
			case "size":
				less = results[i].Size < results[j].Size
			case "modified":
				less = results[i].ModTime.Before(results[j].ModTime)
			default:
				less = results[i].Score < results[j].Score
			}

			if desc {
				less = !less
			}

			if less {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0)
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// GenerateSMBSpotlightConfig 生成SMB Spotlight配置
// 用于smb.conf中的spotlight配置
func GenerateSMBSpotlightConfig(enabled bool, indexPaths []string) string {
	if !enabled {
		return ""
	}

	var config strings.Builder
	config.WriteString("    spotlight = yes\n")

	if len(indexPaths) > 0 {
		config.WriteString("    spotlight indexing paths = ")
		for i, path := range indexPaths {
			if i > 0 {
				config.WriteString(", ")
			}
			config.WriteString(path)
		}
		config.WriteString("\n")
	}

	return config.String()
}

// ExportSpotlightAPI 导出Spotlight API端点
func (si *SpotlightIntegration) ExportSpotlightAPI() map[string]interface{} {
	return map[string]interface{}{
		"version":    "2.403.0",
		"enabled":    si.config.Enabled,
		"indexPaths": si.config.SharePaths,
		"status":     si.GetIndexStatus(),
		"features": []string{
			"macOS Spotlight兼容",
			"中文分词支持",
			"内容全文搜索",
			"Spotlight属性映射",
		},
	}
}