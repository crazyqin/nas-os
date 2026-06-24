package storagereporter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Snapshot 存储快照
type Snapshot struct {
	Timestamp  time.Time        `json:"timestamp"`
	TotalBytes int64            `json:"total_bytes"`
	UsedBytes  int64            `json:"used_bytes"`
	FreeBytes  int64            `json:"free_bytes"`
	ByCategory map[string]int64 `json:"by_category"`
}

// TrendReport 趋势报告
type TrendReport struct {
	Period        string  `json:"period"`
	AvgUsage      float64 `json:"avg_usage"`
	MaxUsage      float64 `json:"max_usage"`
	MinUsage      float64 `json:"min_usage"`
	GrowthRate    float64 `json:"growth_rate"`
	DaysUntilFull int     `json:"days_until_full"`
	Prediction    string  `json:"prediction"`
}

// Manager 存储报告管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	snapshots []*Snapshot
	dataPath  string
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger, dataPath string) *Manager {
	m := &Manager{
		logger:   logger,
		dataPath: dataPath,
	}
	_ = m.loadData()
	return m
}

// TakeSnapshot 获取快照
func (m *Manager) TakeSnapshot(total, used int64, categories map[string]int64) *Snapshot {
	snap := &Snapshot{
		Timestamp:  time.Now(),
		TotalBytes: total,
		UsedBytes:  used,
		FreeBytes:  total - used,
		ByCategory: categories,
	}
	if snap.ByCategory == nil {
		snap.ByCategory = make(map[string]int64)
	}
	m.mu.Lock()
	m.snapshots = append(m.snapshots, snap)
	if len(m.snapshots) > 90 {
		m.snapshots = m.snapshots[len(m.snapshots)-90:]
	}
	m.mu.Unlock()
	_ = m.saveData()
	return snap
}

// GetTrend 获取趋势
func (m *Manager) GetTrend(days int) *TrendReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	report := &TrendReport{Period: fmt.Sprintf("%d days", days)}
	if len(m.snapshots) < 2 {
		report.Prediction = "数据不足"
		return report
	}
	var usages []float64
	for _, s := range m.snapshots {
		if s.TotalBytes > 0 {
			usages = append(usages, float64(s.UsedBytes)/float64(s.TotalBytes)*100)
		}
	}
	if len(usages) == 0 {
		return report
	}
	report.AvgUsage = avg(usages)
	report.MaxUsage = maxVal(usages)
	report.MinUsage = minVal(usages)
	if len(usages) >= 2 {
		growth := usages[len(usages)-1] - usages[0]
		report.GrowthRate = growth / float64(len(usages))
		if report.GrowthRate > 0 && usages[len(usages)-1] < 100 {
			remaining := 100 - usages[len(usages)-1]
			report.DaysUntilFull = int(remaining / report.GrowthRate)
			if report.DaysUntilFull < 30 {
				report.Prediction = "⚠️ 预计30天内磁盘将满"
			} else if report.DaysUntilFull < 90 {
				report.Prediction = "📊 预计90天内需要扩容"
			} else {
				report.Prediction = "✅ 存储空间充足"
			}
		}
	}
	return report
}

// GetLatest 获取最新快照
func (m *Manager) GetLatest() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.snapshots) == 0 {
		return nil
	}
	return m.snapshots[len(m.snapshots)-1]
}

// GetHistory 获取历史
func (m *Manager) GetHistory(limit int) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.snapshots) {
		limit = len(m.snapshots)
	}
	start := len(m.snapshots) - limit
	result := make([]*Snapshot, limit)
	copy(result, m.snapshots[start:])
	return result
}

// GenerateReport 生成报告
func (m *Manager) GenerateReport() map[string]interface{} {
	return map[string]interface{}{
		"current":   m.GetLatest(),
		"trend_7d":  m.GetTrend(7),
		"trend_30d": m.GetTrend(30),
		"generated": time.Now(),
	}
}

func avg(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func maxVal(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func minVal(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func (m *Manager) loadData() error {
	if m.dataPath == "" {
		return nil
	}
	dataFile := filepath.Join(m.dataPath, "storage_reporter.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Snapshots []*Snapshot `json:"snapshots"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	m.snapshots = stored.Snapshots
	return nil
}

func (m *Manager) saveData() error {
	if m.dataPath == "" {
		return nil
	}
	_ = os.MkdirAll(m.dataPath, 0o755)
	stored := struct {
		Snapshots []*Snapshot `json:"snapshots"`
	}{Snapshots: m.snapshots}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dataPath, "storage_reporter.json"), data, 0o644)
}

// Handlers API
type Handlers struct{ mgr *Manager }

func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/storage-report")
	{
		g.POST("/snapshot", h.snapshot)
		g.GET("/trend", h.trend)
		g.GET("/history", h.history)
		g.GET("/report", h.report)
	}
}

func (h *Handlers) snapshot(c *gin.Context) {
	var req struct {
		TotalBytes int64            `json:"total_bytes"`
		UsedBytes  int64            `json:"used_bytes"`
		Categories map[string]int64 `json:"categories"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, h.mgr.TakeSnapshot(req.TotalBytes, req.UsedBytes, req.Categories))
}

func (h *Handlers) trend(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	c.JSON(http.StatusOK, h.mgr.GetTrend(days))
}

func (h *Handlers) history(c *gin.Context) {
	limit := 30
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	c.JSON(http.StatusOK, gin.H{"snapshots": h.mgr.GetHistory(limit)})
}

func (h *Handlers) report(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GenerateReport())
}
