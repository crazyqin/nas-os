// Package dedup 数据去重模块
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Chunk 表示一个数据块
type Chunk struct {
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	Offset   int64  `json:"offset"`
	RefCount int    `json:"refCount"`
}

// FileRecord 表示文件记录
type FileRecord struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	ChunkHashes []string  `json:"chunkHashes"`
	CreatedAt   time.Time `json:"createdAt"`
	ModifiedAt  time.Time `json:"modifiedAt"`
}

// DuplicateGroup 表示一组重复文件
type DuplicateGroup struct {
	Checksum string   `json:"checksum"`
	Size     int64    `json:"size"`
	Files    []string `json:"files"`
	Savings  int64    `json:"savings"` // 去重后可节省的空间
}

// DedupStats 去重统计信息
type DedupStats struct {
	TotalFiles       int   `json:"totalFiles"`
	TotalSize        int64 `json:"totalSize"`
	DuplicateFiles   int   `json:"duplicateFiles"`
	DuplicateSize    int64 `json:"duplicateSize"`
	SavingsPotential int64 `json:"savingsPotential"`
	ChunksStored     int   `json:"chunksStored"`
	ChunkDataSize    int64 `json:"chunkDataSize"`
	LastScanTime     time.Time `json:"lastScanTime"`
}

// ScanResult 扫描结果
type ScanResult struct {
	FilesScanned     int              `json:"filesScanned"`
	TotalSize        int64            `json:"totalSize"`
	DuplicateGroups  int              `json:"duplicateGroups"`
	DuplicatesFound  int              `json:"duplicatesFound"`
	SavingsPotential int64            `json:"savingsPotential"`
	Duration         time.Duration    `json:"duration"`
	Errors           []ScanError      `json:"errors"`
}

// ScanError 扫描错误
type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Config 去重配置
type Config struct {
	Enabled         bool     `json:"enabled"`
	ChunkSize       int64    `json:"chunkSize"`       // 块大小，默认 4MB
	MinFileSize     int64    `json:"minFileSize"`     // 最小文件大小，小于此值不去重
	ScanPaths       []string `json:"scanPaths"`       // 扫描路径
	ExcludePaths    []string `json:"excludePaths"`    // 排除路径
	ExcludePatterns []string `json:"excludePatterns"` // 排除文件模式
	AutoDedup       bool     `json:"autoDedup"`       // 自动去重
	Compression     bool     `json:"compression"`     // 启用压缩
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		ChunkSize:       4 * 1024 * 1024, // 4MB
		MinFileSize:     1024,            // 1KB
		ScanPaths:       []string{},
		ExcludePaths:    []string{"/proc", "/sys", "/dev", "/tmp"},
		ExcludePatterns: []string{"*.tmp", "*.log", "*.cache"},
		AutoDedup:       false,
		Compression:     true,
	}
}

// DedupPolicy 去重策略
type DedupPolicy struct {
	Mode          string `json:"mode"`          // file, chunk, hybrid
	Action        string `json:"action"`        // report, softlink, hardlink
	MinMatchCount int    `json:"minMatchCount"` // 最小匹配数量
	PreserveAttrs bool   `json:"preserveAttrs"` // 保留文件属性
}

// Manager 去重管理器
type Manager struct {
	mu          sync.RWMutex
	config      *Config
	configPath  string
	chunks      map[string]*Chunk         // hash -> chunk
	fileRecords map[string]*FileRecord    // path -> record
	checksums   map[string][]*FileRecord  // checksum -> records
	duplicates  []*DuplicateGroup
	stats       DedupStats
	scanning    bool
	scanCancel  chan struct{}
}

// NewManager 创建去重管理器
func NewManager(configPath string, config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		config:      config,
		configPath:  configPath,
		chunks:      make(map[string]*Chunk),
		fileRecords: make(map[string]*FileRecord),
		checksums:   make(map[string][]*FileRecord),
		duplicates:  make([]*DuplicateGroup, 0),
		scanCancel:  make(chan struct{}),
	}

	// 加载配置
	if err := m.loadConfig(); err != nil {
		if os.IsNotExist(err) {
			if err := m.saveConfig(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// 加载索引
	if err := m.loadIndex(); err != nil {
		// 索引不存在时忽略
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return m, nil
}

// Scan 扫描重复文件
func (m *Manager) Scan(paths []string) (*ScanResult, error) {
	m.mu.Lock()
	if m.scanning {
		m.mu.Unlock()
		return nil, fmt.Errorf("扫描正在进行中")
	}
	m.scanning = true
	m.scanCancel = make(chan struct{})
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.scanning = false
		m.mu.Unlock()
	}()

	startTime := time.Now()
	result := &ScanResult{
		Errors: make([]ScanError, 0),
	}

	// 重置状态
	m.mu.Lock()
	m.checksums = make(map[string][]*FileRecord)
	m.duplicates = make([]*DuplicateGroup, 0)
	m.stats = DedupStats{}
	m.mu.Unlock()

	// 使用传入的路径或配置中的路径
	scanPaths := paths
	if len(scanPaths) == 0 {
		scanPaths = m.config.ScanPaths
	}

	// 扫描文件
	for _, rootPath := range scanPaths {
		select {
		case <-m.scanCancel:
			return result, fmt.Errorf("扫描已取消")
		default:
			m.scanPath(rootPath, result)
		}
	}

	// 分析重复文件
	m.analyzeDuplicates()

	// 计算统计信息
	m.mu.RLock()
	result.DuplicateGroups = len(m.duplicates)
	for _, group := range m.duplicates {
		result.DuplicatesFound += len(group.Files) - 1
		result.SavingsPotential += group.Savings
	}
	result.TotalSize = m.stats.TotalSize
	result.FilesScanned = m.stats.TotalFiles
	result.Duration = time.Since(startTime)
	m.stats.LastScanTime = startTime
	m.mu.RUnlock()

	// 保存索引
	m.saveIndex()

	return result, nil
}

// scanPath 扫描单个路径
func (m *Manager) scanPath(rootPath string, result *ScanResult) {
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    path,
				Message: err.Error(),
			})
			return nil
		}

		// 跳过目录
		if info.IsDir() {
			// 检查是否在排除路径中
			for _, exclude := range m.config.ExcludePaths {
				if strings.HasPrefix(path, exclude) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 检查文件大小
		if info.Size() < m.config.MinFileSize {
			return nil
		}

		// 检查排除模式
		for _, pattern := range m.config.ExcludePatterns {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if matched {
				return nil
			}
		}

		// 计算文件校验和
		checksum, err := m.calculateFileChecksum(path)
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    path,
				Message: err.Error(),
			})
			return nil
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// 创建文件记录
		record := &FileRecord{
			Path:        path,
			Size:        info.Size(),
			Checksum:    checksum,
			CreatedAt:   time.Now(),
			ModifiedAt:  info.ModTime(),
		}

		// 添加到索引
		m.fileRecords[path] = record
		m.checksums[checksum] = append(m.checksums[checksum], record)

		// 更新统计
		m.stats.TotalFiles++
		m.stats.TotalSize += info.Size()

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, ScanError{
			Path:    rootPath,
			Message: err.Error(),
		})
	}
}

// analyzeDuplicates 分析重复文件
func (m *Manager) analyzeDuplicates() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.duplicates = make([]*DuplicateGroup, 0)

	for checksum, records := range m.checksums {
		if len(records) > 1 {
			// 发现重复文件
			files := make([]string, len(records))
			for i, r := range records {
				files[i] = r.Path
			}

			// 计算可节省空间：保留一个，其余都是重复
			size := records[0].Size
			savings := size * int64(len(records)-1)

			group := &DuplicateGroup{
				Checksum: checksum,
				Size:     size,
				Files:    files,
				Savings:  savings,
			}

			m.duplicates = append(m.duplicates, group)

			// 更新统计
			m.stats.DuplicateFiles += len(records) - 1
			m.stats.DuplicateSize += size * int64(len(records)-1)
			m.stats.SavingsPotential += savings
		}
	}
}

// GetDuplicates 获取重复文件列表
func (m *Manager) GetDuplicates() ([]*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DuplicateGroup, len(m.duplicates))
	copy(result, m.duplicates)

	return result, nil
}

// Deduplicate 执行去重操作
func (m *Manager) Deduplicate(checksum string, keepPath string, policy DedupPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	records, exists := m.checksums[checksum]
	if !exists || len(records) <= 1 {
		return fmt.Errorf("没有找到重复文件")
	}

	// 找到要保留的文件
	var keepRecord *FileRecord
	for _, r := range records {
		if r.Path == keepPath {
			keepRecord = r
			break
		}
	}

	if keepRecord == nil {
		return fmt.Errorf("保留路径不在重复文件列表中")
	}

	// 对其他文件执行去重
	for _, record := range records {
		if record.Path == keepPath {
			continue
		}

		switch policy.Action {
		case "softlink":
			// 删除文件并创建软链接
			if err := os.Remove(record.Path); err != nil {
				return fmt.Errorf("删除文件失败：%w", err)
			}
			if err := os.Symlink(keepPath, record.Path); err != nil {
				return fmt.Errorf("创建软链接失败：%w", err)
			}

		case "hardlink":
			// 删除文件并创建硬链接
			if err := os.Remove(record.Path); err != nil {
				return fmt.Errorf("删除文件失败：%w", err)
			}
			if err := os.Link(keepPath, record.Path); err != nil {
				return fmt.Errorf("创建硬链接失败：%w", err)
			}

		case "report":
			// 只报告，不执行操作

		default:
			return fmt.Errorf("不支持的去重操作：%s", policy.Action)
		}
	}

	return nil
}

// GetReport 获取去重报告
func (m *Manager) GetReport() (*DedupReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &DedupReport{
		GeneratedAt:    time.Now(),
		Stats:          m.stats,
		DuplicateGroups: make([]DuplicateGroupSummary, 0),
	}

	// 按大小排序重复组
	sortedDuplicates := make([]*DuplicateGroup, len(m.duplicates))
	copy(sortedDuplicates, m.duplicates)

	// 简单排序（按节省空间降序）
	for i := 0; i < len(sortedDuplicates); i++ {
		for j := i + 1; j < len(sortedDuplicates); j++ {
			if sortedDuplicates[j].Savings > sortedDuplicates[i].Savings {
				sortedDuplicates[i], sortedDuplicates[j] = sortedDuplicates[j], sortedDuplicates[i]
			}
		}
	}

	// 生成摘要
	for _, group := range sortedDuplicates {
		summary := DuplicateGroupSummary{
			Checksum:    group.Checksum[:16] + "...", // 截断显示
			Size:        group.Size,
			FileCount:   len(group.Files),
			Savings:     group.Savings,
			ExampleFiles: group.Files,
		}
		if len(summary.ExampleFiles) > 3 {
			summary.ExampleFiles = group.Files[:3]
		}
		report.DuplicateGroups = append(report.DuplicateGroups, summary)
	}

	return report, nil
}

// DedupReport 去重报告
type DedupReport struct {
	GeneratedAt     time.Time               `json:"generatedAt"`
	Stats           DedupStats              `json:"stats"`
	DuplicateGroups []DuplicateGroupSummary `json:"duplicateGroups"`
}

// DuplicateGroupSummary 重复组摘要
type DuplicateGroupSummary struct {
	Checksum     string   `json:"checksum"`
	Size         int64    `json:"size"`
	FileCount    int      `json:"fileCount"`
	Savings      int64    `json:"savings"`
	ExampleFiles []string `json:"exampleFiles"`
}

// GetStats 获取统计信息
func (m *Manager) GetStats() DedupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// CancelScan 取消扫描
func (m *Manager) CancelScan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.scanning {
		close(m.scanCancel)
	}
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return m.saveConfig()
}

// ========== 块级去重 ==========

// CreateChunk 创建数据块
func (m *Manager) CreateChunk(data []byte) (*Chunk, error) {
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查块是否已存在
	if chunk, exists := m.chunks[hashStr]; exists {
		chunk.RefCount++
		return chunk, nil
	}

	// 创建新块
	chunk := &Chunk{
		Hash:     hashStr,
		Size:     int64(len(data)),
		RefCount: 1,
	}

	m.chunks[hashStr] = chunk
	m.stats.ChunksStored++
	m.stats.ChunkDataSize += int64(len(data))

	return chunk, nil
}

// GetChunk 获取数据块
func (m *Manager) GetChunk(hash string) (*Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chunk, exists := m.chunks[hash]
	if !exists {
		return nil, fmt.Errorf("块不存在：%s", hash)
	}

	return chunk, nil
}

// ========== 内部方法 ==========

func (m *Manager) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, m.config)
}

func (m *Manager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0644)
}

func (m *Manager) loadIndex() error {
	indexPath := m.configPath + ".index"
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	var index struct {
		FileRecords map[string]*FileRecord   `json:"fileRecords"`
		Checksums   map[string][]*FileRecord `json:"checksums"`
		Stats       DedupStats               `json:"stats"`
	}

	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	m.fileRecords = index.FileRecords
	m.checksums = index.Checksums
	m.stats = index.Stats

	return nil
}

func (m *Manager) saveIndex() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexPath := m.configPath + ".index"

	data, err := json.MarshalIndent(struct {
		FileRecords map[string]*FileRecord   `json:"fileRecords"`
		Checksums   map[string][]*FileRecord `json:"checksums"`
		Stats       DedupStats               `json:"stats"`
	}{
		FileRecords: m.fileRecords,
		Checksums:   m.checksums,
		Stats:       m.stats,
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}