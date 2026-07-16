package smartarchive

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// CompressionManager 智能压缩管理器.
type CompressionManager struct {
	mu sync.RWMutex

	// 压缩配置文件
	profiles map[string]*CompressionProfile

	// 文件类型映射
	typeMapping map[string]CompressionAlgorithm

	// 统计
	stats *CompressionStats
}

// CompressionStats 压缩统计.
type CompressionStats struct {
	TotalCompressed     int64                                    `json:"totalCompressed"`
	TotalOriginal       int64                                    `json:"totalOriginal"`
	TotalCompressedSize int64                                    `json:"totalCompressedSize"`
	AvgRatio            float64                                  `json:"avgRatio"`
	ByAlgorithm         map[CompressionAlgorithm]*AlgorithmStats `json:"byAlgorithm"`
	ByFileType          map[string]*FileTypeStats                `json:"byFileType"`
}

// AlgorithmStats 算法统计.
type AlgorithmStats struct {
	Algorithm      CompressionAlgorithm `json:"algorithm"`
	Count          int64                `json:"count"`
	OriginalSize   int64                `json:"originalSize"`
	CompressedSize int64                `json:"compressedSize"`
	AvgRatio       float64              `json:"avgRatio"`
	AvgSpeed       float64              `json:"avgSpeed"` // MB/s
}

// FileTypeStats 文件类型统计.
type FileTypeStats struct {
	Extension      string  `json:"extension"`
	Count          int64   `json:"count"`
	OriginalSize   int64   `json:"originalSize"`
	CompressedSize int64   `json:"compressedSize"`
	AvgRatio       float64 `json:"avgRatio"`
	BestAlgorithm  string  `json:"bestAlgorithm"`
}

// CompressionResult 压缩结果.
type CompressionResult struct {
	Algorithm      CompressionAlgorithm `json:"algorithm"`
	Level          int                  `json:"level"`
	OriginalSize   int64                `json:"originalSize"`
	CompressedSize int64                `json:"compressedSize"`
	Ratio          float64              `json:"ratio"`
	Speed          float64              `json:"speed"`    // MB/s
	Duration       int64                `json:"duration"` // 毫秒
	SpaceSaved     int64                `json:"spaceSaved"`
}

// NewCompressionManager 创建压缩管理器.
func NewCompressionManager() *CompressionManager {
	cm := &CompressionManager{
		profiles:    make(map[string]*CompressionProfile),
		typeMapping: make(map[string]CompressionAlgorithm),
		stats: &CompressionStats{
			ByAlgorithm: make(map[CompressionAlgorithm]*AlgorithmStats),
			ByFileType:  make(map[string]*FileTypeStats),
		},
	}

	// 初始化默认配置
	cm.initDefaults()

	return cm
}

// initDefaults 初始化默认配置.
func (cm *CompressionManager) initDefaults() {
	// 默认压缩配置文件
	cm.profiles["text"] = &CompressionProfile{
		Algorithm:  CompressionBrotli,
		Level:      6,
		MinSize:    1024, // 1KB
		Extensions: []string{".txt", ".log", ".csv", ".json", ".xml", ".html", ".css", ".js", ".md", ".yaml", ".yml", ".ini", ".conf"},
		SpeedScore: 60,
		RatioScore: 90,
		CPUCost:    0.7,
	}

	cm.profiles["document"] = &CompressionProfile{
		Algorithm:  CompressionZstd,
		Level:      3,
		MinSize:    10240, // 10KB
		Extensions: []string{".doc", ".docx", ".pdf", ".xlsx", ".pptx", ".odt", ".ods", ".odp"},
		SpeedScore: 80,
		RatioScore: 70,
		CPUCost:    0.5,
	}

	cm.profiles["image"] = &CompressionProfile{
		Algorithm:  CompressionNone,
		Level:      0,
		MinSize:    0,
		Extensions: []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff"},
		SpeedScore: 100,
		RatioScore: 0,
		CPUCost:    0,
	}

	cm.profiles["video"] = &CompressionProfile{
		Algorithm:  CompressionNone,
		Level:      0,
		MinSize:    0,
		Extensions: []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v"},
		SpeedScore: 100,
		RatioScore: 0,
		CPUCost:    0,
	}

	cm.profiles["audio"] = &CompressionProfile{
		Algorithm:  CompressionNone,
		Level:      0,
		MinSize:    0,
		Extensions: []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a"},
		SpeedScore: 100,
		RatioScore: 0,
		CPUCost:    0,
	}

	cm.profiles["archive"] = &CompressionProfile{
		Algorithm:  CompressionNone,
		Level:      0,
		MinSize:    0,
		Extensions: []string{".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz"},
		SpeedScore: 100,
		RatioScore: 0,
		CPUCost:    0,
	}

	cm.profiles["database"] = &CompressionProfile{
		Algorithm:  CompressionZstd,
		Level:      5,
		MinSize:    102400, // 100KB
		Extensions: []string{".db", ".sqlite", ".sql", ".mdb", ".accdb"},
		SpeedScore: 70,
		RatioScore: 80,
		CPUCost:    0.6,
	}

	cm.profiles["binary"] = &CompressionProfile{
		Algorithm:  CompressionLZ4,
		Level:      1,
		MinSize:    4096, // 4KB
		Extensions: []string{".exe", ".dll", ".so", ".dylib", ".bin", ".dat"},
		SpeedScore: 95,
		RatioScore: 50,
		CPUCost:    0.3,
	}

	cm.profiles["backup"] = &CompressionProfile{
		Algorithm:  CompressionXZ,
		Level:      6,
		MinSize:    1048576, // 1MB
		Extensions: []string{".bak", ".backup", ".dump", ".img", ".iso"},
		SpeedScore: 40,
		RatioScore: 95,
		CPUCost:    0.9,
	}

	cm.profiles["default"] = &CompressionProfile{
		Algorithm:  CompressionGzip,
		Level:      5,
		MinSize:    4096,
		Extensions: []string{},
		SpeedScore: 70,
		RatioScore: 65,
		CPUCost:    0.5,
	}

	// 构建类型映射
	for _, profile := range cm.profiles {
		for _, ext := range profile.Extensions {
			cm.typeMapping[ext] = profile.Algorithm
		}
	}
}

// SelectAlgorithm 智能选择压缩算法.
func (cm *CompressionManager) SelectAlgorithm(filePath string, fileSize int64, mimeType string) *CompressionProfile {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 获取文件扩展名
	ext := getFileExtension(filePath)

	// 查找匹配的配置
	for _, profile := range cm.profiles {
		for _, profileExt := range profile.Extensions {
			if ext == profileExt || ext == "."+profileExt {
				// 检查最小文件大小
				if fileSize < profile.MinSize {
					return cm.profiles["default"]
				}
				return profile
			}
		}
	}

	// 根据 MIME 类型查找
	if mimeType != "" {
		profile := cm.selectByMime(mimeType)
		if profile != nil {
			return profile
		}
	}

	// 返回默认配置
	return cm.profiles["default"]
}

// selectByMime 根据 MIME 类型选择.
func (cm *CompressionManager) selectByMime(mimeType string) *CompressionProfile {
	switch {
	case strings.HasPrefix(mimeType, "text/"):
		return cm.profiles["text"]
	case strings.HasPrefix(mimeType, "image/"):
		return cm.profiles["image"]
	case strings.HasPrefix(mimeType, "video/"):
		return cm.profiles["video"]
	case strings.HasPrefix(mimeType, "audio/"):
		return cm.profiles["audio"]
	case strings.HasPrefix(mimeType, "application/pdf"):
		return cm.profiles["document"]
	case strings.HasPrefix(mimeType, "application/zip") ||
		strings.HasPrefix(mimeType, "application/x-rar"):
		return cm.profiles["archive"]
	default:
		return nil
	}
}

// GetOptimalLevel 获取最优压缩级别.
func (cm *CompressionManager) GetOptimalLevel(algorithm CompressionAlgorithm, priority string) int {
	switch algorithm {
	case CompressionGzip:
		switch priority {
		case "speed":
			return 1
		case "balanced":
			return 5
		case "ratio":
			return 9
		default:
			return 5
		}
	case CompressionZstd:
		switch priority {
		case "speed":
			return 1
		case "balanced":
			return 3
		case "ratio":
			return 9
		default:
			return 3
		}
	case CompressionLZ4:
		return 1 // LZ4 只有一个级别
	case CompressionBrotli:
		switch priority {
		case "speed":
			return 1
		case "balanced":
			return 6
		case "ratio":
			return 11
		default:
			return 6
		}
	case CompressionXZ:
		switch priority {
		case "speed":
			return 1
		case "balanced":
			return 6
		case "ratio":
			return 9
		default:
			return 6
		}
	default:
		return 0
	}
}

// EstimateCompression 估算压缩效果.
func (cm *CompressionManager) EstimateCompression(filePath string, fileSize int64) *CompressionEstimate {
	profile := cm.SelectAlgorithm(filePath, fileSize, "")

	if profile.Algorithm == CompressionNone {
		return &CompressionEstimate{
			Algorithm:       CompressionNone,
			EstimatedSize:   fileSize,
			EstimatedRatio:  1.0,
			EstimatedSaving: 0,
			Recommendation:  "文件已压缩或不适合压缩",
		}
	}

	// 估算压缩率（基于经验值）
	estimatedRatio := cm.estimateRatio(profile.Algorithm, filePath, fileSize)
	estimatedSize := int64(float64(fileSize) * estimatedRatio)

	return &CompressionEstimate{
		Algorithm:       profile.Algorithm,
		Level:           profile.Level,
		EstimatedSize:   estimatedSize,
		EstimatedRatio:  estimatedRatio,
		EstimatedSaving: fileSize - estimatedSize,
		Recommendation:  cm.generateRecommendation(profile, estimatedRatio),
	}
}

// CompressionEstimate 压缩估算.
type CompressionEstimate struct {
	Algorithm       CompressionAlgorithm `json:"algorithm"`
	Level           int                  `json:"level"`
	EstimatedSize   int64                `json:"estimatedSize"`
	EstimatedRatio  float64              `json:"estimatedRatio"`
	EstimatedSaving int64                `json:"estimatedSaving"`
	Recommendation  string               `json:"recommendation"`
}

// estimateRatio 估算压缩率.
func (cm *CompressionManager) estimateRatio(algorithm CompressionAlgorithm, filePath string, fileSize int64) float64 {
	ext := getFileExtension(filePath)

	// 基于算法和文件类型的估算压缩率
	switch algorithm {
	case CompressionGzip:
		switch {
		case isTextFile(ext):
			return 0.3 // 70% 压缩
		case isDocumentFile(ext):
			return 0.7 // 30% 压缩
		default:
			return 0.6
		}
	case CompressionZstd:
		switch {
		case isTextFile(ext):
			return 0.25
		case isDocumentFile(ext):
			return 0.65
		default:
			return 0.55
		}
	case CompressionLZ4:
		return 0.6 // LZ4 压缩率较低但速度快
	case CompressionBrotli:
		switch {
		case isTextFile(ext):
			return 0.2 // 80% 压缩
		default:
			return 0.5
		}
	case CompressionXZ:
		switch {
		case isTextFile(ext):
			return 0.15
		default:
			return 0.4
		}
	default:
		return 0.5
	}
}

// generateRecommendation 生成推荐.
func (cm *CompressionManager) generateRecommendation(profile *CompressionProfile, ratio float64) string {
	switch {
	case ratio < 0.2:
		return fmt.Sprintf("推荐使用 %s，预计压缩率 %.0f%%，效果优秀", profile.Algorithm, (1-ratio)*100)
	case ratio < 0.5:
		return fmt.Sprintf("推荐使用 %s，预计压缩率 %.0f%%，效果良好", profile.Algorithm, (1-ratio)*100)
	case ratio < 0.8:
		return fmt.Sprintf("推荐使用 %s，预计压缩率 %.0f%%，效果一般", profile.Algorithm, (1-ratio)*100)
	default:
		return fmt.Sprintf("使用 %s，预计压缩率 %.0f%%，效果有限", profile.Algorithm, (1-ratio)*100)
	}
}

// RecordCompression 记录压缩结果.
func (cm *CompressionManager) RecordCompression(result *CompressionResult, filePath string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.stats.TotalCompressed++
	cm.stats.TotalOriginal += result.OriginalSize
	cm.stats.TotalCompressedSize += result.CompressedSize

	// 更新算法统计
	algoStats, exists := cm.stats.ByAlgorithm[result.Algorithm]
	if !exists {
		algoStats = &AlgorithmStats{
			Algorithm: result.Algorithm,
		}
		cm.stats.ByAlgorithm[result.Algorithm] = algoStats
	}
	algoStats.Count++
	algoStats.OriginalSize += result.OriginalSize
	algoStats.CompressedSize += result.CompressedSize
	algoStats.AvgRatio = float64(algoStats.CompressedSize) / float64(algoStats.OriginalSize)

	// 更新文件类型统计
	ext := getFileExtension(filePath)
	typeStats, exists := cm.stats.ByFileType[ext]
	if !exists {
		typeStats = &FileTypeStats{
			Extension: ext,
		}
		cm.stats.ByFileType[ext] = typeStats
	}
	typeStats.Count++
	typeStats.OriginalSize += result.OriginalSize
	typeStats.CompressedSize += result.CompressedSize
	typeStats.AvgRatio = float64(typeStats.CompressedSize) / float64(typeStats.OriginalSize)

	// 更新平均压缩率
	if cm.stats.TotalOriginal > 0 {
		cm.stats.AvgRatio = float64(cm.stats.TotalCompressedSize) / float64(cm.stats.TotalOriginal)
	}

	log.Printf("[Compression] 记录压缩: %s, 算法: %s, 压缩率: %.2f",
		filePath, result.Algorithm, result.Ratio)
}

// GetStats 获取压缩统计.
func (cm *CompressionManager) GetStats() *CompressionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.stats
}

// GetBestAlgorithm 获取最佳压缩算法.
func (cm *CompressionManager) GetBestAlgorithm(fileType string) CompressionAlgorithm {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 从统计中查找最佳算法
	if typeStats, exists := cm.stats.ByFileType[fileType]; exists {
		if typeStats.BestAlgorithm != "" {
			return CompressionAlgorithm(typeStats.BestAlgorithm)
		}
	}

	// 返回默认算法
	profile := cm.SelectAlgorithm("file"+fileType, 10240, "")
	return profile.Algorithm
}

// AddProfile 添加压缩配置.
func (cm *CompressionManager) AddProfile(name string, profile *CompressionProfile) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.profiles[name]; exists {
		return fmt.Errorf("配置 %s 已存在", name)
	}

	cm.profiles[name] = profile

	// 更新类型映射
	for _, ext := range profile.Extensions {
		cm.typeMapping[ext] = profile.Algorithm
	}

	log.Printf("[Compression] 添加压缩配置: %s", name)
	return nil
}

// UpdateProfile 更新压缩配置.
func (cm *CompressionManager) UpdateProfile(name string, profile *CompressionProfile) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.profiles[name]; !exists {
		return fmt.Errorf("配置 %s 不存在", name)
	}

	cm.profiles[name] = profile

	// 更新类型映射
	for _, ext := range profile.Extensions {
		cm.typeMapping[ext] = profile.Algorithm
	}

	return nil
}

// RemoveProfile 删除压缩配置.
func (cm *CompressionManager) RemoveProfile(name string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.profiles[name]; !exists {
		return fmt.Errorf("配置 %s 不存在", name)
	}

	profile := cm.profiles[name]

	// 删除类型映射
	for _, ext := range profile.Extensions {
		delete(cm.typeMapping, ext)
	}

	delete(cm.profiles, name)
	return nil
}

// ListProfiles 列出所有配置.
func (cm *CompressionManager) ListProfiles() map[string]*CompressionProfile {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*CompressionProfile, len(cm.profiles))
	for name, profile := range cm.profiles {
		copy := *profile
		result[name] = &copy
	}

	return result
}

// GetAlgorithmInfo 获取算法信息.
func (cm *CompressionManager) GetAlgorithmInfo(algorithm CompressionAlgorithm) *AlgorithmInfo {
	switch algorithm {
	case CompressionGzip:
		return &AlgorithmInfo{
			Algorithm:    CompressionGzip,
			Name:         "Gzip",
			Description:  "通用压缩算法，平衡速度和压缩率",
			MinLevel:     1,
			MaxLevel:     9,
			DefaultLevel: 5,
			SpeedRating:  6,
			RatioRating:  7,
			CPURating:    5,
			UseCases:     []string{"通用文件", "Web 内容", "日志文件"},
		}
	case CompressionZstd:
		return &AlgorithmInfo{
			Algorithm:    CompressionZstd,
			Name:         "Zstandard",
			Description:  "高速压缩算法，优秀的压缩率",
			MinLevel:     1,
			MaxLevel:     9,
			DefaultLevel: 3,
			SpeedRating:  8,
			RatioRating:  8,
			CPURating:    6,
			UseCases:     []string{"数据库备份", "大文件归档", "实时压缩"},
		}
	case CompressionLZ4:
		return &AlgorithmInfo{
			Algorithm:    CompressionLZ4,
			Name:         "LZ4",
			Description:  "极速压缩算法，适合实时场景",
			MinLevel:     1,
			MaxLevel:     1,
			DefaultLevel: 1,
			SpeedRating:  10,
			RatioRating:  4,
			CPURating:    2,
			UseCases:     []string{"实时数据", "缓存", "临时文件"},
		}
	case CompressionBrotli:
		return &AlgorithmInfo{
			Algorithm:    CompressionBrotli,
			Name:         "Brotli",
			Description:  "Google 开发的压缩算法，文本压缩优秀",
			MinLevel:     1,
			MaxLevel:     11,
			DefaultLevel: 6,
			SpeedRating:  5,
			RatioRating:  9,
			CPURating:    7,
			UseCases:     []string{"文本文件", "Web 资源", "静态内容"},
		}
	case CompressionXZ:
		return &AlgorithmInfo{
			Algorithm:    CompressionXZ,
			Name:         "XZ",
			Description:  "高压缩比算法，适合长期归档",
			MinLevel:     1,
			MaxLevel:     9,
			DefaultLevel: 6,
			SpeedRating:  3,
			RatioRating:  10,
			CPURating:    9,
			UseCases:     []string{"长期归档", "备份", "分发包"},
		}
	default:
		return &AlgorithmInfo{
			Algorithm:    CompressionNone,
			Name:         "None",
			Description:  "不压缩",
			MinLevel:     0,
			MaxLevel:     0,
			DefaultLevel: 0,
			SpeedRating:  10,
			RatioRating:  0,
			CPURating:    0,
			UseCases:     []string{"已压缩文件", "媒体文件"},
		}
	}
}

// AlgorithmInfo 算法信息.
type AlgorithmInfo struct {
	Algorithm    CompressionAlgorithm `json:"algorithm"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	MinLevel     int                  `json:"minLevel"`
	MaxLevel     int                  `json:"maxLevel"`
	DefaultLevel int                  `json:"defaultLevel"`
	SpeedRating  int                  `json:"speedRating"` // 1-10
	RatioRating  int                  `json:"ratioRating"` // 1-10
	CPURating    int                  `json:"cpuRating"`   // 1-10
	UseCases     []string             `json:"useCases"`
}

// 辅助函数

// getFileExtension 获取文件扩展名.
func getFileExtension(filePath string) string {
	parts := strings.Split(filePath, ".")
	if len(parts) > 1 {
		return "." + strings.ToLower(parts[len(parts)-1])
	}
	return ""
}

// isTextFile 判断是否为文本文件.
func isTextFile(ext string) bool {
	textExts := []string{".txt", ".log", ".csv", ".json", ".xml", ".html", ".css", ".js", ".md", ".yaml", ".yml", ".ini", ".conf", ".sh", ".bat", ".py", ".go", ".java", ".c", ".cpp", ".h"}
	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}
	return false
}

// isDocumentFile 判断是否为文档文件.
func isDocumentFile(ext string) bool {
	docExts := []string{".doc", ".docx", ".pdf", ".xlsx", ".pptx", ".odt", ".ods", ".odp", ".rtf"}
	for _, docExt := range docExts {
		if ext == docExt {
			return true
		}
	}
	return false
}
