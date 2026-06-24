package aifileorganizer

import (
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

// Category 文件分类
type Category string

const (
	CategoryDocument Category = "document"
	CategoryImage    Category = "image"
	CategoryVideo    Category = "video"
	CategoryAudio    Category = "audio"
	CategoryArchive  Category = "archive"
	CategoryCode     Category = "code"
	CategoryOther    Category = "other"
)

// OrganizeRule 整理规则
type OrganizeRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	SourcePath  string   `json:"source_path"`
	TargetPath  string   `json:"target_path"`
	Categories  []string `json:"categories"`
	DatePattern string   `json:"date_pattern"`
	AutoMove    bool     `json:"auto_move"`
	CreatedAt   time.Time `json:"created_at"`
}

// FileRecord 文件记录
type FileRecord struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Category   Category  `json:"category"`
	Extension  string    `json:"extension"`
	Size       int64     `json:"size"`
	Organized  bool      `json:"organized"`
	TargetPath string    `json:"target_path,omitempty"`
	Score      float64   `json:"score"`
	CreatedAt  time.Time `json:"created_at"`
}

// Manager 文件整理管理器
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	rules    map[string]*OrganizeRule
	records  map[string]*FileRecord
	dataPath string
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger, dataPath string) *Manager {
	m := &Manager{
		logger:   logger,
		rules:    make(map[string]*OrganizeRule),
		records:  make(map[string]*FileRecord),
		dataPath: dataPath,
	}
	_ = m.loadData()
	return m
}

// CreateRule 创建规则
func (m *Manager) CreateRule(name, source, target string, categories []string, autoMove bool) *OrganizeRule {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule := &OrganizeRule{
		ID:         genID(),
		Name:       name,
		Enabled:    true,
		SourcePath: source,
		TargetPath: target,
		Categories: categories,
		AutoMove:   autoMove,
		CreatedAt:  time.Now(),
	}
	m.rules[rule.ID] = rule
	_ = m.saveData()
	return rule
}

// ListRules 列出规则
func (m *Manager) ListRules() []*OrganizeRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*OrganizeRule
	for _, r := range m.rules {
		result = append(result, r)
	}
	return result
}

// DeleteRule 删除规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule not found")
	}
	delete(m.rules, id)
	_ = m.saveData()
	return nil
}

// ScanAndClassify 扫描并分类文件
func (m *Manager) ScanAndClassify(dir string) (int, map[Category]int, error) {
	count := 0
	stats := make(map[Category]int)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		cat := classifyFile(ext)
		stats[cat]++
		record := &FileRecord{
			Path:      path,
			Name:      info.Name(),
			Category:  cat,
			Extension: ext,
			Size:      info.Size(),
			Score:     scoreFile(info),
			CreatedAt: info.ModTime(),
		}
		m.mu.Lock()
		m.records[path] = record
		m.mu.Unlock()
		count++
		return nil
	})
	return count, stats, err
}

// GetSuggestions 获取整理建议
func (m *Manager) GetSuggestions(dir string) []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var suggestions []map[string]interface{}
	for _, r := range m.records {
		if r.Organized || r.Category == CategoryOther {
			continue
		}
		target := filepath.Join(dir, string(r.Category), r.Name)
		suggestions = append(suggestions, map[string]interface{}{
			"path":       r.Path,
			"category":   r.Category,
			"suggestion": target,
			"score":      r.Score,
		})
	}
	return suggestions
}

// ApplyRule 应用规则
func (m *Manager) ApplyRule(ruleID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule, ok := m.rules[ruleID]
	if !ok {
		return 0, fmt.Errorf("rule not found")
	}
	count := 0
	for _, record := range m.records {
		if record.Organized {
			continue
		}
		if !matchesCategories(record.Category, rule.Categories) {
			continue
		}
		if !strings.HasPrefix(record.Path, rule.SourcePath) {
			continue
		}
		targetDir := rule.TargetPath
		if rule.DatePattern != "" {
			targetDir = filepath.Join(targetDir, record.CreatedAt.Format(rule.DatePattern))
		}
		record.TargetPath = filepath.Join(targetDir, record.Name)
		record.Organized = true
		if rule.AutoMove {
			_ = os.MkdirAll(targetDir, 0o755)
			_ = os.Rename(record.Path, record.TargetPath)
		}
		count++
	}
	_ = m.saveData()
	return count, nil
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := map[string]interface{}{
		"total_files":  len(m.records),
		"total_rules":  len(m.rules),
		"organized":    0,
		"by_category":  map[string]int{},
	}
	byCat := stats["by_category"].(map[string]int)
	for _, r := range m.records {
		byCat[string(r.Category)]++
		if r.Organized {
			stats["organized"] = stats["organized"].(int) + 1
		}
	}
	return stats
}

func classifyFile(ext string) Category {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".heic", ".svg":
		return CategoryImage
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm":
		return CategoryVideo
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a":
		return CategoryAudio
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return CategoryArchive
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php", ".html", ".css", ".sh":
		return CategoryCode
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md", ".csv":
		return CategoryDocument
	default:
		return CategoryOther
	}
}

func matchesCategories(cat Category, categories []string) bool {
	if len(categories) == 0 {
		return true
	}
	for _, c := range categories {
		if string(cat) == c {
			return true
		}
	}
	return false
}

func scoreFile(info os.FileInfo) float64 {
	score := 0.5
	if info.Size() > 10*1024*1024 {
		score += 0.2
	}
	if time.Since(info.ModTime()) > 30*24*time.Hour {
		score += 0.1
	}
	return score
}

func genID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func (m *Manager) loadData() error {
	if m.dataPath == "" {
		return nil
	}
	dataFile := filepath.Join(m.dataPath, "file_organizer.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Rules   map[string]*OrganizeRule `json:"rules"`
		Records map[string]*FileRecord   `json:"records"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Rules != nil {
		m.rules = stored.Rules
	}
	if stored.Records != nil {
		m.records = stored.Records
	}
	return nil
}

func (m *Manager) saveData() error {
	if m.dataPath == "" {
		return nil
	}
	_ = os.MkdirAll(m.dataPath, 0o755)
	stored := struct {
		Rules   map[string]*OrganizeRule `json:"rules"`
		Records map[string]*FileRecord   `json:"records"`
	}{Rules: m.rules, Records: m.records}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dataPath, "file_organizer.json"), data, 0o644)
}

// Handlers API
type Handlers struct{ mgr *Manager }

func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/organizer")
	{
		g.POST("/scan", h.scan)
		g.GET("/suggestions", h.suggestions)
		g.GET("/stats", h.stats)
		g.POST("/rules", h.createRule)
		g.GET("/rules", h.listRules)
		g.DELETE("/rules/:id", h.deleteRule)
		g.POST("/rules/:id/apply", h.applyRule)
	}
}

func (h *Handlers) scan(c *gin.Context) {
	var req struct {
		Directory string `json:"directory" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	count, stats, err := h.mgr.ScanAndClassify(req.Directory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"scanned": count, "stats": stats})
}

func (h *Handlers) suggestions(c *gin.Context) {
	dir := c.Query("directory")
	c.JSON(http.StatusOK, gin.H{"suggestions": h.mgr.GetSuggestions(dir)})
}

func (h *Handlers) stats(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetStats())
}

func (h *Handlers) createRule(c *gin.Context) {
	var req struct {
		Name       string   `json:"name" binding:"required"`
		SourcePath string   `json:"source_path" binding:"required"`
		TargetPath string   `json:"target_path" binding:"required"`
		Categories []string `json:"categories"`
		AutoMove   bool     `json:"auto_move"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule := h.mgr.CreateRule(req.Name, req.SourcePath, req.TargetPath, req.Categories, req.AutoMove)
	c.JSON(http.StatusCreated, rule)
}

func (h *Handlers) listRules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"rules": h.mgr.ListRules()})
}

func (h *Handlers) deleteRule(c *gin.Context) {
	if err := h.mgr.DeleteRule(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) applyRule(c *gin.Context) {
	count, err := h.mgr.ApplyRule(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"applied": count})
}
