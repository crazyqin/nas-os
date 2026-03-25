// Package audit 提供 SMB/NFS 文件操作审计日志功能
// 参考：群晖 DSM 7.3 和 TrueNAS Scale 审计日志实现
package audit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== SMB/NFS 审计类型定义 ==========

// Protocol 协议类型.
type Protocol string

// Protocol constants.
const (
	ProtocolSMB    Protocol = "smb"
	ProtocolNFS    Protocol = "nfs"
	ProtocolFTP    Protocol = "ftp"
	ProtocolWebDAV Protocol = "webdav"
)

// FileOperation 文件操作类型.
type FileOperation string

// FileOperation constants.
const (
	OpCreate   FileOperation = "create"   // 创建文件/目录
	OpRead     FileOperation = "read"     // 读取文件
	OpWrite    FileOperation = "write"    // 写入/修改文件
	OpDelete   FileOperation = "delete"   // 删除文件/目录
	OpRename   FileOperation = "rename"   // 重命名
	OpMove     FileOperation = "move"     // 移动
	OpCopy     FileOperation = "copy"     // 复制
	OpMkdir    FileOperation = "mkdir"    // 创建目录
	OpRmdir    FileOperation = "rmdir"    // 删除目录
	OpList     FileOperation = "list"     // 列出目录内容
	OpChmod    FileOperation = "chmod"    // 修改权限
	OpChown    FileOperation = "chown"    // 修改所有者
	OpLock     FileOperation = "lock"     // 文件锁定
	OpUnlock   FileOperation = "unlock"   // 解锁
	OpDownload FileOperation = "download" // 下载
	OpUpload   FileOperation = "upload"   // 上传
)

// 文件操作状态常量.
const (
	StatusDenied Status = "denied" // 拒绝
)

// ========== SMB/NFS 审计日志条目 ==========

// FileAuditEntry SMB/NFS 文件操作审计日志条目.
type FileAuditEntry struct {
	// 基本信息
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// 协议信息
	Protocol  Protocol `json:"protocol"`   // smb/nfs
	ShareName string   `json:"share_name"` // 共享名称
	SharePath string   `json:"share_path"` // 共享路径

	// 用户信息
	UserID    string `json:"user_id"`              // 用户ID
	Username  string `json:"username"`             // 用户名
	GroupID   string `json:"group_id,omitempty"`   // 组ID
	GroupName string `json:"group_name,omitempty"` // 组名

	// 客户端信息
	ClientIP   string `json:"client_ip"`             // 客户端IP
	ClientPort int    `json:"client_port,omitempty"` // 客户端端口

	// 操作信息
	Operation FileOperation `json:"operation"` // 操作类型
	Status    Status        `json:"status"`    // 操作状态

	// 文件信息
	FilePath    string `json:"file_path"`           // 文件路径
	FileName    string `json:"file_name"`           // 文件名
	FileSize    int64  `json:"file_size,omitempty"` // 文件大小
	FileMode    string `json:"file_mode,omitempty"` // 文件权限
	IsDirectory bool   `json:"is_directory"`        // 是否目录

	// 变更信息（用于重命名、移动等操作）
	OldPath string `json:"old_path,omitempty"` // 原路径
	OldName string `json:"old_name,omitempty"` // 原文件名
	NewPath string `json:"new_path,omitempty"` // 新路径
	NewName string `json:"new_name,omitempty"` // 新文件名

	// 扩展信息
	SessionID string                 `json:"session_id,omitempty"` // 会话ID
	ProcessID int                    `json:"process_id,omitempty"` // 进程ID
	Duration  int64                  `json:"duration,omitempty"`   // 操作耗时(ms)
	ErrorMsg  string                 `json:"error_msg,omitempty"`  // 错误信息
	Details   map[string]interface{} `json:"details,omitempty"`    // 详细信息

	// 安全签名
	Signature string `json:"signature,omitempty"` // 数字签名（防篡改）
}

// ========== 审计日志记录器 ==========

// FileAuditLogger SMB/NFS 文件操作审计日志记录器.
type FileAuditLogger struct {
	config     FileAuditConfig
	entries    []*FileAuditEntry
	storage    *FileAuditStorage
	signingKey []byte
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// FileAuditConfig 审计日志配置.
type FileAuditConfig struct {
	// 基本配置
	Enabled       bool   `json:"enabled"`
	LogPath       string `json:"log_path"`        // 日志存储路径
	MaxMemorySize int    `json:"max_memory_size"` // 内存最大条目数

	// 日志轮转配置
	MaxFileSize  int64 `json:"max_file_size"`  // 单文件最大大小(MB)
	MaxFileCount int   `json:"max_file_count"` // 最大文件数
	MaxAgeDays   int   `json:"max_age_days"`   // 最大保留天数
	CompressAge  int   `json:"compress_age"`   // 压缩阈值(天)

	// 安全配置
	EnableSignatures bool `json:"enable_signatures"` // 启用签名防篡改

	// 过滤配置
	ExcludeOperations []FileOperation `json:"exclude_operations"` // 排除的操作
	ExcludePaths      []string        `json:"exclude_paths"`      // 排除的路径
	ExcludeUsers      []string        `json:"exclude_users"`      // 排除的用户

	// 存储间隔
	FlushInterval time.Duration `json:"flush_interval"` // 刷新间隔
}

// DefaultFileAuditConfig 默认配置.
func DefaultFileAuditConfig() FileAuditConfig {
	return FileAuditConfig{
		Enabled:           true,
		LogPath:           "/var/log/nas-os/audit/file-operations",
		MaxMemorySize:     100000,
		MaxFileSize:       100, // 100MB
		MaxFileCount:      30,  // 保留30个文件
		MaxAgeDays:        90,  // 保留90天
		CompressAge:       7,   // 7天后压缩
		EnableSignatures:  true,
		ExcludeOperations: []FileOperation{OpRead, OpList}, // 默认排除读取和列表操作
		FlushInterval:     time.Minute,
	}
}

// NewFileAuditLogger 创建文件操作审计日志记录器.
func NewFileAuditLogger(config FileAuditConfig) (*FileAuditLogger, error) {
	// 创建存储目录
	if err := os.MkdirAll(config.LogPath, 0750); err != nil {
		return nil, fmt.Errorf("创建审计日志目录失败: %w", err)
	}

	// 创建存储管理器
	storage, err := NewFileAuditStorage(config.LogPath, config.MaxFileSize, config.MaxFileCount, config.MaxAgeDays, config.CompressAge)
	if err != nil {
		return nil, fmt.Errorf("创建审计存储管理器失败: %w", err)
	}

	logger := &FileAuditLogger{
		config:     config,
		entries:    make([]*FileAuditEntry, 0),
		storage:    storage,
		signingKey: []byte(uuid.New().String()),
		stopCh:     make(chan struct{}),
	}

	// 启动定时刷新
	go logger.flushLoop()

	// 启动日志清理
	go logger.cleanupLoop()

	return logger, nil
}

// Stop 停止日志记录器.
func (l *FileAuditLogger) Stop() {
	close(l.stopCh)
	l.flush()
}

// ========== 日志记录方法 ==========

// Log 记录文件操作审计日志.
func (l *FileAuditLogger) Log(ctx context.Context, entry *FileAuditEntry) error {
	if !l.config.Enabled {
		return nil
	}

	// 检查是否排除的操作
	if l.isExcludedOperation(entry.Operation) {
		return nil
	}

	// 检查是否排除的路径
	if l.isExcludedPath(entry.FilePath) {
		return nil
	}

	// 检查是否排除的用户
	if l.isExcludedUser(entry.Username) {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 设置默认值
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 生成签名
	if l.config.EnableSignatures {
		entry.Signature = l.generateSignature(entry)
	}

	// 添加到内存
	l.entries = append(l.entries, entry)

	// 限制内存条目数
	if len(l.entries) > l.config.MaxMemorySize {
		l.entries = l.entries[len(l.entries)-l.config.MaxMemorySize:]
	}

	// 写入存储
	if err := l.storage.Write(entry); err != nil {
		// 存储失败不影响内存记录
		return fmt.Errorf("写入审计日志存储失败: %w", err)
	}

	return nil
}

// LogSMBOperation 记录 SMB 操作.
func (l *FileAuditLogger) LogSMBOperation(ctx context.Context, shareName, sharePath, userID, username, clientIP string, op FileOperation, filePath string, status Status, details map[string]interface{}) error {
	entry := &FileAuditEntry{
		Protocol:  ProtocolSMB,
		ShareName: shareName,
		SharePath: sharePath,
		UserID:    userID,
		Username:  username,
		ClientIP:  clientIP,
		Operation: op,
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		Status:    status,
		Details:   details,
	}
	return l.Log(ctx, entry)
}

// LogNFSOperation 记录 NFS 操作.
func (l *FileAuditLogger) LogNFSOperation(ctx context.Context, sharePath, userID, username, clientIP string, op FileOperation, filePath string, status Status, details map[string]interface{}) error {
	entry := &FileAuditEntry{
		Protocol:  ProtocolNFS,
		SharePath: sharePath,
		UserID:    userID,
		Username:  username,
		ClientIP:  clientIP,
		Operation: op,
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		Status:    status,
		Details:   details,
	}
	return l.Log(ctx, entry)
}

// LogFileCreate 记录文件创建.
func (l *FileAuditLogger) LogFileCreate(ctx context.Context, protocol Protocol, shareName, sharePath, userID, username, clientIP, filePath string, isDir bool, status Status) error {
	entry := &FileAuditEntry{
		Protocol:    protocol,
		ShareName:   shareName,
		SharePath:   sharePath,
		UserID:      userID,
		Username:    username,
		ClientIP:    clientIP,
		Operation:   OpCreate,
		FilePath:    filePath,
		FileName:    filepath.Base(filePath),
		IsDirectory: isDir,
		Status:      status,
	}
	return l.Log(ctx, entry)
}

// LogFileDelete 记录文件删除.
func (l *FileAuditLogger) LogFileDelete(ctx context.Context, protocol Protocol, shareName, sharePath, userID, username, clientIP, filePath string, isDir bool, status Status) error {
	entry := &FileAuditEntry{
		Protocol:    protocol,
		ShareName:   shareName,
		SharePath:   sharePath,
		UserID:      userID,
		Username:    username,
		ClientIP:    clientIP,
		Operation:   OpDelete,
		FilePath:    filePath,
		FileName:    filepath.Base(filePath),
		IsDirectory: isDir,
		Status:      status,
	}
	return l.Log(ctx, entry)
}

// LogFileRename 记录文件重命名.
func (l *FileAuditLogger) LogFileRename(ctx context.Context, protocol Protocol, shareName, sharePath, userID, username, clientIP, oldPath, newPath string, status Status) error {
	entry := &FileAuditEntry{
		Protocol:  protocol,
		ShareName: shareName,
		SharePath: sharePath,
		UserID:    userID,
		Username:  username,
		ClientIP:  clientIP,
		Operation: OpRename,
		FilePath:  newPath,
		FileName:  filepath.Base(newPath),
		OldPath:   oldPath,
		OldName:   filepath.Base(oldPath),
		NewPath:   newPath,
		NewName:   filepath.Base(newPath),
		Status:    status,
	}
	return l.Log(ctx, entry)
}

// LogFileMove 记录文件移动.
func (l *FileAuditLogger) LogFileMove(ctx context.Context, protocol Protocol, shareName, sharePath, userID, username, clientIP, oldPath, newPath string, status Status) error {
	entry := &FileAuditEntry{
		Protocol:  protocol,
		ShareName: shareName,
		SharePath: sharePath,
		UserID:    userID,
		Username:  username,
		ClientIP:  clientIP,
		Operation: OpMove,
		FilePath:  newPath,
		FileName:  filepath.Base(newPath),
		OldPath:   oldPath,
		OldName:   filepath.Base(oldPath),
		NewPath:   newPath,
		NewName:   filepath.Base(newPath),
		Status:    status,
	}
	return l.Log(ctx, entry)
}

// LogFileWrite 记录文件写入.
func (l *FileAuditLogger) LogFileWrite(ctx context.Context, protocol Protocol, shareName, sharePath, userID, username, clientIP, filePath string, fileSize int64, status Status) error {
	entry := &FileAuditEntry{
		Protocol:  protocol,
		ShareName: shareName,
		SharePath: sharePath,
		UserID:    userID,
		Username:  username,
		ClientIP:  clientIP,
		Operation: OpWrite,
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		FileSize:  fileSize,
		Status:    status,
	}
	return l.Log(ctx, entry)
}

// ========== 查询方法 ==========

// FileAuditQueryOptions 查询选项.
type FileAuditQueryOptions struct {
	Limit     int           `json:"limit"`
	Offset    int           `json:"offset"`
	StartTime *time.Time    `json:"start_time,omitempty"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
	Protocol  Protocol      `json:"protocol,omitempty"`
	UserID    string        `json:"user_id,omitempty"`
	Username  string        `json:"username,omitempty"`
	ClientIP  string        `json:"client_ip,omitempty"`
	Operation FileOperation `json:"operation,omitempty"`
	Status    Status        `json:"status,omitempty"`
	FilePath  string        `json:"file_path,omitempty"`
	ShareName string        `json:"share_name,omitempty"`
	Keyword   string        `json:"keyword,omitempty"`
}

// FileAuditQueryResult 查询结果.
type FileAuditQueryResult struct {
	Total   int               `json:"total"`
	Entries []*FileAuditEntry `json:"entries"`
}

// Query 查询审计日志.
func (l *FileAuditLogger) Query(opts FileAuditQueryOptions) (*FileAuditQueryResult, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 筛选日志
	filtered := make([]*FileAuditEntry, 0)
	for _, entry := range l.entries {
		if !l.matchesFilter(entry, opts) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// 按时间倒序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// 计算总数
	total := len(filtered)

	// 应用分页
	start := opts.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + opts.Limit
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if end > total {
		end = total
	}

	return &FileAuditQueryResult{
		Total:   total,
		Entries: filtered[start:end],
	}, nil
}

// GetByID 根据ID获取日志.
func (l *FileAuditLogger) GetByID(id string) (*FileAuditEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, entry := range l.entries {
		if entry.ID == id {
			return entry, nil
		}
	}

	return nil, fmt.Errorf("审计日志不存在: %s", id)
}

// ========== 统计方法 ==========

// FileAuditStatistics 审计统计.
type FileAuditStatistics struct {
	TotalOperations int              `json:"total_operations"`
	SuccessCount    int              `json:"success_count"`
	FailureCount    int              `json:"failure_count"`
	DeniedCount     int              `json:"denied_count"`
	ByProtocol      map[string]int   `json:"by_protocol"`
	ByOperation     map[string]int   `json:"by_operation"`
	ByUser          []UserAuditStat  `json:"by_user"`
	ByIP            []IPAuditStat    `json:"by_ip"`
	ByShare         []ShareAuditStat `json:"by_share"`
	TopFiles        []FileAuditStat  `json:"top_files"`
	TodayOperations int              `json:"today_operations"`
	WeekOperations  int              `json:"week_operations"`
	OldestEntry     *time.Time       `json:"oldest_entry,omitempty"`
	NewestEntry     *time.Time       `json:"newest_entry,omitempty"`
}

// UserAuditStat 用户审计统计.
type UserAuditStat struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Count    int    `json:"count"`
}

// IPAuditStat IP审计统计.
type IPAuditStat struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// ShareAuditStat 共享审计统计.
type ShareAuditStat struct {
	ShareName string `json:"share_name"`
	Count     int    `json:"count"`
}

// FileAuditStat 文件审计统计.
type FileAuditStat struct {
	FilePath string `json:"file_path"`
	Count    int    `json:"count"`
}

// GetStatistics 获取审计统计.
func (l *FileAuditLogger) GetStatistics() *FileAuditStatistics {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -7)

	stats := &FileAuditStatistics{
		ByProtocol:  make(map[string]int),
		ByOperation: make(map[string]int),
		ByUser:      make([]UserAuditStat, 0),
		ByIP:        make([]IPAuditStat, 0),
		ByShare:     make([]ShareAuditStat, 0),
		TopFiles:    make([]FileAuditStat, 0),
	}

	userCounts := make(map[string]*UserAuditStat)
	ipCounts := make(map[string]*IPAuditStat)
	shareCounts := make(map[string]*ShareAuditStat)
	fileCounts := make(map[string]int)

	for _, entry := range l.entries {
		stats.TotalOperations++

		// 状态统计
		switch entry.Status {
		case StatusSuccess:
			stats.SuccessCount++
		case StatusFailure:
			stats.FailureCount++
		case StatusDenied:
			stats.DeniedCount++
		}

		// 协议统计
		stats.ByProtocol[string(entry.Protocol)]++

		// 操作统计
		stats.ByOperation[string(entry.Operation)]++

		// 今日统计
		if entry.Timestamp.After(todayStart) || entry.Timestamp.Equal(todayStart) {
			stats.TodayOperations++
		}

		// 本周统计
		if entry.Timestamp.After(weekStart) || entry.Timestamp.Equal(weekStart) {
			stats.WeekOperations++
		}

		// 用户统计
		if entry.UserID != "" {
			if u, exists := userCounts[entry.UserID]; exists {
				u.Count++
			} else {
				userCounts[entry.UserID] = &UserAuditStat{
					UserID:   entry.UserID,
					Username: entry.Username,
					Count:    1,
				}
			}
		}

		// IP统计
		if entry.ClientIP != "" {
			if ip, exists := ipCounts[entry.ClientIP]; exists {
				ip.Count++
			} else {
				ipCounts[entry.ClientIP] = &IPAuditStat{IP: entry.ClientIP, Count: 1}
			}
		}

		// 共享统计
		if entry.ShareName != "" {
			if s, exists := shareCounts[entry.ShareName]; exists {
				s.Count++
			} else {
				shareCounts[entry.ShareName] = &ShareAuditStat{ShareName: entry.ShareName, Count: 1}
			}
		}

		// 文件统计
		if entry.FilePath != "" {
			fileCounts[entry.FilePath]++
		}

		// 时间范围
		if stats.OldestEntry == nil || entry.Timestamp.Before(*stats.OldestEntry) {
			stats.OldestEntry = &entry.Timestamp
		}
		if stats.NewestEntry == nil || entry.Timestamp.After(*stats.NewestEntry) {
			stats.NewestEntry = &entry.Timestamp
		}
	}

	// 转换并排序用户统计
	for _, u := range userCounts {
		stats.ByUser = append(stats.ByUser, *u)
	}
	sort.Slice(stats.ByUser, func(i, j int) bool {
		return stats.ByUser[i].Count > stats.ByUser[j].Count
	})
	if len(stats.ByUser) > 10 {
		stats.ByUser = stats.ByUser[:10]
	}

	// 转换并排序IP统计
	for _, ip := range ipCounts {
		stats.ByIP = append(stats.ByIP, *ip)
	}
	sort.Slice(stats.ByIP, func(i, j int) bool {
		return stats.ByIP[i].Count > stats.ByIP[j].Count
	})
	if len(stats.ByIP) > 10 {
		stats.ByIP = stats.ByIP[:10]
	}

	// 转换并排序共享统计
	for _, s := range shareCounts {
		stats.ByShare = append(stats.ByShare, *s)
	}
	sort.Slice(stats.ByShare, func(i, j int) bool {
		return stats.ByShare[i].Count > stats.ByShare[j].Count
	})

	// 转换并排序文件统计
	for path, count := range fileCounts {
		stats.TopFiles = append(stats.TopFiles, FileAuditStat{FilePath: path, Count: count})
	}
	sort.Slice(stats.TopFiles, func(i, j int) bool {
		return stats.TopFiles[i].Count > stats.TopFiles[j].Count
	})
	if len(stats.TopFiles) > 10 {
		stats.TopFiles = stats.TopFiles[:10]
	}

	return stats
}

// ========== 辅助方法 ==========

// isExcludedOperation 检查是否排除的操作.
func (l *FileAuditLogger) isExcludedOperation(op FileOperation) bool {
	for _, excluded := range l.config.ExcludeOperations {
		if op == excluded {
			return true
		}
	}
	return false
}

// isExcludedPath 检查是否排除的路径.
func (l *FileAuditLogger) isExcludedPath(path string) bool {
	for _, excluded := range l.config.ExcludePaths {
		if strings.HasPrefix(path, excluded) {
			return true
		}
	}
	return false
}

// isExcludedUser 检查是否排除的用户.
func (l *FileAuditLogger) isExcludedUser(username string) bool {
	for _, excluded := range l.config.ExcludeUsers {
		if username == excluded {
			return true
		}
	}
	return false
}

// matchesFilter 检查是否匹配筛选条件.
func (l *FileAuditLogger) matchesFilter(entry *FileAuditEntry, opts FileAuditQueryOptions) bool {
	// 时间范围
	if opts.StartTime != nil && entry.Timestamp.Before(*opts.StartTime) {
		return false
	}
	if opts.EndTime != nil && entry.Timestamp.After(*opts.EndTime) {
		return false
	}

	// 协议
	if opts.Protocol != "" && entry.Protocol != opts.Protocol {
		return false
	}

	// 用户
	if opts.UserID != "" && entry.UserID != opts.UserID {
		return false
	}
	if opts.Username != "" && !strings.Contains(strings.ToLower(entry.Username), strings.ToLower(opts.Username)) {
		return false
	}

	// IP
	if opts.ClientIP != "" && entry.ClientIP != opts.ClientIP {
		return false
	}

	// 操作
	if opts.Operation != "" && entry.Operation != opts.Operation {
		return false
	}

	// 状态
	if opts.Status != "" && entry.Status != opts.Status {
		return false
	}

	// 文件路径
	if opts.FilePath != "" && !strings.Contains(entry.FilePath, opts.FilePath) {
		return false
	}

	// 共享名
	if opts.ShareName != "" && entry.ShareName != opts.ShareName {
		return false
	}

	// 关键词
	if opts.Keyword != "" {
		keyword := strings.ToLower(opts.Keyword)
		matched := strings.Contains(strings.ToLower(entry.FileName), keyword) ||
			strings.Contains(strings.ToLower(entry.FilePath), keyword) ||
			strings.Contains(strings.ToLower(entry.Username), keyword) ||
			strings.Contains(strings.ToLower(entry.ShareName), keyword)
		if !matched {
			return false
		}
	}

	return true
}

// generateSignature 生成数字签名.
func (l *FileAuditLogger) generateSignature(entry *FileAuditEntry) string {
	signData := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.Protocol,
		entry.UserID,
		entry.ClientIP,
		entry.Operation,
		entry.FilePath,
		entry.Status,
		entry.ID,
	)

	h := hmac.New(sha256.New, l.signingKey)
	h.Write([]byte(signData))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifySignature 验证签名.
func (l *FileAuditLogger) VerifySignature(entry *FileAuditEntry) bool {
	if entry.Signature == "" {
		return false
	}
	expectedSig := l.generateSignature(entry)
	return hmac.Equal([]byte(entry.Signature), []byte(expectedSig))
}

// ========== 后台任务 ==========

// flushLoop 定时刷新循环.
func (l *FileAuditLogger) flushLoop() {
	ticker := time.NewTicker(l.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.flush()
		case <-l.stopCh:
			return
		}
	}
}

// cleanupLoop 定时清理循环.
func (l *FileAuditLogger) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = l.storage.Cleanup() // 忽略清理错误，继续运行
		case <-l.stopCh:
			return
		}
	}
}

// flush 刷新到存储.
func (l *FileAuditLogger) flush() {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.entries) == 0 {
		return
	}

	// 存储会自动按日期写入，这里只处理内存清理
	// 按日期分组写入
	entriesByDate := make(map[string][]*FileAuditEntry)
	for _, entry := range l.entries {
		date := entry.Timestamp.Format("2006-01-02")
		entriesByDate[date] = append(entriesByDate[date], entry)
	}

	// 批量写入
	for date, entries := range entriesByDate {
		if err := l.storage.WriteBatch(date, entries); err != nil {
			// 写入失败，记录错误但继续
			fmt.Printf("[WARN] 刷新审计日志失败 (%s): %v\n", date, err)
		}
	}
}

// ========== 导出功能 ==========

// FileExportOptions 文件审计导出选项.
type FileExportOptions struct {
	Format    string    `json:"format"` // json, csv
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Protocol  Protocol  `json:"protocol,omitempty"`
	Compress  bool      `json:"compress"`
}

// Export 导出审计日志.
func (l *FileAuditLogger) Export(opts FileExportOptions) ([]byte, error) {
	queryOpts := FileAuditQueryOptions{
		StartTime: &opts.StartTime,
		EndTime:   &opts.EndTime,
		Protocol:  opts.Protocol,
		Limit:     100000, // 最大导出数量
	}

	result, err := l.Query(queryOpts)
	if err != nil {
		return nil, err
	}

	switch opts.Format {
	case "json":
		return json.MarshalIndent(result.Entries, "", "  ")
	case "csv":
		return l.exportToCSV(result.Entries)
	default:
		return json.MarshalIndent(result.Entries, "", "  ")
	}
}

// exportToCSV 导出为CSV.
func (l *FileAuditLogger) exportToCSV(entries []*FileAuditEntry) ([]byte, error) {
	var csv strings.Builder
	csv.WriteString("ID,Timestamp,Protocol,ShareName,UserID,Username,ClientIP,Operation,FilePath,FileName,Status,IsDirectory\n")

	for _, e := range entries {
		fmt.Fprintf(&csv, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%v\n",
			e.ID,
			e.Timestamp.Format(time.RFC3339),
			string(e.Protocol),
			e.ShareName,
			e.UserID,
			e.Username,
			e.ClientIP,
			string(e.Operation),
			strings.ReplaceAll(e.FilePath, ",", ";"),
			strings.ReplaceAll(e.FileName, ",", ";"),
			string(e.Status),
			e.IsDirectory,
		)
	}

	return []byte(csv.String()), nil
}

// ========== 配置管理 ==========

// SetConfig 设置配置.
func (l *FileAuditLogger) SetConfig(config FileAuditConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = config
}

// GetConfig 获取配置.
func (l *FileAuditLogger) GetConfig() FileAuditConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// Enable 启用审计.
func (l *FileAuditLogger) Enable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Enabled = true
}

// Disable 禁用审计.
func (l *FileAuditLogger) Disable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Enabled = false
}
