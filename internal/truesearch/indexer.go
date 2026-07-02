// Package truesearch 索引器实现
package truesearch

import (
	"crypto/sha256"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Indexer 文档索引器.
type Indexer struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *TrueSearchConfig
	idx         *invertedIndex
	docFreq     map[string]int // term -> document frequency
	totalDocs   int
	pendingDocs chan *Document
	stopChan    chan struct{}
	running     bool
	indexedAt   map[string]time.Time // docID -> index time (用于增量更新)
}

// NewIndexer 创建索引器.
func NewIndexer(logger *zap.Logger, config *TrueSearchConfig) *Indexer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultTrueSearchConfig()
	}

	return &Indexer{
		logger:      logger,
		config:      config,
		idx:         newInvertedIndex(),
		docFreq:     make(map[string]int),
		pendingDocs: make(chan *Document, config.BatchSize*2),
		stopChan:    make(chan struct{}),
		indexedAt:   make(map[string]time.Time),
	}
}

// tokenize 分词，返回小写词列表.
func (idx *Indexer) tokenize(text string) []string {
	if text == "" {
		return nil
	}

	// 将非字母数字字符替换为空格
	re := regexp.MustCompile(`[^\p{L}\p{N}]+`)
	cleaned := re.ReplaceAllString(strings.ToLower(text), " ")

	// 分词
	tokens := strings.Fields(cleaned)

	// 过滤太短的词
	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if len([]rune(t)) >= idx.config.MinTermLen {
			result = append(result, t)
		}
	}

	return result
}

// generateDocID 生成文档 ID.
func (idx *Indexer) generateDocID(path string) string {
	hash := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", hash[:16])
}

// IndexDocument 索引单个文档.
func (idx *Indexer) IndexDocument(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	// 生成 ID
	if doc.ID == "" {
		doc.ID = idx.generateDocID(doc.Path)
	}

	// 设置索引时间
	doc.IndexTime = time.Now()

	// 增量检查：如果文档未修改则跳过
	if idx.config.IncrementalMode {
		idx.mu.RLock()
		lastIndexed, exists := idx.indexedAt[doc.ID]
		idx.mu.RUnlock()

		if exists && !doc.ModTime.After(lastIndexed) {
			idx.logger.Debug("document not modified, skipping",
				zap.String("path", doc.Path))
			return nil
		}
	}

	// 更新文档频率统计
	idx.updateDocFreq(doc)

	// 存入倒排索引
	idx.idx.mu.Lock()
	idx.idx.docs[doc.ID] = doc
	idx.idx.docCount = int64(len(idx.idx.docs))
	idx.idx.mu.Unlock()

	// 更新索引时间
	idx.mu.Lock()
	idx.indexedAt[doc.ID] = doc.IndexTime
	idx.totalDocs++
	idx.mu.Unlock()

	idx.logger.Debug("document indexed",
		zap.String("id", doc.ID),
		zap.String("path", doc.Path))

	return nil
}

// updateDocFreq 更新文档频率.
func (idx *Indexer) updateDocFreq(doc *Document) {
	// 收集文件名和内容的所有词项
	nameTokens := idx.tokenize(doc.Name)
	contentTokens := idx.tokenize(doc.Content)

	// 合并并去重
	termSet := make(map[string]bool)
	for _, t := range nameTokens {
		termSet[t] = true
	}
	for _, t := range contentTokens {
		termSet[t] = true
	}

	idx.mu.Lock()
	for term := range termSet {
		idx.docFreq[term]++
	}
	idx.mu.Unlock()

	// 构建倒排列表
	namePositions := idx.buildPositions(nameTokens)
	contentPositions := idx.buildPositions(contentTokens)

	idx.idx.mu.Lock()
	defer idx.idx.mu.Unlock()

	// 处理文件名词项
	for term, positions := range namePositions {
		posting := &Posting{
			DocID:     doc.ID,
			TermFreq:  len(positions),
			Positions: positions,
		}
		idx.idx.index[term] = append(idx.idx.index[term], posting)
		idx.idx.termCount++
	}

	// 处理内容词项
	for term, positions := range contentPositions {
		posting := &Posting{
			DocID:     doc.ID,
			TermFreq:  len(positions),
			Positions: positions,
		}
		idx.idx.index[term] = append(idx.idx.index[term], posting)
		idx.idx.termCount++
	}
}

// buildPositions 构建词项位置映射.
func (idx *Indexer) buildPositions(tokens []string) map[string][]int {
	positions := make(map[string][]int)
	for i, token := range tokens {
		positions[token] = append(positions[token], i)
	}
	return positions
}

// RemoveDocument 从索引中移除文档.
func (idx *Indexer) RemoveDocument(docID string) error {
	idx.idx.mu.Lock()
	doc, exists := idx.idx.docs[docID]
	if !exists {
		idx.idx.mu.Unlock()
		return fmt.Errorf("document not found: %s", docID)
	}

	// 收集需要清理的词项
	tokens := make(map[string]bool)
	for _, t := range idx.tokenize(doc.Name) {
		tokens[t] = true
	}
	for _, t := range idx.tokenize(doc.Content) {
		tokens[t] = true
	}

	// 从倒排列表中移除
	for term := range tokens {
		postings := idx.idx.index[term]
		newPostings := make([]*Posting, 0, len(postings))
		for _, p := range postings {
			if p.DocID != docID {
				newPostings = append(newPostings, p)
			}
		}
		if len(newPostings) == 0 {
			delete(idx.idx.index, term)
		} else {
			idx.idx.index[term] = newPostings
		}
	}

	// 删除文档
	delete(idx.idx.docs, docID)
	idx.idx.docCount = int64(len(idx.idx.docs))
	idx.idx.mu.Unlock()

	// 更新索引时间
	idx.mu.Lock()
	delete(idx.indexedAt, docID)
	idx.totalDocs--
	idx.mu.Unlock()

	idx.logger.Info("document removed", zap.String("id", docID))
	return nil
}

// GetDocument 获取文档.
func (idx *Indexer) GetDocument(docID string) (*Document, error) {
	idx.idx.mu.RLock()
	defer idx.idx.mu.RUnlock()

	doc, exists := idx.idx.docs[docID]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", docID)
	}

	docCopy := *doc
	return &docCopy, nil
}

// GetPostings 获取词项的倒排列表.
func (idx *Indexer) GetPostings(term string) []*Posting {
	term = strings.ToLower(term)

	idx.idx.mu.RLock()
	defer idx.idx.mu.RUnlock()

	postings := idx.idx.index[term]
	if len(postings) == 0 {
		return nil
	}

	// 复制一份防止修改
	result := make([]*Posting, len(postings))
	copy(result, postings)
	return result
}

// calculateTFIDF 计算 TF-IDF 分数.
func (idx *Indexer) calculateTFIDF(termFreq int, docFreq int, totalDocs int) float64 {
	if totalDocs == 0 || docFreq == 0 {
		return 0
	}

	// TF: 1 + log(termFreq)
	tf := 1.0
	if termFreq > 0 {
		tf = 1.0 + math.Log(float64(termFreq))
	}

	// IDF: log(totalDocs / docFreq)
	idf := math.Log(float64(totalDocs) / float64(docFreq))

	return tf * idf
}

// GetStats 获取索引统计.
func (idx *Indexer) GetStats() *IndexStats {
	idx.idx.mu.RLock()
	docCount := idx.idx.docCount
	termCount := idx.idx.termCount
	idx.idx.mu.RUnlock()

	idx.mu.RLock()
	lastIndexed := time.Time{}
	for _, t := range idx.indexedAt {
		if t.After(lastIndexed) {
			lastIndexed = t
		}
	}
	idx.mu.RUnlock()

	return &IndexStats{
		TotalDocuments: docCount,
		TotalTerms:     termCount,
		LastUpdateTime: lastIndexed,
		Status:         IndexStatusReady,
	}
}

// ListDocuments 列出所有文档.
func (idx *Indexer) ListDocuments(limit int) []*Document {
	idx.idx.mu.RLock()
	defer idx.idx.mu.RUnlock()

	if limit <= 0 || limit > len(idx.idx.docs) {
		limit = len(idx.idx.docs)
	}

	docs := make([]*Document, 0, limit)
	for _, doc := range idx.idx.docs {
		if len(docs) >= limit {
			break
		}
		docCopy := *doc
		docs = append(docs, &docCopy)
	}

	return docs
}

// GetFileExtension 获取文件扩展名.
func GetFileExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// ClassifyFileType 根据扩展名分类文件类型.
func ClassifyFileType(ext string) FileType {
	ext = strings.ToLower(ext)
	switch ext {
	case ".txt", ".doc", ".docx", ".pdf", ".md", ".rtf", ".odt", ".xls", ".xlsx", ".ppt", ".pptx", ".csv":
		return FileTypeDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff":
		return FileTypeImage
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v":
		return FileTypeVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a":
		return FileTypeAudio
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php", ".html", ".css", ".json", ".xml", ".yaml", ".yml", ".sh", ".sql":
		return FileTypeCode
	case ".zip", ".tar", ".gz", ".rar", ".7z", ".bz2", ".xz":
		return FileTypeArchive
	default:
		return FileTypeOther
	}
}

// sortResults 排序搜索结果.
func sortResults(results []SearchResult, order SortOrder) {
	switch order {
	case SortByDate:
		sort.Slice(results, func(i, j int) bool {
			return results[i].ModTime.After(results[j].ModTime)
		})
	case SortBySize:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Size > results[j].Size
		})
	case SortByName:
		sort.Slice(results, func(i, j int) bool {
			return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
		})
	default: // SortByRelevance
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}
}
