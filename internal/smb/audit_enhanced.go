package smb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== 操作类型定义 ==========

// AuditOperationType 审计操作类型.
type AuditOperationType string

const (
	// 文件操作
	OpFileCreate  AuditOperationType = "file_create"
	OpFileDelete  AuditOperationType = "file_delete"
	OpFileModify  AuditOperationType = "file_modify"
	OpFileRename  AuditOperationType = "file_rename"
	OpFileRead    AuditOperationType = "file_read"
	OpFileCopy    AuditOperationType = "file_copy"

	// 目录操作
	OpDirCreate AuditOperationType = "dir_create"
	OpDirDelete AuditOperationType = "dir_delete"

	// 权限操作
	OpPermissionChange AuditOperationType = "permission_change"
	OpPermissionGrant  AuditOperationType = "permission_grant"
	OpPermissionRevoke AuditOperationType = "permission_revoke"

	// 连接操作
	OpConnection    AuditOperationType = "connection"
	OpDisconnection AuditOperationType = "disconnection"

	// 认证操作
	OpAuthSuccess AuditOperationType = "auth_success"
	OpAuthFailure AuditOperationType = "auth_failure"

	// 通用
	OpAccess AuditOperationType = "access"
)

// ========== 增强审计条目 ==========

// EnhancedAuditEntry 增强审计日志条目.
type EnhancedAuditEntry struct {
	// 基础字段
	ID        string             `json:"id"`
	Timestamp time.Time          `json:"timestamp"`
	Operation AuditOperationType `json:"operation"`
	IP        string             `json:"ip"`
	Username  string             `json:"username"`

	// 资源信息
	ShareName string `json:"share_name,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	OldPath   string `json:"old_path,omitempty"` // 重命名操作使用
	NewPath   string `json:"new_path,omitempty"` // 重命名操作使用

	// 操作详情
	Result  string `json:"result"` // success, denied, error
	Details string `json:"details,omitempty"`

	// 权限相关
	OldPermission string `json:"old_permission,omitempty"`
	NewPermission string `json:"new_permission,omitempty"`

	// 连接相关
	ClientName string `json:"client_name,omitempty"`
	Protocol   string `json:"protocol,omitempty"` // SMB2, SMB3

	// 元数据
	FileSize    int64  `json:"file_size,omitempty"`
	IsDirectory bool   `json:"is_directory,omitempty"`
	ErrorMsg    string `json:"error_msg,omitempty"`
}

// GenerateID 生成审计条目ID.
func GenerateID() string {
	return fmt.Sprintf("%d-%06d", time.Now().UnixNano(), time.Now().Nanosecond()%1000000)
}

// ========== 查询过滤器 ==========

// AuditQueryFilter 审计查询过滤器.
type AuditQueryFilter struct {
	// 时间范围
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`

	// 用户过滤
	Username string `json:"username,omitempty"`

	// 操作类型过滤（支持多个）
	Operations []AuditOperationType `json:"operations,omitempty"`

	// 资源过滤
	ShareName string `json:"share_name,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	IP        string `json:"ip,omitempty"`

	// 结果过滤
	Result string `json:"result,omitempty"` // success, denied, error

	// 分页
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ========== 增强审计日志记录器 ==========

// EnhancedAuditLogger 增强审计日志记录器.
type EnhancedAuditLogger struct {
	mu       sync.RWMutex
	entries  []EnhancedAuditEntry
	filePath string
	logger   *zap.SugaredLogger

	// 写入配置
	autoFlush    bool
	flushEntries int // 达到此数量自动刷新到磁盘
}

// EnhancedAuditLoggerConfig 增强审计日志配置.
type EnhancedAuditLoggerConfig struct {
	FilePath     string `json:"file_path"`
	AutoFlush    bool   `json:"auto_flush"`
	FlushEntries int    `json:"flush_entries"` // 条目数达到此值时自动刷新
}

// NewEnhancedAuditLogger 创建增强审计日志记录器.
func NewEnhancedAuditLogger(config EnhancedAuditLoggerConfig, logger *zap.SugaredLogger) (*EnhancedAuditLogger, error) {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	if config.FilePath == "" {
		return nil, fmt.Errorf("审计日志文件路径不能为空")
	}

	// 确保目录存在
	dir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("创建审计日志目录失败: %w", err)
	}

	flushEntries := config.FlushEntries
	if flushEntries <= 0 {
		flushEntries = 100
	}

	al := &EnhancedAuditLogger{
		entries:      make([]EnhancedAuditEntry, 0),
		filePath:     config.FilePath,
		logger:       logger,
		autoFlush:    config.AutoFlush,
		flushEntries: flushEntries,
	}

	// 加载已有日志
	if err := al.loadFromFile(); err != nil {
		logger.Warnw("加载历史审计日志失败", "error", err)
	}

	return al, nil
}

// ========== 核心记录方法 ==========

// Log 记录审计条目.
func (al *EnhancedAuditLogger) Log(entry EnhancedAuditEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if entry.ID == "" {
		entry.ID = GenerateID()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	al.entries = append(al.entries, entry)

	if al.autoFlush && len(al.entries) >= al.flushEntries {
		if err := al.flushToFileLocked(); err != nil {
			al.logger.Warnw("自动刷新审计日志失败", "error", err)
		}
	}
}

// LogFileOperation 记录文件操作审计.
func (al *EnhancedAuditLogger) LogFileOperation(op AuditOperationType, ip, username, shareName, filePath string, result string, opts ...func(*EnhancedAuditEntry)) {
	entry := EnhancedAuditEntry{
		Operation: op,
		IP:        ip,
		Username:  username,
		ShareName: shareName,
		FilePath:  filePath,
		Result:    result,
	}

	for _, opt := range opts {
		opt(&entry)
	}

	al.Log(entry)
}

// LogFileRename 记录文件重命名审计.
func (al *EnhancedAuditLogger) LogFileRename(ip, username, shareName, oldPath, newPath string, result string) {
	al.Log(EnhancedAuditEntry{
		Operation: OpFileRename,
		IP:        ip,
		Username:  username,
		ShareName: shareName,
		OldPath:   oldPath,
		NewPath:   newPath,
		Result:    result,
	})
}

// LogDirOperation 记录目录操作审计.
func (al *EnhancedAuditLogger) LogDirOperation(op AuditOperationType, ip, username, shareName, dirPath string, result string) {
	al.Log(EnhancedAuditEntry{
		Operation:   op,
		IP:          ip,
		Username:    username,
		ShareName:   shareName,
		FilePath:    dirPath,
		Result:      result,
		IsDirectory: true,
	})
}

// LogPermissionChange 记录权限变更审计.
func (al *EnhancedAuditLogger) LogPermissionChange(ip, username, shareName, filePath, oldPerm, newPerm string) {
	al.Log(EnhancedAuditEntry{
		Operation:     OpPermissionChange,
		IP:            ip,
		Username:      username,
		ShareName:     shareName,
		FilePath:      filePath,
		Result:        "success",
		OldPermission: oldPerm,
		NewPermission: newPerm,
	})
}

// LogPermissionGrant 记录权限授予审计.
func (al *EnhancedAuditLogger) LogPermissionGrant(ip, username, shareName, targetUser, permission string) {
	al.Log(EnhancedAuditEntry{
		Operation:     OpPermissionGrant,
		IP:            ip,
		Username:      username,
		ShareName:     shareName,
		Result:        "success",
		NewPermission: permission,
		Details:       fmt.Sprintf("授予用户 %s 权限: %s", targetUser, permission),
	})
}

// LogPermissionRevoke 记录权限撤销审计.
func (al *EnhancedAuditLogger) LogPermissionRevoke(ip, username, shareName, targetUser, permission string) {
	al.Log(EnhancedAuditEntry{
		Operation:     OpPermissionRevoke,
		IP:            ip,
		Username:      username,
		ShareName:     shareName,
		Result:        "success",
		OldPermission: permission,
		Details:       fmt.Sprintf("撤销用户 %s 权限: %s", targetUser, permission),
	})
}

// LogConnection 记录SMB连接审计.
func (al *EnhancedAuditLogger) LogConnection(ip, username, clientName, protocol, shareName string) {
	al.Log(EnhancedAuditEntry{
		Operation:  OpConnection,
		IP:         ip,
		Username:   username,
		ClientName: clientName,
		Protocol:   protocol,
		ShareName:  shareName,
		Result:     "success",
	})
}

// LogDisconnection 记录SMB断开审计.
func (al *EnhancedAuditLogger) LogDisconnection(ip, username, clientName, protocol, shareName string, details string) {
	al.Log(EnhancedAuditEntry{
		Operation:  OpDisconnection,
		IP:         ip,
		Username:   username,
		ClientName: clientName,
		Protocol:   protocol,
		ShareName:  shareName,
		Result:     "success",
		Details:    details,
	})
}

// ========== 查询方法 ==========

// Query 查询审计日志.
func (al *EnhancedAuditLogger) Query(filter AuditQueryFilter) []EnhancedAuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var result []EnhancedAuditEntry

	for _, entry := range al.entries {
		if !al.matchFilter(entry, filter) {
			continue
		}
		result = append(result, entry)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// 分页
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result
}

// matchFilter 检查条目是否匹配过滤条件.
func (al *EnhancedAuditLogger) matchFilter(entry EnhancedAuditEntry, filter AuditQueryFilter) bool {
	// 时间范围
	if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
		return false
	}

	// 用户
	if filter.Username != "" && entry.Username != filter.Username {
		return false
	}

	// 操作类型
	if len(filter.Operations) > 0 {
		matched := false
		for _, op := range filter.Operations {
			if entry.Operation == op {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 共享名
	if filter.ShareName != "" && entry.ShareName != filter.ShareName {
		return false
	}

	// 文件路径
	if filter.FilePath != "" && entry.FilePath != filter.FilePath {
		return false
	}

	// IP
	if filter.IP != "" && entry.IP != filter.IP {
		return false
	}

	// 结果
	if filter.Result != "" && entry.Result != filter.Result {
		return false
	}

	return true
}

// ========== 统计方法 ==========

// AuditStats 审计统计.
type AuditStats struct {
	TotalEntries  int                      `json:"total_entries"`
	ByOperation   map[string]int           `json:"by_operation"`
	ByResult      map[string]int           `json:"by_result"`
	ByUser        map[string]int           `json:"by_user"`
	ByShare       map[string]int           `json:"by_share"`
	TimeRange     *TimeRange               `json:"time_range,omitempty"`
}

// TimeRange 时间范围.
type TimeRange struct {
	Earliest time.Time `json:"earliest"`
	Latest   time.Time `json:"latest"`
}

// GetStats 获取审计统计信息.
func (al *EnhancedAuditLogger) GetStats() AuditStats {
	al.mu.RLock()
	defer al.mu.RUnlock()

	stats := AuditStats{
		TotalEntries: len(al.entries),
		ByOperation:  make(map[string]int),
		ByResult:     make(map[string]int),
		ByUser:       make(map[string]int),
		ByShare:      make(map[string]int),
	}

	if len(al.entries) == 0 {
		return stats
	}

	var earliest, latest time.Time
	first := true

	for _, entry := range al.entries {
		stats.ByOperation[string(entry.Operation)]++
		stats.ByResult[entry.Result]++
		if entry.Username != "" {
			stats.ByUser[entry.Username]++
		}
		if entry.ShareName != "" {
			stats.ByShare[entry.ShareName]++
		}

		if first {
			earliest = entry.Timestamp
			latest = entry.Timestamp
			first = false
		} else {
			if entry.Timestamp.Before(earliest) {
				earliest = entry.Timestamp
			}
			if entry.Timestamp.After(latest) {
				latest = entry.Timestamp
			}
		}
	}

	stats.TimeRange = &TimeRange{
		Earliest: earliest,
		Latest:   latest,
	}

	return stats
}

// ========== 持久化方法 ==========

// Flush 手动刷新审计日志到磁盘.
func (al *EnhancedAuditLogger) Flush() error {
	al.mu.Lock()
	defer al.mu.Unlock()
	return al.flushToFileLocked()
}

// flushToFileLocked 将日志写入文件（调用时已持有写锁）.
func (al *EnhancedAuditLogger) flushToFileLocked() error {
	if len(al.entries) == 0 {
		return nil
	}

	dir := filepath.Dir(al.filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	file, err := os.Create(al.filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(al.entries); err != nil {
		return fmt.Errorf("编码JSON失败: %w", err)
	}

	return nil
}

// loadFromFile 从文件加载审计日志.
func (al *EnhancedAuditLogger) loadFromFile() error {
	data, err := os.ReadFile(al.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(data) == 0 {
		return nil
	}

	var entries []EnhancedAuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	al.entries = entries
	return nil
}

// Close 关闭审计日志记录器.
func (al *EnhancedAuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if len(al.entries) > 0 {
		return al.flushToFileLocked()
	}
	return nil
}

// ========== 导出方法 ==========

// ExportJSON 导出审计日志为JSON格式.
func (al *EnhancedAuditLogger) ExportJSON(filter AuditQueryFilter) ([]byte, error) {
	entries := al.Query(filter)
	return json.MarshalIndent(entries, "", "  ")
}

// ========== 辅助函数 ==========

// WithFileSize 设置文件大小的选项函数.
func WithFileSize(size int64) func(*EnhancedAuditEntry) {
	return func(e *EnhancedAuditEntry) {
		e.FileSize = size
	}
}

// WithDetails 设置详情的选项函数.
func WithDetails(details string) func(*EnhancedAuditEntry) {
	return func(e *EnhancedAuditEntry) {
		e.Details = details
	}
}

// WithError 设置错误信息的选项函数.
func WithError(err string) func(*EnhancedAuditEntry) {
	return func(e *EnhancedAuditEntry) {
		e.ErrorMsg = err
		e.Result = "error"
	}
}

// WithIsDirectory 设置是否为目录的选项函数.
func WithIsDirectory(isDir bool) func(*EnhancedAuditEntry) {
	return func(e *EnhancedAuditEntry) {
		e.IsDirectory = isDir
	}
}

// TotalEntries 返回总条目数.
func (al *EnhancedAuditLogger) TotalEntries() int {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return len(al.entries)
}

// Clear 清空审计日志.
func (al *EnhancedAuditLogger) Clear() {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.entries = make([]EnhancedAuditEntry, 0)
}
