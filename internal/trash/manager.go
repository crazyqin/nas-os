package trash

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrashItem 回收站项目
type TrashItem struct {
	ID           string    `json:"id"`
	OriginalPath string    `json:"original_path"`
	TrashPath    string    `json:"trash_path"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	IsDir        bool      `json:"is_dir"`
	DeletedAt    time.Time `json:"deleted_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	DeletedBy    string    `json:"deleted_by"`
}

// Config 回收站配置
type Config struct {
	Enabled        bool          `json:"enabled"`
	RetentionDays int           `json:"retention_days"`
	MaxSize        int64         `json:"max_size"` // 最大占用空间 (字节)
	AutoEmpty      bool          `json:"auto_empty"`
	EmptySchedule  string        `json:"empty_schedule"` // cron 表达式
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:        true,
		RetentionDays:  30,
		MaxSize:        10 * 1024 * 1024 * 1024, // 10GB
		AutoEmpty:      true,
		EmptySchedule:  "0 3 * * *", // 每天凌晨 3 点
	}
}

// Manager 回收站管理器
type Manager struct {
	mu           sync.RWMutex
	config       *Config
	items        map[string]*TrashItem // id -> item
	configPath   string
	trashRoot    string
	totalSize    int64
	onSizeChange func(int64) // 空间变化回调
}

// NewManager 创建回收站管理器
func NewManager(configPath, trashRoot string, config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		config:     config,
		items:      make(map[string]*TrashItem),
		configPath: configPath,
		trashRoot:  trashRoot,
	}

	// 创建回收站根目录
	if err := os.MkdirAll(trashRoot, 0755); err != nil {
		return nil, fmt.Errorf("创建回收站目录失败：%w", err)
	}

	// 加载配置
	if err := m.loadConfig(); err != nil {
		// 配置不存在时保存默认配置
		if os.IsNotExist(err) {
			if err := m.saveConfig(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// 加载回收站项目
	if err := m.loadItems(); err != nil {
		return nil, err
	}

	// 启动自动清理
	if config.AutoEmpty {
		go m.startAutoClean()
	}

	return m, nil
}

// MoveToTrash 移动到回收站
func (m *Manager) MoveToTrash(originalPath, userID string) (*TrashItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		// 回收站禁用，直接删除
		return nil, os.RemoveAll(originalPath)
	}

	// 检查文件是否存在
	info, err := os.Stat(originalPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在：%w", err)
	}

	// 生成回收站 ID 和路径
	id := generateTrashID()
	trashPath := filepath.Join(m.trashRoot, id)

	// 移动文件到回收站
	if err := os.Rename(originalPath, trashPath); err != nil {
		return nil, fmt.Errorf("移动文件失败：%w", err)
	}

	// 创建回收站项目
	item := &TrashItem{
		ID:           id,
		OriginalPath: originalPath,
		TrashPath:    trashPath,
		Name:         filepath.Base(originalPath),
		Size:         info.Size(),
		IsDir:        info.IsDir(),
		DeletedAt:    time.Now(),
		ExpiresAt:    time.Now().AddDate(0, 0, m.config.RetentionDays),
		DeletedBy:    userID,
	}

	m.items[id] = item
	m.totalSize += item.Size

	// 保存项目列表
	if err := m.saveItems(); err != nil {
		// 回滚
		os.Rename(trashPath, originalPath)
		delete(m.items, id)
		m.totalSize -= item.Size
		return nil, err
	}

	// 检查空间限制
	if m.totalSize > m.config.MaxSize {
		// 自动清理最早的项目
		if err := m.cleanupOldest(); err != nil {
			// 记录错误但不影响当前操作
		}
	}

	// 触发空间变化回调
	if m.onSizeChange != nil {
		m.onSizeChange(m.totalSize)
	}

	return item, nil
}

// Restore 恢复文件
func (m *Manager) Restore(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[id]
	if !exists {
		return fmt.Errorf("回收站项目不存在：%s", id)
	}

	// 确保原目录存在
	originalDir := filepath.Dir(item.OriginalPath)
	if err := os.MkdirAll(originalDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败：%w", err)
	}

	// 移动回原位置
	if err := os.Rename(item.TrashPath, item.OriginalPath); err != nil {
		return fmt.Errorf("恢复文件失败：%w", err)
	}

	// 更新状态
	m.totalSize -= item.Size
	delete(m.items, id)

	// 保存项目列表
	return m.saveItems()
}

// DeletePermanently 永久删除
func (m *Manager) DeletePermanently(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[id]
	if !exists {
		return fmt.Errorf("回收站项目不存在：%s", id)
	}

	// 删除文件
	if err := os.RemoveAll(item.TrashPath); err != nil {
		return fmt.Errorf("删除文件失败：%w", err)
	}

	// 更新状态
	m.totalSize -= item.Size
	delete(m.items, id)

	// 保存项目列表
	return m.saveItems()
}

// Empty 清空回收站
func (m *Manager) Empty() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, item := range m.items {
		if err := os.RemoveAll(item.TrashPath); err != nil {
			// 记录错误但继续删除其他文件
			continue
		}
		delete(m.items, id)
	}

	m.totalSize = 0
	return m.saveItems()
}

// List 列出回收站项目
func (m *Manager) List() []*TrashItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*TrashItem, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, item)
	}

	// 按删除时间倒序排序
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	return items
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_items":  len(m.items),
		"total_size":   m.totalSize,
		"max_size":     m.config.MaxSize,
		"usage_percent": float64(m.totalSize) / float64(m.config.MaxSize) * 100,
		"retention_days": m.config.RetentionDays,
		"enabled":      m.config.Enabled,
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return m.saveConfig()
}

// loadConfig 加载配置
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, m.config)
}

// saveConfig 保存配置
func (m *Manager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// loadItems 加载项目列表
func (m *Manager) loadItems() error {
	itemsPath := filepath.Join(m.trashRoot, "items.json")
	data, err := os.ReadFile(itemsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var items []*TrashItem
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// 验证文件是否存在并计算总大小
	m.totalSize = 0
	for _, item := range items {
		if _, err := os.Stat(item.TrashPath); err == nil {
			m.items[item.ID] = item
			m.totalSize += item.Size
		}
	}

	return nil
}

// saveItems 保存项目列表
func (m *Manager) saveItems() error {
	itemsPath := filepath.Join(m.trashRoot, "items.json")

	items := make([]*TrashItem, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, item)
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(itemsPath, data, 0644)
}

// startAutoClean 启动自动清理
func (m *Manager) startAutoClean() {
	// 每小时检查一次
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpired()
	}
}

// cleanupExpired 清理过期项目
func (m *Manager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var toDelete []string

	for id, item := range m.items {
		if now.After(item.ExpiresAt) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		item := m.items[id]
		os.RemoveAll(item.TrashPath)
		m.totalSize -= item.Size
		delete(m.items, id)
	}

	if len(toDelete) > 0 {
		m.saveItems()
	}
}

// cleanupOldest 清理最早的项目以释放空间
func (m *Manager) cleanupOldest() error {
	// 按删除时间排序
	items := make([]*TrashItem, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, item)
	}

	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		if items[i].DeletedAt.After(items[j].DeletedAt) {
			items[i], items[j] = items[j], items[i]
		}
	}

	// 删除最早的项目直到空间足够
	for _, item := range items {
		if m.totalSize <= m.config.MaxSize {
			break
		}

		os.RemoveAll(item.TrashPath)
		m.totalSize -= item.Size
		delete(m.items, item.ID)
	}

	return m.saveItems()
}

// generateTrashID 生成回收站 ID
func generateTrashID() string {
	return fmt.Sprintf("trash-%d", time.Now().UnixNano())
}

// SetSizeChangeCallback 设置空间变化回调
func (m *Manager) SetSizeChangeCallback(fn func(int64)) {
	m.onSizeChange = fn
}
