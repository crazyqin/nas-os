// Package audit 提供审计日志存储和轮转管理
package audit

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileAuditStorage 文件审计存储管理器
// 负责日志文件的存储、轮转和归档
type FileAuditStorage struct {
	basePath      string        // 基础路径
	maxFileSize   int64         // 单文件最大大小(bytes)
	maxFileCount  int           // 最大文件数
	maxAgeDays    int           // 最大保留天数
	compressAge   int           // 压缩阈值(天)
	mu            sync.Mutex
	currentFile   *os.File      // 当前写入文件
	currentDate   string        // 当前日期
	currentSize   int64         // 当前文件大小
	writeBuffer   []*FileAuditEntry // 写入缓冲
	bufferMu      sync.Mutex
}

// NewFileAuditStorage 创建存储管理器
func NewFileAuditStorage(basePath string, maxFileSize int64, maxFileCount, maxAgeDays, compressAge int) (*FileAuditStorage, error) {
	// 确保目录存在
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	
	storage := &FileAuditStorage{
		basePath:     basePath,
		maxFileSize:  maxFileSize * 1024 * 1024, // MB to bytes
		maxFileCount: maxFileCount,
		maxAgeDays:   maxAgeDays,
		compressAge:  compressAge,
		writeBuffer:  make([]*FileAuditEntry, 0),
	}
	
	// 恢复当日文件
	if err := storage.initCurrentFile(); err != nil {
		return nil, err
	}
	
	return storage, nil
}

// initCurrentFile 初始化当前文件
func (s *FileAuditStorage) initCurrentFile() error {
	today := time.Now().Format("2006-01-02")
	filename := s.getLogFilename(today)
	
	// 检查文件是否存在
	if info, err := os.Stat(filename); err == nil {
		s.currentFile, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("打开日志文件失败: %w", err)
		}
		s.currentSize = info.Size()
	} else {
		// 创建新文件
		s.currentFile, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("创建日志文件失败: %w", err)
		}
		s.currentSize = 0
	}
	
	s.currentDate = today
	return nil
}

// getLogFilename 获取日志文件名
func (s *FileAuditStorage) getLogFilename(date string) string {
	return filepath.Join(s.basePath, fmt.Sprintf("file-audit-%s.log", date))
}

// getCompressedFilename 获取压缩文件名
func (s *FileAuditStorage) getCompressedFilename(date string) string {
	return filepath.Join(s.basePath, fmt.Sprintf("file-audit-%s.log.gz", date))
}

// getArchiveFilename 获取归档文件名
func (s *FileAuditStorage) getArchiveFilename(startMonth, endMonth string) string {
	return filepath.Join(s.basePath, "archive", fmt.Sprintf("file-audit-%s-%s.tar.gz", startMonth, endMonth))
}

// Write 写入单条日志
func (s *FileAuditStorage) Write(entry *FileAuditEntry) error {
	s.bufferMu.Lock()
	s.writeBuffer = append(s.writeBuffer, entry)
	s.bufferMu.Unlock()
	
	// 达到缓冲阈值时刷新
	if len(s.writeBuffer) >= 100 {
		return s.FlushBuffer()
	}
	
	return nil
}

// WriteBatch 批量写入日志
func (s *FileAuditStorage) WriteBatch(date string, entries []*FileAuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 检查日期是否变化
	if date != s.currentDate {
		if err := s.rotateDate(date); err != nil {
			return err
		}
	}
	
	// 批量写入
	for _, entry := range entries {
		if err := s.writeEntry(entry); err != nil {
			return err
		}
	}
	
	return nil
}

// writeEntry 写入单条日志到文件
func (s *FileAuditStorage) writeEntry(entry *FileAuditEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("序列化日志失败: %w", err)
	}
	
	data = append(data, '\n')
	
	// 检查是否需要轮转（基于大小）
	if s.currentSize+int64(len(data)) > s.maxFileSize {
		if err := s.rotateSize(); err != nil {
			return err
		}
	}
	
	n, err := s.currentFile.Write(data)
	if err != nil {
		return fmt.Errorf("写入日志失败: %w", err)
	}
	
	s.currentSize += int64(n)
	return nil
}

// FlushBuffer 刷新缓冲区
func (s *FileAuditStorage) FlushBuffer() error {
	s.bufferMu.Lock()
	entries := s.writeBuffer
	s.writeBuffer = make([]*FileAuditEntry, 0)
	s.bufferMu.Unlock()
	
	if len(entries) == 0 {
		return nil
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 按日期分组
	entriesByDate := make(map[string][]*FileAuditEntry)
	for _, entry := range entries {
		date := entry.Timestamp.Format("2006-01-02")
		entriesByDate[date] = append(entriesByDate[date], entry)
	}
	
	// 写入各日期文件
	for date, dateEntries := range entriesByDate {
		if date != s.currentDate {
			if err := s.rotateDate(date); err != nil {
				return err
			}
		}
		
		for _, entry := range dateEntries {
			if err := s.writeEntry(entry); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// rotateDate 日期轮转
func (s *FileAuditStorage) rotateDate(newDate string) error {
	// 关闭当前文件
	if s.currentFile != nil {
		s.currentFile.Close()
	}
	
	// 创建新日期文件
	filename := s.getLogFilename(newDate)
	var err error
	s.currentFile, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("创建新日志文件失败: %w", err)
	}
	
	s.currentDate = newDate
	s.currentSize = 0
	
	return nil
}

// rotateSize 大小轮转
func (s *FileAuditStorage) rotateSize() error {
	// 关闭当前文件
	if s.currentFile != nil {
		s.currentFile.Close()
	}
	
	// 重命名当前文件（添加序号）
	baseFilename := s.getLogFilename(s.currentDate)
	rotatedFilename := fmt.Sprintf("%s.%d", baseFilename, time.Now().UnixNano())
	if err := os.Rename(baseFilename, rotatedFilename); err != nil {
		return fmt.Errorf("重命名日志文件失败: %w", err)
	}
	
	// 创建新文件
	var err error
	s.currentFile, err = os.OpenFile(baseFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("创建新日志文件失败: %w", err)
	}
	
	s.currentSize = 0
	
	return nil
}

// Cleanup 清理过期日志
func (s *FileAuditStorage) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	cutoffDate := now.AddDate(0, 0, -s.maxAgeDays)
	compressDate := now.AddDate(0, 0, -s.compressAge)
	
	// 遍历日志目录
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return fmt.Errorf("读取日志目录失败: %w", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		// 检查是否过期
		if info.ModTime().Before(cutoffDate) {
			filePath := filepath.Join(s.basePath, name)
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("[WARN] 删除过期日志失败: %s: %v\n", name, err)
			} else {
				fmt.Printf("[INFO] 删除过期日志: %s\n", name)
			}
			continue
		}
		
		// 检查是否需要压缩
		if info.ModTime().Before(compressDate) && !strings.HasSuffix(name, ".gz") {
			if err := s.compressFile(name); err != nil {
				fmt.Printf("[WARN] 压缩日志失败: %s: %v\n", name, err)
			}
		}
	}
	
	// 检查文件数量
	s.enforceMaxFileCount()
	
	return nil
}

// compressFile 压缩日志文件
func (s *FileAuditStorage) compressFile(filename string) error {
	srcPath := filepath.Join(s.basePath, filename)
	dstPath := srcPath + ".gz"
	
	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	// 创建目标文件
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	// 创建gzip写入器
	gzWriter := gzip.NewWriter(dstFile)
	defer gzWriter.Close()
	
	// 复制数据
	if _, err := io.Copy(gzWriter, srcFile); err != nil {
		return err
	}
	
	// 删除源文件
	return os.Remove(srcPath)
}

// enforceMaxFileCount 强制最大文件数限制
func (s *FileAuditStorage) enforceMaxFileCount() {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return
	}
	
	// 过滤并排序日志文件
	type logFile struct {
		name    string
		modTime time.Time
	}
	
	logFiles := make([]logFile, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "file-audit-") {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		logFiles = append(logFiles, logFile{
			name:    name,
			modTime: info.ModTime(),
		})
	}
	
	// 按修改时间排序（旧到新）
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].modTime.Before(logFiles[j].modTime)
	})
	
	// 删除多余文件
	if len(logFiles) > s.maxFileCount {
		removeCount := len(logFiles) - s.maxFileCount
		for i := 0; i < removeCount; i++ {
			filePath := filepath.Join(s.basePath, logFiles[i].name)
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("[WARN] 删除多余日志失败: %s: %v\n", logFiles[i].name, err)
			} else {
				fmt.Printf("[INFO] 删除多余日志: %s\n", logFiles[i].name)
			}
		}
	}
}

// Archive 归档日志
func (s *FileAuditStorage) Archive(startMonth, endMonth string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 创建归档目录
	archiveDir := filepath.Join(s.basePath, "archive")
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		return fmt.Errorf("创建归档目录失败: %w", err)
	}
	
	// 查找指定月份的日志文件
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return fmt.Errorf("读取日志目录失败: %w", err)
	}
	
	filesToArchive := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		
		// 检查文件名格式
		// file-audit-2026-01-01.log 或 file-audit-2026-01-01.log.gz
		if !strings.HasPrefix(name, "file-audit-") {
			continue
		}
		
		// 提取日期
		var dateStr string
		if strings.HasSuffix(name, ".gz") {
			dateStr = strings.TrimPrefix(name, "file-audit-")
			dateStr = strings.TrimSuffix(dateStr, ".log.gz")
		} else {
			dateStr = strings.TrimPrefix(name, "file-audit-")
			dateStr = strings.TrimSuffix(dateStr, ".log")
		}
		
		// 检查是否在归档范围内
		// 假设 startMonth 和 endMonth 格式为 "2026-01"
		if strings.HasPrefix(dateStr, startMonth[:7]) || 
		   (len(endMonth) >= 7 && strings.HasPrefix(dateStr, endMonth[:7])) {
			filesToArchive = append(filesToArchive, name)
		}
	}
	
	if len(filesToArchive) == 0 {
		return fmt.Errorf("没有找到需要归档的日志文件")
	}
	
	// 创建归档文件
	archivePath := s.getArchiveFilename(startMonth, endMonth)
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("创建归档文件失败: %w", err)
	}
	defer archiveFile.Close()
	
	gzWriter := gzip.NewWriter(archiveFile)
	defer gzWriter.Close()
	
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()
	
	// 添加文件到归档
	for _, filename := range filesToArchive {
		filePath := filepath.Join(s.basePath, filename)
		
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		
		// 创建tar头
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			continue
		}
		header.Name = filename
		
		if err := tarWriter.WriteHeader(header); err != nil {
			continue
		}
		
		// 写入文件内容
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}
		
		io.Copy(tarWriter, file)
		file.Close()
		
		// 归档后删除原文件
		os.Remove(filePath)
	}
	
	fmt.Printf("[INFO] 创建归档: %s (包含 %d 个文件)\n", archivePath, len(filesToArchive))
	
	return nil
}

// Load 加载指定日期的日志
func (s *FileAuditStorage) Load(date string) ([]*FileAuditEntry, error) {
	filename := s.getLogFilename(date)
	
	// 检查是否有压缩版本
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		filename = filename + ".gz"
	}
	
	return s.loadFile(filename)
}

// loadFile 加载日志文件
func (s *FileAuditStorage) loadFile(filename string) ([]*FileAuditEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var reader io.Reader = file
	
	// 检查是否为gzip文件
	if strings.HasSuffix(filename, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		reader = gzReader
	}
	
	entries := make([]*FileAuditEntry, 0)
	decoder := json.NewDecoder(reader)
	
	for {
		var entry FileAuditEntry
		if err := decoder.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		entries = append(entries, &entry)
	}
	
	return entries, nil
}

// ListAvailableDates 列出可用的日志日期
func (s *FileAuditStorage) ListAvailableDates() ([]string, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, err
	}
	
	dates := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		
		// 提取日期
		var dateStr string
		if strings.HasSuffix(name, ".gz") {
			dateStr = strings.TrimPrefix(name, "file-audit-")
			dateStr = strings.TrimSuffix(dateStr, ".log.gz")
		} else if strings.Contains(name, ".log.") {
			// 轮转的文件，如 file-audit-2026-01-01.log.1234567890
			continue
		} else {
			dateStr = strings.TrimPrefix(name, "file-audit-")
			dateStr = strings.TrimSuffix(dateStr, ".log")
		}
		
		if len(dateStr) == 10 { // YYYY-MM-DD
			dates[dateStr] = true
		}
	}
	
	result := make([]string, 0, len(dates))
	for date := range dates {
		result = append(result, date)
	}
	
	sort.Strings(result)
	return result, nil
}

// GetStorageInfo 获取存储信息
func (s *FileAuditStorage) GetStorageInfo() (*StorageInfo, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, err
	}
	
	info := &StorageInfo{
		TotalSize:  0,
		FileCount:  0,
		OldestDate: "",
		NewestDate: "",
	}
	
	var oldestTime, newestTime time.Time
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}
		
		info.TotalSize += fileInfo.Size()
		info.FileCount++
		
		modTime := fileInfo.ModTime()
		if oldestTime.IsZero() || modTime.Before(oldestTime) {
			oldestTime = modTime
		}
		if newestTime.IsZero() || modTime.After(newestTime) {
			newestTime = modTime
		}
	}
	
	if !oldestTime.IsZero() {
		info.OldestDate = oldestTime.Format("2006-01-02")
	}
	if !newestTime.IsZero() {
		info.NewestDate = newestTime.Format("2006-01-02")
	}
	
	return info, nil
}

// StorageInfo 存储信息
type StorageInfo struct {
	TotalSize  int64  `json:"total_size"`   // 总大小(bytes)
	FileCount  int    `json:"file_count"`   // 文件数量
	OldestDate string `json:"oldest_date"`  // 最早日期
	NewestDate string `json:"newest_date"`  // 最新日期
}

// Close 关闭存储
func (s *FileAuditStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 刷新缓冲
	s.FlushBuffer()
	
	// 关闭文件
	if s.currentFile != nil {
		return s.currentFile.Close()
	}
	return nil
}