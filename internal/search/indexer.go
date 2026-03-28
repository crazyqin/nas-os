// Package search provides full-text search capabilities using Bleve.
// This file implements file and settings indexers for the global search engine.
package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ================== 文件索引器 ==================

// FileIndexer 文件索引器
// 负责增量索引文件系统变化，支持实时监控和定时扫描.
type FileIndexer struct {
	engine     *Engine
	watchers   map[string]*fsWatcher
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *zap.Logger
	config     IndexerConfig
	indexPath  string // 索引路径，用于增量索引
	indexState *IndexState
}

// IndexerConfig 索引器配置.
type IndexerConfig struct {
	// 并行索引工作线程数
	Workers int `json:"workers"`
	// 批量索引大小
	BatchSize int `json:"batchSize"`
	// 文件变化监听延迟（毫秒）
	WatchDelay int `json:"watchDelay"`
	// 最大索引文件大小（字节）
	MaxFileSize int64 `json:"maxFileSize"`
	// 是否启用实时监控
	EnableWatch bool `json:"enableWatch"`
	// 排除的目录模式
	ExcludePatterns []string `json:"excludePatterns"`
	// 是否索引隐藏文件
	IndexHidden bool `json:"indexHidden"`
	// 增量索引状态文件路径
	StateFile string `json:"stateFile"`
}

// DefaultIndexerConfig 默认索引器配置.
func DefaultIndexerConfig() IndexerConfig {
	return IndexerConfig{
		Workers:     4,
		BatchSize:   100,
		WatchDelay:  500,
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		EnableWatch: true,
		ExcludePatterns: []string{
			".git", ".svn", ".hg", "node_modules", "vendor",
			"tmp", "temp", "cache", ".cache", "__pycache__",
			".idea", ".vscode", "*.tmp", "*.temp", "*.bak",
			".DS_Store", "Thumbs.db", "*.swp", "*.swo",
		},
		IndexHidden: false,
		StateFile:   "/var/lib/nas-os/search/indexer.state",
	}
}

// IndexState 索引状态
// 记录已索引文件的哈希值和元信息，用于增量索引.
type IndexState struct {
	Files      map[string]FileState `json:"files"`
	LastScan   time.Time            `json:"lastScan"`
	TotalCount int64                `json:"totalCount"`
	TotalSize  int64                `json:"totalSize"`
	mu         sync.RWMutex
}

// FileState 文件状态
// 记录单个文件的索引状态.
type FileState struct {
	ModTime int64  `json:"modTime"` // 修改时间 Unix 纳秒
	Size    int64  `json:"size"`    // 文件大小
	Hash    string `json:"hash"`   // 内容哈希（仅文本文件）
	Indexed int64  `json:"indexed"` // 索引时间 Unix 纳秒
}

// fsWatcher 文件系统监控器.
type fsWatcher struct {
	path     string
	events   chan string
	done     chan struct{}
	fileList []string // 监控的文件列表
}

// NewFileIndexer 创建文件索引器.
func NewFileIndexer(engine *Engine, config IndexerConfig, logger *zap.Logger) (*FileIndexer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 加载索引状态
	state, err := loadIndexState(config.StateFile)
	if err != nil {
		logger.Warn("加载索引状态失败，将创建新状态", zap.Error(err))
		state = &IndexState{
			Files: make(map[string]FileState),
		}
	}

	indexer := &FileIndexer{
		engine:     engine,
		watchers:   make(map[string]*fsWatcher),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		config:     config,
		indexPath:  engine.config.IndexPath,
		indexState: state,
	}

	return indexer, nil
}

// IndexPath 索引指定路径
// 扫描目录并索引所有文件，支持增量索引.
func (fi *FileIndexer) IndexPath(path string, forceFull bool) (*IndexResult, error) {
	startTime := time.Now()
	result := &IndexResult{
		Path:      path,
		StartTime: startTime,
	}

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("路径不存在: %w", err)
	}

	if !info.IsDir() {
		// 单文件索引
		err := fi.indexFile(path, forceFull)
		if err != nil {
			return nil, err
		}
		result.IndexedFiles = 1
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		return result, nil
	}

	// 目录索引
	err = fi.indexDirectory(path, forceFull, result)
	if err != nil {
		return nil, err
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)

	// 保存索引状态
	if err := fi.saveState(); err != nil {
		fi.logger.Warn("保存索引状态失败", zap.Error(err))
	}

	return result, nil
}

// indexDirectory 索引目录.
func (fi *FileIndexer) indexDirectory(root string, forceFull bool, result *IndexResult) error {
	workerCh := make(chan string, fi.config.BatchSize*2)
	errCh := make(chan error, fi.config.Workers)
	var wg sync.WaitGroup

	// 启动工作线程
	for i := 0; i < fi.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range workerCh {
				if err := fi.indexFile(path, forceFull); err != nil {
					errCh <- err
				}
			}
		}()
	}

	// 遍历目录
	var errors []error
	go func() {
		for err := range errCh {
			errors = append(errors, err)
			result.Errors++
		}
	}()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Skipped++
			return nil
		}

		// 检查排除模式
		if fi.shouldExclude(path, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			result.Skipped++
			return nil
		}

		// 只索引文件
		if !info.IsDir() {
			result.TotalFiles++
			workerCh <- path
		}

		return nil
	})

	close(workerCh)
	wg.Wait()
	close(errCh)

	if err != nil {
		return fmt.Errorf("遍历目录失败: %w", err)
	}

	if len(errors) > 0 {
		fi.logger.Warn("索引过程中有错误",
			zap.Int("errors", len(errors)),
			zap.Strings("firstFew", formatErrors(errors, 5)))
	}

	return nil
}

// indexFile 索引单个文件
// 使用增量索引：检查文件是否已索引且未修改.
func (fi *FileIndexer) indexFile(path string, forceFull bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 检查是否需要索引
	if !forceFull && !fi.needsIndexing(path, info) {
		return nil // 跳过未修改的文件
	}

	// 检查文件大小
	if info.Size() > fi.config.MaxFileSize {
		fi.logger.Debug("文件过大，跳过",
			zap.String("path", path),
			zap.Int64("size", info.Size()))
		return nil
	}

	// 构建索引文档
	doc := FileInfo{
		Path:    path,
		Name:    info.Name(),
		Ext:     strings.ToLower(filepath.Ext(path)),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   false,
	}

	// 检测 MIME 类型
	doc.MimeType = getMimeType(doc.Ext)

	// 读取文本内容
	if fi.shouldIndexContent(path, info.Size()) {
		content, err := fi.readFileContent(path)
		if err == nil {
			doc.Content = content
		}
	}

	// 索引文档
	if err := fi.engine.index.Index(path, doc); err != nil {
		return fmt.Errorf("索引文件失败: %w", err)
	}

	// 更新索引状态
	fi.updateIndexState(path, info)

	return nil
}

// needsIndexing 检查文件是否需要索引
// 增量索引核心逻辑：比较修改时间和大小.
func (fi *FileIndexer) needsIndexing(path string, info os.FileInfo) bool {
	fi.indexState.mu.RLock()
	state, exists := fi.indexState.Files[path]
	fi.indexState.mu.RUnlock()

	if !exists {
		return true // 新文件
	}

	// 检查修改时间
	if info.ModTime().UnixNano() > state.ModTime {
		return true // 文件已修改
	}

	// 检查大小
	if info.Size() != state.Size {
		return true // 大小变化
	}

	return false // 无需重新索引
}

// updateIndexState 更新索引状态.
func (fi *FileIndexer) updateIndexState(path string, info os.FileInfo) {
	fi.indexState.mu.Lock()
	defer fi.indexState.mu.Unlock()

	state := FileState{
		ModTime: info.ModTime().UnixNano(),
		Size:    info.Size(),
		Indexed: time.Now().UnixNano(),
	}

	// 计算文本文件的哈希
	if fi.shouldIndexContent(path, info.Size()) {
		if hash, err := fi.calculateFileHash(path); err == nil {
			state.Hash = hash
		}
	}

	fi.indexState.Files[path] = state
	fi.indexState.TotalCount++
	fi.indexState.TotalSize += info.Size()
}

// shouldExclude 检查是否应该排除.
func (fi *FileIndexer) shouldExclude(path string, info os.FileInfo) bool {
	// 排除隐藏文件
	if !fi.config.IndexHidden && strings.HasPrefix(info.Name(), ".") {
		return true
	}

	// 检查排除模式
	for _, pattern := range fi.config.ExcludePatterns {
		matched, err := filepath.Match(pattern, info.Name())
		if err == nil && matched {
			return true
		}
		// 目录名匹配
		if info.IsDir() && strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// shouldIndexContent 判断是否应该索引文件内容.
func (fi *FileIndexer) shouldIndexContent(path string, size int64) bool {
	if size > fi.config.MaxFileSize {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return fi.engine.textExts[ext]
}

// readFileContent 读取文件内容.
func (fi *FileIndexer) readFileContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 限制读取大小
	maxSize := fi.config.MaxFileSize
	buf := make([]byte, maxSize)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	return string(buf[:n]), nil
}

// calculateFileHash 计算文件哈希.
func (fi *FileIndexer) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.CopyN(hash, file, fi.config.MaxFileSize); err != nil && err != io.EOF {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil))[:16], nil
}

// saveState 保存索引状态.
func (fi *FileIndexer) saveState() error {
	if fi.config.StateFile == "" {
		return nil
	}

	// 实现状态持久化
	// TODO: 实现文件保存
	return nil
}

// loadIndexState 加载索引状态.
func loadIndexState(path string) (*IndexState, error) {
	if path == "" {
		return &IndexState{Files: make(map[string]FileState)}, nil
	}

	// TODO: 实现文件加载
	return &IndexState{Files: make(map[string]FileState)}, nil
}

// ================== 设置索引器 ==================

// SettingsIndexer 设置索引器
// 索引 NAS 系统设置项，支持全局搜索设置.
type SettingsIndexer struct {
	engine     *Engine
	logger     *zap.Logger
	categories map[string]SettingsCategory
	mu         sync.RWMutex
}

// SettingsCategory 设置分类.
type SettingsCategory struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Items       []SettingsItem    `json:"items"`
	Tags        []string          `json:"tags"`
	Keywords    []string          `json:"keywords"`
	Path        string            `json:"path"` // 设置页面路径
}

// SettingsItem 设置项.
type SettingsItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Section     string   `json:"section"`
	Path        string   `json:"path"`
	Keywords    []string `json:"keywords"`
	Tags        []string `json:"tags"`
	Type        string   `json:"type"` // string, number, bool, select, multi
	Value       any      `json:"value,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Min         any      `json:"min,omitempty"`
	Max         any      `json:"max,omitempty"`
	Step        any      `json:"step,omitempty"`
	Required    bool     `json:"required"`
	ReadOnly    bool     `json:"readOnly"`
	Advanced    bool     `json:"advanced"`
}

// SettingsDocument 设置文档（用于索引）.
type SettingsDocument struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Section     string   `json:"section"`
	Path        string   `json:"path"`
	Keywords    []string `json:"keywords"`
	Tags        []string `json:"tags"`
	Type        string   `json:"type"`
	Value       string   `json:"value,omitempty"`
	DocURL      string   `json:"docUrl,omitempty"`
}

// NewSettingsIndexer 创建设置索引器.
func NewSettingsIndexer(engine *Engine, logger *zap.Logger) *SettingsIndexer {
	si := &SettingsIndexer{
		engine:     engine,
		logger:     logger,
		categories: make(map[string]SettingsCategory),
	}

	// 注册默认设置分类
	si.registerDefaultCategories()

	return si
}

// registerDefaultCategories 注册默认设置分类.
func (si *SettingsIndexer) registerDefaultCategories() {
	categories := []SettingsCategory{
		{
			ID:          "network",
			Name:        "网络设置",
			Description: "配置网络接口、DNS、防火墙等",
			Icon:        "network",
			Path:        "/settings/network",
			Tags:        []string{"网络", "网卡", "IP", "DNS", "防火墙"},
			Keywords:    []string{"network", "interface", "dns", "firewall", "ip", "dhcp", "static"},
		},
		{
			ID:          "storage",
			Name:        "存储设置",
			Description: "配置存储池、卷、共享等",
			Icon:        "storage",
			Path:        "/settings/storage",
			Tags:        []string{"存储", "磁盘", "卷", "共享", "ZFS"},
			Keywords:    []string{"storage", "disk", "volume", "pool", "share", "zfs", "raid"},
		},
		{
			ID:          "users",
			Name:        "用户管理",
			Description: "管理用户、组、权限",
			Icon:        "users",
			Path:        "/settings/users",
			Tags:        []string{"用户", "组", "权限", "认证"},
			Keywords:    []string{"user", "group", "permission", "auth", "account", "password"},
		},
		{
			ID:          "services",
			Name:        "服务管理",
			Description: "管理系统服务、Docker、虚拟机",
			Icon:        "services",
			Path:        "/settings/services",
			Tags:        []string{"服务", "Docker", "虚拟机", "容器"},
			Keywords:    []string{"service", "docker", "vm", "container", "daemon"},
		},
		{
			ID:          "backup",
			Name:        "备份设置",
			Description: "配置备份任务、快照、同步",
			Icon:        "backup",
			Path:        "/settings/backup",
			Tags:        []string{"备份", "快照", "同步", "恢复"},
			Keywords:    []string{"backup", "snapshot", "sync", "restore", "schedule"},
		},
		{
			ID:          "security",
			Name:        "安全设置",
			Description: "配置安全策略、证书、加密",
			Icon:        "security",
			Path:        "/settings/security",
			Tags:        []string{"安全", "证书", "加密", "SSH", "防火墙"},
			Keywords:    []string{"security", "certificate", "ssl", "ssh", "encryption", "firewall"},
		},
		{
			ID:          "system",
			Name:        "系统设置",
			Description: "配置系统时间、语言、更新",
			Icon:        "system",
			Path:        "/settings/system",
			Tags:        []string{"系统", "时间", "语言", "更新"},
			Keywords:    []string{"system", "time", "ntp", "language", "update", "upgrade"},
		},
		{
			ID:          "notifications",
			Name:        "通知设置",
			Description: "配置告警、邮件通知、推送",
			Icon:        "notifications",
			Path:        "/settings/notifications",
			Tags:        []string{"通知", "告警", "邮件", "推送"},
			Keywords:    []string{"notification", "alert", "email", "push", "webhook"},
		},
	}

	for _, cat := range categories {
		si.categories[cat.ID] = cat
	}
}

// IndexSettings 索引设置项.
func (si *SettingsIndexer) IndexSettings(items []SettingsItem) error {
	batch := si.engine.index.NewBatch()

	for _, item := range items {
		doc := SettingsDocument{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			Category:    item.Category,
			Section:     item.Section,
			Path:        item.Path,
			Keywords:    item.Keywords,
			Tags:        item.Tags,
			Type:        item.Type,
		}

		if item.Value != nil {
			doc.Value = fmt.Sprintf("%v", item.Value)
		}

		// 索引文档
		if err := batch.Index("setting:"+item.ID, doc); err != nil {
			si.logger.Warn("索引设置项失败",
				zap.String("id", item.ID),
				zap.Error(err))
		}
	}

	return si.engine.index.Batch(batch)
}

// GetCategory 获取设置分类.
func (si *SettingsIndexer) GetCategory(id string) (SettingsCategory, bool) {
	si.mu.RLock()
	defer si.mu.RUnlock()
	cat, ok := si.categories[id]
	return cat, ok
}

// ListCategories 列出所有设置分类.
func (si *SettingsIndexer) ListCategories() []SettingsCategory {
	si.mu.RLock()
	defer si.mu.RUnlock()

	categories := make([]SettingsCategory, 0, len(si.categories))
	for _, cat := range si.categories {
		categories = append(categories, cat)
	}
	return categories
}

// SettingsSearchResult 定义在 settings.go 中

// ================== 索引结果 ==================

// IndexResult 索引结果.
type IndexResult struct {
	Path         string        `json:"path"`
	TotalFiles   int64         `json:"totalFiles"`
	IndexedFiles int64         `json:"indexedFiles"`
	Skipped      int64         `json:"skipped"`
	Errors       int64         `json:"errors"`
	StartTime    time.Time     `json:"startTime"`
	EndTime      time.Time     `json:"endTime"`
	Duration     time.Duration `json:"duration"`
}

// ================== 辅助函数 ==================

// formatErrors 格式化错误列表.
func formatErrors(errors []error, max int) []string {
	var result []string
	for i, err := range errors {
		if i >= max {
			break
		}
		result = append(result, err.Error())
	}
	return result
}

// Close 关闭索引器.
func (fi *FileIndexer) Close() error {
	fi.cancel()
	// 保存状态
	return fi.saveState()
}

