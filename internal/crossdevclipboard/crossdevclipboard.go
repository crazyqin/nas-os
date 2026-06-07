// Package crossdevclipboard 实现跨设备剪贴板同步功能。
// 支持文本、图片、文件路径的跨设备复制粘贴，端到端加密传输。
package crossdevclipboard

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ClipType 剪贴板内容类型
type ClipType string

const (
	ClipText     ClipType = "text"     // 纯文本
	ClipRichText ClipType = "richtext" // 富文本
	ClipImage    ClipType = "image"    // 图片（base64）
	ClipFilePath ClipType = "filepath" // 文件路径
	ClipURL      ClipType = "url"      // URL链接
)

// ClipItem 剪贴板条目
type ClipItem struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"deviceId"`
	DeviceName string    `json:"deviceName"`
	Type       ClipType  `json:"type"`
	Content    string    `json:"content"`
	Size       int64     `json:"size"`
	Hash       string    `json:"hash"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt,omitempty"`
}

// Device 注册设备信息
type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"` // ios/android/windows/macos/linux/web
	UserAgent string    `json:"userAgent"`
	LastSeen  time.Time `json:"lastSeen"`
	PairedAt  time.Time `json:"pairedAt"`
	PublicKey string    `json:"publicKey,omitempty"`
	Enabled   bool      `json:"enabled"`
}

// SyncConfig 同步配置
type SyncConfig struct {
	MaxContentSize int64         `json:"maxContentSize"` // 最大内容大小（字节）
	MaxHistory     int           `json:"maxHistory"`     // 最大历史条数
	ExpiryDuration time.Duration `json:"expiryDuration"` // 内容过期时间
	EncryptionKey  string        `json:"encryptionKey"`  // 端到端加密密钥
	AutoSync       bool          `json:"autoSync"`       // 自动同步
	AllowedDevices []string      `json:"allowedDevices"` // 允许的设备列表
}

// ClipboardManager 剪贴板管理器
type ClipboardManager struct {
	mu         sync.RWMutex
	items      []ClipItem
	devices    map[string]*Device
	config     SyncConfig
	maxItems   int
	encryptKey []byte
}

// NewClipboardManager 创建剪贴板管理器
func NewClipboardManager(config SyncConfig) *ClipboardManager {
	var key []byte
	if config.EncryptionKey != "" {
		hash := sha256.Sum256([]byte(config.EncryptionKey))
		key = hash[:]
	}
	if config.MaxHistory == 0 {
		config.MaxHistory = 100
	}
	if config.MaxContentSize == 0 {
		config.MaxContentSize = 10 * 1024 * 1024 // 10MB
	}
	if config.ExpiryDuration == 0 {
		config.ExpiryDuration = 24 * time.Hour
	}
	return &ClipboardManager{
		items:      make([]ClipItem, 0),
		devices:    make(map[string]*Device),
		config:     config,
		maxItems:   config.MaxHistory,
		encryptKey: key,
	}
}

// RegisterDevice 注册新设备
func (m *ClipboardManager) RegisterDevice(device Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device.PairedAt = time.Now()
	device.LastSeen = time.Now()
	device.Enabled = true
	m.devices[device.ID] = &device
	return nil
}

// RemoveDevice 移除设备
func (m *ClipboardManager) RemoveDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[deviceID]; !ok {
		return fmt.Errorf("设备 %s 未注册", deviceID)
	}
	delete(m.devices, deviceID)
	return nil
}

// PushContent 推送剪贴板内容
func (m *ClipboardManager) PushContent(deviceID string, clipType ClipType, content string) (*ClipItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证设备
	device, ok := m.devices[deviceID]
	if !ok || !device.Enabled {
		return nil, fmt.Errorf("设备 %s 未注册或已禁用", deviceID)
	}

	// 检查内容大小
	if int64(len(content)) > m.config.MaxContentSize {
		return nil, fmt.Errorf("内容大小 %d 超过限制 %d", len(content), m.config.MaxContentSize)
	}

	// 加密内容
	encrypted := content
	if m.encryptKey != nil {
		enc, err := m.encrypt([]byte(content))
		if err != nil {
			return nil, fmt.Errorf("加密失败: %w", err)
		}
		encrypted = enc
	}

	item := ClipItem{
		ID:         generateID(),
		DeviceID:   deviceID,
		DeviceName: device.Name,
		Type:       clipType,
		Content:    encrypted,
		Size:       int64(len(content)),
		Hash:       computeHash(content),
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(m.config.ExpiryDuration),
	}

	// 添加到历史
	m.items = append([]ClipItem{item}, m.items...)
	if len(m.items) > m.maxItems {
		m.items = m.items[:m.maxItems]
	}

	// 更新设备最后在线时间
	device.LastSeen = time.Now()

	return &item, nil
}

// PullLatest 拉取最新剪贴板内容
func (m *ClipboardManager) PullLatest(deviceID string) (*ClipItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.devices[deviceID]; !ok {
		return nil, fmt.Errorf("设备 %s 未注册", deviceID)
	}

	if len(m.items) == 0 {
		return nil, fmt.Errorf("剪贴板为空")
	}

	item := m.items[0]
	// 不返回自己推送的内容
	if item.DeviceID == deviceID && len(m.items) > 1 {
		item = m.items[1]
	}

	// 解密内容
	if m.encryptKey != nil {
		dec, err := m.decrypt(item.Content)
		if err != nil {
			return nil, fmt.Errorf("解密失败: %w", err)
		}
		result := item
		result.Content = dec
		return &result, nil
	}

	return &item, nil
}

// GetHistory 获取剪贴板历史
func (m *ClipboardManager) GetHistory(deviceID string, limit int) ([]ClipItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.devices[deviceID]; !ok {
		return nil, fmt.Errorf("设备 %s 未注册", deviceID)
	}

	if limit <= 0 || limit > len(m.items) {
		limit = len(m.items)
	}

	result := make([]ClipItem, limit)
	copy(result, m.items[:limit])

	// 解密
	if m.encryptKey != nil {
		for i := range result {
			dec, err := m.decrypt(result[i].Content)
			if err == nil {
				result[i].Content = dec
			}
		}
	}

	return result, nil
}

// ListDevices 列出所有注册设备
func (m *ClipboardManager) ListDevices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

// Cleanup 过期内容清理
func (m *ClipboardManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	valid := make([]ClipItem, 0, len(m.items))
	removed := 0

	for _, item := range m.items {
		if item.ExpiresAt.IsZero() || item.ExpiresAt.After(now) {
			valid = append(valid, item)
		} else {
			removed++
		}
	}
	m.items = valid
	return removed
}

// Stats 剪贴板统计
type Stats struct {
	TotalItems   int       `json:"totalItems"`
	TotalDevices int       `json:"totalDevices"`
	TotalSize    int64     `json:"totalSize"`
	OldestItem   time.Time `json:"oldestItem,omitempty"`
	NewestItem   time.Time `json:"newestItem,omitempty"`
}

// GetStats 获取统计信息
func (m *ClipboardManager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{
		TotalItems:   len(m.items),
		TotalDevices: len(m.devices),
	}

	for _, item := range m.items {
		stats.TotalSize += item.Size
		if stats.OldestItem.IsZero() || item.CreatedAt.Before(stats.OldestItem) {
			stats.OldestItem = item.CreatedAt
		}
		if stats.NewestItem.IsZero() || item.CreatedAt.After(stats.NewestItem) {
			stats.NewestItem = item.CreatedAt
		}
	}

	return stats
}

// encrypt AES-GCM 加密
func (m *ClipboardManager) encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(m.encryptKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt AES-GCM 解密
func (m *ClipboardManager) decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(m.encryptKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("密文太短")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// RegisterRoutes 注册 HTTP 路由
func (m *ClipboardManager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/clipboard/push", m.handlePush)
	mux.HandleFunc("/api/v1/clipboard/pull", m.handlePull)
	mux.HandleFunc("/api/v1/clipboard/history", m.handleHistory)
	mux.HandleFunc("/api/v1/clipboard/devices", m.handleDevices)
	mux.HandleFunc("/api/v1/clipboard/stats", m.handleStats)
}

func (m *ClipboardManager) handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DeviceID string   `json:"deviceId"`
		Type     ClipType `json:"type"`
		Content  string   `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	item, err := m.PushContent(req.DeviceID, req.Type, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(item)
}

func (m *ClipboardManager) handlePull(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("deviceId")
	if deviceID == "" {
		http.Error(w, "deviceId required", http.StatusBadRequest)
		return
	}
	item, err := m.PullLatest(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(item)
}

func (m *ClipboardManager) handleHistory(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("deviceId")
	limit := 20
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	items, err := m.GetHistory(deviceID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(items)
}

func (m *ClipboardManager) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices := m.ListDevices()
	json.NewEncoder(w).Encode(devices)
}

func (m *ClipboardManager) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := m.GetStats()
	json.NewEncoder(w).Encode(stats)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func computeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}
