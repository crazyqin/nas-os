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

// SimilarityDetector 相似度检测器
type SimilarityDetector struct {
	config       Config
	hashIndex    map[string][]string // hash -> file paths
	nameIndex    map[string][]string // normalized name -> file paths
	featureIndex map[string]Features // path -> features
	mu           sync.RWMutex
}

// NewSimilarityDetector 创建相似度检测器
func NewSimilarityDetector(config Config) *SimilarityDetector {
	return &SimilarityDetector{
		config:       config,
		hashIndex:    make(map[string][]string),
		nameIndex:    make(map[string][]string),
		featureIndex: make(map[string]Features),
	}
}

// DetectSimilar 检测相似文件
func (d *SimilarityDetector) DetectSimilar(ctx context.Context, path string, options ...DetectOption) ([]Similarity, error) {
	opts := &DetectOptions{
		MaxResults:   d.config.MaxSimilarFiles,
		MinScore:     d.config.SimilarityThreshold,
		CheckHash:    true,
		CheckName:    true,
		CheckContent: true,
	}

	for _, opt := range options {
		opt(opts)
	}

	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("无法访问文件: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("不支持检测目录相似度")
	}

	// 计算文件哈希
	hash, err := d.calculateFileHash(path)
	if err != nil {
		return nil, fmt.Errorf("计算哈希失败: %w", err)
	}

	// 提取特征
	features, err := d.extractFeatures(path)
	if err != nil {
		features = Features{}
	}

	var results []Similarity
	seen := make(map[string]bool)

	// 1. 检测完全相同的文件（哈希匹配）
	if opts.CheckHash {
		d.mu.RLock()
		if paths, ok := d.hashIndex[hash]; ok {
			for _, p := range paths {
				if p != path && !seen[p] {
					seen[p] = true
					results = append(results, Similarity{
						FileA:      path,
						FileB:      p,
						Score:      1.0,
						SimType:    SimTypeHash,
						Reason:     "文件内容完全相同",
						DetectedAt: time.Now(),
					})
				}
			}
		}
		d.mu.RUnlock()
	}

	// 2. 检测文件名相似的文件
	if opts.CheckName {
		nameSims := d.detectNameSimilarity(path, opts.MinScore)
		for _, sim := range nameSims {
			if !seen[sim.FileB] {
				seen[sim.FileB] = true
				results = append(results, sim)
			}
		}
	}

	// 3. 检测内容相似的文件
	if opts.CheckContent && len(features.Keywords) > 0 {
		contentSims := d.detectContentSimilarity(path, features, opts.MinScore)
		for _, sim := range contentSims {
			if !seen[sim.FileB] {
				seen[sim.FileB] = true
				results = append(results, sim)
			}
		}
	}

	// 4. 检测语义相似的文件（基于特征）
	if opts.CheckSemantic {
		semanticSims := d.detectSemanticSimilarity(path, features, opts.MinScore)
		for _, sim := range semanticSims {
			if !seen[sim.FileB] {
				seen[sim.FileB] = true
				results = append(results, sim)
			}
		}
	}

	// 排序并限制结果数量
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results, nil
}

// DetectOptions 检测选项
type DetectOptions struct {
	MaxResults    int
	MinScore      float64
	CheckHash     bool
	CheckName     bool
	CheckContent  bool
	CheckSemantic bool
}

// DetectOption 检测选项函数
type DetectOption func(*DetectOptions)

// detectNameSimilarity 检测文件名相似度
func (d *SimilarityDetector) detectNameSimilarity(path string, minScore float64) []Similarity {
	var results []Similarity
	fileName := normalizeFileName(filepath.Base(path))

	d.mu.RLock()
	defer d.mu.RUnlock()

	// 遍历名称索引
	for normName, paths := range d.nameIndex {
		if normName == "" {
			continue
		}

		score := calculateStringSimilarity(fileName, normName)
		if score >= minScore {
			for _, p := range paths {
				if p != path {
					results = append(results, Similarity{
						FileA:      path,
						FileB:      p,
						Score:      score,
						SimType:    SimTypeName,
						Reason:     fmt.Sprintf("文件名相似 (%.0f%%)", score*100),
						DetectedAt: time.Now(),
					})
				}
			}
		}
	}

	return results
}

// detectContentSimilarity 检测内容相似度
func (d *SimilarityDetector) detectContentSimilarity(path string, features Features, minScore float64) []Similarity {
	var results []Similarity

	d.mu.RLock()
	defer d.mu.RUnlock()

	// 创建关键词集合
	keywordsA := make(map[string]bool)
	for _, kw := range features.Keywords {
		keywordsA[strings.ToLower(kw)] = true
	}

	// 比较特征
	for otherPath, otherFeatures := range d.featureIndex {
		if otherPath == path || len(otherFeatures.Keywords) == 0 {
			continue
		}

		// 计算 Jaccard 相似度
		keywordsB := make(map[string]bool)
		for _, kw := range otherFeatures.Keywords {
			keywordsB[strings.ToLower(kw)] = true
		}

		score := jaccardSimilarity(keywordsA, keywordsB)
		if score >= minScore {
			results = append(results, Similarity{
				FileA:      path,
				FileB:      otherPath,
				Score:      score,
				SimType:    SimTypeContent,
				Reason:     fmt.Sprintf("内容关键词相似 (%.0f%%)", score*100),
				DetectedAt: time.Now(),
			})
		}
	}

	return results
}

// detectSemanticSimilarity 检测语义相似度
func (d *SimilarityDetector) detectSemanticSimilarity(path string, features Features, minScore float64) []Similarity {
	var results []Similarity

	d.mu.RLock()
	defer d.mu.RUnlock()

	for otherPath, otherFeatures := range d.featureIndex {
		if otherPath == path {
			continue
		}

		score := d.calculateFeatureSimilarity(features, otherFeatures)
		if score >= minScore {
			results = append(results, Similarity{
				FileA:      path,
				FileB:      otherPath,
				Score:      score,
				SimType:    SimTypeSemantic,
				Reason:     fmt.Sprintf("语义特征相似 (%.0f%%)", score*100),
				DetectedAt: time.Now(),
			})
		}
	}

	return results
}

// calculateFeatureSimilarity 计算特征相似度
func (d *SimilarityDetector) calculateFeatureSimilarity(a, b Features) float64 {
	var score float64
	var count float64

	// 比较各种特征
	if a.Width > 0 && b.Width > 0 {
		// 图像尺寸相似度
		widthDiff := float64(abs(a.Width-b.Width)) / float64(max(a.Width, b.Width))
		heightDiff := float64(abs(a.Height-b.Height)) / float64(max(a.Height, b.Height))
		score += (1 - (widthDiff+heightDiff)/2) * 0.3
		count += 0.3
	}

	if a.Duration > 0 && b.Duration > 0 {
		// 时长相似度
		durationDiff := float64(abs(a.Duration-b.Duration)) / float64(max(a.Duration, b.Duration))
		score += (1 - durationDiff) * 0.3
		count += 0.3
	}

	if a.LineCount > 0 && b.LineCount > 0 {
		// 代码行数相似度
		lineDiff := float64(abs(a.LineCount-b.LineCount)) / float64(max(a.LineCount, b.LineCount))
		score += (1 - lineDiff) * 0.2
		count += 0.2
	}

	if len(a.Keywords) > 0 && len(b.Keywords) > 0 {
		// 关键词相似度
		kwA := make(map[string]bool)
		for _, kw := range a.Keywords {
			kwA[strings.ToLower(kw)] = true
		}
		kwB := make(map[string]bool)
		for _, kw := range b.Keywords {
			kwB[strings.ToLower(kw)] = true
		}
		score += jaccardSimilarity(kwA, kwB) * 0.2
		count += 0.2
	}

	if count == 0 {
		return 0
	}

	return score / count
}

// IndexFile 索引文件
func (d *SimilarityDetector) IndexFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	// 计算哈希
	hash, err := d.calculateFileHash(path)
	if err != nil {
		return err
	}

	// 提取特征
	features, err := d.extractFeatures(path)
	if err != nil {
		features = Features{}
	}

	// 标准化文件名
	normName := normalizeFileName(filepath.Base(path))

	d.mu.Lock()
	defer d.mu.Unlock()

	// 更新哈希索引
	d.hashIndex[hash] = append(d.hashIndex[hash], path)

	// 更新名称索引
	if normName != "" {
		d.nameIndex[normName] = append(d.nameIndex[normName], path)
	}

	// 更新特征索引
	d.featureIndex[path] = features

	return nil
}

// IndexDirectory 索引目录
func (d *SimilarityDetector) IndexDirectory(ctx context.Context, dir string, recursive bool, concurrency int) error {
	var paths []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			paths = append(paths, path)
		} else if !recursive && path != dir {
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 并发索引
	if concurrency <= 0 {
		concurrency = 4
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	var indexErr error
	var errMu sync.Mutex

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := d.IndexFile(p); err != nil {
				errMu.Lock()
				if indexErr == nil {
					indexErr = fmt.Errorf("索引 %s 失败: %w", p, err)
				}
				errMu.Unlock()
			}
		}(path)
	}

	wg.Wait()
	return indexErr
}

// RemoveFromIndex 从索引中移除
func (d *SimilarityDetector) RemoveFromIndex(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 从哈希索引移除
	for hash, paths := range d.hashIndex {
		for i, p := range paths {
			if p == path {
				d.hashIndex[hash] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}

	// 从名称索引移除
	for name, paths := range d.nameIndex {
		for i, p := range paths {
			if p == path {
				d.nameIndex[name] = append(paths[:i], paths[i+1:]...)
				break
			}
		}
	}

	// 从特征索引移除
	delete(d.featureIndex, path)
}

// FindDuplicates 查找重复文件
func (d *SimilarityDetector) FindDuplicates(ctx context.Context) ([][]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var duplicates [][]string

	for _, paths := range d.hashIndex {
		if len(paths) > 1 {
			duplicates = append(duplicates, paths)
		}
	}

	return duplicates, nil
}

// GetStatistics 获取统计信息
func (d *SimilarityDetector) GetStatistics() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	duplicateGroups := 0
	duplicateFiles := 0
	for _, paths := range d.hashIndex {
		if len(paths) > 1 {
			duplicateGroups++
			duplicateFiles += len(paths)
		}
	}

	return map[string]interface{}{
		"totalFilesIndexed":  len(d.featureIndex),
		"uniqueHashes":       len(d.hashIndex),
		"uniqueNames":        len(d.nameIndex),
		"duplicateGroups":    duplicateGroups,
		"duplicateFiles":     duplicateFiles,
		"potentialDuplicates": duplicateFiles - duplicateGroups,
	}
}

// SaveIndex 保存索引
func (d *SimilarityDetector) SaveIndex(path string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	data := struct {
		HashIndex    map[string][]string
		NameIndex    map[string][]string
		FeatureIndex map[string]Features
	}{
		HashIndex:    d.hashIndex,
		NameIndex:    d.nameIndex,
		FeatureIndex: d.featureIndex,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0644)
}

// LoadIndex 加载索引
func (d *SimilarityDetector) LoadIndex(path string) error {
	jsonData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data struct {
		HashIndex    map[string][]string
		NameIndex    map[string][]string
		FeatureIndex map[string]Features
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.hashIndex = data.HashIndex
	d.nameIndex = data.NameIndex
	d.featureIndex = data.FeatureIndex

	return nil
}

// calculateFileHash 计算文件哈希
func (d *SimilarityDetector) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// extractFeatures 提取特征
func (d *SimilarityDetector) extractFeatures(path string) (Features, error) {
	ext := strings.ToLower(filepath.Ext(path))
	features := Features{}

	info, err := os.Stat(path)
	if err != nil {
		return features, err
	}

	// 文本文件
	if isTextFile(ext) {
		return d.extractTextFeatures(path, info)
	}

	return features, nil
}

// extractTextFeatures 提取文本特征
func (d *SimilarityDetector) extractTextFeatures(path string, info os.FileInfo) (Features, error) {
	features := Features{}

	maxSize := d.config.MaxContentSize
	if info.Size() < maxSize {
		maxSize = info.Size()
	}

	file, err := os.Open(path)
	if err != nil {
		return features, err
	}
	defer file.Close()

	data := make([]byte, maxSize)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return features, err
	}
	content := string(data[:n])

	features.LineCount = strings.Count(content, "\n") + 1
	features.WordCount = len(strings.Fields(content))
	features.Keywords = extractKeywordsSimple(content)

	return features, nil
}

// normalizeFileName 标准化文件名
func normalizeFileName(name string) string {
	// 移除扩展名
	ext := filepath.Ext(name)
	name = strings.TrimSuffix(name, ext)

	// 移除常见后缀
	re := regexp.MustCompile(`[-_\s]*(copy|副本|backup|备份|\d+)$`)
	name = re.ReplaceAllString(name, "")

	// 转小写
	name = strings.ToLower(name)

	// 移除特殊字符（保留字母、数字和中文）
	name = regexp.MustCompile(`[^a-z0-9\x{4e00}-\x{9fa5}]`).ReplaceAllString(name, "")

	return name
}

// calculateStringSimilarity 计算字符串相似度（Levenshtein 距离）
func calculateStringSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	aLen := len(a)
	bLen := len(b)

	if aLen == 0 || bLen == 0 {
		return 0.0
	}

	// 使用动态规划计算编辑距离
	matrix := make([][]int, aLen+1)
	for i := range matrix {
		matrix[i] = make([]int, bLen+1)
		matrix[i][0] = i
	}
	for j := 0; j <= bLen; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= aLen; i++ {
		for j := 1; j <= bLen; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}

	distance := matrix[aLen][bLen]
	maxLen := max(aLen, bLen)

	return 1.0 - float64(distance)/float64(maxLen)
}

// jaccardSimilarity 计算 Jaccard 相似度
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	intersection := 0
	for k := range a {
		if b[k] {
			intersection++
		}
	}

	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// isTextFile 检查是否是文本文件
func isTextFile(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".xml": true,
		".yaml": true, ".yml": true, ".csv": true, ".log": true,
		".js": true, ".ts": true, ".py": true, ".go": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".html": true, ".css": true, ".sql": true, ".sh": true,
	}
	return textExts[ext]
}

func min(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}