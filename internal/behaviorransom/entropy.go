// Package behaviorransom 提供基于行为分析的勒索软件检测功能
// entropy.go - 文件熵值分析器
package behaviorransom

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// EntropyAnalyzer 熵值分析器
type EntropyAnalyzer struct {
	mu           sync.RWMutex
	threshold    float64
	entropyCache map[string]float64
	stats        EntropyStats
}

// NewEntropyAnalyzer 创建新的熵值分析器
func NewEntropyAnalyzer(threshold float64) *EntropyAnalyzer {
	return &EntropyAnalyzer{
		threshold:    threshold,
		entropyCache: make(map[string]float64),
		stats: EntropyStats{
			EntropyDistribution: make(map[string]int),
		},
	}
}

// CalculateShannonEntropy 计算数据的香农熵
func (ea *EntropyAnalyzer) CalculateShannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	// 统计字节频率
	frequency := make(map[byte]int)
	for _, b := range data {
		frequency[b]++
	}

	// 计算熵
	var entropy float64
	total := float64(len(data))

	for _, count := range frequency {
		p := float64(count) / total
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// AnalyzeFile 分析单个文件的熵值
func (ea *EntropyAnalyzer) AnalyzeFile(path string) (float64, error) {
	// 检查缓存
	ea.mu.RLock()
	if cached, exists := ea.entropyCache[path]; exists {
		ea.mu.RUnlock()
		return cached, nil
	}
	ea.mu.RUnlock()

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	// 计算熵值
	entropy := ea.CalculateShannonEntropy(data)

	// 更新缓存
	ea.mu.Lock()
	ea.entropyCache[path] = entropy
	ea.updateStats(entropy)
	ea.mu.Unlock()

	return entropy, nil
}

// AnalyzeSample 分析数据样本的熵值
func (ea *EntropyAnalyzer) AnalyzeSample(data []byte) float64 {
	entropy := ea.CalculateShannonEntropy(data)

	ea.mu.Lock()
	ea.updateStats(entropy)
	ea.mu.Unlock()

	return entropy
}

// IsHighEntropy 判断熵值是否高于阈值
func (ea *EntropyAnalyzer) IsHighEntropy(entropy float64) bool {
	return entropy >= ea.threshold
}

// IsEncryptedFile 判断文件是否可能被加密
func (ea *EntropyAnalyzer) IsEncryptedFile(path string) (bool, float64, error) {
	entropy, err := ea.AnalyzeFile(path)
	if err != nil {
		return false, 0, err
	}

	return ea.IsHighEntropy(entropy), entropy, nil
}

// ScanDirectory 扫描目录中的高熵值文件
func (ea *EntropyAnalyzer) ScanDirectory(rootPath string, maxFiles int) ([]string, []float64, error) {
	var highEntropyFiles []string
	var entropies []float64
	count := 0

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if count >= maxFiles {
			return filepath.SkipDir
		}

		// 跳过小文件（小于100字节的文件熵值不可靠）
		if info.Size() < 100 {
			return nil
		}

		entropy, err := ea.AnalyzeFile(path)
		if err != nil {
			return nil
		}

		if ea.IsHighEntropy(entropy) {
			highEntropyFiles = append(highEntropyFiles, path)
			entropies = append(entropies, entropy)
		}

		count++
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return highEntropyFiles, entropies, nil
}

// GetStats 获取熵值统计信息
func (ea *EntropyAnalyzer) GetStats() EntropyStats {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.stats
}

// ClearCache 清除熵值缓存
func (ea *EntropyAnalyzer) ClearCache() {
	ea.mu.Lock()
	ea.entropyCache = make(map[string]float64)
	ea.stats = EntropyStats{
		EntropyDistribution: make(map[string]int),
	}
	ea.mu.Unlock()
}

// GetThreshold 获取阈值
func (ea *EntropyAnalyzer) GetThreshold() float64 {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.threshold
}

// SetThreshold 设置阈值
func (ea *EntropyAnalyzer) SetThreshold(threshold float64) {
	ea.mu.Lock()
	ea.threshold = threshold
	ea.mu.Unlock()
}

// DetectEntropySpike 检测熵值突增
func (ea *EntropyAnalyzer) DetectEntropySpike(path string, baselineEntropy float64) (bool, float64, error) {
	currentEntropy, err := ea.AnalyzeFile(path)
	if err != nil {
		return false, 0, err
	}

	delta := currentEntropy - baselineEntropy
	return delta >= 2.0, delta, nil
}

// updateStats 更新统计信息（需要持锁）
func (ea *EntropyAnalyzer) updateStats(entropy float64) {
	ea.stats.SampleCount++

	if ea.stats.SampleCount == 1 {
		ea.stats.MinEntropy = entropy
		ea.stats.MaxEntropy = entropy
		ea.stats.MeanEntropy = entropy
	} else {
		if entropy < ea.stats.MinEntropy {
			ea.stats.MinEntropy = entropy
		}
		if entropy > ea.stats.MaxEntropy {
			ea.stats.MaxEntropy = entropy
		}
		// 增量更新平均值
		ea.stats.MeanEntropy = ea.stats.MeanEntropy + (entropy-ea.stats.MeanEntropy)/float64(ea.stats.SampleCount)
	}

	if ea.IsHighEntropy(entropy) {
		ea.stats.HighEntropyFiles++
	}

	// 更新熵值分布
	bucket := getEntropyBucket(entropy)
	ea.stats.EntropyDistribution[bucket]++
}

// getEntropyBucket 获取熵值所在的分布桶
func getEntropyBucket(entropy float64) string {
	switch {
	case entropy < 1.0:
		return "[0, 1)"
	case entropy < 2.0:
		return "[1, 2)"
	case entropy < 3.0:
		return "[2, 3)"
	case entropy < 4.0:
		return "[3, 4)"
	case entropy < 5.0:
		return "[4, 5)"
	case entropy < 6.0:
		return "[5, 6)"
	case entropy < 7.0:
		return "[6, 7)"
	case entropy < 8.0:
		return "[7, 8)"
	default:
		return "[8, +∞)"
	}
}

// GetFileExtension 获取文件扩展名（小写）
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	return strings.ToLower(ext)
}
