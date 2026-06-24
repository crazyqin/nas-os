package smartmedialib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MediaType 媒体类型
type MediaType string

const (
	MediaTypePhoto MediaType = "photo"
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypeDoc   MediaType = "document"
)

// MediaItem 媒体项
type MediaItem struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Type      MediaType `json:"type"`
	Size      int64     `json:"size"`
	Tags      []string  `json:"tags"`
	Faces     []string  `json:"faces"`
	Albums    []string  `json:"albums"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	Duration  float64   `json:"duration,omitempty"`
	Rating    int       `json:"rating"`
	Favorite  bool      `json:"favorite"`
}

// Album 相册
type Album struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	CoverID     string      `json:"cover_id"`
	ItemCount   int         `json:"item_count"`
	Type        string      `json:"type"` // manual, auto, face, smart
	Items       []string    `json:"items"`
	SmartRules  *SmartRules `json:"smart_rules,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// SmartRules 智能相册规则
type SmartRules struct {
	Tags      []string `json:"tags,omitempty"`
	MinRating int      `json:"min_rating,omitempty"`
	DateFrom  string   `json:"date_from,omitempty"`
	DateTo    string   `json:"date_to,omitempty"`
	MediaType string   `json:"media_type,omitempty"`
	Faces     []string `json:"faces,omitempty"`
}

// Manager 媒体库管理器
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	items    map[string]*MediaItem
	albums   map[string]*Album
	dataPath string
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger, dataPath string) *Manager {
	m := &Manager{
		logger:   logger,
		items:    make(map[string]*MediaItem),
		albums:   make(map[string]*Album),
		dataPath: dataPath,
	}
	_ = m.loadData()
	return m
}

// ScanDirectory 扫描目录
func (m *Manager) ScanDirectory(ctx context.Context, dir string) (int, error) {
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		mediaType := classifyMedia(ext)
		if mediaType == "" {
			return nil
		}
		item := &MediaItem{
			ID:        generateID(path),
			Path:      path,
			Name:      info.Name(),
			Type:      mediaType,
			Size:      info.Size(),
			Tags:      extractTags(path),
			CreatedAt: info.ModTime(),
			UpdatedAt: info.ModTime(),
		}
		m.mu.Lock()
		m.items[item.ID] = item
		m.mu.Unlock()
		count++
		return nil
	})
	if err != nil {
		return count, err
	}
	_ = m.saveData()
	return count, nil
}

// GetItem 获取媒体项
func (m *Manager) GetItem(id string) (*MediaItem, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[id]
	return item, ok
}

// SearchItems 搜索媒体项
func (m *Manager) SearchItems(query string, mediaType MediaType, tags []string) []*MediaItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []*MediaItem
	q := strings.ToLower(query)
	for _, item := range m.items {
		if mediaType != "" && item.Type != mediaType {
			continue
		}
		if len(tags) > 0 && !hasAnyTag(item.Tags, tags) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(item.Name), q) {
			continue
		}
		results = append(results, item)
	}
	return results
}

// CreateAlbum 创建相册
func (m *Manager) CreateAlbum(name, desc, albumType string) *Album {
	m.mu.Lock()
	defer m.mu.Unlock()
	album := &Album{
		ID:          generateID(name),
		Name:        name,
		Description: desc,
		Type:        albumType,
		CreatedAt:   time.Now(),
	}
	m.albums[album.ID] = album
	_ = m.saveData()
	return album
}

// AddToAlbum 添加到相册
func (m *Manager) AddToAlbum(albumID, itemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	album, ok := m.albums[albumID]
	if !ok {
		return fmt.Errorf("album not found: %s", albumID)
	}
	if _, ok := m.items[itemID]; !ok {
		return fmt.Errorf("item not found: %s", itemID)
	}
	for _, id := range album.Items {
		if id == itemID {
			return nil
		}
	}
	album.Items = append(album.Items, itemID)
	album.ItemCount = len(album.Items)
	_ = m.saveData()
	return nil
}

// ToggleFavorite 切换收藏
func (m *Manager) ToggleFavorite(itemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[itemID]
	if !ok {
		return fmt.Errorf("item not found: %s", itemID)
	}
	item.Favorite = !item.Favorite
	item.UpdatedAt = time.Now()
	return nil
}

// SetRating 设置评分
func (m *Manager) SetRating(itemID string, rating int) error {
	if rating < 0 || rating > 5 {
		return fmt.Errorf("rating must be 0-5")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[itemID]
	if !ok {
		return fmt.Errorf("item not found: %s", itemID)
	}
	item.Rating = rating
	item.UpdatedAt = time.Now()
	return nil
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := map[string]interface{}{
		"total_items":   len(m.items),
		"total_albums":  len(m.albums),
		"by_type":       map[string]int{},
		"total_size":    int64(0),
		"favorite_count": 0,
	}
	byType := stats["by_type"].(map[string]int)
	for _, item := range m.items {
		byType[string(item.Type)]++
		stats["total_size"] = stats["total_size"].(int64) + item.Size
		if item.Favorite {
			stats["favorite_count"] = stats["favorite_count"].(int) + 1
		}
	}
	return stats
}

func (m *Manager) loadData() error {
	if m.dataPath == "" {
		return nil
	}
	dataFile := filepath.Join(m.dataPath, "media_library.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Items  map[string]*MediaItem `json:"items"`
		Albums map[string]*Album     `json:"albums"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	m.items = stored.Items
	m.albums = stored.Albums
	return nil
}

func (m *Manager) saveData() error {
	if m.dataPath == "" {
		return nil
	}
	_ = os.MkdirAll(m.dataPath, 0o755)
	dataFile := filepath.Join(m.dataPath, "media_library.json")
	stored := struct {
		Items  map[string]*MediaItem `json:"items"`
		Albums map[string]*Album     `json:"albums"`
	}{Items: m.items, Albums: m.albums}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0o644)
}

func classifyMedia(ext string) MediaType {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".heic", ".heif", ".raw", ".tiff":
		return MediaTypePhoto
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v":
		return MediaTypeVideo
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".wma", ".m4a":
		return MediaTypeAudio
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md":
		return MediaTypeDoc
	}
	return ""
}

func extractTags(path string) []string {
	var tags []string
	dir := filepath.Dir(path)
	parent := filepath.Base(dir)
	if parent != "." && parent != "/" {
		tags = append(tags, strings.ToLower(parent))
	}
	return tags
}

func hasAnyTag(itemTags, queryTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range itemTags {
		tagSet[strings.ToLower(t)] = true
	}
	for _, t := range queryTags {
		if tagSet[strings.ToLower(t)] {
			return true
		}
	}
	return false
}

func generateID(s string) string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// Handlers API handlers
type Handlers struct {
	mgr *Manager
}

func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	media := rg.Group("/media")
	{
		media.POST("/scan", h.scanDirectory)
		media.GET("/search", h.searchItems)
		media.GET("/items/:id", h.getItem)
		media.PUT("/items/:id/favorite", h.toggleFavorite)
		media.PUT("/items/:id/rating", h.setRating)
		media.GET("/stats", h.getStats)
		media.POST("/albums", h.createAlbum)
		media.POST("/albums/:id/items", h.addToAlbum)
		media.GET("/albums/:id", h.getAlbum)
	}
}

func (h *Handlers) scanDirectory(c *gin.Context) {
	var req struct {
		Directory string `json:"directory" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	count, err := h.mgr.ScanDirectory(c.Request.Context(), req.Directory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"scanned": count})
}

func (h *Handlers) searchItems(c *gin.Context) {
	query := c.Query("q")
	mediaType := MediaType(c.Query("type"))
	tags := c.QueryArray("tags")
	items := h.mgr.SearchItems(query, mediaType, tags)
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (h *Handlers) getItem(c *gin.Context) {
	id := c.Param("id")
	item, ok := h.mgr.GetItem(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handlers) toggleFavorite(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.ToggleFavorite(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) setRating(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Rating int `json:"rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.mgr.SetRating(id, req.Rating); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) getStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetStats())
}

func (h *Handlers) createAlbum(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Type == "" {
		req.Type = "manual"
	}
	album := h.mgr.CreateAlbum(req.Name, req.Description, req.Type)
	c.JSON(http.StatusCreated, album)
}

func (h *Handlers) addToAlbum(c *gin.Context) {
	albumID := c.Param("id")
	var req struct {
		ItemID string `json:"item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.mgr.AddToAlbum(albumID, req.ItemID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) getAlbum(c *gin.Context) {
	albumID := c.Param("id")
	h.mgr.mu.RLock()
	defer h.mgr.mu.RUnlock()
	album, ok := h.mgr.albums[albumID]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "album not found"})
		return
	}
	c.JSON(http.StatusOK, album)
}
