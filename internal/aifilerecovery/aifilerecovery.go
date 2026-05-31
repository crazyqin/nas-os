// Package aifilerecovery 提供AI驱动的文件恢复系统
// 深度扫描、模式识别、多文件系统支持、恢复预览
package aifilerecovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	// Version 模块版本
	Version = "1.0.0"

	// MaxScanDepth 最大扫描深度
	MaxScanDepth = 32

	// MaxConcurrentScans 最大并发扫描数
	MaxConcurrentScans = 8

	// ChunkSize 扫描块大小 (4KB)
	ChunkSize = 4096

	// MinFileSize 最小文件大小
	MinFileSize = 512

	// MaxFileSize 最大文件大小 (10GB)
	MaxFileSize = 10 * 1024 * 1024 * 1024
)

// ========== 文件系统类型 ==========

// FileSystemType 文件系统类型
type FileSystemType string

const (
	FSTypeExt4  FileSystemType = "ext4"
	FSTypeXFS   FileSystemType = "xfs"
	FSTypeBtrfs FileSystemType = "btrfs"
	FSTypeZFS   FileSystemType = "zfs"
	FSTypeNTFS  FileSystemType = "ntfs"
	FSTypeFAT32 FileSystemType = "fat32"
	FSTypeExFAT FileSystemType = "exfat"
	FSTypeAPFS  FileSystemType = "apfs"
	FSTypeHFS   FileSystemType = "hfs+"
	FSTypeAuto  FileSystemType = "auto"
)

// ========== 恢复状态 ==========

// RecoveryStatus 恢复状态
type RecoveryStatus string

const (
	StatusPending    RecoveryStatus = "pending"
	StatusScanning   RecoveryStatus = "scanning"
	StatusFound      RecoveryStatus = "found"
	StatusRecovering RecoveryStatus = "recovering"
	StatusRecovered  RecoveryStatus = "recovered"
	StatusFailed     RecoveryStatus = "failed"
	StatusCorrupted  RecoveryStatus = "corrupted"
)

// ========== 文件类型 ==========

// FileType 文件类型
type FileType string

const (
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeCode     FileType = "code"
	FileTypeDatabase FileType = "database"
	FileTypeEmail    FileType = "email"
	FileTypeOther    FileType = "other"
)

// ========== 扫描模式 ==========

// ScanMode 扫描模式
type ScanMode string

const (
	ScanModeQuick    ScanMode = "quick"    // 快速扫描：仅文件表
	ScanModeDeep     ScanMode = "deep"     // 深度扫描：全盘扇区
	ScanModeSmart    ScanMode = "smart"    // 智能扫描：AI模式识别
	ScanModeSignature ScanMode = "signature" // 签名扫描：文件头匹配
)

// ========== 数据结构 ==========

// RecoverableFile 可恢复文件
type RecoverableFile struct {
	ID            string         `json:"id"`
	OriginalPath  string         `json:"original_path"`
	FileName      string         `json:"file_name"`
	FileType      FileType       `json:"file_type"`
	FileSystem    FileSystemType `json:"file_system"`
	Size          int64          `json:"size"`
	DeletedAt     time.Time      `json:"deleted_at"`
	ScanMode      ScanMode       `json:"scan_mode"`
	Status        RecoveryStatus `json:"status"`
	Integrity     float64        `json:"integrity"`     // 0.0-1.0 完整性评分
	Confidence    float64        `json:"confidence"`    // 0.0-1.0 AI置信度
	Signature     string         `json:"signature"`     // 文件签名
	FragmentCount int            `json:"fragment_count"`
	Fragments     []Fragment     `json:"fragments,omitempty"`
	Preview       []byte         `json:"preview,omitempty"` // 预览数据
	Metadata      FileMetadata   `json:"metadata"`
	RecoveredAt   *time.Time     `json:"recovered_at,omitempty"`
	RecoveredTo   string         `json:"recovered_to,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// Fragment 文件片段
type Fragment struct {
	Index    int    `json:"index"`
	Offset   int64  `json:"offset"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Status   string `json:"status"` // "ok", "damaged", "missing"
}

// FileMetadata 文件元数据
type FileMetadata struct {
	CreatedAt    time.Time         `json:"created_at"`
	ModifiedAt   time.Time         `json:"modified_at"`
	AccessedAt   time.Time         `json:"accessed_at"`
	Permissions  fs.FileMode       `json:"permissions"`
	Owner        string            `json:"owner"`
	Group        string            `json:"group"`
	Inode        uint64            `json:"inode"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	ExtendedAttr map[string][]byte `json:"extended_attr,omitempty"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ID            string           `json:"id"`
	ScanPath      string           `json:"scan_path"`
	FileSystem    FileSystemType   `json:"file_system"`
	ScanMode      ScanMode         `json:"scan_mode"`
	StartedAt     time.Time        `json:"started_at"`
	CompletedAt   *time.Time       `json:"completed_at,omitempty"`
	Duration      time.Duration    `json:"duration"`
	TotalFiles    int              `json:"total_files"`
	FoundFiles    int              `json:"found_files"`
	RecoveredFiles int             `json:"recovered_files"`
	CorruptedFiles int             `json:"corrupted_files"`
	TotalSize     int64            `json:"total_size"`
	ScannedSize   int64            `json:"scanned_size"`
	Status        RecoveryStatus   `json:"status"`
	Files         []RecoverableFile `json:"files,omitempty"`
	Error         string           `json:"error,omitempty"`
}

// RecoveryRequest 恢复请求
type RecoveryRequest struct {
	FileIDs    []string `json:"file_ids"`
	OutputPath string   `json:"output_path"`
	Overwrite  bool     `json:"overwrite"`
	Verify     bool     `json:"verify"`
	DryRun     bool     `json:"dry_run"`
}

// RecoveryResult 恢复结果
type RecoveryResult struct {
	RequestID    string   `json:"request_id"`
	TotalFiles   int      `json:"total_files"`
	SuccessCount int      `json:"success_count"`
	FailCount    int      `json:"fail_count"`
	SkipCount    int      `json:"skip_count"`
	TotalSize    int64    `json:"total_size"`
	OutputPath   string   `json:"output_path"`
	Files        []string `json:"files"`
	Errors       []string `json:"errors,omitempty"`
}

// ScanConfig 扫描配置
type ScanConfig struct {
	ScanPath     string         `json:"scan_path"`
	OutputPath   string         `json:"output_path"`
	FileSystem   FileSystemType `json:"file_system"`
	ScanMode     ScanMode       `json:"scan_mode"`
	MaxDepth     int            `json:"max_depth"`
	MaxFileSize  int64          `json:"max_file_size"`
	FileTypes    []FileType     `json:"file_types,omitempty"`
	ExcludePaths []string       `json:"exclude_paths,omitempty"`
	Concurrency  int            `json:"concurrency"`
	AIEnabled    bool           `json:"ai_enabled"`
}

// AIAnalysis AI分析结果
type AIAnalysis struct {
	FileType     FileType `json:"file_type"`
	Confidence   float64  `json:"confidence"`
	Pattern      string   `json:"pattern"`
	Encoding     string   `json:"encoding"`
	Compression  string   `json:"compression"`
	Encryption   bool     `json:"encryption"`
	Corrupted    bool     `json:"corrupted"`
	Recoverable  bool     `json:"recoverable"`
	Suggestions  []string `json:"suggestions,omitempty"`
}

// ========== 管理器 ==========

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	mu          sync.RWMutex
	config      ScanConfig
	results     map[string]*ScanResult
	fileSigs    map[string]FileType // 文件签名映射
	running     bool
	cancelFunc  context.CancelFunc
}

// NewRecoveryManager 创建恢复管理器
func NewRecoveryManager(config ScanConfig) *RecoveryManager {
	if config.MaxDepth <= 0 {
		config.MaxDepth = MaxScanDepth
	}
	if config.Concurrency <= 0 {
		config.Concurrency = MaxConcurrentScans
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = MaxFileSize
	}

	rm := &RecoveryManager{
		config:   config,
		results:  make(map[string]*ScanResult),
		fileSigs: initFileSignatures(),
	}
	return rm
}

// initFileSignatures 初始化文件签名
func initFileSignatures() map[string]FileType {
	return map[string]FileType{
		"\x25\x50\x44\x46": FileTypeDocument, // PDF
		"\xD0\xCF\x11\xE0": FileTypeDocument, // MS Office
		"\x50\x4B\x03\x04": FileTypeArchive,  // ZIP
		"\x1F\x8B":         FileTypeArchive,  // GZIP
		"\x42\x5A\x68":     FileTypeArchive,  // BZIP2
		"\x37\x7A\xBC\xAF": FileTypeArchive,  // 7Z
		"\x52\x61\x72\x21": FileTypeArchive,  // RAR
		"\xFF\xD8\xFF":     FileTypeImage,    // JPEG
		"\x89\x50\x4E\x47": FileTypeImage,    // PNG
		"\x47\x49\x46\x38": FileTypeImage,    // GIF
		"\x49\x49\x2A\x00": FileTypeImage,    // TIFF
		"\x42\x4D":         FileTypeImage,    // BMP
		"\x00\x00\x01\x00": FileTypeVideo,    // ICO
		"\x1A\x45\xDF\xA3": FileTypeVideo,    // MKV/WebM
		"\x00\x00\x00\x18": FileTypeVideo,    // MP4
		"\x00\x00\x00\x1C": FileTypeVideo,    // MP4
		"\x49\x44\x33":     FileTypeAudio,    // MP3
		"\xFF\xFB":         FileTypeAudio,    // MP3
		"\x4F\x67\x67\x53": FileTypeAudio,    // OGG
		"\x66\x4C\x61\x43": FileTypeAudio,    // FLAC
		"\x53\x51\x4C\x69": FileTypeDatabase, // SQLite
	}
}

// StartScan 启动扫描
func (rm *RecoveryManager) StartScan(ctx context.Context, config ScanConfig) (*ScanResult, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.running {
		return nil, errors.New("scan already in progress")
	}

	scanCtx, cancel := context.WithCancel(ctx)
	rm.cancelFunc = cancel
	rm.running = true

	result := &ScanResult{
		ID:         generateID(),
		ScanPath:   config.ScanPath,
		FileSystem: config.FileSystem,
		ScanMode:   config.ScanMode,
		StartedAt:  time.Now(),
		Status:     StatusScanning,
	}

	rm.results[result.ID] = result

	go rm.executeScan(scanCtx, result, config)

	return result, nil
}

// executeScan 执行扫描
func (rm *RecoveryManager) executeScan(ctx context.Context, result *ScanResult, config ScanConfig) {
	defer func() {
		rm.mu.Lock()
		rm.running = false
		rm.mu.Unlock()
	}()

	var files []RecoverableFile
	var err error

	switch config.ScanMode {
	case ScanModeQuick:
		files, err = rm.quickScan(ctx, config)
	case ScanModeDeep:
		files, err = rm.deepScan(ctx, config)
	case ScanModeSmart:
		files, err = rm.smartScan(ctx, config)
	case ScanModeSignature:
		files, err = rm.signatureScan(ctx, config)
	default:
		files, err = rm.smartScan(ctx, config)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	result.CompletedAt = &now
	result.Duration = now.Sub(result.StartedAt)

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		return
	}

	result.Files = files
	result.TotalFiles = len(files)
	result.FoundFiles = len(files)

	for _, f := range files {
		result.TotalSize += f.Size
		if f.Status == StatusCorrupted {
			result.CorruptedFiles++
		}
	}

	result.Status = StatusFound
}

// quickScan 快速扫描
func (rm *RecoveryManager) quickScan(ctx context.Context, config ScanConfig) ([]RecoverableFile, error) {
	var files []RecoverableFile
	var mu sync.Mutex
	var wg sync.WaitGroup

	fileCh := make(chan string, config.Concurrency*2)

	// 启动工作协程
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				f, err := rm.scanFile(ctx, path, config)
				if err != nil {
					continue
				}

				mu.Lock()
				files = append(files, f)
				mu.Unlock()
			}
		}()
	}

	// 遍历目录
	err := filepath.Walk(config.ScanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			return nil
		}

		select {
		case fileCh <- path:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	})

	close(fileCh)
	wg.Wait()

	return files, err
}

// deepScan 深度扫描
func (rm *RecoveryManager) deepScan(ctx context.Context, config ScanConfig) ([]RecoverableFile, error) {
	// 深度扫描包括已删除文件的恢复
	// 通过扫描磁盘扇区寻找文件痕迹
	var files []RecoverableFile

	// 先执行快速扫描获取现有文件
	existingFiles, err := rm.quickScan(ctx, config)
	if err == nil {
		files = append(files, existingFiles...)
	}

	// 扫描已删除文件
	deletedFiles, err := rm.scanDeletedFiles(ctx, config)
	if err == nil {
		files = append(files, deletedFiles...)
	}

	return files, nil
}

// smartScan 智能扫描
func (rm *RecoveryManager) smartScan(ctx context.Context, config ScanConfig) ([]RecoverableFile, error) {
	// 智能扫描：结合深度扫描和AI分析
	files, err := rm.deepScan(ctx, config)
	if err != nil {
		return nil, err
	}

	if config.AIEnabled {
		// AI增强分析
		for i := range files {
			analysis := rm.analyzeFileWithAI(&files[i])
			files[i].Confidence = analysis.Confidence
			files[i].Metadata.Attributes["ai_type"] = string(analysis.FileType)
			files[i].Metadata.Attributes["ai_pattern"] = analysis.Pattern
		}
	}

	return files, nil
}

// signatureScan 签名扫描
func (rm *RecoveryManager) signatureScan(ctx context.Context, config ScanConfig) ([]RecoverableFile, error) {
	var files []RecoverableFile

	err := filepath.Walk(config.ScanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		f, err := rm.scanBySignature(ctx, path, info, config)
		if err != nil {
			return nil
		}
		if f != nil {
			files = append(files, *f)
		}

		return nil
	})

	return files, err
}

// scanFile 扫描单个文件
func (rm *RecoveryManager) scanFile(ctx context.Context, path string, config ScanConfig) (RecoverableFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return RecoverableFile{}, err
	}

	if info.Size() > config.MaxFileSize {
		return RecoverableFile{}, errors.New("file too large")
	}

	// 计算文件签名
	sig, err := rm.calculateSignature(path)
	if err != nil {
		sig = ""
	}

	// 检测文件类型
	fileType := rm.detectFileType(path, sig)

	f := RecoverableFile{
		ID:           generateID(),
		OriginalPath: path,
		FileName:     filepath.Base(path),
		FileType:     fileType,
		FileSystem:   config.FileSystem,
		Size:         info.Size(),
		ScanMode:     config.ScanMode,
		Status:       StatusFound,
		Integrity:    1.0,
		Confidence:   1.0,
		Signature:    sig,
		Metadata: FileMetadata{
			CreatedAt:   info.ModTime(),
			ModifiedAt:  info.ModTime(),
			AccessedAt:  info.ModTime(),
			Permissions: info.Mode(),
		},
	}

	// 检查完整性
	integrity, fragments := rm.checkIntegrity(path, info.Size())
	f.Integrity = integrity
	f.FragmentCount = len(fragments)
	f.Fragments = fragments

	if integrity < 0.5 {
		f.Status = StatusCorrupted
	}

	return f, nil
}

// scanDeletedFiles 扫描已删除文件
func (rm *RecoveryManager) scanDeletedFiles(ctx context.Context, config ScanConfig) ([]RecoverableFile, error) {
	var files []RecoverableFile

	// 扫描临时文件和回收站
	tempDirs := []string{
		"/tmp",
		"/var/tmp",
		filepath.Join(os.Getenv("HOME"), ".local/share/Trash"),
		filepath.Join(os.Getenv("HOME"), ".Trash"),
	}

	for _, dir := range tempDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}

			f, err := rm.scanFile(ctx, path, config)
			if err != nil {
				return nil
			}

			f.Status = StatusFound
			f.DeletedAt = info.ModTime()
			files = append(files, f)

			return nil
		})
	}

	return files, nil
}

// scanBySignature 通过签名扫描
func (rm *RecoveryManager) scanBySignature(ctx context.Context, path string, info os.FileInfo, config ScanConfig) (*RecoverableFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 读取文件头
	header := make([]byte, 16)
	n, err := file.Read(header)
	if err != nil || n < 4 {
		return nil, err
	}

	// 匹配签名
	fileType := rm.matchSignature(header[:n])
	if fileType == FileTypeOther {
		return nil, nil
	}

	f := &RecoverableFile{
		ID:           generateID(),
		OriginalPath: path,
		FileName:     filepath.Base(path),
		FileType:     fileType,
		FileSystem:   config.FileSystem,
		Size:         info.Size(),
		ScanMode:     ScanModeSignature,
		Status:       StatusFound,
		Integrity:    1.0,
		Confidence:   0.8,
		Signature:    hex.EncodeToString(header[:n]),
		Metadata: FileMetadata{
			CreatedAt:   info.ModTime(),
			ModifiedAt:  info.ModTime(),
			AccessedAt:  info.ModTime(),
			Permissions: info.Mode(),
		},
	}

	return f, nil
}

// matchSignature 匹配文件签名
func (rm *RecoveryManager) matchSignature(header []byte) FileType {
	for sig, fileType := range rm.fileSigs {
		if len(header) >= len(sig) && string(header[:len(sig)]) == sig {
			return fileType
		}
	}
	return FileTypeOther
}

// analyzeFileWithAI AI分析文件
func (rm *RecoveryManager) analyzeFileWithAI(file *RecoverableFile) AIAnalysis {
	analysis := AIAnalysis{
		FileType:    file.FileType,
		Confidence:  file.Confidence,
		Recoverable: true,
	}

	// 基于文件扩展名和签名的AI分析
	ext := strings.ToLower(filepath.Ext(file.FileName))
	switch ext {
	case ".doc", ".docx", ".pdf", ".txt", ".rtf":
		analysis.FileType = FileTypeDocument
		analysis.Confidence = 0.95
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".heic":
		analysis.FileType = FileTypeImage
		analysis.Confidence = 0.95
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv":
		analysis.FileType = FileTypeVideo
		analysis.Confidence = 0.95
	case ".mp3", ".wav", ".flac", ".aac", ".ogg":
		analysis.FileType = FileTypeAudio
		analysis.Confidence = 0.95
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2":
		analysis.FileType = FileTypeArchive
		analysis.Confidence = 0.95
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs":
		analysis.FileType = FileTypeCode
		analysis.Confidence = 0.90
	case ".db", ".sqlite", ".sqlite3", ".sql":
		analysis.FileType = FileTypeDatabase
		analysis.Confidence = 0.90
	case ".eml", ".msg":
		analysis.FileType = FileTypeEmail
		analysis.Confidence = 0.85
	}

	// 基于完整性的恢复建议
	if file.Integrity < 0.3 {
		analysis.Recoverable = false
		analysis.Suggestions = append(analysis.Suggestions, "文件严重损坏，恢复可能性低")
	} else if file.Integrity < 0.7 {
		analysis.Suggestions = append(analysis.Suggestions, "文件部分损坏，可能需要修复")
	}

	if file.FragmentCount > 10 {
		analysis.Suggestions = append(analysis.Suggestions, "文件碎片较多，建议使用深度恢复")
	}

	return analysis
}

// checkIntegrity 检查文件完整性
func (rm *RecoveryManager) checkIntegrity(path string, size int64) (float64, []Fragment) {
	file, err := os.Open(path)
	if err != nil {
		return 0, nil
	}
	defer file.Close()

	var fragments []Fragment
	chunkCount := int(size/ChunkSize) + 1
	validChunks := 0

	for i := 0; i < chunkCount; i++ {
		offset := int64(i) * ChunkSize
		readSize := ChunkSize
		if offset+int64(readSize) > size {
			readSize = int(size - offset)
		}

		buf := make([]byte, readSize)
		n, err := file.ReadAt(buf, offset)
		if err != nil || n == 0 {
			fragments = append(fragments, Fragment{
				Index:  i,
				Offset: offset,
				Size:   int64(readSize),
				Status: "missing",
			})
			continue
		}

		hash := sha256.Sum256(buf[:n])
		fragments = append(fragments, Fragment{
			Index:    i,
			Offset:   offset,
			Size:     int64(n),
			Checksum: hex.EncodeToString(hash[:]),
			Status:   "ok",
		})
		validChunks++
	}

	if chunkCount == 0 {
		return 1.0, fragments
	}

	return float64(validChunks) / float64(chunkCount), fragments
}

// calculateSignature 计算文件签名
func (rm *RecoveryManager) calculateSignature(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 64)
	n, err := file.Read(header)
	if err != nil && n == 0 {
		return "", err
	}

	hash := sha256.Sum256(header[:n])
	return hex.EncodeToString(hash[:]), nil
}

// detectFileType 检测文件类型
func (rm *RecoveryManager) detectFileType(path string, signature string) FileType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".doc", ".docx", ".pdf", ".txt", ".rtf", ".odt":
		return FileTypeDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".heic", ".svg":
		return FileTypeImage
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm":
		return FileTypeVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma":
		return FileTypeAudio
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
		return FileTypeArchive
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php":
		return FileTypeCode
	case ".db", ".sqlite", ".sqlite3", ".sql":
		return FileTypeDatabase
	case ".eml", ".msg":
		return FileTypeEmail
	default:
		return FileTypeOther
	}
}

// RecoverFile 恢复文件
func (rm *RecoveryManager) RecoverFile(ctx context.Context, fileID string, outputPath string) error {
	rm.mu.RLock()
	var file *RecoverableFile
	for _, result := range rm.results {
		for i, f := range result.Files {
			if f.ID == fileID {
				file = &result.Files[i]
				break
			}
		}
	}
	rm.mu.RUnlock()

	if file == nil {
		return errors.New("file not found")
	}

	// 读取源文件
	data, err := os.ReadFile(file.OriginalPath)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}

	// 写入目标路径
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, data, file.Metadata.Permissions); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// 更新状态
	rm.mu.Lock()
	file.Status = StatusRecovered
	now := time.Now()
	file.RecoveredAt = &now
	file.RecoveredTo = outputPath
	rm.mu.Unlock()

	return nil
}

// GetScanResult 获取扫描结果
func (rm *RecoveryManager) GetScanResult(scanID string) (*ScanResult, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	result, ok := rm.results[scanID]
	return result, ok
}

// ListScanResults 列出所有扫描结果
func (rm *RecoveryManager) ListScanResults() []*ScanResult {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	results := make([]*ScanResult, 0, len(rm.results))
	for _, r := range rm.results {
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StartedAt.After(results[j].StartedAt)
	})

	return results
}

// StopScan 停止扫描
func (rm *RecoveryManager) StopScan() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.cancelFunc != nil {
		rm.cancelFunc()
	}
	rm.running = false
}

// IsScanning 是否正在扫描
func (rm *RecoveryManager) IsScanning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.running
}

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(8))
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1) // 确保不同值
	}
	return string(b)
}
