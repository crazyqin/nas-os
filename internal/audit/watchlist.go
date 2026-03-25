// Package audit 提供审计日志Watch/Ignore List功能
// 对标TrueNAS的审计监控列表和忽略列表功能
package audit

import (
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== Watch/Ignore List 数据结构 ==========

// ListType 列表类型.
type ListType string

const (
	// ListTypeWatch 监控列表.
	ListTypeWatch ListType = "watch"
	// ListTypeIgnore 忽略列表.
	ListTypeIgnore ListType = "ignore"
)

// WatchOperation 监控的操作类型.
type WatchOperation string

const (
	// WatchOpRead 读取.
	WatchOpRead WatchOperation = "read"
	// WatchOpWrite 写入.
	WatchOpWrite WatchOperation = "write"
	// WatchOpCreate 创建.
	WatchOpCreate WatchOperation = "create"
	// WatchOpDelete 删除.
	WatchOpDelete WatchOperation = "delete"
	// WatchOpRename 重命名.
	WatchOpRename WatchOperation = "rename"
	// WatchOpMove 移动.
	WatchOpMove WatchOperation = "move"
	// WatchOpChmod 权限修改.
	WatchOpChmod WatchOperation = "chmod"
	// WatchOpChown 所有者修改.
	WatchOpChown WatchOperation = "chown"
	// WatchOpAll 所有操作.
	WatchOpAll WatchOperation = "all"
)

// WatchListEntry 监控列表条目.
type WatchListEntry struct {
	ID          string           `json:"id"`                    // 唯一标识
	Path        string           `json:"path"`                  // 文件/目录路径
	Pattern     string           `json:"pattern,omitempty"`     // glob匹配模式（可选）
	Operations  []WatchOperation `json:"operations"`            // 监控的操作类型
	Recursive   bool             `json:"recursive"`             // 是否递归监控子目录
	Enabled     bool             `json:"enabled"`               // 是否启用
	Description string           `json:"description,omitempty"` // 描述
	CreatedAt   time.Time        `json:"created_at"`            // 创建时间
	UpdatedAt   time.Time        `json:"updated_at"`            // 更新时间
	CreatedBy   string           `json:"created_by"`            // 创建者用户ID
	Tags        []string         `json:"tags,omitempty"`        // 标签
}

// IgnoreListEntry 忽略列表条目.
type IgnoreListEntry struct {
	ID          string     `json:"id"`                    // 唯一标识
	Path        string     `json:"path"`                  // 文件/目录路径
	Pattern     string     `json:"pattern,omitempty"`     // glob匹配模式（可选）
	Reason      string     `json:"reason,omitempty"`      // 忽略原因
	Enabled     bool       `json:"enabled"`               // 是否启用
	Description string     `json:"description,omitempty"` // 描述
	CreatedAt   time.Time  `json:"created_at"`            // 创建时间
	UpdatedAt   time.Time  `json:"updated_at"`            // 更新时间
	CreatedBy   string     `json:"created_by"`            // 创建者用户ID
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`  // 过期时间（可选）
	Tags        []string   `json:"tags,omitempty"`        // 标签
}

// WatchListConfig 监控列表配置.
type WatchListConfig struct {
	MaxWatchEntries  int `json:"max_watch_entries"`  // 最大监控条目数
	MaxIgnoreEntries int `json:"max_ignore_entries"` // 最大忽略条目数
}

// DefaultWatchListConfig 默认配置.
func DefaultWatchListConfig() WatchListConfig {
	return WatchListConfig{
		MaxWatchEntries:  1000,
		MaxIgnoreEntries: 1000,
	}
}

// WatchListManager 监控列表管理器.
type WatchListManager struct {
	config        WatchListConfig
	watchEntries  map[string]*WatchListEntry
	ignoreEntries map[string]*IgnoreListEntry
	mu            sync.RWMutex
}

// NewWatchListManager 创建监控列表管理器.
func NewWatchListManager(config WatchListConfig) *WatchListManager {
	return &WatchListManager{
		config:        config,
		watchEntries:  make(map[string]*WatchListEntry),
		ignoreEntries: make(map[string]*IgnoreListEntry),
	}
}

// ========== Watch List 操作 ==========

// AddWatchEntry 添加监控条目.
func (m *WatchListManager) AddWatchEntry(entry *WatchListEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查数量限制
	if len(m.watchEntries) >= m.config.MaxWatchEntries {
		return errors.New("监控列表已达到最大数量限制")
	}

	// 验证路径
	if entry.Path == "" {
		return errors.New("路径不能为空")
	}

	// 设置默认值
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if len(entry.Operations) == 0 {
		entry.Operations = []WatchOperation{WatchOpAll}
	}
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = entry.CreatedAt

	// 检查是否已存在相同路径
	for _, existing := range m.watchEntries {
		if existing.Path == entry.Path && existing.Pattern == entry.Pattern {
			return errors.New("相同路径和模式的监控条目已存在")
		}
	}

	m.watchEntries[entry.ID] = entry
	return nil
}

// UpdateWatchEntry 更新监控条目.
func (m *WatchListManager) UpdateWatchEntry(entry *WatchListEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		return errors.New("ID不能为空")
	}

	existing, exists := m.watchEntries[entry.ID]
	if !exists {
		return errors.New("监控条目不存在")
	}

	// 更新字段
	if entry.Path != "" {
		existing.Path = entry.Path
	}
	if entry.Pattern != "" {
		existing.Pattern = entry.Pattern
	}
	if len(entry.Operations) > 0 {
		existing.Operations = entry.Operations
	}
	existing.Recursive = entry.Recursive
	existing.Enabled = entry.Enabled
	if entry.Description != "" {
		existing.Description = entry.Description
	}
	if len(entry.Tags) > 0 {
		existing.Tags = entry.Tags
	}
	existing.UpdatedAt = time.Now()

	return nil
}

// DeleteWatchEntry 删除监控条目.
func (m *WatchListManager) DeleteWatchEntry(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watchEntries[id]; !exists {
		return errors.New("监控条目不存在")
	}

	delete(m.watchEntries, id)
	return nil
}

// GetWatchEntry 获取监控条目.
func (m *WatchListManager) GetWatchEntry(id string) (*WatchListEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.watchEntries[id]
	if !exists {
		return nil, errors.New("监控条目不存在")
	}

	return entry, nil
}

// ListWatchEntries 列出监控条目.
func (m *WatchListManager) ListWatchEntries(filter WatchListFilter) []*WatchListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*WatchListEntry, 0)
	for _, entry := range m.watchEntries {
		if !m.matchWatchFilter(entry, filter) {
			continue
		}
		result = append(result, entry)
	}

	// 按创建时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	// 应用分页
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result
}

// WatchListFilter 监控列表筛选条件.
type WatchListFilter struct {
	Path       string           `json:"path,omitempty"`
	Operations []WatchOperation `json:"operations,omitempty"`
	Enabled    *bool            `json:"enabled,omitempty"`
	CreatedBy  string           `json:"created_by,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
}

// matchWatchFilter 检查监控条目是否匹配筛选条件.
func (m *WatchListManager) matchWatchFilter(entry *WatchListEntry, filter WatchListFilter) bool {
	if filter.Path != "" && !strings.Contains(entry.Path, filter.Path) {
		return false
	}
	if filter.CreatedBy != "" && entry.CreatedBy != filter.CreatedBy {
		return false
	}
	if filter.Enabled != nil && entry.Enabled != *filter.Enabled {
		return false
	}
	if len(filter.Operations) > 0 {
		found := false
		for _, op := range filter.Operations {
			for _, entryOp := range entry.Operations {
				if op == entryOp || entryOp == WatchOpAll {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			found := false
			for _, entryTag := range entry.Tags {
				if tag == entryTag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// ========== Ignore List 操作 ==========

// AddIgnoreEntry 添加忽略条目.
func (m *WatchListManager) AddIgnoreEntry(entry *IgnoreListEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查数量限制
	if len(m.ignoreEntries) >= m.config.MaxIgnoreEntries {
		return errors.New("忽略列表已达到最大数量限制")
	}

	// 验证路径
	if entry.Path == "" {
		return errors.New("路径不能为空")
	}

	// 设置默认值
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = entry.CreatedAt

	// 检查是否已存在相同路径
	for _, existing := range m.ignoreEntries {
		if existing.Path == entry.Path && existing.Pattern == entry.Pattern {
			return errors.New("相同路径和模式的忽略条目已存在")
		}
	}

	m.ignoreEntries[entry.ID] = entry
	return nil
}

// UpdateIgnoreEntry 更新忽略条目.
func (m *WatchListManager) UpdateIgnoreEntry(entry *IgnoreListEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		return errors.New("ID不能为空")
	}

	existing, exists := m.ignoreEntries[entry.ID]
	if !exists {
		return errors.New("忽略条目不存在")
	}

	// 更新字段
	if entry.Path != "" {
		existing.Path = entry.Path
	}
	if entry.Pattern != "" {
		existing.Pattern = entry.Pattern
	}
	if entry.Reason != "" {
		existing.Reason = entry.Reason
	}
	existing.Enabled = entry.Enabled
	if entry.Description != "" {
		existing.Description = entry.Description
	}
	if entry.ExpiresAt != nil {
		existing.ExpiresAt = entry.ExpiresAt
	}
	if len(entry.Tags) > 0 {
		existing.Tags = entry.Tags
	}
	existing.UpdatedAt = time.Now()

	return nil
}

// DeleteIgnoreEntry 删除忽略条目.
func (m *WatchListManager) DeleteIgnoreEntry(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.ignoreEntries[id]; !exists {
		return errors.New("忽略条目不存在")
	}

	delete(m.ignoreEntries, id)
	return nil
}

// GetIgnoreEntry 获取忽略条目.
func (m *WatchListManager) GetIgnoreEntry(id string) (*IgnoreListEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.ignoreEntries[id]
	if !exists {
		return nil, errors.New("忽略条目不存在")
	}

	return entry, nil
}

// ListIgnoreEntries 列出忽略条目.
func (m *WatchListManager) ListIgnoreEntries(filter IgnoreListFilter) []*IgnoreListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*IgnoreListEntry, 0)
	for _, entry := range m.ignoreEntries {
		// 检查是否过期
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
			continue // 跳过已过期的条目
		}

		if !m.matchIgnoreFilter(entry, filter) {
			continue
		}
		result = append(result, entry)
	}

	// 按创建时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	// 应用分页
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result
}

// IgnoreListFilter 忽略列表筛选条件.
type IgnoreListFilter struct {
	Path      string   `json:"path,omitempty"`
	Enabled   *bool    `json:"enabled,omitempty"`
	CreatedBy string   `json:"created_by,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Expired   *bool    `json:"expired,omitempty"`
	Limit     int      `json:"limit"`
	Offset    int      `json:"offset"`
}

// matchIgnoreFilter 检查忽略条目是否匹配筛选条件.
func (m *WatchListManager) matchIgnoreFilter(entry *IgnoreListEntry, filter IgnoreListFilter) bool {
	if filter.Path != "" && !strings.Contains(entry.Path, filter.Path) {
		return false
	}
	if filter.CreatedBy != "" && entry.CreatedBy != filter.CreatedBy {
		return false
	}
	if filter.Enabled != nil && entry.Enabled != *filter.Enabled {
		return false
	}
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			found := false
			for _, entryTag := range entry.Tags {
				if tag == entryTag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// ========== 匹配检查 ==========

// ShouldWatch 检查路径是否应该被监控
// 返回匹配的监控条目，如果没有匹配则返回nil.
func (m *WatchListManager) ShouldWatch(path string, operation WatchOperation) *WatchListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先检查是否在忽略列表中
	for _, entry := range m.ignoreEntries {
		if !entry.Enabled {
			continue
		}
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
			continue
		}
		if m.matchPath(path, entry.Path, entry.Pattern) {
			return nil // 在忽略列表中，不监控
		}
	}

	// 检查监控列表
	for _, entry := range m.watchEntries {
		if !entry.Enabled {
			continue
		}
		if m.matchPath(path, entry.Path, entry.Pattern) {
			// 检查操作类型是否匹配
			for _, op := range entry.Operations {
				if op == WatchOpAll || op == operation {
					return entry
				}
			}
		}
	}

	return nil
}

// IsIgnored 检查路径是否被忽略.
func (m *WatchListManager) IsIgnored(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, entry := range m.ignoreEntries {
		if !entry.Enabled {
			continue
		}
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
			continue
		}
		if m.matchPath(path, entry.Path, entry.Pattern) {
			return true
		}
	}

	return false
}

// matchPath 检查路径是否匹配.
func (m *WatchListManager) matchPath(targetPath, basePath, pattern string) bool {
	// 清理路径
	targetPath = filepath.Clean(targetPath)
	basePath = filepath.Clean(basePath)

	// 如果没有pattern，进行简单前缀匹配或精确匹配
	if pattern == "" {
		if targetPath == basePath {
			return true
		}
		// 检查是否是子路径
		if strings.HasPrefix(targetPath, basePath+string(filepath.Separator)) {
			return true
		}
		return false
	}

	// 使用glob pattern匹配
	// 支持相对路径匹配
	relPath := strings.TrimPrefix(targetPath, basePath)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

	matched, err := filepath.Match(pattern, relPath)
	if err != nil {
		// pattern无效，尝试正则匹配
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(relPath)
	}

	return matched
}

// ========== 统计功能 ==========

// WatchListStats 监控列表统计.
type WatchListStats struct {
	TotalWatchEntries    int            `json:"total_watch_entries"`
	TotalIgnoreEntries   int            `json:"total_ignore_entries"`
	EnabledWatchEntries  int            `json:"enabled_watch_entries"`
	EnabledIgnoreEntries int            `json:"enabled_ignore_entries"`
	ExpiredIgnoreEntries int            `json:"expired_ignore_entries"`
	OperationsByType     map[string]int `json:"operations_by_type"`
}

// GetStats 获取统计信息.
func (m *WatchListManager) GetStats() *WatchListStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &WatchListStats{
		TotalWatchEntries:  len(m.watchEntries),
		TotalIgnoreEntries: len(m.ignoreEntries),
		OperationsByType:   make(map[string]int),
	}

	now := time.Now()

	for _, entry := range m.watchEntries {
		if entry.Enabled {
			stats.EnabledWatchEntries++
		}
		for _, op := range entry.Operations {
			stats.OperationsByType[string(op)]++
		}
	}

	for _, entry := range m.ignoreEntries {
		if entry.Enabled {
			if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
				stats.ExpiredIgnoreEntries++
			} else {
				stats.EnabledIgnoreEntries++
			}
		}
	}

	return stats
}

// CleanupExpired 清理过期的忽略条目.
func (m *WatchListManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for id, entry := range m.ignoreEntries {
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
			delete(m.ignoreEntries, id)
			cleaned++
		}
	}

	return cleaned
}
