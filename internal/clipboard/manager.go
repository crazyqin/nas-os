// Package clipboard 提供跨设备剪贴板同步功能
package clipboard

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 剪贴板管理器.
type Manager struct {
	mu       sync.RWMutex
	items    map[string]*ClipItem
	users    map[string]map[string]*ClipItem // userID -> deviceID -> last clip
	key      []byte
	maxItems int
}

// NewManager 创建剪贴板管理器.
func NewManager(encryptionKey string, maxItems int) *Manager {
	key := make([]byte, 32)
	copy(key, []byte(encryptionKey))
	return &Manager{
		items:    make(map[string]*ClipItem),
		users:    make(map[string]map[string]*ClipItem),
		key:      key,
		maxItems: maxItems,
	}
}

// Create 创建剪贴板条目.
func (m *Manager) Create(req CreateClipRequest, userID string) (*ClipItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := &ClipItem{
		ID:        generateID(),
		Content:   req.Content,
		Type:      req.Type,
		Source:    req.Source,
		UserID:    userID,
		CreatedAt: time.Now(),
	}

	if req.TTL > 0 {
		item.ExpiresAt = time.Now().Add(time.Duration(req.TTL) * time.Second)
	}

	if item.Type == "" {
		item.Type = detectType(req.Content)
	}

	// 加密存储敏感内容
	if item.Type == ClipTypeText && len(req.Content) > 0 {
		encrypted, err := m.encrypt(req.Content)
		if err == nil {
			item.Content = encrypted
		}
	}

	m.items[item.ID] = item

	// 更新用户设备映射
	if _, ok := m.users[userID]; !ok {
		m.users[userID] = make(map[string]*ClipItem)
	}
	m.users[userID][req.Source] = item

	// 清理过期和超限条目
	m.cleanup()

	return item, nil
}

// Get 获取剪贴板条目.
func (m *Manager) Get(id string) (*ClipItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.items[id]
	if !ok {
		return nil, fmt.Errorf("clip item not found: %s", id)
	}

	// 检查是否过期
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		return nil, fmt.Errorf("clip item expired: %s", id)
	}

	return item, nil
}

// GetLatest 获取用户设备的最新剪贴板.
func (m *Manager) GetLatest(userID, deviceID string) (*ClipItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices, ok := m.users[userID]
	if !ok {
		return nil, fmt.Errorf("no clips for user: %s", userID)
	}

	item, ok := devices[deviceID]
	if !ok {
		// 返回任意设备的最新条目
		for _, v := range devices {
			if item == nil || v.CreatedAt.After(item.CreatedAt) {
				item = v
			}
		}
	}

	if item == nil {
		return nil, fmt.Errorf("no clips found")
	}

	return item, nil
}

// Sync 同步剪贴板到指定设备.
func (m *Manager) Sync(userID, deviceID string, lastSync time.Time) ([]ClipItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ClipItem
	devices, ok := m.users[userID]
	if !ok {
		return result, nil
	}

	seen := make(map[string]bool)
	for _, item := range devices {
		if item.CreatedAt.After(lastSync) && !seen[item.ID] {
			result = append(result, *item)
			seen[item.ID] = true
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

// Search 搜索剪贴板.
func (m *Manager) Search(req SearchRequest) ([]ClipItem, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []ClipItem
	query := strings.ToLower(req.Query)

	for _, item := range m.items {
		if req.UserID != "" && item.UserID != req.UserID {
			continue
		}
		if req.Type != "" && item.Type != req.Type {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Content), query) {
			continue
		}
		results = append(results, *item)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	total := len(results)

	// 分页
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	start := req.Page * req.PageSize
	if start >= len(results) {
		return nil, total, nil
	}
	end := start + req.PageSize
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total, nil
}

// Delete 删除剪贴板条目.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.items[id]; !ok {
		return fmt.Errorf("clip item not found: %s", id)
	}

	delete(m.items, id)

	// 清理用户映射
	for _, devices := range m.users {
		for deviceID, item := range devices {
			if item.ID == id {
				delete(devices, deviceID)
			}
		}
	}

	return nil
}

// ClearUser 清空用户所有剪贴板.
func (m *Manager) ClearUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 删除该用户的所有条目
	for id, item := range m.items {
		if item.UserID == userID {
			delete(m.items, id)
		}
	}

	// 清除用户设备映射
	delete(m.users, userID)

	return nil
}

// Stats 获取剪贴板统计.
func (m *Manager) Stats() ClipboardStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ClipboardStats{
		TotalItems: int64(len(m.items)),
	}

	deviceSet := make(map[string]bool)
	var oldest, newest time.Time

	for _, item := range m.items {
		stats.TotalSize += int64(len(item.Content))
		deviceSet[item.Source] = true

		if oldest.IsZero() || item.CreatedAt.Before(oldest) {
			oldest = item.CreatedAt
		}
		if newest.IsZero() || item.CreatedAt.After(newest) {
			newest = item.CreatedAt
		}
	}

	stats.DeviceCount = len(deviceSet)
	stats.ActiveUsers = len(m.users)
	stats.OldestItem = oldest
	stats.NewestItem = newest

	return stats
}

// cleanup 清理过期和超限条目.
func (m *Manager) cleanup() {
	now := time.Now()

	// 删除过期条目
	for id, item := range m.items {
		if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
			delete(m.items, id)
		}
	}

	// 超限时删除最旧的
	if len(m.items) > m.maxItems {
		items := make([]*ClipItem, 0, len(m.items))
		for _, item := range m.items {
			items = append(items, item)
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		})
		for i := 0; i < len(items)-m.maxItems; i++ {
			delete(m.items, items[i].ID)
		}
	}
}

// encrypt AES加密.
func (m *Manager) Encrypt(plaintext string) (string, error) {
	return m.encrypt(plaintext)
}

func (m *Manager) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt AES解密.
func (m *Manager) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func detectType(content string) ClipType {
	if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
		return ClipTypeLink
	}
	return ClipTypeText
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
